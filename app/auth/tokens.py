"""Port of internal/auth/auth.go"""

from __future__ import annotations

import logging
from datetime import datetime, timedelta, timezone

import jwt
from authlib.integrations.requests_client import OAuth2Session
from authlib.oauth2.rfc6749.errors import OAuth2Error

from app import apperrors

log = logging.getLogger(__name__)


def generate_access_token(user_id: str, email: str, is_premium: bool, secret: str) -> str:
    now = datetime.now(timezone.utc)
    claims = {
        "sub": user_id,
        "email": email,
        "is_premium": is_premium,
        "exp": now + timedelta(minutes=15),
        "iat": now,
    }
    return jwt.encode(claims, secret, algorithm="HS256")


def generate_refresh_token(user_id: str, expires_in: datetime, secret: str) -> str:
    claims = {
        "sub": user_id,
        "exp": expires_in,
        "iat": datetime.now(timezone.utc),
    }
    return jwt.encode(claims, secret, algorithm="HS256")


def exchange_code(session: OAuth2Session, token_url: str, code: str, verifier: str) -> dict:
    try:
        token = session.fetch_token(
            token_url,
            code=code,
            code_verifier=verifier,
            grant_type="authorization_code",
        )
    except OAuth2Error as re:
        error_code = getattr(re, "error", None)
        log.error(f"Error: oauth2 token exchange ErrorCode={error_code} Description={getattr(re, 'description', None)}")
        if error_code in ("invalid_grant", "access_denied"):
            # Please try again in a few
            raise apperrors.AuthErrLoginFailed from re
        if error_code in ("unauthorized_client", "invalid_scope"):
            # internal server error
            raise apperrors.ErrInternalServer from re
        if error_code in ("server_error", "temporarily_unavailable"):
            # auth provider temporarily down
            raise apperrors.AuthErrUnavailableService from re
        # unknown error
        raise
    except Exception as e:
        log.error(f"Error: Non oauth2 error={e}")
        raise apperrors.ErrInternalServer from e
    return token
