-- River is created by its upstream runner, before this checksummed phase.
-- Lock out fetch/update/insert races until validation, backfill and routing
-- become visible together. Never move a job already claimed by a worker.
LOCK TABLE river.river_job IN SHARE ROW EXCLUSIVE MODE;
GRANT UPDATE(queue) ON river.river_job TO commerce_integration_writer;

-- Exact immutable domain linkage determines profile. No caller-supplied UUID
-- cast, environment or mutable merchant configuration influences routing.
CREATE FUNCTION integration.payment_job_queue(p_job bigint)
RETURNS text LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT CASE a.execution_profile
  WHEN 'PROVIDER_MOCK' THEN 'payment_mock_v1'
  WHEN 'SANDBOX' THEN 'payment_sandbox_v1'
  WHEN 'LIVE' THEN 'payment_live_v1' END
 FROM river.river_job j JOIN checkout.payment_attempts a
  ON a.id::text=j.args->>'operation_id'
 WHERE j.id=p_job AND (
  (j.kind='payment_query_v1' AND a.job_id=j.id
   AND j.args=jsonb_build_object('operation_id',a.id::text,'version',1))
  OR (j.kind='payment_reconcile_v1' AND EXISTS(
   SELECT 1 FROM payments.provider_observations o
   WHERE o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.attempt_id=a.id
    AND o.source='QUERY' AND o.execution_profile=a.execution_profile
    AND o.environment=a.environment
    AND j.args=jsonb_build_object('operation_id',a.id::text,
     'report_hash',encode(o.report_hash,'hex'),'version',1))))
$$;
ALTER FUNCTION integration.payment_job_queue(bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.payment_job_queue(bigint) FROM PUBLIC;

DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM river.river_job j
  WHERE j.queue IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1')
   AND j.kind NOT IN ('payment_query_v1','payment_reconcile_v1'))
 OR EXISTS(SELECT 1 FROM river.river_job j
  WHERE j.kind IN ('payment_query_v1','payment_reconcile_v1')
   AND j.state NOT IN ('completed','cancelled','discarded')
   AND (j.state='running' OR j.unique_key IS NOT NULL
    OR integration.payment_job_queue(j.id) IS NULL
    OR j.queue NOT IN ('default',integration.payment_job_queue(j.id)))) THEN
  RAISE EXCEPTION 'payment queue migration requires operator review' USING ERRCODE='55000';
 END IF;
END $$;
UPDATE river.river_job j SET queue=integration.payment_job_queue(j.id)
 WHERE j.kind IN ('payment_query_v1','payment_reconcile_v1') AND j.queue='default'
 AND j.state IN ('available','pending','scheduled','retryable');

CREATE FUNCTION integration.route_payment_queue_v1()
RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE j river.river_job%ROWTYPE; expected text;
BEGIN
 -- Deferred NEW is a snapshot. Inspect the final row under lock before changing
 -- only queue, and disallow a same-transaction kind/args rewrite.
 SELECT x.* INTO j FROM river.river_job x WHERE x.id=NEW.id FOR UPDATE;
 IF NEW.kind IN ('payment_query_v1','payment_reconcile_v1')
  OR NEW.queue IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1')
  OR j.kind IN ('payment_query_v1','payment_reconcile_v1')
  OR j.queue IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1') THEN
  IF j.id IS NULL OR j.kind IS DISTINCT FROM NEW.kind OR j.args IS DISTINCT FROM NEW.args
   OR j.kind NOT IN ('payment_query_v1','payment_reconcile_v1') OR j.unique_key IS NOT NULL THEN
   RAISE EXCEPTION 'invalid payment queue job' USING ERRCODE='22023';
  END IF;
  expected:=integration.payment_job_queue(j.id);
  IF expected IS NULL OR j.queue NOT IN ('default',expected) THEN
   RAISE EXCEPTION 'payment queue linkage mismatch' USING ERRCODE='22023';
  END IF;
  IF j.queue<>expected THEN
   UPDATE river.river_job SET queue=expected WHERE id=j.id;
  END IF;
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION integration.route_payment_queue_v1() OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.route_payment_queue_v1() FROM PUBLIC;
CREATE CONSTRAINT TRIGGER payment_queue_route_v1 AFTER INSERT ON river.river_job
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
 EXECUTE FUNCTION integration.route_payment_queue_v1();

-- Boolean-only startup audit; runtime receives no frozen credential/report read
-- authority. Valid concurrent profiles coexist, but each client consumes one.
CREATE FUNCTION integration.payment_queue_ready()
RETURNS boolean LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_trigger t
  JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid
  JOIN pg_catalog.pg_roles r ON r.oid=p.proowner
  WHERE t.tgrelid='river.river_job'::regclass AND t.tgname='payment_queue_route_v1'
   AND t.tgfoid='integration.route_payment_queue_v1()'::regprocedure
   AND t.tgtype=5 AND t.tgdeferrable AND t.tginitdeferred AND t.tgenabled IN ('O','A')
   AND p.prosecdef AND r.rolname='commerce_integration_writer' AND NOT r.rolcanlogin)
 AND NOT EXISTS(SELECT 1 FROM river.river_job j
  WHERE (j.queue IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1')
   AND j.kind NOT IN ('payment_query_v1','payment_reconcile_v1'))
  OR (j.kind IN ('payment_query_v1','payment_reconcile_v1')
   AND j.state NOT IN ('completed','cancelled','discarded')
   AND (j.unique_key IS NOT NULL
    OR j.queue IS DISTINCT FROM integration.payment_job_queue(j.id))))
$$;
ALTER FUNCTION integration.payment_queue_ready() OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.payment_queue_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.payment_queue_ready() TO commerce_worker;
