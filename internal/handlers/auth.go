package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"main/internal/apperrors"
	"main/internal/middlewares"
	"main/internal/service"
	"main/internal/utils"
	"net/http"
)

type AuthHandler struct {
	auth service.AuthService
}

func NewAuthHandler(providers service.AuthService) *AuthHandler {
	return &AuthHandler{auth: providers}
}

type authRequestBody struct {
	Code     string  `json:"code"`
	Provider string  `json:"provider"`
	Verifier string  `json:"verifier"`
	Platform *string `json:"platform"`
}

type logOutRequestBody struct {
	Refresh string `json:"refresh"`
}

func (ah *AuthHandler) HandleRefresh(writer http.ResponseWriter, request *http.Request) {

	requestData := &logOutRequestBody{}
	if err := utils.DecodePayload(request.Body, requestData); err != nil {
		slog.Error("Error: decoding refresh request body", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInvalidRequest)
		return
	}
	resp, err := ah.auth.RefreshAccessToken(request.Context(), requestData.Refresh)
	if err != nil {
		slog.Error("Error: refreshing access token", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.AuthErrInvalidToken)
	}

	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(resp) // Now 'resp' is used!
}

func (ah *AuthHandler) HandleLogOut(writer http.ResponseWriter, request *http.Request) {
	// Get userId from auth middleware
	userId, ok := request.Context().Value(middlewares.UserIDKey).(int)
	if !ok {
		apperrors.WriteError(writer, *apperrors.ErrUnauthoirized)
		return
	}

	// Get refresh token from request body
	requestData := &logOutRequestBody{}
	if err := utils.DecodePayload(request.Body, requestData); err != nil {
		slog.Error("Error: decoding logout request body", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInvalidRequest)
		return
	}
	err := ah.auth.LogOut(request.Context(), userId, requestData.Refresh)
	if err != nil {
		slog.Error("Error: deleting refresh token in database", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.AuthErrLogoutFailed)
		return
	}
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("Success"))
}
func (ah *AuthHandler) HandleLoginSignup(writer http.ResponseWriter, request *http.Request) {

	var requestData authRequestBody
	if err := utils.DecodePayload(request.Body, &requestData); err != nil {
		// Bad Request because all we did was decode it and got an error meaning invalid JSON
		slog.Error("Error: decoding login request body", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInvalidRequest)
		return
	}
	if err := utils.CheckValidString(requestData.Code); err != nil {
		slog.Error("Error: code is empty", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInvalidRequest)
		return
	}
	if err := utils.CheckValidString(requestData.Provider); err != nil {
		slog.Error("Error: provider is empty", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInvalidRequest)
		return
	}
	resp, err := ah.auth.SignIn(request.Context(), requestData.Code, requestData.Provider, requestData.Verifier, requestData.Platform)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			apperrors.WriteError(writer, *appErr)
			return

		}
		slog.Error("Error: Sign Up / Login Failed", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInternalServer)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(resp)

}
