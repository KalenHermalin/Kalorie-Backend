package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"main/internal/apperrors"
	"main/internal/loggerx"
	"main/internal/models"
	"main/internal/store"
	"net/http"
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
					if errors.Is(err, store.ErrNotFound) {
						return apperrors.New(apperrors.SessionOver, err)
					}
					return fmt.Errorf("failed to retrieve settings for user %s: %w", userId, err)
				}
				return nil
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
			if errors.Is(inErr, store.ErrNotFound) {
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

func (us *UserService) UpsertUserExerciseLogAndSets(ctx context.Context, userId string, exerciseLogs []*models.FullExerciseLog) (newer []*models.FullExerciseLog, failed []*models.FullExerciseLog, err error) {
	if exerciseLogs == nil {
		return nil, nil, apperrors.New(apperrors.BadRequest, fmt.Errorf("no exercise logs submitted to upsert"))
	}
	var failedSync []string
	var newerServerLogs []*models.FullExerciseLog
	err = us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var upsertError error
		failedCount := 0
		for _, log := range exerciseLogs {

			err := us.userStore.UpsertUserExerciseLog(ctx, tx, userId, log.ExerciseLog)
			if err != nil {
				// Newer log data exist so we get it
				newLog, err := us.userStore.GetUserExerciseLogById(ctx, tx, userId, log.ExerciseLog.ID)
				if err != nil {
					failedSync = append(failedSync, log.ExerciseLog.ID)

				} else if newLog != nil {
					sets, err := us.userStore.GetUserExerciseSetsByLogId(ctx, tx, newLog.ID)
					if err != nil {

					}
					newFullLog := &models.FullExerciseLog{
						ExerciseLog:  newLog,
						ExerciseSets: sets,
					}
					newerServerLogs = append(newerServerLogs, newFullLog)
				}
			}
			err = us.userStore.UpsertUserExerciseSets(ctx, tx, log.ExerciseSets)
			if err != nil {

				return err
			}
		}
		// TODO: No need for soft error, we should just return the success and failed?
		// COMEBACK:
		if failedCount > 0 && failedCount < len(exerciseLogs) {
			slog.Error("failed to sync logs", "failed_count", failedCount, "total_count", len(exerciseLogs), "error", upsertError)
			return apperrors.ErrSoftWeightLog
		}
		if failedCount == len(exerciseLogs) {
			// Some kind of hard error?
			slog.Error("failed to sync logs all logs", "failed_count", failedCount, "total_count", len(exerciseLogs), "error", upsertError)
			return apperrors.NewAppError("ERR_UPSERTING_WEIGHT_LOG", "There was an internal error updating your weight logs", http.StatusInternalServerError)

		}

		return nil
	})

	if err != nil {
		if errors.Is(err, apperrors.ErrSoftWeightLog) {
			return newerServerLogs, failedUpsertLogs, err
		} else if errors.Is(err, apperrors.ErrInternalServer) {
			return nil, nil, err
		}
		return nil, failedUpsertLogs, err
	}
	return newerServerLogs, nil, nil

}

func (us *UserService) GetUserExerciseLogs(ctx context.Context, user_id string, last_sync *time.Time) ([]*models.FullExerciseLog, error) {

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
		// Already wrapped from store.getuserexerciselogs
		return nil, err
	}
	var not_synced []*models.FullExerciseLog

	for _, log := range exerciseLogs {
		if log.ExerciseLog.UpdatedAt.After(*last_sync) {
			not_synced = append(not_synced, log)
		}
	}
	return not_synced, nil

} //

// TODO: Rewrite function specifically the soft error and hard error at bottom make no sense and are uneeded
func (us *UserService) UpsertUserFoodLogsAndEntries(ctx context.Context, userId string, FoodLogs []*models.FullFoodLog) ([]*models.FullFoodLog, []*models.FullFoodLog, error) {
	if FoodLogs == nil {

		slog.Error("Error: user serivce got nil exercise log slice", "userID", userId)
		return nil, nil, apperrors.ErrInternalServer
	}
	var failedUpsertLogs []*models.FullFoodLog
	var newerServerLogs []*models.FullFoodLog
	err := us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var upsertError error
		failedCount := 0
		for i, log := range FoodLogs {

			err := us.userStore.UpsertUserFoodLog(ctx, tx, userId, log.FoodLog)
			if err != nil {
				// Newer log data exist so we get it
				slog.Error("Error in upsert", "error", err, "log_id", log.FoodLog.ID)
				newLog, err := us.userStore.GetUserFoodLogById(ctx, tx, userId, log.FoodLog.ID)
				if err != nil {
					slog.Error("Upsert did not work, caused get to fail, log is missing in db", "error", err.Error(), "user_id", userId, "log_id", log.FoodLog.ID)
					failedCount++
					upsertError = err
					failedUpsertLogs = append(failedUpsertLogs, log)

				} else if newLog != nil {
					sets, err := us.userStore.GetUserFoodLogEntriesByLogId(ctx, tx, newLog.ID)
					if err != nil {

					}
					newFullLog := &models.FullFoodLog{
						FoodLog:        newLog,
						FoodLogEntries: sets,
					}
					newerServerLogs = append(newerServerLogs, newFullLog)
				}
			}
			err = us.userStore.UpsertUserFoodLogEntry(ctx, tx, log.FoodLogEntries)
			if err != nil {

				slog.Error("Upsert for food log failed, should be impossible", "error", err.Error(), "user_id", userId, "log_id", log.FoodLog.ID)
				failedUpsertLogs = append(failedUpsertLogs, FoodLogs[i:]...)
				return apperrors.ErrInternalServer
			}
		}
		if failedCount > 0 && failedCount < len(FoodLogs) {
			slog.Error("failed to sync logs", "failed_count", failedCount, "total_count", len(FoodLogs), "error", upsertError)
			return apperrors.ErrSoftWeightLog
		}
		if failedCount == len(FoodLogs) {
			// Some kind of hard error?
			slog.Error("failed to sync logs all logs", "failed_count", failedCount, "total_count", len(FoodLogs), "error", upsertError)
			return apperrors.NewAppError("ERR_UPSERTING_WEIGHT_LOG", "There was an internal error updating your weight logs", http.StatusInternalServerError)

		}

		return nil
	})

	if err != nil {
		if errors.Is(err, apperrors.ErrSoftWeightLog) {
			return newerServerLogs, failedUpsertLogs, err
		} else if errors.Is(err, apperrors.ErrInternalServer) {
			return nil, nil, err
		}
		return nil, failedUpsertLogs, err
	}
	return newerServerLogs, nil, nil

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
	var not_synced []*models.FullFoodLog

	for _, log := range foodLogs {
		if log.FoodLog.UpdatedAt.After(*last_sync) {
			not_synced = append(not_synced, log)
		}
	}
	return not_synced, nil

}
