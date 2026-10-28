package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"main/internal/models"
	"time"
)

type postgressAuthRepo struct {
	db *sql.DB
}

func NewPostgressAuthStore(db *sql.DB) *postgressAuthRepo {
	return &postgressAuthRepo{
		db: db,
	}

}

// DeleteRefreshToken implements [models.AuthRepository].
func (p *postgressAuthRepo) DeleteRefreshToken(ctx context.Context, tx *sql.Tx, token string, userId string) error {
	query := `DELETE FROM refresh_tokens WHERE user_id=$1 AND token=$2`
	result, err := tx.ExecContext(ctx, query, userId, token)
	if err != nil {
		return fmt.Errorf("delete refresh token %s: %w", userId, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete refresh token %s: %w", userId, err)
	}

	if rows == 0 {
		return fmt.Errorf("delete refresh token %s: %w", userId, ErrNoRowsAffected)

	}
	return nil
}

// DeleteUser implements [models.AuthRepository].
func (p *postgressAuthRepo) DeleteUser(ctx context.Context, tx *sql.Tx, userID string) error {
	deleteQuery := `DELETE FROM users WHERE id = $1`
	res, err := tx.ExecContext(ctx, deleteQuery, userID)
	if err != nil {
		return fmt.Errorf("delete user %s: %w", userID, err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("delete user %s: %w", userID, ErrNoRowsAffected)
	}
	return nil
}

// FindRefreshToken implements [models.AuthRepository].
func (p *postgressAuthRepo) FindRefreshToken(ctx context.Context, tx *sql.Tx, refresh string) (*models.User, error) {
	query := `
        SELECT u.id, u.email, u.created_at
        FROM users u
        JOIN refresh_tokens rt ON u.id = rt.user_id
        WHERE rt.token = $1 AND rt.expires_at > NOW()`

	user := &models.User{}
	err := tx.QueryRowContext(ctx, query, refresh).Scan(&user.ID, &user.Email, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("find refresh token: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	return user, nil
}

// SaveRefreshToken implements [models.AuthRepository].
func (p *postgressAuthRepo) SaveRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId string, expiresAt time.Time) error {
	query := `INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`
	_, err := tx.ExecContext(ctx, query, userId, refresh, expiresAt)
	if err != nil {
		return fmt.Errorf("save refresh token %s: %w", userId, err)
	}
	return nil

}

// UpsertUserWithAuth implements [models.AuthRepository].
func (p *postgressAuthRepo) UpsertUserWithAuth(ctx context.Context, tx *sql.Tx, payload *models.AuthPayload) (*models.User, error) {
	var user models.User

	// 1. Upsert User (Update last_login if you have that column)
	query := `INSERT INTO users (email) VALUES ($1) 
                  ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email 
                  RETURNING id, email, created_at`
	if err := tx.QueryRowContext(ctx, query, payload.Email).Scan(&user.ID, &user.Email, &user.CreatedAt); err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	// 2. Upsert Provider Identity
	providerQuery := `INSERT INTO provider_identities (user_id, provider, provider_user_id) 
                          VALUES ($1, $2, $3) 
                          ON CONFLICT (provider, provider_user_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, providerQuery, user.ID, payload.Provider, payload.ID); err != nil {
		return nil, fmt.Errorf("upsert user provider identity: %w", err)
	}
	return &user, nil
}

// WithTx implements [models.AuthRepository].
func (p *postgressAuthRepo) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := p.db.BeginTx(ctx, nil)
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
