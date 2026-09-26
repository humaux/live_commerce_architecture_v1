-- T07 read-side projection only. No send policy, identity linking or commerce
-- writer. See contracts/meta-consumer-v1.md for the immutable source/AAD path.
CREATE ROLE commerce_meta_consumer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT USAGE ON SCHEMA meta_inbox TO commerce_meta_consumer;
CREATE SCHEMA social;
REVOKE ALL ON SCHEMA social FROM PUBLIC;
GRANT USAGE ON SCHEMA social TO commerce_meta_writer;

ALTER TABLE meta_inbox.events DROP CONSTRAINT events_terminal_reason_check;
ALTER TABLE meta_inbox.events ADD CONSTRAINT events_terminal_reason_check
 CHECK(terminal_reason IN ('reviewed_rejected','retention_discarded','processed'));
ALTER TABLE meta_inbox.audit_events DROP CONSTRAINT audit_events_action_check;
ALTER TABLE meta_inbox.audit_events ADD CONSTRAINT audit_events_action_check
 CHECK(action IN ('activate','disable','reviewed_rejected','retention_discarded','processed'));
CREATE UNIQUE INDEX meta_processed_audit ON meta_inbox.audit_events(event_id) WHERE action='processed';
GRANT SELECT(event_id,action,actor,evidence_hash) ON meta_inbox.audit_events TO commerce_meta_writer;

CREATE TABLE social.conversations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,
 app_id text NOT NULL CHECK(app_id ~ '^[0-9]{1,40}$'),
 object text NOT NULL CHECK(object IN ('page','instagram')),
 asset_id text NOT NULL CHECK(asset_id ~ '^[0-9]{1,40}$'),
 peer_key text NOT NULL CHECK(peer_key ~ '^[0-9a-f]{64}$'),
 next_seq bigint NOT NULL DEFAULT 0 CHECK(next_seq>=0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(tenant_id,store_id,app_id,object,asset_id,peer_key),
 UNIQUE(id,tenant_id,store_id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
CREATE TABLE social.messages (
 event_id uuid PRIMARY KEY,tenant_id uuid NOT NULL,store_id uuid NOT NULL,
 conversation_id uuid NOT NULL,server_seq bigint NOT NULL CHECK(server_seq>0),
 event_key text NOT NULL CHECK(event_key ~ '^[0-9a-f]{64}$'),
 occurred_at timestamptz,received_at timestamptz NOT NULL,
 -- Persist the exact running attempt for the deferred COMMIT check, not a GUC
 -- ticket that a caller can replace. Original job id stays in the source event.
 consumer_attempt integer NOT NULL CHECK(consumer_attempt>0),
 key_id text NOT NULL CHECK(key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
 nonce bytea NOT NULL CHECK(octet_length(nonce)=12),
 ciphertext bytea NOT NULL CHECK(octet_length(ciphertext) BETWEEN 17 AND 4194320),
 UNIQUE(conversation_id,server_seq),
 FOREIGN KEY(event_id,tenant_id,store_id) REFERENCES meta_inbox.events(id,tenant_id,store_id),
 FOREIGN KEY(conversation_id,tenant_id,store_id) REFERENCES social.conversations(id,tenant_id,store_id)
);
CREATE TABLE social.comment_events (
 event_id uuid PRIMARY KEY,tenant_id uuid NOT NULL,store_id uuid NOT NULL,
 comment_key text NOT NULL CHECK(comment_key ~ '^[0-9a-f]{64}$'),
 kind text NOT NULL CHECK(kind IN ('page_comment_add','page_comment_edit','page_comment_remove','instagram_comment','instagram_live_comment')),
 occurred_at timestamptz,received_at timestamptz NOT NULL,
 consumer_attempt integer NOT NULL CHECK(consumer_attempt>0),
 key_id text NOT NULL CHECK(key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
 nonce bytea NOT NULL CHECK(octet_length(nonce)=12),
 ciphertext bytea NOT NULL CHECK(octet_length(ciphertext) BETWEEN 17 AND 4194320),
 FOREIGN KEY(event_id,tenant_id,store_id) REFERENCES meta_inbox.events(id,tenant_id,store_id)
);
CREATE INDEX social_comment_history ON social.comment_events(tenant_id,store_id,comment_key,received_at,event_id);
ALTER TABLE social.conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.conversations FORCE ROW LEVEL SECURITY;
ALTER TABLE social.messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.messages FORCE ROW LEVEL SECURITY;
ALTER TABLE social.comment_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.comment_events FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON social.conversations,social.messages,social.comment_events TO commerce_meta_writer;
GRANT UPDATE(next_seq) ON social.conversations TO commerce_meta_writer;
CREATE POLICY social_conversation_read ON social.conversations FOR SELECT TO commerce_meta_writer USING(true);
CREATE POLICY social_conversation_insert ON social.conversations FOR INSERT TO commerce_meta_writer WITH CHECK(true);
CREATE POLICY social_conversation_sequence ON social.conversations FOR UPDATE TO commerce_meta_writer USING(true) WITH CHECK(true);
CREATE POLICY social_message_read ON social.messages FOR SELECT TO commerce_meta_writer USING(true);
CREATE POLICY social_message_insert ON social.messages FOR INSERT TO commerce_meta_writer WITH CHECK(true);
CREATE POLICY social_comment_read ON social.comment_events FOR SELECT TO commerce_meta_writer USING(true);
CREATE POLICY social_comment_insert ON social.comment_events FOR INSERT TO commerce_meta_writer WITH CHECK(true);

-- Extend the one shared authority guard, including reverse mixed-membership
-- rejection in ingress/registrar/curator. Never trust the SECURITY DEFINER user.
CREATE OR REPLACE FUNCTION meta_inbox.require_authority(p_role text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF p_role IS NULL OR p_role NOT IN ('commerce_meta_ingress','commerce_meta_registrar','commerce_meta_curator','commerce_meta_consumer')
 OR current_setting('role')<>'none'
 OR NOT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname=session_user AND r.rolcanlogin
  AND NOT(r.rolsuper OR r.rolbypassrls OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication)
  AND pg_has_role(r.oid,p_role,'USAGE') AND NOT pg_has_role(r.oid,p_role,'SET'))
 OR EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname LIKE 'commerce\_%' ESCAPE '\'
  AND r.rolname<>p_role AND pg_has_role(session_user,r.oid,'MEMBER'))
 OR EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname LIKE 'pg\_%' ESCAPE '\'
  AND pg_has_role(session_user,r.oid,'MEMBER'))
 OR EXISTS(SELECT 1 FROM pg_roles r WHERE pg_has_role(session_user,r.oid,'SET') AND
  (r.rolsuper OR r.rolbypassrls OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication
   OR EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspowner=r.oid AND n.nspname NOT IN ('pg_catalog','information_schema'))
   OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relowner=r.oid AND n.nspname NOT IN ('pg_catalog','information_schema'))
   OR EXISTS(SELECT 1 FROM pg_proc f JOIN pg_namespace n ON n.oid=f.pronamespace WHERE f.proowner=r.oid AND n.nspname NOT IN ('pg_catalog','information_schema')))) THEN
  RAISE EXCEPTION 'meta authority denied' USING ERRCODE='42501';
 END IF;
END $$;

-- Historical idempotency is independent of mutable routes and short-retention
-- source bodies. This helper is private; callers never see its event composite.
CREATE FUNCTION meta_inbox.social_terminal(e meta_inbox.events) RETURNS text
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF e.terminal_reason='processed' THEN
  IF e.terminal_evidence IS DISTINCT FROM e.payload_hash
   OR (SELECT count(*) FROM (
    SELECT m.event_id FROM social.messages m JOIN social.conversations c ON c.id=m.conversation_id
     WHERE m.event_id=e.id AND m.tenant_id=e.tenant_id AND m.store_id=e.store_id AND m.event_key=e.event_key
      AND c.tenant_id=e.tenant_id AND c.store_id=e.store_id AND c.app_id=e.app_id AND c.object=e.object AND c.asset_id=e.asset_id
      AND e.kind IN ('page_message','instagram_message')
    UNION ALL
    SELECT c.event_id FROM social.comment_events c WHERE c.event_id=e.id AND c.tenant_id=e.tenant_id
     AND c.store_id=e.store_id AND c.kind=e.kind
   ) facts)<>1
   OR (SELECT count(*) FROM social.messages WHERE event_id=e.id)+(SELECT count(*) FROM social.comment_events WHERE event_id=e.id)<>1 THEN
    RAISE EXCEPTION 'meta projection history unavailable' USING ERRCODE='XX000';
  END IF;
  RETURN 'ALREADY';
 ELSIF e.terminal_reason IS NOT NULL THEN RETURN 'REVIEWED';
 END IF;
 RETURN NULL;
END $$;

-- Single lock order shared by load, finish and the deferred guard:
-- tenant -> store -> binding -> route -> event -> River row. Holding the running
-- row through COMMIT prevents an old rescued attempt materializing late.
CREATE FUNCTION meta_inbox.social_source(p_event uuid,p_job bigint,p_attempt integer,p_history boolean)
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
 SELECT * INTO j FROM river.river_job WHERE id=p_job FOR SHARE;
 IF j.id IS NULL OR j.kind<>'meta_inbox_v1' OR j.queue<>'meta_inbox' OR j.unique_key IS NOT NULL
  OR j.args IS DISTINCT FROM jsonb_build_object('event_id',e.id::text,'version',1) OR j.args->>'version'<>'1'
  OR j.state<>'running' OR j.attempt<>p_attempt THEN
  RAISE EXCEPTION 'invalid meta consumer job' USING ERRCODE='22023';
 END IF;
 -- The job lock may wait; proof time must be checked again after that wait.
 IF r.proof_expires<=clock_timestamp() THEN RETURN QUERY SELECT 'STALE'::text,e;RETURN;END IF;
 RETURN QUERY SELECT 'READY'::text,e;
END $$;

CREATE FUNCTION meta_inbox.load_social_event(p_event uuid,p_job bigint,p_attempt integer)
RETURNS TABLE(outcome text,app_id text,object text,asset_id text,kind text,event_key text,payload_hash text,
 tenant_id uuid,store_id uuid,route_id uuid,route_epoch bigint,key_id text,nonce bytea,ciphertext bytea)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE status text;e meta_inbox.events%ROWTYPE;body meta_private.event_bodies%ROWTYPE;gate record;
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_consumer');
 SELECT * INTO gate FROM meta_inbox.social_source(p_event,p_job,p_attempt,true);
 status:=gate.outcome;e:=gate.source;
 IF status<>'READY' THEN
  RETURN QUERY SELECT status,NULL::text,NULL::text,NULL::text,NULL::text,NULL::text,NULL::text,
   NULL::uuid,NULL::uuid,NULL::uuid,NULL::bigint,NULL::text,NULL::bytea,NULL::bytea;RETURN;
 END IF;
 SELECT * INTO body FROM meta_private.event_bodies WHERE event_id=e.id AND tenant_id=e.tenant_id AND store_id=e.store_id;
 IF body.event_id IS NULL THEN RAISE EXCEPTION 'meta projection body unavailable' USING ERRCODE='XX000';END IF;
 RETURN QUERY SELECT status,e.app_id,e.object,e.asset_id,e.kind,e.event_key,e.payload_hash,
  e.tenant_id,e.store_id,e.route_id,e.route_epoch,body.key_id,body.nonce,body.ciphertext;
END $$;

CREATE FUNCTION meta_inbox.finish_social_event(p_event uuid,p_job bigint,p_attempt integer,p_family text,p_subject text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE status text;e meta_inbox.events%ROWTYPE;body meta_private.event_bodies%ROWTYPE;c social.conversations%ROWTYPE;gate record;
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_consumer');
 IF p_family IS NULL OR p_family NOT IN ('message','comment') OR p_subject IS NULL OR p_subject !~ '^[0-9a-f]{64}$' THEN
  RAISE EXCEPTION 'invalid meta projection' USING ERRCODE='22023';END IF;
 SELECT * INTO gate FROM meta_inbox.social_source(p_event,p_job,p_attempt,true);
 status:=gate.outcome;e:=gate.source;
 IF (p_family='message') IS DISTINCT FROM (e.kind IN ('page_message','instagram_message')) THEN
  RAISE EXCEPTION 'invalid meta projection family' USING ERRCODE='22023';END IF;
 IF status='ALREADY' THEN
  IF (p_family='message' AND NOT EXISTS(SELECT 1 FROM social.messages m JOIN social.conversations v ON v.id=m.conversation_id WHERE m.event_id=e.id AND v.peer_key=p_subject))
   OR (p_family='comment' AND NOT EXISTS(SELECT 1 FROM social.comment_events WHERE event_id=e.id AND comment_key=p_subject)) THEN
   RAISE EXCEPTION 'invalid meta projection identity' USING ERRCODE='22023';END IF;
  RETURN;
 END IF;
 IF status<>'READY' THEN RAISE EXCEPTION 'meta projection policy denied' USING ERRCODE='PT409';END IF;
 SELECT * INTO body FROM meta_private.event_bodies WHERE event_id=e.id AND tenant_id=e.tenant_id AND store_id=e.store_id;
 IF body.event_id IS NULL THEN RAISE EXCEPTION 'meta projection body unavailable' USING ERRCODE='XX000';END IF;
 IF p_family='message' THEN
  INSERT INTO social.conversations(tenant_id,store_id,app_id,object,asset_id,peer_key)
   VALUES(e.tenant_id,e.store_id,e.app_id,e.object,e.asset_id,p_subject)
   ON CONFLICT(tenant_id,store_id,app_id,object,asset_id,peer_key) DO NOTHING;
  UPDATE social.conversations SET next_seq=next_seq+1
   WHERE tenant_id=e.tenant_id AND store_id=e.store_id AND app_id=e.app_id AND object=e.object AND asset_id=e.asset_id AND peer_key=p_subject RETURNING * INTO c;
  INSERT INTO social.messages(event_id,tenant_id,store_id,conversation_id,server_seq,event_key,occurred_at,received_at,consumer_attempt,key_id,nonce,ciphertext)
   VALUES(e.id,e.tenant_id,e.store_id,c.id,c.next_seq,e.event_key,e.occurred_at,e.created_at,p_attempt,body.key_id,body.nonce,body.ciphertext);
 ELSE
  INSERT INTO social.comment_events(event_id,tenant_id,store_id,comment_key,kind,occurred_at,received_at,consumer_attempt,key_id,nonce,ciphertext)
   VALUES(e.id,e.tenant_id,e.store_id,p_subject,e.kind,e.occurred_at,e.created_at,p_attempt,body.key_id,body.nonce,body.ciphertext);
 END IF;
 UPDATE meta_inbox.events SET terminal_reason='processed',terminal_at=clock_timestamp(),terminal_actor=session_user,terminal_evidence=e.payload_hash WHERE id=e.id;
 INSERT INTO meta_inbox.audit_events(event_id,action,evidence_hash) VALUES(e.id,'processed',e.payload_hash);
END $$;

-- The transaction must still have a current route and running attempt at its
-- commit boundary. This checks the stored fact (not just NEW or a caller GUC).
CREATE FUNCTION meta_inbox.guard_social_insert() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE e meta_inbox.events%ROWTYPE;status text;
BEGIN
 SELECT * INTO e FROM meta_inbox.events WHERE id=NEW.event_id;
 IF e.terminal_reason IS DISTINCT FROM 'processed' OR e.terminal_evidence IS DISTINCT FROM e.payload_hash
  OR e.terminal_actor IS DISTINCT FROM session_user OR meta_inbox.social_terminal(e) IS DISTINCT FROM 'ALREADY'
  OR NOT EXISTS(SELECT 1 FROM meta_inbox.audit_events a WHERE a.event_id=e.id AND a.action='processed' AND a.actor=session_user AND a.evidence_hash=e.payload_hash) THEN
  RAISE EXCEPTION 'incomplete meta projection' USING ERRCODE='22023';END IF;
 SELECT s.outcome INTO status FROM meta_inbox.social_source(e.id,e.job_id,NEW.consumer_attempt,false) s;
 IF status<>'READY' THEN RAISE EXCEPTION 'meta projection policy denied' USING ERRCODE='PT409';END IF;
 IF TG_TABLE_NAME='messages' THEN
  IF NOT EXISTS(SELECT 1 FROM social.messages m JOIN meta_private.event_bodies b ON b.event_id=m.event_id
   JOIN social.conversations c ON c.id=m.conversation_id
   WHERE m.event_id=e.id AND m.tenant_id=e.tenant_id AND m.store_id=e.store_id AND m.event_key=e.event_key
    AND m.occurred_at IS NOT DISTINCT FROM e.occurred_at AND m.received_at=e.created_at
    AND m.consumer_attempt=NEW.consumer_attempt AND m.server_seq<=c.next_seq
    AND b.tenant_id=e.tenant_id AND b.store_id=e.store_id AND m.key_id=b.key_id AND m.nonce=b.nonce AND m.ciphertext=b.ciphertext) THEN
   RAISE EXCEPTION 'incomplete meta projection' USING ERRCODE='22023';END IF;
 ELSE
  IF NOT EXISTS(SELECT 1 FROM social.comment_events c JOIN meta_private.event_bodies b ON b.event_id=c.event_id
   WHERE c.event_id=e.id AND c.tenant_id=e.tenant_id AND c.store_id=e.store_id AND c.kind=e.kind
    AND c.occurred_at IS NOT DISTINCT FROM e.occurred_at AND c.received_at=e.created_at
    AND c.consumer_attempt=NEW.consumer_attempt AND b.tenant_id=e.tenant_id AND b.store_id=e.store_id
    AND c.key_id=b.key_id AND c.nonce=b.nonce AND c.ciphertext=b.ciphertext) THEN
   RAISE EXCEPTION 'incomplete meta projection' USING ERRCODE='22023';END IF;
 END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER social_message_commit AFTER INSERT ON social.messages DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION meta_inbox.guard_social_insert();
CREATE CONSTRAINT TRIGGER social_comment_commit AFTER INSERT ON social.comment_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION meta_inbox.guard_social_insert();

DO $$ DECLARE f record;BEGIN
 FOR f IN SELECT p.oid::regprocedure AS signature FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='meta_inbox' LOOP
  EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_meta_writer',f.signature);
  EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f.signature);
 END LOOP;
END $$;
GRANT EXECUTE ON FUNCTION meta_inbox.load_social_event(uuid,bigint,integer),meta_inbox.finish_social_event(uuid,bigint,integer,text,text) TO commerce_meta_consumer;
