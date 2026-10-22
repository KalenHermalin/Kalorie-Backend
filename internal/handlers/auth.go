package handlers

import (
	"encoding/json"
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

func (ah *AuthHandler) HandleRefresh(writer http.ResponseWriter, request *http.Request) error {

	requestData := &logOutRequestBody{}
	if err := utils.DecodePayload(request.Body, requestData); err != nil {
		return apperrors.NewBadRequestWithFields(err, apperrors.FieldError{Field: "refresh", Problem: "missing or malformed field or request"})

	}
	resp, err := ah.auth.RefreshAccessToken(request.Context(), requestData.Refresh)
	if err != nil {
		return err
	}

	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(resp) // Now 'resp' is used!
	return nil
}

func (ah *AuthHandler) HandleLogOut(writer http.ResponseWriter, request *http.Request) error {
	userId := middlewares.GetUserID(request.Context())
	// Get refresh token from request body
	requestData := &logOutRequestBody{}
	if err := utils.DecodePayload(request.Body, requestData); err != nil {
		return apperrors.NewBadRequestWithFields(err, apperrors.FieldError{Field: "refresh", Problem: "missing or malformed field or request"})
	}
	err := ah.auth.LogOut(request.Context(), userId, requestData.Refresh)
	if err != nil {
		return err
	}
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("Success"))
	return nil
}

func (ah *AuthHandler) HandleDeleteUser(writer http.ResponseWriter, request *http.Request) error {
	userId := middlewares.GetUserID(request.Context())

	err := ah.auth.DeleteUser(request.Context(), userId)
	if err != nil {
		return err
	}
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("Success"))
	return nil
}
func (ah *AuthHandler) HandleLoginSignup(writer http.ResponseWriter, request *http.Request) error {

	var requestData authRequestBody
	if err := utils.DecodePayload(request.Body, &requestData); err != nil {
		return apperrors.NewBadRequestWithFields(err, apperrors.FieldError{Field: "refresh", Problem: "missing or malformed field or request"})

	}
	if err := utils.CheckValidString(requestData.Code); err != nil {
		return apperrors.NewBadRequestWithFields(err, apperrors.FieldError{Field: "refresh", Problem: "code is empty"})

	}
	if err := utils.CheckValidString(requestData.Provider); err != nil {
		return apperrors.NewBadRequestWithFields(err, apperrors.FieldError{Field: "refresh", Problem: "provider is empty"})
	}
	resp, err := ah.auth.SignIn(request.Context(), requestData.Code, requestData.Provider, requestData.Verifier, requestData.Platform)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(resp)
	return nil

}
