# Document 8 — Implementation, Deployment & Operations Guide

> **Status:** Draft / Proposed (For Review in Meeting 2)  
> **Author:** Member 7 — Dispatcher/Admin UI, Courier Web, Traefik Gateway  
> **Target Module / Location:** `web/src/features/dispatcher-admin/` (ops + courier screens), `core-api/internal/admin/` (admin module), `deploy/gateway/traefik/` (gateway)  
> **Key Upstream Dependencies:** Member 1 (real sign-in), Member 9 (Compose, Kubernetes, Helm, observability), Member 8 (events)  
> **Open Questions / Needs Clarification:** Section 15. Still open: U-55 driver library, U-56 shared error package, U-57 new env vars, U-58 frontend variable name (Vite port now fixed at 3000), U-59 real CPU and memory numbers, U-60 TLS secret name, U-61 tracing, U-62 two-key rotation. U-48 (dashboard port) closed by R-10: 8090

**Module:** Dispatcher/Admin/Courier Frontend, Admin Backend Module & API Gateway (Member 7) **Project:** SwiftDrop Parcel Delivery & Courier Management System **Based on:** Documents 1 v4, 2 v3, 3 v2, 5 v2, 6 v2 and 7 v2; Member 7 alignment guide; Member 9 agreements (A2, A5, A7, A9, A10, D6, D7, K10); docs audit (26 Oct); docs/operations/environment-variables.md, local-development.md, deployment-guide.md, runbook.md; deploy/README.md; deploy/gateway/README.md; AGENTS.md sections 4 and 9; web/README.md; .github/workflows/ci.yml **Status:** Draft v2 (26 Oct 2026). v2 follows the team decisions: `/health/live` and `/health/ready`, Secrets `postgres-secrets` and `postgres-migrator-secrets`, role `swiftdrop_migrator`, migrations `000002` and `000011`, no own Compose file, text zone ids with no money, plus the public zone list, merchant admin routes and the delivery report. Items marked **\[TBD\]** wait for a teammate or a team decision. New open questions start at U-55 (section 15). **Date:** 24 October 2026 (revised 26 October 2026)

## 1. How to read this document

This document answers: **how does another developer run, debug, configure and maintain my module?**

It comes with a **working local setup** in `doc8-local/`. It is laid out like the team repo, so files can be moved across later. I ran everything below again on 26 October 2026:

| What | Result | Evidence file |
| --- | --- | --- |
| `make up`: real PostgreSQL 16, roles, 2 migrations (`000002`, `000011`), seed, core-api, Traefik 3.1.7 | Ready in about 3 seconds | `evidence/01-make-up.txt` |
| `make demo`: 51 checks through the gateway (access, public zones, zone rules, merchants, reports, audit, gateway, metrics, logs) | **51 passed** | `evidence/02-demo.txt` |
| `make shutdown-demo`: SIGTERM while requests keep coming | `/health/ready` 503 at once, all 20 requests answered 200, clean exit, restart | `evidence/03-shutdown.txt` |
| `make test-db`: Go tests, including race tests on the real database | **46 passed** (including the 4 real-database tests), runs repeat cleanly | `evidence/04-go-tests.txt` |
| Same race test with the advisory lock removed (experiment) | **Fails**: 2 or 3 overlapping zones saved. So the lock is really needed | section 12.2 |
| `make gateway-check`: Docker/Kubernetes parity, Kubernetes schema check, 25 gateway security checks | Parity OK (11 middlewares, 3 transports), **23 of 23** resources valid, **25 of 25** checks pass | `evidence/06-gateway-check.txt` |
| Same 25 checks on today's repo gateway files | 8 pass, 17 fail (the gaps of Doc 7 section 14) | `evidence/09-gateway-repo-config.txt` |
| Docker images | **Not built**: no Docker daemon here. The binary was built exactly as the Dockerfile does it, and its self-healthcheck works | section 6 |

**What this is not:** the full SwiftDrop. Other members' services are replaced by small stand-ins, and sign-in uses a local test-token tool (Member 1 owns real sign-in). The ops and courier **screens are not built yet**, because they need endpoints from Members 1 to 5. Section 5.5 says how they will run.

v2 follows the 26 Oct decisions (Doc 1 section 11.4). The zone now has a text id chosen by the admin (`ZONE_COLOMBO_01`), a short code (`CMB-01`), a name of 1 to 120 characters, an optional description, and **no money**. There is no Compose file of my own any more: my needs for Member 9's shared file are in `proposals/compose-needs-for-member-09.md` (section 6.3).

Every picture is an image first, with its Mermaid text in a fold-out box underneath. Tags: **\[Repo\]** = in the repository today, **\[Proposed\]** = my proposal, **\[TBD\]** = not decided.

## 2. Languages, frameworks, libraries and versions

| Part | Technology | Version | Source |
| --- | --- | --- | --- |
| Admin backend | Go | **1.22** (CI pins it; local builds also tested with 1.24) | \[Repo\] ci.yml, local-development.md |
| HTTP routing | Go standard `net/http` with 1.22 method patterns (`PATCH /api/v1/admin/zones/{id}`, `POST /api/v1/admin/merchants/{id}/{action}`) | stdlib | \[Proposed\] no router library needed |
| Logs | Go standard `log/slog`, JSON | stdlib | \[Repo\] runbook 2.1 format |
| Metrics | Small Prometheus text writer in `platform/obs` | stdlib | \[Proposed\] avoids a new dependency |
| PostgreSQL driver | `github.com/jackc/pgx/v5` | **5.7.1** | **\[TBD U-55\]**: the only outside library; AGENTS.md says "no new dependencies" |
| Database | PostgreSQL | **16** (local 16.15; image `postgres:16.4-alpine3.20`) | \[Repo\] deploy/README |
| Migrations | golang-migrate CLI | **4.17.1**, built with `-tags postgres` | \[Repo\] local-development.md ("v4.17+") |
| Gateway | Traefik | **3.1.7** (image `traefik:v3.1.7`) | \[Repo\] ADR-001 (v3.1) |
| Container base | `golang:1.22.12-alpine3.21` to build; `gcr.io/distroless/static-debian12:nonroot` to run | pinned | \[Repo\] security-notes 6 |
| Kubernetes | K3s, `apps/v1`, `autoscaling/v2`, `policy/v1`, Traefik CRDs `traefik.io/v1alpha1` | checked against Kubernetes 1.30 schemas | \[Repo\] ADR-002 |
| Manifest check | kubeconform | 0.6.7 | \[Proposed\] for CI |
| Screens (later) | React 18, Vite, TypeScript, Tailwind CSS, TanStack Query, React Router 6, react-hook-form + zod; Vitest, Playwright | Node **20 LTS** | \[Repo\] web/README, local-development.md |
| Test stand-ins | Python 3 standard library | 3.11+ | only for local runs |

## 3. Folder structure and entry points

### 3.1 Folders

```
doc8-local/                          # = repo root layout
├── README.md                        # module README (deliverable)
├── Makefile                         # make up / demo / test-db / gateway-check / down
├── .env.example                     # configuration example (deliverable)
├── proposals/compose-needs-for-member-09.md   # what Member 9's shared Compose file needs (no own Compose file, K10)
├── core-api/
│   ├── cmd/
│   │   ├── api/main.go              # ENTRY POINT: the HTTP server
│   │   ├── seed/main.go             # demo rows; refuses APP_ENV=production
│   │   └── devtoken/main.go         # local key pair + test tokens; refuses production
│   ├── internal/admin/              # MY MODULE
│   │   ├── domain/                  # Zone aggregate, ZoneID, ZoneCode, PostalPrefix, overlap rule (no imports, no money)
│   │   ├── application/             # use cases, ports (interfaces), settings allowlist, reason rule, delivery report
│   │   ├── infrastructure/          # PostgreSQL adapter (pgx): advisory lock, version check, audit insert
│   │   └── transport/               # /api/v1/admin/* handlers behind RequireRole; public GET /api/v1/zones
│   ├── internal/merchants/          # Member 4's AdminService interface + a local fake (Member 4 owns the real one)
│   ├── internal/platform/           # shared helpers the admin module needs
│   │   ├── config/                  # reads env vars, fails fast
│   │   ├── auth/                    # RS256 token check (STAND-IN for Member 1's)
│   │   ├── obs/                     # JSON logs, metrics, request middleware
│   │   ├── web/                     # RequireRole, safe error envelope, audit redaction
│   │   └── apperr/                  # error kinds shared by use cases and handlers
│   ├── migrations/                  # 000002_create_delivery_zones, 000011_create_audit_logs (up + down)
│   ├── Dockerfile                   # core-api image (+ seed binary)
│   └── Dockerfile.migrate           # migrations image, same git SHA
├── deploy/
│   ├── gateway/traefik/             # traefik.yml + dynamic-routes.yml (Docker/local)
│   │   └── kubernetes/              # middlewares.yaml + ingressroutes.yaml (my files, Doc 1)
│   └── kubernetes/core-api/         # Deployment, Service, ConfigMap, Secret example, HPA, PDB, migrate Job
├── scripts/                         # local-up, local-down, demo, shutdown-demo, db-bootstrap.sql, dev-stub-users.sql, gateway_parity.py
├── tools/                           # stand-in services, gateway security test
└── evidence/                        # outputs of the runs in section 1
```

### 3.2 Entry points

| Entry point | Starts | Used by |
| --- | --- | --- |
| `core-api/cmd/api` | HTTP server on `HTTP_ADDR` (`:8080`): 11 admin routes, 1 report route, 1 public route | Docker, Kubernetes, local |
| `core-api/cmd/api -healthcheck` | Calls its own `/health/live`, exits 0 or 1 | Docker `HEALTHCHECK` (distroless has no curl) |
| `core-api/cmd/seed` | Inserts 7 zones, 3 settings, 3 demo users; safe to repeat | local, staging |
| `core-api/cmd/devtoken` | Makes a dev RSA key pair or signs a test token | local only |
| `migrate ... up` | Applies migrations in number order (`000002`, then `000011`), as `swiftdrop_migrator` | local, Member 9's Compose, Kubernetes Job |
| `traefik --configFile=traefik.yml` | Gateway | local, Member 9's Compose; Helm in Kubernetes (Member 9) |
| `web/src/app/routes` → `/ops/*`, `/courier/*` (later) | My screens inside the shared SPA | browser |

### 3.3 How the code fits together

![Figure: code map](doc8-images/d01-code-map.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart TB
    subgraph CMD["cmd/api main.go"]
        M["load config, logger, DB pool, key, routes, graceful shutdown"]
    end
    subgraph MW["Request path, outside to inside"]
        A1["auth.Middleware: verify RS256 token"]
        A2["obs.Middleware: X-Request-ID, access log, metrics, panic guard"]
        A3["withTimeout: 25 s deadline"]
        A4["ServeMux route"]
        A5["web.RequireRole ADMIN, reports also DISPATCHER"]
    end
    subgraph ADM["internal/admin"]
        T["transport: decode, call, map error"]
        AP["application: use cases + ports"]
        D["domain: Zone rules, no imports"]
        I["infrastructure: PostgreSQL adapter"]
    end
    MS["merchants.AdminService (Member 4)"]
    T --> MS
    PG[("PostgreSQL")]
    M --> A1 --> A2 --> A3 --> A4 --> A5 --> T --> AP --> D
    I -. "implements ports of" .-> AP
    AP --> I --> PG
```

</details>

The arrows inside `internal/admin` follow AGENTS.md 4.2: `domain` imports nothing; `application` imports only `domain` (plus the shared error kinds in `platform/apperr`, **\[TBD U-56\]**); `infrastructure` and `transport` sit outside. I checked this with `go list` (section 12.1).

## 4. Environment variables

### 4.1 core-api variables used by the admin module

"New" means the variable is not in `docs/operations/environment-variables.md` yet. I ask to add it there (U-57).

| Variable | What it does | Default | Required | Secret | Safe local example |
| --- | --- | --- | --- | --- | --- |
| `APP_ENV` | `local`, `development`, `staging` or `production`. Seed and devtoken refuse `production` | — | yes | no | `local` |
| `HTTP_ADDR` | Listen address | `:8080` | no | no | `:8080` |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error`. Probes log at `debug` only | `info` | no | no | `info` |
| `DATABASE_URL` | PostgreSQL URL for the **`swiftdrop_app` role** (no DDL, no UPDATE or DELETE on `audit_logs`, no DELETE on zones) | — | yes | **yes** | `postgres://swiftdrop_app:app_dev_only@localhost:5432/swiftdrop_db?sslmode=disable` |
| `MIGRATE_DATABASE_URL` (new) | URL for the **`swiftdrop_migrator` role**, used only by migrations and seed | — | for migrations | **yes** | `postgres://swiftdrop_migrator:migrator_dev_only@localhost:5432/swiftdrop_db?sslmode=disable` |
| `DB_MAX_OPEN_CONNS` | Pool size per pod | `25` | no | no | `25` locally; `20` in Kubernetes (section 7.4) |
| `DB_MAX_IDLE_CONNS` | Connections kept warm | `10` | no | no | `10` |
| `DB_STATEMENT_TIMEOUT_MS` (new) | PostgreSQL `statement_timeout` for every query | `3000` | no | no | `3000` |
| `ADMIN_LOCK_TIMEOUT_MS` (new) | `lock_timeout` inside admin transactions (Doc 7 6.3) | `2000` | no | no | `2000` |
| `JWT_PUBLIC_KEY_PEM` | Public key to check tokens: PEM text or a file path | — | yes | no | `.dev/jwt_public.pem` |
| `JWT_ISSUER` (new) | Expected `iss` claim (ADR-003 3.3) | `http://localhost` | no | no | `http://localhost` |
| `HTTP_REQUEST_TIMEOUT_SECONDS` (new) | Deadline per request; must be below the gateway's 30 s | `25` | no | no | `25` |
| `SHUTDOWN_GRACE_SECONDS` (new) | Time to finish running requests after SIGTERM | `20` | no | no | `20` (deploy/README 5.1) |
| `READINESS_DRAIN_SECONDS` (new) | Time between "not ready" and stopping, so traffic moves away first | `5` | no | no | `5` |
| `GOMEMLIMIT` (Go runtime) | Go frees memory before the container limit | none | no | no | `200MiB` in Kubernetes |

**Fail-fast rule:** at start-up the process checks **all** variables and prints every problem at once, then exits with code 1. Real output:

```text
core-api: bad configuration:
DATABASE_URL is required
JWT_PUBLIC_KEY_PEM is required
APP_ENV must be local, development, staging or production
LOG_LEVEL must be debug, info, warn or error
exit status 1
```

It also refuses a request timeout of 30 s or more. `sslmode=disable` is allowed in every environment, because the database is reached only inside the cluster network (Member 9 A5). The start-up log shows the settings **without** secrets (`"database_url":"[set, hidden]"`).

### 4.2 Configuration example (deliverable): `.env.example`

```bash
# SwiftDrop core-api (admin module part) - LOCAL example values only.
# Copy to .env. Never commit .env. Real values live in Kubernetes Secrets.
APP_ENV=local
HTTP_ADDR=:8080
LOG_LEVEL=info

# Runtime role swiftdrop_app: no DDL, cannot UPDATE or DELETE audit_logs (Doc 5, Member 9 D6/D7).
# Migrations use swiftdrop_migrator. In Kubernetes these come from the Secrets
# postgres-secrets and postgres-migrator-secrets; sslmode=disable inside the cluster.
DATABASE_URL=postgres://swiftdrop_app:app_dev_only@localhost:5432/swiftdrop_db?sslmode=disable
MIGRATE_DATABASE_URL=postgres://swiftdrop_migrator:migrator_dev_only@localhost:5432/swiftdrop_db?sslmode=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=10
DB_STATEMENT_TIMEOUT_MS=3000
ADMIN_LOCK_TIMEOUT_MS=2000

# Public key only. Path to a PEM file, or the PEM text itself. Local key: go run ./cmd/devtoken -genkey .dev
JWT_PUBLIC_KEY_PEM=.dev/jwt_public.pem
JWT_ISSUER=http://localhost

HTTP_REQUEST_TIMEOUT_SECONDS=25
SHUTDOWN_GRACE_SECONDS=20
READINESS_DRAIN_SECONDS=5
```

### 4.3 Gateway and screens

| Part | Configured by | Notes |
| --- | --- | --- |
| Traefik | Files, not env vars: `traefik.yml` (entry points, trusted IPs, timeouts) and `dynamic-routes.yml` (routes, limits); in Kubernetes the CRD files | Change both file sets together; `make gateway-check` fails if they differ |
| Screens: API base URL | `VITE_API_BASE_URL` (web/README) **or** `VITE_API_GATEWAY_URL` (environment-variables.md) | Same thing, two names **\[TBD U-58\]**. Default `/api/v1` (same origin, no CORS needed) |
| Screens: other | `VITE_APP_TITLE`, `VITE_GOOGLE_CLIENT_ID` | Everything `VITE_*` ends up in the browser bundle. **Never put a secret there** |

## 5. Local run, build and test

### 5.1 Prerequisites

| Tool | Check | Install note |
| --- | --- | --- |
| Go 1.22+ | `go version` | |
| PostgreSQL 16 server | `ls /usr/lib/postgresql/16/bin` or `postgres --version` | Ubuntu: `apt install postgresql-16`. Or use Member 9's shared Compose stack (section 6.3) |
| golang-migrate | `migrate -version` | `go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.1` (**the tag is needed**, or you get "unknown driver postgres") |
| Traefik 3.1 | `traefik version` | Download the `v3.1.7` release binary, or set `TRAEFIK=/path/to/traefik` |
| Python 3, curl, make | | For stand-ins and scripts |

### 5.2 Commands

```bash
cd doc8-local
make up              # start everything (section 5.3)
make demo            # 51 checks; needs a fresh database (make wipe && make up)
make shutdown-demo   # graceful shutdown, then restart
make test            # unit tests only, no database
make test-db         # all tests, including real PostgreSQL tests
make lint            # gofmt + go vet
make gateway-check   # gateway parity, Kubernetes schemas, 25 security checks
make build           # static binaries in core-api/bin/
make docker-build    # images (needs a Docker daemon)
make down            # stop and wait until core-api has exited (port 8080 free);  make wipe = also delete the local database
```

### 5.3 What `make up` does

![Figure: local stack](doc8-images/d02-local-stack.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart TB
    S1["1 PostgreSQL 16: initdb if new, start on 5432"] --> S2["2 db-bootstrap.sql: swiftdrop_migrator and swiftdrop_app, grants; users stub"]
    S2 --> S3["3 migrate up as migrator: 000002, 000011"]
    S3 --> S4["4 seed as migrator: 7 zones, 3 settings, 3 users"]
    S4 --> S5["5 devtoken -genkey: local RSA key pair"]
    S5 --> S6["6 core-api as app role on 8080, wait for /health/ready"]
    S6 --> S7["7 Traefik on 8088 + stand-ins: tracking-api 8081, web 3000"]
    S7 --> R["Ready: browser and curl use http://localhost:8088"]
    subgraph PORTS["Local ports"]
        P1["8088 gateway, public"]
        P2["8080 core-api: /health/live, /health/ready, /metrics"]
        P3["8090 Traefik dashboard + ping, internal"]
        P4["5432 PostgreSQL"]
    end
```

</details>

Member 1's `users` table does not exist yet, so `scripts/dev-stub-users.sql` makes a small stand-in for the audit and merchant references. It is **not** a migration and is never shipped. The AI service is **not** started on purpose: its route answers 502, which shows that one missing service does not break the others.

### 5.4 Example session (real output)

```bash
$ TOKEN=$(cd core-api && go run ./cmd/devtoken -role ADMIN)
$ curl -s -H "Authorization: Bearer $TOKEN" -X POST localhost:8088/api/v1/admin/zones \
    -d '{"id":"ZONE_MATARA_01","code":"MTR-01","name":"Matara","postal_prefixes":["81000"]}'
{"data":{"id":"ZONE_MATARA_01","code":"MTR-01","name":"Matara","description":"",
 "postal_prefixes":["81000"],"enabled":true,"version":1}}

$ curl -s -i localhost:8088/api/v1/admin/zones          # no token
HTTP/1.1 401 Unauthorized
Access-Control-Expose-Headers: X-Request-ID,Content-Length
Content-Type: application/json
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-Request-Id: corr_5f3f3e3fac779142dd70a16f
{"error":{"code":"AUTH_REQUIRED","message":"Sign in to continue","correlation_id":"corr_5f3f3e3fac779142dd70a16f",...}}

$ curl -s -i "localhost:8088/api/v1/zones?postal_code=81000"   # public, no token (for Member 6)
HTTP/1.1 200 OK
Cache-Control: public, max-age=300
{"data":{"id":"ZONE_MATARA_01","code":"MTR-01","name":"Matara"}}

$ curl -s -H "Authorization: Bearer $TOKEN" "localhost:8088/api/v1/admin/audit-logs?limit=1"
{"data":[{"id":"4ddd971a-...","actor_id":"00000000-0000-4000-8000-000000000001","actor_role":"ADMIN",
 "action":"ZONE_CREATED","target_type":"ZONE","target_id":"ZONE_MATARA_01","previous_state":null,
 "new_state":{...},"reason":null,"request_id":"corr_bfe36c04ced02acd20c3e2c0","created_at":"2026-10-26T12:25:22.691998Z"}]}

$ DTOKEN=$(cd core-api && go run ./cmd/devtoken -role DISPATCHER)
$ curl -s -H "Authorization: Bearer $DTOKEN" "localhost:8088/api/v1/admin/reports/deliveries?from=2026-10-01&to=2026-10-03"
{"data":{"from":"2026-10-01","to":"2026-10-03","days":[],"delivered":0,"failed":0,"success_rate":null,
 "source":"stand-in: no parcel data until Members 2 and 3 connect"}}

$ curl -s -H "Authorization: Bearer $TOKEN" "localhost:8088/api/v1/admin/merchants?status=PENDING"
{"data":[{"merchant_id":"11111111-...","business_name":"Ceylon Tea Shop","status":"PENDING",...,"version":1,...}],
 "meta":{"page":1,"per_page":20,"total":1}}
```

The 51 demo checks (`evidence/02-demo.txt`) cover: health (`/health/live`, `/health/ready`, not public); no token, courier token and tampered token; the public zone list and postal code lookup (only enabled zones, only id, code and name, cached 5 minutes); create, duplicate id, duplicate short code, prefix overlap, lower-case id, a money field refused, unknown field, reason rule, stale version, disable; settings allowlist; merchants (dispatcher refused, list, filter, approve with `expected_version`, stale version 409 with Member 4's code, suspend needs a reason, reinstate, wrong transition, unknown action); the delivery report (dispatcher allowed, courier refused, range over 92 days refused, stand-in data labelled); 4 audit rows with reason and request id; the app role **cannot** update `audit_logs` or delete zones; even the migrator **cannot** delete audit rows (trigger); `/api/v2` gives JSON 404; AI down gives 502 while the rest works; `X-Request-ID` round trip; metrics; JSON logs with no tokens or passwords.

**Merchants and reports are stand-ins for now.** The merchant routes call a small in-memory `merchants.NewDevFake()` with Member 4's rules; the real `merchants.AdminService` replaces it with no change to my handlers. The report says `source: stand-in` until Members 2 and 3 give their read interfaces.

### 5.5 The screens (when they exist)

```bash
cd web && npm ci && npm run dev        # Vite dev server (local-development.md: port 3000)
npm run test:unit                      # Vitest (Doc 6)
npx playwright test                    # E2E against make up (Doc 6)
```

During development, Vite should forward `/api` to the gateway (`server.proxy` in `vite.config.ts`: `/api` → `http://localhost:8088`). Then the browser sees **one origin**, cookies work with `SameSite=Strict`, and CORS is not needed **\[Proposed\]**. Vite must run on port 3000 (local-development.md). The CORS list allows only `localhost:3000` and `127.0.0.1:3000` (security-notes 4, R-14); `5173` is no longer allowed.

## 6. Docker build and health checks

### 6.1 The core-api image

```dockerfile
FROM golang:1.22.12-alpine3.21 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/core-api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/core-api /core-api
COPY --from=build /out/seed /seed
COPY migrations /migrations
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 CMD ["/core-api", "-healthcheck"]
ENTRYPOINT ["/core-api"]
```

| Rule | How |
| --- | --- |
| Pinned versions, no `latest` (security-notes 6) | All three image tags exist (checked on Docker Hub on 24 Oct 2026) |
| Small, no shell | distroless static: no shell, no package manager. The binary is about 10 MB |
| Non-root (SEC-016) | `USER 10001:10001` |
| Read-only root file system | The app writes nothing to disk (logs go to stdout) |
| No secrets in the image | `.dockerignore` drops `.dev`, `*.pem`, `.env` |
| Health check without curl | `/core-api -healthcheck` calls `/health/live`; tested: exit 0 when up, 1 when not |
| Tag | git SHA from CI, for example `ghcr.io/<org>/swiftdrop/core-api:40dc279` |

### 6.2 Migrations image

`core-api/Dockerfile.migrate` = `migrate/migrate:v4.17.1` + this release's SQL files, tagged with the **same git SHA**. So the migration Job always runs exactly the schema that the new code expects.

### 6.3 Docker Compose (Member 9's shared file)

Member 9 owns the one local stack file, `deploy/docker-compose.yml`, started with `make up` (agreement A7). Members do not keep their own Compose files (K10), so I removed `docker-compose.m7.yml`. What my module needs from the shared file is in `proposals/compose-needs-for-member-09.md`:

| Need | Value |
| --- | --- |
| Database roles before migrations | `scripts/db-bootstrap.sql` as an init script (`swiftdrop_migrator`, `swiftdrop_app`) |
| Migrations one-shot | `migrate/migrate:v4.17.1`, as `swiftdrop_migrator`, files `000002` and `000011` |
| core-api | `DATABASE_URL` as `swiftdrop_app`; public key file only; `read_only`, `no-new-privileges`; `stop_grace_period: 30s`; health check `/core-api -healthcheck` |
| Order | PostgreSQL healthy, then migrate finished, then core-api starts |
| Traefik | My two gateway files mounted read-only; dashboard `127.0.0.1:8090` only (R-10) |

### 6.4 Health endpoints

| Endpoint | Means | Checks | Answer |
| --- | --- | --- | --- |
| `GET /health/live` | "The process is alive" | Nothing outside the process | 200 `{"status":"UP"}` |
| `GET /health/ready` | "Send me traffic" | Database ping (1 s), not shutting down | 200 `{"status":"READY","database":"UP"}`; 503 `{"status":"NOT_READY","database":"DOWN"}`; 503 `{"status":"SHUTTING_DOWN"}` |
| `GET /metrics` | Prometheus scrape | — | text format |

All three are on the pod port only (R-07, Member 9 A2). The gateway never routes them (Doc 6 GW-08, GW-09; demo DEMO-03).

## 7. Kubernetes deployment

### 7.1 Files

All 23 resources pass `kubeconform -strict` against Kubernetes 1.30 and the Traefik CRD schemas.

| File | Kind | Owner |
| --- | --- | --- |
| `deploy/kubernetes/core-api/deployment.yaml` | Deployment | Member 9 (proposal from me) |
| `.../service.yaml` | Service (ClusterIP 8080) | Member 9 |
| `.../configmap.yaml` | ConfigMap `core-api-config` | Member 9 |
| `.../secret.example.yaml` | Secret examples only: `postgres-secrets` (`DATABASE_URL` for `swiftdrop_app`) and `postgres-migrator-secrets` (`MIGRATE_DATABASE_URL` for `swiftdrop_migrator`), Member 9 A5 | Member 9 |
| `.../hpa.yaml` | HorizontalPodAutoscaler | Member 9 |
| `.../pdb.yaml` | PodDisruptionBudget | Member 9 |
| `.../migrate-job.yaml` | Job; reads `postgres-migrator-secrets` | Member 9 |
| `deploy/gateway/traefik/kubernetes/middlewares.yaml` | 11 Middleware + 3 ServersTransport (login, tracking, merchant API and AI limits, stream cap, body caps, header strip, CORS, security headers, CSP) | **Me** |
| `deploy/gateway/traefik/kubernetes/ingressroutes.yaml` | IngressRoute (10 routes with explicit priorities) | **Me** |

![Figure: k8s topology](doc8-images/d03-k8s-topology.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart TB
    CF["Cloudflare"] --> TR
    subgraph NS1["namespace traefik"]
        TR["Traefik (Helm, Member 9)"]
    end
    subgraph NS2["namespace swiftdrop"]
        IR["IngressRoute swiftdrop + Middlewares (mine)"]
        SVC["Service core-api ClusterIP 8080"]
        subgraph DEP["Deployment core-api, 2 to 4 pods (HPA)"]
            P1["pod 1"]
            P2["pod 2"]
        end
        CM["ConfigMap core-api-config"]
        SEC["Secrets postgres-secrets, postgres-migrator-secrets, jwt-public-key"]
        JOB["Job core-api-migrate"]
        PG[("StatefulSet postgres-0")]
        PDB["PDB minAvailable 1"]
    end
    TR --> IR --> SVC --> P1
    SVC --> P2
    CM -.-> P1
    SEC -.-> P1
    P1 --> PG
    P2 --> PG
    JOB --> PG
    PDB -.-> DEP
```

</details>

### 7.2 Probes

| Probe | Path | Timing | Why |
| --- | --- | --- | --- |
| startupProbe | `/health/live` | every 2 s, up to 15 tries (30 s) | Slow first start does not trigger liveness restarts |
| readinessProbe | `/health/ready` | every 5 s, 2 failures | Database down or shutting down → pod leaves the Service, no restart |
| livenessProbe | `/health/live` | every 10 s, 3 failures | Only a stuck process is restarted. **It must not check the database**: a database outage would otherwise restart every pod at once and make things worse |

I tested this locally: with PostgreSQL stopped, `/health/ready` answered 503 and `/health/live` 200; admin requests answered **503 `SERVICE_UNAVAILABLE`** (not a hang, not a crash). When PostgreSQL came back, requests worked again **without a restart**.

### 7.3 Resources and security context

| Setting | Value | Reason |
| --- | --- | --- |
| CPU request | 100m | HPA percentage is counted against this |
| CPU limit | none | CPU limits throttle Go and add latency; memory is the real risk on one node |
| Memory request / limit | 128Mi / 256Mi | Admin traffic is small; `GOMEMLIMIT=200MiB` makes Go clean up first |
| `runAsNonRoot`, user 10001, `readOnlyRootFilesystem`, drop ALL capabilities, seccomp RuntimeDefault | on | SEC-016, SEC-017 |
| `automountServiceAccountToken` | false | core-api never calls the Kubernetes API |

These numbers are **starting guesses** for a single VPS. Member 9 should adjust them after watching real use **\[TBD U-59\]**.

### 7.4 Scaling

| Topic | Rule |
| --- | --- |
| Replicas | 2 to 4, HPA at 70 % CPU (deploy/README 5.1). Scale-down waits 5 minutes |
| Stateless | No sessions or files in the pod. Any pod can answer any request |
| Database connections | `4 pods x DB_MAX_OPEN_CONNS 20 = 80`, below PostgreSQL's 100 (deploy/README 5.1). So Kubernetes uses 20, not the default 25 |
| Safe with many pods | The prefix overlap lock is a **PostgreSQL** advisory lock, so it works across pods. The version check is in SQL. Nothing depends on one pod's memory |
| Not shared | Traefik rate-limit buckets live in Traefik's memory. One Traefik pod today; more pods would multiply the limits **\[TBD U-46\]** |
| Disruptions | PDB keeps at least 1 pod during node drains; rolling update with `maxUnavailable: 0` |

### 7.5 Release order

![Figure: release flow](doc8-images/d04-release-flow.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
sequenceDiagram
    autonumber
    participant CI as CI (Member 9)
    participant REG as Image registry
    participant K as Kubernetes
    participant J as Job core-api-migrate
    participant DB as PostgreSQL
    participant D as Deployment core-api
    CI->>CI: lint, tests, gateway-check
    CI->>REG: push core-api and core-api-migrations, same git SHA
    CI->>K: apply migrate Job with that SHA
    K->>J: run once
    J->>DB: migrate up as swiftdrop_migrator
    alt migration fails
        J-->>CI: Job failed, stop. Old pods keep running
    end
    CI->>K: set image on Deployment
    K->>D: start 1 new pod (maxSurge 1)
    D->>D: startupProbe, then readinessProbe
    K->>D: old pod gets SIGTERM, drains 5 s, finishes requests
    K->>D: repeat until all pods are new
```

</details>

Migrations must be **backward compatible** with the old code that is still running during the rollout (expand first, contract in a later release, Doc 5 6.5).

### 7.6 Gateway in Kubernetes

The IngressRoute uses explicit `priority` values. **Lesson from Doc 7:** Traefik ranks routes by the length of the rule text unless you set priority, and that once hid my SSE limit. The TLS secret is `cloudflare-origin-cert` (deployment-guide.md), but deploy/gateway/README says `traefik-origin-tls` **\[TBD U-60\]**. The dashboard is never exposed; use `kubectl port-forward -n traefik svc/traefik 8090:8090` (port 8090 agreed, R-10).

## 8. Logging, metrics, tracing and alerts

### 8.1 Logs

One JSON object per line on stdout. Kubernetes collects them. Real line:

```json
{"timestamp":"2026-10-26T12:15:21.357985752Z","level":"INFO","msg":"request","service":"core-api","module":"admin",
 "request_id":"corr_5901ed851d729ce2cd083bf8","method":"GET","route":"GET /api/v1/admin/zones","status":200,
 "duration_ms":0.546,"actor_id":"00000000-0000-4000-8000-000000000001","actor_role":"ADMIN"}
```

| Field | Meaning |
| --- | --- |
| `request_id` | Same as the `X-Request-ID` header (R-11), the `correlation_id` field in error bodies (api-conventions) and `request_id` in audit rows. **Search by this first** |
| `route` | The route pattern, not the raw path (no IDs, so no personal data and few distinct values) |
| `actor_id`, `actor_role` | From the verified token |
| `security_event: access_denied` | Added on 401 and 403 (Doc 7 10.1) |
| `level` | `ERROR` for 5xx (with a separate `admin request failed` line holding the cause), `INFO` for normal requests, `DEBUG` for probes and metrics scrapes |

**Never logged:** tokens, passwords, database URLs, request bodies, recipient data. Demo check DEMO-51 searches the log for tokens and passwords and finds none.

### 8.2 Metrics (`GET /metrics` on the pod)

| Metric | Type | Labels | Use |
| --- | --- | --- | --- |
| `http_requests_total` | counter | `route`, `status` | Traffic and error rate |
| `http_request_duration_seconds` | histogram | `route` | Latency (p95) |
| `admin_changes_total` | counter | `action` | Business activity; each one has an audit row |
| `admin_conflicts_total` | counter | `kind` = `version`, `prefix_overlap`, `lock_timeout` | Many conflicts = users fighting over the same zone, or a stuck lock |
| `db_pool_in_use_connections` | gauge | — | Pool pressure |
| `db_pool_max_connections` | gauge | — | Pool size |
| `db_pool_wait_total` | gauge (count since start) | — | Requests that had to wait for a connection |

Traefik has its own metrics (Member 9 enables the Prometheus endpoint on the internal entry point).

### 8.3 Tracing

Today: **request ID only** (`X-Request-ID`, R-11; not `X-Correlation-ID`). The gateway does not create one; core-api reuses a well-formed `X-Request-ID` from the client or makes a new one, returns it in the response header, puts it in every log line, error body and audit row. Proven by DEMO-47 and DEMO-50. OpenTelemetry (`OTEL_EXPORTER_OTLP_ENDPOINT` in environment-variables.md) is **not** implemented in this slice **\[TBD U-61\]**.

### 8.4 Alerts (proposed for Member 9's Prometheus)

| Alert | Rule (PromQL idea) | For | Severity | First step |
| --- | --- | --- | --- | --- |
| Admin API errors | `sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) > 0.05` | 5 m | page | Logs at `level=ERROR`, then runbook Playbook A |
| Slow admin API | p95 of `http_request_duration_seconds` > 1 s | 10 m | warn | Check report queries and DB load |
| Pods not ready | `kube_deployment_status_replicas_available{deployment="core-api"} < 2` | 5 m | page | `kubectl describe pod`; `/health/ready` output |
| DB pool exhausted | `db_pool_in_use_connections / db_pool_max_connections > 0.9` | 5 m | warn | Runbook Playbook A |
| Lock timeouts | `increase(admin_conflicts_total{kind="lock_timeout"}[15m]) > 3` | — | warn | Find the long transaction (section 9) |
| Many denied requests | `increase(http_requests_total{status=~"401|403"}[10m]) > 100` | — | warn | Security log lines by IP and actor (Doc 7 O-04) |
| Gateway 429 spike | Traefik 429 rate high | 10 m | info | Real users behind one IP? (Doc 7 FM-04) |
| Restarts | `increase(kube_pod_container_status_restarts_total{container="core-api"}[30m]) > 2` | — | warn | `kubectl logs --previous` |

## 9. Troubleshooting

![Figure: troubleshoot](doc8-images/d05-troubleshoot.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart TD
    S["A screen shows an error"] --> Q{"Which status?"}
    Q -->|"401"| A1["Token missing, expired or wrong issuer. Check JWT_ISSUER and the public key"]
    Q -->|"403"| A2["Role not allowed. Normal for couriers; dispatchers only reach reports"]
    Q -->|"409"| A3["Version, prefix or merchant status conflict. Reload. If constant, check admin_conflicts_total"]
    Q -->|"502"| A4["Backend pod down or unreachable. kubectl get pods"]
    Q -->|"503"| A5{"Body code?"}
    A5 -->|"SERVICE_UNAVAILABLE"| A6["Database down or slow. /health/ready, pg_isready, runbook A"]
    A5 -->|"TRY_AGAIN"| A7["Lock wait over 2 s. Find long transaction"]
    Q -->|"504"| A8["core-api took over 30 s. Search logs by request_id"]
    Q -->|"500"| A9["Search logs for the correlation_id, read the admin request failed line"]
```

</details>

| Symptom | Likely cause | Check | Fix |
| --- | --- | --- | --- |
| Process exits at once with "bad configuration" | Missing or wrong env var | The message lists them all | Set the variables (section 4) |
| Every request 401, also for admins | `JWT_ISSUER` does not match the token's `iss`, or the wrong public key | Decode the token payload (middle part, base64) | Same issuer and key pair as Member 1's sign-in |
| `/health/ready` 503 `database DOWN` | PostgreSQL down, wrong URL or password, network policy | `pg_isready -h <host>`; core-api error log line | Fix DB; the pod recovers by itself (tested) |
| Admin requests 503 `SERVICE_UNAVAILABLE` | Same as above, or a query hit `statement_timeout` | Log line `admin request failed` has the cause | As above; for timeouts check the query |
| 503 `TRY_AGAIN` on zone saves | Another transaction holds the prefix lock over 2 s | `SELECT pid, state, xact_start, query FROM pg_stat_activity WHERE state <> 'idle' ORDER BY xact_start;` | End the stuck session (runbook Playbook A) |
| 409 `CONCURRENT_MODIFICATION` every time | Screen sends an old `version` | Compare with `GET /admin/zones/{id}` | Reload before editing (Doc 7 section 9.2) |
| `permission denied for table audit_logs` | App tried to change audit rows (a bug), or migrations ran as the app role | Which role is in the URL | Migrations must use `MIGRATE_DATABASE_URL` (`swiftdrop_migrator`) |
| `audit_logs is append-only` | Someone ran UPDATE or DELETE | Who ran it | Expected. Fix forward with a new change |
| `migrate: unknown driver postgres` | migrate built without the tag | `migrate -version` | Reinstall with `-tags postgres` |
| `migrate: Dirty database version N` | A migration failed half way | `SELECT * FROM schema_migrations;` | Fix the SQL, then `migrate force <N-1>` and `up` (local-development.md 5) |
| `/api/v1/...` returns HTML | Request reached the SPA: gateway route missing | Traefik dashboard (internal) or `make gateway-check` | Fix the route; the `/api` catch-all should give JSON 404 |
| 413 on save | Body over 1 MB on a JSON route | Request size | Correct; photos use `/api/v1/deliveries/{id}/proof-photo` (10 MB) |
| Many 429 for one office | Everyone shares one IP | Traefik access log | Raise the limit or add an office rule (Doc 7 FM-04) |
| Pod restarted, `OOMKilled` | Memory over 256Mi | `kubectl describe pod` | Raise the limit; check for large report queries |
| Logs show no `actor_id` | Request had no valid token | `security_event` field | Expected for 401 and for the public `GET /api/v1/zones` |
| `make up` fails with "address already in use" on 8080 | An old core-api was still draining | `make down` now waits for it; run `make down` again | Then `make up` |
| Merchant action 409 | `expected_version` is old, or the move is not allowed (for example reject an APPROVED merchant) | Error `code` comes from Member 4 unchanged | Reload the merchant |
| Report 422 | Range over `reports.max_range_days` (92) | `from` and `to` | Smaller range |

**Useful commands**

```bash
kubectl -n swiftdrop get pods -l app.kubernetes.io/name=core-api
kubectl -n swiftdrop logs deploy/core-api --since=15m | grep '"request_id":"corr_...'
kubectl -n swiftdrop logs <pod> --previous                       # after a crash
kubectl -n swiftdrop port-forward deploy/core-api 8080:8080       # then curl localhost:8080/health/ready /metrics
kubectl -n swiftdrop rollout undo deployment/core-api             # runbook 7
psql "$MIGRATE_DATABASE_URL" -c "SELECT action, target_id, actor_id, reason, request_id, created_at FROM audit_logs ORDER BY created_at DESC LIMIT 20;"
```

## 10. External dependencies

| Dependency | Needed for | If it is down |
| --- | --- | --- |
| PostgreSQL 16 | Everything the admin module does | `/health/ready` 503, requests 503, automatic recovery |
| Member 1's sign-in and key pair | Real tokens; my code only needs the **public** key | Users cannot sign in; already signed-in users work until their 15-minute token ends |
| Traefik (Members 7 and 9) | Only way in from outside | Site unreachable; pods healthy |
| Cloudflare | TLS, DDoS, real client IP | As above |
| Prometheus and Grafana (Member 9) | Metrics and alerts | No effect on users; blind operations |
| Other modules (merchants, dispatch, pricing) | Merchant routes call Member 4's `merchants.AdminService` (a local fake today); the delivery report reads Members 2 and 3 through Go interfaces (a stand-in today). Same process (Doc 3 DD-09) | Same process, so no network failure; their database tables matter |
| RabbitMQ, object storage | **Not used** by the admin module today (U-32 for zone events) | No effect |

## 11. Start-up and shutdown

![Figure: lifecycle](doc8-images/d06-lifecycle.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
sequenceDiagram
    autonumber
    participant K as Kubernetes
    participant P as core-api process
    participant DB as PostgreSQL
    participant T as Traefik
    K->>P: start container
    P->>P: read all env vars, exit 1 on any problem
    P->>P: load public key, build pool (lazy, no crash if DB is late)
    P->>P: listen on 8080
    K->>P: startupProbe /health/live = 200
    K->>P: readinessProbe /health/ready
    P->>DB: ping
    DB-->>P: ok
    P-->>K: 200 READY
    K->>T: add pod to Service endpoints
    Note over K,T: normal traffic
    K->>P: SIGTERM (rollout or scale down)
    P->>P: /health/ready now 503 SHUTTING_DOWN
    K->>T: remove pod from endpoints
    P->>P: wait READINESS_DRAIN_SECONDS 5 s, still serving
    P->>P: stop accepting, finish running requests, up to 20 s
    P->>DB: close pool
    P-->>K: exit 0, within terminationGracePeriod 30 s
```

</details>

**Start-up rules**

1. Bad configuration stops the process **before** it opens anything (fail fast).
2. A late database does **not** crash the process. It starts, `/health/ready` says 503, and it becomes ready when the database answers. This avoids crash loops when the whole stack starts together.
3. Migrations are **not** run by core-api at start-up. They run once, in the Job, before the rollout. Two pods starting together would otherwise race on the schema.

**Shutdown, proven locally** (`evidence/03-shutdown.txt`): 1 second after SIGTERM, `/health/ready` answered `SHUTTING_DOWN` while `/health/live` stayed `UP`. All 20 requests sent during the drain got 200. The process exited about 7 seconds after the signal, then restarted ready.

## 12. Maintaining the module

### 12.1 Everyday tasks

| Task | Steps |
| --- | --- |
| Add an admin endpoint | Add a use case in `application`; handler in `transport`; add it to the `routes` map in `Register` (it is wrapped in `RequireRole(ADMIN)` automatically; any other role list must be written out, like the report route); update `API.md` and `README.md` (AGENTS.md); add tests for success, 401, 403 and the error cases |
| Add a migration | `migrate create -ext sql -dir core-api/migrations -seq -digits 6 <name>`, using the next free number in the team's numbering (Doc 5 6.2, R-06); write **up and down**; expand first, contract later; run `make wipe && make up && make test-db` |
| Add a setting | Add the key and its rule in `application/settings_rules.go`; add a seed row; add a test case. Never for secrets |
| Change a rate limit or route | Edit `dynamic-routes.yml` **and** `kubernetes/middlewares.yaml` or `ingressroutes.yaml`; `make gateway-check` must pass |
| Check layer rules | `go list -f '{{.ImportPath}}: {{.Imports}}' ./internal/admin/...`; `domain` must import only the standard library |
| Rotate the JWT key | security-notes 5: publish the new public key 24 h before switching. **My stand-in checks one key only**; Member 1's real middleware must accept two **\[TBD U-62\]** |
| Upgrade Traefik | Change the version in the image tag and the CI download, run `make gateway-check`, read the Traefik migration notes |
| Roll back | `kubectl rollout undo deployment/core-api`. A migration is rolled back with its `.down.sql` only if no data would be lost |

### 12.2 Tests that protect the important rules

| Test | Proves |
| --- | --- |
| `TestOverlappingPrefixRace` | 20 admins add overlapping prefixes at once: exactly 1 wins, 19 get 409. **With the advisory lock removed, the same test saved 2 or 3 overlapping zones**, so the lock is necessary |
| `TestStaleVersionRace` | 10 saves with the same version: exactly 1 wins, 9 get 409 |
| `TestAuditInSameTransaction` | The zone change and its audit row commit together |
| `TestPublicZonesAndPostalLookup` | Public list shows only enabled zones; the longest matching prefix wins |
| merchants fake tests | Member 4's allowed moves, reason length and `expected_version` |
| `TestRequireRole_*`, `TestVerify` | Roles, token checks (wrong key, expired, `alg:none`, edited payload) |
| `TestLoad_*` | Configuration fails fast and never prints secrets |
| demo checks DEMO-41 to DEMO-43 | Database grants and trigger keep `audit_logs` append-only; the app role cannot delete zones |

While writing the race test I first made a mistake: some of my test prefixes did not overlap each other, so several zones were rightly saved. The test now uses `655`, `6550` and `65500`, which all overlap. The lesson: a race test must be checked by removing the protection and watching it fail, as I did.

## 13. Known limitations

1. **Not in the team repo yet.** This is a working copy; moving it in needs the team's `go.mod` and wiring (U-10).
2. **Sign-in is a stand-in.** `platform/auth` checks RS256 tokens like ADR-003 says, but Member 1's middleware replaces it. No refresh tokens, no key rotation, no `jti` blocklist.
3. **No Idempotency-Key storage.** Waits for the shared table (U-24, U-39). Zone creation is still protected by the unique code.
4. **One outside library** (pgx) against the "no new dependencies" rule (U-55).
5. **Merchant service and report data are stand-ins.** The routes are real; Member 4's service and Members 2 and 3's data replace the stand-ins. Pricing-rule routes are Member 2's (U-65).
6. **No OpenTelemetry tracing**; request ID only (U-61).
7. **Docker images not built here** (no Docker daemon); the binary is built the same way as the Dockerfile does it.
8. **Kubernetes files are schema-checked, not applied** to a cluster.
9. **Screens not built.**
10. **Seed users are a stub** of Member 1's `users` table (`scripts/dev-stub-users.sql`, local only).
11. **Traefik rate limits are per Traefik pod**, in memory.

## 14. Conflicts found in repo documents

| Topic | Says A | Says B | Question |
| --- | --- | --- | --- |
| Clone URL | local-development.md and deployment-guide.md: `github.com/Prashani-beep/SwiftDrop` | Team repo: `github.com/phantomTroupeEAD/SwiftDrop` | Fix the docs (Member 9) |
| Missing files | deployment-guide.md applies `deploy/kubernetes/base/*.yaml`, `values.yaml`, `middlewares.yaml`, `ingressroutes.yaml`, `docker-compose.yml`, `.env.example`, `Makefile`, `deploy.yml` | None of these exist in the repo (commit 40dc279) | My section 7 files can fill part of it |
| Frontend API variable | web/README: `VITE_API_BASE_URL` | environment-variables.md: `VITE_API_GATEWAY_URL` | U-58 |
| Vite port | local-development.md: 3000 | Repo gateway CORS list: 3000 and 5173 | Fixed in my gateway files: 3000 only (R-14) |
| Client tenant header | web/README 4: "`X-Tenant-ID` is passed" | Doc 7: tenant only from the token; gateway strips it (R-15) | Fix web/README (Member 6, U-63) |
| TLS secret name | deployment-guide.md: `cloudflare-origin-cert` | deploy/gateway/README: `traefik-origin-tls` | U-60 |
| Dashboard port | runbook 6: 9000 | Member 9: 8090; repo: 8080 | **Closed** (R-10): 8090; runbook 6 needs updating |
| Readiness body | runbook 1: `{"status":"READY","database":"UP"}` | — | I follow the runbook exactly, on `/health/ready` |
| Request ID header | alignment guide 3.3: `X-Correlation-ID` | security-notes 4, Member 9 A10: `X-Request-ID` | **Closed** (R-11): `X-Request-ID` |
| Role names in the SPA | web/README: `ROLE_ADMIN` etc. | ADR-003: `ADMIN` etc. | Screens must use the token's `ADMIN` |

## 15. Open questions

Earlier questions that affect running the module. Still open: U-24, U-39, U-46. Closed: U-10 (I write all of my module), U-40 (6-digit numbering, R-06), U-48 (8090, R-10).

| ID | Question | Decide with | Status |
| --- | --- | --- | --- |
| U-55 | Which PostgreSQL driver for core-api: `pgx/v5` (my choice: fast, good errors, pool built in) or `database/sql` + `lib/pq`? AGENTS.md forbids new libraries without agreement | Members 1 to 4, Member 9 | Open |
| U-56 | Shared error kinds package (`platform/apperr`) that `application` layers may import, or one set per module? (AGENTS.md: application depends only on domain) | Backend members | Open |
| U-57 | Add the new variables (section 4.1) to environment-variables.md | Member 9 | Open |
| U-58 | One frontend API variable name; use the Vite proxy so CORS is not needed locally | Member 6, Member 9 | Partly closed: Vite port is 3000 (R-14); the variable name is still open |
| U-59 | Real CPU and memory numbers for core-api after a load test | Member 9 | Open |
| U-60 | One name for the Cloudflare origin TLS secret | Member 9 | Open |
| U-61 | OpenTelemetry tracing now, or request IDs only for the project? | Member 9, Member 10 | Open |
| U-62 | Member 1's token middleware must accept two public keys during rotation (security-notes 5) | Member 1 | Open |

## 16. Sign-off

To be reviewed by Members 1, 2, 3, 4, 5, 6, 8, 9 and 10. Member 9 (deployment and observability) and Member 1 (auth) are the main reviewers.

| Member | Approved (date) | Notes |
| --- | --- | --- |
| Member 1 |  |  |
| Member 2 |  |  |
| Member 3 |  |  |
| Member 4 |  |  |
| Member 5 |  |  |
| Member 6 |  |  |
| Member 8 |  |  |
| Member 9 |  |  |
| Member 10 |  |  |
