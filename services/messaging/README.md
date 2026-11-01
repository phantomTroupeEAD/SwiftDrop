# Messaging & Integration Service (`services/messaging`)

> **Service:** Outbox Relay Daemon, RabbitMQ Message Bus, Notification Worker, Webhook Dispatcher & Reconciler  
> **Primary Owner:** Member 8 — Event-Driven Backend & Integrations  
> **Deployment Architecture:** Dedicated Autonomous Background Daemons (Relay + Worker + Reconciler)  
> **Container Image:** `ghcr.io/phantomtroupeead/swiftdrop/messaging:<git-sha>` (`USER 10001:10001`)

---

## 1. Service Overview & Responsibilities
The `messaging` service decouples asynchronous side effects from synchronous Core API request lifecycles. It polls committed records from the PostgreSQL `outbox_events` table, publishes them to RabbitMQ with publisher confirms, and processes asynchronous delivery to customer notification providers and external merchant webhook endpoints.

### Dedicated Binaries:
1. **Outbox Relay Daemon (`cmd/relay`):**
   - High-throughput background poller scanning `outbox_events` using `SELECT ... FOR UPDATE SKIP LOCKED` (batch size 50).
   - Publishes persistent AMQP messages (`DeliveryMode = 2`) to topic exchange `swiftdrop.events.topic`.
   - Enforces **RabbitMQ Publisher Confirms**; updates `published_at` only upon broker ACK.
   - Deployed strictly as a singleton (`replicas: 1`).
2. **Event Worker Service (`cmd/worker`):**
   - Multi-threaded consumer subscribing to `q.notifications.worker` and `q.webhooks.dispatcher`.
   - Consumer idempotency ledger via `processed_events` table.
   - **Direct Real SMTP Notifier:** Delivers milestone emails over TLS/STARTTLS (`net/smtp` + `crypto/tls`); zero mock sinks.
   - **SSRF-Hardened Webhook Dispatcher:** Custom `SafeDialContext` with single DNS lookup and private CIDR blocklist. Signs payloads with HMAC-SHA256 (`X-SwiftDrop-Signature`) and anti-replay timestamps (`X-SwiftDrop-Timestamp`).
   - Exponential backoff with jitter (max 5 retries) before routing to `swiftdrop.dlq`.
3. **Reconciliation Engine (`cmd/reconcile`):**
   - Scheduled Kubernetes CronJob auditing stuck outbox rows (`created_at < NOW() - 5m`) and overdue webhook retries.

---

## 2. Invariants & Guarantees
1. **At-Least-Once Delivery & Idempotency:** Consumers MUST maintain idempotency tables (`processed_events`). Duplicate message deliveries are acknowledged and safely ignored.
2. **Publisher Confirms:** Messages in `outbox_events` are marked `PUBLISHED` only after explicit confirmation from the RabbitMQ cluster.
3. **SSRF Guard:** Webhook worker MUST resolve destination DNS and reject loopback (`127.0.0.0/8`), link-local (`169.254.0.0/16`), and private IP ranges before sending outbound HTTP requests. Redirects are strictly prohibited.
4. **No Database State Bypass:** Relay and workers must never alter parcel state (`parcels` table); all updates reflect authentic PostgreSQL events.
5. **Decrypted Webhook Signing:** The Webhook Worker reads `merchant_webhook_targets_view` and decrypts `secret_ciphertext` using AES-256-GCM before computing HMAC-SHA256 signatures.

---

## 3. Service Ports & Health Probes
- **Outbox Relay Daemon:**
  - Health & Metrics Port: `:8082` (internal only, not routed via gateway)
  - Liveness: `GET /health/live`
  - Readiness: `GET /health/ready` (checks PostgreSQL pool & RabbitMQ connection)
  - Metrics: `GET /metrics` (`swiftdrop_outbox_pending_events`)
- **Event Worker Service:**
  - Health & Metrics Port: `:8083` (internal only, not routed via gateway)
  - Liveness: `GET /health/live`
  - Readiness: `GET /health/ready` (checks PostgreSQL pool & RabbitMQ consumer channels)
  - Metrics: `GET /metrics`

---

## 4. Database Ownership & Migrations
- `processed_events` (Migration `000009_create_worker_dedup_and_webhooks.up.sql`): `consumer_name VARCHAR(64)`, `event_id UUID`, `processed_at TIMESTAMPTZ`. `PRIMARY KEY (consumer_name, event_id)`.
- `webhook_deliveries` (Migration `000009_create_worker_dedup_and_webhooks.up.sql`): `id UUID PRIMARY KEY`, `endpoint_id UUID REFERENCES webhook_endpoints(id)`, `event_id UUID`, `status VARCHAR(32)`, `attempt_count INTEGER`, `response_status_code INTEGER`, `delivered_at TIMESTAMPTZ`.
- `notification_deliveries` (Migration `000009_create_worker_dedup_and_webhooks.up.sql`): `id UUID PRIMARY KEY`, `event_id UUID`, `recipient VARCHAR(255)`, `channel VARCHAR(32)`, `status VARCHAR(32)`, `provider_message_id VARCHAR(128)`, `sent_at TIMESTAMPTZ`.

---

## 5. Changelog & Implementation Status
| Date | Change Summary | Author / Agent |
| :--- | :--- | :--- |
| 2026-10-21 | Initial service architecture baseline created | Architecture Setup |
| 2026-10-25 | Realigned to v2.0.0: AES-256-GCM webhook secret decryption, SSRF SafeDial transport, direct real SMTP mailer, and unified multi-binary container | Post-Submission Alignment |
