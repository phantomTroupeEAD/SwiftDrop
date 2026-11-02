"""Official Model Context Protocol (FastMCP) server for SwiftDrop AI Delivery Assistant."""

import json
from pathlib import Path
from typing import Any, Dict, Optional

try:
    # mcp >= 2.0 (FastMCP renamed to MCPServer in mcp.server.mcpserver)
    from mcp.server.mcpserver import MCPServer as FastMCP
except (ImportError, ModuleNotFoundError):
    # mcp < 2.0 fallback
    from mcp.server.fastmcp import FastMCP

from src.tools.registry import execute_tool

# Initialize FastMCP Server
mcp = FastMCP(
    name="SwiftDrop Logistics Assistant",
    instructions=(
        "Official Model Context Protocol server for the SwiftDrop Courier Management Platform in Sri Lanka. "
        "Allows external AI models, IDE assistants (Cursor), and desktop tools (Claude Desktop) to track "
        "shipment milestones, calculate delivery fees, retrieve active consignments, and query courier policies."
    ),
)


# -------------------------------------------------------------------------
# FastMCP Tools (Strictly Read-Only Operational Capabilities)
# -------------------------------------------------------------------------

@mcp.tool()
async def track_parcel(tracking_number: str) -> Dict[str, Any]:
    """Track real-time shipment status, sanitized milestones, and destination city for a SwiftDrop parcel.

    Args:
        tracking_number: Authoritative parcel tracking code in canonical format SD-YYYYMMDD-XXXXX (e.g. SD-20261021-04921).
    """
    record = await execute_tool("track_parcel_status", {"tracking_number": tracking_number})
    if record.status == "SUCCESS" and isinstance(record.result, dict):
        return record.result
    return {
        "found": False,
        "tracking_number": tracking_number,
        "error": record.error_message,
        "grounded": True,
    }


@mcp.tool()
async def estimate_shipping_fee(
    weight_kg: float,
    pickup_postal_code: str,
    delivery_postal_code: str,
    service_type: str = "STANDARD",
    length_cm: Optional[float] = None,
    width_cm: Optional[float] = None,
    height_cm: Optional[float] = None,
) -> Dict[str, Any]:
    """Calculate authoritative courier delivery fee quotes, estimated transit hours, and prefilled booking actions in Sri Lankan Rupees (LKR).

    Args:
        weight_kg: Gross package weight in kilograms (must be > 0.0 and <= 50.0).
        pickup_postal_code: 5-digit origin pickup postal code (e.g. 00100 for Colombo).
        delivery_postal_code: 5-digit destination delivery postal code (e.g. 20000 for Kandy).
        service_type: Delivery tier (STANDARD, EXPRESS, SAME_DAY).
        length_cm: Optional package length in centimeters (<= 300.0).
        width_cm: Optional package width in centimeters (<= 300.0).
        height_cm: Optional package height in centimeters (<= 300.0).
    """
    arguments: Dict[str, Any] = {
        "weight_kg": weight_kg,
        "pickup_postal_code": pickup_postal_code,
        "delivery_postal_code": delivery_postal_code,
        "service_type": service_type,
    }
    if length_cm is not None:
        arguments["length_cm"] = length_cm
    if width_cm is not None:
        arguments["width_cm"] = width_cm
    if height_cm is not None:
        arguments["height_cm"] = height_cm

    record = await execute_tool("estimate_delivery_fee", arguments)
    if record.status == "SUCCESS" and isinstance(record.result, dict):
        return record.result
    return {"error": record.error_message, "grounded": True}


@mcp.tool()
async def get_my_active_parcels(
    authorization_token: str,
    limit: int = 10,
    status: Optional[str] = None,
) -> Dict[str, Any]:
    """Retrieve currently active shipments belonging to the authenticated customer or merchant.

    Args:
        authorization_token: Customer or merchant JWT Bearer token (required for IDOR isolation).
        limit: Maximum number of active parcels to return (1-50, default: 10).
        status: Optional lifecycle status filter (e.g. IN_TRANSIT, OUT_FOR_DELIVERY).
    """
    arguments: Dict[str, Any] = {"limit": limit}
    if status is not None:
        arguments["status"] = status

    context = {"authorization": authorization_token}
    record = await execute_tool("list_active_parcels", arguments, context=context)
    if record.status == "SUCCESS" and isinstance(record.result, dict):
        return record.result
    return {"error": record.error_message, "count": 0, "parcels": []}


@mcp.tool()
async def explain_company_policies(query: str) -> Dict[str, Any]:
    """Explain courier packaging guidelines, prohibited commodities, transit SLAs, and damaged parcel dispute windows.

    Args:
        query: Specific inquiry topic (e.g. 'prohibited items', 'packaging rules', 'SLA', 'damaged parcel claim').
    """
    record = await execute_tool("explain_company_policies", {"query": query})
    if record.status == "SUCCESS" and isinstance(record.result, dict):
        return record.result
    return {"error": record.error_message}


# -------------------------------------------------------------------------
# Authoritative MCP Resources (Context Injection for External LLMs)
# -------------------------------------------------------------------------

@mcp.resource("swiftdrop://policies")
def get_company_policies_resource() -> str:
    """Authoritative SwiftDrop shipping policies, prohibited goods, packaging standards, and dispute rules."""
    policy_file = Path(__file__).parent.parent / "tools" / "company_policies.md"
    if policy_file.exists():
        return policy_file.read_text(encoding="utf-8")
    return (
        "# SwiftDrop Policies\n"
        "Max weight: 50.0kg. Prohibited items: explosives, hazardous chemicals, narcotics, live animals."
    )


@mcp.resource("swiftdrop://sla-matrix")
def get_sla_matrix_resource() -> str:
    """Structured transit Service Level Agreements (SLAs) across Sri Lanka delivery zones."""
    matrix = {
        "express": {
            "name": "Express Delivery",
            "coverage_zones": "Colombo Metropolitan & Suburbs (10000–10500)",
            "sla_hours": 24,
            "description": "Same-day or guaranteed 24 hours dispatch.",
        },
        "standard": {
            "name": "Standard Delivery",
            "coverage_zones": "Western & Southern Provinces (11000–13500, 80000–82500)",
            "sla_hours": 72,
            "description": "48 to 72 hours linehaul delivery.",
        },
        "outstation": {
            "name": "Outstation Delivery",
            "coverage_zones": "Northern, Eastern, Central & Uva Provinces (20000–72500)",
            "sla_hours": 120,
            "description": "3 to 5 business days delivery via regional hubs.",
        },
    }
    return json.dumps(matrix, indent=2)


# -------------------------------------------------------------------------
# Server Execution Entrypoint (STDIO for Claude Desktop / Cursor)
# -------------------------------------------------------------------------

if __name__ == "__main__":
    # Run STDIO transport by default for desktop clients
    mcp.run(transport="stdio")
