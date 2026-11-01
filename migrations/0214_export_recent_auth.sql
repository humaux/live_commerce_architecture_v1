-- Purpose: FIX-EXPORT-REAUTH, merchant-password-auth-v1 §16: guard the Go-rendered product, carrier and
-- tracking-result exports and the customer/order import-result SQL reader in their existing transaction.
-- Depends on: 0063 merchant_access_denied, 0152 read_batch_results, 0203 require_recent_auth owner grants.
-- Used by: internal/merchanttools, internal/merchantorders and internal/migrationimport CSV exports.
-- Invariants: fixed 10-minute window; authorize the session/store/permissions/GUCs before freshness;
-- no broader EXECUTE grant on require_recent_auth; no data, CSV format, RLS policy or existing ACL change.

-- Dedicated export entry points. Their owner already holds EXECUTE on require_recent_auth via 0203;
-- commerce_runtime may invoke only these fixed-window, fixed-permission exports, never the guard itself.
DO $do$
DECLARE r record;
BEGIN
 FOR r IN SELECT * FROM (VALUES
  ('authorize_products_export', ARRAY['catalog:read','inventory:read']::text[]),
  ('authorize_carrier_export', ARRAY['orders:read','orders:export']::text[]),
  ('authorize_tracking_result_export', ARRAY['orders:read']::text[])
 ) AS exports(name,permissions)
 LOOP
  EXECUTE format($ddl$
CREATE FUNCTION identity.%1$I(p_hash bytea,p_store uuid)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $fn$
DECLARE s record; v_auth_error text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid export request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,%2$L);
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,%3$L::text[],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'export access denied' USING ERRCODE=v_auth_error; END IF;
 PERFORM identity.require_recent_auth(p_hash, interval '10 minutes');
END $fn$;
$ddl$, r.name, r.permissions[1], r.permissions);
  EXECUTE format('ALTER FUNCTION identity.%I(bytea,uuid) OWNER TO commerce_auth', r.name);
  EXECUTE format('REVOKE ALL ON FUNCTION identity.%I(bytea,uuid) FROM PUBLIC', r.name);
  EXECUTE format('GRANT EXECUTE ON FUNCTION identity.%I(bytea,uuid) TO commerce_runtime', r.name);
  EXECUTE format('COMMENT ON FUNCTION identity.%I(bytea,uuid) IS %L', r.name,
   'FIX-EXPORT-REAUTH: scoped merchant CSV export authorization, fixed 10-minute recent auth in the caller transaction; owner commerce_auth, EXECUTE commerce_runtime.');
 END LOOP;
END $do$;

-- Preserve the import reader's owner, signature, ACL and result bytes; fail closed on anchor drift (0203 pattern).
DO $do$
DECLARE v_fn text;
 v_anchor text := $anchor$SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:privacy');$anchor$;
BEGIN
 SELECT pg_get_functiondef('migrationimport.read_batch_results(bytea,uuid,uuid)'::regprocedure) INTO v_fn;
 IF (length(v_fn)-length(replace(v_fn,v_anchor,'')))/length(v_anchor) <> 1 THEN
  RAISE EXCEPTION '0214 import-results authorization anchor drift';
 END IF;
 EXECUTE replace(v_fn,v_anchor,v_anchor || E'\n PERFORM identity.require_recent_auth(p_hash, interval ''10 minutes'');');
END $do$;
