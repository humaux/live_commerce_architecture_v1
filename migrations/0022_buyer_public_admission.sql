-- Public BFF admission and durable logout reuse the existing hash-only ledger.
-- Legacy private issuance/registration remain available to private callers.
CREATE INDEX buyer_capability_store_created ON buyer.capability_sessions(store_id,created_at);

CREATE FUNCTION buyer.register_capability_limited(p_store uuid,p_hash bytea,p_ttl bigint)
RETURNS TABLE(tenant_id uuid,store_id uuid,owner_id uuid,session_id uuid,expires_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_result record; v_created timestamptz; v_minute timestamptz; v_count bigint;
BEGIN
    IF p_store IS NULL OR p_hash IS NULL OR octet_length(p_hash)<>32
       OR p_ttl IS NULL OR p_ttl<60 OR p_ttl>2592000 THEN
        RAISE EXCEPTION 'invalid buyer capability request' USING ERRCODE='PT400';
    END IF;
    -- Advisory locking serializes writers, but cannot refresh an old RR snapshot.
    IF current_setting('transaction_isolation')<>'read committed' THEN
        RAISE EXCEPTION 'admission unavailable' USING ERRCODE='PT503';
    END IF;
    IF EXISTS (SELECT 1 FROM buyer.capability_sessions c WHERE c.token_hash=p_hash) THEN
        RETURN QUERY SELECT r.* FROM buyer.register_capability(p_store,p_hash,p_ttl) r;
        RETURN;
    END IF;
    -- ponytail: one per-store admission lock; no shared in-process counter or
    -- extra quota table. Fixed 600/min ceiling is not a throughput promise.
    PERFORM pg_advisory_xact_lock(hashtextextended('buyer-admission:'||p_store::text,0));
    IF EXISTS (SELECT 1 FROM buyer.capability_sessions c WHERE c.token_hash=p_hash) THEN
        RETURN QUERY SELECT r.* FROM buyer.register_capability(p_store,p_hash,p_ttl) r;
        RETURN;
    END IF;
    SELECT r.* INTO v_result FROM buyer.register_capability(p_store,p_hash,p_ttl) r;
    SELECT c.created_at INTO v_created FROM buyer.capability_sessions c WHERE c.id=v_result.session_id;
    v_minute:=date_trunc('minute',v_created,'UTC');
    SELECT count(*) INTO v_count FROM (
      SELECT 1 FROM buyer.capability_sessions c WHERE c.store_id=p_store
        AND c.created_at>=v_minute AND c.created_at<v_minute+interval '1 minute' LIMIT 601
    ) counted;
    -- Count the INSERT's minute, not the time before a possible clock rollover.
    -- Raising rolls back the newly created owner, session and issue event too.
    IF v_count>600 THEN
        RAISE EXCEPTION 'admission limited' USING ERRCODE='PT429';
    END IF;
    RETURN QUERY SELECT v_result.tenant_id,v_result.store_id,v_result.owner_id,
      v_result.session_id,v_result.expires_at;
END $$;

CREATE FUNCTION buyer.retire_capability(p_store uuid,p_hash bytea,p_ttl bigint)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
    IF p_store IS NULL OR p_hash IS NULL OR octet_length(p_hash)<>32
       OR p_ttl IS NULL OR p_ttl<60 OR p_ttl>2592000 THEN
        RAISE EXCEPTION 'invalid buyer capability request' USING ERRCODE='PT400';
    END IF;
    IF current_setting('transaction_isolation')<>'read committed' THEN
        RAISE EXCEPTION 'retirement unavailable' USING ERRCODE='PT503';
    END IF;
    -- Two retirements must not both acquire SHARE then try upgrading the same
    -- session. Registration itself is serialized by the retained unique hash.
    PERFORM pg_advisory_xact_lock(hashtextextended('buyer-retire:'||encode(p_hash,'hex'),0));
    IF NOT EXISTS (SELECT 1 FROM buyer.capability_sessions c WHERE c.token_hash=p_hash AND c.store_id=p_store) THEN
        BEGIN
            PERFORM r.session_id FROM buyer.register_capability_limited(p_store,p_hash,p_ttl) r;
        EXCEPTION WHEN SQLSTATE 'PT401' THEN
            -- A concurrent winner can already be revoked/expired. Accept only
            -- its exact store/hash row, never a cross-store or absent capability.
            IF NOT EXISTS (SELECT 1 FROM buyer.capability_sessions c WHERE c.token_hash=p_hash AND c.store_id=p_store) THEN
                RAISE;
            END IF;
        END;
    END IF;
    -- Unknown-token DELETE is not a tombstone. Register+revoke in one transaction
    -- makes a delayed bootstrap fail instead of resurrecting a logged-out buyer.
    PERFORM buyer.revoke_capability(p_hash,p_store);
END $$;

ALTER FUNCTION buyer.register_capability_limited(uuid,bytea,bigint) OWNER TO commerce_buyer_writer;
ALTER FUNCTION buyer.retire_capability(uuid,bytea,bigint) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.register_capability_limited(uuid,bytea,bigint),buyer.retire_capability(uuid,bytea,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.register_capability_limited(uuid,bytea,bigint),buyer.retire_capability(uuid,bytea,bigint) TO commerce_buyer_issuer;
