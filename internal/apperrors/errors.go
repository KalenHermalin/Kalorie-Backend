package apperrors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type AppError struct {
	Code    string `json:"code"`    // Machine-Readable (e.g., ERR_USER_NOT_FOND)
	Message string `json:"message"` // Humman-Readable
	Status  int    `json:"-"`       // Not sent in JSON, used to send statusCode
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewAppError(code, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}
func WriteError(w http.ResponseWriter, appErr AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Status)

	// Only sends code and message because of the - in json marshal struct
	json.NewEncoder(w).Encode(appErr)
}

//TODO: App Errors should be for internal use in the service layer

var (
	ErrBadRequestBody = &AppError{
		Code:    "ERR_BAD_REQUEST",
		Message: "Error decoding request body",
		Status:  http.StatusBadRequest,
	}
	ErrUnauthoirized = &AppError{
		Code:    "ERR_UNAUTHORIZED",
		Message: "You are not authorized. Please sign in!",
		Status:  http.StatusUnauthorized,
	}
	ErrInvalidToken = &AppError{
		Code:    "ERR_INVALID_TOKEN",
		Message: "Token invalid or expired. Please try logging in again.",
		Status:  http.StatusUnauthorized,
	}
	ErrInvalidTokenFormat = &AppError{
		Code:    "ERR_INVALID_TOKEN_FORMAT",
		Message: "Token format is not correct. Expected:  `Authorization: Bearer <access_token>`",
		Status:  http.StatusBadRequest,
	}

	ErrInvalidProvider = &AppError{
		Code:    "ERR_INVALID_PROVIDER",
		Message: "Invalid auth provider. Try another!",
		Status:  http.StatusBadRequest,
	}
	AuthErrLoginFailed = &AppError{
		Code:    "ERR_LOGIN_FAILED",
		Message: "Login failed with login provider. Please try again in a few.",
		Status:  http.StatusBadRequest,
	}
	AuthErrUnavailableService = &AppError{
		Code:    "ERR_AUTH_SERVICE_UNAVAILABLE",
		Message: "Authentication service temporarily unavailable. Please try again shortly.",
		Status:  http.StatusServiceUnavailable,
	}
	ErrInternalServer = &AppError{
		Code:    "ERR_INTERNAL_SERVER",
		Message: "An internal server error occured",
		Status:  http.StatusInternalServerError,
	}

	ErrInvalidRequest = &AppError{
		Code:    "ERR_INVALID_REQUEST",
		Message: "The provided request is invalid. Please refer to documentation!",
		Status:  http.StatusBadRequest,
	}
	ErrMissingUserIdInContext = &AppError{
		Code:    "ERR_MISSING_USER_ID",
		Message: "Missing UserId from context",
		Status:  http.StatusInternalServerError,
	}
	ErrUserSettingsMissing = &AppError{
		Code:    "ERR_USER_SETTINGS_MISSING",
		Message: "User Settings do not exist. Please contact support!",
		Status:  http.StatusInternalServerError,
	}
	ErrSyncConflict = &AppError{
		Code:    "ERR_SYNC_CONFLICT",
		Message: "A newer version of this data already exists",
		Status:  http.StatusConflict,
	}
	ErrNotModified = &AppError{
		Code:    "ERR_NOT_MODIFIED",
		Message: "The requested resournce wasnt modified since the date requested",
		Status:  http.StatusNotModified,
	}
)
