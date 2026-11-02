"""Official Model Context Protocol (FastMCP) server package for SwiftDrop."""

from src.mcp.server import (
    explain_company_policies,
    estimate_shipping_fee,
    get_company_policies_resource,
    get_my_active_parcels,
    get_sla_matrix_resource,
    mcp,
    track_parcel,
)

__all__ = [
    "mcp",
    "track_parcel",
    "estimate_shipping_fee",
    "get_my_active_parcels",
    "explain_company_policies",
    "get_company_policies_resource",
    "get_sla_matrix_resource",
]
