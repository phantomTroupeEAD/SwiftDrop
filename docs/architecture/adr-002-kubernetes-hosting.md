# ADR-002: Kubernetes & Hosting Architecture

> **Status:** Accepted  
> **Date:** 2026-10-21 (last updated 2026-10-28: workload table and autoscaling aligned with the Member 9 team agreements)  
> **Owners:** Member 9 (DevOps, Kubernetes & Observability), Member 7 (Admin Frontend & Gateway)  
> **Deciders:** SwiftDrop Engineering Team  
> **References:** [SRS 11.2, 13.2 & 15.3](../../docs/intial/SwiftDrop_SRS_v1.md), [Engineering Standards §10](../../docs/intial/SwiftDrop_Engineering_Standards_and_Best_Practices_v1.0.md), [ADR-001 (Traefik Ingress)](adr-001-traefik-ingress.md)

---

## 1. Context & Problem Statement

SwiftDrop is a comprehensive parcel delivery management system comprising 10 distinct domain capabilities. To demonstrate production readiness, scalability, and meet the deployment requirements specified in the project requirements ([SRS §11.2 & §15.3](../intial/SwiftDrop_SRS_v1.md)), the engineering team requires an authoritative hosting architecture and deployment plan.

Prior documentation contained ambiguities and gaps:
1. Docker Compose was established for local development, but Kubernetes workload mapping and hosting boundaries lacked a binding architectural specification.
2. Section 4 of preliminary hosting notes listed only `Deployments` for all services, violating the SRS §11.2 requirement to distinguish stateful databases (`StatefulSets`) and periodic jobs (`CronJobs`).
3. Cloudflare was selected as edge DNS/proxy, but the configuration required to restore client IPs (`forwardedHeaders.trustedIPs`), protect Traefik rate limiting, and support Server-Sent Events (SSE) was undocumented.
4. K3s ships with a bundled Traefik instance by default, creating a conflict with custom Traefik v3.x deployments unless explicitly configured.

This ADR formalizes the baseline hosting environment, Kubernetes distribution, edge integration, resource mapping, and security posture for SwiftDrop.

---

## 2. Decision: Hosting Environment

We adopt a **single Ubuntu VPS (Virtual Private Server) running K3s** as the primary deployment target, paired with **Docker Compose** for local developer workflows:

1. **Host OS:** Ubuntu 22.04 / 24.04 LTS with a dedicated public IPv4 address.
2. **Kubernetes Distribution:** **K3s** (Rancher/SUSE lightweight Kubernetes).
   - *Rationale:* K3s is fully CNCF-certified, packages control-plane and worker components into a single binary, consumes minimal memory overhead (<512 MB RAM), and includes a built-in `local-path` PersistentVolume provisioner suitable for single-node stateful storage.
3. **Local Development Parity:** Developers run the system via Docker Compose (`deploy/docker-compose.yml`). The Kubernetes manifests in `deploy/kubernetes/` mirror the container images, environment variable schemas, and networking routes used in Compose.
4. **Provider and size** (⏳ open; the team left this choice to Member 9, who decides after trying Azure; status 2026-10-28):
   - **Size rule:** at least **8 GB RAM**, x86. K3s, PostgreSQL (1 GiB limit), RabbitMQ (512 MiB limit), six app workloads, Traefik and the monitoring stack do not fit in 4 GB.
   - **Trying first: Azure for Students,** VM size B2ms (2 vCPU, 8 GB RAM). It uses the $100 student credit and needs no card. Deallocate the VM whenever it is not in use, so the credit lasts.
   - **Paid backup: Hetzner Cloud CX33.** x86, 4 vCPU, 8 GB RAM, 80 GB disk, in Nuremberg, Falkenstein or Helsinki. About €8.49/month plus €0.50 for the IPv4 address (Hetzner price list of 15 June 2026, excluding VAT). Billing is hourly, so it would be rented only from milestone M4. Hetzner had limited CX capacity in 2026.
   - **Practice only: Oracle Cloud trial** ($300 credit, 30 days). Good for a practice K3s cluster on an x86 VM now, not for the demo: the credit and the servers made with it end after 30 days.
   - **Fallback:** a local k3d cluster on the Member 9 laptop (about 16 GB RAM) for the demo, if no server is available.
   - **Rejected:** Hetzner CPX31/CPX32 (about €35/month or more after the June 2026 price rise); DigitalOcean (the student credit ended in August 2026); Oracle Always Free (ARM only, cut to 2 OCPU / 12 GB in 2026 and often out of capacity).

---

## 3. Decision: Edge Ingress & External Access Architecture

External traffic flows through an integrated edge stack:

```
Internet (Browsers, Mobile Couriers, Merchant API Clients)
       │
       ▼
Cloudflare DNS & Edge Proxy (WAF, DDoS Shield, Edge Caching, TLS Termination)
       │ (Encrypted HTTPS — Port 443)
       ▼
Ubuntu VPS Firewall (UFW: Ports 80 & 443 restricted to Cloudflare CIDR blocks only)
       │
       ▼
Traefik Ingress Controller (HostPort/NodePort 80/443, Real IP Restoration)
       ├── IngressRoute: api.swiftdrop.example.com     ──► Core API ClusterIP (:8080)
       ├── IngressRoute: tracking.swiftdrop.example.com──► Tracking API ClusterIP (:8081)
       ├── IngressRoute: ai.swiftdrop.example.com       ──► AI Assistant ClusterIP (:8000)
       └── IngressRoute: portal.swiftdrop.example.com   ──► Web SPA ClusterIP (:80)
```

### 3.1 Network Protection & Firewall Rules
To prevent attackers from bypassing Cloudflare WAF and DDoS mitigation by connecting directly to the VPS IP, the host firewall (`ufw` or `iptables`) restricts inbound traffic on ports `80` and `443` strictly to Cloudflare's published IPv4 and IPv6 CIDR ranges. All other inbound ports are closed, with SSH (port `22`) restricted to key-based authentication.

### 3.2 TLS Configuration
- **Cloudflare Edge:** Strict TLS mode terminating public HTTPS with Cloudflare Universal SSL.
- **Origin Connection:** **Cloudflare Full (Strict)** mode. A Cloudflare Origin CA certificate (valid for 15 years) is issued and mounted as a Kubernetes Secret (`traefik-origin-tls`) in the cluster, ensuring authenticated end-to-end encryption between Cloudflare edge and Traefik.

### 3.3 Evaluation of Cloudflare Tunnel (`cloudflared`)
Cloudflare Tunnel was evaluated as an alternative to eliminate open inbound ports entirely. However, the direct VPS inbound model with UFW Cloudflare IP filtering is adopted as the primary baseline for simplicity, lower latency overhead, and predictable HTTP/2 unbuffered SSE streaming. Cloudflare Tunnel remains an approved drop-in alternative if VPS IP exposure needs to be eliminated entirely.

---

## 4. Decision: Kubernetes Resource Mapping

Workloads must be mapped to appropriate Kubernetes workload primitives according to state and lifecycle characteristics:

| Workload | K8s Resource Type | Replicas | Storage / Lifecycle |
| :--- | :--- | :--- | :--- |
| **Core API** | `Deployment` | 2 (baseline) | Stateless Go monolith. Reads/writes to PostgreSQL and writes to transactional Outbox. |
| **Tracking API** | `Deployment` | 2 (baseline) | Stateless read-optimized SSE projection service. Connects to PostgreSQL read replica/pool. |
| **AI Assistant** | `Deployment` | 1 | Stateless Python/FastAPI service calling Core API internal endpoints. |
| **Web Frontend** | `Deployment` | 2 (baseline) | Nginx container serving React SPA static bundle. |
| **Outbox Relay** | `Deployment` | 1 (singleton) | Dedicated polling daemon reading unpublished events from the `outbox_events` table and publishing to RabbitMQ. Singleton ensures no concurrent lock contention on outbox batches. Never autoscaled. |
| **Event Worker** | `Deployment` | 1–3 (KEDA) | Asynchronous consumer of `q.notifications.worker` and `q.webhooks.dispatcher` (notifications and merchant webhooks). Must be idempotent. |
| **PostgreSQL 16** | `StatefulSet` | 1 | Authoritative database (`max_connections=150`, memory limit 1Gi). Uses `volumeClaimTemplates` with `local-path` StorageClass for durable disk persistence. |
| **RabbitMQ 3.13** | `StatefulSet` | 1 | Event message broker (transport only, never the source of truth). Persistent queues backed by PVC using `local-path` StorageClass. |
| **Redis** *(optional)* | `Deployment` | 1 | Pub/Sub fan-out only, needed only if Tracking API runs more than 1 replica (open item C8, Member 5). No persistence. |
| **Reconciliation Worker** | `CronJob` | Schedule: `*/10 * * * *` | `messaging` image, `/usr/local/bin/reconcile --stale-threshold=5m --batch-size=50` (Member 8). `concurrencyPolicy: Forbid`. Sweeps stale outbox rows and stuck deliveries. |
| **Traefik Ingress** | `Deployment` | 1–2 | Ingress Controller listening on HostPort/NodePort 80/443. Reads `IngressRoute` and `Middleware` CRDs. |

---

## 5. Cloudflare & Traefik Integration Guarantees

### 5.1 Real Client IP Restoration (`trustedIPs`)
Because all inbound traffic traverses Cloudflare edge servers, Traefik natively sees the Cloudflare proxy IP as `RemoteAddr`. To prevent all users from sharing a single IP rate-limiting bucket (which would trigger catastrophic `429 Too Many Requests` cascading errors):
1. Traefik's `entryPoints.web.forwardedHeaders.trustedIPs` and `entryPoints.websecure.forwardedHeaders.trustedIPs` are populated with Cloudflare's published IP ranges.
2. Traefik extracts the real client IP from `CF-Connecting-IP` / `X-Forwarded-For` and passes it to backend services and in-memory rate limiters.

### 5.2 Server-Sent Events (SSE) Proxy Compatibility
Cloudflare proxies buffer HTTP responses by default, which breaks real-time event streaming. To guarantee seamless SSE delivery for Tracking API (`services/tracking`):
1. Tracking API must emit response header `X-Accel-Buffering: no` on every SSE stream under `/api/v1/tracking/` (the exact stream path is open item C18, owned by Member 5).
2. Tracking API must send periodic heartbeat/keepalive comments (e.g. `: ping\n\n` every 15 seconds) to prevent Cloudflare's 100-second idle connection timeout from closing active browser sessions.
3. Cloudflare dashboard setting for WebSockets/HTTP2 is enabled.

---

## 6. K3s Deployment & Traefik Controller Strategy

By default, K3s installs an uncustomized Traefik v2 instance via a built-in HelmChart controller. To ensure SwiftDrop maintains complete control over Traefik v3.x configuration and CRDs:

**Decision:** Install K3s with `--disable traefik`:
```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik" sh -
```
Traefik is then deployed declaratively into namespace `traefik` using official Helm charts or customized manifests in `deploy/gateway/traefik/`. This guarantees:
- Predictable version pinning to Traefik v3.x.
- Clean separation between Kubernetes system components and SwiftDrop application ingress.
- Full access to Traefik Custom Resource Definitions (`IngressRoute`, `Middleware`).

---

## 7. Autoscaling Strategy (HPA & KEDA)

On a single Ubuntu VPS, compute and memory resources are constrained by the physical host. Uncontrolled horizontal autoscaling risks CPU throttling and OOM kills of critical stateful workloads (PostgreSQL).

1. **Baseline Strategy:** Static replica counts with conservative resource requests and limits:
   - Core API: `requests.cpu: 100m`, `limits.cpu: 500m`, `requests.memory: 128Mi`, `limits.memory: 512Mi`
   - Tracking API: `requests.cpu: 50m`, `limits.cpu: 250m`, `requests.memory: 64Mi`, `limits.memory: 256Mi`
   - PostgreSQL: `requests.cpu: 100m`, `limits.cpu: 500m`, `requests.memory: 256Mi`, `limits.memory: 1Gi` (as in `deploy/kubernetes/postgres-statefulset.yml`)
2. **Demonstration Autoscaling (Bonus Requirement):**
   - **HorizontalPodAutoscaler (HPA):** Core API 2–4 replicas at 70% CPU. Tracking API: the manifest today scales 1–2 at 50% CPU; the final range is open item C8 (Member 5), and the DB connection budget allows at most 4 pods.
   - **KEDA (Kubernetes Event-driven Autoscaling):** `ScaledObject` for the `event-worker` with two RabbitMQ triggers, `q.notifications.worker` and `q.webhooks.dispatcher` (`queueLength: 10` each), scaling workers from 1 to 3 pods. The KEDA trigger reads the `RABBITMQ_URL` key of `rabbitmq-secrets`.
   - **Never autoscaled:** `outbox-relay` (singleton), PostgreSQL and RabbitMQ (StatefulSets).

---

## 8. Security & Operational Hardening

1. **Dashboard Security:** The Traefik API/dashboard is configured with `api.insecure: false` in production manifests. Administrative access is restricted to authenticated `kubectl port-forward` sessions.
2. **Kubernetes RBAC:** Services execute under dedicated Kubernetes `ServiceAccount`s with default permissions revoked. Pods run as non-root users with read-only root filesystems where practical.
3. **Secret Hygiene:** Database passwords, JWT secret keys, and RabbitMQ credentials are stored in Kubernetes `Secret` resources and injected as environment variables. No plain secrets are stored in Git.
4. **Network Policies:** In Kubernetes, egress from Core API to external networks is restricted, and direct access to PostgreSQL and RabbitMQ is permitted only from authorized application pods (`core-api`, `outbox-relay`, `event-worker`, `tracking-api`) and the migration Job and reconciler CronJob (PostgreSQL only).

---

## 9. Single-VPS Trade-offs & Disaster Recovery

Operating on a single VPS represents an acknowledged single point of failure (SPOF) for node-level availability:

| Risk | Impact | Mitigation Strategy |
| :--- | :--- | :--- |
| **VPS Hardware / Hypervisor Crash** | Temporary system downtime until VPS reboots. | K3s service and container runtimes configured with `Restart=always`. All pods automatically restart upon node recovery. |
| **Host Disk Failure** | Potential loss of PostgreSQL and RabbitMQ data. | Automated cron script exports encrypted PostgreSQL dumps (`pg_dump`) to remote off-site object storage (e.g. AWS S3 / Cloudflare R2) every 6 hours. |
| **Storage Provisioning** | `local-path` volumes bind to the host filesystem (`/var/lib/rancher/k3s/storage`). | Volume directories are placed on the root or attached block volume with disk usage monitoring and alerting at 80% capacity. |

---

## 10. Outstanding Implementation Checklist

- [x] Author ADR-002 defining Kubernetes & Hosting Architecture.
- [ ] Configure Traefik static configuration with Cloudflare trusted IP ranges (`deploy/gateway/traefik/traefik.yml`).
- [ ] Add production safety annotations to Traefik dashboard configuration.
- [ ] Update Engineering Standards §10 to specify StatefulSets, CronJobs, Cloudflare origin protection, and K3s storage.
- [ ] Update `deploy/gateway/README.md` to document Cloudflare proxy integration and Kubernetes deployment flow.
- [ ] Update ADR-001 verification checklist with Cloudflare real-IP and SSE tests.
- [ ] Update `docs/architecture/domain-boundaries.md` topology diagram to include Cloudflare proxy.
- [ ] Generate baseline Kubernetes manifests (`deploy/kubernetes/`, per-service folders, A8) covering Deployments, StatefulSets, CronJob, and Traefik IngressRoutes.
