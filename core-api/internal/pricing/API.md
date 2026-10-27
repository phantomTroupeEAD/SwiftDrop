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
  "pickup_zone_id": "ZONE-WESTERN-01",
  "dropoff_zone_id": "ZONE-CENTRAL-01",
  "weight_kg": 2.50,
  "dimensions": {
    "length_cm": 30.0,
    "width_cm": 25.0,
    "height_cm": 15.0
  },
  "service_level": "STANDARD"
}
```
*Note: Dimensions are optional if `weight_kg <= 1.0` (Option A threshold policy); dimensions are strictly required if `weight_kg > 1.0`.*

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
    "volumetric_weight_kg": 1.875,
    "chargeable_weight_kg": 2.50,
    "weight_band_id": 4,
    "weight_band_label": "2 < W <= 5 kg",
    "band_min_weight_kg": 2.00,
    "band_max_weight_kg": 5.00
  },
  "fee_cents": 55000,
  "expires_at": "2026-10-25T14:45:00Z",
  "created_at": "2026-10-25T14:30:00Z"
}
```

> [!NOTE]
> **Provisional Pricing Proposal Notice (Pending Team Ratification):**
> Delivery fee is a proposed fixed complete tariff: `delivery_fee = tariff(pickup_zone_id, destination_zone_id, weight_band_id)`. There is no separate base fee, per-kilogram charge, distance fee, or service multiplier. Volumetric divisor is 6000 cm³/kg. Chargeable weight uses Option A (actual weight only when actual weight $\le 1\text{ kg}$; $\max(\text{actual}, \text{volumetric})$ when $> 1\text{ kg}$). Proposed maximum actual and chargeable weight is 30 kg. Quote validity is 15 minutes (`expires_at = created_at + 15m`).
>
> *Conflict & Arithmetic Notice:* This proposed model diverges from `docs/contracts/pricing-rules.md`, which defines a formula with base fee (35000), per-kg surcharge (8000), service multiplier, and divisor 5000 (yielding 51000 cents for 2.5 kg Metro->Metro, or 55000 under rate 10000). The 55000 cents in this example represents an illustrative flat Band 4 complete tariff pending formal team ratification.

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
