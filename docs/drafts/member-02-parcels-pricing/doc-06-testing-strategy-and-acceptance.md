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
- **Deterministic Pricing & Tariff Calculation Verification:** Zone-pair-based tariff lookup with chargeable weight determined by $\max(\text{actual\_weight}, \text{volumetric\_weight})$ using volumetric divisor $5000\text{ cm}^3/\text{kg}$, operational weight limits ($0.01 \le W \le 30.00\text{ kg}$), and 15-minute quote expiration enforcement.
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
| **FR-PAR-001** | Booking Details | `TC-BKG-001`, `TC-BKG-002` | API / DB Integration | Captures dimensions, weights, and immutable JSONB address snapshots. |
| **FR-PAR-002** | Pricing Quote | `TC-PRC-001` to `TC-PRC-011`| Unit / API Integration | Computes zone-pair tariff, chargeable weight $\max(\text{actual}, \text{volumetric})$ with divisor 5000, 15m quote expiry. |
| **FR-PAR-003** | Accepted Price | `TC-PRC-010`, `TC-BKG-007` | Database Integration | Persists immutable rule version and LKR cents snapshot. |
| **FR-PAR-004** | Tracking Number | `TC-TRK-001`, `TC-TRK-002` | Unit / DB Integration | Crockford Base32 format `SD-YYYYMMDD-XXXXXX`; unique. |
| **FR-PAR-005** | Input Validation | `TC-VAL-001` to `TC-VAL-005` | Unit / API Integration | Rejects non-positive dims/weights (400), weights $> 30$ kg, dims $> 150$ cm (422). |
| **FR-PAR-006** | Idempotency | `TC-IDM-001`, `TC-IDM-002` | API / DB Integration | Key replay with `caller_scope` returns cached 201; payload mismatch returns 409. |
| **FR-PAR-007** | Caller Scoping | `TC-SEC-001`, `TC-SEC-002` | Security Integration | Prevents IDOR cross-tenant access to foreign parcels. |
| **FR-PAR-008** | Booking Receipt | `TC-READ-003` | API Integration | Receipt reflects stored price snapshot, not current tariff. |
| **FR-PAR-009** | Public Privacy | `TC-SEC-002` | Security Integration | Public tracking redacts recipient phone, street address, notes. |
| **FR-LIFE-001** | Status Progression | `TC-LCY-001` | Unit / API Integration | Enforces canonical progression through 9 aggregate states. |
| **FR-LIFE-002** | Tracking Visibility | `TC-LCY-001`, `TC-SEC-002` | API Integration | Authoritative state updates reflected in tracking milestones. |
| **FR-LIFE-003** | Transition Auth | `TC-SEC-004` | Security Integration | Enforces actor permissions per lifecycle state transition. |
| **FR-LIFE-004** | Proof of Delivery | `TC-LCY-001` | Domain Integration | Delivery completion requires valid POD attachment (Member 3). |
| **FR-LIFE-005** | Duplicate Transition | `TC-LCY-002` | Unit / DB Integration | Idempotent handling / rejection of duplicate status transitions. |
| **FR-LIFE-006** | Failure Reason Code| `TC-LCY-003` | API / DB Integration | Failed delivery attempts capture standardized reason codes. |
| **FR-LIFE-007** | Audited Corrections| `TC-LCY-006` | DB Integration | Dispatcher corrections logged immutably in status history. |
| **FR-ADM-007** | Tariff Admin API | N/A | Admin Integration | Tariff configuration workflows owned by Member 7 admin workflow. |
| **SEC-004** | Auth Token Verify | `TC-SEC-001` | Security Integration | Rejects missing/expired/invalid JWT bearer tokens (401). |
| **SEC-006** | RBAC Enforcement | `TC-SEC-004` | Security Integration | Rejects unauthorized role access to mutating endpoints (403). |
| **SEC-007** | Tenant Isolation | `TC-SEC-001` | Security Integration | Prevents cross-customer and cross-merchant data leakage. |
| **SEC-008** | Input Sanitization | `TC-VAL-001` to `TC-VAL-005` | Unit / API Integration | Enforces strict bounds on weight, dimensions, and text fields. |
| **SEC-009** | Idempotency Replay | `TC-IDM-001`, `TC-IDM-002` | API / DB Integration | Protects against duplicate billing/booking on network retries. |
| **SEC-015** | PII Masking | `TC-SEC-002` | Security Integration | Public tracking redacts recipient contact name, phone, address. |

---

## 5. Detailed Test Case Specifications

### 5.1 Deterministic Pricing & Quote Management (`TC-PRC-*`)

> [!NOTE]
> **Pricing Alignment Status (Ratified Base Fee + Per-Kg Surcharge):**
> Delivery fees are computed using the ratified formula: $\text{Delivery Fee} = \text{Base Fee} + (\text{Chargeable Weight} \times \text{Per-Kilogram Rate})$.
> Chargeable weight is computed consistently using $\max(\text{actual\_weight}, \text{volumetric\_weight})$ with volumetric divisor $5000\text{ cm}^3/\text{kg}$ across all eligible weights ($0.01 \le W \le 30.00\text{ kg}$). Quotes carry a strict 15-minute validity window. Monetary calculations use exact integer minor units (`BIGINT` LKR cents). Intermediate rounding for fractional kilograms remains an open item pending team confirmation.

#### `TC-PRC-001`: Volumetric Weight Calculation (Divisor 5,000 cm³/kg)
- **Objective:** Verify standard volumetric weight calculation using the IATA standard divisor of $5000\text{ cm}^3/\text{kg}$.
- **Inputs:** Dimensions: Length = 30.00 cm, Width = 20.00 cm, Height = 15.00 cm; Actual weight = 1.00 kg.
- **Calculations:**
  - Volumetric weight = $(30 \times 20 \times 15) / 5000 = 9000 / 5000 = 1.80\text{ kg}$.
  - Chargeable weight = $\max(1.00, 1.80) = 1.80\text{ kg}$.
- **Assertions:** Returns HTTP 201; response body contains `chargeable_weight_kg == 1.80`, `volumetric_weight_kg == 1.80`, and fee calculated based on 1.80 kg.

#### `TC-PRC-002`: Actual Weight Greater Than Volumetric Weight
- **Objective:** Verify that when actual weight exceeds volumetric weight, actual weight is selected as chargeable weight.
- **Inputs:** Dimensions: Length = 20.00 cm, Width = 20.00 cm, Height = 15.00 cm; Actual weight = 4.50 kg.
- **Calculations:**
  - Volumetric weight = $(20 \times 20 \times 15) / 5000 = 6000 / 5000 = 1.20\text{ kg}$.
  - Chargeable weight = $\max(4.50, 1.20) = 4.50\text{ kg}$.
  - For tariff (Base = 35,000 cents, Per-kg = 10,000 cents): Weight surcharge = $4.50 \times 10000 = 45000$ cents. Total fee = $35000 + 45000 = 80000$ cents (LKR 800.00).
- **Assertions:** Returns HTTP 201; response body contains `chargeable_weight_kg == 4.50`, `volumetric_weight_kg == 1.20`, and `fee_cents == 80000`.

#### `TC-PRC-003`: Volumetric Weight Greater Than Actual Weight
- **Objective:** Verify that when volumetric weight exceeds actual weight, volumetric weight is selected as chargeable weight.
- **Inputs:** Dimensions: Length = 40.00 cm, Width = 30.00 cm, Height = 25.00 cm; Actual weight = 2.00 kg.
- **Calculations:**
  - Volumetric weight = $(40 \times 30 \times 25) / 5000 = 30000 / 5000 = 6.00\text{ kg}$.
  - Chargeable weight = $\max(2.00, 6.00) = 6.00\text{ kg}$.
  - For tariff (Base = 30,000 cents, Per-kg = 10,000 cents): Weight surcharge = $6.00 \times 10000 = 60000$ cents. Total fee = $30000 + 60000 = 90000$ cents (LKR 900.00).
- **Assertions:** Returns HTTP 201; response body contains `chargeable_weight_kg == 6.00`, `volumetric_weight_kg == 6.00`, and `fee_cents == 90000`.

#### `TC-PRC-004`: Equal Actual and Volumetric Weight
- **Objective:** Verify correct handling when actual weight and volumetric weight are exactly equal.
- **Inputs:** Dimensions: Length = 25.00 cm, Width = 20.00 cm, Height = 25.00 cm; Actual weight = 2.50 kg.
- **Calculations:**
  - Volumetric weight = $(25 \times 20 \times 25) / 5000 = 12500 / 5000 = 2.50\text{ kg}$.
  - Chargeable weight = $\max(2.50, 2.50) = 2.50\text{ kg}$.
  - For tariff (Base = 30,000 cents, Per-kg = 10,000 cents): Weight surcharge = $2.50 \times 10000 = 25000$ cents. Total fee = $30000 + 25000 = 55000$ cents (LKR 550.00).
- **Assertions:** Returns HTTP 201; response body contains `chargeable_weight_kg == 2.50`, `volumetric_weight_kg == 2.50`, and `fee_cents == 55000`.

#### `TC-PRC-005`: Combined Base Fee and Weight Surcharge Calculation
- **Objective:** Verify deterministic execution of the complete fee formula: $\text{Delivery Fee} = \text{base\_fee\_cents} + (\text{chargeable\_weight\_kg} \times \text{per\_kg\_rate\_cents})$.
- **Inputs:** Zone pair `COLOMBO_METRO` $\to$ `KANDY_CENTRAL`; Chargeable weight = 6.00 kg.
- **Tariff Rule:** `base_fee_cents = 30000` (LKR 300.00), `per_kg_rate_cents = 10000` (LKR 100.00).
- **Calculations:**
  - Base fee component = 30,000 cents.
  - Weight surcharge component = $6.00 \times 10000 = 60000$ cents.
  - Total delivery fee = $30000 + 60000 = 90000$ cents (LKR 900.00).
- **Assertions:** Returns HTTP 201; response body contains `base_fee_cents == 30000`, `per_kg_rate_cents == 10000`, `weight_surcharge_cents == 60000`, and `fee_cents == 90000`.

#### `TC-PRC-006`: Fractional Chargeable Weights and Intermediate Rounding
- **Objective:** Verify tariff calculation with fractional kilogram chargeable weights and ensure result is exact integer cents.
- **Inputs:** Dimensions $25 \times 20 \times 15$ cm (volumetric 1.50 kg), Actual weight = 0.80 kg $\implies$ Chargeable weight = 1.50 kg.
- **Tariff Rule:** `base_fee_cents = 35000` (LKR 350.00), `per_kg_rate_cents = 10000` (LKR 100.00).
- **Calculations:**
  - Continuous calculation: Weight surcharge = $1.50 \times 10000 = 15000$ cents. Total fee = $35000 + 15000 = 50000$ cents (LKR 500.00).
- **Assertions:** Returns HTTP 201; `fee_cents == 50000`.
- **Note on Rounding Policy:** If the team ratifies a ceiling policy (e.g. `ceil(chargeable_weight_kg)`), $1.50\text{ kg} \implies 2.00\text{ kg}$, resulting in fee $35000 + (2 \times 10000) = 55000$ cents. This test verifies exact continuous calculation pending team confirmation of ceiling behavior.

#### `TC-PRC-007`: Isolated Base Fee & Per-Kilogram Surcharge Components
- **Objective:** Verify isolated behavior of the base fee component and per-kilogram rate multiplier.
- **Cases:**
  - Case A (Base Fee Isolation): Minimum billable parcel ($W \to 0$ boundary limit or 0.01 kg) incurs the base fee baseline. Base fee is never bypassed or discounted.
  - Case B (Per-Kg Rate Linearity): For an incremental weight increase of $\Delta W = 1.00\text{ kg}$, fee increases by exactly $\Delta \text{Fee} = \text{per\_kg\_rate\_cents}$.
- **Assertions:** Base fee component remains constant across all weights for the zone pair; surcharge scales linearly with chargeable weight.

#### `TC-PRC-008`: Missing, Inactive, Expired, or Negative Tariff Values
- **Objective:** Return controlled HTTP 422 error if no active tariff exists for the requested zone pair, and fail safely on invalid or ambiguous configuration.
- **Cases:**
  - Case A (Missing Rule): Zone pair with no configured active tariff rule $\implies$ HTTP 422 (`code: "PRICING_UNAVAILABLE"`).
  - Case B (Inactive Rule): Tariff exists with `is_active = FALSE` $\implies$ HTTP 422 (`code: "PRICING_UNAVAILABLE"`).
  - Case C (Expired Rule): Tariff exists but `effective_to < NOW()` $\implies$ HTTP 422 (`code: "PRICING_UNAVAILABLE"`).
  - Case D (Negative Rates): Database CHECK constraint rejects `base_fee_cents < 0` or `per_kg_rate_cents < 0`. If encountered at runtime $\implies$ HTTP 500 (`code: "PRICING_CONFIGURATION_ERROR"`).
  - Case E (Ambiguous Match): Multiple active rules match same zone pair $\implies$ HTTP 500 (`code: "PRICING_CONFIGURATION_ERROR"`).
- **Assertions:** No case fabricates a price or falls back to arbitrary defaults; errors are strictly typed and logged.

#### `TC-PRC-009`: Expired Pricing Quote Enforcement (`current_time >= expires_at`)
- **Objective:** Enforce 15-minute quote expiration at the exact boundary during parcel booking.
- **Setup:** Create quote with `expires_at = T0 + 15m`.
- **Cases:**
  - Case A: Booking submitted at `T0 + 14m 59s` $\implies$ Accepted (HTTP 201 Created).
  - Case B: Booking submitted at `T0 + 15m 00s` (`current_time >= expires_at`) $\implies$ Rejected HTTP 422 (`code: "QUOTE_EXPIRED"`).
  - Case C: Booking submitted at `T0 + 16m 00s` $\implies$ Rejected HTTP 422 (`code: "QUOTE_EXPIRED"`).
- **Assertions:** Expiration is evaluated strictly on the backend against server authoritative time; frontend timer displays are non-authoritative.

#### `TC-PRC-010`: Tariff Version Changes (Rule v1 to v2)
- **Objective:** Confirm that updating an active tariff rule (version increment) does not invalidate or alter already-issued unexpired quotes or confirmed bookings.
- **Setup:** Tariff Rule v1 is active (`base_fee_cents = 30000`, `per_kg_rate_cents = 10000`, `version = 1`). Quote Q1 is generated for 2.0 kg with `fee_cents = 50000`, `pricing_rule_version = 1`. Admin updates tariff to Rule v2 (`base_fee_cents = 35000`, `per_kg_rate_cents = 12000`, `version = 2`).
- **Action:** Customer books parcel using Quote Q1 within its 15-minute validity window.
- **Assertions:** Booking succeeds; parcel accepts Quote Q1's agreed fee snapshot: `parcels.price_cents == 50000`, `parcels.pricing_rule_version == 1`. Subsequent quote requests generate quotes under Tariff v2 ($35000 + (2 \times 12000) = 59000$ cents, version = 2).

#### `TC-PRC-011`: Accepted-Price Snapshot Immutability & Quote Tampering Protection
- **Objective:** Ensure confirmed bookings retain agreed price snapshot permanently in `parcels.price_cents` regardless of subsequent tariff updates, and prevent quote tampering or unauthorized cross-customer quote reuse.
- **Cases:**
  - Case A (Snapshot Immutability): Subsequent rate updates or rule deletions never alter historical parcel booking records (`parcels.price_cents`).
  - Case B (Parameter Mismatch): Client modifies weight, dimensions, or pickup/dropoff zone between quote generation and booking $\implies$ HTTP 422 (`code: "QUOTE_PARAM_MISMATCH"`).
  - Case C (Cross-Customer Theft): Customer B attempts to book with Customer A's quote $\implies$ HTTP 403 Forbidden (`code: "QUOTE_FORBIDDEN"`).

---

### 5.2 Input Validation & Parcel Booking (`TC-VAL-*`, `TC-BKG-*`)

#### `TC-VAL-001`: Invalid Dimensions Validation (Zero, Negative, or Non-Numeric)
- **Objective:** Reject dimension parameters that are non-positive or non-numeric.
- **Cases:**
  - Case A: Length = 0.00 cm, Width = 20.00 cm, Height = 15.00 cm.
  - Case B: Length = -10.00 cm, Width = 20.00 cm, Height = 15.00 cm.
  - Case C: Non-numeric dimension `"length_cm": "invalid"`.
- **Assertions:** All cases rejected with HTTP 400 Bad Request (`code: "INVALID_ARGUMENT"`).

#### `TC-VAL-002`: Invalid Weight Validation (Zero, Negative, or Non-Numeric)
- **Objective:** Reject actual weight parameters that are non-positive or non-numeric.
- **Cases:**
  - Case A: Weight = 0.00 kg.
  - Case B: Weight = -1.50 kg.
  - Case C: Non-numeric weight `"weight_kg": "heavy"`.
- **Assertions:** All cases rejected with HTTP 400 Bad Request (`code: "INVALID_ARGUMENT"`).

#### `TC-VAL-003`: Maximum Parcel Weight Boundary (Exactly 30.00 kg)
- **Objective:** Verify that parcels with actual and chargeable weight of exactly 30.00 kg are accepted.
- **Inputs:** Actual weight = 30.00 kg, dimensions = $30 \times 25 \times 20$ cm (volumetric = 3.00 kg). Chargeable weight = 30.00 kg.
- **Assertions:** Request accepted; HTTP 201 Created (or quote generated successfully for 30.00 kg limit).

#### `TC-VAL-004`: Weight Exceeding Maximum Limit (> 30.00 kg)
- **Objective:** Reject parcels where actual weight or chargeable volumetric weight exceeds the operational limit of 30.00 kg.
- **Cases:**
  - Case A (Actual Weight Exceeded): Actual weight = 30.01 kg, dimensions within limits $\implies$ HTTP 422 Unprocessable Entity (`code: "PARCEL_LIMITS_EXCEEDED"`).
  - Case B (Chargeable Volumetric Exceeded): Actual weight = 8.00 kg, dimensions $80 \times 60 \times 40$ cm $\implies$ Volumetric weight = $(80 \times 60 \times 40) / 5000 = 38.40\text{ kg} > 30.00\text{ kg} \implies$ HTTP 422 Unprocessable Entity (`code: "PARCEL_LIMITS_EXCEEDED"`).
- **Assertions:** Both cases rejected before persistence or quote creation.

#### `TC-VAL-005`: Maximum Dimension Boundary Enforcement (150.00 cm)
- **Objective:** Reject parcels exceeding maximum single dimension of 150.00 cm.
- **Inputs:** Length = 150.50 cm, Width = 20.00 cm, Height = 20.00 cm.
- **Assertions:** HTTP 422 Unprocessable Entity (`code: "DIMENSION_LIMIT_EXCEEDED"`).

#### `TC-BKG-001`: Valid Quoted Parcel Booking Confirmation
- **Objective:** Verify end-to-end parcel booking confirmation with a valid, unexpired quote, generating tracking number and consuming the quote.
- **Action:** Send `POST /api/v1/parcels` with valid `Idempotency-Key` and active `quote_id`.
- **Assertions:**
  - HTTP 201 Created with `Location: /api/v1/parcels/{id}`.
  - Authoritative tracking number generated matching regex `^SD-[0-9]{8}-[0-9A-Z]{6}$`.
  - Initial parcel status is `CREATED`.
  - Quote status marked as consumed (`is_consumed == TRUE`), preventing double booking.
  - Pickup and dropoff addresses persisted as structured JSONB snapshots in `parcels.pickup_address` and `parcels.dropoff_address`.

#### `TC-BKG-002`: Address Snapshot Preservation on Profile Address Change
- **Objective:** Verify that mutating or updating a customer's saved address in their address book does not alter existing parcel address snapshots.
- **Setup:** Book parcel with pickup address snapshot: `{"contact_name": "Kasun", "address_line1": "123 Galle Rd", "city": "Colombo", "postal_code": "00300"}`.
- **Action:** Customer profile modifies saved address to `"456 Kandy Rd", "Kadawatha"`.
- **Assertions:** Query `GET /api/v1/parcels/{id}`: `pickup_address` remains strictly `123 Galle Rd, Colombo, 00300`. Stored JSONB snapshot is completely immutable.

#### `TC-BKG-003`: Atomic Persistence of Parcel, Initial Status History, and Outbox Event
- **Objective:** Verify transactional atomicity: parcel row, initial status history, and outbox event commit together or roll back completely.
- **Setup:** Initiate booking request; inject synthetic database failure on outbox event insertion.
- **Assertions:**
  - Database transaction rolls back entirely.
  - Zero rows inserted into `parcels`, `parcel_status_history`, or `outbox_events`.
  - Client receives HTTP 500 Internal Server Error without orphan partial state.
  - On normal execution without error, all three records are committed in the exact same ACID transaction.

#### `TC-BKG-007`: Accepted-Price Immutability After Booking
- **Objective:** Verify that confirmed parcel booking retains original `price_cents`, currency, and pricing rule version permanently, even after subsequent tariff modifications.
- **Setup:** Book parcel with Quote Q1 under Tariff Rule v1 (`price_cents = 35000`, `pricing_rule_version = 1`).
- **Action:** Admin modifies tariff rule to v2 (`price_cents = 48000`).
- **Assertions:** Query `GET /api/v1/parcels/{id}`: parcel returns `price_cents == 35000`, `currency == "LKR"`, `pricing_rule_version == 1`. Price snapshot is permanent and immutable.

---

### 5.3 Idempotency & Concurrency (`TC-IDM-*`)

#### `TC-IDM-001`: Duplicate Booking Requests & Idempotency Handling (Identical Replay)
- **Objective:** Verify that identical repeated booking requests under the same `Idempotency-Key` return the original cached response without duplicate records.
- **Action:** Send identical `POST /api/v1/parcels` payload with identical `Idempotency-Key` header twice.
- **Assertions:**
  - First request: HTTP 201 Created (new parcel record inserted).
  - Second request: HTTP 201 Created (cached payload returned from `idempotency_keys` record).
  - Exactly 1 row exists in `parcels` table.

#### `TC-IDM-002`: Idempotency Key Reuse with Payload Conflict
- **Objective:** Detect and reject payload tampering or parameter mismatch under a reused idempotency key.
- **Action:** Send initial booking with key `KEY-ABC`. Send second booking with same key `KEY-ABC` but different destination address or weight.
- **Assertions:** Second request returns HTTP 409 Conflict (`code: "IDEMPOTENCY_KEY_PAYLOAD_MISMATCH"`). Zero additional parcels inserted.

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

#### `TC-LCY-002`: Invalid Parcel Lifecycle Transitions
- **Objective:** Reject invalid lifecycle shortcuts, unauthorised mutations, and modifications to terminal states.
- **Cases:**
  - Case A (Shortcut): Attempting to transition directly from `CREATED` to `DELIVERED` $\implies$ HTTP 409 Conflict (`code: "INVALID_STATE_TRANSITION"`).
  - Case B (Terminal Mutation): Attempting to transition a parcel in `DELIVERED`, `CANCELLED`, or `RETURNED` to any other state $\implies$ HTTP 409 Conflict (`code: "INVALID_STATE_TRANSITION"`).
  - Case C (Missing Assignment): Attempting `PICKED_UP` transition when `courier_id` is null $\implies$ HTTP 409 Conflict (`code: "COURIER_NOT_ASSIGNED"`).
- **Assertions:** State remains unchanged; zero audit history rows appended.

#### `TC-LCY-003`: Failed Delivery Attempt Increment (ADR-004)
- **Objective:** Verify transient failed delivery attempt semantics.
- **Setup:** Parcel in `OUT_FOR_DELIVERY` (`attempt_count = 0`).
- **Action:** Courier reports delivery failure with standardized reason code (`RECIPIENT_UNAVAILABLE`).
- **Assertions:**
  - Status transitions to `ATTEMPT_FAILED`.
  - `attempt_count` increments to 1.
  - Parcel is NOT in terminal state; can be rescheduled to `OUT_FOR_DELIVERY`.

#### `TC-LCY-004`: Maximum Delivery Attempts Exceeded (ADR-004)
- **Objective:** Verify terminal return when delivery attempts reach 3.
- **Setup:** Parcel in `ATTEMPT_FAILED` with `attempt_count = 3`.
- **Action:** Dispatcher/system triggers return process.
- **Assertions:** Status transitions to terminal `RETURNED`. No further delivery reattempts allowed.

#### `TC-LCY-005`: Parcel Booking Cancellation
- **Objective:** Verify customer/merchant cancellation prior to courier pickup.
- **Setup:** Parcel in `CREATED` status.
- **Action:** Authenticated resource owner calls `POST /api/v1/parcels/{id}/cancel`.
- **Assertions:**
  - HTTP 200 OK returned; status becomes `CANCELLED`.
  - Immutable entry appended to `parcel_status_history` with `reason_code = 'CUSTOMER_CANCELLED'`.
  - Outbox event `parcel.cancelled.v1` committed atomically.
  - Attempting to cancel a parcel in `PICKED_UP` or `IN_TRANSIT` fails with HTTP 409 Conflict (`INVALID_STATE_TRANSITION`).

#### `TC-LCY-006`: Status History Integrity & Deletion Protection
- **Objective:** Verify that status transitions record complete audit metadata and direct deletion is blocked.
- **Setup:** Parcel transitioned through multiple states.
- **Assertions:**
  - Each `parcel_status_history` record stores `actor_role`, `reason_code`, optional `note`, and `parcel_version`.
  - Executing `DELETE FROM parcels WHERE id = $1` is rejected with a foreign key violation (`ON DELETE RESTRICT`) to preserve regulatory history.

---

### 5.5 Security, Authorization & Privacy (`TC-SEC-*`)

#### `TC-SEC-001`: Tenant & Customer Isolation (IDOR Prevention)
- **Objective:** Prevent Customer A from viewing or modifying Customer B's parcels.
- **Action:** Customer A sends `GET /api/v1/parcels/{id_of_customer_b}`.
- **Assertions:** HTTP 403 Forbidden (or 404 Not Found to prevent resource enumeration).

#### `TC-SEC-002`: Address Validation & Privacy Enforcement
- **Objective:** Verify mandatory address field validation during booking and strict PII redaction on public tracking endpoints.
- **Cases:**
  - Case A (Booking Address Validation): Booking submitted with missing mandatory fields (e.g., missing `contact_phone` or `city`) $\implies$ HTTP 400 Bad Request (`code: "INVALID_ARGUMENT"`).
  - Case B (Public Tracking PII Redaction): Querying public tracking for a parcel returns only milestone statuses, city-level routing (`pickup_city`, `dropoff_city`), and tracking number. Full street address (`address_line1`), recipient name, recipient phone, and internal notes are strictly redacted.
- **Assertions:** Private customer details are never leaked to unauthenticated public tracking clients.

#### `TC-SEC-004`: Role-Based Access Control (RBAC Enforcement)
- **Objective:** Enforce role privileges across mutating operations.
- **Cases:**
  - Customer attempts to call admin endpoints $\implies$ HTTP 403 Forbidden.
  - Courier attempts to transition parcel from `CREATED` directly to `DELIVERED` $\implies$ HTTP 409 Conflict.
  - Unauthenticated request with missing/expired JWT $\implies$ HTTP 401 Unauthorized.

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

#### `TC-LCY-005`: Parcel Booking Cancellation
- **Objective:** Verify customer/merchant cancellation prior to courier pickup.
- **Setup:** Parcel in `CREATED` status.
- **Action:** Authenticated resource owner calls `POST /api/v1/parcels/{id}/cancel`.
- **Assertions:**
  - HTTP 200 OK returned; status becomes `CANCELLED`.
  - Immutable entry appended to `parcel_status_history` with `reason_code = 'CUSTOMER_CANCELLED'`.
  - Outbox event `parcel.cancelled.v1` committed atomically.
  - Attempting to cancel a parcel in `PICKED_UP` or `IN_TRANSIT` fails with HTTP 409 Conflict (`INVALID_STATE_TRANSITION`).

#### `TC-LCY-006`: Status History Integrity & Deletion Protection
- **Objective:** Verify that status transitions record complete audit metadata and direct deletion is blocked.
- **Setup:** Parcel transitioned through multiple states.
- **Assertions:**
  - Each `parcel_status_history` record stores `actor_role`, `reason_code`, optional `note`, and `parcel_version`.
  - Executing `DELETE FROM parcels WHERE id = $1` is rejected with a foreign key violation (`ON DELETE RESTRICT`) to preserve regulatory history.

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

#### `TC-SEC-004`: Role-Based Access Control (RBAC Enforcement)
- **Objective:** Enforce role privileges across mutating operations.
- **Cases:**
  - Customer attempts to call `POST /api/v1/admin/pricing-rules` $\implies$ HTTP 403 Forbidden.
  - Courier attempts to transition parcel from `CREATED` directly to `DELIVERED` $\implies$ HTTP 409 Conflict.
  - Unauthenticated request with missing/expired JWT $\implies$ HTTP 401 Unauthorized.

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
| **Monetary Validation** | Allowed float fee checks | Exact integer LKR cents comparison (`BIGINT`) | Engineering Standards v1.0 Section 6 |
| **Pricing Model** | 7 Fixed Weight Bands | Option 2: Base Fee + Per-Kilogram Surcharge (`base_fee_cents`, `per_kg_rate_cents`) for each `(pickup_zone_id, destination_zone_id)` pair. Fee = $\text{Base Fee} + (\text{Chargeable Weight} \times \text{Per-Kg Rate})$. | **RATIFIED TEAM DECISION** (Aligned with `pricing-rules.md`) |
| **Weight Bands (7 Bands)** | 7 non-overlapping bands | Discontinued. Replaced by continuous chargeable weight $\times$ per-kilogram rate surcharge. | **SUPERSEDED** (Team decision to use Option 2) |
| **Volumetric Policy & Divisor (5000)** | Divisor 6000 / Option A threshold | Divisor strictly $5000\text{ cm}^3/\text{kg}$; chargeable weight always $\max(\text{actual}, \text{volumetric})$ for all eligible parcels; Option A 1kg threshold discontinued | Aligned Decision v2.0 |
| **Quote Expiry Test** | Undefined quote TTL | 15-minute expiration test (`current_time >= expires_at` enforced on backend) | Aligned Architecture Decision |
| **Address Verification** | Tested `parcel_addresses` child table | Tests JSONB snapshots on `parcels`; child table creation omitted; schema conflict documented | Architecture Alignment Item (Conflict with `database-schema.md`) |
| **Failure Semantics** | Tested terminal `FAILED` | Tested transient `ATTEMPT_FAILED` (max 3) vs terminal `RETURNED` | ADR-004 Failed Delivery Semantics |
| **History Deletion Protection** | Unverified / cascade | Tested `ON DELETE RESTRICT` preventing deletion of parcels with status history | Engineering Standards & Audit |
| **Migration Testing** | Unnumbered migrations | Explicit testing of `000004`, `000005`, `000010` | Database Migration Baseline |
| **Tariff Administration** | Mixed ownership tests | Admin write UI & workflow owned by M7; M2 tests calculation engine & quote evaluation | Coordinated Boundary |
| **Receipt File Format** | Tested arbitrary PDF output | M2 tests JSON snapshot; M6 tests rendering | `PENDING TEAM CONFIRMATION` |

### 7.1 Pricing Policy Dependency & Test Suite Impact Analysis

| Test Suite / Area | Invariant Framework (Unaffected by Pricing Changes) | Impact If Pricing Policy Changes |
| :--- | :--- | :--- |
| **Pricing Unit Tests (`internal/pricing/...`)** | Unit test framework, deterministic calculation assertion, zero-float validation. | Mathematical expected values in `TC-PRC-001` through `TC-PRC-007` test Base Fee + Per-Kg Surcharge formulas rather than weight-band lookups. |
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

---

## Diagrams & Test Specification References

The following authoritative diagrams provide visual models of the workflows, state machines, and schemas under test, helping verify acceptance criteria and test coverage:

| Diagram Title | Associated Acceptance Test Cases | Lucidchart Editable URL | Repository Artifacts |
| :--- | :--- | :--- | :--- |
| **01 Parcel Management & Pricing Use-Case Diagram** | Actor boundary validation, RBAC tests (`TC-SEC-001` to `TC-SEC-004`) | [Lucidchart Document](https://lucid.app/lucidchart/da53bbf3-1b48-450c-9945-b1816a32ae17/edit) | [PlantUML Source](./diagrams/01-parcel-pricing-use-case.puml) \| [PNG Preview](./diagrams/01-parcel-pricing-use-case.png) |
| **02 Delivery Fee Estimation Activity Diagram** | `TC-PRC-001` through `TC-PRC-011` (Base fee + per-kg calculation, volumetric thresholds, limit rejections) | [Lucidchart Document](https://lucid.app/lucidchart/e9ac9f8d-5d88-4e74-9a52-5e09e5c696b4/edit) | [PlantUML Source](./diagrams/02-delivery-fee-estimation-activity.puml) \| [PNG Preview](./diagrams/02-delivery-fee-estimation-activity.png) \| [SVG](./diagrams/02-delivery-fee-estimation-activity.svg) |
| **03 Delivery Fee Estimation Sequence Diagram** | `TC-PRC-008` (Zone/rule lookup), `TC-PRC-009` (Quote 15m expiration & persistence) | [Lucidchart Document](https://lucid.app/lucidchart/456c449a-5f68-4d12-a051-027a04e87663/edit) | [PlantUML Source](./diagrams/03-delivery-fee-estimation-sequence.puml) \| [PNG Preview](./diagrams/03-delivery-fee-estimation-sequence.png) |
| **04 Customer Parcel Booking Activity Diagram** | `TC-BKG-001` to `TC-BKG-005` (Syntax, zone validation, quote binding, single transaction) | [Lucidchart Document](https://lucid.app/lucidchart/48f368fa-a237-4fea-bb84-c454dd788216/edit) | [PlantUML Source](./diagrams/04-parcel-booking-activity.puml) \| [PNG Preview](./diagrams/04-parcel-booking-activity.png) |
| **05 Customer Parcel Booking Sequence Diagram** | `TC-BKG-001` through `TC-BKG-007`, `TC-IDM-001` to `TC-IDM-002` (Idempotency, atomic commit, outbox) | [Lucidchart Document](https://lucid.app/lucidchart/c2b3114e-a7b8-4313-8172-1a82336a8cc1/edit) | [PlantUML Source](./diagrams/05-parcel-booking-sequence.puml) \| [PNG Preview](./diagrams/05-parcel-booking-sequence.png) |
| **06 Parcel Lifecycle State Machine Diagram** | `TC-LCY-001` through `TC-LCY-006` (9-state machine, ADR-004 3-attempt ceiling, transitions) | [Lucidchart Document](https://lucid.app/lucidchart/43a950d5-3596-4bb5-84c7-64f813f48a7c/edit) | [PlantUML Source](./diagrams/06-parcel-lifecycle-state-machine.puml) \| [PNG Preview](./diagrams/06-parcel-lifecycle-state-machine.png) |
| **07 Parcel & Pricing Component Architecture** | Clean architecture isolation, repository interfaces, dependency inversion | [Lucidchart Document](https://lucid.app/lucidchart/983208a4-c9f1-44f3-a6d8-118b89757207/edit) | [PlantUML Source](./diagrams/07-parcel-pricing-component.puml) \| [PNG Preview](./diagrams/07-parcel-pricing-component.png) |
| **08 Parcel & Pricing ERD (Logical Schema)** | `TC-SEC-001` to `TC-SEC-004`, schema integrity, migrations `000004`, `000005`, `000010` | [Lucidchart Document](https://lucid.app/lucidchart/d51127dc-51ad-4542-843a-423d44d6fd16/edit) | [PlantUML Source](./diagrams/08-parcel-pricing-erd.puml) \| [PNG Preview](./diagrams/08-parcel-pricing-erd.png) |
