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
├── docker-compose.yml                   # Unified local development stack (make up)
├── gateway/                             # Edge gateway and reverse proxy (Member 7)
│   ├── README.md                        # Gateway architecture and local Docker run guide
│   └── traefik/
│       ├── traefik.yml                  # Static configuration (Cloudflare trusted IPs, entrypoints)
│       └── dynamic-routes.yml           # Dynamic routing, CORS, upload limits & rate limits
└── kubernetes/                          # Kubernetes resource manifests (Member 9)
    │                                    # Target layout (A8). Today the files are still flat:
    │                                    # postgres-statefulset.yml, rabbitmq-statefulset.yml,
    │                                    # cronjob-reconciliation.yml, autoscaling/*.yml (moved in one M9 PR)
    ├── namespace.yaml                   # Namespace definition (swiftdrop)
    ├── postgres/                        # PostgreSQL 16 StatefulSet, Service, PVC
    ├── rabbitmq/                        # RabbitMQ 3.13 StatefulSet, Service, PVC
    ├── redis/                           # Ephemeral Redis StatefulSet / Deployment
    ├── core-api/                        # Deployment, Service, ConfigMap, migrate-job.yaml
    ├── tracking-api/                    # Deployment, Service, hpa.yaml
    ├── messaging/                       # Relay Deployment, Worker Deployment, keda-scaledobject.yaml
    ├── ai-assistant/                    # Deployment, Service, ConfigMap
    ├── web/                             # Deployment, Service (Port 3000)
    └── monitoring/                      # Prometheus & Grafana manifests
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

## 5. Micro-Scaling Profile (HPA & KEDA)

To capture full bonus marks for **SRS §15.3** while preventing Out-Of-Memory (OOM) crashes on a single-node VPS, autoscalers are configured with **Micro-Scaling demonstration envelopes**:
- **Tracking API (HPA):** Scales from `1` &rarr; `2` replicas when CPU utilization exceeds 50% (`autoscaling/hpa-tracking.yml`). The final range is open item C8 (Member 5); the DB budget below allows at most 4 pods, and more than 1 pod needs Redis.
- **Event Worker (KEDA):** Scales from `1` &rarr; `3` replicas when the `q.notifications.worker` or `q.webhooks.dispatcher` queue backlog exceeds 10 messages (`autoscaling/keda-event-worker.yml`).
- **Outbox Relay:** Strictly a singleton (`replicas: 1`) to eliminate row lock contention.

---

## 6. Database Governance & Connection Pooling Budget

PostgreSQL 16 is tuned with `max_connections = 150` (1Gi memory ceiling), set by `args: ["-c", "max_connections=150"]` in `postgres-statefulset.yml`.

Each workload logs in with its own role (D6). `postgres-secrets` holds one key per role, and the manifest maps that key into the `DATABASE_URL` env var the service reads: `DATABASE_URL` (core-api, `swiftdrop_app`), `TRACKING_DATABASE_URL`, `RELAY_DATABASE_URL`, `WORKER_DATABASE_URL`, `RECONCILER_DATABASE_URL`. The migrate Job uses `postgres-migrator-secrets`.

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

```bash
# 1. Start unified local Docker Compose stack
make up

# 2. Run sequential database migrations (000001 to 000011)
make migrate-up

# 3. Seed demonstration data (refuses in production)
make seed

# 4. Run automated test suites
make test
make test-e2e
```
