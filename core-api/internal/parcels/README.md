# Parcel Management Module (`internal/parcels`)

The receipt endpoint uses the same access checks as parcel detail and reads the stored accepted booking price. Payment settlement is outside the parcel module.

> **Domain:** Parcel Aggregate Root, Lifecycle State Machine & Booking  
> **Primary Owner:** Member 2 — Parcel Management & Pricing  
> **Clean Architecture Boundary:** Modular Monolith Internal Module  
> **Status:** Booking, parcel detail, history, and receipts implemented. List and cancellation deferred.

---

## 1. Module Overview & Responsibilities
The `parcels` module is the authoritative owner of the **Parcel Aggregate Root**. It governs the parcel lifecycle from initial booking through assignment, transit, delivery attempts, and terminal states.

### Key Responsibilities:
- **Parcel Aggregate Root Encapsulation:** Authoritative guardian of all parcel business invariants, dimension/weight validation, and state machine legality.
- **Parcel Booking & Accepted Price Snapshot:** Validates quote existence, verifies backend expiration (`current_time >= expires_at`), ensures route and weight match quote inputs, and atomically consumes quote and freezes immutable pricing snapshots (`price_cents`, currency, rule version).
- **Authoritative Lifecycle State Machine:** Enforces the canonical 9-state lifecycle machine. External modules (such as Member 3 Dispatch & Delivery) cannot directly update parcel status in SQL and must request transitions via the Parcel Lifecycle Application Service.
- **Unique Tracking Number Allocation:** Generates standard tracking numbers (`SD-YYYYMMDD-XXXXXX`) using cryptographic entropy and Crockford's Base32 encoding with bounded collision retry.
- **Status Audit History:** Appends an immutable, chronological audit trail in `parcel_status_history` for every status change, tracking actor identity, role, and operational reason.
- **Transactional Outbox Event Emission:** Emits versioned domain events (`parcel.booked.v1`, `parcel.assigned.v1`, `parcel.picked_up.v1`, etc.) within the exact same database transaction as the aggregate state update.
- **Optimistic Concurrency Control:** Guards all status mutations against lost updates using version-checked SQL updates (`WHERE id = $ AND version = $`).
- **Caller-Scoped Request Idempotency:** Deduplicates repeated `BookParcelUseCase` executions via `idempotency_keys` (`PRIMARY KEY (key, caller_scope)`, 24-hour TTL, `created_resource_id UUID NULL`). Acquires a transaction-scoped PostgreSQL advisory lock (`pg_advisory_xact_lock`) and resolves replays/conflicts *before* quote consumption, while writing new idempotency snapshots atomically in the booking transaction.

---

## 2. Domain Invariants & Business Rules
1. **Exclusive Aggregate Root Authority:** Only the `Parcel` aggregate may mutate parcel status. No other module, handler, or raw SQL script may update `parcels.status`.
2. **Canonical 9-State Lifecycle Machine:**
   - `CREATED` → `ASSIGNED` or `CANCELLED`
   - `ASSIGNED` → `PICKED_UP`
   - `PICKED_UP` → `IN_TRANSIT`
   - `IN_TRANSIT` → `OUT_FOR_DELIVERY`
   - `OUT_FOR_DELIVERY` → `DELIVERED` or `ATTEMPT_FAILED`
   - `ATTEMPT_FAILED` → `OUT_FOR_DELIVERY` (if attempts $< 3$) or `RETURNED`
   - `DELIVERED`, `RETURNED`, and `CANCELLED` are terminal states; no further transitions are permitted.
3. **Delivery Failure Ceiling (ADR-004):**
   - Recording an unsuccessful delivery attempt increments `delivery_attempts`.
   - Up to 3 failed attempts permit rescheduling to `OUT_FOR_DELIVERY`.
   - Upon the 3rd failed attempt (`delivery_attempts == 3`), further dispatch for delivery is strictly locked (`ErrMaxDeliveryAttemptsExceeded`). The only permissible transition is to the terminal `RETURNED` status.
4. **Cancellation Restriction:** Cancellation (`CANCELLED`) is permitted **only while the parcel is in `CREATED` status**. Once assigned or in transit, cancellation requests are rejected (`ErrCancellationNotAllowed`).
5. **Audit History Trail & Deletion Protection:** Every status change records an immutable row in `parcel_status_history` containing `from_status`, `to_status`, `actor_id`, `actor_role`, `reason`, and `timestamp`. Deleting parcels with status history is strictly prohibited (`ON DELETE RESTRICT`).
6. **Atomic Outbox Record:** State changes, status history entries, and outbox integration events commit in the exact same PostgreSQL transaction.
7. **Monetary Representation & Immutable Price Snapshot:** Delivery fees are stored in integer minor units (`BIGINT`, LKR cents). Confirmed booking pricing snapshots are never retroactively altered.

---

## 3. Layer Breakdown
```
internal/parcels/
├── domain/                  # Parcel aggregate root, TrackingNumber, Address, Weight & Dimensions, Status enum, domain errors
├── application/             # BookParcelUseCase (with WithIdempotency), TransitionParcelStatusUseCase, GetParcelUseCase, GetParcelHistoryUseCase, ResolveParcelCallerForBooking, ParcelLifecycleService, IdempotencyStore, MerchantReader, Repository Ports, Outbox payloads
├── infrastructure/          # PostgresParcelRepository, PostgresStatusHistoryRepository (Save + FindByParcelID), PostgresQuoteConsumer, PostgresIdempotencyRepository, MerchantRepoAdapter
└── transport/               # HTTP handlers & DTOs (POST /api/v1/parcels, GET /api/v1/parcels/{id}, GET /api/v1/parcels/{id}/history), UUID Idempotency-Key & SHA-256 request hashing, strict JSON decode, error mapping, RegisterRoutes (ready for cmd/api wiring)
```

---

## 4. Lifecycle Application Architecture & Public Interface

### 4.1 Interface Contract
External modules—specifically Member 3's Dispatch & Delivery bounded context—interact with parcel status exclusively through the `ParcelLifecycleService` port:

```go
type ParcelLifecycleService interface {
    // Execute runs a status transition in an isolated, atomic database transaction.
    Execute(ctx context.Context, cmd TransitionParcelStatusCommand) (TransitionParcelStatusResult, error)

    // ExecuteTx runs a status transition within an active database transaction provided by the caller.
    ExecuteTx(ctx context.Context, tx *sql.Tx, cmd TransitionParcelStatusCommand) (TransitionParcelStatusResult, error)
}
```

### 4.2 Command & Result Definitions
```go
type TransitionParcelStatusCommand struct {
    ParcelID        string
    TargetStatus    domain.ParcelStatus
    ActorID         *string
    ActorType       domain.ActorType
    ActorRole       string
    Reason          string
    ExpectedVersion *int
    CorrelationID   string
}

type TransitionParcelStatusResult struct {
    Parcel        domain.Parcel
    StatusHistory domain.StatusHistoryEntry
}
```

### 4.3 Transaction & Concurrency Strategy
The lifecycle usecase guarantees strict consistency and prevents lost updates via optimistic concurrency control:
1. **Load Current State:** Reads the parcel from PostgreSQL via `FindByID` using the active transaction.
2. **Version Precondition Verification:** If the caller supplied `ExpectedVersion`, the usecase verifies that `parcel.Version == *cmd.ExpectedVersion`. Mismatches immediately return `ErrConcurrencyConflict`.
3. **Domain Evaluation & History ID Allocation:** Invokes `parcel.TransitionTo(...)`, which evaluates transition legality, increments `AttemptCount` on failure, advances `parcel.Version`, and updates `parcel.UpdatedAt`; then generates and validates one UUIDv7 (`IDGenerator.GenerateID()`) assigned to `historyEntry.ID`.
4. **Optimistic Database Mutation:**
   ```sql
   UPDATE parcels
   SET status = $1,
       attempt_count = $2,
       version = version + 1,
       updated_at = $3
   WHERE id = $4
     AND version = $5;
   ```
   If zero rows are affected (indicating a concurrent modification occurred between read and write), the repository returns `ErrConcurrencyConflict`.
5. **Status History Append:** Persists an immutable row to `parcel_status_history` with explicit `id = historyEntry.ID`.
6. **Outbox Event Append:** Serializes `ParcelStatusChangedPayload` (including `"status_history_id": historyEntry.ID`) inside a new `events.EventEnvelope` (retaining its own independent envelope `event_id` and `schema_version: "1.0.0"`) and writes it to `outbox_events` within the exact same `*sql.Tx`.
7. **Commit / Rollback:** If any step fails, the entire transaction rolls back cleanly.

---

## 5. Member 3 Dispatch & Delivery Integration Guide

Member 3's dispatch and delivery workflows must **never** run raw SQL updates against `parcels` or bypass domain validation. Below is an authoritative integration example showing how Member 3 coordinates driver assignment with the Parcel lifecycle:

```go
package dispatch

import (
    "context"
    "database/sql"
    "fmt"
    
    "github.com/swiftdrop/core-api/internal/parcels/application"
    "github.com/swiftdrop/core-api/internal/parcels/domain"
    "github.com/swiftdrop/core-api/internal/platform/db"
)

type AssignCourierUseCase struct {
    database        *sql.DB
    parcelLifecycle application.ParcelLifecycleService
    dispatchRepo    CourierAssignmentRepository
}

func (uc *AssignCourierUseCase) Execute(ctx context.Context, parcelID, courierID, dispatcherID string) error {
    return db.WithTx(ctx, uc.database, func(tx *sql.Tx) error {
        // 1. Perform Member 3 specific domain logic: Courier shift, capacity verification, assignment record
        if err := uc.dispatchRepo.CreateAssignmentTx(ctx, tx, courierID, parcelID); err != nil {
            return fmt.Errorf("failed to record courier assignment: %w", err)
        }

        // 2. Request parcel status transition through Member 2's application port within the SAME transaction
        cmd := application.TransitionParcelStatusCommand{
            ParcelID:     parcelID,
            TargetStatus: domain.StatusAssigned,
            ActorID:      dispatcherID,
            ActorRole:    "DISPATCHER",
            Reason:       fmt.Sprintf("Courier %s assigned by dispatcher %s", courierID, dispatcherID),
        }

        result, err := uc.parcelLifecycle.ExecuteTx(ctx, tx, cmd)
        if err != nil {
            return fmt.Errorf("parcel lifecycle transition rejected: %w", err)
        }

        _ = result // Access updated parcel aggregate and history record
        return nil
    })
}
```

---

## 6. Error Handling Reference

| Error Constant | Layer | Description | Standard HTTP Mapping |
| :--- | :--- | :--- | :--- |
| `ErrParcelNotFound` | Application | Parcel ID does not exist in persistence | `404 Not Found` |
| `ErrQuoteNotFound` | Application | Referenced quote ID does not exist | `404 Not Found` |
| `ErrQuoteOwnershipMismatch` | Application | Quote belongs to a different customer or merchant tenant | `403 Forbidden` |
| `ErrQuoteAlreadyUsed` | Application | Referenced quote has already been consumed by a booking | `409 Conflict` |
| `ErrQuoteExpired` | Application | Referenced quote reached/passed 15-minute expiry | `422 Unprocessable Entity` |
| `ErrQuoteWeightMismatch` / `ErrQuoteZoneMismatch` | Application | Weight, dimensions, or address zones do not match the quote | `422 Unprocessable Entity` |
| `ErrForbiddenParcelAccess` | Application | Caller fails role, merchant approval/ownership, or parcel IDOR check (wraps specific cause below) | `403 Forbidden` |
| `ErrInvalidParcelID` | Domain | Path parcel ID is missing or not a valid UUID | `400 Bad Request` |
| `ErrInvalidCallerID` / `ErrMissingTenantID` / `ErrInvalidTenantID` | Application | Non-UUID user ID or missing/non-UUID merchant `tenant_id` (Decision D25) | `403 Forbidden` |
| `ErrMerchantNotFound` / `ErrMerchantOwnerMismatch` / `ErrMerchantNotApproved` | Application | Merchant tenant missing, not owned by `Principal.UserID`, or status != `APPROVED` | `403 Forbidden` |
| `ErrConcurrencyConflict` | Application | Version mismatch between read and update | `409 Conflict` |
| `ErrInvalidTransitionCommand` | Application | Missing required command attributes | `400 Bad Request` |
| `ErrInvalidParcelStatus` | Domain | Target status is not one of the 9 valid statuses | `422 Unprocessable Entity` |
| `ErrIllegalStateTransition` | Domain | Target status is illegal from current state | `409 Conflict` |
| `ErrCancellationNotAllowed` | Domain | Attempted to cancel parcel when status != CREATED | `409 Conflict` |
| `ErrMaxDeliveryAttemptsExceeded` | Domain | Attempted 4th delivery run after 3 failures | `409 Conflict` |
| `ErrTerminalState` | Domain | Attempted to mutate DELIVERED, RETURNED, or CANCELLED parcel | `409 Conflict` |
| `ErrItemDescriptionTooLong` | Domain | Item description exceeds maximum allowed length of 500 characters | `422 Unprocessable Entity` |
| `ErrIdempotencyPayloadMismatch` | Application | Reused idempotency key within 24h with differing request hash/operation | `409 Conflict` |
| `ErrIdempotencyConflict` | Application | Concurrent/active unexpired idempotency key conflict | `409 Conflict` |
| `ErrInvalidIdempotencyKey` / `ErrInvalidCallerScope` / `ErrInvalidIdempotencyOperation` / `ErrInvalidRequestHash` | Application | Missing or malformed idempotency metadata (`Idempotency-Key`, `caller_scope`, `operation`, SHA-256 hex `request_hash`) | `400 Bad Request` |

---

## 7. Database Ownership & Schema
- **Canonical Migration Directory:** `core-api/migrations/`
- **Migration Authoring Ownership:** Member 2 authors pricing/parcels/idempotency DDL; Member 9 (DevOps / Database Coordinator) reviews migrations and coordinates DB role grants
- **Migration Files:**
  - `20261110000001_create_pricing_tables.{up,down}.sql`: `pricing_rules`, `pricing_quotes`
  - `20261110000002_create_parcel_tables.{up,down}.sql`: `parcels`, `parcel_status_history`
  - `20261110000003_create_idempotency_keys.{up,down}.sql`: `idempotency_keys` (`key`, `caller_scope`, `operation`, `request_hash`, `response_status`, `response_body`, `created_resource_id`, `created_at`, `expires_at`; composite `PRIMARY KEY (key, caller_scope)`, index `idx_idempotency_keys_expires_at (expires_at ASC)`)
  - `20261113000001_add_parcel_item_details.{up,down}.sql`: adds `item_description TEXT NULL` and `special_instructions TEXT NULL` to `parcels`
- **Primary Aggregate Table:** `parcels`
- **Audit Table:** `parcel_status_history`
- **Idempotency Table:** `idempotency_keys` (24-hour TTL; expired keys (`expires_at <= now`) are treated as fresh requests and atomically replaced via `ON CONFLICT (key, caller_scope) DO UPDATE ... WHERE idempotency_keys.expires_at <= ...`)

---

## 8. Events Emitted (Transactional Outbox)
Every state change commits a versioned event (`schema_version: "1.0.0"`) to `outbox_events` in the same `*sql.Tx` as the parcel update and `parcel_status_history` insert. For all 8 lifecycle transitions emitted via `TransitionParcelStatusUseCase`, `ParcelStatusChangedPayload.status_history_id` carries the exact UUID persisted to `parcel_status_history.id` while the enclosing envelope keeps its independent `event_id`:

| Event Type | Triggering Transition | Aggregates & Routing Key | Payload Structure |
| :--- | :--- | :--- | :--- |
| `parcel.booked.v1` | Initial parcel booking | `aggregate_type: "parcel"` | `application.ParcelBookedPayload` (includes `item_description`, `special_instructions`) |
| `parcel.assigned.v1` | CREATED → ASSIGNED | `aggregate_type: "parcel"` | `application.ParcelStatusChangedPayload` (includes `status_history_id`) |
| `parcel.picked_up.v1` | ASSIGNED → PICKED_UP | `aggregate_type: "parcel"` | `application.ParcelStatusChangedPayload` (includes `status_history_id`) |
| `parcel.in_transit.v1` | PICKED_UP → IN_TRANSIT | `aggregate_type: "parcel"` | `application.ParcelStatusChangedPayload` (includes `status_history_id`) |
| `parcel.out_for_delivery.v1` | IN_TRANSIT / ATTEMPT_FAILED → OUT_FOR_DELIVERY | `aggregate_type: "parcel"` | `application.ParcelStatusChangedPayload` (includes `status_history_id`) |
| `parcel.delivered.v1` | OUT_FOR_DELIVERY → DELIVERED | `aggregate_type: "parcel"` | `application.ParcelStatusChangedPayload` (includes `status_history_id`) |
| `parcel.delivery_failed.v1` | OUT_FOR_DELIVERY → ATTEMPT_FAILED | `aggregate_type: "parcel"` | `application.ParcelStatusChangedPayload` (includes `status_history_id`) |
| `parcel.returned.v1` | ATTEMPT_FAILED → RETURNED | `aggregate_type: "parcel"` | `application.ParcelStatusChangedPayload` (includes `status_history_id`) |
| `parcel.cancelled.v1` | CREATED → CANCELLED | `aggregate_type: "parcel"` | `application.ParcelStatusChangedPayload` (includes `status_history_id`) |

---

## 9. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Aligned with Audit v2.0.0: Crockford Base32 tracking, JSONB addresses, 9-state machine, integer LKR cents, and migrations 000005 & 000010 | Member 2 |
| 2026-10-26 | Integrated Member 2 pricing decisions: Immutable accepted price snapshot, quote validation, 30 kg operational limits, and backend quote expiry | Member 2 |
| 2026-10-26 | Reconciliation: Added cancellation endpoint `POST /api/v1/parcels/{id}/cancel`, updated `parcel_status_history` fields, `caller_scope`, and dual address specification notice | Member 2 |
| 2026-11-06 | Task 2: Implemented Parcel Management Domain Model (`internal/parcels/domain`). Added `Parcel` aggregate root, 9-state machine (`CREATED` through terminal states), Crockford Base32 tracking number format (`SD-YYYYMMDD-XXXXXX`), immutable address and accepted price snapshots, delivery attempt limit guards (max 3), and status history audit entries | Member 2 |
| 2026-11-06 | Task 4: Implemented Parcel Booking Application and Quote Acceptance Integration (`internal/parcels/application`, `internal/parcels/infrastructure`). Added `BookParcelUseCase` orchestrating atomic quote consumption, tracking number generation with bounded retry, parcel creation, initial CREATED status-history logging, and transactional outbox publication (`parcel.booked.v1`) within a single atomic PostgreSQL transaction via `platform/db.WithTx` | Member 2 |
| 2026-11-08 | Task 5: Implemented Parcel Lifecycle Application & Persistence Integration (`internal/parcels/application`, `internal/parcels/infrastructure`). Added `TransitionParcelStatusUseCase` and `ParcelLifecycleService` port for intra-monolith integration (Member 3 Dispatch & Delivery). Enforced optimistic concurrency control via version condition, audit trail logging in `parcel_status_history`, and atomic transactional outbox publication for all 8 status transition events (`parcel.assigned.v1`, `parcel.picked_up.v1`, `parcel.delivered.v1`, etc.) | Member 2 |
| 2026-11-10 | Task 8A: Added `StatusHistoryID` (`status_history_id`) to `ParcelStatusChangedPayload` and populated it in `TransitionParcelStatusUseCase.ExecuteTx` with the exact validated UUID persisted to `parcel_status_history.id` in the same `*sql.Tx`, preserving envelope `event_id` and `schema_version` `1.0.0`; enforced non-blank `StatusHistoryEntry.ID` in domain validation. | Member 2 |
| 2026-11-10 | Implemented idempotency migration (`20261110000003_create_idempotency_keys.{up,down}.sql`), `IdempotencyStore` application port/types, `PostgresIdempotencyRepository` (advisory lock `pg_advisory_xact_lock`, locked lookup, atomic expired-key replacement), `BookParcelUseCase.WithIdempotency` integration before quote consumption, unit tests, and `SetupTestDB` PostgreSQL integration tests. | Member 2 |
| 2026-11-11 | Phase 1 Read & Ownership Foundation: Added `FindByParcelID` to `StatusHistoryRepository`, `MerchantReader` port + `MerchantRepoAdapter`, `ErrForbiddenParcelAccess` & caller/merchant authorization sentinels, `GetParcelUseCase`, `GetParcelHistoryUseCase`, `ResolveParcelCallerForBooking`, and comprehensive unit tests enforcing CUSTOMER/approved-MERCHANT/ADMIN access and strict COURIER/DISPATCHER denial. | Member 2 |
| 2026-11-11 | Phase 2 Parcel HTTP Transport: Implemented `internal/parcels/transport` (`dto.go`, `handler.go`, `routes.go`, `handler_test.go`) exposing `POST /api/v1/parcels`, `GET /api/v1/parcels/{id}`, and `GET /api/v1/parcels/{id}/history` with Bearer JWT + role authorization, `APPROVED` merchant tenant checks, UUID `Idempotency-Key` validation, 64 KB body cap + raw SHA-256 hashing + `Idempotent-Replay: true` replays, strict unknown-field JSON rejection, `httpx` data/error envelopes, `swag` annotations, and updated `API.md`/`README.md`. | Member 2 |
| 2026-11-13 | Phase 3 API Startup Wiring & Swagger Integration: Implemented `cmd/api/wiring_member02.go` (`registerPricingModule`, `registerParcelsModule`), wired both modules into `cmd/api/wiring.go` under `if dbConn != nil` guard with production fatal checks and local fallback notices, added `cmd/api/wiring_member02_test.go` verifying route registration and security middleware enforcement, regenerated unified OpenAPI specs via `swag init`, and synchronized `docs/swagger.json`, `docs/swagger.yaml`, and `internal/events/swagger/swagger.json`. | Member 2 |
| 2026-11-13 | Parcel item details persistence: Preserved item_description and special_instructions across domain aggregate, application usecase, outbox event (parcel.booked.v1), PostgreSQL schema migration (20261113000001_add_parcel_item_details), repository save/find queries, and transport DTOs (POST & GET responses) with 500-character domain length validation. | Member 2 |
