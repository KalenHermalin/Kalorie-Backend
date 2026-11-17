package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"main/internal/apperrors"
	"main/internal/models"
	"main/internal/store"
	"time"

	"firebase.google.com/go/v4/auth"
)

type AuthService struct {
	us     models.AuthRepository
	client *auth.Client
}

func NewAuthService(ur models.AuthRepository, authClient *auth.Client) *AuthService {
	return &AuthService{us: ur, client: authClient}
}

func (as *AuthService) DeleteUser(ctx context.Context, userId string) error {
	err := as.us.WithTx(ctx, func(tx *sql.Tx) error {
		id, err := as.UUIDToFirebase(ctx, userId)
		if err != nil {
			return err
		}
		deleteError := as.us.DeleteUser(ctx, tx, userId)
		if deleteError != nil {
			return deleteError
		}
		err = as.client.DeleteUser(ctx, id)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, store.ErrNoRowsAffected) {
			return nil
		}
		return fmt.Errorf("delete user %s: %w", userId, err)
	}
	return nil
}

func (as *AuthService) FirebaseToUUID(ctx context.Context, firebaseID string) (string, error) {
	id, err := as.us.LookupFirebaseIdToUserId(ctx, firebaseID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", apperrors.New(apperrors.SessionOver, err)

		}
		return "", err
	}
	return id, nil
}

func (as *AuthService) UUIDToFirebase(ctx context.Context, UUID string) (string, error) {
	id, err := as.us.LookupUserIdToFirebase(ctx, UUID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", apperrors.New(apperrors.SessionOver, err)

		}
		return "", err
	}
	return id, nil
}
func (as *AuthService) SignIn(ctx context.Context, firebaseToken string) (*models.AuthResponse, error) {
	ctx2, cancel := context.WithTimeout(ctx, time.Second*2)
	token, err := as.client.VerifyIDToken(ctx2, firebaseToken)
	if err != nil {
		cancel()
		return nil, apperrors.New(apperrors.InvalidToken, err)
	}
	cancel()
	email, _ := token.Claims["email"].(string)
	email_verified, ok := token.Claims["email_verified"].(bool)
	payload := &models.AuthPayload{
		ID:    token.UID,
		Email: nil,
	}
	if ok && email_verified {
		payload.Email = &email
	}

	resp := &models.AuthResponse{}
	ctx, cancel = context.WithTimeout(ctx, time.Second*4)
	defer cancel()
	err = as.us.WithTx(ctx, func(tx *sql.Tx) error {

		user, err := as.us.UpsertUserWithAuth(ctx, tx, payload)
		if err != nil {
			// Returns wrapped errors
			return err

		}

		resp.UserId = user.ID

		return nil
	})
	if err != nil {
		// this is one of the errors from above in the transaction so no need to wrap

		return nil, err
	}
	return resp, nil

}
