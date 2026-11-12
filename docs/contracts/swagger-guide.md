# SwiftDrop — Swagger UI & API Annotation Developer Guide

> **Authority:** Mandatory standard for all SwiftDrop developers (Members 01 through 10) and AI coding agents.  
> **Source of Truth:** Shared Contract Specification (`docs/contracts/swagger-guide.md`).

---

## 1. Overview & Unified Architecture

SwiftDrop Core API uses automated, comment-driven OpenAPI documentation powered by `swaggo/swag`. All HTTP endpoints across all domain modules are automatically discovered and aggregated into a single, unified Swagger UI dashboard.

- **Interactive Swagger UI:** [http://localhost:8080/swagger/](http://localhost:8080/swagger/)
- **Unified OpenAPI Spec JSON:** [http://localhost:8080/swagger/spec.json](http://localhost:8080/swagger/spec.json)
- **Local Generation Command:** `make swagger`

Developers and coding agents **do not write manual OpenAPI JSON/YAML files**. Instead, you annotate your Go HTTP handler functions with declarative docstrings. Running `make swagger` parses the entire codebase and regenerates `core-api/docs/swagger.json`.

---

## 2. Standard Annotation Taxonomy

Every HTTP handler in `core-api/internal/<module>/transport/` must include standard `swag` comment tags placed directly above the handler function:

| Annotation | Description | Example |
| :--- | :--- | :--- |
| `@Summary` | One-sentence summary of the endpoint action | `@Summary User login` |
| `@Description` | Detailed business logic, invariants, or caveats | `@Description Validates credentials and returns access and refresh JWT tokens.` |
| `@Tags` | Domain module categorization (lowercase) | `@Tags identity`, `@Tags merchants`, `@Tags events` |
| `@Accept` | Request body MIME type | `@Accept json` |
| `@Produce` | Response body MIME type | `@Produce json` |
| `@Param` | Request parameters (path, query, body) | `@Param body body LoginRequest true "Credentials"` |
| `@Success` | Successful response code and DTO model | `@Success 200 {object} LoginResponse` |
| `@Failure` | Error response codes and RFC-7807 / standard error models | `@Failure 401 {object} httpx.ErrorResponse` |
| `@Security` | Required authentication scheme | `@Security BearerAuth` (omit for public endpoints) |
| `@Router` | Path and HTTP method | `@Router /api/v1/auth/login [post]` |

### Approved Domain Tags
To ensure consistent grouping in Swagger UI, all endpoints must use one of the approved domain tags:
- `identity` — Member 01: Authentication, accounts, roles, and profile lookup.
- `parcels` — Member 02: Parcel booking, dimensions, aggregate state machine.
- `pricing` — Member 02: Tariffs, fee estimation, and quotation calculations.
- `dispatch` — Member 03: Courier assignments, routing zones, dispatch board.
- `delivery` — Member 03: Delivery attempts, failure reasons, proof-of-delivery attachments.
- `merchants` — Member 04: Merchant onboarding, API keys, webhook registrations.
- `events` — Member 08: Admin transactional outbox management and webhook delivery logs.
- `admin` — Member 07 / 01: Administrative oversight and system configurations.
- `system` — Health probes (`/health/live`, `/health/ready`, `/api/v1/health`).

---

## 3. Concrete Handler Annotation Examples

### 3.1 Public Endpoint (No Authentication)
```go
// Login godoc
// @Summary      Authenticate user and issue JWT
// @Description  Validates credentials and returns access token and refresh token.
// @Tags         identity
// @Accept       json
// @Produce      json
// @Param        request body LoginRequest true "User credentials"
// @Success      200 {object} LoginResponse
// @Failure      400 {object} httpx.ErrorResponse "Malformed JSON body"
// @Failure      401 {object} httpx.ErrorResponse "Invalid email or password"
// @Router       /api/v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    // ... handler implementation ...
}
```

### 3.2 Protected Endpoint with Bearer Authentication
```go
// GetMe godoc
// @Summary      Get authenticated user profile
// @Description  Returns user profile and tenant details for the bearer token.
// @Tags         identity
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} UserProfileResponse
// @Failure      401 {object} httpx.ErrorResponse "Unauthorized or expired token"
// @Router       /api/v1/auth/me [get]
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
    // ... handler implementation ...
}
```

### 3.3 Path Parameter Endpoint
```go
// RevokeKey godoc
// @Summary      Revoke an active merchant API key
// @Description  Immediately invalidates the specified API key.
// @Tags         merchants
// @Produce      json
// @Security     BearerAuth
// @Param        key_id path string true "API Key ID"
// @Success      204 "API key successfully revoked"
// @Failure      401 {object} httpx.ErrorResponse "Unauthorized"
// @Failure      404 {object} httpx.ErrorResponse "API key not found"
// @Router       /api/v1/merchants/me/api-keys/{key_id} [delete]
func (h *Handler) revokeKey(w http.ResponseWriter, r *http.Request) {
    // ... handler implementation ...
}
```

---

## 4. How to Generate and Verify Locally

### Step 1: Run the Generator
Whenever you add, modify, or delete an HTTP endpoint, execute:
```bash
make swagger
```
*(Alternatively, without `make`: `cd core-api && go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go -o docs --parseDependency --parseInternal`)*

This command scans the entire `core-api/` tree and regenerates:
- `core-api/docs/swagger.json`
- `core-api/docs/swagger.yaml`

### Step 2: Start the Core API Server
```bash
cd core-api && go run ./cmd/api
```

### Step 3: Open Swagger UI
Navigate to [http://localhost:8080/swagger/](http://localhost:8080/swagger/).
- All endpoints will be displayed in accordion sections sorted by `@Tags`.
- Click the green **Authorize** button at the top right, enter your JWT (`Bearer <token>`), and use the interactive **Try it out** button to execute test requests against the running service.

---

## 5. Zero-Drift Enforcement Policy
- **Synchronous Updates Required:** Any PR or agent change that introduces or refactors an HTTP endpoint must include `swag` annotations and commit the regenerated `core-api/docs/swagger.json`.
- **Automated Verification:** The pre-commit check and CI pipeline run `scripts/verify.sh` / `scripts/verify.ps1`, which validates that the generated Swagger specification precisely matches the codebase.

