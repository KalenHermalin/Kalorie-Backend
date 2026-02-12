package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"main/internal/models"

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
	fmt.Printf("Verifier: %s\n", verifier)
	token, err := gh.config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		fmt.Printf("Error: %v", err)
		return nil, err
	}
	client := gh.config.Client(ctx, token)

	// Get user ID
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		fmt.Printf("ERROR 1: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	googleResponse := &GoogleResponse{}
	googlePayload := &models.AuthPayload{}

	err = json.NewDecoder(resp.Body).Decode(googleResponse)

	if err != nil {
		return nil, err
	}
	googlePayload.Provider = gh.GetProviderName()
	googlePayload.Email = googleResponse.Email
	googlePayload.ID = googleResponse.ID
	return googlePayload, nil
}

func (gh *GoogleProvider) GetProviderName() string {
	return "google"
}
func (gh *GoogleProvider) GetPlatform() string {
	return gh.platform
}
