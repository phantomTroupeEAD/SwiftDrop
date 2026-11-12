"""FastAPI API route handlers."""

from src.api.routers.chat import router as chat_router
from src.api.routers.health import router as health_router

__all__ = ["chat_router", "health_router"]
