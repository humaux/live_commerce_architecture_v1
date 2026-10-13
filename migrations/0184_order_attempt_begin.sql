-- Purpose: private OA1 native order, stock hold and offline placement in one outer C2 transaction.
-- Depends on: 0171 provenance, 0178 protected context, 0182 atomic-cart guard, 0183 allocation,
-- and post_river/0024 private expiry insertion (installed before this entry is ever invoked).
-- Used by: the order-attempt coordinator after owner/claim/order/catalog/service/allocation locks.
-- Invariants: I01/I03/I04/I08/I14; frozen money and origins, native stock conservation,
-- no bearer/current Place authorization, no independent commit, no terminal attempt write.

CREATE POLICY order_attempt_reservation_insert ON inventory.reservations FOR INSERT TO commerce_checkout_writer
 WITH CHECK(creator_kind='order_attempt' AND buyer_session_id IS NULL
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND id=create_attempt_id AND checkout_id=id AND generation=1);
CREATE POLICY order_attempt_reservation_commit ON inventory.reservations FOR UPDATE TO commerce_checkout_writer
 USING(creator_kind='order_attempt' AND buyer_session_id IS NULL
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid)
 WITH CHECK(creator_kind='order_attempt' AND buyer_session_id IS NULL AND state='COMMITTED'
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND id=create_attempt_id AND checkout_id=id AND generation=1);
CREATE POLICY order_attempt_ledger_insert ON inventory.ledger FOR INSERT TO commerce_checkout_writer
 WITH CHECK(actor_kind='ORDER_ATTEMPT' AND creator_kind='order_attempt' AND buyer_session_id IS NULL
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND checkout_id=create_attempt_id AND reservation_id=checkout_id);

-- Preserve the final 0072/0088/0155 legacy actor branches and common reservation check;
-- the new actor additionally requires exact admitted principal/attempt identity.
CREATE OR REPLACE FUNCTION inventory.guard_checkout_ledger() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_checkout uuid; v_owner uuid; v_session uuid;
BEGIN
 IF NEW.actor_kind='ORDER_ATTEMPT' THEN
  IF NEW.creator_kind IS DISTINCT FROM 'order_attempt' OR NEW.buyer_session_id IS NOT NULL
   OR NEW.principal_id IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
   OR NEW.principal_id IS NULL
   OR NEW.create_attempt_id IS DISTINCT FROM nullif(current_setting('app.order_attempt_id',true),'')::uuid
   OR NEW.create_attempt_id IS NULL OR NEW.checkout_id IS DISTINCT FROM NEW.create_attempt_id
   OR NEW.checkout_id IS DISTINCT FROM NEW.reservation_id
   OR NEW.buyer_owner_id IS DISTINCT FROM nullif(current_setting('app.buyer_id',true),'')::uuid
   OR nullif(current_setting('app.buyer_session_id',true),'') IS NOT NULL
   OR NOT EXISTS(SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id
    AND r.id=NEW.reservation_id AND r.creator_kind='order_attempt' AND r.create_attempt_id=NEW.create_attempt_id
    AND r.buyer_owner_id=NEW.buyer_owner_id AND r.buyer_session_id IS NULL AND r.state='HELD' AND r.generation=1) THEN
   RAISE EXCEPTION 'ledger actor mismatch' USING ERRCODE='42501'; END IF;
 ELSIF NEW.actor_kind='MERCHANT' AND (NEW.kind IN ('DEALLOCATE','ALLOCATE') OR (NEW.kind='RELEASE' AND NEW.operation='checkout.merchant_cancel')) THEN
  IF NEW.principal_id IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
     OR NEW.checkout_id IS DISTINCT FROM NEW.reservation_id
     OR NEW.buyer_owner_id IS DISTINCT FROM nullif(current_setting('app.buyer_id',true),'')::uuid
     OR NEW.buyer_session_id IS DISTINCT FROM nullif(current_setting('app.buyer_session_id',true),'')::uuid THEN
   RAISE EXCEPTION 'ledger actor mismatch' USING ERRCODE='42501'; END IF;
 ELSIF NEW.actor_kind='MERCHANT' THEN
  IF NEW.principal_id IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
     OR NEW.checkout_id IS NOT NULL OR NEW.buyer_owner_id IS NOT NULL OR NEW.buyer_session_id IS NOT NULL THEN
   RAISE EXCEPTION 'ledger actor mismatch' USING ERRCODE='42501'; END IF;
 ELSE
  IF NEW.principal_id IS NOT NULL OR current_setting('app.principal_id',true)<>''
     OR NEW.buyer_owner_id IS DISTINCT FROM nullif(current_setting('app.buyer_id',true),'')::uuid
     OR NEW.buyer_session_id IS DISTINCT FROM nullif(current_setting('app.buyer_session_id',true),'')::uuid
     OR NEW.checkout_id IS DISTINCT FROM NEW.reservation_id THEN
   RAISE EXCEPTION 'ledger actor mismatch' USING ERRCODE='42501'; END IF;
 END IF;
 IF NEW.reservation_id IS NOT NULL THEN
  SELECT r.checkout_id,r.buyer_owner_id,r.buyer_session_id INTO v_checkout,v_owner,v_session
   FROM inventory.reservations r WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id AND r.id=NEW.reservation_id;
  IF NOT FOUND OR v_checkout IS DISTINCT FROM NEW.checkout_id
     OR v_owner IS DISTINCT FROM NEW.buyer_owner_id OR v_session IS DISTINCT FROM NEW.buyer_session_id THEN
   RAISE EXCEPTION 'ledger reservation mismatch' USING ERRCODE='42501'; END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inventory.guard_checkout_ledger() OWNER TO commerce_inventory_writer;
REVOKE ALL ON FUNCTION inventory.guard_checkout_ledger() FROM PUBLIC;
COMMENT ON FUNCTION inventory.guard_checkout_ledger() IS
 'Inventory trigger: unchanged legacy actor/reservation guards plus exact ORDER_ATTEMPT principal/owner/attempt GUCs, HELD native attempt reservation and no session. 0171 composite creator FKs and actor CHECK remain authoritative; no runtime grants.';

CREATE FUNCTION inventory.guard_order_attempt_ledger() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.actor_kind='ORDER_ATTEMPT' AND (
  NOT EXISTS(SELECT 1 FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id AND l.store_id=NEW.store_id
   AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id AND l.sku_id=NEW.sku_id
   AND ((NEW.kind='RESERVE' AND l.quantity=NEW.delta_reserved AND NEW.delta_allocated=0)
    OR (NEW.kind='ALLOCATE' AND l.quantity=-NEW.delta_reserved AND l.quantity=NEW.delta_allocated)))
  OR NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id
   AND o.id=NEW.checkout_id AND o.id=NEW.create_attempt_id AND o.owner_id=NEW.buyer_owner_id
   AND o.creator_kind='order_attempt' AND o.create_attempt_id=NEW.create_attempt_id AND o.creator_session_id IS NULL
   AND (NEW.kind='RESERVE' OR (NEW.kind='ALLOCATE' AND o.collection_state='PENDING'
    AND ((o.payment_mode='pay_at_pickup' AND o.commercial_state='CONFIRMED')
     OR (o.payment_mode='cash_on_delivery' AND o.commercial_state='AWAITING_COLLECTION')))))
  OR EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
   AND a.order_id=NEW.checkout_id)) THEN
  RAISE EXCEPTION 'attempt ledger mismatch' USING ERRCODE='42501';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inventory.guard_order_attempt_ledger() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.guard_order_attempt_ledger() FROM PUBLIC;
COMMENT ON FUNCTION inventory.guard_order_attempt_ledger() IS
 'Private trigger-only native attempt reserve/offline allocate guard: exact reservation quantity, creator/order/owner, PAP/COD state and no payment attempt. Does not alter legacy pay-at-pickup/payment guards.';
CREATE TRIGGER zz_order_attempt_ledger_guard BEFORE INSERT ON inventory.ledger
 FOR EACH ROW EXECUTE FUNCTION inventory.guard_order_attempt_ledger();

CREATE FUNCTION checkout.order_attempt_lock_offline_safety(p_attempt uuid,p_plan_plaintext text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_variable
DECLARE ctx jsonb; binding jsonb; service jsonb; mode text; tenant uuid; store uuid; version bigint; current_service record;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext); binding:=ctx->'binding';
 service:=ctx->'plan'->'facts'->'catalog'->'service'; mode:=ctx->'plan'->'facts'->'selection'->>'payment_mode';
 tenant:=(binding->>'tenant_id')::uuid; store:=(binding->>'store_id')::uuid;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id','',true),set_config('app.buyer_id',binding->>'buyer_owner_id',true),
  set_config('app.buyer_session_id','',true),set_config('app.order_attempt_id',p_attempt::text,true);
 -- Service level follows sorted catalog and precedes any pickup/destination or
 -- inventory lock. These are the same native rows used by legacy Begin.
 SELECT h.current_version INTO version FROM fulfillment.service_heads h WHERE h.tenant_id=tenant AND h.store_id=store
  AND h.market_id=(service->>'market_id')::uuid AND h.country=service->>'country' AND h.code=service->>'code' FOR SHARE;
 SELECT s.enabled,s.delivery_kind,s.mode,s.policy_method INTO current_service FROM fulfillment.service_versions s
  WHERE s.tenant_id=tenant AND s.store_id=store AND s.market_id=(service->>'market_id')::uuid
   AND s.country=service->>'country' AND s.code=service->>'code' AND s.version=version;
 IF NOT FOUND OR NOT current_service.enabled OR current_service.delivery_kind IS DISTINCT FROM service->>'delivery_kind'
  OR current_service.mode IS DISTINCT FROM service->>'mode' OR current_service.policy_method IS DISTINCT FROM service->>'policy_method' THEN
  RAISE EXCEPTION 'delivery_service_unavailable' USING ERRCODE='PT422'; END IF;
 IF mode='pay_at_pickup' THEN
  PERFORM 1 FROM fulfillment.cvs_store_settings c WHERE c.tenant_id=tenant AND c.store_id=store FOR UPDATE;
 ELSIF mode='cash_on_delivery' THEN
  PERFORM 1 FROM checkout.cash_on_delivery_settings c WHERE c.tenant_id=tenant AND c.store_id=store FOR UPDATE;
 ELSIF mode='bank_transfer' THEN
  PERFORM 1 FROM checkout.bank_transfer_settings c WHERE c.tenant_id=tenant AND c.store_id=store FOR SHARE;
 ELSE RAISE EXCEPTION 'payment_mode_unavailable' USING ERRCODE='PT400';
 END IF;
 IF NOT FOUND THEN RAISE EXCEPTION 'offline_settings_unavailable' USING ERRCODE='PT409'; END IF;
 -- The admitted option, window, surcharge and bank details remain frozen.
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RETURN jsonb_build_object('payment_mode',mode);
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RAISE;
END $$;
ALTER FUNCTION checkout.order_attempt_lock_offline_safety(uuid,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_attempt_lock_offline_safety(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.order_attempt_lock_offline_safety(uuid,text) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION checkout.order_attempt_lock_offline_safety(uuid,text) IS
 'Private C2 service-level suffix before destination/inventory: protected exact plan, current enabled compatible service FOR SHARE, then native offline settings serialization (PAP/COD FOR UPDATE, bank FOR SHARE). Caller retains earlier owner/claim/order/cart/catalog prefix. No current fee/window/option/RBAC comparison or business write; admitted facts honored. Returns payment_mode; coordinator only. Required before allocation and Begin to preserve native offline capacity concurrency.';

CREATE FUNCTION checkout.order_attempt_begin(p_attempt uuid,p_plan_plaintext text,p_cart uuid,p_quote uuid,p_destination uuid,p_lines jsonb)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_variable
DECLARE ctx jsonb; binding jsonb; plan jsonb; catalog jsonb; selection jsonb; service jsonb; payment jsonb;
 tenant uuid; store uuid; owner_id uuid; principal uuid; market uuid; country text; code text; currency text; mode text;
 quote_row record; destination_row record; allocation_row record; pickup_row record; current_service record;
 allocation jsonb; destination jsonb; pickup jsonb; snapshot jsonb; expiry jsonb; result jsonb;
 allocation_version bigint; warehouse_ids uuid[]; line record; stock record;
 line_count integer; distinct_count integer; now_at timestamptz; expires timestamptz; job_id bigint;
 state text; total bigint; request_hash bytea;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 binding:=ctx->'binding'; plan:=ctx->'plan'; catalog:=plan->'facts'->'catalog'; selection:=plan->'facts'->'selection';
 service:=catalog->'service'; payment:=catalog->'payment'; mode:=selection->>'payment_mode';
 tenant:=(binding->>'tenant_id')::uuid; store:=(binding->>'store_id')::uuid;
 owner_id:=(binding->>'buyer_owner_id')::uuid; principal:=(binding->>'principal_id')::uuid;
 market:=(service->>'market_id')::uuid; country:=service->>'country'; code:=service->>'code'; currency:=catalog->'market'->>'currency';
 IF mode IS NULL OR mode NOT IN ('bank_transfer','pay_at_pickup','cash_on_delivery')
  OR payment->>'mode' IS DISTINCT FROM mode OR jsonb_typeof(p_lines) IS DISTINCT FROM 'array'
  OR octet_length(p_lines::text)>1048576 THEN
  RAISE EXCEPTION 'attempt_begin_invalid' USING ERRCODE='PT400'; END IF;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id',principal::text,true),set_config('app.buyer_id',owner_id::text,true),
  set_config('app.buyer_session_id','',true),set_config('app.order_attempt_id',p_attempt::text,true);
 -- Re-read safety facts protected by the retained prefix without acquiring
 -- any earlier lock or comparing a mutable price/display/version.
 IF NOT EXISTS(SELECT 1 FROM control.stores s WHERE s.tenant_id=tenant AND s.id=store AND s.active) THEN
  RAISE EXCEPTION 'store_unavailable' USING ERRCODE='PT404'; END IF;
 IF EXISTS(SELECT 1 FROM jsonb_array_elements(catalog->'lines') x
  LEFT JOIN catalog.skus s ON s.tenant_id=tenant AND s.store_id=store AND s.id=(x->>'sku_id')::uuid
  LEFT JOIN catalog.products p ON p.tenant_id=tenant AND p.store_id=store AND p.id=s.product_id
  WHERE s.id IS NULL OR s.status IS DISTINCT FROM 'active' OR p.status IS DISTINCT FROM 'active'
   OR s.product_id IS DISTINCT FROM (x->>'product_id')::uuid OR s.currency IS DISTINCT FROM currency) THEN
  RAISE EXCEPTION 'sku_unavailable' USING ERRCODE='PT422'; END IF;
 IF EXISTS(SELECT 1 FROM (SELECT (x->>'sku_id')::uuid id,sum((x->>'quantity')::bigint) quantity
  FROM jsonb_array_elements(catalog->'lines') x GROUP BY (x->>'sku_id')::uuid) d JOIN catalog.skus s
  ON s.tenant_id=tenant AND s.store_id=store AND s.id=d.id WHERE d.quantity NOT BETWEEN 1 AND 1000000000
   OR (NOT s.inventory_tracked AND (s.max_per_order IS NULL OR d.quantity>s.max_per_order))) THEN
  RAISE EXCEPTION 'max_per_order_exceeded' USING ERRCODE='PT422'; END IF;
 SELECT s.enabled,s.delivery_kind,s.mode,s.policy_method INTO current_service FROM fulfillment.service_heads h
  JOIN fulfillment.service_versions s ON s.tenant_id=h.tenant_id AND s.store_id=h.store_id AND s.market_id=h.market_id
   AND s.country=h.country AND s.code=h.code AND s.version=h.current_version
  WHERE h.tenant_id=tenant AND h.store_id=store AND h.market_id=market AND h.country=country AND h.code=code;
 IF NOT FOUND OR NOT current_service.enabled OR current_service.delivery_kind IS DISTINCT FROM service->>'delivery_kind'
  OR current_service.mode IS DISTINCT FROM service->>'mode' OR current_service.policy_method IS DISTINCT FROM service->>'policy_method' THEN
  RAISE EXCEPTION 'delivery_service_unavailable' USING ERRCODE='PT422'; END IF;
 -- The prefix has already retained all earlier locks. Read only exact native rows;
 -- 0182 prevents an attempt cart from committing before complete C2 settlement.
 IF NOT EXISTS(SELECT 1 FROM storefront.carts c WHERE c.tenant_id=tenant AND c.store_id=store AND c.owner_id=owner_id
  AND c.id=p_cart AND c.cart_kind='order_attempt' AND c.create_attempt_id=p_attempt AND c.creator_session_id IS NULL
  AND c.version=1 AND c.currency=currency)
  OR EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=tenant AND o.store_id=store
   AND (o.id=p_attempt OR o.create_attempt_id=p_attempt)) THEN
  RAISE EXCEPTION 'attempt_cart_conflict' USING ERRCODE='PT409'; END IF;
 SELECT q.* INTO quote_row FROM storefront.quotes q WHERE q.tenant_id=tenant AND q.store_id=store AND q.owner_id=owner_id
  AND q.id=p_quote AND q.cart_id=p_cart AND q.cart_version=1 AND q.creator_kind='order_attempt'
  AND q.create_attempt_id=p_attempt AND q.creator_session_id IS NULL;
 IF NOT FOUND OR quote_row.market_id IS DISTINCT FROM market OR quote_row.country IS DISTINCT FROM country
  OR quote_row.currency IS DISTINCT FROM currency OR quote_row.snapshot->'amount' IS DISTINCT FROM plan->'calculation'
  OR quote_row.created_at IS DISTINCT FROM (binding->>'admitted_at')::timestamptz
  OR quote_row.expires_at IS DISTINCT FROM (binding->>'execute_by')::timestamptz THEN
  RAISE EXCEPTION 'attempt_quote_conflict' USING ERRCODE='PT409'; END IF;
 SELECT d.* INTO destination_row FROM storefront.destination_snapshots d WHERE d.tenant_id=tenant AND d.store_id=store
  AND d.owner_id=owner_id AND d.id=p_destination AND d.cart_id=p_cart AND d.cart_version=1 AND d.version=1
  AND d.creator_kind='order_attempt' AND d.create_attempt_id=p_attempt AND d.creator_session_id IS NULL;
 IF NOT FOUND OR destination_row.country IS DISTINCT FROM country OR destination_row.kind IS DISTINCT FROM service->>'delivery_kind'
  OR destination_row.selected_at IS DISTINCT FROM (binding->>'admitted_at')::timestamptz
  OR destination_row.expires_at IS DISTINCT FROM (binding->>'execute_by')::timestamptz
  OR destination_row.recipient_name IS DISTINCT FROM selection->'customer'->>'name'
  OR destination_row.phone IS DISTINCT FROM selection->'customer'->>'phone'
  OR NOT EXISTS(SELECT 1 FROM storefront.destination_heads h WHERE h.tenant_id=tenant AND h.store_id=store
   AND h.owner_id=owner_id AND h.cart_id=p_cart AND h.destination_id=p_destination AND h.current_version=1) THEN
  RAISE EXCEPTION 'attempt_destination_conflict' USING ERRCODE='PT409'; END IF;
 destination:=jsonb_build_object('id',p_destination,'version',1,'cart_id',p_cart,'cart_version',1,'kind',destination_row.kind,
  'country',destination_row.country,'recipient_name',destination_row.recipient_name,'phone',destination_row.phone,
  'home_address',selection->'delivery'->'home_address','selected_at',destination_row.selected_at,'expires_at',destination_row.expires_at);
 IF destination_row.pickup_id IS NOT NULL THEN
  SELECT p.id INTO pickup_row FROM fulfillment.pickup_versions p WHERE p.tenant_id=tenant AND p.store_id=store
   AND p.id=destination_row.pickup_id AND p.buyer_owner_id=owner_id AND p.create_attempt_id=p_attempt
   AND p.namespace='attempt.'||p_attempt::text AND p.verification_kind='BUYER_ENTERED';
  IF NOT FOUND THEN RAISE EXCEPTION 'attempt_destination_conflict' USING ERRCODE='PT409'; END IF;
  pickup:=((catalog->'pickup')-ARRAY['buyer_owner_id','create_attempt_id']::text[])||jsonb_build_object('id',pickup_row.id);
  destination:=destination||jsonb_build_object('pickup',pickup);
 ELSIF destination_row.kind<>'home' THEN
  RAISE EXCEPTION 'attempt_destination_conflict' USING ERRCODE='PT409';
 END IF;
 SELECT h.current_version INTO allocation_version FROM fulfillment.allocation_heads h WHERE h.tenant_id=tenant
  AND h.store_id=store AND h.market_id=market AND h.country=country AND h.code=code;
 SELECT a.* INTO allocation_row FROM fulfillment.allocation_versions a WHERE a.tenant_id=tenant AND a.store_id=store
  AND a.market_id=market AND a.country=country AND a.code=code AND a.version=allocation_version;
 IF NOT FOUND THEN RAISE EXCEPTION 'allocation_unavailable' USING ERRCODE='PT409'; END IF;
 SELECT array_agg(a.warehouse_id ORDER BY a.position) INTO warehouse_ids FROM fulfillment.allocation_warehouses a
  WHERE a.tenant_id=tenant AND a.store_id=store AND a.market_id=market AND a.country=country AND a.code=code AND a.version=allocation_version;
 IF cardinality(warehouse_ids) IS DISTINCT FROM allocation_row.warehouse_count::integer THEN
  RAISE EXCEPTION 'allocation_unavailable' USING ERRCODE='PT409'; END IF;
 allocation:=jsonb_build_object('market_id',market,'country',country,'code',code,'version',allocation_version,
  'service_version',allocation_row.service_version,'warehouse_ids',to_jsonb(warehouse_ids));
 SELECT count(*),count(DISTINCT (l.warehouse_id,l.sku_id)) INTO line_count,distinct_count
  FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint);
 IF line_count NOT BETWEEN 0 AND 800 OR line_count<>distinct_count THEN
  RAISE EXCEPTION 'invalid_stock_plan' USING ERRCODE='PT400'; END IF;
 -- Duplicate origins remain separate in the snapshot; only tracked demand aggregates.
 IF EXISTS(WITH demand AS (SELECT s.id sku_id,sum(q.quantity) quantity
   FROM jsonb_to_recordset(quote_row.snapshot->'lines') AS q(sku_id uuid,quantity bigint)
   JOIN catalog.skus s ON s.tenant_id=tenant AND s.store_id=store AND s.id=q.sku_id AND s.inventory_tracked GROUP BY s.id),
  supplied AS (SELECT l.sku_id,sum(l.quantity) quantity FROM jsonb_to_recordset(p_lines) AS l(sku_id uuid,quantity bigint) GROUP BY l.sku_id)
  SELECT 1 FROM demand d FULL JOIN supplied s USING(sku_id)
   WHERE d.sku_id IS NULL OR s.sku_id IS NULL OR d.quantity IS DISTINCT FROM s.quantity) THEN
  RAISE EXCEPTION 'stock_plan_differs_from_quote' USING ERRCODE='PT409'; END IF;
 FOR line IN SELECT l.* FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint)
  ORDER BY l.warehouse_id,l.sku_id LOOP
  IF line.warehouse_id IS NULL OR line.sku_id IS NULL OR line.quantity IS NULL OR line.quantity NOT BETWEEN 1 AND 1000000000
   OR NOT line.warehouse_id=ANY(warehouse_ids)
   OR NOT EXISTS(SELECT 1 FROM inventory.warehouses w WHERE w.tenant_id=tenant AND w.store_id=store AND w.id=line.warehouse_id AND w.active) THEN
   RAISE EXCEPTION 'invalid_stock_plan' USING ERRCODE='PT409'; END IF;
  -- Calls inventory.lock_balance: same final warehouse/SKU level, sorted. No earlier locks.
  SELECT b.* INTO stock FROM inventory.lock_balance(line.warehouse_id,line.sku_id) b;
  IF NOT FOUND OR stock.on_hand-stock.reserved-stock.allocated-stock.unavailable<line.quantity THEN
   RAISE EXCEPTION 'insufficient_stock' USING ERRCODE='PT402'; END IF;
 END LOOP;
 total:=(quote_row.snapshot->'amount'->>'total_minor')::bigint;
 IF mode='pay_at_pickup' AND (destination_row.kind NOT IN ('cvs_711','cvs_familymart','cvs_hilife','cvs_okmart')
  OR currency<>'TWD' OR total%100<>0 OR total NOT BETWEEN 100 AND least((payment->>'max_minor')::bigint,2000000)
  OR NOT fulfillment.ecpay_recipient_ok(destination_row.recipient_name,destination_row.phone)) THEN
  RAISE EXCEPTION 'pay_at_pickup_unavailable' USING ERRCODE='PT422'; END IF;
 IF mode='cash_on_delivery' AND (destination_row.kind<>'home' OR country<>'TW' OR currency<>'TWD'
  OR total%100<>0 OR total NOT BETWEEN 100 AND 2000000 OR total+(payment->>'surcharge_minor')::bigint>(payment->>'max_minor')::bigint) THEN
  RAISE EXCEPTION 'cash_on_delivery_unavailable' USING ERRCODE='PT422'; END IF;
 -- Offline capacity is checked under the settings serialization retained by
 -- the service-level prefix, never by taking an earlier settings lock here.
 IF mode IN ('pay_at_pickup','cash_on_delivery') AND (
  (SELECT count(*) FROM checkout.orders o WHERE o.tenant_id=tenant AND o.store_id=store AND o.payment_mode=mode
   AND o.collection_state='PENDING' AND o.fulfillment_state='MANUAL_UNASSIGNED')>=
    CASE WHEN mode='pay_at_pickup' THEN (payment->>'max_open')::integer
     ELSE coalesce((SELECT c.pay_at_pickup_max_open FROM fulfillment.cvs_store_settings c WHERE c.tenant_id=tenant AND c.store_id=store),20) END
  OR EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=tenant AND o.store_id=store AND o.owner_id=owner_id
   AND o.payment_mode=mode AND o.collection_state='PENDING' AND o.fulfillment_state='MANUAL_UNASSIGNED')) THEN
  RAISE EXCEPTION 'offline_order_limit' USING ERRCODE='PT429'; END IF;
 snapshot:=jsonb_build_object('quote',quote_row.snapshot,'destination',destination,'service',service,'allocation',allocation);
 IF mode='cash_on_delivery' THEN snapshot:=snapshot||jsonb_build_object('expected_cod_surcharge_minor',payment->'surcharge_minor'); END IF;
 -- Calls protected context again after all waits; expiry helper inserts the
 -- native queue prerequisite BEFORE its order FK target exists, in this Tx.
 PERFORM ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 now_at:=clock_timestamp();
 expiry:=checkout.order_attempt_enqueue_expiry(p_attempt,p_plan_plaintext,now_at);
 expires:=(expiry->>'expires_at')::timestamptz; job_id:=(expiry->>'job_id')::bigint;
 state:=CASE mode WHEN 'bank_transfer' THEN 'AWAITING_TRANSFER' WHEN 'pay_at_pickup' THEN 'CONFIRMED' ELSE 'AWAITING_COLLECTION' END;
 result:=jsonb_build_object('order_id',p_attempt,'reservation_id',p_attempt,'generation',1,'expires_at',expires,
  'job_id',job_id,'payment_mode',mode,'commercial_state',state);
 INSERT INTO checkout.orders(tenant_id,store_id,owner_id,id,creator_session_id,cart_id,cart_version,quote_id,destination_id,
  market_id,country,service_code,service_version,allocation_version,currency,total_minor,cod_surcharge_minor,cod_carrier,
  commercial_state,fulfillment_state,generation,expires_at,job_id,snapshot,created_at,updated_at,payment_mode,collection_state,
  buyer_email,locale,creator_kind,create_attempt_id)
 VALUES(tenant,store,owner_id,p_attempt,NULL,p_cart,1,p_quote,p_destination,market,country,code,(service->>'version')::bigint,
  allocation_version,currency,total,CASE WHEN mode='cash_on_delivery' THEN (payment->>'surcharge_minor')::bigint END,
  CASE WHEN mode='cash_on_delivery' THEN payment->>'carrier' END,state,'MANUAL_UNASSIGNED',1,expires,job_id,snapshot,now_at,now_at,
  mode,CASE WHEN mode IN ('pay_at_pickup','cash_on_delivery') THEN 'PENDING' END,
  nullif(selection->'customer'->>'email',''),selection->>'locale','order_attempt',p_attempt);
 INSERT INTO inventory.reservations(tenant_id,store_id,id,state,expires_at,checkout_id,buyer_owner_id,buyer_session_id,generation,
  creator_kind,create_attempt_id) VALUES(tenant,store,p_attempt,'HELD',expires,p_attempt,owner_id,NULL,1,'order_attempt',p_attempt);
 FOR line IN SELECT l.* FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint)
  ORDER BY l.warehouse_id,l.sku_id LOOP
  INSERT INTO inventory.reservation_lines(tenant_id,store_id,reservation_id,warehouse_id,sku_id,quantity)
   VALUES(tenant,store,p_attempt,line.warehouse_id,line.sku_id,line.quantity);
  INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,operation,command_key,reservation_id,
   principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind,creator_kind,create_attempt_id)
   VALUES(tenant,store,line.warehouse_id,line.sku_id,'RESERVE',line.quantity,'checkout.order_attempt.reserve',p_attempt::text,
    p_attempt,principal,p_attempt,owner_id,NULL,'ORDER_ATTEMPT','order_attempt',p_attempt);
 END LOOP;
 IF mode IN ('pay_at_pickup','cash_on_delivery') THEN
  FOR line IN SELECT l.* FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint)
   ORDER BY l.warehouse_id,l.sku_id LOOP
   INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,operation,command_key,
    reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind,creator_kind,create_attempt_id)
    VALUES(tenant,store,line.warehouse_id,line.sku_id,'ALLOCATE',-line.quantity,line.quantity,'checkout.pay_at_pickup.commit',
     p_attempt::text,p_attempt,principal,p_attempt,owner_id,NULL,'ORDER_ATTEMPT','order_attempt',p_attempt);
  END LOOP;
  UPDATE inventory.reservations r SET state='COMMITTED' WHERE r.tenant_id=tenant AND r.store_id=store AND r.id=p_attempt AND r.state='HELD';
 END IF;
 INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind,creator_kind,create_attempt_id)
  VALUES(tenant,store,owner_id,p_attempt,NULL,1,'checkout.held','ORDER_ATTEMPT','order_attempt',p_attempt),
   (tenant,store,owner_id,p_attempt,NULL,1,'checkout.'||mode||'_placed','ORDER_ATTEMPT','order_attempt',p_attempt);
 IF mode='bank_transfer' THEN
  INSERT INTO checkout.bank_transfers(tenant_id,store_id,owner_id,order_id,state,bank_name,branch,account_name,account_number,
   window_hours,currency,created_at,updated_at)
  VALUES(tenant,store,owner_id,p_attempt,'AWAITING',payment->>'bank_name',payment->>'branch',payment->>'account_name',
   payment->>'account_number',(payment->>'window_hours')::integer,currency,now_at,now_at);
 END IF;
 request_hash:=sha256(convert_to((plan->'facts'->'selection')::text,'UTF8'));
 INSERT INTO checkout.command_results(tenant_id,store_id,owner_id,operation,idempotency_key,creator_session_id,request_hash,
  order_id,response,creator_kind,create_attempt_id)
 VALUES(tenant,store,owner_id,'checkout.begin',p_attempt::text,NULL,request_hash,p_attempt,result,'order_attempt',p_attempt);
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RAISE;
END $$;
ALTER FUNCTION checkout.order_attempt_begin(uuid,text,uuid,uuid,uuid,jsonb) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_attempt_begin(uuid,text,uuid,uuid,uuid,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.order_attempt_begin(uuid,text,uuid,uuid,uuid,jsonb) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION checkout.order_attempt_begin(uuid,text,uuid,uuid,uuid,jsonb) IS
 'Private native C2 Begin: protected exact plan and attempt cart/quote/destination, frozen money/service/offline details, current native allocation and sorted stock conservation. Caller retains complete earlier owner/claim/order/catalog/service/offline-capacity/allocation prefix. Inserts fresh expiry job before native order, reservation, guarded ledger, offline fact, events and exact command receipt in the same outer Tx; no bearer, current Place/repricing, payment attempt, source/audit, terminal settlement or internal commit. EXECUTE coordinator only; requires post_river0024 and0182 atomic-cart guard.';
