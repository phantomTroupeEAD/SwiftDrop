"""Production OpenAI-compatible LLM orchestration engine with recursive function calling."""

import json
import re
import time
from typing import Any, AsyncGenerator, Dict, List, Optional
import uuid
from uuid import UUID
import httpx
from openai import (
    APIConnectionError,
    APIStatusError,
    APITimeoutError,
    AsyncOpenAI,
    RateLimitError,
)

from src.core.config import get_settings
from src.domain.chat import (
    ChatMessage,
    ChatMessageResponse,
    GenerativeAction,
    StreamChunkEvent,
    StreamEventType,
)
from src.domain.errors import RateLimitException, UpstreamServiceException
from src.llm.base import BaseLLMClient
from src.llm.guardrails import sanitize_assistant_identity
from src.llm.prompts import SYSTEM_PROMPT
from src.tools.registry import execute_tool, export_openai_tools

# Hard cap on function-calling loop iterations to eliminate infinite loops and denial-of-wallet
MAX_TOOL_ITERATIONS = 3

XML_FUNCTION_RE = re.compile(r"<function\s*=\s*([a-zA-Z0-9_\-]+)>([\s\S]*?)<\/function>", re.IGNORECASE)
XML_PARAM_RE = re.compile(r"<parameter\s*=\s*([a-zA-Z0-9_\-]+)>([\s\S]*?)<\/parameter>", re.IGNORECASE)
XML_TOOL_CALL_BLOCK_RE = re.compile(r"<tool_call>([\s\S]*?)<\/tool_call>", re.IGNORECASE)


def _convert_param_value(key: str, val: str) -> Any:
    cleaned = val.strip()
    if (cleaned.startswith('"') and cleaned.endswith('"')) or (cleaned.startswith("'") and cleaned.endswith("'")):
        cleaned = cleaned[1:-1].strip()

    if (
        key.endswith("postal_code")
        or key.endswith("_code")
        or key.endswith("_number")
        or key == "service_type"
        or key == "status"
        or (len(cleaned) == 5 and cleaned.isdigit())
    ):
        return cleaned

    if cleaned.lower() == "true":
        return True
    if cleaned.lower() == "false":
        return False

    try:
        if cleaned.isdigit() or (cleaned.startswith("-") and cleaned[1:].isdigit()):
            return int(cleaned)
        return float(cleaned)
    except ValueError:
        pass

    try:
        return json.loads(cleaned)
    except Exception:
        pass

    return cleaned


def parse_text_tool_calls(text: str) -> List[Dict[str, Any]]:
    """Extract tool calls embedded as XML or JSON inside model text."""
    if not text or ("<function" not in text and "<tool_call" not in text and "```json" not in text):
        return []

    calls: List[Dict[str, Any]] = []

    # Format 1: XML <function=name><parameter=k>v</parameter></function>
    for match in XML_FUNCTION_RE.finditer(text):
        fn_name = match.group(1).strip()
        body = match.group(2)
        args: Dict[str, Any] = {}
        for p_match in XML_PARAM_RE.finditer(body):
            p_name = p_match.group(1).strip()
            p_val = _convert_param_value(p_name, p_match.group(2))
            args[p_name] = p_val
        calls.append({
            "id": f"call_{uuid.uuid4().hex[:8]}",
            "name": fn_name,
            "arguments": args,
        })

    if calls:
        return calls

    # Format 2: <tool_call>{"name": "...", "arguments": {...}}</tool_call>
    for block in XML_TOOL_CALL_BLOCK_RE.finditer(text):
        content = block.group(1).strip()
        try:
            parsed = json.loads(content)
            if isinstance(parsed, dict):
                name = parsed.get("name") or parsed.get("function")
                raw_args = parsed.get("arguments") or parsed.get("parameters") or {}
                if isinstance(raw_args, str):
                    try:
                        raw_args = json.loads(raw_args)
                    except Exception:
                        pass
                if name and isinstance(raw_args, dict):
                    calls.append({
                        "id": f"call_{uuid.uuid4().hex[:8]}",
                        "name": str(name),
                        "arguments": raw_args,
                    })
        except Exception:
            pass

    return calls


def strip_tool_call_tags(text: str) -> str:
    """Strip XML tool call tags and blocks so raw tags never leak to users."""
    if not text:
        return ""
    cleaned = XML_TOOL_CALL_BLOCK_RE.sub("", text)
    cleaned = XML_FUNCTION_RE.sub("", cleaned)
    cleaned = re.sub(r"<tool_call>[\s\S]*?$", "", cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r"<function\s*=[\s\S]*?$", "", cleaned, flags=re.IGNORECASE)
    return cleaned.strip()


class OpenAIAdapter(BaseLLMClient):
    """Universal OpenAI-compatible client supporting OpenRouter, Groq, NVIDIA NIM, and Ollama."""

    def __init__(self, client: Optional[AsyncOpenAI] = None):
        """Initialize adapter with an optional injected client for testing."""
        self._settings = get_settings()
        if client is not None:
            self._client = client
        else:
            api_key = self._settings.LLM_API_KEY.get_secret_value() or "mock-key"
            timeouts = httpx.Timeout(15.0, connect=3.0)
            self._client = AsyncOpenAI(
                base_url=self._settings.LLM_BASE_URL,
                api_key=api_key,
                timeout=timeouts,
            )

    async def generate_reply(
        self,
        prompt: str,
        history: List[ChatMessage],
        conversation_id: UUID,
        context: Optional[Dict[str, Any]] = None,
    ) -> ChatMessageResponse:
        """Execute multi-turn reasoning loop, dispatching tools and synthesizing final grounded response."""
        start_time = time.perf_counter()
        tools_called: List[str] = []
        actions: List[GenerativeAction] = []

        # 1. Prepare message history with system prompt
        messages: List[Dict[str, Any]] = [{"role": "system", "content": SYSTEM_PROMPT}]
        for turn in history:
            messages.append({"role": turn.role.value, "content": turn.content})
        messages.append({"role": "user", "content": prompt})

        openai_tools = export_openai_tools()
        iterations = 0
        final_reply = ""

        try:
            # 2. Multi-turn function calling execution loop
            while iterations < MAX_TOOL_ITERATIONS:
                response = await self._client.chat.completions.create(
                    model=self._settings.LLM_MODEL,
                    messages=messages,
                    tools=openai_tools if openai_tools else None,
                    tool_choice="auto" if openai_tools else None,
                    temperature=self._settings.LLM_TEMPERATURE,
                    max_tokens=self._settings.LLM_MAX_TOKENS,
                )

                choice = response.choices[0]
                message = choice.message

                text_tool_calls = []
                if not message.tool_calls and message.content:
                    text_tool_calls = parse_text_tool_calls(message.content)

                # Case A: Model responded with final text answer
                if not message.tool_calls and not text_tool_calls:
                    final_reply = strip_tool_call_tags(message.content or "")
                    break

                # Case B: Model emitted one or more tool calls (either structured or text XML)
                iterations += 1

                if message.tool_calls:
                    # Append assistant tool call request to conversation history
                    tool_calls_dict = []
                    for tc in message.tool_calls:
                        tool_calls_dict.append(
                            {
                                "id": tc.id,
                                "type": "function",
                                "function": {
                                    "name": tc.function.name,
                                    "arguments": tc.function.arguments,
                                },
                            }
                        )

                    messages.append(
                        {
                            "role": "assistant",
                            "content": message.content,
                            "tool_calls": tool_calls_dict,
                        }
                    )

                    # Execute requested tools
                    for tc in message.tool_calls:
                        fn_name = tc.function.name
                        tools_called.append(fn_name)

                        try:
                            args = json.loads(tc.function.arguments) if tc.function.arguments else {}
                        except (json.JSONDecodeError, ValueError):
                            args = {}

                        tool_record = await execute_tool(fn_name, args, context=context)

                        # Extract generative UI actions attached to tool results
                        if (
                            tool_record.status == "SUCCESS"
                            and isinstance(tool_record.result, dict)
                            and "action" in tool_record.result
                        ):
                            try:
                                act_data = tool_record.result["action"]
                                actions.append(GenerativeAction(**act_data))
                            except Exception:
                                pass

                        # Format tool output for next turn
                        content_str = (
                            json.dumps(tool_record.result)
                            if tool_record.status == "SUCCESS"
                            else json.dumps({"error": tool_record.error_message})
                        )

                        messages.append(
                            {
                                "role": "tool",
                                "tool_call_id": tc.id,
                                "content": content_str,
                            }
                        )
                else:
                    # Model emitted text tool calls (e.g. Nemotron XML)
                    clean_content = strip_tool_call_tags(message.content or "")
                    messages.append(
                        {
                            "role": "assistant",
                            "content": message.content,
                        }
                    )
                    for tt in text_tool_calls:
                        fn_name = tt["name"]
                        tools_called.append(fn_name)
                        args = tt["arguments"]

                        tool_record = await execute_tool(fn_name, args, context=context)

                        if (
                            tool_record.status == "SUCCESS"
                            and isinstance(tool_record.result, dict)
                            and "action" in tool_record.result
                        ):
                            try:
                                act_data = tool_record.result["action"]
                                actions.append(GenerativeAction(**act_data))
                            except Exception:
                                pass

                        content_str = (
                            json.dumps(tool_record.result)
                            if tool_record.status == "SUCCESS"
                            else json.dumps({"error": tool_record.error_message})
                        )

                        messages.append(
                            {
                                "role": "user",
                                "content": (
                                    f"[Tool Result for {fn_name}]:\n{content_str}\n\n"
                                    "Please provide the final, grounded response to the customer using this tool result. "
                                    "Do not output raw tool call tags."
                                ),
                            }
                        )

                # Cap enforcement: If loop limit reached, force final text synthesis without tools
                if iterations >= MAX_TOOL_ITERATIONS:
                    forced_completion = await self._client.chat.completions.create(
                        model=self._settings.LLM_MODEL,
                        messages=messages,
                        temperature=self._settings.LLM_TEMPERATURE,
                        max_tokens=self._settings.LLM_MAX_TOKENS,
                    )
                    final_reply = strip_tool_call_tags(forced_completion.choices[0].message.content or "")
                    break

        except RateLimitError as rle:
            raise RateLimitException(retry_after_seconds=30) from rle
        except APITimeoutError as toe:
            raise UpstreamServiceException("LLM Provider", "Inference request timed out") from toe
        except APIConnectionError as ace:
            raise UpstreamServiceException(
                "LLM Provider", f"Connection to LLM host failed: {ace}"
            ) from ace
        except APIStatusError as ase:
            raise UpstreamServiceException(
                "LLM Provider", f"Provider error HTTP {ase.status_code}: {ase.message}"
            ) from ase

        latency_ms = int((time.perf_counter() - start_time) * 1000)

        final_reply = sanitize_assistant_identity(strip_tool_call_tags(final_reply))

        return ChatMessageResponse(
            conversation_id=conversation_id,
            reply=final_reply,
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
        """Execute streaming reasoning loop, yielding tool events and incremental token chunks."""
        start_time = time.perf_counter()
        tools_called: List[str] = []
        actions: List[GenerativeAction] = []

        messages: List[Dict[str, Any]] = [{"role": "system", "content": SYSTEM_PROMPT}]
        for turn in history:
            messages.append({"role": turn.role.value, "content": turn.content})
        messages.append({"role": "user", "content": prompt})

        openai_tools = export_openai_tools()
        iterations = 0

        try:
            while iterations < MAX_TOOL_ITERATIONS:
                stream = await self._client.chat.completions.create(
                    model=self._settings.LLM_MODEL,
                    messages=messages,
                    tools=openai_tools if openai_tools else None,
                    tool_choice="auto" if openai_tools else None,
                    temperature=self._settings.LLM_TEMPERATURE,
                    max_tokens=self._settings.LLM_MAX_TOKENS,
                    stream=True,
                )

                tool_calls_acc: Dict[int, Dict[str, str]] = {}
                final_content_acc = ""
                is_text_tool_calling = False
                token_buffer = ""

                async for chunk in stream:
                    if not chunk.choices:
                        continue
                    delta = chunk.choices[0].delta

                    # 1. Accumulate tool call deltas
                    if delta.tool_calls:
                        for tc in delta.tool_calls:
                            idx = tc.index if tc.index is not None else 0
                            entry = tool_calls_acc.setdefault(
                                idx, {"id": "", "name": "", "arguments": ""}
                            )
                            if tc.id:
                                entry["id"] += tc.id
                            if tc.function and tc.function.name:
                                entry["name"] += tc.function.name
                            if tc.function and tc.function.arguments:
                                entry["arguments"] += tc.function.arguments

                    # 2. Handle text content deltas with tool-call suppression
                    if delta.content:
                        final_content_acc += delta.content

                        if not is_text_tool_calling:
                            stripped_acc = final_content_acc.strip()
                            if "<tool_call" in final_content_acc or "<function=" in final_content_acc:
                                is_text_tool_calling = True
                                token_buffer = ""
                            elif stripped_acc.startswith("<") and len(stripped_acc) < 15:
                                token_buffer += delta.content
                            else:
                                if token_buffer:
                                    yield StreamChunkEvent(
                                        event=StreamEventType.TOKEN,
                                        data={"delta": token_buffer},
                                    )
                                    token_buffer = ""
                                yield StreamChunkEvent(
                                    event=StreamEventType.TOKEN,
                                    data={"delta": delta.content},
                                )

                # Flush any remaining buffer if not a tool call
                if token_buffer and not is_text_tool_calling:
                    yield StreamChunkEvent(
                        event=StreamEventType.TOKEN,
                        data={"delta": token_buffer},
                    )
                    token_buffer = ""

                # Check if text tool calls were embedded in the model response
                text_tool_calls = []
                if not tool_calls_acc and is_text_tool_calling:
                    text_tool_calls = parse_text_tool_calls(final_content_acc)

                # If no native tool calls AND no text tool calls were requested, terminal text stream complete
                if not tool_calls_acc and not text_tool_calls:
                    break

                # Otherwise, execute requested tools
                iterations += 1

                if tool_calls_acc:
                    tool_calls_dict = []
                    for _, call_info in sorted(tool_calls_acc.items()):
                        tool_calls_dict.append(
                            {
                                "id": call_info["id"],
                                "type": "function",
                                "function": {
                                    "name": call_info["name"],
                                    "arguments": call_info["arguments"],
                                },
                            }
                        )

                    messages.append(
                        {
                            "role": "assistant",
                            "content": final_content_acc or None,
                            "tool_calls": tool_calls_dict,
                        }
                    )

                    for _, call_info in sorted(tool_calls_acc.items()):
                        fn_name = call_info["name"]
                        call_id = call_info["id"]
                        tools_called.append(fn_name)

                        # Emit TOOL_START event
                        yield StreamChunkEvent(
                            event=StreamEventType.TOOL_START,
                            data={"tool": fn_name, "call_id": call_id},
                        )

                        try:
                            args = json.loads(call_info["arguments"]) if call_info["arguments"] else {}
                        except (json.JSONDecodeError, ValueError):
                            args = {}

                        tool_record = await execute_tool(fn_name, args, context=context)

                        # Emit TOOL_END event
                        yield StreamChunkEvent(
                            event=StreamEventType.TOOL_END,
                            data={
                                "tool": fn_name,
                                "call_id": call_id,
                                "grounded": tool_record.status == "SUCCESS",
                                "duration_ms": tool_record.duration_ms or 0,
                            },
                        )

                        # Extract generative action cards
                        if (
                            tool_record.status == "SUCCESS"
                            and isinstance(tool_record.result, dict)
                            and "action" in tool_record.result
                        ):
                            try:
                                act_data = tool_record.result["action"]
                                action_obj = GenerativeAction(**act_data)
                                actions.append(action_obj)
                                yield StreamChunkEvent(
                                    event=StreamEventType.ACTION_CARD,
                                    data=action_obj.model_dump(),
                                )
                            except Exception:
                                pass

                        content_str = (
                            json.dumps(tool_record.result)
                            if tool_record.status == "SUCCESS"
                            else json.dumps({"error": tool_record.error_message})
                        )
                        messages.append(
                            {
                                "role": "tool",
                                "tool_call_id": call_id,
                                "content": content_str,
                            }
                        )
                else:
                    # Text XML tool calls
                    messages.append(
                        {
                            "role": "assistant",
                            "content": final_content_acc,
                        }
                    )
                    for tt in text_tool_calls:
                        fn_name = tt["name"]
                        call_id = tt["id"]
                        args = tt["arguments"]
                        tools_called.append(fn_name)

                        yield StreamChunkEvent(
                            event=StreamEventType.TOOL_START,
                            data={"tool": fn_name, "call_id": call_id},
                        )

                        tool_record = await execute_tool(fn_name, args, context=context)

                        yield StreamChunkEvent(
                            event=StreamEventType.TOOL_END,
                            data={
                                "tool": fn_name,
                                "call_id": call_id,
                                "grounded": tool_record.status == "SUCCESS",
                                "duration_ms": tool_record.duration_ms or 0,
                            },
                        )

                        if (
                            tool_record.status == "SUCCESS"
                            and isinstance(tool_record.result, dict)
                            and "action" in tool_record.result
                        ):
                            try:
                                act_data = tool_record.result["action"]
                                action_obj = GenerativeAction(**act_data)
                                actions.append(action_obj)
                                yield StreamChunkEvent(
                                    event=StreamEventType.ACTION_CARD,
                                    data=action_obj.model_dump(),
                                )
                            except Exception:
                                pass

                        content_str = (
                            json.dumps(tool_record.result)
                            if tool_record.status == "SUCCESS"
                            else json.dumps({"error": tool_record.error_message})
                        )
                        messages.append(
                            {
                                "role": "user",
                                "content": (
                                    f"[Tool Result for {fn_name}]:\n{content_str}\n\n"
                                    "Please provide the final, grounded response to the customer using this tool result. "
                                    "Do not output raw tool call tags."
                                ),
                            }
                        )

                # Cap enforcement: If loop limit reached, force final text synthesis stream without tools
                if iterations >= MAX_TOOL_ITERATIONS:
                    forced_stream = await self._client.chat.completions.create(
                        model=self._settings.LLM_MODEL,
                        messages=messages,
                        temperature=self._settings.LLM_TEMPERATURE,
                        max_tokens=self._settings.LLM_MAX_TOKENS,
                        stream=True,
                    )
                    async for forced_chunk in forced_stream:
                        if forced_chunk.choices and forced_chunk.choices[0].delta.content:
                            text_chunk = strip_tool_call_tags(forced_chunk.choices[0].delta.content)
                            if text_chunk:
                                yield StreamChunkEvent(
                                    event=StreamEventType.TOKEN,
                                    data={"delta": text_chunk},
                                )
                    break

        except RateLimitError as rle:
            yield StreamChunkEvent(
                event=StreamEventType.ERROR,
                data={"code": "RATE_LIMIT_EXCEEDED", "message": "Upstream LLM rate limit exceeded"},
            )
            raise RateLimitException(retry_after_seconds=30) from rle
        except (APITimeoutError, APIConnectionError, APIStatusError) as exc:
            yield StreamChunkEvent(
                event=StreamEventType.ERROR,
                data={"code": "UPSTREAM_SERVICE_UNAVAILABLE", "message": str(exc)},
            )
            raise UpstreamServiceException("LLM Provider", str(exc)) from exc

        latency_ms = int((time.perf_counter() - start_time) * 1000)

        # Yield terminal DONE event
        yield StreamChunkEvent(
            event=StreamEventType.DONE,
            data={
                "conversation_id": str(conversation_id),
                "latency_ms": latency_ms,
                "tools_called": tools_called,
            },
        )
