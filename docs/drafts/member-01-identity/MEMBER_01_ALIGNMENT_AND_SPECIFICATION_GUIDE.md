# Member 01 — Identity, Access & Security Alignment Guide & Review

> **Author / Domain Owner:** Member 1 — Identity, Access & Security (Umesh)  
> **Target Module:** `core-api/internal/identity/`  
> **Review Status:** Post-Submission Comprehensive Audit (Reviewing Full 8-Document Architecture Suite)  
> **Date:** 25 October 2026 | **Version:** 2.0.0

---

## 1. Domain Overview & Scope of Work

The **Identity, Access & Security** module is the security foundation for the entire SwiftDrop system. Every protected endpoint in Core API, Tracking API, and the Web UI depends on Member 1's authentication, JWT token issuance/validation, and Role-Based Access Control (RBAC) middleware.

### 1.1 In-Scope Responsibilities
- User account registration (`CUSTOMER` self-service).
- Email & password authentication with secure password hashing (**Argon2id** or **bcrypt** with cost 12+).
- **Google OAuth 2.0 / OpenID Connect (OIDC)** authentication (verifying issuer, audience, nonce, signature, expiry).
- Stateless JWT issuance with standard claims (`sub`, `email`, `role`, `tenant_id`, `exp`, `iat`, `jti`).
- Token Refresh Rotation (RTR) with family reuse detection (`refresh_tokens` table).
- Reusable Go HTTP middleware (`AuthMiddleware`, `RequireRole(...)`, `RequireTenant(...)`) in `internal/identity/transport/` and exported to `internal/platform/auth/`.
- User account status management (`is_active` flag, `chk_users_role` check constraint).
- Authoring the **very first database migration**: `000001_create_users_and_roles.up.sql`.

### 1.2 Out-of-Scope Responsibilities
- Merchant application approval workflows (Owned by Member 4 and Member 7).
- Courier shift management and vehicle operational status (Owned by Member 3).
- Dispatch board authorization logic (Owned by Member 7 / Member 3).
- Gateway edge rate limiting (Owned by Member 7 / Traefik).

---

## 2. Review of Submitted Architecture Suite (`doc-01` to `doc-08`)

Member 1 has submitted their complete 8-document architecture suite.

### 2.1 Commendable Engineering Strengths
1. **Refresh Token Rotation (RTR) with Token Families:** In `doc-02` and `doc-05`, Member 1 implements advanced session security: a stolen refresh token exchange revokes the entire `family_id` lineage immediately, defeating replay attacks.
2. **Grace Window for Multi-Tab Browsers:** Incorporates a 10-second grace window where an exchanged parent token returns the newly issued child token, preventing accidental logouts when multiple React tabs refresh concurrently.
3. **Strict Clean Architecture:** Completely isolates `domain/` (pure User entities and password hash value objects) from infrastructure (`pgx/v5`, `golang-jwt/jwt/v5`).
4. **DevOps Alignment with Member 9:** Properly configures Core API port `:8080`, probes `/health/live` and `/health/ready` (Postgres check only), and least-privilege DB role `swiftdrop_app`.

---

## 3. Identified Flaws & Contradictions to Resolve

### 3.1 Flaw 1: Migration Naming Convention (Agreement D2)
- **The Issue:** In `doc-05` Section 3, Member 1 specifies `migrations/20261021000001_create_identity_tables.up.sql`.
- **The Fix:** The team and Database Coordinator Member 9 unanimously ratified the **6-digit sequential numbering convention** (Agreement D2). The migration MUST be named:
  `core-api/migrations/000001_create_users_and_roles.up.sql`.

### 3.2 Flaw 2: Role Representation (`users.role` vs `roles` Table)
- **The Issue:** `database-schema.md` originally specified relational tables `roles` and `user_roles`. Member 1's draft defines `users.role VARCHAR(20)` with a `CHECK` constraint:
  ```sql
  CHECK (role IN ('CUSTOMER', 'MERCHANT', 'COURIER', 'DISPATCHER', 'ADMIN'))
  ```
- **The Resolution:** A single role column on `users` is vastly superior in performance and simplicity for SwiftDrop, as human actors hold exactly one primary role. To preserve forward-compatibility with admin role-management endpoints, include both: keep `role VARCHAR(20) NOT NULL DEFAULT 'CUSTOMER'` on `users`, and maintain the `roles` lookup table.

### 3.3 Flaw 3: Seed Account Email Domain Alignment
- **The Issue:** In `doc-05` and `doc-08`, Member 1 uses domain `@swiftdrop.local` (e.g. `admin@swiftdrop.local`), whereas Member 7, 9, and 10 E2E tests use `@swiftdrop.com`.
- **The Fix:** Standardize seed email addresses to `@swiftdrop.com` (e.g. `admin@swiftdrop.com`, `dispatcher@swiftdrop.com`, `courier.bob@swiftdrop.com`).

---

## 4. Authoritative Database Migration: `000001_create_users_and_roles.up.sql`

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Roles Reference Table
CREATE TABLE roles (
    id VARCHAR(32) PRIMARY KEY,
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO roles (id, description) VALUES
('ADMIN', 'Full system administrator'),
('DISPATCHER', 'Logistics coordinator managing parcel routes and fleet'),
('COURIER', 'Field courier delivering packages and capturing proof'),
('MERCHANT', 'Business sender with bulk shipping and webhook integration'),
('CUSTOMER', 'Retail user booking individual parcel deliveries');

-- 2. Master Users Table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255), -- Nullable for Google OIDC users
    full_name VARCHAR(120) NOT NULL,
    phone VARCHAR(32),
    role VARCHAR(32) NOT NULL DEFAULT 'CUSTOMER' REFERENCES roles(id) ON DELETE RESTRICT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Refresh Tokens (RTR & Family Revocation)
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    family_id UUID NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. OAuth Social Identities
CREATE TABLE oauth_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(32) NOT NULL DEFAULT 'GOOGLE',
    provider_subject VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_subject)
);

CREATE UNIQUE INDEX idx_users_email_lower ON users (LOWER(email));
CREATE INDEX idx_users_role ON users (role);
CREATE INDEX idx_refresh_tokens_family ON refresh_tokens (family_id);
CREATE INDEX idx_refresh_tokens_lookup ON refresh_tokens (token_hash) WHERE is_revoked = FALSE;
```

---

## 5. Member 9 DevOps & Infrastructure Configuration

| Config Parameter | Required Specification | Alignment Rule |
| :--- | :--- | :--- |
| **Container & Port** | Runs inside `core-api` on port `:8080` | Agreement A1 |
| **Health Probes** | Shared Core API probes: `/health/live` & `/health/ready` | Agreement A2 |
| **Kubernetes Secrets**| Mounted from `jwt-secrets` (`JWT_SECRET`, `JWT_PRIVATE_KEY_PEM`) and `integrations-secrets` (`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`) | Agreement A5 |
| **Database Role** | Runtime: `swiftdrop_app`; Migrations: `swiftdrop_migrator` | Agreement D6 |
| **Migration File** | `core-api/migrations/000001_create_users_and_roles.up.sql` | Agreement D3 (1st) |

---
*End of Member 01 Specification & Alignment Guide.*
