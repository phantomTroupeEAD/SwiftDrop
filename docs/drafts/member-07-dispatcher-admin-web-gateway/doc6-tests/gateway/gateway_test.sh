#!/usr/bin/env bash
# Gateway smoke tests for SwiftDrop (Member 7). Runs the REAL Traefik binary with the
# repo's traefik.yml and dynamic-routes.yml. Only ports and backend host names are
# rewritten so the test can run on one machine without Docker.
# Usage: gateway_test.sh <repo>/deploy/gateway/traefik <path-to-traefik-binary>
set -uo pipefail
CFG_DIR=${1:?traefik config dir}; TRAEFIK=${2:?traefik binary}
HERE=$(cd "$(dirname "$0")" && pwd); WORK=$(mktemp -d); mkdir -p "$WORK/dynamic"
GW=http://127.0.0.1:18000
PASS=0; FAIL=0; FINDINGS=0; PIDS=()
cleanup() { for p in "${PIDS[@]}"; do kill "$p" 2>/dev/null; done; rm -rf "$WORK"; }
trap cleanup EXIT

# 1. Copy config, rewrite only ports and hosts.
sed -e 's#address: ":80"#address: ":18000"#' -e 's#address: ":443"#address: ":18443"#' \
    -e "s#directory: \"/etc/traefik/dynamic\"#directory: \"$WORK/dynamic\"#" \
    "$CFG_DIR/traefik.yml" > "$WORK/traefik.yml"
sed -e 's#http://core-api:8080#http://127.0.0.1:19080#' -e 's#http://tracking-api:8081#http://127.0.0.1:19081#' \
    -e 's#http://ai-assistant:8000#http://127.0.0.1:19000#' -e 's#http://web:3000#http://127.0.0.1:19300#' \
    "$CFG_DIR/dynamic-routes.yml" > "$WORK/dynamic/dynamic-routes.yml"

# 2. Start stub backends and Traefik.
for s in "core-api 19080" "tracking-api 19081" "ai-assistant 19000" "web 19300"; do
  python3 -I "$HERE/stub_backend.py" $s & PIDS+=($!)
done
"$TRAEFIK" --configFile="$WORK/traefik.yml" > "$WORK/traefik.log" 2>&1 & PIDS+=($!)
for i in $(seq 1 50); do curl -s -o /dev/null "$GW/" && break; sleep 0.2; done
sleep 1

check() { # id, description, expected, actual, [FINDING]
  local id=$1 desc=$2 exp=$3 act=$4 tag=${5:-}
  if [ "$exp" == "$act" ]; then
    PASS=$((PASS+1)); [ -n "$tag" ] && FINDINGS=$((FINDINGS+1))
    printf 'PASS  %-7s %-62s %s\n' "$id" "$desc" "${tag:+[FINDING: works as configured today, needs fix]}"
  else
    FAIL=$((FAIL+1)); printf 'FAIL  %-7s %-62s expected=%s actual=%s\n' "$id" "$desc" "$exp" "$act"
  fi
}
svc() { curl -s "$GW$1" | python3 -c 'import sys,json
try: print(json.load(sys.stdin)["service"])
except Exception: print("none")'; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

echo "== Routing (INV-06) =="
check GW-01 "/api/v1/tracking/public/SD-1 goes to tracking-api" tracking-api "$(svc /api/v1/tracking/public/SD-1)"
check GW-02 "/api/v1/ai/chat goes to ai-assistant" ai-assistant "$(svc /api/v1/ai/chat)"
check GW-03 "/api/v1/dispatch/board goes to core-api" core-api "$(svc /api/v1/dispatch/board)"
check GW-04 "/api/v1/admin/zones goes to core-api" core-api "$(svc /api/v1/admin/zones)"
check GW-05 "/ops/dispatch goes to web" web "$(svc /ops/dispatch)"
check GW-06 "/api/v1/trackingX is taken by tracking router (R-GW-04)" tracking-api "$(svc /api/v1/trackingX)" FINDING
check GW-07 "/api/v2/parcels falls through to web, not 404 (Doc 2 E-15)" web "$(svc /api/v2/parcels)" FINDING

echo "== Internal paths not public (INV-05) =="
check GW-08 "/health/live does not reach core-api" web "$(svc /health/live)"
check GW-09 "/metrics does not reach core-api" web "$(svc /metrics)"
check GW-10 "dashboard reachable on :8080 because api.insecure=true (M7-GW-06)" 200 "$(code http://127.0.0.1:8080/api/rawdata)" FINDING

echo "== Auth headers and 401/403 pass through unchanged (INV-04, M7-GW-10) =="
AUTH=$(curl -s -H 'Authorization: Bearer abc.def' -H 'X-API-Key: sd_live_test' "$GW/api/v1/parcels")
check GW-11 "Authorization header reaches backend unchanged" "Bearer abc.def" "$(echo "$AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["authorization"])')"
check GW-12 "X-API-Key header reaches backend unchanged" "sd_live_test" "$(echo "$AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["x_api_key"])')"
check GW-13 "401 status passes through" 401 "$(code "$GW/api/v1/test/401")"
check GW-14 "401 WWW-Authenticate header passes through" 'Bearer error="invalid_token"' "$(curl -s -D - -o /dev/null "$GW/api/v1/test/401" | tr -d '\r' | sed -n 's/^[Ww]ww-[Aa]uthenticate: //p')"
check GW-15 "401 body passes through" AUTH_TOKEN_EXPIRED "$(curl -s "$GW/api/v1/test/401" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')"
check GW-16 "403 status and body pass through" "403 ACCESS_DENIED" "$(code "$GW/api/v1/test/403") $(curl -s "$GW/api/v1/test/403" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')"

echo "== Upload size limit 10 MB on core-api (M7-GW-03) =="
head -c 9437184 /dev/zero > "$WORK/9mb"; head -c 11534336 /dev/zero > "$WORK/11mb"
check GW-17 "9 MB POST to core-api passes" 200 "$(code -X POST --data-binary @"$WORK/9mb" "$GW/api/v1/deliveries/p1/complete")"
check GW-18 "11 MB POST to core-api is refused with 413" 413 "$(code -X POST --data-binary @"$WORK/11mb" "$GW/api/v1/deliveries/p1/complete")"

echo "== CORS (M7-GW-04) =="
pre() { curl -s -D - -o /dev/null -X OPTIONS -H "Origin: $1" -H 'Access-Control-Request-Method: POST' -H "Access-Control-Request-Headers: $2" "$GW/api/v1/admin/zones" | tr -d '\r'; }
check GW-19 "preflight from http://localhost:3000 is allowed" "http://localhost:3000" "$(pre http://localhost:3000 'authorization,idempotency-key' | sed -n 's/^[Aa]ccess-[Cc]ontrol-[Aa]llow-[Oo]rigin: //p')"
check GW-20 "preflight from https://evil.example gets no allow-origin" "" "$(pre https://evil.example authorization | sed -n 's/^[Aa]ccess-[Cc]ontrol-[Aa]llow-[Oo]rigin: //p')"
check GW-21 "X-Request-ID is NOT in allowed headers (differs from security-notes)" no "$(pre http://localhost:3000 x-request-id | grep -qi '^access-control-allow-headers:.*x-request-id' && echo yes || echo no)" FINDING

echo "== SSE through the gateway is not buffered (INV-07) =="
T=$(python3 -I - <<PY
import time, urllib.request
t0=time.time(); r=urllib.request.urlopen("$GW/api/v1/tracking/public/SD-1/stream", timeout=10)
r.readline(); first=time.time()-t0; r.read(); total=time.time()-t0
print("ok" if first < 1.0 and total > 2.0 else f"buffered first={first:.2f}s total={total:.2f}s")
PY
)
check GW-22 "first SSE event arrives before the stream ends" ok "$T"

echo "== Tracking rate limit 30/min burst 50 (M7-GW-02) =="
CODES=$(for i in $(seq 1 60); do code -H 'X-Forwarded-For: 203.0.113.10' "$GW/api/v1/tracking/public/SD-2"; echo; done | sort | uniq -c | tr '\n' ' ')
N429=$(echo "$CODES" | grep -o '[0-9]* 429' | awk '{print $1}'); N429=${N429:-0}
check GW-23 "60 quick requests from one visitor get some 429s" yes "$([ "$N429" -gt 0 ] && echo yes || echo no)"
check GW-24 "a DIFFERENT visitor (X-Forwarded-For) shares the same bucket (Doc 2 G-02)" 429 "$(code -H 'X-Forwarded-For: 198.51.100.77' "$GW/api/v1/tracking/public/SD-3")" FINDING
check GW-25 "core-api routes have no rate limit today (login limit missing, U-06)" 0 "$(for i in $(seq 1 60); do code "$GW/api/v1/auth/login"; echo; done | grep -c 429)" FINDING

echo
echo "Summary: $PASS passed, $FAIL failed ($FINDINGS of the passes document known gaps). Rate-limit result: $CODES"
[ "$FAIL" -eq 0 ]
