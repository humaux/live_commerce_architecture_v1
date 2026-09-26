-- MOCK-only durable Start intent. Provider execution is a later migration.
CREATE SCHEMA river_media;
REVOKE ALL ON SCHEMA river_media FROM PUBLIC;

CREATE FUNCTION live.media_plan_ready() RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$ SELECT false $$;
ALTER FUNCTION live.media_plan_ready() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_plan_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.media_plan_ready() TO commerce_runtime;

ALTER TABLE live.programs DROP CONSTRAINT programs_state_check;
ALTER TABLE live.programs ADD CONSTRAINT programs_state_check CHECK (state IN ('DRAFT','READY'));
ALTER TABLE live.programs ADD CONSTRAINT programs_scoped_identity UNIQUE (tenant_id,store_id,session_id,id);
ALTER TABLE live.prepared_media_authorizations ADD CONSTRAINT prepared_media_attempt_identity
 UNIQUE (tenant_id,store_id,session_id,id,attempt_id);

CREATE TABLE live.media_attempts (
 id uuid PRIMARY KEY CHECK (id<>'00000000-0000-0000-0000-000000000000'::uuid),
 tenant_id uuid NOT NULL CHECK (tenant_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 store_id uuid NOT NULL CHECK (store_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 session_id uuid NOT NULL CHECK (session_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 program_id uuid NOT NULL CHECK (program_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 authorization_id uuid NOT NULL UNIQUE CHECK (authorization_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 original_principal_id uuid NOT NULL CHECK (original_principal_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 start_operation_id uuid NOT NULL UNIQUE CHECK (start_operation_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 start_key text NOT NULL CHECK (start_key ~ '^[A-Za-z0-9_.:-]{8,128}$'),
 request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
 environment text NOT NULL CHECK (environment='MOCK'),
 execution_profile text NOT NULL CHECK (execution_profile='PROVIDER_MOCK'),
 room_name text NOT NULL UNIQUE CHECK (room_name='lc_'||replace(id::text,'-','')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE (tenant_id,store_id,id),
 UNIQUE (tenant_id,store_id,session_id),
 UNIQUE (tenant_id,store_id,start_key),
 FOREIGN KEY (tenant_id,store_id,session_id,program_id)
  REFERENCES live.programs(tenant_id,store_id,session_id,id),
 FOREIGN KEY (tenant_id,store_id,session_id,authorization_id,id)
  REFERENCES live.prepared_media_authorizations(tenant_id,store_id,session_id,id,attempt_id),
 FOREIGN KEY (tenant_id,original_principal_id) REFERENCES identity.memberships(tenant_id,principal_id),
 FOREIGN KEY (tenant_id,store_id,start_operation_id)
  REFERENCES integration.operations(tenant_id,store_id,id) DEFERRABLE INITIALLY DEFERRED
);
ALTER TABLE live.media_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.media_attempts FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.media_attempts FROM PUBLIC;
CREATE POLICY media_attempt_writer_read ON live.media_attempts FOR SELECT TO commerce_media_writer USING (true);
CREATE POLICY media_attempt_writer_insert ON live.media_attempts FOR INSERT TO commerce_media_writer WITH CHECK (true);
CREATE POLICY media_attempt_runtime_read ON live.media_attempts FOR SELECT TO commerce_runtime USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT,INSERT ON live.media_attempts TO commerce_media_writer;
GRANT SELECT ON live.media_attempts TO commerce_runtime;

-- Runtime cannot manufacture the new actor, even though it can still plan old MERCHANT operations.
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM integration.operations WHERE actor_kind='MERCHANT'
  AND action IN ('livekit.egress.start','livekit.egress.stop')) THEN
  RAISE EXCEPTION 'existing reserved media operation requires operator review' USING ERRCODE='22023';
 END IF;
END $$;
ALTER TABLE integration.operations ADD COLUMN media_attempt_id uuid;
ALTER TABLE integration.operations DROP CONSTRAINT operation_actor_family;
ALTER TABLE integration.operations ADD CONSTRAINT operation_actor_family CHECK (
 (actor_kind='MERCHANT' AND principal_id IS NOT NULL AND media_attempt_id IS NULL
  AND payment_attempt_id IS NULL AND buyer_owner_id IS NULL AND buyer_session_id IS NULL
  AND action NOT IN ('livekit.egress.start','livekit.egress.stop'))
 OR (actor_kind='BUYER_PAYMENT_QUERY' AND principal_id IS NULL AND media_attempt_id IS NULL
  AND payment_attempt_id=id AND payment_attempt_id IS NOT NULL
  AND buyer_owner_id IS NOT NULL AND buyer_session_id IS NOT NULL
  AND provider='payuni' AND action='payuni.query' AND purpose='transactional'
  AND state NOT IN ('READY','DISPATCHING','BLOCKED_POLICY','STALE_BINDING') AND lease_mode<>'dispatch')
 OR (actor_kind='MEDIA_ATTEMPT' AND principal_id IS NOT NULL AND media_attempt_id IS NOT NULL
  AND payment_attempt_id IS NULL AND buyer_owner_id IS NULL AND buyer_session_id IS NULL
  AND provider='livekit' AND purpose='service' AND action='livekit.egress.start')
);
ALTER TABLE integration.operations ADD CONSTRAINT operation_media_attempt_fk
 FOREIGN KEY (tenant_id,store_id,media_attempt_id) REFERENCES live.media_attempts(tenant_id,store_id,id);

DROP POLICY operation_read ON integration.operations;
CREATE POLICY operation_read ON integration.operations FOR SELECT TO commerce_runtime USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND actor_kind='MERCHANT');
DROP POLICY operation_insert ON integration.operations;
CREATE POLICY operation_insert ON integration.operations FOR INSERT TO commerce_runtime WITH CHECK
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
  AND actor_kind='MERCHANT' AND state='READY' AND generation=0);
DROP POLICY event_read ON integration.operation_events;
CREATE POLICY event_read ON integration.operation_events FOR SELECT TO commerce_runtime USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND EXISTS (SELECT 1 FROM integration.operations o WHERE o.id=operation_id
   AND o.tenant_id=operation_events.tenant_id AND o.store_id=operation_events.store_id
   AND o.actor_kind='MERCHANT'));
DROP POLICY event_insert ON integration.operation_events;
CREATE POLICY event_insert ON integration.operation_events FOR INSERT TO commerce_runtime WITH CHECK
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND state='READY' AND generation=0 AND mode=''
  AND EXISTS (SELECT 1 FROM integration.operations o WHERE o.id=operation_id
   AND o.tenant_id=operation_events.tenant_id AND o.store_id=operation_events.store_id
   AND o.actor_kind='MERCHANT'));
DROP POLICY worker_operation_read ON integration.operations;
CREATE POLICY worker_operation_read ON integration.operations FOR SELECT TO commerce_worker USING
 (actor_kind<>'MEDIA_ATTEMPT');
CREATE POLICY integration_writer_operation_read ON integration.operations FOR SELECT TO commerce_integration_writer USING (true);
DROP POLICY worker_event_read ON integration.operation_events;
CREATE POLICY worker_event_read ON integration.operation_events FOR SELECT TO commerce_worker USING
 (EXISTS (SELECT 1 FROM integration.operations o WHERE o.id=operation_id
  AND o.tenant_id=operation_events.tenant_id AND o.store_id=operation_events.store_id
  AND o.actor_kind<>'MEDIA_ATTEMPT'));
CREATE POLICY integration_writer_event_read ON integration.operation_events FOR SELECT TO commerce_integration_writer USING (true);
CREATE POLICY media_writer_operation_read ON integration.operations FOR SELECT TO commerce_media_writer USING
 (actor_kind='MEDIA_ATTEMPT');
CREATE POLICY media_writer_operation_insert ON integration.operations FOR INSERT TO commerce_media_writer WITH CHECK
 (actor_kind='MEDIA_ATTEMPT');
CREATE POLICY media_writer_event_insert ON integration.operation_events FOR INSERT TO commerce_media_writer WITH CHECK
 (EXISTS (SELECT 1 FROM integration.operations o WHERE o.id=operation_id
  AND o.tenant_id=operation_events.tenant_id AND o.store_id=operation_events.store_id
  AND o.actor_kind='MEDIA_ATTEMPT'));
GRANT SELECT,INSERT ON integration.operations,integration.operation_events TO commerce_media_writer;
GRANT USAGE ON SEQUENCE integration.operation_events_id_seq TO commerce_media_writer;

-- Exact update gates: the trigger runs after its own row lock and each query is fresh.
CREATE FUNCTION live.guard_frozen_session() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF (NEW.title,NEW.scheduled_at,NEW.version,NEW.updated_at)
    IS DISTINCT FROM (OLD.title,OLD.scheduled_at,OLD.version,OLD.updated_at)
    AND EXISTS (SELECT 1 FROM live.media_attempts a WHERE a.tenant_id=OLD.tenant_id
     AND a.store_id=OLD.store_id AND a.session_id=OLD.id) THEN
  RAISE EXCEPTION 'media draft frozen' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_frozen_session() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_frozen_session() FROM PUBLIC;
CREATE TRIGGER frozen_media_session BEFORE UPDATE ON live.sessions
 FOR EACH ROW EXECUTE FUNCTION live.guard_frozen_session();

CREATE FUNCTION live.guard_frozen_program() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF (NEW.aspect_ratio IS DISTINCT FROM OLD.aspect_ratio OR (OLD.state='READY' AND NEW.state<>'READY'))
    AND EXISTS (SELECT 1 FROM live.media_attempts a WHERE a.tenant_id=OLD.tenant_id
     AND a.store_id=OLD.store_id AND a.session_id=OLD.session_id AND a.program_id=OLD.id) THEN
  RAISE EXCEPTION 'media draft frozen' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_frozen_program() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_frozen_program() FROM PUBLIC;
CREATE TRIGGER frozen_media_program BEFORE UPDATE ON live.programs
 FOR EACH ROW EXECUTE FUNCTION live.guard_frozen_program();
REVOKE UPDATE(id) ON live.programs FROM commerce_media_writer;
GRANT UPDATE(state) ON live.programs TO commerce_media_writer;
DROP POLICY media_program_lock ON live.programs;
CREATE POLICY media_program_lock ON live.programs FOR UPDATE TO commerce_media_writer USING (true) WITH CHECK (true);

GRANT USAGE ON SCHEMA identity TO commerce_media_writer;
GRANT SELECT(token_hash,principal_id,audience,revoked_at,expires_at) ON identity.sessions TO commerce_media_writer;
GRANT EXECUTE ON FUNCTION identity.resolve_access(bytea,uuid,text) TO commerce_media_writer;

-- Exact actor correspondence, not only an FK into the same store.
CREATE FUNCTION live.check_media_attempt_operation() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; a live.prepared_media_authorizations%ROWTYPE;
BEGIN
 SELECT * INTO o FROM integration.operations WHERE tenant_id=NEW.tenant_id AND store_id=NEW.store_id
  AND id=NEW.start_operation_id;
 SELECT * INTO a FROM live.prepared_media_authorizations WHERE id=NEW.authorization_id;
 IF o.id IS NULL OR a.id IS NULL OR o.actor_kind<>'MEDIA_ATTEMPT' OR o.media_attempt_id<>NEW.id
  OR o.provider<>'livekit' OR o.purpose<>'service' OR o.action<>'livekit.egress.start'
  OR o.binding_id<>a.media_binding_id OR o.binding_version<>a.media_binding_version
  OR o.external_asset_id<>a.project_id OR o.job_id<1 OR o.request_hash<>NEW.request_hash
  OR o.principal_id IS DISTINCT FROM NEW.original_principal_id
  OR o.semantic_key<>'livekit.start:'||NEW.id::text
  OR o.request IS DISTINCT FROM jsonb_build_object('attempt_id',NEW.id::text,
   'session_id',NEW.session_id::text,'version',1)
  OR NOT EXISTS (SELECT 1 FROM river_media.river_job j WHERE j.id=o.job_id
   AND j.kind='live_media_operation_v1' AND j.queue='media_mock_v1'
   AND j.state='available' AND j.attempt=0 AND j.finalized_at IS NULL
   AND j.scheduled_at<=clock_timestamp()
   AND j.args->>'version'='1'
   AND j.args=jsonb_build_object('operation_id',NEW.start_operation_id::text,'version',1)) THEN
  RAISE EXCEPTION 'media operation mismatch' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION live.check_media_attempt_operation() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.check_media_attempt_operation() FROM PUBLIC;
CREATE CONSTRAINT TRIGGER media_attempt_operation_link AFTER INSERT ON live.media_attempts
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION live.check_media_attempt_operation();

CREATE FUNCTION live.plan_media_start(p_hash bytea,p_store uuid,p_session uuid,p_authorization uuid,
 p_expected bigint,p_key text,p_operation uuid,p_job bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_access record; v_final record; v_auth live.prepared_media_authorizations%ROWTYPE;
 v_session live.sessions%ROWTYPE; v_program live.programs%ROWTYPE;
 v_binding integration.bindings%ROWTYPE; v_destination live.media_authorization_destinations%ROWTYPE;
 v_lock_id uuid; v_tenant uuid; v_principal uuid; v_revision bigint; v_count integer;
 v_digest bytea; v_now timestamptz; v_token_expiry timestamptz;
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
 v_now:=clock_timestamp();
 IF NOT FOUND OR v_auth.tenant_id<>v_tenant OR v_auth.store_id<>p_store
  OR v_auth.session_id<>p_session OR v_auth.environment<>'MOCK'
  OR v_auth.evidence_type<>'MOCK_FIXTURE' OR v_auth.start_before<=v_now
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
 RETURN jsonb_build_object('session_id',p_session::text,'program_id',v_program.id::text,
  'attempt_id',v_auth.attempt_id::text,'operation_id',p_operation::text,'job_id',p_job,
  'room_name','lc_'||replace(v_auth.attempt_id::text,'-',''),'state','READY');
END $$;
ALTER FUNCTION live.plan_media_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.plan_media_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.plan_media_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint) TO commerce_runtime;

-- Static forward copy of 0016 claim; only MEDIA is newly denied before mutation.
CREATE OR REPLACE FUNCTION integration.claim_operation(p_id uuid,p_lease_seconds integer,p_token bytea)
RETURNS TABLE(disposition text,generation bigint,mode text)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_binding_id uuid; b integration.bindings%ROWTYPE; o integration.operations%ROWTYPE;
 v_now timestamptz; v_reason text; v_changed boolean;
BEGIN
 IF p_lease_seconds IS NULL OR p_lease_seconds NOT BETWEEN 5 AND 300 OR p_token IS NULL OR octet_length(p_token)<>32 THEN
  RAISE EXCEPTION 'invalid claim' USING ERRCODE='22023'; END IF;
 SELECT x.binding_id INTO v_binding_id FROM integration.operations x WHERE x.id=p_id;
 IF NOT FOUND THEN RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.id=v_binding_id FOR SHARE;
 SELECT x.* INTO o FROM integration.operations x WHERE x.id=p_id FOR UPDATE;
 IF NOT FOUND OR o.binding_id<>b.id OR o.tenant_id<>b.tenant_id OR o.store_id<>b.store_id THEN
  RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
 IF o.actor_kind='MEDIA_ATTEMPT' THEN
  RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
 v_now:=clock_timestamp();
 IF o.lease_until>v_now THEN RETURN QUERY SELECT 'busy'::text,o.generation,''::text; RETURN; END IF;
 IF o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING') THEN
  RETURN QUERY SELECT 'terminal'::text,o.generation,''::text; RETURN; END IF;
 v_changed:=b.provider<>o.provider OR b.external_asset_id<>o.external_asset_id
  OR (o.actor_kind='MERCHANT' AND (NOT b.enabled OR b.semantic_version<>o.binding_version));
 IF v_changed THEN
  IF o.actor_kind='BUYER_PAYMENT_QUERY' THEN RETURN QUERY SELECT 'blocked_binding'::text,o.generation,''::text; RETURN; END IF;
  IF o.state='UNKNOWN' AND o.lease_until IS NULL AND o.result_code='binding_changed' THEN
   RETURN QUERY SELECT 'blocked_binding'::text,o.generation,''::text; RETURN; END IF;
  UPDATE integration.operations x SET state=CASE WHEN o.state='READY' THEN 'STALE_BINDING' ELSE 'UNKNOWN' END,
   generation=o.generation+1,lease_mode='',lease_until=NULL,lease_token_hash=NULL,result_code='binding_changed',updated_at=v_now
   WHERE x.id=p_id RETURNING x.* INTO o;
  v_reason:='binding_changed'; disposition:=CASE WHEN o.state='STALE_BINDING' THEN 'terminal' ELSE 'blocked_binding' END;
 ELSE
  mode:=CASE WHEN o.state='READY' THEN 'dispatch' ELSE 'reconcile' END;
  UPDATE integration.operations x SET state=CASE WHEN mode='dispatch' THEN 'DISPATCHING' ELSE 'UNKNOWN' END,
   generation=o.generation+1,lease_mode=mode,lease_until=v_now+make_interval(secs=>p_lease_seconds),lease_token_hash=sha256(p_token),updated_at=v_now
   WHERE x.id=p_id RETURNING x.* INTO o;
  disposition:='claimed'; v_reason:=CASE WHEN mode='dispatch' THEN 'dispatch_claimed' ELSE 'reconcile_claimed' END;
 END IF;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,o.generation,o.state,o.lease_mode,v_reason);
 generation:=o.generation; mode:=o.lease_mode; RETURN NEXT;
END $$;
ALTER FUNCTION integration.claim_operation(uuid,integer,bytea) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.claim_operation(uuid,integer,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.claim_operation(uuid,integer,bytea) TO commerce_worker;

-- Static forward copy of 0009 complete; MEDIA is not a generic completion.
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
 IF o.actor_kind='MEDIA_ATTEMPT' THEN
  RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
 IF o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=v_now
  OR o.lease_token_hash<>sha256(p_token) OR o.state NOT IN ('DISPATCHING','UNKNOWN') THEN
  RAISE EXCEPTION 'lease conflict' USING ERRCODE='40001';
 END IF;
 IF p_state='BLOCKED_POLICY' AND (o.lease_mode<>'dispatch' OR p_reference<>'') THEN
  RAISE EXCEPTION 'policy outcome requires undispatched claim' USING ERRCODE='22023';
 END IF;
 v_reason:=CASE WHEN NOT b.enabled OR b.semantic_version<>o.binding_version
  OR b.provider<>o.provider OR b.external_asset_id<>o.external_asset_id
  THEN 'completed_binding_changed' ELSE p_code END;
 UPDATE integration.operations SET state=p_state,result_code=p_code,provider_reference=p_reference,
  lease_mode='',lease_until=NULL,lease_token_hash=NULL,updated_at=v_now WHERE id=p_id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,o.generation,p_state,'',v_reason);
END $$;
ALTER FUNCTION integration.complete_operation(uuid,bigint,bytea,text,text,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.complete_operation(uuid,bigint,bytea,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.complete_operation(uuid,bigint,bytea,text,text,text) TO commerce_worker;
