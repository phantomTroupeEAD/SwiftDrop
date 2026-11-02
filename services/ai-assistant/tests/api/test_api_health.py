"""Tests for health, monitoring, and OpenAPI documentation endpoints."""

import pytest
from fastapi.testclient import TestClient


def test_liveness_probe_returns_200(client: TestClient):
    """Verify /health/live returns 200 OK and UP status."""
    response = client.get("/health/live")
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "UP"
    assert data["service"] == "ai-assistant"
    assert "timestamp" in data


def test_readiness_probe_returns_200(client: TestClient):
    """Verify /health/ready returns 200 OK and readiness metadata."""
    response = client.get("/health/ready")
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "READY"
    assert "engine" in data
    assert "active_sessions" in data
    assert data["registered_tools"] >= 4
    assert "timestamp" in data


def test_metrics_endpoint_returns_200(client: TestClient):
    """Verify /health/metrics returns operational telemetry."""
    response = client.get("/health/metrics")
    assert response.status_code == 200
    data = response.json()
    assert data["service"] == "ai-assistant"
    assert data["version"] == "2.5.1"
    assert data["uptime_seconds"] >= 0
    assert "active_sessions" in data
    assert "max_capacity" in data
    assert data["registered_tools"] >= 4


def test_root_greeting_returns_200(client: TestClient):
    """Verify GET / returns service information and navigation links."""
    response = client.get("/")
    assert response.status_code == 200
    data = response.json()
    assert data["service"] == "SwiftDrop AI Delivery Assistant & FastMCP Service"
    assert data["docs_url"] == "/docs"
    assert data["health_live_url"] == "/health/live"
    assert data["chat_url"] == "/api/v1/ai/chat"


def test_openapi_schema_endpoint(client: TestClient):
    """Verify OpenAPI 3.1.0 schema is served at /openapi.json."""
    response = client.get("/openapi.json")
    assert response.status_code == 200
    schema = response.json()
    assert schema["info"]["title"] == "SwiftDrop AI Delivery Assistant & FastMCP Service"
    assert "/api/v1/ai/chat" in schema["paths"]
    assert "/health/live" in schema["paths"]
    assert "/health/ready" in schema["paths"]


def test_docs_ui_endpoints(client: TestClient):
    """Verify interactive Swagger UI and ReDoc return 200 OK."""
    swagger_res = client.get("/docs")
    assert swagger_res.status_code == 200
    assert "html" in swagger_res.headers.get("content-type", "").lower()

    redoc_res = client.get("/redoc")
    assert redoc_res.status_code == 200
    assert "html" in redoc_res.headers.get("content-type", "").lower()
