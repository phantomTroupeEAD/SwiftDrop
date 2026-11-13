#!/usr/bin/env bash
# SwiftDrop production deploy (Member 9). Used by .github/workflows/deploy.yml on the VM runner,
# and by hand from Git Bash/Linux with a kubeconfig for the cluster.
#
#   deploy/scripts/deploy.sh <sha7> [smoke_url]
#
#   sha7       7-character git SHA of the images published by release-images.yml
#   smoke_url  base URL for the smoke test (default https://swiftdrop.work); "none" skips it
#
# Env: DRY_RUN=1 renders and validates against the cluster (server-side dry run) without changing it.
#      SMOKE_PASSWORD_<ROLE> are passed through to the smoke test.
#
# Steps: render prod overlay with the SHA -> delete finished Jobs (Jobs are immutable) -> apply ->
# wait for the DB Jobs and every rollout -> smoke test. If a rollout or the smoke test fails, every
# app Deployment is rolled back to its previous revision and the script exits 1.
# Migrations are forward-only: a rollback restores the previous code, not the previous schema.
set -euo pipefail

SHA="${1:-}"
SMOKE_URL="${2:-https://swiftdrop.work}"
NS="swiftdrop"
TIMEOUT="${DEPLOY_TIMEOUT:-300s}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# Deployments that run our own images: these are rolled back on failure.
APP_DEPLOYMENTS=(core-api tracking-api outbox-relay event-worker ai-assistant web)

log() { printf '\n==> %s\n' "$*"; }
die() { printf '\nERROR: %s\n' "$*" >&2; exit 1; }

[[ "$SHA" =~ ^[0-9a-f]{7}$ ]] || die "usage: deploy.sh <7-char git sha> [smoke_url|none]"
command -v kubectl >/dev/null || die "kubectl not found"
command -v python3 >/dev/null || PYTHON=python || true
PYTHON="${PYTHON:-python3}"

RENDERED="$(mktemp)"
trap 'rm -f "$RENDERED"' EXIT

log "Render deploy/overlays/prod with image tag $SHA"
kubectl kustomize "$REPO_ROOT/deploy/overlays/prod" | sed "s/REPLACE_WITH_GIT_SHA/$SHA/g" > "$RENDERED"
if grep -q "REPLACE_WITH" "$RENDERED"; then
  die "placeholders left in the rendered manifests: $(grep -o 'REPLACE_WITH_[A-Z_]*' "$RENDERED" | sort -u | tr '\n' ' ')"
fi
echo "images: $(grep -o 'ghcr.io/[^ "]*' "$RENDERED" | sort -u | tr '\n' ' ')"

if [[ "${DRY_RUN:-0}" == "1" ]]; then
  log "DRY RUN: server-side validation only, nothing is changed"
  kubectl apply --dry-run=server -f "$RENDERED" >/dev/null
  echo "server dry run OK"
  exit 0
fi

rollback() {
  log "ROLLBACK: restoring the previous revision of every app Deployment"
  for d in "${APP_DEPLOYMENTS[@]}"; do
    kubectl -n "$NS" rollout undo "deployment/$d" 2>/dev/null && echo "  undo $d" || echo "  $d: no previous revision (first deploy?)"
  done
  for d in "${APP_DEPLOYMENTS[@]}"; do
    kubectl -n "$NS" rollout status "deployment/$d" --timeout="$TIMEOUT" >/dev/null 2>&1 || echo "  $d not healthy after undo"
  done
  log "Diagnostics"
  kubectl -n "$NS" get pods -o wide || true
  kubectl -n "$NS" get events --sort-by=.lastTimestamp 2>/dev/null | tail -20 || true
  echo "Note: migrations are forward-only; the schema was NOT rolled back."
  exit 1
}

log "Delete finished Jobs so they can run again with the new spec"
kubectl -n "$NS" delete job postgres-bootstrap-roles core-api-migrate --ignore-not-found

log "Apply"
kubectl apply -f "$RENDERED"

log "Wait for database Jobs"
kubectl -n "$NS" wait --for=condition=complete job/postgres-bootstrap-roles --timeout="$TIMEOUT" || rollback
kubectl -n "$NS" wait --for=condition=complete job/core-api-migrate --timeout="$TIMEOUT" || rollback

log "Wait for rollouts"
for kind_name in $(kubectl -n "$NS" get deployments,statefulsets -o name); do
  kubectl -n "$NS" rollout status "$kind_name" --timeout="$TIMEOUT" || rollback
done

if [[ "$SMOKE_URL" == "none" ]]; then
  log "Smoke test skipped (smoke_url=none)"
else
  log "Smoke test against $SMOKE_URL"
  "$PYTHON" "$REPO_ROOT/deploy/scripts/smoke-test.py" "$SMOKE_URL" || rollback
fi

log "Deployed $SHA successfully"
kubectl -n "$NS" get deployments -o custom-columns=NAME:.metadata.name,READY:.status.readyReplicas,IMAGE:.spec.template.spec.containers[0].image
