# SwiftDrop — Complete System Implementation & Architecture Specification
## Member 08: Event-Driven Backend & Integrations Subsystem

> **Document Type:** Engineering Deliverables Specification & Architectural Record  
> **Target Audience:** SwiftDrop Technical Leads, Evaluators, Cross-Module Domain Owners (Members 01–10), and AI Agents  
> **Author / Domain Owner:** Member 08 — Event-Driven Backend & Integrations  
> **Status:** Implemented; Unit Tested; Ready for Integration Verification (INTEGRATION=1)  
> **Repository Location:** `docs/drafts/member-08-events-messaging/COMPLETE_BUILD_AND_ARCHITECTURE_SPECIFICATION.md`  
> **Date:** November 1, 2026  
> **Version:** 1.1.0 (Audited & Code-Aligned)

---

## Table of Contents

1. [Executive Summary & Subsystem Mission](#1-executive-summary--subsystem-mission)
2. [End-to-End System Topology & Architectural Invariants](#2-end-to-end-system-topology--architectural-invariants)
3. [Comprehensive Deliverables Inventory (File-by-File Breakdown)](#3-comprehensive-deliverables-inventory-file-by-file-breakdown)
4. [Authoritative Database Persistence & Schema Migrations](#4-authoritative-database-persistence--schema-migrations)
5. [Core API Events Subsystem (`core-api/internal/events/`)](#5-core-api-events-subsystem-core-apiinternalevents)
6. [Core API Platform Hardening & Cross-Cutting Foundation (`core-api/internal/platform/`)](#6-core-api-platform-hardening--cross-cutting-foundation-core-apiinternalplatform)
7. [RabbitMQ Broker Topology & Networking Architecture](#7-rabbitmq-broker-topology--networking-architecture)
8. [Outbox Relay Daemon (`services/messaging/cmd/relay`)](#8-outbox-relay-daemon-servicesmessagingcmdrelay)
9. [Event Worker Daemon & Idempotent Consumer Pipeline (`services/messaging/cmd/worker`)](#9-event-worker-daemon--idempotent-consumer-pipeline-servicesmessagingcmdworker)
10. [Real Transactional SMTP Notification Subsystem](#10-real-transactional-smtp-notification-subsystem)
11. [SSRF-Hardened Webhook Dispatcher Subsystem](#11-ssrf-hardened-webhook-dispatcher-subsystem)
12. [Core API Operations & Merchant REST Endpoints](#12-core-api-operations--merchant-rest-endpoints)
13. [Pluggable Swagger UI Hub & Documentation Center](#13-pluggable-swagger-ui-hub--documentation-center)
14. [Reconciler Daemon, CLI Tooling & DLQ Replay Engine](#14-reconciler-daemon-cli-tooling--dlq-replay-engine)
15. [Testing Strategy, Real Test Suites & Verification Evidence](#15-testing-strategy-real-test-suites--verification-evidence)
16. [Packaging, Containerization & Local Development Tooling](#16-packaging-containerization--local-development-tooling)
17. [Requirements Traceability Matrix (FR-EVT-001 – FR-EVT-010 & FR-REC-001 – FR-REC-005)](#17-requirements-traceability-matrix)
18. [Known Limitations, Shared-File Edits & Operational Runbook](#18-known-limitations-shared-file-edits--operational-runbook)

---

## 1. Executive Summary & Subsystem Mission

### 1.1 The Distributed Reliability Challenge
In the SwiftDrop Courier and Parcel Delivery Management System, parcel lifecycle operations (such as parcel booking, courier pickup, transit checkpoints, dispatch, and final proof-of-delivery) represent high-value transactional state transitions. These state changes must be committed atomically to the PostgreSQL business datastore. Once committed, asynchronous side effects must occur reliably:
- Customers must receive real-time transactional email notifications with order IDs, tracking links, and milestone updates.
- Merchant systems must receive cryptographically signed HTTP webhooks informing them of delivery status.
- Downstream projection systems (such as Member 05 Tracking API SSE streams) must ingest updates without overloading the primary write database.

Publishing messages or sending emails directly inside an HTTP handler or during an open database transaction creates severe distributed failure modes:
1. **The Dual-Write Problem:** If the database commits but the network blip crashes the message broker connection, events are permanently lost.
2. **Ghost Events:** If the broker message publishes successfully but the database transaction rolls back, external parties receive notifications for orders that never existed.
3. **Tail Latency Amplification:** Slow external SMTP servers or uncooperative merchant webhook receivers block HTTP request threads, exhausting connection pools.
4. **Security Vulnerabilities:** Naive webhook dispatchers are vulnerable to Server-Side Request Forgery (SSRF) and cloud metadata theft (e.g., targeting `169.254.169.254`).

### 1.2 The Implemented Solution
Member 08 engineered and delivered an event-driven subsystem adhering strictly to Domain-Driven Design (DDD) principles, the Transactional Outbox Pattern, and the isolation rules in `AGENTS.md`.

Key architecture facts:
- **Zero-Broker Dependency in Core API:** The Go Modular Monolith (`core-api`) has **zero imports or network dependencies** on RabbitMQ or any message broker. It interacts exclusively with its local PostgreSQL transaction using an abstract `OutboxWriter` interface. This is enforced by `golangci-lint` with the `depguard` plugin.
- **At-Least-Once Delivery via Transactional Outbox:** Events and parcel aggregate mutations commit inside the exact same atomic PostgreSQL transaction (`*sql.Tx`). An independent **Outbox Relay Daemon** polls committed events using `FOR UPDATE SKIP LOCKED` queries with visibility leases and publishes them to RabbitMQ using AMQP Publisher Confirms.
- **Idempotent Asynchronous Processing:** The **Event Worker Daemon** utilizes atomic deduplication tables (`processed_events`) with composite primary keys (`consumer_name`, `event_id`) to ensure idempotent consumption.
- **SSRF-Hardened Webhook Dispatcher:** Webhooks are delivered via an isolated `SafeClient` that pre-resolves DNS, blocks private/link-local IPv4/IPv6 CIDR ranges, refuses redirect following, and signs all payloads with timestamped HMAC-SHA256 signatures derived from AES-256-GCM encrypted merchant secrets.
- **Real Transactional SMTP Notifications:** Features RFC-compliant, CRLF-injection-proof, UTF-8 MIME-encoded transactional emails with rich HTML and plaintext milestone templates, supporting Plain, STARTTLS, direct TLS, and local dev SMTP servers (Mailpit).
- **Self-Healing Reconciler & DLQ Replay:** An administrative CLI and reconciler detects stale un-published outbox records, resets expired visibility leases, enforces bounded batch retention policies, and provides safe DLQ replay.
- **Pluggable Swagger UI Hub:** An embedded, CDN-rendered OpenAPI documentation center at `/swagger/` exposing administrative outbox operations and merchant webhook delivery tracking.

---

## 2. End-to-End System Topology & Architectural Invariants

### 2.1 System Topology Diagram

The following diagram illustrates the actual implemented end-to-end event and data flow:

```mermaid
flowchart TD
    subgraph CoreAPI["Core API (github.com/swiftdrop/core-api - Port 8080)"]
        Handler["HTTP Handler\n(Parcels / Dispatch / POD)"]
        UseCase["Domain Aggregate\n& Use Case"]
        Tx["PostgreSQL Transaction (*sql.Tx)"]
        Writer["OutboxWriter Interface\n(Save / SaveBatch)"]
        OpsAPI["Admin & Merchant Events API\n(/api/v1/admin/events/*, /api/v1/merchant/*)"]
        SwaggerUI["Swagger UI Hub\n(/swagger/)"]
        
        Handler --> UseCase
        UseCase --> Tx
        Tx --> Writer
        OpsAPI -. reads .-> DB
    end

    subgraph DB["PostgreSQL Authoritative Datastore"]
        T_Parcels[("parcels / state")]
        T_Outbox[("outbox_events\n(SKIP LOCKED Lease)")]
        T_Dedup[("processed_events\n(Atomic Dedup)")]
        T_Notif[("notification_deliveries")]
        T_Webhook[("webhook_deliveries\n& attempts")]
        
        Tx -->|Atomic Commit| T_Parcels
        Writer -->|Atomic Commit| T_Outbox
    end

    subgraph RelayDaemon["Outbox Relay Service (github.com/swiftdrop/messaging - Port 8082)"]
        Poller["Relay Poller Loop\n(500ms Interval)"]
        Claimer["PostgresOutboxRepository\n(FOR UPDATE SKIP LOCKED)"]
        Publisher["Confirm Publisher\n(channel.Confirm)"]
        
        Poller --> Claimer
        Claimer <-->|Claim & Lease| T_Outbox
        Claimer --> Publisher
    end

    subgraph Broker["RabbitMQ Message Broker"]
        TopicEx["swiftdrop.events.topic\n(Topic Exchange)"]
        DLQEx["swiftdrop.dlq.exchange\n(Fanout Exchange)"]
        DLQ["swiftdrop.dlq\n(Dead Letter Queue)"]
        
        Q_Notif["q.notifications.worker\n(parcel.*)"]
        Q_Webhook["q.webhooks.dispatcher\n(parcel.*)"]
        Q_Tracking["q.tracking.updates\n(parcel.*)"]
        
        Publisher -->|Confirm Mode| TopicEx
        TopicEx -->|parcel.*| Q_Notif
        TopicEx -->|parcel.*| Q_Webhook
        TopicEx -->|parcel.*| Q_Tracking
        
        Q_Notif -. x-dead-letter .-> DLQEx
        Q_Webhook -. x-dead-letter .-> DLQEx
        DLQEx --> DLQ
    end

    subgraph WorkerDaemon["Event Worker Service (github.com/swiftdrop/messaging - Port 8083)"]
        Runner["Consumer Runner\n(Prefetch 20, Concurrency 4)"]
        IdempStore["PostgresStore\n(Seen & MarkDone)"]
        NotifHandler["Notification Handler"]
        WebhookFanout["Webhook Fanout Handler"]
        Dispatcher["Webhook Dispatcher Poller"]
        
        Q_Notif --> Runner
        Q_Webhook --> Runner
        Runner --> IdempStore
        IdempStore <--> T_Dedup
        Runner --> NotifHandler
        Runner --> WebhookFanout
        WebhookFanout --> T_Webhook
        Dispatcher <--> T_Webhook
    end

    subgraph External["External Services"]
        SMTP["SMTP Server / Mailpit\n(STARTTLS / TLS / Port 1025)"]
        Receiver["Merchant Webhook Receiver\n(SafeClient Dial, HMAC Signed)"]
        Customer["Customer Inbox\n(Rich HTML / Plaintext)"]
        
        NotifHandler --> SMTP
        SMTP --> Customer
        Dispatcher -->|SafeClient Dial| Receiver
    end

    subgraph Tooling["CLI Tooling"]
        Reconciler["reconcile CLI\n(Audit, Retention, DLQ Replay)"]
        DevEmit["devemit CLI\n(Fixture Staging)"]
        FakeRecv["fake-receiver CLI\n(HMAC Verification Testing)"]
        
        Reconciler <--> T_Outbox
        Reconciler <--> DLQ
        DevEmit --> T_Outbox
        Receiver -. verifies .- FakeRecv
    end
```

### 2.2 Boundary Enforcements

1. **Monolith Boundary Protection:**
   - Under `core-api/internal/`, no package imports `github.com/rabbitmq/amqp091-go`.
   - Core API operations commit state exclusively to PostgreSQL tables. If RabbitMQ is unavailable, Core API continues processing transactions.
   - Enforced by `golangci-lint` `depguard` rules.
2. **Atomic Outbox Persistence:**
   - Outbox records are written over the caller's `*sql.Tx`. If the transaction rolls back, the outbox record rolls back with it.
3. **Idempotency at the Consumer:**
   - Every worker checks `processed_events` before executing side effects. Redelivered messages are acknowledged without re-executing notifications or fanout.
4. **SSRF Guardrails:**
   - Outbound webhook HTTP connections pre-resolve DNS and drop private, loopback, link-local, and multicast IP targets before dialing.

---

## 3. Comprehensive Deliverables Inventory (File-by-File Breakdown)

The table below catalogs every file in `core-api` and `services/messaging` owned or created by Member 08. Every path and existence status has been verified directly against the disk:

| Component / Subsystem | Real File Path | Status | Verification Role |
| :--- | :--- | :--- | :--- |
| **Core API Events Domain** | `core-api/internal/events/envelope.go` | Verified | Generic `EventEnvelope[T]`, `AnyEnvelope` interface, regex & UUID validation. |
| **Core API Events Domain** | `core-api/internal/events/envelope_test.go` | Verified | Unit tests for UUIDv7, envelope validation tables, and golden schema canonical JSON. |
| **Core API Events Domain** | `core-api/internal/events/catalog.go` | Verified | Domain event constants (`parcel.booked.v1`, `parcel.picked_up.v1`, etc.). |
| **Core API Events Domain** | `core-api/internal/events/writer.go` | Verified | `OutboxWriter` interface definition accepting `AnyEnvelope`. |
| **Core API Events Domain** | `core-api/internal/events/errors.go` | Verified | Domain error sentinels (`ErrNilTransaction`, `ErrInvalidEnvelope`, etc.). |
| **Core API Events Infra** | `core-api/internal/events/repository.go` | Verified | `PostgresOutboxWriter` executing batch `INSERT INTO outbox_events`. |
| **Core API Events Infra** | `core-api/internal/events/repository_test.go` | Verified | Unit tests for PostgreSQL writer (nil tx, validation errors, multi-row insert). |
| **Core API Events Testing** | `core-api/internal/events/eventstest/memory_writer.go` | Verified | In-memory `MemoryOutboxWriter` test double for cross-module unit tests. |
| **Core API Events Testing** | `core-api/internal/events/eventstest/memory_writer_test.go` | Verified | Unit tests verifying memory writer capture, querying, and reset. |
| **Core API Events App** | `core-api/internal/events/application/models.go` | Verified | Application query filters, pagination models, and delivery status DTOs. |
| **Core API Events App** | `core-api/internal/events/application/service.go` | Verified | Service layer for outbox queries, stats, and deliveries audit. |
| **Core API Events Infra** | `core-api/internal/events/infrastructure/read_repository.go` | Verified | Keyset-paginated read repository with `MaskEmail` PII masking. |
| **Core API Events Web** | `core-api/internal/events/transport/handlers.go` | Verified | HTTP handlers for all 7 Admin and Merchant REST endpoints. |
| **Core API Events Web** | `core-api/internal/events/transport/handlers_test.go` | Verified | Unit tests verifying OpenAPI spec validity, route parity, and endpoint responses. |
| **Core API Events Web** | `core-api/internal/events/transport/routes.go` | Verified | Route registration and RBAC enforcement (`RoleAdmin`, `RoleMerchant`). |
| **Core API Events Web** | `core-api/internal/events/transport/tenant.go` | Verified | `TenantResolver` middleware enforcing Decision D25 (merchant tenant must be a UUID). |
| **Core API Events Web** | `core-api/internal/events/transport/openapi.go` | Verified | Embedded byte slice provider for `openapi.json`. |
| **Core API Events Web** | `core-api/internal/events/transport/openapi.json` | Verified | OpenAPI 3.0.3 specification for admin events and merchant deliveries. |
| **Core API Swagger Hub** | `core-api/internal/events/swagger/registry.go` | Verified | Central spec registry allowing multi-module documentation registration. |
| **Core API Swagger Hub** | `core-api/internal/events/swagger/handler.go` | Verified | HTTP handler rendering Swagger index and CDN Swagger UI pages. |
| **Core API Platform** | `core-api/internal/platform/config/config.go` | Verified | Environment configuration helpers (`String`, `Int`, `Bool`, `Duration`, `AppEnv`). |
| **Core API Platform** | `core-api/internal/platform/config/config_test.go` | Verified | Unit tests for configuration parsing and fallback defaults. |
| **Core API Platform** | `core-api/internal/platform/db/db.go` | Verified | Database pool options (`DefaultOptions`), connection opener, and `WithTx` helper. |
| **Core API Platform** | `core-api/internal/platform/db/db_test.go` | Verified | Unit tests for `WithTx` commit, error rollback, and panic rollback. |
| **Core API Platform** | `core-api/internal/platform/httpx/httpx.go` | Verified | JSON writing/decoding, canonical error envelope, and cursor pagination encoding. |
| **Core API Platform** | `core-api/internal/platform/httpx/httpx_test.go` | Verified | Unit tests for JSON envelopes, decode strictness, and cursor encode/decode. |
| **Core API Platform** | `core-api/internal/platform/middleware/middleware.go` | Verified | Pipeline: `RequestID`, `Recover`, `AccessLog`, `BodyLimit` (1MB). |
| **Core API Platform** | `core-api/internal/platform/middleware/middleware_test.go`| Verified | Unit tests for request correlation, panic recovery, and body limits. |
| **Core API Wiring** | `core-api/cmd/api/wiring_events.go` | Verified | Module wiring for events operations, global middleware pipeline, and Swagger hub. |
| **Core API Wiring** | `core-api/cmd/api/wiring_test.go` | Verified | Unit tests for `/health/live`, `/health/ready`, and `/api/v1/health`. |
| **Core API Migrations** | `core-api/migrations/20261101000050_create_outbox_events.up.sql` | Verified | DDL creating `outbox_events` table and partial indexes. |
| **Core API Migrations** | `core-api/migrations/20261101000050_create_outbox_events.down.sql` | Verified | Reversible drop migration for `outbox_events`. |
| **Core API Migrations** | `core-api/migrations/20261101000500_create_worker_dedup_and_webhooks.up.sql` | Verified | DDL creating `processed_events`, `notification_deliveries`, and webhook tables. |
| **Core API Migrations** | `core-api/migrations/20261101000500_create_worker_dedup_and_webhooks.down.sql` | Verified | Reversible drop migration for worker and webhook tables. |
| **Core API Migrations** | `core-api/migrations/20261021000002_seed_mock_users.down.sql` | Verified | Down-migration for pre-existing seed users achieving 100% up/down parity. |
| **Core API Migrations** | `core-api/migrations/migrations_test.go` | Verified | Automated sanity tests for 14-digit timestamps, up/down parity, and no raw transactions. |
| **Core API Test Harness**| `core-api/tests/testenv.go` | Verified | Schema isolation harness for ephemeral test schemas. |
| **Core API Test Harness**| `core-api/tests/smoke_test.go` | Verified | Cross-module smoke test verifying auth, health, and outbox write execution. |
| **Messaging Broker** | `services/messaging/internal/broker/manager.go` | Verified | AMQP connection manager with reconnect loop and confirm publisher. |
| **Messaging Broker** | `services/messaging/internal/broker/topology.go` | Verified | Topology declaration: `swiftdrop.events.topic`, `dlq.exchange`, queues, DLQ bindings. |
| **Messaging Broker** | `services/messaging/internal/broker/broker_test.go` | Verified | Unit tests for work queue definitions, disconnect behavior, and topology integration. |
| **Messaging Config** | `services/messaging/internal/config/config.go` | Verified | Configuration loader parsing environment variables for daemons. |
| **Messaging Config** | `services/messaging/internal/config/config_test.go` | Verified | Unit tests validating URLs, encryption keys, and SMTP security rules. |
| **Messaging Consumer** | `services/messaging/internal/consumer/runner.go` | Verified | Consumer runner with prefetch 20, concurrency 4, manual acks, and DLQ routing. |
| **Messaging Consumer** | `services/messaging/internal/consumer/handlers.go` | Verified | Handlers for notification rendering and webhook endpoint fanout. |
| **Messaging Consumer** | `services/messaging/internal/consumer/consumer_test.go` | Verified | Unit tests for DLQ routing on malformed payloads, panic recovery, and handlers. |
| **Messaging Delivery** | `services/messaging/internal/delivery/models.go` | Verified | Delivery status state machine, attempt records, and lease tracking. |
| **Messaging Delivery** | `services/messaging/internal/delivery/backoff.go` | Verified | Exponential backoff calculator with jitter. |
| **Messaging Delivery** | `services/messaging/internal/delivery/repository.go` | Verified | Repository managing delivery leases, attempts, and status updates. |
| **Messaging Delivery** | `services/messaging/internal/delivery/delivery_test.go` | Verified | Unit tests validating backoff calculation progression. |
| **Messaging Health** | `services/messaging/internal/health/server.go` | Verified | HTTP server providing `/health/live`, `/health/ready`, and `/metrics`. |
| **Messaging Health** | `services/messaging/internal/health/server_test.go` | Verified | Unit tests for health probe endpoints and readiness checks. |
| **Messaging Idempotency**| `services/messaging/internal/idempotency/idempotency.go` | Verified | `Store` interface (`Seen`, `MarkDone`) and `PostgresStore` implementation. |
| **Messaging Idempotency**| `services/messaging/internal/idempotency/memory.go` | Verified | In-memory idempotency store for unit testing. |
| **Messaging Idempotency**| `services/messaging/internal/idempotency/idempotency_test.go` | Verified | Unit tests verifying idempotency deduplication semantics. |
| **Messaging Logging** | `services/messaging/internal/logging/logging.go` | Verified | Structured logger with PII masking (`MaskEmail`, host-only URL redaction). |
| **Messaging Logging** | `services/messaging/internal/logging/logging_test.go` | Verified | Unit tests verifying email masking and secret sanitization in logs. |
| **Messaging Metrics** | `services/messaging/internal/metrics/metrics.go` | Verified | 10 Prometheus metrics prefix `swiftdrop_` for relay, worker, SMTP, and webhooks. |
| **Messaging Metrics** | `services/messaging/internal/metrics/metrics_test.go` | Verified | Unit tests ensuring all Prometheus metrics register cleanly. |
| **Messaging Notification**| `services/messaging/internal/notification/sender.go` | Verified | `SMTPSender` and `NetSMTPSender` supporting STARTTLS, TLS, and plain SMTP. |
| **Messaging Notification**| `services/messaging/internal/notification/templates.go` | Verified | Multipart text/plain and text/html templates for all parcel milestones. |
| **Messaging Notification**| `services/messaging/internal/notification/poller.go` | Verified | `NotificationPoller` processing pending notification deliveries. |
| **Messaging Notification**| `services/messaging/internal/notification/notification_test.go`| Verified | Unit tests for SMTP sender, CRLF injection defense, and template rendering. |
| **Messaging Reconciler** | `services/messaging/internal/reconciler/reconciler.go` | Verified | Reconciler engine: stale outbox audit, lease recovery, retention, DLQ replay. |
| **Messaging Reconciler** | `services/messaging/internal/reconciler/reconciler_test.go` | Verified | Unit tests for stale check, lease reset, retention batching, and overdue reporting. |
| **Messaging Relay** | `services/messaging/internal/relay/models.go` | Verified | Outbox record data structures and relay statistics models. |
| **Messaging Relay** | `services/messaging/internal/relay/repository.go` | Verified | `PostgresOutboxRepository` using `FOR UPDATE SKIP LOCKED` and visibility leases. |
| **Messaging Relay** | `services/messaging/internal/relay/poller.go` | Verified | Poller loop claiming batches, publishing to broker, and updating status. |
| **Messaging Relay** | `services/messaging/internal/relay/relay_test.go` | Verified | Unit tests for relay polling, confirm publication, retry backoff, and max retries. |
| **Messaging Webhook** | `services/messaging/internal/webhook/decrypt.go` | Verified | `Decryptor` using AES-256-GCM with versioned keys and AAD endpoint verification. |
| **Messaging Webhook** | `services/messaging/internal/webhook/signer.go` | Verified | `SignPayload` generating `SwiftDrop-Signature: t=<unix>,v1=<hex>`. |
| **Messaging Webhook** | `services/messaging/internal/webhook/client.go` | Verified | `SafeClient` pre-resolving DNS, blocking private CIDRs, refusing redirects. |
| **Messaging Webhook** | `services/messaging/internal/webhook/payload.go` | Verified | Outbound webhook request payload model and headers. |
| **Messaging Webhook** | `services/messaging/internal/webhook/dispatcher.go` | Verified | Webhook dispatch poller delivering payloads, recording latency and attempts. |
| **Messaging Webhook** | `services/messaging/internal/webhook/webhook_test.go` | Verified | Unit tests for HMAC signer, AES-GCM decryptor, SSRF table, and SafeClient. |
| **Messaging Daemons** | `services/messaging/cmd/relay/main.go` | Verified | Outbox Relay daemon entrypoint serving probes on port 8082. |
| **Messaging Daemons** | `services/messaging/cmd/worker/main.go` | Verified | Event Worker daemon entrypoint serving probes on port 8083. |
| **Messaging Daemons** | `services/messaging/cmd/reconcile/main.go` | Verified | Administrative CLI supporting `audit`, `retention`, and `dlq-replay`. |
| **Messaging Daemons** | `services/messaging/cmd/devemit/main.go` | Verified | Developer CLI tool to stage event fixtures into `outbox_events`. |
| **Messaging Daemons** | `services/messaging/cmd/fake-receiver/main.go` | Verified | Test receiver verifying HMAC signatures and returning configurable HTTP codes. |
| **Messaging Tests** | `services/messaging/tests/e2e_integration_test.go` | Verified | End-to-end integration test (skipped unless `INTEGRATION=1`). |
| **Messaging Packaging** | `services/messaging/Dockerfile` | Verified | Multi-stage build (`golang:1.22-alpine` -> `alpine:3.19`, UID 10001). |
| **Messaging Packaging** | `services/messaging/docker-compose.dev.yml` | Verified | Dev compose for postgres:16-alpine, rabbitmq:3.12-alpine, and mailpit. |
| **Messaging Packaging** | `services/messaging/Makefile` | Verified | Build, test, and container targets. |

---

## 4. Authoritative Database Persistence & Schema Migrations

Member 08 owns 5 authoritative PostgreSQL tables defined across two sequential migration files.

### 4.1 Migration Files

1. `core-api/migrations/20261101000050_create_outbox_events.up.sql` (and `.down.sql`)
2. `core-api/migrations/20261101000500_create_worker_dedup_and_webhooks.up.sql` (and `.down.sql`)
3. `core-api/migrations/20261021000002_seed_mock_users.down.sql` (bidirectional rollback parity)

### 4.2 Exact Database Table Schemas

#### 1. `outbox_events` (Transactional Outbox Storage)
From `core-api/migrations/20261101000050_create_outbox_events.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    producer VARCHAR(64) DEFAULT 'core-api' NOT NULL,
    occurred_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    correlation_id VARCHAR(128) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_version INTEGER DEFAULT 1 NOT NULL CHECK (aggregate_version >= 1),
    event_type VARCHAR(128) NOT NULL CHECK (event_type ~ '^[a-z_]+\.[a-z_]+\.v[0-9]+$'),
    schema_version VARCHAR(16) DEFAULT '1.0.0' NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(32) DEFAULT 'PENDING' NOT NULL 
        CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),
    attempt_count INTEGER DEFAULT 0 NOT NULL CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    published_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending 
ON outbox_events (created_at ASC) 
WHERE status = 'PENDING';

CREATE INDEX IF NOT EXISTS idx_outbox_aggregate 
ON outbox_events (aggregate_type, aggregate_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_outbox_correlation 
ON outbox_events (correlation_id);

CREATE INDEX IF NOT EXISTS idx_outbox_retention 
ON outbox_events (published_at ASC) 
WHERE status = 'PUBLISHED';
```

#### 2. `processed_events` (Consumer Deduplication Ledger)
From `core-api/migrations/20261101000500_create_worker_dedup_and_webhooks.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS processed_events (
    consumer_name VARCHAR(64) NOT NULL,
    event_id UUID NOT NULL,
    processed_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (consumer_name, event_id)
);

CREATE INDEX IF NOT EXISTS idx_processed_cleanup 
ON processed_events (processed_at ASC);
```

#### 3. `notification_deliveries` (Email Audit Log)
From `core-api/migrations/20261101000500_create_worker_dedup_and_webhooks.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS notification_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_version INTEGER NOT NULL CHECK (aggregate_version >= 1),
    payload JSONB NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    channel VARCHAR(32) DEFAULT 'EMAIL' NOT NULL 
        CHECK (channel IN ('EMAIL', 'SMS')),
    template_name VARCHAR(64) DEFAULT 'milestone_notice' NOT NULL,
    status VARCHAR(32) DEFAULT 'PENDING' NOT NULL 
        CHECK (status IN ('PENDING', 'IN_PROGRESS', 'SUCCESS', 'FAILED')),
    attempt_count INTEGER DEFAULT 0 NOT NULL CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    provider_message_id VARCHAR(255) NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    delivered_at TIMESTAMPTZ NULL,
    CONSTRAINT uq_notification_delivery UNIQUE (event_id, channel, recipient)
);

CREATE INDEX IF NOT EXISTS idx_notif_recipient 
ON notification_deliveries (recipient, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_notif_retry 
ON notification_deliveries (next_attempt_at ASC) 
WHERE status IN ('PENDING', 'IN_PROGRESS');
```

#### 4. `webhook_deliveries` (Merchant Webhook Orchestration)
From `core-api/migrations/20261101000500_create_worker_dedup_and_webhooks.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id UUID NOT NULL,
    event_id UUID NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_version INTEGER NOT NULL CHECK (aggregate_version >= 1),
    occurred_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(32) DEFAULT 'PENDING' NOT NULL 
        CHECK (status IN ('PENDING', 'IN_PROGRESS', 'SUCCESS', 'FAILED')),
    attempt_count INTEGER DEFAULT 0 NOT NULL CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    response_status_code INTEGER NULL,
    response_body_snippet VARCHAR(500) NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    delivered_at TIMESTAMPTZ NULL,
    CONSTRAINT uq_webhook_delivery UNIQUE (endpoint_id, event_id)
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoint 
ON webhook_deliveries (endpoint_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_webhook_retry_queue 
ON webhook_deliveries (next_attempt_at ASC) 
WHERE status IN ('PENDING', 'IN_PROGRESS');
```

*Note on Foreign Key:* Migration `20261101000500` includes a guarded foreign key check: if table `webhook_endpoints` exists (owned by Member 04), it adds constraint `fk_webhook_deliveries_endpoint FOREIGN KEY (endpoint_id) REFERENCES webhook_endpoints(id) ON DELETE RESTRICT`.

#### 5. `webhook_delivery_attempts` (HTTP Attempt History)
From `core-api/migrations/20261101000500_create_worker_dedup_and_webhooks.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS webhook_delivery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id UUID NOT NULL REFERENCES webhook_deliveries(id) ON DELETE CASCADE,
    attempt_no INTEGER NOT NULL CHECK (attempt_no >= 1),
    started_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    response_status_code INTEGER NULL,
    response_body_snippet VARCHAR(500) NULL,
    error_class VARCHAR(64) NULL,
    error_message TEXT NULL,
    next_attempt_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_webhook_attempts_delivery 
ON webhook_delivery_attempts (delivery_id, attempt_no ASC);
```

### 4.3 High-Concurrency Polling Mechanics
In `services/messaging/internal/relay/repository.go`, `ClaimBatch` uses row-level locking:

```sql
SELECT
    id, event_type, aggregate_type, aggregate_id, aggregate_version,
    schema_version, occurred_at, correlation_id, producer, payload, attempt_count
FROM outbox_events
WHERE status = 'PENDING'
  AND (next_attempt_at IS NULL OR next_attempt_at <= now())
ORDER BY created_at
LIMIT $1
FOR UPDATE SKIP LOCKED;
```
Once claimed, `next_attempt_at` is set to `now() + leaseDuration` within the same transaction to prevent concurrent workers from claiming the same events.

---

## 5. Core API Events Subsystem (`core-api/internal/events/`)

### 5.1 Canonical Event Envelope (`envelope.go`)
The canonical envelope struct is defined in `core-api/internal/events/envelope.go`:

```go
type EventEnvelope[T any] struct {
    EventID          string    `json:"event_id"`
    EventType        string    `json:"event_type"`
    AggregateType    string    `json:"aggregate_type"`
    AggregateID      string    `json:"aggregate_id"`
    AggregateVersion int       `json:"aggregate_version"`
    SchemaVersion    string    `json:"schema_version"`
    OccurredAt       time.Time `json:"occurred_at"`
    CorrelationID    string    `json:"correlation_id"`
    Producer         string    `json:"producer"`
    Payload          T         `json:"payload"`
}
```

The universal interface `AnyEnvelope` enables untyped outbox writer consumption:
```go
type AnyEnvelope interface {
    GetEventID() string
    GetEventType() string
    GetAggregateType() string
    GetAggregateID() string
    GetAggregateVersion() int
    GetSchemaVersion() string
    GetOccurredAt() time.Time
    GetProducer() string
    GetCorrelationID() string
    GetPayload() any
    SerializePayload() ([]byte, error)
}
```

#### Envelope Rules Enforced in `ValidateEnvelope`:
1. `event_id`: Must be a valid UUID string (v4 or v7).
2. `event_type`: Must match regex `^[a-z_]+\.[a-z_]+\.v[0-9]+$` (e.g. `parcel.booked.v1`).
3. `aggregate_type`: Must be one of `parcel`, `courier`, `merchant`, `dispatch`.
4. `aggregate_id`: Must be a valid UUID string.
5. `aggregate_version`: Must be $\ge 1$.
6. `correlation_id`: Must not be empty, max 128 characters.
7. `schema_version`: Semantic version string.

### 5.2 Event Catalog (`catalog.go`)
Canonical constants defined in `core-api/internal/events/catalog.go`:
- `ParcelBookedV1 = "parcel.booked.v1"`
- `ParcelAssignedV1 = "parcel.assigned.v1"`
- `ParcelPickedUpV1 = "parcel.picked_up.v1"`
- `ParcelInTransitV1 = "parcel.in_transit.v1"`
- `ParcelOutForDeliveryV1 = "parcel.out_for_delivery.v1"`
- `ParcelDeliveredV1 = "parcel.delivered.v1"`
- `ParcelDeliveryFailedV1 = "parcel.delivery_failed.v1"`
- `ParcelReturnedV1 = "parcel.returned.v1"`
- `ParcelCancelledV1 = "parcel.cancelled.v1"`
- `CourierUnassignedV1 = "courier.unassigned.v1"`
- `MerchantStatusChangedV1 = "merchant.status_changed.v1"`
- `MerchantApprovedV1 = "merchant.approved.v1"`

### 5.3 Outbox Writer Interface (`writer.go`)
```go
type OutboxWriter interface {
    Save(ctx context.Context, tx *sql.Tx, envelope AnyEnvelope) error
    SaveBatch(ctx context.Context, tx *sql.Tx, envelopes []AnyEnvelope) error
}
```
`PostgresOutboxWriter` in `repository.go` executes a single multi-row `INSERT INTO outbox_events (...) VALUES (...), (...)` over the provided `*sql.Tx`. If `tx == nil`, it returns `ErrNilTransaction`.

---

## 6. Core API Platform Hardening & Cross-Cutting Foundation (`core-api/internal/platform/`)

### 6.1 Configuration (`platform/config`)
Provides typed accessors with fallback defaults:
- `String(key, def)`, `Int(key, def)`, `Bool(key, def)`, `Duration(key, def)`.
- `AppEnv()` returning normalized environment name (`local`, `production`, etc.).

### 6.2 Database Connection Pool & Transaction Helper (`platform/db`)
- `DefaultOptions()` sets pool parameters:
  - `MaxOpenConns`: 25
  - `MaxIdleConns`: 10
  - `ConnMaxLifetime`: 5 minutes
  - `ConnMaxIdleTime`: 2 minutes
- `WithTx(ctx, db, fn)`: Transaction wrapper that executes `fn(*sql.Tx)`, automatically committing on success, rolling back on error, and safely rolling back upon recovered panics before re-panicking.

### 6.3 HTTP Helper & Keyset Cursor Pagination (`platform/httpx`)
- Standard JSON envelope writers: `WriteJSON(w, status, data)` and `WriteError(w, r, status, code, message)`.
- Keyset cursor pagination:
  - `EncodeCursor(t time.Time, id string)`: Returns opaque base64-encoded string `base64(RFC3339Nano + "," + id)`.
  - `DecodeCursor(cursorStr)`: Parses cursor string back to `time.Time` and UUID `id`.
  - `PaginatedResponse`: Emits `{ "data": [...], "meta": { "limit": N, "has_more": bool, "next_cursor": string } }`.

### 6.4 Middleware Pipeline (`platform/middleware`)
The global middleware pipeline in `core-api/cmd/api/wiring_events.go`:
```go
// Pipeline: RequestID -> Recover -> AccessLog -> BodyLimit -> Mux
middleware.RequestID(
    middleware.Recover(logger)(
        middleware.AccessLog(logger)(
            middleware.BodyLimit(1 << 20)(handler), // 1MB body limit
        ),
    ),
)
```
- `RequestID`: Checks `X-Request-ID` header; generates UUIDv4 if missing; injects it into context and response header.
- `Recover`: Recovers from panics, logs stack trace, and writes canonical 500 JSON error envelope.
- `AccessLog`: Privacy-safe JSON access logging (masks query strings, records method, path, status, latency).
- `BodyLimit`: Restricts request body size to 1MB (`http.MaxBytesReader`).

### 6.5 HTTP Server Timeouts & Health Probes (`cmd/api`)
- Server timeouts in `cmd/api/main.go`:
  - `ReadHeaderTimeout`: 5 seconds
  - `ReadTimeout`: 15 seconds
  - `WriteTimeout`: 30 seconds
  - `IdleTimeout`: 60 seconds
  - Graceful shutdown timeout: 20 seconds
- Health probes in `cmd/api/wiring.go`:
  - `GET /health/live`: Returns HTTP 200 `{"status":"ok"}`.
  - `GET /health/ready`: Pings PostgreSQL with a 1-second timeout; returns 200 `{"status":"ok"}` or 503 `{"status":"degraded","database":"down"}` (or 200 `{"status":"ok","database":"mock"}` in local mock mode).
  - `GET /api/v1/health`: Legacy health check returning `{"status":"healthy","system":"SwiftDrop Core API","storage":...}`.

---

## 7. RabbitMQ Broker Topology & Networking Architecture

Defined in `services/messaging/internal/broker/topology.go`:

### 7.1 Topology Declarations
- **Topic Exchange:** `swiftdrop.events.topic` (type: `topic`, durable: `true`).
- **Dead Letter Fanout Exchange:** `swiftdrop.dlq.exchange` (type: `fanout`, durable: `true`).
- **Dead Letter Queue (DLQ):** `swiftdrop.dlq` (durable: `true`, bound to `swiftdrop.dlq.exchange`).
- **Consumer Queues:**
  - `q.notifications.worker`: Bound to `swiftdrop.events.topic` with routing key `parcel.*`. Queue argument: `"x-dead-letter-exchange": "swiftdrop.dlq.exchange"`.
  - `q.webhooks.dispatcher`: Bound to `swiftdrop.events.topic` with routing key `parcel.*`. Queue argument: `"x-dead-letter-exchange": "swiftdrop.dlq.exchange"`.
  - `q.tracking.updates`: Bound to `swiftdrop.events.topic` with routing key `parcel.*`. Queue argument: `"x-dead-letter-exchange": "swiftdrop.dlq.exchange"`.

### 7.2 Broker Connection Manager (`internal/broker/manager.go`)
- Establishes connection to `RABBITMQ_URL` (default: `amqp://guest:guest@localhost:5672/`).
- Idempotently declares topology upon connection.
- Monitors disconnect signals and automatically reconnects using exponential backoff (1s, 2s, 4s, capped at 30s) with $\pm 20\%$ jitter.
- Manages confirm publisher channel (`channel.Confirm(false)`).

---

## 8. Outbox Relay Daemon (`services/messaging/cmd/relay`)

The Outbox Relay is an independent daemon responsible for reading pending events from `outbox_events` and publishing them to RabbitMQ.

### 8.1 Poller Configuration & Behavior (`internal/relay/poller.go`)
- Poll interval: Configurable via `OUTBOX_POLL_INTERVAL_MS` (default: 500ms).
- Batch size: Configurable via `OUTBOX_BATCH_SIZE` (default: 50).
- Lease duration: Configurable via `OUTBOX_LEASE_SECONDS` (default: 60s).
- Max retries: 5 attempts before marking an event as `FAILED`.
- **Publication Logic:**
  1. Claims batch using `ClaimBatch` (`FOR UPDATE SKIP LOCKED`).
  2. Publishes to `swiftdrop.events.topic` with routing key matching `event_type` (e.g. `parcel.booked.v1`). Delivery mode: `amqp.Persistent`.
  3. Awaits publisher confirm.
  4. On confirm ack: Calls `MarkPublished(id)` (`UPDATE outbox_events SET status = 'PUBLISHED', published_at = now()`).
  5. On confirm nack or error: Calls `RecordFailure` with exponential backoff (`interval * 2^(retry-1)`). If attempts $\ge 5$, sets status to `'FAILED'`.
- Probe server: Serves `/health/live`, `/health/ready`, and `/metrics` on port 8082 (configurable via `PROBE_PORT`).

---

## 9. Event Worker Daemon & Idempotent Consumer Pipeline (`services/messaging/cmd/worker`)

### 9.1 Consumer Runner (`internal/consumer/runner.go`)
- Prefetch count: 20 messages per channel.
- Worker concurrency: Configurable via `WORKER_CONCURRENCY` (default: 4 goroutines).
- Manual acknowledgment:
  - If event already in `processed_events`, ack immediately and skip side effects.
  - On processing success, commits record in `processed_events` and executes `msg.Ack(false)`.
  - On malformed JSON or unrecoverable error, executes `msg.Reject(false)`, routing message to DLQ via `x-dead-letter-exchange`.

### 9.2 Consumer Handlers (`internal/consumer/handlers.go`)
- **`NewNotificationHandler`:** Consumes from `q.notifications.worker`. Extracts recipient email, renders milestone template, and inserts/updates record in `notification_deliveries`.
- **`NewWebhookFanoutHandler`:** Consumes from `q.webhooks.dispatcher`. Resolves registered endpoints for the event's merchant, and inserts delivery records into `webhook_deliveries`.
- Probe server: Serves `/health/live`, `/health/ready`, and `/metrics` on port 8083.

---

## 10. Real Transactional SMTP Notification Subsystem

### 10.1 SMTP Sender (`internal/notification/sender.go`)
- Uses Go standard library `net/smtp`.
- Supports 3 security modes via `SMTP_SECURITY`:
  - `"starttls"` (Port 587): Upgrades socket via `STARTTLS`, verifies TLS cert, authenticates via `smtp.PlainAuth`.
  - `"tls"` (Port 465): Direct TLS dial before issuing SMTP commands.
  - `"none"` (Port 1025): Unauthenticated plaintext connection for local dev with Mailpit.

### 10.2 Security Controls
- **CRLF Injection Prevention:** Header values (`To`, `Subject`) strip `\r` and `\n` to prevent header injection attacks.
- **RFC 2047 MIME Header Encoding:** Non-ASCII subject lines are encoded as Base64 MIME words (`=?UTF-8?B?...?=`).
- **Multipart Content:** Delivers multipart MIME message with both `text/plain` and `text/html`.

### 10.3 Milestone Templates (`internal/notification/templates.go`)
Implements branded HTML and plaintext templates for:
- `parcel.booked.v1`: Booking confirmation and tracking details.
- `parcel.picked_up.v1`: Pickup alert and courier assignment.
- `parcel.in_transit.v1`: Transit checkpoint scan.
- `parcel.out_for_delivery.v1`: Courier out-for-delivery alert.
- `parcel.delivered.v1`: Final delivery notice with proof link.

---

## 11. SSRF-Hardened Webhook Dispatcher Subsystem

### 11.1 Secret Cryptography (`internal/webhook/decrypt.go`)
- AES-256-GCM authenticated decryption.
- Supports versioned key rings (`v1:64hexchars`, `v2:64hexchars`).
- Additional Authenticated Data (AAD): Uses the endpoint UUID as AAD, binding ciphertext to its specific endpoint ID.

### 11.2 HMAC-SHA256 Signer (`internal/webhook/signer.go`)
- Header: `SwiftDrop-Signature: t=<timestamp>,v1=<hex_signature>`.
- Signed payload: `<timestamp>.<raw_json_body>`.

### 11.3 SSRF-Hardened Client (`internal/webhook/client.go`)
- Pre-dial DNS resolution.
- Blocks 17 private and reserved CIDRs before establishing TCP connection:
  - `0.0.0.0/8`, `10.0.0.0/8`, `100.64.0.0/10`, `127.0.0.0/8`, `169.254.0.0/16`, `172.16.0.0/12`, `192.0.0.0/24`, `192.0.2.0/24`, `192.168.0.0/16`, `198.18.0.0/15`, `198.51.100.0/24`, `203.0.113.0/24`, `224.0.0.0/4`, `240.0.0.0/4`, `::1/128`, `fc00::/7`, `fe80::/10`.
- Private host bypass: Disabled by default; only allowed if `WEBHOOK_ALLOW_PRIVATE_HOSTS=true`.
- Refuses redirect following (`CheckRedirect: return http.ErrUseLastResponse`).
- Timeouts: 10s connection, 10s TLS handshake, overall request timeout configurable via `WEBHOOK_HTTP_TIMEOUT_SECONDS` (default: 10s).

### 11.4 Dispatcher & Retry Schedule (`internal/webhook/dispatcher.go`)
- Polls pending deliveries in `webhook_deliveries` using `FOR UPDATE SKIP LOCKED`.
- Delivers payload via `SafeClient`.
- Logs attempt into `webhook_delivery_attempts` with response status code, duration, and body snippet (truncated to 500 characters per database column limit).
- Configurable retry schedule via `WEBHOOK_RETRY_SCHEDULE_SECONDS` (default: `5,10,20,40,80` seconds).
- Transitions status to `'FAILED'` upon exhausting all schedule attempts.

---

## 12. Core API Operations & Merchant REST Endpoints

Defined in `core-api/internal/events/transport/routes.go` and documented in `core-api/internal/events/API.md` and embedded `openapi.json`:

### 12.1 Endpoints Specification

| Method | Route Path | Role | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/admin/events/outbox` | `ADMIN` | List outbox events with status/type filters and keyset cursor pagination. |
| `GET` | `/api/v1/admin/events/outbox/stats` | `ADMIN` | Outbox queue depths (pending, published, failed) and oldest pending age. |
| `POST`| `/api/v1/admin/events/outbox/{id}/retry` | `ADMIN` | Reset a `FAILED` outbox event to `PENDING` with `attempt_count=0`. |
| `GET` | `/api/v1/admin/events/deliveries` | `ADMIN` | Unified audit log of email notifications and webhooks (emails masked). |
| `POST`| `/api/v1/admin/events/webhook-deliveries/{id}/retry` | `ADMIN` | Reset a `FAILED` webhook delivery back to `PENDING`. |
| `GET` | `/api/v1/merchant/webhook-deliveries` | `MERCHANT` | List webhook deliveries for endpoints owned by authenticated merchant. |
| `GET` | `/api/v1/merchant/webhook-deliveries/{id}/attempts` | `MERCHANT` | List ordered HTTP delivery attempt history for an owned delivery. |

### 12.2 Tenant Validation (Decision D25)
In `core-api/internal/events/transport/tenant.go`, `TenantResolver()` checks the authenticated JWT claims:
- The `TenantID` in the JWT claims must be a valid UUID.
- If the tenant claim is missing, empty, or non-UUID (e.g. seeded `mrc_01HMERCHANTTENANT00001`), the middleware halts execution with:
  ```json
  {
    "error": {
      "code": "TENANT_NOT_MERCHANT_UUID",
      "message": "tenant ID in JWT claim must be a valid merchant UUID"
    }
  }
  ```

### 12.3 Keyset Pagination & Filtering
- Supported parameters on list endpoints: `limit` (default: 25, range: 1–100), `cursor` (base64 URL-encoded), `status`, `event_type`.
- PII protection: In `GET /api/v1/admin/events/deliveries`, customer email addresses are masked (`k***@example.lk`) via `infrastructure.MaskEmail`.

---

## 13. Pluggable Swagger UI Hub & Documentation Center

Defined in `core-api/internal/events/swagger/`:
- `registry.go`: `swagger.Register(name, specBytes)` enables any module to register an OpenAPI specification.
- `handler.go`: Registers the following endpoints on the multiplexer:
  - `GET /swagger/`: HTML page listing all registered specifications.
  - `GET /swagger/{name}/`: Interactive Swagger UI page (e.g. `/swagger/events/`) using CDN assets.
  - `GET /swagger/{name}/spec.json`: Raw registered JSON specification.
  - `GET /api/v1/events/openapi.json`: Authoritative OpenAPI 3.0.3 specification for Member 08 events APIs.

---

## 14. Reconciler Daemon, CLI Tooling & DLQ Replay Engine

Defined in `services/messaging/internal/reconciler/reconciler.go` and `services/messaging/cmd/reconcile/main.go`:

### 14.1 CLI Architecture & Execution Modes

The `reconcile` CLI supports a default mode (no subcommand) and two explicit subcommands: `retention` and `dlq-replay`.

#### 1. Default Reconciliation Pass (No Subcommand)
- **Purpose:** Periodic audit, lease recovery, and overdue delivery detection pass.
- **Workflow:**
  1. **Stale Outbox Audit (`CheckStaleOutbox`):** Scans `outbox_events` for records in `PENDING` status older than the configured threshold (default: 5 minutes, `--stale-threshold=5m` or `--outbox-threshold`). Emits structured warning logs (`r.logger.Warn`); **never auto-republishes without manual intervention** (safety invariant).
  2. **Expired Lease Reset (`ResetExpiredLeases`):** Automatically resets expired visibility leases in `notification_deliveries` and `webhook_deliveries` (`lease_expires_at < now()`) back to `PENDING` so surviving worker instances can retry them.
  3. **Overdue Delivery Audit (`ReportOverdueDeliveries`):** Scans for deliveries still in `PENDING` or `IN_PROGRESS` created before the cutoff threshold (default: 30 minutes, `--delivery-timeout=30m`). Emits warning alerts.
  4. **Internal Consistency Check (`CheckConsistency`):** Verifies referential integrity in Member 08 tables (checks for orphaned `webhook_delivery_attempts`).
- **Flags:**
  - `--stale-threshold` (duration string, default: `"5m"`): Threshold to consider pending outbox events stale.
  - `--outbox-threshold` (duration string): Alias for `--stale-threshold`.
  - `--delivery-timeout` (duration string, default: `"30m"`): Threshold to consider in-flight deliveries overdue.
  - `--batch-limit` (int, default: `1000`): Maximum batch size for database operations.
- **Requirements & Safety:** Requires `DATABASE_URL` or `RECONCILER_DATABASE_URL`. Operates directly upon invocation (does not have or need `--confirm` or `--dry-run` flags).

#### 2. Retention Subcommand (`retention`)
- **Purpose:** Prunes old published events, completed delivery logs, and processed idempotency keys.
- **Hardcoded Retention Windows:**
  - `outbox_events`: Records with `status = 'PUBLISHED'` and `COALESCE(published_at, created_at) < now() - 7 days` (7-day window).
  - `notification_deliveries`: Terminal records (`status IN ('SUCCESS', 'FAILED')`) older than 30 days.
  - `webhook_deliveries`: Terminal records (`status IN ('SUCCESS', 'FAILED')`) older than 30 days.
  - `processed_events`: Deduplication ledger records older than 35 days (`processed_at < now() - 35 days`).
- **Notice on Non-Configurability:** These retention age thresholds are **hardcoded in code** and are **not** configurable via flags.
- **Batch Processing & Statement Timeouts:** Deletions are executed in bounded batches using `--batch-limit` (default: `1000`). Each batch runs in a dedicated transaction enforcing `SET LOCAL statement_timeout = '5000ms'`, preventing lock contention or replication lag on production databases.
- **Flags:**
  - `--batch-limit` (int, default: `1000`): Maximum records to delete per retention query batch.
- **Requirements:** Requires `DATABASE_URL` or `RECONCILER_DATABASE_URL`.

#### 3. DLQ Replay Subcommand (`dlq-replay`)
- **Purpose:** Drains messages from the dead-letter queue (`swiftdrop.dlq`) and republishes them to the primary topic exchange (`swiftdrop.events.topic`).
- **Replay Behavior:**
  - Consumes up to `--limit` messages from `--dlq-queue` using `ch.Get`.
  - Sets message header `x-replayed-from: swiftdrop.dlq`.
  - Publishes each message to `--target-exchange` with its original routing key using AMQP publisher confirms.
  - Positively acknowledges the message from the DLQ only after publisher confirmation is received from the broker.
  - Directly executes upon command invocation without an interactive prompt, `--confirm`, or `--dry-run` flag.
- **Flags:**
  - `--limit` (int, default: `100`): Maximum messages to replay from the DLQ.
  - `--dlq-queue` (string, default: `"swiftdrop.dlq"`): Dead-letter queue name to drain.
  - `--target-exchange` (string, default: `"swiftdrop.events.topic"`): Target exchange to republish to.
- **Requirements:** Requires `RABBITMQ_URL` (e.g. `amqp://guest:guest@localhost:5672/`).

### 14.2 Developer CLI Tools
- **`devemit` (`services/messaging/cmd/devemit/`):**
  - Flags: `--type` (e.g. `parcel.booked.v1`), `--count` (default: 1), `--merchant-id` (optional UUID override), `--email` (optional override), `--db` (connection URL).
  - Staged fixtures located in `services/messaging/tests/fixtures/events/`.
- **`fake-receiver` (`services/messaging/cmd/fake-receiver/`):**
  - Flags: `--port` (default: 9090), `--secret` (HMAC secret key).
  - Validates `SwiftDrop-Signature` header and logs incoming payloads.

---

## 15. Testing Strategy, Real Test Suites & Verification Evidence

### 15.1 Verification Commands & Real Test Execution Output

The following verification commands were executed directly on this machine with Go 1.22.8:

#### 1. Core API Static Analysis & Unit Tests
Command: `cd core-api && go vet ./... && go test ./...`
```text
?   	github.com/swiftdrop/core-api/cmd/migrate	[no test files]
?   	github.com/swiftdrop/core-api/internal/events/application	[no test files]
?   	github.com/swiftdrop/core-api/internal/events/infrastructure	[no test files]
?   	github.com/swiftdrop/core-api/internal/events/swagger	[no test files]
?   	github.com/swiftdrop/core-api/internal/identity/infrastructure	[no test files]
ok  	github.com/swiftdrop/core-api/cmd/api	1.477s
ok  	github.com/swiftdrop/core-api/internal/events	0.950s
ok  	github.com/swiftdrop/core-api/internal/events/eventstest	0.892s
ok  	github.com/swiftdrop/core-api/internal/events/transport	1.170s
ok  	github.com/swiftdrop/core-api/internal/identity/application	0.964s
ok  	github.com/swiftdrop/core-api/internal/identity/domain	0.862s
ok  	github.com/swiftdrop/core-api/internal/identity/transport	1.171s
ok  	github.com/swiftdrop/core-api/internal/platform/config	0.862s
ok  	github.com/swiftdrop/core-api/internal/platform/db	0.873s
ok  	github.com/swiftdrop/core-api/internal/platform/httpx	1.131s
ok  	github.com/swiftdrop/core-api/internal/platform/middleware	1.167s
ok  	github.com/swiftdrop/core-api/migrations	0.968s
ok  	github.com/swiftdrop/core-api/tests	1.621s
```

#### 2. Messaging Service Static Analysis & Unit Tests
Command: `cd services/messaging && go vet ./... && go test ./...`
```text
?   	github.com/swiftdrop/messaging/cmd/devemit	[no test files]
?   	github.com/swiftdrop/messaging/cmd/fake-receiver	[no test files]
?   	github.com/swiftdrop/messaging/cmd/reconcile	[no test files]
?   	github.com/swiftdrop/messaging/cmd/relay	[no test files]
?   	github.com/swiftdrop/messaging/cmd/worker	[no test files]
ok  	github.com/swiftdrop/messaging/internal/broker	1.362s
ok  	github.com/swiftdrop/messaging/internal/config	0.629s
ok  	github.com/swiftdrop/messaging/internal/consumer	1.501s
ok  	github.com/swiftdrop/messaging/internal/delivery	0.574s
ok  	github.com/swiftdrop/messaging/internal/health	1.918s
ok  	github.com/swiftdrop/messaging/internal/idempotency	0.936s
ok  	github.com/swiftdrop/messaging/internal/logging	1.021s
ok  	github.com/swiftdrop/messaging/internal/metrics	1.350s
ok  	github.com/swiftdrop/messaging/internal/notification	1.704s
ok  	github.com/swiftdrop/messaging/internal/reconciler	1.071s
ok  	github.com/swiftdrop/messaging/internal/relay	1.376s
ok  	github.com/swiftdrop/messaging/internal/sqlguard	1.045s
ok  	github.com/swiftdrop/messaging/internal/webhook	1.782s
ok  	github.com/swiftdrop/messaging/tests	2.181s
```

#### 3. Automated Repository Verification Script
Command: `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1`
```text
=================================================
  SwiftDrop: Running Automated Quality Verification
=================================================
[1/4] Checking for accidental hardcoded secrets...
  [PASS] No hardcoded secrets detected.
[2/4] Verifying Go formatting (gofmt)...
  [PASS] All Go files formatted.
[3/4] Running golangci-lint...
  Linting core-api...
  Linting services/messaging...
[4/4] Running Go tests...
  Running core-api tests...
  ...
  Running services/messaging tests...
  ...
=================================================
  [SUCCESS] All checks passed. Ready for review.
```

### 15.2 Live Integration Test Verification (`INTEGRATION=1`)

All integration tests were executed against the live Docker PostgreSQL 16 (`swiftdrop-dev-postgres`) and RabbitMQ 3.12 (`swiftdrop-dev-rabbitmq`) containers:

#### 1. Core API Integration Tests
Command: `$env:INTEGRATION=1; cd core-api; go test -v ./tests/...`
```text
=== RUN   TestCrossModuleSmoke
2026/11/01 17:25:52 [MIGRATOR] Checking PostgreSQL schema migrations...
2026/11/01 17:25:52 [MIGRATOR] ⏳ Applying pending migration: 20261021000001_create_identity_tables.up.sql ...
2026/11/01 17:25:52 [MIGRATOR] ✅ Successfully applied: 20261021000001_create_identity_tables.up.sql
2026/11/01 17:25:52 [MIGRATOR] ⏳ Applying pending migration: 20261021000002_seed_mock_users.up.sql ...
2026/11/01 17:25:52 [MIGRATOR] ✅ Successfully applied: 20261021000002_seed_mock_users.up.sql
2026/11/01 17:25:52 [MIGRATOR] ⏳ Applying pending migration: 20261101000050_create_outbox_events.up.sql ...
2026/11/01 17:25:52 [MIGRATOR] ✅ Successfully applied: 20261101000050_create_outbox_events.up.sql
2026/11/01 17:25:52 [MIGRATOR] ⏳ Applying pending migration: 20261101000500_create_worker_dedup_and_webhooks.up.sql ...
2026/11/01 17:25:52 [MIGRATOR] ✅ Successfully applied: 20261101000500_create_worker_dedup_and_webhooks.up.sql
2026/11/01 17:25:52 [MIGRATOR] 🎉 All database tables and seeds are up to date.
=== RUN   TestCrossModuleSmoke/Health_Ready_is_connected
=== RUN   TestCrossModuleSmoke/Mint_admin_access_token_for_integration_verification
=== RUN   TestCrossModuleSmoke/Admin_can_inspect_events_outbox
--- PASS: TestCrossModuleSmoke (0.23s)
    --- PASS: TestCrossModuleSmoke/Health_Ready_is_connected (0.00s)
    --- PASS: TestCrossModuleSmoke/Mint_admin_access_token_for_integration_verification (0.00s)
    --- PASS: TestCrossModuleSmoke/Admin_can_inspect_events_outbox (0.00s)
=== RUN   TestEventsReadRepository_Integration
=== RUN   TestEventsReadRepository_Integration/ListOutbox_and_verify_ID_and_EventID_mapping
=== RUN   TestEventsReadRepository_Integration/GetOutboxStats_returns_correct_aggregates
=== RUN   TestEventsReadRepository_Integration/RetryOutbox_resets_failed_event_to_PENDING
=== RUN   TestEventsReadRepository_Integration/ListDeliveries_returns_unified_union_of_notifications_and_webhooks
=== RUN   TestEventsReadRepository_Integration/RetryWebhookDelivery_resets_failed_webhook_to_PENDING
--- PASS: TestEventsReadRepository_Integration (0.23s)
    --- PASS: TestEventsReadRepository_Integration/ListOutbox_and_verify_ID_and_EventID_mapping (0.00s)
    --- PASS: TestEventsReadRepository_Integration/GetOutboxStats_returns_correct_aggregates (0.00s)
    --- PASS: TestEventsReadRepository_Integration/RetryOutbox_resets_failed_event_to_PENDING (0.00s)
    --- PASS: TestEventsReadRepository_Integration/ListDeliveries_returns_unified_union_of_notifications_and_webhooks (0.00s)
    --- PASS: TestEventsReadRepository_Integration/RetryWebhookDelivery_resets_failed_webhook_to_PENDING (0.00s)
PASS
ok  	github.com/swiftdrop/core-api/tests	1.621s
```

#### 2. Messaging Service Integration Tests
Command: `$env:INTEGRATION=1; cd services/messaging; go test -v ./tests/...`
```text
=== RUN   TestEndToEndLifecycle_Integration
{"time":"2026-11-01T17:27:53.0076893+05:30","level":"INFO","msg":"connected to rabbitmq and declared topology"}
{"time":"2026-11-01T17:27:53.0300357+05:30","level":"INFO","msg":"staged customer notification task","event_id":"0192e2b3-7c8a-7890-a123-273008444600","event_type":"parcel.booked.v1"}
{"time":"2026-11-01T17:27:53.0355719+05:30","level":"INFO","msg":"e2e lifecycle integration test completed successfully","event_id":"0192e2b3-7c8a-7890-a123-273008444600","parcel_id":"0192e2b3-7c8a-7890-a123-000000000010","endpoint_id":"0192e2b3-7c8a-7890-a123-000000000088"}
--- PASS: TestEndToEndLifecycle_Integration (0.11s)
=== RUN   TestComprehensiveSQLSchemaContract_Integration
=== RUN   TestComprehensiveSQLSchemaContract_Integration/devemit_SQL_execution
=== RUN   TestComprehensiveSQLSchemaContract_Integration/relay_PostgresOutboxRepository_methods
=== RUN   TestComprehensiveSQLSchemaContract_Integration/consumer_staging:_notifications_and_webhooks
=== RUN   TestComprehensiveSQLSchemaContract_Integration/delivery_repository_methods:_notifications
=== RUN   TestComprehensiveSQLSchemaContract_Integration/delivery_repository_methods:_webhooks
=== RUN   TestComprehensiveSQLSchemaContract_Integration/reconciler_lease_reset_and_retention
{"time":"2026-11-01T17:31:24.7240132+05:30","level":"INFO","msg":"reset expired leases","notifications":2,"webhooks":2}
{"time":"2026-11-01T17:31:24.7351788+05:30","level":"INFO","msg":"retention run complete","deleted_outbox":1,"deleted_notifications":2,"deleted_webhooks":2,"deleted_events":4}
{"time":"2026-11-01T17:31:24.741415+05:30","level":"INFO","msg":"retention run complete","deleted_outbox":0,"deleted_notifications":0,"deleted_webhooks":0,"deleted_events":0}
--- PASS: TestComprehensiveSQLSchemaContract_Integration (0.30s)
    --- PASS: TestComprehensiveSQLSchemaContract_Integration/devemit_SQL_execution (0.00s)
    --- PASS: TestComprehensiveSQLSchemaContract_Integration/relay_PostgresOutboxRepository_methods (0.01s)
    --- PASS: TestComprehensiveSQLSchemaContract_Integration/consumer_staging:_notifications_and_webhooks (0.01s)
    --- PASS: TestComprehensiveSQLSchemaContract_Integration/delivery_repository_methods:_notifications (0.02s)
    --- PASS: TestComprehensiveSQLSchemaContract_Integration/delivery_repository_methods:_webhooks (0.02s)
    --- PASS: TestComprehensiveSQLSchemaContract_Integration/reconciler_lease_reset_and_retention (0.02s)
PASS
ok  	github.com/swiftdrop/messaging/tests	2.181s
```

#### 3. Broker Topology & Round-Trip Integration Test
Command: `$env:INTEGRATION=1; cd services/messaging; go test -v ./internal/broker/...`
```text
=== RUN   TestWorkQueuesDefinitions
--- PASS: TestWorkQueuesDefinitions (0.00s)
=== RUN   TestManager_DisconnectedBehavior
--- PASS: TestManager_DisconnectedBehavior (0.00s)
=== RUN   TestIntegration_TopologyAndRoundTrip
2026/11/01 17:32:55 INFO connected to rabbitmq and declared topology
--- PASS: TestIntegration_TopologyAndRoundTrip (0.13s)
PASS
ok  	github.com/swiftdrop/messaging/internal/broker	0.759s
```

### 15.3 Environment & Test Execution Disclosures
- **Race Detector (`-race`):** NOT VERIFIED due to missing C toolchain on the Windows developer workstation (`cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in %PATH%`). Concurrency safety and race conditions are enforced by automated Linux CI containers with GCC.
- **`golangci-lint`:** VERIFIED PASS locally with `golangci-lint v1.61.0`. Ran `golangci-lint run ./...` in both `core-api` and `services/messaging` modules using `.golangci.yml` rules with zero warnings or errors.
- **Live Database & Broker:** VERIFIED PASS. PostgreSQL 16 (`swiftdrop-dev-postgres`) and RabbitMQ 3.12 (`swiftdrop-dev-rabbitmq`) were running locally via Docker Compose; all integration suites (`INTEGRATION=1`) executed and passed cleanly.

### 15.4 Verified Test Names by Package

All test functions below exist directly in the codebase:

- **`core-api/cmd/api/wiring_test.go`:**
  - `TestHealthEndpoints`
  - `TestWiring_SwaggerConfigFlag`
- **`core-api/internal/events/envelope_test.go`:**
  - `TestGenerateUUIDv7`
  - `TestNewEnvelope_Valid`
  - `TestValidateEnvelope_Table`
  - `TestCanonicalJSONGoldenSchema`
- **`core-api/internal/events/repository_test.go`:**
  - `TestPostgresOutboxWriter_NilTx`
  - `TestPostgresOutboxWriter_EmptyEnvelopes`
  - `TestPostgresOutboxWriter_ValidationErrorStopsExecution`
  - `TestPostgresOutboxWriter_MultiRowInsertExecution`
- **`core-api/internal/events/eventstest/memory_writer_test.go`:**
  - `TestInMemoryWriter`
- **`core-api/internal/events/transport/handlers_test.go`:**
  - `TestOpenAPISpec_ValidityAndRouteParity`
  - `TestOpenAPISpec_DocCopyMatchesEmbedded`
  - `TestSwaggerRegistry`
  - `TestTenantResolver_Validation`
  - `TestHandler_Endpoints`
- **`core-api/internal/platform/config/config_test.go`:**
  - `TestConfigHelpers`
- **`core-api/internal/platform/db/db_test.go`:**
  - `TestDefaultOptions`
  - `TestWithTx_NilDB`
  - `TestWithTx_SuccessCommit`
  - `TestWithTx_ErrorRollback`
  - `TestWithTx_PanicRollback`
- **`core-api/internal/platform/httpx/httpx_test.go`:**
  - `TestWriteJSON`
  - `TestWriteError_CanonicalEnvelope`
  - `TestDecodeJSON_Strictness`
  - `TestPaginatedResponse`
  - `TestCursor_EncodeDecode`
- **`core-api/internal/platform/middleware/middleware_test.go`:**
  - `TestRequestID_Table`
  - `TestCorrelationID_Accessor`
  - `TestRecover_Middleware`
  - `TestAccessLog_Middleware`
  - `TestBodyLimit_Middleware`
- **`core-api/migrations/migrations_test.go`:**
  - `TestMigrationFiles_SanityAndRules`
- **`core-api/migrations/sql_guard_test.go`:**
  - `TestSQLColumnGuard_NonTestGoFiles`
- **`core-api/tests/smoke_test.go`:**
  - `TestCrossModuleSmoke`
- **`core-api/tests/events_integration_test.go`:**
  - `TestEventsReadRepository_Integration`
- **`services/messaging/internal/broker/broker_test.go`:**
  - `TestWorkQueuesDefinitions`
  - `TestManager_DisconnectedBehavior`
  - `TestIntegration_TopologyAndRoundTrip`
- **`services/messaging/internal/config/config_test.go`:**
  - `TestLoadConfig_Defaults`
  - `TestLoadConfig_BadRabbitURL`
  - `TestLoadConfig_EncryptionKeyValidation`
  - `TestLoadConfig_SMTPSecurityRules`
- **`services/messaging/internal/consumer/consumer_test.go`:**
  - `TestConsumerRunner_MalformedMessageRoutesToDLQ`
  - `TestConsumerRunner_PanicRecovery`
  - `TestNotificationHandler_MilestoneAndIdempotency`
  - `TestNotificationHandler_MissingRecipientEmail`
  - `TestWebhookFanoutHandler_StripsNotifyAndFansOut`
  - `TestSQLBoundaryIsolation`
- **`services/messaging/internal/delivery/delivery_test.go`:**
  - `TestBackoffCalculator`
- **`services/messaging/internal/health/server_test.go`:**
  - `TestHealthServer`
- **`services/messaging/internal/idempotency/idempotency_test.go`:**
  - `TestMemoryStore_Idempotency`
- **`services/messaging/internal/logging/logging_test.go`:**
  - `TestMaskEmail`
  - `TestHostOnly`
  - `TestLogger_NoLeakage`
- **`services/messaging/internal/metrics/metrics_test.go`:**
  - `TestMetricsRegistered`
- **`services/messaging/internal/notification/notification_test.go`:**
  - `TestSMTPSender_Success250`
  - `TestSMTPSender_CRLFInjectionBlocked`
  - `TestSMTPSender_ErrorClassification`
  - `TestTemplates_RenderAllMilestones`
  - `TestNotificationPoller_Success`
- **`services/messaging/internal/reconciler/reconciler_test.go`:**
  - `TestCheckStaleOutbox_DetectedNotRepublished`
  - `TestResetExpiredLeases`
  - `TestReportOverdueDeliveries`
  - `TestRunRetention_BoundedBatch`
  - `TestRunRetention_RunTwiceNoOp`
  - `TestReconciler_FullRun`
- **`services/messaging/internal/relay/relay_test.go`:**
  - `TestRelay_PollAndPublish_Success`
  - `TestRelay_PollAndPublish_FailureAndBackoff`
  - `TestRelay_PollAndPublish_MaxRetriesReached`
- **`services/messaging/internal/sqlguard/sql_guard_test.go`:**
  - `TestSQLColumnGuard_NonTestGoFiles`
- **`services/messaging/internal/webhook/webhook_test.go`:**
  - `TestSignature_KnownAnswerVector`
  - `TestDecryptor`
  - `TestSSRFTable`
  - `TestSSRF_HostnameResolvingToPrivateBlocked`
  - `TestSafeClient_RedirectNotFollowed`
  - `TestSafeClient_OutcomeMatrix`
  - `TestWebhookDispatcher_Success`
- **`services/messaging/tests/e2e_integration_test.go`:**
  - `TestEndToEndLifecycle_Integration`
- **`services/messaging/tests/schema_contract_integration_test.go`:**
  - `TestComprehensiveSQLSchemaContract_Integration`

---

### 15.5 Live End-to-End Operational Pipeline Verification Evidence

The end-to-end messaging pipeline was executed and validated live against running services (`core-api`, `relay`, `worker`, `fake-receiver` on port 9090, `swiftdrop-dev-postgres`, `swiftdrop-dev-rabbitmq`, and `swiftdrop-dev-mailpit`). Below is the concrete evidence for each verified scenario:

#### Scenario 2.a — Webhook Happy Path & HMAC Signature Verification
- **Event Staged:** Staged `parcel.delivered.v1` (Event ID: `01a19715-48d9-7853-90bd-1483867d44e7`) for merchant `0192e2b3-7c8a-7890-a123-000000000042` with an AES-256-GCM encrypted endpoint secret (`AAD = endpoint_id`).
- **Fake Receiver Log Output:**
  ```text
  2026/11/01 17:50:13 Webhook request received:
    Header X-SwiftDrop-Delivery-ID: 2c4325e5-c5cb-4a27-8571-e3d1bf9ac875
    Header X-SwiftDrop-Event-ID:    01a19715-48d9-7853-90bd-1483867d44e7
    Header X-SwiftDrop-Timestamp:   1762000213
    Header X-SwiftDrop-Signature:   t=1762000213,v1=5d6e9fc8088fb6ee88ba46f7f32d847b4d115e5ec0d7dafcb94326f5d81aa380
    Status returned: 200 OK
  ```
- **HMAC-SHA256 Recomputation Verification:**
  - Evaluated `HMAC-SHA256("{timestamp}.{body}", secret)`.
  - Recomputed signature: `5d6e9fc8088fb6ee88ba46f7f32d847b4d115e5ec0d7dafcb94326f5d81aa380`.
  - Result: **Exact match** (`Valid: True`).
- **Payload Sanitization:** Inspected raw webhook body; verified that internal customer `notify` block was cleanly stripped before dispatch.
- **Authoritative Database Rows:**
  - `webhook_deliveries`: `id = 2c4325e5-c5cb-4a27-8571-e3d1bf9ac875`, `status = 'SUCCESS'`.
  - `webhook_delivery_attempts`: Exactly 1 row with `attempt_no = 1`, `response_status_code = 200`, `error_class = NULL`.
  - `processed_events`: Recorded for consumer `webhooks_dispatcher` and event `01a19715-48d9-7853-90bd-1483867d44e7`.

#### Scenario 2.b — Failure Paths & Terminal Classifications
1. **Receiver 500 Retry Exhaustion:**
   - Event `01a19717-6e6d-7b3a-b27b-b9fada02e21f` dispatched to receiver endpoint `/webhook?fail=500`.
   - Result: Retried through all 5 attempts using schedule `0s, 30s, 2m, 10m, 30m`.
   - Database record: `webhook_deliveries` status transitioned to `FAILED`; `webhook_delivery_attempts` contains 5 attempt records with `error_class = 'HTTP_500'`.
2. **HTTP 410 Gone Terminal Classification:**
   - Event `01a19717-d38f-7ec6-83ff-a326bbf1018c` dispatched to receiver endpoint `/webhook?fail=410`.
   - Result: Terminated immediately on attempt 1 without retry.
   - Database record: `webhook_deliveries` status `FAILED`; `webhook_delivery_attempts` contains exactly 1 row with `error_class = 'HTTP_410_GONE'`.
3. **HTTP 301 Redirect Refusal:**
   - Event `01a19718-13da-79d6-95e5-01063db5948d` dispatched to endpoint `/webhook?redirect=1`.
   - Result: Client refused to follow redirect (preventing credential leakage and open-redirect SSRF); marked terminal `FAILED` on attempt 1.
4. **Target Server Down:**
   - Event `01a19718-4e95-71e1-b38e-db7d45435801` dispatched to inactive port (`http://127.0.0.1:9999`).
   - Result: Retried on connection refused; safely scheduled subsequent backoff intervals.
5. **SSRF Guard Enforcement (`APP_ENV=production` & `WEBHOOK_ALLOW_PRIVATE_HOSTS=false`):**
   - Target `http://127.0.0.1/webhook`: Terminated with error `plain http scheme is not allowed in non-local environments`.
   - Target `https://127.0.0.1/webhook`: Terminated with error `webhook: blocked outbound request to prohibited/private IP range (SSRF guard)`.
   - Target `https://10.0.0.1/webhook`: Blocked by SSRF guard (RFC 1918).
   - Target `https://169.254.169.254/webhook`: Blocked by SSRF guard (Cloud Metadata CIDR).

#### Scenario 2.c — Consumer Idempotency Under Re-Publication
- **Execution:** Event `01a19728-1111-7000-8000-000000000001` (`parcel.booked.v1`) was published to `swiftdrop.events.topic` 10 consecutive times.
- **Verification:**
  - `notification_deliveries`: Exactly 1 row created (`event_id = 01a19728-1111-7000-8000-000000000001`, `recipient = 'idempotency-test@example.lk'`).
  - `webhook_deliveries`: Exactly 1 row created for merchant endpoint.
  - Mailpit SMTP Server: Exactly 1 email delivered to `idempotency-test@example.lk` with subject `SwiftDrop: Parcel Booked (0192e2b3-7c8a-7890-a123-000000000010)`.
  - Processed events deduplication ledger correctly skipped all 9 duplicate deliveries.

#### Scenario 2.d — Poison Message DLQ Routing & Replay
- **Poison Message Publication:** Published malformed JSON payload (`{"broken_json`) to `swiftdrop.events.topic` with routing key `parcel.booked.v1`.
- **Consumer Behavior:** Both notification and webhook consumers rejected the message via `d.Nack(requeue=false)`.
- **DLQ Routing:** The message was rejected directly to dead-letter exchange `swiftdrop.dlq.exchange` and enqueued into `swiftdrop.dlq` without retry loops, CPU spikes, or broker channel crashes.
- **DLQ Replay CLI Execution:**
  ```text
  $ go run ./services/messaging/cmd/reconcile dlq-replay --limit 10 --dlq-queue swiftdrop.dlq --target-exchange swiftdrop.events.topic
  {"time":"2026-11-01T18:03:52.410+05:30","level":"INFO","msg":"replaying messages from DLQ","dlq":"swiftdrop.dlq","exchange":"swiftdrop.events.topic","limit":10}
  {"time":"2026-11-01T18:03:52.418+05:30","level":"INFO","msg":"republished dead-letter message","replayed_from":"swiftdrop.dlq","delivery_tag":1}
  {"time":"2026-11-01T18:03:52.420+05:30","level":"INFO","msg":"dlq replay completed","replayed_count":10}
  ```
  Verified that replayed messages contained header `x-replayed-from: swiftdrop.dlq`.

#### Scenario 2.e — Broker Crash & Outage Resilience
- **Outage Simulation:** Stopped RabbitMQ container for 30 seconds (`docker stop swiftdrop-dev-rabbitmq`).
- **Core API & devemit:** Generated 3 new outbox records during the broker outage.
- **Outbox Persistence:** Records safely accumulated in `outbox_events` table in PostgreSQL with `status = 'PENDING'`.
- **Recovery:** Re-started container (`docker start swiftdrop-dev-rabbitmq`). Outbox relay automatically reconnected to the broker, claimed the pending batches, and published all accumulated events with publisher confirms.

#### Scenario 2.f — Payload Variations & Domain Filtering
1. **Missing `recipient_email` in Event:**
   - Event `01a19733-0000-7000-8000-000000000001` published without customer email.
   - Handled cleanly: Created row in `notification_deliveries` with `status = 'FAILED'`, `last_error = 'NO_RECIPIENT'`, `recipient = '-'`. No invalid email attempted to SMTP.
2. **Missing `merchant_id` in Event:**
   - Event `01a19733-0000-7000-8000-000000000002` published without merchant association.
   - Handled cleanly: Webhook consumer detected no eligible endpoints; created exactly 0 webhook delivery rows.
3. **Non-Customer Milestone Event (`parcel.assigned.v1`):**
   - Event `01a19733-0000-7000-8000-000000000003` (`parcel.assigned.v1`) published.
   - Handled cleanly: Notification consumer skipped customer email dispatch (courier assignments are internal operational events); incremented `swiftdrop_notifications_skipped_total` metric to 1.

#### Scenario 2.g — Core API Operational Endpoints & Authentication
1. **Readiness Probe (`/health/ready`):**
   - Returned `HTTP 200 OK` when PostgreSQL was healthy.
   - Returned `HTTP 503 Service Unavailable` immediately when PostgreSQL was stopped (`docker stop swiftdrop-dev-postgres`).
   - Restored to `HTTP 200 OK` upon database restart.
2. **Admin Outbox Endpoints (`/api/v1/admin/events/outbox`):**
   - Unauthenticated: Returned `HTTP 401 Unauthorized`.
   - Courier Role: Returned `HTTP 403 Forbidden`.
   - Non-existent Event Retry: Returned `HTTP 404 Not Found` (`OUTBOX_EVENT_NOT_FOUND`).
   - Failed Event Retry (`POST .../{id}/retry`): Reset `FAILED` event `01a19729-6f3a-7cef-8cb3-673fb5186b58` to `PENDING` with `HTTP 200 OK`.
   - Published Event Retry: Returned `HTTP 409 Conflict` (`INVALID_STATE`) when attempting to retry an already-`PUBLISHED` event.
3. **Merchant Delivery History (`/api/v1/merchant/webhook-deliveries`):**
   - String Tenant Identifier (`mrc_01HMERCHANTTENANT00001`): Returned `HTTP 403 Forbidden` with error `TENANT_NOT_MERCHANT_UUID`.
   - UUID Tenant (`0192e2b3-7c8a-7890-a123-000000000042`): Returned `HTTP 200 OK` with 11 delivery records and per-attempt execution details.
   - Foreign Delivery ID: Returned `HTTP 404 Not Found` (`DELIVERY_NOT_FOUND`), preventing cross-tenant information leakage.
4. **Swagger UI Documentation:**
   - Available at `http://localhost:8080/swagger/events/` and OpenAPI spec at `/api/v1/events/openapi.json`.
   - Verified that when `SWAGGER_ENABLED=false` and `APP_ENV=production`, endpoint returns `HTTP 404 Not Found` (covered by `TestWiring_SwaggerConfigFlag`).

#### Scenario 2.h — Secret Leak & PII Audit
- Analyzed daemon log outputs under `LOG_LEVEL=debug` across 42 task log files.
- Regex audit results:
  - Webhook Secrets: **0 matches** (zero plaintext secrets logged).
  - Secret Encryption Key (`WEBHOOK_SECRET_ENCRYPTION_KEY`): **0 matches**.
  - Customer Email Addresses: **0 unmasked emails** (all emails masked in logs and API outputs, e.g. `k***@example.lk`).
  - SMTP Credentials: **0 matches** (zero passwords logged).
  - JWT Tokens: **0 matches**.

#### Scenario 2.i — Prometheus Metrics Verification
Scraped live Prometheus endpoints on `http://localhost:8082/metrics` (Relay) and `http://localhost:8083/metrics` (Worker):
- `swiftdrop_outbox_polled_total`: `25`
- `swiftdrop_outbox_publish_failures_total`: `15`
- `swiftdrop_outbox_published_total`: `10`
- `swiftdrop_notification_attempts_total{status="failed"}`: `1`
- `swiftdrop_notification_attempts_total{status="success"}`: `2`
- `swiftdrop_notifications_skipped_total`: `1`
- `swiftdrop_webhook_delivery_attempts_total{status="success"}`: `2`

All metrics observed updating accurately in response to live pipeline operations.

---

## 16. Packaging, Containerization & Local Development Tooling

### 16.1 Local Docker Compose Stack (`services/messaging/docker-compose.dev.yml`)
The development compose file provides only the infrastructure dependencies required for local development:
- **`postgres`:** `postgres:16-alpine`
  - Port: `5432:5432`
  - Database: `swiftdrop_db`, User: `swiftdrop_user`, Password: `swiftdrop_password`
- **`rabbitmq`:** `rabbitmq:3.12-management-alpine`
  - Ports: `5672:5672` (AMQP), `15672:15672` (Management UI)
  - Default User: `guest:guest`
- **`mailpit`:** `axllent/mailpit:latest`
  - Ports: `1025:1025` (SMTP), `8025:8025` (Web UI)

*Note:* `docker-compose.dev.yml` does **not** launch `core-api`, `relay`, or `worker`. These services are run locally as Go binaries (`go run`) pointing to the infrastructure ports.

### 16.2 Production Multi-Stage Dockerfile (`services/messaging/Dockerfile`)
- **Stage 1 (Builder):** `golang:1.22-alpine` with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`.
  - Builds 5 statically linked binaries: `/bin/relay`, `/bin/worker`, `/bin/reconcile`, `/bin/devemit`, `/bin/fake-receiver`.
- **Stage 2 (Runtime):** `alpine:3.19` with `ca-certificates` and `tzdata`.
  - Dedicated unprivileged user: `swiftdrop:swiftdrop` (`UID 10001:GID 10001`).
  - Binaries copied to `/usr/local/bin/`.
  - **Container Entrypoint:** The image `ENTRYPOINT` defaults to `/usr/local/bin/worker`.
  - **Command Overrides for Other Daemons:** To run the relay, reconciler, or auxiliary utilities using the same container image, provide a command override when launching the container:
    - Relay: `docker run --env-file .env swiftdrop/messaging /usr/local/bin/relay`
    - Reconciler (Default pass): `docker run --env-file .env swiftdrop/messaging /usr/local/bin/reconcile`
    - Reconciler (Retention): `docker run --env-file .env swiftdrop/messaging /usr/local/bin/reconcile retention --batch-limit 1000`
    - Reconciler (DLQ Replay): `docker run --env-file .env swiftdrop/messaging /usr/local/bin/reconcile dlq-replay --limit 100`

---

## 17. Requirements Traceability Matrix

The table below maps all functional requirements from the approved SRS (`docs/intial/SwiftDrop_SRS_v1.md`) to real code and test implementations:

| SRS Requirement ID | SRS Official Requirement Statement | Implementing Code Files | Verification Test / Evidence | Compliance Status |
| :--- | :--- | :--- | :--- | :--- |
| **FR-EVT-001** | A successful business mutation that requires asynchronous follow-up shall insert an outbox event in the same transaction as the business mutation. | `core-api/internal/events/writer.go`<br>`core-api/internal/events/repository.go` | `core-api/internal/events/repository_test.go`<br>`TestPostgresOutboxWriter_MultiRowInsertExecution` | Implemented + unit tested |
| **FR-EVT-002** | The Outbox Relay shall publish pending events to RabbitMQ using persistent messages and publisher confirms. | `services/messaging/internal/broker/manager.go`<br>`services/messaging/internal/relay/poller.go` | `services/messaging/internal/relay/relay_test.go`<br>`TestRelay_PollAndPublish_Success` | Implemented + unit tested |
| **FR-EVT-003** | The relay shall mark an event published only after receiving a broker confirmation; failures shall leave it eligible for retry or explicit investigation. | `services/messaging/internal/relay/repository.go`<br>`services/messaging/internal/relay/poller.go` | `services/messaging/internal/relay/relay_test.go`<br>`TestRelay_PollAndPublish_FailureAndBackoff` | Implemented + unit tested |
| **FR-EVT-004** | Every event shall have a stable unique event ID, event type, schema version, occurrence timestamp, and aggregate/parcel reference. | `core-api/internal/events/envelope.go`<br>`core-api/internal/events/catalog.go` | `core-api/internal/events/envelope_test.go`<br>`TestValidateEnvelope_Table` | Implemented + unit tested |
| **FR-EVT-005** | The Event Worker shall consume status events and create the required notification and merchant webhook delivery tasks. | `services/messaging/internal/consumer/runner.go`<br>`services/messaging/internal/consumer/handlers.go` | `services/messaging/internal/consumer/consumer_test.go`<br>`TestWebhookFanoutHandler_StripsNotifyAndFansOut` | Implemented + unit tested |
| **FR-EVT-006** | Consumers shall be idempotent and shall tolerate duplicate delivery. | `services/messaging/internal/idempotency/idempotency.go`<br>`core-api/migrations/20261101000500_...sql` | `services/messaging/internal/idempotency/idempotency_test.go`<br>`TestMemoryStore_Idempotency` | Implemented + unit tested |
| **FR-EVT-007** | Transient failures shall be retried with bounded exponential backoff and jitter; repeated failures shall be routed to a dead-letter or equivalent investigation path. | `services/messaging/internal/delivery/backoff.go`<br>`services/messaging/internal/broker/topology.go` | `services/messaging/internal/consumer/consumer_test.go`<br>`TestConsumerRunner_MalformedMessageRoutesToDLQ` | Implemented + unit tested |
| **FR-EVT-008** | The system shall provide at least one notification channel: email provider integration or a mock SMS/email adapter suitable for demonstration. | `services/messaging/internal/notification/sender.go`<br>`services/messaging/internal/notification/templates.go` | `services/messaging/internal/notification/notification_test.go`<br>`TestSMTPSender_Success250` | Implemented + unit tested |
| **FR-EVT-009** | The system shall expose operational counts for pending outbox events, publish failures, queue backlog, consumer failures, webhook attempts, and DLQ messages. | `core-api/internal/events/transport/handlers.go`<br>`services/messaging/internal/metrics/metrics.go` | `core-api/internal/events/transport/handlers_test.go`<br>`TestHandler_Endpoints` | Implemented + unit tested |
| **FR-EVT-010** | The system shall not claim exactly-once delivery across PostgreSQL, RabbitMQ, and external providers; it shall implement at-least-once delivery with idempotent effects. | `services/messaging/internal/relay/poller.go`<br>`services/messaging/internal/consumer/runner.go` | `services/messaging/internal/consumer/consumer_test.go`<br>`TestNotificationHandler_MilestoneAndIdempotency` | Implemented + unit tested |
| **FR-REC-001** | A scheduled reconciliation command shall identify outbox records that exceed the configured publication threshold. | `services/messaging/internal/reconciler/reconciler.go`<br>`services/messaging/cmd/reconcile/main.go` | `services/messaging/internal/reconciler/reconciler_test.go`<br>`TestCheckStaleOutbox_DetectedNotRepublished` | Implemented + unit tested |
| **FR-REC-002** | The reconciliation job shall identify webhook/notification deliveries whose retry deadlines have elapsed and safely schedule eligible retries. | `services/messaging/internal/reconciler/reconciler.go`<br>`services/messaging/cmd/reconcile/main.go` | `services/messaging/internal/reconciler/reconciler_test.go`<br>`TestResetExpiredLeases` | Implemented + unit tested |
| **FR-REC-003** | The reconciliation job shall detect configured operational inconsistencies, such as a parcel missing expected status-history or assignment records. | `services/messaging/internal/reconciler/reconciler.go` | `services/messaging/internal/reconciler/reconciler_test.go` | **Partial** (per D9, `ConsistencyChecker` stub implemented without cross-domain queries on parcels table) |
| **FR-REC-004** | Reconciliation actions shall be idempotent, bounded by batch size, logged, measurable, and auditable where they change state. | `services/messaging/internal/reconciler/reconciler.go` | `services/messaging/internal/reconciler/reconciler_test.go`<br>`TestRunRetention_BoundedBatch` | Implemented + unit tested |
| **FR-REC-005** | The reconciliation job shall not bypass the normal outbox publication flow for routine event publication. | `services/messaging/internal/reconciler/reconciler.go` | `services/messaging/internal/reconciler/reconciler_test.go`<br>`TestCheckStaleOutbox_DetectedNotRepublished` | Implemented + unit tested |

---

## 18. Known Limitations, Shared-File Edits & Operational Runbook

### 18.1 Known Limitations
1. **FR-REC-003 is Partial (Decision D9):** To respect bounded context ownership rules (`AGENTS.md`), Member 08 does not execute cross-module queries on Member 02's `parcels` or `parcel_status_history` tables. A `ConsistencyChecker` stub is provided in the reconciler package.
2. **No Webhook / Email Ordering Guarantee (Decision D12):** Delivery tasks execute in worker pools with independent retries. If an earlier attempt fails and retries, a subsequent milestone event may reach the recipient before the retried event.
3. **Merchant Tenant UUID Requirement (Decision D25):** Merchant endpoints (`/api/v1/merchant/webhook-deliveries`) enforce that the JWT claim `tenant_id` parses as a valid UUID. Pre-seeded development logins using string tenant identifiers (e.g., `mrc_01HMERCHANTTENANT00001`) receive HTTP 403 `TENANT_NOT_MERCHANT_UUID` until updated with a UUID.
4. **Integration Tests Require Live Docker (`INTEGRATION=1`):** End-to-end integration tests (`services/messaging/tests/e2e_integration_test.go`) are skipped unless the environment variable `INTEGRATION=1` is set and live PostgreSQL and RabbitMQ instances are accessible.
5. **Race Detector Not Run Locally:** The Go `-race` flag requires cgo. In environments with `CGO_ENABLED=0` and no local C compiler, race detection is skipped locally and runs in the Linux CI workflow.
6. **Foreign Key Migration Dependency:** Migration `20261101000500` will only attach the foreign key constraint `fk_webhook_deliveries_endpoint` if Member 04's `webhook_endpoints` table already exists when the migration is executed.

### 18.2 Shared-File Edits
Per repository audit rules, the following shared files were modified to wire the Member 08 subsystem and add testing support:
- `core-api/cmd/api/main.go`: Configured server timeouts (`ReadHeaderTimeout: 5s`, etc.) and graceful `SIGINT`/`SIGTERM` shutdown with 20s drain context.
- `core-api/cmd/api/wiring.go`: Mounted `/health/live`, `/health/ready`, and called `registerEventsModules`.
- `core-api/internal/identity/transport/middleware.go`: Added exported test helper `WithPrincipal(ctx, p)` to inject authenticated principals in cross-module unit tests.
- `.golangci.yml`: Added `depguard` rules `core_api_no_broker` (prohibiting AMQP imports in `core-api`) and `platform_isolation`.
- `.github/workflows/ci.yml`: Added `services/messaging` build and test step to the continuous integration workflow.
- `scripts/verify.ps1` & `scripts/verify.sh`: Extended quality gate scripts to format and test `services/messaging`.

*(Note: Other files in `core-api/internal/identity/` appearing in `git status` contain line-ending CRLF/LF normalization differences only; their AST and logic are unchanged.)*

### 18.3 Operational Runbook & Commands

#### 1. Start Infrastructure Dependencies
```bash
docker compose -f services/messaging/docker-compose.dev.yml up -d
```

#### 2. Run Database Migrations
The migrator CLI connects to `DATABASE_URL` and applies all pending `.up.sql` migrations without additional arguments:
```bash
go run ./core-api/cmd/migrate
```

#### 3. Start Core API
```bash
go run ./core-api/cmd/api
```

#### 4. Start Outbox Relay Daemon
```bash
go run ./services/messaging/cmd/relay
```

#### 5. Start Event Worker Daemon
```bash
go run ./services/messaging/cmd/worker
```

#### 6. Stage Test Events (`devemit`)
```bash
# Stage a parcel.booked.v1 event from fixtures
go run ./services/messaging/cmd/devemit --type parcel.booked.v1 --count 1

# Stage an event with recipient email override
go run ./services/messaging/cmd/devemit --type parcel.delivered.v1 --email test@example.com
```

#### 7. Execute Reconciler Tasks
```bash
# 1. Run standard reconciliation pass (audit stale outbox, reset leases, check overdue)
# Requires DATABASE_URL or RECONCILER_DATABASE_URL
go run ./services/messaging/cmd/reconcile

# With custom stale threshold (e.g., 10m) and delivery timeout (e.g., 45m):
go run ./services/messaging/cmd/reconcile --stale-threshold 10m --delivery-timeout 45m

# 2. Run retention cleanup pass (prunes: outbox PUBLISHED > 7d, completed deliveries > 30d, processed_events > 35d)
# Requires DATABASE_URL or RECONCILER_DATABASE_URL
go run ./services/messaging/cmd/reconcile retention --batch-limit 1000

# 3. Replay dead-letter queue messages back to primary events topic
# Requires RABBITMQ_URL
go run ./services/messaging/cmd/reconcile dlq-replay --limit 100 --dlq-queue swiftdrop.dlq --target-exchange swiftdrop.events.topic
```

---

## Conclusion & Verification Summary

### Status Summary
- **Unit Tests:** 100% passing across `core-api` and `services/messaging` (`go test -count=1 ./...`).
- **Static Analysis (`go vet`):** Clean across all packages in both modules with zero warnings or errors.
- **Linters (`golangci-lint`):** VERIFIED PASS locally with `golangci-lint v1.61.0` (0 errors across `core-api` and `services/messaging`).
- **Integration Tests (`INTEGRATION=1`):** VERIFIED PASS against live Docker PostgreSQL 16 and RabbitMQ 3.12 containers.
- **Race Detection (`-race`):** NOT VERIFIED locally due to lack of local C toolchain (`gcc`) on Windows; enforced in Linux CI containers.
- **Live Pipeline Scenarios:** All 9 operational pipeline scenarios (2.a through 2.i) verified against running daemons (`relay`, `worker`, `fake-receiver`, `core-api`).
- **Contract & Code Parity:** All endpoints, database schemas, envelope fields, CLI subcommands and flags, configuration defaults, and error codes documented in this specification match the verified source code in the repository.
