-- A trusted BFF can register the same cryptorandom capability after a lost
-- response. The original issuer remains non-idempotent for existing callers.
-- No plaintext token, extra identity table, runtime privilege or TTL refresh.
CREATE FUNCTION buyer.register_capability(p_store uuid,p_hash bytea,p_ttl bigint)
RETURNS TABLE(tenant_id uuid,store_id uuid,owner_id uuid,session_id uuid,expires_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_constraint text; v_schema text; v_table text;
BEGIN
    IF p_store IS NULL OR p_hash IS NULL OR octet_length(p_hash)<>32
       OR p_ttl IS NULL OR p_ttl<60 OR p_ttl>2592000 THEN
        RAISE EXCEPTION 'invalid buyer capability request' USING ERRCODE='PT400';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM buyer.capability_sessions c WHERE c.token_hash=p_hash) THEN
        BEGIN
            RETURN QUERY SELECT i.* FROM buyer.issue_capability(p_store,p_hash,p_ttl) i;
            RETURN;
        EXCEPTION WHEN unique_violation THEN
            -- A concurrent committed winner may appear only after the unique
            -- check waits. This subtransaction rolls back the losing owner too.
            -- Do not mask an unrelated integrity error as an authorized replay.
            GET STACKED DIAGNOSTICS v_constraint=CONSTRAINT_NAME,
                v_schema=SCHEMA_NAME,v_table=TABLE_NAME;
            IF v_constraint IS DISTINCT FROM 'capability_sessions_token_hash_key'
               OR v_schema IS DISTINCT FROM 'buyer'
               OR v_table IS DISTINCT FROM 'capability_sessions' THEN
                RAISE;
            END IF;
        END;
    END IF;

    -- The existing resolver owns tenant -> store -> owner -> session lock order
    -- and checks expiry AFTER waits. Volatile/READ COMMITTED execution sees a
    -- just-committed winner. Hash matches on another store never grant access.
    RETURN QUERY SELECT s.tenant_id,s.store_id,s.owner_id,s.session_id,c.expires_at
      FROM buyer.resolve_scope(p_hash,p_store) s
      JOIN buyer.capability_sessions c ON c.id=s.session_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401';
    END IF;
END $$;

ALTER FUNCTION buyer.register_capability(uuid,bytea,bigint) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.register_capability(uuid,bytea,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.register_capability(uuid,bytea,bigint) TO commerce_buyer_issuer;
