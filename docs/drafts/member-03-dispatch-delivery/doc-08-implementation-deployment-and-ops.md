# Document 8 — Implementation, Deployment & Operations Guide

> **Module:** Dispatch, Couriers & Proof of Delivery  
> **Author / Owner:** Member 03  
> **Status:** Draft / Baseline (For Review in Team Meeting 2)  
> **Target Production Code Paths:** `core-api/internal/dispatch/` and `core-api/internal/delivery/`  
> **Container Service:** Core API Modular Monolith (`core-api/`)  
> **Runtime:** Go 1.22+ | **Database:** PostgreSQL 16+ | **Cloud Storage:** Cloudflare R2 / AWS S3 / Local MinIO  
> **Collaborating Roles:** Member 1 (Auth), Member 7 (Frontend/Gateway), Member 9 (DevOps/K8s)

---

## 1. Executive Summary & Operational Scope

Member 03's business capabilities are integrated directly into the **Go Core API Modular Monolith** (`core-api/`). This document provides the engineering and operational manual for:
1. **Application Wiring & Dependency Injection:** Instantiating repositories, cloud storage drivers, use cases, and HTTP routers in `core-api/internal/app/wiring.go`.
2. **Configuration & Environment Variables:** Documenting all runtime settings for PostgreSQL, Cloudflare R2 object storage, and configurable validation thresholds.
3. **Local Development & Docker Compose:** Running the application locally with automated migrations, seed data, and local proof storage.
4. **Kubernetes Deployment & Production Operations:** Health probes, resource requests/limits, secret injection, and graceful shutdown.
5. **Observability & Prometheus Metrics:** Metrics instrumentation for courier workloads, assignment latency, and delivery completion/failure rates.
6. **Operational Runbooks:** Step-by-step procedures for handling field disruptions, driver breakdowns, and disputed deliveries.

---

## 2. Monolith Wiring & Dependency Injection (`wiring.go`)

Per Clean Architecture rules (`AGENTS.md` §4), all dependencies are wired cleanly using constructor injection in `core-api/internal/app/wiring.go`:

```go
package app

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"swiftdrop/core-api/internal/delivery/application"
	deliveryInfra "swiftdrop/core-api/internal/delivery/infrastructure"
	deliveryTransport "swiftdrop/core-api/internal/delivery/transport"
	dispatchApp "swiftdrop/core-api/internal/dispatch/application"
	dispatchInfra "swiftdrop/core-api/internal/dispatch/infrastructure"
	dispatchTransport "swiftdrop/core-api/internal/dispatch/transport"
	"swiftdrop/core-api/internal/platform/config"
)

// WireDispatchAndDelivery wires Member 03's contexts into the root HTTP router
func WireDispatchAndDelivery(r chi.Router, db *sql.DB, cfg *config.Config, logger *slog.Logger) {
	// 1. Infrastructure Adapters
	courierRepo := dispatchInfra.NewPostgresCourierRepository(db)
	assignmentRepo := dispatchInfra.NewPostgresAssignmentRepository(db)
	deliveryRepo := deliveryInfra.NewPostgresDeliveryRepository(db)

	var storageClient application.ObjectStorageClient
	if cfg.StorageDriver == "s3" {
		storageClient = deliveryInfra.NewR2StorageClient(cfg.S3Endpoint, cfg.S3Bucket, cfg.S3AccessKey, cfg.S3SecretKey)
	} else {
		storageClient = deliveryInfra.NewLocalStorageClient(cfg.LocalStoragePath)
	}

	// 2. Application Use Cases & Queries
	assignUseCase := dispatchApp.NewAssignCourierUseCase(db, courierRepo, assignmentRepo)
	cancelAssignmentUseCase := dispatchApp.NewCancelAssignmentUseCase(db, assignmentRepo)
	rescheduleUseCase := dispatchApp.NewRescheduleParcelUseCase(db, courierRepo, assignmentRepo)
	toggleShiftUseCase := dispatchApp.NewToggleShiftUseCase(courierRepo)
	getBoardQuery := dispatchApp.NewGetDispatchBoardQuery(db, courierRepo)

	startRunUseCase := application.NewStartDeliveryRunUseCase(db, deliveryRepo, assignmentRepo)
	batchStartRunUseCase := application.NewBatchStartDeliveryRunUseCase(db, deliveryRepo, assignmentRepo)
	completeDeliveryUseCase := application.NewCompleteDeliveryUseCase(db, deliveryRepo, assignmentRepo, cfg.SignatureMinLength)
	recordFailedAttemptUseCase := application.NewRecordFailedAttemptUseCase(db, deliveryRepo, cfg.DeliveryFailureNoteMinLen, cfg.MaxDeliveryAttempts)
	getManifestQuery := application.NewGetCourierManifestQuery(db, deliveryRepo)

	// 3. Transport Handlers & Route Registration
	dispatchHandler := dispatchTransport.NewDispatchHandler(assignUseCase, cancelAssignmentUseCase, rescheduleUseCase, toggleShiftUseCase, getBoardQuery, courierRepo)
	deliveryHandler := deliveryTransport.NewDeliveryHandler(startRunUseCase, batchStartRunUseCase, completeDeliveryUseCase, recordFailedAttemptUseCase, getManifestQuery, storageClient, cfg.ProofPhotoMaxSizeBytes)

	// Protected Routes (JWT Required)
	r.Route("/api/v1", func(r chi.Router) {
		dispatchHandler.RegisterRoutes(r)
		deliveryHandler.RegisterRoutes(r)
	})

	logger.Info("Member 03 (Dispatch & Delivery) modules wired successfully")
}
```

---

## 3. Environment Variables & Configuration Inventory

All runtime parameters are externalized via environment variables. To guarantee system availability and prevent catastrophic crashes during misconfiguration or cloud outages, **every configurable parameter maintains a hardcoded fallback default in Go code** (`config.LoadDispatchAndDeliveryConfig`). If an environment variable is unset, empty, or unparseable, the runtime logs an audit warning and rolls back immediately to its safe constant default:

### 3.1 Configuration Matrix

| Variable Name | Required? | Hardcoded Rollback Default | Type | Description |
| :--- | :---: | :--- | :---: | :--- |
| `DATABASE_URL` | **Yes (Prod)** | `postgres://postgres:postgres@localhost:5432/swiftdrop?sslmode=disable` | `string` | Authoritative PostgreSQL connection string. |
| `DB_MAX_OPEN_CONNS` | No | `50` | `int` | Maximum open database connections in connection pool. |
| `DB_MAX_IDLE_CONNS` | No | `10` | `int` | Minimum idle database connections maintained. |
| `OBJECT_STORAGE_DRIVER` | No | `local` | `string` | Primary storage engine key: `local` for offline dev/CI; `s3` for production Cloudflare R2 / AWS S3. |
| `STORAGE_DRIVER` | No | `local` | `string` | Backward-compatible alias for `OBJECT_STORAGE_DRIVER`. |
| `LOCAL_STORAGE_PATH` | No | `./uploads/proofs` | `string` | Target directory when storage driver is `local`. |
| `S3_ENDPOINT` | Cond. | *N/A (Required if S3)* | `string` | Cloudflare R2 / AWS S3 API endpoint URI. |
| `S3_BUCKET` | Cond. | *N/A (Required if S3)* | `string` | Dedicated S3 bucket for proof-of-delivery photos. |
| `S3_ACCESS_KEY_ID` | Cond. | *N/A (Required if S3)* | `string` | Cloud storage access key ID credential. |
| `S3_SECRET_ACCESS_KEY`| Cond. | *N/A (Required if S3)* | `string` | Cloud storage secret access key credential. |
| `PRESIGNED_URL_TTL_MINUTES`| No | `15` | `int` | Expiration TTL (minutes) for pre-signed photo view links. |
| `DELIVERY_FAILURE_NOTE_MIN_LENGTH`| No | `15` | `int` | Minimum character length for failure reason `OTHER`. |
| `DEFAULT_MOTORCYCLE_CAPACITY` | No | `25` | `int` | Fallback parcel capacity for `MOTORCYCLE` couriers. |
| `DEFAULT_VAN_CAPACITY` | No | `60` | `int` | Fallback parcel capacity for `VAN` couriers. |
| `DEFAULT_TRUCK_CAPACITY` | No | `120` | `int` | Fallback parcel capacity for `TRUCK` couriers. |
| `MAX_DELIVERY_ATTEMPTS` | No | `3` | `int` | Maximum allowed delivery attempts before terminal `RETURNED`. |
| `PROOF_PHOTO_MAX_SIZE_BYTES` | No | `10485760` (10 MB) | `int64` | Maximum allowable POD photo upload file size. |
| `SIGNATURE_MIN_STROKE_POINTS` | No | `50` | `int` | Minimum character/entropy length for vector signature data. |
| `SHUTDOWN_TIMEOUT_SECONDS` | No | `15` | `int` | Maximum grace period for in-flight transactions during `SIGTERM`. |

---

### 3.2 Resilient Configuration Loader Implementation (`config.go`)

```go
package config

import (
	"log/slog"
	"os"
	"strconv"
)

// Hardcoded Fallback Rollback Defaults
const (
	FallbackStorageDriver                = "local"
	FallbackLocalStoragePath             = "./uploads/proofs"
	FallbackPresignedURLTTLMinutes       = 15
	FallbackDeliveryFailureNoteMinLength = 15
	FallbackMotorcycleCapacity           = 25
	FallbackVanCapacity                  = 60
	FallbackTruckCapacity                = 120
	FallbackMaxDeliveryAttempts          = 3
	FallbackProofPhotoMaxSizeBytes       = 10485760 // 10MB
	FallbackSignatureMinLength           = 50
	FallbackDBMaxOpenConns               = 50
	FallbackDBMaxIdleConns               = 10
	FallbackShutdownTimeoutSeconds       = 15
)

type DispatchAndDeliveryConfig struct {
	DatabaseURL               string
	DBMaxOpenConns            int
	DBMaxIdleConns            int
	StorageDriver             string
	LocalStoragePath          string
	S3Endpoint                string
	S3Bucket                  string
	S3AccessKey               string
	S3SecretKey               string
	PresignedURLTTLMinutes    int
	DeliveryFailureNoteMinLen int
	DefaultMotorcycleCapacity int
	DefaultVanCapacity        int
	DefaultTruckCapacity      int
	MaxDeliveryAttempts       int
	ProofPhotoMaxSizeBytes    int64
	SignatureMinLength        int
	ShutdownTimeoutSeconds    int
}

// LoadDispatchAndDeliveryConfig loads settings with safe hardcoded rollbacks
func LoadDispatchAndDeliveryConfig(logger *slog.Logger) *DispatchAndDeliveryConfig {
	return &DispatchAndDeliveryConfig{
		DatabaseURL:               getEnvString("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/swiftdrop?sslmode=disable", logger),
		DBMaxOpenConns:            getEnvInt("DB_MAX_OPEN_CONNS", FallbackDBMaxOpenConns, logger),
		DBMaxIdleConns:            getEnvInt("DB_MAX_IDLE_CONNS", FallbackDBMaxIdleConns, logger),
		StorageDriver:             getEnvString("OBJECT_STORAGE_DRIVER", getEnvString("STORAGE_DRIVER", FallbackStorageDriver, logger), logger),
		LocalStoragePath:          getEnvString("LOCAL_STORAGE_PATH", FallbackLocalStoragePath, logger),
		S3Endpoint:                os.Getenv("S3_ENDPOINT"),
		S3Bucket:                  os.Getenv("S3_BUCKET"),
		S3AccessKey:               os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretKey:               os.Getenv("S3_SECRET_ACCESS_KEY"),
		PresignedURLTTLMinutes:    getEnvInt("PRESIGNED_URL_TTL_MINUTES", FallbackPresignedURLTTLMinutes, logger),
		DeliveryFailureNoteMinLen: getEnvInt("DELIVERY_FAILURE_NOTE_MIN_LENGTH", FallbackDeliveryFailureNoteMinLength, logger),
		DefaultMotorcycleCapacity: getEnvInt("DEFAULT_MOTORCYCLE_CAPACITY", FallbackMotorcycleCapacity, logger),
		DefaultVanCapacity:        getEnvInt("DEFAULT_VAN_CAPACITY", FallbackVanCapacity, logger),
		DefaultTruckCapacity:      getEnvInt("DEFAULT_TRUCK_CAPACITY", FallbackTruckCapacity, logger),
		MaxDeliveryAttempts:       getEnvInt("MAX_DELIVERY_ATTEMPTS", FallbackMaxDeliveryAttempts, logger),
		ProofPhotoMaxSizeBytes:    getEnvInt64("PROOF_PHOTO_MAX_SIZE_BYTES", FallbackProofPhotoMaxSizeBytes, logger),
		SignatureMinLength:        getEnvInt("SIGNATURE_MIN_STROKE_POINTS", FallbackSignatureMinLength, logger),
		ShutdownTimeoutSeconds:    getEnvInt("SHUTDOWN_TIMEOUT_SECONDS", FallbackShutdownTimeoutSeconds, logger),
	}
}

func getEnvString(key, fallback string, logger *slog.Logger) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	logger.Info("Config key unset; using hardcoded rollback default", "key", key, "default", fallback)
	return fallback
}

func getEnvInt(key string, fallback int, logger *slog.Logger) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
		logger.Warn("Invalid integer in config key; rolling back to hardcoded default", "key", key, "invalid_val", val, "rollback_default", fallback)
	}
	return fallback
}

func getEnvInt64(key string, fallback int64, logger *slog.Logger) int64 {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
		logger.Warn("Invalid int64 in config key; rolling back to hardcoded default", "key", key, "invalid_val", val, "rollback_default", fallback)
	}
	return fallback
}

---

## 4. Local Development & Docker Compose Setup

### 4.1 Running with Local Infrastructure
Member 03's modules can be executed locally with PostgreSQL and MinIO (local S3 substitute) via Docker Compose:

```bash
# 1. Start database and storage dependencies
docker compose up -d postgres minio

# 2. Apply PostgreSQL migrations
migrate -path core-api/migrations -database "postgres://postgres:postgres@localhost:5432/swiftdrop?sslmode=disable" up

# 3. Seed deterministic local couriers and test parcels
psql "postgres://postgres:postgres@localhost:5432/swiftdrop?sslmode=disable" -f deploy/seeds/000006_couriers_seed.sql

# 4. Start Core API in development mode
cd core-api
go run cmd/api/main.go
```

### 4.2 Seed Data Verification Script
To verify that the local environment has seeded properly:
```bash
# Verify active couriers
curl -H "Authorization: Bearer <jwt>" "http://localhost:8080/api/v1/dispatch/couriers"

# Verify dispatch board loads
curl -H "Authorization: Bearer <jwt>" "http://localhost:8080/api/v1/dispatch/board?zone_id=ZONE_COLOMBO_01"
```

---

## 5. Kubernetes Deployment & Production Readiness (Member 9 Collaboration)

In production Kubernetes deployments (`deploy/kubernetes/`):

### 5.1 Health & Readiness Probes
```yaml
livenessProbe:
  httpGet:
    path: /health/live
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
  timeoutSeconds: 2
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /health/ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 2
  failureThreshold: 2
```
- `/health/ready` validates active database connectivity (`db.PingContext`) and local process health only; per Agreement A2, it strictly **does NOT probe Cloudflare R2 / S3 storage** synchronously to prevent cascading pod restart failures during external network blips. Object storage failures are isolated per-request (returning HTTP 503 on proof uploads).

### 5.2 Resource Limits & Sizing
```yaml
resources:
  requests:
    cpu: "250m"
    memory: "256Mi"
  limits:
    cpu: "1000m"
    memory: "512Mi"
```

### 5.3 Graceful Shutdown Protocol
When receiving `SIGTERM` or `SIGINT`, the Core API stops accepting new HTTP connections, waits up to **15 seconds** for active `sql.Tx` transactions to finish writing delivery attempts and outbox events, and then closes database connection pools cleanly.

---

## 6. Observability, Prometheus Metrics & Grafana Dashboards

Member 03 exports native Prometheus metrics via `GET /metrics`:

### 6.1 Metric Catalog
| Metric Name | Type | Labels | Operational Meaning |
| :--- | :---: | :--- | :--- |
| `swiftdrop_dispatch_assignments_total` | Counter | `zone_id`, `status` | Total parcel assignments processed by dispatchers. |
| `swiftdrop_courier_active_workload` | Gauge | `courier_id`, `vehicle_type` | Current active parcel count per courier. |
| `swiftdrop_delivery_completions_total` | Counter | `proof_type` | Total successfully delivered parcels partitioned by proof type. |
| `swiftdrop_delivery_failures_total` | Counter | `reason_code` | Total failed delivery attempts partitioned by failure classification. |
| `swiftdrop_assignment_duration_seconds`| Histogram | `zone_id` | Latency distribution of pessimistic row-lock assignments. |

### 6.2 Recommended Grafana Alerting Rules
- **Courier Overload Alert:** Triggers if any courier's `swiftdrop_courier_active_workload` exceeds `max_capacity` (indicates invariant breach).
- **High Failure Rate Alert:** Triggers if failed delivery attempts exceed $15\%$ of total attempts over a rolling 1-hour window.
- **Dispatch Lock Contention Alert:** Triggers if p95 `swiftdrop_assignment_duration_seconds` exceeds 1.5 seconds (indicates database row-lock contention).

---

## 7. Operational Runbooks for Dispatchers & Engineers

### Runbook 1: Courier Vehicle Breakdown On Road (`OUT_FOR_DELIVERY`)
- **Situation:** A driver calls dispatch: their motorcycle broke down with 15 parcels in transit.
- **Procedure:**
  1. Dispatcher opens Dispatch Portal $\rightarrow$ Courier Workload $\rightarrow$ Selects disabled driver.
  2. For parcels on board, dispatcher records operational disruption via `POST /api/v1/deliveries/{id}/failed` with reason `DISPATCH_INTERRUPTED` and note: *"Driver breakdown on Galle Rd; relief van dispatched"*.
  3. Relief driver physically meets disabled driver and takes physical custody of the parcels.
  4. Dispatcher reassigns the parcels to the relief driver via `POST /api/v1/dispatch/assignments`.
  5. Relief driver clicks **"Start Run"** (`POST /api/v1/courier/parcels/{id}/start-run`).

### Runbook 2: Investigating a Disputed Delivery Handover
- **Situation:** A customer calls customer support claiming: *"My tracking says delivered, but I never received my box."*
- **Procedure:**
  1. Support agent opens `GET /api/v1/deliveries/parcels/{id}`.
  2. Inspects `received_by_name`, timestamp, and proof type.
  3. If `proof_type == 'PHOTO'`: Support agent clicks "View Photo". The server generates a 15-minute temporary pre-signed URL to the Cloudflare R2 image showing the doorstep photo.
  4. If `proof_type == 'SIGNATURE'`: Agent inspects the drawn vector signature and recipient relationship name to verify who signed for the item.

### Runbook 3: Emergency Administrative Courier Suspension
- **Situation:** A driver violates safety policies or loses their company mobile phone.
- **Procedure:**
  1. Administrator logs into admin dashboard or issues API call:
     ```bash
     curl -X POST -H "Authorization: Bearer <admin_jwt>" \
          -H "Content-Type: application/json" \
          -d '{"status": "SUSPENDED"}' \
          "https://api.swiftdrop.io/api/v1/dispatch/couriers/usr_courier_bob/status"
     ```
  2. The courier's shift status is set to `SUSPENDED`.
  3. The courier is immediately blocked from accepting new assignments or updating delivery records. Existing assigned parcels are returned to the dispatch board for reassignment.
