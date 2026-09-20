-- Trusted identity authority is intentionally separate from business runtime.
CREATE ROLE commerce_identity NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
CREATE ROLE commerce_identity_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
GRANT USAGE ON SCHEMA identity TO commerce_identity;
GRANT USAGE ON SCHEMA identity, control, inventory, ops TO commerce_identity_writer;

CREATE TABLE identity.login_flows (
    state_hash bytea PRIMARY KEY CHECK (octet_length(state_hash)=32),
    binding_hash bytea NOT NULL CHECK (octet_length(binding_hash)=32),
    provider_key text NOT NULL CHECK (length(provider_key) BETWEEN 1 AND 128),
    nonce text NOT NULL CHECK (length(nonce)=43),
    verifier text NOT NULL CHECK (length(verifier)=43),
    expires_at timestamptz NOT NULL
);
CREATE INDEX login_flows_expiry ON identity.login_flows(expires_at);
CREATE TABLE identity.external_identities (
    issuer text NOT NULL CHECK (length(issuer) BETWEEN 1 AND 2048),
    subject text NOT NULL CHECK (length(subject) BETWEEN 1 AND 255),
    principal_id uuid NOT NULL REFERENCES identity.principals(id),
    PRIMARY KEY (issuer,subject)
);
CREATE INDEX external_identity_principal ON identity.external_identities(principal_id);
CREATE TABLE identity.initial_stores (
    principal_id uuid PRIMARY KEY REFERENCES identity.principals(id),
    idempotency_key text NOT NULL CHECK (idempotency_key ~ '^[A-Za-z0-9_.:-]{8,128}$'),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    FOREIGN KEY (tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
    FOREIGN KEY (tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id),
    FOREIGN KEY (tenant_id,store_id,warehouse_id) REFERENCES inventory.warehouses(tenant_id,store_id,id)
);
CREATE TABLE identity.session_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    principal_id uuid NOT NULL REFERENCES identity.principals(id),
    session_id uuid NOT NULL REFERENCES identity.sessions(id),
    action text NOT NULL CHECK (action IN ('session.issued','session.revoked')),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (session_id,action)
);

GRANT SELECT,INSERT,DELETE ON identity.login_flows TO commerce_identity_writer;
GRANT SELECT,INSERT ON identity.external_identities,identity.initial_stores,
    identity.principals,identity.memberships,identity.store_grants,
    identity.sessions,identity.session_events,control.tenants,control.stores,
    inventory.warehouses TO commerce_identity_writer;
-- PostgreSQL requires an UPDATE column grant for SELECT FOR UPDATE. This role
-- is the trusted issuer, never a tenant credential or the business API login.
GRANT UPDATE (id) ON identity.principals TO commerce_identity_writer;
GRANT UPDATE (revoked_at) ON identity.sessions TO commerce_identity_writer;
GRANT INSERT ON ops.audit_events TO commerce_identity_writer;
CREATE POLICY stores_identity_read ON control.stores FOR SELECT TO commerce_identity_writer USING (true);
CREATE POLICY stores_identity_create ON control.stores FOR INSERT TO commerce_identity_writer WITH CHECK (true);
CREATE POLICY warehouses_identity_read ON inventory.warehouses FOR SELECT TO commerce_identity_writer USING (true);
CREATE POLICY warehouses_identity_create ON inventory.warehouses FOR INSERT TO commerce_identity_writer WITH CHECK (true);
CREATE POLICY audit_identity_create ON ops.audit_events FOR INSERT TO commerce_identity_writer WITH CHECK (true);

-- This fixed function surface, not direct table DML, is the authentication login's
-- authority. No function accepts a target tenant/principal or arbitrary grants.
CREATE FUNCTION identity.start_login_flow(p_state bytea,p_binding bytea,p_provider text,p_nonce text,p_verifier text)
RETURNS timestamptz LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
    INSERT INTO identity.login_flows(state_hash,binding_hash,provider_key,nonce,verifier,expires_at)
    VALUES(p_state,p_binding,p_provider,p_nonce,p_verifier,clock_timestamp()+interval '5 minutes') RETURNING expires_at
$$;
CREATE FUNCTION identity.consume_login_flow(p_state bytea,p_binding bytea,p_provider text)
RETURNS TABLE(nonce text,verifier text) LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
    DELETE FROM identity.login_flows WHERE state_hash=p_state AND binding_hash=p_binding
      AND provider_key=p_provider AND expires_at>clock_timestamp() RETURNING nonce,verifier
$$;
CREATE FUNCTION identity.issue_merchant_session(p_issuer text,p_subject text,p_hash bytea,p_ttl bigint)
RETURNS TABLE(principal_id uuid,expires_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_principal uuid; v_active boolean; v_session uuid; v_expiry timestamptz;
BEGIN
    IF p_ttl IS NULL OR p_ttl<300 OR p_ttl>86400 OR p_issuer IS NULL OR length(p_issuer) NOT BETWEEN 1 AND 2048
       OR p_subject IS NULL OR length(p_subject) NOT BETWEEN 1 AND 255 THEN RAISE EXCEPTION 'invalid identity request' USING ERRCODE='PT400'; END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(json_build_array(p_issuer,p_subject)::text,0));
    SELECT p.id,p.active INTO v_principal,v_active FROM identity.external_identities e
      JOIN identity.principals p ON p.id=e.principal_id WHERE e.issuer=p_issuer AND e.subject=p_subject FOR UPDATE OF p;
    IF NOT FOUND THEN
        INSERT INTO identity.principals(id) VALUES(gen_random_uuid()) RETURNING id INTO v_principal;
        INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES(p_issuer,p_subject,v_principal);
    ELSIF NOT v_active THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at)
      VALUES(gen_random_uuid(),p_hash,v_principal,'merchant',clock_timestamp()+p_ttl*interval '1 second') RETURNING id,identity.sessions.expires_at INTO v_session,v_expiry;
    INSERT INTO identity.session_events(principal_id,session_id,action) VALUES(v_principal,v_session,'session.issued');
    RETURN QUERY SELECT v_principal,v_expiry;
END $$;
CREATE FUNCTION identity.create_initial_store(p_token bytea,p_key text,p_hash bytea,p_tenant_name text,p_store_name text,p_warehouse_name text,p_currency text)
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
      SELECT v_tenant,v_store,v_principal,p FROM unnest(ARRAY['store:read','audit:read','audit:write','catalog:read','catalog:write','inventory:read','inventory:write','inventory:reserve']) p;
    INSERT INTO inventory.warehouses(tenant_id,store_id,name) VALUES(v_tenant,v_store,p_warehouse_name) RETURNING id INTO v_warehouse;
    INSERT INTO identity.initial_stores(principal_id,idempotency_key,request_hash,tenant_id,store_id,warehouse_id)
      VALUES(v_principal,p_key,p_hash,v_tenant,v_store,v_warehouse);
    INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_tenant,v_store,v_principal,'merchant.store_created');
    RETURN QUERY SELECT v_tenant,v_store,v_warehouse;
END $$;
CREATE FUNCTION identity.revoke_merchant_session(p_hash bytea)
RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
    WITH revoked AS (UPDATE identity.sessions SET revoked_at=clock_timestamp()
      WHERE token_hash=p_hash AND audience='merchant' AND revoked_at IS NULL RETURNING id,principal_id)
    INSERT INTO identity.session_events(principal_id,session_id,action) SELECT principal_id,id,'session.revoked' FROM revoked
$$;

ALTER FUNCTION identity.start_login_flow(bytea,bytea,text,text,text) OWNER TO commerce_identity_writer;
ALTER FUNCTION identity.consume_login_flow(bytea,bytea,text) OWNER TO commerce_identity_writer;
ALTER FUNCTION identity.issue_merchant_session(text,text,bytea,bigint) OWNER TO commerce_identity_writer;
ALTER FUNCTION identity.create_initial_store(bytea,text,bytea,text,text,text,text) OWNER TO commerce_identity_writer;
ALTER FUNCTION identity.revoke_merchant_session(bytea) OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION identity.start_login_flow(bytea,bytea,text,text,text),identity.consume_login_flow(bytea,bytea,text),
    identity.issue_merchant_session(text,text,bytea,bigint),identity.create_initial_store(bytea,text,bytea,text,text,text,text),
    identity.revoke_merchant_session(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.start_login_flow(bytea,bytea,text,text,text),identity.consume_login_flow(bytea,bytea,text),
    identity.issue_merchant_session(text,text,bytea,bigint),identity.create_initial_store(bytea,text,bytea,text,text,text,text),
    identity.revoke_merchant_session(bytea) TO commerce_identity;
