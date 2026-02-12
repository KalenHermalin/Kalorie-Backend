package models

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
)

type AuthPayload struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Provider string `json:"provider"`
}
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // Standard naming convention
	User         *User
}
type AuthProvider interface {
	GetProviderName() string
	//TODO: make a HandleCodeExchange without the verifier needed
	HandleCodeExchangeWithVerifier(ctx context.Context, code, verifier string) (*AuthPayload, error)
	GetPlatform() string
}

type CustomClaimsAccess struct {
	UserID int    `json:"sub"`
	Email  string `json:"email"`
	// Future-proofing: add a field for subscription status
	IsPremium bool `json:"is_premium"`
	jwt.RegisteredClaims
}

type CustomClaimsRefresh struct {
	UserID int `json:"sub"`
	jwt.RegisteredClaims
}
