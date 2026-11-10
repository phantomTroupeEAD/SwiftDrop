# Member 08 — Fixes Required in the Design Report PDF

> **Checked:** `SwiftDrop_System_Architecture_and_Design_Specification.pdf`, Member 08 sections (Chapter 11, pp. 87–94) and every place the report mentions your work (pp. 1–24, 96–97, 110–118, 129–133, plus the cross-references in Chapters 5–8).
> **Method:** I extracted the PDF text and compared each Member 08 statement with the implemented code (`core-api/internal/events`, `services/messaging`, the migrations) and with `AI_HANDOFF_PLAN.md`.
> **Limit — diagrams NOT checked visually.** This machine has no PDF page renderer, so I could read figure *captions* and surrounding text but not the drawings (Figures 11.1–11.6, 2.1, 3.1, 7.4, 7.8, 12.1, 12.2, 14.1, 14.2). §6 below lists what each diagram must show. To have them checked, send the figures as PNG/JPG images.

Priority: **P1** = factually wrong vs. the code (a marker can catch it), **P2** = inconsistent or incomplete, **P3** = wording.

---

## 1. Chapter 11 text (pp. 87–94)

### 1.1 §11.2 — component names and paths (P1)
| Report says | Reality | Fix |
| :--- | :--- | :--- |
| Outbox writer in `internal/events/outbox.go` with `AppendEvent(ctx, tx, event)` | `core-api/internal/events/writer.go` + `repository.go`; method is `OutboxWriter.Write(ctx, tx *sql.Tx, ...AnyEnvelope)` | Rename file and method. |
| Relay in `services/messaging/outbox_relay/` | `services/messaging/cmd/relay` (entrypoint) + `internal/relay` | Fix path. |
| Worker in `services/messaging/worker/` | `services/messaging/cmd/worker` + `internal/consumer`, `internal/notification`, `internal/webhook`, `internal/delivery` | Fix path. |
| "three decoupled components" followed by a numbered list of four | Four items are listed | Say "four components" or merge. |
| Routing keys `parcel.*, courier.*, merchant.*` | Bindings use `#`: `parcel.#`, `courier.#`, `merchant.#` (and `q.tracking.updates`: `parcel.#`, `courier.#`) | Use `.#`. |
| (missing) | The **Reconciler** (`cmd/reconcile`: stale-outbox audit, expired-lease reset, retention, DLQ replay) is a third binary | Add it to §11.2 (it appears only in Figure 11.1 per your page list and in §11.3's text). |
| Queues not named | `q.notifications.worker`, `q.webhooks.dispatcher`, `q.tracking.updates`, `swiftdrop.dlq` | Name them (Chapter 8 depends on `q.tracking.updates`). |

### 1.2 §11.3 — flows (P1)
- **Figure 11.4 text** says unparseable messages are "staged back into `outbox_events` by the Reconciler". **Wrong.** `reconcile dlq-replay --limit N` reads `swiftdrop.dlq` and republishes straight to `swiftdrop.events.topic` with header `x-replayed-from: swiftdrop.dlq`, acking the DLQ message only after the broker confirms. It never writes to `outbox_events`.
- Relay loop text ("claims in batches of 50 via SKIP LOCKED and only commits `PUBLISHED` upon broker ACK") is mostly right. Make it exact: the claim runs in a short transaction that sets a 60 s visibility lease (`next_attempt_at = now()+lease`); the broker publish happens **outside** any transaction; on confirm the row is set `PUBLISHED`; on failure `attempt_count+1` with backoff; after 5 failures `FAILED`. A crash after publish can cause a duplicate publish (hence consumer idempotency).

### 1.3 §11.4 — event contract example (P1)
The sample JSON does not match the implemented envelope:

| Report | Implemented |
| :--- | :--- |
| `"event_type": "v1.parcel.delivered"` | `"parcel.delivered.v1"` (pattern `^[a-z_]+\.[a-z_]+\.v[0-9]+$`) |
| `"aggregate_type": "PARCEL"` | `"parcel"` (lower-case; one of parcel, courier, merchant, dispatch) |
| `"timestamp"` | `"occurred_at"` |
| `"version": 1` | `"aggregate_version"` (integer ≥ 1) **and** `"schema_version": "1.0.0"` |
| (missing) | `"correlation_id"` (required, from `X-Request-ID`) |
| `"producer": "core-api/internal/delivery"` | e.g. `"core-api.parcels"` style; any module name string |
| payload contains `proof_photo_url` (raw S3 URL) | Contract forbids raw storage URLs, courier phone, private notes in events; use `proof_type` / `proof_id` |
| payload has `recipient_name` at top level | Customer contact data lives only in an optional `notify` block (`recipient_name`, `recipient_email`) that is **stripped** before webhooks are sent |
| "CloudEvents-compatible" | The envelope is **not** CloudEvents (no `specversion`, `source`, `id`, `type`). Say "SwiftDrop event envelope" or "CloudEvents-inspired". |

Use this example instead:
```json
{
  "event_id": "0192e4a1-9999-7abc-8def-0123456789ab",
  "event_type": "parcel.delivered.v1",
  "aggregate_type": "parcel",
  "aggregate_id": "0192e4a1-0001-7abc-8def-0123456789ab",
  "aggregate_version": 7,
  "schema_version": "1.0.0",
  "occurred_at": "2026-10-28T15:00:00Z",
  "correlation_id": "req_8f3c2a",
  "producer": "core-api.parcels",
  "payload": {
    "parcel_id": "0192e4a1-0001-7abc-8def-0123456789ab",
    "tracking_number": "SD-20261028-00912",
    "merchant_id": "0192e4a1-5555-7abc-8def-0123456789ab",
    "status": "DELIVERED",
    "proof_type": "PHOTO",
    "proof_id": "0192e4a1-7777-7abc-8def-0123456789ab",
    "delivered_at": "2026-10-28T14:59:45Z",
    "notify": { "recipient_name": "Bob Fernando", "recipient_email": "bob@example.lk" }
  }
}
```

### 1.4 §11.5 — data model (P1)
- **Migration names.** The report says `000008_…` / `000009_…`. The repository's migrator sorts timestamp-named files, so the implemented files are `20261101000050_create_outbox_events` and `20261101000500_create_worker_dedup_and_webhooks`. Fix here and in Ch. 14 (below), or add a sentence "logical steps 8 and 9; implemented with timestamped versions because the migrator orders by filename".
- **`outbox_events` columns are incomplete.** Real columns: `id` (UUID PK = the event id), `producer`, `occurred_at`, `correlation_id`, `aggregate_type`, `aggregate_id`, `aggregate_version`, `event_type`, `schema_version`, `payload` (JSONB), `status` (PENDING/PUBLISHED/FAILED), `attempt_count`, `next_attempt_at`, `last_error`, `created_at`, `published_at`. Report lacks correlation_id, versions, retry fields.
- **Index is wrong.** Report: `idx_outbox_pending ON outbox_events(status, created_at) WHERE status='PENDING'`. Real: `ON outbox_events (created_at ASC) WHERE status = 'PENDING'` (the status column is redundant inside a partial index). Same text appears in §14.5 (p. 115) and Table 14.1 (p. 112).
- **"`worker_deduplication`" does not exist.** The table is `processed_events` with a composite primary key `(consumer_name, event_id)`. The report's own §11.7 uses `processed_events … ON CONFLICT (consumer_name, event_id)`, which contradicts §11.5 (`event_id` as sole PK). Replace `worker_deduplication` everywhere (pp. 91, 112, 113, 114, ERD) and fix the PK.
- **Three tables are missing.** Add `notification_deliveries`, `webhook_deliveries` (unique `(endpoint_id, event_id)`), `webhook_delivery_attempts` (append-only). The report's "14-table" ERD (Fig. 14.1, p. 110) must either include them or state which tables it omits; "14 tables" will then be wrong — recount.

### 1.5 §11.6 — Table 11.1 and 11.2 (P3)
Both tables are fine technically. Two wording points: (a) the "Reliability / Operational Overhead / Payload Filtering" row labels are misaligned with the cell text in the PDF layout — check the rendered table; (b) Table 11.2 claims RabbitMQ "<80 MB RAM" and "<3 seconds startup" — unsourced numbers; either soften ("lightweight") or cite a source.

### 1.6 §11.7 — reliability (P1)
- **Webhook exhaustion does NOT go to the DLQ.** The report (here, in Table 7.1, in §7.7 item 3 and Fig. 7.8, in RSK-03) says that after 5 failed webhook attempts the message is routed to `swiftdrop.dlq` / `dlq.events`. Implemented behavior: retries are scheduled in PostgreSQL (`webhook_deliveries.next_attempt_at`); after the 5th attempt the delivery row becomes `FAILED` (kept for audit and manual retry via the admin API). The **DLQ receives only unparseable/poison messages** (rejected with `Nack(requeue=false)`). Rewrite accordingly and keep Fig. 11.4/7.8 consistent.
- **Dead-letter queue name:** it is `swiftdrop.dlq` (via `swiftdrop.dlq.exchange`, fanout, 30-day TTL). `dlq.events` (RSK-03, p. 132) does not exist.
- The idempotency description is mostly right. Add the two layers: (1) unique constraints on delivery rows (`ON CONFLICT DO NOTHING`) at consume time, (2) `processed_events (consumer_name, event_id)` written with the terminal outcome in the same transaction as the delivery update.
- Add the honest guarantee sentence: **at-least-once with idempotent effects; not exactly-once.**
- Add the missing mechanics a marker would expect: retry schedule **0 s, 30 s, 2 min, 10 min, 30 min with ±20 % jitter (5 attempts)**; outcome matrix (2xx success; 408/429/5xx/timeouts retry; other 4xx, redirects and SSRF blocks are terminal); webhooks never follow redirects; per-attempt timeout 5 s.

### 1.7 §11.7 / Fig. 11.6 — SSRF (P2)
- The report names the function `SafeDialContext`. The code uses a custom `DialContext` inside `webhook/client.go` (no function with that name). Either say "a custom `DialContext`" or ignore if the name is only conceptual.
- State the real policy: resolve DNS once, reject if **any** resolved IP is in 0.0.0.0/8, 10/8, 100.64/10, 127/8, 169.254/16, 172.16/12, 192.0.0/24, 192.168/16, 198.18/15, 224/4, 240/4, ::1, fe80::/10, fc00::/7, ff00::/8 or 64:ff9b::/96 (and IPv4-mapped forms), then dial the verified IP directly (TLS SNI preserved). Chapter 7 §7.7 lists only five RFC1918/loopback/metadata ranges — it describes M4's registration-time check; mention that M8 re-checks at **every delivery** (DNS-rebinding defense).

### 1.8 §11.8 — tests (P1)
- `tests/integration/outbox_test.go` does not exist. Real tests: `core-api/internal/events/*_test.go` (writer validation, golden JSON), `core-api/tests/` (cross-module smoke, events repository), `services/messaging/internal/{relay,consumer,delivery,notification,webhook,reconciler,sqlguard}/*_test.go`, `services/messaging/tests/` (end-to-end lifecycle + SQL schema contract; integration tests run with `INTEGRATION=1`).
- Add the acceptance criteria actually verified: rollback leaves no outbox row; 10 duplicate deliveries → one effect; poison message → DLQ without a hot loop; crash/lease recovery; RabbitMQ outage with no event loss; SSRF table test; HMAC known-answer vector.

---

## 2. Other places that describe your work

### 2.1 §1.2 objective 3, §1.3 actor 6, RTM BR-08 (pp. 14–16) — OK with small edits (P3)
Correct in substance. For BR-08 the "End-to-end integration tests asserting at-least-once publishing and idempotency" matches what exists, but mention they require Docker (`INTEGRATION=1`).

### 2.2 §2.4 (p. 20) — wrong actor (P1)
"the Event Worker pushes live telemetry to the Tracking API". Implemented: the **Tracking API consumes `q.tracking.updates` itself** (Member 5); the Event Worker does not call the Tracking API. Rewrite: "the Outbox Relay publishes to RabbitMQ; the Tracking API consumes `q.tracking.updates` and the Event Worker consumes `q.notifications.worker` and `q.webhooks.dispatcher`."

### 2.3 §3.1/§3.2/§3.4/§3.5 (pp. 21–24) — OK (P3)
Text matches (singleton relay, SKIP LOCKED). Add one explicit sentence: **the Outbox Relay must run as exactly one replica** (no horizontal scaling without leader election); the Event Worker scales (KEDA, queue backlog).

### 2.4 Chapter 12 (pp. 96–97) — your containers (P1/P2)
- **Metric name:** `outbox_pending_events_total gauge` → implemented as **`swiftdrop_outbox_pending_events`** (all metrics are prefixed `swiftdrop_`; a gauge should not end in `_total`). Also available: `swiftdrop_outbox_oldest_pending_age_seconds`, `swiftdrop_outbox_published_total`, `swiftdrop_outbox_publish_failures_total`, `swiftdrop_webhook_delivery_attempts_total`, `swiftdrop_notification_attempts_total`, `swiftdrop_notifications_skipped_total`, `swiftdrop_consumer_failures_total`, `swiftdrop_dlq_messages_routed_total`, `swiftdrop_dlq_depth_gauge`.
- **Probes/metrics ports** (relay 8082, worker 8083, endpoints `/health/live`, `/health/ready`, `/metrics`) — matches the report's figure labels; add the endpoint names.
- **Runtime image (§12.2, owned by Member 9):** report says every Go service uses `gcr.io/distroless/static-debian12:nonroot`. The messaging Dockerfile uses **`alpine:3.19`**, user `swiftdrop` UID 10001, three production binaries plus dev tools. Either change the Dockerfile to distroless (preferred for consistency with the report; confirm with Member 9) or correct the report sentence. Do not leave them contradicting.
- **Local compose (Fig. 12.1):** `services/messaging/docker-compose.dev.yml` runs only PostgreSQL, RabbitMQ and Mailpit. If Figure 12.1 shows relay/worker as compose services it describes Member 9's root compose, not yours — confirm it exists, otherwise fix the figure.

### 2.5 Chapter 14 (pp. 110–115) — see §1.4 (P1)
Fix: migration names, `worker_deduplication` → `processed_events`, partial-index definition, missing delivery tables, "14 tables" count. In the §14.4 list, step 9's description ("creates consumer message deduplication registries") should read "creates `processed_events`, `notification_deliveries`, `webhook_deliveries`, `webhook_delivery_attempts`". Figure 14.2: migration 9 depends on Member 4's `webhook_endpoints` (it has a foreign key to it) — the dependency chain must show that edge.

### 2.6 Chapter 15 (pp. 116–117) — claims the code does not back (P1)
- **ISP example `EventPublisher interface { Publish(ctx, event) error }`** — not in the code. The implemented small interface is `events.OutboxWriter` (`Write(ctx, tx, ...AnyEnvelope)`). Replace the example (also valid ISP evidence: `idempotency.Store`, `delivery` repositories, `broker` publisher interfaces — verify names before citing).
- **Observer Pattern in `internal/events/`** — the module is an outbox writer, not an observer. Observer is better attributed to the pub/sub topology (RabbitMQ topic exchange → consumers). Reword: "Publish/Subscribe via RabbitMQ".
- **Circuit Breaker Pattern in the Event Worker** — **not implemented** (no circuit-breaker code anywhere). What exists is bounded retries with jittered backoff, per-attempt timeout, and terminal failure classification. Either remove the claim or implement a breaker. Do not claim it in a graded report if it is not in the code.
- Patterns that **are** true and worth listing: Transactional Outbox, Repository, Idempotent Consumer, Strategy-like retry schedule/backoff calculator, Adapter (SMTP sender, AMQP publisher), Dead Letter Channel (EIP).

### 2.7 Chapter 16 STRIDE (p. 118) (P2)
HMAC-SHA256 on outbound webhooks is correct. Add the M8-owned mitigations: SSRF allow/deny and re-check at delivery time, AES-256-GCM secret decryption with endpoint-id AAD, no redirect following, secrets/emails/URLs never logged, email header-injection rejection, STARTTLS fail-closed outside local mode, DLQ for poison messages.

### 2.8 Chapter 18 (pp. 129–130) (P2)
Row/sheet for Member 8 says paths `services/messaging/outbox_relay/` and "created the Event Worker consumer". Update paths and deliverables to the real list: events library + `InMemoryWriter` for other modules, relay, worker (email + webhooks), reconciler, ops/admin endpoints and Swagger registry, platform packages (`internal/platform/*`), migration tests, quality-gate/CI extensions. Add the cross-module integration test harness (`core-api/tests`).

### 2.9 Chapter 19 (pp. 132–133) (P1/P2)
- **RSK-01 mitigation** says "SKIP LOCKED with parallel worker batching". The relay is a **strict singleton**; the mitigation is batching (50 rows), 500 ms polling, partial index, lag metric `swiftdrop_outbox_oldest_pending_age_seconds` and the stale-outbox audit in the reconciler. Parallel relay workers are explicitly prohibited.
- **RSK-03** mentions `dlq.events` and DLQ for webhook downtime — see §1.6 (webhook failures end as `FAILED` rows with manual retry; DLQ is for poison messages).
- **§19.3 roadmap** lists "Outbox Relay daemon, Event Worker, RabbitMQ" as Phase 2 work. They are implemented now (verify with the team whether Phase 2 text should say "delivered; remaining: live integration with Members 2/3/4 producers and Kubernetes deployment").

### 2.10 Chapter 7 (Merchants) statements about your worker (pp. 60–67) (P1)
- **Retry schedule disagreement inside the report:** Table 7.1 says "(1m, 5m, 15m)" while §7.7 says "(30s, 2m, 10m, 30m)". Implemented: **0 s, 30 s, 2 m, 10 m, 30 m** (five attempts total). Fix Table 7.1.
- "routing to Dead Letter Queues after 5 failures" — see §1.6.
- Webhook header: add `X-SwiftDrop-Timestamp`, `X-SwiftDrop-Event-ID`, `X-SwiftDrop-Delivery-ID` beside `X-SwiftDrop-Signature: t=…,v1=…` (signed string is `"{t}.{raw body}"`).
- Figure 7.4/7.8 and §7.7: the SSRF check that runs at delivery is the **worker's** dialer (see §1.7), in addition to Member 4's registration-time check.
- Figure 7.5 (ERD) references `webhook_deliveries [M8]` — correct; make sure the ERD shows `webhook_delivery_attempts` as its child.
- Tell Member 4 (their chapter): `subscribed_event_types` must contain exact event names such as `parcel.delivered.v1` (the worker matches exact strings).

### 2.11 Chapters 5, 6, 8 (pp. 37–55, 71–76) (P2)
- Chapters 5–6 say parcels/dispatch/delivery call your writer inside the booking/assignment/POD transaction — consistent with `OutboxWriter.Write(ctx, tx, …)`. Make sure they use the name `OutboxWriter.Write` (not `AppendEvent`/`WriteEvent`) and event names `parcel.booked.v1`, `parcel.assigned.v1`, `parcel.picked_up.v1`, `parcel.out_for_delivery.v1`, `parcel.delivered.v1`, `parcel.delivery_failed.v1`, `parcel.returned.v1`, `parcel.cancelled.v1`. Report text in Ch. 11 and elsewhere also uses `courier.assigned` and `attempt.failed` (§11.6, p. 92) — neither is a catalog name; use `parcel.assigned` and `parcel.delivery_failed`.
- Chapter 8 pages should name the queue **`q.tracking.updates`** and the exchange `swiftdrop.events.topic`. My text search for `q.tracking` returned no hit in the whole PDF, so this binding is either only inside a diagram or missing. Add it in text.

---

## 3. Items outside your chapters that I noticed (tell the owners)
- **Chapter 4 / ADR-003 say Argon2id; the implemented code uses salted SHA-256 with 10,000 rounds** (Chapter 17 itself says SHA-256). The report contradicts itself — Member 1.
- Chapter 17 lists "pre-configured test accounts"; the seeded password hashes in the migration do not match the hasher (logins return 401 against a real database) — Member 1 (change request CR-M01-02).
- Merchant JWT tenant IDs are strings (`mrc_…`) while merchant IDs are UUIDs — merchant delivery endpoints return 403 until aligned — Members 1/4.

## 4. Things the report is MISSING that your plan implemented (add if space allows)
1. Reconciler (stale-outbox audit, lease reset, retention windows 7/30/35 days, DLQ replay).
2. Email notification channel (real SMTP, Mailpit locally, templates for booked / out_for_delivery / delivered / delivery_failed / returned, header-injection defense).
3. Ops/admin API and Swagger hub (`/api/v1/admin/events/*`, `/api/v1/merchant/webhook-deliveries*`, `/swagger/`) and the Swagger registry other modules can use.
4. Observability: the metric list above and health endpoints.
5. Platform foundation you added to Core API: request-ID middleware (`correlation_id`), transaction helper, health endpoints, graceful shutdown, SQL schema-contract tests.

## 5. Consistency checklist before you re-export the PDF
- [ ] Search the whole document for: `AppendEvent`, `outbox.go`, `outbox_relay`, `worker_deduplication`, `000008`, `000009`, `dlq.events`, `v1.parcel.delivered`, `courier.assigned`, `attempt.failed`, `outbox_pending_events_total`, `EventPublisher`, `Circuit Breaker`, `SafeDialContext`, "CloudEvents", "(1m, 5m, 15m)" — each must be fixed or removed.
- [ ] Event names use `<aggregate>.<action>.v1` everywhere.
- [ ] DLQ is described the same way in Ch. 7, 11, 13 (Fig. 13.7, SEQ-QA-04 — check that figure too), 19.
- [ ] Table/figure numbers and the List of Figures/Tables still match after edits.
- [ ] Any claim marked "verified"/"100 %" is backed by a test you can name; mention `INTEGRATION=1` for the Docker-based ones and that `-race`/`golangci-lint` run in Linux CI.

## 6. Diagram checklists (I could not view the images)
- **Fig. 11.1:** Core API (writes outbox in same txn) → PostgreSQL `outbox_events`; Relay (singleton) reads/marks; RabbitMQ `swiftdrop.events.topic`; Worker (email → SMTP/Mailpit, webhooks → merchant); Reconciler (CronJob, no broker access except DLQ replay); Tracking API consuming `q.tracking.updates`. Core API must have **no arrow to RabbitMQ**.
- **Fig. 11.2:** exchange `swiftdrop.events.topic` (topic, durable); queues `q.tracking.updates` (`parcel.#`,`courier.#`), `q.notifications.worker` (`parcel.#`), `q.webhooks.dispatcher` (`parcel.#`,`courier.#`,`merchant.#`); DLX `swiftdrop.dlq.exchange` (fanout) → `swiftdrop.dlq` (30-day TTL).
- **Fig. 11.3:** short claim transaction with lease → publish with confirm (outside txn) → mark `PUBLISHED`; failure branch: `attempt_count+1`, backoff, `FAILED` after 5.
- **Fig. 11.4:** poison message → `Nack(requeue=false)` → DLQ; replay → republish to exchange with `x-replayed-from` (not into `outbox_events`).
- **Fig. 11.5 / 7.4 / 7.8 / 13.7:** consume → write `webhook_deliveries` (unique per endpoint+event) → ack; poller claims row with lease → decrypt secret (AES-256-GCM) → SSRF check on the dialed IP → HMAC-signed POST (5 s timeout) → record attempt; retry 30 s/2 m/10 m/30 m; terminal `FAILED` row (not DLQ) after 5 attempts; 410/other 4xx terminal immediately.
- **Fig. 11.6:** resolve DNS once → any IP in blocklist? → reject (terminal) else dial that IP; no redirects.
- **Fig. 14.1/14.2:** include the four delivery/dedup tables and the FK `webhook_deliveries.endpoint_id → webhook_endpoints.id`; migration 9 after Member 4's `webhook_endpoints`.
- **Fig. 12.1/12.2:** relay `:8082` (1 replica), worker `:8083` (KEDA-scaled), RabbitMQ `:5672`, Mailpit `:1025` (SMTP) / `:8025` (UI), PostgreSQL `:5432`.
