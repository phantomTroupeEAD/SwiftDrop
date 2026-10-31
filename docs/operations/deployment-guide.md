# SwiftDrop — Production Deployment Guide (Docker Compose & Kubernetes)

> **Authority:** Fulfills SRS §11.1–11.2 (Deployment & Runtime Architecture) and ADR-002 (Ubuntu VPS Kubernetes Hosting).  
> **Primary Owner:** Member 9 (DevOps, Kubernetes & Observability)  
> **Collaborator:** Member 7 (Admin Frontend & API Gateway)  
> **Last updated:** 2026-10-28 (paths follow the per-service manifest layout A8; names follow the Member 9 team agreements)

---

## 1. Deployment Topology Overview

SwiftDrop supports two production deployment paths:
1. **Docker Compose Mode:** Lightweight single-host orchestration for quick staging, testing, or minimal VPS footprints.
2. **Kubernetes (K3s) Mode:** Enterprise production runtime on an Ubuntu VPS, featuring declarative resource limits, automated restarts, unified Traefik Ingress routing, and future multi-node scale-out capability.

---

## 2. Path A: Production Deployment via Docker Compose

### Step 1: VPS Host Preparation
```bash
# Update and install Docker + Compose
sudo apt-get update && sudo apt-get install -y docker.io docker-compose-plugin
sudo usermod -aG docker $USER
```

### Step 2: Clone & Configure Secrets
```bash
git clone https://github.com/phantomTroupeEAD/SwiftDrop.git /opt/swiftdrop
cd /opt/swiftdrop

# Generate production environment file
cp .env.example .env
nano .env  # Populate production DB passwords, JWT RSA keys, and third-party secrets
```

### Step 3: Launch Production Stack
```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml up -d
```

### Step 4: Run Migrations
The one-shot `core-api-migrate` service already runs before `core-api` starts. To run it again by hand:
```bash
docker compose -f deploy/docker-compose.yml run --rm core-api-migrate
```

---

## 3. Path B: Kubernetes (K3s) Deployment on Ubuntu VPS

### Step 1: Install K3s (Disabling Default Traefik v2)
In accordance with **ADR-001** and **ADR-002**, install lightweight K3s while disabling the packaged legacy Traefik to deploy Traefik v3 via Helm:
```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik --write-kubeconfig-mode 644" sh -

# Verify cluster status
kubectl get nodes
```

### Step 2: Create Namespaces
```bash
kubectl create namespace traefik
kubectl create namespace swiftdrop
kubectl create namespace monitoring
```

### Step 3: Deploy Traefik v3 Ingress Controller
```bash
helm repo add traefik https://traefik.github.io/charts
helm repo update

helm install traefik traefik/traefik \
  --namespace traefik \
  -f deploy/gateway/traefik/values.yaml
```

### Step 4: Configure Cloudflare Origin TLS Certificate
Create the Kubernetes TLS secret using the 15-year Cloudflare Origin CA certificate. The name `traefik-origin-tls` is agreed with Member 7; its final namespace is open item C5.
```bash
kubectl create secret tls traefik-origin-tls \
  --cert=/etc/ssl/swiftdrop/origin.crt \
  --key=/etc/ssl/swiftdrop/origin.key \
  --namespace swiftdrop
```

The paths below follow the per-service layout (A8, `deploy/README.md` §2). Until that move is done, the StatefulSets are the flat files `deploy/kubernetes/postgres-statefulset.yml` and `rabbitmq-statefulset.yml`. Manifests write image tags as `${IMAGE_TAG}`, so apply them through `envsubst` with the git SHA:
```bash
export IMAGE_TAG=<git-sha>
```

### Step 5: Provision Secrets & ConfigMaps
The repo holds placeholders only (`secrets.example.yaml`). Real values never go into git.
```bash
# Fill a private copy OUTSIDE the repo, apply it, then delete it
cp deploy/kubernetes/secrets.example.yaml ~/swiftdrop-secrets.yaml
nano ~/swiftdrop-secrets.yaml      # postgres-secrets (one DB URL key per role), rabbitmq-secrets, jwt-secrets, ...
kubectl apply -f ~/swiftdrop-secrets.yaml -n swiftdrop && shred -u ~/swiftdrop-secrets.yaml

# Non-secret config: one ConfigMap per service
find deploy/kubernetes -name configmap.yaml -exec kubectl apply -n swiftdrop -f {} \;
```

### Step 6: Deploy Authoritative Persistence
```bash
kubectl apply -f deploy/kubernetes/postgres/ -n swiftdrop
kubectl apply -f deploy/kubernetes/rabbitmq/ -n swiftdrop

# Wait for persistence to become ready
kubectl wait --for=condition=ready pod -l app=postgres -n swiftdrop --timeout=120s
kubectl wait --for=condition=ready pod -l app=rabbitmq -n swiftdrop --timeout=120s
```

### Step 7: Create DB Roles (first install only) and Run the Migration Job
```bash
# D6 roles; passwords come from your shell, never from the SQL file
kubectl exec -i postgres-0 -n swiftdrop -- psql -U swiftdrop -d swiftdrop_db \
  -v migrator_pw="$MIGRATOR_PW" -v app_pw="$APP_PW" -v tracking_pw="$TRACKING_PW" \
  -v relay_pw="$RELAY_PW" -v worker_pw="$WORKER_PW" -v reconciler_pw="$RECONCILER_PW" \
  -f - < deploy/postgres/bootstrap-roles.sql

# One Job per release, backoffLimit: 0 (D1)
envsubst < deploy/kubernetes/core-api/migrate-job.yaml | kubectl apply -n swiftdrop -f -
kubectl wait --for=condition=complete job/core-api-migrate-job -n swiftdrop --timeout=120s
```

### Step 8: Deploy Application Workloads and Autoscaling
```bash
# KEDA CRDs must exist before the event-worker ScaledObject
helm repo add kedacore https://kedacore.github.io/charts
helm install keda kedacore/keda -n keda --create-namespace

for dir in core-api tracking-api messaging ai-assistant web; do
  find deploy/kubernetes/$dir -name "*.yaml" ! -name migrate-job.yaml \
    -exec sh -c 'envsubst < "$1" | kubectl apply -n swiftdrop -f -' _ {} \;
done
```

### Step 9: Apply Ingress Routing & Middleware Policies
```bash
kubectl apply -f deploy/gateway/traefik/middlewares.yaml -n swiftdrop
kubectl apply -f deploy/gateway/traefik/ingressroutes.yaml -n swiftdrop
```

---

## 4. Post-Deployment Verification Checklist

Verify system health across all layers:

- [ ] **Nodes & Pods Healthy:**
  ```bash
  kubectl get pods -n swiftdrop
  # Expected: All pods in Running status, 0 restarts
  ```
- [ ] **Ingress Routes Active:**
  ```bash
  kubectl get ingressroutes -n swiftdrop
  ```
- [ ] **Health Probes Passing:**
  ```bash
  # Health is internal only (not routed by Traefik). Distroless images have no wget/curl, so port-forward:
  kubectl port-forward -n swiftdrop deployment/core-api 8080:8080 &
  curl -s http://localhost:8080/health/live
  # Expected: HTTP 200 OK {"status":"UP"}
  ```
- [ ] **Public Tracking Milestone Reachable** (one domain, path routing C17; domain still a placeholder):
  ```bash
  curl -k -I https://swiftdrop.example.com/api/v1/tracking/public/SD-TEST
  # Expected: HTTP 200 or 404 (not 502 Bad Gateway)
  ```
- [ ] **Outbox Relay Publishing:**
  ```bash
  kubectl logs -n swiftdrop -l app=outbox-relay --tail=20
  # Expected: "published outbox batch" or "waiting for new events"
  ```
- [ ] **Web SPA Asset Serving:**
  ```bash
  curl -k -I https://swiftdrop.example.com/
  # Expected: HTTP 200 OK, Content-Type: text/html
  ```

---

## 5. Automated CI/CD Pipeline Flow

When commits land on `main`:
1. **GitHub Actions (`.github/workflows/ci.yml`, exists today):**
   - Secret scan (TruffleHog) on every push and PR.
   - `gofmt`, `golangci-lint` and `go test -race` for `core-api` (they run once `core-api/go.mod` exists).
   - Planned (C25): web lint and tests, Python tests, infra checks, E2E (`pytest tests/e2e/ -v`).
2. **Container Build & Push (`.github/workflows/deploy.yml`, planned):**
   - Builds x86 (`linux/amd64`) Docker images tagged with the git SHA.
   - Pushes images to GitHub Container Registry (`ghcr.io/phantomtroupeead/swiftdrop/*`).
3. **Cluster Rollout:**
   - Authenticates to VPS via SSH / Kubeconfig.
   - Updates image tags on deployments (`kubectl set image deployment/core-api ...`).
   - Verifies rollout completion (`kubectl rollout status deployment/core-api`).
