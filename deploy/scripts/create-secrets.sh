#!/usr/bin/env bash
# Creates every Secret (and the messaging ConfigMap) the prod overlay needs, in namespace swiftdrop.
# Member 9. Run from the repo root in Git Bash / Linux with the ADMIN kubeconfig:
#
#   KUBECONFIG=~/.kube/swiftdrop-azure.yaml bash deploy/scripts/create-secrets.sh
#
# - Generates strong random values itself and keeps them consistent (each role password matches
#   the password inside its DATABASE_URL, one webhook key in both places, one JWT secret).
# - Never prints a secret. The only values written to disk are the demo-account and Grafana
#   passwords you need to log in, saved OUTSIDE the repo in ~/swiftdrop-secrets/logins.txt.
# - A Secret that already exists is left alone (Postgres/RabbitMQ keep the password they were
#   first started with). To rotate one: kubectl -n swiftdrop delete secret <name>, then re-run.
# - Asks only for what only you have: GitHub token (read:packages) and the Cloudflare origin cert.
set -euo pipefail

NS="swiftdrop"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="${HOME}/swiftdrop-secrets"
PY="$(command -v python3 || command -v python)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

rand() { "$PY" -c "import secrets;print(secrets.token_urlsafe($1))"; }   # URL-safe: fine inside URLs
b64key() { "$PY" -c "import secrets,base64;print(base64.b64encode(secrets.token_bytes(32)).decode())"; }
exists() { kubectl -n "$NS" get "$1" "$2" >/dev/null 2>&1; }

# create_from_env <secret-name> : reads KEY=VALUE lines from $TMP/<name>.env
create_from_env() {
  local name="$1"
  if exists secret "$name"; then echo "  = $name (exists, unchanged)"; return; fi
  kubectl -n "$NS" create secret generic "$name" --from-env-file="$TMP/$name.env" >/dev/null
  echo "  + $name"
}

kubectl get namespace "$NS" >/dev/null 2>&1 || kubectl apply -f "$REPO_ROOT/deploy/kubernetes/namespace.yaml" >/dev/null
mkdir -p "$OUT_DIR" && chmod 700 "$OUT_DIR" 2>/dev/null || true

echo "==> Generated secrets"
# --- PostgreSQL: superuser + one password per role, URLs built from the same values ---
PG_PASS="$(rand 24)"
declare -A ROLE_PW
for r in MIGRATOR APP TRACKING RELAY WORKER RECONCILER; do ROLE_PW[$r]="$(rand 24)"; done
url() { echo "postgres://$1:$2@postgres:5432/swiftdrop_db?sslmode=disable"; }
{
  echo "POSTGRES_USER=swiftdrop"; echo "POSTGRES_PASSWORD=$PG_PASS"; echo "POSTGRES_DB=swiftdrop_db"
  for r in MIGRATOR APP TRACKING RELAY WORKER RECONCILER; do echo "${r}_PASSWORD=${ROLE_PW[$r]}"; done
  echo "MIGRATOR_DATABASE_URL=$(url swiftdrop_migrator "${ROLE_PW[MIGRATOR]}")"
  echo "APP_DATABASE_URL=$(url swiftdrop_app "${ROLE_PW[APP]}")"
  echo "TRACKING_DATABASE_URL=$(url swiftdrop_tracking_ro "${ROLE_PW[TRACKING]}")"
  echo "RELAY_DATABASE_URL=$(url swiftdrop_relay "${ROLE_PW[RELAY]}")"
  echo "WORKER_DATABASE_URL=$(url swiftdrop_worker "${ROLE_PW[WORKER]}")"
  echo "RECONCILER_DATABASE_URL=$(url swiftdrop_reconciler "${ROLE_PW[RECONCILER]}")"
} > "$TMP/postgres-secrets.env"
if exists secret postgres-secrets; then
  # Reuse the existing relay/worker passwords so their secrets below stay consistent.
  ROLE_PW[RELAY]="$(kubectl -n "$NS" get secret postgres-secrets -o jsonpath='{.data.RELAY_PASSWORD}' | base64 -d)"
  ROLE_PW[WORKER]="$(kubectl -n "$NS" get secret postgres-secrets -o jsonpath='{.data.WORKER_PASSWORD}' | base64 -d)"
fi
create_from_env postgres-secrets
echo "DATABASE_URL=$(url swiftdrop_relay "${ROLE_PW[RELAY]}")" > "$TMP/outbox-relay-secrets.env"
create_from_env outbox-relay-secrets
echo "DATABASE_URL=$(url swiftdrop_worker "${ROLE_PW[WORKER]}")" > "$TMP/event-worker-secrets.env"
create_from_env event-worker-secrets

# --- RabbitMQ ---
MQ_PASS="$(rand 24)"
printf 'RABBITMQ_DEFAULT_USER=swiftdrop\nRABBITMQ_DEFAULT_PASS=%s\nRABBITMQ_URL=amqp://swiftdrop:%s@rabbitmq:5672/\n' "$MQ_PASS" "$MQ_PASS" > "$TMP/rabbitmq-secrets.env"
create_from_env rabbitmq-secrets

# --- core-api (+ the same webhook key for the worker) ---
WEBHOOK_KEY="$(b64key)"
if exists secret core-api-secrets; then
  WEBHOOK_KEY="$(kubectl -n "$NS" get secret core-api-secrets -o jsonpath='{.data.WEBHOOK_SECRET_ENCRYPTION_KEY}' | base64 -d)"
fi
{
  echo "JWT_SECRET=$(rand 48)"; echo "MERCHANT_API_KEY_PEPPER=$(rand 48)"; echo "WEBHOOK_SECRET_ENCRYPTION_KEY=$WEBHOOK_KEY"
} > "$TMP/core-api-secrets.env"
NEW_CORE=0; exists secret core-api-secrets || NEW_CORE=1
if [[ $NEW_CORE == 1 ]]; then
  : > "$TMP/logins.txt"
  for r in ADMIN DISPATCHER COURIER MERCHANT CUSTOMER; do
    pw="$(rand 15)"; echo "SEED_PASSWORD_$r=$pw" >> "$TMP/core-api-secrets.env"; echo "SEED_PASSWORD_$r=$pw" >> "$TMP/logins.txt"
  done
fi
create_from_env core-api-secrets
printf 'WEBHOOK_SECRET_ENCRYPTION_KEY=%s\nWEBHOOK_SECRET_ENCRYPTION_KEY_FALLBACKS=\nSMTP_USERNAME=\nSMTP_PASSWORD=\n' "$WEBHOOK_KEY" > "$TMP/integrations-secrets.env"
create_from_env integrations-secrets

# --- ai-assistant: empty LLM_API_KEY = offline mock mode until Member 10 provides a real key ---
printf 'LLM_API_KEY=\nSWIFTDROP_API_KEY=\nAI_SERVICE_ROLE_TOKEN=%s\n' "$(rand 32)" > "$TMP/ai-assistant-secrets.env"
create_from_env ai-assistant-secrets

# --- Grafana admin ---
GF_PASS="$(rand 18)"
echo "GF_SECURITY_ADMIN_PASSWORD=$GF_PASS" > "$TMP/grafana-admin.env"
NEW_GF=0; exists secret grafana-admin || NEW_GF=1
create_from_env grafana-admin

# --- messaging-config ConfigMap (ConfigMap part of the template; it holds no secrets) ---
if exists configmap messaging-config; then echo "  = messaging-config (exists, unchanged)"; else
  "$PY" - "$REPO_ROOT/deploy/kubernetes/messaging/messaging-config.yaml.example" > "$TMP/messaging-config.yaml" <<'EOF'
import sys, yaml
docs = [d for d in yaml.safe_load_all(open(sys.argv[1], encoding="utf-8")) if d]
print(yaml.safe_dump(next(d for d in docs if d["kind"] == "ConfigMap"), sort_keys=False))
EOF
  kubectl apply -f "$TMP/messaging-config.yaml" >/dev/null && echo "  + messaging-config (ConfigMap)"
fi

# --- Save the passwords you need to log in (outside the repo) ---
if [[ $NEW_CORE == 1 || $NEW_GF == 1 ]]; then
  {
    echo "# SwiftDrop production logins — created $(date -u +%F). Keep private. Not in git."
    [[ $NEW_CORE == 1 ]] && sed 's/^SEED_PASSWORD_/demo /' "$TMP/logins.txt"
    [[ $NEW_GF == 1 ]] && echo "grafana admin=$GF_PASS"
  } >> "$OUT_DIR/logins.txt"
  chmod 600 "$OUT_DIR/logins.txt" 2>/dev/null || true
  echo "  Saved login passwords to $OUT_DIR/logins.txt (not printed)"
fi

echo; echo "==> Secrets only you can provide"
if exists secret ghcr-pull; then echo "  = ghcr-pull (exists, unchanged)"; else
  read -rp "  GitHub username (for pulling images from ghcr.io, Enter to skip): " GH_USER
  if [[ -n "$GH_USER" ]]; then
    read -rsp "  GitHub token with read:packages (hidden): " GH_TOKEN; echo
    kubectl -n "$NS" create secret docker-registry ghcr-pull --docker-server=ghcr.io \
      --docker-username="$GH_USER" --docker-password="$GH_TOKEN" >/dev/null && echo "  + ghcr-pull"
    unset GH_TOKEN
  else echo "  ! ghcr-pull skipped: pods will show ImagePullBackOff until it exists"; fi
fi
if exists secret traefik-origin-tls; then echo "  = traefik-origin-tls (exists, unchanged)"; else
  read -rp "  Path to Cloudflare origin certificate (origin.pem), Enter to skip: " CERT
  if [[ -n "$CERT" ]]; then
    read -rp "  Path to its private key (origin.key): " KEY
    kubectl -n "$NS" create secret tls traefik-origin-tls --cert="$CERT" --key="$KEY" >/dev/null && echo "  + traefik-origin-tls"
  else echo "  ! traefik-origin-tls skipped: needed before https://swiftdrop.work works"; fi
fi

echo; echo "==> Done. In namespace $NS:"
kubectl -n "$NS" get secrets --no-headers 2>/dev/null | awk '{print "  secret/" $1}'
kubectl -n "$NS" get configmap messaging-config --no-headers 2>/dev/null | awk '{print "  configmap/" $1}'
