# Phase 7 Implementation Plan: Acceptance Tests & Concurrency Verification

> **Module:** Dispatch & Delivery End-to-End Acceptance Tests (`tests/`)  
> **Author:** Member 3 — Dispatch, Couriers & Proof of Delivery  
> **Governing Standards:** SRS v1.0 (§4.6, §4.7), ADR-004, Acceptance Criteria AT-006 & AT-013, `AGENTS.md` (§3, §4.3, §9)  
> **Status:** Pending Review & Execution

---

## 1. Executive Summary & Testing Objectives

Phase 7 implements the formal **End-to-End Acceptance and Concurrency Verification Test Suite** for Member 3.

### Primary Acceptance Criteria:
1. **AT-013 Assignment Concurrency Serialization:**
   - Concurrently execute 10 simultaneous goroutines attempting to assign the same parcel (`parcel_id`) to different couriers.
   - Verify that pessimistic row locking (`SELECT ... FOR UPDATE`) and the partial unique index `idx_assignments_parcel_single_active` serialize all requests.
   - Exactly ONE assignment request must succeed (`201 Created` / `200 OK`).
   - The other 9 requests must be rejected with `409 Conflict` (`PARCEL_ALREADY_ASSIGNED` / `COURIER_CAPACITY_EXCEEDED`).
   - Exactly ONE active assignment must exist in persistence.
2. **AT-006 Courier IDOR Defense & Tenant/Role Isolation:**
   - Parcel is assigned to Courier A (`user_id = CourierA`).
   - Courier B (`user_id = CourierB`) attempts to call:
     - `POST /api/v1/courier/parcels/{id}/start-run`
     - `POST /api/v1/deliveries/{id}/complete`
     - `POST /api/v1/deliveries/{id}/failed`
     - `POST /api/v1/deliveries/{id}/proof-photo`
   - Verify that all unauthorized attempts by Courier B are rejected with `403 Forbidden` (`COURIER_NOT_ASSIGNED`).
   - Verify Courier A can successfully complete the delivery with valid proof.
3. **ADR-004 Delivery Attempt Ceiling & Terminal Return Transition:**
   - Verify sequential recording of 3 failed attempts with approved reason codes.
   - Attempt 1: Outcome `FAILED` $\to$ parcel status `ATTEMPT_FAILED`.
   - Attempt 2: Outcome `FAILED` $\to$ parcel status `ATTEMPT_FAILED`.
   - Attempt 3: Outcome `FAILED` $\to$ parcel transitions to terminal `RETURNED`, assignment marked `COMPLETED`.
   - Attempt 4: Blocked with `409 Conflict` (`ATTEMPT_CEILING_EXCEEDED`).
   - Immediate Terminal Case: Attempt 1 with reason `REFUSED_BY_RECIPIENT` transitions directly to `RETURNED`.
4. **Transaction Atomicity & Rollback:**
   - Force an outbox failure or DB constraint error halfway through delivery completion.
   - Verify that parcel status, delivery attempt log, and assignment state roll back atomically with zero partial commits.

---

## 2. Test Architecture & Concurrency Model

```mermaid
sequenceDiagram
    autonumber
    participant D1 as Dispatcher 1 (Goroutine 1)
    participant D2 as Dispatcher 2 (Goroutine 2..10)
    participant Lock as PostgreSQL / DB Lock
    participant DB as courier_assignments Table
    participant Outbox as outbox_events Table

    rect rgb(230, 245, 230)
    Note over D1,DB: AT-013 Concurrent Assignment Race
    par Dispatcher 1 (Wins Lock)
        D1->>Lock: SELECT ... FOR UPDATE (Parcel / Courier)
        Lock-->>D1: Lock Acquired
        D1->>DB: INSERT INTO courier_assignments (status = 'ACTIVE')
        D1->>Outbox: INSERT INTO outbox_events (parcel.assigned.v1)
        D1-->>Lock: COMMIT Tx
        D1-->>D1: HTTP 201 Created (Success)
    and Dispatcher 2..10 (Serialized)
        D2->>Lock: Wait for Lock / Check Active Assignment
        Lock-->>D2: Lock Released; Active Assignment Detected
        D2-->>D2: HTTP 409 Conflict (PARCEL_ALREADY_ASSIGNED)
    end
    end
```

---

## 3. Test Suite File Structure

```
core-api/tests/
├── at_013_assignment_concurrency_test.go  # AT-013: 10 concurrent assignment race test
├── at_006_courier_idor_test.go            # AT-006: Courier IDOR protection & proof completion
├── adr_004_delivery_ceiling_test.go       # ADR-004: 3-attempt ceiling and REFUSED_BY_RECIPIENT
├── delivery_atomicity_test.go             # Atomicity & single transaction rollback test
├── testenv.go                             # Shared PostgreSQL test schema initializer
└── smoke_test.go                          # Cross-module smoke test
```

---

## 4. Test Specifications

### 4.1 AT-013 Concurrency Test (`at_013_assignment_concurrency_test.go`)
- **Setup:**
  - Create 1 target parcel in `CREATED` status.
  - Create 10 registered on-shift couriers (`CR-01` to `CR-10`), each with capacity 25.
  - Generate valid JWT for Dispatcher.
- **Action:**
  - Launch 10 concurrent goroutines using `sync.WaitGroup` and a synchronization start barrier (`sync.WaitGroup` or channel trigger).
  - Each goroutine posts to `POST /api/v1/dispatch/assignments` with target parcel and its assigned courier.
- **Assertions:**
  - Exactly 1 response returns HTTP `201 Created`.
  - Exactly 9 responses return HTTP `409 Conflict` (code `PARCEL_ALREADY_ASSIGNED`).
  - Active assignment count for target parcel in persistence is exactly 1.
  - Parcel status in persistence is `ASSIGNED`.

### 4.2 AT-006 Courier IDOR Test (`at_006_courier_idor_test.go`)
- **Setup:**
  - Create Courier A (`CR-101`) and Courier B (`CR-102`).
  - Assign parcel to Courier A; status `ASSIGNED`.
  - Issue JWT access tokens for Courier A and Courier B.
- **Action & Assertions:**
  - Courier B calls `POST /api/v1/courier/parcels/{id}/start-run` $\to$ Assert `403 Forbidden` (`COURIER_NOT_ASSIGNED`).
  - Courier A calls `POST /api/v1/courier/parcels/{id}/start-run` $\to$ Assert `200 OK` (parcel status `OUT_FOR_DELIVERY`).
  - Courier B calls `POST /api/v1/deliveries/{id}/proof-photo` $\to$ Assert `403 Forbidden` (`COURIER_NOT_ASSIGNED`).
  - Courier B calls `POST /api/v1/deliveries/{id}/complete` $\to$ Assert `403 Forbidden` (`COURIER_NOT_ASSIGNED`).
  - Courier B calls `POST /api/v1/deliveries/{id}/failed` $\to$ Assert `403 Forbidden` (`COURIER_NOT_ASSIGNED`).
  - Courier A calls `POST /api/v1/deliveries/{id}/complete` with valid POD $\to$ Assert `200 OK` (parcel status `DELIVERED`).

### 4.3 ADR-004 Delivery Attempt Ceiling Test (`adr_004_delivery_ceiling_test.go`)
- **Action & Assertions:**
  - Parcel assigned and moved to `OUT_FOR_DELIVERY`.
  - Failed Attempt 1: `RECIPIENT_UNAVAILABLE` $\to$ Assert `200 OK`, `attempt_number: 1`, `parcel_status: ATTEMPT_FAILED`.
  - Re-dispatch / resume run $\to$ parcel back in `OUT_FOR_DELIVERY`.
  - Failed Attempt 2: `ACCESS_RESTRICTED` $\to$ Assert `200 OK`, `attempt_number: 2`, `parcel_status: ATTEMPT_FAILED`.
  - Re-dispatch / resume run $\to$ parcel back in `OUT_FOR_DELIVERY`.
  - Failed Attempt 3: `CUSTOMER_REQUESTED_RESCHEDULE` $\to$ Assert `200 OK`, `attempt_number: 3`, `parcel_status: RETURNED`, `is_terminal: true`.
  - Attempt 4: Try recording another attempt on the returned parcel $\to$ Assert `409 Conflict` (blocked by terminal state / attempt ceiling).
  - Immediate Terminal Sub-test: New parcel in `OUT_FOR_DELIVERY`; Attempt 1 recorded with `REFUSED_BY_RECIPIENT` $\to$ Assert `200 OK`, `parcel_status: RETURNED`, `is_terminal: true`.

### 4.4 Delivery Atomicity Test (`delivery_atomicity_test.go`)
- **Action & Assertions:**
  - Mock outbox writer to return error on `Write()`.
  - Courier executes `CompleteDelivery`.
  - Assert HTTP error is returned.
  - Verify that parcel status remains `OUT_FOR_DELIVERY`, no delivery attempt record was committed, and courier assignment remains `ACTIVE`.

---

## 5. Execution Steps Checklist

- [ ] **Step 1:** Implement `core-api/tests/at_013_assignment_concurrency_test.go`.
- [ ] **Step 2:** Implement `core-api/tests/at_006_courier_idor_test.go`.
- [ ] **Step 3:** Implement `core-api/tests/adr_004_delivery_ceiling_test.go`.
- [ ] **Step 4:** Implement `core-api/tests/delivery_atomicity_test.go`.
- [ ] **Step 5:** Execute test suite with race detector: `go test -race -v ./tests/...`.
- [ ] **Step 6:** Update co-located living documentation (`internal/dispatch/README.md` and `internal/delivery/README.md`).
- [ ] **Step 7:** Deliver Task Completion Report per `AGENTS.md` §12.

---

## 6. Definition of Done (DoD) for Phase 7

Phase 7 is complete and approved for advancement to **Phase 8 (Living Documentation Final Alignment)** only when:
1. AT-013 concurrency test proves 10 concurrent goroutines racing on the same parcel result in exactly 1 successful assignment and 9 conflict rejections.
2. AT-006 IDOR test proves a foreign courier is barred from all delivery actions with HTTP 403.
3. ADR-004 test verifies the 3-attempt ceiling and immediate return on `REFUSED_BY_RECIPIENT`.
4. Transaction atomicity test verifies complete rollback upon persistence/outbox failures.
5. All tests pass with zero data races (`-race`).

