package auth

import (
	"main/internal/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

	return signedAccess, err
}
