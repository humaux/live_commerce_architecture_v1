-- Prepared MOCK media authority only. No provider operation or LIVE registrar.
CREATE ROLE commerce_media_registrar NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE commerce_media_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT USAGE ON SCHEMA live TO commerce_media_registrar, commerce_media_writer;
GRANT USAGE ON SCHEMA control, integration TO commerce_media_writer;

CREATE TABLE live.prepared_media_authorizations (
    id uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid),
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    session_id uuid NOT NULL,
    attempt_id uuid NOT NULL UNIQUE CHECK (attempt_id <> '00000000-0000-0000-0000-000000000000'::uuid),
    media_binding_id uuid NOT NULL,
    environment text NOT NULL CHECK (environment = 'MOCK'),
    project_id text NOT NULL CHECK (project_id ~ '^[A-Za-z0-9_-]{1,80}$'),
    endpoint_identity text NOT NULL CHECK (endpoint_identity ~ '^https://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.livekit\.cloud$'),
    aspect_ratio text NOT NULL CHECK (aspect_ratio IN ('16:9','9:16')),
    evidence_type text NOT NULL CHECK (evidence_type = 'MOCK_FIXTURE'),
    evidence_hash bytea NOT NULL CHECK (octet_length(evidence_hash) = 32),
    evidence_issuer text NOT NULL CHECK (evidence_issuer ~ '^[A-Za-z0-9_-]{1,64}$'),
    start_before timestamptz NOT NULL CHECK (start_before >= TIMESTAMPTZ '2000-01-01 00:00:00+00'
        AND start_before < TIMESTAMPTZ '2200-01-01 00:00:00+00'),
    budget_currency text NOT NULL CHECK (budget_currency ~ '^[A-Z]{3}$'),
    key_id text NOT NULL CHECK (key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    session_version bigint NOT NULL CHECK (session_version > 0),
    credential_version bigint NOT NULL CHECK (credential_version > 0),
    media_binding_version bigint NOT NULL CHECK (media_binding_version > 0),
    material_version bigint NOT NULL CHECK (material_version > 0),
    max_duration_seconds integer NOT NULL CHECK (max_duration_seconds BETWEEN 1 AND 14400),
    budget_minor bigint NOT NULL CHECK (budget_minor BETWEEN 1 AND 1000000000),
    nonce bytea NOT NULL CHECK (octet_length(nonce) = 12),
    ciphertext bytea NOT NULL CHECK (octet_length(ciphertext) BETWEEN 16 AND 16400),
    registered_by text NOT NULL CHECK (length(registered_by) > 0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, store_id, session_id) REFERENCES live.sessions(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, store_id, session_id) REFERENCES live.programs(tenant_id, store_id, session_id),
    FOREIGN KEY (tenant_id, store_id, media_binding_id) REFERENCES integration.bindings(tenant_id, store_id, id)
);

CREATE TABLE live.media_authorization_destinations (
    authorization_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    ordinal smallint NOT NULL CHECK (ordinal BETWEEN 1 AND 2),
    binding_id uuid NOT NULL,
    binding_version bigint NOT NULL CHECK (binding_version > 0),
    provider text NOT NULL CHECK (provider IN ('facebook','instagram')),
    external_asset_id text NOT NULL CHECK (length(external_asset_id) BETWEEN 1 AND 200
        AND external_asset_id = btrim(external_asset_id) AND external_asset_id !~ '[[:cntrl:]]'),
    PRIMARY KEY (authorization_id, ordinal),
    UNIQUE (authorization_id, binding_id),
    UNIQUE (authorization_id, provider, external_asset_id),
    FOREIGN KEY (tenant_id, store_id, authorization_id)
        REFERENCES live.prepared_media_authorizations(tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, store_id, binding_id)
        REFERENCES integration.bindings(tenant_id, store_id, id)
);

CREATE TABLE live.media_authorization_revocations (
    authorization_id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    reason_code text NOT NULL CHECK (reason_code ~ '^[A-Za-z0-9_.:-]{1,80}$'),
    revoked_by text NOT NULL CHECK (length(revoked_by) > 0),
    revoked_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    FOREIGN KEY (tenant_id, store_id, authorization_id)
        REFERENCES live.prepared_media_authorizations(tenant_id, store_id, id)
);

ALTER TABLE live.prepared_media_authorizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.prepared_media_authorizations FORCE ROW LEVEL SECURITY;
ALTER TABLE live.media_authorization_destinations ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.media_authorization_destinations FORCE ROW LEVEL SECURITY;
ALTER TABLE live.media_authorization_revocations ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.media_authorization_revocations FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.prepared_media_authorizations, live.media_authorization_destinations,
    live.media_authorization_revocations FROM PUBLIC;

CREATE POLICY media_header_read ON live.prepared_media_authorizations FOR SELECT
    TO commerce_media_writer USING (true);
CREATE POLICY media_header_insert ON live.prepared_media_authorizations FOR INSERT
    TO commerce_media_writer WITH CHECK (true);
CREATE POLICY media_header_lock ON live.prepared_media_authorizations FOR UPDATE
    TO commerce_media_writer USING (true) WITH CHECK (false);
CREATE POLICY media_destination_read ON live.media_authorization_destinations FOR SELECT
    TO commerce_media_writer USING (true);
CREATE POLICY media_destination_insert ON live.media_authorization_destinations FOR INSERT
    TO commerce_media_writer WITH CHECK (true);
CREATE POLICY media_revocation_read ON live.media_authorization_revocations FOR SELECT
    TO commerce_media_writer USING (true);
CREATE POLICY media_revocation_insert ON live.media_authorization_revocations FOR INSERT
    TO commerce_media_writer WITH CHECK (true);
GRANT SELECT, INSERT ON live.prepared_media_authorizations, live.media_authorization_destinations,
    live.media_authorization_revocations TO commerce_media_writer;
-- The UPDATE-column privilege permits SELECT FOR SHARE/UPDATE; WITH CHECK false forbids mutation.
GRANT UPDATE(id) ON live.prepared_media_authorizations TO commerce_media_writer;

CREATE POLICY media_binding_read ON integration.bindings FOR SELECT
    TO commerce_media_writer USING (true);
CREATE POLICY media_binding_lock ON integration.bindings FOR UPDATE
    TO commerce_media_writer USING (true) WITH CHECK (false);
CREATE POLICY media_store_read ON control.stores FOR SELECT
    TO commerce_media_writer USING (true);
CREATE POLICY media_store_lock ON control.stores FOR UPDATE
    TO commerce_media_writer USING (true) WITH CHECK (false);
CREATE POLICY media_session_read ON live.sessions FOR SELECT
    TO commerce_media_writer USING (true);
CREATE POLICY media_session_lock ON live.sessions FOR UPDATE
    TO commerce_media_writer USING (true) WITH CHECK (false);
CREATE POLICY media_program_read ON live.programs FOR SELECT
    TO commerce_media_writer USING (true);
CREATE POLICY media_program_lock ON live.programs FOR UPDATE
    TO commerce_media_writer USING (true) WITH CHECK (false);
GRANT SELECT ON control.tenants, control.stores, live.sessions, live.programs,
    integration.bindings TO commerce_media_writer;
GRANT UPDATE(id) ON control.tenants, control.stores, live.sessions, live.programs,
    integration.bindings TO commerce_media_writer;

CREATE FUNCTION live.register_prepared_media(p_spec jsonb, p_nonce bytea, p_ciphertext bytea)
RETURNS uuid LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
    v_required text[] := ARRAY['id','tenant_id','store_id','session_id','attempt_id',
        'media_binding_id','environment','project_id','endpoint_identity','aspect_ratio',
        'evidence_type','evidence_hash','evidence_issuer','start_before','budget_currency',
        'key_id','session_version','credential_version','media_binding_version',
        'material_version','max_duration_seconds','budget_minor','destinations'];
    v_strings text[] := ARRAY['id','tenant_id','store_id','session_id','attempt_id',
        'media_binding_id','environment','project_id','endpoint_identity','aspect_ratio',
        'evidence_type','evidence_hash','evidence_issuer','start_before','budget_currency','key_id'];
    v_numbers text[] := ARRAY['session_version','credential_version','media_binding_version',
        'material_version','max_duration_seconds','budget_minor'];
    v_key text;
    v_number numeric;
    v_id uuid; v_tenant uuid; v_store uuid; v_session uuid; v_attempt uuid; v_media uuid;
    v_session_version bigint; v_credential_version bigint; v_media_version bigint;
    v_material_version bigint; v_duration integer; v_budget bigint; v_deadline timestamptz;
    v_hash bytea; v_now timestamptz;
    v_dest jsonb; v_ordinal integer := 0; v_dest_id uuid; v_dest_version bigint;
    v_dest_provider text; v_dest_asset text;
    v_dest_ids uuid[] := ARRAY[]::uuid[]; v_dest_versions bigint[] := ARRAY[]::bigint[];
    v_dest_providers text[] := ARRAY[]::text[]; v_dest_assets text[] := ARRAY[]::text[];
    v_lock_id uuid; v_binding integration.bindings%ROWTYPE;
    v_header live.prepared_media_authorizations%ROWTYPE;
    v_inserted uuid; v_count integer;
BEGIN
    IF p_spec IS NULL OR jsonb_typeof(p_spec) <> 'object'
       OR octet_length(p_spec::text) > 16384 OR p_nonce IS NULL OR octet_length(p_nonce) <> 12
       OR p_ciphertext IS NULL OR octet_length(p_ciphertext) NOT BETWEEN 16 AND 16400 THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;
    SELECT count(*) INTO v_count FROM jsonb_object_keys(p_spec);
    IF v_count <> cardinality(v_required) OR NOT p_spec ?& v_required THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;
    FOREACH v_key IN ARRAY v_strings LOOP
        IF jsonb_typeof(p_spec->v_key) <> 'string' THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
    END LOOP;
    FOREACH v_key IN ARRAY v_numbers LOOP
        IF jsonb_typeof(p_spec->v_key) <> 'number' THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
        v_number := (p_spec->>v_key)::numeric;
        IF v_number <> trunc(v_number) OR v_number < 1 OR v_number > 9223372036854775807 THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
    END LOOP;
    IF jsonb_typeof(p_spec->'destinations') <> 'array'
       OR jsonb_array_length(p_spec->'destinations') NOT BETWEEN 1 AND 2 THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;
    FOREACH v_key IN ARRAY ARRAY['id','tenant_id','store_id','session_id','attempt_id','media_binding_id'] LOOP
        IF (p_spec->>v_key) !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
           OR (p_spec->>v_key) = '00000000-0000-0000-0000-000000000000' THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
    END LOOP;
    v_id := (p_spec->>'id')::uuid; v_tenant := (p_spec->>'tenant_id')::uuid;
    v_store := (p_spec->>'store_id')::uuid; v_session := (p_spec->>'session_id')::uuid;
    v_attempt := (p_spec->>'attempt_id')::uuid; v_media := (p_spec->>'media_binding_id')::uuid;
    IF (p_spec->>'environment') <> 'MOCK'
       OR (p_spec->>'project_id') !~ '^[A-Za-z0-9_-]{1,80}$'
       OR (p_spec->>'endpoint_identity') !~ '^https://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.livekit\.cloud$'
       OR (p_spec->>'aspect_ratio') NOT IN ('16:9','9:16')
       OR (p_spec->>'evidence_type') <> 'MOCK_FIXTURE'
       OR (p_spec->>'evidence_hash') !~ '^[0-9a-f]{64}$'
       OR (p_spec->>'evidence_issuer') !~ '^[A-Za-z0-9_-]{1,64}$'
       OR (p_spec->>'budget_currency') !~ '^[A-Z]{3}$'
       OR (p_spec->>'key_id') !~ '^[A-Za-z0-9_-]{1,64}$' THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;
    v_hash := decode(p_spec->>'evidence_hash','hex');
    IF (p_spec->>'max_duration_seconds')::numeric > 14400
       OR (p_spec->>'budget_minor')::numeric > 1000000000 THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;
    v_session_version := (p_spec->>'session_version')::numeric::bigint;
    v_credential_version := (p_spec->>'credential_version')::numeric::bigint;
    v_media_version := (p_spec->>'media_binding_version')::numeric::bigint;
    v_material_version := (p_spec->>'material_version')::numeric::bigint;
    v_duration := (p_spec->>'max_duration_seconds')::numeric::integer;
    v_budget := (p_spec->>'budget_minor')::numeric::bigint;
    IF (p_spec->>'start_before') !~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](\.[0-9]+)?Z$' THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;
    v_deadline := (p_spec->>'start_before')::timestamptz;
    IF NOT isfinite(v_deadline) THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;

    FOR v_dest IN SELECT value FROM jsonb_array_elements(p_spec->'destinations') LOOP
        v_ordinal := v_ordinal + 1;
        IF jsonb_typeof(v_dest) <> 'object' THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
        SELECT count(*) INTO v_count FROM jsonb_object_keys(v_dest);
        IF v_count <> 4 OR NOT v_dest ?& ARRAY['binding_id','binding_version','provider','external_asset_id']
           OR jsonb_typeof(v_dest->'binding_id') <> 'string'
           OR jsonb_typeof(v_dest->'binding_version') <> 'number'
           OR jsonb_typeof(v_dest->'provider') <> 'string'
           OR jsonb_typeof(v_dest->'external_asset_id') <> 'string' THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
        IF (v_dest->>'binding_id') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
           OR (v_dest->>'binding_id') = '00000000-0000-0000-0000-000000000000' THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
        v_dest_id := (v_dest->>'binding_id')::uuid;
        v_number := (v_dest->>'binding_version')::numeric;
        IF v_number <> trunc(v_number) OR v_number < 1 OR v_number > 9223372036854775807 THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
        v_dest_version := v_number::bigint;
        v_dest_provider := v_dest->>'provider'; v_dest_asset := v_dest->>'external_asset_id';
        IF v_dest_id = v_media OR v_dest_id = ANY(v_dest_ids)
           OR v_dest_provider NOT IN ('facebook','instagram')
           OR length(v_dest_asset) NOT BETWEEN 1 AND 200 OR v_dest_asset <> btrim(v_dest_asset)
           OR v_dest_asset ~ '[[:cntrl:]]'
           OR (v_dest_provider || ':' || v_dest_asset) = ANY(v_dest_providers) THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
        v_dest_ids := array_append(v_dest_ids,v_dest_id);
        v_dest_versions := array_append(v_dest_versions,v_dest_version);
        v_dest_providers := array_append(v_dest_providers,v_dest_provider || ':' || v_dest_asset);
        v_dest_assets := array_append(v_dest_assets,v_dest_asset);
    END LOOP;

    -- Acquire each binding in UUID order before any control/live row lock.
    FOR v_lock_id IN SELECT DISTINCT x FROM unnest(array_prepend(v_media,v_dest_ids)) AS x ORDER BY x LOOP
        SELECT * INTO v_binding FROM integration.bindings WHERE id=v_lock_id FOR SHARE;
        IF NOT FOUND THEN RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023'; END IF;
    END LOOP;
    SELECT * INTO v_binding FROM integration.bindings WHERE id=v_media;
    IF v_binding.tenant_id <> v_tenant OR v_binding.store_id <> v_store
       OR v_binding.provider <> 'livekit' OR v_binding.external_asset_id <> p_spec->>'project_id'
       OR v_binding.semantic_version <> v_media_version OR NOT v_binding.enabled THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;
    FOR v_ordinal IN 1..cardinality(v_dest_ids) LOOP
        SELECT * INTO v_binding FROM integration.bindings WHERE id=v_dest_ids[v_ordinal];
        IF v_binding.tenant_id <> v_tenant OR v_binding.store_id <> v_store
           OR v_binding.provider <> split_part(v_dest_providers[v_ordinal],':',1)
           OR v_binding.external_asset_id <> v_dest_assets[v_ordinal]
           OR v_binding.semantic_version <> v_dest_versions[v_ordinal] OR NOT v_binding.enabled THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
    END LOOP;
    PERFORM 1 FROM control.tenants WHERE id=v_tenant AND active FOR SHARE;
    IF NOT FOUND THEN RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023'; END IF;
    PERFORM 1 FROM control.stores WHERE tenant_id=v_tenant AND id=v_store AND active FOR SHARE;
    IF NOT FOUND THEN RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023'; END IF;
    PERFORM 1 FROM live.sessions WHERE tenant_id=v_tenant AND store_id=v_store AND id=v_session
        AND version=v_session_version FOR SHARE;
    IF NOT FOUND THEN RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023'; END IF;
    PERFORM 1 FROM live.programs WHERE tenant_id=v_tenant AND store_id=v_store AND session_id=v_session
        AND state='DRAFT' AND aspect_ratio=p_spec->>'aspect_ratio' FOR SHARE;
    IF NOT FOUND THEN RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023'; END IF;

    -- ON CONFLICT waits for the competing insert. Replays are checked after that wait.
    INSERT INTO live.prepared_media_authorizations(
        id,tenant_id,store_id,session_id,attempt_id,media_binding_id,environment,project_id,
        endpoint_identity,aspect_ratio,evidence_type,evidence_hash,evidence_issuer,start_before,
        budget_currency,key_id,session_version,credential_version,media_binding_version,
        material_version,max_duration_seconds,budget_minor,nonce,ciphertext,registered_by)
    VALUES(v_id,v_tenant,v_store,v_session,v_attempt,v_media,'MOCK',p_spec->>'project_id',
        p_spec->>'endpoint_identity',p_spec->>'aspect_ratio','MOCK_FIXTURE',v_hash,
        p_spec->>'evidence_issuer',v_deadline,p_spec->>'budget_currency',p_spec->>'key_id',
        v_session_version,v_credential_version,v_media_version,v_material_version,
        v_duration,v_budget,p_nonce,p_ciphertext,session_user)
    ON CONFLICT (id) DO NOTHING RETURNING id INTO v_inserted;

    SELECT * INTO v_header FROM live.prepared_media_authorizations WHERE id=v_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023'; END IF;
    v_now := clock_timestamp();
    IF v_deadline <= v_now OR v_deadline > v_now + interval '24 hours'
       OR EXISTS (SELECT 1 FROM live.media_authorization_revocations WHERE authorization_id=v_id) THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;
    IF v_inserted IS NULL THEN
        IF (v_header.tenant_id,v_header.store_id,v_header.session_id,v_header.attempt_id,
            v_header.media_binding_id,v_header.environment,v_header.project_id,
            v_header.endpoint_identity,v_header.aspect_ratio,v_header.evidence_type,
            v_header.evidence_hash,v_header.evidence_issuer,v_header.start_before,
            v_header.budget_currency,v_header.key_id,v_header.session_version,
            v_header.credential_version,v_header.media_binding_version,v_header.material_version,
            v_header.max_duration_seconds,v_header.budget_minor,v_header.nonce,v_header.ciphertext)
           IS DISTINCT FROM
           (v_tenant,v_store,v_session,v_attempt,v_media,'MOCK',p_spec->>'project_id',
            p_spec->>'endpoint_identity',p_spec->>'aspect_ratio','MOCK_FIXTURE',v_hash,
            p_spec->>'evidence_issuer',v_deadline,p_spec->>'budget_currency',p_spec->>'key_id',
            v_session_version,v_credential_version,v_media_version,v_material_version,
            v_duration,v_budget,p_nonce,p_ciphertext) THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
        SELECT count(*) INTO v_count FROM live.media_authorization_destinations WHERE authorization_id=v_id;
        IF v_count <> cardinality(v_dest_ids) THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
        FOR v_ordinal IN 1..cardinality(v_dest_ids) LOOP
            PERFORM 1 FROM live.media_authorization_destinations
             WHERE authorization_id=v_id AND tenant_id=v_tenant AND store_id=v_store
               AND ordinal=v_ordinal AND binding_id=v_dest_ids[v_ordinal]
               AND binding_version=v_dest_versions[v_ordinal]
               AND provider=split_part(v_dest_providers[v_ordinal],':',1)
               AND external_asset_id=v_dest_assets[v_ordinal];
            IF NOT FOUND THEN RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023'; END IF;
        END LOOP;
    ELSE
        FOR v_ordinal IN 1..cardinality(v_dest_ids) LOOP
            INSERT INTO live.media_authorization_destinations(
                authorization_id,tenant_id,store_id,ordinal,binding_id,binding_version,provider,external_asset_id)
            VALUES(v_id,v_tenant,v_store,v_ordinal,v_dest_ids[v_ordinal],v_dest_versions[v_ordinal],
                split_part(v_dest_providers[v_ordinal],':',1),v_dest_assets[v_ordinal]);
        END LOOP;
    END IF;
    IF v_deadline <= clock_timestamp() THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;
    RETURN v_id;
EXCEPTION
    WHEN data_exception OR integrity_constraint_violation THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
END $$;
ALTER FUNCTION live.register_prepared_media(jsonb,bytea,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.register_prepared_media(jsonb,bytea,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.register_prepared_media(jsonb,bytea,bytea) TO commerce_media_registrar;

CREATE FUNCTION live.revoke_prepared_media(
    p_tenant uuid,p_store uuid,p_authorization uuid,p_reason text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_header live.prepared_media_authorizations%ROWTYPE; v_existing text;
BEGIN
    IF p_tenant IS NULL OR p_store IS NULL OR p_authorization IS NULL
       OR p_tenant='00000000-0000-0000-0000-000000000000'::uuid
       OR p_store='00000000-0000-0000-0000-000000000000'::uuid
       OR p_authorization='00000000-0000-0000-0000-000000000000'::uuid
       OR p_reason IS NULL OR p_reason !~ '^[A-Za-z0-9_.:-]{1,80}$' THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;
    SELECT * INTO v_header FROM live.prepared_media_authorizations
     WHERE id=p_authorization FOR UPDATE;
    IF NOT FOUND OR v_header.tenant_id<>p_tenant OR v_header.store_id<>p_store THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
    END IF;
    SELECT reason_code INTO v_existing FROM live.media_authorization_revocations
     WHERE authorization_id=p_authorization;
    IF FOUND THEN
        IF v_existing<>p_reason THEN
            RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
        END IF;
        RETURN;
    END IF;
    INSERT INTO live.media_authorization_revocations(
        authorization_id,tenant_id,store_id,reason_code,revoked_by,revoked_at)
    VALUES(p_authorization,p_tenant,p_store,p_reason,session_user,clock_timestamp());
EXCEPTION
    WHEN data_exception OR integrity_constraint_violation THEN
        RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='22023';
END $$;
ALTER FUNCTION live.revoke_prepared_media(uuid,uuid,uuid,text) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.revoke_prepared_media(uuid,uuid,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.revoke_prepared_media(uuid,uuid,uuid,text) TO commerce_media_registrar;
