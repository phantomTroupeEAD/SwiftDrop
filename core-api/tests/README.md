# Cross-Module Integration Test Harness (`core-api/tests`)

> **Owner:** Member 08 (Harness Setup) / Shared by All Team Members (Members 1–8)  
> **Status:** Active & Ready for Integration Testing

---

## 1. Overview
This package provides a standardized integration test harness for running cross-module tests against a real PostgreSQL database with fully applied migrations and schema isolation.

Key capabilities:
- **Zero-Contention Schema Isolation:** Every integration test run creates a temporary schema (`itest_<timestamp>`), runs canonical migrations via `migrations.RunMigrations`, and automatically drops the schema upon completion.
- **Opt-In Execution:** Integration tests only run when `INTEGRATION=1` is set in the environment, preventing unit test slowdowns in local development.
- **Seeded Data Ready:** Includes all seeded accounts from `20261021000002_seed_mock_users.up.sql`.

---

## 2. How to Add Integration Tests for Your Module

Teammates (Members 1, 2, 3, 4, 7, etc.) can write end-to-end integration tests following this pattern:

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

## 3. Running Integration Tests Locally

Ensure local PostgreSQL is running:
```bash
docker compose -f services/messaging/docker-compose.dev.yml up -d postgres
```

Run tests with `INTEGRATION=1`:
```bash
INTEGRATION=1 TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/swiftdrop?sslmode=disable" go test -v -count=1 ./tests/...
```

