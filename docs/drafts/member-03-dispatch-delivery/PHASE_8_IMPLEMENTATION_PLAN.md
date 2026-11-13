# Phase 8 Implementation Plan: Living Documentation Final Alignment & Verification

> **Module:** Dispatch & Delivery (`internal/dispatch/`, `internal/delivery/`, `tests/`, `docs/`)  
> **Author:** Member 3 — Dispatch, Couriers & Proof of Delivery  
> **Governing Standards:** `AGENTS.md` (§6, §10, §12), SRS v1.0 (§4.6, §4.7), ADR-004  
> **Status:** Pending Review & Execution

---

## 1. Executive Summary & Objectives

Phase 8 is the final phase of Member 3's implementation lifecycle: **Living Documentation Final Alignment & Zero-Drift Verification**.

### Primary Objectives:
1. **Contract Audit & Zero-Drift Verification:**
   - Audit all 14 REST API endpoints across `core-api/internal/dispatch/API.md` and `core-api/internal/delivery/API.md` against actual Go transport handlers and route patterns.
   - Audit database migrations (`000006_create_couriers` and `000007_create_dispatch_and_delivery`) against module `README.md` files and `database-schema.md`.
   - Audit domain event schemas and outbox envelopes (`parcel.assigned.v1`, `courier.unassigned.v1`, `parcel.out_for_delivery.v1`, `parcel.delivered.v1`, `parcel.delivery_failed.v1`, `parcel.returned.v1`).
2. **Central Architecture & Integration Harness Alignment:**
   - Update `core-api/tests/README.md` to document execution instructions for Member 3's acceptance test suites (AT-006 Courier IDOR, AT-013 Assignment Concurrency, ADR-004 Three-Attempt Ceiling, and Transaction Atomicity).
   - Verify alignment with `docs/architecture/domain-boundaries.md` regarding allowed parcel status transitions and cross-context port interfaces.
3. **Comprehensive Completion Verification:**
   - Confirm complete alignment across all 8 phases of Member 3's roadmap.
   - Deliver the final Task Completion Report and readiness sign-off for human owner review.

---

## 2. Documentation Audit Matrix

| Document Path | Target Capability | Audit Criteria |
| :--- | :--- | :--- |
| `core-api/internal/dispatch/README.md` | Dispatch & Fleet Context | Reflects domain entities, application use cases, infrastructure drivers, transport handlers, system DI wiring, and AT-013 verification. |
| `core-api/internal/dispatch/API.md` | Dispatch REST Contract | Complete request/response schemas, JSON examples, query params, and error codes for all 7 dispatch endpoints. |
| `core-api/internal/delivery/README.md` | Delivery & POD Context | Reflects domain models, ADR-004 reason codes, storage drivers, transport handlers, Agreement A2 probe decoupling, and AT-006/ADR-004 verification. |
| `core-api/internal/delivery/API.md` | Delivery REST Contract | Complete request/response schemas, multipart form specifications, JSON examples, and error codes for all 7 delivery endpoints. |
| `core-api/tests/README.md` | Integration & Acceptance Harness | Documents commands for running Member 3 unit tests, concurrency tests (`-race`), and integration tests (`INTEGRATION=1`). |
| `docs/drafts/member-03-dispatch-delivery/MEMBER_03_IMPLEMENTATION_PLAN.md` | Master Roadmap | Update status of all 8 phases to COMPLETED. |

---

## 3. Detailed Audit Checklist

### 3.1 Endpoint Contract Checklist (14 Endpoints)
- [ ] `POST /api/v1/dispatch/couriers` (Registration, 201 Created)
- [ ] `GET /api/v1/dispatch/couriers` (Listing & zone filtering, 200 OK)
- [ ] `GET /api/v1/dispatch/board` (Dispatch board aggregation, 200 OK)
- [ ] `POST /api/v1/dispatch/assignments` (Serialized assignment, 201 Created / 409 Conflict)
- [ ] `POST /api/v1/dispatch/assignments/{id}/cancel` (Cancellation, 200 OK)
- [ ] `POST /api/v1/dispatch/parcels/{id}/reschedule` (Reattempt authorization, 200 OK)
- [ ] `POST /api/v1/courier/shift` (Shift status toggling, 200 OK)
- [ ] `GET /api/v1/courier/parcels` (Data-minimized manifest, 200 OK)
- [ ] `POST /api/v1/courier/parcels/{id}/start-run` (Individual start run, 200 OK)
- [ ] `POST /api/v1/courier/runs/start-batch` (Batch start run, 200 OK)
- [ ] `POST /api/v1/deliveries/{id}/proof-photo` (Multipart photo upload, 201 Created)
- [ ] `POST /api/v1/deliveries/{id}/complete` (Doorstep completion with POD, 200 OK)
- [ ] `POST /api/v1/deliveries/{id}/failed` (ADR-004 failed attempt & ceiling, 200 OK)
- [ ] `GET /api/v1/deliveries/parcels/{id}` (Attempt history & proof URLs, 200 OK)

### 3.2 Acceptance & Invariant Guarantees
- [ ] **AT-013 Concurrency Guard:** `idx_assignments_parcel_single_active` and `SELECT ... FOR UPDATE` serialization.
- [ ] **AT-006 Courier IDOR Defense:** `auth.UserID == assignment.courier_id` enforcement on all courier routes.
- [ ] **ADR-004 Three-Attempt Ceiling:** Max 3 attempts, immediate `RETURNED` upon `REFUSED_BY_RECIPIENT`.
- [ ] **Agreement A2 Readiness Probe Decoupling:** `/health/ready` evaluates PostgreSQL only; storage is decoupled.
- [ ] **Single Transaction Atomicity:** Aggregate state, status history, attempt logs, and outbox events commit together.

---

## 4. Execution Steps Checklist

- [ ] **Step 1:** Audit and synchronize `core-api/tests/README.md` with Member 3 test suite documentation.
- [ ] **Step 2:** Final review and polish of `core-api/internal/dispatch/README.md` and `API.md`.
- [ ] **Step 3:** Final review and polish of `core-api/internal/delivery/README.md` and `API.md`.
- [ ] **Step 4:** Update master roadmap `docs/drafts/member-03-dispatch-delivery/MEMBER_03_IMPLEMENTATION_PLAN.md` marking Phases 1 through 8 as complete.
- [ ] **Step 5:** Perform repository-wide integrity verification across all files created and modified.
- [ ] **Step 6:** Deliver final Task Completion Report per `AGENTS.md` §12.

---

## 5. Definition of Done (DoD) for Phase 8

Phase 8 is complete when:
1. Zero drift exists between implementation code and all co-located `README.md` and `API.md` files.
2. `core-api/tests/README.md` provides complete, actionable commands for executing Member 3 unit and integration suites.
3. Master implementation roadmap reflects 100% completion across all 8 phases.
4. Final completion report is rendered.

