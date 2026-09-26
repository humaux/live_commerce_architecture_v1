-- Upstream River owns this table. Admission is an application-owned invariant,
-- so install after River migrations, under the existing migration lock.
GRANT USAGE ON SCHEMA river TO commerce_meta_ingress;
GRANT SELECT,INSERT,UPDATE(kind) ON river.river_job TO commerce_meta_ingress;
GRANT USAGE ON SEQUENCE river.river_job_id_seq TO commerce_meta_ingress;
GRANT SELECT ON river.river_job TO commerce_meta_writer;

CREATE FUNCTION meta_inbox.guard_job_family() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.kind='meta_inbox_v1' OR NEW.queue='meta_inbox'
   OR pg_has_role(session_user,'commerce_meta_ingress','MEMBER') THEN
   PERFORM meta_inbox.require_authority('commerce_meta_ingress');
   IF NEW.kind<>'meta_inbox_v1' OR NEW.queue<>'meta_inbox' OR NEW.unique_key IS NOT NULL
    OR NEW.args IS NULL OR jsonb_typeof(NEW.args)<>'object'
    OR NEW.args->>'event_id' IS NULL OR NEW.args->>'event_id' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    OR NEW.args IS DISTINCT FROM jsonb_build_object('event_id',NEW.args->>'event_id','version',1)
    OR NEW.args->>'version' IS DISTINCT FROM '1' THEN RAISE EXCEPTION 'invalid meta job' USING ERRCODE='22023'; END IF;
  END IF;
 ELSIF OLD.kind='meta_inbox_v1' OR OLD.queue='meta_inbox' OR NEW.kind='meta_inbox_v1' OR NEW.queue='meta_inbox' THEN
  IF OLD.kind<>'meta_inbox_v1' OR OLD.queue<>'meta_inbox'
   OR NEW.id IS DISTINCT FROM OLD.id OR NEW.kind IS DISTINCT FROM OLD.kind OR NEW.queue IS DISTINCT FROM OLD.queue
   OR NEW.args IS DISTINCT FROM OLD.args OR NEW.unique_key IS DISTINCT FROM OLD.unique_key THEN
   RAISE EXCEPTION 'immutable meta job identity' USING ERRCODE='22023'; END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION meta_inbox.guard_job_family() OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION meta_inbox.guard_job_family() FROM PUBLIC;
CREATE TRIGGER meta_job_family BEFORE INSERT OR UPDATE ON river.river_job FOR EACH ROW EXECUTE FUNCTION meta_inbox.guard_job_family();

CREATE FUNCTION meta_inbox.guard_job_link() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.kind='meta_inbox_v1' OR NEW.queue='meta_inbox' THEN
  IF NOT EXISTS(SELECT 1 FROM river.river_job j JOIN meta_inbox.events e ON e.job_id=j.id
   WHERE j.id=NEW.id AND j.kind='meta_inbox_v1' AND j.queue='meta_inbox' AND j.unique_key IS NULL
    AND e.completed AND e.disposition='ROUTED' AND e.admission_xid=pg_current_xact_id()
    AND j.args=jsonb_build_object('event_id',e.id::text,'version',1) AND j.args->>'version'='1') THEN
   RAISE EXCEPTION 'unlinked meta job' USING ERRCODE='22023'; END IF;
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION meta_inbox.guard_job_link() OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION meta_inbox.guard_job_link() FROM PUBLIC;
CREATE CONSTRAINT TRIGGER meta_job_commit AFTER INSERT ON river.river_job DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION meta_inbox.guard_job_link();
