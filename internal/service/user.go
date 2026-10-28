package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"main/internal/apperrors"
	"main/internal/loggerx"
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

func (us *UserService) UpdateUserSettings(ctx context.Context, userId string, settings *models.UserSettings) (newSettings *models.UserSettings, isModified bool, err error) {

	err = us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		settings.UpdatedAt = settings.UpdatedAt.UTC().Truncate(time.Microsecond)
		err := us.userStore.UpdateUserSettings(ctx, tx, userId, settings)
		if err != nil {
			if errors.Is(err, store.ErrNoRowsAffected) {
				// If no rows affected, more recent data exists so get it
				newSettings, err = us.userStore.GetUserSettings(ctx, tx, userId)
				if err != nil {
					if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrUserForeignKeyConstraint) {
						return apperrors.New(apperrors.SessionOver, err)
					}
					return fmt.Errorf("failed to retrieve settings for user %s: %w", userId, err)
				}
				return nil
			} else if errors.Is(err, store.ErrUserForeignKeyConstraint) {
				return apperrors.New(apperrors.SessionOver, err)
			}
			// Unknown DB error
			return fmt.Errorf("update user settings for user %s: %w", userId, err)
		}
		return nil
	})
	if err != nil {
		// err is already wrapped or a KError
		return nil, false, err
	}

	// Success
	if newSettings != nil {
		return newSettings, true, nil
	}
	return nil, false, nil

}

func (us *UserService) GetUserSettings(ctx context.Context, userId string, lastSynced *time.Time) (settings *models.UserSettings, isModified bool, err error) {

	err = us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var inErr error
		settings, inErr = us.userStore.GetUserSettings(ctx, tx, userId)
		if inErr != nil {
			if errors.Is(inErr, store.ErrNotFound) || errors.Is(err, store.ErrUserForeignKeyConstraint) {
				return apperrors.New(apperrors.SessionOver, inErr)
			}
			return fmt.Errorf("get user settings for user %s: %w", userId, inErr)
		}
		return nil
	})
	if err != nil {
		// Error is already wrapped or a KError
		return nil, false, err
	}

	if lastSynced != nil && !settings.UpdatedAt.After(*lastSynced) {
		return nil, false, nil
	}
	return settings, true, nil

}

const MAXBATCHSIZE = 200

func (us *UserService) UpsertUserWeightLogs(ctx context.Context, userId string, weightLogs []*models.WeightLog) (newServerLogs []*models.WeightLog, notSyncedLogs []string, err error) {
	if weightLogs == nil {

		return nil, nil, apperrors.New(apperrors.BadRequest, fmt.Errorf("no weight logs to sync"))
	}
	if len(weightLogs) > MAXBATCHSIZE {
		return nil, nil, apperrors.New(apperrors.MaxBatchSizeExceeded, nil)
	}
	newServerLogs = []*models.WeightLog{}
	notSyncedLogs = []string{}

	for _, log := range weightLogs {
		var newerLog *models.WeightLog

		err = us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
			err := us.userStore.UpsertUserWeightLog(ctx, tx, userId, log)
			if err != nil {
				if errors.Is(err, store.ErrNoRowsAffected) {
					var fErr error
					newerLog, fErr = us.userStore.GetUserWeightLogById(ctx, tx, userId, log.ID)
					return fErr
				} else if errors.Is(err, store.ErrUserForeignKeyConstraint) {
					return apperrors.New(apperrors.SessionOver, err)
				}
				return err
			}
			return nil
		})

		if err != nil {
			loggerx.Log(ctx).Warn("log upsert failed", "error", err, "log_id", log.ID)
			notSyncedLogs = append(notSyncedLogs, log.ID)
			continue
		}
		if newerLog != nil {
			newServerLogs = append(newServerLogs, newerLog)
		}

	}
	return newServerLogs, notSyncedLogs, nil

}

func (us *UserService) GetUserWeightLogs(ctx context.Context, userId string, lastSynced *time.Time) (weightLogs []*models.WeightLog, err error) {

	err = us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var gErr error
		weightLogs, gErr = us.userStore.GetUserWeightLogs(ctx, tx, userId)
		return gErr
	})
	if err != nil {
		if errors.Is(err, store.ErrUserForeignKeyConstraint) {
			return nil, apperrors.New(apperrors.SessionOver, err)
		}
		return nil, err
	}
	var notSynced []*models.WeightLog
	for _, log := range weightLogs {
		if lastSynced != nil {
			if log.UpdatedAt.After(*lastSynced) {
				notSynced = append(notSynced, log)
			}
		}

	}
	if notSynced != nil {
		return notSynced, nil
	}
	if lastSynced != nil {
		return nil, nil
	}
	return weightLogs, nil

}

// TODO Need to workout which foriegn key contraint for user or log id on the sets
func (us *UserService) UpsertUserExerciseLogAndSets(ctx context.Context, userId string, exerciseLogs []*models.FullExerciseLog) (newerServerLogs []*models.FullExerciseLog, failed []string, err error) {
	if exerciseLogs == nil {
		return nil, nil, apperrors.New(apperrors.BadRequest, fmt.Errorf("no exercise logs submitted to upsert"))
	}
	newerServerLogs = []*models.FullExerciseLog{}
	failed = []string{}
	for _, log := range exerciseLogs {
		var newerLog *models.FullExerciseLog

		err = us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
			err := us.userStore.UpsertUserExerciseLog(ctx, tx, userId, log)
			if err != nil {
				if errors.Is(err, store.ErrUserForeignKeyConstraint) {
					return err
				} else if errors.Is(err, store.ErrNoRowsAffected) {
					// Newer log data exist so we get it
					var innerErr error
					newerLog, innerErr = us.userStore.GetUserExerciseLogById(ctx, tx, userId, log.ExerciseLog.ID)
					return innerErr
				}
				return err
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, store.ErrUserForeignKeyConstraint) {
				return nil, nil, apperrors.New(apperrors.SessionOver, err)
			}
			loggerx.Log(ctx).Warn("log upsert failed", "error", err, "log_id", log.ExerciseLog.ID)
			failed = append(failed, log.ExerciseLog.ID)
		}
		if newerLog != nil {
			newerServerLogs = append(newerServerLogs, newerLog)
		}

	}

	return newerServerLogs, failed, nil

}

func (us *UserService) GetUserExerciseLogs(ctx context.Context, user_id string, lastSync *time.Time) ([]*models.FullExerciseLog, error) {

	var exerciseLogs []*models.FullExerciseLog

	err := us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		exerciseLogs, err = us.userStore.GetUserExerciseLogs(ctx, tx, user_id)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, store.ErrUserForeignKeyConstraint) {
			return nil, apperrors.New(apperrors.SessionOver, err)
		}
		// Already wrapped from store.getuserexerciselogs
		return nil, err
	}
	var newLogs []*models.FullExerciseLog

	for _, log := range exerciseLogs {
		if lastSync != nil {
			if log.ExerciseLog.UpdatedAt.After(*lastSync) {
				newLogs = append(newLogs, log)
			}
		} else {
			newLogs = append(newLogs, log)
		}
	}
	return newLogs, nil

}

func (us *UserService) UpsertUserFoodLogsAndEntries(ctx context.Context, userId string, FoodLogs []*models.FullFoodLog) (newerServerLogs []*models.FullFoodLog, failed []string, err error) {
	if FoodLogs == nil {
		return nil, nil, apperrors.New(apperrors.BadRequest, fmt.Errorf("no food logs submitted to upsert"))
	}
	newerServerLogs = []*models.FullFoodLog{}
	failed = []string{}

	for _, log := range FoodLogs {
		var newerLog *models.FullFoodLog

		err = us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
			err := us.userStore.UpsertUserFoodLog(ctx, tx, userId, log)
			if err != nil {
				if errors.Is(err, store.ErrNoRowsAffected) {
					var innerErr error
					// Newer log data exist so we get it
					newerLog, innerErr = us.userStore.GetUserFoodLogById(ctx, tx, userId, log.FoodLog.ID)
					return innerErr
				}
				return err
			}
			return nil
		})

		if err != nil {
			loggerx.Log(ctx).Warn("log upsert failed", "error", err, "log_id", log.FoodLog.ID)
			failed = append(failed, log.FoodLog.ID)
			continue
		}
		if newerLog != nil {
			newerServerLogs = append(newerServerLogs, newerLog)
		}

	}
	return newerServerLogs, failed, nil

}

func (us *UserService) GetUserFoodLogs(ctx context.Context, user_id string, last_sync *time.Time) ([]*models.FullFoodLog, error) {
	var foodLogs []*models.FullFoodLog

	err := us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		foodLogs, err = us.userStore.GetUserFoodLogs(ctx, tx, user_id)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		// Already a wrapped error
		return nil, err
	}
	var newLogs []*models.FullFoodLog

	for _, log := range foodLogs {
		if last_sync != nil {
			if log.FoodLog.UpdatedAt.After(*last_sync) {
				newLogs = append(newLogs, log)
			}
		} else {
			newLogs = append(newLogs, log)
		}
	}
	return newLogs, nil

}
