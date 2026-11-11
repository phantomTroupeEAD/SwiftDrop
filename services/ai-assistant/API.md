# AI Assistant & FastMCP API Specifications

> **Service:** `services/ai-assistant`  
> **Assigned Owner:** Member 10 — AI Assistant/MCP & Cross-Workflow Quality  
> **Base Path:** `/api/v1/ai`  
> **Container Port:** `8000`  
> **Specification Version:** `2.5.1` (Canonical Modular Version)

---

## 1. Endpoint Overview & Interactive Documentation

| Method | URI Path | Description | Required Auth / Role |
| :--- | :--- | :--- | :--- |
| `GET` | `/docs` | Interactive Swagger UI (OpenAPI 3.1.0) | None (Developer UI) |
| `GET` | `/redoc` | Interactive ReDoc Documentation | None (Developer UI) |
| `GET` | `/openapi.json` | Canonical OpenAPI Schema Specification | None (Public) |
| `POST` | `/api/v1/ai/chat` | Natural language chat endpoint with automatic tool invocation and grounded responses | Optional (Public for tracking & quotes; Bearer JWT for customer parcel lookup) |
| `POST` | `/api/v1/ai/chat/stream` | Real-time Server-Sent Events (SSE) token and tool event streaming endpoint | Optional (Public for tracking & quotes; Bearer JWT for customer parcel lookup) |
| `GET` | `/health/live` | Kubernetes Liveness Probe: confirms FastAPI event loop responsiveness | None (Public) |
| `GET` | `/health/ready` | Kubernetes Readiness Probe: verifies local LLM config & memory cache sanity | None (Public) |
| `GET` | `/health/metrics` | Prometheus & runtime metrics endpoint for active sessions, uptime, and capacity | None (Public) |

---

## 2. FastMCP & Read-Only Tool Definitions

The AI Assistant service exposes approved read-only tools via the Model Context Protocol (FastMCP) and standard OpenAI function calling (for a dedicated client setup guide covering Antigravity, Claude Desktop, and Cursor, see **[`MCP.md`](./MCP.md)**):

### 2.1 `track_parcel_status`
- **Description:** Retrieve real-time tracking milestones, status description, and destination city for a parcel.
- **Parameters:**
  - `tracking_number` (string, required): Parcel tracking code matching pattern `^SD-\d{8}-\d{5}$` (e.g. `SD-20261102-00001`).
- **Target Upstream Endpoint:** `GET http://tracking-api:8081/api/v1/tracking/public/{tracking_number}`
- **Security & Privacy:** Strictly enforces SEC-009 privacy masking (omits phone numbers, street addresses, driver notes).
- **Permissions:** Public / Anonymous.

### 2.2 `estimate_delivery_fee`
- **Description:** Compute authoritative delivery fee quote, transit hours estimate, and prefilled booking action based on package weight, dimensions, and postal codes.
- **Parameters:**
  - `weight_kg` (float, required): Package weight in kilograms (`0.01 <= weight_kg <= 50.0`).
  - `pickup_postal_code` (string, required): 5-digit origin postal code (e.g. `"00100"`).
  - `delivery_postal_code` (string, required): 5-digit destination postal code (e.g. `"20000"`).
  - `service_type` (string, optional): Delivery tier: `"STANDARD"`, `"EXPRESS"`, or `"SAME_DAY"` (default: `"STANDARD"`).
  - `length_cm` (float, optional): Package length in cm (`0.0 < length_cm <= 300.0`).
  - `width_cm` (float, optional): Package width in cm (`0.0 < width_cm <= 300.0`).
  - `height_cm` (float, optional): Package height in cm (`0.0 < height_cm <= 300.0`).
- **Validation Invariants (VULN-006, SEC-CLD-002, SEC-CLD-004):**
  - Strict 3D dimension cross-validation: if any dimension is provided, all three (`length_cm`, `width_cm`, `height_cm`) must be supplied together. Partial dimensions trigger a `422 Unprocessable Entity` validation error.
  - Minimum gross weight: strictly enforces `weight_kg >= 0.01 kg`.
  - Maximum package girth: strictly enforces `Length + 2 * (Width + Height) <= 300.0 cm`.
  - Maximum volumetric weight: strictly enforces `(Length * Width * Height) / 5000 <= 50.0 kg`.
  - Billable weight calculation: dynamically evaluates `billable_weight = max(weight_kg, volumetric_kg)`.
  - Upstream 422 error sanitization: upstream schema validation errors are sanitized into human-readable messages without leaking internal trace dictionaries.
- **Target Upstream Endpoint:** `POST http://core-api:8080/api/v1/pricing/quotes`
- **Generative UI Action:** Returns `NAVIGATE_BOOKING` card targeting `/portal/book`.
- **Permissions:** Public / Anonymous.

### 2.3 `get_my_active_parcels`
- **Description:** Retrieve active shipments (picked-up, in-transit, out-for-delivery) belonging to the authenticated customer or merchant.
- **Parameters:**
  - `authorization_token` (string, optional): Customer or merchant JWT Bearer token (defaults to `null`). Can be passed explicitly or injected via environment variable `SWIFTDROP_AUTH_TOKEN`.
  - `status` (string, optional): Filter by parcel lifecycle status (e.g. `"IN_TRANSIT"`). Dispatches both `status` and `status_filter` query parameters to upstream Core API to guarantee contract compatibility (SEC-CLD-001).
  - `limit` (integer, optional): Maximum parcels to return (`1 <= limit <= 50`, default: `10`).
- **Authentication Hierarchy (VULN-001):**
  1. `context["authorization"]` (HTTP / SSE gateway header)
  2. `authorization_token` parameter (optional tool argument)
  3. `SWIFTDROP_AUTH_TOKEN` environment variable (ideal for FastMCP clients to prevent token leakage in LLM prompt transcripts)
- **Target Upstream Endpoint:** `GET http://core-api:8080/api/v1/parcels` (forwards caller's `Authorization: Bearer <JWT>`).
- **Security & Isolation:** IDOR-safe; Core API extracts tenant and customer identity from the verified JWT.
- **Permissions:** Authenticated Customer or Merchant.

### 2.4 `explain_company_policies`
- **Description:** Explain courier company policies, prohibited goods, packaging standards, transit SLAs, and dispute rules.
- **Parameters:**
  - `query` (string, required): Inquired policy topic (e.g. `"prohibited items"`, `"packaging limits"`, `"SLA"`).
- **Concurrency & Caching (VULN-005, SEC-CLD-007):** In-memory 30-minute policy cache protected by `asyncio.Lock()` double-checked locking, preventing thundering herds (cache stampedes) on Core API admin settings. When a query cannot be matched to specific section keywords, the tool returns a concise 4-chapter policy summary index (< 800 chars) instead of dumping the full 5KB markdown into LLM context.
- **Target Upstream Endpoint:** `GET http://core-api:8080/api/v1/admin/settings` (with local `company_policies.md` grounding).
- **Permissions:** Public / Anonymous.

### 2.5 `lookup_postal_code`
- **Description:** Lookup authoritative Sri Lankan 5-digit postal code, administrative district, and province for any city, town, suburb, or postal division (e.g. "Walasmulla" → 82450, "Kadugannawa" → 20430, "Mawanella" → 71500).
- **Parameters:**
  - `query` (string, required): City, town, suburb, or area name (1 to 100 characters).
- **Data Source:** Authoritative in-memory Sri Lanka Postal Directory (`src/core/postal_codes.py`) covering all 25 districts and commercial towns.
- **Permissions:** Public / Anonymous.

### 2.6 Authoritative FastMCP Context Resources
- **`swiftdrop://policies`:** Exposes full authoritative markdown shipping guidelines (`company_policies.md`) for direct LLM context window ingestion.
- **`swiftdrop://sla-matrix`:** Exposes structured JSON transit SLAs (Express 24h, Standard 48–72h, Outstation 3–5 days).

### 2.7 Antigravity MCP Client Configuration

#### Mode A: Local STDIO Subprocess (Desktop / Local Dev)
Configure `~/.gemini/config/mcp_config.json`:
```json
{
  "mcpServers": {
    "swiftdrop-local": {
      "command": "E:\Projects\SwiftDrop\services\ai-assistant\.venv\Scripts\python.exe",
      "args": [
        "E:\Projects\SwiftDrop\services\ai-assistant\src\mcp\server.py"
      ],
      "env": {
        "PYTHONPATH": "E:\Projects\SwiftDrop\services\ai-assistant",
        "CORE_API_URL": "http://localhost:8080",
        "TRACKING_API_URL": "http://localhost:8081",
        "MOCK_LLM_MODE": "true",
        "USE_MOCK_UPSTREAM": "true",
        "SWIFTDROP_API_KEY": "sd_live_demo_merchant_123"
      }
    }
  }
}
```

#### Mode B: Cloud / Remote SSE Server (`http://127.0.0.1:8002/mcp`)
Start the FastMCP SSE daemon (optional `--require-auth` flag or `MCP_REQUIRE_AUTH=true` env var enforces API key / Bearer token validation at SSE handshake):
```powershell
python src/mcp/server.py --transport sse --port 8002 --require-auth
```

Configure `~/.gemini/config/mcp_config.json`:
```json
{
  "mcpServers": {
    "swiftdrop": {
      "url": "http://127.0.0.1:8002/mcp",
      "headers": {
        "X-API-Key": "sd_live_demo_merchant_123"
      }
    }
  }
}
```
*(Or via `mcp-remote` proxy bridge):*
```json
{
  "mcpServers": {
    "swiftdrop": {
      "command": "npx.cmd",
      "args": [
        "-y",
        "mcp-remote",
        "http://127.0.0.1:8002/mcp"
      ],
      "env": {
        "SWIFTDROP_API_KEY": "sd_live_demo_merchant_123"
      }
    }
  }
}
```
---

## 3. REST Endpoint Specifications

### 3.1 Chat with Assistant (`POST /api/v1/ai/chat`)

Processes a user prompt, executes necessary read-only tools, and returns a grounded response with structured generative UI actions and session continuity.

#### Request Headers
- `Content-Type: application/json` (Required)
- `Authorization: Bearer <jwt>` (Optional: Required for queries accessing private account parcels)
- `X-Correlation-ID: <uuid>` or `X-Request-ID: <uuid>` (Optional: Distributed tracing correlation ID; generated automatically if omitted)

#### Response Headers
- `X-Correlation-ID: <uuid>` (Distributed request tracking correlation identifier)
- `X-Frame-Options: DENY` (Clickjacking defense)
- `X-Content-Type-Options: nosniff` (MIME sniffing prevention)
- `Content-Security-Policy: default-src 'self'` (Content security restrictions)
- `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` (SlowAPI rate limit tracking)

#### Request Body Schema (`ChatMessageRequest`)
````json
{
  "prompt": "How much does it cost to ship a 2.5kg package from Colombo to Kandy?",
  "conversation_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "locale": "en-US",
  "history": []
}
```

#### Request Fields
- `prompt` (string, required): Natural language input (1 to 1,000 characters).
- `conversation_id` (string, optional, UUID format): Correlates multi-turn sessions in the cache. Supply the returned `conversation_id` across subsequent requests to preserve conversational context, recall prior tracking codes, and avoid redundant self-introductions. Automatically generated as a UUIDv4 if omitted.
- `locale` (string, optional): Client locale (defaults to `"en-US"`).
- `history` (array of `ChatMessage`, optional): Explicit prior turns if client manages context locally.

#### Success Response (`200 OK`)
````json
{
  "conversation_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "reply": "The estimated shipping fee for a 2.5kg package from Colombo to Kandy is **LKR 750** via **STANDARD** delivery (estimated transit: 48 hours).",
  "actions": [
    {
      "action_type": "NAVIGATE_BOOKING",
      "label": "Book Shipment Now",
      "target_url": "/portal/book",
      "prefill_data": {
        "weight_kg": 2.5,
        "pickup_postal_code": "00100",
        "delivery_postal_code": "20000",
        "service_type": "STANDARD",
        "estimated_fee": 750,
        "estimated_fee_cents": 75000,
        "currency": "LKR"
      }
    }
  ],
  "tools_called": [
    "estimate_delivery_fee"
  ],
  "latency_ms": 142,
  "grounded": true
}
```

> **Note on Sri Lankan Currency Formatting:** In Sri Lankan commerce, transactions operate exclusively in whole integer Sri Lankan Rupees (`LKR`); fractional cents are not used. The assistant formats all conversational fees as clean integers (e.g. `LKR 750`, `LKR 1,595`). Generative action cards provide both `"estimated_fee": 750` (clean integer Rupee ready for booking forms) and `"estimated_fee_cents": 75000` (minor units for Core API and payment gateway compatibility).

#### Error Responses (Standard RFC-7807 Format)

##### 422 Unprocessable Entity (Semantic Validation Failure)
````json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Request payload failed semantic domain validation",
    "details": [
      {
        "field": "prompt",
        "issue": "Prompt cannot be empty or whitespace only"
      }
    ]
  }
}
```

##### 401 Unauthorized (Authentication Failure)
````json
{
  "error": {
    "code": "AUTH_UNAUTHORIZED",
    "message": "Authentication token has expired",
    "details": []
  }
}
```

##### 429 Too Many Requests (Rate Limit Exceeded)
````json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Rate limit exceeded. Please wait 45 seconds before retrying.",
    "details": [
      {
        "field": "rate_limit",
        "issue": "Request frequency exceeded your tier's quota. Retry after 45s."
      }
    ]
  }
}
```

##### 502 Bad Gateway (Upstream Service Failure)
````json
{
  "error": {
    "code": "UPSTREAM_SERVICE_UNAVAILABLE",
    "message": "Upstream service 'core-pricing' failure: Connection timeout",
    "details": [
      {
        "field": "upstream",
        "issue": "core-pricing"
      }
    ]
  }
}
```

##### 500 Internal Server Error (Masked Server Error)
````json
{
  "error": {
    "code": "INTERNAL_SERVER_ERROR",
    "message": "An unexpected server error occurred. Please contact support if the issue persists.",
    "details": []
  }
}
```

---

### 3.2 Chat with Assistant — Server-Sent Events Stream (`POST /api/v1/ai/chat/stream`)

Processes user inquiry and streams response deltas, tool execution events, and generative UI action cards in real time conforming to the standard W3C EventSource wire format.

#### Request Headers
- `Content-Type: application/json` (Required)
- `Authorization: Bearer <jwt>` (Optional: Required for queries accessing private account parcels)
- `X-Correlation-ID: <uuid>` or `X-Request-ID: <uuid>` (Optional: Distributed tracing correlation ID)

#### Response Headers
- `Content-Type: text/event-stream; charset=utf-8`
- `Cache-Control: no-cache, no-transform`
- `Connection: keep-alive`
- `X-Accel-Buffering: no` (Instructs reverse proxies e.g. Traefik/Nginx not to buffer streaming chunks)
- `X-Correlation-ID: <uuid>`

#### Request Body Schema (`ChatMessageRequest`)
Exact same JSON payload structure as `POST /api/v1/ai/chat`:
````json
{
  "prompt": "How much does it cost to ship a 2.5kg package from Colombo to Kandy?",
  "conversation_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"
}
```

#### Event Stream Protocol (`text/event-stream`)

##### 1. `tool_start` Event
Emitted immediately when the LLM triggers a read-only tool call:
```http
event: tool_start
data: {"tool":"estimate_delivery_fee","call_id":"call_01"}
```

##### 2. `tool_end` Event
Emitted when upstream tool execution completes:
```http
event: tool_end
data: {"tool":"estimate_delivery_fee","call_id":"call_01","grounded":true,"duration_ms":38}
```

##### 3. `token` Event
Emitted incrementally for generated text deltas:
```http
event: token
data: {"delta":"The estimated shipping fee is **LKR 750**."}
```

##### 4. `action_card` Event
Emitted when a tool output includes a generative UI action:
```http
event: action_card
data: {"action_type":"NAVIGATE_BOOKING","label":"Book Shipment Now","target_url":"/portal/book","prefill_data":{"weight_kg":2.5,"pickup_postal_code":"00100","delivery_postal_code":"20000","service_type":"STANDARD","estimated_fee":750,"estimated_fee_cents":75000,"currency":"LKR"}}
```

##### 5. `done` Event
Emitted as the final terminal event of the stream:
```http
event: done
data: {"conversation_id":"3fa85f64-5717-4562-b3fc-2c963f66afa6","latency_ms":240,"tools_called":["estimate_delivery_fee"]}
```

##### 6. `error` Event
Emitted if an unrecoverable failure occurs mid-stream:
```http
event: error
data: {"code":"INTERNAL_STREAM_ERROR","message":"An error occurred during stream generation. Please try again."}
```

#### Pre-Stream Error Handling (RFC-7807 JSON)
If an error occurs *before* streaming begins (e.g. invalid payload, expired auth token, rate limit exceeded), the endpoint returns standard `application/json` with the appropriate status code:
- **422 Unprocessable Entity:** When prompt fails schema validation.
- **401 Unauthorized:** When Bearer JWT is expired or tampered.
- **429 Too Many Requests:** When client exceeds tier rate limit ceiling.

---

### 3.3 Liveness Probe (`GET /health/live`)
Confirms that the FastAPI process and asyncio event loop are responsive.

````json
{
  "status": "UP",
  "timestamp": "2026-11-02T13:30:00.000000+00:00",
  "service": "ai-assistant"
}
```

---

### 3.3 Readiness Probe (`GET /health/ready`)
Verifies local cache sanity, engine mode, and tool registrations. **Strictly decoupled from external upstream services.**

````json
{
  "status": "READY",
  "engine": "live",
  "active_sessions": 4,
  "registered_tools": 4,
  "timestamp": "2026-11-02T13:30:00.000000+00:00"
}
```

---

### 3.4 Telemetry Metrics (`GET /health/metrics`)
Operational metrics for health monitoring and capacity audits.

````json
{
  "service": "ai-assistant",
  "version": "2.5.1",
  "uptime_seconds": 1284,
  "active_sessions": 4,
  "max_capacity": 10000,
  "registered_tools": 4,
  "mock_mode": false,
  "timestamp": "2026-11-02T13:30:00.000000+00:00"
}
```
