"""Tests for the primary conversational chat endpoint (POST /api/v1/ai/chat)."""

import uuid
import pytest
from fastapi.testclient import TestClient

from src.core.cache import get_session_cache


def test_chat_endpoint_empty_prompt_rejected(client: TestClient):
    """Verify empty prompt is rejected with 422 Unprocessable Entity."""
    response = client.post("/api/v1/ai/chat", json={"prompt": ""})
    assert response.status_code == 422
    data = response.json()
    assert "error" in data
    assert data["error"]["code"] == "VALIDATION_ERROR"
    assert any("prompt" in str(d.get("field")) for d in data["error"]["details"])


def test_chat_endpoint_whitespace_only_rejected(client: TestClient):
    """Verify whitespace-only prompt is rejected with 422."""
    response = client.post("/api/v1/ai/chat", json={"prompt": "    \t\n   "})
    assert response.status_code == 422
    data = response.json()
    assert data["error"]["code"] == "VALIDATION_ERROR"


def test_chat_endpoint_oversized_prompt_rejected(client: TestClient):
    """Verify prompt > 1000 characters is rejected with 422."""
    oversized = "a" * 1001
    response = client.post("/api/v1/ai/chat", json={"prompt": oversized})
    assert response.status_code == 422
    data = response.json()
    assert data["error"]["code"] == "VALIDATION_ERROR"


def test_chat_endpoint_boundary_1000_char_prompt(client: TestClient):
    """Verify prompt with exactly 1000 characters succeeds with 200 OK."""
    prompt = "Package inquiry " + ("x" * 984)
    assert len(prompt) == 1000
    response = client.post("/api/v1/ai/chat", json={"prompt": prompt})
    assert response.status_code == 200
    data = response.json()
    assert "reply" in data
    assert data["grounded"] is True


def test_chat_endpoint_tracking_inquiry_grounded(client: TestClient):
    """Verify tracking inquiry triggers track_parcel_status tool and returns grounded response."""
    response = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "Where is my parcel SD-20261102-00001?"},
    )
    assert response.status_code == 200
    data = response.json()
    assert "reply" in data
    assert "SD-20261102-00001" in data["reply"]
    assert "track_parcel_status" in data["tools_called"]
    assert data["grounded"] is True
    assert data["latency_ms"] >= 0


def test_chat_endpoint_pricing_quote_with_action_card(client: TestClient):
    """Verify shipping fee query returns quotes and NAVIGATE_BOOKING action card."""
    response = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "How much does it cost to ship a 2.5kg package from Colombo to Kandy?"},
    )
    assert response.status_code == 200
    data = response.json()
    assert any(t in data["tools_called"] for t in ("estimate_shipping_fee", "estimate_delivery_fee"))
    assert len(data["actions"]) >= 1
    action = data["actions"][0]
    assert action["action_type"] == "NAVIGATE_BOOKING"
    assert "/portal/book" in action["target_url"]
    assert action["prefill_data"]["weight_kg"] == 2.5


def test_chat_endpoint_policy_inquiry(client: TestClient):
    """Verify policy inquiry triggers explain_company_policies tool."""
    response = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "What items are prohibited from delivery?"},
    )
    assert response.status_code == 200
    data = response.json()
    assert "explain_company_policies" in data["tools_called"]
    assert "prohibited" in data["reply"].lower() or "hazardous" in data["reply"].lower()


def test_chat_endpoint_auto_generates_conversation_id(client: TestClient):
    """Verify omitting conversation_id automatically generates a valid UUIDv4."""
    response = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "What are your operating hours?"},
    )
    assert response.status_code == 200
    data = response.json()
    # Should parse cleanly as a valid UUID
    conv_id = uuid.UUID(data["conversation_id"])
    assert conv_id.version == 4


@pytest.mark.asyncio
async def test_chat_endpoint_session_memory_continuity(client: TestClient):
    """Verify multi-turn session history is persisted and retrieved across calls with same conversation_id."""
    session_id = str(uuid.uuid4())
    cache = get_session_cache()

    # Turn 1
    res1 = client.post(
        "/api/v1/ai/chat",
        json={
            "conversation_id": session_id,
            "prompt": "I want to track package SD-20261102-00001",
        },
    )
    assert res1.status_code == 200

    # Verify session cache now contains Turn 1 (2 messages: user + assistant)
    history = await cache.get_history(session_id)
    assert len(history) == 2
    assert history[0].role == "user"
    assert "SD-20261102-00001" in history[0].content
    assert history[1].role == "assistant"

    # Turn 2 with same conversation_id
    res2 = client.post(
        "/api/v1/ai/chat",
        json={
            "conversation_id": session_id,
            "prompt": "What are your delivery policies?",
        },
    )
    assert res2.status_code == 200

    # Verify session cache now contains Turn 1 + Turn 2 (4 messages)
    history2 = await cache.get_history(session_id)
    assert len(history2) == 4


def test_chat_endpoint_with_explicit_client_history(client: TestClient):
    """Verify explicit client-supplied history is accepted and prioritized."""
    session_id = str(uuid.uuid4())
    history_payload = [
        {"role": "user", "content": "Hello!"},
        {"role": "assistant", "content": "Hello! How can I help you today?"},
    ]

    response = client.post(
        "/api/v1/ai/chat",
        json={
            "conversation_id": session_id,
            "prompt": "How much for a 1kg box?",
            "history": history_payload,
        },
    )
    assert response.status_code == 200
    assert response.json()["grounded"] is True


def test_chat_endpoint_with_bearer_token(client: TestClient):
    """Verify request with Authorization: Bearer token is parsed without error."""
    response = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "Show my active parcels"},
        headers={"Authorization": "Bearer mock-customer-jwt-token-xyz"},
    )
    assert response.status_code == 200
    data = response.json()
    assert "reply" in data


def test_cors_preflight_headers(client: TestClient):
    """Verify CORS preflight OPTIONS request returns 200 OK and proper headers."""
    response = client.options(
        "/api/v1/ai/chat",
        headers={
            "Origin": "http://localhost:5173",
            "Access-Control-Request-Method": "POST",
            "Access-Control-Request-Headers": "Content-Type,Authorization",
        },
    )
    assert response.status_code == 200
    assert response.headers.get("access-control-allow-origin") in ("*", "http://localhost:5173")
    assert "POST" in response.headers.get("access-control-allow-methods", "")
