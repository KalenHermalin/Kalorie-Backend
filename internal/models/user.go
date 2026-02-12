package models

import (
	"context"
	"database/sql"
	"time"
)

type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Provider struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	UserID uint32 `json:"user_id"`
}

type UserRepository interface {
	UpsertUserWithAuth(ctx context.Context, tx *sql.Tx, payload *AuthPayload) (*User, error)
	DeleteRefreshToken(ctx context.Context, tx *sql.Tx, token string, userId int) error
	SaveRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId int, expiresAt time.Time) error
	FindRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId int) (*User, error)
	WithTx(ctx context.Context, fn func(*sql.Tx) error) error
}
