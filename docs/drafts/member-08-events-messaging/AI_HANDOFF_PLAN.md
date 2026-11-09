# Member 08 — AI Hand-off Plan (THE single file the coding AI reads and follows)

> **This file is complete and self-contained.** It covers Member 8's work (events/outbox, messaging service, Swagger) **and** the Core API foundation work that no other member owns (§2.1, Phases 1B and 8B). It supersedes `MASTER_IMPLEMENTATION_PLAN.md` (now a pointer) and the ordering/decisions in `IMPLEMENTATION_PLAN.md` (background only). **Where anything conflicts with this file, this file wins.**
> **Obey:** `AGENTS.md` and `CLAUDE.md` at repo root. If anything is ambiguous and not decided in §3, **stop and ask**; do not guess.
> **Verified repo state (2026-11-01):** `services/messaging/` contains only `README.md` (empty `cmd/` and `internal/` directories, no Go module). `core-api/internal/events/` contains only `README.md`. `core-api` is Go 1.22, stdlib `net/http` + `database/sql` + `github.com/lib/pq` only. Migrations are timestamp-named and applied by `core-api/migrations/migrator.go` (embeds `*.sql`, sorts by filename, version = text before first `_`, runs only `*.up.sql`). No OpenAPI/Swagger exists anywhere yet.

---

## 1. Can Member 8 finish without waiting for others?

**Short answer: yes for ~95% — build and prove everything alone. Only a few items need other members, and each has a stand-in.**

| Dependency | Owner | Needed for | Blocks you? | Stand-in you use (no waiting) | Real integration needed later |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Real producers calling `OutboxWriter.Write` in booking/assign/deliver transactions | M2, M3, M4 | Real events in the system | **No** | `services/messaging/cmd/devemit` inserts valid outbox rows (§9 Phase 3); the Phase 9 end-to-end test uses a Core-API-style transaction | M2/M3/M4 adopt your library (you publish it first, Phase 1) |
| `webhook_endpoints` table + `merchant_webhook_targets` view | M4 | Webhook fan-out, `webhook_deliveries` FK | **No** | Test-fixture copy of M4's draft DDL (`docs/drafts/member-04-merchants/migrations-draft/20261101000100…000400`) loaded **only** by tests/dev scripts in `services/messaging/tests/fixtures/` | M4 merges real migrations before yours (timestamps already ordered, §6) |
| `parcel`'s `notify` block (`recipient_email`) and `merchant_id` in payloads | M2/M3 | Email + webhook routing | **No** | Payload fixtures in `services/messaging/tests/fixtures/events/*.json` (copied from `doc-04` §3.2) | Change requests CR-M02-01 / CR-M03-02 (already drafted in `CHANGE_REQUESTS.md`) |
| JWT auth middleware (`security.Authenticate`, `RequireRole`, `RequireTenant`) | M1 | Ops/admin API (Phase 7) | **No — already in repo** (`core-api/internal/identity/transport`) | use it as-is | **Verified problem:** the seeded merchant `TenantID` is `mrc_01HMERCHANTTENANT00001` (a string), but `webhook_endpoints.merchant_id` is a UUID — see D25 |
| DB roles (`swiftdrop_relay`, `swiftdrop_worker`, `swiftdrop_reconciler`), K8s manifests, KEDA, CronJob, image registry | M9 | Production deploy | **No for code**; yes for deploy | local compose uses a superuser; guarded `GRANT`s in migrations | Change requests CR-M09-01 + the mismatches in D14/D15 |
| `RABBITMQ`/Redis for Tracking | M5 | Tracking updates | **No** | you declare `q.tracking.updates`; M5 consumes | M5 binds/consumes |
| Meeting-2 sign-off on open decisions (event name, DEC-04, SMTP deviation) | Tech Lead | Final naming | **No** | defaults in §3 | Rename = constants in `catalog.go` only |

**What genuinely must wait:** (1) the final live demo with real booking/dispatch events (needs M2/M3 to call your writer), and (2) deploying on Kubernetes (needs M9). Everything else is testable locally with Docker Compose.

## 2. Scope (what you build) and out-of-scope (never touch)

### 2.1 Core API foundation — unowned items you take over
Searched all member drafts (2026-11-01): **nobody's workload contains** these, and M4 explicitly leaves "platform owner" blank (M4 `doc-03` U-20). They are yours; each is built in Phase 1B or 8B:
`internal/platform/{db,httpx,middleware,config}` · request-ID/correlation middleware (M1's CR-M01-01 only *proposes* it) · DB pool limits rule (`Replicas_max × MaxOpenConns ≤ PostgreSQL MaxConnections`) · graceful shutdown of the Core API (20s) · `/health/live` + `/health/ready` for Core API · Core API config parsing · Swagger registry/index for **all** modules · `core-api/tests/` cross-module harness · migration sanity tests · `.golangci.yml` boundary rules for events/AMQP · `scripts/verify.sh` + `.github/workflows/ci.yml` coverage of `services/messaging`.
**Not yours (do not do):** identity/auth logic, `cmd/seed` accounts (M1), parcel/dispatch/merchant code, admin module, `deploy/` manifests, web frontends.
**Shared files you may touch with *minimal, additive* edits only, and must announce in `CHANGE_REQUESTS.md` (§"Shared-file edits")**: `core-api/cmd/api/main.go`, `core-api/cmd/api/wiring.go` (M1 wrote them), `.golangci.yml`, `scripts/verify.sh`, `scripts/verify.ps1`, `.github/workflows/ci.yml` (M9). Never delete or reformat existing lines; never change existing routes, behaviour or defaults.

**Build:**
1. `core-api/internal/events/` — envelope, `OutboxWriter`, Postgres writer, catalog, fake writer, **ops/admin HTTP API + Swagger** (Phase 7).
2. Migrations (timestamped, §6).
3. `services/messaging/` — Go module with 3 binaries (`relay`, `worker`, `reconcile`) + dev tools (`devemit`, `fake-receiver`), one Docker image.
4. Swagger/OpenAPI specs + docs other members use (§10).
5. Docs: both READMEs, `API.md`, runbook, `CHANGE_REQUESTS.md` additions, seed data, test docs, demo script, report section check (§11).

**Never edit:** other members' modules (`parcels`, `dispatch`, `delivery`, `merchants`, `identity`, `admin`, `pricing`), `docs/contracts/`, `docs/architecture/`, `deploy/`, `web/`, `services/tracking`, `services/ai-assistant`. Propose changes in `CHANGE_REQUESTS.md` only. Do **not** query `parcels`, `parcel_addresses`, `couriers`, or any table you don't own (only the `merchant_webhook_targets` view is allowed).

## 3. Decisions (binding — resolve every conflict found in the specs)

| # | Conflict / gap | **Decision** |
| :--- | :--- | :--- |
| D1 | Envelope names differ (`event-contracts.md`: `version`, `timestamp`, `evt_…`; `doc-04`/DDL: `aggregate_version`, `schema_version`, `occurred_at`, UUID). | Use DDL/`doc-04`. JSON fields: `event_id, event_type, aggregate_type, aggregate_id, aggregate_version, schema_version, occurred_at, correlation_id, producer, payload`. `event_id` = UUID (v7 preferred, any valid UUID accepted). `GetSchemaVersion() string`. File CR for Tech Lead (CR-AUTH-01). |
| D2 | `parcel.failed.v1` vs `parcel.delivery_failed.v1`. | `parcel.delivery_failed.v1`. Constants only in `catalog.go`. |
| D3 | Secret env var name. | `WEBHOOK_SECRET_ENCRYPTION_KEY` = 64 hex chars (32 bytes) for key version 1. Optional `WEBHOOK_SECRET_ENCRYPTION_KEY_V2`… for rotation; select by `secret_key_version`. Missing key for a version ⇒ delivery `FAILED` (`KEY_UNAVAILABLE`), process still starts if V1 valid. |
| D4 | View columns. | Code against M4's real view: `endpoint_id, merchant_id, url, secret_ciphertext, secret_key_version, subscribed_event_types, updated_at`. |
| D5 | Status vocab. | Deliveries: `PENDING, IN_PROGRESS, SUCCESS, FAILED`. Outbox: `PENDING, PUBLISHED, FAILED` (no `IN_PROGRESS`). Never `SENT`. |
| D6 | Roles. | Migrations contain **only guarded `GRANT`s** (`IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname=…)`), never `CREATE ROLE`. Local compose uses the `postgres` superuser. `RECONCILER_DATABASE_URL` optional; falls back to `DATABASE_URL`. |
| D7 | DB name. | `swiftdrop_db` in all M8 docs/examples. Do **not** edit `core-api/cmd/api/wiring.go` default (`swiftdrop`, M1's file); document `DATABASE_URL` override. |
| D8 | Writer name. | `Write(ctx, tx, ...AnyEnvelope)` only. No aliases. |
| D9 | FR-REC-003 needs M2/M3 tables. | Out: document as partial; stub interface `ConsistencyChecker` returning "not configured". No queries on `parcels`. |
| D10 | `notify`/`merchant_id` may be absent. | Optional in parsing. No `notify.recipient_email` ⇒ insert notification row `status=FAILED, last_error='NO_RECIPIENT', recipient='-'`; no `merchant_id` ⇒ ack and skip webhooks (log + counter). |
| **D11** | **Gap:** delivery rows in the DDL don't store the event data, but retries (up to ~42 min) and the 7-day outbox purge mean the poller can't rely on the outbox row. | Extend M8's own tables in the migration (this is M8's DDL): `notification_deliveries` += `event_type VARCHAR(128) NOT NULL`, `aggregate_id UUID NOT NULL`, `aggregate_version INT NOT NULL`, `payload JSONB NOT NULL` (sanitized envelope payload incl. `notify`). `webhook_deliveries` += `event_type`, `aggregate_id`, `aggregate_version`, `occurred_at TIMESTAMPTZ NOT NULL`, `payload JSONB NOT NULL` (**`notify` stripped before storing**). Worker never reads `outbox_events`. Update `doc-05` to match. |
| **D12** | Spec text asks for an aggregate-version "drop older event" guard but no storage exists. | **Do not implement the drop guard.** Webhook/email order is best-effort; receivers order by `aggregate_version` in the payload. Document this limitation. |
| **D13** | DLX type: `doc-01` says Direct, earlier plan says fanout. | **Fanout** `swiftdrop.dlq.exchange` (durable) bound to `swiftdrop.dlq` (works regardless of routing key). Record in README; update `doc-01` §7.3. |
| **D14** | `deploy/kubernetes/autoscaling/keda-event-worker.yml` scales on queue `parcel.events`, which doesn't exist in your topology. | Use your queue names (`q.notifications.worker`, `q.webhooks.dispatcher`). File CR-M09 (do not edit `deploy/`). |
| **D15** | `deploy/kubernetes/cronjob-reconciliation.yml` runs `swiftdrop/core-api:latest … reconcile --outbox-threshold --delivery-timeout`. Your CLI lives in the `messaging` image with different flags. | Implement flags `--stale-threshold` (alias `--outbox-threshold`) and `--delivery-timeout` (accepted; used for stale `IN_PROGRESS`/overdue delivery reporting). File CR-M09 to point the CronJob at the messaging image `/usr/local/bin/reconcile`. |
| **D16** | Webhook timeout: `doc-01`/`doc-04` say 5s, earlier plan 10s. Private-host flag names differ. | Timeout = `WEBHOOK_HTTP_TIMEOUT_SECONDS` default **5**. Dev flag = `WEBHOOK_ALLOW_PRIVATE_HOSTS` (default false). Plain `http://` only when `APP_ENV=local` **or** that flag is true. |
| **D17** | SMTP config names differ. | Use `doc-01` §7.4: `SMTP_HOST, SMTP_PORT, SMTP_SECURITY (starttls|tls|none), SMTP_USERNAME, SMTP_PASSWORD, SMTP_FROM_EMAIL, SMTP_FROM_NAME`. `none` allowed only when `APP_ENV=local`. Fail-closed: `starttls` with no TLS support ⇒ error. |
| **D18** | Webhook headers: earlier plan omitted `X-SwiftDrop-Timestamp`; `webhook-security.md` requires it. | Send all of: `Content-Type: application/json; charset=utf-8`, `User-Agent: SwiftDrop-Webhook-Delivery/1.0`, `X-SwiftDrop-Delivery-ID` (delivery row UUID), `X-SwiftDrop-Event-ID`, `X-SwiftDrop-Timestamp: <unix>`, `X-SwiftDrop-Signature: t=<unix>,v1=<hex>`. Signed string = `"{t}.{raw body}"`, HMAC-SHA256, key = decrypted endpoint secret. |
| **D19** | `subscribed_event_types` format unknown (webhook-security.md uses `parcel.created`; events are `parcel.booked.v1`). | Match **exact** `event_type` string (`= ANY(subscribed_event_types)`). Raise CR-M04 to confirm values are full event types. |
| **D20** | Outbox relay locking vs "never hold tx across broker call". | Claim in a short tx: `SELECT … WHERE status='PENDING' AND (next_attempt_at IS NULL OR next_attempt_at<=now()) ORDER BY created_at LIMIT $batch FOR UPDATE SKIP LOCKED`, then `UPDATE … SET next_attempt_at = now() + lease (default 60s)` and commit (acts as a visibility lease; `attempt_count` NOT incremented). Publish with confirms outside any tx. Success ⇒ `status='PUBLISHED', published_at=now()`. Failure ⇒ `attempt_count+1, last_error, next_attempt_at = now()+backoff(1s,2s,4s,8s,16s ±20%)`; when `attempt_count` reaches 5 ⇒ `FAILED`. Crash ⇒ row reappears after lease with no attempt consumed (duplicate publish possible: documented, consumers dedupe). |
| **D21** | Which events email whom. | Templates: `parcel.booked`, `parcel.out_for_delivery`, `parcel.delivered`, `parcel.delivery_failed`, `parcel.returned`. All other event types: no customer template ⇒ ack, count `swiftdrop_notifications_skipped_total`. |
| **D22** | Test infra. | No testcontainers. Integration tests run against `services/messaging/docker-compose.dev.yml` and are skipped unless `INTEGRATION=1`. Unit tests need no services. |
| **D23** | Dependencies. | `core-api`: **no new deps**. `services/messaging` (new module `github.com/swiftdrop/messaging`, Go 1.22): only `github.com/lib/pq`, `github.com/rabbitmq/amqp091-go`, `github.com/prometheus/client_golang`. Everything else stdlib (`log/slog`, `net/smtp`, `crypto/*`). |
| **D24** | Swagger UI assets without a Go dependency. | Serve an embedded HTML page loading `swagger-ui-dist` from a **version-pinned jsDelivr URL with SRI hashes** (compute the hash at implementation time and record it). Gate with `SWAGGER_ENABLED` (default `true` when `APP_ENV=local`, else `false`). |
| **D25** | Merchant tenancy. **Verified:** `main.go` seeds the merchant with `TenantID = mrc_01HMERCHANTTENANT00001` (not a UUID); `webhook_endpoints.merchant_id` / `merchants.id` are UUID. | Implement merchant endpoints through a `TenantResolver` that parses `Principal.TenantID` as a UUID; if it isn't one return `403` with error code `TENANT_NOT_MERCHANT_UUID`. Admin endpoints do not depend on this. File CR-M01/CR-M04: "JWT tenant claim must be `merchants.id` (UUID)". Test both a UUID tenant (works) and the seeded `mrc_…` tenant (403). Do not change M1's seed. |
| **D26** | Transaction convention: M4 U-20 proposes a `Querier` parameter; M2/M3 agreements and `doc-04` fix `Write(ctx, tx *sql.Tx, …)`. | Keep `*sql.Tx` on `OutboxWriter.Write` (D8). In `platform/db` also provide `type Querier interface{ExecContext; QueryContext; QueryRowContext}` (satisfied by `*sql.DB` and `*sql.Tx`) and `WithTx(ctx, db, func(*sql.Tx) error) error` (begin, run, rollback on error/panic, commit). Document both; neither forces the other. |
| **D27** | Wiring location: `AGENTS.md`/M4 say `internal/app/wiring.go`; M1 and the real code use `cmd/api/wiring.go`. | Keep `cmd/api/wiring.go` as the composition root. Add events/platform wiring there via small additive helper functions (e.g. `registerEventsRoutes(mux, deps)` in a new file `cmd/api/wiring_events.go`), never by rewriting `BuildAppContainer`. |
| **D28** | **Migrator wraps each file in its own transaction** (`migrator.go`: `BeginTx` → `ExecContext(file)` → ledger insert → `Commit`). | **Migration files must NOT contain `BEGIN;`/`COMMIT;`** (the spec DDL in `doc-05` §4 has them — strip them). Do not use `CREATE INDEX CONCURRENTLY`. Do not edit `migrator.go` behaviour; add tests only (Phase 8B). |
| **D29** | `scripts/verify.sh` and CI only run `core-api` tests. `verify.sh` secret scan flags `password = "literal"` anywhere outside `docs/` and `scripts/`. | Extend verify.sh/ps1 and `ci.yml` to also run `services/messaging` (`gofmt`, `go vet`, lint, `go test -race`). In code/tests/compose never write the literal pattern `password = "…"`; read secrets from env or build strings without that shape. |
| **D30** | `.golangci.yml` `depguard` only denies AMQP inside `domain/`. | Add rule `core_api_no_broker`: files `**/core-api/**/*.go` deny `github.com/rabbitmq/amqp091-go` ("Core API must not publish to the broker; use the outbox"). Add `events_no_http_in_domain` not needed (existing rule covers domain). Keep existing rules untouched. |

## 4. Assumptions to state in your plan output (AGENTS.md §3 step 2)
A1 PostgreSQL 16, RabbitMQ 3.12+, Mailpit locally. A2 merchant `TenantID` is currently NOT a UUID (D25: merchant endpoints return 403 until M1/M4 align it). A3 M2/M3 will add `merchant_id` and `notify` to payloads; until then fixtures are used. A4 M4's migrations land before `20261101000500`. A5 Meeting 2 may rename event types; code uses constants only.

## 5. Architecture (do not deviate)

```
Core API txn ──INSERT──► outbox_events ──(Relay: claim/lease, publish+confirm, mark)──► RabbitMQ swiftdrop.events.topic
                                                                                         ├─ q.tracking.updates   (parcel.#, courier.#)  → M5
                                                                                         ├─ q.notifications.worker (parcel.#)           → Worker
                                                                                         ├─ q.webhooks.dispatcher (parcel.#, courier.#, merchant.#) → Worker
                                                                                         └─ unparseable → swiftdrop.dlq.exchange (fanout) → swiftdrop.dlq (TTL 30d)
Worker consume: parse → dedupe → INSERT delivery rows (ON CONFLICT DO NOTHING) → ack
Worker poller:  claim due rows (SKIP LOCKED, lease) → external call OUTSIDE tx → record outcome (+processed_events) 
Reconciler (CronJob): report stale outbox, reset expired leases, retention purge, DLQ replay
```

Rules (non-negotiable):
1. `OutboxWriter.Write` uses the caller's `*sql.Tx`; never begins/commits; core-api never imports AMQP.
2. Relay is a singleton (`replicas: 1`); never add autoscaling.
3. **Never hold a DB transaction across a broker, SMTP or HTTP call.**
4. At-least-once + idempotent effects; never claim exactly-once.
5. Retries live in PostgreSQL rows, never `Nack(requeue=true)` loops. Parse failures → DLQ (`Nack(requeue=false)` with DLX configured on the queues).
6. Retry schedule: 5 attempts at 0s, 30s, 2m, 10m, 30m, each ±20% jitter; configurable via `WEBHOOK_RETRY_SCHEDULE` (same schedule used for emails). Inject RNG/clock for tests.
7. Webhook outcome: `2xx`=SUCCESS; `408/429/5xx`/timeout/network = retry; other `4xx`, `3xx`, SSRF-blocked, endpoint missing/disabled = terminal FAILED. **Never follow redirects** (`CheckRedirect → http.ErrUseLastResponse`). SMTP: 4xx retry, 5xx terminal.
8. Webhook secret decrypt: AES-256-GCM, bytes = `nonce(12) || ciphertext || tag(16)`, AAD = endpoint_id string, reject `<28` bytes. Read target from `merchant_webhook_targets` on **every attempt** (no cache).
9. SSRF: custom `DialContext`: resolve once, reject if **any** IP is in `0.0.0.0/8, 10/8, 100.64/10, 127/8, 169.254/16, 172.16/12, 192.0.0/24, 192.168/16, 198.18/15, 224/4, 240/4, ::1, fe80::/10, fc00::/7, ff00::/8, 64:ff9b::/96` or IPv4-mapped forms of these; dial the verified IP (keep original host for TLS SNI via `tls.Config.ServerName`).
10. Logging: JSON `slog`; fields `correlation_id`, `event_id`, `delivery_id`. **Never log** secrets, signatures, full URLs (log host only), webhook bodies, or emails (log masked `k***@example.lk`). A test asserts this.
11. Webhook response snippet: first 500 bytes, valid UTF-8, printable only, stored in `VARCHAR(500)`.
12. Health/metrics: `/health/live`, `/health/ready`, `/metrics` on `PROBE_PORT` (8082 relay, 8083 worker). Server starts **before** DB/broker connections; readiness reflects them; never crash-loop during an outage (retry with backoff).
13. Container: multi-stage, pinned Go image (no `latest`), `-trimpath`, `USER 10001:10001`, read-only-rootfs compatible, SIGTERM graceful drain (≤20s), release leases.
14. Metrics (prefix `swiftdrop_`, no high-cardinality labels): `outbox_pending_events`, `outbox_oldest_pending_age_seconds`, `outbox_publish_failures_total`, `outbox_published_total`, `consumer_failures_total{consumer}`, `webhook_delivery_attempts_total{outcome}`, `notification_attempts_total{outcome}`, `notifications_skipped_total`, `dlq_messages_routed_total`, `dlq_depth_gauge`.

## 6. Database (all M8-owned; timestamped migrations)

Files go in `core-api/migrations/` (the migrator embeds them). **Do not use `000008/000009`** (they would sort before the identity migrations and before M4's tables).

| File | Content |
| :--- | :--- |
| `20261101000050_create_outbox_events.up.sql` / `.down.sql` | `doc-05` §4.1 DDL **verbatim** (it already includes `producer`, `occurred_at`, `idx_outbox_pending/aggregate/correlation/retention`) + guarded grants: relay `SELECT, UPDATE`; worker none; core app role `INSERT`; reconciler `SELECT, UPDATE, DELETE`. |
| `20261101000500_create_worker_dedup_and_webhooks.up.sql` / `.down.sql` | `doc-05` §4.3 DDL **plus the D11 columns**, plus guarded grants (worker: `SELECT, INSERT, UPDATE` on the four tables + `SELECT` on `merchant_webhook_targets`; reconciler: `SELECT, UPDATE, DELETE`; core app role `SELECT` on delivery tables for the ops API). Must be applied after M4's `20261101000300`/`000400`. |

- **No `BEGIN;`/`COMMIT;` in the files** (D28): the migrator already wraps each file in a transaction and records it in `schema_migrations`; an inner `COMMIT` would end that transaction early. Keep statements idempotent (`IF NOT EXISTS`). Each `.up.sql` needs a matching `.down.sql` (the migrator only runs `.up.sql`; `.down.sql` is for manual rollback and review).
- If M4's tables are absent when `…0500` runs, the FK fails. Do **not** weaken the FK. Instead the dev/test fixtures (`services/messaging/tests/fixtures/m4_stub/*.sql`) create M4's draft tables first; document in README that M4 must merge first.
- Update `doc-05` §4 (names + D11 columns) so docs == migrations.
- Dev seed (never auto-run in prod): `services/messaging/tests/fixtures/08_messaging_seed.sql` (from `doc-05` §8).

## 7. Phases (each ends green: `gofmt`, `go vet`, `go test -race ./...`, `golangci-lint run ./...` in touched module(s); one commit per phase on a feature branch; never commit to `main`)

### Phase 0 — Inspect and plan (no code)
Read: `AGENTS.md`, `docs/architecture/domain-boundaries.md`, `docs/contracts/event-contracts.md`, `docs/contracts/webhook-security.md`, `docs/contracts/api-conventions.md`, `docs/architecture/database-schema.md`, this folder's `doc-02` (flows, retry matrix, SSRF), `doc-03`, `doc-04` (§2–§5), `doc-05` (§2, §4), `doc-07`, `core-api/migrations/migrator.go` (all), `core-api/cmd/api/wiring.go`, `core-api/internal/identity/transport/*` (middleware + `Principal`), M4's `migrations-draft/*`. Output the one-page plan with assumptions/risks (§4). Apply D1–D30 to M8 docs (this folder only) in small edits.

### Phase 1 — Events library + outbox table  ⟵ **publish to team immediately after**
`core-api/internal/events/`:
- `envelope.go`: `EventEnvelope[T any]` + `AnyEnvelope` (exact interface in `doc-04` §2.1 with `GetSchemaVersion() string`); constructor `NewEnvelope[T](eventType, aggregateType, aggregateID string, aggregateVersion int, correlationID, producer string, payload T) (EventEnvelope[T], error)` generating UUIDv7 `event_id` (hand-rolled, stdlib only) and UTC `occurred_at`; default `schema_version="1.0.0"`.
- `writer.go`: `OutboxWriter` interface (D8).
- `repository.go`: `PostgresOutboxWriter{}`; validates: `tx != nil` (`ErrMissingTransaction`), `event_type` regex `^[a-z_]+\.[a-z_]+\.v[0-9]+$`, non-empty `correlation_id` (≤128), UUID `aggregate_id`, UUID `event_id`, `aggregate_version >= 1`, `aggregate_type` ∈ {parcel, courier, merchant, dispatch}; single multi-row `INSERT … (id, producer, occurred_at, …)` via `tx.ExecContext`.
- `catalog.go`: constants `ParcelBookedV1="parcel.booked.v1"`, `ParcelAssignedV1`, `ParcelPickedUpV1`, `ParcelInTransitV1`, `ParcelOutForDeliveryV1`, `ParcelDeliveredV1`, `ParcelDeliveryFailedV1`, `ParcelReturnedV1`, `ParcelCancelledV1`, `CourierUnassignedV1`, `MerchantStatusChangedV1`, `MerchantApprovedV1`.
- `errors.go`: typed errors (`ErrMissingTransaction`, `ErrInvalidEventType`, `ErrMissingCorrelationID`, `ErrInvalidAggregateID`, …) usable with `errors.Is`.
- `eventstest/memory_writer.go`: `InMemoryWriter` implementing `OutboxWriter` (records envelopes; same validation) so M2/M3/M4 test without Postgres.
- Migration `20261101000050_…`.
- Tests: validation table tests; JSON golden test for envelope; `INTEGRATION=1` test: rollback ⇒ 0 rows, commit ⇒ exactly N rows; multi-envelope atomicity.
- Update `internal/events/README.md` with a copy-paste usage snippet (begin tx → mutate aggregate → `writer.Write(ctx, tx, env)` → commit).
- **Checkpoint 1 message to team** (draft it in `CHANGE_REQUESTS.md` §"Announcements"): import path, interface, constructor, `InMemoryWriter`, catalog constants, `X-Request-ID`→`correlation_id` rule.

### Phase 1B — Core API platform packages (unowned; other modules will depend on these)
Read first: `docs/contracts/api-conventions.md` (envelopes, status codes, pagination, idempotency) and `core-api/internal/identity/transport/{middleware,responses}.go` (don't duplicate; reuse names/shape where sensible). All stdlib; no new deps. No `domain/` dirs. Each package gets a short `doc.go` and tests.
- `core-api/internal/platform/config`: tiny helpers `String(key, def)`, `Int(key, def)`, `Bool`, `Duration`, `Require(key)`; never log values; `AppEnv()` returning `local|test|staging|production`.
- `core-api/internal/platform/db`: `Open(ctx, url, Options)` (`DB_MAX_OPEN_CONNS` default 10, `DB_MAX_IDLE_CONNS` default 5, `DB_CONN_MAX_LIFETIME` default 30m, `DB_CONN_MAX_IDLE_TIME` default 5m; ping with timeout), `Querier`, `WithTx` (D26). Document the pool formula in the package README section and `docs`-style comment: `Replicas_max × MaxOpenConns ≤ PostgreSQL max_connections` (with the numbers 3 replicas × 10 = 30 ≤ 100 as example). **Do not** change identity's existing DB opening code.
- `core-api/internal/platform/middleware`: `RequestID` (read `X-Request-ID`; accept only `^[a-zA-Z0-9_\-]{1,128}$`, else generate UUIDv7; store in context; echo in response header), `CorrelationID(ctx) string` accessor (this is the value producers pass to `EventEnvelope.CorrelationID`), `Recover` (returns the standard 500 envelope, logs without PII, never `panic` out), `AccessLog` (method, path pattern, status, duration, request id; **no** query strings, bodies, tokens or IPs beyond what the gateway provides), `BodyLimit(maxBytes)`.
- `core-api/internal/platform/httpx`: `WriteJSON(w, status, v)`, `WriteError(w, status, code, message, details…)` and `DecodeJSON(w, r, &dst, maxBytes)` (strict: unknown fields rejected, single document) producing the **exact** error envelope from `api-conventions.md` §2.3; paginated list helper matching §2.2; cursor encode/decode (opaque base64url, tamper-safe → `400 INVALID_CURSOR`). Do not import identity packages.
- Wire (additive, D27) in `cmd/api/wiring_events.go`: `RequestID`, `Recover`, `AccessLog` wrapping the final handler — **applied by wrapping `container.Mux` in `main.go` with one added line**; verify existing identity tests still pass unchanged.
- Tests: request-id validation table (valid, too long, illegal chars, empty → generated), middleware ordering, panic → 500 envelope with request id, `WithTx` commit/rollback/panic (sqlmock is a new dep — **don't**; use `INTEGRATION=1` against compose Postgres plus a fake `driver` registered in-test via `database/sql/driver` for unit coverage), pagination cursor tamper test, JSON decode strictness.
- Docs: `core-api/internal/platform/README.md` (purpose, packages, usage snippets, who may import: **all modules may import `platform/*`; `platform` must never import a module**). Add a `depguard` rule `platform_isolation` (files `**/internal/platform/**` deny `github.com/swiftdrop/core-api/internal/identity|parcels|pricing|dispatch|delivery|merchants|admin|events`).
- **Checkpoint 1B message to team** (in `CHANGE_REQUESTS.md` §"Announcements"): "use `platform/middleware.CorrelationID(ctx)` for `correlation_id`; use `platform/httpx` for responses; `platform/db.WithTx` for transactions".

### Phase 2 — Messaging module skeleton + dev stack
- `services/messaging/go.mod` (D23), `internal/config` (strict env parsing, fail fast, names from `doc-01` §7.4 + D3/D16/D17 + `APP_ENV`), `internal/logging` (slog JSON + masking helpers `MaskEmail`, `HostOnly(url)`), `internal/health` (starts first), `internal/metrics`, `internal/broker` (reconnecting connection manager, idempotent topology declare, confirm-mode publisher, manual-ack consumer).
- Topology: exchange `swiftdrop.events.topic` (topic, durable); queues `q.tracking.updates` (`parcel.#`,`courier.#`), `q.notifications.worker` (`parcel.#`), `q.webhooks.dispatcher` (`parcel.#`,`courier.#`,`merchant.#`) each with `x-dead-letter-exchange=swiftdrop.dlq.exchange`; `swiftdrop.dlq.exchange` (fanout, durable) → `swiftdrop.dlq` (`x-message-ttl=2592000000`).
- `services/messaging/docker-compose.dev.yml`: `postgres:16` (db `swiftdrop_db`), `rabbitmq:3.12-management`, `axllent/mailpit` (ports 5432/5672/15672/1025/8025), plus `.env.example` (placeholders only).
- Tests: config parsing/validation (bad key length, bad URL ⇒ error); topology declared twice; publish→consume round trip (`INTEGRATION=1`); reconnect after broker restart.

### Phase 3 — Outbox Relay (+ dev emitter)
- `internal/relay`: poller + publisher per D20. Routing key = `event_type`; AMQP headers `event_id`, `correlation_id`; `delivery_mode=2`; `message_id=event_id`; `content_type=application/json`; body = envelope JSON rebuilt from the row (all 10 fields). Poll interval/batch from `OUTBOX_POLL_INTERVAL_MS` (500) / `OUTBOX_BATCH_SIZE` (50). Handle `basic.return` (unroutable) as failure (use `mandatory=true`).
- `cmd/relay/main.go` (port 8082), metrics updater goroutine (pending count, oldest age).
- `cmd/devemit`: CLI `devemit --type parcel.booked.v1 [--count N] [--merchant-id …] [--email …]` inserts schema-valid outbox rows from `tests/fixtures/events/*.json` (dev only; never in image's entrypoint). This is how you and M5 test without M2/M3.
- Tests (unit with fake broker/db interfaces; integration `INTEGRATION=1`): publish + mark PUBLISHED; nack/unroutable ⇒ attempt_count+1 and backoff honoured; 5 failures ⇒ FAILED; lease prevents two relays double-publishing within the lease; crash-after-publish ⇒ duplicate only (assert consumer dedupes later).
- **Checkpoint 2 message:** queue names, routing keys, message headers, envelope example, compose file, `devemit` usage (for M5/M9/M10).

### Phase 4 — Worker core: consume, idempotency, delivery rows
- `internal/consumer`: generic runner (prefetch = `WORKER_CONCURRENCY`, manual ack, panic-safe), parse+validate envelope (missing required field / bad JSON ⇒ `Nack(false,false)` ⇒ DLQ, increment `dlq_messages_routed_total`, no hot loop).
- `internal/idempotency`: `Seen(consumer, eventID)`, `MarkDone(tx, consumer, eventID)`; consumer names: `notifier`, `webhook:<endpoint_id>`.
- Handlers create rows with `ON CONFLICT DO NOTHING` then ack: notifier handler (D21, D10, D11) and webhook fan-out handler (D4, D10, D11, D19; strip `notify` from stored payload).
- `internal/delivery` (shared): claim query for each table: `UPDATE … SET status='IN_PROGRESS', lease_expires_at=now()+interval, attempt_count=attempt_count+1 WHERE id IN (SELECT id … WHERE status IN ('PENDING','IN_PROGRESS' with expired lease) AND (next_attempt_at IS NULL OR next_attempt_at<=now()) ORDER BY next_attempt_at NULLS FIRST LIMIT $n FOR UPDATE SKIP LOCKED) RETURNING …`; outcome writer (single tx: update delivery row + insert attempt row [webhooks] + `processed_events` on terminal); backoff calculator (`internal/backoff`, injected RNG).
- Tests: 10 duplicate deliveries of one event ⇒ 1 row/effect; worker killed mid-attempt ⇒ lease expires ⇒ exactly one more attempt; malformed message ⇒ DLQ; unique constraints enforced; no `outbox_events` or `parcels` query anywhere (grep-style test over SQL strings).

### Phase 5 — Email notifications
- `internal/notification`: `sender.go` (stdlib `net/smtp`; modes per D17; per-send context deadline; generates `Message-ID: <uuid@swiftdrop>` stored in `provider_message_id`; MIME multipart/alternative; RFC 2047-encoded headers; CRLF-injection-safe — reject any header value with `\r`/`\n`), `templates.go` (embedded `text/template` + `html/template`, 5 templates D21, never include internal IDs beyond tracking number, no PII beyond name), `poller.go`.
- Tests: in-process fake SMTP server (stdlib) for unit; Mailpit (`INTEGRATION=1`, assert via Mailpit REST `GET /api/v1/messages`); 4xx ⇒ retry, 5xx ⇒ FAILED; header-injection test; logs contain no email.

### Phase 6 — Webhook dispatcher
- `internal/webhook`: `client.go` (SSRF dialer D/rule 9, timeouts, no redirects), `decrypt.go`, `signer.go` (D18), `payload.go` (build body — fields `event_id, event_type, aggregate_id, status, occurred_at, data` per `doc-04` §3.3 sanitized webhook payload; `status` from `payload.status`; `data` = stored payload without `notify`, courier phone, notes, raw storage URLs), `dispatcher.go` (poller: claim → load target by `endpoint_id` fresh → classify outcome → record attempt row with `error_class`, `next_attempt_at`).
- `cmd/fake-receiver`: local HTTP sink that verifies `X-SwiftDrop-Signature` (secret via env), logs result, supports `?status=500` style fault injection (dev tool, not in the image).
- Tests: signature **known-answer vector** (fixed secret/timestamp/body ⇒ fixed hex; copy into README for merchants); decrypt wrong-AAD/short/tampered ⇒ error; SSRF table (127.0.0.1, 10.x, 172.16.x, 192.168.x, 169.254.169.254, 100.64.x, `::1`, `::ffff:10.0.0.1`, hostname resolving to private via fake resolver, redirect to metadata IP not followed); outcome matrix 200/204/301/400/404/408/410/429/500/503/timeout/conn-refused; 5 failing attempts ⇒ FAILED with 5 attempt rows; secrets never in logs/snippets.

### Phase 7 — Ops/Admin API in Core API + Swagger (**needs Tech Lead OK; if declined, do only §10 for messaging probes + webhook spec**)
Location: `core-api/internal/events/{application,infrastructure,transport}/` (no `domain/` dir — Anti-Boilerplate). Wire in `cmd/api/wiring.go` with existing `security.Authenticate`, `transport.RequireRole`, `transport.RequireTenant` (copy the pattern of `/api/v1/admin/dashboard` and `/api/v1/merchant/portal`). Keep one exported route slice so the spec-vs-routes test can compare. Read-model SQL only on M8 tables. Never select `secret_ciphertext` or recipient emails (mask email as `k***@domain`).

| Method & path | Role | Behaviour |
| :--- | :--- | :--- |
| `GET /api/v1/admin/events/outbox` | ADMIN | list; filters `status`, `event_type`, `from`, `to`; `limit` 1–100 (default 25), `cursor` (opaque base64 of `created_at,id`) |
| `GET /api/v1/admin/events/outbox/stats` | ADMIN | counts by status, `oldest_pending_age_seconds` |
| `POST /api/v1/admin/events/outbox/{id}/retry` | ADMIN | `FAILED`→`PENDING`, `attempt_count=0`, `next_attempt_at=NULL`; 409 if not FAILED; 404 if missing; writes a structured audit log line (this is an outbox column, not a parcel status) |
| `GET /api/v1/admin/events/deliveries` | ADMIN | unified list (`kind=webhook|notification`, `status`, pagination) |
| `POST /api/v1/admin/events/webhook-deliveries/{id}/retry` | ADMIN | `FAILED`→`PENDING`; 409 otherwise |
| `GET /api/v1/merchant/webhook-deliveries` | MERCHANT (+tenant) | own deliveries only, join on `webhook_endpoints.merchant_id`—**but you may only read via the view**; therefore store nothing extra: filter by `endpoint_id IN (SELECT endpoint_id FROM merchant_webhook_targets WHERE merchant_id=$1)`; disabled endpoints won't appear (document) |
| `GET /api/v1/merchant/webhook-deliveries/{id}/attempts` | MERCHANT (+tenant) | attempts of an owned delivery; foreign id ⇒ **404** (no IDOR) |

Use `platform/httpx` (envelopes, pagination, cursors) and `platform/middleware` from Phase 1B; merchant routes resolve the tenant via D25's `TenantResolver`. Error envelope and pagination style: `docs/contracts/api-conventions.md` (read it; do not invent). Tests: 401/403/404/409/422, pagination bounds, tenant isolation, retry state guards, no secret/email leakage, route-slice vs spec test.

### Phase 8 — Reconciler, DLQ replay, retention
`cmd/reconcile` + `internal/reconciler`: default run = (a) log+count `PENDING` outbox older than `--stale-threshold` (default 5m; alias `--outbox-threshold`) — never republish, relay does that; (b) reset expired leases (`IN_PROGRESS` and `lease_expires_at < now()`) to `PENDING` for both delivery tables; (c) report deliveries `PENDING/IN_PROGRESS` older than `--delivery-timeout`; (d) `ConsistencyChecker` stub (D9). Subcommands: `retention` (outbox PUBLISHED >7d, delivery rows >30d, `processed_events` >35d; batched deletes `LIMIT 1000` under `SET LOCAL statement_timeout='5s'`), `dlq-replay --limit N` (consume from `swiftdrop.dlq`, republish to exchange with header `x-replayed-from=swiftdrop.dlq`, ack only after confirm). One JSON log line per action. Exit non-zero on error, zero when nothing to do. Idempotent, batch-bounded.
Tests: stale detected not republished; leases reset; retention bounded; run-twice no-op; replay round trip.

### Phase 8B — Core API hardening, shared quality gates, cross-module test harness
Shared-file edits are **additive and announced** (§2.1). Run the full existing test suite before and after; results must be identical for existing tests.
1. **Graceful shutdown + server timeouts** (`core-api/cmd/api/main.go`): replace `http.ListenAndServe` with an `http.Server` having `ReadHeaderTimeout 5s`, `ReadTimeout 15s`, `WriteTimeout 30s`, `IdleTimeout 60s`; on `SIGTERM`/`SIGINT` call `Shutdown` with a 20s context, then close the DB. Keep every existing banner/route/behaviour. (The banner prints seeded passwords — leave it, but add a note in `CHANGE_REQUESTS.md` that it must be gated to `APP_ENV=local` by M1; **do not** change it yourself.)
2. **Health:** add `GET /health/live` (always 200 `{"status":"ok"}`) and `GET /health/ready` (200 if DB ping ≤1s succeeds; 503 `{"status":"degraded","database":"down"}` otherwise; when running on the mock fallback storage report `"database":"mock"` with 200 only if `APP_ENV=local`, else 503). Leave `/api/v1/health` untouched. Test both.
3. **Swagger index for all modules:** `core-api/internal/events/swagger` registry (§9) + `GET /swagger/` index listing every registered spec with links; register the events spec; add a test that registering two names produces two index entries and duplicate names error. Document how M1/M2/M3/M4/M7 register theirs (one line each).
4. **Migration tests** (`core-api/migrations/migrations_test.go`, no DB needed): every `.up.sql` has a `.down.sql`; version prefixes are unique and 14-digit timestamps; filenames sorted == dependency order for known pairs (outbox `20261101000050` < worker tables `20261101000500`); no migration contains `BEGIN;`/`COMMIT;`/`CONCURRENTLY` (D28). Do not modify `migrator.go`.
5. **Cross-module harness** `core-api/tests/`: `testenv` helper (reads `TEST_DATABASE_URL`; skips unless `INTEGRATION=1`; creates a throwaway schema/database, runs `migrations.RunMigrations`, returns `*sql.DB` + cleanup) and one smoke test: boot `BuildAppContainer()`-equivalent mux against it, log in as a seeded admin, call `/health/ready` and an events admin endpoint. Document how other members add their integration tests.
6. **Lint boundary rules** (`.golangci.yml`): add `core_api_no_broker` and `platform_isolation` (D30, Phase 1B); do not alter existing rules. Run `golangci-lint run ./...` in `core-api`.
7. **Quality gates cover messaging (D29):** extend `scripts/verify.sh` and `scripts/verify.ps1` (add a block mirroring the core-api block for `services/messaging`) and `.github/workflows/ci.yml` (add a `messaging` job or step: setup-go from `services/messaging/go.mod`, `gofmt` check, `go vet`, `golangci-lint`, `go test -race ./...`; integration tests stay opt-in). Keep existing steps intact.
8. **CODEOWNERS:** do not edit (`* @keshaka @rzmy7` covers everything); mention in the report that per-path owners are absent.
9. Record every shared-file edit with before/after summary in `CHANGE_REQUESTS.md` §"Shared-file edits".

### Phase 9 — Packaging + end-to-end
- `services/messaging/Dockerfile` (3 binaries to `/usr/local/bin/{relay,worker,reconcile}`; D/rule 13), `.dockerignore`, `Makefile` or script targets (`make test`, `make it`, `make up`).
- E2E test (`INTEGRATION=1`): insert parcel-like state + outbox rows in one transaction (using the real `PostgresOutboxWriter` through a small helper in core-api tests or SQL equivalent) → relay → RabbitMQ → worker → (a) email visible in Mailpit, (b) webhook received by fake receiver with valid signature, (c) all rows `SUCCESS`, `processed_events` filled. Failure variants: receiver 500×5 ⇒ FAILED + 5 attempts; 410 ⇒ terminal at attempt 1; endpoint disabled mid-retry ⇒ FAILED `ENDPOINT_DISABLED`; broker down then up ⇒ no loss.
- Run `./scripts/verify.sh`.

## 8. Config reference (single source; mirror in README and `.env.example`)
`APP_ENV, DATABASE_URL, RECONCILER_DATABASE_URL, RABBITMQ_URL, OUTBOX_POLL_INTERVAL_MS=500, OUTBOX_BATCH_SIZE=50, OUTBOX_STALE_SECONDS=300, OUTBOX_LEASE_SECONDS=60, WORKER_CONCURRENCY=10, WEBHOOK_HTTP_TIMEOUT_SECONDS=5, WEBHOOK_RETRY_SCHEDULE=0s,30s,2m,10m,30m, WEBHOOK_ALLOW_PRIVATE_HOSTS=false, WEBHOOK_SECRET_ENCRYPTION_KEY (hex64), DELIVERY_LEASE_SECONDS=120, SMTP_HOST, SMTP_PORT, SMTP_SECURITY, SMTP_USERNAME, SMTP_PASSWORD, SMTP_FROM_EMAIL, SMTP_FROM_NAME, PROBE_PORT, SWAGGER_ENABLED, LOG_LEVEL`. Secrets only via env; `.env.example` holds placeholders.

## 9. Swagger / OpenAPI — deliverables and rules

1. **Source of truth:** `core-api/internal/events/transport/openapi.yaml` (OpenAPI 3.0.3, `//go:embed`). Documents every Phase 7 endpoint, `bearerAuth` (JWT), schemas (`OutboxEvent, OutboxStats, Delivery, DeliveryAttempt, Page, Error`), realistic examples, responses 400/401/403/404/409/422. Top-level `webhooks:` (OpenAPI 3.1 feature) can't live in 3.0.3 ⇒ put the **merchant-facing webhook contract** in a separate file `docs/drafts/member-08-events-messaging/swagger/webhook-delivery.openapi.yaml` (OpenAPI 3.1, `webhooks: parcel.event`, header definitions D18, body schema, signature algorithm in description, response expectations 2xx/410 semantics, retry schedule). Mirror in `doc-04` §4 wording (do not contradict).
2. **Messaging probes spec:** `services/messaging/openapi.yaml` (3.0.3): `/health/live`, `/health/ready`, `/metrics` for relay (8082) and worker (8083).
3. **Serving (core-api):** `GET /api/v1/events/openapi.yaml` (raw), `GET /swagger/` index listing registered specs, `GET /swagger/events/` Swagger UI (D24). Provide a tiny registry `events/swagger.Register(name string, spec []byte)` so **other members add their own spec with one line** in their wiring; document in `SWAGGER.md`. Routes only when `SWAGGER_ENABLED=true`.
4. **Docs for the team (create these):**
   - `docs/drafts/member-08-events-messaging/swagger/README.md` ("SWAGGER.md"): how to open Swagger UI locally (Core API at `/swagger/`; or `docker run -p 8089:8080 -e SWAGGER_JSON=/spec/openapi.yaml -v <path>:/spec swaggerapi/swagger-ui`), how to authorize (`POST /api/v1/auth/login` → paste access token in "Authorize"), how other members register their module spec and write specs (conventions: path prefix, `bearerAuth`, shared `Error` schema reference, examples), and how Members 6/7/10 can generate typed clients (`npx openapi-typescript`) — instructions only, no deps added by you.
   - A **copy** of the core-api spec at `docs/drafts/member-08-events-messaging/swagger/events-ops.openapi.yaml`, kept identical by `scripts/sync-swagger.sh` + a Go test that fails if the embedded and docs copies differ.
   - `core-api/internal/events/API.md` (AGENTS §6.2: endpoint overview, headers, params, request/response examples, error envelopes) — must match the spec exactly.
5. **Tests:** (a) Go has no stdlib YAML parser and `core-api` may add no deps, so **author the embedded spec as JSON** (`core-api/internal/events/transport/openapi.json`, OpenAPI 3.0.3) and parse it with `encoding/json` in tests (required keys `openapi`, `info`, `paths`, `components`; every operation has `responses`); the docs copy `swagger/events-ops.openapi.json` is identical (plus a hand-readable YAML rendering is optional, not tested). Use `.json` wherever this plan says `.yaml` for the core-api spec; the messaging and webhook specs may stay YAML (no Go test parses them); (b) every route in the exported route slice appears in the spec (method+path) and vice-versa; (c) examples for each response are valid JSON.
6. **Other members' benefit:** add a section in `core-api/internal/events/README.md` "For other modules" (writer usage + Swagger registry).

## 10. Docs and other deliverables (the part earlier plans under-specified)

| Deliverable | Where | Notes |
| :--- | :--- | :--- |
| Module READMEs | `core-api/internal/events/README.md`, `services/messaging/README.md` | AGENTS §6.1 seven sections, Decisions list (D1–D30), env table, run commands, metrics list, retry schedule, runbook (stuck outbox, DLQ replay, failed webhooks, rotating `WEBHOOK_SECRET_ENCRYPTION_KEY`), limitations (D9, D12) |
| API docs | `core-api/internal/events/API.md` | Phase 7 |
| Change requests | append to `CHANGE_REQUESTS.md` | D1 (contracts), D13, D14, D15, D19, DEC-04, announcements for Checkpoints 1–2 |
| Doc sync | `doc-01`, `doc-03`, `doc-04`, `doc-05` | names, columns (D11), statuses (D5), DLX (D13) |
| Seed + fixtures | `services/messaging/tests/fixtures/` | event JSON samples, `08_messaging_seed.sql`, M4 stub DDL |
| Test docs | `services/messaging/tests/README.md` | how to run unit vs `INTEGRATION=1`; coverage matrix mapping FR-EVT-001–010 and FR-REC-001,002,004,005 to tests (FR-REC-003 partial) per `doc-06` |
| Demo script | `docs/drafts/member-08-events-messaging/DEMO_SCRIPT.md` | step-by-step for the lecturer: compose up → migrate → `devemit` → Mailpit email → fake-receiver webhook → force 500s to show retries/DLQ → `reconcile` → Swagger UI |
| Report section | `docs/drafts/member-08-events-messaging/report/01–07*.md` | review against code after Phase 9; fix drift only (follow `REPORT_DIAGRAMS_PLAN.md`) |

## 11. Acceptance criteria (all must be true)
- [ ] FR-EVT-001–010 and FR-REC-001, 002, 004, 005 covered by named tests; FR-REC-003 documented as partial.
- [ ] `gofmt`, `go vet`, `go test -race ./...`, `golangci-lint run ./...` pass in `core-api` and `services/messaging`; `./scripts/verify.sh` passes; integration suite passes with `INTEGRATION=1`.
- [ ] `core-api` imports no AMQP library and has no new dependency; `services/messaging` has only the three libraries in D23.
- [ ] No SQL against tables other than M8's four + `outbox_events` (+ the view); a test greps SQL constants.
- [ ] No secret/signature/full URL/email/body in logs or stored snippets (tests).
- [ ] Image runs as UID 10001, read-only rootfs, `/health/live` answers before dependencies are up.
- [ ] Swagger UI loads and "Try it out" works with a JWT; spec↔routes↔docs-copy tests pass.
- [ ] Docs match code (names, statuses, env vars, columns).
- [ ] Foundation: `platform/{config,db,middleware,httpx}` exist with tests and README; `X-Request-ID`→context→`correlation_id` works end to end; Core API shuts down gracefully (test sends SIGTERM-equivalent via `Shutdown`); `/health/live` and `/health/ready` tested; migration sanity tests pass; `core-api/tests` harness runs with `INTEGRATION=1`; `.golangci.yml` has the two new rules; `verify.sh`/`verify.ps1`/`ci.yml` run messaging checks; **all pre-existing core-api tests still pass unchanged**; every shared-file edit is additive and listed in `CHANGE_REQUESTS.md`.

## 12. Working rules for the agent
- Smallest complete change per phase; never regenerate whole modules; never weaken/delete tests.
- Show real command output for tests; don't claim "all pass" without it.
- Do not invent requirements; if silent, pick the simplest option, add it under Decisions (D26+), and flag in the report.
- Stop and ask on: Phase 7 approval, anything touching another member's files, `TenantID` ≠ merchant id (D25), migrator can't run your multi-statement files.
- Finish each phase with the `AGENTS.md` §12 completion report.

## 13. Copy-paste prompt for the implementing AI

```markdown
Task: Implement Member 8 (Events & Messaging) end to end, phase by phase
Owner: Member 8 — Event-Driven Backend & Integrations

Read First (in order):
- AGENTS.md, CLAUDE.md
- docs/drafts/member-08-events-messaging/MASTER_IMPLEMENTATION_PLAN.md   (authoritative; supersedes AI_HANDOFF_PLAN.md and IMPLEMENTATION_PLAN.md)
- docs/architecture/domain-boundaries.md, docs/contracts/{event-contracts,webhook-security,api-conventions}.md, docs/architecture/database-schema.md
- docs/drafts/member-08-events-messaging/doc-02..doc-07
- core-api/migrations/migrator.go, core-api/cmd/api/main.go, core-api/cmd/api/wiring.go, core-api/internal/identity/transport/*, .golangci.yml, scripts/verify.sh, .github/workflows/ci.yml

Requirements:
- Start with Phase 0 (output the plan with assumptions/risks), then Phase 1 only. Stop after Phase 1 and report so the team can be notified (Checkpoint 1). Then Phase 1B (platform packages; Checkpoint 1B). Continue phase by phase when I say "continue". Order: 0, 1, 1B, 2, 3, 4, 5, 6, 7*, 8, 8B, 9 (*after my confirmation).
- Core API foundation work (§2.1, Phases 1B and 8B) is mine because nobody else owns it. Edit shared files (`main.go`, `wiring.go`, `.golangci.yml`, `scripts/verify.*`, `ci.yml`) only with minimal additive changes and log each in CHANGE_REQUESTS.md. Existing tests must keep passing unchanged.
- Migration files must contain no BEGIN/COMMIT (the migrator wraps each file in a transaction).
- Follow decisions D1–D30 exactly. If a spec conflicts with the plan, the plan wins; if neither decides, stop and ask.
- Timestamped migrations only (20261101000050, 20261101000500). No new dependencies in core-api. Messaging module deps limited to lib/pq, amqp091-go, prometheus/client_golang.
- Never hold a DB transaction across broker/SMTP/HTTP calls. Never log secrets, URLs with query, emails or webhook bodies.
- Phase 7 (ops API + Swagger) only after I confirm; Swagger deliverables in §9 are mandatory when it runs.
- Update README.md/API.md in the same change as code; keep docs and Swagger identical to code.

Restrictions: do not modify other members' modules, docs/contracts, docs/architecture, deploy/, web/. Record needed changes in CHANGE_REQUESTS.md.

Workflow per phase: plan → smallest complete change → run gofmt/vet/tests/lint and paste real output → update docs → AGENTS.md §12 completion report.
```
