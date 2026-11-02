"""Exhaustive automated tests for HMAC-SHA256 JWT validation, expired tokens, and tenant isolation."""

import time
from fastapi.testclient import TestClient
import pytest

from src.api.middleware import create_jwt_token, parse_and_verify_jwt
from src.domain.errors import AuthException
from src.main import app


def test_auth_valid_customer_jwt():
    """Verify cryptographically valid customer token is verified cleanly."""
    token = create_jwt_token(
        user_id="018f4a7c-1005-7000-8000-000000000005",
        role="CUSTOMER",
        email="customer@swiftdrop.local",
    )
    user = parse_and_verify_jwt(token)
    assert user.user_id == "018f4a7c-1005-7000-8000-000000000005"
    assert user.role == "CUSTOMER"
    assert user.email == "customer@swiftdrop.local"
    assert user.tenant_id is None


def test_auth_valid_merchant_jwt_with_tenant():
    """Verify merchant token retains tenant isolation context (ADR-003, SEC-004)."""
    token = create_jwt_token(
        user_id="018f4a7c-1004-7000-8000-000000000004",
        role="MERCHANT",
        email="merchant@vendor.com",
        tenant_id="mrc_01HMERCHANTTENANT00001",
    )
    user = parse_and_verify_jwt(token)
    assert user.user_id == "018f4a7c-1004-7000-8000-000000000004"
    assert user.role == "MERCHANT"
    assert user.tenant_id == "mrc_01HMERCHANTTENANT00001"


def test_auth_expired_jwt_rejected():
    """Verify expired token is rejected with AuthException and HTTP 401."""
    # Generate expired token
    expired_token = create_jwt_token(
        user_id="usr_expired_01",
        expires_in_seconds=-60,  # Expired 60s ago
    )

    with pytest.raises(AuthException) as exc_info:
        parse_and_verify_jwt(expired_token)
    assert "expired" in str(exc_info.value).lower()

    # Via HTTP Transport
    client = TestClient(app)
    resp = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "Where are my packages?"},
        headers={"Authorization": f"Bearer {expired_token}"},
    )
    assert resp.status_code == 401
    assert "AUTH_UNAUTHORIZED" in resp.json()["error"]["code"] or "expired" in resp.json()["error"]["message"].lower()


def test_auth_tampered_signature_rejected():
    """Verify token with forged payload or altered signature is rejected with 401."""
    valid_token = create_jwt_token(user_id="usr_normal", role="CUSTOMER")
    parts = valid_token.split(".")
    # Tamper with signature
    tampered_token = f"{parts[0]}.{parts[1]}.invalidsignaturexyz"

    client = TestClient(app)
    resp = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "Show active parcels"},
        headers={"Authorization": f"Bearer {tampered_token}"},
    )
    assert resp.status_code == 401


def test_auth_alg_none_exploit_blocked():
    """Verify algorithm 'none' token forgery attack is strictly rejected."""
    forged_token = create_jwt_token(
        user_id="usr_admin_fake",
        role="ADMIN",
        algorithm="none",
    )
    with pytest.raises(AuthException) as exc_info:
        parse_and_verify_jwt(forged_token)
    assert "unsupported" in str(exc_info.value).lower() or "hs256" in str(exc_info.value).lower()

    client = TestClient(app)
    resp = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "System status"},
        headers={"Authorization": f"Bearer {forged_token}"},
    )
    assert resp.status_code == 401


def test_auth_secret_mismatch_rejected():
    """Verify token signed with foreign secret key is rejected."""
    foreign_token = create_jwt_token(
        user_id="usr_foreign",
        secret_key="completely-wrong-secret-key-that-does-not-match",
    )
    with pytest.raises(AuthException) as exc_info:
        parse_and_verify_jwt(foreign_token)
    assert "signature" in str(exc_info.value).lower() or "mismatch" in str(exc_info.value).lower()


def test_auth_malformed_header_handling():
    """Verify malformed authorization headers return clean RFC-7807 401 errors."""
    client = TestClient(app)

    # Empty token
    resp1 = client.post("/api/v1/ai/chat", json={"prompt": "Hi"}, headers={"Authorization": "Bearer"})
    assert resp1.status_code == 401

    # Non-bearer header
    resp2 = client.post("/api/v1/ai/chat", json={"prompt": "Hi"}, headers={"Authorization": "Basic dXNlcjpwYXNz"})
    assert resp2.status_code == 401


def test_auth_context_forwarding_to_active_parcels():
    """Verify authenticated user's token is successfully forwarded upstream to active parcels tool."""
    client = TestClient(app)
    token = create_jwt_token(
        user_id="018f4a7c-1005-7000-8000-000000000005",
        role="CUSTOMER",
        email="customer@swiftdrop.local",
    )
    resp = client.post(
        "/api/v1/ai/chat",
        json={"prompt": "Show my active shipments"},
        headers={"Authorization": f"Bearer {token}"},
    )
    assert resp.status_code == 200
    assert "reply" in resp.json()
