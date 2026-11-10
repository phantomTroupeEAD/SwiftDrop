# Tracking API Specifications (`/api/v1/tracking`)

> **Service:** `services/tracking` | **Owner:** Member 5 | **Status:** Phase 4 (Standalone SSE Infrastructure & Public REST)

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
id: SD-20261025-A8F3K2:OUT_FOR_DELIVERY:1730036700
event: status_update
data: {"tracking_number":"SD-20261025-A8F3K2","status":"OUT_FOR_DELIVERY","description":"Courier is out for final doorstep delivery.","timestamp":"2026-10-25T13:45:00Z"}

:keepalive
```

### Protocol Specifications
1. **Initial Hydration:** Pushes the current parcel state upon connection.
2. **Event Type:** `event: status_update`
3. **Event Identifier:** `id: {tracking_number}:{status}:{unix_timestamp}`
4. **Heartbeats:** Comment frame `:keepalive\n\n` emitted every 15 seconds to prevent gateway timeout.
5. **Reconnection & Replay:** Client sends `Last-Event-ID`. Unseen milestones from committed status history are replayed.
6. **Privacy Invariant (SEC-009):** Only safe allowlisted fields (`tracking_number`, `status`, `description`, `timestamp`) are sent. PII, addresses, notes, and internal actor IDs are strictly excluded.
7. **Unavailable Event Source:** When the real-time event broker is unconfigured, the endpoint explicitly returns HTTP 503 `DEPENDENCY_UNAVAILABLE` rather than accepting dead streams.

### HTTP Error Envelopes
- `400 Bad Request`: `INVALID_TRACKING_NUMBER` — Tracking number syntax invalid (`SD-YYYYMMDD-XXXXXX`).
- `404 Not Found`: `TRACKING_NOT_FOUND` — Parcel does not exist.
- `429 Too Many Requests`: `RATE_LIMIT_EXCEEDED` — Concurrent stream limit exceeded.
- `500 Internal Server Error`: `STREAMING_UNSUPPORTED` — Client/proxy does not support `http.Flusher`.
- `503 Service Unavailable`: `DEPENDENCY_UNAVAILABLE` — Event broker or database connection unavailable.
