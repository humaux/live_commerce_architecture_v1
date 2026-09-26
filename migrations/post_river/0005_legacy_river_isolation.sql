-- See legacy-runtime-isolation-v1: one owner cutover, never a worker-side copy.
-- Fixed ordering also excludes inserts/fetch/maintenance between proof and move.
LOCK TABLE river.river_job IN ACCESS EXCLUSIVE MODE;
LOCK TABLE river.river_queue IN ACCESS EXCLUSIVE MODE;
LOCK TABLE river_payment.river_job IN ACCESS EXCLUSIVE MODE;
LOCK TABLE river_payment.river_queue IN ACCESS EXCLUSIVE MODE;
LOCK TABLE river_expiry.river_job IN ACCESS EXCLUSIVE MODE;
LOCK TABLE river_expiry.river_queue IN ACCESS EXCLUSIVE MODE;

DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM river_payment.river_job)
  OR EXISTS(SELECT 1 FROM river_expiry.river_job) THEN
  RAISE EXCEPTION 'legacy family destination not empty' USING ERRCODE='55000';
 END IF;
 IF EXISTS(SELECT 1 FROM river.river_job j
  WHERE (j.kind IN ('payment_query_v1','payment_reconcile_v1','checkout_expiry_v1')
   OR j.queue IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1','checkout_expiry_v1'))
   AND j.state='running') THEN
  RAISE EXCEPTION 'legacy family cutover requires drain' USING ERRCODE='55000';
 END IF;
 IF EXISTS(SELECT 1 FROM river.river_job j
  WHERE (j.kind IN ('payment_query_v1','payment_reconcile_v1')
   OR j.queue IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1'))
   AND (j.kind NOT IN ('payment_query_v1','payment_reconcile_v1')
    OR j.unique_key IS NOT NULL OR j.args->>'version' IS DISTINCT FROM '1'
    OR integration.payment_job_queue(j.id) IS NULL
    OR NOT (j.queue=integration.payment_job_queue(j.id)
     OR (j.queue='default' AND j.state IN ('completed','cancelled','discarded')))))
 OR EXISTS(SELECT 1 FROM river.river_job j
  WHERE (j.kind='checkout_expiry_v1' OR j.queue='checkout_expiry_v1')
   AND (j.kind<>'checkout_expiry_v1' OR j.unique_key IS NOT NULL
    OR NOT checkout.expiry_job_linked(j.id)
    OR NOT (j.queue='checkout_expiry_v1'
     OR (j.queue='default' AND j.state IN ('completed','cancelled','discarded'))))) THEN
  RAISE EXCEPTION 'legacy family cutover invalid source' USING ERRCODE='22023';
 END IF;
 IF EXISTS(SELECT 1 FROM river_payment.river_queue q
  WHERE q.name NOT IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1')
   OR NOT EXISTS(SELECT 1 FROM river.river_queue old WHERE old.name=q.name AND to_jsonb(old)=to_jsonb(q)))
 OR EXISTS(SELECT 1 FROM river_expiry.river_queue q WHERE q.name<>'checkout_expiry_v1'
   OR NOT EXISTS(SELECT 1 FROM river.river_queue old WHERE old.name=q.name AND to_jsonb(old)=to_jsonb(q))) THEN
  RAISE EXCEPTION 'legacy family destination queue conflict' USING ERRCODE='22023';
 END IF;
END $$;

-- Native schemas have distinct enum OIDs. Full JSON row equality below catches
-- an upstream-added column that the explicit copy list fails to preserve.
INSERT INTO river_payment.river_job
 (id,state,attempt,max_attempts,attempted_at,created_at,finalized_at,scheduled_at,
  priority,args,attempted_by,errors,kind,metadata,queue,tags,unique_key,unique_states)
SELECT id,state::text::river_payment.river_job_state,attempt,max_attempts,attempted_at,
 created_at,finalized_at,scheduled_at,priority,args,attempted_by,errors,kind,metadata,queue,tags,unique_key,unique_states
FROM river.river_job WHERE kind IN ('payment_query_v1','payment_reconcile_v1');
INSERT INTO river_expiry.river_job
 (id,state,attempt,max_attempts,attempted_at,created_at,finalized_at,scheduled_at,
  priority,args,attempted_by,errors,kind,metadata,queue,tags,unique_key,unique_states)
SELECT id,state::text::river_expiry.river_job_state,attempt,max_attempts,attempted_at,
 created_at,finalized_at,scheduled_at,priority,args,attempted_by,errors,kind,metadata,queue,tags,unique_key,unique_states
FROM river.river_job WHERE kind='checkout_expiry_v1';
INSERT INTO river_payment.river_queue(name,created_at,metadata,paused_at,updated_at)
 SELECT name,created_at,metadata,paused_at,updated_at FROM river.river_queue
 WHERE name IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1') ON CONFLICT(name) DO NOTHING;
INSERT INTO river_expiry.river_queue(name,created_at,metadata,paused_at,updated_at)
 SELECT name,created_at,metadata,paused_at,updated_at FROM river.river_queue
 WHERE name='checkout_expiry_v1' ON CONFLICT(name) DO NOTHING;

DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM river.river_job old LEFT JOIN river_payment.river_job new ON new.id=old.id
  WHERE old.kind IN ('payment_query_v1','payment_reconcile_v1') AND to_jsonb(old) IS DISTINCT FROM to_jsonb(new))
 OR EXISTS(SELECT 1 FROM river.river_job old LEFT JOIN river_expiry.river_job new ON new.id=old.id
  WHERE old.kind='checkout_expiry_v1' AND to_jsonb(old) IS DISTINCT FROM to_jsonb(new))
 OR (SELECT count(*) FROM river_payment.river_job)<>(SELECT count(*) FROM river.river_job WHERE kind IN ('payment_query_v1','payment_reconcile_v1'))
 OR (SELECT count(*) FROM river_expiry.river_job)<>(SELECT count(*) FROM river.river_job WHERE kind='checkout_expiry_v1')
 OR EXISTS(SELECT 1 FROM river.river_queue old LEFT JOIN river_payment.river_queue new ON new.name=old.name
  WHERE old.name IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1') AND to_jsonb(old) IS DISTINCT FROM to_jsonb(new))
 OR EXISTS(SELECT 1 FROM river.river_queue old LEFT JOIN river_expiry.river_queue new ON new.name=old.name
  WHERE old.name='checkout_expiry_v1' AND to_jsonb(old) IS DISTINCT FROM to_jsonb(new)) THEN
  RAISE EXCEPTION 'legacy family cutover row mismatch' USING ERRCODE='22023';
 END IF;
 -- setval is not transactional: a retry may leave a gap but cannot recycle a
 -- copied or pruned ID. Different families may subsequently share an ID value.
 PERFORM setval('river_payment.river_job_id_seq',greatest(
  (SELECT last_value FROM river.river_job_id_seq),
  (SELECT last_value FROM river_payment.river_job_id_seq),
  coalesce((SELECT max(id) FROM river_payment.river_job),1),
  coalesce((SELECT max(job_id) FROM checkout.payment_attempts),1),
  coalesce((SELECT max(job_id) FROM integration.operations),1)),true);
 PERFORM setval('river_expiry.river_job_id_seq',greatest(
  (SELECT last_value FROM river.river_job_id_seq),
  (SELECT last_value FROM river_expiry.river_job_id_seq),
  coalesce((SELECT max(id) FROM river_expiry.river_job),1),
  coalesce((SELECT max(job_id) FROM checkout.orders),1)),true);
END $$;

-- Static forward copies below change only the referenced native job table.
-- Keep earlier SQL/checksums immutable; never execute mutable pg_get_functiondef.

CREATE OR REPLACE FUNCTION checkout.begin_hold(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,
 p_order uuid,p_snapshot jsonb,p_lines jsonb,p_job_id bigint) RETURNS jsonb
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_scope record; v_tenant uuid; v_owner uuid; v_session uuid;
 v_previous record; v_cart storefront.carts%ROWTYPE; v_quote storefront.quotes%ROWTYPE;
 v_dest storefront.destination_snapshots%ROWTYPE; v_market pricing.markets%ROWTYPE;
 v_policy pricing.policy_versions%ROWTYPE; v_service fulfillment.service_versions%ROWTYPE;
 v_allocation fulfillment.allocation_versions%ROWTYPE;
 v_source record; v_source_enabled boolean;
 v_head_version bigint; v_head_id uuid; v_line record; v_quote_line record;
 v_now timestamptz; v_expires timestamptz; v_result jsonb;
 v_count integer; v_distinct integer; v_mismatch integer;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
    OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
    OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_job_id IS NULL OR p_job_id<1
    OR p_snapshot IS NULL OR jsonb_typeof(p_snapshot)<>'object' OR octet_length(p_snapshot::text)>1048576
    OR p_lines IS NULL OR jsonb_typeof(p_lines)<>'array' OR octet_length(p_lines::text)>1048576
    OR jsonb_typeof(p_snapshot->'quote')<>'object'
    OR jsonb_typeof(p_snapshot->'destination')<>'object'
    OR jsonb_typeof(p_snapshot->'service')<>'object'
    OR jsonb_typeof(p_snapshot->'allocation')<>'object'
    OR (SELECT count(*) FROM jsonb_object_keys(p_snapshot))<>4 THEN
  RAISE EXCEPTION 'invalid checkout input' USING ERRCODE='PT400';
 END IF;
 SELECT * INTO v_scope FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 v_tenant:=v_scope.tenant_id; v_owner:=v_scope.owner_id; v_session:=v_scope.session_id;
 PERFORM set_config('app.tenant_id',v_tenant::text,true);
 PERFORM set_config('app.store_id',p_store::text,true);
 PERFORM set_config('app.buyer_id',v_owner::text,true);
 PERFORM set_config('app.buyer_session_id',v_session::text,true);
 PERFORM set_config('app.principal_id','',true);
 PERFORM pg_advisory_xact_lock(hashtextextended(
  'checkout.begin|'||v_tenant||'|'||p_store||'|'||v_owner||'|'||p_key,0));
 SELECT r.request_hash,r.response INTO v_previous FROM checkout.command_results r
  WHERE r.tenant_id=v_tenant AND r.store_id=p_store AND r.owner_id=v_owner
   AND r.operation='checkout.begin' AND r.idempotency_key=p_key;
 IF FOUND THEN
  -- Go resolves an authenticated replay before inserting the River job. A
  -- direct repeat at this writer boundary must abort any newly inserted job.
  RAISE EXCEPTION 'checkout request already recorded' USING ERRCODE='PT409';
 END IF;

 SELECT q.* INTO v_quote FROM storefront.quotes q WHERE q.tenant_id=v_tenant
  AND q.store_id=p_store AND q.owner_id=v_owner AND q.id=(p_snapshot->'quote'->>'id')::uuid;
 IF NOT FOUND OR v_quote.snapshot IS DISTINCT FROM p_snapshot->'quote' THEN
  RAISE EXCEPTION 'quote changed' USING ERRCODE='PT409'; END IF;
 SELECT c.* INTO v_cart FROM storefront.carts c WHERE c.tenant_id=v_tenant
  AND c.store_id=p_store AND c.owner_id=v_owner AND c.id=v_quote.cart_id FOR SHARE;
 IF NOT FOUND OR v_cart.version<>v_quote.cart_version OR v_cart.currency<>v_quote.currency
    OR NOT EXISTS(SELECT 1 FROM storefront.cart_lines l WHERE l.tenant_id=v_tenant
      AND l.store_id=p_store AND l.owner_id=v_owner AND l.cart_id=v_cart.id) THEN
  RAISE EXCEPTION 'cart changed' USING ERRCODE='PT409'; END IF;
 IF EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=v_tenant AND o.store_id=p_store
  AND o.owner_id=v_owner AND o.cart_id=v_cart.id AND o.cart_version=v_cart.version
  AND o.commercial_state IN ('DRAFT','AWAITING_PAYMENT','CONFIRMED')) THEN
  RAISE EXCEPTION 'active checkout exists' USING ERRCODE='PT409'; END IF;
 SELECT m.* INTO v_market FROM pricing.markets m WHERE m.tenant_id=v_tenant
  AND m.store_id=p_store AND m.id=v_quote.market_id FOR SHARE;
 IF NOT FOUND OR NOT v_market.active OR v_market.currency<>v_quote.currency
    OR v_market.version<>v_quote.market_version THEN
  RAISE EXCEPTION 'market changed' USING ERRCODE='PT409'; END IF;
 SELECT h.current_version INTO v_head_version FROM pricing.policy_heads h
  WHERE h.tenant_id=v_tenant AND h.store_id=p_store AND h.market_id=v_quote.market_id
   AND h.country=v_quote.country AND h.method=v_quote.method FOR SHARE;
 IF NOT FOUND OR v_head_version<>v_quote.policy_version THEN
  RAISE EXCEPTION 'policy changed' USING ERRCODE='PT409'; END IF;
 SELECT p.* INTO v_policy FROM pricing.policy_versions p WHERE p.tenant_id=v_tenant
  AND p.store_id=p_store AND p.market_id=v_quote.market_id AND p.country=v_quote.country
  AND p.method=v_quote.method AND p.version=v_quote.policy_version;
 IF NOT FOUND OR NOT v_policy.enabled OR v_policy.currency<>v_quote.currency
    OR p_snapshot->'quote'->'policy'->>'method'<>v_quote.method THEN
  RAISE EXCEPTION 'policy changed' USING ERRCODE='PT409'; END IF;

 SELECT d.* INTO v_dest FROM storefront.destination_snapshots d WHERE d.tenant_id=v_tenant
  AND d.store_id=p_store AND d.owner_id=v_owner AND d.id=(p_snapshot->'destination'->>'id')::uuid;
 IF NOT FOUND OR v_dest.cart_id<>v_cart.id OR v_dest.cart_version<>v_cart.version
    OR v_dest.country<>v_quote.country OR v_dest.kind IS DISTINCT FROM p_snapshot->'destination'->>'kind'
    OR p_snapshot->'destination'->>'cart_id' IS DISTINCT FROM v_cart.id::text
    OR p_snapshot->'destination'->>'country' IS DISTINCT FROM v_dest.country
    OR p_snapshot->'destination'->>'recipient_name' IS DISTINCT FROM v_dest.recipient_name
    OR p_snapshot->'destination'->>'phone' IS DISTINCT FROM v_dest.phone
    OR p_snapshot->'destination'->'home_address' IS DISTINCT FROM
     jsonb_build_object('region',v_dest.region,'city',v_dest.city,'postal_code',v_dest.postal_code,
      'line1',v_dest.line1,'line2',v_dest.line2) THEN
  RAISE EXCEPTION 'destination changed' USING ERRCODE='PT409'; END IF;
 IF v_dest.pickup_id IS NOT NULL THEN
  SELECT p.id,p.kind,p.namespace,p.code,p.version,p.country,p.verification_kind,
   p.attested_at,p.valid_until INTO v_source FROM fulfillment.pickup_versions p WHERE p.tenant_id=v_tenant
   AND p.store_id=p_store AND p.id=v_dest.pickup_id;
  IF NOT FOUND OR v_source.kind<>v_dest.kind OR v_source.country<>v_dest.country
     OR v_source.verification_kind<>'MANUAL_ATTESTED'
     OR p_snapshot->'destination'->'pickup'->>'id' IS DISTINCT FROM v_source.id::text THEN
   RAISE EXCEPTION 'pickup source changed' USING ERRCODE='PT409'; END IF;
  SELECT h.pickup_id,h.current_version,h.enabled INTO v_head_id,v_head_version,v_source_enabled
   FROM fulfillment.pickup_heads h WHERE h.tenant_id=v_tenant AND h.store_id=p_store
    AND h.kind=v_source.kind AND h.namespace=v_source.namespace AND h.code=v_source.code FOR SHARE;
  IF NOT FOUND OR NOT v_source_enabled OR v_head_id<>v_source.id OR v_head_version<>v_source.version THEN
   RAISE EXCEPTION 'pickup source changed' USING ERRCODE='PT409'; END IF;
 ELSIF v_dest.kind<>'home' THEN
  RAISE EXCEPTION 'pickup source missing' USING ERRCODE='PT409';
 END IF;
 SELECT h.destination_id,h.current_version INTO v_head_id,v_head_version
  FROM storefront.destination_heads h WHERE h.tenant_id=v_tenant AND h.store_id=p_store
   AND h.owner_id=v_owner AND h.cart_id=v_cart.id FOR SHARE;
 IF NOT FOUND OR v_head_id<>v_dest.id OR v_head_version<>v_dest.version
    OR (p_snapshot->'destination'->>'version')::bigint IS DISTINCT FROM v_dest.version THEN
  RAISE EXCEPTION 'destination changed' USING ERRCODE='PT409'; END IF;

 IF v_quote.method NOT LIKE 'delivery:%' THEN
  RAISE EXCEPTION 'delivery service missing' USING ERRCODE='PT409'; END IF;
 SELECT h.current_version INTO v_head_version FROM fulfillment.service_heads h
  WHERE h.tenant_id=v_tenant AND h.store_id=p_store AND h.market_id=v_quote.market_id
   AND h.country=v_quote.country AND h.code=substring(v_quote.method FROM 10) FOR SHARE;
 IF NOT FOUND OR v_head_version IS DISTINCT FROM (p_snapshot->'service'->>'version')::bigint THEN
  RAISE EXCEPTION 'service changed' USING ERRCODE='PT409'; END IF;
 SELECT s.* INTO v_service FROM fulfillment.service_versions s WHERE s.tenant_id=v_tenant
  AND s.store_id=p_store AND s.market_id=v_quote.market_id AND s.country=v_quote.country
  AND s.code=substring(v_quote.method FROM 10) AND s.version=v_head_version;
 IF NOT FOUND OR NOT v_service.enabled OR NOT v_service.visible OR v_service.mode<>'MANUAL'
    OR v_service.delivery_kind<>v_dest.kind OR v_service.currency<>v_quote.currency
    OR v_service.policy_version<>v_quote.policy_version
    OR p_snapshot->'service'->>'code' IS DISTINCT FROM v_service.code
    OR p_snapshot->'service'->>'market_id' IS DISTINCT FROM v_service.market_id::text
    OR p_snapshot->'service'->>'country' IS DISTINCT FROM v_service.country THEN
  RAISE EXCEPTION 'service changed' USING ERRCODE='PT409'; END IF;
 SELECT h.current_version INTO v_head_version FROM fulfillment.allocation_heads h
  WHERE h.tenant_id=v_tenant AND h.store_id=p_store AND h.market_id=v_quote.market_id
   AND h.country=v_quote.country AND h.code=v_service.code FOR SHARE;
 IF NOT FOUND OR v_head_version IS DISTINCT FROM (p_snapshot->'allocation'->>'version')::bigint THEN
  RAISE EXCEPTION 'allocation changed' USING ERRCODE='PT409'; END IF;
 SELECT a.* INTO v_allocation FROM fulfillment.allocation_versions a WHERE a.tenant_id=v_tenant
  AND a.store_id=p_store AND a.market_id=v_quote.market_id AND a.country=v_quote.country
  AND a.code=v_service.code AND a.version=v_head_version;
 IF NOT FOUND OR v_allocation.warehouse_count<1
    OR p_snapshot->'allocation'->>'market_id' IS DISTINCT FROM v_allocation.market_id::text
    OR p_snapshot->'allocation'->>'country' IS DISTINCT FROM v_allocation.country
    OR p_snapshot->'allocation'->>'code' IS DISTINCT FROM v_allocation.code
    OR p_snapshot->'allocation'->'warehouse_ids' IS DISTINCT FROM
     (SELECT jsonb_agg(a.warehouse_id::text ORDER BY a.position)
       FROM fulfillment.allocation_warehouses a WHERE a.tenant_id=v_tenant AND a.store_id=p_store
        AND a.market_id=v_allocation.market_id AND a.country=v_allocation.country
        AND a.code=v_allocation.code AND a.version=v_allocation.version) THEN
  RAISE EXCEPTION 'allocation unavailable' USING ERRCODE='PT409'; END IF;

 -- Structural plan and exact SKU conservation only. Go's locked, current
 -- price calculation remains the one monetary authority.
 SELECT count(*),count(DISTINCT (l.warehouse_id,l.sku_id)) INTO v_count,v_distinct
  FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint);
 IF v_count NOT BETWEEN 1 AND 800 OR v_count<>v_distinct THEN
  RAISE EXCEPTION 'invalid stock plan' USING ERRCODE='PT400'; END IF;
 SELECT count(*),count(DISTINCT q.sku_id) INTO v_count,v_distinct
  FROM jsonb_to_recordset(v_quote.snapshot->'lines') AS q(sku_id uuid,quantity bigint);
 IF v_count NOT BETWEEN 1 AND 50 OR v_count<>v_distinct THEN
  RAISE EXCEPTION 'invalid quote lines' USING ERRCODE='PT400'; END IF;
 SELECT count(*) INTO v_mismatch FROM (
  WITH demand AS (SELECT q.sku_id,sum(q.quantity) AS quantity
    FROM jsonb_to_recordset(v_quote.snapshot->'lines') AS q(sku_id uuid,quantity bigint)
    GROUP BY q.sku_id),
   plan AS (SELECT l.sku_id,sum(l.quantity) AS quantity
    FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint)
    GROUP BY l.sku_id)
  SELECT 1 FROM demand d FULL JOIN plan p USING(sku_id)
   WHERE d.sku_id IS NULL OR p.sku_id IS NULL OR d.quantity IS DISTINCT FROM p.quantity) mismatch;
 IF v_mismatch<>0 THEN RAISE EXCEPTION 'stock plan differs from quote' USING ERRCODE='PT409'; END IF;
 FOR v_line IN SELECT l.warehouse_id,l.sku_id,l.quantity
  FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint)
  ORDER BY l.warehouse_id,l.sku_id LOOP
  IF v_line.warehouse_id IS NULL OR v_line.sku_id IS NULL
     OR v_line.quantity IS NULL OR v_line.quantity NOT BETWEEN 1 AND 1000000000
     OR NOT EXISTS(SELECT 1 FROM fulfillment.allocation_warehouses a WHERE a.tenant_id=v_tenant
      AND a.store_id=p_store AND a.market_id=v_quote.market_id AND a.country=v_quote.country
      AND a.code=v_service.code AND a.version=v_allocation.version
      AND a.warehouse_id=v_line.warehouse_id)
     OR NOT EXISTS(SELECT 1 FROM inventory.warehouses w WHERE w.tenant_id=v_tenant
      AND w.store_id=p_store AND w.id=v_line.warehouse_id AND w.active) THEN
   RAISE EXCEPTION 'invalid stock plan' USING ERRCODE='PT409'; END IF;
  IF NOT EXISTS(SELECT 1 FROM inventory.balances b WHERE b.tenant_id=v_tenant
    AND b.store_id=p_store AND b.warehouse_id=v_line.warehouse_id AND b.sku_id=v_line.sku_id
    AND b.on_hand-b.reserved-b.allocated-b.unavailable>=v_line.quantity) THEN
   RAISE EXCEPTION 'insufficient stock' USING ERRCODE='PT402'; END IF;
 END LOOP;
 FOR v_quote_line IN SELECT q.sku_id,q.product_id,q.quantity,q.unit_price_minor
  FROM jsonb_to_recordset(v_quote.snapshot->'lines')
   AS q(sku_id uuid,product_id uuid,quantity bigint,unit_price_minor bigint) LOOP
  IF v_quote_line.sku_id IS NULL OR v_quote_line.product_id IS NULL
     OR v_quote_line.quantity IS NULL OR v_quote_line.quantity NOT BETWEEN 1 AND 1000000000
     OR v_quote_line.unit_price_minor IS NULL OR v_quote_line.unit_price_minor NOT BETWEEN 0 AND 1000000000000 THEN
   RAISE EXCEPTION 'invalid quote lines' USING ERRCODE='PT400'; END IF;
 END LOOP;

 -- Final time follows every row and advisory wait. An expiry job is a
 -- scheduled housekeeping task, never a runnable payment attempt.
 v_now:=clock_timestamp();
 SELECT * INTO v_scope FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND OR v_scope.tenant_id<>v_tenant OR v_scope.owner_id<>v_owner
    OR v_scope.session_id<>v_session THEN
  RAISE EXCEPTION 'buyer capability expired' USING ERRCODE='PT401'; END IF;
 IF v_quote.created_at>v_now OR v_quote.expires_at<=v_now
    OR v_dest.selected_at>v_now OR v_dest.expires_at<=v_now THEN
  RAISE EXCEPTION 'checkout source expired' USING ERRCODE='PT409'; END IF;
 IF v_dest.pickup_id IS NOT NULL THEN
  IF v_source.attested_at>v_dest.selected_at OR v_source.attested_at>v_now
     OR v_source.valid_until<=v_now OR v_dest.expires_at>v_source.valid_until THEN
   RAISE EXCEPTION 'pickup source expired' USING ERRCODE='PT409'; END IF;
 END IF;
 IF NOT EXISTS(SELECT 1 FROM river_expiry.river_job j WHERE j.id=p_job_id
  AND j.kind='checkout_expiry_v1' AND j.args->>'order_id'=p_order::text
  AND j.args->>'generation'='1' AND j.args->>'version'='1') THEN
  RAISE EXCEPTION 'expiry job missing' USING ERRCODE='PT409'; END IF;
 v_expires:=v_now+interval '15 minutes';
 v_result:=jsonb_build_object('order_id',p_order::text,'reservation_id',p_order::text,
  'generation',1,'expires_at',v_expires,'job_id',p_job_id);
 INSERT INTO checkout.orders(tenant_id,store_id,owner_id,id,creator_session_id,cart_id,cart_version,
  quote_id,destination_id,market_id,country,service_code,service_version,allocation_version,
  currency,total_minor,commercial_state,fulfillment_state,generation,expires_at,job_id,snapshot,created_at,updated_at)
 VALUES(v_tenant,p_store,v_owner,p_order,v_session,v_cart.id,v_cart.version,v_quote.id,v_dest.id,
  v_quote.market_id,v_quote.country,v_service.code,v_service.version,v_allocation.version,
  v_quote.currency,(v_quote.snapshot->'amount'->>'total_minor')::bigint,
  'DRAFT','MANUAL_UNASSIGNED',1,v_expires,p_job_id,p_snapshot,v_now,v_now);
 INSERT INTO inventory.reservations(tenant_id,store_id,id,state,expires_at,
  checkout_id,buyer_owner_id,buyer_session_id,generation)
 VALUES(v_tenant,p_store,p_order,'HELD',v_expires,p_order,v_owner,v_session,1);
 FOR v_line IN SELECT l.warehouse_id,l.sku_id,l.quantity
  FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint)
  ORDER BY l.warehouse_id,l.sku_id LOOP
  INSERT INTO inventory.reservation_lines(tenant_id,store_id,reservation_id,warehouse_id,sku_id,quantity)
   VALUES(v_tenant,p_store,p_order,v_line.warehouse_id,v_line.sku_id,v_line.quantity);
  INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,
   operation,command_key,reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
   VALUES(v_tenant,p_store,v_line.warehouse_id,v_line.sku_id,'RESERVE',v_line.quantity,
    'checkout.begin',p_order::text,p_order,NULL,p_order,v_owner,v_session,'BUYER');
 END LOOP;
 INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
 VALUES(v_tenant,p_store,v_owner,p_order,v_session,1,'checkout.held','BUYER');
 INSERT INTO checkout.command_results(tenant_id,store_id,owner_id,operation,idempotency_key,
  creator_session_id,request_hash,order_id,response)
 VALUES(v_tenant,p_store,v_owner,'checkout.begin',p_key,v_session,p_request_hash,p_order,v_result);
 RETURN v_result;
EXCEPTION WHEN invalid_text_representation OR numeric_value_out_of_range
 OR invalid_datetime_format THEN
 RAISE EXCEPTION 'invalid checkout input' USING ERRCODE='PT400';
END $$;

CREATE OR REPLACE FUNCTION checkout.start_payment(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,
 p_order uuid,p_method text,p_version bigint,p_profile text,p_attempt uuid,p_job bigint) RETURNS jsonb
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; s_final record; o checkout.orders%ROWTYPE; r inventory.reservations%ROWTYPE;
 m payments.method_versions%ROWTYPE; a integration.merchant_accounts%ROWTYPE;
 b integration.bindings%ROWTYPE; q payments.account_qualifications%ROWTYPE;
 v_version bigint; v_active boolean; v_now timestamptz; v_trade text; v_result jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
 OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
 OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_attempt IS NULL
 OR p_job IS NULL OR p_job<1 OR p_method IS DISTINCT FROM 'payuni_credit'
 OR p_version IS NULL OR p_version<1 OR p_profile IS NULL OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE') THEN
  RAISE EXCEPTION 'invalid payment input' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true);
 PERFORM set_config('app.store_id',p_store::text,true);
 PERFORM set_config('app.buyer_id',s.owner_id::text,true);
 PERFORM set_config('app.buyer_session_id',s.session_id::text,true);
 PERFORM set_config('app.principal_id','',true);
 PERFORM pg_advisory_xact_lock(hashtextextended('checkout.payment.start|'||s.tenant_id||'|'||p_store||'|'||s.owner_id||'|'||p_key,0));
 IF EXISTS(SELECT 1 FROM checkout.command_results x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
 AND x.owner_id=s.owner_id AND x.operation='checkout.payment.start' AND x.idempotency_key=p_key) THEN
  RAISE EXCEPTION 'payment already recorded' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO o FROM checkout.orders x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
 AND x.owner_id=s.owner_id AND x.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO r FROM inventory.reservations x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
 IF NOT FOUND OR o.commercial_state<>'DRAFT' OR r.state<>'HELD' OR r.checkout_id<>o.id
 OR r.buyer_owner_id<>o.owner_id OR r.buyer_session_id<>o.creator_session_id OR r.generation<>o.generation THEN
  RAISE EXCEPTION 'order hold unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.active INTO v_active FROM pricing.markets x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
 AND x.id=o.market_id AND x.currency=o.currency FOR SHARE;
 IF NOT FOUND OR NOT v_active THEN RAISE EXCEPTION 'payment market unavailable' USING ERRCODE='PT409'; END IF;
 SELECT h.current_version INTO v_version FROM payments.method_heads h WHERE h.tenant_id=s.tenant_id AND h.store_id=p_store
 AND h.market_id=o.market_id AND h.country=o.country AND h.code=p_method FOR SHARE;
 IF NOT FOUND OR v_version<>p_version THEN RAISE EXCEPTION 'payment method changed' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO m FROM payments.method_versions x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
 AND x.market_id=o.market_id AND x.country=o.country AND x.code=p_method AND x.version=p_version;
 IF NOT FOUND OR NOT m.enabled OR NOT m.visible OR m.connection_id IS NULL OR m.qualification_id IS NULL
 OR o.currency<>'TWD' OR o.total_minor NOT BETWEEN 100 AND 19999900 OR o.total_minor%100<>0
 OR o.total_minor<m.min_amount_minor OR o.total_minor>m.max_amount_minor THEN
  RAISE EXCEPTION 'payment method unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO a FROM integration.merchant_accounts x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=m.connection_id FOR SHARE;
 IF NOT FOUND OR a.provider<>'payuni' OR a.environment<>m.environment THEN
  RAISE EXCEPTION 'payment account unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=a.binding_id FOR SHARE;
 IF NOT FOUND OR NOT b.enabled OR b.semantic_version<>m.binding_version OR b.provider<>a.provider OR b.external_asset_id<>a.binding_asset THEN
  RAISE EXCEPTION 'payment binding unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO q FROM payments.account_qualifications x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=m.qualification_id FOR SHARE;
 IF NOT FOUND OR q.connection_id<>a.id OR q.credential_version<>a.credential_version OR q.environment<>a.environment OR q.code<>p_method
 OR q.revoked_at IS NOT NULL OR q.proof_class<>(CASE WHEN p_profile='PROVIDER_MOCK' THEN p_profile ELSE 'REAL_'||p_profile END)
 OR (p_profile<>'PROVIDER_MOCK' AND p_profile<>a.environment) THEN
  RAISE EXCEPTION 'payment qualification unavailable' USING ERRCODE='PT409'; END IF;
 v_now:=clock_timestamp();
 IF o.expires_at<=v_now OR r.expires_at<=v_now OR q.observed_at>v_now OR q.expires_at<=v_now THEN
  RAISE EXCEPTION 'payment admission expired' USING ERRCODE='PT409'; END IF;
 IF NOT EXISTS(SELECT 1 FROM river_payment.river_job j WHERE j.id=p_job AND j.kind='payment_query_v1'
 AND j.args=jsonb_build_object('operation_id',p_attempt::text,'version',1)) THEN
  RAISE EXCEPTION 'payment query job missing' USING ERRCODE='PT409'; END IF;
 -- All UUID bits survive in this 23-character provider reference; no truncation.
 v_trade:='P'||rtrim(translate(encode(uuid_send(p_attempt),'base64'),'+/','-_'),'=');
 INSERT INTO checkout.payment_attempts(tenant_id,store_id,owner_id,id,session_id,order_id,market_id,country,
 method_code,method_version,connection_id,credential_version,qualification_id,environment,execution_profile,
 binding_id,binding_version,currency,amount_minor,merchant_trade_no,state,generation,job_id)
 VALUES(s.tenant_id,p_store,s.owner_id,p_attempt,s.session_id,p_order,o.market_id,o.country,p_method,p_version,
 a.id,a.credential_version,q.id,a.environment,p_profile,b.id,b.semantic_version,o.currency,o.total_minor,v_trade,'PAYMENT_PENDING',o.generation+1,p_job);
 INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,external_asset_id,
 purpose,action,semantic_key,request_hash,request,job_id,state,generation,actor_kind,payment_attempt_id,buyer_owner_id,buyer_session_id)
 VALUES(p_attempt,s.tenant_id,p_store,NULL,b.id,b.semantic_version,'payuni',b.external_asset_id,'transactional','payuni.query',
 'payment.query:'||p_attempt,p_request_hash,jsonb_build_object('attempt_id',p_attempt::text),p_job,'UNKNOWN',1,'BUYER_PAYMENT_QUERY',p_attempt,s.owner_id,s.session_id);
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(s.tenant_id,p_store,p_attempt,1,'UNKNOWN','','buyer_payment_started');
 UPDATE checkout.orders SET commercial_state='AWAITING_PAYMENT',generation=o.generation+1,updated_at=v_now WHERE id=p_order;
 -- Reservation provenance remains the original checkout session. The payment
 -- event/attempt separately retain the current authenticated session.
 PERFORM set_config('app.buyer_session_id',o.creator_session_id::text,true);
 UPDATE inventory.reservations SET state='PAYMENT_PENDING',generation=o.generation+1
 WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_order;
 PERFORM set_config('app.buyer_session_id',s.session_id::text,true);
 INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
 VALUES(s.tenant_id,p_store,s.owner_id,p_order,s.session_id,o.generation+1,'checkout.payment_started','BUYER');
 v_result:=jsonb_build_object('order_id',p_order::text,'attempt_id',p_attempt::text,'operation_id',p_attempt::text,
 'job_id',p_job,'generation',o.generation+1,'merchant_trade_no',v_trade,'currency',o.currency,'amount_minor',o.total_minor,'state','PAYMENT_PENDING');
 INSERT INTO checkout.command_results(tenant_id,store_id,owner_id,operation,idempotency_key,creator_session_id,request_hash,order_id,response)
 VALUES(s.tenant_id,p_store,s.owner_id,'checkout.payment.start',p_key,s.session_id,p_request_hash,p_order,v_result);
 -- Recheck after all SQL waits, including FK and fault-injection triggers.
 SELECT * INTO s_final FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND OR s_final.tenant_id<>s.tenant_id OR s_final.owner_id<>s.owner_id OR s_final.session_id<>s.session_id THEN
  RAISE EXCEPTION 'buyer capability expired' USING ERRCODE='PT401'; END IF;
 v_now:=clock_timestamp();
 IF o.expires_at<=v_now OR r.expires_at<=v_now OR q.expires_at<=v_now THEN
  RAISE EXCEPTION 'payment admission expired' USING ERRCODE='PT409'; END IF;
 RETURN v_result;
END $$;

CREATE OR REPLACE FUNCTION integration.record_payment_query(p_id uuid,p_generation bigint,p_token bytea,
 p_profile text,p_report jsonb,p_reconcile_job bigint)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a checkout.payment_attempts%ROWTYPE; v_ref text; v_lease timestamptz; v_hash bytea;
 v_day timestamp;
BEGIN
 a:=integration.require_payment_query(p_id,p_generation,p_token,p_profile);
 IF p_report IS NULL OR jsonb_typeof(p_report)<>'object' OR octet_length(p_report::text)>2048 THEN
  RAISE EXCEPTION 'invalid payment report' USING ERRCODE='22023'; END IF;
 IF NOT p_report ?& ARRAY['MerTradeNo','TradeNo','AmountTWD','PaymentType','TradeStatus','Status','AuthType','CardInst','DataSource','CloseStatus']
 OR (SELECT count(*) FROM jsonb_object_keys(p_report)) NOT BETWEEN 10 AND 16
 OR EXISTS(SELECT 1 FROM jsonb_each(p_report) e WHERE e.key NOT IN
  ('MerTradeNo','TradeNo','AmountTWD','PaymentType','TradeStatus','Status','AuthType','CardInst','DataSource','CloseStatus',
   'CloseAmountTWD','CardRefundType','CardRefundStatus','CardRefundAmountTWD','CardRefundDay','CardRemainAmountTWD'))
 OR EXISTS(SELECT 1 FROM jsonb_each(p_report) e WHERE
  (e.key IN ('AmountTWD','CardInst','CloseAmountTWD','CardRefundAmountTWD','CardRemainAmountTWD')
    AND (jsonb_typeof(e.value)<>'number' OR e.value::text !~ '^(0|[1-9][0-9]{0,5})$'
      OR (e.key<>'CardInst' AND (e.value::text)::bigint>199999)))
  OR (e.key NOT IN ('AmountTWD','CardInst','CloseAmountTWD','CardRefundAmountTWD','CardRemainAmountTWD')
    AND jsonb_typeof(e.value)<>'string'))
 OR p_report->'AmountTWD' IS DISTINCT FROM to_jsonb(a.amount_minor/100)
 OR p_report->'CardInst' IS DISTINCT FROM '0'::jsonb
 OR p_report->>'MerTradeNo'<>a.merchant_trade_no OR a.currency<>'TWD' OR a.method_code<>'payuni_credit'
 OR p_report->>'PaymentType'<>'1' OR p_report->>'AuthType'<>'1' OR p_report->>'Status'<>'SUCCESS'
 OR p_report->>'TradeStatus' NOT IN ('0','1','2','3','4','8','9')
 OR p_report->>'DataSource' NOT IN ('A','B') OR p_report->>'CloseStatus' NOT IN ('','1','2','3','7','9')
 OR p_report->>'CardRefundType' NOT IN ('','2','3')
 OR p_report->>'CardRefundStatus' NOT IN ('','1','2','3','8')
 OR (p_report ? 'CardRefundDay' AND p_report->>'CardRefundDay'<>''
  AND p_report->>'CardRefundDay' !~
   '^[0-9]{4}-(0[1-9]|1[0-2])-([0-2][0-9]|3[01]) ([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$')
 OR (p_report->>'TradeNo'<>'' AND p_report->>'TradeNo' !~ '^[A-Za-z0-9_-]{1,64}$') THEN
  RAISE EXCEPTION 'payment report mismatch' USING ERRCODE='PT409'; END IF;
 IF p_report ? 'CardRefundDay' AND p_report->>'CardRefundDay'<>'' THEN
  BEGIN
   v_day:=make_timestamp(substr(p_report->>'CardRefundDay',1,4)::int,
    substr(p_report->>'CardRefundDay',6,2)::int,substr(p_report->>'CardRefundDay',9,2)::int,
    substr(p_report->>'CardRefundDay',12,2)::int,substr(p_report->>'CardRefundDay',15,2)::int,
    substr(p_report->>'CardRefundDay',18,2)::int);
   IF to_char(v_day,'YYYY-MM-DD HH24:MI:SS')<>p_report->>'CardRefundDay' THEN
    RAISE EXCEPTION 'payment report mismatch' USING ERRCODE='PT409'; END IF;
  EXCEPTION WHEN datetime_field_overflow THEN
   RAISE EXCEPTION 'payment report mismatch' USING ERRCODE='PT409';
  END;
 END IF;
 SELECT x.provider_reference,x.lease_until INTO v_ref,v_lease FROM integration.operations x WHERE x.id=p_id;
 IF v_ref<>'' AND v_ref<>p_report->>'TradeNo' THEN
  RAISE EXCEPTION 'payment reference changed' USING ERRCODE='PT409'; END IF;
 v_hash:=sha256(convert_to(p_report::text,'UTF8'));
 IF p_reconcile_job IS NULL OR p_reconcile_job<1 OR NOT EXISTS(
  SELECT 1 FROM river_payment.river_job j WHERE j.id=p_reconcile_job AND j.kind='payment_reconcile_v1'
   AND j.state='available' AND j.attempt=0
   AND j.args=jsonb_build_object('operation_id',p_id::text,'report_hash',encode(v_hash,'hex'),'version',1)) THEN
  RAISE EXCEPTION 'payment reconcile job missing' USING ERRCODE='PT409'; END IF;
 INSERT INTO payments.provider_observations(tenant_id,store_id,attempt_id,source,execution_profile,environment,
  first_generation,report,report_hash)
 VALUES(a.tenant_id,a.store_id,a.id,'QUERY',p_profile,a.environment,p_generation,p_report,v_hash)
 ON CONFLICT(tenant_id,store_id,attempt_id,report_hash) DO NOTHING;
 PERFORM integration.complete_operation(p_id,p_generation,p_token,'UNKNOWN','payment_report_observed',p_report->>'TradeNo');
 IF clock_timestamp()>=v_lease THEN
  RAISE EXCEPTION 'payment query lease conflict' USING ERRCODE='40001'; END IF;
END $$;

-- Rebind existing link/router OIDs. Their original owners and grants survive.
DROP TRIGGER payment_queue_route_v1 ON river.river_job;
DROP TRIGGER checkout_expiry_queue_route_v1 ON river.river_job;

CREATE OR REPLACE FUNCTION integration.payment_job_queue(p_job bigint)
RETURNS text LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT CASE a.execution_profile
  WHEN 'PROVIDER_MOCK' THEN 'payment_mock_v1'
  WHEN 'SANDBOX' THEN 'payment_sandbox_v1'
  WHEN 'LIVE' THEN 'payment_live_v1' END
 FROM river_payment.river_job j JOIN checkout.payment_attempts a
  ON a.id::text=j.args->>'operation_id'
 WHERE j.id=p_job AND (
  (j.kind='payment_query_v1' AND a.job_id=j.id
   AND j.args=jsonb_build_object('operation_id',a.id::text,'version',1))
  OR (j.kind='payment_reconcile_v1' AND EXISTS(
   SELECT 1 FROM payments.provider_observations o
   WHERE o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.attempt_id=a.id
    AND o.source='QUERY' AND o.execution_profile=a.execution_profile
    AND o.environment=a.environment
    AND j.args=jsonb_build_object('operation_id',a.id::text,
     'report_hash',encode(o.report_hash,'hex'),'version',1))))
$$;

CREATE OR REPLACE FUNCTION integration.route_payment_queue_v1()
RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE j river_payment.river_job%ROWTYPE; expected text;
BEGIN
 -- Deferred NEW is a snapshot. Inspect the final row under lock before changing
 -- only queue, and disallow a same-transaction kind/args rewrite.
 SELECT x.* INTO j FROM river_payment.river_job x WHERE x.id=NEW.id FOR UPDATE;
 IF NEW.kind IN ('payment_query_v1','payment_reconcile_v1')
  OR NEW.queue IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1')
  OR j.kind IN ('payment_query_v1','payment_reconcile_v1')
  OR j.queue IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1') THEN
  IF j.id IS NULL OR j.kind IS DISTINCT FROM NEW.kind OR j.args IS DISTINCT FROM NEW.args
   OR j.kind NOT IN ('payment_query_v1','payment_reconcile_v1') OR j.unique_key IS NOT NULL THEN
   RAISE EXCEPTION 'invalid payment queue job' USING ERRCODE='22023';
  END IF;
  expected:=integration.payment_job_queue(j.id);
  IF expected IS NULL OR j.queue NOT IN ('default',expected) THEN
   RAISE EXCEPTION 'payment queue linkage mismatch' USING ERRCODE='22023';
  END IF;
  IF j.queue<>expected THEN
   UPDATE river_payment.river_job SET queue=expected WHERE id=j.id;
  END IF;
 END IF;
 RETURN NULL;
END $$;

CREATE OR REPLACE FUNCTION checkout.expiry_job_linked(p_job bigint)
RETURNS boolean LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM river_expiry.river_job j JOIN checkout.orders o
  ON o.job_id=j.id
  WHERE j.id=p_job AND j.kind='checkout_expiry_v1'
   AND j.args=jsonb_build_object('order_id',o.id::text,'generation',1,'version',1)
   -- JSONB considers 1.0 equal to 1, but Go's integer decoder does not.
   AND j.args->>'generation'='1' AND j.args->>'version'='1')
$$;

CREATE OR REPLACE FUNCTION checkout.route_expiry_queue_v1()
RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE j river_expiry.river_job%ROWTYPE;
BEGIN
 -- Begin inserts the job before its order in the same transaction. Deferred NEW
 -- is a snapshot: re-read the final row and reject same-transaction rewrites.
 SELECT x.* INTO j FROM river_expiry.river_job x WHERE x.id=NEW.id FOR UPDATE;
 IF NEW.kind='checkout_expiry_v1' OR NEW.queue='checkout_expiry_v1'
  OR j.kind='checkout_expiry_v1' OR j.queue='checkout_expiry_v1' THEN
  IF j.id IS NULL OR j.kind IS DISTINCT FROM NEW.kind OR j.args IS DISTINCT FROM NEW.args
   OR j.kind<>'checkout_expiry_v1' OR j.unique_key IS NOT NULL
   OR j.queue NOT IN ('default','checkout_expiry_v1')
   OR NOT checkout.expiry_job_linked(j.id) THEN
   RAISE EXCEPTION 'invalid checkout expiry queue job' USING ERRCODE='22023';
  END IF;
  IF j.queue<>'checkout_expiry_v1' THEN
   UPDATE river_expiry.river_job SET queue='checkout_expiry_v1' WHERE id=j.id;
  END IF;
 END IF;
 RETURN NULL;
END $$;

CREATE FUNCTION integration.guard_payment_job_family() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.kind NOT IN ('payment_query_v1','payment_reconcile_v1')
  OR NEW.queue NOT IN ('default','payment_mock_v1','payment_sandbox_v1','payment_live_v1')
  OR NEW.unique_key IS NOT NULL OR NEW.args IS NULL OR jsonb_typeof(NEW.args)<>'object'
  OR NEW.args->>'operation_id' IS NULL
  OR NEW.args->>'operation_id' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR NEW.args->>'version' IS DISTINCT FROM '1'
  OR (NEW.kind='payment_query_v1' AND NEW.args IS DISTINCT FROM
   jsonb_build_object('operation_id',NEW.args->>'operation_id','version',1))
  OR (NEW.kind='payment_reconcile_v1' AND
   (NEW.args->>'report_hash' IS NULL OR NEW.args->>'report_hash' !~ '^[0-9a-f]{64}$'
    OR NEW.args IS DISTINCT FROM jsonb_build_object('operation_id',NEW.args->>'operation_id',
     'report_hash',NEW.args->>'report_hash','version',1))) THEN
  RAISE EXCEPTION 'invalid payment family job' USING ERRCODE='22023';
 END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.id IS DISTINCT FROM OLD.id OR NEW.kind IS DISTINCT FROM OLD.kind
   OR NEW.args IS DISTINCT FROM OLD.args OR NEW.unique_key IS DISTINCT FROM OLD.unique_key
   OR (NEW.queue IS DISTINCT FROM OLD.queue AND NOT
    (OLD.queue='default' AND NEW.queue IS NOT DISTINCT FROM integration.payment_job_queue(OLD.id))) THEN
   RAISE EXCEPTION 'immutable payment job identity' USING ERRCODE='22023';
  END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION integration.guard_payment_job_family() OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.guard_payment_job_family() FROM PUBLIC;

CREATE FUNCTION checkout.guard_expiry_job_family() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.kind<>'checkout_expiry_v1' OR NEW.queue NOT IN ('default','checkout_expiry_v1')
  OR NEW.unique_key IS NOT NULL OR NEW.args IS NULL OR jsonb_typeof(NEW.args)<>'object'
  OR NEW.args->>'order_id' IS NULL
  OR NEW.args->>'order_id' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR NEW.args IS DISTINCT FROM jsonb_build_object('order_id',NEW.args->>'order_id','generation',1,'version',1)
  OR NEW.args->>'generation' IS DISTINCT FROM '1' OR NEW.args->>'version' IS DISTINCT FROM '1' THEN
  RAISE EXCEPTION 'invalid expiry family job' USING ERRCODE='22023';
 END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.id IS DISTINCT FROM OLD.id OR NEW.kind IS DISTINCT FROM OLD.kind
   OR NEW.args IS DISTINCT FROM OLD.args OR NEW.unique_key IS DISTINCT FROM OLD.unique_key
   OR (NEW.queue IS DISTINCT FROM OLD.queue AND NOT
    (OLD.queue='default' AND NEW.queue='checkout_expiry_v1' AND checkout.expiry_job_linked(OLD.id))) THEN
   RAISE EXCEPTION 'immutable expiry job identity' USING ERRCODE='22023';
  END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION checkout.guard_expiry_job_family() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.guard_expiry_job_family() FROM PUBLIC;

CREATE TRIGGER payment_job_family BEFORE INSERT OR UPDATE ON river_payment.river_job
 FOR EACH ROW EXECUTE FUNCTION integration.guard_payment_job_family();
CREATE CONSTRAINT TRIGGER payment_queue_route_v1 AFTER INSERT ON river_payment.river_job
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION integration.route_payment_queue_v1();
CREATE TRIGGER expiry_job_family BEFORE INSERT OR UPDATE ON river_expiry.river_job
 FOR EACH ROW EXECUTE FUNCTION checkout.guard_expiry_job_family();
CREATE CONSTRAINT TRIGGER checkout_expiry_queue_route_v1 AFTER INSERT ON river_expiry.river_job
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION checkout.route_expiry_queue_v1();

DELETE FROM river.river_job WHERE kind IN ('payment_query_v1','payment_reconcile_v1','checkout_expiry_v1');
DELETE FROM river.river_queue WHERE name IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1','checkout_expiry_v1');
-- Keep the independent Meta exclusion trigger and old external/default lane.
CREATE FUNCTION integration.reject_legacy_family_job() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.kind IN ('payment_query_v1','payment_reconcile_v1','checkout_expiry_v1')
  OR NEW.queue IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1','checkout_expiry_v1')
  OR (TG_OP='UPDATE' AND (OLD.kind IN ('payment_query_v1','payment_reconcile_v1','checkout_expiry_v1')
   OR OLD.queue IN ('payment_mock_v1','payment_sandbox_v1','payment_live_v1','checkout_expiry_v1'))) THEN
  RAISE EXCEPTION 'legacy family lane disabled' USING ERRCODE='22023';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION integration.reject_legacy_family_job() OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.reject_legacy_family_job() FROM PUBLIC;
CREATE TRIGGER legacy_family_exclusion BEFORE INSERT OR UPDATE ON river.river_job
 FOR EACH ROW EXECUTE FUNCTION integration.reject_legacy_family_job();

GRANT USAGE ON SCHEMA river_payment,river_expiry TO commerce_checkout_runtime,commerce_checkout_writer;
GRANT USAGE ON SCHEMA river_payment TO commerce_integration_writer;
GRANT SELECT,INSERT,UPDATE(kind) ON river_payment.river_job,river_expiry.river_job TO commerce_checkout_runtime;
GRANT USAGE ON SEQUENCE river_payment.river_job_id_seq,river_expiry.river_job_id_seq TO commerce_checkout_runtime;
GRANT SELECT ON river_payment.river_job,river_expiry.river_job TO commerce_checkout_writer;
GRANT UPDATE(queue) ON river_expiry.river_job TO commerce_checkout_writer;
GRANT SELECT,UPDATE(queue) ON river_payment.river_job TO commerce_integration_writer;
REVOKE UPDATE(queue) ON river.river_job FROM commerce_integration_writer;
-- Checkout old-table privileges and native lifecycle grants are also enforced
-- by migrate.go on every Apply, atomically with this post transaction.

CREATE OR REPLACE FUNCTION integration.payment_queue_ready()
RETURNS boolean LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE guards_ready boolean;
BEGIN
 SELECT count(*)=3 INTO guards_ready FROM (VALUES
  ('river_payment.river_job','payment_job_family','integration.guard_payment_job_family()',23,false,'commerce_integration_writer'),
  ('river_payment.river_job','payment_queue_route_v1','integration.route_payment_queue_v1()',5,true,'commerce_integration_writer'),
  ('river.river_job','legacy_family_exclusion','integration.reject_legacy_family_job()',23,false,'commerce_integration_writer')
 ) AS expected(relation_name,trigger_name,function_name,trigger_type,deferred,owner_name)
 -- Catalog resolution does not require old-schema USAGE from checkout's
 -- definer, whose obsolete old-table privileges have been revoked.
 JOIN pg_catalog.pg_trigger t ON t.tgname=expected.trigger_name
  AND t.tgfoid=to_regprocedure(expected.function_name)
 JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid
 JOIN pg_catalog.pg_roles r ON r.oid=p.proowner
 JOIN pg_catalog.pg_class c ON c.oid=t.tgrelid
 JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname||'.'||c.relname=expected.relation_name
  AND t.tgtype=expected.trigger_type AND t.tgenabled IN ('O','A') AND NOT t.tgisinternal
  AND t.tgdeferrable=expected.deferred AND t.tginitdeferred=expected.deferred
  AND t.tgqual IS NULL AND t.tgnargs=0 AND t.tgargs='\x'::bytea AND t.tgattr=''::int2vector
  AND p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog']::text[]
  AND r.rolname=expected.owner_name AND NOT r.rolcanlogin AND NOT r.rolsuper
  AND NOT r.rolbypassrls AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication
  AND p.proowner<>c.relowner AND p.proowner<>n.nspowner
  AND p.proowner<>(SELECT datdba FROM pg_catalog.pg_database WHERE datname=current_database());
 IF NOT guards_ready THEN RETURN false; END IF;
 -- Preserved terminal default-queue rows are valid history; they are never
 -- consumed. Any foreign family/queue, or active linkage drift, fails closed.
 RETURN NOT EXISTS(SELECT 1 FROM river_payment.river_job j WHERE j.kind NOT IN ('payment_query_v1','payment_reconcile_v1')
   OR j.queue NOT IN ('default','payment_mock_v1','payment_sandbox_v1','payment_live_v1')
   OR (j.state NOT IN ('completed','cancelled','discarded') AND
    (j.unique_key IS NOT NULL OR j.args->>'version' IS DISTINCT FROM '1'
     OR j.queue IS DISTINCT FROM integration.payment_job_queue(j.id))));
END $$;

CREATE OR REPLACE FUNCTION checkout.expiry_queue_ready()
RETURNS boolean LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE guards_ready boolean;
BEGIN
 SELECT count(*)=3 INTO guards_ready FROM (VALUES
  ('river_expiry.river_job','expiry_job_family','checkout.guard_expiry_job_family()',23,false,'commerce_checkout_writer'),
  ('river_expiry.river_job','checkout_expiry_queue_route_v1','checkout.route_expiry_queue_v1()',5,true,'commerce_checkout_writer'),
  ('river.river_job','legacy_family_exclusion','integration.reject_legacy_family_job()',23,false,'commerce_integration_writer')
 ) AS expected(relation_name,trigger_name,function_name,trigger_type,deferred,owner_name)
 -- Catalog resolution does not require old-schema USAGE from checkout's
 -- definer, whose obsolete old-table privileges have been revoked.
 JOIN pg_catalog.pg_trigger t ON t.tgname=expected.trigger_name
  AND t.tgfoid=to_regprocedure(expected.function_name)
 JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid
 JOIN pg_catalog.pg_roles r ON r.oid=p.proowner
 JOIN pg_catalog.pg_class c ON c.oid=t.tgrelid
 JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname||'.'||c.relname=expected.relation_name
  AND t.tgtype=expected.trigger_type AND t.tgenabled IN ('O','A') AND NOT t.tgisinternal
  AND t.tgdeferrable=expected.deferred AND t.tginitdeferred=expected.deferred
  AND t.tgqual IS NULL AND t.tgnargs=0 AND t.tgargs='\x'::bytea AND t.tgattr=''::int2vector
  AND p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog']::text[]
  AND r.rolname=expected.owner_name AND NOT r.rolcanlogin AND NOT r.rolsuper
  AND NOT r.rolbypassrls AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication
  AND p.proowner<>c.relowner AND p.proowner<>n.nspowner
  AND p.proowner<>(SELECT datdba FROM pg_catalog.pg_database WHERE datname=current_database());
 IF NOT guards_ready THEN RETURN false; END IF;
 -- Preserved terminal default-queue rows are valid history; they are never
 -- consumed. Any foreign family/queue, or active linkage drift, fails closed.
 RETURN NOT EXISTS(SELECT 1 FROM river_expiry.river_job j WHERE j.kind<>'checkout_expiry_v1' OR j.queue NOT IN ('default','checkout_expiry_v1')
   OR (j.state NOT IN ('completed','cancelled','discarded') AND
    (j.unique_key IS NOT NULL OR j.queue<>'checkout_expiry_v1'
     OR NOT checkout.expiry_job_linked(j.id))));
END $$;
