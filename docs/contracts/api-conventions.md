# SwiftDrop — REST API Conventions & Contract Standards

> **Authority:** All HTTP services (Core API, Tracking API, AI Assistant Gateway) must comply with these conventions.

---

## 1. URI Conventions & Versioning

- **Base Path:** `/api/v1`
- **Plural Nouns:** Use plural nouns for resources (e.g., `/api/v1/parcels`, `/api/v1/couriers`).
- **Sub-resources:** Represent nested ownership clearly: `/api/v1/parcels/{id}/history`, `/api/v1/couriers/{id}/assignments`.
- **Custom Actions:** For actions that do not map cleanly to CRUD, append a verb: `/api/v1/parcels/{id}/assign`, `/api/v1/parcels/{id}/deliver`.

---

## 2. Standard Request & Response Envelopes

### 2.1 Single Resource Response (200 OK / 201 Created)
```json
{
  "data": {
    "id": "prcl_01HXYZ1234567890",
    "tracking_number": "SD-20261021-99812",
    "status": "OUT_FOR_DELIVERY",
    "sender_id": "usr_01HABC9876543210",
    "recipient": {
      "name": "Jane Doe",
      "phone": "+15551234567",
      "address": "123 Main St, Apt 4B, Springfield"
    },
    "dimensions": {
      "weight_kg": 2.5,
      "length_cm": 30.0,
      "width_cm": 20.0,
      "height_cm": 15.0
    },
    "fee": {
      "currency": "LKR",
      "amount_cents": 145000,
      "formatted": "LKR 1,450.00"
    },
    "created_at": "2026-10-21T08:30:00Z",
    "updated_at": "2026-10-21T11:15:00Z"
  }
}
```

### 2.2 Paginated List Response (200 OK)
Use query parameters `?page=1&per_page=20` (maximum `per_page` is 100).
```json
{
  "data": [
    { "id": "prcl_01", "status": "DELIVERED" },
    { "id": "prcl_02", "status": "IN_TRANSIT" }
  ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total_items": 142,
    "total_pages": 8
  }
}
```

### 2.3 Standard Error Response (4xx / 5xx)
Stack traces and raw database error strings must NEVER be exposed.
```json
{
  "error": {
    "code": "PARCEL_INVALID_TRANSITION",
    "message": "Cannot transition parcel from DELIVERED to OUT_FOR_DELIVERY",
    "correlation_id": "corr_c8a9f24e-7b12-4f33-912a",
    "timestamp": "2026-10-21T13:30:00Z",
    "details": [
      {
        "field": "status",
        "issue": "Legal transitions from DELIVERED: none (terminal state)"
      }
    ]
  }
}
```

---

## 3. Standard HTTP Status Codes

| Code | Usage | SwiftDrop Example |
| :--- | :--- | :--- |
| `200 OK` | Successful read, update, or action | Fetching parcel details, updating profile |
| `201 Created` | Successful creation | Booking a new parcel, registering courier |
| `204 No Content` | Successful deletion or state clear | Removing an unverified device token |
| `400 Bad Request` | Syntactically invalid payload | Invalid JSON syntax, malformed data types |
| `401 Unauthorized` | Missing, invalid, or expired auth token | Calling `/api/v1/parcels` without a Bearer JWT |
| `403 Forbidden` | Authenticated but unauthorized | Courier attempting to view another courier's route |
| `404 Not Found` | Resource does not exist | Parcel ID not found |
| `409 Conflict` | State conflict or concurrent update | Assigning an already-assigned courier |
| `422 Unprocessable`| Semantic domain validation failure | Negative parcel weight, invalid postal code |
| `429 Too Many Req`| Rate limit exceeded | Public tracking queries exceeding threshold |
| `500 Server Error` | Unexpected internal exception | Database connection timeout (returns correlation ID) |

---

## 4. Idempotency Key Specification

For non-idempotent mutation operations (such as parcel booking and merchant bulk creation), clients MUST provide an `Idempotency-Key` header containing a UUID v4.

- The server stores the idempotency key, request hash, and corresponding response in an `idempotency_keys` table with an expiration of 24 hours.
- If a client repeats a request with the same idempotency key and identical payload within 24 hours, the server returns the cached response with header `Idempotent-Replay: true`.
- If a client sends a request with an existing key but a different payload, the server returns `409 Conflict` (`IDEMPOTENCY_KEY_PAYLOAD_MISMATCH`).

---

## 5. Public vs. Private Tracking Data Rules

To protect customer privacy and comply with security requirements:

| Field | Private Tracking (Sender / Recipient / Courier) | Public Tracking (`/api/v1/tracking/public/{number}`) |
| :--- | :--- | :--- |
| **Tracking Number** | Yes | Yes |
| **Current Status** | Yes | Yes |
| **Status Timestamp**| Yes | Yes |
| **Delivery Milestones** | Detailed (timestamps, hub names, courier ID) | Generalized (e.g., "Arrived at Regional Hub") |
| **Sender Name/Address**| Full details | Completely Redacted |
| **Recipient Address** | Full street address and apartment | City and State/Region only (e.g., "Chicago, IL") |
| **Recipient Phone** | Unmasked or partially masked | Completely Redacted |
| **Proof of Delivery** | High-res photo / signature image | Status marked "Delivered - Proof Verified" (no image) |
| **Internal Notes** | Visible to Courier / Dispatcher | Completely Redacted |

---

## 6. API Documentation Standard (OpenAPI & Swagger Annotations)

All HTTP endpoints across Core API are automatically documented in a unified interactive Swagger UI interface served at `/swagger/`.

### 6.1 Mandatory Annotation Rule
Every Go HTTP handler must include standard `swag` docstring annotations directly above the handler function:
- `@Summary`: Brief 1-line description of the operation.
- `@Description`: Detailed explanation including authorization and business constraints.
- `@Tags`: Module taxonomy (`identity`, `merchants`, `events`, `parcels`, `pricing`, `dispatch`, `delivery`, `admin`, `system`).
- `@Accept` and `@Produce`: MIME types (typically `json`).
- `@Param`: Input parameters (body schemas, path variables, query options).
- `@Success`: Status code and response model struct.
- `@Failure`: Handled error status codes and RFC-7807/httpx error envelopes.
- `@Security`: `BearerAuth` for any authenticated or role-restricted endpoint.
- `@Router`: Exact URI path and HTTP method in brackets (e.g. `/api/v1/auth/login [post]`).

### 6.2 Generator Execution & Sync
Developers and AI agents must run `make swagger` to regenerate `core-api/docs/swagger.json` synchronously whenever adding, editing, or deleting HTTP routes. See [`docs/contracts/swagger-guide.md`](file:///c:/Users/kesha/OneDrive/Documents/GitHub/SwiftDrop/docs/contracts/swagger-guide.md) for full syntax and conventions.

