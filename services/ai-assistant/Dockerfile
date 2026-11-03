# Multi-stage / Hardened Python 3.11 Runtime for SwiftDrop AI Assistant & FastMCP
# Stage 1: Build virtual environment with runtime dependencies
FROM python:3.11-slim AS builder

WORKDIR /build
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

# Install minimal build tooling if binary compilation is needed, then clean apt cache
RUN apt-get update && \
    apt-get install -y --no-install-recommends gcc python3-dev && \
    rm -rf /var/lib/apt/lists/*

# Build isolated virtual environment
RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

COPY requirements.txt .
RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt

# Stage 2: Ultra-lean production runtime (< 150MB, zero compilers, non-root UID 10001)
FROM python:3.11-slim AS runtime

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PATH="/opt/venv/bin:$PATH" \
    PORT=8000 \
    MCP_PORT=8002

# Create non-root system group & user for security compliance (Least Privilege)
RUN groupadd -g 10001 appgroup && \
    useradd -u 10001 -g appgroup -s /bin/bash -m appuser

# Copy virtual environment from builder stage
COPY --from=builder /opt/venv /opt/venv

# Copy application code and scripts with restricted ownership
COPY --chown=appuser:appgroup . .

# Ensure supervisor entrypoint is executable and create writable scratch directory
RUN chmod +x entrypoint.sh scripts/verify_mcp_cli.py && \
    mkdir -p /tmp/scratch && \
    chown -R appuser:appgroup /tmp/scratch

# Switch to unprivileged execution user
USER appuser

# Expose microservice REST port (8000) and FastMCP SSE port (8002)
EXPOSE 8000 8002

# Decoupled container liveness probe (queries local event loop only, zero upstream dependencies)
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8000/health/live')" || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["dual"]
