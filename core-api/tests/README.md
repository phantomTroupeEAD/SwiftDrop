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

## 2. Acceptance & Concurrency Test Suites (Member 3)

The following high-value end-to-end acceptance tests verify key architectural invariants:

| Test File | Acceptance Criteria | Description |
| :--- | :--- | :--- |
| `at_013_assignment_concurrency_test.go` | **AT-013** | Concurrency Race: 10 simultaneous goroutines racing to assign the same parcel. Verifies that row locking and `idx_assignments_parcel_single_active` serialize all requests: exactly 1 succeeds (`201 Created`), 9 fail (`409 Conflict`), and exactly 1 active assignment is persisted. |
| `at_006_courier_idor_test.go` | **AT-006** | IDOR Defense: Verifies that only the legitimately assigned courier can perform delivery actions (`start-run`, `proof-photo`, `complete`, `failed`). Foreign couriers are barred with `403 Forbidden` (`COURIER_NOT_ASSIGNED`). |
| `adr_004_delivery_ceiling_test.go` | **ADR-004** | Attempt Ceiling: Sequential verification of attempts 1 and 2 transitioning to `ATTEMPT_FAILED`, attempt 3 transitioning to terminal `RETURNED`, blocking of 4th attempts, and immediate return on `REFUSED_BY_RECIPIENT`. |
| `delivery_atomicity_test.go` | **Atomicity** | Single Transaction Guarantee: Proves that downstream outbox or database constraint errors trigger an atomic rollback, leaving parcel status, delivery attempts, and assignments uncorrupted. |

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
