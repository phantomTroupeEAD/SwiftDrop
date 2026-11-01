# Document 1 — Module Scope & Responsibility Specification

> **Status:** Draft / Proposed (For Review in Team Meeting 2)  
> **Author:** Member 9 — DevOps, Kubernetes & Observability + Database Coordinator (Minidu Dhananjana)  
> **Target Module / Location:** `deploy/` (Docker Compose, Kubernetes, Helm, monitoring), `.github/` (CI/CD), root `Makefile`; database coordination of `core-api/migrations/`  
> **Key Upstream Dependencies:** every service owner (Dockerfile, health endpoints, env vars), Member 7 (gateway routes), Member 8 (queues, reconciler), Member 2 (seed, migrations)  
> **Open Questions / Needs Clarification:** Hosting provider and size for the K3s server (Hetzner vs Azure for Students); who has GitHub admin rights for branch protection and code-owner review; domain for Cloudflare; whether Redis and object storage run inside Kubernetes (Members 5 and 3).  
> **Answers:** What does this module own, and what must it never own?  

---

## 1. Purpose and business problem

SwiftDrop is built by **10 developers**. The system they build consists of:

- **6 deployable services:** Core API, Tracking API, Outbox Relay, Event Worker, AI Assistant, Web;
- **2 stateful systems:** PostgreSQL and RabbitMQ;
- an **edge gateway** (Traefik).

Without a platform module:

- each service runs only "on my machine", so integration and the final demo fail (Engineering Standards §15: *"A feature that only runs on one member's machine is not complete"*);
- the **Kubernetes bonus** (SRS §15.3) cannot be shown;
- nobody can see whether the system is healthy, because the observability required by SRS §11.4 is missing;
- 10 people change one shared PostgreSQL schema with no coordination, which leads to duplicate tables, number clashes and broken foreign keys.

**Mission (Work Division §3):** *"Make the full system reproducible locally and deployable/observable on Kubernetes for the assignment bonus."* On top of that, the team elected me **database coordinator** (2026-10-24).

---

## 2. Functional responsibilities

| # | Responsibility | Source |
| :-: | :--- | :--- |
| R1 | Dockerfiles for all 6 services, written **together with** each owner (templates from me, each owner understands their own). The core-api Dockerfile is led by Watti (M8); the **`messaging`** image (relay, worker and reconcile in one Dockerfile) is Watti's; **`web/Dockerfile` + the Nginx config are mine** (Sajini, M6, 2026-10-25) | Work Division §3 and §9; group decision |
| R2 | Local **Docker Compose** stack (`deploy/docker-compose.yml`) with Postgres, RabbitMQ, Redis, MinIO, Traefik and the app services | SRS §11.1; C3 decision |
| R3 | Root **Makefile**: `make up`, `make migrate-up`, `make seed`, `make test-e2e` | C22 decision |
| R4 | **Kubernetes** manifests (and the Helm chart for the bonus): Deployments, Services, StatefulSets, Job, CronJob, ConfigMap and Secret templates, NetworkPolicies | SRS §11.2; ADR-002 |
| R5 | Probes, resource requests and limits, graceful shutdown settings, and **HPA/KEDA** autoscaling | SRS §11.2; ADR-002 §7 |
| R6 | **Traefik install** (Helm) and Ingress connectivity. Route and policy content comes from M7 | ADR-001; ADR-002 |
| R7 | **CI/CD**: quality gates, image build and push, deployment rollout | Engineering Standards §12.3; `deployment-guide.md` §5 |
| R8 | **Observability**: Prometheus scraping, the Grafana dashboard, alerts | SRS §11.4; Engineering Standards §11 |
| R9 | **Operations docs**: deployment guide, runbook, environment variable inventory, ADR-002 | contribution record |
| R10 | **DB coordinator**: review every migration (naming, foreign keys, constraints, indexes, ordering), track who changed what, reserve migration numbers | Engineering Standards §6; Work Division §4; election 2026-10-24 |
| R11 | **Postgres operations**: `max_connections`, pool budget per service, DB login roles, backup and restore | C11 decision; ADR-002 §9; runbook §3 |
| R12 | Help every member **reproduce their service** locally and in the cluster | Work Division §3 |

## 3. Explicit exclusions (this module must NEVER own)

| Not mine | Who owns it | Why |
| :--- | :--- | :--- |
| Business logic, domain rules, parcel states | M1–M5, M8 (domain owners) | Engineering Standards §2.6: M9 works *"without embedding domain logic into deployment configurations"* |
| Debugging other members' application code alone | each service owner | Work Division §3: *"not expected to debug all application logic alone"* |
| Content of other members' tables and migrations | each table owner | Work Division §4. I **review** them; I don't write them |
| Gateway route rules, CORS, rate-limit values | M7 Bagya | ADR-001 §2; `deploy/gateway/README.md` |
| RabbitMQ exchanges, queues, DLQ design, reconciler logic | M8 Watti | Work Division §3 (Member 8) |
| Login, JWT, roles, tenant checks | M1 Umesh + each endpoint owner | ADR-001 §2: the backend does auth, not the gateway |
| End-to-end test suite | M10 Kakkaya coordinates; each owner tests their own | Work Division §4 |
| Being the only person who can deploy | all owners must understand their Dockerfile, health and config | Work Division §9 risk control |

---

## 4. Upstream and downstream dependencies

**Upstream (what I need from others):**

| From | What | Status |
| :--- | :--- | :--- |
| Every service owner | port, start command, health/metrics endpoints, env vars, build context, smoke test | asked through a per-service deployment form (Member 9) |
| M7 Bagya | route map, the `IngressRoute`/`Middleware` CRDs, CORS and rate limits | path-based routing decided (C17) |
| M8 Watti | queue names for KEDA, reconciler command and image, relay/worker metrics | ✅ answered 2026-10-25 (queues, reconciler, ports 8082/8083, metrics, pools); ⏳ health paths, `correlation_id`, `processed_events` |
| M5 Senuji | SSE paths, Redis need, HPA numbers | TBD (C8, C18, C26) |
| M2 Hansi | seed command, core-api pool size confirmation, idempotency table owner | TBD (C11, DB7) |
| Every table owner | migrations that follow the rules in Doc 5 | rules announced to the team |
| Coordinator | milestone dates, VPS + Cloudflare + domain, GitHub admin rights, GHCR access | TBD |

**Downstream (who depends on me):**

| Who | Uses |
| :--- | :--- |
| All 10 developers | `make up`, the Compose stack, CI checks, migration rules |
| M10 Kakkaya | the Compose stack as the end-to-end test environment |
| M7 Bagya | Traefik installed in the cluster and connected to the services |
| M8 Watti | RabbitMQ deployed and monitored; the relay, worker and CronJob deployed |
| M5 Senuji | Tracking Deployment, HPA, Redis (if needed) |
| The demo | K8s pods healthy, health checks, Grafana dashboard, rollback |

---

## 5. Bounded context, domain ownership and authoritative data

- **Context type:** a *generic/supporting technical subdomain*. It is **not** one of the business bounded contexts in `domain-boundaries.md`.
- **Authoritative for:**
  - the **deployment configuration** (Compose, manifests, Helm values, CI workflows);
  - the **migration process** (ordering, review, runner);
  - the **Postgres instance settings** and DB roles.
- **Not authoritative for:** any business data. PostgreSQL stays the source of truth for parcels, and the domain owners decide what goes in it.

---

## 6. Invariants this module must protect

| # | Invariant | Source |
| :-: | :--- | :--- |
| I1 | Outbox Relay always has **`replicas: 1`**; no HPA or KEDA | AGENTS.md §4.4; ADR-002 §7.2 |
| I2 | PostgreSQL and RabbitMQ run as **StatefulSets** with persistent volumes, and are never scaled | AGENTS.md §4.4; SRS §11.2 |
| I3 | **No real secrets** in git, images, CI logs or committed Helm values | SRS §11.1; AGENTS.md §8 |
| I4 | **Pinned image tags**; no `latest` in the final submission | Engineering Standards §10 |
| I5 | **Only Traefik is public.** Postgres, RabbitMQ (including its management UI), health and metrics stay internal | Engineering Standards §10; SRS §11.4 |
| I6 | Containers **run as non-root** (UID 10001) | security-notes §6 |
| I7 | Total DB connections stay **≤ Postgres `max_connections`** | ADR-002 §7.1; C11 |
| I8 | **Migrations run before the app starts.** Every migration is **reviewed by the DB coordinator** and has a **unique number** | C22; Engineering Standards §6 |
| I9 | Every workload that needs them has **probes and requests/limits** | SRS §11.2; Work Division §3 acceptance |
| I10 | `main` stays **runnable from a clean clone** | Work Division §4; Engineering Standards §12.3 |

---

## 7. What other modules may request from this module

| Request | How to ask | My response time goal |
| :--- | :--- | :--- |
| Add or change an env var, secret or ConfigMap key | update `environment-variables.md` + `.env.example` in a PR, or message me | next working day |
| Deploy a new service or change a port | fill a block in the service deployment contract | before the next milestone |
| Change scaling (replicas, HPA/KEDA) | message me with the metric and reason; the singleton rule cannot be waived | discuss in group |
| New CI check or job | PR to `.github/workflows/` (I review) | next working day |
| A migration number, a migration review, or a DB permission | open the migration PR; I am auto-requested via CODEOWNERS | within 1 working day |
| Change your service's DB pool size | message me; I recheck the total budget | next working day |
| Grafana panel or alert | send me the metric name (Doc 4 §5) | M4 milestone |

---

## 8. Ownership of files, tables, APIs, queues and configuration

| Type | Owned by M9 | Shared | Not mine |
| :--- | :--- | :--- | :--- |
| **Files** | `deploy/docker-compose.yml`, `deploy/kubernetes/**`, `deploy/helm/**` *(planned)*, `deploy/docker/templates/**` *(planned)*, `deploy/monitoring/**` *(planned)*, root `Makefile` *(planned)*, `.github/workflows/**`, `.github/CODEOWNERS` *(planned)*, `scripts/verify.*`, `deploy/README.md`, `docs/operations/{deployment-guide,runbook,environment-variables}.md`, ADR-002 | `deploy/gateway/**` (M7 policies, M9 Traefik install/`values.yaml`); each service's Dockerfile (owner + M9 template); `cronjob-reconciliation.yml` (M8 command, M9 manifest) | all source code, `core-api/migrations/*.sql` content (table owners) |
| **Tables** | none of the business tables. `schema_migrations` (golang-migrate) and the DB roles are mine | `docs/architecture/database-schema.md` (each owner edits their table; I review) | every business table (see Doc 5 §2) |
| **APIs** | no business API. I define the health/metrics contract (Doc 4) | — | all `/api/v1/*` |
| **Queues** | none | RabbitMQ broker deployment (me) vs topology (M8) | exchanges, queues, DLQ |
| **Configuration** | `.env.example` (root), Secret/ConfigMap templates, Postgres/RabbitMQ container config, Traefik Helm values | `traefik.yml` / `dynamic-routes.yml` (M7) | app-level business settings (pricing etc.) |

### RACI (R = does it, A = accountable, C = consulted, I = informed)

| Activity | M9 Minidu | Service owner | M7 Bagya | M8 Watti | M10 Kakkaya |
| :--- | :-: | :-: | :-: | :-: | :-: |
| Dockerfile for a service | C (template) | **R/A** | I | I (core-api lead) | I |
| Compose stack and Makefile | **R/A** | C | C | C | I |
| K8s manifests and Helm | **R/A** | C | C (gateway) | C (relay/worker) | I |
| Gateway routes and policies | C | I | **R/A** | I | I |
| Migration content | C (review) | **R/A** (own tables) | — | — | — |
| Migration review and ordering | **R/A** | C | — | — | — |
| CI pipeline | **R/A** | C | C | C | C |
| E2E environment | R (stack) | C | C | C | **A** |
| Grafana dashboard | **R/A** | C (metrics) | — | C | — |

---

## 9. Assumptions, unresolved decisions, out-of-scope work

**Assumptions:**
- the target is one Ubuntu VPS running **K3s** (ADR-002), with **k3d** used locally;
- Cloudflare sits in front;
- images are stored in **GHCR**;
- PostgreSQL and RabbitMQ run **inside** the cluster, because the bonus rubric may require it (SRS §15.3, Work Division §9).

**Decided (2026-10-24):** C1 health paths · C3 Compose location · C4 secret names · C11 M9 owns the pool budget · C17 path routing · C22 golang-migrate + Makefile · Minidu is DB coordinator · Watti leads the core-api Dockerfile.

**Still open:**
- infra conflicts C2, C5, C8, C9, C12, C13 (`correlation_id`), C14, C18–C21 and C23–C26 (C6, C7, C10, C15 and C16 were settled on 2026-10-25);
- schema problems DB1–DB11;
- the VPS provider and size, the domain, and GitHub admin rights.

**Out of scope:** multi-region or HA clusters, disaster recovery with a formal RPO/RTO (SRS §1.5), managed cloud Kubernetes, a service mesh, GitOps tools (Argo CD/Flux), and 24/7 on-call.
