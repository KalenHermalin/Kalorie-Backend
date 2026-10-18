package auth

import (
	"context"
	"fmt"
	"main/internal/auth"
	"main/internal/models"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

const (
	appleTokenURL = "https://appleid.apple.com/auth/token"
	appleAuthURL  = "https://appleid.apple.com/auth/authorize"
	appleKeysURL  = "https://appleid.apple.com/auth/keys"
	appleIssuer   = "https://appleid.apple.com"
	// Well under Apple's 6-month max for the client secret JWT's lifetime.
	appleClientSecretTTL = 30 * time.Minute
)

// AppleProvider implements Sign in with Apple. Unlike GitHub/Google, this
// has no static client secret (a short-lived ES256 JWT signed with your
// Apple-issued private key is generated on every request instead) and no
// userinfo endpoint - the user's id (sub) and email come from the
// id_token, verified against Apple's published JWKS.
type AppleProvider struct {
	clientID   string
	teamID     string
	keyID      string
	privateKey string // PEM, newlines already unescaped
	callback   string
	platform   string
	scopes     []string
	jwks       keyfunc.Keyfunc
}

// NewAppleProvider builds an AppleProvider. platform may be left empty -
// like GitHub, this is meant to be a single provider (not split per
// platform like Google), since Apple issues one client id/key pair per
// app regardless of platform.
func NewAppleProvider(clientID, teamID, keyID, privateKey, callback, platform string, scopes []string) (*AppleProvider, error) {
	if scopes == nil {
		scopes = []string{"name", "email"}
	}
	jwks, err := keyfunc.NewDefault([]string{appleKeysURL})
	if err != nil {
		return nil, err
	}
	return &AppleProvider{
		clientID:   clientID,
		teamID:     teamID,
		keyID:      keyID,
		privateKey: strings.ReplaceAll(privateKey, "\\n", "\n"),
		callback:   callback,
		platform:   platform,
		scopes:     scopes,
		jwks:       jwks,
	}, nil
}

func (ap *AppleProvider) generateClientSecret() (string, error) {
	key, err := jwt.ParseECPrivateKeyFromPEM([]byte(ap.privateKey))
	if err != nil {
		return "", fmt.Errorf("[Apple] parse private key: %w", err)
	}
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    ap.teamID,
		Subject:   ap.clientID,
		Audience:  jwt.ClaimStrings{appleIssuer},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(appleClientSecretTTL)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = ap.keyID
	jwtString, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("[Apple] create signed jwt: %w", err)

	}
	return jwtString, nil
}

func (ap *AppleProvider) HandleCodeExchangeWithVerifier(ctx context.Context, code, verifier string) (*models.AuthPayload, error) {
	clientSecret, err := ap.generateClientSecret()
	if err != nil {
		return nil, fmt.Errorf("[Apple] provider generate secret: %w", err)
	}

	config := oauth2.Config{
		ClientID:     ap.clientID,
		ClientSecret: clientSecret,
		RedirectURL:  ap.callback,
		Endpoint:     oauth2.Endpoint{AuthURL: appleAuthURL, TokenURL: appleTokenURL},
		Scopes:       ap.scopes,
	}

	// Always returns an KError so can return right away
	token, err := auth.ExchangeCode(ctx, code, verifier, config)
	if err != nil {
		return nil, err
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, fmt.Errorf("[Apple] handle code exchange missing <id_token>")
	}

	// Verify the id_token's signature against Apple's published JWKS
	// rather than trusting its claims blindly.
	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(rawIDToken, claims, ap.jwks.Keyfunc, jwt.WithIssuer(appleIssuer), jwt.WithAudience(ap.clientID))
	if err != nil {
		return nil, fmt.Errorf("[Apple] handle code exchange token verification: %w", err)
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return nil, fmt.Errorf("[Apple] handle code exchange sub: missing or not a string")

	}
	if sub == "" {
		return nil, fmt.Errorf("[Apple] handle code exchange: apple returned no sub")
	}
	email, ok := claims["email"].(string)
	if !ok {
		return nil, fmt.Errorf("[Apple] handle code exchange email: missing or not a string")

	}

	return &models.AuthPayload{
		Provider: ap.GetProviderName(),
		Email:    email,
		ID:       sub,
	}, nil
}

func (ap *AppleProvider) GetProviderName() string {
	return "apple"
}

func (ap *AppleProvider) GetPlatform() string {
	return ap.platform
}
