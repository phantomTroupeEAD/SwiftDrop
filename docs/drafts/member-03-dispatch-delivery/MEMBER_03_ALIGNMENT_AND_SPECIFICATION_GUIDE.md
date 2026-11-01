# Member 03 — Dispatch, Couriers & Proof of Delivery Comprehensive Audit, Review & Alignment Guide

> **Author / Domain Owner:** Member 3 — Dispatch, Couriers & Proof of Delivery (Poorna / Kalani)  
> **Target Modules:** `core-api/internal/dispatch/` and `core-api/internal/delivery/`  
> **Review Status:** Post-Submission Comprehensive Audit (Reviewing Full 8-Document Architecture Suite & Questionnaire)  
> **Date:** 25 October 2026 | **Version:** 2.0.0

---

## 1. Executive Summary & Review Status

Member 3 owns the operational heartbeat of physical parcel movement within SwiftDrop: **Courier Operational Profiles**, **Vehicle Capacities & Shifts**, **The Dispatch Board & Parcel Assignment Engine**, **Doorstep Delivery Execution**, **Proof-of-Delivery (POD) Verification**, and **Delivery Attempt Failure Classifications (ADR-004)**.

Member 3 has submitted an extensive specification suite consisting of 8 architectural documents (`doc-01` through `doc-08`), accompanied by an architectural decision questionnaire (`decisions-and-questionnaire.md`). The submitted documents demonstrate strong domain modeling and operational awareness. However, detailed cross-review identified several critical flaws: a database migration sequencing failure (declaring foreign keys to non-existent tables), cascading readiness probe failure risks (checking external cloud storage in Kubernetes pod readiness), embedded seeds in migration directories, unharmonized delivery zone references (`zone_code` vs `zone_id`), and courier assignment status enum mismatches.

This document executes the **exact same rigorous 7-point review and alignment process** applied across all team members:
1. Requirements and workload coverage audit against SwiftDrop SRS v1.0 and ADR-004.
2. Intra-member contradictions (internal inconsistencies across Member 3's 8 documents).
3. Inter-member contradictions (cross-boundary conflicts with Members 1, 2, 4, 5, 6, 7, 8, 9, and 10).
4. Document and workload flaws.
5. Systemic architectural, concurrency, storage, and lifecycle flaws.
6. Required changes, recommended implementations, and industry best practices.
7. Exact DevOps and Database Coordinator configurations per Member 9.

---

## 2. Workload & Requirements Coverage Audit

| Requirement ID | Description | Coverage in Submitted Docs | Assessment | Action / Resolution |
|:---|:---|:---|:---:|:---|
| **FR-DSP-001** | Courier operational profiles & shift lifecycle management. | Covered in `doc-01`, `doc-02` (Flow A), `doc-03`, `doc-04`. | **100% Complete** | Formalized shift state machine (`OFF_SHIFT`, `ON_SHIFT`, `ON_BREAK`, `SUSPENDED`). |
| **FR-DSP-002** | Motorized fleet vehicle types & capacity limits. | Covered in `doc-01`, `doc-02`, `doc-03`, `doc-05`. | **100% Complete** | Hardened vehicle capacities (`MOTORCYCLE`: 25, `VAN`: 60, `TRUCK`: 120). |
| **FR-DSP-003** | Dispatch board with zone filtering and workload aggregations. | Covered in `doc-01`, `doc-02` (Flow B), `doc-04`. | **100% Complete** | Standardized `GET /api/v1/dispatch/board` contract. |
| **FR-DSP-004** | Concurrency-safe courier parcel assignment with capacity check. | Covered in `doc-01`, `doc-02`, `doc-03`, `doc-07`. | **100% Complete** | Implemented pessimistic row locking (`SELECT FOR UPDATE`) on parcel assignment. |
| **FR-POD-001** | Courier active parcel manifest retrieval scoped to `auth.UserID`. | Covered in `doc-01`, `doc-02` (Flow C), `doc-04`. | **100% Complete** | IDOR-safe query `GET /api/v1/courier/parcels` filtering by authenticated courier. |
| **FR-POD-002** | Pickup confirmation and delivery run start (`OUT_FOR_DELIVERY`). | Covered in `doc-01`, `doc-02`, `doc-04`. | **100% Complete** | Single & batch delivery run initiation endpoints. |
| **FR-POD-003** | Tamper-resistant Proof of Delivery (Photo, Signature, Recipient Confirmation). | Covered in `doc-01`, `doc-02` (Flow D), `doc-04`, `doc-07`. | **100% Complete** | Enforced mandatory POD rules (`received_by_name` required for signature). |
| **FR-POD-004** | Cloud object storage integration for POD photos (Cloudflare R2 / MinIO). | Covered in `doc-01`, `doc-05`, `doc-08`. | **100% Complete** | Standardized S3/R2 presigned upload URL flow with 15-min TTL. |
| **FR-POD-005** | Delivery attempt failure logging, reason codes, and 3-attempt ceiling. | Covered in `doc-01`, `doc-02` (Flow E), `doc-04`, `doc-05`. | **100% Complete** | Implemented ADR-004 failure reason codes and terminal return after 3 attempts. |
| **ADR-004** | Dispatcher-gated reattempt authorization for failed parcels. | Covered in `doc-01`, `doc-02`, `doc-04`. | **100% Complete** | Enforced that couriers cannot unilaterally reattempt; requires dispatcher action. |

---

## 3. Intra-Member Contradictions (Within Member 3's Own Files)

Detailed inspection revealed 4 internal discrepancies across Member 3's submitted files:

```mermaid
flowchart TD
    subgraph IntraMemberDiscrepancies ["Member 03 Intra-Document Contradictions"]
        IC1["Contradiction 1: Delivery Zone Key<br/>(doc-01/04/05 zone_code FK to delivery_zones.code vs PK id)"]
        IC2["Contradiction 2: Assignment Status Enum<br/>(doc-05 ASSIGNED/COMPLETED/CANCELLED vs ACTIVE/REASSIGNED)"]
        IC3["Contradiction 3: Delivery Status Milestones<br/>(doc-01/04 start-run skipping PICKED_UP vs canonical state machine)"]
        IC4["Contradiction 4: Cloud Storage Environment Variables<br/>(doc-08 STORAGE_DRIVER vs Member 9 OBJECT_STORAGE_DRIVER)"]
    end
```

### 3.1 Contradiction 1: Delivery Zone Foreign Key (`zone_code` vs `zone_id`)
- **The Inconsistency:**
  - In `doc-01` line 239, `doc-04` Section 3.1, and `doc-05` Section 2, Member 3 defines `couriers.zone_code VARCHAR(50) REFERENCES delivery_zones(code)`.
  - In Member 7's canonical `delivery_zones` table (created in `000002_create_delivery_zones.up.sql`), the primary key is `id VARCHAR(32)` (e.g., `'COLOMBO_01'`). The table does not have a unique column named `code`.
- **The Problem:** A foreign key referencing a non-existent or non-unique column `delivery_zones(code)` causes migration failure in PostgreSQL (`ERROR: column "code" referenced in foreign key constraint does not exist`).
- **The Fix:** Standardize column as `current_zone_id VARCHAR(32) REFERENCES delivery_zones(id) ON DELETE SET NULL` across tables, structs, and DTOs.

### 3.2 Contradiction 2: Courier Assignment Status Values
- **The Inconsistency:**
  - In `doc-05` line 139, the check constraint on `courier_assignments` is:
    `CHECK (status IN ('ASSIGNED', 'COMPLETED', 'CANCELLED'))`.
  - In `doc-04` Section 2 line 49 and `decisions-and-questionnaire.md`, Member 3 introduces reassignment workflows.
  - In the capacity check query, the application queries:
    `WHERE courier_id = $1 AND status = 'ACTIVE'`.
- **The Problem:** If `courier_assignments` only permits `'ASSIGNED'`, `'COMPLETED'`, `'CANCELLED'`, then storing `'ACTIVE'` violates the check constraint, or querying `'ACTIVE'` will return 0 rows.
- **The Fix:** Standardize the assignment status lifecycle:
  ```sql
  status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' 
      CHECK (status IN ('ACTIVE', 'COMPLETED', 'REASSIGNED', 'CANCELLED'))
  ```

### 3.3 Contradiction 3: Delivery Status Milestones vs ADR-004
- **The Inconsistency:**
  - In `doc-01` Section 3.3 and `doc-04` Section 2 line 53, starting a delivery run (`POST /api/v1/courier/parcels/{id}/start-run`) moves the parcel directly from `ASSIGNED` to `OUT_FOR_DELIVERY`.
  - In `docs/architecture/domain-boundaries.md` and ADR-004, the sequence specifies `ASSIGNED` $\to$ `PICKED_UP` $\to$ `IN_TRANSIT` $\to$ `OUT_FOR_DELIVERY`.
- **The Fix:**
  - **Depot Hub Dispatch:** When a courier physically collects assigned packages at the depot and initiates their morning delivery run, the system accepts batch initiation (`POST /api/v1/courier/runs/start-batch`) which atomically updates parcel status through `PICKED_UP` into `OUT_FOR_DELIVERY`.
  - Both granular depot handovers (`PICKED_UP`) and final-mile road runs (`OUT_FOR_DELIVERY`) are fully supported by the domain state machine.

### 3.4 Contradiction 4: Cloud Storage Environment Variable Inconsistency
- **The Inconsistency:**
  - In `doc-08` lines 98–106, Member 3 configures `STORAGE_DRIVER`, `LOCAL_STORAGE_PATH`, `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`.
  - In `docs/architecture/environment-variables.md` (Member 9), the system-wide storage driver is named `OBJECT_STORAGE_DRIVER`.
- **The Fix:** Member 3's configuration loader must accept `OBJECT_STORAGE_DRIVER` as the primary key with fallback to `STORAGE_DRIVER`.

---

## 4. Inter-Member Contradictions & Cross-Boundary Alignment

```mermaid
flowchart TD
    subgraph CrossMemberAlignment ["Member 03 Cross-Boundary Reconciliation"]
        M3M9["M3 vs M9: Migration Sequencing & Health Probes<br/>(000003 crash vs Migrations 6 & 7; Decouple R2 from /health/ready)"]
        M3M2["M3 vs M2: Parcel Aggregate Root & Lifecycle<br/>(No direct SQL updates; Single-transaction commit with history)"]
        M3M1["M3 vs M1: Identity & RBAC Scoping<br/>(Courier accounts linked to users.id; IDOR protection)"]
        M3M7["M3 vs M7: Frontend Dispatcher & Courier UI<br/>(REST API contracts consumed by React web portals)"]
        M3M8["M3 vs M8: Transactional Outbox Events<br/>(parcel.assigned, out_for_delivery, delivered, delivery_failed)"]
        M3M5["M3 vs M5: Tracking API Real-Time Projections<br/>(Public milestone masking; No driver PII in public stream)"]
    end
```

### 4.1 M3 vs M9: Database Migration Sequencing (Agreement D3)
- **The Conflict:** Member 3 drafted `000003_dispatch_and_delivery.up.sql` containing foreign keys to `parcels(id)`, `couriers(user_id)`, and `delivery_zones(id)`. If run third, it immediately crashes because `parcels` (Member 2) is created in migration 5.
- **The Resolution:** Database Coordinator Member 9 and the team split Member 3's tables across two canonical migrations:
  1. **`000006_create_couriers.up.sql`**: Creates `couriers` table (references `users` [M1] and `delivery_zones` [M7]).
  2. **`000007_create_dispatch_and_delivery.up.sql`**: Creates `courier_assignments` and `delivery_attempts` (references `couriers` and `parcels` [M2]).

### 4.2 M3 vs M9: Health Probe Dependency Cascading (Agreement A2)
- **The Conflict:** In `doc-08` Section 5.1, Member 3 included a connectivity check to Cloudflare R2 / MinIO in the Core API `/health/ready` probe. If Cloudflare experiences packet loss, Kubernetes terminates Core API pods, taking down authentication, booking, and the entire platform.
- **The Resolution:** Agreement A2 strictly mandates that Core API `/health/ready` checks **PostgreSQL only**. Object storage failures are isolated to proof photo upload endpoints (returning HTTP 503 per-request).

### 4.3 M3 vs M9: Seed Data Separation (Agreement D12)
- **The Conflict:** Member 3 placed seed data inside `core-api/migrations/seeds/000003_dispatch_seed.sql`.
- **The Resolution:** Migrations contain DDL only. Seed data is moved to `deploy/seeds/000006_couriers_seed.sql` and executed via `make seed` or container init scripts.

### 4.4 M3 vs M2: Parcel Aggregate Invariants & Transaction Boundaries
- **The Conflict:** In legacy systems, dispatchers update parcel status with raw SQL (`UPDATE parcels SET current_status = 'DELIVERED'`).
- **The Resolution:** Direct SQL updates are **strictly prohibited**. Member 3's use cases (`AssignCourier`, `CompleteDelivery`, `RecordFailedAttempt`) must:
  1. Load the `Parcel` aggregate root within a PostgreSQL transaction (`SELECT ... FOR UPDATE`).
  2. Invoke domain transition methods (`parcel.TransitionTo(targetStatus)`).
  3. Atomically commit the parcel update, assignment record, attempt record, `parcel_status_history` row, and outbox event in the **exact same transaction**.

### 4.5 M3 vs M1: Identity & Courier Account Scoping
- **The Alignment:** Couriers are created in `users` with `role = 'COURIER'` by Member 1. Member 3 exposes `POST /api/v1/dispatch/couriers` to attach operational metadata (`employee_code`, `vehicle_type`, `max_capacity`). All courier endpoints extract `auth.UserID` from JWT claims to prevent IDOR access to foreign tasks. **Status: 100% Harmonized.**

### 4.6 M3 vs M7: Dispatcher & Courier Frontend Web Integration
- **The Alignment:** Member 3 provides backend REST endpoints for the Dispatch Board (`GET /api/v1/dispatch/board`, `POST /api/v1/dispatch/assignments`), Courier Shift (`POST /api/v1/courier/shift`), Courier Manifest (`GET /api/v1/courier/parcels`), and POD completion (`POST /api/v1/deliveries/{id}/complete`). Member 7 builds the React UI calling these exact routes. **Status: 100% Harmonized.**

### 4.7 M3 vs M8: Transactional Outbox Events
- **The Alignment:** Member 3 writes the following events to `outbox_events` in the same database transaction:
  - `parcel.assigned.v1`: Courier assigned to parcel.
  - `parcel.out_for_delivery.v1`: Final-mile delivery run initiated.
  - `parcel.delivered.v1`: Parcel delivered with verified proof.
  - `parcel.delivery_failed.v1`: Delivery attempt failed (includes attempt count and reason code).
  - `parcel.returned.v1`: Terminal return triggered after 3 failed attempts.

### 4.8 M3 vs M5: Tracking API Real-Time Projections
- **The Alignment:** Member 5 reads `delivery_attempts` and `parcel_status_history` to project real-time milestones. Public tracking endpoints display milestone events (e.g. "Out for Delivery", "Delivery Attempt Failed - Recipient Unavailable"), while strictly masking courier personal contact information and internal dispatcher notes. **Status: 100% Harmonized.**

---

## 5. Systemic Flaws, Engineering Hardening & Binding Standards

### 5.1 Concurrency-Safe Courier Parcel Assignment
To eliminate race conditions when multiple dispatchers assign parcels simultaneously:
```sql
BEGIN;
-- 1. Pessimistically lock parcel and verify eligibility
SELECT id, current_status, version 
FROM parcels 
WHERE id = $1 
FOR UPDATE;

-- Verify: current_status IN ('CREATED', 'ATTEMPT_FAILED')

-- 2. Verify courier shift and active capacity
SELECT operational_status, max_capacity 
FROM couriers 
WHERE user_id = $2;

-- Verify: operational_status = 'ON_SHIFT'

SELECT COUNT(*) 
FROM courier_assignments 
WHERE courier_id = $2 AND status = 'ACTIVE';

-- Verify: active_count + 1 <= max_capacity

-- 3. Execute assignment atomically
INSERT INTO courier_assignments (parcel_id, courier_id, assigned_by_user_id, status)
VALUES ($1, $2, $3, 'ACTIVE');

UPDATE parcels 
SET current_status = 'ASSIGNED', version = version + 1, updated_at = NOW() 
WHERE id = $1;

INSERT INTO parcel_status_history (parcel_id, from_status, to_status, changed_by_user_id, reason)
VALUES ($1, 'CREATED', 'ASSIGNED', $3, 'Assigned to courier');

INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
VALUES ('parcel', $1, 'parcel.assigned.v1', $payload);

COMMIT;
```

### 5.2 Mandatory Proof-of-Delivery Verification Invariant
A parcel cannot transition to `DELIVERED` without satisfying verified proof constraints:
- **`PHOTO`**: `proof_url` must be non-empty and point to a valid cloud storage path.
- **`SIGNATURE`**: `recipient_signature` must be non-empty ($\ge 50$ characters/points) **AND** `received_by_name` must be provided ($\ge 2$ characters).
- **`RECIPIENT_CONFIRMATION`**: `received_by_name` must be provided ($\ge 2$ characters).
If any constraint is violated, the API rejects the request with HTTP `400 Bad Request` (`MISSING_DELIVERY_PROOF`).

### 5.3 ADR-004 Failed Delivery Semantics & 3-Attempt Ceiling
When a delivery attempt fails:
1. Courier submits `failure_reason_code` (`RECIPIENT_UNAVAILABLE`, `CUSTOMER_REQUESTED_RESCHEDULE`, `ACCESS_RESTRICTED`, `INVALID_ADDRESS`, `REFUSED_BY_RECIPIENT`, `DAMAGED_IN_TRANSIT`, `OTHER`).
2. If `reason_code == 'OTHER'`, `notes` must have length $\ge 15$ characters.
3. The system records an immutable record in `delivery_attempts`.
4. `parcels.attempt_count` increments atomically.
5. If `attempt_count >= 3` OR `reason_code == 'REFUSED_BY_RECIPIENT'`:
   - Parcel transitions to terminal `RETURNED`.
   - Active assignment is marked `COMPLETED`.
   - Outbox event `parcel.returned.v1` is emitted.
6. If `attempt_count < 3`:
   - Parcel transitions to recoverable `ATTEMPT_FAILED`.
   - Courier **cannot** unilaterally reattempt.
   - Dispatcher must authorize a reattempt via `POST /api/v1/dispatch/parcels/{id}/reschedule`.

---

## 6. Authoritative Database Migrations

### 6.1 Migration `000006_create_couriers.up.sql`
```sql
CREATE TABLE couriers (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    employee_code VARCHAR(32) NOT NULL UNIQUE, -- e.g. 'CR-0042'
    vehicle_type VARCHAR(32) NOT NULL 
        CHECK (vehicle_type IN ('MOTORCYCLE', 'VAN', 'TRUCK')),
    max_capacity INT NOT NULL DEFAULT 25 
        CHECK (max_capacity BETWEEN 1 AND 200),
    operational_status VARCHAR(32) NOT NULL DEFAULT 'OFF_SHIFT' 
        CHECK (operational_status IN ('OFF_SHIFT', 'ON_SHIFT', 'ON_BREAK', 'SUSPENDED')),
    current_zone_id VARCHAR(32) REFERENCES delivery_zones(id) ON DELETE SET NULL,
    phone_number VARCHAR(32) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_active_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_couriers_operational ON couriers(operational_status, current_zone_id) WHERE is_active = TRUE;
CREATE INDEX idx_couriers_employee_code ON couriers(employee_code);
```

### 6.2 Migration `000007_create_dispatch_and_delivery.up.sql`
```sql
-- 1. Courier Assignments Table
CREATE TABLE courier_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id UUID NOT NULL REFERENCES parcels(id) ON DELETE CASCADE,
    courier_id UUID NOT NULL REFERENCES couriers(user_id) ON DELETE RESTRICT,
    assigned_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' 
        CHECK (status IN ('ACTIVE', 'COMPLETED', 'REASSIGNED', 'CANCELLED')),
    notes TEXT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    unassigned_at TIMESTAMPTZ
);

-- 2. Delivery Attempts Table (Append-Only Event Log)
CREATE TABLE delivery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id UUID NOT NULL REFERENCES parcels(id) ON DELETE CASCADE,
    courier_id UUID NOT NULL REFERENCES couriers(user_id) ON DELETE RESTRICT,
    attempt_number INT NOT NULL CHECK (attempt_number BETWEEN 1 AND 3),
    outcome VARCHAR(32) NOT NULL CHECK (outcome IN ('SUCCESS', 'FAILED')),
    failure_reason_code VARCHAR(64) 
        CHECK (failure_reason_code IS NULL OR failure_reason_code IN (
            'RECIPIENT_UNAVAILABLE', 'CUSTOMER_REQUESTED_RESCHEDULE', 
            'ACCESS_RESTRICTED', 'INVALID_ADDRESS', 'REFUSED_BY_RECIPIENT', 
            'DAMAGED_IN_TRANSIT', 'OTHER'
        )),
    proof_type VARCHAR(32) 
        CHECK (proof_type IS NULL OR proof_type IN ('PHOTO', 'SIGNATURE', 'RECIPIENT_CONFIRMATION')),
    proof_url TEXT,
    recipient_signature TEXT,
    received_by_name VARCHAR(120),
    latitude NUMERIC(10, 8),
    longitude NUMERIC(11, 8),
    notes TEXT,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_assignments_courier ON courier_assignments(courier_id, status);
CREATE INDEX idx_assignments_parcel ON courier_assignments(parcel_id, status);
CREATE INDEX idx_attempts_parcel ON delivery_attempts(parcel_id, attempt_number ASC);
CREATE INDEX idx_attempts_courier ON delivery_attempts(courier_id, attempted_at DESC);
```

---

## 7. Implementation Code Snippets

### 7.1 Pessimistic Courier Assignment Use Case (`internal/dispatch/application/assign.go`)
```go
package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrParcelNotEligible   = errors.New("parcel is not in an assignable state (must be CREATED or ATTEMPT_FAILED)")
	ErrCourierNotAvailable = errors.New("courier is off-shift, suspended, or inactive")
	ErrCapacityExceeded    = errors.New("courier active parcel capacity exceeded")
)

type AssignCourierUseCase struct {
	db *sql.DB
}

func NewAssignCourierUseCase(db *sql.DB) *AssignCourierUseCase {
	return &AssignCourierUseCase{db: db}
}

func (uc *AssignCourierUseCase) Execute(ctx context.Context, parcelID, courierID, dispatcherID uuid.UUID) error {
	tx, err := uc.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	// 1. Lock parcel row FOR UPDATE
	var status string
	var version int
	err = tx.QueryRowContext(ctx,
		"SELECT current_status, version FROM parcels WHERE id = $1 FOR UPDATE",
		parcelID,
	).Scan(&status, &version)
	if err != nil {
		return fmt.Errorf("failed to fetch parcel: %w", err)
	}

	if status != "CREATED" && status != "ATTEMPT_FAILED" {
		return ErrParcelNotEligible
	}

	// 2. Verify courier shift and max capacity
	var shiftStatus string
	var maxCapacity int
	var isActive bool
	err = tx.QueryRowContext(ctx,
		"SELECT operational_status, max_capacity, is_active FROM couriers WHERE user_id = $1",
		courierID,
	).Scan(&shiftStatus, &maxCapacity, &isActive)
	if err != nil {
		return fmt.Errorf("failed to fetch courier: %w", err)
	}

	if !isActive || shiftStatus != "ON_SHIFT" {
		return ErrCourierNotAvailable
	}

	// 3. Count active assignments
	var activeCount int
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM courier_assignments WHERE courier_id = $1 AND status = 'ACTIVE'",
		courierID,
	).Scan(&activeCount)
	if err != nil {
		return fmt.Errorf("failed to count active assignments: %w", err)
	}

	if activeCount+1 > maxCapacity {
		return ErrCapacityExceeded
	}

	// 4. Insert assignment
	_, err = tx.ExecContext(ctx,
		`INSERT INTO courier_assignments (parcel_id, courier_id, assigned_by_user_id, status)
		 VALUES ($1, $2, $3, 'ACTIVE')`,
		parcelID, courierID, dispatcherID,
	)
	if err != nil {
		return fmt.Errorf("failed to create assignment: %w", err)
	}

	// 5. Update parcel status with optimistic version check
	res, err := tx.ExecContext(ctx,
		`UPDATE parcels 
		 SET current_status = 'ASSIGNED', version = version + 1, updated_at = NOW() 
		 WHERE id = $1 AND version = $2`,
		parcelID, version,
	)
	if err != nil {
		return fmt.Errorf("failed to update parcel status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("optimistic lock conflict on parcel")
	}

	// 6. Record status history
	_, err = tx.ExecContext(ctx,
		`INSERT INTO parcel_status_history (parcel_id, from_status, to_status, changed_by_user_id, reason)
		 VALUES ($1, $2, 'ASSIGNED', $3, 'Courier assigned by dispatcher')`,
		parcelID, status, dispatcherID,
	)
	if err != nil {
		return fmt.Errorf("failed to record status history: %w", err)
	}

	// 7. Write outbox event
	_, err = tx.ExecContext(ctx,
		`INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		 VALUES ('parcel', $1, 'parcel.assigned.v1', jsonb_build_object(
			'parcel_id', $1::text,
			'courier_id', $2::text,
			'assigned_by', $3::text
		 ))`,
		parcelID, courierID, dispatcherID,
	)
	if err != nil {
		return fmt.Errorf("failed to write outbox event: %w", err)
	}

	return tx.Commit()
}
```

### 7.2 Delivery Completion with Mandatory Proof Verification (`internal/delivery/application/complete.go`)
```go
package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrUnauthorizedCourier = errors.New("authenticated courier is not assigned to this parcel")
	ErrInvalidProof        = errors.New("delivery completion requires valid proof (photo, signature with recipient name, or confirmation)")
)

type CompleteDeliveryCommand struct {
	ParcelID           uuid.UUID
	CourierID          uuid.UUID
	ProofType          string // "PHOTO", "SIGNATURE", "RECIPIENT_CONFIRMATION"
	ProofURL           string
	RecipientSignature string
	ReceivedByName     string
	Latitude           *float64
	Longitude          *float64
}

type CompleteDeliveryUseCase struct {
	db *sql.DB
}

func NewCompleteDeliveryUseCase(db *sql.DB) *CompleteDeliveryUseCase {
	return &CompleteDeliveryUseCase{db: db}
}

func (uc *CompleteDeliveryUseCase) Execute(ctx context.Context, cmd CompleteDeliveryCommand) error {
	// 1. Validate proof invariant
	switch cmd.ProofType {
	case "PHOTO":
		if strings.TrimSpace(cmd.ProofURL) == "" {
			return fmt.Errorf("%w: photo URL is required", ErrInvalidProof)
		}
	case "SIGNATURE":
		if len(cmd.RecipientSignature) < 50 || len(strings.TrimSpace(cmd.ReceivedByName)) < 2 {
			return fmt.Errorf("%w: signature data and recipient name (min 2 chars) are required", ErrInvalidProof)
		}
	case "RECIPIENT_CONFIRMATION":
		if len(strings.TrimSpace(cmd.ReceivedByName)) < 2 {
			return fmt.Errorf("%w: recipient name (min 2 chars) is required", ErrInvalidProof)
		}
	default:
		return fmt.Errorf("%w: unsupported proof type %s", ErrInvalidProof, cmd.ProofType)
	}

	tx, err := uc.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	// 2. Verify active assignment to this courier
	var assignmentID uuid.UUID
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM courier_assignments 
		 WHERE parcel_id = $1 AND courier_id = $2 AND status = 'ACTIVE' 
		 FOR UPDATE`,
		cmd.ParcelID, cmd.CourierID,
	).Scan(&assignmentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUnauthorizedCourier
		}
		return fmt.Errorf("failed to verify assignment: %w", err)
	}

	// 3. Close assignment
	_, err = tx.ExecContext(ctx,
		"UPDATE courier_assignments SET status = 'COMPLETED', unassigned_at = NOW() WHERE id = $1",
		assignmentID,
	)
	if err != nil {
		return fmt.Errorf("failed to complete assignment: %w", err)
	}

	// 4. Insert delivery attempt log
	var attemptNumber int
	err = tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(attempt_number), 0) + 1 FROM delivery_attempts WHERE parcel_id = $1",
		cmd.ParcelID,
	).Scan(&attemptNumber)
	if err != nil {
		return fmt.Errorf("failed to calculate attempt number: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO delivery_attempts (
			parcel_id, courier_id, attempt_number, outcome, proof_type, 
			proof_url, recipient_signature, received_by_name, latitude, longitude
		 ) VALUES ($1, $2, $3, 'SUCCESS', $4, $5, $6, $7, $8, $9)`,
		cmd.ParcelID, cmd.CourierID, attemptNumber, cmd.ProofType,
		cmd.ProofURL, cmd.RecipientSignature, cmd.ReceivedByName, cmd.Latitude, cmd.Longitude,
	)
	if err != nil {
		return fmt.Errorf("failed to record attempt: %w", err)
	}

	// 5. Update parcel status to DELIVERED
	var currentStatus string
	var version int
	err = tx.QueryRowContext(ctx,
		"SELECT current_status, version FROM parcels WHERE id = $1 FOR UPDATE",
		cmd.ParcelID,
	).Scan(&currentStatus, &version)
	if err != nil {
		return fmt.Errorf("failed to fetch parcel: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE parcels 
		 SET current_status = 'DELIVERED', version = version + 1, updated_at = NOW() 
		 WHERE id = $1 AND version = $2`,
		cmd.ParcelID, version,
	)
	if err != nil {
		return fmt.Errorf("failed to update parcel: %w", err)
	}

	// 6. Record status history
	_, err = tx.ExecContext(ctx,
		`INSERT INTO parcel_status_history (parcel_id, from_status, to_status, changed_by_user_id, reason)
		 VALUES ($1, $2, 'DELIVERED', $3, 'Delivery confirmed with proof')`,
		cmd.ParcelID, currentStatus, cmd.CourierID,
	)
	if err != nil {
		return fmt.Errorf("failed to record status history: %w", err)
	}

	// 7. Write outbox event
	_, err = tx.ExecContext(ctx,
		`INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		 VALUES ('parcel', $1, 'parcel.delivered.v1', jsonb_build_object(
			'parcel_id', $1::text,
			'courier_id', $2::text,
			'received_by', $3,
			'proof_type', $4
		 ))`,
		cmd.ParcelID, cmd.CourierID, cmd.ReceivedByName, cmd.ProofType,
	)
	if err != nil {
		return fmt.Errorf("failed to write outbox event: %w", err)
	}

	return tx.Commit()
}
```

---

## 8. Verification & Action Checklist for Member 3

Before Team Meeting 2 sign-off, Member 3 must verify the following items:

- [x] **Audit Completed:** Full 8-document suite and architectural questionnaire cross-checked against system requirements.
- [x] **Migration Sequencing Split:** Tables properly separated into `000006_create_couriers.up.sql` and `000007_create_dispatch_and_delivery.up.sql`.
- [x] **Readiness Probe Decoupled:** Cloudflare R2 / MinIO connectivity check removed from `/health/ready` (PostgreSQL only checked).
- [x] **Zone Key Harmonized:** Column standardized as `current_zone_id VARCHAR(32) REFERENCES delivery_zones(id)`.
- [x] **Assignment Status Synchronized:** `status VARCHAR(32) CHECK (status IN ('ACTIVE', 'COMPLETED', 'REASSIGNED', 'CANCELLED'))` aligned with capacity queries.
- [x] **Pessimistic Locking Implemented:** `SELECT ... FOR UPDATE` row locking prevents concurrent duplicate assignment races.
- [x] **Proof-of-Delivery Invariant Bound:** Mandatory proof data enforced before terminal `DELIVERED` status transition.
- [x] **ADR-004 Reattempt Rules Enforced:** Maximum 3 attempts ceiling and dispatcher authorization gate implemented.
- [x] **Outbox Events Aligned:** Single-transaction commit emits `parcel.assigned.v1`, `parcel.out_for_delivery.v1`, `parcel.delivered.v1`, `parcel.delivery_failed.v1`, and `parcel.returned.v1`.

---
*End of Member 03 Comprehensive Audit, Review & Alignment Guide.*
