-- Constraint and view assertions for Member 4 draft migrations.
-- Each check prints PASS or raises an exception (psql ON_ERROR_STOP stops the run).
\set QUIET on
SET client_min_messages = notice;

CREATE OR REPLACE FUNCTION pg_temp.expect_error(label text, stmt text, state text)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
  BEGIN
    EXECUTE stmt;
  EXCEPTION WHEN OTHERS THEN
    IF SQLSTATE = state THEN
      RAISE NOTICE 'PASS  %  (rejected with %)', label, SQLSTATE;
      RETURN;
    END IF;
    RAISE EXCEPTION 'FAIL  %: expected SQLSTATE %, got % (%)', label, state, SQLSTATE, SQLERRM;
  END;
  RAISE EXCEPTION 'FAIL  %: statement succeeded but should have failed', label;
END $$;

CREATE OR REPLACE FUNCTION pg_temp.expect_ok(label text, stmt text)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
  EXECUTE stmt;
  RAISE NOTICE 'PASS  %', label;
END $$;

BEGIN;

INSERT INTO users (id) VALUES
  ('00000000-0000-0000-0000-0000000000a1'),  -- owner A
  ('00000000-0000-0000-0000-0000000000b1'),  -- owner B
  ('00000000-0000-0000-0000-0000000000ad');  -- admin

-- ---------- merchants ----------
SELECT pg_temp.expect_ok('M-01 insert PENDING merchant A',
 $q$INSERT INTO merchants (id, owner_user_id, business_name, contact_email, contact_phone, address)
    VALUES ('10000000-0000-0000-0000-00000000000a','00000000-0000-0000-0000-0000000000a1','Ceylon Tea Shop',
            'ops@ceylontea.lk','+94771234567','{"line1":"12 Temple Rd","city":"Kandy","country":"LK"}')$q$);

SELECT pg_temp.expect_ok('M-02 defaults: status PENDING, version 1',
 $q$DO $d$ BEGIN
      IF (SELECT status||':'||version FROM merchants WHERE id='10000000-0000-0000-0000-00000000000a') <> 'PENDING:1'
      THEN RAISE EXCEPTION 'defaults wrong'; END IF; END $d$$q$);

SELECT pg_temp.expect_error('M-03 second active merchant for same owner (BR-M-05)',
 $q$INSERT INTO merchants (owner_user_id, business_name, contact_email, contact_phone, address)
    VALUES ('00000000-0000-0000-0000-0000000000a1','Second Shop','x@y.lk','+94771234567','{"line1":"a","city":"b","country":"LK"}')$q$,
 '23505');

SELECT pg_temp.expect_error('M-04 invalid status value',
 $q$UPDATE merchants SET status='ACTIVE' WHERE id='10000000-0000-0000-0000-00000000000a'$q$, '23514');

SELECT pg_temp.expect_error('M-05 APPROVED without approved_by/approved_at',
 $q$UPDATE merchants SET status='APPROVED' WHERE id='10000000-0000-0000-0000-00000000000a'$q$, '23514');

SELECT pg_temp.expect_error('M-06 phone not E.164',
 $q$UPDATE merchants SET contact_phone='0771234567' WHERE id='10000000-0000-0000-0000-00000000000a'$q$, '23514');

SELECT pg_temp.expect_error('M-07 business name 1 char',
 $q$UPDATE merchants SET business_name=' A ' WHERE id='10000000-0000-0000-0000-00000000000a'$q$, '23514');

SELECT pg_temp.expect_error('M-08 address missing city',
 $q$UPDATE merchants SET address='{"line1":"x","country":"LK"}' WHERE id='10000000-0000-0000-0000-00000000000a'$q$, '23514');

SELECT pg_temp.expect_error('M-09 version 0',
 $q$UPDATE merchants SET version=0 WHERE id='10000000-0000-0000-0000-00000000000a'$q$, '23514');

SELECT pg_temp.expect_error('M-10 owner must exist (FK)',
 $q$INSERT INTO merchants (owner_user_id, business_name, contact_email, contact_phone, address)
    VALUES ('00000000-0000-0000-0000-000000000999','Ghost','g@h.lk','+94771234567','{"line1":"a","city":"b","country":"LK"}')$q$,
 '23503');

SELECT pg_temp.expect_ok('M-11 approve with approval fields + optimistic version bump',
 $q$DO $d$ DECLARE n int; BEGIN
      UPDATE merchants SET status='APPROVED', approved_by='00000000-0000-0000-0000-0000000000ad',
             approved_at=now(), version=version+1
       WHERE id='10000000-0000-0000-0000-00000000000a' AND version=1;
      GET DIAGNOSTICS n = ROW_COUNT; IF n <> 1 THEN RAISE EXCEPTION 'expected 1 row'; END IF; END $d$$q$);

SELECT pg_temp.expect_ok('M-12 stale version updates 0 rows (BR-M-09)',
 $q$DO $d$ DECLARE n int; BEGIN
      UPDATE merchants SET status='SUSPENDED', status_reason='late', version=version+1
       WHERE id='10000000-0000-0000-0000-00000000000a' AND version=1;
      GET DIAGNOSTICS n = ROW_COUNT; IF n <> 0 THEN RAISE EXCEPTION 'expected 0 rows'; END IF; END $d$$q$);

SELECT pg_temp.expect_ok('M-13 rejected owner may reapply (partial unique index)',
 $q$DO $d$ BEGIN
      INSERT INTO merchants (id, owner_user_id, business_name, status, contact_email, contact_phone, address)
      VALUES ('10000000-0000-0000-0000-0000000000b0','00000000-0000-0000-0000-0000000000b1','B Old','REJECTED',
              'b@b.lk','+94771234568','{"line1":"a","city":"b","country":"LK"}');
      INSERT INTO merchants (id, owner_user_id, business_name, contact_email, contact_phone, address)
      VALUES ('10000000-0000-0000-0000-00000000000b','00000000-0000-0000-0000-0000000000b1','B New',
              'b@b.lk','+94771234568','{"line1":"a","city":"b","country":"LK"}');
    END $d$$q$);

-- ---------- merchant_api_credentials ----------
SELECT pg_temp.expect_ok('K-01 insert valid credential with default scopes',
 $q$INSERT INTO merchant_api_credentials (id, merchant_id, key_prefix, key_hash, label, created_by)
    VALUES ('20000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-00000000000a','Ab3dE9xK',
            repeat('a',64),'prod','00000000-0000-0000-0000-0000000000a1')$q$);

SELECT pg_temp.expect_error('K-02 duplicate key_prefix (global)',
 $q$INSERT INTO merchant_api_credentials (merchant_id, key_prefix, key_hash, label, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000b','Ab3dE9xK',repeat('b',64),'other','00000000-0000-0000-0000-0000000000b1')$q$,
 '23505');

SELECT pg_temp.expect_error('K-03 prefix with invalid char',
 $q$INSERT INTO merchant_api_credentials (merchant_id, key_prefix, key_hash, label, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000a','Ab3d-9xK',repeat('c',64),'x','00000000-0000-0000-0000-0000000000a1')$q$,
 '23514');

SELECT pg_temp.expect_error('K-04 hash not lowercase hex',
 $q$INSERT INTO merchant_api_credentials (merchant_id, key_prefix, key_hash, label, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000a','Zz000001',repeat('G',64),'x','00000000-0000-0000-0000-0000000000a1')$q$,
 '23514');

SELECT pg_temp.expect_error('K-05 scope outside allowed set',
 $q$INSERT INTO merchant_api_credentials (merchant_id, key_prefix, key_hash, label, scopes, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000a','Zz000002',repeat('d',64),'x',ARRAY['tracking:read'],'00000000-0000-0000-0000-0000000000a1')$q$,
 '23514');

SELECT pg_temp.expect_error('K-05b old draft scope parcels:write rejected (ADR-003 names only)',
 $q$INSERT INTO merchant_api_credentials (merchant_id, key_prefix, key_hash, label, scopes, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000a','Zz00000a',repeat('d',64),'x',ARRAY['parcels:write'],'00000000-0000-0000-0000-0000000000a1')$q$,
 '23514');

SELECT pg_temp.expect_error('K-05c webhooks:manage not grantable to API keys (Doc 7 C-3)',
 $q$INSERT INTO merchant_api_credentials (merchant_id, key_prefix, key_hash, label, scopes, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000a','Zz00000b',repeat('d',64),'x',ARRAY['parcels:read','webhooks:manage'],'00000000-0000-0000-0000-0000000000a1')$q$,
 '23514');

SELECT pg_temp.expect_ok('K-05d default scopes are parcels:create + parcels:read',
 $q$DO $d$ BEGIN
      IF (SELECT scopes FROM merchant_api_credentials WHERE id='20000000-0000-0000-0000-000000000001')
         IS DISTINCT FROM ARRAY['parcels:create','parcels:read'] THEN RAISE EXCEPTION 'wrong default scopes'; END IF;
    END $d$$q$);

SELECT pg_temp.expect_error('K-06 empty scopes',
 $q$INSERT INTO merchant_api_credentials (merchant_id, key_prefix, key_hash, label, scopes, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000a','Zz000003',repeat('d',64),'x',ARRAY[]::text[],'00000000-0000-0000-0000-0000000000a1')$q$,
 '23514');

SELECT pg_temp.expect_error('K-07 same active label for same merchant (BR-K-05)',
 $q$INSERT INTO merchant_api_credentials (merchant_id, key_prefix, key_hash, label, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000a','Zz000004',repeat('e',64),'prod','00000000-0000-0000-0000-0000000000a1')$q$,
 '23505');

SELECT pg_temp.expect_ok('K-08 label reusable after revoke',
 $q$DO $d$ BEGIN
      UPDATE merchant_api_credentials SET revoked_at = now() WHERE id='20000000-0000-0000-0000-000000000001';
      INSERT INTO merchant_api_credentials (merchant_id, key_prefix, key_hash, label, created_by)
      VALUES ('10000000-0000-0000-0000-00000000000a','Zz000005',repeat('f',64),'prod','00000000-0000-0000-0000-0000000000a1');
    END $d$$q$);

SELECT pg_temp.expect_ok('K-09 same label allowed for a different merchant',
 $q$INSERT INTO merchant_api_credentials (merchant_id, key_prefix, key_hash, label, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000b','Zz000006',repeat('0',64),'prod','00000000-0000-0000-0000-0000000000b1')$q$);

SELECT pg_temp.expect_error('K-10 expires_at before created_at',
 $q$INSERT INTO merchant_api_credentials (merchant_id, key_prefix, key_hash, label, created_by, expires_at)
    VALUES ('10000000-0000-0000-0000-00000000000a','Zz000007',repeat('1',64),'exp','00000000-0000-0000-0000-0000000000a1', now() - interval '1 day')$q$,
 '23514');

SELECT pg_temp.expect_error('K-11 merchant with credentials cannot be hard-deleted (RESTRICT)',
 $q$DELETE FROM merchants WHERE id='10000000-0000-0000-0000-00000000000a'$q$, '23503');

-- ---------- webhook_endpoints ----------
SELECT pg_temp.expect_ok('W-01 insert webhook for A',
 $q$INSERT INTO webhook_endpoints (id, merchant_id, url, secret_ciphertext, secret_key_version, secret_last4, subscribed_event_types, created_by)
    VALUES ('30000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-00000000000a','https://ceylontea.lk/hooks',
            decode(repeat('ab',40),'hex'),1,'Jk9m',ARRAY['parcel.delivered.v1'],'00000000-0000-0000-0000-0000000000a1')$q$);

SELECT pg_temp.expect_error('W-02 duplicate live URL for same merchant (BR-W-08)',
 $q$INSERT INTO webhook_endpoints (merchant_id, url, secret_ciphertext, secret_key_version, secret_last4, subscribed_event_types, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000a','https://ceylontea.lk/hooks',decode(repeat('ab',40),'hex'),1,'aaaa',ARRAY['parcel.booked.v1'],'00000000-0000-0000-0000-0000000000a1')$q$,
 '23505');

SELECT pg_temp.expect_error('W-03 non-http(s) scheme',
 $q$INSERT INTO webhook_endpoints (merchant_id, url, secret_ciphertext, secret_key_version, secret_last4, subscribed_event_types, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000a','ftp://x.lk/h',decode(repeat('ab',40),'hex'),1,'aaaa',ARRAY['parcel.booked.v1'],'00000000-0000-0000-0000-0000000000a1')$q$,
 '23514');

SELECT pg_temp.expect_error('W-04 empty event list',
 $q$INSERT INTO webhook_endpoints (merchant_id, url, secret_ciphertext, secret_key_version, secret_last4, subscribed_event_types, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000a','https://x.lk/h2',decode(repeat('ab',40),'hex'),1,'aaaa',ARRAY[]::text[],'00000000-0000-0000-0000-0000000000a1')$q$,
 '23514');

SELECT pg_temp.expect_error('W-05 ciphertext too short',
 $q$INSERT INTO webhook_endpoints (merchant_id, url, secret_ciphertext, secret_key_version, secret_last4, subscribed_event_types, created_by)
    VALUES ('10000000-0000-0000-0000-00000000000a','https://x.lk/h3','\x0102'::bytea,1,'aaaa',ARRAY['parcel.booked.v1'],'00000000-0000-0000-0000-0000000000a1')$q$,
 '23514');

SELECT pg_temp.expect_error('W-06 soft-deleted but still enabled',
 $q$UPDATE webhook_endpoints SET deleted_at=now() WHERE id='30000000-0000-0000-0000-000000000001'$q$, '23514');

SELECT pg_temp.expect_ok('W-07 soft delete then re-register same URL',
 $q$DO $d$ BEGIN
      UPDATE webhook_endpoints SET deleted_at=now(), enabled=false WHERE id='30000000-0000-0000-0000-000000000001';
      INSERT INTO webhook_endpoints (id, merchant_id, url, secret_ciphertext, secret_key_version, secret_last4, subscribed_event_types, created_by)
      VALUES ('30000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-00000000000a','https://ceylontea.lk/hooks',
              decode(repeat('cd',40),'hex'),1,'Zx12',ARRAY['parcel.delivered.v1','parcel.booked.v1'],'00000000-0000-0000-0000-0000000000a1');
    END $d$$q$);

-- ---------- view merchant_webhook_targets ----------
SELECT pg_temp.expect_ok('V-01 setup: B webhook enabled but B is PENDING; A disabled webhook',
 $q$DO $d$ BEGIN
      INSERT INTO webhook_endpoints (merchant_id, url, secret_ciphertext, secret_key_version, secret_last4, subscribed_event_types, created_by)
      VALUES ('10000000-0000-0000-0000-00000000000b','https://bshop.lk/h',decode(repeat('ef',40),'hex'),1,'bbbb',ARRAY['parcel.delivered.v1'],'00000000-0000-0000-0000-0000000000b1');
      INSERT INTO webhook_endpoints (merchant_id, url, secret_ciphertext, secret_key_version, secret_last4, subscribed_event_types, enabled, created_by)
      VALUES ('10000000-0000-0000-0000-00000000000a','https://ceylontea.lk/off',decode(repeat('ef',40),'hex'),1,'cccc',ARRAY['parcel.delivered.v1'],false,'00000000-0000-0000-0000-0000000000a1');
    END $d$$q$);

SELECT pg_temp.expect_ok('V-02 view returns only enabled + not deleted + APPROVED (exactly endpoint ...0002)',
 $q$DO $d$ DECLARE ids text; BEGIN
      SELECT string_agg(endpoint_id::text, ',') INTO ids FROM merchant_webhook_targets;
      IF ids IS DISTINCT FROM '30000000-0000-0000-0000-000000000002' THEN RAISE EXCEPTION 'view returned %', ids; END IF;
    END $d$$q$);

SELECT pg_temp.expect_ok('V-03 worker query by merchant + event type',
 $q$DO $d$ DECLARE n int; BEGIN
      SELECT count(*) INTO n FROM merchant_webhook_targets
       WHERE merchant_id='10000000-0000-0000-0000-00000000000a' AND 'parcel.booked.v1' = ANY(subscribed_event_types);
      IF n <> 1 THEN RAISE EXCEPTION 'expected 1, got %', n; END IF;
      SELECT count(*) INTO n FROM merchant_webhook_targets
       WHERE merchant_id='10000000-0000-0000-0000-00000000000a' AND 'parcel.assigned.v1' = ANY(subscribed_event_types);
      IF n <> 0 THEN RAISE EXCEPTION 'expected 0, got %', n; END IF;
    END $d$$q$);

SELECT pg_temp.expect_ok('V-04 suspending merchant removes its endpoints from the view (BR-W-13)',
 $q$DO $d$ DECLARE n int; BEGIN
      UPDATE merchants SET status='SUSPENDED', status_reason='test' WHERE id='10000000-0000-0000-0000-00000000000a';
      SELECT count(*) INTO n FROM merchant_webhook_targets; IF n <> 0 THEN RAISE EXCEPTION 'expected 0, got %', n; END IF;
    END $d$$q$);

SELECT pg_temp.expect_ok('V-05 worker role can SELECT view and NOT base tables',
 $q$DO $d$ BEGIN
      IF NOT has_table_privilege('swiftdrop_worker','merchant_webhook_targets','SELECT') THEN RAISE EXCEPTION 'no SELECT on view'; END IF;
      IF has_table_privilege('swiftdrop_worker','webhook_endpoints','SELECT') THEN RAISE EXCEPTION 'worker can read base table'; END IF;
      IF has_table_privilege('swiftdrop_worker','merchant_api_credentials','SELECT') THEN RAISE EXCEPTION 'worker can read credentials'; END IF;
    END $d$$q$);

-- ---------- indexes exist ----------
SELECT pg_temp.expect_ok('I-01 all planned indexes exist',
 $q$DO $d$ DECLARE missing text; BEGIN
      SELECT string_agg(x, ',') INTO missing FROM unnest(ARRAY[
        'uq_merchants_active_owner','idx_merchants_owner_created','idx_merchants_status_created',
        'uq_credentials_key_prefix','uq_credentials_merchant_label_active','idx_credentials_merchant_created',
        'uq_webhooks_merchant_url_live','idx_webhooks_merchant_live','idx_webhooks_merchant_enabled']) x
      WHERE NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = x);
      IF missing IS NOT NULL THEN RAISE EXCEPTION 'missing indexes: %', missing; END IF;
    END $d$$q$);

SELECT pg_temp.expect_ok('I-02 auth lookup by key_prefix uses unique index',
 $q$DO $d$ DECLARE plan text; BEGIN
      SET LOCAL enable_seqscan = off;
      EXECUTE 'EXPLAIN SELECT key_hash FROM merchant_api_credentials WHERE key_prefix = ''Zz000005''' INTO plan;
      IF plan NOT LIKE '%uq_credentials_key_prefix%' THEN RAISE EXCEPTION 'plan: %', plan; END IF;
    END $d$$q$);

ROLLBACK;
\echo 'ALL ASSERTIONS PASSED'
