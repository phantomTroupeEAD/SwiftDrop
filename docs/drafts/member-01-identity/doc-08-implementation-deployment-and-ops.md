# Document 8 — Implementation, Deployment & Operations Guide

> **Module:** Identity, Access Control & Security  
> **Owning Member:** Member 1 — Identity, Access & Security  
> **Status:** Proposed Draft (For Review in Team Meeting 2)  
> **Target Production Location:** `core-api/internal/identity/`

---

## 1. Technical Stack, Libraries & Versions

- **Programming Language:** Go `1.22+`
- **HTTP Routing:** Standard Library `net/http` / Gin framework compatible
- **Database Driver:** `github.com/jackc/pgx/v5` with connection pooling
- **JWT Cryptography:** `github.com/golang-jwt/jwt/v5`
- **Password Hashing:** `golang.org/x/crypto/argon2` and `golang.org/x/crypto/bcrypt`
- **Validation:** `github.com/go-playground/validator/v10`
- **Testing:** Standard Go `testing`, `github.com/stretchr/testify`, `testcontainers-go`

---

## 2. Environment Variables & Configuration

These environment variables are read during application startup in `core-api/internal/platform/config` matching `docs/operations/environment-variables.md`:

| Variable | Type | Default | Required | Description |
|:---|:---|:---|:---:|:---|
| `JWT_PRIVATE_KEY_PEM` | File/PEM | — | Yes | RS256 private key for asymmetric token signing (`jwt-secrets`). |
| `JWT_PUBLIC_KEY_PEM` | File/PEM | — | Yes | RS256 public key for token verification. |
| `JWT_ACCESS_EXP_MINUTES` | Integer | `15` | No | Access token validity duration in minutes. |
| `JWT_ISSUER` | String | `https://api.swiftdrop.io` | No | Authoritative token issuer (`iss`). |
| `REFRESH_TOKEN_EXPIRY_DAYS`| Integer | `7` | No | Refresh token session validity duration in days (courier: 14). |
| `GOOGLE_CLIENT_ID` | String | — | For OIDC | Google OAuth 2.0 Client ID. |
| `GOOGLE_CLIENT_SECRET` | String | — | For OIDC | Google OAuth 2.0 Client Secret (Kubernetes Secret). |
| `GOOGLE_REDIRECT_URI` | URL | `http://localhost/api/v1/auth/google/callback` | For OIDC | Registered Google redirect callback URL via Traefik. |

---

## 3. Local Development & Testing Instructions

### 3.1 Start Backing Services (PostgreSQL):
From the project root:
```bash
docker compose up -d postgres
```

### 3.2 Run Database Migrations:
```bash
make migrate-up
# Or using golang-migrate directly:
migrate -path core-api/migrations -database "postgres://swiftdrop_app:swiftdrop123@localhost:5432/swiftdrop_db?sslmode=disable" up
```

### 3.3 Run Identity Module Tests:
```bash
# Run pure domain unit tests:
go test -v -race ./core-api/internal/identity/domain/...

# Run transport and integration tests:
go test -v -race ./core-api/internal/identity/transport/...
```

### 3.4 Seed Test Users:
```bash
go run core-api/cmd/seed/main.go
```
This populates standard demo accounts (`admin@swiftdrop.com`, `customer.alice@swiftdrop.com`, etc.) with development credentials configured via the seed script.

---

## 4. Kubernetes Deployment & Operations (Core API Context)

Because the Identity module compiles into the Go Core API modular monolith, its deployment lifecycle is governed by the `core-api` Kubernetes Deployment (`deploy/kubernetes/core-api/deployment.yaml`):

- **Replicas:** 2 baseline (HPA scalable to 4 based on CPU $\ge 70\%$).
- **Service Port:** `8080` (HTTP).
- **Probes:**
  - **Liveness:** `GET /health/live` (Checks HTTP process state only; never calls DB).
  - **Readiness:** `GET /health/ready` (Checks PostgreSQL pool connectivity; fails with 503 if DB is down).
- **Resource Limits:**
  - Requests: `100m` CPU / `256Mi` RAM
  - Limits: `500m` CPU / `512Mi` RAM
- **Graceful Shutdown:** Handles `SIGTERM` with a 20-second connection drain timeout.

---

## 5. Troubleshooting & Operational Runbook

### Issue 1: "AUTH_INVALID_CREDENTIALS" on Seed User Login
- **Cause:** Migrations or seed script was not run, or `APP_ENV=production` blocked the seed script.
- **Remedy:** Execute `make seed` or verify that the user exists via:
  ```sql
  SELECT id, email, role, status FROM users WHERE email = 'customer.alice@swiftdrop.com';
  ```

### Issue 2: "AUTH_TOKEN_REUSE_DETECTED" on Multi-Tab Web Refresh
- **Cause:** Multiple browser tabs triggered simultaneous token refreshes using an already rotated token, triggering instant family revocation.
- **Remedy:** Client browser must redirect to `/login` to establish a clean session. Ensure client-side TanStack Query / Axios refresh mutex serializes refresh requests.

### Issue 3: Google OIDC Callback Fails with "AUTH_OIDC_STATE_MISMATCH"
- **Cause:** User took longer than 5 minutes to complete Google authentication, or browser blocked third-party / cross-site cookies.
- **Remedy:** Ensure `SameSite=Lax` or `SameSite=None; Secure` is configured on the temporary state cookie during cross-origin OAuth redirects.
