"""Postal code lookup tool for Sri Lankan cities, towns, and postal divisions."""

from typing import Any, Dict, Optional
from src.core.postal_codes import lookup_postal_record
from src.domain.tools import LookupPostalCodeInput
from src.tools.registry import register_tool


@register_tool(
    name="lookup_postal_code",
    description="Lookup Sri Lankan 5-digit postal code by city, town, suburb, or postal division (e.g. 'Walasmulla', 'Kadugannawa', 'Colombo 03', 'Anuradhapura', 'Kegalle').",
    input_schema=LookupPostalCodeInput,
    is_read_only=True,
)
async def postal_tool(
    input_data: LookupPostalCodeInput,
    context: Optional[Dict[str, Any]] = None,
) -> Dict[str, Any]:
    """Lookup Sri Lankan 5-digit postal code by city, town, suburb or postal zone."""
    record = lookup_postal_record(input_data.query)

    if record:
        return {
            "found": True,
            "query": input_data.query,
            "city": record.get("city"),
            "postal_code": record.get("code"),
            "district": record.get("district"),
            "province": record.get("province"),
            "formatted_display": f"{record.get('city')} ({record.get('district')} District) -> Postal Code: {record.get('code')}",
            "grounded": True,
        }

    return {
        "found": False,
        "query": input_data.query,
        "message": (
            f"Could not find an authoritative postal code for '{input_data.query}'. "
            "Please specify the nearest major town or administrative district."
        ),
        "grounded": True,
    }
