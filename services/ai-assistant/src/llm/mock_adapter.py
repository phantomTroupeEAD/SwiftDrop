"""Deterministic offline mock LLM adapter for fast, zero-cost CI and local development."""

import re
import time
from typing import Any, Dict, List, Optional
from uuid import UUID

from src.domain.chat import (
    ActionType,
    ChatMessage,
    ChatMessageResponse,
    GenerativeAction,
)
from src.llm.base import BaseLLMClient
from src.llm.guardrails import sanitize_assistant_identity
from src.tools.registry import execute_tool


class MockLLMAdapter(BaseLLMClient):
    """Offline deterministic mock engine executing regex intent classification and real Phase 3 tools."""

    async def generate_reply(
        self,
        prompt: str,
        history: List[ChatMessage],
        conversation_id: UUID,
        context: Optional[Dict[str, Any]] = None,
    ) -> ChatMessageResponse:
        """Route user intent to tools or knowledge base and synthesize deterministic grounded output."""
        start_time = time.perf_counter()
        tools_called: List[str] = []
        actions: List[GenerativeAction] = []
        prompt_lower = prompt.lower()

        # -------------------------------------------------------------
        # Intent 1: Tracking Inquiry with Specific Tracking Code
        # -------------------------------------------------------------
        tracking_match = re.search(r"SD-\d{8}-\d{5}", prompt, re.IGNORECASE)

        if tracking_match:
            tracking_num = tracking_match.group(0).upper()
            record = await execute_tool(
                "track_parcel_status",
                {"tracking_number": tracking_num},
                context=context,
            )
            tools_called.append("track_parcel_status")

            if record.status == "SUCCESS" and record.result.get("found"):
                res = record.result
                reply = (
                    f"**Tracking Update for `{tracking_num}`:**\n"
                    f"- **Status:** {res.get('status')}\n"
                    f"- **Description:** {res.get('status_description')}\n"
                    f"- **Destination:** {res.get('destination_city')}\n"
                    f"- **Estimated Delivery:** {res.get('estimated_delivery') or 'Pending scheduled dispatch'}"
                )
            elif record.status == "SUCCESS" and not record.result.get("found"):
                reply = record.result.get(
                    "message",
                    f"No shipment found matching tracking number `{tracking_num}`.",
                )
            else:
                # If Tracking API is offline in local dev, provide offline preview
                err_msg = (record.error_message or "").lower()
                if "network error" in err_msg or "timed out" in err_msg or "connection" in err_msg:
                    reply = (
                        f"**Tracking Update for `{tracking_num}` (Offline Simulation):**\n"
                        f"- **Status:** IN_TRANSIT\n"
                        f"- **Description:** Shipment is in transit to regional distribution hub\n"
                        f"- **Destination:** Colombo 03\n"
                        f"- **Estimated Delivery:** 2026-10-24T14:00:00Z"
                    )
                else:
                    reply = f"Error retrieving tracking status for `{tracking_num}`: {record.error_message}"

        # -------------------------------------------------------------
        # Intent 2: Tracking Inquiry without Code (e.g. "Can you track my package?")
        # -------------------------------------------------------------
        elif "track" in prompt_lower or "where is" in prompt_lower:
            reply = (
                "Please provide your 15-character tracking number in the format `SD-YYYYMMDD-XXXXX` "
                "(e.g. `SD-20261021-04921`) so I can look up your shipment status."
            )

        # -------------------------------------------------------------
        # Intent 3: Active Parcels List
        # -------------------------------------------------------------
        elif any(
            kw in prompt_lower
            for kw in [
                "my parcel",
                "my package",
                "active shipment",
                "active delivery",
                "active order",
                "active parcel",
                "active packages",
            ]
        ):
            record = await execute_tool(
                "list_active_parcels",
                {"limit": 5},
                context=context,
            )
            tools_called.append("list_active_parcels")

            if record.status == "SUCCESS":
                parcels = record.result.get("parcels", [])
                count = record.result.get("count", 0)
                if count > 0:
                    lines = [f"You currently have **{count} active shipment(s)**:"]
                    for p in parcels:
                        lines.append(
                            f"- `{p.get('tracking_number')}` ({p.get('status')}) → {p.get('recipient_city')}"
                        )
                    reply = "\n".join(lines)
                else:
                    reply = "You do not have any active shipments at this time."
            else:
                reply = (
                    f"Could not retrieve active shipments: {record.error_message}. "
                    "Please ensure you are signed in."
                )

        # -------------------------------------------------------------
        # Intent 4: Pricing / Quote Inquiry
        # -------------------------------------------------------------
        elif any(
            kw in prompt_lower
            for kw in ["cost", "fee", "estimate", "quote", "how much", "price", "tariff"]
        ):
            weight = 2.0
            weight_match = re.search(r"(\d+(\.\d+)?)\s*kg", prompt_lower)
            if weight_match:
                try:
                    weight = float(weight_match.group(1))
                except ValueError:
                    weight = 2.0

            # Find 5-digit postal codes if provided
            postal_codes = re.findall(r"\b\d{5}\b", prompt)
            pickup = postal_codes[0] if len(postal_codes) > 0 else "00100"
            delivery = postal_codes[1] if len(postal_codes) > 1 else "20000"

            record = await execute_tool(
                "estimate_delivery_fee",
                {
                    "weight_kg": weight,
                    "pickup_postal_code": pickup,
                    "delivery_postal_code": delivery,
                    "service_type": "STANDARD",
                },
                context=context,
            )
            tools_called.append("estimate_delivery_fee")

            if record.status == "SUCCESS":
                res = record.result
                if "action" in res and isinstance(res["action"], dict):
                    try:
                        actions.append(GenerativeAction(**res["action"]))
                    except Exception:
                        pass

                reply = (
                    f"**Delivery Fee Estimate:**\n"
                    f"- **Chargeable Weight:** {weight} kg\n"
                    f"- **Route:** Postal Code {pickup} → {delivery}\n"
                    f"- **Total Fee:** {res.get('formatted_display', 'LKR 500.00')}\n"
                    f"- **Estimated Transit:** {res.get('estimated_transit_hours', 48)} hours\n\n"
                    f"I have prepared a **1-click booking card** below for you to finalize this booking."
                )
            else:
                # Upstream Core API offline fallback in local dev / mock mode
                err_msg = (record.error_message or "").lower()
                if "network error" in err_msg or "timed out" in err_msg or "connection" in err_msg:
                    fee_cents = int(35000 + (weight * 10000))
                    act = GenerativeAction(
                        action_type=ActionType.NAVIGATE_BOOKING,
                        label="Book Delivery Now",
                        target_url="/portal/book",
                        prefill_data={
                            "pickup_postal_code": pickup,
                            "delivery_postal_code": delivery,
                            "weight_kg": weight,
                            "estimated_fee_cents": fee_cents,
                            "currency": "LKR",
                        },
                    )
                    actions.append(act)
                    reply = (
                        f"**Delivery Fee Estimate:**\n"
                        f"- **Chargeable Weight:** {weight} kg\n"
                        f"- **Route:** Postal Code {pickup} → {delivery}\n"
                        f"- **Total Fee:** LKR {fee_cents / 100:.2f}\n"
                        f"- **Estimated Transit:** 48 hours\n\n"
                        f"I have prepared a **1-click booking card** below for you to finalize this booking."
                    )
                else:
                    reply = f"Unable to calculate shipping estimate: {record.error_message}"

        # -------------------------------------------------------------
        # Intent 5: Policies, Packaging, SLAs & Prohibited Goods
        # -------------------------------------------------------------
        elif any(
            kw in prompt_lower
            for kw in [
                "policy",
                "prohibited",
                "restricted",
                "rules",
                "sla",
                "transit time",
                "packaging",
                "battery",
                "batteries",
                "narcotic",
                "claim",
                "dispute",
                "damage",
            ]
        ):
            record = await execute_tool(
                "explain_company_policies",
                {"query": prompt},
                context=context,
            )
            tools_called.append("explain_company_policies")

            if record.status == "SUCCESS":
                policy_text = record.result.get("policy_content", "")
                reply = f"**SwiftDrop Courier Policy Overview:**\n\n{policy_text}"
            else:
                reply = f"Could not retrieve policy guidelines: {record.error_message}"

        # -------------------------------------------------------------
        # Intent 6: Assistant Identity / Third-Party Model Inquiry
        # -------------------------------------------------------------
        elif any(
            kw in prompt_lower
            for kw in [
                "who are you",
                "what are you",
                "your name",
                "what model",
                "which model",
                "who created you",
                "who made you",
                "deepseek",
                "kimi",
                "qwen",
                "moonshot",
                "gemini",
                "chatgpt",
                "openai",
                "claude",
                "llama",
                "mistral",
                "particle",
            ]
        ):
            reply = (
                "I am **SwiftDrop AI**, the official AI delivery and logistics assistant "
                "developed specifically for the SwiftDrop courier network in Sri Lanka."
            )

        # -------------------------------------------------------------
        # Intent 7: General Greeting / Default Help
        # -------------------------------------------------------------
        else:
            reply = (
                "Hello! I am **SwiftDrop AI**, the official SwiftDrop AI Delivery Assistant for the SwiftDrop courier network.\n\n"
                "I can assist you with:\n"
                "- 📦 **Tracking a shipment:** Inquire with your tracking code (e.g. `SD-20261021-04921`)\n"
                "- 💰 **Fee quotes:** Ask for a tariff estimate (e.g. *'How much to send 2.5kg from 00100 to 20000?'*)\n"
                "- 📋 **Active shipments:** Check your active deliveries (*'Where are my packages?'*)\n"
                "- ℹ️ **Operating policies:** Inquire about packaging limits, transit SLAs, or prohibited goods."
            )

        latency_ms = int((time.perf_counter() - start_time) * 1000)
        reply = sanitize_assistant_identity(reply)

        return ChatMessageResponse(
            conversation_id=conversation_id,
            reply=reply,
            actions=actions,
            tools_called=tools_called,
            latency_ms=latency_ms,
            grounded=True,
        )
