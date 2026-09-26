-- Meta webhook admission only: no public receiver, consumer, OAuth proof issuer
-- or automatic transfer. Permanent receipts outlive ciphertext and River jobs.
CREATE ROLE commerce_meta_ingress NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE commerce_meta_registrar NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE commerce_meta_curator NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE commerce_meta_writer NOLOGIN NOINHERIT NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE SCHEMA meta_inbox;
CREATE SCHEMA meta_private;
REVOKE ALL ON SCHEMA meta_inbox,meta_private FROM PUBLIC;
GRANT USAGE ON SCHEMA meta_inbox TO commerce_meta_ingress,commerce_meta_registrar,commerce_meta_curator,commerce_meta_writer;
GRANT USAGE ON SCHEMA meta_private,control,integration,river TO commerce_meta_writer;

CREATE TABLE meta_inbox.asset_owners (
 object text NOT NULL CHECK(object IN ('page','instagram')),
 asset_id text NOT NULL CHECK(asset_id ~ '^[0-9]{1,64}$'),
 tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 PRIMARY KEY(object,asset_id), UNIQUE(object,asset_id,tenant_id,store_id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
CREATE TABLE meta_inbox.routes (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 app_id text NOT NULL CHECK(app_id ~ '^[0-9]{1,64}$'),
 object text NOT NULL, asset_id text NOT NULL,
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,binding_id uuid NOT NULL,
 binding_version bigint NOT NULL CHECK(binding_version>0),
 route_epoch bigint NOT NULL CHECK(route_epoch>0),
 proof_hash text NOT NULL CHECK(proof_hash ~ '^[0-9a-f]{64}$'),
 proof_expires timestamptz NOT NULL,enabled boolean NOT NULL DEFAULT true,
 UNIQUE(app_id,object,asset_id),
 FOREIGN KEY(object,asset_id,tenant_id,store_id) REFERENCES meta_inbox.asset_owners(object,asset_id,tenant_id,store_id),
 FOREIGN KEY(tenant_id,store_id,binding_id) REFERENCES integration.bindings(tenant_id,store_id,id)
);
CREATE TABLE meta_inbox.batches (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 app_id text NOT NULL CHECK(app_id ~ '^[0-9]{1,64}$'),
 object text NOT NULL CHECK(object IN ('page','instagram')),
 body_hash text NOT NULL CHECK(body_hash ~ '^[0-9a-f]{64}$'),
 unit_count integer NOT NULL CHECK(unit_count BETWEEN 1 AND 1000),
 finalized boolean NOT NULL DEFAULT false,
 admission_xid xid8 NOT NULL DEFAULT pg_current_xact_id(),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(app_id,object,body_hash)
);
CREATE TABLE meta_inbox.events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 app_id text NOT NULL CHECK(app_id ~ '^[0-9]{1,64}$'),
 object text NOT NULL CHECK(object IN ('page','instagram')),
 event_key text NOT NULL CHECK(event_key ~ '^[0-9a-f]{64}$'),
 payload_hash text NOT NULL CHECK(payload_hash ~ '^[0-9a-f]{64}$'),
 asset_id text NOT NULL CHECK(asset_id='' OR asset_id ~ '^[0-9]{1,64}$'),
 kind text NOT NULL CHECK(kind IN ('page_comment_add','page_comment_edit','page_comment_remove','instagram_comment','instagram_live_comment','page_message','instagram_message','quarantine')),
 is_primary boolean NOT NULL,
 disposition text NOT NULL CHECK(disposition IN ('ROUTED','QUARANTINED')),
 reason text NOT NULL,
 tenant_id uuid,store_id uuid,route_id uuid REFERENCES meta_inbox.routes(id),
 route_epoch bigint,binding_id uuid,binding_version bigint,
 occurred_at timestamptz,job_id bigint UNIQUE,
 completed boolean NOT NULL DEFAULT false,
 admission_xid xid8 NOT NULL DEFAULT pg_current_xact_id(),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 terminal_reason text CHECK(terminal_reason IN ('reviewed_rejected','retention_discarded')),
 terminal_at timestamptz,terminal_actor name,terminal_evidence text,
 UNIQUE(app_id,object,event_key,payload_hash), UNIQUE(id,tenant_id,store_id),
 FOREIGN KEY(tenant_id,store_id,binding_id) REFERENCES integration.bindings(tenant_id,store_id,id),
 CHECK((disposition='ROUTED' AND tenant_id IS NOT NULL AND store_id IS NOT NULL AND route_id IS NOT NULL AND route_epoch>0 AND binding_id IS NOT NULL AND binding_version>0 AND reason='' AND is_primary AND kind<>'quarantine')
   OR (disposition='QUARANTINED' AND tenant_id IS NULL AND store_id IS NULL AND route_id IS NULL AND route_epoch IS NULL AND binding_id IS NULL AND binding_version IS NULL AND job_id IS NULL AND reason<>'')),
 CHECK((terminal_reason IS NULL AND terminal_at IS NULL AND terminal_actor IS NULL AND terminal_evidence IS NULL)
   OR (terminal_reason IS NOT NULL AND terminal_at IS NOT NULL AND terminal_actor IS NOT NULL AND terminal_evidence ~ '^[0-9a-f]{64}$'))
);
CREATE UNIQUE INDEX meta_event_primary ON meta_inbox.events(app_id,object,event_key) WHERE is_primary;
CREATE TABLE meta_inbox.batch_events (
 batch_id uuid NOT NULL REFERENCES meta_inbox.batches(id),
 ordinal integer NOT NULL CHECK(ordinal BETWEEN 1 AND 1000),
 event_id uuid NOT NULL REFERENCES meta_inbox.events(id),
 PRIMARY KEY(batch_id,ordinal)
);
CREATE INDEX meta_batch_event_lookup ON meta_inbox.batch_events(event_id,batch_id);
CREATE TABLE meta_private.raw_bodies (
 batch_id uuid PRIMARY KEY REFERENCES meta_inbox.batches(id),
 key_id text NOT NULL CHECK(key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
 nonce bytea NOT NULL CHECK(octet_length(nonce)=12),
 ciphertext bytea NOT NULL CHECK(octet_length(ciphertext) BETWEEN 17 AND 1048592),
 expires_at timestamptz NOT NULL DEFAULT clock_timestamp()+interval '24 hours'
);
CREATE TABLE meta_private.event_bodies (
 event_id uuid PRIMARY KEY,tenant_id uuid NOT NULL,store_id uuid NOT NULL,
 key_id text NOT NULL CHECK(key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
 nonce bytea NOT NULL CHECK(octet_length(nonce)=12),
 ciphertext bytea NOT NULL CHECK(octet_length(ciphertext) BETWEEN 17 AND 4194320),
 expires_at timestamptz NOT NULL DEFAULT clock_timestamp()+interval '7 days',
 FOREIGN KEY(event_id,tenant_id,store_id) REFERENCES meta_inbox.events(id,tenant_id,store_id)
);
CREATE TABLE meta_private.quarantine_bodies (
 event_id uuid PRIMARY KEY REFERENCES meta_inbox.events(id),
 key_id text NOT NULL CHECK(key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
 nonce bytea NOT NULL CHECK(octet_length(nonce)=12),
 ciphertext bytea NOT NULL CHECK(octet_length(ciphertext) BETWEEN 17 AND 4194320),
 expires_at timestamptz NOT NULL DEFAULT clock_timestamp()+interval '24 hours'
);
CREATE INDEX meta_raw_expiry ON meta_private.raw_bodies(expires_at,batch_id);
CREATE INDEX meta_event_expiry ON meta_private.event_bodies(expires_at,event_id);
CREATE INDEX meta_quarantine_expiry ON meta_private.quarantine_bodies(expires_at,event_id);
CREATE TABLE meta_inbox.audit_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 route_id uuid REFERENCES meta_inbox.routes(id),event_id uuid REFERENCES meta_inbox.events(id),
 action text NOT NULL CHECK(action IN ('activate','disable','reviewed_rejected','retention_discarded')),
 actor name NOT NULL DEFAULT session_user,evidence_hash text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

GRANT SELECT,INSERT ON meta_inbox.asset_owners,meta_inbox.routes,meta_inbox.batches,meta_inbox.events,meta_inbox.batch_events TO commerce_meta_writer;
GRANT UPDATE(binding_version,route_epoch,proof_hash,proof_expires,enabled) ON meta_inbox.routes TO commerce_meta_writer;
GRANT UPDATE(finalized) ON meta_inbox.batches TO commerce_meta_writer;
GRANT UPDATE(job_id,completed,terminal_reason,terminal_at,terminal_actor,terminal_evidence) ON meta_inbox.events TO commerce_meta_writer;
GRANT INSERT ON meta_inbox.audit_events TO commerce_meta_writer;
GRANT USAGE ON SEQUENCE meta_inbox.audit_events_id_seq TO commerce_meta_writer;
GRANT SELECT,INSERT,DELETE ON meta_private.raw_bodies,meta_private.event_bodies,meta_private.quarantine_bodies TO commerce_meta_writer;
-- Row locks require an UPDATE privilege; no body-updating API is exposed.
GRANT UPDATE(batch_id) ON meta_private.raw_bodies TO commerce_meta_writer;
GRANT UPDATE(event_id) ON meta_private.event_bodies,meta_private.quarantine_bodies TO commerce_meta_writer;
ALTER TABLE meta_private.event_bodies ENABLE ROW LEVEL SECURITY;
ALTER TABLE meta_private.event_bodies FORCE ROW LEVEL SECURITY;
CREATE POLICY meta_event_writer_read ON meta_private.event_bodies FOR SELECT TO commerce_meta_writer USING(true);
CREATE POLICY meta_event_writer_insert ON meta_private.event_bodies FOR INSERT TO commerce_meta_writer WITH CHECK(true);
CREATE POLICY meta_event_writer_delete ON meta_private.event_bodies FOR DELETE TO commerce_meta_writer USING(true);
CREATE POLICY meta_event_writer_lock ON meta_private.event_bodies FOR UPDATE TO commerce_meta_writer USING(true) WITH CHECK(false);
GRANT SELECT ON control.tenants,control.stores,integration.bindings TO commerce_meta_writer;
GRANT UPDATE(id) ON control.tenants,control.stores,integration.bindings TO commerce_meta_writer;
CREATE POLICY meta_store_read ON control.stores FOR SELECT TO commerce_meta_writer USING(true);
CREATE POLICY meta_store_lock ON control.stores FOR UPDATE TO commerce_meta_writer USING(true) WITH CHECK(false);
CREATE POLICY meta_binding_read ON integration.bindings FOR SELECT TO commerce_meta_writer USING(true);
CREATE POLICY meta_binding_lock ON integration.bindings FOR UPDATE TO commerce_meta_writer USING(true) WITH CHECK(false);

-- SECURITY DEFINER changes current_user, so authenticate the original login and
-- startup SET ROLE separately. Membership in another authority is always denied.
CREATE FUNCTION meta_inbox.require_authority(p_role text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF p_role NOT IN ('commerce_meta_ingress','commerce_meta_registrar','commerce_meta_curator')
 OR current_setting('role')<>'none'
 OR NOT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname=session_user AND r.rolcanlogin
  AND NOT(r.rolsuper OR r.rolbypassrls OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication)
  AND pg_has_role(r.oid,p_role,'USAGE') AND NOT pg_has_role(r.oid,p_role,'SET'))
 OR EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname LIKE 'commerce\_%' ESCAPE '\'
  AND r.rolname<>p_role AND pg_has_role(session_user,r.oid,'MEMBER'))
 OR EXISTS(SELECT 1 FROM pg_roles r WHERE pg_has_role(session_user,r.oid,'SET') AND
  (r.rolsuper OR r.rolbypassrls OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication
   OR EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspowner=r.oid AND n.nspname NOT IN ('pg_catalog','information_schema'))
   OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relowner=r.oid AND n.nspname NOT IN ('pg_catalog','information_schema'))
   OR EXISTS(SELECT 1 FROM pg_proc f JOIN pg_namespace n ON n.oid=f.pronamespace WHERE f.proowner=r.oid AND n.nspname NOT IN ('pg_catalog','information_schema')))) THEN
  RAISE EXCEPTION 'meta authority denied' USING ERRCODE='42501';
 END IF;
END $$;

CREATE FUNCTION meta_inbox.activate_route(p_app text,p_object text,p_asset text,p_tenant uuid,p_store uuid,p_binding uuid,p_version bigint,p_proof text,p_expires timestamptz,p_epoch bigint)
RETURNS TABLE(route_id uuid,route_epoch bigint) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE r meta_inbox.routes%ROWTYPE; o meta_inbox.asset_owners%ROWTYPE;
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_registrar');
 IF p_app IS NULL OR p_app !~ '^[0-9]{1,64}$' OR p_object IS NULL OR p_object NOT IN ('page','instagram') OR p_asset IS NULL OR p_asset !~ '^[0-9]{1,64}$'
  OR p_tenant IS NULL OR p_store IS NULL OR p_binding IS NULL OR p_version IS NULL OR p_version<=0
  OR p_proof IS NULL OR p_proof !~ '^[0-9a-f]{64}$' OR p_expires IS NULL OR NOT isfinite(p_expires)
  OR p_epoch IS NULL OR p_epoch<0 THEN RAISE EXCEPTION 'invalid meta route' USING ERRCODE='22023'; END IF;
 -- Shared lock order with receiver: scope -> binding -> route. The asset owner
 -- advisory lock serializes first ownership even for two distinct app IDs.
 PERFORM 1 FROM control.tenants WHERE id=p_tenant AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'inactive meta scope' USING ERRCODE='PT409'; END IF;
 PERFORM 1 FROM control.stores WHERE tenant_id=p_tenant AND id=p_store AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'inactive meta scope' USING ERRCODE='PT409'; END IF;
 PERFORM 1 FROM integration.bindings WHERE id=p_binding AND tenant_id=p_tenant AND store_id=p_store AND enabled AND semantic_version=p_version
  AND external_asset_id=p_asset AND provider=CASE p_object WHEN 'page' THEN 'facebook' ELSE 'instagram' END FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid meta binding' USING ERRCODE='PT409'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended(jsonb_build_array('meta-owner',p_object,p_asset)::text,0));
 SELECT * INTO o FROM meta_inbox.asset_owners WHERE object=p_object AND asset_id=p_asset;
 IF FOUND AND (o.tenant_id<>p_tenant OR o.store_id<>p_store) THEN RAISE EXCEPTION 'meta owner conflict' USING ERRCODE='PT409'; END IF;
 IF o.asset_id IS NULL THEN INSERT INTO meta_inbox.asset_owners VALUES(p_object,p_asset,p_tenant,p_store); END IF;
 SELECT * INTO r FROM meta_inbox.routes WHERE app_id=p_app AND object=p_object AND asset_id=p_asset FOR UPDATE;
 IF (r.id IS NULL AND p_epoch<>0) OR (r.id IS NOT NULL AND (r.route_epoch<>p_epoch OR r.binding_id<>p_binding OR r.tenant_id<>p_tenant OR r.store_id<>p_store)) THEN
  RAISE EXCEPTION 'meta route conflict' USING ERRCODE='PT409'; END IF;
 IF p_expires<=clock_timestamp() THEN RAISE EXCEPTION 'expired meta proof' USING ERRCODE='PT409'; END IF;
 IF r.id IS NULL THEN
  INSERT INTO meta_inbox.routes(app_id,object,asset_id,tenant_id,store_id,binding_id,binding_version,route_epoch,proof_hash,proof_expires)
   VALUES(p_app,p_object,p_asset,p_tenant,p_store,p_binding,p_version,1,p_proof,p_expires) RETURNING * INTO r;
 ELSE
  UPDATE meta_inbox.routes SET binding_version=p_version,route_epoch=r.route_epoch+1,proof_hash=p_proof,proof_expires=p_expires,enabled=true WHERE id=r.id RETURNING * INTO r;
 END IF;
 INSERT INTO meta_inbox.audit_events(route_id,action,evidence_hash) VALUES(r.id,'activate',p_proof);
 RETURN QUERY SELECT r.id,r.route_epoch;
END $$;

CREATE FUNCTION meta_inbox.disable_route(p_route uuid,p_epoch bigint) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE r meta_inbox.routes%ROWTYPE; next_epoch bigint;
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_registrar');
 SELECT * INTO r FROM meta_inbox.routes WHERE id=p_route;
 IF r.id IS NULL OR p_epoch IS NULL OR p_epoch<=0 THEN RAISE EXCEPTION 'meta route conflict' USING ERRCODE='PT409'; END IF;
 PERFORM 1 FROM control.tenants WHERE id=r.tenant_id FOR SHARE;
 PERFORM 1 FROM control.stores WHERE tenant_id=r.tenant_id AND id=r.store_id FOR SHARE;
 PERFORM 1 FROM integration.bindings WHERE id=r.binding_id FOR SHARE;
 UPDATE meta_inbox.routes SET enabled=false,route_epoch=route_epoch+1 WHERE id=p_route AND route_epoch=p_epoch RETURNING route_epoch INTO next_epoch;
 IF NOT FOUND THEN RAISE EXCEPTION 'meta route conflict' USING ERRCODE='PT409'; END IF;
 INSERT INTO meta_inbox.audit_events(route_id,action) VALUES(p_route,'disable');
 RETURN next_epoch;
END $$;

CREATE FUNCTION meta_inbox.begin_batch(p_app text,p_object text,p_hash text,p_count integer)
RETURNS TABLE(batch_id uuid,replay boolean) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE b meta_inbox.batches%ROWTYPE;
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_ingress');
 IF p_app IS NULL OR p_app !~ '^[0-9]{1,64}$' OR p_object IS NULL OR p_object NOT IN ('page','instagram')
  OR p_hash IS NULL OR p_hash !~ '^[0-9a-f]{64}$' OR p_count IS NULL OR p_count NOT BETWEEN 1 AND 1000 THEN RAISE EXCEPTION 'invalid meta batch' USING ERRCODE='22023'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended(jsonb_build_array('meta-batch',p_app,p_object,p_hash)::text,0));
 SELECT * INTO b FROM meta_inbox.batches WHERE app_id=p_app AND object=p_object AND body_hash=p_hash;
 IF b.id IS NOT NULL THEN
  IF NOT b.finalized OR b.unit_count<>p_count THEN RAISE EXCEPTION 'meta batch conflict' USING ERRCODE='PT409'; END IF;
  RETURN QUERY SELECT b.id,true; RETURN;
 END IF;
 INSERT INTO meta_inbox.batches(app_id,object,body_hash,unit_count) VALUES(p_app,p_object,p_hash,p_count) RETURNING * INTO b;
 RETURN QUERY SELECT b.id,false;
END $$;

CREATE FUNCTION meta_inbox.prepare_event(p_batch uuid,p_ordinal integer,p_key text,p_hash text,p_asset text,p_kind text,p_reason text,p_occurred timestamptz)
RETURNS TABLE(event_id uuid,needs_body boolean,body_class text,tenant_id uuid,store_id uuid,route_id uuid,route_epoch bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE b meta_inbox.batches%ROWTYPE; e meta_inbox.events%ROWTYPE; r meta_inbox.routes%ROWTYPE;
 primary_exists boolean; valid_route boolean:=false; why text;
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_ingress');
 SELECT * INTO b FROM meta_inbox.batches WHERE id=p_batch AND NOT finalized AND admission_xid=pg_current_xact_id();
 IF b.id IS NULL OR p_ordinal IS NULL OR p_ordinal NOT BETWEEN 1 AND b.unit_count
  OR p_key IS NULL OR p_key !~ '^[0-9a-f]{64}$' OR p_hash IS NULL OR p_hash !~ '^[0-9a-f]{64}$'
  OR p_asset IS NULL OR (p_asset<>'' AND p_asset !~ '^[0-9]{1,64}$') OR p_kind IS NULL
  OR p_kind NOT IN ('page_comment_add','page_comment_edit','page_comment_remove','instagram_comment','instagram_live_comment','page_message','instagram_message','quarantine')
  OR p_reason IS NULL OR (p_kind='quarantine' AND p_reason NOT IN ('object_mismatch','unknown_root_field','empty_or_invalid_entries','invalid_entry','invalid_asset_id','invalid_entry_time','unknown_entry_field','invalid_changes','invalid_messaging','empty_entry','unsupported_change','unsupported_messaging'))
  OR (p_kind<>'quarantine' AND (p_reason<>'' OR p_asset='' OR (b.object='page' AND p_kind NOT LIKE 'page\_%' ESCAPE '\') OR (b.object='instagram' AND p_kind NOT LIKE 'instagram\_%' ESCAPE '\')))
  OR (p_occurred IS NOT NULL AND NOT isfinite(p_occurred)) THEN RAISE EXCEPTION 'invalid meta event' USING ERRCODE='22023'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended(jsonb_build_array('meta-event',b.app_id,b.object,p_key)::text,0));
 SELECT * INTO e FROM meta_inbox.events WHERE app_id=b.app_id AND object=b.object AND event_key=p_key AND payload_hash=p_hash;
 IF e.id IS NOT NULL THEN
  IF e.kind<>p_kind OR e.asset_id<>p_asset THEN RAISE EXCEPTION 'meta event conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  SELECT EXISTS(SELECT 1 FROM meta_inbox.events WHERE app_id=b.app_id AND object=b.object AND event_key=p_key) INTO primary_exists;
  IF primary_exists THEN why:='payload_conflict';
  ELSIF p_kind='quarantine' THEN why:=p_reason;
  ELSE
   why:='untrusted_route';
   SELECT * INTO r FROM meta_inbox.routes WHERE app_id=b.app_id AND object=b.object AND asset_id=p_asset;
   IF r.id IS NOT NULL THEN
    PERFORM 1 FROM control.tenants WHERE id=r.tenant_id AND active FOR SHARE;
    IF FOUND THEN
     PERFORM 1 FROM control.stores s WHERE s.tenant_id=r.tenant_id AND s.id=r.store_id AND active FOR SHARE;
     IF FOUND THEN
      PERFORM 1 FROM integration.bindings WHERE id=r.binding_id AND enabled AND semantic_version=r.binding_version FOR SHARE;
      IF FOUND THEN
       -- Re-read after blocking locks. Immutable scope/binding identity makes the
       -- preliminary lookup safe, but epoch/proof/version remain mutable.
       SELECT * INTO r FROM meta_inbox.routes WHERE id=r.id FOR SHARE;
       SELECT EXISTS(SELECT 1 FROM integration.bindings WHERE id=r.binding_id AND enabled AND semantic_version=r.binding_version) INTO valid_route;
       valid_route:=valid_route AND r.enabled AND r.proof_expires>clock_timestamp();
      END IF;
     END IF;
    END IF;
   END IF;
  END IF;
  INSERT INTO meta_inbox.events(app_id,object,event_key,payload_hash,asset_id,kind,is_primary,disposition,reason,tenant_id,store_id,route_id,route_epoch,binding_id,binding_version,occurred_at)
   VALUES(b.app_id,b.object,p_key,p_hash,p_asset,p_kind,NOT primary_exists,
    CASE WHEN valid_route THEN 'ROUTED' ELSE 'QUARANTINED' END,CASE WHEN valid_route THEN '' ELSE why END,
    CASE WHEN valid_route THEN r.tenant_id END,CASE WHEN valid_route THEN r.store_id END,CASE WHEN valid_route THEN r.id END,
    CASE WHEN valid_route THEN r.route_epoch END,CASE WHEN valid_route THEN r.binding_id END,CASE WHEN valid_route THEN r.binding_version END,p_occurred) RETURNING * INTO e;
 END IF;
 INSERT INTO meta_inbox.batch_events(batch_id,ordinal,event_id) VALUES(p_batch,p_ordinal,e.id);
 RETURN QUERY SELECT e.id,NOT e.completed,CASE WHEN e.disposition='ROUTED' THEN 'event' ELSE 'quarantine' END,e.tenant_id,e.store_id,e.route_id,e.route_epoch;
END $$;

CREATE FUNCTION meta_inbox.complete_event(p_batch uuid,p_event uuid,p_key text,p_nonce bytea,p_cipher bytea,p_job bigint) RETURNS void
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
  IF p_job IS NULL OR NOT EXISTS(SELECT 1 FROM river.river_job j WHERE j.id=p_job AND j.kind='meta_inbox_v1' AND j.queue='meta_inbox' AND j.unique_key IS NULL
    AND j.args=jsonb_build_object('event_id',e.id::text,'version',1) AND j.args->>'version'='1') THEN RAISE EXCEPTION 'invalid meta job' USING ERRCODE='22023'; END IF;
  INSERT INTO meta_private.event_bodies(event_id,tenant_id,store_id,key_id,nonce,ciphertext) VALUES(e.id,e.tenant_id,e.store_id,p_key,p_nonce,p_cipher);
 ELSE
  IF p_job IS NOT NULL THEN RAISE EXCEPTION 'invalid meta job' USING ERRCODE='22023'; END IF;
  INSERT INTO meta_private.quarantine_bodies(event_id,key_id,nonce,ciphertext) VALUES(e.id,p_key,p_nonce,p_cipher);
 END IF;
 UPDATE meta_inbox.events SET completed=true,job_id=p_job WHERE id=e.id;
END $$;

-- Deferred row guards query final transaction state, not the original NEW
-- snapshot. They do not fire on later authorized ciphertext/job retention.
CREATE FUNCTION meta_inbox.check_event(p_event uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE e meta_inbox.events%ROWTYPE;
BEGIN
 SELECT * INTO e FROM meta_inbox.events WHERE id=p_event;
 IF e.id IS NULL OR NOT e.completed THEN RAISE EXCEPTION 'incomplete meta event' USING ERRCODE='22023'; END IF;
 IF e.disposition='ROUTED' THEN
  IF NOT EXISTS(SELECT 1 FROM meta_private.event_bodies WHERE event_id=e.id AND tenant_id=e.tenant_id AND store_id=e.store_id)
   OR EXISTS(SELECT 1 FROM meta_private.quarantine_bodies WHERE event_id=e.id)
   OR NOT EXISTS(SELECT 1 FROM river.river_job j WHERE j.id=e.job_id AND j.kind='meta_inbox_v1' AND j.queue='meta_inbox' AND j.unique_key IS NULL
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
CREATE FUNCTION meta_inbox.check_batch(p_batch uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE b meta_inbox.batches%ROWTYPE;
BEGIN
 SELECT * INTO b FROM meta_inbox.batches WHERE id=p_batch;
 IF b.id IS NULL OR NOT b.finalized OR NOT EXISTS(SELECT 1 FROM meta_private.raw_bodies WHERE batch_id=b.id)
  OR (SELECT count(*) FROM meta_inbox.batch_events WHERE batch_id=b.id)<>b.unit_count
  OR EXISTS(SELECT 1 FROM meta_inbox.batch_events m JOIN meta_inbox.events e ON e.id=m.event_id WHERE m.batch_id=b.id AND (m.ordinal>b.unit_count OR NOT e.completed)) THEN
  RAISE EXCEPTION 'incomplete meta batch' USING ERRCODE='22023'; END IF;
 PERFORM meta_inbox.check_event(e.id) FROM meta_inbox.events e JOIN meta_inbox.batch_events m ON m.event_id=e.id
  WHERE m.batch_id=b.id AND e.admission_xid=pg_current_xact_id();
END $$;
CREATE FUNCTION meta_inbox.guard_insert() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_TABLE_NAME='events' THEN PERFORM meta_inbox.check_event(NEW.id); ELSE PERFORM meta_inbox.check_batch(NEW.id); END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER meta_event_commit AFTER INSERT ON meta_inbox.events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION meta_inbox.guard_insert();
CREATE CONSTRAINT TRIGGER meta_batch_commit AFTER INSERT ON meta_inbox.batches DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION meta_inbox.guard_insert();

CREATE FUNCTION meta_inbox.complete_batch(p_batch uuid,p_key text,p_nonce bytea,p_cipher bytea) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_ingress');
 IF NOT EXISTS(SELECT 1 FROM meta_inbox.batches WHERE id=p_batch AND NOT finalized AND admission_xid=pg_current_xact_id())
  OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_-]{1,64}$' OR p_nonce IS NULL OR octet_length(p_nonce)<>12
  OR p_cipher IS NULL OR octet_length(p_cipher) NOT BETWEEN 17 AND 1048592 THEN RAISE EXCEPTION 'invalid meta batch envelope' USING ERRCODE='22023'; END IF;
 INSERT INTO meta_private.raw_bodies(batch_id,key_id,nonce,ciphertext) VALUES(p_batch,p_key,p_nonce,p_cipher);
 UPDATE meta_inbox.batches SET finalized=true WHERE id=p_batch;
 PERFORM meta_inbox.check_batch(p_batch);
END $$;

CREATE FUNCTION meta_inbox.record_terminal(p_event uuid,p_reason text,p_evidence text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE e meta_inbox.events%ROWTYPE;
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_curator');
 IF p_reason IS NULL OR p_reason NOT IN ('reviewed_rejected','retention_discarded') OR p_evidence IS NULL OR p_evidence !~ '^[0-9a-f]{64}$' THEN RAISE EXCEPTION 'invalid meta review' USING ERRCODE='22023'; END IF;
 SELECT * INTO e FROM meta_inbox.events WHERE id=p_event AND completed FOR UPDATE;
 IF e.id IS NULL THEN RAISE EXCEPTION 'meta review conflict' USING ERRCODE='PT409'; END IF;
 IF e.terminal_reason IS NOT NULL THEN
  IF e.terminal_reason=p_reason AND e.terminal_evidence=p_evidence THEN RETURN; END IF;
  RAISE EXCEPTION 'meta review conflict' USING ERRCODE='PT409';
 END IF;
 UPDATE meta_inbox.events SET terminal_reason=p_reason,terminal_at=clock_timestamp(),terminal_actor=session_user,terminal_evidence=p_evidence WHERE id=e.id;
 INSERT INTO meta_inbox.audit_events(event_id,action,evidence_hash) VALUES(e.id,p_reason,p_evidence);
END $$;

-- Internal predicate deliberately has no execution grant for any login role.
CREATE FUNCTION meta_inbox.purgeable(p_event uuid) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 RETURN EXISTS(SELECT 1 FROM meta_inbox.events e WHERE e.id=p_event AND e.completed AND e.terminal_at IS NOT NULL
  AND NOT EXISTS(SELECT 1 FROM river.river_job j WHERE j.id=e.job_id AND j.state NOT IN ('completed','cancelled','discarded')));
END $$;
CREATE FUNCTION meta_inbox.purge_expired(p_limit integer) RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE candidate record; removed integer:=0; found_id uuid;
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_curator');
 IF p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 1000 THEN RAISE EXCEPTION 'invalid meta purge limit' USING ERRCODE='22023'; END IF;
 -- Pick the oldest eligible candidates across all classes; lock/recheck each
 -- row without waiting. Never infer business completion from age or job state.
 FOR candidate IN
  SELECT * FROM (
   SELECT 'raw' AS class,batch_id AS id,expires_at FROM meta_private.raw_bodies r WHERE expires_at<=clock_timestamp()
    AND NOT EXISTS(SELECT 1 FROM meta_inbox.batch_events m WHERE m.batch_id=r.batch_id AND NOT meta_inbox.purgeable(m.event_id))
   UNION ALL SELECT 'event',event_id,expires_at FROM meta_private.event_bodies WHERE expires_at<=clock_timestamp() AND meta_inbox.purgeable(event_id)
   UNION ALL SELECT 'quarantine',event_id,expires_at FROM meta_private.quarantine_bodies WHERE expires_at<=clock_timestamp() AND meta_inbox.purgeable(event_id)
  ) candidates ORDER BY expires_at,class,id
 LOOP
  EXIT WHEN removed>=p_limit;
  found_id:=NULL;
  IF candidate.class='raw' THEN
   SELECT batch_id INTO found_id FROM meta_private.raw_bodies r WHERE batch_id=candidate.id AND expires_at<=clock_timestamp()
    AND NOT EXISTS(SELECT 1 FROM meta_inbox.batch_events m WHERE m.batch_id=r.batch_id AND NOT meta_inbox.purgeable(m.event_id)) FOR UPDATE SKIP LOCKED;
   IF found_id IS NOT NULL THEN DELETE FROM meta_private.raw_bodies WHERE batch_id=found_id; END IF;
  ELSIF candidate.class='event' THEN
   SELECT event_id INTO found_id FROM meta_private.event_bodies WHERE event_id=candidate.id AND expires_at<=clock_timestamp() AND meta_inbox.purgeable(event_id) FOR UPDATE SKIP LOCKED;
   IF found_id IS NOT NULL THEN DELETE FROM meta_private.event_bodies WHERE event_id=found_id; END IF;
  ELSE
   SELECT event_id INTO found_id FROM meta_private.quarantine_bodies WHERE event_id=candidate.id AND expires_at<=clock_timestamp() AND meta_inbox.purgeable(event_id) FOR UPDATE SKIP LOCKED;
   IF found_id IS NOT NULL THEN DELETE FROM meta_private.quarantine_bodies WHERE event_id=found_id; END IF;
  END IF;
  IF found_id IS NOT NULL THEN removed:=removed+1; END IF;
 END LOOP;
 RETURN removed;
END $$;

-- Keep schema ownership with the migration principal, not the definer. Only
-- fixed functions are callable; granting the private owner role is forbidden.
DO $$ DECLARE f record; BEGIN
 FOR f IN SELECT p.oid::regprocedure AS signature FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='meta_inbox' LOOP
  EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_meta_writer',f.signature);
  EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f.signature);
 END LOOP;
END $$;
GRANT EXECUTE ON FUNCTION meta_inbox.activate_route(text,text,text,uuid,uuid,uuid,bigint,text,timestamptz,bigint),meta_inbox.disable_route(uuid,bigint) TO commerce_meta_registrar;
GRANT EXECUTE ON FUNCTION meta_inbox.begin_batch(text,text,text,integer),meta_inbox.prepare_event(uuid,integer,text,text,text,text,text,timestamptz),meta_inbox.complete_event(uuid,uuid,text,bytea,bytea,bigint),meta_inbox.complete_batch(uuid,text,bytea,bytea) TO commerce_meta_ingress;
GRANT EXECUTE ON FUNCTION meta_inbox.record_terminal(uuid,text,text),meta_inbox.purge_expired(integer) TO commerce_meta_curator;
