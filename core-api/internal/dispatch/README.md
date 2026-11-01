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
- Dispatcher assignment and unassignment operations.
- Out-for-delivery milestone trigger in coordination with `internal/parcels`.
- Dispatcher board query aggregation.

---

## 2. Domain Invariants & Business Rules
1. **Capacity Limit:** A courier cannot be assigned more than their vehicle's maximum configured capacity (default: 25 active parcels).
2. **Assignment Eligibility:** Parcels may only be assigned if their status is currently `CREATED` or `ATTEMPT_FAILED`.
3. **Assigned Status Transition:** Assigning a courier triggers the parcel state machine transition to `ASSIGNED`.
4. **Active Courier Shift:** Assignments may only be made to couriers with active, unblocked status on shift.

---

## 3. Layer Breakdown
```
internal/dispatch/
├── domain/                  # Courier entity, Assignment aggregate, Capacity VO, domain errors
├── application/             # AssignCourier, UnassignCourier, GetDispatchBoard use cases
├── infrastructure/          # PostgresCourierRepository, PostgresDispatchQuery
└── transport/               # HTTP handlers (/api/v1/dispatch/*, /api/v1/couriers/*) and DTOs
```

---

## 4. Database Ownership
- `couriers`: Courier ID, user ID reference (`users.id`), vehicle type, max capacity, active shift status, `current_zone_id` (`delivery_zones.id`). Migrated via `000006_create_couriers.up.sql`.
- `courier_assignments`: Courier ID, parcel ID, assigned by, status (`ACTIVE`, `COMPLETED`, `REASSIGNED`, `CANCELLED`), assigned at, ended at. Migrated via `000007_create_dispatch_and_delivery.up.sql`.

---

## 5. Events Emitted
- `parcel.assigned.v1`: Published upon courier assignment.
- `parcel.out_for_delivery.v1`: Published when courier marks run started.

---

## 6. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial module architecture baseline created | Architecture Setup |
| 2026-10-25 | Standardized migration splitting (000006/000007), current_zone_id FK to delivery_zones, and assignment status enum (ACTIVE) | Member 3 & Review Agent |
