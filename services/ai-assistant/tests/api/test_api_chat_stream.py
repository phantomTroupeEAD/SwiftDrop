"""Integration tests for POST /api/v1/ai/chat/stream SSE endpoint."""

import json
from unittest.mock import patch
import uuid
import pytest
from fastapi.testclient import TestClient

from src.api.middleware.auth import create_jwt_token
from src.api.middleware.rate_limit import limiter
from src.main import app


def parse_sse_events(raw_text: str):
    """Parse raw SSE wire text into structured list of dicts with event and data."""
    events = []
    current_event = None
    current_data = []

    for line in raw_text.splitlines():
        line = line.strip()
        if not line:
            if current_event and current_data:
                try:
                    parsed_json = json.loads("".join(current_data))
                except json.JSONDecodeError:
                    parsed_json = "".join(current_data)
                events.append({"event": current_event, "data": parsed_json})
            current_event = None
            current_data = []
            continue

        if line.startswith("event:"):
            current_event = line[len("event:") :].strip()
        elif line.startswith("data:"):
            current_data.append(line[len("data:") :].strip())

    if current_event and current_data:
        try:
            parsed_json = json.loads("".join(current_data))
        except json.JSONDecodeError:
            parsed_json = "".join(current_data)
        events.append({"event": current_event, "data": parsed_json})

    return events


def test_sse_chat_stream_happy_path(client: TestClient):
    """Verify POST /api/v1/ai/chat/stream returns valid W3C SSE frames with correct headers."""
    response = client.post(
        "/api/v1/ai/chat/stream",
        json={"prompt": "Track parcel SD-20261021-04921"},
    )
    assert response.status_code == 200
    assert "text/event-stream" in response.headers["content-type"]
    assert response.headers.get("x-accel-buffering") == "no"
    assert "no-cache" in response.headers.get("cache-control", "")
    assert "X-Correlation-ID" in response.headers

    events = parse_sse_events(response.text)
    event_names = [e["event"] for e in events]

    assert "tool_start" in event_names
    assert "tool_end" in event_names
    assert "token" in event_names
    assert "done" in event_names

    # Check tool_start payload
    tool_start = next(e for e in events if e["event"] == "tool_start")
    assert tool_start["data"]["tool"] == "track_parcel_status"

    # Check done event
    done = next(e for e in events if e["event"] == "done")
    assert "SD-20261021-04921" in response.text


def test_sse_chat_stream_action_card(client: TestClient):
    """Verify quote inquiries yield action_card events containing NAVIGATE_BOOKING card."""
    response = client.post(
        "/api/v1/ai/chat/stream",
        json={"prompt": "How much for a 3kg parcel from 00100 to 20000?"},
    )
    assert response.status_code == 200
    events = parse_sse_events(response.text)
    event_names = [e["event"] for e in events]

    assert "action_card" in event_names
    action_event = next(e for e in events if e["event"] == "action_card")
    assert action_event["data"]["action_type"] == "NAVIGATE_BOOKING"
    assert action_event["data"]["target_url"] == "/portal/book"
    assert action_event["data"]["prefill_data"]["pickup_postal_code"] == "00100"


def test_sse_validation_failure_returns_422_json(client: TestClient):
    """Verify invalid payloads return 422 JSON RFC-7807 before stream starts (EC-6)."""
    response = client.post(
        "/api/v1/ai/chat/stream",
        json={"prompt": ""},  # Empty prompt
    )
    assert response.status_code == 422
    assert "application/json" in response.headers["content-type"]
    data = response.json()
    assert data["error"]["code"] == "VALIDATION_ERROR"


def test_sse_expired_token_returns_401_json(client: TestClient):
    """Verify expired JWT tokens return 401 JSON RFC-7807 before stream starts (EC-5)."""
    expired_token = create_jwt_token(
        user_id="usr_expired_1",
        expires_in_seconds=-100,
    )
    response = client.post(
        "/api/v1/ai/chat/stream",
        json={"prompt": "Where are my packages?"},
        headers={"Authorization": f"Bearer {expired_token}"},
    )
    assert response.status_code == 401
    assert "application/json" in response.headers["content-type"]
    data = response.json()
    assert data["error"]["code"] == "AUTH_UNAUTHORIZED"


def test_sse_cross_protocol_session_continuity(client: TestClient):
    """Verify session continuity when Turn 1 is JSON (/chat) and Turn 2 is SSE (/chat/stream) (EC-7)."""
    conv_id = str(uuid.uuid4())

    # Turn 1: Standard JSON query about tracking number
    res1 = client.post(
        "/api/v1/ai/chat",
        json={
            "prompt": "Where is my parcel SD-20261021-04921?",
            "conversation_id": conv_id,
        },
    )
    assert res1.status_code == 200

    # Turn 2: SSE Stream follow-up referencing "it" without mentioning the tracking number
    res2 = client.post(
        "/api/v1/ai/chat/stream",
        json={
            "prompt": "Where is it going?",
            "conversation_id": conv_id,
        },
    )
    assert res2.status_code == 200
    events = parse_sse_events(res2.text)
    token_text = "".join(e["data"]["delta"] for e in events if e["event"] == "token")

    # Assistant should recall the previous tracking number from cache
    assert "SD-20261021-04921" in token_text or "destined" in token_text.lower()
