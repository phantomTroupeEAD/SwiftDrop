# Multi-stage / Optimized Python 3.11 Runtime for SwiftDrop AI Assistant
FROM python:3.11-slim AS runtime

# Set environment variables for Python performance & non-buffering
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PORT=8000

# Create non-root system group & user for security compliance (Least Privilege)
RUN groupadd -g 10001 appgroup && \
    useradd -u 10001 -g appgroup -s /bin/bash -m appuser

WORKDIR /app

# Install runtime dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy source code and application assets
COPY --chown=appuser:appgroup . .

# Switch to unprivileged execution user
USER appuser

# Expose microservice HTTP port
EXPOSE 8000

# Container liveness probe
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8000/health/live')" || exit 1

# Start FastAPI ASGI server with Uvicorn
CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000"]
