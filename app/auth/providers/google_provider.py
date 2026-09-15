"""Port of internal/auth/providers/google.go"""

from __future__ import annotations

import logging
from typing import List, Optional

from authlib.integrations.requests_client import OAuth2Session

from app.auth.tokens import exchange_code
from app.models.auth import AuthPayload, AuthProvider

log = logging.getLogger(__name__)

_GOOGLE_TOKEN_URL = "https://oauth2.googleapis.com/token"


class GoogleProvider(AuthProvider):
    def __init__(self, client_id: str, secret: str, callback: str, platform: str, scopes: Optional[List[str]] = None):
        if scopes is None:
            scopes = ["openid", "profile", "email"]
        self._client_id = client_id
        # secret intentionally unused, matching google.go's commented-out
        # `//ClientSecret: secret,` - this is a public (PKCE) client.
        self._callback = callback
        self._scopes = scopes
        self._platform = platform

    def handle_code_exchange_with_verifier(self, code: str, verifier: str) -> AuthPayload:
        # Always returns an apperror so can return right away
        session = OAuth2Session(
            self._client_id,
            redirect_uri=self._callback,
            scope=" ".join(self._scopes),
            token_endpoint_auth_method="none",
        )
        exchange_code(session, _GOOGLE_TOKEN_URL, code, verifier)

        # Get user ID
        try:
            resp = session.get("https://www.googleapis.com/oauth2/v3/userinfo")
        except Exception as e:
            log.error(f"Error: oauth2 user info provider={self.get_provider_name()} error={e}")
            raise

        try:
            google_response = resp.json()
        except Exception as e:
            log.error(f"Error: decoding user data error={e}")
            raise

        return AuthPayload(
            provider=self.get_provider_name(),
            email=google_response.get("email", ""),
            id=google_response.get("sub", ""),
        )

    def get_provider_name(self) -> str:
        return "google"

    def get_platform(self) -> str:
        return self._platform
