"""Port of cmd/main.go"""

from __future__ import annotations

import json
import logging
import os
import sys
import time
from datetime import datetime, timezone

from app.api import Application, Config
from app.auth.providers.github_provider import GitHubProvider
from app.auth.providers.google_provider import GoogleProvider
from app.db import check_db_connection, new_engine, new_session_factory
from app.handlers.auth_handler import AuthHandler
from app.handlers.llm_handler import LLMHandler
from app.handlers.system_handler import SystemHandler
from app.handlers.user_handler import UserHandler
from app.llm.gemini_provider import GeminiProvider
from app.migrate import run_migrations
from app.service.auth_service import AuthService
from app.service.llm_service import LLMService
from app.service.user_service import UserService
from app.store.postgres_user import PostgresUserStore


class _JsonLogFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        return json.dumps(
            {
                "time": datetime.fromtimestamp(record.created, tz=timezone.utc).isoformat(),
                "level": record.levelname,
                "msg": record.getMessage(),
            }
        )


def _require_env(key: str) -> str:
    val = os.environ.get(key)
    if not val:
        logging.error(f"Error: Missing env variable key={key}")
        sys.exit(1)
    return val


def main() -> None:
    model = _require_env("LLM_MODEL")

    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(_JsonLogFormatter())
    logging.basicConfig(level=logging.INFO, handlers=[handler])

    # Setting Up LLM Provider, Service and Handler
    api_key = _require_env("LLM_API_KEY")
    try:
        gemini = GeminiProvider(api_key, model)
    except Exception as e:
        logging.error(str(e))
        sys.exit(1)
    llm_service = LLMService(gemini)
    llm_handler = LLMHandler(llm_service)

    # Setting up User Store, Service and Handler
    conn_string = _require_env("DB_DNS")
    try:
        engine = new_engine(conn_string)
    except Exception as e:
        logging.error(str(e))
        sys.exit(1)
    check_db_connection(engine)
    run_migrations(engine, "./migrations")
    session_factory = new_session_factory(engine)
    user_store = PostgresUserStore(session_factory)

    git_hub_client_id = _require_env("GIT_CLIENT_ID")
    github_client_secret = _require_env("GIT_CLIENT_SECRET")

    github_auth = GitHubProvider(git_hub_client_id, github_client_secret, "kalorie://", None)

    google_client_id_ios = _require_env("GOOGLE_CLIENT_ID_IOS")
    google_client_id_android = _require_env("GOOGLE_CLIENT_ID_ANDROID")
    google_ios_auth = GoogleProvider(
        google_client_id_ios, "", "com.googleusercontent.apps.725051057596-1jsdj1vob2v5mnrhbhi1moj8bfj12cqs://", "ios", None
    )
    google_android_auth = GoogleProvider(
        google_client_id_android, "", "com.googleusercontent.apps.725051057596-gj4kl9f3c4f10cef513qsgahjppuhoqg://", "android", None
    )

    jwt_access_secret = _require_env("JWT_ACCESS_SECRET")
    jwt_refresh_secret = _require_env("JWT_REFRESH_SECRET")

    auth_service = AuthService(user_store, jwt_access_secret, jwt_refresh_secret, github_auth, google_ios_auth, google_android_auth)
    auth_handler = AuthHandler(auth_service)

    # Setting Up User Handler
    user_service = UserService(user_store)
    user_handler = UserHandler(user_service)
    # Setting up System Handler
    system_handler = SystemHandler()

    addr = _require_env("PORT")

    # Initalizing Application
    app = Application(
        config=Config(addr="0.0.0.0:" + addr, jwt_access_secret=jwt_access_secret, jwt_refresh_secret=jwt_refresh_secret),
        auth_handler=auth_handler,
        llm_handler=llm_handler,
        system_handler=system_handler,
        user_handler=user_handler,
    )
    # Running Application
    mux = app.mount()
    app.run(mux)


if __name__ == "__main__":
    main()


# --- Dead code below, kept for fidelity with the bottom of cmd/main.go -------
# (these types aren't referenced anywhere in the Go version either)

from dataclasses import dataclass, field
from typing import List


@dataclass
class Nutrients:
    cal: int = 0
    carbs: int = 0
    fat: int = 0
    protein: int = 0


@dataclass
class Portion:
    label: str = ""
    weight_grams: int = 0


@dataclass
class _SearchResponse:
    source: str = ""
    id: str = ""
    name: str = ""
    nutrients_per_100g: Nutrients = field(default_factory=Nutrients)
    portions: List[Portion] = field(default_factory=list)
