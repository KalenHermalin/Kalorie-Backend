"""Port of internal/models/llm.go

Kept synchronous (no async/await) on purpose - the Go code is plain
blocking code (each request runs on its own goroutine implicitly), and
FastAPI runs sync path functions in a thread pool automatically, which is
the closest match to that without introducing a concurrency style that
wasn't in the original.
"""

from __future__ import annotations

from abc import ABC, abstractmethod

from pydantic import BaseModel


class Macros(BaseModel):
    cal: int = 0
    fat: int = 0
    protein: int = 0
    carbs: int = 0


class LabelPayload(BaseModel):
    units: str = ""
    base_quantity: int = 0
    macros: Macros = Macros()


class MealPayload(BaseModel):
    food_name: str = ""
    cal: int = 0
    fat: int = 0
    protein: int = 0
    carbs: int = 0


class LLMProvider(ABC):
    @abstractmethod
    def analyze_label(self, picture: bytes, system_prompt: str, timeout: float | None = None) -> LabelPayload: ...

    @abstractmethod
    def analyze_picture(self, picture: bytes, system_prompt: str, timeout: float | None = None) -> MealPayload: ...
