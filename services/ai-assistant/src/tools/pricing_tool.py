"""Core API pricing client tool for calculating delivery quotes and attaching booking actions."""

import json
from typing import Any, Dict, Optional
import httpx

from src.core.config import get_settings
from src.domain.chat import ActionType, GenerativeAction
from src.domain.errors import UpstreamServiceException
from src.domain.tools import EstimateFeeInput
from src.tools.registry import register_tool


@register_tool(
    name="estimate_delivery_fee",
    description="Calculate authoritative courier shipping fee quotes and transit estimates based on weight, dimensions, and postal codes.",
    input_schema=EstimateFeeInput,
    is_read_only=True,
)
async def pricing_tool(
    input_data: EstimateFeeInput,
    context: Optional[Dict[str, Any]] = None,
    client: Optional[httpx.AsyncClient] = None,
) -> Dict[str, Any]:
    """Query Member 2 Core API pricing engine and construct pre-filled booking actions."""
    settings = get_settings()
    target_url = f"{settings.core_api_base_url.rstrip('/')}/api/v1/pricing/quotes"

    # Build upstream payload conforming to Core API contract
    request_payload: Dict[str, Any] = {
        "weight_kg": input_data.weight_kg,
        "pickup_postal_code": input_data.pickup_postal_code,
        "delivery_postal_code": input_data.delivery_postal_code,
        "service_type": input_data.service_type.value,
    }

    if (
        input_data.length_cm is not None
        or input_data.width_cm is not None
        or input_data.height_cm is not None
    ):
        request_payload["dimensions"] = {
            "length_cm": input_data.length_cm,
            "width_cm": input_data.width_cm,
            "height_cm": input_data.height_cm,
        }

    timeouts = httpx.Timeout(connect=2.0, read=5.0, write=5.0, pool=5.0)
    close_client = False
    if client is None:
        client = httpx.AsyncClient(timeout=timeouts)
        close_client = True

    try:
        response = await client.post(target_url, json=request_payload)

        if response.status_code >= 400:
            raise UpstreamServiceException(
                "Core API Pricing",
                f"Received HTTP {response.status_code} while calculating quote: {response.text}",
            )

        try:
            quote_data = response.json()
        except (json.JSONDecodeError, ValueError) as json_err:
            raise UpstreamServiceException(
                "Core API Pricing",
                f"Malformed non-JSON response received: {json_err}",
            )

        total_fee_cents = quote_data.get("total_fee_cents") or quote_data.get("total_cents", 0)
        currency = quote_data.get("currency", "LKR")
        display_amount = f"{currency} {total_fee_cents / 100:.2f}"

        # Construct validated GenerativeAction card for the web portal
        action = GenerativeAction(
            action_type=ActionType.NAVIGATE_BOOKING,
            label="Book Delivery Now",
            target_url="/portal/book",
            prefill_data={
                "pickup_postal_code": input_data.pickup_postal_code,
                "delivery_postal_code": input_data.delivery_postal_code,
                "weight_kg": input_data.weight_kg,
                "service_type": input_data.service_type.value,
                "estimated_fee_cents": total_fee_cents,
                "currency": currency,
            },
        )

        return {
            "weight_kg": input_data.weight_kg,
            "pickup_postal_code": input_data.pickup_postal_code,
            "delivery_postal_code": input_data.delivery_postal_code,
            "service_type": input_data.service_type.value,
            "base_fee_cents": quote_data.get("base_fee_cents", 0),
            "weight_fee_cents": quote_data.get("weight_fee_cents", 0),
            "total_fee_cents": total_fee_cents,
            "currency": currency,
            "formatted_display": display_amount,
            "estimated_transit_hours": quote_data.get("estimated_transit_hours"),
            "action": action.model_dump(),
            "grounded": True,
        }

    except httpx.TimeoutException as exc:
        raise UpstreamServiceException(
            "Core API Pricing",
            f"Pricing quote request timed out: {exc}",
        )
    except httpx.RequestError as exc:
        raise UpstreamServiceException(
            "Core API Pricing",
            f"Network error communicating with pricing service: {exc}",
        )
    finally:
        if close_client:
            await client.aclose()
