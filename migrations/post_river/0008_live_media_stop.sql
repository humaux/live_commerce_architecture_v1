-- Stop adds one worker function and one merchant function; preserve every LME gate.
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
  AND NOT EXISTS(SELECT 1 FROM (VALUES
   ('claim_started_at','timestamptz'::regtype,false),
   ('stop_requested_at','timestamptz'::regtype,false),
   ('stop_requested_by','uuid'::regtype,false),
   ('stop_wire_count','smallint'::regtype,true),
   ('stop_first_reserved_at','timestamptz'::regtype,false),
   ('stop_last_reserved_at','timestamptz'::regtype,false),
   ('stop_first_generation','bigint'::regtype,false),
   ('stop_last_generation','bigint'::regtype,false),
   ('stop_observation_id','uuid'::regtype,false),
   ('stop_exhausted_at','timestamptz'::regtype,false))
   AS required(name,kind,mandatory)
   LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid='live.media_execution_state'::regclass
    AND a.attname=required.name WHERE a.attnum IS NULL OR a.attisdropped
    OR a.atttypid<>required.kind OR a.attnotnull<>required.mandatory)
  AND (SELECT count(*)=3 FROM pg_catalog.pg_constraint c
   WHERE c.conrelid='live.media_execution_state'::regclass AND c.convalidated
    AND c.conname IN ('media_stop_request_pair','media_stop_request_member','media_stop_budget'))
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_constraint c
   WHERE c.conrelid='live.media_execution_state'::regclass AND c.conname='media_stop_observation_fk'
    AND c.convalidated AND c.condeferrable AND NOT c.condeferred)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_constraint c
   WHERE c.conrelid='live.media_observations'::regclass AND c.conname='media_observations_source_check'
    AND c.convalidated)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_trigger t
   WHERE t.tgrelid='live.media_execution_state'::regclass AND t.tgname='media_stop_projection'
    AND t.tgenabled='O' AND t.tgtype=23
    AND t.tgfoid='live.guard_media_stop_projection()'::regprocedure)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_proc p WHERE p.oid=
   'live.guard_media_stop_projection()'::regprocedure
   AND p.prosecdef AND p.proowner='commerce_media_writer'::regrole
   AND p.prorettype='trigger'::regtype
   AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[]
   AND NOT EXISTS(SELECT 1 FROM pg_catalog.aclexplode(coalesce(p.proacl,
    pg_catalog.acldefault('f',p.proowner))) acl WHERE acl.privilege_type='EXECUTE'
    AND acl.grantee<>'commerce_media_writer'::regrole))
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_proc p WHERE p.oid=
   'live.request_media_stop(bytea,uuid,uuid,uuid)'::regprocedure
   AND p.prosecdef AND p.proowner='commerce_media_writer'::regrole
   AND p.prorettype='jsonb'::regtype AND NOT p.proretset
   AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[]
   AND has_function_privilege('commerce_runtime',p.oid,'EXECUTE')
   AND NOT has_function_privilege('commerce_media_executor',p.oid,'EXECUTE')
   AND NOT has_function_privilege('commerce_media_worker',p.oid,'EXECUTE')
   AND NOT EXISTS(SELECT 1 FROM pg_catalog.aclexplode(coalesce(p.proacl,
    pg_catalog.acldefault('f',p.proowner))) acl WHERE acl.privilege_type='EXECUTE'
    AND acl.grantee NOT IN ('commerce_media_writer'::regrole,'commerce_runtime'::regrole)))
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

CREATE OR REPLACE FUNCTION live.media_worker_ready() RETURNS boolean
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
     'pg_catalog.text'::regtype,false),
    ('live.record_media_cleanup_query(uuid,bigint,bytea,text,text,text,bigint,bigint,bigint)'::regprocedure,
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
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_proc p WHERE p.oid=
   'live.project_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)'::regprocedure
   AND p.prosecdef AND p.proowner='commerce_media_writer'::regrole
   AND p.prorettype='text'::regtype AND NOT p.proretset
   AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[]
   AND NOT EXISTS(SELECT 1 FROM pg_catalog.aclexplode(coalesce(p.proacl,
    pg_catalog.acldefault('f',p.proowner))) acl WHERE acl.privilege_type='EXECUTE'
    AND acl.grantee<>'commerce_media_writer'::regrole))
  AND NOT has_table_privilege('commerce_media_executor','live.prepared_media_authorizations','SELECT')
  AND NOT has_table_privilege('commerce_media_worker','live.prepared_media_authorizations','SELECT')
  AND NOT has_table_privilege('commerce_media_worker','integration.operations','SELECT')
  AND NOT has_table_privilege('commerce_media_worker','river_media.river_migration','SELECT')
$$;
ALTER FUNCTION live.media_worker_ready() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_worker_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.media_worker_ready() TO commerce_media_executor,commerce_media_worker;
