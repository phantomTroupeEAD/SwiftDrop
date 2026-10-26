# ADR-003: Authentication, Session & Merchant Credential Strategy

> **Status:** Accepted  
> **Date:** 2026-10-21  
> **Owner:** Member 1 (Identity, Access & Security)  
> **Deciders:** SwiftDrop Engineering Team  
> **Resolves:** SRS §8.1 (Authentication & Authorization), §13.2 (Authentication storage), §17.2 (Open Decision #3)

---

## 1. Context & Problem Statement

SwiftDrop serves four distinct client personas across public and private networks:
1. **Interactive Web Portal Users:** Customers, Merchants, Dispatchers, and Administrators using the React SPA.
2. **Couriers on Mobile Web:** Couriers accessing the mobile-optimized driver interface in fluctuating network conditions.
3. **B2B Merchant Systems:** Automated backend ERPs and e-commerce stores creating parcels and subscribing to webhooks.
4. **Third-Party Identity Providers:** Users authenticating via Google OAuth 2.0 / OpenID Connect (OIDC).

SRS §8.1 and §13.2 mandate an explicit architectural decision on token lifetimes, session storage in browsers, credential hashing, and revocation mechanisms prior to core backend implementation.

---

## 2. Decision Summary

We adopt a **Dual-Token Architecture with Encrypted HttpOnly Cookies for Interactive Clients** and **Hashed API Keys for Automated B2B Merchant Clients**:

| Persona | Primary Credential | Delivery / Storage | Lifetime | Revocation Mechanism |
| :--- | :--- | :--- | :--- | :--- |
| **Portal User (Web SPA)** | Short-Lived JWT Access Token | In-Memory (React AuthContext) | 15 Minutes | Expires naturally; refreshed via cookie |
| **Portal Session Refresh** | Opaque Refresh Token | `HttpOnly`, `Secure`, `SameSite=Strict` Cookie | 7 Days | Single-use rotation; stored in DB session table |
| **Courier Mobile Web** | Short-Lived JWT + Refresh Token | Encrypted local storage / HttpOnly Cookie | 15m / 14 Days | Session revocation in Core API |
| **B2B Merchant API** | Hashed API Secret Key (`sd_live_...`) | HTTP Request Header (`X-API-Key`) | Up to 1 Year | Database revocation timestamp / active flag |

---

## 3. JWT Access Token Design

### 3.1 Signing Algorithm & Key Management
- **Algorithm:** Asymmetric **RS256** (RSA Signature with SHA-256) or **EdDSA** (Ed25519). Core API holds the private signing key (`JWT_PRIVATE_KEY_PEM`). Tracking API and downstream microservices verify tokens using the published public key (`/.well-known/jwks.json`), eliminating cross-service database roundtrips for auth checks.
- **Clock Skew Tolerance:** Maximum 60 seconds.

### 3.2 Token Claims Structure
Every access token payload strictly contains:

```json
{
  "iss": "https://api.swiftdrop.io",
  "sub": "0192e4a8-6b2a-7c91-9e12-3456789abcde",
  "role": "MERCHANT",
  "tenant_id": "0192e4a8-7777-7c91-9e12-9876543210ab",
  "email": "logistics@vendor.com",
  "exp": 1729512900,
  "nbf": 1729512000,
  "iat": 1729512000,
  "jti": "0192e4a8-9999-7c91-9e12-abcdef123456"
}
```

- `sub` (*Subject*): Authoritative internal user UUID.
- `role`: Exactly one primary role from `[ADMIN, DISPATCHER, COURIER, MERCHANT, CUSTOMER]`.
- `tenant_id`: Mandatory for `MERCHANT` and merchant-scoped actors to enforce multi-tenant isolation.
- `jti`: Unique token identifier for emergency blocklisting.

### 3.3 Access Token Verification Rules
HTTP middleware in `core-api/internal/platform/auth/` evaluates:
1. Valid RSA signature using current active public key.
2. `exp` is in the future and `nbf` <= current UTC time.
3. `iss` matches `https://api.swiftdrop.io` (or local development issuer).
4. Required claims (`sub`, `role`) are present.
5. Context is enriched with `user_id`, `role`, and `tenant_id` for downstream handlers.

---

## 4. Browser Session & Refresh Token Mechanism

### 4.1 Token Storage Security (Preventing XSS and CSRF)
- **STRICT PROHIBITION:** Access tokens and refresh tokens **MUST NEVER** be stored in browser `localStorage` or `sessionStorage` for desktop portals. `localStorage` is vulnerable to Cross-Site Scripting (XSS) exfiltration.
- **Access Token:** Kept in memory within React application state.
- **Refresh Token:** Transmitted in a secure cookie:
  ```http
  Set-Cookie: swiftdrop_refresh_token=rt_98765abcde...; 
              Path=/api/v1/auth; 
              HttpOnly; 
              Secure; 
              SameSite=Strict; 
              Max-Age=604800
  ```
  *(For couriers, Max-Age is 1209600 / 14 days)*.
  - `Path=/api/v1/auth`: Cookie is only sent to authentication refresh endpoints, reducing exposure across standard API calls.
  - `HttpOnly`: Inaccessible to client JavaScript.
  - `SameSite=Strict`: Immune to Cross-Site Request Forgery (CSRF).

### 4.2 Refresh Token Rotation (RTR) & Reuse Detection
1. When `/api/v1/auth/refresh` is called, the provided refresh token is looked up in the database.
2. If valid, the old refresh token is immediately marked `used` or deleted, and a brand-new refresh token and access token are issued.
3. **Replay Detection:** If a previously used refresh token is presented again (indicating token theft), all sessions associated with that user family are instantly invalidated, and an audit alert is triggered (`SEC-004`).

---

## 5. Google OAuth 2.0 / OpenID Connect (OIDC) Flow

For social login and customer/merchant onboarding:
1. **Initiation (`/api/v1/auth/google`):**
   - Core API generates a cryptographically random, signed `state` parameter and a SHA-256 `nonce` stored in a short-lived HTTP cookie.
   - Redirects user to Google OAuth consent screen requesting scopes `openid email profile`.
2. **Callback (`/api/v1/auth/google/callback`):**
   - Core API verifies the returned `state` cookie to prevent CSRF.
   - Exchanges authorization code with Google token endpoint for an `id_token`.
   - Validates `id_token` signature, audience (`aud`), issuer (`iss`), and `nonce`.
3. **Account Provisioning & Linking:**
   - Checks `oauth_identities` table for `provider = 'GOOGLE'` and `provider_subject = sub`.
   - If not found, checks `users` table for matching email:
     - If user exists, links Google identity to existing account (requires verified email).
     - If user does not exist, registers new user with role `CUSTOMER` (or initiates `MERCHANT` onboarding).
   - Issues SwiftDrop JWT access token and refresh cookie.

---

## 6. B2B Merchant API Key Architecture

### 6.1 Key Generation & Format
Automated merchant integrations bypass JWT workflows using long-lived API keys:
- **Format:** `sd_live_<8-character-prefix>_<32-character-random-entropy>` (e.g. `sd_live_a1b2c3d4_9f8e7d6c5b4a3...`).
- **Storage:** The raw secret is displayed to the merchant **EXACTLY ONCE** upon generation.
- **Database Model:**
  - `key_prefix`: `a1b2c3d4` (for fast indexing and merchant dashboard identification).
  - `key_hash`: Argon2id or salted SHA-256 hash of the complete secret key.
  - `scopes`: JSON array of permissions (`["parcels:create", "parcels:read", "webhooks:manage"]`).
  - `revoked_at`: Nullable timestamp.

### 6.2 Authentication Middleware
When an incoming request presents `X-API-Key`:
1. Middleware extracts the prefix and performs a single indexed query:
   ```sql
   SELECT id, merchant_id, key_hash, scopes, revoked_at, expires_at 
   FROM merchant_api_credentials 
   WHERE key_prefix = $1;
   ```
2. Validates constant-time hash equality.
3. Enforces active status (`revoked_at IS NULL` and `expires_at > NOW()`).
4. Injects `tenant_id = merchant_id` and role `MERCHANT` into request context.

---

## 7. Password Hashing Specification

For username/password local accounts:
- **Algorithm:** **Argon2id** (RFC 9106)
- **Parameters:**
  - Memory: 64 MB (`m=65536`)
  - Iterations / Time Cost: 3 (`t=3`)
  - Parallelism: 2 threads (`p=2`)
  - Salt length: 16 bytes cryptographically secure random (`crypto/rand`)
  - Key length: 32 bytes

---

## 8. Compliance & Security Traceability

| Requirement | Implementation in ADR-003 |
| :--- | :--- |
| **SEC-001** (Secure authentication) | Argon2id for passwords; RS256 JWT; OAuth2 state/nonce. |
| **SEC-002** (Role-Based Access Control) | Explicit `role` claim in JWT enforced in handler layer. |
| **SEC-003** (Credential Storage) | Zero plaintext storage; hashed API keys and passwords. |
| **SEC-004** (Session Management) | Short 15-min JWT; HttpOnly cookie refresh with rotation. |
| **SEC-005** (Multi-tenant Isolation) | `tenant_id` claim strictly enforced on merchant resources. |
