-- Purpose: private OA1 catalog safety and locked allocation facts for the Go allocator.
-- Depends on: 0178 execution context, 0179 quote, 0181 destination, 0182 cart commit guard.
-- Used by: order-attempt coordinator; catalog precedes destination, allocation follows it.
-- Invariants: I01/I03/I04/I08; exact provenance, no repricing, no stock writes or SQL allocator.
-- Table columns are qualified throughout; explicit variable resolution keeps binding names unambiguous.

GRANT EXECUTE ON FUNCTION inventory.lock_warehouse(uuid) TO commerce_checkout_writer;

CREATE FUNCTION checkout.order_attempt_lock_catalog(p_attempt uuid,p_plan_plaintext text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_variable
DECLARE ctx jsonb; binding jsonb; catalog jsonb; lines jsonb; result jsonb;
 tenant uuid; store uuid; owner_id uuid; currency text; ids uuid[]; parents uuid[];
 item record; seen integer:=0; quantity bigint;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 -- Calls ops.order_attempt_execution_context: protected current xid and original plaintext, not caller GUCs.
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 binding:=ctx->'binding'; catalog:=ctx->'plan'->'facts'->'catalog'; lines:=catalog->'lines';
 tenant:=(binding->>'tenant_id')::uuid; store:=(binding->>'store_id')::uuid;
 owner_id:=(binding->>'buyer_owner_id')::uuid; currency:=catalog->'market'->>'currency';
 IF jsonb_typeof(lines) IS DISTINCT FROM 'array' OR jsonb_array_length(lines) NOT BETWEEN 1 AND 50 THEN
  RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400';
 END IF;
 SELECT array_agg(DISTINCT (x->>'sku_id')::uuid ORDER BY (x->>'sku_id')::uuid),
  array_agg(DISTINCT (x->>'product_id')::uuid ORDER BY (x->>'product_id')::uuid)
  INTO ids,parents FROM jsonb_array_elements(lines) x;
 IF array_position(ids,NULL) IS NOT NULL OR array_position(parents,NULL) IS NOT NULL OR currency IS NULL THEN
  RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400';
 END IF;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id','',true),set_config('app.buyer_id',owner_id::text,true),
  set_config('app.buyer_session_id','',true),set_config('app.order_attempt_id',p_attempt::text,true);
 IF NOT EXISTS(SELECT 1 FROM control.stores s WHERE s.tenant_id=tenant AND s.id=store AND s.active) THEN
  RAISE EXCEPTION 'store_unavailable' USING ERRCODE='PT404';
 END IF;
 -- Parent C2 has already held owners/claims/order and materialized its cart.
 -- Product membership comes from the immutable admitted facts, never a current price lookup.
 FOR item IN SELECT p.id,p.status FROM catalog.products p WHERE p.tenant_id=tenant AND p.store_id=store
  AND p.id=ANY(parents) ORDER BY p.id FOR SHARE LOOP
  seen:=seen+1;
  IF item.status<>'active' THEN RAISE EXCEPTION 'sku_inactive' USING ERRCODE='PT422'; END IF;
 END LOOP;
 IF seen<>cardinality(parents) THEN RAISE EXCEPTION 'sku_not_found' USING ERRCODE='PT404'; END IF;
 seen:=0;
 FOR item IN SELECT s.id,s.product_id,s.status,s.currency,s.inventory_tracked,s.max_per_order
  FROM catalog.skus s WHERE s.tenant_id=tenant AND s.store_id=store AND s.id=ANY(ids) ORDER BY s.id FOR SHARE LOOP
  seen:=seen+1;
  IF item.status<>'active' THEN RAISE EXCEPTION 'sku_inactive' USING ERRCODE='PT422'; END IF;
  IF item.currency IS DISTINCT FROM currency OR EXISTS(SELECT 1 FROM jsonb_array_elements(lines) x
   WHERE (x->>'sku_id')::uuid=item.id AND (x->>'product_id')::uuid IS DISTINCT FROM item.product_id) THEN
   RAISE EXCEPTION 'sku_changed' USING ERRCODE='PT422';
  END IF;
  SELECT sum((x->>'quantity')::bigint) INTO quantity FROM jsonb_array_elements(lines) x WHERE (x->>'sku_id')::uuid=item.id;
  IF quantity IS NULL OR quantity NOT BETWEEN 1 AND 1000000000
   OR (NOT item.inventory_tracked AND (item.max_per_order IS NULL OR quantity>item.max_per_order)) THEN
   RAISE EXCEPTION 'max_per_order_exceeded' USING ERRCODE='PT422';
  END IF;
 END LOOP;
 IF seen<>cardinality(ids) THEN RAISE EXCEPTION 'sku_not_found' USING ERRCODE='PT404'; END IF;
 result:=jsonb_build_object('sku_ids',to_jsonb(ids));
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
ALTER FUNCTION checkout.order_attempt_lock_catalog(uuid,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_attempt_lock_catalog(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.order_attempt_lock_catalog(uuid,text) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION checkout.order_attempt_lock_catalog(uuid,text) IS
 'Private C2 pre-destination safety suffix: exact protected execution context, product then unique SKU FOR SHARE in UUID order, current active/currency/membership/aggregated untracked cap; no price/version comparison or writes. Caller holds attempt/owners/claims/order/cart prefix. Returns sorted sku_ids only; EXECUTE coordinator only.';

CREATE FUNCTION checkout.order_attempt_prepare_allocation(p_attempt uuid,p_plan_plaintext text,p_cart uuid,p_quote uuid,p_destination uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_variable
DECLARE ctx jsonb; binding jsonb; catalog jsonb; lines jsonb; service jsonb; result jsonb;
 tenant uuid; store uuid; owner_id uuid; market uuid; country text; code text; currency text;
 quote_row record; destination_row record; allocation_row record; service_row record;
 version bigint; service_version bigint; warehouse_ids uuid[]; tracked_ids uuid[]; warehouse_id uuid; sku_id uuid;
 demands jsonb:='[]'::jsonb; balances jsonb:='[]'::jsonb; allocation jsonb; stock record; warehouse record;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 -- Calls ops.order_attempt_execution_context, order-attempt-v1 §§1/3: no mutable caller identity or money.
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 binding:=ctx->'binding'; catalog:=ctx->'plan'->'facts'->'catalog'; lines:=catalog->'lines'; service:=catalog->'service';
 tenant:=(binding->>'tenant_id')::uuid; store:=(binding->>'store_id')::uuid; owner_id:=(binding->>'buyer_owner_id')::uuid;
 market:=(service->>'market_id')::uuid; country:=service->>'country'; code:=service->>'code'; currency:=catalog->'market'->>'currency';
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id','',true),set_config('app.buyer_id',owner_id::text,true),
  set_config('app.buyer_session_id','',true),set_config('app.order_attempt_id',p_attempt::text,true);
 -- Under 0182, a visible exact PENDING cart cannot have committed in an earlier transaction.
 IF NOT EXISTS(SELECT 1 FROM storefront.carts c WHERE c.tenant_id=tenant AND c.store_id=store AND c.owner_id=owner_id
  AND c.id=p_cart AND c.cart_kind='order_attempt' AND c.create_attempt_id=p_attempt AND c.creator_session_id IS NULL
  AND c.version=1 AND c.currency=currency) THEN
  RAISE EXCEPTION 'attempt_cart_conflict' USING ERRCODE='PT409';
 END IF;
 SELECT q.* INTO quote_row FROM storefront.quotes q WHERE q.tenant_id=tenant AND q.store_id=store AND q.owner_id=owner_id
  AND q.id=p_quote AND q.cart_id=p_cart AND q.cart_version=1 AND q.creator_kind='order_attempt'
  AND q.create_attempt_id=p_attempt AND q.creator_session_id IS NULL;
 IF NOT FOUND OR quote_row.market_id IS DISTINCT FROM market OR quote_row.country IS DISTINCT FROM country
  OR quote_row.currency IS DISTINCT FROM currency OR quote_row.snapshot->'amount' IS DISTINCT FROM ctx->'plan'->'calculation' THEN
  RAISE EXCEPTION 'attempt_quote_conflict' USING ERRCODE='PT409';
 END IF;
 SELECT d.* INTO destination_row FROM storefront.destination_snapshots d WHERE d.tenant_id=tenant AND d.store_id=store
  AND d.owner_id=owner_id AND d.id=p_destination AND d.cart_id=p_cart AND d.cart_version=1 AND d.version=1
  AND d.creator_kind='order_attempt' AND d.create_attempt_id=p_attempt AND d.creator_session_id IS NULL;
 IF NOT FOUND OR destination_row.country IS DISTINCT FROM country OR destination_row.kind IS DISTINCT FROM service->>'delivery_kind'
  OR NOT EXISTS(SELECT 1 FROM storefront.destination_heads h WHERE h.tenant_id=tenant AND h.store_id=store
   AND h.owner_id=owner_id AND h.cart_id=p_cart AND h.destination_id=p_destination AND h.current_version=1) THEN
  RAISE EXCEPTION 'attempt_destination_conflict' USING ERRCODE='PT409';
 END IF;
 -- The fixed coordinator already called lock_catalog before 0181 took service/pickup locks.
 -- Re-read those retained safety facts without reacquiring an earlier lock level.
 IF NOT EXISTS(SELECT 1 FROM control.stores s WHERE s.tenant_id=tenant AND s.id=store AND s.active) THEN
  RAISE EXCEPTION 'store_unavailable' USING ERRCODE='PT404';
 END IF;
 IF EXISTS(SELECT 1 FROM jsonb_array_elements(lines) x LEFT JOIN catalog.skus s
   ON s.tenant_id=tenant AND s.store_id=store AND s.id=(x->>'sku_id')::uuid
   LEFT JOIN catalog.products p ON p.tenant_id=tenant AND p.store_id=store AND p.id=s.product_id
   WHERE s.id IS NULL OR s.status<>'active' OR p.status IS DISTINCT FROM 'active'
    OR s.product_id IS DISTINCT FROM (x->>'product_id')::uuid OR s.currency IS DISTINCT FROM currency) THEN
  RAISE EXCEPTION 'sku_unavailable' USING ERRCODE='PT422';
 END IF;
 IF EXISTS(SELECT 1 FROM (SELECT (x->>'sku_id')::uuid id,sum((x->>'quantity')::bigint) quantity
   FROM jsonb_array_elements(lines) x GROUP BY (x->>'sku_id')::uuid) d JOIN catalog.skus s
   ON s.tenant_id=tenant AND s.store_id=store AND s.id=d.id
   WHERE d.quantity NOT BETWEEN 1 AND 1000000000 OR (NOT s.inventory_tracked AND (s.max_per_order IS NULL OR d.quantity>s.max_per_order))) THEN
  RAISE EXCEPTION 'max_per_order_exceeded' USING ERRCODE='PT422';
 END IF;
 SELECT h.current_version INTO service_version FROM fulfillment.service_heads h WHERE h.tenant_id=tenant AND h.store_id=store
  AND h.market_id=market AND h.country=country AND h.code=code;
 SELECT s.enabled,s.delivery_kind,s.mode,s.policy_method INTO service_row FROM fulfillment.service_versions s
  WHERE s.tenant_id=tenant AND s.store_id=store AND s.market_id=market AND s.country=country AND s.code=code AND s.version=service_version;
 IF NOT FOUND OR NOT service_row.enabled OR service_row.delivery_kind IS DISTINCT FROM service->>'delivery_kind'
  OR service_row.mode IS DISTINCT FROM service->>'mode' OR service_row.policy_method IS DISTINCT FROM service->>'policy_method' THEN
  RAISE EXCEPTION 'delivery_service_unavailable' USING ERRCODE='PT422';
 END IF;
 SELECT h.current_version INTO version FROM fulfillment.allocation_heads h WHERE h.tenant_id=tenant AND h.store_id=store
  AND h.market_id=market AND h.country=country AND h.code=code FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'allocation_unavailable' USING ERRCODE='PT409'; END IF;
 SELECT a.* INTO allocation_row FROM fulfillment.allocation_versions a WHERE a.tenant_id=tenant AND a.store_id=store
  AND a.market_id=market AND a.country=country AND a.code=code AND a.version=version;
 IF NOT FOUND OR allocation_row.warehouse_count NOT BETWEEN 1 AND 16 THEN
  RAISE EXCEPTION 'allocation_unavailable' USING ERRCODE='PT409';
 END IF;
 SELECT array_agg(a.warehouse_id ORDER BY a.position) INTO warehouse_ids FROM fulfillment.allocation_warehouses a
  WHERE a.tenant_id=tenant AND a.store_id=store AND a.market_id=market AND a.country=country AND a.code=code AND a.version=version;
 IF cardinality(warehouse_ids) IS DISTINCT FROM allocation_row.warehouse_count::integer THEN
  RAISE EXCEPTION 'allocation_unavailable' USING ERRCODE='PT409';
 END IF;
 -- Calls inventory.lock_warehouse, then inventory.lock_balance. Business priority is preserved separately.
 FOR warehouse_id IN SELECT x FROM unnest(warehouse_ids) x ORDER BY x LOOP
  SELECT w.* INTO warehouse FROM inventory.lock_warehouse(warehouse_id) w;
  IF NOT FOUND OR NOT warehouse.active THEN RAISE EXCEPTION 'warehouse_unavailable' USING ERRCODE='PT409'; END IF;
 END LOOP;
 SELECT coalesce(jsonb_agg(jsonb_build_object('sku_id',d.id,'quantity',d.quantity) ORDER BY d.id),'[]'::jsonb),
  coalesce(array_agg(d.id ORDER BY d.id),'{}'::uuid[]) INTO demands,tracked_ids
  FROM (SELECT s.id,sum((x->>'quantity')::bigint) quantity FROM jsonb_array_elements(lines) x
   JOIN catalog.skus s ON s.tenant_id=tenant AND s.store_id=store AND s.id=(x->>'sku_id')::uuid AND s.inventory_tracked GROUP BY s.id) d;
 FOR warehouse_id IN SELECT x FROM unnest(warehouse_ids) x ORDER BY x LOOP
  FOREACH sku_id IN ARRAY tracked_ids LOOP
   SELECT b.* INTO stock FROM inventory.lock_balance(warehouse_id,sku_id) b;
   IF FOUND THEN
    balances:=balances||jsonb_build_array(jsonb_build_object('warehouse_id',stock.warehouse_id,'sku_id',stock.sku_id,
     'on_hand',stock.on_hand,'reserved',stock.reserved,'allocated',stock.allocated,'unavailable',stock.unavailable,'version',stock.version));
   END IF;
  END LOOP;
 END LOOP;
 allocation:=jsonb_build_object('market_id',market,'country',country,'code',code,'version',version,
  'service_version',allocation_row.service_version,'warehouse_ids',to_jsonb(warehouse_ids));
 result:=jsonb_build_object('order_id',p_attempt,'warehouse_ids',to_jsonb(warehouse_ids),'demands',demands,'balances',balances,'allocation',allocation);
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
ALTER FUNCTION checkout.order_attempt_prepare_allocation(uuid,text,uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_attempt_prepare_allocation(uuid,text,uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.order_attempt_prepare_allocation(uuid,text,uuid,uuid,uuid) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION checkout.order_attempt_prepare_allocation(uuid,text,uuid,uuid,uuid) IS
 'Private post-destination C2 allocation facts: exact current-xid plan/cart/quote/destination provenance; caller retains earlier owner/claim/order/catalog/service locks. Locks current allocation, sorted warehouses and warehouse/SKU balances; returns priority warehouses, aggregate tracked demands and native balances for Go PlanAllocation. No stock writes, SQL allocation, price drift refusal or earlier-level lock acquisition. EXECUTE coordinator only.';
