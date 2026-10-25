# AI Assistant & FastMCP API Specifications

> **Service:** `services/ai-assistant`  
> **Assigned Owner:** Member 10 — AI Assistant/MCP & Cross-Workflow Quality  
> **Base Path:** `/api/v1/ai`  
> **Container Port:** `8000`  
> **Specification Version:** `2.0.0` (Authoritative System-Wide Baseline)

---

## 1. Endpoint Overview

| Method | URI Path | Description | Required Auth / Role |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/ai/chat` | Natural language chat endpoint with automatic tool invocation and grounded responses | Optional (Public for tracking queries; Bearer JWT for customer parcel lookup) |
| `GET` | `/health/live` | Kubernetes Liveness Probe: confirms FastAPI event loop responsiveness | None (Public) |
| `GET` | `/health/ready` | Kubernetes Readiness Probe: verifies local LLM config & memory cache | None (Public) |
| `GET` | `/metrics` | Prometheus metrics endpoint for request counts and LLM latency | Internal / Prometheus Scraper |

---

## 2. FastMCP Tool Definitions

The AI Assistant service also exposes tools via the Model Context Protocol (FastMCP) for desktop and agentic clients:

### 2.1 `track_parcel`
- **Description:** Retrieve real-time tracking milestones and current status for a parcel.
- **Parameters:**
  - `tracking_number` (string, required): Parcel tracking code matching pattern `^SD-\d{8}-\d{5}$`.
- **Target Backend Call:** `GET http://tracking-api:8001/api/v1/tracking/public/{tracking_number}`
- **Permissions:** Public / Anonymous.

### 2.2 `estimate_shipping_fee`
- **Description:** Compute authoritative delivery fee estimate and tariff breakdown based on parcel dimensions, weight, and delivery zones.
- **Parameters:**
  - `weight_kg` (float, required): Weight in kilograms (> 0).
  - `length_cm` (float, required): Parcel length in cm (> 0).
  - `width_cm` (float, required): Parcel width in cm (> 0).
  - `height_cm` (float, required): Parcel height in cm (> 0).
  - `service_type` (string, required): Delivery tier: `"STANDARD"`, `"EXPRESS"`, or `"SAME_DAY"`.
  - `pickup_postal` (string, required): Postal code for pickup origin (e.g. `"10100"`).
  - `delivery_postal` (string, required): Postal code for delivery destination (e.g. `"00100"`).
- **Target Backend Call:** `POST http://core-api:8080/api/v1/pricing/quotes`
- **Permissions:** Public / Anonymous.

### 2.3 `get_my_active_parcels`
- **Description:** Retrieve currently active shipments (in-transit, out-for-delivery) for the authenticated customer or merchant.
- **Parameters:** None.
- **Target Backend Call:** `GET http://core-api:8080/api/v1/parcels` (forwards caller's `Authorization: Bearer <JWT>`).
- **Permissions:** Customer (`CUSTOMER`) or Merchant (`MERCHANT`).

---

## 3. REST Endpoint Specifications

### 3.1 Chat with Assistant (`POST /api/v1/ai/chat`)

Processes a user prompt, executes necessary FastMCP tools, and returns a grounded response with structured generative UI actions.

#### Request Headers
- `Content-Type: application/json` (Required)
- `Authorization: Bearer <jwt>` (Optional: Required for queries accessing private parcel data)
- `X-Correlation-ID: <uuid>` (Optional: Tracing identifier)

#### Request Body Schema
```json
{
  "type": "object",
  "required": ["message"],
  "properties": {
    "message": {
      "type": "string",
      "minLength": 1,
      "maxLength": 2000,
      "description": "User natural language input query."
    },
    "conversation_id": {
      "type": "string",
      "format": "uuid",
      "description": "Optional UUID identifying existing multi-turn conversation session."
    },
    "locale": {
      "type": "string",
      "default": "en-US",
      "description": "Preferred language for response generation."
    }
  }
}
```

#### Request Payload Example
```json
{
  "message": "How much would it cost to send a 2.5kg box (30x20x15 cm) from Colombo 01 (00100) to Kandy (20000) using Express delivery?",
  "conversation_id": "a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d"
}
```

#### Success Response (`200 OK`)
```json
{
  "success": true,
  "data": {
    "conversation_id": "a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d",
    "response": "Sending a 2.5 kg package (30×20×15 cm) from Colombo 01 to Kandy via **Express Delivery** is estimated at **LKR 750.00**.\n\n### Tariff Breakdown:\n- **Base Shipping Fee:** LKR 500.00\n- **Weight Surcharge (2.5 kg):** LKR 150.00\n- **Express Service Surcharge:** LKR 100.00\n- **Estimated Delivery Window:** Next business day by 5:00 PM.\n\nWould you like to book this shipment now?",
    "tools_used": [
      {
        "name": "estimate_shipping_fee",
        "parameters": {
          "weight_kg": 2.5,
          "length_cm": 30.0,
          "width_cm": 20.0,
          "height_cm": 15.0,
          "service_type": "EXPRESS",
          "pickup_postal": "00100",
          "delivery_postal": "20000"
        },
        "status": "SUCCESS"
      }
    ],
    "suggested_actions": [
      {
        "action_type": "NAVIGATE_BOOKING",
        "label": "Book This Shipment",
        "payload": {
          "weight_kg": 2.5,
          "length_cm": 30.0,
          "width_cm": 20.0,
          "height_cm": 15.0,
          "service_type": "EXPRESS",
          "pickup_postal": "00100",
          "delivery_postal": "20000",
          "estimated_fee_cents": 75000,
          "currency": "LKR"
        }
      }
    ],
    "execution_time_ms": 342
  }
}
```

#### Error Responses

##### 400 Bad Request
Empty message or invalid JSON payload.
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Message field cannot be empty or whitespace only.",
    "details": [
      {
        "field": "message",
        "issue": "String should have at least 1 characters"
      }
    ]
  }
}
```

##### 401 Unauthorized
Attempted to invoke private tool (`get_my_active_parcels`) without a valid Bearer token.
```json
{
  "success": false,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Authentication required to query active account parcels."
  }
}
```

##### 429 Too Many Requests
Rate limit exceeded (e.g. > 30 requests/minute per IP/tenant).
```json
{
  "success": false,
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "AI assistant rate limit exceeded. Please wait 60 seconds before retrying.",
    "retry_after_seconds": 60
  }
}
```

##### 502 Bad Gateway
Upstream Core API or Tracking API returned an unrecoverable network or 5xx error.
```json
{
  "success": false,
  "error": {
    "code": "UPSTREAM_SERVICE_UNAVAILABLE",
    "message": "Failed to retrieve quote from pricing service. Please try again shortly."
  }
}
```

---

### 3.2 Liveness Probe (`GET /health/live`)
Confirms that the FastAPI process and event loop are responsive.

- **Success Response (`200 OK`):**
```json
{
  "status": "alive"
}
```

---

### 3.3 Readiness Probe (`GET /health/ready`)
Verifies local configuration and memory cache readiness. **Strictly decoupled from upstream external services.**

- **Success Response (`200 OK`):**
```json
{
  "status": "ready",
  "llm_backend": "mock",
  "cache_status": "ok"
}
```

- **Failure Response (`503 Service Unavailable`):**
```json
{
  "status": "unavailable",
  "reason": "Memory cache initialization failed"
}
```
