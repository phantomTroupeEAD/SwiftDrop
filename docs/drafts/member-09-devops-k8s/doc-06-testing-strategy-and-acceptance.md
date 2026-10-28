# Document 6 — Testing Strategy & Acceptance Specification

> **Status:** Draft / Proposed (For Review in Team Meeting 2)  
> **Author:** Member 9 — DevOps, Kubernetes & Observability + Database Coordinator (Minidu Dhananjana)  
> **Target Module / Location:** `deploy/` (Docker Compose, Kubernetes, Helm, monitoring), `.github/` (CI/CD), root `Makefile`; database coordination of `core-api/migrations/`  
> **Key Upstream Dependencies:** every service owner (Dockerfile, health endpoints, env vars), Member 7 (gateway routes), Member 8 (queues, reconciler), Member 2 (seed, migrations)  
> **Open Questions / Needs Clarification:** Reconciler command is known (`/usr/local/bin/reconcile --stale-threshold=5m --batch-size=50`, `messaging` image; Member 8); how to make a liveness probe fail safely for the probe test (service owners); is Docker Compose the end-to-end test environment (Member 10)?  
> **Answers:** How will we prove the module is correct?  
> **My acceptance test:** **AT-016**, *"Kubernetes readiness/liveness behavior and reconciliation CronJob are demonstrated"* (SRS §12.2; `test-strategy.md`). Platform tests are part of the SRS §12.1 **"Deployment"** level: *"Docker Compose and Kubernetes acceptance tests: configuration, probes, gateway routes, persistence, CronJob, scaling"*.  

---

## 1. Test levels for the platform

| Level | What it proves | Tools | Runs |
| :--- | :--- | :--- | :--- |
| **Static checks** | config files are valid and follow the rules | `docker compose config -q`, `kubeconform -ignore-missing-schemas`, `yamllint`, `hadolint`, `helm lint`, secret scan | every PR (CI) |
| **Build** | every image builds from a clean clone and runs as non-root | `docker build`, `docker inspect` (User = 10001) | every PR (CI) |
| **Migration** | the schema builds from zero and rolls back; no duplicate numbers | golang-migrate against a fresh Postgres; a shell check | every PR touching migrations (CI) |
| **Local integration** | the Compose stack starts healthy, the gateway routes, start order is right | `make up`, `docker compose ps`, `curl` | before merge (local); nightly (optional) |
| **Cluster acceptance** | K8s workloads become Ready, probes work, data persists, CronJob runs, scaling works | k3d + `kubectl`, `probe_test.sh` | per milestone (M1, M4, M5) |
| **Security** | no secrets, non-root, internal-only services, least privilege | TruffleHog, `verify.sh`, `trivy`, NetworkPolicy tests | PR + milestone |
| **Resilience** | recovery from pod, broker, DB and bad-deploy failures | `kubectl delete pod`, scale to 0, rollback | M4/M5 (with M8 for the outbox) |
| **Clean-clone** | a new member can run everything from the docs only | a fresh folder or VM | M1 and M5 |

## 2. Test-case matrix

**Status key:** ⏳ planned (no code yet) · 🔧 can run now on the existing files.

| ID | Scenario | Type | Requirement | How | Expected | Auto? | Status |
| :-- | :--- | :--- | :--- | :--- | :--- | :-: | :-: |
| PLT-001 | Compose file is valid | static | SRS §11.1 | `docker compose -f deploy/docker-compose.yml config -q` | exit 0 | CI | ⏳ |
| PLT-002 | Existing K8s manifests are valid | static | SRS §11.2 | `kubeconform -summary -ignore-missing-schemas deploy/kubernetes/` | 0 invalid | CI | 🔧 |
| PLT-003 | Dockerfiles follow the rules | static | security-notes §6 | `hadolint` on every Dockerfile | no errors | CI | ⏳ |
| PLT-004 | No secrets committed | security | SRS §11.1; SEC-008 | TruffleHog + `scripts/verify.sh` | PASS | CI | 🔧 |
| PLT-005 | Images run as non-root | security | SRS SEC-016 | `docker inspect -f '{{.Config.User}}' <img>` | `10001:10001` | CI | ⏳ |
| PLT-006 | Image versions are pinned | static | Engineering Standards §10; SRS SEC-016 | grep for `:latest` in manifests and Compose | no match | CI | 🔧 (passes since 2026-10-28: CronJob uses `${IMAGE_TAG}`, K18) |
| PLT-007 | Migrations apply from zero | migration | AGENTS.md §7; SRS §7.3 | fresh Postgres → `migrate up` | exit 0, `dirty = false` | CI | ⏳ |
| PLT-008 | Migrations roll back and re-apply | migration | SRS §7.3 | `up` → `down -all` → `up` | all succeed | CI | ⏳ |
| PLT-009 | No duplicate migration numbers; every `.up` has a `.down` | migration | DB coordinator rule | shell check (duplicate-number script in CI) | no output | CI | ⏳ |
| PLT-010 | Clean clone starts the stack | integration | Work Division M9 acceptance 1 | fresh clone → `cp .env.example .env` → `make up` | all healthy | manual | ⏳ |
| PLT-011 | Migrations run before core-api | integration | C22 | `docker compose ps -a`, logs order | migrate `exited (0)` before core-api starts | manual | ⏳ |
| PLT-012 | Gateway routes reach each service | integration | ADR-001; C17 | `curl localhost/api/v1/tracking/x`, `/api/v1/ai/x`, `/api/v1/x`, `/` | each hits the right stub/service | script | ⏳ |
| PLT-013 | Health and metrics are not public | security | C1; SRS §11.4 | `curl localhost/health/live` via the gateway | not routed to a backend (404 or the web app) | script | ⏳ |
| PLT-014 | Workloads become Ready | cluster | Work Division M9 acceptance 2; AT-016 | `kubectl wait --for=condition=ready pod --all -n swiftdrop` | all Ready | script | ⏳ |
| PLT-015 | Readiness removes a pod when the DB is down | cluster | AT-016; SRS §9.2 | scale postgres to 0 → check core-api endpoints | pods `NotReady`, not restarted; back to Ready when the DB returns | manual | ⏳ |
| PLT-016 | Liveness restarts a stuck pod | cluster | AT-016 | `probe_test.sh` (make the liveness check fail, e.g. a debug flag; TBD with owners) | `RESTARTS` increases | script | ⏳ |
| PLT-017 | Postgres data survives a pod restart | cluster | SRS §11.2 | insert a row → `kubectl delete pod postgres-0` → read the row | row still there | script | 🔧 (on k3d with the existing StatefulSet) |
| PLT-018 | Secrets come from K8s Secrets | security | Work Division M9 acceptance 3 | `kubectl get deploy -o yaml` shows `secretRef`; no literal values | pass | script | ⏳ |
| PLT-019 | Probes and resources on every workload | static | Work Division M9 acceptance 4 | check every Deployment/StatefulSet has probes + requests/limits | none missing | CI | ⏳ |
| PLT-020 | Reconciliation CronJob runs safely | cluster | AT-016; FR-REC-004 | `kubectl create job --from=cronjob/swiftdrop-outbox-reconciler test-run` | `Complete`, logs bounded | manual | ⏳ (command known, C10; needs the `messaging` image) |
| PLT-021 | CronJob never overlaps | cluster | ADR-002 §4 (`concurrencyPolicy: Forbid`) | start a long run, wait for the next schedule | second run skipped (`Forbid`) | manual | ⏳ |
| PLT-022 | HPA scales core-api | cluster | SRS §15.3 | load test → `kubectl get hpa -w` | replicas 2 → up to 4, then back | manual (demo) | ⏳ |
| PLT-023 | KEDA scales event-worker on backlog | cluster | SRS §15.3 | publish more than 10 messages to `q.notifications.worker` or `q.webhooks.dispatcher` | replicas go up, then back to 1 | manual (demo) | ⏳ |
| PLT-024 | Outbox Relay never scales | static | ADR-002 §4 | check no HPA/ScaledObject targets `outbox-relay` and `replicas: 1` | pass | CI | ⏳ |
| PLT-025 | Bad deploy rolls back | resilience | runbook §7; demo | deploy a broken tag → `rollout undo` | the old version serves again | manual (demo) | ⏳ |
| PLT-026 | Concurrent migration runs are safe | migration | Doc 2 §4 | start 2 migrate runs at once | one waits (lock), both end OK, `dirty = false` | manual | ⏳ |
| PLT-027 | Backup and restore drill | resilience | SRS §7.3; runbook §3 | `pg_dump` → drop the DB → `pg_restore` | data back; version matches | manual | ⏳ |
| PLT-028 | Broker outage is recoverable | resilience | SRS §9.2; AT-009 (M8) | scale rabbitmq to 0 → write events → back to 1 | the outbox drains (with M8) | manual | ⏳ |
| PLT-029 | DB connection budget holds at max scale | cluster | C11; I7 | scale all to max → `SELECT count(*) FROM pg_stat_activity` | < `max_connections` | manual | ⏳ |
| PLT-030 | Prometheus scrapes every service | observability | SRS §11.4 | Prometheus targets page | all `UP` | manual | ⏳ |
| PLT-031 | Grafana dashboard shows the key panels | observability | Work Division M9 deliverable | open the dashboard | rate, latency, errors, queue depth, outbox backlog | manual (demo) | ⏳ |

## 3. Negative paths covered

| Kind | Cases |
| :--- | :--- |
| **Invalid input** | broken YAML (PLT-001/002), Dockerfile running as root (PLT-005), duplicate migration number (PLT-009), `:latest` tag (PLT-006) |
| **Authorization / exposure** | health and metrics not public (PLT-013); secrets only from K8s Secrets (PLT-018); NetworkPolicy blocks DB access from non-app pods *(M4, planned)* |
| **Race conditions** | concurrent migrations (PLT-026); CronJob overlap (PLT-021); concurrent deploys (Actions `concurrency`) |
| **Dependency failures** | DB down → readiness (PLT-015); broker down (PLT-028); failed migration stops the deploy (Doc 2 F4); bad image → rollback (PLT-025) |

## 4. Mocking strategy and test environment

| Real thing | Stand-in for tests | Why |
| :--- | :--- | :--- |
| App services (not written yet) | `traefik/whoami` stub containers with the same names | test gateway routing and Compose wiring now |
| VPS K3s | **k3d** (K3s in Docker) | same distribution, free, local |
| Email/SMS providers | `NOTIFICATION_PROVIDER=mock` (M8) | no paid services (SRS §11.1) |
| LLM provider | `LLM_PROVIDER=mock` (M10) | no API key needed |
| Merchant webhook receiver | local fake receiver (M8) | repeatable webhook tests |
| Cloudflare | not used locally; plain HTTP on port 80 (SEC-001 allows it) | — |

## 5. Fixtures, seed data and repeatability

- Every CI run uses a **fresh Postgres** service container; no shared database.
- Demo data comes from `make seed` (deterministic; M2 provides the command).
- `make reset` (proposed) wipes the volumes for a clean start; tests never depend on leftover data.
- Cluster tests run in the `swiftdrop` namespace on a throwaway k3d cluster, which can be recreated with one command.

## 6. Coverage expectations

Line coverage doesn't apply to config files. Platform "coverage" means:

| Measure | Target |
| :--- | :--- |
| Workloads with probes + requests/limits | **100%** of the workloads that need them (PLT-019) |
| Manifests passing the schema check | **100%** (PLT-002) |
| Services in Compose with a healthcheck | **100%** |
| Migrations tested up/down/up in CI | **100%** (PLT-008) |
| AT-016 parts demonstrated | probes ✅ + CronJob ✅ |
| Work Division M9 acceptance criteria 1–5 | all have evidence (§7) |

## 7. Acceptance criteria and evidence

| Criterion (Work Division §3 M9 + AT-016) | Evidence |
| :--- | :--- |
| 1. A new member starts the local stack from a clean clone | PLT-010 log or screen recording; the steps in `deploy/README.md` |
| 2. K8s workloads become Ready and communicate through Services | PLT-014 output; PLT-012 via the gateway |
| 3. Secrets come from K8s Secrets, not committed files | PLT-004 + PLT-018 output |
| 4. Probes and resources are present | PLT-019 CI result |
| 5. The scaling, deployment and monitoring demo is reproducible, with honest claims | PLT-022/023/025/031 demo notes; anything optional that isn't done is listed as a known limitation |
| **AT-016** probes + CronJob | PLT-015, PLT-016, PLT-020 output |
| Migrations correct (DB coordinator) | PLT-007/008/009 CI runs; migration log |

Evidence goes into the team test report (M10 coordinates; SRS §12.3) and the contribution record.

## 8. How to run the tests

**Locally (planned commands; the Makefile names are from C22, the extras are proposed):**

```bash
# static checks
docker compose -f deploy/docker-compose.yml config -q
kubeconform -summary -ignore-missing-schemas deploy/kubernetes/
./scripts/verify.sh            # Windows: .\scripts\verify.ps1

# stack
make up && make migrate-up && make seed
docker compose -f deploy/docker-compose.yml ps

# cluster (k3d); KEDA must be installed first (Helm), and ${IMAGE_TAG} filled in (e.g. envsubst)
kubectl apply -R -f deploy/kubernetes/ -n swiftdrop
kubectl wait --for=condition=ready pod --all -n swiftdrop --timeout=180s
./deploy/kubernetes/tests/probe_test.sh
```

**In CI:** `ci.yml` jobs (secret scan, Go) run today. Planned new jobs: infra lint, migration checks, image build, web and Python tests (Doc 4 §9; C25). Cluster acceptance tests run manually on k3d at each milestone and are recorded in the test report.
