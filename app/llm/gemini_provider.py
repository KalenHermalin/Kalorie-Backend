"""Port of internal/llm/gemini.go

Bug fix applied here: the Go version built a `ctx`/timeout in the handler
but then called `gm.client.Models.GenerateContent(context.Background(), ...)`
inside AnalyzePicture/AnalyzeLabel - ignoring the caller's deadline
entirely, so a slow/hung Gemini call could block indefinitely. Here the
handler's timeout is threaded all the way through and actually enforced
via `future.result(timeout=...)`.
"""

from __future__ import annotations

import json
import logging
from concurrent.futures import ThreadPoolExecutor
from typing import Optional

from google import genai
from google.genai import types

from app.models.llm import LabelPayload, LLMProvider, Macros, MealPayload

log = logging.getLogger(__name__)

_executor = ThreadPoolExecutor(max_workers=8)

_MEAL_SCHEMA = {
    "type": "OBJECT",
    "propertyOrdering": ["success", "payload"],
    "properties": {
        "success": {"type": "BOOLEAN"},
        "payload": {
            "type": "OBJECT",
            "propertyOrdering": ["food_name", "cal", "fat", "protein", "carbs"],
            "properties": {
                "food_name": {"type": "STRING", "description": "Simple name for the meal captured"},
                "cal": {"type": "INTEGER"},
                "fat": {"type": "INTEGER"},
                "protein": {"type": "INTEGER"},
                "carbs": {"type": "INTEGER"},
            },
        },
    },
}

_LABEL_SCHEMA = {
    "type": "OBJECT",
    "propertyOrdering": ["success", "payload"],
    "properties": {
        "success": {"type": "BOOLEAN"},
        "payload": {
            "type": "OBJECT",
            "propertyOrdering": ["units", "base_quantity", "macros"],
            "properties": {
                "units": {"type": "STRING"},
                "base_quantity": {"type": "INTEGER"},
                "macros": {
                    "type": "OBJECT",
                    "propertyOrdering": ["cal", "fat", "protein", "carbs"],
                    "properties": {
                        "cal": {"type": "INTEGER"},
                        "fat": {"type": "INTEGER"},
                        "protein": {"type": "INTEGER"},
                        "carbs": {"type": "INTEGER"},
                    },
                },
            },
        },
    },
}


class GeminiProvider(LLMProvider):
    def __init__(self, api_key: str, model: str):
        self.client = genai.Client(api_key=api_key)
        self.model = model

    # TODO: Seperate concerns, meaning build the genAI parts in sub function and return?
    def _generate(self, picture: bytes, system_prompt: str, response_schema: dict, timeout: Optional[float]) -> dict:
        user_parts = [types.Part(inline_data=types.Blob(data=picture, mime_type="image/jpeg"))]
        system_parts = []
        if system_prompt.strip():
            system_parts = [types.Part(text=system_prompt)]

        config = types.GenerateContentConfig(
            system_instruction=types.Content(parts=system_parts),
            response_mime_type="application/json",
            response_schema=response_schema,
            http_options=types.HttpOptions(timeout=int(timeout * 1000)) if timeout else None,
        )

        def call():
            return self.client.models.generate_content(
                model=self.model,
                contents=[types.Content(parts=user_parts)],
                config=config,
            )

        # Actually enforce the caller's deadline (the Go version didn't).
        future = _executor.submit(call)
        result = future.result(timeout=timeout)
        return json.loads(result.text)

    def analyze_picture(self, picture: bytes, system_prompt: str, timeout: Optional[float] = None) -> MealPayload:
        gemini_response = self._generate(picture, system_prompt, _MEAL_SCHEMA, timeout)
        if not gemini_response.get("success"):
            raise ValueError("No food found!")
        return MealPayload(**gemini_response["payload"])

    def analyze_label(self, picture: bytes, system_prompt: str, timeout: Optional[float] = None) -> LabelPayload:
        gemini_response = self._generate(picture, system_prompt, _LABEL_SCHEMA, timeout)
        if not gemini_response.get("success"):
            raise ValueError("No label found in image")
        payload = gemini_response["payload"]
        return LabelPayload(
            units=payload.get("units", ""),
            base_quantity=payload.get("base_quantity", 0),
            macros=Macros(**payload.get("macros", {})),
        )
