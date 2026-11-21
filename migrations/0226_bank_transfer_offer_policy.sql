-- Purpose: enforce OC-Q1 for new buyer and merchant order options.
-- Depends on: 0088's scoped offer reader and 0202's existing writer-role policy read.
-- Used by: checkout.ListOptions for both buyer checkout and merchant manual orders.
-- Keep the existing signature, privileges and history paths. Only explicit policy open admits an offer.

CREATE OR REPLACE FUNCTION checkout.read_transfer_offer(p_hash bytea,p_store uuid)
RETURNS TABLE(enabled boolean,allow_cvs boolean,window_hours integer)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_scope record; c record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL THEN RAISE EXCEPTION 'invalid offer read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO v_scope FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM v_scope.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text THEN
  RAISE EXCEPTION 'offer scope' USING ERRCODE='PT403'; END IF;
 SELECT s.enabled,s.allow_cvs,s.window_hours INTO c
 FROM checkout.bank_transfer_settings s
 WHERE s.tenant_id=v_scope.tenant_id AND s.store_id=p_store
   AND EXISTS(SELECT 1 FROM checkout.bank_transfer_policy WHERE NOT closed);
 RETURN QUERY SELECT coalesce(c.enabled,false),coalesce(c.allow_cvs,false),coalesce(c.window_hours,72);
END $$;
