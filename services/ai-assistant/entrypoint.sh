#!/bin/sh
set -e

# SwiftDrop AI Assistant & FastMCP Process Supervisor
# Supports execution modes:
#   - api:  Run FastAPI REST API only (:8000)
#   - mcp:  Run FastMCP SSE Gateway only (:8002)
#   - dual: Run both supervised in a single container (:8000 & :8002)
#   - *:    Passthrough execution for diagnostics and CLI tools

MODE="${1:-dual}"
PORT="${PORT:-8000}"
MCP_PORT="${MCP_PORT:-8002}"

echo "[SwiftDrop] Initializing AI Assistant Container..."
echo "[SwiftDrop] Target Mode: $MODE | REST Port: $PORT | FastMCP Port: $MCP_PORT"

case "$MODE" in
  api)
    echo "[SwiftDrop] Starting FastAPI REST & Chat Streaming service on :$PORT..."
    exec uvicorn src.main:app --host 0.0.0.0 --port "$PORT"
    ;;

  mcp)
    echo "[SwiftDrop] Starting FastMCP Remote SSE Gateway on :$MCP_PORT..."
    exec python src/mcp/server.py --transport sse --host 0.0.0.0 --port "$MCP_PORT"
    ;;

  dual)
    echo "[SwiftDrop] Spawning supervised dual service (REST :$PORT + FastMCP SSE :$MCP_PORT)..."

    # Forward termination signals cleanly for Kubernetes graceful shutdown (ADR-002: 20s grace)
    term_handler() {
      echo "[SwiftDrop] SIGTERM / SIGINT received. Initiating graceful shutdown of child processes..."
      kill -TERM "$PID_API" "$PID_MCP" 2>/dev/null || true
      wait "$PID_API" 2>/dev/null || true
      wait "$PID_MCP" 2>/dev/null || true
      echo "[SwiftDrop] All child processes terminated cleanly."
      exit 0
    }
    trap term_handler TERM INT QUIT

    # Start FastAPI REST API in background
    uvicorn src.main:app --host 0.0.0.0 --port "$PORT" &
    PID_API=$!

    # Start FastMCP SSE Server in background
    python src/mcp/server.py --transport sse --host 0.0.0.0 --port "$MCP_PORT" &
    PID_MCP=$!

    echo "[SwiftDrop] Dual processes active: REST PID=$PID_API, FastMCP PID=$PID_MCP"

    # Wait for either process to terminate; if one crashes, shut down the other and exit
    wait -n "$PID_API" "$PID_MCP"
    EXIT_STATUS=$?

    echo "[SwiftDrop] A supervised process exited with status $EXIT_STATUS. Shutting down companion process..."
    kill -TERM "$PID_API" "$PID_MCP" 2>/dev/null || true
    wait "$PID_API" 2>/dev/null || true
    wait "$PID_MCP" 2>/dev/null || true
    exit "$EXIT_STATUS"
    ;;

  *)
    # Custom command passthrough (e.g. pytest, python scripts/verify_mcp_cli.py)
    exec "$@"
    ;;
esac
