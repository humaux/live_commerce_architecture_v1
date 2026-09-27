-- Additive, opt-in observation of the original PROVIDER_MOCK media operation.
CREATE ROLE commerce_media_recovery NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT USAGE ON SCHEMA live TO commerce_media_recovery;

CREATE TABLE live.media_recovery_episode_scope (
 episode_id uuid PRIMARY KEY,
 capture_elapsed_ms bigint NOT NULL CHECK (capture_elapsed_ms>=0),
 deadline_at timestamptz NOT NULL,
 capacity integer NOT NULL CHECK (capacity BETWEEN 1 AND 32),
 candidate_count integer NOT NULL CHECK (candidate_count BETWEEN 0 AND 33),
 coverage_known boolean NOT NULL,
 scope_status text NOT NULL CHECK (scope_status IN
  ('pending','empty','capacity_exceeded','prior_unfinished','overdue')),
 capacity_exceeded_at timestamptz,
 blocked_by_episode_id uuid,
 timeout_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK ((scope_status='capacity_exceeded')=(capacity_exceeded_at IS NOT NULL)),
 CHECK ((scope_status='prior_unfinished')=(blocked_by_episode_id IS NOT NULL))
);
ALTER TABLE live.media_recovery_episode_scope ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.media_recovery_episode_scope FORCE ROW LEVEL SECURITY;
CREATE POLICY media_recovery_scope_writer ON live.media_recovery_episode_scope
 FOR ALL TO commerce_media_writer USING (true) WITH CHECK (true);
REVOKE ALL ON live.media_recovery_episode_scope FROM PUBLIC;
GRANT SELECT,INSERT,UPDATE(scope_status,timeout_at) ON live.media_recovery_episode_scope TO commerce_media_writer;

ALTER TABLE live.media_execution_state
 ADD COLUMN recovery_episode_id uuid,
 ADD COLUMN recovery_baseline_generation bigint CHECK (recovery_baseline_generation>=0),
 ADD COLUMN recovery_disposition text CHECK (recovery_disposition IN
  ('pending','checked','terminal','terminal_at_lock','native_ineligible','ceiling','overdue','timeout','witnessed')),
 ADD COLUMN recovery_observation_id uuid,
 ADD COLUMN recovery_witness_elapsed_ms bigint CHECK (recovery_witness_elapsed_ms BETWEEN 0 AND 90000),
 ADD COLUMN recovery_witness_at timestamptz,
 ADD COLUMN recovery_timeout_at timestamptz,
 ADD CONSTRAINT media_recovery_active_pair CHECK
  ((recovery_episode_id IS NULL)=(recovery_baseline_generation IS NULL));
GRANT UPDATE(recovery_episode_id,recovery_baseline_generation,recovery_disposition,
 recovery_observation_id,recovery_witness_elapsed_ms,recovery_witness_at,recovery_timeout_at)
 ON live.media_execution_state TO commerce_media_writer;

ALTER TABLE integration.operation_events
 ADD COLUMN episode_id uuid,
 ADD COLUMN episode_event_kind text CHECK (episode_event_kind IN
  ('admitted','terminal_at_lock','native_ineligible','ceiling','qualified','witnessed','timeout')),
 ADD COLUMN native_job_id bigint CHECK (native_job_id>0),
 ADD COLUMN observation_id uuid,
 ADD COLUMN elapsed_ms bigint CHECK (elapsed_ms>=0),
 ADD CONSTRAINT media_recovery_event_pair CHECK ((episode_id IS NULL)=(episode_event_kind IS NULL));
CREATE UNIQUE INDEX media_recovery_one_shot_event ON integration.operation_events
 (operation_id,episode_id,episode_event_kind) WHERE episode_id IS NOT NULL;

-- The SECURITY DEFINER functions below run as commerce_media_writer. They
-- must see their own admitted/qualified/witnessed events under FORCE RLS.
CREATE POLICY media_writer_event_read ON integration.operation_events
 FOR SELECT TO commerce_media_writer USING
 (EXISTS (SELECT 1 FROM integration.operations o
  WHERE o.id=operation_events.operation_id
   AND o.tenant_id=operation_events.tenant_id
   AND o.store_id=operation_events.store_id
   AND o.actor_kind='MEDIA_ATTEMPT'));

-- Fires inside the existing projector transaction, including its public QUERY
-- cleanup guard. A later wrapper failure rolls this correlation back too.
CREATE FUNCTION live.qualify_media_recovery_observation() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE x live.media_execution_state%ROWTYPE; a live.media_attempts%ROWTYPE;
 o integration.operations%ROWTYPE; v_started bigint; v_updated bigint; v_ended bigint;
 v_terminal boolean;
BEGIN
 IF NEW.source NOT IN ('ROOM','QUERY') THEN RETURN NULL; END IF;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=NEW.attempt_id;
 IF x.recovery_episode_id IS NULL OR x.recovery_observation_id IS NOT NULL
  OR x.recovery_timeout_at IS NOT NULL OR NEW.generation<=x.recovery_baseline_generation THEN
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
  x.recovery_episode_id,'qualified',NEW.id)
 ON CONFLICT DO NOTHING;
 RETURN NULL;
END $$;
ALTER FUNCTION live.qualify_media_recovery_observation() OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.qualify_media_recovery_observation() FROM PUBLIC;
CREATE TRIGGER media_recovery_observation AFTER INSERT ON live.media_observations
 FOR EACH ROW EXECUTE FUNCTION live.qualify_media_recovery_observation();

-- Identity alone is insufficient: a finalized or exhausted native row cannot
-- authorize another observation lease. No River row is locked or mutated.
CREATE FUNCTION live.media_recovery_native_eligible(p_job bigint,p_operation uuid) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_ok boolean;
BEGIN
 SELECT live.media_native_job(p_job,p_operation) AND j.finalized_at IS NULL
  AND j.state::text IN ('available','pending','scheduled','retryable','running')
  AND j.attempt<j.max_attempts INTO v_ok
 FROM river_media.river_job j WHERE j.id=p_job;
 RETURN coalesce(v_ok,false);
END $$;
ALTER FUNCTION live.media_recovery_native_eligible(bigint,uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_recovery_native_eligible(bigint,uuid) FROM PUBLIC;

-- History, not the mutable active projection, owns same-ID replay.
CREATE FUNCTION live.media_recovery_begin_replay(p_episode uuid)
RETURNS TABLE(disposition text,episode_id uuid,operation_id uuid,job_id bigint,
 baseline_generation bigint,deadline_at timestamptz,candidate_count integer,
 coverage_known boolean,blocked_by_episode_id uuid)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s live.media_recovery_episode_scope%ROWTYPE; v_members integer;
BEGIN
 SELECT * INTO s FROM live.media_recovery_episode_scope z WHERE z.episode_id=p_episode;
 IF s.episode_id IS NOT NULL AND s.scope_status IN ('pending','overdue')
  AND s.candidate_count>0 THEN
  SELECT count(*) INTO v_members FROM integration.operation_events e
   WHERE e.episode_id=p_episode AND e.episode_event_kind='admitted';
  IF v_members<>s.candidate_count THEN
   RAISE EXCEPTION 'media recovery membership unavailable' USING ERRCODE='ME409'; END IF;
 END IF;
 RETURN QUERY SELECT CASE WHEN e.operation_id IS NULL THEN z.scope_status
  WHEN z.scope_status='overdue' THEN 'overdue'
  WHEN EXISTS(SELECT 1 FROM integration.operation_events q WHERE q.operation_id=e.operation_id
   AND q.episode_id=p_episode AND q.episode_event_kind='terminal_at_lock') THEN 'terminal_at_lock'
  WHEN EXISTS(SELECT 1 FROM integration.operation_events q WHERE q.operation_id=e.operation_id
   AND q.episode_id=p_episode AND q.episode_event_kind='native_ineligible') THEN 'native_ineligible'
  WHEN EXISTS(SELECT 1 FROM integration.operation_events q WHERE q.operation_id=e.operation_id
   AND q.episode_id=p_episode AND q.episode_event_kind='ceiling') THEN 'ceiling'
  ELSE 'pending' END,z.episode_id,e.operation_id,e.native_job_id,e.generation,
  z.deadline_at,z.candidate_count,z.coverage_known,z.blocked_by_episode_id
 FROM live.media_recovery_episode_scope z LEFT JOIN integration.operation_events e
  ON e.episode_id=z.episode_id AND e.episode_event_kind='admitted'
 WHERE z.episode_id=p_episode ORDER BY e.operation_id;
END
$$;
ALTER FUNCTION live.media_recovery_begin_replay(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.media_recovery_begin_replay(uuid) FROM PUBLIC;

CREATE FUNCTION live.begin_media_recovery_episode(p_episode uuid,p_elapsed_ms bigint,
 p_capacity integer,p_coverage_known boolean)
RETURNS TABLE(disposition text,episode_id uuid,operation_id uuid,job_id bigint,
 baseline_generation bigint,deadline_at timestamptz,candidate_count integer,
 coverage_known boolean,blocked_by_episode_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s live.media_recovery_episode_scope%ROWTYPE; o integration.operations%ROWTYPE;
 x live.media_execution_state%ROWTYPE; a live.media_attempts%ROWTYPE;
 v_ids uuid[]; v_jobs bigint[]:='{}'; v_gens bigint[]:='{}'; v_modes text[]:='{}';
 v_count integer; v_index integer; v_prior uuid; v_now timestamptz; v_mode text;
BEGIN
 IF p_episode IS NULL OR p_elapsed_ms IS NULL OR p_elapsed_ms<0 OR p_capacity IS NULL
  OR p_capacity NOT BETWEEN 1 AND 32 OR p_coverage_known IS NULL
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media recovery begin' USING ERRCODE='ME400'; END IF;
 SELECT * INTO s FROM live.media_recovery_episode_scope z WHERE z.episode_id=p_episode;
 IF s.episode_id IS NOT NULL THEN
  RETURN QUERY SELECT * FROM live.media_recovery_begin_replay(p_episode); RETURN;
 END IF;
 SELECT array_agg(id ORDER BY id) INTO v_ids FROM (
  SELECT co.id FROM integration.operations co
  JOIN live.media_execution_state mx ON mx.operation_id=co.id
  JOIN live.media_attempts ca ON ca.id=co.media_attempt_id
  WHERE ca.execution_profile='PROVIDER_MOCK' AND mx.wire_reserved_at IS NOT NULL
   AND mx.resource_state<>'TERMINAL'
   AND co.state IN ('DISPATCHING','UNKNOWN')
  ORDER BY co.id LIMIT p_capacity+1
 ) candidates;
 v_count:=coalesce(array_length(v_ids,1),0);
 v_now:=clock_timestamp();
 IF v_count>p_capacity THEN
  INSERT INTO live.media_recovery_episode_scope(episode_id,capture_elapsed_ms,deadline_at,
   capacity,candidate_count,coverage_known,scope_status,capacity_exceeded_at)
  VALUES(p_episode,p_elapsed_ms,v_now+make_interval(secs=>greatest(0,90000-least(p_elapsed_ms,90000))::double precision/1000),
   p_capacity,v_count,p_coverage_known,'capacity_exceeded',v_now) ON CONFLICT DO NOTHING;
  IF NOT FOUND THEN
   RETURN QUERY SELECT * FROM live.media_recovery_begin_replay(p_episode); RETURN;
  END IF;
  SELECT * INTO s FROM live.media_recovery_episode_scope z WHERE z.episode_id=p_episode;
  RETURN QUERY SELECT s.scope_status,s.episode_id,NULL::uuid,NULL::bigint,NULL::bigint,
   s.deadline_at,s.candidate_count,s.coverage_known,s.blocked_by_episode_id;
  RETURN;
 END IF;
 -- All business locks precede the scope write. Revalidate after each wait.
 FOR v_index IN 1..v_count LOOP
  o:=live.lock_media_operation(v_ids[v_index]);
  SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
  SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
  IF x.recovery_episode_id IS NOT NULL AND x.recovery_episode_id<>p_episode
   AND EXISTS(SELECT 1 FROM live.media_recovery_episode_scope old_scope
    WHERE old_scope.episode_id=x.recovery_episode_id AND old_scope.timeout_at IS NULL
     AND (NOT old_scope.coverage_known
      OR (old_scope.candidate_count>0 AND
       (SELECT count(*) FROM integration.operation_events old_member
        WHERE old_member.episode_id=old_scope.episode_id
         AND old_member.episode_event_kind='admitted')<>old_scope.candidate_count)
      OR EXISTS
      (SELECT 1 FROM integration.operation_events prior_member
       WHERE prior_member.episode_id=old_scope.episode_id
        AND prior_member.episode_event_kind='admitted'
        AND NOT EXISTS(SELECT 1 FROM integration.operation_events prior_witness
         WHERE prior_witness.episode_id=old_scope.episode_id
          AND prior_witness.operation_id=prior_member.operation_id
          AND prior_witness.episode_event_kind='witnessed')))) THEN
   v_prior:=x.recovery_episode_id; EXIT; END IF;
  IF a.execution_profile<>'PROVIDER_MOCK' OR x.wire_reserved_at IS NULL THEN
   RAISE EXCEPTION 'media recovery membership changed' USING ERRCODE='ME409'; END IF;
  v_mode:=CASE
   WHEN x.resource_state='TERMINAL' OR o.state IN
    ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING') THEN 'terminal_at_lock'
   WHEN NOT live.media_recovery_native_eligible(o.job_id,o.id) THEN 'native_ineligible'
   WHEN x.escalated_at IS NOT NULL OR o.generation>=4096
    OR clock_timestamp()>=o.created_at+interval '24 hours' THEN 'ceiling'
   ELSE 'pending' END;
  v_jobs:=array_append(v_jobs,o.job_id);
  v_gens:=array_append(v_gens,o.generation);
  v_modes:=array_append(v_modes,v_mode);
 END LOOP;
 v_now:=clock_timestamp();
 IF v_prior IS NOT NULL THEN
  INSERT INTO live.media_recovery_episode_scope(episode_id,capture_elapsed_ms,deadline_at,
   capacity,candidate_count,coverage_known,scope_status,blocked_by_episode_id)
  VALUES(p_episode,p_elapsed_ms,v_now+make_interval(secs=>greatest(0,90000-least(p_elapsed_ms,90000))::double precision/1000),
   p_capacity,v_count,p_coverage_known,'prior_unfinished',v_prior) ON CONFLICT DO NOTHING;
  IF NOT FOUND THEN
   RETURN QUERY SELECT * FROM live.media_recovery_begin_replay(p_episode); RETURN;
  END IF;
  SELECT * INTO s FROM live.media_recovery_episode_scope z WHERE z.episode_id=p_episode;
  RETURN QUERY SELECT s.scope_status,s.episode_id,NULL::uuid,NULL::bigint,NULL::bigint,
   s.deadline_at,s.candidate_count,s.coverage_known,s.blocked_by_episode_id;
  RETURN;
 END IF;
 INSERT INTO live.media_recovery_episode_scope(episode_id,capture_elapsed_ms,deadline_at,
  capacity,candidate_count,coverage_known,scope_status,timeout_at)
 VALUES(p_episode,p_elapsed_ms,v_now+make_interval(secs=>greatest(0,90000-least(p_elapsed_ms,90000))::double precision/1000),
  p_capacity,v_count,p_coverage_known,
  CASE WHEN v_count=0 THEN 'empty' WHEN p_elapsed_ms>=90000 THEN 'overdue' ELSE 'pending' END,
  CASE WHEN p_elapsed_ms>=90000 AND (v_count>0 OR NOT p_coverage_known) THEN v_now ELSE NULL END)
 ON CONFLICT DO NOTHING;
 IF NOT FOUND THEN
  -- Concurrent same-ID begin won. No active projection was touched by this call.
  RETURN QUERY SELECT * FROM live.media_recovery_begin_replay(p_episode); RETURN;
 END IF;
 SELECT * INTO s FROM live.media_recovery_episode_scope z WHERE z.episode_id=p_episode;
 FOR v_index IN 1..v_count LOOP
  o:=live.lock_media_operation(v_ids[v_index]);
  SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
  v_mode:=CASE WHEN p_elapsed_ms>=90000 THEN 'overdue' ELSE v_modes[v_index] END;
  UPDATE live.media_execution_state SET recovery_episode_id=p_episode,
   recovery_baseline_generation=v_gens[v_index],recovery_disposition=v_mode,
   recovery_observation_id=NULL,recovery_witness_elapsed_ms=NULL,recovery_witness_at=NULL,
   recovery_timeout_at=CASE WHEN p_elapsed_ms>=90000 THEN v_now ELSE NULL END
   WHERE attempt_id=o.media_attempt_id;
  INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
   reason_code,episode_id,episode_event_kind,native_job_id)
  VALUES(o.tenant_id,o.store_id,o.id,v_gens[v_index],o.state,o.lease_mode,
   'media_recovery_admitted',p_episode,'admitted',v_jobs[v_index]);
  IF v_modes[v_index] IN ('terminal_at_lock','native_ineligible','ceiling') THEN
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
    reason_code,episode_id,episode_event_kind)
   VALUES(o.tenant_id,o.store_id,o.id,v_gens[v_index],o.state,o.lease_mode,
    'media_recovery_'||v_modes[v_index],p_episode,v_modes[v_index]);
  END IF;
  IF p_elapsed_ms>=90000 THEN
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
    reason_code,episode_id,episode_event_kind,elapsed_ms)
   VALUES(o.tenant_id,o.store_id,o.id,v_gens[v_index],o.state,o.lease_mode,
    'media_recovery_timeout',p_episode,'timeout',p_elapsed_ms);
  END IF;
  disposition:=v_mode; episode_id:=p_episode; operation_id:=o.id;
  job_id:=v_jobs[v_index]; baseline_generation:=v_gens[v_index];
  deadline_at:=s.deadline_at; candidate_count:=v_count;
  coverage_known:=s.coverage_known; blocked_by_episode_id:=NULL;
  RETURN NEXT;
 END LOOP;
 IF v_count=0 THEN
  disposition:='empty'; episode_id:=p_episode; operation_id:=NULL; job_id:=NULL;
  baseline_generation:=NULL; deadline_at:=s.deadline_at; candidate_count:=0;
  coverage_known:=s.coverage_known; blocked_by_episode_id:=NULL; RETURN NEXT;
 END IF;
END $$;
ALTER FUNCTION live.begin_media_recovery_episode(uuid,bigint,integer,boolean) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.begin_media_recovery_episode(uuid,bigint,integer,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.begin_media_recovery_episode(uuid,bigint,integer,boolean) TO commerce_media_recovery;

CREATE FUNCTION live.claim_recovery_observation(p_episode uuid,p_operation uuid,
 p_job bigint,p_token bytea)
RETURNS TABLE(disposition text,generation bigint,project_id text,credential_version bigint,
 endpoint_identity text,room_name text,egress_id text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; h live.prepared_media_authorizations%ROWTYPE;
 s live.media_recovery_episode_scope%ROWTYPE; v_now timestamptz;
BEGIN
 IF p_episode IS NULL OR p_operation IS NULL OR p_job IS NULL OR p_job<1
  OR p_token IS NULL OR octet_length(p_token)<>32
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media recovery claim' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_operation);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 SELECT * INTO h FROM live.prepared_media_authorizations WHERE id=a.authorization_id;
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=p_episode;
 IF x.recovery_episode_id IS DISTINCT FROM p_episode OR s.episode_id IS NULL
  OR s.scope_status<>'pending' OR a.execution_profile<>'PROVIDER_MOCK'
  OR o.job_id<>p_job OR x.wire_reserved_at IS NULL
  OR NOT EXISTS (SELECT 1 FROM integration.operation_events e WHERE e.operation_id=o.id
   AND e.episode_id=p_episode AND e.episode_event_kind='admitted'
   AND e.native_job_id=p_job) THEN
  RAISE EXCEPTION 'media recovery membership unavailable' USING ERRCODE='ME409'; END IF;
 generation:=o.generation; project_id:=NULL; credential_version:=NULL;
 endpoint_identity:=NULL; room_name:=NULL; egress_id:=NULL;
 IF x.recovery_timeout_at IS NOT NULL OR s.timeout_at IS NOT NULL
  OR clock_timestamp()>=s.deadline_at THEN
  disposition:='overdue'; RETURN NEXT; RETURN; END IF;
 IF x.recovery_observation_id IS NOT NULL THEN
  disposition:='already_observed'; RETURN NEXT; RETURN; END IF;
 IF x.resource_state='TERMINAL' OR o.state IN
  ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING') THEN
  disposition:='terminal_at_lock'; RETURN NEXT; RETURN; END IF;
 IF x.escalated_at IS NOT NULL OR o.generation>=4096
  OR clock_timestamp()>=o.created_at+interval '24 hours' THEN
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
  RAISE EXCEPTION 'media recovery lease unavailable' USING ERRCODE='ME409'; END IF;
 disposition:='claimed'; generation:=o.generation+1;
 project_id:=h.project_id; credential_version:=h.credential_version;
 endpoint_identity:=h.endpoint_identity; room_name:=a.room_name; egress_id:=x.egress_id;
 RETURN NEXT;
END $$;
ALTER FUNCTION live.claim_recovery_observation(uuid,uuid,bigint,bytea) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.claim_recovery_observation(uuid,uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.claim_recovery_observation(uuid,uuid,bigint,bytea) TO commerce_media_recovery;

CREATE FUNCTION live.record_recovery_observation(p_episode uuid,p_operation uuid,
 p_generation bigint,p_token bytea,p_source text,p_egress text,p_room text,p_status text,
 p_started bigint,p_updated bigint,p_ended bigint)
RETURNS TABLE(disposition text,observation_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 a live.media_attempts%ROWTYPE; v_result text;
BEGIN
 IF p_episode IS NULL OR p_operation IS NULL OR p_source NOT IN ('ROOM','QUERY')
  OR p_source IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media recovery observation' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_operation);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO a FROM live.media_attempts WHERE id=o.media_attempt_id;
 IF x.recovery_episode_id IS DISTINCT FROM p_episode OR a.execution_profile<>'PROVIDER_MOCK'
  OR x.recovery_observation_id IS NOT NULL OR x.recovery_timeout_at IS NOT NULL
  OR x.wire_reserved_at IS NULL OR o.generation<>p_generation OR o.lease_mode<>'reconcile'
  OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN
  RAISE EXCEPTION 'media recovery observation unavailable' USING ERRCODE='ME409'; END IF;
 v_result:=live.project_media_observation(p_operation,p_generation,p_token,p_source,
  p_egress,p_room,p_status,p_started,p_updated,p_ended);
 SELECT x2.recovery_observation_id INTO observation_id FROM live.media_execution_state x2
  WHERE x2.attempt_id=o.media_attempt_id AND x2.recovery_episode_id=p_episode;
 IF observation_id IS NULL OR NOT EXISTS (SELECT 1 FROM live.media_observations v
  WHERE v.id=observation_id AND v.operation_id=o.id AND v.generation=p_generation
   AND v.source=p_source AND (v.egress_id,v.room_name,v.status,v.started_at_ns,
    v.updated_at_ns,v.ended_at_ns) IS NOT DISTINCT FROM
    (p_egress,p_room,p_status,p_started,p_updated,p_ended)) THEN
  RAISE EXCEPTION 'media recovery observation unavailable' USING ERRCODE='ME409'; END IF;
 disposition:=CASE WHEN v_result='terminal' THEN 'terminal' ELSE 'checked' END;
 RETURN NEXT;
END $$;
ALTER FUNCTION live.record_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text,
 bigint,bigint,bigint) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.record_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,
 text,bigint,bigint,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.record_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,
 text,bigint,bigint,bigint) TO commerce_media_recovery;

CREATE FUNCTION live.finish_recovery_observation(p_episode uuid,p_operation uuid,
 p_generation bigint,p_token bytea,p_code text) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
BEGIN
 IF p_episode IS NULL OR p_operation IS NULL OR p_code IS NULL OR p_code NOT IN
  ('remote_unknown','not_observed','invalid_observation','credential_unavailable','material_invalid')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media recovery finish' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_operation);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 IF x.recovery_episode_id IS DISTINCT FROM p_episode OR x.recovery_observation_id IS NOT NULL
  OR o.generation<>p_generation OR o.lease_mode<>'reconcile' OR o.state<>'UNKNOWN'
  OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN
  RAISE EXCEPTION 'media recovery finish unavailable' USING ERRCODE='ME409'; END IF;
 PERFORM live.finish_media_uncertain(p_operation,p_generation,p_token,p_code);
 RETURN 'released';
END $$;
ALTER FUNCTION live.finish_recovery_observation(uuid,uuid,bigint,bytea,text) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.finish_recovery_observation(uuid,uuid,bigint,bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.finish_recovery_observation(uuid,uuid,bigint,bytea,text) TO commerce_media_recovery;

CREATE FUNCTION live.read_media_recovery_episode(p_episode uuid)
RETURNS TABLE(episode_id uuid,scope_status text,coverage_known boolean,candidate_count integer,
 blocked_by_episode_id uuid,operation_id uuid,disposition text,baseline_generation bigint,
 observation_id uuid,observation_source text,observation_generation bigint,
 witness_elapsed_ms bigint,timeout_at timestamptz,cleanup_required boolean)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s live.media_recovery_episode_scope%ROWTYPE; v_members integer; v_witnesses integer;
 v_status text;
BEGIN
 IF p_episode IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media recovery read' USING ERRCODE='ME400'; END IF;
 SELECT * INTO s FROM live.media_recovery_episode_scope z WHERE z.episode_id=p_episode;
 IF s.episode_id IS NULL THEN
  RAISE EXCEPTION 'media recovery episode unavailable' USING ERRCODE='ME409'; END IF;
 SELECT count(*),count(*) FILTER (WHERE EXISTS(SELECT 1 FROM integration.operation_events w
  WHERE w.operation_id=e.operation_id AND w.episode_id=p_episode AND w.episode_event_kind='witnessed'))
 INTO v_members,v_witnesses FROM integration.operation_events e
 WHERE e.episode_id=p_episode AND e.episode_event_kind='admitted';
 IF s.scope_status IN ('pending','overdue') AND s.candidate_count>0
  AND v_members<>s.candidate_count THEN
  RAISE EXCEPTION 'media recovery membership unavailable' USING ERRCODE='ME409'; END IF;
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
   WHEN q.id IS NOT NULL THEN CASE WHEN q.reason_code='media_recovery_terminal'
    THEN 'terminal' ELSE 'checked' END
   ELSE 'pending' END,
  e.generation,q.observation_id,v.source,v.generation,w.elapsed_ms,
  CASE WHEN e.operation_id IS NULL THEN s.timeout_at ELSE t.created_at END,x.cleanup_required
 FROM live.media_recovery_episode_scope z
 LEFT JOIN integration.operation_events e ON e.episode_id=z.episode_id AND e.episode_event_kind='admitted'
 LEFT JOIN live.media_execution_state x ON x.operation_id=e.operation_id
 LEFT JOIN integration.operation_events t ON t.operation_id=e.operation_id AND t.episode_id=z.episode_id AND t.episode_event_kind='timeout'
 LEFT JOIN integration.operation_events w ON w.operation_id=e.operation_id AND w.episode_id=z.episode_id AND w.episode_event_kind='witnessed'
 LEFT JOIN integration.operation_events l ON l.operation_id=e.operation_id AND l.episode_id=z.episode_id AND l.episode_event_kind='terminal_at_lock'
 LEFT JOIN integration.operation_events n ON n.operation_id=e.operation_id AND n.episode_id=z.episode_id AND n.episode_event_kind='native_ineligible'
 LEFT JOIN integration.operation_events c ON c.operation_id=e.operation_id AND c.episode_id=z.episode_id AND c.episode_event_kind='ceiling'
 LEFT JOIN integration.operation_events q ON q.operation_id=e.operation_id AND q.episode_id=z.episode_id AND q.episode_event_kind='qualified'
 LEFT JOIN live.media_observations v ON v.id=q.observation_id
 WHERE z.episode_id=p_episode ORDER BY e.operation_id;
END $$;
ALTER FUNCTION live.read_media_recovery_episode(uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.read_media_recovery_episode(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.read_media_recovery_episode(uuid) TO commerce_media_recovery;

CREATE FUNCTION live.witness_media_recovery_episode(p_episode uuid,p_operation uuid,
 p_observation uuid,p_readback_elapsed_ms bigint) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 v_qualified integration.operation_events%ROWTYPE; v_witness integration.operation_events%ROWTYPE;
 v_now timestamptz;
BEGIN
 IF p_episode IS NULL OR p_operation IS NULL OR p_observation IS NULL
  OR p_readback_elapsed_ms IS NULL OR p_readback_elapsed_ms NOT BETWEEN 0 AND 90000
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media recovery witness' USING ERRCODE='ME400'; END IF;
 o:=live.lock_media_operation(p_operation);
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
 SELECT * INTO v_witness FROM integration.operation_events e WHERE e.operation_id=o.id
  AND e.episode_id=p_episode AND e.episode_event_kind='witnessed';
 IF v_witness.id IS NOT NULL THEN
  IF v_witness.observation_id=p_observation AND v_witness.elapsed_ms=p_readback_elapsed_ms THEN
   RETURN 'already_witnessed'; END IF;
  RETURN 'unqualified';
 END IF;
 IF x.recovery_timeout_at IS NOT NULL AND x.recovery_episode_id=p_episode
  OR EXISTS(SELECT 1 FROM integration.operation_events e WHERE e.operation_id=o.id
   AND e.episode_id=p_episode AND e.episode_event_kind='timeout') THEN
  RETURN 'timeout_wins'; END IF;
 SELECT * INTO v_qualified FROM integration.operation_events e WHERE e.operation_id=o.id
  AND e.episode_id=p_episode AND e.episode_event_kind='qualified';
 IF x.recovery_episode_id IS DISTINCT FROM p_episode OR v_qualified.id IS NULL
  OR v_qualified.observation_id<>p_observation
  OR NOT EXISTS (SELECT 1 FROM live.media_observations v WHERE v.id=p_observation
   AND v.operation_id=o.id AND v.source IN ('ROOM','QUERY')
   AND v.generation>x.recovery_baseline_generation) THEN RETURN 'unqualified'; END IF;
 v_now:=clock_timestamp();
 UPDATE live.media_execution_state SET recovery_disposition='witnessed',
  recovery_witness_elapsed_ms=p_readback_elapsed_ms,recovery_witness_at=v_now
  WHERE attempt_id=o.media_attempt_id AND recovery_episode_id=p_episode;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
  reason_code,episode_id,episode_event_kind,observation_id,elapsed_ms)
 VALUES(o.tenant_id,o.store_id,o.id,v_qualified.generation,o.state,o.lease_mode,
  'media_recovery_witnessed',p_episode,'witnessed',p_observation,p_readback_elapsed_ms);
 RETURN 'witnessed';
END $$;
ALTER FUNCTION live.witness_media_recovery_episode(uuid,uuid,uuid,bigint) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.witness_media_recovery_episode(uuid,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.witness_media_recovery_episode(uuid,uuid,uuid,bigint) TO commerce_media_recovery;

CREATE FUNCTION live.timeout_media_recovery_episode(p_episode uuid,p_elapsed_ms bigint)
RETURNS TABLE(disposition text,affected_count integer)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s live.media_recovery_episode_scope%ROWTYPE; o integration.operations%ROWTYPE;
 x live.media_execution_state%ROWTYPE; v_id uuid; v_count integer:=0; v_members integer;
 v_now timestamptz;
BEGIN
 IF p_episode IS NULL OR p_elapsed_ms IS NULL OR p_elapsed_ms<90000
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid media recovery timeout' USING ERRCODE='ME400'; END IF;
 SELECT * INTO s FROM live.media_recovery_episode_scope WHERE episode_id=p_episode;
 IF s.episode_id IS NULL THEN RAISE EXCEPTION 'media recovery episode unavailable' USING ERRCODE='ME409'; END IF;
 IF s.scope_status IN ('pending','overdue') AND s.candidate_count>0 THEN
  SELECT count(*) INTO v_members FROM integration.operation_events e
   WHERE e.episode_id=p_episode AND e.episode_event_kind='admitted';
  IF v_members<>s.candidate_count THEN
   RAISE EXCEPTION 'media recovery membership unavailable' USING ERRCODE='ME409'; END IF;
 END IF;
 IF s.scope_status='empty' AND s.coverage_known THEN
  RETURN QUERY SELECT 'already_finished'::text,0; RETURN; END IF;
 IF s.timeout_at IS NOT NULL THEN
  RETURN QUERY SELECT 'already_timed_out'::text,0; RETURN; END IF;
 FOR v_id IN SELECT e.operation_id FROM integration.operation_events e
  WHERE e.episode_id=p_episode AND e.episode_event_kind='admitted' ORDER BY e.operation_id LOOP
  o:=live.lock_media_operation(v_id);
  SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=o.media_attempt_id FOR UPDATE;
  IF NOT EXISTS (SELECT 1 FROM integration.operation_events w WHERE w.operation_id=v_id
   AND w.episode_id=p_episode AND w.episode_event_kind='witnessed')
   AND NOT EXISTS (SELECT 1 FROM integration.operation_events t WHERE t.operation_id=v_id
    AND t.episode_id=p_episode AND t.episode_event_kind='timeout') THEN
   v_now:=clock_timestamp();
   INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,
    reason_code,episode_id,episode_event_kind,elapsed_ms)
   VALUES(o.tenant_id,o.store_id,o.id,o.generation,o.state,o.lease_mode,
    'media_recovery_timeout',p_episode,'timeout',p_elapsed_ms);
   IF x.recovery_episode_id=p_episode THEN
    UPDATE live.media_execution_state SET recovery_disposition='timeout',recovery_timeout_at=v_now
     WHERE attempt_id=o.media_attempt_id;
   END IF;
   v_count:=v_count+1;
  END IF;
 END LOOP;
 -- A witness may commit while this call waits for an operation lock. Recheck
 -- completion only after all member locks, never from the pre-lock snapshot.
 IF s.coverage_known AND s.scope_status='pending' AND s.candidate_count>0
  AND NOT EXISTS(SELECT 1 FROM integration.operation_events e WHERE e.episode_id=p_episode
   AND e.episode_event_kind='admitted' AND NOT EXISTS
    (SELECT 1 FROM integration.operation_events w WHERE w.operation_id=e.operation_id
     AND w.episode_id=p_episode AND w.episode_event_kind='witnessed')) THEN
  RETURN QUERY SELECT 'already_finished'::text,0; RETURN; END IF;
 v_now:=clock_timestamp();
 UPDATE live.media_recovery_episode_scope SET timeout_at=v_now
  WHERE episode_id=p_episode AND timeout_at IS NULL;
 IF NOT FOUND THEN
  RETURN QUERY SELECT 'already_timed_out'::text,0; RETURN; END IF;
 RETURN QUERY SELECT 'timed_out'::text,v_count;
END $$;
ALTER FUNCTION live.timeout_media_recovery_episode(uuid,bigint) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.timeout_media_recovery_episode(uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.timeout_media_recovery_episode(uuid,bigint) TO commerce_media_recovery;
