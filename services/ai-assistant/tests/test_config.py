"""Unit tests for configuration loading, SecretStr masking, and SSRF URL validation."""

import pytest
from pydantic import ValidationError
from src.core.config import Settings, get_settings


def test_default_settings():
    """Verify default settings initialization."""
    settings = Settings()
    assert settings.PORT == 8000
    assert settings.ENVIRONMENT in ["development", "test"]
    assert settings.is_mock_mode is True


def test_secret_str_masking():
    """Verify that sensitive API keys and tokens are masked during string serialization."""
    settings = Settings(LLM_API_KEY="sk-real-secret-key-12345")
    assert "sk-real-secret-key-12345" not in str(settings.LLM_API_KEY)
    assert "**********" in str(settings.LLM_API_KEY)
    assert settings.LLM_API_KEY.get_secret_value() == "sk-real-secret-key-12345"


def test_is_mock_mode_logic():
    """Verify mock mode decision matrix."""
    # When mock mode is explicitly True
    s1 = Settings(MOCK_LLM_MODE=True, LLM_API_KEY="sk-test-key")
    assert s1.is_mock_mode is True

    # When mock mode is False but API key is empty
    s2 = Settings(MOCK_LLM_MODE=False, LLM_API_KEY="")
    assert s2.is_mock_mode is True

    # When mock mode is False and a valid API key is present
    s3 = Settings(MOCK_LLM_MODE=False, LLM_API_KEY="sk-live-production-key")
    assert s3.is_mock_mode is False


def test_url_validation_and_ssrf_blocking():
    """Verify URL scheme enforcement and AWS/GCP metadata service SSRF blocking."""
    # Valid URLs
    s = Settings(
        CORE_API_URL="http://core-api:8080",
        TRACKING_API_URL="https://tracking.swiftdrop.local:8081",
    )
    assert s.CORE_API_URL == "http://core-api:8080"
    assert s.TRACKING_API_URL == "https://tracking.swiftdrop.local:8081"

    # Invalid scheme (ftp, file, etc.)
    with pytest.raises(ValidationError) as exc_info:
        Settings(CORE_API_URL="ftp://internal-server:8080")
    assert "URL must start with http:// or https://" in str(exc_info.value)

    with pytest.raises(ValidationError) as exc_info:
        Settings(CORE_API_URL="file:///etc/passwd")
    assert "URL must start with http:// or https://" in str(exc_info.value)

    # Cloud metadata SSRF blocking
    with pytest.raises(ValidationError) as exc_info:
        Settings(CORE_API_URL="http://169.254.169.254/latest/meta-data/")
    assert "Cloud metadata endpoint is strictly prohibited" in str(exc_info.value)


def test_port_bounds_validation():
    """Verify valid port ranges (1024-65535)."""
    # Valid port
    assert Settings(PORT=8080).PORT == 8080

    # Privileged port < 1024
    with pytest.raises(ValidationError):
        Settings(PORT=80)

    # Out of range port > 65535
    with pytest.raises(ValidationError):
        Settings(PORT=70000)


def test_service_role_token_fallback():
    """Verify fallback mechanism to legacy INTERNAL_SERVICE_KEY."""
    s1 = Settings(AI_SERVICE_ROLE_TOKEN="custom-primary-token-123")
    assert s1.effective_service_role_token == "custom-primary-token-123"

    s2 = Settings(
        AI_SERVICE_ROLE_TOKEN="dev-internal-secret-key-123",
        INTERNAL_SERVICE_KEY="legacy-fallback-key-456",
    )
    assert s2.effective_service_role_token == "legacy-fallback-key-456"


def test_get_settings_singleton():
    """Verify that get_settings() returns the same cached instance."""
    s1 = get_settings()
    s2 = get_settings()
    assert s1 is s2
