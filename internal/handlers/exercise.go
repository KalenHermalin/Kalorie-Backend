package handlers

import (
	"encoding/json"
	"main/internal/apperrors"
	"main/internal/service"
	"net/http"
	"strconv"
)

type ExerciseHandler struct {
	es service.ExerciseService
}

func NewExerciseHandler(exerciseService service.ExerciseService) *ExerciseHandler {
	return &ExerciseHandler{es: exerciseService}
}

func (eh *ExerciseHandler) SearchExercisesWithFuzzy(writer http.ResponseWriter, request *http.Request) error {
	query := request.URL.Query().Get("q")
	limitStr := request.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			// Return a 400 error if they passed something like ?limit=abc
			return apperrors.NewBadRequestWithFields(err, apperrors.FieldError{Field: "limit", Problem: "invalid limit parameter"})
		}
		if parsedLimit != 0 {
			limit = parsedLimit
		}
	}

	exercises, err := eh.es.SearchExercises(request.Context(), query, limit)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(exercises)
	return nil
}
