"""Conversational assistant router providing grounded responses, action cards, and session continuity."""

from typing import Optional
from fastapi import APIRouter, Depends, Request, Response, status
from fastapi.responses import StreamingResponse
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

from src.api.middleware.rate_limit import get_dynamic_rate_limit, limiter
from src.core.security import AuthenticatedUser
from src.domain.chat import (
    ChatMessageRequest,
    ChatMessageResponse,
)
from src.llm import get_llm_client
from src.services.chat_service import ChatService, get_chat_service

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
    chat_service: ChatService = Depends(get_chat_service),
) -> ChatMessageResponse:
    """Execute conversational turn with session continuity and tool grounding via ChatService."""
    auth_token = auth.credentials if auth and auth.credentials else None
    user: Optional[AuthenticatedUser] = getattr(request.state, "user", None)

    return await chat_service.process_chat(
        request=body,
        user=user,
        auth_token=auth_token,
    )


@router.post(
    "/chat/stream",
    summary="Process Conversational Chat Turn (SSE Stream)",
    description=(
        "Processes a natural language inquiry from customers or merchants with token-by-token "
        "Server-Sent Events (SSE) streaming. Emits tool_start, tool_end, token, action_card, "
        "and done events conforming to standard W3C EventSource wire format."
    ),
    response_class=StreamingResponse,
    status_code=status.HTTP_200_OK,
)
@limiter.limit(get_dynamic_rate_limit)
async def chat_stream_endpoint(
    request: Request,
    response: Response,
    body: ChatMessageRequest,
    auth: Optional[HTTPAuthorizationCredentials] = Depends(bearer_scheme),
    chat_service: ChatService = Depends(get_chat_service),
) -> StreamingResponse:
    """Execute conversational turn with real-time SSE token and tool event streaming."""
    auth_token = auth.credentials if auth and auth.credentials else None
    user: Optional[AuthenticatedUser] = getattr(request.state, "user", None)

    stream_generator = chat_service.stream_chat(
        request=body,
        user=user,
        auth_token=auth_token,
        is_disconnected=request.is_disconnected,
    )

    headers = {
        "Content-Type": "text/event-stream; charset=utf-8",
        "Cache-Control": "no-cache, no-transform",
        "Connection": "keep-alive",
        "X-Accel-Buffering": "no",
    }

    return StreamingResponse(
        stream_generator,
        media_type="text/event-stream",
        headers=headers,
    )
