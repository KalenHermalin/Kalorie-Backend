package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"main/internal/apperrors"
	"main/internal/middlewares"
	"main/internal/models"
	"main/internal/service"
	"main/internal/utils"
	"net/http"
	"time"
)

type UserHandler struct {
	user service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{user: userService}
}

type esyncSettingsbody struct {
	Settings models.UserSettings `json:"settings"`
}

func (uh *UserHandler) EgressSyncSettings(writer http.ResponseWriter, request *http.Request) {

	id, ok := request.Context().Value(middlewares.UserIDKey).(string)
	if !ok {
		apperrors.WriteError(writer, *apperrors.ErrMissingUserIdInContext)
	}
	var requestBody esyncSettingsbody
	err := utils.DecodePayload(request.Body, &requestBody)
	if err != nil {
		slog.Error("Error: decoding refresh request body", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrBadRequestBody)
		return

	}

	newSettings, err := uh.user.UpdateUserSettings(request.Context(), id, &requestBody.Settings)
	if err != nil {
		if errors.Is(err, apperrors.ErrSyncConflict) {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusConflict) // 409
			json.NewEncoder(writer).Encode(newSettings)
			return // Critical: Stop execution here
		}

		// 2. Handle standard AppErrors
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			apperrors.WriteError(writer, *appErr)
			return // Critical: Stop execution here
		}

		// 3. Fallback for unexpected errors
		slog.Error("Error: Updating Settings Failed", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInternalServer)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(map[string]string{"status": "success"})
}
func (uh *UserHandler) IngressSyncSettings(writer http.ResponseWriter, request *http.Request) {

	id, ok := request.Context().Value(middlewares.UserIDKey).(string)
	if !ok {
		apperrors.WriteError(writer, *apperrors.ErrMissingUserIdInContext)
	}
	last_synced := request.URL.Query().Get("last_synced_at")
	var sinceTime *time.Time
	if last_synced != "" {
		t, err := time.Parse(time.RFC3339Nano, last_synced) // Matches your Postgres precision
		if err == nil {
			sinceTime = &t
		}

	}
	if sinceTime == nil {
		apperrors.WriteError(writer, *apperrors.NewAppError("ERR_MISSING_QUERY", "Missing the 'last_synced_at' query", http.StatusBadRequest))
		return
	}
	newSettings, err := uh.user.GetUserSettings(request.Context(), id, sinceTime)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			apperrors.WriteError(writer, *appErr)
			return // Critical: Stop execution here
		}

		// 3. Fallback for unexpected errors
		slog.Error("Error: Updating Settings Failed", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInternalServer)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(newSettings)
}

type SyncWeightLogsRequestBody struct {
	WeightLogs []*models.WeightLog `json:"weight_logs"`
}
type SyncWeightLogsResponse struct {
	NewLogs    []*models.WeightLog `json:"new_logs"`
	FailedLogs []*models.WeightLog `json:"failed_logs"`
	Err        error               `json:"error"`
}

func (uh *UserHandler) EgressSyncWeightLogs(writer http.ResponseWriter, request *http.Request) {
	id, ok := request.Context().Value(middlewares.UserIDKey).(string)
	if !ok {
		apperrors.WriteError(writer, *apperrors.ErrMissingUserIdInContext)
	}
	var requestBody SyncWeightLogsRequestBody
	err := utils.DecodePayload(request.Body, &requestBody)
	if err != nil {
		slog.Error("Error: decoding refresh request body", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrBadRequestBody)
		return

	}

	successful, failed, err := uh.user.UpsertUserWeightLogs(request.Context(), id, requestBody.WeightLogs)
	if err != nil {

		if errors.Is(err, apperrors.ErrSoftWeightLog) {
			writer.Header().Set("Content-Type", "application/json")
			response := &SyncWeightLogsResponse{
				NewLogs:    successful,
				FailedLogs: failed,
				Err:        err,
			}
			json.NewEncoder(writer).Encode(response)
			return
		}
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			apperrors.WriteError(writer, *appErr)
			return
		}
		apperrors.WriteError(writer, *apperrors.ErrInternalServer)
		return

	}
	response := &SyncWeightLogsResponse{
		NewLogs:    successful,
		FailedLogs: nil,
		Err:        nil,
	}
	json.NewEncoder(writer).Encode(response)
}
func (uh *UserHandler) IngressSyncWeightLogs(writer http.ResponseWriter, request *http.Request) {

	id, ok := request.Context().Value(middlewares.UserIDKey).(string)
	if !ok {
		apperrors.WriteError(writer, *apperrors.ErrMissingUserIdInContext)
	}
	last_synced := request.URL.Query().Get("last_synced_at")
	var sinceTime *time.Time
	if last_synced != "" {
		t, err := time.Parse(time.RFC3339Nano, last_synced) // Matches your Postgres precision
		if err == nil {
			sinceTime = &t
		}

	}
	if sinceTime == nil {
		apperrors.WriteError(writer, *apperrors.NewAppError("ERR_MISSING_QUERY", "Missing the 'last_synced_at' query", http.StatusBadRequest))
		return
	}
	weightLogs, err := uh.user.GetUserWeightLogs(request.Context(), id, sinceTime)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			apperrors.WriteError(writer, *appErr)
			return // Critical: Stop execution here
		}

		// 3. Fallback for unexpected errors
		slog.Error("Error: Updating Settings Failed", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInternalServer)
		return
	}
	if len(weightLogs) == 0 {
		writer.WriteHeader(http.StatusNotModified)
		return

	}
	if weightLogs == nil {
		writer.WriteHeader(http.StatusNotModified)
		return

	}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(weightLogs)
}

type SyncExerciseLogsRequestBody struct {
	ExerciseLogs []*models.FullExerciseLog `json:"exercise_logs"`
}
type SyncExerciseLogsResponse struct {
	NewLogs    []*models.FullExerciseLog `json:"new_logs"`
	FailedLogs []*models.FullExerciseLog `json:"failed_logs"`
	Err        error                     `json:"error"`
}

func (uh *UserHandler) EgressSyncExerciseLogs(writer http.ResponseWriter, request *http.Request) {
	id, ok := request.Context().Value(middlewares.UserIDKey).(string)
	if !ok {
		apperrors.WriteError(writer, *apperrors.ErrMissingUserIdInContext)
	}
	var requestBody SyncExerciseLogsRequestBody
	err := utils.DecodePayload(request.Body, &requestBody)
	if err != nil {
		slog.Error("Error: decoding refresh request body", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrBadRequestBody)
		return

	}

	successful, failed, err := uh.user.UpsertUserExerciseLogAndSets(request.Context(), id, requestBody.ExerciseLogs)
	if err != nil {

		if errors.Is(err, apperrors.ErrSoftWeightLog) {
			writer.Header().Set("Content-Type", "application/json")
			response := &SyncExerciseLogsResponse{
				NewLogs:    successful,
				FailedLogs: failed,
				Err:        err,
			}
			json.NewEncoder(writer).Encode(response)
			return
		}
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			apperrors.WriteError(writer, *appErr)
			return
		}
		apperrors.WriteError(writer, *apperrors.ErrInternalServer)
		return

	}
	response := &SyncExerciseLogsResponse{
		NewLogs:    successful,
		FailedLogs: nil,
		Err:        nil,
	}
	json.NewEncoder(writer).Encode(response)
}

func (uh *UserHandler) IngressSyncExerciseLogs(writer http.ResponseWriter, request *http.Request) {

	id, ok := request.Context().Value(middlewares.UserIDKey).(string)
	if !ok {
		apperrors.WriteError(writer, *apperrors.ErrMissingUserIdInContext)
	}
	last_synced := request.URL.Query().Get("last_synced_at")
	var sinceTime *time.Time
	if last_synced != "" {
		t, err := time.Parse(time.RFC3339Nano, last_synced) // Matches your Postgres precision
		if err == nil {
			sinceTime = &t
		}

	}
	if sinceTime == nil {
		apperrors.WriteError(writer, *apperrors.NewAppError("ERR_MISSING_QUERY", "Missing the 'last_synced_at' query", http.StatusBadRequest))
		return
	}
	newLogs, err := uh.user.GetUserExerciseLogs(request.Context(), id, sinceTime)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			apperrors.WriteError(writer, *appErr)
			return // Critical: Stop execution here
		}

		// 3. Fallback for unexpected errors
		slog.Error("Error: Updating Settings Failed", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInternalServer)
		return
	}
	if len(newLogs) == 0 {
		writer.WriteHeader(http.StatusNotModified)
		return

	}
	if newLogs == nil {
		writer.WriteHeader(http.StatusNotModified)
		return

	}

	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(newLogs)
}

type SyncFoodLogsRequestBody struct {
	FoodLogs []*models.FullFoodLog `json:"food_logs"`
}
type SyncFoodLogsResponse struct {
	NewLogs    []*models.FullFoodLog `json:"new_logs"`
	FailedLogs []*models.FullFoodLog `json:"failed_logs"`
	Err        error                 `json:"error"`
}

func (uh *UserHandler) EgressSyncFoodLogs(writer http.ResponseWriter, request *http.Request) {
	id, ok := request.Context().Value(middlewares.UserIDKey).(string)
	if !ok {
		apperrors.WriteError(writer, *apperrors.ErrMissingUserIdInContext)
	}
	var requestBody SyncFoodLogsRequestBody
	err := utils.DecodePayload(request.Body, &requestBody)
	if err != nil {
		slog.Error("Error: decoding refresh request body", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrBadRequestBody)
		return

	}

	successful, failed, err := uh.user.UpsertUserFoodLogsAndEntries(request.Context(), id, requestBody.FoodLogs)
	if err != nil {

		if errors.Is(err, apperrors.ErrSoftWeightLog) {
			writer.Header().Set("Content-Type", "application/json")
			response := &SyncFoodLogsResponse{
				NewLogs:    successful,
				FailedLogs: failed,
				Err:        err,
			}
			json.NewEncoder(writer).Encode(response)
			return
		}
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			apperrors.WriteError(writer, *appErr)
			return
		}
		apperrors.WriteError(writer, *apperrors.ErrInternalServer)
		return

	}
	response := &SyncFoodLogsResponse{
		NewLogs:    successful,
		FailedLogs: nil,
		Err:        nil,
	}
	json.NewEncoder(writer).Encode(response)
}

func (uh *UserHandler) IngressSyncFoodLogs(writer http.ResponseWriter, request *http.Request) {

	id, ok := request.Context().Value(middlewares.UserIDKey).(string)
	if !ok {
		apperrors.WriteError(writer, *apperrors.ErrMissingUserIdInContext)
	}
	last_synced := request.URL.Query().Get("last_synced_at")
	var sinceTime *time.Time
	if last_synced != "" {
		t, err := time.Parse(time.RFC3339Nano, last_synced) // Matches your Postgres precision
		if err == nil {
			sinceTime = &t
		}

	}
	if sinceTime == nil {
		apperrors.WriteError(writer, *apperrors.NewAppError("ERR_MISSING_QUERY", "Missing the 'last_synced_at' query", http.StatusBadRequest))
		return
	}
	newLogs, err := uh.user.GetUserFoodLogs(request.Context(), id, sinceTime)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			apperrors.WriteError(writer, *appErr)
			return // Critical: Stop execution here
		}

		// 3. Fallback for unexpected errors
		slog.Error("Error: Updating Settings Failed", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInternalServer)
		return
	}
	if len(newLogs) == 0 {
		writer.WriteHeader(http.StatusNotModified)
		return

	}
	if newLogs == nil {
		writer.WriteHeader(http.StatusNotModified)
		return

	}

	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(newLogs)
}
