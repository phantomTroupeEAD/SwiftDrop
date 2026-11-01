# Core API Platform Foundation Packages (`internal/platform`)

> **Domain:** Shared Cross-Cutting Technical Drivers & HTTP Platform Middleware  
> **Primary Owner:** Core API Foundation (Maintained by Member 8 — Event-Driven Backend & Integrations)  
> **Package Isolation Rule:** In accordance with DDD Architecture Layering Rules, **all domain modules** (`parcels`, `dispatch`, `delivery`, `merchants`, `identity`, `admin`, `events`, `pricing`) may import `internal/platform/*`. Conversely, **`internal/platform` must NEVER import any internal domain module**. This is enforced automatically by CI and `golangci-lint` `depguard`.

---

## 1. Package Overview & Architecture

The `platform` directory contains foundational technical packages designed to prevent duplication across domain modules and standardize configuration, persistence connection pooling, request correlation, error responses, and HTTP serialization.

```
internal/platform/
├── config/                  # Environment variable parsing, fail-fast validation & APP_ENV
├── db/                      # PostgreSQL connection pooling, Querier interface & WithTx
├── middleware/              # X-Request-ID propagation, correlation ID, panic recovery & access logging
└── httpx/                   # Canonical REST envelopes, error serializers, strict JSON decoding & cursors
```

---

## 2. Package Breakdown & Usage Guide

### 2.1 `platform/config`
Provides strongly-typed environment parsing with fallback defaults. Values are never printed or logged to protect secrets.

```go
import "github.com/swiftdrop/core-api/internal/platform/config"

// Parse configuration safely
port := config.String("PORT", "8080")
batchSize := config.Int("BATCH_SIZE", 50)
pollInterval := config.Duration("POLL_INTERVAL", 500*time.Millisecond)

// Require mandatory secret (fails fast if unset)
jwtSecret, err := config.Require("JWT_SECRET")

// Check environment
if config.IsProduction() {
    // enforce strict TLS
}
```

---

### 2.2 `platform/db`
Provides PostgreSQL connection management, atomic transaction lifecycle helpers (`WithTx`), and a unified `Querier` interface satisfied by both `*sql.DB` and `*sql.Tx`.

#### Connection Pool Budget Formula (Architecture §4.4)
Across all horizontally autoscaled stateless Core API pods, the aggregate connection pool must obey:

$$\text{Replicas}_{\text{max}} \times \text{MaxOpenConns} \le \text{PostgreSQL MaxConnections}$$

*Example:* With 3 max replicas and `MaxOpenConns = 10`, the total maximum connection count is:
$$3 \times 10 = 30 \le 100\text{ (standard PostgreSQL limit)}$$

#### Usage Snippets:
```go
import "github.com/swiftdrop/core-api/internal/platform/db"

// 1. Initialize pooled connection with verified limits
database, err := db.Open(ctx, "postgres", dbURL, db.DefaultOptions())

// 2. Execute transactional business operation with automatic commit/rollback
err := db.WithTx(ctx, database, func(tx *sql.Tx) error {
    if err := parcelRepo.Save(ctx, tx, parcel); err != nil {
        return err // automatically rolls back tx
    }
    return outboxWriter.Write(ctx, tx, event) // atomically committed together
})
```

---

### 2.3 `platform/middleware`
Standardizes cross-cutting HTTP request handling, distributed tracing, and panic safety.

- **`RequestID`:** Inspects incoming `X-Request-ID` (validating `^[a-zA-Z0-9_\-]{1,128}$`). If missing or invalid, generates a time-ordered RFC 9562 UUIDv7. Injects ID into request context and echoes in the `X-Request-ID` response header.
- **`CorrelationID(ctx)`:** Extracts correlation ID from context to pass into `EventEnvelope.CorrelationID`.
- **`Recover(logger)`:** Catches panics, logs structured details without PII, and returns standard HTTP 500 JSON error envelope.
- **`AccessLog(logger)`:** Emits structured JSON access log lines (`method`, `path`, `status`, `duration_ms`, `correlation_id`). Strictly omits query strings, bodies, authorization headers, and tokens.
- **`BodyLimit(maxBytes)`:** Enforces maximum request body size via `http.MaxBytesReader`.

---

### 2.4 `platform/httpx`
Standardizes API response and request formats adhering to [`docs/contracts/api-conventions.md`](../../../docs/contracts/api-conventions.md).

#### Standard JSON Response:
```go
import "github.com/swiftdrop/core-api/internal/platform/httpx"

// Write single resource: {"data": {...}}
httpx.WriteData(w, http.StatusOK, parcelResponse)

// Write canonical error envelope: {"error": {"code": "...", "message": "...", "correlation_id": "...", "timestamp": "..."}}
httpx.WriteError(w, r, http.StatusUnprocessableEntity, "PARCEL_INVALID_WEIGHT", "Weight exceeds limit", httpx.ErrorDetail{
    Field: "weight_kg",
    Issue: "Must be <= 30.0 kg",
})
```

#### Strict Request Decoding:
```go
var req CreateParcelRequest
// Fails on unknown fields, extra trailing tokens, or bodies exceeding 1MB
if err := httpx.DecodeJSON(w, r, &req, 1<<20); err != nil {
    httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST_PAYLOAD", err.Error())
    return
}
```

#### Cursor Pagination:
```go
// Generate opaque tamper-safe cursor
cursor := httpx.EncodeCursor(parcel.CreatedAt(), parcel.ID().String())

// Decode cursor
createdAt, id, err := httpx.DecodeCursor(queryCursor)
if err != nil {
    httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "Cursor is malformed")
    return
}
```

---

## 3. Dependency Inversion & Isolation Rule

To preserve modular monolith boundaries and avoid circular dependencies:
- **Rule:** Modules $\to$ Platform (Allowed)
- **Rule:** Platform $\to$ Modules (**STRICTLY PROHIBITED**)

Linter rule `platform_isolation` inside `.golangci.yml` prevents any package inside `internal/platform/` from importing `internal/identity`, `internal/parcels`, `internal/pricing`, `internal/dispatch`, `internal/delivery`, `internal/merchants`, `internal/admin`, or `internal/events`.

