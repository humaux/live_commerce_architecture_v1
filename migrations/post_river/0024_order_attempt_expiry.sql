-- Purpose: private OA1 expiry-queue prerequisite, inserted before its native order in the outer Tx.
-- Depends on: root 0184 Begin, 0178 protected execution context, postRiver0005 expiry-family/link guards.
-- Used by: checkout.order_attempt_begin (NOLOGIN checkout domain owner only).
-- Invariants: I04/I08; exact frozen offline window, fresh nonunique scheduled native job,
-- no runtime queue writes or independent commit; existing deferred order link remains mandatory.

GRANT INSERT(kind,args,max_attempts,queue,state,scheduled_at) ON river_expiry.river_job TO commerce_checkout_writer;
GRANT USAGE ON SEQUENCE river_expiry.river_job_id_seq TO commerce_checkout_writer;

CREATE FUNCTION checkout.order_attempt_enqueue_expiry(p_attempt uuid,p_plan_plaintext text,p_created_at timestamptz)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; mode text; hours integer; expires timestamptz; job_id bigint;
BEGIN
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 mode:=ctx->'plan'->'facts'->'selection'->>'payment_mode';
 IF p_created_at IS NULL OR p_created_at<(ctx->'binding'->>'admitted_at')::timestamptz
  OR p_created_at>clock_timestamp() OR mode IS NULL OR mode NOT IN ('bank_transfer','pay_at_pickup','cash_on_delivery') THEN
  RAISE EXCEPTION 'attempt_expiry_invalid' USING ERRCODE='PT400'; END IF;
 expires:=p_created_at+interval '15 minutes';
 IF mode='bank_transfer' THEN
  hours:=(ctx->'plan'->'facts'->'catalog'->'payment'->>'window_hours')::integer;
  IF hours IS NULL OR hours NOT BETWEEN 6 AND 168 THEN
   RAISE EXCEPTION 'attempt_expiry_invalid' USING ERRCODE='PT400'; END IF;
  expires:=p_created_at+make_interval(hours=>hours);
 END IF;
 -- Native BEFORE family guard and deferred route/link guard are retained.
 -- A new job adds no lock on an existing business record.
 INSERT INTO river_expiry.river_job(kind,args,max_attempts,queue,state,scheduled_at)
 VALUES('checkout_expiry_v1',jsonb_build_object('order_id',p_attempt::text,'generation',1,'version',1),25,
  'checkout_expiry_v1','scheduled',expires) RETURNING id INTO job_id;
 RETURN jsonb_build_object('job_id',job_id,'expires_at',expires);
END $$;
ALTER FUNCTION checkout.order_attempt_enqueue_expiry(uuid,text,timestamptz) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_attempt_enqueue_expiry(uuid,text,timestamptz) FROM PUBLIC;
COMMENT ON FUNCTION checkout.order_attempt_enqueue_expiry(uuid,text,timestamptz) IS
 'Private checkout-owner-only OA1 Begin dependency: protected exact current-xid plan, native generation-one/version-one expiry job, frozen bank window or15minutes, exact scheduled_at. New job before native order in same Tx, existing deferred link guard retained. NOLOGIN owner implicit EXECUTE only; no runtime/coordinator grant.';
