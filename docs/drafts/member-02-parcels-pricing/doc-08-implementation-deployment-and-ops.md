# SwiftDrop Parcel Delivery & Courier Management System

## Document 8 — Implementation, Deployment & Operations Guide

> **Module:** Member 2 — Parcel Management & Pricing (`core-api/internal/parcels/`, `core-api/internal/pricing/`)
> **Status:** Aligned Operations Guide (Post-Audit v2.0.0 Alignment)
> **Date:** 25 October 2026
> **Order of Precedence:** SwiftDrop SRS v1.0 > Approved Architecture & Contracts > Engineering Standards v1.0 > AI Rulebook (`AGENTS.md`) > Member 2 Specifications

---

## 1. Purpose and Scope

This guide provides operational and implementation instructions for Member 2's **Parcel Management & Pricing** modules within the SwiftDrop Core API Go modular monolith. It specifies package architecture, configuration variables, database migration execution, local Docker Compose setup, containerization standards, health probes, observability telemetry, and operational runbooks.

### 1.1 In-Scope Operational Capabilities
- **Modular Monolith Packages:** `core-api/internal/parcels/` and `core-api/internal/pricing/`.
- **Database Migrations:** Sequential migrations `000004`, `000005`, and `000010` in `core-api/migrations/`.
- **Configuration Management:** Validation and parsing of environment variables (`PRICING_DEFAULT_CURRENCY=LKR`, `PRICING_QUOTE_TTL_MINUTES=15`, etc.).
- **Health Probes:** Liveness (`/healthz`) and readiness (`/readyz`) probe endpoints.
- **Transactional Outbox Coordination:** Emitting `parcel.booked.v1` and `parcel.cancelled.v1` events into `outbox_events` for Member 8's relay daemon.

---

## 2. Technical Stack & Runtime Dependencies

| Component | Standard / Technology | Justification & Rules |
| :--- | :--- | :--- |
| **Language & Toolchain** | Go 1.23+ | Strict type safety, high concurrency, zero-reflection JSON encoding where applicable. |
| **Database Engine** | PostgreSQL 16+ | Authoritative transactional business store; JSONB support for address snapshots. |
| **Database Driver** | `jackc/pgx/v5` with `pgxpool` | High-performance PostgreSQL native driver with connection pooling. |
| **HTTP Router** | `go-chi/chi/v5` | Lightweight, standard-library-compatible HTTP routing and middleware chaining. |
| **Migration Tool** | `golang-migrate/migrate/v4` | Sequential, version-controlled `.sql` up/down migrations. |
| **Asynchronous Bus** | RabbitMQ 3.13+ (AMQP 0-9-1) | Decoupled event bus; consumed via outbox pattern (Member 8). |
| **Containerization** | Docker Engine with multi-stage builds | Minimal, unprivileged scratch/alpine runtime images. |

---

## 3. Package Structure & Clean Layer Architecture

Both `parcels` and `pricing` modules strictly adhere to Clean / Hexagonal Architecture within `core-api/internal/`:

```
core-api/internal/
├── parcels/
│   ├── domain/               # Pure business invariants, aggregates, errors (ZERO external deps)
│   │   ├── parcel.go         # Parcel aggregate root & 9-state machine transitions
│   │   ├── tracking.go       # Crockford Base32 tracking number generator
│   │   └── errors.go         # Domain errors (ErrInvalidTransition, ErrMaxAttemptsExceeded)
│   ├── application/          # Use cases, commands, and repository interfaces
│   │   ├── create_parcel.go  # Book parcel command handler & tx boundary
│   │   ├── get_parcel.go     # Query handlers (detail, list, history, receipt)
│   │   └── repository.go     # ParcelRepository & OutboxWriter interfaces
│   ├── infrastructure/       # PostgreSQL pgx implementations & outbox adapter
│   │   ├── postgres_repo.go  # Atomic SQL statements & version checks
│   │   └── outbox_adapter.go # Writes to shared outbox_events table
│   ├── transport/            # HTTP handlers, request DTOs, response mappers
│   │   ├── handler.go        # Chi HTTP routes (/api/v1/parcels)
│   │   └── dto.go            # Request/Response JSON serialization & validation
│   ├── API.md                # Co-located living API contract
│   └── README.md             # Co-located living module documentation
├── pricing/
│   ├── domain/               # Deterministic calculation engine & monetary values
│   │   ├── formula.go        # Option A threshold (1 kg), divisor 6000 cm³/kg, 7 weight bands (<= 30 kg)
│   │   └── money.go          # Exact integer minor units (LKR cents)
│   ├── application/          # Quote calculation & persistence use cases
│   │   ├── quote_usecase.go  # Evaluates active rule, sets 15m TTL
│   │   └── repository.go     # PricingRuleRepository & QuoteRepository interfaces
│   ├── infrastructure/       # SQL queries for pricing_rules and pricing_quotes
│   │   └── postgres_repo.go  # Rule lookup & quote persistence
│   ├── transport/            # HTTP handler for POST /api/v1/pricing/quotes
│   │   ├── handler.go        # Chi routes (/api/v1/pricing/quotes)
│   │   └── dto.go            # Quote request/response schemas
│   ├── API.md                # Co-located living pricing API contract
│   └── README.md             # Co-located living pricing documentation
└── platform/                 # Shared technical drivers (Auth middleware, DB pool, logging)
```

---

## 4. Environment Variables & Configuration Specification

Configuration is parsed at startup using standard environment variables. Missing mandatory variables cause the process to fail fast.

| Variable Name | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `APP_ENV` | String | `development` | Environment mode (`development`, `staging`, `production`). |
| `HTTP_PORT` | Integer | `8080` | Port for the HTTP REST API server. |
| `DATABASE_URL` | String | (Required) | PostgreSQL connection URI: `postgres://user:pass@host:5432/dbname?sslmode=disable`. |
| `DATABASE_MAX_OPEN_CONNS` | Integer | `25` | Maximum active PostgreSQL pool connections. |
| `DATABASE_MAX_IDLE_CONNS` | Integer | `10` | Maximum idle PostgreSQL pool connections. |
| `DATABASE_CONN_MAX_LIFETIME` | Duration | `15m` | Maximum connection reuse duration. |
| `PARCEL_MAX_WEIGHT_KG` | Float | `30.0` | Maximum allowable actual and chargeable parcel weight. |
| `PARCEL_MAX_DIMENSION_CM` | Float | `150.0` | Maximum allowable single dimension per parcel. |
| `PRICING_DEFAULT_CURRENCY` | String | `LKR` | Authoritative ISO 4217 currency code. |
| `PRICING_QUOTE_TTL_MINUTES` | Integer | `15` | Guaranteed quote freeze duration in minutes. |
| `IDEMPOTENCY_RETENTION_HOURS`| Integer | `24` | Key deduplication cache window in hours. |
| `LOG_LEVEL` | String | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`). |
| `LOG_FORMAT` | String | `json` | Log output format (`json` or `text`). |

### 4.1 Sample Development Configuration (`.env.example`)
```dotenv
APP_ENV=development
HTTP_PORT=8080
DATABASE_URL=postgres://swiftdrop:swiftdrop_dev_password@localhost:5432/swiftdrop?sslmode=disable
DATABASE_MAX_OPEN_CONNS=25
DATABASE_MAX_IDLE_CONNS=10
DATABASE_CONN_MAX_LIFETIME=15m
PARCEL_MAX_WEIGHT_KG=30.0
PARCEL_MAX_DIMENSION_CM=150.0
PRICING_DEFAULT_CURRENCY=LKR
PRICING_QUOTE_TTL_MINUTES=15
IDEMPOTENCY_RETENTION_HOURS=24
LOG_LEVEL=debug
LOG_FORMAT=json
```

---

## 5. Database Migrations & Seed Operations

### 5.1 Owned Migration Sequence
Member 2 owns the following database migrations located in `core-api/migrations/`:
1. `000004_create_pricing_rules.up.sql` (Creates `pricing_rules` and `pricing_quotes` tables).
2. `000005_create_parcels_and_quotes.up.sql` (Creates `parcels` and `parcel_status_history` tables).
3. `000010_create_idempotency_keys.up.sql` (Creates `idempotency_keys` table).

### 5.2 Executing Migrations
Migrations are applied using `golang-migrate`:

```powershell
# Run all forward migrations
migrate -path core-api/migrations -database "postgres://swiftdrop:swiftdrop_dev_password@localhost:5432/swiftdrop?sslmode=disable" up

# Rollback last applied migration
migrate -path core-api/migrations -database "postgres://swiftdrop:swiftdrop_dev_password@localhost:5432/swiftdrop?sslmode=disable" down 1
```

### 5.3 Deterministic Tariff Seeding & Test Fixtures
> [!NOTE]
> **Illustrative Test Fixture Notice (Not Commercial Price Promises):**
> Commercial tariffs are determined by business operations and configured exclusively through Member 7's administration workflow. Member 2 never hard-codes commercial rates. For local testing and development only, database fixtures provide illustrative complete tariffs per zone pair and weight band:
- **Inter-zone Fixture:** `ZONE-WESTERN-01` $\to$ `ZONE-CENTRAL-01`: Illustrative complete tariffs for Bands 1–7 (e.g., Band 1: 35,000 cents, Band 2: 45,000 cents, Band 3: 55,000 cents, Band 4: 75,000 cents, Band 5: 110,000 cents, Band 6: 160,000 cents, Band 7: 220,000 cents).
- **Intra-zone Fixture:** `ZONE-WESTERN-01` $\to$ `ZONE-WESTERN-01`: Illustrative complete tariffs for Bands 1–7.

---

## 6. Local Development Workflow with Docker Compose

### 6.1 Starting Infrastructure
From the repository root, start the supporting PostgreSQL and RabbitMQ containers:
```powershell
docker compose up -d postgres rabbitmq
```

### 6.2 Verifying Database Connectivity
```powershell
docker compose exec postgres psql -U swiftdrop -d swiftdrop -c "SELECT 1;"
```

### 6.3 Running Core API Locally
```powershell
# Navigate to core-api directory
cd core-api

# Execute unit and integration tests
go test -v ./internal/pricing/... ./internal/parcels/...

# Run the API server
go run cmd/api/main.go
```

---

## 7. Containerization & Multi-Stage Dockerfile

The Core API utilizes a production-grade multi-stage Docker build:

```dockerfile
# Stage 1: Build binary
FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /bin/swiftdrop-api ./cmd/api

# Stage 2: Minimal runtime image
FROM alpine:3.20
WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -S swiftdrop && adduser -S swiftdrop -G swiftdrop
USER swiftdrop
COPY --from=builder /bin/swiftdrop-api /app/swiftdrop-api
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1
ENTRYPOINT ["/app/swiftdrop-api"]
```

---

## 8. Health Checks, Telemetry & Observability

### 8.1 Probe Specifications
- **Liveness Probe (`GET /healthz`):** Verifies the HTTP listener is accepting connections. Returns `200 OK` (`{"status":"UP"}`).
- **Readiness Probe (`GET /readyz`):** Executes `SELECT 1` via `pgxpool`. If database connection is active, returns `200 OK` (`{"status":"READY","database":"UP"}`). If database is unreachable, returns `503 Service Unavailable`.

### 8.2 Structured Operational Metrics
Exposed via Prometheus standard endpoint `/metrics`:
- `swiftdrop_parcel_bookings_total{status="success|failed"}`: Counter for booking attempts.
- `swiftdrop_pricing_quotes_total{service_level="standard|express|same_day"}`: Quote request counter.
- `swiftdrop_quote_calculation_duration_seconds`: Histogram of tariff calculation latency.
- `swiftdrop_idempotency_conflicts_total`: Counter for 409 conflict responses.

---

## 9. Alignment & Decision Tracking Matrix

| Operational Topic | Pre-Audit Draft State | Post-Audit Aligned Decision | Source / Status |
| :--- | :--- | :--- | :--- |
| **Pricing Route** | Ambiguous `/pricing/estimate` | `POST /api/v1/pricing/quotes` with quote persistence | Aligned with Member 10 & API Conventions |
| **Pricing Model & Complete Tariff** | Undefined formula / base+per-kg | Complete tariff per zone-pair and weight band: `tariff(pickup, dropoff, band)`; no base/per-kg fees | **PROVISIONAL PROPOSAL (PENDING TEAM CONFIRMATION)** |
| **Weight Bands (7 Bands)** | Undefined band boundaries | 7 non-overlapping bands up to 30 kg; max actual/chargeable weight 30.00 kg | **PROVISIONAL PROPOSAL (PENDING TEAM CONFIRMATION)** |
| **Volumetric Policy & Divisor (6000)** | Volumetric divisor 5000 | Option A: actual weight <= 1 kg uses actual weight only; > 1 kg requires dimensions and uses `max(actual, (L*W*H)/6000)` | **PROVISIONAL PROPOSAL (PENDING TEAM CONFIRMATION)** |
| **Quote Expiration TTL** | Unpersisted / arbitrary TTL | 15 minutes (`PRICING_QUOTE_TTL_MINUTES=15`); backend enforced `current_time >= expires_at` | **PROVISIONAL PROPOSAL (PENDING TEAM CONFIRMATION)** |
| **Monetary Type** | Floating-point fees | Exact integer cents (`BIGINT`, `LKR`) | Engineering Standards v1.0 Section 6 |
| **Database Migrations** | Unnumbered / unspecified | Migrations `000004`, `000005`, `000010` | Database Migration Baseline |
| **Zone Reference Type** | Ambiguous UUID vs VARCHAR | `VARCHAR(64)` matching `delivery_zones.id` | Database Schema Standards |
| **Idempotency Window** | Undefined retention | 24 hours (`IDEMPOTENCY_RETENTION_HOURS=24`) | Engineering Standards v1.0 Section 7 |
| **Delivery Failure Logic** | Ambiguous terminal failure | ADR-004: max 3 attempts (`ATTEMPT_FAILED` $\to$ `RETURNED`) | ADR-004 Failed Delivery Semantics |
| **Tariff Administration** | Mixed Member 2 admin claims | Admin write UI & workflow owned by M7; M2 owns calculation engine | Coordinated Boundary |
| **Receipt File Format** | Undecided JSON vs PDF | Member 2 stores JSON snapshot; M6 renders | `PENDING TEAM CONFIRMATION` |

### 9.1 Pricing Policy Dependency & Operational Impact Analysis

| Operational Configuration | Invariant Aspect | Impact If Pricing Policy Changes |
| :--- | :--- | :--- |
| **`PRICING_QUOTE_TTL_MINUTES`** | Environment-driven configuration parameter. | Can be dynamically adjusted via environment variable without modifying application code; enforced strictly by backend `current_time >= expires_at`. |
| **`PRICING_DEFAULT_CURRENCY`** | Controlled via config; default `LKR`. Stored in integer minor units. | Must remain aligned with system-wide currency standards; no code changes required. |
| **Database Migration 000004** | Table `pricing_rules` structure storing complete tariffs. | If band definitions or zone pairing changes, additive migration scripts coordinate with Member 9 DevOps without affecting existing confirmed parcel bookings. |

---

## 10. Operational Runbook & Acceptance Checklist

- [ ] Core API connects successfully to local PostgreSQL database and passes readiness checks (`/readyz`).
- [ ] Database migrations `000004`, `000005`, and `000010` apply cleanly in order.
- [ ] `POST /api/v1/pricing/quotes` calculates fees deterministically and persists a 15-minute quote.
- [ ] `POST /api/v1/parcels` with valid `Idempotency-Key` creates a parcel and emits `parcel.booked.v1` in `outbox_events`.
- [ ] Replaying the identical booking request with the same `Idempotency-Key` returns cached 201 without duplicate database rows.
- [ ] Public tracking queries expose milestone status but redact recipient phone numbers and street addresses.
- [ ] Docker container builds cleanly without root privileges and passes container health checks.
