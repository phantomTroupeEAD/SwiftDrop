# Member 7 starter automated tests (Document 6)

These tests were written and run on 24 Oct 2026 for Document 6. The admin-domain suite was re-run on 26 Oct 2026 after the docs audit (text zone id `ZONE_COLOMBO_01`, short code `CMB-01`, no money in zones); it is the same code as `doc8-local/core-api/internal/admin/domain/`. They are **not yet in the team repo**.
Move them into the repo paths below when the real code is created.

| Folder | What it tests | Move to | Run |
| --- | --- | --- | --- |
| `admin-domain/` | Zone aggregate, value objects, PrefixOverlapPolicy (reference code + tests) | `core-api/internal/admin/domain/` | `go test -race -count=1 -cover ./...` |
| `web-client/` | Shared API client: 401 refresh once, single-flight refresh, Idempotency-Key rules, error mapping, courier helpers | `web/src/shared/api/` and `web/src/features/dispatcher-admin/shared-ops/` | `npm ci && npx tsc --noEmit && npx vitest run --coverage` |
| `gateway/` | Real Traefik binary with the repo's `traefik.yml` + `dynamic-routes.yml` and stub backends | `deploy/tests/gateway_test.sh` (path named in docs/testing/test-strategy.md) | `./gateway_test.sh <repo>/deploy/gateway/traefik <traefik-binary>` |

Get Traefik v3.1.7 for the gateway test:
`curl -sSfL https://github.com/traefik/traefik/releases/download/v3.1.7/traefik_v3.1.7_linux_amd64.tar.gz | tar xz traefik`

Saved results: `admin-domain/go-test-output.txt`, `admin-domain/coverage.txt`, `web-client/vitest-output.txt`, `web-client/tsc-output.txt` (empty = no errors), `gateway/gateway-test-output.txt`.
