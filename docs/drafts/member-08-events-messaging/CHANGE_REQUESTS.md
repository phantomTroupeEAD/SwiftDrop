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
| CR-M01-02 | M1 | Regenerate seed password hashes with StandardPasswordHasher | Proposed |
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

### CR-M01-02: Regenerate Seed User Password Hashes with StandardPasswordHasher
- **Recipient Domain:** Member 01 (Identity, Access & Security)
- **Affected Documentation & Code:** `core-api/migrations/20261021000002_seed_mock_users.up.sql`
- **Current State:** Seeded password hashes in migration `20261021000002` were placeholder values rather than outputs of `StandardPasswordHasher` (iterative 10,000-round salted SHA-256). Consequently, HTTP authentication against PostgreSQL fails with `AUTH_INVALID_CREDENTIALS`.
- **Requested Action:** Member 01 must update migration `20261021000002_seed_mock_users.up.sql` with real hashes generated via `StandardPasswordHasher.HashPassword(...)` for all 5 canonical seeded accounts (`admin@swiftdrop.local` with `AdminPassword123!`, etc.).

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


---

## 5. Team Announcements & Checkpoints

### Checkpoint 1 Announcement: Events Library & OutboxWriter Ready for Consumption
- **Date:** 2026-11-01
- **Author:** Member 08 (Event-Driven Backend & Messaging)
- **Recipients:** Members 02 (Parcels & Pricing), 03 (Dispatch & Delivery), 04 (Merchants), Technical Lead

#### 1. Available Package & Import Paths
All team members can now import the transactional outbox interfaces and envelope definitions:
- Production interface & envelopes: `github.com/swiftdrop/core-api/internal/events`
- Testing mock writer: `github.com/swiftdrop/core-api/internal/events/eventstest`

#### 2. Key Interfaces & Constructors
- **Interface:** `events.OutboxWriter`
  ```go
  type OutboxWriter interface {
      Write(ctx context.Context, tx *sql.Tx, envelopes ...AnyEnvelope) error
  }
  ```
- **Envelope Constructor:**
  ```go
  env, err := events.NewEnvelope(
      events.ParcelBookedV1,    // Event type constant from events/catalog.go
      "parcel",                // Aggregate type: parcel, courier, merchant, or dispatch
      parcel.ID().String(),    // Aggregate ID (valid UUID)
      parcel.Version(),         // Aggregate version (>= 1)
      correlationID,            // Request tracing ID from context (X-Request-ID)
      "core-api.parcels",       // Producer identifier
      payload,                  // Strongly-typed domain payload struct
  )
  ```
- **Production Postgres Writer:** `events.NewPostgresOutboxWriter()`
- **Unit Testing Mock Writer:** `eventstest.NewInMemoryWriter()` (enforces exact same invariants without Postgres).

#### 3. Canonical Event Catalog Constants (`events/catalog.go`)
- Parcels: `ParcelBookedV1`, `ParcelAssignedV1`, `ParcelPickedUpV1`, `ParcelInTransitV1`, `ParcelOutForDeliveryV1`, `ParcelDeliveredV1`, `ParcelDeliveryFailedV1`, `ParcelReturnedV1`, `ParcelCancelledV1`
- Couriers: `CourierUnassignedV1`
- Merchants: `MerchantStatusChangedV1`, `MerchantApprovedV1`

#### 4. Critical Calling Rules (Architecture §4.3)
1. **Pass active `*sql.Tx`:** Always pass the active transaction committing the domain aggregate change.
2. **Correlation ID propagation:** Extract the incoming request ID from `platform/middleware.CorrelationID(ctx)` or the HTTP `X-Request-ID` header.
3. **Self-contained payloads:** Include `merchant_id` and the `notify` block (`recipient_name`, `recipient_email`) inside the payload for milestone events so the downstream worker avoids cross-boundary database queries.


---

### Checkpoint 1B Announcement: Core API Platform Packages Ready for Consumption
- **Date:** 2026-11-01
- **Author:** Member 08 (Event-Driven Backend & Messaging / Core API Foundation)
- **Recipients:** All Core API Developers (Members 01 through 04, 07, 10), Technical Lead

#### 1. Available Packages & Capabilities
All domain modules can now import standard platform helpers:
- `github.com/swiftdrop/core-api/internal/platform/config`: Fail-fast environment variable parsing (`String`, `Int`, `Bool`, `Duration`, `Require`, `AppEnv`). Never logs values.
- `github.com/swiftdrop/core-api/internal/platform/db`: Connection pool options with budget formula, `Querier` interface, and `WithTx(ctx, db, fn)` for automatic rollback on error or panic.
- `github.com/swiftdrop/core-api/internal/platform/middleware`: `RequestID` (`X-Request-ID` $\to$ context), `CorrelationID(ctx)`, `Recover`, `AccessLog`, and `BodyLimit`.
- `github.com/swiftdrop/core-api/internal/platform/httpx`: Canonical JSON response envelopes (`WriteJSON`, `WriteData`), canonical error responses (`WriteError`), strict request decoding (`DecodeJSON`), and cursor pagination (`EncodeCursor`, `DecodeCursor`).

#### 2. Key Developer Guidelines
1. **Correlation Tracing:** Pass `platform/middleware.CorrelationID(ctx)` to `EventEnvelope.CorrelationID` and when logging cross-cutting operations.
2. **API Responses:** Use `platform/httpx.WriteData` for single-resource responses and `platform/httpx.WriteError` for standard error envelopes (`api-conventions.md` §2).
3. **Transactions:** Use `platform/db.WithTx` for business operations that need transactional commits and automatic rollback on error or panic.

---

### Shared-File Edits Log (Phases 1 & 1B)

In compliance with `AGENTS.md` and `AI_HANDOFF_PLAN.md` §2.1, all shared-file edits are minimal, additive, and documented here:

1. **`core-api/cmd/api/main.go`**:
   - **Edit:** Wrapped `container.Mux` with `wrapGlobalMiddleware(container.Mux)` inside HTTP server instantiation.
   - **Purpose:** Applies global `RequestID`, `Recover`, `AccessLog`, and `BodyLimit` without altering any existing routes or handler behaviors.
2. **`core-api/cmd/api/wiring_events.go`** (New helper):
   - **Edit:** Added helper function `wrapGlobalMiddleware(http.Handler) http.Handler` instantiating platform middleware pipeline with structured JSON logger.
3. **`.golangci.yml`**:
   - **Edit:** Added `depguard` rules `platform_isolation` (forbids `internal/platform` from importing domain modules) and `core_api_no_broker` (forbids Core API from importing RabbitMQ AMQP directly).

---

### Checkpoint 2 Announcement: RabbitMQ Topology, Outbox Relay & Fixture Emitter Ready
- **Date:** 2026-11-01
- **Author:** Member 08 (Event-Driven Backend & Messaging)
- **Recipients:** Member 05 (Tracking API), Member 09 (DevOps/K8s), Member 10 (AI Assistant), Technical Lead

#### 1. RabbitMQ Topology & Queue Names
All exchanges and queues are declared with durable settings:
- **Primary Topic Exchange:** `swiftdrop.events.topic` (Topic, Durable)
- **Dead-Letter Exchange (DLX):** `swiftdrop.dlq.exchange` (Fanout, Durable)
- **Dead-Letter Queue (DLQ):** `swiftdrop.dlq` (`x-message-ttl: 2592000000` = 30 days)
- **Queues Declared:**
  - `q.tracking.updates` (bound with `parcel.#`, `courier.#`) $\to$ Dedicated for Member 05 Tracking API
  - `q.notifications.worker` (bound with `parcel.#`) $\to$ Consumed by Member 08 Notification Worker
  - `q.webhooks.dispatcher` (bound with `parcel.#`, `courier.#`, `merchant.#`) $\to$ Consumed by Member 08 Webhook Dispatcher
  - All worker queues configured with `x-dead-letter-exchange: swiftdrop.dlq.exchange`.

#### 2. Message Envelope & AMQP Headers
Published AMQP messages use `delivery_mode=2` (Persistent) and `content_type=application/json`:
- **AMQP Headers:**
  - `event_id`: UUIDv7 string matching the outbox event ID
  - `correlation_id`: Correlation ID passed from the initiating HTTP request
- **Message ID:** `event_id`
- **Routing Key:** exact `event_type` (e.g. `parcel.booked.v1`, `parcel.delivered.v1`)
- **JSON Payload Envelope Structure:**
```json
{
  "event_id": "0192e2b3-7c8a-7890-a123-000000000001",
  "event_type": "parcel.booked.v1",
  "aggregate_type": "PARCEL",
  "aggregate_id": "0192e2b3-7c8a-7890-a123-000000000010",
  "aggregate_version": 1,
  "schema_version": "1.0.0",
  "occurred_at": "2026-11-01T10:00:00Z",
  "correlation_id": "0192e2b3-7c8a-7890-a123-000000000099",
  "producer": "core-api",
  "payload": {
    "parcel_id": "0192e2b3-7c8a-7890-a123-000000000010",
    "tracking_number": "SD-20261101-0001",
    "merchant_id": "0192e2b3-7c8a-7890-a123-000000000042",
    "status": "BOOKED",
    "notify": {
      "recipient_name": "Kasun Perera",
      "recipient_email": "kasun.perera@example.lk"
    }
  }
}
```

#### 3. Local Development Infrastructure
A complete local stack is available via Docker Compose in `services/messaging/docker-compose.dev.yml`:
- PostgreSQL 16 on port `5432` (`swiftdrop_db`)
- RabbitMQ 3.12 Management on ports `5672` (AMQP) & `15672` (Management UI)
- Mailpit on ports `1025` (SMTP) & `8025` (Web UI)
Run with: `docker compose -f services/messaging/docker-compose.dev.yml up -d`

#### 4. Development Event Fixture Emitter (`devemit`)
For Member 05 (Tracking API) and local testing without running Member 02/03 booking flows:
```bash
cd services/messaging
go run ./cmd/devemit --type parcel.booked.v1 --count 5
```
Options:
- `--type <event_type>`: Fixture type (e.g. `parcel.booked.v1`, `parcel.out_for_delivery.v1`, `parcel.delivered.v1`)
- `--count <N>`: Number of events to stage into `outbox_events`
- `--merchant-id <UUID>`: Override merchant ID in payload
- `--email <email>`: Override recipient email for notification testing
When the Outbox Relay (`cmd/relay`) is running, staged rows are immediately published to RabbitMQ.


---

## 4. Shared-File Edits Log (Phase 8B Quality Gates & Alignment)

In compliance with `AGENTS.md` and Phase 8B instructions, all shared-file edits are strictly additive, non-breaking, and formally documented below:

### 1. `core-api/cmd/api/main.go`
- **Before:** Direct call to `srv.ListenAndServe()` without OS signal trapping or server connection draining; `WriteTimeout` was 15 seconds.
- **After:** Configured `&http.Server` with `ReadHeaderTimeout: 5s`, `ReadTimeout: 15s`, `WriteTimeout: 30s`, `IdleTimeout: 60s`. Added signal listener on `SIGINT` and `SIGTERM` triggering `srv.Shutdown(ctx)` with a 20-second drain window, followed by `container.DB.Close()`.
- **Note for Member 1:** The startup banner prints seeded accounts and passwords. This was preserved verbatim to avoid breaking local developer ergonomics, but Member 1 should gate password printing to `APP_ENV=local` in production builds.

### 2. `core-api/cmd/api/wiring.go` & `wiring_events.go`
- **Before:** Contained only legacy `/api/v1/health` endpoint.
- **After:** Added standard Kubernetes readiness and liveness probes:
  - `GET /health/live`: Always returns 200 OK `{"status":"ok"}`.
  - `GET /health/ready`: Probes PostgreSQL connection with a 1-second timeout. Returns 200 OK `{"status":"ok","database":"connected"}` if reachable. If unreachable, returns 503 `{"status":"degraded","database":"down"}` (in `APP_ENV=local` on mock fallback, returns 200 OK `{"status":"ok","database":"mock"}`).
  - Wired Member 08 events and Swagger UI routes via isolated `wiring_events.go`.

### 3. `core-api/internal/identity/transport/middleware.go`
- **Before:** Contained `GetPrincipal(ctx)` but had unexported `principalContextKey`, preventing other modules from injecting mock principals in unit tests without generating signed JWTs.
- **After:** Added standard `WithPrincipal(ctx context.Context, p *application.Principal) context.Context` helper. No existing middleware behavior or authentication checks were modified.

### 4. `.golangci.yml`
- **Before:** Contained `domain_purity` and `application_purity` rules.
- **After:** Added:
  - `core_api_no_broker`: Denies `github.com/rabbitmq/amqp091-go` inside `core-api/**` ("Core API must not publish to the broker; use the outbox").
  - `platform_isolation`: Denies domain module imports inside `core-api/internal/platform/**`.

### 5. `scripts/verify.ps1` & `scripts/verify.sh`
- **Before:** Only checked and ran tests for `core-api`.
- **After:** Extended to run `gofmt`, `golangci-lint`, and `go test` for `services/messaging` in parallel, maintaining identical Layer 3 quality gate enforcement.

### 6. `.github/workflows/ci.yml`
- **Before:** Contained jobs for secret scanning and `core-api` verification.
- **After:** Added `messaging-checks` job executing Go 1.22 setup, `gofmt` verification, `go vet`, `golangci-lint`, and `go test -race ./...` for `services/messaging`.

### 7. `core-api/migrations/20261021000002_seed_mock_users.down.sql`
- **Before:** Missing pairing file for `20261021000002_seed_mock_users.up.sql`.
- **After:** Added idempotent `DELETE` statements removing seeded mock users and role associations.
