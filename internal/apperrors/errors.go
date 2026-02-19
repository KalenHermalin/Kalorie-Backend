package apperrors

import (
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

var (
	ErrUnauthoirized = &AppError{
		Code:    "ERR_UNAUTHORIZED",
		Message: "You are not authorized. Please sign in!",
		Status:  http.StatusUnauthorized,
	}
	ErrInvalidProvider = &AppError{
		Code:    "ERR_INVALID_PROVIDER",
		Message: "Invalid auth provider. Try another!",
		Status:  http.StatusBadRequest,
	}
	ErrLoginFailed = &AppError{
		Code:    "ERR_LOGIN_FAILED",
		Message: "Login failed. Please try again in a few.",
		Status:  http.StatusBadRequest,
	}
	ErrInternalAuth = &AppError{
		Code:    "ERR_INTERNAL_AUTH",
		Message: "Internal server authentication error. Please contact support!",
		Status:  http.StatusInternalServerError,
	}
	ErrUnavailableAuthService = &AppError{
		Code:    "ERR_AUTH_SERVICE_UNAVAILABLE",
		Message: "Authentication service temporarily unavailable. Please try again shortly.",
		Status:  http.StatusServiceUnavailable,
	}
	ErrUnexpectedAuth = &AppError{
		Code:    "ERR_UNEXPECTED_AUTH_ERROR",
		Message: "An unexpected authentication error occurred.",
		Status:  http.StatusInternalServerError,
	}
	ErrInternalServer = &AppError{
		Code:    "ERR_INTERNAL_SERVER",
		Message: "An internal server error occured",
		Status:  http.StatusInternalServerError,
	}
)
