# SwiftDrop Core API Database Migrations

## Overview & Authority
This directory houses the authoritative PostgreSQL schema migrations for the SwiftDrop Core API Modular Monolith, governed by **Member 1 (Identity)**, **Member 2 (Parcels & Pricing)**, **Member 3 (Dispatch & Delivery)**, **Member 4 (Merchants)**, **Member 7 (Admin)**, and **Member 8 (Events/Outbox)**.

All schema modifications follow strict forward-and-backward compatibility rules defined in `AGENTS.md` and `docs/contracts/database-schema.md`.

---

## 1. Migration File Conventions
Every migration is versioned with a 14-digit UTC timestamp prefix (`YYYYMMDDHHMMSS`) and **must** have both an `.up.sql` and a `.down.sql` counterpart:
```
core-api/migrations/
├── 20261021000001_create_identity_tables.up.sql
├── 20261021000001_create_identity_tables.down.sql
├── ...
├── 20261110000003_create_idempotency_keys.up.sql
└── 20261110000003_create_idempotency_keys.down.sql
```

### Prohibited Constructs in Migrations (Enforced by `sql_guard_test.go`)
- `BEGIN;` / `COMMIT;` (The transactional migrator manages transactions atomically per file).
- `CONCURRENTLY` (Cannot run inside a transaction block in PostgreSQL).

---

## 2. In-Process Migrator (`migrator.go`)
The Go backend embeds all migration SQL files (`//go:embed *.sql`) and provides three core lifecycle functions:

- `RunMigrations(ctx, db)`: Sequentially executes all pending `.up.sql` files in ascending numerical order and records them in the `schema_migrations` ledger.
- `RollbackMigrations(ctx, db)`: Reverses all applied migrations in **descending numerical order** using `.down.sql` files, safely honoring reverse foreign-key constraints.
- `ResetMigrations(ctx, db)`: Executes `RollbackMigrations` followed by `RunMigrations` for clean local or CI resets.

---

## 3. Automated Reversibility & Rollback Testing
Automated testing in [`migrations_test.go`](migrations_test.go) enforces:
1. **File Sanity & Symmetry:** Every `.up.sql` has a matching `.down.sql`, with zero duplicate timestamps.
2. **Reverse Dependency Ordering:** Down migrations cleanly drop dependent child tables before parent tables (e.g. `parcels` before `merchants` and `pricing`).
3. **Live Up $\rightarrow$ Down $\rightarrow$ Up Lifecycle:** `TestMigrations_LiveLifecycle_UpDownUp` verifies against a live PostgreSQL instance that:
   - All migrations apply (`Up`).
   - All migrations roll back completely leaving 0 application tables (`Down`).
   - All migrations re-apply cleanly from scratch (`Up`).

---

## 4. Migration Catalog & Table Schemas

### `20261120000001_create_user_notifications`
- **Direction:** `20261120000001_create_user_notifications.up.sql` / `20261120000001_create_user_notifications.down.sql`
- **Domain Scope:** User In-App Notifications & Event Fanout
- **Table:** `user_notifications`

#### Schema Definition
| Column | Type | Constraints / Defaults | Description |
| :--- | :--- | :--- | :--- |
| `id` | `UUID` | `PRIMARY KEY DEFAULT gen_random_uuid()` | Unique notification identifier |
| `user_id` | `TEXT` | `NOT NULL` | Target recipient user ID |
| `tenant_id` | `TEXT` | `NULL` | Tenant/Merchant scope identifier (optional) |
| `event_type` | `TEXT` | `NOT NULL` | Originating domain event type (e.g. `parcel.booked.v1`) |
| `aggregate_id` | `TEXT` | `NOT NULL` | ID of the source aggregate root |
| `title` | `TEXT` | `NOT NULL` | Notification headline |
| `body` | `TEXT` | `NOT NULL` | Full notification message body |
| `metadata` | `JSONB` | `NOT NULL DEFAULT '{}'` | Additional payload context & attributes |
| `is_read` | `BOOLEAN` | `NOT NULL DEFAULT FALSE` | Read status flag |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT now()` | Event receipt & record creation timestamp |
| `read_at` | `TIMESTAMPTZ` | `NULL` | Timestamp when user marked notification as read |

#### Indexes
- `idx_user_notifications_user_unread ON user_notifications (user_id, is_read, created_at DESC)`: Optimizes user inbox feed queries for unread notifications and pagination.
- `idx_user_notifications_tenant ON user_notifications (tenant_id, created_at DESC) WHERE tenant_id IS NOT NULL`: Partial index for tenant-scoped notification queries and filtering.
- `idx_user_notifications_created_at ON user_notifications (created_at)`: Supports retention cleanup, archival, and time-range filtering.

