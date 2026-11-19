"""Tests for XML text-based tool call parsing, tag sanitization, and Core/Tracking API payload alignment."""

import asyncio
import json
import httpx
import pytest

from src.core.postal_codes import lookup_postal_record, resolve_postal_code
from src.domain.tools import EstimateFeeInput, ServiceType, TrackParcelInput, ActiveParcelsInput
from src.llm.openai_adapter import parse_text_tool_calls, strip_tool_call_tags
from src.tools.parcels_tool import parcels_tool
from src.tools.pricing_tool import pricing_tool, postal_to_default_zone
from src.tools.tracking_tool import mask_tracking_data, track_parcel_tool


def test_parse_nemotron_xml_tool_call():
    """Verify parsing Nemotron-style XML function calls with typed parameters."""
    raw_xml = """<tool_call>
<function=estimate_delivery_fee>
<parameter=weight_kg>
4
</parameter>
<parameter=pickup_postal_code>
50000
</parameter>
<parameter=delivery_postal_code>
11850
</parameter>
<parameter=service_type>
STANDARD
</parameter>
</function>
</tool_call>"""

    parsed = parse_text_tool_calls(raw_xml)
    assert len(parsed) == 1
    call = parsed[0]
    assert call["name"] == "estimate_delivery_fee"
    args = call["arguments"]
    assert args["weight_kg"] == 4 or args["weight_kg"] == 4.0
    assert args["pickup_postal_code"] == "50000"
    assert args["delivery_postal_code"] == "11850"
    assert args["service_type"] == "STANDARD"


def test_parse_json_tool_call_in_xml():
    """Verify parsing JSON object wrapped inside <tool_call> tags."""
    raw_text = '<tool_call>{"name": "track_parcel_status", "arguments": {"tracking_number": "SD-20261021-04921"}}</tool_call>'
    parsed = parse_text_tool_calls(raw_text)
    assert len(parsed) == 1
    assert parsed[0]["name"] == "track_parcel_status"
    assert parsed[0]["arguments"]["tracking_number"] == "SD-20261021-04921"


def test_strip_tool_call_tags():
    """Verify raw XML tool call tags are stripped and do not leak into customer-facing text."""
    dirty_text = (
        "Here is your rate:\n"
        "<tool_call>\n<function=estimate_delivery_fee>\n<parameter=weight_kg>\n4\n</parameter>\n</function>\n</tool_call>\n"
        "Please let me know if you would like to proceed."
    )
    cleaned = strip_tool_call_tags(dirty_text)
    assert "<tool_call>" not in cleaned
    assert "</tool_call>" not in cleaned
    assert "<function=" not in cleaned
    assert "Here is your rate:" in cleaned
    assert "Please let me know if you would like to proceed." in cleaned


def test_postal_code_lookup_hunnasgiriya_and_kahatagasdigiliya():
    """Verify newly added towns Hunnasgiriya and Kahatagasdigiliya resolve accurately."""
    # Hunnasgiriya (Kandy District)
    hunnas = lookup_postal_record("Hunnasgiriya")
    assert hunnas is not None
    assert hunnas["code"] == "20948"
    assert hunnas["district"] == "Kandy"
    assert resolve_postal_code("Hunnasgiriya") == "20948"

    # Kahatagasdigiliya (Anuradhapura District)
    kahatagas = lookup_postal_record("Kahatagasdigiliya")
    assert kahatagas is not None
    assert kahatagas["code"] == "50130"
    assert kahatagas["district"] == "Anuradhapura"
    assert resolve_postal_code("Kahatagasdigiliya") == "50130"


def test_tracking_tool_with_wrapped_data_envelope():
    """Verify tracking tool unwrap {"data": {...}} envelope from Tracking API."""
    async def _test():
        async def mock_handler(request: httpx.Request) -> httpx.Response:
            # Wrapped in single data envelope per Member 5 contract
            payload = {
                "data": {
                    "tracking_number": "SD-20261021-04921",
                    "status": "IN_TRANSIT",
                    "status_description": "In transit between Anuradhapura and Kadawatha",
                    "destination": {"city": "Kadawatha"},
                    "estimated_delivery": "2026-10-23T14:00:00Z",
                    "milestones": [
                        {
                            "status": "PICKED_UP",
                            "timestamp": "2026-10-21T09:30:00Z",
                            "location": "Anuradhapura Hub",
                            "description": "Package received at hub",
                        }
                    ],
                }
            }
            return httpx.Response(200, json=payload)

        transport = httpx.MockTransport(mock_handler)
        async with httpx.AsyncClient(transport=transport) as client:
            inp = TrackParcelInput(tracking_number="SD-20261021-04921")
            result = await track_parcel_tool(inp, client=client)

            assert result["found"] is True
            assert result["tracking_number"] == "SD-20261021-04921"
            assert result["status"] == "IN_TRANSIT"
            assert result["destination_city"] == "Kadawatha"
            assert len(result["milestones"]) == 1
            assert result["grounded"] is True

    asyncio.run(_test())


def test_pricing_tool_core_api_structure_and_envelope():
    """Verify pricing tool correctly handles Core API CreateQuoteRequest schema and unwraps data envelope."""
    async def _test():
        async def mock_handler(request: httpx.Request) -> httpx.Response:
            body = json.loads(request.content)
            # Must contain Core API required fields
            assert "origin_zone_id" in body
            assert "destination_zone_id" in body
            assert "actual_weight_kg" in body
            assert "dimensions" in body
            assert "service_level" in body

            return httpx.Response(
                201,
                json={
                    "data": {
                        "quote_id": "QUO-20261021-8899",
                        "fee_cents": 75000,
                        "currency": "LKR",
                        "chargeable_weight_kg": 4.0,
                        "expires_at": "2026-10-21T18:00:00Z",
                        "estimated_transit_hours": 48,
                    }
                },
            )

        transport = httpx.MockTransport(mock_handler)
        async with httpx.AsyncClient(transport=transport) as client:
            inp = EstimateFeeInput(
                weight_kg=4.0,
                pickup_postal_code="50000",
                delivery_postal_code="11850",
                service_type=ServiceType.STANDARD,
            )
            result = await pricing_tool(inp, client=client)

            assert result["total_fee_cents"] == 75000
            assert result["formatted_display"] == "LKR 750"
            assert result["quote_id"] == "QUO-20261021-8899"
            assert result["grounded"] is True
            assert result["action"]["action_type"] == "NAVIGATE_BOOKING"

    asyncio.run(_test())


def test_parcels_tool_merchant_orders_and_data_envelope():
    """Verify parcels tool handles merchant orders endpoint and unwraps data envelope."""
    async def _test():
        async def mock_handler(request: httpx.Request) -> httpx.Response:
            assert "/api/v1/merchant/orders" in str(request.url)
            assert request.headers.get("X-API-Key") == "sd_live_test_merchant_key"
            payload = {
                "data": [
                    {
                        "id": "ord-001",
                        "tracking_number": "SD-20261021-04921",
                        "status": "IN_TRANSIT",
                        "recipient_city": "Kadawatha",
                        "service_type": "STANDARD",
                        "created_at": "2026-10-21T10:00:00Z",
                    }
                ],
                "meta": {"page": 1, "per_page": 1, "total_items": 1, "total_pages": 1},
            }
            return httpx.Response(200, json=payload)

        transport = httpx.MockTransport(mock_handler)
        async with httpx.AsyncClient(transport=transport) as client:
            inp = ActiveParcelsInput(limit=5)
            result = await parcels_tool(
                inp,
                context={"api_key": "sd_live_test_merchant_key", "role": "MERCHANT"},
                client=client,
            )
            assert result["count"] == 1
            assert len(result["parcels"]) == 1
            assert result["parcels"][0]["tracking_number"] == "SD-20261021-04921"
            assert result["parcels"][0]["recipient_city"] == "Kadawatha"
            assert result["grounded"] is True

    asyncio.run(_test())
