# Document 7 test files (Member 7)

Not in the repo yet. Run on 24 Oct 2026.

## gateway/
Runs the real Traefik 3.1.7 binary with stand-in backends (no Docker needed).

    ./security_gateway_test.sh <static.yml> <dynamic.yml> <path-to-traefik>

- Today's repo config (deploy/gateway/traefik/): 8 passed, 13 failed -> current-output.txt
- Proposed config (traefik.proposed.yml + dynamic-routes.proposed.yml): 21 passed -> proposed-output.txt

## admin-security/
Go starter code for the admin module: RequireRole guard, safe error envelope,
correlation ID check, system_settings allowlist, reason check, audit redaction.

    go test -race -cover ./...

Result: 17 passed, 100% statement coverage -> go-test-output.txt, coverage.txt
