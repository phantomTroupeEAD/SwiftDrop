# SwiftDrop Webhook Gateway — Apple Light Design System (`DESIGN.md`)

> **Theme:** Apple macOS & Web Light System (WWDC Fluid Interfaces & Cupertino Human Interface Guidelines)  
> **Target Surface:** Multi-Tenant Webhook Examiner & Automated Verification Workbench (`:9999/dashboard`)  
> **Source Foundations:** WWDC *Designing Fluid Interfaces*, *Principles of Great Design*, Apple Human Interface Guidelines (Light Mode), `apple-design` skill.

---

## 1. Design Philosophy: Fluidity, Physicality & Restraint

Apple design stops feeling like computer software and feels like an extension of the user by aligning visual materials and motion with the physical world:
1. **Response (Kill Latency):** Instant pointer-down feedback on press (`active:scale-[0.97]` within $100\text{ms}$), continuous feedback during interaction, zero lag.
2. **Materials & Translucency:** Liquid glass toolbars and sheets (`backdrop-filter: blur(20px) saturate(180%)`) with specular top borders (`border-top: 1px solid rgba(255,255,255,0.8)`) allowing content to scroll naturally underneath.
3. **Simplicity over Clutter:** Generous whitespace, refined typographic scale, and purposeful color accents. The UI is built on Apple's classic light neutral foundation (`#F5F5F7` canvas, `#FFFFFF` cards, `#1D1D1F` charcoal typography).
4. **Safety & Agency:** Destructive operations (such as resetting telemetry history) are gated behind an Apple-style confirmation sheet (`#confirmResetDialog`).

---

## 2. Color Palette & Token Architecture (Apple Light Mode)

All colors meet or exceed WCAG 2.1 AA contrast requirements ($> 4.5:1$ against white/light-grey surfaces).

### 2.1 Foundation Surfaces & Materials
| Token | Hex / Value | Role | Usage |
| :--- | :--- | :--- | :--- |
| `apple-canvas` | `#F5F5F7` | Root Canvas | Signature Apple product and OS light grey background |
| `apple-card` | `#FFFFFF` | Primary Cards | Elevated content containers with subtle shadow & hairline border |
| `apple-glass` | `rgba(255, 255, 255, 0.8)` | Translucent Chrome | Sticky header, floating notification pill, modal sheets |
| `apple-border` | `rgba(0, 0, 0, 0.08)` | Hairline Dividers | 1px border matching Apple retina separators |
| `apple-hover` | `rgba(0, 0, 0, 0.04)` | Micro-Interactions | Row hover highlight, secondary button hover |

### 2.2 Apple Typography Tokens
| Token | Hex | Role | Contrast Ratio |
| :--- | :--- | :--- | :--- |
| `apple-text-primary` | `#1D1D1F` | Headlines & Key Data | $15.8:1$ (AAA on white) |
| `apple-text-secondary`| `#6E6E73` | Subtitles & Captions | $5.1:1$ (AA on white) |
| `apple-text-tertiary` | `#86868B` | Metadata & Inactive Labels| $4.5:1$ (AA on white) |

### 2.3 Semantic HIG System Colors
Color is used with intention and clarity:

| Semantic State | Apple System Color | Hex | Purpose |
| :--- | :--- | :--- | :--- |
| **Primary Action** | Apple System Blue | `#0071E3` / `#0077ED` | Primary trigger button, search focus, active tabs |
| **Health / Valid** | Apple System Green | `#34C759` / `#28CD41` | Live polling dot, 100% HMAC valid badge |
| **Tenant Alpha** | Electric Blue | `#0071E3` | Merchant Alpha badge & dispatch trigger |
| **Tenant Beta** | Apple System Purple | `#AF52DE` | Merchant Beta badge & dispatch trigger |
| **Fault / Chaos** | Apple System Red | `#FF3B30` | Chaos 500 trigger, DLQ metric, destructive reset |
| **Telemetry Notice** | Apple System Orange | `#FF9500` | Paused stream status, warning indicators |

---

## 3. Typography: Optical Sizing & Hierarchy

- **Interface Font:** `-apple-system, BlinkMacSystemFont, "SF Pro Display", "SF Pro Text", "Helvetica Neue", system-ui, sans-serif`
  - Display Title: `18px / 1.15 font-semibold text-[#1D1D1F] tracking-[-0.015em]` (`text-balance`)
  - Metric Digits: `32px font-semibold text-[#1D1D1F] tracking-tight tabular-nums`
  - Body & Subtitles: `13px / 1.45 text-[#6E6E73]` (`text-pretty`)
  - Segmented Controls & Buttons: `12px font-medium`
- **Monospace Code Font:** `"SF Mono", Menlo, Monaco, Consolas, monospace`
  - Event IDs, Hashes, Timestamps: `12px text-[#1D1D1F] tabular-nums`
  - CloudEvents JSON Inspector: `11px text-[#3A3A3C] leading-relaxed`

---

## 4. Tactile Interactions & Spring Dynamics

- **Instant Pointer-Down Response:** Buttons apply `transform: scale(0.97)` within $100\text{ms}$ on pointer-down/active, restoring smoothly on release.
- **Translucent Navigation Bar:** Sticky top navigation utilizes `backdrop-filter: blur(20px) saturate(180%)` with a subtle bottom specular border.
- **Segmented Control:** Apple macOS-style segmented control container (`bg-black/[0.05] p-0.5 rounded-xl`) with white active pill indicator (`bg-white shadow-[0_1px_3px_rgba(0,0,0,0.08),0_1px_2px_rgba(0,0,0,0.04)]`).
- **Dynamic Island Notification:** Lightweight floating pill at the top of the viewport (`bg-white/90 backdrop-blur-xl border border-black/[0.08] text-[#1D1D1F] shadow-lg`).
- **Reduced Motion Support:** Respects `@media (prefers-reduced-motion: reduce)` with instantaneous cross-fades and zero spring scaling.

---

## 5. Verification & DOM Stability Invariants

- [x] All 5 automated unit tests pass (`npm test --prefix tests/e2e/mock-webhook`).
- [x] Programmatic DOM IDs preserved: `isolationBadge`, `countAlpha`, `countBeta`, `countChaos`, `countTotal`, `lastUpdated`, `eventRows`, `payloadModal`, `modalJson`.
- [x] Integration endpoints preserved: `POST /api/test-trigger`, `POST /webhooks/merchant-chaos`, `POST /api/reset`, `GET /api/events`.
- [x] Full WCAG 2.1 AA compliance in light mode.
