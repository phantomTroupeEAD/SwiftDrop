# 2. Architecture & Subsystem Decomposition

The SwiftDrop Messaging Subsystem is designed as an event-driven asynchronous processing pipeline that bridges the synchronous Go Modular Monolith (`core-api`) with external consumers and third-party systems. In accordance with Domain-Driven Design (DDD) principles and the architectural constraints defined in `AGENTS.md`, message publishing is completely decoupled from Core API request-response lifecycles. This architecture isolates external network failures (such as mail server delays or third-party webhook outages) from core parcel transactions, guarantees that all committed business state changes produce corresponding integration events, and provides independent horizontal scalability for intensive I/O operations.

---

### Figure 2.1 — System Context
**Purpose:** Illustrates the boundaries and high-level data flows between the messaging subsystem, platform components, and external services.

```mermaid
flowchart LR
    classDef producer fill:#dbeafe,stroke:#2563eb,stroke-width:1px;
    classDef messaging fill:#dcfce7,stroke:#16a34a,stroke-width:1px;
    classDef datastore fill:#ffedd5,stroke:#ea580c,stroke-width:1px;
    classDef external fill:#e5e7eb,stroke:#4b5563,stroke-width:1px;
    classDef failure fill:#fee2e2,stroke:#dc2626,stroke-width:1px;

    CoreAPI["Core API Monolith"]:::producer
    Postgres[("PostgreSQL Store")]:::datastore
    Broker["RabbitMQ Broker"]:::datastore
    Messaging["Messaging Subsystem"]:::messaging
    Tracking["Tracking API"]:::producer
    SMTP["SMTP Server"]:::external
    Merchants["Merchant Endpoints"]:::external

    CoreAPI -->|"stages"| Postgres
    Messaging -->|"claims"| Postgres
    Messaging -->|"publishes"| Broker
    Broker -->|"routes"| Messaging
    Broker -->|"streams"| Tracking
    Messaging -->|"sends"| SMTP
    Messaging -->|"dispatches"| Merchants
```

**Key points**
- Core API stages outbox records directly into PostgreSQL without touching external networks.
- The messaging subsystem acts as the exclusive publisher to RabbitMQ and external dispatchers.
- Downstream services such as the Tracking API consume events independently from the shared broker.

---

### Figure 2.2 — Container View
**Purpose:** Details the three independently deployable messaging executables and their communication pathways with platform infrastructure.

```mermaid
flowchart LR
    classDef producer fill:#dbeafe,stroke:#2563eb,stroke-width:1px;
    classDef messaging fill:#dcfce7,stroke:#16a34a,stroke-width:1px;
    classDef datastore fill:#ffedd5,stroke:#ea580c,stroke-width:1px;
    classDef external fill:#e5e7eb,stroke:#4b5563,stroke-width:1px;
    classDef failure fill:#fee2e2,stroke:#dc2626,stroke-width:1px;

    subgraph DataTier["Authoritative Data Tier"]
        DB[("PostgreSQL Database")]:::datastore
        Rabbit["RabbitMQ Message Broker"]:::datastore
    end

    subgraph MessagingDaemons["Messaging Binaries"]
        Relay["Outbox Relay (1 Replica)"]:::messaging
        Worker["Event Worker (1-3 Replicas)"]:::messaging
        Reconciler["Reconciler (CronJob)"]:::messaging
    end

    subgraph ExternalTier["External Systems"]
        MailServer["SMTP Mail Server"]:::external
        MerchantWH["Merchant Webhook Servers"]:::external
    end

    Relay -->|"claims pending"| DB
    Relay -->|"publishes persistent"| Rabbit
    Rabbit -->|"delivers tasks"| Worker
    Worker -->|"leases and updates"| DB
    Worker -->|"transmits TLS"| MailServer
    Worker -->|"HTTPS POST"| MerchantWH
    Reconciler -->|"audits and re-arms"| DB
```

**Key points**
- The Outbox Relay runs as a strict singleton to preserve sequential batch publication.
- Event Worker instances scale horizontally from 1 to 3 replicas based on queue depth.
- The Reconciler runs periodically to clean stale leases without publishing directly to the broker.

---

### Figure 2.3 — Layered Internal Package Structure
**Purpose:** Shows the architectural layering and internal package responsibilities within the messaging codebase.

```mermaid
flowchart TD
    classDef producer fill:#dbeafe,stroke:#2563eb,stroke-width:1px;
    classDef messaging fill:#dcfce7,stroke:#16a34a,stroke-width:1px;
    classDef datastore fill:#ffedd5,stroke:#ea580c,stroke-width:1px;
    classDef external fill:#e5e7eb,stroke:#4b5563,stroke-width:1px;
    classDef failure fill:#fee2e2,stroke:#dc2626,stroke-width:1px;

    subgraph Entrypoints["Entrypoints Layer (cmd/)"]
        CmdRelay["cmd/relay"]:::messaging
        CmdWorker["cmd/worker"]:::messaging
        CmdReconcile["cmd/reconcile"]:::messaging
    end

    subgraph Application["Application Services Layer"]
        OutboxApp["internal/outbox"]:::messaging
        NotifierApp["internal/notifier"]:::messaging
        WebhookApp["internal/webhook"]:::messaging
        ReconcileApp["internal/reconcile"]:::messaging
    end

    subgraph Adapters["Adapters & Infrastructure Layer"]
        BrokerPkg["internal/broker"]:::datastore
        IdempotencyPkg["internal/idempotency"]:::datastore
        HealthPkg["internal/health"]:::messaging
        ConfigPkg["internal/config"]:::messaging
    end

    CmdRelay --> OutboxApp
    CmdWorker --> NotifierApp
    CmdWorker --> WebhookApp
    CmdReconcile --> ReconcileApp

    OutboxApp --> BrokerPkg
    NotifierApp --> BrokerPkg
    NotifierApp --> IdempotencyPkg
    WebhookApp --> BrokerPkg
    WebhookApp --> IdempotencyPkg

    CmdRelay --> HealthPkg
    CmdWorker --> HealthPkg
    CmdRelay --> ConfigPkg
    CmdWorker --> ConfigPkg
    CmdReconcile --> ConfigPkg
```

**Key points**
- Command entrypoints wire dependencies cleanly and launch isolated background runtimes.
- Application logic packages implement core domain handling without depending on concrete transports.
- Shared infrastructure adapters manage RabbitMQ topology, PostgreSQL connections, and Prometheus metrics.

---

### Figure 2.4 — Messaging Broker Topology
**Purpose:** Details exchange configurations, topic routing keys, queue bindings, and dead-letter channels in a readable vertical layout.

```mermaid
flowchart TD
    classDef producer fill:#dbeafe,stroke:#2563eb,stroke-width:1px;
    classDef messaging fill:#dcfce7,stroke:#16a34a,stroke-width:1px;
    classDef datastore fill:#ffedd5,stroke:#ea580c,stroke-width:1px;
    classDef external fill:#e5e7eb,stroke:#4b5563,stroke-width:1px;
    classDef failure fill:#fee2e2,stroke:#dc2626,stroke-width:1px;

    RelayPub["Outbox Relay Publisher"]:::messaging
    Exchange{{"swiftdrop.events.topic"}}:::datastore
    
    subgraph Queues["Durable Application Queues"]
        QTracking[["q.tracking.updates"]]:::datastore
        QNotif[["q.notifications.worker"]]:::datastore
        QWebhook[["q.webhooks.dispatcher"]]:::datastore
    end

    subgraph DeadLetter["Dead-Letter Channel"]
        DLX{{"swiftdrop.dlq.exchange"}}:::failure
        QDLQ[["swiftdrop.dlq"]]:::failure
    end

    TrackingConsumer["Member 5 Tracking API"]:::producer
    WorkerConsumer["Event Worker Consumer"]:::messaging

    RelayPub -->|"publishes"| Exchange
    Exchange -->|"parcel.#, courier.#"| QTracking
    Exchange -->|"parcel.#"| QNotif
    Exchange -->|"parcel.#, courier.#, merchant.#"| QWebhook

    QTracking --> TrackingConsumer
    QNotif --> WorkerConsumer
    QWebhook --> WorkerConsumer

    QNotif -.->|"rejected poison"| DLX
    QWebhook -.->|"rejected poison"| DLX
    DLX --> QDLQ
```

**Key points**
- `swiftdrop.events.topic` distributes events across distinct domain queues using topic pattern matching.
- Queue `q.tracking.updates` is consumed exclusively by Member 05 for SSE public projections.
- Poison messages with unrecoverable formatting errors are rejected directly to `swiftdrop.dlq` without blocking queues.

---

### Figure 2.5 — Kubernetes Deployment View
**Purpose:** Illustrates the orchestration topology, pod replication policies, and scaling mechanisms across the cluster.

```mermaid
flowchart LR
    classDef producer fill:#dbeafe,stroke:#2563eb,stroke-width:1px;
    classDef messaging fill:#dcfce7,stroke:#16a34a,stroke-width:1px;
    classDef datastore fill:#ffedd5,stroke:#ea580c,stroke-width:1px;
    classDef external fill:#e5e7eb,stroke:#4b5563,stroke-width:1px;
    classDef failure fill:#fee2e2,stroke:#dc2626,stroke-width:1px;

    subgraph Cluster["Kubernetes Cluster Boundary"]
        subgraph StatefulServices["Stateful Workloads"]
            PG[(PostgreSQL StatefulSet)]:::datastore
            RMQ[(RabbitMQ StatefulSet)]:::datastore
        end

        subgraph MessagingWorkloads["Messaging Workloads"]
            RelayDep["Deployment: outbox-relay (1 Pod)"]:::messaging
            WorkerDep["Deployment: event-worker (1-3 Pods)"]:::messaging
            KEDA["KEDA ScaledObject (Queue Threshold 10)"]:::messaging
            Cron["CronJob: outbox-reconciler (Every 10m)"]:::messaging
        end

        subgraph Sinks["Integration Sinks"]
            Mailpit["Mailpit (Local/CI Test Sink Only)"]:::external
        end
    end

    RelayDep --> PG
    RelayDep --> RMQ
    WorkerDep --> PG
    WorkerDep --> RMQ
    WorkerDep --> Mailpit
    Cron --> PG
    KEDA -.->|"scales pods"| WorkerDep
    RMQ -.->|"metrics"| KEDA
```

**Key points**
- Outbox Relay runs as a fixed 1-pod Deployment to avoid distributed lock contention.
- Event Worker is autoscaled between 1 and 3 replicas by KEDA based on a queue length threshold of 10 messages.
- RabbitMQ runs as a StatefulSet with a Persistent Volume to safeguard queue durability across pod restarts.
- Mailpit runs in local and CI environments as an in-cluster test sink; production routes to an external SMTP provider.

---

## 2. Component Responsibilities & Operational Behaviors

| Component | Primary Architectural Responsibility | Scaling Rule | Failure & Recovery Behavior |
| :--- | :--- | :--- | :--- |
| **Core API `OutboxWriter`** | Commits event envelopes to `outbox_events` within aggregate transactions. | Scales with Core API (horizontal). | Rolls back with business transaction; guarantees zero phantom events. |
| **Outbox Relay Daemon** | Polls pending rows (`FOR UPDATE SKIP LOCKED`), publishes with confirms, updates `PUBLISHED`. | Exactly 1 replica (singleton). | On crash, unconfirmed rows remain `PENDING`; new pod republishes (at-least-once). |
| **RabbitMQ StatefulSet** | Topic exchange routing, persistent disk buffering, consumer work distribution. | Single replica StatefulSet with PV. | Durable queues and persistent delivery mode preserve messages across restarts. |
| **Event Worker Daemon** | Consumes queues, verifies idempotency, dispatches TLS emails and signed webhooks. | 1 to 3 pods (KEDA autoscaled). | On crash, unreleased DB leases expire (60s) and are re-armed by Reconciler. |
| **Reconciler Engine** | Scans stuck outbox records (>5m), resets expired leases, purges aged data. | CronJob (`*/10 * * * *`, `Forbid`). | Idempotent queries bounded by batch limit 50; logs to structured stdout. |

---

## 3. Architecture Justification

The decision to architect Member 08 as a dedicated asynchronous messaging subsystem rather than embedding publishing and delivery logic directly within the `core-api` web monolith rests on three foundational pillars:

1. **Elimination of Dual-Write Latency and Failure Windows:**  
   If Core API HTTP handlers attempted to publish directly to RabbitMQ during user requests, any network jitter or broker outage would either roll back valid business state changes or cause dual-write inconsistency where a database record exists without an emitted event. By writing exclusively to the transactional outbox table in PostgreSQL within the aggregate transaction, commit times remain minimal (local SQL insert latency), zero network hops are incurred during HTTP requests, and atomic consistency is enforced by relational ACID transactions.

2. **Compliance with DDD Modular Monolith Constraints:**  
   As mandated by `AGENTS.md` §4, Core API domain modules must retain strict package boundaries and zero infrastructure dependencies. Separating the asynchronous daemon plane (`services/messaging/`) allows workers to run complex long-polling loops, retry timers, cryptographic signing, and external network I/O without bloating the Core API container or introducing goroutine leaks into the synchronous request pipeline.

3. **Asymmetric Scalability Between Ingestion and Dispatch:**  
   Publishing outbox events to RabbitMQ is an extremely fast, CPU-light database polling operation. Running the Outbox Relay as a strict singleton completely eliminates row lock contention and out-of-order event delivery while easily sustaining estimated throughputs of over 1,000 events per second. Conversely, outbound webhook dispatches and SMTP transmissions involve high-latency third-party network connections (100ms–5000ms). Separating the Event Worker into an independently autoscaled workload (1–3 pods managed by KEDA) ensures external latency spikes do not starve database connections or delay internal outbox publication.

---

## 4. Requirements Traceability

| Figure | Architectural Focus | Related SRS Requirements | Source Document Reference |
| :--- | :--- | :--- | :--- |
| **Figure 2.1** | System Context & Boundaries | `FR-EVT-001`, `FR-EVT-002`, `FR-EVT-005` | `doc-01` §1; `doc-03` §1 |
| **Figure 2.2** | Container Architecture | `FR-EVT-002`, `FR-EVT-005`, `FR-REC-001` | `doc-03` §1; `doc-08` §1 |
| **Figure 2.3** | Internal Package Layering | `FR-EVT-004`, `FR-EVT-006`, `SEC-010` | `doc-03` §2 |
| **Figure 2.4** | Broker Exchange & Queue Topology | `FR-EVT-002`, `FR-EVT-007`, `FR-MER-009` | `doc-04` §3 |
| **Figure 2.5** | Kubernetes Deployment & KEDA Scaling | `FR-EVT-009`, `FR-REC-001`, `SEC-016` | `doc-08` §6 |
