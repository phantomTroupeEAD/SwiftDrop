#!/usr/bin/env bash
# Stop everything local-up.sh started. Add --wipe to delete the local database too.
set -uo pipefail
RUN=${RUN_DIR:-/tmp/m7-local}
PGBIN=${PGBIN:-$(ls -d /usr/lib/postgresql/16/bin 2>/dev/null || dirname "$(command -v postgres)")}
as_pg() { if [ "$(id -u)" = 0 ]; then runuser -u postgres -- "$@"; else "$@"; fi; }
PIDS=""
for f in traefik core-api stubs; do
  [ -f "$RUN/$f.pid" ] && PIDS="$PIDS $(cat "$RUN/$f.pid")" && kill $(cat "$RUN/$f.pid") 2>/dev/null; rm -f "$RUN/$f.pid"
done
# core-api drains for READINESS_DRAIN_SECONDS before it exits; wait so its port is free
# for the next "make up" (up to 30 s).
for i in $(seq 1 60); do
  alive=0; for p in $PIDS; do kill -0 "$p" 2>/dev/null && alive=1; done
  [ "$alive" = 0 ] && break; sleep 0.5
done
as_pg "$PGBIN/pg_ctl" -D "$RUN/pg" -m fast stop >/dev/null 2>&1 && echo "stopped"
if [ "${1:-}" = "--wipe" ]; then rm -rf "$RUN" && echo "wiped $RUN"; fi
exit 0
