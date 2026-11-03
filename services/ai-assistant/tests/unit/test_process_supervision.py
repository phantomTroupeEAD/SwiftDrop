"""Unit test suite verifying entrypoint process supervision script structure, modes, and signal trapping."""

from pathlib import Path
import re
import pytest

ENTRYPOINT_PATH = Path(__file__).resolve().parent.parent.parent / "entrypoint.sh"


@pytest.fixture
def entrypoint_content() -> str:
    assert ENTRYPOINT_PATH.exists(), f"entrypoint.sh not found at {ENTRYPOINT_PATH}"
    with open(ENTRYPOINT_PATH, "r", encoding="utf-8") as f:
        return f.read()


def test_entrypoint_shebang_and_mode_handling(entrypoint_content: str):
    """TC-SUP-001: Verify entrypoint starts with POSIX shebang and supports all three operational modes."""
    assert entrypoint_content.startswith("#!/bin/sh")
    assert 'MODE="${1:-dual}"' in entrypoint_content
    assert "api)" in entrypoint_content
    assert "mcp)" in entrypoint_content
    assert "dual)" in entrypoint_content


def test_entrypoint_api_mode_command(entrypoint_content: str):
    """Verify 'api' mode launches Uvicorn on configured PORT."""
    assert 'exec uvicorn src.main:app --host 0.0.0.0 --port "$PORT"' in entrypoint_content


def test_entrypoint_mcp_mode_command(entrypoint_content: str):
    """Verify 'mcp' mode launches FastMCP SSE server on configured MCP_PORT."""
    assert 'exec python src/mcp/server.py --transport sse --host 0.0.0.0 --port "$MCP_PORT"' in entrypoint_content


def test_entrypoint_dual_mode_supervision(entrypoint_content: str):
    """Verify 'dual' mode spawns both processes concurrently in the background."""
    assert 'uvicorn src.main:app --host 0.0.0.0 --port "$PORT" &' in entrypoint_content
    assert 'PID_API=$!' in entrypoint_content
    assert 'python src/mcp/server.py --transport sse --host 0.0.0.0 --port "$MCP_PORT" &' in entrypoint_content
    assert 'PID_MCP=$!' in entrypoint_content


def test_entrypoint_sigterm_trapping(entrypoint_content: str):
    """TC-SUP-002: Verify trap handler catches TERM, INT, QUIT and terminates both child PIDs."""
    assert "term_handler()" in entrypoint_content
    assert 'kill -TERM "$PID_API" "$PID_MCP"' in entrypoint_content
    assert "trap term_handler TERM INT QUIT" in entrypoint_content


def test_entrypoint_failure_propagation(entrypoint_content: str):
    """TC-SUP-003: Verify supervisor monitors both processes and exits if one terminates."""
    assert 'wait -n "$PID_API" "$PID_MCP"' in entrypoint_content
    assert 'exit "$EXIT_STATUS"' in entrypoint_content
