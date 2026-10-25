# Pricing Module (`internal/pricing`)

> **Domain:** Deterministic Fee Estimation, Tariffs & Volumetric Weight  
> **Primary Owner:** Member 2 — Parcel Management & Pricing  
> **Clean Architecture Boundary:** Modular Monolith Internal Module  
> **Status:** Aligned Specification (Post-Audit v2.0.0 Alignment)

---

## 1. Module Overview & Responsibilities
The `pricing` module calculates delivery fees based on parcel weight, dimensions (volumetric weight), zones, and delivery service levels. It persists quotes in `pricing_quotes` with a 15-minute price freeze window.

### Key Responsibilities:
- Tariff lookup and rule evaluation.
- Deterministic formula calculation:
  - $\text{Volumetric Weight (kg)} = \frac{L \times W \times H \text{ (cm)}}{5000}$.
  - $\text{Chargeable Weight} = \max(\text{actual\_weight}, \text{volumetric\_weight})$.
  - $\text{Billable Extra Weight} = \lceil \max(0, \text{chargeable\_weight} - 1.0) \rceil$.
  - $\text{Total Fee (cents)} = (\text{BaseFeeCents} + \text{Billable Extra Weight} \times \text{PerKgRateCents}) \times \text{ServiceMultiplier}$.
- Quote persistence in `pricing_quotes` with 15-minute expiration timestamp (`expires_at = created_at + 15m`).
- In-process domain service interface consumed by `internal/parcels`.
- Public/authenticated fee estimation endpoint `POST /api/v1/pricing/quotes`.

---

## 2. Domain Invariants & Business Rules
1. **Server Authority:** The frontend must NEVER calculate authoritative delivery fees. All fees must be calculated and validated by the backend — *Confirmed Project Standard*.
2. **Exact Integer Monetary Units:** Money values are represented strictly using exact integer minor units (`BIGINT` in SQL, `int64` in Go) with currency `LKR` (cents). Binary floating-point types (`float32`/`float64`) are strictly prohibited in financial logic — *Confirmed Project Standard*.
3. **Deterministic Calculations:** Given identical inputs (dimensions, weight, pickup/delivery zone, speed), the fee estimation produces identical results for a given pricing rule version — *Confirmed Project Standard*.
4. **Volumetric Divisor:** Standard domestic divisor is proposed at 5000: $\text{volumetric\_kg} = \frac{L \times W \times H \text{ (cm)}}{5000}$ — *Provisional Proposal (Pending Team Confirmation)*.
5. **Quote Validity Window:** Quotes are proposed to freeze prices for 15 minutes (`expires_at = created_at + 15m`) — *Provisional Proposal (Pending Team Confirmation)*. Checkout submitted after expiration is rejected with `QUOTE_EXPIRED`.

---

## 3. Layer Breakdown
```
internal/pricing/
├── domain/                  # Money VO (LKR cents), Tariff entity, FeeCalculationPolicy, domain errors
├── application/             # CalculateQuote use case & PricingCalculator interface
├── infrastructure/          # PostgresTariffRepository & PostgresQuoteRepository
└── transport/               # HTTP handler (/api/v1/pricing/quotes) and request/response DTOs
```

---

## 4. Database Ownership & Migrations
- `000004_create_pricing_rules.up.sql`:
  - `pricing_rules`: Tariff configuration (ID, version, origin/destination zones, base fee, per-kg rate, multipliers, effective dates, active flag).
  - `pricing_quotes`: Pre-booking quotation cache (ID, rule ID/version, fee in cents, input snapshot JSONB, 15m expiration timestamp).

---

## 5. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Aligned with Audit v2.0.0: Standardized on POST /api/v1/pricing/quotes, integer LKR cents, 15-minute quote validity, and migration 000004 | Member 2 |
