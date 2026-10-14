-- Purpose: private OA1 frozen attempt-cart/quote materialization in the outer C2 transaction.
-- Depends on: 0171 provenance/FKs, 0178 protected execution context, frozen C1 facts/calculation.
-- Used by: commerce_order_attempt_owner after the owner/claim/order prefix; never a public buyer entry.
-- Invariants: I01/I04/I08/I14; no current pricing, buyer cart replacement, capability, or independent commit.

GRANT USAGE ON SCHEMA storefront TO commerce_buyer_writer,commerce_order_attempt_owner;
GRANT INSERT(tenant_id,store_id,owner_id,id,creator_session_id,currency,version,cart_kind,create_attempt_id)
 ON storefront.carts TO commerce_buyer_writer;
GRANT INSERT(tenant_id,store_id,owner_id,cart_id,sku_id,quantity,cart_kind,create_attempt_id,line_id,line_ordinal,origin_kind,origin_id)
 ON storefront.cart_lines TO commerce_buyer_writer;
GRANT INSERT(tenant_id,store_id,owner_id,id,cart_id,creator_session_id,cart_version,market_id,market_version,
 country,method,policy_version,currency,created_at,expires_at,snapshot,creator_kind,create_attempt_id)
 ON storefront.quotes TO commerce_buyer_writer;
GRANT INSERT(tenant_id,store_id,owner_id,id,session_id,cart_id,quote_id,action,created_at,creator_kind,create_attempt_id)
 ON storefront.events TO commerce_buyer_writer;

-- This NOLOGIN writer has no storefront SELECT/UPDATE/DELETE authority. Its new
-- INSERT policies accept only the exact attempt context established by the definer;
-- the restrictive browser provenance policies and all existing FKs remain intact.
CREATE POLICY order_attempt_quote_insert ON storefront.carts FOR INSERT TO commerce_buyer_writer
 WITH CHECK(cart_kind='order_attempt' AND creator_session_id IS NULL
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid);
CREATE POLICY order_attempt_quote_insert ON storefront.cart_lines FOR INSERT TO commerce_buyer_writer
 WITH CHECK(cart_kind='order_attempt' AND origin_kind IN ('manual','claim')
  AND origin_id<>'00000000-0000-0000-0000-000000000000'::uuid
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid);
CREATE POLICY order_attempt_quote_insert ON storefront.quotes FOR INSERT TO commerce_buyer_writer
 WITH CHECK(creator_kind='order_attempt' AND creator_session_id IS NULL
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid);
CREATE POLICY order_attempt_quote_insert ON storefront.events FOR INSERT TO commerce_buyer_writer
 WITH CHECK(creator_kind='order_attempt' AND session_id IS NULL
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid);

CREATE FUNCTION storefront.order_attempt_materialize_quote(p_attempt uuid,p_plan_plaintext text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 v_context jsonb; v_binding jsonb; v_plan jsonb; v_catalog jsonb; v_claims jsonb;
 v_calculation jsonb; v_lines jsonb:='[]'::jsonb; v_line jsonb; v_claim jsonb; v_quote jsonb; v_snapshot_line jsonb;
 v_tenant uuid; v_store uuid; v_owner uuid; v_cart uuid; v_quote_id uuid;
 v_admitted timestamptz; v_deadline timestamptz; v_origin uuid; v_line_id uuid;
 v_ordinal bigint; v_last_ordinal bigint:=0; v_index integer:=0; v_claim_count integer; v_seen_claims integer:=0;
 v_price bigint; v_kind text; v_currency text;
 v_uuid_pattern constant text:='^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
 prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true);
 prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 -- ops owns the protected current-transaction witness, exact plaintext digest,
 -- immutable metadata and deadline check. Caller JSON/GUCs never authorize writes.
 v_context:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 v_binding:=v_context->'binding'; v_plan:=v_context->'plan';
 v_tenant:=(v_binding->>'tenant_id')::uuid;
 v_store:=(v_binding->>'store_id')::uuid;
 v_owner:=(v_binding->>'buyer_owner_id')::uuid;
 v_admitted:=(v_binding->>'admitted_at')::timestamptz;
 v_deadline:=(v_binding->>'execute_by')::timestamptz;
 v_catalog:=v_plan->'facts'->'catalog';
 v_claims:=v_plan->'facts'->'claims'->'lines';
 v_calculation:=v_plan->'calculation';
 v_currency:=v_catalog->'market'->>'currency';
 IF v_tenant IS NULL OR v_store IS NULL OR v_owner IS NULL
  OR v_binding->>'attempt_id' IS DISTINCT FROM p_attempt::text
  OR v_admitted IS NULL OR v_deadline IS NULL OR v_currency IS NULL
  OR jsonb_typeof(v_catalog->'lines') IS DISTINCT FROM 'array'
  OR jsonb_typeof(v_claims) IS DISTINCT FROM 'array'
  OR jsonb_typeof(v_calculation->'lines') IS DISTINCT FROM 'array'
  OR jsonb_typeof(v_catalog->'policy') IS DISTINCT FROM 'object' THEN
  RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
 END IF;
 IF jsonb_array_length(v_catalog->'lines') NOT BETWEEN 1 AND 50
  OR jsonb_array_length(v_catalog->'lines')<>jsonb_array_length(v_calculation->'lines') THEN
  RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
 END IF;
 FOR v_line IN SELECT value FROM jsonb_array_elements(v_catalog->'lines') LOOP
  -- Old/malformed sealed plans without C1's server-owned origin fail closed.
  -- C2 allocates physical line UUIDs but never invents or repairs logical origins.
  IF jsonb_typeof(v_line->'origin_id') IS DISTINCT FROM 'string'
   OR coalesce(v_line->>'origin_id','') !~ v_uuid_pattern
   OR v_line->>'origin_id'='00000000-0000-0000-0000-000000000000'
   OR jsonb_typeof(v_line->'ordinal') IS DISTINCT FROM 'number'
   OR coalesce(v_line->>'ordinal','') !~ '^[0-9]{1,9}$'
   OR v_line->>'currency' IS DISTINCT FROM v_currency THEN
   RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
  END IF;
  v_ordinal:=(v_line->>'ordinal')::bigint;
  IF v_ordinal<=v_last_ordinal THEN
   RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
  END IF;
  v_last_ordinal:=v_ordinal; v_origin:=(v_line->>'origin_id')::uuid;
  v_price:=(v_line->>'unit_price_minor')::bigint; v_kind:='manual';
  SELECT count(*) INTO v_claim_count FROM jsonb_array_elements(v_claims) c
   WHERE c.value->>'ordinal'=v_line->>'ordinal';
  IF (nullif(v_line->>'bundle_id','') IS NULL)<>(nullif(v_line->>'offer_id','') IS NULL) THEN
   RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
  END IF;
  IF nullif(v_line->>'bundle_id','') IS NOT NULL THEN
   v_kind:='claim';
   IF v_claim_count<>1 THEN RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400'; END IF;
   SELECT value INTO v_claim FROM jsonb_array_elements(v_claims)
    WHERE value->>'ordinal'=v_line->>'ordinal';
   IF v_claim->>'origin_id' IS DISTINCT FROM v_line->>'origin_id'
    OR v_claim->>'bundle_id' IS DISTINCT FROM v_line->>'bundle_id'
    OR v_claim->>'offer_id' IS DISTINCT FROM v_line->>'offer_id'
    OR v_claim->>'sku_id' IS DISTINCT FROM v_line->>'sku_id'
    OR v_claim->>'quantity' IS DISTINCT FROM v_line->>'quantity' THEN
    RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
   END IF;
   v_seen_claims:=v_seen_claims+1;
   IF v_claim->'live_price_minor' IS DISTINCT FROM 'null'::jsonb THEN
    IF jsonb_typeof(v_claim->'live_price_minor') IS DISTINCT FROM 'number'
     OR v_claim->>'reason' IS DISTINCT FROM '' THEN
     RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
    END IF;
    v_price:=(v_claim->>'live_price_minor')::bigint;
    IF v_price NOT BETWEEN 1 AND 1000000000000 THEN
     RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
    END IF;
   END IF;
  ELSIF v_claim_count<>0 THEN
   RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
  END IF;
  IF v_price IS NULL OR v_price NOT BETWEEN 0 AND 1000000000000 THEN
   RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
  END IF;
  v_line_id:=gen_random_uuid();
  v_snapshot_line:=jsonb_build_object('sku_id',v_line->'sku_id','product_id',v_line->'product_id',
   'code',v_line->'code','name',v_line->'name','description',v_line->'description',
   'sku_version',v_line->'sku_version','product_version',v_line->'product_version',
   'quantity',v_line->'quantity','unit_price_minor',v_price,'amount',v_calculation->'lines'->v_index,
   'line_id',v_line_id,'line_ordinal',v_ordinal,'origin_kind',v_kind,'origin_id',v_origin);
  IF v_kind='claim' AND v_claim->'live_price_minor' IS DISTINCT FROM 'null'::jsonb THEN
   v_snapshot_line:=v_snapshot_line||jsonb_build_object('price_rule','live_claim',
    'catalog_unit_price_minor',v_line->'unit_price_minor','claim_bundle_id',v_line->'bundle_id',
    'claim_offer_id',v_line->'offer_id');
  END IF;
  v_lines:=v_lines||jsonb_build_array(v_snapshot_line); v_index:=v_index+1;
 END LOOP;
 IF v_seen_claims<>jsonb_array_length(v_claims) THEN
  RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
 END IF;
 v_cart:=gen_random_uuid(); v_quote_id:=gen_random_uuid();
 -- I08: amounts are copied from C1's existing Go calculator, never recomputed in
 -- SQL or read from current market/policy/SKU/offer rows. The parent validates the
 -- frozen arithmetic in Go before composing this helper inside the same Tx.
 v_quote:=jsonb_build_object('id',v_quote_id,'cart_id',v_cart,'currency',v_currency,
  'calculation_version','v1','cart_version',1,'market_version',v_catalog->'market'->'version',
  'policy',v_catalog->'policy','lines',v_lines,'amount',v_calculation,
  'created_at',v_admitted,'expires_at',v_deadline);
 PERFORM set_config('app.tenant_id',v_tenant::text,true),set_config('app.store_id',v_store::text,true),
  set_config('app.buyer_id',v_owner::text,true),set_config('app.buyer_session_id','',true),
  set_config('app.order_attempt_id',p_attempt::text,true);
 INSERT INTO storefront.carts(tenant_id,store_id,owner_id,id,creator_session_id,currency,version,cart_kind,create_attempt_id)
  VALUES(v_tenant,v_store,v_owner,v_cart,NULL,v_currency,1,'order_attempt',p_attempt);
 -- The new-row SKU FK acquires catalog KEY SHARE. Its acquisition is deterministic
 -- by SKU then ordinal; the snapshot above remains aligned by frozen ordinal.
 -- No explicit owner/claim/order/scope/subject or current catalog locks occur here.
 FOR v_line IN SELECT value FROM jsonb_array_elements(v_lines)
  ORDER BY (value->>'sku_id')::uuid,(value->>'line_ordinal')::bigint LOOP
  INSERT INTO storefront.cart_lines(tenant_id,store_id,owner_id,cart_id,sku_id,quantity,cart_kind,
   create_attempt_id,line_id,line_ordinal,origin_kind,origin_id)
   VALUES(v_tenant,v_store,v_owner,v_cart,(v_line->>'sku_id')::uuid,(v_line->>'quantity')::bigint,
    'order_attempt',p_attempt,(v_line->>'line_id')::uuid,(v_line->>'line_ordinal')::bigint,
    v_line->>'origin_kind',(v_line->>'origin_id')::uuid);
 END LOOP;
 INSERT INTO storefront.quotes(tenant_id,store_id,owner_id,id,cart_id,creator_session_id,cart_version,
  market_id,market_version,country,method,policy_version,currency,created_at,expires_at,snapshot,creator_kind,create_attempt_id)
  VALUES(v_tenant,v_store,v_owner,v_quote_id,v_cart,NULL,1,(v_catalog->'market'->>'id')::uuid,
   (v_catalog->'market'->>'version')::bigint,v_catalog->'policy'->>'country',v_catalog->'policy'->>'method',
   (v_catalog->'policy'->>'version')::bigint,v_currency,v_admitted,v_deadline,v_quote,'order_attempt',p_attempt);
 INSERT INTO storefront.events(tenant_id,store_id,owner_id,id,session_id,cart_id,quote_id,action,created_at,creator_kind,create_attempt_id)
  VALUES(v_tenant,v_store,v_owner,gen_random_uuid(),NULL,v_cart,NULL,'cart.updated',v_admitted,'order_attempt',p_attempt),
   (v_tenant,v_store,v_owner,gen_random_uuid(),NULL,v_cart,v_quote_id,'quote.created',v_admitted,'order_attempt',p_attempt);
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.buyer_id',coalesce(prior_buyer,''),true),set_config('app.buyer_session_id',coalesce(prior_session,''),true),
  set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RETURN jsonb_build_object('cart_id',v_cart,'quote_id',v_quote_id,'cart_version',1,'quote',v_quote);
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.buyer_id',coalesce(prior_buyer,''),true),set_config('app.buyer_session_id',coalesce(prior_session,''),true),
  set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RAISE;
END $$;
ALTER FUNCTION storefront.order_attempt_materialize_quote(uuid,text) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION storefront.order_attempt_materialize_quote(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION storefront.order_attempt_materialize_quote(uuid,text) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION storefront.order_attempt_materialize_quote(uuid,text) IS
 'storefront/orderattempt private C2 seam: coordinator EXECUTE only, buyer NOLOGIN owner; protected ops execution context binds exact plan/transaction/identity/deadline. Inserts only its attempt cart(v1), sorted physical lines, frozen quote and provenance events in the caller Tx. Never current pricing, browser cart mutation, capability, destination, hold/order/claim writes or standalone commit; all caller GUCs restored.';
