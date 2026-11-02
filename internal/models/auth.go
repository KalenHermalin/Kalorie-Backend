package models

import (
	"context"
	"database/sql"
	"time"

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
	// Explicit lowercase tag: without one, an untagged exported field
	// serializes using its literal Go name ("User", capitalized),
	// breaking the snake_case convention every other field here follows -
	// and silently, since it's still valid JSON, just not what a client
	// decoding a `user` field would ever see.
	User *User `json:"user"`
}
type AuthProvider interface {
	GetProviderName() string
	HandleCodeExchangeWithVerifier(ctx context.Context, code, verifier string) (*AuthPayload, error)
	GetPlatform() string
}

type AuthRepository interface {
	UpsertUserWithAuth(ctx context.Context, tx *sql.Tx, payload *AuthPayload) (*User, error)
	DeleteRefreshToken(ctx context.Context, tx *sql.Tx, token string, userId string) error
	SaveRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId string, expiresAt time.Time) error
	FindRefreshToken(ctx context.Context, tx *sql.Tx, refresh string) (*User, error)
	DeleteUser(ctx context.Context, tx *sql.Tx, userID string) error
	WithTx(ctx context.Context, fn func(*sql.Tx) error) error
}
type CustomClaimsAccess struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	// Future-proofing: add a field for subscription status
	IsPremium bool `json:"is_premium"`
	jwt.RegisteredClaims
}

type CustomClaimsRefresh struct {
	UserID string `json:"sub"`
	jwt.RegisteredClaims
}
