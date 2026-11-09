"""Application service orchestrating conversational assistant workflows.

Separates business workflow (session retrieval, context assembly, LLM dispatch,
identity guardrails, and turn persistence) from HTTP transport controllers.
"""

from typing import Any, AsyncGenerator, Callable, Coroutine, Dict, List, Optional
from uuid import UUID

from src.core.cache import SessionCache, get_session_cache
from src.core.security import AuthenticatedUser
from src.domain.chat import (
    ChatMessage,
    ChatMessageRequest,
    ChatMessageResponse,
    MessageRole,
    StreamChunkEvent,
    StreamEventType,
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

    def _build_execution_context(
        self,
        user: Optional[AuthenticatedUser] = None,
        auth_token: Optional[str] = None,
    ) -> Dict[str, str]:
        """Construct unified context dictionary forwarding credentials & caller identity."""
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
        return context

    async def _persist_session_turn(
        self,
        conversation_id: UUID,
        prompt: str,
        reply: str,
    ) -> None:
        """Atomically persist user turn and truncated assistant reply into session cache."""
        user_turn = ChatMessage(role=MessageRole.USER, content=prompt)
        assistant_content = reply[:9000] if len(reply) > 9000 else reply
        assistant_turn = ChatMessage(role=MessageRole.ASSISTANT, content=assistant_content)

        await self.cache.append_turns(
            conversation_id=conversation_id,
            user_turn=user_turn,
            assistant_turn=assistant_turn,
        )

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

        # 2. Build execution context
        context = self._build_execution_context(user=user, auth_token=auth_token)

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
        await self._persist_session_turn(
            conversation_id=request.conversation_id,
            prompt=request.prompt,
            reply=response.reply,
        )

        return response

    async def stream_chat(
        self,
        request: ChatMessageRequest,
        user: Optional[AuthenticatedUser] = None,
        auth_token: Optional[str] = None,
        is_disconnected: Optional[Callable[[], Coroutine[Any, Any, bool]]] = None,
    ) -> AsyncGenerator[str, None]:
        """Execute a streaming turn, yielding formatted SSE wire frames and persisting turns on completion."""
        # 1. Resolve multi-turn history
        if request.history:
            history: List[ChatMessage] = list(request.history)
        else:
            history = await self.cache.get_history(request.conversation_id)

        # 2. Build execution context
        context = self._build_execution_context(user=user, auth_token=auth_token)

        accumulated_reply = ""

        try:
            generator = self.llm_client.generate_reply_stream(
                prompt=request.prompt,
                history=history,
                conversation_id=request.conversation_id,
                context=context,
            )

            async for chunk in generator:
                # EC-1: Check client disconnection mid-stream
                if is_disconnected and await is_disconnected():
                    break

                if chunk.event == StreamEventType.TOKEN:
                    delta = chunk.data.get("delta", "")
                    # SEC-009 / Model Guardrails sanitization
                    delta_clean = sanitize_assistant_identity(delta)
                    accumulated_reply += delta_clean
                    chunk.data["delta"] = delta_clean

                yield chunk.to_sse_frame()

        except Exception:
            err_frame = StreamChunkEvent(
                event=StreamEventType.ERROR,
                data={
                    "code": "INTERNAL_STREAM_ERROR",
                    "message": "An error occurred during stream generation. Please try again.",
                },
            ).to_sse_frame()
            yield err_frame
            return

        # 3. Atomically persist conversation turns into cache on successful stream
        if accumulated_reply:
            await self._persist_session_turn(
                conversation_id=request.conversation_id,
                prompt=request.prompt,
                reply=accumulated_reply,
            )


# Singleton / dependency provider for FastAPI injection
_default_chat_service: Optional[ChatService] = None


def get_chat_service() -> ChatService:
    """Return the global ChatService instance for dependency injection."""
    global _default_chat_service
    if _default_chat_service is None:
        _default_chat_service = ChatService()
    return _default_chat_service
