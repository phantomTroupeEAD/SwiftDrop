# SwiftDrop — Claude Code Instructions

> **Primary Directive:** Always read and strictly adhere to `AGENTS.md` at the repository root before making changes.

## Quick Reference & Commands
- **Authority:** `AGENTS.md` defines universal architecture, DDD boundaries, and coding standards.
- **Domain Invariants:** Consult `docs/architecture/domain-boundaries.md` before modifying business logic or parcel state transitions.
- **Service Topology:**
  - Core API (Go Modular Monolith): `core-api/`
  - Tracking API: `services/tracking/`
  - Outbox Relay & Event Worker: `services/messaging/`
  - AI Assistant (Python/FastAPI): `services/ai-assistant/`
  - Web Frontends (React/TypeScript): `web/`

## Common Commands
```bash
# Run local quality verification (formatting, linting, tests, secrets)
./scripts/verify.sh

# Run Go Core API tests
cd core-api && go test -v -race ./...

# Run Go linter
cd core-api && golangci-lint run ./...

# Format Go code
cd core-api && gofmt -w .

# Run Frontend Dev Server
cd web && npm run dev

# Run Frontend Linter & Type Check
cd web && npm run lint

# Run Frontend Unit & Component Tests (Vitest)
cd web && npm run test:unit

# Run Frontend End-to-End Tests (Playwright)
cd web && npm run test:e2e
```

## Frontend & Testing Invariants
- Consult `.cursor/rules/frontend.mdc` or `web/CLAUDE.md` before making any frontend changes.
- Never calculate pricing or fee totals client-side; always fetch authoritatively via `POST /api/v1/pricing/quote`.
- Never evaluate parcel state transitions in the browser; the backend `Parcel` aggregate root is the single source of truth.
- Dual-token auth: access tokens remain strictly in-memory (`AuthContext`); refresh tokens remain in `HttpOnly` cookies. Never store tokens in `localStorage`.
- Public tracking (`/track/:trackingNumber`) must strictly redact recipient phone, full street address, and proof attachments.
- Unit testing: Vitest + React Testing Library for components, custom hooks, and formatters (`npm run test:unit`).
- E2E testing: Playwright (`@playwright/test`) for critical user journeys (booking, tracking, dispatch, courier POD, failed attempts) using Page Object Models and `data-testid` selectors (`npm run test:e2e`).

## Kubernetes Scalability Rules
- **Stateless Scalable Services:** Core API, Tracking API, and Web Frontend scale via HPA. Core API per-pod DB pool must obey: $\text{Replicas}_{\text{max}} \times \text{MaxOpenConns} \le \text{PostgreSQL MaxConnections}$. Tracking API uses Redis Pub/Sub for SSE fan-out.
- **Event Worker Scaling:** Scales via KEDA on RabbitMQ queue backlog (`queueLength > 10`). All consumers MUST be strictly idempotent.
- **Outbox Relay Prohibition:** The Outbox Relay is strictly a singleton (`replicas: 1`). Never scale horizontally without leader election.
- **Stateful Infrastructure:** PostgreSQL and RabbitMQ run as StatefulSets with PVCs; never scale write primaries horizontally via Deployments.

## Mandatory Workflow
1. Read `AGENTS.md` and the relevant domain documentation.
2. Formulate and state an implementation plan.
3. Make the smallest complete change without modifying unrelated modules.
4. Run tests and verify invariants.
5. Synchronously update the module's living `README.md` and `API.md` (if endpoints exist).
6. Provide the structured completion report required by `AGENTS.md`.


