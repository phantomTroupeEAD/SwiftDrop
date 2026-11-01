#!/usr/bin/env bash
# Walk through the module on the running local stack (start it with scripts/local-up.sh).
# Every step goes through the gateway, like the browser would. Each line is a check.
set -uo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
GW=http://localhost:${GW_PORT:-8088}
PGPORT=${PGPORT:-5432}
PASS=0; FAIL=0
check() { if [ "$3" == "$4" ]; then PASS=$((PASS+1)); printf 'PASS  %-8s %s\n' "$1" "$2"; else FAIL=$((FAIL+1)); printf 'FAIL  %-8s %-60s expected=%s actual=%s\n' "$1" "$2" "$3" "$4"; fi; }
jq_() { python3 -c "import sys,json; d=json.load(sys.stdin); print($1)"; }
(cd "$ROOT/core-api" && go build -o /tmp/m7-local/devtoken ./cmd/devtoken)
tok() { (cd "$ROOT/core-api" && /tmp/m7-local/devtoken -role "$1" -sub "$2"); }
ADMIN=$(tok ADMIN 00000000-0000-4000-8000-000000000001)
COURIER=$(tok COURIER 00000000-0000-4000-8000-000000000003)
api() { curl -s -H "Authorization: Bearer $1" -H 'Content-Type: application/json' "${@:2}"; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

if api $ADMIN $GW/api/v1/admin/zones | grep -q ZONE_MATARA; then
  echo "The demo needs a fresh database: make wipe && make up"; exit 2
fi

echo "== Health (direct to the pod, never through the gateway) =="
check DEMO-01 "GET :8080/livez" '{"status":"UP"}' "$(curl -s localhost:8080/livez)"
check DEMO-02 "GET :8080/readyz shows the database" '{"status":"READY","database":"UP"}' "$(curl -s localhost:8080/readyz)"
check DEMO-03 "/readyz is not public through the gateway" no "$(curl -s $GW/readyz | grep -q READY && echo yes || echo no)"

echo "== Access control =="
check DEMO-04 "no token: 401" 401 "$(code $GW/api/v1/admin/zones)"
check DEMO-05 "courier token: 403" 403 "$(code -H "Authorization: Bearer $COURIER" $GW/api/v1/admin/zones)"
check DEMO-06 "tampered token: 401" 401 "$(code -H "Authorization: Bearer ${ADMIN%?}x" $GW/api/v1/admin/zones)"
check DEMO-07 "admin token: 7 seeded zones" 7 "$(api $ADMIN $GW/api/v1/admin/zones | jq_ 'len(d["data"])')"

echo "== Zone rules =="
NEW=$(api $ADMIN -X POST $GW/api/v1/admin/zones -d '{"code":"ZONE_MATARA","name":"Matara","base_surcharge_cents":17500,"postal_prefixes":["81000"]}')
ZID=$(echo "$NEW" | jq_ 'd["data"]["id"]')
check DEMO-08 "create ZONE_MATARA: version 1, LKR cents" "1 17500" "$(echo "$NEW" | jq_ 'd["data"]["version"], d["data"]["base_surcharge_cents"]' | tr -d '(),')"
check DEMO-09 "same code again: 409 ZONE_CODE_ALREADY_EXISTS" ZONE_CODE_ALREADY_EXISTS "$(api $ADMIN -X POST $GW/api/v1/admin/zones -d '{"code":"ZONE_MATARA","name":"X","base_surcharge_cents":0,"postal_prefixes":["81100"]}' | jq_ 'd["error"]["code"]')"
check DEMO-10 "prefix 003 overlaps Colombo 00300: 409" ZONE_POSTAL_CODE_CONFLICT "$(api $ADMIN -X POST $GW/api/v1/admin/zones -d '{"code":"ZONE_TEST_OVERLAP","name":"T","base_surcharge_cents":0,"postal_prefixes":["003"]}' | jq_ 'd["error"]["code"]')"
check DEMO-11 "surcharge as a decimal string: 400" 400 "$(code -H "Authorization: Bearer $ADMIN" -X POST $GW/api/v1/admin/zones -d '{"code":"ZONE_BAD","name":"B","base_surcharge_cents":"3.50","postal_prefixes":["70000"]}')"
check DEMO-12 "unknown field (created_by) refused: 400" 400 "$(code -H "Authorization: Bearer $ADMIN" -X PATCH $GW/api/v1/admin/zones/$ZID -d '{"version":1,"created_by":"x"}')"
check DEMO-13 "surcharge change without reason: 422" REASON_REQUIRED "$(api $ADMIN -X PATCH $GW/api/v1/admin/zones/$ZID -d '{"version":1,"base_surcharge_cents":20000}' | jq_ 'd["error"]["code"]')"
check DEMO-14 "surcharge change with reason: version 2" 2 "$(api $ADMIN -X PATCH $GW/api/v1/admin/zones/$ZID -d '{"version":1,"base_surcharge_cents":20000,"reason":"Fuel cost increase approved by ops"}' | jq_ 'd["data"]["version"]')"
check DEMO-15 "stale version 1 after that: 409" CONCURRENT_MODIFICATION "$(api $ADMIN -X PATCH $GW/api/v1/admin/zones/$ZID -d '{"version":1,"name":"Matara Town"}' | jq_ 'd["error"]["code"]')"
check DEMO-16 "disable zone with reason: enabled false" False "$(api $ADMIN -X PATCH $GW/api/v1/admin/zones/$ZID/status -d '{"version":2,"enabled":false,"reason":"Courier shortage this week"}' | jq_ 'd["data"]["enabled"]')"
check DEMO-17 "setting not on allowlist: 422" SETTING_UNKNOWN "$(api $ADMIN -X PUT $GW/api/v1/admin/settings/jwt.private_key -d '{"version":1,"value":"x","reason":"trying to store a secret"}' | jq_ 'd["error"]["code"]')"
check DEMO-18 "board refresh setting to 45 s" 45 "$(api $ADMIN -X PUT $GW/api/v1/admin/settings/ops.board_refresh_seconds -d '{"version":1,"value":45,"reason":"Reduce load during sale week"}' | jq_ 'd["data"]["value"]')"

echo "== Audit trail =="
AUD=$(api $ADMIN "$GW/api/v1/admin/audit-logs?limit=10")
check DEMO-19 "4 audit rows, newest first" "SETTING_CHANGED ZONE_DISABLED ZONE_UPDATED ZONE_CREATED" "$(echo "$AUD" | jq_ '" ".join(r["action"] for r in d["data"])')"
check DEMO-20 "audit row has reason and request id" "yes" "$(echo "$AUD" | jq_ '"yes" if d["data"][1]["reason"] and d["data"][1]["request_id"] else "no"')"
check DEMO-21 "app role cannot UPDATE audit_logs" "permission denied" "$(PGPASSWORD=app_dev_only psql -h localhost -p $PGPORT -U swiftdrop_app -d swiftdrop_db -c "UPDATE audit_logs SET reason='x'" 2>&1 | grep -o 'permission denied')"
check DEMO-22 "even the owner cannot DELETE (trigger)" "append-only" "$(PGPASSWORD=owner_dev_only psql -h localhost -p $PGPORT -U swiftdrop_owner -d swiftdrop_db -c "DELETE FROM audit_logs" 2>&1 | grep -o 'append-only')"

echo "== Gateway behaviour =="
check DEMO-23 "/api/v2/zones: JSON 404 from core-api" NOT_FOUND "$(curl -s $GW/api/v2/zones | jq_ 'd["error"]["code"]')"
check DEMO-24 "/ops/zones served by web (SPA stand-in)" web "$(curl -s $GW/ops/zones | jq_ 'd["service"]')"
check DEMO-25 "AI service not running: 502, rest fine" "502 200" "$(code $GW/api/v1/ai/chat) $(code -H "Authorization: Bearer $ADMIN" $GW/api/v1/admin/zones)"
check DEMO-26 "X-Request-ID sent by client comes back" req_demo_42 "$(curl -s -D - -o /dev/null -H 'X-Request-ID: req_demo_42' $GW/api/v1/admin/zones | tr -d '\r' | sed -n 's/^x-request-id: //Ip')"

echo "== Observability =="
M=$(curl -s localhost:8080/metrics)
check DEMO-27 "metrics count admin changes" yes "$(echo "$M" | grep -q 'admin_changes_total{action="ZONE_CREATED"} 1' && echo yes || echo no)"
check DEMO-28 "metrics count conflicts by kind" yes "$(echo "$M" | grep -q 'admin_conflicts_total{kind="prefix_overlap"} 1' && echo yes || echo no)"
check DEMO-29 "access log line is JSON with route and request id" yes "$(grep req_demo_42 /tmp/m7-local/logs/core-api.log | tail -1 | python3 -c 'import sys,json; d=json.loads(sys.stdin.read()); print("yes" if d["route"]=="GET /api/v1/admin/zones" and d["status"]==401 else "no")')"
check DEMO-30 "no token or password in logs" 0 "$(grep -c -E 'eyJ|app_dev_only|owner_dev_only' /tmp/m7-local/logs/core-api.log)"

echo
echo "Summary: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
