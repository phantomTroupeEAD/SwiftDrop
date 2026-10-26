# Document 7 test files (Member 7)

Not in the repo yet. Run on 24 Oct 2026, re-run on 26 Oct 2026 after the docs audit.

## gateway/
Runs the real Traefik 3.1.7 binary with stand-in backends (no Docker needed).

    ./security_gateway_test.sh <static.yml> <dynamic.yml> <path-to-traefik>

- Today's repo config (deploy/gateway/traefik/): 8 passed, 17 failed -> current-output.txt
- Proposed config (traefik.proposed.yml + dynamic-routes.proposed.yml): 25 passed -> proposed-output.txt
- 26 Oct additions: GS-09b signed-in stream path, GS-11b CORS origins and exposed headers exactly as security-notes 4,
  GS-20 AI limit 10/min burst 2, GS-21 merchant API limit 120/min burst 20. GS-08 now uses Member 3's
  upload path /api/v1/deliveries/{id}/proof-photo.

## admin-security/
Go starter code for the admin module: RequireRole guard, safe error envelope,
correlation ID check, system_settings allowlist, reason check, audit redaction.

    go test -race -cover ./...

Result: 17 passed, 100% statement coverage -> go-test-output.txt, coverage.txt
