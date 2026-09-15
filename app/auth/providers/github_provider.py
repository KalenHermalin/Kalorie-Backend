"""Port of internal/auth/providers/github.go"""

from __future__ import annotations

import logging
from typing import List, Optional

from authlib.integrations.requests_client import OAuth2Session

from app import apperrors
from app.auth.tokens import exchange_code
from app.models.auth import AuthPayload, AuthProvider

log = logging.getLogger(__name__)

_GITHUB_TOKEN_URL = "https://github.com/login/oauth/access_token"


class GitHubProvider(AuthProvider):
    def __init__(self, client_id: str, client_secret: str, callback: str, scopes: Optional[List[str]] = None):
        if scopes is None:
            scopes = ["read:user", "user:email"]
        self._client_id = client_id
        self._client_secret = client_secret
        self._callback = callback
        self._scopes = scopes

    def handle_code_exchange_with_verifier(self, code: str, verifier: str) -> AuthPayload:
        # Only ever returns an apperror, can be instantly returned to handler
        session = OAuth2Session(
            self._client_id,
            self._client_secret,
            redirect_uri=self._callback,
            scope=" ".join(self._scopes),
            token_endpoint_auth_method="client_secret_post",
        )
        exchange_code(session, _GITHUB_TOKEN_URL, code, verifier)

        # Get user ID
        try:
            resp = session.get("https://api.github.com/user")
        except Exception as e:
            log.error(f"Error: oauth2 user info provider={self.get_provider_name()} error={e}")
            raise

        try:
            git_payload = resp.json()
        except Exception as e:
            log.error(f"Error: decoding user data error={e}")
            raise

        # Getting Email
        try:
            resp = session.get("https://api.github.com/user/emails")
        except Exception as e:
            log.error(f"Error: oauth2 user email info provider={self.get_provider_name()} error={e}")
            raise
        if resp.status_code != 200:
            log.error(
                f"Error: oauth provider api error status=provider={self.get_provider_name()} "
                f"{resp.status_code} message={resp.text}"
            )
            raise apperrors.AuthErrUnavailableService

        try:
            emails = resp.json()
        except Exception as e:
            log.error(f"Error: extracting oauth2 user email info provider={self.get_provider_name()} error={e}")
            raise

        # 2. Find the primary one
        primary_email = ""
        for e in emails:
            if e.get("primary") and e.get("verified"):
                primary_email = e.get("email", "")
                break

        return AuthPayload(
            email=primary_email,
            provider=self.get_provider_name(),
            id=str(git_payload.get("id", "")),
        )

    def get_provider_name(self) -> str:
        return "github"

    def get_platform(self) -> str:
        return ""
