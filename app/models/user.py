"""Port of internal/models/user.go"""

from __future__ import annotations

from abc import ABC, abstractmethod
from datetime import datetime, timezone
from typing import TYPE_CHECKING, Callable, List, Optional, TypeVar

from pydantic import BaseModel, ConfigDict, Field
from sqlalchemy.orm import Session

if TYPE_CHECKING:
    from app.models.auth import AuthPayload

# Go's zero value for `time.Time{}` - "0001-01-01T00:00:00Z". Used as the
# default for every field that was a plain `time.Time` (not `*time.Time`)
# in the Go structs: those can never be JSON `null` in Go, only ever a
# real (possibly zero-value) timestamp - unlike `deleted_at`, which was
# always a `*time.Time` and is genuinely nullable. Getting this wrong
# sends `null` for a field a strict client-side decoder expects to always
# be a valid date string, which is exactly the kind of thing that breaks
# decoding on the client with no server-side error to show for it.
GO_ZERO_TIME = datetime(1, 1, 1, tzinfo=timezone.utc)


class User(BaseModel):
    id: str = ""
    email: str = ""
    created_at: datetime = GO_ZERO_TIME


class UserSettings(BaseModel):
    units: str = ""
    calories_target: int = 0
    protein_target: int = 0
    carbs_target: int = 0
    fat_target: int = 0
    updated_at: datetime = GO_ZERO_TIME
    deleted_at: Optional[datetime] = None
    theme: str = ""


class WeightLog(BaseModel):
    id: str = ""
    weight_kg: float = 0
    log_date: datetime = GO_ZERO_TIME
    updated_at: datetime = GO_ZERO_TIME
    deleted_at: Optional[datetime] = None


class ExerciseLog(BaseModel):
    id: str = ""
    exercise_id: str = ""
    created_at: datetime = GO_ZERO_TIME
    updated_at: datetime = GO_ZERO_TIME
    deleted_at: Optional[datetime] = None


class ExerciseSet(BaseModel):
    id: str = ""
    log_id: str = ""
    set_number: int = 0
    weight: float = 0
    reps: int = 0
    updated_at: datetime = GO_ZERO_TIME
    deleted_at: Optional[datetime] = None


class FullExerciseLog(BaseModel):
    exercise_log: Optional[ExerciseLog] = None
    exercise_sets: List[ExerciseSet] = []


class FoodLog(BaseModel):
    id: str = ""
    name: str = ""
    # NOTE: this is a plain string in the Go model too (not a datetime),
    # despite backing a TIMESTAMPTZ column - kept as-is.
    Time: str = ""
    created_at: datetime = GO_ZERO_TIME
    updated_at: datetime = GO_ZERO_TIME
    deleted_at: Optional[datetime] = None


class FoodLogEntry(BaseModel):
    id: str = ""
    log_id: str = ""
    food_id: str = ""
    food_name: str = ""
    serving_id: str = ""
    unit: str = ""
    cal: int = 0
    fat: float = 0
    carbs: float = 0
    protein: float = 0
    quantity: float = 0
    updated_at: datetime = GO_ZERO_TIME
    deleted_at: Optional[datetime] = None


class FullFoodLog(BaseModel):
    model_config = ConfigDict(populate_by_name=True)

    food_log: Optional[FoodLog] = None
    # alias matches the Go json tag `json:"food-log_entries"`
    food_log_entries: List[FoodLogEntry] = Field(default_factory=list, alias="food-log_entries")


class Provider(BaseModel):
    id: str = ""
    name: str = ""
    user_id: int = 0


T = TypeVar("T")


class UserRepository(ABC):
    @abstractmethod
    def upsert_user_with_auth(self, session: Session, payload: "AuthPayload") -> User: ...

    @abstractmethod
    def delete_refresh_token(self, session: Session, token: str, user_id: str) -> None: ...

    @abstractmethod
    def save_refresh_token(self, session: Session, refresh: str, user_id: str, expires_at: datetime) -> None: ...

    @abstractmethod
    def find_refresh_token(self, session: Session, refresh: str, user_id: str) -> User: ...

    @abstractmethod
    def update_user_settings(self, session: Session, user_id: str, settings: UserSettings) -> None: ...

    @abstractmethod
    def get_user_settings(self, session: Session, user_id: str) -> UserSettings: ...

    @abstractmethod
    def upsert_user_weight_log(self, session: Session, user_id: str, payload: WeightLog) -> None: ...

    @abstractmethod
    def get_user_weight_log_by_id(self, session: Session, user_id: str, log_id: str) -> WeightLog: ...

    @abstractmethod
    def get_user_weight_logs(self, session: Session, user_id: str) -> List[WeightLog]: ...

    @abstractmethod
    def upsert_user_exercise_sets(self, session: Session, payload: List[ExerciseSet]) -> None: ...

    @abstractmethod
    def get_user_exercise_logs(self, session: Session, user_id: str) -> List[FullExerciseLog]: ...

    @abstractmethod
    def upsert_user_exercise_log(self, session: Session, user_id: str, payload: ExerciseLog) -> None: ...

    @abstractmethod
    def get_user_exercise_log_by_id(self, session: Session, user_id: str, log_id: str) -> ExerciseLog: ...

    @abstractmethod
    def get_user_exercise_sets_by_log_id(self, session: Session, log_id: str) -> List[ExerciseSet]: ...

    @abstractmethod
    def upsert_user_food_log(self, session: Session, user_id: str, payload: FoodLog) -> None: ...

    @abstractmethod
    def get_user_food_logs(self, session: Session, user_id: str) -> List[FullFoodLog]: ...

    @abstractmethod
    def get_user_food_log_by_id(self, session: Session, user_id: str, log_id: str) -> FoodLog: ...

    @abstractmethod
    def upsert_user_food_log_entry(self, session: Session, payload: List[FoodLogEntry]) -> None: ...

    @abstractmethod
    def get_user_food_log_entries_by_log_id(self, session: Session, log_id: str) -> List[FoodLogEntry]: ...

    @abstractmethod
    def with_tx(self, fn: Callable[[Session], T]) -> T: ...
