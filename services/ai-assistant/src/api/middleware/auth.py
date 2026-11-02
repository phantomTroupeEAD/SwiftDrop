"""Authoritative JWT authentication verifier and tenant extraction middleware.

Matches Member 1 Go Core API cryptographic signing contract (ADR-003, HS256).
Zero external dependencies: implemented purely with Python's hmac, hashlib, and base64.
"""

import base64
from dataclasses import dataclass
import hashlib
import hmac
import json
import re
import time
from typing import Any, Dict, Optional

from fastapi import Request
from fastapi.responses import JSONResponse
from starlette.middleware.base import BaseHTTPMiddleware

from src.core.config import get_settings
from src.domain.errors import AuthException


@dataclass(frozen=True)
class AuthenticatedUser:
    """Authoritative representation of an authenticated SwiftDrop principal."""

    user_id: str
    role: str
    email: str
    tenant_id: Optional[str] = None


def b64url_decode(input_str: str) -> bytes:
    """Decode raw URL-safe Base64 string without padding."""
    rem = len(input_str) % 4
    if rem > 0:
        input_str += "=" * (4 - rem)
    return base64.urlsafe_b64decode(input_str.encode("ascii"))


def b64url_encode(input_bytes: bytes) -> str:
    """Encode bytes into raw URL-safe Base64 string without '=' padding."""
    return base64.urlsafe_b64encode(input_bytes).rstrip(b"=").decode("ascii")


def create_jwt_token(
    user_id: str,
    role: str = "CUSTOMER",
    email: str = "user@swiftdrop.local",
    tenant_id: Optional[str] = None,
    expires_in_seconds: int = 900,
    secret_key: Optional[str] = None,
    issuer: str = "https://api.swiftdrop.io",
    algorithm: str = "HS256",
) -> str:
    """Generate a canonical SwiftDrop Access Token matching Core API's structure (ADR-003)."""
    settings = get_settings()
    secret = secret_key or settings.jwt_secret_key
    now = int(time.time())

    header = {"alg": algorithm, "typ": "JWT"}
    claims: Dict[str, Any] = {
        "iss": issuer,
        "sub": user_id,
        "role": role,
        "email": email,
        "iat": now,
        "exp": now + expires_in_seconds,
        "jti": f"jwt_{hashlib.sha256(f'{user_id}:{now}'.encode()).hexdigest()[:12]}",
    }
    if tenant_id:
        claims["tenant_id"] = tenant_id

    header_b64 = b64url_encode(json.dumps(header, separators=(",", ":")).encode("utf-8"))
    claims_b64 = b64url_encode(json.dumps(claims, separators=(",", ":")).encode("utf-8"))
    signing_input = f"{header_b64}.{claims_b64}"

    if algorithm == "none":
        return f"{signing_input}."

    signature = hmac.new(
        secret.encode("utf-8"),
        signing_input.encode("ascii"),
        hashlib.sha256,
    ).digest()
    sig_b64 = b64url_encode(signature)

    return f"{signing_input}.{sig_b64}"


def parse_and_verify_jwt(
    token_str: str,
    secret_key: Optional[str] = None,
) -> AuthenticatedUser:
    """Verify HMAC-SHA256 signature, expiry, and extract AuthenticatedUser principal."""
    settings = get_settings()
    secret = secret_key or settings.jwt_secret_key

    parts = token_str.strip().split(".")
    if len(parts) != 3:
        raise AuthException("Malformed JWT: token must contain exactly 3 dot-separated segments")

    header_b64, claims_b64, sig_b64 = parts[0], parts[1], parts[2]

    # 1. Parse header and enforce algorithm
    try:
        header_json = json.loads(b64url_decode(header_b64).decode("utf-8"))
    except Exception as exc:
        raise AuthException(f"Invalid JWT header encoding: {exc}")

    alg = header_json.get("alg")
    if alg != "HS256":
        raise AuthException(f"Unsupported JWT algorithm '{alg}'. Only HS256 is accepted.")

    # 2. Verify signature with constant-time comparison
    signing_input = f"{header_b64}.{claims_b64}"
    expected_sig = hmac.new(
        secret.encode("utf-8"),
        signing_input.encode("ascii"),
        hashlib.sha256,
    ).digest()
    expected_b64 = b64url_encode(expected_sig)

    if not hmac.compare_digest(sig_b64, expected_b64):
        raise AuthException("Invalid JWT signature: token has been forged or secret key mismatch")

    # 3. Parse claims
    try:
        claims_json = json.loads(b64url_decode(claims_b64).decode("utf-8"))
    except Exception as exc:
        raise AuthException(f"Invalid JWT claims encoding: {exc}")

    # 4. Enforce expiration time
    exp = claims_json.get("exp")
    if exp is None:
        raise AuthException("Token missing mandatory expiration timestamp ('exp')")

    if time.time() > float(exp):
        raise AuthException("Authentication token has expired")

    user_id = claims_json.get("sub") or claims_json.get("user_id")
    if not user_id:
        raise AuthException("Token missing subject identifier ('sub')")

    role = claims_json.get("role", "CUSTOMER")
    email = claims_json.get("email", "")
    tenant_id = claims_json.get("tenant_id")

    return AuthenticatedUser(
        user_id=str(user_id),
        role=str(role).upper(),
        email=str(email),
        tenant_id=str(tenant_id) if tenant_id else None,
    )


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
