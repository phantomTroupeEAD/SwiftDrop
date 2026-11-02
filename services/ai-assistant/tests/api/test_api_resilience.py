"""Fault injection, resilience, and error boundary tests for the transport layer."""

from unittest.mock import patch
import pytest
from fastapi.testclient import TestClient

from src.domain.errors import UpstreamServiceException


def test_global_exception_handler_masks_internal_error():
    """Verify unhandled exceptions return 500 RFC-7807 envelope with zero stack trace leakage."""
    from src.main import app
    safe_client = TestClient(app, raise_server_exceptions=False)
    with patch("src.services.chat_service.get_llm_client") as mock_get_client:
        mock_get_client.side_effect = RuntimeError("Simulated catastrophic memory corruption!")
        response = safe_client.post("/api/v1/ai/chat", json={"prompt": "Hello"})
        assert response.status_code == 500
        data = response.json()
        assert "error" in data
        assert data["error"]["code"] == "INTERNAL_SERVER_ERROR"
        assert "Simulated catastrophic memory corruption" not in data["error"]["message"]
        assert "Traceback" not in response.text


def test_malformed_json_returns_422(client: TestClient):
    """Verify malformed JSON payloads return 422 with RFC-7807 envelope."""
    response = client.post(
        "/api/v1/ai/chat",
        content="{broken json: true,",
        headers={"Content-Type": "application/json"},
    )
    assert response.status_code == 422
    data = response.json()
    assert "error" in data
    assert data["error"]["code"] == "VALIDATION_ERROR"


def test_decoupled_health_probes_during_upstream_outage(client: TestClient):
    """Verify health probes return 200 OK without making external calls, even during an upstream outage."""
    with patch("httpx.AsyncClient.get") as mock_http:
        mock_http.side_effect = ConnectionRefusedError("Upstream service is down")

        live_res = client.get("/health/live")
        assert live_res.status_code == 200
        assert live_res.json()["status"] == "UP"

        ready_res = client.get("/health/ready")
        assert ready_res.status_code == 200
        assert ready_res.json()["status"] == "READY"

        # Assert no HTTP calls were made by health probes
        mock_http.assert_not_called()


def test_upstream_domain_exception_handling(client: TestClient):
    """Verify UpstreamServiceException is converted to RFC-7807 502 Bad Gateway response."""
    with patch("src.services.chat_service.get_llm_client") as mock_get_client:
        mock_client = mock_get_client.return_value
        mock_client.generate_reply.side_effect = UpstreamServiceException(
            service_name="core-pricing",
            message="Connection timeout after 5000ms",
        )

        response = client.post("/api/v1/ai/chat", json={"prompt": "How much for 2kg?"})
        assert response.status_code == 502
        data = response.json()
        assert data["error"]["code"] == "UPSTREAM_SERVICE_UNAVAILABLE"
        assert "core-pricing" in data["error"]["message"]
