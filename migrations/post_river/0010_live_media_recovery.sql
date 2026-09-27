-- Read-only pin for the opt-in recovery surface, installed after native River.
CREATE FUNCTION live.media_recovery_ready() RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT
  (SELECT count(*)=7 FROM (VALUES
   ('live.begin_media_recovery_episode(uuid,bigint,integer,boolean)',
    'TABLE(disposition text, episode_id uuid, operation_id uuid, job_id bigint, baseline_generation bigint, deadline_at timestamp with time zone, candidate_count integer, coverage_known boolean, blocked_by_episode_id uuid)'),
   ('live.claim_recovery_observation(uuid,uuid,bigint,bytea)',
    'TABLE(disposition text, generation bigint, project_id text, credential_version bigint, endpoint_identity text, room_name text, egress_id text)'),
   ('live.record_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)',
    'TABLE(disposition text, observation_id uuid)'),
   ('live.finish_recovery_observation(uuid,uuid,bigint,bytea,text)', 'text'),
   ('live.read_media_recovery_episode(uuid)',
    'TABLE(episode_id uuid, scope_status text, coverage_known boolean, candidate_count integer, blocked_by_episode_id uuid, operation_id uuid, disposition text, baseline_generation bigint, observation_id uuid, observation_source text, observation_generation bigint, witness_elapsed_ms bigint, timeout_at timestamp with time zone, cleanup_required boolean)'),
   ('live.witness_media_recovery_episode(uuid,uuid,uuid,bigint)', 'text'),
   ('live.timeout_media_recovery_episode(uuid,bigint)',
    'TABLE(disposition text, affected_count integer)')
  ) AS expected(signature,result_shape)
  JOIN pg_catalog.pg_proc p ON p.oid=pg_catalog.to_regprocedure(expected.signature)
  WHERE pg_catalog.pg_get_function_result(p.oid)=expected.result_shape
   AND p.proowner='commerce_media_writer'::regrole AND p.prosecdef
   AND 'search_path=pg_catalog'=ANY(p.proconfig)
   AND pg_catalog.has_function_privilege('commerce_media_recovery',p.oid,'EXECUTE')
   AND NOT EXISTS(SELECT 1 FROM pg_catalog.aclexplode(
    coalesce(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl
    WHERE acl.privilege_type='EXECUTE'
     AND acl.grantee NOT IN (p.proowner,'commerce_media_recovery'::regrole)))
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c
   WHERE c.oid='live.media_recovery_episode_scope'::regclass
    AND c.relrowsecurity AND c.relforcerowsecurity)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c
   WHERE c.oid='live.media_execution_state'::regclass
    AND c.relrowsecurity AND c.relforcerowsecurity)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_class c
   WHERE c.oid='live.media_observations'::regclass
    AND c.relrowsecurity AND c.relforcerowsecurity)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_trigger t
   WHERE t.tgrelid='live.media_execution_state'::regclass
    AND t.tgname='media_execution_identity' AND t.tgenabled='O'
    AND t.tgfoid='live.guard_media_execution_identity()'::regprocedure)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_trigger t
   WHERE t.tgrelid='live.media_observations'::regclass
    AND t.tgname='media_recovery_observation' AND t.tgenabled='O'
    AND t.tgfoid='live.qualify_media_recovery_observation()'::regprocedure)
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_proc p WHERE p.oid='live.media_native_job(bigint,uuid)'::regprocedure
   AND p.proowner='commerce_media_writer'::regrole AND p.prosecdef
   AND pg_catalog.pg_get_functiondef(p.oid) LIKE '%river_media.river_job%')
  AND pg_catalog.to_regclass('river_media.river_job') IS NOT NULL
$$;
ALTER FUNCTION live.media_recovery_ready() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_recovery_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.media_recovery_ready() TO commerce_media_recovery;
