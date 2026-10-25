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
- Unique tracking number generation (`SD-YYYYMMDD-XXXXXX` using Crockford's Base32).
- Dimension and weight validation against tariff classes.
- Address snapshot persistence using immutable JSONB fields (`pickup_address`, `dropoff_address`).
- Atomic state persistence, status audit logging (`parcel_status_history`), and outbox event creation (`parcel.booked.v1`, `parcel.cancelled.v1`).

---

## 2. Domain Invariants & Business Rules
1. **Exclusive Aggregate Root Authority:** Only the `Parcel` aggregate may mutate parcel status. No other module, handler, or raw SQL script may update `parcels.status`.
2. **Transition Legality:** State transitions strictly follow the 9-state machine: `CREATED`, `ASSIGNED`, `PICKED_UP`, `IN_TRANSIT`, `OUT_FOR_DELIVERY`, `DELIVERED`, `ATTEMPT_FAILED`, `RETURNED`, `CANCELLED`.
3. **Delivery Failure Ceiling (ADR-004):** Failed delivery attempts increment `attempt_count`. Up to 3 attempts transition to transient `ATTEMPT_FAILED`. After 3 failures, state transitions to terminal `RETURNED`.
4. **Audit History Trail:** Every status change must record an immutable row in `parcel_status_history` containing `from_status`, `to_status`, `actor_id`, `actor_role`, `occurred_at`, and `parcel_version`.
5. **Atomic Outbox Record:** Every domain state change commits an integration event (e.g., `parcel.booked.v1`, `parcel.cancelled.v1`) to `outbox_events` in the exact same PostgreSQL transaction.
6. **Idempotent Booking:** Repeated booking requests with identical `Idempotency-Key` headers return the previously booked parcel from the `idempotency_keys` cache. Reusing a key with altered parameters returns HTTP 409 Conflict.
7. **Monetary Representation:** Stored strictly in integer minor units (`BIGINT`, LKR cents). Binary floating-point types are strictly forbidden.

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
  - `parcel_status_history`: Append-only audit log of all historical transitions.
- `000010_create_idempotency_keys.up.sql`:
  - `idempotency_keys`: Keys, caller scope, request hash (SHA-256), cached response status/payload, and 24-hour expiration.

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
