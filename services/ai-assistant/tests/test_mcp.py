"""Unit and integration tests for Model Context Protocol (FastMCP) server and resources."""

import asyncio
import json
import httpx
import pytest

from src.mcp.server import (
    estimate_shipping_fee,
    explain_company_policies,
    get_my_active_parcels,
    mcp,
    track_parcel,
)
from src.tools.pricing_tool import pricing_tool
from src.tools.tracking_tool import track_parcel_tool


def test_mcp_server_initialization():
    """Verify FastMCP server instance metadata and instructions."""
    assert mcp.name == "SwiftDrop Logistics Assistant"
    assert "SwiftDrop" in mcp.instructions
    assert "Sri Lanka" in mcp.instructions


def test_mcp_registered_tools_discovery():
    """Verify all 4 approved read-only tools are registered with schemas on FastMCP."""
    async def _test():
        tools = await mcp.list_tools()
        tool_names = [t.name for t in tools]
        expected_tools = [
            "track_parcel",
            "estimate_shipping_fee",
            "get_my_active_parcels",
            "explain_company_policies",
        ]
        for expected in expected_tools:
            assert expected in tool_names

        for tool in tools:
            assert tool.description is not None
            schema = getattr(tool, "input_schema", getattr(tool, "inputSchema", None))
            assert schema is not None
    asyncio.run(_test())


def test_mcp_registered_resources_discovery():
    """Verify authoritative MCP resources are exposed via swiftdrop:// URIs."""
    async def _test():
        resources = await mcp.list_resources()
        uris = [str(r.uri) for r in resources]
        assert "swiftdrop://policies" in uris
        assert "swiftdrop://sla-matrix" in uris
    asyncio.run(_test())


def test_mcp_resource_policies_content():
    """Verify swiftdrop://policies resource content provides authoritative company policies."""
    async def _test():
        content_list = await mcp.read_resource("swiftdrop://policies")
        assert len(content_list) > 0
        raw_text = content_list[0].content
        assert "Parcel Dimension & Packaging Standards" in raw_text
        assert "Prohibited & Restricted Commodities" in raw_text
        assert "Transit Service Level Agreements" in raw_text
    asyncio.run(_test())


def test_mcp_resource_sla_matrix_content():
    """Verify swiftdrop://sla-matrix resource content returns structured JSON delivery SLAs."""
    async def _test():
        content_list = await mcp.read_resource("swiftdrop://sla-matrix")
        assert len(content_list) > 0
        sla_data = json.loads(content_list[0].content)

        assert "express" in sla_data
        assert "standard" in sla_data
        assert "outstation" in sla_data
        assert sla_data["express"]["sla_hours"] == 24
        assert sla_data["standard"]["sla_hours"] == 72
        assert sla_data["outstation"]["sla_hours"] == 120
    asyncio.run(_test())


def test_mcp_track_parcel_success():
    """Verify FastMCP track_parcel executes tracking lookup with SEC-009 privacy masking."""
    async def _test():
        async def mock_handler(request: httpx.Request) -> httpx.Response:
            assert "SD-20261021-04921" in str(request.url)
            return httpx.Response(
                200,
                json={
                    "status": "IN_TRANSIT",
                    "status_description": "Shipment in transit between sorting hubs",
                    "destination": {"city": "Kandy"},
                    "recipient_phone": "+94771234567",  # Leaked PII to test SEC-009
                    "milestones": [
                        {"status": "PICKED_UP", "timestamp": "2026-10-21T09:00:00Z", "location": "Colombo"}
                    ],
                },
            )

        transport = httpx.MockTransport(mock_handler)
        # Verify track_parcel tool execution
        async with httpx.AsyncClient(transport=transport) as client:
            from src.domain.tools import TrackParcelInput
            direct_res = await track_parcel_tool(
                TrackParcelInput(tracking_number="SD-20261021-04921"),
                client=client,
            )
            assert direct_res["found"] is True
            assert direct_res["destination_city"] == "Kandy"
            assert "+94771234567" not in json.dumps(direct_res)

        # Call tool through FastMCP directly
        mcp_res = await track_parcel("SD-20261021-04921")
        assert mcp_res["tracking_number"] == "SD-20261021-04921"
    asyncio.run(_test())


def test_mcp_track_parcel_invalid_format():
    """Verify FastMCP track_parcel safely handles malformed tracking codes."""
    async def _test():
        res = await track_parcel("INVALID-TRACKING")
        assert res["found"] is False
        assert "error" in res or "Invalid arguments" in str(res)
    asyncio.run(_test())


def test_mcp_estimate_shipping_fee_success():
    """Verify FastMCP estimate_shipping_fee computes tariff and attaches booking action."""
    async def _test():
        async def mock_pricing(request: httpx.Request) -> httpx.Response:
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

        transport = httpx.MockTransport(mock_pricing)
        async with httpx.AsyncClient(transport=transport) as client:
            from src.domain.tools import EstimateFeeInput
            direct = await pricing_tool(
                EstimateFeeInput(weight_kg=2.0, pickup_postal_code="00100", delivery_postal_code="20000"),
                client=client,
            )
            assert direct["total_fee_cents"] == 50000
            assert direct["action"]["action_type"] == "NAVIGATE_BOOKING"

        # Verify tool call via FastMCP
        res = await estimate_shipping_fee(
            weight_kg=2.0,
            pickup_postal_code="00100",
            delivery_postal_code="20000",
        )
        assert res["total_fee_cents"] == 50000
    asyncio.run(_test())


def test_mcp_estimate_shipping_fee_weight_limit_error():
    """Verify FastMCP estimate_shipping_fee rejects weights over 50.0kg."""
    async def _test():
        res = await estimate_shipping_fee(
            weight_kg=75.0,  # Exceeds max 50kg limit
            pickup_postal_code="00100",
            delivery_postal_code="20000",
        )
        assert "error" in res
        assert "Invalid arguments" in res["error"] or "50.0" in res["error"]
    asyncio.run(_test())


def test_mcp_active_parcels_auth_enforcement():
    """Verify FastMCP get_my_active_parcels enforces Bearer token authentication."""
    async def _test():
        # Without auth token
        res_no_auth = await get_my_active_parcels(authorization_token="")
        assert "error" in res_no_auth
        assert "Authentication token required" in res_no_auth["error"]

        # With auth token
        async def mock_parcels(request: httpx.Request) -> httpx.Response:
            return httpx.Response(200, json=[])

        transport = httpx.MockTransport(mock_parcels)
        from src.domain.tools import ActiveParcelsInput
        from src.tools.parcels_tool import parcels_tool

        async with httpx.AsyncClient(transport=transport) as client:
            res_auth = await parcels_tool(
                ActiveParcelsInput(limit=5),
                context={"authorization": "Bearer valid-token"},
                client=client,
            )
            assert res_auth["count"] == 0
    asyncio.run(_test())


def test_mcp_explain_company_policies():
    """Verify FastMCP explain_company_policies retrieves relevant sections from static markdown."""
    async def _test():
        res = await explain_company_policies(query="prohibited items narcotics explosives")
        assert "policy_content" in res
        assert "Hazardous Materials" in res["policy_content"]
        assert "Narcotics" in res["policy_content"]
    asyncio.run(_test())


def test_mcp_read_only_invariant():
    """Verify FastMCP server exposes strictly read-only capabilities and zero write/delete tools."""
    async def _test():
        tools = await mcp.list_tools()
        tool_names = [t.name.lower() for t in tools]
        for name in tool_names:
            assert not any(action in name for action in ["create", "update", "delete", "cancel", "book", "assign", "wipe"])
    asyncio.run(_test())
