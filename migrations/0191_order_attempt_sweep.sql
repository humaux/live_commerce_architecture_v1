-- Purpose: bounded worker-only discovery and attempt-first execution-job repair.
-- Depends: protected attempt identity, 0178 fence conventions and post-River0025.
-- Used by: Executor.Sweep prepare/InsertTx/finish; expired rows execute separately.
-- Invariants: no scope/principal/subject locks; one attempt UPDATE per repair;
-- only a current-xid witness can authorize a fresh native River job.

CREATE TABLE ops.order_attempt_repair_enqueues (
 attempt_id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 intent_generation bigint NOT NULL CHECK(intent_generation>0),
 enqueue_xid xid8 NOT NULL,
 enqueue_job_id bigint CHECK(enqueue_job_id>0),
 FOREIGN KEY(tenant_id,store_id,attempt_id)
  REFERENCES ops.order_attempts(tenant_id,store_id,attempt_id) ON DELETE CASCADE
);
ALTER TABLE ops.order_attempt_repair_enqueues OWNER TO commerce_order_attempt_owner;
ALTER TABLE ops.order_attempt_repair_enqueues ENABLE ROW LEVEL SECURITY;
ALTER TABLE ops.order_attempt_repair_enqueues FORCE ROW LEVEL SECURITY;
CREATE POLICY order_attempt_repair_owner ON ops.order_attempt_repair_enqueues
 TO commerce_order_attempt_owner
 USING(enqueue_xid=pg_current_xact_id() OR
  (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND store_id=nullif(current_setting('app.store_id',true),'')::uuid))
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
REVOKE ALL ON ops.order_attempt_repair_enqueues FROM PUBLIC;
COMMENT ON TABLE ops.order_attempt_repair_enqueues IS
 'Private repair witness. Its only FK is to the already-locked attempt; never to scope or principal. River INSERT consumes the slot here; finish alone updates the attempt. Old xids authorize nothing; cascade with retained attempt deletion.';

CREATE FUNCTION ops.order_attempt_sweep(p_phase text,p_attempt uuid,p_generation bigint)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE; w ops.order_attempt_repair_enqueues%ROWTYPE;
 result jsonb; disposition text;
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true);
BEGIN
 IF NOT pg_has_role(session_user,'commerce_claims_worker','MEMBER')
  OR pg_has_role(session_user,'commerce_runtime','MEMBER') THEN
  RAISE EXCEPTION 'invalid_sweep_authority' USING ERRCODE='PT403';
 END IF;
 IF current_setting('transaction_isolation')<>'read committed'
  OR p_phase IS NULL OR p_phase NOT IN ('discover','prepare','finish')
  OR (p_phase='discover' AND (p_attempt IS NOT NULL OR p_generation IS NOT NULL))
  OR (p_phase<>'discover' AND (p_attempt IS NULL OR p_generation IS NULL OR p_generation<1)) THEN
  RAISE EXCEPTION 'invalid_sweep_request' USING ERRCODE='PT400';
 END IF;
 PERFORM set_config('app.tenant_id','',true);
 PERFORM set_config('app.store_id','',true);
 PERFORM set_config('app.principal_id','',true);
 IF p_phase='discover' THEN
  SELECT coalesce(jsonb_agg(jsonb_build_object('attempt_id',x.attempt_id,
   'intent_generation',x.intent_generation) ORDER BY x.admitted_at,x.attempt_id),'[]'::jsonb)
  INTO result FROM (SELECT attempt_id,intent_generation,admitted_at FROM ops.order_attempts
   WHERE state='PENDING' AND admission_finished ORDER BY admitted_at,attempt_id LIMIT 100) x;
 ELSE
  -- The unlocked locator supplies scope only; re-read every mutable field under
  -- the first row lock. Never inspect or lock a River row before that fence.
  SELECT x.* INTO a FROM ops.order_attempts x WHERE x.attempt_id=p_attempt;
  IF FOUND THEN
   PERFORM set_config('app.tenant_id',a.tenant_id::text,true);
   PERFORM set_config('app.store_id',a.store_id::text,true);
   SELECT x.* INTO a FROM ops.order_attempts x WHERE x.attempt_id=p_attempt
    AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id FOR UPDATE SKIP LOCKED;
  END IF;
  IF NOT FOUND OR a.state<>'PENDING' OR NOT a.admission_finished OR a.intent_generation<>p_generation THEN
   IF p_phase='finish' THEN
    RAISE EXCEPTION 'attempt_repair_unavailable' USING ERRCODE='PT409';
   END IF;
   disposition:='SKIP';
  ELSIF p_phase='prepare' THEN
   IF clock_timestamp()>=a.execute_by THEN
    disposition:='EXPIRED';
   ELSIF ops.order_attempt_job_live(a.execution_job_id,a.attempt_id,a.intent_generation) THEN
    disposition:='SKIP';
   ELSE
    -- Do not modify the attempt here: a second UPDATE in this Tx would cause
    -- PostgreSQL to recheck its unchanged scope FK after the attempt lock.
    IF EXISTS(SELECT 1 FROM ops.order_attempt_repair_enqueues r
     WHERE r.attempt_id=a.attempt_id AND r.enqueue_xid=pg_current_xact_id()) THEN
     RAISE EXCEPTION 'attempt_repair_already_prepared' USING ERRCODE='PT409';
    END IF;
    INSERT INTO ops.order_attempt_repair_enqueues(attempt_id,tenant_id,store_id,intent_generation,enqueue_xid)
     VALUES(a.attempt_id,a.tenant_id,a.store_id,a.intent_generation,pg_current_xact_id())
     ON CONFLICT(attempt_id) DO UPDATE SET enqueue_xid=EXCLUDED.enqueue_xid,enqueue_job_id=NULL;
    disposition:='REQUEUE';
   END IF;
  ELSE
   SELECT r.* INTO w FROM ops.order_attempt_repair_enqueues r WHERE r.attempt_id=a.attempt_id
    AND r.tenant_id=a.tenant_id AND r.store_id=a.store_id AND r.intent_generation=a.intent_generation
    AND r.enqueue_xid=pg_current_xact_id() AND r.enqueue_job_id IS NOT NULL;
   IF NOT FOUND OR NOT ops.order_attempt_job_live(w.enqueue_job_id,a.attempt_id,a.intent_generation)
    OR a.execution_job_id=w.enqueue_job_id THEN
    RAISE EXCEPTION 'attempt_repair_unavailable' USING ERRCODE='PT409';
   END IF;
   IF clock_timestamp()>=a.execute_by THEN
    RAISE EXCEPTION 'attempt_expired' USING ERRCODE='PT409';
   END IF;
   UPDATE ops.order_attempts SET enqueue_xid=pg_current_xact_id(),enqueue_job_id=w.enqueue_job_id,
    execution_job_id=w.enqueue_job_id WHERE attempt_id=a.attempt_id AND tenant_id=a.tenant_id
    AND store_id=a.store_id AND intent_generation=a.intent_generation AND state='PENDING';
   IF NOT FOUND THEN RAISE EXCEPTION 'attempt_repair_unavailable' USING ERRCODE='PT409'; END IF;
   result:=jsonb_build_object('attempt_id',a.attempt_id,'intent_generation',a.intent_generation,
    'execution_job_id',w.enqueue_job_id);
  END IF;
  IF p_phase='prepare' THEN
   result:=jsonb_build_object('action',disposition,'attempt_id',p_attempt,'intent_generation',p_generation);
  END IF;
 END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.order_attempt_sweep(text,uuid,bigint) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.order_attempt_sweep(text,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.order_attempt_sweep(text,uuid,bigint) TO commerce_claims_worker;
COMMENT ON FUNCTION ops.order_attempt_sweep(text,uuid,bigint) IS
 'Worker-only OrderAttemptSweep: opaque oldest100 unlocked discovery, one-attempt SKIP LOCKED prepare, and single-UPDATE finish after native InsertTx. Expiry returns metadata only; caller ends repair Tx before normal fenced Execute. No runtime grants, scope locks or deadline extension.';
