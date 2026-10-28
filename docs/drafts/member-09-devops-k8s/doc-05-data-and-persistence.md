# Document 5 — Data & Persistence Specification

> **Status:** Draft / Proposed (For Review in Team Meeting 2)  
> **Author:** Member 9 — DevOps, Kubernetes & Observability + Database Coordinator (Minidu Dhananjana)  
> **Target Module / Location:** `deploy/` (Docker Compose, Kubernetes, Helm, monitoring), `.github/` (CI/CD), root `Makefile`; database coordination of `core-api/migrations/`  
> **Key Upstream Dependencies:** every service owner (Dockerfile, health endpoints, env vars), Member 7 (gateway routes), Member 8 (queues, reconciler), Member 2 (seed, migrations)  
> **Open Questions / Needs Clarification:** Are cross-module foreign keys allowed (DB1)?; outbox `correlation_id` and the `processed_events` table in `database-schema.md` (DB2, DB3, Member 8); ai_conversations vs the AI no-database rule (DB5, Member 10); idempotency_keys owner and key (DB7, Member 2); `parcel_addresses` table vs JSONB (DB13, Member 2); rename of the webhook `secret_hash` column (DB11). *(Settled: refresh-token table exists (DB4); pool sizes (D8); migration order (D3).)*  
> **Answers:** What data does this module own, and how is consistency maintained?  
> **System ERD:** not duplicated here; see `docs/architecture/database-schema.md` §1 (the template allows a reference).  

---

## 1. What data this module owns or runs

M9 owns **no business tables**. It owns the **places data lives**, and the **process** by which the schema changes.

| Data / store | Kind | Owner | Notes |
| :--- | :--- | :--- | :--- |
| PostgreSQL 16 instance (`postgres-0`) | stateful store | **M9** (run, tune, back up) | business tables owned by M1–M8 (§2) |
| `schema_migrations` table | golang-migrate bookkeeping | **M9** | columns `version`, `dirty`; never edit by hand except `migrate force` |
| DB login roles | access control | **M9** (create), M1 (security review) | see §2.2 (DB10) |
| PVC `postgres-data` | 10Gi, `local-path` ✅ | **M9** | `deploy/kubernetes/postgres-statefulset.yml` |
| RabbitMQ data (`rabbitmq-0`) | broker storage, 5Gi PVC `local-path` ✅ | **M9** (run) / M8 (topology) | transport only, never the source of truth (SRS §2.1) |
| Redis *(optional)* | Pub/Sub fan-out | M9 (run) / M5 (use) | no persistence; not a cache of truth (Engineering Standards §2.7) |
| MinIO / object storage | proof-of-delivery files | M9 (run) / M3 (use) | Compose locally; K8s choice TBD (C26) |
| PostgreSQL backups | encrypted `pg_dump` files | **M9** | off-site (ADR-002 §9); contain PII, so protect them |
| Prometheus data / Grafana dashboards | metrics / JSON in git | **M9** | dashboards as code in `deploy/monitoring/` |

### Data dictionary (platform-owned items)

| Item | Field | Type | Meaning |
| :--- | :--- | :--- | :--- |
| `schema_migrations` | `version` | bigint | last applied migration number |
| `schema_migrations` | `dirty` | boolean | `true` means the last migration failed half-way; nothing else runs until it's fixed |
| Backup file | name | text | `swiftdrop_<YYYYMMDD_HHMMSS>.dump` (runbook §3.1 style) |
| Migration log row | No., file, owner, tables, change, PR, review date, applied (local / k3d / VPS) | — | my record of who changed what (kept by the DB coordinator) |

---

## 2. Table ownership and permission boundaries

### 2.1 Ownership (verified against `database-schema.md` §2)

| Table(s) | Owner | Module path |
| :--- | :--- | :--- |
| `users`, `roles`, `user_roles`, `oauth_identities` | M1 Umesh | `core-api/internal/identity` |
| `pricing_rules`, `pricing_quotes` | M2 Hansi | `core-api/internal/pricing` |
| `parcels`, `parcel_addresses`, `parcel_status_history` | M2 Hansi | `core-api/internal/parcels` |
| `couriers`, `courier_assignments` | M3 Kalani | `core-api/internal/dispatch` |
| `delivery_attempts` | M3 Kalani | `core-api/internal/delivery` |
| `merchants`, `merchant_api_credentials`, `webhook_endpoints` | M4 Nagitha | `core-api/internal/merchants` |
| `delivery_zones`, `audit_logs` | M7 Bagya / M1 Umesh | `core-api/internal/admin`, audit middleware |
| `outbox_events`, `notification_deliveries`, `webhook_deliveries` (+ `processed_events`, DB3) | M8 Watti | `core-api/internal/events`, `services/messaging` |
| `idempotency_keys` | **TBD (DB7)**, proposed M2 Hansi | platform middleware |
| `ai_conversations` | M10 Kakkaya | **on hold (DB5)**: the AI service has no DB access |

**Boundary rule:** a module may **read and write only its own tables**. Cross-module data goes through Go interfaces or events (`domain-boundaries.md` §4). DB-level foreign keys across modules are allowed per SRS §7.2; cross-module JOIN queries are not (DB1, to confirm).

### 2.2 Database roles (agreed, D6)

Core API is **one process** (modular monolith), so all its modules share one role. Module-level ownership is enforced by **code review and the migration review**, not by the database. Each role has its own URL key in `postgres-secrets`, mapped into the pod's `DATABASE_URL` (A5 amendment, Doc 4 §4).

| Role | Used by | Permissions |
| :--- | :--- | :--- |
| `swiftdrop_migrator` | migrate Job / Compose service only | owns the schema; DDL (CREATE/ALTER/DROP) |
| `swiftdrop_reconciler` | reconciler CronJob (`messaging` image, Member 8) | SELECT on core consistency views; UPDATE on `outbox_events`, `webhook_deliveries`, `notification_deliveries` |
| `swiftdrop_app` | core-api | SELECT/INSERT/UPDATE on core tables; DELETE only where a module needs it |
| `swiftdrop_tracking_ro` | tracking-api | **SELECT only** on `parcels`, `parcel_status_history`, `parcel_addresses` (read-only projection query) |
| `swiftdrop_relay` | outbox-relay | SELECT/UPDATE on `outbox_events` |
| `swiftdrop_worker` | event-worker | INSERT/UPDATE on `notification_deliveries`, `webhook_deliveries`, `processed_events`; SELECT on the `merchant_webhook_targets` view (Member 4's name, DB12) |
| *(none)* | ai-assistant | **no DB access** (FR-AI-005) |
| `postgres` superuser | M9 admin only | never used by an app |

---

## 3. Migration strategy and ordering

**Tool and run (decided C22):**
- golang-migrate;
- locally, a one-shot Compose service `core-api-migrate`; other services wait with `service_completed_successfully`;
- in K8s, a Job once per deploy (proposed instead of a per-pod initContainer; Doc 3 §8);
- the command is `make migrate-up`.

**Rules (DB coordinator):**
1. Every change is a file pair in `core-api/migrations/`: `NNNNNN_name.up.sql` + `.down.sql` (AGENTS.md §7). No manual changes.
2. Create files with `migrate create -ext sql -dir core-api/migrations -seq <name>`, then check the newest number on `main` and rename if there's a clash. CI rejects duplicate numbers.
3. **Never edit a merged migration**; write a new one.
4. Only change your own tables. In the same PR, update `database-schema.md` and your module README's "Data Ownership" section.
5. A foreign key to another member's table means tagging them as a reviewer.
6. **Backward compatible (expand → contract):** add new columns or tables first (nullable or with defaults), deploy code that uses them, and remove old ones only in a later release, because old pods still run during a rolling update.
7. Keep each file small and focused. If one fails, golang-migrate marks the DB `dirty`. To fix it:
   - correct the SQL;
   - run `migrate -path core-api/migrations -database "$DATABASE_URL" force <last good version>`;
   - run `up` again (`local-development.md` §5).

**First-wave order (D3: the hub list in `docs/drafts/README.md` §4, reserved numbers):**

| No. | File | Owner | Tables | Needs (foreign keys) |
| :-: | :--- | :--- | :--- | :--- |
| 000001 | `create_users_and_roles` | M1 Umesh | `users`, `roles`, `user_roles`, `oauth_identities` (+ `refresh_tokens`, Member 1 to confirm) | — |
| 000002 | `create_delivery_zones` | M7 Bagya | `delivery_zones` | — |
| 000003 | `create_merchants_and_credentials` | M4 Nagitha | `merchants`, `merchant_api_credentials`, `webhook_endpoints` | 000001 |
| 000004 | `create_pricing_rules` | M2 Hansi | `pricing_rules` | 000002 |
| 000005 | `create_parcels_and_quotes` | M2 Hansi | `parcels`, `parcel_addresses` (or JSONB, DB13), `pricing_quotes`, `parcel_status_history` | 000001–000004 |
| 000006 | `create_couriers` | M3 Kalani | `couriers` | 000001, 000002 |
| 000007 | `create_dispatch_and_delivery` | M3 Kalani | `courier_assignments`, `delivery_attempts` | 000005, 000006 |
| 000008 | `create_outbox_events` | M8 Watti | `outbox_events` | — |
| 000009 | `create_worker_dedup_and_webhooks` | M8 Watti | `processed_events`, `webhook_deliveries`, **`notification_deliveries`** (missing from the hub list) | 000003, 000008 |
| 000010 | `create_idempotency_keys` | M2 / platform (DB7) | `idempotency_keys` | — |
| 000011 | `create_audit_logs` | M7 / platform | `audit_logs` | 000001 |

`pricing_quotes.accepted_parcel_id` points at `parcels`, so quotes must live in `000005` or later, never before parcels. After the first wave, every change takes the next free number. The seed runs after all migrations.

---

## 4. Transaction boundaries, isolation and concurrency

| Topic | Rule | Owner |
| :--- | :--- | :--- |
| Business transaction | parcel update + `parcel_status_history` + `outbox_events` in **one** transaction | M2 / M8 (AGENTS.md §4.3) |
| Isolation level | PostgreSQL default **READ COMMITTED**, plus **optimistic locking** (`version` column, `WHERE version = $n`, 0 rows means 409) | domain owners (`database-schema.md` §3.3) |
| Concurrent migrations | golang-migrate takes an advisory lock, so a second run waits. Only one Job runs per deploy | M9 |
| Relay claiming rows | single relay (`replicas: 1`); `FOR UPDATE SKIP LOCKED` in the sequence diagrams | M8 |
| Long transactions | no external HTTP calls inside a DB transaction (SRS §10.2); a runbook query finds `idle in transaction` sessions | all; M9 monitors |
| Restore | writers scaled to 0 during the restore | M9 |

## 5. Connection budget (owned by M9, decided C11)

**Rule:** `Σ (max replicas × pool size) + admin/migration headroom ≤ max_connections` (**150**: raised from the default 100 by the DB coordinator on 2026-10-25; Postgres memory limit 1Gi).

| Service | Max replicas | Pool per pod (`DB_MAX_OPEN_CONNS`) | Total | Owner confirms |
| :--- | :-: | :-: | :-: | :--- |
| core-api | 4 | 12 *(example)* | 48 | Hansi |
| tracking-api | TBD (C8), e.g. 4 | 5 | 20 | Senuji |
| outbox-relay | 1 | **5** (idle 2) | 5 | ✅ Watti |
| event-worker | 3 (KEDA) | **10** (idle 5) | 30 | ✅ Watti |
| reconciler | 1 | 2 *(to confirm)* | 2 | Watti |
| migrations + admin + spare | — | — | ~45 | Minidu |
| **Total** | | | **150** (105 used) | |

`environment-variables.md` now defaults `DB_MAX_OPEN_CONNS` to 12 (the core-api value), so the smaller per-service values above **must** still go into each ConfigMap. If more capacity is needed, raise `max_connections` and the Postgres memory together. **Done on 2026-10-25: 150 connections, 1Gi**, so Member 8's pools fit.

## 6. Data validation and integrity rules (checked in every migration review)

These come from SRS §7.2, `database-schema.md` §3 and the DB coordinator review checklist:

- `uuid` primary keys. `roles` and `delivery_zones` are text codes (DB8).
- `FOREIGN KEY ... ON DELETE RESTRICT`; `CASCADE` only for tightly bound children.
- `NOT NULL`, `CHECK` (status values), `UNIQUE` (e.g. `tracking_number`, `email`, `key_prefix`).
- Money as **`bigint` cents** in LKR plus a `currency` column; never float (DB9).
- `timestamptz` everywhere, written in UTC.
- `version` column on aggregates (`parcels`, `merchants`).
- The indexes in `database-schema.md` §4 (tracking number, sender/merchant lists, status, history, active assignments, outbox pending, webhook retry, idempotency lookup).

## 7. Retention, deletion, audit and privacy

| Data | Retention | Source |
| :--- | :--- | :--- |
| Audit logs, parcel status history | ≥ 2 years | `database-schema.md` §5 |
| Proof-of-delivery files | 90 days after delivery; pre-signed URLs expire in 15 min | `database-schema.md` §5 |
| Published outbox events | deleted 7 days after `PUBLISHED` | `database-schema.md` §5 |
| Webhook delivery logs | 30 days | `database-schema.md` §5 |
| PostgreSQL backups | **TBD** (proposal: keep the last 7 daily + 4 weekly), encrypted, off-site | ADR-002 §9 |
| Prometheus metrics | **TBD** (proposal: 7–15 days, sized to the VPS disk) | — |

**Privacy rules for the platform:**
- Backups contain **PII**: encrypt them, keep them off-site, never commit them, and never copy them to personal laptops.
- SRS §7.3: *"Secrets and proof files shall not be included in database dumps."* See **DB11** below.
- Logs and metrics contain no PII or secrets (SEC-008; SRS §11.4 label rule).
- Demo and test data are fake only (Engineering Standards §6).

**Audit of schema changes:** the migration log, git history, PR reviews and CODEOWNERS (DB coordinator review gate).

### New schema problem found while writing this doc

| # | Problem | Where | Recommendation | Owner |
| :-: | :--- | :--- | :--- | :--- |
| **DB11** | **Webhook signing secret storage.** `webhook_endpoints.secret_hash` is described as the *"HMAC signing secret"*, but HMAC signing needs the **real secret**, not a hash. If the real secret sits in the DB, it ends up in every `pg_dump`, which breaks SRS §7.3. SRS §7.1 says the table holds a *"secret reference"* | `database-schema.md`; `webhook-security.md` (no storage rule) | Store the secret **encrypted** (AES-GCM; security-notes SEC-007 already names AES-GCM-256) with the key in a K8s Secret, or store only a reference. Rename the column (e.g. `secret_ciphertext`) | Nagitha (M4) + Watti (M8); Minidu reviews |

---

## 8. Caching policy

| Layer | Caching | Freshness |
| :--- | :--- | :--- |
| PostgreSQL | none added; it is the source of truth | — |
| Redis | **Pub/Sub fan-out only**, no cached business data | clients re-fetch after reconnect (FR-TRK-005) |
| Traefik | no response caching | — |
| Cloudflare | static web assets only (hashed files `max-age=31536000`, `index.html` not cached; Member 9 proposal) | new deploy = new hashed filenames |
| API responses | not cached at the edge | — |

## 9. Seed data and test data

| Need | Plan |
| :--- | :--- |
| Local demo data | `make seed` loads deterministic users, zones, tariffs, couriers and parcels (command from M2 Hansi). Demo accounts are listed in `local-development.md` §2 and are **local and demo only** |
| CI tests | a **fresh Postgres per CI run** (GitHub Actions service container) → `migrate up` → `down` → `up` → tests. No shared test database |
| Integration tests | Testcontainers or Compose (SRS §12.1) |
| Rules | fake data only; no real phone numbers or addresses; repeatable (Engineering Standards §12.2) |
| VPS demo | separate demo passwords from a K8s Secret, never the local defaults |

## 10. Summary of open data items

DB1 (FK rule), DB2 (outbox `correlation_id`), DB3 (`processed_events`), DB5 (`ai_conversations`), DB6 (references to two tables), DB7 (idempotency owner and key), DB8 (text primary keys), DB9 (money type), **DB11 (webhook secret column)**, DB12 (webhook targets view name), DB13 (`parcel_addresses` vs JSONB). Resolved: DB4 (`refresh_tokens` exists), DB10 (roles agreed in D6). Each one is listed in this folder's README under "Open questions for Meeting 2"; DB11 is above.
