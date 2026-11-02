"""Conversational assistant router providing grounded responses, action cards, and session continuity."""

import time
from typing import Optional
from fastapi import APIRouter, Depends, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

from src.core.cache import get_session_cache
from src.domain.chat import (
    ChatMessage,
    ChatMessageRequest,
    ChatMessageResponse,
    MessageRole,
)
from src.llm import get_llm_client

router = APIRouter()

# Optional Bearer token extractor (auto_error=False permits public unauthenticated access)
bearer_scheme = HTTPBearer(auto_error=False)


@router.post(
    "/chat",
    summary="Process Conversational Chat Turn",
    description=(
        "Processes a natural language inquiry from customers or merchants. "
        "Grounds answers strictly using read-only domain tools (tracking, quotes, policies) "
        "and generates 1-click booking navigation cards where appropriate."
    ),
    response_model=ChatMessageResponse,
    status_code=status.HTTP_200_OK,
)
async def chat_endpoint(
    request: ChatMessageRequest,
    auth: Optional[HTTPAuthorizationCredentials] = Depends(bearer_scheme),
) -> ChatMessageResponse:
    """Execute conversational turn with session continuity and tool grounding."""
    cache = get_session_cache()

    # 1. Resolve multi-turn conversational history:
    # If caller provided explicit history in the request, prioritize it; otherwise fetch from session cache.
    if request.history:
        history = list(request.history)
    else:
        history = await cache.get_history(request.conversation_id)

    # 2. Build execution context (e.g. forward Bearer token if present for active parcel queries)
    context = {}
    if auth and auth.credentials:
        context["auth_token"] = auth.credentials

    # 3. Dispatch to configured LLM engine (OpenAIAdapter or MockLLMAdapter)
    client = get_llm_client()
    response = await client.generate_reply(
        prompt=request.prompt,
        history=history,
        conversation_id=request.conversation_id,
        context=context,
    )

    # 4. Atomically persist user turn and assistant reply to in-memory session cache
    user_turn = ChatMessage(role=MessageRole.USER, content=request.prompt)
    # Safe bound for cached turn
    assistant_content = response.reply[:9000] if len(response.reply) > 9000 else response.reply
    assistant_turn = ChatMessage(role=MessageRole.ASSISTANT, content=assistant_content)
    await cache.append_turns(
        conversation_id=request.conversation_id,
        user_turn=user_turn,
        assistant_turn=assistant_turn,
    )

    return response
