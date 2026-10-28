-- Purpose: OC-03 re-authentication guard for the merchant Stripe refund command (contracts/merchant-password-auth-v1.md §16).
--   payments.request_stripe_refund is created and last replaced in the post-River phase (post_river/0013, 0016), so its guard
--   cannot be applied by the numbered migration 0203; this file patches it IN PLACE (pg_get_functiondef + replace, exactly one
--   anchor or the migration raises) with identity.require_recent_auth(p_hash, 10 minutes), called right after the payments:refund
--   authorization succeeded and before any lock or write. Owner, ACL and signature are unchanged; EXECUTE on require_recent_auth
--   goes to the definer's owner (also granted by 0203 through decide_bank_transfer, same role; idempotent).
-- Depends on: 0201 (identity.require_recent_auth), 0203, post_river/0016 (current request_stripe_refund).
-- Used by: internal/merchantorders (refunds.go) through payments.request_stripe_refund.
-- Not guarded: payments.request_stripe_refund_refresh (records a throttled status-refresh signal, moves no money).
-- Status: REAL_PG only.

DO $do$
DECLARE v_sig constant text := 'payments.request_stripe_refund(bytea,uuid,uuid,text,bytea,bigint,text,bigint,uuid,bigint)';
 v_needle constant text := $n$v_tenant:=v_access.tenant_id; v_principal:=v_access.principal_id; v_revision:=v_access.authz_revision;$n$;
 v_fn text; v_owner text;
BEGIN
 v_fn := pg_get_functiondef(v_sig::regprocedure);
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.request_stripe_refund has an unexpected shape for the post_river/0028 patch'; END IF;
 EXECUTE replace(v_fn,v_needle,'PERFORM identity.require_recent_auth(p_hash, interval ''10 minutes'');'||E'\n'||v_needle);
 SELECT pg_get_userbyid(p.proowner) INTO v_owner FROM pg_proc p WHERE p.oid = v_sig::regprocedure;
 EXECUTE format('GRANT EXECUTE ON FUNCTION identity.require_recent_auth(bytea,interval) TO %I', v_owner);
END $do$;
