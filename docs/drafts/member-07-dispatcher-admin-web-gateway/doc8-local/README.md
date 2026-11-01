# Member 7 module: admin backend + gateway (local, runnable)

This folder is a **working copy of my parts of SwiftDrop**, laid out like the team repo so it can be moved in later:

| Part | Folder here | Folder in the repo |
| --- | --- | --- |
| Admin backend module (zones, postal prefixes, settings, audit) | `core-api/internal/admin/` | same |
| Small platform pieces the admin module needs (config, JWT check, logs, metrics, errors) | `core-api/internal/platform/` | same (Member 1 owns `auth`; mine is a stand-in) |
| Entry points | `core-api/cmd/api`, `cmd/seed`, `cmd/devtoken` | same |
| Migrations | `core-api/migrations/` | same |
| Gateway config (Docker / local) | `deploy/gateway/traefik/*.yml` | same |
| Gateway config (Kubernetes CRDs) | `deploy/gateway/traefik/kubernetes/` | same (Member 7) |
| core-api Kubernetes files | `deploy/kubernetes/core-api/` | proposal for Member 9 |
| Ops and dispatcher screens | not built yet | `web/src/features/dispatcher-admin/` |

Full guide: **Document 8** (Implementation, Deployment & Operations).

## Quick start (no Docker needed)

Needs: Go 1.22+, PostgreSQL 16 server binaries, `migrate` v4.17 built with `-tags postgres`, Traefik v3.1 binary, Python 3, curl.

```bash
make up              # PostgreSQL -> roles -> migrations -> seed -> core-api -> Traefik   (~3 s)
make demo            # 30 checks through the gateway (needs a fresh database: make wipe && make up)
make shutdown-demo   # SIGTERM -> readyz 503 -> requests finish -> exit -> restart
make test-db         # 41 Go tests, including real PostgreSQL race tests
make gateway-check   # file/Kubernetes parity + schema check + 21 gateway security checks
make down            # stop (make wipe also deletes the local database)
```

Then:

```bash
TOKEN=$(cd core-api && go run ./cmd/devtoken -role ADMIN)
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8088/api/v1/admin/zones
```

| URL | What |
| --- | --- |
| http://localhost:8088 | Gateway (what the browser uses) |
| http://localhost:8080/livez, /readyz, /metrics | core-api pod endpoints (never through the gateway) |
| http://localhost:8090/dashboard/ | Traefik dashboard, internal port only |
| `/tmp/m7-local/logs/` | core-api and Traefik logs (JSON) |

## Configuration

Copy `.env.example` to `.env`. Every variable is described in Document 8 section 3. The process **refuses to start** when a required value is missing or wrong, and lists every problem at once. It never prints secrets.

## API (admin, all need an ADMIN token)

| Method and path | Does |
| --- | --- |
| `GET /api/v1/admin/zones` | List zones with prefixes |
| `POST /api/v1/admin/zones` | Create zone `{code, name, base_surcharge_cents, postal_prefixes}` |
| `GET /api/v1/admin/zones/{id}` | One zone |
| `PATCH /api/v1/admin/zones/{id}` | Change name, surcharge (reason required) or prefixes; needs `version` |
| `PATCH /api/v1/admin/zones/{id}/status` | Enable or disable; needs `version` and `reason` |
| `GET /api/v1/admin/settings` | List settings |
| `PUT /api/v1/admin/settings/{key}` | Change an allowlisted setting; needs `version` and `reason` |
| `GET /api/v1/admin/audit-logs?limit=50` | Newest audit rows (read only) |

Errors use the team envelope (`docs/contracts/api-conventions.md` 2.3): 400 bad JSON or unknown field, 401, 403, 404, 409 (`ZONE_CODE_ALREADY_EXISTS`, `ZONE_POSTAL_CODE_CONFLICT`, `CONCURRENT_MODIFICATION`), 422, 503 `TRY_AGAIN` (lock timeout).

## What is not here yet

- Real sign-in (Member 1). `cmd/devtoken` signs local test tokens and refuses `APP_ENV=production`.
- Idempotency-Key storage (shared platform table, U-24, U-39).
- The React screens. They need the backend endpoints of Members 1 to 5 first.
- Docker images were **not built** in my environment (no Docker daemon). The Compose file passes `docker compose config`, and the binary was built the same way the Dockerfile builds it.
