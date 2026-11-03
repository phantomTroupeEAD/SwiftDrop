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
    volumetric_kg = 0.0
    billable_weight = input_data.weight_kg
    if (
        input_data.length_cm is not None
        and input_data.width_cm is not None
        and input_data.height_cm is not None
    ):
        volumetric_kg = round(
            (input_data.length_cm * input_data.width_cm * input_data.height_cm) / 5000.0,
            2,
        )
        billable_weight = max(input_data.weight_kg, volumetric_kg)

    request_payload: Dict[str, Any] = {
        "weight_kg": billable_weight,
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

    timeouts = httpx.Timeout(
        connect=0.2 if settings.is_mock_mode else 2.0,
        read=5.0,
        write=5.0,
        pool=5.0,
    )
    close_client = False
    if client is None:
        client = httpx.AsyncClient(timeout=timeouts)
        close_client = True

    try:
        response = await client.post(target_url, json=request_payload)

        if response.status_code >= 400:
            error_msg = f"Received HTTP {response.status_code} while calculating quote"
            try:
                err_json = response.json()
                if isinstance(err_json, dict):
                    if "detail" in err_json and isinstance(err_json["detail"], list):
                        details = [
                            f"{'.'.join(str(loc) for loc in d.get('loc', []))}: {d.get('msg')}"
                            for d in err_json["detail"]
                            if isinstance(d, dict)
                        ]
                        if details:
                            error_msg = f"Validation error: {'; '.join(details)}"
                    elif "message" in err_json:
                        error_msg = str(err_json["message"])
                    elif "error" in err_json and isinstance(err_json["error"], dict) and "message" in err_json["error"]:
                        error_msg = str(err_json["error"]["message"])
            except Exception:
                pass
            raise UpstreamServiceException("Core API Pricing", error_msg)

        try:
            quote_data = response.json()
        except (json.JSONDecodeError, ValueError) as json_err:
            raise UpstreamServiceException(
                "Core API Pricing",
                f"Malformed non-JSON response received: {json_err}",
            )

        total_fee_cents = quote_data.get("total_fee_cents") or quote_data.get("total_cents", 0)
        total_fee_lkr = quote_data.get("total_fee") or int(round(total_fee_cents / 100.0))
        currency = quote_data.get("currency", "LKR")
        display_amount = f"{currency} {total_fee_lkr:,}"

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
                "estimated_fee": total_fee_lkr,
                "estimated_fee_cents": total_fee_cents,
                "currency": currency,
            },
        )

        return {
            "weight_kg": input_data.weight_kg,
            "pickup_postal_code": input_data.pickup_postal_code,
            "delivery_postal_code": input_data.delivery_postal_code,
            "service_type": input_data.service_type.value,
            "estimated_fee": total_fee_lkr,
            "base_fee": int(round(quote_data.get("base_fee_cents", 0) / 100.0)),
            "weight_fee": int(round(quote_data.get("weight_fee_cents", 0) / 100.0)),
            "base_fee_cents": quote_data.get("base_fee_cents", 0),
            "weight_fee_cents": quote_data.get("weight_fee_cents", 0),
            "total_fee_cents": total_fee_cents,
            "currency": currency,
            "formatted_display": display_amount,
            "estimated_transit_hours": quote_data.get("estimated_transit_hours"),
            "action": action.model_dump(),
            "grounded": True,
        }

    except (httpx.TimeoutException, httpx.RequestError) as exc:
        if close_client and settings.is_mock_mode:
            # Deterministic offline mock calculation for CI / local test execution
            fee_cents = int(35000 + (billable_weight * 7500))
            total_fee_lkr = int(round(fee_cents / 100.0))
            currency = "LKR"
            action = GenerativeAction(
                action_type=ActionType.NAVIGATE_BOOKING,
                label="Book Delivery Now",
                target_url="/portal/book",
                prefill_data={
                    "pickup_postal_code": input_data.pickup_postal_code,
                    "delivery_postal_code": input_data.delivery_postal_code,
                    "weight_kg": input_data.weight_kg,
                    "service_type": input_data.service_type.value,
                    "estimated_fee": total_fee_lkr,
                    "estimated_fee_cents": fee_cents,
                    "currency": currency,
                },
            )
            return {
                "weight_kg": input_data.weight_kg,
                "pickup_postal_code": input_data.pickup_postal_code,
                "delivery_postal_code": input_data.delivery_postal_code,
                "service_type": input_data.service_type.value,
                "estimated_fee": total_fee_lkr,
                "base_fee": 350,
                "weight_fee": total_fee_lkr - 350,
                "base_fee_cents": 35000,
                "weight_fee_cents": fee_cents - 35000,
                "total_fee_cents": fee_cents,
                "currency": currency,
                "formatted_display": f"{currency} {total_fee_lkr:,}",
                "estimated_transit_hours": 48,
                "action": action.model_dump(),
                "grounded": True,
            }
        raise UpstreamServiceException(
            "Core API Pricing",
            f"Network error communicating with pricing service: {exc}",
        )
    finally:
        if close_client:
            await client.aclose()
