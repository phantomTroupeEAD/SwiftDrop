# Document 4 — Interface & Integration Contract

**Project:** SwiftDrop — Parcel Delivery & Courier Management System  
**Module:** Tracking API & Real-Time Updates  
**Module Owner:** Member 5 (Senuji)  
**Version:** 1.0.0 — Post-Audit Harmonized  
**Status:** Approved & Ratified  
**Primary Technology:** Go 1.22+, REST/JSON, Server-Sent Events (SSE), RabbitMQ, Redis Pub/Sub  
**Target Codepath:** `services/tracking/` (Port `:8081`)  
**Related Documents:** Documents 1–3, `docs/contracts/api-conventions.md`, `docs/contracts/event-contracts.md`, SRS v1.0 (FR-TRK-001..007, SEC-009)

---

## 1. Purpose & Standards Alignment

This document defines the formal network interfaces and integration contracts for the Tracking API service.

All endpoints adhere strictly to the ratified team-wide standards:
1. **JSON Convention:** All field names use `snake_case` wrapped in a top-level `{ "data": ... }` response envelope (`api-conventions.md`).
2. **Standard Tracking Identifiers:** Tracking numbers follow the standard format `SD-YYYYMMDD-XXXXXX` (e.g., `SD-20261025-A8F3K2`).
3. **Public Privacy Protection (SEC-009):** Public responses contain destination `city` only; street address, phone number, recipient name, internal notes, fee amounts, and `zone_id` are strictly redacted.
4. **SSE Event Specification:** Event type is standardized as **`status_update`**, accompanied by 15-second `:keepalive\n\n` comments.

---

## 2. API Endpoint Catalog

| Endpoint | Method | Access | Format | Purpose |
|---|---|---|---|---|
| `/api/v1/tracking/public/{tracking_number}` | `GET` | Public | JSON | Retrieve current status and sanitized milestone timeline. |
| `/api/v1/tracking/public/{tracking_number}/stream` | `GET` | Public | SSE | Live real-time stream of parcel milestone events (public alias). |
| `/api/v1/tracking/{tracking_number}/stream` | `GET` | Public | SSE | Canonical real-time stream of parcel milestone events. |
| `/api/v1/tracking/{tracking_number}` | `GET` | Bearer JWT | JSON | Complete historical audit trail for authorized parcel owner. |
| `/health/live` | `GET` | Internal | JSON | Kubernetes liveness probe. |
| `/health/ready` | `GET` | Internal | JSON | Kubernetes readiness probe (PostgreSQL/Redis connectivity). |
| `/metrics` | `GET` | Internal | Text | Prometheus metrics scraper. |

---

## 3. Public Parcel Tracking (`GET /api/v1/tracking/public/{tracking_number}`)

### 3.1 Request Specification
- **Path Parameter:** `tracking_number` (string, required) — format `^SD-[0-9]{8}-[A-Z0-9]{6}$`.
- **Headers:** `Accept: application/json`
- **Authentication:** None (Public).

### 3.2 Success Response (`200 OK`)

```json
{
  "data": {
    "tracking_number": "SD-20261025-A8F3K2",
    "status": "OUT_FOR_DELIVERY",
    "status_description": "Courier is out for final doorstep delivery.",
    "destination": {
      "city": "Colombo"
    },
    "estimated_delivery": "2026-10-25T16:00:00Z",
    "milestones": [
      {
        "status": "CREATED",
        "description": "Shipment booked and delivery label generated.",
        "timestamp": "2026-10-25T08:30:00Z"
      },
      {
        "status": "PICKED_UP",
        "description": "Parcel collected by courier from sender.",
        "timestamp": "2026-10-25T10:15:00Z"
      },
      {
        "status": "IN_TRANSIT",
        "description": "Parcel sorted and in transit between hubs.",
        "timestamp": "2026-10-25T12:00:00Z"
      },
      {
        "status": "OUT_FOR_DELIVERY",
        "description": "Courier is out for final doorstep delivery.",
        "timestamp": "2026-10-25T13:45:00Z"
      }
    ]
  }
}
```

> **Privacy Invariant (SEC-009):** Notice that `zone_id` is completely removed. Destination exposes **city only**.

---

## 4. Authenticated Tracking History (`GET /api/v1/tracking/{tracking_number}`)

### 4.1 Request Specification
- **Path Parameter:** `tracking_number` (string, required).
- **Headers:** 
  - `Accept: application/json`
  - `Authorization: Bearer <jwt_access_token>` (RS256 signed by Member 1)

### 4.2 Authorization Rules
- Customer token: `auth.UserID == parcels.sender_id`.
- Merchant token: `auth.TenantID == parcels.merchant_id`.
- Admin/Dispatcher: Allowed for operational tracking.
- If unauthorized: Returns `403 Forbidden` (`ACCESS_DENIED`).

### 4.3 Success Response (`200 OK`)

```json
{
  "data": {
    "tracking_number": "SD-20261025-A8F3K2",
    "parcel_id": "7b8f9e2d-3c4a-4e5b-9d1a-8f2c3e4b5a6c",
    "status": "ATTEMPT_FAILED",
    "status_description": "Delivery attempt unsuccessful. Reattempt scheduled.",
    "destination": {
      "city": "Colombo",
      "postal_code": "00300",
      "zone_id": "COLOMBO_03"
    },
    "milestones": [
      {
        "status": "CREATED",
        "description": "Shipment booked and delivery label generated.",
        "actor_role": "MERCHANT",
        "timestamp": "2026-10-25T08:30:00Z"
      },
      {
        "status": "PICKED_UP",
        "description": "Parcel collected by courier from sender.",
        "actor_role": "COURIER",
        "timestamp": "2026-10-25T10:15:00Z"
      },
      {
        "status": "OUT_FOR_DELIVERY",
        "description": "Courier dispatched for doorstep delivery.",
        "actor_role": "COURIER",
        "timestamp": "2026-10-25T13:45:00Z"
      },
      {
        "status": "ATTEMPT_FAILED",
        "description": "Customer unavailable at delivery premises.",
        "reason_code": "RECIPIENT_UNAVAILABLE",
        "actor_role": "COURIER",
        "timestamp": "2026-10-25T15:30:00Z"
      }
    ]
  }
}
```

---

## 5. Server-Sent Events (SSE) Live Stream Contract

### 5.1 Endpoint Details
- **Paths:** 
  - Canonical: `GET /api/v1/tracking/{tracking_number}/stream`
  - Public Alias: `GET /api/v1/tracking/public/{tracking_number}/stream`
- **Protocol:** `text/event-stream; charset=utf-8`
- **HTTP Headers:**
  - `Content-Type: text/event-stream`
  - `Cache-Control: no-cache, no-transform`
  - `Connection: keep-alive`
  - `X-Accel-Buffering: no` (bypasses reverse proxy buffering)

### 5.2 SSE Wire Protocol & Framing

```text
:keepalive

event: status_update
id: SD-20261025-A8F3K2:OUT_FOR_DELIVERY:1730036700
data: {"tracking_number":"SD-20261025-A8F3K2","status":"OUT_FOR_DELIVERY","description":"Courier is out for final doorstep delivery.","timestamp":"2026-10-25T13:45:00Z"}

:keepalive
```

### 5.3 SSE Framing Invariants
1. **Initial Hydration:** The server immediately pushes the parcel's current status upon connection before entering the listen loop.
2. **Heartbeat Frame:** The server sends a comment line `:keepalive\n\n` every **15 seconds** (Agreement A2).
3. **Event Identifier:** The `id` field combines `{tracking_number}:{status}:{unix_timestamp}`, enabling client reconciliation on reconnect via `Last-Event-ID`.
4. **Data Payload:** Sanitized JSON object containing `tracking_number`, `status`, `description`, and ISO 8601 UTC `timestamp`.

---

## 6. HTTP Error Response Contract

All error responses strictly follow the standard SwiftDrop error envelope (`api-conventions.md`):

```json
{
  "error": {
    "code": "TRACKING_NOT_FOUND",
    "message": "Shipment not found for tracking number SD-20261025-A8F3K2",
    "details": []
  }
}
```

### Standard Error Codes

| HTTP Status | Error Code | Trigger Condition |
|---|---|---|
| `400 Bad Request` | `INVALID_TRACKING_NUMBER` | Malformed tracking number syntax. |
| `401 Unauthorized` | `UNAUTHENTICATED` | Missing, expired, or invalid Bearer JWT. |
| `403 Forbidden` | `ACCESS_DENIED` | Authenticated user is not the parcel owner or merchant. |
| `404 Not Found` | `TRACKING_NOT_FOUND` | No parcel exists matching the tracking number. |
| `429 Too Many Requests` | `RATE_LIMIT_EXCEEDED` | Request rate exceeds gateway threshold. |
| `503 Service Unavailable` | `DEPENDENCY_UNAVAILABLE` | Database pool exhausted or connection down. |

---

## 7. Event Ingestion Contract (RabbitMQ & Redis)

### 7.1 Upstream RabbitMQ Consumer (`q.tracking.updates`)

- **Exchange:** `swiftdrop.events.topic` (Topic Exchange)
- **Queue Name:** `q.tracking.updates` (Durable, Quorum or Classic)
- **Routing Keys:** `parcel.*`, `courier.*`
- **Delivery Guarantee:** At-least-once delivery; consumer acknowledges after republishing to Redis.

#### Incoming RabbitMQ Event Schema Example (`parcel.delivered.v1`)
```json
{
  "event_id": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
  "event_type": "parcel.delivered.v1",
  "schema_version": 1,
  "aggregate_id": "7b8f9e2d-3c4a-4e5b-9d1a-8f2c3e4b5a6c",
  "occurred_at": "2026-10-25T14:30:00Z",
  "producer": "dispatch-worker",
  "data": {
    "tracking_number": "SD-20261025-A8F3K2",
    "status": "DELIVERED",
    "reason_code": null,
    "proof_id": "prf_8f9a2b1c",
    "delivered_at": "2026-10-25T14:30:00Z"
  }
}
```

### 7.2 Internal Redis Pub/Sub Payload (`tracking:updates`)

When a `tracking-api` pod consumes a message from `q.tracking.updates`, it publishes this compact payload to Redis channel `tracking:updates`:

```json
{
  "tracking_number": "SD-20261025-A8F3K2",
  "status": "DELIVERED",
  "description": "Parcel successfully delivered to recipient.",
  "timestamp": "2026-10-25T14:30:00Z"
}
```

---

## 8. Ratified Contract Decisions Table

| Contract Element | Ratified Specification |
|---|---|
| **JSON Style** | `snake_case` keys enclosed in `{ "data": ... }` envelope. |
| **Tracking Number Format** | `SD-YYYYMMDD-XXXXXX` (e.g., `SD-20261025-A8F3K2`). |
| **Public Privacy Scope** | Destination `city` only; street address, phone, notes, and `zone_id` redacted. |
| **SSE Event Type** | `event: status_update` |
| **SSE Heartbeat** | `:keepalive\n\n` emitted every 15 seconds. |
| **Ingestion Queue** | `q.tracking.updates` on RabbitMQ topic exchange `swiftdrop.events.topic`. |
| **Internal Fan-Out** | Redis Pub/Sub channel `tracking:updates`. |
| **Error Format** | `{ "error": { "code": "...", "message": "...", "details": [] } }` |

---
*End of Document 4 — Interface & Integration Contract (v1.0.0)*