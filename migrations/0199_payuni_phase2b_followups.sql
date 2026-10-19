-- Purpose: PAY-RM2 phase-2b follow-ups (issue #93, review output/pay-rm2-p2b/review-1d7af85d0.md P2-1/2/3/5):
--   (1) a stricter unreconciled-PAYUNi guard: besides 0197's two clauses it also refuses while a PAYUNi reservation is
--       still PAYMENT_PENDING (an operator cancelled the order without RELEASE) or a stored PAYUNi observation was never
--       applied: the 0018 apply body would still write a fact or review case for it (review-105-336b06942 P1: a
--       report 0018 applied as a no-op is not unapplied; only the dropped apply_capture_payuni_v1 could have applied it);
--   (2) closes the window between the numbered and post-River commits: EXECUTE on checkout.start_payment and
--       integration.record_payment_query is revoked from every grantee here, wherever those functions still exist
--       (post_river/0026 then drops them); post_river/0027 re-runs this guard inside the post-River transaction;
--   (3) revokes the leftover hosted-form writer authority that no definer uses since 0197 (UNKNOWN 2): INSERT and
--       UPDATE(handed_out_at) on checkout.hosted_payment_pages with policies hosted_insert/hosted_take, and the
--       hosted_credential_read policy with its ciphertext column grant on integration.account_credentials.
-- Depends on: 0197 (guard clauses, dropped hosted definers), 0198 (form erase), 0025 (grants/policies revoked here),
--   0016/post_river/0005 (start_payment, record_payment_query grants), 0002 (inventory.reservations), 0017/0018
--   (provider_observations, facts, review_cases).
-- Used by: migrations.Apply (numbered phase); tests/foundation/payuni_phase2b_followup_test.go (PFU01-PFU04).
-- Invariants: no row is changed or deleted. hosted_read (SELECT) and the SELECT grant stay, so hosted_payment_view[_v2]
--   and merchant projections keep reading history. The refusal runs before any DDL in the one numbered transaction;
--   a refusal rolls the whole numbered batch back (operator note: docs/runbooks/deploy.md section 4).
--   No CASCADE.

-- 1) Stricter guard. row_security=off makes a role that is subject to RLS fail instead of counting zero.
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
 -- v_obs replays the decision of the dropped 0018 apply body (migrations/0018_payment_capture.sql payments.apply_capture,
 -- renamed apply_capture_payuni_v1 by 0061, body unchanged). A report counts only if 0018 would accept it (QUERY source,
 -- operation present, the report provenance clauses) AND applying it would still write a missing row: the AUTHORIZED
 -- or CAPTURED fact (CAPTURED also allocates stock and confirms the order), or the CAPTURE_EVIDENCE_INCOMPLETE,
 -- REFUND_HISTORY or CONFLICTING_REPORT review case. 0018 keys facts and review cases per attempt (ON CONFLICT DO
 -- NOTHING), so presence is checked per attempt. Reports 0018 applied without writing anything (TradeStatus 0/2/3/4
 -- before any capture, DataSource B, empty TradeNo, a repeated authorization) never count. 0018's attempt/account/
 -- binding clauses are not repeated: leaving out a refusal reason can only count more. A malformed refund amount
 -- fails the cast here as it failed 0018, so the migration fails closed.
 SELECT count(*) INTO v_obs FROM payments.provider_observations x
  JOIN checkout.payment_attempts a ON a.tenant_id=x.tenant_id AND a.store_id=x.store_id AND a.id=x.attempt_id
  JOIN integration.operations op ON op.tenant_id=a.tenant_id AND op.store_id=a.store_id AND op.id=a.id
  CROSS JOIN LATERAL (SELECT x.report AS r, a.amount_minor/100 AS amt,
   (x.report->>'TradeNo'<>'' AND x.report->>'DataSource'='A' AND x.report->>'TradeStatus'='1') AS v_auth,
   EXISTS(SELECT 1 FROM payments.facts f WHERE f.tenant_id=a.tenant_id AND f.store_id=a.store_id
    AND f.attempt_id=a.id AND f.kind='AUTHORIZED') AS has_auth,
   EXISTS(SELECT 1 FROM payments.facts f WHERE f.tenant_id=a.tenant_id AND f.store_id=a.store_id
    AND f.attempt_id=a.id AND f.kind='CAPTURED') AS has_cap,
   EXISTS(SELECT 1 FROM payments.review_cases c WHERE c.tenant_id=a.tenant_id AND c.store_id=a.store_id
    AND c.attempt_id=a.id AND c.reason='CAPTURE_EVIDENCE_INCOMPLETE') AS has_cei,
   EXISTS(SELECT 1 FROM payments.review_cases c WHERE c.tenant_id=a.tenant_id AND c.store_id=a.store_id
    AND c.attempt_id=a.id AND c.reason='REFUND_HISTORY') AS has_ref,
   EXISTS(SELECT 1 FROM payments.review_cases c WHERE c.tenant_id=a.tenant_id AND c.store_id=a.store_id
    AND c.attempt_id=a.id AND c.reason='CONFLICTING_REPORT') AS has_con) s
  CROSS JOIN LATERAL (SELECT s.v_auth AND s.r->>'CloseStatus'='2' AND s.r->'CloseAmountTWD' IS NOT NULL
   AND s.r->'CloseAmountTWD'=to_jsonb(s.amt) AS v_capture) k
  WHERE a.method_code='payuni_credit' AND x.source='QUERY'
   AND CASE WHEN (s.r->>'MerTradeNo'<>a.merchant_trade_no
     OR s.r->'AmountTWD' IS DISTINCT FROM to_jsonb(s.amt)
     OR (s.r->>'TradeNo'<>'' AND s.r->>'TradeNo' IS DISTINCT FROM op.provider_reference)
     OR s.r->>'Status'<>'SUCCESS' OR s.r->>'PaymentType'<>'1' OR s.r->>'AuthType'<>'1'
     OR s.r->'CardInst' IS DISTINCT FROM '0'::jsonb
     OR s.r->>'TradeStatus' NOT IN ('0','1','2','3','4','8','9')
     OR s.r->>'DataSource' NOT IN ('A','B')
     OR s.r->>'CloseStatus' NOT IN ('','1','2','3','7','9')) IS TRUE THEN false
    ELSE (s.v_auth IS TRUE AND NOT s.has_auth)
     OR (k.v_capture IS TRUE AND NOT s.has_cap)
     OR ((s.r->>'TradeNo'<>'' AND s.r->>'DataSource'='A' AND s.r->>'CloseStatus'='2' AND NOT k.v_capture) IS TRUE
      AND NOT s.has_cei)
     OR ((coalesce(s.r->>'CardRefundType','')<>'' OR coalesce(s.r->>'CardRefundStatus','')<>''
      OR coalesce(s.r->>'CardRefundDay','')<>'' OR coalesce((s.r->>'CardRefundAmountTWD')::bigint,0)>0
      OR (s.r ? 'CardRemainAmountTWD' AND (s.r->>'CardRemainAmountTWD')::bigint<s.amt)) IS TRUE AND NOT s.has_ref)
     OR (((s.r ? 'CardRemainAmountTWD' AND (s.r->>'CardRemainAmountTWD')::bigint>s.amt)
      OR (s.has_cap AND s.r->>'TradeNo'<>'' AND s.r->>'DataSource'='A'
       AND (s.r->>'TradeStatus' IN ('2','3','4') OR s.r->>'CloseStatus'='3'))) IS TRUE AND NOT s.has_con)
    END;
 PERFORM set_config('row_security','on',true);
 IF v_ops>0 OR v_orders>0 OR v_res>0 OR v_obs>0 THEN
  RAISE EXCEPTION 'PAYUNi phase-2b follow-up refused: % non-terminal PAYUNi operation(s), % PAYUNi order(s) awaiting payment, % PAYUNi reservation(s) still PAYMENT_PENDING, % stored PAYUNi observation(s) never applied; an operator must cancel and RELEASE the first three; the last is a provider money signal no fact or review case ever recorded and goes to the owner, never a hand-written fact or review case; data is never changed silently',
   v_ops, v_orders, v_res, v_obs USING ERRCODE='22023';
 END IF;
END $$;

-- 2) No grantee may run the two PAYUNi definers that only post_river/0026 drops.
DO $$
DECLARE f text; g text;
BEGIN
 FOREACH f IN ARRAY ARRAY['checkout.start_payment(bytea,uuid,text,bytea,uuid,text,bigint,text,uuid,bigint)',
  'integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint)'] LOOP
  CONTINUE WHEN to_regprocedure(f) IS NULL;
  FOR g IN SELECT DISTINCT CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE a.grantee::regrole::text END
   FROM pg_proc p, aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
   WHERE p.oid=to_regprocedure(f) AND a.grantee<>p.proowner LOOP
   EXECUTE format('REVOKE EXECUTE ON FUNCTION %s FROM %s',f,g);
  END LOOP;
 END LOOP;
END $$;

-- 3) Leftover hosted-form writer authority (no definer uses it since 0197).
DROP POLICY IF EXISTS hosted_insert ON checkout.hosted_payment_pages;
DROP POLICY IF EXISTS hosted_take ON checkout.hosted_payment_pages;
REVOKE INSERT ON checkout.hosted_payment_pages FROM commerce_checkout_writer;
REVOKE UPDATE(handed_out_at) ON checkout.hosted_payment_pages FROM commerce_checkout_writer;
DROP POLICY IF EXISTS hosted_credential_read ON integration.account_credentials;
REVOKE SELECT(tenant_id,store_id,connection_id,version,key_id,nonce,ciphertext)
 ON integration.account_credentials FROM commerce_checkout_writer;
