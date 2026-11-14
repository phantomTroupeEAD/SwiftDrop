# Dispatch & Couriers Module (`internal/dispatch`)

> **Domain:** Courier Assignment, Fleet Capacity & Dispatch Board  
> **Primary Owner:** Member 3 — Dispatch, Couriers & Proof of Delivery  
> **Clean Architecture Boundary:** Modular Monolith Internal Module

---

## 1. Module Overview & Responsibilities
The `dispatch` module governs operational logistics: managing active courier fleet capacity, dispatch board aggregation, and assigning/reassigning parcels to couriers.

### Key Responsibilities:
- Courier profiles, vehicle classifications, and active shift statuses.
- Courier load/capacity tracking (maximum concurrent deliveries).
- Dispatcher assignment, cancellation, and reattempt scheduling operations.
- Out-for-delivery milestone trigger in coordination with `internal/parcels`.
- Dispatcher board query aggregation.

---

## 2. Domain Invariants & Business Rules
1. **Capacity Limit:** A courier cannot be assigned more than their vehicle's maximum configured capacity (default: 25 active parcels for motorcycle, 60 for van, 120 for truck).
2. **Assignment Eligibility:** Parcels may only be assigned if their status is currently `CREATED` or `ATTEMPT_FAILED`.
3. **Assigned Status Transition:** Assigning a courier triggers the parcel state machine transition to `ASSIGNED`.
4. **Active Courier Shift:** Assignments may only be made to couriers with active, unblocked status on shift (`ON_SHIFT`).
5. **AT-013 Concurrency Safety:** Multiple dispatchers assigning the same parcel concurrently are serialized via pessimistic row locking; duplicate assignments are barred at the database engine level via partial unique index `idx_assignments_parcel_single_active`.
6. **BR-FLT-001 Courier Operational Profile Integrity:** Operational fleet profiles are strictly bound 1:1 with user accounts that possess the `COURIER` role. Registration rejects non-courier roles (`ADMIN`, `DISPATCHER`, `MERCHANT`, `CUSTOMER`) with `422 Unprocessable Entity` (`USER_NOT_A_COURIER`), missing user accounts with `404 Not Found` (`USER_NOT_FOUND`), and duplicate registrations with `409 Conflict` (`COURIER_PROFILE_EXISTS`).

---

## 3. Layer Breakdown
```
internal/dispatch/
├── API.md                   # Living REST API Specifications & Request/Response Contracts
├── domain/                  # Pure Domain Logic (Zero external dependencies)
│   ├── courier.go           # Courier aggregate root, capacity & shift invariant methods
│   ├── courier_test.go      # Unit tests for capacity limits, active workload & shift changes
│   ├── assignment.go        # CourierAssignment aggregate, Complete/Cancel/Reassign methods
│   ├── assignment_test.go   # Unit tests for assignment lifecycle & note validations
│   ├── vehicle_type.go      # VehicleType enum (MOTORCYCLE, VAN, TRUCK) & default capacities
│   ├── vehicle_type_test.go # Unit tests for vehicle validations & capacities
│   ├── shift_status.go      # ShiftStatus state machine (self-service vs administrative)
│   ├── shift_status_test.go # Unit tests for courier shift transitions
│   └── errors.go            # Sentinel domain errors (ErrCourierOverloaded, etc.)
├── application/             # Use Cases, Command Handlers & Collaborator Ports
│   ├── collaborators.go     # TransactionManager, ParcelLifecycleUpdater, OutboxWriter, UserRoleChecker ports
│   ├── repository.go        # CourierRepository, AssignmentRepository, DispatchBoardReader
│   ├── commands.go          # Inbound command structs and DTO response envelopes
│   ├── assign_courier.go    # AssignCourierUseCase (Pessimistic lock, capacity check, AT-013)
│   ├── cancel_assignment.go # CancelAssignmentUseCase (Terminates assignment, reverts to CREATED)
│   ├── reschedule_parcel.go # RescheduleParcelUseCase (ADR-004 dispatcher-gated reattempt)
│   ├── toggle_shift.go      # ToggleShiftUseCase & RegisterCourierUseCase
│   ├── get_dispatch_board.go# GetDispatchBoardQueryHandler & ListCouriersQueryHandler
│   └── application_test.go  # In-memory mock tests for assignment & capacity enforcement
├── infrastructure/          # Concrete Persistence Drivers & External Adapters
│   ├── postgres_tx_manager.go            # PostgresTransactionManager (database/sql ReadCommitted wrapper)
│   ├── postgres_courier_repo.go          # PostgresCourierRepository (FOR UPDATE pessimistic row locking, UserRoleChecker)
│   ├── postgres_assignment_repo.go       # PostgresAssignmentRepository (AT-013 unique index guard)
│   ├── postgres_dispatch_board_reader.go # PostgresDispatchBoardReader (Optimized aggregation queries)
│   ├── parcel_updater.go                 # PostgresParcelLifecycleUpdater (Aggregate lifecycle & audit trail)
│   ├── outbox_writer.go                  # PostgresOutboxWriter (Transactional outbox_events publisher)
│   └── postgres_infra_test.go            # Infrastructure constructors & smoke tests
└── transport/               # HTTP REST Handlers, Routing & Error Mappers
    ├── doc.go               # Transport package overview
    ├── requests.go          # Inbound request DTOs with validation rules
    ├── responses.go         # Outbound single-resource JSON response envelopes
    ├── error_mapper.go      # RFC 7807 / SwiftDrop canonical error status mapper
    ├── dispatch_handlers.go # Handlers for fleet registration, board queries & assignments
    ├── courier_handlers.go  # Handler for courier shift state transitions
    ├── routes.go            # ServeMux route registration with RBAC middleware
    └── transport_test.go    # Unit tests with httptest covering happy & error paths
```

---

## 4. Database Ownership
- `couriers`: Courier ID, user ID reference (`users.id`), vehicle type, max capacity, active shift status, `current_zone_id` (`delivery_zones.id`). Migrated via `20261110000004_create_couriers.up.sql` (Canonical Sequence 000004).
- `courier_assignments`: Courier ID, parcel ID, assigned by, status (`ACTIVE`, `COMPLETED`, `REASSIGNED`, `CANCELLED`), assigned at, ended at. Concurrency-guarded via partial unique index `idx_assignments_parcel_single_active`. Migrated via `20261110000005_create_dispatch_and_delivery.up.sql` (Canonical Sequence 000005).

---

## 5. Events Emitted
- `parcel.assigned.v1`: Canonical event emitted by Member 2's `TransitionParcelStatusUseCase.ExecuteTx` within the same transaction as courier assignment (includes `tracking_number`, `new_status`, `status_history_id`, and transition metadata). Dispatch avoids duplicate event emission.
- `courier.unassigned.v1`: Published upon assignment cancellation.
- `parcel.out_for_delivery.v1`: Published when courier marks run started.

---

## 6. System Wiring & Dependency Injection
All dispatch components are centrally registered in `core-api/cmd/api/wiring_dispatch_delivery.go` via `registerDispatchAndDeliveryModules(mux, dbConn, security)`.
- **Authoritative Database Persistence:** Concrete PostgreSQL persistence drivers with `FOR UPDATE` pessimistic row locking are wired to guarantee ACID properties (Atomicity and Durability).
- **Parcel Lifecycle Integration:** `PostgresParcelLifecycleUpdater` delegates state transitions directly to Member 2's `TransitionParcelStatusUseCase.ExecuteTx` in the active database transaction, preserving aggregate invariants, optimistic concurrency control, immutable audit history (`parcel_status_history`), and canonical outbox publication.
- **Fail-Fast Offline Invariant:** When the database connection is unavailable (`dbConn == nil`), operations requiring persistence fail immediately with `503 Service Unavailable` (`DATABASE_UNAVAILABLE`). The system strictly forbids writing mutations to volatile in-memory maps, ensuring records never disappear silently upon application restart.

---

## 7. Acceptance Criteria & Concurrency Verification
- **AT-013 Concurrency Test (`tests/at_013_assignment_concurrency_test.go`):** 10 concurrent goroutines simultaneously race to assign the same parcel. Pessimistic row locking and partial unique index `idx_assignments_parcel_single_active` serialize all requests: exactly 1 returns `201 Created` and 9 return `409 Conflict` (`PARCEL_ALREADY_ASSIGNED`), guaranteeing exactly 1 active assignment.

---

## 8. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Standardized migration splitting (000006/000007), current_zone_id FK to delivery_zones, and assignment status enum (ACTIVE) | Member 3 & Review Agent |
| 2026-11-02 | Implemented Phase 1 DDL migrations `20261021000006_create_couriers.up.sql` and `20261021000007_create_dispatch_and_delivery.up.sql` with symmetrical down rollbacks, partial unique concurrency index, and seeds | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 2 pure domain layer: Courier aggregate root, CourierAssignment, VehicleType, ShiftStatus state machine, domain errors, and in-memory unit tests | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 3 application layer: AssignCourierUseCase (AT-013), CancelAssignmentUseCase, RescheduleParcelUseCase (ADR-004), ToggleShiftUseCase, GetDispatchBoardQueryHandler, collaborator ports, and mock unit tests | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 4 infrastructure persistence layer: PostgresTransactionManager, PostgresCourierRepository (FOR UPDATE row locking), PostgresAssignmentRepository (AT-013 constraint handling), PostgresDispatchBoardReader, PostgresParcelLifecycleUpdater, and PostgresOutboxWriter | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 5 transport layer: DispatchHandler, CourierShiftHandler, ErrorMapper, Request/Response DTOs, Route registration with RBAC middleware, co-located living API.md, and httptest unit tests | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 6 system wiring: registered in cmd/api/wiring_dispatch_delivery.go, hooked into BuildAppContainer(), wired fallback mocks for local dev, and added container tests | Member 3 (Antigravity) |
| 2026-11-02 | Implemented Phase 7 acceptance tests: AT-013 10-goroutine assignment race test proving concurrency serialization and unique active assignment invariant | Member 3 (Antigravity) |
| 2026-11-03 | Phase 8 living documentation alignment: zero-drift contract verification against API.md, synchronized tests/README.md, and finalized roadmap sign-off | Member 3 (Antigravity) |
| 2026-11-09 | Eliminated in-memory mock fallback from runtime wiring; enforced ACID atomicity and durability by failing queries with 503 DATABASE_UNAVAILABLE when PostgreSQL is disconnected | Member 3 (Antigravity) |
| 2026-11-04 | Comprehensive test gap analysis & hardening: Added BVA tests for Courier capacity [1, 200] and zone trimming, assignment reassignment error paths, AssignmentStatus enum validation, transport negative tests for malformed JSON and payload bounds, and mapped ErrParcelNotEligibleForAssignment to 409 Conflict. | Member 3 (Antigravity) |
| 2026-11-10 | Resequenced migrations to 20261110000003_create_couriers and 20261110000004_create_dispatch_and_delivery, removed stub parcels table to sequence cleanly after Member 2 parcels | Member 3 (Antigravity) |
| 2026-11-10 | Enforced BR-FLT-001 role validation & uniqueness in RegisterCourierUseCase: rejects non-courier user accounts with 422 USER_NOT_A_COURIER, non-existent users with 404 USER_NOT_FOUND, and duplicate profiles with 409 COURIER_PROFILE_EXISTS. | Member 3 (Antigravity) |
| 2026-11-14 | Integrated Member 2's `TransitionParcelStatusUseCase.ExecuteTx` in `PostgresParcelLifecycleUpdater` for lifecycle transitions within existing database transactions; corrected PostgreSQL schema columns (`parcels.status`, `parcel_status_history.reason`, `actor_type`); removed duplicate `parcel.assigned.v1` outbox write in Dispatch workflow to ensure canonical event publication with tracking number, status history ID, and new status. | Member 3 (Antigravity) |
