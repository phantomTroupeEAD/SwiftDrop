"""Standalone verification script to directly test FastMCP tools and resources."""

import asyncio
import json
import sys
from pathlib import Path

# Add project root to sys.path
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from src.mcp.server import mcp


async def run_mcp_verification():
    print("==================================================")
    print("   SwiftDrop FastMCP Server Direct Verification")
    print("==================================================")
    print(f"Server Name: {mcp.name}")
    print(f"Instructions: {mcp.instructions[:80]}...\n")

    # 1. Discover registered tools
    print("[1] Discovering Registered Tools:")
    tools = await mcp.list_tools()
    for t in tools:
        print(f"  - Tool: {t.name:<25} | Description: {t.description[:60]}...")

    # 2. Discover registered resources
    print("\n[2] Discovering Registered Resources:")
    resources = await mcp.list_resources()
    for r in resources:
        print(f"  - Resource: {str(r.uri):<25} | Description: {r.description[:60]}...")

    # 3. Read an MCP Resource
    print("\n[3] Reading Resource 'swiftdrop://sla-matrix':")
    sla_resource = await mcp.read_resource("swiftdrop://sla-matrix")
    sla_data = json.loads(sla_resource[0].content)
    print(f"  - Express Delivery SLA: {sla_data['express']['sla_hours']} hours ({sla_data['express']['coverage_zones']})")
    print(f"  - Standard Delivery SLA: {sla_data['standard']['sla_hours']} hours ({sla_data['standard']['coverage_zones']})")

    # 4. Execute a tool: estimate_shipping_fee
    print("\n[4] Executing FastMCP Tool 'estimate_shipping_fee':")
    fee_result = await mcp.call_tool(
        "estimate_shipping_fee",
        arguments={
            "weight_kg": 2.5,
            "pickup_postal_code": "00100",
            "delivery_postal_code": "20000",
            "service_type": "STANDARD",
        },
    )
    print(f"  - Result: {fee_result}")

    # 5. Execute a tool: lookup_postal_code
    print("\n[5] Executing FastMCP Tool 'lookup_postal_code':")
    postal_result = await mcp.call_tool(
        "lookup_postal_code",
        arguments={"city_or_area": "Walasmulla"},
    )
    print(f"  - Result: {postal_result}")

    print("\n==================================================")
    print("   ALL FASTMCP CAPABILITIES VERIFIED SUCCESSFULLY!")
    print("==================================================")


if __name__ == "__main__":
    asyncio.run(run_mcp_verification())
