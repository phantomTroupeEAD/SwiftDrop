# SwiftDrop Unified Web Application (`web/`)

Read the [SwiftDrop UI design system](design.mdx) before building a screen. Start with the end user's role preset and complete the screen brief.

After sign-in, the customer and merchant home pages share the same layout and display recent shipments and status counts from their account-scoped APIs. Counts describe the returned collection. Merchant integrations and customer business-account applications have separate sections.

The merchant home and orders page display the business profile, tenant ID, and approval status returned by the merchant API.

The customer and merchant dashboards share the navigation header with their portal pages. On phones, navigation wraps below the brand and session action. Empty shipment lists link to the next action, and merchant profile errors have an independent retry.

## Member 6 Apple-style frontend

The customer and merchant workflows use `public/scratch-apple.html` as their visual reference. From `web/`, run `npm ci` and `npm run dev`. Open `/track` for public tracking, `/auth/register` for registration, or sign in as a customer or merchant to open the corresponding portal. The static prototypes and `/design-preview` are separate review surfaces.

Public tracking and registration use the centered blue profile. Customer booking and merchant applications use the blue customer profile. Merchant orders, API keys, and webhooks use the teal merchant profile. On phones, merchant orders show tracking and status first, with reference and date below.

- **Customer:** Four-step quote and booking flow, server-issued tracking number, and merchant application/status.
- **Merchant:** Orders with search and status filters, CSV import with per-record results and retries, API key management, and webhook registration and removal.
- **Public tracking:** Redacted snapshot, event-driven refresh, and polling while the event stream reconnects.
- **Signed-in tracking:** Customers and merchants can open `/portal/tracking` to look up parcels they can access. Merchant tracking numbers link directly to this view.

Portal requests use `VITE_API_BASE_URL` (default `/api/v1`). Vite's local fallback only signs in a demo account. The workflows require the corresponding backend services to return data. See [API.md](API.md) for integration status and [test-ids.md](test-ids.md) for active selectors.

For local integration, Vite forwards tracking requests to the tracking service on port 8081 and the remaining API requests to Core API on port 8080. A real booking test needs a customer session and active pickup and delivery zones with a pricing rule.

Reloading a protected page restores a real session using the server-issued refresh cookie. Local demo sign-ins last until reload.

For live integration, start Vite with demo auth unset. By default it connects to Core API on port 8080 and Tracking API on port 8081. Set `SWIFTDROP_CORE_API_URL` and `SWIFTDROP_TRACKING_API_URL` before starting Vite if the team provides different service origins. Use the gateway origin for both variables when it serves both APIs.

For a UI-only preview, set `SWIFTDROP_DEMO_AUTH=1` and bind Vite to loopback. Demo credentials only open the frontend screens. Live API calls require a session issued by the real backend.

Booking prefills the sender's name and phone from the signed-in profile. The sender can edit both fields and enter the pickup address.

Pickup and delivery forms support browser address autofill with separate sender and recipient sections. Merchant order search and status filters apply to the full fetched list before it is displayed in pages of 20.

After booking, **View receipt** fetches the official receipt using the returned parcel ID when the parcels API supports it.

Clean UI update (6 November 2026): the design-preview routes now render the teal/ivory shipping UI directly. Removed theme-selection chrome, palette specifications, coordination notes, technical banners, reviewer state controls and simulated SSE connectivity. Sidebar navigation, forms, orders filters and the appearance toggle remain. A small **Sample data** label and action-specific feedback preserve honesty about incomplete integration. This change is presentation-only; real APIs and shared M7/auth code are not changed.

Clean UI verification: production build passed; browser checks exercised all four wizard steps and the unconnected confirmation message, orders navigation/search/zero-match result, and page-width checks at 1440px and 375px. No page-level horizontal overflow was observed at those widths. These checks do not establish real API integration or end-to-end authorization.

Theme-preview update (6 November 2026): `/design-preview` shows the earlier **Swift Signature (Teal & Warm Ivory)** direction with sample customer and merchant flows. See [`design.mdx`](design.mdx) for the Apple-style implementation baseline and [`test-ids.md`](test-ids.md) for selector coverage.

## Implementation checkpoint — 5 November 2026

The sections below include the target architecture and planned commands. The checked-in app currently implements a login page, five prototype role dashboards, and an isolated design preview studio (`/design-preview`). Actual scripts in `package.json` are `dev`, `build`, and `preview`; lint, unit and E2E scripts/tooling have not been installed. TanStack Query is described below but is not yet a dependency.


[`test-ids.md`](test-ids.md) records all implemented customer/merchant prototype selectors, registers the visual preview studio selectors, and reserves canonical IDs for future integrated features. Input labels in the customer prototype and preview studio are properly associated with their controls via `htmlFor`/`id`.

Current customer/merchant content is under `src/features/dashboards/`, rather than the planned `customer-merchant/` structure. Customer pricing is locally calculated example data, booking and receipt actions only show alerts, tracking is static, CSV browsing is a placeholder, and the merchant key is a fixed example. These behaviours do not meet production pricing, tracking, booking or one-time-secret requirements. API integration and the four-step wizard are the next feature slice; see [`API.md`](API.md) for the integration checkpoint.

Foundation verification: `npm run build` passed (`tsc && vite build`). All selector literals are documented, form labels are connected, and the diff contains no whitespace errors. No unit/E2E suite was executed because the corresponding scripts/dependencies are not present.

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
2. **Unified Authentication & RBAC Shell:** After sign-in, `/dashboard` displays the customer or merchant home. It routes admins to `/admin/dashboard`, dispatchers to `/dispatcher/dashboard`, and couriers to their delivery tasks at `/courier/dashboard`. Protected routes check the session role before rendering.
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

Sign-in, customer registration, profile completion, and customer and merchant portal errors show a request reference when available. Include it when reporting a failed request.

All backend HTTP requests flow through a central client configured in `src/shared/api/`:
- **Base URL:** Driven by `VITE_API_BASE_URL` (defaulting to `/api/v1` through the reverse proxy).
- **JWT Storage:** Short-lived access tokens (15 minutes) are stored **only in memory** in React `AuthContext`. Tokens are never written to `localStorage` or `sessionStorage`.
- **JWT Injection:** Outgoing requests automatically inject `Authorization: Bearer <accessToken>`.
- **Transparent Refresh:** On receiving `401 Unauthorized`, the client dispatches `POST /api/v1/auth/refresh` using the `HttpOnly` refresh cookie before retrying the failed request.
- **Tenant Context:** Multi-tenant isolation is derived server-side from authenticated JWT claims (`tenant_id`); client-supplied headers cannot override server-verified tenant identity.
- **Zero Client-Side Calculation:** Frontend components **never calculate delivery fees or determine valid status transitions independently**. All prices and statuses are fetched authoritatively from Core API endpoints.
- **One-Time Secret Display:** Newly created API keys (`sd_live_*`) and webhook secrets (`whsec_*`) are presented in an emergency modal dialog and held only in transient component memory; they are permanently purged upon closing.
- **Idempotency Header:** All non-idempotent mutation requests (`POST /api/v1/parcels`, etc.) MUST inject `Idempotency-Key: <uuid-v4>`.
- **Public Masking:** Public tracking views (`/track/:trackingNumber`) MUST redact recipient phone, full street address, and proof attachments.

---

## 5. Frontend Engineering Ruleset & Invariant Enforcement

Every developer and AI agent contributing to `web/` must adhere to the formal rulesets:
- **Cursor IDE Rule:** [`.cursor/rules/frontend.mdc`](../.cursor/rules/frontend.mdc)
- **Antigravity Rule:** [`.agents/rules/frontend.md`](../.agents/rules/frontend.md)
- **Claude Code Guide:** [`web/CLAUDE.md`](CLAUDE.md)
- **Master Ruleset & Implementation Plan:** Detailed plan covering state management, tokens, role guards, and design tokens is located in the repository artifacts.

### Key Invariants Summary:
1. **Pricing Invariant:** Quotes must be obtained via `POST /api/v1/pricing/quote`; pass the returned `quote_id` during booking. Never compute fee totals client-side.
2. **State Transition Invariant:** Aggregate transitions are governed solely by the backend `Parcel` root; never simulate or update statuses directly.
3. **Dual-Token Storage Invariant:** Access tokens remain strictly in-memory (`AuthContext`); refresh tokens remain in `HttpOnly`, `SameSite=Strict` cookies. Never use `localStorage` or `sessionStorage` for tokens.
4. **Currency Invariant:** Financial values are stored and transferred as integer cents in Sri Lankan Rupees (LKR). Format using `formatLKR(cents)` (e.g. `45000` &rarr; `LKR 450.00`).

---

## 6. Testing Strategy & Execution

### 6.1 Unit & Component Testing (Vitest + React Testing Library)
- **Tooling:** Vitest with `@testing-library/react` and `@testing-library/jest-dom`.
- **Target Coverage:**
  - Currency and date formatting utilities (`formatLKR`).
  - Custom hooks (`useAuth`, `useSSE`, `useDebounce`).
  - Zod validation schemas for booking and merchant ingestion.
  - Route guards (`RequireAuth`, `RoleGuard`, `GuestOnly`).
  - Five core component states (`Loading`, `Empty`, `Validation Error`, `Server Error`, `Success`).

### 6.2 End-to-End Testing (Playwright)
- **Tooling:** `@playwright/test` running in headless CI and interactive UI modes.
- **Architecture:** Page Object Models (POM) in `web/tests/e2e/pages/`.
- **Critical Journeys Covered:**
  1. **Customer Booking Journey:** Form completion &rarr; live pricing quote countdown &rarr; idempotent submission &rarr; tracking number & receipt view.
  2. **Public Tracking Redaction Journey:** Visit `/track/:trackingNumber` as anonymous visitor &rarr; verify milestone timeline &rarr; verify phone, address, and proof photos are scrubbed.
  3. **Dispatch & Courier POD Journey:** Dispatcher assigns parcel &rarr; Courier accepts on mobile viewport &rarr; captures signature canvas and photo &rarr; status updates to `DELIVERED`.
  4. **Failed Attempt & Reschedule Journey:** Courier logs failure with reason code &rarr; `ATTEMPT_FAILED` &rarr; Dispatcher authorizes re-attempt.
  5. **RBAC Guard Enforcement:** Customer attempting to access `/ops/*` receives 403 Forbidden.

---

## 7. Interactive Design Showcases & Component Studios

The [M6 UI review guide](M6-UI-REVIEW.md) explains how to open the customer and merchant preview and what to review. Two standalone component showcases in `web/public/` can be opened directly or served by Vite:

1. **`web/public/scratch.html` (Option 3 · Swift Signature Showcase)**:
   - Evaluates the Teal & Warm Ivory design theme.
   - Comprehensive component catalog with 4-step booking progression, courier POD signature capture, live pricing countdown, and transit milestone tracking.

2. **`web/public/scratch-apple.html` (Apple Cupertino & Shadcn/UI Interactive Studio)**:
   - **4 Design Archetypes:** Apple Cupertino (`data-style="apple"`), Linear/Craft (`data-style="linear"`), Framed Sheet (`data-style="framed"`), and Swift Signature (`data-style="swiftsig"`).
   - **Dynamic Accent Color Picker:** 7 calibrated operational accent colors (Apple Blue, Swift Teal, Zinc Slate, Oxford Navy, Amber, Emerald, Crimson) plus arbitrary native hex picker with live CSS custom property updates.
   - **Shadcn/UI Component Primitives:** Accordion with rotating chevrons, Dialog modal with zoom-in backdrop, Toast notification banner with auto-dismiss, Tooltips, Spring toggle switches, Shimmer skeleton loaders, Tabs, and tactile buttons with active press feedback (`scale(0.98)`).
   - **Recommended Micro-Animations:** Apple-standard cubic bezier easing (`cubic-bezier(0.16, 1, 0.3, 1)`), 150ms–220ms durations, zero sluggish ease-in animations.
   - **Embedded Lucide SVG Icons:** High-precision SVG iconography across navigation, actions, form inputs, status chips, and a dedicated 20-icon logistics catalog.
   - **Taste-Skill Anti-Slop Compliant:** Strictly enforces `color-on-data <= 3%`, zero resting card shadows, zero purple/violet gradients, and verified via `taste-skill/scripts/lint.mjs --app` (`0 FAIL, 0 WARN`).

---

## 8. Development & Verification Commands

```bash
# Navigate to web frontend workspace
cd web

# Install dependencies (Node.js 20+)
npm install

# Run Vite development server (HMR enabled on http://localhost:3000)
npm run dev

# Run TypeScript type-checking & ESLint
npm run lint

# Run unit and component tests (Vitest)
npm run test:unit

# Run unit tests with coverage
npm run test:unit:coverage

# Run End-to-End tests (Playwright)
npm run test:e2e

# Run Playwright tests with interactive UI
npx playwright test --ui

# Build production bundle with minification and gzip optimization
npm run build

# Preview production build locally
npm run preview
```

---

## 9. Living Documentation Maintenance Policy
- **Member 6** synchronously updates this document and [`web/API.md`](./API.md) whenever new customer or merchant routes, workflows, or views are introduced.
- **Member 7** synchronously updates this document and [`web/API.md`](./API.md) whenever new dispatcher boards, courier mobile workflows, or admin views are added.
- **Zero Drift:** Any change affecting routing, shared API contracts, or role guards must be documented here synchronously with code modifications.
