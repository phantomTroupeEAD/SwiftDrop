# AI Assistant & FastMCP API Specifications

> **Service:** `services/ai-assistant`  
> **Assigned Owner:** Member 10 — AI Assistant/MCP & Cross-Workflow Quality  
> **Base Path:** `/api/v1/ai`  
> **Container Port:** `8000`  
> **Specification Version:** `2.5.1` (Authoritative System Baseline)

---

## 1. Endpoint Overview & Interactive Documentation

| Method | URI Path | Description | Required Auth / Role |
| :--- | :--- | :--- | :--- |
| `GET` | `/docs` | Interactive Swagger UI (OpenAPI 3.1.0) | None (Developer UI) |
| `GET` | `/redoc` | Interactive ReDoc Documentation | None (Developer UI) |
| `GET` | `/openapi.json` | Canonical OpenAPI Schema Specification | None (Public) |
| `POST` | `/api/v1/ai/chat` | Natural language chat endpoint with automatic tool invocation and grounded responses | Optional (Public for tracking & quotes; Bearer JWT for customer parcel lookup) |
| `GET` | `/health/live` | Kubernetes Liveness Probe: confirms FastAPI event loop responsiveness | None (Public) |
| `GET` | `/health/ready` | Kubernetes Readiness Probe: verifies local LLM config & memory cache sanity | None (Public) |
| `GET` | `/health/metrics` | Prometheus & runtime metrics endpoint for active sessions, uptime, and capacity | None (Public) |

---

## 2. FastMCP & Read-Only Tool Definitions

The AI Assistant service exposes approved read-only tools via the Model Context Protocol (FastMCP) and standard OpenAI function calling:

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
  - `weight_kg` (float, required): Package weight in kilograms (`0.0 < weight_kg <= 50.0`).
  - `pickup_postal_code` (string, required): 5-digit origin postal code (e.g. `"00100"`).
  - `delivery_postal_code` (string, required): 5-digit destination postal code (e.g. `"20000"`).
  - `service_type` (string, optional): Delivery tier: `"STANDARD"`, `"EXPRESS"`, or `"SAME_DAY"` (default: `"STANDARD"`).
  - `length_cm` (float, optional): Package length in cm (`0.0 < length_cm <= 300.0`).
  - `width_cm` (float, optional): Package width in cm (`0.0 < width_cm <= 300.0`).
  - `height_cm` (float, optional): Package height in cm (`0.0 < height_cm <= 300.0`).
- **Target Upstream Endpoint:** `POST http://core-api:8080/api/v1/pricing/quotes`
- **Generative UI Action:** Returns `NAVIGATE_BOOKING` card targeting `/portal/book`.
- **Permissions:** Public / Anonymous.

### 2.3 `get_my_active_parcels`
- **Description:** Retrieve active shipments (picked-up, in-transit, out-for-delivery) belonging to the authenticated customer or merchant.
- **Parameters:**
  - `status` (string, optional): Filter by parcel lifecycle status (e.g. `"IN_TRANSIT"`).
  - `limit` (integer, optional): Maximum parcels to return (`1 <= limit <= 50`, default: `10`).
- **Target Upstream Endpoint:** `GET http://core-api:8080/api/v1/parcels` (forwards caller's `Authorization: Bearer <JWT>`).
- **Security & Isolation:** IDOR-safe; Core API extracts tenant and customer identity from the verified JWT.
- **Permissions:** Authenticated Customer or Merchant.

### 2.4 `explain_company_policies`
- **Description:** Explain courier company policies, prohibited goods, packaging standards, transit SLAs, and dispute rules.
- **Parameters:**
  - `query` (string, required): Inquired policy topic (e.g. `"prohibited items"`, `"packaging limits"`, `"SLA"`).
- **Target Upstream Endpoint:** `GET http://core-api:8080/api/v1/admin/settings` (with local `company_policies.md` grounding).
- **Permissions:** Public / Anonymous.

### 2.5 Authoritative FastMCP Context Resources
- **`swiftdrop://policies`:** Exposes full authoritative markdown shipping guidelines (`company_policies.md`) for direct LLM context window ingestion.
- **`swiftdrop://sla-matrix`:** Exposes structured JSON transit SLAs (Express 24h, Standard 48–72h, Outstation 3–5 days).

---

## 3. REST Endpoint Specifications

### 3.1 Chat with Assistant (`POST /api/v1/ai/chat`)

Processes a user prompt, executes necessary read-only tools, and returns a grounded response with structured generative UI actions and session continuity.

#### Request Headers
- `Content-Type: application/json` (Required)
- `Authorization: Bearer <jwt>` (Optional: Required for queries accessing private account parcels)

#### Request Body Schema (`ChatMessageRequest`)
```json
{
  "prompt": "How much does it cost to ship a 2.5kg package from Colombo to Kandy?",
  "conversation_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "locale": "en-US",
  "history": []
}
```

#### Request Fields
- `prompt` (string, required): Natural language input (1 to 1,000 characters).
- `conversation_id` (string, optional, UUID format): Correlates multi-turn sessions in the cache. Automatically generated as a UUIDv4 if omitted.
- `locale` (string, optional): Client locale (defaults to `"en-US"`).
- `history` (array of `ChatMessage`, optional): Explicit prior turns if client manages context locally.

#### Success Response (`200 OK`)
```json
{
  "conversation_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "reply": "The estimated shipping fee for a 2.5kg package from Colombo to Kandy is **LKR 750.00** via **STANDARD** delivery (estimated transit: 48 hours).",
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

#### Error Responses (Standard RFC-7807 Format)

##### 422 Unprocessable Entity (Semantic Validation Failure)
```json
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

##### 502 Bad Gateway (Upstream Service Failure)
```json
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
```json
{
  "error": {
    "code": "INTERNAL_SERVER_ERROR",
    "message": "An unexpected server error occurred. Please contact support if the issue persists.",
    "details": []
  }
}
```

---

### 3.2 Liveness Probe (`GET /health/live`)
Confirms that the FastAPI process and asyncio event loop are responsive.

```json
{
  "status": "UP",
  "timestamp": "2026-11-02T13:30:00.000000+00:00",
  "service": "ai-assistant"
}
```

---

### 3.3 Readiness Probe (`GET /health/ready`)
Verifies local cache sanity, engine mode, and tool registrations. **Strictly decoupled from external upstream services.**

```json
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

```json
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
