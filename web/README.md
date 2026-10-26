# SwiftDrop Unified Web Application (`web/`)

> **Application Type:** Single-Page Application (SPA)  
> **Tech Stack:** React 18+, Vite 5+, TypeScript (strict mode), Tailwind CSS, TanStack Query v5, React Router v6  
> **Primary Ownership:**  
> - **Member 6:** Customer & Merchant Portal (`src/features/customer-merchant/`)  
> - **Member 7:** Dispatcher Board & Admin Backoffice (`src/features/dispatcher-admin/`)  
> - **Shared:** Application Shell, Auth & Design System (`src/app/`, `src/shared/`)  
> **Container Port:** `3000` (Nginx Alpine reverse proxy)  
> **API Contracts:** Detailed in co-located [`web/API.md`](./API.md)

---

## 1. Architectural Blueprint & Rationale

SwiftDrop adopts a **Unified Single-Page Application** architecture rather than splitting the client into multiple separate repositories or deployments.

### Why Unified SPA?
1. **Single Deployment & Port:** Built once into static assets and served via an optimized Nginx container on port `3000` behind the Traefik API Gateway. No complex multi-port routing or cross-origin overhead.
2. **Unified Authentication & RBAC Shell:** Single login screen (`/auth/login`). Upon JWT authentication, the user is redirected dynamically based on their role (`CUSTOMER`, `MERCHANT` &rarr; `/portal/*` vs `DISPATCHER`, `ADMIN` &rarr; `/ops/*`, `COURIER` &rarr; `/courier/*`).
3. **Shared Design System:** A unified component system (buttons, modals, form inputs, status chips, notification toasts) ensures consistent brand visual standards across all portals.
4. **Feature-Sliced Isolation:** Code is isolated into dedicated feature folders (`customer-merchant` vs `dispatcher-admin`). Member 6 and Member 7 work in decoupled directories without merge conflicts.
5. **Route-Based Code Splitting:** Vite dynamically splits bundles at route boundaries (`React.lazy`). Customers never download the dispatcher board or admin reporting code.

---

## 2. Directory Structure

```text
web/
├── API.md                               # Consumed API contracts & schemas (Co-located)
├── README.md                            # Living frontend documentation (This file)
├── package.json                         # Dependencies and build scripts
├── vite.config.ts                       # Vite bundling configuration & path aliases
├── tsconfig.json                        # Strict TypeScript compiler options
├── tailwind.config.js                   # Design tokens, color palette & typography
├── public/                              # Static public assets (favicons, logos)
└── src/
    ├── app/                             # Core application shell & providers (Shared)
    │   ├── routes/                      # Route definitions with lazy-loaded views
    │   ├── providers/                   # AuthProvider, QueryClientProvider, ToastProvider
    │   ├── layouts/                     # PortalLayout, OpsLayout, PublicLayout, AuthLayout
    │   └── guards/                      # RequireAuth, RoleGuard, GuestOnly
    ├── shared/                          # Cross-cutting UI components & utilities (Shared)
    │   ├── api/                         # Axios/Fetch client, interceptors, error handler
    │   ├── components/                  # Button, Modal, Input, DataTable, Badge, Skeleton
    │   ├── hooks/                       # useAuth, useDebounce, useSSE, useNotification
    │   ├── types/                       # Shared models (User, Role, ApiEnvelope, ParcelStatus)
    │   └── utils/                       # formatLKR(cents), date helpers, validators
    └── features/
        ├── customer-merchant/           # Owned by Member 6 (Sajini)
        │   ├── auth/                    # Customer self-registration & login forms
        │   ├── booking/                 # Multi-step parcel booking wizard & fee estimation
        │   ├── tracking/                # Interactive tracking view & SSE live updates
        │   ├── orders/                  # Merchant bulk order creation & CSV upload
        │   └── settings/                # Merchant profile, API keys & webhook registration
        └── dispatcher-admin/            # Owned by Member 7
            ├── dispatch-board/          # Live courier assignment & unassigned parcel board
            ├── fleet/                   # Courier status, vehicle capacity & shift management
            ├── courier-mobile/          # Mobile-first courier task list & POD capture
            ├── zones/                   # Admin delivery zone configuration & polygon maps
            └── reports/                 # Operational metrics, SLA charts & audit logs
```

---

## 3. Role-Based Routing & Access Control Matrix

The router guards ensure strict authorization before any protected view is mounted:

| Path Pattern | Allowed Roles | Feature Owner | Description |
| :--- | :--- | :---: | :--- |
| `/auth/login` | *Public / Guest* | Shared | Unified authentication screen (email & password / SSO). |
| `/auth/register` | *Public / Guest* | Member 6 | Customer self-registration form. |
| `/track/:trackingNumber` | *Public* | Member 6 | Sanitized public shipment tracker (SSE real-time stream, **SEC-009** masked). |
| `/portal/dashboard` | `CUSTOMER`, `MERCHANT` | Member 6 | Customer shipment overview, recent activity, quick booking action. |
| `/portal/book` | `CUSTOMER`, `MERCHANT` | Member 6 | Multi-step parcel booking wizard with live fee estimation (`/pricing/quotes`). |
| `/portal/tracking` | `CUSTOMER`, `MERCHANT` | Member 6 | Authenticated tracking view with full delivery history and receipt download. |
| `/portal/merchants/apply` | `CUSTOMER`, `MERCHANT` | Member 6 | Merchant onboarding application form. |
| `/portal/merchants/orders` | `MERCHANT` | Member 6 | Merchant B2B batch order ingestion, CSV upload, and label generation. |
| `/portal/merchants/api-keys`| `MERCHANT` | Member 6 | Machine-to-machine API key management & one-time copy modal. |
| `/portal/merchants/webhooks`| `MERCHANT` | Member 6 | Webhook endpoint registration & one-time signing secret modal. |
| `/ops/dispatch` | `DISPATCHER`, `ADMIN` | Member 7 | Interactive dispatch board: parcel queues, courier workload, assignment. |
| `/ops/couriers` | `DISPATCHER`, `ADMIN` | Member 7 | Active courier fleet management, load monitoring, availability toggle. |
| `/ops/zones` | `ADMIN` | Member 7 | Delivery zone geometry, base tariffs, and operational boundary setup. |
| `/ops/reports` | `ADMIN` | Member 7 | SLA reports, courier performance metrics, and audit log explorer. |
| `/courier/tasks` | `COURIER` | Member 7 | Mobile-responsive courier route, pickup/delivery tasks, barcode scanner. |
| `/courier/tasks/:id/pod` | `COURIER` | Member 7 | Proof-of-delivery capture (camera photo upload, recipient signature). |

---

## 4. Shared API Client & Authentication Guarantees

All backend HTTP requests flow through a central client configured in `src/shared/api/`:
- **Base URL:** Driven by `VITE_API_BASE_URL` (defaulting to `/api/v1` through the reverse proxy).
- **JWT Storage:** Short-lived access tokens (15 minutes) are stored **only in memory** in React `AuthContext`. Tokens are never written to `localStorage` or `sessionStorage`.
- **JWT Injection:** Outgoing requests automatically inject `Authorization: Bearer <accessToken>`.
- **Transparent Refresh:** On receiving `401 Unauthorized`, the client dispatches `POST /api/v1/auth/refresh` using the `HttpOnly` refresh cookie before retrying the failed request.
- **Tenant Context:** Multi-tenant isolation is derived server-side from authenticated JWT claims (`tenant_id`); client-supplied headers cannot override server-verified tenant identity.
- **Zero Client-Side Calculation:** Frontend components **never calculate delivery fees or determine valid status transitions independently**. All prices and statuses are fetched authoritatively from Core API endpoints.
- **One-Time Secret Display:** Newly created API keys (`sd_live_*`) and webhook secrets (`whsec_*`) are presented in an emergency modal dialog and held only in transient component memory; they are permanently purged upon closing.

---

## 5. Development & Verification Commands

```bash
# Navigate to web frontend workspace
cd web

# Install dependencies (Node.js 20+)
npm install

# Run Vite development server (HMR enabled on http://localhost:3000)
npm run dev

# Run TypeScript type-checking & ESLint
npm run lint

# Build production bundle with minification and gzip optimization
npm run build

# Preview production build locally
npm run preview
```

---

## 6. Living Documentation Maintenance Policy
- **Member 6** synchronously updates this document and [`web/API.md`](./API.md) whenever new customer or merchant routes, workflows, or views are introduced.
- **Member 7** synchronously updates this document and [`web/API.md`](./API.md) whenever new dispatcher boards, courier mobile workflows, or admin views are added.
- **Zero Drift:** Any change affecting routing, shared API contracts, or role guards must be documented here synchronously with code modifications.
