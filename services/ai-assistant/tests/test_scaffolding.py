"""Verification smoke tests for Phase 1 Scaffolding."""

from pathlib import Path
import re
import pytest

def test_package_structure():
    """Verify that all core modules exist and have valid package initializers."""
    base_dir = Path(__file__).resolve().parent.parent
    packages = [
        "src",
        "src/api",
        "src/api/middleware",
        "src/api/routers",
        "src/core",
        "src/domain",
        "src/llm",
        "src/mcp",
        "src/tools",
        "tests",
    ]
    for pkg in packages:
        init_file = base_dir / pkg / "__init__.py"
        assert init_file.exists(), f"Missing __init__.py in package: {pkg}"

def test_zero_database_dependency_invariant():
    """Enforce strict invariant: zero direct database packages in requirements.txt."""
    req_file = Path(__file__).resolve().parent.parent / "requirements.txt"
    assert req_file.exists(), "requirements.txt does not exist"
    
    content = req_file.read_text(encoding="utf-8").lower()
    forbidden_db_drivers = [
        "psycopg2",
        "asyncpg",
        "sqlalchemy",
        "tortoise-orm",
        "peewee",
        "databases",
        "aiomysql",
        "pymysql",
    ]
    for forbidden in forbidden_db_drivers:
        assert forbidden not in content, (
            f"Strict Zero-DB invariant violated: '{forbidden}' found in requirements.txt. "
            "AI Assistant must access data exclusively via upstream REST APIs."
        )

def test_env_example_has_mandatory_keys():
    """Verify that .env.example defines all mandatory configuration variables."""
    env_file = Path(__file__).resolve().parent.parent / ".env.example"
    assert env_file.exists(), ".env.example does not exist"

    content = env_file.read_text(encoding="utf-8")
    mandatory_keys = [
        "PORT",
        "LLM_BASE_URL",
        "LLM_API_KEY",
        "LLM_MODEL",
        "MOCK_LLM_MODE",
        "CORE_API_URL",
        "TRACKING_API_URL",
        "AI_SERVICE_ROLE_TOKEN",
        "AI_RATE_LIMIT_RPM",
        "AI_SESSION_TTL_MINUTES",
        "AI_SESSION_MAX_ACTIVE",
    ]
    for key in mandatory_keys:
        pattern = rf"^{key}="
        assert re.search(pattern, content, re.MULTILINE), (
            f"Mandatory configuration key '{key}' missing from .env.example"
        )

def test_dockerfile_security_invariants():
    """Verify that Dockerfile specifies non-root execution and healthcheck."""
    dockerfile = Path(__file__).resolve().parent.parent / "Dockerfile"
    assert dockerfile.exists(), "Dockerfile does not exist"

    content = dockerfile.read_text(encoding="utf-8")
    assert "USER appuser" in content, "Dockerfile must switch to unprivileged USER appuser"
    assert "HEALTHCHECK" in content, "Dockerfile must define a container HEALTHCHECK probe"
    assert "EXPOSE 8000" in content, "Dockerfile must expose assigned port 8000"
