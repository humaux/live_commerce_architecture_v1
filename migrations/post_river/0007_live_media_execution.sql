-- Upgrade only the fifth native River lane; never widen the producer's job grants.
CREATE OR REPLACE FUNCTION live.guard_media_job_family() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  -- River's app-clock timestamp is not the DB clock. Available means immediate.
  IF NEW.state='available' THEN NEW.scheduled_at:=clock_timestamp(); END IF;
  IF NEW.state IS DISTINCT FROM 'available' OR NEW.attempt IS DISTINCT FROM 0
   OR NEW.finalized_at IS NOT NULL OR NEW.scheduled_at>clock_timestamp() THEN
   RAISE EXCEPTION 'invalid media job family' USING ERRCODE='22023'; END IF;
 END IF;
 IF NEW.kind IS DISTINCT FROM 'live_media_operation_v1' OR NEW.queue IS DISTINCT FROM 'media_mock_v1'
  OR NEW.unique_key IS NOT NULL OR NEW.args IS NULL OR jsonb_typeof(NEW.args)<>'object'
  OR NEW.args->>'operation_id' IS NULL
  OR NEW.args->>'operation_id' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR NEW.args->>'version' IS DISTINCT FROM '1'
  OR NEW.args IS DISTINCT FROM jsonb_build_object('operation_id',NEW.args->>'operation_id','version',1) THEN
  RAISE EXCEPTION 'invalid media job family' USING ERRCODE='22023'; END IF;
 IF TG_OP='UPDATE' AND (NEW.id,NEW.kind,NEW.queue,NEW.args,NEW.unique_key)
  IS DISTINCT FROM (OLD.id,OLD.kind,OLD.queue,OLD.args,OLD.unique_key) THEN
  RAISE EXCEPTION 'immutable media job identity' USING ERRCODE='22023'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_media_job_family() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_media_job_family() FROM PUBLIC;

CREATE OR REPLACE FUNCTION live.media_native_job(p_job bigint,p_operation uuid) RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM river_media.river_job j WHERE j.id=p_job
  AND j.kind='live_media_operation_v1' AND j.queue='media_mock_v1' AND j.unique_key IS NULL
  AND j.args=jsonb_build_object('operation_id',p_operation::text,'version',1))
$$;
ALTER FUNCTION live.media_native_job(bigint,uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_native_job(bigint,uuid) FROM PUBLIC;

GRANT USAGE ON SCHEMA river_media TO commerce_media_worker;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA river_media TO commerce_media_worker;
REVOKE ALL ON river_media.river_migration FROM commerce_media_worker;
GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA river_media TO commerce_media_worker;

-- Native lifecycle may change state/attempts; business identity and linkage may not.
CREATE OR REPLACE FUNCTION live.media_plan_ready() RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT
  (SELECT count(*)=2 FROM pg_catalog.pg_trigger t
   WHERE t.tgrelid='river_media.river_job'::regclass AND t.tgenabled='O' AND
    ((t.tgname='media_job_family' AND t.tgfoid='live.guard_media_job_family()'::regprocedure)
     OR (t.tgname='media_job_link' AND t.tgfoid='live.check_media_job_link()'::regprocedure
      AND t.tgdeferrable AND t.tginitdeferred)))
  AND (SELECT count(*)=4 FROM pg_catalog.pg_trigger t
   WHERE t.tgrelid IN ('river.river_job'::regclass,'river_meta.river_job'::regclass,
    'river_payment.river_job'::regclass,'river_expiry.river_job'::regclass)
    AND t.tgname='reject_media_job' AND t.tgenabled='O'
    AND t.tgfoid='live.reject_legacy_media_job()'::regprocedure)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_proc p WHERE p.oid=
   'live.plan_media_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint)'::regprocedure
   AND p.prosecdef AND p.proowner='commerce_media_writer'::regrole
   AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[]
   AND has_function_privilege('commerce_runtime',p.oid,'EXECUTE')
   AND NOT has_function_privilege('commerce_media_worker',p.oid,'EXECUTE')
   AND NOT has_function_privilege('commerce_media_executor',p.oid,'EXECUTE'))
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_proc p WHERE p.oid=
   'live.guard_media_job_family()'::regprocedure AND p.prosecdef
   AND p.proowner='commerce_media_writer'::regrole
   AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[])
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_proc p WHERE p.oid=
   'live.check_media_job_link()'::regprocedure AND p.prosecdef
   AND p.proowner='commerce_media_writer'::regrole
   AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[])
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_proc p WHERE p.oid=
   'live.reject_legacy_media_job()'::regprocedure AND p.prosecdef
   AND p.proowner='commerce_media_writer'::regrole
   AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[])
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c WHERE c.oid='live.media_attempts'::regclass
   AND c.relrowsecurity AND c.relforcerowsecurity)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c WHERE c.oid='live.media_execution_state'::regclass
   AND c.relrowsecurity AND c.relforcerowsecurity)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c WHERE c.oid='live.media_observations'::regclass
   AND c.relrowsecurity AND c.relforcerowsecurity)
  AND NOT has_table_privilege('commerce_worker','river_media.river_job','SELECT')
  AND NOT has_table_privilege('commerce_worker','live.media_attempts','SELECT')
  AND NOT EXISTS(SELECT 1 FROM integration.operations o
   LEFT JOIN live.media_attempts a ON a.tenant_id=o.tenant_id AND a.store_id=o.store_id
    AND a.id=o.media_attempt_id
   LEFT JOIN live.media_execution_state x ON x.attempt_id=a.id
   WHERE o.actor_kind='MEDIA_ATTEMPT'
   AND o.state NOT IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING')
   AND (a.id IS NULL OR a.start_operation_id<>o.id
    OR ((x.escalated_at IS NULL OR o.state<>'UNKNOWN')
     AND NOT live.media_native_job(o.job_id,o.id))
    OR (x.escalated_at IS NOT NULL AND EXISTS(SELECT 1 FROM river_media.river_job j WHERE j.id=o.job_id)
     AND NOT live.media_native_job(o.job_id,o.id))))
$$;
ALTER FUNCTION live.media_plan_ready() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_plan_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.media_plan_ready() TO commerce_runtime;

CREATE FUNCTION live.media_worker_ready() RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT live.media_plan_ready()
  AND NOT EXISTS(SELECT 1 FROM (VALUES
    ('live.claim_media_operation(uuid,bigint,integer,bytea)'::regprocedure,
     'pg_catalog.record'::regtype,true),
    ('live.load_media_material(uuid,bigint,bytea)'::regprocedure,
     'pg_catalog.jsonb'::regtype,false),
    ('live.reserve_media_start(uuid,bigint,bytea)'::regprocedure,
     'pg_catalog.void'::regtype,false),
    ('live.record_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)'::regprocedure,
     'pg_catalog.text'::regtype,false),
    ('live.finish_media_uncertain(uuid,bigint,bytea,text)'::regprocedure,
     'pg_catalog.text'::regtype,false)) AS required(oid,result_type,result_set)
   LEFT JOIN pg_catalog.pg_proc p ON p.oid=required.oid
   WHERE p.oid IS NULL OR NOT p.prosecdef OR p.proowner<>'commerce_media_writer'::regrole
    OR p.prorettype<>required.result_type OR p.proretset<>required.result_set
    OR NOT (p.proconfig @> ARRAY['search_path=pg_catalog']::text[])
    OR NOT has_function_privilege('commerce_media_executor',p.oid,'EXECUTE')
    OR EXISTS(SELECT 1 FROM pg_catalog.aclexplode(
      coalesce(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl
     WHERE acl.privilege_type='EXECUTE' AND acl.grantee NOT IN
      ('commerce_media_writer'::regrole,'commerce_media_executor'::regrole)))
  AND (SELECT count(*)=1 FROM pg_catalog.pg_trigger t
   WHERE t.tgrelid='live.media_execution_state'::regclass AND t.tgname='media_execution_identity'
    AND t.tgenabled='O')
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_trigger t WHERE t.tgrelid='live.media_execution_state'::regclass
    AND t.tgname='media_execution_identity' AND t.tgenabled='O'
    AND t.tgfoid='live.guard_media_execution_identity()'::regprocedure)
  AND NOT has_table_privilege('commerce_media_executor','live.prepared_media_authorizations','SELECT')
  AND NOT has_table_privilege('commerce_media_worker','live.prepared_media_authorizations','SELECT')
  AND NOT has_table_privilege('commerce_media_worker','integration.operations','SELECT')
  AND NOT has_table_privilege('commerce_media_worker','river_media.river_migration','SELECT')
$$;
ALTER FUNCTION live.media_worker_ready() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_worker_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.media_worker_ready() TO commerce_media_executor,commerce_media_worker;
