# Document 8 — Implementation, Deployment & Operations Guide

> **Status:** Draft / Proposed (For Review in Meeting 2)  
> **Author:** Member 4 — Merchant Integration  
> **Target Module / Location:** `core-api/internal/merchants/` (runs inside the `core-api` process)  
> **Key Upstream Dependencies:** Member 9 (Compose, Kubernetes, CI, secrets), Member 1 (JWT middleware, users, audit), Member 2 (parcel booking), Member 7 (gateway, admin routes), Member 8 (webhook delivery reads the view)  
> **Open Questions / Needs Clarification:** Inherits U-01 … U-42. New: U-43 … U-47 (see [§13](#13-new-open-questions)).

---

## 1. Deliverables

| Deliverable (spec) | Where | Verified |
|---|---|---|
| Module README | [`doc8-local/README.md`](./doc8-local/README.md) (graduates to `core-api/internal/merchants/README.md`) | — |
| Configuration example | [`doc8-local/.env.example`](./doc8-local/.env.example), [§4](#4-environment-variables) | Config tests + startup refusal test |
| Operational notes | §7–§11 of this document | — |
| **Demonstrable local setup** | [`doc8-local/`](./doc8-local/) — runnable `core-api` slice with Docker Compose | ✅ **49/49** end-to-end checks ([evidence](./doc8-local/evidence/demo-output.txt)) |
| Kubernetes needs | [`doc8-local/deploy/kubernetes/merchants-config-proposal.yaml`](./doc8-local/deploy/kubernetes/merchants-config-proposal.yaml) | YAML parses; not applied to a cluster |

### 1.1 What the demo proves (run 24 Oct 2026, fresh database)

| Area | Checks | Highlights |
|---|---|---|
| Health | 2 | `/livez`, `/readyz` with DB ping |
| Application & approval | 10 | 409 on second application, 422 on bad phone, 403 non-admin, 409 invalid transition, 409 stale version |
| API keys | 6 | ADR-003 format `sd_test_<8>_<32>`, `Cache-Control: no-store`, key rejected on management routes, query-string key ignored, tampered key 401, ambiguous credentials 400 |
| Orders (Member 2 stand-in) | 4 | Idempotency: replay returns same parcel + `Idempotent-Replay: true`; different body 409 |
| Webhooks & SSRF | 8 | Metadata IP, `localhost`, port 5432, **hostname resolving to a private Docker IP** all rejected; client secret rejected; secret shown once |
| Tenant isolation | 3 | Merchant B cannot delete A's webhook, revoke A's key, or see A's orders (404 / empty) |
| Lifecycle | 10 | Suspend blocks key on next request; reinstate restores; revoke idempotent; revoked key 401; disable revokes all keys; DISABLED terminal |
| Evidence | 6 | Exact ordered audit trail; no secrets in audit rows, DB, or logs; view excludes disabled merchant; metrics populated |

Also verified: graceful shutdown ([evidence](./doc8-local/evidence/shutdown-output.txt)), refusal to start on bad config ([evidence](./doc8-local/evidence/config-failure-output.txt)), unit tests with `-race` ([evidence](./doc8-local/evidence/unit-test-output.txt)), and **migrations applied by the real golang-migrate v4.17.1** (closes the Doc 5 caveat).

---

## 2. Language, Framework, Libraries, Versions

| Item | Version | Why |
|---|---|---|
| Go | **1.22** (CI pins 1.22; code avoids 1.23-only APIs such as `http.Request.Pattern`) | Team standard (`.github/workflows/ci.yml`) |
| HTTP | standard library `net/http` with Go 1.22 `ServeMux` method+path patterns | No framework needed; fewer dependencies |
| PostgreSQL driver | `github.com/jackc/pgx/v5` **v5.7.1** (`pgxpool`) | Same version as Member 7's slice; native PG error codes for constraint mapping |
| Logging | `log/slog` JSON handler | Matches runbook log format (`docs/operations/runbook.md` §2.1) |
| Crypto | stdlib `crypto/hmac`, `crypto/sha256`, `crypto/aes`+`cipher` (GCM), `crypto/rand`, `crypto/subtle` | No third-party crypto |
| JWT (stand-in) | stdlib RS256 verify | Replaced by Member 1's middleware |
| Metrics (stand-in) | tiny Prometheus text exporter (`internal/platform/obs`) | Until Member 9 picks the shared library (U-45) |
| Migrations | golang-migrate **v4.17.1** (`migrate/migrate:v4.17.1` image) | Repo standard (`local-development.md`) |
| Database | PostgreSQL **16** (`postgres:16-alpine`) | Repo standard |
| Container base | `golang:1.22-bookworm` (build), `gcr.io/distroless/static-debian12:nonroot` (run) | Static binary, no shell, non-root |

Direct Go dependencies: **one** (`pgx/v5`). Indirect: `pgpassfile`, `pgservicefile`, `puddle/v2`, `x/crypto`, `x/sync`, `x/text`.

---

## 3. Folder Structure and Entry Points

```text
doc8-local/                                  # moves into the repo root layout
├── compose.yml                              # postgres, migrate, seed, api, webhook-sink
├── .env.example                             # config template (§4)
├── scripts/  up.sh  demo.sh  shutdown-demo.sh  down.sh
├── deploy/kubernetes/merchants-config-proposal.yaml   # for Member 9
├── evidence/                                # saved run outputs
└── core-api/
    ├── Dockerfile  .dockerignore  go.mod  go.sum
    ├── cmd/
    │   ├── api/main.go        # server; `api -healthcheck` for Docker HEALTHCHECK
    │   ├── seed/main.go       # local demo users (refuses staging/production)
    │   └── devtoken/main.go   # local RS256 tokens (refuses production)
    ├── migrations/            # 000010 stubs (M1/M7 tables) + Member 4 000100..000400
    └── internal/
        ├── merchants/
        │   ├── domain/          # pure Go (status machine, key format, SSRF policy, profile rules) + tests
        │   ├── application/     # use cases + ports (Doc 3 §6.2)
        │   ├── infrastructure/  # pgx repos, crypto, DNS resolver, Member 2 booking STUB, Member 1 role STUB
        │   └── transport/       # HTTP handlers, auth middleware, error mapping
        └── platform/            # STAND-INS: config, auth (JWT), web, db, obs
```

| Entry point | Purpose | Run |
|---|---|---|
| `cmd/api` | HTTP server on `HTTP_ADDR` | `docker compose up api` or `go run ./cmd/api` with env set |
| `cmd/api -healthcheck` | Exit 0 if `/livez` is 200 (no shell needed in distroless) | Docker `HEALTHCHECK` |
| `cmd/seed` | Insert demo users idempotently | Compose `seed` service |
| `cmd/devtoken` | `-genkey DIR` makes RSA pair; otherwise prints a signed token | `./.dev/devtoken -key ./.dev/jwt_private.pem -role ADMIN` |

At integration, `cmd/api/main.go` wiring moves into `core-api/internal/app/wiring.go` (AGENTS.md §4.1) and the platform stand-ins are deleted in favour of the shared packages.

---

## 4. Environment Variables

The process **refuses to start** if any required value is missing or invalid, prints every problem at once, and never prints values ([evidence](./doc8-local/evidence/config-failure-output.txt)).

### 4.1 Shared (already in `docs/operations/environment-variables.md`)

| Variable | Used by merchants for | Safe local example |
|---|---|---|
| `APP_ENV` | Enables local-only relaxations; production forces `sd_live_` | `local` |
| `HTTP_ADDR` | Listen address | `:8080` |
| `LOG_LEVEL` | Log level | `info` |
| `DATABASE_URL` | PostgreSQL DSN (**secret**) | `postgres://swiftdrop:swiftdrop_dev@postgres:5432/swiftdrop_db?sslmode=disable` |
| `JWT_PUBLIC_KEY_PEM` | Verify access tokens (PEM text or file path) | `/run/keys/jwt_public.pem` |

### 4.2 New for the merchants module (to add to `environment-variables.md` §2)

| Variable | Description | Default | Required | Secret | Safe example |
|---|---|---|---|---|---|
| `JWT_ISSUER` | Expected `iss` claim (ADR-003 §3.3) | — | **Yes** | No | `http://localhost` |
| `MERCHANT_API_KEY_PREFIX` | `sd_live_` or `sd_test_`; must be `sd_live_` in production | `sd_test_` (non-prod), `sd_live_` (prod) | No | No | `sd_test_` |
| `MERCHANT_API_KEY_PEPPER` | HMAC pepper for key hashes, ≥ 32 chars (DD-02) | — | **Yes** | **Yes** | *(generated by `up.sh`)* |
| `MERCHANT_API_KEY_PEPPER_VERSION` | Version stored with each hash (U-29) | `1` | No | No | `1` |
| `WEBHOOK_SECRET_ENCRYPTION_KEY` | Base64 of 32 bytes; AES-256-GCM key for signing secrets (DD-05). **Shared with Member 8** | — | **Yes** | **Yes** | *(generated by `up.sh`)* |
| `WEBHOOK_SECRET_KEY_VERSION` | Version stored with each ciphertext (U-22) | `1` | No | No | `1` |
| `WEBHOOK_ALLOW_HTTP` | Accept `http://` webhook URLs; **refused unless `APP_ENV=local`** | `false` | No | No | `true` (local) |
| `WEBHOOK_ALLOWED_PRIVATE_HOSTS` | Comma list of hostnames allowed to resolve to private IPs; **refused unless `APP_ENV=local`** (U-44) | empty | No | No | `webhook-sink` |
| `MERCHANT_MAX_ACTIVE_KEYS` | BR-K-09 | `5` | No | No | `5` |
| `WEBHOOK_MAX_ENDPOINTS_PER_MERCHANT` | BR-W-09 | `10` | No | No | `10` |
| `SHUTDOWN_DRAIN_SECONDS` | Time `/readyz` returns 503 before closing (§9) | `3` | No | No | `2` (local), `5` (K8s) |
| `SHUTDOWN_TIMEOUT_SECONDS` | Max wait for in-flight requests | `15` | No | No | `15` |

Generate secrets:

```bash
head -c 48 /dev/urandom | base64      # MERCHANT_API_KEY_PEPPER
head -c 32 /dev/urandom | base64      # WEBHOOK_SECRET_ENCRYPTION_KEY
```

---

## 5. Local Run, Build, and Test

### 5.1 Prerequisites

Docker Desktop (running), Go 1.22+, bash (Git Bash on Windows), curl. No `make` required (Windows machines in the team may not have it).

### 5.2 Run

```bash
cd docs/drafts/member-04-merchants/doc8-local
./scripts/up.sh             # creates .dev/ keys + .env, builds image, starts stack, waits for /readyz
./scripts/demo.sh           # 49 checks (fresh DB: ./scripts/down.sh --wipe && ./scripts/up.sh first)
./scripts/shutdown-demo.sh  # graceful shutdown demo
./scripts/down.sh [--wipe]
```

Startup order (Compose `depends_on`): `postgres` (healthy) → `migrate` (exit 0) → `seed` (exit 0) → `api` (healthy). `webhook-sink` starts independently.

### 5.3 Build

```bash
cd core-api
go build ./...                                # host build
docker build -t m4-core-api:local .           # image (31 MB, user 10001:10001)
```

### 5.4 Test

```bash
cd core-api
go vet ./... && go test -count=1 -cover ./...                        # quick
docker run --rm -v "$PWD:/src" -w /src golang:1.22-bookworm \
  go test -race -count=1 ./...                                        # -race on Windows (needs CGO, see Doc 6 U-35)
cd .. && ./scripts/demo.sh                                            # end-to-end
```

Starter suites from Document 6 (`doc6-tests/`) remain the reference for domain and migration tests.

---

## 6. Docker Build and Health Checks

| Requirement | Implementation |
|---|---|
| Multi-stage, small | `golang:1.22-bookworm` build → `distroless/static-debian12:nonroot` run; **31 MB** |
| Static binary | `CGO_ENABLED=0`, `-trimpath -ldflags="-s -w"` |
| Non-root (SRS SEC-016) | `USER 10001:10001` (verified with `docker inspect`) |
| No secrets in image | `.dockerignore` excludes `.env`, `*.pem`, tests; config only from env |
| Pinned versions | All images pinned (`postgres:16-alpine`, `migrate/migrate:v4.17.1`, `busybox:1.36`, `golang:1.22-bookworm`) |
| Health check without shell | `HEALTHCHECK CMD ["/app/api","-healthcheck"]` → GET `/livez`; exits 1 when no server (verified) |
| Runtime hardening (Compose) | `read_only: true`, `cap_drop: [ALL]`, `no-new-privileges`, port bound to `127.0.0.1` |

### Health endpoints (match `runbook.md` §1)

| Endpoint | Meaning | Response |
|---|---|---|
| `GET /livez` | Process is up (never checks dependencies, so a DB outage does not restart pods) | `200 {"status":"UP"}` |
| `GET /readyz` | DB ping (1 s timeout) and not draining | `200 {"status":"READY","database":"UP"}` / `503 {"status":"NOT_READY","database":"DOWN"}` / `503 {"status":"DRAINING"}` |
| `GET /metrics` | Prometheus text | counters (§8.2) |

`/metrics`, `/livez`, `/readyz` must **not** be routed through the public gateway (Member 7).

---

## 7. Kubernetes Deployment

The merchants module has **no separate Deployment**: it ships inside `core-api` (modular monolith). Member 9 owns `deploy/kubernetes/`; Member 4 supplies the needs below as a [proposal file](./doc8-local/deploy/kubernetes/merchants-config-proposal.yaml).

| Topic | Merchants requirement |
|---|---|
| ConfigMap | `core-api-merchants-config` (non-secret vars from §4.2) |
| Secret | `core-api-merchants-secrets`: `MERCHANT_API_KEY_PEPPER`, `WEBHOOK_SECRET_ENCRYPTION_KEY`. The encryption key is also mounted read-only into Member 8's worker |
| Service | Existing `core-api` ClusterIP on 8080; nothing new |
| Probes | liveness `/livez` (10 s, 3 failures); readiness `/readyz` (3 s, 1 failure); startup `/livez` (2 s × 15) |
| Security context | `runAsNonRoot`, `runAsUser: 10001`, `readOnlyRootFilesystem`, drop ALL capabilities |
| Resources (merchants share) | request 100m CPU / 64 Mi; limit 256 Mi memory. Module is stateless and light; auth is one indexed query |
| Termination | `terminationGracePeriodSeconds: 30` ≥ drain (5 s) + shutdown timeout (20 s) |
| Scaling | Stateless; scales with `core-api` replicas (≥ 2 for availability). No sticky sessions. The only per-replica state is the `last_used_at` throttle map (harmless if lost) and the Member 2 stand-in, which disappears at integration |
| Network | Egress DNS only (registration-time resolution). Merchants never connects to merchant URLs — Member 8 does |
| Migrations | Run by Member 9's `core-api-migrate` Job before rollout (`deployment-guide.md` step 7) |
| DB role for worker | `swiftdrop_worker` must exist **before** migration `000400` so the view grant applies (otherwise the grant is skipped silently) |

---

## 8. Logging, Metrics, Tracing, Alerts

### 8.1 Logs

One JSON line per request (`docs/operations/runbook.md` §2.1 format):

```json
{"time":"2026-10-24T13:29:30Z","level":"INFO","msg":"http_request","service":"core-api","module":"merchants",
 "request_id":"req_4e22…","method":"POST","route":"POST /api/v1/merchant/orders","status":201,
 "duration_ms":3.2,"merchant_id":"3f0c…","key_prefix":"Ab3dE9xK"}
```

- Allow-listed fields only (Doc 7 §4): no headers, no query strings, no bodies, no tokens, no keys, no secrets. **Verified** by demo check 48.
- `route` is the matched pattern (low cardinality), not the raw path.
- `X-Request-ID` is accepted (≤ 64 chars, no CR/LF) or generated, echoed in the response, written to audit rows and error bodies as `correlation_id`.

### 8.2 Metrics (implemented)

| Metric | Labels | Use |
|---|---|---|
| `merchant_api_auth_total` | `result` = ok, invalid, revoked, expired, merchant_not_approved, error | Brute force / wrong pepper (Doc 7 §11.2) |
| `webhook_ssrf_blocked_total` | — | SSRF probing |
| `merchant_status_changes_total` | `action` | Admin activity |
| `http_requests_total` | `route`, `status` | Traffic and error rate |
| `http_request_duration_seconds_sum` | `route` | Mean latency (with `_total`) |

Histograms (p95) need the shared metrics library (U-45).

### 8.3 Tracing

**Not implemented in the slice.** Proposal: OpenTelemetry via the shared `OTEL_EXPORTER_OTLP_ENDPOINT`; one span per use case, child spans for the DB transaction, DNS lookup, and the Member 2 booking call; `request_id` as a span attribute. Depends on Member 9's tracing setup (U-45).

### 8.4 Alerts

The five alert rules in Doc 7 §11.2 (`MerchantAuthFailureSpike`, `MerchantAuthFailuresSinglePrefix`, `WebhookSSRFBlocks`, `MerchantKeyCreationBurst`, `MerchantInvariantViolation`) map directly to the metrics above. Add: `CoreAPINotReady` (readiness failing > 2 min) — owned by Member 9.

---

## 9. Startup and Shutdown Behaviour

**Startup:** load + validate config (exit 1 on any problem) → parse JWT public key → create DB pool (statement timeout 5 s, lock timeout 2 s) → register routes → listen. The pool connects lazily; `/readyz` stays 503 until the DB answers, so Kubernetes will not route traffic early.

**Shutdown** (SIGTERM / SIGINT), verified ([evidence](./doc8-local/evidence/shutdown-output.txt)):

```mermaid
sequenceDiagram
    participant K as Kubernetes / Docker
    participant A as core-api
    participant LB as Service endpoints
    K->>A: SIGTERM
    A->>A: log "shutdown started", readyz returns 503 DRAINING
    LB->>A: readiness probe fails
    LB->>LB: remove pod from endpoints
    A->>A: wait SHUTDOWN_DRAIN_SECONDS
    A->>A: http.Server.Shutdown (finish in-flight, up to SHUTDOWN_TIMEOUT_SECONDS)
    A->>A: close DB pool, log "shutdown complete", exit 0
```

In-flight transactions either commit fully or roll back (PostgreSQL atomicity); idempotent order retries make client retries safe.

---

## 10. External Dependencies

| Dependency | Needed for | If unavailable |
|---|---|---|
| PostgreSQL 16 | Everything | `/readyz` 503; requests 5xx with correlation ID; no partial writes |
| DNS (cluster resolver) | Webhook registration / URL change only | `422 WEBHOOK_URL_UNRESOLVABLE` after 2 s; other features unaffected |
| Member 1 JWT keys | All JWT routes | Startup refuses if key missing |
| Member 2 booking | Merchant orders | Orders fail; keys/webhooks unaffected (stand-in locally) |
| Member 8 worker | Webhook delivery | Not a runtime dependency of merchants (reads the view independently) |
| Gateway (Member 7) | Public routing, rate limits, TLS | Module still serves on cluster IP |

---

## 11. Troubleshooting

| Symptom | Likely cause | Check / fix |
|---|---|---|
| Container exits immediately, log `invalid configuration:` | Missing or invalid env var | The message lists every bad variable; fix `.env` / Secret |
| `/readyz` 503 `database DOWN` | DB unreachable / wrong `DATABASE_URL` | `docker compose logs postgres`; `psql` on `localhost:55432` |
| All merchant API calls 401 `API_KEY_INVALID` after a deploy | Pepper changed or wrong `MERCHANT_API_KEY_PREFIX` for the environment | Compare secret version; `merchant_api_auth_total{result="invalid"}` spike (Doc 7 FM-06) |
| Keys rejected with `API_KEY_INVALID` locally | Using an `sd_live_` key against `sd_test_` config (or vice versa) | Prefix must match `MERCHANT_API_KEY_PREFIX` |
| Webhook registration `422 WEBHOOK_URL_UNRESOLVABLE` | Host has no DNS record / container has no DNS | `docker compose exec api` cannot run shell (distroless) — test from another container: `docker run --rm --network m4-merchants_default busybox nslookup <host>` |
| Local webhook to `webhook-sink` rejected `SSRF_PROHIBITED_URL` | `WEBHOOK_ALLOWED_PRIVATE_HOSTS` not set or `APP_ENV` not `local` | Set both in `.env` (local only) |
| `409 MERCHANT_VERSION_CONFLICT` in admin UI | Another admin changed the merchant | Reload merchant, retry with new `version` |
| Merchant says webhooks stopped | Merchant suspended/disabled, endpoint disabled/deleted, or delivery failing | `SELECT * FROM merchant_webhook_targets WHERE merchant_id=…`; then Member 8's `webhook_deliveries` (runbook §5) |
| `Grant` on `merchant_webhook_targets` missing | `swiftdrop_worker` role created after migration `000400` | Re-run `GRANT SELECT ON merchant_webhook_targets TO swiftdrop_worker` (U-46) |
| `migrate` reports dirty version | Failed migration | `runbook.md` §3.3; `migrate … force <version>` after fixing |
| Trace a request | — | Find `request_id` in the response header / error body → `kubectl logs -l app=core-api \| grep <id>` → `audit_logs.request_id` |

Useful SQL:

```sql
-- merchant overview
SELECT id, business_name, status, version, approved_at FROM merchants WHERE id = $1;
-- keys (never shows secrets)
SELECT key_prefix, label, scopes, revoked_at, expires_at, last_used_at FROM merchant_api_credentials WHERE merchant_id = $1;
-- audit trail
SELECT created_at, action, actor_id, new_state, request_id FROM audit_logs WHERE target_id = $1 ORDER BY created_at;
```

---

## 12. Known Limitations

| # | Limitation | Plan |
|---|---|---|
| L-1 | Platform packages (config, JWT, web, db, metrics) are **stand-ins** | Replace with shared packages at integration |
| L-2 | Parcel booking is an **in-memory stand-in**; idempotency resets on restart | Member 2 adapter (Doc 4 §5.1) |
| L-3 | Admin routes served by the merchants slice | Member 7 admin module calls `AdminService` (U-10) |
| L-4 | Not implemented: profile `PATCH`, webhook `PATCH`/`rotate-secret`/`GET one`, `GET /merchant/orders/{id}`, retention + reconciliation jobs | Next increment; specified in Docs 2, 4, 5, 7 |
| L-5 | No unit tests yet for `application`/`transport`/`infrastructure` packages; covered end-to-end by the demo only | Add L2/L4/L3 tests per Doc 6 §4 |
| L-6 | No tracing; metrics without histograms | U-45 |
| L-7 | Role grant writes to `user_roles_stub`; JWT `tenant_id` comes from `devtoken` | Member 1 integration (U-15) |
| L-8 | Client IP in audit rows is the direct peer; behind Traefik it must come from the trusted proxy header | Member 7 gateway config + shared middleware |

---

## 13. New Open Questions

| ID | Question | Proposal | Needs |
|---|---|---|---|
| U-43 | **CI risk:** `.github/workflows/ci.yml` runs Go lint/tests whenever *any* `go.mod` exists, then `cd core-api && go test ./...`. `core-api/` has no `go.mod` yet, while draft folders (Member 7's and this one) contain `go.mod` files. The test step may fail on this branch. | Scope CI to `core-api/` only when `core-api/go.mod` exists, and exclude `docs/drafts/**` from Go jobs. | M9 |
| U-44 | New env var `WEBHOOK_ALLOWED_PRIVATE_HOSTS` (local only) is required so a local webhook receiver works despite the SSRF policy (its Docker IP is private). Doc 5 §11.1's seed URL would otherwise be blocked. | Accept; config refuses it outside `APP_ENV=local`. Add to `environment-variables.md`. | M8, M9 |
| U-45 | Shared metrics/tracing library (Prometheus client, OpenTelemetry) for all Core API modules. | Member 9 picks; merchants replaces `platform/obs`. | M9 |
| U-46 | Ordering: `swiftdrop_worker` DB role must exist before migration `000400`, or the grant is skipped. | M9 creates roles in an init step before migrations; or a later migration re-applies grants. | M9 |
| U-47 | `.golangci.yml` already enforces `domain`/`application` purity via depguard (partly resolves U-21). Should it also forbid cross-module imports of internals (Doc 3 §7)? | Yes; add rule per module. | M9, team |
