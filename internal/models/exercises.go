package models

import (
	"context"
	"time"
)

type ExerciseRepository interface {
	SearchExercises(ctx context.Context, searchTerm string, limit int) ([]*Exercise, error)
}

type Exercise struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Equipment        string    `json:"equipment"`
	PrimaryMuscle    string    `json:"primary_muscle"`
	SecondaryMuscles []string  `json:"secondary_muscles"`
	Steps            []string  `json:"steps"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
