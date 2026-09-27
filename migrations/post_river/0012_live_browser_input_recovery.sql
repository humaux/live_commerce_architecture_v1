-- BRW07 readiness is opt-in; the historic seven-function readiness is unchanged.
CREATE FUNCTION live.media_browser_input_recovery_ready() RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT live.media_recovery_ready()
 AND (SELECT count(*)=7 FROM (VALUES
  ('live.begin_media_recovery_episode_with_input(uuid,bigint,integer,boolean)',
   'TABLE(disposition text, episode_id uuid, operation_id uuid, job_id bigint, baseline_generation bigint, deadline_at timestamp with time zone, candidate_count integer, coverage_known boolean, blocked_by_episode_id uuid, execution_profile text, input_required boolean, egress_required boolean)'),
  ('live.claim_media_recovery_observation_with_input(uuid,uuid,bigint,bytea)',
   'TABLE(disposition text, generation bigint, project_id text, credential_version bigint, endpoint_identity text, room_name text, publisher_identity text, egress_id text, execution_profile text, input_required boolean, egress_required boolean, input_observation_id uuid, egress_observation_id uuid)'),
  ('live.record_browser_input_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text)',
   'TABLE(disposition text, input_observation_id uuid)'),
  ('live.record_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)',
   'TABLE(disposition text, observation_id uuid)'),
  ('live.finish_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text)','text'),
  ('live.read_media_recovery_episode_with_input(uuid)',
   'TABLE(episode_id uuid, scope_status text, coverage_known boolean, candidate_count integer, blocked_by_episode_id uuid, operation_id uuid, disposition text, baseline_generation bigint, observation_id uuid, observation_source text, observation_generation bigint, witness_elapsed_ms bigint, timeout_at timestamp with time zone, cleanup_required boolean, execution_profile text, input_required boolean, egress_required boolean, input_observation_id uuid, input_observation_source text, input_observation_result text, input_observation_generation bigint, job_id bigint)'),
  ('live.witness_media_recovery_episode_with_input(uuid,uuid,uuid,uuid,bigint)','text')
 ) required(signature,result_shape)
 JOIN pg_catalog.pg_proc p ON p.oid=pg_catalog.to_regprocedure(required.signature)
 WHERE pg_catalog.pg_get_function_result(p.oid)=required.result_shape
  AND p.proowner='commerce_media_writer'::regrole AND p.prosecdef AND p.provolatile='v'
  AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[]
  AND pg_catalog.has_function_privilege('commerce_media_recovery',p.oid,'EXECUTE')
  AND NOT EXISTS(SELECT 1 FROM pg_catalog.aclexplode(
   coalesce(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl
   WHERE acl.privilege_type='EXECUTE'
    AND acl.grantee NOT IN (p.proowner,'commerce_media_recovery'::regrole)))
 AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_proc p
  JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
  WHERE n.nspname='live' AND p.proname IN
   ('begin_media_recovery_episode_with_input','claim_media_recovery_observation_with_input',
    'record_browser_input_recovery_observation','record_media_recovery_observation_with_input',
    'finish_media_recovery_observation_with_input','read_media_recovery_episode_with_input',
    'witness_media_recovery_episode_with_input')
   AND p.oid NOT IN (SELECT pg_catalog.to_regprocedure(signature) FROM (VALUES
    ('live.begin_media_recovery_episode_with_input(uuid,bigint,integer,boolean)'),
    ('live.claim_media_recovery_observation_with_input(uuid,uuid,bigint,bytea)'),
    ('live.record_browser_input_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text)'),
    ('live.record_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)'),
    ('live.finish_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text)'),
    ('live.read_media_recovery_episode_with_input(uuid)'),
    ('live.witness_media_recovery_episode_with_input(uuid,uuid,uuid,uuid,bigint)')) expected(signature)))
 AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c WHERE c.oid=
  'live.media_input_recovery_observations'::regclass AND c.relrowsecurity AND c.relforcerowsecurity)
 AND NOT EXISTS(SELECT 1 FROM (VALUES
  ('commerce_runtime'),('commerce_worker'),('commerce_media_executor'),
  ('commerce_media_worker'),('commerce_media_recovery'),('commerce_media_registrar')) denied(role_name)
  WHERE has_table_privilege(denied.role_name,'live.media_input_recovery_observations',
   'SELECT,INSERT,UPDATE,DELETE')
   OR has_any_column_privilege(denied.role_name,'live.media_input_recovery_observations',
    'SELECT,INSERT,UPDATE,REFERENCES'))
 AND NOT EXISTS(SELECT 1 FROM pg_catalog.aclexplode((SELECT c.relacl FROM pg_catalog.pg_class c
  WHERE c.oid='live.media_input_recovery_observations'::regclass)) acl
  WHERE acl.grantee=0 AND acl.privilege_type IN ('SELECT','INSERT','UPDATE','DELETE'))
 AND EXISTS(SELECT 1 FROM pg_catalog.pg_policy p
  WHERE p.polrelid='live.media_input_recovery_observations'::regclass
   AND p.polname='media_input_recovery_writer' AND p.polroles=ARRAY['commerce_media_writer'::regrole::oid])
 AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_policy p
  WHERE p.polrelid='live.media_input_recovery_observations'::regclass
   AND p.polname<>'media_input_recovery_writer')
 AND (SELECT count(*)=3 FROM (VALUES
  ('live.media_input_recovery_observations'::regclass,'media_input_recovery_receipt_guard',
   'live.guard_media_input_recovery_receipt()'::regprocedure,31),
  ('live.media_observations'::regclass,'media_recovery_observation',
   'live.qualify_media_recovery_observation()'::regprocedure,5),
  ('live.media_observations'::regclass,'media_recovery_mixed_egress',
   'live.qualify_mixed_media_recovery_egress()'::regprocedure,5)
 ) expected(rel,name,fn,type_code)
 JOIN pg_catalog.pg_trigger t ON t.tgrelid=expected.rel AND t.tgname=expected.name
  AND t.tgfoid=expected.fn AND t.tgenabled='O' AND t.tgtype=expected.type_code
  AND NOT t.tgisinternal)
 AND NOT EXISTS(SELECT 1 FROM (VALUES
  ('integration.operation_events'::regclass,'media_recovery_admitted_mask'),
  ('live.media_input_recovery_observations'::regclass,'media_input_recovery_observations_source_check'),
  ('live.media_input_recovery_observations'::regclass,'media_input_recovery_observations_result_check')
 ) expected(rel,name) LEFT JOIN pg_catalog.pg_constraint c
  ON c.conrelid=expected.rel AND c.conname=expected.name AND c.convalidated
 WHERE c.oid IS NULL)
 AND (SELECT count(*)=3 FROM pg_catalog.pg_constraint c
  WHERE c.conrelid='live.media_input_recovery_observations'::regclass
   AND c.contype='f' AND c.convalidated AND c.confrelid IN
    ('live.media_recovery_episode_scope'::regclass,'integration.operations'::regclass,
     'live.media_input_custody'::regclass))
 AND EXISTS(SELECT 1 FROM pg_catalog.pg_constraint c
  WHERE c.conrelid='live.media_input_recovery_observations'::regclass
   AND c.contype='u' AND c.convalidated)
 AND EXISTS(SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid=
  'live.media_recovery_episode_scope'::regclass AND a.attname='include_browser_input'
  AND a.attnotnull AND a.atthasdef)
 AND EXISTS(SELECT 1 FROM pg_catalog.pg_proc p
  WHERE p.oid='live.claim_media_recovery_observation_with_input(uuid,uuid,bigint,bytea)'::regprocedure
   AND pg_catalog.pg_get_functiondef(p.oid) LIKE '%media_recovery_native_eligible%')
 AND EXISTS(SELECT 1 FROM pg_catalog.pg_proc p
  WHERE p.oid='live.qualify_mixed_media_recovery_egress()'::regprocedure
   AND p.proowner='commerce_media_writer'::regrole AND p.prosecdef
   AND pg_catalog.pg_get_functiondef(p.oid) LIKE '%recovery_egress_required%')
 AND EXISTS(SELECT 1 FROM pg_catalog.pg_proc p
  WHERE p.oid='live.guard_media_input_recovery_receipt()'::regprocedure
   AND p.proowner='commerce_media_writer'::regrole AND p.prosecdef
   AND pg_catalog.pg_get_functiondef(p.oid) LIKE '%input recovery receipt immutable%')
 AND (SELECT count(*)=9 FROM (VALUES
  ('live.assert_legacy_media_recovery_scope(uuid)'),
  ('live.begin_media_recovery_episode_kernel(uuid,bigint,integer,boolean)'),
  ('live.claim_recovery_observation_kernel(uuid,uuid,bigint,bytea)'),
  ('live.record_recovery_observation_kernel(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)'),
  ('live.finish_recovery_observation_kernel(uuid,uuid,bigint,bytea,text)'),
  ('live.read_media_recovery_episode_kernel(uuid)'),
  ('live.witness_media_recovery_episode_kernel(uuid,uuid,uuid,bigint)'),
  ('live.qualify_mixed_media_recovery_egress()'),
  ('live.guard_media_input_recovery_receipt()')
 ) helper(signature) JOIN pg_catalog.pg_proc p ON p.oid=pg_catalog.to_regprocedure(helper.signature)
 WHERE p.proowner='commerce_media_writer'::regrole AND p.prosecdef
  AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[]
  AND NOT pg_catalog.has_function_privilege('commerce_media_recovery',p.oid,'EXECUTE')
  AND NOT EXISTS(SELECT 1 FROM pg_catalog.aclexplode(
   coalesce(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl
   WHERE acl.privilege_type='EXECUTE' AND acl.grantee NOT IN (p.proowner)))
$$;
ALTER FUNCTION live.media_browser_input_recovery_ready() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_browser_input_recovery_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.media_browser_input_recovery_ready() TO commerce_media_recovery;
