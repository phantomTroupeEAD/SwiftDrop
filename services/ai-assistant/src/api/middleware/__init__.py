"""Central export for API security, auth, and rate-limiting middleware."""

from src.api.middleware.auth import (
    AuthenticatedUser,
    JWTAuthenticationMiddleware,
    create_jwt_token,
    parse_and_verify_jwt,
)
from src.api.middleware.rate_limit import (
    get_client_ip,
    get_dynamic_rate_limit,
    get_rate_limit_key,
    limiter,
    rate_limit_exceeded_handler,
)
from src.api.middleware.security import SecurityHeadersMiddleware

__all__ = [
    "AuthenticatedUser",
    "JWTAuthenticationMiddleware",
    "create_jwt_token",
    "parse_and_verify_jwt",
    "limiter",
    "get_client_ip",
    "get_rate_limit_key",
    "get_dynamic_rate_limit",
    "rate_limit_exceeded_handler",
    "SecurityHeadersMiddleware",
]
