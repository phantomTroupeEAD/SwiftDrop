"""Cross-module end-to-end integration workflows verifying multi-service interaction contracts."""

from fastapi.testclient import TestClient
import pytest

from src.api.middleware import create_jwt_token
from src.main import app


def test_e2e_workflow_town_name_to_fee_quote_and_booking_card():
    """Verify natural language quote with non-district towns generates clean integer LKR and booking card."""
    client = TestClient(app)
    response = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "How much to ship a 4.5kg package from Walasmulla to Borella?"},
    )
    assert response.status_code == 200
    data = response.json()

    # 1. Reply must be grounded with resolved postal codes and clean integer rupees (no cents)
    reply = data["reply"]
    assert "82450" in reply or "Walasmulla" in reply
    assert "00800" in reply or "Borella" in reply
    assert "LKR" in reply
    assert ".00" not in reply  # Clean integer Rupees per Sri Lankan commerce

    # 2. Action card verification
    assert len(data["actions"]) == 1
    action = data["actions"][0]
    assert action["action_type"] == "NAVIGATE_BOOKING"
    assert action["target_url"] == "/portal/book"

    prefill = action["prefill_data"]
    assert prefill["pickup_postal_code"] == "82450"
    assert prefill["delivery_postal_code"] == "00800"
    assert prefill["weight_kg"] == 4.5
    assert "estimated_fee" in prefill
    assert isinstance(prefill["estimated_fee"], int)
    assert "estimated_fee_cents" in prefill


def test_e2e_workflow_public_tracking_milestones():
    """Verify tracking inquiry calls tracking tool and returns milestone history with SEC-009 privacy."""
    client = TestClient(app)
    response = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "Track my parcel SD-20261102-00001"},
    )
    assert response.status_code == 200
    data = response.json()

    reply = data["reply"]
    assert "SD-20261102-00001" in reply
    # Must not leak personal recipient phone number or street address
    assert "+9477" not in reply
    assert "Street" not in reply
    assert data["grounded"] is True


def test_e2e_workflow_authenticated_active_parcels():
    """Verify authenticated user can retrieve their active parcel list using a valid JWT."""
    client = TestClient(app)
    token = create_jwt_token(
        user_id="018f4a7c-1005-7000-8000-000000000005",
        role="CUSTOMER",
        email="customer@swiftdrop.local",
    )

    response = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "What are my current active packages?"},
        headers={"Authorization": f"Bearer {token}"},
    )
    assert response.status_code == 200
    data = response.json()
    assert "active" in data["reply"].lower() or "shipment" in data["reply"].lower()
    assert data["grounded"] is True


def test_e2e_workflow_company_policies_and_prohibited_goods():
    """Verify company policies query grounds transit SLA times and restricted goods list."""
    client = TestClient(app)
    response = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "What items are prohibited and what are your standard delivery SLA times?"},
    )
    assert response.status_code == 200
    data = response.json()
    reply = data["reply"]

    # Must contain SLA or prohibited items grounding
    assert any(sla in reply.lower() for sla in ["24", "48", "hours", "sla", "transit", "prohibited", "items", "explosives"])
    assert data["grounded"] is True


def test_e2e_workflow_multi_turn_session_memory():
    """Verify conversation_id persists entities across multiple turns without asking user to repeat."""
    client = TestClient(app)
    conv_id = "3fa85f64-5717-4562-b3fc-2c963f66afa7"

    # Turn 1: Inquire about a specific parcel
    resp1 = client.post(
        "/api/v1/ai/chat",
        json={
            "prompt": "Where is my parcel SD-20261102-00001?",
            "conversation_id": conv_id,
        },
    )
    assert resp1.status_code == 200
    assert "SD-20261102-00001" in resp1.json()["reply"]

    # Turn 2: Follow-up question referring to "it"
    resp2 = client.post(
        "/api/v1/ai/chat",
        json={
            "prompt": "When will it arrive at its destination?",
            "conversation_id": conv_id,
        },
    )
    assert resp2.status_code == 200
    # Must recall the tracking number from turn 1 rather than asking the user to provide it again
    assert "SD-20261102-00001" in resp2.json()["reply"]
