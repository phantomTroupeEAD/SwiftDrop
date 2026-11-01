# SwiftDrop — Environment Variables & Configuration Reference

> **Authority:** Fulfills SRS §11.3 (Environment Variables) and Engineering Standards §11 (Configuration Management).  
> **Rule:** Never commit production values, passwords, or private keys to the repository. Use `.env.example` templates and Kubernetes Secrets.

---

## 1. Global & Common Variables

Variables shared across multiple backend services and daemons:

| Variable Name | Description | Default / Example Value | Required? | Secret? | Consumed By |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `APP_ENV` | Application runtime environment (`local`, `development`, `staging`, `production`). | `local` | **Yes** | No | All services |
| `LOG_LEVEL` | Minimum log output level (`debug`, `info`, `warn`, `error`). | `info` | No | No | All services |
| `DATABASE_URL` | PostgreSQL connection DSN (`postgres://user:pass@host:5432/dbname?sslmode=disable`). Each service logs in with its own role (D6): core-api `swiftdrop_app`, tracking `swiftdrop_tracking_ro`, relay `swiftdrop_relay`, worker `swiftdrop_worker`, reconciler `swiftdrop_reconciler`. In Kubernetes the value comes from the matching key of `postgres-secrets` (§7). Never the `swiftdrop` superuser. | `postgres://swiftdrop_app:<change-me-local-only>@postgres:5432/swiftdrop_db?sslmode=disable` | **Yes** | **Yes** | Core API, Tracking, Outbox Relay, Event Worker, Reconciler |
| `DB_MAX_OPEN_CONNS` | Maximum database connection pool size per pod (D8 budget: Core 12, Tracking 5, Relay 5, Worker 10, Reconciler 2; max 105 active vs 150 server limit). | `12` | No | No | Core API, Tracking, Outbox Relay |
| `DB_MAX_IDLE_CONNS` | Maximum idle connection pool size per pod. | `5` | No | No | Core API, Tracking, Outbox Relay |
| `RABBITMQ_URL` | AMQP 0-9-1 connection URI (cluster user `swiftdrop`, vhost `/`). | `amqp://swiftdrop:swiftdrop_dev@rabbitmq:5672/` | **Yes** | **Yes** | Outbox Relay, Event Worker, Tracking |
| `OTEL_EXPORTER_OTLP_ENDPOINT`| OpenTelemetry Collector gRPC/HTTP endpoint. | `http://tempo:4317` | No | No | Core API, Tracking, Worker |

---

## 2. Core API (`core-api`)

Primary backend modular monolith (HTTP Port `8080`, Probes at `/health/live`, `/health/ready`, `/metrics`):

| Variable Name | Description | Default / Example Value | Required? | Secret? |
| :--- | :--- | :--- | :--- | :--- |
| `HTTP_ADDR` | TCP address and port for HTTP server to listen on. | `:8080` | **Yes** | No |
| `PUBLIC_BASE_URL` | Public-facing base domain URL. | `https://swiftdrop.example.com` (one domain, path routing C17; real domain still open) (local: `http://localhost`) | **Yes** | No |
| `JWT_PRIVATE_KEY_PEM` | Path to the RSA private key PEM file for minting access tokens. In Kubernetes, `jwt-secrets` is mounted as files under `/etc/secrets/jwt/`, because multi-line PEM text in env vars breaks line endings (Member 1, 2026-11-01). | `/etc/secrets/jwt/private.pem` | **Yes** | **Yes** |
| `JWT_PUBLIC_KEY_PEM` | Path to the RSA public key PEM file for validating tokens. | `/etc/secrets/jwt/public.pem` | **Yes** | No |
| `JWT_ACCESS_EXP_MINUTES`| Lifetime duration for access tokens. | `15` | No | No |
| `COOKIE_SECURE` | Secure flag on the refresh-token cookie. Must be `true` in production (HTTPS only). Local Docker runs plain HTTP, so it must be `false` there, or the browser drops the cookie (Member 1, 2026-11-01). | `false` (local), `true` (production) | **Yes** | No |
| `GOOGLE_CLIENT_ID` | OAuth 2.0 client ID from Google Cloud Console. | `123456789-abc.apps.googleusercontent.com` | **Yes** | No |
| `GOOGLE_CLIENT_SECRET` | OAuth 2.0 client secret. | `GOCSPX-abc123xyz...` | **Yes** | **Yes** |
| `GOOGLE_REDIRECT_URI` | Authorized OAuth redirect callback URL, through the gateway. The path `/api/v1/auth/google/callback` follows SRS §6.2 (confirmed by Member 1, 2026-11-01). It must match the URI registered in Google Cloud Console exactly, or sign-in fails with `redirect_uri_mismatch`. | `http://localhost/api/v1/auth/google/callback` (production: `https://swiftdrop.example.com/api/v1/auth/google/callback`) | **Yes** | No |
| `IDEMPOTENCY_EXPIRY_HOURS`| Lifetime before stored idempotency cache rows expire. | `24` | No | No |
| `WEBHOOK_SECRET_ENCRYPTION_KEY`| 32-byte AES-256-GCM hex key for encrypting merchant webhook secrets (shared with Event Worker). | `0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef` | **Yes** | **Yes** |
| `MERCHANT_API_KEY_PEPPER`| Server-side pepper string for merchant API key HMAC hashing. | *(Secret pepper string)* | **Yes** | **Yes** |
| `OBJECT_STORAGE_ENDPOINT` / `S3_ENDPOINT` | S3-compatible storage endpoint URL. | `http://minio:9000` (local) or Cloudflare R2 | No | No |
| `OBJECT_STORAGE_BUCKET` / `S3_BUCKET` | Bucket name for proof-of-delivery attachments. | `swiftdrop-pod-attachments` | No | No |
| `OBJECT_STORAGE_ACCESS_KEY` / `S3_ACCESS_KEY_ID`| S3 access key ID. | `minioadmin` | No | **Yes** |
| `OBJECT_STORAGE_SECRET_KEY` / `S3_SECRET_ACCESS_KEY`| S3 secret access key. | `minioadmin` | No | **Yes** |

---

## 3. Tracking API (`services/tracking`)

Read-optimized public milestone and SSE projection service (HTTP Port `8081`, Probes at `/health/live`, `/health/ready`, `/metrics`):

| Variable Name | Description | Default / Example Value | Required? | Secret? |
| :--- | :--- | :--- | :--- | :--- |
| `TRACKING_HTTP_ADDR` | Listen address for Tracking service. | `:8081` | **Yes** | No |
| `JWT_PUBLIC_KEY_PEM` | Path to the shared public key file for optional authenticated stream validation. Tracking mounts only `public.pem` from `jwt-secrets`. | `/etc/secrets/jwt/public.pem` | No | No |
| `SSE_HEARTBEAT_SECONDS` | Interval in seconds between keep-alive SSE comments (`: ping` or `:keepalive`). | `15` | No | No |
| `REDIS_URL` | Optional Redis URL for cross-replica SSE fan-out. | `redis://redis:6379/0` | No | No |

---

## 4. Asynchronous Messaging & Workers (`services/messaging/`)

### 4.1 Outbox Relay Daemon (`services/messaging/relay`, Port `8082`)
| Variable Name | Description | Default / Example Value | Required? | Secret? |
| :--- | :--- | :--- | :--- | :--- |
| `OUTBOX_POLL_INTERVAL_MS`| Interval between outbox table scans. | `500` (ms) | No | No |
| `OUTBOX_BATCH_SIZE` | Max pending rows processed per batch. | `50` | No | No |
| `OUTBOX_STALE_SECONDS` | Age threshold in seconds before unconfirmed messages are retried. | `60` | No | No |

### 4.2 Event Worker Service (`services/messaging/worker`, Port `8083`)
| Variable Name | Description | Default / Example Value | Required? | Secret? |
| :--- | :--- | :--- | :--- | :--- |
| `WORKER_CONCURRENCY` | Number of parallel worker goroutines per pod. | `10` | No | No |
| `WEBHOOK_HTTP_TIMEOUT_SECONDS` | Socket timeout for outbound merchant webhooks. | `5` | No | No |
| `WEBHOOK_SECRET_ENCRYPTION_KEY`| 32-byte AES-256-GCM hex key for decrypting merchant webhook signing secrets (shared with Core API). | `0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef` | **Yes** | **Yes** |
| `NOTIFICATION_PROVIDER` | Notification provider backend (`mock`, `smtp`, `twilio`, `sendgrid`). | `mock` (local) or `smtp` | **Yes** | No |
| `SMTP_HOST` | SMTP server hostname (e.g. `mailpit` locally). | `mailpit` | No | No |
| `SMTP_PORT` | SMTP server port (e.g. `1025` for Mailpit). | `1025` | No | No |
| `SMTP_USER` | SMTP username. | `swiftdrop` | No | No |
| `SMTP_PASSWORD` | SMTP password. | *(Password)* | No | **Yes** |
| `SMTP_FROM` | Transactional sender email address. | `no-reply@swiftdrop.com` | No | No |
| `SENDGRID_API_KEY` | Optional SendGrid API key for production transactional emails. | `SG.abc...` | If SendGrid | **Yes** |
| `TWILIO_ACCOUNT_SID` | Optional Twilio Account SID for production transactional SMS. | `ACabc...` | If Twilio | **Yes** |
| `TWILIO_AUTH_TOKEN` | Optional Twilio Auth Token. | `xyz...` | If Twilio | **Yes** |

---

## 5. AI Assistant Service (`services/ai-assistant`)

Python/FastAPI LLM query and agent service (HTTP Port `8000`, Probes at `/health/live`, `/health/ready`):

| Variable Name | Description | Default / Example Value | Required? | Secret? |
| :--- | :--- | :--- | :--- | :--- |
| `AI_HTTP_PORT` | HTTP port for FastAPI server. | `8000` | **Yes** | No |
| `CORE_API_URL` | Internal cluster base URL to Core API for tool queries. | `http://core-api:8080` | **Yes** | No |
| `AI_SERVICE_ROLE_TOKEN` | Service-to-service JWT token with read-only scopes. | *(Bearer JWT)* | **Yes** | **Yes** |
| `LLM_PROVIDER` | LLM backend provider (`openai`, `anthropic`, `mock`). | `openai` (or `mock`) | **Yes** | No |
| `LLM_API_KEY` | Secret API key for chosen LLM provider. | `sk-...` | If not mock | **Yes** |
| `LLM_BASE_URL` | Optional custom base URL for OpenAI-compatible endpoint. | `https://api.openai.com/v1` | No | No |
| `LLM_MODEL` | Foundation model name. | `gpt-4o-mini` | No | No |
| `LLM_MAX_TOKENS` | Maximum tokens per response. | `1024` | No | No |
| `AI_RATE_LIMIT_RPM` | Rate limit requests per minute per IP/session. | `20` | No | No |

---

## 6. Web Frontend (`web`)

React Single-Page Application (Vite build variables, prefixed with `VITE_`):

| Variable Name | Description | Default / Example Value | Exposed to Client? |
| :--- | :--- | :--- | :--- |
| `VITE_API_BASE_URL` | Canonical API base URL (one build for all environments; proxied via Traefik/Nginx). | `/api/v1` | **Yes** (Client-side bundle) |
| `VITE_API_GATEWAY_URL` | Alias for `VITE_API_BASE_URL`. | `http://localhost/api/v1` | **Yes** |
| `VITE_TRACKING_SSE_URL` | URL for Server-Sent Events live tracking stream. | `/api/v1/tracking/public` | **Yes** |
| `VITE_GOOGLE_CLIENT_ID`| Public Google OAuth Client ID for sign-in button. | `123456789-abc.apps...` | **Yes** |
| `VITE_APP_TITLE` | Application branding title in browser header. | `SwiftDrop Courier Logistics` | **Yes** |

---

## 7. Kubernetes Secret & ConfigMap Mapping Matrix

Standardized secret naming scheme per Member 9 Agreement A5:

| Configuration Group | Kubernetes Resource Name | Resource Type | Key Examples |
| :--- | :--- | :--- | :--- |
| Core API Config | `core-api-config` | `ConfigMap` | `APP_ENV`, `HTTP_ADDR`, `PUBLIC_BASE_URL` |
| Database App Credentials | `postgres-secrets` | `Secret` | One key per login role, mapped into each pod's `DATABASE_URL`: `DATABASE_URL` (core-api, `swiftdrop_app`), `TRACKING_DATABASE_URL` (`swiftdrop_tracking_ro`), `RELAY_DATABASE_URL` (`swiftdrop_relay`), `WORKER_DATABASE_URL` (`swiftdrop_worker`), `RECONCILER_DATABASE_URL` (`swiftdrop_reconciler`); plus `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` for the Postgres container only |
| Database Migration Credentials | `postgres-migrator-secrets` | `Secret` | `DATABASE_URL` (role `swiftdrop_migrator`) |
| RabbitMQ Credentials | `rabbitmq-secrets` | `Secret` | `RABBITMQ_URL` (user `swiftdrop`) |
| JWT Keys | `jwt-secrets` | `Secret` (mounted as files at `/etc/secrets/jwt/`, not env vars) | `private.pem` (core-api only), `public.pem` (core-api, tracking-api) |
| Third-Party API Keys & Encryption | `integrations-secrets` | `Secret` | `WEBHOOK_SECRET_ENCRYPTION_KEY`, `MERCHANT_API_KEY_PEPPER`, `GOOGLE_CLIENT_SECRET`, `LLM_API_KEY`, `SMTP_PASSWORD` |
| Module-Specific Secrets | `<service>-<module>-secrets` | `Secret` | E.g. `core-api-merchants-secrets` |
| Traefik Configuration | `traefik-config` | `ConfigMap` | `traefik.yml` static configuration |
