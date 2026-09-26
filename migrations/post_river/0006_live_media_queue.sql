-- The fifth native River schema is an inert, MOCK-only producer lane.
-- The private readiness function resolves these schema-qualified relation names.
GRANT USAGE ON SCHEMA river,river_meta,river_payment,river_expiry TO commerce_media_writer;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM river.river_job WHERE kind='live_media_operation_v1' OR queue='media_mock_v1')
  OR EXISTS (SELECT 1 FROM river_meta.river_job WHERE kind='live_media_operation_v1' OR queue='media_mock_v1')
  OR EXISTS (SELECT 1 FROM river_payment.river_job WHERE kind='live_media_operation_v1' OR queue='media_mock_v1')
  OR EXISTS (SELECT 1 FROM river_expiry.river_job WHERE kind='live_media_operation_v1' OR queue='media_mock_v1') THEN
  RAISE EXCEPTION 'unexpected legacy media job requires operator review' USING ERRCODE='22023';
 END IF;
END $$;

CREATE FUNCTION live.guard_media_job_family() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 -- River v0.40.0 stamps immediate jobs with the app clock. Normalize an
 -- available INSERT to the DB clock so it cannot be parked by clock skew.
 IF TG_OP='INSERT' AND NEW.state='available' THEN
  NEW.scheduled_at:=clock_timestamp();
 END IF;
 IF NEW.kind IS DISTINCT FROM 'live_media_operation_v1'
  OR NEW.queue IS DISTINCT FROM 'media_mock_v1' OR NEW.unique_key IS NOT NULL
  OR NEW.state IS DISTINCT FROM 'available' OR NEW.attempt IS DISTINCT FROM 0
  OR NEW.finalized_at IS NOT NULL OR NEW.scheduled_at>clock_timestamp()
  OR NEW.args IS NULL OR jsonb_typeof(NEW.args)<>'object'
  OR NEW.args->>'operation_id' IS NULL
  OR NEW.args->>'operation_id' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR NEW.args->>'version' IS DISTINCT FROM '1'
  OR NEW.args IS DISTINCT FROM jsonb_build_object('operation_id',NEW.args->>'operation_id','version',1) THEN
  RAISE EXCEPTION 'invalid media job family' USING ERRCODE='22023';
 END IF;
 IF TG_OP='UPDATE' AND (NEW.id IS DISTINCT FROM OLD.id OR NEW.kind IS DISTINCT FROM OLD.kind
  OR NEW.args IS DISTINCT FROM OLD.args OR NEW.queue IS DISTINCT FROM OLD.queue
  OR NEW.unique_key IS DISTINCT FROM OLD.unique_key) THEN
  RAISE EXCEPTION 'immutable media job identity' USING ERRCODE='22023';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_media_job_family() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_media_job_family() FROM PUBLIC;
CREATE TRIGGER media_job_family BEFORE INSERT OR UPDATE ON river_media.river_job
 FOR EACH ROW EXECUTE FUNCTION live.guard_media_job_family();

CREATE FUNCTION live.check_media_job_link() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE j river_media.river_job%ROWTYPE; o integration.operations%ROWTYPE;
 a live.media_attempts%ROWTYPE;
BEGIN
 SELECT * INTO j FROM river_media.river_job WHERE id=NEW.id;
 IF NOT FOUND OR j.kind IS DISTINCT FROM NEW.kind OR j.queue IS DISTINCT FROM NEW.queue
  OR j.args IS DISTINCT FROM NEW.args OR j.unique_key IS DISTINCT FROM NEW.unique_key
  OR j.kind<>'live_media_operation_v1' OR j.queue<>'media_mock_v1'
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
  OR o.request_hash<>a.request_hash OR o.media_attempt_id<>a.id THEN
  RAISE EXCEPTION 'media job mismatch' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION live.check_media_job_link() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.check_media_job_link() FROM PUBLIC;
CREATE CONSTRAINT TRIGGER media_job_link AFTER INSERT ON river_media.river_job
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION live.check_media_job_link();

CREATE FUNCTION live.reject_legacy_media_job() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.kind='live_media_operation_v1' OR NEW.queue='media_mock_v1'
  OR (TG_OP='UPDATE' AND (OLD.kind='live_media_operation_v1' OR OLD.queue='media_mock_v1')) THEN
  RAISE EXCEPTION 'media job belongs to isolated lane' USING ERRCODE='22023';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.reject_legacy_media_job() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.reject_legacy_media_job() FROM PUBLIC;
CREATE TRIGGER reject_media_job BEFORE INSERT OR UPDATE ON river.river_job
 FOR EACH ROW EXECUTE FUNCTION live.reject_legacy_media_job();
CREATE TRIGGER reject_media_job BEFORE INSERT OR UPDATE ON river_meta.river_job
 FOR EACH ROW EXECUTE FUNCTION live.reject_legacy_media_job();
CREATE TRIGGER reject_media_job BEFORE INSERT OR UPDATE ON river_payment.river_job
 FOR EACH ROW EXECUTE FUNCTION live.reject_legacy_media_job();
CREATE TRIGGER reject_media_job BEFORE INSERT OR UPDATE ON river_expiry.river_job
 FOR EACH ROW EXECUTE FUNCTION live.reject_legacy_media_job();

GRANT USAGE ON SCHEMA river_media TO commerce_runtime,commerce_media_writer;
GRANT SELECT,INSERT,UPDATE(kind) ON river_media.river_job TO commerce_runtime;
GRANT USAGE ON SEQUENCE river_media.river_job_id_seq TO commerce_runtime;
GRANT SELECT ON river_media.river_job TO commerce_media_writer;

CREATE OR REPLACE FUNCTION live.media_plan_ready() RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT
  (SELECT count(*)=2 FROM pg_catalog.pg_trigger t
   WHERE t.tgrelid='river_media.river_job'::regclass
    AND t.tgenabled='O' AND
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
   AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[])
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
  AND NOT has_table_privilege('commerce_worker','river_media.river_job','SELECT')
  AND NOT has_table_privilege('commerce_worker','live.media_attempts','SELECT')
  AND NOT EXISTS(SELECT 1 FROM integration.operations o
   LEFT JOIN live.media_attempts a ON a.tenant_id=o.tenant_id AND a.store_id=o.store_id
    AND a.id=o.media_attempt_id
   WHERE o.actor_kind='MEDIA_ATTEMPT'
   AND o.state NOT IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING')
   AND (a.id IS NULL OR a.start_operation_id<>o.id
    OR NOT EXISTS(SELECT 1 FROM river_media.river_job j WHERE j.id=o.job_id
     AND j.kind='live_media_operation_v1' AND j.queue='media_mock_v1'
     AND j.state='available' AND j.attempt=0 AND j.finalized_at IS NULL
     AND j.scheduled_at<=clock_timestamp()
     AND j.unique_key IS NULL AND j.args->>'version'='1'
     AND j.args=jsonb_build_object('operation_id',o.id::text,'version',1))))
$$;
ALTER FUNCTION live.media_plan_ready() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_plan_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.media_plan_ready() TO commerce_runtime;
