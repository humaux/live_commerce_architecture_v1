-- Purpose: loyalty export and erasure recent re-authentication guards (contracts/merchant-password-auth-v1.md §16).
--   (1) loyalty.export_member and loyalty.erase_member call identity.require_recent_auth(h, 10 minutes)
--       right after customers.tn_authority and before member row lock/read/writes.
--   (2) loyalty.require_recent_privacy: tiny fixed-window authority wrapper for Go Erase() same-tx
--       fresh and replay validation.
-- Depends on: 0201/0203 recent-auth semantics, 0223 loyalty points definers.
-- Used by: internal/loyalty (Export, Erase); direct SQL definers.
-- Migration rules: forward-only. Existing functions patched in place with pg_get_functiondef + exactly-one-anchor replace.

DO $do$
DECLARE
 r record;
 v_fn text;
 v_anchor text := $anchor$SELECT * INTO a FROM customers.tn_authority(h,s,'customers:privacy');$anchor$;
BEGIN
 FOR r IN SELECT * FROM (VALUES
  ('loyalty.export_member(bytea,uuid,uuid)'::regprocedure),
  ('loyalty.erase_member(bytea,uuid,uuid)'::regprocedure)
 ) AS t(sig) LOOP
  v_fn := pg_get_functiondef(r.sig);
  IF (length(v_fn)-length(replace(v_fn,v_anchor,'')))/length(v_anchor) <> 1 THEN
   RAISE EXCEPTION '% has an unexpected shape for the 0225 patch at: %', r.sig, left(v_anchor,60);
  END IF;
  EXECUTE replace(v_fn, v_anchor, v_anchor || E'\n PERFORM identity.require_recent_auth(h, interval ''10 minutes'');');
 END LOOP;
END $do$;

CREATE FUNCTION loyalty.require_recent_privacy(p_hash bytea, p_store uuid) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(p_hash, p_store, 'customers:privacy');
 PERFORM identity.require_recent_auth(p_hash, interval '10 minutes');
END;
$$;
ALTER FUNCTION loyalty.require_recent_privacy(bytea, uuid) OWNER TO commerce_privacy_writer;
REVOKE ALL ON FUNCTION loyalty.require_recent_privacy(bytea, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION loyalty.require_recent_privacy(bytea, uuid) TO commerce_runtime;
COMMENT ON FUNCTION loyalty.require_recent_privacy(bytea, uuid) IS 'internal/loyalty LP-1: require_recent_privacy; scoped merchant ledger/privacy contract, recent reauth guard within 10 minutes.';
