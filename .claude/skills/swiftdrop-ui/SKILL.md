---
name: swiftdrop-ui
description: Build, restyle or review any SwiftDrop screen under web/ (admin, courier, dispatcher, customer, merchant). Use before writing UI code, so the screen follows the SwiftDrop design system, passes npm run lint:ui, and handles every required state.
---

# SwiftDrop UI skill

Follow this every time you create, change or review a screen in `web/`. It points to the design documents instead of copying them. When this skill and a source document disagree, the source document wins; say so and fix this skill in the same PR.

## 1. Read these first

| File | What it is for |
| :-- | :-- |
| `AGENTS.md` (section "React & TypeScript Frontend", Rules 1 to 9) | Binding frontend rules. Rule 4 freezes `web/design.md`, `web/tailwind.config.js`, `web/src/index.css`, `web/components.json` and every primitive in `web/src/shared/components/ui/`. Rule 9 freezes the legacy ledger. |
| `web/design.md` | **Canonical design spec.** Tokens, role palettes, radii, typography, component rules, UI states, banned patterns. Do not edit it unless a human explicitly asks. If it conflicts with `web/public/scratch-apple.html`, stop and ask. |
| `web/design.mdx` | **Process.** Action rules (section 2), role profiles (section 4), the screen brief (section 5), the review checklist (section 7). Its older shared-colour and role-palette tables are superseded by `design.md`; where numbers differ, `design.md` wins. |
| `web/test-ids.md` | Stable test selectors. Add or update an entry when a testable control changes. |
| `web/README.md`, `web/API.md` | Routes, role access, consumed API contracts. |
| `web/scripts/lint-ui.mjs` | The guardrails CI enforces (see section 3). |
| `core-api/internal/<module>/API.md` | The backend contract for the screen's data. Check it before building. |

## 2. Role presets

Put `data-role="ADMIN"` (or the role) on the screen's root wrapper (AGENTS.md Rule 6). The tokens in `web/src/index.css` then cascade. Never write the hex values in feature code; use `bg-primary`, `text-primary`, `ring-ring` and the other token classes.

| Role | Primary | Hover | Tint |
| :-- | :-- | :-- | :-- |
| `ADMIN` | `#18181b` (dark mode `#e4e4e7`) | `#27272a` | `rgba(24,24,27,.08)` |
| `COURIER` | `#b45309` | `#92400e` | `rgba(180,83,9,.08)` |

Other roles for reference: `PUBLIC` and `CUSTOMER` `#0071e3`, `MERCHANT` `#0b5955`, `DISPATCHER` `#1e40af`. Primary foreground is white. Check `design.md` section 3 for the current values. Use the role colour only for the main action, focus ring and active selection (colour budget about 3 percent). Status badges keep their own colours and never take the role colour.

## 3. Hard rules (CI fails on these)

`npm run build` runs `npm run lint:ui`, which fails on any of the following in feature code (everything outside `src/shared/components/ui/`):

- Raw `<button>`, `<input>`, `<select>`, `<textarea>`, `<table>` or `<dialog>`. Use the primitives.
- Direct `@radix-ui/*` imports, or adding or removing a Radix dependency.
- Raw Tailwind palette classes (`bg-blue-600`, `text-rose-400`), arbitrary brackets (`bg-[#fff]`, `p-[19px]`, `shadow-[...]`), raw hex codes, and inline colour styles.

Also banned by `design.md`: any shadow (`shadow-*`, blur radius above 0; depth comes from 1px borders and tone steps), saturated surface fills, custom button classes, emoji or cartoon art in empty states, icon libraries other than `lucide-react`.

**Missing-component protocol:** if the screen needs a component, variant or state that is not in `@/shared/components/ui/`, stop, do not improvise or inline-style a substitute, never run `npx shadcn add`, and flag it in the PR description. Never add files to the legacy ledger in `scripts/lint-ui.mjs` or `.eslintrc.cjs`; entries may only be removed by migrating a screen.

## 4. Spacing, type, shape, touch

From `design.md` sections 2, 5 and 6; confirm there before relying on a number.

- **Type:** page title 32px/700, section heading 19px/600, card title 15px/600, body 14px, controls 13.5px, table headers and captions 12px. Tracking numbers, weights, money, timestamps and coordinates use `font-mono` with tabular numerals.
- **Spacing:** page container max 1200px (fluid wider screens are allowed per the decision log), padding 32px 40px desktop and 20px 16px on phones; grid gap 20px; form rows 16px; label to input 6px.
- **Radii:** buttons, inputs, selects, textarea, badges 8px; cards, tables, popovers 12px; dialogs and sheets 16px; pills and dots full.
- **Touch targets:** primary controls must be at least 44px high on phones (below 900px) and always on courier screens. Use the `touch:` variants (`touch:min-h-[44px]` is already built into the Button and Input primitives).
- **Icons:** `lucide-react`, stroke width 2, size 15px in nav and actions, 22px in card headers, 24px in empty states.
- **Phones:** single column, at least 16px side padding at 375px, wide tables in their own scroll area, no page-level horizontal scroll.

## 5. Shared components to reuse

Import from `@/shared/components/ui/`. Available today: `accordion`, `avatar`, `badge` (variants `neutral`, `active`, `success`, `warning`, `error`, `showDot`), `button` (variants `default`, `secondary`, `outline`, `ghost`, `destructive`, `link`; sizes `default`, `sm`, `lg`, `icon`), `card`, `checkbox`, `dialog`, `dropdown-menu`, `input`, `label`, `milestone-rail`, `native-select`, `popover`, `scroll-area`, `select`, `separator`, `skeleton`, `sonner` (toasts), `stepper`, `switch`, `table`, `tabs`, `textarea`, `tooltip`. Helper: `@/shared/lib/utils` (`cn`). Read a primitive's file before using it; do not assume props.

Status words and badge variants: `DELIVERED`, `VERIFIED`, `ACTIVE` success; `ASSIGNED`, `PICKED_UP`, `IN_TRANSIT`, `OUT_FOR_DELIVERY` active; `ATTEMPT_FAILED`, `DELAYED` warning; `RETURNED`, `CANCELLED`, `LOST`, `SUSPENDED` error; `CREATED`, `PENDING_QUOTE`, `DRAFT` neutral. Always show a text label, not colour alone.

Our own screens live in `web/src/features/dispatcher-admin/` (admin console, zones, later courier and dispatcher). Shell, routes and shared UI belong to the shell owner (`src/app`, `src/shared`): do not edit `App.tsx` routes unless asked.

## 6. Required states for every data screen

| State | What to show |
| :-- | :-- |
| Loading | Skeleton that keeps the layout, plus a polite status announcement. A mutation shows "Saving..." (the active verb) on the button and blocks a second submit. |
| Empty | Centred panel: 24px Lucide icon, bold 15px title ("No orders yet"), one sentence of explanation, one primary action. A filtered empty list says "No ... match your filters" with **Reset filters**. |
| Validation | Message beside the field saying what to fix. Keep what the user typed. |
| Error | Inline alert with an `AlertCircle` icon and a remedy ("Couldn't load zones. Try again."), a **Retry** button for transient failures, and the server's **reference id** (`correlation_id`) when the response has one. Never show tokens, SQL or stack traces. |
| Success | Confirm the actual result near the action ("Zone ZONE_GALLE_01 created."). A toast may reinforce it. |
| Disabled and focus | Disabled keeps its size. The dual focus ring is never hidden. |
| Motion | 150 to 220ms, opacity or small movement only, and nothing animated when `prefers-reduced-motion` is set. Keep status text visible without the animation. |

Data rules: use real API data only; label any sample data as sample; never invent numbers or fees; the frontend never computes fees or parcel transitions; money is integer LKR cents formatted for display only; access tokens stay in memory, never in `localStorage`.

## 7. Screen brief (fill in before writing UI code)

Post this in the PR description or issue (from `design.mdx` section 5):

```text
User role and preset:
Screen / route:
Feature owner or reviewer:
User goal and allowed actions by workflow state:
Shared components and role variants:
Desktop and phone layout (navigation, spacing, table or card display):
Primary / secondary / destructive actions and confirmation:
Loading / empty / validation / error / success messages:
Animation and reduced-motion behavior:
API, permissions, and new or updated test IDs:
```

Use verb-and-object action labels ("Save changes", "Disable zone"). One primary action per decision area. A destructive action names its target and explains the consequence, with a clear cancel. Show a state-changing action only when the role, the workflow state and the backend contract allow it.

## 8. Finish checklist

Run from `web/`: `npm run lint:ui`, then `npx tsc --noEmit`. Then confirm:

- [ ] The root wrapper has `data-role` and the intended preset; no hex, palette class, arbitrary value, shadow or raw element in the feature code.
- [ ] Only `@/shared/components/ui/` primitives; any missing component is flagged in the PR instead of improvised.
- [ ] The next action is obvious for the signed-in role and the current state.
- [ ] Loading, empty, validation, error (with reference id and Retry) and success states exist.
- [ ] Keyboard works, focus is visible, dialogs are named, close on Escape and return focus to the opener.
- [ ] Fits 375px with no page-level horizontal scroll; primary touch targets are at least 44px.
- [ ] Real API data only; sample data labelled; the contract in `API.md` matches what the screen sends and reads.
- [ ] `web/test-ids.md` and `web/README.md` updated for new controls, states or routes.
- [ ] No edits to frozen files (`design.md`, `index.css`, `tailwind.config.js`, `components.json`, `ui/*`) and no additions to the legacy ledger.
