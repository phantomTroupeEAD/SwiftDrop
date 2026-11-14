# SwiftDrop — Deployment Guide (Kubernetes on Azure, plus local Compose and k3d)

> **Authority:** SRS §11.1–11.2 (Deployment & Runtime Architecture), ADR-001 (Traefik), ADR-002 (hosting).
> **Owner:** Member 9 (DevOps, Kubernetes & Observability). **Gateway routes and middlewares:** Member 7.
> **Last updated:** 2026-11-12

---

## 1. Environments

| Environment | How | Config |
| :--- | :--- | :--- |
| Local development | Docker Compose (`make up`) | `deploy/docker-compose.yml`, secrets in `deploy/.env` (from `deploy/.env.example`) |
| Local Kubernetes test | k3d | `kubectl apply -k deploy/overlays/dev` |
| Production | K3s on one Azure VM (`Standard_B2ms`), behind Cloudflare at `https://swiftdrop.work` | `kubectl apply -k deploy/overlays/prod` |

Compose is for development only; production runs on Kubernetes. Kubernetes manifests are a Kustomize base (`deploy/kubernetes/kustomization.yaml`) that also pulls in Member 7's gateway CRDs (`deploy/gateway/traefik/kubernetes/`) and the observability stack (`deploy/observability/`). Environment differences are patches in `deploy/overlays/{dev,prod}`.

---

## 2. Production prerequisites (once)

### 2.1 Azure VM
- Azure for Students subscription, VM `Standard_B2ms` (2 vCPU, 8 GB), Ubuntu 24.04, 64 GB Standard SSD, static public IP, SSH key login only.
- Network security group: SSH (22) and the Kubernetes API (6443) from the admin IP only; 80/443 from Cloudflare's IP ranges only (https://www.cloudflare.com/ips/).
- Cost guards: nightly auto-shutdown, a budget alert in Cost Management, and `az vm deallocate` after every session (a stopped-but-allocated VM still costs money).

### 2.2 Cloudflare and TLS
1. Add `swiftdrop.work` to Cloudflare (free plan) and switch the registrar's nameservers to Cloudflare.
2. DNS: `A swiftdrop.work → <VM public IP>`, **Proxied**.
3. SSL/TLS mode: **Full (strict)**.
4. SSL/TLS → Origin Server → create an Origin CA certificate for `swiftdrop.work` and `*.swiftdrop.work`. Keep the certificate and key off git; they become the `traefik-origin-tls` Secret (§3.3).

### 2.3 K3s, Traefik settings and KEDA (on the VM)
```bash
# K3s with its bundled Traefik v3 (do NOT disable it; the IngressRoute CRDs target it)
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--write-kubeconfig-mode 600" sh -
sudo kubectl get nodes

# Copy /etc/rancher/k3s/k3s.yaml to the laptop as ~/.kube/config and replace 127.0.0.1 with the VM IP
# (6443 must be open to the admin IP only), or use an SSH tunnel.

# Traefik: Cloudflare real-IP trust, SSE-safe timeouts, http -> https redirect
kubectl apply -f deploy/kubernetes/traefik/helmchartconfig.yaml

# KEDA (needed by deploy/kubernetes/messaging/keda.yaml). Pin the chart version you tested.
helm repo add kedacore https://kedacore.github.io/charts
helm install keda kedacore/keda -n keda --create-namespace --version <tested version>
```

### 2.4 Images
Images are published to `ghcr.io/phantomtroupeead/swiftdrop/<service>:<7-char git SHA>` by `.github/workflows/release-images.yml`: automatically on every push to `main`, or by hand from GitHub → Actions → **SwiftDrop Publish Images** → Run workflow (any branch). Note the SHA.

---

## 3. Secrets and config (fresh values; never commit them)

### 3.0 Where the production secrets live (GitHub `production` environment)
Production secrets are the **GitHub environment secrets** of the `production` environment (Settings → Environments → production → Secrets). On every deploy, `.github/workflows/deploy.yml` runs `deploy/scripts/sync-secrets.sh`, which writes them into the cluster as Kubernetes Secrets (a changed Secret restarts the pods that read it). Nothing is stored in git or in a file on a laptop. The `production` environment only releases them to the `main` branch after the reviewer approves, and each pod reads only the Secrets its own manifest references.

GitHub shows a secret only once, so **keep a private backup of the values** (password manager or paper). A lost value can still be read back from the cluster with `kubectl -n swiftdrop get secret <name> -o jsonpath='{.data.<KEY>}' | base64 -d` (run locally; never paste the output into chat or tickets).

| GitHub secret (names only) | Used for |
| :--- | :--- |
| `POSTGRES_PASSWORD`, `RABBITMQ_PASSWORD` | Superuser/broker password. **Must equal the value the running instance was first started with**; changing it here does not change the password inside Postgres/RabbitMQ. Letters, digits and `. _ ~ -` only. |
| `MIGRATOR_PASSWORD`, `APP_PASSWORD`, `TRACKING_PASSWORD`, `RELAY_PASSWORD`, `WORKER_PASSWORD`, `RECONCILER_PASSWORD` | One Postgres role per service. The bootstrap Job applies them on every deploy (rotation = change the secret, deploy). Same character rule. |
| `JWT_SECRET` (≥32 chars), `MERCHANT_API_KEY_PEPPER`, `WEBHOOK_SECRET_ENCRYPTION_KEY` (base64, 32 bytes) | core-api (+ tracking, messaging). Changing the webhook key makes already-stored webhook secrets unreadable. |
| `SEED_PASSWORD_ADMIN/DISPATCHER/COURIER/MERCHANT/CUSTOMER` (≥12 chars) | New passwords for the five demo accounts (only applied while an account still has its published default password). The smoke test reuses them. |
| `GRAFANA_ADMIN_PASSWORD` | Grafana login. |
| `GHCR_PULL_TOKEN` + variable `GHCR_USER` | Pulling private images from ghcr.io (`read:packages`). |
| `ORIGIN_CERT_PEM`, `ORIGIN_KEY_PEM` | Cloudflare Origin certificate and key (whole PEM text) for `traefik-origin-tls`. |
| optional: `AI_SERVICE_ROLE_TOKEN`, `LLM_API_KEY`, `SWIFTDROP_API_KEY`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMOKE_PASSWORD_<ROLE>` | ai-assistant / mail / smoke-test overrides. Empty = mock or skipped. |

If a required secret is empty the deploy stops before changing anything and lists the missing names. To rotate a value: edit the GitHub secret, run/approve a deploy.

**First-time or manual alternative: one script creates everything.** It generates strong random values, keeps them consistent (role passwords match their `*_DATABASE_URL`, one webhook key and one JWT secret everywhere), never prints them, leaves existing Secrets unchanged, and saves only the login passwords (demo accounts, Grafana) to `~/swiftdrop-secrets/logins.txt` outside the repo. It asks only for your GitHub token (`read:packages`) and the Cloudflare origin certificate paths.
```powershell
$env:KUBECONFIG = "$HOME\.kube\swiftdrop-azure.yaml"
& "C:\Program Files\Git\bin\bash.exe" deploy/scripts/create-secrets.sh
```
> **Windows:** plain `bash` in PowerShell is **WSL**, which does not see `$env:KUBECONFIG` and talks to a different cluster. Use Git Bash as above. Without bash, create `ghcr-pull` with `kubectl create secret docker-registry ghcr-pull --docker-server=ghcr.io --docker-username=<user> --docker-password=<token> -n swiftdrop`.

The manual alternative (templates) follows. Generate random values (PowerShell example: `[Convert]::ToBase64String((1..48 | % { Get-Random -Max 256 }))`). Use different values from local development.

### 3.1 Namespace
```bash
kubectl apply -f deploy/kubernetes/namespace.yaml
```

### 3.2 Application secrets
Copy the templates **outside the repo**, fill them in, apply, then delete the copies:
```bash
cp deploy/kubernetes/secrets.example.yaml ~/sd-secrets.yaml
cp deploy/kubernetes/messaging/messaging-config.yaml.example ~/sd-messaging.yaml
cp deploy/kubernetes/ai-assistant/secret.yaml.example ~/sd-ai.yaml
# edit all three, then:
kubectl apply -f ~/sd-secrets.yaml -f ~/sd-messaging.yaml -f ~/sd-ai.yaml
shred -u ~/sd-secrets.yaml ~/sd-messaging.yaml ~/sd-ai.yaml
```

| Secret | Keys | Rule |
| :--- | :--- | :--- |
| `postgres-secrets` | `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `MIGRATOR_/APP_/TRACKING_/RELAY_/WORKER_/RECONCILER_PASSWORD`, matching `*_DATABASE_URL` | Each `*_PASSWORD` must equal the password inside its `*_DATABASE_URL`. The bootstrap Job sets the role passwords from these keys. |
| `rabbitmq-secrets` | `RABBITMQ_DEFAULT_USER`, `RABBITMQ_DEFAULT_PASS`, `RABBITMQ_URL` | The URL must contain the same user and password. Defined only in `secrets.example.yaml`. |
| `core-api-secrets` | `JWT_SECRET` (≥ 32 chars), `MERCHANT_API_KEY_PEPPER` (≥ 32 bytes), `WEBHOOK_SECRET_ENCRYPTION_KEY` (base64 of 32 bytes), optional `SEED_PASSWORD_ADMIN/_DISPATCHER/_COURIER/_MERCHANT/_CUSTOMER` (≥ 12 chars) | tracking-api and ai-assistant read `JWT_SECRET` from here too. Production suspends every demo account whose `SEED_PASSWORD_<ROLE>` is not set (their default passwords are public). |
| `integrations-secrets` | `WEBHOOK_SECRET_ENCRYPTION_KEY`, SMTP credentials | Same webhook key as `core-api-secrets`. |
| `outbox-relay-secrets`, `event-worker-secrets` | `DATABASE_URL` | Passwords equal `RELAY_PASSWORD` / `WORKER_PASSWORD`. |
| `ai-assistant-secrets` | `LLM_API_KEY`, `SWIFTDROP_API_KEY`, `AI_SERVICE_ROLE_TOKEN` | Values from Member 10. |
| ConfigMap `messaging-config` | relay/worker settings | From the messaging template (ConfigMap part). |

### 3.3 Cluster-only secrets
```bash
kubectl create secret tls traefik-origin-tls -n swiftdrop --cert=origin.pem --key=origin.key
kubectl create secret docker-registry ghcr-pull -n swiftdrop \
  --docker-server=ghcr.io --docker-username=<github user> --docker-password=<token with read:packages>
kubectl create secret generic grafana-admin -n swiftdrop --from-literal=GF_SECURITY_ADMIN_PASSWORD=<strong password>
```

---

## 4. Deploy

```bash
cd deploy/overlays/prod
# Pin every image to the published SHA (repeat for tracking-api, messaging, web, ai-assistant)
kustomize edit set image ghcr.io/phantomtroupeead/swiftdrop/core-api=ghcr.io/phantomtroupeead/swiftdrop/core-api:<sha>
cd -

kubectl apply -k deploy/overlays/prod
kubectl get pods -n swiftdrop -w
```

What happens, in order (no manual steps):
1. `postgres-0` starts. The Job `postgres-bootstrap-roles` creates the six roles, sets their passwords from `postgres-secrets`, creates `pgcrypto`, and sets default privileges, all in **one transaction**.
2. The Job `core-api-migrate` waits (init container) until `swiftdrop_migrator` can log in, then runs `/app/migrate` (embedded migrations). Migrations grant table rights to the relay, worker and reconciler roles, so they must run after step 1.
3. Apps start and become Ready: core-api (2–4 pods, HPA), tracking-api, outbox-relay (**always 1 replica**), event-worker (1–3, KEDA), ai-assistant, web, Prometheus, Grafana.

**New release:** Jobs are immutable. Delete the migrate Job before applying a new image tag:
```bash
kubectl delete job core-api-migrate -n swiftdrop --ignore-not-found
kubectl apply -k deploy/overlays/prod
```

**Rollback:** `kubectl rollout undo deployment/<name> -n swiftdrop` (migrations are forward-only; roll back code, not schema).

> Never scale `outbox-relay`, `postgres` or `rabbitmq`. Keep `max core-api replicas × 10 connections` within Postgres `max_connections` (150).

---

## 5. Verify

- [ ] `kubectl get pods -n swiftdrop`: every pod Running and Ready; both Jobs Completed.
- [ ] `kubectl get hpa,scaledobject -n swiftdrop`: core-api HPA and the event-worker ScaledObject are active.
- [ ] `kubectl logs -n swiftdrop deploy/outbox-relay`: `connected to rabbitmq and declared topology`.
- [ ] `https://swiftdrop.work/` returns the SPA; `http://` redirects to `https://`.
- [ ] Gateway smoke test (logins, `/auth/me`, role checks, `correlation_id`): `python deploy/scripts/smoke-test.py https://swiftdrop.work`. In production pass the demo passwords as `SMOKE_PASSWORD_<ROLE>` env vars; roles without one are skipped.
- [ ] core-api log shows `Demo accounts secured: rotated=[...] suspended=[...]`, and the published demo passwords no longer log in.
- [ ] Monitoring: `kubectl port-forward -n swiftdrop svc/grafana 3001:3000` → **SwiftDrop Overview**; `kubectl port-forward -n swiftdrop svc/prometheus 9090` → targets UP (core-api, tracking-api, ai-assistant show DOWN until they expose Prometheus `/metrics`).
- [ ] Health endpoints stay internal: check them with `kubectl port-forward`, never through the public domain.

---

## 6. Local Kubernetes test (k3d)

```bash
k3d cluster create swiftdrop -p "80:80@loadbalancer"
docker compose -f deploy/docker-compose.yml build
# tag each local image as ghcr.io/phantomtroupeead/swiftdrop/<name>:dev, then:
k3d image import ghcr.io/phantomtroupeead/swiftdrop/<name>:dev -c swiftdrop
helm install keda kedacore/keda -n keda --create-namespace
# create the secrets from §3 (test values), then:
kubectl apply -k deploy/overlays/dev
```
The dev overlay serves Member 7's IngressRoute over plain HTTP. It still matches `Host()`, so add `127.0.0.1 <production host>` to your hosts file. Do not apply `deploy/kubernetes/traefik/helmchartconfig.yaml` on k3d.

---

## 7. CI/CD

| Workflow | Trigger | Does |
| :--- | :--- | :--- |
| `.github/workflows/ci.yml` | push / PR to `main`, `dev` | Secret scan, Go (core-api, messaging, tracking) lint and tests, ai-assistant pytest, web build, Docker build check, `docker compose config`, kubeconform on `deploy/kubernetes` and `deploy/observability/kubernetes` |
| `.github/workflows/release-images.yml` | push to `main`, manual | Builds and pushes the five images to ghcr, tagged with the 7-char SHA |
| `.github/workflows/e2e.yml` | PR to `main`, manual | Gateway system checks (Member 7) |
| `.github/workflows/deploy.yml` | after "SwiftDrop Publish Images" succeeds on `main`; manual (`sha` input) | Deploys that SHA to the Azure cluster after **approval**; smoke test; automatic rollback |

### 7.1 Automatic deployment (CD)
```
merge dev -> main ──> release-images.yml builds <service>:<sha7> ──> deploy.yml queued on the VM runner
   ──> Minidu approves (GitHub "production" environment) ──> deploy/scripts/deploy.sh <sha7>
   ──> apply prod overlay -> DB Jobs -> rollouts -> smoke test ──> success, or automatic rollback
```
- **Runner on the VM, not on GitHub's machines.** It only makes outbound HTTPS calls to GitHub, so the Kubernetes API stays closed to the internet. While the VM is deallocated the job waits (up to 24 h) and runs when the VM starts.
- **Least privilege.** The runner uses ServiceAccount `cd-deployer` (`deploy/cd/deployer-rbac.yaml`): full control of namespace `swiftdrop`, nothing else (no `kube-system`, no other namespaces, no nodes).
- **Approval.** Every production deploy waits for a required reviewer in the `production` environment.
- **Rollback.** If a DB Job or a rollout fails, or the smoke test fails, every app Deployment is rolled back with `kubectl rollout undo`. Migrations are forward-only: the schema is not rolled back.
- **Manual run.** Actions → *Deploy to Azure* → Run workflow → `sha` = a published image tag. Or from a laptop with an admin kubeconfig: `bash deploy/scripts/deploy.sh <sha7> [smoke_url|none]` (`DRY_RUN=1` for a server-side dry run).

### 7.2 One-time CD setup
1. `kubectl apply -f deploy/cd/deployer-rbac.yaml` (admin kubeconfig).
2. GitHub → Settings → Actions → Runners → *New self-hosted runner* (Linux x64): copy the registration token.
3. On the VM: `bash install-runner.sh` (copied from `deploy/cd/install-runner.sh`), paste the token when asked. The runner `swiftdrop-azure` must show **Idle**.
4. GitHub → Settings → Environments → `production`: required reviewer = Minidu; deployment branches = `main`; variable `SMOKE_URL` (`none` until the site is public, then `https://swiftdrop.work`); secrets `SMOKE_PASSWORD_<ROLE>` = the `SEED_PASSWORD_<ROLE>` values (optional).
5. GitHub → Settings → Actions → General: fork pull request workflows → *Require approval for all outside collaborators*.
