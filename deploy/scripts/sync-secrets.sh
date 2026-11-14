#!/usr/bin/env bash
# Writes every Kubernetes Secret the prod overlay needs from environment variables. Member 9.
# Used by .github/workflows/deploy.yml: the values come from the GitHub "production" environment
# secrets, so nothing is stored in git or in a file on a laptop. Safe to run repeatedly: an unchanged
# Secret is left alone, a changed one is updated in place.
#
#   POSTGRES_PASSWORD=... APP_PASSWORD=... (see REQUIRED below) bash deploy/scripts/sync-secrets.sh
#
# Never prints a secret value. If any Secret was created or changed and GITHUB_ENV exists, it sets
# SECRETS_CHANGED=1 so deploy.sh restarts the pods that read them (env vars are read at start-up only).
#
# WARNING: POSTGRES_PASSWORD and RABBITMQ_PASSWORD are only read by Postgres/RabbitMQ the FIRST time
# their volume is created. Keep them equal to the values the running instances already use; changing
# them here does not change the password inside the database/broker.
set -euo pipefail
set +x

NS="swiftdrop"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP="$(mktemp -d)"
chmod 700 "$TMP"
trap 'rm -rf "$TMP"' EXIT

REQUIRED=(
  POSTGRES_PASSWORD
  MIGRATOR_PASSWORD APP_PASSWORD TRACKING_PASSWORD RELAY_PASSWORD WORKER_PASSWORD RECONCILER_PASSWORD
  RABBITMQ_PASSWORD
  JWT_SECRET MERCHANT_API_KEY_PEPPER WEBHOOK_SECRET_ENCRYPTION_KEY
  SEED_PASSWORD_ADMIN SEED_PASSWORD_DISPATCHER SEED_PASSWORD_COURIER SEED_PASSWORD_MERCHANT SEED_PASSWORD_CUSTOMER
  GRAFANA_ADMIN_PASSWORD
)
# Optional: GHCR_USER + GHCR_PULL_TOKEN (private images), ORIGIN_CERT_PEM + ORIGIN_KEY_PEM (Cloudflare TLS),
# AI_SERVICE_ROLE_TOKEN, LLM_API_KEY, SWIFTDROP_API_KEY (ai-assistant), SMTP_USERNAME, SMTP_PASSWORD.

missing=()
for v in "${REQUIRED[@]}"; do [[ -n "${!v:-}" ]] || missing+=("$v"); done
if ((${#missing[@]})); then
  echo "ERROR: these GitHub environment secrets are empty or missing:" >&2
  printf '  - %s\n' "${missing[@]}" >&2
  exit 1
fi
for v in WEBHOOK_SECRET_ENCRYPTION_KEY; do
  printf '%s' "${!v}" | base64 -d >/dev/null 2>&1 || { echo "ERROR: $v must be base64 (32 bytes)" >&2; exit 1; }
done
for v in SEED_PASSWORD_ADMIN SEED_PASSWORD_DISPATCHER SEED_PASSWORD_COURIER SEED_PASSWORD_MERCHANT SEED_PASSWORD_CUSTOMER; do
  val="${!v}"; ((${#val} >= 12)) || { echo "ERROR: $v must be at least 12 characters" >&2; exit 1; }
done
# Passwords go inside postgres:// and amqp:// URLs: only URL-safe characters are allowed.
for v in POSTGRES_PASSWORD MIGRATOR_PASSWORD APP_PASSWORD TRACKING_PASSWORD RELAY_PASSWORD WORKER_PASSWORD RECONCILER_PASSWORD RABBITMQ_PASSWORD; do
  [[ "${!v}" =~ ^[A-Za-z0-9._~-]+$ ]] || { echo "ERROR: $v may only contain letters, digits and . _ ~ - (it is embedded in a URL)" >&2; exit 1; }
done

CHANGED=0
# apply_secret <name> <env-file> : create or update from KEY=VALUE lines
apply_secret() {
  local out
  out="$(kubectl -n "$NS" create secret generic "$1" --from-env-file="$2" --dry-run=client -o yaml | kubectl apply -f -)"
  echo "  $out"
  [[ "$out" == *unchanged* ]] || CHANGED=1
}
url() { echo "postgres://$1:$2@postgres:5432/swiftdrop_db?sslmode=disable"; }

kubectl get namespace "$NS" >/dev/null 2>&1 || kubectl apply -f "$REPO_ROOT/deploy/kubernetes/namespace.yaml" >/dev/null

echo "==> Syncing secrets into namespace $NS"
{
  echo "POSTGRES_USER=swiftdrop"; echo "POSTGRES_PASSWORD=$POSTGRES_PASSWORD"; echo "POSTGRES_DB=swiftdrop_db"
  echo "MIGRATOR_PASSWORD=$MIGRATOR_PASSWORD"; echo "APP_PASSWORD=$APP_PASSWORD"; echo "TRACKING_PASSWORD=$TRACKING_PASSWORD"
  echo "RELAY_PASSWORD=$RELAY_PASSWORD"; echo "WORKER_PASSWORD=$WORKER_PASSWORD"; echo "RECONCILER_PASSWORD=$RECONCILER_PASSWORD"
  echo "MIGRATOR_DATABASE_URL=$(url swiftdrop_migrator "$MIGRATOR_PASSWORD")"
  echo "APP_DATABASE_URL=$(url swiftdrop_app "$APP_PASSWORD")"
  echo "TRACKING_DATABASE_URL=$(url swiftdrop_tracking_ro "$TRACKING_PASSWORD")"
  echo "RELAY_DATABASE_URL=$(url swiftdrop_relay "$RELAY_PASSWORD")"
  echo "WORKER_DATABASE_URL=$(url swiftdrop_worker "$WORKER_PASSWORD")"
  echo "RECONCILER_DATABASE_URL=$(url swiftdrop_reconciler "$RECONCILER_PASSWORD")"
} > "$TMP/postgres-secrets.env"
apply_secret postgres-secrets "$TMP/postgres-secrets.env"

echo "DATABASE_URL=$(url swiftdrop_relay "$RELAY_PASSWORD")" > "$TMP/outbox-relay-secrets.env"
apply_secret outbox-relay-secrets "$TMP/outbox-relay-secrets.env"
echo "DATABASE_URL=$(url swiftdrop_worker "$WORKER_PASSWORD")" > "$TMP/event-worker-secrets.env"
apply_secret event-worker-secrets "$TMP/event-worker-secrets.env"

printf 'RABBITMQ_DEFAULT_USER=swiftdrop\nRABBITMQ_DEFAULT_PASS=%s\nRABBITMQ_URL=amqp://swiftdrop:%s@rabbitmq:5672/\n' \
  "$RABBITMQ_PASSWORD" "$RABBITMQ_PASSWORD" > "$TMP/rabbitmq-secrets.env"
apply_secret rabbitmq-secrets "$TMP/rabbitmq-secrets.env"

{
  echo "JWT_SECRET=$JWT_SECRET"; echo "MERCHANT_API_KEY_PEPPER=$MERCHANT_API_KEY_PEPPER"
  echo "WEBHOOK_SECRET_ENCRYPTION_KEY=$WEBHOOK_SECRET_ENCRYPTION_KEY"
  for r in ADMIN DISPATCHER COURIER MERCHANT CUSTOMER; do v="SEED_PASSWORD_$r"; echo "$v=${!v}"; done
} > "$TMP/core-api-secrets.env"
apply_secret core-api-secrets "$TMP/core-api-secrets.env"

printf 'WEBHOOK_SECRET_ENCRYPTION_KEY=%s\nWEBHOOK_SECRET_ENCRYPTION_KEY_FALLBACKS=\nSMTP_USERNAME=%s\nSMTP_PASSWORD=%s\n' \
  "$WEBHOOK_SECRET_ENCRYPTION_KEY" "${SMTP_USERNAME:-}" "${SMTP_PASSWORD:-}" > "$TMP/integrations-secrets.env"
apply_secret integrations-secrets "$TMP/integrations-secrets.env"

printf 'LLM_API_KEY=%s\nSWIFTDROP_API_KEY=%s\nAI_SERVICE_ROLE_TOKEN=%s\n' \
  "${LLM_API_KEY:-}" "${SWIFTDROP_API_KEY:-}" "${AI_SERVICE_ROLE_TOKEN:-}" > "$TMP/ai-assistant-secrets.env"
apply_secret ai-assistant-secrets "$TMP/ai-assistant-secrets.env"

echo "GF_SECURITY_ADMIN_PASSWORD=$GRAFANA_ADMIN_PASSWORD" > "$TMP/grafana-admin.env"
apply_secret grafana-admin "$TMP/grafana-admin.env"

# Image pull secret and Cloudflare origin certificate: only when provided.
if [[ -n "${GHCR_PULL_TOKEN:-}" && -n "${GHCR_USER:-}" ]]; then
  out="$(kubectl -n "$NS" create secret docker-registry ghcr-pull --docker-server=ghcr.io \
    --docker-username="$GHCR_USER" --docker-password="$GHCR_PULL_TOKEN" --dry-run=client -o yaml | kubectl apply -f -)"
  echo "  $out"; [[ "$out" == *unchanged* ]] || CHANGED=1
else
  kubectl -n "$NS" get secret ghcr-pull >/dev/null 2>&1 && echo "  ghcr-pull kept (GHCR_USER/GHCR_PULL_TOKEN not provided)" \
    || echo "  ! ghcr-pull missing: pods will show ImagePullBackOff"
fi
if [[ -n "${ORIGIN_CERT_PEM:-}" && -n "${ORIGIN_KEY_PEM:-}" ]]; then
  printf '%s\n' "$ORIGIN_CERT_PEM" > "$TMP/origin.pem"; printf '%s\n' "$ORIGIN_KEY_PEM" > "$TMP/origin.key"
  out="$(kubectl -n "$NS" create secret tls traefik-origin-tls --cert="$TMP/origin.pem" --key="$TMP/origin.key" \
    --dry-run=client -o yaml | kubectl apply -f -)"
  echo "  $out"; [[ "$out" == *unchanged* ]] || CHANGED=1
else
  kubectl -n "$NS" get secret traefik-origin-tls >/dev/null 2>&1 && echo "  traefik-origin-tls kept (cert not provided)" \
    || echo "  ! traefik-origin-tls missing: https will not work until it exists"
fi

# messaging-config ConfigMap (no secrets in it): first YAML document of the template, only if absent.
if ! kubectl -n "$NS" get configmap messaging-config >/dev/null 2>&1; then
  awk '/^---/{exit} {print}' "$REPO_ROOT/deploy/kubernetes/messaging/messaging-config.yaml.example" | kubectl apply -f - >/dev/null
  echo "  configmap/messaging-config created"
fi

if [[ "$CHANGED" == 1 ]]; then
  echo "==> Some secrets changed: pods that read them will be restarted by deploy.sh"
  [[ -n "${GITHUB_ENV:-}" ]] && echo "SECRETS_CHANGED=1" >> "$GITHUB_ENV"
fi
echo "==> Secrets synced"
