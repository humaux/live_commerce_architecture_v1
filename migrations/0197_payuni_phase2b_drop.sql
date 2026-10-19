-- Purpose: PAY-RM2 phase 2b (owner-approved 2026-10-19): forward-only removal of the retained PAYUNi SQL
--   execution authority that phase 2a left behind (output/pay-rm2-p2a/DELIVERY.md "Phase 2b"): the query
--   guard/loader/finisher, the hosted-form material definers, the private apply_capture_payuni_v1 body and
--   the PAYUNi branch of the shared payments.apply_capture dispatcher. Every EXECUTE grant goes with its function.
-- Depends on: 0017/0018/0096 (payment query definers), 0025 (hosted material definers), 0061/0062 (dispatcher
--   and apply_capture_payuni_v1). checkout.start_payment and integration.record_payment_query are dropped by
--   post_river/0026_payuni_phase2b_drop.sql instead: post_river/0005 CREATE OR REPLACEs them after this numbered
--   phase on a fresh database, so dropping them here would only be undone there.
-- Used by: migrations.Apply (numbered phase); tests/foundation/payuni_phase2b_test.go (P2B01-P2B03).
-- Invariants: no row is deleted or rewritten. Historical attempts, operations, observations, facts, hosted pages,
--   orders, reservations and their read paths (hosted_payment_view[_v2], hosted_payment_provider, merchant
--   projections) stay. The refusal below runs before any DDL in the one numbered-phase transaction, so a refused
--   run leaves the database unchanged. The Stripe branch of apply_capture is byte-identical to 0062; CREATE OR
--   REPLACE keeps its OID, owner, SECURITY DEFINER, search_path and ACL. A PAYUNi attempt now reaches the
--   dispatcher's existing PT409 'payment operation unavailable' refusal and writes nothing.

-- 1) Refuse while any PAYUNi money state is unreconciled: a non-terminal PAYUNi query operation (the buyer may
--    have paid and only the dropped query/record path could observe it) or a PAYUNi attempt whose order still
--    awaits payment. row_security=off makes a role that is subject to RLS fail here instead of counting zero.
DO $$
DECLARE v_ops bigint; v_orders bigint;
BEGIN
 PERFORM set_config('row_security','off',true);
 SELECT count(*) INTO v_ops FROM integration.operations o
  WHERE o.provider='payuni' AND o.state IN ('READY','DISPATCHING','UNKNOWN','ACKNOWLEDGED');
 SELECT count(*) INTO v_orders FROM checkout.payment_attempts a
  JOIN checkout.orders o ON o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.id=a.order_id
  WHERE a.method_code='payuni_credit' AND o.commercial_state='AWAITING_PAYMENT';
 PERFORM set_config('row_security','on',true);
 IF v_ops>0 OR v_orders>0 THEN
  RAISE EXCEPTION 'PAYUNi phase-2b drop refused: % non-terminal PAYUNi operation(s), % PAYUNi order(s) awaiting payment; an operator must reconcile them first, data is never changed silently',
   v_ops, v_orders USING ERRCODE='22023';
 END IF;
END $$;

-- 2) Dispatcher: the 0062 body with only the PAYUNi branch removed (its IF becomes the Stripe test).
CREATE OR REPLACE FUNCTION payments.apply_capture(p_attempt uuid,p_report_hash bytea)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_provider text; v_attempt checkout.payment_attempts%ROWTYPE; v_object text;
BEGIN
 IF p_attempt IS NULL OR p_report_hash IS NULL OR octet_length(p_report_hash)<>32 THEN
  RAISE EXCEPTION 'invalid capture input' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO v_attempt FROM checkout.payment_attempts x WHERE x.id=p_attempt;
 IF NOT FOUND THEN RAISE EXCEPTION 'payment operation unavailable' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.tenant_id',v_attempt.tenant_id::text,true);
 PERFORM set_config('app.store_id',v_attempt.store_id::text,true);
 PERFORM set_config('app.buyer_id',v_attempt.owner_id::text,true);
 SELECT o.provider INTO v_provider FROM integration.operations o WHERE o.id=p_attempt
  AND o.tenant_id=v_attempt.tenant_id AND o.store_id=v_attempt.store_id
  AND o.actor_kind='BUYER_PAYMENT_QUERY';
 IF v_provider='stripe' THEN
  -- refund and charge reports carry an Object key; checkout reports never do
  SELECT x.report->>'Object' INTO v_object FROM payments.provider_observations x
   WHERE x.tenant_id=v_attempt.tenant_id AND x.store_id=v_attempt.store_id AND x.attempt_id=p_attempt
    AND x.report_hash=p_report_hash;
  IF v_object='refund' THEN
   PERFORM payments.apply_stripe_refund(p_attempt,p_report_hash);
  ELSIF v_object='charge' THEN
   PERFORM payments.apply_stripe_charge(p_attempt,p_report_hash);
  ELSE
   PERFORM payments.apply_stripe_observation(p_attempt,p_report_hash);
  END IF;
 ELSE
  RAISE EXCEPTION 'payment operation unavailable' USING ERRCODE='PT409';
 END IF;
END $$;
COMMENT ON FUNCTION payments.apply_capture(uuid,bytea) IS 'payments owner; worker calls provider dispatcher; Stripe checkout, Stripe refund and Stripe charge retain separate private fact paths, no direct ledger write by worker; PAYUNi attempts are refused (PAY-RM2 phase 2b)';

-- 3) Drop the retained PAYUNi execution definers. No CASCADE: an unexpected dependent stops the migration.
DROP FUNCTION payments.apply_capture_payuni_v1(uuid,bytea);
DROP FUNCTION integration.load_payment_query(uuid,bigint,bytea,text);
DROP FUNCTION integration.finish_payment_query(uuid,bigint,bytea,text,text,text);
DROP FUNCTION integration.require_payment_query(uuid,bigint,bytea,text);
DROP FUNCTION checkout.take_hosted_page(bytea,uuid,uuid,text,bytea);
DROP FUNCTION checkout.save_hosted_page(bytea,uuid,uuid,text,text,bytea,jsonb);
DROP FUNCTION checkout.load_hosted_material(bytea,uuid,uuid,text,text);
DROP FUNCTION checkout.hosted_guard(bytea,uuid,uuid,text,boolean);

-- 4) No other overload of a dropped name may survive.
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
  WHERE (n.nspname,p.proname) IN (('payments','apply_capture_payuni_v1'),('integration','load_payment_query'),
   ('integration','finish_payment_query'),('integration','require_payment_query'),('checkout','take_hosted_page'),
   ('checkout','save_hosted_page'),('checkout','load_hosted_material'),('checkout','hosted_guard'))) THEN
  RAISE EXCEPTION 'PAYUNi phase-2b drop left a legacy definer overload'; END IF;
END $$;
