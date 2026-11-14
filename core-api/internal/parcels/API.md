# Parcel Management API Specifications (`/api/v1/parcels`)

> **Module:** `internal/parcels` | **Owner:** Member 2 — Parcel Management & Pricing | **Status:** Booking, parcel detail, history, and receipts implemented. List and cancellation deferred.

---

## 1. Book Parcel
Creates a new parcel booking for an authorized caller (`CUSTOMER`, approved `MERCHANT`, or `ADMIN`) by atomically consuming a valid, unexpired quote issued via [`POST /api/v1/pricing/quotes`](../pricing/API.md), allocating a Crockford Base32 tracking number (`SD-YYYYMMDD-XXXXXX`), persisting initial `CREATED` status history and a `parcel.booked.v1` outbox event, and storing a caller-scoped idempotency snapshot. Direct unquoted booking is not supported.

- **Method / Path:** `POST /api/v1/parcels`
- **Auth Required:** **Bearer JWT required** (`CUSTOMER`, `MERCHANT`, `ADMIN`)
- **Headers:**
  - `Authorization: Bearer <jwt>` (Mandatory)
  - `Content-Type: application/json` (Mandatory when sent; empty treated as JSON, non-JSON rejected with `415`)
  - `Idempotency-Key: <uuid>` (Mandatory valid UUID; trimmed and normalized to lowercase, 24-hour TTL scoped by `USER:<user_uuid>` or `MERCHANT:<merchant_uuid>`)

### Authorization, Ownership & Security Rules
- Request bodies are bounded to **64 KB** (`413 PAYLOAD_TOO_LARGE`) and decoded strictly with unknown fields disallowed (`400 INVALID_REQUEST_PAYLOAD`).
- Client-supplied `sender_id`, `merchant_id`, `delivery_fee_cents`, `pricing_rule_id`, `pricing_rule_version`, `service_level`, `status`, or `tracking_number` fields are rejected (`400 INVALID_REQUEST_PAYLOAD`). Delivery fees, tariff version, and service level come exclusively from the consumed quote.
- **`CUSTOMER` / `ADMIN`:** Resolves `SenderID = Principal.UserID` (valid UUID) and `MerchantID = nil` (self-owned parcel policy).
- **`MERCHANT`:** Enforces Decision D25 and authoritative lookup via `MerchantReader`:
  1. `Principal.TenantID` must be present and non-blank (`403 MISSING_TENANT_ID`).
  2. `Principal.TenantID` must be a valid UUID (`403 TENANT_NOT_MERCHANT_UUID`).
  3. Merchant record must exist in `merchants` (`403 MERCHANT_NOT_FOUND`).
  4. `merchants.owner_user_id` must equal `Principal.UserID` (`403 MERCHANT_OWNER_MISMATCH`).
  5. `merchants.status` must equal `APPROVED` (`403 MERCHANT_NOT_APPROVED`).
  6. Binds `SenderID = Principal.UserID` and `MerchantID = merchants.id`.
- **`COURIER` / `DISPATCHER`:** Rejected with `403`.
- **Idempotency Replay:** Repeat `POST /api/v1/parcels` with the same UUID `Idempotency-Key`, caller scope, and byte-for-byte identical JSON body within 24 hours returns the cached `201 Created` response with `Idempotent-Replay: true` without re-consuming the quote or creating a duplicate parcel. Reusing the key with different payload bytes returns `409 IDEMPOTENCY_KEY_PAYLOAD_MISMATCH`.

### Request Body
```json
{
  "quote_id": "0192e2b3-7c8a-7890-a123-000000000777",
  "recipient_name": "Nimal Fernando",
  "recipient_phone": "+94719876543",
  "weight_kg": 2.50,
  "dimensions": {
    "length_cm": 30.0,
    "width_cm": 25.0,
    "height_cm": 15.0
  },
  "pickup": {
    "contact_name": "Kasun Perera",
    "contact_phone": "+94771234567",
    "address_line1": "45 Galle Road",
    "address_line2": "Suite 3B",
    "city": "Colombo 03",
    "postal_code": "00300",
    "zone_id": "ZONE_WESTERN_01",
    "instructions": "Ring intercom 3B"
  },
  "dropoff": {
    "contact_name": "Nimal Fernando",
    "contact_phone": "+94719876543",
    "address_line1": "12 Kandy Road",
    "address_line2": "",
    "city": "Peradeniya",
    "postal_code": "20400",
    "zone_id": "ZONE_CENTRAL_01",
    "instructions": "Leave at front security desk"
  },
  "item_description": "Electronics - Replacement Laptop Battery",
  "special_instructions": "Handle with care"
}
```
*Field Constraints:*
- `quote_id`: Mandatory valid quote UUID matching an active, unexpired quote owned by the caller.
- `recipient_name`, `recipient_phone`: Mandatory non-blank strings.
- `weight_kg`: Positive finite weight in kg matching the quote's chargeable weight (`max(actual, (L*W*H)/5000) <= 30.00 kg`).
- `dimensions`: Mandatory object with positive finite `length_cm`, `width_cm`, `height_cm`.
- `pickup`, `dropoff`: Mandatory objects with non-blank `contact_name`, `contact_phone`, `address_line1`, `city`, `postal_code`, and `zone_id` matching `^ZONE_[A-Z0-9_]{2,27}$` and matching the quote's origin and destination zones.

### Success Response (`201 Created`)
Wrapped in the canonical `httpx.WriteData` single-resource envelope (`{"data": ...}`):
```json
{
  "data": {
    "id": "0192e2b3-7c8a-7890-a123-000000000101",
    "tracking_number": "SD-20261111-000001",
    "sender_id": "018f4a7c-1005-7000-8000-000000000005",
    "recipient_name": "Nimal Fernando",
    "recipient_phone": "+94719876543",
    "quote_id": "0192e2b3-7c8a-7890-a123-000000000777",
    "pricing_rule_id": "rule_west_central_v2",
    "pricing_rule_version": 2,
    "delivery_fee_cents": 47000,
    "currency": "LKR",
    "origin_zone_id": "ZONE_WESTERN_01",
    "destination_zone_id": "ZONE_CENTRAL_01",
    "service_level": "STANDARD",
    "weight": {
      "actual_kg": 2.5,
      "volumetric_kg": 2.25,
      "chargeable_kg": 2.5
    },
    "dimensions": {
      "length_cm": 30.0,
      "width_cm": 25.0,
      "height_cm": 15.0
    },
    "pickup": {
      "contact_name": "Kasun Perera",
      "contact_phone": "+94771234567",
      "address_line1": "45 Galle Road",
      "address_line2": "Suite 3B",
      "city": "Colombo 03",
      "postal_code": "00300",
      "zone_id": "ZONE_WESTERN_01",
      "instructions": "Ring intercom 3B"
    },
    "dropoff": {
      "contact_name": "Nimal Fernando",
      "contact_phone": "+94719876543",
      "address_line1": "12 Kandy Road",
      "address_line2": "",
      "city": "Peradeniya",
      "postal_code": "20400",
      "zone_id": "ZONE_CENTRAL_01",
      "instructions": "Leave at front security desk"
    },
    "status": "CREATED",
    "attempt_count": 0,
    "max_attempts": 3,
    "version": 1,
    "created_at": "2026-11-11T10:00:00Z",
    "updated_at": "2026-11-11T10:00:00Z"
  }
}
```
*(Approved `MERCHANT` bookings also include `"merchant_id": "<merchant-uuid>"`. Idempotent replay responses include header `Idempotent-Replay: true`.)*

### Error Responses
Handler errors use the canonical `httpx.WriteError` envelope:
```json
{
  "error": {
    "code": "QUOTE_PARAM_MISMATCH",
    "message": "Parcel weight, dimensions, or address zones do not match the referenced pricing quote",
    "correlation_id": "req-123",
    "timestamp": "2026-11-11T10:00:00Z"
  }
}
```
*(Middleware-level `401`/`403` rejections from `identity`'s `SecurityMiddleware` / `RequireRole` emit `{"error": {"code": "...", "message": "..."}}`.)*

- `400 Bad Request`: `MISSING_IDEMPOTENCY_KEY`, `INVALID_IDEMPOTENCY_KEY`, `INVALID_REQUEST_PAYLOAD`, `INVALID_IDEMPOTENCY_METADATA`.
- `401 Unauthorized`: `AUTH_UNAUTHORIZED`, `AUTH_MALFORMED_HEADER`, `AUTH_INVALID_TOKEN`, `AUTH_TOKEN_EXPIRED`.
- `403 Forbidden`: `AUTH_FORBIDDEN`, `PARCEL_FORBIDDEN`, `INVALID_CALLER_ID`, `MISSING_TENANT_ID`, `TENANT_NOT_MERCHANT_UUID`, `MERCHANT_NOT_FOUND`, `MERCHANT_OWNER_MISMATCH`, `MERCHANT_NOT_APPROVED`, `QUOTE_FORBIDDEN`.
- `404 Not Found`: `QUOTE_NOT_FOUND`.
- `409 Conflict`: `IDEMPOTENCY_KEY_PAYLOAD_MISMATCH`, `IDEMPOTENCY_CONFLICT`, `QUOTE_ALREADY_USED`.
- `413 Payload Too Large`: `PAYLOAD_TOO_LARGE` (body > 64 KB).
- `415 Unsupported Media Type`: `UNSUPPORTED_MEDIA_TYPE` (non-`application/json` `Content-Type`).
- `422 Unprocessable Entity`: `INVALID_QUOTE_ID`, `QUOTE_EXPIRED`, `QUOTE_PARAM_MISMATCH`, `INVALID_ZONE_ID`, `ZONE_MISMATCH`, `INVALID_DIMENSIONS`, `INVALID_WEIGHT`, `PARCEL_LIMITS_EXCEEDED`, `INVALID_RECIPIENT`, `INVALID_ADDRESS`, `INVALID_SERVICE_LEVEL`, `INVALID_PARCEL`.
- `500 Internal Server Error`: `INTERNAL_ERROR`.
- `503 Service Unavailable`: `TRACKING_NUMBER_UNAVAILABLE` (tracking number allocation collision retries exhausted; safe to retry with the same `Idempotency-Key`).

---

## 2. Get Parcel Details
Retrieves authoritative parcel details for authorized callers: `CUSTOMER` (own parcels only), approved `MERCHANT` (own tenant parcels only), or `ADMIN` (unrestricted). `COURIER`, `DISPATCHER`, and non-approved merchants receive `403 Forbidden`.

- **Method / Path:** `GET /api/v1/parcels/{id}`
- **Auth Required:** Bearer JWT (`CUSTOMER`, `MERCHANT` [`APPROVED`], `ADMIN`)
- **Path Parameters:**
  - `id` (string, required): Canonical Parcel UUID.
- **Headers:**
  - `Authorization: Bearer <jwt>` (Mandatory)

### Success Response (`200 OK`)
Returns `{"data": ParcelResponse}` using the identical `ParcelResponse` schema shown in Section 1.

### Error Responses
- `400 Bad Request`: `INVALID_PARCEL_ID` — Path parameter `id` is not a valid UUID.
- `401 Unauthorized`: `AUTH_UNAUTHORIZED`, `AUTH_MALFORMED_HEADER`, `AUTH_INVALID_TOKEN`, `AUTH_TOKEN_EXPIRED`.
- `403 Forbidden`: `AUTH_FORBIDDEN` (`COURIER`/`DISPATCHER`), `PARCEL_FORBIDDEN` (cross-customer/cross-merchant IDOR), `INVALID_CALLER_ID`, `MISSING_TENANT_ID`, `TENANT_NOT_MERCHANT_UUID`, `MERCHANT_NOT_FOUND`, `MERCHANT_OWNER_MISMATCH`, `MERCHANT_NOT_APPROVED`.
- `404 Not Found`: `PARCEL_NOT_FOUND` — No parcel exists for the supplied UUID.
- `500 Internal Server Error`: `INTERNAL_ERROR`.

---

## 3. Get Status History
Retrieves chronological lifecycle status history entries from `parcel_status_history` after enforcing caller role, approved merchant status, and object-level parcel ownership before any history rows are queried.

- **Method / Path:** `GET /api/v1/parcels/{id}/history`
- **Auth Required:** Bearer JWT (`CUSTOMER`, `MERCHANT` [`APPROVED`], `ADMIN`)
- **Path Parameters:**
  - `id` (string, required): Canonical Parcel UUID.
- **Headers:**
  - `Authorization: Bearer <jwt>` (Mandatory)

### Success Response (`200 OK`)
Wrapped in `httpx.WriteData` (`{"data": ...}`):
```json
{
  "data": {
    "parcel_id": "0192e2b3-7c8a-7890-a123-000000000101",
    "tracking_number": "SD-20261111-000001",
    "status": "ASSIGNED",
    "history": [
      {
        "id": "0192e2b3-7c8a-7890-a123-000000000501",
        "parcel_id": "0192e2b3-7c8a-7890-a123-000000000101",
        "from_status": null,
        "to_status": "CREATED",
        "actor_id": "018f4a7c-1005-7000-8000-000000000005",
        "actor_type": "USER",
        "actor_role": "CUSTOMER",
        "reason": "Initial parcel booking",
        "occurred_at": "2026-11-11T10:00:00Z"
      },
      {
        "id": "0192e2b3-7c8a-7890-a123-000000000599",
        "parcel_id": "0192e2b3-7c8a-7890-a123-000000000101",
        "from_status": "CREATED",
        "to_status": "ASSIGNED",
        "actor_id": "018f4a7c-1001-7000-8000-000000000001",
        "actor_type": "USER",
        "actor_role": "ADMIN",
        "reason": "Dispatched to courier",
        "occurred_at": "2026-11-11T10:15:00Z"
      }
    ]
  }
}
```
*(Initial booking transitions explicitly serialize `"from_status": null`. Empty history arrays serialize as `"history": []`.)*

### Error Responses
- `400 Bad Request`: `INVALID_PARCEL_ID`.
- `401 Unauthorized`: `AUTH_UNAUTHORIZED`, `AUTH_MALFORMED_HEADER`, `AUTH_INVALID_TOKEN`, `AUTH_TOKEN_EXPIRED`.
- `403 Forbidden`: `AUTH_FORBIDDEN`, `PARCEL_FORBIDDEN`, `INVALID_CALLER_ID`, `MISSING_TENANT_ID`, `TENANT_NOT_MERCHANT_UUID`, `MERCHANT_NOT_FOUND`, `MERCHANT_OWNER_MISMATCH`, `MERCHANT_NOT_APPROVED`.
- `404 Not Found`: `PARCEL_NOT_FOUND`.
- `500 Internal Server Error`: `INTERNAL_ERROR`.

---

## 4. Known Cross-Module Contract Mismatches & Wiring Notes
- **Startup Wiring (`cmd/api`):** Route registration (`RegisterRoutes`) and HTTP handlers are implemented and verified in `internal/parcels/transport`, pending dependency injection wiring in `cmd/api/wiring.go` in the startup integration milestone.
- **Member 4 (`internal/merchants` `BookingStub`):** Member 4's merchant order stub remains untouched in this milestone.
- **Member 6 (`web/API.md`) & Earlier Pre-Implementation Draft Differences:** Earlier planning drafts used unwrapped responses, nested `"price": {"price_cents": ...}`, hyphenated `"ZONE-*"` IDs, client-supplied `"service_level"`, and omitted top-level `recipient_name`/`recipient_phone`. The authoritative implementation wraps responses in `{"data": ...}`, uses flat `delivery_fee_cents`/`pricing_rule_id`/`pricing_rule_version` + `weight.*`, requires `^ZONE_[A-Z0-9_]{2,27}$` underscores, derives `service_level` authoritatively from `quote_id`, and rejects unknown JSON fields and non-UUID `Idempotency-Key` values.
- **Member 10 (`services/ai-assistant`):** Any AI tool reading parcel details or booking parcels must pass Bearer JWT headers, unwrap `{"data": ...}`, and conform to the documented field schemas.

---

## 5. Get Booking Receipt

`GET /api/v1/parcels/{id}/receipt` uses the same Bearer authentication, ownership checks, and errors as Get Parcel. It returns `200 OK` with the following `data` envelope and a `Cache-Control: no-store` header.

```json
{
  "data": {
    "receipt_number": "RCP-SD-20261025-A3X9K7",
    "parcel_id": "7f8b3c10-e29b-4b14-9f44-8d961234ef01",
    "tracking_number": "SD-20261025-A3X9K7",
    "booking_date": "2026-10-25T14:32:00Z",
    "customer_name": "Kasun Perera",
    "service_level": "STANDARD",
    "financial_summary": {
      "currency": "LKR",
      "price_cents": 55000,
      "total_fee_formatted": "LKR 550.00",
      "pricing_rule_id": "pr-rule-zone-w-c-v2",
      "pricing_rule_version": 2
    },
    "package_summary": {
      "chargeable_weight_kg": 2.5,
      "origin_zone": "ZONE_WESTERN_01",
      "destination_zone": "ZONE_CENTRAL_01"
    },
    "payment_status": "NOT_TRACKED"
  }
}
```

The name comes from the booking's pickup contact. The amount and pricing rule come from the accepted booking. The parcel module does not track payment settlement.

## 6. Deferred Endpoints (Planned — Not Yet Implemented in HTTP Transport)

> [!NOTE]
> **Implementation Status Notice:**
> The routes below are deferred from the current HTTP transport milestone and are **not** registered in `internal/parcels/transport/routes.go`.

- `GET /api/v1/parcels`: Paginated list of parcels visible to caller (`CUSTOMER`, `MERCHANT`, `ADMIN`).
- `POST /api/v1/parcels/{id}/cancel`: Cancel parcel prior to pickup (`CREATED -> CANCELLED`; application lifecycle support already exists in `TransitionParcelStatusUseCase`).
