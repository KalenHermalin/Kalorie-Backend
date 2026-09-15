"""Port of internal/auth/providers/google.go

Originally ported 1:1 from the Go version, which called Google's
userinfo REST endpoint (an extra authenticated network round-trip) to
get the user's id/email. Google is a full OIDC provider and already
returns a signed `id_token` in the token response, so this now verifies
that instead (via app.auth.oidc, built on Authlib's own JOSE
implementation) - one fewer network call, and verified data instead of
data merely fetched over HTTPS.
"""

from __future__ import annotations

import logging
from typing import List, Optional

from authlib.integrations.requests_client import OAuth2Session

from app import apperrors
from app.auth.oidc import verify_id_token
from app.auth.tokens import exchange_code
from app.models.auth import AuthPayload, AuthProvider

log = logging.getLogger(__name__)

_GOOGLE_TOKEN_URL = "https://oauth2.googleapis.com/token"
_GOOGLE_JWKS_URL = "https://www.googleapis.com/oauth2/v3/certs"
_GOOGLE_ISSUER = "https://accounts.google.com"


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
        session = OAuth2Session(
            self._client_id,
            redirect_uri=self._callback,
            scope=" ".join(self._scopes),
            token_endpoint_auth_method="none",
        )
        token = exchange_code(session, _GOOGLE_TOKEN_URL, code, verifier)

        id_token = token.get("id_token")
        if not id_token:
            log.error("Error: google token response missing id_token")
            raise apperrors.ErrInternalServer

        # Verify the id_token's signature against Google's published JWKS
        # rather than trusting its claims blindly.
        try:
            claims = verify_id_token(id_token, _GOOGLE_JWKS_URL, _GOOGLE_ISSUER, self._client_id)
        except Exception as e:
            log.error(f"Error: verifying google id_token error={e}")
            raise apperrors.ErrInvalidToken

        return AuthPayload(
            provider=self.get_provider_name(),
            email=claims.get("email", ""),
            id=claims.get("sub", ""),
        )

    def get_provider_name(self) -> str:
        return "google"

    def get_platform(self) -> str:
        return self._platform
