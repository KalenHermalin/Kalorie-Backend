"""Port of internal/handlers/auth.go

Handlers are `async def` only so the request body can be read the same
way Go's manual `utils.DecodePayload(request.Body, ...)` did (giving full
control over exactly which AppError comes back for a bad body, matching
API.md); the actual service calls are still the same synchronous,
Go-style blocking code, just dispatched with `run_in_threadpool` so one
slow request doesn't stall every other request on the single event loop -
the closest match to Go's goroutine-per-request model.
"""

from __future__ import annotations

import logging
from typing import Optional

from fastapi import Request, Response
from pydantic import BaseModel
from starlette.concurrency import run_in_threadpool

from app import apperrors, utils
from app.apperrors import AppError
from app.handlers.common import json_response
from app.middlewares.auth import AuthContext
from app.service.auth_service import AuthService

log = logging.getLogger(__name__)


class _AuthRequestBody(BaseModel):
    code: str = ""
    provider: str = ""
    verifier: str = ""
    platform: Optional[str] = None


class _LogOutRequestBody(BaseModel):
    refresh: str = ""


class AuthHandler:
    def __init__(self, auth_service: AuthService):
        self.auth = auth_service

    async def handle_refresh(self, request: Request) -> Response:
        body = await request.body()
        try:
            request_data = utils.decode_payload(body, _LogOutRequestBody)
        except ValueError as e:
            log.error(f"Error: decoding refresh request body error={e}")
            raise apperrors.ErrBadRequestBody

        resp, err = await run_in_threadpool(self.auth.refresh_access_token, request_data.refresh)
        if err is not None:
            if isinstance(err, AppError):
                raise err
            log.error(f"Error: Sign Up / Login Failed error={err}")
            raise apperrors.ErrInternalServer

        return json_response(resp)

    async def handle_log_out(self, request: Request, auth_ctx: AuthContext) -> Response:
        # Get userId from auth dependency
        user_id = auth_ctx.user_id

        # Get refresh token from request body
        body = await request.body()
        try:
            request_data = utils.decode_payload(body, _LogOutRequestBody)
        except ValueError as e:
            log.error(f"Error: decoding logout request body error={e}")
            raise apperrors.ErrInvalidRequest

        err = await run_in_threadpool(self.auth.log_out, user_id, request_data.refresh)
        if err is not None:
            log.error(f"Error: deleting refresh token in database error={err}")
            if isinstance(err, AppError):
                raise err
            log.error(f"logout had an error error={err}")
            raise apperrors.new_app_error("ERR_LOGOUT_FAILED", "Unkown logout error occured. Please try again in a few", 500)

        return Response(content="Success", status_code=200)

    async def handle_login_signup(self, request: Request) -> Response:
        body = await request.body()
        try:
            request_data = utils.decode_payload(body, _AuthRequestBody)
        except ValueError as e:
            # Bad Request because all we did was decode it and got an error meaning invalid JSON
            log.error(f"Error: decoding login request body error={e}")
            raise apperrors.ErrInvalidRequest

        try:
            utils.check_valid_string(request_data.code)
        except ValueError as e:
            log.error(f"Error: code is empty error={e}")
            raise apperrors.ErrInvalidRequest
        try:
            utils.check_valid_string(request_data.provider)
        except ValueError as e:
            log.error(f"Error: provider is empty error={e}")
            raise apperrors.ErrInvalidRequest

        resp, err = await run_in_threadpool(
            self.auth.sign_in, request_data.code, request_data.provider, request_data.verifier, request_data.platform
        )
        if err is not None:
            if isinstance(err, AppError):
                raise err
            log.error(f"Error: Sign Up / Login Failed error={err}")
            raise apperrors.ErrInternalServer

        return json_response(resp)

