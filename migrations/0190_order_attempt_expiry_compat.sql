-- Purpose: expire native bank-transfer attempt holds through the existing expiry worker.
-- Depends on: native checkout/inventory guards, 0171 creator provenance and 0184 placement.
-- Used by: commerce_expiry_worker through the unchanged checkout.expire_held ABI.
-- Invariants: I01/I03/I04/I08/I14; order advisory precedes order/reservation/stock locks,
-- exact native creator survives, stock releases once, terminal attempt receipts stay immutable.

-- The old session policy cannot authorize a sessionless attempt. These branches
-- are confined to the private checkout writer and the exact due native bank hold.
CREATE POLICY order_attempt_reservation_expire ON inventory.reservations
 FOR UPDATE TO commerce_checkout_writer
 USING(creator_kind='order_attempt' AND buyer_session_id IS NULL AND state='HELD'
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND id=create_attempt_id AND checkout_id=id AND generation=1)
 WITH CHECK(creator_kind='order_attempt' AND buyer_session_id IS NULL AND state='EXPIRED'
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND id=create_attempt_id AND checkout_id=id AND generation=1 AND expires_at<=clock_timestamp()
  AND EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=reservations.tenant_id
   AND o.store_id=reservations.store_id AND o.id=reservations.id AND o.owner_id=reservations.buyer_owner_id
   AND o.creator_kind='order_attempt' AND o.create_attempt_id=reservations.create_attempt_id
   AND o.creator_session_id IS NULL AND o.generation=reservations.generation
   AND o.payment_mode='bank_transfer' AND o.commercial_state='AWAITING_TRANSFER'
   AND o.expires_at<=clock_timestamp()));

CREATE POLICY order_attempt_ledger_expire ON inventory.ledger
 FOR INSERT TO commerce_checkout_writer
 WITH CHECK(actor_kind='SYSTEM_EXPIRY' AND principal_id IS NULL
  AND nullif(current_setting('app.principal_id',true),'') IS NULL
  AND creator_kind='order_attempt' AND buyer_session_id IS NULL
  AND nullif(current_setting('app.buyer_session_id',true),'') IS NULL
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND checkout_id=create_attempt_id AND reservation_id=checkout_id
  AND kind='RELEASE' AND operation='checkout.expire' AND command_key=checkout_id::text
  AND delta_on_hand=0 AND delta_allocated=0 AND delta_unavailable=0
  AND payment_attempt_id IS NULL AND payment_fact_kind IS NULL
  AND EXISTS(SELECT 1 FROM inventory.reservations r
   JOIN inventory.reservation_lines l ON l.tenant_id=r.tenant_id AND l.store_id=r.store_id AND l.reservation_id=r.id
   JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.id=r.checkout_id
   WHERE r.tenant_id=ledger.tenant_id AND r.store_id=ledger.store_id AND r.id=ledger.reservation_id
    AND r.buyer_owner_id=ledger.buyer_owner_id AND r.creator_kind='order_attempt'
    AND r.create_attempt_id=ledger.create_attempt_id AND r.buyer_session_id IS NULL
    AND r.state='HELD' AND r.generation=1 AND r.expires_at<=clock_timestamp()
    AND l.warehouse_id=ledger.warehouse_id AND l.sku_id=ledger.sku_id AND l.quantity=-ledger.delta_reserved
    AND o.owner_id=r.buyer_owner_id AND o.creator_kind=r.creator_kind AND o.create_attempt_id=r.create_attempt_id
    AND o.creator_session_id IS NULL AND o.generation=r.generation
    AND o.payment_mode='bank_transfer' AND o.commercial_state='AWAITING_TRANSFER'
    AND o.expires_at<=clock_timestamp()));

CREATE OR REPLACE FUNCTION checkout.expire_held(p_order uuid,p_generation bigint)
 RETURNS TABLE(disposition text,retry_at timestamptz)
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_identity record; v_order checkout.orders%ROWTYPE; v_reservation inventory.reservations%ROWTYPE;
 v_line record; v_now timestamptz; v_disposition text:='STALE'; v_retry timestamptz;
 v_tenant text:=current_setting('app.tenant_id',true);
 v_store text:=current_setting('app.store_id',true);
 v_buyer text:=current_setting('app.buyer_id',true);
 v_session text:=current_setting('app.buyer_session_id',true);
 v_principal text:=current_setting('app.principal_id',true);
 v_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 IF p_order IS NULL OR p_generation IS NULL OR p_generation<1 THEN
  RAISE EXCEPTION 'invalid expiry input' USING ERRCODE='PT400'; END IF;
 <<expiry>>
 BEGIN
  SELECT o.tenant_id,o.store_id INTO v_identity FROM checkout.orders o WHERE o.id=p_order;
  IF NOT FOUND THEN EXIT expiry; END IF;
  -- Same server-derived key as C2 and all compatible commands. No scope,
  -- principal, attempt or buyer-owner lock is acquired in this suffix.
  PERFORM pg_advisory_xact_lock(hashtextextended('checkout.order|'||v_identity.tenant_id::text
   ||'|'||v_identity.store_id::text||'|'||p_order::text,0));
  SELECT o.* INTO v_order FROM checkout.orders o WHERE o.id=p_order
   AND o.tenant_id=v_identity.tenant_id AND o.store_id=v_identity.store_id FOR UPDATE;
  IF NOT FOUND OR v_order.generation<>p_generation
   OR v_order.commercial_state NOT IN ('DRAFT','AWAITING_TRANSFER') THEN EXIT expiry; END IF;
  IF v_order.creator_kind='order_attempt' AND (v_order.create_attempt_id IS DISTINCT FROM p_order
   OR v_order.creator_session_id IS NOT NULL OR v_order.payment_mode<>'bank_transfer'
   OR v_order.commercial_state<>'AWAITING_TRANSFER') THEN EXIT expiry; END IF;
  PERFORM set_config('app.tenant_id',v_order.tenant_id::text,true);
  PERFORM set_config('app.store_id',v_order.store_id::text,true);
  PERFORM set_config('app.buyer_id',v_order.owner_id::text,true);
  PERFORM set_config('app.buyer_session_id',coalesce(v_order.creator_session_id::text,''),true);
  PERFORM set_config('app.principal_id','',true);
  PERFORM set_config('app.order_attempt_id',coalesce(v_order.create_attempt_id::text,''),true);
  SELECT r.* INTO v_reservation FROM inventory.reservations r
   WHERE r.tenant_id=v_order.tenant_id AND r.store_id=v_order.store_id AND r.id=p_order FOR UPDATE;
  IF NOT FOUND OR v_reservation.checkout_id IS DISTINCT FROM p_order
   OR v_reservation.buyer_owner_id IS DISTINCT FROM v_order.owner_id
   OR v_reservation.buyer_session_id IS DISTINCT FROM v_order.creator_session_id
   OR v_reservation.creator_kind IS DISTINCT FROM v_order.creator_kind
   OR v_reservation.create_attempt_id IS DISTINCT FROM v_order.create_attempt_id
   OR v_reservation.generation<>p_generation OR v_reservation.state<>'HELD' THEN EXIT expiry; END IF;
  FOR v_line IN SELECT l.warehouse_id,l.sku_id,l.quantity FROM inventory.reservation_lines l
   WHERE l.tenant_id=v_order.tenant_id AND l.store_id=v_order.store_id AND l.reservation_id=p_order
   ORDER BY l.warehouse_id,l.sku_id LOOP
   PERFORM 1 FROM inventory.lock_balance(v_line.warehouse_id,v_line.sku_id);
  END LOOP;
  v_now:=clock_timestamp();
  IF v_now<v_order.expires_at OR v_now<v_reservation.expires_at THEN
   v_disposition:='NOT_DUE'; v_retry:=greatest(v_order.expires_at,v_reservation.expires_at); EXIT expiry;
  END IF;
  FOR v_line IN SELECT l.warehouse_id,l.sku_id,l.quantity FROM inventory.reservation_lines l
   WHERE l.tenant_id=v_order.tenant_id AND l.store_id=v_order.store_id AND l.reservation_id=p_order
   ORDER BY l.warehouse_id,l.sku_id LOOP
   INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,
    operation,command_key,reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind,
    creator_kind,create_attempt_id)
   VALUES(v_order.tenant_id,v_order.store_id,v_line.warehouse_id,v_line.sku_id,'RELEASE',-v_line.quantity,
    'checkout.expire',p_order::text,p_order,NULL,p_order,v_order.owner_id,v_order.creator_session_id,'SYSTEM_EXPIRY',
    v_order.creator_kind,v_order.create_attempt_id);
  END LOOP;
  UPDATE inventory.reservations SET state='EXPIRED' WHERE tenant_id=v_order.tenant_id
   AND store_id=v_order.store_id AND id=p_order AND state='HELD' AND generation=p_generation;
  IF NOT FOUND THEN RAISE EXCEPTION 'expiry reservation mismatch' USING ERRCODE='PT409'; END IF;
  UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED',updated_at=v_now
   WHERE id=p_order AND generation=p_generation AND commercial_state IN ('DRAFT','AWAITING_TRANSFER');
  IF NOT FOUND THEN RAISE EXCEPTION 'expiry order mismatch' USING ERRCODE='PT409'; END IF;
  UPDATE checkout.bank_transfers SET state='EXPIRED',updated_at=v_now WHERE tenant_id=v_order.tenant_id
   AND store_id=v_order.store_id AND order_id=p_order AND state IN ('AWAITING','SUBMITTED','REJECTED');
  INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind,
   creator_kind,create_attempt_id)
  VALUES(v_order.tenant_id,v_order.store_id,v_order.owner_id,p_order,v_order.creator_session_id,
   p_generation,'checkout.expired','SYSTEM_EXPIRY',v_order.creator_kind,v_order.create_attempt_id);
  v_disposition:='EXPIRED';
 END expiry;
 PERFORM set_config('app.tenant_id',coalesce(v_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(v_store,''),true);
 PERFORM set_config('app.buyer_id',coalesce(v_buyer,''),true);
 PERFORM set_config('app.buyer_session_id',coalesce(v_session,''),true);
 PERFORM set_config('app.principal_id',coalesce(v_principal,''),true);
 PERFORM set_config('app.order_attempt_id',coalesce(v_attempt,''),true);
 RETURN QUERY SELECT v_disposition,v_retry;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(v_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(v_store,''),true);
 PERFORM set_config('app.buyer_id',coalesce(v_buyer,''),true);
 PERFORM set_config('app.buyer_session_id',coalesce(v_session,''),true);
 PERFORM set_config('app.principal_id',coalesce(v_principal,''),true);
 PERFORM set_config('app.order_attempt_id',coalesce(v_attempt,''),true);
 RAISE;
END $$;
ALTER FUNCTION checkout.expire_held(uuid,bigint) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.expire_held(uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.expire_held(uuid,bigint) TO commerce_expiry_worker;
COMMENT ON FUNCTION checkout.expire_held(uuid,bigint) IS
 'Existing expiry worker ABI: canonical order advisory then native order/reservation/sorted stock; exact storefront or attempt creator copied to one SYSTEM_EXPIRY release/event; no earlier lock or terminal attempt mutation; caller context restored.';
