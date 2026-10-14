-- Purpose: native River liveness and reciprocal repair-witness enforcement.
-- Depends on: 0191 repair storage, 0023 initial admission/job guards, upstream River.
-- Used by: worker-only sweep prepare/InsertTx/finish on one physical pgx.Tx.
-- Invariants: C1 keeps its original witness; repair INSERT never updates attempt;
-- only finish binds the fresh job, and repair must commit before C2 settlement.

CREATE FUNCTION ops.order_attempt_job_live(p_job bigint,p_attempt uuid,p_generation bigint)
RETURNS boolean LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM river.river_job j WHERE j.id=p_job
  AND j.kind='order_attempt_v1' AND j.queue='claims' AND j.priority=1
  AND j.unique_key IS NULL
  AND j.args=jsonb_build_object('attempt_id',p_attempt::text,'intent_generation',p_generation)
  AND j.state IN ('available','pending','scheduled','retryable','running'))
$$;
ALTER FUNCTION ops.order_attempt_job_live(bigint,uuid,bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION ops.order_attempt_job_live(bigint,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.order_attempt_job_live(bigint,uuid,bigint) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION ops.order_attempt_job_live(bigint,uuid,bigint) IS
 'Private exact native River liveness read after attempt fence. Missing/completed/discarded/cancelled are orphaned; running/retryable remain live even at max_attempts. No River row locks or caller grants.';

CREATE OR REPLACE FUNCTION ops.guard_order_attempt_job() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE; generation bigint;
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
BEGIN
 IF NEW.kind NOT IN ('order_attempt_v1','order_attempt_sweep_v1') AND NEW.queue<>'claims'
  AND NOT (TG_OP='UPDATE' AND (OLD.kind IN ('order_attempt_v1','order_attempt_sweep_v1') OR OLD.queue='claims')) THEN
  RETURN NEW;
 END IF;
 IF TG_OP='UPDATE' THEN
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
 PERFORM set_config('app.tenant_id','',true);
 PERFORM set_config('app.store_id','',true);
 SELECT x.* INTO a FROM ops.order_attempts x WHERE x.attempt_id=(NEW.args->>'attempt_id')::uuid
  AND x.intent_generation=generation AND x.state='PENDING';
 IF NOT FOUND THEN
  RAISE EXCEPTION 'missing order attempt enqueue witness' USING ERRCODE='22023';
 END IF;
 PERFORM set_config('app.tenant_id',a.tenant_id::text,true);
 PERFORM set_config('app.store_id',a.store_id::text,true);
 IF a.admission_finished THEN
  -- prepare already holds the attempt. Only its separate witness is mutable
  -- here; finish remains the sole attempt UPDATE and cannot reverse-lock scope.
  IF NOT pg_has_role(session_user,'commerce_claims_worker','MEMBER') THEN
   RAISE EXCEPTION 'missing order attempt repair witness' USING ERRCODE='22023';
  END IF;
  UPDATE ops.order_attempt_repair_enqueues SET enqueue_job_id=NEW.id
   WHERE attempt_id=a.attempt_id AND tenant_id=a.tenant_id AND store_id=a.store_id
    AND intent_generation=generation AND enqueue_xid=pg_current_xact_id() AND enqueue_job_id IS NULL;
 ELSE
  -- C1's unfinished candidate retains the original protected enqueue slot.
  UPDATE ops.order_attempts SET enqueue_job_id=NEW.id WHERE attempt_id=a.attempt_id
   AND tenant_id=a.tenant_id AND store_id=a.store_id AND intent_generation=generation
   AND enqueue_xid=pg_current_xact_id() AND enqueue_job_id IS NULL AND state='PENDING';
 END IF;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'spent order attempt enqueue witness' USING ERRCODE='22023';
 END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 RETURN NEW;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.guard_order_attempt_job() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.guard_order_attempt_job() FROM PUBLIC;
COMMENT ON FUNCTION ops.guard_order_attempt_job() IS
 'Internal exact-identity River guard. C1 consumes its unfinished attempt slot; admitted repair consumes only protected current-xid repair storage. No runtime helper EXECUTE or business DML.';

CREATE OR REPLACE FUNCTION ops.check_order_attempt_job_link() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE; repair boolean;
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
BEGIN
 PERFORM set_config('app.tenant_id','',true);
 PERFORM set_config('app.store_id','',true);
 SELECT x.* INTO a FROM ops.order_attempts x WHERE x.attempt_id=(NEW.args->>'attempt_id')::uuid
  AND x.intent_generation=(NEW.args->>'intent_generation')::bigint
  AND x.enqueue_xid=pg_current_xact_id();
 IF NOT FOUND OR NOT a.admission_finished OR a.enqueue_job_id IS DISTINCT FROM NEW.id
  OR a.execution_job_id IS DISTINCT FROM NEW.id
  OR NOT ops.order_attempt_job_matches(NEW.id,a.attempt_id,a.intent_generation) THEN
  RAISE EXCEPTION 'unlinked order attempt job' USING ERRCODE='23514';
 END IF;
 SELECT EXISTS(SELECT 1 FROM ops.order_attempt_repair_enqueues r WHERE r.attempt_id=a.attempt_id
  AND r.tenant_id=a.tenant_id AND r.store_id=a.store_id AND r.intent_generation=a.intent_generation
  AND r.enqueue_xid=pg_current_xact_id() AND r.enqueue_job_id=NEW.id) INTO repair;
 -- A caller may not combine the repair UPDATE and a later C2 terminal UPDATE
 -- in this physical transaction. Such a path would reacquire the scope FK.
 IF repair AND a.state<>'PENDING' THEN
  RAISE EXCEPTION 'repair must commit before execution' USING ERRCODE='23514';
 END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 RETURN NULL;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.check_order_attempt_job_link() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.check_order_attempt_job_link() FROM PUBLIC;
COMMENT ON FUNCTION ops.check_order_attempt_job_link() IS
 'Deferred fresh-job reciprocal proof: final current-xid admitted attempt, exact native job and both final pointers. Repair rows must still be PENDING; execute only in a later transaction. Historical job retention remains independent.';
