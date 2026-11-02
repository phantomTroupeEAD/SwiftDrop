"""Conversational assistant router providing grounded responses, action cards, and session continuity."""

import time
from typing import Optional
from fastapi import APIRouter, Depends, Request, Response, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

from src.api.middleware.rate_limit import get_dynamic_rate_limit, limiter
from src.core.cache import get_session_cache
from src.domain.chat import (
    ChatMessage,
    ChatMessageRequest,
    ChatMessageResponse,
    MessageRole,
)
from src.llm import get_llm_client
from src.llm.guardrails import sanitize_assistant_identity

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
@limiter.limit(get_dynamic_rate_limit)
async def chat_endpoint(
    request: Request,
    response: Response,
    body: ChatMessageRequest,
    auth: Optional[HTTPAuthorizationCredentials] = Depends(bearer_scheme),
) -> ChatMessageResponse:
    """Execute conversational turn with session continuity and tool grounding."""
    cache = get_session_cache()

    # 1. Resolve multi-turn conversational history:
    # If caller provided explicit history in the request, prioritize it; otherwise fetch from session cache.
    if body.history:
        history = list(body.history)
    else:
        history = await cache.get_history(body.conversation_id)

    # 2. Build execution context (e.g. forward Bearer token if present for active parcel queries)
    context = {}
    if auth and auth.credentials:
        context["auth_token"] = auth.credentials
        context["authorization"] = f"Bearer {auth.credentials}"

    # Forward verified user identity if authenticated
    user = getattr(request.state, "user", None)
    if user:
        context["user_id"] = user.user_id
        context["role"] = user.role
        context["email"] = user.email
        if user.tenant_id:
            context["tenant_id"] = user.tenant_id

    # 3. Dispatch to configured LLM engine (OpenAIAdapter or MockLLMAdapter)
    client = get_llm_client()
    response = await client.generate_reply(
        prompt=body.prompt,
        history=history,
        conversation_id=body.conversation_id,
        context=context,
    )

    # 3b. Guarantee SwiftDrop AI identity and scrub any third-party model leaks
    response.reply = sanitize_assistant_identity(response.reply)

    # 4. Atomically persist user turn and assistant reply to in-memory session cache
    user_turn = ChatMessage(role=MessageRole.USER, content=body.prompt)
    # Safe bound for cached turn
    assistant_content = response.reply[:9000] if len(response.reply) > 9000 else response.reply
    assistant_turn = ChatMessage(role=MessageRole.ASSISTANT, content=assistant_content)
    await cache.append_turns(
        conversation_id=body.conversation_id,
        user_turn=user_turn,
        assistant_turn=assistant_turn,
    )

    return response
