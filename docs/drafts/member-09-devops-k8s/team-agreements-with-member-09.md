# Team Agreements with Member 9 (DevOps, Kubernetes & Observability + Database Coordinator)

> **Status:** Draft / Proposed (For Review in Team Meeting 2)  
> **Author:** Member 9 — Minidu Dhananjana  
> **Purpose:** One page of **shared rules** every member must agree with Member 9, so that all 10 services build, run, deploy and migrate together. It also answers the questions other members asked Member 9 in their drafts, and lists the places where submitted drafts disagree with team decisions.  
> **Based on:** the group decisions of 2026-10-24, the repo docs (SRS, ADR-001/002, AGENTS.md, `docs/operations/*`, `database-schema.md`) and the drafts submitted on this branch by Members 2, 3, 4, 7 and 10 (checked 2026-10-25).  
> **How to reply:** tick your row in §6, or write your objection next to the item number (e.g. "A5: I disagree because…").

---

## 1. Agreements for ALL members

| # | Topic | Agreement | Source / status |
| :-: | :--- | :--- | :--- |
| **A1** | **Service names and ports** | `core-api` **8080** · `tracking-api` **8081** · `ai-assistant` **8000** · `web` **3000** · `outbox-relay` **8082** · `event-worker` **8083** (internal health + metrics only). The same names are used in Compose, Kubernetes and Traefik | ADR-001, `dynamic-routes.yml`, `environment-variables.md`; relay/worker ports from Member 8 (2026-10-25) |
| **A2** | **Health and metrics paths** | Every HTTP service exposes **`/health/live`** (process only, never checks the DB), **`/health/ready`** (checks only the dependencies the *whole* service cannot work without; returns 503 when one is down) and **`/metrics`**. **Internal only**: never routed by Traefik. For core-api, readiness checks **PostgreSQL only**. Optional or external dependencies (object storage, LLM, email, webhooks) must **not** make pods unready: their failures are handled per request (e.g. 503 on photo upload only), otherwise one outside outage takes the whole API offline | ✅ group decision C1 (SRS §6.2) + readiness rule (proposal) |
| **A3** | **Dockerfile rules** | multi-stage build · pinned base image (no `latest`) · **`USER 10001:10001`** · no secrets inside the image · `.dockerignore` · `EXPOSE` the A1 port · JSON logs to stdout · handle SIGTERM. A health check must not use tools the image doesn't contain (e.g. `curl` is **not** in `python:*-slim` or distroless images). **Images:** `core-api`, `tracking-api`, **`messaging`** (one image with 3 binaries: `/usr/local/bin/relay`, `worker`, `reconcile`; owned by Member 8), `ai-assistant`, `web`, `core-api-migrations`, all tagged `ghcr.io/phantomtroupeead/swiftdrop/<image>:<git-sha>`. **Exception:** `web/Dockerfile` + the Nginx config are **owned by Member 9**; Members 6 and 7 give build requirements and review | security-notes §6, SRS SEC-016; Member 8 and Member 6 answers (2026-10-25) |
| **A4** | **Env vars** | every variable your service reads must be listed in `docs/operations/environment-variables.md` and the root `.env.example` (placeholders only). New variables: send them to Member 9, who updates both in one PR | SRS §11.1–11.3 |
| **A5** | **Secrets (Kubernetes)** | Shared names: `postgres-secrets` (DB URLs, one key per login role, see below), `postgres-migrator-secrets` (migration URL), `rabbitmq-secrets`, `jwt-secrets` (private key), `integrations-secrets` (Google, LLM, email/SMS, object storage). **Module-specific secrets** use the pattern `<service>-<module>-secrets` (e.g. `core-api-merchants-secrets`). A key needed by **two services** (e.g. `WEBHOOK_SECRET_ENCRYPTION_KEY`, used by core-api and the worker) goes into one shared secret that both mount. **RabbitMQ:** apps and KEDA use one key, `RABBITMQ_URL` = `amqp://swiftdrop:<password>@rabbitmq.swiftdrop.svc.cluster.local:5672/` (user `swiftdrop`, vhost `/`). **DB URLs (amendment 2026-10-28):** one shared `DATABASE_URL` would make every service log in as `swiftdrop_app` and break D6. So `postgres-secrets` holds `DATABASE_URL` (core-api, `swiftdrop_app`), `TRACKING_DATABASE_URL`, `RELAY_DATABASE_URL`, `WORKER_DATABASE_URL` and `RECONCILER_DATABASE_URL` (key names from Member 8's doc-08). Every service still reads the env var **`DATABASE_URL`**; the manifest maps the right key into it. Never commit real values | ✅ C4 decision + this proposal for the new cases; RabbitMQ format agreed with Member 8; ⏳ DB URL amendment needs a team OK (Members 5, 8) |
| **A6** | **Non-secret config** | One ConfigMap per service: `<service>-config`. Module-specific ConfigMaps use the pattern `<service>-<module>-config` | proposal |
| **A7** | **Local stack** | One Compose file: **`deploy/docker-compose.yml`**, run through the root **`Makefile`**: `make up`, `make migrate-up`, `make seed`, `make test-e2e`. Members send their Compose needs as proposals; Member 9 merges them into the one file. No per-member Compose files in the final repo | ✅ C3 + C22 decisions |
| **A8** | **Kubernetes files** | Member 9 owns `deploy/kubernetes/`. **Layout: one folder per service** (`deploy/kubernetes/core-api/deployment.yaml`, `service.yaml`, …), extension **`.yaml`**. The existing flat `.yml` files move into this layout in one Member 9 PR. Members send needs as proposal files (as Members 4 and 7 already did) | proposal (adopts Member 7's layout) |
| **A9** | **Gateway** | Path-based routing on one domain: `/api/v1/tracking` → tracking-api, `/api/v1/ai` → ai-assistant, `/api/v1` → core-api, `/` → web. Member 7 writes the routes and middlewares; Member 9 installs Traefik (Helm `values.yaml`), TLS and the dashboard (**port 8090, internal only**) | ✅ C17 decision; C16 resolved |
| **A10** | **Request/correlation ID** | Header **`X-Request-ID`**. The gateway passes it on (or creates it); every service logs it as `request_id`; outbox events copy it to `correlation_id` | runbook §2.2, security-notes §4 |
| **A11** | **Observability** | Go services use **`prometheus/client_golang`** for `/metrics` (one shared helper in `core-api/internal/platform`). **Every custom metric starts with `swiftdrop_`** (e.g. Member 8's `swiftdrop_outbox_publish_failures_total`, `swiftdrop_dlq_messages_routed_total`, `swiftdrop_webhook_delivery_attempts_total`); names are listed in doc-04 §5. **No high-cardinality labels** (no tracking numbers, user IDs, emails, raw URLs). **Tracing:** correlation IDs in logs for the baseline; OpenTelemetry is optional, later | SRS §11.4; answers Member 4 U-45, Member 7 U-61 |
| **A12** | **CI** | `.github/` is Member 9's. Every member gives Member 9 their lint/test commands. Member 9 adds the jobs (Go, web, Python, gateway smoke, Playwright, infra checks) and pins tool versions | Engineering Standards §12.3 |
| **A13** | **Drafts and PRs** | Drafts go to `docs/module-spec-reviews` in your own `docs/drafts/member-XX/` folder; **never open draft PRs to `main`**. Real work goes on short branches from `main`, with at least one reviewer | `docs/drafts/README.md`; lesson from PRs #1–#4 |

## 2. Database agreements (Member 9 is the database coordinator)

| # | Topic | Agreement | Status |
| :-: | :--- | :--- | :--- |
| **D1** | **Migration tool** | golang-migrate. One-shot container locally (`migrate/migrate:v4.17.1`, as Members 4 and 7 already use); in Kubernetes **one Job per release** (`backoffLimit: 0`: a failed migration needs a person, not retries), image `core-api-migrations:<git-sha>` | ✅ C22 + Member 7's Job design |
| **D2** | **File names** | **Ratified: 6-digit sequential numbering** `000NNN_<module>_<change>.up.sql` / `.down.sql` (`000001` through `000011` initial wave), per AGENTS.md §7 and Master Audit. Guarantees deterministic ordering and cross-platform consistency. | ✅ Ratified |
| **D3** | **First-wave order** | The hub list (`docs/drafts/README.md` §4) is the one canonical order: `000001` users and roles (M1) · `000002` delivery zones (M7) · `000003` merchants and credentials (M4) · `000004` pricing rules (M2) · `000005` parcels and quotes (M2) · `000006` couriers (M3) · `000007` dispatch and delivery (M3) · `000008` outbox events (M8) · `000009` worker dedup and webhooks: `processed_events`, `webhook_deliveries` **and `notification_deliveries`** (M8; the hub list forgot this table) · `000010` idempotency keys (M2 / platform) · `000011` audit logs (M7 / platform). Every foreign key points at an earlier number. Whether `000005` keeps a `parcel_addresses` table or JSONB snapshots is Member 2's call | ✅ Ratified (hub list chosen by the DB coordinator 2026-10-28) |
| **D4** | **Stub tables** | **Stubs stay in your test folders.** Never put another member's table (e.g. a stub `users`) in the real `core-api/migrations/`. Members 4 and 7 both have stub `users`/audit migrations; the real `users` comes from Member 1, and `audit_logs` is created **once**, by its owner | ✅ Agreed |
| **D5** | **Ownership and review** | each owner writes only their own tables' migrations and updates `database-schema.md` in the same PR. **Member 9 reviews every migration** (CODEOWNERS); the owner of a referenced table is tagged. A merged migration is never edited | ✅ elected 2026-10-24 |
| **D6** | **DB login roles** | Created by Member 9 **once, before migrations** (bootstrap SQL). Names: **`swiftdrop_migrator`** (owns the schema; DDL), **`swiftdrop_app`** (core-api), **`swiftdrop_tracking_ro`** (tracking; read only), **`swiftdrop_relay`**, **`swiftdrop_worker`**, **`swiftdrop_reconciler`**. The AI service gets **no** DB login | ✅ Agreed |
| **D7** | **Default rights** | `swiftdrop_app` gets **SELECT, INSERT, UPDATE** on new tables by default. **DELETE is granted per table only when a module needs it.** Append-only tables (`audit_logs`, status history) never get UPDATE/DELETE | proposal (fits Member 4's "no DELETE" and Member 7's audit rule) |
| **D8** | **Connection budget** | Member 9 sets the pool size per service. **Postgres `max_connections` = 150** (memory limit raised from 512Mi to 1Gi). Pools: core-api 12 × 4 pods = 48 · tracking 5 × 4 = 20 · relay **5** · worker **10** × 3 pods = 30 · reconciler 2 → **105 used, about 45 spare** for migrations, admin and growth | ✅ C11; limit 150 decided by Member 9 (DB coordinator) 2026-10-25; relay/worker pools from Member 8 |
| **D9** | **Timeouts** | per-role defaults: `statement_timeout = 5s`, `lock_timeout = 2s` (Member 4 U-30); longer only for the migrator | proposal |
| **D10** | **DB driver** | **`pgx/v5`** with its built-in pool (Member 7 U-55), for all Go services, so pool settings are the same everywhere | ⏳ agree (Members 1–4, 8) |
| **D11** | **TLS inside the cluster** | `sslmode=disable` **inside** the cluster for the project (Postgres has no TLS today; `sslmode=require` would fail). Protection comes from NetworkPolicy and no public DB port. Written down as a known limitation | proposal |
| **D12** | **Seed data** | Seed is a binary in the core-api image (`/app/seed`, run by `make seed`). It **refuses to run when `APP_ENV=production`** (Member 7) | proposal |

## 3. Answers to questions other members asked Member 9

| From | Their item | Member 9's answer / proposal |
| :--- | :--- | :--- |
| Member 4 | U-08: how the worker reads webhook config | Worker reads the `merchant_webhook_targets` view (Member 4's name) with the `swiftdrop_worker` role, which has SELECT only on that view (D6) ✅ |
| Member 4 | U-21 / U-47 (Member 7 U-31): depguard per module | Yes. Member 9 adds the rules to `.golangci.yml`; each owner sends their module's rule |
| Member 4 | U-22: webhook secret encryption key | Agreed: AES-256-GCM with a `key_version` column, key from a K8s Secret shared by core-api and the worker (A5). This also solves Member 9's schema item DB11 |
| Member 4 | U-29: pepper rotation | Member 9 stores versioned pepper secrets; Member 4 owns the rotation procedure |
| Member 4 | U-30: DB timeouts, key prefix per environment | D9 ✅; `MERCHANT_API_KEY_PREFIX` goes in `core-api-merchants-config` (`sd_test_` locally) |
| Member 4 | U-33: integration DB in CI | **Testcontainers** for Go integration tests (isolated, parallel); Compose only for end-to-end |
| Member 4 | U-35: `-race` on Windows | Run race tests in CI (Linux); locally use the Docker command Member 4 documented |
| Member 4 / Member 7 | U-43: CI breaks because of `go.mod` files in draft folders | **Confirmed, and Member 9 will fix it.** CI runs only on `main`/`develop`, so it breaks when this branch merges into `main`. Fix: key the Go jobs on `core-api/go.mod` and ignore `docs/drafts/**` |
| Member 4 | U-45: shared metrics library | `prometheus/client_golang` (A11) |
| Member 4 | U-46: roles must exist before migrations | Yes: bootstrap SQL before the first migration (D6) |
| Member 4 | merchants retention CronJob | Member 9 deploys `core-api maintenance merchants-retention` as a daily CronJob (`concurrencyPolicy: Forbid`) |
| Member 7 | U-09: correlation header, domain, CORS | `X-Request-ID` (A10). Domain is pending the hosting vote; keep the `swiftdrop.example.com` placeholder |
| Member 7 | U-11: paths or subdomains | Paths on one domain ✅ (C17) |
| Member 7 | U-12 / U-48: dashboard port | **8090**, bound to localhost/internal only ✅ |
| Member 7 | U-33: where the route map lives | `deploy/gateway/README.md` (Member 7's living doc) |
| Member 7 | U-36: gateway access-log retention | **14 days**, readable only by cluster admins |
| Member 7 | U-40: migration numbering | 6-digit sequential numbers (D2, required by AGENTS.md §7); order per D3 |
| Member 7 | U-43: CI jobs | Member 9 adds `web-checks`, `gateway-smoke`, integration and E2E jobs, and pins golangci-lint and TruffleHog |
| Member 7 | U-46: proxies in front of Traefik | Cloudflare only (no cloud load balancer), so `ipStrategy.depth` = 1. `trustedIPs` keeps only the Cloudflare ranges plus the cluster pod network |
| Member 7 | U-55: DB driver | `pgx/v5` (D10) |
| Member 7 | U-57 / U-58: new env vars, one web API variable | Member 9 adds them to `environment-variables.md`. Web variable name: Members 6 and 7 pick one; Member 9 records it |
| Member 7 | U-59: core-api CPU/memory | Start with Member 7's guesses (100m / 128Mi request, 256Mi limit); Member 9 adjusts after a load test |
| Member 7 | U-60: origin TLS secret name | **`traefik-origin-tls`** |
| Member 7 | U-61: tracing | Correlation IDs only for the baseline (A11) |
| Member 7 | `PathPrefix(/api)` 404 router | Agreed; Member 7 adds it, Member 9 adds it to the smoke test |
| Member 10 | Dockerfile, port, probes, Compose | See conflicts **K1–K4** below |
| Member 10 | Playwright in CI | Yes: `npx playwright install --with-deps chromium` in the E2E job |
| Member 2 | Compose/CI test services, migration checks, evidence | Covered by A7, A12, D1–D5; Member 2 sends module test commands |
| Member 8 *(answers 2026-10-25)* | one `messaging` image; relay 8082, worker 8083 | ✅ agreed (A1, A3) |
| Member 8 | worker queues | ✅ `event-worker` consumes `q.notifications.worker` + `q.webhooks.dispatcher` in one Deployment. KEDA gets **two triggers**, `queueLength 10` each (resolves C7) |
| Member 8 | DLQ and outbox columns | ✅ DLQ `swiftdrop.dlq` (exchange `swiftdrop.dlq.exchange`); outbox columns from `database-schema.md` (`status`, `attempt_count`, `published_at`, `last_error`). ⏳ still to add: `correlation_id` (A10) and the `processed_events` dedup table |
| Member 8 | reconciler | ✅ messaging image, `/usr/local/bin/reconcile --stale-threshold=5m --batch-size=50`, every 10 min (resolves C10). Member 9 updates the CronJob manifest |
| Member 8 | `RABBITMQ_URL`, pools, metrics | ✅ A5, D8, A11 |
| Member 6 *(answers 2026-10-25)* | web build, API variable, Nginx | ✅ `npm run build` → `dist/`; **`VITE_API_BASE_URL=/api/v1`** (one image for every environment); Nginx on **3000** with SPA fallback (resolves C15 and the web part of C21) |
| Member 6 | web Dockerfile owner | ✅ **Member 9 owns `web/Dockerfile` + Nginx config**; Members 6 and 7 review (A3) |

## 4. Conflicts found in submitted drafts (needs a change)

| # | Member | In their draft | Agreed value | Who changes |
| :-: | :--- | :--- | :--- | :--- |
| **K1** | Member 10 | AI port **8003** | **8000** (A1: gateway, env-vars doc, runbook and ADR-001 all use 8000) | Member 10 *(or the team votes to move everything to 8003)* |
| **K2** | Member 10 | probes `/healthz`, `/readyz` | `/health/live`, `/health/ready`, `/metrics` (A2) | Member 10 |
| **K3** | Member 10 | Dockerfile: `USER appuser` (random UID); `HEALTHCHECK` uses `curl` (not in `python:3.11-slim`) | `USER 10001:10001`; a Python-based health check or rely on K8s probes (A3) | Member 10 |
| **K4** | Member 10 | "root `docker-compose.yml`"; new env vars (`LLM_BASE_URL`, `TRACKING_API_URL`, `AI_RATE_LIMIT_*`, `AI_SESSION_*`) | `deploy/docker-compose.yml` (A7); Member 9 adds the variables (A4) | Member 10 + Member 9 |
| **K5** | Member 7 | K8s probes `/livez`, `/readyz` | `/health/live`, `/health/ready` (A2) | Member 7 (proposal file) |
| **K6** | Member 7 | secret `postgres-credentials`, `postgres-owner-credentials`, role `swiftdrop_owner` | `postgres-secrets`, `postgres-migrator-secrets`, `swiftdrop_migrator` (A5, D6) | Member 7 + Member 9 |
| **K7** | Member 7 | `DATABASE_URL … sslmode=require` | `sslmode=disable` in-cluster (D11) | Member 9 |
| **K8** | Member 4 | role `swiftdrop_core_api`, `swiftdrop_tracking`; migrations run as superuser in its Compose file | `swiftdrop_app`, `swiftdrop_tracking_ro`; migrations as `swiftdrop_migrator` (D6) | Member 4 |
| **K9** | Members 4 and 7 | timestamp-named migrations, plus stub `users`/audit tables | 6-digit names `000003` / `000002` + `000011` (D2, D3); stubs stay in test folders (D4) | Members 4 and 7 |
| **K10** | Members 4 and 7 | separate Compose files (`compose.yml`, `docker-compose.m7.yml`) | merged into the one `deploy/docker-compose.yml` by Member 9 (A7) | Member 9 |
| **K11** | Member 9's own drafts | 6-digit migration numbers with reserved 000001–000011; migrate Job `backoffLimit: 2`; flat `.yml` files; role names | Resolved: 6-digit ratified (D2); Job `backoffLimit: 0` (D1); per-service `.yaml` folders (A8); `swiftdrop_reconciler` added (D6) | Member 9 ✅ |
| **K12** | Member 3 | migration `000003_dispatch_and_delivery`, which runs **before parcels and couriers exist**, but its tables have foreign keys to them | must run **after** parcels (M2) and couriers in the D3 order; file name per the D2 vote | Member 3 |
| **K13** | Member 3 | seed file inside `core-api/migrations/seeds/`, loaded with `psql` | seeds are not migrations: run them through `make seed` / the seed binary (D12) | Member 3 + Member 2 |
| **K14** | Member 3 | `/health/ready` also checks **Cloudflare R2** | core-api readiness = PostgreSQL only (A2); an R2 outage should fail photo uploads only, not take the whole core-api offline | Member 3 |
| **K15** | Member 3 | env names `STORAGE_DRIVER`, `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` | `environment-variables.md` updated to alias both `OBJECT_STORAGE_*` and `S3_*` in `integrations-secrets` | Member 9 ✅ |
| **K16** | Member 3 | `docker compose up -d postgres minio` (root Compose), DB URL `postgres:postgres@…/swiftdrop` | `make up` (A7); database `swiftdrop_db` with the `swiftdrop_app` login (D6) | Member 3 |
| **K17** | Member 8 | relay/worker health paths `/livez`, `/readyz` | `/health/live`, `/health/ready` (A2, group decision C1) | Member 8 ✅ (closed in Member 8's `CHANGE_REQUESTS.md`) |
| **K18** | Member 8 | image `swiftdrop/messaging:latest` | `ghcr.io/phantomtroupeead/swiftdrop/messaging:<git-sha>`, never `latest` (A3) | Member 8 ✅ (drafts use `${IMAGE_TAG}`); Member 9 ✅ CronJob manifest fixed 2026-10-28 |
| **K19** | Member 8 | gauge named `swiftdrop_outbox_pending_count` | Prometheus convention reserves `_count` for histograms; gauge standardized to `swiftdrop_outbox_pending_events` | Member 8 ✅ |

## 5. What each member agrees with Member 9 (checklist)

| Member | Must agree / provide | Draft submitted? |
| :--- | :--- | :-: |
| **M1 Umesh** | A1–A6, A10–A13 · real `users`/roles migrations `000001` (D3, D4) · JWT key delivery (`jwt-secrets`) · review of DB login roles (D6) | ✅ (v2.0.0 aligned) |
| **M2 Hansi** | A1–A13 · D1–D12 · seed command (D12) · core-api pool size (D8) · owner of `idempotency_keys` (proposed: Member 2) | ✅ |
| **M3 Kalani** | K12–K16 · A1–A6, A10–A13 · proof storage: **Cloudflare R2 in production, MinIO locally** (from Member 3's draft; this settles the storage part of C26). Member 9 creates the R2 bucket keys as secrets | ✅ (added 2026-10-24 by Poorna) |
| **M4 Nagitha** | K8, K9 · secret pattern A5 · webhook key shared with M8 · D6 role names | ✅ |
| **M5 Senuji** | A1–A6, A10–A13 · SSE `/api/v1/tracking/public` · pool size 5 · `swiftdrop_tracking_ro` | ✅ (v2.0.0 aligned) |
| **M6 Sajini** | A1, A3, A4, A7, A9, A12, A13 · build `dist/` · `VITE_API_BASE_URL=/api/v1` · Nginx on port 3000 | ✅ (8 docs submitted) |
| **M7 Bagya** | K5, K6, K9, K10 · A8 layout (adopts Member 7's) · A9 split · access-log retention 14 days | ✅ |
| **M8 Watti** | A1, A3, C7 (2 triggers), C10 (reconciler), `RABBITMQ_URL`, pool sizes (relay 5, worker 10) | ✅ (v2.0.0 aligned) |
| **M10 Kakkaya** | K1–K4 · E2E environment = Compose stack · Playwright CI job | ✅ |

## 6. Sign-off

| Member | Agree? (Yes / Yes with changes / No) | Notes | Date |
| :--- | :--- | :--- | :--- |
| M1 Umesh | Yes | Aligned with RS256 JWT, `jwt-secrets`, migration 000001 | 2026-10-25 |
| M2 Hansi | Yes | Aligned with integer cents, 000004/000005, pool size 12 | 2026-10-25 |
| M3 Kalani | Yes | Aligned with Cloudflare R2 / MinIO, migrations 000006/000007 | 2026-10-25 |
| M4 Nagitha | Yes | Aligned with AES-256-GCM webhook secret, migration 000003 | 2026-10-25 |
| M5 Senuji | Yes | Aligned with port 8081, pool size 5, swiftdrop_tracking_ro | 2026-10-25 |
| M6 Sajini | Yes | Aligned with Nginx 3000, VITE_API_BASE_URL, dist/ | 2026-10-25 |
| M7 Bagya | Yes | Aligned with Traefik 80/443, dashboard 8090, migration 000002 | 2026-10-25 |
| M8 Watti | Yes | Aligned with messaging image, KEDA 2 triggers, reconcile job | 2026-10-25 |
| M10 Kakkaya | Yes | Aligned with port 8000, stateless AI service, Playwright | 2026-10-25 |
