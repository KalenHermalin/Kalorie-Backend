"""Apple ("Sign in with Apple") auth provider.

There's no Go equivalent to port - the `provider_identities` table's
`CHECK` constraint already allowed 'apple' (see migrations/20260204000511_provider_identities.sql)
but no provider was ever implemented for it. This follows the same shape
as github_provider.py / google_provider.py so it plugs into AuthService
the same way.

Apple's OAuth flow differs from GitHub/Google in two ways this file has
to account for:
  1. There's no static client secret - instead you sign a short-lived
     ES256 JWT with your Apple-issued private key on every request and
     send that as the client_secret.
  2. There's no userinfo endpoint. The user's id (`sub`) and email come
     from the `id_token` (a JWT) in the token response, which is verified
     against Apple's published JWKS (via app.auth.oidc, built on
     Authlib's own JOSE implementation) rather than trusted blindly.
"""

from __future__ import annotations

import logging
import time
from typing import List, Optional

import jwt as pyjwt
from authlib.integrations.requests_client import OAuth2Session

from app import apperrors
from app.auth.oidc import verify_id_token
from app.auth.tokens import exchange_code
from app.models.auth import AuthPayload, AuthProvider

log = logging.getLogger(__name__)

_APPLE_TOKEN_URL = "https://appleid.apple.com/auth/token"
_APPLE_KEYS_URL = "https://appleid.apple.com/auth/keys"
_APPLE_ISSUER = "https://appleid.apple.com"
_CLIENT_SECRET_TTL_SECONDS = 60 * 30  # 30 minutes - Apple allows up to 6 months


class AppleProvider(AuthProvider):
    def __init__(
        self,
        client_id: str,
        team_id: str,
        key_id: str,
        private_key: str,
        callback: str,
        platform: str = "",
        scopes: Optional[List[str]] = None,
    ):
        if scopes is None:
            scopes = ["name", "email"]
        self._client_id = client_id
        self._team_id = team_id
        self._key_id = key_id
        # Private keys stored in env vars/.env files usually have their
        # newlines escaped - unescape them if so, otherwise use as-is.
        self._private_key = private_key.replace("\\n", "\n")
        self._callback = callback
        self._platform = platform
        self._scopes = scopes

    def _generate_client_secret(self) -> str:
        now = int(time.time())
        claims = {
            "iss": self._team_id,
            "iat": now,
            "exp": now + _CLIENT_SECRET_TTL_SECONDS,
            "aud": _APPLE_ISSUER,
            "sub": self._client_id,
        }
        return pyjwt.encode(claims, self._private_key, algorithm="ES256", headers={"kid": self._key_id})

    def handle_code_exchange_with_verifier(self, code: str, verifier: str) -> AuthPayload:
        session = OAuth2Session(
            self._client_id,
            self._generate_client_secret(),
            redirect_uri=self._callback,
            scope=" ".join(self._scopes),
            token_endpoint_auth_method="client_secret_post",
        )
        token = exchange_code(session, _APPLE_TOKEN_URL, code, verifier)

        id_token = token.get("id_token")
        if not id_token:
            log.error("Error: apple token response missing id_token")
            raise apperrors.ErrInternalServer

        # Verify the id_token's signature against Apple's published JWKS
        # rather than trusting its claims blindly.
        try:
            claims = verify_id_token(id_token, _APPLE_KEYS_URL, _APPLE_ISSUER, self._client_id)
        except Exception as e:
            log.error(f"Error: verifying apple id_token error={e}")
            raise apperrors.ErrInvalidToken

        return AuthPayload(
            provider=self.get_provider_name(),
            email=claims.get("email", ""),
            id=claims.get("sub", ""),
        )

    def get_provider_name(self) -> str:
        return "apple"

    def get_platform(self) -> str:
        return self._platform
