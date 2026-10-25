# Document 08 — Implementation, Deployment & Operations Guide

> **Module:** Event-Driven Backend & Messaging (`internal/events`, `services/messaging`)  
> **Owning Member:** Member 08 — Event-Driven Backend & Integrations  
> **Status:** Draft Baseline Specification for Review & Implementation  
> **Target Production Locations:**  
> - `core-api/internal/events/` (In-Monolith Outbox Writer & Envelopes)  
> - `services/messaging/` (Outbox Relay Daemon, Real SMTP Worker, Webhook Dispatcher, Reconciler)  
> **Relevant Contracts:** [deployment-guide.md](../../operations/deployment-guide.md), [environment-variables.md](../../operations/environment-variables.md), [runbook.md](../../operations/runbook.md), [local-development.md](../../operations/local-development.md)

---

## 1. Technology Stack, Languages & Libraries

The Messaging & Event subsystem is engineered using standard, production-grade Go tooling without heavy third-party enterprise frameworks:

| Component | Technology | Version | Purpose / Justification |
| :--- | :--- | :--- | :--- |
| **Language Runtime** | Go | `1.22+` | Strongly typed, high concurrency via goroutines, memory safety. |
| **AMQP 0-9-1 Broker Driver** | `github.com/rabbitmq/amqp091-go` | `v1.10.0` | Official RabbitMQ Go driver with Publisher Confirms & Manual Acks. |
| **PostgreSQL Driver** | `github.com/lib/pq` / `pgx/v5` | `v1.10.9` | Relational SQL driver supporting `SKIP LOCKED` and binary JSONB. |
| **UUID Generation** | `github.com/google/uuid` | `v1.6.0` | RFC 4122 / UUIDv7 generation for time-ordered event identifiers. |
| **Metrics Exposition** | `github.com/prometheus/client_golang` | `v1.19.1` | Standard Prometheus metric registry for scrapable operational metrics. |
| **Real SMTP Emailer** | Standard Library `net/smtp` + `crypto/tls` | Go 1.22 Stdlib | Direct TLS/STARTTLS SMTP protocol delivery; zero mock sinks. |
| **Hardened HTTP Transport** | Standard Library `net/http` + `net` | Go 1.22 Stdlib | Custom `SafeDialContext` enforcing single DNS lookup & CIDR blacklist. |
| **Container Engine** | Docker & Kubernetes | Docker 24+, K8s 1.28+ | Multi-stage scratch/alpine containerization and Pod orchestration. |
| **Autoscaling Engine** | KEDA | `v2.14+` | Queue-depth driven horizontal autoscaling for event workers. |

---

## 2. Directory Layout & Application Entrypoints

The repository maintains an explicit separation between monolith integration plumbing and standalone worker daemons:

```
SwiftDrop Repository
├── core-api/
│   └── internal/
│       └── events/                       # Monolith In-Process Support Library
│           ├── envelope.go               # Generic EventEnvelope[T] struct
│           ├── writer.go                 # OutboxWriter interface definition
│           ├── repository.go             # PostgreSQL outbox writer implementation
│           ├── catalog.go                # Centralized event type string constants
│           └── errors.go                 # Domain event errors
└── services/
    └── messaging/                        # Standalone Deployable Daemons
        ├── cmd/
        │   ├── relay/main.go             # Entrypoint: Outbox Relay Daemon (Singleton)
        │   ├── worker/main.go            # Entrypoint: Multithreaded Event Worker
        │   └── reconcile/main.go         # Entrypoint: Reconciliation CLI / CronJob
        ├── internal/
        │   ├── config/config.go          # Strongly typed environment configuration
        │   ├── broker/                   # RabbitMQ connection, publisher confirms & consumer
        │   ├── outbox/                   # Outbox polling engine (SKIP LOCKED)
        │   ├── idempotency/              # processed_events store
        │   ├── notifier/                 # Real SMTP mailer, templates & delivery logging
        │   ├── webhook/                  # SafeDialContext, HMAC signer, backoff calculator
        │   └── reconcile/                # Stale outbox discovery and retry rescheduling
        ├── tests/                        # Integration and end-to-end tests
        ├── Dockerfile                    # Multi-stage multi-binary Dockerfile (Agreement A3)
        ├── go.mod
        ├── go.sum
        └── README.md                     # Living documentation
```

### Binary Entrypoints:
1. `services/messaging/cmd/relay/main.go`: Launches the single-threaded poller that streams pending rows from `outbox_events` to RabbitMQ with publisher confirms.
2. `services/messaging/cmd/worker/main.go`: Launches the concurrent consumer subscribing to `q.notifications.worker` (real SMTP delivery) and `q.webhooks.dispatcher` (SSRF-hardened merchant webhooks).
3. `services/messaging/cmd/reconcile/main.go`: Standalone task executed on a schedule or manually to audit stuck pending outbox rows and reschedule expired retries.

---

## 3. Environment Variables & Configuration Reference

Create local environment files from this template (`services/messaging/.env`):

```bash
# ==============================================================================
# SwiftDrop Messaging Subsystem Configuration
# ==============================================================================

# Global Runtime Environment
APP_ENV=local                             # local | staging | production
LOG_LEVEL=info                            # debug | info | warn | error

# PostgreSQL Database Connections (Per-Role Least-Privilege Credentials)
# Relay Daemon (swiftdrop_relay role: outbox_events access only)
RELAY_DATABASE_URL=postgres://swiftdrop_relay:relay_secret_pass@localhost:5432/swiftdrop_db?sslmode=disable
RELAY_DB_MAX_OPEN_CONNS=5
RELAY_DB_MAX_IDLE_CONNS=2

# Worker Daemon (swiftdrop_worker role: processed_events, deliveries, targets view)
WORKER_DATABASE_URL=postgres://swiftdrop_worker:worker_secret_pass@localhost:5432/swiftdrop_db?sslmode=disable
WORKER_DB_MAX_OPEN_CONNS=10
WORKER_DB_MAX_IDLE_CONNS=5

# Reconciler Daemon (swiftdrop_reconciler role: audit, reset expired leases, retention purge)
RECONCILER_DATABASE_URL=postgres://swiftdrop_reconciler:reconcile_secret_pass@localhost:5432/swiftdrop_db?sslmode=disable
RECONCILER_DB_MAX_OPEN_CONNS=2
RECONCILER_DB_MAX_IDLE_CONNS=1

# RabbitMQ Message Broker
RABBITMQ_URL=amqp://swiftdrop:app_secret_pass@localhost:5672/   # in-cluster: amqp://swiftdrop:<pw>@rabbitmq.swiftdrop.svc.cluster.local:5672/ (Agreement A5)
RABBITMQ_RECONNECT_BACKOFF_SECONDS=2      # Initial retry backoff on connection loss
RABBITMQ_PREFETCH_COUNT=20                # AMQP prefetch buffer per consumer pod

# Outbox Relay Daemon Parameters
OUTBOX_POLL_INTERVAL_MS=500               # Frequency of SKIP LOCKED batch polls (ms)
OUTBOX_BATCH_SIZE=50                      # Batch limit per poll tick
OUTBOX_STALE_SECONDS=300                  # Alert threshold (5m) for unconfirmed pending events

# Event Worker Parameters
WORKER_CONCURRENCY=10                     # Goroutine worker pool capacity
WEBHOOK_HTTP_TIMEOUT_SECONDS=5            # Strict socket timeout for outbound HTTP POSTs
WEBHOOK_RETRY_SCHEDULE=0s,30s,2m,10m,30m  # Explicit 5-attempt retry intervals
WEBHOOK_ALLOW_PRIVATE_HOSTS=false         # Prohibit private/loopback IPs (true strictly in local dev test)

# Real SMTP Email Server Configuration (MANDATORY: NO MOCK CODE PATHS)
# In local/CI: uses Mailpit container on localhost:1025 with SMTP_SECURITY=none
# In staging/prod: uses provider on port 587/465 with SMTP_SECURITY=starttls|tls
SMTP_HOST=localhost                       # "mailpit" in Docker Compose; SMTP provider in prod
SMTP_PORT=1025                            # 1025 (local Mailpit), 587 (STARTTLS), 465 (implicit TLS)
SMTP_SECURITY=none                        # "none" (local Mailpit only), "starttls", or "tls"
SMTP_USERNAME=""                          # SMTP auth username (empty for local Mailpit)
SMTP_PASSWORD=""                          # SMTP auth password / API token (Secret)
SMTP_FROM_EMAIL=notifications@swiftdrop.io
SMTP_FROM_NAME="SwiftDrop Deliveries"
SMTP_DIAL_TIMEOUT_SECONDS=5

# Operational Health & Metrics Probe Server Ports (Agreement A1/A2)
PROBE_PORT=8082                           # 8082 for relay; 8083 for worker

# Webhook Secret Encryption Key (Member 04 & 09 Agreement A5)
# 32-byte (64 hex chars) key for AES-256-GCM webhook secret decryption (Secret: integrations-secrets, shared with core-api).
# Used for secret_key_version = 1; key rotation naming to be agreed with Member 04/09.
WEBHOOK_SECRET_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

---

## 4. Local Development, Build & Run Instructions

### 4.1 Prerequisites
Ensure the following tools are running locally:
- Go `1.22+` (`go version`)
- Docker & Docker Compose (`docker compose version`)
- PostgreSQL 16+, RabbitMQ 3.12+, and Mailpit (launched via Compose)

### 4.2 Step-by-Step Developer Workflow

#### Step 1: Start Infrastructure Containers
From repository root:
```bash
docker compose up -d postgres rabbitmq mailpit
```
*Verify containers are healthy:*
```bash
docker compose ps
# PostgreSQL: localhost:5432
# RabbitMQ AMQP: localhost:5672 | Management UI: http://localhost:15672 (guest/guest)
# Mailpit SMTP: localhost:1025 | Web UI: http://localhost:8025
```

#### Step 2: Apply Database Migrations
Apply sequential database migrations (including Member 08 migrations `000008` and `000009`):
```bash
migrate -path core-api/migrations -database "postgres://swiftdrop_migrator:secret@localhost:5432/swiftdrop_db?sslmode=disable" up
```

#### Step 3: Run the Outbox Relay Daemon
In Terminal 1:
```bash
cd services/messaging
PROBE_PORT=8082 DATABASE_URL=$RELAY_DATABASE_URL go run cmd/relay/main.go
# Outputs structured JSON: {"level":"INFO","msg":"Outbox Relay started. Polling every 500ms"}
```

#### Step 4: Run the Event Worker Service
In Terminal 2:
```bash
cd services/messaging
PROBE_PORT=8083 DATABASE_URL=$WORKER_DATABASE_URL go run cmd/worker/main.go
# Outputs structured JSON: {"level":"INFO","msg":"Event Worker consumer pool started. Concurrency: 10"}
```

#### Step 5: Execute Automated Quality Gates
Run formatting, static linting, and race-detection tests:
```bash
# 1. Run unit and integration tests with data race detection
cd services/messaging && go test -v -race ./...

# 2. Run repository-wide automated verification script
./scripts/verify.sh
```

---

## 5. Docker Container Build & Multi-Stage Packaging

Daemons are packaged into a single hardened, non-root multi-binary image adhering to Member 09 Agreement A3:

### 5.1 Dockerfile (`services/messaging/Dockerfile`)
```dockerfile
# ------------------------------------------------------------------------------
# Build Stage
# ------------------------------------------------------------------------------
FROM golang:1.23.2-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache ca-certificates tzdata

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source tree
COPY . .

# Compile all three statically linked binary entrypoints with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/relay ./cmd/relay && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/reconcile ./cmd/reconcile

# ------------------------------------------------------------------------------
# Production Hardened Runtime Image
# ------------------------------------------------------------------------------
FROM alpine:3.20

# Create standard SwiftDrop non-root user (Agreement A3: UID/GID 10001)
RUN addgroup -g 10001 swiftdrop && \
    adduser -D -u 10001 -G swiftdrop swiftdrop && \
    apk add --no-cache ca-certificates tzdata

# Copy compiled binaries into /usr/local/bin/
COPY --from=builder /bin/relay /usr/local/bin/relay
COPY --from=builder /bin/worker /usr/local/bin/worker
COPY --from=builder /bin/reconcile /usr/local/bin/reconcile

# Drop all privileges and run as non-root user
USER 10001:10001

# Operational probe ports (Relay: 8082, Worker: 8083)
EXPOSE 8082 8083

# Default entrypoint (overridden per Kubernetes Deployment)
ENTRYPOINT ["/usr/local/bin/worker"]
```

### 5.2 Build & Tagging Commands (Agreement A3)
```bash
# Build Single Multi-Binary Image with Pinned Git SHA:
GIT_SHA=$(git rev-parse --short HEAD)
docker build -t ghcr.io/phantomtroupeead/swiftdrop/messaging:${GIT_SHA} -f services/messaging/Dockerfile services/messaging
```

---

## 6. Kubernetes Deployment, Autoscaling & CronJob Specs

Member 08 provides the specifications and manifests for Member 09 to deploy to the Kubernetes cluster.

### 6.1 Outbox Relay Manifest (`deploy/kubernetes/outbox-relay.yml`)
> [!IMPORTANT]
> The Outbox Relay MUST remain a strict singleton (`replicas: 1`). Never scale horizontally without distributed leader election.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: outbox-relay
  namespace: swiftdrop
  labels:
    app.kubernetes.io/name: outbox-relay
    app.kubernetes.io/part-of: swiftdrop
spec:
  replicas: 1 # Strict Singleton Invariant
  strategy:
    type: Recreate # Prevents two pods from running concurrently during rolling updates
  selector:
    matchLabels:
      app: outbox-relay
  template:
    metadata:
      labels:
        app: outbox-relay
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8082"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: swiftdrop-relay-sa
      terminationGracePeriodSeconds: 30
      securityContext:
        runAsNonRoot: true
        runAsUser: 10001
        runAsGroup: 10001
        fsGroup: 10001
      containers:
        - name: outbox-relay
          image: ghcr.io/phantomtroupeead/swiftdrop/messaging:${IMAGE_TAG}
          command: ["/usr/local/bin/relay"]
          ports:
            - containerPort: 8082
              name: http-probes
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
          envFrom:
            - configMapRef:
                name: messaging-config
            - secretRef:
                name: outbox-relay-secrets # Dedicated credentials for swiftdrop_relay role
            - secretRef:
                name: rabbitmq-secrets
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
          startupProbe:
            httpGet:
              path: /health/live
              port: 8082
            initialDelaySeconds: 2
            periodSeconds: 3
            failureThreshold: 10
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8082
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8082
            initialDelaySeconds: 5
            periodSeconds: 5
```

---

### 6.2 Event Worker & KEDA Autoscaler (`deploy/kubernetes/event-worker.yml`)
Stateless consumer scaling horizontally based on RabbitMQ queue backlog:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: event-worker
  namespace: swiftdrop
  labels:
    app.kubernetes.io/name: event-worker
    app.kubernetes.io/part-of: swiftdrop
spec:
  replicas: 1
  selector:
    matchLabels:
      app: event-worker
  template:
    metadata:
      labels:
        app: event-worker
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8083"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: swiftdrop-worker-sa
      terminationGracePeriodSeconds: 30
      securityContext:
        runAsNonRoot: true
        runAsUser: 10001
        runAsGroup: 10001
        fsGroup: 10001
      containers:
        - name: event-worker
          image: ghcr.io/phantomtroupeead/swiftdrop/messaging:${IMAGE_TAG}
          command: ["/usr/local/bin/worker"]
          ports:
            - containerPort: 8083
              name: http-probes
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
          envFrom:
            - configMapRef:
                name: messaging-config
            - secretRef:
                name: event-worker-secrets # Dedicated credentials for swiftdrop_worker role
            - secretRef:
                name: rabbitmq-secrets
            - secretRef:
                name: integrations-secrets
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
          startupProbe:
            httpGet:
              path: /health/live
              port: 8083
            initialDelaySeconds: 2
            periodSeconds: 3
            failureThreshold: 10
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8083
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8083
            initialDelaySeconds: 5
            periodSeconds: 5
---
# KEDA TriggerAuthentication for RabbitMQ
apiVersion: keda.sh/v1alpha1
kind: TriggerAuthentication
metadata:
  name: keda-rabbitmq-auth
  namespace: swiftdrop
spec:
  secretTargetRef:
    - parameter: host
      name: rabbitmq-secrets
      key: RABBITMQ_URL
---
# KEDA Autoscaler based on Queue Length (Bounded to 1-3 replicas to prevent pool exhaustion)
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: event-worker-scaler
  namespace: swiftdrop
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: event-worker
  minReplicaCount: 1
  maxReplicaCount: 3 # Hard limit to prevent PostgreSQL connection saturation
  pollingInterval: 15
  cooldownPeriod: 30
  triggers:
    - type: rabbitmq
      metadata:
        protocol: amqp
        queueName: q.notifications.worker
        mode: QueueLength
        value: "10" # Scales up when notification queue backlog exceeds 10 messages
      authenticationRef:
        name: keda-rabbitmq-auth
    - type: rabbitmq
      metadata:
        protocol: amqp
        queueName: q.webhooks.dispatcher
        mode: QueueLength
        value: "10" # Scales up when webhook dispatcher queue backlog exceeds 10 messages
      authenticationRef:
        name: keda-rabbitmq-auth
```

---

### 6.3 Scheduled Reconciliation CronJob (`deploy/kubernetes/reconciliation-cronjob.yml`)
Periodically audits for stuck outbox events (>5m), resets expired in-progress leases, and executes retention purges:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: swiftdrop-outbox-reconciler
  namespace: swiftdrop
spec:
  schedule: "*/10 * * * *" # Runs every 10 minutes
  concurrencyPolicy: Forbid
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 5
  jobTemplate:
    spec:
      backoffLimit: 2
      template:
        spec:
          serviceAccountName: swiftdrop-reconciler-sa
          restartPolicy: OnFailure
          securityContext:
            runAsNonRoot: true
            runAsUser: 10001
            runAsGroup: 10001
          containers:
            - name: reconciler
              image: ghcr.io/phantomtroupeead/swiftdrop/messaging:${IMAGE_TAG}
              command: ["/usr/local/bin/reconcile"]
              args: ["--stale-threshold=5m", "--batch-size=50"]
              securityContext:
                allowPrivilegeEscalation: false
                readOnlyRootFilesystem: true
                capabilities:
                  drop:
                    - ALL
              envFrom:
                - configMapRef:
                    name: messaging-config
                - secretRef:
                    name: outbox-reconciler-secrets # Dedicated credentials for swiftdrop_reconciler role
                - secretRef:
                    name: rabbitmq-secrets
              resources:
                requests:
                  cpu: 50m
                  memory: 64Mi
                limits:
                  cpu: 250m
                  memory: 128Mi
```
*Note:* The reconciler binary also supports manual dead-letter queue replay via the CLI flag `--replay-dlq`.

---

## 7. Observability, Prometheus Metrics & Alerts

### 7.1 Prometheus Metrics Exposed (`/metrics`)
Both daemons expose Prometheus counters and histograms:

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `swiftdrop_outbox_pending_events` | Gauge | Current count of unconfirmed pending rows in `outbox_events`. |
| `swiftdrop_outbox_oldest_pending_age_seconds` | Gauge | Age in seconds of the oldest unconfirmed row with `status = 'PENDING'`. |
| `swiftdrop_outbox_publish_duration_seconds` | Histogram | Latency distribution of RabbitMQ publisher confirms. |
| `swiftdrop_outbox_publish_failures_total` | Counter | Total count of failed AMQP publish attempts. |
| `swiftdrop_smtp_deliveries_total` | Counter | Total real SMTP emails dispatched (labeled by `status=SUCCESS|FAILED`). |
| `swiftdrop_smtp_latency_seconds` | Histogram | Latency distribution of TLS SMTP handshakes and data transmission. |
| `swiftdrop_webhook_delivery_attempts_total` | Counter | Total outbound webhook attempts (labeled by `http_status` & `status`). |
| `swiftdrop_webhook_ssrf_blocked_total` | Counter | Total outbound webhooks aborted due to SSRF blacklist detection. |
| `swiftdrop_dlq_depth` | Gauge | Current number of unacknowledged poison messages in `swiftdrop.dlq`. |
| `swiftdrop_retention_purged_rows_total` | Counter | Total rows purged by retention jobs (labeled by `table_name`). |

### 7.2 Core Alerting Rules
- **OutboxBacklogHigh:** `swiftdrop_outbox_pending_events > 100` for 5m. (Indicates Outbox Relay is stuck or broker is unreachable).
- **DeadLetterQueueActive:** `increase(swiftdrop_dlq_messages_routed_total[10m]) > 0`. (Alerts on poison pills or dying merchant endpoints).
- **SMTPFailureRateHigh:** `rate(swiftdrop_smtp_deliveries_total{status="FAILED"}[5m]) > 0.05`. (Alerts on bad SMTP credentials or provider outage).
- **SSRFViolationDetected:** `increase(swiftdrop_webhook_ssrf_blocked_total[5m]) > 0`. (Alerts security team to malicious webhook registrations).

---

## 8. Graceful Shutdown & Startup Lifecycles

To prevent message loss or interrupted TCP dialogues during container updates:

### 8.1 Startup Sequence
1. Load configuration and validate required secrets (`DATABASE_URL`, `RABBITMQ_URL`, `SMTP_PASSWORD`).
2. Establish PostgreSQL connection pool; verify connectivity via `db.PingContext(ctx)`.
3. Establish RabbitMQ AMQP connection; declare topology (`swiftdrop.events.topic` exchange and durable queues).
4. For Real SMTP Worker: Perform initial test handshake to verify `SMTP_HOST` connectivity.
5. Bind HTTP server on probe port (`/health/live`, `/health/ready`, `/metrics`); mark readiness probe `200 OK`.
6. Start background worker goroutines.

### 8.2 Graceful Shutdown Sequence (`SIGTERM` / `SIGINT`)
1. Intercept `os.Interrupt` / `syscall.SIGTERM`.
2. Immediately set `/health/ready` probe to `503 Service Unavailable` so Kubernetes detaches traffic.
3. **Outbox Relay:** Stop poller ticker. Wait for current batch publisher confirms to complete (timeout: 10s).
4. **Event Worker:** Cancel AMQP channel subscriptions. Allow active worker goroutines up to 15 seconds to complete in-flight SMTP handshakes and webhook requests.
5. Close RabbitMQ AMQP connection.
6. Close PostgreSQL database connection pool.
7. Process terminates with exit code `0`.

---

## 9. Known Limitations & Operational Caveats

1. **Relay Single-Pod Throughput Ceiling:** A single Outbox Relay pod processes $\sim 500\text{ msg/sec}$ sustained. For future workloads exceeding 5,000 msg/sec, distributed partition hashing (e.g. `aggregate_id % N`) or leader election (Raft/Postgres advisory locks) will be required to scale horizontally.
2. **Real SMTP Provider Quotas:** Third-party SMTP providers (e.g. Mailtrap Free, SendGrid Essentials) enforce strict hourly sending limits. If the quota is exceeded, the mail server returns `421` or `452`, triggering exponential worker backoff. Production deployments must use a dedicated high-volume SMTP relay (e.g. Amazon SES).
3. **Webhook Receiver Latency:** If many merchant endpoints take the full 5.0-second socket timeout, worker goroutines can become saturated. KEDA horizontal scaling mitigates this by spawning additional worker replicas as the queue backlog grows.

---

## 10. Summary Checklist of Document 08 Deliverables

- [x] Complete inventory of programming languages, Go version (`1.22+`), and libraries.
- [x] Full folder layout and binary entrypoints for relay, worker, and reconciler.
- [x] Comprehensive `.env.example` reference with descriptions and safe values.
- [x] Real SMTP server configuration specified from day one (no fake/mock sinks).
- [x] Local developer setup guide: Docker commands, migration steps, and execution commands.
- [x] Multi-stage alpine Dockerfile with standard non-root user execution (`USER 10001:10001`, Agreement A3).
- [x] Complete Kubernetes manifests: Singleton Relay Deployment, KEDA ScaledObject, and Reconciliation CronJob.
- [x] Scrapable Prometheus metrics list and critical alerting thresholds.
- [x] Graceful startup and shutdown lifecycle handling `SIGTERM` with in-flight drain.
- [x] Documented architectural limitations, relay scaling ceilings, and SMTP provider caveats.
