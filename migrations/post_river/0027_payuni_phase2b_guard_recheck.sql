-- Purpose: PAY-RM2 phase-2b follow-up (issue #93, review P2-1): re-run the 0199 unreconciled-PAYUNi guard inside the
--   post-River transaction, so post_river/0026's drop of checkout.start_payment / integration.record_payment_query and
--   this check commit or roll back together; any PAYUNi money state created between the numbered and post-River
--   commits refuses here (22023) and the whole post-River batch, including 0026, rolls back.
-- Depends on: numbered 0199_payuni_phase2b_followups.sql (same guard text, same counts); post_river/0026.
-- Used by: migrations.Apply (post-River phase); tests/foundation/payuni_phase2b_followup_test.go (PFU02).
-- Invariants: read-only; changes no row and no object. row_security=off makes an RLS-bound migrator fail closed.

DO $$
DECLARE v_ops bigint; v_orders bigint; v_res bigint; v_obs bigint;
BEGIN
 PERFORM set_config('row_security','off',true);
 SELECT count(*) INTO v_ops FROM integration.operations o
  WHERE o.provider='payuni' AND o.state IN ('READY','DISPATCHING','UNKNOWN','ACKNOWLEDGED');
 SELECT count(*) INTO v_orders FROM checkout.payment_attempts a
  JOIN checkout.orders o ON o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.id=a.order_id
  WHERE a.method_code='payuni_credit' AND o.commercial_state='AWAITING_PAYMENT';
 SELECT count(*) INTO v_res FROM checkout.payment_attempts a
  JOIN inventory.reservations r ON r.tenant_id=a.tenant_id AND r.store_id=a.store_id AND r.id=a.order_id
  WHERE a.method_code='payuni_credit' AND r.state='PAYMENT_PENDING';
 SELECT count(*) INTO v_obs FROM payments.provider_observations x
  JOIN checkout.payment_attempts a ON a.tenant_id=x.tenant_id AND a.store_id=x.store_id AND a.id=x.attempt_id
  WHERE a.method_code='payuni_credit'
   AND NOT EXISTS(SELECT 1 FROM payments.facts f WHERE f.tenant_id=x.tenant_id AND f.store_id=x.store_id
    AND f.attempt_id=x.attempt_id AND f.source_report_hash=x.report_hash)
   AND NOT EXISTS(SELECT 1 FROM payments.review_cases c WHERE c.tenant_id=x.tenant_id AND c.store_id=x.store_id
    AND c.attempt_id=x.attempt_id AND c.source_report_hash=x.report_hash);
 PERFORM set_config('row_security','on',true);
 IF v_ops>0 OR v_orders>0 OR v_res>0 OR v_obs>0 THEN
  RAISE EXCEPTION 'PAYUNi phase-2b post-River re-check refused: % non-terminal PAYUNi operation(s), % PAYUNi order(s) awaiting payment, % PAYUNi reservation(s) still PAYMENT_PENDING, % stored PAYUNi observation(s) never applied; an operator must reconcile them first (RELEASE stock, record a review case), data is never changed silently',
   v_ops, v_orders, v_res, v_obs USING ERRCODE='22023';
 END IF;
END $$;
