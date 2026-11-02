"""Concurrency and async non-blocking load tests for the transport layer."""

import asyncio
import uuid
import httpx
import pytest

from src.core.cache import get_session_cache
from src.main import app


@pytest.mark.asyncio
async def test_concurrent_async_chat_requests():
    """Verify that multiple concurrent async chat requests complete successfully without race conditions."""
    async with httpx.AsyncClient(
        transport=httpx.ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        # Launch 10 simultaneous chat requests with unique session IDs
        prompts = [
            f"Where is parcel SD-20261102-0000{i}?" for i in range(1, 6)
        ] + [
            f"How much to ship a {i}kg parcel from Colombo to Kandy?" for i in range(1, 6)
        ]

        tasks = [
            client.post(
                "/api/v1/ai/chat",
                json={
                    "conversation_id": str(uuid.uuid4()),
                    "prompt": prompt,
                },
            )
            for prompt in prompts
        ]

        responses = await asyncio.gather(*tasks)

        for res in responses:
            assert res.status_code == 200
            data = res.json()
            assert "reply" in data
            assert data["grounded"] is True


@pytest.mark.asyncio
async def test_concurrent_session_cache_thread_safety():
    """Verify thread-safe concurrent reads and writes to SessionCache without KeyErrors."""
    cache = get_session_cache()
    num_sessions = 20

    async def worker(idx: int):
        conv_id = f"concurrent-conv-{idx}"
        from src.domain.chat import ChatMessage, MessageRole

        user_m = ChatMessage(role=MessageRole.USER, content=f"Hello turn {idx}")
        asst_m = ChatMessage(role=MessageRole.ASSISTANT, content=f"Reply turn {idx}")

        await cache.append_turns(conv_id, user_m, asst_m)
        history = await cache.get_history(conv_id)
        assert len(history) == 2
        assert history[0].content == f"Hello turn {idx}"

    tasks = [worker(i) for i in range(num_sessions)]
    await asyncio.gather(*tasks)

    stats = await cache.get_stats()
    assert stats["active_sessions"] >= num_sessions


@pytest.mark.asyncio
async def test_health_probe_latency_under_load():
    """Verify that liveness probe responds immediately even while chat queries are being processed."""
    async with httpx.AsyncClient(
        transport=httpx.ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        # Send chat request and health check concurrently
        chat_task = client.post(
            "/api/v1/ai/chat",
            json={"prompt": "How much for a 3kg parcel from Colombo to Galle?"},
        )
        health_task = client.get("/health/live")

        chat_res, health_res = await asyncio.gather(chat_task, health_task)

        assert chat_res.status_code == 200
        assert health_res.status_code == 200
        assert health_res.json()["status"] == "UP"
