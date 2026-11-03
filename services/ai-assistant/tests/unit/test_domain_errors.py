"""Unit tests for domain errors, status codes, and JSON error envelope serialization."""

from src.domain.errors import (
    AppException,
    AuthException,
    ErrorDetail,
    ErrorResponse,
    PromptInjectionException,
    RateLimitException,
    ResourceNotFoundException,
    UpstreamServiceException,
    ValidationException,
)


def test_validation_exception():
    """Verify ValidationException defaults and structure."""
    exc = ValidationException(
        message="Invalid parcel inputs",
        details=[ErrorDetail(field="weight_kg", issue="Must be > 0.0")],
    )
    assert exc.status_code == 422
    assert exc.code == "VALIDATION_ERROR"
    data = exc.to_dict()
    assert data["error"]["code"] == "VALIDATION_ERROR"
    assert data["error"]["details"][0]["field"] == "weight_kg"
    # Validate against ErrorResponse schema
    ErrorResponse.model_validate(data)


def test_prompt_injection_exception():
    """Verify PromptInjectionException maps to HTTP 400."""
    exc = PromptInjectionException("Adversarial prompt detected")
    assert exc.status_code == 400
    assert exc.code == "PROMPT_INJECTION_DETECTED"
    data = exc.to_dict()
    ErrorResponse.model_validate(data)


def test_resource_not_found_exception():
    """Verify ResourceNotFoundException maps to HTTP 404."""
    exc = ResourceNotFoundException(
        message="No shipment found", resource_id="SD-20261021-00000"
    )
    assert exc.status_code == 404
    assert exc.code == "PARCEL_NOT_FOUND"
    data = exc.to_dict()
    assert "SD-20261021-00000" in data["error"]["details"][0]["issue"]
    ErrorResponse.model_validate(data)


def test_upstream_service_exception():
    """Verify UpstreamServiceException maps to HTTP 502."""
    exc = UpstreamServiceException(service_name="Tracking API", message="Connection refused")
    assert exc.status_code == 502
    assert exc.code == "UPSTREAM_SERVICE_UNAVAILABLE"
    data = exc.to_dict()
    assert "Tracking API" in data["error"]["message"]
    ErrorResponse.model_validate(data)


def test_rate_limit_exception():
    """Verify RateLimitException maps to HTTP 429."""
    exc = RateLimitException(retry_after_seconds=30)
    assert exc.status_code == 429
    assert exc.code == "RATE_LIMIT_EXCEEDED"
    assert "30 seconds" in exc.message
    data = exc.to_dict()
    ErrorResponse.model_validate(data)


def test_auth_exception():
    """Verify AuthException maps to HTTP 401."""
    exc = AuthException("Expired token")
    assert exc.status_code == 401
    assert exc.code == "AUTH_UNAUTHORIZED"
    data = exc.to_dict()
    ErrorResponse.model_validate(data)
