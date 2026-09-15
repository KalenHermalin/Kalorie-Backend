"""Port of internal/apperrors/errors.go

AppError is used both as the thing handlers raise/return, and (via the
FastAPI exception handler installed in api.py) as the thing that gets
written back to the client - mirroring Go's `apperrors.WriteError`.
"""

from __future__ import annotations

from fastapi import status


class AppError(Exception):
    """Code = machine-readable (e.g. ERR_USER_NOT_FOUND), Message = human-readable."""

    def __init__(self, code: str, message: str, status_code: int):
        self.code = code
        self.message = message
        self.status = status_code
        super().__init__(f"[{code}] {message}")

    def to_dict(self) -> dict:
        # Only code + message are sent, matching the `json:"-"` on Status in Go.
        return {"code": self.code, "message": self.message}


def new_app_error(code: str, message: str, status_code: int) -> AppError:
    return AppError(code, message, status_code)


# TODO: App Errors should be for internal use in the service layer

ErrBadRequestBody = AppError(
    "ERR_BAD_REQUEST",
    "Error decoding request body",
    status.HTTP_400_BAD_REQUEST,
)
ErrUnauthoirized = AppError(
    "ERR_UNAUTHORIZED",
    "You are not authorized. Please sign in!",
    status.HTTP_401_UNAUTHORIZED,
)
ErrInvalidToken = AppError(
    "ERR_INVALID_TOKEN",
    "Token invalid or expired. Please try logging in again.",
    status.HTTP_401_UNAUTHORIZED,
)
ErrInvalidTokenFormat = AppError(
    "ERR_INVALID_TOKEN_FORMAT",
    "Token format is not correct. Expected:  `Authorization: Bearer <access_token>`",
    status.HTTP_400_BAD_REQUEST,
)

ErrInvalidProvider = AppError(
    "ERR_INVALID_PROVIDER",
    "Invalid auth provider. Try another!",
    status.HTTP_400_BAD_REQUEST,
)
AuthErrLoginFailed = AppError(
    "ERR_LOGIN_FAILED",
    "Login failed with login provider. Please try again in a few.",
    status.HTTP_400_BAD_REQUEST,
)
AuthErrUnavailableService = AppError(
    "ERR_AUTH_SERVICE_UNAVAILABLE",
    "Authentication service temporarily unavailable. Please try again shortly.",
    status.HTTP_503_SERVICE_UNAVAILABLE,
)
ErrInternalServer = AppError(
    "ERR_INTERNAL_SERVER",
    "An internal server error occured",
    status.HTTP_500_INTERNAL_SERVER_ERROR,
)

ErrInvalidRequest = AppError(
    "ERR_INVALID_REQUEST",
    "The provided request is invalid. Please refer to documentation!",
    status.HTTP_400_BAD_REQUEST,
)
ErrMissingUserIdInContext = AppError(
    "ERR_MISSING_USER_ID",
    "Missing UserId from context",
    status.HTTP_500_INTERNAL_SERVER_ERROR,
)
ErrUserSettingsMissing = AppError(
    "ERR_USER_SETTINGS_MISSING",
    "User Settings do not exist. Please contact support!",
    status.HTTP_500_INTERNAL_SERVER_ERROR,
)
ErrSyncConflict = AppError(
    "ERR_SYNC_CONFLICT",
    "A newer version of this data already exists",
    status.HTTP_409_CONFLICT,
)
ErrNotModified = AppError(
    "ERR_NOT_MODIFIED",
    "The requested resournce wasnt modified since the date requested",
    status.HTTP_304_NOT_MODIFIED,
)
ErrSoftWeightLog = AppError(
    "ERR_SOFT_UPSETING_WEIGHT_LOG",
    "There was an internal error updating some of yout weight logs",
    status.HTTP_207_MULTI_STATUS,
)
