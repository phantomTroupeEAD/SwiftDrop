# Document 8: Implementation, Deployment & Operations Guide

**Project:** SwiftDrop — Parcel Delivery & Courier Management System  
**Module:** Tracking API & Real-Time Updates  
**Module Owner:** Member 5  
**Status:** Draft — Pending Team Review

---

## 1. Purpose

This document explains how to implement, configure, run, test, deploy, and operate the Tracking API module.

It provides implementation guidance for developers and operational guidance for integrating the module with the other SwiftDrop services.

## 2. Module Responsibilities

The Tracking API module is responsible for:

- Providing public parcel tracking information.
- Providing authorized access to parcel tracking history.
- Delivering real-time tracking updates using Server-Sent Events (SSE), if enabled in the approved design.
- Reading authoritative tracking data from PostgreSQL or another explicitly approved consistent read interface.
- Validating requests and protecting private parcel information.
- Handling temporary database, messaging, and connection failures.
- Exposing health checks and operational metrics according to team standards.

The module must follow the team's agreed API contracts, database schema, event schema, authentication mechanism, and deployment conventions.

## 3. Implementation Structure

The final folder structure must follow the team's approved Go project conventions. The following is a proposed structure, not a confirmed repository layout.

```text
tracking-api/
├── cmd/
│   └── tracking-api/
│       └── main.go
├── internal/
│   ├── config/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   ├── middleware/
│   ├── events/
│   └── health/
├── tests/
├── Dockerfile
├── go.mod
├── go.sum
└── README.md
```

### 3.1 Component Responsibilities

| Component | Responsibility |
|---|---|
| `cmd/tracking-api` | Application entry point and initialization |
| `internal/config` | Loads and validates configuration |
| `internal/handler` | Handles HTTP requests and responses |
| `internal/service` | Implements tracking business logic |
| `internal/repository` | Reads tracking data from PostgreSQL |
| `internal/middleware` | Handles authentication, request validation, and applicable security controls |
| `internal/events` | Integrates with the approved event-delivery mechanism |
| `internal/health` | Implements health checks and metrics integration |

The exact package names and boundaries may be adjusted to match the existing repository.

## 4. Configuration Management

The service should load configuration from environment variables or the team's approved configuration mechanism.

Suggested configuration categories:

| Category | Examples |
|---|---|
| HTTP server | Listening address, port, read timeout, write timeout |
| Database | Connection URL or individual connection settings, pool limits |
| Authentication | Required validation settings and secret references |
| Messaging | Broker address, credentials reference, consumer configuration |
| SSE | Heartbeat interval, connection limits, reconnect-related settings |
| Logging | Log level and output format |
| Monitoring | Health-check and metrics configuration |

**Important:** The actual variable names, ports, timeouts, and limits must be confirmed against the repository and team deployment configuration before implementation.

Secrets must be supplied through approved secret management mechanisms. They must not be hardcoded or committed to Git.

## 5. Local Development Setup

Before starting the service, ensure that:

- The approved Go version is installed.
- The repository has been cloned.
- Required Go dependencies are available.
- PostgreSQL is running with the approved schema.
- Required authentication configuration is available.
- The approved messaging infrastructure is running if event consumption is enabled.
- Local environment variables have been configured.

### 5.1 Install Dependencies

From the module directory, run:

```bash
go mod download
```

### 5.2 Run Tests

```bash
go test ./...
```

### 5.3 Run the Application

The exact command depends on the final Go entry-point location.

For the proposed structure above, the command would be:

```bash
go run ./cmd/tracking-api
```

Use this command only if the repository adopts that structure. Otherwise, follow the actual module entry point.

## 6. API and Integration Configuration

The module should integrate with the API Gateway using the routes agreed by the team.

Proposed public tracking route:

```http
GET /api/v1/tracking/public/{trackingNumber}
```

Proposed SSE route:

```http
GET /api/v1/tracking/public/{trackingNumber}/stream
```

These paths remain proposals until the API Gateway owner and module owner confirm them.

Integration requirements:

- The API Gateway must route tracking requests to the Tracking API.
- Gateway settings must support long-lived SSE connections when SSE is enabled.
- Response buffering and connection timeouts must be configured appropriately for SSE.
- Authentication and authorization requirements must match the approved contract.
- Public tracking responses must contain only approved fields.
- The frontend must refresh the current tracking state after reconnecting to an SSE stream.

The authenticated history route must be taken from the approved API contract rather than invented independently.

## 7. Database and Event Integration

### 7.1 PostgreSQL

The Tracking API must use the team-approved database schema and connection settings.

Requirements:

- Use safe, parameterized queries or approved ORM methods.
- Use bounded database connection pools.
- Avoid modifying parcel data through tracking-only read operations.
- Handle database connection failures without returning fabricated tracking data.
- Coordinate pool limits with the deployment owner and database coordinator.
- Keep schema changes under the team's agreed migration process.

The database username, table names, column names, and pool size must be confirmed by the relevant team members before being treated as final.

### 7.2 Event Delivery

If real-time updates consume events from RabbitMQ or another approved messaging system:

- Follow the event contract supplied by the responsible team member.
- Validate incoming event payloads.
- Handle duplicate or delayed events safely.
- Follow the approved acknowledgement, retry, and failure-handling strategy.
- Do not assume that a shared queue broadcasts every message to every Tracking API replica.
- Coordinate multi-replica event delivery with the messaging and deployment owners.

Redis or another fan-out mechanism should only be introduced if the approved architecture requires it.

## 8. Docker Configuration

The module should provide a Dockerfile consistent with the team's shared container standards.

The container should:

- Build the Go application reproducibly.
- Include only runtime dependencies required by the final image.
- Run as a non-root user where feasible.
- Read configuration from the approved environment or secret mechanism.
- Avoid including credentials in the image.
- Expose the configured application port.
- Shut down gracefully when receiving termination signals.

The actual Dockerfile and start command must match the repository's build output and agreed port configuration.

## 9. Deployment

Deployment must follow the team's approved Docker Compose and Kubernetes configuration.

### 9.1 Docker Compose

The module should be integrated into the shared Compose configuration maintained by the deployment owner.

Confirm:

- Service name and container port.
- Environment variables and secret references.
- Database and messaging dependencies.
- Health-check configuration.
- Network and gateway routing.
- Startup ordering and readiness behavior.

Do not independently create conflicting Compose files or duplicate shared infrastructure definitions without team agreement.

### 9.2 Kubernetes

If deployed on Kubernetes, the module should use the approved Deployment, Service, configuration, and secret resources.

Confirm:

- Container image and application port.
- Readiness and liveness probes.
- Resource requests and limits.
- Replica count and autoscaling policy.
- Database connection pool limits.
- Graceful shutdown behavior.
- Gateway routing and SSE timeout configuration.

Replica counts, CPU thresholds, memory limits, and autoscaling values must be decided by the deployment owner using the team's workload and infrastructure constraints.

## 10. Health Checks and Metrics

Suggested endpoints, subject to the shared team standard:

- `/health/live` — reports whether the application process is alive.
- `/health/ready` — reports whether the instance is ready to handle requests.
- `/metrics` — exposes monitoring metrics if required by the deployment configuration.

Monitoring should cover:

- Request volume and latency.
- HTTP error rates.
- Active SSE connections.
- SSE disconnections and reconnects.
- Event-processing failures.
- Database connection utilization.
- Resource usage.
- Rate-limit rejections.

Health and metrics endpoints must not expose secrets or private tracking information.

## 11. Operational Procedures

### 11.1 Service Startup

1. Load and validate configuration.
2. Initialize logging and monitoring.
3. Establish required dependency connections.
4. Initialize the HTTP server and approved integrations.
5. Report readiness only when the service is ready to handle requests.

### 11.2 Service Shutdown

1. Stop accepting new requests as appropriate.
2. Allow in-flight requests to finish within the configured shutdown period.
3. Close active SSE connections gracefully where possible.
4. Stop event consumers according to the approved acknowledgement strategy.
5. Close database and messaging connections.
6. Exit without exposing secrets or sensitive information in logs.

### 11.3 Troubleshooting

| Problem | Checks |
|---|---|
| Tracking endpoint returns an error | Check application logs, request validation, and database connectivity |
| Tracking data appears outdated | Verify the authoritative database state and the approved read path |
| SSE updates are not received | Check stream routing, gateway timeouts, event consumption, and client reconnection |
| Database connections are exhausted | Review connection pool limits and concurrent service replicas |
| Events are missing | Check the approved messaging topology, consumer status, retries, and event schema |
| Readiness check fails | Check configuration and required dependency initialization |
| High latency | Review database queries, connection utilization, request volume, and active SSE connections |

Troubleshooting must follow the team's operational access and security policies.

## 12. Documentation and Handover

The module README should explain:

- Module purpose and responsibilities.
- Required software and dependencies.
- Local setup instructions.
- Environment configuration.
- Commands for running and testing the service.
- API routes and response contracts.
- SSE connection and reconnection behavior.
- Docker build and run instructions.
- Deployment dependencies.
- Health-check and monitoring endpoints.
- Known limitations and unresolved decisions.

Sensitive credentials and production secrets must never be included in documentation.

## 13. Pending Team Confirmations

Before the implementation and deployment guide is finalized, confirm:

- Final Go project structure and application start command.
- Application port.
- Approved API routes.
- Database schema, user, and pool configuration.
- Event contract and messaging topology.
- Whether cross-replica fan-out is required.
- Health-check and metrics conventions.
- Docker Compose and Kubernetes configuration.
- Replica count, resource limits, and autoscaling policy.
- Gateway configuration for SSE.
- CI/CD and deployment responsibilities.

## 14. Completion Criteria

The module is ready for handover when:

1. The application starts with the approved configuration.
2. Tracking endpoints follow the approved API contracts.
3. Unit, integration, and API tests pass.
4. Security and privacy requirements are satisfied.
5. SSE behavior works according to the approved design, if enabled.
6. Docker build and local execution instructions are verified.
7. Deployment configuration has been reviewed with the deployment owner.
8. Health checks and monitoring are integrated as agreed.
9. Operational troubleshooting guidance is documented.
10. All unresolved decisions and known limitations are recorded.

---

**End of Document 8**