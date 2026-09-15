"""Port of internal/store/main_test.go"""

from __future__ import annotations

import os
from pathlib import Path

import pytest
from sqlalchemy import text

from app.db import check_db_connection, new_engine, new_session_factory
from app.migrate import run_migrations
from app.store.postgres_user import PostgresUserStore

_MIGRATIONS_DIR = str(Path(__file__).resolve().parent.parent.parent / "migrations")


@pytest.fixture(scope="session")
def engine():
    conn_string = os.environ.get("TEST_DB_URL")
    if not conn_string:
        pytest.exit("Could not find TEST_DB_URL key in environment")
    eng = new_engine(conn_string)
    check_db_connection(eng)
    run_migrations(eng, _MIGRATIONS_DIR)
    yield eng
    eng.dispose()


@pytest.fixture()
def store(engine):
    return PostgresUserStore(new_session_factory(engine))


@pytest.fixture(autouse=True)
def clear_tables(engine):
    # RESTART IDENTITY resets your auto-incrementing IDs back to 1
    # CASCADE handles the foreign key dependencies automatically
    with engine.begin() as conn:
        conn.execute(text("TRUNCATE users, provider_identities, refresh_tokens RESTART IDENTITY CASCADE"))
    yield
    with engine.begin() as conn:
        conn.execute(text("TRUNCATE users, provider_identities, refresh_tokens RESTART IDENTITY CASCADE"))
