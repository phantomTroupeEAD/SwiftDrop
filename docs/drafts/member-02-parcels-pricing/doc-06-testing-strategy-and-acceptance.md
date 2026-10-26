# SwiftDrop Parcel Delivery & Courier Management System

## Document 6 â€” Testing Strategy & Acceptance Specification

> **Module:** Member 2 â€” Parcel Management & Pricing (`core-api/internal/parcels/`, `core-api/internal/pricing/`)
> **Status:** Aligned Testing Specification (Post-Audit v2.0.0 Alignment)
> **Date:** 25 October 2026
> **Order of Precedence:** SwiftDrop SRS v1.0 > Approved Architecture & Contracts > Engineering Standards v1.0 > AI Rulebook (`AGENTS.md`) > Member 2 Specifications

---

## 1. Purpose and Scope

This document defines the comprehensive testing strategy, quality assurance gates, requirement traceability, test case specifications, and acceptance criteria for Member 2 (**Parcel Management & Pricing**) in the SwiftDrop Core API.

### 1.1 In-Scope Test Boundaries
- **Deterministic Pricing & Complete Tariff Verification:** Complete tariff lookup per `(pickup_zone_id, destination_zone_id, weight_band_id)`, Option A volumetric threshold (actual weight $\le 1\text{ kg}$ requires no dimensions; $> 1\text{ kg}$ requires dimensions with volumetric divisor $6000\text{ cm}^3/\text{kg}$), 7 non-overlapping weight bands up to 30 kg, and 15-minute quote expiration enforcement.
- **Parcel Booking & Aggregate Invariants:** Quoted booking validation, direct booking tariff evaluation, tracking number generation (`SD-YYYYMMDD-XXXXXX`), address snapshot persistence in JSONB, immutable price snapshot, and optimistic concurrency versioning.
- **Idempotency Protection:** Replay verification of cached 201 responses, payload alteration conflict detection (409 Conflict), and 24-hour retention window verification.
- **Authoritative 9-State Lifecycle (ADR-004):** State transition rules, actor permission checks, delivery attempt ceiling (max 3), transition to `ATTEMPT_FAILED` vs terminal `RETURNED`, and optimistic locking conflicts.
- **Transactional Outbox Atomicity:** Verifying atomic persistence of parcel state, status history, and outbox integration events (`parcel.booked.v1`, `parcel.cancelled.v1`).
- **Data Privacy & Authorization:** Cross-customer/cross-tenant access denial (IDOR prevention) and redaction of PII from public tracking projections.
- **Database Migrations:** Clean schema creation and rollback verification for migrations `000004`, `000005`, and `000010`.

---

## 2. Core Testing Principles

1. **Deterministic Financial Verification:** Monetary amounts are asserted strictly in integer minor units (LKR cents). Floating-point comparisons are prohibited.
2. **Real Persistence for Invariants:** Database constraints, transaction rollbacks, uniqueness checks, and outbox atomicity are verified against a real PostgreSQL instance (via Docker Compose or Testcontainers), never via unit mocks.
3. **Fail-Fast Boundary Testing:** Input parameters (weights $> 30$ kg, dimensions $> 150$ cm, negative values) are verified to fail early with structured 400/422 errors before reaching persistence layers.
4. **Zero-PII Disclosure Checks:** Public tracking tests assert that recipient phone numbers, complete street addresses, and internal notes are strictly omitted from responses.
5. **No Synthetic Pass Claims:** Test execution reports must reflect actual execution output and exit codes. Test cases document expected verification suites, distinguishing documented specifications from code executed.

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
| **FR-PAR-002** | Pricing Quote | `TC-PRC-001` to `TC-PRC-011`| Unit / API Integration | Computes complete tariff, Option A threshold, divisor 6000, 7 bands, 15m price freeze. |
| **FR-PAR-003** | Accepted Price | `TC-PRC-010`, `TC-BKG-007` | Database Integration | Persists immutable rule version and LKR cents snapshot. |
| **FR-PAR-004** | Tracking Number | `TC-TRK-001`, `TC-TRK-002` | Unit / DB Integration | Crockford Base32 format `SD-YYYYMMDD-XXXXXX`; unique. |
| **FR-PAR-005** | Input Validation | `TC-VAL-001`, `TC-VAL-002` | Unit / API Integration | Rejects weights $> 30$ kg, dimensions $> 150$ cm (422). |
| **FR-PAR-006** | Idempotency | `TC-IDM-001`, `TC-IDM-002` | API / DB Integration | Key replay returns cached 201; payload mismatch returns 409. |
| **FR-PAR-007** | Caller Scoping | `TC-SEC-001`, `TC-SEC-002` | Security Integration | Prevents IDOR cross-tenant access to foreign parcels. |
| **FR-PAR-008** | Booking Receipt | `TC-READ-003` | API Integration | Receipt reflects stored price snapshot, not current tariff. |
| **FR-PAR-009** | Public Privacy | `TC-SEC-003` | Security Integration | Public tracking redacts recipient phone, street address, notes. |
| **FR-LIFE-001-004**| Lifecycle Machine | `TC-LCY-001` to `TC-LCY-006`| Unit / DB Integration | Enforces 9 states, max 3 attempts, atomic outbox commit. |

---

## 5. Detailed Test Case Specifications

### 5.1 Deterministic Pricing & Quote Management (`TC-PRC-*`)

> [!NOTE]
> **Provisional Pricing Specification Notice:**
> The following test cases verify the proposed Member 2 pricing baseline pending team approval: fixed complete tariff per zone pair and weight band, 7 weight bands up to 30 kg, Option A threshold (dims required only when actual weight > 1 kg), volumetric divisor 6000 cmÂ³/kg, and 15-minute quote validity window.

#### `TC-PRC-001`: Option A Threshold & Bulky Parcel ($\le 1\text{ kg}$)
- **Objective:** Verify that parcels with actual weight $\le 1\text{ kg}$ use actual weight only; dimensions do not affect price.
- **Inputs:** Actual weight = 0.8 kg, dimensions = $40 \times 30 \times 28$ cm, pickup = `ZONE-WESTERN-01`, destination = `ZONE-CENTRAL-01`.
- **Calculations:**
  - Actual weight $\le 1.0\text{ kg}$ $\implies$ `chargeable_weight` = 0.8 kg.
  - Volumetric calculation bypassed for pricing determination.
  - Applicable band: Band 2 ($0.5 < W \le 1.0\text{ kg}$).
- **Assertions:** Returns HTTP 201; `chargeable_weight_kg == 0.80`, `weight_band_id == 2`, `fee_cents` equals configured Band 2 tariff, `expires_at == created_at + 15m`.

#### `TC-PRC-002`: Exact Threshold Boundary at 1.0 kg vs 1.001 kg
- **Objective:** Verify strict transition at the 1 kg threshold boundary.
- **Setup & Cases:**
  - **Case A (Exactly 1.00 kg):** Dimensions omitted or provided; `chargeable_weight` = 1.00 kg $\implies$ Band 2 ($0.5 < W \le 1.0\text{ kg}$).
  - **Case B (1.001 kg, Dims Missing):** Dimensions omitted $\implies$ HTTP 422 Unprocessable Entity (`code: "DIMENSIONS_REQUIRED_ABOVE_1KG"`).
  - **Case C (1.001 kg, Dims Provided):** Dims $20 \times 15 \times 10$ cm $\implies$ Volumetric = $(20 \times 15 \times 10) / 6000 = 0.50\text{ kg}$. Chargeable = $\max(1.001, 0.50) = 1.001\text{ kg} \implies$ Band 3 ($1.0 < W \le 2.0\text{ kg}$).
- **Assertions:** Case A succeeds (Band 2); Case B rejected (422); Case C succeeds (Band 3).

#### `TC-PRC-003`: Volumetric Divisor 6000 & Volumetric Exceeding Actual Weight
- **Objective:** Verify divisor 6000 and selection of volumetric weight when greater than actual weight.
- **Inputs:** Actual weight = 2.0 kg, dimensions = $40 \times 30 \times 28$ cm.
- **Calculations:**
  - Volumetric weight = $(40 \times 30 \times 28) / 6000 = 33600 / 6000 = 5.60\text{ kg}$.
  - Chargeable weight = $\max(2.0, 5.60) = 5.60\text{ kg}$.
  - Applicable band: Band 5 ($5.0 < W \le 10.0\text{ kg}$).
- **Assertions:** Returns HTTP 201; `chargeable_weight_kg == 5.60`, `weight_band_id == 5`, fee matches Band 5 tariff.

#### `TC-PRC-004`: Actual Weight Exceeding Volumetric Weight
- **Objective:** Verify selection of actual weight when actual weight exceeds volumetric weight.
- **Inputs:** Actual weight = 4.0 kg, dimensions = $20 \times 20 \times 15$ cm.
- **Calculations:**
  - Volumetric weight = $(20 \times 20 \times 15) / 6000 = 6000 / 6000 = 1.00\text{ kg}$.
  - Chargeable weight = $\max(4.0, 1.00) = 4.00\text{ kg}$.
  - Applicable band: Band 4 ($2.0 < W \le 5.0\text{ kg}$).
- **Assertions:** Returns HTTP 201; `chargeable_weight_kg == 4.00`, `weight_band_id == 4`.

#### `TC-PRC-005`: All Seven Weight Bands & Boundary Invariants
- **Objective:** Verify non-overlapping coverage of all 7 bands at exact boundary points.
- **Test Matrix:**
  - $W = 0.50\text{ kg} \implies$ Band 1 ($0 < W \le 0.5\text{ kg}$).
  - $W = 0.501\text{ kg} \implies$ Band 2 ($0.5 < W \le 1.0\text{ kg}$).
  - $W = 1.00\text{ kg} \implies$ Band 2.
  - $W = 1.001\text{ kg} \implies$ Band 3 ($1.0 < W \le 2.0\text{ kg}$).
  - $W = 2.00\text{ kg} \implies$ Band 3.
  - $W = 2.001\text{ kg} \implies$ Band 4 ($2.0 < W \le 5.0\text{ kg}$).
  - $W = 5.00\text{ kg} \implies$ Band 4.
  - $W = 5.001\text{ kg} \implies$ Band 5 ($5.0 < W \le 10.0\text{ kg}$).
  - $W = 10.00\text{ kg} \implies$ Band 5.
  - $W = 10.001\text{ kg} \implies$ Band 6 ($10.0 < W \le 20.0\text{ kg}$).
  - $W = 20.00\text{ kg} \implies$ Band 6.
  - $W = 20.001\text{ kg} \implies$ Band 7 ($20.0 < W \le 30.0\text{ kg}$).
  - $W = 30.00\text{ kg} \implies$ Band 7.
- **Assertions:** Every test weight matches exactly one band; zero ambiguity or missing coverage.

#### `TC-PRC-006`: No Premature Rounding Before Band Selection
- **Objective:** Ensure calculations preserve decimal precision and do not round prematurely.
- **Inputs:** Parcel resulting in chargeable weight 5.02 kg.
- **Assertions:** Must NOT round down to 5.00 kg (Band 4); must select Band 5 ($5 < W \le 10\text{ kg}$).

#### `TC-PRC-007`: Maximum Weight Limit Enforcement (30 kg Actual and Chargeable)
- **Objective:** Reject quotes when actual weight or chargeable weight exceeds 30.00 kg.
- **Cases:**
  - Case A: Actual weight = 30.01 kg $\implies$ Rejected HTTP 422 (`code: "PARCEL_LIMITS_EXCEEDED"`).
  - Case B: Actual weight = 8.0 kg, volumetric = 32.0 kg $\implies$ Chargeable = 32.0 kg $\implies$ Rejected HTTP 422 (`code: "PARCEL_LIMITS_EXCEEDED"`).
  - Case C: Chargeable weight exactly 30.00 kg $\implies$ Accepted in Band 7.
- **Assertions:** Cases A & B return HTTP 422; Case C returns HTTP 201.

#### `TC-PRC-008`: Missing Tariff & Ambiguity Handling
- **Objective:** Return controlled error if no tariff exists, and fail safely on ambiguous rules.
- **Cases:**
  - Case A (Missing Rule): Zone pair with no configured rule $\implies$ HTTP 422 (`code: "PRICING_UNAVAILABLE"`).
  - Case B (Ambiguous Match): Multiple active rules match same zone pair and band $\implies$ HTTP 500 (`code: "PRICING_CONFIGURATION_ERROR"`).
- **Assertions:** Neither case fabricates a price or falls back to arbitrary defaults.

#### `TC-PRC-009`: Quote Expiry at Exact Boundary (`current_time >= expires_at`)
- **Objective:** Enforce 15-minute quote expiration at the exact boundary.
- **Setup:** Create quote with `expires_at = T0 + 15m`.
- **Cases:**
  - Case A: Booking submitted at `T0 + 14m 59s` $\implies$ Accepted (HTTP 201).
  - Case B: Booking submitted at `T0 + 15m 00s` (`current_time >= expires_at`) $\implies$ Rejected HTTP 422 (`code: "QUOTE_EXPIRED"`).
  - Case C: Booking submitted at `T0 + 16m` $\implies$ Rejected HTTP 422 (`code: "QUOTE_EXPIRED"`).
- **Assertions:** Boundary check evaluates strictly on the backend; frontend expiration is non-authoritative.

#### `TC-PRC-010`: Tariff Version Change & Historical Quote References
- **Objective:** Confirm that updating a pricing rule does not alter already-issued quotes or confirmed bookings.
- **Setup:** Rule v1 issued for quote Q1. Admin creates Rule v2 with new tariff.
- **Action:** Customer books parcel using quote Q1 within its 15-minute window.
- **Assertions:** Booking succeeds using Q1's agreed fee snapshot; `parcels.pricing_rule_version == 1` and `parcels.price_cents` matches Q1.

#### `TC-PRC-011`: Tampered or Mismatched Quote Inputs & Authorization Protection
- **Objective:** Prevent quote tampering and unauthorized cross-customer quote reuse.
- **Cases:**
  - Case A: Altering weight, dimensions, or route between quote and booking $\implies$ HTTP 422 (`code: "QUOTE_PARAM_MISMATCH"`).
  - Case B: Customer B attempts to book with Customer A's quote $\implies$ HTTP 403 Forbidden (`code: "QUOTE_FORBIDDEN"`).

---

### 5.2 Parcel Booking & Input Validation (`TC-BKG-*`, `TC-VAL-*`)

#### `TC-VAL-001`: Maximum Weight Boundary Enforcement
- **Objective:** Reject bookings exceeding operational limit of 30.00 kg.
- **Inputs:** Weight = 30.01 kg.
- **Assertions:** HTTP 422 Unprocessable Entity (`code: "PARCEL_LIMITS_EXCEEDED"`).

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

#### `TC-BKG-007`: Immutable Accepted Price Snapshot Verification
- **Objective:** Verify that confirmed parcel booking retains original `price_cents`, currency, and pricing rule version even after active tariff changes.
- **Setup:** Book parcel using Quote Q1 under Tariff Rule v1.
- **Action:** Member 7 admin workflow creates Tariff Rule v2 with modified fee. Retrieve booked parcel via `GET /api/v1/parcels/{id}`.
- **Assertions:** Stored `price_cents` matches Q1 snapshot exactly; `pricing_rule_version == 1`; historical price remains immutable.

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
| **Pricing Model & Complete Tariff** | General estimation tests | Fixed complete tariff `tariff(pickup, dropoff, band)`; no separate base/per-kg/distance fees | **PROVISIONAL PROPOSAL (PENDING TEAM CONFIRMATION)** |
| **Weight Bands (7 Bands)** | Undefined band boundaries | 7 non-overlapping bands: (0, 0.5], (0.5, 1], (1, 2], (2, 5], (5, 10], (10, 20], (20, 30] kg; proposed max 30 kg weight limits | **PROVISIONAL PROPOSAL (PENDING TEAM CONFIRMATION)** |
| **Volumetric Policy & Divisor (6000)** | Divisor 5000 / arbitrary calculation | Option A: actual weight <= 1 kg uses actual weight only; > 1 kg requires dimensions and uses `max(actual, (L*W*H)/6000)` | **PROVISIONAL PROPOSAL (PENDING TEAM CONFIRMATION)** |
| **Quote Expiry Test** | Undefined quote TTL | 15-minute expiration test (`current_time >= expires_at` enforced on backend) | **PROVISIONAL PROPOSAL (PENDING TEAM CONFIRMATION)** |
| **Address Verification** | Tested `parcel_addresses` child table | Tests JSONB snapshots on `parcels` | Database Schema Standards |
| **Failure Semantics** | Tested terminal `FAILED` | Tested transient `ATTEMPT_FAILED` (max 3) vs `RETURNED` | ADR-004 Failed Delivery Semantics |
| **Migration Testing** | Unnumbered migrations | Explicit testing of `000004`, `000005`, `000010` | Database Migration Baseline |
| **Tariff Administration** | Mixed ownership tests | Admin write UI & workflow owned by M7; M2 tests calculation engine & lookup | Coordinated Boundary |
| **Receipt File Format** | Tested arbitrary PDF output | M2 tests JSON snapshot; M6 tests rendering | `PENDING TEAM CONFIRMATION` |

### 7.1 Pricing Policy Dependency & Test Suite Impact Analysis

| Test Suite / Area | Invariant Framework (Unaffected by Pricing Changes) | Impact If Pricing Policy Changes |
| :--- | :--- | :--- |
| **Pricing Unit Tests (`internal/pricing/...`)** | Unit test framework, deterministic calculation assertion, zero-float validation. | Mathematical expected values in `TC-PRC-001` through `TC-PRC-007` test complete tariff lookups and band selections rather than base/per-kg formulas. |
| **Quote Expiration Tests** | Test verifying expired quotes are rejected during booking with HTTP 422 (`QUOTE_EXPIRED`). | If the 15-minute TTL changes, the synthetic timestamp delta in `TC-PRC-009` will adjust to match the ratified duration. |
| **Booking & Idempotency Integration** | Booking transaction atomicity, `Idempotency-Key` replay, payload mismatch conflict (409), address snapshots. | **Zero Impact:** Booking workflows test the accepted price snapshot (`TC-BKG-007`) irrespective of internal tariff rules. |
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
