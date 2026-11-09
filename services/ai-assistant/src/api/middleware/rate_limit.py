"""SlowAPI rate limiting middleware, dynamic tiered ceilings, and RFC-7807 429 handler."""

import re
from typing import Optional
from fastapi import Request
from fastapi.responses import JSONResponse
from slowapi import Limiter
from slowapi.errors import RateLimitExceeded
from slowapi.util import get_remote_address

from src.core.config import get_settings


def get_client_ip(request: Request) -> str:
    """Safely extract the origin client IP handling reverse proxy headers (Traefik)."""
    forwarded_for = request.headers.get("X-Forwarded-For")
    if forwarded_for:
        # X-Forwarded-For format: client, proxy1, proxy2
        first_ip = forwarded_for.split(",")[0].strip()
        # Basic sanity check on IP string format
        if re.match(r"^[0-9a-fA-F:\.]+$", first_ip):
            return first_ip

    real_ip = request.headers.get("X-Real-IP")
    if real_ip:
        clean_real = real_ip.strip()
        if re.match(r"^[0-9a-fA-F:\.]+$", clean_real):
            return clean_real

    if request.client and request.client.host:
        return request.client.host

    return "127.0.0.1"


def get_rate_limit_key(request: Request) -> str:
    """Derive rate limit key using authenticated user ID or sanitized client IP."""
    user = getattr(request.state, "user", None)
    if user:
        role = str(getattr(user, "role", "CUSTOMER")).upper()
        if role in ("MERCHANT", "ADMIN", "DISPATCHER") or getattr(user, "tenant_id", None):
            identifier = getattr(user, "tenant_id", None) or getattr(user, "user_id", "unknown")
            return f"tenant:{identifier}"
        return f"user:{getattr(user, 'user_id', 'unknown')}"

    client_ip = get_client_ip(request)
    return f"ip:{client_ip}"


def get_dynamic_rate_limit(key: str = "") -> str:
    """Calculate role-aware tiered rate limit dynamically loaded from settings based on key prefix."""
    settings = get_settings()

    if key.startswith("tenant:"):
        return f"{settings.RATE_LIMIT_MERCHANT_RPM}/minute"
    if key.startswith("user:"):
        return f"{settings.RATE_LIMIT_CUSTOMER_RPM}/minute"

    return f"{settings.RATE_LIMIT_ANONYMOUS_RPM}/minute"


# Initialize centralized in-memory SlowAPI Limiter
limiter = Limiter(
    key_func=get_rate_limit_key,
    default_limits=[],  # Chat endpoint defines its dynamic limit explicitly
    storage_uri="memory://",
    headers_enabled=True,
)


async def rate_limit_exceeded_handler(request: Request, exc: RateLimitExceeded) -> JSONResponse:
    """Format SlowAPI RateLimitExceeded into strict RFC-7807 compliant error envelope."""
    # Extract retry window from exception details or default to 60 seconds
    retry_after = 60
    if hasattr(exc, "detail") and isinstance(exc.detail, str):
        match = re.search(r"(\d+)\s*second", exc.detail)
        if match:
            retry_after = int(match.group(1))

    content = {
        "error": {
            "code": "RATE_LIMIT_EXCEEDED",
            "message": f"Rate limit exceeded. Please wait {retry_after} seconds before retrying.",
            "details": [
                {
                    "field": "rate_limit",
                    "issue": f"Request frequency exceeded your tier's quota. Retry after {retry_after}s.",
                }
            ],
        }
    }

    headers = {
        "Retry-After": str(retry_after),
        "X-RateLimit-Limit": "20",
        "X-RateLimit-Remaining": "0",
    }

    return JSONResponse(
        status_code=429,
        content=content,
        headers=headers,
    )
