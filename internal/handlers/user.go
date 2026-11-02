package handlers

import (
	"encoding/json"
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
type egressResponse struct {
	NewSettings any  `json:"new"`
	IsConflcit  bool `json:"is_conflict"`
}

func (uh *UserHandler) EgressSyncSettings(writer http.ResponseWriter, request *http.Request) error {

	id := middlewares.GetUserID(request.Context())
	var requestBody esyncSettingsbody
	err := utils.DecodePayload(request.Body, &requestBody)
	if err != nil {
		return apperrors.New(apperrors.BadRequest, err)

	}

	newSettings, isModified, err := uh.user.UpdateUserSettings(request.Context(), id, &requestBody.Settings)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(egressResponse{
		NewSettings: newSettings,
		IsConflcit:  isModified,
	})
	return nil
}
func (uh *UserHandler) IngressSyncSettings(writer http.ResponseWriter, request *http.Request) error {

	id := middlewares.GetUserID(request.Context())

	last_synced := request.URL.Query().Get("last_synced_at")
	var sinceTime *time.Time
	if last_synced != "" {
		t, err := time.Parse(time.RFC3339Nano, last_synced) // Matches your Postgres precision
		if err == nil {
			sinceTime = &t
		}

	}

	newSettings, isModified, err := uh.user.GetUserSettings(request.Context(), id, sinceTime)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json")
	if !isModified {
		writer.WriteHeader(http.StatusNotModified)
		return nil
	}

	json.NewEncoder(writer).Encode(newSettings)
	return nil
}

type SyncWeightLogsRequestBody struct {
	WeightLogs []*models.WeightLog `json:"weight_logs"`
}
type SyncWeightLogsResponse struct {
	NewLogs    []*models.WeightLog `json:"new_logs"`
	FailedLogs []string            `json:"failed_logs"`
}

func (uh *UserHandler) EgressSyncWeightLogs(writer http.ResponseWriter, request *http.Request) error {
	id := middlewares.GetUserID(request.Context())

	var requestBody SyncWeightLogsRequestBody
	err := utils.DecodePayload(request.Body, &requestBody)
	if err != nil {
		return apperrors.New(apperrors.BadRequest, err)
	}

	successful, failed, err := uh.user.UpsertUserWeightLogs(request.Context(), id, requestBody.WeightLogs)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json")

	if len(failed) != 0 {
		writer.WriteHeader(http.StatusMultiStatus)
	}
	response := &SyncWeightLogsResponse{
		NewLogs:    successful,
		FailedLogs: failed,
	}

	json.NewEncoder(writer).Encode(response)
	return nil
}
func (uh *UserHandler) IngressSyncWeightLogs(writer http.ResponseWriter, request *http.Request) error {

	id := middlewares.GetUserID(request.Context())

	last_synced := request.URL.Query().Get("last_synced_at")
	var sinceTime *time.Time
	if last_synced != "" {
		t, err := time.Parse(time.RFC3339Nano, last_synced) // Matches your Postgres precision
		if err == nil {
			sinceTime = &t
		}

	}
	weightLogs, err := uh.user.GetUserWeightLogs(request.Context(), id, sinceTime)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json")
	if len(weightLogs) == 0 {
		writer.WriteHeader(http.StatusNotModified)
		return nil

	}

	json.NewEncoder(writer).Encode(weightLogs)
	return nil
}

type SyncExerciseLogsRequestBody struct {
	ExerciseLogs []*models.FullExerciseLog `json:"exercise_logs"`
}
type SyncExerciseLogsResponse struct {
	NewLogs    []*models.FullExerciseLog `json:"new_logs"`
	FailedLogs []string                  `json:"failed_logs"`
}

func (uh *UserHandler) EgressSyncExerciseLogs(writer http.ResponseWriter, request *http.Request) error {
	id := middlewares.GetUserID(request.Context())

	var requestBody SyncExerciseLogsRequestBody
	err := utils.DecodePayload(request.Body, &requestBody)
	if err != nil {
		return apperrors.New(apperrors.BadRequest, err)

	}

	successful, failed, err := uh.user.UpsertUserExerciseLogAndSets(request.Context(), id, requestBody.ExerciseLogs)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json")

	if len(failed) > 0 {
		writer.WriteHeader(http.StatusMultiStatus)
	}
	response := &SyncExerciseLogsResponse{
		NewLogs:    successful,
		FailedLogs: failed,
	}

	json.NewEncoder(writer).Encode(response)
	return nil
}

func (uh *UserHandler) IngressSyncExerciseLogs(writer http.ResponseWriter, request *http.Request) error {

	id := middlewares.GetUserID(request.Context())

	last_synced := request.URL.Query().Get("last_synced_at")
	var sinceTime *time.Time
	if last_synced != "" {
		t, err := time.Parse(time.RFC3339Nano, last_synced) // Matches your Postgres precision
		if err == nil {
			sinceTime = &t
		}

	}

	newLogs, err := uh.user.GetUserExerciseLogs(request.Context(), id, sinceTime)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json")
	if len(newLogs) == 0 {
		writer.WriteHeader(http.StatusNotModified)
		return nil

	}

	json.NewEncoder(writer).Encode(newLogs)
	return nil
}

type SyncFoodLogsRequestBody struct {
	FoodLogs []*models.FullFoodLog `json:"food_logs"`
}
type SyncFoodLogsResponse struct {
	NewLogs    []*models.FullFoodLog `json:"new_logs"`
	FailedLogs []string              `json:"failed_logs"`
}

func (uh *UserHandler) EgressSyncFoodLogs(writer http.ResponseWriter, request *http.Request) error {
	id := middlewares.GetUserID(request.Context())

	var requestBody SyncFoodLogsRequestBody
	err := utils.DecodePayload(request.Body, &requestBody)
	if err != nil {
		return apperrors.New(apperrors.BadRequest, err)

	}

	successful, failed, err := uh.user.UpsertUserFoodLogsAndEntries(request.Context(), id, requestBody.FoodLogs)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json")

	if len(failed) > 0 {
		writer.WriteHeader(http.StatusMultiStatus)
	}
	response := &SyncFoodLogsResponse{
		NewLogs:    successful,
		FailedLogs: failed,
	}

	json.NewEncoder(writer).Encode(response)
	return nil
}

func (uh *UserHandler) IngressSyncFoodLogs(writer http.ResponseWriter, request *http.Request) error {
	id := middlewares.GetUserID(request.Context())

	last_synced := request.URL.Query().Get("last_synced_at")
	var sinceTime *time.Time
	if last_synced != "" {
		t, err := time.Parse(time.RFC3339Nano, last_synced) // Matches your Postgres precision
		if err == nil {
			sinceTime = &t
		}

	}

	newLogs, err := uh.user.GetUserFoodLogs(request.Context(), id, sinceTime)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json")
	if len(newLogs) == 0 {
		writer.WriteHeader(http.StatusNotModified)
		return nil

	}
	json.NewEncoder(writer).Encode(newLogs)
	return nil
}
