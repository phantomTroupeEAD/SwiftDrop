# Parcel Management Module (`internal/parcels`)

> **Domain:** Parcel Aggregate Root, Lifecycle State Machine & Booking
> **Primary Owner:** Member 2 — Parcel Management & Pricing
> **Clean Architecture Boundary:** Modular Monolith Internal Module
> **Status:** Aligned Specification (Post-Audit v2.0.0 Alignment)

---

## 1. Module Overview & Responsibilities
The `parcels` module is the authoritative owner of the **Parcel Aggregate Root**. It governs the parcel lifecycle from initial booking through assignment, transit, delivery attempts, and terminal states.

### Key Responsibilities:
- Parcel aggregate root encapsulation and state transition invariant enforcement.
- Customer and merchant parcel booking workflows with idempotency protection.
- Quote verification and accepted price snapshot: Validates quote existence, checks backend expiry (`current_time >= expires_at`), ensures route and weight inputs match, and freezes an immutable accepted price snapshot (`price_cents`, currency, rule/version).
- Unique tracking number generation (`SD-YYYYMMDD-XXXXXX` using Crockford's Base32).
- Dimension and weight validation against tariff classes (proposed max 30.00 kg actual and chargeable weight).
- Address snapshot persistence using immutable JSONB fields (`pickup_address`, `dropoff_address`).
- Atomic state persistence, status audit logging (`parcel_status_history`), and outbox event creation (`parcel.booked.v1`, `parcel.cancelled.v1`).

---

## 2. Domain Invariants & Business Rules
1. **Exclusive Aggregate Root Authority:** Only the `Parcel` aggregate may mutate parcel status. No other module, handler, or raw SQL script may update `parcels.status`.
2. **Transition Legality:** State transitions strictly follow the 9-state machine: `CREATED`, `ASSIGNED`, `PICKED_UP`, `IN_TRANSIT`, `OUT_FOR_DELIVERY`, `DELIVERED`, `ATTEMPT_FAILED`, `RETURNED`, `CANCELLED`.
3. **Delivery Failure Ceiling (ADR-004):** Failed delivery attempts increment `attempt_count`. Up to 3 attempts transition to transient `ATTEMPT_FAILED`. After 3 failures, state transitions to terminal `RETURNED`.
4. **Audit History Trail & Deletion Protection:** Every status change must record an immutable row in `parcel_status_history` containing `from_status`, `to_status`, `actor_id`, `actor_role`, `reason_code`, `note`, `occurred_at`, and `parcel_version`. Deleting parcels with status history is strictly prohibited (`ON DELETE RESTRICT`) to protect regulatory audit records.
5. **Atomic Outbox Record:** Every domain state change commits an integration event (e.g., `parcel.booked.v1`, `parcel.cancelled.v1`) to `outbox_events` in the exact same PostgreSQL transaction.
6. **Idempotent Booking:** Repeated booking requests with identical `Idempotency-Key` headers return the previously booked parcel from the `idempotency_keys` cache scoped by `(key, caller_scope)`. Reusing a key with altered parameters returns HTTP 409 Conflict.
7. **Monetary Representation & Immutable Price Snapshot:** Stored strictly in integer minor units (`BIGINT`, LKR cents). Once confirmed, `price_cents`, `currency`, and `pricing_rule_version` are frozen immutably. Subsequent tariff modifications by Member 7 never retroactively alter confirmed bookings or receipts.
8. **Backend Quote Expiration:** Quote validity is strictly enforced by the backend (`current_time >= expires_at`). Expired or tampered quotes are rejected with structured HTTP 422 errors.

---

## 3. Layer Breakdown
```
internal/parcels/
├── domain/                  # Parcel aggregate, TrackingNumber VO, Address VO, Status enum, domain errors
├── application/             # BookParcel, GetParcelDetails, CancelParcel use cases & repository interfaces
├── infrastructure/          # PostgresParcelRepository, OutboxAdapter, TrackingNumberGenerator
└── transport/               # HTTP handlers (/api/v1/parcels/*) and request/response DTOs
```

---

## 4. Database Ownership & Migrations
- `000005_create_parcels_and_quotes.up.sql`:
  - `parcels`: Primary aggregate table (ID, tracking number, sender ID, JSONB addresses, dimensions, price in cents, status, courier ID, version, timestamps).
  - `parcel_status_history`: Append-only audit log of all historical transitions with `ON DELETE RESTRICT`.
  - *Architecture Alignment Item:* Dual Specification accommodates normalized child table `parcel_addresses` if team ratifies relational address persistence over JSONB snapshots.
- `000010_create_idempotency_keys.up.sql`:
  - `idempotency_keys`: Keys, `caller_scope`, request hash (SHA-256), cached response status/payload, and 24-hour expiration.

---

## 5. Events Emitted (Transactional Outbox)
- `parcel.booked.v1`: Published upon successful parcel creation.
- `parcel.cancelled.v1`: Published when a parcel is cancelled before courier pickup.

---

## 6. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Aligned with Audit v2.0.0: Crockford Base32 tracking, JSONB addresses, 9-state machine, integer LKR cents, and migrations 000005 & 000010 | Member 2 |
| 2026-10-26 | Integrated Member 2 pricing decisions: Immutable accepted price snapshot, quote validation, 30 kg operational limits, and backend quote expiry | Member 2 |
| 2026-10-26 | Reconciliation: Added cancellation endpoint `POST /api/v1/parcels/{id}/cancel`, updated `parcel_status_history` fields (`reason_code`, `note`, `RESTRICT`), `caller_scope`, and dual address specification notice | Member 2 |

---

## 6. Authoritative Architecture & Flow Diagrams

The following authoritative engineering diagrams govern the `parcels` module:

| Diagram Title | Type & Standard | Lucidchart Editable URL | Repository Artifacts |
| :--- | :--- | :--- | :--- |
| **01 Parcel Management & Pricing Use-Case Diagram** | UML Use Case | [Lucidchart Document](https://lucid.app/lucidchart/da53bbf3-1b48-450c-9945-b1816a32ae17/edit) | [PNG Preview](../../../docs/drafts/member-02-parcels-pricing/diagrams/01-parcel-pricing-use-case.png) |
| **04 Customer Parcel Booking Activity (5 Swimlanes)** | UML Activity Diagram (5 Swimlanes) | [Lucidchart Document](https://lucid.app/lucidchart/48f368fa-a237-4fea-bb84-c454dd788216/edit) | [PNG Preview](../../../docs/drafts/member-02-parcels-pricing/diagrams/04-parcel-booking-activity.png) |
| **05 Customer Parcel Booking Sequence** | UML Sequence Diagram | [Lucidchart Document](https://lucid.app/lucidchart/c2b3114e-a7b8-4313-8172-1a82336a8cc1/edit) | [PNG Preview](../../../docs/drafts/member-02-parcels-pricing/diagrams/05-parcel-booking-sequence.png) |
| **06 Parcel Lifecycle State Machine** | UML State Machine | [Lucidchart Document](https://lucid.app/lucidchart/43a950d5-3596-4bb5-84c7-64f813f48a7c/edit) | [PNG Preview](../../../docs/drafts/member-02-parcels-pricing/diagrams/06-parcel-lifecycle-state-machine.png) |
| **07 Parcel & Pricing Component Architecture** | UML Component Diagram | [Lucidchart Document](https://lucid.app/lucidchart/983208a4-c9f1-44f3-a6d8-118b89757207/edit) | [PNG Preview](../../../docs/drafts/member-02-parcels-pricing/diagrams/07-parcel-pricing-component.png) |
| **08 Parcel & Pricing Entity-Relationship Diagram** | Logical ERD (Crow's-Foot) | [Lucidchart Document](https://lucid.app/lucidchart/d51127dc-51ad-4542-843a-423d44d6fd16/edit) | [PNG Preview](../../../docs/drafts/member-02-parcels-pricing/diagrams/08-parcel-pricing-erd.png) |

