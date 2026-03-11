package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"main/internal/apperrors"
	"main/internal/middlewares"
	"main/internal/models"
	"main/internal/store"
)

type UserService struct {
	userStore models.UserRepository
}

func NewUserService(store models.UserRepository) *UserService {
	return &UserService{userStore: store}
}

func (us *UserService) UpdateUserSettings(ctx context.Context, settings *models.UserSettings) error {

	id, ok := ctx.Value(middlewares.UserIDKey).(string)
	if !ok {
		return apperrors.ErrMissingUserIdInContext
	}
	err := us.userStore.WithTx(ctx, func(tx *sql.Tx) error {

		return us.userStore.UpdateUserSettings(ctx, tx, id, settings)
	})
	if err != nil {
		slog.Error("Error: Updating User Settings", "error", err.Error(), "userID", id)
		if errors.Is(err, store.ErrNoRowsAffected) {
			return apperrors.ErrUserSettingsMissing
		}

		return apperrors.ErrInternalServer
	}
	return nil

}
