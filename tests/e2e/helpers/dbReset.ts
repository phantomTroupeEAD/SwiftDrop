import { execSync } from 'node:child_process';

/**
 * Resets the PostgreSQL database for the Evaluator Demo so that only the canonical
 * Admin account exists, while preserving migrations, delivery zones, postal codes,
 * and pricing rules required for live order calculation.
 */
export function resetDatabaseToAdminOnly(): void {
  const sql = `
BEGIN;
ALTER TABLE parcel_status_history DISABLE TRIGGER trg_parcel_status_history_immutable;
ALTER TABLE audit_logs DISABLE TRIGGER trg_audit_logs_immutable;

DELETE FROM webhook_delivery_attempts;
DELETE FROM webhook_deliveries;
DELETE FROM webhook_endpoints;
DELETE FROM notification_deliveries;
DELETE FROM processed_events;
DELETE FROM outbox_events;
DELETE FROM delivery_attempts;
DELETE FROM courier_assignments;
DELETE FROM parcel_status_history;
DELETE FROM parcels;
DELETE FROM pricing_quotes;
DELETE FROM merchant_api_credentials;
DELETE FROM merchants;
DELETE FROM couriers;
DELETE FROM idempotency_keys;
DELETE FROM email_otps;
DELETE FROM oauth_identities;
DELETE FROM refresh_tokens;
DELETE FROM audit_logs;
DELETE FROM user_roles WHERE user_id != '018f4a7c-1001-7000-8000-000000000001';
DELETE FROM users WHERE id != '018f4a7c-1001-7000-8000-000000000001';

ALTER TABLE parcel_status_history ENABLE TRIGGER trg_parcel_status_history_immutable;
ALTER TABLE audit_logs ENABLE TRIGGER trg_audit_logs_immutable;
COMMIT;
`;

  try {
    execSync('docker exec -i swiftdrop-postgres psql -U swiftdrop -d swiftdrop_db', {
      input: sql,
      stdio: ['pipe', 'pipe', 'pipe'],
    });
    console.log('[dbReset] Database successfully reset: only Admin user remains.');
  } catch (err: any) {
    console.error('[dbReset] Failed to reset database:', err?.message || err);
    throw err;
  }
}

if (process.argv[1]?.includes('dbReset')) {
  resetDatabaseToAdminOnly();
}
