"""Tests for the Unified Mock Upstream Server and tool integration routing."""

import pytest
from fastapi.testclient import TestClient

from scripts.mock_upstream_server import app as mock_app
from src.core.config import get_settings


@pytest.fixture
def mock_client():
    """Client for testing the mock upstream server directly."""
    return TestClient(mock_app)


def test_mock_tracking_parcel_found(mock_client: TestClient):
    """Verify mock tracking returns 200 OK and milestone data for seeded parcel."""
    response = mock_client.get("/api/v1/tracking/public/SD-20261102-00001")
    assert response.status_code == 200
    data = response.json()
    assert data["tracking_number"] == "SD-20261102-00001"
    assert data["status"] == "IN_TRANSIT"
    assert data["destination_city"] == "Colombo"
    assert len(data["milestones"]) >= 2
    # Check that mock includes raw PII (which AI Assistant must scrub)
    assert "recipient_phone" in data


def test_mock_tracking_parcel_not_found_404(mock_client: TestClient):
    """Verify mock tracking returns 404 for unknown tracking code."""
    response = mock_client.get("/api/v1/tracking/public/SD-99999999-99999")
    assert response.status_code == 404


def test_mock_pricing_quote_standard(mock_client: TestClient):
    """Verify mock pricing returns tariff in integer cents."""
    payload = {
        "weight_kg": 2.0,
        "pickup_postal_code": "00100",
        "delivery_postal_code": "20000",
        "service_type": "STANDARD",
    }
    response = mock_client.post("/api/v1/pricing/quotes", json=payload)
    assert response.status_code == 200
    data = response.json()
    assert data["currency"] == "LKR"
    # Base 35000 + (2.0 * 15000) = 65000 cents
    assert data["total_fee_cents"] == 65000
    assert data["estimated_transit_hours"] == 48


def test_mock_pricing_quote_express(mock_client: TestClient):
    """Verify mock pricing applies 1.5x multiplier for express."""
    payload = {
        "weight_kg": 2.0,
        "pickup_postal_code": "00100",
        "delivery_postal_code": "20000",
        "service_type": "EXPRESS",
    }
    response = mock_client.post("/api/v1/pricing/quotes", json=payload)
    assert response.status_code == 200
    data = response.json()
    # (35000 + 30000) * 1.5 = 97500 cents
    assert data["total_fee_cents"] == 97500
    assert data["estimated_transit_hours"] == 24


def test_mock_pricing_weight_limit_422(mock_client: TestClient):
    """Verify mock pricing rejects packages > 50kg."""
    payload = {
        "weight_kg": 55.0,
        "pickup_postal_code": "00100",
        "delivery_postal_code": "20000",
    }
    response = mock_client.post("/api/v1/pricing/quotes", json=payload)
    assert response.status_code == 422


def test_mock_parcels_unauthorized(mock_client: TestClient):
    """Verify mock parcels endpoint returns 401 without Bearer token."""
    response = mock_client.get("/api/v1/parcels")
    assert response.status_code == 401


def test_mock_parcels_authorized(mock_client: TestClient):
    """Verify mock parcels endpoint returns active shipments with valid Bearer token."""
    response = mock_client.get(
        "/api/v1/parcels",
        headers={"Authorization": "Bearer mock-customer-jwt-token"},
    )
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)
    assert len(data) >= 1
    assert data[0]["tracking_number"] == "SD-20261102-00001"


def test_mock_admin_settings_authorized(mock_client: TestClient):
    """Verify mock admin settings returns SLAs and prohibited items."""
    response = mock_client.get(
        "/api/v1/admin/settings",
        headers={"Authorization": "Bearer dev-internal-secret-key-123"},
    )
    assert response.status_code == 200
    data = response.json()
    assert "settings" in data
    assert "prohibited_items" in data["settings"]
    assert data["settings"]["standard_sla_hours"] == 48


def test_config_use_mock_upstream_routing(monkeypatch):
    """Verify Settings.core_api_base_url and tracking_api_base_url route to mock server when USE_MOCK_UPSTREAM=True."""
    monkeypatch.setenv("USE_MOCK_UPSTREAM", "true")
    monkeypatch.setenv("MOCK_CORE_API_URL", "http://127.0.0.1:8085")
    monkeypatch.setenv("MOCK_TRACKING_API_URL", "http://127.0.0.1:8085")
    monkeypatch.setenv("CORE_API_URL", "http://core-api:8080")
    monkeypatch.setenv("TRACKING_API_URL", "http://tracking-api:8081")

    get_settings.cache_clear()
    settings = get_settings()

    assert settings.USE_MOCK_UPSTREAM is True
    assert settings.core_api_base_url == "http://127.0.0.1:8085"
    assert settings.tracking_api_base_url == "http://127.0.0.1:8085"

    # Reset
    monkeypatch.setenv("USE_MOCK_UPSTREAM", "false")
    get_settings.cache_clear()
    default_settings = get_settings()
    assert default_settings.USE_MOCK_UPSTREAM is False
    assert default_settings.core_api_base_url == "http://core-api:8080"
    assert default_settings.tracking_api_base_url == "http://tracking-api:8081"
