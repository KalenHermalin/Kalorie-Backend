"""Port of internal/service/auth_test.go

Bug fix applied here: the Go `MockUserRepo` didn't implement every method
of the (by-then-larger) `UserRepository` interface, so `go test
./internal/service` failed to even compile. `UserRepository` here is an
`abc.ABC` (see app/models/user.py) with every method `@abstractmethod`,
so a `MockUserRepo` missing one of them would fail to instantiate with a
clear `TypeError` - the same kind of safety net Go's compiler gave for
free. This one implements every method for real, so these tests run.
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import jwt as pyjwt

from app.auth import tokens as auth
from app.models.auth import AuthPayload, AuthProvider
from app.models.user import UserRepository, User
from app.service.auth_service import AuthService
from app import utils


class MockUserRepo(UserRepository):
    def __init__(self, mock_user=None, mock_refresh=None, mock_err=None):
        self.mock_user = mock_user
        self.mock_refresh = mock_refresh
        self.mock_err = mock_err

    def upsert_user_with_auth(self, session, payload):
        if self.mock_err is not None:
            raise self.mock_err
        return self.mock_user

    def with_tx(self, fn):
        return fn(None)

    def save_refresh_token(self, session, refresh, user_id, expires_at):
        return None

    def delete_refresh_token(self, session, refresh, user_id):
        return None

    def find_refresh_token(self, session, refresh, user_id):
        return None

    def update_user_settings(self, session, user_id, settings):
        return None

    def get_user_settings(self, session, user_id):
        return None

    def upsert_user_weight_log(self, session, user_id, payload):
        return None

    def get_user_weight_log_by_id(self, session, user_id, log_id):
        return None

    def get_user_weight_logs(self, session, user_id):
        return []

    def upsert_user_exercise_sets(self, session, payload):
        return None

    def get_user_exercise_logs(self, session, user_id):
        return []

    def upsert_user_exercise_log(self, session, user_id, payload):
        return None

    def get_user_exercise_log_by_id(self, session, user_id, log_id):
        return None

    def get_user_exercise_sets_by_log_id(self, session, log_id):
        return []

    def upsert_user_food_log(self, session, user_id, payload):
        return None

    def get_user_food_logs(self, session, user_id):
        return []

    def get_user_food_log_by_id(self, session, user_id, log_id):
        return None

    def upsert_user_food_log_entry(self, session, payload):
        return None

    def get_user_food_log_entries_by_log_id(self, session, log_id):
        return []


class MockAuthProvider(AuthProvider):
    def __init__(self, name, payload=None, err=None):
        self.name = name
        self.payload = payload
        self.err = err

    def get_platform(self):
        return self.name

    def get_provider_name(self):
        return self.name

    def handle_code_exchange_with_verifier(self, code, verifier):
        if self.err is not None:
            raise self.err
        return self.payload


def test_sign_in_full_flow():
    # 1. Setup mocks
    mock_user = User(id="1", email="kalen@laurier.ca", created_at=datetime.now(timezone.utc))
    repo = MockUserRepo(mock_user=mock_user, mock_refresh="fake_refresh_token")

    provider = MockAuthProvider(
        "github",
        payload=AuthPayload(email="kalen@laurier.ca", id="12345", provider="github"),
    )

    # 2. Initialize Service with the mocks
    svc = AuthService(repo, "test_secret", "test_refresh", provider)

    # 3. Execute the Sign In
    resp, err = svc.sign_in("valid_code", "github", "", None)

    # 4. Assertions
    assert err is None, f"Expected no error, got {err}"
    utils.check_valid_string(resp.access_token)  # raises if empty
    assert resp.user.id == mock_user.id


def test_sign_in_database_failure():
    # 1. Setup mocks
    repo = MockUserRepo(mock_err=Exception("Database Connection Failed"))

    provider = MockAuthProvider(
        "github",
        payload=AuthPayload(email="kalen@laurier.ca", id="12345", provider="github"),
    )

    # 2. Initialize Service with the mocks
    svc = AuthService(repo, "test_secret", "test_refresh", provider)

    # 3. Execute the Sign In
    resp, err = svc.sign_in("valid_code", "github", "", None)

    # 4. Assertions
    assert err is not None, "Expected an error from the service when DB failes, but got nil"
    assert resp is None, "Expected response to be nill when error occurs"


def test_sign_in_provider_failure():
    # Dont need DB because we fail before reaching it
    repo = MockUserRepo()

    provider = MockAuthProvider("github", err=Exception("invalid oauth code"))

    svc = AuthService(repo, "test_secert", "test_refresh", provider)

    resp, err = svc.sign_in("expired_code", "github", "", None)

    assert err is not None and str(err) == "invalid oauth code"
    assert resp is None, "Expected response to be nil when provider fails, but got a non-nil object"


def test_generate_jwt():
    # 1. Setup
    secret = "my-laurier-secret-123"
    # We only need the secret for this test, so we can pass a nil repo

    test_user_id = "42"
    test_email = "kalen@laurier.ca"
    test_is_premium = False

    # 2. Execution
    token_string = auth.generate_access_token(test_user_id, test_email, test_is_premium, secret)

    # 3. Validation: Decode the token to check the claims
    claims = pyjwt.decode(token_string, secret, algorithms=["HS256"])

    # 4. Assertions
    assert claims["sub"] == test_user_id, f"Expected userID {test_user_id}, got {claims['sub']}"
    assert claims["email"] == test_email, f"Expected email {test_email}, got {claims['email']}"
    assert claims["is_premium"] == test_is_premium

    expires_at = datetime.fromtimestamp(claims["exp"], tz=timezone.utc)
    assert expires_at - datetime.now(timezone.utc) <= timedelta(minutes=15), "Expiration time is too far in the future"
