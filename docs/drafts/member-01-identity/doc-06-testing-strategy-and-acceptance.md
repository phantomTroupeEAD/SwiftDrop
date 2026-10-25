# Document 6 — Testing Strategy & Acceptance Specification

> **Module:** Identity, Access Control & Security  
> **Owning Member:** Member 1 — Identity, Access & Security  
> **Status:** Proposed Draft (For Review in Team Meeting 2)  
> **Target Production Location:** `core-api/internal/identity/`

---

## 1. Test Strategy Overview

The testing strategy for the Identity module follows the project-wide Testing Pyramid to guarantee that core authentication and authorization invariants are rigorously verified before code is merged.

```
          / \
         / E2E \        Playwright: User register -> Login -> Protected Route (Member 10/1)
        /───────\
       /   API   \      Go httptest: Endpoint status codes, error envelopes, headers
      /───────────\
     / Integration \    PostgreSQL testcontainers: UserRepository, TokenRepository, RTR
    /───────────────\
   /      Unit       \  Go tests: Password hashing, JWT claims, Domain Role guards
  /───────────────────\
```

- **Target Coverage:** $\ge 85\%$ line coverage on `domain/`, `application/`, and `transport/middleware.go`.
- **Zero Mocking on Domain Invariants:** Pure domain tests execute without mocks or I/O.
- **Hermetic Testing:** Integration tests run against ephemeral PostgreSQL Docker instances (`testcontainers-go`).

---

## 2. Test-Case Traceability Matrix (Requirements & Invariants)

| Test ID | Category | Scenario / Action | Precondition / Input | Expected Result | SRS / AT Trace |
|:---|:---|:---|:---|:---|:---|
| **TC-ID-01** | Unit | Valid Registration | Valid email, strong password, name | User aggregate created with role `CUSTOMER`, password hashed | FR-IAM-001, AT-001 |
| **TC-ID-02** | Unit | Weak Password Rejection | Password `< 8` chars or missing uppercase/digit | Returns `ErrInvalidPassword` | SEC-001, SEC-006 |
| **TC-ID-03** | Integration | Duplicate Email Collision | Register with existing email | DB constraint fails, returns `409 Conflict` | FR-IAM-001 |
| **TC-ID-04** | Unit / Int | Successful Login | Correct email and password | Returns valid 15-min JWT + sets `swiftdrop_refresh_token` cookie | FR-IAM-003, AT-001 |
| **TC-ID-05** | Security | Bad Password Handling | Invalid password provided | Returns `401 Unauthorized` (`AUTH_INVALID_CREDENTIALS`) | SEC-001 |
| **TC-ID-06** | Security | Non-Existent Account | Email not in database | Returns identical `401` error in identical response time | SEC-001, AT-004 |
| **TC-ID-07** | Integration | Refresh Token Rotation | Valid refresh cookie presented | Returns fresh JWT + new refresh cookie; old token invalidated | ADR-003, SEC-004 |
| **TC-ID-08** | Security | Refresh Token Reuse Attack | Present already rotated refresh token | Immediate 401; all tokens in `family_id` revoked | SEC-004 |
| **TC-ID-09** | Unit | Middleware Valid Token | `Authorization: Bearer <valid_jwt>` | Context populated with `Principal` (`sub`, `role`) | FR-IAM-004, SEC-002 |
| **TC-ID-10** | Security | Expired Token Presentation | JWT expired 1 minute ago | Request rejected with `401 Unauthorized` | FR-IAM-004, AT-001 |
| **TC-ID-11** | Security | Role Guard Rejection | Customer calls `/api/v1/admin/users` | Middleware returns `403 Forbidden` (`AUTH_INSUFFICIENT_PERMISSIONS`) | SEC-002, AT-004 |
| **TC-ID-12** | Integration | Google OIDC Success | Valid Google code and verified ID token | Account auto-provisioned or linked; JWT returned | FR-IAM-002, AT-014 |
| **TC-ID-13** | Security | Google OIDC Nonce Mismatch | Forged ID token with tampered nonce | Callback rejected with `401 Unauthorized` | AT-014 |
| **TC-ID-14** | Integration | Admin Role Promotion | Admin promotes customer to dispatcher | User role updated in DB to `DISPATCHER` | FR-ADM-004 |
| **TC-ID-15** | Security | Non-Admin Role Tampering | Customer calls `PUT /admin/users/{id}/role` | Returns `403 Forbidden` | FR-ADM-004 |

---

## 3. Mocking Strategy & Test Dependencies

### 3.1 Unit Testing:
- `UserRepository` and `TokenRepository` are mocked using simple struct implementations or Go interfaces.
- `GoogleOIDCClient` is mocked with a deterministic HTTP mock server returning pre-signed test ID tokens.

### 3.2 Integration Testing:
- Uses `testcontainers-go` to spin up a clean `postgres:16-alpine` container on port `5432`.
- Applies real database migrations (`migrations/*.up.sql`).
- Tests execute real SQL queries to verify `UNIQUE`, `CHECK`, and `FOREIGN KEY` constraints.

---

## 4. Execution Commands (Local & CI)

### Run Unit Tests:
```bash
go test -v -race ./core-api/internal/identity/domain/...
go test -v -race ./core-api/internal/identity/application/...
```

### Run Integration & Security Tests:
```bash
go test -v -race -tags=integration ./core-api/internal/identity/infrastructure/...
go test -v -race ./core-api/internal/identity/transport/...
```

### Check Test Coverage:
```bash
go test -coverprofile=coverage.out ./core-api/internal/identity/...
go tool cover -func=coverage.out
```

---

## 5. Acceptance Criteria (Definition of Done for Member 1)

1. **Authentication Baseline:** Local registration, login, logout, and token refresh pass all automated unit and integration tests.
2. **OIDC Federation:** Google login flow succeeds end-to-end in the local development environment (with mock fallback if live credentials are not supplied).
3. **Defense-in-Depth Middleware:** `AuthMiddleware` and `RequireRole` are fully tested, documented, and ready for import by Members 2, 3, 4, 7, and 8.
4. **Zero Plaintext Secrets:** No passwords or cryptographic keys are ever logged or persisted in plaintext.
5. **CI Quality Gate:** All tests pass with `-race` enabled and zero `golangci-lint` depguard violations.
