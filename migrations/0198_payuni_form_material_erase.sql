-- Purpose: PAY-RM2 phase 2c (owner decision 2026-10-19, "PAYUNi 表单清除", PR #85 UNKNOWN 3): erase, not archive,
--   the stored PAYUNi hosted-page FORM material. That material is exactly checkout.hosted_payment_pages.form, the
--   {action, fields:{MerID,Version,EncryptInfo,HashInfo}} object 0025's save_hosted_page stored (EncryptInfo/HashInfo
--   are the AES-GCM ciphertext and hash of the UPP request). It is replaced by the fixed redaction marker
--   {"erased":"0198"}, which still satisfies the 0025 NOT NULL and CHECK(object, <=32 KiB), so no constraint changes.
-- Depends on: 0025 (checkout.hosted_payment_pages), 0014 (integration.merchant_accounts.provider), 0016/0061
--   (checkout.payment_attempts.connection_id), 0197 (same unreconciled-PAYUNi guard; writers already dropped).
-- Used by: migrations.Apply (numbered phase); tests/foundation/payuni_form_erase_test.go (PFE01-PFE04).
-- Invariants: only rows whose attempt's merchant account has provider='payuni' are touched; any other provider's
--   rows (Stripe) are never read for update. Only the form column changes: attempt_id, locale, config_digest,
--   prepared_at, expires_at, handed_out_at stay, so hosted_payment_view[_v2] (which never reads form) and the
--   merchant projections keep reading historical orders. No row is deleted, nothing is archived, no CASCADE.
--   Idempotent: a second run matches zero rows. Out of scope (owner-pending): historical PAYUNi credential versions
--   in integration.account_credentials are not touched.

-- 1) Refuse while any PAYUNi money state is unreconciled (identical to 0197's guard). row_security=off makes a role
--    that is subject to RLS fail here instead of counting or erasing zero rows.
DO $$
DECLARE v_ops bigint; v_orders bigint;
BEGIN
 PERFORM set_config('row_security','off',true);
 SELECT count(*) INTO v_ops FROM integration.operations o
  WHERE o.provider='payuni' AND o.state IN ('READY','DISPATCHING','UNKNOWN','ACKNOWLEDGED');
 SELECT count(*) INTO v_orders FROM checkout.payment_attempts a
  JOIN checkout.orders o ON o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.id=a.order_id
  WHERE a.method_code='payuni_credit' AND o.commercial_state='AWAITING_PAYMENT';
 IF v_ops>0 OR v_orders>0 THEN
  RAISE EXCEPTION 'PAYUNi form erase refused: % non-terminal PAYUNi operation(s), % PAYUNi order(s) awaiting payment; an operator must reconcile them first, data is never changed silently',
   v_ops, v_orders USING ERRCODE='22023';
 END IF;

 -- 2) Erase PAYUNi form material only.
 UPDATE checkout.hosted_payment_pages h SET form='{"erased":"0198"}'::jsonb
  FROM checkout.payment_attempts a
  JOIN integration.merchant_accounts c ON c.tenant_id=a.tenant_id AND c.store_id=a.store_id AND c.id=a.connection_id
  WHERE a.id=h.attempt_id AND c.provider='payuni' AND h.form IS DISTINCT FROM '{"erased":"0198"}'::jsonb;

 -- 3) Postcondition: no PAYUNi page keeps form material.
 IF EXISTS(SELECT 1 FROM checkout.hosted_payment_pages h JOIN checkout.payment_attempts a ON a.id=h.attempt_id
  JOIN integration.merchant_accounts c ON c.tenant_id=a.tenant_id AND c.store_id=a.store_id AND c.id=a.connection_id
  WHERE c.provider='payuni' AND h.form IS DISTINCT FROM '{"erased":"0198"}'::jsonb) THEN
  RAISE EXCEPTION 'PAYUNi form erase left form material behind';
 END IF;
 PERFORM set_config('row_security','on',true);
END $$;
