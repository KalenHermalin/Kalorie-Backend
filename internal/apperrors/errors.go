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

var (
	ErrUnauthoirized = &AppError{
		Code:    "ERR_UNAUTHORIZED",
		Message: "You are not authorized. Please sign in!",
		Status:  http.StatusUnauthorized,
	}
	AuthErrInvalidFormat = &AppError{
		Code:    "ERR_INVALID_TOKEN_FORMAT",
		Message: "Invalid authorization format.",
		Status:  http.StatusUnauthorized,
	}
	AuthErrInvalidToken = &AppError{
		Code:    "ERR_INVALID_TOKEN",
		Message: "Token invalid or expired. Please try logging in again.",
		Status:  http.StatusUnauthorized,
	}
	AuthErrInvalidProvider = &AppError{
		Code:    "ERR_INVALID_PROVIDER",
		Message: "Invalid auth provider. Try another!",
		Status:  http.StatusBadRequest,
	}
	AuthErrLogoutFailed = &AppError{
		Code:    "ERR_LOGOUT_FAILED",
		Message: "Logout failed. Please try again in a few.",
		Status:  http.StatusInternalServerError,
	}
	AuthErrLoginFailed = &AppError{
		Code:    "ERR_LOGIN_FAILED",
		Message: "Login failed. Please try again in a few.",
		Status:  http.StatusBadRequest,
	}
	AuthErrInternal = &AppError{
		Code:    "ERR_INTERNAL_AUTH",
		Message: "Internal server authentication error. Please contact support!",
		Status:  http.StatusInternalServerError,
	}
	AuthErrUnavailableService = &AppError{
		Code:    "ERR_AUTH_SERVICE_UNAVAILABLE",
		Message: "Authentication service temporarily unavailable. Please try again shortly.",
		Status:  http.StatusServiceUnavailable,
	}
	AuthErrUnexpected = &AppError{
		Code:    "ERR_UNEXPECTED_AUTH_ERROR",
		Message: "An unexpected authentication error occurred.",
		Status:  http.StatusInternalServerError,
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
)
