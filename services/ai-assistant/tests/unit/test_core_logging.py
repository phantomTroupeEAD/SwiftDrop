"""Unit tests for enterprise structured logging and correlation ID propagation."""

import logging
from unittest.mock import MagicMock
from fastapi import FastAPI
from fastapi.testclient import TestClient
import pytest

from src.core.logging import (
    CorrelationIdMiddleware,
    StructuredLogFormatter,
    get_correlation_id,
    set_correlation_id,
    setup_logging,
)


def test_correlation_id_context_accessors():
    """Verify get and set correlation ID context variables."""
    assert get_correlation_id() is None
    set_correlation_id("test-corr-id-123")
    assert get_correlation_id() == "test-corr-id-123"


def test_correlation_id_middleware_with_custom_header():
    """Verify middleware respects incoming X-Correlation-ID."""
    test_app = FastAPI()
    test_app.add_middleware(CorrelationIdMiddleware)

    @test_app.get("/ping")
    def ping():
        return {"corr_id": get_correlation_id()}

    client = TestClient(test_app)
    response = client.get("/ping", headers={"X-Correlation-ID": "custom-trace-999"})
    assert response.status_code == 200
    assert response.headers["X-Correlation-ID"] == "custom-trace-999"
    assert response.json()["corr_id"] == "custom-trace-999"


def test_correlation_id_middleware_with_request_id_header():
    """Verify middleware accepts X-Request-ID fallback."""
    test_app = FastAPI()
    test_app.add_middleware(CorrelationIdMiddleware)

    @test_app.get("/ping")
    def ping():
        return {"corr_id": get_correlation_id()}

    client = TestClient(test_app)
    response = client.get("/ping", headers={"X-Request-ID": "req-id-888"})
    assert response.status_code == 200
    assert response.headers["X-Correlation-ID"] == "req-id-888"


def test_correlation_id_middleware_generates_uuid_when_absent():
    """Verify middleware generates a valid UUID correlation ID when header is missing."""
    test_app = FastAPI()
    test_app.add_middleware(CorrelationIdMiddleware)

    @test_app.get("/ping")
    def ping():
        return {"corr_id": get_correlation_id()}

    client = TestClient(test_app)
    response = client.get("/ping")
    assert response.status_code == 200
    cid = response.headers.get("X-Correlation-ID")
    assert cid is not None
    assert len(cid) >= 32
    assert response.json()["corr_id"] == cid


def test_structured_log_formatter():
    """Verify StructuredLogFormatter formats log record with correlation_id."""
    formatter = StructuredLogFormatter(fmt="%(levelname)s [%(correlation_id)s] %(message)s")
    record = logging.LogRecord(
        name="test_logger",
        level=logging.INFO,
        pathname="",
        lineno=0,
        msg="Test message",
        args=(),
        exc_info=None,
    )

    set_correlation_id("corr-xyz")
    formatted = formatter.format(record)
    assert "[corr-xyz]" in formatted

    set_correlation_id(None)
    record2 = logging.LogRecord(
        name="test_logger",
        level=logging.INFO,
        pathname="",
        lineno=0,
        msg="Another message",
        args=(),
        exc_info=None,
    )
    formatted2 = formatter.format(record2)
    assert "[-]" in formatted2


def test_setup_logging():
    """Verify setup_logging configures root logger without error."""
    setup_logging("DEBUG")
    root_logger = logging.getLogger()
    assert root_logger.level == logging.DEBUG
    assert len(root_logger.handlers) == 1
    # Reset back to INFO
    setup_logging("INFO")
