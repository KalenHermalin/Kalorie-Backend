package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"main/internal/models"
)

type postgressAuthRepo struct {
	db *sql.DB
}

func NewPostgressAuthStore(db *sql.DB) *postgressAuthRepo {
	return &postgressAuthRepo{
		db: db,
	}

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

func (p *postgressAuthRepo) LookupFirebaseIdToUserId(ctx context.Context, firebaseID string) (string, error) {
	selectQuery := `SELECT id FROM users where firebase_id = $1`
	var id string
	err := p.db.QueryRowContext(ctx, selectQuery, firebaseID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return id, nil
}
func (p *postgressAuthRepo) LookupUserIdToFirebase(ctx context.Context, userId string) (string, error) {
	selectQuery := `SELECT firebase_id FROM users where id = $1`
	var id string
	err := p.db.QueryRowContext(ctx, selectQuery, userId).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return id, nil
}

// UpsertUserWithAuth implements [models.AuthRepository].
func (p *postgressAuthRepo) UpsertUserWithAuth(ctx context.Context, tx *sql.Tx, payload *models.AuthPayload) (*models.User, error) {
	var user models.User

	// 1. Upsert User (Update last_login if you have that column)
	query := `INSERT INTO users (email, firebase_id) VALUES ($1, $2) 
                  ON CONFLICT (firebase_id) DO UPDATE SET email = EXCLUDED.email 
                  RETURNING id, email, created_at`
	if err := tx.QueryRowContext(ctx, query, payload.Email, payload.ID).Scan(&user.ID, &user.Email, &user.CreatedAt); err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
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
