-- BIC is a MOCK-only custody kernel. No token, consumer or provider call is enabled.
ALTER TABLE live.media_attempts DROP CONSTRAINT media_attempts_execution_profile_check;
ALTER TABLE live.media_attempts ADD CONSTRAINT media_attempts_execution_profile_check
 CHECK ((environment='MOCK' AND execution_profile IN ('PROVIDER_MOCK','LOCAL_SFU_MOCK_EGRESS')));

CREATE TABLE live.prepared_media_input_profiles (
 authorization_id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 execution_profile text NOT NULL DEFAULT 'LOCAL_SFU_MOCK_EGRESS'
  CHECK (execution_profile='LOCAL_SFU_MOCK_EGRESS'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE (tenant_id,store_id,authorization_id),
 FOREIGN KEY (tenant_id,store_id,authorization_id)
  REFERENCES live.prepared_media_authorizations(tenant_id,store_id,id)
);
ALTER TABLE live.prepared_media_input_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.prepared_media_input_profiles FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.prepared_media_input_profiles FROM PUBLIC;
CREATE POLICY media_input_profile_read ON live.prepared_media_input_profiles FOR SELECT
 TO commerce_media_writer USING (true);
CREATE POLICY media_input_profile_insert ON live.prepared_media_input_profiles FOR INSERT
 TO commerce_media_writer WITH CHECK (true);
GRANT SELECT,INSERT ON live.prepared_media_input_profiles TO commerce_media_writer;

CREATE TABLE live.media_input_custody (
 attempt_id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, session_id uuid NOT NULL,
 authorization_id uuid NOT NULL, operation_id uuid NOT NULL UNIQUE,
 execution_profile text NOT NULL CHECK (execution_profile='LOCAL_SFU_MOCK_EGRESS'),
 room_name text NOT NULL CHECK (room_name ~ '^lc_[0-9a-f]{32}$'),
 project_id text NOT NULL CHECK (project_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 endpoint_identity text NOT NULL CHECK
  (endpoint_identity ~ '^https://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.livekit\.cloud$'),
 credential_version bigint NOT NULL CHECK (credential_version>0),
 publisher_identity text NOT NULL UNIQUE CHECK (publisher_identity ~ '^lcp_[0-9a-f]{32}$'),
 session_version bigint NOT NULL CHECK (session_version>0),
 created_at timestamptz NOT NULL CHECK (isfinite(created_at)),
 lifetime_deadline timestamptz NOT NULL CHECK (isfinite(lifetime_deadline)),
 grant_iat bigint, grant_exp bigint,
 state text NOT NULL DEFAULT 'UNISSUED'
  CHECK (state IN ('UNISSUED','RESERVED','CLOSING','UNKNOWN','CLOSED')),
 admission_closed_at timestamptz,
 close_reason text NOT NULL DEFAULT '' CHECK (close_reason IN
  ('','merchant_stop','login_lost','permission_lost','authorization_lost',
   'binding_lost','expired','egress_terminal','reconcile_exhausted','runtime_unavailable')),
 escalated_at timestamptz, updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK ((grant_iat IS NULL)=(grant_exp IS NULL)),
 CHECK (grant_iat IS NULL OR (grant_iat>0 AND grant_exp>grant_iat
  AND grant_exp::numeric-grant_iat::numeric<=60)),
 CHECK ((admission_closed_at IS NULL)=(close_reason='')),
 CHECK ((admission_closed_at IS NULL)=(state IN ('UNISSUED','RESERVED'))),
 CHECK (state<>'UNISSUED' OR grant_iat IS NULL),
 CHECK (state NOT IN ('RESERVED','CLOSING','UNKNOWN') OR grant_iat IS NOT NULL),
 CHECK (state<>'CLOSED' OR (admission_closed_at IS NOT NULL AND grant_iat IS NULL)),
 CHECK (lifetime_deadline>created_at),
 UNIQUE (tenant_id,store_id,attempt_id),
 FOREIGN KEY (tenant_id,store_id,attempt_id)
  REFERENCES live.media_attempts(tenant_id,store_id,id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id,store_id,authorization_id)
  REFERENCES live.prepared_media_input_profiles(tenant_id,store_id,authorization_id),
 FOREIGN KEY (tenant_id,store_id,operation_id)
  REFERENCES integration.operations(tenant_id,store_id,id),
 FOREIGN KEY (attempt_id) REFERENCES live.media_login_custody(attempt_id) ON DELETE CASCADE
);
ALTER TABLE live.media_input_custody ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.media_input_custody FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.media_input_custody FROM PUBLIC;
CREATE POLICY media_input_custody_read ON live.media_input_custody FOR SELECT
 TO commerce_media_writer USING (true);
CREATE POLICY media_input_custody_insert ON live.media_input_custody FOR INSERT
 TO commerce_media_writer WITH CHECK (true);
CREATE POLICY media_input_custody_update ON live.media_input_custody FOR UPDATE
 TO commerce_media_writer USING (true) WITH CHECK (true);
GRANT SELECT,INSERT ON live.media_input_custody TO commerce_media_writer;
GRANT UPDATE(grant_iat,grant_exp,state,admission_closed_at,close_reason,
 escalated_at,updated_at)
 ON live.media_input_custody TO commerce_media_writer;

CREATE FUNCTION live.guard_media_input_custody() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.state<>'UNISSUED' OR NEW.grant_iat IS NOT NULL
   OR NEW.admission_closed_at IS NOT NULL OR NEW.close_reason<>'' THEN
   RAISE EXCEPTION 'media input custody unavailable' USING ERRCODE='MP409'; END IF;
  RETURN NEW;
 END IF;
 IF (NEW.attempt_id,NEW.tenant_id,NEW.store_id,NEW.session_id,NEW.authorization_id,
  NEW.operation_id,NEW.execution_profile,NEW.room_name,NEW.project_id,
  NEW.endpoint_identity,NEW.credential_version,NEW.publisher_identity,
  NEW.session_version,NEW.created_at,NEW.lifetime_deadline)
  IS DISTINCT FROM
  (OLD.attempt_id,OLD.tenant_id,OLD.store_id,OLD.session_id,OLD.authorization_id,
   OLD.operation_id,OLD.execution_profile,OLD.room_name,OLD.project_id,
   OLD.endpoint_identity,OLD.credential_version,OLD.publisher_identity,
   OLD.session_version,OLD.created_at,OLD.lifetime_deadline)
  OR (OLD.grant_iat IS NOT NULL AND (NEW.grant_iat,NEW.grant_exp)
   IS DISTINCT FROM (OLD.grant_iat,OLD.grant_exp))
  OR (OLD.admission_closed_at IS NOT NULL AND
   (NEW.admission_closed_at,NEW.close_reason) IS DISTINCT FROM
   (OLD.admission_closed_at,OLD.close_reason))
  OR (OLD.state='CLOSED' AND NEW.state<>'CLOSED')
  OR (OLD.state='UNKNOWN' AND NEW.state<>'UNKNOWN')
  OR (OLD.escalated_at IS NOT NULL AND NEW.escalated_at IS DISTINCT FROM OLD.escalated_at) THEN
  RAISE EXCEPTION 'media input custody unavailable' USING ERRCODE='MP409'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_media_input_custody() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_media_input_custody() FROM PUBLIC;
CREATE TRIGGER media_input_custody_identity BEFORE INSERT OR UPDATE ON live.media_input_custody
 FOR EACH ROW EXECUTE FUNCTION live.guard_media_input_custody();

CREATE FUNCTION live.register_media_input_profile(p_authorization uuid) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE h live.prepared_media_authorizations%ROWTYPE;
BEGIN
 IF p_authorization IS NULL OR p_authorization='00000000-0000-0000-0000-000000000000'::uuid
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media input profile' USING ERRCODE='MP400'; END IF;
 SELECT * INTO h FROM live.prepared_media_authorizations
  WHERE id=p_authorization FOR UPDATE;
 IF h.id IS NULL OR h.environment<>'MOCK' OR h.evidence_type<>'MOCK_FIXTURE'
  OR h.start_before<=clock_timestamp()
  OR EXISTS(SELECT 1 FROM live.media_authorization_revocations r WHERE r.authorization_id=h.id) THEN
  RAISE EXCEPTION 'media input profile unavailable' USING ERRCODE='MP409'; END IF;
 IF EXISTS(SELECT 1 FROM live.prepared_media_input_profiles i WHERE i.authorization_id=h.id) THEN
  RETURN;
 END IF;
 IF EXISTS(SELECT 1 FROM live.media_attempts a WHERE a.authorization_id=h.id) THEN
  RAISE EXCEPTION 'media input profile unavailable' USING ERRCODE='MP409'; END IF;
 INSERT INTO live.prepared_media_input_profiles(authorization_id,tenant_id,store_id)
 VALUES(h.id,h.tenant_id,h.store_id) ON CONFLICT (authorization_id) DO NOTHING;
END $$;
ALTER FUNCTION live.register_media_input_profile(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.register_media_input_profile(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.register_media_input_profile(uuid) TO commerce_media_registrar;

-- Post-River 0009 replaces this fail-closed placeholder after its guards exist.
CREATE FUNCTION live.media_input_plan_ready() RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$ SELECT false $$;
ALTER FUNCTION live.media_input_plan_ready() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_input_plan_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.media_input_plan_ready() TO commerce_runtime,
 commerce_media_executor,commerce_media_worker;


-- Static copies preserve every legacy authorization, binding, clock and receipt gate.
CREATE OR REPLACE FUNCTION live.plan_media_start(p_hash bytea,p_store uuid,p_session uuid,p_authorization uuid,
 p_expected bigint,p_key text,p_operation uuid,p_job bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_access record; v_final record; v_auth live.prepared_media_authorizations%ROWTYPE;
 v_session live.sessions%ROWTYPE; v_program live.programs%ROWTYPE;
 v_binding integration.bindings%ROWTYPE; v_destination live.media_authorization_destinations%ROWTYPE;
 v_lock_id uuid; v_tenant uuid; v_principal uuid; v_revision bigint; v_count integer;
 v_digest bytea; v_now timestamptz; v_token_expiry timestamptz;
 v_login_id uuid; v_login_expiry timestamptz;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_session IS NULL
  OR p_authorization IS NULL OR p_operation IS NULL OR p_expected IS NULL OR p_expected<1
  OR p_store='00000000-0000-0000-0000-000000000000'::uuid
  OR p_session='00000000-0000-0000-0000-000000000000'::uuid
  OR p_authorization='00000000-0000-0000-0000-000000000000'::uuid
  OR p_operation='00000000-0000-0000-0000-000000000000'::uuid
  OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_job IS NULL OR p_job<1 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media start' USING ERRCODE='MP400';
 END IF;
 IF NOT live.media_plan_ready() THEN
  RAISE EXCEPTION 'media queue unavailable' USING ERRCODE='MP409';
 END IF;
 SELECT * INTO v_access FROM identity.resolve_access(p_hash,p_store,'live:manage');
 IF v_access.access_status='unauthorized' THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP401';
 END IF;
 IF v_access.access_status<>'ok' THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403';
 END IF;
 v_tenant:=v_access.tenant_id; v_principal:=v_access.principal_id;
 v_revision:=v_access.authz_revision;
 IF v_tenant IS NULL OR v_principal IS NULL OR v_revision IS NULL OR v_revision<1
  OR v_tenant IS DISTINCT FROM nullif(current_setting('app.tenant_id',true),'')::uuid
  OR p_store IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid
  OR v_principal IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
  OR v_revision IS DISTINCT FROM nullif(current_setting('app.authz_revision',true),'')::bigint THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403';
 END IF;
 -- Immutable locators are read without authority, then checked again under locks.
 SELECT * INTO v_auth FROM live.prepared_media_authorizations WHERE id=p_authorization;
 IF NOT FOUND OR v_auth.tenant_id<>v_tenant OR v_auth.store_id<>p_store
  OR v_auth.session_id<>p_session THEN
  RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='MP404';
 END IF;
 SELECT count(*) INTO v_count FROM live.media_authorization_destinations
  WHERE authorization_id=p_authorization;
 IF v_count NOT BETWEEN 1 AND 2 THEN
  RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='MP409';
 END IF;
 FOR v_lock_id IN SELECT DISTINCT id FROM (
  SELECT v_auth.media_binding_id AS id
  UNION ALL SELECT binding_id FROM live.media_authorization_destinations
   WHERE authorization_id=p_authorization) ids ORDER BY id LOOP
  PERFORM 1 FROM integration.bindings WHERE id=v_lock_id FOR SHARE;
  IF NOT FOUND THEN RAISE EXCEPTION 'media binding unavailable' USING ERRCODE='MP409'; END IF;
 END LOOP;
 SELECT * INTO v_binding FROM integration.bindings WHERE id=v_auth.media_binding_id;
 IF v_binding.tenant_id<>v_tenant OR v_binding.store_id<>p_store OR NOT v_binding.enabled
  OR v_binding.provider<>'livekit' OR v_binding.external_asset_id<>v_auth.project_id
  OR v_binding.semantic_version<>v_auth.media_binding_version THEN
  RAISE EXCEPTION 'media binding unavailable' USING ERRCODE='MP409';
 END IF;
 FOR v_destination IN SELECT * FROM live.media_authorization_destinations
  WHERE authorization_id=p_authorization ORDER BY ordinal LOOP
  SELECT * INTO v_binding FROM integration.bindings WHERE id=v_destination.binding_id;
  IF v_destination.tenant_id<>v_tenant OR v_destination.store_id<>p_store
   OR v_binding.tenant_id<>v_tenant OR v_binding.store_id<>p_store OR NOT v_binding.enabled
   OR v_binding.provider<>v_destination.provider
   OR v_binding.external_asset_id<>v_destination.external_asset_id
   OR v_binding.semantic_version<>v_destination.binding_version THEN
   RAISE EXCEPTION 'media destination unavailable' USING ERRCODE='MP409';
  END IF;
 END LOOP;
 PERFORM 1 FROM control.tenants WHERE id=v_tenant AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='MP403'; END IF;
 PERFORM 1 FROM control.stores WHERE tenant_id=v_tenant AND id=p_store AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='MP403'; END IF;
 SELECT * INTO v_session FROM live.sessions WHERE tenant_id=v_tenant AND store_id=p_store
  AND id=p_session FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media session unavailable' USING ERRCODE='MP404'; END IF;
 SELECT * INTO v_program FROM live.programs WHERE tenant_id=v_tenant AND store_id=p_store
  AND session_id=p_session FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media program unavailable' USING ERRCODE='MP404'; END IF;
 SELECT * INTO v_auth FROM live.prepared_media_authorizations WHERE id=p_authorization FOR SHARE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='MP409'; END IF;
 -- The identity-owned helper waits after the established media lock order.
 SELECT login_session_id,login_expires_at INTO v_login_id,v_login_expiry
  FROM identity.lock_media_login(p_hash,v_principal);
 v_now:=clock_timestamp();
 IF v_auth.tenant_id<>v_tenant OR v_auth.store_id<>p_store
  OR v_auth.session_id<>p_session OR v_auth.environment<>'MOCK'
  OR v_auth.evidence_type<>'MOCK_FIXTURE' OR v_auth.start_before<=v_now
  OR v_login_expiry<=v_now
  OR v_auth.max_duration_seconds NOT BETWEEN 1 AND 14400
  OR v_auth.budget_minor NOT BETWEEN 1 AND 1000000000
  OR v_session.version<>p_expected OR v_auth.session_version<>p_expected
  OR v_program.state<>'DRAFT' OR v_program.aspect_ratio<>v_auth.aspect_ratio
  OR EXISTS(SELECT 1 FROM live.media_authorization_revocations
   WHERE authorization_id=p_authorization)
  OR EXISTS(SELECT 1 FROM live.prepared_media_input_profiles WHERE authorization_id=p_authorization)
  OR EXISTS(SELECT 1 FROM live.media_attempts WHERE tenant_id=v_tenant
   AND store_id=p_store AND session_id=p_session) THEN
  RAISE EXCEPTION 'media start unavailable' USING ERRCODE='MP409';
 END IF;
 v_digest:=sha256(convert_to(jsonb_build_object('principal_id',v_principal::text,
  'session_id',p_session::text,'authorization_id',p_authorization::text,
  'expected_session_version',p_expected)::text,'UTF8'));
 IF NOT EXISTS(SELECT 1 FROM river_media.river_job j WHERE j.id=p_job
  AND j.kind='live_media_operation_v1' AND j.queue='media_mock_v1'
  AND j.state='available' AND j.attempt=0 AND j.finalized_at IS NULL
  AND j.scheduled_at<=clock_timestamp()
  AND j.unique_key IS NULL AND j.args->>'version'='1'
  AND j.args=jsonb_build_object('operation_id',p_operation::text,'version',1)) THEN
  RAISE EXCEPTION 'media job unavailable' USING ERRCODE='MP409';
 END IF;
 INSERT INTO live.media_attempts(id,tenant_id,store_id,session_id,program_id,authorization_id,
  original_principal_id,start_operation_id,start_key,request_hash,environment,execution_profile,room_name)
 VALUES(v_auth.attempt_id,v_tenant,p_store,p_session,v_program.id,p_authorization,
  v_principal,p_operation,p_key,v_digest,'MOCK','PROVIDER_MOCK',
  'lc_'||replace(v_auth.attempt_id::text,'-',''));
 INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,
  provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id,
  state,generation,actor_kind,media_attempt_id)
 VALUES(p_operation,v_tenant,p_store,v_principal,v_auth.media_binding_id,v_auth.media_binding_version,
  'livekit',v_auth.project_id,'service','livekit.egress.start',
  'livekit.start:'||v_auth.attempt_id::text,v_digest,
  jsonb_build_object('attempt_id',v_auth.attempt_id::text,'session_id',p_session::text,'version',1),
  p_job,'READY',0,'MEDIA_ATTEMPT',v_auth.attempt_id);
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(v_tenant,p_store,p_operation,0,'READY','','media_start_planned');
 INSERT INTO live.media_login_custody(attempt_id,tenant_id,store_id,login_session_id,
  authz_revision,login_expires_at)
 VALUES(v_auth.attempt_id,v_tenant,p_store,v_login_id,v_revision,v_login_expiry);
 UPDATE live.programs SET state='READY' WHERE tenant_id=v_tenant AND store_id=p_store
  AND session_id=p_session AND id=v_program.id AND state='DRAFT';
 -- This is a new inner query after all row/event waits, not a cached access record.
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'live:manage');
 SELECT expires_at INTO v_token_expiry FROM identity.sessions
  WHERE token_hash=p_hash AND audience='merchant' AND revoked_at IS NULL;
 v_now:=clock_timestamp();
 IF v_final.access_status='unauthorized' OR v_token_expiry IS NULL OR v_token_expiry<=v_now THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP401';
 END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM v_tenant
  OR v_final.principal_id IS DISTINCT FROM v_principal
  OR v_final.authz_revision IS DISTINCT FROM v_revision
  OR v_auth.start_before<=v_now THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403';
 END IF;
 PERFORM live.assert_media_start_login(p_hash,p_store,v_auth.attempt_id);
 RETURN jsonb_build_object('session_id',p_session::text,'program_id',v_program.id::text,
  'attempt_id',v_auth.attempt_id::text,'operation_id',p_operation::text,'job_id',p_job,
 'room_name','lc_'||replace(v_auth.attempt_id::text,'-',''),'state','READY');
END $$;

CREATE OR REPLACE FUNCTION live.plan_media_input_start(p_hash bytea,p_store uuid,p_session uuid,p_authorization uuid,
 p_expected bigint,p_key text,p_operation uuid,p_job bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_access record; v_final record; v_auth live.prepared_media_authorizations%ROWTYPE;
 v_session live.sessions%ROWTYPE; v_program live.programs%ROWTYPE;
 v_binding integration.bindings%ROWTYPE; v_destination live.media_authorization_destinations%ROWTYPE;
 v_lock_id uuid; v_tenant uuid; v_principal uuid; v_revision bigint; v_count integer;
 v_digest bytea; v_now timestamptz; v_token_expiry timestamptz;
 v_login_id uuid; v_login_expiry timestamptz;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_session IS NULL
  OR p_authorization IS NULL OR p_operation IS NULL OR p_expected IS NULL OR p_expected<1
  OR p_store='00000000-0000-0000-0000-000000000000'::uuid
  OR p_session='00000000-0000-0000-0000-000000000000'::uuid
  OR p_authorization='00000000-0000-0000-0000-000000000000'::uuid
  OR p_operation='00000000-0000-0000-0000-000000000000'::uuid
  OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_job IS NULL OR p_job<1 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media start' USING ERRCODE='MP400';
 END IF;
 IF NOT live.media_input_plan_ready() THEN
  RAISE EXCEPTION 'media queue unavailable' USING ERRCODE='MP409';
 END IF;
 SELECT * INTO v_access FROM identity.resolve_access(p_hash,p_store,'live:manage');
 IF v_access.access_status='unauthorized' THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP401';
 END IF;
 IF v_access.access_status<>'ok' THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403';
 END IF;
 v_tenant:=v_access.tenant_id; v_principal:=v_access.principal_id;
 v_revision:=v_access.authz_revision;
 IF v_tenant IS NULL OR v_principal IS NULL OR v_revision IS NULL OR v_revision<1
  OR v_tenant IS DISTINCT FROM nullif(current_setting('app.tenant_id',true),'')::uuid
  OR p_store IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid
  OR v_principal IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
  OR v_revision IS DISTINCT FROM nullif(current_setting('app.authz_revision',true),'')::bigint THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403';
 END IF;
 -- Immutable locators are read without authority, then checked again under locks.
 SELECT * INTO v_auth FROM live.prepared_media_authorizations WHERE id=p_authorization;
 IF NOT FOUND OR v_auth.tenant_id<>v_tenant OR v_auth.store_id<>p_store
  OR v_auth.session_id<>p_session THEN
  RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='MP404';
 END IF;
 SELECT count(*) INTO v_count FROM live.media_authorization_destinations
  WHERE authorization_id=p_authorization;
 IF v_count NOT BETWEEN 1 AND 2 THEN
  RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='MP409';
 END IF;
 FOR v_lock_id IN SELECT DISTINCT id FROM (
  SELECT v_auth.media_binding_id AS id
  UNION ALL SELECT binding_id FROM live.media_authorization_destinations
   WHERE authorization_id=p_authorization) ids ORDER BY id LOOP
  PERFORM 1 FROM integration.bindings WHERE id=v_lock_id FOR SHARE;
  IF NOT FOUND THEN RAISE EXCEPTION 'media binding unavailable' USING ERRCODE='MP409'; END IF;
 END LOOP;
 SELECT * INTO v_binding FROM integration.bindings WHERE id=v_auth.media_binding_id;
 IF v_binding.tenant_id<>v_tenant OR v_binding.store_id<>p_store OR NOT v_binding.enabled
  OR v_binding.provider<>'livekit' OR v_binding.external_asset_id<>v_auth.project_id
  OR v_binding.semantic_version<>v_auth.media_binding_version THEN
  RAISE EXCEPTION 'media binding unavailable' USING ERRCODE='MP409';
 END IF;
 FOR v_destination IN SELECT * FROM live.media_authorization_destinations
  WHERE authorization_id=p_authorization ORDER BY ordinal LOOP
  SELECT * INTO v_binding FROM integration.bindings WHERE id=v_destination.binding_id;
  IF v_destination.tenant_id<>v_tenant OR v_destination.store_id<>p_store
   OR v_binding.tenant_id<>v_tenant OR v_binding.store_id<>p_store OR NOT v_binding.enabled
   OR v_binding.provider<>v_destination.provider
   OR v_binding.external_asset_id<>v_destination.external_asset_id
   OR v_binding.semantic_version<>v_destination.binding_version THEN
   RAISE EXCEPTION 'media destination unavailable' USING ERRCODE='MP409';
  END IF;
 END LOOP;
 PERFORM 1 FROM control.tenants WHERE id=v_tenant AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='MP403'; END IF;
 PERFORM 1 FROM control.stores WHERE tenant_id=v_tenant AND id=p_store AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='MP403'; END IF;
 SELECT * INTO v_session FROM live.sessions WHERE tenant_id=v_tenant AND store_id=p_store
  AND id=p_session FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media session unavailable' USING ERRCODE='MP404'; END IF;
 SELECT * INTO v_program FROM live.programs WHERE tenant_id=v_tenant AND store_id=p_store
  AND session_id=p_session FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media program unavailable' USING ERRCODE='MP404'; END IF;
 SELECT * INTO v_auth FROM live.prepared_media_authorizations WHERE id=p_authorization FOR SHARE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'media authorization unavailable' USING ERRCODE='MP409'; END IF;
 -- The identity-owned helper waits after the established media lock order.
 SELECT login_session_id,login_expires_at INTO v_login_id,v_login_expiry
  FROM identity.lock_media_login(p_hash,v_principal);
 v_now:=clock_timestamp();
 IF v_auth.tenant_id<>v_tenant OR v_auth.store_id<>p_store
  OR v_auth.session_id<>p_session OR v_auth.environment<>'MOCK'
  OR v_auth.evidence_type<>'MOCK_FIXTURE' OR v_auth.start_before<=v_now
  OR v_login_expiry<=v_now
  OR v_auth.max_duration_seconds NOT BETWEEN 1 AND 14400
  OR v_auth.budget_minor NOT BETWEEN 1 AND 1000000000
  OR v_session.version<>p_expected OR v_auth.session_version<>p_expected
  OR v_program.state<>'DRAFT' OR v_program.aspect_ratio<>v_auth.aspect_ratio
  OR EXISTS(SELECT 1 FROM live.media_authorization_revocations
   WHERE authorization_id=p_authorization)
  OR NOT EXISTS(SELECT 1 FROM live.prepared_media_input_profiles WHERE authorization_id=p_authorization)
  OR EXISTS(SELECT 1 FROM live.media_attempts WHERE tenant_id=v_tenant
   AND store_id=p_store AND session_id=p_session) THEN
  RAISE EXCEPTION 'media start unavailable' USING ERRCODE='MP409';
 END IF;
 v_digest:=sha256(convert_to(jsonb_build_object('principal_id',v_principal::text,
  'session_id',p_session::text,'authorization_id',p_authorization::text,
  'expected_session_version',p_expected)::text,'UTF8'));
 IF NOT EXISTS(SELECT 1 FROM river_media.river_job j WHERE j.id=p_job
  AND j.kind='live_media_operation_v1' AND j.queue='media_input_mock_v1'
  AND j.state='available' AND j.attempt=0 AND j.finalized_at IS NULL
  AND j.scheduled_at<=clock_timestamp()
  AND j.unique_key IS NULL AND j.args->>'version'='1'
  AND j.args=jsonb_build_object('operation_id',p_operation::text,'version',1)) THEN
  RAISE EXCEPTION 'media job unavailable' USING ERRCODE='MP409';
 END IF;
 INSERT INTO live.media_attempts(id,tenant_id,store_id,session_id,program_id,authorization_id,
  original_principal_id,start_operation_id,start_key,request_hash,environment,execution_profile,room_name)
 VALUES(v_auth.attempt_id,v_tenant,p_store,p_session,v_program.id,p_authorization,
  v_principal,p_operation,p_key,v_digest,'MOCK','LOCAL_SFU_MOCK_EGRESS',
  'lc_'||replace(v_auth.attempt_id::text,'-',''));
 INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,
  provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id,
  state,generation,actor_kind,media_attempt_id)
 VALUES(p_operation,v_tenant,p_store,v_principal,v_auth.media_binding_id,v_auth.media_binding_version,
  'livekit',v_auth.project_id,'service','livekit.egress.start',
  'livekit.start:'||v_auth.attempt_id::text,v_digest,
  jsonb_build_object('attempt_id',v_auth.attempt_id::text,'session_id',p_session::text,'version',1),
  p_job,'READY',0,'MEDIA_ATTEMPT',v_auth.attempt_id);
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(v_tenant,p_store,p_operation,0,'READY','','media_start_planned');
 INSERT INTO live.media_login_custody(attempt_id,tenant_id,store_id,login_session_id,
  authz_revision,login_expires_at)
 VALUES(v_auth.attempt_id,v_tenant,p_store,v_login_id,v_revision,v_login_expiry);
 INSERT INTO live.media_input_custody(attempt_id,tenant_id,store_id,session_id,
  authorization_id,operation_id,execution_profile,room_name,project_id,
  endpoint_identity,credential_version,publisher_identity,session_version,created_at,lifetime_deadline)
 SELECT a.id,a.tenant_id,a.store_id,a.session_id,a.authorization_id,p_operation,
  'LOCAL_SFU_MOCK_EGRESS',a.room_name,v_auth.project_id,v_auth.endpoint_identity,
  v_auth.credential_version,'lcp_'||replace(gen_random_uuid()::text,'-',''),
  v_auth.session_version,a.created_at,
  a.created_at+make_interval(secs=>v_auth.max_duration_seconds)
 FROM live.media_attempts a WHERE a.id=v_auth.attempt_id;
 UPDATE live.programs SET state='READY' WHERE tenant_id=v_tenant AND store_id=p_store
  AND session_id=p_session AND id=v_program.id AND state='DRAFT';
 -- This is a new inner query after all row/event waits, not a cached access record.
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'live:manage');
 SELECT expires_at INTO v_token_expiry FROM identity.sessions
  WHERE token_hash=p_hash AND audience='merchant' AND revoked_at IS NULL;
 v_now:=clock_timestamp();
 IF v_final.access_status='unauthorized' OR v_token_expiry IS NULL OR v_token_expiry<=v_now THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP401';
 END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM v_tenant
  OR v_final.principal_id IS DISTINCT FROM v_principal
  OR v_final.authz_revision IS DISTINCT FROM v_revision
  OR v_auth.start_before<=v_now THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403';
 END IF;
 PERFORM live.assert_media_start_login(p_hash,p_store,v_auth.attempt_id);
 RETURN jsonb_build_object('session_id',p_session::text,'program_id',v_program.id::text,
  'attempt_id',v_auth.attempt_id::text,'operation_id',p_operation::text,'job_id',p_job,
  'room_name','lc_'||replace(v_auth.attempt_id::text,'-',''),'state','READY');
END $$;
ALTER FUNCTION live.plan_media_input_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint)
 OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.plan_media_input_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint)
 FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.plan_media_input_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint)
 TO commerce_runtime;

CREATE OR REPLACE FUNCTION live.lock_media_operation(p_id uuid) RETURNS integration.operations
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 o integration.operations%ROWTYPE; v_binding uuid; v_tenant control.tenants%ROWTYPE;
 v_store control.stores%ROWTYPE; v_session live.sessions%ROWTYPE; v_program live.programs%ROWTYPE;
BEGIN
 SELECT * INTO o FROM integration.operations WHERE id=p_id;
 IF NOT FOUND OR o.actor_kind<>'MEDIA_ATTEMPT' OR o.provider<>'livekit'
  OR o.action<>'livekit.egress.start' OR o.purpose<>'service' THEN
  RAISE EXCEPTION 'media operation unavailable' USING ERRCODE='ME404'; END IF;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 IF a.id IS NULL OR h.id IS NULL OR a.start_operation_id<>p_id
  OR (a.tenant_id,a.store_id,a.session_id,a.authorization_id,a.id)
   IS DISTINCT FROM (h.tenant_id,h.store_id,h.session_id,h.id,h.attempt_id)
  OR o.tenant_id<>a.tenant_id OR o.store_id<>a.store_id THEN
  RAISE EXCEPTION 'media operation unavailable' USING ERRCODE='ME404'; END IF;
 FOR v_binding IN SELECT DISTINCT id FROM (
  SELECT h.media_binding_id AS id UNION ALL
  SELECT d.binding_id FROM live.media_authorization_destinations d WHERE d.authorization_id=h.id
 ) ids ORDER BY id LOOP
  PERFORM 1 FROM integration.bindings WHERE id=v_binding FOR SHARE;
  IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='ME409'; END IF;
 END LOOP;
 SELECT * INTO v_tenant FROM control.tenants WHERE id=a.tenant_id FOR SHARE;
 SELECT * INTO v_store FROM control.stores WHERE tenant_id=a.tenant_id AND id=a.store_id FOR SHARE;
 SELECT * INTO v_session FROM live.sessions WHERE tenant_id=a.tenant_id AND store_id=a.store_id AND id=a.session_id FOR SHARE;
 SELECT * INTO v_program FROM live.programs WHERE tenant_id=a.tenant_id AND store_id=a.store_id
  AND id=a.program_id AND session_id=a.session_id FOR SHARE;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE tenant_id=a.tenant_id
  AND store_id=a.store_id AND id=a.authorization_id FOR SHARE;
 SELECT * INTO a FROM live.media_attempts WHERE tenant_id=h.tenant_id AND store_id=h.store_id
  AND id=h.attempt_id FOR SHARE;
 SELECT * INTO o FROM integration.operations WHERE id=p_id FOR UPDATE;
 IF v_tenant.id IS NULL OR v_store.id IS NULL OR v_session.id IS NULL OR v_program.id IS NULL
  OR h.id IS NULL OR a.id IS NULL OR o.id IS NULL OR o.actor_kind<>'MEDIA_ATTEMPT'
  OR o.media_attempt_id<>a.id OR o.principal_id<>a.original_principal_id
  OR o.binding_id<>h.media_binding_id OR o.binding_version<>h.media_binding_version
  OR o.external_asset_id<>h.project_id OR o.job_id<1 OR a.start_operation_id<>o.id
  OR a.room_name<>'lc_'||replace(a.id::text,'-','') OR a.environment<>'MOCK'
  OR a.execution_profile NOT IN ('PROVIDER_MOCK','LOCAL_SFU_MOCK_EGRESS') OR h.environment<>'MOCK'
  OR (a.execution_profile='LOCAL_SFU_MOCK_EGRESS' AND NOT EXISTS
   (SELECT 1 FROM live.media_input_custody i WHERE i.attempt_id=a.id
    AND i.operation_id=o.id AND i.execution_profile=a.execution_profile
    AND i.tenant_id=a.tenant_id AND i.store_id=a.store_id))
  OR (a.execution_profile='PROVIDER_MOCK' AND EXISTS
   (SELECT 1 FROM live.media_input_custody i WHERE i.attempt_id=a.id))
  OR o.tenant_id<>a.tenant_id OR o.store_id<>a.store_id OR a.session_id<>h.session_id
  OR a.authorization_id<>h.id OR o.request_hash<>a.request_hash
  OR o.request IS DISTINCT FROM jsonb_build_object('attempt_id',a.id::text,
   'session_id',a.session_id::text,'version',1) THEN
  RAISE EXCEPTION 'media operation unavailable' USING ERRCODE='ME409'; END IF;
 RETURN o;
END $$;
ALTER FUNCTION live.lock_media_operation(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.lock_media_operation(uuid) FROM PUBLIC;

-- Caller must already hold the original operation lock. A local issued grant
-- has no qualified revocation proof; closing admission never erases liability.
CREATE FUNCTION live.close_media_input_custody(p_attempt uuid,p_reason text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE i live.media_input_custody%ROWTYPE; v_now timestamptz;
BEGIN
 IF p_reason NOT IN ('merchant_stop','login_lost','permission_lost','authorization_lost',
  'binding_lost','expired','egress_terminal','reconcile_exhausted','runtime_unavailable') THEN
  RAISE EXCEPTION 'media input close unavailable' USING ERRCODE='ME400'; END IF;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=p_attempt FOR UPDATE;
 IF i.attempt_id IS NULL THEN RETURN true; END IF;
 IF i.state='CLOSED' THEN RETURN true; END IF;
 v_now:=clock_timestamp();
 IF i.grant_iat IS NULL THEN
  UPDATE live.media_input_custody SET state='CLOSED',
   admission_closed_at=coalesce(i.admission_closed_at,v_now),
   close_reason=CASE WHEN i.admission_closed_at IS NULL THEN p_reason ELSE i.close_reason END,
   updated_at=v_now WHERE attempt_id=i.attempt_id;
  RETURN true;
 END IF;
 UPDATE live.media_input_custody SET state='UNKNOWN',
  admission_closed_at=coalesce(i.admission_closed_at,v_now),
  close_reason=CASE WHEN i.admission_closed_at IS NULL THEN p_reason ELSE i.close_reason END,
  updated_at=v_now WHERE attempt_id=i.attempt_id;
 RETURN false;
END $$;
ALTER FUNCTION live.close_media_input_custody(uuid,text) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.close_media_input_custody(uuid,text) FROM PUBLIC;

CREATE FUNCTION live.reserve_media_input(p_hash bytea,p_store uuid,p_session uuid,
 p_attempt uuid,p_expected bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_access record; v_final record; a live.media_attempts%ROWTYPE;
 h live.prepared_media_authorizations%ROWTYPE; o integration.operations%ROWTYPE;
 x live.media_execution_state%ROWTYPE; i live.media_input_custody%ROWTYPE;
 c live.media_login_custody%ROWTYPE; v_login uuid; v_login_expiry timestamptz;
 v_binding uuid; v_now timestamptz; v_iat bigint; v_exp bigint; v_limit timestamptz;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_session IS NULL
  OR p_attempt IS NULL OR p_expected IS NULL OR p_expected<1
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media input request' USING ERRCODE='MP400'; END IF;
 SELECT * INTO v_access FROM identity.resolve_access(p_hash,p_store,'live:manage');
 IF v_access.access_status='unauthorized' THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP401'; END IF;
 IF v_access.access_status<>'ok' OR v_access.tenant_id IS DISTINCT FROM
  nullif(current_setting('app.tenant_id',true),'')::uuid
  OR p_store IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid
  OR v_access.principal_id IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
  OR v_access.authz_revision IS DISTINCT FROM nullif(current_setting('app.authz_revision',true),'')::bigint THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403'; END IF;
 SELECT * INTO a FROM live.media_attempts WHERE id=p_attempt;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 IF a.id IS NULL OR h.id IS NULL OR a.tenant_id<>v_access.tenant_id
  OR a.store_id<>p_store OR a.session_id<>p_session
  OR a.original_principal_id<>v_access.principal_id OR a.execution_profile<>'LOCAL_SFU_MOCK_EGRESS'
  OR a.environment<>'MOCK' OR h.attempt_id<>a.id THEN
  RAISE EXCEPTION 'media input unavailable' USING ERRCODE='MP404'; END IF;
 FOR v_binding IN SELECT DISTINCT id FROM (
  SELECT h.media_binding_id AS id UNION ALL
  SELECT d.binding_id FROM live.media_authorization_destinations d WHERE d.authorization_id=h.id
 ) ids ORDER BY id LOOP
  PERFORM 1 FROM integration.bindings WHERE id=v_binding FOR SHARE;
  IF NOT FOUND THEN RAISE EXCEPTION 'media binding unavailable' USING ERRCODE='MP409'; END IF;
 END LOOP;
 PERFORM 1 FROM control.tenants WHERE id=a.tenant_id AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='MP403'; END IF;
 PERFORM 1 FROM control.stores WHERE tenant_id=a.tenant_id AND id=a.store_id AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='MP403'; END IF;
 PERFORM 1 FROM live.sessions WHERE tenant_id=a.tenant_id AND store_id=a.store_id
  AND id=a.session_id FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media session unavailable' USING ERRCODE='MP404'; END IF;
 PERFORM 1 FROM live.programs WHERE tenant_id=a.tenant_id AND store_id=a.store_id
  AND id=a.program_id AND session_id=a.session_id FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media program unavailable' USING ERRCODE='MP404'; END IF;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=h.id FOR SHARE;
 SELECT * INTO c FROM live.media_login_custody WHERE attempt_id=a.id;
 IF c.attempt_id IS NULL OR c.tenant_id<>a.tenant_id OR c.store_id<>a.store_id
  OR c.authz_revision<>v_access.authz_revision OR NOT isfinite(c.login_expires_at) THEN
  RAISE EXCEPTION 'media login custody unavailable' USING ERRCODE='MP409'; END IF;
 SELECT login_session_id,login_expires_at INTO v_login,v_login_expiry
  FROM identity.lock_media_login(p_hash,v_access.principal_id);
 IF v_login IS DISTINCT FROM c.login_session_id THEN
  RAISE EXCEPTION 'media login unavailable' USING ERRCODE='MP403'; END IF;
 SELECT * INTO a FROM live.media_attempts WHERE id=p_attempt FOR SHARE;
 SELECT * INTO o FROM integration.operations WHERE id=a.start_operation_id FOR UPDATE;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=a.id FOR UPDATE;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id FOR UPDATE;
 v_now:=clock_timestamp();
 IF o.id IS NULL OR i.attempt_id IS NULL OR i.operation_id<>o.id
  OR i.tenant_id<>a.tenant_id OR i.store_id<>a.store_id OR i.session_id<>a.session_id
  OR i.authorization_id<>h.id OR i.session_version<>p_expected
  OR a.start_operation_id<>o.id OR a.execution_profile<>'LOCAL_SFU_MOCK_EGRESS'
  OR a.session_id<>p_session OR o.job_id<1
  OR (i.grant_iat IS NULL AND (o.state<>'READY' OR o.generation<>0
   OR o.lease_until IS NOT NULL OR o.lease_token_hash IS NOT NULL))
  OR (i.grant_iat IS NOT NULL AND (o.state NOT IN ('READY','UNKNOWN')
   OR o.lease_mode NOT IN ('','reconcile')))
  OR x.wire_reserved_at IS NOT NULL OR x.stop_requested_at IS NOT NULL
  OR x.escalated_at IS NOT NULL OR x.resource_state='TERMINAL'
  OR i.admission_closed_at IS NOT NULL OR i.state NOT IN ('UNISSUED','RESERVED')
  OR h.start_before<=v_now OR i.lifetime_deadline<=v_now
  OR v_login_expiry<=v_now OR c.login_expires_at<=v_now
  OR NOT live.media_native_job(o.job_id,o.id)
  OR NOT live.media_dispatch_eligible(o.id) OR NOT live.media_login_eligible(o.id) THEN
  RAISE EXCEPTION 'media input unavailable' USING ERRCODE='MP409'; END IF;
 IF i.grant_iat IS NULL THEN
  v_iat:=floor(extract(epoch FROM v_now))::bigint;
  v_limit:=least(v_now+interval '60 seconds',h.start_before,i.lifetime_deadline,
   v_login_expiry,c.login_expires_at);
  v_exp:=floor(extract(epoch FROM v_limit))::bigint;
  IF v_exp<=v_iat OR to_timestamp(v_exp)<=clock_timestamp() THEN
   RAISE EXCEPTION 'media input expired' USING ERRCODE='MP409'; END IF;
  UPDATE live.media_input_custody SET grant_iat=v_iat,grant_exp=v_exp,state='RESERVED',
   updated_at=clock_timestamp() WHERE attempt_id=a.id;
 ELSE
  v_iat:=i.grant_iat; v_exp:=i.grant_exp;
  IF to_timestamp(v_exp)<=v_now THEN
   RAISE EXCEPTION 'media input expired' USING ERRCODE='MP409'; END IF;
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'live:manage');
 IF v_final.access_status='unauthorized' THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP401'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM a.tenant_id
  OR v_final.principal_id IS DISTINCT FROM a.original_principal_id
  OR v_final.authz_revision IS DISTINCT FROM c.authz_revision
  OR h.start_before<=clock_timestamp() OR i.lifetime_deadline<=clock_timestamp()
  OR v_login_expiry<=clock_timestamp() OR c.login_expires_at<=clock_timestamp()
  OR to_timestamp(v_exp)<=clock_timestamp()
  OR NOT live.media_dispatch_eligible(o.id) OR NOT live.media_login_eligible(o.id) THEN
  RAISE EXCEPTION 'media input unavailable' USING ERRCODE='MP403'; END IF;
 RETURN jsonb_build_object('attempt_id',a.id::text,'room_name',i.room_name,
  'publisher_identity',i.publisher_identity,'project_id',i.project_id,
  'endpoint_identity',i.endpoint_identity,'credential_version',i.credential_version,
  'issued_at',v_iat,'expires_at',v_exp);
END $$;
ALTER FUNCTION live.reserve_media_input(bytea,uuid,uuid,uuid,bigint) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.reserve_media_input(bytea,uuid,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.reserve_media_input(bytea,uuid,uuid,uuid,bigint) TO commerce_runtime;

CREATE FUNCTION live.claim_media_input_operation(p_id uuid,p_job bigint,p_lease_seconds integer,p_token bytea)
RETURNS TABLE(disposition text,generation bigint,mode text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 i live.media_input_custody%ROWTYPE; v_now timestamptz; v_closed boolean;
BEGIN
 IF p_id IS NULL OR p_job IS NULL OR p_job<1 OR p_lease_seconds IS NULL
  OR p_lease_seconds NOT BETWEEN 1 AND 30 OR p_token IS NULL OR octet_length(p_token)<>32
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media input claim' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 IF a.execution_profile<>'LOCAL_SFU_MOCK_EGRESS' OR p_job<>o.job_id
  OR NOT live.media_native_job(p_job,o.id) THEN
  RAISE EXCEPTION 'media input job unavailable' USING ERRCODE='ME409'; END IF;
 INSERT INTO live.media_execution_state(attempt_id,tenant_id,store_id,session_id,
  authorization_id,operation_id,project_id)
 VALUES(a.id,a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id)
 ON CONFLICT (attempt_id) DO NOTHING;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=a.id FOR UPDATE;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id FOR UPDATE;
 IF x.attempt_id IS NULL OR i.attempt_id IS NULL OR x.operation_id<>o.id
  OR i.operation_id<>o.id OR x.project_id<>h.project_id THEN
  RAISE EXCEPTION 'media input projection unavailable' USING ERRCODE='ME409'; END IF;
 v_now:=clock_timestamp();
 IF i.state='CLOSED' AND (x.resource_state='TERMINAL'
  OR (x.wire_reserved_at IS NULL AND (x.stop_requested_at IS NOT NULL
   OR i.admission_closed_at IS NOT NULL))) THEN
  RETURN QUERY SELECT 'terminal'::text,o.generation,''::text; RETURN; END IF;
 IF o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING') THEN
  RAISE EXCEPTION 'media input terminal mismatch' USING ERRCODE='ME409'; END IF;
 IF o.lease_until>v_now THEN
  RETURN QUERY SELECT 'busy'::text,o.generation,''::text; RETURN; END IF;
 IF o.generation>=4096 OR v_now>=o.created_at+interval '24 hours' THEN
  v_closed:=live.close_media_input_custody(a.id,'reconcile_exhausted');
  UPDATE live.media_execution_state SET escalated_at=coalesce(escalated_at,v_now),
   escalation_code='reconcile_exhausted',updated_at=v_now WHERE attempt_id=a.id;
  UPDATE integration.operations SET state='UNKNOWN',generation=greatest(o.generation,1),
   lease_mode='',lease_until=NULL,lease_token_hash=NULL,result_code='reconcile_exhausted',updated_at=v_now
   WHERE id=o.id;
  INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
   VALUES(o.tenant_id,o.store_id,o.id,greatest(o.generation,1),'UNKNOWN','','media_reconcile_exhausted');
  RETURN QUERY SELECT 'held'::text,greatest(o.generation,1),''::text; RETURN;
 END IF;
 IF i.admission_closed_at IS NULL AND (v_now>=i.lifetime_deadline OR v_now>=h.start_before) THEN
  PERFORM live.close_media_input_custody(a.id,'expired');
 ELSIF i.admission_closed_at IS NULL AND (NOT live.media_login_eligible(o.id)
  OR NOT live.media_dispatch_eligible(o.id)) THEN
  PERFORM live.close_media_input_custody(a.id,'permission_lost');
 END IF;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id;
 IF i.state='CLOSED' AND x.wire_reserved_at IS NULL THEN
  UPDATE integration.operations SET state='BLOCKED_POLICY',generation=o.generation+1,
   lease_mode='',lease_until=NULL,lease_token_hash=NULL,result_code='policy_denied',updated_at=clock_timestamp()
   WHERE id=o.id;
  INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
   VALUES(o.tenant_id,o.store_id,o.id,o.generation+1,'BLOCKED_POLICY','','media_policy_denied');
  RETURN QUERY SELECT 'terminal'::text,o.generation+1,''::text; RETURN; END IF;
 IF i.state='UNISSUED' AND x.stop_requested_at IS NULL THEN
  RETURN QUERY SELECT 'await_admission'::text,o.generation,''::text; RETURN; END IF;
 v_now:=clock_timestamp();
 UPDATE integration.operations SET state='UNKNOWN',generation=o.generation+1,
  lease_mode='reconcile',lease_until=v_now+make_interval(secs=>p_lease_seconds),
  lease_token_hash=sha256(p_token),updated_at=v_now WHERE id=o.id;
 UPDATE live.media_execution_state SET claim_started_at=v_now,updated_at=v_now WHERE attempt_id=a.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,o.generation+1,'UNKNOWN','reconcile','media_reconcile_claimed');
 IF v_now+make_interval(secs=>p_lease_seconds)<=clock_timestamp() THEN
  RAISE EXCEPTION 'media input lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN QUERY SELECT 'claimed'::text,o.generation+1,'reconcile'::text;
END $$;
ALTER FUNCTION live.claim_media_input_operation(uuid,bigint,integer,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.claim_media_input_operation(uuid,bigint,integer,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.claim_media_input_operation(uuid,bigint,integer,bytea)
 TO commerce_media_executor;

CREATE FUNCTION live.load_media_input_custody(p_id uuid,p_generation bigint,p_token bytea)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 i live.media_input_custody%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL
  OR octet_length(p_token)<>32 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media input load' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=i.authorization_id;
 IF i.attempt_id IS NULL OR i.operation_id<>o.id OR x.operation_id<>o.id
  OR o.job_id<1 OR NOT live.media_native_job(o.job_id,o.id)
  OR o.state<>'UNKNOWN' OR o.lease_mode<>'reconcile' OR o.generation<>p_generation
  OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN
  RAISE EXCEPTION 'media input lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN jsonb_build_object('attempt_id',i.attempt_id::text,'operation_id',o.id::text,
  'session_id',i.session_id::text,'execution_profile',i.execution_profile,
  'state',i.state,'room_name',i.room_name,'publisher_identity',i.publisher_identity,
  'project_id',i.project_id,'endpoint_identity',i.endpoint_identity,
  'credential_version',i.credential_version,'session_version',i.session_version,
  'issued_at',i.grant_iat,'expires_at',i.grant_exp,
  'start_before',floor(extract(epoch FROM h.start_before))::bigint,
  'lifetime_deadline',floor(extract(epoch FROM i.lifetime_deadline))::bigint,
  'admission_closed',i.admission_closed_at IS NOT NULL,'close_reason',i.close_reason,
  'egress_state',x.resource_state,'wire_reserved',x.wire_reserved_at IS NOT NULL,
  'held',i.state='UNKNOWN' OR x.escalated_at IS NOT NULL);
END $$;
ALTER FUNCTION live.load_media_input_custody(uuid,bigint,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.load_media_input_custody(uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.load_media_input_custody(uuid,bigint,bytea) TO commerce_media_executor;

CREATE FUNCTION live.close_media_input_admission(p_id uuid,p_generation bigint,p_token bytea,p_reason text)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 i live.media_input_custody%ROWTYPE; v_now timestamptz; v_done boolean;
 v_state text; v_result text;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL
  OR octet_length(p_token)<>32 OR p_reason IS NULL
  OR p_reason NOT IN ('merchant_stop','login_lost','permission_lost','authorization_lost',
   'binding_lost','expired','egress_terminal','reconcile_exhausted','runtime_unavailable')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media input close' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 IF i.attempt_id IS NULL OR i.operation_id<>o.id OR x.operation_id<>o.id
  OR NOT live.media_native_job(o.job_id,o.id) OR o.state<>'UNKNOWN'
  OR o.lease_mode<>'reconcile' OR o.generation<>p_generation
  OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN
  RAISE EXCEPTION 'media input lease unavailable' USING ERRCODE='ME409'; END IF;
 v_done:=live.close_media_input_custody(i.attempt_id,p_reason);
 v_now:=clock_timestamp();
 IF v_done AND (x.resource_state='TERMINAL' OR x.wire_reserved_at IS NULL) THEN
  v_state:=CASE WHEN x.resource_state='TERMINAL' AND x.transport_status='EGRESS_COMPLETE'
   THEN 'SUCCEEDED' WHEN x.resource_state='TERMINAL' THEN 'FAILED_FINAL'
   WHEN p_reason='merchant_stop' THEN 'CANCELLED' ELSE 'BLOCKED_POLICY' END;
  v_result:='terminal';
 ELSE
  v_state:='UNKNOWN';
  v_result:=CASE WHEN v_done THEN 'observe' ELSE 'held' END;
 END IF;
 UPDATE integration.operations SET state=v_state,lease_mode='',lease_until=NULL,
  lease_token_hash=NULL,result_code=CASE WHEN v_result='terminal' THEN 'media_input_terminal'
   ELSE 'media_input_held' END,updated_at=v_now WHERE id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,p_generation,v_state,'',
   CASE WHEN v_result='terminal' THEN 'media_input_terminal' ELSE 'media_input_held' END);
 IF o.lease_until<=clock_timestamp() THEN
  RAISE EXCEPTION 'media input lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION live.close_media_input_admission(uuid,bigint,bytea,text) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.close_media_input_admission(uuid,bigint,bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.close_media_input_admission(uuid,bigint,bytea,text)
 TO commerce_media_executor;

CREATE OR REPLACE FUNCTION live.request_media_stop(p_auth_hash bytea,p_store uuid,p_session uuid,p_attempt uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_access record; v_final record; v_tenant uuid; v_principal uuid; v_revision bigint;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 v_binding uuid; v_now timestamptz; v_expiry timestamptz; v_state text; v_input_done boolean;
BEGIN
 IF p_auth_hash IS NULL OR octet_length(p_auth_hash)<>32 OR p_store IS NULL OR p_session IS NULL
  OR p_attempt IS NULL OR p_store='00000000-0000-0000-0000-000000000000'::uuid
  OR p_session='00000000-0000-0000-0000-000000000000'::uuid
  OR p_attempt='00000000-0000-0000-0000-000000000000'::uuid
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media stop request' USING ERRCODE='MP400'; END IF;
 SELECT * INTO v_access FROM identity.resolve_access(p_auth_hash,p_store,'live:manage');
 IF v_access.access_status='unauthorized' THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP401'; END IF;
 IF v_access.access_status<>'ok' THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403'; END IF;
 v_tenant:=v_access.tenant_id; v_principal:=v_access.principal_id;
 v_revision:=v_access.authz_revision;
 IF v_tenant IS NULL OR v_principal IS NULL OR v_revision IS NULL OR v_revision<1
  OR v_tenant IS DISTINCT FROM nullif(current_setting('app.tenant_id',true),'')::uuid
  OR p_store IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid
  OR v_principal IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
  OR v_revision IS DISTINCT FROM nullif(current_setting('app.authz_revision',true),'')::bigint THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403'; END IF;
 SELECT * INTO a FROM live.media_attempts WHERE id=p_attempt;
 IF a.id IS NULL OR a.tenant_id<>v_tenant OR a.store_id<>p_store OR a.session_id<>p_session THEN
  RAISE EXCEPTION 'media attempt unavailable' USING ERRCODE='MP404'; END IF;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 IF h.id IS NULL OR h.attempt_id<>a.id OR h.environment<>'MOCK'
  OR a.environment<>'MOCK' OR a.execution_profile NOT IN ('PROVIDER_MOCK','LOCAL_SFU_MOCK_EGRESS') THEN
  RAISE EXCEPTION 'media attempt unavailable' USING ERRCODE='MP409'; END IF;
 FOR v_binding IN SELECT DISTINCT id FROM (
  SELECT h.media_binding_id AS id UNION ALL
  SELECT d.binding_id FROM live.media_authorization_destinations d WHERE d.authorization_id=h.id
 ) ids ORDER BY id LOOP
  PERFORM 1 FROM integration.bindings WHERE id=v_binding FOR SHARE;
  IF NOT FOUND THEN RAISE EXCEPTION 'media binding unavailable' USING ERRCODE='MP409'; END IF;
 END LOOP;
 PERFORM 1 FROM control.tenants WHERE id=v_tenant AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='MP403'; END IF;
 PERFORM 1 FROM control.stores WHERE tenant_id=v_tenant AND id=p_store AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='MP403'; END IF;
 PERFORM 1 FROM live.sessions WHERE tenant_id=v_tenant AND store_id=p_store AND id=p_session FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media session unavailable' USING ERRCODE='MP404'; END IF;
 PERFORM 1 FROM live.programs WHERE tenant_id=v_tenant AND store_id=p_store
  AND id=a.program_id AND session_id=p_session FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media program unavailable' USING ERRCODE='MP404'; END IF;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id FOR SHARE;
 SELECT * INTO a FROM live.media_attempts WHERE id=p_attempt FOR SHARE;
 SELECT * INTO o FROM integration.operations WHERE id=a.start_operation_id FOR UPDATE;
 IF h.id IS NULL OR a.id IS NULL OR o.id IS NULL OR a.id<>p_attempt
  OR a.tenant_id<>v_tenant OR a.store_id<>p_store OR a.session_id<>p_session
  OR h.id<>a.authorization_id OR h.attempt_id<>a.id OR o.media_attempt_id<>a.id
  OR o.actor_kind<>'MEDIA_ATTEMPT' OR o.action<>'livekit.egress.start'
  OR o.principal_id<>a.original_principal_id OR o.job_id<1 THEN
  RAISE EXCEPTION 'media stop unavailable' USING ERRCODE='MP409'; END IF;
 INSERT INTO live.media_execution_state(attempt_id,tenant_id,store_id,session_id,
  authorization_id,operation_id,project_id)
 VALUES(a.id,a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id)
 ON CONFLICT (attempt_id) DO NOTHING;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=a.id FOR UPDATE;
 IF x.attempt_id IS NULL OR (x.tenant_id,x.store_id,x.session_id,x.authorization_id,x.operation_id,x.project_id)
  IS DISTINCT FROM (a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id) THEN
  RAISE EXCEPTION 'media stop unavailable' USING ERRCODE='MP409'; END IF;
 v_input_done:=live.close_media_input_custody(a.id,'merchant_stop');
 IF NOT v_input_done AND o.state IN
  ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING') THEN
  RAISE EXCEPTION 'media input terminal mismatch' USING ERRCODE='MP409'; END IF;
 v_now:=clock_timestamp();
 IF x.stop_requested_at IS NOT NULL THEN
  v_state:=CASE WHEN o.state='CANCELLED' AND x.wire_reserved_at IS NULL THEN 'cancelled_before_start'
   WHEN x.escalated_at IS NOT NULL THEN 'escalated'
   WHEN v_input_done AND (x.resource_state='TERMINAL' OR o.state IN ('SUCCEEDED','FAILED_FINAL','BLOCKED_POLICY','STALE_BINDING'))
    THEN 'terminal' ELSE 'requested' END;
 ELSIF x.escalated_at IS NOT NULL THEN v_state:='escalated';
 ELSIF v_input_done AND (x.resource_state='TERMINAL' OR o.state IN
  ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING')) THEN v_state:='terminal';
 ELSE
  UPDATE live.media_execution_state SET stop_requested_at=v_now,stop_requested_by=v_principal,
   cleanup_required=true,updated_at=v_now WHERE attempt_id=a.id;
  v_state:='requested';
  IF x.wire_reserved_at IS NULL THEN
   UPDATE integration.operations SET state=CASE WHEN v_input_done THEN 'CANCELLED' ELSE 'UNKNOWN' END,
    generation=o.generation+1,
    lease_mode='',lease_until=NULL,lease_token_hash=NULL,
    result_code=CASE WHEN v_input_done THEN 'media_cancelled_before_start' ELSE 'media_input_held' END,
    updated_at=v_now
    WHERE id=o.id;
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
    VALUES(o.tenant_id,o.store_id,o.id,o.generation+1,
     CASE WHEN v_input_done THEN 'CANCELLED' ELSE 'UNKNOWN' END,'',
     CASE WHEN v_input_done THEN 'media_cancelled_before_start' ELSE 'media_input_held' END);
   v_state:=CASE WHEN v_input_done THEN 'cancelled_before_start' ELSE 'requested' END;
  ELSE
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
    VALUES(o.tenant_id,o.store_id,o.id,greatest(o.generation,1),o.state,coalesce(o.lease_mode,''),
     'media_stop_requested');
  END IF;
 END IF;
 -- A fresh inner authorization and token-expiry read follow every lock/event wait.
 SELECT * INTO v_final FROM identity.resolve_access(p_auth_hash,p_store,'live:manage');
 SELECT expires_at INTO v_expiry FROM identity.sessions
  WHERE token_hash=p_auth_hash AND audience='merchant' AND revoked_at IS NULL;
 v_now:=clock_timestamp();
 IF v_final.access_status='unauthorized' OR v_expiry IS NULL OR v_expiry<=v_now THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP401'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM v_tenant
  OR v_final.principal_id IS DISTINCT FROM v_principal
  OR v_final.authz_revision IS DISTINCT FROM v_revision THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403'; END IF;
 RETURN jsonb_build_object('session_id',p_session::text,'attempt_id',a.id::text,
  'operation_id',o.id::text,'state',v_state);
END $$;
ALTER FUNCTION live.request_media_stop(bytea,uuid,uuid,uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.request_media_stop(bytea,uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.request_media_stop(bytea,uuid,uuid,uuid) TO commerce_runtime;

CREATE OR REPLACE FUNCTION live.claim_media_operation(p_id uuid,p_job bigint,p_lease_seconds integer,p_token bytea)
RETURNS TABLE(disposition text,generation bigint,mode text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 v_now timestamptz; v_mode text;
BEGIN
 IF p_id IS NULL OR p_job IS NULL OR p_job<1 OR p_lease_seconds IS NULL
  OR p_lease_seconds NOT BETWEEN 5 AND 300 OR p_token IS NULL OR octet_length(p_token)<>32
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media claim' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 IF a.execution_profile<>'PROVIDER_MOCK' THEN
  RAISE EXCEPTION 'media claim unavailable' USING ERRCODE='ME409'; END IF;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 INSERT INTO live.media_execution_state(attempt_id,tenant_id,store_id,session_id,
  authorization_id,operation_id,project_id)
 VALUES(a.id,a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id)
 ON CONFLICT (attempt_id) DO NOTHING;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=a.id FOR UPDATE;
 IF x.attempt_id IS NULL OR (x.tenant_id,x.store_id,x.session_id,x.authorization_id,x.operation_id,x.project_id)
  IS DISTINCT FROM (a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id) THEN
  RAISE EXCEPTION 'media projection unavailable' USING ERRCODE='ME409'; END IF;
 IF x.escalated_at IS NOT NULL THEN
  RETURN QUERY SELECT 'escalated'::text,o.generation,''::text; RETURN; END IF;
 IF x.resource_state='TERMINAL' OR o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING') THEN
  RETURN QUERY SELECT 'terminal'::text,o.generation,''::text; RETURN; END IF;
 IF p_job<>o.job_id OR NOT live.media_native_job(p_job,o.id) THEN
  RAISE EXCEPTION 'media job unavailable' USING ERRCODE='ME409'; END IF;
 v_now:=clock_timestamp();
 IF o.lease_until>v_now THEN RETURN QUERY SELECT 'busy'::text,o.generation,''::text; RETURN; END IF;
 IF o.generation>=4096 OR v_now>=o.created_at+interval '24 hours' THEN
  UPDATE live.media_execution_state SET escalated_at=v_now,escalation_code='reconcile_exhausted',updated_at=v_now
   WHERE attempt_id=a.id;
  UPDATE integration.operations SET state='UNKNOWN',generation=greatest(o.generation,1),
   lease_mode='',lease_until=NULL,lease_token_hash=NULL,result_code='reconcile_exhausted',updated_at=v_now
   WHERE id=o.id;
  INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
   VALUES(o.tenant_id,o.store_id,o.id,greatest(o.generation,1),'UNKNOWN','','media_reconcile_exhausted');
  RETURN QUERY SELECT 'escalated'::text,greatest(o.generation,1),''::text; RETURN;
 END IF;
 IF x.wire_reserved_at IS NULL THEN
  IF x.stop_requested_at IS NOT NULL THEN
   RAISE EXCEPTION 'media dispatch cancelled' USING ERRCODE='ME409'; END IF;
  IF NOT live.media_dispatch_eligible(o.id) THEN
   UPDATE integration.operations SET state='BLOCKED_POLICY',generation=o.generation+1,
    lease_mode='',lease_until=NULL,lease_token_hash=NULL,result_code='policy_denied',updated_at=clock_timestamp()
    WHERE id=o.id;
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
    VALUES(o.tenant_id,o.store_id,o.id,o.generation+1,'BLOCKED_POLICY','','media_policy_denied');
   RETURN QUERY SELECT 'terminal'::text,o.generation+1,''::text; RETURN;
  END IF;
  v_mode:='dispatch';
 ELSE
  v_mode:='reconcile';
  IF live.media_lifetime_revoked(o.id) THEN
   UPDATE live.media_execution_state SET cleanup_required=true,updated_at=clock_timestamp() WHERE attempt_id=a.id;
  END IF;
 END IF;
 v_now:=clock_timestamp();
 UPDATE integration.operations SET state=CASE WHEN v_mode='dispatch' THEN 'DISPATCHING' ELSE 'UNKNOWN' END,
  generation=o.generation+1,lease_mode=v_mode,lease_until=v_now+make_interval(secs=>p_lease_seconds),
  lease_token_hash=sha256(p_token),updated_at=v_now WHERE id=o.id;
 UPDATE live.media_execution_state SET claim_started_at=v_now,updated_at=v_now WHERE attempt_id=a.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,o.generation+1,
   CASE WHEN v_mode='dispatch' THEN 'DISPATCHING' ELSE 'UNKNOWN' END,v_mode,
   CASE WHEN v_mode='dispatch' THEN 'media_dispatch_claimed' ELSE 'media_reconcile_claimed' END);
 IF v_now+make_interval(secs=>p_lease_seconds)<=clock_timestamp()
  OR (v_mode='dispatch' AND NOT live.media_dispatch_eligible(o.id)) THEN
  RAISE EXCEPTION 'media claim unavailable' USING ERRCODE='ME409'; END IF;
 RETURN QUERY SELECT 'claimed'::text,o.generation+1,v_mode;
END $$;
ALTER FUNCTION live.claim_media_operation(uuid,bigint,integer,bytea) OWNER TO commerce_media_writer;

CREATE OR REPLACE FUNCTION live.load_media_material(p_id uuid,p_generation bigint,p_token bytea) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 v_key text; v_nonce text; v_ciphertext text;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL
  OR octet_length(p_token)<>32 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media material' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 IF a.execution_profile<>'PROVIDER_MOCK' THEN
  RAISE EXCEPTION 'media material unavailable' USING ERRCODE='ME409'; END IF;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 IF x.attempt_id IS NULL OR x.operation_id<>o.id OR x.project_id<>h.project_id
  OR o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token)
  OR o.lease_mode NOT IN ('dispatch','reconcile')
  OR (o.lease_mode='dispatch' AND o.state<>'DISPATCHING')
  OR (o.lease_mode='reconcile' AND o.state<>'UNKNOWN')
  OR x.escalated_at IS NOT NULL OR x.resource_state='TERMINAL' THEN
  RAISE EXCEPTION 'media lease unavailable' USING ERRCODE='ME409'; END IF;
 IF o.lease_mode='dispatch' THEN
  IF x.wire_reserved_at IS NOT NULL OR x.stop_requested_at IS NOT NULL
   OR NOT live.media_dispatch_eligible(o.id) THEN
   RAISE EXCEPTION 'media material unavailable' USING ERRCODE='ME409'; END IF;
  v_key:=h.key_id; v_nonce:=encode(h.nonce,'hex'); v_ciphertext:=encode(h.ciphertext,'hex');
 ELSE
  IF x.wire_reserved_at IS NULL THEN RAISE EXCEPTION 'media material unavailable' USING ERRCODE='ME409'; END IF;
  v_key:=''; v_nonce:=''; v_ciphertext:='';
 END IF;
 IF o.lease_until<=clock_timestamp() OR (o.lease_mode='dispatch' AND (x.stop_requested_at IS NOT NULL
  OR NOT live.media_dispatch_eligible(o.id))) THEN
  RAISE EXCEPTION 'media material unavailable' USING ERRCODE='ME409'; END IF;
 RETURN jsonb_build_object('tenant_id',a.tenant_id::text,'store_id',a.store_id::text,
  'session_id',a.session_id::text,'attempt_id',a.id::text,'project_id',h.project_id,
  'endpoint_identity',h.endpoint_identity,'credential_version',h.credential_version,
  'material_version',h.material_version,'room_name',a.room_name,'aspect_ratio',h.aspect_ratio,
  'egress_id',coalesce(x.egress_id,''),'mode',o.lease_mode,'key_id',v_key,
  'nonce_hex',v_nonce,'ciphertext_hex',v_ciphertext);
END $$;
ALTER FUNCTION live.load_media_material(uuid,bigint,bytea) OWNER TO commerce_media_writer;

CREATE OR REPLACE FUNCTION live.reserve_media_start(p_id uuid,p_generation bigint,p_token bytea) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; v_now timestamptz;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL
  OR octet_length(p_token)<>32 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media reservation' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 IF a.execution_profile<>'PROVIDER_MOCK' THEN
  RAISE EXCEPTION 'media reservation unavailable' USING ERRCODE='ME409'; END IF;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 IF x.attempt_id IS NULL OR x.operation_id<>o.id OR x.wire_reserved_at IS NOT NULL
  OR o.generation<>p_generation OR o.state<>'DISPATCHING' OR o.lease_mode<>'dispatch'
  OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token)
  OR x.stop_requested_at IS NOT NULL OR NOT live.media_dispatch_eligible(o.id) THEN
  RAISE EXCEPTION 'media reservation unavailable' USING ERRCODE='ME409'; END IF;
 v_now:=clock_timestamp();
 UPDATE live.media_execution_state SET wire_reserved_at=v_now,wire_generation=p_generation,updated_at=v_now
  WHERE attempt_id=x.attempt_id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,p_generation,'DISPATCHING','dispatch','media_start_reserved');
 IF o.lease_until<=clock_timestamp() OR NOT live.media_dispatch_eligible(o.id) THEN
  RAISE EXCEPTION 'media reservation unavailable' USING ERRCODE='ME409'; END IF;
END $$;
ALTER FUNCTION live.reserve_media_start(uuid,bigint,bytea) OWNER TO commerce_media_writer;

CREATE OR REPLACE FUNCTION live.project_media_observation(p_id uuid,p_generation bigint,p_token bytea,
 p_source text,p_egress text,p_room text,p_status text,p_started bigint,p_updated bigint,p_ended bigint)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 v_terminal boolean; v_egress_terminal boolean; v_joint boolean; v_rank integer; v_old_rank integer; v_coherent boolean;
 v_started bigint; v_updated bigint; v_ended bigint;
 v_hash bytea; v_now timestamptz; v_cleanup boolean;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL OR octet_length(p_token)<>32
  OR p_source IS NULL OR p_source NOT IN ('START','ROOM','QUERY','STOP')
  OR p_egress IS NULL OR p_egress !~ '^EG_[A-Za-z0-9_-]{1,100}$'
  OR p_room IS NULL OR p_room !~ '^lc_[0-9a-f]{32}$'
  OR p_status IS NULL OR p_status NOT IN ('EGRESS_STARTING','EGRESS_ACTIVE','EGRESS_ENDING',
   'EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED')
  OR p_started IS NULL OR p_started<0 OR p_updated IS NULL OR p_updated<0 OR p_ended IS NULL OR p_ended<0
  OR (p_started>0 AND p_updated>0 AND p_updated<p_started)
  OR (p_ended>0 AND ((p_started>0 AND p_ended<p_started)
   OR (p_updated>0 AND p_updated<p_ended)))
  OR (p_ended>0 AND p_status NOT IN ('EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED'))
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media observation' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 IF x.attempt_id IS NULL OR x.operation_id<>o.id OR x.project_id<>h.project_id
  OR x.wire_reserved_at IS NULL OR x.resource_state='TERMINAL' OR x.escalated_at IS NOT NULL
  OR o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token)
  OR o.lease_mode NOT IN ('dispatch','reconcile')
  OR (o.lease_mode='dispatch' AND o.state<>'DISPATCHING')
  OR (o.lease_mode='reconcile' AND o.state<>'UNKNOWN')
  OR p_room<>a.room_name OR (x.egress_id IS NOT NULL AND x.egress_id<>p_egress)
  OR (p_source='START' AND (o.lease_mode<>'dispatch' OR x.wire_generation<>p_generation))
  OR (p_source='ROOM' AND (o.lease_mode<>'reconcile' OR x.egress_id IS NOT NULL))
  OR (p_source='QUERY' AND (o.lease_mode<>'reconcile' OR x.egress_id IS NULL OR x.egress_id<>p_egress))
  OR (p_source='STOP' AND (o.lease_mode<>'reconcile' OR x.egress_id IS NULL
   OR x.egress_id<>p_egress OR x.stop_wire_count<1 OR x.stop_last_generation<>p_generation
   OR x.stop_observation_id IS NULL)) THEN
  RAISE EXCEPTION 'media observation unavailable' USING ERRCODE='ME409'; END IF;
 v_started:=greatest(x.started_at_ns,p_started);
 v_updated:=greatest(x.updated_at_ns,p_updated);
 v_ended:=greatest(x.ended_at_ns,p_ended);
 v_coherent:=NOT ((v_started>0 AND v_updated>0 AND v_updated<v_started)
  OR (v_started>0 AND v_ended>0 AND v_ended<v_started)
  OR (v_updated>0 AND v_ended>0 AND v_updated<v_ended));
 IF NOT v_coherent THEN
  v_started:=x.started_at_ns; v_updated:=x.updated_at_ns; v_ended:=x.ended_at_ns;
 END IF;
 v_terminal:=v_coherent AND p_status IN ('EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED') AND v_ended>0;
 v_egress_terminal:=v_terminal;
 IF v_terminal THEN
  v_joint:=live.close_media_input_custody(a.id,'egress_terminal');
  v_terminal:=v_joint;
 END IF;
 v_rank:=CASE p_status WHEN 'EGRESS_STARTING' THEN 1 WHEN 'EGRESS_ACTIVE' THEN 2
  WHEN 'EGRESS_ENDING' THEN 3 ELSE 4 END;
 v_old_rank:=CASE x.transport_status WHEN 'EGRESS_STARTING' THEN 1 WHEN 'EGRESS_ACTIVE' THEN 2
  WHEN 'EGRESS_ENDING' THEN 3 WHEN '' THEN 0 ELSE 4 END;
 v_hash:=sha256(convert_to(jsonb_build_object('attempt_id',a.id::text,'operation_id',o.id::text,
  'generation',p_generation,'source',p_source,'project_id',h.project_id,'room_name',p_room,
  'egress_id',p_egress,'status',p_status,'started_at_ns',p_started,
  'updated_at_ns',p_updated,'ended_at_ns',p_ended)::text,'UTF8'));
 INSERT INTO live.media_observations(tenant_id,store_id,attempt_id,operation_id,generation,source,
  project_id,room_name,egress_id,status,started_at_ns,updated_at_ns,ended_at_ns,report_hash)
 VALUES(a.tenant_id,a.store_id,a.id,o.id,p_generation,p_source,h.project_id,p_room,p_egress,
  p_status,p_started,p_updated,p_ended,v_hash) ON CONFLICT DO NOTHING;
 v_cleanup:=x.cleanup_required OR live.media_lifetime_revoked(o.id)
  OR (v_started>0 AND v_updated>0 AND
  v_updated::numeric-v_started::numeric>=h.max_duration_seconds::numeric*1000000000);
 v_now:=clock_timestamp();
 UPDATE live.media_execution_state SET egress_id=coalesce(x.egress_id,p_egress),
  resource_state=CASE WHEN v_egress_terminal THEN 'TERMINAL' ELSE 'OBSERVED' END,
  transport_status=CASE WHEN v_rank>=v_old_rank THEN p_status ELSE x.transport_status END,
  started_at_ns=v_started,updated_at_ns=v_updated,
  ended_at_ns=v_ended,cleanup_required=v_cleanup,updated_at=v_now
 WHERE attempt_id=a.id;
 UPDATE integration.operations SET state=CASE WHEN v_terminal THEN
   CASE WHEN p_status='EGRESS_COMPLETE' THEN 'SUCCEEDED' ELSE 'FAILED_FINAL' END ELSE 'UNKNOWN' END,
  lease_mode='',lease_until=NULL,lease_token_hash=NULL,
  result_code=CASE WHEN v_terminal THEN 'media_terminal' ELSE 'media_observed' END,
  provider_reference=p_egress,updated_at=v_now WHERE id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(o.tenant_id,o.store_id,o.id,p_generation,
  CASE WHEN v_terminal THEN CASE WHEN p_status='EGRESS_COMPLETE' THEN 'SUCCEEDED' ELSE 'FAILED_FINAL' END
   ELSE 'UNKNOWN' END,'',CASE WHEN v_terminal THEN 'media_terminal' ELSE 'media_observed' END);
 IF NOT v_cleanup AND live.media_lifetime_revoked(o.id) THEN
  UPDATE live.media_execution_state SET cleanup_required=true,updated_at=clock_timestamp()
   WHERE attempt_id=a.id;
 END IF;
 -- A nonterminal reply to the second committed Stop consumes the final
 -- budget now; do not wait for another Query that may never arrive.
 IF p_source='STOP' AND x.stop_wire_count=2 AND x.stop_exhausted_at IS NULL
  AND NOT v_terminal THEN
  v_now:=clock_timestamp();
  UPDATE live.media_execution_state SET stop_exhausted_at=v_now,updated_at=v_now WHERE attempt_id=a.id;
  INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
   VALUES(o.tenant_id,o.store_id,o.id,p_generation,'UNKNOWN','','media_stop_budget_exhausted');
 END IF;
 IF o.lease_until<=clock_timestamp() THEN RAISE EXCEPTION 'media lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN CASE WHEN v_terminal THEN 'terminal' ELSE 'observe' END;
END $$;

ALTER FUNCTION live.project_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint) OWNER TO commerce_media_writer;

CREATE OR REPLACE FUNCTION live.finish_media_uncertain(p_id uuid,p_generation bigint,p_token bytea,p_code text)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 v_now timestamptz; v_state text; v_done boolean:=true;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL OR octet_length(p_token)<>32
  OR p_code IS NULL OR p_code NOT IN ('remote_unknown','not_observed','invalid_observation',
   'credential_unavailable','material_invalid','policy_denied')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media finish' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 IF x.attempt_id IS NULL OR x.operation_id<>o.id OR x.escalated_at IS NOT NULL OR x.resource_state='TERMINAL'
  OR o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token)
  OR o.lease_mode NOT IN ('dispatch','reconcile')
  OR (o.lease_mode='dispatch' AND o.state<>'DISPATCHING')
  OR (o.lease_mode='reconcile' AND o.state<>'UNKNOWN') THEN
  RAISE EXCEPTION 'media lease unavailable' USING ERRCODE='ME409'; END IF;
 IF p_code='policy_denied' THEN
  v_done:=live.close_media_input_custody(o.media_attempt_id,'permission_lost'); END IF;
 IF p_code='policy_denied' AND x.wire_reserved_at IS NULL AND v_done THEN v_state:='BLOCKED_POLICY';
 ELSE v_state:='UNKNOWN'; END IF;
 v_now:=clock_timestamp();
 IF p_code='policy_denied' AND x.wire_reserved_at IS NOT NULL THEN
  UPDATE live.media_execution_state SET cleanup_required=true,updated_at=v_now WHERE attempt_id=x.attempt_id;
 END IF;
 UPDATE integration.operations SET state=v_state,lease_mode='',lease_until=NULL,lease_token_hash=NULL,
  result_code=p_code,updated_at=v_now WHERE id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(o.tenant_id,o.store_id,o.id,p_generation,v_state,'','media_'||p_code);
 IF o.lease_until<=clock_timestamp() THEN RAISE EXCEPTION 'media lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN CASE WHEN v_state='BLOCKED_POLICY' THEN 'terminal' ELSE 'observe' END;
END $$;
ALTER FUNCTION live.finish_media_uncertain(uuid,bigint,bytea,text) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.finish_media_uncertain(uuid,bigint,bytea,text) FROM PUBLIC;
