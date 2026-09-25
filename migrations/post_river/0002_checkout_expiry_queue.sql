-- Applied after River under the existing migration advisory lock. No worker may
-- fetch a legacy job between validating its domain link and moving its queue.
LOCK TABLE river.river_job IN SHARE ROW EXCLUSIVE MODE;
GRANT UPDATE(queue) ON river.river_job TO commerce_checkout_writer;

CREATE FUNCTION checkout.expiry_job_linked(p_job bigint)
RETURNS boolean LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM river.river_job j JOIN checkout.orders o
  ON o.job_id=j.id
  WHERE j.id=p_job AND j.kind='checkout_expiry_v1'
   AND j.args=jsonb_build_object('order_id',o.id::text,'generation',1,'version',1)
   -- JSONB considers 1.0 equal to 1, but Go's integer decoder does not.
   AND j.args->>'generation'='1' AND j.args->>'version'='1')
$$;
ALTER FUNCTION checkout.expiry_job_linked(bigint) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.expiry_job_linked(bigint) FROM PUBLIC;

DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM river.river_job j
  WHERE j.queue='checkout_expiry_v1' AND j.kind<>'checkout_expiry_v1')
 OR EXISTS(SELECT 1 FROM river.river_job j
  WHERE j.kind='checkout_expiry_v1' AND j.state NOT IN ('completed','cancelled','discarded')
   AND (j.state='running' OR j.unique_key IS NOT NULL
    OR NOT checkout.expiry_job_linked(j.id)
    OR j.queue NOT IN ('default','checkout_expiry_v1'))) THEN
  RAISE EXCEPTION 'checkout expiry queue migration requires operator review' USING ERRCODE='55000';
 END IF;
END $$;
UPDATE river.river_job SET queue='checkout_expiry_v1'
 WHERE kind='checkout_expiry_v1' AND queue='default'
 AND state IN ('available','pending','scheduled','retryable');

CREATE FUNCTION checkout.route_expiry_queue_v1()
RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE j river.river_job%ROWTYPE;
BEGIN
 -- Begin inserts the job before its order in the same transaction. Deferred NEW
 -- is a snapshot: re-read the final row and reject same-transaction rewrites.
 SELECT x.* INTO j FROM river.river_job x WHERE x.id=NEW.id FOR UPDATE;
 IF NEW.kind='checkout_expiry_v1' OR NEW.queue='checkout_expiry_v1'
  OR j.kind='checkout_expiry_v1' OR j.queue='checkout_expiry_v1' THEN
  IF j.id IS NULL OR j.kind IS DISTINCT FROM NEW.kind OR j.args IS DISTINCT FROM NEW.args
   OR j.kind<>'checkout_expiry_v1' OR j.unique_key IS NOT NULL
   OR j.queue NOT IN ('default','checkout_expiry_v1')
   OR NOT checkout.expiry_job_linked(j.id) THEN
   RAISE EXCEPTION 'invalid checkout expiry queue job' USING ERRCODE='22023';
  END IF;
  IF j.queue<>'checkout_expiry_v1' THEN
   UPDATE river.river_job SET queue='checkout_expiry_v1' WHERE id=j.id;
  END IF;
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION checkout.route_expiry_queue_v1() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.route_expiry_queue_v1() FROM PUBLIC;
CREATE CONSTRAINT TRIGGER checkout_expiry_queue_route_v1 AFTER INSERT ON river.river_job
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
 EXECUTE FUNCTION checkout.route_expiry_queue_v1();

-- Startup checks route enforcement and active drift without disclosing orders.
-- Do not compare current order generation: payment-start advances it; the old
-- generation-1 job must reach expire_held's STALE fence, never release its stock.
CREATE FUNCTION checkout.expiry_queue_ready()
RETURNS boolean LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_trigger t
  JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid
  JOIN pg_catalog.pg_roles r ON r.oid=p.proowner
  WHERE t.tgrelid='river.river_job'::regclass AND t.tgname='checkout_expiry_queue_route_v1'
   AND t.tgfoid='checkout.route_expiry_queue_v1()'::regprocedure
   AND t.tgtype=5 AND t.tgdeferrable AND t.tginitdeferred AND t.tgenabled IN ('O','A')
   AND p.prosecdef AND r.rolname='commerce_checkout_writer' AND NOT r.rolcanlogin)
 AND NOT EXISTS(SELECT 1 FROM river.river_job j
  WHERE (j.queue='checkout_expiry_v1' AND j.kind<>'checkout_expiry_v1')
  OR (j.kind='checkout_expiry_v1' AND j.state NOT IN ('completed','cancelled','discarded')
   AND (j.unique_key IS NOT NULL OR j.queue<>'checkout_expiry_v1'
    OR NOT checkout.expiry_job_linked(j.id))))
$$;
ALTER FUNCTION checkout.expiry_queue_ready() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.expiry_queue_ready() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.expiry_queue_ready() TO commerce_worker;
