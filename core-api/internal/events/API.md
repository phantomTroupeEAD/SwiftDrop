# SwiftDrop Events & Messaging Operations API Specification (`core-api/internal/events`)

> **Module Owner:** Member 08 — Event-Driven Backend & Integrations  
> **Authority:** Living API specification governed by `AGENTS.md` §6.2 and `docs/contracts/api-conventions.md`.

---

## 1. Endpoints Overview

| Method | URI Path | Required RBAC Role | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/admin/events/outbox` | `ADMIN` | List staged outbox events with status/type filtering and cursor pagination. |
| `GET` | `/api/v1/admin/events/outbox/stats` | `ADMIN` | Get aggregate counts by outbox status and age of oldest pending event. |
| `POST` | `/api/v1/admin/events/outbox/{id}/retry` | `ADMIN` | Un-stall a `FAILED` outbox event and reset status to `PENDING`. |
| `GET` | `/api/v1/admin/events/deliveries` | `ADMIN` | Unified audit log of customer email notifications and merchant webhooks. |
| `POST` | `/api/v1/admin/events/webhook-deliveries/{id}/retry` | `ADMIN` | Reset a `FAILED` webhook delivery back to `PENDING`. |
| `GET` | `/api/v1/merchant/webhook-deliveries` | `MERCHANT` | List webhook deliveries for endpoints owned by the authenticated merchant. |
| `GET` | `/api/v1/merchant/webhook-deliveries/{id}/attempts` | `MERCHANT` | List ordered HTTP delivery attempt history for an owned webhook delivery. |

---

## 2. Authentication & Authorization Headers

All endpoints require a valid JWT Bearer access token:
```http
Authorization: Bearer <jwt_access_token>
```
Merchant endpoints additionally require tenant validation via `TenantResolver`. The `TenantID` in the JWT claims must parse as a valid UUID matching the merchant record; non-UUID tenant claims return `403 TENANT_NOT_MERCHANT_UUID`.

---

## 3. Endpoint Contracts & Schemas

### 3.1 `GET /api/v1/admin/events/outbox`
Retrieves outbox events staged in PostgreSQL.

**Query Parameters:**
- `status` (string, optional): Filter by `PENDING`, `PUBLISHED`, or `FAILED`.
- `event_type` (string, optional): Exact event type match (e.g. `parcel.booked.v1`).
- `limit` (integer, optional, default: 25, range: 1–100): Page size.
- `cursor` (string, optional): Opaque base64 URL-encoded cursor for pagination.

**Success Response (`200 OK`):**
```json
{
  "data": [
    {
      "id": "0192e2b3-7c8a-7890-a123-456789abcdef",
      "event_id": "0192e2b3-7c8a-7890-a123-000000000001",
      "event_type": "parcel.booked.v1",
      "aggregate_type": "PARCEL",
      "aggregate_id": "0192e2b3-7c8a-7890-a123-000000000010",
      "aggregate_version": 1,
      "status": "PUBLISHED",
      "attempt_count": 1,
      "created_at": "2026-11-01T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 25,
    "has_more": false,
    "next_cursor": null
  }
}
```

---

### 3.2 `GET /api/v1/admin/events/outbox/stats`
Retrieves current queue depths and oldest pending event age.

**Success Response (`200 OK`):**
```json
{
  "data": {
    "pending_count": 2,
    "published_count": 1420,
    "failed_count": 0,
    "oldest_pending_age_seconds": 14.5
  }
}
```

---

### 3.3 `POST /api/v1/admin/events/outbox/{id}/retry`
Resets a `FAILED` outbox event to `PENDING` with `attempt_count=0`.

**Path Parameters:**
- `id` (UUID, required): Primary key of the outbox record.

**Success Response (`200 OK`):**
```json
{
  "data": {
    "message": "outbox event reset to PENDING",
    "id": "0192e2b3-7c8a-7890-a123-456789abcdef"
  }
}
```

**Conflict Error (`409 Conflict`):**
```json
{
  "error": {
    "code": "OUTBOX_EVENT_NOT_FAILED",
    "message": "outbox event can only be retried if currently in FAILED state",
    "correlation_id": "0192e2b3-7c8a-7890-a123-999999999999",
    "timestamp": "2026-11-01T12:00:00Z"
  }
}
```

---

### 3.4 `GET /api/v1/admin/events/deliveries`
Returns a unified audit stream of notifications and webhooks. Customer email addresses are masked for privacy (`k***@example.lk`).

**Query Parameters:**
- `kind` (string, optional): `notification` or `webhook`.
- `status` (string, optional): `PENDING`, `IN_PROGRESS`, `SUCCESS`, `FAILED`.
- `limit` (integer, optional, default: 25, max: 100).
- `cursor` (string, optional).

**Success Response (`200 OK`):**
```json
{
  "data": [
    {
      "id": "0192e2b3-7c8a-7890-a123-000000000099",
      "kind": "notification",
      "endpoint_or_recipient": "k***@example.lk",
      "event_id": "0192e2b3-7c8a-7890-a123-000000000001",
      "event_type": "parcel.delivered.v1",
      "status": "SUCCESS",
      "attempt_count": 1,
      "created_at": "2026-11-01T12:00:00Z"
    }
  ],
  "meta": {
    "limit": 25,
    "has_more": false,
    "next_cursor": null
  }
}
```

---

### 3.5 `GET /api/v1/merchant/webhook-deliveries`
Returns deliveries for endpoints belonging to the authenticated merchant.

**Success Response (`200 OK`):**
```json
{
  "data": [
    {
      "id": "0192e2b3-7c8a-7890-a123-000000000077",
      "endpoint_id": "0192e2b3-7c8a-7890-a123-000000000088",
      "event_id": "0192e2b3-7c8a-7890-a123-000000000001",
      "event_type": "parcel.booked.v1",
      "status": "SUCCESS",
      "attempt_count": 1,
      "response_status_code": 200,
      "created_at": "2026-11-01T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 25,
    "has_more": false,
    "next_cursor": null
  }
}
```

---

### 3.6 `GET /api/v1/merchant/webhook-deliveries/{id}/attempts`
Returns the append-only attempt history for an owned delivery. If the delivery belongs to another merchant, `404 Not Found` is returned to prevent IDOR disclosure.

**Success Response (`200 OK`):**
```json
{
  "data": [
    {
      "attempt_no": 1,
      "started_at": "2026-11-01T10:00:01Z",
      "response_status_code": 200,
      "response_body_snippet": "{\"status\":\"received\"}",
      "error_class": null,
      "error_message": null
    }
  ]
}
```

---

## 4. Error Code Reference

| Status | Code | Meaning |
| :--- | :--- | :--- |
| `400` | `INVALID_EVENT_ID` / `INVALID_DELIVERY_ID` | Path parameter is not a valid UUID string. |
| `400` | `INVALID_CURSOR` | Pagination cursor is malformed or invalid base64. |
| `401` | `UNAUTHORIZED` | Bearer token is missing, invalid, or expired. |
| `403` | `TENANT_NOT_MERCHANT_UUID` | JWT tenant claim is not a valid merchant UUID. |
| `403` | `FORBIDDEN` | Caller lacks the required RBAC role. |
| `404` | `OUTBOX_EVENT_NOT_FOUND` | Outbox record does not exist. |
| `404` | `DELIVERY_NOT_FOUND` | Webhook delivery does not exist or belongs to another merchant. |
| `409` | `OUTBOX_EVENT_NOT_FAILED` | Attempted retry on a record whose status is not `FAILED`. |
| `422` | `INVALID_LIMIT` | `limit` query parameter is outside the valid range [1, 100]. |

