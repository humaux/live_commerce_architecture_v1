-- Joint custody is mandatory even if the input row is missing or contradictory.
CREATE OR REPLACE FUNCTION live.media_input_job_open(p_job bigint) RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM integration.operations o
  JOIN live.media_attempts a ON a.id=o.media_attempt_id
  LEFT JOIN live.media_input_custody i ON i.attempt_id=a.id AND i.operation_id=o.id
  LEFT JOIN live.media_execution_state x ON x.attempt_id=a.id AND x.operation_id=o.id
  WHERE o.job_id=p_job AND a.execution_profile='LOCAL_SFU_MOCK_EGRESS'
   AND NOT coalesce(i.state='CLOSED' AND x.attempt_id IS NOT NULL AND
    ((x.resource_state='TERMINAL' AND x.egress_id IS NOT NULL AND x.ended_at_ns>0
      AND x.transport_status IN ('EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED'))
     OR (x.wire_reserved_at IS NULL AND i.admission_closed_at IS NOT NULL)),false))
$$;
ALTER FUNCTION live.media_input_job_open(bigint) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_input_job_open(bigint) FROM PUBLIC;

CREATE OR REPLACE FUNCTION live.guard_media_job_family() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  IF live.media_input_job_open(OLD.id) THEN
   RAISE EXCEPTION 'open media input job' USING ERRCODE='22023'; END IF;
  RETURN OLD;
 END IF;
 IF TG_OP='INSERT' THEN
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
  AND (NEW.finalized_at IS NOT NULL OR NEW.state NOT IN ('available','scheduled','retryable','running')) THEN
  IF EXISTS(SELECT 1 FROM live.media_input_custody i JOIN integration.operations o ON o.id=i.operation_id
   WHERE o.job_id=NEW.id AND i.runtime_version=1) THEN
   -- Native rescue can update many rows. Retain this exact unresolved row without
   -- aborting the whole batch or inventing a replacement/third Stop job.
   NEW.state:='pending'; NEW.finalized_at:=NULL;
  ELSE
   RAISE EXCEPTION 'open media input job' USING ERRCODE='22023';
  END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_media_job_family() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_media_job_family() FROM PUBLIC;

CREATE OR REPLACE FUNCTION live.media_browser_input_worker_ready() RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT live.media_worker_ready() AND live.media_input_plan_ready()
 AND (SELECT count(*)=16 FROM (VALUES
  ('live.claim_browser_input_operation(uuid,bigint,integer,bytea)',
   'TABLE(disposition text, generation bigint, mode text)','commerce_media_executor'),
  ('live.next_media_input_turn(uuid,bigint,bytea)','text','commerce_media_executor'),
  ('live.finish_media_input_turn(uuid,bigint,bytea,text)','text','commerce_media_executor'),
  ('live.reserve_media_input_start(uuid,bigint,bytea,text,text,text,text,boolean,boolean,boolean,boolean)',
   'jsonb','commerce_media_executor'),
  ('live.load_media_input_material(uuid,bigint,bytea)','jsonb','commerce_media_executor'),
  ('live.reserve_media_input_cleanup(uuid,bigint,bytea)','jsonb','commerce_media_executor'),
  ('live.record_media_input_cleanup(uuid,bigint,bytea,integer,text,text)','text','commerce_media_executor'),
  ('live.load_media_input_runtime_profile(bytea,uuid,uuid,uuid,bigint)','jsonb','commerce_runtime'),
  ('live.register_media_input_runtime_profile(uuid)','void','commerce_media_registrar'),
  ('live.guard_browser_input_custody()','trigger','commerce_media_writer'),
  ('live.guard_media_input_wire_step()','trigger','commerce_media_writer'),
  ('live.lock_browser_input_operation(uuid,bigint,bytea)','integration.operations','commerce_media_writer'),
  ('live.hold_browser_input(uuid,text)','void','commerce_media_writer'),
  ('live.complete_browser_input(uuid)','boolean','commerce_media_writer'),
  ('live.browser_input_next_action(uuid)','text','commerce_media_writer'),
  ('live.claim_media_input_kernel(uuid,bigint,integer,bytea)',
   'TABLE(disposition text, generation bigint, mode text)','commerce_media_writer')
 ) required(signature,result_shape,caller)
 JOIN pg_catalog.pg_proc p ON p.oid=pg_catalog.to_regprocedure(required.signature)
 WHERE pg_catalog.pg_get_function_result(p.oid)=required.result_shape
  AND p.proowner='commerce_media_writer'::regrole AND p.prosecdef AND p.provolatile='v'
  AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[]
  AND has_function_privilege(required.caller,p.oid,'EXECUTE')
  AND NOT EXISTS(SELECT 1 FROM pg_catalog.aclexplode(coalesce(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl
   WHERE acl.privilege_type='EXECUTE' AND acl.grantee NOT IN (p.proowner,required.caller::regrole)))
 AND NOT EXISTS(SELECT 1 FROM (VALUES
  ('live.prepared_media_input_runtime_profiles'::regclass),('live.media_input_wire_steps'::regclass)
 ) required(oid) JOIN pg_catalog.pg_class c ON c.oid=required.oid
 WHERE NOT c.relrowsecurity OR NOT c.relforcerowsecurity
  OR EXISTS(SELECT 1 FROM (VALUES ('commerce_runtime'),('commerce_media_executor'),
    ('commerce_media_worker'),('commerce_media_registrar')) denied(role_name)
   WHERE has_table_privilege(denied.role_name,c.oid,'SELECT,INSERT,UPDATE,DELETE')
    OR has_any_column_privilege(denied.role_name,c.oid,'SELECT,INSERT,UPDATE,REFERENCES')))
 AND (SELECT count(*)=2 FROM (VALUES
  ('live.media_input_custody'::regclass,'browser_input_custody_identity','live.guard_browser_input_custody()'::regprocedure),
  ('live.media_input_wire_steps'::regclass,'browser_input_wire_identity','live.guard_media_input_wire_step()'::regprocedure)
 ) expected(rel,name,fn) JOIN pg_catalog.pg_trigger t ON t.tgrelid=expected.rel
  AND t.tgname=expected.name AND t.tgfoid=expected.fn
  WHERE t.tgenabled='O' AND t.tgtype=23 AND NOT t.tgisinternal)
 AND NOT EXISTS(SELECT 1 FROM live.media_input_custody i
  LEFT JOIN live.prepared_media_input_runtime_profiles r ON r.authorization_id=i.authorization_id
  WHERE i.runtime_version=1 AND (r.authorization_id IS NULL OR r.runtime_version<>1
   OR (i.tenant_id,i.store_id) IS DISTINCT FROM (r.tenant_id,r.store_id)))
$$;
ALTER FUNCTION live.media_browser_input_worker_ready() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_browser_input_worker_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.media_browser_input_worker_ready() TO commerce_media_executor,commerce_media_worker;

-- Forward the BIC readiness predicate: native held rows are valid only for explicit runtime 1.
CREATE OR REPLACE FUNCTION live.media_input_plan_ready() RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT live.media_worker_ready()
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c WHERE c.oid=
   'live.prepared_media_input_profiles'::regclass AND c.relrowsecurity AND c.relforcerowsecurity)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c WHERE c.oid=
   'live.media_input_custody'::regclass AND c.relrowsecurity AND c.relforcerowsecurity)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_trigger t WHERE t.tgrelid=
   'live.media_input_custody'::regclass AND t.tgname='media_input_custody_identity'
   AND t.tgenabled='O' AND t.tgtype=23 AND NOT t.tgisinternal
   AND t.tgfoid='live.guard_media_input_custody()'::regprocedure)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_trigger t WHERE t.tgrelid=
   'river_media.river_job'::regclass AND t.tgname='media_job_family'
   AND t.tgenabled='O' AND t.tgtype=31 AND NOT t.tgisinternal
   AND t.tgfoid='live.guard_media_job_family()'::regprocedure)
  AND NOT EXISTS(SELECT 1 FROM (VALUES
   ('commerce_runtime'::regrole),('commerce_media_executor'::regrole),
   ('commerce_media_worker'::regrole),('commerce_media_registrar'::regrole)) AS denied(role_id)
   WHERE has_table_privilege(denied.role_id,'live.media_input_custody',
    'SELECT,INSERT,UPDATE,DELETE')
    OR has_table_privilege(denied.role_id,'live.prepared_media_input_profiles',
     'SELECT,INSERT,UPDATE,DELETE'))
  AND NOT EXISTS(SELECT 1 FROM (VALUES
   ('live.plan_media_input_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint)'::regprocedure,
    'jsonb'::regtype,false,ARRAY['commerce_media_writer'::regrole,'commerce_runtime'::regrole]),
   ('live.reserve_media_input(bytea,uuid,uuid,uuid,bigint)'::regprocedure,
    'jsonb'::regtype,false,ARRAY['commerce_media_writer'::regrole,'commerce_runtime'::regrole]),
   ('live.claim_media_input_operation(uuid,bigint,integer,bytea)'::regprocedure,
    'record'::regtype,true,ARRAY['commerce_media_writer'::regrole,'commerce_media_executor'::regrole]),
   ('live.close_media_input_admission(uuid,bigint,bytea,text)'::regprocedure,
    'text'::regtype,false,ARRAY['commerce_media_writer'::regrole,'commerce_media_executor'::regrole]),
   ('live.load_media_input_custody(uuid,bigint,bytea)'::regprocedure,
    'jsonb'::regtype,false,ARRAY['commerce_media_writer'::regrole,'commerce_media_executor'::regrole]),
   ('live.register_media_input_profile(uuid)'::regprocedure,
    'void'::regtype,false,ARRAY['commerce_media_writer'::regrole,'commerce_media_registrar'::regrole]),
   ('live.close_media_input_custody(uuid,text)'::regprocedure,
    'boolean'::regtype,false,ARRAY['commerce_media_writer'::regrole]),
   ('live.guard_media_input_custody()'::regprocedure,
    'trigger'::regtype,false,ARRAY['commerce_media_writer'::regrole]),
   ('live.media_input_job_open(bigint)'::regprocedure,
    'boolean'::regtype,false,ARRAY['commerce_media_writer'::regrole]),
   ('live.media_input_plan_ready()'::regprocedure,
    'boolean'::regtype,false,ARRAY['commerce_media_writer'::regrole,
     'commerce_runtime'::regrole,'commerce_media_executor'::regrole,
     'commerce_media_worker'::regrole])) AS required(oid,result_type,result_set,allowed)
   JOIN pg_catalog.pg_proc p ON p.oid=required.oid
   WHERE NOT p.prosecdef OR p.proowner<>'commerce_media_writer'::regrole
    OR p.provolatile<>'v' OR p.prorettype<>required.result_type
    OR p.proretset<>required.result_set
    OR NOT (p.proconfig @> ARRAY['search_path=pg_catalog']::text[])
    OR NOT EXISTS(SELECT 1 FROM pg_catalog.aclexplode(
      coalesce(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl
     WHERE acl.privilege_type='EXECUTE' AND acl.grantee=ANY(required.allowed))
    OR EXISTS(SELECT 1 FROM pg_catalog.aclexplode(
      coalesce(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl
     WHERE acl.privilege_type='EXECUTE' AND acl.grantee<>ALL(required.allowed)))
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
    'commerce_media_executor'::regrole),
   ('live.register_media_input_profile(uuid)'::regprocedure,
    'commerce_media_registrar'::regrole),
   ('live.media_input_plan_ready()'::regprocedure,'commerce_runtime'::regrole),
   ('live.media_input_plan_ready()'::regprocedure,'commerce_media_executor'::regrole),
   ('live.media_input_plan_ready()'::regprocedure,'commerce_media_worker'::regrole))
   AS required(oid,caller) WHERE NOT has_function_privilege(required.caller,required.oid,'EXECUTE'))
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
     OR j.finalized_at IS NOT NULL OR (j.state NOT IN
     ('available','scheduled','retryable','running') AND NOT (i.runtime_version=1 AND j.state='pending')))))
$$;
