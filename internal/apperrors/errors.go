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
	BadRequest          = ErrorKind{Code: "ERR_INVALID_REQUEST", Status: http.StatusBadRequest, Message: "The Request is invalid"}
	Internal            = ErrorKind{Code: "ERR_INTERNAL_SERVER", Status: http.StatusInternalServerError, Message: "The server encountered an unexpected error"}
	Unauthorized        = ErrorKind{Code: "ERR_UNAUTHORIZED", Status: http.StatusUnauthorized, Message: "Please sign in"}
	InvalidToken        = ErrorKind{Code: "ERR_INVALID_TOKEN", Status: http.StatusUnauthorized, Message: "Your login token is not valid"}
	MalformedAuthHeader = ErrorKind{Code: "ERR_AUTH_HEADER_MALFORMED", Status: http.StatusBadRequest, Message: "Malform auth header. Should be Authorization: Bearer <token>"}
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

func NewInvalid(fields ...FieldError) *KError { // Bad request error with fields
	return &KError{
		Kind:   BadRequest,
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

/*var (


	AuthErrUnavailableService = &AppError{
		Code:    "ERR_AUTH_SERVICE_UNAVAILABLE",
		Message: "Authentication service temporarily unavailable. Please try again shortly.",
		Status:  http.StatusServiceUnavailable,
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
	ErrSoftWeightLog = &AppError{
		Code:    "ERR_SOFT_UPSETING_WEIGHT_LOG",
		Message: "There was an internal error updating some of yout weight logs",
		Status:  http.StatusMultiStatus,
	}
	ErrLLMProvider = &AppError{
		Code:    "ERR_LLM_PRIVDER",
		Message: "There is an error with the LLM provider",
		Status:  http.StatusInternalServerError,
	}
	ErrNoFoodFound = &AppError{
		Code:    "ERR_NO_FOOD",
		Message: "There was no food found in the submitted photo",
		Status:  http.StatusBadRequest,
	}
	ErrNoLabelFound = &AppError{
		Code:    "ERR_NO_:LABEL",
		Message: "There was no label found in the submitted photo",
		Status:  http.StatusBadRequest,
	}
)*/
