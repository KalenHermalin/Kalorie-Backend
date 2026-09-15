"""Port of internal/middlewares/auth.go

Bug fix applied here: Go's GetUserID() type-asserted the context value to
`int`, but it was always stored as a `string` (the JWT `sub` claim), so it
silently always returned 0. Fixed below to read it as a string. Like in
Go, neither GetUserID nor GetIsPremium is actually called by any handler
today - handlers use the auth dependency's return value directly instead
(the same way the Go handlers read `middlewares.UserIDKey` straight out of
the request context rather than calling GetUserID).
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Callable

import jwt as pyjwt
from fastapi import Request

from app import apperrors

USER_ID_KEY = "userID"
IS_PREMIUM_KEY = "isPremium"


@dataclass
class AuthContext:
    user_id: str
    is_premium: bool


def auth(jwt_secret: str) -> Callable[[Request], AuthContext]:
    """Mirrors middlewares.Auth(jwtSecret) - a factory returning the actual
    FastAPI dependency (chi's `mux.Use(middlewares.Auth(secret))` becomes
    `Depends(auth(secret))` on each protected route)."""

    def require_auth(request: Request) -> AuthContext:
        # 1. Get the Authorization header
        auth_header = request.headers.get("Authorization", "")
        if auth_header == "":
            raise apperrors.ErrUnauthoirized

        # 2. Parse the Bearer token
        parts = auth_header.split(" ")
        if len(parts) != 2 or parts[0] != "Bearer":
            raise apperrors.ErrInvalidTokenFormat

        # 3. Validate the token
        try:
            claims = pyjwt.decode(parts[1], jwt_secret, algorithms=["HS256"])
        except Exception:
            raise apperrors.ErrInvalidToken

        user_id = claims.get("sub", "")
        is_premium = bool(claims.get("is_premium", False))

        # 4. Inject UserID into request state for your handlers (mirrors
        # context.WithValue in Go)
        setattr(request.state, USER_ID_KEY, user_id)
        setattr(request.state, IS_PREMIUM_KEY, is_premium)

        return AuthContext(user_id=user_id, is_premium=is_premium)

    return require_auth


def get_user_id(request: Request) -> str:
    return getattr(request.state, USER_ID_KEY, "")


def get_is_premium(request: Request) -> bool:
    return getattr(request.state, IS_PREMIUM_KEY, False)
