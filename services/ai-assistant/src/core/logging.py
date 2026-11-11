"""Enterprise structured logging and distributed request correlation context.

Provides correlation ID propagation (X-Correlation-ID / X-Request-ID) across
async tasks, FastAPI middlewares, and structured log records.
"""

from contextvars import ContextVar
import json
import logging
import uuid
from typing import Any, Dict, Optional

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response

# Context variable preserving correlation ID across async execution flows
correlation_id_ctx: ContextVar[Optional[str]] = ContextVar("correlation_id", default=None)


def get_correlation_id() -> Optional[str]:
    """Retrieve the active request's correlation ID."""
    return correlation_id_ctx.get()


def set_correlation_id(cid: str) -> None:
    """Explicitly set the active correlation ID in context."""
    correlation_id_ctx.set(cid)


class CorrelationIdMiddleware(BaseHTTPMiddleware):
    """ASGI middleware capturing or generating request correlation IDs and binding them to responses."""

    HEADER_NAMES = ("x-correlation-id", "x-request-id")

    async def dispatch(self, request: Request, call_next) -> Response:
        # Check inbound headers for existing correlation ID from API Gateway / Traefik
        cid: Optional[str] = None
        for header in self.HEADER_NAMES:
            val = request.headers.get(header)
            if val and val.strip():
                cid = val.strip()
                break

        if not cid:
            cid = str(uuid.uuid4())

        # Set in async context
        token = correlation_id_ctx.set(cid)
        try:
            response = await call_next(request)
            response.headers["X-Correlation-ID"] = cid
            return response
        finally:
            correlation_id_ctx.reset(token)


class StructuredLogFormatter(logging.Formatter):
    """Structured log formatter injecting correlation_id into log output."""

    def format(self, record: logging.LogRecord) -> str:
        cid = get_correlation_id()
        record.correlation_id = cid or "-"
        return super().format(record)


def setup_logging(log_level: str = "INFO") -> None:
    """Initialize structured console logging for the SwiftDrop AI Assistant."""
    handler = logging.StreamHandler()
    formatter = StructuredLogFormatter(
        fmt="%(asctime)s [%(levelname)s] [corr_id:%(correlation_id)s] %(name)s: %(message)s"
    )
    handler.setFormatter(formatter)

    root_logger = logging.getLogger()
    # Avoid duplicate handlers on re-runs
    root_logger.handlers = [handler]
    root_logger.setLevel(getattr(logging, log_level.upper(), logging.INFO))
