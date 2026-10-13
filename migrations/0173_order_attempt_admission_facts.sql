-- Purpose: C1 scoped, locked catalog/pricing/delivery/payment facts for order-attempt-v1.
-- Depends on: ops.order_attempt_admission_context, catalog/pricing/fulfillment tables,
-- checkout offline payment settings; pure Go pricing.CalculateWith computes the amounts.
-- Used by: commerce_order_attempt_owner admission coordinator in its existing pgx Tx.
-- Invariants: I01/I02/I08, order-attempt-v1 §§1–2; no browser identity, no caller prices,
-- no eligibility guesses, no stock writes; every explicit origin survives catalog lookup.

-- FOR SHARE needs an UPDATE privilege. It is confined to the existing NOLOGIN
-- domain owner; WITH CHECK(false) prevents using this new privilege for row writes.
GRANT UPDATE(id) ON catalog.products,catalog.skus TO commerce_checkout_writer;
CREATE POLICY order_attempt_catalog_lock ON catalog.products FOR UPDATE TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid) WITH CHECK(false);
CREATE POLICY order_attempt_catalog_lock ON catalog.skus FOR UPDATE TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid) WITH CHECK(false);

CREATE FUNCTION checkout.order_attempt_admission_facts(p_attempt uuid,p_selection jsonb) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 v_context jsonb; v_tenant uuid; v_store uuid; v_owner uuid;
 v_old_tenant text:=coalesce(current_setting('app.tenant_id',true),'');
 v_old_store text:=coalesce(current_setting('app.store_id',true),'');
 v_old_principal text:=coalesce(current_setting('app.principal_id',true),'');
 v_old_buyer text:=coalesce(current_setting('app.buyer_id',true),'');
 v_old_session text:=coalesce(current_setting('app.buyer_session_id',true),'');
 v_market_id uuid; v_country text; v_code text; v_mode text;
 v_items jsonb; v_item jsonb; v_delivery jsonb; v_customer jsonb; v_for jsonb;
 v_uuid_pattern constant text:='^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
 v_count integer; v_version bigint; v_parents jsonb; v_ordinals integer[]:='{}';
 v_keys text[]:='{}'; v_key text; v_field text; v_value text;
 v_market record; v_policy record; v_product record; v_sku record;
 v_service record; v_allocation record; v_profile record; v_payment record;
 v_market_json jsonb; v_policy_json jsonb; v_service_json jsonb;
 v_allocation_json jsonb; v_payment_json jsonb; v_lines jsonb; v_warehouses jsonb;
 v_result jsonb; v_home boolean; v_profiled boolean:=false;
BEGIN
 -- Calls ops.order_attempt_admission_context, §2: exact authenticated candidate,
 -- unfinished admission stage and sha256(selection::text::bytea), before domain reads.
 v_context:=ops.order_attempt_admission_context(p_attempt,p_selection);
 v_tenant:=(v_context->>'tenant_id')::uuid;
 v_store:=(v_context->>'store_id')::uuid;
 v_owner:=(v_context->>'buyer_owner_id')::uuid;
 IF v_tenant IS NULL OR v_store IS NULL OR v_owner IS NULL THEN
  RAISE EXCEPTION 'attempt_not_found' USING ERRCODE='PT404';
 END IF;
 IF jsonb_typeof(p_selection) IS DISTINCT FROM 'object' OR octet_length(p_selection::text)>65536 THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 IF EXISTS(SELECT 1 FROM jsonb_object_keys(p_selection) k
  WHERE k NOT IN ('schema_version','items','delivery','customer','payment_mode','locale','for','send_payment_link'))
  OR p_selection->'schema_version' IS DISTINCT FROM '1'::jsonb
  OR jsonb_typeof(p_selection->'items') IS DISTINCT FROM 'array'
  OR jsonb_typeof(p_selection->'delivery') IS DISTINCT FROM 'object'
  OR jsonb_typeof(p_selection->'customer') IS DISTINCT FROM 'object'
  OR jsonb_typeof(p_selection->'for') IS DISTINCT FROM 'object'
  OR jsonb_typeof(p_selection->'payment_mode') IS DISTINCT FROM 'string'
  OR jsonb_typeof(p_selection->'locale') IS DISTINCT FROM 'string'
  OR p_selection->>'locale' NOT IN ('zh-CN','zh-TW','en')
  OR jsonb_typeof(p_selection->'send_payment_link') IS DISTINCT FROM 'boolean' THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 v_items:=p_selection->'items'; v_delivery:=p_selection->'delivery';
 v_customer:=p_selection->'customer'; v_for:=p_selection->'for';
 IF jsonb_array_length(v_items) NOT BETWEEN 1 AND 50 THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 FOR v_item IN SELECT value FROM jsonb_array_elements(v_items) LOOP
  IF jsonb_typeof(v_item) IS DISTINCT FROM 'object' THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
  IF EXISTS(SELECT 1 FROM jsonb_object_keys(v_item) k
    WHERE k NOT IN ('sku_id','quantity','ordinal','origin_id','bundle_id','offer_id'))
   OR jsonb_typeof(v_item->'sku_id') IS DISTINCT FROM 'string'
   OR coalesce(v_item->>'sku_id','') !~ v_uuid_pattern
   OR jsonb_typeof(v_item->'quantity') IS DISTINCT FROM 'number'
   OR coalesce(v_item->>'quantity','') !~ '^[0-9]{1,4}$'
   OR jsonb_typeof(v_item->'ordinal') IS DISTINCT FROM 'number'
   OR coalesce(v_item->>'ordinal','') !~ '^[0-9]{1,9}$' THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
  IF (v_item->>'quantity')::integer NOT BETWEEN 1 AND 1000 OR (v_item->>'ordinal')::integer<1
   OR (v_item->>'ordinal')::integer=ANY(v_ordinals) THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
  v_ordinals:=array_append(v_ordinals,(v_item->>'ordinal')::integer);
  FOREACH v_field IN ARRAY ARRAY['origin_id','bundle_id','offer_id'] LOOP
   IF v_item ? v_field AND v_item->v_field<>'null'::jsonb AND
    (jsonb_typeof(v_item->v_field)<>'string' OR coalesce(v_item->>v_field,'') !~ v_uuid_pattern) THEN
    RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
   END IF;
  END LOOP;
  IF ((v_item->>'bundle_id') IS NULL)<>((v_item->>'offer_id') IS NULL)
   OR (v_item->>'bundle_id' IS NOT NULL AND v_item->>'origin_id' IS NULL) THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
  v_key:=(v_item->>'sku_id')||'|'||coalesce(v_item->>'origin_id','');
  IF v_key=ANY(v_keys) THEN RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400'; END IF;
  v_keys:=array_append(v_keys,v_key);
 END LOOP;
 IF EXISTS(SELECT 1 FROM jsonb_object_keys(v_delivery) k
   WHERE k NOT IN ('market_id','country','service_code','home_address','cvs'))
  OR jsonb_typeof(v_delivery->'market_id') IS DISTINCT FROM 'string'
  OR coalesce(v_delivery->>'market_id','') !~ v_uuid_pattern
  OR jsonb_typeof(v_delivery->'country') IS DISTINCT FROM 'string'
  OR coalesce(v_delivery->>'country','') !~ '^[A-Z]{2}$'
  OR jsonb_typeof(v_delivery->'service_code') IS DISTINCT FROM 'string'
  OR coalesce(v_delivery->>'service_code','') !~ '^[a-z][a-z0-9_-]{0,39}$'
  OR ((coalesce(v_delivery->'home_address','null')='null')=(coalesce(v_delivery->'cvs','null')='null')) THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 v_home:=coalesce(v_delivery->'home_address','null')<>'null';
 v_item:=CASE WHEN v_home THEN v_delivery->'home_address' ELSE v_delivery->'cvs' END;
 IF jsonb_typeof(v_item) IS DISTINCT FROM 'object' THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 IF EXISTS(SELECT 1 FROM jsonb_object_keys(v_item) k WHERE
  (v_home AND k NOT IN ('region','city','postal_code','line1','line2')) OR
  (NOT v_home AND k NOT IN ('store_code','store_name','store_address'))) THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 -- Match manual.go's plain-string bounds. Customer/destination are data; no
 -- identity resolution by contact text and no logging of the selection.
 FOREACH v_field IN ARRAY (CASE WHEN v_home THEN ARRAY['city','line1'] ELSE ARRAY['store_code','store_name','store_address'] END) LOOP
  v_value:=v_item->>v_field;
  IF jsonb_typeof(v_item->v_field) IS DISTINCT FROM 'string' OR v_value<>btrim(v_value)
   OR v_value ~ '[[:cntrl:]]' OR char_length(v_value)<CASE WHEN v_field='store_address' THEN 5 ELSE 1 END
   OR char_length(v_value)>CASE v_field WHEN 'line1' THEN 200 WHEN 'city' THEN 100
    WHEN 'store_code' THEN 32 WHEN 'store_name' THEN 40 ELSE 120 END THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
 END LOOP;
 IF v_home THEN
  FOREACH v_field IN ARRAY ARRAY['region','postal_code','line2'] LOOP
   IF v_item ? v_field THEN
    v_value:=v_item->>v_field;
    IF jsonb_typeof(v_item->v_field) IS DISTINCT FROM 'string' OR v_value<>btrim(v_value)
     OR v_value ~ '[[:cntrl:]]' OR char_length(v_value)>CASE v_field WHEN 'region' THEN 100 WHEN 'postal_code' THEN 20 ELSE 200 END THEN
     RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
    END IF;
   END IF;
  END LOOP;
 END IF;
 IF EXISTS(SELECT 1 FROM jsonb_object_keys(v_customer) k WHERE k NOT IN ('name','phone','email')) THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 FOREACH v_field IN ARRAY ARRAY['name','phone','email'] LOOP
  v_value:=v_customer->>v_field;
  IF jsonb_typeof(v_customer->v_field) IS DISTINCT FROM 'string' OR v_value<>btrim(v_value)
   OR v_value ~ '[[:cntrl:]]' OR char_length(v_value)>CASE v_field WHEN 'name' THEN 120 WHEN 'phone' THEN 32 ELSE 254 END THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
 END LOOP;
 IF v_customer->>'name'='' OR v_customer->>'phone' !~ '^[0-9+() -]{6,32}$'
  OR char_length(regexp_replace(v_customer->>'phone','[^0-9]','','g')) NOT BETWEEN 6 AND 20
  OR (v_customer->>'email'<>'' AND v_customer->>'email' !~ '^[^[:space:]@]+@[^[:space:]@]+$') THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 IF EXISTS(SELECT 1 FROM jsonb_object_keys(v_for) k WHERE k NOT IN ('bundle_ids','conversation_id','customer_id'))
  OR jsonb_typeof(v_for->'bundle_ids') IS DISTINCT FROM 'array' THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 IF jsonb_array_length(v_for->'bundle_ids')>50 OR EXISTS(SELECT 1 FROM jsonb_array_elements(v_for->'bundle_ids') x
  WHERE jsonb_typeof(x)<>'string' OR trim(both '"' FROM x::text) !~ v_uuid_pattern) THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 FOREACH v_field IN ARRAY ARRAY['conversation_id','customer_id'] LOOP
  IF v_for ? v_field AND v_for->v_field<>'null'::jsonb AND
   (jsonb_typeof(v_for->v_field)<>'string' OR coalesce(v_for->>v_field,'') !~ v_uuid_pattern) THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
 END LOOP;
 v_market_id:=(v_delivery->>'market_id')::uuid; v_country:=v_delivery->>'country';
 v_code:=v_delivery->>'service_code'; v_mode:=p_selection->>'payment_mode';
 IF v_mode NOT IN ('bank_transfer','cash_on_delivery','pay_at_pickup') THEN
  RAISE EXCEPTION 'payment_mode_unavailable' USING ERRCODE='PT422';
 END IF;
 PERFORM set_config('app.tenant_id',v_tenant::text,true),set_config('app.store_id',v_store::text,true),
  set_config('app.principal_id','',true),set_config('app.buyer_id',v_owner::text,true),set_config('app.buyer_session_id','',true);

 -- Lock order after the coordinator's claims prefix: market -> policy -> products
 -- sorted -> SKUs sorted -> service -> profile -> payment -> allocation. Both C1
 -- and C2 must retain this order; the facts remain locked until the caller commits.
 SELECT id,code,name,currency,version,active INTO v_market FROM pricing.markets
  WHERE tenant_id=v_tenant AND store_id=v_store AND id=v_market_id FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'market_not_found' USING ERRCODE='PT404'; END IF;
 IF NOT v_market.active THEN RAISE EXCEPTION 'market_disabled' USING ERRCODE='PT422'; END IF;
 v_market_json:=to_jsonb(v_market);
 SELECT current_version INTO v_version FROM pricing.policy_heads
  WHERE tenant_id=v_tenant AND store_id=v_store AND market_id=v_market_id AND country=v_country
  AND method='delivery:'||v_code FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'delivery_policy_not_found' USING ERRCODE='PT404'; END IF;
 SELECT market_id,country,method,currency,shipping_mode,tax_mode,tax_basis,version,shipping_minor,tax_rate_bps,
  quote_ttl_seconds,enabled,free_shipping_threshold_minor INTO v_policy FROM pricing.policy_versions
  WHERE tenant_id=v_tenant AND store_id=v_store AND market_id=v_market_id AND country=v_country
  AND method='delivery:'||v_code AND version=v_version;
 IF NOT FOUND OR NOT v_policy.enabled THEN RAISE EXCEPTION 'delivery_policy_disabled' USING ERRCODE='PT422'; END IF;
 IF v_policy.currency<>v_market.currency THEN RAISE EXCEPTION 'currency_mismatch' USING ERRCODE='PT422'; END IF;
 v_policy_json:=jsonb_strip_nulls(to_jsonb(v_policy));
 SELECT jsonb_object_agg(s.id::text,s.product_id::text),count(*) INTO v_parents,v_count
  FROM catalog.skus s WHERE s.tenant_id=v_tenant AND s.store_id=v_store
  AND s.id IN (SELECT (value->>'sku_id')::uuid FROM jsonb_array_elements(v_items));
 IF v_count<>(SELECT count(DISTINCT value->>'sku_id') FROM jsonb_array_elements(v_items)) THEN
  RAISE EXCEPTION 'sku_not_found' USING ERRCODE='PT404';
 END IF;
 FOR v_product IN SELECT id,status FROM catalog.products WHERE tenant_id=v_tenant AND store_id=v_store
  AND id IN (SELECT value::uuid FROM jsonb_each_text(v_parents)) ORDER BY id FOR SHARE LOOP
  IF v_product.status<>'active' THEN RAISE EXCEPTION 'sku_inactive' USING ERRCODE='PT422'; END IF;
 END LOOP;
 FOR v_sku IN SELECT id,product_id,status,currency,max_per_order FROM catalog.skus
  WHERE tenant_id=v_tenant AND store_id=v_store AND id IN (SELECT key::uuid FROM jsonb_each_text(v_parents)) ORDER BY id FOR SHARE LOOP
  IF v_sku.product_id::text IS DISTINCT FROM v_parents->>v_sku.id::text THEN
   RAISE EXCEPTION 'sku_changed' USING ERRCODE='PT422';
  END IF;
  IF v_sku.status<>'active' THEN RAISE EXCEPTION 'sku_inactive' USING ERRCODE='PT422'; END IF;
  IF v_sku.currency<>v_market.currency THEN RAISE EXCEPTION 'currency_mismatch' USING ERRCODE='PT422'; END IF;
  IF (SELECT sum((value->>'quantity')::bigint) FROM jsonb_array_elements(v_items)
   WHERE value->>'sku_id'=v_sku.id::text)>v_sku.max_per_order THEN
   RAISE EXCEPTION 'sku_quantity_exceeds' USING ERRCODE='PT422';
  END IF;
 END LOOP;
 SELECT jsonb_agg(jsonb_build_object('sku_id',s.id,'product_id',s.product_id,'code',s.code,
  'name',p.name,'description',p.description,'sku_version',s.version,'product_version',p.version,
  'quantity',(i.value->>'quantity')::bigint,'unit_price_minor',s.price_minor,'currency',s.currency,
  'inventory_tracked',s.inventory_tracked,'max_per_order',s.max_per_order,'ordinal',(i.value->>'ordinal')::integer)
  || jsonb_strip_nulls(jsonb_build_object('origin_id',i.value->>'origin_id','bundle_id',i.value->>'bundle_id',
   'offer_id',i.value->>'offer_id')) ORDER BY (i.value->>'ordinal')::integer) INTO v_lines
  FROM jsonb_array_elements(v_items) i JOIN catalog.skus s ON s.id=(i.value->>'sku_id')::uuid
   AND s.tenant_id=v_tenant AND s.store_id=v_store JOIN catalog.products p
   ON (p.tenant_id,p.store_id,p.id)=(s.tenant_id,s.store_id,s.product_id);
 IF jsonb_array_length(v_lines) IS DISTINCT FROM jsonb_array_length(v_items) THEN
  RAISE EXCEPTION 'sku_not_found' USING ERRCODE='PT404';
 END IF;
 SELECT current_version INTO v_version FROM fulfillment.service_heads WHERE tenant_id=v_tenant AND store_id=v_store
  AND market_id=v_market_id AND country=v_country AND code=v_code FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'delivery_service_not_found' USING ERRCODE='PT404'; END IF;
 SELECT market_id,country,code,version,policy_method,policy_version,currency,name_hans,name_hant,name_en,
  delivery_kind,mode,enabled,visible,sort_order,binding_id,binding_version INTO v_service FROM fulfillment.service_versions
  WHERE tenant_id=v_tenant AND store_id=v_store AND market_id=v_market_id AND country=v_country AND code=v_code AND version=v_version;
 IF NOT FOUND OR NOT v_service.enabled OR NOT v_service.visible OR v_service.mode NOT IN ('MANUAL','API')
  OR v_service.policy_method<>v_policy.method OR v_service.policy_version<>v_policy.version
  OR (v_service.delivery_kind='home')<>v_home OR (v_home AND v_service.mode<>'MANUAL') THEN
  RAISE EXCEPTION 'delivery_service_unavailable' USING ERRCODE='PT422';
 END IF;
 IF v_service.currency<>v_market.currency THEN RAISE EXCEPTION 'currency_mismatch' USING ERRCODE='PT422'; END IF;
 v_service_json:=jsonb_strip_nulls(to_jsonb(v_service));
 IF NOT v_home THEN
  SELECT pr.qualified_credential_version,a.credential_version INTO v_profile
   FROM integration.ecpay_logistics_profiles pr JOIN integration.merchant_accounts a
    ON (a.tenant_id,a.store_id,a.id)=(pr.tenant_id,pr.store_id,pr.connection_id)
   WHERE pr.tenant_id=v_tenant AND pr.store_id=v_store AND pr.enabled FOR SHARE OF pr;
  v_profiled:=FOUND AND v_profile.qualified_credential_version IS NOT DISTINCT FROM v_profile.credential_version;
  -- This selection carries merchant-entered CVS data, not a provider directory proof.
  IF v_service.mode<>'MANUAL' OR v_profiled THEN RAISE EXCEPTION 'cvs_source_mismatch' USING ERRCODE='PT422'; END IF;
 END IF;
 IF v_mode='bank_transfer' THEN
  SELECT enabled,allow_cvs,window_hours,bank_name,branch,account_name,account_number,version INTO v_payment
   FROM checkout.bank_transfer_settings WHERE tenant_id=v_tenant AND store_id=v_store FOR SHARE;
  IF NOT FOUND OR NOT v_payment.enabled OR (NOT v_home AND NOT v_payment.allow_cvs) THEN
   RAISE EXCEPTION 'bank_transfer_unavailable' USING ERRCODE='PT422';
  END IF;
  v_payment_json:=to_jsonb(v_payment)||jsonb_build_object('mode',v_mode);
 ELSIF v_mode='cash_on_delivery' THEN
  IF NOT v_home OR v_country<>'TW' OR v_market.currency<>'TWD' THEN
   RAISE EXCEPTION 'cash_on_delivery_unavailable' USING ERRCODE='PT422';
  END IF;
  SELECT enabled,max_twd::bigint*100 AS max_minor,surcharge_twd::bigint*100 AS surcharge_minor,carrier,version INTO v_payment
   FROM checkout.cash_on_delivery_settings WHERE tenant_id=v_tenant AND store_id=v_store FOR SHARE;
  IF NOT FOUND OR NOT v_payment.enabled THEN RAISE EXCEPTION 'cash_on_delivery_unavailable' USING ERRCODE='PT422'; END IF;
  IF v_customer->>'name' !~ '^[A-Za-z㐀-䶿一-鿿豈-﫿Ａ-Ｚａ-ｚ]+( [A-Za-z㐀-䶿一-鿿豈-﫿Ａ-Ｚａ-ｚ]+)*$'
   OR (CASE WHEN regexp_replace(v_customer->>'phone','[ ()-]','','g') ~ '^\+8869[0-9]{8}$'
    THEN '0'||substr(regexp_replace(v_customer->>'phone','[ ()-]','','g'),5)
    ELSE regexp_replace(v_customer->>'phone','[ ()-]','','g') END) !~ '^09[0-9]{8}$' THEN
   RAISE EXCEPTION 'cvs_recipient_rejected' USING ERRCODE='PT422';
  END IF;
  v_payment_json:=to_jsonb(v_payment)||jsonb_build_object('mode',v_mode);
 ELSE
  IF v_home OR v_market.currency<>'TWD' OR v_service.delivery_kind NOT IN ('cvs_711','cvs_familymart','cvs_hilife','cvs_okmart') THEN
   RAISE EXCEPTION 'pay_at_pickup_unavailable' USING ERRCODE='PT422';
  END IF;
  SELECT pay_at_pickup_enabled AS enabled,enabled_chains,pay_at_pickup_max_twd::bigint*100 AS max_minor,
   pay_at_pickup_max_open AS max_open,version INTO v_payment FROM fulfillment.cvs_store_settings
   WHERE tenant_id=v_tenant AND store_id=v_store FOR SHARE;
  IF NOT FOUND OR NOT v_payment.enabled OR NOT v_service.delivery_kind=ANY(v_payment.enabled_chains) THEN
   RAISE EXCEPTION 'pay_at_pickup_unavailable' USING ERRCODE='PT422';
  END IF;
  -- Calls fulfillment.ecpay_recipient_ok: storefront-v2 §16.2 offline pickup safety.
  IF NOT fulfillment.ecpay_recipient_ok(v_customer->>'name',v_customer->>'phone') THEN
   RAISE EXCEPTION 'cvs_recipient_rejected' USING ERRCODE='PT422';
  END IF;
  v_payment_json:=to_jsonb(v_payment)||jsonb_build_object('mode',v_mode);
 END IF;
 SELECT current_version INTO v_version FROM fulfillment.allocation_heads WHERE tenant_id=v_tenant AND store_id=v_store
  AND market_id=v_market_id AND country=v_country AND code=v_code FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid_allocation' USING ERRCODE='PT422'; END IF;
 SELECT market_id,country,code,version,service_version,warehouse_count INTO v_allocation FROM fulfillment.allocation_versions
  WHERE tenant_id=v_tenant AND store_id=v_store AND market_id=v_market_id AND country=v_country AND code=v_code AND version=v_version;
 IF NOT FOUND OR v_allocation.warehouse_count NOT BETWEEN 1 AND 16 OR v_allocation.service_version<>v_service.version THEN
  RAISE EXCEPTION 'invalid_allocation' USING ERRCODE='PT422';
 END IF;
 SELECT jsonb_agg(warehouse_id ORDER BY position),count(DISTINCT warehouse_id) INTO v_warehouses,v_count
  FROM fulfillment.allocation_warehouses WHERE tenant_id=v_tenant AND store_id=v_store
  AND market_id=v_market_id AND country=v_country AND code=v_code AND version=v_version;
 IF v_count<>v_allocation.warehouse_count OR jsonb_array_length(v_warehouses) IS DISTINCT FROM v_count THEN
  RAISE EXCEPTION 'invalid_allocation' USING ERRCODE='PT422';
 END IF;
 v_allocation_json:=(to_jsonb(v_allocation)-'warehouse_count')||jsonb_build_object('warehouse_ids',v_warehouses);
 v_result:=jsonb_build_object('market',v_market_json,'policy',v_policy_json,'lines',v_lines,
  'service',v_service_json,'allocation',v_allocation_json,'payment',v_payment_json);
 PERFORM set_config('app.tenant_id',v_old_tenant,true),set_config('app.store_id',v_old_store,true),
  set_config('app.principal_id',v_old_principal,true),set_config('app.buyer_id',v_old_buyer,true),set_config('app.buyer_session_id',v_old_session,true);
 RETURN v_result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',v_old_tenant,true),set_config('app.store_id',v_old_store,true),
  set_config('app.principal_id',v_old_principal,true),set_config('app.buyer_id',v_old_buyer,true),set_config('app.buyer_session_id',v_old_session,true);
 RAISE;
END $$;
ALTER FUNCTION checkout.order_attempt_admission_facts(uuid,jsonb) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_attempt_admission_facts(uuid,jsonb) FROM PUBLIC;
GRANT USAGE ON SCHEMA checkout TO commerce_order_attempt_owner;
GRANT EXECUTE ON FUNCTION checkout.order_attempt_admission_facts(uuid,jsonb) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION checkout.order_attempt_admission_facts(uuid,jsonb) IS
 'order-attempt-v1 §2 C1: exact admitting candidate facts under domain locks; catalog-only origins, offline fees, no stock/cart/order writes. Caller freezes CalculateWith result and seals C1 in the same Tx.';
