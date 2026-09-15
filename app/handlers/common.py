"""Small shared helper for handlers - not present in the Go version (Go's
encoding/json handled this implicitly via `json.NewEncoder(writer).Encode(...)`),
but needed here since we're not routing every response through a pydantic
`response_model` and want full control over the exact bytes written, the
same way the Go handlers had full control by writing to `http.ResponseWriter`
directly.
"""

from __future__ import annotations

import json
from typing import Any

from fastapi import Response
from pydantic import BaseModel


def _to_jsonable(value: Any) -> Any:
    if isinstance(value, BaseModel):
        return value.model_dump(mode="json", by_alias=True)
    if isinstance(value, list):
        return [_to_jsonable(v) for v in value]
    if isinstance(value, dict):
        return {k: _to_jsonable(v) for k, v in value.items()}
    return value


def json_response(data: Any, status_code: int = 200) -> Response:
    return Response(content=json.dumps(_to_jsonable(data)), status_code=status_code, media_type="application/json")
