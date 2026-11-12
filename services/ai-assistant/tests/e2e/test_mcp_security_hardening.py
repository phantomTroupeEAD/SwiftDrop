"""End-to-end security hardening and adversarial tests for FastMCP server."""

import inspect
from typing import Any, Dict
import httpx
from pydantic import SecretStr
import pytest

from src.core.config import get_settings
from src.mcp.server import (
    estimate_shipping_fee,
    explain_company_policies,
    get_my_active_parcels,
    lookup_postal_code,
    mcp,
    track_parcel,
)
from src.tools.registry import execute_tool


@pytest.mark.asyncio
async def test_mcp_get_my_active_parcels_with_env_token(monkeypatch):
    """Verify that get_my_active_parcels succeeds without authorization_token arg when SWIFTDROP_AUTH_TOKEN is set."""
    settings = get_settings()
    monkeypatch.setattr(settings, "SWIFTDROP_AUTH_TOKEN", SecretStr("test-env-jwt-token-xyz"))

    # Execute without passing authorization_token
    res = await get_my_active_parcels(authorization_token=None, limit=5)
    assert isinstance(res, dict)
    assert "error" not in res
    assert "parcels" in res
    assert isinstance(res["parcels"], list)


@pytest.mark.asyncio
async def test_mcp_get_my_active_parcels_transcript_scrubbing(monkeypatch):
    """Verify that using SWIFTDROP_AUTH_TOKEN does not leak the secret token into tool arguments."""
    settings = get_settings()
    secret_token = "secret-super-private-token-999"
    monkeypatch.setattr(settings, "SWIFTDROP_AUTH_TOKEN", SecretStr(secret_token))

    # Execute via tool registry without passing secret in arguments
    record = await execute_tool("list_active_parcels", {"limit": 5})
    assert record.status == "SUCCESS"

    # Assert secret token is NOT leaked into recorded arguments
    arg_str = str(record.arguments)
    assert secret_token not in arg_str
    assert "authorization_token" not in record.arguments


@pytest.mark.asyncio
async def test_mcp_estimate_shipping_fee_partial_dimensions_returns_clean_error():
    """Verify that providing partial dimensions returns a structured error rather than crashing."""
    res = await estimate_shipping_fee(
        weight_kg=2.5,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
        length_cm=20.0,
        # width_cm and height_cm omitted
    )
    assert "error" in res
    assert "All three dimensions (length_cm, width_cm, height_cm) must be provided together" in res["error"]


@pytest.mark.asyncio
async def test_mcp_estimate_shipping_fee_full_dimensions_succeeds():
    """Verify that providing all three dimensions succeeds and includes dimensions."""
    res = await estimate_shipping_fee(
        weight_kg=2.5,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
        length_cm=20.0,
        width_cm=15.0,
        height_cm=10.0,
    )
    assert res.get("grounded") is True
    assert "estimated_fee" in res
    assert "action" in res


def test_mcp_tool_signature_authorization_token_is_optional():
    """Verify that get_my_active_parcels has authorization_token typed with default None."""
    sig = inspect.signature(get_my_active_parcels)
    param = sig.parameters.get("authorization_token")
    assert param is not None
    assert param.default is None, "authorization_token should default to None"
