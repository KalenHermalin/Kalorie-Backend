package models

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type User struct {
	ID        string    `json:"id"`
	Email     *string   `json:"email"`
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
	LogDate   string     `json:"log_date"` // Format YYYY-MM-DD
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

type WorkoutTemplate struct {
	ID        string                      `json:"id"`
	Name      string                      `json:"name"`
	Exercises WorkoutTemplateExerciseList `json:"exercises"`
	CreatedAt time.Time                   `json:"created_at"`
	UpdatedAt time.Time                   `json:"updated_at"`
	DeletedAt *time.Time                  `json:"deleted_at"`
}
type WorkoutTemplateExercises struct {
	ExerciseID string `json:"exercise_id"`
	Sets       uint8  `json:"sets"`
}

type WorkoutTemplateExerciseList []*WorkoutTemplateExercises

// Value runs when the list is passed as a query argument (Go → Postgres).
func (l WorkoutTemplateExerciseList) Value() (driver.Value, error) {
	if l == nil {
		l = WorkoutTemplateExerciseList{} // store [] instead of JSON null
	}
	b, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("marshal workout template exercises: %w", err)
	}
	return string(b), nil // string, not []byte: lib/pq would send []byte as bytea
}

// Scan runs when you Scan a column into the list (Postgres → Go).
func (l *WorkoutTemplateExerciseList) Scan(src any) error {
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	case nil:
		*l = WorkoutTemplateExerciseList{}
		return nil
	default:
		return fmt.Errorf("scan workout template exercises: unsupported type %T", src)
	}
	if err := json.Unmarshal(b, l); err != nil {
		return fmt.Errorf("unmarshal workout template exercises: %w", err)
	}
	if *l == nil { // the column held JSON null
		*l = WorkoutTemplateExerciseList{}
	}
	return nil
}

type UserRepository interface {
	UpdateUserSettings(ctx context.Context, tx *sql.Tx, userId string, settings *UserSettings) error
	GetUserSettings(ctx context.Context, tx *sql.Tx, userId string) (*UserSettings, error)

	UpsertUserWeightLog(ctx context.Context, tx *sql.Tx, userId string, payload *WeightLog) error
	GetUserWeightLogById(ctx context.Context, tx *sql.Tx, userId string, logId string) (*WeightLog, error)
	GetUserWeightLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*WeightLog, error)

	GetUserExerciseLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*FullExerciseLog, error)
	// Upserts the log item and its sets in a single transaction
	// rolls back on error
	UpsertUserExerciseLog(ctx context.Context, tx *sql.Tx, userId string, payload *FullExerciseLog) error
	// Returns a single exercise log with all sets
	GetUserExerciseLogById(ctx context.Context, tx *sql.Tx, userId, logId string) (*FullExerciseLog, error)

	UpsertUserFoodLog(ctx context.Context, tx *sql.Tx, userId string, payload *FullFoodLog) error
	GetUserFoodLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*FullFoodLog, error)
	GetUserFoodLogById(ctx context.Context, tx *sql.Tx, userId, logId string) (*FullFoodLog, error)

	UpsertUserWorkoutTemplate(ctx context.Context, tx *sql.Tx, userId string, payload *WorkoutTemplate) error
	GetUserWorkoutTemplateById(ctx context.Context, tx *sql.Tx, userId, templateId string) (*WorkoutTemplate, error)
	GetUserWorkoutTemplates(ctx context.Context, tx *sql.Tx, userId string) ([]*WorkoutTemplate, error)
	WithTx(ctx context.Context, fn func(*sql.Tx) error) error
}
