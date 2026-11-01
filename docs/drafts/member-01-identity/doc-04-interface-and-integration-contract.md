# Document 4 — Interface & Integration Contract

> **Module:** Identity, Access Control & Security  
> **Owning Member:** Member 1 — Identity, Access & Security  
> **Status:** Proposed Draft (For Review in Team Meeting 2)  
> **Target Production Location:** `core-api/internal/identity/`

---

## 1. Exposed HTTP APIs (Machine-Readable Contract)

All endpoints conform to the SwiftDrop API Conventions: base path `/api/v1`, camelCase JSON payload, UTC ISO 8601 timestamps, and standardized error envelopes.

### Summary Endpoint Inventory

| Method | Path | Auth | Roles Permitted | Description |
|:---|:---|:---|:---|:---|
| `POST` | `/api/v1/auth/register` | None | Public | Register new customer account. |
| `POST` | `/api/v1/auth/login` | None | Public | Local email/password login; issues JWT + cookie. |
| `POST` | `/api/v1/auth/refresh` | Cookie | Any active session | Rotate single-use refresh token; issue fresh JWT. |
| `POST` | `/api/v1/auth/logout` | Bearer | Any authenticated | Revoke active session family; clear refresh cookie. |
| `GET` | `/api/v1/auth/me` | Bearer | Any authenticated | Get current user profile and role context. |
| `PUT` | `/api/v1/auth/me` | Bearer | Any authenticated | Update current user profile details (name, phone). |
| `GET` | `/api/v1/auth/google` | None | Public | Initiate Google OIDC authorization code flow. |
| `GET` | `/api/v1/auth/google/callback`| None | Public | Google OIDC redirect callback endpoint. |
| `GET` | `/api/v1/admin/users` | Bearer | `ADMIN` | List users with pagination and role filters. |
| `PUT` | `/api/v1/admin/users/{id}/role`| Bearer | `ADMIN` | Promote or update a user's assigned role. |

---

### Detailed Endpoint Specifications

#### 1. Register Customer (`POST /api/v1/auth/register`)
- **Headers:** `Content-Type: application/json`
- **Request Body Schema:**
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["email", "password", "fullName"],
  "properties": {
    "email": { "type": "string", "format": "email", "maxLength": 255 },
    "password": { "type": "string", "minLength": 8, "maxLength": 128 },
    "fullName": { "type": "string", "minLength": 2, "maxLength": 100 },
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
    "fullName": "Jane Doe",
    "phone": "+94771234567",
    "role": "CUSTOMER",
    "createdAt": "2026-10-25T10:00:00Z"
  }
}
```
- **Error Responses:**
  - `400 Bad Request`: Validation failure (`VALIDATION_ERROR`).
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
  - **Set-Cookie Header:** `swiftdrop_refresh_token=rt_xyz123...; Path=/api/v1/auth; HttpOnly; Secure; SameSite=Strict; Max-Age=604800`
```json
{
  "data": {
    "accessToken": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
    "tokenType": "Bearer",
    "expiresIn": 900,
    "user": {
      "id": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
      "email": "customer@example.com",
      "fullName": "Jane Doe",
      "role": "CUSTOMER",
      "tenantId": null
    }
  }
}
```
- **Error Responses:**
  - `401 Unauthorized`: Bad credentials (`AUTH_INVALID_CREDENTIALS`).
  - `403 Forbidden`: Suspended account (`AUTH_ACCOUNT_SUSPENDED`).
  - `429 Too Many Requests`: Exceeded login rate limit (`AUTH_RATE_LIMIT_EXCEEDED`).

---

#### 3. Refresh Access Token (`POST /api/v1/auth/refresh`)
- **Headers:** `Cookie: swiftdrop_refresh_token=...`
- **Success Response (`200 OK`):**
  - **Set-Cookie Header:** Rotated new `swiftdrop_refresh_token`
```json
{
  "data": {
    "accessToken": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.new...",
    "tokenType": "Bearer",
    "expiresIn": 900
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
    "email": "ops@swiftdrop.local",
    "role": "DISPATCHER",
    "updatedAt": "2026-10-25T11:15:00Z"
  }
}
```
- **Error Responses:**
  - `400 Bad Request`: Invalid role value (`INVALID_ROLE_ENUM`).
  - `403 Forbidden`: Non-admin actor (`AUTH_INSUFFICIENT_PERMISSIONS`).
  - `404 Not Found`: Target user ID not found (`USER_NOT_FOUND`).

---

## 2. In-Process Go Middleware Contracts

This module exports reusable middleware functions used by all modular monolith components:

```go
package identity

import (
	"context"
	"net/http"
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
	UserID   string
	Email    string
	Role     Role
	TenantID *string // Non-nil for Merchant actors
}

// AuthMiddleware validates the Bearer JWT and injects the Principal into context.
func AuthMiddleware(verifier TokenVerifier) func(http.Handler) http.Handler

// RequireRole guards an endpoint against unauthorized roles.
func RequireRole(allowedRoles ...Role) func(http.Handler) http.Handler

// Context Extraction Helpers
func GetPrincipal(ctx context.Context) (Principal, bool)
func GetUserID(ctx context.Context) string
func GetUserRole(ctx context.Context) Role
func GetTenantID(ctx context.Context) (string, bool)
```

---

## 3. JWT Claims Specification

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
| **Member 4 (Merchants)** | `Principal.TenantID`, `RequireRole(RoleMerchant)` | Merchant verification checks |
| **Member 5 (Tracking API)** | Asymmetric JWT validation (Public Key) | None |
| **Member 6 & 7 (Web)** | Login, Register, Google Callback, Refresh REST APIs | None |
| **Member 8 (Events)** | Actor metadata for outbox envelopes | None |
| **Member 10 (AI Assistant)** | Forwarded JWT verification on tool endpoints | None |

---

## 5. Compatibility & Versioning Policy

- All HTTP routes maintain the `/api/v1` prefix.
- Additive changes (e.g. adding non-required profile fields or optional claims) are permitted without version increment.
- Any breaking change (e.g. altering JWT claim names, changing error envelopes, or modifying required registration fields) mandates deprecation notice and transition to `/api/v2`.
