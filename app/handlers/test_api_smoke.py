"""Not a port of anything in the Go version - a small smoke test added
purely to verify the FastAPI app factory (app/api.py's `mount()`) wires
routes/dependencies correctly, without needing a real database or any
external services.
"""

from fastapi.testclient import TestClient

from app.api import Application, Config
from app.handlers.auth_handler import AuthHandler
from app.handlers.llm_handler import LLMHandler
from app.handlers.system_handler import SystemHandler
from app.handlers.user_handler import UserHandler


def _build_app():
    application = Application(
        config=Config(addr="0.0.0.0:8080", jwt_access_secret="test-secret", jwt_refresh_secret="test-refresh-secret"),
        auth_handler=AuthHandler(auth_service=None),  # type: ignore[arg-type]
        llm_handler=LLMHandler(llm_service=None),  # type: ignore[arg-type]
        system_handler=SystemHandler(),
        user_handler=UserHandler(user_service=None),  # type: ignore[arg-type]
    )
    return application.mount()


def test_health_check_is_public():
    client = TestClient(_build_app())
    resp = client.get("/system/health")
    assert resp.status_code == 200
    assert resp.text == "Ok!"


def test_protected_route_requires_authorization_header():
    client = TestClient(_build_app())
    resp = client.get("/v1/settings/sync?last_synced_at=2024-01-01T00:00:00Z")
    assert resp.status_code == 401
    assert resp.json()["code"] == "ERR_UNAUTHORIZED"


def test_protected_route_rejects_malformed_bearer_header():
    client = TestClient(_build_app())
    resp = client.get("/v1/settings/sync", headers={"Authorization": "NotBearer abc"})
    assert resp.status_code == 400
    assert resp.json()["code"] == "ERR_INVALID_TOKEN_FORMAT"


def test_login_route_is_public_and_rejects_bad_body():
    client = TestClient(_build_app())
    resp = client.post("/auth/login", content=b"not json")
    assert resp.status_code == 400
    assert resp.json()["code"] == "ERR_INVALID_REQUEST"
