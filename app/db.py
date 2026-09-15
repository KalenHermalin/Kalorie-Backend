"""Engine/session creation - Python side of `sql.Open("postgres", connString)`."""

from __future__ import annotations

import time

from sqlalchemy import Engine, create_engine, text
from sqlalchemy.orm import Session, sessionmaker


def new_engine(conn_string: str) -> Engine:
    return create_engine(conn_string, future=True)


def check_db_connection(engine: Engine) -> None:
    """Port of main.go's CheckDBConnection: try for ~30s before giving up."""
    connected = False
    for _ in range(10):
        try:
            with engine.connect() as conn:
                conn.execute(text("SELECT 1"))
            connected = True
            break
        except Exception:
            print("Waiting for database connection...")
            time.sleep(3)
    if not connected:
        raise RuntimeError("Error: Could not connect to database after 30sec")


def new_session_factory(engine: Engine) -> sessionmaker[Session]:
    return sessionmaker(bind=engine, autoflush=False, expire_on_commit=False)
