# SwiftDrop GitHub Copilot Instructions

All GitHub Copilot completions and chat interactions in this repository must comply with the rules defined in `AGENTS.md`.

## Key Architectural Principles
- **Core API Architecture:** Go modular monolith with Clean/Hexagonal principles in `core-api/internal/`.
- **Domain Purity:** The `domain/` layer contains pure business logic, entities, value objects, and domain errors. It MUST NOT import `net/http`, `database/sql`, RabbitMQ, or any infrastructure package.
- **Aggregate Root Protection:** State transitions on parcels must be executed by the `Parcel` aggregate root. Do not generate unvalidated SQL statements directly modifying parcel status or assignment columns.
- **Transactional Outbox:** Always ensure aggregate mutations, status audit history, and outbox event records commit atomically in a single PostgreSQL transaction.
- **Tenant & Ownership Isolation:** Every database query involving merchant data or parcel access must strictly enforce tenant/user isolation. Never assume frontend checks are sufficient.
- **Living Documentation Requirement:** Any code change modifying domain rules, tables, or API contracts must concurrently update the module's co-located `README.md` and `API.md`.
- **Testing Integrity:** When fixing or refactoring code, never delete, mock out, or weaken existing test assertions. Always add test coverage for new edge cases and failure paths.
- **Kubernetes Scalability Rules:** Stateless services (Core API, Tracking API, Web) must support horizontal scaling with HPA, bounded DB connection pools, and graceful `SIGTERM` shutdown; Event Worker scales with KEDA and requires strict consumer idempotency; Outbox Relay is strictly a singleton (`replicas: 1`); PostgreSQL and RabbitMQ run as StatefulSets with PVCs.

