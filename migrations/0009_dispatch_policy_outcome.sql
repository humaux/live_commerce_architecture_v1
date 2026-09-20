-- Preserve policy denial separately from a provider's final failure. Forward
-- replacement retains the same owner, signature, search_path and EXECUTE ACL.
CREATE OR REPLACE FUNCTION integration.complete_operation(p_id uuid,p_generation bigint,p_token bytea,p_state text,p_code text,p_reference text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_binding_id uuid; b integration.bindings%ROWTYPE; o integration.operations%ROWTYPE;
    v_now timestamptz; v_reason text;
BEGIN
    IF p_token IS NULL OR octet_length(p_token)<>32 OR p_generation IS NULL OR p_generation<1
        OR p_state IS NULL OR p_state NOT IN ('SUCCEEDED','FAILED_FINAL','UNKNOWN','ACKNOWLEDGED','BLOCKED_POLICY')
        OR p_code IS NULL OR p_code !~ '^[A-Za-z0-9_.:-]{1,80}$'
        OR p_reference IS NULL OR length(p_reference)>200 OR p_reference ~ '[[:cntrl:]]' THEN
        RAISE EXCEPTION 'invalid completion' USING ERRCODE='22023';
    END IF;
    SELECT x.binding_id INTO v_binding_id FROM integration.operations x WHERE x.id=p_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
    SELECT x.* INTO b FROM integration.bindings x WHERE x.id=v_binding_id FOR SHARE;
    SELECT x.* INTO o FROM integration.operations x WHERE x.id=p_id FOR UPDATE;
    v_now:=clock_timestamp();
    IF NOT FOUND OR o.binding_id<>b.id OR o.tenant_id<>b.tenant_id OR o.store_id<>b.store_id THEN
        RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002';
    END IF;
    IF o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=v_now
        OR o.lease_token_hash<>sha256(p_token) OR o.state NOT IN ('DISPATCHING','UNKNOWN') THEN
        RAISE EXCEPTION 'lease conflict' USING ERRCODE='40001';
    END IF;
    IF p_state='BLOCKED_POLICY' AND (o.lease_mode<>'dispatch' OR p_reference<>'') THEN
        RAISE EXCEPTION 'policy outcome requires undispatched claim' USING ERRCODE='22023';
    END IF;
    -- An observed remote result is still a fact if authorization changed while
    -- I/O was in flight. UNKNOWN cannot be relabelled as a local policy denial.
    v_reason:=CASE WHEN NOT b.enabled OR b.semantic_version<>o.binding_version
        OR b.provider<>o.provider OR b.external_asset_id<>o.external_asset_id
        THEN 'completed_binding_changed' ELSE p_code END;
    UPDATE integration.operations SET state=p_state,result_code=p_code,provider_reference=p_reference,
        lease_mode='',lease_until=NULL,lease_token_hash=NULL,updated_at=v_now WHERE id=p_id;
    INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
        VALUES(o.tenant_id,o.store_id,o.id,o.generation,p_state,'',v_reason);
END
$$;
