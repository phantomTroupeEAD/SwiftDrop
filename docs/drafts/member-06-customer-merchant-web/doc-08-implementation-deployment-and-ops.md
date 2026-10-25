# Document 8 — Implementation, Deployment & Operations Guide

> **Module:** Customer & Merchant Frontend  
> **Owning Member:** Sajini, Member 6 — Customer & Merchant Frontend  
> **Target Workspace:** `web/` (Inside Unified React SPA)  
> **Status:** Draft Specifications Complete (Ready for Team Review in Meeting 2)  
> **Governing Standards:** SwiftDrop SRS, ADR-001, ADR-002, `AGENTS.md`, and Team Agreements with Member 9 (A1, A3, A4, A9, A12)  
> **Date:** 25 October 2026 | **Version:** 1.0.0

---

## 1. Workspace Organization & Build Pipeline

The customer and merchant interfaces are compiled as part of the unified SwiftDrop Single-Page Application (SPA) located at `web/`.

### 1.1 Local Workspace Structure
```text
web/
├── package.json                            # Unified dependencies and scripts
├── tsconfig.json                           # TypeScript strict mode configuration
├── vite.config.ts                          # Vite build tooling and local dev proxy
├── tailwind.config.js                      # Tailwind design system tokens
├── index.html                              # SPA HTML entry point
├── src/                                    # TypeScript application source
└── dist/                                   # Production build output directory
```

### 1.2 Planned NPM Lifecycle Scripts
All developer workflows and CI validation tasks are executed via standard `npm` scripts:

```bash
cd web

# 1. Dependency Installation (Clean install from lockfile in CI)
npm install         # Local development
npm ci              # CI / Docker builder stage

# 2. Local Development Server (Vite HMR on port 3000)
npm run dev

# 3. Static Analysis & Linting
npm run lint

# 4. Unit & Component Testing
npm run test:unit
npm run test:unit:coverage

# 5. Playwright End-to-End Tests (Headless)
npm run test:e2e

# 6. Production Compilation
npm run build       # Compiles TypeScript and bundles assets into dist/

# 7. Local Production Preview Server
npm run preview
```

---

## 2. Environment Configuration Standards

### 2.1 Public Build-Time Variables
The frontend requires only a single environment variable:

```dotenv
# web/.env.example
VITE_API_BASE_URL=/api/v1
```

### 2.2 Inviolable Configuration Rules
1. **Public Baking Rule:** In Vite, any variable prefixed with `VITE_` is statically injected into the JavaScript bundle during `npm run build`. Changing container environment variables at runtime will **not** alter the compiled bundle.
2. **Relative Path Advantage:** By setting `VITE_API_BASE_URL=/api/v1`, the compiled SPA makes relative HTTP requests to the same origin (e.g. `https://swiftdrop.example.com/api/v1/parcels`). This allows a single container image (`ghcr.io/phantomtroupeead/swiftdrop/web:<git-sha>`) to run unchanged across Local, Staging, and Production environments without rebuilds.
3. **Zero Secrets in Frontend:** Database connection strings, JWT signing keys, Google OAuth client secrets, and webhook encryption keys must **never** be defined in `web/.env` or bundled into client code.

---

## 3. DevOps Handoff & Containerization Architecture (Coordinated with Member 9)

In accordance with Team Agreement **A3** and **A12** with Member 9:

| Specification Parameter | Agreed Value | Ownership & Authority |
| :--- | :--- | :--- |
| **Dockerfile Ownership** | `web/Dockerfile` is owned and maintained by **Member 9** | Agreement A3 |
| **Nginx Config Ownership** | Nginx template and routing rules owned by **Member 9** | Agreement A3 |
| **Build Command** | `npm run build` | Member 6 specification |
| **Static Output Directory** | `dist/` | Member 6 specification |
| **Container Port** | **Port 3000** (`EXPOSE 3000`) | Agreement A1 |
| **Runtime Base Image** | `nginx:alpine` (pinned, non-root user `10001:10001`) | Agreement A3 |
| **SPA Routing Fallback** | Nginx directive: `try_files $uri $uri/ /index.html;` | Member 6 requirement |
| **Health Probe Route** | Dedicated `GET /health/live` returning `200 OK` | Agreement A2 |
| **Traefik Edge Routing** | Routes `/` and `/portal/**` to `web:3000` | Agreement A9 (Member 7 & 9) |

### 3.1 Multi-Stage Dockerfile Blueprint (Member 9 Reference)
```dockerfile
# Stage 1: Build static assets
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ENV VITE_API_BASE_URL=/api/v1
RUN npm run build

# Stage 2: Serve with high-performance Nginx
FROM nginx:1.27-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
USER 10001:10001
EXPOSE 3000
CMD ["nginx", "-g", "daemon off;"]
```

### 3.2 Nginx SPA & Health Configuration
```nginx
server {
    listen 3000;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    # Dedicated Kubernetes / Docker health probe (Never falls through to SPA)
    location = /health/live {
        access_log off;
        default_type text/plain;
        return 200 "healthy\n";
    }

    # Static asset caching (fingerprinted Vite bundles)
    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # SPA Fallback for deep-linking (e.g. /portal/book, /track/SD-101)
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

---

## 4. Local Development Proxying with Vite

During local development (`npm run dev`), the Vite development server runs on `http://localhost:3000`. To route relative API requests (`/api/v1/**`) seamlessly to the local Docker Compose gateway without triggering Cross-Origin Resource Sharing (CORS) complications, `vite.config.ts` includes an HTTP/SSE proxy:

```typescript
// web/vite.config.ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api/v1': {
        target: 'http://localhost:8080', // Points to Traefik Gateway or Core API locally
        changeOrigin: true,
        ws: true, // Supports WebSocket / SSE streaming
      },
    },
  },
});
```

---

## 5. Troubleshooting & Operational Runbook

| Symptom | Root Cause | Investigative Action & Fix |
| :--- | :--- | :--- |
| **Blank White Screen** | Missing JS bundle or script syntax error | Open browser developer console. Check network tab for 404s on `/assets/*.js`. Ensure base URL is relative. |
| **404 on Browser Page Refresh** | Nginx missing SPA fallback directive | Ensure Nginx has `try_files $uri $uri/ /index.html;`. Direct navigation to `/portal/book` must return `index.html`. |
| **API 404 on Booking Submission** | Duplicated API path segment | Inspect failed URL in Network tab. If URL is `/api/v1/api/v1/parcels`, check API client base URL configuration. |
| **SSE Tracking Stays Stale** | Gateway or Nginx buffering responses | Verify Traefik and Tracking API set header `X-Accel-Buffering: no` and `Content-Type: text/event-stream`. |
| **Login Session Lost on Reload** | HttpOnly refresh cookie blocked | Verify browser cookie settings. In local development without HTTPS, ensure `Secure` flag is relaxed for localhost. |
| **Raw Secrets Stored in Browser** | Component state leak | Audit `localStorage` and `sessionStorage`. Confirm one-time modals destroy keys immediately on unmount. |
