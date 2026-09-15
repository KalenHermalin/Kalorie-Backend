"""SQLAlchemy ORM classes mapped 1:1 onto the tables created by migrations/*.sql.

These are intentionally kept separate from app/models/*.py (the plain DTOs
that flow through service/handler code) - postgres_user.py is the only
place that touches these directly, then converts rows to/from the DTOs.
This mirrors how the Go store package was the only place doing raw
`Scan()` calls into `models.*` structs.
"""

from __future__ import annotations

import uuid
from datetime import date, datetime

from sqlalchemy import CheckConstraint, Date, DateTime, ForeignKey, Integer, Numeric, Text, UniqueConstraint
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


class UserRow(Base):
    __tablename__ = "users"

    id: Mapped[str] = mapped_column(UUID(as_uuid=False), primary_key=True, default=lambda: str(uuid.uuid4()))
    email: Mapped[str] = mapped_column(Text, unique=True, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False, server_default="now()")
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False, server_default="now()")


class ProviderIdentityRow(Base):
    __tablename__ = "provider_identities"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(UUID(as_uuid=False), ForeignKey("users.id", ondelete="CASCADE"), nullable=False)
    provider: Mapped[str] = mapped_column(Text, nullable=False)
    provider_user_id: Mapped[str] = mapped_column(Text, nullable=False)

    __table_args__ = (
        CheckConstraint("provider IN ('github', 'google', 'apple')", name="check_allowed_providers"),
        UniqueConstraint("provider", "provider_user_id"),
    )


class RefreshTokenRow(Base):
    __tablename__ = "refresh_tokens"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(UUID(as_uuid=False), ForeignKey("users.id", ondelete="CASCADE"), nullable=False)
    token: Mapped[str] = mapped_column(Text, nullable=False, unique=True)
    expires_at: Mapped[datetime] = mapped_column(DateTime(timezone=False), nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=False), server_default="CURRENT_TIMESTAMP")


class UserSettingsRow(Base):
    __tablename__ = "user_settings"

    user_id: Mapped[str] = mapped_column(UUID(as_uuid=False), ForeignKey("users.id", ondelete="CASCADE"), primary_key=True)
    units: Mapped[str] = mapped_column(Text, nullable=False)
    calories_target: Mapped[int] = mapped_column(Integer, nullable=False)
    protein_target: Mapped[int] = mapped_column(Integer, nullable=False)
    carbs_target: Mapped[int] = mapped_column(Integer, nullable=False)
    fat_target: Mapped[int] = mapped_column(Integer, nullable=False)
    updated_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), server_default="now()")
    deleted_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    theme: Mapped[str] = mapped_column(Text, nullable=False, server_default="system")

    __table_args__ = (CheckConstraint("theme IN ('system', 'light', 'dark')", name="check_allowed_themes"),)


class WeightLogRow(Base):
    __tablename__ = "weight_logs"

    id: Mapped[str] = mapped_column(UUID(as_uuid=False), primary_key=True, default=lambda: str(uuid.uuid4()))
    user_id: Mapped[str] = mapped_column(UUID(as_uuid=False), ForeignKey("users.id", ondelete="CASCADE"), nullable=False)
    weight_kg: Mapped[float] = mapped_column(Numeric, nullable=False)
    log_date: Mapped[date] = mapped_column(Date, nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False, server_default="now()")
    deleted_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)

    __table_args__ = (UniqueConstraint("user_id", "log_date", name="unique_user_daily_weight"),)


class ExerciseLogRow(Base):
    __tablename__ = "exercise_logs"

    id: Mapped[str] = mapped_column(UUID(as_uuid=False), primary_key=True)
    user_id: Mapped[str] = mapped_column(UUID(as_uuid=False), ForeignKey("users.id", ondelete="CASCADE"), nullable=False)
    exercise_id: Mapped[str] = mapped_column(Text, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    deleted_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class ExerciseSetRow(Base):
    __tablename__ = "exercise_sets"

    id: Mapped[str] = mapped_column(UUID(as_uuid=False), primary_key=True)
    log_id: Mapped[str] = mapped_column(UUID(as_uuid=False), ForeignKey("exercise_logs.id", ondelete="CASCADE"), nullable=False)
    set_number: Mapped[int] = mapped_column(Integer, nullable=False)
    weight: Mapped[float] = mapped_column(Numeric, nullable=False)
    reps: Mapped[int] = mapped_column(Integer, nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    deleted_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class FoodLogRow(Base):
    __tablename__ = "food_logs"

    id: Mapped[str] = mapped_column(UUID(as_uuid=False), primary_key=True)
    user_id: Mapped[str] = mapped_column(UUID(as_uuid=False), ForeignKey("users.id", ondelete="CASCADE"), nullable=False)
    time: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    name: Mapped[str] = mapped_column(Text, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    deleted_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class FoodLogItemRow(Base):
    __tablename__ = "food_log_items"

    id: Mapped[str] = mapped_column(UUID(as_uuid=False), primary_key=True)
    log_id: Mapped[str] = mapped_column(UUID(as_uuid=False), ForeignKey("food_logs.id", ondelete="CASCADE"), nullable=False)
    food_id: Mapped[str | None] = mapped_column(Text, nullable=True)
    food_name: Mapped[str] = mapped_column(Text, nullable=False)
    serving_id: Mapped[str | None] = mapped_column(Text, nullable=True)
    quantity: Mapped[float | None] = mapped_column(Numeric, nullable=True)
    unit: Mapped[str | None] = mapped_column(Text, nullable=True)
    cal: Mapped[int] = mapped_column(Integer, nullable=False)
    fat: Mapped[float] = mapped_column(Numeric, nullable=False)
    carbs: Mapped[float] = mapped_column(Numeric, nullable=False)
    protein: Mapped[float] = mapped_column(Numeric, nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    deleted_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class SchemaMigrationRow(Base):
    """Tracks which migrations/*.sql files have been applied - the Python
    equivalent of goose's own bookkeeping table."""

    __tablename__ = "schema_migrations"

    filename: Mapped[str] = mapped_column(Text, primary_key=True)
    applied_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default="now()")
