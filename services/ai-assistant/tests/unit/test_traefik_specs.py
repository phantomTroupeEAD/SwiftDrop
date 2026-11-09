"""Unit test suite verifying Traefik dynamic routing rules, middleware bindings, and FastMCP SSE proxy routes."""

from pathlib import Path
import pytest
import yaml

TRAEFIK_ROUTES_PATH = Path(__file__).resolve().parent.parent.parent.parent.parent / "deploy" / "gateway" / "traefik" / "dynamic-routes.yml"


@pytest.fixture
def traefik_config() -> dict:
    assert TRAEFIK_ROUTES_PATH.exists(), f"Traefik dynamic routes file not found at {TRAEFIK_ROUTES_PATH}"
    with open(TRAEFIK_ROUTES_PATH, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)


def test_traefik_yaml_validity(traefik_config: dict):
    """TC-TRK-001: Verify Traefik dynamic routing file is valid YAML and contains http routers/services."""
    assert "http" in traefik_config
    assert "routers" in traefik_config["http"]
    assert "services" in traefik_config["http"]


def test_traefik_ai_rest_router(traefik_config: dict):
    """TC-TRK-002: Verify AI Assistant REST API router rule and upstream service mapping."""
    routers = traefik_config["http"]["routers"]
    assert "ai-router" in routers
    ai_router = routers["ai-router"]
    
    assert ai_router["rule"] == "PathPrefix(`/api/v1/ai`)"
    assert "web" in ai_router["entryPoints"]
    assert ai_router["service"] == "ai-service"
    assert "cors-middleware" in ai_router["middlewares"]


def test_traefik_ai_mcp_sse_router(traefik_config: dict):
    """TC-TRK-003: Verify FastMCP SSE Gateway router rule and upstream port 8002 mapping."""
    routers = traefik_config["http"]["routers"]
    assert "ai-mcp-router" in routers
    mcp_router = routers["ai-mcp-router"]
    
    assert mcp_router["rule"] == "PathPrefix(`/mcp`)"
    assert "web" in mcp_router["entryPoints"]
    assert mcp_router["service"] == "ai-mcp-service"
    assert "cors-middleware" in mcp_router["middlewares"]


def test_traefik_upstream_services(traefik_config: dict):
    """Verify backend load balancer targets for ai-service (:8000) and ai-mcp-service (:8002)."""
    services = traefik_config["http"]["services"]
    
    assert "ai-service" in services
    ai_servers = services["ai-service"]["loadBalancer"]["servers"]
    assert any(s["url"] == "http://ai-assistant:8000" for s in ai_servers)
    
    assert "ai-mcp-service" in services
    mcp_servers = services["ai-mcp-service"]["loadBalancer"]["servers"]
    assert any(s["url"] == "http://ai-assistant:8002" for s in mcp_servers)
