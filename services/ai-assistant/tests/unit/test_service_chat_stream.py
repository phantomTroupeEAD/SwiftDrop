"""Unit tests for ChatService streaming orchestration and session persistence."""

from unittest.mock import AsyncMock, MagicMock
import uuid
import pytest

from src.core.cache import SessionCache
from src.core.security import AuthenticatedUser
from src.domain.chat import (
    ChatMessage,
    ChatMessageRequest,
    MessageRole,
    StreamChunkEvent,
    StreamEventType,
)
from src.llm.base import BaseLLMClient
from src.services.chat_service import ChatService


class StreamingMockLLMClient(BaseLLMClient):
    """Custom mock LLM client for testing ChatService.stream_chat."""

    def __init__(self, chunks=None, error=None):
        self.chunks = chunks or []
        self.error = error
        self.last_context = None

    async def generate_reply(self, prompt, history, conversation_id, context=None):
        raise NotImplementedError

    async def generate_reply_stream(self, prompt, history, conversation_id, context=None):
        self.last_context = context
        if self.error:
            raise self.error
        for chunk in self.chunks:
            yield chunk


@pytest.mark.asyncio
async def test_chat_service_stream_chat_accumulates_and_persists_to_cache():
    """Verify stream_chat yields SSE frames and persists aggregated assistant turn to cache."""
    mock_cache = AsyncMock(spec=SessionCache)
    conv_id = uuid.uuid4()

    chunks = [
        StreamChunkEvent(event=StreamEventType.TOKEN, data={"delta": "Hello "}),
        StreamChunkEvent(event=StreamEventType.TOKEN, data={"delta": "from "}),
        StreamChunkEvent(event=StreamEventType.TOKEN, data={"delta": "SwiftDrop!"}),
        StreamChunkEvent(event=StreamEventType.DONE, data={"conversation_id": str(conv_id), "latency_ms": 50}),
    ]
    mock_llm = StreamingMockLLMClient(chunks=chunks)
    service = ChatService(cache=mock_cache, llm_client=mock_llm)

    request = ChatMessageRequest(prompt="Greetings", conversation_id=conv_id)
    frames = []

    async for frame in service.stream_chat(request):
        frames.append(frame)

    assert len(frames) == 4
    assert frames[0].startswith("event: token\ndata:")
    assert frames[-1].startswith("event: done\ndata:")

    # Verify session cache persistence was called with complete aggregated message
    mock_cache.append_turns.assert_called_once()
    call_kwargs = mock_cache.append_turns.call_args[1]
    assert call_kwargs["conversation_id"] == conv_id
    assert call_kwargs["user_turn"].content == "Greetings"
    assert call_kwargs["assistant_turn"].content == "Hello from SwiftDrop!"


@pytest.mark.asyncio
async def test_chat_service_stream_chat_client_disconnect_terminates_early():
    """Verify stream_chat aborts when client disconnects mid-stream (EC-1)."""
    mock_cache = AsyncMock(spec=SessionCache)
    conv_id = uuid.uuid4()

    chunks = [
        StreamChunkEvent(event=StreamEventType.TOKEN, data={"delta": "Chunk 1"}),
        StreamChunkEvent(event=StreamEventType.TOKEN, data={"delta": "Chunk 2"}),
        StreamChunkEvent(event=StreamEventType.TOKEN, data={"delta": "Chunk 3"}),
    ]
    mock_llm = StreamingMockLLMClient(chunks=chunks)
    service = ChatService(cache=mock_cache, llm_client=mock_llm)

    # Disconnect after the first chunk
    call_count = 0
    async def is_disconnected_mock():
        nonlocal call_count
        call_count += 1
        return call_count > 1

    request = ChatMessageRequest(prompt="Test disconnect", conversation_id=conv_id)
    frames = []

    async for frame in service.stream_chat(request, is_disconnected=is_disconnected_mock):
        frames.append(frame)

    # Should only emit the first chunk before disconnecting
    assert len(frames) == 1
    assert "Chunk 1" in frames[0]


@pytest.mark.asyncio
async def test_chat_service_stream_chat_sanitizes_model_identity():
    """Verify streaming token deltas sanitize model identity leaks."""
    mock_cache = AsyncMock(spec=SessionCache)
    conv_id = uuid.uuid4()

    chunks = [
        StreamChunkEvent(
            event=StreamEventType.TOKEN,
            data={"delta": "I am an AI assistant trained by OpenAI."},
        ),
    ]
    mock_llm = StreamingMockLLMClient(chunks=chunks)
    service = ChatService(cache=mock_cache, llm_client=mock_llm)

    request = ChatMessageRequest(prompt="Who are you?", conversation_id=conv_id)
    frames = []

    async for frame in service.stream_chat(request):
        frames.append(frame)

    assert len(frames) == 1
    assert "OpenAI" not in frames[0]
    assert "SwiftDrop AI" in frames[0]


@pytest.mark.asyncio
async def test_chat_service_stream_chat_with_auth_context():
    """Verify stream_chat forwards authenticated user identity and credentials to context."""
    mock_cache = AsyncMock(spec=SessionCache)
    conv_id = uuid.uuid4()

    chunks = [
        StreamChunkEvent(event=StreamEventType.TOKEN, data={"delta": "Authorized output."}),
    ]
    mock_llm = StreamingMockLLMClient(chunks=chunks)
    service = ChatService(cache=mock_cache, llm_client=mock_llm)

    user = AuthenticatedUser(
        user_id="usr_stream_99",
        role="MERCHANT",
        email="merchant@stream.local",
        tenant_id="ten_stream_99",
    )
    request = ChatMessageRequest(prompt="My orders", conversation_id=conv_id)

    frames = [f async for f in service.stream_chat(request, user=user, auth_token="token_stream_xyz")]

    assert len(frames) == 1
    assert mock_llm.last_context["user_id"] == "usr_stream_99"
    assert mock_llm.last_context["tenant_id"] == "ten_stream_99"
    assert mock_llm.last_context["auth_token"] == "token_stream_xyz"


@pytest.mark.asyncio
async def test_chat_service_stream_chat_handles_generator_exception():
    """Verify unhandled stream generator exceptions emit structured error event frame."""
    mock_cache = AsyncMock(spec=SessionCache)
    conv_id = uuid.uuid4()

    mock_llm = StreamingMockLLMClient(error=RuntimeError("Stream broke!"))
    service = ChatService(cache=mock_cache, llm_client=mock_llm)

    request = ChatMessageRequest(prompt="Break it", conversation_id=conv_id)
    frames = [f async for f in service.stream_chat(request)]

    assert len(frames) == 1
    assert "event: error" in frames[0]
    assert "INTERNAL_STREAM_ERROR" in frames[0]
