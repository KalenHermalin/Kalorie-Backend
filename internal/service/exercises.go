package service

import (
	"context"
	"main/internal/apperrors"
	"main/internal/models"
	"strings"
	"unicode/utf8"
)

type ExerciseService struct {
	exerciseStore models.ExerciseRepository
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func NewExerciseService(store models.ExerciseRepository) *ExerciseService {
	return &ExerciseService{exerciseStore: store}
}

func (es *ExerciseService) SearchExercises(ctx context.Context, searchTerm string, limit int) ([]*models.Exercise, error) {
	searchTerm = strings.TrimSpace(searchTerm)
	if utf8.RuneCountInString(searchTerm) < 2 {
		return nil, apperrors.NewBadRequestWithFields(nil, apperrors.FieldError{Field: "q", Problem: "Query is too short. Must be 2 or more characters"})
	}
	if utf8.RuneCountInString(searchTerm) > 50 {
		return nil, apperrors.NewBadRequestWithFields(nil, apperrors.FieldError{Field: "q", Problem: "Query is too long. Max 50 characters"})
	}
	if limit < 0 {
		return nil, apperrors.NewBadRequestWithFields(nil, apperrors.FieldError{Field: "limit", Problem: "limit must be a positive integer"})
	}
	if limit > 50 {
		limit = 50
	}
	escapedSearchTerm := likeEscaper.Replace(searchTerm)
	lowerSearchTerm := strings.ToLower(escapedSearchTerm)
	results, err := es.exerciseStore.SearchExercises(ctx, lowerSearchTerm, limit)
	if err != nil {
		return nil, err
	}
	return results, nil
}
