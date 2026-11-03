"""Unit tests for ChatService application service layer."""

import uuid
import pytest
from unittest.mock import AsyncMock

from src.core.cache import SessionCache
from src.core.security import AuthenticatedUser
from src.domain.chat import (
    ChatMessage,
    ChatMessageRequest,
    ChatMessageResponse,
    MessageRole,
    StreamChunkEvent,
    StreamEventType,
)
from src.llm.base import BaseLLMClient
from src.services.chat_service import ChatService, get_chat_service


class DummyLLMClient(BaseLLMClient):
    """Mock LLM client for testing ChatService."""

    def __init__(self, reply: str = "Test reply"):
        self.reply = reply
        self.last_prompt = None
        self.last_history = None
        self.last_context = None

    async def generate_reply(
        self,
        prompt: str,
        history,
        conversation_id: str,
        context=None,
    ) -> ChatMessageResponse:
        self.last_prompt = prompt
        self.last_history = history
        self.last_context = context
        return ChatMessageResponse(
            conversation_id=conversation_id,
            reply=self.reply,
            tool_calls_executed=[],
            action_cards=[],
            suggestions=[],
            latency_ms=12,
        )

    async def generate_reply_stream(
        self,
        prompt: str,
        history,
        conversation_id: str,
        context=None,
    ):
        yield StreamChunkEvent(
            event=StreamEventType.TOKEN,
            data={"delta": self.reply},
        )


@pytest.mark.asyncio
async def test_chat_service_default_instantiation():
    """Verify ChatService lazily initializes default cache and LLM client."""
    service = ChatService()
    assert service.cache is not None
    assert service.llm_client is not None
    singleton = get_chat_service()
    assert singleton is not None


@pytest.mark.asyncio
async def test_chat_service_process_chat_with_explicit_history():
    """Verify explicit history in request is respected over cache."""
    mock_cache = AsyncMock(spec=SessionCache)
    mock_llm = DummyLLMClient(reply="Hello from SwiftDrop!")
    service = ChatService(cache=mock_cache, llm_client=mock_llm)
    conv_id = uuid.uuid4()

    request = ChatMessageRequest(
        prompt="Where is my order?",
        conversation_id=conv_id,
        history=[
            ChatMessage(role=MessageRole.USER, content="Hello"),
            ChatMessage(role=MessageRole.ASSISTANT, content="Hi! How can I help?"),
        ],
    )

    response = await service.process_chat(request)
    assert response.reply == "Hello from SwiftDrop!"
    # Ensure cache.get_history was NOT called because explicit history was passed
    mock_cache.get_history.assert_not_called()
    # Ensure append_turns was called with user turn and assistant reply
    mock_cache.append_turns.assert_called_once()
    call_kwargs = mock_cache.append_turns.call_args[1]
    assert call_kwargs["conversation_id"] == conv_id
    assert call_kwargs["user_turn"].content == "Where is my order?"
    assert call_kwargs["assistant_turn"].content == "Hello from SwiftDrop!"


@pytest.mark.asyncio
async def test_chat_service_process_chat_with_cache_history_and_context():
    """Verify history is fetched from cache when missing, and context is correctly populated."""
    mock_cache = AsyncMock(spec=SessionCache)
    mock_cache.get_history.return_value = [
        ChatMessage(role=MessageRole.USER, content="Prior question"),
    ]
    mock_llm = DummyLLMClient(reply="I am an AI assistant trained by OpenAI.")
    service = ChatService(cache=mock_cache, llm_client=mock_llm)

    user = AuthenticatedUser(
        user_id="usr_test_99",
        role="MERCHANT",
        email="merchant@test.com",
        tenant_id="ten_test_99",
    )
    conv_id = uuid.uuid4()

    request = ChatMessageRequest(
        prompt="Show my shipments",
        conversation_id=conv_id,
    )

    response = await service.process_chat(
        request=request,
        user=user,
        auth_token="jwt-token-xyz",
    )

    # 1. Verify history was fetched from cache
    mock_cache.get_history.assert_called_once_with(conv_id)

    # 2. Verify context forwarded to LLM
    assert mock_llm.last_context["auth_token"] == "jwt-token-xyz"
    assert mock_llm.last_context["authorization"] == "Bearer jwt-token-xyz"
    assert mock_llm.last_context["user_id"] == "usr_test_99"
    assert mock_llm.last_context["role"] == "MERCHANT"
    assert mock_llm.last_context["email"] == "merchant@test.com"
    assert mock_llm.last_context["tenant_id"] == "ten_test_99"

    # 3. Verify identity sanitization
    assert "OpenAI" not in response.reply
    assert "SwiftDrop AI" in response.reply


@pytest.mark.asyncio
async def test_chat_service_truncation_of_very_large_reply():
    """Verify that extraordinarily long assistant replies are bounded when persisted to cache."""
    mock_cache = AsyncMock(spec=SessionCache)
    huge_reply = "A" * 12000
    mock_llm = DummyLLMClient(reply=huge_reply)
    service = ChatService(cache=mock_cache, llm_client=mock_llm)
    conv_id = uuid.uuid4()

    request = ChatMessageRequest(
        prompt="Tell me everything",
        conversation_id=conv_id,
    )

    response = await service.process_chat(request)
    assert len(response.reply) == 12000

    call_kwargs = mock_cache.append_turns.call_args[1]
    cached_assistant_turn = call_kwargs["assistant_turn"]
    assert len(cached_assistant_turn.content) == 9000
