"""Port of internal/store/postgres_user.go

Bug fixes applied while porting (see the plan for the full list):
  * GetUserExerciseLogById - Go scanned into an unallocated struct (nil
    pointer panic). The ORM's select() naturally returns a real row or
    None, so this class of bug can't recur here.
  * GetUserExerciseSetsByLogId / GetUserFoodLogEntriesByLogId - Go looped
    over rows but never appended them to the result list (and, for the
    food-entries version, shadowed the outer list variable). Fixed below
    by actually building and returning the list.
  * GetUserExerciseLogs - Go scanned `entry.DeletedAt` instead of
    `&entry.DeletedAt` (wrong pointer level). The ORM maps columns to
    attributes directly, so this can't happen here either.

Everything else - including things that look a little off, like
UpsertUserExerciseSets never updating `weight` on conflict, or
FindRefreshToken accepting but not actually filtering on user_id - is
kept exactly as the Go version had it.
"""

from __future__ import annotations

from datetime import date, datetime, time, timezone
from typing import Callable, List, TypeVar

from sqlalchemy import delete, func, select, update
from sqlalchemy.dialects.postgresql import insert as pg_insert
from sqlalchemy.orm import Session, sessionmaker

from app.models import auth as auth_models
from app.models import user as user_models
from app.store import orm_models as rows

T = TypeVar("T")

# Mirrors Go's `var ErrNoRowsAffected = errors.New("no rows affected")`
class NoRowsAffected(Exception):
    pass


def _to_datetime(d: date | datetime | None) -> datetime | None:
    if d is None:
        return None
    if isinstance(d, datetime):
        return d
    return datetime.combine(d, time.min, tzinfo=timezone.utc)


def _to_date(d: date | datetime | None) -> date | None:
    if d is None:
        return None
    if isinstance(d, datetime):
        return d.date()
    return d


def _weight_log_to_model(row: rows.WeightLogRow) -> user_models.WeightLog:
    return user_models.WeightLog(
        id=row.id,
        weight_kg=row.weight_kg,
        log_date=_to_datetime(row.log_date),
        updated_at=row.updated_at,
        deleted_at=row.deleted_at,
    )


def _exercise_log_to_model(row: rows.ExerciseLogRow) -> user_models.ExerciseLog:
    return user_models.ExerciseLog(
        id=row.id,
        exercise_id=row.exercise_id,
        created_at=row.created_at,
        updated_at=row.updated_at,
        deleted_at=row.deleted_at,
    )


def _exercise_set_to_model(row: rows.ExerciseSetRow) -> user_models.ExerciseSet:
    return user_models.ExerciseSet(
        id=row.id,
        log_id=row.log_id,
        set_number=row.set_number,
        weight=row.weight,
        reps=row.reps,
        updated_at=row.updated_at,
        deleted_at=row.deleted_at,
    )


def _food_log_to_model(row: rows.FoodLogRow) -> user_models.FoodLog:
    return user_models.FoodLog(
        id=row.id,
        name=row.name,
        Time=row.time.isoformat() if row.time else "",
        created_at=row.created_at,
        updated_at=row.updated_at,
        deleted_at=row.deleted_at,
    )


def _food_log_entry_to_model(row: rows.FoodLogItemRow) -> user_models.FoodLogEntry:
    return user_models.FoodLogEntry(
        id=row.id,
        log_id=row.log_id,
        food_id=row.food_id or "",
        food_name=row.food_name,
        serving_id=row.serving_id or "",
        unit=row.unit or "",
        cal=row.cal,
        fat=row.fat,
        carbs=row.carbs,
        protein=row.protein,
        quantity=row.quantity or 0,
        updated_at=row.updated_at,
        deleted_at=row.deleted_at,
    )


class PostgresUserStore(user_models.UserRepository):
    def __init__(self, session_factory: sessionmaker[Session]):
        self._session_factory = session_factory

    # --- Auth / refresh tokens -------------------------------------------------

    def save_refresh_token(self, session: Session, refresh: str, user_id: str, expires_at: datetime) -> None:
        session.add(rows.RefreshTokenRow(user_id=user_id, token=refresh, expires_at=expires_at))
        session.flush()

    def find_refresh_token(self, session: Session, refresh: str, user_id: str) -> user_models.User:
        stmt = (
            select(rows.UserRow.id, rows.UserRow.email)
            .join(rows.RefreshTokenRow, rows.UserRow.id == rows.RefreshTokenRow.user_id)
            .where(rows.RefreshTokenRow.token == refresh, rows.RefreshTokenRow.expires_at > func.now())
        )
        row = session.execute(stmt).first()
        if row is None:
            raise LookupError("sql: no rows in result set")
        return user_models.User(id=row.id, email=row.email)

    def delete_refresh_token(self, session: Session, token: str, user_id: str) -> None:
        result = session.execute(
            delete(rows.RefreshTokenRow).where(
                rows.RefreshTokenRow.user_id == user_id, rows.RefreshTokenRow.token == token
            )
        )
        rows_affected = result.rowcount
        if rows_affected == 0:
            raise Exception("Could not delete token")
        if rows_affected == 1:
            return
        raise Exception("Unknown Error Occured")

    def upsert_user_with_auth(self, session: Session, payload: auth_models.AuthPayload) -> user_models.User:
        user_stmt = pg_insert(rows.UserRow).values(email=payload.email)
        user_stmt = user_stmt.on_conflict_do_update(
            index_elements=["email"], set_={"email": user_stmt.excluded.email}
        ).returning(rows.UserRow.id, rows.UserRow.email, rows.UserRow.created_at)
        row = session.execute(user_stmt).first()
        user = user_models.User(id=row.id, email=row.email, created_at=row.created_at)

        provider_stmt = pg_insert(rows.ProviderIdentityRow).values(
            user_id=user.id, provider=payload.provider, provider_user_id=payload.id
        )
        provider_stmt = provider_stmt.on_conflict_do_nothing(index_elements=["provider", "provider_user_id"])
        session.execute(provider_stmt)
        session.flush()
        return user

    # --- Settings ----------------------------------------------------------

    def get_user_settings(self, session: Session, user_id: str) -> user_models.UserSettings:
        row = session.execute(
            select(rows.UserSettingsRow).where(rows.UserSettingsRow.user_id == user_id)
        ).scalar_one_or_none()
        if row is None:
            raise NoRowsAffected("no rows affected")
        return user_models.UserSettings(
            units=row.units,
            calories_target=row.calories_target,
            protein_target=row.protein_target,
            carbs_target=row.carbs_target,
            fat_target=row.fat_target,
            updated_at=row.updated_at,
            deleted_at=row.deleted_at,
            theme=row.theme,
        )

    def update_user_settings(self, session: Session, user_id: str, settings: user_models.UserSettings) -> None:
        result = session.execute(
            update(rows.UserSettingsRow)
            .where(rows.UserSettingsRow.user_id == user_id, rows.UserSettingsRow.updated_at < settings.updated_at)
            .values(
                units=settings.units,
                calories_target=settings.calories_target,
                protein_target=settings.protein_target,
                carbs_target=settings.carbs_target,
                fat_target=settings.fat_target,
                updated_at=settings.updated_at,
                deleted_at=settings.deleted_at,
                theme=settings.theme,
            )
        )
        if result.rowcount == 0:
            raise NoRowsAffected("no rows affected")

    # --- Weight logs ---------------------------------------------------------

    def upsert_user_weight_log(self, session: Session, user_id: str, payload: user_models.WeightLog) -> None:
        stmt = pg_insert(rows.WeightLogRow).values(
            id=payload.id,
            user_id=user_id,
            weight_kg=payload.weight_kg,
            log_date=_to_date(payload.log_date),
            updated_at=payload.updated_at,
            deleted_at=payload.deleted_at,
        )
        stmt = stmt.on_conflict_do_update(
            index_elements=["user_id", "log_date"],
            set_={
                "weight_kg": stmt.excluded.weight_kg,
                "updated_at": stmt.excluded.updated_at,
                "deleted_at": stmt.excluded.deleted_at,
            },
            where=(stmt.excluded.updated_at > rows.WeightLogRow.updated_at),
        )
        session.execute(stmt)

    def get_user_weight_log_by_id(self, session: Session, user_id: str, log_id: str) -> user_models.WeightLog:
        row = session.execute(
            select(rows.WeightLogRow).where(rows.WeightLogRow.user_id == user_id, rows.WeightLogRow.id == log_id)
        ).scalar_one_or_none()
        if row is None:
            raise NoRowsAffected("no rows affected")
        return _weight_log_to_model(row)

    def get_user_weight_logs(self, session: Session, user_id: str) -> List[user_models.WeightLog]:
        result = session.execute(
            select(rows.WeightLogRow)
            .where(rows.WeightLogRow.user_id == user_id)
            .order_by(rows.WeightLogRow.log_date.desc())
        )
        return [_weight_log_to_model(row) for row in result.scalars().all()]

    # --- Exercise logs ---------------------------------------------------------

    def upsert_user_exercise_log(self, session: Session, user_id: str, payload: user_models.ExerciseLog) -> None:
        stmt = pg_insert(rows.ExerciseLogRow).values(
            id=payload.id,
            user_id=user_id,
            exercise_id=payload.exercise_id,
            created_at=payload.created_at,
            updated_at=payload.updated_at,
            deleted_at=payload.deleted_at,
        )
        stmt = stmt.on_conflict_do_update(
            index_elements=["id"],
            set_={
                "exercise_id": stmt.excluded.exercise_id,
                "updated_at": stmt.excluded.updated_at,
                "deleted_at": stmt.excluded.deleted_at,
            },
            where=(stmt.excluded.updated_at > rows.ExerciseLogRow.updated_at),
        )
        session.execute(stmt)

    # NOTE (matches a `// TODO: Fix like done for the food logs` left in the
    # Go source): this still does one query for the logs, then one query
    # PER log for its sets, instead of a join. Kept as-is.
    def get_user_exercise_logs(self, session: Session, user_id: str) -> List[user_models.FullExerciseLog]:
        log_rows = (
            session.execute(
                select(rows.ExerciseLogRow)
                .where(rows.ExerciseLogRow.user_id == user_id)
                .order_by(rows.ExerciseLogRow.created_at.desc())
            )
            .scalars()
            .all()
        )

        full_logs: List[user_models.FullExerciseLog] = []
        for log_row in log_rows:
            set_rows = (
                session.execute(select(rows.ExerciseSetRow).where(rows.ExerciseSetRow.log_id == log_row.id))
                .scalars()
                .all()
            )
            full_logs.append(
                user_models.FullExerciseLog(
                    exercise_log=_exercise_log_to_model(log_row),
                    exercise_sets=[_exercise_set_to_model(s) for s in set_rows],
                )
            )
        return full_logs

    def get_user_exercise_log_by_id(self, session: Session, user_id: str, log_id: str) -> user_models.ExerciseLog:
        row = session.execute(
            select(rows.ExerciseLogRow)
            .where(rows.ExerciseLogRow.user_id == user_id, rows.ExerciseLogRow.id == log_id)
            .order_by(rows.ExerciseLogRow.created_at.desc())
        ).scalar_one_or_none()
        if row is None:
            raise LookupError("sql: no rows in result set")
        return _exercise_log_to_model(row)

    def upsert_user_exercise_sets(self, session: Session, payload: List[user_models.ExerciseSet]) -> None:
        if payload is None:
            raise ValueError("PARAMETER MISSING")

        for entry in payload:
            stmt = pg_insert(rows.ExerciseSetRow).values(
                id=entry.id,
                log_id=entry.log_id,
                set_number=entry.set_number,
                weight=entry.weight,
                reps=entry.reps,
                updated_at=entry.updated_at,
                deleted_at=entry.deleted_at,
            )
            # NOTE: `weight` is intentionally NOT in the DO UPDATE SET below,
            # matching the Go query exactly - updating a set's weight after
            # the first insert is a no-op, same as the original.
            stmt = stmt.on_conflict_do_update(
                index_elements=["id"],
                set_={
                    "set_number": stmt.excluded.set_number,
                    "reps": stmt.excluded.reps,
                    "updated_at": stmt.excluded.updated_at,
                    "deleted_at": stmt.excluded.deleted_at,
                },
                where=(stmt.excluded.updated_at > rows.ExerciseSetRow.updated_at),
            )
            session.execute(stmt)

    def get_user_exercise_sets_by_log_id(self, session: Session, log_id: str) -> List[user_models.ExerciseSet]:
        result = session.execute(select(rows.ExerciseSetRow).where(rows.ExerciseSetRow.log_id == log_id))
        return [_exercise_set_to_model(row) for row in result.scalars().all()]

    # --- Food logs ---------------------------------------------------------

    def upsert_user_food_log(self, session: Session, user_id: str, payload: user_models.FoodLog) -> None:
        # payload.Time is a plain string on the model (matches the Go
        # struct); parse it back to a datetime for the TIMESTAMPTZ column.
        time_value = datetime.fromisoformat(payload.Time) if payload.Time else None
        stmt = pg_insert(rows.FoodLogRow).values(
            id=payload.id,
            user_id=user_id,
            time=time_value,
            name=payload.name,
            created_at=payload.created_at,
            updated_at=payload.updated_at,
            deleted_at=payload.deleted_at,
        )
        stmt = stmt.on_conflict_do_update(
            index_elements=["id"],
            set_={
                "name": stmt.excluded.name,
                "time": stmt.excluded.time,
                "updated_at": stmt.excluded.updated_at,
                "deleted_at": stmt.excluded.deleted_at,
            },
            where=(stmt.excluded.updated_at > rows.FoodLogRow.updated_at),
        )
        session.execute(stmt)

    def get_user_food_logs(self, session: Session, user_id: str) -> List[user_models.FullFoodLog]:
        log_rows = (
            session.execute(
                select(rows.FoodLogRow)
                .where(rows.FoodLogRow.user_id == user_id)
                .order_by(rows.FoodLogRow.created_at.desc())
            )
            .scalars()
            .all()
        )

        full_logs: List[user_models.FullFoodLog] = []
        for log_row in log_rows:
            entry_rows = (
                session.execute(select(rows.FoodLogItemRow).where(rows.FoodLogItemRow.log_id == log_row.id))
                .scalars()
                .all()
            )
            full_logs.append(
                user_models.FullFoodLog(
                    food_log=_food_log_to_model(log_row),
                    food_log_entries=[_food_log_entry_to_model(e) for e in entry_rows],
                )
            )
        return full_logs

    def get_user_food_log_by_id(self, session: Session, user_id: str, log_id: str) -> user_models.FoodLog:
        row = session.execute(
            select(rows.FoodLogRow).where(rows.FoodLogRow.user_id == user_id, rows.FoodLogRow.id == log_id)
        ).scalar_one_or_none()
        if row is None:
            raise LookupError("sql: no rows in result set")
        return _food_log_to_model(row)

    def upsert_user_food_log_entry(self, session: Session, payload: List[user_models.FoodLogEntry]) -> None:
        if payload is None:
            raise ValueError("PARAMETER MISSING")

        for entry in payload:
            stmt = pg_insert(rows.FoodLogItemRow).values(
                id=entry.id,
                log_id=entry.log_id,
                food_id=entry.food_id,
                food_name=entry.food_name,
                serving_id=entry.serving_id,
                quantity=entry.quantity,
                unit=entry.unit,
                cal=entry.cal,
                fat=entry.fat,
                carbs=entry.carbs,
                protein=entry.protein,
                updated_at=entry.updated_at,
                deleted_at=entry.deleted_at,
            )
            stmt = stmt.on_conflict_do_update(
                index_elements=["id"],
                set_={
                    "food_name": stmt.excluded.food_name,
                    "serving_id": stmt.excluded.serving_id,
                    "quantity": stmt.excluded.quantity,
                    "unit": stmt.excluded.unit,
                    "cal": stmt.excluded.cal,
                    "fat": stmt.excluded.fat,
                    "carbs": stmt.excluded.carbs,
                    "protein": stmt.excluded.protein,
                    "updated_at": stmt.excluded.updated_at,
                    "deleted_at": stmt.excluded.deleted_at,
                },
                where=(stmt.excluded.updated_at > rows.FoodLogItemRow.updated_at),
            )
            session.execute(stmt)

    def get_user_food_log_entries_by_log_id(self, session: Session, log_id: str) -> List[user_models.FoodLogEntry]:
        result = session.execute(select(rows.FoodLogItemRow).where(rows.FoodLogItemRow.log_id == log_id))
        return [_food_log_entry_to_model(row) for row in result.scalars().all()]

    # --- Transactions ---------------------------------------------------------

    def with_tx(self, fn: Callable[[Session], T]) -> T:
        session = self._session_factory()
        try:
            result = fn(session)
        except Exception:
            session.rollback()
            session.close()
            raise
        else:
            session.commit()
            session.close()
            return result
