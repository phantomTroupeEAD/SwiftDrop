# SwiftDrop Member 6 Clean UI Review Handoff

**Author:** Sajini (Member 6 — Customer & Merchant Portal)
**Target Reviewer:** Bhagya (Member 7 — Dispatcher & Admin Backoffice)
**Date:** 6 November 2026
**Status:** Ready for Review

---

## 1. Purpose & Scope of this Branch

This review branch isolates the clean visual foundation and customer/merchant user experience for Member 7 (Bhagya) to evaluate before we unify our shared application layouts (`src/app/layouts/DashboardLayout.tsx`) and finalize global design tokens.

The UI presents a clean, production-styled shipping application rather than a design showcase. All design-studio evaluation chrome, multi-theme comparison toggles, reviewer forced-state switches, and simulated SSE connection indicators have been removed. Unconnected features explicitly display honest action feedback to prevent confusion during testing.

- **Base Commit:** `3cdc71fb93e964e0291ce69c4a3d557eeac80383` (`main`)
- **Review Branch:** `review/m6-clean-ui-bhagya`
- **Scope Boundary:** Scoped strictly to the Member 6 customer-merchant preview and associated living documentation. Operational dashboards (`src/features/dashboards/DispatcherDashboard.tsx`, `AdminDashboard.tsx`), authentication guards, and Go Core API contracts remain untouched.

---

## 2. Colour Palette & Intended Roles

The UI adopts the **Teal & Warm Ivory ("Swift Signature")** palette, designed to deliver high contrast, comfortable data entry, and distinct logistics branding:

| Token Name | Hex Value (Daylight) | Hex Value (Evening) | Intended Semantic Role |
| :--- | :--- | :--- | :--- |
| **Primary Teal** | `#0B5955` | `#85D4B5` | Primary CTA buttons, active wizard steps, active sidebar links, key brand markers |
| **Primary Hover** | `#08433F` | `#A3E3CB` | Hover and active interaction states |
| **Warm Ivory Canvas** | `#F7F5EE` | `#102824` | Page background canvas; reduces eye strain during prolonged screen use |
| **Pure White** | `#FFFFFF` | `#193630` | Elevated card surfaces, form containers, and tables |
| **Soft Mint** | `#E3EFE8` | `#214339` | Quiet surfaces, tracking preview cards, and active table rows |
| **Forest Ink** | `#142E2B` | `#F7F5EE` | Headings, primary body typography, and sidebar background |
| **Muted Ink** | `#4B635C` | `#B9CEC3` | Secondary labels, field helpers, timestamps, table column headers |
| **Border / Line** | `#D1DED6` | `#37574D` | Form borders, card boundaries, table cell dividers |
| **Burnt-Orange Accent** | `#B84D26` | `#F2A17D` | Logo dot and subtle brand accents (used sparingly; never used as an alert) |
| **Dedicated Amber** | `#B45309` | `#FBBF24` | Delivery exceptions, warnings, action needed (separate from brand orange) |
| **Emerald Success** | `#047857` | `#34D399` | Confirmed status, delivered badges, check indicators |
| **Rose Error** | `#B91C1C` | `#F87171` | Field validation failures and network errors |

---

## 3. Customer & Merchant Screens Available

The clean UI is organized into two primary workflows accessible via the sidebar navigation:

### Screen A: Customer Delivery Booking Wizard (`#customer`)
An accessible, responsive 4-step wizard:
1. **Addresses:** Sender full name, mobile number, pickup address & city; recipient full name, mobile number, delivery address & city.
2. **Parcel Details:** Weight in kg (with 30kg courier limit helper), dimensions (L × W × H cm), pickup zone selector (`WESTERN_COLOMBO_01`), destination zone selector (`CENTRAL_KANDY_01`), delivery priority (Standard vs Express), and fragile item handling checkbox.
3. **Delivery Estimate (Quote):** Displays estimated delivery charges, weight surcharges, and total formatted in LKR (`formatLKR`). Marked with a clear "Sample quote" badge to reflect that the live Pricing API is not yet wired.
4. **Review & Confirm:** Origin/destination summary, parcel specs, service priority, and a "Confirm booking" primary action.
5. **Integrated Tracking Lookup:** Public tracking card with tracking number input and a privacy-safe milestone timeline preview (SEC-016 compliant: masks phone numbers and private addresses; displays milestone times and destination city only).

### Screen B: Merchant Orders Dashboard (`#merchant`)
A clean logistics management view:
1. **Concise Summary KPIs:** Stat cards showing Total orders (`5`), In transit (`2`), Delivered (`1`), and Needs attention (`1` with amber badge).
2. **Search & Filters:** Real-time text search for tracking numbers, customer names, or destination cities; quick filter pills (`All`, `In Transit`, `Delivered`, `Action Needed`).
3. **Orders Table:** Clean, responsive data table with Tracking Number (monospace), Recipient & Destination, Status Badge, Ingested Date, and a Details action button.
4. **Zero-Match Result:** Helpful message ("No orders match your search") displayed when filtering yields 0 rows.

---

## 4. How to Install, Run & Open the UI

### Prerequisites
Node.js 18+ and npm installed locally.

### Execution Commands
```bash
# Navigate to the frontend workspace
cd web

# Install dependencies (if not already installed)
npm install

# Start the Vite development server on port 3000
npm run dev
```

### Actual Routes & Preview URLs
- **Clean UI Route:** `http://localhost:3000/design-preview` (also mapped to `http://localhost:3000/preview/design`)
- **Customer Booking Direct:** `http://localhost:3000/design-preview?screen=customer-booking`
- **Merchant Orders Direct:** `http://localhost:3000/design-preview?screen=merchant-orders`
- **Evening Mode Direct:** `http://localhost:3000/design-preview?screen=customer-booking&palette=evening`
- **Login Screen Entry:** `http://localhost:3000/auth/login` includes a direct link button: *"Preview Customer & Merchant Portal →"*

---

## 5. Implemented Interactions vs. Sample / Unconnected Behavior

To ensure transparent review, the following matrix distinguishes real interactive UI state from mocked backend behavior:

| Feature / Control | Implemented Interaction | Unconnected / Sample Behavior |
| :--- | :--- | :--- |
| **Wizard Stepper** | Fully interactive; user can navigate forward/backward or click stepper tabs | Input values persist in local state across steps |
| **Booking Submission** | Clicking "Confirm booking" triggers feedback banner | **No parcel created;** displays *"Booking is not connected yet. No parcel has been created."* |
| **Rate Quote Calculation** | Dynamic layout adjustment based on standard vs express | **Fixed sample quote;** frontend does not calculate tariffs client-side |
| **Tracking Lookup** | Input field accepts tracking numbers | **Sample timeline;** clicking "Track Shipment" notifies user that live SSE lookup is pending |
| **Order Search & Filter** | Instant client-side filtering by tracking #, name, city, or status | Filters operate over `SAMPLE_ORDERS` dataset; zero-match state handles empty queries |
| **Batch CSV Button** | Accessible button with focus rings | **Placeholder;** clicking notifies user that CSV file upload is not yet connected |
| **Appearance Toggle** | Topbar Sun/Moon button toggles between Daylight Ivory and Evening Palette | Stored in component-local state |
| **Unobtrusive Badge** | Header displays a single **"Sample data"** label | Informs reviewers without cluttering the interface |

---

## 6. Test-ID Register & Selector Changes

All test IDs are formally registered in [`web/test-ids.md`](./test-ids.md).

### Active Selectors Implemented
- **Root & Shell:** `signature-theme-root`, `signature-night-toggle`, `login-theme-preview-link`
- **Booking Steps:** `signature-booking-step-1`, `signature-booking-step-2`, `signature-booking-step-3`, `signature-booking-step-4`
- **Booking Inputs:** `signature-booking-sender-name`, `signature-booking-sender-phone`, `signature-pickup-input`, `signature-booking-recipient-name`, `signature-booking-recipient-phone`, `signature-delivery-input`, `signature-booking-weight`, `signature-booking-dims`, `signature-booking-zone-select`, `signature-booking-fragile-check`
- **Booking Actions:** `signature-booking-next-btn`, `signature-booking-back-btn`, `signature-booking-preview-btn`, `signature-booking-quote-display`, `signature-booking-confirm-btn`
- **Tracking:** `signature-tracking-input`, `signature-tracking-search-btn`
- **Merchant Controls:** `signature-merchant-stat-total`, `signature-merchant-stat-transit`, `signature-merchant-stat-delivered`, `signature-merchant-stat-warning`, `signature-merchant-search-input`, `signature-merchant-csv-btn`, `signature-merchant-orders-table`
- **Feedback:** `signature-demo-feedback`

### Retired Selectors (Removed from Product Interface)
The following review-only selectors from earlier design comparisons were retired and should **not** be asserted by E2E tests:
- `theme-preview-root`, `theme-mode-toggle-*`, `theme-select-*`, `coordination-checklist-section`
- `signature-nav-customer-btn`, `signature-nav-merchant-btn`, `signature-nav-foundation-btn`
- `signature-merchant-loading-state`, `signature-merchant-empty-state`, `signature-merchant-error-state` (reviewer forced switches)
- `preview-*` (all legacy prototype showcase elements)

---

## 7. Shared-Shell & Theme Decisions for Bhagya's Review

Before we unify our portal shells in `src/app/layouts/DashboardLayout.tsx` and consolidate Tailwind tokens, please review:

1. **Brand Identity Consensus:**
   - Does Member 7 endorse **Teal & Warm Ivory ("Swift Signature")** as the primary theme for SwiftDrop?
   - Or should SwiftDrop offer a toggle between Teal/Ivory and the initial Refined Dark theme?
2. **Daylight vs. Evening Operational Standard:**
   - Proposal: Customer and merchant screens default to Daylight Ivory (`#F7F5EE`), while Dispatcher Board and Courier tasks default to the Evening variant (`#102824` / `#193630`) for low-light workstation comfort. Does this fit M7's operational vision?
3. **Primary Action Color:**
   - Confirm whether Primary Teal (`#0B5955`) replaces `blue-600` (`#2563eb`) as the global primary button color across all roles.
4. **Status Badge Matrix Alignment:**
   - Confirm the 4-status color scheme: `IN_TRANSIT` (Teal), `OUT_FOR_DELIVERY` (Teal), `DELIVERED` (Emerald), and `ATTEMPT_FAILED` / `NEEDS_ATTENTION` (Amber).
5. **Shared Shell Geometry:**
   - Header bar height (`h-16`), sidebar width (`240px`), sticky positioning, and mobile drawer behavior.

---

## 8. Known Limitations & Remaining Backend Integration Work

- **Pricing API (`POST /api/v1/pricing/quotes`):** Quote values are currently sample placeholders. Real integration requires submitting weight, dimensions, and zones to obtain an authoritative `quote_id` (15-min expiry).
- **Parcels API (`POST /api/v1/parcels`):** Booking confirmation currently updates local state only. Production integration requires injecting `Idempotency-Key: <uuid-v4>` with the active `quote_id`.
- **SSE Stream (`/api/v1/tracking/{trackingNumber}/stream`):** Live streaming with reconnect backoff and polling fallback is specified in `web/API.md` and will be wired via the `useSSE` hook during the integration slice.
- **Batch CSV Upload:** Ingestion endpoint requires multipart form upload and validation error parsing.

---

## 9. Verification Results & Screenshot Locations

### Verification Execution Summary
| Check | Command | Result |
| :--- | :--- | :--- |
| **TypeScript & Bundling** | `npm run build` | **PASSED** (1512 modules transformed, 0 errors) |
| **Git Diff Cleanliness** | `git diff --check` | **PASSED** (No whitespace or line-break conflicts) |
| **Viewports Tested** | 1440px (Desktop), 768px (Tablet), 375px (Mobile) | **PASSED** (Fluid layout, zero horizontal page blowout) |
| **Touch Target Audit** | All interactive controls audited | **PASSED** (Minimum 44px × 44px on buttons, tabs, inputs) |

### Local Screenshot Artifacts
- **Customer Booking Preview (1440px):** `C:\Users\sajin\.gemini\antigravity\brain\d1500dae-de7a-46a9-bae7-88c3cca6773f\customer_booking_1440.png`
- **Merchant Orders Dashboard (1440px):** `C:\Users\sajin\.gemini\antigravity\brain\d1500dae-de7a-46a9-bae7-88c3cca6773f\merchant_orders_1440.png`
- **Tablet Responsive Layout (768px):** `C:\Users\sajin\.gemini\antigravity\brain\d1500dae-de7a-46a9-bae7-88c3cca6773f\responsive_tablet_768.png`
- **Mobile Responsive Layout (375px):** `C:\Users\sajin\.gemini\antigravity\brain\d1500dae-de7a-46a9-bae7-88c3cca6773f\responsive_mobile_375.png`
- **Evening Operational Mode (1440px):** `C:\Users\sajin\.gemini\antigravity\brain\d1500dae-de7a-46a9-bae7-88c3cca6773f\evening_palette_1440.png`

---

## 10. Concise Review Checklist for Bhagya

- [ ] **Visual Identity:** Review Teal (`#0B5955`) & Warm Ivory (`#F7F5EE`) at `http://localhost:3000/design-preview`.
- [ ] **Daylight vs. Evening:** Toggle the topbar Sun/Moon icon to inspect both lighting conditions.
- [ ] **Customer Wizard:** Click through Steps 1 → 2 → 3 → 4 to verify form layout, input readability, and step flow.
- [ ] **Tracking Sample:** Inspect the tracking card for privacy-safe milestone representation.
- [ ] **Merchant Orders:** Test order search and status filter buttons; verify that zero-match queries render a clean message.
- [ ] **Mobile Responsiveness:** Shrink browser to 375px; verify that sidebar collapses and tables scroll cleanly without page overflow.
- [ ] **Shell Unification:** Note any required adjustments for your upcoming Dispatcher Board layout.

---

## 11. Files Changed on this Branch

| File Path | Nature of Change | Coordination Implication for M7 |
| :--- | :--- | :--- |
| `web/src/features/customer-merchant/preview/SwiftSignatureTheme.tsx` | Implements the clean Teal & Ivory UI (booking wizard, orders table, tracking) | Isolated M6 preview; no conflict with M7 |
| `web/src/features/customer-merchant/preview/swift-signature.css` | Scoped CSS variables and styling for Option 3 | Scoped to `.swift-signature`; does not affect global Tailwind |
| `web/src/features/customer-merchant/preview/DesignPreviewPage.tsx` | Entrypoint wrapper mounting `SwiftSignatureTheme` directly | Renders clean UI at `/design-preview` |
| `web/src/features/auth/LoginPage.tsx` | Updated link label to *"Preview Customer & Merchant Portal →"* | Minor shared entry button; coordinates login navigation |
| `web/src/App.tsx` | Routes `/design-preview` and `/preview/design` mapped to preview | Public preview routes for team evaluation |
| `web/src/features/dashboards/CustomerDashboard.tsx` | Retains initial testability hooks (`data-testid`, accessible labels) | Unchanged from initial testability baseline |
| `web/src/features/dashboards/MerchantDashboard.tsx` | Retains initial testability hooks (`data-testid`, accessible labels) | Unchanged from initial testability baseline |
| `web/design.md` | Documents Option 3 tokens, component rules, and M7 alignment | Shared documentation reference |
| `web/test-ids.md` | Registers active selectors and documents retired review selectors | Reference for QA (Member 10) and E2E suites |
| `web/README.md` | Living documentation checkpoint updated | Living repository doc |
| `web/API.md` | Consumed API contracts aligned | Unchanged HTTP contract baseline |
| `web/M6-UI-REVIEW-FOR-BHAGYA.md` | Handoff document for Member 7 review | This document |
