"""Security, resilience, and injection penetration tests for SSE streaming."""

import json
import pytest
from fastapi.testclient import TestClient

from src.api.middleware.rate_limit import limiter
from tests.api.test_api_chat_stream import parse_sse_events


@pytest.fixture(autouse=True)
def reset_rate_limiter():
    """Reset SlowAPI rate limiter state before each test."""
    limiter.reset()
    yield
    limiter.reset()


def test_sse_unicode_and_newlines_integrity(client: TestClient):
    """Verify special characters, newlines, and Sinhala/Tamil text are properly framed (EC-8, EC-9)."""
    sinhala_tamil_prompt = "Hello! කරුණාකර උදව් කරන්න / தயவுசெய்து உதවவும்: SD-20261021-04921"
    response = client.post(
        "/api/v1/ai/chat/stream",
        json={"prompt": sinhala_tamil_prompt},
    )
    assert response.status_code == 200
    events = parse_sse_events(response.text)
    assert len(events) > 0
    # Confirm stream terminated cleanly with DONE event
    assert events[-1]["event"] == "done"


def test_sse_pii_phone_number_redaction(client: TestClient):
    """Verify customer phone numbers are masked in streamed token deltas (EC-10)."""
    prompt_with_phone = "Please call recipient on +94 77 123 4567 regarding SD-20261021-04921"
    response = client.post(
        "/api/v1/ai/chat/stream",
        json={"prompt": prompt_with_phone},
    )
    assert response.status_code == 200
    # Assert raw phone number is never emitted in stream
    assert "+94 77 123 4567" not in response.text


def test_sse_rate_limit_exceeded_returns_429_json(client: TestClient):
    """Verify rate limit exhaustion on /chat/stream returns 429 RFC-7807 JSON (EC-4)."""
    # Exhaust anonymous rate limit (20 RPM)
    for _ in range(20):
        client.post(
            "/api/v1/ai/chat/stream",
            json={"prompt": "Ping"},
            headers={"X-Forwarded-For": "198.51.100.55"},
        )

    # 21st request should be rate-limited
    res_blocked = client.post(
        "/api/v1/ai/chat/stream",
        json={"prompt": "Blocked stream attempt"},
        headers={"X-Forwarded-For": "198.51.100.55"},
    )
    assert res_blocked.status_code == 429
    assert "application/json" in res_blocked.headers["content-type"]
    data = res_blocked.json()
    assert data["error"]["code"] == "RATE_LIMIT_EXCEEDED"
