# Event & Integration Support Module (`internal/events`)

> **Domain:** Transactional Outbox Writer, Event Envelopes & Domain Event Support  
> **Primary Owner:** Member 8 — Event-Driven Backend & Integrations  
> **Clean Architecture Boundary:** Modular Monolith Internal Module | **Port:** In-Monolith Go Library

---

## 1. Module Overview & Responsibilities
The `events` module provides the foundational integration plumbing used by all Core API domain modules (Parcels, Pricing, Dispatch, Delivery, Merchants) to emit domain events safely via the **Transactional Outbox Pattern**.

### Key Responsibilities:
- **Canonical Event Envelope Definitions:** Strongly typed `EventEnvelope[T]` Go struct and universal JSON serialization contracts.
- **Transactional Outbox Writer:** `OutboxWriter` interface used by application use cases to append event records to `outbox_events` within the active PostgreSQL transaction (`*sql.Tx`).
- **Event Catalog Maintenance:** Authoritative registry of standard domain event strings (e.g., `parcel.booked.v1`, `parcel.assigned.v1`, `parcel.picked_up.v1`, `parcel.delivered.v1`, `parcel.delivery_failed.v1`).
- **Distributed Correlation Tracing:** Propagates `correlation_id` across database records and event headers.

---

## 2. Domain Invariants & Guarantees
1. **Atomic Same-Transaction Commit:** Outbox records MUST be inserted using the exact same PostgreSQL `*sql.Tx` as the domain state mutation. If the domain transaction rolls back, the event is rolled back.
2. **Zero Broker Dependency in Core API:** The Core API does NOT import RabbitMQ libraries (`amqp091-go`) or publish network messages directly. It writes only to PostgreSQL. The autonomous `services/messaging` Outbox Relay handles network publishing.
3. **Immutability of Committed Events:** Once written to `outbox_events`, event payloads and correlation metadata cannot be altered.

---

## 3. Clean Layer Organization
```
internal/events/
├── envelope.go              # EventEnvelope[T] generic struct & JSON serialization
├── writer.go                # OutboxWriter interface definition
├── repository.go            # PostgresOutboxWriter implementation (appends to outbox_events)
├── catalog.go               # Standard event type string constants (e.g. parcel.delivered.v1)
└── errors.go                # ErrMissingTransaction and serialization errors
```

---

## 4. Database Ownership & Migrations
This module owns the following PostgreSQL table:
- `outbox_events` (Migration `000008_create_outbox_events.up.sql`): `id UUID PRIMARY KEY`, `aggregate_type VARCHAR(64)`, `aggregate_id UUID`, `aggregate_version INTEGER`, `event_type VARCHAR(128)`, `schema_version VARCHAR(16)`, `payload JSONB`, `correlation_id VARCHAR(128)`, `status VARCHAR(32)`, `attempt_count INTEGER`, `created_at TIMESTAMPTZ`, `published_at TIMESTAMPTZ`.

---

## 5. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Upgraded to v2.0.0 post-submission alignment: standardized envelope schemas, OutboxWriter interfaces, and zero-broker monolith rule | Post-Submission Alignment |
