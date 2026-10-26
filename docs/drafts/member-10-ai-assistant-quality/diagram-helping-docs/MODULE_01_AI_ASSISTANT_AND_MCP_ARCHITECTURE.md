# Module 01 — AI Delivery Assistant & FastMCP Architecture Specification

> **Module Name:** AI Delivery Assistant & Model Context Protocol (FastMCP) Service  
> **Repository Location:** `services/ai-assistant/`  
> **Assigned Container Port:** `8000` (Reverse proxied at `/api/v1/ai` via Traefik Gateway)  
> **Target Folder:** `docs/drafts/member-10-ai-assistant-quality/diagram-helping-docs/`  
> **Purpose:** Authoritative architectural specification designed to serve as the direct blueprint for drawing the **Module 01 Component & Deployment Architecture Diagram**. Contains exhaustive technical details on component boundaries, clean layered structure, FastMCP dual-channel communication, memory lifecycle, and security guardrails without containing visual diagrams.  
> **Specification Version:** `2.0.0` (Authoritative Baseline)  
> **Evaluation Scope:** **+5 Bonus Marks** (AI Assistant, Generative UI Action Directives & FastMCP)

---

## 1. System Identity, Container Packaging & Ingress Routing

### 1.1 Microservice Deployment Topology
- **Service Name:** `ai-assistant`
- **Container Listening Port:** Internal port `8000` (`0.0.0.0:8000`).
- **Edge Routing & Ingress:** Traefik API Gateway (`http://traefik:80`) proxies external traffic using rule `PathPrefix('/api/v1/ai')` directly to `http://ai-assistant:8000`. The service handles `/api/v1/ai/*` natively without path stripping.
- **Edge Security & Rate Limiting:** Traefik enforces an edge rate limit of 20 requests per minute (burst 30) per client IP address before traffic reaches the service container.
- **Container Base Image & Security Context:**
  - Base Image: `python:3.11-slim`.
  - Non-Root Execution: Runs strictly under dedicated unprivileged system user `USER 10001:10001` (`swiftdrop:swiftdrop`).
  - Packaging: Multi-stage Docker build separating dependency compilation in builder stage (`/opt/venv`) from runtime execution stage.
  - Filesystem Permissions: Read-only root filesystem with ephemeral memory storage.
- **Kubernetes Pod Specs:** Configured for 100m CPU request (250m limit) and 128Mi RAM request (256Mi limit).

---

## 2. Clean Layered Component Architecture

The service internal architecture is organized strictly into 5 decoupled clean architectural layers:

```
services/ai-assistant/
├── src/
│   ├── api/          # 1. Transport Layer (Routers, Middleware, Ingress Handlers)
│   ├── domain/       # 2. Domain & Application Layer (Pydantic Entities, Errors)
│   ├── llm/          # 3. LLM Abstraction Layer (OpenAI Live + Deterministic Mock)
│   ├── tools/        # 4. Sandboxed Tool Registry (4 Approved Read-Only Tools)
│   └── core/         # 5. Ephemeral State & Cache Layer (RAM LRU, Policy Hydrator)
├── knowledge/        # Baseline Qualitative Knowledge (company_policies.md)
└── mcp_server.py     # Standalone FastMCP Server Entrypoint (STDIO & SSE)
```

### 2.1 Transport Layer (`src/api/`)
Handles HTTP protocol serialization, security headers, rate limiting, and request lifecycle:
1. **Chat Router (`routers/chat.py`):**
   - Exposes `POST /api/v1/ai/chat`.
   - Validates inbound JSON payloads via Pydantic; dispatches prompt processing to the domain orchestrator.
2. **Operational Health Router (`routers/health.py`):**
   - Exposes `/health/live` (process event loop responsiveness) and `/health/ready` (local cache and model configuration status).
   - Exposes `/metrics` for Prometheus scraping (request counters, latency histograms).
3. **Administration Router (`routers/admin.py`):**
   - Exposes `POST /api/v1/ai/admin/refresh-knowledge` to receive internal policy invalidation webhooks.
4. **Middleware Stack (`middleware/`):**
   - *Auth Context Extractor:* Extracts and parses `Authorization: Bearer <jwt>`, populating user ID, tenant ID, and role without verifying signatures locally (Core API handles verification).
   - *Input Hygiene Guard:* Rejects payloads $>1,000$ characters with `400 Bad Request` (`MESSAGE_EXCEEDS_LENGTH_LIMIT`) to prevent buffer overflow and token stuffing.
   - *SlowAPI Token Bucket Limiter:* Enforces an application-level limit of 20 requests per minute per IP address.

### 2.2 Domain & Application Layer (`src/domain/`)
Encapsulates domain entities, response contracts, and structured generative UI actions:
1. **Request & Response Envelopes (`chat.py`):**
   - `ChatMessageRequest`: Validates user message text (1 to 1,000 chars), optional UUIDv4 `conversation_id`, and locale (`en-US`).
   - `ChatMessageResponse`: Encapsulates conversation UUID, natural language grounded answer, tool execution metadata list, and suggested action cards.
2. **Generative UI Action Directives (`ActionPayload`):**
   - Synthesizes structured client-side navigation actions (`NAVIGATE_BOOKING` and `VIEW_TRACKING`).
   - Packages pre-filled URL query strings pointing to `/portal/book` (aliased to `/customer/book`) and payload dictionaries containing destination zone, weight, and calculated fee cents.
3. **Custom Domain Exceptions (`errors.py`):**
   - Standardized error envelopes with machine-readable error codes (`VALIDATION_FAILED`, `RATE_LIMIT_EXCEEDED`, `UPSTREAM_UNAVAILABLE`, `LLM_PROVIDER_DOWN`).

### 2.3 LLM Abstraction Layer (`src/llm/`)
Decouples prompt engineering and tool execution from specific AI vendors:
1. **Abstract Base Interface (`base.py`):**
   - Defines `LLMProvider` protocol requiring `generate_response(user_message, session_context, tools)`.
2. **Universal AsyncOpenAI Adapter (`openai_adapter.py`):**
   - Leverages `AsyncOpenAI` SDK configured with `LLM_BASE_URL`, `LLM_API_KEY`, and `LLM_MODEL`.
   - Connects to any OpenAI-compatible provider (OpenRouter, NVIDIA NIM, OpenAI, or Ollama).
   - Enforces low temperature (`0.2`) to guarantee deterministic function calling and eliminate creative hallucinations.
   - Formats registered tool JSON schemas; receives structured tool invocation arguments from model output.
3. **Deterministic Offline Mock Adapter (`mock_adapter.py`):**
   - Automatically engaged when `TEST_MODE=true` or when `LLM_API_KEY` is omitted.
   - Regex and keyword matcher simulating natural language understanding with zero external network connectivity and \$0 API cost:
     - Regex `^SD-\d{8}-[A-Za-z0-9]+$` maps to `track_parcel_status`.
     - Keywords `fee`, `cost`, `how much` map to `estimate_shipping_fee`.
     - Keyword `where is my package` maps to `get_my_active_parcels`.
   - Dispatches real tool calls against Core and Tracking APIs and returns formatted answers in $<150\text{ms}$.

### 2.4 Sandboxed Read-Only Tool Registry (`src/tools/`)
Enforces the least-privilege tool allow-list containing **exactly 4 read-only tools**:
1. **`estimate_shipping_fee` (`pricing_tool.py`):**
   - Validates dimensions ($length, width, height > 0$) and weight ($0.1 \le w \le 30.0\text{ kg}$).
   - Dispatches HTTP POST to Core API: `http://core-api:8080/api/v1/pricing/quotes`.
   - Returns integer cents fee breakdown (`fee_cents`, `base_fee_cents`, `weight_surcharge_cents`).
2. **`track_parcel_status` (`tracking_tool.py`):**
   - Validates tracking code regex `^SD-\d{8}-[A-Za-z0-9]+$`.
   - Dispatches HTTP GET to Tracking API: `http://tracking-api:8081/api/v1/tracking/public/{trackingNumber}`.
   - Returns milestone timeline and sanitized destination city/district.
3. **`get_my_active_parcels` (`parcels_tool.py`):**
   - Forwards caller's Bearer JWT to Core API: `http://core-api:8080/api/v1/parcels`.
   - Returns array of active non-delivered shipments owned by the authenticated caller.
4. **`explain_company_policies` (`policy_tool.py`):**
   - Queries `PolicyContextHydrator` for grounded dangerous goods, packaging, and sizing rules.

### 2.5 Ephemeral State & Cache Layer (`src/core/`)
Maintains process-level ephemeral data without database storage:
1. **In-Memory LRU Session Cache (`cache.py`):**
   - Implemented via thread-safe `cachetools.TTLCache`.
   - Capacity: Max 50 active sessions in RAM per container replica.
   - Expiration: Strict 15-minute time-to-live (`AI_SESSION_TTL_MINUTES=15`).
   - Turn Cap: Sliding window retaining at most 5 alternating turns (10 total messages) per session to prevent token accumulation.
2. **Policy Context Hydrator (`PolicyContextHydrator`):**
   - Ingests qualitative markdown from `knowledge/company_policies.md` at startup.
   - Queries live operational thresholds from Core API `GET /api/v1/admin/settings` (cached 5 minutes).
   - Enforces the **Dynamic Live Precedence Invariant**: live Core API numbers override static markdown text.

---

## 3. FastMCP Protocol Architecture (STDIO & SSE Transports)

The service exposes its tool registry via the official Model Context Protocol (FastMCP) through two distinct transport mechanisms:

### 3.1 Local STDIO Transport (Desktop Client Integration)
- **Primary Consumers:** Claude Desktop application and Cursor IDE.
- **Execution Mechanism:** Host desktop spawns `python services/ai-assistant/mcp_server.py` as a local sub-process.
- **Communication Protocol:** JSON-RPC 2.0 frames transmitted over standard input (`stdin`) and standard output (`stdout`).
- **Tool Advertisement:** FastMCP advertises `tools/list` on handshake; executes tool invocations synchronously in child processes.

### 3.2 HTTP SSE Dual-Channel Transport (Remote Client Integration)
- **Primary Consumers:** Remote AI agents, evaluation harnesses, and web-based LLMs.
- **Why WebSockets Were Avoided:** WebSockets require protocol upgrade headers (`Upgrade: websocket`) that cause connection timeouts behind standard enterprise proxies and Traefik load balancers.
- **Channel 1 (Persistent Downstream SSE Stream):**
  - Client initiates connection: `GET /api/v1/ai/mcp` with `Accept: text/event-stream`.
  - FastMCP server holds the HTTP connection open and pushes an `endpoint` event containing a unique session identifier:
    `event: endpoint\ndata: /api/v1/ai/mcp/messages?session_id=<uuid>\n\n`
- **Channel 2 (Upstream HTTP POST Messages):**
  - Client sends JSON-RPC 2.0 tool requests via standard HTTP `POST` to that session endpoint (`/api/v1/ai/mcp/messages?session_id=<uuid>`).
- **Execution & Stream Return:**
  - FastMCP receives the upstream POST, validates parameters via Pydantic, queries backend services over internal network, packages the response into a JSON-RPC envelope, and pushes it back downstream through the persistent Channel 1 SSE stream.

---

## 4. Security, Guardrails & Anti-Abuse Architecture

1. **Zero Mutation Tools (Write Defense):**
   - The tool registry contains **zero write, update, or delete tools**. Even under successful prompt injection, the model has no programmatic ability to modify parcel states, balances, or courier assignments.
2. **Untrusted Data Delimiter Wrapping:**
   - All dynamic text retrieved from backend APIs (milestone descriptions, city names, delivery notes) is wrapped inside `<retrieved_data>...</retrieved_data>` tags.
   - System prompts instruct the LLM that content inside delimiters must be treated strictly as passive text data and never evaluated as executable instructions.
3. **Secret Isolation Principle:**
   - Zero credentials, connection strings, JWT signing keys, or internal tokens are placed inside system prompts.
4. **Customer PII Stripping (`SEC-008`):**
   - Public tracking responses exclude recipient street addresses and telephone numbers at the source API level. The AI service receives and presents only city/district names.
5. **Least Privilege for MCP API Keys:**
   - External MCP integrations authenticate via merchant API keys (`X-API-Key`). Core API validates that keys carry **`parcels:read` scope ONLY**. Any booking attempt returns `403 Forbidden (MERCHANT_SCOPE_DENIED)`.

---

## 5. Operational Probes & Failure Recovery Architecture

1. **Decoupled Local Health Probes:**
   - `/health/live`: Verifies FastAPI event loop. Returns `200 OK {"status": "alive"}`.
   - `/health/ready`: Verifies local memory cache and model configuration only.
   - **Cascading Failure Protection:** `/health/ready` **never pings Core API or Tracking API**. If an upstream dependency restarts, the AI Assistant remains marked `Ready` in Kubernetes endpoints.
2. **Graceful Dependency Degradation:**
   - *Core Pricing API Down:* Assistant returns polite maintenance message; **zero fee guessing**.
   - *Tracking API Down:* Assistant directs user to public tracking web page; **zero status fabrication**.
   - *External LLM Provider Down:* Circuit breaker trips after 10-second timeout; returns HTTP `503 Service Unavailable`.

---

## 6. Downstream Inter-Service Communication Contracts

| Target Subsystem | Endpoint Path | Method | Auth / Headers | Ingestion Purpose |
| :--- | :--- | :---: | :--- | :--- |
| **Core API Pricing** | `/api/v1/pricing/quotes` | `POST` | `Content-Type: application/json` | Authoritative tariff calculation in integer cents. |
| **Core API Parcels** | `/api/v1/parcels` | `GET` | `Authorization: Bearer <jwt>` | Customer active parcel history lookup. |
| **Core API Admin** | `/api/v1/admin/settings` | `GET` | Internal Network | Live system thresholds (max weight, COD limits). |
| **Tracking API** | `/api/v1/tracking/public/{id}`| `GET` | Public / None | Sanitized milestone history and status. |

---

## 7. Diagram 1 Visual Layout & Construction Guide

When drawing the **Module 01 Component Architecture Diagram**, structure the visual layout as follows:

1. **Top Ingress Layer:**
   - Box for `Clients` (Browser React Chat Drawer, Claude Desktop / Cursor IDE).
   - Box for `Traefik API Gateway (:80)` showing route `PathPrefix('/api/v1/ai')` and rate limit (20 RPM).
2. **Central Microservice Boundary (`ai-assistant:8000` Container):**
   - Draw an outer container boundary labeled `USER 10001:10001 (Non-Root)`.
   - Inside, place 5 stacked clean architectural layers:
     - *Layer 1 (Transport):* `ChatRouter`, `HealthRouter` (`/health/live`, `/health/ready`), `SlowAPI RateLimiter`, `InputHygieneGuard`.
     - *Layer 2 (Domain):* `Pydantic Schemas`, `Generative UI Action Synthesizer` (`NAVIGATE_BOOKING`).
     - *Layer 3 (LLM Engine):* `LLMProvider Interface`, with dual branches to `OpenAIAdapter` (Live) and `MockLLMAdapter` (Offline CI).
     - *Layer 4 (Tool Sandbox):* Tool Registry with 4 read-only tools (`estimate_shipping_fee`, `track_parcel_status`, `get_my_active_parcels`, `explain_company_policies`).
     - *Layer 5 (State & Cache):* `TTLCache LRU Session Store` (RAM Only, 15m TTL) and `PolicyContextHydrator` with baseline `company_policies.md`.
   - Side Box: `FastMCP Server` showing STDIO pipe and HTTP SSE Dual-Channel connections.
3. **Bottom Downstream REST Layer:**
   - Box for `Core API (:8080)` (Pricing Quotes, Parcels, Admin Settings).
   - Box for `Tracking API (:8081)` (Sanitized Public Tracking).
   - Explicitly highlight **ZERO DATABASE CONNECTION** (no arrows pointing to PostgreSQL).

---
*End of Module 01 Architecture Specification.*
