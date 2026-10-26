# Parcel Management API Specifications (`/api/v1/parcels`)

> **Module:** `internal/parcels` | **Owner:** Member 2 — Parcel Management & Pricing | **Status:** Aligned (Post-Audit v2.0.0 Alignment)

---

## 1. Book Parcel
Creates a new parcel booking for the authenticated sender (Customer or Merchant). Supports both quoted booking (with unexpired `quote_id`) and direct booking.

- **Method / Path:** `POST /api/v1/parcels`
- **Auth Required:** Bearer JWT (`CUSTOMER`, `MERCHANT`, `ADMIN`)
- **Headers:**
  - `Authorization: Bearer <jwt>`
  - `Content-Type: application/json`
  - `Idempotency-Key: <uuid-or-string>` (Mandatory, max 255 chars)

### Request Body
```json
{
  "quote_id": "d3b07384-d113-494b-9c8e-aa672580c822",
  "service_level": "STANDARD",
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
    "address_line2": "Apartment 3B",
    "city": "Colombo 03",
    "postal_code": "00300",
    "zone_id": "ZONE-WESTERN-01",
    "instructions": "Ring intercom 3B"
  },
  "dropoff": {
    "contact_name": "Nimal Fernando",
    "contact_phone": "+94719876543",
    "address_line1": "12 Kandy Road",
    "address_line2": "",
    "city": "Peradeniya",
    "postal_code": "20400",
    "zone_id": "ZONE-CENTRAL-01",
    "instructions": "Leave at front security desk"
  },
  "item_description": "Electronics - Replacement Laptop Battery",
  "special_instructions": "Handle with care. Fragile."
}
```

### Success Response (`201 Created`)
```json
{
  "id": "7f8b3c10-e29b-4b14-9f44-8d961234ef01",
  "tracking_number": "SD-20261025-A3X9K7",
  "customer_id": "usr-8812c9b4-3312-4aa8-b112-998877665544",
  "status": "CREATED",
  "service_level": "STANDARD",
  "weight_kg": 2.50,
  "dimensions": {
    "length_cm": 30.0,
    "width_cm": 25.0,
    "height_cm": 15.0
  },
  "chargeable_weight_kg": 2.50,
  "price": {
    "currency": "LKR",
    "price_cents": 55000,
    "pricing_rule_id": "pr-rule-zone-w-c-v2",
    "pricing_rule_version": 2
  },
  "pickup": {
    "contact_name": "Kasun Perera",
    "contact_phone": "+94771234567",
    "address_line1": "45 Galle Road",
    "address_line2": "Apartment 3B",
    "city": "Colombo 03",
    "postal_code": "00300",
    "zone_id": "ZONE-WESTERN-01",
    "instructions": "Ring intercom 3B"
  },
  "dropoff": {
    "contact_name": "Nimal Fernando",
    "contact_phone": "+94719876543",
    "address_line1": "12 Kandy Road",
    "address_line2": "",
    "city": "Peradeniya",
    "postal_code": "20400",
    "zone_id": "ZONE-CENTRAL-01",
    "instructions": "Leave at front security desk"
  },
  "attempt_count": 0,
  "max_attempts": 3,
  "version": 1,
  "created_at": "2026-10-25T14:32:00Z",
  "updated_at": "2026-10-25T14:32:00Z"
}
```

> [!NOTE]
> **Pricing Contract Authority:**
> Complete pricing formulas, weight band boundaries, Option A threshold rules, and tariff lookup behavior are defined authoritatively in [`core-api/internal/pricing/API.md`](file:///d:/SwiftDrop/SwiftDrop/core-api/internal/pricing/API.md). The parcel booking endpoint validates the quote reference, verifies backend expiration (`current_time >= expires_at`), ensures weight/dimension inputs match the quote snapshot, and persists an immutable price snapshot (`price_cents`, currency, rule version).

### Error Responses
- `400 Bad Request`: `MISSING_IDEMPOTENCY_KEY` or `MALFORMED_JSON`.
- `401 Unauthorized`: Missing or invalid Bearer token.
- `403 Forbidden`: `QUOTE_FORBIDDEN` — Quote was generated for a different customer/tenant context.
- `409 Conflict`: `IDEMPOTENCY_KEY_PAYLOAD_MISMATCH` — Reusing idempotency key with different payload body.
- `422 Unprocessable Entity`: `QUOTE_EXPIRED` (Quote older than 15m; `current_time >= expires_at`), `QUOTE_PARAM_MISMATCH` (inputs altered), `PARCEL_LIMITS_EXCEEDED` (actual or chargeable weight $> 30.00\text{ kg}$), `DIMENSIONS_REQUIRED_ABOVE_1KG` (missing dimensions when weight $> 1\text{ kg}$), or `PRICING_UNAVAILABLE` (no tariff rule found).

---

## 2. Get Parcel Details
Retrieves authoritative parcel details for authorized parties (Customer [own parcel], Merchant [own tenant], Courier [assigned], Admin).

- **Method / Path:** `GET /api/v1/parcels/{id}`
- **Auth Required:** Bearer JWT
- **Headers:** `Authorization: Bearer <jwt>`

### Success Response (`200 OK`)
Returns the complete parcel response payload as described in Section 1.

---

## 3. List Visible Parcels
Retrieves a paginated list of parcels visible to the calling principal.

- **Method / Path:** `GET /api/v1/parcels`
- **Auth Required:** Bearer JWT (`CUSTOMER`, `MERCHANT`, `ADMIN`)
- **Query Parameters:**
  - `status` (string, optional): Filter by status (`CREATED`, `IN_TRANSIT`, etc.).
  - `page` (integer, default 1): Page number.
  - `page_size` (integer, default 20, max 100): Results per page.

### Success Response (`200 OK`)
```json
{
  "items": [
    {
      "id": "7f8b3c10-e29b-4b14-9f44-8d961234ef01",
      "tracking_number": "SD-20261025-A3X9K7",
      "status": "CREATED",
      "service_level": "STANDARD",
      "price_cents": 55000,
      "currency": "LKR",
      "pickup_city": "Colombo 03",
      "dropoff_city": "Peradeniya",
      "created_at": "2026-10-25T14:32:00Z"
    }
  ],
  "pagination": {
    "current_page": 1,
    "page_size": 20,
    "total_items": 1,
    "total_pages": 1
  }
}
```

---

## 4. Get Status History
Retrieves the append-only audit trail of parcel status transitions.

- **Method / Path:** `GET /api/v1/parcels/{id}/history`
- **Auth Required:** Bearer JWT (`CUSTOMER`, `MERCHANT`, `COURIER`, `ADMIN`)

### Success Response (`200 OK`)
```json
{
  "parcel_id": "7f8b3c10-e29b-4b14-9f44-8d961234ef01",
  "tracking_number": "SD-20261025-A3X9K7",
  "history": [
    {
      "id": "his-11a2b3c4-9988-4433-2211-abcdef012345",
      "from_status": null,
      "to_status": "CREATED",
      "actor_id": "usr-8812c9b4-3312-4aa8-b112-998877665544",
      "actor_role": "CUSTOMER",
      "reason": "Initial customer booking confirmation",
      "occurred_at": "2026-10-25T14:32:00Z",
      "parcel_version": 1
    }
  ]
}
```

---

## 5. Get Booking Receipt
Retrieves the financial receipt snapshot for a booking, using the immutable accepted-price record.

- **Method / Path:** `GET /api/v1/parcels/{id}/receipt`
- **Auth Required:** Bearer JWT (`CUSTOMER`, `MERCHANT`, `ADMIN`)

### Success Response (`200 OK`)
```json
{
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
    "chargeable_weight_kg": 2.50,
    "origin_zone": "ZONE-WESTERN-01",
    "destination_zone": "ZONE-CENTRAL-01"
  },
  "payment_status": "PAID"
}
```
