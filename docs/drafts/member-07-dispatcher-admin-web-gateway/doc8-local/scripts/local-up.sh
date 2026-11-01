#!/usr/bin/env bash
# Start Member 7's module locally WITHOUT Docker:
#   PostgreSQL 16 (real) -> migrations -> seed -> core-api -> Traefik gateway (+ stand-ins
#   for tracking-api, ai-assistant and web, which other members own).
# Needs: go 1.22+, PostgreSQL 16 server binaries, golang-migrate built with -tags postgres,
#        traefik v3.1 binary, python3, curl.
# Usage: scripts/local-up.sh            (stop with scripts/local-down.sh)
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
RUN=${RUN_DIR:-/tmp/m7-local}
PGBIN=${PGBIN:-$(ls -d /usr/lib/postgresql/16/bin 2>/dev/null || dirname "$(command -v postgres)")}
TRAEFIK=${TRAEFIK:-traefik}
MIGRATE=${MIGRATE:-migrate}
PGPORT=${PGPORT:-5432}
GW_PORT=${GW_PORT:-8088}
mkdir -p "$RUN/logs"
as_pg() { if [ "$(id -u)" = 0 ]; then runuser -u postgres -- "$@"; else "$@"; fi; }

echo "[1/7] PostgreSQL 16 on :$PGPORT"
if [ ! -f "$RUN/pg/PG_VERSION" ]; then
  mkdir -p "$RUN/pg"; [ "$(id -u)" = 0 ] && chown postgres "$RUN/pg"
  as_pg "$PGBIN/initdb" -D "$RUN/pg" -U postgres --auth=trust >/dev/null
fi
as_pg "$PGBIN/pg_ctl" -D "$RUN/pg" status >/dev/null 2>&1 || as_pg "$PGBIN/pg_ctl" -D "$RUN/pg" -o "-p $PGPORT -k /tmp" -l "$RUN/pg/postgres.log" -w start >/dev/null
psql_su() { psql -h /tmp -p "$PGPORT" -U postgres -v ON_ERROR_STOP=1 -q "$@"; }
psql_su -tc "SELECT 1 FROM pg_database WHERE datname='swiftdrop_db'" | grep -q 1 || psql_su -c "CREATE DATABASE swiftdrop_db"

echo "[2/7] Roles and grants (scripts/db-bootstrap.sql)"
psql_su -d swiftdrop_db -f "$ROOT/scripts/db-bootstrap.sql"

set -a; . "$ROOT/.env.example"; [ -f "$ROOT/.env" ] && . "$ROOT/.env"; set +a
export DATABASE_URL=${DATABASE_URL/:5432/:$PGPORT} MIGRATE_DATABASE_URL=${MIGRATE_DATABASE_URL/:5432/:$PGPORT}

echo "[3/7] Migrations (golang-migrate, owner role)"
"$MIGRATE" -path "$ROOT/core-api/migrations" -database "$MIGRATE_DATABASE_URL" up 2>&1 | sed 's/^/      /'

echo "[4/7] Seed (7 zones, 3 settings, demo users)"
(cd "$ROOT/core-api" && DATABASE_URL=$MIGRATE_DATABASE_URL go run ./cmd/seed | sed 's/^/      /')

echo "[5/7] Dev JWT key pair"
(cd "$ROOT/core-api" && [ -f .dev/jwt_public.pem ] || go run ./cmd/devtoken -genkey .dev)

echo "[6/7] core-api on $HTTP_ADDR (app role)"
(cd "$ROOT/core-api" && go build -o "$RUN/core-api" ./cmd/api)
(cd "$ROOT/core-api" && exec "$RUN/core-api" >> "$RUN/logs/core-api.log" 2>&1 < /dev/null) & echo $! > "$RUN/core-api.pid"
for i in $(seq 1 50); do curl -sf localhost:8080/readyz >/dev/null && break; sleep 0.2; done
curl -s localhost:8080/readyz; echo

echo "[7/7] Traefik gateway on :$GW_PORT (+ stand-ins for other members' services)"
mkdir -p "$RUN/gw/dynamic"
sed -e "s#address: \":80\"#address: \":$GW_PORT\"#" -e 's#address: ":443"#address: ":8443"#' \
    -e "s#directory: \"/etc/traefik/dynamic\"#directory: \"$RUN/gw/dynamic\"#" "$ROOT/deploy/gateway/traefik/traefik.yml" > "$RUN/gw/traefik.yml"
sed -e 's#http://core-api:8080#http://127.0.0.1:8080#' -e 's#http://tracking-api:8081#http://127.0.0.1:8081#' \
    -e 's#http://ai-assistant:8000#http://127.0.0.1:8000#' -e 's#http://web:3000#http://127.0.0.1:3000#' \
    "$ROOT/deploy/gateway/traefik/dynamic-routes.yml" > "$RUN/gw/dynamic/routes.yml"
for s in "tracking-api 8081" "web 3000"; do
  python3 -I "$ROOT/tools/stub_backend.py" $s >/dev/null 2>&1 < /dev/null & echo $! >> "$RUN/stubs.pid"
done
"$TRAEFIK" --configFile="$RUN/gw/traefik.yml" > "$RUN/logs/traefik.log" 2>&1 < /dev/null & echo $! > "$RUN/traefik.pid"
for i in $(seq 1 50); do curl -s -o /dev/null "localhost:$GW_PORT/" && break; sleep 0.2; done
echo
echo "Ready. Gateway: http://localhost:$GW_PORT  core-api: http://localhost:8080  Traefik dashboard (internal): http://localhost:8090/dashboard/"
echo "Token:  (cd core-api && go run ./cmd/devtoken -role ADMIN)"
echo "Logs:   $RUN/logs/"
