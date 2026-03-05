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

func (us postgressUserRepo) SaveRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId int, expiresAt time.Time) error {
	query := `INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`
	_, err := tx.ExecContext(ctx, query, userId, refresh, expiresAt)
	return err

}
func (us postgressUserRepo) FindRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId int) (*models.User, error) {
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

func (us postgressUserRepo) DeleteRefreshToken(ctx context.Context, tx *sql.Tx, token string, userId int) error {
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
func (us postgressUserRepo) UpsertUserWithAuth(ctx context.Context, tx *sql.Tx, payload *models.AuthPayload) (*models.User, error) {
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

func NewPostgressUserStore(db *sql.DB) *postgressUserRepo {
	return &postgressUserRepo{
		db: db,
	}

}

func (us postgressUserRepo) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
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
