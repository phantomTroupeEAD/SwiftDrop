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

## Team agreements with Member 9 (read this first)

📄 **[team-agreements-with-member-09.md](./team-agreements-with-member-09.md)**: the shared rules every member agrees with Member 9:
- ports, health paths, Dockerfile rules, env vars and secrets, the Compose/Makefile stack, Kubernetes layout, gateway split, request ID, metrics, CI;
- the database rules: migration tool, file names, order, stubs, review, DB roles, rights, connection budget, timeouts, driver, seed.

It also has answers to every question other members asked Member 9, a list of conflicts found in submitted drafts, and a sign-off table.

🤖 **Check your drafts with your own AI (5 min):** [AI-REVIEW-PROMPT.md](./AI-REVIEW-PROMPT.md). Paste the prompt into Claude Code, Cursor, Copilot or ChatGPT; it lists exactly what to change in your folder.

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

## Already decided by the team (2026-10-24 / 25)

| Code | Decision |
| :-: | :--- |
| C1 | Health paths follow the SRS: `/health/live`, `/health/ready`, `/metrics` (internal only) |
| C3 | Docker Compose lives at `deploy/docker-compose.yml` |
| C4 | Secret names: `postgres-secrets`, `rabbitmq-secrets`, `core-api-secrets`, `jwt-secrets`, `integrations-secrets`. Later refined in A5: `postgres-migrator-secrets` added, and core-api module secrets use `<service>-<module>-secrets` (e.g. `core-api-merchants-secrets`) |
| C11 | Member 9 owns the DB connection limits and pool sizes |
| C17 | Path-based routing on one domain (`/api/v1/tracking`, `/api/v1/ai`, `/api/v1`, `/`) |
| C22 | golang-migrate as a one-shot container (+ K8s Job); root `Makefile` (`make up`, `migrate-up`, `seed`, `test-e2e`) |
| — | Member 9 is the database coordinator; Member 8 leads the core-api Dockerfile; **Member 9 owns `web/Dockerfile` + Nginx config** (Member 6, 2026-10-25) |
| C6 | KEDA and apps use `RABBITMQ_URL` (`amqp://swiftdrop:<password>@rabbitmq.swiftdrop.svc.cluster.local:5672/`) (Member 8, 2026-10-25) |
| C7 | `event-worker` consumes `q.notifications.worker` + `q.webhooks.dispatcher` (one Deployment, two KEDA triggers, `queueLength 10`) (Member 8) |
| C10 | Reconciler = `messaging` image, `/usr/local/bin/reconcile --stale-threshold=5m --batch-size=50`, every 10 min (Member 8) |
| C13 | DLQ `swiftdrop.dlq`; outbox columns from `database-schema.md` (`status`, `attempt_count`, `published_at`, `last_error`) (Member 8) |
| C15 | Web Nginx listens on **3000**, SPA fallback; build with `VITE_API_BASE_URL=/api/v1` (Member 6) |
| DB budget | Postgres `max_connections` = **150** (memory 1Gi); relay pool 5, worker pool 10 per pod (Member 9 + Member 8) |
| C16 | Traefik dashboard on port **8090**, localhost/internal only (Member 7 + Member 9) |
| D2 | 6-digit sequential migration naming (`000NNN_name.up.sql`, `000001`–`000011`) ratified (also required by AGENTS.md §7) |
| D3 | First-wave migration order = the hub list (`docs/drafts/README.md` §4), plus `notification_deliveries` in `000009` (DB coordinator, 2026-10-28) |
| E2E | `make test-e2e` = `pytest tests/e2e/ -v` (Member 10's Python suite); CI adds `playwright install --with-deps chromium` (2026-10-28; Member 10 left it to Member 9) |
| D6 | Bootstrap DB roles ratified (`swiftdrop_migrator`, `swiftdrop_app`, `swiftdrop_tracking_ro`, `swiftdrop_relay`, `swiftdrop_worker`, `swiftdrop_reconciler`) |
| A8 | Kubernetes manifest layout standardized to per-service modular `.yaml` folders |

## Open questions for Meeting 2

The docs refer to these by code (C = infrastructure doc conflict, DB = schema problem).

**Infrastructure (C):**

| Code | Question | Who |
| :-: | :--- | :--- |
| C2 | Manifest layout: ✅ resolved: per-service modular `.yaml` folders (A8) | M9 |
| C5 | TLS origin-certificate secret name and namespace (`traefik-origin-tls`) | M9, M7 |
| C8 | Tracking API HPA range: the manifest scales 1→2 today; Member 5 picks the final range (max 4 pods, DB budget). Redis is needed at 2 or more | M5 |
| C9 | Core API and web HPA files are referenced but missing | M9 |
| C12 | ✅ Runbook commands and StatefulSet/CronJob names resolved in `runbook.md` | M9 |
| C13 | ✅ DLQ and columns agreed (see above); ⏳ still open: a `correlation_id` column | M8 |
| C14 | ✅ Postgres probe user set to `swiftdrop` on `swiftdrop_db` with 1Gi limit | M9 |
| C18 | Public tracking and SSE paths | M5 |
| C19 | Rate-limit values (gateway vs security notes) | M7 |
| C20 | CORS settings (gateway vs security notes) | M7 |
| C21 | ✅ Env var names aligned in `environment-variables.md` (web `VITE_API_BASE_URL`, storage aliases `S3_*` / `OBJECT_STORAGE_*`) | M1, M3, M7, M9 |
| C23 | Outbox Relay scaling wording (SRS "may scale" vs singleton rule); singleton kept | M8 |
| C24 | ✅ registry `ghcr.io/phantomtroupeead/swiftdrop/<image>:<git-sha>` (A3); ⏳ demo domain | M9, coordinator |
| HOST | Hosting (the team left it to Member 9): trying **Azure for Students** B2ms first; **Hetzner CX33** is the paid backup; Oracle trial only for practice now. At least 8 GB RAM. See ADR-002 §2 | M9 |
| C25 | ✅ Go CI jobs now key on `core-api/go.mod`, so draft `go.mod` files no longer break CI (2026-10-28); ⏳ unpinned actions (TruffleHog `@main`, golangci-lint `latest`), no web/Python jobs | M9 |
| A5+ | DB URL keys per login role inside `postgres-secrets` (`TRACKING_`, `RELAY_`, `WORKER_`, `RECONCILER_DATABASE_URL`); every service still reads `DATABASE_URL` (amendment 2026-10-28) | M9, M5, M8 |
| C26 | ✅ object storage: Cloudflare R2 in production, MinIO locally (Member 3); ⏳ Redis in Kubernetes | M5 |

**Database (DB):**

| Code | Question | Who |
| :-: | :--- | :--- |
| DB1 | Cross-module foreign keys: allowed as DB constraints (SRS §7.2), but no cross-module JOINs? | team |
| DB2 | ✅ columns agreed: `database-schema.md` version (Member 8); ⏳ add `correlation_id` | M8 |
| DB3 | `processed_events` (consumer dedup) table missing from `database-schema.md` | M8 |
| DB4 | ✅ `refresh_tokens` is now in `database-schema.md` | M1 |
| DB5 | `ai_conversations` conflicts with the AI no-database rule; leave it out of the baseline? | M10 |
| DB6 | `parcels.sender_id` and `pricing_quotes.requester_id` point at two different tables | M2 |
| DB7 | ✅ `idempotency_keys` owned by Member 2 (migration `000010`), primary key (`key`, `caller_scope`), 24-hour TTL (Member 2, 2026-11-01) | M2 |
| DB8 | Text primary keys for `roles` / `delivery_zones` (fine as lookup codes) | M9 |
| DB9 | Money type: `bigint` cents only (drop the `NUMERIC` option) | M9 |
| DB10 | Who creates the per-service DB login roles (proposal: M9, reviewed by M1) | M9, M1 |
| DB11 | Webhook signing secret: ✅ Member 4 designed AES-256-GCM encryption with a key from a K8s Secret (shared with the worker); ⏳ rename the `secret_hash` column in `database-schema.md` | M4, M8 |
| DB12 | Webhook targets view name: owner Member 4 uses `merchant_webhook_targets`; the Master Audit says `merchant_webhook_targets_view`. Member 9 docs follow the owner | M4, M8 |
| DB13 | ✅ JSONB snapshots on `parcels` (`pickup_address`, `dropoff_address`); no `parcel_addresses` table (Member 2, 2026-11-01). ⏳ Member 2 pushes the `database-schema.md` update; the hub list and Master Audit §6.4 still name `parcel_addresses` | M2 |
