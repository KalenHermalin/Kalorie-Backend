package models

import (
	"context"
	"database/sql"
	"time"
)

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
	ID        string     `json:"id"`
	WeightKg  float64    `json:"weight_kg"`
	LogDate   time.Time  `json:"log_date"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}
type ExerciseLog struct {
	ID         string     `json:"id"`
	ExerciseId string     `json:"exercise_id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at"`
}
type ExerciseSet struct {
	ID         string     `json:"id"`
	LogId      string     `json:"log_id"`
	Set_number int        `json:"set_number"`
	Weight     float32    `json:"weight"`
	Reps       int        `json:"reps"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at"`
}
type FullExerciseLog struct {
	ExerciseLog  *ExerciseLog   `json:"exercise_log"`
	ExerciseSets []*ExerciseSet `json:"exercise_sets"`
}

type FoodLog struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Time      string     `json:"Time"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}
type FoodLogEntry struct {
	ID        string     `json:"id"`
	LogId     string     `json:"log_id"`
	FoodId    string     `json:"food_id"`
	FoodName  string     `json:"food_name"`
	ServingId string     `json:"serving_id"`
	Unit      string     `json:"unit"`
	Cal       int        `json:"cal"`
	Fat       float32    `json:"fat"`
	Carbs     float32    `json:"carbs"`
	Protein   float32    `json:"protein"`
	Quantity  float32    `json:"quantity"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

type FullFoodLog struct {
	FoodLog        *FoodLog        `json:"food_log"`
	FoodLogEntries []*FoodLogEntry `json:"food-log_entries"`
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
	GetUserWeightLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*WeightLog, error)

	UpsertUserExerciseSets(ctx context.Context, tx *sql.Tx, payload []*ExerciseSet) error
	GetUserExerciseLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*FullExerciseLog, error)
	UpsertUserExerciseLog(ctx context.Context, tx *sql.Tx, userId string, payload *ExerciseLog) error
	GetUserExerciseLogById(ctx context.Context, tx *sql.Tx, userId, logId string) (*ExerciseLog, error)
	GetUserExerciseSetsByLogId(ctx context.Context, tx *sql.Tx, logId string) ([]*ExerciseSet, error)

	UpsertUserFoodLog(ctx context.Context, tx *sql.Tx, userId string, payload *FoodLog) error
	GetUserFoodLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*FullFoodLog, error)
	GetUserFoodLogById(ctx context.Context, tx *sql.Tx, userId, logId string) (*FoodLog, error)
	UpsertUserFoodLogEntry(ctx context.Context, tx *sql.Tx, payload []*FoodLogEntry) error
	GetUserFoodLogEntriesByLogId(ctx context.Context, tx *sql.Tx, logId string) ([]*FoodLogEntry, error)
	WithTx(ctx context.Context, fn func(*sql.Tx) error) error
}
