# Member 4 starter automated tests (Document 6)

Written and run on 24 Oct 2026 for Document 6. They are **not yet in the team repo** — move them when the real module is created.

| Folder | What it tests | Move to | Run |
| --- | --- | --- | --- |
| `merchants-domain/domain/` | Merchant status machine (all 25 status×action pairs), reasons, business name, API key format + redaction, scopes, credential status/revoke/expiry, webhook URL parsing, SSRF host + resolved-address policy, fuzz test | `core-api/internal/merchants/domain/` | `go test -race -count=1 -cover ./...` |
| `merchants-domain/keycrypto/` | Base62 key generation (uniform, unique), HMAC-SHA256 hashing, constant-time compare | `core-api/internal/merchants/infrastructure/` | same |
| `migrations/` | Draft migrations: up → 41 constraint/view/index/grant assertions → down → clean check → up → assertions again | `core-api/tests/migrations/` (or CI job) | `./run_migration_test.sh m4-migr-test` |

## Prerequisites

- Go 1.22+ (`-race` needs CGO + a 64-bit C compiler; on Windows without one, run in Docker — see below).
- Docker, for the migration test (PostgreSQL 16) and for race tests on Windows.

```bash
# Domain tests (Linux/macOS, or Windows with 64-bit gcc)
cd merchants-domain && go test -race -count=1 -cover ./...

# Same, in Docker (used for the saved evidence)
docker run --rm -v "$PWD/merchants-domain:/src" -w /src golang:1.22-bookworm go test -race -count=1 -cover ./...

# Fuzz the URL parser for 30 s
cd merchants-domain && go test -run '^$' -fuzz FuzzParseWebhookURL -fuzztime 30s ./domain

# Migration test
docker run -d --name m4-migr-test -e POSTGRES_PASSWORD=test -e POSTGRES_DB=swiftdrop_test postgres:16-alpine
./migrations/run_migration_test.sh m4-migr-test
docker rm -f m4-migr-test
```

## Saved results

| File | Result |
| --- | --- |
| `merchants-domain/go-test-output.txt` | 119 passed, 0 failed, `-race`, total coverage **96.2 %** (domain 95.7 %, keycrypto 100 %) |
| `migrations/migration-test-output.txt` | 41/41 assertions passed twice; down removed all objects |

Re-run on 24 Oct 2026 after aligning with ADR-003 (key format `sd_live_<8>_<32>`, scopes `parcels:create`/`parcels:read`).
| Fuzz (not saved, console) | 30 s, ~6.5 M executions, no crash |
