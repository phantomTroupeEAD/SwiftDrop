"""Health and telemetry probes for service monitoring, Kubernetes liveness/readiness, and metrics."""

from datetime import datetime, timezone
import time
from typing import Any, Dict
from fastapi import APIRouter

from src import __version__
from src.core.cache import get_session_cache
from src.core.config import get_settings
from src.tools.registry import get_registered_tools

router = APIRouter()

# Track service process initialization time
_START_TIME = time.time()


@router.get(
    "",
    summary="Health Overview",
    description="Verifies the service process and asyncio event loop are responsive.",
    response_model=Dict[str, Any],
)
async def get_health_overview() -> Dict[str, Any]:
    """Return 200 UP indicating that the service event loop is healthy."""
    return {
        "status": "UP",
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "service": "ai-assistant",
    }


@router.get(
    "/live",
    summary="Liveness Probe",
    description="Verifies the service process and asyncio event loop are responsive.",
    response_model=Dict[str, Any],
)
async def get_liveness_probe() -> Dict[str, Any]:
    """Return 200 UP indicating that the service event loop is healthy."""
    return {
        "status": "UP",
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "service": "ai-assistant",
    }


@router.get(
    "/ready",
    summary="Readiness Probe",
    description=(
        "Verifies that internal memory state and tools are ready to handle incoming traffic. "
        "Strictly decoupled: never makes synchronous upstream network calls."
    ),
    response_model=Dict[str, Any],
)
async def get_readiness_probe() -> Dict[str, Any]:
    """Return 200 READY indicating cache sanity and tool readiness."""
    settings = get_settings()
    cache = get_session_cache()
    stats = await cache.get_stats()
    tools = get_registered_tools()

    return {
        "status": "READY",
        "engine": "mock" if settings.is_mock_mode else "live",
        "active_sessions": stats["active_sessions"],
        "registered_tools": len(tools),
        "timestamp": datetime.now(timezone.utc).isoformat(),
    }


@router.get(
    "/metrics",
    summary="Operational Telemetry Metrics",
    description="Exposes basic runtime metrics for health audits and capacity dashboards.",
    response_model=Dict[str, Any],
)
async def get_metrics() -> Dict[str, Any]:
    """Return operational capacity, uptime, and cache statistics."""
    settings = get_settings()
    cache = get_session_cache()
    stats = await cache.get_stats()
    tools = get_registered_tools()
    uptime_seconds = max(0, int(time.time() - _START_TIME))

    return {
        "service": "ai-assistant",
        "version": __version__,
        "uptime_seconds": uptime_seconds,
        "active_sessions": stats["active_sessions"],
        "max_capacity": stats["max_capacity"],
        "registered_tools": len(tools),
        "mock_mode": settings.is_mock_mode,
        "timestamp": datetime.now(timezone.utc).isoformat(),
    }
