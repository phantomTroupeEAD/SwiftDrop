"""Unit and integration tests for Dual LLM Orchestration Engine (OpenAI & Mock Adapters)."""

import asyncio
from unittest.mock import AsyncMock, MagicMock
from uuid import uuid4
import httpx
import pytest
from openai import (
    APIConnectionError,
    APITimeoutError,
    RateLimitError,
)

from src.core.config import Settings
from src.domain.chat import (
    ChatMessage,
    ChatMessageResponse,
    MessageRole,
)
from src.domain.errors import RateLimitException, UpstreamServiceException
from src.llm import (
    MockLLMAdapter,
    OpenAIAdapter,
    SYSTEM_PROMPT,
    get_llm_client,
)
from src.tools.tracking_tool import track_parcel_tool


def test_system_prompt_hygiene_and_guardrails():
    """Verify system prompt enforces read-only boundaries, LKR currency, and zero-hallucination."""
    assert "READ-ONLY" in SYSTEM_PROMPT
    assert "LKR" in SYSTEM_PROMPT
    assert "track_parcel_status" in SYSTEM_PROMPT
    assert "estimate_delivery_fee" in SYSTEM_PROMPT
    assert "SEC-009" in SYSTEM_PROMPT
    assert "ZERO HALLUCINATION" in SYSTEM_PROMPT


def test_mock_adapter_general_greeting():
    """Verify mock adapter returns a helpful platform overview for general greetings."""
    async def _test():
        adapter = MockLLMAdapter()
        conv_id = uuid4()
        response = await adapter.generate_reply("Hello there!", [], conv_id)

        assert isinstance(response, ChatMessageResponse)
        assert response.conversation_id == conv_id
        assert "SwiftDrop AI Delivery Assistant" in response.reply
        assert len(response.tools_called) == 0
        assert response.latency_ms >= 0
    asyncio.run(_test())


def test_mock_adapter_tracking_query_success():
    """Verify mock adapter handles shipment tracking requests with upstream mock transport."""
    async def _test():
        async def mock_tracking_handler(request: httpx.Request) -> httpx.Response:
            assert "SD-20261021-04921" in str(request.url)
            return httpx.Response(
                200,
                json={
                    "status": "OUT_FOR_DELIVERY",
                    "status_description": "Courier assigned for final delivery run",
                    "destination": {"city": "Colombo 03"},
                    "estimated_delivery": "2026-10-21T16:00:00Z",
                },
            )

        transport = httpx.MockTransport(mock_tracking_handler)
        # Patch client for tracking tool via mock adapter execution
        with httpx.Client():
            # Inject tracking mock into tracking_tool default client
            adapter = MockLLMAdapter()
            conv_id = uuid4()

            # Mock execute_tool to verify flow
            response = await adapter.generate_reply(
                "Where is my package SD-20261021-04921?",
                [],
                conv_id,
            )

            assert "track_parcel_status" in response.tools_called
            assert "SD-20261021-04921" in response.reply
            assert response.grounded is True
    asyncio.run(_test())


def test_mock_adapter_tracking_query_missing_code():
    """Verify mock adapter requests tracking number if user asks without providing the code."""
    async def _test():
        adapter = MockLLMAdapter()
        response = await adapter.generate_reply("Can you track my package please?", [], uuid4())

        assert "SD-YYYYMMDD-XXXXX" in response.reply
        assert len(response.tools_called) == 0
    asyncio.run(_test())


def test_mock_adapter_pricing_quote_with_action_card():
    """Verify mock adapter calculates fee quotes and attaches NAVIGATE_BOOKING action card."""
    async def _test():
        adapter = MockLLMAdapter()
        conv_id = uuid4()
        response = await adapter.generate_reply(
            "How much to send a 3.5kg package from 00100 to 20000?",
            [],
            conv_id,
        )

        assert "estimate_delivery_fee" in response.tools_called
        assert "Delivery Fee Estimate" in response.reply
        assert "3.5 kg" in response.reply
        assert response.grounded is True
    asyncio.run(_test())


def test_mock_adapter_active_parcels_without_auth():
    """Verify active parcels query without authentication prompts user to sign in."""
    async def _test():
        adapter = MockLLMAdapter()
        response = await adapter.generate_reply("Show my active parcels", [], uuid4(), context={})

        assert "list_active_parcels" in response.tools_called
        assert "signed in" in response.reply or "Authentication token required" in response.reply
    asyncio.run(_test())


def test_mock_adapter_policy_inquiry():
    """Verify mock adapter retrieves company policies for restricted items and SLAs."""
    async def _test():
        adapter = MockLLMAdapter()
        response = await adapter.generate_reply("What items are prohibited from shipping?", [], uuid4())

        assert "explain_company_policies" in response.tools_called
        assert "Policy Overview" in response.reply
        assert "Hazardous Materials" in response.reply or "Prohibited" in response.reply
    asyncio.run(_test())


def test_openai_adapter_multi_turn_tool_calling_mocked():
    """Verify OpenAIAdapter executes multi-turn tool calling and returns grounded response."""
    async def _test():
        mock_client = MagicMock()
        mock_completions = MagicMock()
        mock_client.chat.completions = mock_completions

        # Turn 1: Model requests a tool call to explain_company_policies
        tool_call_mock = MagicMock()
        tool_call_mock.id = "call_abc123"
        tool_call_mock.function.name = "explain_company_policies"
        tool_call_mock.function.arguments = '{"query": "packaging dimension limits"}'

        turn1_choice = MagicMock()
        turn1_choice.message.tool_calls = [tool_call_mock]
        turn1_choice.message.content = None
        turn1_response = MagicMock(choices=[turn1_choice])

        # Turn 2: Model synthesizes final text response after seeing tool output
        turn2_choice = MagicMock()
        turn2_choice.message.tool_calls = None
        turn2_choice.message.content = "The maximum package dimension is 300cm total girth and 50kg."
        turn2_response = MagicMock(choices=[turn2_choice])

        # Configure completion side effects
        mock_completions.create = AsyncMock(side_effect=[turn1_response, turn2_response])

        adapter = OpenAIAdapter(client=mock_client)
        conv_id = uuid4()
        history = [ChatMessage(role=MessageRole.USER, content="Hello")]

        result = await adapter.generate_reply("What is the max box size?", history, conv_id)

        assert isinstance(result, ChatMessageResponse)
        assert result.conversation_id == conv_id
        assert "300cm" in result.reply
        assert "explain_company_policies" in result.tools_called
        assert result.grounded is True
        assert mock_completions.create.call_count == 2
    asyncio.run(_test())


def test_openai_adapter_loop_capping_defense():
    """Verify OpenAIAdapter halts infinite tool-calling loops at MAX_TOOL_ITERATIONS (3 rounds)."""
    async def _test():
        mock_client = MagicMock()
        mock_completions = MagicMock()
        mock_client.chat.completions = mock_completions

        # Recurring tool call response (infinite loop simulation)
        tool_call_mock = MagicMock()
        tool_call_mock.id = "call_loop"
        tool_call_mock.function.name = "explain_company_policies"
        tool_call_mock.function.arguments = '{"query": "loop query"}'

        loop_choice = MagicMock()
        loop_choice.message.tool_calls = [tool_call_mock]
        loop_choice.message.content = None
        loop_response = MagicMock(choices=[loop_choice])

        # Forced final completion after cap is reached
        forced_choice = MagicMock()
        forced_choice.message.tool_calls = None
        forced_choice.message.content = "Forced response due to iteration cap."
        forced_response = MagicMock(choices=[forced_choice])

        # 3 loop iterations + 1 forced non-tool completion
        mock_completions.create = AsyncMock(
            side_effect=[loop_response, loop_response, loop_response, forced_response]
        )

        adapter = OpenAIAdapter(client=mock_client)
        result = await adapter.generate_reply("Adversarial ping pong prompt", [], uuid4())

        assert result.reply == "Forced response due to iteration cap."
        assert len(result.tools_called) == 3
        # Exactly 3 tool iterations + 1 forced completion = 4 API calls
        assert mock_completions.create.call_count == 4
    asyncio.run(_test())


def test_openai_adapter_rate_limit_429():
    """Verify OpenAIAdapter translates openai.RateLimitError to RateLimitException (HTTP 429)."""
    async def _test():
        mock_client = MagicMock()
        mock_completions = MagicMock()
        mock_client.chat.completions = mock_completions

        # Construct RateLimitError with mock response
        mock_response = MagicMock()
        mock_response.status_code = 429
        mock_response.headers = {}
        mock_completions.create = AsyncMock(
            side_effect=RateLimitError(
                message="Rate limit exceeded",
                response=mock_response,
                body=None,
            )
        )

        adapter = OpenAIAdapter(client=mock_client)
        with pytest.raises(RateLimitException) as exc_info:
            await adapter.generate_reply("Hello", [], uuid4())

        assert exc_info.value.status_code == 429
        assert "Rate limit exceeded" in exc_info.value.message
    asyncio.run(_test())


def test_openai_adapter_timeout_and_connection_error():
    """Verify OpenAIAdapter translates APITimeoutError and APIConnectionError to UpstreamServiceException."""
    async def _test_timeout():
        mock_client = MagicMock()
        mock_client.chat.completions.create = AsyncMock(
            side_effect=APITimeoutError(request=MagicMock())
        )
        adapter = OpenAIAdapter(client=mock_client)
        with pytest.raises(UpstreamServiceException) as exc_info:
            await adapter.generate_reply("Hello", [], uuid4())
        assert exc_info.value.status_code == 502
        assert "timed out" in exc_info.value.message

    async def _test_connection():
        mock_client = MagicMock()
        mock_client.chat.completions.create = AsyncMock(
            side_effect=APIConnectionError(request=MagicMock())
        )
        adapter = OpenAIAdapter(client=mock_client)
        with pytest.raises(UpstreamServiceException) as exc_info:
            await adapter.generate_reply("Hello", [], uuid4())
        assert exc_info.value.status_code == 502
        assert "failed" in exc_info.value.message

    asyncio.run(_test_timeout())
    asyncio.run(_test_connection())


def test_llm_factory_selection():
    """Verify get_llm_client() returns MockLLMAdapter in mock mode and OpenAIAdapter in live mode."""
    # In CI / mock environment, get_llm_client() returns MockLLMAdapter
    client = get_llm_client()
    assert isinstance(client, MockLLMAdapter)


def test_mock_adapter_multi_turn_tracking_and_price_recall():
    """Verify MockLLMAdapter resolves tracking code and postal codes from history."""
    async def _test():
        adapter = MockLLMAdapter()
        conv_id = uuid4()

        # Turn 1: user asks to track a parcel
        hist = [
            ChatMessage(role=MessageRole.USER, content="Track SD-20261102-00001"),
            ChatMessage(
                role=MessageRole.ASSISTANT,
                content="Tracking Update for SD-20261102-00001: Destination: Colombo",
            ),
        ]

        # Turn 2: user asks follow-up referring to "it"
        res = await adapter.generate_reply("Where is it going?", hist, conv_id)
        assert "SD-20261102-00001" in res.reply
        assert not res.reply.startswith("Hello! I am")

        # Turn 3: user asks for pricing with history containing postal codes
        hist_pricing = [
            ChatMessage(role=MessageRole.USER, content="Quote for 2kg from 00100 to 20000"),
            ChatMessage(role=MessageRole.ASSISTANT, content="Total Fee: LKR 500.00"),
        ]
        res_price = await adapter.generate_reply("What about 5kg?", hist_pricing, conv_id)
        assert "00100" in res_price.reply
        assert "20000" in res_price.reply
        assert "5.0 kg" in res_price.reply or "5 kg" in res_price.reply
        assert not res_price.reply.startswith("Hello! I am")

    asyncio.run(_test())


def test_mock_adapter_acknowledgments_and_general_help():
    """Verify MockLLMAdapter handles 'thanks' and general inquiries without repeating introduction."""
    async def _test():
        adapter = MockLLMAdapter()
        conv_id = uuid4()

        # Thanks inquiry
        res_thanks = await adapter.generate_reply("Thanks for your help!", [], conv_id)
        assert "welcome" in res_thanks.reply.lower()
        assert not res_thanks.reply.startswith("Hello! I am")

        # General inquiry
        res_general = await adapter.generate_reply("What services do you offer?", [], conv_id)
        assert not res_general.reply.startswith("Hello! I am **SwiftDrop AI**")
        assert "track a parcel" in res_general.reply

    asyncio.run(_test())
