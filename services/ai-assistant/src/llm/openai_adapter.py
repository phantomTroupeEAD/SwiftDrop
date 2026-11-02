"""Production OpenAI-compatible LLM orchestration engine with recursive function calling."""

import json
import time
from typing import Any, Dict, List, Optional
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
)
from src.domain.errors import RateLimitException, UpstreamServiceException
from src.llm.base import BaseLLMClient
from src.llm.guardrails import sanitize_assistant_identity
from src.llm.prompts import SYSTEM_PROMPT
from src.tools.registry import execute_tool, export_openai_tools

# Hard cap on function-calling loop iterations to eliminate infinite loops and denial-of-wallet
MAX_TOOL_ITERATIONS = 3


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

                # Case A: Model responded with final text answer
                if not message.tool_calls:
                    final_reply = message.content or ""
                    break

                # Case B: Model emitted one or more tool calls
                iterations += 1

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

                # Cap enforcement: If loop limit reached, force final text synthesis without tools
                if iterations >= MAX_TOOL_ITERATIONS:
                    forced_completion = await self._client.chat.completions.create(
                        model=self._settings.LLM_MODEL,
                        messages=messages,
                        temperature=self._settings.LLM_TEMPERATURE,
                        max_tokens=self._settings.LLM_MAX_TOKENS,
                    )
                    final_reply = forced_completion.choices[0].message.content or ""
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

        final_reply = sanitize_assistant_identity(final_reply)

        return ChatMessageResponse(
            conversation_id=conversation_id,
            reply=final_reply,
            actions=actions,
            tools_called=tools_called,
            latency_ms=latency_ms,
            grounded=True,
        )
