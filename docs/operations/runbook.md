# SwiftDrop — Operational Runbook & Incident Response Guide

> **Authority:** Fulfills SRS §11.4 (Observability & Runbook), §14 (Project & Technical Risks), and Engineering Standards §11 & §14.  
> **Audience:** DevOps Engineers (Member 9), System Administrators, and On-Call Developers.

---

## 1. Service Health Check Reference

All deployed services provide standard Kubernetes Liveness and Readiness probes:

| Service | Port | Endpoint | Purpose | Expected Status / Body |
| :--- | :--- | :--- | :--- | :--- |
| **Core API** | `8080` | `GET /health/live`<br>`GET /health/ready`<br>`GET /metrics` | Liveness (process up)<br>Readiness (PostgreSQL pool connected)<br>Prometheus metrics | `200 OK` `{"status":"UP"}`<br>`200 OK` `{"status":"READY","database":"UP"}`<br>`200 OK` (metrics) |
| **Tracking API** | `8081` | `GET /health/live`<br>`GET /health/ready`<br>`GET /metrics` | Liveness (process up)<br>Readiness (DB connection)<br>Prometheus metrics | `200 OK` `{"status":"UP"}`<br>`200 OK` `{"status":"READY"}`<br>`200 OK` (metrics) |
| **AI Assistant** | `8000` | `GET /health/live`<br>`GET /health/ready` | Liveness (process up)<br>Readiness (local model cache up) | `200 OK` `{"status":"healthy"}`<br>`200 OK` `{"status":"ready"}` |
| **Outbox Relay** | `8082` | `GET /health/live`<br>`GET /health/ready`<br>`GET /metrics` | Liveness (daemon up)<br>Readiness (DB pool connected)<br>Prometheus metrics | `200 OK` `{"status":"UP"}`<br>`200 OK` `{"status":"READY"}`<br>`200 OK` (metrics) |
| **Event Worker** | `8083` | `GET /health/live`<br>`GET /health/ready`<br>`GET /metrics` | Liveness (worker up)<br>Readiness (RabbitMQ + DB connected)<br>Prometheus metrics | `200 OK` `{"status":"UP"}`<br>`200 OK` `{"status":"READY"}`<br>`200 OK` (metrics) |
| **Traefik Ingress**| `9000` (ping)<br>`8090` (dashboard) | `GET /ping` (9000)<br>`GET /dashboard/` (8090) | Internal proxy liveness<br>Local admin dashboard | `200 OK` `OK`<br>`200 OK` (dashboard UI) |

---

## 2. Structured Logging & Distributed Tracing

### 2.1 Log Format Specification
All backend services output structured **JSON** lines to `stdout`:
```json
{
  "timestamp": "2026-10-21T14:30:00.123Z",
  "level": "INFO",
  "service": "core-api",
  "request_id": "req_01HXYZ1234567890",
  "actor_id": "usr_01HABC9876543210",
  "method": "POST",
  "path": "/api/v1/parcels/book",
  "status": 201,
  "duration_ms": 42.5,
  "msg": "Parcel successfully booked and outbox event emitted"
}
```

### 2.2 Trace Correlation Workflow
1. Client or Edge Ingress injects header `X-Request-ID: <uuid>`.
2. Core API logs include `"request_id": "<uuid>"`.
3. Outbox event payload records `"correlation_id": "<uuid>"`.
4. Downstream Event Workers and Webhook dispatchers include this ID in their execution logs.
5. **Investigation Query:**
   ```bash
   # Search logs across cluster by Request ID
   kubectl logs -n swiftdrop -l app=core-api | grep "req_01HXYZ1234567890"
   kubectl logs -n swiftdrop -l app=event-worker | grep "req_01HXYZ1234567890"
   ```

---

## 3. PostgreSQL Operational Procedures

### 3.1 Automated & Manual Backup
```bash
# Execute compressed PostgreSQL dump from inside VPS
docker exec -t swiftdrop-postgres pg_dump -U swiftdrop -d swiftdrop_db -F c -b -v -f /backups/swiftdrop_$(date +%Y%m%d_%H%M%S).dump

# Or via Kubernetes pod:
kubectl exec -n swiftdrop postgres-0 -- pg_dump -U swiftdrop swiftdrop_db -F c > backup_$(date +%Y%m%d).dump
```

### 3.2 Database Restore Procedure
> [!CAUTION]
> Restoring a database will overwrite current state. Always stop write workloads (`core-api`, `outbox-relay`) first!

```bash
# 1. Scale down writers
kubectl scale deployment/core-api deployment/outbox-relay -n swiftdrop --replicas=0

# 2. Restore database from dump
pg_restore -U swiftdrop -d swiftdrop_db -c -v /backups/swiftdrop_target.dump

# 3. Scale writers back up
kubectl scale deployment/core-api -n swiftdrop --replicas=2
kubectl scale deployment/outbox-relay -n swiftdrop --replicas=1
```

### 3.3 Database Migration Rollback
```bash
# Run golang-migrate rollback step
migrate -path core-api/migrations -database "$DATABASE_URL" down 1
```

---

## 4. RabbitMQ & Outbox Queue Recovery

### 4.1 Stuck Outbox Investigation & Replay
If events are generated in PostgreSQL but tracking/notifications are delayed:
```sql
-- Check count and oldest pending outbox events
SELECT count(*), min(created_at), max(attempt_count)
FROM outbox_events
WHERE status = 'PENDING';

-- Inspect failed or stuck events older than 5 minutes
SELECT id, aggregate_id, event_type, attempt_count, last_error, created_at
FROM outbox_events
WHERE status = 'PENDING' AND created_at < NOW() - INTERVAL '5 minutes'
ORDER BY created_at ASC LIMIT 10;

-- Force retry of stuck pending rows after fixing broker outage:
UPDATE outbox_events
SET attempt_count = 0, last_error = NULL
WHERE status = 'PENDING' AND attempt_count > 5;
```

### 4.2 RabbitMQ Dead Letter Queue (DLQ) Drain Procedure
When unprocessable messages land in `events.dlq`:
1. **Access RabbitMQ CLI:**
   ```bash
   kubectl exec -it -n swiftdrop statefulset/rabbitmq -- rabbitmqctl list_queues name messages messages_ready
   ```
2. **Inspect Poison Pill Message:** Read first message without acknowledging to identify malformed schema or unhandled exception.
3. **Replay DLQ:** Run the reconciliation script:
   ```bash
   kubectl create job --from=cronjob/swiftdrop-outbox-reconciler dlq-replay-manual -n swiftdrop
   ```

---

## 5. Webhook Failure Triage & Manual Redelivery

When a merchant reports missing webhook notifications:
```sql
-- 1. Identify failing endpoint and delivery status
SELECT wd.id, we.url, wd.status, wd.attempt_count, wd.response_status_code, wd.last_error
FROM webhook_deliveries wd
JOIN webhook_endpoints we ON wd.endpoint_id = we.id
WHERE we.merchant_id = 'YOUR_MERCHANT_UUID'
ORDER BY wd.created_at DESC LIMIT 10;

-- 2. Force manual redelivery for an event
UPDATE webhook_deliveries
SET status = 'PENDING', next_attempt_at = NOW(), attempt_count = 0
WHERE id = 'TARGET_DELIVERY_UUID';
```

---

## 6. Traefik Ingress Gateway Inspection

```bash
# Access internal Traefik dashboard via secure port-forward
kubectl port-forward svc/traefik -n traefik 8090:8090

# Open browser to: http://localhost:8090/dashboard/
# Verify active Routers, Middlewares, and Service health
```

---

## 7. Emergency Deployment Rollback

If a new container image introduces a critical regression in production:
```bash
# 1. Inspect rollout history
kubectl rollout history deployment/core-api -n swiftdrop

# 2. Roll back to immediate previous revision
kubectl rollout undo deployment/core-api -n swiftdrop

# 3. Monitor rollout status
kubectl rollout status deployment/core-api -n swiftdrop
```

---

## 8. Incident Playbooks for Common Outages (SRS §14)

### Playbook A: Database Connection Pool Exhaustion (`too many clients`)
1. **Symptom:** Core API returns HTTP 500 (`pq: remaining connection slots are reserved for non-replication superuser connections`).
2. **Immediate Action:** Check pool status:
   ```sql
   SELECT count(*), state FROM pg_stat_activity GROUP BY state;
   ```
3. **Mitigation:**
   - Terminate idle connections holding locks:
     ```sql
     SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'idle in transaction' AND state_change < NOW() - INTERVAL '2 minutes';
     ```
   - Restart non-essential read worker pods.

### Playbook B: RabbitMQ Broker Crash / Split Brain
1. **Symptom:** Outbox Relay logs `connection refused` or AMQP channel disconnects.
2. **Guaranteed Behavior:** PostgreSQL commits continue safely. Outbox events accumulate safely in the database with status `PENDING`. **Zero business data is lost.**
3. **Recovery Action:**
   - Restart RabbitMQ pod: `kubectl rollout restart deployment/rabbitmq -n swiftdrop`.
   - Verify broker reaches `Ready` state.
   - Outbox Relay poller will automatically resume batch publishing without human manual replay.
