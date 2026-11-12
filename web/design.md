# SwiftDrop UI Design System Specification (`design.md`)

> **Status:** Canonical Living Specification | **Visual Origin (Frozen):** `web/public/scratch-apple.html`  
> **Authority:** Layer 2 Architecture & Domain Contract. Binding on all human developers and AI coding agents.  
> **Modification Governance:** Modified **strictly by explicit human instruction**. Never self-extended or altered by agents. If this document and `scratch-apple.html` ever conflict, stop and request clarification.

---

## 1. Architectural Invariants & Core Philosophy

1. **Primitive Substrate:** Built exclusively on `@/shared/components/ui/` using shadcn/ui primitives rethemed to Apple Human Interface Guidelines (HIG). Feature code **must never** import `@radix-ui/*` directly or write ad-hoc interactive primitives.
2. **Color Budget $\le$ 3%:** Color is strictly reserved for actionable primary controls, data status indicators, and active selection state. Page canvases and card surfaces remain pure neutrals.
3. **Zero-Elevation Invariant:** No component or overlay possesses rest blur shadows (no `shadow-sm`, `shadow-md`, `shadow-lg`, or blur radius $> 0\text{px}$). Depth is achieved exclusively via 1px hairline borders (`var(--border)`) and tone steps (`var(--surface-subtle)`). All shadcn/ui default drop-shadows are strictly stripped.
4. **Canonical Archetype:** **Apple Cupertino Precision** (`[data-style="apple"]`). All other archetypes in `scratch-apple.html` (`linear`, `framed`, `swiftsig`) and demo palette swatches (`Emerald Green`, `Crimson Rose` outside errors) are demo-only and **strictly forbidden** in product code.
5. **Reconciled Effective Radii:** The product ships with Apple HIG radii:
   - Controls / Badges / Tags / Inner Affixes: `8px` (`--radius-sm`)
   - Inputs / Cards / Buttons / Tables: `12px` (`--radius-md` / `--radius`)
   - Modals / Sheets / Dialogs / Popovers: `16px` (`--radius-lg`)
   - Pills / Status Dots / Avatars: `9999px` (`--radius-full`)
6. **Missing-Component Protocol:** If a required component, variant, or state is missing from `@/shared/components/ui/`, **stop and flag it in the pull request description**. Never improvise, never inline-style a substitute, and never execute `shadcn add` without explicit permission.

---

## 2. Design Tokens Reference

### 2.1 Base Neutrals & Surfaces (Apple Archetype)

| CSS Variable | Tailwind Utility | Light Value | Dark Value (`[data-theme="dark"]`) | Usage |
| :--- | :--- | :--- | :--- | :--- |
| `--canvas` | `bg-canvas` | `#f5f5f7` | `#000000` | Application background behind cards and sidebars |
| `--surface` | `bg-surface` | `#ffffff` | `#1c1c1e` | Card panels, dialog surfaces, inputs, topbars |
| `--surface-subtle` | `bg-surface-subtle` | `#fbfbfd` | `#2c2c2e` | Secondary button fills, table headers, hover states |
| `--surface-active` | `bg-surface-active` | `#e2e8f0` | `#27272e` | Pressed items, selected table rows |
| `--border` | `border-border` | `rgba(0, 0, 0, 0.08)` | `rgba(255, 255, 255, 0.10)` | Hairline dividers, card outlines, table separators |
| `--border-subtle` | `border-border-subtle` | `#edf2f7` | `#1e1e24` | Inner nested cell dividers |
| `--text` | `text-foreground` | `#0f172a` | `#f8fafc` | Primary typography, headers, table figures |
| `--text-muted` | `text-muted-foreground` | `#64748b` | `#94a3b8` | Form labels, helper captions, timestamp units |
| `--text-subtle` | `text-subtle` | `#94a3b8` | `#64748b` | Disabled placeholders, inactive icons |
| `--ring` | `ring-ring` | Scoped by role | Scoped by role | Keyboard focus ring (`0 0 0 2px var(--surface), 0 0 0 4px var(--ring)`) |

### 2.2 Reconciled Apple Radii

| CSS Variable | Tailwind Utility | Exact Value | Usage |
| :--- | :--- | :--- | :--- |
| `--radius-sm` | `rounded-sm` | `8px` | Badges, small buttons (`btn-sm`), affixes, tooltips |
| `--radius-md` | `rounded-md` | `12px` | Inputs, default buttons, cards, table wrappers |
| `--radius-lg` | `rounded-lg` | `16px` | Dialog modals, drawer sheets, alert containers |
| `--radius-full` | `rounded-full` | `9999px` | Status dot indicators, toggle thumbs, circular avatars |

### 2.3 Motion & Transition Tokens

| CSS Variable | Exact Value | Usage |
| :--- | :--- | :--- |
| `--ease-out` | `cubic-bezier(0.16, 1, 0.3, 1)` | Micro-interactions, button active press, dropdown openings |
| `--ease-in-out` | `cubic-bezier(0.65, 0, 0.35, 1)` | Shimmer sweeps, accordion collapsibles |
| `--duration-fast` | `150ms` | Hover states, tab switches, toggle flips, tooltip fades |
| `--duration-normal`| `220ms` | Modal zoom-ins, toast slide-ins, accordion expansions |
| `--duration-slow`  | `350ms` | Sheet drawer entrances, full-page transitions |

> **Motion Guardrail:** Springs, bounces, and physics-based overshooting are strictly forbidden. When `prefers-reduced-motion: reduce` is active, all durations collapse to `0.01ms`.

---

## 3. Canonical Role Palette Presets (`[data-role]`)

Role palettes are scoped via the `[data-role="..."]` attribute on layout shells (`AppLayout` or page roots). Primary actions, focus rings, and active states automatically inherit these tokens.

| Role Identifier | Role Scope Selector | Primary Hex (`--primary`) | Hover Hex (`--primary-hover`) | Tint RGBA (`--primary-tint`) | Primary Foreground | Usage / Context |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Public / Visitor**| `[data-role="PUBLIC"]` | `#0071e3` | `#0077ed` | `rgba(0, 113, 227, 0.08)` | `#ffffff` | Public tracking, login, shared landing |
| **Customer** | `[data-role="CUSTOMER"]` | `#0071e3` | `#0077ed` | `rgba(0, 113, 227, 0.08)` | `#ffffff` | Consumer booking, quote confirmation |
| **Merchant** | `[data-role="MERCHANT"]` | `#0b5955` | `#08433f` | `rgba(11, 89, 85, 0.08)` | `#ffffff` | Bulk shipping, API keys, webhook center |
| **Dispatcher** | `[data-role="DISPATCHER"]` | `#1e40af` | `#1e3a8a` | `rgba(30, 64, 175, 0.08)` | `#ffffff` | Routing console, manual courier assignment |
| **Courier** | `[data-role="COURIER"]` | `#b45309` | `#92400e` | `rgba(180, 83, 9, 0.08)` | `#ffffff` | Mobile delivery runs, barcode scanner, POD |
| **Admin** | `[data-role="ADMIN"]` | `#18181b` | `#27272a` | `rgba(24, 24, 27, 0.08)` | `#ffffff` | Tariff management, user roles, system config |

---

## 4. Status Badges & Lifecycle Indicators

Colors in status indicators are decoupled from role themes and represent authoritative domain aggregate states. Every badge requires a 6px circular indicator dot.

| Semantic State | Foreground Ink | Background Fill | Border Rule | Domain Lifecycle Statuses Represented |
| :--- | :--- | :--- | :--- | :--- |
| **Neutral** | `#64748b` (`var(--text-muted)`) | `#f1f5f9` (`var(--surface-subtle)`)| `1px solid var(--border)` | `CREATED`, `PENDING_QUOTE`, `DRAFT` |
| **Active / Progress** | Role `--primary` | Role `--primary-tint` | `1px solid transparent` | `ASSIGNED`, `PICKED_UP`, `IN_TRANSIT`, `OUT_FOR_DELIVERY` |
| **Success** | `#047857` (`--status-success-ink`) | `#ecfdf5` (`--status-success-bg`) | `1px solid #a7f3d0` | `DELIVERED`, `VERIFIED`, `ACTIVE`, `PAYMENT_CLEARED` |
| **Warning** | `#b45309` (`--status-warning-ink`) | `#fffbeb` (`--status-warning-bg`) | `1px solid #fde68a` | `ATTEMPT_FAILED`, `DELAYED`, `EXPIRING_SOON` |
| **Error / Danger** | `#b91c1c` (`--status-error-ink`) | `#fef2f2` (`--status-error-bg`) | `1px solid #fecaca` | `RETURNED`, `CANCELLED`, `LOST`, `SUSPENDED` |

Dark mode automatically swaps ink and background variables per `scratch-apple.html` lines 91–99.

---

## 5. Typography, Spacing & Tap Targets

### 5.1 Typography Scale
- **Display / Headers:** `-apple-system, BlinkMacSystemFont, "SF Pro Display", "SF Pro", "Segoe UI", sans-serif`
  - Page Titles: `32px` (`text-3xl`), font-weight `700`, letter-spacing `-0.03em`, line-height `1.2`
  - Section Headlines: `19px` (`text-lg`), font-weight `600`, letter-spacing `-0.02em`
  - Card Titles: `15px` (`text-base`), font-weight `600`, letter-spacing `-0.01em`
- **Body & Controls:** `-apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro", "Segoe UI", sans-serif`
  - Body Text: `14px` (`text-sm`), font-weight `400` / `500`, line-height `1.5`
  - Controls / Inputs: `13.5px`, font-weight `500`
  - Table Headers / Captions: `12px` (`text-xs`), font-weight `500`
- **Tabular Numerals (Strict Requirement):**
  - Tracking numbers, weights, currencies, timestamps, and coordinates **must** use `font-mono` (`SF Mono`, monospace) with `font-variant-numeric: tabular-nums`.

### 5.2 Spacing & Layout Rhythm
- **Page Container:** Max-width `1200px`, padding `32px 40px 100px` (desktop), `20px 16px 80px` (mobile).
- **Component Gaps:** Grid gap `20px` (`grid-2`), form row margin `16px`, label-to-input gap `6px`.
- **Minimum Tap Target:** Primary interactive controls on mobile viewports (`max-width: 900px`) and all courier interfaces **must** have a touch target height of at least **`44px`** (or `min-h-[44px]`).

---

## 6. Rethemed shadcn/ui Component Specifications

All components reside in `@/shared/components/ui/`. Feature code imports from this directory only.

### 6.1 Button (`@/shared/components/ui/button.tsx`)
- **Base shadcn:** Radix Slot wrapper with class-variance-authority.
- **Exact Overrides from Scratch:**
  - Active press compression: `active:scale-[0.98] transition-transform duration-100 ease-out`
  - Default radius: `rounded-[12px]` (sm: `rounded-[8px]`)
  - Min heights: Default `38px` (desktop) / `44px` (touch targets), Small `30px`.
  - Zero shadows: replace `shadow` / `shadow-sm` with `border border-transparent`.
- **Variants:**
  - `primary`: `bg-[var(--primary)] text-[var(--primary-foreground)] hover:bg-[var(--primary-hover)]`
  - `secondary` (`tone`): `bg-[var(--surface-subtle)] text-[var(--text)] hover:bg-[var(--surface-active)]`
  - `outline`: `bg-transparent text-[var(--text)] border-[var(--border)] hover:bg-[var(--surface-subtle)]`
  - `ghost`: `bg-transparent text-[var(--text)] hover:bg-[var(--surface-subtle)]`
  - `destructive`: `bg-[var(--status-error-bg)] text-[var(--status-error-ink)] border-[var(--status-error-border)]`
  - `link`: `text-[var(--primary)] underline-offset-4 hover:underline p-0 h-auto`
- **Loading State:** Must accept `isLoading?: boolean`. When true, disables button, renders `<Loader2 className="w-4 h-4 animate-spin mr-2" />`, and displays `"Processing..."` or action label.

### 6.2 Card & Panels (`@/shared/components/ui/card.tsx`)
- **Base shadcn:** Div wrapper with header, title, description, content, footer.
- **Exact Overrides from Scratch:**
  - Remove `shadow-sm` / `shadow-md`.
  - Border: `1px solid var(--border)` (`border-[var(--border)]`).
  - Radius: `rounded-[12px]`.
  - Padding: `p-[22px]` (compact: `p-4`).
  - Background: `bg-[var(--surface)]`. Alternate flat panel: `bg-[var(--surface-subtle)] border-none`.

### 6.3 Input & Select (`@/shared/components/ui/input.tsx`, `select.tsx`)
- **Exact Overrides from Scratch:**
  - Height: `38px` (`h-[38px]`, courier touch: `h-[44px]`).
  - Border: `1px solid var(--border)`.
  - Focus ring: `outline-none focus:border-[var(--ring)] focus:ring-2 focus:ring-[var(--surface)] focus:ring-offset-2 focus:ring-offset-[var(--ring)]`.
  - Radius: `rounded-[8px]`.
  - Font size: `13.5px`.
  - Affix support: Inputs with left/right icons must set `pl-[38px]` / `pr-[38px]` with absolute centered icons.

### 6.4 Status Badge (`@/shared/components/ui/badge.tsx`)
- **Exact Overrides from Scratch:**
  - Padding: `py-[2px] px-[8px]`.
  - Radius: `rounded-[8px]`.
  - Font: `text-[12px] font-medium leading-[1.35] whitespace-nowrap`.
  - Structure: Flex container with child `<span className="w-1.5 h-1.5 rounded-full bg-current" />` dot followed by uppercase status string.

### 6.5 Data Table (`@/shared/components/ui/table.tsx`)
- **Exact Overrides from Scratch:**
  - Wrapper: `border border-[var(--border)] rounded-[12px] overflow-x-auto`.
  - Header: `bg-[var(--surface-subtle)] text-[var(--text-muted)] text-[12px] font-medium py-[10px] px-[16px] border-b border-[var(--border)]`.
  - Cells: `py-[12px] px-[16px] border-b border-[var(--border-subtle)] text-[13px] text-[var(--text)]`.
  - Row Hover: `hover:bg-[var(--surface-subtle)] transition-colors duration-150`.
  - Numeric columns (weights, fees, tracking IDs): Right-aligned with `tabular-nums font-mono`.

### 6.6 Dialog / Modal (`@/shared/components/ui/dialog.tsx`)
- **Exact Overrides from Scratch:**
  - Backdrop: `fixed inset-0 bg-black/45 z-50 flex items-center justify-center p-4 transition-opacity duration-220`.
  - Dialog Card: `bg-[var(--surface)] border border-[var(--border)] rounded-[16px] max-w-[500px] w-full transform scale-95 data-[state=open]:scale-100 transition-transform duration-220 ease-[cubic-bezier(0.16,1,0.3,1)]`.
  - Shadows: **Zero blur shadows** (strip `shadow-lg` / `shadow-2xl`).
  - Structure: Header (`p-[20px_22px_14px] border-b border-[var(--border)]`), Body (`p-[20px_22px]`), Footer (`p-[14px_22px] bg-[var(--surface-subtle)] border-t border-[var(--border)] flex justify-end gap-[10px]`).

### 6.7 Toast Notifications (`@/shared/components/ui/toast.tsx` / Sonner)
- **Exact Overrides from Scratch:**
  - Position: Bottom-right (`bottom-6 right-6`).
  - Card: `bg-[var(--surface)] border border-[var(--border)] rounded-[8px] p-[12px_18px] flex items-center gap-3`.
  - Shadows: **Strip all blur shadows**.
  - Slide-in transition: `translate-y-4 data-[state=open]:translate-y-0 opacity-0 data-[state=open]:opacity-100 transition-all duration-220 ease-[cubic-bezier(0.16,1,0.3,1)]`.

### 6.8 Logistics Specialized Primitives
- **Milestone Rail (`@/shared/components/ui/milestone-rail.tsx`):**
  - Vertical rail line: `1px solid var(--border)` positioned at `left: 6px`.
  - Milestone pin: `14px` circle, `1px solid var(--text-muted)` at rest; active: `bg-[var(--surface)] border-[var(--primary)] ring-4 ring-[var(--primary-tint)]`; done: `bg-[var(--primary)] border-[var(--primary)]`.
- **4-Step Wizard Stepper (`@/shared/components/ui/stepper.tsx`):**
  - Horizontal track line: `1px solid var(--border)` at `top: 13px`.
  - Node dot: `26px` circle; completed: `bg-[var(--primary)] text-white`; active: `bg-[var(--surface)] border-[var(--primary)] text-[var(--primary)] ring-4 ring-[var(--primary-tint)]`.

---

## 7. Universal UI State Machine & Patterns

Every feature screen and interactive component must explicitly handle the standard 6 states:

| UI State | Mandatory Representation Pattern |
| :--- | :--- |
| **Hover** | Background transition to `--surface-subtle` or `--primary-hover` over `150ms ease`. Scale up `1.15` on color swatches only. |
| **Active / Press** | Micro-compression `scale(0.98)` on buttons. Active indicators display `box-shadow: 0 0 0 3px var(--primary-tint)` (0 blur). |
| **Focus** | Dual accessibility ring: `box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px var(--ring)`. Never hide focus rings. |
| **Disabled** | `opacity-45 cursor-not-allowed pointer-events-none`. Retain button dimensions. |
| **Loading (Button)** | Button disabled, text changes to active verb (`"Saving..."`, `"Booking..."`), accompanied by animated spinner. |
| **Loading (Page / Table)**| **Shimmer Skeleton:** Animated linear gradient sweep over `--surface-subtle`: `linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.12) 50%, transparent 100%)` with `1.6s infinite ease-in-out`. Never show jarring full-page blank screens. |
| **Error (Inline)** | Red border (`--status-error-border`), light pink tint background (`--status-error-bg`), accompanied by `AlertCircle` icon and actionable remedy text. |
| **Empty State** | Centered layout inside `card-panel`, muted `Package` or `Search` icon (24px, stroke 1.5), bold 15px title (`"No orders found"`), descriptive explanation, and a single primary action button (`"Create Booking"` or `"Reset Filters"`). |

---

## 8. Icon System Rules (Lucide)

1. **Approved Library:** `lucide-react` is the sole authorized icon library.
2. **Standard Sizing:**
   - Nav items & Actions: `15px x 15px` (`w-[15px] h-[15px]`)
   - Card headers & Logistics catalog: `22px x 22px` (`w-[22px] h-[22px]`)
   - Touch controls / Empty states: `24px x 24px`
3. **Stroke Width:** Strictly `2px` (or `strokeWidth={2}`). Thin 1px or bold 3px strokes are forbidden.
4. **Color Inheritance:** Icons must always use `currentColor` or `var(--primary)` to naturally inherit state transitions.

---

## 9. Negative Constraints & Prohibited Anti-Patterns

Any pull request containing the following patterns is subject to immediate rejection:

- ❌ **NO Custom Button Classes:** Never write `<button className="bg-blue-600 ...">`. Use `<Button variant="...">`.
- ❌ **NO Arbitrary Tailwind Shadows:** Classes `shadow-sm`, `shadow-md`, `shadow-lg`, `shadow-xl`, `shadow-2xl` are completely blacklisted.
- ❌ **NO Saturated Surface Fills:** Never use saturated background fills (e.g. `bg-purple-600`, `bg-blue-500`) on page layouts or cards.
- ❌ **NO Raw Hex / Arbitrary CSS Values:** Classes like `bg-[#0071e3]` or `text-[#123456]` are prohibited in feature code. Use token classes (`bg-[var(--primary)]` or `bg-surface`).
- ❌ **NO Unchecked Mobile Tap Targets:** Primary action controls with heights $< 44\text{px}$ on mobile viewports are prohibited.
- ❌ **NO Ad-Hoc Empty State Art:** Cartoon SVG vectors, emojis, or external illustrations are strictly forbidden. Use standard Lucide icons with muted typography.
