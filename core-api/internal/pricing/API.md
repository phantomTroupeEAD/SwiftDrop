# Pricing Module API Specifications (`/api/v1/pricing`)

> **Module:** `internal/pricing` | **Owner:** Member 2 — Parcel Management & Pricing | **Status:** Proposed Baseline (Pending Team Approval)

---

## 1. Create Pricing Quote
Calculates the complete delivery fee tariff based on pickup/dropoff zones and chargeable weight band before booking. Persists the quote in `pricing_quotes` with a guaranteed 15-minute price freeze window.

- **Method / Path:** `POST /api/v1/pricing/quotes`
- **Auth Required:** Optional (Public with rate limit, or Bearer JWT)
- **Headers:**
  - `Content-Type: application/json`
  - `Authorization: Bearer <jwt>` (Optional)

### Request Body
```json
{
  "origin_zone_id": "ZONE-WESTERN-01",
  "destination_zone_id": "ZONE-CENTRAL-01",
  "actual_weight_kg": 2.50,
  "dimensions": {
    "length_cm": 30.0,
    "width_cm": 25.0,
    "height_cm": 15.0
  },
  "service_level": "STANDARD"
}
```
*Note: Dimensions and actual weight are mandatory. Volumetric weight is calculated using divisor 5000 cm³/kg.*

### Success Response (`201 Created`)
```json
{
  "quote_id": "d3b07384-d113-494b-9c8e-aa672580c822",
  "pricing_rule_id": "pr-rule-zone-w-c-v2",
  "pricing_rule_version": 2,
  "service_level": "STANDARD",
  "currency": "LKR",
  "breakdown": {
    "actual_weight_kg": 2.50,
    "volumetric_weight_kg": 2.25,
    "chargeable_weight_kg": 2.50,
    "base_fee_cents": 35000,
    "per_kg_rate_cents": 8000,
    "weight_fee_cents": 12000,
    "service_multiplier": 1.0
  },
  "fee_cents": 47000,
  "expires_at": "2026-11-06T10:15:00Z",
  "created_at": "2026-11-06T10:00:00Z"
}
```

> [!NOTE]
> **Approved Option A Pricing Architecture:**
> Delivery fee is calculated using geographic tariff (base fee covering first 1 kg + proportional additional weight rate) and service-level multiplier:
> `ExtraKg = max(0, ChargeableWeightKg - 1)`
> `WeightFee = ExtraKg * PerKgRate`
> `FinalFee = (BaseFee + WeightFee) * ServiceMultiplier`
> Service multipliers: STANDARD = 1.0x (1/1), EXPRESS = 1.5x (3/2), SAME_DAY = 2.0x (2/1).
> Volumetric divisor is 5000 cm³/kg. Chargeable weight is $\max(\text{actual}, \text{volumetric})$, capped at 30 kg. Quote validity is 15 minutes (`expires_at = created_at + 15m`).

### Error Responses
- `400 Bad Request`: `PRICING_INVALID_INPUT` — Non-positive dimension, weight $\le 0$, or malformed JSON.
- `422 Unprocessable Entity`: `DIMENSIONS_REQUIRED_ABOVE_1KG` — Parcel weight exceeds 1.0 kg but dimensions are missing.
- `422 Unprocessable Entity`: `PARCEL_LIMITS_EXCEEDED` — Actual weight or applicable chargeable weight exceeds 30.00 kg.
- `422 Unprocessable Entity`: `DIMENSION_LIMIT_EXCEEDED` — Length, width, or height exceeds 150.00 cm.
- `422 Unprocessable Entity`: `PRICING_UNAVAILABLE` — No active pricing rule covers the requested zone combination and weight band. Never fabricates a fee.
- `500 Internal Server Error`: `PRICING_CONFIGURATION_ERROR` — Multiple active rules match the same zone pair and weight band; fails safely.

---

## 2. Admin Pricing Tariff Rules Management (Candidate Endpoints)

> [!NOTE]
> **Boundary Coordination (Member 2 / Member 7):**
> Member 2's pricing context serves the candidate backend tariff CRUD REST endpoints to allow administrative tariff maintenance. Member 7 owns the Admin UI portal consuming these endpoints. Full ratification of endpoint ownership is pending team approval.

- `POST /api/v1/admin/pricing-rules`: Create a new tariff rule for a zone-pair and weight band.
- `GET /api/v1/admin/pricing-rules`: List all active/inactive tariff rules with filtering by zone and band.
- `GET /api/v1/admin/pricing-rules/{id}`: Retrieve detailed pricing rule definition.
- `PUT /api/v1/admin/pricing-rules/{id}`: Update tariff fee or validity window (creates new rule version).
- `DELETE /api/v1/admin/pricing-rules/{id}`: Deactivate tariff rule.
