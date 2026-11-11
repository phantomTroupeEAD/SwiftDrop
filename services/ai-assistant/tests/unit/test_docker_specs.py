"""Unit test suite verifying Dockerfile multi-stage specs, security hardening, and zero-database invariant."""

from pathlib import Path
import re
import pytest

DOCKERFILE_PATH = Path(__file__).resolve().parent.parent.parent / "Dockerfile"
REQUIREMENTS_PATH = Path(__file__).resolve().parent.parent.parent / "requirements.txt"


@pytest.fixture
def dockerfile_content() -> str:
    assert DOCKERFILE_PATH.exists(), f"Dockerfile not found at {DOCKERFILE_PATH}"
    with open(DOCKERFILE_PATH, "r", encoding="utf-8") as f:
        return f.read()


@pytest.fixture
def requirements_content() -> str:
    assert REQUIREMENTS_PATH.exists(), f"requirements.txt not found at {REQUIREMENTS_PATH}"
    with open(REQUIREMENTS_PATH, "r", encoding="utf-8") as f:
        return f.read()


def test_dockerfile_multistage_structure(dockerfile_content: str):
    """TC-DOC-001: Verify multi-stage build contains both builder and runtime stages."""
    assert "FROM python:3.11-slim AS builder" in dockerfile_content
    assert "FROM python:3.11-slim AS runtime" in dockerfile_content
    # Runtime stage copies virtual environment from builder stage
    assert "COPY --from=builder /opt/venv /opt/venv" in dockerfile_content


def test_dockerfile_non_root_security(dockerfile_content: str):
    """TC-DOC-002: Verify non-root user (UID 10001) creation and execution."""
    assert "groupadd -g 10001" in dockerfile_content
    assert "useradd -u 10001" in dockerfile_content
    assert "USER appuser" in dockerfile_content


def test_dockerfile_dual_ports_exposed(dockerfile_content: str):
    """TC-DOC-003: Verify both REST (8000) and FastMCP SSE (8002) ports are exposed."""
    match = re.search(r"EXPOSE\s+.*8000.*8002", dockerfile_content)
    assert match is not None, "Dockerfile must expose both port 8000 and 8002"


def test_dockerfile_decoupled_healthcheck(dockerfile_content: str):
    """TC-DOC-004: Verify HEALTHCHECK queries /health/live without requiring curl."""
    assert "HEALTHCHECK" in dockerfile_content
    assert "/health/live" in dockerfile_content
    # Ensure it uses python urllib rather than external binaries
    assert "urllib.request" in dockerfile_content


def test_dockerfile_entrypoint_supervisor(dockerfile_content: str):
    """Verify ENTRYPOINT uses entrypoint.sh supervisor with dual mode as default CMD."""
    assert 'ENTRYPOINT ["/app/entrypoint.sh"]' in dockerfile_content
    assert 'CMD ["dual"]' in dockerfile_content


def test_zero_database_drivers_in_requirements(requirements_content: str):
    """TC-DOC-006: Invariant verification: Zero direct SQL/database drivers in requirements.txt."""
    prohibited_db_packages = [
        "psycopg2",
        "psycopg",
        "asyncpg",
        "sqlalchemy",
        "databases",
        "tortoise-orm",
        "peewee",
        "pymysql",
        "mysqlclient",
        "aiomysql",
        "aiosqlite",
    ]
    for pkg in prohibited_db_packages:
        pattern = rf"^\s*{re.escape(pkg)}(\[.*\])?([=<>!~].*)?$"
        match = re.search(pattern, requirements_content, re.MULTILINE | re.IGNORECASE)
        assert match is None, f"Strict Invariant Violation: Prohibited database driver '{pkg}' found in requirements.txt"
