-- Startup audit only: no new projection/write authority. PL/pgSQL defers River
-- relation resolution because business migrations precede River on fresh DBs.
CREATE FUNCTION meta_inbox.runtime_ready()
RETURNS boolean LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE guards_ready boolean;
BEGIN
 SELECT count(*)=4 INTO guards_ready
 FROM (VALUES
  ('river.river_job','meta_job_family','meta_inbox.guard_job_family()',23,false),
  ('river.river_job','meta_job_commit','meta_inbox.guard_job_link()',5,true),
  ('social.messages','social_message_commit','meta_inbox.guard_social_insert()',5,true),
  ('social.comment_events','social_comment_commit','meta_inbox.guard_social_insert()',5,true)
 ) AS expected(relation_name,trigger_name,function_name,trigger_type,deferred)
 JOIN pg_catalog.pg_trigger t ON t.tgrelid=to_regclass(expected.relation_name)
  AND t.tgname=expected.trigger_name AND t.tgfoid=to_regprocedure(expected.function_name)
 JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid
 JOIN pg_catalog.pg_roles r ON r.oid=p.proowner
 JOIN pg_catalog.pg_class c ON c.oid=t.tgrelid
 JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE t.tgtype=expected.trigger_type AND t.tgenabled IN ('O','A') AND NOT t.tgisinternal
  AND t.tgdeferrable=expected.deferred AND t.tginitdeferred=expected.deferred
  AND t.tgqual IS NULL AND t.tgnargs=0 AND t.tgargs='\x'::bytea AND t.tgattr=''::int2vector
  AND p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog']::text[]
  AND r.rolname='commerce_meta_writer' AND NOT r.rolcanlogin AND NOT r.rolsuper
  AND NOT r.rolbypassrls AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication
  AND p.proowner<>c.relowner AND p.proowner<>n.nspowner
  AND p.proowner<>(SELECT datdba FROM pg_catalog.pg_database WHERE datname=current_database());
 IF NOT guards_ready THEN RETURN false; END IF;

 -- Do not fetch unrelated jobs or expose encrypted/raw payloads to the worker.
 -- Match by text instead of casting untrusted args to UUID (fail closed, no
 -- input-dependent SQL error or short-circuit evaluation assumptions).
 RETURN NOT EXISTS (
  SELECT 1 FROM river.river_job j
  WHERE (j.kind='meta_inbox_v1' OR j.queue='meta_inbox')
   AND j.state NOT IN ('completed','cancelled','discarded')
   AND (j.kind<>'meta_inbox_v1' OR j.queue<>'meta_inbox' OR j.unique_key IS NOT NULL
    OR NOT EXISTS (
     SELECT 1 FROM meta_inbox.events e
     WHERE e.job_id=j.id AND e.completed AND e.disposition='ROUTED'
      AND j.args=jsonb_build_object('event_id',e.id::text,'version',1)
      AND j.args->>'version'='1'
    ))
 );
END $$;
ALTER FUNCTION meta_inbox.runtime_ready() OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION meta_inbox.runtime_ready() FROM PUBLIC;
-- Schema USAGE alone cannot read or mutate any Meta table/function. Existing
-- explicit function/table revocations remain in force.
GRANT USAGE ON SCHEMA meta_inbox TO commerce_worker;
GRANT EXECUTE ON FUNCTION meta_inbox.runtime_ready() TO commerce_meta_ingress,commerce_worker;
