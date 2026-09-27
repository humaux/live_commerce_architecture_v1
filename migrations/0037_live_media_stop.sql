-- Bounded MOCK Stop is a cleanup phase of the original Start operation/job.
-- All new columns default to no intent; upgrading old attempts grants no Stop wire.
ALTER TABLE live.media_execution_state
 ADD COLUMN claim_started_at timestamptz,
 ADD COLUMN stop_requested_at timestamptz,
 ADD COLUMN stop_requested_by uuid,
 ADD COLUMN stop_wire_count smallint NOT NULL DEFAULT 0,
 ADD COLUMN stop_first_reserved_at timestamptz,
 ADD COLUMN stop_last_reserved_at timestamptz,
 ADD COLUMN stop_first_generation bigint,
 ADD COLUMN stop_last_generation bigint,
 ADD COLUMN stop_observation_id uuid,
 ADD COLUMN stop_exhausted_at timestamptz;
ALTER TABLE live.media_execution_state
 ADD CONSTRAINT media_stop_request_pair CHECK ((stop_requested_at IS NULL)=(stop_requested_by IS NULL)),
 ADD CONSTRAINT media_stop_request_member FOREIGN KEY (tenant_id,stop_requested_by)
  REFERENCES identity.memberships(tenant_id,principal_id),
 ADD CONSTRAINT media_stop_budget CHECK (stop_wire_count BETWEEN 0 AND 2 AND (
  (stop_wire_count=0 AND stop_first_reserved_at IS NULL AND stop_last_reserved_at IS NULL
   AND stop_first_generation IS NULL AND stop_last_generation IS NULL AND stop_observation_id IS NULL)
  OR (stop_wire_count=1 AND stop_first_reserved_at IS NOT NULL
   AND stop_last_reserved_at IS NOT NULL AND stop_first_generation IS NOT NULL
   AND stop_last_generation IS NOT NULL
   AND stop_first_reserved_at=stop_last_reserved_at AND stop_first_generation=stop_last_generation
   AND stop_first_generation>0 AND stop_observation_id IS NOT NULL)
  OR (stop_wire_count=2 AND stop_first_reserved_at IS NOT NULL AND stop_last_reserved_at IS NOT NULL
   AND stop_first_generation IS NOT NULL AND stop_last_generation IS NOT NULL
   AND stop_last_reserved_at>=stop_first_reserved_at+interval '5 seconds'
   AND stop_first_generation>0 AND stop_last_generation>stop_first_generation
   AND stop_observation_id IS NOT NULL))
  AND (stop_exhausted_at IS NULL OR stop_wire_count=2));
ALTER TABLE live.media_observations DROP CONSTRAINT media_observations_source_check;
ALTER TABLE live.media_observations ADD CONSTRAINT media_observations_source_check
 CHECK (source IN ('START','ROOM','QUERY','STOP'));
ALTER TABLE live.media_observations
 ADD CONSTRAINT media_observation_stop_scope UNIQUE
 (id,tenant_id,store_id,attempt_id,operation_id);
ALTER TABLE live.media_execution_state
 ADD CONSTRAINT media_stop_observation_fk FOREIGN KEY
 (stop_observation_id,tenant_id,store_id,attempt_id,operation_id)
 REFERENCES live.media_observations(id,tenant_id,store_id,attempt_id,operation_id)
 DEFERRABLE INITIALLY IMMEDIATE;
GRANT UPDATE(claim_started_at,stop_requested_at,stop_requested_by,stop_wire_count,
 stop_first_reserved_at,stop_last_reserved_at,stop_first_generation,stop_last_generation,
 stop_observation_id,stop_exhausted_at) ON live.media_execution_state TO commerce_media_writer;

-- The old trigger still protects immutable attempt/target fields. This second
-- trigger protects the sticky Stop request and monotone, at-most-two budget.
CREATE FUNCTION live.guard_media_stop_projection() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_observation live.media_observations%ROWTYPE;
BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.stop_wire_count<>0 OR NEW.stop_requested_at IS NOT NULL
   OR NEW.claim_started_at IS NOT NULL OR NEW.stop_exhausted_at IS NOT NULL THEN
   RAISE EXCEPTION 'media stop projection unavailable' USING ERRCODE='ME409'; END IF;
  RETURN NEW;
 END IF;
 IF (OLD.stop_requested_at IS NOT NULL AND
  (NEW.stop_requested_at,NEW.stop_requested_by) IS DISTINCT FROM
  (OLD.stop_requested_at,OLD.stop_requested_by))
  OR NEW.stop_wire_count<OLD.stop_wire_count OR NEW.stop_wire_count>OLD.stop_wire_count+1
  OR (OLD.stop_wire_count>=1 AND
   (NEW.stop_first_reserved_at,NEW.stop_first_generation) IS DISTINCT FROM
   (OLD.stop_first_reserved_at,OLD.stop_first_generation))
  OR (OLD.stop_wire_count=NEW.stop_wire_count AND OLD.stop_wire_count>0 AND
   (NEW.stop_last_reserved_at,NEW.stop_last_generation,NEW.stop_observation_id)
    IS DISTINCT FROM (OLD.stop_last_reserved_at,OLD.stop_last_generation,OLD.stop_observation_id))
  OR (OLD.stop_exhausted_at IS NOT NULL AND NEW.stop_exhausted_at IS DISTINCT FROM OLD.stop_exhausted_at)
  OR (OLD.claim_started_at IS NOT NULL AND NEW.claim_started_at IS NOT NULL
   AND NEW.claim_started_at<OLD.claim_started_at) THEN
  RAISE EXCEPTION 'media stop projection unavailable' USING ERRCODE='ME409'; END IF;
 IF NEW.stop_wire_count>OLD.stop_wire_count OR
  NEW.stop_observation_id IS DISTINCT FROM OLD.stop_observation_id THEN
  SELECT * INTO v_observation FROM live.media_observations WHERE id=NEW.stop_observation_id;
  IF v_observation.id IS NULL OR v_observation.source<>'QUERY'
   OR (v_observation.tenant_id,v_observation.store_id,v_observation.attempt_id,
    v_observation.operation_id,v_observation.project_id,v_observation.generation,
    v_observation.egress_id)
   IS DISTINCT FROM (NEW.tenant_id,NEW.store_id,NEW.attempt_id,NEW.operation_id,
    NEW.project_id,NEW.stop_last_generation,NEW.egress_id)
   OR v_observation.room_name<>'lc_'||replace(NEW.attempt_id::text,'-','') THEN
   RAISE EXCEPTION 'media stop evidence unavailable' USING ERRCODE='ME409'; END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_media_stop_projection() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_media_stop_projection() FROM PUBLIC;
CREATE TRIGGER media_stop_projection BEFORE INSERT OR UPDATE ON live.media_execution_state
 FOR EACH ROW EXECUTE FUNCTION live.guard_media_stop_projection();

-- Merchant authority is rechecked after all binding, state and event waits.
CREATE FUNCTION live.request_media_stop(p_auth_hash bytea,p_store uuid,p_session uuid,p_attempt uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_access record; v_final record; v_tenant uuid; v_principal uuid; v_revision bigint;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 v_binding uuid; v_now timestamptz; v_expiry timestamptz; v_state text;
BEGIN
 IF p_auth_hash IS NULL OR octet_length(p_auth_hash)<>32 OR p_store IS NULL OR p_session IS NULL
  OR p_attempt IS NULL OR p_store='00000000-0000-0000-0000-000000000000'::uuid
  OR p_session='00000000-0000-0000-0000-000000000000'::uuid
  OR p_attempt='00000000-0000-0000-0000-000000000000'::uuid
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media stop request' USING ERRCODE='MP400'; END IF;
 SELECT * INTO v_access FROM identity.resolve_access(p_auth_hash,p_store,'live:manage');
 IF v_access.access_status='unauthorized' THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP401'; END IF;
 IF v_access.access_status<>'ok' THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403'; END IF;
 v_tenant:=v_access.tenant_id; v_principal:=v_access.principal_id;
 v_revision:=v_access.authz_revision;
 IF v_tenant IS NULL OR v_principal IS NULL OR v_revision IS NULL OR v_revision<1
  OR v_tenant IS DISTINCT FROM nullif(current_setting('app.tenant_id',true),'')::uuid
  OR p_store IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid
  OR v_principal IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
  OR v_revision IS DISTINCT FROM nullif(current_setting('app.authz_revision',true),'')::bigint THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403'; END IF;
 SELECT * INTO a FROM live.media_attempts WHERE id=p_attempt;
 IF a.id IS NULL OR a.tenant_id<>v_tenant OR a.store_id<>p_store OR a.session_id<>p_session THEN
  RAISE EXCEPTION 'media attempt unavailable' USING ERRCODE='MP404'; END IF;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 IF h.id IS NULL OR h.attempt_id<>a.id OR h.environment<>'MOCK'
  OR a.environment<>'MOCK' OR a.execution_profile<>'PROVIDER_MOCK' THEN
  RAISE EXCEPTION 'media attempt unavailable' USING ERRCODE='MP409'; END IF;
 FOR v_binding IN SELECT DISTINCT id FROM (
  SELECT h.media_binding_id AS id UNION ALL
  SELECT d.binding_id FROM live.media_authorization_destinations d WHERE d.authorization_id=h.id
 ) ids ORDER BY id LOOP
  PERFORM 1 FROM integration.bindings WHERE id=v_binding FOR SHARE;
  IF NOT FOUND THEN RAISE EXCEPTION 'media binding unavailable' USING ERRCODE='MP409'; END IF;
 END LOOP;
 PERFORM 1 FROM control.tenants WHERE id=v_tenant AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='MP403'; END IF;
 PERFORM 1 FROM control.stores WHERE tenant_id=v_tenant AND id=p_store AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media scope unavailable' USING ERRCODE='MP403'; END IF;
 PERFORM 1 FROM live.sessions WHERE tenant_id=v_tenant AND store_id=p_store AND id=p_session FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media session unavailable' USING ERRCODE='MP404'; END IF;
 PERFORM 1 FROM live.programs WHERE tenant_id=v_tenant AND store_id=p_store
  AND id=a.program_id AND session_id=p_session FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'media program unavailable' USING ERRCODE='MP404'; END IF;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id FOR SHARE;
 SELECT * INTO a FROM live.media_attempts WHERE id=p_attempt FOR SHARE;
 SELECT * INTO o FROM integration.operations WHERE id=a.start_operation_id FOR UPDATE;
 IF h.id IS NULL OR a.id IS NULL OR o.id IS NULL OR a.id<>p_attempt
  OR a.tenant_id<>v_tenant OR a.store_id<>p_store OR a.session_id<>p_session
  OR h.id<>a.authorization_id OR h.attempt_id<>a.id OR o.media_attempt_id<>a.id
  OR o.actor_kind<>'MEDIA_ATTEMPT' OR o.action<>'livekit.egress.start'
  OR o.principal_id<>a.original_principal_id OR o.job_id<1 THEN
  RAISE EXCEPTION 'media stop unavailable' USING ERRCODE='MP409'; END IF;
 INSERT INTO live.media_execution_state(attempt_id,tenant_id,store_id,session_id,
  authorization_id,operation_id,project_id)
 VALUES(a.id,a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id)
 ON CONFLICT (attempt_id) DO NOTHING;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=a.id FOR UPDATE;
 IF x.attempt_id IS NULL OR (x.tenant_id,x.store_id,x.session_id,x.authorization_id,x.operation_id,x.project_id)
  IS DISTINCT FROM (a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id) THEN
  RAISE EXCEPTION 'media stop unavailable' USING ERRCODE='MP409'; END IF;
 v_now:=clock_timestamp();
 IF x.stop_requested_at IS NOT NULL THEN
  v_state:=CASE WHEN o.state='CANCELLED' AND x.wire_reserved_at IS NULL THEN 'cancelled_before_start'
   WHEN x.escalated_at IS NOT NULL THEN 'escalated'
   WHEN x.resource_state='TERMINAL' OR o.state IN ('SUCCEEDED','FAILED_FINAL','BLOCKED_POLICY','STALE_BINDING')
    THEN 'terminal' ELSE 'requested' END;
 ELSIF x.escalated_at IS NOT NULL THEN v_state:='escalated';
 ELSIF x.resource_state='TERMINAL' OR o.state IN
  ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING') THEN v_state:='terminal';
 ELSE
  UPDATE live.media_execution_state SET stop_requested_at=v_now,stop_requested_by=v_principal,
   cleanup_required=true,updated_at=v_now WHERE attempt_id=a.id;
  v_state:='requested';
  IF x.wire_reserved_at IS NULL THEN
   UPDATE integration.operations SET state='CANCELLED',generation=o.generation+1,
    lease_mode='',lease_until=NULL,lease_token_hash=NULL,result_code='media_cancelled_before_start',updated_at=v_now
    WHERE id=o.id;
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
    VALUES(o.tenant_id,o.store_id,o.id,o.generation+1,'CANCELLED','','media_cancelled_before_start');
   v_state:='cancelled_before_start';
  ELSE
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
    VALUES(o.tenant_id,o.store_id,o.id,greatest(o.generation,1),o.state,coalesce(o.lease_mode,''),
     'media_stop_requested');
  END IF;
 END IF;
 -- A fresh inner authorization and token-expiry read follow every lock/event wait.
 SELECT * INTO v_final FROM identity.resolve_access(p_auth_hash,p_store,'live:manage');
 SELECT expires_at INTO v_expiry FROM identity.sessions
  WHERE token_hash=p_auth_hash AND audience='merchant' AND revoked_at IS NULL;
 v_now:=clock_timestamp();
 IF v_final.access_status='unauthorized' OR v_expiry IS NULL OR v_expiry<=v_now THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP401'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM v_tenant
  OR v_final.principal_id IS DISTINCT FROM v_principal
  OR v_final.authz_revision IS DISTINCT FROM v_revision THEN
  RAISE EXCEPTION 'media access unavailable' USING ERRCODE='MP403'; END IF;
 RETURN jsonb_build_object('session_id',p_session::text,'attempt_id',a.id::text,
  'operation_id',o.id::text,'state',v_state);
END $$;
ALTER FUNCTION live.request_media_stop(bytea,uuid,uuid,uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.request_media_stop(bytea,uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.request_media_stop(bytea,uuid,uuid,uuid) TO commerce_runtime;
-- Existing public entry point remains byte-for-byte compatible. All projections,
-- including cleanup Query and Stop replies, pass one private validator/merger.
ALTER FUNCTION live.record_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 RENAME TO project_media_observation;
REVOKE ALL ON FUNCTION live.project_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 FROM commerce_media_executor;
CREATE OR REPLACE FUNCTION live.project_media_observation(p_id uuid,p_generation bigint,p_token bytea,
 p_source text,p_egress text,p_room text,p_status text,p_started bigint,p_updated bigint,p_ended bigint)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 v_terminal boolean; v_rank integer; v_old_rank integer; v_coherent boolean;
 v_started bigint; v_updated bigint; v_ended bigint;
 v_hash bytea; v_now timestamptz; v_cleanup boolean;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL OR octet_length(p_token)<>32
  OR p_source IS NULL OR p_source NOT IN ('START','ROOM','QUERY','STOP')
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
  OR (p_source='QUERY' AND (o.lease_mode<>'reconcile' OR x.egress_id IS NULL OR x.egress_id<>p_egress))
  OR (p_source='STOP' AND (o.lease_mode<>'reconcile' OR x.egress_id IS NULL
   OR x.egress_id<>p_egress OR x.stop_wire_count<1 OR x.stop_last_generation<>p_generation
   OR x.stop_observation_id IS NULL)) THEN
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
 IF NOT v_cleanup AND live.media_lifetime_revoked(o.id) THEN
  UPDATE live.media_execution_state SET cleanup_required=true,updated_at=clock_timestamp()
   WHERE attempt_id=a.id;
 END IF;
 IF o.lease_until<=clock_timestamp() THEN RAISE EXCEPTION 'media lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN CASE WHEN v_terminal THEN 'terminal' ELSE 'observe' END;
END $$;
CREATE FUNCTION live.record_media_observation(p_id uuid,p_generation bigint,p_token bytea,
 p_source text,p_egress text,p_room text,p_status text,p_started bigint,p_updated bigint,p_ended bigint)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_result text; v_cleanup boolean;
BEGIN
 v_result:=live.project_media_observation(p_id,p_generation,p_token,p_source,p_egress,p_room,
  p_status,p_started,p_updated,p_ended);
 IF p_source='QUERY' AND v_result<>'terminal' THEN
  SELECT x.cleanup_required INTO v_cleanup FROM live.media_execution_state x
   WHERE x.operation_id=p_id;
  -- An already-running old worker lacks the atomic cleanup entry point.
  -- Fail closed rather than silently observing forever; terminal proof survives.
  IF v_cleanup THEN
   RAISE EXCEPTION 'media cleanup requires current worker' USING ERRCODE='ME409'; END IF;
 END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION live.record_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.record_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.record_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 TO commerce_media_executor;

-- This synchronous Query result is the only authority for a Stop reservation.
-- The private projector releases the lease inside this transaction; on a
-- successful reservation we restore it before COMMIT, never exposing a gap.
CREATE FUNCTION live.record_media_cleanup_query(p_id uuid,p_generation bigint,p_token bytea,
 p_egress text,p_room text,p_status text,p_started bigint,p_updated bigint,p_ended bigint)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 v_result text; v_observation live.media_observations%ROWTYPE;
 v_now timestamptz; v_reserved boolean:=false;
BEGIN
 -- The projector validates every untrusted report and exact frozen target.
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<1 OR p_token IS NULL
  OR octet_length(p_token)<>32 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media cleanup query' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_id);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 IF x.attempt_id IS NULL OR x.operation_id<>o.id OR x.wire_reserved_at IS NULL
  OR x.escalated_at IS NOT NULL OR x.resource_state='TERMINAL'
  OR o.generation<>p_generation OR o.state<>'UNKNOWN' OR o.lease_mode<>'reconcile'
  OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN
  RAISE EXCEPTION 'media lease unavailable' USING ERRCODE='ME409'; END IF;
 v_result:=live.project_media_observation(p_id,p_generation,p_token,'QUERY',
  p_egress,p_room,p_status,p_started,p_updated,p_ended);
 IF v_result='terminal' THEN RETURN 'terminal'; END IF;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO v_observation FROM live.media_observations
  WHERE tenant_id=o.tenant_id AND store_id=o.store_id AND attempt_id=a.id
   AND operation_id=o.id AND generation=p_generation AND source='QUERY'
   AND project_id=h.project_id AND room_name=a.room_name AND egress_id=p_egress
   AND status=p_status AND started_at_ns=p_started AND updated_at_ns=p_updated
   AND ended_at_ns=p_ended ORDER BY observed_at DESC LIMIT 1;
 IF v_observation.id IS NULL OR x.egress_id IS DISTINCT FROM p_egress THEN
  RAISE EXCEPTION 'media cleanup evidence unavailable' USING ERRCODE='ME409'; END IF;
 IF x.stop_wire_count=2 THEN
  IF x.stop_exhausted_at IS NULL THEN
   v_now:=clock_timestamp();
   UPDATE live.media_execution_state SET stop_exhausted_at=v_now,updated_at=v_now WHERE attempt_id=a.id;
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
    VALUES(o.tenant_id,o.store_id,o.id,p_generation,'UNKNOWN','','media_stop_budget_exhausted');
   IF o.lease_until<=clock_timestamp() THEN
    RAISE EXCEPTION 'media lease unavailable' USING ERRCODE='ME409'; END IF;
  END IF;
  RETURN 'observe';
 END IF;
 v_now:=clock_timestamp();
 IF NOT x.cleanup_required OR x.escalated_at IS NOT NULL OR x.resource_state<>'OBSERVED'
  OR x.transport_status NOT IN ('EGRESS_STARTING','EGRESS_ACTIVE')
  OR (x.stop_wire_count=1 AND (p_status<>'EGRESS_ACTIVE'
   OR x.transport_status<>'EGRESS_ACTIVE'
   OR p_generation<=x.stop_first_generation
   OR v_now<x.stop_first_reserved_at+interval '5 seconds'))
  OR (x.stop_wire_count=0 AND p_status NOT IN ('EGRESS_STARTING','EGRESS_ACTIVE'))
  OR x.claim_started_at IS NULL OR v_now<x.claim_started_at
  OR v_now>x.claim_started_at+interval '5 seconds'
  OR o.lease_until<=v_now THEN
  RETURN 'observe';
 END IF;
 -- A nested subtransaction preserves the valid Query if the final DB-clock
 -- gate expires after event writes. It never leaks a stale wire permission.
 BEGIN
  UPDATE live.media_execution_state SET
   stop_wire_count=x.stop_wire_count+1,
   stop_first_reserved_at=CASE WHEN x.stop_wire_count=0 THEN v_now ELSE x.stop_first_reserved_at END,
   stop_last_reserved_at=v_now,
   stop_first_generation=CASE WHEN x.stop_wire_count=0 THEN p_generation ELSE x.stop_first_generation END,
   stop_last_generation=p_generation,stop_observation_id=v_observation.id,updated_at=v_now
   WHERE attempt_id=a.id;
  UPDATE integration.operations SET state='UNKNOWN',lease_mode='reconcile',
   lease_until=o.lease_until,lease_token_hash=o.lease_token_hash,
   result_code='media_stop_reserved',updated_at=v_now WHERE id=o.id;
  INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
   VALUES(o.tenant_id,o.store_id,o.id,p_generation,'UNKNOWN','reconcile','media_stop_reserved');
  SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=a.id;
  SELECT * INTO v_observation FROM live.media_observations WHERE id=x.stop_observation_id;
  v_now:=clock_timestamp();
  IF o.lease_until<=v_now OR x.claim_started_at IS NULL OR v_now<x.claim_started_at
   OR v_now>x.claim_started_at+interval '5 seconds'
   OR x.stop_last_generation<>p_generation OR v_observation.id IS NULL
   OR v_observation.source<>'QUERY' OR v_observation.generation<>p_generation
   OR (v_observation.tenant_id,v_observation.store_id,v_observation.attempt_id,
    v_observation.operation_id,v_observation.project_id,v_observation.room_name,
    v_observation.egress_id)
   IS DISTINCT FROM (o.tenant_id,o.store_id,a.id,o.id,h.project_id,a.room_name,p_egress)
   OR (x.stop_wire_count=2 AND (v_now<x.stop_first_reserved_at+interval '5 seconds'
    OR p_generation<=x.stop_first_generation))
   OR x.transport_status NOT IN ('EGRESS_STARTING','EGRESS_ACTIVE')
   OR (x.stop_wire_count=2 AND x.transport_status<>'EGRESS_ACTIVE') THEN
   RAISE EXCEPTION 'media stop gate expired' USING ERRCODE='LMR01'; END IF;
  v_reserved:=true;
 EXCEPTION WHEN SQLSTATE 'LMR01' THEN
  v_reserved:=false;
 END;
 RETURN CASE WHEN v_reserved THEN 'stop_reserved' ELSE 'observe' END;
END $$;
ALTER FUNCTION live.record_media_cleanup_query(uuid,bigint,bytea,text,text,text,bigint,bigint,bigint)
 OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.record_media_cleanup_query(uuid,bigint,bytea,text,text,text,bigint,bigint,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.record_media_cleanup_query(uuid,bigint,bytea,text,text,text,bigint,bigint,bigint)
 TO commerce_media_executor;

-- Preserve the old claim signature and modes, but timestamp every successful
-- claim in the same DB transaction and fence explicit pre-Start cancellation.
CREATE OR REPLACE FUNCTION live.claim_media_operation(p_id uuid,p_job bigint,p_lease_seconds integer,p_token bytea)
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
  IF x.stop_requested_at IS NOT NULL THEN
   RAISE EXCEPTION 'media dispatch cancelled' USING ERRCODE='ME409'; END IF;
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
 UPDATE live.media_execution_state SET claim_started_at=v_now,updated_at=v_now WHERE attempt_id=a.id;
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

-- Explicit cleanup cannot be converted back into Start by a worker holding
-- previously loaded material.
CREATE OR REPLACE FUNCTION live.load_media_material(p_id uuid,p_generation bigint,p_token bytea) RETURNS jsonb
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
  IF x.wire_reserved_at IS NOT NULL OR x.stop_requested_at IS NOT NULL
   OR NOT live.media_dispatch_eligible(o.id) THEN
   RAISE EXCEPTION 'media material unavailable' USING ERRCODE='ME409'; END IF;
  v_key:=h.key_id; v_nonce:=encode(h.nonce,'hex'); v_ciphertext:=encode(h.ciphertext,'hex');
 ELSE
  IF x.wire_reserved_at IS NULL THEN RAISE EXCEPTION 'media material unavailable' USING ERRCODE='ME409'; END IF;
  v_key:=''; v_nonce:=''; v_ciphertext:='';
 END IF;
 IF o.lease_until<=clock_timestamp() OR (o.lease_mode='dispatch' AND (x.stop_requested_at IS NOT NULL
  OR NOT live.media_dispatch_eligible(o.id))) THEN
  RAISE EXCEPTION 'media material unavailable' USING ERRCODE='ME409'; END IF;
 RETURN jsonb_build_object('tenant_id',a.tenant_id::text,'store_id',a.store_id::text,
  'session_id',a.session_id::text,'attempt_id',a.id::text,'project_id',h.project_id,
  'endpoint_identity',h.endpoint_identity,'credential_version',h.credential_version,
  'material_version',h.material_version,'room_name',a.room_name,'aspect_ratio',h.aspect_ratio,
  'egress_id',coalesce(x.egress_id,''),'mode',o.lease_mode,'key_id',v_key,
  'nonce_hex',v_nonce,'ciphertext_hex',v_ciphertext);
END $$;
ALTER FUNCTION live.load_media_material(uuid,bigint,bytea) OWNER TO commerce_media_writer;

CREATE OR REPLACE FUNCTION live.reserve_media_start(p_id uuid,p_generation bigint,p_token bytea) RETURNS void
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
  OR x.stop_requested_at IS NOT NULL OR NOT live.media_dispatch_eligible(o.id) THEN
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
