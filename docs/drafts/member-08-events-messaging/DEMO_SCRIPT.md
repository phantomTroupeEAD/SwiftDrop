# SwiftDrop Member 08 — Live Demonstration Walkthrough Script

> **Audience:** Evaluators, Course Lecturers, and Development Team  
> **Domain:** Member 08 — Event-Driven Backend & Messaging Subsystem  
> **Duration:** 5–7 minutes

---

## 1. Prerequisites & Environment Setup

Open a terminal at the repository root:

```bash
# 1. Start local PostgreSQL, RabbitMQ, and Mailpit
docker compose -f services/messaging/docker-compose.dev.yml up -d

# 2. Verify containers are healthy
docker compose -f services/messaging/docker-compose.dev.yml ps
```

| Service | Port | Dashboard / UI URL |
| :--- | :--- | :--- |
| **PostgreSQL 16** | `5432` | `swiftdrop_db` |
| **RabbitMQ 3.12** | `5672` (AMQP), `15672` (Web UI) | http://localhost:15672 (guest/guest) |
| **Mailpit** | `1025` (SMTP), `8025` (Web UI) | http://localhost:8025 |

---

## 2. Step 1: Run Migrations & Seed Data

Apply canonical schema migrations including outbox and worker tables:

```bash
cd core-api
go run ./cmd/migrate
```

Notice:
- `20261101000050_create_outbox_events` creates `outbox_events` with partial indexes.
- `20261101000500_create_worker_dedup_and_webhooks` creates `processed_events`, `notification_deliveries`, `webhook_deliveries`, and `webhook_delivery_attempts`.

---

## 3. Step 2: Start Background Messaging Daemons

In separate terminal tabs:

### Terminal A: Start Outbox Relay
```bash
cd services/messaging
go run ./cmd/relay
```
*Expected log:* `outbox poller started batch_size=50 poll_interval=500ms lease=60s`.

### Terminal B: Start Event Worker
```bash
cd services/messaging
go run ./cmd/worker
```
*Expected log:* `declared topology exchange=swiftdrop.events.topic dlx=swiftdrop.dlq.exchange dlq=swiftdrop.dlq`, followed by consumer startups for `q.notifications.worker` and `q.webhooks.dispatcher`.

### Terminal C: Start Core API
```bash
cd core-api
go run ./cmd/api
```
*Expected log:* Server listening on http://localhost:8080.

---

## 4. Step 3: Emit Real Domain Events via `devemit` CLI

Emit a batch of canonical `parcel.booked.v1` events directly into `outbox_events`:

```bash
cd services/messaging
go run ./cmd/devemit --type parcel.booked.v1 --count 3 --email "student@university.edu"
```

### Observe Real-Time Event Pipeline:
1. **Outbox Relay (Terminal A):** Immediately claims the 3 events with `FOR UPDATE SKIP LOCKED` and publishes them with publisher confirms to `swiftdrop.events.topic`.
2. **RabbitMQ Management UI:** Browse http://localhost:15672/#/queues to observe message ingress into `q.notifications.worker`.
3. **Event Worker (Terminal B):** Consumes messages, marks `processed_events` for deduplication, and schedules transactional email notifications.
4. **Mailpit Web UI:** Open http://localhost:8025 to view the rendered HTML milestone emails for `SD-BOOKED-0001`!

---

## 5. Step 4: Webhook Dispatch & HMAC Signature Verification

Start the included fake merchant webhook receiver to demonstrate signed delivery:

```bash
cd services/messaging
go run ./cmd/fake-receiver --port 9090
```

1. The fake receiver listens at `http://localhost:9090/webhook`.
2. As webhook deliveries occur, observe:
   - Header `X-SwiftDrop-Signature: t=<unix>,v1=<hex>`.
   - Header `X-SwiftDrop-Timestamp: <unix>`.
   - Automatic HMAC-SHA256 signature verification passing.

---

## 6. Step 5: Fault Injection, Retries, and DLQ Quarantine

Test fault tolerance and poison message routing:

### Trigger HTTP 500 Failures on the Webhook Receiver:
```bash
# fake-receiver supports fault injection via ?status=<code query parameter in the webhook URL
# (e.g. http://localhost:9090/webhook?status=500).
go run ./cmd/fake-receiver --port 9090
```
- Observe the Event Worker backing off exponentially (`0s, 30s, 2m, 10m, 30m`).
- Inspect attempt audit log in PostgreSQL: `SELECT response_status_code, error_class, error_message FROM webhook_delivery_attempts ORDER BY created_at DESC LIMIT 5;`.

### Simulate Poison Message DLQ Quarantine:
Publish a malformed, invalid JSON payload directly to the queue:
- Worker recovers safely without panicking.
- Poison message is rejected with `Nack(requeue=false)` directly into `swiftdrop.dlq`.

---

## 7. Step 6: Reconciliation, Recovery, and DLQ Replay

Run the reconciliation CLI (`cmd/reconcile`):

### 1. Audit Stale Outbox & Recover Expired Leases:
```bash
cd services/messaging
go run ./cmd/reconcile
```
*Output:* One JSON log line detailing audited outbox rows, reset worker leases, and consistency health.

### 2. Execute Data Retention Pruning:
```bash
go run ./cmd/reconcile retention --batch-limit 500
```
*Output:* Bounded batch deletion of published outbox events (>7d) and terminal delivery rows (>30d).

### 3. Replay Quarantined Messages from DLQ:
```bash
go run ./cmd/reconcile dlq-replay --limit 10
```
*Output:* Safely consumes poisoned/failed messages from `swiftdrop.dlq`, attaches header `x-replayed-from: swiftdrop.dlq`, and republishes to `swiftdrop.events.topic` upon confirmed publisher receipt.

---

## 8. Step 7: Swagger UI & Living Documentation

Open your browser and navigate to:
- **Interactive Swagger UI:** http://localhost:8080/swagger/events/
- **Raw OpenAPI Specification:** http://localhost:8080/swagger/events/spec.json
- **Swagger Documentation Hub:** http://localhost:8080/swagger/

Demonstrate:
1. `GET /api/v1/admin/events/outbox`: Keyset-paginated outbox events.
2. `GET /api/v1/admin/events/outbox/stats`: Aggregate status counts and oldest pending age.
3. `POST /api/v1/admin/events/outbox/{id}/retry`: Failed event retry re-queueing.
4. `GET /api/v1/merchant/webhook-deliveries`: Tenant-isolated merchant delivery history.

