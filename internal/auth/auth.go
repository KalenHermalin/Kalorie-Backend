package auth

import (
	"context"
	"fmt"
	"main/internal/apperrors"
	"main/internal/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

func GenerateAccessToken(userID string, email string, isPremium bool, secret string) (string, error) {
	// 1. Create Access Token (JWT)
	claims := models.CustomClaimsAccess{
		UserID:    userID,
		Email:     email,
		IsPremium: isPremium,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedAccess, err := accessToken.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("generate access token: %w", err)
	}

	return signedAccess, nil
}
func GenerateRefreshToken(userID string, expiresIn time.Time, secret string) (string, error) {
	// 1. Create Access Token (JWT)
	claims := models.CustomClaimsRefresh{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresIn),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedAccess, err := refreshToken.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return signedAccess, nil
}

func ExchangeCode(ctx context.Context, code, verifier string, client oauth2.Config) (*oauth2.Token, error) {
	token, err := client.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		if re, ok := err.(*oauth2.RetrieveError); ok {
			switch re.ErrorCode {

			case "invalid_grant", "access_denied":
				//Please try again in a few
				return nil, apperrors.New(apperrors.BadLoginRequest, err)

			case "unauthorized_client", "invalid_client", "invalid_scope":
				// Configuration Error
				return nil, apperrors.New(apperrors.Internal, err)

			case "server_error", "temporarily_unavailable":
				// auth provider temporarily down
				return nil, apperrors.New(apperrors.AuthUnavailable, err)
			default:
				//unknown error
				return nil, apperrors.New(apperrors.Internal, err)

			}

		}
		return nil, apperrors.New(apperrors.Internal, err)
	}
	return token, nil

}
