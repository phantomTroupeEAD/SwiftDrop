"""Core API parcels client tool for retrieving authenticated shipments with IDOR defense."""

import json
from typing import Any, Dict, List, Optional
import httpx

from src.core.config import get_settings
from src.domain.errors import AuthException, UpstreamServiceException
from src.domain.tools import ActiveParcelsInput
from src.tools.registry import register_tool


def sanitize_parcel_item(item: Dict[str, Any]) -> Dict[str, Any]:
    """Strip sensitive PII (recipient phone, recipient street address) from parcel record."""
    recipient = item.get("recipient") or {}
    recipient_city = (
        recipient.get("city")
        if isinstance(recipient, dict)
        else item.get("recipient_city", "Unknown")
    )

    return {
        "id": item.get("id"),
        "tracking_number": item.get("tracking_number"),
        "status": item.get("status"),
        "recipient_city": recipient_city,
        "service_type": item.get("service_type"),
        "created_at": item.get("created_at"),
        "updated_at": item.get("updated_at"),
    }


@register_tool(
    name="list_active_parcels",
    description="Retrieve a list of active shipments belonging to the authenticated customer or merchant.",
    input_schema=ActiveParcelsInput,
    is_read_only=True,
)
async def parcels_tool(
    input_data: ActiveParcelsInput,
    context: Optional[Dict[str, Any]] = None,
    client: Optional[httpx.AsyncClient] = None,
) -> Dict[str, Any]:
    """Query Member 2 Core API parcels endpoint strictly forwarding authenticated user credentials."""
    context = context or {}
    settings = get_settings()

    auth_header = (
        context.get("authorization")
        or context.get("Authorization")
        or context.get("customer_jwt")
        or context.get("authorization_token")
    )

    # Fallback to configured SWIFTDROP_AUTH_TOKEN setting if not provided in context
    if not auth_header:
        env_token = settings.swiftdrop_auth_token.get_secret_value().strip()
        if env_token:
            auth_header = env_token

    if not auth_header or not auth_header.strip():
        raise AuthException(
            "Authentication token required to retrieve customer shipments. "
            "Please configure SWIFTDROP_AUTH_TOKEN in environment or provide authorization."
        )

    # Format bearer token cleanly
    auth_clean = auth_header.strip()
    if not auth_clean.lower().startswith("bearer "):
        bearer_token = f"Bearer {auth_clean}"
    else:
        bearer_token = auth_clean

    settings = get_settings()
    target_url = f"{settings.core_api_base_url.rstrip('/')}/api/v1/parcels"

    headers = {
        "Authorization": bearer_token,
        "Accept": "application/json",
    }

    params: Dict[str, Any] = {"limit": input_data.limit}
    if input_data.status:
        params["status"] = input_data.status.value

    timeouts = httpx.Timeout(connect=2.0, read=5.0, write=5.0, pool=5.0)
    close_client = False
    if client is None:
        client = httpx.AsyncClient(timeout=timeouts)
        close_client = True

    try:
        response = await client.get(target_url, headers=headers, params=params)

        if response.status_code in (401, 403):
            raise AuthException("Unauthorized or expired session credentials")

        if response.status_code >= 400:
            raise UpstreamServiceException(
                "Core API Parcels",
                f"Received HTTP {response.status_code} from parcels service: {response.text}",
            )

        try:
            raw_parcels = response.json()
        except (json.JSONDecodeError, ValueError) as json_err:
            raise UpstreamServiceException(
                "Core API Parcels",
                f"Malformed non-JSON response received: {json_err}",
            )

        # Handle list response directly or envelope {"parcels": [...], "items": [...]}
        items: List[Dict[str, Any]] = []
        if isinstance(raw_parcels, list):
            items = raw_parcels
        elif isinstance(raw_parcels, dict):
            items = raw_parcels.get("parcels") or raw_parcels.get("items") or []

        sanitized_items = [sanitize_parcel_item(p) for p in items if isinstance(p, dict)]

        return {
            "count": len(sanitized_items),
            "parcels": sanitized_items,
            "grounded": True,
        }

    except httpx.TimeoutException as exc:
        raise UpstreamServiceException(
            "Core API Parcels",
            f"Parcels request timed out: {exc}",
        )
    except httpx.RequestError as exc:
        raise UpstreamServiceException(
            "Core API Parcels",
            f"Network error communicating with parcels service: {exc}",
        )
    finally:
        if close_client:
            await client.aclose()
