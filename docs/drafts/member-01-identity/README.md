# Member 01 — Identity, Access Control & Security Specifications

> **Owner:** Member 01  
> **Status:** Draft Specifications Complete (Ready for Team Review in Meeting 2)  
> **Target Production Location:** `core-api/internal/identity/`  
> **Governing Standards:** SwiftDrop SRS, ADR-003, `AGENTS.md`, and Team Agreements with Member 09

---

## Required Documents Checklist

- [x] [Document 1 — Module Scope & Responsibility Specification](./doc-01-scope-and-responsibilities.md)
- [x] [Document 2 — Functional Flows & Business Rules](./doc-02-functional-flows-and-business-rules.md)
- [x] [Document 3 — Internal Architecture & Design Specification](./doc-03-internal-architecture-and-design.md)
- [x] [Document 4 — Interface & Integration Contract](./doc-04-interface-and-integration-contract.md)
- [x] [Document 5 — Data & Persistence Specification](./doc-05-data-and-persistence.md)
- [x] [Document 6 — Testing Strategy & Acceptance Specification](./doc-06-testing-strategy-and-acceptance.md)
- [x] [Document 7 — Security, Failure & Recovery Specification](./doc-07-security-failure-and-recovery.md)
- [x] [Document 8 — Implementation, Deployment & Operations Guide](./doc-08-implementation-deployment-and-ops.md)

---

## Executive Summary of Member 01 Specifications

The **Identity, Access Control & Security** module provides the authoritative security and principal management foundation for the entire SwiftDrop ecosystem:

1. **Authoritative Account Ownership:** Exclusively owns the `users`, `refresh_tokens`, and `oauth_identities` PostgreSQL tables.
2. **Dual-Token Authentication (ADR-003):** Issues short-lived (15-minute) JWT access tokens paired with single-use Refresh Token Rotation (RTR) stored in `HttpOnly`, `SameSite=Strict`, `Secure` cookies with reuse detection.
3. **Password Security:** Memory-hard Argon2id salted hashing with fallback support for bcrypt. Zero plaintext password persistence.
4. **Google OAuth 2.0 / OIDC:** Complete authorization code flow with cryptographic `state` and `nonce` verification, automated identity linking, and default `CUSTOMER` role mapping.
5. **Reusable Monolith Guards:** Exports Go HTTP middlewares (`AuthMiddleware`, `RequireRole`, `RequireTenant`, `RequireOwnership`) used by all other modular monolith capabilities (`parcels`, `dispatch`, `delivery`, `merchants`, `admin`).
