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
		apperrors.WriteError(writer, *apperrors.NewAppError("ERR_MISSING_QUERY", "Missing the 'since' query", http.StatusBadRequest))
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
