package auth

import (
	"context"
	"log/slog"
	"main/internal/apperrors"
	"main/internal/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

func GenerateAccessToken(userID int, email string, isPremium bool, secret string) (string, error) {
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
		return "", err
	}

	return signedAccess, err
}
func GenerateRefreshToken(userID int, expiresIn time.Time, secret string) (string, error) {
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
		return "", err
	}
	return signedAccess, nil
}

func ExchangeCode(ctx context.Context, code, verifier string, client oauth2.Config) (*oauth2.Token, error) {
	token, err := client.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		if re, ok := err.(*oauth2.RetrieveError); ok {
			slog.Error("Error: oauth2 token exchange", "ErrorCode", re.ErrorCode, "Description", re.ErrorDescription)
			switch re.ErrorCode {

			case "invalid_grant", "access_denied":
				//Please try again in a few
				return nil, apperrors.AuthErrLoginFailed

			case "unauthorized_client", "invalid_scope":
				// internal server erro
				return nil, apperrors.AuthErrInternal

			case "server_error", "temporarily_unavailable":
				// auth provider temporarily down
				return nil, apperrors.AuthErrUnavailableService
			default:
				//unknown error
				return nil, apperrors.AuthErrUnexpected

			}

		}
		slog.Error("Error: Non oauth2", "error", err.Error())
		return nil, apperrors.ErrInternalServer
	}
	return token, nil

}
