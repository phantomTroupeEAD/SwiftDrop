# Document 1 — Module Scope & Responsibility Specification

> **Module:** Identity, Access Control & Security  
> **Owning Member:** Member 1 — Identity, Access & Security  
> **Status:** Proposed Draft (For Review in Team Meeting 2)  
> **Target Production Location:** `core-api/internal/identity/`  
> **Upstream Dependencies:** Traefik Edge Gateway (Member 7/9)  
> **Downstream Dependencies:** All other modules (`parcels`, `pricing`, `dispatch`, `delivery`, `merchants`, `admin`, `events`, `tracking-api`, `ai-assistant`, `web-frontend`)

---

## 1. Purpose & Business Problem

### 1.1 Purpose
The **Identity, Access Control & Security** module provides the authoritative security and principal management foundation for the entire SwiftDrop ecosystem. It manages user credentials, customer registration, Google OAuth 2.0 / OpenID Connect (OIDC) identity federation, session lifecycle via dual-token authentication (short-lived access tokens and single-use refresh token rotation), role-based access control (RBAC), and tenant/resource ownership verification middleware used across the modular monolith.

### 1.2 Business Problem
Without a centralized, robust identity and security domain:
- Individual modules would implement fragmented authentication or trust unverified client-supplied identifiers (`user_id`, `merchant_id`), leading directly to Insecure Direct Object Reference (IDOR) vulnerabilities (CWE-639).
- Role verification would be scattered or mistakenly relegated to frontend UI visibility checks, allowing malicious users to forge administrative, courier, or dispatcher actions by calling backend endpoints directly.
- Session hijacking, credential stuffing, and replay attacks would threaten sensitive customer parcel history, recipient contact information, and merchant proprietary delivery records.
- Third-party social logins (Google) would risk improper account linking, CSRF, and identity spoofing without strict cryptographic nonce and state validation.

### 1.3 Core Objectives
1. **Authoritative Identity Store:** Centralize human accounts (`CUSTOMER`, `MERCHANT`, `COURIER`, `DISPATCHER`, `ADMIN`) in a dedicated `users` table with strict data integrity and unique constraint protections.
2. **Dual-Token Authentication (ADR-003):** Deliver short-lived (15-minute) JWT access tokens paired with revocable, cryptographically secure refresh tokens stored in `HttpOnly`, `SameSite=Strict`, `Secure` cookies with single-use Refresh Token Rotation (RTR).
3. **OpenID Connect (OIDC) Federation:** Enable seamless, secure customer sign-in via Google with automated state and nonce verification, linking Google subjects (`sub`) to SwiftDrop user accounts.
4. **Defense-in-Depth Middleware:** Publish reusable Go HTTP middleware (`AuthMiddleware`, `RequireRole`, `RequireTenant`, `RequireOwnership`) that extracts, cryptographically verifies, and injects the authenticated `SecurityContext` into request contexts before downstream business handlers execute.
5. **Administrative User Management:** Provide administrative capabilities for user search, account lock/suspension, and role assignment.

---

## 2. Functional Responsibilities & Explicit Exclusions

### 2.1 In-Scope Responsibilities

| ID | Capability | Description | SRS / ADR Trace |
|:---|:---|:---|:---|
| **M1-01** | User Registration | Accept customer registration with email, password, full name, and phone. Validate password complexity and ensure email uniqueness. | FR-IAM-001, AT-001 |
| **M1-02** | Credential Hashing | Enforce memory-hard, salted password hashing using Argon2id (fallback bcrypt) with zero plaintext persistence. | FR-IAM-002, SEC-001 |
| **M1-03** | Local Authentication | Authenticate email/password credentials, handle failed attempt penalties, and issue initial token pairs. | FR-IAM-003, AT-001 |
| **M1-04** | Google OIDC Federation | Orchestrate authorization code flow with Google, validate ID tokens (signature, issuer, audience, nonce), and link identities. | FR-IAM-002, ADR-003, AT-014 |
| **M1-05** | Token Lifecycle & RTR | Issue RS256/HS256 access JWTs and handle refresh token rotation, invalidating entire token families upon detected token reuse. | ADR-003, SEC-004 |
| **M1-06** | Session Invalidation | Implement logout endpoint revoking active refresh tokens and purging session cookies. | FR-IAM-003 |
| **M1-07** | Reusable Middleware | Provide HTTP middleware to authenticate tokens, verify roles, enforce tenant boundaries, and populate Go `context.Context`. | FR-IAM-004, SEC-002 |
| **M1-08** | Account Administration | Provide administrator endpoints to list users, filter by role/status, inspect audit profiles, and promote/demote roles. | FR-IAM-005, FR-ADM-004 |
| **M1-09** | Identity Audit Logging | Log security events (login success/failure, role modification, token revocation) to `audit_logs`. | FR-IAM-009, SEC-009 |

### 2.2 Explicit Exclusions (What This Module Does NOT Own)

| Excluded Capability | True Owner | Architectural Boundary & Hand-Off |
|:---|:---|:---|
| **B2B Merchant API Keys** | Member 4 (`internal/merchants`) | Member 4 owns `merchant_api_credentials`, prefix generation (`sd_live_...`), and key verification for automated machine-to-machine traffic. Member 1 handles human merchant logins. |
| **Merchant Business Profiles** | Member 4 (`internal/merchants`) | Member 1 manages the underlying human `User` account; Member 4 manages business name, approval status, and warehouse addresses. |
| **Courier Fleet Operations** | Member 3 (`internal/dispatch`) | Member 1 stores the user account with role `COURIER`; Member 3 manages vehicle type, shift status, current load, and assignments. |
| **Edge Rate Limiting & TLS** | Member 7 & Member 9 (`deploy/gateway`) | Traefik edge gateway terminates TLS and enforces IP-based token-bucket rate limits on `/api/v1/auth/login`. Member 1 enforces application-tier auth locks. |
| **Frontend Authentication UI** | Member 6 & Member 7 (`web/`) | Frontends render forms and manage in-memory token state; Member 1 owns HTTP API contracts and token issuance. |
| **Tracking Projections** | Member 5 (`services/tracking`) | Tracking API validates Member 1's public JWT to unlock authenticated detailed history; Member 1 does not serve tracking milestones. |

---

## 3. Upstream & Downstream Dependencies

### 3.1 Upstream Dependencies (What Member 1 Consumes)
- **Traefik API Gateway:** Routes external HTTP requests to `/api/v1/auth/*` and `/api/v1/admin/users/*`; passes through real visitor IPs via `X-Forwarded-For` / `CF-Connecting-IP` and request tracing IDs via `X-Request-ID`.
- **Google Identity Platform (OIDC):** External identity provider serving OAuth 2.0 authorization endpoints, code exchange token endpoints, and public JWKS keysets (`https://www.googleapis.com/oauth2/v3/certs`).
- **PostgreSQL 16 Engine:** Backing relational persistence for `users`, `refresh_tokens`, and `oauth_identities`.

### 3.2 Downstream Dependencies (Who Consumes Member 1)
- **Member 2 (Parcels & Pricing):** Uses `AuthMiddleware` and `RequireRole(RoleCustomer, RoleMerchant)` to protect booking and fee snapshotting. Uses `GetUserID(ctx)` to bind `customer_id`.
- **Member 3 (Dispatch & Delivery):** Uses `RequireRole(RoleDispatcher, RoleAdmin)` for assignment endpoints and `RequireRole(RoleCourier)` for proof-of-delivery uploads.
- **Member 4 (Merchants):** Links merchant business accounts to Member 1's `owner_user_id`. Relies on `AuthMiddleware` for merchant admin portal access.
- **Member 5 (Tracking API):** Reuses the shared JWT verification logic (JWKS / public key) to distinguish anonymous public tracking from authenticated tracking requests.
- **Member 6 & 7 (Web Portals):** Call auth endpoints to log users in, manage profile state, and trigger silent token refresh.
- **Member 8 (Events & Messaging):** Reads `actor_id` and `actor_role` from the authenticated context to annotate outbox events.
- **Member 10 (AI Assistant):** Forwards user Bearer JWTs to backend APIs during authorized tool execution.

---

## 4. Bounded Context, Domain Ownership & Authoritative Data

```
┌────────────────────────────────────────────────────────────────────────┐
│                      IDENTITY & ACCESS BOUNDED CONTEXT                 │
│                                                                        │
│   [Aggregate Root: User]                                               │
│   ├── UserID (UUIDv7)                                                  │
│   ├── Email (Value Object: Normalized RFC 5322)                        │
│   ├── PasswordHash (Value Object: Argon2id/Bcrypt)                     │
│   ├── FullName (Value Object)                                          │
│   ├── UserRole (Enum: CUSTOMER, MERCHANT, COURIER, DISPATCHER, ADMIN)  │
│   └── UserStatus (Enum: ACTIVE, SUSPENDED, PENDING_VERIFICATION)       │
│                                                                        │
│   [Entity: RefreshTokenSession]                                        │
│   ├── TokenID (UUID)                                                   │
│   ├── UserID (FK -> User)                                              │
│   ├── TokenHash (SHA-256 digest of plaintext secret)                   │
│   ├── FamilyID (UUID: Traces session lineage across RTR)               │
│   ├── IsRevoked (Boolean)                                              │
│   └── ExpiresAt (Timestamp UTC)                                        │
│                                                                        │
│   [Entity: OAuthIdentity]                                              │
│   ├── IdentityID (UUID)                                                │
│   ├── UserID (FK -> User)                                              │
│   ├── Provider (Enum: GOOGLE)                                          │
│   ├── ProviderSubjectID (External Unique Sub)                          │
│   └── LinkedEmail (String)                                             │
└────────────────────────────────────────────────────────────────────────┘
```

### Authoritative Tables Owned Exclusively:
1. `users`: Master identity records and role assignments.
2. `refresh_tokens`: Session lifecycle, cryptographically hashed token secrets, rotation family IDs.
3. `oauth_identities`: Third-party provider identity mappings.

---

## 5. Invariants That This Module Must Protect

1. **Email Uniqueness:** No two user accounts may share the same normalized email address (case-insensitive, whitespace-trimmed).
2. **Zero Plaintext Secrets:** Passwords, OAuth client secrets, and refresh token values must **never** be stored, printed in debug logs, or cached in plaintext.
3. **Immutability of Role Self-Assignment:** A client registering via `/api/v1/auth/register` is **always** assigned `role = 'CUSTOMER'`. Only authenticated `ADMIN` users can modify role assignments via `/api/v1/admin/users/{id}/role`.
4. **Token Family Invalidation on Reuse:** If a previously used refresh token is presented again (indicating token theft), the entire `FamilyID` session tree must be revoked immediately (`SEC-004`).
5. **Fail-Closed Authorization:** If credentials, tokens, signatures, or database assertions fail, requests must be rejected immediately with `401 Unauthorized` or `403 Forbidden`. No fallback to default open access is permitted.
6. **Masked Authentication Errors:** Login failure messages must be strictly generic (`"INVALID_CREDENTIALS"`) to prevent email enumeration attacks.

---

## 6. What Other Modules Are Allowed to Request

| Requesting Module | Allowed Operation / Call | Transport / Mechanism |
|:---|:---|:---|
| **All Modules** | `AuthMiddleware()` | Go in-process HTTP middleware |
| **All Modules** | `RequireRole(roles ...Role)` | Go in-process HTTP middleware |
| **All Modules** | `GetSecurityContext(ctx) -> Principal` | Go in-process context helper |
| **Member 4 (Merchants)** | Validate user exists and has role `MERCHANT` | Go in-process query interface `UserFinder` |
| **Member 7 (Admin Gateway)** | Update user status / role via `/api/v1/admin/users/*` | REST API over HTTP/2 |
| **Member 5 (Tracking API)** | Verify access token validity against public key / secret | In-process JWT validation engine |

---

## 7. Ownership of Assets & Artifacts

- **Source Code Directory:** `core-api/internal/identity/`
- **Database Tables:** `users`, `refresh_tokens`, `oauth_identities` (DDL in `migrations/000001_create_identity_tables.up.sql`)
- **API Routes:** `/api/v1/auth/*`, `/api/v1/admin/users/*`
- **Kubernetes Secrets Owned:** `jwt-secrets` (private/public keys or HMAC shared secrets), `integrations-secrets` (Google Client ID & Secret)

---

## 8. Responsibility Matrix (RACI)

| Capability / Action | M1 (Identity) | M4 (Merchants) | M6/M7 (Web) | M7 (Gateway) | M9 (DevOps) |
|:---|:---:|:---:|:---:|:---:|:---:|
| User Registration & Login | **A / R** | I | C | I | I |
| Google OIDC Token Validation | **A / R** | I | C | I | I |
| Access Token Signing & Parsing | **A / R** | I | I | I | I |
| Refresh Token Rotation & Storage | **A / R** | I | C | I | I |
| RBAC Route Guard Enforcement | **A / R** | C | I | I | I |
| Admin Role Promotion / Demotion | **A / R** | I | C (UI) | I | I |
| Edge Ingress Rate Limiting on Login | I | I | I | **A / R** | C |
| JWT Secret Injection in Kubernetes | C | I | I | I | **A / R** |

---

## 9. Assumptions, Unresolved Decisions & Review Items

1. **JWT Algorithm Selection:** ADR-003 and Engineering Standards evaluate RS256 (asymmetric) vs HS256 (symmetric).
   - *Working Agreement:* For initial monolith simplicity, HS256 is fully supported with a shared `JWT_SECRET`; however, asymmetric RS256 is designed into `infrastructure/jwt_provider.go` to allow independent microservice verification (Tracking API) without sharing signing keys.
2. **Account Deletion / GDPR:** Hard deletion cascades would violate parcel audit history invariants. Soft deletion (`status = 'SUSPENDED'` or `'DEACTIVATED'`) with PII anonymization is the agreed policy.
3. **Seed Data Admin Credentials:** Predefined deterministic accounts (`admin@swiftdrop.local`, `customer@swiftdrop.local`, etc.) must be created exclusively via `core-api/cmd/seed/` and prohibited from executing in `APP_ENV=production`.
