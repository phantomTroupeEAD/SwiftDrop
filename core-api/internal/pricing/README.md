# Pricing Module (`internal/pricing`)

> **Domain:** Deterministic Fee Estimation, Tariffs & Volumetric Weight
> **Primary Owner:** Member 2 — Parcel Management & Pricing
> **Clean Architecture Boundary:** Modular Monolith Internal Module
> **Status:** Proposed Baseline (Pending Team Ratification vs `docs/contracts/pricing-rules.md`)

---

## 1. Module Overview & Responsibilities
The `pricing` module calculates delivery fees based on parcel weight, dimensions (volumetric weight), origin and destination zones, and requested service levels. It persists quotes in `pricing_quotes` with a 15-minute price freeze window.

### Key Responsibilities:
- Geographic tariff lookup and rule evaluation:
  - Tariffs represent geographic zone pricing (`OriginZoneID`, `DestinationZoneID`) independent of service level.
  - Base fee covers the first 1 kg.
  - Additional weight is charged proportionally: $\text{ExtraKg} = \max(0, \text{ChargeableWeightKg} - 1)$.
  - $\text{WeightFee} = \text{ExtraKg} \times \text{PerKgRate}$.
  - $\text{Subtotal} = \text{BaseFee} + \text{WeightFee}$.
- Service Level Multipliers:
  - Multipliers are applied separately by the pricing calculator.
  - `STANDARD`: multiplier ratio 1/1 (1.0x).
  - `EXPRESS`: multiplier ratio 3/2 (1.5x).
  - `SAME_DAY`: multiplier ratio 2/1 (2.0x).
  - $\text{FinalFee} = \text{Subtotal} \times \text{ServiceMultiplier}$ (rounded half-up to nearest LKR minor unit / cent).
- Volumetric Divisor Policy:
  - $\text{Volumetric Weight (kg)} = \frac{L \times W \times H \text{ (cm)}}{5000}$.
  - $\text{Chargeable Weight} = \max(\text{Actual Weight}, \text{Volumetric Weight})$.
  - Mandatory dimensions and actual weight; maximum chargeable weight is 30.00 kg.
- Quote persistence in `pricing_quotes` with 15-minute expiration timestamp (`expires_at = created_at + 15m`).
- Backend-enforced expiration check (`current_time >= expires_at` is expired).
- Safe failure: Returns controlled `ErrTariffNotFound` if no tariff exists. Never fabricates a price.
- In-process domain service interface consumed by `internal/parcels`.
- Public/authenticated fee estimation endpoint `POST /api/v1/pricing/quotes`.
- Candidate REST CRUD endpoints for administrative tariff management (`/api/v1/admin/pricing-rules`) served for Member 7's Admin UI.

---

## 2. Domain Invariants & Business Rules
1. **Server Authority:** The frontend must NEVER calculate authoritative delivery fees. All fees must be calculated and validated by the backend — *Confirmed Project Standard*.
2. **Exact Integer Monetary Units:** Money values are represented strictly using exact integer minor units (`BIGINT` in SQL, `int64` in Go) with currency `LKR` (cents). Binary floating-point types (`float32`/`float64`) are strictly prohibited in financial logic — *Confirmed Project Standard*.
3. **Approved Option A Architecture:** `Tariff` represents geographic pricing only (`OriginZoneID`, `DestinationZoneID`, `BaseFee`, `PerKgRate`, `Version`, `IsActive`, `EffectiveFrom`, `EffectiveUntil`). Service levels and multipliers are applied independently during calculation to eliminate duplicate pricing rules.
4. **Volumetric Divisor (5000 cm³/kg):** Dimensions in cm are required for all parcels. $\text{Volumetric Weight (kg)} = \frac{L \times W \times H}{5000}$. $\text{Chargeable Weight} = \max(\text{Actual Weight}, \text{Volumetric Weight})$, capped at 30 kg — *Confirmed Project Standard*.
5. **Quote Validity Window (15m):** Quotes freeze prices for 15 minutes (`expires_at = created_at + 15m`). Enforced strictly by the backend (`current_time >= expires_at`) — *Confirmed Project Standard*.
6. **Administrative Boundary:** Member 7 owns admin UI and tariff-administration workflow; Member 2 owns calculation engine behavior, lookup interface, and candidate REST CRUD endpoints.

---

## 3. Layer Breakdown
```
internal/pricing/
├── domain/                  # Money VO (LKR cents), Tariff entity, ServiceLevel VO, Quote aggregate, Calculator, domain errors
├── application/             # GenerateQuoteUseCase, ports (TariffRepository, QuoteRepository, Clock, QuoteIDGenerator)
├── infrastructure/          # PostgresTariffRepository & PostgresQuoteRepository (db.Querier backed)
└── transport/               # HTTP handler (/api/v1/pricing/quotes) and request/response DTOs
```

---

## 4. Database Ownership & Migrations
- **Canonical Migration Directory:** `core-api/migrations/`
- **Migration Authoring Ownership:** Member 9 (DevOps / Database Migrations)
- **File Naming Convention:** 14-digit UTC timestamp format `YYYYMMDDHHMMSS_name.up.sql` (e.g., `20261106000001_create_pricing_rules.up.sql`), matching existing migrations in the repository:
  - `20261021000001_create_identity_tables.up.sql`
  - `20261021000002_seed_mock_users.up.sql`
  - `20261101000050_create_outbox_events.up.sql`
  - `20261101000500_create_worker_dedup_and_webhooks.up.sql`
- **Proposed Pricing Migrations (Member 9):**
  - `pricing_rules`: Geographic base tariffs (composite PK `(id, version)`, `origin_zone_id`, `destination_zone_id`, `base_fee_cents`, `per_kg_rate_cents`, `effective_from`, `effective_until`, `version`, `is_active`, `created_at`, `updated_at`).
  - `pricing_quotes`: Pre-booking quotation cache (PK `id` UUID, `tariff_id`, `tariff_version`, `chargeable_weight_kg`, `service_level`, `fee_cents`, `currency`, `created_at`, `expires_at`, `used_at`).
  - *Insert-Only Semantics:* `QuoteRepository.Save` performs strict `INSERT` without `ON CONFLICT DO UPDATE`. Duplicate IDs are rejected with `ErrDuplicateQuote`.
  - *Atomic Single-Use Quote Consumption:* Quote consumption during parcel booking must be executed via an atomic conditional SQL update (`UPDATE pricing_quotes SET used_at = $2 WHERE id = $1 AND used_at IS NULL AND expires_at > $2`).

---

## 5. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Aligned with Audit v2.0.0: Standardized on POST /api/v1/pricing/quotes, integer LKR cents, 15-minute quote validity, and migration 000004 | Member 2 |
| 2026-10-26 | Integrated Member 2 pricing decisions: Complete tariff model, Option A threshold (1 kg), divisor 6000 cm³/kg, 7 weight bands up to 30 kg, backend quote expiry | Member 2 |
| 2026-10-26 | Reconciliation: Clarified provisional proposal status vs pricing-rules.md, explained 55000 vs 51000 arithmetic discrepancy, added candidate admin tariff CRUD APIs | Member 2 |
| 2026-11-06 | Task 1: Refactored Tariff domain model to Option A architecture. Removed ServiceLevel from Tariff; renamed Pickup/DestinationZone to Origin/DestinationZoneID; added IsActive, CreatedAt, UpdatedAt; moved multipliers to ServiceLevel; updated Calculator and GenerateQuoteUseCase to support independent service level pricing | Member 2 |
| 2026-11-06 | Task 3: Implemented and refined Pricing PostgreSQL Repositories (`PostgresTariffRepository`, `PostgresQuoteRepository`). Enforced Option A geographic tariff lookup with version and temporal window queries, strictly insert-only quote persistence (`ErrDuplicateQuote` on duplicate ID; no `ON CONFLICT DO UPDATE` overwriting `used_at`), and comprehensive in-memory test suite via standard `database/sql/driver` test mock | Member 2 |

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

