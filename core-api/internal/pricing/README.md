# Pricing Module (`internal/pricing`)

> **Domain:** Deterministic Fee Estimation, Tariffs & Volumetric Weight
> **Primary Owner:** Member 2 â€” Parcel Management & Pricing
> **Clean Architecture Boundary:** Modular Monolith Internal Module
> **Status:** Proposed Baseline (Pending Team Approval)

---

## 1. Module Overview & Responsibilities
The `pricing` module calculates delivery fees based on parcel weight, dimensions (volumetric weight), pickup and destination zones, and chargeable weight bands. It persists quotes in `pricing_quotes` with a 15-minute price freeze window.

### Key Responsibilities:
- Complete tariff lookup and rule evaluation:
  - $\text{delivery\_fee} = \text{tariff}(\text{pickup\_zone\_id}, \text{destination\_zone\_id}, \text{weight\_band\_id})$.
  - Complete tariff represents the total fee; there is no separate base fee, per-kg charge, distance fee, or service multiplier.
- Option A Volumetric Threshold Policy:
  - Actual weight $\le 1.00\text{ kg}$: dimensions do not affect price; $\text{chargeable\_weight} = \text{actual\_weight}$.
  - Actual weight $> 1.00\text{ kg}$: dimensions in cm are required; $\text{volumetric\_weight\_kg} = \frac{L \times W \times H \text{ (cm)}}{6000}$; $\text{chargeable\_weight} = \max(\text{actual\_weight}, \text{volumetric\_weight})$.
- Weight Band Selection (7 non-overlapping bands up to 30 kg):
  - Band 1: $0 < W \le 0.5\text{ kg}$
  - Band 2: $0.5 < W \le 1.0\text{ kg}$
  - Band 3: $1.0 < W \le 2.0\text{ kg}$
  - Band 4: $2.0 < W \le 5.0\text{ kg}$
  - Band 5: $5.0 < W \le 10.0\text{ kg}$
  - Band 6: $10.0 < W \le 20.0\text{ kg}$
  - Band 7: $20.0 < W \le 30.0\text{ kg}$
- Maximum limits: Proposed maximum actual weight and chargeable weight is 30.00 kg.
- Quote persistence in `pricing_quotes` with 15-minute expiration timestamp (`expires_at = created_at + 15m`).
- Backend-enforced expiration check (`current_time >= expires_at` is expired).
- Safe failure: Returns controlled `pricing_unavailable` if no tariff exists; fails safely on ambiguous multiple matches. Never fabricates a price.
- In-process domain service interface consumed by `internal/parcels`.
- Public/authenticated fee estimation endpoint `POST /api/v1/pricing/quotes`.

---

## 2. Domain Invariants & Business Rules
1. **Server Authority:** The frontend must NEVER calculate authoritative delivery fees. All fees must be calculated and validated by the backend â€” *Confirmed Project Standard*.
2. **Exact Integer Monetary Units:** Money values are represented strictly using exact integer minor units (`BIGINT` in SQL, `int64` in Go) with currency `LKR` (cents). Binary floating-point types (`float32`/`float64`) are strictly prohibited in financial logic â€” *Confirmed Project Standard*.
3. **Deterministic Complete Tariff:** Given identical zone pairs and weight band, tariff lookup returns the unique configured fee for that rule version â€” *Confirmed Project Standard*.
4. **Option A Threshold & Volumetric Divisor (6000):** Actual weight $\le 1\text{ kg}$ uses actual weight only. Weight $> 1\text{ kg}$ requires dimensions and uses divisor 6000 cmÂ³/kg: $\text{volumetric\_kg} = \frac{L \times W \times H}{6000}$ â€” *Proposed Baseline (Pending Team Approval)*.
5. **Quote Validity Window (15m):** Quotes freeze prices for 15 minutes (`expires_at = created_at + 15m`). Enforced strictly by the backend (`current_time >= expires_at`) â€” *Proposed Baseline (Pending Team Approval)*.
6. **Administrative Boundary:** Member 7 owns admin UI and tariff-administration workflow; Member 2 owns calculation engine behavior and lookup interface.

---

## 3. Layer Breakdown
```
internal/pricing/
â”œâ”€â”€ domain/                  # Money VO (LKR cents), Tariff entity, FeeCalculationPolicy, domain errors
â”œâ”€â”€ application/             # CalculateQuote use case & PricingCalculator interface
â”œâ”€â”€ infrastructure/          # PostgresTariffRepository & PostgresQuoteRepository
â””â”€â”€ transport/               # HTTP handler (/api/v1/pricing/quotes) and request/response DTOs
```

---

## 4. Database Ownership & Migrations
- `000004_create_pricing_rules.up.sql`:
  - `pricing_rules`: Tariff configuration (ID, version, pickup/destination zones, weight_band_id, min/max weight bounds, fee_cents, currency, effective dates, active flag).
  - `pricing_quotes`: Pre-booking quotation cache (ID, rule ID/version, fee in cents, input snapshot JSONB, 15m expiration timestamp, optional accepted_parcel_id).

---

## 5. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Aligned with Audit v2.0.0: Standardized on POST /api/v1/pricing/quotes, integer LKR cents, 15-minute quote validity, and migration 000004 | Member 2 |
| 2026-10-26 | Integrated Member 2 pricing decisions: Complete tariff model, Option A threshold (1 kg), divisor 6000 cmÂ³/kg, 7 weight bands up to 30 kg, backend quote expiry | Member 2 |
