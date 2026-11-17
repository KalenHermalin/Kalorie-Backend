package models

import (
	"context"
	"database/sql"

	"github.com/golang-jwt/jwt/v5"
)

type AuthPayload struct {
	ID    string  `json:"id"`
	Email *string `json:"email"`
}
type AuthResponse struct {
	UserId string `json:"user_id"`
}
type AuthProvider interface {
	GetProviderName() string
	HandleCodeExchangeWithVerifier(ctx context.Context, code, verifier string) (*AuthPayload, error)
	GetPlatform() string
}
type AuthLookUp interface {
	// Returns users UUID as string or an error
	FirebaseToUUID(ctx context.Context, firebaseID string) (string, error)
}
type AuthRepository interface {
	UpsertUserWithAuth(ctx context.Context, tx *sql.Tx, payload *AuthPayload) (*User, error)
	DeleteUser(ctx context.Context, tx *sql.Tx, userID string) error
	LookupFirebaseIdToUserId(ctx context.Context, firebaseID string) (string, error)
	LookupUserIdToFirebase(ctx context.Context, userid string) (string, error)
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
