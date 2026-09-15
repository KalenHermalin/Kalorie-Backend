"""Port of internal/models/auth.go"""

from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime
from typing import Optional

from pydantic import BaseModel, ConfigDict, Field

from app.models.user import User


class AuthPayload(BaseModel):
    id: str = ""
    email: str = ""
    provider: str = ""


class AuthResponse(BaseModel):
    # NOTE: the field is named `user` (not `User`) on purpose - Pydantic v2
    # resolves the `User` type annotation lazily, and a field literally
    # named `User` shadows the `User` type it's supposed to refer to,
    # making Pydantic think the field's type is `None` (it then mis-
    # serializes any real User value assigned to it). The alias below
    # keeps the wire format identical to Go's (an untagged `User` field
    # serializes as `"User"`, capitalized) without the name collision.
    model_config = ConfigDict(populate_by_name=True)

    access_token: str = ""
    refresh_token: str = ""
    expires_in: int = 0  # Standard naming convention
    user: Optional[User] = Field(default=None, alias="User")


class AuthProvider(ABC):
    @abstractmethod
    def get_provider_name(self) -> str: ...

    # TODO: make a HandleCodeExchange without the verifier needed
    @abstractmethod
    def handle_code_exchange_with_verifier(self, code: str, verifier: str) -> AuthPayload: ...

    @abstractmethod
    def get_platform(self) -> str: ...


@dataclass
class CustomClaimsAccess:
    user_id: str  # json:"sub"
    email: str
    # Future-proofing: add a field for subscription status
    is_premium: bool
    expires_at: Optional[datetime] = None
    issued_at: Optional[datetime] = None


@dataclass
class CustomClaimsRefresh:
    user_id: str  # json:"sub"
    expires_at: Optional[datetime] = None
    issued_at: Optional[datetime] = None
