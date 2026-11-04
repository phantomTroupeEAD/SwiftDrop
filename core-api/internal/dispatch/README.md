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
│   ├── collaborators.go     # TransactionManager, ParcelLifecycleUpdater, OutboxWriter ports
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
│   ├── postgres_courier_repo.go          # PostgresCourierRepository (FOR UPDATE pessimistic row locking)
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
- `couriers`: Courier ID, user ID reference (`users.id`), vehicle type, max capacity, active shift status, `current_zone_id` (`delivery_zones.id`). Migrated via `20261021000006_create_couriers.up.sql` (Canonical Sequence 000006).
- `courier_assignments`: Courier ID, parcel ID, assigned by, status (`ACTIVE`, `COMPLETED`, `REASSIGNED`, `CANCELLED`), assigned at, ended at. Concurrency-guarded via partial unique index `idx_assignments_parcel_single_active`. Migrated via `20261021000007_create_dispatch_and_delivery.up.sql` (Canonical Sequence 000007).

---

## 5. Events Emitted
- `parcel.assigned.v1`: Published upon courier assignment or reattempt rescheduling.
- `courier.unassigned.v1`: Published upon assignment cancellation.
- `parcel.out_for_delivery.v1`: Published when courier marks run started.

---

## 6. System Wiring & Dependency Injection
All dispatch components are centrally registered in `core-api/cmd/api/wiring_dispatch_delivery.go` via `registerDispatchAndDeliveryModules(mux, dbConn, security)`.
- When PostgreSQL is connected (`dbConn != nil`), concrete persistence drivers with `FOR UPDATE` pessimistic row locking are instantiated.
- When running in local development mode without a database (`dbConn == nil`), an in-memory mock fallback is wired with pre-seeded test couriers so routes remain responsive.

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
