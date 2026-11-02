"""Authoritative JWT authentication verifier and tenant extraction ASGI middleware.

Matches Member 1 Go Core API cryptographic signing contract (ADR-003, HS256).
Delegates core token verification and user extraction to `src.core.security`.
"""

import re
from fastapi import Request
from fastapi.responses import JSONResponse
from starlette.middleware.base import BaseHTTPMiddleware

from src.core.config import get_settings
from src.core.security import (
    AuthenticatedUser,
    b64url_decode,
    b64url_encode,
    create_jwt_token,
    parse_and_verify_jwt,
)
from src.domain.errors import AuthException

__all__ = [
    "AuthenticatedUser",
    "b64url_decode",
    "b64url_encode",
    "create_jwt_token",
    "parse_and_verify_jwt",
    "JWTAuthenticationMiddleware",
]


class JWTAuthenticationMiddleware(BaseHTTPMiddleware):
    """ASGI Middleware extracting and validating Authorization Bearer JWT on inbound requests."""

    async def dispatch(self, request: Request, call_next):
        request.state.user = None
        auth_header = request.headers.get("Authorization")

        if auth_header:
            auth_clean = auth_header.strip()
            # Enforce clean Bearer prefix format
            bearer_match = re.match(r"^Bearer\s+(.+)$", auth_clean, flags=re.IGNORECASE)
            if not bearer_match:
                return JSONResponse(
                    status_code=401,
                    content={
                        "error": {
                            "code": "AUTH_INVALID_HEADER",
                            "message": "Malformed Authorization header: must start with 'Bearer <token>'",
                            "details": [],
                        }
                    },
                )

            token = bearer_match.group(1).strip()
            settings = get_settings()

            # Handle mock tokens during offline testing / CI
            if settings.is_mock_mode and ("mock-" in token or token.startswith("test-") or "." not in token):
                request.state.user = AuthenticatedUser(
                    user_id="usr_mock_001",
                    role="CUSTOMER",
                    email="customer@swiftdrop.local",
                )
            else:
                try:
                    user = parse_and_verify_jwt(token)
                    request.state.user = user
                except AuthException as auth_exc:
                    return JSONResponse(
                        status_code=401,
                        content=auth_exc.to_dict(),
                    )

        response = await call_next(request)
        return response
