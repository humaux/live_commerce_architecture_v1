-- The new queue is paired only with LOCAL_SFU_MOCK_EGRESS; legacy remains exact.
CREATE FUNCTION live.media_input_job_open(p_job bigint) RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM integration.operations o
  JOIN live.media_input_custody i ON i.operation_id=o.id
  WHERE o.job_id=p_job AND i.state<>'CLOSED')
$$;
ALTER FUNCTION live.media_input_job_open(bigint) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_input_job_open(bigint) FROM PUBLIC;

CREATE OR REPLACE FUNCTION live.guard_media_job_family() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  IF live.media_input_job_open(OLD.id) THEN
   RAISE EXCEPTION 'open media input job' USING ERRCODE='22023'; END IF;
  RETURN OLD;
 END IF;
 IF TG_OP='INSERT' THEN
  -- River's app-clock timestamp is not the DB clock. Available means immediate.
  IF NEW.state='available' THEN NEW.scheduled_at:=clock_timestamp(); END IF;
  IF NEW.state IS DISTINCT FROM 'available' OR NEW.attempt IS DISTINCT FROM 0
   OR NEW.finalized_at IS NOT NULL OR NEW.scheduled_at>clock_timestamp() THEN
   RAISE EXCEPTION 'invalid media job family' USING ERRCODE='22023'; END IF;
 END IF;
 IF NEW.kind IS DISTINCT FROM 'live_media_operation_v1' OR NEW.queue NOT IN ('media_mock_v1','media_input_mock_v1')
  OR NEW.unique_key IS NOT NULL OR NEW.args IS NULL OR jsonb_typeof(NEW.args)<>'object'
  OR NEW.args->>'operation_id' IS NULL
  OR NEW.args->>'operation_id' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR NEW.args->>'version' IS DISTINCT FROM '1'
  OR NEW.args IS DISTINCT FROM jsonb_build_object('operation_id',NEW.args->>'operation_id','version',1) THEN
  RAISE EXCEPTION 'invalid media job family' USING ERRCODE='22023'; END IF;
 IF TG_OP='UPDATE' AND (NEW.id,NEW.kind,NEW.queue,NEW.args,NEW.unique_key)
  IS DISTINCT FROM (OLD.id,OLD.kind,OLD.queue,OLD.args,OLD.unique_key) THEN
  RAISE EXCEPTION 'immutable media job identity' USING ERRCODE='22023'; END IF;
 IF TG_OP='UPDATE' AND live.media_input_job_open(NEW.id)
  AND (NEW.finalized_at IS NOT NULL OR NEW.state NOT IN
   ('available','scheduled','retryable','running')) THEN
  RAISE EXCEPTION 'open media input job' USING ERRCODE='22023'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_media_job_family() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_media_job_family() FROM PUBLIC;


DROP TRIGGER media_job_family ON river_media.river_job;
CREATE TRIGGER media_job_family BEFORE INSERT OR UPDATE OR DELETE ON river_media.river_job
 FOR EACH ROW EXECUTE FUNCTION live.guard_media_job_family();

CREATE OR REPLACE FUNCTION live.media_native_job(p_job bigint,p_operation uuid) RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM integration.operations o
  JOIN live.media_attempts a ON a.id=o.media_attempt_id
  JOIN river_media.river_job j ON j.id=o.job_id WHERE j.id=p_job AND o.id=p_operation
  AND j.kind='live_media_operation_v1' AND j.queue=CASE WHEN a.execution_profile='LOCAL_SFU_MOCK_EGRESS' THEN
   'media_input_mock_v1' ELSE 'media_mock_v1' END AND j.unique_key IS NULL
  AND j.args=jsonb_build_object('operation_id',p_operation::text,'version',1))
$$;
ALTER FUNCTION live.media_native_job(bigint,uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_native_job(bigint,uuid) FROM PUBLIC;

CREATE OR REPLACE FUNCTION live.check_media_attempt_operation() RETURNS trigger
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
   AND j.kind='live_media_operation_v1' AND j.queue=CASE WHEN NEW.execution_profile='LOCAL_SFU_MOCK_EGRESS' THEN
    'media_input_mock_v1' ELSE 'media_mock_v1' END
   AND ((NEW.execution_profile='LOCAL_SFU_MOCK_EGRESS' AND EXISTS
    (SELECT 1 FROM live.media_input_custody i WHERE i.attempt_id=NEW.id AND i.operation_id=o.id))
    OR (NEW.execution_profile='PROVIDER_MOCK' AND NOT EXISTS
    (SELECT 1 FROM live.media_input_custody i WHERE i.attempt_id=NEW.id)))
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

CREATE OR REPLACE FUNCTION live.check_media_job_link() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE j river_media.river_job%ROWTYPE; o integration.operations%ROWTYPE;
 a live.media_attempts%ROWTYPE;
BEGIN
 SELECT * INTO j FROM river_media.river_job WHERE id=NEW.id;
 IF NOT FOUND OR j.kind IS DISTINCT FROM NEW.kind OR j.queue IS DISTINCT FROM NEW.queue
  OR j.args IS DISTINCT FROM NEW.args OR j.unique_key IS DISTINCT FROM NEW.unique_key
  OR j.kind<>'live_media_operation_v1' OR j.queue NOT IN ('media_mock_v1','media_input_mock_v1')
  OR j.state<>'available' OR j.attempt<>0 OR j.finalized_at IS NOT NULL
  OR j.scheduled_at>clock_timestamp()
  OR j.args->>'version' IS DISTINCT FROM '1' OR j.unique_key IS NOT NULL THEN
  RAISE EXCEPTION 'media job mismatch' USING ERRCODE='23514';
 END IF;
 SELECT * INTO o FROM integration.operations WHERE id=(j.args->>'operation_id')::uuid;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 IF o.id IS NULL OR a.id IS NULL OR o.job_id<>j.id OR o.actor_kind<>'MEDIA_ATTEMPT'
  OR o.provider<>'livekit' OR o.action<>'livekit.egress.start' OR o.purpose<>'service'
  OR o.tenant_id<>a.tenant_id OR o.store_id<>a.store_id OR a.start_operation_id<>o.id
  OR o.request_hash<>a.request_hash OR o.media_attempt_id<>a.id
  OR a.execution_profile NOT IN ('PROVIDER_MOCK','LOCAL_SFU_MOCK_EGRESS')
  OR (a.execution_profile='PROVIDER_MOCK' AND j.queue<>'media_mock_v1')
  OR (a.execution_profile='LOCAL_SFU_MOCK_EGRESS' AND
   (j.queue<>'media_input_mock_v1' OR NOT EXISTS
   (SELECT 1 FROM live.media_input_custody i WHERE i.attempt_id=a.id AND i.operation_id=o.id))) THEN
  RAISE EXCEPTION 'media job mismatch' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION live.check_media_job_link() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.check_media_job_link() FROM PUBLIC;

CREATE OR REPLACE FUNCTION live.reject_legacy_media_job() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.kind='live_media_operation_v1' OR NEW.queue IN ('media_mock_v1','media_input_mock_v1')
  OR (TG_OP='UPDATE' AND (OLD.kind='live_media_operation_v1' OR OLD.queue IN ('media_mock_v1','media_input_mock_v1'))) THEN
  RAISE EXCEPTION 'media job belongs to isolated lane' USING ERRCODE='22023';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.reject_legacy_media_job() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.reject_legacy_media_job() FROM PUBLIC;

CREATE OR REPLACE FUNCTION live.media_input_plan_ready() RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT live.media_worker_ready()
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c WHERE c.oid=
   'live.prepared_media_input_profiles'::regclass AND c.relrowsecurity AND c.relforcerowsecurity)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c WHERE c.oid=
   'live.media_input_custody'::regclass AND c.relrowsecurity AND c.relforcerowsecurity)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_trigger t WHERE t.tgrelid=
   'live.media_input_custody'::regclass AND t.tgname='media_input_custody_identity'
   AND t.tgenabled='O' AND t.tgfoid='live.guard_media_input_custody()'::regprocedure)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_trigger t WHERE t.tgrelid=
   'river_media.river_job'::regclass AND t.tgname='media_job_family'
   AND t.tgenabled='O' AND t.tgfoid='live.guard_media_job_family()'::regprocedure)
  AND NOT has_table_privilege('commerce_runtime','live.media_input_custody','SELECT')
  AND NOT has_table_privilege('commerce_media_executor','live.media_input_custody','SELECT')
  AND NOT has_table_privilege('commerce_media_worker','live.media_input_custody','SELECT')
  AND NOT has_table_privilege('commerce_media_registrar','live.media_input_custody','SELECT')
  AND NOT EXISTS(SELECT 1 FROM (VALUES
   ('live.plan_media_input_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint)'::regprocedure,
    'commerce_runtime'::regrole),
   ('live.reserve_media_input(bytea,uuid,uuid,uuid,bigint)'::regprocedure,
    'commerce_runtime'::regrole),
   ('live.claim_media_input_operation(uuid,bigint,integer,bytea)'::regprocedure,
    'commerce_media_executor'::regrole),
   ('live.close_media_input_admission(uuid,bigint,bytea,text)'::regprocedure,
    'commerce_media_executor'::regrole),
   ('live.load_media_input_custody(uuid,bigint,bytea)'::regprocedure,
    'commerce_media_executor'::regrole)) AS required(oid,caller)
   JOIN pg_catalog.pg_proc p ON p.oid=required.oid
   WHERE NOT p.prosecdef OR p.proowner<>'commerce_media_writer'::regrole
    OR NOT (p.proconfig @> ARRAY['search_path=pg_catalog']::text[])
    OR NOT has_function_privilege(required.caller,p.oid,'EXECUTE')
    OR EXISTS(SELECT 1 FROM pg_catalog.aclexplode(
      coalesce(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl
     WHERE acl.privilege_type='EXECUTE' AND acl.grantee=0))
  AND NOT EXISTS(SELECT 1 FROM live.prepared_media_input_profiles p
   LEFT JOIN live.prepared_media_authorizations h ON h.id=p.authorization_id
    AND h.tenant_id=p.tenant_id AND h.store_id=p.store_id
   WHERE h.id IS NULL OR h.environment<>'MOCK' OR h.evidence_type<>'MOCK_FIXTURE')
  AND NOT EXISTS(SELECT 1 FROM live.media_input_custody i
   LEFT JOIN live.media_attempts a ON a.id=i.attempt_id
   LEFT JOIN integration.operations o ON o.id=i.operation_id
   LEFT JOIN live.prepared_media_authorizations h ON h.id=i.authorization_id
   LEFT JOIN live.media_login_custody c ON c.attempt_id=i.attempt_id
   LEFT JOIN river_media.river_job j ON j.id=o.job_id
   WHERE a.id IS NULL OR o.id IS NULL OR h.id IS NULL OR c.attempt_id IS NULL
    OR (i.tenant_id,i.store_id) IS DISTINCT FROM (c.tenant_id,c.store_id)
    OR (i.tenant_id,i.store_id,i.session_id,i.authorization_id)
      IS DISTINCT FROM (a.tenant_id,a.store_id,a.session_id,a.authorization_id)
    OR a.start_operation_id<>o.id OR a.authorization_id<>h.id
    OR o.actor_kind<>'MEDIA_ATTEMPT' OR o.provider<>'livekit'
    OR o.action<>'livekit.egress.start' OR o.media_attempt_id<>a.id
    OR a.execution_profile<>'LOCAL_SFU_MOCK_EGRESS'
    OR a.room_name<>i.room_name OR i.session_version<>h.session_version
    OR i.created_at IS DISTINCT FROM a.created_at
    OR (i.project_id,i.endpoint_identity,i.credential_version)
      IS DISTINCT FROM (h.project_id,h.endpoint_identity,h.credential_version)
    OR i.lifetime_deadline IS DISTINCT FROM
     a.created_at+make_interval(secs=>h.max_duration_seconds)
    OR j.id IS NULL OR j.kind<>'live_media_operation_v1'
    OR j.queue<>'media_input_mock_v1' OR j.unique_key IS NOT NULL
    OR j.args IS DISTINCT FROM jsonb_build_object('operation_id',o.id::text,'version',1)
    OR (i.state<>'CLOSED' AND (o.state IN
     ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING')
     OR j.finalized_at IS NOT NULL OR j.state NOT IN
     ('available','scheduled','retryable','running'))))
$$;
ALTER FUNCTION live.media_input_plan_ready() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_input_plan_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.media_input_plan_ready() TO commerce_runtime,
 commerce_media_executor,commerce_media_worker;
