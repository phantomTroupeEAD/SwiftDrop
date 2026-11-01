# 4. Sequence & Activity Flows

This section models the end-to-end execution sequences for critical asynchronous operations within SwiftDrop. In accordance with assignment grading criteria, these diagrams capture both the nominal "happy path" and operational failure branches for the assignment-mandated flow ("Courier status update triggering notifications"), the Outbox Relay batch publication loop, customer transactional email delivery, signed merchant webhook dispatching with SSRF defenses, and automated failure recovery. Every flow enforces bounded contexts, atomic database transactions, at-least-once delivery semantics, and idempotent consumer guarantees.

---

### Figure 4.1 — Courier Status Update Triggering Notifications (Assignment Required Flow)
**Purpose:** Demonstrates how a courier delivery status update commits atomically with an outbox event, propagates through RabbitMQ, and triggers notifications.

```mermaid
sequenceDiagram
    autonumber
    participant Courier as Courier Web App
    participant Core as Core API
    participant DB as PostgreSQL
    participant Relay as Outbox Relay
    participant Broker as RabbitMQ
    participant Worker as Event Worker

    Courier->>Core: POST /deliveries/{id}/complete
    activate Core
    Note over Core,DB: Single PostgreSQL Transaction
    Core->>DB: Apply status transition (Parcel aggregate)
    Core->>DB: INSERT parcel_status_history
    Core->>DB: INSERT outbox_events (parcel.delivered.v1)
    DB-->>Core: Commit OK
    Core-->>Courier: 200 OK (Completed)
    deactivate Core

    Relay->>DB: Claim pending (SKIP LOCKED)
    activate Relay
    DB-->>Relay: Return row
    Relay->>Broker: Publish persistent message
    Broker-->>Relay: Confirm ACK
    Relay->>DB: Mark status = 'PUBLISHED'
    deactivate Relay

    Broker->>Worker: Deliver from q.notifications.worker
    activate Worker
    Worker->>DB: Claim delivery task (60s lease)
    Worker->>Worker: Render & send notification
    Worker->>DB: Record SUCCESS & processed_events
    Worker-->>Broker: AMQP Ack
    deactivate Worker
```

**Key points**
- Core API commits domain state, status history, and the outbox event in one atomic transaction.
- The Outbox Relay publishes to RabbitMQ asynchronously and marks the record `PUBLISHED` only after receiving broker confirmation.
- The Event Worker consumes the message, claims a lease, executes the notification, and acknowledges the message.

**Operational Notes**
- **Atomic Transaction:** Domain state, status history, and the outbox event commit in a single PostgreSQL transaction, ensuring no event is created if the status update fails.
- **Publisher Confirms:** RabbitMQ confirms persistent write (`DeliveryMode = 2`) before the relay updates the database status to `PUBLISHED`.
- **Task Lease:** The delivery task lease (`lease_expires_at = NOW() + 60s`) prevents concurrent execution by other worker pods.

---

### Figure 4.2 — Outbox Relay Publication Loop
**Purpose:** Details the batch claiming, publisher confirm validation, and error backoff handling of the Outbox Relay daemon.

```mermaid
sequenceDiagram
    autonumber
    participant Relay as Outbox Relay
    participant DB as PostgreSQL
    participant Broker as RabbitMQ

    loop Every 500ms Ticker
        Relay->>DB: Claim PENDING batch (SKIP LOCKED)
        alt No pending rows
            DB-->>Relay: Empty set
        else Batch returned (up to 50 rows)
            DB-->>Relay: Return pending rows
            loop For each row in batch
                Relay->>Broker: Publish message (DeliveryMode 2)
                alt Broker confirms ACK within 5s
                    Broker-->>Relay: Confirm ACK
                    Relay->>DB: UPDATE status = 'PUBLISHED'
                else Broker NACK or timeout
                    Broker-->>Relay: NACK or Timeout (5s)
                    Relay->>DB: Increment attempt & schedule delay
                end
            end
        end
    end
```

**Key points**
- `FOR UPDATE SKIP LOCKED` guarantees non-blocking queries and prevents lock contention.
- Broker confirms are evaluated per message; successful messages commit immediately without batch-wide rollbacks.
- Unconfirmed messages stay `PENDING` with an incremented attempt counter and scheduled retry backoff.

**Operational Notes**
- **Confirmation Timeout:** Publisher confirms run under a 5.0-second context deadline to avoid indefinite relay thread hangs.
- **Retry Exhaustion:** If `attempt_count` reaches 5, the relay transitions the status to `FAILED` and raises a Prometheus alert.

---

### Figure 4.3 — Customer Email Notification Delivery Flow
**Purpose:** Illustrates real SMTP delivery handling with TLS, lease claims, and post-effect deduplication.

```mermaid
sequenceDiagram
    autonumber
    participant Broker as RabbitMQ
    participant Worker as Event Worker
    participant DB as PostgreSQL
    participant SMTP as SMTP Server (587)

    Broker->>Worker: Deliver message (parcel.delivered.v1)
    activate Worker
    Worker->>DB: Claim task (lease = 60s)
    
    alt Task Already SUCCESS or FAILED
        DB-->>Worker: Already completed
        Worker-->>Broker: AMQP Ack (Duplicate ignored)
    else Lease Granted
        DB-->>Worker: Lease active
        Worker->>Worker: Render template
        Worker->>SMTP: Connect STARTTLS, AUTH, send DATA
        
        alt SMTP Returns 250 OK
            SMTP-->>Worker: 250 OK (Message Queued)
            Worker->>DB: UPDATE status = 'SUCCESS' & processed_events
            Worker-->>Broker: AMQP Ack
        else SMTP Returns 4xx / Timeout (Retryable)
            SMTP-->>Worker: 421 Service Unavailable
            Worker->>DB: UPDATE status = 'PENDING' with delay
            Worker-->>Broker: AMQP Ack (Scheduled for DB poller)
        end
    end
    deactivate Worker
```

**Key points**
- Deduplication is evaluated via database leases, ensuring safe idempotent handling of redelivered messages.
- The mailer communicates with a real SMTP server using standard TLS/STARTTLS authentication.
- Deduplication records in `processed_events` are committed only after successful transmission.

**Operational Notes**
- **Lease Protection:** Delivery tasks are claimed with `status = 'IN_PROGRESS'` to prevent split-brain delivery across worker replicas.
- **Retry Scheduling:** Transient SMTP failures (4xx) return the status to `PENDING` with exponential backoff and ACK the AMQP message.

---

### Figure 4.4 — Merchant Webhook Delivery & SSRF Defense Flow
**Purpose:** Depicts the secure dispatching of outbound merchant webhooks, including secret decryption, signature computation, and SSRF filtering.

```mermaid
sequenceDiagram
    autonumber
    participant Broker as RabbitMQ
    participant Worker as Event Worker
    participant DB as PostgreSQL
    participant DNS as DNS Resolver
    participant Target as Merchant Server

    Broker->>Worker: Deliver message (parcel.delivered.v1)
    activate Worker
    Worker->>DB: Query merchant_webhook_targets
    DB-->>Worker: Return target URL & AES ciphertext
    Worker->>Worker: Decrypt secret & compute HMAC signature
    
    Note over Worker,DNS: SSRF SafeDialContext Guard
    Worker->>DNS: Resolve hostname (LookupIP)
    DNS-->>Worker: Resolved IP addresses
    
    alt Any IP in Blocked CIDR (Private, Loopback, Metadata)
        Worker->>DB: Record attempt FAILED (SSRF_BLOCKED)
        Worker-->>Broker: AMQP Ack (Security block)
    else All IPs Public & Valid
        Worker->>Target: POST payload with signature
        alt HTTP 2xx Success
            Target-->>Worker: 200 OK
            Worker->>DB: Record SUCCESS in deliveries and attempts
            Worker-->>Broker: AMQP Ack
        else HTTP 5xx / 429 / Timeout (Retryable)
            Target-->>Worker: 503 Service Unavailable
            Worker->>DB: Record attempt & reschedule PENDING
            Worker-->>Broker: AMQP Ack
        end
    end
    deactivate Worker
```

**Key points**
- Signing secrets are decrypted in memory using AES-256-GCM from the read-only targets view.
- Every outbound connection validates all resolved IP addresses against a strict private/cloud-metadata blocklist.
- Successful transmissions record response metrics and append immutable attempt logs (FR-MER-011).

**Operational Notes**
- **DNS Rebinding Prevention:** `SafeDialContext` executes DNS resolution once and binds directly to the verified public IP to prevent DNS rebinding.
- **Terminal Rejections:** Non-retryable errors (4xx client errors, 3xx redirects) immediately mark the delivery as `FAILED` without retrying.

---

### Figure 4.5 — Worker Crash and Automated Lease Recovery Flow
**Purpose:** Details how the system self-heals when a worker crashes mid-transmission using expired lease reclamation in a compact sequence.

```mermaid
sequenceDiagram
    autonumber
    participant WorkerA as Worker Pod 1
    participant DB as PostgreSQL
    participant Cron as Reconciler
    participant WorkerB as Worker Pod 2

    WorkerA->>DB: Claim task (lease = NOW() + 60s)
    DB-->>WorkerA: Lease granted
    Note over WorkerA: Pod crashes mid-execution
    Note over WorkerA,DB: 60s Lease Expires

    Cron->>DB: Run audit (every 10m)
    activate Cron
    Cron->>DB: Reset expired leases to PENDING
    DB-->>Cron: 1 lease re-armed
    deactivate Cron

    WorkerB->>DB: Poll due tasks (next_attempt_at <= NOW())
    activate WorkerB
    DB-->>WorkerB: Return reclaimed row
    WorkerB->>WorkerB: Execute delivery successfully
    WorkerB->>DB: UPDATE status = 'SUCCESS'
    deactivate WorkerB
```

**Key points**
- Worker crashes never permanently strand tasks because delivery rows carry bounded 60-second leases.
- The Reconciler CronJob re-arms expired leases back to `PENDING` without modifying attempt counters.
- Available worker pods pick up reclaimed tasks on their next polling cycle and execute them to completion.

**Operational Notes**
- **Preserved Attempt Counters:** Reconciler resets only the lease and status; it preserves `attempt_count` to prevent infinite execution of poison tasks.
- **Database Boundary:** Preserves **SRS FR-REC-005** by updating database state only and never publishing events directly to RabbitMQ.

---

### Figure 4.6 — Dead-Letter Queue (DLQ) Routing & Operator Replay Flow
**Purpose:** Illustrates how unparseable poison messages route to the DLQ and how operators safely inspect and replay them.

```mermaid
sequenceDiagram
    autonumber
    participant Broker as Normal Queue
    participant Worker as Event Worker
    participant DLQ as swiftdrop.dlq
    participant Operator as Operator CLI
    participant Job as Replay Job
    participant DB as outbox_events
    participant Relay as Outbox Relay

    Broker->>Worker: Deliver malformed message
    activate Worker
    Worker->>Worker: Validation fails
    Worker->>Broker: Reject (requeue=false)
    deactivate Worker
    Broker->>DLQ: Route to DLQ via exchange

    Operator->>DLQ: Inspect poison message (kubectl)
    Note over Operator: Identify root cause & deploy fix
    Operator->>Job: Trigger Job (reconcile --replay-dlq)
    activate Job
    Note over Job: Mounts DB & RabbitMQ secrets
    Job->>DLQ: Consume poison message
    Job->>DB: Stage clean row (status = 'PENDING')
    Job-->>Operator: Replay complete (logs to stdout)
    deactivate Job

    Relay->>DB: Poll staged event (SKIP LOCKED)
    Relay->>Broker: Publish confirmed message
```

**Key points**
- Corrupted messages are rejected with `requeue = false`, diverting them immediately to `swiftdrop.dlq`.
- Dead-lettered messages are retained for 30 days without impacting healthy message processing.
- The replay Job consumes from the DLQ and stages clean rows in `outbox_events`, preserving the outbox publication guarantee (FR-REC-005).

**Operational Notes**
- **Dead-Letter Routing:** Rejecting with `requeue = false` triggers RabbitMQ dead-letter exchange forwarding.
- **Credentials & Path:** The replay Job mounts both `outbox-reconciler-secrets` (DB) and `rabbitmq-secrets` (AMQP) to safely consume from the DLQ and stage events into `outbox_events`.

---

## 2. Requirements Traceability

| Figure | Flow Description | Related SRS Requirements | Source Document Reference |
| :--- | :--- | :--- | :--- |
| **Figure 4.1** | Courier Update & Notification Dispatch | `FR-EVT-001`, `FR-EVT-002`, `FR-EVT-005`, `FR-EVT-008` | `doc-02` §3 (Flows 1 & 3); `sequence-diagrams.md` |
| **Figure 4.2** | Outbox Relay Publication Loop | `FR-EVT-002`, `FR-EVT-003`, `FR-EVT-007` | `doc-02` §3 (Flow 2); `doc-03` §3.3 |
| **Figure 4.3** | Real SMTP Customer Notification | `FR-EVT-005`, `FR-EVT-006`, `FR-EVT-008` | `doc-02` §3 (Flow 3); `doc-07` §2 |
| **Figure 4.4** | Webhook Dispatch & SSRF Defense | `FR-EVT-005`, `FR-MER-009`, `FR-MER-010`, `SEC-010`, `SEC-011` | `doc-02` §3 (Flow 4); `doc-04` §4 |
| **Figure 4.5** | Worker Crash & Lease Recovery | `FR-EVT-006`, `FR-REC-002`, `FR-REC-004`, `FR-REC-005` | `doc-02` §5, §7; `doc-07` §3 |
| **Figure 4.6** | Poison Message Routing & DLQ Replay | `FR-EVT-007`, `FR-EVT-009`, `SEC-015` | `doc-02` §7; `doc-07` §3.4 |
