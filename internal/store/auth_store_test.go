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

func TestUpsertUserWithAuthCreatesUserAndDefaultSettings(t *testing.T) {
	resetDB(t)

	user := createUser(t, "fb-create", strPtr("create@test.com"))

	assert.NotEmpty(t, user.ID)
	require.NotNil(t, user.Email)
	assert.Equal(t, "create@test.com", *user.Email)
	assert.False(t, user.CreatedAt.IsZero())

	// The insert trigger gives every new user a settings row with defaults.
	var settings *models.UserSettings
	require.NoError(t, inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
		var err error
		settings, err = s.GetUserSettings(ctx, tx, user.ID)
		return err
	}))
	assert.Equal(t, "metric", settings.Units)
	assert.Equal(t, 2000, settings.CaloriesTarget)
	assert.Equal(t, 150, settings.ProteinTarget)
	assert.Equal(t, 200, settings.CarbsTarget)
	assert.Equal(t, 70, settings.FatTarget)
	assert.Equal(t, "system", settings.Theme)
	assert.Nil(t, settings.DeletedAt)
}

func TestUpsertUserWithAuthExistingFirebaseIdUpdatesEmail(t *testing.T) {
	resetDB(t)

	first := createUser(t, "fb-same", strPtr("old@test.com"))
	second := createUser(t, "fb-same", strPtr("new@test.com"))

	assert.Equal(t, first.ID, second.ID, "signing in again must not create a new user")
	require.NotNil(t, second.Email)
	assert.Equal(t, "new@test.com", *second.Email)
	assert.True(t, first.CreatedAt.Equal(second.CreatedAt), "created_at must not change on later sign-ins")
	assert.Equal(t, 1, countRows(t, `SELECT count(*) FROM users WHERE firebase_id = 'fb-same'`))
	assert.Equal(t, 1, countRows(t, `SELECT count(*) FROM user_settings WHERE user_id = $1`, first.ID),
		"signing in again must not create a second settings row")
}

func TestUpsertUserWithAuthNilEmail(t *testing.T) {
	resetDB(t)

	user := createUser(t, "fb-no-email", nil)
	assert.Nil(t, user.Email)

	// A later sign-in without an email clears a stored one (EXCLUDED.email is NULL).
	createUser(t, "fb-had-email", strPtr("had@test.com"))
	again := createUser(t, "fb-had-email", nil)
	assert.Nil(t, again.Email)
}

func TestUpsertUserWithAuthDifferentFirebaseIds(t *testing.T) {
	resetDB(t)

	a := createUser(t, "fb-a", strPtr("same@test.com"))
	b := createUser(t, "fb-b", strPtr("same@test.com"))

	assert.NotEqual(t, a.ID, b.ID, "the email is not unique; users are keyed by firebase id")
}

func TestLookupFirebaseIdToUserId(t *testing.T) {
	resetDB(t)
	authStore := NewPostgressAuthStore(testDB)
	user := createUser(t, "fb-lookup", nil)
	createUser(t, "fb-other", nil)

	t.Run("found", func(t *testing.T) {
		id, err := authStore.LookupFirebaseIdToUserId(context.Background(), "fb-lookup")
		require.NoError(t, err)
		assert.Equal(t, user.ID, id)
	})
	t.Run("not found", func(t *testing.T) {
		id, err := authStore.LookupFirebaseIdToUserId(context.Background(), "fb-missing")
		assert.ErrorIs(t, err, ErrNotFound)
		assert.Empty(t, id)
	})
	t.Run("empty id", func(t *testing.T) {
		_, err := authStore.LookupFirebaseIdToUserId(context.Background(), "")
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestLookupUserIdToFirebase(t *testing.T) {
	resetDB(t)
	authStore := NewPostgressAuthStore(testDB)
	user := createUser(t, "fb-reverse", nil)

	t.Run("found", func(t *testing.T) {
		firebaseID, err := authStore.LookupUserIdToFirebase(context.Background(), user.ID)
		require.NoError(t, err)
		assert.Equal(t, "fb-reverse", firebaseID)
	})
	t.Run("not found", func(t *testing.T) {
		firebaseID, err := authStore.LookupUserIdToFirebase(context.Background(), newID())
		assert.ErrorIs(t, err, ErrNotFound)
		assert.Empty(t, firebaseID)
	})
	t.Run("not a uuid", func(t *testing.T) {
		_, err := authStore.LookupUserIdToFirebase(context.Background(), "not-a-uuid")
		assert.Error(t, err)
	})
}

func TestDeleteUserRemovesUserAndEverythingTheyOwn(t *testing.T) {
	resetDB(t)
	authStore := NewPostgressAuthStore(testDB)
	user := createUser(t, "fb-delete", strPtr("delete@test.com"))
	keep := createUser(t, "fb-keep", nil)

	// Give both users one of everything.
	for _, owner := range []string{user.ID, keep.ID} {
		require.NoError(t, inUserTx(func(ctx context.Context, s *postgressUserRepo, tx *sql.Tx) error {
			if err := s.UpsertUserWeightLog(ctx, tx, owner, &models.WeightLog{ID: newID(), WeightKg: 80, LogDate: "2026-11-20", UpdatedAt: at(0)}); err != nil {
				return err
			}
			logID := newID()
			if err := s.UpsertUserExerciseLog(ctx, tx, owner, &models.FullExerciseLog{
				ExerciseLog:  &models.ExerciseLog{ID: logID, ExerciseId: "barbell-bench-press", CreatedAt: at(0), UpdatedAt: at(0)},
				ExerciseSets: []*models.ExerciseSet{{ID: newID(), LogId: logID, Set_number: 1, Weight: 60, Reps: 10, UpdatedAt: at(0)}},
			}); err != nil {
				return err
			}
			foodID := newID()
			if err := s.UpsertUserFoodLog(ctx, tx, owner, &models.FullFoodLog{
				FoodLog:        &models.FoodLog{ID: foodID, Name: "Lunch", Time: at(0).Format(time.RFC3339), CreatedAt: at(0), UpdatedAt: at(0)},
				FoodLogEntries: []*models.FoodLogEntry{{ID: newID(), LogId: foodID, FoodId: "f", FoodName: "Rice", ServingId: "s", Unit: "g", Cal: 100, Quantity: 1, UpdatedAt: at(0)}},
			}); err != nil {
				return err
			}
			return s.UpsertUserWorkoutTemplate(ctx, tx, owner, &models.WorkoutTemplate{ID: newID(), Name: "Push", CreatedAt: at(0), UpdatedAt: at(0)})
		}))
	}

	err := authStore.WithTx(context.Background(), func(tx *sql.Tx) error {
		return authStore.DeleteUser(context.Background(), tx, user.ID)
	})
	require.NoError(t, err)

	_, err = authStore.LookupFirebaseIdToUserId(context.Background(), "fb-delete")
	assert.ErrorIs(t, err, ErrNotFound)

	for _, table := range []string{"user_settings", "weight_logs", "exercise_logs", "food_logs", "workout_templates"} {
		assert.Equal(t, 0, countRows(t, `SELECT count(*) FROM `+table+` WHERE user_id = $1`, user.ID), "%s not removed", table)
		assert.Equal(t, 1, countRows(t, `SELECT count(*) FROM `+table+` WHERE user_id = $1`, keep.ID), "%s of another user removed", table)
	}
	assert.Equal(t, 1, countRows(t, `SELECT count(*) FROM exercise_sets`), "only the other user's set should remain")
	assert.Equal(t, 1, countRows(t, `SELECT count(*) FROM food_log_items`), "only the other user's food entry should remain")
}

func TestDeleteUserUnknownId(t *testing.T) {
	resetDB(t)
	authStore := NewPostgressAuthStore(testDB)

	err := authStore.WithTx(context.Background(), func(tx *sql.Tx) error {
		return authStore.DeleteUser(context.Background(), tx, newID())
	})
	assert.ErrorIs(t, err, ErrNoRowsAffected)
}

func TestDeleteUserTwice(t *testing.T) {
	resetDB(t)
	authStore := NewPostgressAuthStore(testDB)
	user := createUser(t, "fb-twice", nil)

	del := func() error {
		return authStore.WithTx(context.Background(), func(tx *sql.Tx) error {
			return authStore.DeleteUser(context.Background(), tx, user.ID)
		})
	}
	require.NoError(t, del())
	assert.ErrorIs(t, del(), ErrNoRowsAffected)
}

func TestAuthWithTx(t *testing.T) {
	resetDB(t)
	authStore := NewPostgressAuthStore(testDB)

	t.Run("commits when fn succeeds", func(t *testing.T) {
		err := authStore.WithTx(context.Background(), func(tx *sql.Tx) error {
			_, err := authStore.UpsertUserWithAuth(context.Background(), tx, &models.AuthPayload{ID: "fb-commit"})
			return err
		})
		require.NoError(t, err)
		_, err = authStore.LookupFirebaseIdToUserId(context.Background(), "fb-commit")
		assert.NoError(t, err)
	})

	t.Run("rolls back and returns fn's error unchanged", func(t *testing.T) {
		fnErr := errors.New("boom")
		err := authStore.WithTx(context.Background(), func(tx *sql.Tx) error {
			if _, err := authStore.UpsertUserWithAuth(context.Background(), tx, &models.AuthPayload{ID: "fb-rollback"}); err != nil {
				return err
			}
			return fnErr
		})
		assert.Same(t, fnErr, err)
		_, err = authStore.LookupFirebaseIdToUserId(context.Background(), "fb-rollback")
		assert.ErrorIs(t, err, ErrNotFound, "the user should have been rolled back")
	})
}
