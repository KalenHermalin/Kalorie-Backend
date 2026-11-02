package apperrors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type ErrorKind struct {
	Code    string `json:"code"`    // Machine-Readable (e.g., ERR_USER_NOT_FOND)
	Message string `json:"message"` // Humman-Readable
	Status  int    `json:"-"`
}

var (
	BadRequest           = ErrorKind{Code: "ERR_INVALID_REQUEST", Status: http.StatusBadRequest, Message: "The request is invalid"}
	Internal             = ErrorKind{Code: "ERR_INTERNAL_SERVER", Status: http.StatusInternalServerError, Message: "The server encountered an unexpected error"}
	Unauthorized         = ErrorKind{Code: "ERR_UNAUTHORIZED", Status: http.StatusUnauthorized, Message: "Please sign in"}
	InvalidToken         = ErrorKind{Code: "ERR_INVALID_TOKEN", Status: http.StatusUnauthorized, Message: "Your login token is not valid"}
	MalformedAuthHeader  = ErrorKind{Code: "ERR_AUTH_HEADER_MALFORMED", Status: http.StatusBadRequest, Message: "Malformed auth header. Should be Authorization: Bearer <token>"}
	BadLoginRequest      = ErrorKind{Code: "ERR_LOGIN_REQUEST", Status: http.StatusBadRequest, Message: "Sign in didn't work. Please try again!"}
	AuthUnavailable      = ErrorKind{Code: "ERR_AUTH_UNAVAILABLE", Status: http.StatusServiceUnavailable, Message: "Service currently unavailable. Please wait a few minutes!"}
	SessionOver          = ErrorKind{Code: "ERR_SESSION_OVER", Status: http.StatusUnauthorized, Message: "Your current session already ended. Please sign in again!"}
	InvalidProvider      = ErrorKind{Code: "ERR_INVALID_PROVIDER", Status: http.StatusBadRequest, Message: "The login provider selected is not valid"}
	MaxBatchSizeExceeded = ErrorKind{Code: "ERR_MAX_BATCH_SIZE", Status: http.StatusBadRequest, Message: "Exceeded match batch size. Refer to documentation."}
	NoFoodFound          = ErrorKind{Code: "ERR_NO_FOOD", Status: http.StatusBadRequest, Message: "There was no food found in the image."}
	NoLabelFound         = ErrorKind{Code: "ERR_NO_LABEL", Status: http.StatusBadRequest, Message: "There was no label found in the image."}
	LlmProviderDown      = ErrorKind{Code: "ERR_LLM_PROVIDER", Status: http.StatusServiceUnavailable, Message: "The AI provider is currently experiencing issues. Try again later."}
	Conflict             = ErrorKind{Code: "ERR_CONFLICT", Status: http.StatusConflict, Message: "The email already exists"}
)

type FieldError struct {
	Field   string `json:"field"`
	Problem string `json:"problem"`
}
type KError struct {
	Kind   ErrorKind
	Fields []FieldError
	cause  error // The original error
}

func New(k ErrorKind, cause error) *KError {
	return &KError{
		Kind:  k,
		cause: cause,
	}
}

func NewBadRequestWithFields(cause error, fields ...FieldError) *KError { // Bad request error with fields
	return &KError{
		Kind:   BadRequest,
		cause:  cause,
		Fields: fields,
	}
}

func (e *KError) Error() string { // For logging code and cause
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Kind.Code, e.cause)

	}
	return e.Kind.Code
}
func (e *KError) Unwrap() error { // To expose cause for use in errors.Is/As
	return e.cause
}

type errorResponse struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
}

func Write(w http.ResponseWriter, kerr *KError) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(kerr.Kind.Status)
	resp := &errorResponse{
		Code:    kerr.Kind.Code,
		Message: kerr.Kind.Message,
		Fields:  kerr.Fields,
	}
	json.NewEncoder(w).Encode(resp)
}

func Is(e error, k ErrorKind) bool {
	var kerr *KError
	if !errors.As(e, &kerr) { // Search error chain and fills kerr with a KError found in the chain
		return false // if not found they cant be the same
	}
	return kerr.Kind == k // Else return based on if they are the same
}
