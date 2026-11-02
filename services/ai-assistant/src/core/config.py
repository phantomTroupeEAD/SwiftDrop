"""Central runtime configuration and environment settings for SwiftDrop AI Assistant."""

from functools import lru_cache
from typing import Optional
from pydantic import Field, SecretStr, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Configuration settings loaded from environment variables or .env file."""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
        case_sensitive=True,
    )

    # Server Network & Runtime Environment
    PORT: int = Field(default=8000, ge=1024, le=65535, description="ASGI listening port")
    ENVIRONMENT: str = Field(
        default="development",
        pattern="^(development|test|production)$",
        description="Deployment stage",
    )
    LOG_LEVEL: str = Field(
        default="info",
        pattern="^(debug|info|warning|error)$",
        description="Logging verbosity",
    )

    # LLM Provider Configuration (Universal OpenAI-Compatible Client)
    LLM_BASE_URL: str = Field(
        default="https://openrouter.ai/api/v1",
        description="Base URL for OpenAI-compatible endpoint",
    )
    LLM_API_KEY: SecretStr = Field(
        default=SecretStr(""),
        description="Secret API key for chosen LLM provider (masked in logs)",
    )
    LLM_MODEL: str = Field(
        default="meta-llama/llama-3.1-70b-instruct",
        description="Target model identifier",
    )
    LLM_TEMPERATURE: float = Field(
        default=0.2,
        ge=0.0,
        le=1.0,
        description="LLM sampling temperature",
    )

    # Offline CI / Automated Testing Toggle
    MOCK_LLM_MODE: bool = Field(
        default=True,
        description="When true or if LLM_API_KEY is unset, offline deterministic mock is used",
    )

    # Upstream Microservice Endpoints
    CORE_API_URL: str = Field(
        default="http://core-api:8080",
        description="Internal network URL for Core API Modular Monolith",
    )
    TRACKING_API_URL: str = Field(
        default="http://tracking-api:8081",
        description="Internal network URL for Tracking Service",
    )

    # Security, Service Tokens & Rate Limiting
    AI_SERVICE_ROLE_TOKEN: SecretStr = Field(
        default=SecretStr("dev-internal-secret-key-123"),
        description="Shared secret token for internal Core API service-role requests",
    )
    INTERNAL_SERVICE_KEY: Optional[SecretStr] = Field(
        default=None,
        description="Legacy fallback alias for AI_SERVICE_ROLE_TOKEN",
    )
    AI_RATE_LIMIT_RPM: int = Field(
        default=20,
        ge=1,
        le=1000,
        description="Max requests per minute per IP matching Traefik edge rate limit",
    )

    # In-Memory Conversational Session Cache (Stateless Isolation)
    AI_SESSION_TTL_MINUTES: int = Field(
        default=15,
        ge=1,
        le=1440,
        description="Session context time-to-live window in minutes",
    )
    AI_SESSION_MAX_ACTIVE: int = Field(
        default=50,
        ge=1,
        le=100000,
        description="Maximum concurrent active sessions before LRU eviction",
    )
    POLICY_CACHE_TTL_MINUTES: int = Field(
        default=30,
        ge=1,
        le=1440,
        description="In-memory cache TTL for company policy overrides",
    )

    @field_validator("LLM_BASE_URL", "CORE_API_URL", "TRACKING_API_URL")
    @classmethod
    def validate_http_url(cls, v: str) -> str:
        """Validate URL scheme and block link-local/cloud metadata SSRF targets."""
        v_clean = v.strip().rstrip("/")
        if not (v_clean.startswith("http://") or v_clean.startswith("https://")):
            raise ValueError(f"URL must start with http:// or https://: {v}")
        if "169.254.169.254" in v_clean:
            raise ValueError(f"Cloud metadata endpoint is strictly prohibited (SSRF defense): {v}")
        return v_clean

    @property
    def core_api_base_url(self) -> str:
        return self.CORE_API_URL

    @property
    def tracking_api_base_url(self) -> str:
        return self.TRACKING_API_URL

    @property
    def ai_service_role_token(self) -> SecretStr:
        return self.AI_SERVICE_ROLE_TOKEN

    @property
    def is_mock_mode(self) -> bool:
        """Determine if service should execute offline deterministic mock LLM."""
        return self.MOCK_LLM_MODE or len(self.LLM_API_KEY.get_secret_value().strip()) == 0

    @property
    def effective_service_role_token(self) -> str:
        """Get the active service-role token, falling back to legacy key if needed."""
        primary = self.AI_SERVICE_ROLE_TOKEN.get_secret_value().strip()
        if primary and primary != "dev-internal-secret-key-123":
            return primary
        if self.INTERNAL_SERVICE_KEY:
            fallback = self.INTERNAL_SERVICE_KEY.get_secret_value().strip()
            if fallback:
                return fallback
        return primary or "dev-internal-secret-key-123"


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    """Return cached application settings singleton."""
    return Settings()
