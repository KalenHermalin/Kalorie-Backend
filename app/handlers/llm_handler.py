"""Port of internal/handlers/llm.go

Bug fix applied here: Go built a 15-second `context.WithTimeout` here and
passed it into AnalyzePicture/AnalyzeLabel, but the Gemini provider
ignored it entirely. The timeout is still built here, exactly the same
way, but now it's actually honoured (see app/llm/gemini_provider.py).
"""

from __future__ import annotations

import logging

from fastapi import Request, Response
from pydantic import Base64Bytes, BaseModel
from starlette.concurrency import run_in_threadpool

from app import apperrors, utils
from app.apperrors import AppError
from app.handlers.common import json_response
from app.middlewares.auth import AuthContext
from app.service.llm_service import LLMService

log = logging.getLogger(__name__)

_ANALYZE_TIMEOUT_SECONDS = 15.0


class _AnalyzeRequestPayload(BaseModel):
    picture: Base64Bytes = b""


class LLMHandler:
    def __init__(self, llm_service: LLMService):
        self.llm_service = llm_service

    async def analyze_food_handler(self, request: Request, auth_ctx: AuthContext) -> Response:
        body = await request.body()
        try:
            request_data = utils.decode_payload(body, _AnalyzeRequestPayload)
        except ValueError as e:
            log.error(f"Error: decoding analyze food body error={e}")
            raise apperrors.ErrInvalidRequest

        try:
            payload = await run_in_threadpool(
                self.llm_service.provider.analyze_picture,
                request_data.picture,
                utils.ANALYZEFOODSYSTEMPROMPT,
                _ANALYZE_TIMEOUT_SECONDS,
            )
        except AppError:
            raise
        except Exception as e:
            log.error(f"Error: Analyze Food error={e}")
            raise apperrors.ErrInternalServer

        return json_response(payload)

    async def analyze_label_handler(self, request: Request, auth_ctx: AuthContext) -> Response:
        body = await request.body()
        try:
            request_data = utils.decode_payload(body, _AnalyzeRequestPayload)
        except ValueError as e:
            log.error(f"Error: decoding analyze label body error={e}")
            raise apperrors.ErrInvalidRequest

        try:
            payload = await run_in_threadpool(
                self.llm_service.provider.analyze_label,
                request_data.picture,
                utils.ANALYZELABELSYSTEMPROMPT,
                _ANALYZE_TIMEOUT_SECONDS,
            )
        except AppError:
            raise
        except Exception as e:
            log.error(f"Error: Analyze label error={e}")
            raise apperrors.ErrInternalServer

        return json_response(payload)
