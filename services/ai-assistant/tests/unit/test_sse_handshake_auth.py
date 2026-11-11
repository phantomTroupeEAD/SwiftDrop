"""Unit tests verifying SSE handshake authentication middleware (SEC-CLD-006)."""

import pytest
from starlette.applications import Starlette
from starlette.responses import PlainTextResponse
from starlette.routing import Route
from starlette.testclient import TestClient

from src.mcp.server import SSEHandshakeAuthMiddleware


def dummy_endpoint(request):
    return PlainTextResponse("SSE Stream Connected")


app = Starlette(routes=[Route("/mcp", endpoint=dummy_endpoint, methods=["GET"])])
authed_app = SSEHandshakeAuthMiddleware(app)
client = TestClient(authed_app)


def test_sse_handshake_auth_missing_credentials_rejected():
    """GET /mcp without credentials must be rejected with 401 Unauthorized."""
    resp = client.get("/mcp")
    assert resp.status_code == 401
    assert "valid X-API-Key or Bearer token required" in resp.text


def test_sse_handshake_auth_invalid_api_key_rejected():
    """GET /mcp with malformed API key must be rejected with 401."""
    resp = client.get("/mcp", headers={"X-API-Key": "malformed_key_123"})
    assert resp.status_code == 401


def test_sse_handshake_auth_valid_api_key_accepted():
    """GET /mcp with canonical sd_live_* key must be accepted."""
    resp = client.get("/mcp", headers={"X-API-Key": "sd_live_demo_merchant_12345"})
    assert resp.status_code == 200
    assert resp.text == "SSE Stream Connected"


def test_sse_handshake_auth_valid_bearer_token_accepted():
    """GET /mcp with valid Bearer token must be accepted."""
    resp = client.get("/mcp", headers={"Authorization": "Bearer valid_jwt_token_here"})
    assert resp.status_code == 200
    assert resp.text == "SSE Stream Connected"
