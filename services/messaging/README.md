# SwiftDrop Messaging Service (`services/messaging`)

> **Owner:** Member 08 — Event-Driven Backend & Integrations  
> **Status:** Production Ready (Phases 2–9 Completed)

---

## 1. Module Purpose & Capabilities
The **SwiftDrop Messaging Service** provides the asynchronous integration backbone for the SwiftDrop courier system. It encompasses:
- **Outbox Relay (`cmd/relay`):** Reliable at-least-once publishing of transactional outbox events to RabbitMQ with publisher confirms and exponential backoff.
- **Event Worker (`cmd/worker`):** Idempotent consumption of domain events, customer milestone email dispatch (real SMTP / STARTTLS / TLS / Mailpit), and signed HTTP merchant webhook delivery.
- **Reconciliation Engine (`cmd/reconcile`):** Scheduled audit, expired worker lease recovery, retention purges, and dead-letter queue (DLQ) replay.
- **Developer Tools (`cmd/devemit`, `cmd/fake-receiver`):** Local CLI tools for emitting canonical test events and receiving/verifying HMAC webhook signatures.

---

## 2. Business Invariants & Rules
1. **Transactional Outbox Guarantee:** Business data state mutations and outbox records commit in the exact same PostgreSQL transaction.
2. **Zero In-Transaction Network I/O:** Database transactions never block on external network I/O (SMTP or HTTP webhooks).
3. **Strict Outbox Relay Singleton:** Exactly one outbox relay instance runs (`replicas: 1`) to guarantee ordered event sequence per aggregate and prevent connection contention.
4. **Idempotent Consumers:** All consumers must be strictly idempotent using `processed_events` deduplication (`consumer_name` + `event_id`).
5. **SSRF Hardened Webhooks:** Outbound webhooks block all non-public CIDRs (private, loopback, link-local, cloud metadata `169.254.169.254`, CGNAT, IPv6 ULA) and disable HTTP redirect following.
6. **Payload Sanitization (Decision D4):** Customer email addresses in delivery records are masked on read (`k***@domain.com`). No passwords, tokens, API keys, courier phone numbers, or private delivery notes may ever be exposed via merchant webhooks or tracking queues.
7. **Dead-Letter Safety:** Poison messages that fail JSON schema validation or unrecoverable processing are rejected directly to `swiftdrop.dlq` without tight retry loops.
8. **Resilient Broker Connection & Cold-Start Recovery:** RabbitMQ connection manager retries with exponential backoff on cold starts and starts an asynchronous background retry loop if the initial dial fails. Outbox relay and worker daemons wait on broker readiness before polling or opening consumer channels, preventing crash loops and false outbox delivery failures during infrastructure boots.

---

## 3. RabbitMQ Topology & Contracts
- **Primary Topic Exchange:** `swiftdrop.events.topic` (Topic, Durable)
- **Dead-Letter Exchange (DLX):** `swiftdrop.dlq.exchange` (Fanout, Durable)
- **Dead-Letter Queue (DLQ):** `swiftdrop.dlq` (`x-message-ttl: 2592000000` = 30 days)
- **Work Queues:**
  | Queue Name | Routing Keys | DLX | Purpose |
  | :--- | :--- | :--- | :--- |
  | `q.notifications.worker` | `parcel.#` | `swiftdrop.dlq.exchange` | Customer milestone transactional emails |
  | `q.webhooks.dispatcher` | `parcel.#`, `courier.#`, `merchant.#` | `swiftdrop.dlq.exchange` | Outbound HMAC-signed merchant webhooks |
  | `q.tracking.updates` | `parcel.#`, `courier.#` | `swiftdrop.dlq.exchange` | Projection stream for Member 05 Tracking API |

---

## 4. Internal Layer Organization
```
services/messaging/
├── cmd/
│   ├── relay/main.go             # Outbox Relay entrypoint (probe port 8082)
│   ├── worker/main.go            # Event Worker entrypoint (probe port 8083)
│   ├── reconcile/main.go         # Reconciliation & Retention CLI
│   ├── devemit/main.go           # Local dev event fixture emitter CLI
│   └── fake-receiver/main.go     # Test webhook receiver with HMAC signature verification
├── internal/
│   ├── config/                   # Strict environment variable parsing & validation
│   ├── logging/                  # Slog JSON logging with PII/secret masking
│   ├── metrics/                  # Prometheus metrics registry (10 swiftdrop_ metrics)
│   ├── health/                   # HTTP probes (/health/live, /health/ready, /metrics)
│   ├── broker/                   # RabbitMQ connection manager, confirms & topology
│   ├── relay/                    # Transactional outbox polling & publisher
│   ├── consumer/                 # Generic queue runner, panic recovery & DLQ routing
│   ├── idempotency/              # Deduplication ledger in PostgreSQL (processed_events)
│   ├── delivery/                 # Delivery repository & exponential backoff calculator
│   ├── notification/             # Real SMTP email dispatch & milestone templates
│   ├── webhook/                  # SSRF-hardened HTTP client, AES-GCM decrypt & HMAC signer
│   └── reconciler/               # Stale audit, expired lease reset, retention & DLQ replay
├── tests/                        # Integration and unit tests with fixtures
├── docker-compose.dev.yml        # Local development infrastructure (Postgres, RabbitMQ, Mailpit)
├── openapi.yaml                  # Probe specification (/health/live, /health/ready, /metrics)
└── .env.example                  # Environment configuration template
```

---

## 5. Environment Variables & Configuration Reference

| Variable | Default Value | Description |
| :--- | :--- | :--- |
| `APP_ENV` | `local` | Application runtime environment (`local`, `production`) |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/swiftdrop_db?sslmode=disable` | Authoritative PostgreSQL connection string |
| `RECONCILER_DATABASE_URL` | `${DATABASE_URL}` | Dedicated connection string for the reconciler |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | RabbitMQ broker AMQP URL |
| `OUTBOX_POLL_INTERVAL_MS`| `500` | Polling frequency for pending outbox events (ms) |
| `OUTBOX_BATCH_SIZE` | `50` | Maximum outbox rows claimed per iteration |
| `OUTBOX_STALE_SECONDS` | `300` | Threshold (5 min) to consider un-published outbox rows stale |
| `OUTBOX_LEASE_SECONDS` | `60` | In-progress lease duration for outbox claims |
| `WORKER_CONCURRENCY` | `10` | Worker prefetch count and execution threads |
| `WEBHOOK_HTTP_TIMEOUT_SECONDS` | `5` | Strict timeout for outbound webhook HTTP calls |
| `WEBHOOK_RETRY_SCHEDULE` | `0s,30s,2m,10m,30m` | Exponential backoff retry progression |
| `WEBHOOK_ALLOW_PRIVATE_HOSTS` | `false` | When true (or in `APP_ENV=local`), permits `http://` localhost targets |
| `WEBHOOK_SECRET_ENCRYPTION_KEY` | *(Required)* | 32-byte AES-256-GCM primary key (Base64-encoded or 64 hex characters) |
| `WEBHOOK_SECRET_ENCRYPTION_KEY_FALLBACKS` | *(Optional)* | Comma-separated `key_version=hex_key` for zero-downtime rotation |
| `DELIVERY_LEASE_SECONDS` | `120` | Worker delivery claim lease duration |
| `SMTP_HOST` | `localhost` | Outgoing SMTP host (e.g., Mailpit in dev) |
| `SMTP_PORT` | `1025` | SMTP port |
| `SMTP_SECURITY` | `none` | Security mode: `starttls`, `tls`, or `none` (`none` local only) |
| `SMTP_USERNAME` | *(Optional)* | SMTP authentication username |
| `SMTP_PASSWORD` | *(Optional)* | SMTP authentication password |
| `SMTP_FROM_EMAIL` | `notifications@swiftdrop.local` | Sender envelope email |
| `SMTP_FROM_NAME` | `SwiftDrop Courier` | Sender display name |
| `PROBE_PORT` | `8082` (Relay), `8083` (Worker) | Health check and metrics port |
| `LOG_LEVEL` | `info` | Structured logging verbosity (`debug`, `info`, `warn`, `error`) |

---

## 6. Webhook Retry Schedule & Outcome Classification

The dispatcher implements an exponential retry schedule: `0s, 30s, 2m, 10m, 30m` (5 total attempts).

| Response Status | Classification | Behavior |
| :--- | :--- | :--- |
| `2xx` (200, 204) | Success | Marked `SUCCESS`, logged, attempt marked completed |
| `410 Gone` | Terminal Client Failure | Marked `FAILED` immediately (no further retries) |
| `4xx` (400, 401, 403, 404, 422) | Terminal Client Failure | Marked `FAILED` immediately (invalid target endpoint) |
| `408 Request Timeout` | Retryable Failure | Scheduled for next retry interval up to attempt 5 |
| `429 Too Many Requests` | Retryable Failure | Scheduled for next retry interval up to attempt 5 |
| `5xx` (500, 502, 503, 504) | Retryable Server Failure | Scheduled for next retry interval up to attempt 5 |
| Connection Timeout / Network Drop | Retryable Failure | Scheduled for next retry interval up to attempt 5 |
| SSRF Violation / Host Blocked | Terminal Security Fault | Marked `FAILED` immediately with class `SSRF_BLOCKED` |
| Attempt 5 Failure | Exhausted | Marked `FAILED` permanently after 5 attempts |

---

## 7. Operational Runbooks

### Runbook 1: Stuck Outbox Recovery
1. Check Prometheus gauge: `swiftdrop_outbox_oldest_pending_age_seconds`.
2. If age exceeds 300s, check Outbox Relay logs:
   ```bash
   kubectl logs -l app=swiftdrop-outbox-relay
   ```
3. Run the audit check:
   ```bash
   reconcile --stale-threshold 5m
   ```
4. If RabbitMQ was unreachable, the relay reconnects automatically. Once broker connectivity is restored, the relay drains pending rows sequentially with publisher confirms.

### Runbook 2: Dead-Letter Queue (DLQ) Replay
1. Inspect dead-lettered poison messages in RabbitMQ Management UI (http://localhost:15672) on `swiftdrop.dlq`.
2. Fix upstream schema validation or network issues.
3. Drain and safely republish messages from the DLQ:
   ```bash
   reconcile dlq-replay --limit 100 --dlq-queue swiftdrop.dlq --target-exchange swiftdrop.events.topic
   ```
4. Messages are republished with header `x-replayed-from: swiftdrop.dlq` and acknowledged only upon confirmed broker receipt.

### Runbook 3: Rotating `WEBHOOK_SECRET_ENCRYPTION_KEY`
Zero-downtime key rotation is supported via versioned keys (Decision D3):
1. Generate a new 32-byte AES key (64 hex characters): `openssl rand -hex 32`.
2. Set `WEBHOOK_SECRET_ENCRYPTION_KEY` to the new key (primary version, e.g. v2).
3. Set `WEBHOOK_SECRET_ENCRYPTION_KEY_FALLBACKS="1=<old_hex_key>"`.
4. Deploy the updated worker. Existing endpoint secrets encrypted with v1 decrypt via fallback, while new endpoints encrypt with v2.

---

## 8. Known Limitations & Domain Boundaries

1. **Merchant Tenant UUID Alignment (Decision D25):** The seeded merchant account in `core-api` has `tenant_id = 'mrc_01HMERCHANTTENANT00001'`. The merchant delivery history endpoints require a valid UUID. Until Member 1 migrates merchant tenant IDs to UUIDs, seeded tokens will receive `403 TENANT_NOT_MERCHANT_UUID`.
2. **Authoritative Table Isolation (Decision D9):** The Messaging service does not own or modify `parcels`, `couriers`, or `parcel_addresses`. Any status inquiries must be made through published event streams or Member 02/03 public APIs.
3. **Relay Strict Singleton:** The Outbox Relay must remain a single replica (`replicas: 1`) to preserve strict aggregate sequence ordering and prevent concurrent lease locks.

---

## 9. Prometheus Metrics Reference
All metrics use the `swiftdrop_` namespace:
- `swiftdrop_outbox_events_polled_total`: Counter of outbox events polled.
- `swiftdrop_outbox_events_published_total`: Counter of events confirmed published.
- `swiftdrop_outbox_events_failed_total`: Counter of outbox publishing failures.
- `swiftdrop_outbox_pending_count`: Gauge of currently pending outbox events.
- `swiftdrop_outbox_oldest_pending_age_seconds`: Gauge of oldest pending event age.
- `swiftdrop_worker_events_consumed_total`: Counter of consumed queue messages.
- `swiftdrop_notifications_sent_total`: Counter of sent email notifications.
- `swiftdrop_notifications_failed_total`: Counter of failed email notifications.
- `swiftdrop_webhooks_dispatched_total`: Counter of attempted webhook dispatches.
- `swiftdrop_dlq_messages_routed_total`: Counter of poison messages rejected to DLQ.

---

## 10. Local Development & Verification
Start local dependencies:
```bash
docker compose -f docker-compose.dev.yml up -d
```

Run unit tests:
```bash
go test -v -count=1 ./...
```
