"""Shared pytest fixtures and test configuration for SwiftDrop AI Assistant."""

import os
import pytest
from fastapi.testclient import TestClient

from src.core.cache import get_session_cache
from src.main import app


@pytest.fixture(autouse=True)
def setup_test_environment(monkeypatch):
    """Ensure tests run with offline mock LLM mode and test service URLs."""
    monkeypatch.setenv("ENVIRONMENT", "test")
    monkeypatch.setenv("MOCK_LLM_MODE", "true")
    monkeypatch.setenv("CORE_API_URL", "http://test-core-api:8080")
    monkeypatch.setenv("TRACKING_API_URL", "http://test-tracking-api:8081")
    monkeypatch.setenv("AI_SERVICE_ROLE_TOKEN", "test-internal-token-123")
    monkeypatch.setenv("AI_RATE_LIMIT_RPM", "100")


@pytest.fixture(autouse=True)
async def reset_session_cache():
    """Clear session cache before and after each test to ensure test isolation."""
    cache = get_session_cache()
    await cache.clear_all()
    yield
    await cache.clear_all()


@pytest.fixture
def client():
    """Provide a synchronous FastAPI TestClient."""
    return TestClient(app)
