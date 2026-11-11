# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

SwiftDrop is a 10-member academic project: a parcel delivery & courier management system. **`AGENTS.md` is the binding rulebook** (precedence: SRS in `docs/intial/` > `docs/contracts/` + `docs/architecture/` > Engineering Standards > `AGENTS.md` > existing code). Read it and `docs/architecture/domain-boundaries.md` before changing business logic. Frontend work also follows `web/CLAUDE.md` / `.cursor/rules/frontend.mdc`.

## What actually exists vs. what the docs describe

Much of the documentation (AGENTS.md, module READMEs, `docs/drafts/`) describes the *target* design. Check the code before relying on a doc claim.

| Area | State on this branch |
| :--- | :--- |
| `core-api/internal/identity`, `merchants`, `events`, `platform` | Implemented Go code |
| `core-api/internal/parcels`, `pricing`, `dispatch`, `delivery`, `admin` | `README.md` + `API.md` only (work lives on other members' feature branches) |
| `services/messaging` | Implemented (separate Go module) |
| `services/tracking` | `README.md` + `API.md` only; no code, no Dockerfile |
| `services/ai-assistant` | Implemented (Python/FastAPI + FastMCP) |
| `web` | Shell only (`src/features/auth`, `dashboards`); `package.json` has just `dev`, `build`, `preview`. The `lint` / `test:unit` / `test:e2e` scripts in `web/CLAUDE.md` do not exist yet |
| `tests/e2e` | Cross-module Playwright **TypeScript** suite (Member 10) |
| `docs/drafts/**` | Per-member design drafts, including draft `go.mod`/Dockerfiles. Not built; CI keys Go jobs on `core-api/go.mod` |

Branch flow: feature branches → `dev` → `main` (CI runs on push/PR to `main` and `dev`).

## Commands

**Repo root**
```bash
./scripts/verify.sh            # Windows: .\scripts\verify.ps1 — secret scan, gofmt, golangci-lint, go test -race (core-api + messaging)
make up | down | ps | logs | reset   # full stack via deploy/docker-compose.yml; Windows: .\scripts\make.ps1 <target>
                               # overrides: deploy/.env (from deploy/.env.example) — Compose ignores a root .env
make swagger                   # regenerate core-api/docs/swagger.json from swag annotations and copy it to internal/events/swagger/
```
`make test-e2e` (runs `pytest`) and `make seed` (re-runs the migrator) are stale; use the commands below.

**core-api** (Go 1.22, module `github.com/swiftdrop/core-api`)
```bash
cd core-api
go run ./cmd/api                                   # :8080 (PORT env); see "Startup behaviour" below
go run ./cmd/migrate                               # apply embedded migrations to DATABASE_URL
go test -race ./...
go test -race -run TestName ./internal/identity/...   # single test
golangci-lint run ./...                            # uses root .golangci.yml (depguard layer rules)
# Integration tests (opt-in):
docker compose -f ../services/messaging/docker-compose.dev.yml up -d postgres
INTEGRATION=1 TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/swiftdrop?sslmode=disable" go test -v -count=1 ./tests/...
```

**services/messaging** (separate Go module `github.com/swiftdrop/messaging`)
```bash
cd services/messaging
make test                      # unit tests
make it                        # INTEGRATION=1 go test ./tests/... (needs `make up` → docker-compose.dev.yml)
go run ./cmd/relay             # also ./cmd/worker, ./cmd/reconcile; dev helpers ./cmd/devemit, ./cmd/fake-receiver
```

**services/ai-assistant** (Python 3.11)
```bash
cd services/ai-assistant
pip install -r requirements.txt
uvicorn src.main:app --port 8000 --reload          # MOCK_LLM_MODE=true USE_MOCK_UPSTREAM=true to run offline
python -m pytest -v                                 # single: python -m pytest tests/unit/test_x.py::test_name
```
The container runs `entrypoint.sh dual`: REST on :8000 and the FastMCP SSE server on :8002 (`api` / `mcp` modes run one of them).

**tests/e2e**: `npm install`, then `npx playwright test` (or `npm test`). `npm run test:dry-run` runs a smoke spec, and `npm run start:webhook` starts the mock merchant webhook receiver.

**web**: `npm run dev` / `npm run build` (`tsc && vite build`).

## Architecture

**Core API composition root** is `core-api/cmd/api/wiring.go` (`BuildAppContainer`) plus one `wiring_<module>.go` per module. Every module registers routes on a single `http.ServeMux` using Go 1.22 method patterns (`"GET /api/v1/..."`). AGENTS.md mentions `internal/app/wiring.go`, but that file does not exist.

**Startup behaviour:** core-api pings `DATABASE_URL`.
- If Postgres is reachable, it **auto-runs the embedded migrations** and uses the Postgres repositories.
- If not, identity falls back to in-memory mock repositories with seeded demo users (see `migrations/20261021000002_seed_mock_users.up.sql`). The merchants module stays disabled, and that is fatal when `APP_ENV=production`.

**Layer rules are lint-enforced** (`.golangci.yml` depguard):
- `domain/` may not import `net/http`, `database/sql`, `amqp091-go`, `infrastructure/` or `transport/`.
- `application/` may not import `net/http` or `transport/`.
- `internal/platform` may not import any business module.
- **No core-api code may import the RabbitMQ client at all.**

**Event flow (transactional outbox):**
1. core-api writes the state change and an `outbox_events` row in one transaction (`internal/events`).
2. The relay (`services/messaging/cmd/relay`) publishes to RabbitMQ. It is a strict singleton.
3. The worker consumes `q.notifications.worker` and `q.webhooks.dispatcher` (DLQ `swiftdrop.dlq`). Consumers must be idempotent.
4. `reconcile` runs as a CronJob and sweeps stale rows.

The relay, worker and reconcile probes serve `/health/live`, `/health/ready` and `/metrics` on `PROBE_PORT` (defaults 8082, 8083, 8084).

**Migrations** live in `core-api/migrations/`.
- They are embedded with `go:embed` and applied in lexical order by a custom migrator, which keeps a `schema_migrations(version, name, applied_at)` ledger.
- `migrations_test.go` enforces:
  - **14-digit timestamp names** `YYYYMMDDHHMMSS_name.up.sql`. AGENTS.md's `NNNNNN_` format is outdated.
  - a matching `.down.sql` for every `.up.sql`;
  - no transaction statements or `CONCURRENTLY`.
- `sql_guard_test.go` parses the SQL in `core-api/internal/events` and `services/messaging` and checks it against its own column map for `outbox_events` and the worker tables. Update that map when you change those tables.
- Compose's `core-api-migrate` service runs golang-migrate against the same `schema_migrations` table name, which conflicts with the custom ledger.

**Runtime topology:** Traefik routes by path on one domain (`deploy/gateway/traefik/dynamic-routes.yml`):

| Path | Service |
| :--- | :--- |
| `/api/v1/tracking` | tracking-api:8081 |
| `/api/v1/ai` | ai-assistant:8000 |
| `/mcp` | ai-assistant:8002 |
| `/api/v1` | core-api:8080 |
| `/` | web:3000 (Nginx; `/healthz`) |

Health endpoints stay internal and are never routed. Core API serves `/health/live` and `/health/ready` (the ready check pings the DB). Kubernetes manifests are per-service folders under `deploy/kubernetes/<service>/` (target: K3s, see ADR-002).

**API conventions** (`docs/contracts/api-conventions.md`):
- Responses use `{"data": ...}`.
- Errors use `{"error": {code, message, correlation_id, ...}}`.
- Booking-style POSTs need an `Idempotency-Key`.
- Every core-api handler needs `swag` annotations; run `make swagger` after you change endpoints.

## Invariants that are easy to break

- Parcel status changes only go through the `Parcel` aggregate. No raw `UPDATE parcels SET status`. The state change, `parcel_status_history` and `outbox_events` commit in one transaction.
- Money is integer minor units (LKR cents), never floats.
- The frontend never computes fees (use `POST /api/v1/pricing/quote`) and never decides parcel transitions.
- Access tokens live in memory only. Refresh tokens stay in an HttpOnly cookie. Nothing goes in `localStorage`.
- Public tracking redacts recipient phone, full address and proof attachments.
- Scaling:
  - the outbox relay is always `replicas: 1`;
  - Postgres and RabbitMQ are StatefulSets and are never scaled;
  - core-api's `max replicas × pool size` must stay within Postgres `max_connections` (150);
  - event-worker scales with KEDA, so its consumers must be idempotent.
- Module ownership is in AGENTS.md §5. Don't change another member's module, contract or migration without coordinating.
- Living docs: every code change updates that module's `README.md`, and its `API.md` if it has endpoints, in the same change. Finish with the Task Completion Report from AGENTS.md §12.
