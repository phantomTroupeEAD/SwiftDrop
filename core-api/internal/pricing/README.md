# Pricing Module (`internal/pricing`)

> **Domain:** Deterministic Fee Estimation, Tariffs & Volumetric Weight
> **Primary Owner:** Member 2 — Parcel Management & Pricing
> **Clean Architecture Boundary:** Modular Monolith Internal Module
> **Status:** Ratified Baseline (Aligned with Base Fee + Per-Kg Surcharge Model)

---

## 1. Module Overview & Responsibilities
The `pricing` module calculates delivery fees based on parcel weight, dimensions (volumetric weight), pickup and destination zones, and chargeable weight. It persists quotes in `pricing_quotes` with a 15-minute price freeze window.

### Key Responsibilities:
- Tariff lookup and rule evaluation:
  - $\text{Delivery Fee} = \text{Base Fee} + (\text{Chargeable Weight} \times \text{Per-Kilogram Rate})$.
  - Identifies applicable tariff based on pickup zone and destination zone; retrieves rule (`base_fee_cents`, `per_kg_rate_cents`), calculates delivery fee, and generates pricing quotes.
  - Preserves tariff ID and tariff version used for the quote.
- Volumetric Weight Calculation (Divisor 5,000 cm³/kg):
  - $\text{Volumetric Weight (kg)} = \frac{\text{Length (cm)} \times \text{Width (cm)} \times \text{Height (cm)}}{5000}$
  - Chargeable weight is always $\max(\text{Actual Weight}, \text{Volumetric Weight})$ across all eligible parcel weights ($0.01 \le W \le 30.00\text{ kg}$).
  - Consistent calculation across all eligible weights; Option A 1 kg exemption threshold is discontinued.
- Ratified Pricing Formula:
  - Base Fee + Per-Kilogram Surcharge: For each zone pair, the tariff specifies `base_fee_cents` and `per_kg_rate_cents`.
  - Delivery Fee = $\text{base\_fee\_cents} + (\text{chargeable\_weight\_kg} \times \text{per\_kg\_rate\_cents})$.
  - Fixed weight bands (7 bands) discontinued per team design decision.
- Maximum limits: Operational maximum actual weight and chargeable weight is 30.00 kg. Single dimension limit is 150.00 cm.
- Quote persistence in `pricing_quotes` with confirmed 15-minute expiration timestamp (`expires_at = created_at + 15m`).
- Backend-enforced expiration check (`current_time >= expires_at` is expired).
- Safe failure: Returns controlled `pricing_unavailable` if no tariff exists; fails safely on ambiguous multiple matches. Never fabricates a price.
- In-process domain service interface consumed by `internal/parcels`.
- Public/authenticated fee estimation endpoint `POST /api/v1/pricing/quotes`.
- Administrative boundary: Tariff creation, updating, activation, and administrative configuration belong to Member 7's admin workflow. Member 2 does not expose competing admin endpoints.

---

## 2. Domain Invariants & Business Rules
1. **Server Authority:** The frontend must NEVER calculate authoritative delivery fees. All fees must be calculated and validated by the backend — *Confirmed Project Standard*.
2. **Exact Integer Monetary Units:** Money values are represented strictly using exact integer minor units (`BIGINT` in SQL, `int64` in Go) with currency `LKR` (cents). Binary floating-point types (`float32`/`float64`) are strictly prohibited in financial logic — *Confirmed Project Standard*.
3. **Deterministic Tariff Lookup:** Given identical zone pairs and chargeable weight, tariff lookup returns the unique configured fee for that rule version — *Confirmed Project Standard*.
4. **Standard Volumetric Divisor (5000 cm³/kg):** Chargeable weight is always $\max(\text{actual\_weight}, \text{volumetric\_weight})$ using standard IATA divisor 5,000 cm³/kg: $\text{Volumetric Weight (kg)} = \frac{L \times W \times H}{5000}$. Applied consistently for all weights; Option A 1 kg threshold is discontinued.
5. **Quote Validity Window (15m):** Quotes freeze prices for 15 minutes (`expires_at = created_at + 15m`). Enforced strictly by the backend (`current_time >= expires_at`) — *Confirmed Architecture Standard*.
6. **Administrative Boundary:** Member 7 owns admin UI and tariff-administration workflow (`/api/v1/admin/pricing-rules`); Member 2 owns calculation engine behavior, quote generation, and tariff-retrieval interface.

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
  - `pricing_rules`: Tariff configuration (ID, version, pickup/destination zones, base_fee_cents, per_kg_rate_cents, currency, effective dates, active flag).
  - `pricing_quotes`: Pre-booking quotation cache (ID, rule ID/version, fee in cents, input snapshot JSONB, 15m expiration timestamp, optional accepted_parcel_id).

---

## 5. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Aligned with Audit v2.0.0: Standardized on POST /api/v1/pricing/quotes, integer LKR cents, 15-minute quote validity, and migration 000004 | Member 2 |
| 2026-10-26 | Integrated Member 2 pricing decisions: Complete tariff model, Option A threshold (1 kg), divisor 6000 cm³/kg, 7 weight bands up to 30 kg, backend quote expiry | Member 2 |
| 2026-10-26 | Reconciliation: Clarified provisional proposal status vs pricing-rules.md, explained 55000 vs 51000 arithmetic discrepancy, added candidate admin tariff CRUD APIs | Member 2 |
| 2026-10-28 | Aligned with Final Decisions: Standardized on divisor 5000 cm³/kg, max(actual, volumetric) for all eligible parcels, discontinued Option A 1kg threshold, affirmed Member 7 admin workflow ownership | Member 2 |
| 2026-11-04 | Ratified Pricing Model: Transitioned from 7 fixed weight bands to Option 2: Base Fee + Per-Kilogram Surcharge (`base_fee_cents`, `per_kg_rate_cents`); aligned with `pricing-rules.md` | Member 2 |

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

