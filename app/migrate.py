"""Goose-equivalent migration runner.

The migrations/*.sql files are untouched (same goose-annotated format).
For each file we take everything between `-- +goose Up` and `-- +goose Down`,
strip the `-- +goose StatementBegin` / `-- +goose StatementEnd` marker lines,
and run the remainder as a single script - this correctly handles the
trigger-function migration's internal semicolons since nothing gets split
on `;` the way a naive statement-splitter would.

Applied filenames are tracked in a `schema_migrations` table, mirroring
goose's own bookkeeping table, and only unapplied files are run - same
"goose.Up" behaviour main.go relied on.

One-time adoption case: a database that was previously migrated by the
Go app's goose runner already has every table these migrations create,
but our own `schema_migrations` starts out empty - naively "replaying"
every migration against it would fail immediately (e.g. "relation users
already exists"). If `schema_migrations` is empty AND goose's own
`goose_db_version` bookkeeping table is present, that means goose
already fully applied every migration up to the current migrations/
folder, so we back-fill `schema_migrations` with every existing filename
instead of re-running their SQL.
"""

from __future__ import annotations

import os
import re

from sqlalchemy import Engine, text

_UP_RE = re.compile(r"--\s*\+goose Up\s*\n(.*?)--\s*\+goose Down", re.DOTALL)
_MARKER_RE = re.compile(r"^\s*--\s*\+goose Statement(Begin|End)\s*$", re.MULTILINE)


def _extract_up_script(sql_text: str) -> str:
    match = _UP_RE.search(sql_text)
    if not match:
        raise ValueError("migration file is missing a '-- +goose Up' / '-- +goose Down' block")
    up_block = match.group(1)
    return _MARKER_RE.sub("", up_block).strip()


def _goose_table_exists(conn) -> bool:
    return bool(
        conn.execute(
            text("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'goose_db_version')")
        ).scalar()
    )


def run_migrations(engine: Engine, migrations_dir: str) -> None:
    filenames = sorted(f for f in os.listdir(migrations_dir) if f.endswith(".sql"))

    with engine.begin() as conn:
        conn.execute(
            text(
                """
                CREATE TABLE IF NOT EXISTS schema_migrations (
                    filename TEXT PRIMARY KEY,
                    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
                )
                """
            )
        )
        applied = {row[0] for row in conn.execute(text("SELECT filename FROM schema_migrations"))}

        if not applied and _goose_table_exists(conn):
            # Adopting a database goose already fully migrated - record
            # every migration as applied rather than replaying their SQL.
            for filename in filenames:
                conn.execute(
                    text("INSERT INTO schema_migrations (filename) VALUES (:filename)"),
                    {"filename": filename},
                )
            applied = set(filenames)

    for filename in filenames:
        if filename in applied:
            continue

        path = os.path.join(migrations_dir, filename)
        with open(path, "r") as f:
            sql_text = f.read()
        up_script = _extract_up_script(sql_text)

        with engine.begin() as conn:
            if up_script:
                conn.exec_driver_sql(up_script)
            conn.execute(
                text("INSERT INTO schema_migrations (filename) VALUES (:filename)"),
                {"filename": filename},
            )
