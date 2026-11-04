# Pricing Module API Specifications (`/api/v1/pricing`)

> **Module:** `internal/pricing` | **Owner:** Member 2 — Parcel Management & Pricing | **Status:** Ratified Baseline (Aligned with Base Fee + Per-Kg Surcharge)

---

## 1. Create Pricing Quote
Calculates the delivery fee tariff based on pickup/dropoff zones and chargeable weight (using the ratified base fee + per-kg surcharge formula) before booking. Persists the quote in `pricing_quotes` with a guaranteed 15-minute price freeze window.

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
*Note: Dimensions and weight must be strictly positive numeric values ($0.01 \le W \le 30.00\text{ kg}$). Volumetric weight is calculated using standard divisor 5000 cm³/kg: $(L \times W \times H) / 5000$.*

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
    "base_fee_cents": 30000,
    "per_kg_rate_cents": 10000,
    "weight_surcharge_cents": 25000
  },
  "fee_cents": 55000,
  "expires_at": "2026-10-25T14:45:00Z",
  "created_at": "2026-10-25T14:30:00Z"
}
```

> [!NOTE]
> **Pricing Alignment Status (Ratified Base Fee + Per-Kg Surcharge):**
> Volumetric divisor is standardized strictly on 5000 cm³/kg. Chargeable weight is always $\max(\text{actual}, \text{volumetric})$ across all eligible parcel weights ($0.01 \le W \le 30.00\text{ kg}$). The previous Option A 1 kg threshold exemption is discontinued. Operational maximum actual and chargeable weight is 30.00 kg. Quote validity is confirmed at 15 minutes (`expires_at = created_at + 15m`).
>
> *Ratified Pricing Formula:* Delivery fee is calculated as $\text{Delivery Fee} = \text{base\_fee\_cents} + (\text{chargeable\_weight\_kg} \times \text{per\_kg\_rate\_cents})$. Stored in integer minor units (`BIGINT` cents). Fixed weight bands (7 bands) are discontinued per team design decision. Intermediate rounding of fractional kilograms remains an open item pending team confirmation.

### Error Responses
- `400 Bad Request`: `PRICING_INVALID_INPUT` — Non-positive dimension, weight $\le 0$, non-numeric values, or malformed JSON.
- `422 Unprocessable Entity`: `PARCEL_LIMITS_EXCEEDED` — Actual weight or chargeable volumetric weight exceeds 30.00 kg.
- `422 Unprocessable Entity`: `DIMENSION_LIMIT_EXCEEDED` — Length, width, or height exceeds 150.00 cm.
- `422 Unprocessable Entity`: `PRICING_UNAVAILABLE` — No active pricing rule covers the requested zone combination. Never fabricates a fee.
- `500 Internal Server Error`: `PRICING_CONFIGURATION_ERROR` — Multiple active rules match the same zone pair; fails safely.

---

## 2. Admin Pricing Tariff Rules Management (Member 7 Boundary)

> [!NOTE]
> **Boundary Coordination (Member 2 / Member 7):**
> Administrative creation, updating, activation, and deactivation of tariff rules (`/api/v1/admin/pricing-rules`) belong strictly to **Member 7's admin workflow**. Member 2 is responsible for evaluating active rules, calculating delivery fees, and validating quotes during booking, and does not maintain or expose competing administrative CRUD endpoints.
