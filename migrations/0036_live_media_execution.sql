-- Dedicated MOCK Start execution. The business lease stays in integration.operations.
CREATE ROLE commerce_media_worker NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE commerce_media_executor NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT USAGE ON SCHEMA live TO commerce_media_executor;
GRANT USAGE ON SCHEMA identity TO commerce_media_writer;
GRANT SELECT(id,active) ON identity.principals TO commerce_media_writer;
GRANT SELECT(tenant_id,principal_id,active) ON identity.memberships TO commerce_media_writer;
GRANT SELECT(tenant_id,store_id,principal_id,permission) ON identity.store_grants TO commerce_media_writer;
CREATE POLICY media_operation_update ON integration.operations FOR UPDATE TO commerce_media_writer
 USING (actor_kind='MEDIA_ATTEMPT') WITH CHECK (actor_kind='MEDIA_ATTEMPT');
GRANT UPDATE(state,generation,lease_mode,lease_until,lease_token_hash,result_code,
 provider_reference,updated_at) ON integration.operations TO commerce_media_writer;
-- FOR SHARE requires UPDATE-column privilege; its policy forbids mutation.
CREATE POLICY media_attempt_execution_lock ON live.media_attempts FOR UPDATE TO commerce_media_writer
 USING (true) WITH CHECK (false);
GRANT UPDATE(id) ON live.media_attempts TO commerce_media_writer;

CREATE TABLE live.media_execution_state (
 attempt_id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, session_id uuid NOT NULL,
 authorization_id uuid NOT NULL, operation_id uuid NOT NULL UNIQUE,
 project_id text NOT NULL CHECK (project_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 wire_reserved_at timestamptz, wire_generation bigint,
 egress_id text CHECK (egress_id IS NULL OR egress_id ~ '^EG_[A-Za-z0-9_-]{1,100}$'),
 resource_state text NOT NULL DEFAULT 'UNOBSERVED' CHECK (resource_state IN ('UNOBSERVED','OBSERVED','TERMINAL')),
 transport_status text NOT NULL DEFAULT '' CHECK (transport_status IN ('','EGRESS_STARTING','EGRESS_ACTIVE',
  'EGRESS_ENDING','EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED')),
 started_at_ns bigint NOT NULL DEFAULT 0 CHECK (started_at_ns>=0),
 updated_at_ns bigint NOT NULL DEFAULT 0 CHECK (updated_at_ns>=0),
 ended_at_ns bigint NOT NULL DEFAULT 0 CHECK (ended_at_ns>=0),
 cleanup_required boolean NOT NULL DEFAULT false,
 escalated_at timestamptz, escalation_code text NOT NULL DEFAULT ''
  CHECK (escalation_code IN ('','reconcile_exhausted')),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK ((wire_reserved_at IS NULL)=(wire_generation IS NULL)),
 CHECK (wire_generation IS NULL OR wire_generation>0),
 CHECK ((escalated_at IS NULL)=(escalation_code='')),
 CHECK (resource_state<>'TERMINAL' OR (egress_id IS NOT NULL AND ended_at_ns>0)),
 UNIQUE (tenant_id,store_id,attempt_id),
 UNIQUE (project_id,egress_id),
 FOREIGN KEY (tenant_id,store_id,attempt_id) REFERENCES live.media_attempts(tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,store_id,operation_id) REFERENCES integration.operations(tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,store_id,authorization_id) REFERENCES live.prepared_media_authorizations(tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,store_id,session_id) REFERENCES live.sessions(tenant_id,store_id,id)
);
CREATE TABLE live.media_observations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, attempt_id uuid NOT NULL,
 operation_id uuid NOT NULL, generation bigint NOT NULL CHECK (generation>0),
 source text NOT NULL CHECK (source IN ('START','ROOM','QUERY')),
 project_id text NOT NULL, room_name text NOT NULL CHECK (room_name ~ '^lc_[0-9a-f]{32}$'),
 egress_id text NOT NULL CHECK (egress_id ~ '^EG_[A-Za-z0-9_-]{1,100}$'),
 status text NOT NULL CHECK (status IN ('EGRESS_STARTING','EGRESS_ACTIVE','EGRESS_ENDING',
  'EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED')),
 started_at_ns bigint NOT NULL CHECK (started_at_ns>=0),
 updated_at_ns bigint NOT NULL CHECK (updated_at_ns>=0),
 ended_at_ns bigint NOT NULL CHECK (ended_at_ns>=0),
 observed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 report_hash bytea NOT NULL CHECK (octet_length(report_hash)=32),
 UNIQUE (attempt_id,generation,source,report_hash),
 FOREIGN KEY (tenant_id,store_id,attempt_id) REFERENCES live.media_execution_state(tenant_id,store_id,attempt_id),
 FOREIGN KEY (tenant_id,store_id,operation_id) REFERENCES integration.operations(tenant_id,store_id,id)
);
ALTER TABLE live.media_execution_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.media_execution_state FORCE ROW LEVEL SECURITY;
ALTER TABLE live.media_observations ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.media_observations FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.media_execution_state,live.media_observations FROM PUBLIC;
CREATE POLICY media_execution_read ON live.media_execution_state FOR SELECT TO commerce_media_writer USING (true);
CREATE POLICY media_execution_insert ON live.media_execution_state FOR INSERT TO commerce_media_writer WITH CHECK (true);
CREATE POLICY media_execution_update ON live.media_execution_state FOR UPDATE TO commerce_media_writer USING (true) WITH CHECK (true);
CREATE POLICY media_observation_read ON live.media_observations FOR SELECT TO commerce_media_writer USING (true);
CREATE POLICY media_observation_insert ON live.media_observations FOR INSERT TO commerce_media_writer WITH CHECK (true);
GRANT SELECT,INSERT ON live.media_execution_state TO commerce_media_writer;
GRANT UPDATE(wire_reserved_at,wire_generation,egress_id,resource_state,transport_status,
 started_at_ns,updated_at_ns,ended_at_ns,cleanup_required,escalated_at,escalation_code,updated_at)
 ON live.media_execution_state TO commerce_media_writer;
GRANT SELECT,INSERT ON live.media_observations TO commerce_media_writer;

CREATE FUNCTION live.guard_media_execution_identity() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF (NEW.attempt_id,NEW.tenant_id,NEW.store_id,NEW.session_id,NEW.authorization_id,NEW.operation_id,NEW.project_id)
  IS DISTINCT FROM (OLD.attempt_id,OLD.tenant_id,OLD.store_id,OLD.session_id,OLD.authorization_id,OLD.operation_id,OLD.project_id)
  OR (OLD.wire_reserved_at IS NOT NULL AND (NEW.wire_reserved_at,NEW.wire_generation)
   IS DISTINCT FROM (OLD.wire_reserved_at,OLD.wire_generation))
  OR (OLD.egress_id IS NOT NULL AND NEW.egress_id IS DISTINCT FROM OLD.egress_id)
  OR (OLD.cleanup_required AND NOT NEW.cleanup_required)
  OR (OLD.escalated_at IS NOT NULL AND (NEW.escalated_at,NEW.escalation_code)
   IS DISTINCT FROM (OLD.escalated_at,OLD.escalation_code))
  OR (OLD.resource_state='TERMINAL' AND NEW.resource_state<>'TERMINAL') THEN
  RAISE EXCEPTION 'media execution identity unavailable' USING ERRCODE='ME409';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_media_execution_identity() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_media_execution_identity() FROM PUBLIC;
CREATE TRIGGER media_execution_identity BEFORE UPDATE ON live.media_execution_state
 FOR EACH ROW EXECUTE FUNCTION live.guard_media_execution_identity();

-- False until the native fifth River schema and post migration are installed.
CREATE FUNCTION live.media_native_job(p_job bigint,p_operation uuid) RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$ SELECT false $$;
ALTER FUNCTION live.media_native_job(bigint,uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_native_job(bigint,uuid) FROM PUBLIC;

-- Lock order is shared by all five fixed entry points. Locator reads grant no authority.
CREATE FUNCTION live.lock_media_operation(p_id uuid) RETURNS integration.operations
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 o integration.operations%ROWTYPE; v_binding uuid; v_tenant control.tenants%ROWTYPE;
 v_store control.stores%ROWTYPE; v_session live.sessions%ROWTYPE; v_program live.programs%ROWTYPE;
BEGIN
 SELECT * INTO o FROM integration.operations WHERE id=p_id;
 IF NOT FOUND OR o.actor_kind<>'MEDIA_ATTEMPT' OR o.provider<>'livekit'
  OR o.action<>'livekit.egress.start' OR o.purpose<>'service' THEN
  RAISE EXCEPTION 'media operation unavailable' USING ERRCODE='ME404'; END IF;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 IF a.id IS NULL OR h.id IS NULL OR a.start_operation_id<>p_id
  OR (a.tenant_id,a.store_id,a.session_id,a.authorization_id,a.id)
   IS DISTINCT FROM (h.tenant_id,h.store_id,h.session_id,h.id,h.attempt_id)
  OR o.tenant_id<>a.tenant_id OR o.store_id<>a.store_id THEN
  RAISE EXCEPTION 'media operation unavailable' USING ERRCODE='ME404'; END IF;
 FOR v_binding IN SELECT DISTINCT id FROM (
  SELECT h.media_binding_id AS id UNION ALL
  SELECT d.binding_id FROM live.media_authorization_destinations d WHERE d.authorization_id=h.id
 ) ids ORDER BY id LOOP
  PERFORM 1 FROM integration.bindings WHERE id=v_binding FOR SHARE;
  IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='ME409'; END IF;
 END LOOP;
 SELECT * INTO v_tenant FROM control.tenants WHERE id=a.tenant_id FOR SHARE;
 SELECT * INTO v_store FROM control.stores WHERE tenant_id=a.tenant_id AND id=a.store_id FOR SHARE;
 SELECT * INTO v_session FROM live.sessions WHERE tenant_id=a.tenant_id AND store_id=a.store_id AND id=a.session_id FOR SHARE;
 SELECT * INTO v_program FROM live.programs WHERE tenant_id=a.tenant_id AND store_id=a.store_id
  AND id=a.program_id AND session_id=a.session_id FOR SHARE;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE tenant_id=a.tenant_id
  AND store_id=a.store_id AND id=a.authorization_id FOR SHARE;
 SELECT * INTO a FROM live.media_attempts WHERE tenant_id=h.tenant_id AND store_id=h.store_id
  AND id=h.attempt_id FOR SHARE;
 SELECT * INTO o FROM integration.operations WHERE id=p_id FOR UPDATE;
 IF v_tenant.id IS NULL OR v_store.id IS NULL OR v_session.id IS NULL OR v_program.id IS NULL
  OR h.id IS NULL OR a.id IS NULL OR o.id IS NULL OR o.actor_kind<>'MEDIA_ATTEMPT'
  OR o.media_attempt_id<>a.id OR o.principal_id<>a.original_principal_id
  OR o.binding_id<>h.media_binding_id OR o.binding_version<>h.media_binding_version
  OR o.external_asset_id<>h.project_id OR o.job_id<1 OR a.start_operation_id<>o.id
  OR a.room_name<>'lc_'||replace(a.id::text,'-','') OR a.environment<>'MOCK'
  OR a.execution_profile<>'PROVIDER_MOCK' OR h.environment<>'MOCK'
  OR o.tenant_id<>a.tenant_id OR o.store_id<>a.store_id OR a.session_id<>h.session_id
  OR a.authorization_id<>h.id OR o.request_hash<>a.request_hash
  OR o.request IS DISTINCT FROM jsonb_build_object('attempt_id',a.id::text,
   'session_id',a.session_id::text,'version',1) THEN
  RAISE EXCEPTION 'media operation unavailable' USING ERRCODE='ME409'; END IF;
 RETURN o;
END $$;
ALTER FUNCTION live.lock_media_operation(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.lock_media_operation(uuid) FROM PUBLIC;

-- Mutable Start eligibility. Reserved reconciliation does not use this helper.
CREATE FUNCTION live.media_dispatch_eligible(p_id uuid) RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS (
  SELECT 1 FROM integration.operations o
  JOIN live.media_attempts a ON a.id=o.media_attempt_id AND a.start_operation_id=o.id
  JOIN live.prepared_media_authorizations h ON h.id=a.authorization_id
  JOIN control.tenants t ON t.id=a.tenant_id AND t.active
  JOIN control.stores s ON s.tenant_id=a.tenant_id AND s.id=a.store_id AND s.active
  JOIN live.sessions ls ON ls.id=a.session_id AND ls.tenant_id=a.tenant_id AND ls.store_id=a.store_id
  JOIN live.programs p ON p.id=a.program_id AND p.session_id=a.session_id AND p.tenant_id=a.tenant_id AND p.store_id=a.store_id
  JOIN identity.principals ip ON ip.id=a.original_principal_id AND ip.active
  JOIN identity.memberships im ON im.tenant_id=a.tenant_id AND im.principal_id=ip.id AND im.active
  JOIN identity.store_grants g ON g.tenant_id=a.tenant_id AND g.store_id=a.store_id
   AND g.principal_id=ip.id AND g.permission='live:manage'
  JOIN integration.bindings b ON b.id=h.media_binding_id AND b.tenant_id=a.tenant_id AND b.store_id=a.store_id
  WHERE o.id=p_id AND o.actor_kind='MEDIA_ATTEMPT' AND o.principal_id=ip.id
   AND h.tenant_id=a.tenant_id AND h.store_id=a.store_id AND h.session_id=a.session_id
   AND h.attempt_id=a.id AND h.environment='MOCK' AND h.evidence_type='MOCK_FIXTURE'
   AND h.start_before>clock_timestamp() AND h.max_duration_seconds BETWEEN 1 AND 14400
   AND h.budget_minor BETWEEN 1 AND 1000000000
   AND ls.version=h.session_version AND p.state='READY' AND p.aspect_ratio=h.aspect_ratio
   AND b.enabled AND b.provider='livekit' AND b.external_asset_id=h.project_id
   AND b.semantic_version=h.media_binding_version
   AND NOT EXISTS(SELECT 1 FROM live.media_authorization_revocations r WHERE r.authorization_id=h.id)
   AND (SELECT count(*) FROM live.media_authorization_destinations d WHERE d.authorization_id=h.id) BETWEEN 1 AND 2
   AND NOT EXISTS (
    SELECT 1 FROM live.media_authorization_destinations d
    LEFT JOIN integration.bindings db ON db.id=d.binding_id AND db.tenant_id=a.tenant_id AND db.store_id=a.store_id
    WHERE d.authorization_id=h.id AND (db.id IS NULL OR NOT db.enabled
     OR db.semantic_version<>d.binding_version OR db.provider<>d.provider
     OR db.external_asset_id<>d.external_asset_id))
 )
$$;
ALTER FUNCTION live.media_dispatch_eligible(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_dispatch_eligible(uuid) FROM PUBLIC;

-- Lifetime liability excludes the start deadline; it does not authorize Stop.
CREATE FUNCTION live.media_lifetime_revoked(p_id uuid) RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT NOT EXISTS(
  SELECT 1 FROM integration.operations o
  JOIN live.media_attempts a ON a.id=o.media_attempt_id
  JOIN live.prepared_media_authorizations h ON h.id=a.authorization_id
  JOIN control.tenants t ON t.id=a.tenant_id AND t.active
  JOIN control.stores s ON s.tenant_id=a.tenant_id AND s.id=a.store_id AND s.active
  JOIN identity.principals ip ON ip.id=a.original_principal_id AND ip.active
  JOIN identity.memberships im ON im.tenant_id=a.tenant_id AND im.principal_id=ip.id AND im.active
  JOIN identity.store_grants g ON g.tenant_id=a.tenant_id AND g.store_id=a.store_id
   AND g.principal_id=ip.id AND g.permission='live:manage'
  JOIN integration.bindings b ON b.id=h.media_binding_id AND b.tenant_id=a.tenant_id AND b.store_id=a.store_id
  WHERE o.id=p_id AND o.principal_id=ip.id AND h.attempt_id=a.id
   AND b.enabled AND b.provider='livekit' AND b.external_asset_id=h.project_id
   AND b.semantic_version=h.media_binding_version
   AND NOT EXISTS(SELECT 1 FROM live.media_authorization_revocations r WHERE r.authorization_id=h.id)
   AND NOT EXISTS(SELECT 1 FROM live.media_authorization_destinations d
    LEFT JOIN integration.bindings db ON db.id=d.binding_id AND db.tenant_id=a.tenant_id AND db.store_id=a.store_id
    WHERE d.authorization_id=h.id AND (db.id IS NULL OR NOT db.enabled OR db.provider<>d.provider
     OR db.external_asset_id<>d.external_asset_id OR db.semantic_version<>d.binding_version))
 )
$$;
ALTER FUNCTION live.media_lifetime_revoked(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_lifetime_revoked(uuid) FROM PUBLIC;

CREATE FUNCTION live.claim_media_operation(p_id uuid,p_job bigint,p_lease_seconds integer,p_token bytea)
RETURNS TABLE(disposition text,generation bigint,mode text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 v_now timestamptz; v_mode text;
BEGIN
 IF p_id IS NULL OR p_job IS NULL OR p_job<1 OR p_lease_seconds IS NULL
  OR p_lease_seconds NOT BETWEEN 5 AND 300 OR p_token IS NULL OR octet_length(p_token)<>32
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media claim' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 INSERT INTO live.media_execution_state(attempt_id,tenant_id,store_id,session_id,
  authorization_id,operation_id,project_id)
 VALUES(a.id,a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id)
 ON CONFLICT (attempt_id) DO NOTHING;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=a.id FOR UPDATE;
 IF x.attempt_id IS NULL OR (x.tenant_id,x.store_id,x.session_id,x.authorization_id,x.operation_id,x.project_id)
  IS DISTINCT FROM (a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id) THEN
  RAISE EXCEPTION 'media projection unavailable' USING ERRCODE='ME409'; END IF;
 IF x.escalated_at IS NOT NULL THEN
  RETURN QUERY SELECT 'escalated'::text,o.generation,''::text; RETURN; END IF;
 IF x.resource_state='TERMINAL' OR o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING') THEN
  RETURN QUERY SELECT 'terminal'::text,o.generation,''::text; RETURN; END IF;
 IF p_job<>o.job_id OR NOT live.media_native_job(p_job,o.id) THEN
  RAISE EXCEPTION 'media job unavailable' USING ERRCODE='ME409'; END IF;
 v_now:=clock_timestamp();
 IF o.lease_until>v_now THEN RETURN QUERY SELECT 'busy'::text,o.generation,''::text; RETURN; END IF;
 IF o.generation>=4096 OR v_now>=o.created_at+interval '24 hours' THEN
  UPDATE live.media_execution_state SET escalated_at=v_now,escalation_code='reconcile_exhausted',updated_at=v_now
   WHERE attempt_id=a.id;
  UPDATE integration.operations SET state='UNKNOWN',generation=greatest(o.generation,1),
   lease_mode='',lease_until=NULL,lease_token_hash=NULL,result_code='reconcile_exhausted',updated_at=v_now
   WHERE id=o.id;
  INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
   VALUES(o.tenant_id,o.store_id,o.id,greatest(o.generation,1),'UNKNOWN','','media_reconcile_exhausted');
  RETURN QUERY SELECT 'escalated'::text,greatest(o.generation,1),''::text; RETURN;
 END IF;
 IF x.wire_reserved_at IS NULL THEN
  IF NOT live.media_dispatch_eligible(o.id) THEN
   UPDATE integration.operations SET state='BLOCKED_POLICY',generation=o.generation+1,
    lease_mode='',lease_until=NULL,lease_token_hash=NULL,result_code='policy_denied',updated_at=clock_timestamp()
    WHERE id=o.id;
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
    VALUES(o.tenant_id,o.store_id,o.id,o.generation+1,'BLOCKED_POLICY','','media_policy_denied');
   RETURN QUERY SELECT 'terminal'::text,o.generation+1,''::text; RETURN;
  END IF;
  v_mode:='dispatch';
 ELSE
  v_mode:='reconcile';
  IF live.media_lifetime_revoked(o.id) THEN
   UPDATE live.media_execution_state SET cleanup_required=true,updated_at=clock_timestamp() WHERE attempt_id=a.id;
  END IF;
 END IF;
 v_now:=clock_timestamp();
 UPDATE integration.operations SET state=CASE WHEN v_mode='dispatch' THEN 'DISPATCHING' ELSE 'UNKNOWN' END,
  generation=o.generation+1,lease_mode=v_mode,lease_until=v_now+make_interval(secs=>p_lease_seconds),
  lease_token_hash=sha256(p_token),updated_at=v_now WHERE id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,o.generation+1,
   CASE WHEN v_mode='dispatch' THEN 'DISPATCHING' ELSE 'UNKNOWN' END,v_mode,
   CASE WHEN v_mode='dispatch' THEN 'media_dispatch_claimed' ELSE 'media_reconcile_claimed' END);
 IF v_now+make_interval(secs=>p_lease_seconds)<=clock_timestamp()
  OR (v_mode='dispatch' AND NOT live.media_dispatch_eligible(o.id)) THEN
  RAISE EXCEPTION 'media claim unavailable' USING ERRCODE='ME409'; END IF;
 RETURN QUERY SELECT 'claimed'::text,o.generation+1,v_mode;
END $$;
ALTER FUNCTION live.claim_media_operation(uuid,bigint,integer,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.claim_media_operation(uuid,bigint,integer,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.claim_media_operation(uuid,bigint,integer,bytea) TO commerce_media_executor;

CREATE FUNCTION live.load_media_material(p_id uuid,p_generation bigint,p_token bytea) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 v_key text; v_nonce text; v_ciphertext text;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL
  OR octet_length(p_token)<>32 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media material' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 IF x.attempt_id IS NULL OR x.operation_id<>o.id OR x.project_id<>h.project_id
  OR o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token)
  OR o.lease_mode NOT IN ('dispatch','reconcile')
  OR (o.lease_mode='dispatch' AND o.state<>'DISPATCHING')
  OR (o.lease_mode='reconcile' AND o.state<>'UNKNOWN')
  OR x.escalated_at IS NOT NULL OR x.resource_state='TERMINAL' THEN
  RAISE EXCEPTION 'media lease unavailable' USING ERRCODE='ME409'; END IF;
 IF o.lease_mode='dispatch' THEN
  IF x.wire_reserved_at IS NOT NULL OR NOT live.media_dispatch_eligible(o.id) THEN
   RAISE EXCEPTION 'media material unavailable' USING ERRCODE='ME409'; END IF;
  v_key:=h.key_id; v_nonce:=encode(h.nonce,'hex'); v_ciphertext:=encode(h.ciphertext,'hex');
 ELSE
  IF x.wire_reserved_at IS NULL THEN RAISE EXCEPTION 'media material unavailable' USING ERRCODE='ME409'; END IF;
  v_key:=''; v_nonce:=''; v_ciphertext:='';
 END IF;
 IF o.lease_until<=clock_timestamp() OR (o.lease_mode='dispatch' AND NOT live.media_dispatch_eligible(o.id)) THEN
  RAISE EXCEPTION 'media material unavailable' USING ERRCODE='ME409'; END IF;
 RETURN jsonb_build_object('tenant_id',a.tenant_id::text,'store_id',a.store_id::text,
  'session_id',a.session_id::text,'attempt_id',a.id::text,'project_id',h.project_id,
  'endpoint_identity',h.endpoint_identity,'credential_version',h.credential_version,
  'material_version',h.material_version,'room_name',a.room_name,'aspect_ratio',h.aspect_ratio,
  'egress_id',coalesce(x.egress_id,''),'mode',o.lease_mode,'key_id',v_key,
  'nonce_hex',v_nonce,'ciphertext_hex',v_ciphertext);
END $$;
ALTER FUNCTION live.load_media_material(uuid,bigint,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.load_media_material(uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.load_media_material(uuid,bigint,bytea) TO commerce_media_executor;

CREATE FUNCTION live.reserve_media_start(p_id uuid,p_generation bigint,p_token bytea) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE; v_now timestamptz;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL
  OR octet_length(p_token)<>32 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media reservation' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 IF x.attempt_id IS NULL OR x.operation_id<>o.id OR x.wire_reserved_at IS NOT NULL
  OR o.generation<>p_generation OR o.state<>'DISPATCHING' OR o.lease_mode<>'dispatch'
  OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token)
  OR NOT live.media_dispatch_eligible(o.id) THEN
  RAISE EXCEPTION 'media reservation unavailable' USING ERRCODE='ME409'; END IF;
 v_now:=clock_timestamp();
 UPDATE live.media_execution_state SET wire_reserved_at=v_now,wire_generation=p_generation,updated_at=v_now
  WHERE attempt_id=x.attempt_id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,p_generation,'DISPATCHING','dispatch','media_start_reserved');
 IF o.lease_until<=clock_timestamp() OR NOT live.media_dispatch_eligible(o.id) THEN
  RAISE EXCEPTION 'media reservation unavailable' USING ERRCODE='ME409'; END IF;
END $$;
ALTER FUNCTION live.reserve_media_start(uuid,bigint,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.reserve_media_start(uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.reserve_media_start(uuid,bigint,bytea) TO commerce_media_executor;

CREATE FUNCTION live.record_media_observation(p_id uuid,p_generation bigint,p_token bytea,
 p_source text,p_egress text,p_room text,p_status text,p_started bigint,p_updated bigint,p_ended bigint)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 v_terminal boolean; v_rank integer; v_old_rank integer; v_coherent boolean;
 v_started bigint; v_updated bigint; v_ended bigint;
 v_hash bytea; v_now timestamptz; v_cleanup boolean;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL OR octet_length(p_token)<>32
  OR p_source IS NULL OR p_source NOT IN ('START','ROOM','QUERY')
  OR p_egress IS NULL OR p_egress !~ '^EG_[A-Za-z0-9_-]{1,100}$'
  OR p_room IS NULL OR p_room !~ '^lc_[0-9a-f]{32}$'
  OR p_status IS NULL OR p_status NOT IN ('EGRESS_STARTING','EGRESS_ACTIVE','EGRESS_ENDING',
   'EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED')
  OR p_started IS NULL OR p_started<0 OR p_updated IS NULL OR p_updated<0 OR p_ended IS NULL OR p_ended<0
  OR (p_started>0 AND p_updated>0 AND p_updated<p_started)
  OR (p_ended>0 AND ((p_started>0 AND p_ended<p_started)
   OR (p_updated>0 AND p_updated<p_ended)))
  OR (p_ended>0 AND p_status NOT IN ('EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED'))
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media observation' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 IF x.attempt_id IS NULL OR x.operation_id<>o.id OR x.project_id<>h.project_id
  OR x.wire_reserved_at IS NULL OR x.resource_state='TERMINAL' OR x.escalated_at IS NOT NULL
  OR o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token)
  OR o.lease_mode NOT IN ('dispatch','reconcile')
  OR (o.lease_mode='dispatch' AND o.state<>'DISPATCHING')
  OR (o.lease_mode='reconcile' AND o.state<>'UNKNOWN')
  OR p_room<>a.room_name OR (x.egress_id IS NOT NULL AND x.egress_id<>p_egress)
  OR (p_source='START' AND (o.lease_mode<>'dispatch' OR x.wire_generation<>p_generation))
  OR (p_source='ROOM' AND (o.lease_mode<>'reconcile' OR x.egress_id IS NOT NULL))
  OR (p_source='QUERY' AND (o.lease_mode<>'reconcile' OR x.egress_id IS NULL OR x.egress_id<>p_egress)) THEN
  RAISE EXCEPTION 'media observation unavailable' USING ERRCODE='ME409'; END IF;
 v_started:=greatest(x.started_at_ns,p_started);
 v_updated:=greatest(x.updated_at_ns,p_updated);
 v_ended:=greatest(x.ended_at_ns,p_ended);
 v_coherent:=NOT ((v_started>0 AND v_updated>0 AND v_updated<v_started)
  OR (v_started>0 AND v_ended>0 AND v_ended<v_started)
  OR (v_updated>0 AND v_ended>0 AND v_updated<v_ended));
 IF NOT v_coherent THEN
  v_started:=x.started_at_ns; v_updated:=x.updated_at_ns; v_ended:=x.ended_at_ns;
 END IF;
 v_terminal:=v_coherent AND p_status IN ('EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED') AND v_ended>0;
 v_rank:=CASE p_status WHEN 'EGRESS_STARTING' THEN 1 WHEN 'EGRESS_ACTIVE' THEN 2
  WHEN 'EGRESS_ENDING' THEN 3 ELSE 4 END;
 v_old_rank:=CASE x.transport_status WHEN 'EGRESS_STARTING' THEN 1 WHEN 'EGRESS_ACTIVE' THEN 2
  WHEN 'EGRESS_ENDING' THEN 3 WHEN '' THEN 0 ELSE 4 END;
 v_hash:=sha256(convert_to(jsonb_build_object('attempt_id',a.id::text,'operation_id',o.id::text,
  'generation',p_generation,'source',p_source,'project_id',h.project_id,'room_name',p_room,
  'egress_id',p_egress,'status',p_status,'started_at_ns',p_started,
  'updated_at_ns',p_updated,'ended_at_ns',p_ended)::text,'UTF8'));
 INSERT INTO live.media_observations(tenant_id,store_id,attempt_id,operation_id,generation,source,
  project_id,room_name,egress_id,status,started_at_ns,updated_at_ns,ended_at_ns,report_hash)
 VALUES(a.tenant_id,a.store_id,a.id,o.id,p_generation,p_source,h.project_id,p_room,p_egress,
  p_status,p_started,p_updated,p_ended,v_hash) ON CONFLICT DO NOTHING;
 v_cleanup:=x.cleanup_required OR live.media_lifetime_revoked(o.id)
  OR (v_started>0 AND v_updated>0 AND
  v_updated::numeric-v_started::numeric>=h.max_duration_seconds::numeric*1000000000);
 v_now:=clock_timestamp();
 UPDATE live.media_execution_state SET egress_id=coalesce(x.egress_id,p_egress),
  resource_state=CASE WHEN v_terminal THEN 'TERMINAL' ELSE 'OBSERVED' END,
  transport_status=CASE WHEN v_rank>=v_old_rank THEN p_status ELSE x.transport_status END,
  started_at_ns=v_started,updated_at_ns=v_updated,
  ended_at_ns=v_ended,cleanup_required=v_cleanup,updated_at=v_now
 WHERE attempt_id=a.id;
 UPDATE integration.operations SET state=CASE WHEN v_terminal THEN
   CASE WHEN p_status='EGRESS_COMPLETE' THEN 'SUCCEEDED' ELSE 'FAILED_FINAL' END ELSE 'UNKNOWN' END,
  lease_mode='',lease_until=NULL,lease_token_hash=NULL,
  result_code=CASE WHEN v_terminal THEN 'media_terminal' ELSE 'media_observed' END,
  provider_reference=p_egress,updated_at=v_now WHERE id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(o.tenant_id,o.store_id,o.id,p_generation,
  CASE WHEN v_terminal THEN CASE WHEN p_status='EGRESS_COMPLETE' THEN 'SUCCEEDED' ELSE 'FAILED_FINAL' END
   ELSE 'UNKNOWN' END,'',CASE WHEN v_terminal THEN 'media_terminal' ELSE 'media_observed' END);
 IF o.lease_until<=clock_timestamp() THEN RAISE EXCEPTION 'media lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN CASE WHEN v_terminal THEN 'terminal' ELSE 'observe' END;
END $$;
ALTER FUNCTION live.record_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.record_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.record_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 TO commerce_media_executor;

CREATE FUNCTION live.finish_media_uncertain(p_id uuid,p_generation bigint,p_token bytea,p_code text)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 v_now timestamptz; v_state text;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL OR octet_length(p_token)<>32
  OR p_code IS NULL OR p_code NOT IN ('remote_unknown','not_observed','invalid_observation',
   'credential_unavailable','material_invalid','policy_denied')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media finish' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 IF x.attempt_id IS NULL OR x.operation_id<>o.id OR x.escalated_at IS NOT NULL OR x.resource_state='TERMINAL'
  OR o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token)
  OR o.lease_mode NOT IN ('dispatch','reconcile')
  OR (o.lease_mode='dispatch' AND o.state<>'DISPATCHING')
  OR (o.lease_mode='reconcile' AND o.state<>'UNKNOWN') THEN
  RAISE EXCEPTION 'media lease unavailable' USING ERRCODE='ME409'; END IF;
 IF p_code='policy_denied' AND x.wire_reserved_at IS NULL THEN v_state:='BLOCKED_POLICY';
 ELSE v_state:='UNKNOWN'; END IF;
 v_now:=clock_timestamp();
 IF p_code='policy_denied' AND x.wire_reserved_at IS NOT NULL THEN
  UPDATE live.media_execution_state SET cleanup_required=true,updated_at=v_now WHERE attempt_id=x.attempt_id;
 END IF;
 UPDATE integration.operations SET state=v_state,lease_mode='',lease_until=NULL,lease_token_hash=NULL,
  result_code=p_code,updated_at=v_now WHERE id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(o.tenant_id,o.store_id,o.id,p_generation,v_state,'','media_'||p_code);
 IF o.lease_until<=clock_timestamp() THEN RAISE EXCEPTION 'media lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN CASE WHEN v_state='BLOCKED_POLICY' THEN 'terminal' ELSE 'observe' END;
END $$;
ALTER FUNCTION live.finish_media_uncertain(uuid,bigint,bytea,text) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.finish_media_uncertain(uuid,bigint,bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.finish_media_uncertain(uuid,bigint,bytea,text) TO commerce_media_executor;
