package handlers

import (
	"encoding/json"
	"log"
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
	if err := utils.DecodePayload(request, requestData); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := ah.auth.RefreshAccessToken(request.Context(), requestData.Refresh)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		log.Printf("Error refreshing access token: %v\n", err)
	}

	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(resp) // Now 'resp' is used!
}

func (ah *AuthHandler) HandleLogOut(writer http.ResponseWriter, request *http.Request) {
	// Get userId from auth middleware
	userId, ok := request.Context().Value(middlewares.UserIDKey).(int)
	if !ok {
		http.Error(writer, "Unauthorzied", http.StatusUnauthorized)
		return
	}

	// Get refresh token from request body
	requestData := &logOutRequestBody{}
	if err := utils.DecodePayload(request, requestData); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	err := ah.auth.LogOut(request.Context(), userId, requestData.Refresh)
	if err != nil {
		log.Printf("Failed to delete refresh token: %v", err)
	}
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("Success"))
}
func (ah *AuthHandler) HandleLoginSignup(writer http.ResponseWriter, request *http.Request) {

	var requestData authRequestBody
	if err := utils.DecodePayload(request, &requestData); err != nil {
		// Bad Request because all we did was decode it and got an error meaning invalid JSON
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := utils.CheckValidString(requestData.Code); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := utils.CheckValidString(requestData.Provider); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := ah.auth.SignIn(request.Context(), requestData.Code, requestData.Provider, requestData.Verifier, requestData.Platform)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(resp) // Now 'resp' is used!

}
