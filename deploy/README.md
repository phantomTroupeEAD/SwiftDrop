# SwiftDrop Infrastructure & Deployment Tier (`deploy/`)

> **Primary Ownership:**  
> - **Member 9:** DevOps, Kubernetes Manifests, Helm Charts, CI/CD, Observability & Database Coordinator  
> - **Member 7:** API Gateway & Edge Routing Policies (`deploy/gateway/`)  
> - **Member 8:** Outbox Reconciliation CronJob & Event Worker Deployment  
> **Target Environment:** K3s Lightweight Kubernetes on a single-node VPS (at least 8 GB RAM; provider open, see ADR-002 §2) | Local Docker Compose

---

## 1. Workload Architecture & Service Topology

To guarantee data safety, high availability, and prevent volume mount conflicts, cluster workloads are strictly separated into **Stateless**, **Stateful**, and **Scheduled** paradigms:

```
SwiftDrop Kubernetes Cluster (Namespace: `swiftdrop`)
├── Edge & Gateway Tier:
│   └── Traefik Ingress Controller (Ports: :80 HTTP, :443 HTTPS, :8090 Dashboard)
├── Stateless Workloads (Deployments):
│   ├── core-api (Go Modular Monolith: :8080)
│   ├── tracking-api (Go SSE Service: :8081 — Autoscaled via HPA, replicas 1-2 today; final range open, C8)
│   ├── ai-assistant (FastAPI Service: :8000 — Zero DB access)
│   ├── web (React/TS Unified SPA + Nginx: :3000)
│   ├── outbox-relay (Go Transactional Outbox Publisher daemon — Singleton replicas: 1)
│   └── event-worker (Go RabbitMQ Consumer worker — Autoscaled via KEDA, replicas 1-3)
├── Database Release Migration (Job):
│   └── core-api-migrate-job (One-shot Job per release, backoffLimit: 0)
├── Scheduled Batch Workloads (CronJob):
│   └── swiftdrop-outbox-reconciler (CronJob: `*/10 * * * *` — Sweeps stuck outbox rows & retries)
└── Stateful Workloads (StatefulSets):
    ├── postgres (PostgreSQL 16 StatefulSet + headless Service + 10Gi PVC on `local-path`)
    ├── rabbitmq (RabbitMQ 3.13 StatefulSet + headless Service + 5Gi PVC on `local-path`)
    └── redis (Optional Ephemeral Redis for multi-replica Tracking SSE fan-out)
```

---

## 2. Directory Layout

```
deploy/
├── README.md                            # Living deployment documentation (This file)
├── .env.example                         # Optional Compose overrides → copy to deploy/.env (gitignored)
├── docker-compose.yml                   # Unified local development stack (make up)
├── gateway/                             # Edge gateway and reverse proxy (Member 7)
│   ├── README.md                        # Gateway architecture and local Docker run guide
│   └── traefik/
│       ├── traefik.yml                  # Static configuration (Cloudflare trusted IPs, entrypoints)
│       ├── dynamic-routes.yml           # Compose routing, CORS, upload limits & rate limits
│       └── kubernetes/                  # Same routes as IngressRoute + Middleware CRDs (the K8s ingress)
├── seeds/                               # Idempotent seed SQL, loaded by `make seed` after migrations
├── scripts/smoke-test.py                # Gateway smoke test: python deploy/scripts/smoke-test.py [base URL]
├── scripts/deploy.sh                    # Production deploy: render prod overlay with a SHA, apply, smoke, auto-rollback
├── scripts/sync-secrets.sh              # CD step: GitHub environment secrets -> cluster Secrets (see deployment-guide §3.0)
├── scripts/create-secrets.sh            # Creates all cluster Secrets with generated values (Git Bash on Windows)
├── cd/                                  # CD setup: deployer-rbac.yaml (least-privilege SA), install-runner.sh (VM runner)
├── observability/                       # Prometheus, Grafana, alerts (Compose profile + K8s, section 9)
├── overlays/                            # Kustomize overlays: kubectl apply -k deploy/overlays/<env>
│   ├── dev/                             # k3d: :dev images, ingress over plain HTTP
│   └── prod/                            # Azure K3s: git-SHA images, swiftdrop.work CORS, ghcr pull secret
└── kubernetes/                          # Kustomize base (Member 9 per Agreement A8)
    ├── kustomization.yaml               # Lists every manifest below + ../observability
    ├── namespace.yaml                   # Namespace definition (swiftdrop)
    ├── secrets.example.yaml             # Template for cluster secrets (not applied; create real ones with kubectl)
    ├── traefik/helmchartconfig.yaml     # K3s Traefik: Cloudflare real IPs, https redirect (Azure only, applied separately)
    ├── postgres/                        # PostgreSQL 16 StatefulSet + Service + role bootstrap Job (passwords from Secret)
    ├── rabbitmq/                        # RabbitMQ 3.13 StatefulSet + Service (metrics on 15692)
    ├── redis/                           # Ephemeral Redis Deployment & Service (port 6379)
    ├── core-api/                        # Deployment (2-4 replicas), ClusterIP Service, HPA, migrate Job (/app/migrate)
    ├── tracking-api/                    # Deployment, ClusterIP Service
    ├── messaging/                       # outbox-relay (STRICT SINGLETON), event-worker, KEDA ScaledObject,
    │                                    #   ServiceAccounts, messaging-config.yaml.example
    ├── ai-assistant/                    # Deployment, Service, ConfigMap, HPA (Member 10)
    ├── web/                             # Deployment (port 3000), ClusterIP Service
    └── reconciliation/                  # CronJob (*/10 * * * *, sweeps stale outbox rows)
```

---

## 3. Stateful Workloads: Why `StatefulSet`?

As specified in **SRS §11.2** (*"do not assume a Deployment alone makes stateful data safe"*), in-cluster PostgreSQL and RabbitMQ are deployed as **StatefulSets** rather than standard Deployments:
1. **Deterministic Identity:** Guaranteed ordinal names (`postgres-0`, `rabbitmq-0`) and permanent internal DNS hostnames (`postgres-0.postgres-headless.swiftdrop.svc.cluster.local`).
2. **Dedicated Storage via `volumeClaimTemplates`:** Bound to K3s's built-in `local-path` storage class.
3. **No Multi-Attach Deadlocks:** Prevents volume attachment race conditions when pods are rescheduled or restarted on node reboot.

---

## 4. Cloudflare + Traefik Real-IP Trust

Because Cloudflare sits in front of the cluster edge, Traefik is configured with `forwardedHeaders.trustedIPs` in `deploy/gateway/traefik/traefik.yml` covering all official Cloudflare IPv4 CIDR blocks.
- **Without trusted IPs:** Traefik would attribute all inbound traffic to Cloudflare's edge IPs, causing rate limiters to lock out all users globally.
- **With trusted IPs:** Traefik restores the true client IP from `CF-Connecting-IP` / `X-Forwarded-For`, isolating rate-limiting buckets per visitor.

---

## 5. Workload Scalability Rules & Autoscaling Profiles (HPA & KEDA)

To capture full bonus marks for **SRS §15.3** while preventing Out-Of-Memory (OOM) crashes and database connection saturation on a single-node VPS, cluster workloads adhere to strict scalability profiles:

### 5.1 Scalable Workloads
1. **Core API (HorizontalPodAutoscaler):**
   - **Metrics:** Scales between `2` and `4` replicas when CPU utilization exceeds 70%.
   - **Stateless Invariant:** Pods store zero session state in memory and zero files on local disk (shared S3/MinIO for attachments).
   - **Connection Pool Ceiling:** Per-pod DB pool is tuned such that: $\text{Replicas}_{\text{max}} \times \text{MaxOpenConns} \le \text{PostgreSQL MaxConnections}$ (e.g. $4 \times 20 = 80 \le 100$).
   - **Graceful Termination:** Listens for `SIGTERM` and allows a 20-second grace period for in-flight requests and database transactions to commit cleanly.
2. **Tracking API (HorizontalPodAutoscaler):**
   - **Metrics:** Scales from `1` &rarr; `2` (or `2` &rarr; `4`) replicas when CPU exceeds 50% or active HTTP/2 connections peak.
   - **Realtime Fan-Out Invariant:** When $>1$ replica is active, live Server-Sent Events (SSE) streams are broadcasted across pods via **Redis Pub/Sub**.
   - **Edge Proxy Safety:** Emits `X-Accel-Buffering: no` and sends 15-second keepalive pings (`: ping\n\n`) to prevent Cloudflare/Traefik stream termination.
3. **Event Worker (KEDA ScaledObject):**
   - **Metrics:** Scales from `1` &rarr; `3` replicas based on the RabbitMQ `events` queue backlog (`queueLength > 10`).
   - **Consumer Idempotency Invariant:** Every consumer MUST verify incoming event IDs against the authoritative `idempotency_keys` table before executing side-effects (SMS/email notifications, merchant webhooks). Duplicate message deliveries must be safely acknowledged without duplicate actions.
4. **Web Frontend (HorizontalPodAutoscaler):**
   - **Metrics:** Scales from `2` &rarr; `3` replicas based on traffic volume. Stateless Nginx container serving immutable hashed JS/CSS assets.

### 5.2 Non-Scalable Singletons & Stateful Systems
1. **Outbox Relay Daemon (STRICT SINGLETON — `replicas: 1`):**
   - **PROHIBITION:** The Outbox Relay MUST NEVER be scaled horizontally. A single poller guarantees transactional outbox row ordering and prevents database lock contention or duplicate publishing to RabbitMQ.
2. **PostgreSQL 16 & RabbitMQ 3.13 (`StatefulSet`):**
   - Primary database and message broker run as single-instance StatefulSets bound to dedicated `local-path` PersistentVolumeClaims. They must NEVER be scaled via standard Deployment replicas.
3. **Outbox Reconciler (`CronJob`):**
   - Scheduled periodic job (`*/10 * * * *`) configured with `concurrencyPolicy: Forbid` to prevent concurrent overlapping executions.

To capture full bonus marks for **SRS §15.3** while preventing Out-Of-Memory (OOM) crashes on a single-node VPS, autoscalers are configured with **Micro-Scaling demonstration envelopes**:
- **Tracking API (HPA):** Scales from `1` &rarr; `2` replicas when CPU utilization exceeds 50% (`autoscaling/hpa-tracking.yml`). The final range is open item C8 (Member 5); the DB budget below allows at most 4 pods, and more than 1 pod needs Redis.
- **Event Worker (KEDA):** Scales from `1` &rarr; `3` replicas when the `q.notifications.worker` or `q.webhooks.dispatcher` queue backlog exceeds 10 messages (`autoscaling/keda-event-worker.yml`).
- **Outbox Relay:** Strictly a singleton (`replicas: 1`) to eliminate row lock contention.

---

## 6. Database Governance & Connection Pooling Budget

PostgreSQL 16 is tuned with `max_connections = 150` (1Gi memory ceiling), set by `args: ["-c", "max_connections=150"]` in `postgres-statefulset.yml`.

Each workload logs in with its own role (D6). `postgres-secrets` holds one key per role, and the manifest maps that key into the `DATABASE_URL` env var the service reads: `DATABASE_URL` (core-api, `swiftdrop_app`), `TRACKING_DATABASE_URL`, `RELAY_DATABASE_URL`, `WORKER_DATABASE_URL`, `RECONCILER_DATABASE_URL`. The migrate Job uses `postgres-migrator-secrets`.

### 6.1 Role Privileges & Automated Default Permissions
During initial bootstrap (`deploy/postgres/bootstrap-roles.sql` and `deploy/kubernetes/postgres/bootstrap-job.yaml`), PostgreSQL's default privileges are configured for the schema owner `swiftdrop_migrator`:
- **`swiftdrop_app`:** Receives default `SELECT, INSERT, UPDATE` on all future tables and `USAGE, SELECT` on sequences.
- **`swiftdrop_tracking_ro`:** Receives default `SELECT` on all future tables and `USAGE, SELECT` on sequences, enforcing read-only projection queries (`default_transaction_read_only = on`) across all domain models (including `parcels` and `parcel_status_history`).

Connection budgets are strictly enforced per pod ConfigMap:
- `core-api`: 12 conns/pod $\times$ 4 replicas = 48
- `tracking-api`: 5 conns/pod $\times$ 4 replicas = 20
- `outbox-relay`: 5 conns
- `event-worker`: 10 conns/pod $\times$ 3 replicas = 30
- `reconciler`: 2 conns
- **Active Subsystem Total:** 105 connections
- **Headroom for migrations, admin & growth:** 45 connections

---

## 7. Local & Cluster Deployment Commands

### 7.0 Where configuration lives

| Running | Config source |
| :--- | :--- |
| Full stack (`make up`) | Compose defaults. To override: `cp deploy/.env.example deploy/.env` and edit it. Compose reads `.env` from `deploy/`, **not** the repo root |
| One service on its own | That service's own template, e.g. `services/ai-assistant/.env.example`, `services/messaging/.env.example` (owned by the service owner) |
| Kubernetes | ConfigMaps and Secrets only (`deploy/kubernetes/`). Kubernetes never reads `.env` files |

Every Dockerfile that copies its whole folder (`COPY . .`) needs a `.dockerignore` excluding `.env*`, so a developer's local `.env` is never baked into an image.

### 7.1 Using GNU Make (Linux / macOS / Git Bash)
```bash
# 1. Start unified local Docker Compose stack
make up

# 2. Run sequential database migrations (000001 to 000011)
make migrate-up

# 3. Seed demonstration data (refuses in production)
make seed

# 4. Check status and follow logs
make ps
make logs

# 5. Run automated test suites
make test
make test-e2e

# 6. Stop containers / reset volumes
make down
make reset
```

### 7.2 Native Windows PowerShell (No Make installation required)
```powershell
# 1. Start stack
.\scripts\make.ps1 up

# 2. Run migrations
.\scripts\make.ps1 migrate-up

# 3. Seed demo accounts
.\scripts\make.ps1 seed

# 4. Check status & logs
.\scripts\make.ps1 ps
.\scripts\make.ps1 logs

# 5. Stop stack
.\scripts\make.ps1 down
```

---

## 8. Service Port & Health Probe Inventory

| Container Name | Internal Port | Host Port | Health Check / Probe | Purpose |
| :--- | :---: | :---: | :--- | :--- |
| `swiftdrop-traefik` | 80, 8090 | `80`, `127.0.0.1:8090` | Traefik `/ping` | Edge ingress gateway & reverse proxy |
| `swiftdrop-postgres` | 5432 | `5432` | `pg_isready -U swiftdrop -d swiftdrop_db` | Authoritative PostgreSQL 16 store |
| `swiftdrop-rabbitmq` | 5672, 15672 | `5672`, `15672` | `rabbitmq-diagnostics -q ping` | AMQP 0-9-1 broker & management UI |
| `swiftdrop-redis` | 6379 | `6379` | `redis-cli ping` | Ephemeral cache & SSE pub/sub fan-out |
| `swiftdrop-minio` | 9000, 9001 | `9000`, `9001` | S3 API endpoint | Local S3 object store for proof-of-delivery |
| `swiftdrop-mailpit` | 1025, 8025 | `1025`, `8025` | Mailpit HTTP API | Transactional email & SMS development sandbox |
| `swiftdrop-core-api-migrate` | — | — | One-shot completion | Runs core-api `/app/migrate` (embedded migrations, `schema_migrations` ledger) |
| `swiftdrop-core-api` | 8080 | `8080` | `GET /health/live`, `/health/ready` | Core modular monolith |
| `swiftdrop-tracking-api` | 8081 | `8081` | `GET /health/live`, `/health/ready` | Tracking API & SSE projections service |
| `swiftdrop-outbox-relay` | 8082 | `8082` | `GET /health/live`, `/metrics` | Transactional outbox publisher (singleton) |
| `swiftdrop-event-worker` | 8083 | `8083` | `GET /health/live`, `/metrics` | Asynchronous consumer worker (KEDA scaled) |
| `swiftdrop-ai-assistant` | 8000, 8002 | `8000`, `8002` | `GET /health/live` | Python LLM assistant & FastMCP gateway |
| `swiftdrop-web` | 3000 | `3000` | `GET /healthz` | React/TypeScript SPA served by Nginx |

---

## 9. Observability (Prometheus & Grafana)

All files live in `deploy/observability/`. Compose and Kubernetes share the same alert rules (`alerts.yml`) and dashboard (`grafana/dashboards/swiftdrop-overview.json`), so there is one copy.

**Local (Compose profile, off by default):**
```bash
docker compose -f deploy/docker-compose.yml --profile observability up -d
```
- Prometheus: http://localhost:9090 (targets, alerts)
- Grafana: http://localhost:3001 (user `admin`, password `GRAFANA_ADMIN_PASSWORD` from `deploy/.env`, default `admin`). Opens on the **SwiftDrop Overview** dashboard.

**Kubernetes:** included in the base Kustomization (`deploy/observability/kustomization.yaml`). Create the Grafana secret first, then reach both with port-forward (they are never exposed by the ingress):
```bash
kubectl create secret generic grafana-admin -n swiftdrop --from-literal=GF_SECURITY_ADMIN_PASSWORD=<strong password>
kubectl port-forward -n swiftdrop svc/grafana 3001:3000
kubectl port-forward -n swiftdrop svc/prometheus 9090
```
Prometheus discovers pods with `prometheus.io/scrape: "true"` plus `prometheus.io/port` and `prometheus.io/path` annotations.

| Target | Metrics | Status |
| :--- | :--- | :--- |
| outbox-relay `:8082`, event-worker `:8083` | Outbox backlog and age, publish rate and failures, DLQ depth (`swiftdrop_*`) | Scraped |
| RabbitMQ `:15692` | Queue depth, connections (`rabbitmq_prometheus` plugin) | Scraped |
| Postgres (via `postgres-exporter :9187`) | Connections against the 150 budget, database stats | Scraped |
| core-api, tracking-api, ai-assistant | None yet: `/metrics` returns 404 (ai-assistant returns JSON) | Shown as DOWN until their owners add a Prometheus endpoint |

**Alerts** (`alerts.yml`): `ServiceDown` (2 min), `OutboxBacklogStale` (oldest pending event > 5 min), `OutboxPublishFailures`, `DeadLetterQueueNotEmpty`, `PostgresConnectionsHigh` (> 80% of `max_connections`).
