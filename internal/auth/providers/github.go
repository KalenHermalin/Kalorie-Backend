package auth

import (
	"context"
	"io"
	"log/slog"
	"main/internal/apperrors"
	"main/internal/auth"
	"main/internal/models"
	"main/internal/utils"
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

	// Only ever returns an apperror, can be instantly returned to handler
	token, err := auth.ExchangeCode(ctx, code, verifier, *gh.config)
	if err != nil {
		return nil, err
	}
	client := gh.config.Client(ctx, token)

	// Get user ID
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		slog.Error("Error: oauth2 user info", "provider", gh.GetProviderName(), "error", err.Error())
		return nil, apperrors.ErrUnexpectedAuth
	}

	gitPayload := &models.AuthPayload{}
	err = utils.DecodePayload(resp.Body, gitPayload)
	if err != nil {
		slog.Error("Error: decoding user data", "error", err.Error())
		return nil, apperrors.ErrUnexpectedAuth
	}
	// Getting Email
	resp, err = client.Get("https://api.github.com/user/emails")
	if err != nil {
		slog.Error("Error: oauth2 user email info", "provider", gh.GetProviderName(), "error", err.Error())
		return nil, apperrors.ErrUnexpectedAuth
	}
	if resp.StatusCode != http.StatusOK {
		// Read the body as a string to see the actual error message from GitHub
		bodyBytes, _ := io.ReadAll(resp.Body)

		slog.Error("Error: oauth provider api error", "status", "provider", gh.GetProviderName(), resp.StatusCode, "message", string(bodyBytes))
		return nil, apperrors.ErrUnavailableAuthService
	}
	var emails []GitHubEmail
	err = utils.DecodePayload(resp.Body, &emails)
	//err = json.NewDecoder(resp.Body).Decode(&emails)
	//defer resp.Body.Close()

	if err != nil {
		slog.Error("Error: extracting oauth2 user email info", "provider", gh.GetProviderName(), "error", err.Error())
		return nil, apperrors.ErrUnexpectedAuth
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
