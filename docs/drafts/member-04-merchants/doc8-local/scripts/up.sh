#!/usr/bin/env bash
# Start the local merchants stack: dev keys -> .env -> postgres -> migrate -> seed -> api.
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p .dev
if [ ! -f .dev/jwt_public.pem ]; then
  echo "== generating dev JWT key pair (.dev/)"
  (cd core-api && go run ./cmd/devtoken -genkey ../.dev)
fi
if [ ! -f .dev/devtoken ] && [ ! -f .dev/devtoken.exe ]; then
  (cd core-api && go build -o ../.dev/ ./cmd/devtoken)
fi

if [ ! -f .env ]; then
  echo "== creating .env with fresh random secrets"
  pepper=$(head -c 48 /dev/urandom | base64 | tr -d '\n=+/' | head -c 48)
  enckey=$(head -c 32 /dev/urandom | base64 | tr -d '\n')
  sed -e "s|^MERCHANT_API_KEY_PEPPER=GENERATE|MERCHANT_API_KEY_PEPPER=${pepper}|" \
      -e "s|^WEBHOOK_SECRET_ENCRYPTION_KEY=GENERATE|WEBHOOK_SECRET_ENCRYPTION_KEY=${enckey}|" \
      .env.example > .env
fi

echo "== docker compose up"
docker compose up -d --build

echo -n "== waiting for http://localhost:8084/readyz "
for _ in $(seq 1 60); do
  if curl -fs http://localhost:8084/readyz >/dev/null 2>&1; then echo " READY"; exit 0; fi
  echo -n "."; sleep 1
done
echo " TIMEOUT"; docker compose logs --tail=50 api; exit 1
