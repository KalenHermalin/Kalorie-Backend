package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"main/internal/apperrors"
	"main/internal/auth"
	"main/internal/models"
	"main/internal/store"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

func (as *AuthService) LogOut(ctx context.Context, userId, token string) error {

	err := as.us.WithTx(ctx, func(tx *sql.Tx) error {
		if err := as.us.DeleteRefreshToken(ctx, tx, token, userId); err != nil {
			if errors.Is(err, store.ErrNoRowsAffected) {
				return nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("logout for user %s: %w", userId, err)
	}
	return nil
}

func (as *AuthService) DeleteUser(ctx context.Context, userId string) error {
	err := as.us.WithTx(ctx, func(tx *sql.Tx) error {
		deleteError := as.us.DeleteUser(ctx, tx, userId)
		return deleteError

	})

	if err != nil {
		if errors.Is(err, store.ErrNoRowsAffected) {
			return nil
		}
		return fmt.Errorf("delete user %s: %w", userId, err)
	}
	return nil
}
func (as *AuthService) RefreshAccessToken(ctx context.Context, refresh string) (*models.AuthResponse, error) {

	// 1. Parse refreshToken string to get claims (and the UserID)
	var claims models.CustomClaimsRefresh
	token, err := jwt.ParseWithClaims(refresh, &claims, func(token *jwt.Token) (any, error) {
		return []byte(as.jwtRefreshSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, apperrors.New(apperrors.SessionOver, err)
	}

	// 2. Start Transaction
	resp := &models.AuthResponse{}
	err = as.us.WithTx(ctx, func(tx *sql.Tx) error {

		// 3. Verify token exists in DB for that UserID
		user, err := as.us.FindRefreshToken(ctx, tx, refresh)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				nErr := fmt.Errorf("for user %s: %w", claims.UserID, err)
				return apperrors.New(apperrors.SessionOver, nErr)
			}
			return fmt.Errorf("refresh access token find token for user %s: %w", claims.UserID, err)
		}
		// 4. Delete old token

		err = as.us.DeleteRefreshToken(ctx, tx, refresh, claims.UserID)
		if err != nil {
			if errors.Is(err, store.ErrNoRowsAffected) {
				nErr := fmt.Errorf("for user %s: %w", claims.UserID, err)
				return apperrors.New(apperrors.SessionOver, nErr)
			}
			return fmt.Errorf("refresh access token delete token for user %s: %w", claims.UserID, err)
		}
		// 5. Generate new tokens
		accessToken, err := auth.GenerateAccessToken(user.ID, user.Email, false, as.jwtAccessSecret)
		if err != nil {
			return fmt.Errorf("refresh access token generate access for user %s: %w", claims.UserID, err)
		}
		refreshToken, err := auth.GenerateRefreshToken(user.ID, time.Now().Add(time.Hour*24*30), as.jwtRefreshSecret)
		if err != nil {
			return fmt.Errorf("refresh access token generate refresh for user %s: %w", claims.UserID, err)

		}
		err = as.us.SaveRefreshToken(ctx, tx, refreshToken, user.ID, time.Now().Add(time.Hour*24*30))
		if err != nil {
			return fmt.Errorf("refresh access token saving refresh for user %s: %w", claims.UserID, err)
		}
		resp.AccessToken = accessToken
		resp.RefreshToken = refreshToken
		resp.ExpiresIn = 900
		resp.User = user

		return nil
	})
	if err != nil {
		// this is one of the errors from above in the transaction so no need to wrap
		return nil, err
	}
	return resp, nil
}
func (as *AuthService) SignIn(ctx context.Context, code, provider, verifier string, platform *string) (*models.AuthResponse, error) {
	// 1. Exchange the code for the AuthPayload (Email, ID, etc.)

	payload, err := as.ExchangeCode(ctx, code, provider, verifier, platform)
	if err != nil {
		// Error is appError or generic
		return nil, err
	}
	resp := &models.AuthResponse{}
	err = as.us.WithTx(ctx, func(tx *sql.Tx) error {

		user, err := as.us.UpsertUserWithAuth(ctx, tx, payload)
		if err != nil {
			// Returns wrapped errors
			return err

		}

		// 3. Generate the stateless JWT for the mobile app
		accessToken, err := auth.GenerateAccessToken(user.ID, user.Email, false, as.jwtAccessSecret)
		if err != nil {
			return fmt.Errorf("sign in generate access for user %s: %w", user.ID, err)

		}
		refresh, err := auth.GenerateRefreshToken(user.ID, time.Now().Add(time.Hour*24*30), as.jwtRefreshSecret)
		if err != nil {
			return fmt.Errorf("sign in generate refresh for user %s: %w", user.ID, err)

		}

		err = as.us.SaveRefreshToken(ctx, tx, refresh, user.ID, time.Now().Add(time.Hour*24*30))
		if err != nil {
			return fmt.Errorf("sign in saving refresh for user %s: %w", user.ID, err)
		}
		resp.AccessToken = accessToken
		resp.RefreshToken = refresh
		resp.ExpiresIn = 900
		resp.User = user

		return nil
	})
	if err != nil {
		// this is one of the errors from above in the transaction so no need to wrap

		return nil, err
	}
	return resp, nil

}

func (as *AuthService) ExchangeCode(ctx context.Context, code, provider, verifier string, platform *string) (*models.AuthPayload, error) {
	if platform == nil {
		for _, p := range as.providers {
			if p.GetProviderName() == provider {
				// Returns KErrors or wrapped so can leave as is
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
			// returns both an appError and generic error
			auth, err := p.HandleCodeExchangeWithVerifier(ctx, code, verifier)
			if err != nil {
				return nil, err
			}
			return auth, nil
		}
	}
	return nil, apperrors.New(apperrors.InvalidProvider, nil)
}
