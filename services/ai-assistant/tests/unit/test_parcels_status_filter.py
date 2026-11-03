"""Unit tests verifying status query parameter synchronization (SEC-CLD-001) and API key validation (SEC-CLD-005)."""

import json
from typing import Any, Dict
from fastapi.testclient import TestClient
import httpx
from pydantic import SecretStr
import pytest

from scripts.mock_upstream_server import app as mock_app
from src.core.config import get_settings
from src.domain.tools import ActiveParcelsInput, ParcelStatus
from src.tools.parcels_tool import parcels_tool


class MockCaptureTransport(httpx.AsyncBaseTransport):
    """Transport that captures query parameters sent upstream."""

    def __init__(self):
        self.captured_params: Dict[str, str] = {}

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        self.captured_params = dict(request.url.params)
        return httpx.Response(
            status_code=200,
            content=json.dumps({"parcels": [], "count": 0}).encode("utf-8"),
            headers={"Content-Type": "application/json"},
            request=request,
        )


@pytest.mark.asyncio
async def test_parcels_status_filter_sent_in_query_params():
    """Verify that both status and status_filter are sent upstream."""
    transport = MockCaptureTransport()
    input_data = ActiveParcelsInput(status=ParcelStatus.OUT_FOR_DELIVERY, limit=10)

    async with httpx.AsyncClient(transport=transport) as client:
        await parcels_tool(
            input_data,
            context={"x-api-key": "sd_live_valid_key_123"},
            client=client,
        )

    # SEC-CLD-001: Verify both query parameter aliases are populated
    assert transport.captured_params.get("status") == "OUT_FOR_DELIVERY"
    assert transport.captured_params.get("status_filter") == "OUT_FOR_DELIVERY"
    assert transport.captured_params.get("limit") == "10"


def test_mock_upstream_parcels_status_filter_respected():
    """Verify mock upstream filters parcels accurately when status_filter is supplied."""
    client = TestClient(mock_app)
    headers = {"X-API-Key": "sd_live_merchant_test_12345"}

    # 1. Query for OUT_FOR_DELIVERY
    resp = client.get("/api/v1/parcels?status_filter=OUT_FOR_DELIVERY", headers=headers)
    assert resp.status_code == 200
    parcels = resp.json()
    assert len(parcels) == 1
    assert parcels[0]["status"] == "OUT_FOR_DELIVERY"

    # 2. Query for IN_TRANSIT
    resp_transit = client.get("/api/v1/parcels?status_filter=IN_TRANSIT", headers=headers)
    assert resp_transit.status_code == 200
    transit_parcels = resp_transit.json()
    assert len(transit_parcels) == 1
    assert transit_parcels[0]["status"] == "IN_TRANSIT"


def test_mock_upstream_api_key_format_validation():
    """Verify mock upstream enforces canonical API key patterns (SEC-CLD-005)."""
    client = TestClient(mock_app)

    # 1. Arbitrary invalid key string must be rejected
    resp_invalid = client.get("/api/v1/parcels", headers={"X-API-Key": "invalid_key_999"})
    assert resp_invalid.status_code == 401
    assert "Invalid or unrecognized X-API-Key format" in resp_invalid.text

    # 2. Valid canonical live key must be accepted
    resp_valid_live = client.get("/api/v1/parcels", headers={"X-API-Key": "sd_live_acme_corp_12345678"})
    assert resp_valid_live.status_code == 200

    # 3. Valid canonical test key must be accepted
    resp_valid_test = client.get("/api/v1/parcels", headers={"X-API-Key": "sd_test_sandbox_12345678"})
    assert resp_valid_test.status_code == 200
