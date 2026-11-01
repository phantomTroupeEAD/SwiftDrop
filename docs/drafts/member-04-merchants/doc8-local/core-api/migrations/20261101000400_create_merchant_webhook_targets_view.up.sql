-- Member 4 — Merchant Integration
-- DRAFT migration. Read contract for Member 8 (see contracts/webhook-config-read-contract.sql).
-- Depends on: merchants, webhook_endpoints. Grant step depends on role created by Member 9.

CREATE OR REPLACE VIEW merchant_webhook_targets AS
SELECT
    w.id                      AS endpoint_id,
    w.merchant_id             AS merchant_id,
    w.url                     AS url,
    w.secret_ciphertext       AS secret_ciphertext,
    w.secret_key_version      AS secret_key_version,
    w.subscribed_event_types  AS subscribed_event_types,
    w.updated_at              AS updated_at
FROM webhook_endpoints w
JOIN merchants m ON m.id = w.merchant_id
WHERE w.enabled = true
  AND w.deleted_at IS NULL
  AND m.status = 'APPROVED';

COMMENT ON VIEW merchant_webhook_targets IS
  'Owned by Member 4. Read-only contract for Member 8 Event Worker (doc-04 I-08).';

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_worker') THEN
        EXECUTE 'GRANT SELECT ON merchant_webhook_targets TO swiftdrop_worker';
    END IF;
END $$;
