# Event & Integration Support Module (`internal/events`)

> **Domain:** Transactional Outbox Writer, Event Envelopes, Ops/Admin Monitoring & Merchant Webhook History  
> **Primary Owner:** Member 8 — Event-Driven Backend & Integrations  
> **Clean Architecture Boundary:** Modular Monolith Internal Infrastructure, Application, and Transport Layers  
> **Package Import Rule:** In accordance with DDD boundaries and Architecture Rule §4.2, any Core API domain module (`parcels`, `dispatch`, `delivery`, `merchants`) may import `internal/events`. The `internal/events` package has zero external broker dependencies (no AMQP imports) and depends only on Go standard library and approved platform helpers.

---

## 1. Module Overview & Purpose

The `events` module provides the shared integration plumbing used by all Core API domain modules to emit domain events safely via the **Transactional Outbox Pattern**, as well as the Ops/Admin and Merchant inspection APIs and Swagger UI registry.

### Key Responsibilities:
- **Canonical Event Envelope (`EventEnvelope[T]` & `AnyEnvelope`):** Strongly typed Go struct adhering to the canonical schema contract with RFC 9562 UUIDv7 identifiers, ISO-8601 UTC timestamps, and schema versioning.
- **Transactional Outbox Writer (`OutboxWriter`):** Atomic writer interface allowing domain application use cases to persist event records directly inside active PostgreSQL transactions (`*sql.Tx`).
- **PostgreSQL Outbox Implementation (`PostgresOutboxWriter`):** High-performance multi-row batch insert into `outbox_events` with validation for UUIDs, event naming regex, correlation IDs, and aggregate types.
- **Authoritative Event Catalog (`catalog.go`):** Centralized constants for all system domain event types (e.g., `parcel.booked.v1`, `parcel.delivered.v1`).
- **In-Memory Testing Writer (`eventstest.InMemoryWriter`):** Fast, concurrency-safe mock writer for peer module unit and integration tests without a PostgreSQL database.
- **Ops/Admin Monitoring APIs (`application/`, `infrastructure/`, `transport/`):** Keyset-paginated inspection and retry triggers for outbox events, notification deliveries, and webhook deliveries.
- **Merchant Webhook Delivery History:** Tenant-isolated endpoints for merchants to inspect their webhook deliveries and per-attempt debug logs.
- **Unified Swagger UI Registry (`swagger/`):** Pluggable, modular documentation registry hosting interactive Swagger UI documentation at `/swagger/` without external build tools.

---

## 2. Business Invariants & Guarantees

1. **Same-Transaction Atomic Commit (Decision D8, D26):**
   - Outbox records MUST be inserted using the exact same PostgreSQL `*sql.Tx` as the domain state mutation.
   - If the business transaction fails and rolls back, the event is guaranteed not to exist.
   - Passing `nil` as the `*sql.Tx` immediately fails with `ErrMissingTransaction`.
2. **Zero Broker Dependency in Core API (Architecture §4.2):**
   - The Core API does **not** import RabbitMQ (`amqp091-go`) or make direct network calls to publish messages.
   - All network publishing is handled asynchronously by the standalone Outbox Relay daemon (`services/messaging/cmd/relay`).
3. **Strict Validation Invariants:**
   - `event_type` must strictly match regex `^[a-z_]+\.[a-z_]+\.v[0-9]+$` (e.g. `parcel.booked.v1`).
   - `aggregate_type` must be one of: `parcel`, `courier`, `merchant`, `dispatch`.
   - `aggregate_id` and `event_id` must be valid 36-character UUIDs.
   - `aggregate_version` must be $\ge 1$.
   - `correlation_id` must be non-empty and $\le 128$ characters (propagated from HTTP `X-Request-ID`).
   - `payload` must serialize to valid, non-null JSON.
4. **UUIDv7 Monotonic Deduplication:**
   - Generated event IDs are time-ordered UUIDv7s, enabling downstream consumers to detect duplicate deliveries in $O(1)$ time.
5. **Tenant Isolation & IDOR Protection (Decision D25):**
   - All merchant webhook delivery queries require an authenticated merchant tenant UUID.
   - Seeded non-UUID tenant identifiers (e.g., `mrc_01HMERCHANTTENANT00001`) are rejected with `403 TENANT_NOT_MERCHANT_UUID`.
   - Delivery attempts for foreign merchants are hidden behind `404 NOT_FOUND` to prevent identifier enumeration.
6. **Strict PII Masking in Responses (Decision D4):**
   - Email addresses in admin delivery inspection endpoints are always masked on read (e.g., `k***@domain.com`).

---

## 3. Domain Models & Layer Breakdown

```
internal/events/
├── envelope.go                  # EventEnvelope[T], AnyEnvelope, NewEnvelope, GenerateUUIDv7
├── writer.go                    # OutboxWriter interface (Write(ctx, tx, ...AnyEnvelope))
├── repository.go                # PostgresOutboxWriter (multi-row INSERT into outbox_events)
├── catalog.go                   # Authoritative event type string constants
├── errors.go                    # Strongly-typed domain & persistence validation errors
├── API.md                       # Co-located living API documentation for Ops & Merchant endpoints
├── application/
│   ├── models.go                # Query filters, pagination models, and read DTOs
│   └── service.go               # EventsService business use cases
├── infrastructure/
│   └── read_repository.go       # PostgresEventsReadRepository (keyset-paginated read queries)
├── transport/
│   ├── handlers.go              # HTTP handlers for Admin and Merchant operations
│   ├── routes.go                # Route registration and RouteDescriptor definitions
│   ├── tenant.go                # TenantResolver middleware enforcing UUID tenants (D25)
│   ├── openapi.go               # go:embed openapi.json
│   ├── openapi.json             # OpenAPI 3.0.3 specification source of truth
│   └── handlers_test.go         # OpenAPI parity, route coverage, and endpoint tests
├── swagger/
│   ├── registry.go              # Thread-safe spec registry (Register, ListSpecs, GetSpec)
│   └── handler.go               # Embedded Swagger UI HTML & JSON spec handlers
└── eventstest/
    ├── memory_writer.go         # InMemoryWriter implementation for testing
    └── memory_writer_test.go    # Unit tests verifying mock behavior & filtering
```

---

## 4. For Other Modules: How to Emit Events

### Step 1: Inject `events.OutboxWriter`
Add `events.OutboxWriter` as a dependency into your application service or use case handler:

```go
type BookParcelUseCase struct {
    parcelRepo ParcelRepository
    outbox     events.OutboxWriter
}
```

### Step 2: Write Inside the Aggregate Transaction
Construct the envelope and persist it within the exact same database transaction that saves the aggregate root:

```go
func (uc *BookParcelUseCase) Execute(ctx context.Context, cmd BookParcelCommand) error {
    tx, err := uc.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 1. Mutate domain aggregate
    parcel, err := domain.NewParcel(...)
    if err != nil {
        return err
    }
    if err := uc.parcelRepo.Save(ctx, tx, parcel); err != nil {
        return err
    }

    // 2. Build canonical event envelope
    payload := ParcelBookedPayload{
        ParcelID:       parcel.ID().String(),
        TrackingNumber: parcel.TrackingNumber(),
        MerchantID:     cmd.MerchantID,
        Status:         string(parcel.Status()),
        Notify: NotifyBlock{
            RecipientName:  cmd.RecipientName,
            RecipientEmail: cmd.RecipientEmail,
        },
    }

    env, err := events.NewEnvelope(
        events.ParcelBookedV1,
        "parcel",
        parcel.ID().String(),
        parcel.Version(),
        cmd.CorrelationID, // From platform/middleware.CorrelationID(ctx)
        "core-api.parcels",
        payload,
    )
    if err != nil {
        return fmt.Errorf("failed to create event envelope: %w", err)
    }

    // 3. Persist outbox event inside tx
    if err := uc.outbox.Write(ctx, tx, env); err != nil {
        return fmt.Errorf("failed to write outbox event: %w", err)
    }

    // 4. Commit atomically
    return tx.Commit()
}
```

### Step 3: Unit Testing Your Use Case with `InMemoryWriter`
Peer modules (Members 2, 3, 4) do not need PostgreSQL running to test event emission:

```go
func TestBookParcelUseCase(t *testing.T) {
    mockOutbox := eventstest.NewInMemoryWriter()
    useCase := NewBookParcelUseCase(mockRepo, mockOutbox)

    // Execute use case with dummy transaction or mock tx
    err := useCase.Execute(context.Background(), cmd)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // Assert event was emitted
    if mockOutbox.Count() != 1 {
        t.Fatalf("expected 1 event, got %d", mockOutbox.Count())
    }

    events := mockOutbox.FindByEventType(events.ParcelBookedV1)
    if len(events) != 1 {
        t.Fatalf("expected ParcelBookedV1 event")
    }
}
```

---

## 5. Ops/Admin & Merchant API Reference

All endpoints return uniform envelope responses:
- Success: `{"data": ..., "meta": ...}`
- Error: `{"error": {"code": "...", "message": "...", "details": [...]}}`

### Endpoints Overview

| Method | Path | Auth / Role | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/admin/events/outbox` | Admin | List outbox events with status/aggregate filter & cursor pagination |
| `GET` | `/api/v1/admin/events/outbox/stats` | Admin | Aggregate counts by status and oldest pending event age |
| `POST` | `/api/v1/admin/events/outbox/{id}/retry` | Admin | Re-queue a failed outbox event (`FAILED` $\to$ `PENDING`) |
| `GET` | `/api/v1/admin/events/deliveries` | Admin | Inspect unified notification & webhook deliveries |
| `POST` | `/api/v1/admin/events/deliveries/{id}/retry` | Admin | Re-queue a failed webhook delivery (`FAILED` $\to$ `PENDING`) |
| `GET` | `/api/v1/merchant/webhook-deliveries` | Merchant | List tenant-scoped webhook delivery history |
| `GET` | `/api/v1/merchant/webhook-deliveries/{id}/attempts` | Merchant | Inspect per-attempt HTTP status, duration, and error snippet |
| `GET` | `/swagger/` | Public | Interactive Swagger UI index listing all registered API modules |
| `GET` | `/swagger/{module}/` | Public | Interactive Swagger UI client loaded for `{module}` |
| `GET` | `/swagger/{module}/spec.json` | Public | Raw OpenAPI 3.0 specification JSON |

For detailed payload schemas, query parameters, and example responses, see the co-located [`API.md`](file:///c:/Users/kesha/OneDrive/Documents/GitHub/SwiftDrop/core-api/internal/events/API.md).

---

## 6. Swagger UI Registry Integration

The `swagger` package provides a standalone, zero-dependency Swagger UI documentation server embedded directly into the Core API:
- Any team module can register its embedded OpenAPI JSON specification during startup:
  ```go
  swagger.Register("events", transport.OpenAPISpec)
  swagger.Register("identity", identitytransport.OpenAPISpec)
  ```
- Serves Swagger UI via official CDN assets (`swagger-ui-dist@5.11.0`) with dark/light responsive layout.
- Accessible at `/swagger/events/` and `/swagger/`.

---

## 7. PostgreSQL Tables Owned by Module

1. **`outbox_events`** (Migration `20261101000050`):
   - Outbox storage for guaranteed at-least-once message publication.
   - Partial indexes on `status = 'PENDING'` and `status = 'PUBLISHED'` for efficient polling and automated retention cleanup.
2. **`processed_events`** (Migration `20261101000500`):
   - Deduplication store recording incoming event IDs per consumer group.
3. **`notification_deliveries`** (Migration `20261101000500`):
   - Delivery ledger for transactional milestone email notifications.
4. **`webhook_deliveries`** (Migration `20261101000500`):
   - Delivery ledger for merchant webhook events.
5. **`webhook_delivery_attempts`** (Migration `20261101000500`):
   - Audit trail capturing HTTP response status, duration, and sanitized error snippets per attempt.

---

## 8. Event Catalog Reference

| Constant | Event Type String | Emitting Module | Description |
| :--- | :--- | :--- | :--- |
| `ParcelBookedV1` | `parcel.booked.v1` | `internal/parcels` (M2) | New parcel booked by merchant/customer |
| `ParcelAssignedV1` | `parcel.assigned.v1` | `internal/dispatch` (M3) | Courier assigned to parcel pickup/delivery |
| `ParcelPickedUpV1` | `parcel.picked_up.v1` | `internal/parcels` (M2) | Courier picked up parcel from sender |
| `ParcelInTransitV1` | `parcel.in_transit.v1` | `internal/dispatch` (M3) | Parcel transferred between logistics hubs |
| `ParcelOutForDeliveryV1` | `parcel.out_for_delivery.v1` | `internal/dispatch` (M3) | Courier initiated last-mile delivery |
| `ParcelDeliveredV1` | `parcel.delivered.v1` | `internal/parcels` (M2) | Successful delivery confirmed with proof |
| `ParcelDeliveryFailedV1` | `parcel.delivery_failed.v1` | `internal/delivery` (M3) | Delivery attempt unsuccessful with reason |
| `ParcelReturnedV1` | `parcel.returned.v1` | `internal/parcels` (M2) | Parcel returned to sender/origin hub |
| `ParcelCancelledV1` | `parcel.cancelled.v1` | `internal/parcels` (M2) | Parcel booking cancelled prior to transit |
| `CourierUnassignedV1` | `courier.unassigned.v1` | `internal/dispatch` (M3) | Courier unassigned from dispatch run |
| `MerchantStatusChangedV1` | `merchant.status_changed.v1` | `internal/merchants` (M4) | Merchant onboarding state changed |
| `MerchantApprovedV1` | `merchant.approved.v1` | `internal/merchants` (M4) | Merchant KYC approved for live shipping |

---

## 9. Implementation Status & Changelog

| Date | Status / Change | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial architecture draft | Architecture Baseline |
| 2026-10-25 | Harmonized event contracts and modular monolith boundary | Team Agreement |
| 2026-11-01 | **Phase 1 Complete:** Canonical `EventEnvelope[T]` & `AnyEnvelope`, UUIDv7 generator, `PostgresOutboxWriter`, `InMemoryWriter`, migration `20261101000050`. | Member 8 AI Implementation |
| 2026-11-01 | **Phase 1B Complete:** Hardened platform packages (`httpx`, `middleware`, `config`, `db`) with cursor pagination and correlation tracking. | Member 8 AI Implementation |
| 2026-11-01 | **Phase 7 Complete:** Admin Ops endpoints, Merchant Webhook endpoints, `TenantResolver`, Swagger UI registry, OpenAPI 3.0 embedded spec and parity tests. | Member 8 AI Implementation |


---

## 10. Architectural Decisions & Invariants Summary (D1–D30)

| Decision | Invariant / Boundary | Implementation Enforcement |
| :--- | :--- | :--- |
| **D4** | Email PII masking on read | `infrastructure.MaskEmail` transforms emails to `k***@domain.com` before returning in responses |
| **D8 / D26** | Single-Tx atomic commit | `OutboxWriter.Write` requires non-nil `*sql.Tx`; zero standalone commits |
| **D9** | Authoritative table isolation | Core API events module reads and writes only M8-owned tables; never queries `parcels` directly |
| **D22** | Keyset & page pagination | Keyset cursor (`before`/`after` with base64 encoded cursor) implemented via `platform/httpx` |
| **D24** | Pluggable Swagger registry | Embedded Swagger UI served at `/swagger/` and `/swagger/events/` without external binaries |
| **D25** | Tenant UUID format check | `TenantResolver` returns `403 TENANT_NOT_MERCHANT_UUID` on seeded slug `mrc_01HMERCHANTTENANT00001` |
| **D28** | No transaction wrapping in migrations | Canonical SQL migrations omit raw `BEGIN;`/`COMMIT;` and `CONCURRENTLY` |
| **D30** | Zero broker imports in Core API | Enforced via `.golangci.yml` `depguard` rule `core_api_no_broker` |
