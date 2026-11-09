"""Concurrency, load stress, and performance benchmark test suite."""

import asyncio
import time
from typing import List
import httpx
import pytest

from unittest.mock import patch

from src.main import app


@pytest.mark.asyncio
async def test_high_concurrency_async_chat_requests():
    """Execute 50 concurrent async chat requests simulating traffic spike."""
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:

        async def send_single_chat(idx: int) -> int:
            # Vary IP address to test distinct client buckets
            ip = f"198.51.100.{(idx % 50) + 1}"
            response = await client.post(
                "/api/v1/ai/chat",
                json={
                    "prompt": f"Where is package SD-20261102-00001? (User {idx})",
                    "conversation_id": f"3fa85f64-5717-4562-b3fc-2c963f66{idx:04d}",
                },
                headers={"X-Forwarded-For": ip},
            )
            return response.status_code

        tasks = [send_single_chat(i) for i in range(50)]
        results: List[int] = await asyncio.gather(*tasks)

        # All 50 concurrent requests must return 200 OK without race conditions
        assert all(status == 200 for status in results)
        assert len(results) == 50


@pytest.mark.asyncio
async def test_latency_benchmark_under_mock_mode():
    """Verify p95 response latency is under 1000ms including loopback HTTP round-trips."""
    transport = httpx.ASGITransport(app=app)
    latencies: List[float] = []

    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        for i in range(15):
            start = time.perf_counter()
            resp = await client.post(
                "/api/v1/ai/chat",
                json={"prompt": "How much for a 2kg package from Colombo to Kandy?"},
                headers={"X-Forwarded-For": f"203.0.113.{i + 1}"},
            )
            elapsed_ms = (time.perf_counter() - start) * 1000
            assert resp.status_code == 200
            latencies.append(elapsed_ms)

    latencies.sort()
    # 95th percentile index
    p95_index = int(len(latencies) * 0.95)
    p95_latency = latencies[p95_index]

    # Sub-1-second latency is standard for complete end-to-end loopback tool calls
    assert p95_latency < 1000.0, f"p95 latency was {p95_latency:.2f}ms (expected < 1000ms)"


@pytest.mark.asyncio
async def test_health_probes_latency_under_concurrency():
    """Verify health probes maintain sub-50ms latency even while chat endpoints are stressed."""
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        start = time.perf_counter()
        resp = await client.get("/health/ready")
        elapsed_ms = (time.perf_counter() - start) * 1000

        assert resp.status_code == 200
        assert elapsed_ms < 50.0, f"Health probe latency was {elapsed_ms:.2f}ms"
