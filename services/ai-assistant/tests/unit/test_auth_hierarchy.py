"""Unit tests verifying the authentication resolution hierarchy for customer parcel retrieval."""

from typing import Any, Dict
import httpx
from pydantic import SecretStr
import pytest

from src.core.config import get_settings
from src.domain.errors import AuthException
from src.domain.tools import ActiveParcelsInput
from src.tools.parcels_tool import parcels_tool


class MockParcelsTransport(httpx.AsyncBaseTransport):
    """Transport that captures the Authorization header sent upstream."""

    def __init__(self):
        self.captured_auth_header: str = ""

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        self.captured_auth_header = request.headers.get("Authorization", "")
        import json
        return httpx.Response(
            status_code=200,
            content=json.dumps({"parcels": [], "count": 0}).encode("utf-8"),
            headers={"Content-Type": "application/json"},
            request=request,
        )


@pytest.mark.asyncio
async def test_auth_hierarchy_context_takes_highest_priority(monkeypatch):
    """Context authorization takes precedence over environment variable."""
    settings = get_settings()
    monkeypatch.setattr(settings, "SWIFTDROP_AUTH_TOKEN", SecretStr("env_token_123"))

    transport = MockParcelsTransport()
    async with httpx.AsyncClient(transport=transport) as client:
        context = {"authorization": "Bearer context_token_456"}
        await parcels_tool(ActiveParcelsInput(limit=5), context=context, client=client)

    assert transport.captured_auth_header == "Bearer context_token_456"


@pytest.mark.asyncio
async def test_auth_hierarchy_env_token_used_when_context_missing(monkeypatch):
    """When context has no authorization, SWIFTDROP_AUTH_TOKEN is used."""
    settings = get_settings()
    monkeypatch.setattr(settings, "SWIFTDROP_AUTH_TOKEN", SecretStr("Bearer env_token_789"))

    transport = MockParcelsTransport()
    async with httpx.AsyncClient(transport=transport) as client:
        await parcels_tool(ActiveParcelsInput(limit=5), context={}, client=client)

    assert transport.captured_auth_header == "Bearer env_token_789"


@pytest.mark.asyncio
async def test_auth_hierarchy_normalizes_raw_token_without_bearer(monkeypatch):
    """Raw token without 'Bearer ' prefix is cleanly normalized."""
    settings = get_settings()
    monkeypatch.setattr(settings, "SWIFTDROP_AUTH_TOKEN", SecretStr("raw_jwt_without_bearer"))

    transport = MockParcelsTransport()
    async with httpx.AsyncClient(transport=transport) as client:
        await parcels_tool(ActiveParcelsInput(limit=5), context={}, client=client)

    assert transport.captured_auth_header == "Bearer raw_jwt_without_bearer"


@pytest.mark.asyncio
async def test_auth_hierarchy_missing_all_tokens_raises_auth_exception(monkeypatch):
    """When no token is provided in context or env, a clean AuthException is raised."""
    settings = get_settings()
    monkeypatch.setattr(settings, "SWIFTDROP_AUTH_TOKEN", SecretStr(""))

    transport = MockParcelsTransport()
    async with httpx.AsyncClient(transport=transport) as client:
        with pytest.raises(AuthException) as exc_info:
            await parcels_tool(ActiveParcelsInput(limit=5), context={}, client=client)

    assert "Authentication token required" in str(exc_info.value)
    assert transport.captured_auth_header == ""


@pytest.mark.asyncio
async def test_auth_hierarchy_whitespace_tokens_rejected(monkeypatch):
    """Whitespace tokens in context and environment are rejected."""
    settings = get_settings()
    monkeypatch.setattr(settings, "SWIFTDROP_AUTH_TOKEN", SecretStr("   "))

    transport = MockParcelsTransport()
    async with httpx.AsyncClient(transport=transport) as client:
        with pytest.raises(AuthException):
            await parcels_tool(ActiveParcelsInput(limit=5), context={"authorization": "   "}, client=client)
