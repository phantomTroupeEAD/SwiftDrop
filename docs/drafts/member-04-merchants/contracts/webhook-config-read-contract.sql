-- =============================================================================
-- Webhook configuration read contract: Member 4 (merchants) -> Member 8 (Event Worker)
-- Status: DRAFT for Team Meeting 2 (decision U-08). Column names finalised in doc-05.
--
-- Purpose
--   The Event Worker (services/messaging/worker) is a separate process and cannot call
--   Member 4's Go interfaces. It reads webhook targets through this view ONLY.
--   The view is the contract; the underlying tables may change behind it.
--
-- Guarantees provided by Member 4
--   1. Only rows that are currently deliverable appear:
--        webhook_endpoints.enabled = true AND deleted_at IS NULL
--        AND merchants.status = 'APPROVED'                                     (BR-W-13)
--   2. subscribed_event_types contains exact event_type strings from
--      docs/contracts/event-contracts.md (e.g. 'parcel.delivered.v1')        (U-06)
--   3. secret_ciphertext is AES-256-GCM (nonce || ciphertext || tag) encrypted with the
--      key identified by secret_key_version; plaintext is never stored        (U-03, U-22)
--   4. Column removals/renames follow expand -> migrate -> contract with at least one
--      release of overlap, announced to Member 8 before merge.
--
-- Rules for Member 8
--   - SELECT only. No INSERT/UPDATE/DELETE on webhook_endpoints or merchants.
--   - Re-read the view before each delivery attempt (endpoint may be disabled/deleted,
--     merchant may be suspended between attempts)                              (EC-11)
--   - Do not cache rows longer than 60 seconds.
-- =============================================================================

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
WHERE w.enabled = TRUE
  AND w.deleted_at IS NULL
  AND m.status = 'APPROVED';

COMMENT ON VIEW merchant_webhook_targets IS
  'Owned by Member 4 (merchants). Read-only contract for Member 8 Event Worker. See docs/drafts/member-04-merchants/doc-04.';

-- Least-privilege role for the Event Worker (role itself created by Member 9 / Member 8).
-- GRANT SELECT ON merchant_webhook_targets TO swiftdrop_worker;
-- REVOKE ALL ON webhook_endpoints, merchants, merchant_api_credentials FROM swiftdrop_worker;

-- Typical worker query (index: webhook_endpoints (merchant_id) WHERE enabled):
--   SELECT endpoint_id, url, secret_ciphertext, secret_key_version
--   FROM merchant_webhook_targets
--   WHERE merchant_id = $1
--     AND $2 = ANY (subscribed_event_types);
