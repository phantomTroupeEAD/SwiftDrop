# SwiftDrop Parcel Delivery & Courier Management System

## Document 6 — Testing Strategy & Acceptance Specification

> **Module:** Member 2 — Parcel Management & Pricing (`core-api/internal/parcels/`, `core-api/internal/pricing/`)  
> **Status:** Aligned Testing Specification (Post-Audit v2.0.0 Alignment)  
> **Date:** 25 October 2026  
> **Order of Precedence:** SwiftDrop SRS v1.0 > Approved Architecture & Contracts > Engineering Standards v1.0 > AI Rulebook (`AGENTS.md`) > Member 2 Specifications

---

## 1. Purpose and Scope

This document defines the comprehensive testing strategy, quality assurance gates, requirement traceability, test case specifications, and acceptance criteria for Member 2 (**Parcel Management & Pricing**) in the SwiftDrop Core API.

### 1.1 In-Scope Test Boundaries
- **Deterministic Pricing & Quote Verification:** Volumetric calculations ($L \times W \times H / 5000$), chargeable weight, billable extra weight ceilings, service tier multipliers, and 15-minute quote expiration enforcement.
- **Parcel Booking & Aggregate Invariants:** Quoted booking validation, direct booking tariff evaluation, tracking number generation (`SD-YYYYMMDD-XXXXXX`), address snapshot persistence in JSONB, and optimistic concurrency versioning.
- **Idempotency Protection:** Replay verification of cached 201 responses, payload alteration conflict detection (409 Conflict), and 24-hour retention window verification.
- **Authoritative 9-State Lifecycle (ADR-004):** State transition rules, actor permission checks, delivery attempt ceiling (max 3), transition to `ATTEMPT_FAILED` vs terminal `RETURNED`, and optimistic locking conflicts.
- **Transactional Outbox Atomicity:** Verifying atomic persistence of parcel state, status history, and outbox integration events (`parcel.booked.v1`, `parcel.cancelled.v1`).
- **Data Privacy & Authorization:** Cross-customer/cross-tenant access denial (IDOR prevention) and redaction of PII from public tracking projections.
- **Database Migrations:** Clean schema creation and rollback verification for migrations `000004`, `000005`, and `000010`.

---

## 2. Core Testing Principles

1. **Deterministic Financial Verification:** Monetary amounts are asserted strictly in integer minor units (LKR cents). Floating-point comparisons are prohibited.
2. **Real Persistence for Invariants:** Database constraints, transaction rollbacks, uniqueness checks, and outbox atomicity are verified against a real PostgreSQL instance (via Docker Compose or Testcontainers), never via unit mocks.
3. **Fail-Fast Boundary Testing:** Input parameters (weights $> 50$ kg, dimensions $> 150$ cm, negative values) are verified to fail early with structured 400/422 errors before reaching persistence layers.
4. **Zero-PII Disclosure Checks:** Public tracking tests assert that recipient phone numbers, complete street addresses, and internal notes are strictly omitted from responses.
5. **No Synthetic Pass Claims:** Test execution reports must reflect actual execution output and exit codes.

---

## 3. Test Levels and Execution Framework

```mermaid
flowchart TD
    subgraph UnitTests["1. Pure Unit Tests (In-Memory)"]
        UT1["Pricing Formula & Volumetric Engine"]
        UT2["Aggregate Lifecycle State Transitions"]
        UT3["Input Validation & Crockford Tracking Generator"]
    end

    subgraph IntegrationTests["2. Persistence & API Integration (Real PostgreSQL)"]
        IT1["PostgreSQL Migrations 000004, 000005, 000010"]
        IT2["Idempotency SHA-256 Hashing & Replay"]
        IT3["Atomic Booking Tx (Parcel + History + Outbox)"]
        IT4["Optimistic Locking Version Conflict (409)"]
    end

    subgraph SecurityContractTests["3. Security & Contract Tests"]
        ST1["Tenant & Customer Scoped Isolation (IDOR)"]
        ST2["Public Tracking Privacy Field Redaction"]
        ST3["OpenAPI Schema Contract Compliance"]
    end

    UnitTests --> IntegrationTests --> SecurityContractTests
```

| Test Level | Scope | Primary Tools & Drivers | Execution Target |
| :--- | :--- | :--- | :--- |
| **Unit** | Pure domain logic, pricing formulas, state machine, input validators. | Go standard `testing`, `testify/assert` | Fast ($< 2$s), zero external dependencies. |
| **Database Integration** | SQL repositories, transactions, migrations, concurrency locks, outbox inserts. | Go `testing`, `pgxpool`, Docker PostgreSQL | Local Docker Compose test DB. |
| **API Integration** | HTTP handlers, middleware, request parsing, response status codes, error shapes. | `net/http/httptest`, `chi.Router` | In-process HTTP server against test DB. |
| **Security & Privacy** | Multi-tenant scoping, RBAC role enforcement, PII masking in tracking views. | Integration test harness with mock JWTs | Verifies 403 Forbidden and redacted DTOs. |
| **Migration Quality** | Sequential migration execution (`up`) and clean reversal (`down`). | `golang-migrate/migrate/v4` | Clean ephemeral database. |

---

## 4. Requirements Traceability Matrix

| Requirement ID | Domain Area | Test Case IDs | Test Level | Verification Objective |
| :--- | :--- | :--- | :--- | :--- |
| **FR-PAR-001** | Booking Details | `TC-BKG-001`, `TC-BKG-002` | API / DB Integration | Captures dimensions, weights, and JSONB addresses. |
| **FR-PAR-002** | Pricing Quote | `TC-PRC-001`, `TC-PRC-002` | Unit / API Integration | Computes volumetric fee; guarantees 15m price freeze. |
| **FR-PAR-003** | Accepted Price | `TC-PRC-005`, `TC-BKG-007` | Database Integration | Persists immutable rule version and LKR cents snapshot. |
| **FR-PAR-004** | Tracking Number | `TC-TRK-001`, `TC-TRK-002` | Unit / DB Integration | Crockford Base32 format `SD-YYYYMMDD-XXXXXX`; unique. |
| **FR-PAR-005** | Input Validation | `TC-VAL-001`, `TC-VAL-002` | Unit / API Integration | Rejects weights $> 50$ kg, dimensions $> 150$ cm (422). |
| **FR-PAR-006** | Idempotency | `TC-IDM-001`, `TC-IDM-002` | API / DB Integration | Key replay returns cached 201; payload mismatch returns 409. |
| **FR-PAR-007** | Caller Scoping | `TC-SEC-001`, `TC-SEC-002` | Security Integration | Prevents IDOR cross-tenant access to foreign parcels. |
| **FR-PAR-008** | Booking Receipt | `TC-READ-003` | API Integration | Receipt reflects stored price snapshot, not current tariff. |
| **FR-PAR-009** | Public Privacy | `TC-SEC-003` | Security Integration | Public tracking redacts recipient phone, street address, notes. |
| **FR-LIFE-001-004**| Lifecycle Machine | `TC-LCY-001` to `TC-LCY-006`| Unit / DB Integration | Enforces 9 states, max 3 attempts, atomic outbox commit. |

---

## 5. Detailed Test Case Specifications

### 5.1 Deterministic Pricing & Quote Management (`TC-PRC-*`)

#### `TC-PRC-001`: Standard Tariff Calculation & Volumetric Weight
- **Objective:** Verify deterministic fee calculation using $(L \times W \times H) / 5000$. *(Provisional formula proposal pending team confirmation)*.
- **Inputs:** Dimensions $30 \times 25 \times 15$ cm, actual weight $2.00$ kg, service level `STANDARD`.
- **Calculations (Illustrative Baseline):**
  - Volumetric weight = $(30 \times 25 \times 15) / 5000 = 2.25$ kg.
  - Chargeable weight = $\max(2.00, 2.25) = 2.25$ kg.
  - Billable extra weight = $\lceil \max(0, 2.25 - 1.0) \rceil = 2.0$ kg.
  - Base fee = 35,000 cents (LKR 350.00), per-kg fee = 10,000 cents (LKR 100.00).
  - Total fee = $(35000 + 2 \times 10000) \times 1.0 = 55,000$ cents (LKR 550.00).
- **Assertions:** Returns HTTP 201; `fee_cents == 55000`, `currency == "LKR"`, `expires_at == created_at + 15m`.

> [!NOTE]
> **Provisional Pricing Test Fixture Notice:**
> Test cases `TC-PRC-001`, `TC-PRC-002`, and `TC-PRC-003` assert calculations against provisional pricing proposals (divisor 5000, 1.5x/2.0x multipliers, 15m TTL). If the team adopts alternative tariff structures or formulas, the testing harness and invariant assertions remain valid while the specific mathematical expected values will be updated to match the ratified policy.

#### `TC-PRC-002`: Service Tier Multipliers
- **Objective:** Verify `EXPRESS` (1.5x) and `SAME_DAY` (2.0x) multipliers.
- **Inputs:** Identical parcel as `TC-PRC-001` evaluated under `EXPRESS` and `SAME_DAY`.
- **Assertions:**
  - `EXPRESS`: $55000 \times 1.5 = 82,500$ cents (HTTP 201).
  - `SAME_DAY`: $55000 \times 2.0 = 110,000$ cents (HTTP 201).

#### `TC-PRC-003`: Quote Expiration Enforcement
- **Objective:** Attempt to book a parcel with an expired quote.
- **Setup:** Create a quote with `expires_at = NOW() - INTERVAL '1 minute'`.
- **Action:** Submit `POST /api/v1/parcels` referencing the expired `quote_id`.
- **Assertions:** Request rejected with HTTP 422 Unprocessable Entity (`code: "QUOTE_EXPIRED"`). No parcel created.

#### `TC-PRC-004`: Quote Parameter Mismatch
- **Objective:** Prevent price tampering by altering parcel weight between quote and booking.
- **Setup:** Generate quote for 2.0 kg. Submit booking referencing quote with weight 5.0 kg.
- **Assertions:** Request rejected with HTTP 422 Unprocessable Entity (`code: "QUOTE_PARAM_MISMATCH"`).

---

### 5.2 Parcel Booking & Input Validation (`TC-BKG-*`, `TC-VAL-*`)

#### `TC-VAL-001`: Maximum Weight Boundary Enforcement
- **Objective:** Reject bookings exceeding operational limit of 50.00 kg.
- **Inputs:** Weight = 50.01 kg.
- **Assertions:** HTTP 422 Unprocessable Entity (`code: "WEIGHT_LIMIT_EXCEEDED"`).

#### `TC-VAL-002`: Maximum Dimension Boundary Enforcement
- **Objective:** Reject parcels exceeding maximum single dimension of 150.00 cm.
- **Inputs:** Length = 150.50 cm.
- **Assertions:** HTTP 422 Unprocessable Entity (`code: "DIMENSION_LIMIT_EXCEEDED"`).

#### `TC-BKG-001`: Successful Quoted Customer Booking
- **Objective:** Verify end-to-end booking with valid quote, address snapshots, and tracking number.
- **Action:** `POST /api/v1/parcels` with valid `Idempotency-Key` and unexpired `quote_id`.
- **Assertions:**
  - HTTP 201 Created with `Location: /api/v1/parcels/{id}`.
  - Tracking number matches regex `^SD-[0-9]{8}-[0-9A-Z]{6}$`.
  - `parcels.pickup_address` and `parcels.dropoff_address` stored as JSONB.
  - `parcel_status_history` contains initial row (`from_status: null`, `to_status: "CREATED"`).
  - `outbox_events` contains `parcel.booked.v1` event with matching payload.

---

### 5.3 Idempotency & Concurrency (`TC-IDM-*`)

#### `TC-IDM-001`: Exact Request Replay
- **Objective:** Verify that identical repeated booking requests return the original response without duplicating records.
- **Action:** Send identical `POST /api/v1/parcels` payload with identical `Idempotency-Key` header twice.
- **Assertions:**
  - First request: HTTP 201 Created (new parcel record inserted).
  - Second request: HTTP 201 Created (cached payload returned).
  - Exactly 1 row exists in `parcels` table.

#### `TC-IDM-002`: Key Reuse with Payload Conflict
- **Objective:** Detect and reject payload tampering under a reused idempotency key.
- **Action:** Send initial booking with key `KEY-ABC`. Send second booking with same key `KEY-ABC` but different destination address.
- **Assertions:** Second request returns HTTP 409 Conflict (`code: "IDEMPOTENCY_KEY_PAYLOAD_MISMATCH"`). No second parcel inserted.

#### `TC-IDM-003`: Optimistic Concurrency Control (OCC)
- **Objective:** Prevent lost updates when two actors concurrently modify a parcel.
- **Setup:** Load parcel at `version = 1`.
- **Action:** Actor A updates parcel status to `ASSIGNED` (`version` becomes 2). Actor B attempts to update parcel status using stale `version = 1`.
- **Assertions:** Actor B's update affects 0 rows and fails with HTTP 409 Conflict (`code: "VERSION_CONFLICT"`).

---

### 5.4 Lifecycle State Machine & ADR-004 Delivery Semantics (`TC-LCY-*`)

#### `TC-LCY-001`: Canonical Transition Sequence
- **Objective:** Transition parcel through happy path: `CREATED` $\to$ `ASSIGNED` $\to$ `PICKED_UP` $\to$ `IN_TRANSIT` $\to$ `OUT_FOR_DELIVERY` $\to$ `DELIVERED`.
- **Assertions:** Every transition succeeds, increments `version`, appends to `parcel_status_history`, and writes to `outbox_events`.

#### `TC-LCY-002`: Illegal State Transition Rejection
- **Objective:** Reject invalid lifecycle shortcuts (e.g., `CREATED` $\to$ `DELIVERED`).
- **Assertions:** HTTP 409 Conflict (`code: "INVALID_STATE_TRANSITION"`). Database state remains unchanged.

#### `TC-LCY-003`: Failed Delivery Attempt Increment (ADR-004)
- **Objective:** Verify transient failed delivery attempt semantics.
- **Setup:** Parcel in `OUT_FOR_DELIVERY` (`attempt_count = 0`).
- **Action:** Courier reports delivery failure (e.g., recipient unavailable).
- **Assertions:**
  - Status transitions to `ATTEMPT_FAILED`.
  - `attempt_count` increments to 1.
  - Parcel is NOT in terminal state; can be rescheduled to `OUT_FOR_DELIVERY`.

#### `TC-LCY-004`: Maximum Delivery Attempts Exceeded (ADR-004)
- **Objective:** Verify terminal return when delivery attempts reach 3.
- **Setup:** Parcel in `ATTEMPT_FAILED` with `attempt_count = 3`.
- **Action:** Dispatcher/system triggers return process.
- **Assertions:** Status transitions to terminal `RETURNED`. No further delivery reattempts allowed.

---

### 5.5 Security, Authorization & Privacy (`TC-SEC-*`)

#### `TC-SEC-001`: Tenant & Customer Isolation (IDOR Prevention)
- **Objective:** Prevent Customer A from viewing or modifying Customer B's parcels.
- **Action:** Customer A sends `GET /api/v1/parcels/{id_of_customer_b}`.
- **Assertions:** HTTP 403 Forbidden (or 404 Not Found to prevent resource enumeration).

#### `TC-SEC-002`: Public Tracking PII Redaction
- **Objective:** Ensure public tracking endpoint redacts all recipient PII.
- **Action:** Query public tracking projection for an existing parcel.
- **Assertions:**
  - `status`, `tracking_number`, `pickup_city`, `dropoff_city` are present.
  - `recipient_phone`, `recipient_name`, `address_line1`, `price_cents`, and `special_instructions` are completely absent.

---

### 5.6 Database Migrations (`TC-DB-*`)

#### `TC-DB-001`: Full Forward Migration Execution
- **Objective:** Apply migrations `000004`, `000005`, and `000010` on clean PostgreSQL.
- **Assertions:** All tables, indexes, checks, and foreign keys created successfully.

#### `TC-DB-002`: Clean Migration Rollback
- **Objective:** Reverse migrations `000010`, `000005`, and `000004`.
- **Assertions:** All tables dropped cleanly; database returns to pre-migration state.

---

## 6. Local Test Execution Instructions

### 6.1 Unit Test Execution
Execute all pure domain and pricing calculation unit tests:
```powershell
go test -v -race -cover ./internal/pricing/... ./internal/parcels/...
```

### 6.2 Database Integration Test Execution
Spin up the local PostgreSQL container and execute repository integration tests:
```powershell
docker compose up -d postgres
$env:TEST_DATABASE_URL="postgres://swiftdrop:swiftdrop_dev_password@localhost:5432/swiftdrop_test?sslmode=disable"
go test -v -tags=integration ./internal/parcels/infrastructure/...
```

---

## 7. Alignment & Decision Tracking Matrix

| Testing Topic | Initial Draft State | Aligned Final Specification | Source / Status |
| :--- | :--- | :--- | :--- |
| **Monetary Validation** | Allowed float fee checks | Exact integer LKR cents comparison | Engineering Standards v1.0 Section 6 |
| **Pricing Formula Test** | General estimation tests | Volumetric ($/ 5000$), billable extra weight, tier multipliers | **PROVISIONAL PROPOSAL (PENDING TEAM CONFIRMATION)** |
| **Tariffs & Multipliers Test** | Undefined test rates | Multipliers (1.0x, 1.5x, 2.0x) and illustrative rates | **PROVISIONAL PROPOSAL (PENDING TEAM CONFIRMATION)** |
| **Quote Expiry Test** | Undefined quote TTL | 15-minute expiration test (`TC-PRC-003`) | **PROVISIONAL PROPOSAL (PENDING TEAM CONFIRMATION)** |
| **Address Verification** | Tested `parcel_addresses` child table | Tests JSONB snapshots on `parcels` | Database Schema Standards |
| **Failure Semantics** | Tested terminal `FAILED` | Tested transient `ATTEMPT_FAILED` (max 3) vs `RETURNED` | ADR-004 Failed Delivery Semantics |
| **Migration Testing** | Unnumbered migrations | Explicit testing of `000004`, `000005`, `000010` | Database Migration Baseline |
| **Tariff Administration** | Mixed ownership tests | Admin writes owned by M7/M1; M2 tests calculation | `PENDING TEAM CONFIRMATION` |
| **Receipt File Format** | Tested arbitrary PDF output | M2 tests JSON snapshot; M6 tests rendering | `PENDING TEAM CONFIRMATION` |

### 7.1 Pricing Policy Dependency & Test Suite Impact Analysis

| Test Suite / Area | Invariant Framework (Unaffected by Pricing Changes) | Impact If Pricing Policy Changes |
| :--- | :--- | :--- |
| **Pricing Unit Tests (`internal/pricing/...`)** | Unit test framework, deterministic calculation assertion, zero-float validation. | Mathematical expected values in `TC-PRC-001` and `TC-PRC-002` will be updated to match the newly ratified formula/rates. |
| **Quote Expiration Tests** | Test verifying expired quotes are rejected during booking with HTTP 422 (`QUOTE_EXPIRED`). | If the 15-minute TTL changes, the synthetic timestamp delta in `TC-PRC-003` will adjust to match the ratified duration. |
| **Booking & Idempotency Integration** | Booking transaction atomicity, `Idempotency-Key` replay, payload mismatch conflict (409), address snapshots. | **Zero Impact:** Booking workflows test the accepted price snapshot irrespective of how the fee was calculated. |
| **Lifecycle & Concurrency Tests** | 9-state machine transitions, ADR-004 attempt ceiling (max 3), optimistic locking (`version INT`). | **Zero Impact:** Parcel state machine is decoupled from pricing formulas. |

---

## 8. Acceptance Checklist & Quality Gates

- [ ] Core package code coverage reaches or exceeds 70% line coverage.
- [ ] 100% of defined invariant tests (`TC-PRC-*`, `TC-BKG-*`, `TC-IDM-*`, `TC-LCY-*`) pass without flake.
- [ ] Zero binary floating-point assertions exist in monetary test suites.
- [ ] Transaction rollbacks tested: simulated failure during outbox insert confirms zero partial commits.
- [ ] Concurrency verified: simultaneous duplicate requests under same idempotency key create exactly 1 parcel.
- [ ] Public tracking privacy tests confirm complete redaction of customer phone and street address.
- [ ] Clean migration up and down tests succeed against real PostgreSQL.
