"""Domain error envelopes, error codes, and application exception hierarchies."""

from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field


class ErrorDetail(BaseModel):
    """Specific field-level or attribute issue detail."""

    field: Optional[str] = None
    issue: str


class ErrorPayload(BaseModel):
    """Canonical error payload matching SwiftDrop system standard."""

    code: str
    message: str
    details: List[ErrorDetail] = Field(default_factory=list)


class ErrorResponse(BaseModel):
    """Standardized error envelope returned across all HTTP responses."""

    error: ErrorPayload


class AppException(Exception):
    """Base application exception for all domain errors."""

    def __init__(
        self,
        code: str,
        message: str,
        status_code: int = 500,
        details: Optional[List[ErrorDetail]] = None,
    ):
        super().__init__(message)
        self.code = code
        self.message = message
        self.status_code = status_code
        self.details = details or []

    def to_dict(self) -> Dict[str, Any]:
        """Convert exception to canonical JSON error envelope."""
        return {
            "error": {
                "code": self.code,
                "message": self.message,
                "details": [d.model_dump() for d in self.details],
            }
        }


class ValidationException(AppException):
    """Raised when client request payload fails semantic domain validation."""

    def __init__(self, message: str, details: Optional[List[ErrorDetail]] = None):
        super().__init__(
            code="VALIDATION_ERROR",
            message=message,
            status_code=422,
            details=details,
        )


class PromptInjectionException(AppException):
    """Raised when an inbound prompt contains adversarial injection or jailbreak patterns."""

    def __init__(self, message: str = "Adversarial prompt pattern detected and rejected"):
        super().__init__(
            code="PROMPT_INJECTION_DETECTED",
            message=message,
            status_code=400,
        )


class ResourceNotFoundException(AppException):
    """Raised when a requested resource (e.g. tracking number) is not found upstream."""

    def __init__(self, message: str, resource_id: Optional[str] = None):
        details = [ErrorDetail(field="id", issue=f"Resource {resource_id} not found")] if resource_id else []
        super().__init__(
            code="PARCEL_NOT_FOUND",
            message=message,
            status_code=404,
            details=details,
        )


class UpstreamServiceException(AppException):
    """Raised when an upstream Core or Tracking API call fails or times out."""

    def __init__(self, service_name: str, message: str):
        super().__init__(
            code="UPSTREAM_SERVICE_UNAVAILABLE",
            message=f"Upstream service '{service_name}' failure: {message}",
            status_code=502,
            details=[ErrorDetail(field="upstream", issue=service_name)],
        )


class RateLimitException(AppException):
    """Raised when client exceeds rate limit quotas."""

    def __init__(self, retry_after_seconds: int = 60):
        super().__init__(
            code="RATE_LIMIT_EXCEEDED",
            message=f"Rate limit exceeded. Try again in {retry_after_seconds} seconds.",
            status_code=429,
        )


class AuthException(AppException):
    """Raised when client is unauthorized or provides invalid bearer credentials."""

    def __init__(self, message: str = "Invalid or missing authorization credentials"):
        super().__init__(
            code="AUTH_UNAUTHORIZED",
            message=message,
            status_code=401,
        )
