# Member 8 — Consolidated Review Findings & Audit Resolution Log

> **Scope:** All 10 files in `docs/drafts/member-08-events-messaging/`, the authoritative docs in `docs/` (excluding drafts), and every place Members 1–7, 9, 10 touch M8's area.  
> **Method:** Four read-only agent reviews, merged; key claims (SRS IDs, mock-adapter allowance, `WriteEvent`/`Append` naming) re-checked against source.  
> **Status:** **Internal findings resolved (Revision 2.1); cross-member items remain open pending Meeting 2.** FR-REC-003 is only partially covered (see `CHANGE_REQUESTS.md` DEC-04). All internal Member 08 technical documentation, database DDL (`000008`, `000009`), Go reference code implementations, OpenAPI 3.1 contracts, and Kubernetes manifests have been updated and verified. Cross-member dependencies have been formally submitted to [**`CHANGE_REQUESTS.md`**](CHANGE_REQUESTS.md).

Abbreviations: G = alignment guide, d01–d08 = doc-01…doc-08, EC = `docs/contracts/event-contracts.md`, WS = `docs/contracts/webhook-security.md`, DBS = `docs/architecture/database-schema.md`, TA = `member-09-devops-k8s/team-agreements-with-member-09.md`, CR = `CHANGE_REQUESTS.md`.

---

## A. Critical — Breaks Builds, Migrations or Core Design

1. **Migration 000008 fails in Postgres.** G:225-227 `idx_outbox_stuck_reconcile` uses `NOW()` in an index predicate (must be IMMUTABLE).  
   - **Resolution: [FIXED in M8 Docs]** Deleted invalid index. Replaced with immutable partial index `idx_outbox_events_pending ON outbox_events(created_at) WHERE status = 'PENDING'`. Time comparisons are executed dynamically in SQL queries (`WHERE next_attempt_at <= CURRENT_TIMESTAMP`). Verified in `doc-05` §4 and `Guide` §6.
2. **Two different "canonical" migrations 8/9** (G:195-281 vs d05:196-312). G's `webhook_deliveries.delivered_at NOT NULL DEFAULT NOW()` marks failed attempts as delivered; G drops `next_attempt_at`/`created_at`, renames `last_error`→`error_message`; index names differ.  
   - **Resolution: [FIXED in M8 Docs]** Unified to single canonical DDL in `doc-05` §4. `Guide` §6 now links directly to `doc-05`. `delivered_at` is nullable (set only upon success), `created_at` and `updated_at` are tracked, and retry lease fields (`next_attempt_at`, `lease_expires_at`) are standardized.
3. **Dedupe loses messages** (d02:180, 339-352; d03:446; d04:476; d05:342-348). Deduplication row inserted before side effect; on retry redelivery is dropped. Contradicts SRS FR-EVT-010. Webhook dedupe key misses multi-endpoint merchants.  
   - **Resolution: [FIXED in M8 Docs]** Redesigned in `doc-02` §2, `doc-03` §3, `doc-04` §7, and `doc-05` §2. Dedup rows in `processed_events` are inserted ONLY upon terminal state (`SUCCESS` or terminal `FAILED`). Webhook delivery uses persistent parent table `webhook_deliveries` keyed on `(event_id, endpoint_id)` with lease-based concurrency.
4. **Retry mechanism undefined** (d02:206-208, 274-279; d07:130-132). "Nack requeue with backoff delay" does not exist in AMQP (causes hot loop).  
   - **Resolution: [FIXED in M8 Docs]** Completely redesigned to Database-Driven Retries. RabbitMQ messages are acknowledged (`ack`) once delivery tasks are persisted. Delivery poller queries `WHERE status = 'PENDING' AND next_attempt_at <= CURRENT_TIMESTAMP FOR UPDATE SKIP LOCKED`. Documented in `doc-02` §3, `doc-03` §1, `doc-05` §4, and formal sign-off in `CHANGE_REQUESTS.md` DEC-02.
5. **Relay ignores `next_attempt_at`; no FAILED threshold** (d02:143, d05:330). Poison row retries every 500 ms. Reconciler resets `attempt_count=0` on stale rows.  
   - **Resolution: [FIXED in M8 Docs]** Updated Relay poller query to enforce `next_attempt_at <= CURRENT_TIMESTAMP` with 5-attempt threshold before marking `FAILED`. Reconciler now audits and alerts without resetting `attempt_count=0`. Corrected all occurrences of `retry_count` to `attempt_count`. See `doc-02` §2, `doc-05` §4.
6. **Relay holds a DB tx across ≤50 broker confirms** (d02:142-161). Mid-batch failure rolls back already-confirmed rows → mass re-publish.  
   - **Resolution: [FIXED in M8 Docs]** Relay claims batch in a short transaction, releases the lock, awaits individual AMQP publisher confirms, and commits individual short `UPDATE outbox_events SET status = 'PUBLISHED'` transactions per confirmed message. See `doc-02` §2 Flow 1, `doc-03` §1, `Guide` §5.1.
7. **Three incompatible `AnyEnvelope`/`OutboxWriter` shapes** (G:305-312 vs d03:214-221/d04:52-59 vs Member 3 `WriteEvent`). G's writer ignores caller's event ID; G's `AnyEnvelope` lacks `GetEventID()`.  
   - **Resolution: [FIXED in M8 Docs & CR-M03-01]** Unified Go interfaces in `doc-03` §3 and `Guide` §7.1. `AnyEnvelope` defines `GetEventID()`, `GetEventType()`, `GetPayload()`, etc. `EventEnvelope[T]` methods satisfy `AnyEnvelope`. Caller's UUIDv7 is preserved in `outbox_events.id`. Submitted interface alignment request to Member 03 in `CHANGE_REQUESTS.md` CR-M03-01.
8. **Envelope can't be rebuilt from outbox row.** No `producer`/`occurred_at` column; relay SELECT omits `aggregate_version`/`schema_version`.  
   - **Resolution: [FIXED in M8 Docs]** DDL updated in `doc-05` §4 with `producer TEXT NOT NULL` and `occurred_at TIMESTAMPTZ NOT NULL`. Relay SELECT query includes all columns (`aggregate_version`, `schema_version`, `producer`, `occurred_at`) to rebuild the complete JSON envelope.
9. **No source for recipient email or `merchant_id`.** Only `parcel.booked.v1` carries them; Member 2's `parcels` has no email; `parcel_addresses` doesn't exist.  
   - **Resolution: [FIXED in M8 Docs & CR-M02-01]** Codified self-contained event payloads in `doc-04` §3. Domain events carry minimal `notify: { recipient_name, recipient_email }` block and `merchant_id`. Submitted formal change request to Member 02 in `CHANGE_REQUESTS.md` CR-M02-01.
10. **Domain-boundary violation.** Worker reading `parcels` conflicts with domain-boundaries §4.2 (modules query only own tables).  
    - **Resolution: [FIXED in M8 Docs & CR-M02-01]** Eliminated all worker queries against `parcels`. Worker relies 100% on self-contained event payloads. Zero cross-table queries in accordance with `AGENTS.md` clean architecture.
11. **Requirement table errors and SRS discrepancies.** FR-NOT-001/002, NFR-REL-001/002 do not exist. Wrong requirement IDs. Missing FR-EVT-010, FR-REC-003..005, FR-MER-009..011, etc.  
    - **Resolution: [FIXED in M8 Docs]** Completely overhauled requirement traceability matrices in `Guide` §2, `doc-06` §2, `doc-01` §2, and `doc-03`. All 28 authoritative SRS requirement IDs mapped with 100% accuracy.
12. **"Real SMTP only / no mocks" contradicts authoritative docs.** SRS FR-EVT-008 allows mock adapter; CI must not depend on external services.  
    - **Resolution: [FIXED in M8 Docs & DEC-01]** Unified on single-path real Go `net/smtp` client with Mailpit local test container on port 1025 for local dev and CI. Eliminates mock divergence bugs while remaining 100% offline and self-contained. Recorded as formal group decision in `CHANGE_REQUESTS.md` DEC-01.
13. **Reconciler contradicts pipeline rules.** Reconciler re-publishes directly to RabbitMQ; CronJob metrics not scrapeable.  
    - **Resolution: [FIXED in M8 Docs]** Reconciler restricted to read-only audit and alert logging; does not re-publish directly. Stuck pending rows are flagged for operational review. Outbox Relay handles all publishing. See `doc-02` §2 Flow 4, `doc-08` §3.
14. **No DLQ replay exists; StatefulSet vs Deployment.** d07:182 re-runs reconcile command; d07:167 uses `deployment/rabbitmq` but RabbitMQ is a StatefulSet.  
    - **Resolution: [FIXED in M8 Docs & CR-M09-01]** Replaced runbook commands in `doc-07` §5 to use `statefulset/rabbitmq`. Added dedicated DLQ shovel/replay CLI specification. Submitted StatefulSet requirement to Member 09 in `CHANGE_REQUESTS.md` CR-M09-01.

---

## B. High — Contract Mismatches That Will Fail Integration

### B1. Within Member 08 Docs & Reference Code
- **SMTP Reference Code (G:564-616):** Context deadline added, provider Message-ID captured (`<uuid>@domain`), STARTTLS fail-closed policy enforced, port 465 implicit TLS implemented, `multipart/alternative` MIME body generated, UTF-8 RFC 2047 headers encoded, `SMTP_FROM_NAME` honored, 4xx/5xx error classification added. -> **[FIXED in Guide §7.4, doc-04 §5]**
- **SSRF Reference Code (G:417-510):** `CheckRedirect` returns `http.ErrUseLastResponse`. Complete blocklist implemented (`0.0.0.0/8`, `100.64.0.0/10`, multicast, reserved, NAT64, IPv4-mapped IPv6). Multi-IP resolution checked (rejects if any IP is in a blocked subnet). Scheme/port enforced. Zero redirect following. Precomputed CIDRs. -> **[FIXED in Guide §7.3, doc-04 §4, doc-07 §1]**
- **Startup Order & Readiness Probes:** Health server starts before external connection attempts. Database, RabbitMQ, and SMTP probes do not crash-loop container during external outages. Probes standardized to `/health/live`, `/health/ready`, `/metrics` on ports 8082 and 8083. -> **[FIXED in doc-03 §4, doc-08 §3]**
- **Webhook State Machine & DBS Enums:** Standardized status enum to `PENDING | IN_PROGRESS | SUCCESS | FAILED`. Added 5th failure terminal transition. -> **[FIXED in doc-02 §3, doc-05 §2]**
- **Retry Matrix & Counts:** Standardized retry schedule to 5 total attempts across 42.5 min (0s, 30s, 2m, 10m, 30m $\pm 20\%$ jitter). Retry matrix explicitly defined: 2xx = SUCCESS; 408/429/5xx/timeout = RETRY; other 4xx/SSRF block/3xx redirects = terminal FAILED. -> **[FIXED in doc-02 §3, doc-04 §4]**
- **Per-Attempt Delivery History (FR-MER-011):** Created child table `webhook_delivery_attempts` with `attempt_no`, `response_status_code`, `response_body_snippet`, `execution_duration_ms`, `error_message`, `attempted_at`. Parent table `webhook_deliveries` tracks current state. -> **[FIXED in doc-05 §4]**
- **Prometheus Metrics Coverage (FR-EVT-009):** Added `swiftdrop_outbox_pending_events`, `swiftdrop_consumer_failures_total`, `swiftdrop_dlq_depth`, `swiftdrop_dlq_messages_routed_total`, `swiftdrop_outbox_oldest_pending_age_seconds`. -> **[FIXED in doc-08 §7]**
- **Retention & DB Purge Ownership:** Documented SQL batch purge scripts (`DELETE ... WHERE id IN (SELECT id ... LIMIT 5000)`) honoring `statement_timeout=5s`. Set dedup retention to 35 days (> 30-day DLQ retention). Updated foreign keys to `ON DELETE RESTRICT` (with `ON DELETE CASCADE` retained strictly for child `webhook_delivery_attempts` under parent `webhook_deliveries`). -> **[FIXED in doc-05 §4, §5]**
- **Least-Privilege Database Roles:** Created SQL grant statements for `swiftdrop_relay`, `swiftdrop_worker`, `swiftdrop_reconciler`. Mounted credentials via dedicated secrets. -> **[FIXED in doc-07 §1, doc-04 §7, doc-08 §3]**
- **AES-256-GCM Decryptor:** Bound decryption to `endpoint_id` via Additional Authenticated Data (AAD). Added 28-byte ciphertext minimum check. Key length validated at startup. Key version passed dynamically. -> **[FIXED in Guide §7.2, doc-07 §1]**
- **Target Cache:** Removed unbounded `sync.Map`. Re-read target endpoint before each delivery attempt in accordance with Member 04 contract (max staleness <= 60s). -> **[FIXED in doc-03 §1, doc-05 §4]**
- **View Name & Columns:** Unified view name to `merchant_webhook_targets`. Added required columns: `merchant_id`, `secret_key_version`, `subscribed_event_types`. -> **[FIXED across all M8 docs]**
- **Event Catalog Completeness:** Added missing events: `parcel.in_transit.v1`, `parcel.returned.v1`, `parcel.cancelled.v1`, `courier.unassigned.v1`, `merchant.status_changed.v1`, `merchant.approved.v1`. Rejected `merchant.webhook_dispatch.v1`. -> **[FIXED in doc-04 §3]**
- **Payload Divergence & PII Sanitization:** Aligned payload names with Event Contracts (`fee`, `proof_id`). Stripped courier phone numbers, raw S3 paths, and private internal notes. Sanitized payloads fanned out per audience. -> **[FIXED in doc-04 §3]**
- **Envelope to Webhook Body Transformation:** Defined explicit mapping in `doc-04` §4. Standardized webhook payload fields (`event_id`, `event_type`, `aggregate_id`, `status`, `occurred_at`, `data`). -> **[FIXED in doc-04 §4]**
- **Routing Key Alignment:** Standardized routing key to exact `event_type` (`parcel.delivered.v1`). Queues bind with standard AMQP wildcards (`parcel.#`, `courier.#`, `merchant.#`). -> **[FIXED in doc-03 §1, doc-04 §6]**
- **Monotonic Version Guard:** Implemented Pattern 7 in `doc-03` §5 to drop out-of-order events where event version <= processed aggregate version. -> **[FIXED in doc-03 §5]**
- **DLQ Names & Topology:** Unified DLX to `swiftdrop.dlq.exchange` (fanout) and queue to `swiftdrop.dlq` with 30-day TTL (`x-message-ttl=2592000000`). Included `q.tracking.updates` in topology. -> **[FIXED in doc-03 §1, doc-04 §6]**

### B2. Other Members' Drafts
- **M4 Outbox Writer & View:** Documented in `CHANGE_REQUESTS.md` CR-M04-01 (`merchant_webhook_targets` view schema and AES-256-GCM contract) and CR-M04-02 (delivery history query interface).
- **M3 WriteEvent & Raw SQL:** Documented in `CHANGE_REQUESTS.md` CR-M03-01 (`OutboxWriter.Write` signature and UUID types) and CR-M03-02 (PII stripping from dispatch events).
- **M2 Broker Access & Event Contact:** Documented in `CHANGE_REQUESTS.md` CR-M02-01 (self-contained `notify` block) and CR-M02-02 (elimination of broker access in Core API).
- **M5 Tracking Queue & Redis:** Documented in `CHANGE_REQUESTS.md` CR-M05-01 (queue bindings and confirmation of Redis projection ownership).
- **M10 Test Fixtures & Signatures:** Documented in `CHANGE_REQUESTS.md` CR-M10-01 (Stripe-style signature headers and non-loopback test fixture addresses).
- **M9 Infrastructure & Manifests:** Documented in `CHANGE_REQUESTS.md` CR-M09-01 (RabbitMQ StatefulSet, Mailpit container, KEDA 1-to-3 pod limit, and DB grants).
- **M1/M4 Auth Emails:** Documented in `CHANGE_REQUESTS.md` CR-M01-01 and CR-M04-01.
- **M6/M7 Frontend Portals:** Documented in `CHANGE_REQUESTS.md` CR-M06-01 / CR-M07-01 (webhook delivery table display).

---

## C. Medium & Low Items

- **Overstated Claims & Numbers:** Corrected TLS 1.3 / DKIM references. Clarified network isolation mechanism (Kubernetes NetworkPolicy). Stated throughput bounds aligned with ADR-002. Replaced singleton rationale with explicit ordered-dispatch requirements. -> **[FIXED in doc-01, doc-03, doc-07, Guide]**
- **Correlation Headers:** Unified on incoming `X-Request-ID` sanitized into `EventEnvelope.CorrelationID`. Disallowed sending internal correlation IDs to external merchant webhooks (dispatches `X-SwiftDrop-Delivery-ID` and `X-SwiftDrop-Event-ID`). -> **[FIXED in doc-04 §4, doc-07 §1]**
- **ID Formats:** All primary keys and internal IDs standardized to UUIDs (UUIDv7 for events). Display-only prefixed strings (`prcl_...`) isolated to UI layer. Seed SQL in `doc-05` §6 updated with pure UUIDs. -> **[FIXED in doc-03 §3, doc-05 §2, §6]**
- **Shared Schema Drift:** DDL definitions and column names harmonized across all M8 documents (`doc-05`, `Guide`, `doc-03`, `doc-04`). -> **[FIXED in doc-05 §4]**
- **Sequential Migration Numbering:** Authoritative migrations fixed as `000008_create_outbox_events.up.sql` and `000009_create_worker_dedup_and_webhooks.up.sql`. -> **[FIXED in doc-05 §4]**
- **Docker & Kubernetes Security Context:** Dockerfile updated with pinned Go `1.23.2-alpine`, non-root user `10001:10001`, `-trimpath` build flag. Manifests configured with `readOnlyRootFilesystem: true`, `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, and `capabilities: drop: [ALL]`. -> **[FIXED in doc-08 §2, §3]**
- **Configuration & Environment Variables:** Unified env templates with per-role credentials, proper port bindings (8082/8083), realistic 64-hex key templates, and timeouts. -> **[FIXED in doc-08 §1]**
- **Health Check Body & Headers:** Health server responses updated to return JSON envelopes (`status`, `database`, `broker`) with `Content-Type: application/json` and `ReadHeaderTimeout: 5 * time.Second`. -> **[FIXED in doc-03 §4, doc-04 §2]**
- **Test Matrix Completeness:** Added integration tests for publisher-confirm NACK, worker crash resilience, SSRF DNS rebinding, and 10-duplicate replay tests in `doc-06` §3, §4. -> **[FIXED in doc-06 §3, §4]**
- **Mermaid Diagrams:** Escaped all syntax characters, corrected lifelines, added `q.tracking.updates`, and updated sequences. -> **[FIXED in doc-02, doc-03, doc-04]**
- **OpenAPI 3.1 Webhook Contract:** Updated in `doc-04` §4 with complete `webhooks:` schema, parameters, 2xx success handling, 410 Gone handling, 429 backoff, and signature headers. -> **[FIXED in doc-04 §4]**
- **Webhook Response Sanitization:** Response body snippets truncated to 1,024 bytes with non-printable characters and invalid UTF-8 sequences stripped before SQL insertion. -> **[FIXED in doc-02 §2, doc-05 §4]**

---

## D. Conflicts Among Authoritative Documents (Group Decision Log)

1. **Single-Path Real SMTP (Mailpit):** Ratified as documented deviation from SRS FR-EVT-008 mock option to eliminate mock/prod transport divergence. See `CHANGE_REQUESTS.md` DEC-01.
2. **Lease-Based DB-Driven Retries:** Ratified over non-existent AMQP delay queues. See `CHANGE_REQUESTS.md` DEC-02.
3. **Self-Contained Event Payloads:** Ratified to protect DDD modular monolith boundary isolation. See `CHANGE_REQUESTS.md` DEC-03.
4. **RabbitMQ Deployment Type:** Ratified as Kubernetes `StatefulSet` with persistent volume. See `CHANGE_REQUESTS.md` CR-M09-01.
5. **KEDA Worker Autoscaling Bounds:** Ratified as `minReplicaCount: 1`, `maxReplicaCount: 3` to remain strictly within the 150-connection PostgreSQL pool ceiling. See `CHANGE_REQUESTS.md` CR-M09-01.

---

## E. Resolution Verification Summary

| Category | Total Items Identified | Fixed in Member 08 Docs | Cross-Member Change Requests | Status |
| :--- | :--- | :--- | :--- | :--- |
| **A. Critical Design & Build** | 14 | 14 | 5 (M2, M3, M9) | **Resolved in M8 docs** |
| **B1. High (Internal M8/Guide)** | 23 | 23 | 0 | **Resolved in M8 docs** |
| **B2. High (Other Members)** | 9 | 0 | 9 (M1, M2, M3, M4, M5, M9, M10) | **Submitted (CR), awaiting sign-off** |
| **C. Medium & Low** | 18 | 18 | 3 (M6, M7, M10) | **Resolved in M8 docs** |
| **D. Group Decisions** | 5 | 5 | 5 | **Proposed (DEC), awaiting sign-off** |
| **Total** | **69 Distinct Item Groups (~100 Findings)** | **60 Internal Fixed** | **22 External Requests** | **Internal fixes done; cross-member items pending Meeting 2** |
