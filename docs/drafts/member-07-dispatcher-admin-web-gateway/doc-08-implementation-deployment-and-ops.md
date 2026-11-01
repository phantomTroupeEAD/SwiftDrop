# Document 8 — Implementation, Deployment & Operations Guide

> **Status:** Draft / Proposed (For Review in Meeting 2)  
> **Author:** Member 7 — Dispatcher/Admin UI, Courier Web, Traefik Gateway  
> **Target Module / Location:** `web/src/features/dispatcher-admin/` (ops + courier screens), `core-api/internal/admin/` (admin module), `deploy/gateway/traefik/` (gateway)  
> **Key Upstream Dependencies:** Member 1 (real sign-in), Member 9 (Compose, Kubernetes, Helm, observability), Member 8 (events)  
> **Open Questions / Needs Clarification:** U-55 to U-62 in section 15 (runtime config, import rules, deployment and ops ownership)

**Module:** Dispatcher/Admin/Courier Frontend, Admin Backend Module & API Gateway (Member 7) **Project:** SwiftDrop Parcel Delivery & Courier Management System **Based on:** Documents 1 v3, 2 v2, 3 v1, 5 v1, 6 v1 and 7 v1; docs/operations/environment-variables.md, local-development.md, deployment-guide.md, runbook.md; deploy/README.md; deploy/gateway/README.md; AGENTS.md sections 4 and 9; web/README.md; .github/workflows/ci.yml **Status:** Draft v1. Items marked **\[TBD\]** wait for a teammate or a team decision. New open questions start at U-55 (section 15). **Date:** 24 October 2026

## 1. How to read this document

This document answers: **how does another developer run, debug, configure and maintain my module?**

It comes with a **working local setup** in `doc8-local/`. It is laid out like the team repo, so files can be moved across later. I ran everything below on 24 October 2026:

| What | Result | Evidence file |
| --- | --- | --- |
| `make up`: real PostgreSQL 16, roles, 6 migrations, seed, core-api, Traefik 3.1.7 | Ready in about 3 seconds | `evidence/01-make-up.txt` |
| `make demo`: 30 checks through the gateway (access, zone rules, audit, gateway, metrics, logs) | **30 passed** | `evidence/02-demo.txt` |
| `make shutdown-demo`: SIGTERM while requests keep coming | readyz 503 at once, all 20 requests answered 200, clean exit, restart | `evidence/03-shutdown.txt` |
| `make test-db`: Go tests, including race tests on the real database | **41 passed**, 5 runs in a row | `evidence/04-go-tests.txt` |
| Same race test with the advisory lock removed (experiment) | **Fails**: 2 or 3 overlapping zones saved. So the lock is really needed | section 12.2 |
| `make gateway-check`: Docker/Kubernetes parity, Kubernetes schema check, 21 gateway security checks | Parity OK, **20 of 20** resources valid, **21 of 21** checks pass | `evidence/06-gateway-check.txt` |
| Docker Compose file | `docker compose config` passes | — |
| Docker images | **Not built**: no Docker daemon here. The binary was built exactly as the Dockerfile does it, and its self-healthcheck works | section 6 |

**What this is not:** the full SwiftDrop. Other members' services are replaced by small stand-ins, and sign-in uses a local test-token tool (Member 1 owns real sign-in). The ops and courier **screens are not built yet**, because they need endpoints from Members 1 to 5. Section 5.5 says how they will run.

While writing this, I also corrected three small mismatches in Document 7, which now match Document 5: the zone code format (`ZONE_COLOMBO_CENTRAL`), the zone name length (1 to 100 characters) and the setting keys.

Every picture is an image first, with its Mermaid text in a fold-out box underneath. Tags: **\[Repo\]** = in the repository today, **\[Proposed\]** = my proposal, **\[TBD\]** = not decided.

## 2. Languages, frameworks, libraries and versions

| Part | Technology | Version | Source |
| --- | --- | --- | --- |
| Admin backend | Go | **1.22** (CI pins it; local builds also tested with 1.24) | \[Repo\] ci.yml, local-development.md |
| HTTP routing | Go standard `net/http` with 1.22 method patterns (`PATCH /api/v1/admin/zones/{id}`) | stdlib | \[Proposed\] no router library needed |
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
├── docker-compose.m7.yml            # proposal for Member 9's compose file
├── core-api/
│   ├── cmd/
│   │   ├── api/main.go              # ENTRY POINT: the HTTP server
│   │   ├── seed/main.go             # demo rows; refuses APP_ENV=production
│   │   └── devtoken/main.go         # local key pair + test tokens; refuses production
│   ├── internal/admin/              # MY MODULE
│   │   ├── domain/                  # Zone aggregate, ZoneCode, PostalPrefix, Money, overlap rule (no imports)
│   │   ├── application/             # use cases, ports (interfaces), settings allowlist, reason rule
│   │   ├── infrastructure/          # PostgreSQL adapter (pgx): advisory lock, version check, audit insert
│   │   └── transport/               # /api/v1/admin/* handlers, all behind RequireRole(ADMIN)
│   ├── internal/platform/           # shared helpers the admin module needs
│   │   ├── config/                  # reads env vars, fails fast
│   │   ├── auth/                    # RS256 token check (STAND-IN for Member 1's)
│   │   ├── obs/                     # JSON logs, metrics, request middleware
│   │   ├── web/                     # RequireRole, safe error envelope, audit redaction
│   │   └── apperr/                  # error kinds shared by use cases and handlers
│   ├── migrations/                  # 6 up + 6 down files (1 is a users stub for Member 1's table)
│   ├── Dockerfile                   # core-api image (+ seed binary)
│   └── Dockerfile.migrate           # migrations image, same git SHA
├── deploy/
│   ├── gateway/traefik/             # traefik.yml + dynamic-routes.yml (Docker/local)
│   │   └── kubernetes/              # middlewares.yaml + ingressroutes.yaml (my files, Doc 1)
│   └── kubernetes/core-api/         # Deployment, Service, ConfigMap, Secret example, HPA, PDB, migrate Job
├── scripts/                         # local-up, local-down, demo, shutdown-demo, db-bootstrap.sql, gateway_parity.py
├── tools/                           # stand-in services, gateway security test
└── evidence/                        # outputs of the runs in section 1
```

### 3.2 Entry points

| Entry point | Starts | Used by |
| --- | --- | --- |
| `core-api/cmd/api` | HTTP server on `HTTP_ADDR` (`:8080`) | Docker, Kubernetes, local |
| `core-api/cmd/api -healthcheck` | Calls its own `/livez`, exits 0 or 1 | Docker `HEALTHCHECK` (distroless has no curl) |
| `core-api/cmd/seed` | Inserts 7 zones, 3 settings, 3 demo users; safe to repeat | local, staging |
| `core-api/cmd/devtoken` | Makes a dev RSA key pair or signs a test token | local only |
| `migrate ... up` | Applies migrations in timestamp order | local, Compose, Kubernetes Job |
| `traefik --configFile=traefik.yml` | Gateway | local, Compose; Helm in Kubernetes (Member 9) |
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
        A5["web.RequireRole ADMIN"]
    end
    subgraph ADM["internal/admin"]
        T["transport: decode, call, map error"]
        AP["application: use cases + ports"]
        D["domain: Zone rules, no imports"]
        I["infrastructure: PostgreSQL adapter"]
    end
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
| `DATABASE_URL` | PostgreSQL URL for the **app role** (no UPDATE or DELETE on `audit_logs`) | — | yes | **yes** | `postgres://swiftdrop_app:app_dev_only@localhost:5432/swiftdrop_db?sslmode=disable` |
| `MIGRATE_DATABASE_URL` (new) | URL for the **owner role**, used only by migrations and seed | — | for migrations | **yes** | `postgres://swiftdrop_owner:owner_dev_only@localhost:5432/swiftdrop_db?sslmode=disable` |
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

It also refuses `sslmode=disable` when `APP_ENV=production`, and a request timeout of 30 s or more. The start-up log shows the settings **without** secrets (`"database_url":"[set, hidden]"`).

### 4.2 Configuration example (deliverable): `.env.example`

```bash
# SwiftDrop core-api (admin module part) - LOCAL example values only.
# Copy to .env. Never commit .env. Real values live in Kubernetes Secrets.
APP_ENV=local
HTTP_ADDR=:8080
LOG_LEVEL=info

# Runtime role: cannot UPDATE or DELETE audit_logs (Doc 5). Migrations use the owner role.
DATABASE_URL=postgres://swiftdrop_app:app_dev_only@localhost:5432/swiftdrop_db?sslmode=disable
MIGRATE_DATABASE_URL=postgres://swiftdrop_owner:owner_dev_only@localhost:5432/swiftdrop_db?sslmode=disable
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
| PostgreSQL 16 server | `ls /usr/lib/postgresql/16/bin` or `postgres --version` | Ubuntu: `apt install postgresql-16`. Or use Docker Compose (section 6.3) |
| golang-migrate | `migrate -version` | `go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.1` (**the tag is needed**, or you get "unknown driver postgres") |
| Traefik 3.1 | `traefik version` | Download the `v3.1.7` release binary, or set `TRAEFIK=/path/to/traefik` |
| Python 3, curl, make | | For stand-ins and scripts |

### 5.2 Commands

```bash
cd doc8-local
make up              # start everything (section 5.3)
make demo            # 30 checks; needs a fresh database (make wipe && make up)
make shutdown-demo   # graceful shutdown, then restart
make test            # unit tests only, no database
make test-db         # all tests, including real PostgreSQL tests
make lint            # gofmt + go vet
make gateway-check   # gateway parity, Kubernetes schemas, 21 security checks
make build           # static binaries in core-api/bin/
make docker-build    # images (needs a Docker daemon)
make down            # stop;  make wipe = stop and delete the local database
```

### 5.3 What `make up` does

![Figure: local stack](doc8-images/d02-local-stack.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart TB
    S1["1 PostgreSQL 16: initdb if new, start on 5432"] --> S2["2 db-bootstrap.sql: owner and app roles, grants"]
    S2 --> S3["3 migrate up as owner: 6 migrations"]
    S3 --> S4["4 seed as owner: 7 zones, 3 settings, 3 users"]
    S4 --> S5["5 devtoken -genkey: local RSA key pair"]
    S5 --> S6["6 core-api as app role on 8080, wait for readyz"]
    S6 --> S7["7 Traefik on 8088 + stand-ins: tracking-api 8081, web 3000"]
    S7 --> R["Ready: browser and curl use http://localhost:8088"]
    subgraph PORTS["Local ports"]
        P1["8088 gateway, public"]
        P2["8080 core-api: livez, readyz, metrics"]
        P3["8090 Traefik dashboard + ping, internal"]
        P4["5432 PostgreSQL"]
    end
```

</details>

The AI service is **not** started on purpose: its route answers 502, which shows that one missing service does not break the others.

### 5.4 Example session (real output)

```bash
$ TOKEN=$(cd core-api && go run ./cmd/devtoken -role ADMIN)
$ curl -s -H "Authorization: Bearer $TOKEN" -X POST localhost:8088/api/v1/admin/zones \
    -d '{"code":"ZONE_MATARA","name":"Matara","base_surcharge_cents":17500,"postal_prefixes":["81000"]}'
{"data":{"id":"374ddf69-e2ae-4766-91ba-2e1a22b9d1ff","code":"ZONE_MATARA","name":"Matara",
 "base_surcharge_cents":17500,"currency":"LKR","postal_prefixes":["81000"],"enabled":true,"version":1}}

$ curl -s -i localhost:8088/api/v1/admin/zones          # no token
HTTP/1.1 401 Unauthorized
Content-Type: application/json
X-Content-Type-Options: nosniff
X-Request-Id: corr_d5ddc6e95939cd73374064e5
{"error":{"code":"AUTH_REQUIRED","message":"Sign in to continue","correlation_id":"corr_d5ddc6e95939cd73374064e5",...}}

$ curl -s -H "Authorization: Bearer $TOKEN" "localhost:8088/api/v1/admin/audit-logs?limit=1"
{"data":[{"actor_id":"00000000-0000-4000-8000-000000000001","actor_role":"ADMIN","action":"ZONE_CREATED",
 "target_type":"ZONE","target_id":"374ddf69-...","previous_state":null,"new_state":{...},
 "request_id":"corr_3fad748b87248bb497b95406","created_at":"2026-10-24T05:53:30.727458Z"}]}
```

The 30 demo checks (`evidence/02-demo.txt`) cover: health; no token, courier token and tampered token; create, duplicate code, prefix overlap, money as text, unknown field, reason rule, stale version, disable; settings allowlist; 4 audit rows; the app role **cannot** update `audit_logs`; even the owner **cannot** delete them (trigger); `/api/v2` gives JSON 404; AI down gives 502 while the rest works; `X-Request-ID` round trip; metrics; JSON logs with no tokens or passwords.

### 5.5 The screens (when they exist)

```bash
cd web && npm ci && npm run dev        # Vite dev server (local-development.md: port 3000)
npm run test:unit                      # Vitest (Doc 6)
npx playwright test                    # E2E against make up (Doc 6)
```

During development, Vite should forward `/api` to the gateway (`server.proxy` in `vite.config.ts`: `/api` → `http://localhost:8088`). Then the browser sees **one origin**, cookies work with `SameSite=Strict`, and CORS is not needed **\[Proposed\]**. Note the port conflict: local-development.md runs Vite on 3000, the gateway CORS list also allows 5173 (Vite's default) **\[TBD U-58\]**.

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
| Non-root (SEC-012) | `USER 10001:10001` |
| Read-only root file system | The app writes nothing to disk (logs go to stdout) |
| No secrets in the image | `.dockerignore` drops `.dev`, `*.pem`, `.env` |
| Health check without curl | `/core-api -healthcheck` calls `/livez`; tested: exit 0 when up, 1 when not |
| Tag | git SHA from CI, for example `ghcr.io/<org>/swiftdrop/core-api:40dc279` |

### 6.2 Migrations image

`core-api/Dockerfile.migrate` = `migrate/migrate:v4.17.1` + this release's SQL files, tagged with the **same git SHA**. So the migration Job always runs exactly the schema that the new code expects.

### 6.3 Docker Compose (proposal for Member 9)

`docker-compose.m7.yml` runs PostgreSQL (bootstrap SQL as an init script), the migrations one-shot, core-api (read-only, `no-new-privileges`) and Traefik. Health checks gate the order: PostgreSQL healthy → migrate finished → core-api starts. `docker compose config` passes. Ports for the database and dashboard bind to `127.0.0.1` only.

```bash
cd core-api && go run ./cmd/devtoken -genkey .dev && cd ..
docker compose -f docker-compose.m7.yml up -d --build
docker compose -f docker-compose.m7.yml run --rm --entrypoint /seed core-api
```

### 6.4 Health endpoints

| Endpoint | Means | Checks | Answer |
| --- | --- | --- | --- |
| `GET /livez` | "The process is alive" | Nothing outside the process | 200 `{"status":"UP"}` |
| `GET /readyz` | "Send me traffic" | Database ping (1 s), not shutting down | 200 `{"status":"READY","database":"UP"}`; 503 `{"status":"NOT_READY","database":"DOWN"}`; 503 `{"status":"SHUTTING_DOWN"}` |
| `GET /metrics` | Prometheus scrape | — | text format |

All three are on the pod port only. The gateway never routes them (Doc 6 GW-08, GW-09; demo DEMO-03).

## 7. Kubernetes deployment

### 7.1 Files

All 20 resources pass `kubeconform -strict` against Kubernetes 1.30 and the Traefik CRD schemas.

| File | Kind | Owner |
| --- | --- | --- |
| `deploy/kubernetes/core-api/deployment.yaml` | Deployment | Member 9 (proposal from me) |
| `.../service.yaml` | Service (ClusterIP 8080) | Member 9 |
| `.../configmap.yaml` | ConfigMap `core-api-config` | Member 9 |
| `.../secret.example.yaml` | Secret example only | Member 9 |
| `.../hpa.yaml` | HorizontalPodAutoscaler | Member 9 |
| `.../pdb.yaml` | PodDisruptionBudget | Member 9 |
| `.../migrate-job.yaml` | Job | Member 9 |
| `deploy/gateway/traefik/kubernetes/middlewares.yaml` | 9 Middleware + 3 ServersTransport | **Me** |
| `deploy/gateway/traefik/kubernetes/ingressroutes.yaml` | IngressRoute (8 routes with explicit priorities) | **Me** |

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
        SEC["Secrets postgres-credentials, jwt-public-key"]
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
| startupProbe | `/livez` | every 2 s, up to 15 tries (30 s) | Slow first start does not trigger liveness restarts |
| readinessProbe | `/readyz` | every 5 s, 2 failures | Database down or shutting down → pod leaves the Service, no restart |
| livenessProbe | `/livez` | every 10 s, 3 failures | Only a stuck process is restarted. **It must not check the database**: a database outage would otherwise restart every pod at once and make things worse |

I tested this locally: with PostgreSQL stopped, `/readyz` answered 503 and `/livez` 200; admin requests answered **503 `SERVICE_UNAVAILABLE`** (not a hang, not a crash). When PostgreSQL came back, requests worked again **without a restart**.

### 7.3 Resources and security context

| Setting | Value | Reason |
| --- | --- | --- |
| CPU request | 100m | HPA percentage is counted against this |
| CPU limit | none | CPU limits throttle Go and add latency; memory is the real risk on one node |
| Memory request / limit | 128Mi / 256Mi | Admin traffic is small; `GOMEMLIMIT=200MiB` makes Go clean up first |
| `runAsNonRoot`, user 10001, `readOnlyRootFilesystem`, drop ALL capabilities, seccomp RuntimeDefault | on | SEC-012 |
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
    J->>DB: migrate up as owner role
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

The IngressRoute uses explicit `priority` values. **Lesson from Doc 7:** Traefik ranks routes by the length of the rule text unless you set priority, and that once hid my SSE limit. The TLS secret is `cloudflare-origin-cert` (deployment-guide.md), but deploy/gateway/README says `traefik-origin-tls` **\[TBD U-60\]**. The dashboard is never exposed; use `kubectl port-forward -n traefik svc/traefik 8090:8090` **\[TBD U-48: port\]**.

## 8. Logging, metrics, tracing and alerts

### 8.1 Logs

One JSON object per line on stdout. Kubernetes collects them. Real line:

```json
{"timestamp":"2026-10-24T05:49:27.522811997Z","level":"INFO","msg":"request","service":"core-api","module":"admin",
 "request_id":"corr_996063b746a15d1a6238a665","method":"GET","route":"GET /api/v1/admin/zones","status":200,
 "duration_ms":0.671,"actor_id":"00000000-0000-4000-8000-000000000001","actor_role":"ADMIN"}
```

| Field | Meaning |
| --- | --- |
| `request_id` | Same as the `X-Request-ID` header and the `correlation_id` in error bodies and audit rows. **Search by this first** |
| `route` | The route pattern, not the raw path (no IDs, so no personal data and few distinct values) |
| `actor_id`, `actor_role` | From the verified token |
| `security_event: access_denied` | Added on 401 and 403 (Doc 7 10.1) |
| `level` | `ERROR` for 5xx (with a separate `admin request failed` line holding the cause), `INFO` for normal requests, `DEBUG` for probes and metrics scrapes |

**Never logged:** tokens, passwords, database URLs, request bodies, recipient data. Demo check DEMO-30 searches the log for tokens and passwords and finds none.

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

Today: **correlation ID only**. The gateway does not create one; core-api reuses a well-formed `X-Request-ID` from the client or makes a new one, returns it in the response header, puts it in every log line, error body and audit row. Proven by DEMO-26 and DEMO-29. OpenTelemetry (`OTEL_EXPORTER_OTLP_ENDPOINT` in environment-variables.md) is **not** implemented in this slice **\[TBD U-61\]**.

### 8.4 Alerts (proposed for Member 9's Prometheus)

| Alert | Rule (PromQL idea) | For | Severity | First step |
| --- | --- | --- | --- | --- |
| Admin API errors | `sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) > 0.05` | 5 m | page | Logs at `level=ERROR`, then runbook Playbook A |
| Slow admin API | p95 of `http_request_duration_seconds` > 1 s | 10 m | warn | Check report queries and DB load |
| Pods not ready | `kube_deployment_status_replicas_available{deployment="core-api"} < 2` | 5 m | page | `kubectl describe pod`; readyz output |
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
    Q -->|"403"| A2["Role is not ADMIN. Normal for couriers and dispatchers"]
    Q -->|"409"| A3["Version or prefix conflict. Reload. If constant, check admin_conflicts_total"]
    Q -->|"502"| A4["Backend pod down or unreachable. kubectl get pods"]
    Q -->|"503"| A5{"Body code?"}
    A5 -->|"SERVICE_UNAVAILABLE"| A6["Database down or slow. readyz, pg_isready, runbook A"]
    A5 -->|"TRY_AGAIN"| A7["Lock wait over 2 s. Find long transaction"]
    Q -->|"504"| A8["core-api took over 30 s. Search logs by request_id"]
    Q -->|"500"| A9["Search logs for the correlation_id, read the admin request failed line"]
```

</details>

| Symptom | Likely cause | Check | Fix |
| --- | --- | --- | --- |
| Process exits at once with "bad configuration" | Missing or wrong env var | The message lists them all | Set the variables (section 4) |
| Every request 401, also for admins | `JWT_ISSUER` does not match the token's `iss`, or the wrong public key | Decode the token payload (middle part, base64) | Same issuer and key pair as Member 1's sign-in |
| `/readyz` 503 `database DOWN` | PostgreSQL down, wrong URL or password, network policy | `pg_isready -h <host>`; core-api error log line | Fix DB; the pod recovers by itself (tested) |
| Admin requests 503 `SERVICE_UNAVAILABLE` | Same as above, or a query hit `statement_timeout` | Log line `admin request failed` has the cause | As above; for timeouts check the query |
| 503 `TRY_AGAIN` on zone saves | Another transaction holds the prefix lock over 2 s | `SELECT pid, state, xact_start, query FROM pg_stat_activity WHERE state <> 'idle' ORDER BY xact_start;` | End the stuck session (runbook Playbook A) |
| 409 `CONCURRENT_MODIFICATION` every time | Screen sends an old `version` | Compare with `GET /admin/zones/{id}` | Reload before editing (Doc 7 section 9.2) |
| `permission denied for table audit_logs` | App tried to change audit rows (a bug), or migrations ran as the app role | Which role is in the URL | Migrations must use `MIGRATE_DATABASE_URL` (owner) |
| `audit_logs is append-only` | Someone ran UPDATE or DELETE | Who ran it | Expected. Fix forward with a new change |
| `migrate: unknown driver postgres` | migrate built without the tag | `migrate -version` | Reinstall with `-tags postgres` |
| `migrate: Dirty database version N` | A migration failed half way | `SELECT * FROM schema_migrations;` | Fix the SQL, then `migrate force <N-1>` and `up` (local-development.md 5) |
| `/api/v1/...` returns HTML | Request reached the SPA: gateway route missing | Traefik dashboard (internal) or `make gateway-check` | Fix the route; the `/api` catch-all should give JSON 404 |
| 413 on save | Body over 1 MB on a JSON route | Request size | Correct; photos use the upload route (10 MB) |
| Many 429 for one office | Everyone shares one IP | Traefik access log | Raise the limit or add an office rule (Doc 7 FM-04) |
| Pod restarted, `OOMKilled` | Memory over 256Mi | `kubectl describe pod` | Raise the limit; check for large report queries |
| Logs show no `actor_id` | Request had no valid token | `security_event` field | Expected for 401 |

**Useful commands**

```bash
kubectl -n swiftdrop get pods -l app.kubernetes.io/name=core-api
kubectl -n swiftdrop logs deploy/core-api --since=15m | grep '"request_id":"corr_...'
kubectl -n swiftdrop logs <pod> --previous                       # after a crash
kubectl -n swiftdrop port-forward deploy/core-api 8080:8080       # then curl localhost:8080/readyz /metrics
kubectl -n swiftdrop rollout undo deployment/core-api             # runbook 7
psql "$MIGRATE_DATABASE_URL" -c "SELECT action, target_id, actor_id, reason, request_id, created_at FROM audit_logs ORDER BY created_at DESC LIMIT 20;"
```

## 10. External dependencies

| Dependency | Needed for | If it is down |
| --- | --- | --- |
| PostgreSQL 16 | Everything the admin module does | `/readyz` 503, requests 503, automatic recovery |
| Member 1's sign-in and key pair | Real tokens; my code only needs the **public** key | Users cannot sign in; already signed-in users work until their 15-minute token ends |
| Traefik (Members 7 and 9) | Only way in from outside | Site unreachable; pods healthy |
| Cloudflare | TLS, DDoS, real client IP | As above |
| Prometheus and Grafana (Member 9) | Metrics and alerts | No effect on users; blind operations |
| Other modules (pricing, dispatch, merchants) | Reports and merchant approval in later work, through Go interfaces in the same process (Doc 3 DD-09) | Same process, so no network failure; their database tables matter |
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
    K->>P: startupProbe /livez = 200
    K->>P: readinessProbe /readyz
    P->>DB: ping
    DB-->>P: ok
    P-->>K: 200 READY
    K->>T: add pod to Service endpoints
    Note over K,T: normal traffic
    K->>P: SIGTERM (rollout or scale down)
    P->>P: readyz now 503 SHUTTING_DOWN
    K->>T: remove pod from endpoints
    P->>P: wait READINESS_DRAIN_SECONDS 5 s, still serving
    P->>P: stop accepting, finish running requests, up to 20 s
    P->>DB: close pool
    P-->>K: exit 0, within terminationGracePeriod 30 s
```

</details>

**Start-up rules**

1. Bad configuration stops the process **before** it opens anything (fail fast).
2. A late database does **not** crash the process. It starts, `/readyz` says 503, and it becomes ready when the database answers. This avoids crash loops when the whole stack starts together.
3. Migrations are **not** run by core-api at start-up. They run once, in the Job, before the rollout. Two pods starting together would otherwise race on the schema.

**Shutdown, proven locally** (`evidence/03-shutdown.txt`): 1 second after SIGTERM, `/readyz` answered `SHUTTING_DOWN` while `/livez` stayed `UP`. All 20 requests sent during the drain got 200. The process exited about 7 seconds after the signal, then restarted ready.

## 12. Maintaining the module

### 12.1 Everyday tasks

| Task | Steps |
| --- | --- |
| Add an admin endpoint | Add a use case in `application`; handler in `transport`; register it in `Register` (it is wrapped in `RequireRole` automatically); update `API.md` and `README.md` (AGENTS.md); add tests for success, 401, 403 and the error cases |
| Add a migration | `migrate create -ext sql -dir core-api/migrations -format 20060102150405 admin_<name>` (Doc 5 U-40); write **up and down**; expand first, contract later; run `make wipe && make up && make test-db` |
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
| `TestAuditInSameTransaction` | If the audit row fails, the zone is not saved either |
| `TestRequireRole_*`, `TestVerify` | Roles, token checks (wrong key, expired, `alg:none`, edited payload) |
| `TestLoad_*` | Configuration fails fast and never prints secrets |
| demo checks 21 and 22 | Database grants and trigger keep `audit_logs` append-only |

While writing the race test I first made a mistake: some of my test prefixes did not overlap each other, so several zones were rightly saved. The test now uses `655`, `6550` and `65500`, which all overlap. The lesson: a race test must be checked by removing the protection and watching it fail, as I did.

## 13. Known limitations

1. **Not in the team repo yet.** This is a working copy; moving it in needs the team's `go.mod` and wiring (U-10).
2. **Sign-in is a stand-in.** `platform/auth` checks RS256 tokens like ADR-003 says, but Member 1's middleware replaces it. No refresh tokens, no key rotation, no `jti` blocklist.
3. **No Idempotency-Key storage.** Waits for the shared table (U-24, U-39). Zone creation is still protected by the unique code.
4. **One outside library** (pgx) against the "no new dependencies" rule (U-55).
5. **Reports, merchant approval and pricing rules** are not in this slice; they need Members 2 to 4.
6. **No OpenTelemetry tracing**; correlation ID only (U-61).
7. **Docker images not built here**; only the Compose file is checked and the binary is built the same way.
8. **Kubernetes files are schema-checked, not applied** to a cluster.
9. **Screens not built.**
10. **Seed users are a stub** of Member 1's `users` table.
11. **Traefik rate limits are per Traefik pod**, in memory.

## 14. Conflicts found in repo documents

| Topic | Says A | Says B | Question |
| --- | --- | --- | --- |
| Clone URL | local-development.md and deployment-guide.md: `github.com/Prashani-beep/SwiftDrop` | Team repo: `github.com/phantomTroupeEAD/SwiftDrop` | Fix the docs (Member 9) |
| Missing files | deployment-guide.md applies `deploy/kubernetes/base/*.yaml`, `values.yaml`, `middlewares.yaml`, `ingressroutes.yaml`, `docker-compose.yml`, `.env.example`, `Makefile`, `deploy.yml` | None of these exist in the repo (commit 40dc279) | My section 7 files can fill part of it |
| Frontend API variable | web/README: `VITE_API_BASE_URL` | environment-variables.md: `VITE_API_GATEWAY_URL` | U-58 |
| Vite port | local-development.md: 3000 | Gateway CORS list: 3000 and 5173 | U-58 |
| Client tenant header | web/README 4: "`X-Tenant-ID` is passed" | Doc 7: tenant only from the token; gateway strips it | Fix web/README (Member 6) |
| TLS secret name | deployment-guide.md: `cloudflare-origin-cert` | deploy/gateway/README: `traefik-origin-tls` | U-60 |
| Dashboard port | runbook 6: 9000 | Member 9: 8090; repo: 8080 | U-48 |
| Readiness body | runbook 1: `{"status":"READY","database":"UP"}` | — | I follow the runbook exactly |
| Role names in the SPA | web/README: `ROLE_ADMIN` etc. | ADR-003: `ADMIN` etc. | Screens must use the token's `ADMIN` |

## 15. Open questions

Earlier questions that affect running the module: U-10, U-24, U-39, U-40, U-46, U-48.

| ID | Question | Decide with |
| --- | --- | --- |
| U-55 | Which PostgreSQL driver for core-api: `pgx/v5` (my choice: fast, good errors, pool built in) or `database/sql` + `lib/pq`? AGENTS.md forbids new libraries without agreement | Members 1 to 4, Member 9 |
| U-56 | Shared error kinds package (`platform/apperr`) that `application` layers may import, or one set per module? (AGENTS.md: application depends only on domain) | Backend members |
| U-57 | Add the new variables (section 4.1) to environment-variables.md | Member 9 |
| U-58 | One frontend API variable name and one Vite dev port; use the Vite proxy so CORS is not needed locally | Member 6, Member 9 |
| U-59 | Real CPU and memory numbers for core-api after a load test | Member 9 |
| U-60 | One name for the Cloudflare origin TLS secret | Member 9 |
| U-61 | OpenTelemetry tracing now, or correlation IDs only for the project? | Member 9, Member 10 |
| U-62 | Member 1's token middleware must accept two public keys during rotation (security-notes 5) | Member 1 |

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
