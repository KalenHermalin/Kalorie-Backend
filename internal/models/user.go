package models

import (
	"context"
	"database/sql"
	"time"
)

// TODO: Change User.ID to string for UUID, update all methods accordinly
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type UserSettings struct {
	Units          string     `json:"units"`
	CaloriesTarget int        `json:"calories_target"`
	ProteinTarget  int        `json:"protein_target"`
	CarbsTarget    int        `json:"carbs_target"`
	FatTarget      int        `json:"fat_target"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at"`
	Theme          string     `json:"theme"`
}

type WeightLog struct {
	ID        string       `json:"id"`
	WeightKg  float64      `json:"weight_kg"`
	LogDate   time.Time    `json:"log_date"`
	UpdatedAt time.Time    `json:"updated_at"`
	DeletedAt sql.NullTime `json:"deleted_at"`
}

type Provider struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	UserID uint32 `json:"user_id"`
}

type UserRepository interface {
	UpsertUserWithAuth(ctx context.Context, tx *sql.Tx, payload *AuthPayload) (*User, error)
	DeleteRefreshToken(ctx context.Context, tx *sql.Tx, token string, userId string) error
	SaveRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId string, expiresAt time.Time) error
	FindRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId string) (*User, error)

	UpdateUserSettings(ctx context.Context, tx *sql.Tx, userId string, settings *UserSettings) error
	GetUserSettings(ctx context.Context, tx *sql.Tx, userId string) (*UserSettings, error)

	UpsertUserWeightLog(ctx context.Context, tx *sql.Tx, userId string, payload *WeightLog) error
	GetUserWeightLogById(ctx context.Context, tx *sql.Tx, userId string, logId string) (*WeightLog, error)
	WithTx(ctx context.Context, fn func(*sql.Tx) error) error
}
