# Document 8 — Implementation, Deployment & Operations Guide

> **Status:** Draft / Proposed (For Review in Team Meeting 2)  
> **Author:** Member 9 — DevOps, Kubernetes & Observability + Database Coordinator (Minidu Dhananjana)  
> **Target Module / Location:** `deploy/` (Docker Compose, Kubernetes, Helm, monitoring), `.github/` (CI/CD), root `Makefile`; database coordination of `core-api/migrations/`  
> **Key Upstream Dependencies:** every service owner (Dockerfile, health endpoints, env vars), Member 7 (gateway routes), Member 8 (queues, reconciler), Member 2 (seed, migrations)  
> **Open Questions / Needs Clarification:** Compose healthcheck for distroless images (service owners); tracing backend (none planned yet); exact pinned versions of base images, K3s, KEDA and charts.  
> **Answers:** How does another developer run, debug, configure and maintain this module?  
> **Note:** most files here are **planned** (Stage 1 has not started building yet). Commands show the agreed design (C3, C22). When the files exist, this content moves into `deploy/README.md` and `docs/operations/*`.  

---

## 1. Tools, frameworks and versions

| Tool | Version | Use |
| :--- | :--- | :--- |
| Docker Engine + Compose v2 | 24+ | images, local stack |
| GNU make | any | root `Makefile` (C22) |
| K3s (VPS) | pin with `INSTALL_K3S_VERSION` at install | production cluster (ADR-002) |
| k3d | 5.x | local K3s in Docker |
| kubectl | 1.29+ | cluster access |
| Helm | 3.x | Traefik, KEDA, monitoring, our chart |
| Traefik | v3.1 (`deploy/gateway/README.md`) | gateway / ingress |
| KEDA | 2.x (pin on install) | event-worker scaling |
| metrics-server | bundled with K3s | HPA |
| PostgreSQL | `postgres:16-alpine` ✅ (pin the exact patch before submission) | database |
| RabbitMQ | `rabbitmq:3.13-management-alpine` ✅ (pin the exact patch) | broker |
| Redis | `redis:7-alpine` (optional) | SSE fan-out |
| golang-migrate | v4.17+ | migrations |
| Prometheus + Grafana | kube-prometheus-stack via Helm (proposed; chart version pinned) | observability |
| GitHub Actions | `actions/checkout@v4`, `actions/setup-go@v5` ✅ (pin the others) | CI/CD |
| Linters | hadolint, kubeconform, yamllint, trivy | quality and security checks |

## 2. Folder structure and entry points

The full tree is in Doc 3 §2. **Entry points:**

| Entry point | Purpose |
| :--- | :--- |
| `Makefile` | everything a developer runs locally |
| `deploy/docker-compose.yml` | the local stack |
| `deploy/kubernetes/` (later `deploy/helm/swiftdrop/`) | the cluster |
| `deploy/gateway/traefik/` | gateway config (shared with M7) |
| `.github/workflows/ci.yml` ✅, `deploy.yml` (planned) | CI/CD |
| `scripts/verify.sh` / `verify.ps1` ✅ | local pre-PR checks |
| `docs/operations/deployment-guide.md`, `runbook.md`, `environment-variables.md` ✅ | operations docs |

## 3. Environment variables (platform level)

These are the variables the **platform itself** needs. Each service's variables are listed in `docs/operations/environment-variables.md` (I own that file). All values below are **safe placeholders**.

| Variable | Used by | Example | Secret? |
| :--- | :--- | :--- | :-: |
| `POSTGRES_USER` | postgres container | `swiftdrop` | no |
| `POSTGRES_PASSWORD` | postgres container | `<change-me-local-only>` | **yes** |
| `POSTGRES_DB` | postgres container | `swiftdrop_db` | no |
| `RABBITMQ_DEFAULT_USER` | rabbitmq container | `swiftdrop` | no |
| `RABBITMQ_DEFAULT_PASS` | rabbitmq container | `<change-me-local-only>` | **yes** |
| `DATABASE_URL` | migrate service, apps | `postgres://swiftdrop:<password>@postgres:5432/swiftdrop_db?sslmode=disable` | **yes** |
| `IMAGE_TAG` *(proposed)* | Compose / deploy | `dev` locally, git SHA in CI | no |
| `REGISTRY` *(proposed)* | CI / Helm | `ghcr.io/<org-lowercase>/swiftdrop` | no |
| `KUBECONFIG` | your shell only | path to your kubeconfig | **yes** (never commit) |

## 4. Local run, build and test

**First time:**

```bash
git clone https://github.com/phantomTroupeEAD/SwiftDrop.git
cd SwiftDrop
cp .env.example .env          # then put local-only values in .env
make up                       # start the stack (Compose in deploy/)
make migrate-up               # usually automatic: the migrate one-shot runs before core-api
make seed                     # demo data
```

**Every day:**

| Task | Command |
| :--- | :--- |
| Start / stop | `make up` / `make down` *(proposed)* |
| Clean everything | `make reset` *(proposed)*: removes the volumes, so data is lost |
| See status | `docker compose -f deploy/docker-compose.yml ps` |
| Logs of one service | `docker compose -f deploy/docker-compose.yml logs -f core-api` |
| Run one service natively instead | stop its container, then `go run ...` (see `local-development.md` §3) |
| End-to-end tests | `make test-e2e` |
| Pre-PR checks | `./scripts/verify.sh` (Windows: `.\scripts\verify.ps1`) |

**Local Kubernetes (k3d):**

```bash
k3d cluster create swiftdrop-dev --k3s-arg "--disable=traefik@server:0"
docker build -t swiftdrop/core-api:dev core-api/          # when Dockerfiles exist
k3d image import swiftdrop/core-api:dev -c swiftdrop-dev  # use local images without a registry
kubectl apply -f deploy/kubernetes/namespace.yml
kubectl apply -f deploy/kubernetes/ -n swiftdrop
kubectl get pods -n swiftdrop -w
```

## 5. Docker build and health-check requirements

1. Multi-stage build; small pinned base image; `USER 10001:10001`.
2. No secrets in the image; add a `.dockerignore`.
3. `EXPOSE` the contract port: core-api 8080, tracking 8081, ai 8000, web 3000.
4. **`/health/live`, `/health/ready` and `/metrics`** on every HTTP service (C1; Doc 4 §2).
5. Handle SIGTERM within the grace period.
6. JSON logs to stdout with `request_id`.
7. **Compose healthcheck:** prefer a check that needs no extra tools in the image. For distroless Go images there is no shell or curl, so either rely on K8s probes, or add a tiny `healthcheck` subcommand to the binary (owners decide; TBD).

## 6. Kubernetes workloads, Services, probes, resources and scaling

| Workload | Kind | Service (port) | Probes | Requests / limits | Scaling |
| :--- | :--- | :--- | :--- | :--- | :--- |
| core-api | Deployment | ClusterIP 8080 | startup + ready + live (Doc 4 §2) | TBD | HPA 2–4 @ CPU 70% |
| tracking-api | Deployment | ClusterIP 8081 | same | TBD | HPA, TBD (C8) |
| ai-assistant | Deployment | ClusterIP 8000 | same | TBD | 1 replica |
| web | Deployment | ClusterIP 3000 | HTTP `/` | TBD | HPA 2–3 @ CPU 80% |
| outbox-relay | Deployment | internal health port TBD | same | TBD | **always 1** |
| event-worker | Deployment | internal health port TBD | same | TBD | KEDA 1–3 (C7) |
| postgres | StatefulSet ✅ | `postgres` + headless ✅ | `pg_isready` ✅ (fix user, C14) | 100m/256Mi → 500m/512Mi ✅ | never |
| rabbitmq | StatefulSet ✅ | `rabbitmq` + headless ✅ | `rabbitmq-diagnostics` ✅ | 100m/256Mi → 500m/512Mi ✅ | never |
| redis *(if needed)* | Deployment | ClusterIP 6379 | `redis-cli ping` | small | 1 |
| migrate | Job | — | — | small | once per deploy |
| reconciler | CronJob ✅ | — | — | 50m/64Mi → 200m/128Mi ✅ | `*/10`, Forbid ✅ |
| traefik | Deployment (Helm) | LoadBalancer/NodePort 80/443 | `/ping` | TBD | 1–2 |

**Deploy order (K8s):**
1. namespace;
2. Secrets and ConfigMaps;
3. postgres and rabbitmq (wait for Ready);
4. migrate Job (wait for Complete);
5. apps;
6. Traefik routes;
7. HPA/KEDA.

This follows `deployment-guide.md` §3 with the decided names.

## 7. Logging, metrics, tracing and alerts

**Logging:** JSON lines to stdout (runbook §2). Search across pods with
`kubectl logs -n swiftdrop -l app=core-api | grep <request_id>`.

**Metrics:**
- Prometheus scrapes `/metrics` from every service (names in Doc 4 §5);
- RabbitMQ is scraped through its Prometheus plugin;
- pods and nodes through kube-prometheus-stack.

**Grafana dashboard "SwiftDrop Overview" (proposed panels):**

| Panel | Query idea | Target |
| :--- | :--- | :--- |
| Request rate per service | `sum(rate(http_requests_total[5m])) by (service)` | — |
| p95 latency | `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service))` | ≤ 500 ms (NFR-PERF-001) |
| Error rate (5xx) | `sum(rate(http_requests_total{status=~"5.."}[5m])) by (service)` | ~0 |
| Outbox backlog / oldest age | `outbox_pending_events`, `outbox_oldest_pending_age_seconds` | publish ≤ 5 s (NFR-PERF-003) |
| Queue depth / DLQ | RabbitMQ plugin metrics | DLQ = 0 |
| Worker failures, webhook outcomes | `worker_consumer_failures_total`, `webhook_deliveries_total` | — |
| SSE connections | `sse_active_connections` | — |
| DB connections | `db_pool_open_connections` + Postgres total | < 100 (C11) |
| Pod restarts, CPU, memory | kube metrics | — |

**Tracing:** `OTEL_EXPORTER_OTLP_ENDPOINT` exists in `environment-variables.md` (pointing at `tempo:4317`), but **no tracing backend is planned yet**, so treat it as optional. Correlation through `request_id` / `correlation_id` in logs is the baseline (runbook §2).

**Alerts (proposed):**
- a pod restarting repeatedly;
- a Deployment not Available;
- outbox oldest pending > 5 min;
- DLQ > 0;
- disk > 80% (ADR-002 §9);
- 5xx rate high;
- p95 > 500 ms;
- DB connections > 90.

## 8. Troubleshooting

| Problem | Likely cause | Fix |
| :--- | :--- | :--- |
| `port is already allocated` | local Postgres or RabbitMQ, or 8080 clash (C16) | change the host port in Compose/.env; stop the local service |
| core-api never starts in Compose | migrate failed | `docker compose logs core-api-migrate`; fix the SQL; `force` if dirty |
| `dirty database version N` | migration failed half-way | fix it, then `migrate ... force <last good>`, then `up` |
| Pod `CrashLoopBackOff` | bad config or a crash at start | `kubectl logs <pod> --previous`; check env and Secrets |
| Pod `Running` but not Ready | dependency down or wrong ready path | `kubectl describe pod`; check `/health/ready` |
| `ImagePullBackOff` | wrong tag or no GHCR access | check the tag; for local images use `k3d image import` |
| HPA shows `<unknown>` | no metrics yet or no CPU requests | wait about a minute; add `resources.requests.cpu` |
| KEDA not scaling | wrong queue name (C7) or missing secret key (C6) | `kubectl describe scaledobject -n swiftdrop` |
| `too many clients` from Postgres | pool budget exceeded | runbook Playbook A; lower the pool sizes (Doc 5 §5) |
| PVC `Pending` | no `local-path` StorageClass | `kubectl get storageclass` (K3s/k3d has it by default) |
| Gateway returns 404 | route rule or service name mismatch | Traefik dashboard via port-forward; check names against Doc 4 §7 |
| SSE disconnects after ~100s | buffering, no heartbeat | tracking must send `X-Accel-Buffering: no` + 15s ping (ADR-002 §5.2) |

## 9. External dependencies, startup/shutdown and known limitations

**External dependencies:**
- GitHub (repo, Actions, GHCR);
- Docker Hub (base images);
- Cloudflare (M4);
- the VPS provider (TBD);
- off-site backup storage (TBD).

**Startup:** Postgres and RabbitMQ healthy → migrations → apps → gateway. **Shutdown:** SIGTERM → apps drain (core-api 20s) → stop. Stateful pods stop last.

**Known limitations (honest list for the demo):**
1. **Single VPS** = single point of failure (ADR-002 §9); no HA database or broker.
2. `local-path` volumes live on one disk; backups are the only protection.
3. Traefik rate limits are **per replica, in memory** (ADR-001 §3).
4. No distributed tracing backend (logs correlation only).
5. Autoscaling ranges are small, to fit one VPS.
6. Optional parts (Helm chart, KEDA, the full monitoring stack) are claimed only once they are demonstrated (Work Division M9 acceptance 5).

## 10. Deliverables checklist (template Document 8)

- [ ] `deploy/README.md` updated: architecture, commands, troubleshooting (living doc)
- [ ] Root `.env.example` with safe placeholders
- [ ] Operational notes: `runbook.md`, `deployment-guide.md` updated with the decided names (C1, C3, C4, C12)
- [ ] A **demonstrable local setup**: `make up && make migrate-up && make seed` works from a clean clone (PLT-010)
