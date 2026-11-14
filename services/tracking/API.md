# Tracking API Specifications (`/api/v1/tracking`)

> **Service:** `services/tracking` | **Owner:** Member 5 | **Status:** Phase 3 & 4 (Authenticated Tracking REST, Standalone SSE Infrastructure & Public REST)

---

## 1. Public Tracking REST Lookup
Retrieves current parcel status, destination city, and sanitized chronological milestone timeline. Strictly redacts all PII per SEC-009.

- **Method / Path:** `GET /api/v1/tracking/public/{tracking_number}`
- **Auth Required:** None (Public)
- **Headers:** `Accept: application/json`

### Success Response (`200 OK`)
```json
{
  "data": {
    "tracking_number": "SD-20261025-A8F3K2",
    "status": "OUT_FOR_DELIVERY",
    "status_description": "Courier is out for final doorstep delivery.",
    "destination": {
      "city": "Colombo"
    },
    "estimated_delivery": null,
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

---

## 2. Real-Time Server-Sent Events (SSE) Stream
Streams real-time milestone events and 15-second heartbeat keepalive frames. Strictly isolated to the public namespace to preserve `/api/v1/tracking/*` for authenticated tracking.

- **Canonical Public Path:** `GET /api/v1/tracking/public/{tracking_number}/stream`
- **Auth Required:** None (Public)
- **Response Headers:**
  - `Content-Type: text/event-stream`
  - `Cache-Control: no-cache, no-transform`
  - `Connection: keep-alive`
  - `X-Accel-Buffering: no`

### Wire Protocol & Framing
```text
id: SD-20261025-A8F3K2:OUT_FOR_DELIVERY:1730036700:0192e2b3-7c8a-7890-a123-000000000042
event: status_update
data: {"tracking_number":"SD-20261025-A8F3K2","status":"OUT_FOR_DELIVERY","description":"Courier is out for final doorstep delivery.","timestamp":"2026-10-25T13:45:00Z"}

:keepalive
```

### Protocol Specifications
1. **Initial Hydration:** Pushes the current parcel state upon connection.
2. **Event Type:** `event: status_update`
3. **Event Identifier:** `id: {tracking_number}:{status}:{unix_timestamp}[:{status_history_id}]` (incorporates Member 2's `status_history_id` as tiebreaker).
4. **Heartbeats:** Comment frame `:keepalive\n\n` emitted every 15 seconds to prevent gateway timeout.
5. **Reconnection & Replay:** Client sends `Last-Event-ID`. Unseen milestones from committed status history are replayed chronologically without gaps or skipped duplicates.
6. **Privacy Invariant (SEC-009):** Only safe allowlisted fields (`tracking_number`, `status`, `description`, `timestamp`) are sent. PII, addresses, notes, and internal actor IDs are strictly excluded.
7. **Unavailable Event Source:** When the real-time event broker is unconfigured, the endpoint explicitly returns HTTP 503 `DEPENDENCY_UNAVAILABLE` rather than accepting dead streams.

### HTTP Error Envelopes
- `400 Bad Request`: `INVALID_TRACKING_NUMBER` — Tracking number syntax invalid (`SD-YYYYMMDD-XXXXXX`).
- `404 Not Found`: `TRACKING_NOT_FOUND` — Parcel does not exist.
- `429 Too Many Requests`: `RATE_LIMIT_EXCEEDED` — Concurrent stream limit exceeded.
- `500 Internal Server Error`: `STREAMING_UNSUPPORTED` — Client/proxy does not support `http.Flusher`.
- `503 Service Unavailable`: `DEPENDENCY_UNAVAILABLE` — Event broker or database connection unavailable.

---

## 3. Authenticated Parcel Tracking History
Retrieves full parcel status history, destination details, and operational milestone audit trail for authorized parcel owners. Strictly enforces server-side database ownership checks.

- **Method / Path:** `GET /api/v1/tracking/{tracking_number}`
- **Auth Required:** Bearer JWT Access Token (HS256 signed by Member 1)
- **Headers:**
  - `Accept: application/json`
  - `Authorization: Bearer <jwt_access_token>`

### Authorization Rules
1. **Customer (`role == "CUSTOMER"`):** Authorized if and only if `parcels.sender_id == auth.UserID`.
2. **Merchant (`role == "MERCHANT"`):** Authorized if and only if `auth.TenantID != nil` and `parcels.merchant_id == *auth.TenantID`.
3. **Operations (`role IN ("ADMIN", "DISPATCHER")`):** Authorized for operational parcel oversight.
4. **Courier (`role == "COURIER"`):** Forbidden (`403 ACCESS_DENIED`); couriers must use dedicated delivery execution routes.
5. **Unauthorized Caller:** Returns `403 Forbidden` (`ACCESS_DENIED`) without leaking parcel details.

### Success Response (`200 OK`)
```json
{
  "data": {
    "tracking_number": "SD-20261025-A8F3K2",
    "parcel_id": "7b8f9e2d-3c4a-4e5b-9d1a-8f2c3e4b5a6c",
    "status": "ATTEMPT_FAILED",
    "status_description": "Delivery attempt was unsuccessful; reattempt will be scheduled.",
    "destination": {
      "city": "Colombo",
      "postal_code": "00300",
      "zone_id": "ZONE_COLOMBO_03"
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
        "status": "IN_TRANSIT",
        "description": "Parcel sorted and in transit between hubs.",
        "actor_role": "COURIER",
        "timestamp": "2026-10-25T12:00:00Z"
      },
      {
        "status": "OUT_FOR_DELIVERY",
        "description": "Courier is out for final doorstep delivery.",
        "actor_role": "COURIER",
        "timestamp": "2026-10-25T13:45:00Z"
      },
      {
        "status": "ATTEMPT_FAILED",
        "description": "Delivery attempt was unsuccessful; reattempt will be scheduled.",
        "actor_role": "COURIER",
        "reason_code": "RECIPIENT_UNAVAILABLE",
        "timestamp": "2026-10-25T15:30:00Z"
      }
    ]
  }
}
```

### Privacy & Security Invariants (SEC-009)
1. **Privacy Masking:** Recipient personal names, recipient phone numbers, exact street address line 1, and internal employee user IDs are strictly withheld from the payload.
2. **Access Control:** An unauthenticated request is rejected with `401 UNAUTHENTICATED`.
3. **Object-Level Authorization (IDOR Prevention):** A valid JWT alone does not grant access; the caller's verified `sub` (or `tenant_id`) must match the authoritative parcel record.
4. **Information Disclosure Prevention:** Attempting to access an unauthorized parcel returns `403 ACCESS_DENIED` with zero parcel data disclosed.

### HTTP Error Envelopes
- `400 Bad Request`: `INVALID_TRACKING_NUMBER` — Tracking number syntax invalid (`SD-YYYYMMDD-XXXXXX`).
- `401 Unauthorized`: `UNAUTHENTICATED` — Missing, expired, malformed, or invalid Bearer JWT.
- `403 Forbidden`: `ACCESS_DENIED` — Authenticated caller is not the authorized owner of the parcel.
- `404 Not Found`: `TRACKING_NOT_FOUND` — Parcel does not exist.
- `503 Service Unavailable`: `DEPENDENCY_UNAVAILABLE` — Database connection unavailable.
