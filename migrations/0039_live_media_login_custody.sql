-- Forward-only initiating-login custody for existing MOCK media attempts.
-- No browser token or LIVE execution is enabled by this migration.
CREATE TABLE live.media_login_custody (
 attempt_id uuid PRIMARY KEY CHECK (attempt_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 login_session_id uuid NOT NULL REFERENCES identity.sessions(id),
 authz_revision bigint NOT NULL CHECK (authz_revision>0),
 login_expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK (login_expires_at>created_at),
 FOREIGN KEY (tenant_id,store_id,attempt_id)
  REFERENCES live.media_attempts(tenant_id,store_id,id) ON DELETE CASCADE
);
ALTER TABLE live.media_login_custody ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.media_login_custody FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.media_login_custody FROM PUBLIC;
CREATE POLICY media_login_custody_read ON live.media_login_custody FOR SELECT
 TO commerce_media_writer USING (true);
CREATE POLICY media_login_custody_insert ON live.media_login_custody FOR INSERT
 TO commerce_media_writer WITH CHECK (true);
GRANT SELECT,INSERT ON live.media_login_custody TO commerce_media_writer;
GRANT SELECT(id) ON identity.sessions TO commerce_media_writer;
GRANT SELECT(authz_revision) ON identity.memberships TO commerce_media_writer;

-- The identity writer already has session UPDATE privilege needed for FOR SHARE.
-- Only the media function owner can request this exact lock; no bearer leaves SQL.
CREATE FUNCTION identity.lock_media_login(p_hash bytea,p_principal uuid)
RETURNS TABLE(login_session_id uuid,login_expires_at timestamptz)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_id uuid; v_expiry timestamptz; v_revoked timestamptz;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_principal IS NULL
  OR p_principal='00000000-0000-0000-0000-000000000000'::uuid
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media login' USING ERRCODE='MP400'; END IF;
 SELECT s.id,s.expires_at,s.revoked_at INTO v_id,v_expiry,v_revoked
 FROM identity.sessions s WHERE s.token_hash=p_hash AND s.principal_id=p_principal
  AND s.audience='merchant' FOR SHARE;
 IF v_id IS NULL OR v_revoked IS NOT NULL OR v_expiry<=clock_timestamp() THEN
  RAISE EXCEPTION 'media login unavailable' USING ERRCODE='MP401'; END IF;
 RETURN QUERY SELECT v_id,v_expiry;
END $$;
ALTER FUNCTION identity.lock_media_login(bytea,uuid) OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION identity.lock_media_login(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.lock_media_login(bytea,uuid) TO commerce_media_writer;

-- A missing historical child is not inferred from the current caller's login.
CREATE FUNCTION live.media_login_eligible(p_operation uuid) RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS (
  SELECT 1 FROM integration.operations o
  JOIN live.media_attempts a ON a.id=o.media_attempt_id AND a.start_operation_id=o.id
  JOIN live.media_login_custody c ON c.attempt_id=a.id
   AND c.tenant_id=a.tenant_id AND c.store_id=a.store_id
  JOIN identity.sessions login ON login.id=c.login_session_id
   AND login.principal_id=a.original_principal_id AND login.audience='merchant'
   AND login.revoked_at IS NULL AND login.expires_at>clock_timestamp()
  JOIN control.tenants t ON t.id=a.tenant_id AND t.active
  JOIN control.stores s ON s.tenant_id=a.tenant_id AND s.id=a.store_id AND s.active
  JOIN identity.principals ip ON ip.id=a.original_principal_id AND ip.active
  JOIN identity.memberships m ON m.tenant_id=a.tenant_id AND m.principal_id=ip.id
   AND m.active AND m.authz_revision=c.authz_revision
  WHERE o.id=p_operation AND o.actor_kind='MEDIA_ATTEMPT'
   AND o.tenant_id=a.tenant_id AND o.store_id=a.store_id
   AND o.principal_id=a.original_principal_id
   AND c.login_expires_at>clock_timestamp()
   AND EXISTS (SELECT 1 FROM identity.store_grants g WHERE g.tenant_id=a.tenant_id
    AND g.store_id=a.store_id AND g.principal_id=ip.id AND g.permission='store:read')
   AND EXISTS (SELECT 1 FROM identity.store_grants g WHERE g.tenant_id=a.tenant_id
    AND g.store_id=a.store_id AND g.principal_id=ip.id AND g.permission='live:manage')
 )
$$;
ALTER FUNCTION live.media_login_eligible(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_login_eligible(uuid) FROM PUBLIC;

-- Runs after command.Run on both the fresh and cached-response paths.
CREATE FUNCTION live.assert_media_start_login(p_hash bytea,p_store uuid,p_attempt uuid)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_access record; v_final record; a live.media_attempts%ROWTYPE;
 c live.media_login_custody%ROWTYPE; v_login uuid; v_expiry timestamptz; v_now timestamptz;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_attempt IS NULL
  OR p_store='00000000-0000-0000-0000-000000000000'::uuid
  OR p_attempt='00000000-0000-0000-0000-000000000000'::uuid
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media login assertion' USING ERRCODE='MP400'; END IF;
 SELECT * INTO v_access FROM identity.resolve_access(p_hash,p_store,'live:manage');
 IF v_access.access_status='unauthorized' THEN
  RAISE EXCEPTION 'media login unavailable' USING ERRCODE='MP401'; END IF;
 IF v_access.access_status<>'ok' THEN
  RAISE EXCEPTION 'media login unavailable' USING ERRCODE='MP403'; END IF;
 IF v_access.tenant_id IS DISTINCT FROM nullif(current_setting('app.tenant_id',true),'')::uuid
  OR p_store IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid
  OR v_access.principal_id IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
  OR v_access.authz_revision IS DISTINCT FROM nullif(current_setting('app.authz_revision',true),'')::bigint THEN
  RAISE EXCEPTION 'media login unavailable' USING ERRCODE='MP403'; END IF;
 SELECT * INTO a FROM live.media_attempts WHERE id=p_attempt;
 IF a.id IS NULL OR a.tenant_id<>v_access.tenant_id OR a.store_id<>p_store THEN
  RAISE EXCEPTION 'media attempt unavailable' USING ERRCODE='MP404'; END IF;
 SELECT * INTO c FROM live.media_login_custody WHERE attempt_id=a.id;
 IF c.attempt_id IS NULL OR c.tenant_id<>a.tenant_id OR c.store_id<>a.store_id
  OR a.original_principal_id<>v_access.principal_id
  OR c.authz_revision<>v_access.authz_revision THEN
  RAISE EXCEPTION 'media login custody unavailable' USING ERRCODE='MP409'; END IF;
 SELECT login_session_id,login_expires_at INTO v_login,v_expiry
  FROM identity.lock_media_login(p_hash,v_access.principal_id);
 v_now:=clock_timestamp();
 IF v_login IS DISTINCT FROM c.login_session_id OR v_expiry<=v_now
  OR c.login_expires_at<=v_now THEN
  RAISE EXCEPTION 'media login unavailable' USING ERRCODE='MP403'; END IF;
 -- Fresh nested read follows the possible login-row wait.
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'live:manage');
 IF v_final.access_status='unauthorized' THEN
  RAISE EXCEPTION 'media login unavailable' USING ERRCODE='MP401'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM a.tenant_id
  OR v_final.principal_id IS DISTINCT FROM a.original_principal_id
  OR v_final.authz_revision IS DISTINCT FROM c.authz_revision
  OR v_expiry<=clock_timestamp() OR c.login_expires_at<=clock_timestamp() THEN
  RAISE EXCEPTION 'media login unavailable' USING ERRCODE='MP403'; END IF;
END $$;
ALTER FUNCTION live.assert_media_start_login(bytea,uuid,uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.assert_media_start_login(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.assert_media_start_login(bytea,uuid,uuid) TO commerce_runtime;

-- Replace touched functions statically; old MOCK checks and cleanup predicates remain.
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
 -- The identity-owned helper waits after the established media lock order.
 SELECT login_session_id,login_expires_at INTO v_login_id,v_login_expiry
  FROM identity.lock_media_login(p_hash,v_principal);
 v_now:=clock_timestamp();
 IF NOT FOUND OR v_auth.tenant_id<>v_tenant OR v_auth.store_id<>p_store
  OR v_auth.session_id<>p_session OR v_auth.environment<>'MOCK'
  OR v_auth.evidence_type<>'MOCK_FIXTURE' OR v_auth.start_before<=v_now
  OR v_login_expiry<=v_now
  OR v_auth.max_duration_seconds NOT BETWEEN 1 AND 14400
  OR v_auth.budget_minor NOT BETWEEN 1 AND 1000000000
  OR v_session.version<>p_expected OR v_auth.session_version<>p_expected
  OR v_program.state<>'DRAFT' OR v_program.aspect_ratio<>v_auth.aspect_ratio
  OR EXISTS(SELECT 1 FROM live.media_authorization_revocations
   WHERE authorization_id=p_authorization)
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
CREATE OR REPLACE FUNCTION live.media_dispatch_eligible(p_id uuid) RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS (
  SELECT 1 FROM integration.operations o
  JOIN live.media_attempts a ON a.id=o.media_attempt_id AND a.start_operation_id=o.id
  JOIN live.prepared_media_authorizations h ON h.id=a.authorization_id
  JOIN control.tenants t ON t.id=a.tenant_id AND t.active
  JOIN control.stores s ON s.tenant_id=a.tenant_id AND s.id=a.store_id AND s.active
  JOIN live.sessions ls ON ls.id=a.session_id AND ls.tenant_id=a.tenant_id AND ls.store_id=a.store_id
  JOIN live.programs p ON p.id=a.program_id AND p.session_id=a.session_id AND p.tenant_id=a.tenant_id AND p.store_id=a.store_id
  JOIN identity.principals ip ON ip.id=a.original_principal_id AND ip.active
  JOIN identity.memberships im ON im.tenant_id=a.tenant_id AND im.principal_id=ip.id AND im.active
  JOIN identity.store_grants g ON g.tenant_id=a.tenant_id AND g.store_id=a.store_id
   AND g.principal_id=ip.id AND g.permission='live:manage'
  JOIN integration.bindings b ON b.id=h.media_binding_id AND b.tenant_id=a.tenant_id AND b.store_id=a.store_id
  WHERE o.id=p_id AND o.actor_kind='MEDIA_ATTEMPT' AND o.principal_id=ip.id
   AND live.media_login_eligible(o.id)
   AND h.tenant_id=a.tenant_id AND h.store_id=a.store_id AND h.session_id=a.session_id
   AND h.attempt_id=a.id AND h.environment='MOCK' AND h.evidence_type='MOCK_FIXTURE'
   AND h.start_before>clock_timestamp() AND h.max_duration_seconds BETWEEN 1 AND 14400
   AND h.budget_minor BETWEEN 1 AND 1000000000
   AND ls.version=h.session_version AND p.state='READY' AND p.aspect_ratio=h.aspect_ratio
   AND b.enabled AND b.provider='livekit' AND b.external_asset_id=h.project_id
   AND b.semantic_version=h.media_binding_version
   AND NOT EXISTS(SELECT 1 FROM live.media_authorization_revocations r WHERE r.authorization_id=h.id)
   AND (SELECT count(*) FROM live.media_authorization_destinations d WHERE d.authorization_id=h.id) BETWEEN 1 AND 2
   AND NOT EXISTS (
    SELECT 1 FROM live.media_authorization_destinations d
    LEFT JOIN integration.bindings db ON db.id=d.binding_id AND db.tenant_id=a.tenant_id AND db.store_id=a.store_id
    WHERE d.authorization_id=h.id AND (db.id IS NULL OR NOT db.enabled
     OR db.semantic_version<>d.binding_version OR db.provider<>d.provider
     OR db.external_asset_id<>d.external_asset_id))
 )
$$;
CREATE OR REPLACE FUNCTION live.media_lifetime_revoked(p_id uuid) RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT NOT EXISTS(
  SELECT 1 FROM integration.operations o
  JOIN live.media_attempts a ON a.id=o.media_attempt_id
  JOIN live.prepared_media_authorizations h ON h.id=a.authorization_id
  JOIN control.tenants t ON t.id=a.tenant_id AND t.active
  JOIN control.stores s ON s.tenant_id=a.tenant_id AND s.id=a.store_id AND s.active
  JOIN identity.principals ip ON ip.id=a.original_principal_id AND ip.active
  JOIN identity.memberships im ON im.tenant_id=a.tenant_id AND im.principal_id=ip.id AND im.active
  JOIN identity.store_grants g ON g.tenant_id=a.tenant_id AND g.store_id=a.store_id
   AND g.principal_id=ip.id AND g.permission='live:manage'
  JOIN integration.bindings b ON b.id=h.media_binding_id AND b.tenant_id=a.tenant_id AND b.store_id=a.store_id
  WHERE o.id=p_id AND o.principal_id=ip.id AND h.attempt_id=a.id
   AND b.enabled AND b.provider='livekit' AND b.external_asset_id=h.project_id
   AND b.semantic_version=h.media_binding_version
   AND NOT EXISTS(SELECT 1 FROM live.media_authorization_revocations r WHERE r.authorization_id=h.id)
   AND NOT EXISTS(SELECT 1 FROM live.media_authorization_destinations d
    LEFT JOIN integration.bindings db ON db.id=d.binding_id AND db.tenant_id=a.tenant_id AND db.store_id=a.store_id
    WHERE d.authorization_id=h.id AND (db.id IS NULL OR NOT db.enabled OR db.provider<>d.provider
     OR db.external_asset_id<>d.external_asset_id OR db.semantic_version<>d.binding_version))
 ) OR NOT live.media_login_eligible(p_id)
$$;
