"""Abstract base class interface for conversational LLM client engines."""

from abc import ABC, abstractmethod
from typing import Any, AsyncGenerator, Dict, List, Optional
from uuid import UUID

from src.domain.chat import ChatMessage, ChatMessageResponse, StreamChunkEvent


class BaseLLMClient(ABC):
    """Abstract interface for LLM conversational resolution engines."""

    @abstractmethod
    async def generate_reply(
        self,
        prompt: str,
        history: List[ChatMessage],
        conversation_id: UUID,
        context: Optional[Dict[str, Any]] = None,
    ) -> ChatMessageResponse:
        """Process conversational turn, execute required read-only tools, and return a grounded response."""
        pass

    @abstractmethod
    async def generate_reply_stream(
        self,
        prompt: str,
        history: List[ChatMessage],
        conversation_id: UUID,
        context: Optional[Dict[str, Any]] = None,
    ) -> AsyncGenerator[StreamChunkEvent, None]:
        """Process conversational turn and yield incremental stream chunk events."""
        pass
