-- BRW07: one recovery episode, two independent immutable observation receipts.
ALTER TABLE live.media_recovery_episode_scope
 ADD COLUMN include_browser_input boolean NOT NULL DEFAULT false;

ALTER TABLE integration.operation_events
 ADD COLUMN recovery_execution_profile text CHECK
  (recovery_execution_profile IN ('PROVIDER_MOCK','LOCAL_SFU_MOCK_EGRESS')),
 ADD COLUMN recovery_input_required boolean,
 ADD COLUMN recovery_egress_required boolean,
 ADD COLUMN input_observation_id uuid,
 ADD CONSTRAINT media_recovery_admitted_mask CHECK
  (episode_event_kind<>'admitted' OR recovery_execution_profile IS NULL OR
   (recovery_input_required IS NOT NULL AND recovery_egress_required IS NOT NULL
    AND (recovery_input_required OR recovery_egress_required)
    AND (recovery_execution_profile='LOCAL_SFU_MOCK_EGRESS' OR NOT recovery_input_required)));

ALTER TABLE integration.operation_events DROP CONSTRAINT operation_events_episode_event_kind_check;
ALTER TABLE integration.operation_events ADD CONSTRAINT operation_events_episode_event_kind_check
 CHECK (episode_event_kind IN ('admitted','terminal_at_lock','native_ineligible','ceiling',
  'qualified','input_qualified','witnessed','timeout'));

CREATE TABLE live.media_input_recovery_observations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 episode_id uuid NOT NULL REFERENCES live.media_recovery_episode_scope(episode_id),
 tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 operation_id uuid NOT NULL, attempt_id uuid NOT NULL, job_id bigint NOT NULL CHECK (job_id>0),
 generation bigint NOT NULL CHECK (generation>0),
 project_id text NOT NULL, credential_version bigint NOT NULL CHECK (credential_version>0),
 endpoint_identity text NOT NULL, room_name text NOT NULL,
 publisher_identity text NOT NULL,
 source text NOT NULL CHECK (source IN ('PARTICIPANT','ROOM')),
 result text NOT NULL CHECK (result IN ('PRESENT','ABSENT')),
 participant_sid text, participant_state text,
 observed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE (episode_id,operation_id),
 FOREIGN KEY (tenant_id,store_id,operation_id)
  REFERENCES integration.operations(tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,store_id,attempt_id)
  REFERENCES live.media_input_custody(tenant_id,store_id,attempt_id),
 CHECK (((source='PARTICIPANT' AND result='PRESENT'
   AND participant_sid ~ '^PA_[A-Za-z0-9_-]{1,100}$'
   AND participant_state IN ('JOINING','JOINED','ACTIVE','DISCONNECTED')) OR
  (source='ROOM' AND result='ABSENT' AND participant_sid IS NULL AND participant_state IS NULL)) IS TRUE)
);
ALTER TABLE live.media_input_recovery_observations ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.media_input_recovery_observations FORCE ROW LEVEL SECURITY;
CREATE POLICY media_input_recovery_writer ON live.media_input_recovery_observations
 FOR ALL TO commerce_media_writer USING (true) WITH CHECK (true);
REVOKE ALL ON live.media_input_recovery_observations FROM PUBLIC;
GRANT SELECT,INSERT ON live.media_input_recovery_observations TO commerce_media_writer;

-- The event and receipt are inserted by the same definer transaction. These
-- guards also protect against an owner maintenance statement drifting identity.
CREATE FUNCTION live.guard_media_input_recovery_receipt() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a live.media_attempts%ROWTYPE; i live.media_input_custody%ROWTYPE;
 e integration.operation_events%ROWTYPE; s live.media_recovery_episode_scope%ROWTYPE;
BEGIN
 IF TG_OP<>'INSERT' THEN
  RAISE EXCEPTION 'input recovery receipt immutable' USING ERRCODE='ME409'; END IF;
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=NEW.episode_id;
 SELECT * INTO e FROM integration.operation_events WHERE episode_id=NEW.episode_id
  AND operation_id=NEW.operation_id AND episode_event_kind='admitted';
 SELECT * INTO a FROM live.media_attempts WHERE id=NEW.attempt_id;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=NEW.attempt_id;
 IF s.include_browser_input IS DISTINCT FROM true OR s.scope_status<>'pending'
  OR s.timeout_at IS NOT NULL OR NEW.observed_at>s.deadline_at
  OR e.id IS NULL
  OR e.native_job_id<>NEW.job_id OR e.generation>=NEW.generation
  OR e.recovery_input_required IS DISTINCT FROM true
  OR e.recovery_execution_profile<>'LOCAL_SFU_MOCK_EGRESS'
  OR a.id IS NULL OR a.execution_profile<>e.recovery_execution_profile
  OR i.attempt_id IS NULL OR i.runtime_version<>1 OR i.operation_id<>NEW.operation_id
  OR i.tenant_id<>NEW.tenant_id OR i.store_id<>NEW.store_id
  OR (i.project_id,i.credential_version,i.endpoint_identity,i.room_name,i.publisher_identity)
   IS DISTINCT FROM
   (NEW.project_id,NEW.credential_version,NEW.endpoint_identity,NEW.room_name,NEW.publisher_identity) THEN
  RAISE EXCEPTION 'input recovery receipt unavailable' USING ERRCODE='ME409'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.guard_media_input_recovery_receipt() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.guard_media_input_recovery_receipt() FROM PUBLIC;
CREATE TRIGGER media_input_recovery_receipt_guard
 BEFORE INSERT OR UPDATE OR DELETE ON live.media_input_recovery_observations
 FOR EACH ROW EXECUTE FUNCTION live.guard_media_input_recovery_receipt();

-- Existing public signatures remain exact. Kernels are owner-only; every
-- legacy entry checks scope before its historic replay/terminal branches.
CREATE FUNCTION live.assert_legacy_media_recovery_scope(p_episode uuid) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM live.media_recovery_episode_scope
  WHERE episode_id=p_episode AND include_browser_input) THEN
  RAISE EXCEPTION 'mixed media recovery scope' USING ERRCODE='ME409'; END IF;
END $$;
ALTER FUNCTION live.assert_legacy_media_recovery_scope(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.assert_legacy_media_recovery_scope(uuid) FROM PUBLIC;

ALTER FUNCTION live.begin_media_recovery_episode(uuid,bigint,integer,boolean) RENAME TO begin_media_recovery_episode_kernel;
REVOKE ALL ON FUNCTION live.begin_media_recovery_episode_kernel(uuid,bigint,integer,boolean) FROM commerce_media_recovery;
CREATE FUNCTION live.begin_media_recovery_episode(p_episode uuid,p_elapsed bigint,p_capacity integer,p_known boolean)
RETURNS TABLE(disposition text,episode_id uuid,operation_id uuid,job_id bigint,
 baseline_generation bigint,deadline_at timestamptz,candidate_count integer,
 coverage_known boolean,blocked_by_episode_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF p_episode IS NOT NULL THEN
  PERFORM pg_catalog.pg_advisory_xact_lock(44,pg_catalog.hashtext(p_episode::text)); END IF;
 PERFORM live.assert_legacy_media_recovery_scope(p_episode);
 RETURN QUERY SELECT * FROM live.begin_media_recovery_episode_kernel(p_episode,p_elapsed,p_capacity,p_known);
END $$;
ALTER FUNCTION live.begin_media_recovery_episode(uuid,bigint,integer,boolean) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.begin_media_recovery_episode(uuid,bigint,integer,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.begin_media_recovery_episode(uuid,bigint,integer,boolean) TO commerce_media_recovery;

ALTER FUNCTION live.claim_recovery_observation(uuid,uuid,bigint,bytea) RENAME TO claim_recovery_observation_kernel;
REVOKE ALL ON FUNCTION live.claim_recovery_observation_kernel(uuid,uuid,bigint,bytea) FROM commerce_media_recovery;
CREATE FUNCTION live.claim_recovery_observation(p_episode uuid,p_operation uuid,p_job bigint,p_token bytea)
RETURNS TABLE(disposition text,generation bigint,project_id text,credential_version bigint,
 endpoint_identity text,room_name text,egress_id text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM live.assert_legacy_media_recovery_scope(p_episode);
 RETURN QUERY SELECT * FROM live.claim_recovery_observation_kernel(p_episode,p_operation,p_job,p_token);
END $$;
ALTER FUNCTION live.claim_recovery_observation(uuid,uuid,bigint,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.claim_recovery_observation(uuid,uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.claim_recovery_observation(uuid,uuid,bigint,bytea) TO commerce_media_recovery;

ALTER FUNCTION live.record_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 RENAME TO record_recovery_observation_kernel;
REVOKE ALL ON FUNCTION live.record_recovery_observation_kernel(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 FROM commerce_media_recovery;
CREATE FUNCTION live.record_recovery_observation(p_episode uuid,p_operation uuid,p_generation bigint,
 p_token bytea,p_source text,p_egress text,p_room text,p_status text,
 p_started bigint,p_updated bigint,p_ended bigint)
RETURNS TABLE(disposition text,observation_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM live.assert_legacy_media_recovery_scope(p_episode);
 RETURN QUERY SELECT * FROM live.record_recovery_observation_kernel(p_episode,p_operation,p_generation,
  p_token,p_source,p_egress,p_room,p_status,p_started,p_updated,p_ended);
END $$;
ALTER FUNCTION live.record_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.record_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.record_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 TO commerce_media_recovery;

ALTER FUNCTION live.finish_recovery_observation(uuid,uuid,bigint,bytea,text) RENAME TO finish_recovery_observation_kernel;
REVOKE ALL ON FUNCTION live.finish_recovery_observation_kernel(uuid,uuid,bigint,bytea,text) FROM commerce_media_recovery;
CREATE FUNCTION live.finish_recovery_observation(p_episode uuid,p_operation uuid,p_generation bigint,p_token bytea,p_code text)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM live.assert_legacy_media_recovery_scope(p_episode);
 RETURN live.finish_recovery_observation_kernel(p_episode,p_operation,p_generation,p_token,p_code);
END $$;
ALTER FUNCTION live.finish_recovery_observation(uuid,uuid,bigint,bytea,text) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.finish_recovery_observation(uuid,uuid,bigint,bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.finish_recovery_observation(uuid,uuid,bigint,bytea,text) TO commerce_media_recovery;

ALTER FUNCTION live.read_media_recovery_episode(uuid) RENAME TO read_media_recovery_episode_kernel;
REVOKE ALL ON FUNCTION live.read_media_recovery_episode_kernel(uuid) FROM commerce_media_recovery;
CREATE FUNCTION live.read_media_recovery_episode(p_episode uuid)
RETURNS TABLE(episode_id uuid,scope_status text,coverage_known boolean,candidate_count integer,
 blocked_by_episode_id uuid,operation_id uuid,disposition text,baseline_generation bigint,
 observation_id uuid,observation_source text,observation_generation bigint,
 witness_elapsed_ms bigint,timeout_at timestamptz,cleanup_required boolean)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM live.assert_legacy_media_recovery_scope(p_episode);
 RETURN QUERY SELECT * FROM live.read_media_recovery_episode_kernel(p_episode);
END $$;
ALTER FUNCTION live.read_media_recovery_episode(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.read_media_recovery_episode(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.read_media_recovery_episode(uuid) TO commerce_media_recovery;

ALTER FUNCTION live.witness_media_recovery_episode(uuid,uuid,uuid,bigint) RENAME TO witness_media_recovery_episode_kernel;
REVOKE ALL ON FUNCTION live.witness_media_recovery_episode_kernel(uuid,uuid,uuid,bigint) FROM commerce_media_recovery;
CREATE FUNCTION live.witness_media_recovery_episode(p_episode uuid,p_operation uuid,p_observation uuid,p_elapsed bigint)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM live.assert_legacy_media_recovery_scope(p_episode);
 RETURN live.witness_media_recovery_episode_kernel(p_episode,p_operation,p_observation,p_elapsed);
END $$;
ALTER FUNCTION live.witness_media_recovery_episode(uuid,uuid,uuid,bigint) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.witness_media_recovery_episode(uuid,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.witness_media_recovery_episode(uuid,uuid,uuid,bigint) TO commerce_media_recovery;

CREATE FUNCTION live.begin_media_recovery_episode_with_input(p_episode uuid,p_elapsed_ms bigint,
 p_capacity integer,p_coverage_known boolean)
RETURNS TABLE(disposition text,episode_id uuid,operation_id uuid,job_id bigint,
 baseline_generation bigint,deadline_at timestamptz,candidate_count integer,
 coverage_known boolean,blocked_by_episode_id uuid,execution_profile text,
 input_required boolean,egress_required boolean)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s live.media_recovery_episode_scope%ROWTYPE; o integration.operations%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 x live.media_execution_state%ROWTYPE; i live.media_input_custody%ROWTYPE;
 v_ids uuid[]; v_jobs bigint[]; v_profiles text[]; v_inputs boolean[]; v_egresses boolean[];
 v_gens bigint[]:='{}'; v_modes text[]:='{}';
 v_count integer; v_n integer; v_prior uuid; v_entry timestamptz:=clock_timestamp();
 v_now timestamptz; v_mode text; v_input boolean; v_egress boolean;
BEGIN
 IF p_episode IS NULL OR p_elapsed_ms IS NULL OR p_elapsed_ms<0 OR p_capacity IS NULL
  OR p_capacity NOT BETWEEN 1 AND 32 OR p_coverage_known IS NULL
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid mixed media recovery begin' USING ERRCODE='ME400'; END IF;
 PERFORM pg_catalog.pg_advisory_xact_lock(44,pg_catalog.hashtext(p_episode::text));
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=p_episode;
 IF s.episode_id IS NOT NULL THEN
  IF NOT s.include_browser_input THEN
   RAISE EXCEPTION 'mixed media recovery scope unavailable' USING ERRCODE='ME409'; END IF;
  RETURN QUERY SELECT r.disposition,r.episode_id,r.operation_id,r.job_id,r.baseline_generation,
   r.deadline_at,r.candidate_count,r.coverage_known,r.blocked_by_episode_id,
   e.recovery_execution_profile,coalesce(e.recovery_input_required,false),
   coalesce(e.recovery_egress_required,false)
   FROM live.media_recovery_begin_replay(p_episode) r
   LEFT JOIN integration.operation_events e ON e.operation_id=r.operation_id
    AND e.episode_id=p_episode AND e.episode_event_kind='admitted';
  RETURN;
 END IF;
 SELECT array_agg(c.id ORDER BY c.id),array_agg(c.job_id ORDER BY c.id),
  array_agg(c.execution_profile ORDER BY c.id),array_agg(c.input_needed ORDER BY c.id),
  array_agg(c.egress_needed ORDER BY c.id)
  INTO v_ids,v_jobs,v_profiles,v_inputs,v_egresses FROM (
   SELECT co.id,co.job_id,ca.execution_profile,
    (ca.execution_profile='LOCAL_SFU_MOCK_EGRESS' AND ci.grant_iat IS NOT NULL
     AND ci.state<>'CLOSED') AS input_needed,
    (mx.wire_reserved_at IS NOT NULL AND mx.resource_state<>'TERMINAL') AS egress_needed
   FROM integration.operations co
   JOIN live.media_attempts ca ON ca.id=co.media_attempt_id AND ca.start_operation_id=co.id
   LEFT JOIN live.media_execution_state mx ON mx.operation_id=co.id
   LEFT JOIN live.media_input_custody ci ON ci.attempt_id=ca.id AND ci.operation_id=co.id
   WHERE ((ca.execution_profile='PROVIDER_MOCK'
      AND co.state IN ('DISPATCHING','UNKNOWN') AND mx.wire_reserved_at IS NOT NULL
      AND mx.resource_state<>'TERMINAL') OR
     (ca.execution_profile='LOCAL_SFU_MOCK_EGRESS'
      AND co.state IN ('READY','DISPATCHING','UNKNOWN') AND ci.runtime_version=1 AND
      ((ci.grant_iat IS NOT NULL AND ci.state<>'CLOSED') OR
       (mx.wire_reserved_at IS NOT NULL AND mx.resource_state<>'TERMINAL'))))
   ORDER BY co.id LIMIT p_capacity+1
  ) c;
 v_count:=coalesce(array_length(v_ids,1),0);
 IF v_count>p_capacity THEN
  v_now:=clock_timestamp();
  INSERT INTO live.media_recovery_episode_scope(episode_id,capture_elapsed_ms,deadline_at,
   capacity,candidate_count,coverage_known,scope_status,capacity_exceeded_at,include_browser_input)
  VALUES(p_episode,p_elapsed_ms,v_entry+make_interval(secs=>greatest(0,90000-least(p_elapsed_ms,90000))::double precision/1000),
   p_capacity,v_count,p_coverage_known,'capacity_exceeded',v_now,true);
  RETURN QUERY SELECT 'capacity_exceeded'::text,p_episode,NULL::uuid,NULL::bigint,NULL::bigint,
   v_entry+make_interval(secs=>greatest(0,90000-least(p_elapsed_ms,90000))::double precision/1000),
   v_count,p_coverage_known,NULL::uuid,NULL::text,false,false;
  RETURN;
 END IF;
 -- The snapshot bits never shrink while acquiring these business locks.
 FOR v_n IN 1..v_count LOOP
  o:=live.lock_media_operation(v_ids[v_n]);
  SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
  SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
  IF o.job_id IS DISTINCT FROM v_jobs[v_n] OR a.execution_profile IS DISTINCT FROM v_profiles[v_n]
   OR a.start_operation_id<>o.id OR h.id IS NULL THEN
   RAISE EXCEPTION 'mixed media recovery identity changed' USING ERRCODE='ME409'; END IF;
  IF a.execution_profile='LOCAL_SFU_MOCK_EGRESS' THEN
   INSERT INTO live.media_execution_state(attempt_id,tenant_id,store_id,session_id,
    authorization_id,operation_id,project_id)
   VALUES(a.id,a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id)
   ON CONFLICT DO NOTHING;
  END IF;
  SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=a.id FOR UPDATE;
  IF x.attempt_id IS NULL OR x.operation_id<>o.id OR x.project_id<>h.project_id THEN
   RAISE EXCEPTION 'mixed media recovery projection changed' USING ERRCODE='ME409'; END IF;
  IF a.execution_profile='LOCAL_SFU_MOCK_EGRESS' THEN
   SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id FOR UPDATE;
   IF i.attempt_id IS NULL OR i.operation_id<>o.id OR i.runtime_version<>1
    OR i.authorization_id<>h.id OR
    (i.project_id,i.credential_version,i.endpoint_identity,i.room_name)
     IS DISTINCT FROM (h.project_id,h.credential_version,h.endpoint_identity,a.room_name) THEN
    RAISE EXCEPTION 'mixed media recovery input changed' USING ERRCODE='ME409'; END IF;
  ELSE
   i:=NULL;
  END IF;
  IF x.recovery_episode_id IS NOT NULL AND x.recovery_episode_id<>p_episode
   AND EXISTS(SELECT 1 FROM live.media_recovery_episode_scope old_scope
    WHERE old_scope.episode_id=x.recovery_episode_id AND old_scope.timeout_at IS NULL
     AND (NOT old_scope.coverage_known OR
      (old_scope.candidate_count>0 AND (SELECT count(*) FROM integration.operation_events old_member
       WHERE old_member.episode_id=old_scope.episode_id AND old_member.episode_event_kind='admitted')
        <>old_scope.candidate_count) OR
      EXISTS(SELECT 1 FROM integration.operation_events old_member
       WHERE old_member.episode_id=old_scope.episode_id AND old_member.episode_event_kind='admitted'
        AND NOT EXISTS(SELECT 1 FROM integration.operation_events old_witness
         WHERE old_witness.episode_id=old_scope.episode_id
          AND old_witness.operation_id=old_member.operation_id
          AND old_witness.episode_event_kind='witnessed')))) THEN
   v_prior:=x.recovery_episode_id; EXIT; END IF;
  v_input:=v_inputs[v_n] OR (a.execution_profile='LOCAL_SFU_MOCK_EGRESS'
   AND i.grant_iat IS NOT NULL AND i.state<>'CLOSED');
  v_egress:=v_egresses[v_n] OR
   (x.wire_reserved_at IS NOT NULL AND x.resource_state<>'TERMINAL');
  IF NOT (v_input OR v_egress) THEN
   RAISE EXCEPTION 'mixed media recovery mask unavailable' USING ERRCODE='ME409'; END IF;
  v_inputs[v_n]:=v_input; v_egresses[v_n]:=v_egress;
  v_mode:=CASE
   WHEN NOT live.media_recovery_native_eligible(o.job_id,o.id) THEN 'native_ineligible'
   WHEN o.generation>=4096 OR clock_timestamp()>=o.created_at+interval '24 hours' THEN 'ceiling'
   WHEN (NOT v_input OR i.state='CLOSED') AND
    (NOT v_egress OR x.resource_state='TERMINAL' OR x.escalated_at IS NOT NULL)
    THEN 'terminal_at_lock'
   ELSE 'pending' END;
  v_gens:=array_append(v_gens,o.generation);
  v_modes:=array_append(v_modes,v_mode);
 END LOOP;
 v_now:=clock_timestamp();
 IF v_prior IS NOT NULL THEN
  INSERT INTO live.media_recovery_episode_scope(episode_id,capture_elapsed_ms,deadline_at,
   capacity,candidate_count,coverage_known,scope_status,blocked_by_episode_id,include_browser_input)
  VALUES(p_episode,p_elapsed_ms,v_entry+make_interval(secs=>greatest(0,90000-least(p_elapsed_ms,90000))::double precision/1000),
   p_capacity,v_count,p_coverage_known,'prior_unfinished',v_prior,true);
  RETURN QUERY SELECT 'prior_unfinished'::text,p_episode,NULL::uuid,NULL::bigint,NULL::bigint,
   v_entry+make_interval(secs=>greatest(0,90000-least(p_elapsed_ms,90000))::double precision/1000),
   v_count,p_coverage_known,v_prior,NULL::text,false,false;
  RETURN;
 END IF;
 INSERT INTO live.media_recovery_episode_scope(episode_id,capture_elapsed_ms,deadline_at,
  capacity,candidate_count,coverage_known,scope_status,timeout_at,include_browser_input)
 VALUES(p_episode,p_elapsed_ms,v_entry+make_interval(secs=>greatest(0,90000-least(p_elapsed_ms,90000))::double precision/1000),
  p_capacity,v_count,p_coverage_known,
  CASE WHEN v_count=0 THEN 'empty' WHEN p_elapsed_ms>=90000 OR v_now>=v_entry+make_interval(secs=>greatest(0,90000-least(p_elapsed_ms,90000))::double precision/1000)
   THEN 'overdue' ELSE 'pending' END,
  CASE WHEN (p_elapsed_ms>=90000 OR v_now>=v_entry+make_interval(secs=>greatest(0,90000-least(p_elapsed_ms,90000))::double precision/1000))
   AND (v_count>0 OR NOT p_coverage_known) THEN v_now ELSE NULL END,true);
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=p_episode;
 FOR v_n IN 1..v_count LOOP
  o:=live.lock_media_operation(v_ids[v_n]);
  SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
  v_mode:=CASE WHEN s.scope_status='overdue' THEN 'overdue' ELSE v_modes[v_n] END;
  UPDATE live.media_execution_state SET recovery_episode_id=p_episode,
   recovery_baseline_generation=v_gens[v_n],recovery_disposition=v_mode,
   recovery_observation_id=NULL,recovery_witness_elapsed_ms=NULL,recovery_witness_at=NULL,
   recovery_timeout_at=s.timeout_at WHERE attempt_id=o.media_attempt_id;
  INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
   reason_code,episode_id,episode_event_kind,native_job_id,recovery_execution_profile,
   recovery_input_required,recovery_egress_required)
  VALUES(o.tenant_id,o.store_id,o.id,v_gens[v_n],o.state,o.lease_mode,
   'media_recovery_admitted',p_episode,'admitted',v_jobs[v_n],v_profiles[v_n],
   v_inputs[v_n],v_egresses[v_n]);
  IF v_modes[v_n] IN ('terminal_at_lock','native_ineligible','ceiling') THEN
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
    reason_code,episode_id,episode_event_kind)
   VALUES(o.tenant_id,o.store_id,o.id,v_gens[v_n],o.state,o.lease_mode,
    'media_recovery_'||v_modes[v_n],p_episode,v_modes[v_n]); END IF;
  IF s.scope_status='overdue' THEN
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
    reason_code,episode_id,episode_event_kind,elapsed_ms)
   VALUES(o.tenant_id,o.store_id,o.id,v_gens[v_n],o.state,o.lease_mode,
    'media_recovery_timeout',p_episode,'timeout',p_elapsed_ms); END IF;
  disposition:=v_mode; episode_id:=p_episode; operation_id:=o.id; job_id:=v_jobs[v_n];
  baseline_generation:=v_gens[v_n]; deadline_at:=s.deadline_at; candidate_count:=v_count;
  coverage_known:=s.coverage_known; blocked_by_episode_id:=NULL;
  execution_profile:=v_profiles[v_n]; input_required:=v_inputs[v_n];
  egress_required:=v_egresses[v_n]; RETURN NEXT;
 END LOOP;
 IF v_count=0 THEN
  disposition:='empty'; episode_id:=p_episode; operation_id:=NULL; job_id:=NULL;
  baseline_generation:=NULL; deadline_at:=s.deadline_at; candidate_count:=0;
  coverage_known:=s.coverage_known; blocked_by_episode_id:=NULL;
  execution_profile:=NULL; input_required:=false; egress_required:=false; RETURN NEXT;
 END IF;
END $$;
ALTER FUNCTION live.begin_media_recovery_episode_with_input(uuid,bigint,integer,boolean) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.begin_media_recovery_episode_with_input(uuid,bigint,integer,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.begin_media_recovery_episode_with_input(uuid,bigint,integer,boolean) TO commerce_media_recovery;

CREATE FUNCTION live.claim_media_recovery_observation_with_input(p_episode uuid,p_operation uuid,
 p_job bigint,p_token bytea)
RETURNS TABLE(disposition text,generation bigint,project_id text,credential_version bigint,
 endpoint_identity text,room_name text,publisher_identity text,egress_id text,
 execution_profile text,input_required boolean,egress_required boolean,
 input_observation_id uuid,egress_observation_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 i live.media_input_custody%ROWTYPE; s live.media_recovery_episode_scope%ROWTYPE;
 e integration.operation_events%ROWTYPE; v_input uuid; v_egress uuid;
 v_input_safe boolean; v_egress_safe boolean; v_now timestamptz;
BEGIN
 IF p_episode IS NULL OR p_operation IS NULL OR p_job IS NULL OR p_job<1
  OR p_token IS NULL OR octet_length(p_token)<>32
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid mixed media recovery claim' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_operation);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id FOR UPDATE;
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=p_episode;
 SELECT * INTO e FROM integration.operation_events WHERE operation_id=o.id
  AND episode_id=p_episode AND episode_event_kind='admitted';
 IF s.episode_id IS NULL OR NOT s.include_browser_input OR s.scope_status<>'pending'
  OR e.id IS NULL OR e.native_job_id<>p_job OR o.job_id<>p_job
  OR e.recovery_execution_profile IS DISTINCT FROM a.execution_profile
  OR NOT (coalesce(e.recovery_input_required,false) OR coalesce(e.recovery_egress_required,false))
  OR x.recovery_episode_id IS DISTINCT FROM p_episode
  OR x.recovery_baseline_generation IS DISTINCT FROM e.generation
  OR x.operation_id<>o.id OR x.project_id<>h.project_id
  OR (a.execution_profile='LOCAL_SFU_MOCK_EGRESS' AND
   (i.attempt_id IS NULL OR i.runtime_version<>1 OR i.operation_id<>o.id
    OR (i.project_id,i.credential_version,i.endpoint_identity,i.room_name)
     IS DISTINCT FROM (h.project_id,h.credential_version,h.endpoint_identity,a.room_name)))
  OR (a.execution_profile='PROVIDER_MOCK' AND e.recovery_input_required) THEN
  RAISE EXCEPTION 'mixed media recovery membership unavailable' USING ERRCODE='ME409'; END IF;
 SELECT r.id INTO v_input FROM live.media_input_recovery_observations r
  WHERE r.episode_id=p_episode AND r.operation_id=o.id;
 SELECT q.observation_id INTO v_egress FROM integration.operation_events q
  WHERE q.episode_id=p_episode AND q.operation_id=o.id AND q.episode_event_kind='qualified';
 generation:=o.generation; project_id:=NULL; credential_version:=NULL;
 endpoint_identity:=NULL; room_name:=NULL; publisher_identity:=NULL; egress_id:=NULL;
 execution_profile:=NULL; input_required:=false; egress_required:=false;
 input_observation_id:=NULL; egress_observation_id:=NULL;
 IF x.recovery_timeout_at IS NOT NULL OR s.timeout_at IS NOT NULL
  OR clock_timestamp()>=s.deadline_at THEN
  disposition:='overdue'; RETURN NEXT; RETURN; END IF;
 IF (NOT e.recovery_input_required OR v_input IS NOT NULL)
  AND (NOT e.recovery_egress_required OR v_egress IS NOT NULL) THEN
  disposition:='already_observed'; RETURN NEXT; RETURN; END IF;
 v_input_safe:=e.recovery_input_required AND v_input IS NULL
  AND i.grant_iat IS NOT NULL AND i.state<>'CLOSED';
 v_egress_safe:=e.recovery_egress_required AND v_egress IS NULL
  AND x.wire_reserved_at IS NOT NULL AND x.resource_state<>'TERMINAL'
  AND x.escalated_at IS NULL;
 IF NOT (v_input_safe OR v_egress_safe) THEN
  disposition:='terminal_at_lock'; RETURN NEXT; RETURN; END IF;
 IF o.generation>=4096 OR clock_timestamp()>=o.created_at+interval '24 hours' THEN
  disposition:='ceiling'; RETURN NEXT; RETURN; END IF;
 IF NOT live.media_recovery_native_eligible(p_job,o.id) THEN
  disposition:='native_ineligible'; RETURN NEXT; RETURN; END IF;
 IF o.lease_until IS NOT NULL AND o.lease_until>clock_timestamp() THEN
  disposition:='busy'; RETURN NEXT; RETURN; END IF;
 v_now:=clock_timestamp();
 UPDATE integration.operations SET state='UNKNOWN',generation=o.generation+1,
  lease_mode='reconcile',lease_until=v_now+interval '30 seconds',
  lease_token_hash=sha256(p_token),updated_at=v_now WHERE id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(o.tenant_id,o.store_id,o.id,o.generation+1,'UNKNOWN','reconcile','media_recovery_claimed');
 IF v_now+interval '30 seconds'<=clock_timestamp() THEN
  RAISE EXCEPTION 'mixed media recovery lease unavailable' USING ERRCODE='ME409'; END IF;
 disposition:='claimed'; generation:=o.generation+1;
 project_id:=h.project_id; credential_version:=h.credential_version;
 endpoint_identity:=h.endpoint_identity; room_name:=a.room_name;
 publisher_identity:=CASE WHEN a.execution_profile='LOCAL_SFU_MOCK_EGRESS'
  THEN i.publisher_identity ELSE NULL END;
 egress_id:=x.egress_id; execution_profile:=a.execution_profile;
 input_required:=e.recovery_input_required; egress_required:=e.recovery_egress_required;
 input_observation_id:=v_input; egress_observation_id:=v_egress;
 RETURN NEXT;
END $$;
ALTER FUNCTION live.claim_media_recovery_observation_with_input(uuid,uuid,bigint,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.claim_media_recovery_observation_with_input(uuid,uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.claim_media_recovery_observation_with_input(uuid,uuid,bigint,bytea) TO commerce_media_recovery;

CREATE FUNCTION live.record_browser_input_recovery_observation(p_episode uuid,p_operation uuid,
 p_generation bigint,p_token bytea,p_source text,p_result text,p_sid text,p_state text)
RETURNS TABLE(disposition text,input_observation_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; i live.media_input_custody%ROWTYPE;
 s live.media_recovery_episode_scope%ROWTYPE; e integration.operation_events%ROWTYPE;
 v_now timestamptz;
BEGIN
 IF p_episode IS NULL OR p_operation IS NULL OR p_generation IS NULL OR p_generation<1
  OR p_token IS NULL OR octet_length(p_token)<>32 OR
  ((p_source='PARTICIPANT' AND p_result='PRESENT'
    AND p_sid ~ '^PA_[A-Za-z0-9_-]{1,100}$'
    AND p_state IN ('JOINING','JOINED','ACTIVE','DISCONNECTED')) OR
   (p_source='ROOM' AND p_result='ABSENT' AND p_sid IS NULL AND p_state IS NULL)) IS NOT TRUE
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid input recovery observation' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_operation);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id FOR UPDATE;
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=p_episode;
 SELECT * INTO e FROM integration.operation_events WHERE operation_id=o.id
  AND episode_id=p_episode AND episode_event_kind='admitted';
 v_now:=clock_timestamp();
 IF s.episode_id IS NULL OR NOT s.include_browser_input OR s.scope_status<>'pending'
  OR s.timeout_at IS NOT NULL OR v_now>s.deadline_at OR e.id IS NULL
  OR e.recovery_execution_profile<>'LOCAL_SFU_MOCK_EGRESS'
  OR e.recovery_input_required IS DISTINCT FROM true OR e.native_job_id<>o.job_id
  OR a.execution_profile<>'LOCAL_SFU_MOCK_EGRESS' OR i.runtime_version<>1
  OR i.operation_id<>o.id OR i.authorization_id<>a.authorization_id
  OR NOT EXISTS(SELECT 1 FROM live.prepared_media_authorizations h WHERE h.id=a.authorization_id
   AND (h.project_id,h.credential_version,h.endpoint_identity)
    IS NOT DISTINCT FROM (i.project_id,i.credential_version,i.endpoint_identity))
  OR i.attempt_id IS NULL OR i.grant_iat IS NULL OR i.state='CLOSED'
  OR x.recovery_episode_id IS DISTINCT FROM p_episode
  OR x.recovery_baseline_generation IS DISTINCT FROM e.generation
  OR x.recovery_timeout_at IS NOT NULL OR p_generation<=e.generation
  OR EXISTS(SELECT 1 FROM integration.operation_events q WHERE q.operation_id=o.id
   AND q.episode_id=p_episode AND q.episode_event_kind='input_qualified')
  OR o.generation<>p_generation OR o.lease_mode<>'reconcile' OR o.state<>'UNKNOWN'
  OR o.lease_until IS NULL OR o.lease_until<=v_now
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN
  RAISE EXCEPTION 'input recovery observation unavailable' USING ERRCODE='ME409'; END IF;
 INSERT INTO live.media_input_recovery_observations(episode_id,tenant_id,store_id,
  operation_id,attempt_id,job_id,generation,project_id,credential_version,
  endpoint_identity,room_name,publisher_identity,source,result,participant_sid,
  participant_state,observed_at)
 VALUES(p_episode,i.tenant_id,i.store_id,o.id,a.id,o.job_id,p_generation,
  i.project_id,i.credential_version,i.endpoint_identity,i.room_name,i.publisher_identity,
  p_source,p_result,p_sid,p_state,v_now) RETURNING id INTO input_observation_id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
  reason_code,episode_id,episode_event_kind,input_observation_id)
 VALUES(o.tenant_id,o.store_id,o.id,p_generation,o.state,o.lease_mode,
  'media_recovery_input_checked',p_episode,'input_qualified',input_observation_id);
 IF NOT e.recovery_egress_required OR EXISTS(SELECT 1 FROM integration.operation_events q
  WHERE q.operation_id=o.id AND q.episode_id=p_episode AND q.episode_event_kind='qualified') THEN
  UPDATE live.media_execution_state SET recovery_disposition='checked'
   WHERE attempt_id=a.id AND recovery_episode_id=p_episode; END IF;
 disposition:='checked'; RETURN NEXT;
END $$;
ALTER FUNCTION live.record_browser_input_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text)
 OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.record_browser_input_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.record_browser_input_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text)
 TO commerce_media_recovery;

-- The historic trigger remains for legacy scopes; its only change is the
-- scope-kind gate. Mixed Egress receipts use the parallel exact-mask trigger.
CREATE OR REPLACE FUNCTION live.qualify_media_recovery_observation() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE x live.media_execution_state%ROWTYPE; a live.media_attempts%ROWTYPE;
 o integration.operations%ROWTYPE; v_started bigint; v_updated bigint; v_ended bigint;
 v_terminal boolean;
BEGIN
 IF NEW.source NOT IN ('ROOM','QUERY') THEN RETURN NULL; END IF;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=NEW.attempt_id;
 IF x.recovery_episode_id IS NULL OR x.recovery_observation_id IS NOT NULL
  OR x.recovery_timeout_at IS NOT NULL OR NEW.generation<=x.recovery_baseline_generation
  OR EXISTS(SELECT 1 FROM live.media_recovery_episode_scope s
   WHERE s.episode_id=x.recovery_episode_id AND s.include_browser_input) THEN
  RETURN NULL; END IF;
 SELECT * INTO a FROM live.media_attempts WHERE id=NEW.attempt_id;
 SELECT * INTO o FROM integration.operations WHERE id=NEW.operation_id;
 IF a.execution_profile<>'PROVIDER_MOCK' OR x.operation_id<>o.id
  OR x.project_id<>NEW.project_id OR o.generation<>NEW.generation
  OR o.lease_mode<>'reconcile' OR o.state<>'UNKNOWN'
  OR x.wire_reserved_at IS NULL OR x.resource_state='TERMINAL'
  OR NEW.room_name<>a.room_name OR (x.egress_id IS NOT NULL AND x.egress_id<>NEW.egress_id)
  OR NOT EXISTS (SELECT 1 FROM integration.operation_events e WHERE e.operation_id=o.id
    AND e.episode_id=x.recovery_episode_id AND e.episode_event_kind='admitted') THEN
  RETURN NULL; END IF;
 v_started:=greatest(x.started_at_ns,NEW.started_at_ns);
 v_updated:=greatest(x.updated_at_ns,NEW.updated_at_ns);
 v_ended:=greatest(x.ended_at_ns,NEW.ended_at_ns);
 v_terminal:=NEW.status IN ('EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED')
  AND v_ended>0 AND NOT ((v_started>0 AND v_updated>0 AND v_updated<v_started)
   OR (v_started>0 AND v_ended>0 AND v_ended<v_started)
   OR (v_updated>0 AND v_ended>0 AND v_updated<v_ended));
 UPDATE live.media_execution_state SET recovery_observation_id=NEW.id,
  recovery_disposition=CASE WHEN v_terminal THEN 'terminal' ELSE 'checked' END
  WHERE attempt_id=NEW.attempt_id AND recovery_episode_id=x.recovery_episode_id
   AND recovery_observation_id IS NULL;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
  reason_code,episode_id,episode_event_kind,observation_id)
 VALUES(o.tenant_id,o.store_id,o.id,NEW.generation,o.state,o.lease_mode,
  CASE WHEN v_terminal THEN 'media_recovery_terminal' ELSE 'media_recovery_checked' END,
  x.recovery_episode_id,'qualified',NEW.id) ON CONFLICT DO NOTHING;
 RETURN NULL;
END $$;
ALTER FUNCTION live.qualify_media_recovery_observation() OWNER TO commerce_media_writer;

CREATE FUNCTION live.qualify_mixed_media_recovery_egress() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE x live.media_execution_state%ROWTYPE; a live.media_attempts%ROWTYPE;
 o integration.operations%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 i live.media_input_custody%ROWTYPE; s live.media_recovery_episode_scope%ROWTYPE;
 e integration.operation_events%ROWTYPE; v_started bigint; v_updated bigint;
 v_ended bigint; v_terminal boolean;
BEGIN
 IF NEW.source NOT IN ('ROOM','QUERY') THEN RETURN NULL; END IF;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=NEW.attempt_id;
 IF x.recovery_episode_id IS NULL THEN RETURN NULL; END IF;
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=x.recovery_episode_id;
 IF NOT s.include_browser_input THEN RETURN NULL; END IF;
 SELECT * INTO a FROM live.media_attempts WHERE id=NEW.attempt_id;
 SELECT * INTO o FROM integration.operations WHERE id=NEW.operation_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id;
 SELECT * INTO e FROM integration.operation_events WHERE operation_id=o.id
  AND episode_id=s.episode_id AND episode_event_kind='admitted';
 IF s.scope_status<>'pending' OR s.timeout_at IS NOT NULL OR NEW.observed_at>s.deadline_at
  OR x.recovery_timeout_at IS NOT NULL OR x.recovery_observation_id IS NOT NULL
  OR e.id IS NULL OR e.recovery_egress_required IS DISTINCT FROM true
  OR e.recovery_execution_profile IS DISTINCT FROM a.execution_profile
  OR e.native_job_id<>o.job_id OR NEW.generation<=e.generation
  OR x.recovery_baseline_generation IS DISTINCT FROM e.generation
  OR x.operation_id<>o.id OR x.project_id<>NEW.project_id
  OR h.project_id<>NEW.project_id OR a.room_name<>NEW.room_name
  OR x.wire_reserved_at IS NULL OR x.resource_state='TERMINAL' OR x.escalated_at IS NOT NULL
  OR o.generation<>NEW.generation OR o.lease_mode<>'reconcile' OR o.state<>'UNKNOWN'
  OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR (x.egress_id IS NOT NULL AND x.egress_id<>NEW.egress_id)
  OR (a.execution_profile='LOCAL_SFU_MOCK_EGRESS' AND
   (i.attempt_id IS NULL OR i.runtime_version IS DISTINCT FROM 1
    OR i.operation_id IS DISTINCT FROM o.id OR i.project_id IS DISTINCT FROM h.project_id
    OR (i.credential_version,i.endpoint_identity,i.room_name)
     IS DISTINCT FROM (h.credential_version,h.endpoint_identity,NEW.room_name))) THEN RETURN NULL; END IF;
 v_started:=greatest(x.started_at_ns,NEW.started_at_ns);
 v_updated:=greatest(x.updated_at_ns,NEW.updated_at_ns);
 v_ended:=greatest(x.ended_at_ns,NEW.ended_at_ns);
 v_terminal:=NEW.status IN ('EGRESS_COMPLETE','EGRESS_FAILED','EGRESS_ABORTED','EGRESS_LIMIT_REACHED')
  AND v_ended>0 AND NOT ((v_started>0 AND v_updated>0 AND v_updated<v_started)
   OR (v_started>0 AND v_ended>0 AND v_ended<v_started)
   OR (v_updated>0 AND v_ended>0 AND v_updated<v_ended));
 UPDATE live.media_execution_state SET recovery_observation_id=NEW.id,
  recovery_disposition=CASE WHEN v_terminal THEN 'terminal' ELSE 'checked' END
  WHERE attempt_id=NEW.attempt_id AND recovery_episode_id=s.episode_id
   AND recovery_observation_id IS NULL;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
  reason_code,episode_id,episode_event_kind,observation_id)
 VALUES(o.tenant_id,o.store_id,o.id,NEW.generation,o.state,o.lease_mode,
  CASE WHEN v_terminal THEN 'media_recovery_terminal' ELSE 'media_recovery_checked' END,
  s.episode_id,'qualified',NEW.id);
 RETURN NULL;
END $$;
ALTER FUNCTION live.qualify_mixed_media_recovery_egress() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.qualify_mixed_media_recovery_egress() FROM PUBLIC;
CREATE TRIGGER media_recovery_mixed_egress AFTER INSERT ON live.media_observations
 FOR EACH ROW EXECUTE FUNCTION live.qualify_mixed_media_recovery_egress();

CREATE FUNCTION live.record_media_recovery_observation_with_input(p_episode uuid,p_operation uuid,
 p_generation bigint,p_token bytea,p_source text,p_egress text,p_room text,p_status text,
 p_started bigint,p_updated bigint,p_ended bigint)
RETURNS TABLE(disposition text,observation_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; i live.media_input_custody%ROWTYPE;
 h live.prepared_media_authorizations%ROWTYPE; s live.media_recovery_episode_scope%ROWTYPE;
 e integration.operation_events%ROWTYPE; v_result text; v_observation live.media_observations%ROWTYPE;
BEGIN
 IF p_episode IS NULL OR p_operation IS NULL OR p_generation IS NULL OR p_generation<1
  OR p_token IS NULL OR octet_length(p_token)<>32
  OR p_source IS NULL OR p_source NOT IN ('ROOM','QUERY')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid mixed media recovery observation' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_operation);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id FOR UPDATE;
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=p_episode;
 SELECT * INTO e FROM integration.operation_events WHERE operation_id=o.id
  AND episode_id=p_episode AND episode_event_kind='admitted';
 IF s.episode_id IS NULL OR NOT s.include_browser_input OR s.scope_status<>'pending'
  OR s.timeout_at IS NOT NULL OR clock_timestamp()>s.deadline_at
  OR e.id IS NULL OR e.recovery_egress_required IS DISTINCT FROM true
  OR e.recovery_execution_profile IS DISTINCT FROM a.execution_profile
  OR e.native_job_id<>o.job_id OR p_generation<=e.generation
  OR x.recovery_episode_id IS DISTINCT FROM p_episode
  OR x.recovery_baseline_generation IS DISTINCT FROM e.generation
  OR x.recovery_observation_id IS NOT NULL OR x.recovery_timeout_at IS NOT NULL
  OR x.wire_reserved_at IS NULL OR x.resource_state='TERMINAL' OR x.escalated_at IS NOT NULL
  OR x.operation_id<>o.id OR x.project_id<>h.project_id
  OR (a.execution_profile='LOCAL_SFU_MOCK_EGRESS' AND
   (i.attempt_id IS NULL OR i.runtime_version IS DISTINCT FROM 1
    OR i.operation_id IS DISTINCT FROM o.id OR
    (i.project_id,i.credential_version,i.endpoint_identity,i.room_name)
     IS DISTINCT FROM (h.project_id,h.credential_version,h.endpoint_identity,a.room_name)))
  OR o.generation<>p_generation OR o.lease_mode<>'reconcile' OR o.state<>'UNKNOWN'
  OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token)
  OR EXISTS(SELECT 1 FROM integration.operation_events q WHERE q.operation_id=o.id
   AND q.episode_id=p_episode AND q.episode_event_kind='qualified') THEN
  RAISE EXCEPTION 'mixed media recovery observation unavailable' USING ERRCODE='ME409'; END IF;
 v_result:=live.project_media_observation(p_operation,p_generation,p_token,p_source,
  p_egress,p_room,p_status,p_started,p_updated,p_ended);
 SELECT * INTO e FROM integration.operation_events q WHERE q.operation_id=o.id
  AND q.episode_id=p_episode AND q.episode_event_kind='qualified';
 SELECT * INTO v_observation FROM live.media_observations v WHERE v.id=e.observation_id;
 IF e.id IS NULL OR v_observation.id IS NULL OR v_observation.observed_at>s.deadline_at
  OR v_observation.operation_id<>o.id OR v_observation.generation<>p_generation
  OR v_observation.source<>p_source OR
  (v_observation.egress_id,v_observation.room_name,v_observation.status,
   v_observation.started_at_ns,v_observation.updated_at_ns,v_observation.ended_at_ns)
   IS DISTINCT FROM (p_egress,p_room,p_status,p_started,p_updated,p_ended) THEN
  RAISE EXCEPTION 'mixed media recovery observation unavailable' USING ERRCODE='ME409'; END IF;
 disposition:=CASE WHEN v_result='terminal' THEN 'terminal' ELSE 'checked' END;
 observation_id:=v_observation.id; RETURN NEXT;
END $$;
ALTER FUNCTION live.record_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.record_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.record_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)
 TO commerce_media_recovery;

CREATE FUNCTION live.finish_media_recovery_observation_with_input(p_episode uuid,p_operation uuid,
 p_generation bigint,p_token bytea,p_code text) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; i live.media_input_custody%ROWTYPE;
 s live.media_recovery_episode_scope%ROWTYPE; e integration.operation_events%ROWTYPE;
 v_now timestamptz;
BEGIN
 IF p_episode IS NULL OR p_operation IS NULL OR p_generation IS NULL OR p_generation<1
  OR p_token IS NULL OR octet_length(p_token)<>32 OR p_code IS NULL OR p_code NOT IN
  ('remote_unknown','not_observed','invalid_observation','credential_unavailable','material_invalid')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid mixed media recovery finish' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_operation);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=p_episode;
 SELECT * INTO e FROM integration.operation_events WHERE operation_id=o.id
  AND episode_id=p_episode AND episode_event_kind='admitted';
 v_now:=clock_timestamp();
 IF s.episode_id IS NULL OR NOT s.include_browser_input OR e.id IS NULL
  OR e.native_job_id<>o.job_id OR x.recovery_episode_id IS DISTINCT FROM p_episode
  OR e.recovery_execution_profile IS DISTINCT FROM a.execution_profile
  OR (e.recovery_input_required AND (i.attempt_id IS NULL OR i.runtime_version<>1
   OR i.operation_id<>o.id))
  OR x.recovery_baseline_generation IS DISTINCT FROM e.generation
  OR o.generation<>p_generation OR o.lease_mode<>'reconcile' OR o.state<>'UNKNOWN'
  OR o.lease_until IS NULL OR o.lease_until<=v_now
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN
  RAISE EXCEPTION 'mixed media recovery finish unavailable' USING ERRCODE='ME409'; END IF;
 UPDATE integration.operations SET state='UNKNOWN',lease_mode='',lease_until=NULL,
  lease_token_hash=NULL,result_code=p_code,updated_at=v_now WHERE id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(o.tenant_id,o.store_id,o.id,p_generation,'UNKNOWN','','media_'||p_code);
 IF o.lease_until<=clock_timestamp() THEN
  RAISE EXCEPTION 'mixed media recovery lease unavailable' USING ERRCODE='ME409'; END IF;
 RETURN 'released';
END $$;
ALTER FUNCTION live.finish_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text)
 OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.finish_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.finish_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text)
 TO commerce_media_recovery;

CREATE FUNCTION live.read_media_recovery_episode_with_input(p_episode uuid)
RETURNS TABLE(episode_id uuid,scope_status text,coverage_known boolean,candidate_count integer,
 blocked_by_episode_id uuid,operation_id uuid,disposition text,baseline_generation bigint,
 observation_id uuid,observation_source text,observation_generation bigint,
 witness_elapsed_ms bigint,timeout_at timestamptz,cleanup_required boolean,
 execution_profile text,input_required boolean,egress_required boolean,
 input_observation_id uuid,input_observation_source text,input_observation_result text,
 input_observation_generation bigint,job_id bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s live.media_recovery_episode_scope%ROWTYPE; v_members integer;
 v_witnesses integer; v_status text;
BEGIN
 IF p_episode IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid mixed media recovery read' USING ERRCODE='ME400'; END IF;
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=p_episode;
 IF s.episode_id IS NULL OR NOT s.include_browser_input THEN
  RAISE EXCEPTION 'mixed media recovery scope unavailable' USING ERRCODE='ME409'; END IF;
 SELECT count(*),count(*) FILTER (WHERE EXISTS(SELECT 1 FROM integration.operation_events w
  WHERE w.operation_id=e.operation_id AND w.episode_id=p_episode AND w.episode_event_kind='witnessed'))
 INTO v_members,v_witnesses FROM integration.operation_events e
 WHERE e.episode_id=p_episode AND e.episode_event_kind='admitted';
 IF s.scope_status IN ('pending','overdue') AND s.candidate_count>0 AND v_members<>s.candidate_count THEN
  RAISE EXCEPTION 'mixed media recovery membership unavailable' USING ERRCODE='ME409'; END IF;
 IF EXISTS(SELECT 1 FROM integration.operation_events e WHERE e.episode_id=p_episode
  AND e.episode_event_kind='admitted' AND
   (e.recovery_execution_profile IS NULL OR e.native_job_id IS NULL
    OR e.recovery_input_required IS NULL OR e.recovery_egress_required IS NULL
    OR NOT (e.recovery_input_required OR e.recovery_egress_required))) THEN
  RAISE EXCEPTION 'mixed media recovery membership unavailable' USING ERRCODE='ME409'; END IF;
 v_status:=CASE WHEN s.scope_status='empty' AND s.coverage_known THEN 'empty'
  WHEN s.scope_status IN ('capacity_exceeded','prior_unfinished') THEN s.scope_status
  WHEN s.timeout_at IS NOT NULL THEN 'overdue'
  WHEN s.scope_status='empty' THEN 'empty'
  WHEN s.coverage_known AND v_members>0 AND v_members=v_witnesses THEN 'finished'
  ELSE 'pending' END;
 RETURN QUERY SELECT s.episode_id,v_status,s.coverage_known,s.candidate_count,
  s.blocked_by_episode_id,e.operation_id,
  CASE WHEN e.operation_id IS NULL THEN NULL::text
   WHEN t.id IS NOT NULL THEN 'timeout'
   WHEN w.id IS NOT NULL THEN 'witnessed'
   WHEN l.id IS NOT NULL THEN 'terminal_at_lock'
   WHEN n.id IS NOT NULL THEN 'native_ineligible'
   WHEN c.id IS NOT NULL THEN 'ceiling'
   WHEN (NOT e.recovery_input_required OR r.id IS NOT NULL)
    AND (NOT e.recovery_egress_required OR v.id IS NOT NULL) THEN
     CASE WHEN q.reason_code='media_recovery_terminal' THEN 'terminal' ELSE 'checked' END
   ELSE 'pending' END,
  e.generation,v.id,v.source,v.generation,w.elapsed_ms,
  CASE WHEN e.operation_id IS NULL THEN s.timeout_at ELSE t.created_at END,
  x.cleanup_required,e.recovery_execution_profile,
  coalesce(e.recovery_input_required,false),coalesce(e.recovery_egress_required,false),
  r.id,r.source,r.result,r.generation,e.native_job_id
 FROM live.media_recovery_episode_scope z
 LEFT JOIN integration.operation_events e ON e.episode_id=z.episode_id
  AND e.episode_event_kind='admitted'
 LEFT JOIN live.media_execution_state x ON x.operation_id=e.operation_id
 LEFT JOIN integration.operation_events t ON t.operation_id=e.operation_id
  AND t.episode_id=z.episode_id AND t.episode_event_kind='timeout'
 LEFT JOIN integration.operation_events w ON w.operation_id=e.operation_id
  AND w.episode_id=z.episode_id AND w.episode_event_kind='witnessed'
 LEFT JOIN integration.operation_events l ON l.operation_id=e.operation_id
  AND l.episode_id=z.episode_id AND l.episode_event_kind='terminal_at_lock'
 LEFT JOIN integration.operation_events n ON n.operation_id=e.operation_id
  AND n.episode_id=z.episode_id AND n.episode_event_kind='native_ineligible'
 LEFT JOIN integration.operation_events c ON c.operation_id=e.operation_id
  AND c.episode_id=z.episode_id AND c.episode_event_kind='ceiling'
 LEFT JOIN integration.operation_events q ON q.operation_id=e.operation_id
  AND q.episode_id=z.episode_id AND q.episode_event_kind='qualified'
 LEFT JOIN live.media_observations v ON v.id=q.observation_id AND v.operation_id=e.operation_id
 LEFT JOIN integration.operation_events iq ON iq.operation_id=e.operation_id
  AND iq.episode_id=z.episode_id AND iq.episode_event_kind='input_qualified'
 LEFT JOIN live.media_input_recovery_observations r ON r.id=iq.input_observation_id
  AND r.operation_id=e.operation_id AND r.episode_id=z.episode_id AND r.job_id=e.native_job_id
 WHERE z.episode_id=p_episode ORDER BY e.operation_id;
END $$;
ALTER FUNCTION live.read_media_recovery_episode_with_input(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.read_media_recovery_episode_with_input(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.read_media_recovery_episode_with_input(uuid) TO commerce_media_recovery;

CREATE FUNCTION live.witness_media_recovery_episode_with_input(p_episode uuid,p_operation uuid,
 p_input uuid,p_egress uuid,p_elapsed bigint) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 i live.media_input_custody%ROWTYPE; s live.media_recovery_episode_scope%ROWTYPE;
 e integration.operation_events%ROWTYPE; iq integration.operation_events%ROWTYPE;
 q integration.operation_events%ROWTYPE; w integration.operation_events%ROWTYPE;
 r live.media_input_recovery_observations%ROWTYPE; v live.media_observations%ROWTYPE;
 v_now timestamptz;
BEGIN
 IF p_episode IS NULL OR p_operation IS NULL OR p_elapsed IS NULL OR p_elapsed NOT BETWEEN 0 AND 90000
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid mixed media recovery witness' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_operation);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=p_episode;
 IF s.episode_id IS NULL OR NOT s.include_browser_input THEN
  RAISE EXCEPTION 'mixed media recovery scope unavailable' USING ERRCODE='ME409'; END IF;
 SELECT * INTO e FROM integration.operation_events WHERE operation_id=o.id
  AND episode_id=p_episode AND episode_event_kind='admitted';
 SELECT * INTO w FROM integration.operation_events WHERE operation_id=o.id
  AND episode_id=p_episode AND episode_event_kind='witnessed';
 IF w.id IS NOT NULL THEN
  IF w.input_observation_id IS NOT DISTINCT FROM p_input
   AND w.observation_id IS NOT DISTINCT FROM p_egress AND w.elapsed_ms=p_elapsed THEN
   RETURN 'already_witnessed'; END IF;
  RETURN 'unqualified'; END IF;
 IF s.timeout_at IS NOT NULL OR EXISTS(SELECT 1 FROM integration.operation_events t
  WHERE t.operation_id=o.id AND t.episode_id=p_episode AND t.episode_event_kind='timeout') THEN
  RETURN 'timeout_wins'; END IF;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 SELECT * INTO iq FROM integration.operation_events WHERE operation_id=o.id
  AND episode_id=p_episode AND episode_event_kind='input_qualified';
 SELECT * INTO q FROM integration.operation_events WHERE operation_id=o.id
  AND episode_id=p_episode AND episode_event_kind='qualified';
 SELECT * INTO r FROM live.media_input_recovery_observations WHERE id=iq.input_observation_id;
 SELECT * INTO v FROM live.media_observations WHERE id=q.observation_id;
 IF e.id IS NULL OR e.native_job_id<>o.job_id OR e.recovery_execution_profile IS DISTINCT FROM a.execution_profile
  OR NOT (e.recovery_input_required OR e.recovery_egress_required)
  OR (e.recovery_input_required AND p_input IS NULL)
  OR (NOT e.recovery_input_required AND p_input IS NOT NULL)
  OR (e.recovery_egress_required AND p_egress IS NULL)
  OR (NOT e.recovery_egress_required AND p_egress IS NOT NULL)
  OR x.recovery_episode_id IS DISTINCT FROM p_episode
  OR x.recovery_baseline_generation IS DISTINCT FROM e.generation
  OR x.recovery_timeout_at IS NOT NULL OR x.operation_id<>o.id OR x.project_id<>h.project_id
  OR (e.recovery_input_required AND
   (iq.id IS NULL OR iq.input_observation_id IS DISTINCT FROM p_input
    OR r.id IS NULL OR r.episode_id<>p_episode OR r.operation_id<>o.id
    OR r.attempt_id<>a.id OR r.job_id<>e.native_job_id OR r.generation<=e.generation
    OR r.observed_at>s.deadline_at OR i.attempt_id IS NULL OR i.runtime_version<>1
    OR i.operation_id<>o.id
    OR (r.project_id,r.credential_version,r.endpoint_identity,r.room_name,r.publisher_identity)
     IS DISTINCT FROM (i.project_id,i.credential_version,i.endpoint_identity,i.room_name,i.publisher_identity)))
  OR (e.recovery_egress_required AND
   (q.id IS NULL OR q.observation_id IS DISTINCT FROM p_egress
    OR v.id IS NULL OR v.operation_id<>o.id OR v.attempt_id<>a.id
    OR v.generation<=e.generation OR v.observed_at>s.deadline_at
    OR v.source NOT IN ('ROOM','QUERY') OR v.project_id<>h.project_id
    OR v.room_name<>a.room_name OR (a.execution_profile='LOCAL_SFU_MOCK_EGRESS'
     AND (i.attempt_id IS NULL OR i.room_name<>v.room_name
      OR i.project_id<>v.project_id)))) THEN RETURN 'unqualified'; END IF;
 v_now:=clock_timestamp();
 UPDATE live.media_execution_state SET recovery_disposition='witnessed',
  recovery_witness_elapsed_ms=p_elapsed,recovery_witness_at=v_now
  WHERE attempt_id=a.id AND recovery_episode_id=p_episode;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
  reason_code,episode_id,episode_event_kind,observation_id,input_observation_id,elapsed_ms)
 VALUES(o.tenant_id,o.store_id,o.id,greatest(coalesce(q.generation,0),coalesce(iq.generation,0),e.generation),o.state,o.lease_mode,
  'media_recovery_witnessed',p_episode,'witnessed',p_egress,p_input,p_elapsed);
 RETURN 'witnessed';
END $$;
ALTER FUNCTION live.witness_media_recovery_episode_with_input(uuid,uuid,uuid,uuid,bigint)
 OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.witness_media_recovery_episode_with_input(uuid,uuid,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.witness_media_recovery_episode_with_input(uuid,uuid,uuid,uuid,bigint)
 TO commerce_media_recovery;
