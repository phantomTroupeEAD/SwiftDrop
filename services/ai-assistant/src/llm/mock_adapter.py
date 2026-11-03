"""Deterministic offline mock LLM adapter for fast, zero-cost CI and local development."""

import re
import time
from typing import Any, AsyncGenerator, Dict, List, Optional
from uuid import UUID

from src.core.postal_codes import SRI_LANKA_CITY_POSTAL_CODES, resolve_postal_code
from src.domain.chat import (
    ActionType,
    ChatMessage,
    ChatMessageResponse,
    GenerativeAction,
    StreamChunkEvent,
    StreamEventType,
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

        # Extract conversational entities from history for multi-turn continuity
        hist_tracking: Optional[str] = None
        hist_pickup: Optional[str] = None
        hist_delivery: Optional[str] = None
        hist_weight: Optional[float] = None

        if history:
            for turn in reversed(history):
                content = turn.content
                if not hist_tracking:
                    m = re.search(r"SD-\d{8}-\d{5}", content, re.IGNORECASE)
                    if m:
                        hist_tracking = m.group(0).upper()
                if not hist_pickup or not hist_delivery:
                    codes = re.findall(r"\b\d{5}\b", content)
                    if len(codes) >= 2:
                        hist_pickup, hist_delivery = codes[0], codes[1]
                if hist_weight is None:
                    wm = re.search(r"(\d+(\.\d+)?)\s*kg", content.lower())
                    if wm:
                        try:
                            hist_weight = float(wm.group(1))
                        except ValueError:
                            pass

        # -------------------------------------------------------------
        # Intent 1: Tracking Inquiry with Specific or Contextual Tracking Code
        # -------------------------------------------------------------
        tracking_match = re.search(r"SD-\d{8}-\d{5}", prompt, re.IGNORECASE)
        resolved_tracking = tracking_match.group(0).upper() if tracking_match else None

        # If no explicit tracking code in current prompt, check if user is referring to previously tracked parcel
        is_tracking_followup = bool(
            hist_tracking
            and any(
                kw in prompt_lower
                for kw in [
                    "where", "status", "arrive", "when", "track", "it", "destination",
                    "package", "parcel", "delivery", "update", "courier", "location"
                ]
            )
        )
        if not resolved_tracking and is_tracking_followup:
            resolved_tracking = hist_tracking

        if resolved_tracking:
            tracking_num = resolved_tracking
            record = await execute_tool(
                "track_parcel_status",
                {"tracking_number": tracking_num},
                context=context,
            )
            tools_called.append("track_parcel_status")

            if record.status == "SUCCESS" and record.result.get("found"):
                res = record.result
                dest_city = res.get("destination_city", "Destination Hub")
                status_val = res.get("status", "UNKNOWN")
                status_desc = res.get("status_description", "")
                est_del = res.get("estimated_delivery") or "Pending scheduled dispatch"

                # Provide targeted answer for specific conversational follow-up questions
                if any(kw in prompt_lower for kw in ["where", "destination", "city"]):
                    reply = f"Shipment `{tracking_num}` is destined for **{dest_city}**."
                elif any(kw in prompt_lower for kw in ["when", "arrive", "time", "date"]):
                    reply = f"Shipment `{tracking_num}` is estimated to arrive on **{est_del}**."
                elif any(kw in prompt_lower for kw in ["what is the status", "status update"]):
                    reply = f"The current status of `{tracking_num}` is **{status_val}**: {status_desc}."
                else:
                    reply = (
                        f"**Tracking Update for `{tracking_num}`:**\n"
                        f"- **Status:** {status_val}\n"
                        f"- **Description:** {status_desc}\n"
                        f"- **Destination:** {dest_city}\n"
                        f"- **Estimated Delivery:** {est_del}"
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
                    if any(kw in prompt_lower for kw in ["where", "destination", "city"]):
                        reply = f"Shipment `{tracking_num}` is destined for **Colombo 03**."
                    elif any(kw in prompt_lower for kw in ["when", "arrive", "time"]):
                        reply = f"Shipment `{tracking_num}` is estimated to arrive on **2026-10-24T14:00:00Z**."
                    else:
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
                    "Please ensure you are signed in to the SwiftDrop web portal."
                )

        # -------------------------------------------------------------
        # Intent 4: Pricing / Quote Inquiry
        # -------------------------------------------------------------
        elif (
            bool((hist_pickup and hist_delivery) and re.search(r"(\d+(\.\d+)?)\s*kg", prompt_lower))
            or any(
                kw in prompt_lower
                for kw in ["cost", "fee", "estimate", "quote", "how much", "price", "tariff"]
            )
        ):
            weight = 2.0
            weight_match = re.search(r"(\d+(\.\d+)?)\s*kg", prompt_lower)
            if weight_match:
                try:
                    weight = float(weight_match.group(1))
                except ValueError:
                    weight = 2.0
            elif hist_weight is not None:
                weight = hist_weight

            # Detect explicit postal codes or Sri Lankan city names in order of appearance
            postal_codes = re.findall(r"\b\d{5}\b", prompt)
            city_matches = []
            for city, code in SRI_LANKA_CITY_POSTAL_CODES.items():
                m = re.search(rf"\b{re.escape(city)}\b", prompt_lower)
                if m:
                    city_matches.append((m.start(), code))
            city_matches.sort(key=lambda x: x[0])
            # De-duplicate codes while preserving order
            detected_cities = []
            for _, code in city_matches:
                if code not in detected_cities:
                    detected_cities.append(code)

            if len(postal_codes) >= 2:
                pickup, delivery = postal_codes[0], postal_codes[1]
            elif len(detected_cities) >= 2:
                pickup, delivery = detected_cities[0], detected_cities[1]
            elif len(detected_cities) == 1 and len(postal_codes) == 1:
                pickup, delivery = postal_codes[0], detected_cities[0]
            elif hist_pickup and hist_delivery:
                pickup, delivery = hist_pickup, hist_delivery
            else:
                pickup = postal_codes[0] if len(postal_codes) > 0 else (detected_cities[0] if detected_cities else "00100")
                delivery = postal_codes[1] if len(postal_codes) > 1 else (detected_cities[1] if len(detected_cities) > 1 else "20000")

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
                    total_fee_lkr = fee_cents // 100
                    act = GenerativeAction(
                        action_type=ActionType.NAVIGATE_BOOKING,
                        label="Book Delivery Now",
                        target_url="/portal/book",
                        prefill_data={
                            "pickup_postal_code": pickup,
                            "delivery_postal_code": delivery,
                            "weight_kg": weight,
                            "service_type": "STANDARD",
                            "estimated_fee": total_fee_lkr,
                            "estimated_fee_cents": fee_cents,
                            "currency": "LKR",
                        },
                    )
                    actions.append(act)
                    reply = (
                        f"**Delivery Fee Estimate:**\n"
                        f"- **Chargeable Weight:** {weight} kg\n"
                        f"- **Route:** Postal Code {pickup} → {delivery}\n"
                        f"- **Total Fee:** LKR {total_fee_lkr:,}\n"
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
        # Intent 5b: Postal Code Inquiry (e.g. "What is the postal code for Walasmulla?")
        # -------------------------------------------------------------
        elif any(kw in prompt_lower for kw in ["postal code", "zip code", "post code", "postal number"]):
            record = await execute_tool("lookup_postal_code", {"query": prompt}, context=context)
            tools_called.append("lookup_postal_code")
            if record.status == "SUCCESS" and record.result.get("found"):
                res = record.result
                reply = (
                    f"The postal code for **{res.get('city')}** "
                    f"({res.get('district')} District, {res.get('province')} Province) is **`{res.get('postal_code')}`**."
                )
            else:
                reply = record.result.get(
                    "message",
                    "Could not find an authoritative postal code matching your query."
                )

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
        # Intent 7: Conversational Acknowledgment (e.g. "thanks", "ok", "great")
        # -------------------------------------------------------------
        elif any(
            kw in prompt_lower
            for kw in [
                "thank",
                "thanks",
                "thx",
                "great",
                "ok",
                "okay",
                "perfect",
                "got it",
                "understood",
                "awesome",
            ]
        ):
            reply = "You're welcome! Let me know if you need any further assistance with your shipments or delivery quotes."

        # -------------------------------------------------------------
        # Intent 8: Explicit Greeting
        # -------------------------------------------------------------
        elif any(
            kw in prompt_lower
            for kw in ["hello", "hi", "hey", "good morning", "good afternoon", "good evening", "greetings"]
        ):
            reply = (
                "Hello! I am **SwiftDrop AI**, the official SwiftDrop AI Delivery Assistant.\n\n"
                "I can assist you with:\n"
                "- 📦 **Tracking a shipment:** Inquire with your tracking code (e.g. `SD-20261021-04921`)\n"
                "- 💰 **Fee quotes:** Ask for a tariff estimate (e.g. *'How much to send 2.5kg from 00100 to 20000?'*)\n"
                "- 📋 **Active shipments:** Check your active deliveries (*'Where are my packages?'*)\n"
                "- ℹ️ **Operating policies:** Inquire about packaging limits, transit SLAs, or prohibited goods."
            )

        # -------------------------------------------------------------
        # Intent 9: General / Fallback Question (Direct answer without repeating intro)
        # -------------------------------------------------------------
        else:
            reply = (
                "I am here to assist with all your SwiftDrop courier and logistics needs. "
                "You can ask me to track a parcel (`SD-YYYYMMDD-XXXXX`), estimate delivery tariffs, "
                "check your active deliveries, or explain shipping regulations."
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

    async def generate_reply_stream(
        self,
        prompt: str,
        history: List[ChatMessage],
        conversation_id: UUID,
        context: Optional[Dict[str, Any]] = None,
    ) -> AsyncGenerator[StreamChunkEvent, None]:
        """Process conversational turn and yield incremental stream chunk events deterministically."""
        # 1. Resolve full conversational turn
        response = await self.generate_reply(
            prompt=prompt,
            history=history,
            conversation_id=conversation_id,
            context=context,
        )

        # 2. Emit tool call start and end events for any executed tools
        for idx, tool_name in enumerate(response.tools_called):
            call_id = f"mock_call_{idx}_{tool_name}"
            yield StreamChunkEvent(
                event=StreamEventType.TOOL_START,
                data={"tool": tool_name, "call_id": call_id},
            )
            yield StreamChunkEvent(
                event=StreamEventType.TOOL_END,
                data={
                    "tool": tool_name,
                    "call_id": call_id,
                    "grounded": response.grounded,
                    "duration_ms": 15,
                },
            )

        # 3. Emit any generated action cards
        for action in response.actions:
            yield StreamChunkEvent(
                event=StreamEventType.ACTION_CARD,
                data=action.model_dump(),
            )

        # 4. Stream tokens word by word / chunk by chunk
        tokens = re.findall(r"\S+\s*|\n+", response.reply)
        for token in tokens:
            yield StreamChunkEvent(
                event=StreamEventType.TOKEN,
                data={"delta": token},
            )

        # 5. Emit terminal done event
        yield StreamChunkEvent(
            event=StreamEventType.DONE,
            data={
                "conversation_id": str(response.conversation_id),
                "latency_ms": response.latency_ms,
                "tools_called": response.tools_called,
            },
        )
