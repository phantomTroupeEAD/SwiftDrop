# Member 4 merchants module — local, runnable slice

A **working copy of the merchants module** inside a minimal `core-api`, laid out like the team repo so it can be moved in later. Full guide: [Document 8](../doc-08-implementation-deployment-and-ops.md).

| Part | Folder here | Folder in the repo | Owner |
| --- | --- | --- | --- |
| Merchants module (domain, application, infrastructure, transport) | `core-api/internal/merchants/` | same | Member 4 |
| Platform stand-ins (config, JWT check, web helpers, DB, metrics) | `core-api/internal/platform/` | same | shared / Member 1 (auth) — **replace at integration** |
| Entry points | `core-api/cmd/api`, `cmd/seed`, `cmd/devtoken` | same | — |
| Migrations (stub users/audit + Member 4 tables + view) | `core-api/migrations/` | `core-api/migrations/` (stubs dropped) | Member 4 (stubs: Members 1/7) |
| Parcel booking | `infrastructure/booking_stub.go` | replaced by Member 2 adapter | **stand-in** |
| Admin routes `/api/v1/admin/merchants/**` | `transport/http.go` | Member 7's admin module calls `AdminService` | **stand-in** |
| Kubernetes needs | `deploy/kubernetes/merchants-config-proposal.yaml` | merge into Member 9's manifests | proposal |

## Quick start

Needs: Docker Desktop, Go 1.22+, bash (Git Bash on Windows), curl.

```bash
./scripts/up.sh              # dev keys + .env (random secrets) -> postgres -> migrate -> seed -> api  (~20 s first time)
./scripts/demo.sh            # 49 end-to-end checks (needs a fresh DB: ./scripts/down.sh --wipe && ./scripts/up.sh)
./scripts/shutdown-demo.sh   # SIGTERM -> readyz 503 DRAINING -> exit 0 -> restart
./scripts/down.sh            # stop  (--wipe also deletes the database)
```

| URL | What |
| --- | --- |
| http://localhost:8084/livez, /readyz, /metrics | Health and metrics |
| http://localhost:8084/api/v1/... | Merchants API (see `../contracts/merchants-openapi.v1.yaml`) |
| `postgres://swiftdrop:swiftdrop_dev@localhost:55432/swiftdrop_db` | Database (local only) |

Make a token (stand-in for Member 1 login):

```bash
./.dev/devtoken -key ./.dev/jwt_private.pem -role ADMIN -sub 00000000-0000-4000-8000-0000000000ad
./.dev/devtoken -key ./.dev/jwt_private.pem -role MERCHANT -sub 00000000-0000-4000-8000-0000000000a1 -tenant <merchant_id>
```

## Implemented vs. not in this slice

| Implemented (demo-verified) | Not in this slice (specified in Docs 2/4) |
| --- | --- |
| Apply, view own merchant, admin list/get/approve/reject/suspend/reinstate/disable | `PATCH /merchants/me` (profile update) |
| Create / list / revoke API keys; API-key auth middleware | Key expiry UI flow (supported in code, not demoed) |
| Register / list / delete webhooks with SSRF policy + DNS check | `PATCH` webhook, `rotate-secret`, `GET` single webhook |
| Merchant orders via API key or JWT (Member 2 **stand-in**, in-memory) | Real parcel creation (Member 2), `GET /merchant/orders/{id}` |
| Audit rows, metrics, structured logs, graceful shutdown | Reconciliation / retention jobs (Doc 5 §8, Doc 7 §10.6) |

## Saved results (24 Oct 2026)

| File | Result |
| --- | --- |
| `evidence/demo-output.txt` | **49 / 49 passed** on a fresh stack |
| `evidence/shutdown-output.txt` | readyz 503 during drain, `Exited (0)`, restart READY |
| `evidence/config-failure-output.txt` | missing/invalid secrets → refuses to start, lists 4 problems, no values printed |
| `evidence/unit-test-output.txt` | `go vet` clean; unit tests pass with `-race` |
