package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"main/internal/apperrors"
	"main/internal/models"
	"main/internal/store"
	"time"
)

type UserService struct {
	userStore models.UserRepository
}

func NewUserService(store models.UserRepository) *UserService {
	return &UserService{userStore: store}
}

func (us *UserService) UpdateUserSettings(ctx context.Context, userId string, settings *models.UserSettings) (*models.UserSettings, error) {

	var newSettings *models.UserSettings

	err := us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		settings.UpdatedAt = settings.UpdatedAt.UTC().Truncate(time.Microsecond)
		err := us.userStore.UpdateUserSettings(ctx, tx, userId, settings)
		if err != nil {
			if errors.Is(err, store.ErrNoRowsAffected) {
				// If no rows affected, more recent data exists so get it
				newSettings, err = us.userStore.GetUserSettings(ctx, tx, userId)
				if err != nil {
					// There was a get error, meaning the row really doesnt exist
					return apperrors.ErrUserSettingsMissing
				}
				// No rows exist, major error
				return apperrors.ErrSyncConflict
			}
			// Unknown DB error
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, apperrors.ErrUserSettingsMissing) {
			return nil, err
		} else if errors.Is(err, apperrors.ErrSyncConflict) {
			return newSettings, err
		}

		slog.Error("Error: Updating User Settings", "error", err.Error(), "userID", userId)
		return nil, apperrors.ErrInternalServer
	}
	// Success
	return nil, nil

}
func (us *UserService) UpsertUserWeightLogs(ctx context.Context, userId string, weightLogs []*models.WeightLog) error {
	if weightLogs == nil {

		slog.Error("Error: user serivce got nil weight log slice", "error", "userID", userId)
		return apperrors.ErrInternalServer
	}
	recentLogs := []*models.WeightLog{}
	us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var upsertError error
		failedCount := 0
		for _, log := range weightLogs {
			err := us.userStore.UpsertUserWeightLog(ctx, tx, userId, log)
			if err != nil {
				// Newer log data exist so we get it
				log, err := us.userStore.GetUserWeightLogById(ctx, tx, userId, log.ID)
				if err != nil {
					slog.Error("Upsert did not work, so log is missing in db", "error", err.Error(), "user_id", userId, "log_id", log.ID)
					failedCount++
					upsertError = err
				} else if log != nil {
					recentLogs = append(recentLogs, log)
				}
			}
		}
		// TODO: Decide what to do with lgos that failed upsert and get? do we return them in the recent logs, how do we process?
		if failedCount > 0 && failedCount < len(weightLogs) {
			slog.Error("failed to sync logs", "count", failedCount, "error", upsertError)
			// Some kind of error of something soft error
		}

		if failedCount == len(weightLogs) {
			// Some kind of hard error?
		}

		return nil
	})

}
func (us *UserService) GetUserSettings(ctx context.Context, userId string, last_synced *time.Time) (*models.UserSettings, error) {

	var settings *models.UserSettings
	err := us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		settings, err = us.userStore.GetUserSettings(ctx, tx, userId)
		return err
	})
	if err != nil {
		slog.Error("Error: Getting User Settings", "error", err.Error(), "userID", userId)
		if errors.Is(err, store.ErrNoRowsAffected) {
			return nil, apperrors.ErrUserSettingsMissing
		}

		return nil, apperrors.ErrInternalServer
	}

	if last_synced != nil && !settings.UpdatedAt.After(*last_synced) {
		return nil, apperrors.ErrNotModified
	}
	return settings, nil

}
