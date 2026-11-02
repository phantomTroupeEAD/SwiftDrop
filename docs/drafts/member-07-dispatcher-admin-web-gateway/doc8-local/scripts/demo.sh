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
DISPATCHER=$(tok DISPATCHER 00000000-0000-4000-8000-000000000002)
COURIER=$(tok COURIER 00000000-0000-4000-8000-000000000003)
api() { curl -s -H "Authorization: Bearer $1" -H 'Content-Type: application/json' "${@:2}"; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
PENDING=11111111-1111-4111-8111-111111111111
APPROVED=22222222-2222-4222-8222-222222222222

if api $ADMIN $GW/api/v1/admin/zones | grep -q ZONE_MATARA_01; then
  echo "The demo needs a fresh database: make wipe && make up"; exit 2
fi

echo "== Health (direct to the pod, never through the gateway; Member 9 A2) =="
check DEMO-01 "GET :8080/health/live" '{"status":"UP"}' "$(curl -s localhost:8080/health/live)"
check DEMO-02 "GET :8080/health/ready shows the database" '{"status":"READY","database":"UP"}' "$(curl -s localhost:8080/health/ready)"
check DEMO-03 "/health/ready is not public through the gateway" no "$(curl -s $GW/health/ready | grep -q READY && echo yes || echo no)"

echo "== Access control =="
check DEMO-04 "no token: 401" 401 "$(code $GW/api/v1/admin/zones)"
check DEMO-05 "courier token: 403" 403 "$(code -H "Authorization: Bearer $COURIER" $GW/api/v1/admin/zones)"
# Change the FIRST signature character (all its bits count; the last one may only hold padding bits).
BAD=$(python3 -c "import sys; h,p,s=sys.argv[1].split('.'); print(h+'.'+p+'.'+('B' if s[0]=='A' else 'A')+s[1:])" "$ADMIN")
check DEMO-06 "tampered token: 401" 401 "$(code -H "Authorization: Bearer $BAD" $GW/api/v1/admin/zones)"
check DEMO-07 "admin token: 7 seeded zones" 7 "$(api $ADMIN $GW/api/v1/admin/zones | jq_ 'len(d["data"])')"

echo "== Public zone list for booking forms (no sign-in, Doc 1 R-18) =="
PUB=$(curl -s $GW/api/v1/zones)
check DEMO-08 "6 enabled zones, only id, code, name" "6 code,id,name" "$(echo "$PUB" | jq_ 'str(len(d["data"]))+" "+",".join(sorted(d["data"][0].keys()))')"
check DEMO-09 "postal code 00300 is in ZONE_COLOMBO_01" ZONE_COLOMBO_01 "$(curl -s "$GW/api/v1/zones?postal_code=00300" | jq_ 'd["data"]["id"]')"
check DEMO-10 "postal code in a disabled zone: 404" ZONE_NOT_FOUND "$(curl -s "$GW/api/v1/zones?postal_code=99999" | jq_ 'd["error"]["code"]')"
check DEMO-11 "bad postal code: 422" 422 "$(code "$GW/api/v1/zones?postal_code=abc")"
check DEMO-12 "cacheable for 5 minutes" "public, max-age=300" "$(curl -s -D - -o /dev/null $GW/api/v1/zones | tr -d '\r' | sed -n 's/^cache-control: //Ip')"

echo "== Zone rules (text id, no money) =="
NEW=$(api $ADMIN -X POST $GW/api/v1/admin/zones -d '{"id":"ZONE_MATARA_01","code":"MTR-01","name":"Matara","postal_prefixes":["81000"]}')
ZID=$(echo "$NEW" | jq_ 'd["data"]["id"]')
check DEMO-13 "create ZONE_MATARA_01: version 1, no money fields" "ZONE_MATARA_01 1 no" "$(echo "$NEW" | jq_ 'd["data"]["id"]+" "+str(d["data"]["version"])+" "+("yes" if any("cents" in k or "surcharge" in k for k in d["data"]) else "no")')"
check DEMO-14 "same id again: 409" ZONE_CODE_ALREADY_EXISTS "$(api $ADMIN -X POST $GW/api/v1/admin/zones -d '{"id":"ZONE_MATARA_01","code":"MTR-02","name":"X","postal_prefixes":["81100"]}' | jq_ 'd["error"]["code"]')"
check DEMO-15 "same short code, other id: 409" ZONE_CODE_ALREADY_EXISTS "$(api $ADMIN -X POST $GW/api/v1/admin/zones -d '{"id":"ZONE_MATARA_02","code":"MTR-01","name":"X","postal_prefixes":["81100"]}' | jq_ 'd["error"]["code"]')"
check DEMO-16 "prefix 003 overlaps Colombo 00300: 409" ZONE_POSTAL_CODE_CONFLICT "$(api $ADMIN -X POST $GW/api/v1/admin/zones -d '{"id":"ZONE_TEST_OVERLAP","code":"T-OVL","name":"T","postal_prefixes":["003"]}' | jq_ 'd["error"]["code"]')"
check DEMO-17 "lower-case id refused: 422" VALIDATION_FAILED "$(api $ADMIN -X POST $GW/api/v1/admin/zones -d '{"id":"zone_bad","code":"BAD-1","name":"B","postal_prefixes":["70000"]}' | jq_ 'd["error"]["code"]')"
check DEMO-18 "a surcharge field is refused (fees are pricing rules): 400" 400 "$(code -H "Authorization: Bearer $ADMIN" -X POST $GW/api/v1/admin/zones -d '{"id":"ZONE_BAD_01","code":"BAD-1","name":"B","base_surcharge_cents":5000,"postal_prefixes":["70000"]}')"
check DEMO-19 "unknown field (created_by) refused: 400" 400 "$(code -H "Authorization: Bearer $ADMIN" -X PATCH $GW/api/v1/admin/zones/$ZID -d '{"version":1,"created_by":"x"}')"
check DEMO-20 "rename and new short code: version 2" "2 MTR-1A" "$(api $ADMIN -X PATCH $GW/api/v1/admin/zones/$ZID -d '{"version":1,"name":"Matara Town","code":"MTR-1A"}' | jq_ 'str(d["data"]["version"])+" "+d["data"]["code"]')"
check DEMO-21 "stale version 1 after that: 409" CONCURRENT_MODIFICATION "$(api $ADMIN -X PATCH $GW/api/v1/admin/zones/$ZID -d '{"version":1,"name":"Matara City"}' | jq_ 'd["error"]["code"]')"
check DEMO-22 "disable without reason: 422" REASON_REQUIRED "$(api $ADMIN -X PATCH $GW/api/v1/admin/zones/$ZID/status -d '{"version":2,"enabled":false}' | jq_ 'd["error"]["code"]')"
check DEMO-23 "disable zone with reason: enabled false" False "$(api $ADMIN -X PATCH $GW/api/v1/admin/zones/$ZID/status -d '{"version":2,"enabled":false,"reason":"Courier shortage this week"}' | jq_ 'd["data"]["enabled"]')"
check DEMO-24 "setting not on allowlist: 422" SETTING_UNKNOWN "$(api $ADMIN -X PUT $GW/api/v1/admin/settings/jwt.private_key -d '{"version":1,"value":"x","reason":"trying to store a secret"}' | jq_ 'd["error"]["code"]')"
check DEMO-25 "board refresh setting to 45 s" 45 "$(api $ADMIN -X PUT $GW/api/v1/admin/settings/ops.board_refresh_seconds -d '{"version":1,"value":45,"reason":"Reduce load during sale week"}' | jq_ 'd["data"]["value"]')"

echo "== Merchant admin (my routes, Member 4's rules; local stand-in for Member 4) =="
check DEMO-26 "dispatcher cannot list merchants: 403" 403 "$(code -H "Authorization: Bearer $DISPATCHER" $GW/api/v1/admin/merchants)"
check DEMO-27 "admin list: 2 merchants, page meta" "2 2" "$(api $ADMIN "$GW/api/v1/admin/merchants?page=1&per_page=20" | jq_ 'str(len(d["data"]))+" "+str(d["meta"]["total"])')"
check DEMO-28 "status filter PENDING: Ceylon Tea Shop" "Ceylon Tea Shop" "$(api $ADMIN "$GW/api/v1/admin/merchants?status=PENDING" | jq_ 'd["data"][0]["business_name"]')"
check DEMO-29 "approve with expected_version 1: APPROVED v2" "APPROVED 2" "$(api $ADMIN -X POST $GW/api/v1/admin/merchants/$PENDING/approve -d '{"expected_version":1}' | jq_ 'd["data"]["status"]+" "+str(d["data"]["version"])')"
check DEMO-30 "same old version again: 409 (Member 4 code)" MERCHANT_VERSION_CONFLICT "$(api $ADMIN -X POST $GW/api/v1/admin/merchants/$PENDING/suspend -d '{"expected_version":1,"reason":"Late payments"}' | jq_ 'd["error"]["code"]')"
check DEMO-31 "suspend without reason: 422" REASON_REQUIRED "$(api $ADMIN -X POST $GW/api/v1/admin/merchants/$APPROVED/suspend -d '{"expected_version":1}' | jq_ 'd["error"]["code"]')"
check DEMO-32 "suspend, then reinstate" "SUSPENDED APPROVED" "$(api $ADMIN -X POST $GW/api/v1/admin/merchants/$APPROVED/suspend -d '{"expected_version":1,"reason":"Chargeback review"}' | jq_ 'd["data"]["status"]') $(api $ADMIN -X POST $GW/api/v1/admin/merchants/$APPROVED/reinstate -d '{"expected_version":2}' | jq_ 'd["data"]["status"]')"
check DEMO-33 "reject an APPROVED merchant: 409" MERCHANT_INVALID_TRANSITION "$(api $ADMIN -X POST $GW/api/v1/admin/merchants/$APPROVED/reject -d '{"expected_version":3,"reason":"Wrong documents"}' | jq_ 'd["error"]["code"]')"
check DEMO-34 "unknown action: 404" 404 "$(code -H "Authorization: Bearer $ADMIN" -X POST $GW/api/v1/admin/merchants/$APPROVED/archive -d '{"expected_version":3}')"

echo "== Delivery report (ADMIN and DISPATCHER, Doc 1 R-16) =="
check DEMO-35 "dispatcher may read the report" 200 "$(code -H "Authorization: Bearer $DISPATCHER" "$GW/api/v1/admin/reports/deliveries?from=2026-10-01&to=2026-10-07")"
check DEMO-36 "report says its data is a stand-in" yes "$(api $DISPATCHER "$GW/api/v1/admin/reports/deliveries?from=2026-10-01&to=2026-10-07" | jq_ '"yes" if d["data"]["source"].startswith("stand-in") else "no"')"
check DEMO-37 "courier may not: 403" 403 "$(code -H "Authorization: Bearer $COURIER" "$GW/api/v1/admin/reports/deliveries?from=2026-10-01&to=2026-10-07")"
check DEMO-38 "range over reports.max_range_days (92): 422" 422 "$(code -H "Authorization: Bearer $ADMIN" "$GW/api/v1/admin/reports/deliveries?from=2026-01-01&to=2026-10-01")"

echo "== Audit trail =="
AUD=$(api $ADMIN "$GW/api/v1/admin/audit-logs?limit=10")
check DEMO-39 "4 audit rows, newest first" "SETTING_CHANGED ZONE_DISABLED ZONE_UPDATED ZONE_CREATED" "$(echo "$AUD" | jq_ '" ".join(r["action"] for r in d["data"])')"
check DEMO-40 "audit row has reason and request id" "yes" "$(echo "$AUD" | jq_ '"yes" if d["data"][1]["reason"] and d["data"][1]["request_id"] else "no"')"
check DEMO-41 "app role cannot UPDATE audit_logs" "permission denied" "$(PGPASSWORD=app_dev_only psql -h localhost -p $PGPORT -U swiftdrop_app -d swiftdrop_db -c "UPDATE audit_logs SET reason='x'" 2>&1 | grep -o 'permission denied')"
check DEMO-42 "even the migrator cannot DELETE (trigger)" "append-only" "$(PGPASSWORD=migrator_dev_only psql -h localhost -p $PGPORT -U swiftdrop_migrator -d swiftdrop_db -c "DELETE FROM audit_logs" 2>&1 | grep -o 'append-only')"
check DEMO-43 "app role cannot DELETE a zone (D7)" "permission denied" "$(PGPASSWORD=app_dev_only psql -h localhost -p $PGPORT -U swiftdrop_app -d swiftdrop_db -c "DELETE FROM delivery_zones WHERE id='ZONE_MATARA_01'" 2>&1 | grep -o 'permission denied')"

echo "== Gateway behaviour =="
check DEMO-44 "/api/v2/zones: JSON 404 from core-api" NOT_FOUND "$(curl -s $GW/api/v2/zones | jq_ 'd["error"]["code"]')"
check DEMO-45 "/ops/zones served by web (SPA stand-in)" web "$(curl -s $GW/ops/zones | jq_ 'd["service"]')"
check DEMO-46 "AI service not running: 502, rest fine" "502 200" "$(code $GW/api/v1/ai/chat) $(code -H "Authorization: Bearer $ADMIN" $GW/api/v1/admin/zones)"
check DEMO-47 "X-Request-ID sent by client comes back" req_demo_42 "$(curl -s -D - -o /dev/null -H 'X-Request-ID: req_demo_42' $GW/api/v1/admin/zones | tr -d '\r' | sed -n 's/^x-request-id: //Ip')"

echo "== Observability =="
M=$(curl -s localhost:8080/metrics)
check DEMO-48 "metrics count admin changes" yes "$(echo "$M" | grep -q 'admin_changes_total{action="ZONE_CREATED"} 1' && echo "$M" | grep -q 'admin_changes_total{action="MERCHANT_APPROVE"} 1' && echo yes || echo no)"
check DEMO-49 "metrics count conflicts by kind" yes "$(echo "$M" | grep -q 'admin_conflicts_total{kind="prefix_overlap"} 1' && echo "$M" | grep -q 'admin_conflicts_total{kind="merchant_version"} 1' && echo yes || echo no)"
check DEMO-50 "access log line is JSON with route and request id" yes "$(grep req_demo_42 /tmp/m7-local/logs/core-api.log | tail -1 | python3 -c 'import sys,json; d=json.loads(sys.stdin.read()); print("yes" if d["route"]=="GET /api/v1/admin/zones" and d["status"]==401 else "no")')"
check DEMO-51 "no token or password in logs" 0 "$(grep -c -E 'eyJ|app_dev_only|migrator_dev_only' /tmp/m7-local/logs/core-api.log)"

echo
echo "Summary: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
