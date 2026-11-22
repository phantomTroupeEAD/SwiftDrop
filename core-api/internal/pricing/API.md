# Pricing Module API Specifications (`/api/v1/pricing`)

> **Module:** `internal/pricing` | **Owner:** Member 2 — Parcel Management & Pricing | **Status:** Implemented HTTP Transport (Ready for Startup Wiring)

---

## 1. Create Pricing Quote
Calculates the complete delivery fee using active geographic tariffs, volumetric weight, and service-level multipliers, and persists a single-use quote in `pricing_quotes` with a 15-minute price freeze window (`expires_at = created_at + 15m`).

- **Method / Path:** `POST /api/v1/pricing/quotes`
- **Auth Required:** **Bearer JWT required** (`CUSTOMER`, `MERCHANT`, `ADMIN`)
- **Headers:**
  - `Authorization: Bearer <jwt>` (Mandatory)
  - `Content-Type: application/json` (Mandatory when sent; empty treated as JSON, non-JSON rejected with `415`)

### Ownership Resolution & Security Rules
- Request bodies are decoded strictly with unknown fields disallowed; client-supplied `requester_id` or `merchant_id` fields are rejected (`400 INVALID_REQUEST_PAYLOAD`).
- **`CUSTOMER` / `ADMIN`:** Resolves `RequesterID = Principal.UserID` (UUID) with `MerchantID = nil` (self-owned quote policy).
- **`MERCHANT`:** Enforces Decision D25 and authoritative database verification via `MerchantReader`:
  1. `Principal.TenantID` must be present and non-blank (`403 MISSING_TENANT_ID`).
  2. `Principal.TenantID` must be a valid UUID (`403 TENANT_NOT_MERCHANT_UUID`).
  3. Merchant record must exist in `merchants` (`403 MERCHANT_NOT_FOUND`).
  4. `merchants.owner_user_id` must equal `Principal.UserID` (`403 MERCHANT_OWNER_MISMATCH`).
  5. `merchants.status` must equal `APPROVED` (`403 MERCHANT_NOT_APPROVED`).
  6. Binds both `RequesterID = Principal.UserID` and `MerchantID = merchants.id`.

### Request Body
```json
{
  "origin_zone_id": "ZONE_WESTERN_01",
  "destination_zone_id": "ZONE_CENTRAL_01",
  "actual_weight_kg": 2.50,
  "dimensions": {
    "length_cm": 30.0,
    "width_cm": 25.0,
    "height_cm": 15.0
  },
  "service_level": "STANDARD"
}
```
*Field Constraints:*
- `origin_zone_id`, `destination_zone_id`: Must match canonical format `^ZONE_[A-Z0-9_]{2,27}$` (uppercase letters, digits, and underscores).
- `actual_weight_kg`: Positive finite decimal in kilograms (`chargeable_weight_kg` capped at `30.00 kg`).
- `dimensions`: Mandatory object; `length_cm`, `width_cm`, `height_cm` must be positive finite numbers in centimeters. Volumetric weight uses divisor `5000 cm³/kg`.
- `service_level`: One of `STANDARD` (`1.0x`), `EXPRESS` (`1.5x`), or `SAME_DAY` (`2.0x`).

### Success Response (`201 Created`)
Wrapped in the canonical `httpx.WriteData` single-resource envelope (`{"data": ...}`):
```json
{
  "data": {
    "quote_id": "0192e2b3-7c8a-7890-a123-000000000777",
    "pricing_rule_id": "rule_west_central_v2",
    "pricing_rule_version": 2,
    "requester_id": "018f4a7c-1005-7000-8000-000000000005",
    "chargeable_weight_kg": 2.5,
    "service_level": "STANDARD",
    "fee_cents": 47000,
    "currency": "LKR",
    "created_at": "2026-11-11T10:00:00Z",
    "expires_at": "2026-11-11T10:15:00Z"
  }
}
```
*(For approved `MERCHANT` quotes, `"merchant_id": "<merchant-uuid>"` is also included alongside `"requester_id"`.)*

> [!NOTE]
> **Approved Option A Pricing Architecture:**
> Delivery fee is calculated using geographic tariff (base fee covering first 1 kg + proportional additional weight rate) and service-level multiplier:
> `ExtraKg = max(0, ChargeableWeightKg - 1)`
> `WeightFee = ExtraKg * PerKgRate`
> `FinalFee = (BaseFee + WeightFee) * ServiceMultiplier`
> Service multipliers: `STANDARD = 1.0x (1/1)`, `EXPRESS = 1.5x (3/2)`, `SAME_DAY = 2.0x (2/1)`.
> Volumetric divisor is `5000 cm³/kg`. `ChargeableWeightKg = max(actual, volumetric)`, capped at `30.00 kg`. Quote validity is 15 minutes (`expires_at = created_at + 15m`; `current_time >= expires_at` is expired).

### Error Responses
Handler errors use the canonical `httpx.WriteError` envelope:
```json
{
  "error": {
    "code": "INVALID_ZONE_ID",
    "message": "Origin and destination zones must follow the canonical ZONE_* format",
    "correlation_id": "req-123",
    "timestamp": "2026-11-11T10:00:00Z",
    "details": [
      {
        "field": "zone_id",
        "issue": "must match ^ZONE_[A-Z0-9_]{2,27}$"
      }
    ]
  }
}
```
*(Middleware-level `401`/`403` rejections from `identity`'s `SecurityMiddleware` / `RequireRole` emit `{"error": {"code": "...", "message": "..."}}`.)*

- `400 Bad Request`: `INVALID_REQUEST_PAYLOAD` — Malformed JSON, empty body, trailing JSON, wrong JSON field types, or unknown/disallowed fields (`requester_id`, `merchant_id`, `fee_cents`).
- `401 Unauthorized`: `AUTH_UNAUTHORIZED`, `AUTH_MALFORMED_HEADER`, `AUTH_INVALID_TOKEN`, `AUTH_TOKEN_EXPIRED` — Missing or invalid Bearer JWT.
- `403 Forbidden`:
  - `AUTH_FORBIDDEN` — Role not in `CUSTOMER`, `MERCHANT`, `ADMIN` (e.g., `COURIER`, `DISPATCHER`).
  - `INVALID_CALLER_ID` — Authenticated `UserID` is not a valid UUID.
  - `MISSING_TENANT_ID` — `MERCHANT` token is missing `tenant_id` (Decision D25).
  - `TENANT_NOT_MERCHANT_UUID` — `MERCHANT` token `tenant_id` is not a valid UUID (e.g., seeded `mrc_*` slug).
  - `MERCHANT_NOT_FOUND` — No `merchants` row exists for the claimed `tenant_id`.
  - `MERCHANT_OWNER_MISMATCH` — Authenticated user does not match `merchants.owner_user_id`.
  - `MERCHANT_NOT_APPROVED` — Merchant tenant status is not `APPROVED` (`PENDING`, `REJECTED`, `SUSPENDED`, `DISABLED`).
- `413 Payload Too Large`: `PAYLOAD_TOO_LARGE` — Request JSON exceeds 64 KB.
- `415 Unsupported Media Type`: `UNSUPPORTED_MEDIA_TYPE` — Non-`application/json` `Content-Type` header.
- `422 Unprocessable Entity`:
  - `INVALID_ZONE_ID` — Origin or destination zone does not match `^ZONE_[A-Z0-9_]{2,27}$`.
  - `INVALID_SERVICE_LEVEL` — Unsupported service level.
  - `INVALID_WEIGHT` — Non-positive or non-finite `actual_weight_kg`.
  - `INVALID_DIMENSIONS` — Missing `dimensions` object or non-positive/non-finite dimensions.
  - `PARCEL_LIMITS_EXCEEDED` — Chargeable weight exceeds `30.00 kg`.
  - `PRICING_UNAVAILABLE` — No active pricing rule covers the requested zone pair at evaluation time. Never fabricates a fee.
  - `INVALID_QUOTE` / `INVALID_MONEY` — Quote or money invariant failure.
- `500 Internal Server Error`: `MERCHANT_LOOKUP_FAILED`, `PRICING_CONFIGURATION_ERROR`, `PRICING_CALCULATION_ERROR`, `INTERNAL_ERROR`.

### Known Cross-Module Contract Mismatches (Pending Consumer Alignment)
- **Member 6 (Web Frontend — `web/API.md`):** Currently documents optional auth, `200 OK` status, `pickup_zone_id`/`dropoff_zone_id` and flat `length_cm`/`width_cm`/`height_cm` request fields, and `data.total_fee_cents` / `data.base_fee_cents` / `data.weight_surcharge_cents`. Core API instead enforces Bearer JWT auth, `201 Created`, nested `dimensions`, `origin_zone_id`/`destination_zone_id`, and `data.fee_cents`.
- **Member 10 (AI Assistant — `services/ai-assistant/src/tools/pricing_tool.py`):** Currently sends unauthenticated requests with `pickup_postal_code`/`delivery_postal_code`/`weight_kg`/`service_type` and reads top-level `total_fee_cents`, `base_fee_cents`, and `weight_fee_cents` without unwrapping `data`. Requires consumer-side alignment to forward caller Bearer JWT, supply `ZONE_*` + nested `dimensions`, and unwrap `data.fee_cents` (unless an itemized breakdown DTO or dedicated unauthenticated estimation contract is approved later).

---

## 2. Admin Pricing Tariff Rules Management

### List Pricing Rules
Retrieves a paginated list of geographic pricing rules (tariffs) from PostgreSQL for administrative oversight, filtering, and audit.

- **Method / Path:** `GET /api/v1/admin/pricing-rules`
- **Auth Required:** **Bearer JWT required** (`ADMIN` role strictly enforced)
- **Headers:**
  - `Authorization: Bearer <jwt>` (Mandatory)

### Query Parameters
| Parameter | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `page` | Integer | No | `1` | Page number ($\ge 1$). |
| `per_page` | Integer | No | `20` | Page size ($1 \le \text{per\_page} \le 100$). |
| `origin_zone_id` | String | No | - | Optional filter matching canonical `^ZONE_[A-Z0-9_]{2,27}$`. |
| `destination_zone_id` | String | No | - | Optional filter matching canonical `^ZONE_[A-Z0-9_]{2,27}$`. |
| `is_active` | Boolean | No | - | Optional filter (`true` or `false`). |

### Success Response (`200 OK`)
Wrapped in the canonical `httpx.NewPaginatedResponse` list envelope (`{"data": [...], "meta": {...}}`):
```json
{
  "data": [
    {
      "id": "rule_west_central_v2",
      "version": 2,
      "origin_zone_id": "ZONE_WESTERN_01",
      "destination_zone_id": "ZONE_CENTRAL_01",
      "base_weight_kg": 1.0,
      "max_weight_kg": 30.0,
      "base_fee_cents": 35000,
      "per_kg_rate_cents": 8000,
      "currency": "LKR",
      "effective_from": "2026-11-10T00:00:00Z",
      "effective_until": null,
      "is_active": true,
      "created_at": "2026-11-10T00:00:00Z",
      "updated_at": "2026-11-10T00:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total_items": 1,
    "total_pages": 1
  }
}
```

### Error Responses
- `401 Unauthorized`: `AUTH_UNAUTHORIZED` — Missing or invalid Bearer JWT.
- `403 Forbidden`: `AUTH_FORBIDDEN` — Authenticated role is not `ADMIN` (e.g. `CUSTOMER`, `MERCHANT`, `COURIER`, `DISPATCHER`).
- `422 Unprocessable Entity`:
  - `VALIDATION_FAILED` — Invalid query parameter values (e.g. `page < 1`, `per_page < 1`, `per_page > 100`, or non-boolean `is_active`).
  - `INVALID_ZONE_ID` — `origin_zone_id` or `destination_zone_id` query filter does not match `^ZONE_[A-Z0-9_]{2,27}$`.
- `500 Internal Server Error`: `INTERNAL_ERROR` — Database query failure.

---

### Candidate Administrative Endpoints (Deferred / Not Yet Implemented)
> [!NOTE]
> Create, Update, and Delete endpoints remain deferred for future iterations per focused Member 2 listing scope.
- `POST /api/v1/admin/pricing-rules`: Create a new tariff rule for a zone-pair.
- `GET /api/v1/admin/pricing-rules/{id}`: Retrieve detailed pricing rule definition.
- `PUT /api/v1/admin/pricing-rules/{id}`: Update tariff fee or validity window (creates new rule version).
- `DELETE /api/v1/admin/pricing-rules/{id}`: Deactivate tariff rule.
