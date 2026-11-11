"""Official Model Context Protocol (FastMCP) server package for SwiftDrop."""

# Defer imports of server symbols to avoid runpy RuntimeWarning on `python -m src.mcp.server`
def __getattr__(name: str):
    if name in (
        "mcp",
        "track_parcel",
        "estimate_shipping_fee",
        "get_my_active_parcels",
        "explain_company_policies",
        "get_company_policies_resource",
        "get_sla_matrix_resource",
    ):
        from src.mcp import server as _server
        return getattr(_server, name)
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")


__all__ = [
    "mcp",
    "track_parcel",
    "estimate_shipping_fee",
    "get_my_active_parcels",
    "explain_company_policies",
    "get_company_policies_resource",
    "get_sla_matrix_resource",
]
