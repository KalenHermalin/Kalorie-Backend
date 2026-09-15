"""Port of internal/handlers/user.go

Bug fix applied here (the 8x missing `return`): in Go, every one of these
handlers read the user id out of the request context with a type
assertion (`id, ok := ...; if !ok { WriteError(...) }`) and forgot the
`return` after it, so a missing id fell through into the rest of the
handler anyway. Here the user id comes from `AuthContext`, supplied by
FastAPI's `Depends(require_auth)` *before* the handler body ever runs -
so there's no "assert then maybe-forget-to-return" step left to get
wrong, which is the fix.

Everything else - including the redundant double check in every ingress
handler (`if len(new_logs) == 0: ...` immediately followed by
`if new_logs is None: ...`, which the Go code also did) - is kept as-is.
"""

from __future__ import annotations

import logging
from datetime import datetime
from typing import List, Optional

from fastapi import Request, Response
from pydantic import BaseModel
from starlette.concurrency import run_in_threadpool

from app import apperrors, utils
from app.apperrors import AppError
from app.handlers.common import json_response
from app.middlewares.auth import AuthContext
from app.models.user import FullExerciseLog, FullFoodLog, UserSettings, WeightLog
from app.service.user_service import UserService

log = logging.getLogger(__name__)


def _parse_last_synced_at(request: Request) -> Optional[datetime]:
    last_synced = request.query_params.get("last_synced_at", "")
    since_time: Optional[datetime] = None
    if last_synced != "":
        try:
            since_time = datetime.fromisoformat(last_synced.replace("Z", "+00:00"))
        except ValueError:
            pass
    return since_time


class _EsyncSettingsBody(BaseModel):
    settings: UserSettings = UserSettings()


class _SyncWeightLogsRequestBody(BaseModel):
    weight_logs: List[WeightLog] = []


class _SyncExerciseLogsRequestBody(BaseModel):
    exercise_logs: List[FullExerciseLog] = []


class _SyncFoodLogsRequestBody(BaseModel):
    food_logs: List[FullFoodLog] = []


class UserHandler:
    def __init__(self, user_service: UserService):
        self.user = user_service

    # --- Settings ------------------------------------------------------------

    async def egress_sync_settings(self, request: Request, auth_ctx: AuthContext) -> Response:
        user_id = auth_ctx.user_id

        body = await request.body()
        try:
            request_data = utils.decode_payload(body, _EsyncSettingsBody)
        except ValueError as e:
            log.error(f"Error: decoding refresh request body error={e}")
            raise apperrors.ErrBadRequestBody

        new_settings, err = await run_in_threadpool(self.user.update_user_settings, user_id, request_data.settings)
        if err is not None:
            if err is apperrors.ErrSyncConflict:
                return json_response(new_settings, status_code=409)
            if isinstance(err, AppError):
                raise err
            log.error(f"Error: Updating Settings Failed error={err}")
            raise apperrors.ErrInternalServer

        return json_response({"status": "success"})

    async def ingress_sync_settings(self, request: Request, auth_ctx: AuthContext) -> Response:
        user_id = auth_ctx.user_id

        since_time = _parse_last_synced_at(request)
        if since_time is None:
            raise apperrors.new_app_error("ERR_MISSING_QUERY", "Missing the 'last_synced_at' query", 400)

        new_settings, err = await run_in_threadpool(self.user.get_user_settings, user_id, since_time)
        if err is not None:
            if isinstance(err, AppError):
                raise err
            log.error(f"Error: Updating Settings Failed error={err}")
            raise apperrors.ErrInternalServer

        return json_response(new_settings)

    # --- Weight logs -----------------------------------------------------------

    async def egress_sync_weight_logs(self, request: Request, auth_ctx: AuthContext) -> Response:
        user_id = auth_ctx.user_id

        body = await request.body()
        try:
            request_data = utils.decode_payload(body, _SyncWeightLogsRequestBody)
        except ValueError as e:
            log.error(f"Error: decoding refresh request body error={e}")
            raise apperrors.ErrBadRequestBody

        successful, failed, err = await run_in_threadpool(self.user.upsert_user_weight_logs, user_id, request_data.weight_logs)
        if err is not None:
            if err is apperrors.ErrSoftWeightLog:
                return json_response({"new_logs": successful, "failed_logs": failed, "error": err.to_dict()})
            if isinstance(err, AppError):
                raise err
            raise apperrors.ErrInternalServer

        return json_response({"new_logs": successful, "failed_logs": None, "error": None})

    async def ingress_sync_weight_logs(self, request: Request, auth_ctx: AuthContext) -> Response:
        user_id = auth_ctx.user_id

        since_time = _parse_last_synced_at(request)
        if since_time is None:
            raise apperrors.new_app_error("ERR_MISSING_QUERY", "Missing the 'last_synced_at' query", 400)

        weight_logs, err = await run_in_threadpool(self.user.get_user_weight_logs, user_id, since_time)
        if err is not None:
            if isinstance(err, AppError):
                raise err
            log.error(f"Error: Updating Settings Failed error={err}")
            raise apperrors.ErrInternalServer

        if not weight_logs:
            return Response(status_code=304)
        if weight_logs is None:
            return Response(status_code=304)

        return json_response(weight_logs)

    # --- Exercise logs -----------------------------------------------------------

    async def egress_sync_exercise_logs(self, request: Request, auth_ctx: AuthContext) -> Response:
        user_id = auth_ctx.user_id

        body = await request.body()
        try:
            request_data = utils.decode_payload(body, _SyncExerciseLogsRequestBody)
        except ValueError as e:
            log.error(f"Error: decoding refresh request body error={e}")
            raise apperrors.ErrBadRequestBody

        successful, failed, err = await run_in_threadpool(
            self.user.upsert_user_exercise_log_and_sets, user_id, request_data.exercise_logs
        )
        if err is not None:
            if err is apperrors.ErrSoftWeightLog:
                return json_response({"new_logs": successful, "failed_logs": failed, "error": err.to_dict()})
            if isinstance(err, AppError):
                raise err
            raise apperrors.ErrInternalServer

        return json_response({"new_logs": successful, "failed_logs": None, "error": None})

    async def ingress_sync_exercise_logs(self, request: Request, auth_ctx: AuthContext) -> Response:
        user_id = auth_ctx.user_id

        since_time = _parse_last_synced_at(request)
        if since_time is None:
            raise apperrors.new_app_error("ERR_MISSING_QUERY", "Missing the 'last_synced_at' query", 400)

        new_logs, err = await run_in_threadpool(self.user.get_user_exercise_logs, user_id, since_time)
        if err is not None:
            if isinstance(err, AppError):
                raise err
            log.error(f"Error: Updating Settings Failed error={err}")
            raise apperrors.ErrInternalServer

        if not new_logs:
            return Response(status_code=304)
        if new_logs is None:
            return Response(status_code=304)

        return json_response(new_logs)

    # --- Food logs -----------------------------------------------------------

    async def egress_sync_food_logs(self, request: Request, auth_ctx: AuthContext) -> Response:
        user_id = auth_ctx.user_id

        body = await request.body()
        try:
            request_data = utils.decode_payload(body, _SyncFoodLogsRequestBody)
        except ValueError as e:
            log.error(f"Error: decoding refresh request body error={e}")
            raise apperrors.ErrBadRequestBody

        successful, failed, err = await run_in_threadpool(
            self.user.upsert_user_food_logs_and_entries, user_id, request_data.food_logs
        )
        if err is not None:
            if err is apperrors.ErrSoftWeightLog:
                return json_response({"new_logs": successful, "failed_logs": failed, "error": err.to_dict()})
            if isinstance(err, AppError):
                raise err
            raise apperrors.ErrInternalServer

        return json_response({"new_logs": successful, "failed_logs": None, "error": None})

    async def ingress_sync_food_logs(self, request: Request, auth_ctx: AuthContext) -> Response:
        user_id = auth_ctx.user_id

        since_time = _parse_last_synced_at(request)
        if since_time is None:
            raise apperrors.new_app_error("ERR_MISSING_QUERY", "Missing the 'last_synced_at' query", 400)

        new_logs, err = await run_in_threadpool(self.user.get_user_food_logs, user_id, since_time)
        if err is not None:
            if isinstance(err, AppError):
                raise err
            log.error(f"Error: Updating Settings Failed error={err}")
            raise apperrors.ErrInternalServer

        if not new_logs:
            return Response(status_code=304)
        if new_logs is None:
            return Response(status_code=304)

        return json_response(new_logs)
