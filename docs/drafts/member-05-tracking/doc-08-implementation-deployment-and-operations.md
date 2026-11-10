# Document 8 — Implementation, Deployment & Operations Guide

**Project:** SwiftDrop — Parcel Delivery & Courier Management System  
**Module:** Tracking API & Real-Time Updates  
**Module Owner:** Member 5 (Senuji)  
**Version:** 1.0.0 — Post-Audit Harmonized  
**Status:** Approved & Ratified  
**Primary Technology:** Go 1.22+, Docker, Kubernetes, Traefik, PostgreSQL, Redis, RabbitMQ  
**Target Codepath:** `services/tracking/` (Port `:8081`)  
**Related Documents:** Documents 1–7, `docs/operations/deployment-guide.md`, `docs/operations/environment-variables.md`, Member 9 Agreements (A1, A2, A5, D6, D8)

---

## 1. Module Overview & Deployment Topology

The Tracking API is packaged and deployed as an independent, lightweight Go container service:
- **Service Name:** `tracking-api`
- **Source Repository Path:** `services/tracking/` (Agreement A1)
- **Container Port:** `:8081` (Agreement A1)
- **Execution User:** `USER 10001:10001` (Non-root security context)
- **Kubernetes Deployment:** Autoscaling stateless Deployment (HPA 2 to 4 pods)
- **Ingress Gateway:** Traefik routing `/api/v1/tracking/*` $\to$ `tracking-api:8081` with unbuffered streaming

```mermaid
flowchart TD
    Client[Web Browser / React Client]
    Gateway[Traefik Ingress Controller<br/>port 80/443]

    subgraph K8sCluster ["Kubernetes Cluster (Namespace: swiftdrop)"]
        subgraph HPARange ["Tracking Service (HPA 2–4 Replicas)"]
            Pod1["tracking-api Pod 1<br/>(:8081)"]
            Pod2["tracking-api Pod 2<br/>(:8081)"]
        end

        PG[(PostgreSQL 16<br/>Role: swiftdrop_tracking_ro<br/>Pool: 5 conns/pod)]
        Redis[(Redis Cluster<br/>Channel: tracking:updates)]
        RabbitMQ[(RabbitMQ Broker<br/>Queue: q.tracking.updates)]
    end

    Client -->|HTTPS / SSE| Gateway
    Gateway -->|Round-Robin /stream| Pod1
    Gateway -->|Round-Robin /public| Pod2
    Pod1 & Pod2 -->|Read-Only SELECT| PG
    Pod1 & Pod2 <-->|Pub/Sub Fan-Out| Redis
    RabbitMQ -->|q.tracking.updates| Pod1
    RabbitMQ -->|q.tracking.updates| Pod2
```

---

## 2. Directory Structure & Source Code Layout

```text
services/tracking/
├── cmd/
│   └── tracking/
│       └── main.go                 # Entry point, dependency wiring & signal trapping
├── internal/
│   ├── config/
│   │   └── config.go               # Validated env loader
│   ├── domain/
│   │   ├── tracking.go             # Domain entities and milestone definitions
│   │   └── errors.go               # Domain errors
│   ├── service/
│   │   ├── tracking_service.go     # Application service use cases
│   │   └── dto.go                  # Request & response DTOs ({data} envelope)
│   ├── ports/
│   │   └── repository.go           # TrackingRepository interface
│   ├── repository/
│   │   └── postgres.go             # pgx/v5 read-only queries (swiftdrop_tracking_ro)
│   ├── events/
│   │   ├── hub.go                  # In-memory SSE client hub (32-slot buffer)
│   │   ├── rabbit_consumer.go      # RabbitMQ subscriber for q.tracking.updates
│   │   └── redis_pubsub.go         # Redis publisher/subscriber for tracking:updates
│   └── handler/
│       ├── rest.go                 # REST HTTP endpoints
│       ├── sse.go                  # SSE streaming handler with 15s keepalive
│       ├── health.go               # Liveness (/health/live) & Readiness (/health/ready)
│       └── middleware.go           # JWT RS256 validation & CORS
├── Dockerfile                      # Production multi-stage Docker build
├── go.mod                          # Go module dependencies
├── go.sum                          # Go checksums
└── README.md                       # Service documentation & run instructions
```

> **Note on Migrations:** Member 5 owns **zero database migrations**. All database tables are maintained centrally in `core-api/migrations/` and deployed by Member 9.

---

## 3. Environment Variables Specification

The service loads configuration from environment variables (Agreement A5):

| Variable Name | Required | Default Value | Description |
|---|:---:|---|---|
| `PORT` | No | `8081` | TCP port for the HTTP/SSE server (Agreement A1). |
| `DATABASE_URL` / `TRACKING_DATABASE_URL` | **Yes** | — | PostgreSQL connection string for role `swiftdrop_tracking_ro`. In Kubernetes, sourced from `postgres-secrets` under key `TRACKING_DATABASE_URL` (Agreement A5, D6). Uses `sslmode=disable` in-cluster per Agreement D11. |
| `DATABASE_MAX_OPEN_CONNS` | No | `5` | Maximum database connections per pod (Agreement D8: 5 conns × max 4 pods = 20 conns max). |
| `DATABASE_MAX_IDLE_CONNS` | No | `2` | Maximum idle connections per pod. |
| `DATABASE_STATEMENT_TIMEOUT` | No | `3000ms` | Hard SQL query timeout (3s). |
| `REDIS_URL` | **Yes** | `redis://redis:6379/0` | Redis connection URL for multi-replica SSE fan-out on channel `tracking:updates` (mandatory when replicas ≥ 2). |
| `RABBITMQ_URL` | **Yes** | `amqp://swiftdrop:swiftdrop123@rabbitmq:5672/` | RabbitMQ connection string for `q.tracking.updates` (sourced from `rabbitmq-secrets`). |
| `JWT_PUBLIC_KEY_PEM` | **Yes** | — | RSA public key for validating Bearer tokens on protected history endpoints (from `jwt-secrets`). |
| `LOG_LEVEL` | No | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`). |
| `KEEPALIVE_INTERVAL` | No | `15s` | SSE heartbeat interval (Agreement A2, ADR-002 §5.2). |

---

## 4. Production Multi-Stage Dockerfile

```dockerfile
# Build Stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /bin/tracking-api ./cmd/tracking

# Production Runtime Stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

# Security: Non-root user (10001:10001)
RUN addgroup -g 10001 swiftdrop && \
    adduser -u 10001 -G swiftdrop -D -H -s /sbin/nologin swiftdrop

USER 10001:10001

COPY --from=builder /bin/tracking-api /usr/local/bin/tracking-api

EXPOSE 8081

# Liveness probe on container level
HEALTHCHECK --interval=15s --timeout=3s --retries=3 \
    CMD wget -qO- http://localhost:8081/health/live || exit 1

ENTRYPOINT ["/usr/local/bin/tracking-api"]
```

---

## 5. Kubernetes Deployment & Autoscaling Manifests

### 5.1 Deployment Specification (`deploy/kubernetes/tracking-api/deployment.yaml`)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tracking-api
  namespace: swiftdrop
  labels:
    app: tracking-api
spec:
  replicas: 2
  selector:
    matchLabels:
      app: tracking-api
  template:
    metadata:
      labels:
        app: tracking-api
    spec:
      containers:
        - name: tracking-api
          image: ghcr.io/phantomtroupeead/swiftdrop/tracking-api:<git-sha>
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 8081
              name: http
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: postgres-secrets
                  key: TRACKING_DATABASE_URL
            - name: TRACKING_DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: postgres-secrets
                  key: TRACKING_DATABASE_URL
            - name: RABBITMQ_URL
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secrets
                  key: RABBITMQ_URL
            - name: JWT_PUBLIC_KEY_PEM
              valueFrom:
                secretKeyRef:
                  name: jwt-secrets
                  key: JWT_PUBLIC_KEY
            - name: REDIS_URL
              value: "redis://redis:6379/0"
          envFrom:
            - configMapRef:
                name: tracking-api-config
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "500m"
              memory: "256Mi"
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 5
            timeoutSeconds: 3
```

### 5.2 Horizontal Pod Autoscaler (`deploy/kubernetes/autoscaling/hpa-tracking.yml` / Conflict C8)

To support dynamic traffic spikes while adhering to PostgreSQL connection limits (**Agreement D8**):
- **Database Budget Invariant:** The database pool budget for `swiftdrop_tracking_ro` is capped at $5 \text{ connections/pod} \times 4 \text{ pods} = \mathbf{20 \text{ connections}}$. Therefore, `maxReplicas` must not exceed **`4`**.
- **Redis Multi-Pod Requirement:** For any deployment with $> 1$ replica, an active Redis instance (`redis:6379`) is **mandatory** for Server-Sent Events fan-out on channel `tracking:updates`.
- **HPA Envelopes:**
  - **High-Availability Production:** `minReplicas: 2`, `maxReplicas: 4` (70% CPU target).
  - **Single-Node VPS Demonstration:** `minReplicas: 1`, `maxReplicas: 4` (or `2`), 50%–70% CPU target.

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: tracking-api-hpa
  namespace: swiftdrop
  labels:
    app.kubernetes.io/name: tracking-api
    app.kubernetes.io/part-of: swiftdrop
    app.kubernetes.io/component: autoscaler
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: tracking-api
  minReplicas: 1 # Set to 2 for HA production; 1 for VPS demonstration baseline
  maxReplicas: 4 # Strictly capped at 4 pods to respect DB budget (5 conns × 4 = 20 max)
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 15
      policies:
        - type: Percent
          value: 100
          periodSeconds: 15
    scaleDown:
      stabilizationWindowSeconds: 60
      policies:
        - type: Percent
          value: 100
          periodSeconds: 60
```

---

## 6. Traefik Ingress & Streaming Configuration (Agreement A9)

Traefik routes tracking traffic and disables response buffering for SSE:

```yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: tracking-api-ingress
  namespace: swiftdrop
spec:
  entryPoints:
    - websecure
  routes:
    - match: PathPrefix(`/api/v1/tracking`)
      kind: Rule
      services:
        - name: tracking-api
          port: 8081
      middlewares:
        - name: tracking-ratelimit
        - name: sse-unbuffered

---
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: sse-unbuffered
  namespace: swiftdrop
spec:
  buffering:
    maxResponseBodyBytes: 0 # Disables buffering; immediate flush for SSE
```

---

## 7. Local Development & Operational Runbook

### 7.1 Running Locally with Docker Compose

Ensure the shared infrastructure is active:
```bash
# 1. Start core dependencies
docker compose up -d postgres redis rabbitmq

# 2. Export environment variables
export PORT=8081
export DATABASE_URL="postgres://swiftdrop_tracking_ro:trackingpass@localhost:5432/swiftdrop_db?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
export RABBITMQ_URL="amqp://swiftdrop:swiftdrop123@localhost:5672/"
export JWT_PUBLIC_KEY_PEM="$(cat ../../certs/jwt_public.pem)"

# 3. Run the service
cd services/tracking
go run ./cmd/tracking
```

### 7.2 Verifying Service Health & Streaming

```bash
# 1. Check Liveness
curl -i http://localhost:8081/health/live

# 2. Check Readiness (verifies PostgreSQL and Redis connectivity)
curl -i http://localhost:8081/health/ready

# 3. Query Public Tracking Lookup
curl -i http://localhost:8081/api/v1/tracking/public/SD-20261025-A8F3K2

# 4. Stream Real-Time SSE Updates
curl -N -i http://localhost:8081/api/v1/tracking/public/SD-20261025-A8F3K2/stream
```

---

## 8. Operational Troubleshooting & Runbook

| Alert / Symptom | Probable Cause | Action Runbook |
|---|---|---|
| **`/health/ready` returns 503** | PostgreSQL unreachable or role `swiftdrop_tracking_ro` password mismatch. | 1. Check PostgreSQL pod status (`kubectl get pods -l app=postgres`).<br/>2. Test login with `psql -U swiftdrop_tracking_ro -d swiftdrop_db`.<br/>3. Verify secret `postgres-secrets` (key `TRACKING_DATABASE_URL`). |
| **SSE stream drops after 100s** | Missing keepalive frames or gateway idle timeout. | Verify server is emitting `:keepalive\n\n` every 15s. Ensure Traefik `maxResponseBodyBytes: 0` is set. |
| **SSE updates not received on client** | RabbitMQ consumer stalled or Redis Pub/Sub disconnected. | 1. Inspect queue `q.tracking.updates` in RabbitMQ Management UI.<br/>2. Verify Redis channel `tracking:updates` using `redis-cli monitor`.<br/>3. Check tracking-api pod logs for `Failed to unmarshal tracking update`. |
| **High p95 Latency (>200ms)** | Missing database index on `parcels(tracking_number)`. | Execute `EXPLAIN ANALYZE SELECT ... FROM parcels WHERE tracking_number = '...'`. Verify index `idx_parcels_tracking_number` is being used. |
| **`connection pool exhausted`** | Leak of `sql.Rows` without `Close()`. | Review repository queries; ensure all `rows.Close()` calls are deferred immediately after `err == nil`. |

---
*End of Document 8 — Implementation, Deployment & Operations Guide (v1.0.0)*