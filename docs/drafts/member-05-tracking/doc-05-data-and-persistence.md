# Document 5 — Data & Persistence Specification

**Project:** SwiftDrop — Parcel Delivery & Courier Management System  
**Module:** Tracking API & Real-Time Updates  
**Module Owner:** Member 5  
**Version:** 0.1 — Initial Draft  
**Status:** Draft for team review  
**Primary Technology:** Go, PostgreSQL  
**Related Documents:** Documents 1–4, SwiftDrop SRS, Engineering Standards & Best Practices, Team Work Division

---

## 1. Purpose

This document defines how the Tracking API accesses and uses persistent parcel-tracking data.

The Tracking API is a read-oriented service. It retrieves authoritative parcel information and status history but does not own parcel creation, parcel status changes, or the shared parcel database schema.

The purpose of this specification is to establish:

- Database ownership and access boundaries.
- Data required for public tracking and authenticated history.
- Read-only permissions and query rules.
- Data consistency and transaction considerations.
- Indexing and query performance requirements.
- Privacy, retention, and caching rules.
- Migration and test-data responsibilities.

**Important:** Actual table names, column names, constraints, and relationships must follow Member 2's approved database schema. This document does not introduce a replacement schema.

## 2. Data Ownership

### 2.1 Authoritative data source

PostgreSQL is the authoritative source of parcel and status-history data.

The Tracking API must use the approved shared schema or an explicitly approved consistent read interface.

It must not treat RabbitMQ messages, Redis Pub/Sub messages, in-memory state, or SSE events as the authoritative record of parcel status.

### 2.2 Data ownership matrix

| Data or resource | Ownership | Tracking API responsibility |
|---|---|---|
| Parcel records | Relevant core domain/database owner | Read approved fields only. |
| Current parcel status | Relevant core parcel/status owner | Retrieve the latest committed status. |
| Parcel status history | Approved domain/schema owner | Read and return permitted history. |
| Customer identity and authentication data | Authentication/customer domain owner | Use the approved identity and authorization mechanism. |
| Tracking API response DTOs | Member 5 | Map database results to approved public/private responses. |
| Tracking-specific database objects, if any | Requires team approval | Create only if a genuine requirement is identified and ownership is agreed. |
| Event-delivery infrastructure | Shared event/infrastructure owners | Consume agreed notifications without treating them as authoritative data. |

### 2.3 Database ownership rule

The Tracking API must not:

- Modify parcel records or parcel statuses.
- Insert, update, or delete parcel status-history records.
- Duplicate shared parcel tables.
- Introduce conflicting status definitions.
- Change another module's schema without review and approval.

The API should use a dedicated read-only database role with access limited to the data required by its queries.

## 3. Data Required by the Tracking API

The following is a **logical data requirement**, not a proposed physical database schema.

| Logical information | Purpose | Required source |
|---|---|---|
| Tracking identifier | Locate a parcel for public tracking | Approved parcel schema |
| Parcel reference | Associate current status and history with the correct parcel | Approved parcel schema |
| Current status | Display the latest committed parcel state | Authoritative status data |
| Status timestamp | Display when the status was recorded or became effective, according to the agreed definition | Approved status data |
| Status history | Display the sequence of tracking events | Approved history data |
| Customer/owner reference | Authorize access to private history | Approved identity and parcel ownership relationship |
| Event ordering information, if available | Support reliable reconciliation and duplicate/out-of-order handling | Approved event/data contract |

The final field names, types, nullability, and relationships must be taken from Member 2's schema documentation.

## 4. Database Schema and ERD

### 4.1 Schema source

Member 2 owns the approved shared database schema or coordinates its approval with the database coordinator.

Member 5 must use that schema when implementing repository queries.

### 4.2 ERD requirement

An ERD showing the relevant parcel, status-history, and ownership relationships should be included here once the approved schema is available.

**Pending:** The actual table names, primary keys, foreign keys, cardinalities, and column definitions must be confirmed by Member 2. No physical ERD is finalized in this draft because those details are not yet confirmed.

### 4.3 Data dictionary

The final data dictionary must document:

| Property | Description |
|---|---|
| Table and column | Exact approved database identifier |
| Data type | Actual PostgreSQL type |
| Nullability | Whether the field may be null |
| Key/constraint | Primary key, foreign key, unique constraint, or other constraint |
| Purpose | Meaning of the field |
| Tracking usage | Whether and how the Tracking API reads it |
| Exposure classification | Internal, private, or approved public information |

The data dictionary should cover only the database objects relevant to the Tracking API rather than duplicating the entire system-wide ERD.

## 5. Read-Only Database Access

### 5.1 Database role

The Tracking API should connect through a dedicated read-only database account.

The proposed username `tracking_reader` may be used if approved by Member 2 and Member 9. It is not yet a confirmed team-wide name.

### 5.2 Permissions

The account should have only the permissions required to execute approved tracking queries.

It must not have permission to:

- Insert or update parcel records.
- Change parcel statuses.
- Insert or delete status-history records.
- Alter shared database tables.
- Perform schema migrations.

Access to customer-related fields must be restricted to the minimum necessary for authorized history lookups.

Credentials must be provided through the team's approved secret-management mechanism. They must never be committed to source control.

### 5.3 Query requirements

Repository queries must:

1. Use parameterized SQL.
2. Follow the approved schema.
3. Retrieve only necessary fields.
4. Apply authorization constraints to private history queries.
5. Handle missing records safely.
6. Avoid unbounded history retrieval.
7. Return sanitized application errors rather than raw database errors.
8. Respect configured query timeouts and connection-pool limits.

## 6. Data Consistency and Transaction Boundaries

### 6.1 Authoritative status

The Tracking API must return the latest committed authoritative status available to its query.

A status notification received from the event system indicates that data may have changed. It does not independently establish the authoritative current status.

### 6.2 Status history

Status history must be read from the approved authoritative history source and returned in the agreed chronological order.

If multiple history records have identical timestamps, a deterministic tie-breaking rule should be agreed using the actual schema, such as an approved sequence or unique identifier.

The Tracking API must not invent missing history records or change their timestamps.

### 6.3 Transactions

The Tracking API is primarily read-only.

The core domain module that changes parcel status is responsible for committing the status change and its history record according to the system's transaction rules.

The Tracking API should use appropriate database query/transaction behavior to avoid presenting inconsistent combinations of current status and history. The exact isolation strategy must be selected after reviewing the approved schema and query requirements.

### 6.4 Eventual notification

A live-update notification may arrive after the database transaction commits.

When processing a notification, the Tracking API should retrieve or reconcile against the authoritative data before presenting a status as current, where required by the agreed event design.

## 7. Indexing and Query Performance

The Tracking API must coordinate indexing requirements with Member 2 rather than independently changing the shared schema.

Potential indexing considerations include:

- An index supporting lookup by tracking identifier.
- An index supporting retrieval of status history for a parcel.
- An index or ordering strategy supporting chronological history queries.
- Appropriate constraints ensuring tracking identifiers are unique if required by the domain model.

These are recommendations for schema review, not claims that the corresponding indexes already exist.

Indexes should be selected based on actual query patterns, data volume, and query-plan testing.

### 7.1 Query performance requirements

- Public lookup should retrieve a bounded amount of data.
- History retrieval should support pagination if large histories are possible and the API contract allows it.
- Queries must avoid unnecessary columns and full-table scans where practical.
- Performance must be validated using representative test data.
- Connection pools and query timeouts must be configured to protect the shared PostgreSQL service.

Exact latency targets and connection-pool values remain subject to team agreement and load testing.

## 8. Caching Policy

### 8.1 Initial policy

The recommended initial implementation is to read authoritative status from PostgreSQL without introducing a separate tracking-status cache.

This avoids introducing cache invalidation and stale-status risks before the system's tracking workload is measured.

### 8.2 Future caching

If caching is introduced, the design must specify:

- Which fields may be cached.
- Maximum permitted staleness.
- Cache expiration and invalidation behavior.
- How status changes invalidate or update cached values.
- How cache failures are handled.
- Whether private data is excluded from shared caches.

Redis Pub/Sub, if selected, is a notification/fan-out mechanism. It must not be treated as a durable cache or authoritative status store.

## 9. Privacy, Retention, and Deletion

### 9.1 Privacy

The Tracking API must return only the fields approved for the requesting interface.

Public tracking must not expose full customer addresses, phone numbers, merchant secrets, credentials, or other unapproved private data.

Authenticated history must enforce the agreed ownership or access-control rules.

### 9.2 Retention and deletion

The Tracking API does not independently define retention or deletion policies for shared parcel and status-history records.

Those policies belong to the responsible domain and system-level data governance owners.

The Tracking API must respect approved deletion, retention, and access policies and must not create additional persistent copies without approval.

### 9.3 Logging

Logs must not contain database credentials, access tokens, or unnecessary personal information.

Tracking identifiers and request metadata should be logged only according to the team's privacy and observability standards.

## 10. Migration Strategy

### 10.1 Shared schema migrations

The Tracking API must not independently manage migrations for tables owned by another domain module.

Changes to shared parcel/status tables must follow the team's approved migration workflow and review process.

### 10.2 Tracking-owned migrations

If a genuine tracking-specific table is later approved, its migration must:

1. Have an agreed owner.
2. Follow the shared migration tool and naming conventions.
3. Be reviewed for compatibility with existing modules.
4. Include appropriate constraints and indexes.
5. Be tested before deployment.
6. Have a documented rollback or recovery approach consistent with the team's migration policy.

No tracking-specific table is required by this draft.

### 10.3 Migration ordering

The Tracking API must not start using a schema change until the required migration has completed successfully and the deployment process confirms compatibility.

The exact migration tool and execution workflow follow the team's shared database migration decision.

## 11. Seed and Test Data Requirements

Tests should use synthetic data representing:

- A parcel with multiple status-history records.
- A parcel with only an initial status.
- An unknown tracking identifier.
- An invalid tracking identifier.
- Multiple parcels associated with different customers, where the approved schema supports this.
- A parcel with records sharing a timestamp, if supported by the data model.
- Status data suitable for testing event duplication and out-of-order notifications.

Test data must not contain real customer personal information or production credentials.

Seed data must follow the approved schema and must not modify production data.

## 12. Testing Requirements

The persistence implementation must be tested for:

- Correct lookup by tracking identifier.
- Retrieval of the latest committed status.
- Correct chronological history ordering.
- Correct handling of missing records.
- Read-only database permissions.
- Parameterized query behavior.
- Authorization constraints for private history.
- Database timeouts and connection failures.
- Handling of empty or unusually large histories.
- Query performance against representative data.
- Compatibility with the approved schema and migrations.

The detailed test matrix and execution instructions will be maintained in Document 6.

## 13. Unresolved Decisions

| Item | Status | Required action |
|---|---|---|
| Parcel and status-history table names | Pending | Obtain approved schema from Member 2. |
| Column names, types, and relationships | Pending | Confirm against the shared ERD/data dictionary. |
| Tracking-number uniqueness | Pending | Confirm with Member 2. |
| Status timestamp and ordering rule | Pending | Confirm the authoritative semantics and tie-breaker. |
| Read-only database username | Proposed | Coordinate with Members 2 and 9. |
| Connection-pool limits | Pending | Agree with Member 9 based on the shared PostgreSQL budget. |
| Indexes and query plans | Pending | Review the approved schema and expected workload. |
| Tracking-owned tables | None proposed | Require explicit approval before adding any. |
| Caching | No separate cache initially | Revisit only if justified by measured workload. |
| Retention and deletion policies | Shared policy | Follow the responsible domain owners' rules. |
| Migration workflow | Shared decision | Follow the team's approved migration conventions. |

## 14. Definition of Done

This document is ready for approval when:

- Data ownership and read-only access boundaries are agreed.
- The relevant schema and ERD have been referenced.
- Required fields and query relationships are confirmed.
- Indexing and query-performance considerations are reviewed.
- Transaction and consistency expectations are documented.
- Caching, privacy, retention, and migration responsibilities are clear.
- Test-data requirements are defined.
- All pending decisions have owners and follow-up actions.

---

**End of Document 5 — Initial Draft**