# Document 8 — Implementation, Deployment & Operations Guide

> **Status:** Draft / Proposed (For Review in Team Meeting 2)  
> **Author:** Member 10 — AI Delivery Assistant (Service & MCP) & Cross-Workflow Quality Coordination  
> **Target Production Locations:** `services/ai-assistant/` & `tests/e2e/`  
> **Container Port:** 8000 | **Ingress Gateway Route:** `/api/v1/ai`

---

## 1. Overview & Operational Scope

This document provides a complete operations, deployment, configuration, and troubleshooting manual for:
1. **The AI Delivery Assistant & FastMCP Service (`services/ai-assistant/`):** Running, configuring, containerizing, and observing the Python microservice locally, in Docker, and on Kubernetes.
2. **The Cross-Workflow Quality & E2E Testing Framework (`tests/e2e/`):** Running, debugging, and capturing live demo video/trace artifacts for automated grading.

---

## Part A: AI Delivery Assistant & FastMCP Service Operations

### A.1 Technology Stack & Versions

- **Runtime:** Python `3.11-slim`
- **Web Framework:** FastAPI `>=0.111.0` with Uvicorn `>=0.30.0`
- **Data Validation & Settings:** Pydantic `>=2.7.0` & Pydantic-Settings `>=2.3.0`
- **LLM Integration:** OpenAI Python SDK `>=1.35.0` (`AsyncOpenAI` for universal compatibility)
- **Model Context Protocol:** FastMCP / `mcp>=1.0.0`
- **HTTP Client:** `httpx>=0.27.0` (async)
- **Rate Limiting:** `slowapi>=0.1.9`

---

### A.2 Environment Variables Inventory (`services/ai-assistant/.env.example`)

| Variable Name | Required? | Default / Safe Example | Description |
| :--- | :---: | :--- | :--- |
| `LLM_BASE_URL` | No | `https://openrouter.ai/api/v1` | Base URL for OpenAI-compatible LLM endpoint (OpenRouter, NVIDIA, OpenAI, or Ollama). |
| `LLM_API_KEY` | **Yes (Prod)**| `sk-or-v1-xxxxxxxxxxxx` | Secret API key for the chosen LLM provider. |
| `LLM_MODEL` | No | `meta-llama/llama-3.1-70b-instruct` | Target LLM model identifier. |
| `TEST_MODE` | No | `false` | When `true` or key unset, enables offline deterministic `MockLLMClient` for CI builds. |
| `CORE_API_URL` | Yes | `http://core-api:8080` | Internal network URL for Core API Modular Monolith. |
| `TRACKING_API_URL`| Yes | `http://tracking-api:8081` | Internal network URL for Tracking Service. |
| `INTERNAL_SERVICE_KEY`| Yes| `dev-internal-secret-key-123` | Shared secret header (`X-Internal-Service-Key`) for policy cache refresh. |
| `AI_RATE_LIMIT_ANONYMOUS_RPM` | No | `10` | Max chat requests per minute per IP for anonymous users. |
| `AI_RATE_LIMIT_AUTHENTICATED_RPM`| No | `30` | Max chat requests per minute per user ID for authenticated JWT users. |
| `AI_SESSION_TTL_MINUTES` | No | `15` | In-memory session context expiration window in minutes. |
| `AI_SESSION_MAX_ACTIVE` | No | `50` | Maximum concurrent active conversational sessions retained in memory. |
| `POLICY_CACHE_TTL_MINUTES` | No | `30` | Background cache expiration time for company policies. |

---

### A.3 Local Setup & Development Instructions

#### 1. Setup Virtual Environment
```bash
cd services/ai-assistant
python -m venv .venv

# On Windows (PowerShell):
.\.venv\Scripts\Activate.ps1
# On Linux/macOS:
source .venv/bin/activate

pip install --upgrade pip
pip install -r requirements.txt
```

#### 2. Configure Environment
```bash
cp .env.example .env
# Edit .env with your LLM_API_KEY (e.g. from OpenRouter or NVIDIA NIM)
# If no key is set, the service defaults to TEST_MODE=true (Offline Mock)
```

#### 3. Run the HTTP REST Server (Local Port 8000)
```bash
uvicorn main:app --host 0.0.0.0 --port 8000 --reload
```
- Interactive Swagger UI: `http://localhost:8000/docs`
- Health Check: `http://localhost:8000/health/live`

#### 4. Run the FastMCP Server (For Cursor / Claude Desktop)
```bash
# Runs tool server over STDIO
python mcp_server.py
```

#### 5. Configuring External AI Clients (Cursor & Claude Desktop)
To enable external AI agents to use SwiftDrop's tools:

- **For Claude Desktop (`claude_desktop_config.json`):**
  Add under `"mcpServers"`:
  ```json
  {
    "mcpServers": {
      "swiftdrop": {
        "command": "python",
        "args": ["E:/Projects/SwiftDrop/services/ai-assistant/mcp_server.py"],
        "env": {
          "CORE_API_URL": "http://localhost:8080",
          "TRACKING_API_URL": "http://localhost:8081"
        }
      }
    }
  }
  ```
- **For Cursor IDE (`.cursor/mcp.json`):**
  Configure the command to run `services/ai-assistant/mcp_server.py`.
- **Testing in Chat:**
  Ask Cursor or Claude:
  - *"Use SwiftDrop tools to estimate fee for sending 3 kg from Western Zone to Central Zone."*
  - *"Track SwiftDrop parcel SD-20261021-04921."*
  - *"Login to SwiftDrop as customer.a@swiftdrop.lk with password Password123!"*

---

### A.4 Docker Containerization & Health Probes

#### Dockerfile (`services/ai-assistant/Dockerfile`):
```dockerfile
# Multi-stage lightweight build
FROM python:3.11-slim AS builder
WORKDIR /app
RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

FROM python:3.11-slim AS runner
WORKDIR /app
COPY --from=builder /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"
ENV PYTHONUNBUFFERED=1
ENV PORT=8000

COPY app/ ./app/
COPY knowledge/ ./knowledge/
COPY mcp_server.py main.py ./

# Security: Run strictly as non-root user 10001:10001 per Agreement A3
USER 10001:10001
EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8000/health/live')" || exit 1

ENTRYPOINT ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
```

#### Docker Compose Integration (`docker-compose.yml` entry for Member 9):
```yaml
ai-assistant:
  build:
    context: ./services/ai-assistant
    dockerfile: Dockerfile
  container_name: swiftdrop-ai-assistant
  restart: unless-stopped
  environment:
    - LLM_BASE_URL=${LLM_BASE_URL:-https://openrouter.ai/api/v1}
    - LLM_API_KEY=${LLM_API_KEY:-}
    - LLM_MODEL=${LLM_MODEL:-meta-llama/llama-3.1-70b-instruct}
    - CORE_API_URL=http://core-api:8080
    - TRACKING_API_URL=http://tracking-api:8081
    - INTERNAL_SERVICE_KEY=${INTERNAL_SERVICE_KEY:-dev-internal-secret-key-123}
  ports:
    - "8000:8000"
  depends_on:
    - core-api
    - tracking-api
  networks:
    - swiftdrop-network
```

---

### A.5 Kubernetes Deployment Specification (Bonus Marks)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-assistant
  namespace: swiftdrop
  labels:
    app: ai-assistant
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ai-assistant
  template:
    metadata:
      labels:
        app: ai-assistant
    spec:
      containers:
        - name: ai-assistant
          image: swiftdrop/ai-assistant:latest
          ports:
            - containerPort: 8000
          envFrom:
            - configMapRef:
                name: ai-assistant-config
            - secretRef:
                name: ai-assistant-secrets
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 250m
              memory: 256Mi
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8000
            initialDelaySeconds: 10
            periodSeconds: 15
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8000
            initialDelaySeconds: 5
            periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: ai-assistant
  namespace: swiftdrop
spec:
  selector:
    app: ai-assistant
  ports:
    - port: 8000
      targetPort: 8000
```

---

## Part B: Cross-Workflow Quality Framework Operations

### B.1 Setup & Installation (`tests/e2e/`)

The automated E2E test suite runs **100% in Python**:

```bash
cd tests/e2e
python -m venv .venv-test

# On Windows:
.\.venv-test\Scripts\Activate.ps1
# On Linux/macOS:
source .venv-test/bin/activate

pip install -r requirements-test.txt

# Install Playwright browser binaries (Chromium):
playwright install chromium
```

---

### B.2 Execution Commands for Live Demo & Verification

```bash
# 1. Run all tests headlessly (Fast CI verification):
pytest tests/e2e/ -v

# 2. Run browser UI tests with visible visual browser (For live evaluation demo):
pytest tests/e2e/ui/ -v --headed

# 3. Run browser tests with automatic video recording:
pytest tests/e2e/ui/ -v --video=on --output=tests/e2e/artifacts/videos/

# 4. Run browser tests with interactive Playwright Traces:
pytest tests/e2e/ui/ -v --tracing=on --output=tests/e2e/artifacts/traces/

# 5. Open and inspect Playwright trace artifact:
playwright show-trace tests/e2e/artifacts/traces/trace.zip

# 6. Run only headless RabbitMQ and Merchant Webhook tests:
pytest tests/e2e/api/ -v
```

---

## 2. Operational Troubleshooting & Runbook

| Symptom / Error | Root Cause | Diagnosis & Resolution Action |
| :--- | :--- | :--- |
| **`503 Service Unavailable` on `/api/v1/ai/chat`** | External LLM provider unreachable or invalid `LLM_API_KEY`. | 1. Check container logs: `docker logs swiftdrop-ai-assistant`.<br>2. Verify `LLM_API_KEY` balance.<br>3. Set `TEST_MODE=true` to engage offline mock for testing. |
| **`429 Too Many Requests` on Chat** | User or IP exceeded rate limits. | 1. Normal security behavior.<br>2. If tuning for demo: Increase `AI_RATE_LIMIT_ANONYMOUS_RPM=60` in `.env` and restart service. |
| **AI outputs stale policy information** | Company policy was updated in DB but cache not refreshed. | Manually trigger cache invalidation webhook:<br>`curl -X POST http://localhost:8000/api/v1/ai/admin/refresh-knowledge -H "X-Internal-Service-Key: dev-internal-secret-key-123"` |
| **Playwright test fails with `TimeoutError: locator.click`** | Frontend element is missing the required `data-testid` attribute. | 1. Open the failure screenshot in `tests/e2e/artifacts/`.<br>2. Check `team-integration-and-handoff-guide.md` to ensure Members 6 & 7 attached the matching `data-testid`. |
| **Merchant webhook test fails with HMAC mismatch** | Webhook signature secret mismatch. | Verify merchant secret in `seeds/dev_seed.sql` matches `tests/e2e/fixtures/test_users.py`. |

---

## 3. Known Limitations & Future Architecture Roadmap

1. **Stateless Multi-Pod Context:**
   - Conversation sessions reside in memory per pod.
   - For high-availability multi-pod deployments without sticky sessions, the React client transmits the last 2 conversation turns in the request body, enabling any pod to resume context seamlessly.
2. **Read-Only Baseline:**
   - The assistant does not autonomously mutate the database or finalize bookings.
   - All bookings require explicit user confirmation through the official booking portal using pre-filled deep links (`NAVIGATE_BOOKING`).
3. **LLM Provider Redundancy:**
   - If the primary provider (e.g. OpenRouter) experiences an outage, operators can hot-swap `LLM_BASE_URL` to NVIDIA NIM or OpenAI via Kubernetes ConfigMap without recompiling container images.

---

## 4. Review Checklist for Team Meeting 2

- [ ] **Member 9 (DevOps):** Confirm inclusion of `services/ai-assistant` on port `8000` in `docker-compose.yml` and Traefik routing rules.
- [ ] **Member 9 (DevOps):** Confirm `playwright install chromium` step is included in `.github/workflows/ci.yml`.
- [ ] **All Members (1–9):** Ensure services can be spun up cleanly with a single command: `docker compose up -d` before running `pytest tests/e2e/ -v`.
