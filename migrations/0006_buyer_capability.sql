-- Anonymous cart authority is not a merchant membership or a verified customer.
-- Neither login role has table access, and neither is granted to merchant roles.
CREATE ROLE commerce_buyer_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE commerce_buyer_issuer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE commerce_buyer_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE SCHEMA buyer;
REVOKE ALL ON SCHEMA buyer FROM PUBLIC;
GRANT USAGE ON SCHEMA buyer TO commerce_buyer_runtime, commerce_buyer_issuer, commerce_buyer_writer;
GRANT USAGE ON SCHEMA control TO commerce_buyer_writer;

CREATE TABLE buyer.owners (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
CREATE TABLE buyer.capability_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CHECK (expires_at>created_at),
    UNIQUE (tenant_id,store_id,owner_id,id),
    FOREIGN KEY (tenant_id,store_id,owner_id) REFERENCES buyer.owners(tenant_id,store_id,id)
);
CREATE INDEX buyer_capability_expiry ON buyer.capability_sessions(expires_at);
CREATE TABLE buyer.capability_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    session_id uuid NOT NULL,
    action text NOT NULL CHECK (action IN ('capability.issued','capability.revoked')),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (session_id,action),
    FOREIGN KEY (tenant_id,store_id,owner_id,session_id)
        REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id)
);

ALTER TABLE buyer.owners ENABLE ROW LEVEL SECURITY;
ALTER TABLE buyer.owners FORCE ROW LEVEL SECURITY;
ALTER TABLE buyer.capability_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE buyer.capability_sessions FORCE ROW LEVEL SECURITY;
ALTER TABLE buyer.capability_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE buyer.capability_events FORCE ROW LEVEL SECURITY;
CREATE POLICY owners_writer ON buyer.owners TO commerce_buyer_writer USING (true) WITH CHECK (true);
CREATE POLICY sessions_writer ON buyer.capability_sessions TO commerce_buyer_writer USING (true) WITH CHECK (true);
CREATE POLICY events_writer ON buyer.capability_events TO commerce_buyer_writer USING (true) WITH CHECK (true);
GRANT SELECT,INSERT ON buyer.owners,buyer.capability_sessions,buyer.capability_events TO commerce_buyer_writer;
GRANT SELECT ON control.tenants,control.stores TO commerce_buyer_writer;
CREATE POLICY stores_buyer_lookup ON control.stores FOR SELECT TO commerce_buyer_writer USING (true);
-- PG also checks UPDATE-row visibility for SELECT ... FOR SHARE. This permits
-- locking visible stores, but WITH CHECK false still rejects an actual UPDATE.
CREATE POLICY stores_buyer_lock ON control.stores FOR UPDATE TO commerce_buyer_writer
    USING (true) WITH CHECK (false);
-- Row locks require an UPDATE column grant. No public/login role inherits this
-- role; the fixed functions below never update these identity/scope columns.
GRANT UPDATE (id) ON control.tenants,control.stores,buyer.owners TO commerce_buyer_writer;
GRANT UPDATE (revoked_at) ON buyer.capability_sessions TO commerce_buyer_writer;

-- Internal-only issuer: its caller must eventually resolve a published store
-- from trusted server configuration. Active here does NOT mean publicly live.
CREATE FUNCTION buyer.issue_capability(p_store uuid,p_hash bytea,p_ttl bigint)
RETURNS TABLE(tenant_id uuid,store_id uuid,owner_id uuid,session_id uuid,expires_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store_active boolean; v_tenant_active boolean;
    v_owner uuid; v_session uuid; v_created timestamptz; v_expiry timestamptz;
BEGIN
    IF p_store IS NULL OR p_hash IS NULL OR octet_length(p_hash)<>32
       OR p_ttl IS NULL OR p_ttl<60 OR p_ttl>2592000 THEN
        RAISE EXCEPTION 'invalid buyer capability request' USING ERRCODE='PT400';
    END IF;
    SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=p_store;
    -- Fixed scope lock order: tenant -> store -> owner -> session. Initial reads
    -- only locate rows; authorization is rechecked after all locks are held.
    SELECT t.active INTO v_tenant_active FROM control.tenants t WHERE t.id=v_tenant FOR SHARE OF t;
    SELECT s.active INTO v_store_active FROM control.stores s
      WHERE (s.tenant_id,s.id)=(v_tenant,p_store) FOR SHARE OF s;
    IF NOT FOUND OR NOT v_store_active OR NOT v_tenant_active THEN
        RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401';
    END IF;
    v_created:=clock_timestamp();
    v_expiry:=v_created+make_interval(secs=>p_ttl);
    INSERT INTO buyer.owners(tenant_id,store_id) VALUES(v_tenant,p_store) RETURNING id INTO v_owner;
    INSERT INTO buyer.capability_sessions(tenant_id,store_id,owner_id,token_hash,created_at,expires_at)
      VALUES(v_tenant,p_store,v_owner,p_hash,v_created,v_expiry) RETURNING id INTO v_session;
    INSERT INTO buyer.capability_events(tenant_id,store_id,owner_id,session_id,action)
      VALUES(v_tenant,p_store,v_owner,v_session,'capability.issued');
    RETURN QUERY SELECT v_tenant,p_store,v_owner,v_session,v_expiry;
END $$;

CREATE FUNCTION buyer.resolve_scope(p_hash bytea,p_store uuid)
RETURNS TABLE(tenant_id uuid,store_id uuid,owner_id uuid,session_id uuid)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_session buyer.capability_sessions%ROWTYPE; v_owner_active boolean;
    v_store_active boolean; v_tenant_active boolean;
BEGIN
    SELECT c.* INTO v_session FROM buyer.capability_sessions c
      WHERE c.token_hash=p_hash AND c.store_id=p_store;
    IF NOT FOUND THEN RETURN; END IF;
    SELECT t.active INTO v_tenant_active FROM control.tenants t
      WHERE t.id=v_session.tenant_id FOR SHARE OF t;
    SELECT s.active INTO v_store_active FROM control.stores s
      WHERE (s.tenant_id,s.id)=(v_session.tenant_id,p_store) FOR SHARE OF s;
    SELECT o.active INTO v_owner_active FROM buyer.owners o
      WHERE (o.tenant_id,o.store_id,o.id)=(v_session.tenant_id,p_store,v_session.owner_id)
      FOR SHARE OF o;
    SELECT c.* INTO v_session FROM buyer.capability_sessions c
      WHERE c.token_hash=p_hash AND c.store_id=p_store FOR SHARE OF c;
    -- Evaluate expiry AFTER acquiring every lock, not before a potentially long
    -- wait. Locks last through the caller's transaction; revoke waits for it.
    IF NOT FOUND OR v_owner_active IS NOT TRUE OR v_store_active IS NOT TRUE OR v_tenant_active IS NOT TRUE
       OR v_session.revoked_at IS NOT NULL OR v_session.expires_at<=clock_timestamp() THEN RETURN; END IF;
    RETURN QUERY SELECT v_session.tenant_id,v_session.store_id,v_session.owner_id,v_session.id;
END $$;

CREATE FUNCTION buyer.revoke_capability(p_hash bytea,p_store uuid)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_session buyer.capability_sessions%ROWTYPE;
BEGIN
    IF p_store IS NULL OR p_hash IS NULL OR octet_length(p_hash)<>32 THEN
        RAISE EXCEPTION 'invalid buyer capability request' USING ERRCODE='PT400';
    END IF;
    SELECT c.* INTO v_session FROM buyer.capability_sessions c
      WHERE c.token_hash=p_hash AND c.store_id=p_store FOR UPDATE OF c;
    IF NOT FOUND OR v_session.revoked_at IS NOT NULL OR v_session.expires_at<=clock_timestamp() THEN RETURN; END IF;
    UPDATE buyer.capability_sessions SET revoked_at=clock_timestamp() WHERE id=v_session.id;
    INSERT INTO buyer.capability_events(tenant_id,store_id,owner_id,session_id,action)
      VALUES(v_session.tenant_id,v_session.store_id,v_session.owner_id,v_session.id,'capability.revoked');
END $$;

ALTER FUNCTION buyer.issue_capability(uuid,bytea,bigint) OWNER TO commerce_buyer_writer;
ALTER FUNCTION buyer.resolve_scope(bytea,uuid) OWNER TO commerce_buyer_writer;
ALTER FUNCTION buyer.revoke_capability(bytea,uuid) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.issue_capability(uuid,bytea,bigint),buyer.resolve_scope(bytea,uuid),
    buyer.revoke_capability(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.issue_capability(uuid,bytea,bigint),buyer.revoke_capability(bytea,uuid) TO commerce_buyer_issuer;
GRANT EXECUTE ON FUNCTION buyer.resolve_scope(bytea,uuid) TO commerce_buyer_runtime;
