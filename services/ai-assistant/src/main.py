"""SwiftDrop AI Delivery Assistant & FastMCP Service — Main FastAPI Application Entrypoint."""

from contextlib import asynccontextmanager
import logging
from typing import AsyncGenerator
from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from src.api.routers import chat_router, health_router
from src.core.config import get_settings
from src.domain.errors import AppException, ErrorDetail
from src.tools.registry import get_registered_tools

# Configure structured logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("swiftdrop.ai_assistant")


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Manage application startup, engine pre-warming, and graceful shutdown."""
    settings = get_settings()
    tools = get_registered_tools()
    engine_name = "Deterministic Mock (CI Mode)" if settings.is_mock_mode else f"Live LLM ({settings.LLM_MODEL})"

    logger.info("Starting SwiftDrop AI Assistant Service v2.5.1...")
    logger.info("Active Engine: %s", engine_name)
    logger.info("Registered Tools (%d): %s", len(tools), ", ".join(tools.keys()))
    logger.info("Interactive API Docs available at /docs and /redoc")

    yield

    logger.info("Shutting down SwiftDrop AI Assistant Service...")


# Initialize FastAPI application
app = FastAPI(
    title="SwiftDrop AI Delivery Assistant & FastMCP Service",
    description=(
        "Production AI Delivery Assistant providing grounded parcel tracking, "
        "instant shipping quotations, active parcel summaries, and standardized FastMCP tools."
    ),
    version="2.5.1",
    docs_url="/docs",
    redoc_url="/redoc",
    openapi_url="/openapi.json",
    lifespan=lifespan,
)

# Configure CORS Middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# Global Exception Handler 1: Domain / Application Exceptions
@app.exception_handler(AppException)
async def domain_app_exception_handler(request: Request, exc: AppException) -> JSONResponse:
    """Format known domain exceptions into standard SwiftDrop RFC-7807 error envelopes."""
    return JSONResponse(
        status_code=exc.status_code,
        content=exc.to_dict(),
    )


# Global Exception Handler 2: Request Payload Validation Failures
@app.exception_handler(RequestValidationError)
async def request_validation_exception_handler(
    request: Request,
    exc: RequestValidationError,
) -> JSONResponse:
    """Format Pydantic schema validation failures into clean field-level RFC-7807 details."""
    details = []
    for err in exc.errors():
        loc_parts = [str(x) for x in err.get("loc", []) if x != "body"]
        field_name = ".".join(loc_parts) if loc_parts else None
        details.append(
            ErrorDetail(
                field=field_name,
                issue=err.get("msg", "Invalid field format or value"),
            ).model_dump()
        )

    return JSONResponse(
        status_code=422,
        content={
            "error": {
                "code": "VALIDATION_ERROR",
                "message": "Request payload failed semantic domain validation",
                "details": details,
            }
        },
    )


# Global Exception Handler 3: Uncaught Internal Server Exceptions
@app.exception_handler(Exception)
async def unhandled_server_exception_handler(
    request: Request,
    exc: Exception,
) -> JSONResponse:
    """Intercept uncaught server errors, logging the stack trace without leaking secrets or internals to clients."""
    logger.error("Unhandled internal server exception: %s", exc, exc_info=True)
    return JSONResponse(
        status_code=500,
        content={
            "error": {
                "code": "INTERNAL_SERVER_ERROR",
                "message": "An unexpected server error occurred. Please contact support if the issue persists.",
                "details": [],
            }
        },
    )


# Mount Subsystem Routers
app.include_router(health_router, prefix="/health", tags=["Health & Monitoring"])
app.include_router(chat_router, prefix="/api/v1/ai", tags=["Conversational Assistant"])


@app.get("/", summary="Service Information & Quick Links", tags=["Root"])
async def root_greeting() -> dict:
    """Return service identity and quick navigation links."""
    return {
        "service": "SwiftDrop AI Delivery Assistant & FastMCP Service",
        "version": "2.5.1",
        "status": "OPERATIONAL",
        "docs_url": "/docs",
        "redoc_url": "/redoc",
        "health_live_url": "/health/live",
        "health_ready_url": "/health/ready",
        "chat_url": "/api/v1/ai/chat",
    }
