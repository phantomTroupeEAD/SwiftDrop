"""Unit tests for SessionCache, TTL expiration, tenant isolation, and concurrency."""

import asyncio
from uuid import uuid4
from src.core.cache import SessionCache
from src.domain.chat import ChatMessage, MessageRole


def test_session_cache_basic_append_and_get():
    """Verify appending and retrieving conversational turns."""
    async def _test():
        cache = SessionCache(maxsize=10, ttl_seconds=60)
        conv_id = uuid4()

        u1 = ChatMessage(role=MessageRole.USER, content="Hello")
        a1 = ChatMessage(role=MessageRole.ASSISTANT, content="Hi there!")

        await cache.append_turns(conv_id, u1, a1)
        history = await cache.get_history(conv_id)

        assert len(history) == 2
        assert history[0].content == "Hello"
        assert history[1].content == "Hi there!"

    asyncio.run(_test())


def test_session_cache_history_capping():
    """Verify that internal cache caps turns at 10 messages."""
    async def _test():
        cache = SessionCache(maxsize=10, ttl_seconds=60)
        conv_id = uuid4()

        for i in range(8):
            u = ChatMessage(role=MessageRole.USER, content=f"Q{i}")
            a = ChatMessage(role=MessageRole.ASSISTANT, content=f"A{i}")
            await cache.append_turns(conv_id, u, a)

        # 8 pairs = 16 messages, should be capped at last 10
        history = await cache.get_history(conv_id)
        assert len(history) == 10
        assert history[-1].content == "A7"

    asyncio.run(_test())


def test_session_cache_tenant_isolation():
    """Verify tenant isolation: same conversation_id under different tenants are strictly isolated."""
    async def _test():
        cache = SessionCache(maxsize=10, ttl_seconds=60)
        conv_id = uuid4()

        u_a = ChatMessage(role=MessageRole.USER, content="Tenant A confidential message")
        a_a = ChatMessage(role=MessageRole.ASSISTANT, content="Tenant A reply")
        await cache.append_turns(conv_id, u_a, a_a, tenant_id="tenant_A")

        # Accessing as Tenant B should find nothing
        hist_b = await cache.get_history(conv_id, tenant_id="tenant_B")
        assert len(hist_b) == 0

        # Accessing as Tenant A returns the message
        hist_a = await cache.get_history(conv_id, tenant_id="tenant_A")
        assert len(hist_a) == 2
        assert hist_a[0].content == "Tenant A confidential message"

    asyncio.run(_test())


def test_session_cache_ttl_expiration():
    """Verify that sessions expire gracefully after TTL seconds."""
    async def _test():
        # 1 second TTL
        cache = SessionCache(maxsize=10, ttl_seconds=1)
        conv_id = uuid4()

        u = ChatMessage(role=MessageRole.USER, content="Test")
        a = ChatMessage(role=MessageRole.ASSISTANT, content="Test")
        await cache.append_turns(conv_id, u, a)

        # Immediate access succeeds
        assert len(await cache.get_history(conv_id)) == 2

        # Wait for expiration
        await asyncio.sleep(1.1)
        # Expired session returns empty list gracefully without throwing
        assert len(await cache.get_history(conv_id)) == 0

    asyncio.run(_test())


def test_session_cache_lru_eviction():
    """Verify that exceeding maxsize evicts the least recently used session."""
    async def _test():
        cache = SessionCache(maxsize=2, ttl_seconds=60)
        id1 = uuid4()
        id2 = uuid4()
        id3 = uuid4()

        u = ChatMessage(role=MessageRole.USER, content="Ping")
        a = ChatMessage(role=MessageRole.ASSISTANT, content="Pong")

        await cache.append_turns(id1, u, a)
        await cache.append_turns(id2, u, a)

        # Both id1 and id2 are in cache
        assert len(await cache.get_history(id1)) == 2
        assert len(await cache.get_history(id2)) == 2

        # Adding id3 must evict the least recently used (which is id1 since id2 was accessed last)
        await cache.append_turns(id3, u, a)

        assert len(await cache.get_history(id3)) == 2
        assert len(await cache.get_history(id2)) == 2
        # id1 should have been evicted
        assert len(await cache.get_history(id1)) == 0

    asyncio.run(_test())


def test_session_cache_concurrency_safety():
    """Verify concurrent reads and writes do not corrupt cache state."""
    async def _test():
        cache = SessionCache(maxsize=100, ttl_seconds=60)
        sessions = [uuid4() for _ in range(20)]

        async def worker(conv_id):
            for i in range(5):
                u = ChatMessage(role=MessageRole.USER, content=f"User {conv_id} {i}")
                a = ChatMessage(role=MessageRole.ASSISTANT, content=f"AI {conv_id} {i}")
                await cache.append_turns(conv_id, u, a)
                await cache.get_history(conv_id)

        # Run 20 concurrent session workers
        await asyncio.gather(*(worker(s) for s in sessions))

        stats = await cache.get_stats()
        assert stats["active_sessions"] == 20

    asyncio.run(_test())
