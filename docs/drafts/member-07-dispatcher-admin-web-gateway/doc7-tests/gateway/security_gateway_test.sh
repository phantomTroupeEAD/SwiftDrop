#!/usr/bin/env bash
# Doc 7 gateway security and failure checks (Member 7). Runs the REAL Traefik binary.
# Usage: security_gateway_test.sh <static.yml> <dynamic.yml> <traefik-binary>
# Run it once with the repo config (shows today's gaps) and once with the proposed config.
# Test-only rewrites: ports, backend hosts, and responseHeaderTimeout 30s -> 2s (so the
# timeout check takes seconds, not half a minute). Nothing else is changed.
set -uo pipefail
STATIC=${1:?static config}; DYN=${2:?dynamic config}; TRAEFIK=${3:?traefik binary}
HERE=$(cd "$(dirname "$0")" && pwd); WORK=$(mktemp -d); mkdir -p "$WORK/dynamic"
GW=http://127.0.0.1:18000
PASS=0; FAIL=0; PIDS=()
cleanup() { for p in "${PIDS[@]}"; do kill "$p" 2>/dev/null; done; rm -rf "$WORK"; }
trap cleanup EXIT

sed -e 's#address: ":80"#address: ":18000"#' -e 's#address: ":443"#address: ":18443"#' \
    -e 's#address: ":8090"#address: ":18090"#' \
    -e "s#directory: \"/etc/traefik/dynamic\"#directory: \"$WORK/dynamic\"#" "$STATIC" > "$WORK/traefik.yml"
sed -e 's#http://core-api:8080#http://127.0.0.1:19080#' -e 's#http://tracking-api:8081#http://127.0.0.1:19081#' \
    -e 's#http://ai-assistant:8000#http://127.0.0.1:19000#' -e 's#http://web:3000#http://127.0.0.1:19300#' \
    -e 's#responseHeaderTimeout: 30s#responseHeaderTimeout: 2s#' "$DYN" > "$WORK/dynamic/routes.yml"

# ai-assistant is deliberately NOT started: it plays the "dependency is down" role (GS-15).
for s in "core-api 19080" "tracking-api 19081" "web 19300"; do
  python3 -I "$HERE/stub_backend.py" $s 2>/dev/null & PIDS+=($!)
done
"$TRAEFIK" --configFile="$WORK/traefik.yml" > "$WORK/traefik.log" 2>&1 & PIDS+=($!)
for i in $(seq 1 50); do curl -s -o /dev/null "$GW/" && break; sleep 0.2; done
sleep 1

check() { # id, description, expected, actual
  if [ "$3" == "$4" ]; then PASS=$((PASS+1)); printf 'PASS  %-6s %s\n' "$1" "$2"
  else FAIL=$((FAIL+1)); printf 'FAIL  %-6s %-66s expected=%s actual=%s\n' "$1" "$2" "$3" "$4"; fi
}
j() { python3 -c "import sys,json
try: d=json.load(sys.stdin); print($1)
except Exception: print('none')"; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
hdr() { curl -s -D - -o /dev/null "${@:2}" | tr -d '\r' | sed -n "s/^$1: //Ip" | head -1; }

echo "== Routing edges =="
check GS-01 "/api/v1/tracking/public/SD-1 goes to tracking-api" tracking-api "$(curl -s -H 'X-Forwarded-For: 192.0.2.1' $GW/api/v1/tracking/public/SD-1 | j 'd["service"]')"
check GS-02 "/api/v1/admin/zones goes to core-api" core-api "$(curl -s $GW/api/v1/admin/zones | j 'd["service"]')"
check GS-03 "/api/v1/trackingX is NOT taken by the tracking router" core-api "$(curl -s $GW/api/v1/trackingX | j 'd["service"]')"
check GS-04 "/api/v2/parcels goes to core-api (404 JSON), not the SPA" core-api "$(curl -s $GW/api/v2/parcels | j 'd["service"]')"

echo "== Abuse limits =="
for i in $(seq 1 60); do code -H 'X-Forwarded-For: 203.0.113.10' "$GW/api/v1/tracking/public/SD-2" >/dev/null; done
check GS-05 "visitor A used up tracking quota; visitor B still gets 200" 200 "$(code -H 'X-Forwarded-For: 198.51.100.77' "$GW/api/v1/tracking/public/SD-3")"
check GS-05b "faking an extra left-hand X-Forwarded-For entry does not reset A's quota" 429 "$(code -H 'X-Forwarded-For: 10.9.9.9, 203.0.113.10' "$GW/api/v1/tracking/public/SD-4")"
L=$(for i in $(seq 1 8); do code -X POST -H 'X-Forwarded-For: 203.0.113.20' "$GW/api/v1/auth/login"; echo; done | grep -c 429)
check GS-06 "8 quick logins from one visitor: some get 429" yes "$([ "$L" -gt 0 ] && echo yes || echo no)"
head -c 2097152 /dev/zero > "$WORK/2mb"; head -c 9437184 /dev/zero > "$WORK/9mb"; head -c 11534336 /dev/zero > "$WORK/11mb"
check GS-07 "2 MB body to a JSON route (/admin/zones) is refused with 413" 413 "$(code -X POST --data-binary @"$WORK/2mb" "$GW/api/v1/admin/zones")"
check GS-08 "9 MB proof photo passes, 11 MB is refused (Member 3 path)" "200 413" "$(code -X POST --data-binary @"$WORK/9mb" "$GW/api/v1/deliveries/d1/proof-photo") $(code -X POST --data-binary @"$WORK/11mb" "$GW/api/v1/deliveries/d1/proof-photo")"
S=$(for i in 1 2 3 4 5 6; do curl -s -o /dev/null -w '%{http_code}\n' -m 6 -H 'X-Forwarded-For: 203.0.113.30' "$GW/api/v1/tracking/public/SD-5/stream" & done; wait)
check GS-09 "6 open SSE streams from one visitor: exactly 1 refused (limit 5)" 1 "$(echo "$S" | grep -c 429)"
S=$(for i in 1 2 3 4 5 6; do curl -s -o /dev/null -w '%{http_code}\n' -m 6 -H 'X-Forwarded-For: 203.0.113.31' "$GW/api/v1/tracking/SD-7/stream" & done; wait)
check GS-09b "signed-in stream path gets the same stream router (1 of 6 refused)" 1 "$(echo "$S" | grep -c 429)"
A=$(for i in 1 2 3 4 5; do code -H 'X-Forwarded-For: 203.0.113.40' "$GW/api/v1/ai/chat"; echo; done | grep -c 429)
check GS-20 "5 quick AI calls from one visitor: some get 429 (10/min, burst 2)" yes "$([ "$A" -gt 0 ] && echo yes || echo no)"
M=$(for i in $(seq 1 30); do code -H 'X-Forwarded-For: 203.0.113.50' "$GW/api/v1/merchant/orders"; echo; done | grep -c 429)
check GS-21 "30 quick merchant API calls: some 429 (120/min, burst 20); /merchants (web) not limited" "yes 200" "$([ "$M" -gt 0 ] && echo yes || echo no) $(code -H 'X-Forwarded-For: 203.0.113.50' "$GW/api/v1/merchants/me")"

echo "== Identity headers and CORS =="
E=$(curl -s -H 'Authorization: Bearer abc.def' -H 'X-Tenant-ID: mrc_other' -H 'X-User-Role: ADMIN' "$GW/api/v1/parcels")
check GS-10 "client-sent X-Tenant-ID and X-User-Role are removed before core-api" "None None" "$(echo "$E" | j 'd["headers"].get("x-tenant-id"), d["headers"].get("x-user-role")' | tr -d "(),'")"
check GS-10b "Authorization header still reaches core-api unchanged" "Bearer abc.def" "$(echo "$E" | j 'd["headers"].get("authorization")')"
P=$(curl -s -D - -o /dev/null -X OPTIONS -H 'Origin: http://localhost:3000' -H 'Access-Control-Request-Method: POST' -H 'Access-Control-Request-Headers: x-request-id' "$GW/api/v1/admin/zones" | tr -d '\r' | sed -n 's/^access-control-allow-headers: //Ip')
check GS-11 "CORS allows X-Request-ID (security-notes 4) and not X-Tenant-ID" "yes no" "$(echo "$P" | grep -qi x-request-id && echo yes || echo no) $(echo "$P" | grep -qi x-tenant-id && echo yes || echo no)"
O=$(curl -s -D - -o /dev/null -H 'Origin: http://localhost:5173' "$GW/api/v1/admin/zones" | tr -d '\r' | sed -n 's/^access-control-allow-origin: //Ip')
X=$(curl -s -D - -o /dev/null -H 'Origin: http://127.0.0.1:3000' "$GW/api/v1/admin/zones" | tr -d '\r' | sed -n 's/^access-control-expose-headers: //Ip')
check GS-11b "CORS: :5173 not allowed; exposed headers exactly X-Request-ID, Content-Length" "none X-Request-ID,Content-Length" "${O:-none} $X"
check GS-12 "Traefik dashboard API is NOT reachable on open port 8080" no "$(curl -s -m 2 http://127.0.0.1:8080/api/rawdata | grep -q '"routers"' && echo yes || echo no)"

echo "== Dependency failure =="
T0=$(date +%s.%N); C=$(code -X POST "$GW/api/v1/test/slow"); T1=$(date +%s.%N)
check GS-13 "slow core-api: gateway answers 504 before the backend does" "504 fast" "$C $(python3 -c "print('fast' if $T1-$T0 < 3.5 else 'slow')")"
sleep 3
check GS-14 "after that timeout the POST reached core-api exactly once (no retry)" 1 "$(curl -s "$GW/api/v1/test/hits" | j 'd["slow"]')"
T0=$(date +%s.%N); C=$(code "$GW/api/v1/ai/chat"); T1=$(date +%s.%N)
check GS-15 "AI service down: 502 within 2 s, other routes unaffected" "502 fast 200" "$C $(python3 -c "print('fast' if $T1-$T0 < 2 else 'slow')") $(code $GW/api/v1/admin/zones)"

echo "== Response hardening =="
check GS-16 "API and SPA responses carry nosniff and frame DENY" "nosniff DENY nosniff DENY" "$(hdr X-Content-Type-Options $GW/api/v1/admin/zones) $(hdr X-Frame-Options $GW/api/v1/admin/zones) $(hdr X-Content-Type-Options $GW/ops/dispatch) $(hdr X-Frame-Options $GW/ops/dispatch)"
check GS-17 "backend X-Powered-By header is removed" "" "$(hdr X-Powered-By $GW/api/v1/admin/zones)"
check GS-18 "SPA has a Content-Security-Policy with frame-ancestors 'none'" yes "$(hdr Content-Security-Policy $GW/ops/dispatch | grep -q "frame-ancestors 'none'" && echo yes || echo no)"
F=$(python3 -I - <<PY
import time, urllib.request
req=urllib.request.Request("$GW/api/v1/tracking/public/SD-6/stream", headers={"X-Forwarded-For":"192.0.2.99"})
t0=time.time(); r=urllib.request.urlopen(req, timeout=10)
r.readline(); first=time.time()-t0; r.read(); total=time.time()-t0
print("ok" if first < 1.0 and total > 2.0 else f"buffered first={first:.2f}s total={total:.2f}s")
PY
)
check GS-19 "SSE is still not buffered with the new middlewares and timeouts" ok "$F"

echo
echo "Summary: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
