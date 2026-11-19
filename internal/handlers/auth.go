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
	TokenID string `json:"token_id"`
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
		return apperrors.NewBadRequestWithFields(err, apperrors.FieldError{Field: "body", Problem: "malformed request body"})

	}
	if err := utils.CheckValidString(requestData.TokenID); err != nil {
		return apperrors.NewBadRequestWithFields(err, apperrors.FieldError{Field: "token_id", Problem: "token_id is empty"})

	}
	resp, err := ah.auth.SignIn(request.Context(), requestData.TokenID)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(resp)
	return nil

}
