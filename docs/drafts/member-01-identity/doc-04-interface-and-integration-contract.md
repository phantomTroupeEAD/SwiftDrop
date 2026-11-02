# Document 4 — Interface & Integration Contract

> **Module:** Identity, Access Control & Security  
> **Owning Member:** Member 1 — Identity, Access & Security  
> **Status:** Proposed Draft (For Review in Team Meeting 2)  
> **Target Production Location:** `core-api/internal/identity/`

---

## 1. Exposed HTTP APIs (Machine-Readable Contract)

All endpoints conform to the SwiftDrop API Conventions: base path `/api/v1`, snake_case JSON payload wrapped in standard `{ "data": ... }` envelopes, UTC ISO 8601 timestamps, and standardized error envelopes with HTTP 422 for validation failures.

### Summary Endpoint Inventory

| Method | Path | Auth | Roles Permitted | Description |
|:---|:---|:---|:---|:---|
| `POST` | `/api/v1/auth/register` | None | Public | Register new customer account. |
| `POST` | `/api/v1/auth/login` | None | Public | Local email/password login; issues RS256 JWT + cookie. |
| `POST` | `/api/v1/auth/refresh` | Cookie | Any active session | Rotate single-use refresh token; issue fresh RS256 JWT. |
| `POST` | `/api/v1/auth/logout` | Bearer | Any authenticated | Revoke active session family; clear refresh cookie. |
| `GET` | `/api/v1/auth/me` | Bearer | Any authenticated | Get current user profile and role context. |
| `PUT` | `/api/v1/auth/me` | Bearer | Any authenticated | Update current user profile details (name, phone). |
| `GET` | `/api/v1/auth/google` | None | Public | Initiate Google OIDC authorization code flow. |
| `GET` | `/api/v1/auth/google/callback`| None | Public | Google OIDC redirect callback endpoint. |
| `GET` | `/.well-known/jwks.json` | None | Public | Expose RS256 public JSON Web Key Set for verification. |
| `GET` | `/api/v1/admin/users` | Bearer | `ADMIN` | List users with pagination and role filters. |
| `PUT` | `/api/v1/admin/users/{id}/role`| Bearer | `ADMIN` | Promote or update a user's assigned role. |
| `POST` | `/api/v1/admin/users/{id}/suspend`| Bearer | `ADMIN` | Suspend or lock user account (FR-IAM-007). |
| `POST` | `/api/v1/admin/users/{id}/revoke-sessions`| Bearer | `ADMIN` | Revoke all active refresh sessions for user (FR-IAM-009). |

---

### Detailed Endpoint Specifications

#### 1. Register Customer (`POST /api/v1/auth/register`)
- **Headers:** `Content-Type: application/json`
- **Request Body Schema:**
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["email", "password", "full_name"],
  "properties": {
    "email": { "type": "string", "format": "email", "maxLength": 255 },
    "password": { "type": "string", "minLength": 8, "maxLength": 128 },
    "full_name": { "type": "string", "minLength": 2, "maxLength": 100 },
    "phone": { "type": "string", "pattern": "^\\+?[0-9]{9,15}$" }
  }
}
```
- **Success Response (`201 Created`):**
```json
{
  "data": {
    "id": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
    "email": "customer@example.com",
    "full_name": "Jane Doe",
    "phone": "+94771234567",
    "role": "CUSTOMER",
    "status": "ACTIVE",
    "created_at": "2026-10-25T10:00:00Z"
  }
}
```
- **Error Responses:**
  - `422 Unprocessable Entity`: Validation failure (`VALIDATION_ERROR`).
  - `409 Conflict`: Email already in use (`AUTH_EMAIL_ALREADY_EXISTS`).

---

#### 2. User Login (`POST /api/v1/auth/login`)
- **Headers:** `Content-Type: application/json`
- **Request Body Schema:**
```json
{
  "type": "object",
  "required": ["email", "password"],
  "properties": {
    "email": { "type": "string", "format": "email" },
    "password": { "type": "string" }
  }
}
```
- **Success Response (`200 OK`):**
  - **Set-Cookie Header:** `swiftdrop_refresh_token=rt_xyz123...; Path=/api/v1/auth; HttpOnly; Secure; SameSite=Strict; Max-Age=604800` (for COURIER: `Max-Age=1209600` per ADR-003)
```json
{
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": {
      "id": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
      "email": "customer@example.com",
      "full_name": "Jane Doe",
      "role": "CUSTOMER",
      "status": "ACTIVE",
      "tenant_id": null
    }
  }
}
```
- **Error Responses:**
  - `401 Unauthorized`: Bad credentials (`AUTH_INVALID_CREDENTIALS`).
  - `403 Forbidden`: Suspended account (`AUTH_ACCOUNT_SUSPENDED`).
  - `422 Unprocessable Entity`: Invalid request syntax (`VALIDATION_ERROR`).
  - `429 Too Many Requests`: Exceeded login rate limit (`AUTH_RATE_LIMIT_EXCEEDED`).

---

#### 3. Refresh Access Token (`POST /api/v1/auth/refresh`)
- **Headers:** `Cookie: swiftdrop_refresh_token=...`
- **Success Response (`200 OK`):**
  - **Set-Cookie Header:** Rotated new `swiftdrop_refresh_token`
```json
{
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.new...",
    "token_type": "Bearer",
    "expires_in": 900
  }
}
```
- **Error Responses:**
  - `401 Unauthorized`: Missing, expired, or previously rotated token (`AUTH_SESSION_EXPIRED` or `AUTH_TOKEN_REUSE_DETECTED`).

---

#### 4. Update User Role (`PUT /api/v1/admin/users/{id}/role`)
- **Headers:** `Authorization: Bearer <ADMIN_JWT>`, `Content-Type: application/json`
- **Path Parameter:** `id` (UUIDv7)
- **Request Body:**
```json
{
  "role": "DISPATCHER"
}
```
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "id": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
    "email": "ops@swiftdrop.com",
    "role": "DISPATCHER",
    "status": "ACTIVE",
    "updated_at": "2026-10-25T11:15:00Z"
  }
}
```
- **Error Responses:**
  - `403 Forbidden`: Non-admin actor (`AUTH_INSUFFICIENT_PERMISSIONS`).
  - `404 Not Found`: Target user ID not found (`USER_NOT_FOUND`).
  - `422 Unprocessable Entity`: Invalid role value (`INVALID_ROLE_ENUM`).

---

#### 5. Suspend User Account (`POST /api/v1/admin/users/{id}/suspend`)
- **Headers:** `Authorization: Bearer <ADMIN_JWT>`, `Content-Type: application/json`
- **Path Parameter:** `id` (UUIDv7)
- **Request Body:**
```json
{
  "reason": "Suspicious login activity / payment fraud violation"
}
```
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "id": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
    "status": "SUSPENDED",
    "updated_at": "2026-10-25T11:20:00Z"
  }
}
```
- **Error Responses:**
  - `403 Forbidden`: Insufficient role (`AUTH_INSUFFICIENT_PERMISSIONS`).
  - `404 Not Found`: Target user not found (`USER_NOT_FOUND`).

---

#### 6. Revoke User Sessions (`POST /api/v1/admin/users/{id}/revoke-sessions`)
- **Headers:** `Authorization: Bearer <ADMIN_JWT>`
- **Path Parameter:** `id` (UUIDv7)
- **Success Response (`200 OK`):**
```json
{
  "data": {
    "id": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
    "revoked_sessions": 3,
    "revoked_at": "2026-10-25T11:25:00Z"
  }
}
```
- **Error Responses:**
  - `403 Forbidden`: Insufficient role (`AUTH_INSUFFICIENT_PERMISSIONS`).
  - `404 Not Found`: Target user not found (`USER_NOT_FOUND`).

---

#### 7. JWKS Public Keys (`GET /.well-known/jwks.json`)
- **Headers:** None (Public)
- **Success Response (`200 OK`):**
```json
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "alg": "RS256",
      "kid": "swiftdrop-identity-key-2026-v1",
      "n": "u1g5v...",
      "e": "AQAB"
    }
  ]
}
```

---

## 2. In-Process Go Middleware & Cross-Module Contracts

This module exports reusable middleware functions and domain interfaces used across the modular monolith:

```go
package identity

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Role string

const (
	RoleCustomer   Role = "CUSTOMER"
	RoleMerchant   Role = "MERCHANT"
	RoleCourier    Role = "COURIER"
	RoleDispatcher Role = "DISPATCHER"
	RoleAdmin      Role = "ADMIN"
)

type Principal struct {
	UserID   uuid.UUID
	Email    string
	Role     Role
	TenantID *uuid.UUID // Non-nil for Merchant actors
}

// AuthMiddleware validates the Bearer JWT (RS256) and injects the Principal into context.
func AuthMiddleware(verifier TokenVerifier) func(http.Handler) http.Handler

// RequireRole guards an endpoint against unauthorized roles.
func RequireRole(allowedRoles ...Role) func(http.Handler) http.Handler

// Context Extraction Helpers
func GetPrincipal(ctx context.Context) (Principal, bool)
func GetUserID(ctx context.Context) uuid.UUID
func GetUserRole(ctx context.Context) Role
func GetTenantID(ctx context.Context) (*uuid.UUID, bool)

// --- Cross-Module Exported Interfaces ---

// RoleAssigner allows authorized domain modules (e.g. Member 4 Merchants) to grant roles atomically within their transaction.
type RoleAssigner interface {
	GrantMerchantRole(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error
}

// AuditWriter allows domain modules to commit structured security audit entries into audit_logs within an active database transaction.
type AuditEntry struct {
	ActorID      uuid.UUID
	ActorRole    Role
	Action       string // e.g. "MERCHANT_APPROVED", "USER_SUSPENDED", "ROLE_ASSIGNED"
	TargetType   string
	TargetID     uuid.UUID
	ClientIP     string
	UserAgent    string
	OccurredAt   time.Time
}

type AuditWriter interface {
	Write(ctx context.Context, tx pgx.Tx, entry AuditEntry) error
}

// UserFinder allows other modules to verify user existence and status.
type UserFinder interface {
	FindUserByID(ctx context.Context, userID uuid.UUID) (*Principal, error)
}
```

---

## 3. JWT Claims Specification (RS256)

```json
{
  "iss": "https://api.swiftdrop.io",
  "sub": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
  "aud": "swiftdrop-api",
  "email": "customer@example.com",
  "role": "CUSTOMER",
  "tenant_id": null,
  "iat": 1729854000,
  "nbf": 1729854000,
  "exp": 1729854900,
  "jti": "0192e4a8-9999-7c91-9e12-abcdef123456"
}
```

---

## 4. Integration Matrix with Other Modules

| Module | What Member 1 Exposes | What Member 1 Consumes |
|:---|:---|:---|
| **Member 2 (Parcels)** | `AuthMiddleware`, `GetUserID(ctx)` | None |
| **Member 3 (Dispatch)** | `RequireRole(RoleDispatcher, RoleCourier)` | None |
| **Member 4 (Merchants)** | `Principal.TenantID`, `RoleAssigner.GrantMerchantRole(tx)`, `AuditWriter.Write(tx)`, `UserFinder` | None (Decoupled tenant resolution) |
| **Member 5 (Tracking API)** | Asymmetric RS256 JWT validation via `/.well-known/jwks.json` | None |
| **Member 6 & 7 (Web)** | Login, Register, Google Callback, Refresh, Admin Suspend/Revoke APIs | None |
| **Member 7 (Gateway)** | JWKS endpoint, Admin user/session management endpoints | None |
| **Member 8 (Events)** | Actor metadata for outbox envelopes | None |
| **Member 10 (AI Assistant)** | Forwarded JWT verification on tool endpoints | None |

---

## 5. Compatibility & Versioning Policy

- All HTTP routes maintain the `/api/v1` prefix.
- Additive changes (e.g. adding non-required profile fields or optional claims) are permitted without version increment.
- Any breaking change (e.g. altering JWT claim names, changing error envelopes, or modifying required registration fields) mandates deprecation notice and transition to `/api/v2`.
