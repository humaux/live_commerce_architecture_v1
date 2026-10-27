-- Purpose: OC-03 re-authentication SQL guards (contracts/merchant-password-auth-v1.md §16, ADR-20261026 T21R-01).
--   (1) Every guarded business definer calls identity.require_recent_auth(<session hash>, 10 minutes) in its own transaction,
--       right after its authorization succeeded (an unknown, revoked or expired session still answers 401/404/403 as before) and
--       before any lock or write: staff_invite / staff_set_role / staff_remove, set_platform_stripe, request_stripe_refund
--       (post_river/0028), decide_bank_transfer (also the offline refunds), the exports (unshipped orders, finance summary,
--       report/products CSV, customer privacy export), customer erasure, the billing:manage writers (pin_customer,
--       record_checkout_session) and the ads approve/publish pair. The merchant branch only for the dual-mode privacy definers.
--   (2) The owner role cannot be created: staff_invite and staff_set_role refuse role 'owner', staff_accept refuses an owner
--       invitation issued before this migration (PT403 owner_role_disabled); no data is rewritten.
--   (3) EXECUTE on identity.require_recent_auth goes to the owner role of each guarded definer, nobody else.
--   (4) Issue #215 items 1 and 2 on the 0201 functions: complete_oidc_reauth never moves authenticated_at backwards;
--       require_recent_auth treats an authenticated_at more than one minute in the future (or infinity) as not recent.
-- Depends on: 0201 (require_recent_auth, complete_oidc_reauth), 0089, 0137, 0099, 0073, 0078, 0147, 0156, 0079, 0074.
-- Used by: internal/identity, merchantorders, payments, reporting, customers, billing, ads through their existing SQL calls.
-- Migration rules: forward-only. Existing functions are patched IN PLACE with the pg_get_functiondef + replace idiom (0163, 0200):
--   each anchor must match exactly once or the migration raises and nothing is applied. CREATE OR REPLACE keeps owner, ACL and
--   signature. The window is the fixed default of §16 (10 minutes); the Go layer's COMMERCE_REAUTH_WINDOW_MINUTES only decides
--   when the UI asks earlier.
-- Not guarded here (no session-hash entry): see output/oc-03/DEFINERS.md.
-- Status: REAL_PG only.

DO $do$
DECLARE r record; v_fn text; v_owner text; v_ins text; v_repl text;
BEGIN
 FOR r IN SELECT * FROM (VALUES
  -- staff: the owner context is the authorization; owner role refused right after the freshness check
  ('identity.staff_invite(bytea,uuid,uuid,text,text,text,bytea)',
   $n$SELECT * INTO c FROM identity.staff_owner_ctx(p_hash, p_store);$n$, 'after', 'p_hash',
   $e$IF p_role = 'owner' THEN RAISE EXCEPTION 'owner_role_disabled' USING ERRCODE = 'PT403'; END IF;$e$),
  ('identity.staff_set_role(bytea,uuid,uuid,text)',
   $n$SELECT * INTO c FROM identity.staff_owner_ctx(p_hash, p_store);$n$, 'after', 'p_hash', NULL),
  -- the target lookup (404) stays first, then the owner role is refused even when it would be a no-op
  ('identity.staff_set_role(bytea,uuid,uuid,text)',
   $n$IF v_old = p_role THEN RETURN; END IF;$n$, 'before', NULL,
   $e$IF p_role = 'owner' THEN RAISE EXCEPTION 'owner_role_disabled' USING ERRCODE = 'PT403'; END IF;$e$),
  ('identity.staff_remove(bytea,uuid,uuid)',
   $n$SELECT * INTO c FROM identity.staff_owner_ctx(p_hash, p_store);$n$, 'after', 'p_hash', NULL),
  -- an owner invitation issued before this migration is refused after the invitation is known valid and before it is used
  ('identity.staff_accept(bytea,bytea)',
   $n$IF EXISTS (SELECT 1 FROM identity.store_staff t WHERE t.tenant_id = i.tenant_id AND t.store_id = i.store_id AND t.principal_id = v_principal) THEN$n$, 'before', NULL,
   $e$IF i.role = 'owner' THEN RAISE EXCEPTION 'owner_role_disabled' USING ERRCODE = 'PT403'; END IF;$e$),
  -- billing:manage
  ('payments.set_platform_stripe(bytea,uuid,text,boolean,text,text,bigint)',
   $n$IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;$n$, 'after', 'p_token', NULL),
  ('billing.pin_customer(bytea,uuid,text,text,text)',
   $n$IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;$n$, 'after', 'p_hash', NULL),
  ('billing.record_checkout_session(bytea,uuid,text,text,timestamptz)',
   $n$IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;$n$, 'after', 'p_hash', NULL),
  -- offline refunds and every bank-transfer decision (confirm / reject / refund_offline / refund_offline_restock)
  ('payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text)',
   $n$IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;$n$, 'after', 'p_hash', NULL),
  -- exports
  ('identity.export_unshipped_orders(bytea,uuid,integer)',
   $n$IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;$n$, 'after', 'p_hash', NULL),
  ('identity.export_finance_summary(bytea,uuid,date,date)',
   $n$IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;$n$, 'after', 'p_hash', NULL),
  ('identity.export_report(bytea,uuid,text,date,date,uuid,text)',
   $n$SELECT * INTO s FROM identity.report_open(p_hash,p_store,p_from,p_to,'orders:export',p_environment);$n$, 'after', 'p_hash', NULL),
  -- customer privacy: export and erasure, merchant branch only (the buyer branch authenticates a capability session, not a merchant)
  ('customers.record_export(bytea,uuid,uuid,text,uuid,jsonb)',
   $n$IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;$n$, 'after', 'p_hash', NULL),
  ('customers.export_import_profile(bytea,uuid,uuid)',
   $n$SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:privacy');$n$, 'after', 'p_hash', NULL),
  ('customers.erase_owner(bytea,uuid,uuid,text,uuid)',
   $n$IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;$n$, 'after', 'p_hash', NULL),
  -- Meta ads
  ('ads.approve_draft(bytea,uuid,uuid,integer)',
   $n$SELECT * INTO a FROM ads.auth(p_hash,p_store,ARRAY['ads:approve']);$n$, 'after', 'p_hash', NULL),
  ('ads.publish_draft(bytea,uuid,uuid,integer,uuid,bigint)',
   $n$SELECT * INTO a FROM ads.auth(p_hash,p_store,ARRAY['ads:approve']);$n$, 'after', 'p_hash', NULL),
  -- issue #215 item 2: a future (beyond one minute of skew) or infinite authenticated_at is not recent
  ('identity.require_recent_auth(bytea,interval)',
   $n$IF NOT FOUND OR v_at IS NULL OR v_at < now() - p_window THEN$n$, 'replace', NULL,
   $e$IF NOT FOUND OR v_at IS NULL OR v_at < now() - p_window OR v_at > clock_timestamp() + interval '1 minute' THEN$e$),
  -- issue #215 item 1: an OIDC re-auth never moves authenticated_at backwards
  ('identity.complete_oidc_reauth(bytea,text,text,timestamptz)',
   $n$SET authenticated_at = LEAST(p_auth_time, clock_timestamp())$n$, 'replace', NULL,
   $e$SET authenticated_at = GREATEST(s.authenticated_at, LEAST(p_auth_time, clock_timestamp()))$e$)
 ) AS t(sig, needle, pos, hv, extra) LOOP
  v_fn := pg_get_functiondef(r.sig::regprocedure);
  IF (length(v_fn)-length(replace(v_fn,r.needle,'')))<>length(r.needle) THEN
   RAISE EXCEPTION '% has an unexpected shape for the 0203 patch at: %', r.sig, left(r.needle,60); END IF;
  v_ins := CASE WHEN r.hv IS NULL THEN '' ELSE 'PERFORM identity.require_recent_auth('||r.hv||', interval ''10 minutes'');'||E'\n' END
           || coalesce(r.extra,'');
  v_repl := CASE r.pos WHEN 'after' THEN r.needle||E'\n'||v_ins WHEN 'before' THEN v_ins||E'\n'||r.needle ELSE r.extra END;
  EXECUTE replace(v_fn,r.needle,v_repl);
  IF r.hv IS NOT NULL THEN
   -- EXECUTE for the owner role of the guarded definer, the role that actually makes the call
   SELECT pg_get_userbyid(p.proowner) INTO v_owner FROM pg_proc p WHERE p.oid = r.sig::regprocedure;
   EXECUTE format('GRANT EXECUTE ON FUNCTION identity.require_recent_auth(bytea,interval) TO %I', v_owner);
  END IF;
 END LOOP;
END $do$;
