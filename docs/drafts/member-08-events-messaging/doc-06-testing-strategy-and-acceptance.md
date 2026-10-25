# Document 06 — Testing Strategy & Acceptance Specification

> **Module:** Event-Driven Backend & Messaging (`internal/events`, `services/messaging`)  
> **Owning Member:** Member 08 — Event-Driven Backend & Integrations  
> **Status:** Draft Baseline Specification for Review & Implementation  
> **Target Production Locations:**  
> - `core-api/internal/events/` (In-Monolith Outbox Writer & Envelopes)  
> - `services/messaging/` (Outbox Relay Daemon, Real SMTP Worker, Webhook Dispatcher, Reconciler)  
> **Relevant Contracts:** [event-contracts.md](../../contracts/event-contracts.md), [webhook-security.md](../../contracts/webhook-security.md), [test-strategy.md](../../testing/test-strategy.md)

---

## 1. Testing Philosophy & Verification Pyramid

The Event-Driven Messaging and Integrations subsystem forms the critical asynchronous backbone of SwiftDrop. Because it deals with financial and operational parcel milestones, data loss, duplicate notifications, or SSRF security breaches are completely intolerable.

Testing follows the **SwiftDrop Quality Verification Pyramid**:

```
                       ▲
                      / \
                     /   \
                    / E2E \     Cross-Workflow Resiliency & Crash Tests (10%)
                   /-------\    (Broker restarts, worker crashes, reconciliation)
                  /         \
                 / Integrat. \   Real Component Testing with Testcontainers (30%)
                /-------------\  (PostgreSQL transactions, RabbitMQ confirms, Real SMTP)
               /               \
              /  Contract & Sec \ Webhook HMAC verification & SSRF dialer defense (20%)
             /-------------------\
            /      Unit Tests     \ Pure logic: Envelopes, backoff, templates, dedupe (40%)
           /-----------------------\
```

### Key Testing Principles:
1. **Never Mock the Database for Outbox Atomicity:** The atomic commit of domain entities and outbox rows must be tested against real PostgreSQL instances (via Testcontainers or local Postgres) to verify transaction commit and rollback semantics.
2. **Real Network Verification for SMTP:** Following the core directive, email notification testing validates real SMTP protocol interactions (TLS/STARTTLS handshakes, authentication, and `250 OK` responses) using dedicated SMTP test credentials (e.g. Mailtrap or dedicated test SMTP relay).
3. **Hostile Network Dialing for SSRF:** The outbound HTTP transport must be subjected to real hostile network requests targeting loopback (`127.0.0.1`), metadata IP (`169.254.169.254`), and private RFC 1918 subnets to prove that connection attempts are severed before any TCP socket connects.

---

## 2. Requirement-to-Test Traceability Matrix

Every requirement from the approved SRS, Engineering Standards, and Domain Contracts maps to specific automated tests:

| Requirement ID | Specification | Primary Verification Test | Test Type |
| :--- | :--- | :--- | :--- |
| **FR-EVT-001** | Atomic outbox event insertion in same transaction as business mutation. | `TestOutboxWriter_AtomicRollbackAndCommit` | Integration (Postgres) |
| **FR-EVT-002** | Outbox Relay publishes with persistent delivery mode to RabbitMQ. | `TestOutboxRelay_PublishesPersistentMessage` | Integration (RabbitMQ) |
| **FR-EVT-003** | Status updated to `PUBLISHED` only after RabbitMQ Publisher Confirm; retry or investigation on failure. | `TestOutboxRelay_PublisherConfirms_StatusUpdated` | Integration (AMQP) |
| **FR-EVT-004** | Stable unique event ID, event type, schema version, occurrence timestamp, aggregate reference. | `TestEventEnvelope_FieldValidationAndSerialization` | Unit |
| **FR-EVT-005** | Event Worker consumes status events and creates delivery tasks. | `TestEventWorker_ConsumesAndDispatches` | Integration |
| **FR-EVT-006** | Consumers are idempotent; duplicate messages produce no duplicate side effects. | `TestConsumerIdempotency_DuplicateDeliveryIgnored` | Integration / Unit |
| **FR-EVT-007** | Transient failures retried with bounded backoff and jitter; poison routed to DLQ. | `TestWebhookWorker_RetryBackoffAndDLQExhaustion` | Integration / E2E |
| **FR-EVT-008** | Notification channel: real SMTP integration (with Mailpit test sink). | `TestSMTPNotifier_RealTLSDelivery` | Integration (Real SMTP) |
| **FR-EVT-009** | Operational metrics for outbox age, backlog, consumer failures, DLQ depth. | `TestMetrics_ExposesOperationalCounters` | Unit / HTTP |
| **FR-EVT-010** | At-least-once delivery with idempotent effects (no false exactly-once claim). | `TestPipeline_AtLeastOnceWithIdempotentEffects` | Integration / E2E |
| **FR-REC-001** | Reconciliation identifies outbox records stuck in `PENDING` > 5 minutes. | `TestReconciler_DetectsStaleOutboxEvents` | Integration |
| **FR-REC-002** | Reconciliation reschedules overdue webhook/notification retries safely. | `TestReconciler_ReschedulesOverdueWebhookRetries` | Integration |
| **FR-REC-003** | Reconciliation detects inconsistencies in M8-owned data (outbox, deliveries, leases). Parcel history/assignment checks are pending DEC-04. | `TestReconciler_DetectsDataInconsistencies` | Integration |
| **FR-REC-004** | Reconciliation actions are idempotent, batch-bounded, logged, measurable, auditable. | `TestReconciler_IdempotentBatchBoundedExecution` | Integration / Audit |
| **FR-REC-005** | Reconciliation does not bypass normal outbox publication flow. | `TestReconciler_PreservesOutboxPublicationFlow` | Integration |
| **FR-MER-008** | Merchant webhook registration URL validation and secret management policy. | `TestWebhookConfig_URLValidationAndSecretPolicy` | Unit / Security |
| **FR-MER-009** | Asynchronous worker pipeline delivers configured webhook events after status changes. | `TestWebhookWorker_DeliversSubscribedEvents` | Integration |
| **FR-MER-010** | Webhook requests include event ID, type, schema version, timestamp, parcel ref, HMAC signature. | `TestWebhookSigner_HMACCalculationAndVerification` | Contract / Unit |
| **FR-MER-011** | Records delivery attempts, HTTP outcomes, attempt times, next retry, final state. | `TestWebhookWorker_RecordsDetailedAttemptHistory` | Integration (Postgres) |
| **SEC-010** | SSRF prevention: blocks loopback, private IPs, link-local, cloud metadata, redirects, DNS rebinding. | `TestSafeDialContext_BlocksSSRFAndRedirects` | Security / Unit |
| **SEC-011** | Webhook signature verification and 300-second timestamp/replay protection. | `TestWebhookSigner_AntiReplayTimestampWindow` | Contract / Unit |
| **SEC-015** | Audit logging for administrative actions and state changes; bounded retention. | `TestReconciler_AuditLogAndRetention` | Integration / Audit |
| **NFR-PERF-002** | Status update visible to tracking within 5s under normal load. | `TestTrackingLatency_UnderFiveSeconds` | Performance / Integration |
| **NFR-PERF-003** | Committed outbox event published to RabbitMQ within 5s under normal load. | `TestOutboxRelay_PublishLatencyUnderFiveSeconds` | Performance / Integration |
| **AT-009** | Outbox Relay durability: published only after broker confirmation; recovers after restart. | `TestE2E_OutboxRelayDurabilityAndBrokerRecovery` | E2E |
| **AT-010** | Duplicate RabbitMQ delivery handling produces single side effect. | `TestE2E_DuplicateDeliveryProducesSingleSideEffect` | E2E |
| **AT-012** | Merchant receives signed webhook with retry after transient failure. | `TestE2E_MerchantWebhookDeliveryWithSimulatedRetry` | E2E |
| **AT-016** | Kubernetes readiness/liveness probes and reconciliation CronJob demonstration. | `TestHealthEndpoints_LivenessAndReadiness` | Operational / E2E |

---

## 3. Comprehensive Test-Case Specifications

---

### 3.1 Unit Testing Suite

#### TC-UNIT-01: Event Envelope Serialization & Validation
- **Component:** `core-api/internal/events/envelope.go`
- **Goal:** Verify that domain envelopes serialize to valid JSON with required metadata.
- **Inputs:** `EventEnvelope[ParcelBookedPayload]` with UUIDv7, UTC timestamp, and correlation ID.
- **Assertions:**
  - JSON output contains `event_id`, `event_type`, `aggregate_type`, `aggregate_id`, `version`, `correlation_id`, `payload`.
  - Serializing with empty `event_id` or `event_type` returns a domain validation error.

#### TC-UNIT-02: Exponential Backoff with Jitter Calculation
- **Component:** `services/messaging/internal/webhook/backoff- **Goal:** Verify delay calculations for attempts 1 through 5 (0s, 30s, 2m, 10m, 30m).
- **Assertions:**
  - Attempt 1: Delay == 0s (immediate first dispatch).
  - Attempt 2: $27\text{s} \le \text{Delay} \le 33\text{s}$ (30s $\pm$ 10% jitter).
  - Attempt 3: $108\text{s} \le \text{Delay} \le 132\text{s}$ (120s $\pm$ 10% jitter).
  - Attempt 4: $540\text{s} \le \text{Delay} \le 660\text{s}$ (600s $\pm$ 10% jitter).
  - Attempt 5: $1620\text{s} \le \text{Delay} \le 1980\text{s}$ (1800s $\pm$ 10% jitter).
  - Attempt 6: Returns `ErrMaxRetriesExceeded` (transitions delivery to terminal `FAILED`).

#### TC-UNIT-03: HMAC-SHA256 Webhook Signer & Anti-Replay
- **Component:** `services/messaging/internal/webhook/signer.go`
- **Goal:** Validate signature creation, multi-key rotation, and 5-minute replay window defense.
- **Assertions:**
  - Generates header `X-SwiftDrop-Signature: t={timestamp},v1={hash}`.
  - Verification succeeds with identical secret and body using constant-time comparison.
  - Verification fails if body is altered by even one character.
  - Verification succeeds when signing secret rotation produces multiple `v1=` tokens (`t=...,v1=sig_old,v1=sig_new`) and receiver checks against either key.
  - Verification fails if timestamp is older than 300 seconds ($t < \text{Now} - 300\text{s}$) or in the future ($t > \text{Now} + 300\text{s}$).

#### TC-UNIT-04: Responsive HTML Email Template Rendering
- **Component:** `services/messaging/internal/notifier/templates.go`
- **Goal:** Verify that templates render clean, XSS-sanitized HTML and plaintext.
- **Assertions:**
  - Parcel milestone templates render recipient name, tracking number, and status clearly.
  - HTML tags in parcel notes are automatically escaped by Go `html/template`.
  - Both text and HTML MIME parts are generated with matching delivery milestone metadata.

#### TC-UNIT-05: AES-256-GCM Webhook Secret Decryption with AAD
- **Component:** `services/messaging/internal/webhook/decrypt.go`
- **Goal:** Verify accurate plaintext recovery of merchant secrets encrypted by Member 04, enforcing AAD binding.
- **Assertions:**
  - Standard AES-256-GCM ciphertext (`nonce || ciphertext || tag`) with correct `endpoint_id` as AAD decrypts to expected plaintext secret.
  - Truncated ciphertext (< 28 bytes) returns descriptive error `ErrInvalidCiphertext` ("ciphertext payload is invalid or truncated").
  - Decryption with mismatched AAD (`endpoint_id`) fails authentication (`cipher: message authentication failed`).
  - Ciphertext with modified tag or data bytes returns authentication error.
  - Wrong 32-byte encryption key fails authentication check.

#### TC-UNIT-06: SMTP CRLF Header Injection Defense
- **Component:** `services/messaging/internal/notifier/smtp.go`
- **Goal:** Verify that carriage returns and line feeds in subject, recipient, or sender display names are stripped.
- **Assertions:**
  - Subject containing `Update\r\nBcc: victim@spam.org` is sanitized to `UpdateBcc: victim@spam.org`.
  - Recipient email address containing `victim@spam.org\r\n` fails `mail.ParseAddress()` validation.
  - Non-ASCII characters in email subject are properly encoded via RFC 2047 Q-encoding (`=?utf-8?q?...?=`).

#### TC-UNIT-07: Webhook Response Snippet Truncation & Sanitization
- **Component:** `services/messaging/internal/webhook/dispatcher.go`
- **Goal:** Verify that external merchant responses are capped to 512 bytes with non-UTF-8 and NUL characters stripped before database persistence.
- **Assertions:**
  - A 10 MB error payload returned by a merchant server is truncated to exactly 512 bytes in `webhook_deliveries.response_body_snippet` and `webhook_delivery_attempts.response_body_snippet`.
  - Binary garbage containing NUL (`0x00`) bytes and invalid UTF-8 sequences is sanitized to valid UTF-8 strings before persistence.

#### TC-UNIT-08: Monotonic Aggregate Version Guard
- **Component:** `services/messaging/internal/consumer/order_guard.go`
- **Goal:** Enforce that out-of-order events for the same aggregate are detected and rejected.
- **Assertions:**
  - An event with `aggregate_version = 2` arriving after `aggregate_version = 3` for the same `aggregate_id` is detected as out-of-order, logged with warning, and acknowledged without overwriting newer state.

#### TC-UNIT-09: Missing Email Template Skip-Ack Semantics
- **Component:** `services/messaging/internal/notifier/handler.go`
- **Goal:** Verify that events without customer email templates are skipped cleanly.
- **Assertions:**
  - Receiving `parcel.assigned.v1` or `parcel.picked_up.v1` logs `DEBUG: skipping notification for event_type`, skips SMTP dialogue, and calls AMQP `Ack()`.
  - Zero rows are inserted into `notification_deliveries`.

---

### 3.2 Security & SSRF Defense Testing Suite

#### TC-SEC-01: Prohibited CIDR Blocklist & Multi-IP Resolution Enforcement
- **Component:** `services/messaging/internal/webhook/transport.go` (`SafeDialContext`)
- **Goal:** Verify that any outbound HTTP request to private, loopback, cloud metadata, or reserved IPs is aborted immediately before socket creation.
- **Test Scenarios:**
  ```go
  var blockedTargets = []string{
      "http://127.0.0.1:8080/test",         // IPv4 Loopback
      "http://localhost:5432/test",         // Localhost hostname
      "http://0.0.0.0:8080/test",           // Current network
      "http://169.254.169.254/latest/meta", // Cloud Metadata
      "http://10.0.0.5:8080/internal",      // RFC 1918 Class A
      "http://172.16.1.10:8080/internal",   // RFC 1918 Class B
      "http://192.168.1.1:8080/internal",   // RFC 1918 Class C
      "http://100.64.0.1:8080/cgnat",       // Carrier-Grade NAT (RFC 6598)
      "http://255.255.255.255:8080/test",   // Broadcast
      "http://[::1]:8080/test",             // IPv6 Loopback
      "http://[fe80::1]:8080/test",         // IPv6 Link-Local
      "http://[fc00::1]:8080/test",         // IPv6 Unique Local Address
      "http://[::ffff:127.0.0.1]:8080/t",   // IPv4-mapped IPv6
  }
  ```
- **Assertions:**
  - Every URL in `blockedTargets` returns `ErrBlockedDestinationIP`.
  - Zero network packets leave the machine towards the target IP.
  - Multi-IP Rule: A hostname resolving to `[192.168.1.50, 93.184.216.34]` is rejected immediately because at least one resolved IP is in a blocked CIDR.

#### TC-SEC-02: Open Redirect SSRF Bypass Defense
- **Component:** `services/messaging/internal/webhook/transport.go`
- **Goal:** Verify that when a merchant server returns an HTTP 301, 302, 307, or 308 redirect, the worker does NOT follow it.
- **Setup:** A mock public HTTP test server returns `302 Found` with `Location: http://169.254.169.254/`.
- **Assertions:**
  - The HTTP client's `CheckRedirect` interceptor returns `http.ErrUseLastResponse`.
  - The HTTP 302 response is returned to caller, logged as `SECURITY_REDIRECT_BLOCKED`, and marked as terminal `status = 'FAILED'`.
  - Zero requests are ever issued to `169.254.169.254`.

---

### 3.3 Integration Testing Suite (Real PostgreSQL & RabbitMQ)

#### TC-INT-01: Atomic Transaction Outbox Commit and Rollback
- **Target:** `core-api/internal/events/repository.go`
- **Goal:** Prove that outbox events commit if and only if the domain transaction commits.
- **Scenario A (Rollback):**
  1. Open PostgreSQL transaction `tx`.
  2. Insert mock parcel aggregate.
  3. Call `OutboxWriter.Write(ctx, tx, event)`.
  4. Execute `tx.Rollback()`.
  5. Query `SELECT count(*) FROM outbox_events WHERE id = $1`.
  - **Assertion:** Count == 0. The outbox row vanished with the rollback.
- **Scenario B (Commit):**
  1. Open PostgreSQL transaction `tx`.
  2. Insert mock parcel aggregate.
  3. Call `OutboxWriter.Write(ctx, tx, event)`.
  4. Execute `tx.Commit()`.
  5. Query `SELECT status FROM outbox_events WHERE id = $1`.
  - **Assertion:** Count == 1, `status = 'PENDING'`.

#### TC-INT-02: Outbox Relay Publisher Confirms & NACK Rollback
- **Target:** `services/messaging/internal/outbox/relay.go`
- **Goal:** Verify that the Relay polls pending rows, publishes to RabbitMQ, and updates DB status to `PUBLISHED` individually after broker confirmation.
- **Execution:**
  1. Seed 3 rows in `outbox_events` with `status = 'PENDING'`.
  2. Run Outbox Relay poller with publisher confirms enabled.
  3. Simulate broker NACK on message 2.
- **Assertions:**
  - Messages 1 and 3 arrive on RabbitMQ topic exchange and transition to `status = 'PUBLISHED'`.
  - Message 2 remains `status = 'PENDING'` with incremented `attempt_count = 1`.
  - Zero rows are lost or double-committed.

#### TC-INT-03: Real SMTP Email Delivery Integration Test (Mailpit)
- **Target:** `services/messaging/internal/notifier/smtp.go`
- **Goal:** Verify that the real SMTP mailer connects to a configured real SMTP server, negotiates protocol, and captures the provider message ID.
- **Execution:**
  1. Point `SMTP_HOST=localhost`, `SMTP_PORT=1025`, `SMTP_SECURITY=none` to local Mailpit container.
  2. Invoke `RealSMTPNotifier.SendMilestoneEmail(ctx, "customer@example.com", "Delivery Confirmation", "text", "<html>")`.
- **Assertions:**
  - Real TCP connection established on port 1025.
  - Message transmitted via RFC 5321 dialogue.
  - Mailpit returns `250 OK`.
  - `provider_message_id` is captured and persisted to `notification_deliveries`.
  - `status` is set to `'SUCCESS'`.

#### TC-INT-04: Consumer Idempotency & 10-Duplicate Replay Test (AT-010)
- **Target:** `services/messaging/internal/idempotency/store.go`
- **Goal:** Prove that duplicate RabbitMQ messages for the same event produce zero duplicate emails or webhook calls.
- **Execution:**
  1. Deliver message `0192e4a1-8d2b-7abc-8def-0123456789ab` to `q.notifications.worker`.
  2. Worker processes message, delivers email via Mailpit, records in `processed_events`.
  3. Re-deliver identical message 10 consecutive times to the same queue.
- **Assertions:**
  - Worker detects conflict on `(event_id, consumer_name)` on every redelivery.
  - Worker immediately issues AMQP `Ack`.
  - Total records in `notification_deliveries` remains exactly 1.
  - Exactly 1 email is delivered to Mailpit.

#### TC-INT-05: Concurrent SKIP LOCKED Outbox Polling Test
- **Target:** `services/messaging/internal/outbox/poller.go`
- **Goal:** Prove that multiple concurrent database sessions querying with `FOR UPDATE SKIP LOCKED` claim disjoint sets of pending events without deadlocks or row contention.
- **Execution:**
  1. Seed 100 rows in `outbox_events` (`status = 'PENDING'`).
  2. Spawn 4 concurrent poller goroutines, each requesting `batch_size = 25`.
- **Assertions:**
  - Each poller receives exactly 25 distinct rows.
  - Zero rows are claimed by more than one poller.
  - Total rows claimed across all 4 pollers equals 100.
  - Zero database deadlocks occur.

#### TC-INT-06: Kubernetes Operational Health & Readiness Probes
- **Component:** `services/messaging/internal/health/server.go`
- **Goal:** Validate compliance with Member 09 Agreement A2, K17, and SRS §6.2.
- **Assertions:**
  - `GET /health/live`: Returns `200 OK` `{"status":"UP"}` immediately without querying database.
  - `GET /health/ready`: Returns `200 OK` `{"status":"READY","database":"UP","broker":"UP"}` when PostgreSQL and RabbitMQ are healthy.
  - `GET /health/ready` (Simulated DB Outage): Returns `503 Service Unavailable` with `{"status":"DOWN","reason":"db_unreachable"}`.
  - `GET /health/ready` (Simulated Broker Outage): Returns `503 Service Unavailable` with `{"status":"DOWN","reason":"broker_disconnected"}`.
  - `GET /metrics`: Returns `200 OK` with valid Prometheus text format containing `swiftdrop_outbox_pending_events` and `swiftdrop_outbox_oldest_pending_age_seconds`.

#### TC-INT-07: Sequential Database Migrations Up/Down Test
- **Target:** `000008_create_outbox_events` and `000009_create_worker_dedup_and_webhooks`
- **Goal:** Verify that migrations apply cleanly, roll down cleanly, and re-apply cleanly without orphaned constraints.
- **Execution:**
  1. Execute `migrate up`.
  2. Execute `migrate down 2`.
  3. Execute `migrate up`.
- **Assertions:**
  - All 5 tables (`outbox_events`, `processed_events`, `webhook_deliveries`, `webhook_delivery_attempts`, `notification_deliveries`) are created, dropped, and recreated without error.

#### TC-INT-08: Database Least-Privilege Role Separation Test
- **Target:** PostgreSQL roles `swiftdrop_relay`, `swiftdrop_worker`, `swiftdrop_reconciler`
- **Goal:** Verify database grant isolation.
- **Assertions:**
  - `swiftdrop_relay` connection attempting `SELECT * FROM parcels` returns `permission denied for table parcels`.
  - `swiftdrop_relay` connection attempting `SELECT * FROM webhook_deliveries` returns permission denied.
  - `swiftdrop_worker` connection attempting `UPDATE outbox_events` returns permission denied.

#### TC-INT-09: Bounded Retention Purge Test
- **Target:** `services/messaging/internal/reconcile/retention.go`
- **Goal:** Verify that retention queries delete rows in bounded batches ($\le 500$ rows) without long-held table locks.
- **Execution:**
  1. Seed 1,500 expired rows in `outbox_events` (`status = 'PUBLISHED'`, `created_at = NOW() - 10d`).
  2. Run retention purge iteration.
- **Assertions:**
  - Purge executes in 3 sequential batches of $\le 500$ rows.
  - All 1,500 expired rows are deleted.
  - `swiftdrop_retention_purged_rows_total` increments by 1,500.

---

### 3.4 End-to-End Resilience & Failure Recovery Testing Suite

#### TC-E2E-01: Broker Outage & Relay Self-Healing
- **Scenario:** RabbitMQ container is forcefully stopped (`docker stop rabbitmq`) while Core API continues booking parcels.
- **Execution:**
  1. Core API books 10 parcels. 10 rows enter `outbox_events` (`status = 'PENDING'`).
  2. Outbox Relay detects broken AMQP connection, logs warning, and enters exponential backoff sleep.
  3. Restart RabbitMQ container (`docker start rabbitmq`).
  4. Relay connection manager auto-reconnects.
  5. Relay resumes polling.
- **Assertions:**
  - Zero parcel bookings fail in Core API (Postgres was unaffected).
  - All 10 pending outbox events are published to RabbitMQ once the broker recovers.
  - All 10 rows transition to `status = 'PUBLISHED'`.

#### TC-E2E-02: Webhook Endpoint 500 Outage & DB-Driven Retry
- **Scenario:** A merchant server continuously returns `500 Internal Server Error`.
- **Execution:**
  1. Trigger webhook dispatch for the merchant endpoint.
  2. Worker executes attempts 1 through 5 according to the retry schedule (0s, 30s, 2m, 10m, 30m).
  3. Attempt 5 fails.
- **Assertions:**
  - Exactly 5 attempt rows are appended to `webhook_delivery_attempts`.
  - After attempt 5, `webhook_deliveries.status` transitions to `'FAILED'`.
  - The AMQP message was acknowledged upon initial scheduling; RabbitMQ queues remain at depth 0 (zero CPU-pinning NACK loops).

#### TC-E2E-03: Reconciliation CronJob Execution
- **Scenario:** An outbox row has been in `PENDING` for 10 minutes due to an unhandled publisher confirmation edge case.
- **Execution:**
  1. Seed row with `status = 'PENDING'`, `attempt_count = 5`, `created_at = NOW() - 10m`.
  2. Seed row in `webhook_deliveries` with `status = 'IN_PROGRESS'`, `lease_expires_at = NOW() - 2m`.
  3. Execute `services/messaging/cmd/reconcile`.
- **Assertions:**
  - Reconciler logs detection of stale outbox event and alerts.
  - Reconciler resets `next_attempt_at = NOW()`.
  - Reconciler resets expired delivery lease to `status = 'PENDING'`, `lease_expires_at = NULL`.
  - On the next tick, Outbox Relay successfully publishes the row.

#### TC-E2E-04: Poison AMQP Pill Message & DLQ Dead-Lettering
- **Scenario:** An unparseable binary message or schema-violating payload is published to `swiftdrop.events.topic`.
- **Execution:**
  1. Publish malformed JSON bytes to `swiftdrop.events.topic` with routing key `parcel.booked.v1`.
  2. Event Worker receives message, fails JSON unmarshaling.
- **Assertions:**
  - Worker logs error with correlation ID.
  - Worker rejects message via `Nack(requeue=false)`.
  - RabbitMQ routes message via `swiftdrop.dlq.exchange` to `swiftdrop.dlq`.
  - Message in `swiftdrop.dlq` retains original byte payload and `x-death` headers.

#### TC-E2E-05: Relay Crash Mid-Batch & Worker Deduplication
- **Scenario:** The Outbox Relay process is killed immediately after publishing AMQP messages but before updating PostgreSQL.
- **Execution:**
  1. Relay publishes batch of 5 events to RabbitMQ.
  2. Kill Relay process (`kill -9`).
  3. Workers consume the 5 events, process side effects, and write `processed_events`.
  4. Restart Outbox Relay. Relay re-reads rows from PostgreSQL (still `PENDING`) and re-publishes them.
- **Assertions:**
  - Workers detect duplicate `event_id` in `processed_events`.
  - Workers acknowledge duplicate AMQP messages without re-sending emails or re-calling webhooks.
  - Exactly 5 deliveries occur.

#### TC-E2E-06: Graceful Shutdown Lifecycle
- **Scenario:** Kubernetes issues `SIGTERM` to worker pod while an email delivery is in-flight.
- **Execution:**
  1. Worker begins 3-second SMTP STARTTLS delivery.
  2. Send `SIGTERM` to worker process.
- **Assertions:**
  - Worker immediately switches `/health/ready` to `503 Service Unavailable`.
  - In-flight SMTP email completes and is recorded in `notification_deliveries`.
  - AMQP channel is closed cleanly.
  - Worker process exits with code 0 within grace period (30s).

---

## 4. Test Environment & Mocking Strategy

To maintain test reliability while eliminating fragile external dependencies during local developer iteration:

| Component | Local Unit Tests | Integration Tests | CI / PR Pipeline |
| :--- | :--- | :--- | :--- |
| **PostgreSQL 16+** | In-memory SQLite / sqlmock (prohibited for tx tests) | Real local Docker PostgreSQL or Testcontainers | Ephemeral Docker PostgreSQL container |
| **RabbitMQ 3.12+** | Fake AMQP Channel in-memory | Real local Docker RabbitMQ | Ephemeral Docker RabbitMQ container |
| **Real SMTP Server** | In-memory loopback listener (`net.Listen("tcp", "127.0.0.1:0")`) | Real Mailpit container (`localhost:1025`, `SMTP_SECURITY=none`) | Real Mailpit container in CI pipeline |
| **Merchant HTTP Webhook** | In-process `httptest.Server` | In-process `httptest.Server` with TLS | `httptest.Server` simulating 200, 500, timeouts, and SSRF redirects |

---

## 5. Coverage Expectations for Critical Logic

Automated quality gates configured in CI (`.github/workflows/ci.yml` and `scripts/verify.sh`) enforce minimum test coverage thresholds:

| Module / Package | Critical Invariants Tested | Minimum Statement Coverage |
| :--- | :--- | :--- |
| `core-api/internal/events/` | OutboxWriter transaction atomicity, JSON serialization | **90%** |
| `services/messaging/internal/webhook/` | SSRF `SafeDialContext`, CIDR blocklist, HMAC signer, Backoff | **95%** (100% on SSRF filter) |
| `services/messaging/internal/idempotency/` | Primary key conflict handling, duplicate suppression | **95%** |
| `services/messaging/internal/notifier/` | Real SMTP protocol dialogue, TLS negotiation, 4xx/5xx parsing | **85%** |
| `services/messaging/internal/outbox/` | `SKIP LOCKED` query execution, publisher confirms handling | **90%** |
| `services/messaging/internal/reconcile/`| Stale row discovery, safe retry re-scheduling | **85%** |

---

## 6. How Tests Are Run Locally and in CI

### 6.1 Local Test Commands
Developers run tests using native Go toolchain commands:

```bash
# 1. Run all unit tests with data race detector
go test -v -race ./core-api/internal/events/... ./services/messaging/...

# 2. Run SSRF security unit tests specifically
go test -v -run TestSafeDialContext ./services/messaging/internal/webhook/...

# 3. Run integration tests (Requires Docker Postgres & RabbitMQ running)
docker compose up -d postgres rabbitmq
go test -v -race -tags=integration ./services/messaging/tests/integration/...

# 4. Check code coverage
go test -coverprofile=coverage.out ./services/messaging/...
go tool cover -func=coverage.out
```

### 6.2 CI Pipeline Integration (`scripts/verify.sh`)
As defined in [scripts/verify.sh](../../../scripts/verify.sh), every Pull Request must pass:
1. Secret scanning check (no committed SMTP passwords or private keys).
2. Go code formatting verification (`gofmt`).
3. Static analysis and architecture linter (`golangci-lint run ./...` with `depguard`).
4. Unit and integration test suites with `-race` detection enabled.

---

## 7. Acceptance Criteria for Task Completion

Before Member 08 can mark their workload complete, the following verified proof is mandatory:

1. **Transaction Atomicity Proof:** Integration test logs demonstrating that a rolled-back business transaction in `core-api` leaves zero records in `outbox_events`.
2. **Publisher Confirms Proof:** Test output proving an outbox row remains `PENDING` if RabbitMQ is paused, and transitions to `PUBLISHED` only after RabbitMQ acknowledges.
3. **Real SMTP Transmission Proof:** Captured SMTP log showing successful STARTTLS handshake, authentication, and `250 OK` receipt with captured message ID in `notification_deliveries`.
4. **Idempotency Proof (AT-010):** Automated test logs demonstrating 10 duplicate deliveries of the same RabbitMQ message generating exactly 1 SMTP email and 1 webhook push.
5. **SSRF Rejection Proof (SEC-010):** Test logs showing instant rejection of `127.0.0.1`, `169.254.169.254`, and `10.0.0.1` without establishing TCP connections.
6. **Reconciliation Proof:** Execution output of `cmd/reconcile` detecting and safely un-stalling simulated stale outbox records.

---

## 8. Summary Checklist of Document 06 Deliverables

- [x] Verification pyramid outlining Unit, Security, Integration, and E2E test proportions.
- [x] Traceability matrix mapping requirements (FR-EVT, FR-REC, SEC, AT) to concrete test cases.
- [x] Detailed test case specifications covering happy paths, rollbacks, and edge cases.
- [x] Specific security test cases validating SSRF blocklist enforcement and open redirect defenses.
- [x] Real SMTP integration test specification detailing TLS handshake and 250 OK verification.
- [x] Resilience scenarios covering broker crashes, merchant 500 outages, and DLQ routing.
- [x] Test environment setup, Testcontainers strategy, and mocking boundaries.
- [x] Minimum coverage target matrix enforcing $\ge 90\%$ on critical components.
- [x] CLI commands and CI verification integration via `scripts/verify.sh`.
- [x] Concrete acceptance criteria and verification evidence checklist.
