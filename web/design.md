# SwiftDrop UI Design System Specification (`design.md`)

> **Status:** Canonical Living Specification | **Visual origin:** the "SwiftDrop UI Redesign" design canvas (Foundations, Foundations: dark mode and UI states boards), agreed by the team on 2026-11-20. It replaces `web/public/scratch-apple.html`, which is kept only as history.
> **Authority:** Layer 2 Architecture & Domain Contract. Binding on all human developers and AI coding agents.
> **Modification Governance:** Modified **strictly by explicit human instruction**. Never self-extended or altered by agents. Record every change in the decision log at the end.

The look is called **Waybill**: paper and ink with one signal colour, borrowed from shipping labels. Tracking numbers, money and times are always set in mono.

---

## 1. Architectural Invariants & Core Philosophy

1. **Primitive Substrate:** Built on `@/shared/components/ui/` (shadcn/ui primitives rethemed to Waybill). Feature code **must never** import `@radix-ui/*` directly or write ad-hoc interactive primitives.
2. **Colour budget:** Signal orange is reserved for the main action in an area, the focus ring and the current step or selection. Pages and cards stay paper and white. Ink (`chrome`) is used for the ops sidebar, dark bands on public pages and label-card header strips.
3. **Zero-Elevation Invariant:** No blurred shadows anywhere (`shadow-sm` … `shadow-2xl` are banned). Depth comes from 1px borders and tone steps. 0-blur rings (focus ring, selected-row inset bar) are allowed.
4. **No glass or decoration effects:** no `backdrop-blur`, gradient washes, glow orbs or blurred background photos.
5. **Radii:** controls, badges, inputs, buttons, toasts `8px` (`--radius-sm`); cards, tables, popovers, dropdowns `12px` (`--radius-md`); dialogs and sheets `16px` (`--radius-lg`); dots, avatars, switch tracks full. Tooltips and inner tab triggers `4px`.
6. **Missing-Component Protocol:** If a required component, variant or state is missing from `@/shared/components/ui/`, **stop and flag it in the pull request description**. Never improvise, inline-style a substitute, or run `shadcn add` without explicit permission.

---

## 2. Design Tokens Reference

Tokens live in `web/src/index.css` (`:root` and `[data-theme="dark"]`) and are exposed to Tailwind in `web/tailwind.config.js`. Feature code uses the Tailwind token classes only; never hex values or bracket classes (`lint:ui` enforces this).

### 2.1 Surfaces, ink and lines

| CSS variable | Tailwind class | Light | Dark | Usage |
| :-- | :-- | :-- | :-- | :-- |
| `--canvas` | `bg-canvas` | `#f5f3ee` | `#121316` | Page background ("paper") |
| `--surface` | `bg-surface` | `#ffffff` | `#1c1e23` | Cards, inputs, header bars |
| `--surface-subtle` | `bg-surface-subtle` | `#faf9f6` | `#24272e` | Table headers, hover |
| `--surface-active` | `bg-surface-active` | `#ece9e2` | `#2a2d34` | Notes, neutral badges, pressed rows |
| `--border` | `border-border` | `#e2ded5` | `#33363e` | Card outlines, dividers |
| `--border-subtle` | `border-border-subtle` | `#efece6` | `#2a2d34` | Row dividers inside tables |
| `--border-strong` | `border-border-strong` | `#8f897c` | `#6b6f78` | Input, select, checkbox and outline-button borders (3:1 against surface) |
| `--text` | `text-text`, `text-foreground` | `#15171c` | `#f2efe8` | Primary text, ink |
| `--text-muted` | `text-text-muted` | `#4a4e57` | `#a9a49a` | Body copy, labels, captions |
| `--text-subtle` | `text-text-subtle` | `#5b5f68` | `#8c887f` | Placeholders, tertiary text |

### 2.2 Chrome (ink areas)

| CSS variable | Tailwind class | Light | Dark | Usage |
| :-- | :-- | :-- | :-- | :-- |
| `--chrome` | `bg-chrome` | `#15171c` | `#0b0c0e` | Ops sidebar, dark bands, label-card strips |
| `--chrome-foreground` | `text-chrome-foreground` | `#f5f3ee` | `#f2efe8` | Text on chrome |
| `--chrome-muted` | `text-chrome-muted` | `#c9c4b8` | `#a9a49a` | Secondary text on chrome |
| `--chrome-line` | `border-chrome-line` | `#3a3d45` | `#2a2d34` | Dividers on chrome |
| `--chrome-hover` | `bg-chrome-hover` | `#24272e` | `#1c1e23` | Hover on chrome nav items |
| `--chrome-accent` | `text-chrome-accent` | `#f08c5a` | `#f08c5a` | Eyebrows and role tags on chrome |

### 2.3 Signal

| CSS variable | Tailwind class | Light | Dark | Usage |
| :-- | :-- | :-- | :-- | :-- |
| `--primary` | `bg-primary`, `text-primary` | `#c2410c` | `#f08c5a` | Main action, focus ring, current step |
| `--primary-hover` | `bg-primary-hover`, `text-primary-hover` | `#9a3412` | `#f5a97f` | Hover; orange text on paper (eyebrows, links) |
| `--primary-foreground` | `text-primary-foreground` | `#ffffff` | `#15171c` | Text on a primary fill |
| `--primary-tint` | `bg-primary-tint` | `rgba(194,65,12,.10)` | `rgba(240,140,90,.14)` | Text selection, light emphasis |
| `--ring` | `ring-ring` | `#c2410c` | `#f08c5a` | Focus ring |

### 2.4 Radii and motion

| Variable | Value | Usage |
| :-- | :-- | :-- |
| `--radius-sm` | `8px` | Controls, badges, toasts, callouts |
| `--radius-md` | `12px` | Cards, tables, popovers |
| `--radius-lg` | `16px` | Dialogs, sheets |
| `--radius-full` | `9999px` | Dots, avatars, switch |
| `--ease-out` | `cubic-bezier(0.16, 1, 0.3, 1)` | Presses, menus |
| `--ease-in-out` | `cubic-bezier(0.65, 0, 0.35, 1)` | Shimmer, accordions |
| `--duration-fast` / `-normal` / `-slow` | `150ms` / `220ms` / `350ms` | Hover / dialogs, toasts / sheets |

> **Motion guardrail:** no springs or bounces. Under `prefers-reduced-motion: reduce` every animation and transition collapses to `0.01ms`; status text must stay readable without the animation.

### 2.5 Dark mode

Dark mode is switched on with `data-theme="dark"` on `<html>`. It is a separate set of steps, not an inverted light theme: warm charcoal instead of pure black, a lighter signal (`#f08c5a`) with ink text on it, and darker status tints. Every token above has a dark value; never write a light-only colour in feature code.

---

## 3. Role Scopes

Every layout root still carries `data-role="PUBLIC" | "CUSTOMER" | "MERCHANT" | "DISPATCHER" | "COURIER" | "ADMIN"` (AGENTS.md Rule 6), and dialogs rendered in a portal receive it on `<body>`.

**All roles use the same signal colour.** The role is shown as a label (the ops sidebar role tag and the `ops-role-badge`), not by recolouring the interface. To bring per-role colours back, override `--primary`, `--primary-hover`, `--primary-tint` and `--ring` per `[data-role]` in `index.css`; no component changes are needed.

---

## 4. Status Badges & Lifecycle Indicators

Status colours are independent of the brand colour. Every badge shows a 6px dot and an **uppercase text label** (colour is never the only signal). Use `<Badge variant="…">`.

| Variant | Ink / background / border tokens | Light ink on bg | Statuses |
| :-- | :-- | :-- | :-- |
| `neutral` | `text-muted` / `surface-active` / `border` | `#4a4e57` on `#ece9e2` | `CREATED`, `PENDING_QUOTE`, `DRAFT`, `PENDING` |
| `active` | `status-info-*` | `#1e40af` on `#e8eefc` | `ASSIGNED`, `PICKED_UP`, `IN_TRANSIT`, `OUT_FOR_DELIVERY` |
| `success` | `status-success-*` | `#17663f` on `#e3f1e8` | `DELIVERED`, `VERIFIED`, `ACTIVE`, `APPROVED` |
| `warning` | `status-warning-*` | `#8a4b00` on `#fbefd8` | `ATTEMPT_FAILED`, `DELAYED`, `EXPIRING_SOON` |
| `error` | `status-error-*` | `#a61b12` on `#fbe7e4` | `RETURNED`, `CANCELLED`, `LOST`, `SUSPENDED`, `REJECTED` |

Dark values: info `#8ab4ff` on `#1b2a4a`, success `#6fcf97` on `#173326`, warning `#f2c14e` on `#3a2c0e`, error `#ff8a80` on `#3f1a17`.

---

## 5. Typography, Spacing & Tap Targets

### 5.1 Fonts

Self-hosted through `@fontsource-variable/*` (imported in `src/main.tsx`), so the Content Security Policy needs no font hosts.

| Role | Family | Tailwind | Used for |
| :-- | :-- | :-- | :-- |
| Display | Space Grotesk | `font-display` (default for `h1`–`h3`) | Page titles, section and card headings |
| Text | Inter | `font-sans` (body default) | Body copy, labels, controls |
| Data | JetBrains Mono | `font-mono` | Tracking numbers, money, weights, times, ids, table headers, eyebrows |

### 5.2 Scale

- Landing display: 48 → 72px / 700, tight tracking. Page titles: 32px / 700, `-0.03em`. Section headings: 19–20px / 600. Card titles: 15px / 600.
- Body: 14px (ops) to 16px (public pages), line-height 1.5. Controls: 13.5px / 600 on buttons. Captions and table headers: 12px.
- Table headers and eyebrows: mono, uppercase, `0.06em` tracking.
- **Tabular numerals are required** for tracking numbers, weights, money, timestamps and coordinates (`font-mono`, `tabular-nums`). Money is formatted with `formatLKR(cents)`.

### 5.3 Spacing and layout

- Content container max `1600px`, side padding `16px` on phones, `32px` on tablets, `40–56px` on desktop.
- Grid gap `20–24px`; form rows `16px`; label to input `6px`.
- Ops screens: ink sidebar (`w-60`) from `lg` up, with a menu button below `lg`. Public pages: single column on phones, no page-level horizontal scroll at 375px.
- **Tap targets:** primary controls are at least **44px** high on touch devices (`touch:` variant) and always on courier screens.

---

## 6. Component Specifications

All components live in `@/shared/components/ui/`.

| Component | Waybill rules |
| :-- | :-- |
| **Button** | 8px radius, 600 weight, 38px (44px touch) default height, `active:scale-[0.98]`. Variants: `default` (signal fill), `outline` (white fill, `border-strong`), `secondary` (`surface-active`), `ghost`, `destructive` (error tint), `link`. One signal button per decision area; use `outline` or an ink treatment for strong secondary actions. |
| **Badge** | 8px radius, 11.5px / 600, uppercase, `0.04em` tracking, 6px dot for status variants. |
| **Input / Textarea / Select / Native select** | 8px radius, `bg-surface`, `border-border-strong`, 38px (44px touch) high, dual focus ring. Validation: error border plus a message beside the field that says how to fix it. |
| **Checkbox / Switch** | `border-border-strong` outline so the control is visible at 3:1. |
| **Card** | 12px radius, 1px `border`, `bg-surface`, padding 22px, no shadow. |
| **Table** | Wrapper 12px radius with border. Header row `surface-subtle`, mono uppercase 12px. Numeric columns mono and right-aligned. Row hover `surface-subtle`. Wide tables scroll inside their own box. |
| **Dialog** | 16px radius, 1px border, no shadow, max 500px; header and footer separated by borders; destructive dialogs name the target and the consequence and offer a clear cancel ("Keep pending"). |
| **Toast** | Bottom-right, 8px radius, 1px border, no shadow. |
| **Stepper** | Connected circles: done = ink fill with a tick, current = 2px signal ring, ahead = outlined; dashed connector ahead, solid behind. |
| **Milestone rail (`m6-timeline`)** | 2px ink rail, ink dots, mono timestamps. Public tracking shows status, time and city only (SEC-016). |
| **Label card** | The Waybill signature: 12px card with a 1px ink border, a `bg-chrome` header strip in mono caps, dashed (`border-dashed border-border-strong`) tear lines between sections, optional `waybill-barcode` and `waybill-route` decorations (always `aria-hidden`). Used for the tracking lookup, quotes and the courier's current stop. |

Global utility classes in `index.css`: `.focus-ring`, `.waybill-barcode`, `.waybill-route` with `.waybill-stop`, `.waybill-stop--next`, `.waybill-leg`, `.waybill-leg--ahead`.

---

## 7. Universal UI States

Every data screen handles these states (see the "UI states" board on the canvas):

| State | Pattern |
| :-- | :-- |
| **Hover** | `surface-subtle` (or `primary-hover` on signal buttons) over 150ms. |
| **Press** | `scale(0.98)` on buttons. |
| **Focus** | Dual ring `0 0 0 2px var(--surface), 0 0 0 4px var(--ring)`. Never hidden. |
| **Disabled** | `opacity: 0.45`, `cursor: not-allowed`, keeps its size and colour. |
| **Loading (button)** | Disabled, spinner, active verb ("Saving…", "Booking…"); blocks a second submit. |
| **Loading (page/table)** | Shimmer skeleton that keeps the layout, plus a polite `role="status"` announcement. |
| **Empty** | Centred in a card: 24px Lucide icon, bold 15px title, one sentence, one action. Filtered empty says "No … match your filters" with **Reset filters**. |
| **Error** | Inline alert with `AlertCircle`, a remedy, **Retry** for transient failures and the server's reference id (`correlation_id`) in mono. Conflicts (`409 CONCURRENT_MODIFICATION`) use the warning tint and say to reload. |
| **Success** | Confirm the actual result next to the action ("Zone ZONE_GALLE_01 created."); a toast may reinforce it. |
| **Live updates degraded** | Say that live updates are paused and how the page is updating instead. |

---

## 8. Icon System Rules (Lucide)

1. `lucide-react` is the only icon library.
2. Sizes: 15px in nav and actions, 22px in card headers, 24px in empty states and touch controls.
3. Stroke width 2. Icons inherit `currentColor`.
4. Decorative icons are `aria-hidden`; icon-only buttons have an `aria-label`.

---

## 9. Prohibited Patterns

- ❌ Custom button classes or raw `<button>`, `<input>`, `<select>`, `<textarea>`, `<table>`, `<dialog>` in feature code. Use the primitives.
- ❌ Blurred shadows, `backdrop-blur`, glass panels, gradient washes, glow orbs.
- ❌ Raw hex values, Tailwind palette classes (`bg-blue-600`) or bracket colour/size classes (`bg-[…]`, `text-[…]`) in feature code. Add a token instead.
- ❌ Colour-only status. Every status has a text label.
- ❌ Fabricated data on real pages. Sample data in a design or demo is labelled "Example" or "Sample".
- ❌ Primary controls under 44px on touch devices.
- ❌ Emoji, cartoon art or external illustrations in empty states.

---

## Appendix: Decision Log

### 2026-11-20 — Waybill redesign (team agreement; recorded by Member 9)
- **Adopted:** the Waybill look from the "SwiftDrop UI Redesign" canvas. It replaces the Apple Cupertino archetype and `scratch-apple.html` as the visual origin.
- **Tokens:** paper/ink neutrals, signal orange `#c2410c` (dark `#f08c5a`), new `chrome` family for ink areas, `border-strong` for control borders (3:1), new `status-info` family for in-progress statuses (badge `active` no longer uses the role colour).
- **Fonts:** Space Grotesk / Inter / JetBrains Mono, self-hosted with `@fontsource-variable` (no CSP change).
- **Role colours:** replaced by one signal colour for every role; roles are shown as labels. **Pending Design Owner confirmation**, because it supersedes the 2026-11-12 role palette decision below. Reverting is a token-only change (§3).
- **Kept from the previous spec:** zero blur shadows, 8/12/16 radii, 44px touch targets, dual focus ring, `opacity-45` disabled, uppercase status badges, mono tabular numerals, 1600px fluid container, reduced-motion rule.
- **Removed:** glassmorphism headers, blurred hero photos and glow orbs on the login page, invented marketing figures on the landing page.

### 2026-11-12 — §6.0 Effective Radii Dispositions (Design Owner) — *still in force*
- Select triggers, affix containers, textarea, callouts and default buttons `8px`; popover and dropdown `12px`; sheets and destructive dialogs `16px`; avatar full; tooltip and inner tab trigger `4px`.

### 2026-11-12 — Role Palettes & Architecture Decisions (Design Owner) — *role palettes superseded on 2026-11-20, pending confirmation*
- ~~Role palettes: MERCHANT teal `#0b5955`, DISPATCHER navy `#1e40af`, COURIER amber `#b45309`, ADMIN zinc `#18181b`, PUBLIC/CUSTOMER Apple blue `#0071e3`.~~
- 44px touch targets via the `touch:` variant — still in force.
- Dual focus ring as the global `:focus-visible` standard and `.focus-ring` utility — still in force.
- Opacity-slash colour classes replaced by explicit tokens in `ui/*.tsx` — still in force.
- Reduced-motion block in `index.css` — still in force.

### 2026-11-12 — Studio Visual Parity & Contrast Refinements (Design Owner)
- Explicit `text-text` on Input, Textarea and DialogContent — still in force.
- ~~Admin dark-mode zinc inversion~~ — superseded (single signal colour).
- Fluid container up to `1600px` — still in force.
