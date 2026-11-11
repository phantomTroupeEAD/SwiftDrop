"""Concurrency and cache stampede (thundering herd) tests for policy_tool."""

import asyncio
from typing import Any, Dict
import httpx
from pydantic import SecretStr
import pytest

from src.core.config import get_settings
from src.domain.tools import CompanyPolicyInput
from src.tools.policy_tool import _POLICY_CACHE, policy_tool, reset_policy_cache


class StampedeTrackingTransport(httpx.AsyncBaseTransport):
    """Transport that counts calls and introduces realistic async latency."""

    def __init__(self, status_code: int = 200, json_body: Dict[str, Any] = None):
        self.call_count = 0
        self.status_code = status_code
        self.json_body = json_body or {
            "policies": {
                "prohibited_items": ["explosives", "radioactive material", "corrosive acids"],
                "max_weight_kg": 50.0,
            }
        }

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        self.call_count += 1
        # Realistic network delay to ensure other coroutines contend for lock
        await asyncio.sleep(0.05)
        import json
        return httpx.Response(
            status_code=self.status_code,
            content=json.dumps(self.json_body).encode("utf-8"),
            headers={"Content-Type": "application/json"},
            request=request,
        )


@pytest.mark.asyncio
async def test_policy_cache_stampede_single_upstream_call(monkeypatch):
    """Ensure 50 simultaneous requests result in exactly ONE upstream HTTP call."""
    reset_policy_cache()
    transport = StampedeTrackingTransport()

    # Configure service token so dynamic fetch is triggered
    settings = get_settings()
    monkeypatch.setattr(settings, "AI_SERVICE_ROLE_TOKEN", SecretStr("mock_service_token"))

    async with httpx.AsyncClient(transport=transport) as shared_client:
        # Launch 50 concurrent requests simultaneously
        tasks = [
            policy_tool(
                CompanyPolicyInput(query="prohibited items"),
                client=shared_client,
            )
            for _ in range(50)
        ]
        results = await asyncio.gather(*tasks)

    # 1. Assert all 50 completed successfully
    assert len(results) == 50
    for r in results:
        assert r["grounded"] is True
        assert "prohibited" in r["policy_content"].lower() or "explosives" in r["policy_content"].lower()

    # 2. Critical invariant: Upstream Core API was called EXACTLY ONCE
    assert transport.call_count == 1, f"Expected 1 upstream call, got {transport.call_count}"

    # 3. Subsequent calls reuse cache immediately without calling transport
    async with httpx.AsyncClient(transport=transport) as another_client:
        res_cached = await policy_tool(
            CompanyPolicyInput(query="prohibited"),
            client=another_client,
        )
        assert res_cached["source"] == "dynamic_admin_settings_cached"
        assert transport.call_count == 1


@pytest.mark.asyncio
async def test_policy_cache_upstream_failure_releases_lock():
    """Ensure upstream HTTP 500 error releases the lock and falls back to static markdown without deadlock."""
    reset_policy_cache()
    error_transport = StampedeTrackingTransport(status_code=500, json_body={"error": "Database down"})

    async with httpx.AsyncClient(transport=error_transport) as error_client:
        # 10 concurrent requests during upstream outage
        tasks = [
            policy_tool(
                CompanyPolicyInput(query="prohibited"),
                client=error_client,
            )
            for _ in range(10)
        ]
        results = await asyncio.gather(*tasks)

    assert len(results) == 10
    for r in results:
        assert r["grounded"] is True
        # Fallen back to static markdown knowledge base
        assert r["source"] == "static_knowledge_base"
        assert "prohibited" in r["policy_content"].lower()
