#!/usr/bin/env bash
# Shows graceful shutdown: SIGTERM -> /health/ready says 503 -> running request finishes -> exit.
# Then starts core-api again. Needs the stack from local-up.sh.
set -uo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
RUN=${RUN_DIR:-/tmp/m7-local}
PID=$(cat "$RUN/core-api.pid")
ADMIN=$(cd "$ROOT/core-api" && go run ./cmd/devtoken -role ADMIN)
echo "1. ready before:  $(curl -s localhost:8080/health/ready)"
# Keep requests coming while SIGTERM arrives, to show that none are dropped.
( for i in $(seq 1 20); do curl -s -o /dev/null -w '%{http_code} ' -H "Authorization: Bearer $ADMIN" localhost:8080/api/v1/admin/zones; sleep 0.2; done; echo ) > "$RUN/inflight.txt" &
sleep 0.5
T0=$(date +%s.%N)
kill -TERM "$PID"
sleep 1
echo "2. ready 1 s after SIGTERM: $(curl -s localhost:8080/health/ready)  <- Kubernetes stops sending traffic"
echo "3. live at the same time:   $(curl -s localhost:8080/health/live)  <- not restarted, just draining"
while kill -0 "$PID" 2>/dev/null; do sleep 0.2; done
T1=$(date +%s.%N)
wait
echo "4. process exited after $(python3 -c "print(round($T1-$T0,1))") s (READINESS_DRAIN_SECONDS + finishing requests)"
echo "5. requests sent during drain got: $(cat "$RUN/inflight.txt")"
echo "6. shutdown log lines:"
grep -E '"msg":"(shutdown started|shutdown complete)"' "$RUN/logs/core-api.log" | tail -2 | sed 's/^/     /'
(cd "$ROOT/core-api" && set -a && . ../.env.example && set +a && exec "$RUN/core-api" >> "$RUN/logs/core-api.log" 2>&1 < /dev/null) & echo $! > "$RUN/core-api.pid"
for i in $(seq 1 50); do curl -sf localhost:8080/health/ready >/dev/null && break; sleep 0.2; done
echo "7. restarted, ready: $(curl -s localhost:8080/health/ready)"
