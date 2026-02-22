package service

import (
	"context"
	"database/sql"
	"github.com/golang-jwt/jwt/v5"
	"log/slog"
	"main/internal/apperrors"
	"main/internal/auth"
	"main/internal/models"
	"time"
)

type AuthService struct {
	providers        []models.AuthProvider
	us               models.UserRepository
	jwtAccessSecret  string
	jwtRefreshSecret string
}

func NewAuthService(ur models.UserRepository, accessSecret, refreshSecret string, auth ...models.AuthProvider) *AuthService {
	return &AuthService{us: ur, jwtAccessSecret: accessSecret, jwtRefreshSecret: refreshSecret, providers: auth}
}

func (as *AuthService) LogOut(ctx context.Context, userId int, token string) error {

	err := as.us.WithTx(ctx, func(tx *sql.Tx) error {
		if err := as.us.DeleteRefreshToken(ctx, tx, token, userId); err != nil {
			return err
		}
		return nil
	})
	return err
}
func (as *AuthService) RefreshAccessToken(ctx context.Context, refresh string) (*models.AuthResponse, error) {

	// 1. Parse refreshToken string to get claims (and the UserID)
	var claims models.CustomClaimsRefresh
	token, err := jwt.ParseWithClaims(refresh, &claims, func(token *jwt.Token) (any, error) {
		return []byte(as.jwtRefreshSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, apperrors.AuthErrInvalidToken
	}

	// 2. Start Transaction
	resp := &models.AuthResponse{}
	err = as.us.WithTx(ctx, func(tx *sql.Tx) error {

		// 3. Verify token exists in DB for that UserID
		user, err := as.us.FindRefreshToken(ctx, tx, refresh, claims.UserID)
		if err != nil {
			slog.Error("Error: Couldnt Find Refresh Token", "error", err.Error(), "userID", claims.UserID)
			return apperrors.AuthErrInvalidToken
		}
		// 4. Delete old token

		err = as.us.DeleteRefreshToken(ctx, tx, refresh, claims.UserID)
		if err != nil {
			slog.Error("Error: Couldnt Delete Refresh Token", "error", err.Error(), "userID", claims.UserID)
			return apperrors.AuthErrInvalidToken
		}
		// 5. Generate new tokens
		accessToken, err := auth.GenerateAccessToken(user.ID, user.Email, false, as.jwtAccessSecret)
		if err != nil {

			slog.Error("Error: Couldnt Generate Access Token", "error", err.Error(), "userID", claims.UserID)
			return apperrors.AuthErrInternal
		}
		refreshToken, err := auth.GenerateRefreshToken(user.ID, time.Now().Add(time.Hour*24*30), as.jwtRefreshSecret)
		if err != nil {
			slog.Error("Error: Couldnt Generate Refresh Token", "error", err.Error(), "userID", claims.UserID)
			return apperrors.AuthErrInternal
		}
		err = as.us.SaveRefreshToken(ctx, tx, refreshToken, user.ID, time.Now().Add(time.Hour*24*30))
		if err != nil {
			slog.Error("Error: Couldnt Save Refresh Token", "error", err.Error(), "userID", claims.UserID)
			return apperrors.AuthErrInternal
		}
		resp.AccessToken = accessToken
		resp.RefreshToken = refreshToken
		resp.ExpiresIn = 900
		resp.User = user

		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}
func (as *AuthService) SignIn(ctx context.Context, code, provider, verifier string, platform *string) (*models.AuthResponse, error) {
	// 1. Exchange the code for the AuthPayload (Email, ID, etc.)
	payload, err := as.ExchangeCode(ctx, code, provider, verifier, platform)
	if err != nil {
		return nil, err
	}
	resp := &models.AuthResponse{}
	err = as.us.WithTx(ctx, func(tx *sql.Tx) error {

		user, err := as.us.UpsertUserWithAuth(ctx, tx, payload)
		if err != nil {
			slog.Error("Error: Couldnt Upsert User", "error", err.Error(), "user_email", payload.Email)
			return apperrors.AuthErrInternal
		}

		// 3. Generate the stateless JWT for the mobile app
		accessToken, err := auth.GenerateAccessToken(user.ID, user.Email, false, as.jwtAccessSecret)
		if err != nil {
			slog.Error("Error: Couldnt Generate Access Token", "error", err.Error(), "userID", user.ID, "user_email", user.Email)
			return apperrors.AuthErrInternal
		}
		refresh, err := auth.GenerateRefreshToken(user.ID, time.Now().Add(time.Hour*24*30), as.jwtRefreshSecret)
		if err != nil {
			slog.Error("Error: Couldnt Generate Refresh Token", "error", err.Error(), "userID", user.ID, "user_email", user.Email)
			return apperrors.AuthErrInternal
		}

		err = as.us.SaveRefreshToken(ctx, tx, refresh, user.ID, time.Now().Add(time.Hour*24*30))
		if err != nil {
			slog.Error("Error: Couldnt Save Refresh Token", "error", err.Error(), "userID", user.ID, "user_email", user.Email)
			return apperrors.AuthErrInternal
		}
		resp.AccessToken = accessToken
		resp.RefreshToken = refresh
		resp.ExpiresIn = 900
		resp.User = user

		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil

}

func (as *AuthService) ExchangeCode(ctx context.Context, code, provider, verifier string, platform *string) (*models.AuthPayload, error) {
	if platform == nil {
		for _, p := range as.providers {
			if p.GetProviderName() == provider {
				// Already returns apperror due to helper function in /auth/auth.go
				auth, err := p.HandleCodeExchangeWithVerifier(ctx, code, verifier)
				if err != nil {
					return nil, err
				}
				return auth, nil
			}
		}
	}
	for _, p := range as.providers {
		if p.GetProviderName() == provider && p.GetPlatform() == *platform {
			// Already returns apperror due to helper function in /auth/auth.go
			auth, err := p.HandleCodeExchangeWithVerifier(ctx, code, verifier)
			if err != nil {
				return nil, err
			}
			return auth, nil
		}
	}
	return nil, apperrors.AuthErrInvalidProvider
}
