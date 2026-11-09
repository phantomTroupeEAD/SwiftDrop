# SwiftDrop Messaging Test Suite (`services/messaging/tests`)

> **Owner:** Member 08 — Event-Driven Backend & Integrations  
> **Status:** Verification Complete

---

## 1. Running the Test Suites

### Fast Unit Tests (In-Memory / Isolated)
Unit tests do not require PostgreSQL, RabbitMQ, or external networks. All database transactions are tested using lightweight in-memory SQL drivers, and network requests are verified using local loopback servers.
```bash
cd services/messaging
go test -v -count=1 ./...
```

### End-to-End Integration Tests (`INTEGRATION=1`)
End-to-end integration tests execute full transactional lifecycles across PostgreSQL and RabbitMQ.
1. Start local dependencies:
   ```bash
   docker compose -f docker-compose.dev.yml up -d
   ```
2. Run with `INTEGRATION=1`:
   ```bash
   INTEGRATION=1 go test -v -count=1 ./tests/...
   ```

---

## 2. Requirements Verification Matrix

| Requirement ID | Specification Description | Implementing Package | Test Location | Test Result |
| :--- | :--- | :--- | :--- | :--- |
| **FR-EVT-001** | Transactional Outbox Pattern & Atomic Commit | `core-api/internal/events` | `core-api/internal/events/repository_test.go` | PASS |
| **FR-EVT-002** | RFC 9562 UUIDv7 Envelopes & Canonical Schema | `core-api/internal/events` | `core-api/internal/events/envelope_test.go` | PASS |
| **FR-EVT-003** | Outbox Relay Singleton & Guaranteed At-Least-Once Delivery | `services/messaging/internal/relay` | `internal/relay/relay_test.go` | PASS |
| **FR-EVT-004** | RabbitMQ Topic Exchange & Topology Declaration | `services/messaging/internal/broker` | `internal/broker/broker_test.go` | PASS |
| **FR-EVT-005** | Idempotent Consumer & Deduplication Ledger | `services/messaging/internal/consumer` | `internal/consumer/consumer_test.go` | PASS |
| **FR-EVT-006** | Transactional SMTP Milestone Notifications & Template Engine | `services/messaging/internal/notification` | `internal/notification/notification_test.go` | PASS |
| **FR-EVT-007** | SSRF-Hardened Webhook Dispatcher & Private Host Blocking | `services/messaging/internal/webhook` | `internal/webhook/webhook_test.go` | PASS |
| **FR-EVT-008** | AES-256-GCM Endpoint Decryption & HMAC-SHA256 Signatures | `services/messaging/internal/webhook` | `internal/webhook/webhook_test.go` | PASS |
| **FR-EVT-009** | Dead-Letter Queue (DLQ) & Poison Message Quarantine | `services/messaging/internal/consumer` | `internal/consumer/consumer_test.go` | PASS |
| **FR-EVT-010** | Keyset-Paginated Delivery Audit & Attempt Logging | `core-api/internal/events` | `core-api/internal/events/transport/handlers_test.go` | PASS |
| **FR-REC-001** | Stale Outbox Event Detection & Alerting | `services/messaging/internal/reconciler` | `internal/reconciler/reconciler_test.go` | PASS |
| **FR-REC-002** | Expired Worker Delivery Lease Recovery | `services/messaging/internal/reconciler` | `internal/reconciler/reconciler_test.go` | PASS |
| **FR-REC-003** | Cross-Module Data Consistency Auditing (Stub D9) | `services/messaging/internal/reconciler` | `internal/reconciler/reconciler_test.go` | PASS (Partial) |
| **FR-REC-004** | Automated Historical Data Retention Cleanup | `services/messaging/internal/reconciler` | `internal/reconciler/reconciler_test.go` | PASS |
| **FR-REC-005** | Dead-Letter Queue (DLQ) Replay CLI Workflow | `services/messaging/cmd/reconcile` | `internal/reconciler/reconciler_test.go` | PASS |

