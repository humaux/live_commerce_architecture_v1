-- BRW opt-in runtime. Historical BIC rows stay marker 0 and cannot make wire calls.
-- The original operation lease and River job remain the only execution owner.
CREATE TABLE live.prepared_media_input_runtime_profiles (
 authorization_id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 runtime_version integer NOT NULL DEFAULT 1 CHECK (runtime_version=1),
 FOREIGN KEY (tenant_id,store_id,authorization_id)
  REFERENCES live.prepared_media_input_profiles(tenant_id,store_id,authorization_id) ON DELETE CASCADE
);
ALTER TABLE live.prepared_media_input_runtime_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.prepared_media_input_runtime_profiles FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.prepared_media_input_runtime_profiles FROM PUBLIC;
CREATE POLICY browser_input_profile_read ON live.prepared_media_input_runtime_profiles
 FOR SELECT TO commerce_media_writer USING (true);
CREATE POLICY browser_input_profile_insert ON live.prepared_media_input_runtime_profiles
 FOR INSERT TO commerce_media_writer WITH CHECK (true);
GRANT SELECT,INSERT ON live.prepared_media_input_runtime_profiles TO commerce_media_writer;

ALTER TABLE live.media_input_custody
 ADD COLUMN runtime_version integer NOT NULL DEFAULT 0 CHECK (runtime_version IN (0,1)),
 ADD COLUMN last_runtime_generation bigint NOT NULL DEFAULT 0 CHECK (last_runtime_generation>=0),
 ADD COLUMN last_runtime_turn text NOT NULL DEFAULT '' CHECK
  (last_runtime_turn IN ('','INPUT_OBSERVE','EGRESS','INPUT_CLEANUP','HELD')),
 ADD COLUMN input_cleanup_held_at timestamptz,
 ADD COLUMN input_cleanup_hold_reason text NOT NULL DEFAULT '' CHECK
  (input_cleanup_hold_reason IN ('','budget_exhausted','deadline','unsafe_retry',
   'credential_unavailable','revocation_unproven','reconcile_exhausted')),
 ADD COLUMN observed_participant_sid text CHECK
  (observed_participant_sid ~ '^PA_[A-Za-z0-9_-]{1,100}$'),
 ADD COLUMN input_observed_at timestamptz,
 ADD CONSTRAINT browser_input_hold_pair CHECK
  ((input_cleanup_held_at IS NULL)=(input_cleanup_hold_reason=''));
GRANT UPDATE(last_runtime_generation,last_runtime_turn,input_cleanup_held_at,
 input_cleanup_hold_reason,observed_participant_sid,input_observed_at)
 ON live.media_input_custody TO commerce_media_writer;

CREATE FUNCTION live.guard_browser_input_custody() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  -- plan_media_input_start holds the authorization lock before this insert.
  SELECT coalesce((SELECT r.runtime_version FROM live.prepared_media_input_runtime_profiles r
   WHERE r.authorization_id=NEW.authorization_id AND r.tenant_id=NEW.tenant_id
    AND r.store_id=NEW.store_id),0) INTO NEW.runtime_version;
  IF NEW.last_runtime_generation<>0 OR NEW.last_runtime_turn<>''
   OR NEW.input_cleanup_held_at IS NOT NULL OR NEW.observed_participant_sid IS NOT NULL
   OR NEW.input_observed_at IS NOT NULL THEN
   RAISE EXCEPTION 'browser input custody unavailable' USING ERRCODE='ME409'; END IF;
 ELSIF NEW.runtime_version IS DISTINCT FROM OLD.runtime_version
  OR NEW.last_runtime_generation<OLD.last_runtime_generation
  OR (NEW.last_runtime_generation=OLD.last_runtime_generation
   AND NEW.last_runtime_turn IS DISTINCT FROM OLD.last_runtime_turn)
  OR (OLD.input_cleanup_held_at IS NOT NULL AND
   (NEW.input_cleanup_held_at,NEW.input_cleanup_hold_reason) IS DISTINCT FROM
   (OLD.input_cleanup_held_at,OLD.input_cleanup_hold_reason))
  OR (OLD.input_observed_at IS NOT NULL AND
   (NEW.input_observed_at,NEW.observed_participant_sid) IS DISTINCT FROM
   (OLD.input_observed_at,OLD.observed_participant_sid)) THEN
  RAISE EXCEPTION 'browser input custody unavailable' USING ERRCODE='ME409';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_browser_input_custody() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_browser_input_custody() FROM PUBLIC;
CREATE TRIGGER browser_input_custody_identity BEFORE INSERT OR UPDATE ON live.media_input_custody
 FOR EACH ROW EXECUTE FUNCTION live.guard_browser_input_custody();

CREATE FUNCTION live.register_media_input_runtime_profile(p_authorization uuid) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE h live.prepared_media_authorizations%ROWTYPE;
BEGIN
 IF p_authorization IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid browser input profile' USING ERRCODE='MP400'; END IF;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=p_authorization FOR UPDATE;
 IF h.id IS NULL OR h.environment<>'MOCK' OR h.evidence_type<>'MOCK_FIXTURE'
  OR h.start_before<=clock_timestamp()
  OR NOT EXISTS(SELECT 1 FROM live.prepared_media_input_profiles WHERE authorization_id=h.id)
  OR EXISTS(SELECT 1 FROM live.media_authorization_revocations WHERE authorization_id=h.id)
  OR EXISTS(SELECT 1 FROM live.media_attempts WHERE authorization_id=h.id) THEN
  RAISE EXCEPTION 'browser input profile unavailable' USING ERRCODE='MP409'; END IF;
 INSERT INTO live.prepared_media_input_runtime_profiles(authorization_id,tenant_id,store_id)
 VALUES(h.id,h.tenant_id,h.store_id) ON CONFLICT DO NOTHING;
 IF h.start_before<=clock_timestamp() THEN
  RAISE EXCEPTION 'browser input profile unavailable' USING ERRCODE='MP409'; END IF;
END $$;
ALTER FUNCTION live.register_media_input_runtime_profile(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.register_media_input_runtime_profile(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.register_media_input_runtime_profile(uuid) TO commerce_media_registrar;

CREATE FUNCTION live.load_media_input_runtime_profile(p_hash bytea,p_store uuid,p_session uuid,
 p_authorization uuid,p_expected bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v record; h live.prepared_media_authorizations%ROWTYPE; a live.media_attempts%ROWTYPE;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_session IS NULL
  OR p_authorization IS NULL OR p_expected IS NULL OR p_expected<1
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid browser input profile' USING ERRCODE='MP400'; END IF;
 SELECT * INTO v FROM identity.resolve_access(p_hash,p_store,'live:manage');
 IF v.access_status='unauthorized' THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP401'; END IF;
 IF v.access_status<>'ok' OR v.tenant_id IS DISTINCT FROM
   nullif(current_setting('app.tenant_id',true),'')::uuid
  OR p_store IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid
  OR v.principal_id IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
  OR v.authz_revision IS DISTINCT FROM nullif(current_setting('app.authz_revision',true),'')::bigint THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403'; END IF;
 -- Locator read only: the planner takes ordered binding/session/authorization locks.
 -- Replaying its receipt still checks the same marked, login-owned attempt.
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=p_authorization;
 IF h.id IS NULL OR h.tenant_id<>v.tenant_id OR h.store_id<>p_store OR h.session_id<>p_session THEN
  RAISE EXCEPTION 'browser input profile unavailable' USING ERRCODE='MP404'; END IF;
 IF h.environment<>'MOCK' OR h.evidence_type<>'MOCK_FIXTURE' OR h.session_version<>p_expected
  OR h.start_before<=clock_timestamp()
  OR EXISTS(SELECT 1 FROM live.media_authorization_revocations WHERE authorization_id=h.id)
  OR NOT EXISTS(SELECT 1 FROM live.sessions WHERE id=p_session AND tenant_id=v.tenant_id
   AND store_id=p_store AND version=p_expected)
  OR NOT EXISTS(SELECT 1 FROM live.prepared_media_input_runtime_profiles
   WHERE authorization_id=h.id AND tenant_id=h.tenant_id AND store_id=h.store_id AND runtime_version=1) THEN
  RAISE EXCEPTION 'browser input profile unavailable' USING ERRCODE='MP409'; END IF;
 IF NOT live.media_browser_input_worker_ready() THEN
  RAISE EXCEPTION 'browser input worker unavailable' USING ERRCODE='MP409'; END IF;
 SELECT * INTO a FROM live.media_attempts WHERE authorization_id=h.id;
 IF FOUND THEN
  PERFORM live.assert_media_start_login(p_hash,p_store,a.id);
  IF NOT EXISTS(SELECT 1 FROM live.media_input_custody WHERE attempt_id=a.id
   AND runtime_version=1 AND admission_closed_at IS NULL AND lifetime_deadline>clock_timestamp())
   OR NOT live.media_dispatch_eligible(a.start_operation_id) THEN
   RAISE EXCEPTION 'browser input profile unavailable' USING ERRCODE='MP409'; END IF;
 END IF;
 RETURN jsonb_build_object('project_id',h.project_id,'endpoint_identity',h.endpoint_identity,
  'credential_version',h.credential_version,'runtime_version',1);
END $$;
ALTER FUNCTION live.load_media_input_runtime_profile(bytea,uuid,uuid,uuid,bigint) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.load_media_input_runtime_profile(bytea,uuid,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.load_media_input_runtime_profile(bytea,uuid,uuid,uuid,bigint) TO commerce_runtime;

CREATE TABLE live.media_input_wire_steps (
 attempt_id uuid NOT NULL, ordinal integer NOT NULL CHECK (ordinal BETWEEN 1 AND 8),
 operation_id uuid NOT NULL, generation bigint NOT NULL CHECK (generation>0),
 action text NOT NULL, reserved_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 result text NOT NULL DEFAULT 'PENDING' CHECK (result IN ('PENDING','ACK','PRESENT','ABSENT','UNKNOWN')),
 resource_sid text NOT NULL DEFAULT '', result_at timestamptz,
 PRIMARY KEY(attempt_id,ordinal), UNIQUE(attempt_id,generation),
 FOREIGN KEY (attempt_id) REFERENCES live.media_input_custody(attempt_id) ON DELETE CASCADE,
 FOREIGN KEY (operation_id) REFERENCES live.media_input_custody(operation_id) ON DELETE CASCADE,
 CHECK (action=CASE (ordinal-1)%4 WHEN 0 THEN 'REMOVE' WHEN 1 THEN 'DELETE_ROOM'
  WHEN 2 THEN 'READ_PARTICIPANT' ELSE 'READ_ROOM' END),
 CHECK ((result='PENDING')=(result_at IS NULL)),
 CHECK (resource_sid='' OR (action='READ_PARTICIPANT' AND result='PRESENT'
  AND resource_sid ~ '^PA_[A-Za-z0-9_-]{1,100}$') OR (action='READ_ROOM' AND result='PRESENT'
  AND resource_sid ~ '^RM_[A-Za-z0-9_-]{1,100}$'))
);
ALTER TABLE live.media_input_wire_steps ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.media_input_wire_steps FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.media_input_wire_steps FROM PUBLIC;
CREATE POLICY browser_input_wire_read ON live.media_input_wire_steps FOR SELECT TO commerce_media_writer USING (true);
CREATE POLICY browser_input_wire_insert ON live.media_input_wire_steps FOR INSERT TO commerce_media_writer WITH CHECK (true);
CREATE POLICY browser_input_wire_update ON live.media_input_wire_steps FOR UPDATE TO commerce_media_writer USING (true) WITH CHECK (true);
GRANT SELECT,INSERT,UPDATE(result,resource_sid,result_at) ON live.media_input_wire_steps TO commerce_media_writer;
CREATE FUNCTION live.guard_media_input_wire_step() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.result<>'PENDING' OR NEW.resource_sid<>'' OR NEW.result_at IS NOT NULL
   OR NOT EXISTS(SELECT 1 FROM live.media_input_custody WHERE attempt_id=NEW.attempt_id
    AND operation_id=NEW.operation_id AND runtime_version=1) THEN
   RAISE EXCEPTION 'browser input step unavailable' USING ERRCODE='ME409'; END IF;
 ELSIF OLD.result<>'PENDING' OR NEW.result='PENDING'
  OR (NEW.attempt_id,NEW.ordinal,NEW.operation_id,NEW.generation,NEW.action,NEW.reserved_at)
   IS DISTINCT FROM (OLD.attempt_id,OLD.ordinal,OLD.operation_id,OLD.generation,OLD.action,OLD.reserved_at) THEN
  RAISE EXCEPTION 'browser input step unavailable' USING ERRCODE='ME409'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_media_input_wire_step() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_media_input_wire_step() FROM PUBLIC;
CREATE TRIGGER browser_input_wire_identity BEFORE INSERT OR UPDATE ON live.media_input_wire_steps
 FOR EACH ROW EXECUTE FUNCTION live.guard_media_input_wire_step();

-- Private shared fence: every provider action uses the original lease, not a new ledger.
CREATE FUNCTION live.lock_browser_input_operation(p_id uuid,p_generation bigint,p_token bytea)
RETURNS integration.operations LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; i live.media_input_custody%ROWTYPE;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL OR octet_length(p_token)<>32
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid browser input lease' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 PERFORM 1 FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id AND operation_id=o.id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'browser input projection unavailable' USING ERRCODE='ME409'; END IF;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 IF i.attempt_id IS NULL OR i.operation_id<>o.id OR i.runtime_version<>1
  OR NOT EXISTS(SELECT 1 FROM live.prepared_media_input_runtime_profiles WHERE authorization_id=i.authorization_id)
  OR NOT live.media_native_job(o.job_id,o.id)
  OR o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token)
  OR NOT ((o.state='UNKNOWN' AND o.lease_mode='reconcile')
   OR (o.state='DISPATCHING' AND o.lease_mode='dispatch')) THEN
  RAISE EXCEPTION 'browser input lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN o;
END $$;
ALTER FUNCTION live.lock_browser_input_operation(uuid,bigint,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.lock_browser_input_operation(uuid,bigint,bytea) FROM PUBLIC;

-- A separate input hold must never suppress outstanding Egress work.
CREATE FUNCTION live.hold_browser_input(p_attempt uuid,p_reason text) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 UPDATE live.media_input_custody SET input_cleanup_held_at=clock_timestamp(),
  input_cleanup_hold_reason=p_reason,updated_at=clock_timestamp()
 WHERE attempt_id=p_attempt AND input_cleanup_held_at IS NULL AND runtime_version=1;
END $$;
ALTER FUNCTION live.hold_browser_input(uuid,text) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.hold_browser_input(uuid,text) FROM PUBLIC;

-- Preserve the tested kernel body; the new public entry cannot lease runtime-1 rows.
ALTER FUNCTION live.claim_media_input_operation(uuid,bigint,integer,bytea) RENAME TO claim_media_input_kernel;
REVOKE ALL ON FUNCTION live.claim_media_input_kernel(uuid,bigint,integer,bytea) FROM commerce_media_executor;
CREATE FUNCTION live.claim_media_input_operation(p_id uuid,p_job bigint,p_seconds integer,p_token bytea)
RETURNS TABLE(disposition text,generation bigint,mode text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM live.media_input_custody WHERE operation_id=p_id AND runtime_version<>0) THEN
  RAISE EXCEPTION 'kernel input claim unavailable' USING ERRCODE='ME409'; END IF;
 RETURN QUERY SELECT * FROM live.claim_media_input_kernel(p_id,p_job,p_seconds,p_token);
END $$;
ALTER FUNCTION live.claim_media_input_operation(uuid,bigint,integer,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.claim_media_input_operation(uuid,bigint,integer,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.claim_media_input_operation(uuid,bigint,integer,bytea) TO commerce_media_executor;

-- Joint terminal proof, not elapsed time or a successful Remove/Delete response.
CREATE FUNCTION live.complete_browser_input(p_id uuid) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 i live.media_input_custody%ROWTYPE; v_state text;
BEGIN
 SELECT * INTO o FROM integration.operations WHERE id=p_id;
 SELECT * INTO x FROM live.media_execution_state WHERE operation_id=p_id;
 SELECT * INTO i FROM live.media_input_custody WHERE operation_id=p_id;
 IF i.runtime_version IS DISTINCT FROM 1 OR i.state IS DISTINCT FROM 'CLOSED'
  OR x.attempt_id IS NULL OR o.id IS NULL OR NOT
  ((x.resource_state='TERMINAL' AND x.egress_id IS NOT NULL AND x.ended_at_ns>0
    AND x.transport_status IN ('EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED'))
   OR (x.wire_reserved_at IS NULL AND i.admission_closed_at IS NOT NULL)) THEN RETURN false; END IF;
 v_state:=CASE WHEN x.resource_state='TERMINAL' AND x.transport_status='EGRESS_COMPLETE' THEN 'SUCCEEDED'
  WHEN x.resource_state='TERMINAL' THEN 'FAILED_FINAL'
  WHEN x.stop_requested_at IS NOT NULL THEN 'CANCELLED' ELSE 'BLOCKED_POLICY' END;
 IF o.state<>v_state THEN
  UPDATE integration.operations SET state=v_state,generation=greatest(o.generation,1),
   lease_mode='',lease_until=NULL,lease_token_hash=NULL,result_code='media_input_terminal',
   updated_at=clock_timestamp() WHERE id=o.id;
  INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
   VALUES(o.tenant_id,o.store_id,o.id,greatest(o.generation,1),v_state,'','media_input_terminal');
 END IF;
 RETURN true;
END $$;
ALTER FUNCTION live.complete_browser_input(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.complete_browser_input(uuid) FROM PUBLIC;

-- Pure selection under the operation lock; persist the decision before wire I/O.
CREATE FUNCTION live.browser_input_next_action(p_attempt uuid) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE i live.media_input_custody%ROWTYPE; x live.media_execution_state%ROWTYPE;
 v_input boolean; v_egress boolean;
BEGIN
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=p_attempt;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=p_attempt;
 IF i.runtime_version IS DISTINCT FROM 1 OR x.attempt_id IS NULL THEN RETURN 'HELD'; END IF;
 v_input:=i.admission_closed_at IS NOT NULL AND i.grant_iat IS NOT NULL
  AND i.state<>'CLOSED' AND i.input_cleanup_held_at IS NULL;
 v_egress:=x.wire_reserved_at IS NOT NULL AND x.resource_state<>'TERMINAL' AND x.escalated_at IS NULL;
 IF v_input AND (NOT v_egress OR i.last_runtime_turn<>'INPUT_CLEANUP') THEN RETURN 'INPUT_CLEANUP'; END IF;
 IF v_egress THEN RETURN 'EGRESS'; END IF;
 IF x.wire_reserved_at IS NULL AND i.state='RESERVED' AND i.admission_closed_at IS NULL
  AND i.input_cleanup_held_at IS NULL THEN RETURN 'INPUT_OBSERVE'; END IF;
 RETURN 'HELD';
END $$;
ALTER FUNCTION live.browser_input_next_action(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.browser_input_next_action(uuid) FROM PUBLIC;

CREATE FUNCTION live.claim_browser_input_operation(p_id uuid,p_job bigint,p_seconds integer,p_token bytea)
RETURNS TABLE(disposition text,generation bigint,mode text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; a live.media_attempts%ROWTYPE;
 h live.prepared_media_authorizations%ROWTYPE; i live.media_input_custody%ROWTYPE;
 x live.media_execution_state%ROWTYPE; v_now timestamptz;
BEGIN
 IF p_id IS NULL OR p_job IS NULL OR p_job<1 OR p_seconds IS NULL OR p_seconds NOT BETWEEN 1 AND 30
  OR p_token IS NULL OR octet_length(p_token)<>32 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid browser input claim' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id;
 IF a.execution_profile<>'LOCAL_SFU_MOCK_EGRESS' OR i.attempt_id IS NULL OR p_job<>o.job_id
  OR NOT live.media_native_job(p_job,o.id) THEN
  RAISE EXCEPTION 'browser input job unavailable' USING ERRCODE='ME409'; END IF;
 IF i.runtime_version=0 THEN
  RETURN QUERY SELECT 'kernel_only'::text,o.generation,''::text; RETURN; END IF;
 IF NOT EXISTS(SELECT 1 FROM live.prepared_media_input_runtime_profiles WHERE authorization_id=h.id) THEN
  RAISE EXCEPTION 'browser input profile unavailable' USING ERRCODE='ME409'; END IF;
 INSERT INTO live.media_execution_state(attempt_id,tenant_id,store_id,session_id,authorization_id,operation_id,project_id)
 VALUES(a.id,a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id) ON CONFLICT DO NOTHING;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=a.id FOR UPDATE;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id FOR UPDATE;
 IF x.operation_id<>o.id OR x.project_id<>i.project_id THEN
  RAISE EXCEPTION 'browser input projection unavailable' USING ERRCODE='ME409'; END IF;
 v_now:=clock_timestamp();
 IF o.lease_until>v_now THEN RETURN QUERY SELECT 'busy'::text,o.generation,''::text; RETURN; END IF;
 -- An expired reservation is consumed; an old reply cannot reclaim its budget.
 UPDATE live.media_input_wire_steps SET result='UNKNOWN',result_at=v_now
 WHERE attempt_id=a.id AND result='PENDING';
 IF i.admission_closed_at IS NULL THEN
  IF x.stop_requested_at IS NOT NULL THEN PERFORM live.close_media_input_custody(a.id,'merchant_stop');
  ELSIF v_now>=i.lifetime_deadline OR (x.wire_reserved_at IS NULL AND v_now>=h.start_before) THEN
   PERFORM live.close_media_input_custody(a.id,'expired');
  ELSIF (x.wire_reserved_at IS NULL AND NOT live.media_dispatch_eligible(o.id))
   OR (x.wire_reserved_at IS NOT NULL AND live.media_lifetime_revoked(o.id)) THEN
   PERFORM live.close_media_input_custody(a.id,'permission_lost');
  END IF;
 END IF;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id;
 IF i.admission_closed_at IS NOT NULL AND x.wire_reserved_at IS NOT NULL THEN
  UPDATE live.media_execution_state SET cleanup_required=true,updated_at=v_now WHERE attempt_id=a.id;
 END IF;
 IF live.complete_browser_input(o.id) THEN
  RETURN QUERY SELECT 'terminal'::text,greatest(o.generation,1),''::text; RETURN; END IF;
 IF o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING') THEN
  RAISE EXCEPTION 'browser input terminal mismatch' USING ERRCODE='ME409'; END IF;
 IF o.generation>=4096 OR v_now>=o.created_at+interval '24 hours' THEN
  PERFORM live.close_media_input_custody(a.id,'reconcile_exhausted');
  IF live.complete_browser_input(o.id) THEN
   RETURN QUERY SELECT 'terminal'::text,greatest(o.generation,1),''::text; RETURN; END IF;
  PERFORM live.hold_browser_input(a.id,'reconcile_exhausted');
  UPDATE live.media_execution_state SET escalated_at=coalesce(escalated_at,v_now),
   escalation_code=CASE WHEN escalated_at IS NULL THEN 'reconcile_exhausted' ELSE escalation_code END,
   updated_at=v_now WHERE attempt_id=a.id;
  UPDATE integration.operations SET state=CASE WHEN o.generation=0 THEN 'READY' ELSE 'UNKNOWN' END,
   lease_mode='',lease_until=NULL,lease_token_hash=NULL,
   result_code='reconcile_exhausted',updated_at=v_now WHERE id=o.id;
  RETURN QUERY SELECT 'held'::text,o.generation,''::text; RETURN;
 END IF;
 IF i.state='UNISSUED' AND i.admission_closed_at IS NULL THEN
  RETURN QUERY SELECT 'await_admission'::text,o.generation,''::text; RETURN; END IF;
 IF i.admission_closed_at+interval '180 seconds'<=v_now THEN
  PERFORM live.hold_browser_input(a.id,'deadline'); END IF;
 IF live.browser_input_next_action(a.id)='HELD' THEN
  UPDATE integration.operations SET state=CASE WHEN o.generation=0 THEN 'READY' ELSE 'UNKNOWN' END,
   lease_mode='',lease_until=NULL,lease_token_hash=NULL,
   result_code='input_held',updated_at=v_now WHERE id=o.id;
  RETURN QUERY SELECT 'held'::text,o.generation,''::text; RETURN; END IF;
 UPDATE integration.operations SET state='UNKNOWN',generation=o.generation+1,lease_mode='reconcile',
  lease_until=v_now+make_interval(secs=>p_seconds),lease_token_hash=sha256(p_token),updated_at=v_now WHERE id=o.id;
 UPDATE live.media_execution_state SET claim_started_at=v_now,updated_at=v_now WHERE attempt_id=a.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(o.tenant_id,o.store_id,o.id,o.generation+1,'UNKNOWN','reconcile','media_reconcile_claimed');
 IF v_now+make_interval(secs=>p_seconds)<=clock_timestamp() THEN
  RAISE EXCEPTION 'browser input lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN QUERY SELECT 'claimed'::text,o.generation+1,'reconcile'::text;
END $$;
ALTER FUNCTION live.claim_browser_input_operation(uuid,bigint,integer,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.claim_browser_input_operation(uuid,bigint,integer,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.claim_browser_input_operation(uuid,bigint,integer,bytea) TO commerce_media_executor;

CREATE FUNCTION live.next_media_input_turn(p_id uuid,p_generation bigint,p_token bytea) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; i live.media_input_custody%ROWTYPE; v_turn text;
BEGIN
 o:=live.lock_browser_input_operation(p_id,p_generation,p_token);
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=o.media_attempt_id;
 IF i.last_runtime_generation=p_generation THEN RETURN i.last_runtime_turn; END IF;
 IF i.admission_closed_at+interval '180 seconds'<=clock_timestamp() THEN
  PERFORM live.hold_browser_input(i.attempt_id,'deadline'); END IF;
 v_turn:=live.browser_input_next_action(i.attempt_id);
 UPDATE live.media_input_custody SET last_runtime_generation=p_generation,last_runtime_turn=v_turn,
  updated_at=clock_timestamp() WHERE attempt_id=i.attempt_id;
 IF o.lease_until<=clock_timestamp() THEN
  RAISE EXCEPTION 'browser input lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN v_turn;
END $$;
ALTER FUNCTION live.next_media_input_turn(uuid,bigint,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.next_media_input_turn(uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.next_media_input_turn(uuid,bigint,bytea) TO commerce_media_executor;

CREATE FUNCTION live.finish_media_input_turn(p_id uuid,p_generation bigint,p_token bytea,p_reason text) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; i live.media_input_custody%ROWTYPE; v_result text;
BEGIN
 IF p_reason IS NULL OR p_reason NOT IN ('input_not_ready','input_unknown','credential_unavailable',
  'material_invalid','cleanup_unknown') THEN
  RAISE EXCEPTION 'invalid browser input result' USING ERRCODE='ME400'; END IF;
 o:=live.lock_browser_input_operation(p_id,p_generation,p_token);
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=o.media_attempt_id;
 IF p_reason='credential_unavailable' AND i.last_runtime_turn IN ('INPUT_OBSERVE','INPUT_CLEANUP') THEN
  PERFORM live.close_media_input_custody(i.attempt_id,'runtime_unavailable');
  PERFORM live.hold_browser_input(i.attempt_id,'credential_unavailable');
 END IF;
 IF live.complete_browser_input(o.id) THEN RETURN 'terminal'; END IF;
 v_result:=CASE WHEN live.browser_input_next_action(i.attempt_id)='HELD' THEN 'held' ELSE 'observe' END;
 UPDATE integration.operations SET state='UNKNOWN',lease_mode='',lease_until=NULL,lease_token_hash=NULL,
  result_code=p_reason,updated_at=clock_timestamp() WHERE id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(o.tenant_id,o.store_id,o.id,p_generation,'UNKNOWN','',p_reason);
 IF o.lease_until<=clock_timestamp() THEN
  RAISE EXCEPTION 'browser input lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION live.finish_media_input_turn(uuid,bigint,bytea,text) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.finish_media_input_turn(uuid,bigint,bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.finish_media_input_turn(uuid,bigint,bytea,text) TO commerce_media_executor;

CREATE FUNCTION live.reserve_media_input_start(p_id uuid,p_generation bigint,p_token bytea,
 p_room text,p_identity text,p_sid text,p_state text,p_camera boolean,p_camera_muted boolean,
 p_microphone boolean,p_microphone_muted boolean) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; i live.media_input_custody%ROWTYPE;
 x live.media_execution_state%ROWTYPE; a live.media_attempts%ROWTYPE;
 h live.prepared_media_authorizations%ROWTYPE; v_now timestamptz;
BEGIN
 IF p_room IS NULL OR p_identity IS NULL OR p_sid IS NULL OR p_sid !~ '^PA_[A-Za-z0-9_-]{1,100}$'
  OR p_state IS NULL OR p_state NOT IN ('JOINED','ACTIVE') OR p_camera IS DISTINCT FROM true
  OR p_microphone IS DISTINCT FROM true OR p_camera_muted IS DISTINCT FROM false
  OR p_microphone_muted IS DISTINCT FROM false THEN
  RAISE EXCEPTION 'browser input not ready' USING ERRCODE='ME409'; END IF;
 o:=live.lock_browser_input_operation(p_id,p_generation,p_token);
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=o.media_attempt_id;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=i.attempt_id;
 SELECT * INTO a FROM live.media_attempts WHERE id=i.attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=i.authorization_id;
 IF o.lease_mode<>'reconcile' OR i.last_runtime_generation<>p_generation
  OR i.last_runtime_turn<>'INPUT_OBSERVE' OR i.room_name<>p_room OR i.publisher_identity<>p_identity
  OR i.state<>'RESERVED' OR i.admission_closed_at IS NOT NULL OR i.input_cleanup_held_at IS NOT NULL
  OR i.lifetime_deadline<=clock_timestamp() OR h.start_before<=clock_timestamp()
  OR x.wire_reserved_at IS NOT NULL OR x.stop_requested_at IS NOT NULL OR x.escalated_at IS NOT NULL
  OR x.resource_state='TERMINAL' OR NOT live.media_dispatch_eligible(o.id) THEN
  RAISE EXCEPTION 'browser input start unavailable' USING ERRCODE='ME409'; END IF;
 v_now:=clock_timestamp();
 UPDATE live.media_input_custody SET observed_participant_sid=p_sid,input_observed_at=v_now,
  updated_at=v_now WHERE attempt_id=i.attempt_id;
 UPDATE live.media_execution_state SET wire_reserved_at=v_now,wire_generation=p_generation,
  updated_at=v_now WHERE attempt_id=i.attempt_id;
 UPDATE integration.operations SET state='DISPATCHING',lease_mode='dispatch',updated_at=v_now WHERE id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(o.tenant_id,o.store_id,o.id,p_generation,'DISPATCHING','dispatch','media_start_reserved');
 IF o.lease_until<=clock_timestamp() OR i.lifetime_deadline<=clock_timestamp()
  OR NOT live.media_dispatch_eligible(o.id) THEN
  RAISE EXCEPTION 'browser input start unavailable' USING ERRCODE='ME409'; END IF;
 RETURN jsonb_build_object('tenant_id',a.tenant_id::text,'store_id',a.store_id::text,
  'session_id',a.session_id::text,'attempt_id',a.id::text,'project_id',h.project_id,
  'endpoint_identity',h.endpoint_identity,'credential_version',h.credential_version,
  'material_version',h.material_version,'room_name',a.room_name,'aspect_ratio',h.aspect_ratio,
  'egress_id','','mode','dispatch','key_id',h.key_id,
  'nonce_hex',encode(h.nonce,'hex'),'ciphertext_hex',encode(h.ciphertext,'hex'));
END $$;
ALTER FUNCTION live.reserve_media_input_start(uuid,bigint,bytea,text,text,text,text,boolean,boolean,boolean,boolean)
 OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.reserve_media_input_start(uuid,bigint,bytea,text,text,text,text,boolean,boolean,boolean,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.reserve_media_input_start(uuid,bigint,bytea,text,text,text,text,boolean,boolean,boolean,boolean)
 TO commerce_media_executor;

CREATE FUNCTION live.load_media_input_material(p_id uuid,p_generation bigint,p_token bytea) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; i live.media_input_custody%ROWTYPE;
 x live.media_execution_state%ROWTYPE; a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
BEGIN
 o:=live.lock_browser_input_operation(p_id,p_generation,p_token);
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=o.media_attempt_id;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=i.attempt_id;
 SELECT * INTO a FROM live.media_attempts WHERE id=i.attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=i.authorization_id;
 IF o.lease_mode<>'reconcile' OR i.last_runtime_generation<>p_generation OR i.last_runtime_turn<>'EGRESS'
  OR x.wire_reserved_at IS NULL OR x.resource_state='TERMINAL' OR x.escalated_at IS NOT NULL THEN
  RAISE EXCEPTION 'browser input material unavailable' USING ERRCODE='ME409'; END IF;
 RETURN jsonb_build_object('tenant_id',a.tenant_id::text,'store_id',a.store_id::text,
  'session_id',a.session_id::text,'attempt_id',a.id::text,'project_id',h.project_id,
  'endpoint_identity',h.endpoint_identity,'credential_version',h.credential_version,
  'material_version',h.material_version,'room_name',a.room_name,'aspect_ratio',h.aspect_ratio,
  'egress_id',coalesce(x.egress_id,''),'mode','reconcile','key_id','','nonce_hex','','ciphertext_hex','');
END $$;
ALTER FUNCTION live.load_media_input_material(uuid,bigint,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.load_media_input_material(uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.load_media_input_material(uuid,bigint,bytea) TO commerce_media_executor;

CREATE FUNCTION live.reserve_media_input_cleanup(p_id uuid,p_generation bigint,p_token bytea) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; i live.media_input_custody%ROWTYPE;
 v_ordinal integer; v_action text; v_now timestamptz;
BEGIN
 o:=live.lock_browser_input_operation(p_id,p_generation,p_token);
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=o.media_attempt_id;
 IF o.lease_mode<>'reconcile' OR i.last_runtime_generation<>p_generation
  OR i.last_runtime_turn<>'INPUT_CLEANUP' OR i.admission_closed_at IS NULL
  OR i.grant_iat IS NULL OR i.state='CLOSED' THEN
  RAISE EXCEPTION 'browser input cleanup unavailable' USING ERRCODE='ME409'; END IF;
 IF EXISTS(SELECT 1 FROM live.media_input_wire_steps WHERE attempt_id=i.attempt_id AND generation=p_generation) THEN
  RAISE EXCEPTION 'browser input step consumed' USING ERRCODE='ME409'; END IF;
 IF i.input_cleanup_held_at IS NOT NULL THEN RETURN NULL; END IF;
 v_now:=clock_timestamp();
 IF i.admission_closed_at+interval '180 seconds'<=v_now THEN
  PERFORM live.hold_browser_input(i.attempt_id,'deadline'); RETURN NULL; END IF;
 SELECT coalesce(max(ordinal),0)+1 INTO v_ordinal FROM live.media_input_wire_steps WHERE attempt_id=i.attempt_id;
 IF v_ordinal>8 THEN PERFORM live.hold_browser_input(i.attempt_id,'budget_exhausted'); RETURN NULL; END IF;
 IF v_ordinal IN (5,6) AND (SELECT count(*) FROM live.media_input_wire_steps
  WHERE attempt_id=i.attempt_id AND ordinal IN (3,4) AND result='PRESENT'
   AND result_at>=v_now-interval '30 seconds')<>2 THEN
  PERFORM live.hold_browser_input(i.attempt_id,'unsafe_retry'); RETURN NULL; END IF;
 v_action:=CASE (v_ordinal-1)%4 WHEN 0 THEN 'REMOVE' WHEN 1 THEN 'DELETE_ROOM'
  WHEN 2 THEN 'READ_PARTICIPANT' ELSE 'READ_ROOM' END;
 INSERT INTO live.media_input_wire_steps(attempt_id,ordinal,operation_id,generation,action,reserved_at)
 VALUES(i.attempt_id,v_ordinal,o.id,p_generation,v_action,v_now);
 IF o.lease_until<=clock_timestamp() OR i.admission_closed_at+interval '180 seconds'<=clock_timestamp() THEN
  RAISE EXCEPTION 'browser input lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN jsonb_build_object('ordinal',v_ordinal,'action',v_action,'room_name',i.room_name,
  'publisher_identity',i.publisher_identity,'project_id',i.project_id,'endpoint_identity',i.endpoint_identity,
  'credential_version',i.credential_version,'revoke_before',
  CASE WHEN v_action='REMOVE' THEN floor(extract(epoch FROM v_now))::bigint ELSE NULL END);
END $$;
ALTER FUNCTION live.reserve_media_input_cleanup(uuid,bigint,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.reserve_media_input_cleanup(uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.reserve_media_input_cleanup(uuid,bigint,bytea) TO commerce_media_executor;

CREATE FUNCTION live.record_media_input_cleanup(p_id uuid,p_generation bigint,p_token bytea,
 p_ordinal integer,p_result text,p_sid text) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; w live.media_input_wire_steps%ROWTYPE;
BEGIN
 IF p_ordinal IS NULL OR p_ordinal NOT BETWEEN 1 AND 8 OR p_result IS NULL OR p_sid IS NULL THEN
  RAISE EXCEPTION 'invalid browser input result' USING ERRCODE='ME400'; END IF;
 o:=live.lock_browser_input_operation(p_id,p_generation,p_token);
 SELECT * INTO w FROM live.media_input_wire_steps WHERE attempt_id=o.media_attempt_id AND ordinal=p_ordinal FOR UPDATE;
 IF w.attempt_id IS NULL OR w.operation_id<>p_id OR w.generation<>p_generation OR w.result<>'PENDING'
  OR o.lease_mode<>'reconcile' THEN
  RAISE EXCEPTION 'browser input step unavailable' USING ERRCODE='ME409'; END IF;
 IF (w.action IN ('REMOVE','DELETE_ROOM') AND (p_result NOT IN ('ACK','UNKNOWN') OR p_sid<>''))
  OR (w.action='READ_PARTICIPANT' AND (p_result NOT IN ('PRESENT','UNKNOWN')
   OR (p_result='PRESENT' AND p_sid !~ '^PA_[A-Za-z0-9_-]{1,100}$') OR (p_result='UNKNOWN' AND p_sid<>'')))
  OR (w.action='READ_ROOM' AND (p_result NOT IN ('PRESENT','ABSENT','UNKNOWN')
   OR (p_result='PRESENT' AND p_sid !~ '^RM_[A-Za-z0-9_-]{1,100}$') OR (p_result<>'PRESENT' AND p_sid<>''))) THEN
  RAISE EXCEPTION 'invalid browser input result' USING ERRCODE='ME400'; END IF;
 UPDATE live.media_input_wire_steps SET result=p_result,resource_sid=p_sid,result_at=clock_timestamp()
 WHERE attempt_id=w.attempt_id AND ordinal=w.ordinal;
 -- Local room absence/TTL is never proof of cached-token revocation.
 IF w.ordinal=8 THEN PERFORM live.hold_browser_input(w.attempt_id,'budget_exhausted');
 ELSIF w.ordinal=4 AND (p_result<>'PRESENT' OR NOT EXISTS
  (SELECT 1 FROM live.media_input_wire_steps WHERE attempt_id=w.attempt_id AND ordinal=3 AND result='PRESENT')) THEN
  PERFORM live.hold_browser_input(w.attempt_id,'revocation_unproven'); END IF;
 RETURN live.finish_media_input_turn(p_id,p_generation,p_token,'cleanup_unknown');
END $$;
ALTER FUNCTION live.record_media_input_cleanup(uuid,bigint,bytea,integer,text,text) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.record_media_input_cleanup(uuid,bigint,bytea,integer,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.record_media_input_cleanup(uuid,bigint,bytea,integer,text,text) TO commerce_media_executor;

CREATE FUNCTION live.media_browser_input_worker_ready() RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$ SELECT false $$;
ALTER FUNCTION live.media_browser_input_worker_ready() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_browser_input_worker_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.media_browser_input_worker_ready() TO commerce_media_executor,commerce_media_worker;
