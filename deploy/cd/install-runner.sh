#!/usr/bin/env bash
# Installs the GitHub Actions self-hosted runner on the SwiftDrop VM (Member 9). Run ONCE on the VM:
#
#   scp deploy/cd/install-runner.sh azureuser@<vm-ip>:~ && ssh azureuser@<vm-ip>
#   bash install-runner.sh [runner_version]          # default below; it asks for the token
#
# Before running: kubectl apply -f deploy/cd/deployer-rbac.yaml (creates the cd-deployer token),
# and copy a registration token from GitHub: repo Settings -> Actions -> Runners ->
# New self-hosted runner. The token is typed in (hidden), never stored in git or shell history.
#
# Result: Linux user "gh-runner" (no sudo) runs the runner as a systemd service with labels
# self-hosted,linux,x64,swiftdrop-azure. Its kubeconfig uses the least-privilege cd-deployer
# ServiceAccount against https://127.0.0.1:6443, so no port is opened: the runner only makes
# outbound HTTPS calls to GitHub.
set -euo pipefail

RUNNER_VERSION="${1:-2.338.2}"
REPO_URL="https://github.com/phantomTroupeEAD/SwiftDrop"
RUNNER_USER="gh-runner"
RUNNER_HOME="/home/${RUNNER_USER}"
RUNNER_DIR="${RUNNER_HOME}/actions-runner"
RUNNER_NAME="swiftdrop-azure"
LABELS="swiftdrop-azure"

log() { printf '\n==> %s\n' "$*"; }

[[ "$(uname -m)" == "x86_64" ]] || { echo "x86_64 only"; exit 1; }
command -v k3s >/dev/null || { echo "k3s not found: install K3s first"; exit 1; }

read -rsp "GitHub runner registration token: " REG_TOKEN; echo
[[ -n "$REG_TOKEN" ]] || { echo "empty token"; exit 1; }

log "Packages the deploy job needs (git, python3, curl)"
sudo apt-get update -qq
sudo apt-get install -y -qq git python3 curl >/dev/null

log "User ${RUNNER_USER} (no sudo, no password login)"
id "$RUNNER_USER" >/dev/null 2>&1 || sudo useradd --create-home --shell /bin/bash "$RUNNER_USER"

log "kubeconfig for the cd-deployer ServiceAccount (namespace swiftdrop only)"
SA_TOKEN="$(sudo k3s kubectl -n swiftdrop get secret cd-deployer-token -o jsonpath='{.data.token}' | base64 -d)"
SA_CA="$(sudo k3s kubectl -n swiftdrop get secret cd-deployer-token -o jsonpath='{.data.ca\.crt}')"
[[ -n "$SA_TOKEN" && -n "$SA_CA" ]] || { echo "cd-deployer-token missing: apply deploy/cd/deployer-rbac.yaml first"; exit 1; }
sudo install -d -m 700 -o "$RUNNER_USER" -g "$RUNNER_USER" "${RUNNER_HOME}/.kube"
sudo tee "${RUNNER_HOME}/.kube/config" >/dev/null <<EOF
apiVersion: v1
kind: Config
clusters:
  - name: swiftdrop
    cluster:
      server: https://127.0.0.1:6443
      certificate-authority-data: ${SA_CA}
users:
  - name: cd-deployer
    user:
      token: ${SA_TOKEN}
contexts:
  - name: cd-deployer
    context: {cluster: swiftdrop, user: cd-deployer, namespace: swiftdrop}
current-context: cd-deployer
EOF
unset SA_TOKEN
sudo chown "$RUNNER_USER:$RUNNER_USER" "${RUNNER_HOME}/.kube/config"
sudo chmod 600 "${RUNNER_HOME}/.kube/config"
sudo -u "$RUNNER_USER" KUBECONFIG="${RUNNER_HOME}/.kube/config" kubectl auth whoami -o jsonpath='{.status.userInfo.username}{"\n"}'

log "Download Actions runner v${RUNNER_VERSION}"
TARBALL="actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"
sudo -u "$RUNNER_USER" mkdir -p "$RUNNER_DIR"
sudo -u "$RUNNER_USER" curl -fsSL -o "${RUNNER_DIR}/${TARBALL}" \
  "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/${TARBALL}"
sudo -u "$RUNNER_USER" tar -xzf "${RUNNER_DIR}/${TARBALL}" -C "$RUNNER_DIR"
sudo -u "$RUNNER_USER" rm -f "${RUNNER_DIR}/${TARBALL}"

log "Register runner ${RUNNER_NAME} (labels: ${LABELS})"
sudo -u "$RUNNER_USER" bash -c "cd '$RUNNER_DIR' && ./config.sh --unattended --replace \
  --url '$REPO_URL' --token '$REG_TOKEN' --name '$RUNNER_NAME' --labels '$LABELS' --work _work"
unset REG_TOKEN
# Every job on this runner uses the cd-deployer kubeconfig.
echo "KUBECONFIG=${RUNNER_HOME}/.kube/config" | sudo -u "$RUNNER_USER" tee -a "${RUNNER_DIR}/.env" >/dev/null

log "Install and start the systemd service"
# The runner home is private to gh-runner, so change into it as root (not as the calling user).
sudo bash -c "cd '$RUNNER_DIR' && ./svc.sh install '$RUNNER_USER' && ./svc.sh start"
sudo bash -c "cd '$RUNNER_DIR' && ./svc.sh status | head -5" || true

log "Done. GitHub -> Settings -> Actions -> Runners should show '${RUNNER_NAME}' as Idle."
