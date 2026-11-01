#!/usr/bin/env bash
# Stop everything local-up.sh started. Add --wipe to delete the local database too.
set -uo pipefail
RUN=${RUN_DIR:-/tmp/m7-local}
PGBIN=${PGBIN:-$(ls -d /usr/lib/postgresql/16/bin 2>/dev/null || dirname "$(command -v postgres)")}
as_pg() { if [ "$(id -u)" = 0 ]; then runuser -u postgres -- "$@"; else "$@"; fi; }
for f in traefik core-api stubs; do
  [ -f "$RUN/$f.pid" ] && kill $(cat "$RUN/$f.pid") 2>/dev/null; rm -f "$RUN/$f.pid"
done
sleep 1
as_pg "$PGBIN/pg_ctl" -D "$RUN/pg" -m fast stop >/dev/null 2>&1 && echo "stopped"
if [ "${1:-}" = "--wipe" ]; then rm -rf "$RUN" && echo "wiped $RUN"; fi
exit 0
