-- Native River maintenance is schema-wide, not queue-scoped. Apply only after
-- an authorized old-Meta drain. This transaction moves the existing lane, not
-- its receipts, encryption envelopes or social facts. 0031 keeps readiness
-- false if any lock/preflight/upstream/cutover step fails.
SET LOCAL lock_timeout='1s';
LOCK TABLE river.river_job IN ACCESS EXCLUSIVE MODE;
LOCK TABLE river.river_queue IN ACCESS EXCLUSIVE MODE;
LOCK TABLE river_meta.river_job IN ACCESS EXCLUSIVE MODE;
LOCK TABLE river_meta.river_queue IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM river_meta.river_job) THEN
  RAISE EXCEPTION 'meta destination is not empty' USING ERRCODE='22023';
 END IF;
 IF EXISTS(SELECT 1 FROM river.river_job WHERE (kind='meta_inbox_v1' OR queue='meta_inbox') AND state='running') THEN
  RAISE EXCEPTION 'meta cutover requires drained workers' USING ERRCODE='55000';
 END IF;
 IF EXISTS(SELECT 1 FROM river.river_job j WHERE (kind='meta_inbox_v1' OR queue='meta_inbox')
  AND (j.kind<>'meta_inbox_v1' OR j.queue<>'meta_inbox' OR j.unique_key IS NOT NULL
   OR NOT EXISTS(SELECT 1 FROM meta_inbox.events e WHERE e.job_id=j.id AND e.completed AND e.disposition='ROUTED'
    AND j.args=jsonb_build_object('event_id',e.id::text,'version',1) AND j.args->>'version'='1'))) THEN
  RAISE EXCEPTION 'invalid source meta job' USING ERRCODE='22023';
 END IF;
 IF EXISTS(SELECT 1 FROM river_meta.river_queue q WHERE q.name<>'meta_inbox'
  OR NOT EXISTS(SELECT 1 FROM river.river_queue old WHERE old.name=q.name AND to_jsonb(old)=to_jsonb(q))) THEN
  RAISE EXCEPTION 'meta destination queue conflict' USING ERRCODE='22023';
 END IF;
END $$;

-- List the pinned River v0.40 columns explicitly. Only its schema-local enum
-- requires a text cast. The full-row equality check below fails closed if an
-- upstream upgrade adds a persisted field that this migration does not retain.
INSERT INTO river_meta.river_job
 (id,state,attempt,max_attempts,attempted_at,created_at,finalized_at,scheduled_at,
  priority,args,attempted_by,errors,kind,metadata,queue,tags,unique_key,unique_states)
SELECT id,state::text::river_meta.river_job_state,attempt,max_attempts,attempted_at,
 created_at,finalized_at,scheduled_at,priority,args,attempted_by,errors,kind,metadata,queue,tags,unique_key,unique_states
FROM river.river_job WHERE kind='meta_inbox_v1' OR queue='meta_inbox';
INSERT INTO river_meta.river_queue(name,created_at,metadata,paused_at,updated_at)
 SELECT name,created_at,metadata,paused_at,updated_at FROM river.river_queue WHERE name='meta_inbox'
 ON CONFLICT(name) DO NOTHING;

DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM river.river_job old LEFT JOIN river_meta.river_job new ON new.id=old.id
  WHERE (old.kind='meta_inbox_v1' OR old.queue='meta_inbox') AND to_jsonb(old) IS DISTINCT FROM to_jsonb(new))
 OR (SELECT count(*) FROM river_meta.river_job)<>(SELECT count(*) FROM river.river_job WHERE kind='meta_inbox_v1' OR queue='meta_inbox')
 OR EXISTS(SELECT 1 FROM river.river_queue old LEFT JOIN river_meta.river_queue new ON new.name=old.name
  WHERE old.name='meta_inbox' AND to_jsonb(old) IS DISTINCT FROM to_jsonb(new)) THEN
  RAISE EXCEPTION 'meta cutover row mismatch' USING ERRCODE='22023';
 END IF;
 -- nextval/setval are not transactional; advancing across a rollback merely
 -- leaves a gap. Never lower either high-water mark or recycle a pruned ID.
 PERFORM setval('river_meta.river_job_id_seq',greatest(
  (SELECT last_value FROM river.river_job_id_seq),
  (SELECT last_value FROM river_meta.river_job_id_seq),
  coalesce((SELECT max(id) FROM river_meta.river_job),1),
  coalesce((SELECT max(job_id) FROM meta_inbox.events),1)),true);
END $$;

-- Existing SECURITY DEFINER functions keep their owner and ACL when replaced.
-- Static copies below change only the River relation; do not derive executable
-- migration SQL from mutable function definitions in the target database.
CREATE OR REPLACE FUNCTION meta_inbox.complete_event(p_batch uuid,p_event uuid,p_key text,p_nonce bytea,p_cipher bytea,p_job bigint) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE e meta_inbox.events%ROWTYPE;
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_ingress');
 SELECT e1.* INTO e FROM meta_inbox.events e1 WHERE e1.id=p_event AND NOT completed AND admission_xid=pg_current_xact_id()
  AND EXISTS(SELECT 1 FROM meta_inbox.batch_events m JOIN meta_inbox.batches b ON b.id=m.batch_id
   WHERE m.event_id=e1.id AND b.id=p_batch AND NOT b.finalized AND b.admission_xid=pg_current_xact_id());
 IF e.id IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_-]{1,64}$' OR p_nonce IS NULL OR octet_length(p_nonce)<>12
  OR p_cipher IS NULL OR octet_length(p_cipher) NOT BETWEEN 17 AND 4194320 THEN RAISE EXCEPTION 'invalid meta envelope' USING ERRCODE='22023'; END IF;
 IF e.disposition='ROUTED' THEN
  IF p_job IS NULL OR NOT EXISTS(SELECT 1 FROM river_meta.river_job j WHERE j.id=p_job AND j.kind='meta_inbox_v1' AND j.queue='meta_inbox' AND j.unique_key IS NULL
    AND j.args=jsonb_build_object('event_id',e.id::text,'version',1) AND j.args->>'version'='1') THEN RAISE EXCEPTION 'invalid meta job' USING ERRCODE='22023'; END IF;
  INSERT INTO meta_private.event_bodies(event_id,tenant_id,store_id,key_id,nonce,ciphertext) VALUES(e.id,e.tenant_id,e.store_id,p_key,p_nonce,p_cipher);
 ELSE
  IF p_job IS NOT NULL THEN RAISE EXCEPTION 'invalid meta job' USING ERRCODE='22023'; END IF;
  INSERT INTO meta_private.quarantine_bodies(event_id,key_id,nonce,ciphertext) VALUES(e.id,p_key,p_nonce,p_cipher);
 END IF;
 UPDATE meta_inbox.events SET completed=true,job_id=p_job WHERE id=e.id;
END $$;

CREATE OR REPLACE FUNCTION meta_inbox.check_event(p_event uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE e meta_inbox.events%ROWTYPE;
BEGIN
 SELECT * INTO e FROM meta_inbox.events WHERE id=p_event;
 IF e.id IS NULL OR NOT e.completed THEN RAISE EXCEPTION 'incomplete meta event' USING ERRCODE='22023'; END IF;
 IF e.disposition='ROUTED' THEN
  IF NOT EXISTS(SELECT 1 FROM meta_private.event_bodies WHERE event_id=e.id AND tenant_id=e.tenant_id AND store_id=e.store_id)
   OR EXISTS(SELECT 1 FROM meta_private.quarantine_bodies WHERE event_id=e.id)
   OR NOT EXISTS(SELECT 1 FROM river_meta.river_job j WHERE j.id=e.job_id AND j.kind='meta_inbox_v1' AND j.queue='meta_inbox' AND j.unique_key IS NULL
    AND j.args=jsonb_build_object('event_id',e.id::text,'version',1) AND j.args->>'version'='1')
   OR NOT EXISTS(SELECT 1 FROM meta_inbox.routes r JOIN control.tenants t ON t.id=r.tenant_id JOIN control.stores s ON s.tenant_id=t.id AND s.id=r.store_id
    JOIN integration.bindings b ON b.id=r.binding_id
    WHERE r.id=e.route_id AND r.tenant_id=e.tenant_id AND r.store_id=e.store_id AND r.route_epoch=e.route_epoch AND r.binding_id=e.binding_id AND r.binding_version=e.binding_version
     AND r.enabled AND r.proof_expires>clock_timestamp() AND t.active AND s.active AND b.enabled AND b.semantic_version=e.binding_version) THEN
    RAISE EXCEPTION 'incomplete meta event' USING ERRCODE='22023'; END IF;
 ELSIF NOT EXISTS(SELECT 1 FROM meta_private.quarantine_bodies WHERE event_id=e.id) OR EXISTS(SELECT 1 FROM meta_private.event_bodies WHERE event_id=e.id) OR e.job_id IS NOT NULL THEN
  RAISE EXCEPTION 'incomplete meta event' USING ERRCODE='22023';
 END IF;
END $$;

CREATE OR REPLACE FUNCTION meta_inbox.purgeable(p_event uuid) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 RETURN EXISTS(SELECT 1 FROM meta_inbox.events e WHERE e.id=p_event AND e.completed AND e.terminal_at IS NOT NULL
  AND NOT EXISTS(SELECT 1 FROM river_meta.river_job j WHERE j.id=e.job_id AND j.state NOT IN ('completed','cancelled','discarded')));
END $$;

CREATE OR REPLACE FUNCTION meta_inbox.lock_purgeable(p_event uuid) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE e meta_inbox.events%ROWTYPE; job_state text;
BEGIN
 SELECT * INTO e FROM meta_inbox.events WHERE id=p_event AND completed AND terminal_at IS NOT NULL;
 IF e.id IS NULL THEN RETURN false; END IF;
 IF e.job_id IS NULL THEN RETURN true; END IF;
 SELECT state::text INTO job_state FROM river_meta.river_job WHERE id=e.job_id FOR SHARE SKIP LOCKED;
 IF NOT FOUND THEN RETURN NOT EXISTS(SELECT 1 FROM river_meta.river_job WHERE id=e.job_id); END IF;
 RETURN job_state IN ('completed','cancelled','discarded');
END $$;

CREATE OR REPLACE FUNCTION meta_inbox.social_source(p_event uuid,p_job bigint,p_attempt integer,p_history boolean)
RETURNS TABLE(outcome text,source meta_inbox.events)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE e meta_inbox.events%ROWTYPE;r meta_inbox.routes%ROWTYPE;b integration.bindings%ROWTYPE;
 tenant_active boolean;store_active boolean;terminal text;j record;
BEGIN
 IF p_event IS NULL OR p_job IS NULL OR p_job<=0 OR p_attempt IS NULL OR p_attempt<=0
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid meta consumer job' USING ERRCODE='22023';
 END IF;
 SELECT * INTO e FROM meta_inbox.events WHERE id=p_event;
 IF e.id IS NULL OR NOT e.completed OR NOT e.is_primary OR e.disposition<>'ROUTED' OR e.job_id IS DISTINCT FROM p_job THEN
  RAISE EXCEPTION 'invalid meta consumer job' USING ERRCODE='22023';
 END IF;
 IF p_history THEN
  terminal:=meta_inbox.social_terminal(e);
  IF terminal IS NOT NULL THEN RETURN QUERY SELECT terminal,e;RETURN;END IF;
 END IF;
 SELECT active INTO tenant_active FROM control.tenants WHERE id=e.tenant_id FOR SHARE;
 SELECT active INTO store_active FROM control.stores WHERE tenant_id=e.tenant_id AND id=e.store_id FOR SHARE;
 SELECT * INTO b FROM integration.bindings WHERE tenant_id=e.tenant_id AND store_id=e.store_id AND id=e.binding_id FOR SHARE;
 SELECT * INTO r FROM meta_inbox.routes WHERE id=e.route_id FOR SHARE;
 SELECT * INTO e FROM meta_inbox.events WHERE id=p_event FOR UPDATE;
 IF p_history THEN
  terminal:=meta_inbox.social_terminal(e);
  IF terminal IS NOT NULL THEN RETURN QUERY SELECT terminal,e;RETURN;END IF;
 END IF;
 IF NOT coalesce(tenant_active AND store_active AND b.enabled AND b.semantic_version=e.binding_version
  AND r.enabled AND r.proof_expires>clock_timestamp() AND r.tenant_id=e.tenant_id AND r.store_id=e.store_id
  AND r.app_id=e.app_id AND r.object=e.object AND r.asset_id=e.asset_id AND r.binding_id=e.binding_id
  AND r.binding_version=e.binding_version AND r.route_epoch=e.route_epoch,false) THEN
  RETURN QUERY SELECT 'STALE'::text,e;RETURN;
 END IF;
 SELECT * INTO j FROM river_meta.river_job WHERE id=p_job FOR SHARE;
 IF j.id IS NULL OR j.kind<>'meta_inbox_v1' OR j.queue<>'meta_inbox' OR j.unique_key IS NOT NULL
  OR j.args IS DISTINCT FROM jsonb_build_object('event_id',e.id::text,'version',1) OR j.args->>'version'<>'1'
  OR j.state<>'running' OR j.attempt<>p_attempt THEN
  RAISE EXCEPTION 'invalid meta consumer job' USING ERRCODE='22023';
 END IF;
 -- The job lock may wait; proof time must be checked again after that wait.
 IF r.proof_expires<=clock_timestamp() THEN RETURN QUERY SELECT 'STALE'::text,e;RETURN;END IF;
 RETURN QUERY SELECT 'READY'::text,e;
END $$;
-- Historical rows have already been validated and copied; do not forge their
-- original admission_xid. The new guards apply only after that validated copy.
DROP TRIGGER meta_job_family ON river.river_job;
DROP TRIGGER meta_job_commit ON river.river_job;
CREATE OR REPLACE FUNCTION meta_inbox.guard_job_family() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.kind<>'meta_inbox_v1' OR NEW.queue<>'meta_inbox' OR NEW.unique_key IS NOT NULL
  OR NEW.args IS NULL OR jsonb_typeof(NEW.args)<>'object'
  OR NEW.args->>'event_id' IS NULL OR NEW.args->>'event_id' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR NEW.args IS DISTINCT FROM jsonb_build_object('event_id',NEW.args->>'event_id','version',1)
  OR NEW.args->>'version' IS DISTINCT FROM '1' THEN
  RAISE EXCEPTION 'invalid meta job' USING ERRCODE='22023';
 END IF;
 IF TG_OP='INSERT' THEN
  PERFORM meta_inbox.require_authority('commerce_meta_ingress');
 ELSIF OLD.kind<>'meta_inbox_v1' OR OLD.queue<>'meta_inbox'
  OR NEW.id IS DISTINCT FROM OLD.id OR NEW.kind IS DISTINCT FROM OLD.kind OR NEW.queue IS DISTINCT FROM OLD.queue
  OR NEW.args IS DISTINCT FROM OLD.args OR NEW.unique_key IS DISTINCT FROM OLD.unique_key THEN
  RAISE EXCEPTION 'immutable meta job identity' USING ERRCODE='22023';
 END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION meta_inbox.guard_job_link() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM river_meta.river_job j JOIN meta_inbox.events e ON e.job_id=j.id
  WHERE j.id=NEW.id AND j.kind='meta_inbox_v1' AND j.queue='meta_inbox' AND j.unique_key IS NULL
   AND e.completed AND e.disposition='ROUTED' AND e.admission_xid=pg_current_xact_id()
   AND j.args=jsonb_build_object('event_id',e.id::text,'version',1) AND j.args->>'version'='1') THEN
  RAISE EXCEPTION 'unlinked meta job' USING ERRCODE='22023';
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER meta_job_family BEFORE INSERT OR UPDATE ON river_meta.river_job FOR EACH ROW EXECUTE FUNCTION meta_inbox.guard_job_family();
CREATE CONSTRAINT TRIGGER meta_job_commit AFTER INSERT ON river_meta.river_job DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION meta_inbox.guard_job_link();

DELETE FROM river.river_job WHERE kind='meta_inbox_v1' OR queue='meta_inbox';
DELETE FROM river.river_queue WHERE name='meta_inbox';

CREATE FUNCTION meta_inbox.reject_legacy_job() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.kind='meta_inbox_v1' OR NEW.queue='meta_inbox'
  OR (TG_OP='UPDATE' AND (OLD.kind='meta_inbox_v1' OR OLD.queue='meta_inbox')) THEN
  RAISE EXCEPTION 'legacy meta lane disabled' USING ERRCODE='22023';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION meta_inbox.reject_legacy_job() OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION meta_inbox.reject_legacy_job() FROM PUBLIC;
CREATE TRIGGER meta_job_legacy BEFORE INSERT OR UPDATE ON river.river_job FOR EACH ROW EXECUTE FUNCTION meta_inbox.reject_legacy_job();

-- Table-level REVOKE does not revoke separately granted column privileges.
REVOKE ALL ON river.river_job FROM commerce_meta_ingress,commerce_meta_writer;
REVOKE UPDATE(kind) ON river.river_job FROM commerce_meta_ingress;
REVOKE UPDATE(id) ON river.river_job FROM commerce_meta_writer;
REVOKE ALL ON river.river_job_id_seq FROM commerce_meta_ingress;
REVOKE ALL ON SCHEMA river FROM commerce_meta_ingress,commerce_meta_writer;
REVOKE ALL ON FUNCTION meta_inbox.runtime_ready() FROM commerce_worker;
REVOKE USAGE ON SCHEMA meta_inbox FROM commerce_worker;

REVOKE ALL ON ALL TABLES IN SCHEMA river_meta FROM PUBLIC,commerce_worker;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA river_meta FROM PUBLIC,commerce_worker;
GRANT USAGE ON SCHEMA river_meta TO commerce_meta_ingress,commerce_meta_writer,commerce_meta_worker;
GRANT SELECT,INSERT,UPDATE(kind) ON river_meta.river_job TO commerce_meta_ingress;
GRANT USAGE ON SEQUENCE river_meta.river_job_id_seq TO commerce_meta_ingress;
GRANT SELECT,UPDATE(id) ON river_meta.river_job TO commerce_meta_writer;
GRANT USAGE ON SCHEMA meta_inbox TO commerce_meta_worker;
GRANT EXECUTE ON FUNCTION meta_inbox.runtime_ready() TO commerce_meta_ingress,commerce_meta_worker;

CREATE OR REPLACE FUNCTION meta_inbox.runtime_ready()
RETURNS boolean LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE guards_ready boolean;
BEGIN
 SELECT count(*)=5 INTO guards_ready
 FROM (VALUES
  ('river_meta.river_job','meta_job_family','meta_inbox.guard_job_family()',23,false),
  ('river_meta.river_job','meta_job_commit','meta_inbox.guard_job_link()',5,true),
  ('river.river_job','meta_job_legacy','meta_inbox.reject_legacy_job()',23,false),
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
 -- Terminal linkage may have been pruned, but foreign families never pass.
 RETURN NOT EXISTS (
  SELECT 1 FROM river_meta.river_job j WHERE j.kind<>'meta_inbox_v1' OR j.queue<>'meta_inbox'
   OR (j.state NOT IN ('completed','cancelled','discarded')
    AND (j.unique_key IS NOT NULL OR NOT EXISTS (
     SELECT 1 FROM meta_inbox.events e WHERE e.job_id=j.id AND e.completed AND e.disposition='ROUTED'
      AND j.args=jsonb_build_object('event_id',e.id::text,'version',1) AND j.args->>'version'='1'
    )))
 );
END $$;
