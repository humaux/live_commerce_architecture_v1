-- Purpose: fresh transaction-bound order-attempt River enqueue and reciprocal links.
-- Depends: 0170/0172 protected admission storage, upstream River and its existing ACLs.
-- Used by: runtime C1 InsertTx, claims-worker fenced re-enqueue and periodic sweep.
-- Invariants: INSERT consumes one protected witness; identity UPDATE cannot admit;
-- final-state commit checks run after caller GUC restoration; no PII in job args.

-- Reuse the integration owner's existing River SELECT; the attempt coordinator
-- gets only an exact boolean helper, never direct River or other-domain DML.
GRANT USAGE ON SCHEMA ops TO commerce_integration_writer;
CREATE FUNCTION ops.order_attempt_job_matches(p_job bigint,p_attempt uuid,p_generation bigint)
RETURNS boolean LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM river.river_job j WHERE j.id=p_job
  AND j.kind='order_attempt_v1' AND j.queue='claims' AND j.priority=1
  AND j.unique_key IS NULL
  AND j.args=jsonb_build_object('attempt_id',p_attempt::text,'intent_generation',p_generation))
$$;
ALTER FUNCTION ops.order_attempt_job_matches(bigint,uuid,bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION ops.order_attempt_job_matches(bigint,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.order_attempt_job_matches(bigint,uuid,bigint) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION ops.order_attempt_job_matches(bigint,uuid,bigint) IS
 'Private exact River identity predicate for order-attempt reciprocal triggers; only coordinator EXECUTE, no runtime API or business DML.';

CREATE FUNCTION ops.guard_order_attempt_job() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE; generation bigint; prior_tenant text; prior_store text;
BEGIN
 IF NEW.kind NOT IN ('order_attempt_v1','order_attempt_sweep_v1') AND NEW.queue<>'claims'
  AND NOT (TG_OP='UPDATE' AND (OLD.kind IN ('order_attempt_v1','order_attempt_sweep_v1') OR OLD.queue='claims')) THEN
  RETURN NEW;
 END IF;
 IF TG_OP='UPDATE' THEN
  -- River's ON CONFLICT UPDATE(kind) and lifecycle updates preserve identity.
  -- In particular they never consume or refresh an enqueue witness.
  IF ROW(NEW.id,NEW.kind,NEW.args,NEW.queue,NEW.priority,NEW.unique_key)
   IS DISTINCT FROM ROW(OLD.id,OLD.kind,OLD.args,OLD.queue,OLD.priority,OLD.unique_key) THEN
   RAISE EXCEPTION 'immutable order attempt job identity' USING ERRCODE='22023';
  END IF;
  RETURN NEW;
 END IF;
 IF NEW.kind NOT IN ('order_attempt_v1','order_attempt_sweep_v1')
  OR NEW.queue IS DISTINCT FROM 'claims' OR NEW.priority IS DISTINCT FROM 1
  OR NEW.unique_key IS NOT NULL OR NEW.args IS NULL OR jsonb_typeof(NEW.args)<>'object' THEN
  RAISE EXCEPTION 'invalid order attempt job' USING ERRCODE='22023';
 END IF;
 IF NEW.kind='order_attempt_sweep_v1' THEN
  IF NEW.args<>'{}'::jsonb OR NOT pg_has_role(session_user,'commerce_claims_worker','MEMBER') THEN
   RAISE EXCEPTION 'invalid order attempt sweep job' USING ERRCODE='22023';
  END IF;
  RETURN NEW;
 END IF;
 IF NOT (pg_has_role(session_user,'commerce_runtime','MEMBER')
  OR pg_has_role(session_user,'commerce_claims_worker','MEMBER'))
  OR jsonb_typeof(NEW.args->'attempt_id') IS DISTINCT FROM 'string'
  OR NEW.args->>'attempt_id' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR jsonb_typeof(NEW.args->'intent_generation') IS DISTINCT FROM 'number'
  OR NEW.args->>'intent_generation' !~ '^[1-9][0-9]{0,18}$' THEN
  RAISE EXCEPTION 'invalid order attempt job' USING ERRCODE='22023';
 END IF;
 BEGIN
  generation:=(NEW.args->>'intent_generation')::bigint;
 EXCEPTION WHEN numeric_value_out_of_range THEN
  RAISE EXCEPTION 'invalid order attempt job' USING ERRCODE='22023';
 END;
 IF NEW.args IS DISTINCT FROM jsonb_build_object('attempt_id',NEW.args->>'attempt_id','intent_generation',generation) THEN
  RAISE EXCEPTION 'invalid order attempt job' USING ERRCODE='22023';
 END IF;
 -- 0172's SELECT-only locator exposes only a protected witness minted in this
 -- transaction. No caller GUC or xmin/new-row shortcut supplies authority.
 SELECT * INTO a FROM ops.order_attempts WHERE attempt_id=(NEW.args->>'attempt_id')::uuid
  AND intent_generation=generation AND enqueue_xid=pg_current_xact_id()
  AND enqueue_job_id IS NULL AND state='PENDING';
 IF NOT FOUND THEN
  RAISE EXCEPTION 'missing order attempt enqueue witness' USING ERRCODE='22023';
 END IF;
 prior_tenant:=current_setting('app.tenant_id',true);
 prior_store:=current_setting('app.store_id',true);
 PERFORM set_config('app.tenant_id',a.tenant_id::text,true);
 PERFORM set_config('app.store_id',a.store_id::text,true);
 -- C1 owns its new row and sweep already holds its attempt fence. Recheck every
 -- witness field in UPDATE; the private row is the authority, not caller scope.
 UPDATE ops.order_attempts SET enqueue_job_id=NEW.id WHERE attempt_id=a.attempt_id
  AND tenant_id=a.tenant_id AND store_id=a.store_id AND intent_generation=generation
  AND enqueue_xid=pg_current_xact_id() AND enqueue_job_id IS NULL AND state='PENDING';
 IF NOT FOUND THEN
  RAISE EXCEPTION 'spent order attempt enqueue witness' USING ERRCODE='22023';
 END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 RETURN NEW;
END $$;
ALTER FUNCTION ops.guard_order_attempt_job() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.guard_order_attempt_job() FROM PUBLIC;
COMMENT ON FUNCTION ops.guard_order_attempt_job() IS
 'Internal River trigger: exact opaque args/lane, immutable identity and INSERT-only consumption of current protected enqueue witness; no application EXECUTE.';
CREATE TRIGGER order_attempt_job_family BEFORE INSERT OR UPDATE ON river.river_job
 FOR EACH ROW EXECUTE FUNCTION ops.guard_order_attempt_job();

CREATE FUNCTION ops.check_order_attempt_job_link() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE;
BEGIN
 -- Re-read both final rows. INSERT's NEW has neither C1's final admission nor
 -- proof that the job survived later statements in this transaction.
 SELECT * INTO a FROM ops.order_attempts WHERE attempt_id=(NEW.args->>'attempt_id')::uuid
  AND intent_generation=(NEW.args->>'intent_generation')::bigint
  AND enqueue_xid=pg_current_xact_id();
 IF NOT FOUND OR NOT a.admission_finished OR a.enqueue_job_id IS DISTINCT FROM NEW.id
  OR a.execution_job_id IS DISTINCT FROM NEW.id
  OR NOT ops.order_attempt_job_matches(NEW.id,a.attempt_id,a.intent_generation) THEN
  RAISE EXCEPTION 'unlinked order attempt job' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION ops.check_order_attempt_job_link() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.check_order_attempt_job_link() FROM PUBLIC;
COMMENT ON FUNCTION ops.check_order_attempt_job_link() IS
 'Internal deferred new-job-to-attempt constraint: final sealed admission and exact bound fresh job, independent of caller GUCs.';
CREATE CONSTRAINT TRIGGER order_attempt_job_commit AFTER INSERT ON river.river_job
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (NEW.kind='order_attempt_v1')
 EXECUTE FUNCTION ops.check_order_attempt_job_link();

CREATE FUNCTION ops.check_order_attempt_enqueue_link() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE; prior_tenant text; prior_store text; present boolean;
BEGIN
 -- An existing unchanged pointer may outlive River retention. C2 settlement
 -- does not require a permanent River FK; new C1/sweep bindings do require it.
 IF TG_OP='UPDATE' AND ROW(NEW.enqueue_xid,NEW.enqueue_job_id,NEW.execution_job_id,NEW.admission_finished)
  IS NOT DISTINCT FROM ROW(OLD.enqueue_xid,OLD.enqueue_job_id,OLD.execution_job_id,OLD.admission_finished) THEN
  RETURN NULL;
 END IF;
 prior_tenant:=current_setting('app.tenant_id',true);
 prior_store:=current_setting('app.store_id',true);
 PERFORM set_config('app.tenant_id',NEW.tenant_id::text,true);
 PERFORM set_config('app.store_id',NEW.store_id::text,true);
 SELECT * INTO a FROM ops.order_attempts WHERE attempt_id=NEW.attempt_id
  AND tenant_id=NEW.tenant_id AND store_id=NEW.store_id;
 present:=FOUND;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 IF NOT present OR NOT a.admission_finished OR a.enqueue_xid IS DISTINCT FROM pg_current_xact_id()
  OR a.enqueue_job_id IS NULL OR a.execution_job_id IS DISTINCT FROM a.enqueue_job_id
  OR NOT ops.order_attempt_job_matches(a.enqueue_job_id,a.attempt_id,a.intent_generation) THEN
  RAISE EXCEPTION 'order attempt has no fresh execution job' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION ops.check_order_attempt_enqueue_link() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.check_order_attempt_enqueue_link() FROM PUBLIC;
COMMENT ON FUNCTION ops.check_order_attempt_enqueue_link() IS
 'Internal deferred attempt-to-job constraint: re-read final admission/pointer/current witness; no permanent River retention dependency after enqueue commits.';
CREATE CONSTRAINT TRIGGER order_attempt_enqueue_commit AFTER INSERT OR UPDATE ON ops.order_attempts
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ops.check_order_attempt_enqueue_link();
