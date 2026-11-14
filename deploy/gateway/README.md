# SwiftDrop API Gateway & Ingress Tier (`deploy/gateway`)

> **Primary Owners:** Member 7 (Routing & Middleware Policies), Member 9 (Kubernetes Ingress & Helm)  
> **Technology:** Traefik v3.x  
> **Architecture Decision Records:**  
> - [ADR-001: Adoption of Traefik as Ingress Controller](../../docs/architecture/adr-001-traefik-ingress.md)  
> - [ADR-002: Kubernetes & Hosting Architecture](../../docs/architecture/adr-002-kubernetes-hosting.md)

---

## 1. Architectural Role & Boundary

The SwiftDrop Gateway is a **lean edge proxy and Ingress controller**. Fronted by Cloudflare DNS and edge proxy, it provides a single hardened network entrypoint, terminating TLS, restoring real client IPs, enforcing edge rate limits and CORS, and dispatching incoming HTTP traffic to backend cluster services.

```
Incoming Request (Internet: Browsers, Couriers, Merchant API Clients)
       │
       ▼
┌────────────────────────────────────────────────────────┐
│ Cloudflare DNS & Edge Proxy (WAF, DDoS Shield, SSL)   │
└──────────────────────────┬─────────────────────────────┘
                           │ (HTTPS Port 443)
                           ▼
┌────────────────────────────────────────────────────────┐
│ Ubuntu VPS Firewall (UFW: Ports 80 & 443 Cloudflare)  │
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────┐
│ Traefik Ingress Controller / API Gateway               │
│  ├── Real Client IP Restoration (`trustedIPs`)         │
│  ├── TLS Termination & Ingress Routing                 │
│  ├── Strip identity headers, security headers, CORS    │
│  ├── Per-visitor rate limits & open-stream caps        │
│  └── Body limits (1 MB JSON, 10 MB proof photo)        │
└──────┬───────────────────┬───────────────────┬─────────┘
       │                   │                   │
       ▼                   ▼                   ▼
┌──────────────┐   ┌──────────────┐   ┌─────────────────┐
│ Core API     │   │ Tracking API │   │ AI Assistant    │
│ (:8080)      │   │ (:8081)      │   │ (:8000)         │
│ • JWT / RBAC │   │ • SSE Stream │   │ • MCP Sandbox   │
│ • API Keys   │   │ • Sanitized  │   │                 │
│ • Mutex / DB │   │   Milestones │   │                 │
└──────────────┘   └──────────────┘   └─────────────────┘
```

---

## 2. Responsibilities Split

| Responsibility | Handled By | Implementation Details |
| :--- | :--- | :--- |
| **Path Routing** | Traefik Router | Matches `PathPrefix` rules and forwards to internal service endpoints (`/api/v1/*`, `/portal/*`). |
| **Real Client IP Restoration** | Traefik Entrypoints | Configured via `forwardedHeaders.trustedIPs` matching Cloudflare's published CIDRs. Restores real visitor IP from `CF-Connecting-IP` / `X-Forwarded-For`. |
| **CORS Policy** | Traefik Middleware | `cors-middleware`, exactly as `docs/security/security-notes.md` §4. Allowed headers: `Authorization`, `Content-Type`, `X-API-Key`, `Idempotency-Key`, `X-Request-ID`. Exposed: `X-Request-ID`, `Content-Length`. `X-Tenant-ID` is **not** allowed: the tenant comes from the token. Local origins `http://localhost:3000`, `http://127.0.0.1:3000`; production origins in `kubernetes/middlewares.yaml`. |
| **Identity Header Stripping** | Traefik Middleware | `strip-identity-headers` removes client-sent `X-User-ID`, `X-User-Role`, `X-Tenant-ID`, `X-Merchant-ID`, `X-Forwarded-User` before any backend (defence in depth). `Authorization` passes unchanged. |
| **Response Hardening** | Traefik Middleware | `security-headers`: `nosniff`, `X-Frame-Options: DENY`, strict referrer policy, HSTS (HTTPS only), removes `Server` and `X-Powered-By`. `spa-csp` adds a Content-Security-Policy with `frame-ancestors 'none'` on the SPA. |
| **Request Size Limits** | Traefik Middleware | `json-limit` 1 MB on every API route (SEC-006); `upload-limit-middleware` 10 MB only on `POST /api/v1/deliveries/{id}/proof-photo`. Stream routes are never buffered. |
| **Edge Rate Limiting** | Traefik Middleware | Per visitor IP (right-most `X-Forwarded-For`, depth 1): tracking 30/min burst 10; login 5/min burst 3; AI and MCP 10/min burst 2; merchant API 120/min burst 20. SSE streams also capped at 5 open per visitor (`stream-inflight`). |
| **Dependency Timeouts** | Traefik ServersTransport | `backend-transport` 30 s to first response header, `stream-transport` 10 s, `ai-transport` 60 s; dial 5 s. No retry middleware: the gateway never resends a write. |
| **JWT & Merchant API Key Auth** | **Core API** | Handled natively by Go HTTP middleware in `core-api/internal/identity/` and `core-api/internal/merchants/`. Traefik forwards `Authorization` and `X-API-Key` without modifying them. |
| **SSE Streaming Compatibility** | Traefik + Tracking API | Unbuffered streaming; entry point `writeTimeout: 0s`. Tracking API emits `X-Accel-Buffering: no` and periodic `: ping` heartbeats (every 15s) to prevent Cloudflare edge proxy connection drops. |

### Routes

| Priority | Path | Backend | Middlewares (after strip-identity, security-headers, CORS) |
| :-- | :-- | :-- | :-- |
| 100 | `^/api/v1/tracking/(public/)?{id}/stream$` | `tracking-api:8081` | tracking rate limit, stream cap |
| 90 | `/api/v1/tracking`, `/api/v1/tracking/*` | `tracking-api:8081` | tracking rate limit |
| 80 | `/api/v1/ai`, `/api/v1/ai/*` | `ai-assistant:8000` | AI rate limit, 1 MB |
| 75 | `/mcp`, `/mcp/*` | `ai-assistant:8002` | AI rate limit, stream cap |
| 70 | `/api/v1/auth/login` | `core-api:8080` | login rate limit, 1 MB |
| 60 | `^/api/v1/deliveries/{id}/proof-photo$` | `core-api:8080` | 10 MB |
| 55 | `/api/v1/merchant/*` | `core-api:8080` | merchant rate limit, 1 MB |
| 50 | `/api/v1/*` | `core-api:8080` | 1 MB |
| 2 | `/api/*` (unknown versions answer 404 JSON) | `core-api:8080` | 1 MB, no CORS |
| 1 | everything else | `web:3000` | security headers and SPA CSP only (no identity strip, no CORS) |

The file provider ranks routers without a `priority` by rule length; the Kubernetes file sets every priority explicitly in the same order.

---

## 3. Directory Layout

```
deploy/gateway/
├── README.md                            # Living gateway architecture documentation
└── traefik/
    ├── traefik.yml                      # Static config: entry points, timeouts, trustedIPs, internal dashboard
    ├── dynamic-routes.yml               # File provider (Docker Compose): routers, services, middlewares, transports
    └── kubernetes/
        ├── ingressroutes.yaml           # IngressRoute CRD: same routes for the cluster (namespace swiftdrop)
        └── middlewares.yaml             # Middleware + ServersTransport CRDs: same values as dynamic-routes.yml
```

`scripts/gateway_parity.py` (needs Python with PyYAML) fails if any middleware or servers transport differs between `dynamic-routes.yml` and `kubernetes/middlewares.yaml`. Only the CORS allowed origins may differ. Change both files in the same PR.

---

## 4. Local Testing with Docker

Docker Compose (`deploy/docker-compose.yml`) mounts both files. To run Traefik alone:
```bash
docker run -d \
  -p 80:80 \
  -p 127.0.0.1:8090:8090 \
  -v $(pwd)/traefik/traefik.yml:/etc/traefik/traefik.yml:ro \
  -v $(pwd)/traefik/dynamic-routes.yml:/etc/traefik/dynamic/dynamic-routes.yml:ro \
  --name swiftdrop-gateway \
  traefik:v3.1.7
```
The dashboard is served only on the internal `traefik` entry point: `http://127.0.0.1:8090/dashboard/` (health: `/ping`). Port 8080 serves nothing.

Local behaviour to expect: CORS only allows `http://localhost:3000` and `http://127.0.0.1:3000`, logins are limited to 5 per minute per IP (burst 3), and API bodies over 1 MB get `413`.

---

## 5. Cloudflare Integration & Origin Protection

In the production Kubernetes environment (Ubuntu VPS):

1. **Origin Firewalling:** Host ports `80` and `443` are restricted via `ufw` to accept incoming traffic strictly from Cloudflare's published IP ranges (https://www.cloudflare.com/ips/). Direct access to the VPS IP from unauthorized clients is dropped.
2. **TLS Termination (Full Strict):** Cloudflare communicates with Traefik over HTTPS using a Cloudflare Origin CA certificate installed as a Kubernetes Secret (`traefik-origin-tls`). This provides 15-year certificate validity with strict origin authenticity.
3. **Real Client IP Preservation:** Traefik's `entryPoints.web.forwardedHeaders.trustedIPs` and `entryPoints.websecure.forwardedHeaders.trustedIPs` trust Cloudflare's proxies, ensuring backend audit logs and in-memory rate limiters operate on real client IPs rather than Cloudflare edge IPs.

---

## 6. Kubernetes Deployment with K3s

As documented in [ADR-002](../../docs/architecture/adr-002-kubernetes-hosting.md):

1. **K3s Installation:** K3s is installed with the default bundled Traefik disabled:
   ```bash
   curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik" sh -
   ```
2. **Traefik v3.x Installation:** Traefik is deployed declaratively into namespace `traefik` using official Helm charts or customized manifests with Custom Resource Definitions (`IngressRoute`, `Middleware`).
3. **Production Hardening:** The Traefik dashboard is configured with `api.insecure: false`. Cluster administrators inspect routing state and metrics exclusively via authenticated `kubectl port-forward` sessions.
4. **Routes and middlewares:**
   ```bash
   kubectl apply -f deploy/gateway/traefik/kubernetes/middlewares.yaml
   kubectl apply -f deploy/gateway/traefik/kubernetes/ingressroutes.yaml
   ```
   Prerequisites: namespace `swiftdrop`; TLS Secret `traefik-origin-tls` (Cloudflare Origin CA, ADR-002); ClusterIP Services `core-api:8080`, `tracking-api:8081`, `ai-assistant:8000` and `:8002`, `web:3000`. Only `ai-assistant` has a manifest in `deploy/kubernetes/` today.
5. **Host:** one host, paths choose the service. `swiftdrop.example.com` in `ingressroutes.yaml` is a placeholder until the production domain is fixed. ADR-002's per-service subdomain diagram predates this choice and should be updated to match.

### 6.1 Verification status

- The 25 behaviour checks (GS-01 to GS-21, in `docs/drafts/member-07-dispatcher-admin-web-gateway/doc7-tests/gateway/`) passed against the proposed `traefik.yml` and `dynamic-routes.yml` on 26 Oct 2026. The files here are identical to that proposal except that the MCP route is `/mcp` on port 8002 (the AI service's real MCP path) instead of the unused `/api/v1/ai/mcp/sse`. The `/mcp` route is not covered by those checks.
- The Kubernetes files have not been applied to a cluster or validated with `kubeconform` in this repo yet.
