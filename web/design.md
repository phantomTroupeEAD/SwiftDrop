# Frontend design baseline

## Clean application presentation — 6 November 2026

The selected teal/ivory design now opens directly at `/design-preview` and `/preview/design` without the evaluation-studio wrapper. The UI retains sidebar navigation, the booking wizard, tracking sample, order search/status filters, the sample orders table and an appearance toggle. Theme comparisons, token/specification navigation, team checklists, reviewer state switches, simulated connection indicators, idempotency tokens, and implementation notes have been removed from the visible product interface.

One unobtrusive **Sample data** label remains because API integration is incomplete. Preview actions explain their actual outcome only after activation. The displayed quote is fixed sample data, not a local tariff calculation. Merchant counts derive from the sample rows. Public tracking sample content never uses the booking form's destination address or the typed lookup number as an alleged backend result.

This is the final visual direction for review, not completed production booking, authentication or tracking. All backend contracts and M7 screens remain unchanged. Earlier theme-comparison descriptions below are historical design rationale rather than current navigation instructions.

Owner: M6 (Sajini); shared visual decisions require coordination across the frontend team.
Status: Proposal phase; evaluates three themes (Option 1: Refined Dark, Option 2: Modern Light, Option 3: Swift Signature Teal & Ivory) via an isolated interactive preview studio (`/design-preview`). Documents visual tokens, component rules, and M7 alignment before full feature implementation.

---

## 1. Visual Foundation: Evaluated Options

SwiftDrop is a mission-critical parcel delivery platform serving retail customers, high-volume merchant shippers, dispatchers, and couriers. The visual foundation must project modern reliability, high contrast for logistics legibility, and straightforward form progression.

Three visual options are implemented in the isolated Design Preview Studio at route `/design-preview`:

### Option 1: Refined Dark Theme ("Obsidian Slate & Electric Azure")
- **Visual Concept:** A polished, contrast-tuned evolution of the initial repository prototype. Uses deep obsidian canvas tones (`slate-950`), elevated slate surfaces (`slate-900`/`slate-800`), crisp borders (`slate-800`/`slate-700`), and high-energy electric azure accents (`blue-600` / `#2563eb`).
- **Logistics Strengths:** High visual comfort in low-light environments (dispatch boards, overnight sorting hubs, warehouse workstations); matches existing prototype screens with zero visual disruption.
- **Trade-Offs:** Requires careful contrast tuning in bright outdoor sunlight for couriers or customers viewing receipts on mobile devices.

### Option 2: Modern Light Theme ("Crisp White & Ocean Navy")
- **Visual Concept:** Clean, trustworthy enterprise logistics styling. Built on an ultra-light canvas (`slate-50`), pure white surface cards (`white`), subtle slate separation borders (`slate-200`/`slate-300`), deep slate typography (`slate-900`), and ocean navy brand actions (`blue-600` / `blue-700`).
- **Logistics Strengths:** Exceptional readability under all lighting conditions; familiar enterprise delivery portal aesthetic (aligning with international logistics standards like DHL, FedEx, UPS); effortless scanning of tabular tracking histories and printed receipts.
- **Trade-Offs:** Requires updating prototype dashboard background utility classes from dark slate to light slate when universally adopted across the application shell.

### Option 3: Swift Signature Theme ("Teal & Warm Ivory") — Refined Proposal
- **Visual Concept:** A distinctive, characterful logistics identity designed specifically for everyday retail and merchant parcel shipping. Features deep primary teal (`#0B5955`), warm ivory canvas (`#F7F5EE`), high-contrast forest ink typography (`#142E2B`), a small burnt-orange brand detail (`#B84D26`), quiet soft mint surfaces (`#E3EFE8`), and elevated pure white cards (`#FFFFFF`).
- **Logistics Strengths:** Combines warmth with high legibility; avoids generic corporate monotony; dedicated separate amber warning system (`#B45309`) prevents confusion with brand orange accents; includes a fully styled paired evening variant (`#102824` / `#193630`) for low-light night shifts.
- **Trade-Offs:** Distinct brand departure from the initial prototype palette; requires formal consensus with Member 7 before shared layout migration in `src/app/`.

---

## 2. Option 3 ("Teal & Warm Ivory") Design Token Matrix

The scoped tokens in `swift-signature.css` define Option 3 for the M6 customer and merchant portal.

### Core Palette & Roles

| Token Name | Hex Code (Daylight) | Hex Code (Evening) | Semantic Role & Hierarchy |
| :--- | :--- | :--- | :--- |
| **Primary Teal** | `#0B5955` | `#85D4B5` | Primary action buttons, active navigation, step progress, key brand CTAs |
| **Primary Hover** | `#08433F` | `#A3E3CB` | Primary button hover & active states |
| **Warm Ivory / Forest Canvas** | `#F7F5EE` | `#102824` | Page background & outer container canvas |
| **Elevated Surface (Card)** | `#FFFFFF` | `#193630` | Cards, forms, modals, tables |
| **Quiet Surface (Mint)** | `#E3EFE8` | `#214339` | Inset summaries, tracking panels, table row hovers |
| **Forest Ink / Light Ink** | `#142E2B` | `#F7F5EE` | Headings, primary body text, sidebar background |
| **Muted Ink** | `#4B635C` | `#B9CEC3` | Secondary labels, helper text, timestamps, table headers |
| **Border / Divider** | `#D1DED6` | `#37574D` | Form borders, card outlines, table row dividers |
| **Subtle Divider** | `#E3EDE6` | `#24423A` | Internal card separations, table inner lines |
| **Burnt-Orange Accent** | `#B84D26` | `#F2A17D` | Logo dot, brand eyebrow tags, subtle focus indicators (used sparingly) |

### Semantic State Colors (Explicit & Never Color-Alone)

| Semantic State | Text Color | Background | Border | Usage & Accessible Treatment |
| :--- | :--- | :--- | :--- | :--- |
| **Warning / Attention** | `#B45309` (Amber) | `#FEF3C7` | `#FDE68A` | Delivery exceptions, action needed, expiring quotes; always paired with `AlertTriangle` icon and "Action Needed" text. |
| **Success / Confirmed** | `#047857` (Emerald) | `#D1FAE5` | `#A7F3D0` | Confirmed bookings, delivered parcels, valid tokens; paired with `Check` icon and "Delivered" label. |
| **Error / Failure** | `#B91C1C` (Rose) | `#FEE2E2` | `#FCA5A5` | Form validation failures, gateway 503 timeouts; paired with correlation ID and retry CTA. |
| **In Transit / Active** | `#0B5955` (Teal) | `#E3EFE8` | `#C5DED0` | Active sorting, out for delivery; paired with `Truck` icon and status text. |
| **Connection (Live SSE)**| `#047857` (Emerald) | `#D1FAE5` | `#A7F3D0` | Live event stream indicator with pulsing green dot and latency text. |
| **Connection (Degraded)**| `#B45309` (Amber) | `#FEF3C7` | `#FDE68A` | Fallback 30-second polling notice with warning badge. |

---

## 3. Typography & Sizing Standards

To guarantee comfortable reading across all screen sizes:

- **Body & Form Input Text:** Set to approximately **14px–16px** (default 15px, `line-height: 1.5`) for optimal legibility during data entry.
- **Form Labels & Supporting Text:** Set to approximately **12px–14px** (default 13px, `font-weight: 600`), clearly associated with inputs.
- **Headings Hierarchy:**
  - Page / Screen H2: **24px**, `font-weight: 700`, `letter-spacing: -0.03em`.
  - Card H3: **20px**, `font-weight: 700`, `letter-spacing: -0.02em`.
  - Section H4: **18px**, `font-weight: 700`.
  - Eyebrow labels: **12px**, `font-weight: 700`, uppercase, `letter-spacing: 0.08em`.
- **Prohibition on Tiny Text:** No essential customer or merchant information is rendered below 11px. Monospace identifiers (tracking codes, idempotency keys) use 12px–13px.

---

## 4. Layout, Touch & Accessibility Standards

- **Touch Target Sizes:** All interactive buttons, tabs, input fields, selects, and checkboxes have a minimum dimension of **44px × 44px** (`min-height: 44px; min-width: 44px`), compliant with WCAG 2.1 Success Criterion 2.5.5.
- **Visible Keyboard Focus:** Form elements and interactive controls display an explicit **3px outline** (`outline: 3px solid var(--ss-accent); outline-offset: 3px`) upon `:focus-visible`.
- **Card Padding & Corner Radii:** Unified card padding of **24px** (reduced to 16px–20px on mobile); standard corner radius of **12px** for cards and **8px–9px** for buttons/inputs.
- **Responsive Layout & Mobile Invariants:**
  - Tested across 375px (mobile), 768px (tablet), and 1440px (desktop).
  - Multi-column grids collapse cleanly into single-column layouts on mobile.
  - Tables are encapsulated in `overflow-x: auto` containers to eliminate horizontal page blowout.
  - Zero horizontal overflow on the root container.
- **Reduced Motion Support:** `@media (prefers-reduced-motion: reduce)` disables all transitions, animations, and smooth scrolls.

---

## 5. Demonstrated Realistic Preview Screens

The Option 3 preview at `/design-preview` demonstrates two realistic, isolated portal workflows with 100% sample data:

### Screen A: Customer Delivery Booking Wizard
- **Connected Stepper Track:** Visual horizontal rail connecting all 4 steps (Addresses → Parcel details → Quote → Review), featuring circular numbered nodes, checkmark indicators (`✓`) for completed steps, active ring highlights, and fluid scaling across viewports.
- **Step 1: Addresses:** Sender name, sender mobile (`+94 77 123 4567`), pickup address, recipient name, recipient mobile (`+94 71 987 6543`), delivery destination.
- **Step 2: Parcel & Zones:** Weight (kg) with limit helper, dimensions (L×W×H cm) for volumetric check, pickup zone selector (`WESTERN_COLOMBO_01`), destination zone selector (`CENTRAL_KANDY_01`), service level selector (Standard vs Express), fragile handling checkbox.
- **Step 3: Server Quote:** Simulated response displaying itemized fee breakdown (delivery charge + weight surcharge) and total fee formatted in LKR (`formatLKR`) using tabular figures (`tabular-nums`). Includes explicit "Sample quote" badge.
- **Step 4: Review & Idempotent Confirm:** Origin/destination summary, package specs, service priority, and preview confirmation action with honest user feedback banner explaining the action is unconnected.
- **Quick Tracking Card:** Privacy-safe (SEC-016) public tracking card with masked milestones and destination city.

### Screen B: Merchant Orders & Operations Dashboard
- **Summary KPI Bar:** Total orders (`5`), In Transit (`2`), Delivered (`1`), Needs attention (`1` with dedicated amber treatment).
- **Search & Filter Controls:** Search input for tracking numbers or cities; status filter buttons (`All`, `In Transit`, `Delivered`, `Action Needed`); Batch CSV upload placeholder button.
- **Refined Orders Table:** Full shipment list with monospace tracking number, recipient, weight (right-aligned tabular numbers with unit separation), status pill badges with colored status dots (`.ss-badge-dot`) plus text labels, tabular dates, and touch-friendly details action (`ss-btn-sm`).
- **Zero-Match Result:** Friendly empty state with package icon and quick "Reset filters" button when search or filters return 0 rows.
- **Clean Application Presentation:** Reviewer evaluation chrome, forced state toggles, and fake live connection pulses have been removed in favor of a clean, production-styled shipping application with a single subtle **"Sample data"** label in the top bar.

---

## 6. Demo Mock Indicators & Invariant Protection

All preview components, prototypes, and demo showcases are explicitly annotated with visible demo markers (e.g., `[DEMO - MOCK]`, `[Preview Demo · Mock Data]`).
- **Strict Invariant:** Client-side tariff calculations or hardcoded tracking numbers never masquerade as real backend responses.
- **Test ID Separation:** All preview selectors are explicitly prefixed with `signature-*` or `preview-*` to prevent automated E2E tests from mistaking mock previews for verified production workflows.

---

## 7. Cross-Module Frontend Coordination

Before universal theme tokens are applied across shared application shell layouts (`src/app/`), the frontend team should agree on the following:

1. **Theme Selection:** Review Option 1 (Refined Dark), Option 2 (Modern Light), and Option 3 (Teal & Ivory) to decide on the default identity or provide a user-configurable theme toggle.
2. **Daylight vs Evening Operations:** If Option 3 is selected, agree whether Member 7's dispatch board will use the evening palette (`#102824` / `#193630`) while customer/merchant screens default to daylight ivory (`#F7F5EE` / `#FFFFFF`).
3. **Primary Action Color:** Confirm whether `brand-600` (`#2563eb`) or Deep Teal (`#0B5955`) serves as the universal primary action across all portals.
4. **Status Badge Matrix:** Align on the unified status colors (`IN_TRANSIT`, `DELIVERED`, `ATTEMPT_FAILED`) across customer tracking and dispatcher board tables.
5. **Shared Shell Layout (`DashboardLayout.tsx`):** Align on header bar height (`h-16`), user role badge styling, and mobile sidebar drawer styling.

The working preview is live at `/design-preview` for interactive team review.
