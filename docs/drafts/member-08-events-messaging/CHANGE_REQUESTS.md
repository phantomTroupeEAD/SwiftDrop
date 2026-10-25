# Member 08 — Cross-Member Interface Change Requests & Architectural Alignment

> **Author:** Member 08 (Event-Driven Backend & Messaging)  
> **Target Audience:** SwiftDrop Technical Lead, System Architect, and Members 01 through 07, 09, 10  
> **Status:** Revision 2.1 — Formal Proposals Awaiting Cross-Member Review & Sign-Off (Meeting 2)  
> **Authority:** In compliance with `AGENTS.md` Layer 1 & Layer 4, Member 08 does not modify files owned by other members. These change requests specify the interface, contract, and schema adjustments required for the Member 08 design to integrate with the other drafts.
> **Revision 2.1 note:** Re-verified against the other members' current drafts and `team-agreements-with-member-09.md`. Corrected folder paths, the `merchant_webhook_targets` column list (now matches Member 4's actual view), the database-role ownership (Member 9 bootstrap, Agreement D6), and added requests against the authoritative `docs/contracts/event-contracts.md` and `docs/architecture/database-schema.md`.

---

## 1. Executive Summary of Alignment Items

All internal Member 08 files, migrations (`000008`, `000009`) and reference code follow one canonical standard. Clean integration across SwiftDrop needs reciprocal updates from the owners below. Every request lists the affected files, the verified current state, and the requested change.

| ID | Recipient | Subject | Status |
| :--- | :--- | :--- | :--- |
| CR-M01-01 | M1 | `X-Request-ID` sanitising, actor-neutral envelopes | Proposed |
| CR-M02-01 | M2 | `notify` block, `merchant_id`, `recipient_email` | Proposed (M2 `doc-04` has no email field today) |
| CR-M02-02 | M2 | Optional `RABBITMQ_URL` in Core API | Proposed |
| CR-M03-01 | M3 | `OutboxWriter` signature, UUID IDs, `correlation_id`, exchange name | Proposed |
| CR-M03-02 | M3 | Remove courier phone, notes, raw S3 URLs; align event names | Proposed |
| CR-M04-01 | M4 | `merchant_webhook_targets` columns and AES-256-GCM contract | Mostly aligned; M8 adapts to M4's view |
| CR-M04-02 | M4 | `merchant.webhook_dispatch.v1` removal | Agreed in principle (M4 U-07) |
| CR-M04-03 | M4 / M6 / M7 | Delivery-history read access | Proposed |
| CR-M05-01 | M5 | Queue binding `q.tracking.updates`, Redis isolation | Proposed (M5 draft leaves broker arrangement open) |
| CR-M06-01 / CR-M07-01 | M6 / M7 | Webhook status enum in UI | Proposed |
| CR-M09-01 | M9 | StatefulSet, Mailpit, KEDA, roles, CronJob, probes | Partly agreed (see §2) |
| CR-M10-01 | M10 | Signature header, test receiver, retry expectations | Proposed |
| CR-AUTH-01 | Tech Lead | Edits to `event-contracts.md` | Proposed |
| CR-AUTH-02 | M9 (DB coordinator) / Tech Lead | Edits to `database-schema.md` | Proposed |

---

## 2. Cross-Member Change Requests

```
                              ┌────────────────────────┐
                              │     Member 08 Event    │
                              │  Messaging Subsystem   │
                              └───────────┬────────────┘
                                          │
       ┌──────────────────┬───────────────┼───────────────┬──────────────────┐
       ▼                  ▼               ▼               ▼                  ▼
┌──────────────┐   ┌──────────────┐┌──────────────┐┌──────────────┐   ┌──────────────┐
│  Member 02   │   │  Member 03   ││  Member 04   ││  Member 05   │   │  Member 09   │
│ Parcels & DDL│   │Dispatch & Pod││Merchants View││ Tracking SSE │   │ DevOps & K8s │
└──────────────┘   └──────────────┘└──────────────┘└──────────────┘   └──────────────┘
```

---

### CR-M01-01: Distributed Tracing Header Alignment (`X-Request-ID`) & Envelope Actor Scoping
- **Recipient Domain:** Member 01 (Identity, Access & Security)
- **Affected Documentation & Code:** `docs/architecture/api-standards.md` (if present), `docs/contracts/api-conventions.md`, `core-api/internal/identity/`, `core-api/internal/platform/middleware/`
- **Current State:** Team Agreement A10 fixes the header as `X-Request-ID`, and says outbox events copy it to `correlation_id`. Some draft diagrams use `X-Correlation-ID`. The authoritative `event-contracts.md` shows `corr_…`-prefixed IDs.
- **Requested Action:**
  1. Use `X-Request-ID` as the only HTTP tracing header across Traefik and Core API.
  2. In Core API middleware, validate it against `^[a-zA-Z0-9_\-]{1,128}$`, generate a fresh UUIDv7 if missing or invalid, and put it in `context.Context`.
  3. Aggregate use cases pass this value into `EventEnvelope.CorrelationID` when calling `OutboxWriter.Write`.
  4. Envelopes stay actor-neutral: `actor_id` / `actor_role` may appear in the payload for audit, never raw JWTs.

---

### CR-M02-01: Self-Contained Domain Events for Notifications (`notify` Block) & Zero Monolith Boundary Leaks
- **Recipient Domain:** Member 02 (Parcels & Pricing)
- **Affected Documentation & Code:** `docs/drafts/member-02-parcels-pricing/` (`doc-04`, `doc-05`), `docs/contracts/event-contracts.md`, `core-api/internal/parcels/`
- **Current State:** SRS FR-EVT-008 needs a recipient email for notifications. Member 2's `doc-04` / `doc-05` define no `recipient_email`. Domain-boundaries §4.2 forbids the Event Worker from querying `parcels`.
- **Requested Action:**
  1. Collect `recipient_email` at booking (API schema and booking wizard; coordinate with M6) and store it with the parcel recipient data.
  2. `parcel.booked.v1`, `parcel.out_for_delivery.v1`, `parcel.delivered.v1`, `parcel.failed.v1` carry a minimal `notify` block:
     ```json
     "notify": { "recipient_name": "Jane Doe", "recipient_email": "jane.doe@example.com" }
     ```
  3. Every parcel event payload includes `merchant_id` (UUID, null for non-merchant parcels) so the webhook dispatcher can resolve targets without reading M2 tables.
  4. Events that M3 emits (assignment, pickup, delivery) must get the same `notify` and `merchant_id` fields from the parcel data M3 already reads through M2's interface.

---

### CR-M02-02: Direct Broker Access in Core API Parcels Domain
- **Recipient Domain:** Member 02 (Parcels & Pricing)
- **Affected Documentation & Code:** `docs/drafts/member-02-parcels-pricing/doc-08-implementation-deployment-and-ops.md` (lines ~137, ~159), `core-api/internal/parcels/`
- **Current State:** M2's `doc-08` lists `RABBITMQ_URL` as "optional direct dependency only if this process is approved to use it". Team Agreement and the Master Audit §5.1 say Core API never connects to RabbitMQ.
- **Requested Action:**
  1. Remove `RABBITMQ_URL` and any AMQP library from Core API config and docs.
  2. Write events only through `events.OutboxWriter.Write(ctx, tx, envelopes...)` inside the transaction that commits the parcel.

---

### CR-M03-01: Outbox Writer Interface Alignment (`OutboxWriter.Write`)
- **Recipient Domain:** Member 03 (Dispatch, Couriers & Proof of Delivery)
- **Affected Documentation & Code:** `docs/drafts/member-03-dispatch-delivery/doc-04` (§4 and the `OutboxWriter` interface near line 812), `core-api/internal/dispatch/`, `core-api/internal/delivery/`
- **Current State:** Three different names exist: M3 `WriteEvent(ctx, tx, eventType, aggregateID, payload)`, M4 `OutboxWriter.Append(ctx, tx, envelope)`, M8 `Write(ctx, tx, ...AnyEnvelope)`. M3 uses `prcl_…`-prefixed IDs in examples and publishes to exchange `swiftdrop.events`; the M8 exchange is `swiftdrop.events.topic` (matches `event-contracts.md` §4).
- **Requested Action:**
  1. Everyone (M2, M3, M4) calls the single M8 interface:
     ```go
     type OutboxWriter interface {
         Write(ctx context.Context, tx *sql.Tx, envelopes ...AnyEnvelope) error
     }
     ```
     M8 will publish the interface and `EventEnvelope[T]` early (work-division §coordination).
  2. Use canonical UUIDs for `aggregate_id` and `event_id` (UUIDv7). The `prcl_…` strings are display-only.
  3. Propagate `correlation_id` from the request context.
  4. Refer to the exchange as `swiftdrop.events.topic`.

---

### CR-M03-02: Stripping PII & Sensitive Data from Dispatch and Delivery Events; Event Name Alignment
- **Recipient Domain:** Member 03 (Dispatch, Couriers & Proof of Delivery)
- **Affected Documentation & Code:** `docs/drafts/member-03-dispatch-delivery/doc-04` (§4 event payloads), `docs/contracts/event-contracts.md`
- **Current State:** M3's draft payloads and API examples carry courier `phone_number`, free-text `notes` / `dispatch_notes` (some with phone-call details), and `recipient_phone`. Those must not reach merchant webhooks or the shared bus. M3 names the failure event `parcel.delivery_failed.v1`; M8's catalog uses `parcel.failed.v1`.
- **Requested Action:**
  1. `parcel.assigned.v1`: only `courier_id` and `courier_name`, no phone.
  2. `parcel.delivered.v1`: use `proof_id` (opaque) or a short-lived HTTPS URL, never `s3://` paths.
  3. Failure event: replace free-text notes with an enum `reason_code` (`RECIPIENT_UNAVAILABLE`, `INCORRECT_ADDRESS`, `ACCESS_RESTRICTED`, `OTHER`).
  4. **Decision needed (Meeting 2):** keep M3's `parcel.delivery_failed.v1` (already in `event-contracts.md`) or rename to `parcel.failed.v1`. M8's recommendation: **keep `parcel.delivery_failed.v1`**, as it is the authoritative name; M8 docs will be switched to it once confirmed.
  5. Emit these events: `parcel.assigned.v1`, `courier.unassigned.v1`, `parcel.picked_up.v1`, `parcel.in_transit.v1`, `parcel.out_for_delivery.v1`, `parcel.delivered.v1`, and the failure event above.

---

### CR-M04-01: Canonical PostgreSQL View `merchant_webhook_targets` & AES-256-GCM Contract
- **Recipient Domain:** Member 04 (Merchant Integration)
- **Affected Documentation & Code:** `docs/drafts/member-04-merchants/` (`migrations-draft/20261101000400_…view.up.sql`, `contracts/webhook-config-read-contract.sql`), `docs/contracts/webhook-security.md`
- **Current State (verified):** M4 already owns a view named `merchant_webhook_targets` exposing `endpoint_id, merchant_id, url, secret_ciphertext, secret_key_version, subscribed_event_types, updated_at`, filtered on `enabled = true AND deleted_at IS NULL AND merchants.status = 'APPROVED'`. M4's migration grants `SELECT` to `swiftdrop_worker` when the role exists. **Member 8 adopts M4's view as-is**; the earlier M8 text that used `target_url` and `is_active` is wrong and is being corrected in M8 docs.
- **Requested Action:**
  1. Keep the view name and column list above stable; any change goes through a versioned change request to M8.
  2. Confirm the **encryption contract**: AES-256-GCM, stored bytes = 12-byte nonce + ciphertext + 16-byte tag (minimum 28 bytes); AAD = `endpoint_id` string; 256-bit key from `WEBHOOK_SECRET_ENCRYPTION_KEY` (64 hex chars, one shared secret per Agreement A5); `secret_key_version` selects the key.
  3. Confirm that `M4` stores the signing secret recoverably (no `secret_hash` for signing).

---

### CR-M04-02: Rejection of `merchant.webhook_dispatch.v1` from Message Bus
- **Recipient Domain:** Member 04, plus Tech Lead for `event-contracts.md`
- **Affected Documentation & Code:** `docs/contracts/event-contracts.md` §2, M4 `doc-04` (U-07)
- **Current State:** `event-contracts.md` lists the event with producer `internal/merchants`. M4 states it does not produce it and asks to reassign or remove it (U-07).
- **Requested Action:** Remove it from the catalog. Webhook credentials and private URLs never travel on RabbitMQ. M8 consumes native domain events on `q.webhooks.dispatcher`, reads `merchant_webhook_targets`, and delivers directly. If M4's proposed `merchant.status_changed.v1` (U-09) is approved, M8 consumes it via `merchant.#`.

---

### CR-M04-03: Webhook Delivery Audit Querying Interface
- **Recipient Domain:** Member 04, Members 06 and 07 (portals)
- **Affected Documentation & Code:** `docs/drafts/member-04-merchants/`, `core-api/internal/merchants/`
- **Requested Action:**
  1. M8 owns `webhook_deliveries` (`id, endpoint_id, event_id, status, attempt_count, next_attempt_at, lease_expires_at, response_status_code, response_body_snippet, last_error, created_at, delivered_at`) and `webhook_delivery_attempts` (`delivery_id, attempt_no, started_at, response_status_code, response_body_snippet, error_class, error_message, next_attempt_at`), exactly as in M8 `doc-05` §4.3. `webhook_deliveries` has no `merchant_id`; merchant filtering joins through `endpoint_id` to `webhook_endpoints.merchant_id`.
  2. Merchant-facing history (`GET /api/v1/merchants/webhooks/deliveries`) is served by Core API using a **read-only grant to the Core API role (`swiftdrop_app`)** on these two tables, always filtered by the authenticated `merchant_id`. The grant is added in the bootstrap SQL (see CR-M09-01). M4 must not write to them.

---

### CR-M05-01: Tracking Queue Bindings & Redis Projection Isolation
- **Recipient Domain:** Member 05 (Tracking API & Real-Time Updates)
- **Affected Documentation & Code:** `docs/drafts/member-05-tracking/` (`doc-03`, `doc-04`), `services/tracking/`
- **Current State:** M5 `doc-03` says the broker-to-consumer arrangement "remains subject to team agreement" and names no queue. The authoritative `event-contracts.md` already lists `q.tracking.updates`.
- **Requested Action:**
  1. M8 declares queue `q.tracking.updates` bound to `swiftdrop.events.topic` with keys `parcel.#` and `courier.#`.
  2. M5 consumes that queue as an independent consumer, updates its own read projection or cache, and applies the same idempotency rule (dedupe by `event_id`). M8 has no connection to Redis.
  3. M5 sanitises public tracking output to mask recipient PII (SRS SEC-009).
  4. M5 must tell M9 whether Redis is required; M8 does not depend on it.

---

### CR-M06-01 & CR-M07-01: Webhook Delivery History UI & Admin Diagnostics
- **Recipient Domains:** Member 06 (Customer/Merchant Frontend) & Member 07 (Dispatcher/Admin Frontend)
- **Affected Documentation & Code:** `web/` merchant webhook pages, admin diagnostics pages
- **Requested Action:**
  1. Use the status enum `PENDING | IN_PROGRESS | SUCCESS | FAILED` (not `RETRYING`, `DLQ`, `SENT`) and the field `attempt_count` (1–5).
  2. Show per-attempt rows from `webhook_delivery_attempts` (HTTP status, latency, short error text). Do not display raw response bodies beyond the stored 1 KB snippet.

---

### CR-M09-01: Infrastructure Provisioning, K8s Manifests & RBAC Configuration
- **Recipient Domain:** Member 09 (DevOps, Kubernetes & Observability)
- **Affected Documentation & Code:** `docs/drafts/member-09-devops-k8s/`, Compose files, `k8s/` manifests, bootstrap SQL
- **Already agreed with M9 (2026-10-25, `team-agreements-with-member-09.md`):** one `messaging` image with `relay`, `worker`, `reconcile` binaries; ports 8082/8083; `event-worker` consumes `q.notifications.worker` + `q.webhooks.dispatcher` with two KEDA triggers (`queueLength 10`), max 3 pods; DLQ `swiftdrop.dlq` / `swiftdrop.dlq.exchange`; reconciler `reconcile --stale-threshold=5m --batch-size=50` every 10 minutes; `RABBITMQ_URL` per A5; pools relay 5 / worker 10 per pod / reconciler 2 (D8); metric prefix `swiftdrop_` (A11).
- **Closes M9 items:** K17 (health paths now `/health/live`, `/health/ready`, `/metrics`) and K18 (image `ghcr.io/phantomtroupeead/swiftdrop/messaging:<git-sha>`, `USER 10001:10001`) are done on the M8 side; K19 is adopted (`swiftdrop_outbox_pending_events`). `correlation_id` and `processed_events` are now in M8's DDL (migration 000008/000009).
- **Requested Action:**
  1. **RabbitMQ as a StatefulSet** with a persistent volume (`volumeClaimTemplates`), not a stateless Deployment.
  2. **Mailpit** (`axllent/mailpit:v1.21`, SMTP 1025, UI 8025) in the root Compose file and CI, for DEC-01.
  3. **Database roles (Agreement D6):** the roles are created once by M9's bootstrap SQL, **before** migrations; M8 migrations contain only `GRANT`s, no `CREATE ROLE`. Roles used by M8 are `swiftdrop_relay` and `swiftdrop_worker`. **Open question for M9:** add `swiftdrop_reconciler`, or let the reconciler run as `swiftdrop_worker`? M8 recommends adding it (reconciler needs `UPDATE`/`DELETE` on `outbox_events`, which the worker must not have). Grant details are in M8 `doc-07` §1; M8's `doc-07` no longer contains `CREATE ROLE`; it lists grants only.
  4. **Secrets:** follow Agreement A5 — `postgres-secrets`, `rabbitmq-secrets`, shared `integrations-secrets` for `WEBHOOK_SECRET_ENCRYPTION_KEY` and SMTP, and module-specific `<service>-<module>-secrets` for per-role DB URLs. M8 docs now use `outbox-relay-secrets`, `event-worker-secrets` and `outbox-reconciler-secrets`; M9 please confirm these names.
  5. **KEDA:** `ScaledObject` `event-worker`, `minReplicaCount: 1`, `maxReplicaCount: 3`, secret-based `TriggerAuthentication`.
  6. **CronJob:** `swiftdrop-outbox-reconciler`, `*/10 * * * *`, `concurrencyPolicy: Forbid`.
  7. **Database name:** use `swiftdrop_db` everywhere (K16). M8 docs will be aligned.
  8. **Probes and scrape annotations** on ports 8082 (relay) and 8083 (worker). The CronJob exposes no metrics; its results are log lines.

---

### CR-M10-01: End-to-End Testing Fixtures & Webhook Signature Alignment
- **Recipient Domain:** Member 10 (AI Assistant, MCP & Cross-Workflow Quality)
- **Affected Documentation & Code:** `docs/drafts/member-10-ai-assistant-quality/` (`doc-02`, `doc-03`, `doc-05`, `doc-06`), `tests/e2e/`
- **Current State:** M10 expects `X-SwiftDrop-Signature` and a receiver at `http://localhost:9999/webhook` (`doc-05:263`, `doc-06:132`). M8's SSRF-safe client blocks loopback, so that fixture fails with `ErrBlockedDestinationIP`. M10 `doc-02:279` asks M8 to confirm the header name.
- **Requested Action:**
  1. Header format: `X-SwiftDrop-Signature: t=<unix-timestamp>,v1=<hex-hmac-sha256>`; also sent: `X-SwiftDrop-Event-ID`, `X-SwiftDrop-Delivery-ID`.
  2. Run the mock receiver on a non-loopback container hostname in the test network (M8 provides a local fake receiver as the work division requires), or use the test-only allow-list that M8 exposes for CI.
  3. Retry expectations: 5 total attempts over about 42.5 minutes (0s, 30s, 2m, 10m, 30m, ±20% jitter). Tests should use a compressed schedule configured by environment variable.

---

### CR-AUTH-01: Edits to `docs/contracts/event-contracts.md`
- **Recipient:** Technical Lead (owner of authoritative docs), with M2, M3, M5
- **Current State vs. M8 design:**
  | Item | `event-contracts.md` | M8 design |
  | :--- | :--- | :--- |
  | IDs | `evt_…`, `prcl_…`, `corr_…` strings | UUIDv7 (`event_id`, `aggregate_id`); `outbox_events.id` is `UUID` |
  | Timestamp field | `timestamp` | `occurred_at` (work division says `occurred_at`) |
  | Schema version | `version` (int) | `schema_version` (string, e.g. `1.0.0`) plus `aggregate_version` |
  | Catalog | `merchant.webhook_dispatch.v1`; no `in_transit`, `returned`, `cancelled`, `courier.unassigned`, `merchant.*` | Add those events; remove `webhook_dispatch` (CR-M04-02) |
  | Payload | no `merchant_id`, no `notify` | See CR-M02-01 |
  | Routing | `parcel.*.v1` examples | exact `event_type` as key; queues bind `parcel.#`, `courier.#`, `merchant.#` |
  | DLQ | queue `swiftdrop.dlq` only | exchange `swiftdrop.dlq.exchange` (fanout) + queue `swiftdrop.dlq`, 30-day TTL |
  | `processed_events.event_id` | `VARCHAR(64)` | `UUID` |
- **Requested Action:** Update the file, or declare the M8 `doc-04` as authoritative and link it.

---

### CR-AUTH-02: Edits to `docs/architecture/database-schema.md`
- **Recipient:** Member 9 (database coordinator) / Technical Lead
- **Current State vs. M8 design:**
  - `outbox_events`: add `correlation_id`, `producer`, `occurred_at`; the shown ERD omits them.
  - `webhook_deliveries`: add `IN_PROGRESS` status and `lease_expires_at`; `event_id` is a plain UUID with no foreign key to `outbox_events` (outbox is purged at 7 days; delivery audit is kept 30 days); unique `(endpoint_id, event_id)`. Add child table `webhook_delivery_attempts`. Update index `idx_webhook_retry_queue` to include `IN_PROGRESS`/lease.
  - `notification_deliveries`: status `PENDING/IN_PROGRESS/SUCCESS/FAILED` (not `SENT`), `attempt_count` (not `attempts`), `lease_expires_at`, `UNIQUE (event_id, channel, recipient)`.
  - `processed_events`: add (`consumer_name`, `event_id UUID`) to the ERD and ownership table.
  - Migration file name for 000009 is `000009_create_worker_dedup_and_webhooks`.

---

## 3. Formal Architectural Decisions for Team Lead Sign-Off

### DEC-01: Single-Path Real SMTP (Mailpit in Local/CI)
- **Decision:** Remove the in-memory mock email adapter and keep one Go `net/smtp` path in all environments. Local development and CI use Mailpit (`localhost:1025`).
- **SRS Reference:** Deviation from FR-EVT-008, which allows a mock adapter; the SRS requires at least one notification channel and does not forbid a real one.
- **Trade-off:** This adds a Mailpit container to CI. The work division suggests a safe demo sink, which Mailpit is.
- **Recommendation:** **APPROVE**.

### DEC-02: Lease-Based Database-Driven Retries (No AMQP Requeue Loops)
- **Decision:** Retries are driven by PostgreSQL rows (`next_attempt_at`, `lease_expires_at`) claimed with `FOR UPDATE SKIP LOCKED`. `Nack` with `requeue=true` is forbidden.
- **Trade-off:** The worker now polls the database as well as consuming the queue; the DB pool budget in D8 covers this.
- **Recommendation:** **APPROVE**.

### DEC-03: Self-Contained Domain Event Payloads
- **Decision:** Events include `merchant_id` and a minimal `notify` block so the worker never queries other modules' tables.
- **Trade-off:** Recipient email is copied into the outbox payload (PII at rest for the 7-day outbox retention); M8 must purge on schedule and never log payloads.
- **Recommendation:** **APPROVE**.

### DEC-04 (new): Reconciler Scope and FR-REC-003
- **Decision needed:** FR-REC-003 asks the reconciler to detect "a parcel missing expected status-history or assignment records". That needs read access to `parcels`/history tables, which domain-boundaries §4.2 does not allow M8 to query directly.
- **Options:** (a) M2/M3 expose a read-only view or a `core-api maintenance` check that M8 calls; (b) M8 limits FR-REC-003 to outbox/delivery inconsistencies and records the gap as a deviation.
- **Recommendation:** **(a)**, with M2 owning a read-only consistency view granted to the reconciler role. Until agreed, M8 marks FR-REC-003 as *partially covered*.

---

## 4. Revision History

| Rev | Change |
| :--- | :--- |
| 2.0 | First submission of CR-M01 to CR-M10 and DEC-01 to DEC-03. |
| 2.1 | Fixed folder paths; aligned CR-M04-01 to M4's real view (`url`, `enabled`, `updated_at`); role ownership moved to M9 bootstrap (D6); recorded items already agreed with M9; added CR-AUTH-01/02, DEC-04, event-name decision (`delivery_failed`), `Write` vs `WriteEvent`/`Append`, and exchange name alignment; corrected CR-M02-02 and CR-M03-02 to quote what the drafts actually contain. |
