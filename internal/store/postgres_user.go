package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"main/internal/models"
	"time"

	"github.com/lib/pq" // The underscore is required
)

type postgressUserRepo struct {
	db *sql.DB
}

var ErrNoRowsAffected = errors.New("no rows affected")
var ErrNotFound = errors.New("no row found")
var ErrUserForeignKeyConstraint = errors.New("no user exists")
var ErrLogIdForeignKeyConstraint = errors.New("no log id exists")
var foreignKeyConstraintErrorCode = "23503"

func NewPostgressUserStore(db *sql.DB) *postgressUserRepo {
	return &postgressUserRepo{
		db: db,
	}

}

// UpsertUserWeightLog updates or inserts a users weight log into the database.
// It returns an error if something goes wrong and rollsback the transaction
//
// # Parameters
//   - ctx is a [context.Context] retrieved from the http request
//   - tx is a non-nil *[sql.Tx]
//   - userId is a non empty string that is a in the users table (valid user)
//   - payload is a non-nil *[models.WeightLog]
//
// # Errors
//   - Returns [ErrUserForeignKeyConstraint] if the userId provided is no longer in the database, happens when a user deletes account but a session, with a valid access token, is still available on another device. (rare because of 15 min validity)
//   - Returns [ErrNoRowsAffected] when payload.updated_at is older than stored weightlog. Happens when another device synced a newer log data
//   - Returns fmt.error for any other errors
func (us *postgressUserRepo) UpsertUserWeightLog(ctx context.Context, tx *sql.Tx, userId string, payload *models.WeightLog) error {
	query := `
        INSERT INTO weight_logs (id, user_id, weight_kg, log_date, updated_at, deleted_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (user_id, log_date) 
        DO UPDATE SET 
            weight_kg = EXCLUDED.weight_kg,
            updated_at = EXCLUDED.updated_at,
            deleted_at = EXCLUDED.deleted_at
        WHERE EXCLUDED.updated_at > weight_logs.updated_at;`

	res, err := tx.ExecContext(ctx, query,
		payload.ID,        // $1
		userId,            // $2
		payload.WeightKg,  // $3
		payload.LogDate,   // $4
		payload.UpdatedAt, // $5
		payload.DeletedAt, // $6
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return fmt.Errorf("upsert weight log for user %s: %w", userId, ErrUserForeignKeyConstraint)

			}
		}
		return fmt.Errorf("upsert weight log for %s: %w", userId, err)
	}

	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		return fmt.Errorf("upsert weight log for %s: %w", userId, ErrNoRowsAffected)
	}
	return nil

}

// GetUserWeightLogById retrieves a users weight log by the log id
// It returns a tuple of ([*models.WeightLog], [error]), one of which is nil, and one will have a value.
// Standard is to check if err == nil, then access weight log
//
// # Parameters
//   - ctx is a [context.Context] retrieved from the http request
//   - tx is a non-nil *[sql.Tx]
//   - userId is a non empty string that is a in the users table (valid user)
//   - logId is a non empty string of a weight log stored in the database
//
// # Errors
//   - Returns [ErrUserForeignKeyConstraint] if the userId provided is no longer in the database, happens when a user deletes account but a session, with a valid access token, is still available on another device. (rare because of 15 min validity)
//   - Returns [ErrNotFound] when no log with the specified id is found.
//   - Returns fmt.error for any other errors
func (us *postgressUserRepo) GetUserWeightLogById(ctx context.Context, tx *sql.Tx, userId string, logId string) (*models.WeightLog, error) {
	query := `
	SELECT id, weight_kg, log_date, updated_at, deleted_at
	FROM weight_logs
	WHERE user_id = $1 AND id = $2;`

	log := &models.WeightLog{}
	err := tx.QueryRowContext(ctx, query, userId, logId).Scan(
		&log.ID,
		&log.WeightKg,
		&log.LogDate,
		&log.UpdatedAt,
		&log.DeletedAt,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return nil, fmt.Errorf("get weight log %s for user %s: %w", logId, userId, ErrUserForeignKeyConstraint)
			}
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get weight log %s for user %s: %w", logId, userId, ErrNotFound)
		}
		return nil, fmt.Errorf("get weight log %s for user %s: %w", logId, userId, err)
	}

	return log, nil
}

// GetUserWeightLogs retrieves all users weight logs
// It returns a tuple of ([][*models.WeightLog], [error]), one of which is nil, and one will have a value.
// Standard is to check if err == nil, then access weight log
//
// # Parameters
//   - ctx is a [context.Context] retrieved from the http request
//   - tx is a non-nil *[sql.Tx]
//   - userId is a non empty string that is a in the users table (valid user)
//
// # Errors
//   - Returns [ErrUserForeignKeyConstraint] if the userId provided is no longer in the database, happens when a user deletes account but a session, with a valid access token, is still available on another device. (rare because of 15 min validity)
//   - Returns fmt.error for any other errors
func (us *postgressUserRepo) GetUserWeightLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*models.WeightLog, error) {
	query := `
    SELECT id, weight_kg, log_date, updated_at, deleted_at
    FROM weight_logs
    WHERE user_id = $1
    ORDER BY log_date DESC;` // Good practice to sort by date

	rows, err := tx.QueryContext(ctx, query, userId)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return nil, fmt.Errorf("get weight logs for user %s: %w", userId, ErrUserForeignKeyConstraint)
			}
		}
		return nil, fmt.Errorf("get weight logs %s: %w", userId, err)
	}
	defer rows.Close() // Critical: Always close rows to free up DB connections

	var logs []*models.WeightLog

	for rows.Next() {
		log := &models.WeightLog{}
		err := rows.Scan(
			&log.ID,
			&log.WeightKg,
			&log.LogDate,
			&log.UpdatedAt,
			&log.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("get weight logs scan %s: %w", userId, err)
		}
		logs = append(logs, log)
	}

	// Check for errors that occurred during iteration
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("get weight logs %s: %w", userId, err)
	}

	return logs, nil
}

// GetUserSettings retrieves the specified users settings
// It returns a tuple of ([*models.UserSettings], [error]), one of which is nil, and one will have a value.
// Standard is to check if err == nil, then access user settings
//
// # Parameters
//   - ctx is a [context.Context] retrieved from the http request
//   - tx is a non-nil *[sql.Tx]
//   - userId is a non empty string that is a in the users table (valid user)
//
// # Errors
//   - Returns [ErrUserForeignKeyConstraint] if the userId provided is no longer in the database, happens when a user deletes account but a session, with a valid access token, is still available on another device. (rare because of 15 min validity)
//   - Returns [ErrNotFound] when no setting row is found.
//   - Returns fmt.error for any other errors
func (us *postgressUserRepo) GetUserSettings(ctx context.Context, tx *sql.Tx, userId string) (*models.UserSettings, error) {

	query := `
SELECT 
    units, 
    calories_target, 
    protein_target, 
    carbs_target, 
    fat_target, 
    updated_at, 
    deleted_at, 
    theme 
FROM user_settings 
WHERE user_id = $1;`

	settings := &models.UserSettings{}
	err := tx.QueryRowContext(ctx, query, userId).Scan(
		&settings.Units,          // string
		&settings.CaloriesTarget, // int
		&settings.ProteinTarget,  // int
		&settings.CarbsTarget,    // int
		&settings.FatTarget,      // int
		&settings.UpdatedAt,      // time.Time
		&settings.DeletedAt,      // *time.Time (pointer handles the NULL)
		&settings.Theme,          // string
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return nil, fmt.Errorf("get user settings for user %s: %w", userId, ErrUserForeignKeyConstraint)
			}
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get settings %s: %w", userId, ErrNotFound)
		}
		return nil, fmt.Errorf("get settings %s: %w", userId, err)
	}
	return settings, nil

}

// UpdateUserSettings updates a users setting in the database
// It returns an [error] if something goes wrong.
// Standard is to check if err == nil, then proceed
//
// # Parameters
//   - ctx is a [context.Context] retrieved from the http request
//   - tx is a non-nil *[sql.Tx]
//   - userId is a non empty string that is a in the users table (valid user)
//   - settings is a non nil *[models.UserSettings]
//
// # Errors
//   - Returns [ErrUserForeignKeyConstraint] if the userId provided is no longer in the database, happens when a user deletes account but a session, with a valid access token, is still available on another device. (rare because of 15 min validity)
//   - Returns [ErrNoRowsAffected] when the settings.updated_at is older than the store updated_at. Happens when another device changed the settings
//   - Returns fmt.error for any other errors
func (us *postgressUserRepo) UpdateUserSettings(ctx context.Context, tx *sql.Tx, userId string, settings *models.UserSettings) error {

	// 1. Upsert User (Update last_login if you have that column)
	query := `UPDATE user_settings 
          SET units = $2, calories_target = $3, protein_target = $4, carbs_target = $5, fat_target = $6, updated_at = $7, deleted_at = $8, theme = $9 
          WHERE user_id = $1 AND $7 > updated_at`
	res, err := tx.ExecContext(ctx, query, userId, settings.Units, settings.CaloriesTarget, settings.ProteinTarget, settings.CarbsTarget, settings.FatTarget, settings.UpdatedAt, settings.DeletedAt, settings.Theme)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return fmt.Errorf("update user settings for user %s: %w", userId, ErrUserForeignKeyConstraint)
			}
		}
		return fmt.Errorf("update settings %s: %w", userId, err)
	}

	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		return fmt.Errorf("update settings %s: %w", userId, ErrNoRowsAffected)
	}
	return nil

}

// TODO: Finish this docstring, cant finish because i have to decide if i want to redesign the function or what maybe combine the sql into one statement that does both the log and sets in one
// UpsertUserExerciseLog updates or inserts a users exercise log and its sets
// It returns a [error] if something goes wrong
// Standard is to check if err == nil, then continue
//
// # Parameters
//   - ctx is a [context.Context] retrieved from the http request
//   - tx is a non-nil *[sql.Tx]
//   - userId is a non empty string that is a in the users table (valid user)
//   - payload is a non-nil *[models.FullExerciseLog]
//
// # Errors
//   - Returns [ErrUserForeignKeyConstraint] if the userId provided is no longer in the database, happens when a user deletes account but a session, with a valid access token, is still available on another device. (rare because of 15 min validity)
//   - Returns [ErrNotFound] when no log with the specified id is found.
//   - Returns fmt.error for any other errors
func (us *postgressUserRepo) UpsertUserExerciseLog(ctx context.Context, tx *sql.Tx, userId string, payload *models.FullExerciseLog) error {

	query := `
	WITH upsert_log AS (
		INSERT INTO exercise_logs (id, user_id, exercise_id, created_at, updated_at, deleted_at)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (id) 
	DO UPDATE SET
		exercise_id = EXCLUDED.exercise_id,
		updated_at = EXCLUDED.updated_at,
		deleted_at = EXCLUDED.deleted_at
	WHERE EXCLUDED.updated_at > exercise_logs.updated_at
	)
	INSERT INTO exercise_sets (id, log_id, set_number, weight, reps, updated_at, deleted_at)
	SELECT
		unnest($7::uuid[]),
		$1,
		unnest($8::integer[]),
		unnest($9::real[]),
		unnest($10::integer[]),
		unnest($11::TIMESTAMPTZ[]),
		unnest($12::TIMESTAMPTZ[])
	ON CONFLICT (id) 
	DO UPDATE SET
		set_number = EXCLUDED.set_number,
		reps = EXCLUDED.reps,
		updated_at = EXCLUDED.updated_at,
		weight = EXCLUDED.weight,
		deleted_at = EXCLUDED.deleted_at
	WHERE EXCLUDED.updated_at > exercise_sets.updated_at;
`
	/*query := `
	INSERT INTO exercise_logs (id, user_id, exercise_id, created_at, updated_at, deleted_at)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (id)
	DO UPDATE SET
	    exercise_id = EXCLUDED.exercise_id,
	    updated_at = EXCLUDED.updated_at,
	    deleted_at = EXCLUDED.deleted_at
	WHERE EXCLUDED.updated_at > exercise_logs.updated_at;`*/
	ids := make([]string, len(payload.ExerciseSets))
	reps := make([]int, len(payload.ExerciseSets))
	setNumbers := make([]int, len(payload.ExerciseSets))
	weights := make([]float32, len(payload.ExerciseSets))
	updateAts := make([]time.Time, len(payload.ExerciseSets))
	deletedAts := make([]*time.Time, len(payload.ExerciseSets))

	for i, s := range payload.ExerciseSets {
		ids[i] = s.ID
		reps[i] = s.Reps
		setNumbers[i] = s.Set_number
		weights[i] = s.Weight
		updateAts[i] = s.UpdatedAt
		deletedAts[i] = s.DeletedAt

	}
	res, err := tx.ExecContext(ctx, query,
		payload.ExerciseLog.ID,         // $1
		userId,                         // $2
		payload.ExerciseLog.ExerciseId, // $3
		payload.ExerciseLog.CreatedAt,  // $4
		payload.ExerciseLog.UpdatedAt,  // $5
		payload.ExerciseLog.DeletedAt,  // $6
		pq.Array(ids),                  // $7
		pq.Array(setNumbers),           // $8
		pq.Array(weights),              // $9
		pq.Array(reps),                 // $10
		pq.Array(updateAts),            // $11
		pq.Array(deletedAts),           // $12

	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return fmt.Errorf("upsert exercise log %s for user %s: %w", payload.ExerciseLog.ID, userId, ErrUserForeignKeyConstraint)
			}
		}
		return fmt.Errorf("upsert exercise log %s: %w", userId, err)
	}
	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		return fmt.Errorf("upsert exercise log %s: %w", userId, ErrNoRowsAffected)
	}
	/*err = us.upsertUserExerciseSets(ctx, tx, payload.ExerciseSets)
	if err != nil {
		return err
	}*/

	return nil

}

// TODO: Rework sql to use a join (inner or left)
func (us *postgressUserRepo) GetUserExerciseLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*models.FullExerciseLog, error) {
	queryExerciseLog := `
    SELECT id, exercise_id, created_at, updated_at, deleted_at
    FROM exercise_logs
    WHERE user_id = $1
    ORDER BY created_at DESC;` // Good practice to sort by date

	queryExerciseSet := `
    SELECT id, log_id, set_number, weight, reps, updated_at, deleted_at
    FROM exercise_sets WHERE log_id = ANY($1) 
	ORDER BY updated_at DESC;
    `
	rows, err := tx.QueryContext(ctx, queryExerciseLog, userId)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return nil, fmt.Errorf("get exercise logs for user %s: %w", userId, ErrUserForeignKeyConstraint)
			}
		}
		return nil, fmt.Errorf("get exercise logs %s: %w", userId, err)
	}
	defer rows.Close()

	var FullExerciselogs []*models.FullExerciseLog
	for rows.Next() {
		log := &models.ExerciseLog{}
		err := rows.Scan(
			&log.ID,
			&log.ExerciseId,
			&log.CreatedAt,
			&log.UpdatedAt,
			&log.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("get exercise logs scan %s: %w", userId, err)
		}
		fullLog := models.FullExerciseLog{ExerciseLog: log, ExerciseSets: nil}
		FullExerciselogs = append(FullExerciselogs, &fullLog)

	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("get exercise logs %s: %w", userId, err)
	}
	rows.Close()
	logIds := []string{}
	logMap := map[string][]*models.ExerciseSet{}
	for _, log := range FullExerciselogs {
		logIds = append(logIds, log.ExerciseLog.ID)
		logMap[log.ExerciseLog.ID] = []*models.ExerciseSet{}

	}
	rows, err = tx.QueryContext(ctx, queryExerciseSet, logIds)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return nil, fmt.Errorf("get exercise sets for log ids: %w", ErrLogIdForeignKeyConstraint)
			}
		}
		return nil, fmt.Errorf("get exercise logs set %s: %w", userId, err)
	}
	defer rows.Close()
	for rows.Next() {
		entry := &models.ExerciseSet{}
		err := rows.Scan(
			&entry.ID,
			&entry.LogId,
			&entry.Set_number,
			&entry.Weight,
			&entry.Reps,
			&entry.UpdatedAt,
			&entry.DeletedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("get exercise logs set scan %s: %w", userId, err)
		}

		logMap[entry.LogId] = append(logMap[entry.LogId], entry)
	}
	rows.Close()

	for _, full := range FullExerciselogs {
		full.ExerciseSets = logMap[full.ExerciseLog.ID]

	}

	// Check for errors that occurred during iteration
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("get exercise logs %s: %w", userId, err)
	}

	return FullExerciselogs, nil

}

// TODO: Rework sql to use a join (inner or left)
// Returns the full Exercise Log including sets
func (us *postgressUserRepo) GetUserExerciseLogById(ctx context.Context, tx *sql.Tx, userId, logId string) (*models.FullExerciseLog, error) {
	queryExerciseLog := `
    SELECT id, exercise_id, created_at, updated_at, deleted_at
    FROM exercise_logs
    WHERE user_id = $1 and id = $2
    ORDER BY created_at DESC;` // Good practice to sort by date

	log := &models.FullExerciseLog{}
	err := tx.QueryRowContext(ctx, queryExerciseLog, userId, logId).Scan(
		&log.ExerciseLog.ID,
		&log.ExerciseLog.ExerciseId,
		&log.ExerciseLog.CreatedAt,
		&log.ExerciseLog.UpdatedAt,
		&log.ExerciseLog.DeletedAt,
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return nil, fmt.Errorf("get exercise log %s for user %s: %w", logId, userId, ErrUserForeignKeyConstraint)
			}
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get exercise log %s for user %s: %w", logId, userId, ErrNotFound)
		}
		return nil, fmt.Errorf("get exercise log %s for user %s: %w", logId, userId, err)
	}

	log.ExerciseSets, err = us.getUserExerciseSetsByLogId(ctx, tx, logId)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return nil, fmt.Errorf("get exercise log %s: %w", logId, ErrLogIdForeignKeyConstraint)
			}
		}
		return nil, err
	}

	return log, nil

}

// TODO: Remove once the other exercise funtions are reworked
func (us *postgressUserRepo) upsertUserExerciseSets(ctx context.Context, tx *sql.Tx, payload []*models.ExerciseSet) error {
	if payload == nil {
		return errors.New("upsert exercise sets: nil payload")
	}

	query := `
INSERT INTO exercise_sets (id, log_id, set_number, weight, reps, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) 
DO UPDATE SET
    set_number = EXCLUDED.set_number,
    reps = EXCLUDED.reps,
    updated_at = EXCLUDED.updated_at,
	weight = EXCLUDED.weight,
    deleted_at = EXCLUDED.deleted_at
WHERE EXCLUDED.updated_at > exercise_sets.updated_at;`

	for _, set := range payload {

		_, err := tx.ExecContext(ctx, query,
			set.ID,         // $1
			set.LogId,      // $2
			set.Set_number, // $3
			set.Weight,     // $4
			set.Reps,       //$5
			set.UpdatedAt,  // $6
			set.DeletedAt,  // $7
		)
		if err != nil {
			return fmt.Errorf("upsert exercise set %s: %w", set.ID, err)
		}
	}

	return nil

}

// TODO: Remove once the other exercise funtions are reworked
func (us *postgressUserRepo) getUserExerciseSetsByLogId(ctx context.Context, tx *sql.Tx, logId string) ([]*models.ExerciseSet, error) {
	queryExerciseSet := `
    SELECT id, log_id, set_number, weight, reps, updated_at, deleted_at
    FROM exercise_sets WHERE log_id = $1
`
	var sets []*models.ExerciseSet
	rows, err := tx.QueryContext(ctx, queryExerciseSet, logId)
	if err != nil {
		return nil, fmt.Errorf("get exercise set by log %s: %w", logId, err)
	}
	defer rows.Close()
	for rows.Next() {
		set := &models.ExerciseSet{}
		scanErr := rows.Scan(
			&set.ID,
			&set.LogId,
			&set.Set_number,
			&set.Weight,
			&set.Reps,
			&set.UpdatedAt,
			&set.DeletedAt,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("get exercise set by log %s scan: %w", logId, scanErr)
		}
		sets = append(sets, set)
	}
	rowErr := rows.Err()
	if rowErr != nil {
		return nil, fmt.Errorf("get exercise set by log %s: %w", logId, rowErr)
	}

	return sets, nil

}

// TODO: same [postgressUserRepo.UpsertUserExerciseLog] combine the sql into one statement that does both the log and sets in one
func (us *postgressUserRepo) UpsertUserFoodLog(ctx context.Context, tx *sql.Tx, userId string, payload *models.FullFoodLog) error {
	query := `
INSERT INTO food_logs (id, user_id, time, name, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) 
DO UPDATE SET
    name = EXCLUDED.name,
    time = EXCLUDED.time,
    updated_at = EXCLUDED.updated_at,
    deleted_at = EXCLUDED.deleted_at
WHERE EXCLUDED.updated_at > food_logs.updated_at;`

	res, err := tx.ExecContext(ctx, query,
		payload.FoodLog.ID,        // $1
		userId,                    // $2
		payload.FoodLog.Time,      // $3
		payload.FoodLog.Name,      // $4
		payload.FoodLog.CreatedAt, // $5
		payload.FoodLog.UpdatedAt, // $6
		payload.FoodLog.DeletedAt, // $7
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return fmt.Errorf("get food log %s for user %s: %w", payload.FoodLog.ID, userId, ErrUserForeignKeyConstraint)
			}
		}
		return fmt.Errorf("upsert food log %s for user %s: %w", payload.FoodLog.ID, userId, err)
	}
	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		return fmt.Errorf("upsert food log %s: %w", userId, ErrNoRowsAffected)
	}
	err = us.upsertUserFoodLogEntry(ctx, tx, payload.FoodLogEntries)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return fmt.Errorf("get food log %s for user %s: %w", payload.FoodLog.ID, userId, ErrLogIdForeignKeyConstraint)
			}
		}
		return err
	}
	return nil

}

// TODO: Rework sql to use a join (inner or left)
func (us *postgressUserRepo) GetUserFoodLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*models.FullFoodLog, error) {
	queryFoodLog := `
    SELECT id, time, name, created_at, updated_at, deleted_at
    FROM food_logs
    WHERE user_id = $1
    ORDER BY created_at DESC;` // Good practice to sort by date
	queryFoodLogEntries := `
    SELECT id, log_id, food_id, food_name, serving_id, quantity, unit, cal, fat, carbs, protein, updated_at, deleted_at
    FROM food_log_items WHERE log_id = ANY($1);
`

	rows, err := tx.QueryContext(ctx, queryFoodLog, userId)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return nil, fmt.Errorf("get food logs for user %s: %w", userId, ErrUserForeignKeyConstraint)
			}
		}
		return nil, fmt.Errorf("get food logs %s: %w", userId, err)
	}

	var FullFoodLogs []*models.FullFoodLog
	defer rows.Close()
	for rows.Next() {
		log := &models.FoodLog{}
		err := rows.Scan(
			&log.ID,
			&log.Time,
			&log.Name,
			&log.CreatedAt,
			&log.UpdatedAt,
			&log.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("get food logs scan %s: %w", userId, err)
		}
		fullLog := models.FullFoodLog{FoodLog: log, FoodLogEntries: nil}
		FullFoodLogs = append(FullFoodLogs, &fullLog)

	}
	// Check for errors that occurred during iteration
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("get food logs %s: %w", userId, err)
	}
	rows.Close()

	logIds := []string{}

	logMap := map[string][]*models.FoodLogEntry{}

	for _, log := range FullFoodLogs {
		logIds = append(logIds, log.FoodLog.ID)
		logMap[log.FoodLog.ID] = []*models.FoodLogEntry{}
	}

	rows, err = tx.QueryContext(ctx, queryFoodLogEntries, logIds)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return nil, fmt.Errorf("get food logs for user %s: %w", userId, ErrLogIdForeignKeyConstraint)
			}
		}
		return nil, fmt.Errorf("get food logs entries %s : %w", userId, err)
	}
	defer rows.Close()
	for rows.Next() {
		entry := &models.FoodLogEntry{}
		err := rows.Scan(
			&entry.ID,
			&entry.LogId,
			&entry.FoodId,
			&entry.FoodName,
			&entry.ServingId,
			&entry.Quantity,
			&entry.Unit,
			&entry.Cal,
			&entry.Fat,
			&entry.Carbs,
			&entry.Protein,
			&entry.UpdatedAt,
			&entry.DeletedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("get food logs entries scan %s: %w", userId, err)
		}
		logMap[entry.LogId] = append(logMap[entry.LogId], entry)

	}
	rows.Close()
	for _, full := range FullFoodLogs {
		full.FoodLogEntries = logMap[full.FoodLog.ID]
	}

	// Check for errors that occurred during iteration
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("get food logs %s: %w", userId, err)
	}

	return FullFoodLogs, nil

}

// TODO: Rework sql to use a join (inner or left)
func (us *postgressUserRepo) GetUserFoodLogById(ctx context.Context, tx *sql.Tx, userId, logId string) (*models.FullFoodLog, error) {
	queryFoodLog := `
    SELECT id, time, name, created_at, updated_at, deleted_at
    FROM food_logs
    WHERE user_id = $1 AND id = $2;`

	log := &models.FullFoodLog{}
	err := tx.QueryRowContext(ctx, queryFoodLog, userId, logId).Scan(
		&log.FoodLog.ID,
		&log.FoodLog.Time,
		&log.FoodLog.Name,
		&log.FoodLog.CreatedAt,
		&log.FoodLog.UpdatedAt,
		&log.FoodLog.DeletedAt,
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return nil, fmt.Errorf("get food log %s for user %s: %w", logId, userId, ErrUserForeignKeyConstraint)
			}
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get food log %s for user %s: %w", logId, userId, ErrNotFound)
		}
		return nil, fmt.Errorf("get food log %s for user %s: %w", logId, userId, err)
	}

	log.FoodLogEntries, err = us.getUserFoodLogEntriesByLogId(ctx, tx, logId)
	if err != nil {
		return nil, err
	}

	return log, nil

}

// TODO: Remove once the other exercise funtions are reworked
func (us *postgressUserRepo) upsertUserFoodLogEntry(ctx context.Context, tx *sql.Tx, payload []*models.FoodLogEntry) error {
	if payload == nil {
		return errors.New("upsert food log entry: nil payload")
	}

	query := `
INSERT INTO food_log_items (id, log_id, food_id, food_name, serving_id, quantity, unit, cal, fat, carbs, protein, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (id) 
DO UPDATE SET
    food_name = EXCLUDED.food_name,
    serving_id = EXCLUDED.serving_id,
    quantity = EXCLUDED.quantity,
    unit = EXCLUDED.unit,
    cal = EXCLUDED.cal,
    fat = EXCLUDED.fat,
    carbs = EXCLUDED.carbs,
    protein = EXCLUDED.protein,
    updated_at = EXCLUDED.updated_at,
    deleted_at = EXCLUDED.deleted_at
WHERE EXCLUDED.updated_at > food_log_items.updated_at;`

	for _, entry := range payload {

		_, err := tx.ExecContext(ctx, query,
			entry.ID,        // $1
			entry.LogId,     // $2
			entry.FoodId,    // $3
			entry.FoodName,  // $4
			entry.ServingId, //$5
			entry.Quantity,  // $6
			entry.Unit,      // $7
			entry.Cal,       // $8,
			entry.Fat,       // $9,
			entry.Carbs,     // $10,
			entry.Protein,   // $ 11
			entry.UpdatedAt, // $12
			entry.DeletedAt, // $13
		)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok {
				if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
					return fmt.Errorf("get food logs for user %s: %w", entry.LogId, ErrLogIdForeignKeyConstraint)
				}
			}
			return fmt.Errorf("upsert food log entry %s: %w", entry.ID, err)
		}
	}

	return nil

}

// TODO: Remove once the other exercise funtions are reworked

func (us *postgressUserRepo) getUserFoodLogEntriesByLogId(ctx context.Context, tx *sql.Tx, logId string) ([]*models.FoodLogEntry, error) {
	queryFoodLogEntries := `
    SELECT id, log_id, food_id, food_name, serving_id, quantity, unit, cal, fat, carbs, protein, updated_at, deleted_at
    FROM food_log_items WHERE log_id = $1
`
	var entries []*models.FoodLogEntry
	rows, err := tx.QueryContext(ctx, queryFoodLogEntries, logId)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pq.ErrorCode(foreignKeyConstraintErrorCode) {
				return nil, fmt.Errorf("get food log entries for log %s: %w", logId, ErrLogIdForeignKeyConstraint)
			}
		}
		return nil, fmt.Errorf("get food log entry for log %s: %w", logId, err)
	}
	defer rows.Close()
	for rows.Next() {
		entry := &models.FoodLogEntry{}
		scanErr := rows.Scan(
			&entry.ID,
			&entry.LogId,
			&entry.FoodId,
			&entry.FoodName,
			&entry.ServingId,
			&entry.Quantity,
			&entry.Unit,
			&entry.Cal,
			&entry.Fat,
			&entry.Carbs,
			&entry.Protein,
			&entry.UpdatedAt,
			&entry.DeletedAt,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("get food log entry scan for log %s: %w", logId, scanErr)
		}
		entries = append(entries, entry)
	}
	rowErr := rows.Err()
	if rowErr != nil {
		return nil, fmt.Errorf("get food log entry for log %s: %w", logId, rowErr)
	}

	return entries, nil

}

func (us *postgressUserRepo) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := us.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("transaction begin: %w", err)
	}

	// If the function returns an error, we rollback
	// If it succeeds, we commit
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("transaction commit: %w", err)
	}
	return nil
}
