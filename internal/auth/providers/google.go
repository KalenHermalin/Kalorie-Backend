package auth

import (
	"context"
	"log/slog"
	"main/internal/apperrors"
	"main/internal/auth"
	"main/internal/models"
	"main/internal/utils"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleProvider struct {
	config   *oauth2.Config
	platform string
}

func NewGoogleProvider(id, secret, callback, platform string, scopes []string) *GoogleProvider {
	if scopes == nil {
		scopes = []string{"openid", "profile", "email"}
	}
	return &GoogleProvider{
		config: &oauth2.Config{
			ClientID: id,
			//ClientSecret: secret,
			RedirectURL: callback,
			Endpoint:    google.Endpoint, // Pre-defined by the library
			Scopes:      scopes,
		},
		platform: platform,
	}
}

type GoogleResponse struct {
	ID    string `json:"sub"`
	Email string `json:"email"`
}

func (gh *GoogleProvider) HandleCodeExchangeWithVerifier(ctx context.Context, code string, verifier string) (*models.AuthPayload, error) {
	// Always returns an apperror so can return right away
	token, err := auth.ExchangeCode(ctx, code, verifier, *gh.config)
	if err != nil {
		return nil, err
	}
	client := gh.config.Client(ctx, token)

	// Get user ID
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		slog.Error("Error: oauth2 user info", "provider", gh.GetProviderName(), "error", err.Error())
		return nil, apperrors.AuthErrUnexpected

	}
	googleResponse := &GoogleResponse{}
	err = utils.DecodePayload(resp.Body, googleResponse)
	if err != nil {
		slog.Error("Error: decoding user data", "error", err.Error())
		return nil, apperrors.AuthErrUnexpected

	}

	googlePayload := &models.AuthPayload{
		Provider: gh.GetProviderName(),
		Email:    googleResponse.Email,
		ID:       googleResponse.ID,
	}
	return googlePayload, nil
}

func (gh *GoogleProvider) GetProviderName() string {
	return "google"
}
func (gh *GoogleProvider) GetPlatform() string {
	return gh.platform
}
