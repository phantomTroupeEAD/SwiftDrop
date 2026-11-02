"""Shared pytest fixtures and test configuration for SwiftDrop AI Assistant."""

import os
import pytest

@pytest.fixture(autouse=True)
def setup_test_environment(monkeypatch):
    """Ensure tests run with offline mock LLM mode and test service URLs."""
    monkeypatch.setenv("ENVIRONMENT", "test")
    monkeypatch.setenv("MOCK_LLM_MODE", "true")
    monkeypatch.setenv("CORE_API_URL", "http://test-core-api:8080")
    monkeypatch.setenv("TRACKING_API_URL", "http://test-tracking-api:8081")
    monkeypatch.setenv("AI_SERVICE_ROLE_TOKEN", "test-internal-token-123")
    monkeypatch.setenv("AI_RATE_LIMIT_RPM", "100")
