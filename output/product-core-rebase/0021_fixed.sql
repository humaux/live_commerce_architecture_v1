-- Post-River 0021 checkout.begin_hold, same 10-argument signature as post_river/0020 (home-cod), plus the A6 untracked-SKU
-- path (docs/delivery/units/product-editor.md section f ruling 1; contracts/buyer-checkout-v1.md amendment). Applied AFTER
-- post_river/0020 (which owns the cash_on_delivery branch) and AFTER the numbered 0109 (which adds catalog.skus.inventory_tracked
-- and max_per_order), so this CREATE OR REPLACE is the final body; owner, EXECUTE grant and the "exactly one begin_hold with
-- 10 arguments" invariant stay.
--
-- Deltas from 0020 (everything else is that body verbatim, cash_on_delivery included):
--  (1) the plan count bound is 0..800, not 1..800: an order whose lines are all untracked sends an empty plan (Go locks
--      and deducts nothing), and an empty plan is legitimate.
--  (2) the plan-to-quote conservation demand is tracked SKUs only (JOIN catalog.skus WHERE inventory_tracked): an untracked
--      SKU is charged in the quote but absent from the plan, so it must not trip the FULL JOIN mismatch.
--  (3) a new PT422 max_per_order_exceeded guard: an untracked SKU's line quantity must be <= its max_per_order (the SKU has
--      no balance row, so the stock lock cannot bound it).
-- Non-goals unchanged: no price computation, no payment attempt, tracked SKUs keep the no-oversell stock lock, and the
-- home-cod rules of 0020 (cash_on_delivery mode, cap/surcharge/expected-surcharge, recipient gate, carrier snapshot,
-- AWAITING_COLLECTION state, collected_at plumbing downstream) all survive this replace.
-- Caller: internal/checkout Service.Begin only (commerce_checkout_runtime login).

CREATE OR REPLACE FUNCTION checkout.begin_hold(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,
 p_order uuid,p_snapshot jsonb,p_lines jsonb,p_job_id bigint,p_payment_environment text,p_payment_mode text) RETURNS jsonb
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
 v_profile record; v_profiled boolean:=false; v_settings record; v_subtotal bigint; v_total bigint; v_ns text[];
 v_dir boolean:=false; v_api boolean:=false; v_pap boolean:=false; v_sub text;
 v_bt boolean:=false; v_bank record; v_hold interval:=interval '15 minutes';
 v_cod boolean:=false; v_cod_settings record; v_cod_surcharge bigint; v_expected_surcharge bigint; v_cod_carrier text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
    OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
    OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_job_id IS NULL OR p_job_id<1
    OR p_payment_environment IS NULL OR p_payment_environment NOT IN ('SANDBOX','LIVE','')
    OR p_payment_mode IS NULL OR p_payment_mode NOT IN ('card','pay_at_pickup','bank_transfer','cash_on_delivery')
    OR p_snapshot IS NULL OR jsonb_typeof(p_snapshot)<>'object' OR octet_length(p_snapshot::text)>1048576
    OR p_lines IS NULL OR jsonb_typeof(p_lines)<>'array' OR octet_length(p_lines::text)>1048576
    OR jsonb_typeof(p_snapshot->'quote')<>'object'
    OR jsonb_typeof(p_snapshot->'destination')<>'object'
    OR jsonb_typeof(p_snapshot->'service')<>'object'
    OR jsonb_typeof(p_snapshot->'allocation')<>'object'
    OR (SELECT count(*) FROM jsonb_object_keys(p_snapshot)) NOT BETWEEN 4 AND 5
    OR ((SELECT count(*) FROM jsonb_object_keys(p_snapshot))=5 AND NOT p_snapshot ? 'expected_cod_surcharge_minor') THEN
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
  AND o.commercial_state IN ('DRAFT','AWAITING_PAYMENT','AWAITING_TRANSFER','CONFIRMED','AWAITING_COLLECTION')) THEN
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
     OR v_source.verification_kind NOT IN ('MANUAL_ATTESTED','PROVIDER_DIRECTORY_VERIFIED','BUYER_ENTERED')
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
 IF NOT FOUND OR NOT v_service.enabled OR NOT v_service.visible OR v_service.mode NOT IN ('MANUAL','API')
    OR v_service.delivery_kind<>v_dest.kind OR v_service.currency<>v_quote.currency
    OR v_service.policy_version<>v_quote.policy_version
    OR p_snapshot->'service'->>'code' IS DISTINCT FROM v_service.code
    OR p_snapshot->'service'->>'market_id' IS DISTINCT FROM v_service.market_id::text
    OR p_snapshot->'service'->>'country' IS DISTINCT FROM v_service.country THEN
  RAISE EXCEPTION 'service changed' USING ERRCODE='PT409'; END IF;
 -- The store's single enabled profile (lock order: pickup head -> destination head -> service head -> profile FOR SHARE -> allocation).
 SELECT pr.connection_id,pr.mode,pr.ok_verified,pr.hilife_verified,pr.qualified_credential_version,a.environment,
  a.credential_version,a.binding_id INTO v_profile
  FROM integration.ecpay_logistics_profiles pr
  JOIN integration.merchant_accounts a ON a.tenant_id=pr.tenant_id AND a.store_id=pr.store_id AND a.id=pr.connection_id
  WHERE pr.tenant_id=v_tenant AND pr.store_id=p_store AND pr.enabled FOR SHARE OF pr;
 v_profiled:=FOUND AND v_profile.qualified_credential_version IS NOT DISTINCT FROM v_profile.credential_version;
 v_api:=v_service.mode='API';
 IF v_dest.pickup_id IS NOT NULL THEN v_dir:=v_source.verification_kind='PROVIDER_DIRECTORY_VERIFIED'; END IF;
 -- (b) API service: enabled+visible (above) and bound to the store's enabled qualified profile for this destination kind.
 IF v_api AND (NOT v_profiled OR v_service.binding_id IS DISTINCT FROM v_profile.binding_id
    OR (v_dest.kind='cvs_okmart' AND NOT v_profile.ok_verified) OR (v_dest.kind='cvs_hilife' AND NOT v_profile.hilife_verified)
    OR v_dest.kind NOT IN ('cvs_711','cvs_familymart','cvs_hilife','cvs_okmart')) THEN
  RAISE EXCEPTION 'service changed' USING ERRCODE='PT409'; END IF;
 -- (a)/§16.1: a buyer-entered store is never verified: MANUAL service only, only while the store has no qualified profile.
 IF v_dest.pickup_id IS NOT NULL THEN
  IF v_source.verification_kind='BUYER_ENTERED' THEN
   IF v_service.mode<>'MANUAL' OR v_profiled THEN RAISE EXCEPTION 'cvs_source_mismatch' USING ERRCODE='PT422'; END IF;
   IF v_source.namespace<>'buyer.'||v_owner::text THEN RAISE EXCEPTION 'pickup source changed' USING ERRCODE='PT409'; END IF;
  END IF;
 END IF;
 IF v_api AND NOT v_dir THEN RAISE EXCEPTION 'cvs_source_mismatch' USING ERRCODE='PT422'; END IF;
 -- (c) provider-verified source or API service: ECPay limits, all money in TWD minor units (0061 convention, x100; I05: the
 -- server quote, never the client), recipient rule F5, and the ECPay environment equals the deployment payment environment.
 IF v_api OR v_dir THEN
  v_subtotal:=(v_quote.snapshot->'amount'->>'subtotal_minor')::bigint;
  IF v_quote.currency<>'TWD' OR v_subtotal IS NULL OR v_subtotal%100<>0 OR v_subtotal NOT BETWEEN 100 AND 2000000 THEN
   RAISE EXCEPTION 'cvs_amount_exceeds' USING ERRCODE='PT422'; END IF;
  IF NOT fulfillment.ecpay_recipient_ok(v_dest.recipient_name,v_dest.phone) THEN
   RAISE EXCEPTION 'cvs_recipient_rejected' USING ERRCODE='PT422'; END IF;
  IF NOT v_profiled OR p_payment_environment='' OR v_profile.environment<>p_payment_environment THEN
   RAISE EXCEPTION 'cvs_environment_mismatch' USING ERRCODE='PT422'; END IF;
  IF v_dir THEN
   v_ns:=regexp_match(v_source.namespace,'^ecpay\.(sandbox|live)\.([a-z0-9]+)$');
   v_sub:=upper(v_ns[2]);
   IF v_ns IS NULL OR upper(v_ns[1])<>v_profile.environment
    OR v_sub NOT IN ('UNIMARTC2C','FAMIC2C','HILIFEC2C','OKMARTC2C','UNIMART','FAMI','HILIFE')
    OR (v_profile.mode='C2C' AND v_sub NOT LIKE '%C2C') OR (v_profile.mode='B2C' AND v_sub LIKE '%C2C') THEN
    RAISE EXCEPTION 'cvs_environment_mismatch' USING ERRCODE='PT422'; END IF;
  END IF;
 END IF;
 -- §16.2/§16.5 pay_at_pickup: every guard fails with zero holds. Settings are locked FOR UPDATE right after the service head
 -- and profile, which serialises pay-at-pickup placements per store (a missing row means pay-at-pickup is off).
 v_pap:=p_payment_mode='pay_at_pickup';
 IF v_pap THEN
  IF v_dest.kind NOT IN ('cvs_711','cvs_familymart','cvs_hilife','cvs_okmart') THEN
   RAISE EXCEPTION 'pay_at_pickup_unavailable' USING ERRCODE='PT422'; END IF;
  SELECT c.pay_at_pickup_enabled,c.pay_at_pickup_max_twd,c.pay_at_pickup_max_open,c.enabled_chains INTO v_settings
   FROM fulfillment.cvs_store_settings c WHERE c.tenant_id=v_tenant AND c.store_id=p_store FOR UPDATE;
  IF NOT FOUND OR NOT v_settings.pay_at_pickup_enabled OR NOT v_dest.kind=ANY(v_settings.enabled_chains)
   OR v_quote.currency<>'TWD' THEN RAISE EXCEPTION 'pay_at_pickup_unavailable' USING ERRCODE='PT422'; END IF;
  v_total:=(v_quote.snapshot->'amount'->>'total_minor')::bigint;
  IF v_total IS NULL OR v_total%100<>0 OR v_total NOT BETWEEN 100 AND least(v_settings.pay_at_pickup_max_twd,20000)::bigint*100 THEN
   RAISE EXCEPTION 'pay_at_pickup_amount_exceeds' USING ERRCODE='PT422'; END IF;
  -- C3: the recipient must be a real name and a mobile for every pay_at_pickup order, whichever store source.
  IF NOT fulfillment.ecpay_recipient_ok(v_dest.recipient_name,v_dest.phone) THEN
   RAISE EXCEPTION 'cvs_recipient_rejected' USING ERRCODE='PT422'; END IF;
  -- R4-3: open (unshipped, uncollected) pay-at-pickup orders bound what anonymous buyers can lock (partial index orders_pay_at_pickup_open).
  IF (SELECT count(*) FROM checkout.orders x WHERE x.tenant_id=v_tenant AND x.store_id=p_store AND x.payment_mode='pay_at_pickup'
     AND x.collection_state='PENDING' AND x.fulfillment_state='MANUAL_UNASSIGNED')>=v_settings.pay_at_pickup_max_open
   OR EXISTS(SELECT 1 FROM checkout.orders x WHERE x.tenant_id=v_tenant AND x.store_id=p_store AND x.owner_id=v_owner
     AND x.payment_mode='pay_at_pickup' AND x.collection_state='PENDING' AND x.fulfillment_state='MANUAL_UNASSIGNED') THEN
   RAISE EXCEPTION 'pay_at_pickup_limit' USING ERRCODE='PT429'; END IF;
 END IF;
 -- storefront-v2 §C bank_transfer: the store's settings row (locked FOR SHARE so a concurrent settings change serialises with this placement)
 -- must be enabled, and a CVS destination needs allow_cvs. The window (6..168 h) is how long the stock stays reserved.
 v_bt:=p_payment_mode='bank_transfer';
 IF v_bt THEN
  SELECT c.enabled,c.allow_cvs,c.window_hours,c.bank_name,c.branch,c.account_name,c.account_number INTO v_bank
   FROM checkout.bank_transfer_settings c WHERE c.tenant_id=v_tenant AND c.store_id=p_store FOR SHARE;
  IF NOT FOUND OR NOT v_bank.enabled OR (v_dest.kind<>'home' AND NOT v_bank.allow_cvs) THEN
   RAISE EXCEPTION 'bank_transfer_unavailable' USING ERRCODE='PT422'; END IF;
  v_hold:=make_interval(hours=>v_bank.window_hours);
 END IF;
 -- home-cod (0107): home delivery only; the store's cash_on_delivery_settings row (locked FOR UPDATE so a concurrent settings change
 -- serialises with this placement) must be enabled; the whole-TWD total plus the whole-TWD surcharge must not exceed the per-order cap;
 -- the pay_at_pickup open-orders limit (default 20) also bounds unshipped COD orders. The hold stays 15 minutes because the order is
 -- COMMITTED at placement (like pay_at_pickup), so expiry is moot.
 v_cod:=p_payment_mode='cash_on_delivery';
 IF v_cod THEN
  IF v_dest.kind<>'home' OR v_quote.country<>'TW' THEN
   RAISE EXCEPTION 'cash_on_delivery_unavailable' USING ERRCODE='PT422'; END IF;
  SELECT c.enabled,c.max_twd,c.surcharge_twd,c.carrier INTO v_cod_settings
   FROM checkout.cash_on_delivery_settings c WHERE c.tenant_id=v_tenant AND c.store_id=p_store FOR UPDATE;
  IF NOT FOUND OR NOT v_cod_settings.enabled OR v_quote.currency<>'TWD' THEN
   RAISE EXCEPTION 'cash_on_delivery_unavailable' USING ERRCODE='PT422'; END IF;
  v_total:=(v_quote.snapshot->'amount'->>'total_minor')::bigint;
  v_cod_surcharge:=v_cod_settings.surcharge_twd::bigint*100;
  v_cod_carrier:=v_cod_settings.carrier;
  -- P2-1: the buyer re-quotes when the surcharge they were shown changed (the snapshot carries the shown value only when the
  -- storefront sent it). Checked before the amount test so a settings change surfaces as a re-quote, never as a cap refusal.
  v_expected_surcharge:=(p_snapshot->>'expected_cod_surcharge_minor')::bigint;
  IF v_expected_surcharge IS NOT NULL AND v_expected_surcharge<>v_cod_surcharge THEN
   RAISE EXCEPTION 'cod_surcharge_changed' USING ERRCODE='PT409'; END IF;
  IF v_total IS NULL OR v_total%100<>0 OR v_total NOT BETWEEN 100 AND 2000000
   OR v_total+v_cod_surcharge>v_cod_settings.max_twd::bigint*100 THEN
   RAISE EXCEPTION 'cash_on_delivery_amount_exceeds' USING ERRCODE='PT422'; END IF;
  -- P2-6: a reachable recipient before committing stock — a real name (letters/CJK/full-width, no digits/symbols/emoji,
  -- spaces only between words) and a Taiwan mobile. This is fulfillment.ecpay_recipient_ok without ECPay's 4..10-wide
  -- ReceiverName label-field rule (home delivery has no such field), so a Latin name like the harness "Synthetic Buyer"
  -- stays valid while a junk name/phone still refuses before stock is committed.
  IF v_dest.recipient_name IS NULL OR v_dest.phone IS NULL
   OR v_dest.recipient_name !~ '^[A-Za-z㐀-䶿一-鿿豈-﫿Ａ-Ｚａ-ｚ]+( [A-Za-z㐀-䶿一-鿿豈-﫿Ａ-Ｚａ-ｚ]+)*$'
   OR (CASE WHEN regexp_replace(v_dest.phone,'[ ()-]','','g') ~ '^\+8869[0-9]{8}$'
       THEN '0'||substr(regexp_replace(v_dest.phone,'[ ()-]','','g'),5)
       ELSE regexp_replace(v_dest.phone,'[ ()-]','','g') END) !~ '^09[0-9]{8}$' THEN
   RAISE EXCEPTION 'cvs_recipient_rejected' USING ERRCODE='PT422'; END IF;
  -- The pay_at_pickup open-orders cap (cvs_store_settings.pay_at_pickup_max_open, default 20) is the shared cap on anonymous
  -- unshipped offline orders; COD adds its own store+owner predicate on the orders_cod_open partial index.
  IF (SELECT count(*) FROM checkout.orders x WHERE x.tenant_id=v_tenant AND x.store_id=p_store
     AND x.payment_mode='cash_on_delivery' AND x.collection_state='PENDING' AND x.fulfillment_state='MANUAL_UNASSIGNED')
     >=coalesce((SELECT c.pay_at_pickup_max_open FROM fulfillment.cvs_store_settings c WHERE c.tenant_id=v_tenant AND c.store_id=p_store),20)
   OR EXISTS(SELECT 1 FROM checkout.orders x WHERE x.tenant_id=v_tenant AND x.store_id=p_store AND x.owner_id=v_owner
     AND x.payment_mode='cash_on_delivery' AND x.collection_state='PENDING' AND x.fulfillment_state='MANUAL_UNASSIGNED') THEN
   RAISE EXCEPTION 'cash_on_delivery_limit' USING ERRCODE='PT429'; END IF;
 END IF;
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
 -- price calculation remains the one monetary authority. A6: the plan holds
 -- tracked SKUs only (Go never locks an untracked SKU), so an all-untracked
 -- order is a legitimate empty plan (bound 0..800).
 SELECT count(*),count(DISTINCT (l.warehouse_id,l.sku_id)) INTO v_count,v_distinct
  FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint);
 IF v_count NOT BETWEEN 0 AND 800 OR v_count<>v_distinct THEN
  RAISE EXCEPTION 'invalid stock plan' USING ERRCODE='PT400'; END IF;
 SELECT count(*),count(DISTINCT q.sku_id) INTO v_count,v_distinct
  FROM jsonb_to_recordset(v_quote.snapshot->'lines') AS q(sku_id uuid,quantity bigint);
 IF v_count NOT BETWEEN 1 AND 50 OR v_count<>v_distinct THEN
  RAISE EXCEPTION 'invalid quote lines' USING ERRCODE='PT400'; END IF;
 SELECT count(*) INTO v_mismatch FROM (
  WITH demand AS (SELECT q.sku_id,sum(q.quantity) AS quantity
    FROM jsonb_to_recordset(v_quote.snapshot->'lines') AS q(sku_id uuid,quantity bigint)
    JOIN catalog.skus s ON s.tenant_id=v_tenant AND s.store_id=p_store AND s.id=q.sku_id AND s.inventory_tracked
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
 -- A6: an untracked SKU is absent from the plan, so its per-order cap is its only quantity bound; enforce it here
 -- against the live SKU row (a tracked SKU is bounded by the stock lock above and has no max_per_order).
 IF EXISTS(SELECT 1 FROM jsonb_to_recordset(v_quote.snapshot->'lines') AS q(sku_id uuid,quantity bigint)
   JOIN catalog.skus s ON s.tenant_id=v_tenant AND s.store_id=p_store AND s.id=q.sku_id
   WHERE NOT s.inventory_tracked AND q.quantity>s.max_per_order) THEN
  RAISE EXCEPTION 'max_per_order_exceeded' USING ERRCODE='PT422'; END IF;

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
 v_expires:=v_now+v_hold;
 v_result:=jsonb_build_object('order_id',p_order::text,'reservation_id',p_order::text,
  'generation',1,'expires_at',v_expires,'job_id',p_job_id,'payment_mode',p_payment_mode,
  'commercial_state',CASE WHEN v_pap THEN 'CONFIRMED' WHEN v_bt THEN 'AWAITING_TRANSFER' WHEN v_cod THEN 'AWAITING_COLLECTION' ELSE 'DRAFT' END);
 INSERT INTO checkout.orders(tenant_id,store_id,owner_id,id,creator_session_id,cart_id,cart_version,
  quote_id,destination_id,market_id,country,service_code,service_version,allocation_version,
  currency,total_minor,cod_surcharge_minor,cod_carrier,commercial_state,fulfillment_state,generation,expires_at,job_id,snapshot,created_at,updated_at,
  payment_mode,collection_state)
 VALUES(v_tenant,p_store,v_owner,p_order,v_session,v_cart.id,v_cart.version,v_quote.id,v_dest.id,
  v_quote.market_id,v_quote.country,v_service.code,v_service.version,v_allocation.version,
  v_quote.currency,(v_quote.snapshot->'amount'->>'total_minor')::bigint,
  CASE WHEN v_cod THEN v_cod_surcharge END,
  v_cod_carrier,
  CASE WHEN v_pap THEN 'CONFIRMED' WHEN v_bt THEN 'AWAITING_TRANSFER' WHEN v_cod THEN 'AWAITING_COLLECTION' ELSE 'DRAFT' END,
  'MANUAL_UNASSIGNED',1,v_expires,p_job_id,p_snapshot,v_now,v_now,
  p_payment_mode,CASE WHEN v_pap OR v_cod THEN 'PENDING' END);
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
 IF v_pap OR v_cod THEN
  -- §11.5 HELD -> COMMITTED at placement (stock follows the normal path, no Stripe session, no payment attempt): one ALLOCATE
  -- row per line (BUYER, checkout.pay_at_pickup.commit reused for cash_on_delivery, command_key = order id), after inventory.lock_balance.
  -- Evidence: order id + collection_state PENDING (guard inventory.guard_pay_at_pickup_ledger admits pay_at_pickup CONFIRMED and
  -- cash_on_delivery AWAITING_COLLECTION).
  FOR v_line IN SELECT l.warehouse_id,l.sku_id,l.quantity
   FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint)
   ORDER BY l.warehouse_id,l.sku_id LOOP
   PERFORM 1 FROM inventory.lock_balance(v_line.warehouse_id,v_line.sku_id);
   INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,
    operation,command_key,reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
    VALUES(v_tenant,p_store,v_line.warehouse_id,v_line.sku_id,'ALLOCATE',-v_line.quantity,v_line.quantity,
     'checkout.pay_at_pickup.commit',p_order::text,p_order,NULL,p_order,v_owner,v_session,'BUYER');
  END LOOP;
  UPDATE inventory.reservations SET state='COMMITTED' WHERE tenant_id=v_tenant AND store_id=p_store AND id=p_order AND state='HELD';
 END IF;
 INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
 VALUES(v_tenant,p_store,v_owner,p_order,v_session,1,'checkout.held','BUYER');
 IF v_pap THEN
  INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
  VALUES(v_tenant,p_store,v_owner,p_order,v_session,1,'checkout.pay_at_pickup_placed','BUYER');
 END IF;
 IF v_bt THEN
  INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
  VALUES(v_tenant,p_store,v_owner,p_order,v_session,1,'checkout.bank_transfer_placed','BUYER');
  INSERT INTO checkout.bank_transfers(tenant_id,store_id,owner_id,order_id,state,bank_name,branch,account_name,account_number,window_hours,
   currency,created_at,updated_at)
  VALUES(v_tenant,p_store,v_owner,p_order,'AWAITING',v_bank.bank_name,v_bank.branch,v_bank.account_name,v_bank.account_number,v_bank.window_hours,
   v_quote.currency,v_now,v_now);
 END IF;
 IF v_cod THEN
  INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
  VALUES(v_tenant,p_store,v_owner,p_order,v_session,1,'checkout.cash_on_delivery_placed','BUYER');
 END IF;
 INSERT INTO checkout.command_results(tenant_id,store_id,owner_id,operation,idempotency_key,
  creator_session_id,request_hash,order_id,response)
 VALUES(v_tenant,p_store,v_owner,'checkout.begin',p_key,v_session,p_request_hash,p_order,v_result);
 RETURN v_result;
EXCEPTION WHEN invalid_text_representation OR numeric_value_out_of_range
 OR invalid_datetime_format THEN
 RAISE EXCEPTION 'invalid checkout input' USING ERRCODE='PT400';
END $$;
-- Owner and EXECUTE grant are unchanged by CREATE OR REPLACE (set by post_river/0017); re-stated so a fresh reader sees the final ACL here.
ALTER FUNCTION checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint,text,text) TO commerce_checkout_runtime;
COMMENT ON FUNCTION checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint,text,text) IS
 'internal/checkout Service.Begin only; EXECUTE commerce_checkout_runtime. Order + stock hold from a priced snapshot; CVS deltas of taiwan-cvs-logistics-v1 §4.3/§16.2 (pickup kinds, API services, ECPay guards, pay_at_pickup: HELD -> COMMITTED with BUYER ALLOCATE rows), storefront-v2 §C bank_transfer (PT422 bank_transfer_unavailable; order AWAITING_TRANSFER, reservation stays HELD for the merchant window, checkout.bank_transfers snapshot row, no payment attempt), home-cod 0107 (PT422 cash_on_delivery_unavailable/amount_exceeds, PT429 cash_on_delivery_limit; home only, order AWAITING_COLLECTION through collection, reservation HELD -> COMMITTED with the same BUYER ALLOCATE rows, cod_surcharge_minor snapshotted, no payment attempt) and A6 (0109): the plan holds tracked SKUs only (0..800; an all-untracked order is an empty plan), conservation ignores untracked SKUs, and an untracked line over its max_per_order raises PT422 max_per_order_exceeded. Exactly one checkout.begin_hold exists after a full Apply.';
