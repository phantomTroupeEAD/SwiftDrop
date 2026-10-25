# Document 6: Testing Strategy & Acceptance Specification

**Project:** SwiftDrop — Parcel Delivery & Courier Management System  
**Module:** Tracking API & Real-Time Updates  
**Module Owner:** Member 5  
**Status:** Draft — Pending Team Review

---

## 1. Purpose

This document defines the testing strategy and acceptance criteria for the Tracking API module. It ensures that tracking endpoints, parcel status history, real-time updates, security controls, and failure-handling mechanisms work correctly.

## 2. Testing Objectives

The main objectives are to:

- Verify that tracking endpoints return correct parcel information.
- Ensure public tracking responses do not expose private information.
- Validate authenticated tracking history access.
- Verify Server-Sent Events (SSE) connection and reconnection behavior.
- Test handling of invalid requests, unknown tracking numbers, and service failures.
- Verify integration with PostgreSQL and the agreed event-delivery mechanism.
- Check performance under expected concurrent requests and connections.

## 3. Testing Levels

### 3.1 Unit Testing

Test individual functions and components independently.

Examples:
- Tracking number validation.
- Public response field filtering.
- Parcel status and history mapping.
- Error response formatting.
- Authorization checks.
- SSE event formatting and heartbeat generation.

### 3.2 Integration Testing

Test interactions between the Tracking API and its dependencies.

Examples:
- Reading parcel tracking information from PostgreSQL.
- Retrieving parcel status history.
- Validating authentication middleware.
- Receiving tracking events through the team-approved messaging mechanism.
- Handling database connection failures.
- Verifying SSE updates after a parcel status change.

### 3.3 API Testing

Test the REST endpoints using valid and invalid requests.

Verify:
- HTTP status codes.
- Response structure and field names.
- Request validation.
- Authentication and authorization.
- Error response consistency.
- Public and authenticated endpoint separation.

### 3.4 Real-Time Testing

Test SSE behavior, including:

- Establishing an SSE connection.
- Receiving valid tracking updates.
- Sending heartbeat comments when no updates occur.
- Reconnecting after a temporary disconnection.
- Handling an expired or invalid tracking number.
- Fetching the latest authoritative tracking state after reconnection.

The client must not assume that it received every update while disconnected. It should refresh the current tracking state after reconnecting.

### 3.5 Security and Privacy Testing

Verify that:

- Public endpoints expose only approved tracking fields.
- Full addresses, phone numbers, merchant secrets, and other private information are not exposed publicly.
- Authenticated endpoints reject unauthorized requests.
- Invalid or expired credentials are handled correctly.
- Input validation prevents malformed requests.
- Rate limiting is applied according to the agreed policy.
- Error messages do not reveal database details, credentials, or internal implementation information.

## 4. Test Case Matrix

| ID | Test Scenario | Expected Result |
|---|---|---|
| TC-01 | Request tracking information using a valid tracking number | Returns the correct public tracking information |
| TC-02 | Request tracking information using an unknown tracking number | Returns the agreed not-found response |
| TC-03 | Submit an invalid tracking number format | Returns the agreed validation error |
| TC-04 | Inspect the public tracking response for private fields | No unapproved private information is returned |
| TC-05 | Request authenticated tracking history without credentials | Request is rejected |
| TC-06 | Request tracking history with valid authorization | Returns only the history the user is permitted to access |
| TC-07 | Establish an SSE connection for a valid tracking number | Connection is established and follows the agreed SSE contract |
| TC-08 | Change a parcel's status while the client is connected | The client receives the corresponding approved update |
| TC-09 | Keep an SSE connection idle | Heartbeat comments are sent according to the agreed interval |
| TC-10 | Disconnect and reconnect an SSE client | The client can reconnect and refresh the latest tracking state |
| TC-11 | Make repeated requests beyond the configured rate limit | Requests are limited according to the agreed policy |
| TC-12 | Make a database query while PostgreSQL is unavailable | The service returns a controlled error and does not expose internal details |
| TC-13 | Send malformed input to a tracking endpoint | The request is safely rejected |
| TC-14 | Deliver an event with an invalid or unsupported payload | The event is safely rejected or handled according to the agreed contract |
| TC-15 | Run multiple Tracking API replicas, if configured | Tracking updates are delivered according to the approved multi-replica design |
| TC-16 | Run concurrent tracking requests | The service remains correct and stable within the agreed performance limits |

**Note:** Exact endpoint paths, status codes, rate limits, heartbeat intervals, and performance thresholds must match the approved team contracts.

## 5. Performance Testing

Performance testing should cover:

- Concurrent public tracking requests.
- Concurrent authenticated tracking-history requests.
- Concurrent SSE connections.
- Tracking updates delivered while many clients are connected.
- Database connection pool usage under load.
- CPU and memory usage during sustained activity.

Record the following measurements:

- Request throughput.
- Response latency, including p95 and p99 where possible.
- Error rate.
- Active SSE connection count.
- Event delivery delay.
- Database connection utilization.
- CPU and memory utilization.

Numerical acceptance thresholds must be agreed with the team and deployment owner before performance testing is considered complete.

## 6. Test Environment and Tools

The testing environment should use:

- A dedicated test database or isolated test schema.
- Mocked dependencies for unit tests.
- Test instances of PostgreSQL and the approved messaging infrastructure for integration tests.
- Automated API tests.
- An SSE-capable test client.
- Load-testing tools selected by the team.
- Sanitized test data that does not contain real customer information.

Test configuration and credentials must not be committed to the repository.

## 7. Test Data Requirements

Test data should cover:

- A parcel with no status history.
- A parcel with multiple status updates.
- A parcel with an unknown or invalid tracking number.
- Parcels with different delivery statuses.
- Users with valid and invalid authorization.
- Tracking records containing private fields to verify public response filtering.
- Duplicate, delayed, malformed, or unsupported events where relevant.

Use synthetic data and avoid real customer personal information.

## 8. Acceptance Criteria

The Tracking API module is ready for acceptance when:

1. Approved tracking endpoints return the correct response structures.
2. Public tracking responses contain only approved public fields.
3. Authenticated tracking history enforces authorization correctly.
4. Invalid requests and unknown tracking numbers are handled consistently.
5. SSE connections deliver valid updates according to the agreed event contract.
6. Reconnection allows clients to recover the latest tracking state.
7. Database and messaging failures are handled safely.
8. Agreed rate-limiting and security controls are tested.
9. Automated unit, integration, and API tests pass.
10. Multi-replica event delivery is verified if the deployment uses multiple replicas.
11. Performance results and known limitations are documented.
12. The README explains how to run the service and execute its tests.

## 9. Test Execution and Reporting

For each test run, record:

- Test date.
- Environment and configuration.
- Test cases executed.
- Passed and failed test cases.
- Defects discovered.
- Performance measurements, where applicable.
- Outstanding issues and their impact.
- Retest results after fixes.

Critical privacy, authorization, and data-correctness failures must be resolved before acceptance.

## 10. Dependencies and Pending Decisions

The following items require confirmation before final acceptance:

- Final endpoint paths and response schemas.
- Approved PostgreSQL schema and status-history structure.
- Approved event schema and messaging topology.
- Authentication and authorization contract.
- Rate limits and SSE heartbeat interval.
- Performance targets and expected concurrent load.
- Multi-replica deployment and event fan-out strategy.
- Team-wide testing tools and CI requirements.

## 11. Deliverables

The module owner should provide:

- Automated unit tests.
- Integration and API tests.
- SSE behavior tests.
- Security and privacy test cases.
- Performance test notes and results.
- A record of known limitations.
- Updated instructions for running tests.

---

**End of Document 6**
