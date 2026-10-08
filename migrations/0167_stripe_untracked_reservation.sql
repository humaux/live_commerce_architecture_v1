-- Purpose: allow Stripe terminal observations for legitimate A6 all-untracked orders without stock writes.
-- Depends on: 0062 apply_stripe_observation, 0109 catalog.skus.inventory_tracked, immutable order snapshot,
--   scoped reservation and RESERVE ledger facts; existing checkout-writer SELECT/RLS from 0013.
-- Used by: payments.apply_capture from the payment reconcile worker; no new callable surface or network path.
-- Invariants: nonempty reservations keep their existing sorted locks and ledger transitions; missing tracked
--   stock never qualifies for the zero-line exception, including after a tracked-to-untracked catalogue edit.
-- Only the two zero-iteration guards change. Existing owner, ACL, SECURITY DEFINER, search_path and comment stay exact.
-- A post-order untracked-to-tracked edit still fails closed: historical tracking flags are not in the order snapshot.

CREATE OR REPLACE FUNCTION payments.apply_stripe_observation(p_attempt uuid,p_report_hash bytea)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a checkout.payment_attempts%ROWTYPE; obs payments.provider_observations%ROWTYPE;
 op integration.operations%ROWTYPE; acct integration.merchant_accounts%ROWTYPE;
 sess payments.stripe_sessions%ROWTYPE; ord checkout.orders%ROWTYPE;
 res inventory.reservations%ROWTYPE; ln record; r jsonb;
 v_now timestamptz; v_money_check boolean; v_paid boolean; v_closed boolean;
 v_review boolean; v_captured boolean; v_new_capture boolean:=false;
 v_closed_before boolean; v_reference text; v_valid_close boolean;
BEGIN
 IF p_attempt IS NULL OR p_report_hash IS NULL OR octet_length(p_report_hash)<>32 THEN
  RAISE EXCEPTION 'invalid capture input' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO a FROM checkout.payment_attempts x WHERE x.id=p_attempt;
 IF NOT FOUND THEN RAISE EXCEPTION 'payment observation unavailable' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.tenant_id',a.tenant_id::text,true);
 PERFORM set_config('app.store_id',a.store_id::text,true);
 PERFORM set_config('app.buyer_id',a.owner_id::text,true);
 PERFORM set_config('app.principal_id','',true);
 SELECT x.* INTO obs FROM payments.provider_observations x
  WHERE x.tenant_id=a.tenant_id AND x.store_id=a.store_id AND x.attempt_id=a.id
   AND x.report_hash=p_report_hash AND x.source IN ('QUERY','LOCAL');
 IF NOT FOUND OR obs.execution_profile<>a.execution_profile OR obs.environment<>a.environment
  OR obs.report_hash<>sha256(convert_to(obs.report::text,'UTF8')) THEN
  RAISE EXCEPTION 'payment observation mismatch' USING ERRCODE='PT409'; END IF;
 r:=obs.report;
 SELECT x.* INTO op FROM integration.operations x WHERE x.id=a.id AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.id=a.connection_id
  AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id;
 SELECT x.* INTO sess FROM payments.stripe_sessions x WHERE x.attempt_id=a.id
  AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id;
 IF op.id IS NULL OR acct.id IS NULL OR sess.attempt_id IS NULL
  OR op.actor_kind<>'BUYER_PAYMENT_QUERY' OR op.payment_attempt_id<>a.id
  OR op.buyer_owner_id<>a.owner_id OR op.buyer_session_id<>a.session_id
  OR op.binding_id<>a.binding_id OR op.binding_version<>a.binding_version
  OR op.provider<>'stripe' OR op.action<>'stripe.checkout_session' OR op.purpose<>'transactional'
  OR op.external_asset_id<>acct.environment||':'||acct.account_id
  OR acct.provider<>'stripe' OR acct.environment<>a.environment OR acct.binding_id<>a.binding_id
  OR sess.environment<>a.environment OR sess.account_id<>acct.account_id
  OR (a.execution_profile='PROVIDER_MOCK' AND a.environment<>'SANDBOX')
  OR (a.execution_profile<>'PROVIDER_MOCK' AND a.execution_profile<>a.environment)
  OR a.method_code<>'stripe_checkout' OR r->>'Provider'<>'stripe' OR r->'Version'<>'1'::jsonb
  OR r->>'AccountID'<>sess.account_id THEN
  RAISE EXCEPTION 'Stripe payment provenance mismatch' USING ERRCODE='PT409'; END IF;
 -- §11.5: the order is the aggregate lock, followed by reservation and sorted balances.
 SELECT x.* INTO ord FROM checkout.orders x WHERE x.tenant_id=a.tenant_id AND x.store_id=a.store_id
  AND x.owner_id=a.owner_id AND x.id=a.order_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'payment order unavailable' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.buyer_session_id',ord.creator_session_id::text,true);
 SELECT x.* INTO res FROM inventory.reservations x WHERE x.tenant_id=a.tenant_id
  AND x.store_id=a.store_id AND x.id=ord.id FOR UPDATE;
 IF NOT FOUND OR res.checkout_id<>ord.id OR res.buyer_owner_id<>ord.owner_id
  OR res.buyer_session_id<>ord.creator_session_id OR a.owner_id<>ord.owner_id
  OR a.currency<>ord.currency OR a.amount_minor<>ord.total_minor THEN
  RAISE EXCEPTION 'payment aggregate mismatch' USING ERRCODE='PT409'; END IF;
 v_now:=clock_timestamp();
 -- I05: a provider identity/money conflict creates review evidence before any terminal early return.
 IF (r->>'SessionID'<>'' AND (r->>'ClientReferenceID' IS DISTINCT FROM a.id::text
  OR r->>'MetadataAttempt' IS DISTINCT FROM a.id::text
  OR r->>'MetadataProfile' IS DISTINCT FROM a.execution_profile
  OR r->>'Mode' IS DISTINCT FROM 'payment'
  OR r->'PaymentMethodTypes' IS DISTINCT FROM '["card"]'::jsonb
  OR (r->>'Livemode')::boolean IS DISTINCT FROM (a.environment='LIVE')
  OR (r->>'ExpiresAt')::bigint IS DISTINCT FROM extract(epoch FROM sess.expires_at)::bigint)) THEN
  INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'PROVIDER_IDENTITY_MISMATCH',obs.report_hash)
   ON CONFLICT DO NOTHING;
  RETURN;
 END IF;
 IF r->>'SessionID'<>'' AND r->>'SessionID' IS DISTINCT FROM sess.session_id THEN
  INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'PROVIDER_SESSION_DUPLICATE',obs.report_hash)
   ON CONFLICT DO NOTHING;
  RETURN;
 END IF;
 v_money_check:=r->>'Status' IN ('complete','expired') OR r->>'PaymentStatus'='paid';
 IF v_money_check AND
  (r->>'Currency' IS DISTINCT FROM a.currency
   OR (r->>'AmountTotal')::bigint IS DISTINCT FROM sess.unit_amount
   OR (r->>'AmountSubtotal')::bigint IS DISTINCT FROM (r->>'AmountTotal')::bigint
   OR (r->>'AmountDiscount')::bigint IS DISTINCT FROM 0
   OR (r->>'AmountTax')::bigint IS DISTINCT FROM 0
   OR (r->>'AmountShipping')::bigint IS DISTINCT FROM 0
   OR (r->>'PaymentStatus'='paid' AND
    ((r->>'PaymentIntentAmountReceived')::bigint IS DISTINCT FROM (r->>'AmountTotal')::bigint
     OR r->>'PaymentIntentCurrency' IS DISTINCT FROM r->>'Currency'))) THEN
  INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'PROVIDER_AMOUNT_MISMATCH',obs.report_hash)
   ON CONFLICT DO NOTHING;
  RETURN;
 END IF;
 IF r->>'PresentmentCurrency' NOT IN ('',r->>'Currency')
  OR r->'CurrencyConversion'='true'::jsonb
  OR (r->>'PresentmentAmount' IS NOT NULL
   AND (r->>'PresentmentAmount')::bigint<>(r->>'AmountTotal')::bigint) THEN
  INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'PROVIDER_PRESENTMENT_DRIFT',obs.report_hash)
   ON CONFLICT DO NOTHING;
 END IF;
 IF obs.source='LOCAL' AND r->>'Via'='escalate' THEN
  IF r->>'LocalReason'='EXPIRY_UNCONFIRMED' AND v_now>=sess.expires_at+interval '60 minutes'
   AND EXISTS(SELECT 1 FROM payments.provider_observations x WHERE x.attempt_id=a.id
    AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id AND x.source='QUERY'
    AND x.report->>'Status'='open'
    AND NOT EXISTS(SELECT 1 FROM payments.provider_observations newer
     WHERE newer.attempt_id=x.attempt_id AND newer.tenant_id=x.tenant_id
      AND newer.store_id=x.store_id AND newer.source='QUERY' AND newer.received_at>x.received_at)) THEN
   INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
    VALUES(a.tenant_id,a.store_id,a.id,'PROVIDER_EXPIRY_UNCONFIRMED',obs.report_hash)
    ON CONFLICT DO NOTHING;
  ELSIF r->>'LocalReason'='ASYNC_PENDING' AND EXISTS(
   SELECT 1 FROM payments.provider_observations x WHERE x.attempt_id=a.id
    AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id AND x.source='QUERY'
    AND x.report->>'Status'='complete' AND x.report->>'PaymentStatus'='unpaid'
    AND x.received_at<=v_now-interval '60 minutes') THEN
   INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
    VALUES(a.tenant_id,a.store_id,a.id,'PROVIDER_ASYNC_PENDING',obs.report_hash)
    ON CONFLICT DO NOTHING;
  END IF;
  RETURN;
 END IF;
 SELECT EXISTS(SELECT 1 FROM payments.facts f WHERE f.tenant_id=a.tenant_id
  AND f.store_id=a.store_id AND f.attempt_id=a.id AND f.kind='CLOSED_UNPAID') INTO v_closed_before;
 SELECT EXISTS(SELECT 1 FROM payments.facts f WHERE f.tenant_id=a.tenant_id
  AND f.store_id=a.store_id AND f.attempt_id=a.id AND f.kind='CAPTURED') INTO v_captured;
 v_paid:=r->>'Status'='complete' AND r->>'PaymentStatus'='paid'
  AND r->>'PaymentIntentStatus'='succeeded';
 IF r->>'Status'='complete' AND r->>'PaymentStatus'='paid' AND NOT v_paid THEN
  INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'CONFLICTING_REPORT',obs.report_hash)
   ON CONFLICT DO NOTHING;
 END IF;
 IF v_paid THEN
  INSERT INTO payments.facts(tenant_id,store_id,attempt_id,kind,amount_minor,currency,
   provider_reference,connection_id,execution_profile,environment,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'CAPTURED',a.amount_minor,a.currency,
    sess.session_id,a.connection_id,a.execution_profile,a.environment,obs.report_hash)
   ON CONFLICT(tenant_id,store_id,attempt_id,kind) DO NOTHING;
  v_new_capture:=FOUND;
  v_captured:=true;
  IF v_closed_before THEN
   INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
    VALUES(a.tenant_id,a.store_id,a.id,'CLOSURE_CONTRADICTED',obs.report_hash),
     (a.tenant_id,a.store_id,a.id,'PAID_ALLOCATION_FAILED',obs.report_hash)
    ON CONFLICT DO NOTHING;
  ELSIF v_new_capture AND (res.state<>'PAYMENT_PENDING' OR ord.commercial_state<>'AWAITING_PAYMENT'
   OR a.generation<>ord.generation OR res.generation<>ord.generation) THEN
   INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
    VALUES(a.tenant_id,a.store_id,a.id,'PAID_ALLOCATION_FAILED',obs.report_hash)
    ON CONFLICT DO NOTHING;
  END IF;
  SELECT EXISTS(SELECT 1 FROM payments.review_cases c WHERE c.tenant_id=a.tenant_id
   AND c.store_id=a.store_id AND c.attempt_id=a.id) INTO v_review;
  -- stripe-refund-v1 §4.4 (A1): the ONLY delta against 0061. A review inserted after the capture (every
  -- REFUND_% reason, REFUND_HISTORY, a later CONFLICTING_REPORT or PROVIDER_PRESENTMENT_DRIFT) never
  -- flips the order to PAID_ALLOCATION_FAILED nor rewrites the work item; it falls through to the
  -- existing NOT v_new_capture return below.
  IF v_review AND (v_new_capture OR v_closed_before) THEN
   IF v_closed_before OR (res.state<>'PAYMENT_PENDING' OR ord.commercial_state<>'AWAITING_PAYMENT') THEN
    UPDATE checkout.orders SET fulfillment_state='PAID_ALLOCATION_FAILED',updated_at=clock_timestamp()
     WHERE tenant_id=a.tenant_id AND store_id=a.store_id AND owner_id=a.owner_id AND id=ord.id;
   END IF;
   INSERT INTO fulfillment.payment_work_items(tenant_id,store_id,owner_id,order_id,attempt_id,state)
    VALUES(a.tenant_id,a.store_id,a.owner_id,ord.id,a.id,'REVIEW_REQUIRED')
    ON CONFLICT(tenant_id,store_id,order_id) DO UPDATE SET state='REVIEW_REQUIRED';
   RETURN;
  END IF;
  IF NOT v_new_capture OR EXISTS(SELECT 1 FROM fulfillment.payment_work_items w
   WHERE w.tenant_id=a.tenant_id AND w.store_id=a.store_id AND w.order_id=ord.id) THEN RETURN; END IF;
  FOR ln IN SELECT l.warehouse_id,l.sku_id,l.quantity FROM inventory.reservation_lines l
   WHERE l.tenant_id=a.tenant_id AND l.store_id=a.store_id AND l.reservation_id=ord.id
   ORDER BY l.warehouse_id,l.sku_id LOOP
   PERFORM 1 FROM inventory.lock_balance(ln.warehouse_id,ln.sku_id);
  END LOOP;
  -- A6 empty-reservation proof: current catalog flags alone cannot erase an old tracked hold.
  IF NOT FOUND THEN
   IF jsonb_typeof(ord.snapshot->'quote'->'lines') IS DISTINCT FROM 'array' THEN
    RAISE EXCEPTION 'payment reservation empty' USING ERRCODE='PT409';
   END IF;
   IF jsonb_array_length(ord.snapshot->'quote'->'lines') NOT BETWEEN 1 AND 50
    OR EXISTS (
     SELECT 1 FROM jsonb_array_elements(ord.snapshot->'quote'->'lines') q(line)
     WHERE jsonb_typeof(q.line) IS DISTINCT FROM 'object'
      OR coalesce(q.line->>'quantity','') !~ '^[1-9][0-9]{0,8}$'
      OR NOT EXISTS (SELECT 1 FROM catalog.skus s
       WHERE s.tenant_id=a.tenant_id AND s.store_id=a.store_id
        AND s.id::text=q.line->>'sku_id' AND NOT s.inventory_tracked))
    OR EXISTS (SELECT 1 FROM inventory.ledger l
     WHERE l.tenant_id=a.tenant_id AND l.store_id=a.store_id AND l.kind='RESERVE'
      AND (l.reservation_id=ord.id OR l.checkout_id=ord.id)) THEN
    RAISE EXCEPTION 'payment reservation empty' USING ERRCODE='PT409';
   END IF;
  END IF;
  -- End A6 empty-reservation proof.
  FOR ln IN SELECT l.warehouse_id,l.sku_id,l.quantity FROM inventory.reservation_lines l
   WHERE l.tenant_id=a.tenant_id AND l.store_id=a.store_id AND l.reservation_id=ord.id
   ORDER BY l.warehouse_id,l.sku_id LOOP
   INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,
    operation,command_key,reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,
    actor_kind,payment_attempt_id,payment_fact_kind)
    VALUES(a.tenant_id,a.store_id,ln.warehouse_id,ln.sku_id,'ALLOCATE',-ln.quantity,ln.quantity,
     'checkout.payment.capture',a.id::text,ord.id,NULL,ord.id,ord.owner_id,ord.creator_session_id,
     'SYSTEM_PAYMENT',a.id,'CAPTURED');
  END LOOP;
  UPDATE inventory.reservations SET state='COMMITTED' WHERE tenant_id=a.tenant_id
   AND store_id=a.store_id AND id=ord.id AND state='PAYMENT_PENDING';
  UPDATE checkout.orders SET commercial_state='CONFIRMED',updated_at=clock_timestamp()
   WHERE tenant_id=a.tenant_id AND store_id=a.store_id AND owner_id=a.owner_id AND id=ord.id
    AND commercial_state='AWAITING_PAYMENT';
  INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
   VALUES(a.tenant_id,a.store_id,a.owner_id,ord.id,ord.creator_session_id,ord.generation,
    'checkout.payment_captured','SYSTEM_PAYMENT');
  INSERT INTO fulfillment.payment_work_items(tenant_id,store_id,owner_id,order_id,attempt_id,state)
   VALUES(a.tenant_id,a.store_id,a.owner_id,ord.id,a.id,'READY');
  RETURN;
 END IF;
 v_valid_close:=NOT v_captured AND NOT v_closed_before AND NOT EXISTS(
  SELECT 1 FROM payments.review_cases c WHERE c.tenant_id=a.tenant_id AND c.store_id=a.store_id
   AND c.attempt_id=a.id AND c.reason IN ('PROVIDER_AMOUNT_MISMATCH','PROVIDER_SESSION_DUPLICATE'))
  AND NOT EXISTS(
  SELECT 1 FROM payments.facts f WHERE f.tenant_id=a.tenant_id AND f.store_id=a.store_id
   AND f.attempt_id=a.id AND f.kind='AUTHORIZED') AND (
   (r->>'Status'='expired' AND r->>'PaymentStatus'='unpaid'
    AND r->>'PaymentIntentStatus' IN ('','canceled','requires_payment_method'))
   OR (r->>'Via'='create' AND r->>'ErrorClass'='rejected'
    AND (r->>'SendCount')::integer=1 AND sess.session_id IS NULL
    -- RD4: only the very first send proves non-existence; after any resend the key may have executed.
    AND sess.create_send_count=1 AND sess.create_last_sent_at=sess.create_first_sent_at)
   OR (r->>'Via'='list' AND (r->>'ListMatchCount')::integer=0 AND sess.session_id IS NULL
    AND sess.create_first_sent_at IS NOT NULL AND v_now>=sess.expires_at+interval '15 minutes')
   OR (obs.source='LOCAL' AND r->>'Via'='unsent' AND sess.create_suppressed_at IS NOT NULL
    AND sess.create_first_sent_at IS NULL));
 IF v_valid_close THEN
  v_reference:=coalesce(sess.session_id,a.merchant_trade_no);
  INSERT INTO payments.facts(tenant_id,store_id,attempt_id,kind,amount_minor,currency,
   provider_reference,connection_id,execution_profile,environment,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'CLOSED_UNPAID',0,a.currency,v_reference,
    a.connection_id,a.execution_profile,a.environment,obs.report_hash)
   ON CONFLICT(tenant_id,store_id,attempt_id,kind) DO NOTHING;
  v_closed:=FOUND;
  IF v_closed AND res.state='PAYMENT_PENDING' AND ord.commercial_state='AWAITING_PAYMENT'
   AND a.generation=ord.generation AND res.generation=ord.generation THEN
   -- §11.5: only a CLOSED_UNPAID fact permits releasing reserved inventory.
   FOR ln IN SELECT l.warehouse_id,l.sku_id,l.quantity FROM inventory.reservation_lines l
    WHERE l.tenant_id=a.tenant_id AND l.store_id=a.store_id AND l.reservation_id=ord.id
    ORDER BY l.warehouse_id,l.sku_id LOOP
    PERFORM 1 FROM inventory.lock_balance(ln.warehouse_id,ln.sku_id);
   END LOOP;
   -- A6 empty-reservation proof: current catalog flags alone cannot erase an old tracked hold.
  IF NOT FOUND THEN
   IF jsonb_typeof(ord.snapshot->'quote'->'lines') IS DISTINCT FROM 'array' THEN
    RAISE EXCEPTION 'payment reservation empty' USING ERRCODE='PT409';
   END IF;
   IF jsonb_array_length(ord.snapshot->'quote'->'lines') NOT BETWEEN 1 AND 50
    OR EXISTS (
     SELECT 1 FROM jsonb_array_elements(ord.snapshot->'quote'->'lines') q(line)
     WHERE jsonb_typeof(q.line) IS DISTINCT FROM 'object'
      OR coalesce(q.line->>'quantity','') !~ '^[1-9][0-9]{0,8}$'
      OR NOT EXISTS (SELECT 1 FROM catalog.skus s
       WHERE s.tenant_id=a.tenant_id AND s.store_id=a.store_id
        AND s.id::text=q.line->>'sku_id' AND NOT s.inventory_tracked))
    OR EXISTS (SELECT 1 FROM inventory.ledger l
     WHERE l.tenant_id=a.tenant_id AND l.store_id=a.store_id AND l.kind='RESERVE'
      AND (l.reservation_id=ord.id OR l.checkout_id=ord.id)) THEN
    RAISE EXCEPTION 'payment reservation empty' USING ERRCODE='PT409';
   END IF;
  END IF;
  -- End A6 empty-reservation proof.
   FOR ln IN SELECT l.warehouse_id,l.sku_id,l.quantity FROM inventory.reservation_lines l
    WHERE l.tenant_id=a.tenant_id AND l.store_id=a.store_id AND l.reservation_id=ord.id
    ORDER BY l.warehouse_id,l.sku_id LOOP
    INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,
     operation,command_key,reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,
     actor_kind,payment_attempt_id,payment_fact_kind)
     VALUES(a.tenant_id,a.store_id,ln.warehouse_id,ln.sku_id,'RELEASE',-ln.quantity,0,
      'checkout.payment.close',a.id::text,ord.id,NULL,ord.id,ord.owner_id,ord.creator_session_id,
      'SYSTEM_PAYMENT',a.id,'CLOSED_UNPAID');
   END LOOP;
   UPDATE inventory.reservations SET state='RELEASED' WHERE tenant_id=a.tenant_id
    AND store_id=a.store_id AND id=ord.id AND state='PAYMENT_PENDING';
   UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED',
    updated_at=clock_timestamp() WHERE tenant_id=a.tenant_id AND store_id=a.store_id
    AND owner_id=a.owner_id AND id=ord.id AND commercial_state='AWAITING_PAYMENT';
   INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
    VALUES(a.tenant_id,a.store_id,a.owner_id,ord.id,ord.creator_session_id,ord.generation,
     'checkout.payment_closed','SYSTEM_PAYMENT');
  END IF;
 ELSIF (r->>'Status'='expired' AND (r->>'PaymentStatus'='paid'
  OR r->>'PaymentIntentStatus'='succeeded')) OR r->>'PaymentStatus'='no_payment_required' THEN
  INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'CONFLICTING_REPORT',obs.report_hash)
   ON CONFLICT DO NOTHING;
 END IF;
END $$;
COMMENT ON FUNCTION payments.apply_stripe_observation(uuid,bytea) IS 'payments owner; private Stripe observation to immutable facts and single inventory ledger path; post-capture reviews never change order or work-item state (stripe-refund-v1 §4.4); no network or webhook authority';

ALTER FUNCTION payments.apply_stripe_observation(uuid,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION payments.apply_stripe_observation(uuid,bytea) FROM PUBLIC;
-- The inherited EXECUTE ACL is owner-only (0061): no login-role GRANT is added.
