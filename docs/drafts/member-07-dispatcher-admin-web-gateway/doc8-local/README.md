# Member 7 module: admin backend + gateway (local, runnable)

This folder is a **working copy of my parts of SwiftDrop**, laid out like the team repo so it can be moved in later:

| Part | Folder here | Folder in the repo |
| --- | --- | --- |
| Admin backend module (zones, postal prefixes, public zone list, settings, reports, audit, merchant admin routes) | `core-api/internal/admin/` | same |
| Stand-in for Member 4's merchants facade (`AdminService` interface + local fake) | `core-api/internal/merchants/` | Member 4's real module |
| Small platform pieces the admin module needs (config, JWT check, logs, metrics, errors) | `core-api/internal/platform/` | same (Member 1 owns `auth`; mine is a stand-in) |
| Entry points | `core-api/cmd/api`, `cmd/seed`, `cmd/devtoken` | same |
| Migrations `000002_create_delivery_zones`, `000011_create_audit_logs` | `core-api/migrations/` | same (6-digit numbers, team plan) |
| Local stub `users` table (Member 1's `000001` stands in) | `scripts/dev-stub-users.sql` | never in the repo migrations (Member 9 D4) |
| Compose needs for Member 9 | `proposals/compose-needs-for-member-09.md` | merged by Member 9 into `deploy/docker-compose.yml` (A7) |
| Gateway config (Docker / local) | `deploy/gateway/traefik/*.yml` | same |
| Gateway config (Kubernetes CRDs) | `deploy/gateway/traefik/kubernetes/` | same (Member 7) |
| core-api Kubernetes files | `deploy/kubernetes/core-api/` | proposal for Member 9 |
| Ops and dispatcher screens | not built yet | `web/src/features/dispatcher-admin/` |

Full guide: **Document 8** (Implementation, Deployment & Operations).

## Quick start (no Docker needed)

Needs: Go 1.22+, PostgreSQL 16 server binaries, `migrate` v4.17 built with `-tags postgres`, Traefik v3.1 binary, Python 3, curl.

```bash
make up              # PostgreSQL -> roles -> migrations -> seed -> core-api -> Traefik   (~3 s)
make demo            # 51 checks through the gateway (needs a fresh database: make wipe && make up)
make shutdown-demo   # SIGTERM -> /health/ready 503 -> requests finish -> exit -> restart
make test-db         # 46 Go tests, including real PostgreSQL race tests
make gateway-check   # file/Kubernetes parity + schema check + 25 gateway security checks
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
| http://localhost:8080/health/live, /health/ready, /metrics | core-api pod endpoints, Member 9 A2 (never through the gateway) |
| http://localhost:8090/dashboard/ | Traefik dashboard, internal port only |
| `/tmp/m7-local/logs/` | core-api and Traefik logs (JSON) |

## Configuration

Copy `.env.example` to `.env`. Every variable is described in Document 8 section 3. The process **refuses to start** when a required value is missing or wrong, and lists every problem at once. It never prints secrets.

## API

Zones hold **no money**: fees, including any zone surcharge, are Member 2's pricing rules. Zone ids are text, such as `ZONE_COLOMBO_01`, because Members 2 and 3 point at them with `VARCHAR(32)` foreign keys.

| Method and path | Who | Does |
| --- | --- | --- |
| `GET /api/v1/zones` | anyone | Enabled zones (`id`, `code`, `name`) for booking forms; `?postal_code=00300` finds one zone or 404 `ZONE_NOT_FOUND`; cached 5 minutes |
| `GET /api/v1/admin/reports/deliveries?from=2026-10-01&to=2026-10-07` | ADMIN, DISPATCHER | Deliveries per day and success rate. Locally the data is a stand-in and the response says so in `source` |
| `GET /api/v1/admin/merchants?status=&q=&page=&per_page=` | ADMIN | Merchant list (Member 4's rules, local fake) |
| `GET /api/v1/admin/merchants/{id}` | ADMIN | One merchant |
| `POST /api/v1/admin/merchants/{id}/{approve,reject,suspend,reinstate,disable}` | ADMIN | Body `{expected_version, reason}`; reason (5 to 500 characters) needed for reject, suspend, disable. Member 4's error codes are passed through |

All routes below need an ADMIN token.

| Method and path | Does |
| --- | --- |
| `GET /api/v1/admin/zones` | List zones with prefixes |
| `POST /api/v1/admin/zones` | Create zone `{id, code, name, description, postal_prefixes}` |
| `GET /api/v1/admin/zones/{id}` | One zone |
| `PATCH /api/v1/admin/zones/{id}` | Change code, name, description or prefixes; needs `version` (one save = version + 1) |
| `PATCH /api/v1/admin/zones/{id}/status` | Enable or disable; needs `version` and `reason` |
| `GET /api/v1/admin/settings` | List settings |
| `PUT /api/v1/admin/settings/{key}` | Change an allowlisted setting; needs `version` and `reason` |
| `GET /api/v1/admin/audit-logs?limit=50` | Newest audit rows (read only) |

Errors use the team envelope (`docs/contracts/api-conventions.md` 2.3): 400 bad JSON or unknown field, 401, 403, 404 (`ZONE_NOT_FOUND`, `MERCHANT_NOT_FOUND`), 409 (`ZONE_CODE_ALREADY_EXISTS`, `ZONE_POSTAL_CODE_CONFLICT`, `CONCURRENT_MODIFICATION`, `MERCHANT_VERSION_CONFLICT`, `MERCHANT_INVALID_TRANSITION`), 422, 503 `TRY_AGAIN` (lock timeout).

## Database roles

`scripts/db-bootstrap.sql` makes the two logins from Member 9's agreement D6 before any migration: `swiftdrop_migrator` (owns the schema, runs migrations) and `swiftdrop_app` (core-api; SELECT, INSERT, UPDATE by default; DELETE only on `zone_postal_codes`; INSERT and SELECT only on `audit_logs`). In Kubernetes the URLs come from the Secrets `postgres-secrets` and `postgres-migrator-secrets`, with `sslmode=disable` inside the cluster.

## What is not here yet

- Real sign-in (Member 1). `cmd/devtoken` signs local test tokens and refuses `APP_ENV=production`.
- Idempotency-Key storage (shared platform table, U-24, U-39).
- The React screens. They need the backend endpoints of Members 1 to 5 first.
- Real delivery numbers for the report (Members 2 and 3 read interfaces) and Member 4's real merchants module. Stand-ins are used and named.
- Docker images were **not built** in my environment (no Docker daemon). The binary was built the same way the Dockerfile builds it. There is no Compose file here on purpose (Member 9 A7, K10); my needs are in `proposals/`.
