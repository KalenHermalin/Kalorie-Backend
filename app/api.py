"""Port of cmd/api.go

`mount()` plays the same role as Go's `(app *application) mount() http.Handler`:
build the router, attach the same middleware chain (request id / logging,
"real ip" is a no-op here since nothing in the app ever reads the client
IP, panic recovery, a 1-minute request timeout), and register the same
routes with the same auth requirements. `run()` mirrors
`(app *application) run(mux)`.
"""

from __future__ import annotations

import logging
import time
import uuid
from dataclasses import dataclass

import uvicorn
from fastapi import Depends, FastAPI, Request
from fastapi.responses import JSONResponse
from starlette.middleware.base import BaseHTTPMiddleware

from app.apperrors import AppError, ErrInternalServer
from app.handlers.auth_handler import AuthHandler
from app.handlers.llm_handler import LLMHandler
from app.handlers.system_handler import SystemHandler
from app.handlers.user_handler import UserHandler
from app.middlewares.auth import AuthContext, auth as auth_dependency_factory

log = logging.getLogger(__name__)


@dataclass
class Config:
    addr: str
    jwt_access_secret: str
    jwt_refresh_secret: str


@dataclass
class Application:
    config: Config
    auth_handler: AuthHandler
    llm_handler: LLMHandler
    system_handler: SystemHandler
    user_handler: UserHandler

    def mount(self) -> FastAPI:
        app = FastAPI()

        # middleware.RequestID + middleware.Logger
        @app.middleware("http")
        async def request_id_and_logging(request: Request, call_next):
            request_id = str(uuid.uuid4())
            start = time.monotonic()
            response = await call_next(request)
            duration_ms = (time.monotonic() - start) * 1000
            log.info(f"request_id={request_id} method={request.method} path={request.url.path} status={response.status_code} duration_ms={duration_ms:.1f}")
            response.headers["X-Request-Id"] = request_id
            return response

        # middleware.Timeout(time.Minute)
        @app.middleware("http")
        async def timeout(request: Request, call_next):
            import asyncio

            try:
                return await asyncio.wait_for(call_next(request), timeout=60)
            except asyncio.TimeoutError:
                return JSONResponse(status_code=504, content={"code": "ERR_TIMEOUT", "message": "Request timed out"})

        # middleware.Recoverer
        @app.exception_handler(Exception)
        async def recoverer(request: Request, exc: Exception):
            log.error(f"panic recovered: {exc}")
            return JSONResponse(status_code=ErrInternalServer.status, content=ErrInternalServer.to_dict())

        @app.exception_handler(AppError)
        async def handle_app_error(request: Request, exc: AppError):
            return JSONResponse(status_code=exc.status, content=exc.to_dict())

        require_auth = auth_dependency_factory(self.config.jwt_access_secret)

        app.add_api_route("/auth/login", self.auth_handler.handle_login_signup, methods=["POST"])
        app.add_api_route("/auth/refresh", self.auth_handler.handle_refresh, methods=["POST"])
        app.add_api_route("/system/health", self.system_handler.health_handler, methods=["GET"])

        async def logout_route(request: Request, auth_ctx: AuthContext = Depends(require_auth)):
            return await self.auth_handler.handle_log_out(request, auth_ctx)

        app.add_api_route("/auth/logout", logout_route, methods=["POST"])

        async def egress_settings_route(request: Request, auth_ctx: AuthContext = Depends(require_auth)):
            return await self.user_handler.egress_sync_settings(request, auth_ctx)

        async def ingress_settings_route(request: Request, auth_ctx: AuthContext = Depends(require_auth)):
            return await self.user_handler.ingress_sync_settings(request, auth_ctx)

        async def egress_weight_logs_route(request: Request, auth_ctx: AuthContext = Depends(require_auth)):
            return await self.user_handler.egress_sync_weight_logs(request, auth_ctx)

        async def ingress_weight_logs_route(request: Request, auth_ctx: AuthContext = Depends(require_auth)):
            return await self.user_handler.ingress_sync_weight_logs(request, auth_ctx)

        async def egress_food_logs_route(request: Request, auth_ctx: AuthContext = Depends(require_auth)):
            return await self.user_handler.egress_sync_food_logs(request, auth_ctx)

        async def ingress_food_logs_route(request: Request, auth_ctx: AuthContext = Depends(require_auth)):
            return await self.user_handler.ingress_sync_food_logs(request, auth_ctx)

        async def egress_exercise_logs_route(request: Request, auth_ctx: AuthContext = Depends(require_auth)):
            return await self.user_handler.egress_sync_exercise_logs(request, auth_ctx)

        async def ingress_exercise_logs_route(request: Request, auth_ctx: AuthContext = Depends(require_auth)):
            return await self.user_handler.ingress_sync_exercise_logs(request, auth_ctx)

        async def analyze_food_route(request: Request, auth_ctx: AuthContext = Depends(require_auth)):
            return await self.llm_handler.analyze_food_handler(request, auth_ctx)

        async def analyze_label_route(request: Request, auth_ctx: AuthContext = Depends(require_auth)):
            return await self.llm_handler.analyze_label_handler(request, auth_ctx)

        app.add_api_route("/v1/settings/sync", egress_settings_route, methods=["PUT"])
        app.add_api_route("/v1/settings/sync", ingress_settings_route, methods=["GET"])

        app.add_api_route("/v1/weight-logs/sync", egress_weight_logs_route, methods=["PUT"])
        app.add_api_route("/v1/weight-logs/sync", ingress_weight_logs_route, methods=["GET"])

        app.add_api_route("/v1/food-logs/sync", egress_food_logs_route, methods=["PUT"])
        app.add_api_route("/v1/food-logs/sync", ingress_food_logs_route, methods=["GET"])

        app.add_api_route("/v1/exercise-logs/sync", egress_exercise_logs_route, methods=["PUT"])
        app.add_api_route("/v1/exercise-logs/sync", ingress_exercise_logs_route, methods=["GET"])

        app.add_api_route("/v1/analyze/food", analyze_food_route, methods=["POST"])
        app.add_api_route("/v1/analyze/label", analyze_label_route, methods=["POST"])

        return app

    def run(self, app: FastAPI) -> None:
        host, _, port = self.config.addr.rpartition(":")
        print(f"Server has started and is listening at {self.config.addr}")
        uvicorn.run(app, host=host or "0.0.0.0", port=int(port))
