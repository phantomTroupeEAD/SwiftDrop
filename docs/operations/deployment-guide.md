# SwiftDrop — Production Deployment Guide (Docker Compose & Kubernetes)

> **Authority:** Fulfills SRS §11.1–11.2 (Deployment & Runtime Architecture) and ADR-002 (Ubuntu VPS Kubernetes Hosting).  
> **Primary Owner:** Member 9 (DevOps, Kubernetes & Observability)  
> **Collaborator:** Member 7 (Admin Frontend & API Gateway)

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
git clone https://github.com/Prashani-beep/SwiftDrop.git /opt/swiftdrop
cd /opt/swiftdrop

# Generate production environment file
cp .env.example .env
nano .env  # Populate production DB passwords, JWT RSA keys, and third-party secrets
```

### Step 3: Launch Production Stack
```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

### Step 4: Run Migrations
```bash
docker compose run --rm core-api-migrate
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
Create the Kubernetes TLS secret using the 15-year Cloudflare Origin CA certificate:
```bash
kubectl create secret tls cloudflare-origin-cert \
  --cert=/etc/ssl/swiftdrop/origin.crt \
  --key=/etc/ssl/swiftdrop/origin.key \
  --namespace swiftdrop
```

### Step 5: Provision Secrets & ConfigMaps
```bash
# Provision database credentials and JWT keys
kubectl apply -f deploy/kubernetes/base/secrets.yaml -n swiftdrop
kubectl apply -f deploy/kubernetes/base/configmaps.yaml -n swiftdrop
```

### Step 6: Deploy Authoritative Persistence
```bash
kubectl apply -f deploy/kubernetes/base/postgres-statefulset.yaml -n swiftdrop
kubectl apply -f deploy/kubernetes/base/rabbitmq-statefulset.yaml -n swiftdrop

# Wait for persistence to become ready
kubectl wait --for=condition=ready pod -l app=postgres -n swiftdrop --timeout=120s
kubectl wait --for=condition=ready pod -l app=rabbitmq -n swiftdrop --timeout=120s
```

### Step 7: Run Database Migrations Job
```bash
kubectl apply -f deploy/kubernetes/base/migrate-job.yaml -n swiftdrop
kubectl wait --for=condition=complete job/core-api-migrate -n swiftdrop --timeout=60s
```

### Step 8: Deploy Application Microservices
```bash
kubectl apply -f deploy/kubernetes/base/core-api-deployment.yaml -n swiftdrop
kubectl apply -f deploy/kubernetes/base/tracking-deployment.yaml -n swiftdrop
kubectl apply -f deploy/kubernetes/base/outbox-relay-deployment.yaml -n swiftdrop
kubectl apply -f deploy/kubernetes/base/event-worker-deployment.yaml -n swiftdrop
kubectl apply -f deploy/kubernetes/base/ai-assistant-deployment.yaml -n swiftdrop
kubectl apply -f deploy/kubernetes/base/web-frontend-deployment.yaml -n swiftdrop
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
  # Verify internal health probes (internal only, not routed by external ingress):
  kubectl exec -n swiftdrop deployment/core-api -- wget -qO- http://localhost:8080/health/live
  # Expected: HTTP 200 OK {"status":"UP"}
  ```
- [ ] **Public Tracking Milestone Reachable:**
  ```bash
  curl -k -I https://api.swiftdrop.io/api/v1/tracking/public/SD-TEST
  # Expected: HTTP 200 or 404 (not 502 Bad Gateway)
  ```
- [ ] **Outbox Relay Publishing:**
  ```bash
  kubectl logs -n swiftdrop -l app=outbox-relay --tail=20
  # Expected: "published outbox batch" or "waiting for new events"
  ```
- [ ] **Web SPA Asset Serving:**
  ```bash
  curl -k -I https://swiftdrop.io/
  # Expected: HTTP 200 OK, Content-Type: text/html
  ```

---

## 5. Automated CI/CD Pipeline Flow

When commits land on `main`:
1. **GitHub Actions (`.github/workflows/ci.yml`):**
   - Runs `golangci-lint` and `npm run lint`.
   - Executes unit tests across Go and React (`go test -v ./...`).
2. **Container Build & Push (`.github/workflows/deploy.yml`):**
   - Builds multi-arch Docker images tagged with git SHA.
   - Pushes images to GitHub Container Registry (`ghcr.io/prashani-beep/swiftdrop/*`).
3. **Cluster Rollout:**
   - Authenticates to VPS via SSH / Kubeconfig.
   - Updates image tags on deployments (`kubectl set image deployment/core-api ...`).
   - Verifies rollout completion (`kubectl rollout status deployment/core-api`).
