-- Purpose: 0162 (unit cancel-closes-work-item): a merchant cancel CLOSES the order's payment work item — the root-cause fix for
--   work_state=READY on CANCELLED orders. W3-08B (0155) never closed fulfillment.payment_work_items, so every cancelled captured order
--   kept its READY row and the merchant projection showed work needing a human that no longer exists (the 503 symptom the relaxed
--   Go/TS validators only papered over). Money that still needs a human after a cancel stays visible through the fact-based gap list
--   (returns.list_cancel_refund_gaps), which never reads the work item.
-- Depends on: fulfillment.payment_work_items (0018: state CHECK READY/REVIEW_REQUIRED, forced RLS private_writer on the tenant/store
--   GUCs), fulfillment.merchant_cancel_order (0155 — full current body copied below, never anchor-patched since), checkout.orders,
--   checkout.payment_attempts, payments.facts, payments.stripe_refunds, payments.refund_facts (the gap predicate mirrors
--   returns.list_cancel_refund_gaps, 0155). Forward-only; no new table, no new engine, no external system.
-- Used by: cmd/migrate (embedded, checksummed); internal/fulfillment CancelOrder (unchanged wrapper of the definer replaced here);
--   tests/foundation TestMerchantCancelClosesWorkItem (its backfill subtest re-executes this whole file, 0117 runBackfill pattern);
--   identity.read_merchant_orders / read_merchant_orders_v2 project work_state=coalesce(w.state,'NONE') from the rows closed here.
-- Invariants: I04 (the work item closes in the cancel's own transaction), I05/I13 (the cancel still never writes payments.* and never
--   starts a refund), I24 (an open cancel-refund gap keeps its READY row: money needing a human is never projected as no-work).
-- Status: REAL_PG (no external system). Contract: contracts/returns-v1.md §3 Amendment 0162.

-- ---------------------------------------------------------------------------------------------------
-- 1. The cancel definer (SECURITY DEFINER, owner commerce_checkout_writer) deletes the READY row; 0018 granted only
--    SELECT,INSERT + UPDATE(state). commerce_auth keeps SELECT-only (merchant_orders_test pins its no-write boundary), and
--    RLS (FORCE, private_writer policy keyed on the app.tenant_id/app.store_id GUCs returns.authorize sets) still scopes every
--    row to the acting store, exactly like the ledger writes in the same function.
-- ---------------------------------------------------------------------------------------------------
GRANT DELETE ON fulfillment.payment_work_items TO commerce_checkout_writer;

-- ---------------------------------------------------------------------------------------------------
-- 2. fulfillment.merchant_cancel_order: full copy of the CURRENT body (0155:664-798, never anchor-patched since; grep
--    'merchant_cancel_order' migrations/*.sql matches only 0155) with ONE addition in the CONFIRMED-card branch: after the
--    guarded DEALLOCATE loop, DELETE the order's READY payment work item in the same transaction (0162 close). The refund
--    coverage gate (409 refund_first) has already proven held >= CAPTURED at this point, so no READY work remains on the
--    order itself; if a counted in-flight refund later FAILS, the fact-based gap list is the surface for that money and the
--    work item never re-opens (returns-v1 §3 Amendment 0162 records the ruling and the rejected in-flight-keep alternative).
--    Signature, owner, ACL and every other branch are unchanged (manual_fulfilment_schema_test pins them).
-- ---------------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION fulfillment.merchant_cancel_order(p_hash bytea,p_store uuid,p_order uuid,p_key text,p_request_hash bytea,p_expected_state text,p_reason text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_auth jsonb; v_t uuid; v_p uuid; o record; r record; l record; g record; v_group uuid; v_group2 uuid; v_saved bytea; v_response jsonb;
 v_fail_code text; v_fail_msg text; v_now timestamptz; v_lines int:=0; v_capt bigint; v_held bigint; v_attempt uuid; v_alloc bigint; v_left int;
 v_grp jsonb:=NULL; v_delegate jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_reason IS NULL OR length(btrim(p_reason)) NOT BETWEEN 1 AND 240
  OR p_expected_state IS NULL OR p_expected_state NOT IN ('DRAFT','AWAITING_PAYMENT','AWAITING_TRANSFER','CONFIRMED','AWAITING_COLLECTION')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid cancel request' USING ERRCODE='PT400'; END IF;
 v_auth:=returns.authorize(p_hash,p_store,'fulfillment:write',NULL);
 v_t:=(v_auth->>'t')::uuid; v_p:=(v_auth->>'p')::uuid;
 -- parcel group first (group -> order, like the group shipment), found without a lock and re-checked under the order lock below
 SELECT m.group_id INTO v_group FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.order_id=p_order;
 IF v_group IS NOT NULL THEN
  PERFORM 1 FROM fulfillment.parcel_groups x WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=v_group FOR UPDATE; END IF;
 <<work>>
 BEGIN
  SELECT k.owner_id,k.creator_session_id,k.payment_mode,k.commercial_state,k.fulfillment_state,k.collection_state INTO o
   FROM checkout.orders k WHERE k.tenant_id=v_t AND k.store_id=p_store AND k.id=p_order FOR UPDATE;
  IF NOT FOUND THEN v_fail_code:='PT404'; v_fail_msg:='order not found'; EXIT work; END IF;
  PERFORM set_config('app.buyer_id',o.owner_id::text,true),set_config('app.buyer_session_id',o.creator_session_id::text,true);
  -- Membership changed since the unlocked read above (the order joined or left a group while this waited for the order lock): the group lock
  -- must never be taken AFTER the order lock (begin_parcel_group_shipment locks group -> order), so refuse before any write; the retry locks
  -- group -> order deterministically.
  SELECT m.group_id INTO v_group2 FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.order_id=p_order;
  IF v_group2 IS DISTINCT FROM v_group THEN v_fail_code:='PT409'; v_fail_msg:='retry_later'; EXIT work; END IF;
  SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=v_t AND c.store_id=p_store
   AND c.operation='fulfillment.merchant_cancel' AND c.idempotency_key=p_key;
  IF FOUND THEN
   IF v_saved<>p_request_hash THEN v_fail_code:='PT409'; v_fail_msg:='idempotency_conflict'; END IF;
   EXIT work;
  END IF;
  IF o.commercial_state='CANCELLED' THEN v_fail_code:='PT409'; v_fail_msg:='already_cancelled'; EXIT work; END IF;
  IF o.commercial_state<>p_expected_state THEN v_fail_code:='PT409'; v_fail_msg:='state_changed'; EXIT work; END IF;
  IF o.commercial_state='AWAITING_PAYMENT' THEN v_fail_code:='PT409'; v_fail_msg:='payment_in_flight'; EXIT work; END IF;
  IF o.fulfillment_state IN ('MERCHANT_SHIPPED','PROVIDER_LABEL_CREATED') THEN v_fail_code:='PT409'; v_fail_msg:='already_shipped'; EXIT work; END IF;
  IF o.fulfillment_state<>'MANUAL_UNASSIGNED' THEN v_fail_code:='PT422'; v_fail_msg:='not_cancellable'; EXIT work; END IF;
  v_now:=clock_timestamp();

  IF o.payment_mode IN ('pay_at_pickup','cash_on_delivery') THEN
   -- the §16.8 definer owns this path (auth, replay under its own operation, CVS attempt settle, ledger); its refusals propagate as-is
   IF o.collection_state IS DISTINCT FROM 'PENDING' THEN v_fail_code:='PT409'; v_fail_msg:='state_changed'; EXIT work; END IF;
   -- Calls inventory.release_pay_at_pickup(cancel) (taiwan-cvs-logistics-v1 §16.8); same Idempotency-Key and request hash.
   v_delegate:=inventory.release_pay_at_pickup(p_hash,p_store,p_order,p_key,p_request_hash,'cancel','PENDING');
   v_lines:=coalesce((v_delegate->>'released_lines')::int,0);
  ELSIF o.commercial_state='DRAFT' THEN
   IF EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.tenant_id=v_t AND a.store_id=p_store AND a.order_id=p_order) THEN
    v_fail_code:='PT409'; v_fail_msg:='payment_in_flight'; EXIT work; END IF;
   SELECT x.state INTO r FROM inventory.reservations x WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
   IF NOT FOUND OR r.state<>'HELD' THEN v_fail_code:='PT409'; v_fail_msg:='state_changed'; EXIT work; END IF;
   UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED',updated_at=v_now
    WHERE tenant_id=v_t AND store_id=p_store AND owner_id=o.owner_id AND id=p_order;
   UPDATE inventory.reservations SET state='RELEASED' WHERE tenant_id=v_t AND store_id=p_store AND id=p_order;
   FOR l IN SELECT x.warehouse_id,x.sku_id,x.quantity FROM inventory.reservation_lines x WHERE x.tenant_id=v_t AND x.store_id=p_store
    AND x.reservation_id=p_order ORDER BY x.warehouse_id,x.sku_id LOOP
    PERFORM 1 FROM inventory.lock_balance(l.warehouse_id,l.sku_id);
    -- Writes inventory.ledger RELEASE (checkout.merchant_cancel; provenance: inventory.guard_returns_ledger, contracts/returns-v1.md §5).
    INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,operation,command_key,reservation_id,reason,
     principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
    VALUES(v_t,p_store,l.warehouse_id,l.sku_id,'RELEASE',-l.quantity,'checkout.merchant_cancel',p_order::text,p_order,'merchant_cancel',
     v_p,p_order,o.owner_id,o.creator_session_id,'MERCHANT');
    v_lines:=v_lines+1;
   END LOOP;
  ELSIF o.payment_mode='card' AND o.commercial_state='CONFIRMED' THEN
   PERFORM fulfillment.settle_cvs_attempt(v_t,p_store,p_order);
   IF EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=v_t AND c.store_id=p_store AND c.order_id=p_order
     AND c.state IN ('REQUESTED','UNKNOWN')) THEN v_fail_code:='PT409'; v_fail_msg:='cvs_attempt_in_flight'; EXIT work; END IF;
   IF EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=v_t AND c.store_id=p_store AND c.order_id=p_order
     AND c.state NOT IN ('FAILED','ABANDONED')) THEN v_fail_code:='PT409'; v_fail_msg:='already_shipped'; EXIT work; END IF;
   IF EXISTS(SELECT 1 FROM returns.rmas m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.order_id=p_order AND m.state<>'CANCELLED') THEN
    v_fail_code:='PT409'; v_fail_msg:='has_returns'; EXIT work; END IF;
   SELECT a.id,f.amount_minor INTO v_attempt,v_capt FROM checkout.payment_attempts a JOIN payments.facts f ON f.tenant_id=a.tenant_id
    AND f.store_id=a.store_id AND f.attempt_id=a.id AND f.kind='CAPTURED' AND f.amount_minor=a.amount_minor
    WHERE a.tenant_id=v_t AND a.store_id=p_store AND a.order_id=p_order;
   IF v_attempt IS NULL THEN v_fail_code:='PT422'; v_fail_msg:='not_cancellable'; EXIT work; END IF;
   IF EXISTS(SELECT 1 FROM payments.review_cases rc WHERE rc.tenant_id=v_t AND rc.store_id=p_store AND rc.attempt_id=v_attempt
     AND rc.reason<>'PROVIDER_PRESENTMENT_DRIFT') THEN v_fail_code:='PT409'; v_fail_msg:='payment_review_open'; EXIT work; END IF;
   -- refund held = every refund that is not failed/cancelled/rejected (succeeded + in flight), the MD6 definition
   SELECT coalesce(sum(sr.amount_minor),0) INTO v_held FROM payments.stripe_refunds sr WHERE sr.tenant_id=v_t AND sr.store_id=p_store
    AND sr.attempt_id=v_attempt AND NOT EXISTS(SELECT 1 FROM payments.refund_facts rf WHERE rf.tenant_id=sr.tenant_id AND rf.store_id=sr.store_id
     AND rf.refund_id=sr.id AND rf.kind IN ('FAILED','CANCELED','REJECTED'));
   IF v_held<v_capt THEN v_fail_code:='PT409'; v_fail_msg:='refund_first'; EXIT work; END IF;
   SELECT x.state INTO r FROM inventory.reservations x WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
   IF NOT FOUND OR r.state<>'COMMITTED' THEN v_fail_code:='PT409'; v_fail_msg:='state_changed'; EXIT work; END IF;
   UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED',updated_at=v_now
    WHERE tenant_id=v_t AND store_id=p_store AND owner_id=o.owner_id AND id=p_order;
   UPDATE inventory.reservations SET state='RELEASED' WHERE tenant_id=v_t AND store_id=p_store AND id=p_order;
   FOR l IN SELECT x.warehouse_id,x.sku_id,x.quantity FROM inventory.reservation_lines x WHERE x.tenant_id=v_t AND x.store_id=p_store
    AND x.reservation_id=p_order ORDER BY x.warehouse_id,x.sku_id LOOP
    PERFORM 1 FROM inventory.lock_balance(l.warehouse_id,l.sku_id);
    SELECT coalesce(sum(a.delta_allocated),0) INTO v_alloc FROM inventory.ledger a WHERE a.tenant_id=v_t AND a.store_id=p_store
     AND a.checkout_id=p_order AND a.warehouse_id=l.warehouse_id AND a.sku_id=l.sku_id AND a.kind IN ('ALLOCATE','DEALLOCATE');
    IF v_alloc<>l.quantity THEN v_fail_code:='PT409'; v_fail_msg:='not_cancellable'; EXIT work; END IF;  -- allocation missing or already released
    -- Writes inventory.ledger DEALLOCATE (checkout.merchant_cancel; provenance: inventory.guard_returns_ledger, contracts/returns-v1.md §5).
    INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_allocated,operation,command_key,reservation_id,reason,
     principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
    VALUES(v_t,p_store,l.warehouse_id,l.sku_id,'DEALLOCATE',-l.quantity,'checkout.merchant_cancel',p_order::text,p_order,'merchant_cancel',
     v_p,p_order,o.owner_id,o.creator_session_id,'MERCHANT');
    v_lines:=v_lines+1;
   END LOOP;
   -- 0162 (returns-v1 §3 Amendment): close the order's payment work item in THIS transaction (I04). The refund-coverage gate
   -- above (409 refund_first) proved held >= CAPTURED, so no READY work remains on the order; if a counted in-flight refund
   -- later FAILS, returns.list_cancel_refund_gaps (fact-based) is the surface for that money — the work item never re-opens.
   -- Only READY: a REVIEW_REQUIRED item belongs to a PAID_ALLOCATION_FAILED order, refused 422 not_cancellable above.
   DELETE FROM fulfillment.payment_work_items w
    WHERE w.tenant_id=v_t AND w.store_id=p_store AND w.order_id=p_order AND w.state='READY';
   -- W3-07B parcel group: an OPEN group loses this order (a group needs >= 2 orders, so the last survivor dissolves it).
   IF v_group2 IS NOT NULL THEN
    -- already locked (group -> order) before the order lock; v_group2 = v_group was verified above
    SELECT x.state,x.version INTO g FROM fulfillment.parcel_groups x WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=v_group2 FOR UPDATE;
    IF g.state<>'OPEN' THEN v_fail_code:='PT409'; v_fail_msg:='already_shipped'; EXIT work; END IF;
    DELETE FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.order_id=p_order;
    SELECT count(*)::int INTO v_left FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.group_id=v_group2;
    IF v_left<=1 THEN
     DELETE FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.group_id=v_group2;
     UPDATE fulfillment.parcel_groups x SET state='DISSOLVED',version=x.version+1 WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=v_group2;
     INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_t,p_store,v_p,'fulfillment.parcel_group_dissolved');
     v_grp:=jsonb_build_object('id',v_group2,'state','DISSOLVED','version',g.version+1);
    ELSE
     UPDATE fulfillment.parcel_groups x SET version=x.version+1 WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=v_group2;
     v_grp:=jsonb_build_object('id',v_group2,'state','OPEN','version',g.version+1);
    END IF;
   END IF;
  ELSE
   v_fail_code:='PT422'; v_fail_msg:='not_cancellable'; EXIT work;  -- bank transfer (offline refund path), anything else
  END IF;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_t,p_store,v_p,'orders.merchant_cancelled');
  v_response:=jsonb_build_object('order_id',p_order,'commercial_state','CANCELLED','released_lines',v_lines,'parcel_group',v_grp,'reason',btrim(p_reason));
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(v_t,p_store,'fulfillment.merchant_cancel',p_key,p_request_hash,v_response,v_p);
 END work;
 PERFORM returns.reauthorize(p_hash,p_store,'fulfillment:write',NULL,v_auth);
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION fulfillment.merchant_cancel_order(bytea,uuid,uuid,text,bytea,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.merchant_cancel_order(bytea,uuid,uuid,text,bytea,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.merchant_cancel_order(bytea,uuid,uuid,text,bytea,text,text) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.merchant_cancel_order(bytea,uuid,uuid,text,bytea,text,text) IS 'internal/fulfillment CancelOrder only; EXECUTE commerce_runtime. fulfillment:write, Idempotency-Key, CAS on expected_state. DRAFT hold -> RELEASE rows; CONFIRMED unshipped card order -> only after refunds cover the capture (else 409 refund_first) -> DEALLOCATE of the whole allocation, leaves its OPEN parcel group; pay-at-pickup/COD delegates to inventory.release_pay_at_pickup; AWAITING_PAYMENT 409 payment_in_flight; shipped 409 already_shipped. Never starts a refund, never writes payments.*. 0162: the card branch also DELETEs the order''s READY payment work item in the same transaction (the cancel closes the work; refund coverage was gated first, and a counted in-flight refund that later FAILS is a returns.list_cancel_refund_gaps row, never a re-opened work item).';

-- ---------------------------------------------------------------------------------------------------
-- 3. Forward-only backfill (existing rows): close the READY work items the pre-0162 cancels left behind on CANCELLED orders,
--    EXCEPT rows whose money still needs a human — an outstanding cancel-refund gap (non-failed refunds below the CAPTURED
--    amount of the work item's own attempt: the same held/failed vocabulary as returns.list_cancel_refund_gaps, 0155, tied to
--    w.attempt_id). Those keep READY (I24; the relaxed Go/TS validators accept CANCELLED+READY for exactly these legacy rows).
--    REVIEW_REQUIRED rows are never touched. Count assertion: the DELETE's own row count (one data-modifying CTE statement, so
--    count and delete share one snapshot) plus a post-count that must be zero — a partial application fails the migration loudly.
--    Runs as the migration owner (RLS bypass, like every data migration here); idempotent — a re-run counts and deletes zero.
-- ---------------------------------------------------------------------------------------------------
DO $$
DECLARE v_deleted bigint; v_left bigint;
BEGIN
 WITH gone AS (
  DELETE FROM fulfillment.payment_work_items w
   USING checkout.orders o
   WHERE o.tenant_id=w.tenant_id AND o.store_id=w.store_id AND o.id=w.order_id
     AND o.commercial_state='CANCELLED' AND w.state='READY'
     AND NOT EXISTS(
      SELECT 1 FROM checkout.payment_attempts a
      JOIN payments.facts f ON f.tenant_id=a.tenant_id AND f.store_id=a.store_id AND f.attempt_id=a.id
       AND f.kind='CAPTURED' AND f.amount_minor=a.amount_minor
      CROSS JOIN LATERAL (SELECT coalesce(sum(r.amount_minor),0) AS held FROM payments.stripe_refunds r
       WHERE r.tenant_id=a.tenant_id AND r.store_id=a.store_id AND r.attempt_id=a.id
        AND NOT EXISTS(SELECT 1 FROM payments.refund_facts rf WHERE rf.tenant_id=r.tenant_id AND rf.store_id=r.store_id
         AND rf.refund_id=r.id AND rf.kind IN ('FAILED','CANCELED','REJECTED'))) h
      WHERE a.tenant_id=w.tenant_id AND a.store_id=w.store_id AND a.id=w.attempt_id AND h.held<f.amount_minor)
   RETURNING 1)
 SELECT count(*) INTO v_deleted FROM gone;
 SELECT count(*) INTO v_left FROM fulfillment.payment_work_items w
  JOIN checkout.orders o ON o.tenant_id=w.tenant_id AND o.store_id=w.store_id AND o.id=w.order_id
  WHERE o.commercial_state='CANCELLED' AND w.state='READY'
    AND NOT EXISTS(
     SELECT 1 FROM checkout.payment_attempts a
     JOIN payments.facts f ON f.tenant_id=a.tenant_id AND f.store_id=a.store_id AND f.attempt_id=a.id
      AND f.kind='CAPTURED' AND f.amount_minor=a.amount_minor
     CROSS JOIN LATERAL (SELECT coalesce(sum(r.amount_minor),0) AS held FROM payments.stripe_refunds r
      WHERE r.tenant_id=a.tenant_id AND r.store_id=a.store_id AND r.attempt_id=a.id
       AND NOT EXISTS(SELECT 1 FROM payments.refund_facts rf WHERE rf.tenant_id=r.tenant_id AND rf.store_id=r.store_id
        AND rf.refund_id=r.id AND rf.kind IN ('FAILED','CANCELED','REJECTED'))) h
     WHERE a.tenant_id=w.tenant_id AND a.store_id=w.store_id AND a.id=w.attempt_id AND h.held<f.amount_minor);
 IF v_left<>0 THEN
  RAISE EXCEPTION '0162 backfill: % cancelled-order work items without an outstanding gap are still READY', v_left;
 END IF;
 RAISE NOTICE '0162 backfill: closed % READY work item(s) of merchant-cancelled orders (open-gap rows kept)', v_deleted;
END $$;
