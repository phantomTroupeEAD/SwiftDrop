"""Comprehensive unit and mock integration tests for approved read-only tools and registry."""

import asyncio
import json
from typing import Any, Dict
import httpx
import pytest

from src.core.config import get_settings
from src.domain.chat import ToolCallRecord
from src.domain.errors import AuthException, UpstreamServiceException
from src.domain.tools import (
    ActiveParcelsInput,
    CompanyPolicyInput,
    EstimateFeeInput,
    ParcelStatus,
    ServiceType,
    TrackParcelInput,
)
from src.tools import (
    clear_registry,
    execute_tool,
    export_openai_tools,
    get_registered_tools,
    mask_tracking_data,
    parcels_tool,
    policy_tool,
    pricing_tool,
    sanitize_parcel_item,
    track_parcel_tool,
)


def test_tool_registry_registration_and_export():
    """Verify that all four approved tools are registered with schemas and read-only status."""
    tools = get_registered_tools()
    expected_tools = {
        "track_parcel_status",
        "estimate_delivery_fee",
        "list_active_parcels",
        "explain_company_policies",
    }
    assert expected_tools.issubset(set(tools.keys()))

    for name in expected_tools:
        tool_def = tools[name]
        assert tool_def.is_read_only is True
        assert tool_def.description
        assert tool_def.input_schema is not None

    openai_tools = export_openai_tools()
    assert len(openai_tools) >= 4
    tool_names = [t["function"]["name"] for t in openai_tools]
    assert "track_parcel_status" in tool_names
    assert "estimate_delivery_fee" in tool_names
    assert "list_active_parcels" in tool_names
    assert "explain_company_policies" in tool_names

    for ot in openai_tools:
        assert ot["type"] == "function"
        assert "parameters" in ot["function"]
        assert ot["function"]["parameters"]["type"] == "object"


def test_execute_tool_unknown_tool():
    """Verify executing an unregistered tool returns an ERROR ToolCallRecord."""
    async def _test():
        record = await execute_tool("non_existent_tool", {"foo": "bar"})
        assert isinstance(record, ToolCallRecord)
        assert record.status == "ERROR"
        assert "not registered" in record.error_message
        assert record.duration_ms is not None
    asyncio.run(_test())


def test_execute_tool_invalid_arguments():
    """Verify executing a tool with invalid arguments returns an ERROR ToolCallRecord."""
    async def _test():
        # Invalid tracking number format (fails regex ^SD-\d{8}-\d{5}$)
        record = await execute_tool("track_parcel_status", {"tracking_number": "INVALID-123"})
        assert record.status == "ERROR"
        assert "Invalid arguments" in record.error_message
    asyncio.run(_test())


def test_track_parcel_tool_success():
    """Verify tracking tool parses upstream 200 OK response with milestones and city."""
    async def _test():
        async def mock_handler(request: httpx.Request) -> httpx.Response:
            assert "SD-20261021-04921" in str(request.url)
            payload = {
                "status": "IN_TRANSIT",
                "status_description": "Shipment is in transit between Colombo and Kandy",
                "destination": {"city": "Kandy", "postal_code": "20000"},
                "estimated_delivery": "2026-10-23T14:00:00Z",
                "milestones": [
                    {
                        "status": "PICKED_UP",
                        "timestamp": "2026-10-21T09:30:00Z",
                        "location": "Colombo Central Hub",
                        "description": "Package received at hub",
                    },
                    {
                        "status": "IN_TRANSIT",
                        "timestamp": "2026-10-21T18:00:00Z",
                        "location": "Linehaul Highway Dispatch",
                        "description": "In transit to destination hub",
                    },
                ],
            }
            return httpx.Response(200, json=payload)

        transport = httpx.MockTransport(mock_handler)
        async with httpx.AsyncClient(transport=transport) as client:
            inp = TrackParcelInput(tracking_number="SD-20261021-04921")
            result = await track_parcel_tool(inp, client=client)

            assert result["found"] is True
            assert result["tracking_number"] == "SD-20261021-04921"
            assert result["status"] == "IN_TRANSIT"
            assert result["destination_city"] == "Kandy"
            assert len(result["milestones"]) == 2
            assert result["grounded"] is True
    asyncio.run(_test())


def test_track_parcel_tool_sec009_privacy_masking():
    """Verify SEC-009 privacy masking strips phone numbers, street addresses, and driver notes."""
    raw_leak_data = {
        "status": "OUT_FOR_DELIVERY",
        "status_description": "Courier out for delivery",
        "recipient_name": "John Doe",
        "recipient_phone": "+94771234567",
        "street_address": "45/2 Peradeniya Road, Apt 4B",
        "internal_driver_notes": "Gate code 4921, beware of dog",
        "destination": {
            "city": "Kandy",
            "full_address": "45/2 Peradeniya Road, Kandy",
        },
        "milestones": [
            {
                "status": "OUT_FOR_DELIVERY",
                "timestamp": "2026-10-22T08:00:00Z",
                "location": "Kandy Delivery Hub",
                "driver_contact": "+94779876543",
            }
        ],
    }

    sanitized = mask_tracking_data(raw_leak_data, "SD-20261021-04921")
    serialized = json.dumps(sanitized)

    # SEC-009 Invariant checks: sensitive PII MUST NOT be present
    assert "+94771234567" not in serialized
    assert "+94779876543" not in serialized
    assert "Peradeniya Road" not in serialized
    assert "Gate code 4921" not in serialized
    assert "John Doe" not in serialized
    # Non-sensitive tracking milestone information MUST be preserved
    assert sanitized["status"] == "OUT_FOR_DELIVERY"
    assert sanitized["destination_city"] == "Kandy"
    assert sanitized["grounded"] is True


def test_track_parcel_tool_404_not_found():
    """Verify 404 response produces a deterministic grounded not-found message."""
    async def _test():
        async def mock_handler(request: httpx.Request) -> httpx.Response:
            return httpx.Response(404, json={"error": "Not Found"})

        transport = httpx.MockTransport(mock_handler)
        async with httpx.AsyncClient(transport=transport) as client:
            inp = TrackParcelInput(tracking_number="SD-20261021-99999")
            result = await track_parcel_tool(inp, client=client)

            assert result["found"] is False
            assert result["tracking_number"] == "SD-20261021-99999"
            assert "No shipment found" in result["message"]
            assert result["grounded"] is True
    asyncio.run(_test())


def test_track_parcel_tool_upstream_500_and_timeout():
    """Verify tracking tool translates upstream 500 or timeout into UpstreamServiceException."""
    async def _test_500():
        async def mock_500(request: httpx.Request) -> httpx.Response:
            return httpx.Response(500, text="Internal Server Error")

        transport = httpx.MockTransport(mock_500)
        async with httpx.AsyncClient(transport=transport) as client:
            inp = TrackParcelInput(tracking_number="SD-20261021-04921")
            with pytest.raises(UpstreamServiceException) as exc_info:
                await track_parcel_tool(inp, client=client)
            assert "Tracking API" in str(exc_info.value)

    async def _test_timeout():
        def mock_timeout(request: httpx.Request):
            raise httpx.ReadTimeout("Read timed out")

        transport = httpx.MockTransport(mock_timeout)
        async with httpx.AsyncClient(transport=transport) as client:
            inp = TrackParcelInput(tracking_number="SD-20261021-04921")
            with pytest.raises(UpstreamServiceException) as exc_info:
                await track_parcel_tool(inp, client=client)
            assert "timed out" in str(exc_info.value)

    asyncio.run(_test_500())
    asyncio.run(_test_timeout())


def test_pricing_tool_success():
    """Verify pricing tool returns exact integer cents and generates NAVIGATE_BOOKING action."""
    async def _test():
        async def mock_handler(request: httpx.Request) -> httpx.Response:
            body = json.loads(request.content)
            assert body["weight_kg"] == 2.5
            assert body["pickup_postal_code"] == "00100"
            assert body["delivery_postal_code"] == "20000"
            assert body["service_type"] == "STANDARD"

            return httpx.Response(
                200,
                json={
                    "base_fee_cents": 35000,
                    "weight_fee_cents": 15000,
                    "total_fee_cents": 50000,
                    "currency": "LKR",
                    "estimated_transit_hours": 48,
                },
            )

        transport = httpx.MockTransport(mock_handler)
        async with httpx.AsyncClient(transport=transport) as client:
            inp = EstimateFeeInput(
                weight_kg=2.5,
                pickup_postal_code="00100",
                delivery_postal_code="20000",
                service_type=ServiceType.STANDARD,
                length_cm=20.0,
                width_cm=15.0,
                height_cm=10.0,
            )
            result = await pricing_tool(inp, client=client)

            assert result["total_fee_cents"] == 50000
            assert result["formatted_display"] == "LKR 500.00"
            assert result["estimated_transit_hours"] == 48

            action = result["action"]
            assert action["action_type"] == "NAVIGATE_BOOKING"
            assert action["target_url"] == "/portal/book"
            assert action["prefill_data"]["weight_kg"] == 2.5
            assert action["prefill_data"]["estimated_fee_cents"] == 50000
    asyncio.run(_test())


def test_pricing_tool_malformed_json_and_upstream_error():
    """Verify pricing tool gracefully handles non-JSON HTML error pages."""
    async def _test():
        async def mock_html_error(request: httpx.Request) -> httpx.Response:
            return httpx.Response(502, text="<html><body>502 Bad Gateway</body></html>")

        transport = httpx.MockTransport(mock_html_error)
        async with httpx.AsyncClient(transport=transport) as client:
            inp = EstimateFeeInput(
                weight_kg=1.0,
                pickup_postal_code="10000",
                delivery_postal_code="10500",
            )
            with pytest.raises(UpstreamServiceException):
                await pricing_tool(inp, client=client)
    asyncio.run(_test())


def test_parcels_tool_success_with_bearer():
    """Verify active parcels queries Core API with caller's Bearer token and sanitizes PII."""
    async def _test():
        async def mock_handler(request: httpx.Request) -> httpx.Response:
            assert request.headers.get("Authorization") == "Bearer secret-customer-jwt"
            payload = [
                {
                    "id": "par-001",
                    "tracking_number": "SD-20261021-04921",
                    "status": "IN_TRANSIT",
                    "service_type": "STANDARD",
                    "recipient": {
                        "name": "Jane Doe",
                        "phone": "+94778889999",
                        "city": "Galle",
                        "street": "10 Harbor Road",
                    },
                    "created_at": "2026-10-21T10:00:00Z",
                    "updated_at": "2026-10-21T14:00:00Z",
                }
            ]
            return httpx.Response(200, json=payload)

        transport = httpx.MockTransport(mock_handler)
        async with httpx.AsyncClient(transport=transport) as client:
            inp = ActiveParcelsInput(status=ParcelStatus.IN_TRANSIT, limit=5)
            context = {"authorization": "Bearer secret-customer-jwt"}
            result = await parcels_tool(inp, context=context, client=client)

            assert result["count"] == 1
            parcel = result["parcels"][0]
            assert parcel["tracking_number"] == "SD-20261021-04921"
            assert parcel["recipient_city"] == "Galle"
            # SEC-009 check: recipient phone & street address stripped
            assert "phone" not in parcel
            assert "street" not in parcel
            assert "Jane Doe" not in json.dumps(parcel)
    asyncio.run(_test())


def test_parcels_tool_missing_or_unauthorized():
    """Verify parcels tool raises AuthException when token is missing or upstream returns 401."""
    async def _test_missing():
        inp = ActiveParcelsInput(limit=5)
        with pytest.raises(AuthException) as exc_info:
            await parcels_tool(inp, context={})
        assert "Authentication token required" in str(exc_info.value)

    async def _test_unauthorized():
        async def mock_handler(request: httpx.Request) -> httpx.Response:
            return httpx.Response(401, json={"error": "Token expired"})

        transport = httpx.MockTransport(mock_handler)
        async with httpx.AsyncClient(transport=transport) as client:
            inp = ActiveParcelsInput(limit=5)
            context = {"authorization": "Bearer expired-token"}
            with pytest.raises(AuthException) as exc_info:
                await parcels_tool(inp, context=context, client=client)
            assert "Unauthorized" in str(exc_info.value)

    asyncio.run(_test_missing())
    asyncio.run(_test_unauthorized())


def test_policy_tool_static_fallback():
    """Verify company policy tool accurately retrieves static rules for prohibited goods and SLAs."""
    async def _test():
        inp = CompanyPolicyInput(query="prohibited items narcotics explosives")
        result = await policy_tool(inp)

        assert result["grounded"] is True
        assert "Prohibited" in result["policy_content"]
        assert "Hazardous Materials" in result["policy_content"]
        assert "Narcotics" in result["policy_content"]
        assert "Weapons" in result["policy_content"]
        assert result["source"] == "static_knowledge_base"
    asyncio.run(_test())


def test_policy_tool_transit_sla_query():
    """Verify company policy tool accurately filters Transit SLAs for Colombo metropolitan express."""
    async def _test():
        inp = CompanyPolicyInput(query="express delivery SLA transit time Colombo")
        result = await policy_tool(inp)

        assert "Express Delivery" in result["policy_content"]
        assert "24 Hours" in result["policy_content"] or "Same-Day" in result["policy_content"]
        assert "Colombo" in result["policy_content"]
    asyncio.run(_test())


def test_execute_tool_full_lifecycle_via_registry():
    """Verify execute_tool handles complete lifecycle with timing and error isolation."""
    async def _test():
        # Execute registered explain_company_policies tool via central registry
        record = await execute_tool(
            name="explain_company_policies",
            arguments={"query": "packaging dimension limits"},
        )
        assert record.status == "SUCCESS"
        assert record.tool_name == "explain_company_policies"
        assert record.duration_ms is not None
        assert record.duration_ms >= 0
        assert "300.0 cm" in record.result["policy_content"]
        assert "50.0 kg" in record.result["policy_content"]
    asyncio.run(_test())
