#!/usr/bin/env bash
# End-to-end demo of the merchants slice over real HTTP + PostgreSQL.
# Needs a FRESH stack:  ./scripts/down.sh --wipe && ./scripts/up.sh && ./scripts/demo.sh
set -uo pipefail
cd "$(dirname "$0")/.."
BASE=${BASE:-http://localhost:8084}
DT=./.dev/devtoken; [ -f ./.dev/devtoken.exe ] && DT=./.dev/devtoken.exe
ADMIN=00000000-0000-4000-8000-0000000000ad
OWNER_A=00000000-0000-4000-8000-0000000000a1
OWNER_B=00000000-0000-4000-8000-0000000000b1
PASS=0; FAIL=0; N=0
HDR=$(mktemp)

tok() { "$DT" -key ./.dev/jwt_private.pem "$@"; }
# call EXPECTED METHOD PATH [curl args...]  -> sets CODE, BODY
call() {
  local exp=$1 m=$2 p=$3; shift 3
  local out; out=$(curl -s -D "$HDR" -w $'\n%{http_code}' -X "$m" "$BASE$p" "$@")
  CODE=${out##*$'\n'}; BODY=${out%$'\n'*}; EXPECT=$exp
}
check() { # check "description" [extra-condition-result]
  N=$((N+1)); local extra=${2:-0}
  if [ "$CODE" = "$EXPECT" ] && [ "$extra" = 0 ]; then PASS=$((PASS+1)); printf "PASS %2d  %-70s %s\n" "$N" "$1" "$CODE"
  else FAIL=$((FAIL+1)); printf "FAIL %2d  %-70s got %s want %s\n         body: %.300s\n" "$N" "$1" "$CODE" "$EXPECT" "$BODY"; fi
}
has() { grep -q -- "$1" <<<"$BODY"; echo $?; }
hdr() { grep -qi -- "$1" "$HDR"; echo $?; }
jget() { sed -n "s/.*\"$1\":\"\([^\"]*\)\".*/\1/p" <<<"$BODY" | head -1; }
jnum() { sed -n "s/.*\"$1\":\([0-9]*\).*/\1/p" <<<"$BODY" | head -1; }
J=(-H "Content-Type: application/json")
APP_A='{"business_name":"Ceylon Tea Shop","contact_email":"ops@ceylontea.lk","contact_phone":"+94771234567","address":{"line1":"12 Temple Rd","city":"Kandy","country":"LK"}}'
APP_B='{"business_name":"Spice Shop","contact_email":"hi@spice.lk","contact_phone":"+94771112222","address":{"line1":"1 Galle Rd","city":"Colombo","country":"LK"}}'
ORDER='{"merchant_reference":"CTS-1001","pickup":{"city":"Kandy"},"dropoff":{"city":"Colombo"},"parcel":{"weight_kg":1.2},"merchant_id":"ffffffff-ffff-4fff-8fff-ffffffffffff"}'

T_ADMIN=$(tok -role ADMIN -sub $ADMIN)
T_A_APPLICANT=$(tok -role CUSTOMER -sub $OWNER_A)

echo "== Health and metrics"
call 200 GET /livez; check "livez"
call 200 GET /readyz; check "readyz (database UP)" "$(has '"database":"UP"')"

echo "== Application and approval (F-01, F-02)"
call 401 POST /api/v1/merchants/applications "${J[@]}" -d "$APP_A"; check "apply without token -> 401"
call 201 POST /api/v1/merchants/applications "${J[@]}" -H "Authorization: Bearer $T_A_APPLICANT" -d "$APP_A"
MID_A=$(jget merchant_id); check "owner A applies -> 201 PENDING" "$(has '"status":"PENDING"')"
call 409 POST /api/v1/merchants/applications "${J[@]}" -H "Authorization: Bearer $T_A_APPLICANT" -d "$APP_A"; check "second active application -> 409 MERCHANT_ALREADY_EXISTS" "$(has MERCHANT_ALREADY_EXISTS)"
call 422 POST /api/v1/merchants/applications "${J[@]}" -H "Authorization: Bearer $(tok -role CUSTOMER -sub $OWNER_B)" -d "${APP_B/+94771112222/0771112222}"; check "local phone format -> 422 VALIDATION_FAILED (contact_phone)" "$(has contact_phone)"
call 200 GET /api/v1/merchants/me -H "Authorization: Bearer $T_A_APPLICANT"; check "applicant sees own PENDING merchant (no tenant_id yet)" "$(has '"status":"PENDING"')"
T_A=$(tok -role MERCHANT -sub $OWNER_A -tenant "$MID_A")
call 403 POST /api/v1/merchants/me/api-keys "${J[@]}" -H "Authorization: Bearer $T_A" -d '{"label":"prod"}'; check "create key while PENDING -> 403 MERCHANT_NOT_APPROVED" "$(has MERCHANT_NOT_APPROVED)"
call 403 POST /api/v1/admin/merchants/$MID_A/approve "${J[@]}" -H "Authorization: Bearer $T_A" -d '{"expected_version":1}'; check "non-admin approve -> 403"
call 409 POST /api/v1/admin/merchants/$MID_A/suspend "${J[@]}" -H "Authorization: Bearer $T_ADMIN" -d '{"expected_version":1,"reason":"testing"}'; check "suspend a PENDING merchant -> 409 MERCHANT_INVALID_TRANSITION" "$(has MERCHANT_INVALID_TRANSITION)"
call 409 POST /api/v1/admin/merchants/$MID_A/approve "${J[@]}" -H "Authorization: Bearer $T_ADMIN" -d '{"expected_version":7}'; check "approve with stale version -> 409 MERCHANT_VERSION_CONFLICT" "$(has MERCHANT_VERSION_CONFLICT)"
call 200 POST /api/v1/admin/merchants/$MID_A/approve "${J[@]}" -H "Authorization: Bearer $T_ADMIN" -d '{"expected_version":1}'; check "admin approves -> 200 APPROVED version 2" "$(has '"version":2')"

echo "== API keys (F-04, F-06)"
call 201 POST /api/v1/merchants/me/api-keys "${J[@]}" -H "Authorization: Bearer $T_A" -d '{"label":"prod-server"}'
KEY=$(jget api_key); KEY_ID=$(jget key_id)
check "create key -> 201, ADR-003 format, Cache-Control no-store" "$( [[ $KEY =~ ^sd_test_[0-9A-Za-z]{8}_[0-9A-Za-z]{32}$ ]] && hdr 'cache-control: no-store' || echo 1)"
SECRET_PART=${KEY:17}
call 200 GET /api/v1/merchants/me/api-keys -H "Authorization: Bearer $T_A"; check "list keys shows prefix, never the secret" "$( [ "$(has "$SECRET_PART")" = 1 ] && has '"key_prefix"' || echo 1)"
call 401 GET /api/v1/merchants/me/api-keys -H "X-API-Key: $KEY"; check "API key on key-management route -> 401 API_KEY_NOT_ALLOWED" "$(has API_KEY_NOT_ALLOWED)"
call 401 POST "/api/v1/merchant/orders?api_key=$KEY" "${J[@]}" -d "$ORDER"; check "key in query string ignored -> 401 API_KEY_MISSING" "$(has API_KEY_MISSING)"
call 401 POST /api/v1/merchant/orders "${J[@]}" -H "X-API-Key: ${KEY%?}X" -d "$ORDER"; check "tampered key -> 401 API_KEY_INVALID" "$(has API_KEY_INVALID)"
call 400 POST /api/v1/merchant/orders "${J[@]}" -H "X-API-Key: $KEY" -H "Authorization: Bearer $T_A" -d "$ORDER"; check "both X-API-Key and Authorization -> 400"

echo "== Merchant orders (F-07, stand-in for Member 2)"
IDEM=6f1c2b9e-4c4f-4a8e-9b1d-2f3a4b5c6d7e
call 400 POST /api/v1/merchant/orders "${J[@]}" -H "X-API-Key: $KEY" -d "$ORDER"; check "order without Idempotency-Key -> 400 IDEMPOTENCY_KEY_REQUIRED" "$(has IDEMPOTENCY_KEY_REQUIRED)"
call 201 POST /api/v1/merchant/orders "${J[@]}" -H "X-API-Key: $KEY" -H "Idempotency-Key: $IDEM" -d "$ORDER"
PARCEL=$(jget parcel_id); check "order with key -> 201 tracking number" "$(has '"tracking_number":"SD-')"
call 201 POST /api/v1/merchant/orders "${J[@]}" -H "X-API-Key: $KEY" -H "Idempotency-Key: $IDEM" -d "$ORDER"
check "same key + same body -> same parcel, Idempotent-Replay: true" "$( [ "$(jget parcel_id)" = "$PARCEL" ] && hdr 'idempotent-replay: true' || echo 1)"
call 409 POST /api/v1/merchant/orders "${J[@]}" -H "X-API-Key: $KEY" -H "Idempotency-Key: $IDEM" -d "${ORDER/1.2/2.5}"; check "same key + different body -> 409 IDEMPOTENCY_KEY_PAYLOAD_MISMATCH" "$(has IDEMPOTENCY_KEY_PAYLOAD_MISMATCH)"

echo "== Webhooks and SSRF (F-09)"
call 400 POST /api/v1/merchants/me/webhooks "${J[@]}" -H "Authorization: Bearer $T_A" -d '{"url":"http://169.254.169.254/latest/meta-data","events":["parcel.delivered.v1"]}'; check "cloud metadata URL -> 400 SSRF_PROHIBITED_URL" "$(has SSRF_PROHIBITED_URL)"
call 400 POST /api/v1/merchants/me/webhooks "${J[@]}" -H "Authorization: Bearer $T_A" -d '{"url":"http://localhost/hook","events":["parcel.delivered.v1"]}'; check "localhost URL -> 400 SSRF_PROHIBITED_URL" "$(has SSRF_PROHIBITED_URL)"
call 422 POST /api/v1/merchants/me/webhooks "${J[@]}" -H "Authorization: Bearer $T_A" -d '{"url":"http://postgres:5432/","events":["parcel.delivered.v1"]}'; check "internal service on port 5432 -> 422 WEBHOOK_URL_INVALID (port rule)" "$(has WEBHOOK_URL_INVALID)"
call 400 POST /api/v1/merchants/me/webhooks "${J[@]}" -H "Authorization: Bearer $T_A" -d '{"url":"http://postgres/","events":["parcel.delivered.v1"]}'; check "hostname resolving to private Docker IP -> 400 SSRF (DNS check)" "$(has SSRF_PROHIBITED_URL)"
call 400 POST /api/v1/merchants/me/webhooks "${J[@]}" -H "Authorization: Bearer $T_A" -d '{"url":"http://webhook-sink:8089/hooks","events":["parcel.delivered.v1"],"secret":"mine"}'; check "client-supplied secret -> 400 WEBHOOK_SECRET_NOT_ALLOWED" "$(has WEBHOOK_SECRET_NOT_ALLOWED)"
call 422 POST /api/v1/merchants/me/webhooks "${J[@]}" -H "Authorization: Bearer $T_A" -d '{"url":"http://webhook-sink/hooks","events":["parcel.teleported.v1"]}'; check "unknown event type -> 422" "$(has WEBHOOK_EVENT_UNSUPPORTED)"
call 201 POST /api/v1/merchants/me/webhooks "${J[@]}" -H "Authorization: Bearer $T_A" -d '{"url":"http://webhook-sink/hooks","events":["parcel.booked.v1","parcel.delivered.v1"]}'
WH_A=$(jget webhook_id); WSECRET=$(jget signing_secret)
check "register webhook -> 201, whsec_ secret shown once" "$( [[ $WSECRET =~ ^whsec_ ]] && echo 0 || echo 1)"
call 200 GET /api/v1/merchants/me/webhooks -H "Authorization: Bearer $T_A"; check "list webhooks has secret_last4, never the secret" "$( [ "$(has "$WSECRET")" = 1 ] && has secret_last4 || echo 1)"

echo "== Tenant isolation (AT-011)"
call 201 POST /api/v1/merchants/applications "${J[@]}" -H "Authorization: Bearer $(tok -role CUSTOMER -sub $OWNER_B)" -d "$APP_B"; MID_B=$(jget merchant_id)
call 200 POST /api/v1/admin/merchants/$MID_B/approve "${J[@]}" -H "Authorization: Bearer $T_ADMIN" -d '{"expected_version":1}'
T_B=$(tok -role MERCHANT -sub $OWNER_B -tenant "$MID_B")
call 404 DELETE /api/v1/merchants/me/webhooks/$WH_A -H "Authorization: Bearer $T_B"; check "merchant B deletes A's webhook -> 404" "$(has WEBHOOK_NOT_FOUND)"
call 404 DELETE /api/v1/merchants/me/api-keys/$KEY_ID -H "Authorization: Bearer $T_B"; check "merchant B revokes A's key -> 404" "$(has API_KEY_NOT_FOUND)"
call 200 GET /api/v1/merchant/orders -H "Authorization: Bearer $T_B"; check "merchant B lists orders -> none of A's" "$(has "$PARCEL" | sed 's/1/0/;t;s/0/1/')"

echo "== Suspend, reinstate, revoke, disable"
call 200 POST /api/v1/admin/merchants/$MID_A/suspend "${J[@]}" -H "Authorization: Bearer $T_ADMIN" -d '{"expected_version":2,"reason":"chargeback review"}'; check "admin suspends A (reason)"
call 403 POST /api/v1/merchant/orders "${J[@]}" -H "X-API-Key: $KEY" -H "Idempotency-Key: 11111111-1111-4111-8111-111111111111" -d "$ORDER"; check "suspended merchant's key -> 403 MERCHANT_NOT_APPROVED (next request)" "$(has MERCHANT_NOT_APPROVED)"
call 200 POST /api/v1/admin/merchants/$MID_A/reinstate "${J[@]}" -H "Authorization: Bearer $T_ADMIN" -d '{"expected_version":3}'; check "admin reinstates A"
call 201 POST /api/v1/merchant/orders "${J[@]}" -H "X-API-Key: $KEY" -H "Idempotency-Key: 22222222-2222-4222-8222-222222222222" -d "$ORDER"; check "same key works again after reinstate"
call 204 DELETE /api/v1/merchants/me/api-keys/$KEY_ID -H "Authorization: Bearer $T_A"; check "revoke key -> 204"
call 204 DELETE /api/v1/merchants/me/api-keys/$KEY_ID -H "Authorization: Bearer $T_A"; check "revoke again -> 204 (idempotent)"
call 401 POST /api/v1/merchant/orders "${J[@]}" -H "X-API-Key: $KEY" -H "Idempotency-Key: 33333333-3333-4333-8333-333333333333" -d "$ORDER"; check "revoked key -> 401 API_KEY_REVOKED on next request" "$(has API_KEY_REVOKED)"
call 201 POST /api/v1/merchants/me/api-keys "${J[@]}" -H "Authorization: Bearer $T_A" -d '{"label":"second"}'; KEY2_ID=$(jget key_id)
call 200 POST /api/v1/admin/merchants/$MID_A/disable "${J[@]}" -H "Authorization: Bearer $T_ADMIN" -d '{"expected_version":4,"reason":"fraud confirmed"}'; check "admin disables A"
call 200 GET /api/v1/merchants/me/api-keys -H "Authorization: Bearer $T_A"; check "disable revoked every key (no ACTIVE left)" "$(has '"status":"ACTIVE"' | sed 's/1/0/;t;s/0/1/')"
call 409 POST /api/v1/admin/merchants/$MID_A/reinstate "${J[@]}" -H "Authorization: Bearer $T_ADMIN" -d '{"expected_version":5}'; check "DISABLED is terminal -> 409" "$(has MERCHANT_INVALID_TRANSITION)"

echo "== Evidence in the database and logs"
PSQL=(docker compose exec -T postgres psql -U swiftdrop -d swiftdrop_db -tAc)
AUD=$("${PSQL[@]}" "SELECT string_agg(action, ',' ORDER BY created_at) FROM audit_logs WHERE target_id='$MID_A'")
N=$((N+1)); want="MERCHANT_APPROVED,API_KEY_CREATED,WEBHOOK_CREATED,MERCHANT_SUSPENDED,MERCHANT_REINSTATED,API_KEY_REVOKED,API_KEY_CREATED,MERCHANT_DISABLED"
if [ "$AUD" = "$want" ]; then PASS=$((PASS+1)); echo "PASS $N  audit trail for A is complete and in order"; else FAIL=$((FAIL+1)); echo "FAIL $N  audit trail: $AUD"; fi
N=$((N+1)); LEAK=$("${PSQL[@]}" "SELECT count(*) FROM audit_logs WHERE new_state::text LIKE '%$SECRET_PART%' OR new_state::text LIKE '%whsec_%'")
if [ "$LEAK" = 0 ]; then PASS=$((PASS+1)); echo "PASS $N  no API key secret or webhook secret in audit rows"; else FAIL=$((FAIL+1)); echo "FAIL $N  secrets in audit rows"; fi
N=$((N+1)); STORED=$("${PSQL[@]}" "SELECT count(*) FROM merchant_api_credentials WHERE key_hash LIKE '%$SECRET_PART%'")
WS=$("${PSQL[@]}" "SELECT count(*) FROM webhook_endpoints WHERE convert_from(secret_ciphertext,'SQL_ASCII') LIKE '%${WSECRET:6}%'" 2>/dev/null || echo 0)
if [ "$STORED" = 0 ] && [ "$WS" = 0 ]; then PASS=$((PASS+1)); echo "PASS $N  database holds only key hash and encrypted webhook secret"; else FAIL=$((FAIL+1)); echo "FAIL $N  plaintext found in DB"; fi
N=$((N+1)); VIEW=$("${PSQL[@]}" "SELECT count(*) FROM merchant_webhook_targets WHERE merchant_id='$MID_A'")
if [ "$VIEW" = 0 ]; then PASS=$((PASS+1)); echo "PASS $N  disabled merchant's webhooks are gone from Member 8's view"; else FAIL=$((FAIL+1)); echo "FAIL $N  view still lists $VIEW"; fi
N=$((N+1)); LOGS=$(docker compose logs api 2>&1)
if ! grep -q -- "$SECRET_PART" <<<"$LOGS" && ! grep -q -- "${WSECRET:6}" <<<"$LOGS" && ! grep -q "Bearer " <<<"$LOGS"; then PASS=$((PASS+1)); echo "PASS $N  api logs contain no key, webhook secret or bearer token"; else FAIL=$((FAIL+1)); echo "FAIL $N  secret found in logs"; fi
call 200 GET /metrics; check "metrics expose auth results and SSRF blocks" "$( [ "$(has 'merchant_api_auth_total{result="revoked"}')" = 0 ] && has webhook_ssrf_blocked_total || echo 1)"

echo
echo "RESULT: $PASS passed, $FAIL failed (of $N)"
rm -f "$HDR"
[ "$FAIL" = 0 ]
