"""Conversational assistant router providing grounded responses, action cards, and session continuity."""

from typing import Optional
from fastapi import APIRouter, Depends, Request, Response, status
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
