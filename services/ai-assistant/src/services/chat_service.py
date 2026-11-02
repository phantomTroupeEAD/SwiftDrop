"""Application service orchestrating conversational assistant workflows.

Separates business workflow (session retrieval, context assembly, LLM dispatch,
identity guardrails, and turn persistence) from HTTP transport controllers.
"""

from typing import Dict, List, Optional

from src.core.cache import SessionCache, get_session_cache
from src.core.security import AuthenticatedUser
from src.domain.chat import (
    ChatMessage,
    ChatMessageRequest,
    ChatMessageResponse,
    MessageRole,
)
from src.llm import get_llm_client
from src.llm.base import BaseLLMClient
from src.llm.guardrails import sanitize_assistant_identity


class ChatService:
    """Orchestrates end-to-end chat conversation turns across cache and LLM adapters."""

    def __init__(
        self,
        cache: Optional[SessionCache] = None,
        llm_client: Optional[BaseLLMClient] = None,
    ) -> None:
        self._cache = cache
        self._llm_client = llm_client

    @property
    def cache(self) -> SessionCache:
        if self._cache is None:
            return get_session_cache()
        return self._cache

    @property
    def llm_client(self) -> BaseLLMClient:
        if self._llm_client is None:
            return get_llm_client()
        return self._llm_client

    async def process_chat(
        self,
        request: ChatMessageRequest,
        user: Optional[AuthenticatedUser] = None,
        auth_token: Optional[str] = None,
    ) -> ChatMessageResponse:
        """Execute a conversational turn with session continuity and tool grounding."""
        # 1. Resolve multi-turn history:
        # Caller-supplied history takes precedence over in-memory session cache
        if request.history:
            history: List[ChatMessage] = list(request.history)
        else:
            history = await self.cache.get_history(request.conversation_id)

        # 2. Build execution context (forward credentials & user identity to tools)
        context: Dict[str, str] = {}
        if auth_token:
            context["auth_token"] = auth_token
            context["authorization"] = f"Bearer {auth_token}"

        if user:
            context["user_id"] = user.user_id
            context["role"] = user.role
            context["email"] = user.email
            if user.tenant_id:
                context["tenant_id"] = user.tenant_id

        # 3. Dispatch prompt and context to configured LLM client
        response: ChatMessageResponse = await self.llm_client.generate_reply(
            prompt=request.prompt,
            history=history,
            conversation_id=request.conversation_id,
            context=context,
        )

        # 4. Enforce strict SwiftDrop AI identity and scrub model leaks
        response.reply = sanitize_assistant_identity(response.reply)

        # 5. Atomically persist user turn and assistant reply into session cache
        user_turn = ChatMessage(role=MessageRole.USER, content=request.prompt)
        assistant_content = (
            response.reply[:9000] if len(response.reply) > 9000 else response.reply
        )
        assistant_turn = ChatMessage(role=MessageRole.ASSISTANT, content=assistant_content)

        await self.cache.append_turns(
            conversation_id=request.conversation_id,
            user_turn=user_turn,
            assistant_turn=assistant_turn,
        )

        return response


# Singleton / dependency provider for FastAPI injection
_default_chat_service: Optional[ChatService] = None


def get_chat_service() -> ChatService:
    """Return the global ChatService instance for dependency injection."""
    global _default_chat_service
    if _default_chat_service is None:
        _default_chat_service = ChatService()
    return _default_chat_service
