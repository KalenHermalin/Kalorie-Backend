package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"main/internal/models"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type GitHubProvider struct {
	config *oauth2.Config
}

func NewGitHubProvider(id, secret, callback string, scopes []string) *GitHubProvider {
	if scopes == nil {
		scopes = []string{"read:user", "user:email"}
	}
	return &GitHubProvider{
		config: &oauth2.Config{
			ClientID:     id,
			ClientSecret: secret,
			RedirectURL:  callback,
			Endpoint:     github.Endpoint, // Pre-defined by the library
			Scopes:       scopes,
		},
	}
}

type GitHubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (gh *GitHubProvider) HandleCodeExchangeWithVerifier(ctx context.Context, code string, verifier string) (*models.AuthPayload, error) {
	//TODO: Parent should handle validation of input to amke sure not empty
	token, err := gh.config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, err
	}
	client := gh.config.Client(ctx, token)

	// Get user ID
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		fmt.Printf("ERROR 1: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	gitPayload := &models.AuthPayload{}

	err = json.NewDecoder(resp.Body).Decode(gitPayload)

	if err != nil {
		return nil, err
	}
	// Getting Email
	resp, err = client.Get("https://api.github.com/user/emails")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Read the body as a string to see the actual error message from GitHub
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("GitHub API error (status %d): %s", resp.StatusCode, string(bodyBytes))
		return nil, errors.New("TEST")
	}
	var emails []GitHubEmail
	err = json.NewDecoder(resp.Body).Decode(&emails)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return nil, err
	}

	// 2. Find the primary one
	var primaryEmail string
	for _, e := range emails {
		if e.Primary && e.Verified {
			primaryEmail = e.Email
			break
		}
	}
	gitPayload.Email = primaryEmail
	gitPayload.Provider = gh.GetProviderName()
	return gitPayload, nil
}

func (gh *GitHubProvider) GetProviderName() string {
	return "github"
}
func (gh *GitHubProvider) GetPlatform() string {
	return ""
}
