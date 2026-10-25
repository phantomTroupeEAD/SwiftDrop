# Team Agreements with Member 9 (DevOps, Kubernetes & Observability + Database Coordinator)

> **Status:** Draft / Proposed (For Review in Team Meeting 2)  
> **Author:** Member 9 — Minidu Dhananjana  
> **Purpose:** One page of **shared rules** every member must agree with Member 9, so that all 10 services build, run, deploy and migrate together. It also answers the questions other members asked Member 9 in their drafts, and lists the places where submitted drafts disagree with team decisions.  
> **Based on:** the group decisions of 2026-10-24, the repo docs (SRS, ADR-001/002, AGENTS.md, `docs/operations/*`, `database-schema.md`) and the drafts submitted on this branch by Members 2, 4, 7 and 10 (checked 2026-10-25).  
> **How to reply:** tick your row in §6, or write your objection next to the item number (e.g. "A5: I disagree because…").

---

## 1. Agreements for ALL members

| # | Topic | Agreement | Source / status |
| :-: | :--- | :--- | :--- |
| **A1** | **Service names and ports** | `core-api` **8080** · `tracking-api` **8081** · `ai-assistant` **8000** · `web` **3000** · `outbox-relay` / `event-worker`: small internal health port (M8 to name). The same names are used in Compose, Kubernetes and Traefik | ADR-001, `dynamic-routes.yml`, `environment-variables.md` |
| **A2** | **Health and metrics paths** | Every HTTP service exposes **`/health/live`** (process only, never checks the DB), **`/health/ready`** (checks its own dependencies; returns 503 when one is down) and **`/metrics`**. **Internal only**: never routed by Traefik | ✅ group decision C1 (SRS §6.2) |
| **A3** | **Dockerfile rules** | multi-stage build · pinned base image (no `latest`) · **`USER 10001:10001`** · no secrets inside the image · `.dockerignore` · `EXPOSE` the A1 port · JSON logs to stdout · handle SIGTERM. A health check must not use tools the image doesn't contain (e.g. `curl` is **not** in `python:*-slim` or distroless images) | security-notes §6, SRS SEC-016 |
| **A4** | **Env vars** | every variable your service reads must be listed in `docs/operations/environment-variables.md` and the root `.env.example` (placeholders only). New variables: send them to Member 9, who updates both in one PR | SRS §11.1–11.3 |
| **A5** | **Secrets (Kubernetes)** | Shared names: `postgres-secrets` (app `DATABASE_URL`), `postgres-migrator-secrets` (migration URL), `rabbitmq-secrets`, `jwt-secrets` (private key), `integrations-secrets` (Google, LLM, email/SMS, object storage). **Module-specific secrets** use the pattern `<service>-<module>-secrets` (e.g. `core-api-merchants-secrets`). A key needed by **two services** (e.g. `WEBHOOK_SECRET_ENCRYPTION_KEY`, used by core-api and the worker) goes into one shared secret that both mount. Never commit real values | ✅ C4 decision + this proposal for the new cases |
| **A6** | **Non-secret config** | One ConfigMap per service: `<service>-config`. Module-specific ConfigMaps use the pattern `<service>-<module>-config` | proposal |
| **A7** | **Local stack** | One Compose file: **`deploy/docker-compose.yml`**, run through the root **`Makefile`**: `make up`, `make migrate-up`, `make seed`, `make test-e2e`. Members send their Compose needs as proposals; Member 9 merges them into the one file. No per-member Compose files in the final repo | ✅ C3 + C22 decisions |
| **A8** | **Kubernetes files** | Member 9 owns `deploy/kubernetes/`. **Layout: one folder per service** (`deploy/kubernetes/core-api/deployment.yaml`, `service.yaml`, …), extension **`.yaml`**. The existing flat `.yml` files move into this layout in one Member 9 PR. Members send needs as proposal files (as Members 4 and 7 already did) | proposal (adopts Member 7's layout) |
| **A9** | **Gateway** | Path-based routing on one domain: `/api/v1/tracking` → tracking-api, `/api/v1/ai` → ai-assistant, `/api/v1` → core-api, `/` → web. Member 7 writes the routes and middlewares; Member 9 installs Traefik (Helm `values.yaml`), TLS and the dashboard (**port 8090, internal only**) | ✅ C17 decision; C16 resolved |
| **A10** | **Request/correlation ID** | Header **`X-Request-ID`**. The gateway passes it on (or creates it); every service logs it as `request_id`; outbox events copy it to `correlation_id` | runbook §2.2, security-notes §4 |
| **A11** | **Observability** | Go services use **`prometheus/client_golang`** for `/metrics` (one shared helper in `core-api/internal/platform`). Metric names follow Member 9's naming list (doc-04 §5). **No high-cardinality labels** (no tracking numbers, user IDs, emails, raw URLs). **Tracing:** correlation IDs in logs for the baseline; OpenTelemetry is optional, later | SRS §11.4; answers Member 4 U-45, Member 7 U-61 |
| **A12** | **CI** | `.github/` is Member 9's. Every member gives Member 9 their lint/test commands. Member 9 adds the jobs (Go, web, Python, gateway smoke, Playwright, infra checks) and pins tool versions | Engineering Standards §12.3 |
| **A13** | **Drafts and PRs** | Drafts go to `docs/module-spec-reviews` in your own `docs/drafts/member-XX/` folder; **never open draft PRs to `main`**. Real work goes on short branches from `main`, with at least one reviewer | `docs/drafts/README.md`; lesson from PRs #1–#4 |

## 2. Database agreements (Member 9 is the database coordinator)

| # | Topic | Agreement | Status |
| :-: | :--- | :--- | :--- |
| **D1** | **Migration tool** | golang-migrate. One-shot container locally (`migrate/migrate:v4.17.1`, as Members 4 and 7 already use); in Kubernetes **one Job per release** (`backoffLimit: 0`: a failed migration needs a person, not retries), image `core-api-migrations:<git-sha>` | ✅ C22 + Member 7's Job design |
| **D2** | **File names** | **Proposal: timestamps** `YYYYMMDDHHMMSS_<module>_<change>.up.sql` / `.down.sql` (golang-migrate's default format), instead of the 6-digit numbers in AGENTS.md §7. Two members already use timestamps, and Member 7 proposed it (U-40); timestamps never clash between 10 people. AGENTS.md §7 gets a one-line update after the vote | **⏳ vote needed** |
| **D3** | **First-wave order** | Tables must be created in foreign-key order: identity (M1) → zones (M7) → merchants (M4) → couriers (M3) → pricing rules (M2) → parcels (M2) → quotes (M2) → dispatch/delivery (M3) → events (M8) → idempotency → audit. With timestamps, Member 9 hands out one **time block per module** so the order is right | proposal |
| **D4** | **Stub tables** | **Stubs stay in your test folders.** Never put another member's table (e.g. a stub `users`) in the real `core-api/migrations/`. Members 4 and 7 both have stub `users`/audit migrations; the real `users` comes from Member 1, and `audit_logs` is created **once**, by its owner | ⏳ agree |
| **D5** | **Ownership and review** | each owner writes only their own tables' migrations and updates `database-schema.md` in the same PR. **Member 9 reviews every migration** (CODEOWNERS); the owner of a referenced table is tagged. A merged migration is never edited | ✅ elected 2026-10-24 |
| **D6** | **DB login roles** | Created by Member 9 **once, before migrations** (a bootstrap SQL, as in Member 7's draft). Names (merging the three proposals): **`swiftdrop_migrator`** (owns the schema; DDL), **`swiftdrop_app`** (core-api), **`swiftdrop_tracking_ro`** (tracking; read only), **`swiftdrop_relay`**, **`swiftdrop_worker`**. The AI service gets **no** DB login | ⏳ agree (Members 4 and 7 use different names today) |
| **D7** | **Default rights** | `swiftdrop_app` gets **SELECT, INSERT, UPDATE** on new tables by default. **DELETE is granted per table only when a module needs it.** Append-only tables (`audit_logs`, status history) never get UPDATE/DELETE | proposal (fits Member 4's "no DELETE" and Member 7's audit rule) |
| **D8** | **Connection budget** | Member 9 sets the pool size per service, so the total stays ≤ Postgres `max_connections` (100). Example: core-api 12 × 4 pods, tracking 5 × 4, relay 4, worker 3 × 3 | ✅ C11 (numbers to confirm) |
| **D9** | **Timeouts** | per-role defaults: `statement_timeout = 5s`, `lock_timeout = 2s` (Member 4 U-30); longer only for the migrator | proposal |
| **D10** | **DB driver** | **`pgx/v5`** with its built-in pool (Member 7 U-55), for all Go services, so pool settings are the same everywhere | ⏳ agree (Members 1–4, 8) |
| **D11** | **TLS inside the cluster** | `sslmode=disable` **inside** the cluster for the project (Postgres has no TLS today; `sslmode=require` would fail). Protection comes from NetworkPolicy and no public DB port. Written down as a known limitation | proposal |
| **D12** | **Seed data** | Seed is a binary in the core-api image (`/app/seed`, run by `make seed`). It **refuses to run when `APP_ENV=production`** (Member 7) | proposal |

## 3. Answers to questions other members asked Member 9

| From | Their item | Member 9's answer / proposal |
| :--- | :--- | :--- |
| Member 4 | U-08: how the worker reads webhook config | Worker reads the `merchant_webhook_targets` view with the read-only `swiftdrop_worker` role (D6) ✅ |
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
| Member 7 | U-40: migration numbering | Timestamps, proposed in D2 (vote) |
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
| **K9** | Members 4 and 7 | timestamp-named migrations, plus stub `users`/audit tables | D2 vote; stubs stay in test folders (D4) | Members 4 and 7 |
| **K10** | Members 4 and 7 | separate Compose files (`compose.yml`, `docker-compose.m7.yml`) | merged into the one `deploy/docker-compose.yml` by Member 9 (A7) | Member 9 |
| **K11** | Member 9's own drafts | 6-digit migration numbers with reserved 000001–000011; migrate Job `backoffLimit: 2`; flat `.yml` files; role names | update after the votes on D2, D1, A8, D6 | Member 9 |

## 5. What each member agrees with Member 9 (checklist)

| Member | Must agree / provide | Draft submitted? |
| :--- | :--- | :-: |
| **M1 Umesh** | A1–A6, A10–A13 · real `users`/roles migrations first (D3, D4) · JWT key delivery (`jwt-secrets` private key, public key as file mount) · review of the DB login roles (D6) · Google callback path | ❌ |
| **M2 Hansi** | A1–A13 · D1–D12 · seed command (D12) · core-api pool size (D8) · owner of `idempotency_keys` (proposed: Member 2) | ✅ |
| **M3 Kalani** | A1–A6, A10–A13 · migrations in D3 order · proof storage choice (MinIO locally; K8s option) | ❌ |
| **M4 Nagitha** | K8, K9 · secret pattern A5 · webhook key shared with M8 · D6 role names | ✅ |
| **M5 Senuji** | A1–A6, A10–A13 · SSE and public tracking paths · Redis need · HPA range · `swiftdrop_tracking_ro` | ❌ |
| **M6 Sajini** | A1, A3, A4, A7, A9, A12, A13 · web build command and `VITE_*` names · Nginx on port 3000 | ❌ (the first M6 draft PR, #1, was reverted) |
| **M7 Bagya** | K5, K6, K9, K10 · A8 layout (adopts Member 7's) · A9 split · access-log retention 14 days | ✅ |
| **M8 Watti** | A1–A6, A10–A13 · relay/worker health port · KEDA queue name · DLQ name · reconciler command · outbox table columns · `processed_events` · core-api Dockerfile lead | ❌ |
| **M10 Kakkaya** | K1–K4 · E2E environment = Compose stack · Playwright CI job | ✅ |

## 6. Sign-off

| Member | Agree? (Yes / Yes with changes / No) | Notes | Date |
| :--- | :--- | :--- | :--- |
| M1 Umesh | | | |
| M2 Hansi | | | |
| M3 Kalani | | | |
| M4 Nagitha | | | |
| M5 Senuji | | | |
| M6 Sajini | | | |
| M7 Bagya | | | |
| M8 Watti | | | |
| M10 Kakkaya | | | |
