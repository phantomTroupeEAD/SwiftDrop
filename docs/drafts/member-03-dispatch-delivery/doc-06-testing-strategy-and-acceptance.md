# Document 6 — Testing Strategy & Acceptance Specification

> **Module:** Dispatch, Couriers & Proof of Delivery  
> **Author / Owner:** Member 03  
> **Status:** Draft / Baseline (For Review in Team Meeting 2)  
> **Target Production Code Paths:** `core-api/internal/dispatch/` and `core-api/internal/delivery/`  
> **Test File Locations:** `core-api/internal/dispatch/..._test.go`, `core-api/internal/delivery/..._test.go`, `core-api/tests/integration/`  
> **Traceability:** SRS §4.6 (`FR-COU-001` to `FR-COU-007`), §4.7 (`FR-ADM-001` to `FR-ADM-004`), Acceptance Tests **AT-006**, **AT-013**

---

## 1. Executive Summary & Testing Philosophy

To guarantee that Member 03's operational capabilities perform reliably under high-concurrency dispatching and adverse mobile network conditions, testing follows a strict **Three-Tier Testing Pyramid**:

```mermaid
flowchart TD
    subgraph Pyramid ["Member 03 Testing Pyramid"]
        L1["Layer 1: Unit Tests (Go stdlib testing & testify)<br/>Pure domain models, value objects, mock repositories, zero I/O"]
        L2["Layer 2: Integration Tests (PostgreSQL + MinIO/R2)<br/>Real database transactions, row-level locks, outbox atomicity"]
        L3["Layer 3: System & Acceptance Tests (SRS Scenarios)<br/>Multi-actor workflows, courier IDOR isolation, concurrent assignments"]
    end

    L1 --> CI["Automated Verification Pipeline (scripts/verify.sh)"]
    L2 --> CI
    L3 --> CI
```

### Core Verification Directives (`AGENTS.md` §9):
- **Zero Test Weakening:** Existing assertions must never be deleted, commented out, or mocked away to achieve green builds.
- **Race Condition Detection:** All Go unit and integration tests must run with the Go race detector enabled (`go test -race`).
- **Deterministic Offline Execution:** Unit and integration test suites must execute cleanly in local environments and CI without requiring paid cloud API keys or external internet connectivity.

---

## 2. Unit Test Suite Design (`*_test.go`)

Unit tests reside co-located with domain and application code in `internal/dispatch/` and `internal/delivery/`. They execute completely in memory with zero database or network dependencies.

### 2.1 Fleet & Dispatch Domain Unit Tests (`internal/dispatch/domain/`)

| Test File | Target Struct / Method | Test Scenarios | Expected Assertions |
| :--- | :--- | :--- | :--- |
| `courier_test.go` | `Courier.CanAcceptAssignment` | 1. Courier is `OFF_SHIFT`<br>2. Courier is `SUSPENDED`<br>3. Active workload equals `max_capacity`<br>4. Active workload is under `max_capacity` | 1. Returns `ErrCourierNotOnShift`<br>2. Returns `ErrCourierSuspended`<br>3. Returns `ErrCourierOverloaded`<br>4. Returns `nil` (Success) |
| `courier_test.go` | `Courier.ToggleShift` | 1. Self-transition `OFF_SHIFT` $\rightarrow$ `ON_SHIFT`<br>2. Self-transition `ON_SHIFT` $\rightarrow$ `ON_BREAK`<br>3. Self-transition to `SUSPENDED` by courier | 1. Status updated to `ON_SHIFT`<br>2. Status updated to `ON_BREAK`<br>3. Returns `ErrInvalidShiftTransition` |
| `vehicle_type_test.go` | `VehicleType.Validate` | 1. `MOTORCYCLE`, `VAN`, `TRUCK`<br>2. Invalid strings (`BICYCLE`, `CAR`, `DRONE`) | 1. Returns `nil`<br>2. Returns `ErrInvalidVehicleType` |
| `assignment_test.go` | `CourierAssignment.Cancel` | 1. Close active assignment with note<br>2. Attempt to cancel already completed assignment | 1. Status set to `CANCELLED`, `ended_at` populated<br>2. Returns `ErrAssignmentAlreadyClosed` |

#### Sample Domain Unit Test: `courier_test.go`
```go
package domain_test

import (
	"testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"swiftdrop/core-api/internal/dispatch/domain"
)

func TestCourier_CanAcceptAssignment(t *testing.T) {
	c := &domain.Courier{
		UserID:      uuid.New(),
		MaxCapacity: 25,
		Status:      domain.ShiftStatusOnShift,
		IsActive:    true,
	}

	// 1. Success path: Workload below limit
	assert.NoError(t, c.CanAcceptAssignment(24))

	// 2. Overload path: Workload at maximum capacity
	err := c.CanAcceptAssignment(25)
	assert.ErrorIs(t, err, domain.ErrCourierOverloaded)

	// 3. Shift status check: Off-shift driver cannot accept assignments
	c.Status = domain.ShiftStatusOffShift
	err = c.CanAcceptAssignment(10)
	assert.ErrorIs(t, err, domain.ErrCourierNotOnShift)
}
```

---

### 2.2 Proof of Delivery & Failure Reason Unit Tests (`internal/delivery/domain/`)

| Test File | Target Struct / Method | Test Scenarios | Expected Assertions |
| :--- | :--- | :--- | :--- |
| `proof_of_delivery_test.go` | `ProofOfDelivery.Validate` | 1. `PHOTO` with valid storage URL<br>2. `PHOTO` with empty URL<br>3. `SIGNATURE` with vector data $\ge 50$ chars & recipient name<br>4. `SIGNATURE` missing recipient name<br>5. `SIGNATURE` with short stroke data ($< 50$ chars) | 1. Returns `nil`<br>2. Returns `ErrMissingProofPhotoURL`<br>3. Returns `nil`<br>4. Returns `ErrRecipientNameRequired`<br>5. Returns `ErrInvalidSignatureData` |
| `failure_reason_test.go` | `FailureReasonCode.Validate` | 1. 7 standard approved reason codes<br>2. Arbitrary unapproved string (e.g. `TIRED`)<br>3. Reason `OTHER` with note $\ge 15$ chars<br>4. Reason `OTHER` with short note ($< 15$ chars) | 1. Returns `nil`<br>2. Returns `ErrInvalidFailureReasonCode`<br>3. Returns `nil`<br>4. Returns `ErrFailureNoteTooShort` |
| `delivery_attempt_test.go` | `DeliveryAttempt.IsTerminal` | 1. Attempt 3 failure with any code<br>2. Attempt 1 failure with `REFUSED_BY_RECIPIENT`<br>3. Attempt 1 failure with `RECIPIENT_UNAVAILABLE` | 1. Returns `true`<br>2. Returns `true`<br>3. Returns `false` (Awaiting reattempt) |

---

## 3. Integration Test Suite Design (Real PostgreSQL)

Integration tests validate transaction boundaries, database constraints, pessimistic locks, and concurrency protection using a real PostgreSQL test instance (via Docker Compose or `testcontainers-go`).

### 3.1 Concurrency & Race Condition Test (**AT-013**)
- **Test File:** `core-api/tests/integration/dispatch_concurrency_test.go`
- **Objective:** Prove that concurrent assignment requests for the same parcel never produce dual active courier assignments or database corruption.
- **Execution Plan:**
  1. Seed 1 parcel in `CREATED` status (`prcl_race_01`).
  2. Seed 2 couriers in `ON_SHIFT` status (`courier_bob`, `courier_alice`).
  3. Launch **20 concurrent goroutines** simulating 2 dispatchers clicking "Assign" simultaneously.
  4. Collect HTTP / UseCase status codes.
- **Pass Criteria:**
  - Exactly **1 goroutine succeeds** with `200 OK`.
  - Exactly **19 goroutines fail** with `409 Conflict` (`PARCEL_ALREADY_ASSIGNED`).
  - Database verification: `SELECT COUNT(*) FROM courier_assignments WHERE parcel_id = 'prcl_race_01' AND status = 'ACTIVE'` equals **exactly 1**.

```go
func TestConcurrentParcelAssignment_AT013(t *testing.T) {
	ctx := context.Background()
	parcelID := seedTestParcel(t, "CREATED")
	courier1 := seedTestCourier(t, "CR-01", 25)
	courier2 := seedTestCourier(t, "CR-02", 25)

	concurrency := 20
	var wg sync.WaitGroup
	results := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		targetCourier := courier1
		if i%2 == 0 { targetCourier = courier2 }

		go func(cID uuid.UUID) {
			defer wg.Done()
			err := assignUseCase.Execute(ctx, parcelID, cID, dispatcherID)
			results <- err
		}(targetCourier)
	}

	wg.Wait()
	close(results)

	successCount := 0
	conflictCount := 0
	for err := range results {
		if err == nil {
			successCount++
		} else if errors.Is(err, domain.ErrParcelAlreadyAssigned) {
			conflictCount++
		}
	}

	assert.Equal(t, 1, successCount, "Exactly one assignment must succeed")
	assert.Equal(t, 19, conflictCount, "All other concurrent attempts must be rejected with 409 Conflict")
}
```

---

### 3.2 Single Database Transaction Rollback Test
- **Test File:** `core-api/tests/integration/atomic_rollback_test.go`
- **Objective:** Prove that if a failure occurs while recording a delivery (e.g. outbox event serialization fails), the entire transaction rolls back cleanly.
- **Pass Criteria:**
  - Injected error triggers `tx.Rollback()`.
  - `delivery_attempts` table contains zero new rows for this parcel.
  - `parcels.current_status` remains `OUT_FOR_DELIVERY` (does not become `DELIVERED`).
  - `outbox_events` contains zero records.

---

## 4. Security & Authorization Test Suite (IDOR & RBAC)

### 4.1 Courier IDOR & Object Isolation (**AT-006**)
- **Test File:** `core-api/tests/integration/courier_idor_test.go`
- **Scenarios Evaluated:**
  1. **Unauthorized Manifest Access:** Courier Bob requests `GET /api/v1/courier/parcels` $\rightarrow$ returns only parcels assigned to Bob; Alice's parcels are completely invisible.
  2. **Unauthorized Delivery Completion:** Courier Bob attempts to complete delivery on Courier Alice's assigned parcel via `POST /api/v1/deliveries/prcl_alice_01/complete`:
     - System rejects request with `403 Forbidden` (`NOT_ASSIGNED_COURIER`).
     - Parcel status remains unchanged.
  3. **Unauthorized Attempt Logging:** Courier Bob attempts to log a failed attempt on Alice's parcel $\rightarrow$ rejected with `403 Forbidden`.

### 4.2 Dispatcher Authorization Gate Test
- **Test File:** `core-api/tests/integration/dispatcher_gate_test.go`
- **Scenarios Evaluated:**
  1. **Courier Self-Reschedule Attempt:** Courier attempts to call `POST /api/v1/dispatch/parcels/{id}/reschedule`:
     - System rejects with `403 Forbidden` (`DISPATCHER_APPROVAL_REQUIRED`).
  2. **Dispatcher Reschedule:** Authenticated Dispatcher calls the same endpoint:
     - System accepts with `200 OK`; parcel status moves back to `ASSIGNED`.

---

## 5. Traceability Matrix & SRS Acceptance Mapping

| SRS Req / Test ID | Description | Enforcing Component | Verification Level | Test File |
| :--- | :--- | :--- | :--- | :--- |
| **FR-COU-001** | Courier views assigned deliveries | `internal/delivery` | Integration / API | `courier_manifest_test.go` |
| **FR-COU-002** | Courier views only permitted details | Transport DTO | Unit / API | `courier_dto_test.go` |
| **FR-COU-003** | Courier updates only assigned parcels | Application Auth | Integration (**AT-006**) | `courier_idor_test.go` |
| **FR-COU-004** | Mandatory proof for completion | Domain Value Object | Unit / Integration | `proof_of_delivery_test.go` |
| **FR-COU-005** | Mandatory reason for failed attempt | Domain Enum / ADR-004 | Unit / Integration | `failure_reason_test.go` |
| **FR-ADM-001** | Dispatcher lists unassigned parcels | `internal/dispatch` | Integration / Query | `dispatch_board_test.go` |
| **FR-ADM-002** | Dispatcher assigns parcel to courier | `internal/dispatch` | Integration / UseCase | `assign_courier_test.go` |
| **FR-ADM-003** | Prevent concurrent conflicting assignments | Pessimistic Lock | Integration (**AT-013**) | `dispatch_concurrency_test.go` |
| **FR-ADM-004** | Dispatcher monitors courier workloads | Repository Query | Integration | `courier_workload_test.go` |
| **ADR-004** | 3-Attempt ceiling & terminal `RETURNED` | Delivery Aggregate | Integration | `reattempt_lifecycle_test.go` |

---

## 6. Test Execution Runbook

### 6.1 Running Unit Tests Locally
```bash
# Run all Member 03 domain and application unit tests with race detection
go test -v -race -cover ./core-api/internal/dispatch/... ./core-api/internal/delivery/...
```

### 6.2 Running Integration Tests with PostgreSQL
```bash
# Start local test database
docker compose up -d postgres

# Run integration suite against local test instance
TEST_DB_URL="postgres://postgres:postgres@localhost:5432/swiftdrop_test?sslmode=disable" \
go test -v -race -tags=integration ./core-api/tests/integration/...
```

### 6.3 Automated CI Verification (`scripts/verify.sh`)
The project's central verification script enforces that:
1. `gofmt` passes with zero formatting diffs.
2. `golangci-lint run ./core-api/...` passes with zero warnings.
3. All unit and integration test packages report `PASS` with code coverage $\ge 85\%$.
