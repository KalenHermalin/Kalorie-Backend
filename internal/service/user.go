package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"main/internal/apperrors"
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

func (us *UserService) UpsertUserWeightLogs(ctx context.Context, userId string, weightLogs []*models.WeightLog) (new_server_logs []*models.WeightLog, failed []*models.WeightLog, err error) {
	if weightLogs == nil {

		slog.Error("Error: user serivce got nil weight log slice", "error", "userID", userId)
		return nil, nil, apperrors.ErrInternalServer
	}
	recentServerLogs := []*models.WeightLog{}
	failedUpsertLogs := []*models.WeightLog{}
	us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var upsertError error
		failedCount := 0
		for _, log := range weightLogs {
			err := us.userStore.UpsertUserWeightLog(ctx, tx, userId, log)
			if err != nil {
				// Newer log data exist so we get it
				newlog, err := us.userStore.GetUserWeightLogById(ctx, tx, userId, log.ID)
				if err != nil {
					slog.Error("Upsert did not work, caused get to fail, log is missing in db", "error", err.Error(), "user_id", userId, "log_id", log.ID)
					failedCount++
					upsertError = err
					failedUpsertLogs = append(failedUpsertLogs, log)
				} else if newlog != nil {
					recentServerLogs = append(recentServerLogs, newlog)
				}
			}
		}
		if failedCount > 0 && failedCount < len(weightLogs) {
			slog.Error("failed to sync logs", "failed_count", failedCount, "total_count", len(weightLogs), "error", upsertError)
			// Some kind of error of something soft error
			return apperrors.ErrSoftWeightLog
		}
		if failedCount == len(weightLogs) {
			// Some kind of hard error?
			slog.Error("failed to sync logs all logs", "failed_count", failedCount, "total_count", len(weightLogs), "error", upsertError)
			return apperrors.NewAppError("ERR_UPSERTING_WEIGHT_LOG", "There was an internal error updating your weight logs", http.StatusInternalServerError)

		}

		return nil
	})

	if err != nil {
		if errors.Is(err, apperrors.ErrSoftWeightLog) {
			return recentServerLogs, failedUpsertLogs, err
		}
		return nil, failedUpsertLogs, err
	}
	return recentServerLogs, nil, nil

}

func (us *UserService) GetUserWeightLogs(ctx context.Context, userId string, last_synced *time.Time) ([]*models.WeightLog, error) {

	if last_synced == nil {
		return nil, apperrors.ErrInternalServer
	}

	var weightLogs []*models.WeightLog
	err := us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		weightLogs, err = us.userStore.GetUserWeightLogs(ctx, tx, userId)
		return err
	})
	if err != nil {
		slog.Error("Error: Getting User Weight Logs", "error", err.Error(), "userID", userId)
		if errors.Is(err, store.ErrNoRowsAffected) {
			return nil, apperrors.ErrUserSettingsMissing
		}

		return nil, apperrors.ErrInternalServer
	}
	var not_synced []*models.WeightLog
	for _, log := range weightLogs {
		if log.UpdatedAt.After(*last_synced) {
			not_synced = append(not_synced, log)
		}

	}
	return not_synced, nil

}

func (us *UserService) UpsertUserExerciseLogAndSets(ctx context.Context, userId string, exerciseLogs []*models.FullExerciseLog) ([]*models.FullExerciseLog, []*models.FullExerciseLog, error) {
	if exerciseLogs == nil {

		slog.Error("Error: user serivce got nil exercise log slice", "error", "userID", userId)
		return nil, nil, apperrors.ErrInternalServer
	}
	var failedUpsertLogs []*models.FullExerciseLog
	var newerServerLogs []*models.FullExerciseLog
	err := us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var upsertError error
		failedCount := 0
		for i, log := range exerciseLogs {

			err := us.userStore.UpsertUserExerciseLog(ctx, tx, userId, log.ExerciseLog)
			if err != nil {
				// Newer log data exist so we get it
				newLog, err := us.userStore.GetUserExerciseLogById(ctx, tx, userId, log.ExerciseLog.ID)
				if err != nil {
					slog.Error("Upsert did not work, caused get to fail, log is missing in db", "error", err.Error(), "user_id", userId, "log_id", log.ExerciseLog.ID)
					failedCount++
					upsertError = err
					failedUpsertLogs = append(failedUpsertLogs, log)

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

				slog.Error("Upsert for exercise set failed, should be impossible", "error", err.Error(), "user_id", userId, "log_id", log.ExerciseLog.ID)
				failedUpsertLogs = append(failedUpsertLogs, exerciseLogs[i:]...)
				return apperrors.ErrInternalServer
			}
		}
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
	if last_sync == nil {

		return nil, apperrors.ErrInternalServer
	}
	var exerciseLogs []*models.FullExerciseLog

	err := us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		exerciseLogs, err = us.userStore.GetUserExerciseLogs(ctx, tx, user_id)
		if err != nil {
			slog.Error("Internal error occured when getting exercise logs", "error", err.Error(), "user_id", user_id, "last_sync", last_sync)
			return apperrors.ErrInternalServer
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var not_synced []*models.FullExerciseLog

	for _, log := range exerciseLogs {
		if log.ExerciseLog.UpdatedAt.After(*last_sync) {
			not_synced = append(not_synced, log)
		}
	}
	return not_synced, nil

}
func (us *UserService) UpsertUserFoodLogsAndEntries(ctx context.Context, userId string, FoodLogs []*models.FullFoodLog) ([]*models.FullFoodLog, []*models.FullFoodLog, error) {
	if FoodLogs == nil {

		slog.Error("Error: user serivce got nil exercise log slice", "error", "userID", userId)
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
	if last_sync == nil {

		return nil, apperrors.ErrInternalServer
	}
	var foodLogs []*models.FullFoodLog

	err := us.userStore.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		foodLogs, err = us.userStore.GetUserFoodLogs(ctx, tx, user_id)
		if err != nil {
			slog.Error("Internal error occured when getting food logs", "error", err.Error(), "user_id", user_id, "last_sync", last_sync)
			return apperrors.ErrInternalServer
		}
		return nil
	})
	if err != nil {
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
