"""Port of internal/service/user.go

Service methods keep Go's `(value, error)` return shape (as a 2- or
3-tuple, `None` in the error slot on success) rather than switching to
Python's raise/except everywhere - that keeps the handler code (and its
"is this a conflict / soft-failure / app error / fallback 500" branching)
a close, easy-to-diff match for the original Go handlers.

Bug fix applied here: UpsertUserWeightLogs used to discard the result of
`WithTx(...)` entirely (the `err` named return was never assigned), so a
transaction failure or partial failure was silently reported as full
success. Fixed below by actually capturing and handling the exception
`with_tx` raises, the same way UpsertUserExerciseLogAndSets and
UpsertUserFoodLogsAndEntries already did in Go.
"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import List, Optional, Tuple

from app import apperrors
from app.apperrors import AppError
from app.models.user import ExerciseLog, FoodLog, FullExerciseLog, FullFoodLog, UserRepository, UserSettings, WeightLog
from app.store.postgres_user import NoRowsAffected

log = logging.getLogger(__name__)


class UserService:
    def __init__(self, user_store: UserRepository):
        self.user_store = user_store

    def update_user_settings(self, user_id: str, settings: UserSettings) -> Tuple[Optional[UserSettings], Optional[Exception]]:
        new_settings: Optional[UserSettings] = None

        def txn(session):
            nonlocal new_settings
            if settings.updated_at is not None:
                settings.updated_at = settings.updated_at.astimezone(timezone.utc)
            try:
                self.user_store.update_user_settings(session, user_id, settings)
            except NoRowsAffected:
                # If no rows affected, more recent data exists so get it
                try:
                    new_settings = self.user_store.get_user_settings(session, user_id)
                except Exception:
                    # There was a get error, meaning the row really doesnt exist
                    raise apperrors.ErrUserSettingsMissing
                # No rows exist, major error
                raise apperrors.ErrSyncConflict

        try:
            self.user_store.with_tx(txn)
        except AppError as e:
            if e is apperrors.ErrUserSettingsMissing:
                return None, e
            if e is apperrors.ErrSyncConflict:
                return new_settings, e
            log.error(f"Error: Updating User Settings error={e} userID={user_id}")
            return None, apperrors.ErrInternalServer
        except Exception as e:
            log.error(f"Error: Updating User Settings error={e} userID={user_id}")
            return None, apperrors.ErrInternalServer
        # Success
        return None, None

    def get_user_settings(self, user_id: str, last_synced: Optional[datetime]) -> Tuple[Optional[UserSettings], Optional[Exception]]:
        settings: Optional[UserSettings] = None

        def txn(session):
            nonlocal settings
            settings = self.user_store.get_user_settings(session, user_id)

        try:
            self.user_store.with_tx(txn)
        except NoRowsAffected as e:
            log.error(f"Error: Getting User Settings error={e} userID={user_id}")
            return None, apperrors.ErrUserSettingsMissing
        except Exception as e:
            log.error(f"Error: Getting User Settings error={e} userID={user_id}")
            return None, apperrors.ErrInternalServer

        if last_synced is not None and not (settings.updated_at and settings.updated_at > last_synced):
            return None, apperrors.ErrNotModified
        return settings, None

    def upsert_user_weight_logs(
        self, user_id: str, weight_logs: Optional[List[WeightLog]]
    ) -> Tuple[Optional[List[WeightLog]], Optional[List[WeightLog]], Optional[Exception]]:
        if weight_logs is None:
            log.error(f"Error: user serivce got nil weight log slice error= userID={user_id}")
            return None, None, apperrors.ErrInternalServer

        recent_server_logs: List[WeightLog] = []
        failed_upsert_logs: List[WeightLog] = []

        def txn(session):
            upsert_error: Optional[Exception] = None
            failed_count = 0
            for wlog in weight_logs:
                try:
                    self.user_store.upsert_user_weight_log(session, user_id, wlog)
                except Exception:
                    # Newer log data exist so we get it
                    try:
                        new_log = self.user_store.get_user_weight_log_by_id(session, user_id, wlog.id)
                    except Exception as get_err:
                        log.error(
                            f"Upsert did not work, caused get to fail, log is missing in db error={get_err} "
                            f"user_id={user_id} log_id={wlog.id}"
                        )
                        failed_count += 1
                        upsert_error = get_err
                        failed_upsert_logs.append(wlog)
                    else:
                        if new_log is not None:
                            recent_server_logs.append(new_log)

            if 0 < failed_count < len(weight_logs):
                log.error(f"failed to sync logs failed_count={failed_count} total_count={len(weight_logs)} error={upsert_error}")
                raise apperrors.ErrSoftWeightLog
            if failed_count == len(weight_logs):
                log.error(f"failed to sync logs all logs failed_count={failed_count} total_count={len(weight_logs)} error={upsert_error}")
                raise apperrors.new_app_error(
                    "ERR_UPSERTING_WEIGHT_LOG", "There was an internal error updating your weight logs", 500
                )

        try:
            self.user_store.with_tx(txn)
        except AppError as e:
            if e is apperrors.ErrSoftWeightLog:
                return recent_server_logs, failed_upsert_logs, e
            return None, failed_upsert_logs, e
        except Exception as e:
            return None, failed_upsert_logs, e

        return recent_server_logs, None, None

    def get_user_weight_logs(self, user_id: str, last_synced: Optional[datetime]) -> Tuple[Optional[List[WeightLog]], Optional[Exception]]:
        if last_synced is None:
            return None, apperrors.ErrInternalServer

        weight_logs: List[WeightLog] = []

        def txn(session):
            nonlocal weight_logs
            weight_logs = self.user_store.get_user_weight_logs(session, user_id)

        try:
            self.user_store.with_tx(txn)
        except NoRowsAffected as e:
            log.error(f"Error: Getting User Weight Logs error={e} userID={user_id}")
            return None, apperrors.ErrUserSettingsMissing
        except Exception as e:
            log.error(f"Error: Getting User Weight Logs error={e} userID={user_id}")
            return None, apperrors.ErrInternalServer

        not_synced = [wlog for wlog in weight_logs if wlog.updated_at and wlog.updated_at > last_synced]
        return not_synced, None

    def upsert_user_exercise_log_and_sets(
        self, user_id: str, exercise_logs: Optional[List[FullExerciseLog]]
    ) -> Tuple[Optional[List[FullExerciseLog]], Optional[List[FullExerciseLog]], Optional[Exception]]:
        if exercise_logs is None:
            log.error(f"Error: user serivce got nil exercise log slice error= userID={user_id}")
            return None, None, apperrors.ErrInternalServer

        failed_upsert_logs: List[FullExerciseLog] = []
        newer_server_logs: List[FullExerciseLog] = []

        def txn(session):
            upsert_error: Optional[Exception] = None
            failed_count = 0
            for i, flog in enumerate(exercise_logs):
                try:
                    self.user_store.upsert_user_exercise_log(session, user_id, flog.exercise_log)
                except Exception:
                    try:
                        new_log = self.user_store.get_user_exercise_log_by_id(session, user_id, flog.exercise_log.id)
                    except Exception as get_err:
                        log.error(
                            f"Upsert did not work, caused get to fail, log is missing in db error={get_err} "
                            f"user_id={user_id} log_id={flog.exercise_log.id}"
                        )
                        failed_count += 1
                        upsert_error = get_err
                        failed_upsert_logs.append(flog)
                    else:
                        if new_log is not None:
                            try:
                                sets = self.user_store.get_user_exercise_sets_by_log_id(session, new_log.id)
                            except Exception:
                                sets = []
                            newer_server_logs.append(FullExerciseLog(exercise_log=new_log, exercise_sets=sets))

                try:
                    self.user_store.upsert_user_exercise_sets(session, flog.exercise_sets)
                except Exception:
                    log.error(
                        f"Upsert for exercise set failed, should be impossible user_id={user_id} log_id={flog.exercise_log.id}"
                    )
                    failed_upsert_logs.extend(exercise_logs[i:])
                    raise apperrors.ErrInternalServer

            if 0 < failed_count < len(exercise_logs):
                log.error(f"failed to sync logs failed_count={failed_count} total_count={len(exercise_logs)} error={upsert_error}")
                raise apperrors.ErrSoftWeightLog
            if failed_count == len(exercise_logs):
                log.error(f"failed to sync logs all logs failed_count={failed_count} total_count={len(exercise_logs)} error={upsert_error}")
                raise apperrors.new_app_error(
                    "ERR_UPSERTING_WEIGHT_LOG", "There was an internal error updating your weight logs", 500
                )

        try:
            self.user_store.with_tx(txn)
        except AppError as e:
            if e is apperrors.ErrSoftWeightLog:
                return newer_server_logs, failed_upsert_logs, e
            if e is apperrors.ErrInternalServer:
                return None, None, e
            return None, failed_upsert_logs, e
        except Exception as e:
            return None, failed_upsert_logs, e

        return newer_server_logs, None, None

    def get_user_exercise_logs(self, user_id: str, last_sync: Optional[datetime]) -> Tuple[Optional[List[FullExerciseLog]], Optional[Exception]]:
        if last_sync is None:
            return None, apperrors.ErrInternalServer

        exercise_logs: List[FullExerciseLog] = []

        def txn(session):
            nonlocal exercise_logs
            try:
                exercise_logs = self.user_store.get_user_exercise_logs(session, user_id)
            except Exception as e:
                log.error(f"Internal error occured when getting exercise logs error={e} user_id={user_id} last_sync={last_sync}")
                raise apperrors.ErrInternalServer

        try:
            self.user_store.with_tx(txn)
        except Exception as e:
            return None, e

        not_synced = [
            flog for flog in exercise_logs if flog.exercise_log and flog.exercise_log.updated_at and flog.exercise_log.updated_at > last_sync
        ]
        return not_synced, None

    def upsert_user_food_logs_and_entries(
        self, user_id: str, food_logs: Optional[List[FullFoodLog]]
    ) -> Tuple[Optional[List[FullFoodLog]], Optional[List[FullFoodLog]], Optional[Exception]]:
        if food_logs is None:
            log.error(f"Error: user serivce got nil exercise log slice error= userID={user_id}")
            return None, None, apperrors.ErrInternalServer

        failed_upsert_logs: List[FullFoodLog] = []
        newer_server_logs: List[FullFoodLog] = []

        def txn(session):
            upsert_error: Optional[Exception] = None
            failed_count = 0
            for i, flog in enumerate(food_logs):
                try:
                    self.user_store.upsert_user_food_log(session, user_id, flog.food_log)
                except Exception as e:
                    log.error(f"Error in upsert error={e} log_id={flog.food_log.id}")
                    try:
                        new_log = self.user_store.get_user_food_log_by_id(session, user_id, flog.food_log.id)
                    except Exception as get_err:
                        log.error(
                            f"Upsert did not work, caused get to fail, log is missing in db error={get_err} "
                            f"user_id={user_id} log_id={flog.food_log.id}"
                        )
                        failed_count += 1
                        upsert_error = get_err
                        failed_upsert_logs.append(flog)
                    else:
                        if new_log is not None:
                            try:
                                sets = self.user_store.get_user_food_log_entries_by_log_id(session, new_log.id)
                            except Exception:
                                sets = []
                            newer_server_logs.append(FullFoodLog(food_log=new_log, food_log_entries=sets))

                try:
                    self.user_store.upsert_user_food_log_entry(session, flog.food_log_entries)
                except Exception:
                    log.error(f"Upsert for food log failed, should be impossible user_id={user_id} log_id={flog.food_log.id}")
                    failed_upsert_logs.extend(food_logs[i:])
                    raise apperrors.ErrInternalServer

            if 0 < failed_count < len(food_logs):
                log.error(f"failed to sync logs failed_count={failed_count} total_count={len(food_logs)} error={upsert_error}")
                raise apperrors.ErrSoftWeightLog
            if failed_count == len(food_logs):
                log.error(f"failed to sync logs all logs failed_count={failed_count} total_count={len(food_logs)} error={upsert_error}")
                raise apperrors.new_app_error(
                    "ERR_UPSERTING_WEIGHT_LOG", "There was an internal error updating your weight logs", 500
                )

        try:
            self.user_store.with_tx(txn)
        except AppError as e:
            if e is apperrors.ErrSoftWeightLog:
                return newer_server_logs, failed_upsert_logs, e
            if e is apperrors.ErrInternalServer:
                return None, None, e
            return None, failed_upsert_logs, e
        except Exception as e:
            return None, failed_upsert_logs, e

        return newer_server_logs, None, None

    def get_user_food_logs(self, user_id: str, last_sync: Optional[datetime]) -> Tuple[Optional[List[FullFoodLog]], Optional[Exception]]:
        if last_sync is None:
            return None, apperrors.ErrInternalServer

        food_logs: List[FullFoodLog] = []

        def txn(session):
            nonlocal food_logs
            try:
                food_logs = self.user_store.get_user_food_logs(session, user_id)
            except Exception as e:
                log.error(f"Internal error occured when getting food logs error={e} user_id={user_id} last_sync={last_sync}")
                raise apperrors.ErrInternalServer

        try:
            self.user_store.with_tx(txn)
        except Exception as e:
            return None, e

        not_synced = [
            flog for flog in food_logs if flog.food_log and flog.food_log.updated_at and flog.food_log.updated_at > last_sync
        ]
        return not_synced, None
