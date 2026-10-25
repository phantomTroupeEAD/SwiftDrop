# Pricing Module API Specifications (`/api/v1/pricing`)

> **Module:** `internal/pricing` | **Owner:** Member 2 — Parcel Management & Pricing | **Status:** Aligned (Post-Audit v2.0.0 Alignment)

---

## 1. Create Pricing Quote
Calculates the estimated delivery fee based on weight, dimensions, pickup/dropoff zones, and service level before booking. Persists the quote in `pricing_quotes` with a guaranteed 15-minute price freeze.

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
    "billable_extra_weight_kg": 2.0,
    "base_fee_cents": 35000,
    "per_kg_fee_cents": 10000,
    "additional_fee_cents": 20000,
    "service_multiplier": 1.0
  },
  "fee_cents": 55000,
  "expires_at": "2026-10-25T14:45:00Z",
  "created_at": "2026-10-25T14:30:00Z"
}
```

> [!NOTE]
> **Provisional Pricing Proposal Notice:**
> The mathematical breakdown (volumetric divisor 5000, billable extra weight, service tier multipliers, and 15-minute quote validity) represents a **provisional proposal pending team confirmation**. The endpoint route, integer LKR cents standard, server-side authority, and quote persistence are confirmed architectural requirements.

### Error Responses
- `400 Bad Request`: `PRICING_INVALID_INPUT` — Non-positive dimension, weight $\le 0$, or malformed JSON.
- `422 Unprocessable Entity`: `UNSERVICEABLE_ZONE_PAIR` — No active pricing rule covers the requested zone combination.
- `422 Unprocessable Entity`: `WEIGHT_LIMIT_EXCEEDED` — Weight exceeds 50.00 kg.
- `422 Unprocessable Entity`: `DIMENSION_LIMIT_EXCEEDED` — Length, width, or height exceeds 150.00 cm.
