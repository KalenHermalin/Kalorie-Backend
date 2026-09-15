"""Port of internal/store/store_test.go

Needs a real Postgres reachable via the TEST_DB_URL env var (see
run-int-tests.sh) - same as the Go integration tests.
"""

from __future__ import annotations

import uuid
from datetime import datetime, timedelta, timezone

import pytest

from app.auth import tokens as auth
from app.models.auth import AuthPayload
from app.models.user import GO_ZERO_TIME, UserSettings, WeightLog
from app.store.postgres_user import NoRowsAffected


def test_find_refresh_token_created_at_is_never_null(store):
    """Regression test: find_refresh_token() never fetches created_at (same
    as the Go version), and User.created_at must never serialize as JSON
    null - Go's CreatedAt is a plain time.Time (not a pointer), so it's
    always a real date string, defaulting to the zero-value one rather
    than null. A client with a non-optional date field for this would
    fail to decode the response entirely if this regresses."""
    payload = AuthPayload(email="kalen@laurier.ca", id="12345", provider="github")

    def txn(session):
        user = store.upsert_user_with_auth(session, payload)
        refresh = auth.generate_refresh_token(user.id, datetime.now(timezone.utc) + timedelta(days=30), "test")
        store.save_refresh_token(session, refresh, user.id, datetime.now(timezone.utc) + timedelta(days=30))

        found_user = store.find_refresh_token(session, refresh, user.id)
        assert found_user.created_at == GO_ZERO_TIME
        # the actual thing that matters: the wire format is a real date
        # string, never JSON null.
        assert found_user.model_dump(mode="json")["created_at"] == "0001-01-01T00:00:00Z"

    store.with_tx(txn)


def test_delete_refresh_token(store):
    payload = AuthPayload(email="kalen@laurier.ca", id="12345", provider="github")

    def txn(session):
        # Test Case 1: First Login (Insert)
        user = store.upsert_user_with_auth(session, payload)
        assert user.email == payload.email

        refresh = auth.generate_refresh_token(user.id, datetime.now(timezone.utc) + timedelta(days=30), "test")
        store.save_refresh_token(session, refresh, user.id, datetime.now(timezone.utc) + timedelta(days=30))

        store.delete_refresh_token(session, refresh, user.id)

        # This SHOULD raise now because the rows affected will be 0
        with pytest.raises(Exception):
            store.delete_refresh_token(session, refresh, user.id)

    store.with_tx(txn)


def test_refresh_refresh_token(store):
    payload = AuthPayload(email="kalen@laurier.ca", id="12345", provider="github")

    def txn(session):
        user = store.upsert_user_with_auth(session, payload)
        assert user.email == payload.email

        refresh = auth.generate_refresh_token(user.id, datetime.now(timezone.utc) + timedelta(days=30), "test")
        store.save_refresh_token(session, refresh, user.id, datetime.now(timezone.utc) + timedelta(days=30))

        user = store.find_refresh_token(session, refresh, user.id)

        store.delete_refresh_token(session, refresh, user.id)

        with pytest.raises(Exception):
            store.find_refresh_token(session, refresh, user.id)

        refresh_token = auth.generate_refresh_token(user.id, datetime.now(timezone.utc) + timedelta(days=30), "test")
        store.save_refresh_token(session, refresh_token, user.id, datetime.now(timezone.utc) + timedelta(days=30))
        # should not raise - refresh rotation saved correctly
        store.find_refresh_token(session, refresh_token, user.id)

    store.with_tx(txn)


def _assert_settings_equal(expected: UserSettings, actual: UserSettings):
    assert expected.units == actual.units
    assert expected.calories_target == actual.calories_target
    assert expected.protein_target == actual.protein_target
    assert expected.carbs_target == actual.carbs_target
    assert expected.fat_target == actual.fat_target
    assert expected.theme == actual.theme

    assert expected.updated_at.replace(microsecond=expected.updated_at.microsecond) == actual.updated_at, (
        f"UpdatedAt mismatch: expected {expected.updated_at}, got {actual.updated_at}"
    )

    assert (expected.deleted_at is not None) == (actual.deleted_at is not None), "DeletedAt 'Valid' state mismatch"
    if expected.deleted_at is not None:
        assert expected.deleted_at == actual.deleted_at, f"DeletedAt mismatch: expected {expected.deleted_at}, got {actual.deleted_at}"


def test_update_user_settings_success(store):
    payload = AuthPayload(email="kalen@laurier.ca", id="12345", provider="github")

    def txn(session):
        user = store.upsert_user_with_auth(session, payload)
        assert user.email == payload.email

        now = datetime.now(timezone.utc)
        settings = UserSettings(
            calories_target=3000,
            protein_target=150,
            fat_target=25,
            carbs_target=400,
            units="metric",
            theme="dark",
            updated_at=now,
            deleted_at=None,
        )
        store.update_user_settings(session, user.id, settings)

        new_settings = store.get_user_settings(session, user.id)
        _assert_settings_equal(settings, new_settings)

    store.with_tx(txn)


def test_update_user_settings_failure(store):
    payload = AuthPayload(email="kalen@laurier.ca", id="12345", provider="github")

    def txn(session):
        user = store.upsert_user_with_auth(session, payload)
        assert user.email == payload.email

        settings = UserSettings(
            calories_target=3000,
            protein_target=150,
            fat_target=25,
            carbs_target=400,
            units="metric",
            theme="dark",
            updated_at=datetime.now(timezone.utc) + timedelta(minutes=10),
            deleted_at=None,
        )
        # user id does not exist -> expect NoRowsAffected
        with pytest.raises(NoRowsAffected):
            store.update_user_settings(session, "b38e5cee-8726-47cb-a4b6-755f8e521818", settings)

    store.with_tx(txn)


def test_upsert_user_with_auth(store):
    payload = AuthPayload(email="kalen@laurier.ca", id="12345", provider="github")

    def txn(session):
        user = store.upsert_user_with_auth(session, payload)
        assert user.email == payload.email

        # Test Case 2: Second Login (Update) - proves the ON CONFLICT logic works
        user2 = store.upsert_user_with_auth(session, payload)
        assert user.id == user2.id, "Expected same user ID for duplicate login, but got a new one"

    store.with_tx(txn)


def _assert_weight_log_equal(expected: WeightLog, actual: WeightLog):
    assert expected.log_date.date() == actual.log_date.date()
    assert expected.updated_at.replace(microsecond=expected.updated_at.microsecond) == actual.updated_at


def test_upsert_user_weight_log(store):
    payload = AuthPayload(email="kalen@laurier.ca", id="12345", provider="github")

    def txn(session):
        user = store.upsert_user_with_auth(session, payload)
        assert user.email == payload.email

        weight_log = WeightLog(
            id="b38e5cee-8726-47cb-a4b6-755f8e521818",
            weight_kg=55,
            log_date=datetime.now(timezone.utc),
            updated_at=datetime.now(timezone.utc),
            deleted_at=None,
        )
        # Test first weight log
        store.upsert_user_weight_log(session, user.id, weight_log)

        # Test updating weight log
        weight_log2 = WeightLog(
            id="b38e5cee-8726-47cb-a4b6-755f8e521818",
            weight_kg=45,
            log_date=datetime(weight_log.log_date.year, weight_log.log_date.month, weight_log.log_date.day, tzinfo=timezone.utc),
            updated_at=datetime.now(timezone.utc) + timedelta(minutes=10),
            deleted_at=None,
        )
        store.upsert_user_weight_log(session, user.id, weight_log2)

        log = store.get_user_weight_log_by_id(session, user.id, weight_log.id)
        _assert_weight_log_equal(weight_log2, log)

    store.with_tx(txn)
