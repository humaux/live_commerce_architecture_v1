-- Purpose: native merchant cancellation of sessionless OA1 pickup/COD and unpaid bank orders.
-- Depends on: 0171 creator XOR/FKs, 0184 native stock, 0187 order credentials and legacy 0162 cancellation.
-- Used by: unchanged fulfillment.merchant_cancel_order and inventory.release_pay_at_pickup entries.
-- Invariants: parcel/order advisory before order/stock; exact creator provenance; no late attempt,
-- scope or owner lock; no renewed hold/credential. READY/operation pairing and C3 stay OA2.

CREATE POLICY order_attempt_reservation_release ON inventory.reservations
 FOR UPDATE TO commerce_checkout_writer
 USING(creator_kind='order_attempt' AND buyer_session_id IS NULL AND state IN ('HELD','COMMITTED')
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND id=create_attempt_id AND checkout_id=id AND generation=1)
 WITH CHECK(creator_kind='order_attempt' AND buyer_session_id IS NULL AND state='RELEASED'
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND id=create_attempt_id AND checkout_id=id AND generation=1
  AND EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=reservations.tenant_id AND o.store_id=reservations.store_id
   AND o.id=reservations.id AND o.owner_id=reservations.buyer_owner_id AND o.creator_kind='order_attempt'
   AND o.create_attempt_id=reservations.create_attempt_id AND o.creator_session_id IS NULL
   AND (o.commercial_state='CANCELLED' OR o.collection_state='RESTOCKED')));

CREATE POLICY order_attempt_merchant_release_ledger ON inventory.ledger
 FOR INSERT TO commerce_checkout_writer
 WITH CHECK(actor_kind='MERCHANT' AND creator_kind='order_attempt' AND buyer_session_id IS NULL
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND checkout_id=create_attempt_id AND reservation_id=checkout_id AND command_key=checkout_id::text
  AND payment_attempt_id IS NULL AND payment_fact_kind IS NULL
  AND delta_on_hand=0 AND delta_unavailable=0
  AND ((kind='RELEASE' AND operation='checkout.merchant_cancel' AND delta_reserved<0 AND delta_allocated=0)
   OR (kind='DEALLOCATE' AND operation IN ('fulfillment.pay_at_pickup.cancel','fulfillment.pay_at_pickup.restock')
    AND delta_allocated<0 AND delta_reserved=0)));

-- Revocation is one metadata column under the already-held order lock. This
-- trigger also covers the independent native pickup release entrance and expiry.
-- No credential version, secret, original issue time or deadline is rewritten.
GRANT UPDATE(revoked_at) ON checkout.order_pay_entries TO commerce_checkout_writer;
CREATE POLICY order_pay_entries_cancel_revoke ON checkout.order_pay_entries
 FOR UPDATE TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid)
 WITH CHECK(revoked_at IS NOT NULL AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=order_pay_entries.tenant_id AND o.store_id=order_pay_entries.store_id
   AND o.id=order_pay_entries.order_id AND o.owner_id=order_pay_entries.owner_id AND o.creator_kind='order_attempt'
   AND o.create_attempt_id=order_pay_entries.create_attempt_id AND o.creator_session_id IS NULL AND o.commercial_state='CANCELLED'));

CREATE FUNCTION checkout.revoke_cancelled_order_attempt_pay_entry() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_buyer text:=current_setting('app.buyer_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 PERFORM set_config('app.tenant_id',NEW.tenant_id::text,true),set_config('app.store_id',NEW.store_id::text,true),
  set_config('app.buyer_id',NEW.owner_id::text,true),set_config('app.order_attempt_id',NEW.create_attempt_id::text,true);
 UPDATE checkout.order_pay_entries SET revoked_at=clock_timestamp()
  WHERE tenant_id=NEW.tenant_id AND store_id=NEW.store_id AND owner_id=NEW.owner_id
   AND order_id=NEW.id AND create_attempt_id=NEW.create_attempt_id AND revoked_at IS NULL;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.buyer_id',coalesce(prior_buyer,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RETURN NEW;
END $$;
ALTER FUNCTION checkout.revoke_cancelled_order_attempt_pay_entry() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.revoke_cancelled_order_attempt_pay_entry() FROM PUBLIC;
COMMENT ON FUNCTION checkout.revoke_cancelled_order_attempt_pay_entry() IS
 'Trigger-only native cancellation credential invalidation. No caller EXECUTE, new public route, rotation, earlier locks or READY/C3 bridge.';
CREATE TRIGGER revoke_cancelled_order_attempt_pay_entry AFTER UPDATE OF commercial_state ON checkout.orders
 FOR EACH ROW WHEN (NEW.creator_kind='order_attempt' AND NEW.commercial_state='CANCELLED' AND OLD.commercial_state IS DISTINCT FROM NEW.commercial_state)
 EXECUTE FUNCTION checkout.revoke_cancelled_order_attempt_pay_entry();

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
 PERFORM pg_advisory_xact_lock(hashtextextended('checkout.order|'||v_t::text||'|'||p_store::text||'|'||p_order::text,0));
 <<work>>
 BEGIN
  SELECT k.owner_id,k.creator_session_id,k.payment_mode,k.commercial_state,k.fulfillment_state,k.collection_state,k.creator_kind,k.create_attempt_id INTO o
   FROM checkout.orders k WHERE k.tenant_id=v_t AND k.store_id=p_store AND k.id=p_order FOR UPDATE;
  IF NOT FOUND THEN v_fail_code:='PT404'; v_fail_msg:='order not found'; EXIT work; END IF;
  PERFORM set_config('app.buyer_id',o.owner_id::text,true),set_config('app.buyer_session_id',coalesce(o.creator_session_id::text,''),true),
   set_config('app.order_attempt_id',coalesce(o.create_attempt_id::text,''),true);
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
  ELSIF o.commercial_state='DRAFT' OR (o.creator_kind='order_attempt' AND o.create_attempt_id=p_order
    AND o.creator_session_id IS NULL AND o.payment_mode='bank_transfer' AND o.commercial_state='AWAITING_TRANSFER') THEN
   IF o.payment_mode='bank_transfer' AND NOT EXISTS(SELECT 1 FROM checkout.bank_transfers t
     WHERE t.tenant_id=v_t AND t.store_id=p_store AND t.order_id=p_order AND t.owner_id=o.owner_id
      AND t.state IN ('AWAITING','REJECTED') AND t.confirmed_at IS NULL AND t.confirmed_by IS NULL
      AND t.confirmed_amount_minor IS NULL AND t.refunded_at IS NULL) THEN
    v_fail_code:='PT409'; v_fail_msg:='payment_in_flight'; EXIT work; END IF;
   IF EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.tenant_id=v_t AND a.store_id=p_store AND a.order_id=p_order) THEN
    v_fail_code:='PT409'; v_fail_msg:='payment_in_flight'; EXIT work; END IF;
   SELECT x.* INTO r FROM inventory.reservations x WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
   IF NOT FOUND OR r.creator_kind IS DISTINCT FROM o.creator_kind OR r.create_attempt_id IS DISTINCT FROM o.create_attempt_id
    OR r.buyer_session_id IS DISTINCT FROM o.creator_session_id OR r.buyer_owner_id IS DISTINCT FROM o.owner_id
    OR r.checkout_id IS DISTINCT FROM p_order OR r.state<>'HELD' THEN v_fail_code:='PT409'; v_fail_msg:='state_changed'; EXIT work; END IF;
   UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED',updated_at=v_now
    WHERE tenant_id=v_t AND store_id=p_store AND owner_id=o.owner_id AND id=p_order;
   UPDATE inventory.reservations SET state='RELEASED' WHERE tenant_id=v_t AND store_id=p_store AND id=p_order;
   IF o.creator_kind='order_attempt' AND o.payment_mode='bank_transfer' THEN
    UPDATE checkout.bank_transfers SET state='EXPIRED',updated_at=v_now WHERE tenant_id=v_t AND store_id=p_store
     AND order_id=p_order AND state IN ('AWAITING','REJECTED');
   END IF;
   FOR l IN SELECT x.warehouse_id,x.sku_id,x.quantity FROM inventory.reservation_lines x WHERE x.tenant_id=v_t AND x.store_id=p_store
    AND x.reservation_id=p_order ORDER BY x.warehouse_id,x.sku_id LOOP
    PERFORM 1 FROM inventory.lock_balance(l.warehouse_id,l.sku_id);
    -- Writes inventory.ledger RELEASE (checkout.merchant_cancel; provenance: inventory.guard_returns_ledger, contracts/returns-v1.md §5).
    INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,operation,command_key,reservation_id,reason,
     principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind,creator_kind,create_attempt_id)
    VALUES(v_t,p_store,l.warehouse_id,l.sku_id,'RELEASE',-l.quantity,'checkout.merchant_cancel',p_order::text,p_order,'merchant_cancel',
     v_p,p_order,o.owner_id,o.creator_session_id,'MERCHANT',o.creator_kind,o.create_attempt_id);
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

-- Retains 0107 and the 0083 restock guards; adds the same order advisory and exact creator propagation.
CREATE OR REPLACE FUNCTION inventory.release_pay_at_pickup(p_hash bytea,p_store uuid,p_order uuid,p_key text,p_request_hash bytea,
 p_action text,p_expected_state text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; o record; r record; l record; v_saved bytea; v_response jsonb; v_op text; v_to text; v_lines integer:=0;
 v_now timestamptz;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_key IS NULL
  OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$' OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32
  OR p_action IS NULL OR p_action NOT IN ('cancel','restock')
  OR p_expected_state IS NULL OR p_expected_state NOT IN ('PENDING','RETURNED')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid release request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 PERFORM pg_advisory_xact_lock(hashtextextended('checkout.order|'||s.tenant_id::text||'|'||p_store::text||'|'||p_order::text,0));
 SELECT k.owner_id,k.creator_session_id,k.payment_mode,k.commercial_state,k.fulfillment_state,k.collection_state,k.creator_kind,k.create_attempt_id INTO o
  FROM checkout.orders k WHERE k.tenant_id=s.tenant_id AND k.store_id=p_store AND k.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 PERFORM set_config('app.buyer_id',o.owner_id::text,true),set_config('app.buyer_session_id',coalesce(o.creator_session_id::text,''),true),
  set_config('app.order_attempt_id',coalesce(o.create_attempt_id::text,''),true);
 SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=s.tenant_id
  AND c.store_id=p_store AND c.operation='inventory.pay_at_pickup.release' AND c.idempotency_key=p_key;
 IF FOUND THEN
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  IF o.payment_mode NOT IN ('pay_at_pickup','cash_on_delivery') THEN RAISE EXCEPTION 'not_pay_at_pickup' USING ERRCODE='PT422'; END IF;
  IF p_action='cancel' AND p_expected_state<>'PENDING' OR p_action='restock' AND p_expected_state<>'RETURNED' THEN
   RAISE EXCEPTION 'invalid release request' USING ERRCODE='PT400'; END IF;
  IF o.collection_state IS DISTINCT FROM p_expected_state THEN
   RAISE EXCEPTION 'collection_state_changed' USING ERRCODE='PT409'; END IF;
  PERFORM fulfillment.settle_cvs_attempt(s.tenant_id,p_store,p_order);
  IF p_action='cancel' THEN
   IF o.commercial_state NOT IN ('CONFIRMED','AWAITING_COLLECTION') OR o.fulfillment_state<>'MANUAL_UNASSIGNED' THEN
    RAISE EXCEPTION 'not_cancellable' USING ERRCODE='PT422'; END IF;
   IF EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store AND c.order_id=p_order
     AND c.state IN ('REQUESTED','UNKNOWN')) THEN RAISE EXCEPTION 'cvs_attempt_in_flight' USING ERRCODE='PT409'; END IF;
   IF EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store AND c.order_id=p_order
     AND c.state NOT IN ('FAILED','ABANDONED')) THEN RAISE EXCEPTION 'not_cancellable' USING ERRCODE='PT422'; END IF;
   v_op:='fulfillment.pay_at_pickup.cancel'; v_to:='CANCELLED';
  ELSE
   IF NOT fulfillment.cvs_parcel_returned(s.tenant_id,p_store,p_order) THEN
    RAISE EXCEPTION 'parcel_not_returned' USING ERRCODE='PT409'; END IF;
   v_op:='fulfillment.pay_at_pickup.restock'; v_to:='RESTOCKED';
  END IF;
  v_now:=clock_timestamp();
  IF p_action='cancel' THEN
   UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED',collection_state='CANCELLED',updated_at=v_now
    WHERE tenant_id=s.tenant_id AND store_id=p_store AND owner_id=o.owner_id AND id=p_order;
  ELSE
   UPDATE checkout.orders SET collection_state='RESTOCKED',updated_at=v_now
    WHERE tenant_id=s.tenant_id AND store_id=p_store AND owner_id=o.owner_id AND id=p_order;
  END IF;
  SELECT x.* INTO r FROM inventory.reservations x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
  IF NOT FOUND OR r.creator_kind IS DISTINCT FROM o.creator_kind OR r.create_attempt_id IS DISTINCT FROM o.create_attempt_id
   OR r.buyer_session_id IS DISTINCT FROM o.creator_session_id OR r.buyer_owner_id IS DISTINCT FROM o.owner_id
   OR r.checkout_id IS DISTINCT FROM p_order OR r.state<>'COMMITTED' THEN RAISE EXCEPTION 'collection_state_changed' USING ERRCODE='PT409'; END IF;
  UPDATE inventory.reservations SET state='RELEASED' WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_order;
  FOR l IN SELECT x.warehouse_id,x.sku_id,x.quantity FROM inventory.reservation_lines x
   WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.reservation_id=p_order ORDER BY x.warehouse_id,x.sku_id LOOP
   PERFORM 1 FROM inventory.lock_balance(l.warehouse_id,l.sku_id);
   INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_allocated,operation,command_key,reservation_id,
    reason,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind,creator_kind,create_attempt_id)
   VALUES(s.tenant_id,p_store,l.warehouse_id,l.sku_id,'DEALLOCATE',-l.quantity,v_op,p_order::text,p_order,p_expected_state,
    s.principal_id,p_order,o.owner_id,o.creator_session_id,'MERCHANT',o.creator_kind,o.create_attempt_id);
   v_lines:=v_lines+1;
  END LOOP;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
   VALUES(s.tenant_id,p_store,s.principal_id,CASE p_action WHEN 'cancel' THEN 'fulfillment.pay_at_pickup_cancelled' ELSE 'fulfillment.pay_at_pickup_restocked' END);
  v_response:=jsonb_build_object('order_id',p_order,'collection_state',v_to,
   'commercial_state',CASE WHEN p_action='cancel' THEN 'CANCELLED' ELSE o.commercial_state END,'released_lines',v_lines);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(s.tenant_id,p_store,'inventory.pay_at_pickup.release',p_key,p_request_hash,v_response,s.principal_id);
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF v_final.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text) TO commerce_runtime;

-- Retains the 0155 operation restriction, plus a separate exact attempt allocation branch.
CREATE OR REPLACE FUNCTION inventory.guard_pay_at_pickup_ledger() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.actor_kind='BUYER' AND NEW.kind='ALLOCATE' THEN
  IF NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id
    AND o.owner_id=NEW.buyer_owner_id AND o.id=NEW.checkout_id AND o.creator_session_id=NEW.buyer_session_id
    AND o.payment_mode IN ('pay_at_pickup','cash_on_delivery') AND o.collection_state='PENDING'
    AND o.commercial_state IN ('CONFIRMED','AWAITING_COLLECTION'))
   OR NOT EXISTS(SELECT 1 FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id
    AND l.store_id=NEW.store_id AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id
    AND l.sku_id=NEW.sku_id AND l.quantity=-NEW.delta_reserved)
   OR EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
    AND a.order_id=NEW.checkout_id) THEN
   RAISE EXCEPTION 'pay-at-pickup ledger mismatch' USING ERRCODE='42501'; END IF;
 ELSIF NEW.actor_kind='MERCHANT' AND NEW.kind='DEALLOCATE' AND NEW.operation='checkout.bank_transfer.refund_restock' THEN
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
 ELSIF NEW.actor_kind='MERCHANT' AND NEW.kind='DEALLOCATE' AND NEW.operation IN ('fulfillment.pay_at_pickup.cancel','fulfillment.pay_at_pickup.restock') THEN
  IF NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id
    AND o.owner_id=NEW.buyer_owner_id AND o.id=NEW.checkout_id AND o.payment_mode IN ('pay_at_pickup','cash_on_delivery')
    AND o.creator_kind=NEW.creator_kind AND o.create_attempt_id IS NOT DISTINCT FROM NEW.create_attempt_id
    AND o.creator_session_id IS NOT DISTINCT FROM NEW.buyer_session_id
    AND ((NEW.operation='fulfillment.pay_at_pickup.cancel' AND o.collection_state='CANCELLED')
      OR (NEW.operation='fulfillment.pay_at_pickup.restock' AND o.collection_state='RESTOCKED')))
   OR NOT EXISTS(SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id
    AND r.id=NEW.reservation_id AND r.state='RELEASED')
   OR NOT EXISTS(SELECT 1 FROM inventory.ledger a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
    AND a.checkout_id=NEW.checkout_id AND a.warehouse_id=NEW.warehouse_id AND a.sku_id=NEW.sku_id
    AND a.kind='ALLOCATE' AND a.delta_allocated=-NEW.delta_allocated
    AND a.buyer_owner_id=NEW.buyer_owner_id AND a.reservation_id=NEW.reservation_id
    AND a.creator_kind=NEW.creator_kind AND a.create_attempt_id IS NOT DISTINCT FROM NEW.create_attempt_id
    AND a.buyer_session_id IS NOT DISTINCT FROM NEW.buyer_session_id
    AND ((NEW.creator_kind='storefront' AND NEW.create_attempt_id IS NULL AND NEW.buyer_session_id IS NOT NULL AND a.actor_kind='BUYER')
     OR (NEW.creator_kind='order_attempt' AND NEW.create_attempt_id=NEW.checkout_id AND NEW.buyer_session_id IS NULL
      AND a.actor_kind='ORDER_ATTEMPT' AND a.operation='checkout.pay_at_pickup.commit' AND a.command_key=NEW.checkout_id::text)))
   OR NOT EXISTS(SELECT 1 FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id
    AND l.store_id=NEW.store_id AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id
    AND l.sku_id=NEW.sku_id AND l.quantity=-NEW.delta_allocated) THEN
   RAISE EXCEPTION 'pay-at-pickup release mismatch' USING ERRCODE='42501'; END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inventory.guard_pay_at_pickup_ledger() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.guard_pay_at_pickup_ledger() FROM PUBLIC;

-- Retains the shared conservation/refund invariant and legacy session branch.
CREATE OR REPLACE FUNCTION inventory.guard_returns_ledger() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_line bigint; v_alloc bigint;
BEGIN
 -- THE invariant (every writer: RMA restock, cancel release, bank-transfer refund restock, pay-at-pickup/COD cancel and restock): a DEALLOCATE
 -- never exceeds what the ledger still holds allocated for that order line (sum ALLOCATE + DEALLOCATE of that checkout/warehouse/sku). A second
 -- release of the same units (e.g. after a shipment void) therefore cannot create phantom stock even when other orders hold the SKU.
 IF NEW.kind='DEALLOCATE' THEN
  SELECT coalesce(sum(a.delta_allocated),0) INTO v_alloc FROM inventory.ledger a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
   AND a.checkout_id=NEW.checkout_id AND a.warehouse_id=NEW.warehouse_id AND a.sku_id=NEW.sku_id AND a.kind IN ('ALLOCATE','DEALLOCATE');
  IF NEW.checkout_id IS NULL OR NEW.delta_allocated>=0 OR v_alloc<-NEW.delta_allocated THEN
   RAISE EXCEPTION 'deallocate exceeds the remaining allocation of the order line' USING ERRCODE='42501'; END IF;
 END IF;
 IF NEW.operation NOT IN ('returns.rma.restock','checkout.merchant_cancel') THEN RETURN NEW; END IF;
 IF NEW.actor_kind<>'MERCHANT' OR NEW.checkout_id IS NULL OR NEW.reservation_id IS DISTINCT FROM NEW.checkout_id
  OR NEW.principal_id IS NULL OR NEW.kind NOT IN ('DEALLOCATE','RELEASE') THEN
  RAISE EXCEPTION 'returns ledger mismatch' USING ERRCODE='42501'; END IF;
 -- the order lines of THIS reservation: quantity of the line and what the ledger still holds allocated for it
 SELECT l.quantity INTO v_line FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id AND l.store_id=NEW.store_id
  AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id AND l.sku_id=NEW.sku_id;
 IF NEW.operation='returns.rma.restock' THEN
  IF NEW.kind<>'DEALLOCATE' OR NEW.reason<>'rma_restock' OR NEW.delta_allocated>=0
   OR NEW.command_key !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
   RAISE EXCEPTION 'rma restock ledger mismatch' USING ERRCODE='42501'; END IF;
  -- CLOSED RMA of this order whose line says exactly this quantity is sellable, closed by this principal; order returnable (CONFIRMED, or a
  -- collected COD order) with a COMMITTED reservation (a cancelled/released order has nothing allocated to give back); the remaining
  -- allocation bound is the generic check above.
  IF NOT EXISTS(SELECT 1 FROM returns.rmas r
    JOIN returns.rma_lines l ON l.tenant_id=r.tenant_id AND l.store_id=r.store_id AND l.rma_id=r.id
    JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.owner_id=r.owner_id AND o.id=r.order_id
    JOIN inventory.reservations v ON v.tenant_id=o.tenant_id AND v.store_id=o.store_id AND v.id=o.id
    WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id AND r.id=NEW.command_key::uuid AND r.order_id=NEW.checkout_id
     AND r.state='CLOSED' AND r.closed_by=NEW.principal_id AND l.warehouse_id=NEW.warehouse_id AND l.sku_id=NEW.sku_id
     AND l.qty_restock=-NEW.delta_allocated AND returns.order_returnable(o.commercial_state,o.payment_mode,o.collection_state)
     AND o.owner_id=NEW.buyer_owner_id AND o.creator_session_id=NEW.buyer_session_id AND v.state='COMMITTED') THEN
   RAISE EXCEPTION 'rma restock ledger mismatch' USING ERRCODE='42501'; END IF;
  RETURN NEW;
 END IF;
 -- checkout.merchant_cancel: the order is already CANCELLED/CANCELLED and the reservation RELEASED (the definer wrote both first), for the
 -- exact line quantity, command_key = order id.
 IF NEW.reason<>'merchant_cancel' OR NEW.command_key<>NEW.checkout_id::text OR NOT EXISTS(SELECT 1 FROM checkout.orders o
   JOIN inventory.reservations v ON v.tenant_id=o.tenant_id AND v.store_id=o.store_id AND v.id=o.id
   WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id AND o.id=NEW.checkout_id AND o.owner_id=NEW.buyer_owner_id
    AND ((NEW.creator_kind='storefront' AND NEW.create_attempt_id IS NULL AND NEW.buyer_session_id IS NOT NULL
      AND o.creator_kind='storefront' AND o.create_attempt_id IS NULL AND o.creator_session_id=NEW.buyer_session_id)
     OR (NEW.creator_kind='order_attempt' AND NEW.create_attempt_id=NEW.checkout_id AND NEW.buyer_session_id IS NULL
      AND o.creator_kind='order_attempt' AND o.create_attempt_id=NEW.create_attempt_id AND o.creator_session_id IS NULL
      AND NEW.kind='RELEASE' AND o.payment_mode='bank_transfer'
      AND EXISTS(SELECT 1 FROM checkout.bank_transfers t WHERE t.tenant_id=o.tenant_id AND t.store_id=o.store_id
       AND t.order_id=o.id AND t.state='EXPIRED' AND t.confirmed_at IS NULL AND t.confirmed_by IS NULL AND t.confirmed_amount_minor IS NULL)))
    AND v.creator_kind=NEW.creator_kind AND v.create_attempt_id IS NOT DISTINCT FROM NEW.create_attempt_id
    AND v.buyer_owner_id=NEW.buyer_owner_id AND v.buyer_session_id IS NOT DISTINCT FROM NEW.buyer_session_id
    AND o.commercial_state='CANCELLED' AND o.fulfillment_state='CANCELLED'
    AND v.state='RELEASED') THEN
  RAISE EXCEPTION 'merchant cancel ledger mismatch' USING ERRCODE='42501'; END IF;
 IF NEW.kind='RELEASE' THEN
  -- unpaid hold: reserved goes down by exactly the line quantity; the order never had a payment attempt
  IF NEW.delta_reserved>=0 OR v_line IS DISTINCT FROM -NEW.delta_reserved OR EXISTS(SELECT 1 FROM checkout.payment_attempts a
    WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id AND a.order_id=NEW.checkout_id) THEN
   RAISE EXCEPTION 'merchant cancel ledger mismatch' USING ERRCODE='42501'; END IF;
  RETURN NEW;
 END IF;
 -- paid card order: the whole remaining allocation of the line goes back, only after a CAPTURED fact and refunds (succeeded + in flight)
 -- covering the whole capture (stripe-refund-v1 RD6: the refund itself never released stock; this cancel does).
 IF NEW.delta_allocated>=0 OR v_line IS DISTINCT FROM -NEW.delta_allocated OR v_alloc IS DISTINCT FROM v_line
  OR NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id AND o.id=NEW.checkout_id
    AND o.payment_mode='card')
  OR NOT EXISTS(SELECT 1 FROM checkout.payment_attempts a JOIN payments.facts f ON f.tenant_id=a.tenant_id AND f.store_id=a.store_id
    AND f.attempt_id=a.id AND f.kind='CAPTURED' AND f.amount_minor=a.amount_minor
    WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id AND a.order_id=NEW.checkout_id
     AND (SELECT coalesce(sum(r.amount_minor),0) FROM payments.stripe_refunds r WHERE r.tenant_id=a.tenant_id AND r.store_id=a.store_id
       AND r.attempt_id=a.id AND NOT EXISTS(SELECT 1 FROM payments.refund_facts rf WHERE rf.tenant_id=r.tenant_id AND rf.store_id=r.store_id
        AND rf.refund_id=r.id AND rf.kind IN ('FAILED','CANCELED','REJECTED')))>=f.amount_minor) THEN
  RAISE EXCEPTION 'merchant cancel ledger mismatch' USING ERRCODE='42501'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inventory.guard_returns_ledger() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.guard_returns_ledger() FROM PUBLIC;

