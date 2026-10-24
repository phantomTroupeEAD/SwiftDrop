#!/usr/bin/env bash
# Shows graceful shutdown: SIGTERM -> /readyz 503 DRAINING -> in-flight work finishes -> exit 0 -> restart.
set -uo pipefail
cd "$(dirname "$0")/.."
BASE=${BASE:-http://localhost:8084}

echo "== before: $(curl -s $BASE/readyz)"
docker compose kill -s SIGTERM api >/dev/null
sleep 0.5
code=$(curl -s -o /tmp/m4-ready.json -w '%{http_code}' $BASE/readyz)
echo "== during drain: HTTP $code $(cat /tmp/m4-ready.json)"
sleep 4
state=$(docker compose ps -a api --format '{{.State}} {{.Status}}')
echo "== after drain: container $state"
echo "== shutdown log lines:"
docker compose logs api 2>&1 | grep -E '"msg":"(shutdown started|shutdown complete|shutdown incomplete)"' | tail -2 | sed 's/^/   /'
docker compose start api >/dev/null
for _ in $(seq 1 30); do curl -fs $BASE/readyz >/dev/null 2>&1 && break; sleep 1; done
echo "== restarted: $(curl -s $BASE/readyz)"
[ "$code" = 503 ] && grep -q "Exited (0)" <<<"$state"
