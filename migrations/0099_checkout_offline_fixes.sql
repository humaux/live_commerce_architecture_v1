-- 0099 checkout-offline fixes (contracts/storefront-v2.md §C, FROZEN; integrator rulings K3-01..K3-04 of the K3 review of 0088).
--
-- Owns: (1) K3-01: payments.decide_bank_transfer takes the same key-scoped advisory lock its sibling definers use
-- (0088:348/453), so two concurrent decisions under one Idempotency-Key serialize and the second caller gets the existing
-- idempotent-replay / idempotency_conflict result instead of a raw 23505. (2) K3-02: a new p_action value
-- 'refund_offline_restock' (same 7-argument signature, same receipt operation 'checkout.bank_transfer.refund_offline' so a
-- key reused with the restock flag flipped still conflicts): while the order has not been handed to fulfilment it releases
-- the confirm allocation through the existing guarded ledger DEALLOCATE path in the SAME transaction as the refund fact,
-- audited; after handover the refund still works but restock is refused with PT422 already_shipped. (3) K3-03: the buyer
-- transfer proof is in the buyer privacy export and erasure clears proof_last5 (amount/paid_at stay as the financial
-- record with no identity). (4) K3-04: a confirm with no buyer submission stays allowed; a second audit row records it as
-- checkout.bank_transfer_confirmed_without_proof and the receipt response carries confirmed_without_proof.
--
-- Non-goals: no signature/grant changes on any replaced definer (CREATE OR REPLACE only), no edit of 0088, no restock of a
-- handed-over order (the merchant restocks through the fulfilment flow there), no new grant to commerce_privacy_writer
-- (the frozen customers-billing-v1 §3.1 list enforced by TestCustomersBillingCB02Schema), no auto-anything: every act
-- stays a merchant decision under payments:refund.
-- Callers: internal/merchantorders (DecideTransfer), internal/customers (BuyerExport/erase), tests/foundation.

-- ---------------------------------------------------------------------------------------------------
-- A. K3-01 + K3-02 + K3-04: payments.decide_bank_transfer, re-created with the identical 0088 signature/owner/grant and
-- three deltas. Body re-derived from 0088:556-647.
-- ---------------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION payments.decide_bank_transfer(p_hash bytea,p_store uuid,p_order uuid,p_key text,p_request_hash bytea,p_action text,p_reason text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; o record; t record; r record; l record; v_saved bytea; v_response jsonb; v_now timestamptz;
 v_lines integer:=0; v_op text; v_restock boolean:=false; v_no_proof boolean:=false; v_audit text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_action IS NULL
  OR p_action NOT IN ('confirm','reject','refund_offline','refund_offline_restock')
  OR (p_action='reject')<>(p_reason IS NOT NULL)
  OR (p_action='reject' AND (char_length(btrim(p_reason)) NOT BETWEEN 1 AND 200 OR p_reason ~ '[[:cntrl:]]'))
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid transfer decision' USING ERRCODE='PT400'; END IF;
 -- K3-02: the restock flag is a p_action value, not a new argument; the receipt operation stays shared so the same key
 -- with the flag flipped is an idempotency_conflict (the flag is inside the request hash).
 v_op:='checkout.bank_transfer.'||CASE WHEN p_action='refund_offline_restock' THEN 'refund_offline' ELSE p_action END;
 -- Authorize before any lock (0063 A2); tenant/store/principal GUCs come from this result.
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'payments:refund');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 -- K3-01: same key-scoped advisory lock as submit_transfer_proof (0088:453) and set_bank_transfer_settings (0088:348):
 -- a concurrent decision under the same (operation, key) waits for the first transaction instead of racing the
 -- command_results PK insert into a raw 23505; after the wait it sees the receipt and gets the replay/conflict result.
 PERFORM pg_advisory_xact_lock(hashtextextended('checkout.bank_transfer.decide|'||s.tenant_id||'|'||p_store||'|'||v_op||'|'||p_key,0));
 SELECT k.owner_id,k.creator_session_id,k.payment_mode,k.commercial_state,k.fulfillment_state,k.total_minor,k.currency,k.expires_at INTO o
  FROM checkout.orders k WHERE k.tenant_id=s.tenant_id AND k.store_id=p_store AND k.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 -- The ledger guard compares the buyer columns with these GUCs (0073:1796): they come from the locked order row.
 PERFORM set_config('app.buyer_id',o.owner_id::text,true),set_config('app.buyer_session_id',o.creator_session_id::text,true);
 SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=s.tenant_id
  AND c.store_id=p_store AND c.operation=v_op AND c.idempotency_key=p_key;
 IF FOUND THEN
  -- Idempotent replay: the same key and body returns the saved answer; another body under the key is a conflict.
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  IF o.payment_mode<>'bank_transfer' THEN RAISE EXCEPTION 'not_bank_transfer' USING ERRCODE='PT422'; END IF;
  SELECT x.state,x.confirmed_at INTO t FROM checkout.bank_transfers x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.order_id=p_order FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'not_bank_transfer' USING ERRCODE='PT422'; END IF;
  v_now:=clock_timestamp();
  IF p_action='refund_offline' OR p_action='refund_offline_restock' THEN
   -- Offline refund: the merchant returned the money outside the platform (no PSP call); only a CONFIRMED transfer, once.
   v_restock:=p_action='refund_offline_restock';
   IF t.state='REFUNDED_OFFLINE' THEN RAISE EXCEPTION 'already_refunded' USING ERRCODE='PT409'; END IF;
   IF t.state<>'CONFIRMED' OR o.commercial_state<>'CONFIRMED' THEN RAISE EXCEPTION 'transfer_not_confirmed' USING ERRCODE='PT422'; END IF;
   IF v_restock THEN
    -- K3-02: restock only while fulfilment has NOT been handed over (MANUAL_UNASSIGNED, i.e. no recorded manual shipment, and no CVS
    -- shipment attempt other than FAILED). After handover the refund stays allowed, the restock is the typed refusal.
    IF o.fulfillment_state<>'MANUAL_UNASSIGNED'
     OR EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store
       AND c.order_id=p_order AND c.state<>'FAILED') THEN
     RAISE EXCEPTION 'already_shipped' USING ERRCODE='PT422'; END IF;
   END IF;
   UPDATE checkout.bank_transfers SET state='REFUNDED_OFFLINE',refunded_at=v_now,refunded_by=s.principal_id,updated_at=v_now
    WHERE tenant_id=s.tenant_id AND store_id=p_store AND order_id=p_order;
   IF v_restock THEN
    -- The same release shape as inventory.release_pay_at_pickup (0073:1778): reservation COMMITTED -> RELEASED, then one
    -- guarded MERCHANT DEALLOCATE ledger row per line, all in THIS transaction with the refund fact above.
    SELECT x.state INTO r FROM inventory.reservations x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
    IF NOT FOUND OR r.state<>'COMMITTED' THEN RAISE EXCEPTION 'restock_unavailable' USING ERRCODE='PT409'; END IF;
    UPDATE inventory.reservations SET state='RELEASED' WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_order;
    FOR l IN SELECT x.warehouse_id,x.sku_id,x.quantity FROM inventory.reservation_lines x
     WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.reservation_id=p_order ORDER BY x.warehouse_id,x.sku_id LOOP
     PERFORM 1 FROM inventory.lock_balance(l.warehouse_id,l.sku_id);
     INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,operation,command_key,reservation_id,
      principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind,reason)
     VALUES(s.tenant_id,p_store,l.warehouse_id,l.sku_id,'DEALLOCATE',0,-l.quantity,'checkout.bank_transfer.refund_restock',p_order::text,p_order,
      s.principal_id,p_order,o.owner_id,o.creator_session_id,'MERCHANT','REFUNDED_OFFLINE');
     v_lines:=v_lines+1;
    END LOOP;
    IF v_lines=0 THEN RAISE EXCEPTION 'restock_unavailable' USING ERRCODE='PT409'; END IF;
   END IF;
   v_response:=jsonb_build_object('order_id',p_order,'state','REFUNDED_OFFLINE','commercial_state',o.commercial_state,'released_lines',v_lines,'restocked',v_restock);
  ELSE
   IF t.state IN ('CONFIRMED','REFUNDED_OFFLINE') THEN RAISE EXCEPTION 'already_confirmed' USING ERRCODE='PT409'; END IF;
   IF t.state='EXPIRED' OR o.commercial_state<>'AWAITING_TRANSFER' OR t.state NOT IN ('AWAITING','SUBMITTED','REJECTED') THEN
    RAISE EXCEPTION 'transfer_not_open' USING ERRCODE='PT409'; END IF;
   IF p_action='reject' THEN
    -- Rejects the buyer's submission, never the order: the buyer may submit again until the window ends; expiry is the only cancel.
    IF t.state<>'SUBMITTED' THEN RAISE EXCEPTION 'transfer_not_submitted' USING ERRCODE='PT409'; END IF;
    UPDATE checkout.bank_transfers SET state='REJECTED',reject_reason=btrim(p_reason),rejected_at=v_now,updated_at=v_now
     WHERE tenant_id=s.tenant_id AND store_id=p_store AND order_id=p_order;
    v_response:=jsonb_build_object('order_id',p_order,'state','REJECTED','commercial_state',o.commercial_state,'released_lines',0);
   ELSE
    -- Confirm: only inside the window (after it the expiry may already be releasing the stock; it wins the order row lock).
    IF v_now>=o.expires_at THEN RAISE EXCEPTION 'transfer_window_closed' USING ERRCODE='PT409'; END IF;
    SELECT x.state,x.checkout_id,x.buyer_owner_id,x.buyer_session_id INTO r FROM inventory.reservations x
     WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
    IF NOT FOUND OR r.state<>'HELD' OR r.checkout_id<>p_order OR r.buyer_owner_id<>o.owner_id OR r.buyer_session_id<>o.creator_session_id THEN
     RAISE EXCEPTION 'transfer_not_open' USING ERRCODE='PT409'; END IF;
    -- K3-04: legal per §C (buyers often send proof via LINE) but never silent: AWAITING at confirm = no submission ever.
    v_no_proof:=t.state='AWAITING';
    UPDATE checkout.orders SET commercial_state='CONFIRMED',updated_at=v_now
     WHERE tenant_id=s.tenant_id AND store_id=p_store AND owner_id=o.owner_id AND id=p_order AND commercial_state='AWAITING_TRANSFER';
    UPDATE checkout.bank_transfers SET state='CONFIRMED',confirmed_at=v_now,confirmed_by=s.principal_id,confirmed_amount_minor=o.total_minor,updated_at=v_now
     WHERE tenant_id=s.tenant_id AND store_id=p_store AND order_id=p_order;
    UPDATE inventory.reservations SET state='COMMITTED' WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_order AND state='HELD';
    -- §11.5 HELD -> COMMITTED: evidence = order id + transfer row CONFIRMED + merchant principal on every ALLOCATE row
    -- (guard inventory.guard_bank_transfer_ledger); balances locked in the global (warehouse, sku) order first.
    FOR l IN SELECT x.warehouse_id,x.sku_id,x.quantity FROM inventory.reservation_lines x
     WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.reservation_id=p_order ORDER BY x.warehouse_id,x.sku_id LOOP
     PERFORM 1 FROM inventory.lock_balance(l.warehouse_id,l.sku_id);
     INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,operation,command_key,reservation_id,
      principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
     VALUES(s.tenant_id,p_store,l.warehouse_id,l.sku_id,'ALLOCATE',-l.quantity,l.quantity,'checkout.bank_transfer.confirm',p_order::text,p_order,
      s.principal_id,p_order,o.owner_id,o.creator_session_id,'MERCHANT');
     v_lines:=v_lines+1;
    END LOOP;
    IF v_lines=0 THEN RAISE EXCEPTION 'transfer_not_open' USING ERRCODE='PT409'; END IF;
    v_response:=jsonb_build_object('order_id',p_order,'state','CONFIRMED','commercial_state','CONFIRMED','released_lines',v_lines,'confirmed_without_proof',v_no_proof);
   END IF;
  END IF;
  -- ops.audit_events has no details column (0001): every act keeps its one canonical row (existing readers/counts are
  -- unchanged), the K3-02 restock is its own action name, and K3-04 adds a second flag row in the same transaction.
  -- The receipt response above also records confirmed_without_proof / restocked as data. Action regex ^[a-z][a-z0-9_.:]{0,79}$.
  v_audit:=CASE p_action WHEN 'confirm' THEN 'checkout.bank_transfer_confirmed'
   WHEN 'reject' THEN 'checkout.bank_transfer_rejected'
   ELSE CASE WHEN v_restock THEN 'checkout.bank_transfer_refunded_offline_restock' ELSE 'checkout.bank_transfer_refunded_offline' END END;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
   VALUES(s.tenant_id,p_store,s.principal_id,v_audit);
  IF v_no_proof THEN
   INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
    VALUES(s.tenant_id,p_store,s.principal_id,'checkout.bank_transfer_confirmed_without_proof'); END IF;
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(s.tenant_id,p_store,v_op,p_key,p_request_hash,v_response,s.principal_id);
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'payments:refund');
 IF v_final.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text) TO commerce_runtime;
COMMENT ON FUNCTION payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text) IS 'internal/merchantorders only; EXECUTE commerce_runtime. The ONLY writer of a transfer confirmation: payments:refund, key-scoped advisory lock (K3-01), order row lock, idempotent receipt (ops.command_results), one audit row per act (checkout.bank_transfer_confirmed|_rejected|_refunded_offline|_refunded_offline_restock) plus checkout.bank_transfer_confirmed_without_proof for a confirm with no buyer submission. confirm = order CONFIRMED + reservation COMMITTED + MERCHANT ALLOCATE ledger rows (K3-04: AWAITING at confirm adds a confirmed_without_proof audit row); reject = submission REJECTED with a reason; refund_offline = REFUNDED_OFFLINE, no PSP; refund_offline_restock (K3-02) additionally releases the allocation in the same transaction while fulfilment is not handed over, else PT422 already_shipped.';

-- ---------------------------------------------------------------------------------------------------
-- B. K3-02 constraint + guard for the one new ledger row shape (MERCHANT DEALLOCATE, operation
-- 'checkout.bank_transfer.refund_restock'). Same shape-verified patch style as 0072:107 / 0088:152: the 0088 migration
-- inserted its MERCHANT ALLOCATE branch, so the 0072 MERCHANT DEALLOCATE ANY-array is extended in place. The RLS policy
-- checkout_writer_pay_at_pickup_release (0073:178) already permits any MERCHANT DEALLOCATE with matching GUCs.
-- ---------------------------------------------------------------------------------------------------
DO $$
DECLARE v_def text; v_anchor text:='''fulfillment.pay_at_pickup.cancel''::text, ''fulfillment.pay_at_pickup.restock''::text';
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c WHERE c.conrelid='inventory.ledger'::regclass AND c.conname='ledger_checkout_actor';
 IF v_def IS NULL OR v_def LIKE '%checkout.bank_transfer.refund_restock%' OR length(v_def)-length(replace(v_def,v_anchor,''))<>length(v_anchor) THEN
  RAISE EXCEPTION 'inventory.ledger ledger_checkout_actor has an unexpected shape: %',v_def; END IF;
 ALTER TABLE inventory.ledger DROP CONSTRAINT ledger_checkout_actor;
 EXECUTE format('ALTER TABLE inventory.ledger ADD CONSTRAINT ledger_checkout_actor %s',replace(v_def,v_anchor,
  v_anchor||', ''checkout.bank_transfer.refund_restock''::text'));
END $$;

-- guard_pay_at_pickup_ledger (0072:190), re-created with the identical signature/owner and one added branch: the
-- bank-transfer restock DEALLOCATE must belong to a bank_transfer order whose transfer row is REFUNDED_OFFLINE by this
-- principal, on a RELEASED reservation, for exactly the MERCHANT ALLOCATE rows of 'checkout.bank_transfer.confirm'.
CREATE OR REPLACE FUNCTION inventory.guard_pay_at_pickup_ledger() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.actor_kind='BUYER' AND NEW.kind='ALLOCATE' THEN
  IF NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id
    AND o.owner_id=NEW.buyer_owner_id AND o.id=NEW.checkout_id AND o.creator_session_id=NEW.buyer_session_id
    AND o.payment_mode='pay_at_pickup' AND o.collection_state='PENDING' AND o.commercial_state='CONFIRMED')
   OR NOT EXISTS(SELECT 1 FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id
    AND l.store_id=NEW.store_id AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id
    AND l.sku_id=NEW.sku_id AND l.quantity=-NEW.delta_reserved)
   OR EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
    AND a.order_id=NEW.checkout_id) THEN
   RAISE EXCEPTION 'pay-at-pickup ledger mismatch' USING ERRCODE='42501'; END IF;
 ELSIF NEW.actor_kind='MERCHANT' AND NEW.kind='DEALLOCATE' AND NEW.operation='checkout.bank_transfer.refund_restock' THEN
  -- 0099 K3-02: decide_bank_transfer 'refund_offline_restock' only; the transfer row was already marked
  -- REFUNDED_OFFLINE by this principal in the same transaction, and the reservation was RELEASED before the rows.
  IF NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id
    AND o.owner_id=NEW.buyer_owner_id AND o.id=NEW.checkout_id AND o.creator_session_id=NEW.buyer_session_id
    AND o.payment_mode='bank_transfer' AND o.commercial_state='CONFIRMED')
   OR NOT EXISTS(SELECT 1 FROM checkout.bank_transfers t WHERE t.tenant_id=NEW.tenant_id AND t.store_id=NEW.store_id
    AND t.order_id=NEW.checkout_id AND t.state='REFUNDED_OFFLINE' AND t.refunded_by=NEW.principal_id)
   OR NOT EXISTS(SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id
    AND r.id=NEW.reservation_id AND r.state='RELEASED')
   OR NOT EXISTS(SELECT 1 FROM inventory.ledger a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
    AND a.checkout_id=NEW.checkout_id AND a.warehouse_id=NEW.warehouse_id AND a.sku_id=NEW.sku_id
    AND a.kind='ALLOCATE' AND a.actor_kind='MERCHANT' AND a.operation='checkout.bank_transfer.confirm'
    AND a.delta_allocated=-NEW.delta_allocated)
   OR NOT EXISTS(SELECT 1 FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id
    AND l.store_id=NEW.store_id AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id
    AND l.sku_id=NEW.sku_id AND l.quantity=-NEW.delta_allocated) THEN
   RAISE EXCEPTION 'bank-transfer restock mismatch' USING ERRCODE='42501'; END IF;
 ELSIF NEW.actor_kind='MERCHANT' AND NEW.kind='DEALLOCATE' THEN
  IF NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id
    AND o.owner_id=NEW.buyer_owner_id AND o.id=NEW.checkout_id AND o.payment_mode='pay_at_pickup'
    AND ((NEW.operation='fulfillment.pay_at_pickup.cancel' AND o.collection_state='CANCELLED')
      OR (NEW.operation='fulfillment.pay_at_pickup.restock' AND o.collection_state='RESTOCKED')))
   OR NOT EXISTS(SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id
    AND r.id=NEW.reservation_id AND r.state='RELEASED')
   OR NOT EXISTS(SELECT 1 FROM inventory.ledger a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
    AND a.checkout_id=NEW.checkout_id AND a.warehouse_id=NEW.warehouse_id AND a.sku_id=NEW.sku_id
    AND a.kind='ALLOCATE' AND a.actor_kind='BUYER' AND a.delta_allocated=-NEW.delta_allocated)
   OR NOT EXISTS(SELECT 1 FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id
    AND l.store_id=NEW.store_id AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id
    AND l.sku_id=NEW.sku_id AND l.quantity=-NEW.delta_allocated) THEN
   RAISE EXCEPTION 'pay-at-pickup release mismatch' USING ERRCODE='42501'; END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inventory.guard_pay_at_pickup_ledger() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.guard_pay_at_pickup_ledger() FROM PUBLIC;
COMMENT ON FUNCTION inventory.guard_pay_at_pickup_ledger() IS 'inventory trigger guard (BEFORE INSERT on ledger): the only BUYER ALLOCATE is the pay-at-pickup commit of a pay_at_pickup order for the reserved quantity; the only MERCHANT DEALLOCATE releases a cancelled/restocked pay_at_pickup order (§16.2, §16.8) or the bank-transfer offline-refund restock of a REFUNDED_OFFLINE transfer for exactly the confirm allocation (0099 K3-02). DEFINER (commerce_checkout_writer).';

-- ---------------------------------------------------------------------------------------------------
-- C. K3-03: the buyer transfer proof in the privacy export + erasure. Same signatures/owners/grants as 0088:70 and
-- 0078:632 (+ the 0088:669 patch); commerce_privacy_writer keeps exactly its frozen §3.1 grants.
-- ---------------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION checkout.clear_buyer_email(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS integer
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 WITH cleared AS (UPDATE checkout.orders SET buyer_email=NULL
   WHERE tenant_id=p_tenant AND store_id=p_store AND owner_id=p_owner AND buyer_email IS NOT NULL RETURNING 1),
 proofs AS (
  -- storefront-v2 §C (0099 K3-03): the buyer-typed account fragment is the same PII class as the buyer email; the
  -- amount and paid_at stay as the financial record with no identity. state<>'SUBMITTED' is belt-and-braces for the
  -- bank_transfers CHECK (erasure is already blocked while a hold is open, and SUBMITTED implies an open hold).
  UPDATE checkout.bank_transfers SET proof_last5=NULL,updated_at=clock_timestamp()
   WHERE tenant_id=p_tenant AND store_id=p_store AND owner_id=p_owner AND proof_last5 IS NOT NULL AND state<>'SUBMITTED' RETURNING 1)
 SELECT count(*)::integer FROM cleared
$$;
ALTER FUNCTION checkout.clear_buyer_email(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.clear_buyer_email(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.clear_buyer_email(uuid,uuid,uuid) TO commerce_privacy_writer;
COMMENT ON FUNCTION checkout.clear_buyer_email(uuid,uuid,uuid) IS 'customers.apply_erasure only (the definer of that function, commerce_privacy_writer, holds the sole EXECUTE): sets checkout.orders.buyer_email NULL and checkout.bank_transfers.proof_last5 NULL (0099 K3-03) for one owner and returns the cleared order count. The proof amount and paid_at stay as the financial record. Non-goal: no other column, no authorization of its own (the caller holds the owner row lock and the scope).';

-- 0078:632 as patched by 0088:669 (payment_mode/buyer_email), with one added key: bank_transfer_proof. The definer is
-- owned by commerce_checkout_writer and sets the buyer GUCs, so it reads checkout.bank_transfers under RLS; no grant change.
CREATE OR REPLACE FUNCTION customers.buyer_export_orders(p_hash bytea,p_store uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_scope record; v_result jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL THEN
  RAISE EXCEPTION 'invalid export read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO v_scope FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 PERFORM set_config('app.tenant_id',v_scope.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.buyer_id',v_scope.owner_id::text,true),set_config('app.buyer_session_id',v_scope.session_id::text,true),
  set_config('app.principal_id','',true);
 SELECT coalesce(jsonb_agg(jsonb_build_object(
   'order_id',o.id,'created_at',to_char(o.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
   'commercial_state',o.commercial_state,'fulfillment_state',o.fulfillment_state,
   'currency',o.currency,'total_minor',o.total_minor,'payment_mode',o.payment_mode,'buyer_email',o.buyer_email,'snapshot',o.snapshot-'allocation',
   -- storefront-v2 §C (0099 K3-03): the buyer's own transfer proof belongs to the export; after erasure last5 is NULL
   -- (the owner is revoked then, so this shape is only observable pre-erasure or via restore).
   'bank_transfer_proof',(SELECT CASE WHEN t.proof_amount_minor IS NULL THEN NULL ELSE jsonb_build_object(
      'last5',t.proof_last5,'amount_minor',t.proof_amount_minor,
      'paid_at',to_char(t.proof_paid_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')) END
     FROM checkout.bank_transfers t WHERE t.tenant_id=o.tenant_id AND t.store_id=o.store_id AND t.owner_id=o.owner_id AND t.order_id=o.id),
   'shipment',(SELECT jsonb_build_object('status',v.status,'carrier_code',v.carrier_code,'carrier_name',v.carrier_name,
      'tracking_number',v.tracking_number,'tracking_url',v.tracking_url,
      'recorded_at',to_char(v.recorded_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'))
     FROM fulfillment.manual_shipment_heads h JOIN fulfillment.manual_shipment_versions v
      ON v.tenant_id=h.tenant_id AND v.store_id=h.store_id AND v.order_id=h.order_id AND v.version=h.current_version
     WHERE h.tenant_id=o.tenant_id AND h.store_id=o.store_id AND h.owner_id=o.owner_id AND h.order_id=o.id AND v.status='SHIPPED'))
   ORDER BY o.created_at DESC,o.id DESC),'[]'::jsonb) INTO v_result
  FROM (SELECT x.* FROM checkout.orders x WHERE x.tenant_id=v_scope.tenant_id AND x.store_id=p_store
    AND x.owner_id=v_scope.owner_id ORDER BY x.created_at DESC,x.id DESC LIMIT 201) o;
 RETURN v_result;
END $$;
ALTER FUNCTION customers.buyer_export_orders(bytea,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION customers.buyer_export_orders(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION customers.buyer_export_orders(bytea,uuid) TO commerce_buyer_runtime;
COMMENT ON FUNCTION customers.buyer_export_orders(bytea,uuid) IS
 'internal/customers.BuyerExport only; owner commerce_checkout_writer, EXECUTE commerce_buyer_runtime. The caller''s own orders newest first (<= 201, Go refuses > 200): snapshot without allocation plus the SHIPPED head, the payment mode, the buyer email and the bank-transfer proof (0099 K3-03). Not in the frozen §3.1 table: the buyer pool cannot read checkout tables.';
