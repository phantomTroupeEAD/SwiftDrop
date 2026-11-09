"""Unit tests verifying volumetric pricing calculation and upstream error sanitization in pricing_tool."""

import json
from typing import Any, Dict
import httpx
import pytest

from src.domain.errors import UpstreamServiceException
from src.domain.tools import EstimateFeeInput, ServiceType
from src.tools.pricing_tool import pricing_tool


class MockPricingTransport(httpx.AsyncBaseTransport):
    """Transport simulating Core API Pricing responses."""

    def __init__(self, status_code: int = 200, response_data: Dict[str, Any] = None):
        self.status_code = status_code
        self.response_data = response_data or {
            "base_fee_cents": 35000,
            "weight_fee_cents": 37500,
            "total_fee_cents": 72500,
            "currency": "LKR",
            "estimated_transit_hours": 48,
        }
        self.captured_payload: Dict[str, Any] = {}

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        self.captured_payload = json.loads(request.content.decode("utf-8"))
        return httpx.Response(
            status_code=self.status_code,
            content=json.dumps(self.response_data).encode("utf-8"),
            headers={"Content-Type": "application/json"},
            request=request,
        )


@pytest.mark.asyncio
async def test_pricing_volumetric_billable_weight_sent_upstream():
    """When volumetric weight exceeds gross weight, billable weight is used."""
    # Gross: 2.0 kg. Dimensions: 40x40x40 cm -> Volumetric: (40*40*40)/5000 = 12.8 kg
    input_data = EstimateFeeInput(
        weight_kg=2.0,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
        length_cm=40.0,
        width_cm=40.0,
        height_cm=40.0,
    )

    transport = MockPricingTransport()
    async with httpx.AsyncClient(transport=transport) as client:
        result = await pricing_tool(input_data, client=client)

    # 1. Verify billable weight of 12.8 kg was sent to Core API
    assert transport.captured_payload["weight_kg"] == 12.8
    assert transport.captured_payload["dimensions"]["length_cm"] == 40.0

    # 2. Verify result returns grounded quote
    assert result["grounded"] is True
    assert result["estimated_fee"] == 725


@pytest.mark.asyncio
async def test_pricing_gross_weight_used_when_exceeding_volumetric():
    """When gross weight exceeds volumetric weight, gross weight is used."""
    # Gross: 10.0 kg. Dimensions: 20x20x10 cm -> Volumetric: 4000/5000 = 0.8 kg
    input_data = EstimateFeeInput(
        weight_kg=10.0,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
        length_cm=20.0,
        width_cm=20.0,
        height_cm=10.0,
    )

    transport = MockPricingTransport()
    async with httpx.AsyncClient(transport=transport) as client:
        await pricing_tool(input_data, client=client)

    assert transport.captured_payload["weight_kg"] == 10.0


@pytest.mark.asyncio
async def test_pricing_upstream_422_sanitized_without_raw_json():
    """Upstream HTTP 422 validation errors must be formatted cleanly without raw trace JSON."""
    raw_upstream_422 = {
        "detail": [
            {
                "type": "greater_than_equal",
                "loc": ["body", "weight_kg"],
                "msg": "Input should be greater than or equal to 0.01",
                "input": 0.001,
                "ctx": {"ge": 0.01},
            }
        ]
    }
    transport = MockPricingTransport(status_code=422, response_data=raw_upstream_422)

    input_data = EstimateFeeInput(
        weight_kg=0.01,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
    )

    async with httpx.AsyncClient(transport=transport) as client:
        with pytest.raises(UpstreamServiceException) as exc_info:
            await pricing_tool(input_data, client=client)

    err_str = str(exc_info.value)
    # 1. Contains human-readable validation summary
    assert "Validation error: body.weight_kg: Input should be greater than or equal to 0.01" in err_str
    # 2. Does NOT contain raw JSON braces or unparsed trace dumps
    assert '{"detail":' not in err_str
