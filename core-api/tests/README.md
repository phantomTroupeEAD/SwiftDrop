# Cross-Module Integration & Acceptance Test Harness (`core-api/tests`)

> **Owner:** Member 08 (Harness Setup) / Shared by All Team Members (Members 1–8)  
> **Status:** Active & Ready for Integration Testing

---

## 1. Overview
This package provides a standardized integration and acceptance test harness for running cross-module tests against a real PostgreSQL database with fully applied migrations, schema isolation, and concurrent race detection.

Key capabilities:
- **Zero-Contention Schema Isolation:** Every integration test run creates a temporary schema (`itest_<timestamp>`), runs canonical migrations via `migrations.RunMigrations`, and automatically drops the schema upon completion.
- **Opt-In Execution:** Integration tests only run when `INTEGRATION=1` is set in the environment, preventing unit test slowdowns in local development.
- **Seeded Data Ready:** Includes all seeded accounts from `20261021000002_seed_mock_users.up.sql` and `deploy/seeds/000006_couriers_seed.sql`.

---

## 2. Acceptance & Concurrency Test Suites (Member 10 Lead & Domain Owners)

The following high-value end-to-end acceptance tests verify key architectural invariants:

| Test File | Acceptance Criteria | Description |
| :--- | :--- | :--- |
| `at_001_full_lifecycle_test.go` | **AT-001** | Full Parcel Lifecycle: Validates sequential state machine progression (`CREATED` -> `ASSIGNED` -> `OUT_FOR_DELIVERY` -> `DELIVERED`), dynamic tariff pricing, POD photo proof, and versioned `parcel.delivered.v1` outbox commitment. |
| `at_002_validation_rejection_test.go` | **AT-002** | Validation Rejection: Proves that negative weights, exceeding 50kg limit, missing contact names/phones, empty streets, and malformed zone IDs return domain errors with zero partial database writes. |
| `at_004_tenant_idor_test.go` | **AT-004** | Cross-Tenant IDOR Defense: Verifies that Merchant Alpha cannot read, mutate, or delete Merchant Beta's tenant resources (403 Forbidden), and unauthenticated access returns 401 Unauthorized. |
| `at_005_public_tracking_masking_test.go` | **AT-005** | Public Tracking Masking (SEC-009): Verifies public lookups return sanitized milestone timelines without requiring authentication, while strictly omitting recipient telephone numbers, street addresses, and private courier notes. |
| `at_006_courier_idor_test.go` | **AT-006** | IDOR Defense: Verifies that only the legitimately assigned courier can perform delivery actions (`start-run`, `proof-photo`, `complete`, `failed`). Foreign couriers are barred with `403 Forbidden` (`COURIER_NOT_ASSIGNED`). |
| `adr_004_delivery_ceiling_test.go` | **AT-007 / ADR-004** | Attempt Ceiling: Sequential verification of attempts 1 and 2 transitioning to `ATTEMPT_FAILED`, attempt 3 transitioning to terminal `RETURNED`, blocking of 4th attempts, and immediate return on `REFUSED_BY_RECIPIENT`. |
| `at_010_consumer_idempotency_test.go` | **AT-010** | Consumer Idempotency: Proves that duplicate RabbitMQ message deliveries with identical `event_id` are deduplicated via the idempotency ledger, safely ACKed, and execute zero duplicate downstream webhooks or customer notifications. |
| `at_011_merchant_isolation_test.go` | **AT-011** | Merchant Suspension Lockout: Verifies that newly registered merchants are `PENDING` with zero booking privileges, approval restores credentials, and administrative suspension (`SUSPENDED`) immediately locks out order operations. |
| `at_012_webhook_retry_chaos_test.go` | **AT-012** | Webhook HMAC & Chaos Retry: Validates canonical HMAC-SHA256 signature formatting (`t=...,v1=...`), tamper detection, exponential retry backoff on transient HTTP 500 errors, and Dead-Letter Queue (DLQ) escalation. |
| `at_013_assignment_concurrency_test.go` | **AT-013** | Concurrency Race: 10 simultaneous goroutines racing to assign the same parcel. Verifies that row locking and `idx_assignments_parcel_single_active` serialize all requests: exactly 1 succeeds (`201 Created`), 9 fail (`409 Conflict`), and exactly 1 active assignment is persisted. |
| `delivery_atomicity_test.go` | **Atomicity** | Single Transaction Guarantee: Proves that downstream outbox or database constraint errors trigger an atomic rollback, leaving parcel status, delivery attempts, and assignments uncorrupted. |
| `test_auth_helpers.go` | **Auth Fixtures** | Shared Token Helpers: Reuses canonical `identityinfra.JWTTokenProvider` to issue signed test tokens for Customer, Merchant Alpha, Merchant Beta, Dispatcher, Courier, and Admin. Verified by `test_auth_helpers_test.go`. |

---

## 3. How to Add Integration Tests for Your Module

Teammates can write end-to-end integration tests following this pattern:

```go
func TestMyModule_Integration(t *testing.T) {
    // 1. Initialize isolated PostgreSQL schema
    db, cleanup := tests.SetupTestDB(t)
    defer cleanup()

    // 2. Wire up your module repositories & use cases against db
    myRepo := mymodule.NewPostgresRepository(db)
    outboxWriter := events.NewPostgresOutboxWriter(db)
    myUseCase := mymodule.NewUseCase(myRepo, outboxWriter)

    // 3. Execute domain workflow within a database transaction
    tx, err := db.BeginTx(context.Background(), nil)
    if err != nil {
        t.Fatal(err)
    }
    defer tx.Rollback()

    // Perform state changes and outbox event writes...
    // err := myUseCase.Execute(ctx, tx, cmd)
    // tx.Commit()

    // 4. Assert database state and outbox entries
}
```

---

## 4. Running Tests Locally

### Running Unit & Concurrency Tests (with Race Detector):
```bash
go test -race -v -count=1 ./tests/...
```

### Running Full Database Integration Tests:
Ensure local PostgreSQL is running:
```bash
docker compose -f services/messaging/docker-compose.dev.yml up -d postgres
```

Run tests with `INTEGRATION=1`:
```bash
INTEGRATION=1 TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/swiftdrop?sslmode=disable" go test -v -count=1 ./tests/...
```
