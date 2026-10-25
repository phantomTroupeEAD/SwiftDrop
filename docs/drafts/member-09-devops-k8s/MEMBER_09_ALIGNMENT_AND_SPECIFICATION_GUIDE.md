# Member 09 — DevOps, Kubernetes & Database Coordinator Blueprint

> **Author / Domain Owner:** Member 9 — DevOps, Kubernetes & Observability (Minidu Dhananjana)  
> **Target Directories:** `deploy/`, `.github/`, root `Makefile`, `core-api/migrations/`  
> **Status:** Authoritative Architectural Alignment & Action Sheet for Team Meeting 2  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Domain Overview & Scope of Work

Member 9 serves as the platform architect and database coordinator for SwiftDrop. Responsibilities span the entire operational lifecycle: **Local Development Orchestration (Docker Compose)**, **Production Kubernetes Manifests (K3s)**, **Continuous Integration & Delivery (GitHub Actions)**, **Prometheus/Grafana Observability**, and **Database Migration Governance**.

### 1.1 In-Scope Responsibilities
- Orchestrating the unified local stack via `deploy/docker-compose.yml` and root `Makefile`.
- Authoring and maintaining Kubernetes manifests under `deploy/kubernetes/<service>/`.
- Kubernetes stateful storage: PostgreSQL 16 StatefulSet and RabbitMQ StatefulSet.
- Ingress Gateway integration (Traefik v3) and TLS termination.
- Autoscaling implementations: KEDA (RabbitMQ worker) and HPA (Tracking API).
- Container image build pipelines (GitHub Packages `ghcr.io/phantomtroupeead/swiftdrop/*`).
- System-wide Prometheus scraping, Grafana dashboards, and Alertmanager rules.
- Reviewing, sequencing, and approving all database migrations in `core-api/migrations/`.

### 1.2 Out-of-Scope Responsibilities
- Authoring business logic, SQL queries, or HTTP endpoints for internal Go modules.
- Writing feature unit tests (Each capability owner authors their own unit tests).
- Managing AI prompt engineering or LLM model fine-tuning (Owned by Member 10).

---

## 2. Ratification of Team Agreements (A1–A13 and D1–D12)

The 25 team agreements authored by Member 9 are hereby formally ratified as binding system law:

### 2.1 Ratified Universal Agreements Summary (A1–A13)
- **A1 (Ports):** `core-api` **8080**, `tracking-api` **8081**, `outbox-relay` **8082**, `event-worker` **8083**, `ai-assistant` **8000**, `web` **3000**, Traefik Dashboard **8090**.
- **A2 (Probes):** Strict standard paths `/health/live` (process responsiveness), `/health/ready` (critical infra dependencies only), and `/metrics`.
- **A3 (Images):** Multi-stage builds, non-root `USER 10001:10001`, pinned tags (`<git-sha>`), no `:latest`.
- **A5 (Secrets):** Standard names `postgres-secrets`, `postgres-migrator-secrets`, `rabbitmq-secrets`, `jwt-secrets`, `integrations-secrets`.
- **A7 (Compose):** Single file `deploy/docker-compose.yml`, run via root `Makefile`.
- **A8 (Kubernetes Layout):** Folder-per-service (`deploy/kubernetes/<service>/`) using `.yaml` extensions.
- **A9 (Gateway):** Path-based routing on single domain via Traefik.
- **A10 & A11 (Telemetry):** Header `X-Request-ID`, metrics prefix `swiftdrop_*`.

### 2.2 Ratified Database Agreements Summary (D1–D12)
- **D1 & D5:** Single shared database `swiftdrop_db`, PostgreSQL 16+, native UUIDs, UTC `timestamptz`.
- **D2 (Migration Naming):** Unanimously adopted **6-digit prefix format**: `000NNN_name.up.sql`.
- **D3 (Migration Sequence):** Strict 11-step sequence to guarantee foreign key integrity.
- **D6 (Database Roles):** Strict least-privilege roles (`swiftdrop_migrator`, `swiftdrop_app`, `swiftdrop_tracking_ro`, `swiftdrop_relay`, `swiftdrop_worker`).
- **D7 (Currency):** Integer minor units (`bigint` cents in `LKR`). Zero floating-point types.
- **D11 (SSL Mode):** `sslmode=disable` in-cluster private pod networking.
- **D12 (Seeds):** Seeds are managed via `make seed` and reside outside migration folders.

---

## 3. Kubernetes Deployment Architecture & Manifest Layout

Manifests are organized strictly by service under `deploy/kubernetes/`:

```
deploy/kubernetes/
├── namespace.yaml
├── postgres/
│   ├── statefulset.yaml
│   ├── service.yaml
│   └── pvc.yaml
├── rabbitmq/
│   ├── statefulset.yaml
│   └── service.yaml
├── core-api/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   └── migrate-job.yaml
├── tracking-api/
│   ├── deployment.yaml
│   ├── service.yaml
│   └── hpa.yaml
├── messaging/
│   ├── deployment-relay.yaml
│   ├── deployment-worker.yaml
│   ├── cronjob-reconcile.yaml
│   └── keda-scaledobject.yaml
├── ai-assistant/
│   ├── deployment.yaml
│   └── service.yaml
├── web/
│   ├── deployment.yaml
│   └── service.yaml
└── monitoring/
    ├── prometheus.yaml
    └── grafana.yaml
```

---

## 4. Root Makefile Command Orchestration

Member 9 provides the developer automation interface via the root `Makefile`:

```makefile
.PHONY: up down migrate-up migrate-down seed test test-e2e build

up:
	docker compose -f deploy/docker-compose.yml up -d

down:
	docker compose -f deploy/docker-compose.yml down -v

migrate-up:
	docker compose -f deploy/docker-compose.yml --profile migration run --rm migrator up

seed:
	docker compose -f deploy/docker-compose.yml exec core-api /app/bin/seed

test:
	go test -v -race ./core-api/... ./services/tracking/... ./services/messaging/...

test-e2e:
	npx playwright test --config=tests/e2e/playwright.config.ts

build:
	docker compose -f deploy/docker-compose.yml build
```

---

## 5. Continuous Integration (CI) Workflow (`.github/workflows/ci.yml`)

The GitHub Actions pipeline enforces quality gates before any pull request can merge:
1. **Linting:** Runs `golangci-lint` with `depguard` on Go code, ESLint on React web, and Ruff on Python AI service.
2. **Unit Tests:** Executes Go, TypeScript, and Python unit tests with race detection.
3. **Migration Verification:** Spins up ephemeral PostgreSQL, applies all 11 migrations sequentially, and verifies clean rollback (`down.sql`).
4. **Container Build:** Verifies multi-stage Docker builds pass for all 5 images.
5. **E2E Smoke Tests:** Launches Compose stack in CI and runs Member 10's Playwright test suite.

---
*End of Member 09 Specification & Blueprint Guide.*
