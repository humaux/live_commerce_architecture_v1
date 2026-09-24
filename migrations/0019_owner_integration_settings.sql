-- New initial-store owners can configure their own provider accounts and
-- method settings. Do not backfill existing members or grant execution.
CREATE OR REPLACE FUNCTION identity.create_initial_store(p_token bytea,p_key text,p_hash bytea,p_tenant_name text,p_store_name text,p_warehouse_name text,p_currency text)
RETURNS TABLE(tenant_id uuid,store_id uuid,warehouse_id uuid)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_principal uuid; v_tenant uuid; v_store uuid; v_warehouse uuid; v_key text; v_hash bytea;
BEGIN
    IF p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$' OR p_hash IS NULL OR octet_length(p_hash)<>32
       OR p_tenant_name IS NULL OR length(p_tenant_name) NOT BETWEEN 1 AND 120
       OR p_store_name IS NULL OR length(p_store_name) NOT BETWEEN 1 AND 120
       OR p_warehouse_name IS NULL OR length(p_warehouse_name) NOT BETWEEN 1 AND 120
       OR p_currency IS NULL OR p_currency !~ '^[A-Z]{3}$' THEN RAISE EXCEPTION 'invalid store request' USING ERRCODE='PT400'; END IF;
    -- Global lock order is principal then session. Recheck session after locking
    -- principal: the preliminary lookup does not authorize the transaction.
    SELECT s.principal_id INTO v_principal FROM identity.sessions s WHERE s.token_hash=p_token AND s.audience='merchant';
    IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    PERFORM 1 FROM identity.principals p WHERE p.id=v_principal AND p.active FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    PERFORM 1 FROM identity.sessions s WHERE s.token_hash=p_token AND s.principal_id=v_principal AND s.audience='merchant'
      AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    SELECT i.idempotency_key,i.request_hash,i.tenant_id,i.store_id,i.warehouse_id INTO v_key,v_hash,v_tenant,v_store,v_warehouse
      FROM identity.initial_stores i WHERE i.principal_id=v_principal;
    IF FOUND THEN
        IF v_key<>p_key OR v_hash<>p_hash THEN RAISE EXCEPTION 'initial store conflict' USING ERRCODE='PT409'; END IF;
        PERFORM 1 FROM identity.memberships m JOIN control.tenants t ON t.id=m.tenant_id AND t.active
          JOIN control.stores s ON s.tenant_id=t.id AND s.id=v_store AND s.active
          WHERE m.tenant_id=v_tenant AND m.principal_id=v_principal AND m.active;
        IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
        RETURN QUERY SELECT v_tenant,v_store,v_warehouse; RETURN;
    END IF;
    INSERT INTO control.tenants(id,name) VALUES(gen_random_uuid(),p_tenant_name) RETURNING id INTO v_tenant;
    INSERT INTO control.stores(tenant_id,id,name,currency) VALUES(v_tenant,gen_random_uuid(),p_store_name,p_currency) RETURNING id INTO v_store;
    INSERT INTO identity.memberships(tenant_id,principal_id) VALUES(v_tenant,v_principal);
    INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
      SELECT v_tenant,v_store,v_principal,p FROM unnest(ARRAY['store:read','audit:read','audit:write','catalog:read','catalog:write','inventory:read','inventory:write','inventory:reserve','pricing:read','pricing:write','integration:read','integration:manage']) p;
    INSERT INTO inventory.warehouses(tenant_id,store_id,name) VALUES(v_tenant,v_store,p_warehouse_name) RETURNING id INTO v_warehouse;
    INSERT INTO identity.initial_stores(principal_id,idempotency_key,request_hash,tenant_id,store_id,warehouse_id)
      VALUES(v_principal,p_key,p_hash,v_tenant,v_store,v_warehouse);
    INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_tenant,v_store,v_principal,'merchant.store_created');
    RETURN QUERY SELECT v_tenant,v_store,v_warehouse;
END $$;

ALTER FUNCTION identity.create_initial_store(bytea,text,bytea,text,text,text,text) OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION identity.create_initial_store(bytea,text,bytea,text,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.create_initial_store(bytea,text,bytea,text,text,text,text) TO commerce_identity;
