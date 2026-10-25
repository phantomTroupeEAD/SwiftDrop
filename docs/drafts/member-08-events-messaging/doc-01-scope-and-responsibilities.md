# Document 01 — Module Scope & Responsibility Specification

> **Module:** Event-Driven Backend & Messaging (`internal/events`, `services/messaging`)  
> **Owning Member:** Member 08 — Event-Driven Backend & Integrations  
> **Status:** Draft Baseline Specification for Review & Implementation  
> **Target Production Locations:**  
> - `core-api/internal/events/` (Transactional Outbox Writer, Event Envelopes, Schemas)  
> - `services/messaging/` (Outbox Relay Daemon, RabbitMQ Topology, Event Worker, Webhook Dispatcher, Real SMTP Notifier, Reconciliation Engine)  
> **Relevant Contracts:** [event-contracts.md](../../contracts/event-contracts.md), [webhook-security.md](../../contracts/webhook-security.md), [database-schema.md](../../architecture/database-schema.md), [domain-boundaries.md](../../architecture/domain-boundaries.md)

---

## 1. Purpose & Business Problem

### 1.1 The Business Problem
In a modern logistics and courier management platform like **SwiftDrop**, business actions occur synchronously (e.g., booking a parcel, dispatching a courier, confirming a delivery attempt). However, secondary side effects—such as customer milestone notifications, real-time merchant webhook callbacks, and tracking projections—must never degrade the performance, latency, or availability of the primary transaction path.

Furthermore, attempting direct asynchronous communication within synchronous HTTP handlers creates severe failure modes:
1. **The Dual-Write Problem:** Updating the database and immediately making a network call to a message broker (RabbitMQ) or external third-party API (SMTP server, webhook URL) causes data loss if the broker or network fails after the database commits, or leaves orphaned broker messages if the database transaction aborts.
2. **Blast Radius & Cascade Outages:** If an external merchant webhook endpoint or email gateway is slow or down, synchronous HTTP request threads become exhausted, locking database connections and cascading into complete Core API downtime.
3. **Security Vulnerabilities (SSRF & Replay):** Delivering outbound HTTP notifications to arbitrary merchant endpoints exposes internal cloud and cluster infrastructure to Server-Side Request Forgery (SSRF) and DNS rebinding attacks.

### 1.2 Module Purpose
The **Event-Driven Backend & Integrations** module eliminates dual-write anomalies, isolates failure domains, and guarantees reliable, at-least-once message delivery. 

It accomplishes this by implementing:
- A transactional outbox integration within the Core API database boundary.
- An autonomous **Outbox Relay Daemon** that reliably transfers committed events to RabbitMQ with publisher confirmation guarantees.
- An **Event Worker Service** executing idempotent consumption, cryptographic merchant webhook delivery with strict SSRF defenses, and **direct real SMTP email delivery** for transactional notifications from the very beginning (no mock/fake sinks).
- A **Reconciliation CLI & CronJob** that detects, alerts, and safely heals stuck publications without bypassing outbox integrity.

---

## 2. Functional Responsibilities & Explicit Exclusions

### 2.1 What Member 08 Owns (Functional Responsibilities)
1. **Canonical Event Envelope & Serialization:**
   - Define and maintain the universal `EventEnvelope[T]` Go struct and JSON serialization contracts (`event_id`, `event_type`, `aggregate_type`, `aggregate_id`, `version`, `timestamp`, `correlation_id`, `producer`, `payload`).
   - Maintain the authoritative event catalog and schema versioning (`parcel.booked.v1`, `parcel.assigned.v1`, `parcel.picked_up.v1`, `parcel.delivered.v1`, `parcel.delivery_failed.v1`, etc.).
2. **Core API Transactional Outbox Writer:**
   - Provide the `OutboxWriter` interface used across all Core API application use cases.
   - Implement the PostgreSQL outbox repository that appends records to `outbox_events` within the caller's active database transaction (`*sql.Tx`).
3. **Dedicated Outbox Relay Daemon (`services/messaging/cmd/relay`):**
   - Implement an autonomous background poller utilizing `SELECT ... FOR UPDATE SKIP LOCKED`.
   - Establish reliable RabbitMQ AMQP connections with persistent message delivery (`DeliveryMode = 2`).
   - Enforce **RabbitMQ Publisher Confirms**; update `outbox_events.published_at` only after explicit broker ACK.
   - Run strictly as a singleton (`replicas: 1`) to eliminate database row contention and duplicate publishing.
4. **RabbitMQ Broker Topology & Management:**
   - Define durable topic exchange `swiftdrop.events.topic`.
   - Declare durable queues (`q.notifications.worker`, `q.webhooks.dispatcher`, `q.tracking.updates`) and topic routing bindings (`parcel.*.v1`).
   - Configure dead-letter exchange/queue `swiftdrop.dlq` for poison-pill isolation.
5. **Event Worker Service (`services/messaging/cmd/worker`):**
   - Consume messages with manual acknowledgements (`Ack`/`Nack`).
   - Implement mandatory consumer idempotency deduplication via the `processed_events` table.
   - Implement bounded exponential backoff with randomized jitter (up to 5 attempts) before escalating to the DLQ.
6. **Real SMTP Email Notification Adapter:**
   - **Full Real SMTP Delivery:** Direct production-ready SMTP email delivery adapter connecting over TLS/STARTTLS to a configured real SMTP server (e.g., Mailtrap, Postmark, SendGrid, Amazon SES, or custom SMTP relay) from day one. No fake, simulated, or demo sinks.
   - Render responsive HTML and plaintext transactional email templates for parcel milestones (Booking Confirmation, Out for Delivery, Delivery Success, Delivery Failure).
   - Track delivery outcomes, provider message IDs, timestamps, and errors in `notification_deliveries`.
7. **Merchant Webhook Delivery Pipeline & SSRF Protection:**
   - Query registered webhook endpoints from `webhook_endpoints` (configured by Member 04).
   - Sign payloads using HMAC-SHA256 (`X-SwiftDrop-Signature: t={timestamp},v1={hash}`) with timestamp anti-replay verification (300-second window).
   - Implement hardened outbound HTTP client (`SafeDialContext`) with single-resolution DNS and CIDR blocklist (prohibiting `127.0.0.0/8`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `169.254.0.0/16`, `::1`, `fe80::/10`, `fc00::/7`).
   - Disallow HTTP redirects, enforce strict 5.0-second socket timeouts, and truncate logged error responses to 500 characters.
   - Record delivery attempts in `webhook_deliveries`.
8. **Operational Reconciliation & Runbooks:**
   - Provide a standalone CLI and Kubernetes CronJob command (`services/messaging/cmd/reconcile`) to identify stale outbox events (`created_at < NOW() - 5m` with `status = 'PENDING'`) and overdue webhook retries.
   - Author operational playbooks for DLQ replay, stuck outbox draining, and merchant delivery audits.

### 2.2 Explicit Exclusions (What Member 08 Must NEVER Own)
To prevent architectural drift and preserve strict domain boundaries (per [domain-boundaries.md](../../architecture/domain-boundaries.md)):
- **NO Business Domain Invariant Evaluation:** Member 08 never decides whether a parcel *can* transition from `ASSIGNED` to `PICKED_UP`, nor calculates pricing fees. Business invariants belong strictly to Member 02 (`internal/parcels`, `internal/pricing`) and Member 03 (`internal/dispatch`, `internal/delivery`).
- **NO Direct Database State Mutation of Other Domains:** Member 08 never updates `parcels.current_status`, `courier_assignments`, or `users`.
- **NO In-Monolith Broker Imports:** Core API domain and application layers must NEVER import `github.com/rabbitmq/amqp091-go` or publish messages directly over the network.
- **NO Merchant Webhook Management APIs:** Creating, updating, rotating secrets, or deleting merchant webhook URLs is owned by Member 04 (`internal/merchants`). Member 08 only consumes and delivers to them.
- **NO Frontend UIs or Public Tracking Lookups:** Tracking query endpoints, caching, and Server-Sent Events (SSE) connections are owned by Member 05 (`services/tracking`). Customer/Merchant dashboards are owned by Member 06.
- **NO Infrastructure Provisioning:** Kubernetes manifests, Helm charts, and CI/CD deployment pipelines are authored and deployed by Member 09. Member 08 delivers container entrypoints, configuration specifications, and probe endpoints.

---

## 3. Upstream and Downstream Dependencies

```mermaid
flowchart LR
    subgraph Upstream ["Upstream Domain Producers"]
        M2["Member 02:<br/>Parcels & Pricing"]
        M3["Member 03:<br/>Dispatch & Delivery"]
        M4["Member 04:<br/>Merchant Context"]
    end

    subgraph M8_Core ["Member 08: Core API Library"]
        OBW["OutboxWriter Interface<br/>& Event Envelopes"]
    end

    subgraph Storage ["PostgreSQL Authoritative DB"]
        OBT[("outbox_events")]
        DLV[("notification_deliveries<br/>webhook_deliveries")]
        IDM[("processed_events")]
        WHE[("webhook_endpoints<br/>(Owned by M4)")]
    end

    subgraph M8_Daemons ["Member 08: Messaging Daemons"]
        REL["Outbox Relay Daemon<br/>(cmd/relay)"]
        WKR["Event Worker Service<br/>(cmd/worker)"]
        REC["Reconciliation Engine<br/>(cmd/reconcile)"]
    end

    subgraph Broker ["Message Broker"]
        RMQ{{"RabbitMQ Topic Exchange<br/>swiftdrop.events.topic"}}
        DLQ{{"Dead Letter Queue<br/>swiftdrop.dlq"}}
    end

    subgraph Downstream ["Downstream Consumers & External Endpoints"]
        M5["Member 05:<br/>Tracking API (SSE)"]
        SMTP["Real SMTP Server<br/>(TLS / Auth Port 587)"]
        WH["Merchant Webhook Endpoints<br/>(Public HTTPS)"]
    end

    M2 -->|"Uses OutboxWriter in *sql.Tx"| OBW
    M3 -->|"Uses OutboxWriter in *sql.Tx"| OBW
    M4 -->|"Uses OutboxWriter in *sql.Tx"| OBW
    OBW -->|"INSERT (Same Tx)"| OBT

    REL -->|"SELECT ... FOR UPDATE SKIP LOCKED"| OBT
    REL -->|"Publish with Publisher Confirms"| RMQ
    RMQ -->|"Confirm ACK"| REL
    REL -->|"UPDATE published_at"| OBT

    RMQ -->|"Consume q.tracking.updates"| M5
    RMQ -->|"Consume q.notifications.worker"| WKR
    RMQ -->|"Consume q.webhooks.dispatcher"| WKR

    WKR -->|"Check Idempotency"| IDM
    WKR -->|"Read Active Webhook URLs/Secrets"| WHE
    WKR -->|"Log Delivery Attempts"| DLV

    WKR -->|"Deliver Real Transactional Email"| SMTP
    WKR -->|"Deliver Signed HTTPS Webhook (SSRF Guard)"| WH
    WKR -->|"Route Poison Messages (>5 retries)"| DLQ

    REC -->|"Audit Stale Outbox / Overdue Retries"| OBT
    REC -->|"Audit Stuck Deliveries"| DLV
```

### 3.1 Upstream Dependencies (Inputs Required)
- **Domain Event Definitions (Members 02, 03, 04):** Event names, schema payloads, and triggering conditions for parcel booking, dispatch, delivery confirmation, failure, and merchant orders.
- **Shared DB Transaction Convention (`*sql.Tx`):** Modular Monolith use cases must pass the active database transaction into the `OutboxWriter`.
- **Merchant Webhook Configuration (Member 04):** The authoritative schema of `webhook_endpoints` (containing `id`, `merchant_id`, `url`, `secret_hash`, `subscribed_event_types`, `enabled`).
- **Cluster Broker & Secret Provisioning (Member 09):** RabbitMQ cluster connectivity parameters (`RABBITMQ_URL`) and Kubernetes secret references for SMTP credentials.

### 3.2 Downstream Dependencies (Outputs Provided)
- **Tracking API (Member 05):** Publishes guaranteed parcel milestone events to `swiftdrop.events.topic` using routing key `parcel.*.v1`, allowing Member 05 to update read models and push SSE streams to end users.
- **Real SMTP Mail Server:** Connects directly to external SMTP relays over TLS (e.g., Mailtrap, Postmark, AWS SES, SendGrid) to deliver notifications to recipients.
- **Merchant Systems:** Dispatches authenticated HTTPS webhooks with HMAC-SHA256 signatures and delivery audit trails.
- **DevOps & Operations (Member 09):** Exposes Prometheus metrics, health probes (`/livez`, `/readyz`), and the `reconcile` CLI command for Kubernetes CronJob scheduling.

---

## 4. Bounded Context, Domain Ownership & Authoritative Data

### 4.1 Bounded Context Boundary
This module operates across two architectural planes:
1. **In-Monolith Support Plane (`core-api/internal/events/`):** A shared architectural capability within the Core API providing the transactional outbox contract and generic event envelopes. It has zero external transport or broker dependencies.
2. **Asynchronous Daemon Plane (`services/messaging/`):** A dedicated, standalone Go service containing the Outbox Relay, the Event Worker consumers, and the Reconciliation job.

### 4.2 Authoritative Database Tables Owned by Member 08
In strict compliance with the **10-Member Capability Ownership Matrix** ([database-schema.md §2](../../architecture/database-schema.md)):

| Table Name | Owning Bounded Context | Primary Write Path | Purpose |
| :--- | :--- | :--- | :--- |
| `outbox_events` | Event Support | Use case transactions (producer) & Outbox Relay (publisher) | Authoritative transactional event log awaiting broker confirmation. |
| `processed_events` | Event Worker | Event Worker consumer goroutines | Authoritative consumer idempotency deduplication ledger. |
| `notification_deliveries` | Event Worker | Event Worker notification pipeline | Authoritative log of real SMTP email attempts, provider message IDs, and errors. |
| `webhook_deliveries` | Event Worker | Event Worker webhook dispatcher | Authoritative audit history of outbound HTTP webhook attempts, status codes, and latencies. |

### 4.3 Read-Only Table Access
- `webhook_endpoints`: Owned by Member 04 (`internal/merchants`). Member 08 reads active endpoint URLs and signing secrets to deliver webhooks. Member 08 **never** inserts or modifies rows in this table.

---

## 5. Invariants That This Module Must Protect

Every AI agent and developer touching this module must respect the following non-negotiable invariants:

1. **Atomic Outbox Invariant:** An event row in `outbox_events` MUST be inserted using the exact same PostgreSQL `*sql.Tx` as the domain entity state change. No event may ever be written outside an active business mutation.
2. **Zero Broker Dependency in Core API:** The Core API monolith must NEVER connect to RabbitMQ or import AMQP libraries. All broker networking is strictly isolated to `services/messaging`.
3. **Publisher Confirms Before Status Update:** The Outbox Relay must NEVER mark an outbox row as `status = 'PUBLISHED'` or set `published_at` until the RabbitMQ broker explicitly acknowledges receipt via publisher confirms.
4. **Strict Relay Singleton:** The Outbox Relay process must ALWAYS run as a strict singleton (`replicas: 1`). Horizontal scaling without distributed leader election is strictly prohibited to prevent database row-lock contention.
5. **Mandatory Consumer Idempotency:** Because RabbitMQ provides at-least-once delivery, every consumer MUST check and record `processed_events` before executing side effects. Duplicate events must be acknowledged and discarded without re-sending emails or re-calling webhooks.
6. **Non-Blocking Transactions:** No database transaction may ever be held open while waiting for external network calls (SMTP connections or HTTP merchant webhooks).
7. **Strict SSRF Immunity:** The outbound webhook client MUST resolve destination hostnames once, validate that the IP does NOT fall within blacklisted CIDRs (private, loopback, link-local, cloud metadata), and dial the validated IP directly without following HTTP redirects.
8. **Real SMTP Notifications:** Transactional milestone notifications must be dispatched through a real, configured SMTP mail server using TLS/STARTTLS authentication. No fake/mock sinks or console-only drops are permitted in the delivery pipeline.
9. **State Purity in Reconciliation:** The reconciliation engine must NEVER invent synthetic parcel statuses or bypass the outbox pipeline. It only un-stalls or reschedules genuine events.

---

## 6. What Other Modules Are Allowed to Request

### Permitted Interactions
- **Application Use Cases:** Call `OutboxWriter.Write(ctx, tx, events...)` to stage one or more domain events within an active transaction.
- **Module Consumers:** Import `EventEnvelope[T]` and the event catalog constants to construct strongly typed payloads.
- **Member 05 (Tracking API):** Subscribe to `swiftdrop.events.topic` queues in RabbitMQ to project real-time updates to SSE clients.
- **Member 04 (Merchants):** Query `webhook_deliveries` to display delivery history and HTTP response status codes on the merchant dashboard.
- **Member 09 (DevOps):** Execute `services/messaging/cmd/reconcile` on a scheduled Kubernetes `CronJob` and scrape `/metrics` and `/readyz` probes.

### Strictly Prohibited Requests
- Other modules may **NOT** call Member 08 to synchronously send an email or webhook during an HTTP request.
- Other modules may **NOT** insert directly into `notification_deliveries`, `webhook_deliveries`, or `processed_events`.
- Other modules may **NOT** publish raw messages directly to `swiftdrop.events.topic` bypassing the outbox relay.

---

## 7. Ownership Matrix: Files, Tables, Queues & Configuration

### 7.1 Repository File Ownership
```
SwiftDrop Repository
├── core-api/
│   └── internal/
│       └── events/                       # Monolith Outbox Support
│           ├── envelope.go               # EventEnvelope[T] generic contract
│           ├── writer.go                 # OutboxWriter interface definition
│           ├── repository.go             # PostgreSQL outbox implementation
│           ├── catalog.go                # Authoritative event types & version constants
│           └── README.md                 # Living documentation for internal/events
├── services/
│   └── messaging/                        # Standalone Messaging Service
│       ├── cmd/
│       │   ├── relay/main.go             # Outbox Relay entrypoint
│       │   ├── worker/main.go            # Event Worker entrypoint
│       │   └── reconcile/main.go         # Reconciliation CLI entrypoint
│       ├── internal/
│       │   ├── broker/                   # RabbitMQ connection, exchange, queues & confirms
│       │   ├── consumer/                 # Generic consumer runner, acks, and DLQ router
│       │   ├── idempotency/              # processed_events repository
│       │   ├── notifier/                 # Real SMTP email adapter, templates, & delivery tracking
│       │   ├── webhook/                  # SSRF-hardened HTTP client, HMAC signer, & backoff
│       │   └── relay/                    # Polling logic, locking, and batch publisher
│       ├── tests/                        # Integration and resilience test suites
│       └── README.md                     # Living documentation for messaging service
└── docs/
    └── drafts/
        └── member-08-events-messaging/   # Member 08 8-document specifications
```

### 7.2 Database Tables
- `outbox_events` (Full write/read ownership)
- `processed_events` (Full write/read ownership)
- `notification_deliveries` (Full write/read ownership)
- `webhook_deliveries` (Full write/read ownership)
- `webhook_endpoints` (Read-only consumer access; owned by Member 04)

### 7.3 RabbitMQ Topology
- **Exchange:** `swiftdrop.events.topic` (Topic, Durable)
- **Queues:**
  - `q.notifications.worker` (Durable, bound to `parcel.*.v1`)
  - `q.webhooks.dispatcher` (Durable, bound to `parcel.*.v1`)
  - `q.tracking.updates` (Durable, declared for Member 05)
  - `swiftdrop.dlq` (Durable dead-letter queue)

### 7.4 Configuration & Environment Variables
| Variable Name | Description | Default / Example Value | Secret? |
| :--- | :--- | :--- | :--- |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://swiftdrop:swiftdrop_dev@postgres:5432/swiftdrop_db?sslmode=disable` | Yes |
| `RABBITMQ_URL` | RabbitMQ AMQP connection URI | `amqp://guest:guest@rabbitmq:5672/` | Yes |
| `OUTBOX_POLL_INTERVAL_MS` | Polling frequency for pending outbox events | `500` | No |
| `OUTBOX_BATCH_SIZE` | Max rows processed per relay iteration | `50` | No |
| `OUTBOX_STALE_SECONDS` | Age before unconfirmed outbox events trigger warnings | `300` | No |
| `WORKER_CONCURRENCY` | Goroutine worker pool size per consumer pod | `10` | No |
| `WEBHOOK_HTTP_TIMEOUT_SECONDS`| Socket timeout for outbound merchant HTTP POSTs | `5` | No |
| `SMTP_HOST` | Real SMTP server hostname | `smtp.mailtrap.io` (or production provider) | No |
| `SMTP_PORT` | SMTP port (STARTTLS: 587, TLS: 465) | `587` | No |
| `SMTP_USERNAME` | SMTP authentication username | `smtp_user_123` | Yes |
| `SMTP_PASSWORD` | SMTP authentication password / API token | `smtp_secret_pass` | Yes |
| `SMTP_FROM_EMAIL` | Authoritative system sender address | `notifications@swiftdrop.io` | No |
| `SMTP_FROM_NAME` | Sender display name | `SwiftDrop Deliveries` | No |
| `SMTP_TLS_ENABLED` | Enforce TLS/STARTTLS handshake | `true` | No |

---

## 8. Assumptions, Unresolved Decisions & Out-of-Scope Work

### 8.1 Assumptions
1. **PostgreSQL Relational Guarantee:** PostgreSQL is configured with `READ COMMITTED` isolation level and supports `SKIP LOCKED` (PostgreSQL 12+; project baseline is 16+).
2. **RabbitMQ Topic Routing:** RabbitMQ 3.12+ is deployed with publisher confirms enabled and cluster storage persistence.
3. **Real SMTP Server Availability:** Valid SMTP credentials with STARTTLS on port 587 or TLS on port 465 are supplied via environment variables in all development, staging, and production environments.
4. **Merchant HTTPS Baseline:** In staging/production, merchant webhook URLs must use valid `https://` schemas. Plain `http://` is allowed strictly under `APP_ENV=local`.

### 8.2 Unresolved Decisions for Team Coordination
1. **Payload Hydration vs Self-Contained Events:** Currently, event payloads emitted by Member 02 and Member 03 contain core tracking and actor identifiers. If notifications require recipient emails not present in the payload, the Event Worker will perform a read-only lookup against `parcels` and `parcel_addresses`. The team will finalize whether all recipient data is embedded directly in the event payload to make consumer goroutines fully autonomous.
2. **KEDA Scaling Metric for Workers:** Member 09 has configured KEDA to scale `event-worker` when RabbitMQ backlog exceeds 10 messages. We must benchmark whether worker pods should scale on queue depth or message processing rate.

### 8.3 Out-of-Scope Work
- **SMS Gateway Integrations:** Real telecom SMS integrations (Twilio, Infobip) are deferred in favor of full real SMTP email delivery; the email adapter is the primary real notification channel.
- **Frontend Push Notifications:** Web push notifications (Service Workers / VAPID) are not owned by Member 08.
- **Merchant Webhook Self-Service UI:** The UI for merchants to input webhook URLs and view signing keys is owned by Member 06.

---

## 9. Responsibility Matrix & Cross-Module Boundaries

The following matrix formally defines boundaries between Member 08 and all other team capability owners:

| Capability / Concern | Member 08 (Events & Messaging) | Other Members | Clear Boundary Rule |
| :--- | :--- | :--- | :--- |
| **Parcel State Transition** | None (Consumes events) | Member 02 (`internal/parcels`) | Only Member 02 can mutate parcel status. Member 08 only records and relays the resulting event. |
| **Outbox Event Persistence** | Owns `OutboxWriter` & `outbox_events` table | Members 02, 03, 04 | Domain use cases instantiate and pass their `*sql.Tx` to Member 08's `OutboxWriter`. |
| **RabbitMQ Publishing** | Owns Relay daemon & broker topology | Zero access for other backend members | Only `services/messaging/cmd/relay` connects to RabbitMQ to publish messages. |
| **Real-Time Tracking Stream** | Publishes to `swiftdrop.events.topic` | Member 05 (`services/tracking`) | Member 08 delivers messages to `q.tracking.updates`; Member 05 reads the queue and serves browser SSE connections. |
| **Merchant Webhooks** | Dispatches HTTP requests, signs HMAC, SSRF defense, retries | Member 04 (`internal/merchants`) | Member 04 provides CRUD endpoints for webhook registration. Member 08 executes delivery attempts and logs outcomes. |
| **Customer Notifications** | Owns real SMTP email adapter, templates, & delivery tracking | Member 06 (`web`) | Member 08 dispatches real emails upon status changes; Member 06 shows in-app tracking status. |
| **Deployment & Orchestration**| Provides Dockerfiles, config specs, and probe endpoints | Member 09 (`deploy/`) | Member 09 deploys RabbitMQ, manifests, and schedules the reconciliation `CronJob`. |
| **Cross-Flow Test Verification** | Writes unit & integration test suites for relay, workers, and SSRF | Member 10 (`ai-assistant & quality`) | Member 10 coordinates cross-module end-to-end integration flows across all 10 members. |

---

## 10. Summary Checklist of Document 01 Deliverables

- [x] Clear statement of module purpose and dual-write business problem.
- [x] Explicit list of functional responsibilities and non-negotiable exclusions.
- [x] Complete upstream and downstream dependency mapping.
- [x] Authoritative database table ownership and isolation rules.
- [x] Non-negotiable architectural invariants (Outbox atomicity, publisher confirms, relay singleton, SSRF defense, idempotency).
- [x] Full real SMTP email delivery specified from day one (no fake/demo sinks).
- [x] Ownership matrix of files, tables, queues, and environment variables.
- [x] Clear responsibility matrix preventing overlapping implementations with other team members.
