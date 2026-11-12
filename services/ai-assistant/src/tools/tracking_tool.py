"""Tracking API client tool for querying public shipment status with SEC-009 privacy masking."""

import json
from typing import Any, Dict, List, Optional
import urllib.parse
import httpx

from src.core.config import get_settings
from src.domain.errors import UpstreamServiceException
from src.domain.tools import TrackParcelInput
from src.tools.registry import register_tool


def mask_tracking_data(raw_data: Dict[str, Any], tracking_number: str) -> Dict[str, Any]:
    """Apply SEC-009 privacy masking, extracting only non-sensitive milestone and city information."""
    destination_city = "Unknown"
    dest_obj = raw_data.get("destination")
    if isinstance(dest_obj, dict) and dest_obj.get("city"):
        destination_city = dest_obj["city"]
    elif raw_data.get("destination_city"):
        destination_city = raw_data["destination_city"]

    raw_milestones = raw_data.get("milestones") or []
    sanitized_milestones: List[Dict[str, Any]] = []

    for m in raw_milestones:
        if isinstance(m, dict):
            sanitized_milestones.append(
                {
                    "status": m.get("status"),
                    "timestamp": m.get("timestamp"),
                    "location": m.get("location") or m.get("hub_name") or "In Transit",
                    "description": m.get("description") or m.get("status_description"),
                }
            )

    return {
        "found": True,
        "tracking_number": tracking_number,
        "status": raw_data.get("status", "UNKNOWN"),
        "status_description": raw_data.get("status_description") or raw_data.get("description", ""),
        "destination_city": destination_city,
        "estimated_delivery": raw_data.get("estimated_delivery") or raw_data.get("estimated_delivery_date"),
        "milestones": sanitized_milestones,
        "grounded": True,
    }


@register_tool(
    name="track_parcel_status",
    description="Track real-time shipment status, milestones, and estimated delivery for a parcel by tracking number.",
    input_schema=TrackParcelInput,
    is_read_only=True,
    aliases=["track_parcel"],
)
async def track_parcel_tool(
    input_data: TrackParcelInput,
    context: Optional[Dict[str, Any]] = None,
    client: Optional[httpx.AsyncClient] = None,
) -> Dict[str, Any]:
    """Query Member 5 Tracking API public endpoint with strict SEC-009 privacy masking and 404 safety."""
    settings = get_settings()
    encoded_num = urllib.parse.quote(input_data.tracking_number, safe="")
    target_url = f"{settings.tracking_api_base_url.rstrip('/')}/api/v1/tracking/public/{encoded_num}"

    timeouts = httpx.Timeout(
        connect=0.2 if settings.is_mock_mode else 2.0,
        read=5.0,
        write=5.0,
        pool=5.0,
    )

    # Use injected client if provided (e.g. for mock testing), else create one
    close_client = False
    if client is None:
        client = httpx.AsyncClient(timeout=timeouts)
        close_client = True

    try:
        response = await client.get(target_url)

        # Explicit 404 interceptor — prevent LLM hallucination on unknown parcels
        if response.status_code == 404:
            return {
                "found": False,
                "tracking_number": input_data.tracking_number,
                "message": (
                    f"No shipment found matching tracking number '{input_data.tracking_number}'. "
                    "Please verify the tracking number and try again."
                ),
                "grounded": True,
            }

        # Check for upstream server errors
        if response.status_code >= 400:
            raise UpstreamServiceException(
                "Tracking API",
                f"Received HTTP {response.status_code} from tracking service",
            )

        # Parse JSON securely
        try:
            data = response.json()
        except (json.JSONDecodeError, ValueError) as json_err:
            raise UpstreamServiceException(
                "Tracking API",
                f"Malformed non-JSON response received: {json_err}",
            )

        # Apply SEC-009 masking
        return mask_tracking_data(data, input_data.tracking_number)

    except httpx.TimeoutException as exc:
        raise UpstreamServiceException(
            "Tracking API",
            f"Connection timed out while fetching parcel status: {exc}",
        )
    except httpx.RequestError as exc:
        raise UpstreamServiceException(
            "Tracking API",
            f"Network error communicating with tracking service: {exc}",
        )
    finally:
        if close_client:
            await client.aclose()
