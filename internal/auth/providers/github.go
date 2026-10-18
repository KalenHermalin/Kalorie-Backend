package auth

import (
	"context"
	"fmt"
	"io"
	"main/internal/auth"
	"main/internal/models"
	"main/internal/utils"
	"net/http"
	"strconv"

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
type initialGitPayload struct {
	ID       int    `json:"id"`
	Email    string `json:"email"`
	Provider string `json:"provider"`
}

func (gh *GitHubProvider) HandleCodeExchangeWithVerifier(ctx context.Context, code string, verifier string) (*models.AuthPayload, error) {

	// Always returns an KError so can return right away
	token, err := auth.ExchangeCode(ctx, code, verifier, *gh.config)
	if err != nil {
		return nil, err
	}
	client := gh.config.Client(ctx, token)

	// Get user ID
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("[GitHub] handle code exchange user info: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// A non-200 here (e.g. a bad/expired token) still has a JSON body,
		// just not one with `id`/`email` fields - decoding it below would
		// otherwise silently succeed with a zero-value payload instead of
		// surfacing an error.
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("[GitHub] handle code exchange user info resp | status: %d : %s", resp.StatusCode, bodyBytes)
	}

	gitPayload := &initialGitPayload{}
	err = utils.DecodePayload(resp.Body, gitPayload)
	if err != nil {
		return nil, fmt.Errorf("[GitHub] handle code exchange decode user info: %w", err)
	}
	if gitPayload.ID == 0 {
		return nil, fmt.Errorf("[GitHub] handle code exchange: github returned no id")

	}
	// Getting Email
	resp, err = client.Get("https://api.github.com/user/emails")
	if err != nil {
		return nil, fmt.Errorf("[GitHub] handle code exchange user email: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// Read the body as a string to see the actual error message from GitHub
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("[GitHub] handle code exchange user email resp | status: %d : %s", resp.StatusCode, bodyBytes)
	}
	var emails []GitHubEmail
	err = utils.DecodePayload(resp.Body, &emails)
	if err != nil {
		return nil, fmt.Errorf("[GitHub] handle code exchange decode user email: %w", err)
	}

	// 2. Find the primary one
	var primaryEmail string
	for _, e := range emails {
		if e.Primary && e.Verified {
			primaryEmail = e.Email
			break
		}
	}
	authPayload := &models.AuthPayload{}
	authPayload.Email = primaryEmail
	authPayload.Provider = gh.GetProviderName()
	authPayload.ID = strconv.Itoa(gitPayload.ID)
	return authPayload, nil
}

func (gh *GitHubProvider) GetProviderName() string {
	return "github"
}
func (gh *GitHubProvider) GetPlatform() string {
	return ""
}
