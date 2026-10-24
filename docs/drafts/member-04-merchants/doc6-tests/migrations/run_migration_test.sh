#!/usr/bin/env bash
# Member 4 migration test: up -> assert -> down -> verify clean -> up -> assert.
# Usage: ./run_migration_test.sh [container-name]
# Needs a running PostgreSQL 16 container, e.g.:
#   docker run -d --name m4-migr-test -e POSTGRES_PASSWORD=test -e POSTGRES_DB=swiftdrop_test postgres:16-alpine
set -euo pipefail

CONTAINER="${1:-m4-migr-test}"
HERE="$(cd "$(dirname "$0")" && pwd)"
MIG="$HERE/../../migrations-draft"
PSQL=(docker exec -i "$CONTAINER" psql -U postgres -d swiftdrop_test -v ON_ERROR_STOP=1 -q)

run() { echo "  > $(basename "$1")"; "${PSQL[@]}" < "$1"; }

echo "== reset schema"
"${PSQL[@]}" -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;" >/dev/null
run "$HERE/00_prereq_stub.sql"

echo "== up (1st)"
for f in $(ls "$MIG"/*.up.sql | sort); do run "$f"; done

echo "== assertions (1st)"
"${PSQL[@]}" -t < "$HERE/10_assertions.sql" 2>&1 | sed -e 's/^NOTICE:  /  /' -e '/^[[:space:]]*$/d'

echo "== down"
for f in $(ls "$MIG"/*.down.sql | sort -r); do run "$f"; done

echo "== verify nothing left behind"
LEFT=$("${PSQL[@]}" -tAc "SELECT string_agg(relname, ',') FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname='public' AND relname IN ('merchants','merchant_api_credentials','webhook_endpoints','merchant_webhook_targets')")
if [ -n "$LEFT" ]; then echo "FAIL objects remain after down: $LEFT"; exit 1; fi
echo "  PASS  down removed all Member 4 objects"

echo "== up (2nd, proves down is clean and up is repeatable)"
for f in $(ls "$MIG"/*.up.sql | sort); do run "$f"; done

echo "== assertions (2nd)"
"${PSQL[@]}" -t < "$HERE/10_assertions.sql" 2>&1 | grep -cE "^NOTICE:  PASS" | sed 's/^/  PASS count: /'
"${PSQL[@]}" -t < "$HERE/10_assertions.sql" 2>&1 | grep -E "ALL ASSERTIONS PASSED|FAIL" | sed 's/^/  /'

echo "== migration test complete"
