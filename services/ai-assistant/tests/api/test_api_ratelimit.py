"""Exhaustive automated tests for SlowAPI tiered rate limiting, headers, and probe exemptions."""

import pytest
from fastapi.testclient import TestClient

from src.api.middleware import create_jwt_token, limiter
from src.main import app


@pytest.fixture(autouse=True)
def reset_rate_limiter():
    """Ensure in-memory rate limiter state is reset between tests."""
    limiter.reset()
    yield
    limiter.reset()


def test_rate_limit_anonymous_tier_enforcement():
    """Verify unauthenticated requests are capped at RATE_LIMIT_ANONYMOUS_RPM (20 req/min)."""
    client = TestClient(app)
    ip_headers = {"X-Forwarded-For": "198.51.100.10"}

    # 1. First 20 requests must succeed
    for i in range(20):
        resp = client.post(
            "/api/v1/ai/chat",
            json={"prompt": f"Test anonymous ping {i}"},
            headers=ip_headers,
        )
        assert resp.status_code == 200, f"Request {i+1} failed with {resp.status_code}"

    # 2. 21st request must be blocked with HTTP 429
    blocked_resp = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "Test blocked ping 21"},
        headers=ip_headers,
    )
    assert blocked_resp.status_code == 429
    data = blocked_resp.json()

    # 3. Assert RFC-7807 error format and headers
    assert "error" in data
    assert data["error"]["code"] == "RATE_LIMIT_EXCEEDED"
    assert "Retry-After" in blocked_resp.headers
    assert "X-RateLimit-Limit" in blocked_resp.headers


def test_rate_limit_health_and_docs_exempt():
    """Verify health probes and documentation endpoints are never blocked by chat rate limits."""
    client = TestClient(app)
    ip_headers = {"X-Forwarded-For": "198.51.100.20"}

    # Exhaust rate limit on chat endpoint
    for _ in range(21):
        client.post("/api/v1/ai/chat", json={"prompt": "Ping"}, headers=ip_headers)

    # Health and docs probes must still return 200 OK
    assert client.get("/health/live", headers=ip_headers).status_code == 200
    assert client.get("/health/ready", headers=ip_headers).status_code == 200
    assert client.get("/health/metrics", headers=ip_headers).status_code == 200
    assert client.get("/docs", headers=ip_headers).status_code == 200
    assert client.get("/openapi.json", headers=ip_headers).status_code == 200


def test_rate_limit_distinct_ip_isolation():
    """Verify rate limits are isolated between distinct client IPs."""
    client = TestClient(app)
    ip_a = {"X-Forwarded-For": "203.0.113.1"}
    ip_b = {"X-Forwarded-For": "203.0.113.2"}

    # Exhaust IP A
    for _ in range(20):
        client.post("/api/v1/ai/chat", json={"prompt": "IP A ping"}, headers=ip_a)

    assert client.post("/api/v1/ai/chat", json={"prompt": "IP A over"}, headers=ip_a).status_code == 429

    # IP B must not be impacted
    assert client.post("/api/v1/ai/chat", json={"prompt": "IP B initial"}, headers=ip_b).status_code == 200


def test_rate_limit_customer_tier_higher_quota():
    """Verify authenticated customer can exceed the anonymous 20 RPM limit (up to 60 RPM)."""
    client = TestClient(app)
    customer_token = create_jwt_token(
        user_id="018f4a7c-1005-7000-8000-000000000005",
        role="CUSTOMER",
        email="customer@swiftdrop.local",
    )
    auth_headers = {
        "Authorization": f"Bearer {customer_token}",
        "X-Forwarded-For": "198.51.100.30",
    }

    # Send 25 requests as authenticated customer (exceeding anonymous 20 RPM limit)
    for i in range(25):
        resp = client.post(
            "/api/v1/ai/chat",
            json={"prompt": f"Customer ping {i}"},
            headers=auth_headers,
        )
        assert resp.status_code == 200, f"Customer was throttled at request {i+1} with {resp.status_code}"


def test_rate_limit_merchant_tier_higher_quota():
    """Verify authenticated merchant can exceed customer limit (up to 100 RPM)."""
    client = TestClient(app)
    merchant_token = create_jwt_token(
        user_id="018f4a7c-1004-7000-8000-000000000004",
        role="MERCHANT",
        email="merchant@vendor.com",
        tenant_id="mrc_01HMERCHANTTENANT00001",
    )
    auth_headers = {
        "Authorization": f"Bearer {merchant_token}",
        "X-Forwarded-For": "198.51.100.40",
    }

    # Send 30 requests as authenticated merchant
    for i in range(30):
        resp = client.post(
            "/api/v1/ai/chat",
            json={"prompt": f"Merchant ping {i}"},
            headers=auth_headers,
        )
        assert resp.status_code == 200, f"Merchant was throttled at request {i+1}"
