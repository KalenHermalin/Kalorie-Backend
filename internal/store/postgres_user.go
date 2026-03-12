package store

import (
	"context"
	"database/sql"
	"errors"
	"main/internal/models"
	"time"
)

type postgressUserRepo struct {
	db *sql.DB
}

var ErrNoRowsAffected = errors.New("no rows affected")

func NewPostgressUserStore(db *sql.DB) *postgressUserRepo {
	return &postgressUserRepo{
		db: db,
	}

}

func (us *postgressUserRepo) SaveRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId string, expiresAt time.Time) error {
	query := `INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`
	_, err := tx.ExecContext(ctx, query, userId, refresh, expiresAt)
	return err

}
func (us *postgressUserRepo) FindRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId string) (*models.User, error) {
	query := `
        SELECT u.id, u.email 
        FROM users u
        JOIN refresh_tokens rt ON u.id = rt.user_id
        WHERE rt.token = $1 AND rt.expires_at > NOW()`

	user := &models.User{}
	err := tx.QueryRowContext(ctx, query, refresh).Scan(&user.ID, &user.Email)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (us *postgressUserRepo) DeleteRefreshToken(ctx context.Context, tx *sql.Tx, token string, userId string) error {
	query := `DELETE FROM refresh_tokens WHERE user_id=$1 AND token=$2`
	result, err := tx.ExecContext(ctx, query, userId, token)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errors.New("Could not delete token")
	}
	if rows == 1 {
		return nil
	}
	return errors.New("Unknown Error Occured")

}
func (us *postgressUserRepo) UpsertUserWithAuth(ctx context.Context, tx *sql.Tx, payload *models.AuthPayload) (*models.User, error) {
	var user models.User

	// 1. Upsert User (Update last_login if you have that column)
	query := `INSERT INTO users (email) VALUES ($1) 
                  ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email 
                  RETURNING id, email, created_at`
	if err := tx.QueryRowContext(ctx, query, payload.Email).Scan(&user.ID, &user.Email, &user.CreatedAt); err != nil {
		return nil, err
	}
	// 2. Upsert Provider Identity
	providerQuery := `INSERT INTO provider_identities (user_id, provider, provider_user_id) 
                          VALUES ($1, $2, $3) 
                          ON CONFLICT (provider, provider_user_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, providerQuery, user.ID, payload.Provider, payload.ID); err != nil {
		return nil, err
	}
	return &user, nil
}

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

	_, err := tx.ExecContext(ctx, query,
		payload.ID,        // $1
		userId,            // $2
		payload.WeightKg,  // $3
		payload.LogDate,   // $4
		payload.UpdatedAt, // $5
		payload.DeletedAt, // $6
	)
	if err != nil {
		return err
	}

	return nil

}

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
		if err == sql.ErrNoRows {
			return nil, ErrNoRowsAffected
		}
		return nil, err
	}

	return log, nil
}

func (us *postgressUserRepo) GetUserWeightLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*models.WeightLog, error) {
	query := `
    SELECT id, weight_kg, log_date, updated_at, deleted_at
    FROM weight_logs
    WHERE user_id = $1
    ORDER BY log_date DESC;` // Good practice to sort by date

	rows, err := tx.QueryContext(ctx, query, userId)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		logs = append(logs, log)
	}

	// Check for errors that occurred during iteration
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}

func (us *postgressUserRepo) UpdateUserSettings(ctx context.Context, tx *sql.Tx, userId string, settings *models.UserSettings) error {

	// 1. Upsert User (Update last_login if you have that column)
	query := `UPDATE user_settings 
          SET units = $2, calories_target = $3, protein_target = $4, carbs_target = $5, fat_target = $6, updated_at = $7, deleted_at = $8, theme = $9 
          WHERE user_id = $1 AND $7 > updated_at`
	res, err := tx.ExecContext(ctx, query, userId, settings.Units, settings.CaloriesTarget, settings.ProteinTarget, settings.CarbsTarget, settings.FatTarget, settings.UpdatedAt, settings.DeletedAt, settings.Theme)
	if err != nil {
		return err
	}

	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		return ErrNoRowsAffected
	}
	return nil

}
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRowsAffected
		}
		return nil, err
	}
	return settings, nil

}

func (us *postgressUserRepo) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := us.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// If the function returns an error, we rollback
	// If it succeeds, we commit
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}
