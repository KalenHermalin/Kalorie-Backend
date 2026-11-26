package store

import (
	"context"
	"database/sql"
	"errors"
	"main/internal/models"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Edge cases for the user store. The happy paths for each log type are also in store_test.go.
// Every test commits real rows and resets the database, so tests don't depend on each other.

// ---------- Weight logs ----------

func upsertWeight(log *models.WeightLog, userID string) error {
	return inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		return s.UpsertUserWeightLog(ctx, tx, userID, log)
	})
}

func getWeight(userID, logID string) (*models.WeightLog, error) {
	var log *models.WeightLog
	err := inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		var err error
		log, err = s.GetUserWeightLogById(ctx, tx, userID, logID)
		return err
	})
	return log, err
}

func getWeights(t *testing.T, userID string) []*models.WeightLog {
	t.Helper()
	var logs []*models.WeightLog
	require.NoError(t, inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		var err error
		logs, err = s.GetUserWeightLogs(ctx, tx, userID)
		return err
	}))
	return logs
}

func TestWeightLogInsertReturnsEveryField(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-weight", nil)
	log := &models.WeightLog{ID: newID(), WeightKg: 81.5, LogDate: "2026-11-20", UpdatedAt: at(0)}

	require.NoError(t, upsertWeight(log, user.ID))

	got, err := getWeight(user.ID, log.ID)
	require.NoError(t, err)
	assert.Equal(t, log.ID, got.ID)
	assert.Equal(t, 81.5, got.WeightKg)
	assert.Equal(t, "2026-11-20", got.LogDate)
	assert.True(t, log.UpdatedAt.Equal(got.UpdatedAt))
	assert.Nil(t, got.DeletedAt)
}

func TestWeightLogConflictResolution(t *testing.T) {
	tests := []struct {
		name       string
		updatedAt  time.Time
		wantErr    error
		wantWeight float64
	}{
		{"newer wins", at(10), nil, 90},
		{"older is rejected", at(-10), ErrNoRowsAffected, 80},
		{"same timestamp is rejected", at(0), ErrNoRowsAffected, 80},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetDB(t)
			user := createUser(t, "fb-weight", nil)
			id := newID()
			require.NoError(t, upsertWeight(&models.WeightLog{ID: id, WeightKg: 80, LogDate: "2026-11-20", UpdatedAt: at(0)}, user.ID))

			err := upsertWeight(&models.WeightLog{ID: id, WeightKg: 90, LogDate: "2026-11-20", UpdatedAt: tc.updatedAt}, user.ID)
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.wantErr)
			}

			got, err := getWeight(user.ID, id)
			require.NoError(t, err)
			assert.Equal(t, tc.wantWeight, got.WeightKg)
		})
	}
}

func TestWeightLogSoftDeleteRoundTrip(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-weight", nil)
	id := newID()
	require.NoError(t, upsertWeight(&models.WeightLog{ID: id, WeightKg: 80, LogDate: "2026-11-20", UpdatedAt: at(0)}, user.ID))

	deletedAt := at(5)
	require.NoError(t, upsertWeight(&models.WeightLog{ID: id, WeightKg: 80, LogDate: "2026-11-20", UpdatedAt: at(5), DeletedAt: &deletedAt}, user.ID))

	got, err := getWeight(user.ID, id)
	require.NoError(t, err)
	require.NotNil(t, got.DeletedAt)
	assert.True(t, deletedAt.Equal(*got.DeletedAt))

	// Restoring (deleted_at back to null) with a newer timestamp also works.
	require.NoError(t, upsertWeight(&models.WeightLog{ID: id, WeightKg: 80, LogDate: "2026-11-20", UpdatedAt: at(6)}, user.ID))
	got, err = getWeight(user.ID, id)
	require.NoError(t, err)
	assert.Nil(t, got.DeletedAt)
}

// Weight logs are unique per user per day, and the conflict target is (user_id, log_date), not id.
// A second log for the same day with a different id updates the existing row and keeps the first id.
func TestWeightLogSameDayDifferentIdUpdatesExistingRow(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-weight", nil)
	firstID, secondID := newID(), newID()
	require.NoError(t, upsertWeight(&models.WeightLog{ID: firstID, WeightKg: 80, LogDate: "2026-11-20", UpdatedAt: at(0)}, user.ID))

	require.NoError(t, upsertWeight(&models.WeightLog{ID: secondID, WeightKg: 85, LogDate: "2026-11-20", UpdatedAt: at(1)}, user.ID))

	logs := getWeights(t, user.ID)
	require.Len(t, logs, 1)
	assert.Equal(t, firstID, logs[0].ID)
	assert.Equal(t, 85.0, logs[0].WeightKg)

	_, err := getWeight(user.ID, secondID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestWeightLogSameDayDifferentUsers(t *testing.T) {
	resetDB(t)
	a := createUser(t, "fb-a", nil)
	b := createUser(t, "fb-b", nil)

	require.NoError(t, upsertWeight(&models.WeightLog{ID: newID(), WeightKg: 80, LogDate: "2026-11-20", UpdatedAt: at(0)}, a.ID))
	require.NoError(t, upsertWeight(&models.WeightLog{ID: newID(), WeightKg: 60, LogDate: "2026-11-20", UpdatedAt: at(0)}, b.ID))

	assert.Len(t, getWeights(t, a.ID), 1)
	assert.Len(t, getWeights(t, b.ID), 1)
}

func TestWeightLogUnknownUser(t *testing.T) {
	resetDB(t)
	err := upsertWeight(&models.WeightLog{ID: newID(), WeightKg: 80, LogDate: "2026-11-20", UpdatedAt: at(0)}, newID())
	assert.ErrorIs(t, err, ErrUserForeignKeyConstraint)
}

func TestWeightLogInvalidDate(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-weight", nil)
	err := upsertWeight(&models.WeightLog{ID: newID(), WeightKg: 80, LogDate: "2026-02-30", UpdatedAt: at(0)}, user.ID)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoRowsAffected)
}

func TestGetWeightLogByIdNotFound(t *testing.T) {
	resetDB(t)
	owner := createUser(t, "fb-owner", nil)
	other := createUser(t, "fb-other", nil)
	id := newID()
	require.NoError(t, upsertWeight(&models.WeightLog{ID: id, WeightKg: 80, LogDate: "2026-11-20", UpdatedAt: at(0)}, owner.ID))

	_, err := getWeight(owner.ID, newID())
	assert.ErrorIs(t, err, ErrNotFound, "unknown log id")

	_, err = getWeight(other.ID, id)
	assert.ErrorIs(t, err, ErrNotFound, "another user's log must not be returned")
}

func TestGetUserWeightLogs(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-weight", nil)
	other := createUser(t, "fb-other", nil)

	t.Run("no logs", func(t *testing.T) {
		assert.Empty(t, getWeights(t, user.ID))
	})

	deletedAt := at(0)
	for _, l := range []*models.WeightLog{
		{ID: newID(), WeightKg: 80, LogDate: "2026-11-18", UpdatedAt: at(0)},
		{ID: newID(), WeightKg: 81, LogDate: "2026-11-20", UpdatedAt: at(0), DeletedAt: &deletedAt},
		{ID: newID(), WeightKg: 82, LogDate: "2026-11-19", UpdatedAt: at(0)},
	} {
		require.NoError(t, upsertWeight(l, user.ID))
	}
	require.NoError(t, upsertWeight(&models.WeightLog{ID: newID(), WeightKg: 50, LogDate: "2026-11-21", UpdatedAt: at(0)}, other.ID))

	t.Run("newest date first, own logs only, deleted included", func(t *testing.T) {
		logs := getWeights(t, user.ID)
		require.Len(t, logs, 3)
		assert.Equal(t, []string{"2026-11-20", "2026-11-19", "2026-11-18"},
			[]string{logs[0].LogDate, logs[1].LogDate, logs[2].LogDate})
		assert.NotNil(t, logs[0].DeletedAt)
	})
}

// ---------- Settings ----------

func getSettings(userID string) (*models.UserSettings, error) {
	var settings *models.UserSettings
	err := inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		var err error
		settings, err = s.GetUserSettings(ctx, tx, userID)
		return err
	})
	return settings, err
}

func updateSettings(userID string, settings *models.UserSettings) error {
	return inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		return s.UpdateUserSettings(ctx, tx, userID, settings)
	})
}

func TestUpdateUserSettingsEveryField(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-settings", nil)
	later := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond) // must be after the trigger's now()
	deletedAt := later
	want := &models.UserSettings{
		Units: "imperial", CaloriesTarget: 2500, ProteinTarget: 180, CarbsTarget: 250, FatTarget: 80,
		Theme: "light", UpdatedAt: later, DeletedAt: &deletedAt,
	}

	require.NoError(t, updateSettings(user.ID, want))

	got, err := getSettings(user.ID)
	require.NoError(t, err)
	AssertSettingsEqual(t, want, got)
}

func TestUpdateUserSettingsConflictResolution(t *testing.T) {
	stored := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	tests := []struct {
		name      string
		updatedAt time.Time
		wantErr   error
		wantCals  int
	}{
		{"newer wins", stored.Add(time.Minute), nil, 3000},
		{"older is rejected", stored.Add(-time.Minute), ErrNoRowsAffected, 2500},
		{"same timestamp is rejected", stored, ErrNoRowsAffected, 2500},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetDB(t)
			user := createUser(t, "fb-settings", nil)
			base := models.UserSettings{Units: "metric", CaloriesTarget: 2500, ProteinTarget: 150, CarbsTarget: 200, FatTarget: 70, Theme: "dark", UpdatedAt: stored}
			require.NoError(t, updateSettings(user.ID, &base))

			next := base
			next.CaloriesTarget = 3000
			next.UpdatedAt = tc.updatedAt
			err := updateSettings(user.ID, &next)
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.wantErr)
			}

			got, err := getSettings(user.ID)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCals, got.CaloriesTarget)
		})
	}
}

func TestUpdateUserSettingsInvalidTheme(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-settings", nil)
	err := updateSettings(user.ID, &models.UserSettings{
		Units: "metric", CaloriesTarget: 2000, ProteinTarget: 150, CarbsTarget: 200, FatTarget: 70,
		Theme: "purple", UpdatedAt: time.Now().Add(time.Hour),
	})
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoRowsAffected, "a constraint violation is a real error, not a lost conflict")
}

func TestUpdateUserSettingsUnknownUser(t *testing.T) {
	resetDB(t)
	err := updateSettings(newID(), &models.UserSettings{Units: "metric", Theme: "dark", UpdatedAt: time.Now().Add(time.Hour)})
	assert.ErrorIs(t, err, ErrNoRowsAffected)
}

func TestGetUserSettingsUnknownUser(t *testing.T) {
	resetDB(t)
	_, err := getSettings(newID())
	assert.ErrorIs(t, err, ErrNotFound)
}

// ---------- Exercise logs ----------

func exerciseLog(id, exerciseID string, updatedAt time.Time, sets ...*models.ExerciseSet) *models.FullExerciseLog {
	for _, s := range sets {
		s.LogId = id
	}
	if sets == nil {
		sets = []*models.ExerciseSet{}
	}
	return &models.FullExerciseLog{
		ExerciseLog:  &models.ExerciseLog{ID: id, ExerciseId: exerciseID, CreatedAt: at(0), UpdatedAt: updatedAt},
		ExerciseSets: sets,
	}
}

func exerciseSet(id string, number, reps int, weight float32, updatedAt time.Time) *models.ExerciseSet {
	return &models.ExerciseSet{ID: id, Set_number: number, Reps: reps, Weight: weight, UpdatedAt: updatedAt}
}

func upsertExercise(userID string, log *models.FullExerciseLog) error {
	return inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		return s.UpsertUserExerciseLog(ctx, tx, userID, log)
	})
}

func getExercise(userID, logID string) (*models.FullExerciseLog, error) {
	var log *models.FullExerciseLog
	err := inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		var err error
		log, err = s.GetUserExerciseLogById(ctx, tx, userID, logID)
		return err
	})
	return log, err
}

func getExercises(t *testing.T, userID string) []*models.FullExerciseLog {
	t.Helper()
	var logs []*models.FullExerciseLog
	require.NoError(t, inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		var err error
		logs, err = s.GetUserExerciseLogs(ctx, tx, userID)
		return err
	}))
	return logs
}

func TestExerciseLogNewerUpdatesLogAndSets(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-exercise", nil)
	logID, set1, set2 := newID(), newID(), newID()
	require.NoError(t, upsertExercise(user.ID, exerciseLog(logID, "barbell-bench-press", at(0),
		exerciseSet(set1, 1, 10, 60, at(0)))))

	deletedAt := at(5)
	updated := exerciseLog(logID, "dumbbell-bench-press", at(5),
		exerciseSet(set1, 1, 8, 62.5, at(5)), // changed
		exerciseSet(set2, 2, 6, 65, at(5)),   // added
	)
	updated.ExerciseLog.DeletedAt = &deletedAt
	require.NoError(t, upsertExercise(user.ID, updated))

	got, err := getExercise(user.ID, logID)
	require.NoError(t, err)
	AssertExerciseLogEqual(t, updated, got)
}

func TestExerciseLogConflictResolution(t *testing.T) {
	tests := []struct {
		name      string
		updatedAt time.Time
		wantErr   error
		want      string
	}{
		{"older is rejected", at(-5), ErrNoRowsAffected, "barbell-bench-press"},
		{"same timestamp is rejected", at(0), ErrNoRowsAffected, "barbell-bench-press"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetDB(t)
			user := createUser(t, "fb-exercise", nil)
			logID, setID := newID(), newID()
			original := exerciseLog(logID, "barbell-bench-press", at(0), exerciseSet(setID, 1, 10, 60, at(0)))
			require.NoError(t, upsertExercise(user.ID, original))

			err := upsertExercise(user.ID, exerciseLog(logID, "squat", tc.updatedAt, exerciseSet(setID, 1, 1, 200, tc.updatedAt)))
			assert.ErrorIs(t, err, tc.wantErr)

			got, err := getExercise(user.ID, logID)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.ExerciseLog.ExerciseId)
			require.Len(t, got.ExerciseSets, 1)
			assert.Equal(t, 10, got.ExerciseSets[0].Reps, "a rejected log must not change its sets")
		})
	}
}

func TestExerciseLogOtherUsersIdIsRejected(t *testing.T) {
	resetDB(t)
	owner := createUser(t, "fb-owner", nil)
	attacker := createUser(t, "fb-other", nil)
	logID := newID()
	require.NoError(t, upsertExercise(owner.ID, exerciseLog(logID, "barbell-bench-press", at(0))))

	err := upsertExercise(attacker.ID, exerciseLog(logID, "squat", at(10)))
	assert.ErrorIs(t, err, ErrNoRowsAffected)

	got, err := getExercise(owner.ID, logID)
	require.NoError(t, err)
	assert.Equal(t, "barbell-bench-press", got.ExerciseLog.ExerciseId)
	assert.Empty(t, getExercises(t, attacker.ID))
}

func TestExerciseLogUnknownUser(t *testing.T) {
	resetDB(t)
	err := upsertExercise(newID(), exerciseLog(newID(), "barbell-bench-press", at(0)))
	assert.ErrorIs(t, err, ErrUserForeignKeyConstraint)
}

func TestExerciseLogNilSets(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-exercise", nil)
	log := exerciseLog(newID(), "barbell-bench-press", at(0))
	log.ExerciseSets = nil

	assert.Error(t, upsertExercise(user.ID, log))
	assert.Empty(t, getExercises(t, user.ID), "the log must be rolled back with the failed sets")
}

// The set upsert only updates a set that already belongs to the same log; a set id from another log is skipped.
func TestExerciseSetFromAnotherLogIsNotMoved(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-exercise", nil)
	logA, logB, setID := newID(), newID(), newID()
	require.NoError(t, upsertExercise(user.ID, exerciseLog(logA, "barbell-bench-press", at(0), exerciseSet(setID, 1, 10, 60, at(0)))))

	require.NoError(t, upsertExercise(user.ID, exerciseLog(logB, "squat", at(1), exerciseSet(setID, 1, 3, 140, at(1)))))

	a, err := getExercise(user.ID, logA)
	require.NoError(t, err)
	require.Len(t, a.ExerciseSets, 1)
	assert.Equal(t, 10, a.ExerciseSets[0].Reps)

	b, err := getExercise(user.ID, logB)
	require.NoError(t, err)
	assert.Empty(t, b.ExerciseSets)
}

func TestGetExerciseLogByIdNotFound(t *testing.T) {
	resetDB(t)
	owner := createUser(t, "fb-owner", nil)
	other := createUser(t, "fb-other", nil)
	logID := newID()
	require.NoError(t, upsertExercise(owner.ID, exerciseLog(logID, "barbell-bench-press", at(0))))

	_, err := getExercise(owner.ID, newID())
	assert.ErrorIs(t, err, ErrNotFound, "unknown log id")
	_, err = getExercise(other.ID, logID)
	assert.ErrorIs(t, err, ErrNotFound, "another user's log must not be returned")
}

// Logs returned by id go back to the client in new_logs, where the API promises arrays, never null.
func TestGetExerciseLogByIdWithoutSetsReturnsEmptySlice(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-exercise", nil)
	logID := newID()
	require.NoError(t, upsertExercise(user.ID, exerciseLog(logID, "plank", at(0))))

	got, err := getExercise(user.ID, logID)
	require.NoError(t, err)
	assert.NotNil(t, got.ExerciseSets, "exercise_sets would be encoded as null")
	assert.Empty(t, got.ExerciseSets)
}

func TestGetUserExerciseLogsOwnLogsOnly(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-exercise", nil)
	other := createUser(t, "fb-other", nil)

	assert.Empty(t, getExercises(t, user.ID), "no logs")

	mine := newID()
	require.NoError(t, upsertExercise(user.ID, exerciseLog(mine, "barbell-bench-press", at(0), exerciseSet(newID(), 1, 5, 100, at(0)))))
	require.NoError(t, upsertExercise(other.ID, exerciseLog(newID(), "squat", at(0), exerciseSet(newID(), 1, 5, 100, at(0)))))

	logs := getExercises(t, user.ID)
	require.Len(t, logs, 1)
	assert.Equal(t, mine, logs[0].ExerciseLog.ID)
	assert.Len(t, logs[0].ExerciseSets, 1)
}

// ---------- Food logs ----------

func foodLog(id, name string, updatedAt time.Time, entries ...*models.FoodLogEntry) *models.FullFoodLog {
	for _, e := range entries {
		e.LogId = id
	}
	if entries == nil {
		entries = []*models.FoodLogEntry{}
	}
	return &models.FullFoodLog{
		FoodLog:        &models.FoodLog{ID: id, Name: name, Time: at(0).Format(time.RFC3339), CreatedAt: at(0), UpdatedAt: updatedAt},
		FoodLogEntries: entries,
	}
}

func foodEntry(id, name string, cal int, updatedAt time.Time) *models.FoodLogEntry {
	return &models.FoodLogEntry{ID: id, FoodId: "food-" + name, FoodName: name, ServingId: "serving", Unit: "g",
		Cal: cal, Fat: 1.5, Carbs: 20, Protein: 10, Quantity: 2.5, UpdatedAt: updatedAt}
}

func upsertFood(userID string, log *models.FullFoodLog) error {
	return inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		return s.UpsertUserFoodLog(ctx, tx, userID, log)
	})
}

func getFood(userID, logID string) (*models.FullFoodLog, error) {
	var log *models.FullFoodLog
	err := inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		var err error
		log, err = s.GetUserFoodLogById(ctx, tx, userID, logID)
		return err
	})
	return log, err
}

func getFoods(t *testing.T, userID string) []*models.FullFoodLog {
	t.Helper()
	var logs []*models.FullFoodLog
	require.NoError(t, inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		var err error
		logs, err = s.GetUserFoodLogs(ctx, tx, userID)
		return err
	}))
	return logs
}

func TestFoodLogNewerUpdatesLogAndEntries(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-food", nil)
	logID, rice, chicken := newID(), newID(), newID()
	require.NoError(t, upsertFood(user.ID, foodLog(logID, "Lunch", at(0), foodEntry(rice, "Rice", 200, at(0)))))

	deletedAt := at(5)
	riceDeleted := foodEntry(rice, "Brown Rice", 180, at(5))
	riceDeleted.FoodId = "food-Rice" // the upsert never changes food_id, only the other fields
	riceDeleted.DeletedAt = &deletedAt
	updated := foodLog(logID, "Big Lunch", at(5), riceDeleted, foodEntry(chicken, "Chicken", 300, at(5)))
	updated.FoodLog.Time = at(30).Format(time.RFC3339)
	require.NoError(t, upsertFood(user.ID, updated))

	got, err := getFood(user.ID, logID)
	require.NoError(t, err)
	AssertFoodLogEqual(t, updated, got)
}

func TestFoodLogConflictResolution(t *testing.T) {
	tests := []struct {
		name      string
		updatedAt time.Time
	}{
		{"older is rejected", at(-5)},
		{"same timestamp is rejected", at(0)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetDB(t)
			user := createUser(t, "fb-food", nil)
			logID, entryID := newID(), newID()
			require.NoError(t, upsertFood(user.ID, foodLog(logID, "Lunch", at(0), foodEntry(entryID, "Rice", 200, at(0)))))

			err := upsertFood(user.ID, foodLog(logID, "Dinner", tc.updatedAt, foodEntry(entryID, "Pasta", 900, tc.updatedAt)))
			assert.ErrorIs(t, err, ErrNoRowsAffected)

			got, err := getFood(user.ID, logID)
			require.NoError(t, err)
			assert.Equal(t, "Lunch", got.FoodLog.Name)
			require.Len(t, got.FoodLogEntries, 1)
			assert.Equal(t, 200, got.FoodLogEntries[0].Cal, "a rejected log must not change its entries")
		})
	}
}

func TestFoodLogOtherUsersIdIsRejected(t *testing.T) {
	resetDB(t)
	owner := createUser(t, "fb-owner", nil)
	attacker := createUser(t, "fb-other", nil)
	logID := newID()
	require.NoError(t, upsertFood(owner.ID, foodLog(logID, "Lunch", at(0))))

	err := upsertFood(attacker.ID, foodLog(logID, "Hacked", at(10)))
	assert.ErrorIs(t, err, ErrNoRowsAffected)

	got, err := getFood(owner.ID, logID)
	require.NoError(t, err)
	assert.Equal(t, "Lunch", got.FoodLog.Name)
	assert.Empty(t, getFoods(t, attacker.ID))
}

func TestFoodLogUnknownUser(t *testing.T) {
	resetDB(t)
	err := upsertFood(newID(), foodLog(newID(), "Lunch", at(0)))
	assert.ErrorIs(t, err, ErrUserForeignKeyConstraint)
}

func TestFoodLogNilEntries(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-food", nil)
	log := foodLog(newID(), "Lunch", at(0))
	log.FoodLogEntries = nil

	assert.Error(t, upsertFood(user.ID, log))
	assert.Empty(t, getFoods(t, user.ID), "the log must be rolled back with the failed entries")
}

func TestFoodLogInvalidTime(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-food", nil)
	log := foodLog(newID(), "Lunch", at(0))
	log.FoodLog.Time = "not a time"

	err := upsertFood(user.ID, log)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoRowsAffected)
}

// The entry upsert only updates an entry that already belongs to the same log; an entry id from another log is skipped.
func TestFoodEntryFromAnotherLogIsNotMoved(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-food", nil)
	logA, logB, entryID := newID(), newID(), newID()
	require.NoError(t, upsertFood(user.ID, foodLog(logA, "Lunch", at(0), foodEntry(entryID, "Rice", 200, at(0)))))

	require.NoError(t, upsertFood(user.ID, foodLog(logB, "Dinner", at(1), foodEntry(entryID, "Pasta", 900, at(1)))))

	a, err := getFood(user.ID, logA)
	require.NoError(t, err)
	require.Len(t, a.FoodLogEntries, 1)
	assert.Equal(t, "Rice", a.FoodLogEntries[0].FoodName)

	b, err := getFood(user.ID, logB)
	require.NoError(t, err)
	assert.Empty(t, b.FoodLogEntries)
}

func TestGetFoodLogByIdNotFound(t *testing.T) {
	resetDB(t)
	owner := createUser(t, "fb-owner", nil)
	other := createUser(t, "fb-other", nil)
	logID := newID()
	require.NoError(t, upsertFood(owner.ID, foodLog(logID, "Lunch", at(0))))

	_, err := getFood(owner.ID, newID())
	assert.ErrorIs(t, err, ErrNotFound, "unknown log id")
	_, err = getFood(other.ID, logID)
	assert.ErrorIs(t, err, ErrNotFound, "another user's log must not be returned")
}

// Logs returned by id go back to the client in new_logs, where the API promises arrays, never null.
func TestGetFoodLogByIdWithoutEntriesReturnsEmptySlice(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-food", nil)
	logID := newID()
	require.NoError(t, upsertFood(user.ID, foodLog(logID, "Water", at(0))))

	got, err := getFood(user.ID, logID)
	require.NoError(t, err)
	assert.NotNil(t, got.FoodLogEntries, "food log entries would be encoded as null")
	assert.Empty(t, got.FoodLogEntries)
}

func TestGetUserFoodLogsOwnLogsOnly(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-food", nil)
	other := createUser(t, "fb-other", nil)

	assert.Empty(t, getFoods(t, user.ID), "no logs")

	mine := newID()
	require.NoError(t, upsertFood(user.ID, foodLog(mine, "Lunch", at(0), foodEntry(newID(), "Rice", 200, at(0)))))
	require.NoError(t, upsertFood(other.ID, foodLog(newID(), "Dinner", at(0), foodEntry(newID(), "Pasta", 900, at(0)))))

	logs := getFoods(t, user.ID)
	require.Len(t, logs, 1)
	assert.Equal(t, mine, logs[0].FoodLog.ID)
	assert.Len(t, logs[0].FoodLogEntries, 1)
}

// ---------- Workout templates ----------

func template(id, name string, createdAt, updatedAt time.Time, exercises ...*models.WorkoutTemplateExercises) *models.WorkoutTemplate {
	return &models.WorkoutTemplate{ID: id, Name: name, Exercises: exercises, CreatedAt: createdAt, UpdatedAt: updatedAt}
}

func upsertTemplate(userID string, tpl *models.WorkoutTemplate) error {
	return inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		return s.UpsertUserWorkoutTemplate(ctx, tx, userID, tpl)
	})
}

func getTemplate(userID, id string) (*models.WorkoutTemplate, error) {
	var tpl *models.WorkoutTemplate
	err := inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		var err error
		tpl, err = s.GetUserWorkoutTemplateById(ctx, tx, userID, id)
		return err
	})
	return tpl, err
}

func getTemplates(t *testing.T, userID string) []*models.WorkoutTemplate {
	t.Helper()
	var tpls []*models.WorkoutTemplate
	require.NoError(t, inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		var err error
		tpls, err = s.GetUserWorkoutTemplates(ctx, tx, userID)
		return err
	}))
	return tpls
}

func TestWorkoutTemplateNewerUpdatesNameAndExercises(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-template", nil)
	id := newID()
	require.NoError(t, upsertTemplate(user.ID, template(id, "Push", at(0), at(0),
		&models.WorkoutTemplateExercises{ExerciseID: "barbell-bench-press", Sets: 3},
		&models.WorkoutTemplateExercises{ExerciseID: "overhead-press", Sets: 3},
	)))

	deletedAt := at(5)
	updated := template(id, "Push Day", at(0), at(5),
		&models.WorkoutTemplateExercises{ExerciseID: "overhead-press", Sets: 4}, // reordered
		&models.WorkoutTemplateExercises{ExerciseID: "barbell-bench-press", Sets: 255},
		&models.WorkoutTemplateExercises{ExerciseID: "skull-crusher", Sets: 0},
	)
	updated.DeletedAt = &deletedAt
	require.NoError(t, upsertTemplate(user.ID, updated))

	got, err := getTemplate(user.ID, id)
	require.NoError(t, err)
	AssertWorkoutTemplateEqual(t, updated, got)
	require.NotNil(t, got.DeletedAt)
	assert.True(t, deletedAt.Equal(*got.DeletedAt))
}

func TestWorkoutTemplateClearingExercises(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-template", nil)
	id := newID()
	require.NoError(t, upsertTemplate(user.ID, template(id, "Legs", at(0), at(0),
		&models.WorkoutTemplateExercises{ExerciseID: "barbell-back-squat", Sets: 5})))

	require.NoError(t, upsertTemplate(user.ID, template(id, "Legs", at(0), at(1)))) // nil exercises

	got, err := getTemplate(user.ID, id)
	require.NoError(t, err)
	assert.NotNil(t, got.Exercises)
	assert.Empty(t, got.Exercises)
}

func TestWorkoutTemplateCreatedAtIsKept(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-template", nil)
	id := newID()
	require.NoError(t, upsertTemplate(user.ID, template(id, "Pull", at(0), at(0))))

	require.NoError(t, upsertTemplate(user.ID, template(id, "Pull", at(100), at(1))))

	got, err := getTemplate(user.ID, id)
	require.NoError(t, err)
	assert.True(t, at(0).Equal(got.CreatedAt), "created_at is set by the first sync only")
}

func TestWorkoutTemplateSameTimestampIsRejected(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-template", nil)
	id := newID()
	require.NoError(t, upsertTemplate(user.ID, template(id, "Pull", at(0), at(0))))

	err := upsertTemplate(user.ID, template(id, "Changed", at(0), at(0)))
	assert.ErrorIs(t, err, ErrNoRowsAffected)

	got, err := getTemplate(user.ID, id)
	require.NoError(t, err)
	assert.Equal(t, "Pull", got.Name)
}

func TestWorkoutTemplateOtherUsersIdIsRejected(t *testing.T) {
	resetDB(t)
	owner := createUser(t, "fb-owner", nil)
	attacker := createUser(t, "fb-other", nil)
	id := newID()
	require.NoError(t, upsertTemplate(owner.ID, template(id, "Push", at(0), at(0))))

	err := upsertTemplate(attacker.ID, template(id, "Hacked", at(0), at(10)))
	assert.ErrorIs(t, err, ErrNoRowsAffected)

	got, err := getTemplate(owner.ID, id)
	require.NoError(t, err)
	assert.Equal(t, "Push", got.Name)
	assert.Empty(t, getTemplates(t, attacker.ID))
}

func TestWorkoutTemplateUnknownUser(t *testing.T) {
	resetDB(t)
	err := upsertTemplate(newID(), template(newID(), "Push", at(0), at(0)))
	assert.ErrorIs(t, err, ErrUserForeignKeyConstraint)
}

func TestGetWorkoutTemplateByIdNotFound(t *testing.T) {
	resetDB(t)
	owner := createUser(t, "fb-owner", nil)
	other := createUser(t, "fb-other", nil)
	id := newID()
	require.NoError(t, upsertTemplate(owner.ID, template(id, "Push", at(0), at(0))))

	_, err := getTemplate(owner.ID, newID())
	assert.ErrorIs(t, err, ErrNotFound, "unknown template id")
	_, err = getTemplate(other.ID, id)
	assert.ErrorIs(t, err, ErrNotFound, "another user's template must not be returned")
}

func TestGetUserWorkoutTemplates(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-template", nil)
	other := createUser(t, "fb-other", nil)

	t.Run("no templates is an empty slice", func(t *testing.T) {
		tpls := getTemplates(t, user.ID)
		assert.NotNil(t, tpls)
		assert.Empty(t, tpls)
	})

	oldest, middle, newest := newID(), newID(), newID()
	deletedAt := at(0)
	deleted := template(middle, "B", at(1), at(1))
	deleted.DeletedAt = &deletedAt
	require.NoError(t, upsertTemplate(user.ID, template(oldest, "A", at(0), at(0))))
	require.NoError(t, upsertTemplate(user.ID, template(newest, "C", at(2), at(2))))
	require.NoError(t, upsertTemplate(user.ID, deleted))
	require.NoError(t, upsertTemplate(other.ID, template(newID(), "Other", at(3), at(3))))

	t.Run("newest created first, own templates only, deleted included", func(t *testing.T) {
		tpls := getTemplates(t, user.ID)
		require.Len(t, tpls, 3)
		assert.Equal(t, []string{newest, middle, oldest}, []string{tpls[0].ID, tpls[1].ID, tpls[2].ID})
		assert.NotNil(t, tpls[1].DeletedAt)
	})
}

// ---------- Transactions ----------

func TestUserStoreWithTxRollsBackOnError(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-tx", nil)
	fnErr := errors.New("boom")

	err := inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		if err := s.UpsertUserWeightLog(ctx, tx, user.ID, &models.WeightLog{ID: newID(), WeightKg: 80, LogDate: "2026-11-20", UpdatedAt: at(0)}); err != nil {
			return err
		}
		return fnErr
	})
	assert.Equal(t, fnErr, err, "WithTx must return fn's error unchanged")
	assert.Empty(t, getWeights(t, user.ID), "the weight log should have been rolled back")
}

func TestUserStoreWithTxCommitsOnSuccess(t *testing.T) {
	resetDB(t)
	user := createUser(t, "fb-tx", nil)

	require.NoError(t, upsertWeight(&models.WeightLog{ID: newID(), WeightKg: 80, LogDate: "2026-11-20", UpdatedAt: at(0)}, user.ID))
	assert.Len(t, getWeights(t, user.ID), 1)
}
