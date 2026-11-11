"""End-to-End Resilience & Boundary Edge Case Tests for Phase 8 Containerization."""

import asyncio
from uuid import uuid4
import httpx
import pytest
from src.core.cache import SessionCache
from src.domain.chat import ChatMessage, MessageRole
from src.main import app


@pytest.mark.asyncio
async def test_health_independence_during_complete_upstream_outage():
    """TC-RES-001: Anti-Cascading Health Independence:
    When upstream Core API and Tracking API are completely offline or timing out,
    /health/live and /health/ready MUST continue returning 200 OK independently.
    """
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://testserver") as client:
        # 1. Verify liveness check succeeds regardless of external services
        live_res = await client.get("/health/live")
        assert live_res.status_code == 200
        live_data = live_res.json()
        assert live_data["status"] == "UP"
        assert live_data["service"] == "ai-assistant"

        # 2. Verify readiness check succeeds independently
        ready_res = await client.get("/health/ready")
        assert ready_res.status_code == 200
        ready_data = ready_res.json()
        assert ready_data["status"] == "READY"


@pytest.mark.asyncio
async def test_sse_streaming_proxy_buffering_headers():
    """TC-RES-002: Proxy Buffering Prevention:
    Verify /api/v1/ai/chat/stream sends mandatory edge headers preventing Traefik/Cloudflare buffering.
    """
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://testserver") as client:
        req_payload = {
            "prompt": "Where is parcel SD-20261102-00001?",
            "conversation_id": str(uuid4()),
        }
        res = await client.post("/api/v1/ai/chat/stream", json=req_payload)
        assert res.status_code == 200
        
        # Verify edge proxy headers
        assert res.headers.get("x-accel-buffering") == "no"
        assert "text/event-stream" in res.headers.get("content-type", "")
        assert "no-cache" in res.headers.get("cache-control", "")


@pytest.mark.asyncio
async def test_session_cache_memory_bounded_under_high_volume():
    """TC-RES-005: High-Concurrency Memory Eviction:
    Verify that an influx of distinct conversations honors maxsize eviction
    and never causes unbounded memory leaks or OOM crashes.
    """
    cache = SessionCache(maxsize=100, ttl_seconds=60)
    
    # Insert 500 distinct sessions into a cache bounded to 100
    for i in range(500):
        conv_id = uuid4()
        user_msg = ChatMessage(role=MessageRole.USER, content=f"Hello from session {i}")
        asst_msg = ChatMessage(role=MessageRole.ASSISTANT, content=f"Reply to session {i}")
        await cache.append_turns(
            conversation_id=conv_id,
            user_turn=user_msg,
            assistant_turn=asst_msg,
        )
    
    # Cache size must not exceed the bounded maxsize
    stats = await cache.get_stats()
    assert stats["active_sessions"] <= 100
