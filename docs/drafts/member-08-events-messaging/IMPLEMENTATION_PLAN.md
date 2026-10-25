# Member 08 — Implementation Plan (hand-off for a coding AI)

> **Audience:** An AI coding agent implementing Member 8's scope (Transactional Outbox, Outbox Relay, Event Worker, Reconciler) in the SwiftDrop repo.
> **Status:** Specs in this folder are drafts pending Meeting 2. Implement against them, but follow §2 (pre-flight decisions) first.
> **You must obey:** `AGENTS.md` (repo root) and `CLAUDE.md`. Where they conflict with this plan, they win. If something is ambiguous, stop and ask rather than guess.

---

## 1. Goal and scope

Build the asynchronous backbone: `database commit → outbox row → RabbitMQ → consumer → email / merchant webhook`, with retries, DLQ, idempotency, reconciliation, and tests that prove it.

| Deliverable | Location |
| :--- | :--- |
| Outbox writer + envelope (in-process Go library used by M2/M3/M4) | `core-api/internal/events/` |
| Migrations `000008`, `000009` | `core-api/migrations/` |
| Outbox Relay (`cmd/relay`, port 8082) | `services/messaging/` |
| Event Worker (`cmd/worker`, port 8083): email notifications + webhooks | `services/messaging/` |
| Reconciler CLI (`cmd/reconcile`) | `services/messaging/` |
| One image `messaging` containing 3 binaries `/usr/local/bin/{relay,worker,reconcile}` | `services/messaging/Dockerfile` |
| Module docs | `services/messaging/README.md`, `core-api/internal/events/README.md` (update both; no `API.md` needed unless you add public HTTP endpoints — health/metrics are internal) |

**Out of scope (owned by others — do not edit):** parcel/dispatch/merchant code, `webhook_endpoints`/`merchant_webhook_targets` (M4), Tracking API and Redis (M5), Kubernetes/Compose manifests under `deploy/` (M9; you may *propose* changes in `CHANGE_REQUESTS.md`), docs outside `docs/drafts/member-08-events-messaging/`.

## 2. Pre-flight: resolve these before writing code

The M8 drafts contradict each other or other members in the places below. Fix the M8 docs first (small edits, in this folder only), then code to the resolved version. Record each decision in a short "Decisions" section of `services/messaging/README.md`.

| # | Conflict | Required resolution |
| :--- | :--- | :--- |
| D1 | `doc-03` §3.1 envelope uses `Version int` + `Timestamp`; DDL (`doc-05` §4.1) stores `schema_version VARCHAR '1.0.0'` and `occurred_at`; `AnyEnvelope.GetSchemaVersion()` returns `int`. | Use the DDL. Envelope JSON fields: `event_id, event_type, aggregate_type, aggregate_id, aggregate_version, schema_version (string), occurred_at, correlation_id, producer, payload`. `GetSchemaVersion() string`. |
| D2 | Failure event name: `parcel.failed.v1` (M8 docs) vs `parcel.delivery_failed.v1` (authoritative `event-contracts.md`, M3). | Use `parcel.delivery_failed.v1` unless the Tech Lead says otherwise (CR-M03-02 item 4). Define names only as constants in `catalog.go`. |
| D3 | Webhook secret env var: `WEBHOOK_SECRET_KEY_V1` (`doc-07`, `doc-08`) vs `WEBHOOK_SECRET_ENCRYPTION_KEY` (Agreement A5). | Use `WEBHOOK_SECRET_ENCRYPTION_KEY` (64 hex chars). Key is chosen by `secret_key_version`; support a map `{1: key}` internally. |
| D4 | View columns: M8 text says `target_url`, `is_active`. M4's real view (`docs/drafts/member-04-merchants/migrations-draft/20261101000400_…view.up.sql`) exposes `endpoint_id, merchant_id, url, secret_ciphertext, secret_key_version, subscribed_event_types, updated_at`. | Code against M4's real columns (`url`). |
| D5 | `SENT` status in `doc-03:400`, `doc-04:749`. | Use `PENDING / IN_PROGRESS / SUCCESS / FAILED` everywhere. |
| D6 | DB roles: `doc-07` runs `CREATE ROLE`; Agreement D6 says Member 9 bootstraps roles. | Migrations contain `GRANT` only, guarded (`IF EXISTS (SELECT 1 FROM pg_roles …)`), never `CREATE ROLE`. Roles: `swiftdrop_relay`, `swiftdrop_worker`; `swiftdrop_reconciler` is pending M9 — design the reconciler so the role is a config value (`RECONCILER_DATABASE_URL`) that can point to either. |
| D7 | Database name: `swiftdrop` vs `swiftdrop_db`. | `swiftdrop_db`. |
| D8 | Outbox writer name: `Write` (M8) vs `WriteEvent` (M3) vs `Append` (M4). | Implement `Write(ctx, tx, ...AnyEnvelope)`. Do not add aliases. Producers adapt. |
| D9 | FR-REC-003 (detect parcels missing history/assignments) needs read access to M2/M3 tables, which the boundary rules forbid. | Implement only outbox/delivery inconsistency checks. Leave FR-REC-003 as a documented `TODO(DEC-04)` with a stub interface `ConsistencyChecker`. Do **not** query `parcels`. |
| D10 | `notify` block / `merchant_id` / `recipient_email` are not yet in M2's contract. | Treat them as optional in payload parsing. If `notify.recipient_email` is absent, mark the notification `FAILED` with `last_error = 'NO_RECIPIENT'` and continue; never query another module's tables. |

## 3. Non-negotiable design rules (from specs and AGENTS.md)

1. **Atomicity:** `OutboxWriter.Write` uses the caller's `*sql.Tx`. It never opens its own transaction or connects to RabbitMQ. Core API never imports an AMQP library.
2. **Relay:** claim a small batch in a short transaction using `FOR UPDATE SKIP LOCKED` with `status='PENDING' AND (next_attempt_at IS NULL OR next_attempt_at <= now())`; release the lock; publish each message with `delivery_mode=2` and wait for its publisher confirm; then run a short `UPDATE … SET status='PUBLISHED', published_at=now()` per confirmed row. Never hold a DB transaction across broker calls. After 5 failed attempts set `FAILED` and keep `attempt_count` (never reset it). The relay runs as exactly one replica.
3. **At-least-once, idempotent effects.** Do not claim exactly-once (FR-EVT-010). `processed_events (consumer_name, event_id)` is written only after the terminal outcome (`SUCCESS` or terminal `FAILED`), in the same transaction as the delivery row update.
4. **Retries live in PostgreSQL, not in AMQP.** On consuming an event: persist delivery rows (`webhook_deliveries` unique `(endpoint_id, event_id)`, `notification_deliveries` unique `(event_id, channel, recipient)`) with `ON CONFLICT DO NOTHING`, then `ack`. A poller claims due rows (`FOR UPDATE SKIP LOCKED`, sets `IN_PROGRESS` + `lease_expires_at`), performs the external call **outside any DB transaction**, and records the outcome. Never `Nack(requeue=true)` in a loop. Messages that cannot be parsed go to the DLQ (`swiftdrop.dlq.exchange` → `swiftdrop.dlq`, 30-day TTL).
5. **Retry schedule:** 5 attempts at 0s, 30s, 2m, 10m, 30m, each ±20% jitter (≈42.5 min). Make the schedule configurable by env var so tests can compress it. Webhook outcome matrix: 2xx = `SUCCESS`; 408/429/5xx/timeout/network error = retry; other 4xx, 3xx, and SSRF-blocked = terminal `FAILED`. Never follow redirects.
6. **Webhooks:** target read from `merchant_webhook_targets` on every attempt (no long-lived cache). Decrypt `secret_ciphertext` with AES-256-GCM: layout `12-byte nonce || ciphertext || 16-byte tag`, AAD = `endpoint_id` string, reject inputs under 28 bytes. Sign with HMAC-SHA256 over `"{t}.{body}"`; send `X-SwiftDrop-Signature: t=<unix>,v1=<hex>`, `X-SwiftDrop-Event-ID`, `X-SwiftDrop-Delivery-ID`. Body fields: `event_id, event_type, aggregate_id, status, occurred_at, data`. Store response snippets truncated to 500 bytes (the DDL column is `VARCHAR(500)`; if `doc-04` says 1,024, follow the DDL), valid UTF-8, printable only. Never log secrets, full URLs with query strings, or recipient emails.
7. **SSRF guard:** custom `DialContext` that resolves the host, rejects if **any** resolved IP is loopback, private (RFC 1918), link-local, `100.64.0.0/10`, `0.0.0.0/8`, multicast, reserved, NAT64, or IPv4-mapped IPv6 of those, then dials the checked IP (defeats DNS rebinding). HTTPS only (HTTP allowed only behind an explicit `WEBHOOK_ALLOW_INSECURE_TEST=true` flag used by tests and Mailpit-style local receivers). `CheckRedirect` returns `http.ErrUseLastResponse`. Per-request timeout 10s.
8. **Email:** single `net/smtp`-based sender used in every environment; Mailpit (`localhost:1025`) in local and CI. STARTTLS fail-closed when `SMTP_REQUIRE_TLS=true`; implicit TLS on 465. Generate a `Message-ID` and store it in `provider_message_id`. Context deadline on every send.
9. **Domain boundaries:** the worker reads only `merchant_webhook_targets`, its own tables, and the event payload. Zero queries against `parcels`, `couriers`, or other module tables.
10. **Observability:** `/health/live`, `/health/ready`, `/metrics` on 8082 (relay) / 8083 (worker). The health server starts **before** connecting to Postgres/RabbitMQ; readiness reflects dependency state but the process must not crash-loop during an outage. Metrics use `prometheus/client_golang`, names prefixed `swiftdrop_`, no high-cardinality labels. Required: `swiftdrop_outbox_pending_events`, `swiftdrop_outbox_oldest_pending_age_seconds`, `swiftdrop_outbox_publish_failures_total`, `swiftdrop_consumer_failures_total`, `swiftdrop_webhook_delivery_attempts_total`, `swiftdrop_dlq_messages_routed_total`, `swiftdrop_dlq_depth_gauge` (names per `doc-08` §4 and Agreement A11).
11. **Container:** multi-stage, pinned Go base (no `latest`), `USER 10001:10001`, `-trimpath`, read-only root filesystem compatible, handle SIGTERM with graceful drain (finish in-flight confirms/deliveries, release leases).
12. **Reconciler:** CLI `reconcile --stale-threshold=5m --batch-size=50` (plus a `retention` subcommand). It never connects to RabbitMQ. It (a) logs and counts `PENDING` outbox rows older than the threshold, (b) resets expired leases (`IN_PROGRESS` with `lease_expires_at < now()`) to `PENDING`, (c) purges per retention: outbox published > 7 days, delivery rows > 30 days, `processed_events` > 35 days, in batches that fit a 5s `statement_timeout`. Idempotent and batch-bounded; one structured JSON log line per action.

## 4. Phases

Each phase ends green on `go test -race ./...` and `golangci-lint run ./...` for the touched module(s). Commit per phase on a feature branch; do not commit to `main`.

### Phase 0 — Inspect and reconcile specs (no code)
- Read: `AGENTS.md`, `docs/architecture/domain-boundaries.md`, `docs/contracts/event-contracts.md`, `docs/architecture/database-schema.md`, and this folder's `MEMBER_08_ALIGNMENT_AND_SPECIFICATION_GUIDE.md`, `doc-02` (flows 1–5, retry matrix), `doc-03` (§2–§3), `doc-04` (envelope, webhook contract, topology), `doc-05` §4, `doc-07` §1.
- Read existing deploy manifests you must stay compatible with: `deploy/kubernetes/keda-event-worker.yml`, `cronjob-reconciliation.yml`, `rabbitmq-statefulset.yml`.
- Apply the §2 decisions to M8 docs. Output: a one-page plan stating assumptions and risks (required by `AGENTS.md` §3).

### Phase 1 — Events library and migrations
- `core-api/internal/events/`: `envelope.go` (`EventEnvelope[T]`, `AnyEnvelope`), `writer.go` (interface), `repository.go` (SQL insert preserving the caller's UUIDv7 `event_id` as `outbox_events.id`; validates `event_type` against `^[a-z_]+\.[a-z_]+\.v[0-9]+$`, requires non-empty `correlation_id`), `catalog.go` (type constants incl. `parcel.booked/assigned/picked_up/in_transit/out_for_delivery/delivered/delivery_failed/returned/cancelled.v1`, `courier.unassigned.v1`, `merchant.status_changed.v1`, `merchant.approved.v1`), `errors.go`.
- Migrations `000008_create_outbox_events` and `000009_create_worker_dedup_and_webhooks` (up/down) taken from `doc-05` §4, with the D6 guarded grants appended. `000009` depends on M4's `webhook_endpoints`; if M4's migration is not in the repo yet, add a clearly marked local test stub under test fixtures only — do not add it to `core-api/migrations/`.
- Tests: unit tests for serialization, invalid type, missing correlation ID; an integration test (testcontainers or the repo's chosen approach) that rolls back the caller transaction and asserts **no** outbox row remains, and commits and asserts exactly one row.
- **Publish the `OutboxWriter` interface as early as possible**; M2/M3/M4 are blocked on it.

### Phase 2 — Messaging service skeleton
- `services/messaging/` Go module: `internal/config` (strict env parsing, fail fast on bad key length/URLs), `internal/health` (server up first), `internal/broker` (connection manager with reconnect, topology declare, confirm-mode publisher, consumer with manual ack), logging (JSON, `request_id`/`correlation_id` fields), metrics registry.
- Topology: exchange `swiftdrop.events.topic` (topic, durable); queues `q.tracking.updates` (`parcel.#`, `courier.#`), `q.notifications.worker` (`parcel.#`), `q.webhooks.dispatcher` (`parcel.#`, `courier.#`, `merchant.#`); DLX `swiftdrop.dlq.exchange` (fanout) → `swiftdrop.dlq` with `x-message-ttl=2592000000`. All durable. Declaration is idempotent.
- Tests: topology declare twice; consumer receives a published message; connection-loss reconnect.

### Phase 3 — Outbox Relay
- `internal/outbox/poller.go`, `relay.go` as in rule 2. Routing key = `event_type`. Message headers: `event_id`, `correlation_id`; body = full envelope rebuilt from the row (include `aggregate_version`, `schema_version`, `producer`, `occurred_at`).
- Tests (integration, real Postgres + RabbitMQ): publishes and marks `PUBLISHED`; broker NACK/unroutable leaves row `PENDING` with `attempt_count+1` and honours `next_attempt_at`; poison row reaches `FAILED` after 5; two relays on the same DB never double-claim a row; crash after publish but before DB update causes a duplicate publish only (documented, consumers dedupe).

### Phase 4 — Worker core and idempotency
- `internal/idempotency/store.go`, delivery-row creation on consume (rule 4), lease claim poller, backoff calculator (`internal/webhook/backoff.go`, deterministic with injected RNG for tests), monotonic aggregate-version guard (drop an event whose `aggregate_version` ≤ the last processed for that aggregate for ordered effects).
- Tests: 10 duplicate deliveries → one effect; worker killed mid-attempt → lease expires → exactly one more attempt; unparseable message → DLQ with no hot loop.

### Phase 5 — Notifications (email)
- `internal/notifier/{sender,templates,repository,handler}.go`. Templates: booked, out_for_delivery, delivered, delivery_failed (embedded, plain + HTML, UTF-8 safe headers). Map event → template; read `notify` block (D10). Classify SMTP 4xx = retry, 5xx = terminal.
- Tests: against an in-process SMTP listener and Mailpit in CI; assert `provider_message_id` stored; permanent failure → `FAILED`; no PII in logs.

### Phase 6 — Webhooks
- `internal/webhook/{client,decrypt,signer,dispatcher,repository}.go` per rules 5–7. One delivery row per `(endpoint, event)`; match `subscribed_event_types` against `event_type`; append one `webhook_delivery_attempts` row per attempt.
- Tests: signature vector test (documented known-answer so receivers can verify); decrypt with wrong AAD or short input fails; SSRF table test (loopback, RFC1918, link-local, metadata IP, IPv4-mapped IPv6, DNS name resolving to a private IP, redirect to metadata IP); 410/429/5xx/timeout matrix; secrets never appear in logs or stored snippets.
- Provide a local fake receiver (`services/messaging/tests/mocks/` or a `cmd/fake-receiver` dev tool) because the SRS/work-division requires a demo sink.

### Phase 7 — Reconciler
- `cmd/reconcile` and `internal/reconcile/reconciler.go` per rule 12, with the D9 stub. Subcommands: default run, `retention`. Exit code non-zero on failures, zero on "nothing to do".
- Tests: stale outbox detected and counted but not re-published; expired leases reset; retention batch-bounded; running twice is a no-op.

### Phase 8 — Packaging, DLQ replay, docs
- `Dockerfile` (rule 11), `.dockerignore`, local `docker-compose` snippet for Postgres + RabbitMQ + Mailpit under `services/messaging/` (not root Compose — that is M9's).
- DLQ inspect/replay: a `reconcile dlq-replay --limit N` (or separate subcommand) that republishes selected DLQ messages to the exchange with a `x-replayed-from` header; document in a runbook section.
- Update `services/messaging/README.md` and `core-api/internal/events/README.md`: architecture, env vars (names match `doc-08` §1 after D3/D7), run instructions, health ports, metrics list, retry schedule, runbook (stuck outbox, DLQ replay, failed webhooks), and the Decisions list.
- Run `./scripts/verify.sh`.

### Phase 9 — End-to-end proof
- Integration test: insert parcel-like state + outbox row in one Core-API-style transaction → relay → RabbitMQ → worker → (a) email visible in Mailpit, (b) webhook received by the fake receiver with a verifiable signature, (c) all rows `SUCCESS`, (d) `processed_events` populated. Plus failure variants: receiver returns 500 five times → `FAILED` + attempts recorded; receiver 410 → terminal.

## 5. Acceptance criteria

- [ ] All FR-EVT-001–010 and FR-REC-001, 002, 004, 005 satisfied; FR-REC-003 explicitly marked partial (D9).
- [ ] `go test -race ./...` and `golangci-lint run ./...` pass in `core-api` (events package) and `services/messaging`; `./scripts/verify.sh` passes.
- [ ] No import of `database/sql` or AMQP in any `domain/` package; no query on tables M8 does not own except `merchant_webhook_targets`.
- [ ] No secret, full webhook URL, or email address appears in logs (test asserts this).
- [ ] Image runs as UID 10001, with read-only root FS, and responds on `/health/live` before dependencies are ready.
- [ ] M8 docs and READMEs updated and consistent with the code (names, statuses, env vars).

## 6. Working rules for the agent

- Smallest complete change per phase; never regenerate whole modules. Never weaken or delete tests to get green.
- Do not edit other members' files or `docs/contracts/`, `docs/architecture/`, `deploy/`. Put needed changes into `CHANGE_REQUESTS.md` instead.
- Do not invent requirements. When a spec is silent, choose the simplest option, record it under "Decisions", and flag it in your report.
- Finish with the structured completion report required by `AGENTS.md` §12: what changed, files touched, tests run and results, invariants verified, assumptions, and open items (including anything in §2 still unresolved).
