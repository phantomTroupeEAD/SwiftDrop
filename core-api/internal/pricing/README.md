# Pricing Module (`internal/pricing`)

> **Domain:** Deterministic Fee Estimation, Tariffs & Volumetric Weight
> **Primary Owner:** Member 2 — Parcel Management & Pricing
> **Clean Architecture Boundary:** Modular Monolith Internal Module
> **Status:** Proposed Baseline (Pending Team Ratification vs `docs/contracts/pricing-rules.md`)

---

## 1. Module Overview & Responsibilities
The `pricing` module calculates delivery fees based on parcel weight, dimensions (volumetric weight), pickup and destination zones, and chargeable weight bands. It persists quotes in `pricing_quotes` with a 15-minute price freeze window.

### Key Responsibilities:
- Complete tariff lookup and rule evaluation:
  - $\text{Delivery Fee} = \text{Tariff}(\text{Pickup Zone}, \text{Destination Zone}, \text{Weight Band})$.
  - Complete tariff represents the total fee; there is no separate base fee, per-kg charge, distance fee, or service multiplier (provisional proposal diverging from legacy `pricing-rules.md`).
- Option A Volumetric Threshold Policy:
  - Actual weight $\le 1.00\text{ kg}$: dimensions do not affect price; $\text{Chargeable Weight} = \text{Actual Weight}$.
  - Actual weight $> 1.00\text{ kg}$: dimensions in cm are required; $\text{Volumetric Weight (kg)} = \frac{L \times W \times H \text{ (cm)}}{6000}$; $\text{Chargeable Weight} = \max(\text{Actual Weight}, \text{Volumetric Weight})$.
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
- Candidate REST CRUD endpoints for administrative tariff management (`/api/v1/admin/pricing-rules`) served for Member 7's Admin UI.

---

## 2. Domain Invariants & Business Rules
1. **Server Authority:** The frontend must NEVER calculate authoritative delivery fees. All fees must be calculated and validated by the backend — *Confirmed Project Standard*.
2. **Exact Integer Monetary Units:** Money values are represented strictly using exact integer minor units (`BIGINT` in SQL, `int64` in Go) with currency `LKR` (cents). Binary floating-point types (`float32`/`float64`) are strictly prohibited in financial logic — *Confirmed Project Standard*.
3. **Deterministic Complete Tariff:** Given identical zone pairs and weight band, tariff lookup returns the unique configured fee for that rule version — *Confirmed Project Standard*.
4. **Option A Threshold & Volumetric Divisor (6000):** Actual weight $\le 1\text{ kg}$ uses actual weight only. Weight $> 1\text{ kg}$ requires dimensions and uses divisor 6000 cm³/kg: $\text{Volumetric Weight (kg)} = \frac{L \times W \times H}{6000}$ — *Proposed Baseline (Pending Team Ratification)*.
5. **Quote Validity Window (15m):** Quotes freeze prices for 15 minutes (`expires_at = created_at + 15m`). Enforced strictly by the backend (`current_time >= expires_at`) — *Proposed Baseline (Pending Team Ratification)*.
6. **Administrative Boundary:** Member 7 owns admin UI and tariff-administration workflow; Member 2 owns calculation engine behavior, lookup interface, and candidate REST CRUD endpoints.

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
  - `pricing_rules`: Tariff configuration (ID, version, pickup/destination zones, weight_band_id, min/max weight bounds, fee_cents, currency, effective dates, active flag).
  - `pricing_quotes`: Pre-booking quotation cache (ID, rule ID/version, fee in cents, input snapshot JSONB, 15m expiration timestamp, optional accepted_parcel_id).

---

## 5. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Aligned with Audit v2.0.0: Standardized on POST /api/v1/pricing/quotes, integer LKR cents, 15-minute quote validity, and migration 000004 | Member 2 |
| 2026-10-26 | Integrated Member 2 pricing decisions: Complete tariff model, Option A threshold (1 kg), divisor 6000 cm³/kg, 7 weight bands up to 30 kg, backend quote expiry | Member 2 |
| 2026-10-26 | Reconciliation: Clarified provisional proposal status vs pricing-rules.md, explained 55000 vs 51000 arithmetic discrepancy, added candidate admin tariff CRUD APIs | Member 2 |

---

## 6. Authoritative Architecture & Flow Diagrams

The following authoritative engineering diagrams govern the `pricing` module:

| Diagram Title | Type & Standard | Lucidchart Editable URL | Repository Artifacts |
| :--- | :--- | :--- | :--- |
| **01 Parcel Management & Pricing Use-Case Diagram** | UML Use Case | [Lucidchart Document](https://lucid.app/lucidchart/da53bbf3-1b48-450c-9945-b1816a32ae17/edit) | [PNG Preview](../../../docs/drafts/member-02-parcels-pricing/diagrams/01-parcel-pricing-use-case.png) |
| **02 Delivery Fee Estimation Activity (4 Swimlanes)** | UML Activity Diagram (4 Swimlanes) | [Lucidchart Document](https://lucid.app/lucidchart/e9ac9f8d-5d88-4e74-9a52-5e09e5c696b4/edit) | [PNG Preview](../../../docs/drafts/member-02-parcels-pricing/diagrams/02-delivery-fee-estimation-activity.png) |
| **03 Delivery Fee Estimation Sequence** | UML Sequence Diagram | [Lucidchart Document](https://lucid.app/lucidchart/456c449a-5f68-4d12-a051-027a04e87663/edit) | [PNG Preview](../../../docs/drafts/member-02-parcels-pricing/diagrams/03-delivery-fee-estimation-sequence.png) |
| **07 Parcel & Pricing Component Architecture** | UML Component Diagram | [Lucidchart Document](https://lucid.app/lucidchart/983208a4-c9f1-44f3-a6d8-118b89757207/edit) | [PNG Preview](../../../docs/drafts/member-02-parcels-pricing/diagrams/07-parcel-pricing-component.png) |
| **08 Parcel & Pricing Entity-Relationship Diagram** | Logical ERD (Crow's-Foot) | [Lucidchart Document](https://lucid.app/lucidchart/d51127dc-51ad-4542-843a-423d44d6fd16/edit) | [PNG Preview](../../../docs/drafts/member-02-parcels-pricing/diagrams/08-parcel-pricing-erd.png) |

