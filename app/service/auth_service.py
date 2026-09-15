"""Port of internal/service/auth.go"""

from __future__ import annotations

import logging
from datetime import datetime, timedelta, timezone
from typing import List, Optional, Tuple

import jwt as pyjwt

from app import apperrors
from app.apperrors import AppError
from app.auth import tokens as auth
from app.models.auth import AuthPayload, AuthProvider, AuthResponse
from app.models.user import UserRepository

log = logging.getLogger(__name__)

_THIRTY_DAYS = timedelta(days=30)


class AuthService:
    def __init__(self, user_repository: UserRepository, access_secret: str, refresh_secret: str, *providers: AuthProvider):
        self.us = user_repository
        self.jwt_access_secret = access_secret
        self.jwt_refresh_secret = refresh_secret
        self.providers: List[AuthProvider] = list(providers)

    def log_out(self, user_id: str, token: str) -> Optional[Exception]:
        def txn(session):
            try:
                self.us.delete_refresh_token(session, token, user_id)
            except Exception as e:
                log.error(f"DELETING_REFRESH_TOKEN error={e}")
                raise apperrors.new_app_error("ERR_DELETING_REFRESH_TOKEN", f"error deleting refresh token: {e}", 500)

        try:
            self.us.with_tx(txn)
        except Exception as e:
            return e
        return None

    def refresh_access_token(self, refresh: str) -> Tuple[Optional[AuthResponse], Optional[Exception]]:
        try:
            claims = pyjwt.decode(refresh, self.jwt_refresh_secret, algorithms=["HS256"])
        except Exception as e:
            log.error(f"INVALID_TOKEN error={e}")
            return None, apperrors.ErrInvalidToken

        user_id = claims.get("sub", "")
        resp = AuthResponse()

        def txn(session):
            try:
                user = self.us.find_refresh_token(session, refresh, user_id)
            except Exception as e:
                log.error(f"REFRESH_TOKEN_DOESNT_EXIST error={e} userID={user_id}")
                raise apperrors.ErrInvalidToken

            try:
                self.us.delete_refresh_token(session, refresh, user_id)
            except Exception as e:
                log.error(f"REFRESH_TOKEN_DELETION_FAILED error={e} userID={user_id}")
                raise apperrors.new_app_error(
                    "ERR_DELETING_REFRESH_TOKEN",
                    "There was an error deleting the refresh token. Please try again in a few. If problem presists contact support.",
                    500,
                )

            try:
                access_token = auth.generate_access_token(user.id, user.email, False, self.jwt_access_secret)
            except Exception as e:
                log.error(f"Error: Couldnt Generate Access Token error={e} userID={user_id}")
                raise apperrors.new_app_error(
                    "ERR_GENERATING_ACCESS_TOKEN",
                    "There was an error generating access token. Please try again in a few. If problem presists contact support.",
                    500,
                )

            try:
                refresh_token = auth.generate_refresh_token(user.id, datetime.now(timezone.utc) + _THIRTY_DAYS, self.jwt_refresh_secret)
            except Exception as e:
                log.error(f"Error: Couldnt Generate Refresh Token error={e} userID={user_id}")
                raise apperrors.new_app_error(
                    "ERR_GENERATING_ACCESS_TOKEN",
                    "There was an error generating refresh token. Please try again in a few. If problem presist contact support.",
                    500,
                )

            try:
                self.us.save_refresh_token(session, refresh_token, user.id, datetime.now(timezone.utc) + _THIRTY_DAYS)
            except Exception as e:
                log.error(f"Error: Couldnt Save Refresh Token error={e} userID={user_id}")
                raise apperrors.new_app_error(
                    "ERR_SAVING_REFRESH_TOKEN",
                    "There was an error saving refresh token. Please try again in a few. If problem presist contact support.",
                    500,
                )

            resp.access_token = access_token
            resp.refresh_token = refresh_token
            resp.expires_in = 900
            resp.User = user

        try:
            self.us.with_tx(txn)
        except Exception as e:
            return None, e
        return resp, None

    def sign_in(self, code: str, provider: str, verifier: str, platform: Optional[str]) -> Tuple[Optional[AuthResponse], Optional[Exception]]:
        try:
            payload = self._exchange_code(code, provider, verifier, platform)
        except Exception as e:
            # Error is appError or generic
            return None, e

        resp = AuthResponse()

        def txn(session):
            try:
                user = self.us.upsert_user_with_auth(session, payload)
            except Exception as e:
                log.error(f"Error: Couldnt Upsert User error={e} user_email={payload.email}")
                raise apperrors.new_app_error("ERR_UPSERT_USER", "Error signing in, please try again later", 500)

            try:
                access_token = auth.generate_access_token(user.id, user.email, False, self.jwt_access_secret)
            except Exception as e:
                log.error(f"Error: Couldnt Generate Access Token error={e} userID={user.id} user_email={user.email}")
                raise apperrors.new_app_error(
                    "ERR_GENERATING_ACCESS_TOKEN",
                    "There was an error generating access token. Please try again in a few. If problem presists contact support.",
                    500,
                )

            try:
                refresh = auth.generate_refresh_token(user.id, datetime.now(timezone.utc) + _THIRTY_DAYS, self.jwt_refresh_secret)
            except Exception as e:
                log.error(f"Error: Couldnt Generate Refresh Token error={e} userID={user.id} user_email={user.email}")
                raise apperrors.new_app_error(
                    "ERR_GENERATING_ACCESS_TOKEN",
                    "There was an error generating refresh token. Please try again in a few. If problem presist contact support.",
                    500,
                )

            try:
                self.us.save_refresh_token(session, refresh, user.id, datetime.now(timezone.utc) + _THIRTY_DAYS)
            except Exception as e:
                log.error(f"Error: Couldnt Save Refresh Token error={e} userID={user.id} user_email={user.email}")
                raise apperrors.new_app_error(
                    "ERR_SAVING_REFRESH_TOKEN",
                    "There was an error saving refresh token. Please try again in a few. If problem presist contact support.",
                    500,
                )

            resp.access_token = access_token
            resp.refresh_token = refresh
            resp.expires_in = 900
            resp.User = user

        try:
            self.us.with_tx(txn)
        except Exception as e:
            return None, e
        return resp, None

    def _exchange_code(self, code: str, provider: str, verifier: str, platform: Optional[str]) -> AuthPayload:
        if platform is None:
            for p in self.providers:
                if p.get_provider_name() == provider:
                    # returns both an appError and generic error
                    return p.handle_code_exchange_with_verifier(code, verifier)

        for p in self.providers:
            if p.get_provider_name() == provider and p.get_platform() == platform:
                # returns both an appError and generic error
                return p.handle_code_exchange_with_verifier(code, verifier)

        raise apperrors.ErrInvalidProvider
