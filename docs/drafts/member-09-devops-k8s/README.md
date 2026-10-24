# Member 09 - DevOps, Kubernetes & Observability Specifications

> **Owner:** Member 09 — Minidu Dhananjana (also the team's **database coordinator**, elected 2026-10-24)  
> **Status:** Draft Specifications for Review in Team Meeting 2  
> **Target Production Location:** `deploy/`, `.github/`, root `Makefile` (see docs/architecture/domain-boundaries.md); database coordination of `core-api/migrations/`

---

## Required Documents Checklist

- [x] [doc-01-scope-and-responsibilities.md](./doc-01-scope-and-responsibilities.md)
- [x] [doc-02-functional-flows-and-business-rules.md](./doc-02-functional-flows-and-business-rules.md)
- [x] [doc-03-internal-architecture-and-design.md](./doc-03-internal-architecture-and-design.md)
- [x] [doc-04-interface-and-integration-contract.md](./doc-04-interface-and-integration-contract.md)
- [x] [doc-05-data-and-persistence.md](./doc-05-data-and-persistence.md)
- [x] [doc-06-testing-strategy-and-acceptance.md](./doc-06-testing-strategy-and-acceptance.md)
- [x] [doc-07-security-failure-and-recovery.md](./doc-07-security-failure-and-recovery.md)
- [x] [doc-08-implementation-deployment-and-ops.md](./doc-08-implementation-deployment-and-ops.md)

---

## How this module differs from business modules

The 8-document template was written for business modules such as Parcels or Merchants. Member 9's module is a **technical supporting module**: it has **no business entities, no business REST API and no domain events**. Where the template asks for those, the docs explain the platform equivalent:

| Template asks for | For the platform this means |
| :--- | :--- |
| REST APIs exposed | The **health/metrics contract** every service must implement, the **Make targets**, the **env var / secret contract**, the **image naming contract** and the **CI checks** (doc-04) |
| Domain entities and aggregates | Not used (doc-03 explains why). Building blocks are images, Compose services, K8s workloads and pipelines |
| Tables owned | **No business tables.** As **DB coordinator** Member 9 owns the *migration process*, Postgres itself, `schema_migrations`, the DB login roles and the connection budget (doc-05) |
| State machines | Migration lifecycle, deployment rollout, pod readiness (doc-02) |
| Events | None produced or consumed. KEDA only *reads* RabbitMQ queue length |

---

## Already decided by the team (2026-10-24)

| Code | Decision |
| :-: | :--- |
| C1 | Health paths follow the SRS: `/health/live`, `/health/ready`, `/metrics` (internal only) |
| C3 | Docker Compose lives at `deploy/docker-compose.yml` |
| C4 | Secret names: `postgres-secrets`, `rabbitmq-secrets`, `core-api-secrets`, `jwt-secrets`, `integrations-secrets` |
| C11 | Member 9 owns the DB connection limits and pool sizes |
| C17 | Path-based routing on one domain (`/api/v1/tracking`, `/api/v1/ai`, `/api/v1`, `/`) |
| C22 | golang-migrate as a one-shot container (+ K8s Job); root `Makefile` (`make up`, `migrate-up`, `seed`, `test-e2e`) |
| — | Member 9 is the database coordinator; Member 8 leads the core-api Dockerfile |

## Open questions for Meeting 2

The docs refer to these by code (C = infrastructure doc conflict, DB = schema problem).

**Infrastructure (C):**

| Code | Question | Who |
| :-: | :--- | :--- |
| C2 | Manifest folder: keep `deploy/kubernetes/*.yml` (real files) instead of `deploy/k8s/` (ADR-002) | M9 |
| C5 | TLS origin-certificate secret name and namespace (`traefik-origin-tls`) | M9, M7 |
| C6 | KEDA secret key `amqp-uri` is not defined anywhere; use `RABBITMQ_URL` | M9, M8 |
| C7 | Real queue name(s) for KEDA (`parcel.events` doesn't exist) and the replica max | M8 |
| C8 | Tracking API HPA numbers (manifest 1→2 vs docs 2→4); Redis needed if ≥ 2 | M5 |
| C9 | Core API and web HPA files are referenced but missing | M9 |
| C10 | Reconciler schedule (`*/10` vs `*/15`), name, pinned image, exact command | M8 |
| C12 | Runbook commands that don't match the real resources (StatefulSet names) | M9 |
| C13 | Outbox column names and DLQ name differ between docs | M8 |
| C14 | Postgres probe uses user `postgres`, but the app user is `swiftdrop` | M9 |
| C15 | Web container port 3000 vs 80 | M6, M7 |
| C16 | Host port 8080 clash (Traefik dashboard vs native Core API) | M7, M9 |
| C18 | Public tracking and SSE paths | M5 |
| C19 | Rate-limit values (gateway vs security notes) | M7 |
| C20 | CORS settings (gateway vs security notes) | M7 |
| C21 | Env var names (`VITE_API_GATEWAY_URL` vs `VITE_API_BASE_URL`, Google callback path, SRS-only vars) | M1, M6, M7, M8 |
| C23 | Outbox Relay scaling wording (SRS "may scale" vs singleton rule); singleton kept | M8 |
| C24 | Repo URL, image registry (GHCR), demo domain | M9, coordinator |
| C25 | CI gaps (unpinned actions, no web/Python jobs) | M9 |
| C26 | Redis and object storage (MinIO / R2) in Kubernetes | M5, M3 |

**Database (DB):**

| Code | Question | Who |
| :-: | :--- | :--- |
| DB1 | Cross-module foreign keys: allowed as DB constraints (SRS §7.2), but no cross-module JOINs? | team |
| DB2 | Outbox table defined twice with different columns; `correlation_id` missing in one | M8 |
| DB3 | `processed_events` (consumer dedup) table missing from `database-schema.md` | M8 |
| DB4 | Refresh-token / session table missing | M1 |
| DB5 | `ai_conversations` conflicts with the AI no-database rule; leave it out of the baseline? | M10 |
| DB6 | `parcels.sender_id` and `pricing_quotes.requester_id` point at two different tables | M2 |
| DB7 | `idempotency_keys` has no named owner, and its key should be (caller, operation, key) | M2 |
| DB8 | Text primary keys for `roles` / `delivery_zones` (fine as lookup codes) | M9 |
| DB9 | Money type: `bigint` cents only (drop the `NUMERIC` option) | M9 |
| DB10 | Who creates the per-service DB login roles (proposal: M9, reviewed by M1) | M9, M1 |
| DB11 | Webhook signing secret stored as `secret_hash`: signing needs the real secret, and secrets must not be in DB dumps (SRS §7.3) | M4, M8 |
