"""Unit tests for MockLLMAdapter and OpenAIAdapter streaming capabilities."""

import json
from unittest.mock import AsyncMock, MagicMock, patch
import uuid
import pytest
from openai import APITimeoutError, RateLimitError

from src.domain.chat import (
    ChatMessage,
    MessageRole,
    StreamChunkEvent,
    StreamEventType,
    to_sse_frame,
)
from src.domain.errors import RateLimitException, UpstreamServiceException
from src.llm.mock_adapter import MockLLMAdapter
from src.llm.openai_adapter import OpenAIAdapter


def test_sse_frame_serializer():
    """Verify to_sse_frame formats Pydantic models, dicts, and primitive values as standard SSE."""
    # 1. From StreamChunkEvent
    chunk = StreamChunkEvent(event=StreamEventType.TOKEN, data={"delta": "Hello"})
    frame = chunk.to_sse_frame()
    assert frame == 'event: token\ndata: {"delta":"Hello"}\n\n'

    # 2. From to_sse_frame helper with dict
    f2 = to_sse_frame("tool_start", {"tool": "test_tool"})
    assert f2 == 'event: tool_start\ndata: {"tool":"test_tool"}\n\n'

    # 3. From to_sse_frame helper with primitive
    f3 = to_sse_frame(StreamEventType.PING, "keepalive")
    assert f3 == 'event: ping\ndata: {"value":"keepalive"}\n\n'


@pytest.mark.asyncio
async def test_mock_llm_adapter_stream_tracking_intent():
    """Verify MockLLMAdapter yields tool_start, tool_end, tokens, and done events."""
    adapter = MockLLMAdapter()
    conv_id = uuid.uuid4()
    events = []

    async for chunk in adapter.generate_reply_stream(
        prompt="Where is package SD-20261021-04921?",
        history=[],
        conversation_id=conv_id,
    ):
        events.append(chunk)

    event_types = [e.event for e in events]
    assert StreamEventType.TOOL_START in event_types
    assert StreamEventType.TOOL_END in event_types
    assert StreamEventType.TOKEN in event_types
    assert StreamEventType.DONE in event_types

    # Verify tool events
    tool_start = next(e for e in events if e.event == StreamEventType.TOOL_START)
    assert tool_start.data["tool"] == "track_parcel_status"

    # Verify done event
    done_event = next(e for e in events if e.event == StreamEventType.DONE)
    assert done_event.data["conversation_id"] == str(conv_id)
    assert "track_parcel_status" in done_event.data["tools_called"]

    # Verify reconstructed tokens match full reply
    tokens = [e.data["delta"] for e in events if e.event == StreamEventType.TOKEN]
    reconstructed = "".join(tokens)
    assert "SD-20261021-04921" in reconstructed


@pytest.mark.asyncio
async def test_mock_llm_adapter_stream_action_card_intent():
    """Verify MockLLMAdapter yields action_card event during quote inquiry."""
    adapter = MockLLMAdapter()
    conv_id = uuid.uuid4()
    events = []

    async for chunk in adapter.generate_reply_stream(
        prompt="How much to ship 2.5kg from 00100 to 20000?",
        history=[],
        conversation_id=conv_id,
    ):
        events.append(chunk)

    event_types = [e.event for e in events]
    assert StreamEventType.ACTION_CARD in event_types
    action_event = next(e for e in events if e.event == StreamEventType.ACTION_CARD)
    assert action_event.data["action_type"] == "NAVIGATE_BOOKING"
    assert action_event.data["target_url"] == "/portal/book"


@pytest.mark.asyncio
async def test_openai_adapter_stream_text_and_tool_events():
    """Verify OpenAIAdapter accumulates deltas, executes tools, and streams tokens."""
    mock_openai_client = AsyncMock()
    adapter = OpenAIAdapter(client=mock_openai_client)
    conv_id = uuid.uuid4()

    # Create mock chunk for text streaming
    mock_chunk1 = MagicMock()
    mock_chunk1.choices = [MagicMock(delta=MagicMock(content="Hello ", tool_calls=None))]

    mock_chunk2 = MagicMock()
    mock_chunk2.choices = [MagicMock(delta=MagicMock(content="world!", tool_calls=None))]

    # Async generator returning mock chunks
    async def mock_stream():
        yield mock_chunk1
        yield mock_chunk2

    mock_openai_client.chat.completions.create.return_value = mock_stream()

    events = []
    async for chunk in adapter.generate_reply_stream(
        prompt="Hi",
        history=[],
        conversation_id=conv_id,
    ):
        events.append(chunk)

    token_deltas = [e.data["delta"] for e in events if e.event == StreamEventType.TOKEN]
    assert token_deltas == ["Hello ", "world!"]
    assert events[-1].event == StreamEventType.DONE


@pytest.mark.asyncio
async def test_openai_adapter_stream_with_tool_call_deltas():
    """Verify OpenAIAdapter handles tool call delta accumulation and second streaming pass."""
    mock_openai_client = AsyncMock()
    adapter = OpenAIAdapter(client=mock_openai_client)
    conv_id = uuid.uuid4()

    # Stream 1: Emits tool call chunks
    tc_delta1 = MagicMock()
    tc_delta1.choices = [
        MagicMock(
            delta=MagicMock(
                content=None,
                tool_calls=[
                    MagicMock(
                        index=0,
                        id="call_999",
                        function=MagicMock(name="track_parcel_status", arguments='{"tracking_number": "SD-20261021-04921"}'),
                    )
                ],
            )
        )
    ]

    async def stream1():
        yield tc_delta1

    # Stream 2: After tool execution, emits final text
    text_delta = MagicMock()
    text_delta.choices = [
        MagicMock(delta=MagicMock(content="Shipment is in transit to Colombo.", tool_calls=None))
    ]

    async def stream2():
        yield text_delta

    mock_openai_client.chat.completions.create.side_effect = [stream1(), stream2()]

    with patch("src.llm.openai_adapter.execute_tool") as mock_exec:
        mock_exec.return_value = MagicMock(
            status="SUCCESS",
            result={"found": True, "status": "IN_TRANSIT", "action": {"action_type": "NAVIGATE_BOOKING", "label": "Book", "target_url": "/portal/book"}},
            duration_ms=25,
            error_message=None,
        )

        events = []
        async for chunk in adapter.generate_reply_stream(
            prompt="Track it",
            history=[],
            conversation_id=conv_id,
        ):
            events.append(chunk)

    event_types = [e.event for e in events]
    assert StreamEventType.TOOL_START in event_types
    assert StreamEventType.TOOL_END in event_types
    assert StreamEventType.ACTION_CARD in event_types
    assert StreamEventType.TOKEN in event_types
    assert StreamEventType.DONE in event_types


@pytest.mark.asyncio
async def test_openai_adapter_stream_rate_limit_emits_error_chunk():
    """Verify OpenAIAdapter yields ERROR chunk and raises RateLimitException on 429."""
    mock_openai_client = AsyncMock()
    adapter = OpenAIAdapter(client=mock_openai_client)
    conv_id = uuid.uuid4()

    mock_response = MagicMock(status_code=429, headers={})
    mock_openai_client.chat.completions.create.side_effect = RateLimitError(
        message="Rate limit exceeded",
        response=mock_response,
        body=None,
    )

    events = []
    with pytest.raises(RateLimitException):
        async for chunk in adapter.generate_reply_stream(
            prompt="Hello",
            history=[],
            conversation_id=conv_id,
        ):
            events.append(chunk)

    assert any(e.event == StreamEventType.ERROR for e in events)


@pytest.mark.asyncio
async def test_openai_adapter_stream_upstream_service_exception():
    """Verify OpenAIAdapter yields ERROR chunk on upstream connection failure."""
    mock_openai_client = AsyncMock()
    adapter = OpenAIAdapter(client=mock_openai_client)
    conv_id = uuid.uuid4()

    mock_openai_client.chat.completions.create.side_effect = APITimeoutError(request=MagicMock())

    events = []
    with pytest.raises(UpstreamServiceException):
        async for chunk in adapter.generate_reply_stream(
            prompt="Hello",
            history=[],
            conversation_id=conv_id,
        ):
            events.append(chunk)

    assert any(e.event == StreamEventType.ERROR for e in events)
