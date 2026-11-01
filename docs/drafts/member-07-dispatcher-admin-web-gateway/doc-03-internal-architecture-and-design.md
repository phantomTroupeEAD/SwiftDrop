# Document 3 — Internal Architecture & Design Specification

> **Status:** Draft / Proposed (For Review in Meeting 2)  
> **Author:** Member 7 — Dispatcher/Admin UI, Courier Web, Traefik Gateway  
> **Target Module / Location:** `web/src/features/dispatcher-admin/` (ops + courier screens), `core-api/internal/admin/` (admin module), `deploy/gateway/traefik/` (gateway)  
> **Key Upstream Dependencies:** Member 1 (Auth, RBAC, JWT), Member 2 (Parcels, Pricing), Member 3 (Dispatch, Delivery), Member 4 (Merchants), Member 5 (Tracking SSE), Member 8 (Outbox, events), Member 9 (Compose, Kubernetes)  
> **Open Questions / Needs Clarification:** U-28 to U-34 in section 9: ESLint boundary rules, shared Money type, audit writer owner, more depguard rules, zone events to outbox, route map location, version column on delivery_zones

**Module:** Dispatcher/Admin/Courier Frontend, Admin Backend Module & API Gateway (Member 7) **Project:** SwiftDrop Parcel Delivery & Courier Management System **Based on:** Document 1 v3 (scope), Document 2 v2 (flows), AGENTS.md, CLAUDE.md, web/CLAUDE.md, .cursor/rules/frontend.mdc, .golangci.yml, Engineering Standards v1.0 sections 2, 4, 10, 11, domain-boundaries.md, database-schema.md, ADR-001 to ADR-004, deploy/gateway/ **Status:** Draft v1. Items marked **\[TBD\]** wait for a teammate or a team decision. Open questions reuse U-01 to U-27 from Documents 1 and 2; new ones start at U-28 (section 9). **Date:** 24 October 2026

## 1. How to read this document

This document answers: **how should my module be built inside, and why?**

My module has three separate parts, so each part gets its own design section:

| Part | Where the code lives | Language | Section |
| --- | --- | --- | --- |
| Ops and courier screens | `web/src/features/dispatcher-admin/` | React + TypeScript | 3 |
| Admin backend module | `core-api/internal/admin/` | Go | 4 |
| Gateway configuration | `deploy/gateway/traefik/` | Traefik YAML | 5 |

Section 6 lists every design decision with the other options I looked at. Section 7 covers scaling and speed.

Every picture is shown as an image first. Under each image there is a "Diagram source" box with the Mermaid text, so anyone can edit it later.

Tags used: **\[Repo\]** = already written in the team repository. **\[Proposed\]** = my proposal, not yet agreed. **\[TBD\]** = not decided.

## 2. Big picture: components

The figure shows all parts of my module (green) and what they talk to. Arrows point from the caller to the thing it calls.

![Figure: components](doc3-images/d01-components.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart LR
    subgraph Browser["Browser or phone"]
        subgraph SPA["Shared web app (web/)"]
            SHELL["App shell, routes, guards (shared with M6)"]
            SAPI["Shared API client (shared with M6)"]
            OPS["Ops screens /ops/* (M7)"]
            COU["Courier screens /courier/* (M7)"]
        end
    end
    subgraph Edge["Edge"]
        CF["Cloudflare"]
        GW["Traefik gateway config (M7 routes, M9 install)"]
    end
    subgraph Core["Core API (Go modular monolith)"]
        ADM["admin module (M7, M1, M4)"]
        IDN["identity (M1)"]
        PAR["parcels and pricing (M2)"]
        DSP["dispatch and delivery (M3)"]
        MER["merchants (M4)"]
        PLT["platform: DB pool, auth, idempotency"]
    end
    TRK["Tracking API (M5)"]
    DB[("PostgreSQL")]
    OPS --> SAPI
    COU --> SAPI
    SHELL --> OPS
    SHELL --> COU
    SAPI --> CF --> GW
    GW --> Core
    GW --> TRK
    ADM --> PLT
    ADM -. "Go interfaces" .-> MER
    ADM -. "Go interfaces" .-> PAR
    ADM -. "Go interfaces" .-> DSP
    PLT --> DB
    classDef mine fill:#d9f2d9,stroke:#2e7d32,color:#111
    class OPS,COU,GW,ADM mine
```

</details>

**Key points**

- The screens never talk to the database or to other services directly. Everything goes through the shared API client, then the gateway, then the Core API (Doc 1 X-05).
- The admin module talks to other Core API modules only through **Go interfaces** in the same process. It never calls them over HTTP or RabbitMQ, and it never reads their tables (domain-boundaries 4.1 and 4.2) \[Repo\].
- The gateway only routes and limits. It never checks tokens (ADR-001) \[Repo\].

## 3. Frontend design (ops and courier screens)

### 3.1 Package structure

The top folders come from web/README \[Repo\]. The sub-folders inside each feature are my proposal \[Proposed\]. Every feature has the same inside layout so any developer knows where to look.

```
web/src/
├── app/                                  # shared with Member 6
│   ├── routes/opsRoutes.tsx              # lazy routes for /ops/* and /courier/* (mine, plugged into shared router)
│   ├── layouts/OpsLayout.tsx             # ops menu by role (usability only)
│   └── guards/RoleGuard.tsx              # shared
├── shared/                               # shared with Member 6
│   ├── api/
│   │   ├── client.ts                     # fetch wrapper: token, refresh, errors, Idempotency-Key
│   │   ├── refreshLock.ts                # single shared refresh (Doc 2 F-02)
│   │   ├── errors.ts                     # ApiError type and error envelope parsing
│   │   └── idempotency.ts                # key creation and reuse rules (Doc 2 3.3)
│   ├── types/                            # ParcelStatus, Role, ApiEnvelope, Cents
│   └── utils/formatLKR.ts, dates.ts      # money and Sri Lanka time display
└── features/dispatcher-admin/            # Member 7 only
    ├── dispatch-board/                   # F-03, F-04, F-05
    │   ├── api/dispatchApi.ts            # typed calls + DTO to view-model mapping
    │   ├── api/queryKeys.ts              # TanStack Query keys for this feature
    │   ├── hooks/useDispatchBoard.ts     # query hook with 30 s refresh
    │   ├── hooks/useAssignParcel.ts      # mutation hook with Idempotency-Key
    │   ├── components/                   # ParcelQueue, CourierRoster, AssignDialog (no API calls inside)
    │   ├── pages/DispatchBoardPage.tsx   # route page: joins hooks and components
    │   ├── schemas/                      # zod schemas for forms
    │   ├── types.ts                      # view models used by components
    │   └── __tests__/                    # Vitest + React Testing Library
    ├── failed-deliveries/                # F-06 [Proposed new folder]
    ├── fleet/                            # F-07
    ├── merchants/                        # F-08 [Proposed new folder]
    ├── users/                            # F-09 [Proposed new folder]
    ├── zones/                            # F-10
    ├── pricing/                          # F-11 [Proposed new folder]
    ├── reports/                          # F-12, F-13 (audit explorer inside reports, as web/README says)
    ├── corrections/                      # F-14 [TBD U-08]
    ├── courier-mobile/                   # C-01 to C-05
    │   ├── ...same inside layout...
    │   └── utils/resizeImage.ts          # photo resize before upload (Doc 2 C-04)
    └── shared-ops/                       # code used by 2+ of MY features only (status badge text, reason code labels)
```

Folders not in web/README today (`failed-deliveries`, `merchants`, `users`, `pricing`, `corrections`, `shared-ops`) must be added to web/README in the same change that creates them (AGENTS.md 6.3).

### 3.2 What each layer does

| Layer | Files | Does | Must not do |
| --- | --- | --- | --- |
| Route page | `pages/*Page.tsx` | Reads URL params, calls feature hooks, chooses which state to show (loading, empty, error, success), passes data to components | Call `fetch`; hold business rules |
| Presentational components | `components/*` | Draw the screen from props, raise events (onAssign, onSubmit) | Call the API; read tokens; compute prices or valid statuses |
| Feature hooks | `hooks/use*.ts` | Wrap TanStack Query: queries (read) and mutations (write). Make the Idempotency-Key, refresh related queries after success | Build URLs by hand; parse error bodies themselves |
| Feature API module | `api/*Api.ts` | One typed function per endpoint. Turns backend DTOs (snake\_case) into view models (camelCase) and back. Only place that knows URL paths | Hold state; show messages |
| Form schemas | `schemas/*` | zod rules for quick form hints (required, length, format) | Be treated as the real check (backend repeats it) |
| Shared API client | `shared/api/client.ts` | Adds `Authorization`, runs the single shared refresh on 401, sends `Idempotency-Key`, turns error envelopes into `ApiError` | Know any feature; store tokens anywhere but memory |
| Auth context | `app/providers/AuthProvider` | Keeps the access token and user role in memory | Use localStorage or sessionStorage (INV-11) |

![Figure: frontend layers](doc3-images/d02-frontend-layers.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart TD
    P["Route page"] --> H["Feature hooks (TanStack Query)"]
    P --> C["Presentational components"]
    H --> A["Feature API module"]
    A --> S["Shared API client"]
    S --> AU["Auth context (token in memory)"]
    S --> NET["fetch to /api/v1 via gateway"]
    C -. "events only (onAssign, onSubmit)" .-> P
    P --> Z["zod form schemas"]
```

</details>

### 3.3 Allowed imports (dependency direction)

| From | May import | May NOT import |
| --- | --- | --- |
| `features/dispatcher-admin/<feature>/components` | its own `types.ts`, `shared-ops/`, `src/shared/components`, `src/shared/utils` | its own `api/`, any `hooks/` that call the API, other features |
| `features/dispatcher-admin/<feature>/pages` | own `hooks/`, `components/`, `schemas/`, `types.ts`, `shared-ops/`, `src/shared/*` | other features' internals |
| `features/dispatcher-admin/<feature>/hooks` | own `api/`, `src/shared/api`, `src/shared/hooks` | components |
| `features/dispatcher-admin/<feature>/api` | `src/shared/api`, `src/shared/types` | React, components, hooks |
| any `dispatcher-admin` file | `src/shared/*`, `src/app/` route helpers | **anything in `features/customer-merchant/`** \[Repo\] web/CLAUDE.md 2 |
| `src/shared/*` | only `src/shared/*` and libraries | any `features/*` folder |

**How it is enforced \[Proposed\]:** an ESLint `no-restricted-imports` rule (or `eslint-plugin-boundaries`) in `npm run lint`, so a wrong import fails CI. The Go side already does the same with `depguard` \[Repo\] .golangci.yml. Adding the ESLint rule touches the shared lint config, so Member 6 must agree **\[TBD U-28\]**.

![Figure: frontend imports](doc3-images/d03-frontend-imports.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart LR
    subgraph M7["features/dispatcher-admin (M7)"]
        F1["dispatch-board"]
        F2["courier-mobile"]
        F3["zones, reports, ..."]
        SO["shared-ops"]
    end
    subgraph M6["features/customer-merchant (M6)"]
        G1["booking, tracking, ..."]
    end
    SH["src/shared (API client, types, components, utils)"]
    APP["src/app (shell, routes, guards)"]
    F1 --> SO
    F2 --> SO
    F3 --> SO
    F1 --> SH
    F2 --> SH
    F3 --> SH
    SO --> SH
    G1 --> SH
    APP --> F1
    APP --> F2
    APP --> F3
    APP --> G1
    F1 -. "NOT allowed" .-x G1
    G1 -. "NOT allowed" .-x F2
    SH -. "NOT allowed" .-x F1
```

</details>

### 3.4 Key interfaces (TypeScript)

These are the contracts between layers. Names are proposals; field names follow the API contracts in Document 2.

```ts
// src/shared/types/money.ts  — a "branded" number so cents cannot be mixed up with rupees
export type Cents = number & { readonly __brand: 'LKR_CENTS' };

// src/shared/api/errors.ts
export interface ApiError {
  status: number;              // HTTP status, 0 for network error
  code: string;                // e.g. "COURIER_CAPACITY_EXCEEDED"
  message: string;             // safe text from backend
  correlationId?: string;
  fieldErrors: Record<string, string>; // from error.details[]
}

// src/shared/api/client.ts
export interface RequestOptions {
  idempotencyKey?: string;     // required by convention for POST that changes data
  signal?: AbortSignal;
}
export interface ApiClient {
  get<T>(path: string, query?: Record<string, string | number>, opts?: RequestOptions): Promise<T>;
  post<T>(path: string, body: unknown, opts?: RequestOptions): Promise<T>;
  patch<T>(path: string, body: unknown, opts?: RequestOptions): Promise<T>;
}

// features/dispatcher-admin/dispatch-board/types.ts  (view models)
export interface BoardParcel { id: string; trackingNumber: string; weightKg: number; city: string; createdAt: Date; }
export interface BoardCourier { id: string; name: string; status: string; active: number; max: number; isFull: boolean; }
export interface DispatchBoard { zoneCode: string; parcels: BoardParcel[]; couriers: BoardCourier[]; }

// features/dispatcher-admin/dispatch-board/api/dispatchApi.ts
export interface DispatchApi {
  getBoard(zoneCode: string): Promise<DispatchBoard>;
  assign(input: { parcelId: string; courierId: string }, idempotencyKey: string): Promise<{ assignmentId: string }>;
}

// features/dispatcher-admin/dispatch-board/hooks/useAssignParcel.ts
export function useAssignParcel(): {
  assign(parcelId: string, courierId: string): void; // makes or reuses the key (Doc 2 3.3)
  retry(): void;                                     // same input, same key
  isPending: boolean;
  error: ApiError | null;
};
```

`isFull` is display only (`active >= max` from API numbers). It greys out a courier but the backend still decides (INV-01).

![Figure: frontend classes](doc3-images/d04-frontend-classes.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
classDiagram
    class ApiClient {
        +get(path, query, opts)
        +post(path, body, opts)
        +patch(path, body, opts)
    }
    class RefreshLock {
        -pending: Promise
        +runOnce() Promise
    }
    class AuthContext {
        -accessToken: string
        +role: Role
        +setToken(token)
        +clear()
    }
    class ApiError {
        +status: number
        +code: string
        +message: string
        +correlationId: string
        +fieldErrors: map
    }
    class DispatchApi {
        +getBoard(zoneCode) DispatchBoard
        +assign(input, key) Assignment
    }
    class useAssignParcel {
        +assign(parcelId, courierId)
        +retry()
        +isPending: boolean
    }
    ApiClient --> RefreshLock : on 401
    ApiClient --> AuthContext : reads token
    ApiClient ..> ApiError : throws
    DispatchApi --> ApiClient : uses
    useAssignParcel --> DispatchApi : calls
```

</details>

### 3.5 How one request travels through the frontend

![Figure: frontend request](doc3-images/d05-frontend-request.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
sequenceDiagram
    autonumber
    participant Page as DispatchBoardPage
    participant Hook as useAssignParcel
    participant Api as dispatchApi
    participant Client as Shared API client
    participant GW as Gateway
    Page->>Hook: assign(parcelId, courierId)
    Hook->>Hook: Make or reuse Idempotency-Key
    Hook->>Api: assign(input, key)
    Api->>Api: Map camelCase to snake_case DTO
    Api->>Client: post /dispatch/assignments
    Client->>Client: Add Authorization from memory
    Client->>GW: HTTP request
    GW-->>Client: Response
    alt Error envelope
        Client-->>Hook: throw ApiError
        Hook-->>Page: error state
    else Success
        Client-->>Api: data
        Api-->>Hook: view model
        Hook->>Hook: Invalidate board query (refetch)
        Hook-->>Page: success state
    end
```

</details>

### 3.6 State management

| Kind of state | Where it lives | Why |
| --- | --- | --- |
| Data from the server (board, lists, reports) | TanStack Query cache \[Repo\] stack | Handles loading, errors, refetch on focus, timed refresh and cache. Server stays the source of truth (Doc 1 section 5) |
| Access token, role | `AuthContext` (memory) \[Repo\] | Required by ADR-003 |
| Form input | `react-hook-form` + zod \[Repo\] frontend.mdc | Quick hints and field errors |
| Screen choices (selected zone, open dialog, filters) | Local component state or URL query string | Filters in the URL can be shared and survive reload; nothing sensitive goes in the URL |
| Idempotency keys | Inside the mutation hook (memory) | Doc 2 3.3 |

There is **no global store** (no Redux, no Zustand). Reason in decision DD-02.

### 3.7 Design patterns used in the frontend

| Pattern | Where | Why |
| --- | --- | --- |
| Feature-sliced folders | `features/dispatcher-admin/<feature>` | Each screen area is self-contained; Member 6 and I do not edit the same files \[Repo\] web/README 1 |
| Container and presentational split | `pages` vs `components` | Components can be tested with plain props, without a server |
| Adapter (DTO mapper) | `api/*Api.ts` | If the backend renames a field (many endpoints are still \[TBD\]), only one file changes |
| Facade | Shared API client | One place for token, refresh, keys and errors; features stay simple |
| Single-flight (mutex) | `refreshLock.ts` | Many 401s cause only one refresh, so rotation does not log the user out (Doc 2 F-02) \[Repo\] web/CLAUDE.md "mutex-locked token refresh" |
| Query key factory | `api/queryKeys.ts` | Correct list refresh after each action; no stale data |
| Error mapping table | `shared/api/errors.ts` + one `ErrorBanner` | Same behaviour for every status code (Doc 2 3.2) |
| Lazy routes (code splitting) | `opsRoutes.tsx` with `React.lazy` | Customers never download ops code; couriers download only courier code \[Repo\] web/README 1 |
| Page Object Model | `web/tests/e2e/pages/DispatchBoardPage.ts`, `CourierTasksPage.ts` | Required by web/CLAUDE.md 5 \[Repo\] |

### 3.8 Courier screens: special design points

- **Separate code chunk.** `/courier/*` is its own lazy chunk with no charts or tables library, so the first load on a slow phone stays small. Target under 200 KB gzipped JavaScript for the courier chunk plus shell **\[Proposed budget\]**.
- **Photo resize before upload.** `resizeImage.ts` uses the browser canvas to shrink a photo to about 1600 px wide JPEG. A 6 MB phone photo becomes about 0.5 to 1 MB. This keeps uploads under the 10 MB gateway limit and makes them faster on 3G.
- **No offline queue.** Retries are manual with the same Idempotency-Key (DD-15).
- **Token storage.** Same in-memory rule as staff until U-01 is decided.

## 4. Admin backend module design (`core-api/internal/admin/`)

Who writes which part is still open (U-10). This section describes the target design so whoever writes it follows one plan.

### 4.1 Package structure

The four layer folders are required by AGENTS.md 4.2 \[Repo\]. File names are \[Proposed\].

```
core-api/internal/admin/
├── README.md, API.md                         # living docs (AGENTS.md 6)
├── domain/                                   # pure Go, no imports of net/http, database/sql, amqp
│   ├── zone.go                               # Zone aggregate root
│   ├── zone_code.go                          # ZoneCode value object
│   ├── postal_prefix.go                      # PostalPrefix value object
│   ├── money.go                              # Money value object (LKR cents) [or shared, U-29]
│   ├── prefix_policy.go                      # PrefixOverlapPolicy domain service
│   ├── setting.go                            # SystemSetting entity, SettingKey value object
│   ├── report_period.go                      # ReportPeriod value object (from/to, Sri Lanka days)
│   ├── audit_filter.go                       # AuditQueryFilter value object
│   ├── events.go                             # ZoneCreated, ZoneChanged, ZoneStatusChanged, SettingChanged
│   └── errors.go                             # ErrZoneCodeTaken, ErrPrefixConflict, ErrInvalidPrefix, ...
├── application/                              # use cases + interfaces they need ("ports")
│   ├── ports.go                              # ZoneRepository, SettingRepository, ReportReader, AuditReader,
│   │                                         # AuditWriter, MerchantStatusChanger, TxManager, Clock
│   ├── create_zone.go                        # CreateZone use case
│   ├── update_zone.go                        # UpdateZone use case
│   ├── set_zone_status.go                    # EnableZone / DisableZone
│   ├── update_setting.go                     # UpdateSetting
│   ├── generate_report.go                    # GenerateOperationalReport (query)
│   ├── query_audit_log.go                    # QueryAuditLog (query)
│   └── approve_merchant.go                   # ApproveMerchant: calls M4 through MerchantStatusChanger
├── infrastructure/                           # adapters that implement the ports
│   ├── postgres_zone_repository.go
│   ├── postgres_setting_repository.go
│   ├── postgres_report_reader.go             # read-only SQL or calls into other modules' read interfaces
│   ├── postgres_audit_reader.go
│   ├── postgres_audit_writer.go              # [TBD U-30: may move to platform as a shared audit writer]
│   └── merchant_adapter.go                   # wraps the merchants module's exported interface
└── transport/
    ├── routes.go                             # mounts /api/v1/admin/* with ADMIN role middleware
    ├── zone_handler.go, report_handler.go, audit_handler.go, setting_handler.go, merchant_handler.go
    ├── dto.go                                # request/response JSON structs (separate from domain types)
    └── errors.go                             # domain error -> HTTP status and error code table
```

Wiring is done in `core-api/internal/app/wiring.go` \[Repo\] AGENTS.md 4.1: it creates the Postgres adapters, passes them into the use cases, and passes the use cases into the handlers.

### 4.2 Responsibilities of each layer

| Layer | Responsibilities | Example in admin |
| --- | --- | --- |
| **Transport (handlers)** | Parse JSON and query strings, check syntax (types, required), read user id and role from the auth middleware context, call one use case, map result or error to HTTP | `POST /admin/zones` handler turns `ZoneCreateRequest` DTO into `CreateZoneCommand` |
| **Application (use cases)** | Own the transaction, load and save aggregates through ports, call domain methods, write audit rows, return results. One file per use case | `CreateZone.Execute(ctx, cmd)` opens a tx, checks prefixes, saves, writes audit |
| **Domain (objects)** | Hold the rules that are true no matter how data is stored: valid code format, prefix format, no duplicate prefix inside one zone, enable and disable rules, money never negative | `domain.NewZone(code, name, surcharge, prefixes)` returns error if a rule fails |
| **Repositories (ports)** | Interfaces declared in `application/ports.go`, written from the use case's point of view | `ZoneRepository.Save(ctx, *Zone)` |
| **Infrastructure (adapters)** | SQL, row mapping, unique-constraint error to domain error, calls into other modules | `postgresZoneRepository` maps `23505 unique_violation` on `zone_postal_codes` to `ErrPrefixConflict` |
| **Platform (shared, not mine)** | DB pool, auth middleware, idempotency middleware, logging, request id | Used by `routes.go` and `wiring.go` |

![Figure: admin layers](doc3-images/d06-admin-layers.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart TD
    T["transport: handlers, DTOs, error mapping"] --> A["application: use cases, ports (interfaces)"]
    A --> D["domain: Zone, value objects, rules, events, errors"]
    I["infrastructure: Postgres adapters, merchant adapter"] --> A
    I --> D
    W["app/wiring.go"] --> T
    W --> I
    W --> A
    T --> PL["platform: auth, idempotency middleware"]
    I --> PL2["platform: DB pool, tx"]
    classDef pure fill:#fff3cd,stroke:#b58900,color:#111
    class D pure
```

</details>

Arrows show "imports". The domain (yellow) imports nothing from the project. Infrastructure points **up** to application because it **implements** the interfaces the application declares. This is the "dependency inversion" rule of Clean/Hexagonal Architecture \[Repo\] Engineering Standards 2.5.

### 4.3 Allowed imports in Go

| Package | May import | Must NOT import | Checked by |
| --- | --- | --- | --- |
| `admin/domain` | Go standard library (`errors`, `fmt`, `strings`, `time`, `regexp`) | `net/http`, `database/sql`, amqp, any `infrastructure` or `transport` | `depguard` rule `domain_purity` \[Repo\] |
| `admin/application` | `admin/domain`, `context`, other modules' **exported interfaces or types only if needed for a port** | `net/http`, any `transport` | `depguard` rule `application_purity` \[Repo\] |
| `admin/infrastructure` | `admin/application`, `admin/domain`, `platform/db`, `database/sql` or pgx, the merchants module's exported service interface | `admin/transport`, other modules' `infrastructure` or tables | review + proposed depguard rule \[U-31\] |
| `admin/transport` | `admin/application`, `admin/domain` (errors only), `platform/http`, `platform/auth`, `net/http` | `admin/infrastructure`, `database/sql` | proposed depguard rule \[U-31\] |
| Other modules | `admin` exported interfaces only, if ever needed | `admin/infrastructure`, admin tables | review |

**Talking to other modules.** Go idiom: "the consumer declares the interface". The admin `application/ports.go` declares a tiny interface such as `MerchantStatusChanger` with only the one method admin needs. The adapter in `infrastructure/merchant_adapter.go` wraps the merchants module's public service. Admin never imports the merchants module's `domain` or `infrastructure` and never touches `merchants` tables.

### 4.4 Key Go interfaces

```go
// application/ports.go
package application

type TxManager interface {
    // WithinTx runs fn inside one PostgreSQL transaction. The tx travels in ctx,
    // so every repository call made with that ctx uses the same tx.
    WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type ZoneRepository interface {
    NextID() string
    FindByID(ctx context.Context, id string) (*domain.Zone, error)          // ErrZoneNotFound
    ExistingPrefixes(ctx context.Context, near []domain.PostalPrefix) ([]domain.PrefixOwner, error)
    LockPrefixSpace(ctx context.Context) error                               // see 4.7
    Insert(ctx context.Context, z *domain.Zone) error                         // ErrZoneCodeTaken, ErrPrefixConflict
    Update(ctx context.Context, z *domain.Zone) error                         // ErrConcurrentConflict when version differs
    List(ctx context.Context, f ZoneListFilter, page Page) ([]*domain.Zone, int, error)
}

type SettingRepository interface {
    Get(ctx context.Context, key domain.SettingKey) (*domain.SystemSetting, error)
    Save(ctx context.Context, s *domain.SystemSetting) error
}

type AuditWriter interface {
    Write(ctx context.Context, e AuditEntry) error // INSERT only, same tx as the change
}

type AuditReader interface {
    Query(ctx context.Context, f domain.AuditQueryFilter, page Page) ([]AuditEntry, int, error)
}

type ReportReader interface {
    Operational(ctx context.Context, p domain.ReportPeriod, zone *domain.ZoneCode) (OperationalReport, error)
}

type MerchantStatusChanger interface { // implemented by an adapter over Member 4's module
    Approve(ctx context.Context, merchantID string, actor Actor, reason string) (MerchantStatus, error)
    Suspend(ctx context.Context, merchantID string, actor Actor, reason string) (MerchantStatus, error)
}

type Clock interface{ Now() time.Time } // makes tests repeatable

type Actor struct { UserID string; Role string; IP string; RequestID string } // from auth context, never from the body
```

```go
// domain/zone.go  (shape only)
package domain

type Zone struct {
    id        string
    code      ZoneCode
    name      string
    surcharge Money
    prefixes  []PostalPrefix
    enabled   bool
    version   int
    events    []Event // recorded, read by the use case for audit
}

func NewZone(id string, code ZoneCode, name string, surcharge Money, prefixes []PostalPrefix) (*Zone, error)
func (z *Zone) Rename(name string) error
func (z *Zone) ChangeSurcharge(m Money) error
func (z *Zone) ReplacePrefixes(p []PostalPrefix) error // no duplicates, at least one
func (z *Zone) Disable() error                          // ErrAlreadyDisabled
func (z *Zone) Enable() error
func (z *Zone) PullEvents() []Event
```

```go
// domain/postal_prefix.go — value object, checks itself on creation
type PostalPrefix struct{ v string }
func NewPostalPrefix(s string) (PostalPrefix, error) // digits only, 3 to 5 chars [Proposed, Sri Lanka codes are 5 digits]
func (p PostalPrefix) Overlaps(o PostalPrefix) bool  // "627" overlaps "62701"
```

### 4.5 DDD concepts used, and why

| Concept | What I use | Why (and why not more) |
| --- | --- | --- |
| **Bounded context** | Admin & Operations (shared with M1, M4) \[Repo\] domain-boundaries 2 | Zones, settings, reports and audit queries are back-office concerns that do not belong to parcels or dispatch |
| **Aggregate root** | `Zone` (owns its list of `PostalPrefix`) | A zone and its prefixes must change together and stay valid together (at least one prefix, no duplicates inside). Kept small, as Engineering Standards 2.3 asks |
| **Entity** | `Zone`, `SystemSetting` | They have an identity and change over time |
| **Value objects** | `ZoneCode`, `PostalPrefix`, `Money`, `ReportPeriod`, `AuditQueryFilter`, `SettingKey` | Checked once when made, cannot be changed afterwards, so bad values cannot travel through the code. `Money` stops float mistakes (LKR cents, INV-13) |
| **Domain service** | `PrefixOverlapPolicy` | "Does this new prefix clash with prefixes of **other** zones?" involves more than one aggregate, so it does not fit inside one `Zone`. It is a pure function: given the new prefixes and the existing owners, return the clashes |
| **Domain events** | `ZoneCreated`, `ZoneChanged`, `ZoneStatusChanged`, `SettingChanged` | Recorded by the aggregate and turned into `audit_logs` rows by the use case in the same transaction. **Not** published to RabbitMQ (I publish nothing, Doc 1 X-08). If Member 2's pricing later needs to react, they can be written to the outbox with Member 8's writer **\[TBD U-32\]** |
| **Repositories** | `ZoneRepository`, `SettingRepository` | Hide SQL from use cases; make unit tests possible with in-memory fakes |
| **Read models (no aggregates)** | `ReportReader`, `AuditReader` | Reports and audit search only read. Wrapping them in aggregates adds code with no rule to protect (Engineering Standards 2.8: "do not model every table as an aggregate") |
| **Not used** | Event sourcing, CQRS framework, sagas | Forbidden without approval (AGENTS.md 10) and not needed here |

![Figure: admin domain](doc3-images/d07-admin-domain.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
classDiagram
    class Zone {
        <<aggregate root>>
        -id: string
        -code: ZoneCode
        -name: string
        -surcharge: Money
        -prefixes: PostalPrefix[]
        -enabled: bool
        -version: int
        +Rename(name)
        +ChangeSurcharge(m)
        +ReplacePrefixes(p)
        +Enable()
        +Disable()
        +PullEvents() Event[]
    }
    class ZoneCode {
        <<value object>>
        +String()
    }
    class PostalPrefix {
        <<value object>>
        +Overlaps(other) bool
    }
    class Money {
        <<value object>>
        +Cents int64
        +Currency LKR
    }
    class PrefixOverlapPolicy {
        <<domain service>>
        +Clashes(newPrefixes, existingOwners) Clash[]
    }
    class SystemSetting {
        <<entity>>
        -key: SettingKey
        -value: string
        +Change(value)
    }
    class ReportPeriod {
        <<value object>>
        +FromUTC()
        +ToUTC()
    }
    class AuditQueryFilter {
        <<value object>>
    }
    class ZoneEvent {
        <<domain event>>
        ZoneCreated
        ZoneChanged
        ZoneStatusChanged
    }
    Zone *-- ZoneCode
    Zone *-- "1..*" PostalPrefix
    Zone *-- Money
    Zone ..> ZoneEvent : records
    PrefixOverlapPolicy ..> PostalPrefix : compares
```

</details>

### 4.6 Ports and adapters view

![Figure: admin hexagon](doc3-images/d08-admin-hexagon.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart LR
    subgraph In["Driving side (who calls us)"]
        H1["Zone handler"]
        H2["Report handler"]
        H3["Audit handler"]
        H4["Merchant approval handler"]
    end
    subgraph Core["admin application + domain"]
        UC1["CreateZone, UpdateZone, SetZoneStatus"]
        UC2["GenerateOperationalReport"]
        UC3["QueryAuditLog"]
        UC4["ApproveMerchant"]
        DM["Domain: Zone, value objects, PrefixOverlapPolicy"]
        P1(["ZoneRepository"])
        P2(["ReportReader"])
        P3(["AuditReader / AuditWriter"])
        P4(["MerchantStatusChanger"])
        P5(["TxManager"])
    end
    subgraph Out["Driven side (what we call)"]
        A1["postgresZoneRepository"]
        A2["postgresReportReader"]
        A3["postgresAudit adapters"]
        A4["merchantAdapter -> merchants module (M4)"]
        A5["platform tx manager"]
        DB[("PostgreSQL")]
    end
    H1 --> UC1
    H2 --> UC2
    H3 --> UC3
    H4 --> UC4
    UC1 --> DM
    UC1 --> P1
    UC1 --> P3
    UC1 --> P5
    UC2 --> P2
    UC3 --> P3
    UC4 --> P4
    P1 -.-> A1
    P2 -.-> A2
    P3 -.-> A3
    P4 -.-> A4
    P5 -.-> A5
    A1 --> DB
    A2 --> DB
    A3 --> DB
    A5 --> DB
```

</details>

### 4.7 Transaction boundaries and consistency

**Rule:** one use case = one transaction, opened by the use case with `TxManager.WithinTx` \[Repo\] AGENTS.md 4.3. Handlers never open transactions. Repositories never commit.

| Use case | Inside one transaction | Isolation and locking | Consistency |
| --- | --- | --- | --- |
| CreateZone | idempotency record, prefix-space lock, read clashing prefixes, insert zone, insert prefixes, insert audit row | READ COMMITTED + `pg_advisory_xact_lock(<admin-prefix-key>)` + unique index on `zone_postal_codes.prefix` | Strong: all or nothing |
| UpdateZone (incl. prefixes) | same as above + `UPDATE ... WHERE id = $1 AND version = $2` | as above + optimistic version check | Strong; stale edit gets 409 |
| Enable/DisableZone | update status with version check, audit row | optimistic version check | Strong |
| UpdateSetting | read old value, update, audit row | `SELECT ... FOR UPDATE` on the setting row | Strong |
| ApproveMerchant | M4's status change + audit row, in **the same** tx passed through ctx | M4's merchant `version` check | Strong across both modules, because they share one database and one tx (modular monolith) |
| GenerateOperationalReport | read-only tx (or none) | READ COMMITTED, `statement_timeout` (proposed 5 s) | Read can be a few seconds behind writes in progress; acceptable for reports |
| QueryAuditLog | read-only | READ COMMITTED | Same |

**Why the advisory lock?** The unique index stops two zones getting the **exact same** prefix. It cannot stop "627" in one zone and "62701" in another (overlap). Two admins could each check, find no clash, and both commit. A short transaction-level advisory lock makes prefix changes run one after another. Zone edits are rare (a few per week), so waiting is not a problem. Alternative: SERIALIZABLE isolation with retry (DD-07).

**Why audit in the same transaction?** If the change commits, its audit row must exist, and if the change fails, there must be no audit row (SEC-009, INV-12). An async audit through the outbox could lose or delay the record (DD-10).

**Idempotency** is handled by the shared platform middleware using the `idempotency_keys` table (scope `USER:<id>`, operation, request hash, saved response, 24 h) \[Repo\] api-conventions 4, database-schema. The key row is written in the same transaction as the change, so a crash cannot leave "key saved, change lost". Owner of the middleware **\[TBD U-24\]**.

![Figure: admin tx](doc3-images/d09-admin-tx.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
sequenceDiagram
    autonumber
    participant H as Zone handler
    participant UC as CreateZone use case
    participant TX as TxManager
    participant R as ZoneRepository (Postgres)
    participant POL as PrefixOverlapPolicy
    participant AW as AuditWriter
    participant DB as PostgreSQL
    H->>UC: Execute(cmd with Actor from token)
    UC->>UC: Build value objects (ZoneCode, PostalPrefix, Money)
    UC->>TX: WithinTx
    TX->>DB: BEGIN
    UC->>R: LockPrefixSpace
    R->>DB: pg_advisory_xact_lock
    UC->>R: ExistingPrefixes(near new ones)
    R->>DB: SELECT
    UC->>POL: Clashes(new, existing)
    alt Clash found
        UC-->>TX: ErrPrefixConflict
        TX->>DB: ROLLBACK
    else No clash
        UC->>R: Insert(zone)
        R->>DB: INSERT delivery_zones, zone_postal_codes
        UC->>AW: Write(ZoneCreated audit entry)
        AW->>DB: INSERT audit_logs
        TX->>DB: COMMIT (idempotency row also saved here)
    end
    UC-->>H: Zone or error
    H->>H: Map to 201 or 409
```

</details>

### 4.8 Error handling design

Domain and application return typed errors. Only `transport/errors.go` knows HTTP.

| Go error | HTTP | Error code (proposed) |
| --- | --- | --- |
| `ErrInvalidZoneCode`, `ErrInvalidPrefix`, `ErrNegativeMoney`, `ErrNoPrefixes`, `ErrDuplicatePrefixInZone` | 422 | `VALIDATION_FAILED` with `details[]` |
| `ErrZoneCodeTaken` | 409 | `ZONE_CODE_ALREADY_EXISTS` |
| `ErrPrefixConflict` | 409 | `ZONE_POSTAL_CODE_CONFLICT` |
| `ErrConcurrentConflict` | 409 | `CONCURRENT_MODIFICATION` **\[TBD: shared name with M2\]** |
| `ErrZoneNotFound` | 404 | `ZONE_NOT_FOUND` |
| `ErrForbidden` (from auth) | 403 | `ACCESS_DENIED` |
| anything else | 500 | `INTERNAL_ERROR` + correlation id, details only in logs |

Errors are wrapped with context (`fmt.Errorf("create zone: %w", err)`) and checked with `errors.Is` \[Repo\] AGENTS.md 9. No `panic`.

### 4.9 Cross-module call: merchant approval

![Figure: admin merchant](doc3-images/d10-admin-merchant.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart LR
    H["admin transport: merchant handler"] --> UC["admin application: ApproveMerchant"]
    UC --> P(["port: MerchantStatusChanger"])
    P -.-> AD["admin infrastructure: merchantAdapter"]
    AD --> MS["merchants module public service (M4)"]
    MS --> MD["merchants domain: Merchant aggregate (M4)"]
    MS --> MDB[("merchants table (M4 owns)")]
    UC --> AW["AuditWriter"]
    AW --> ADB[("audit_logs")]
    TX["One transaction passed in ctx"] -.- UC
    TX -.- MS
```

</details>

Member 4 keeps all merchant rules (which status changes are allowed). Admin only adds the ADMIN check, the reason, and the audit row **\[TBD U-10, U-26\]**.

## 5. Gateway design (`deploy/gateway/traefik/`)

### 5.1 File structure

```
deploy/gateway/
├── README.md                         # living doc (route map lives here or in docs/, U-33)
├── traefik/
│   ├── traefik.yml                   # [Repo] static config: entrypoints, trusted IPs, logs, dashboard
│   ├── dynamic-routes.yml            # [Repo] Docker/file provider: routers, services, middlewares
│   ├── ingressroutes.yaml            # [Proposed, M7] Kubernetes IngressRoute CRDs, same routes
│   └── middlewares.yaml              # [Proposed, M7] Kubernetes Middleware CRDs, same limits
└── tests/
    ├── smoke.sh                      # [Proposed] curl checks: route per service, 429, 413, 401/403 unchanged, internal paths blocked
    └── route-parity.sh               # [Proposed] checks Docker and Kubernetes files list the same routes and limits
```

Member 9 owns `values.yaml` (Helm install) and the Docker Compose file (Doc 1 D-11).

### 5.2 Components inside Traefik

Traefik is built from four kinds of parts. Each request goes left to right.

![Figure: gateway components](doc3-images/d11-gateway-components.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart LR
    subgraph EP["Entrypoints (traefik.yml)"]
        E1["web :80"]
        E2["websecure :443, origin TLS secret"]
    end
    subgraph RT["Routers (match path)"]
        R1["tracking: /api/v1/tracking"]
        R2["ai: /api/v1/ai"]
        R3["core: /api/v1"]
        R4["api-404: /api (proposed)"]
        R5["web: /"]
    end
    subgraph MW["Middleware chains"]
        M1["cors, tracking-ratelimit, tracking-inflight (proposed)"]
        M2["cors, ai-ratelimit (proposed), small body limit (proposed)"]
        M3["cors, login-ratelimit on /auth/login (proposed), upload-limit 10 MB"]
        M4["none"]
    end
    subgraph SV["Services"]
        S1["tracking-api :8081"]
        S2["ai-assistant :8000"]
        S3["core-api :8080"]
        S4["404 JSON (proposed)"]
        S5["web :3000"]
    end
    E1 --> RT
    E2 --> RT
    R1 --> M1 --> S1
    R2 --> M2 --> S2
    R3 --> M3 --> S3
    R4 --> S4
    R5 --> M4 --> S5
```

</details>

### 5.3 Router and middleware design

| Router | Rule | Priority | Middlewares (in order) | Service | Status |
| --- | --- | --- | --- | --- | --- |
| `login-router` | `Path(/api/v1/auth/login)` | high (explicit) | cors, login-ratelimit (5/min, burst 3, per real IP) | core-api | \[Proposed, U-06\] |
| `tracking-router` | `PathPrefix(/api/v1/tracking)` | auto (by rule length) | cors, tracking-ratelimit, tracking-inflight (5 per IP) | tracking-api | ratelimit \[Repo\], inflight \[Proposed\] |
| `ai-router` | `PathPrefix(/api/v1/ai)` | auto | cors, ai-ratelimit (per IP), ai-body-limit (1 MB) | ai-assistant | \[Proposed, U-06\] |
| `core-api-router` | `PathPrefix(/api/v1)` | auto | cors, upload-limit (10 MB) | core-api | \[Repo\] |
| `api-404-router` | `PathPrefix(/api)` | auto (shorter than /api/v1) | none | a small 404 JSON responder **\[Proposed, Doc 2 G-01; method TBD with M9\]** | \[Proposed\] |
| `frontend-router` | `PathPrefix(/)` | lowest | none (later: security headers) | web | \[Repo\] |

**Design rules for the files**

- Every rate limit middleware sets `sourceCriterion` so the **real visitor IP** is counted (for example `requestHeaderName: CF-Connecting-IP` in production behind Cloudflare, `ipStrategy` in local). Without it, all Cloudflare traffic shares one bucket (Doc 2 G-02).
- No `retry` middleware on any router that accepts writes (Doc 2 R-GW-03).
- No middleware that reads, checks or changes `Authorization` or `X-API-Key` (INV-04).
- The tracking router never gets the buffering middleware (INV-07).
- Names are the same in Docker and Kubernetes files so smoke tests and logs match.

Example of the proposed Kubernetes middleware (placeholders only, no secrets):

```yaml
# middlewares.yaml (proposed)
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata: { name: tracking-ratelimit, namespace: swiftdrop }
spec:
  rateLimit:
    average: 30
    period: 1m
    burst: 50            # U-15: 50 or 10
    sourceCriterion:
      requestHeaderName: CF-Connecting-IP
---
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata: { name: tracking-inflight, namespace: swiftdrop }
spec:
  inFlightReq:
    amount: 5            # Member 5: 5 open streams per IP (proposed)
    sourceCriterion:
      requestHeaderName: CF-Connecting-IP
```

### 5.4 Patterns used in the gateway

| Pattern | Where | Why |
| --- | --- | --- |
| API gateway / edge router | Traefik routers | One front door for all clients (ADR-001) |
| Chain of responsibility | Middleware chains | Each middleware does one job and can stop the request (429, 413) |
| Configuration as code | All files in git | Rebuildable from a clean clone (INV-09) |
| Smoke test as contract | `tests/smoke.sh` | Proves routes and limits after every change (Doc 2 G-07) |
| Parity check | `tests/route-parity.sh` | Docker and Kubernetes files cannot drift apart (DD-12) |

## 6. Design decisions, alternatives and trade-offs

| ID | Decision | Options considered | Chosen and why | Trade-off accepted |
| --- | --- | --- | --- | --- |
| DD-01 | Where ops and courier screens live | (a) own separate app, (b) one shared SPA with feature folders | (b) \[Repo\] web/README 1: one login, one design system, one deploy | Must agree shared shell changes with Member 6 |
| DD-02 | How screens hold server data | (a) Redux/Zustand global store, (b) TanStack Query cache + local state | (b): built-in loading, errors, refetch and cache; server stays the truth; less code | Team must learn query keys and invalidation |
| DD-03 | How the dispatch board stays fresh | (a) SSE live stream, (b) WebSocket, (c) polling every 30 s + refetch after actions | (c) \[Proposed\]: no staff stream exists; SSE is only for public tracking (M5); polling is simple and cheap at our size (7.1) | Up to 30 s delay; backend conflict checks cover the gap |
| DD-04 | Token refresh | (a) each call refreshes itself, (b) single shared refresh | (b): refresh tokens are single use; (a) would trigger "token reuse" logout | Small extra code in the shared client |
| DD-05 | Where Idempotency-Keys are made | (a) in the shared client for every POST, (b) in each mutation hook per user action | (b): the hook knows when input changed and when a press is a retry | Each mutation hook must follow the rule (tested) |
| DD-06 | Zone model | (a) Zone and PostalPrefix as two aggregates, (b) Zone aggregate owns its prefixes | (b): they change together and the "at least one prefix" rule needs both | Cross-zone rule still needs a domain service + DB (DD-07) |
| DD-07 | Stopping prefix overlaps between zones | (a) unique index only, (b) SERIALIZABLE isolation with retry, (c) unique index + advisory lock + overlap check | (c): exact duplicates and overlaps both blocked; simple; edits are rare | Prefix edits run one at a time |
| DD-08 | Reports | (a) aggregates, (b) separate reporting service, (c) materialized views, (d) read-only queries behind `ReportReader` | (d): simplest; meets NFR at our data size; can switch to (c) later behind the same port | Heavy ranges may be slow; capped range + timeout (7.2) |
| DD-09 | How admin reaches other modules | (a) read their tables with SQL, (b) HTTP calls, (c) RabbitMQ, (d) in-process Go interfaces | (d) \[Repo\] domain-boundaries 4: no shared-table coupling, no network hop, can share one tx | Other owners must export a small interface |
| DD-10 | Writing audit rows | (a) async through outbox and worker, (b) same transaction as the change | (b): an audit row exists if and only if the change happened | Slightly longer transaction |
| DD-11 | Concurrent edits of zones and rules | (a) last write wins, (b) optimistic `version` column, (c) row locks held while the user edits | (b), same as parcels \[Repo\] database-schema 3.3; needs a migration adding `version` to `delivery_zones` | User must reload on 409 |
| DD-12 | Gateway files for Docker and Kubernetes | (a) plain Kubernetes Ingress + annotations, (b) generate both from one template, (c) two hand-written sets + parity test | (c): Traefik CRDs support all needed middlewares; files stay readable for a student team; parity test stops drift | Two places to edit per change |
| DD-13 | Where rate limits live | (a) only gateway, (b) only backend, (c) gateway per IP + backend per user or key | (c): gateway cannot see users (ADR-001); backend per-user limits are Members 1, 4, 10's choice (U-06) | Per pod, so limits are approximate (7.3) |
| DD-14 | Photo size | (a) send original, (b) resize on the phone | (b): faster on slow networks, stays under 10 MB | Small quality loss; some old phones resize slowly |
| DD-15 | Courier offline mode | (a) offline queue with service worker, (b) manual retry with same key | (b): offline queue is complex (ordering, conflicts, stale tasks) and not required by the SRS | Courier must press "Try again" when back online |
| DD-16 | Admin module layering | (a) one flat package for simple CRUD, (b) four layers | (b) for zones (real rules) and settings; reports and audit stay thin (no domain objects) as the anti-boilerplate rule allows \[Repo\] AGENTS.md 4.2 | Some folders have few files |

## 7. Scaling and performance

### 7.1 Frontend

| Topic | Expected load | Design response |
| --- | --- | --- |
| Dispatch board polling | About 10 dispatchers × 1 call per 30 s = 0.3 calls per second | Negligible. Polling pauses when the browser tab is hidden (TanStack Query default) |
| Board size | Up to a few hundred unassigned parcels per zone **\[TBD with M3: does the board paginate?\]** | Zone filter is required; list uses virtual scrolling above 200 rows **\[Proposed\]** |
| Bundle size | Ops chunk includes charts; courier chunk must be small | Lazy routes per feature; charts library only in `reports` chunk; budget checked by `npm run build` output |
| Couriers on slow networks | 3G, high delay | Small courier chunk, resized photos, no polling on courier screens (refresh on open and pull-to-refresh) |
| Stale data | Two people editing | Server is truth; 409 then refetch |

### 7.2 Admin backend module

| Topic | Design response |
| --- | --- |
| Stateless pods | Core API runs 2 to 4 replicas with HPA at 70 % CPU \[Repo\] Engineering Standards 10. Admin keeps nothing in memory between requests |
| Database connections | `replicas_max × MaxOpenConns ≤ PostgreSQL max_connections` \[Repo\]. Example: 4 × 20 = 80 ≤ 100. Admin uses the shared pool, no own pool |
| Speed target | p95 ≤ 500 ms at 50 concurrent users (NFR-PERF-001). Zone writes are small. Reports are the risk |
| Report queries | Range capped (proposed 92 days), `statement_timeout` 5 s, indexes on `parcels(created_at)`, `parcel_status_history(to_status, occurred_at)`, `delivery_attempts(attempted_at)` **\[TBD with M2, M3: they own those tables\]**. If still slow: materialized view refreshed by a CronJob (DD-08) |
| Audit search | Paginated (max 100 per page, NFR-PERF-004); index on `audit_logs(created_at DESC)`, `(actor_id, created_at)`, `(target_type, target_id)` **\[Proposed; owner U-30\]** |
| Graceful shutdown | Handled by the Core API main (20 s on SIGTERM) \[Repo\]; admin requests are short |
| Metrics | Request count and latency per route; no user or parcel ids as labels \[Repo\] Engineering Standards 11 |

### 7.3 Gateway

| Topic | Design response |
| --- | --- |
| Replicas | Traefik runs as a Deployment that can scale (domain-boundaries 5). Rate limits are **per pod**: with 3 pods a client can get up to 3 × the limit. Set numbers per pod with that in mind (ADR-001 3) |
| Real client IP | `trustedIPs` + `sourceCriterion` so limits and audit logs see the real visitor |
| Memory | About 50 to 80 MB per instance (ADR-001). The 10 MB buffering limit means many big uploads at once use memory; Traefik writes large bodies to disk above its memory threshold **\[to verify in test\]** |
| SSE connections | Long open connections are cheap for Traefik; limited to 5 per IP by `inFlightReq` (proposed). Heartbeat keeps Cloudflare from closing them (U-17) |
| Hot reload | File provider `watch: true` reloads routes without restart (Docker); CRDs reload on apply (Kubernetes) |

### 7.4 Deployment view

![Figure: deployment](doc3-images/d12-deployment.png)

<details>
<summary>Diagram source (Mermaid)</summary>

```mermaid
flowchart TB
    U["Users: dispatchers, admins, couriers"] --> CF["Cloudflare"]
    CF --> T1
    subgraph K3S["K3s cluster on Ubuntu VPS"]
        subgraph TR["Traefik Deployment (can scale)"]
            T1["traefik pod 1"]
            T2["traefik pod 2"]
        end
        subgraph WEB["web Deployment (HPA)"]
            W1["nginx + SPA"]
        end
        subgraph CORE["core-api Deployment (HPA 2 to 4)"]
            C1["core-api pod (includes admin module)"]
            C2["core-api pod"]
        end
        subgraph TRKD["tracking-api (HPA 2 to 4)"]
            K1["tracking pod"]
        end
        PG[("PostgreSQL StatefulSet, 1 primary")]
    end
    T1 --> W1
    T1 --> C1
    T1 --> C2
    T1 --> K1
    T2 --> C1
    C1 --> PG
    C2 --> PG
```

</details>

## 8. How the design supports testing (short)

| Part | Test style | Made possible by |
| --- | --- | --- |
| Frontend components | Vitest + RTL with plain props | Container and presentational split |
| Frontend hooks and API modules | Vitest with a fake `ApiClient` (or MSW) | All calls go through one client interface |
| Shared refresh | Unit test: 5 parallel 401s cause 1 refresh call | `RefreshLock` is its own small module |
| Admin domain | Plain Go unit tests, no database | Pure `domain/` package |
| Admin use cases | Go tests with in-memory fakes of the ports | Interfaces in `application/ports.go` |
| Admin adapters | Integration tests on real PostgreSQL (Testcontainers) for unique index, advisory lock, version check | Infrastructure kept separate |
| Gateway | `smoke.sh` and `route-parity.sh` against Member 9's Docker Compose | Config as code |

Full test plans belong in a later document.

## 9. Open questions

Earlier questions U-01 to U-27 still apply. Those that most affect this design: U-01 (courier tokens), U-06 (rate limits), U-10 (who writes admin Go code), U-13 (zone model), U-24 (idempotency support), U-26 (merchant statuses).

New questions:

| ID | Question | Decide with |
| --- | --- | --- |
| U-28 | Add an ESLint import-boundary rule to the shared lint config so cross-feature imports fail CI? | Member 6 |
| U-29 | One shared `Money` value object (in `platform` or a `shared kernel` package) or one per module? | Members 1, 2 |
| U-30 | Who owns the audit writer and `audit_logs` indexes: admin module or `platform` (database-schema lists "Member 7 / 1, centralized audit middleware")? | Member 1 |
| U-31 | Add depguard rules for `transport` and `infrastructure` packages (today only `domain` and `application` are checked)? | All backend members, Member 9 for CI |
| U-32 | Should zone changes also go to the outbox so pricing or tracking can react? | Members 2, 8 |
| U-33 | Where the route map lives: `deploy/gateway/README.md` or a new file in `docs/` | Member 9 |
| U-34 | Add a `version` column to `delivery_zones` (needs a migration) | Members 1, 4 (admin co-owners) |

## 10. Sign-off

To be reviewed by Members 1, 2, 3, 4, 5, 6, 8, 9 and 10. Members 1 and 4 for section 4 (shared admin module), Member 6 for section 3 (shared app shell and lint), Member 9 for section 5.

| Member | Approved (date) | Notes |
| --- | --- | --- |
| Member 1 |  |  |
| Member 2 |  |  |
| Member 3 |  |  |
| Member 4 |  |  |
| Member 5 |  |  |
| Member 6 |  |  |
| Member 8 |  |  |
| Member 9 |  |  |
| Member 10 |  |  |
