-- Purpose: reject stale merchant billing actions before external provider writes (BILL-271).
-- Depends on: 0079 billing authority and 0201/0203 recent-auth semantics/owner grants.
-- Used by: internal/billing StartCheckout and OpenPortal through a short scoped transaction.
-- Invariants: fixed window; server-authenticated store scope; identity guard ACL unchanged;
-- final pin_customer and record_checkout_session guards still run in their write transactions.
CREATE FUNCTION billing.require_recent_manage(p_hash bytea, p_store uuid) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s pg_catalog.record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash) <> 32 OR p_store IS NULL
    OR current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'invalid billing preflight' USING ERRCODE='PT400';
 END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'billing:manage');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::pg_catalog.text
 OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::pg_catalog.text
 OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::pg_catalog.text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
 END IF;
 PERFORM identity.require_recent_auth(p_hash, interval '10 minutes');
END;
$$;
ALTER FUNCTION billing.require_recent_manage(bytea,uuid) OWNER TO commerce_billing_writer;
REVOKE ALL ON FUNCTION billing.require_recent_manage(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION billing.require_recent_manage(bytea,uuid) TO commerce_runtime;
