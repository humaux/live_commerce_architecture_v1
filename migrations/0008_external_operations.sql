-- Internal T06 ledger; no public producer and no provider credentials.
-- River owns job execution state; permanent business dedup lives below.
CREATE ROLE commerce_worker NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE commerce_integration_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE SCHEMA integration;
REVOKE ALL ON SCHEMA integration FROM PUBLIC;
GRANT USAGE ON SCHEMA integration TO commerce_runtime,commerce_worker,commerce_integration_writer;

CREATE TABLE integration.bindings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    principal_id uuid NOT NULL,
    provider text NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'),
    external_asset_id text NOT NULL CHECK (length(external_asset_id) BETWEEN 1 AND 200 AND external_asset_id !~ '[[:cntrl:]]'),
    semantic_version bigint NOT NULL DEFAULT 1 CHECK (semantic_version>0),
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
    FOREIGN KEY (tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);

CREATE TABLE integration.operations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    principal_id uuid NOT NULL,
    binding_id uuid NOT NULL,
    binding_version bigint NOT NULL CHECK (binding_version>0),
    provider text NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'),
    external_asset_id text NOT NULL CHECK (length(external_asset_id) BETWEEN 1 AND 200 AND external_asset_id !~ '[[:cntrl:]]'),
    purpose text NOT NULL CHECK (purpose IN ('transactional','service','marketing')),
    action text NOT NULL CHECK (action ~ '^[a-z][a-z0-9_.:]{0,79}$'),
    semantic_key text NOT NULL CHECK (semantic_key ~ '^[A-Za-z0-9_.:-]{8,128}$'),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
    request jsonb NOT NULL CHECK (jsonb_typeof(request)='object' AND octet_length(request::text)<=65536),
    -- Historical reference only: River may prune completed jobs. No retention FK.
    job_id bigint NOT NULL CHECK (job_id>0),
    state text NOT NULL DEFAULT 'READY' CHECK (state IN ('READY','DISPATCHING','UNKNOWN','ACKNOWLEDGED','SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING')),
    generation bigint NOT NULL DEFAULT 0 CHECK (generation>=0),
    lease_mode text NOT NULL DEFAULT '' CHECK (lease_mode IN ('','dispatch','reconcile')),
    lease_until timestamptz,
    lease_token_hash bytea CHECK (octet_length(lease_token_hash)=32),
    result_code text NOT NULL DEFAULT '' CHECK (result_code ~ '^[A-Za-z0-9_.:-]{0,80}$'),
    provider_reference text NOT NULL DEFAULT '' CHECK (length(provider_reference)<=200 AND provider_reference !~ '[[:cntrl:]]'),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id,store_id,id),
    UNIQUE (tenant_id,store_id,semantic_key),
    FOREIGN KEY (tenant_id,store_id,binding_id) REFERENCES integration.bindings(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id),
    CHECK ((lease_until IS NULL)=(lease_mode='')),
    CHECK ((lease_until IS NULL)=(lease_token_hash IS NULL)),
    CHECK ((state='DISPATCHING' AND lease_mode='dispatch' AND lease_until IS NOT NULL)
        OR (state='UNKNOWN' AND lease_mode IN ('','reconcile'))
        OR (state NOT IN ('DISPATCHING','UNKNOWN') AND lease_mode='' AND lease_until IS NULL)),
    CHECK (generation>0 OR (state='READY' AND lease_until IS NULL))
);
CREATE INDEX operation_recovery ON integration.operations(state,lease_until,updated_at)
    WHERE state IN ('READY','DISPATCHING','UNKNOWN','ACKNOWLEDGED');

CREATE TABLE integration.operation_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    operation_id uuid NOT NULL,
    generation bigint NOT NULL CHECK (generation>=0),
    state text NOT NULL CHECK (state IN ('READY','DISPATCHING','UNKNOWN','ACKNOWLEDGED','SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING')),
    mode text NOT NULL CHECK (mode IN ('','dispatch','reconcile')),
    reason_code text NOT NULL CHECK (reason_code ~ '^[A-Za-z0-9_.:-]{1,80}$'),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    FOREIGN KEY (tenant_id,store_id,operation_id) REFERENCES integration.operations(tenant_id,store_id,id)
);
CREATE INDEX operation_event_history ON integration.operation_events(tenant_id,store_id,operation_id,id);

ALTER TABLE integration.bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration.bindings FORCE ROW LEVEL SECURITY;
ALTER TABLE integration.operations ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration.operations FORCE ROW LEVEL SECURITY;
ALTER TABLE integration.operation_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration.operation_events FORCE ROW LEVEL SECURITY;

CREATE POLICY binding_read ON integration.bindings FOR SELECT TO commerce_runtime
USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY binding_insert ON integration.bindings FOR INSERT TO commerce_runtime
WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY binding_update ON integration.bindings FOR UPDATE TO commerce_runtime
USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY operation_read ON integration.operations FOR SELECT TO commerce_runtime
USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY operation_insert ON integration.operations FOR INSERT TO commerce_runtime
WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid AND state='READY' AND generation=0);
CREATE POLICY event_read ON integration.operation_events FOR SELECT TO commerce_runtime
USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY event_insert ON integration.operation_events FOR INSERT TO commerce_runtime
WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND state='READY' AND generation=0 AND mode='');

-- The dedicated worker sees only integration projections and the queue, not
-- customer identity or transaction tables. It is never an HTTP authority.
CREATE POLICY worker_binding_read ON integration.bindings FOR SELECT TO commerce_worker,commerce_integration_writer USING (true);
CREATE POLICY worker_binding_lock ON integration.bindings FOR UPDATE TO commerce_integration_writer USING (true) WITH CHECK (false);
CREATE POLICY worker_operation_read ON integration.operations FOR SELECT TO commerce_worker,commerce_integration_writer USING (true);
CREATE POLICY worker_operation_update ON integration.operations FOR UPDATE TO commerce_integration_writer USING (true) WITH CHECK (true);
CREATE POLICY worker_event_insert ON integration.operation_events FOR INSERT TO commerce_integration_writer WITH CHECK (true);
CREATE POLICY worker_event_read ON integration.operation_events FOR SELECT TO commerce_worker,commerce_integration_writer USING (true);

GRANT SELECT,INSERT ON integration.bindings,integration.operations,integration.operation_events TO commerce_runtime;
GRANT UPDATE(enabled,semantic_version,updated_at) ON integration.bindings TO commerce_runtime;
GRANT SELECT ON integration.bindings,integration.operations,integration.operation_events TO commerce_worker,commerce_integration_writer;
-- SELECT FOR SHARE needs an UPDATE-column grant; RLS WITH CHECK false forbids mutation.
GRANT UPDATE(id) ON integration.bindings TO commerce_integration_writer;
GRANT UPDATE(state,generation,lease_mode,lease_until,lease_token_hash,result_code,provider_reference,updated_at) ON integration.operations TO commerce_integration_writer;
GRANT INSERT ON integration.operation_events TO commerce_integration_writer;
GRANT USAGE ON SEQUENCE integration.operation_events_id_seq TO commerce_runtime,commerce_integration_writer;

-- Fixed EXECUTE boundary: a worker can operate River, but cannot bypass a
-- business generation/lease predicate with direct UPDATE or fabricate events.
CREATE FUNCTION integration.claim_operation(p_id uuid,p_lease_seconds integer,p_token bytea)
RETURNS TABLE(disposition text,generation bigint,mode text)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_binding_id uuid; b integration.bindings%ROWTYPE; o integration.operations%ROWTYPE;
    v_now timestamptz; v_reason text; v_changed boolean;
BEGIN
    IF p_lease_seconds IS NULL OR p_lease_seconds NOT BETWEEN 5 AND 300 OR p_token IS NULL OR octet_length(p_token)<>32 THEN
        RAISE EXCEPTION 'invalid claim' USING ERRCODE='22023';
    END IF;
    -- Locator is immutable; revalidate after taking binding -> operation locks.
    SELECT x.binding_id INTO v_binding_id FROM integration.operations x WHERE x.id=p_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
    SELECT x.* INTO b FROM integration.bindings x WHERE x.id=v_binding_id FOR SHARE;
    SELECT x.* INTO o FROM integration.operations x WHERE x.id=p_id FOR UPDATE;
    IF NOT FOUND OR o.binding_id<>b.id OR o.tenant_id<>b.tenant_id OR o.store_id<>b.store_id THEN
        RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002';
    END IF;
    v_now:=clock_timestamp();
    IF o.lease_until>v_now THEN RETURN QUERY SELECT 'busy'::text,o.generation,''::text; RETURN; END IF;
    IF o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING') THEN
        RETURN QUERY SELECT 'terminal'::text,o.generation,''::text; RETURN;
    END IF;
    v_changed:=NOT b.enabled OR b.semantic_version<>o.binding_version OR b.provider<>o.provider OR b.external_asset_id<>o.external_asset_id;
    IF v_changed THEN
        -- Only READY proves no dispatch claim ever occurred. Preserve possible
        -- remote side effects as UNKNOWN; never call the replacement binding.
        IF o.state='UNKNOWN' AND o.lease_until IS NULL AND o.result_code='binding_changed' THEN
            RETURN QUERY SELECT 'blocked_binding'::text,o.generation,''::text; RETURN;
        END IF;
        UPDATE integration.operations x SET state=CASE WHEN o.state='READY' THEN 'STALE_BINDING' ELSE 'UNKNOWN' END,
            generation=o.generation+1,lease_mode='',lease_until=NULL,lease_token_hash=NULL,
            result_code='binding_changed',updated_at=v_now WHERE x.id=p_id RETURNING x.* INTO o;
        v_reason:='binding_changed';
        disposition:=CASE WHEN o.state='STALE_BINDING' THEN 'terminal' ELSE 'blocked_binding' END;
    ELSE
        mode:=CASE WHEN o.state='READY' THEN 'dispatch' ELSE 'reconcile' END;
        UPDATE integration.operations x SET state=CASE WHEN mode='dispatch' THEN 'DISPATCHING' ELSE 'UNKNOWN' END,
            generation=o.generation+1,lease_mode=mode,lease_until=v_now+make_interval(secs=>p_lease_seconds),
            lease_token_hash=sha256(p_token),updated_at=v_now WHERE x.id=p_id RETURNING x.* INTO o;
        disposition:='claimed'; v_reason:=CASE WHEN mode='dispatch' THEN 'dispatch_claimed' ELSE 'reconcile_claimed' END;
    END IF;
    INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
        VALUES(o.tenant_id,o.store_id,o.id,o.generation,o.state,o.lease_mode,v_reason);
    generation:=o.generation; mode:=o.lease_mode; RETURN NEXT;
END
$$;
ALTER FUNCTION integration.claim_operation(uuid,integer,bytea) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.claim_operation(uuid,integer,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.claim_operation(uuid,integer,bytea) TO commerce_worker;

CREATE FUNCTION integration.complete_operation(p_id uuid,p_generation bigint,p_token bytea,p_state text,p_code text,p_reference text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_binding_id uuid; b integration.bindings%ROWTYPE; o integration.operations%ROWTYPE;
    v_now timestamptz; v_reason text;
BEGIN
    IF p_token IS NULL OR octet_length(p_token)<>32 OR p_generation IS NULL OR p_generation<1
        OR p_state IS NULL OR p_state NOT IN ('SUCCEEDED','FAILED_FINAL','UNKNOWN','ACKNOWLEDGED')
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
    -- Completion records the frozen action's observed fact. Revoking a binding
    -- cannot erase a payment/message that already happened remotely.
    v_reason:=CASE WHEN NOT b.enabled OR b.semantic_version<>o.binding_version
        OR b.provider<>o.provider OR b.external_asset_id<>o.external_asset_id
        THEN 'completed_binding_changed' ELSE p_code END;
    UPDATE integration.operations SET state=p_state,result_code=p_code,provider_reference=p_reference,
        lease_mode='',lease_until=NULL,lease_token_hash=NULL,updated_at=v_now WHERE id=p_id;
    INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
        VALUES(o.tenant_id,o.store_id,o.id,o.generation,p_state,'',v_reason);
END
$$;
ALTER FUNCTION integration.complete_operation(uuid,bigint,bytea,text,text,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.complete_operation(uuid,bigint,bytea,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.complete_operation(uuid,bigint,bytea,text,text,text) TO commerce_worker;
