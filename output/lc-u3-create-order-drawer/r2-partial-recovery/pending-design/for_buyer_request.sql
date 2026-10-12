-- Purpose: PROPOSAL ONLY, pending owner allocation of a new migration number. Add A16R's read-only original reservation lookup.
-- Depends on: 0129 claims.for_buyer_scope, inbox.order_for_buyer; identity.principal_holds. No table, policy or table grant changes.
-- Used by: internal/claims.RequestForBuyer from merchanttools.ForBuyer.Recover, under its original checkout Begin lock.
-- Invariants: I01/I02/I03/I11; authenticated tenant/store/key, exact body/private buyer owner, no reservation or grant creation.
-- Status: DESIGN; root owns REAL_PG validation after installing this in an allocated new migration.

CREATE FUNCTION claims.for_buyer_request(p_key_hash bytea, p_request_hash bytea, p_buyer uuid) RETURNS uuid
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; v_request uuid;
BEGIN
 v:=claims.for_buyer_scope();
 IF p_key_hash IS NULL OR octet_length(p_key_hash)<>32
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_buyer IS NULL THEN
  RAISE EXCEPTION 'invalid for-buyer request' USING ERRCODE='22023';
 END IF;
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve']) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
 END IF;
 SELECT x.request_id INTO v_request FROM inbox.order_for_buyer x
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.idempotency_key_hash=p_key_hash LIMIT 1;
 IF NOT FOUND THEN RETURN NULL; END IF;
 -- Validate every matching row, including released/placed history. Current authority may differ from its stored principal.
 IF EXISTS (SELECT 1 FROM inbox.order_for_buyer x
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.idempotency_key_hash=p_key_hash
   AND (x.request_id IS DISTINCT FROM v_request OR x.request_hash IS DISTINCT FROM p_request_hash OR x.buyer_id IS DISTINCT FROM p_buyer)) THEN
  RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409';
 END IF;
 -- Release runs only once while pending exists: repeated recovery must not retimestamp expired grants.
 IF EXISTS (SELECT 1 FROM inbox.order_for_buyer x
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.request_id=v_request
   AND x.idempotency_key_hash=p_key_hash AND x.state='pending') THEN RETURN v_request; END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION claims.for_buyer_request(bytea,bytea,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.for_buyer_request(bytea,bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.for_buyer_request(bytea,bytea,uuid) TO commerce_runtime;
COMMENT ON FUNCTION claims.for_buyer_request(bytea,bytea,uuid) IS
 'internal/claims (live-console-v1 §5.1.1 A16R; caller commerce_runtime inside the authenticated merchant transaction, inventory:reserve re-verified): validates all original tenant/store/key reservation rows against one request, canonical body hash and server-issued private buyer owner; returns its request id only while pending rows remain, otherwise NULL. Read only, no principal-id filter, table grants or buyer binding changes; called under the original checkout Begin lock before atomic release and not_created seal.';
