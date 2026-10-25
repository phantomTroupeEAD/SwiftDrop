# Document 06 — Testing Strategy & Acceptance Specification

> **Module:** Event-Driven Backend & Messaging (`internal/events`, `services/messaging`)  
> **Owning Member:** Member 08 — Event-Driven Backend & Integrations  
> **Status:** Draft Baseline Specification for Review & Implementation  
> **Target Production Locations:**  
> - `core-api/internal/events/` (In-Monolith Outbox Writer & Envelopes)  
> - `services/messaging/` (Outbox Relay Daemon, Real SMTP Worker, Webhook Dispatcher, Reconciler)  
> **Relevant Contracts:** [event-contracts.md](file:///c:/Users/kesha/OneDrive/Documents/GitHub/SwiftDrop/docs/contracts/event-contracts.md), [webhook-security.md](file:///c:/Users/kesha/OneDrive/Documents/GitHub/SwiftDrop/docs/contracts/webhook-security.md), [test-strategy.md](file:///c:/Users/kesha/OneDrive/Documents/GitHub/SwiftDrop/docs/testing/test-strategy.md)

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
| **FR-EVT-003** | Status updated to `PUBLISHED` only after RabbitMQ Publisher Confirm. | `TestOutboxRelay_PublisherConfirms_StatusUpdated` | Integration (AMQP) |
| **FR-EVT-004** | Unique event ID, aggregate ID, ISO-8601 UTC timestamp, correlation ID. | `TestEventEnvelope_FieldValidationAndSerialization` | Unit |
| **FR-EVT-005** | Event Worker creates notification and webhook delivery records. | `TestEventWorker_ConsumesAndDispatches` | Integration |
| **FR-EVT-006** | Consumers are idempotent; duplicate messages produce no duplicate side effects. | `TestConsumerIdempotency_DuplicateDeliveryIgnored` | Integration / Unit |
| **FR-EVT-007** | Bounded exponential backoff with jitter; routes to DLQ after 5 attempts. | `TestWebhookWorker_RetryBackoffAndDLQExhaustion` | Integration / E2E |
| **FR-EVT-008** | Real SMTP email notification delivery with TLS/STARTTLS. | `TestSMTPNotifier_RealTLSDelivery` | Integration (Real SMTP) |
| **FR-EVT-009** | Operational metrics for outbox age, backlog, DLQ depth, and failures. | `TestMetrics_ExposesOperationalCounters` | Unit / HTTP |
| **FR-REC-001** | Reconciliation identifies outbox records stuck in `PENDING` > 5 minutes. | `TestReconciler_DetectsStaleOutboxEvents` | Integration |
| **FR-REC-002** | Reconciliation reschedules overdue webhook retries without bypassing relay. | `TestReconciler_ReschedulesOverdueWebhookRetries` | Integration |
| **SEC-010** | SSRF prevention: blocks loopback, private IPs, cloud metadata, redirects. | `TestSafeDialContext_BlocksSSRFAndRedirects` | Security / Unit |
| **SEC-011** | HMAC-SHA256 signature verification with 300-second anti-replay window. | `TestWebhookSigner_HMACCalculationAndVerification` | Contract / Unit |
| **AT-010** | Assignment Acceptance Test 010: Duplicate RabbitMQ delivery handling. | `TestE2E_DuplicateDeliveryProducesSingleSideEffect` | E2E |
| **AT-012** | Assignment Acceptance Test 012: Merchant receives signed webhook with retry. | `TestE2E_MerchantWebhookDeliveryWithSimulatedRetry` | E2E |

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
- **Component:** `services/messaging/internal/webhook/backoff.go`
- **Goal:** Verify delay calculations for attempts 1 through 5.
- **Assertions:**
  - Attempt 1: Delay == 0s.
  - Attempt 2: $25\text{s} \le \text{Delay} \le 35\text{s}$ (30s $\pm$ 5s).
  - Attempt 3: $105\text{s} \le \text{Delay} \le 135\text{s}$ (120s $\pm$ 15s).
  - Attempt 4: $540\text{s} \le \text{Delay} \le 660\text{s}$ (600s $\pm$ 60s).
  - Attempt 5: $1620\text{s} \le \text{Delay} \le 1980\text{s}$ (1800s $\pm$ 180s).
  - Attempt 6: Returns `ErrMaxRetriesExceeded`.

#### TC-UNIT-03: HMAC-SHA256 Webhook Signer & Anti-Replay
- **Component:** `services/messaging/internal/webhook/signer.go`
- **Goal:** Validate signature creation and 5-minute replay window defense.
- **Assertions:**
  - Generates header `X-SwiftDrop-Signature: t={timestamp},v1={hash}`.
  - Verification succeeds with identical secret and body.
  - Verification fails if body is altered by even one character.
  - Verification fails if timestamp is older than 300 seconds ($t < \text{Now} - 300\text{s}$) or in the future ($t > \text{Now} + 300\text{s}$).

#### TC-UNIT-04: Responsive HTML Email Template Rendering
- **Component:** `services/messaging/internal/notifier/templates.go`
- **Goal:** Verify that templates render clean, XSS-sanitized HTML and plaintext.
- **Assertions:**
  - Parcel milestone templates render recipient name, tracking number, and status clearly.
  - HTML tags in parcel notes are automatically escaped by Go `html/template`.

---

### 3.2 Security & SSRF Defense Testing Suite

#### TC-SEC-01: Prohibited CIDR Blocklist Enforcement
- **Component:** `services/messaging/internal/webhook/client.go` (`SafeDialContext`)
- **Goal:** Verify that any outbound HTTP request to private or cloud metadata IPs is aborted immediately before socket creation.
- **Test Scenarios:**
  ```go
  var blockedTargets = []string{
      "http://127.0.0.1:8080/test",         // IPv4 Loopback
      "http://localhost:5432/test",         // Localhost hostname
      "http://169.254.169.254/latest/meta", // AWS/GCP Metadata
      "http://10.0.0.5:8080/internal",      // RFC 1918 Private Class A
      "http://172.16.1.10:8080/internal",   // RFC 1918 Private Class B (Docker Bridge)
      "http://192.168.1.1:8080/internal",   // RFC 1918 Private Class C (LAN)
      "http://[::1]:8080/test",             // IPv6 Loopback
      "http://[fe80::1]:8080/test",         // IPv6 Link-Local
  }
  ```
- **Assertions:**
  - Every URL in `blockedTargets` returns `ErrBlockedDestinationIP` or `SSRF violation`.
  - Zero network packets leave the machine towards the target IP.

#### TC-SEC-02: Open Redirect SSRF Bypass Defense
- **Component:** `services/messaging/internal/webhook/client.go`
- **Goal:** Verify that when a merchant server returns an HTTP 301/302 redirect pointing to an internal IP, the worker does NOT follow it.
- **Setup:** A mock public HTTP test server returns `302 Found` with `Location: http://169.254.169.254/`.
- **Assertions:**
  - The HTTP client's `CheckRedirect` interceptor immediately halts execution.
  - The delivery is logged as `FAILED` (HTTP 302 not followed).
  - No request is ever issued to `169.254.169.254`.

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

#### TC-INT-02: Outbox Relay Publisher Confirms & State Transition
- **Target:** `services/messaging/internal/outbox/relay.go`
- **Goal:** Verify that the Relay polls pending rows, publishes to RabbitMQ, and updates DB status to `PUBLISHED` only after broker confirmation.
- **Execution:**
  1. Seed 3 rows in `outbox_events` with `status = 'PENDING'`.
  2. Run one iteration of Outbox Relay poller.
  3. Inspect RabbitMQ queue `q.notifications.worker`.
  4. Query `outbox_events`.
- **Assertions:**
  - All 3 messages arrive on RabbitMQ topic exchange.
  - `status` in `outbox_events` becomes `'PUBLISHED'`.
  - `published_at` is NOT NULL and within 2 seconds of current time.

#### TC-INT-03: Real SMTP Email Delivery Integration Test
- **Target:** `services/messaging/internal/notifier/sender.go`
- **Goal:** Verify that the real SMTP mailer connects to a configured real SMTP server, negotiates TLS/STARTTLS, authenticates, and captures the provider message ID.
- **Execution:**
  1. Configure `SMTP_HOST`, `SMTP_PORT=587`, `SMTP_USERNAME`, `SMTP_PASSWORD` pointing to Mailtrap or dedicated test SMTP relay.
  2. Invoke `SMTPSender.Send(ctx, EmailMessage{ To: "test@swiftdrop.io", Subject: "POD Confirmation", ... })`.
- **Assertions:**
  - Real TCP connection established on port 587.
  - STARTTLS handshake succeeds.
  - Provider returns `250 OK`.
  - `provider_message_id` is captured and persisted to `notification_deliveries`.
  - `status` is set to `'SENT'`.

#### TC-INT-04: Consumer Idempotency & Deduplication
- **Target:** `services/messaging/internal/idempotency/store.go`
- **Goal:** Prove that duplicate RabbitMQ messages for the same event produce zero duplicate emails or webhook calls.
- **Execution:**
  1. Deliver message `evt_01HXYZ` to `q.notifications.worker`.
  2. Worker processes message, sends real SMTP email, records in `processed_events`.
  3. Re-deliver identical message `evt_01HXYZ` to the same queue.
  4. Worker receives duplicate message.
- **Assertions:**
  - Worker detects conflict on `(event_id, consumer_name)`.
  - Worker immediately issues AMQP `Ack`.
  - Total records in `notification_deliveries` remains exactly 1.
  - Zero duplicate emails are transmitted.

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

#### TC-E2E-02: Webhook Endpoint 500 Outage & DLQ Routing
- **Scenario:** A merchant server continuously returns `500 Internal Server Error`.
- **Execution:**
  1. Trigger webhook dispatch for the merchant.
  2. Worker executes attempts 1 through 5 according to the backoff schedule.
  3. Attempt 5 fails.
- **Assertions:**
  - 5 delivery attempts are recorded in `webhook_deliveries`.
  - After attempt 5, the message is routed to `swiftdrop.dlq`.
  - Message in `swiftdrop.dlq` retains original payload, failure headers, and correlation ID.
  - Merchant endpoint is flagged as `DEGRADED`.

#### TC-E2E-03: Reconciliation CronJob Execution
- **Scenario:** An outbox row has been in `PENDING` for 10 minutes due to a temporary network drop during publisher confirmation.
- **Execution:**
  1. Seed row with `status = 'PENDING'`, `attempt_count = 5`, `created_at = NOW() - 10m`.
  2. Execute `services/messaging/cmd/reconcile`.
- **Assertions:**
  - Reconciler logs detection of stale outbox event.
  - Reconciler resets `attempt_count = 0` and `last_error = NULL`.
  - On the next tick, Outbox Relay successfully publishes the row.

---

## 4. Test Environment & Mocking Strategy

To maintain test reliability while eliminating fragile external dependencies during local developer iteration:

| Component | Local Unit Tests | Integration Tests | CI / PR Pipeline |
| :--- | :--- | :--- | :--- |
| **PostgreSQL 16+** | In-memory SQLite / sqlmock (prohibited for tx tests) | Real local Docker PostgreSQL or Testcontainers | Ephemeral Docker PostgreSQL container |
| **RabbitMQ 3.12+** | Fake AMQP Channel in-memory | Real local Docker RabbitMQ | Ephemeral Docker RabbitMQ container |
| **Real SMTP Server** | In-memory loopback listener (`net.Listen("tcp", "127.0.0.1:0")`) | Real Mailtrap SMTP or dedicated test SMTP relay | Real Mailtrap SMTP credentials in GitHub Secrets |
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
As defined in [scripts/verify.sh](file:///c:/Users/kesha/OneDrive/Documents/GitHub/SwiftDrop/scripts/verify.sh), every Pull Request must pass:
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
