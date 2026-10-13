-- Purpose: private OA1 frozen local entered-store and destination materialization.
-- Depends on: 0171 creator/cart FKs, 0173 frozen facts, 0178 execution context,
-- 0179 attempt cart and 0182 deferred atomic-cart COMMIT guard (required together).
-- Used by: the outer C2 coordinator after owner/claim/order/quote/catalog locks.
-- Invariants: I01/I04/I08/I14; service before pickup/destination; no bearer,
-- repricing, newest-address substitution, browser cart mutation or internal commit.

GRANT USAGE ON SCHEMA storefront TO commerce_buyer_writer,commerce_order_attempt_owner,commerce_checkout_writer;
GRANT USAGE ON SCHEMA fulfillment TO commerce_buyer_writer;
GRANT SELECT(tenant_id,store_id,owner_id,id,version,cart_kind,create_attempt_id,creator_session_id)
 ON storefront.carts TO commerce_buyer_writer;
CREATE POLICY order_attempt_destination_cart_read ON storefront.carts FOR SELECT TO commerce_buyer_writer
 USING(cart_kind='order_attempt' AND creator_session_id IS NULL
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid);

GRANT INSERT(tenant_id,store_id,owner_id,id,cart_id,cart_version,creator_session_id,version,kind,country,
 recipient_name,phone,region,city,postal_code,line1,line2,pickup_id,selected_at,expires_at,creator_kind,create_attempt_id)
 ON storefront.destination_snapshots TO commerce_buyer_writer;
GRANT INSERT(tenant_id,store_id,owner_id,cart_id,current_version,destination_id)
 ON storefront.destination_heads TO commerce_buyer_writer;
GRANT SELECT(tenant_id,store_id,owner_id,cart_id) ON storefront.destination_heads TO commerce_buyer_writer;
GRANT INSERT(tenant_id,store_id,owner_id,id,destination_id,session_id,action,created_at,creator_kind,create_attempt_id)
 ON storefront.destination_events TO commerce_buyer_writer;
CREATE POLICY order_attempt_destination_insert ON storefront.destination_snapshots FOR INSERT TO commerce_buyer_writer
 WITH CHECK(creator_kind='order_attempt' AND creator_session_id IS NULL
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid);
CREATE POLICY order_attempt_destination_insert ON storefront.destination_heads FOR INSERT TO commerce_buyer_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND EXISTS(SELECT 1 FROM storefront.carts c WHERE c.tenant_id=destination_heads.tenant_id
   AND c.store_id=destination_heads.store_id AND c.owner_id=destination_heads.owner_id AND c.id=destination_heads.cart_id
   AND c.cart_kind='order_attempt' AND c.create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid));
CREATE POLICY order_attempt_destination_head_read ON storefront.destination_heads FOR SELECT TO commerce_buyer_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND EXISTS(SELECT 1 FROM storefront.carts c WHERE c.tenant_id=destination_heads.tenant_id
   AND c.store_id=destination_heads.store_id AND c.owner_id=destination_heads.owner_id AND c.id=destination_heads.cart_id
   AND c.cart_kind='order_attempt' AND c.create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid));
CREATE POLICY order_attempt_destination_insert ON storefront.destination_events FOR INSERT TO commerce_buyer_writer
 WITH CHECK(creator_kind='order_attempt' AND session_id IS NULL
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid);

-- Checkout already owns the local CVS writer and service-head lock privileges.
-- Extend only its INSERT namespace; browser/provider policies are unchanged.
-- The new namespace must also be reserved against the legacy merchant DML
-- grants and hidden from direct browser-role directory reads. Owned checkout
-- snapshots carry their pickup projection through the existing domain definer.
CREATE POLICY order_attempt_pickup_namespace ON fulfillment.pickup_versions AS RESTRICTIVE FOR INSERT TO commerce_runtime
 WITH CHECK(namespace !~ '^attempt\.');
CREATE POLICY order_attempt_pickup_namespace_insert ON fulfillment.pickup_heads AS RESTRICTIVE FOR INSERT TO commerce_runtime
 WITH CHECK(namespace !~ '^attempt\.');
CREATE POLICY order_attempt_pickup_namespace_update ON fulfillment.pickup_heads AS RESTRICTIVE FOR UPDATE TO commerce_runtime
 USING(namespace !~ '^attempt\.') WITH CHECK(namespace !~ '^attempt\.');
CREATE POLICY order_attempt_pickup_private ON fulfillment.pickup_versions AS RESTRICTIVE FOR SELECT
 TO commerce_buyer_runtime,commerce_checkout_runtime USING(namespace !~ '^attempt\.');
CREATE POLICY order_attempt_pickup_private ON fulfillment.pickup_heads AS RESTRICTIVE FOR SELECT
 TO commerce_buyer_runtime,commerce_checkout_runtime USING(namespace !~ '^attempt\.');
CREATE POLICY order_attempt_pickup_insert ON fulfillment.pickup_versions FOR INSERT TO commerce_checkout_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND namespace='attempt.'||current_setting('app.order_attempt_id',true)
  AND verification_kind='BUYER_ENTERED' AND principal_id IS NULL);
CREATE POLICY order_attempt_pickup_insert ON fulfillment.pickup_heads FOR INSERT TO commerce_checkout_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND namespace='attempt.'||nullif(current_setting('app.order_attempt_id',true),''));

CREATE FUNCTION fulfillment.order_attempt_materialize_pickup(p_attempt uuid,p_plan_plaintext text,p_cart uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 v_context jsonb; v_binding jsonb; v_catalog jsonb; v_service jsonb; v_pickup jsonb;
 v_tenant uuid; v_store uuid; v_owner uuid; v_market uuid; v_country text; v_code text;
 v_current bigint; v_current_service record; v_id uuid; v_result jsonb;
 v_admitted timestamptz; v_deadline timestamptz;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_buyer text:=current_setting('app.buyer_id',true); prior_session text:=current_setting('app.buyer_session_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 v_context:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 v_binding:=v_context->'binding'; v_catalog:=v_context->'plan'->'facts'->'catalog';
 v_service:=v_catalog->'service'; v_pickup:=v_catalog->'pickup';
 v_tenant:=(v_binding->>'tenant_id')::uuid; v_store:=(v_binding->>'store_id')::uuid;
 v_owner:=(v_binding->>'buyer_owner_id')::uuid; v_market:=(v_service->>'market_id')::uuid;
 v_country:=v_service->>'country'; v_code:=v_service->>'code';
 v_admitted:=(v_binding->>'admitted_at')::timestamptz; v_deadline:=(v_binding->>'execute_by')::timestamptz;
 PERFORM set_config('app.tenant_id',v_tenant::text,true),set_config('app.store_id',v_store::text,true),
  set_config('app.buyer_id',v_owner::text,true),set_config('app.buyer_session_id','',true),
  set_config('app.principal_id','',true),set_config('app.order_attempt_id',p_attempt::text,true);
 -- With 0182 a PENDING attempt cart cannot survive an earlier COMMIT. Under
 -- this protected current-xid PENDING fence, a visible exact cart is in this Tx.
 -- Repeat the check here: this private pickup seam must not accept no-cart calls.
 IF NOT EXISTS(SELECT 1 FROM storefront.carts c WHERE c.tenant_id=v_tenant AND c.store_id=v_store
  AND c.owner_id=v_owner AND c.id=p_cart AND c.cart_kind='order_attempt' AND c.create_attempt_id=p_attempt
  AND c.creator_session_id IS NULL AND c.version=1) THEN
  RAISE EXCEPTION 'attempt_cart_conflict' USING ERRCODE='PT409';
 END IF;
 IF jsonb_typeof(v_service) IS DISTINCT FROM 'object' OR v_market IS NULL OR v_country IS NULL OR v_code IS NULL
  OR v_service->>'delivery_kind' NOT IN ('home','cvs_711','cvs_familymart','cvs_hilife','cvs_okmart')
  OR v_service->>'mode' IS DISTINCT FROM 'MANUAL' THEN
  RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
 END IF;
 -- Only safety is current. Visibility, policy/service revisions, names and fees
 -- never replace frozen facts. Hold this service lock before any pickup key.
 SELECT h.current_version INTO v_current FROM fulfillment.service_heads h WHERE h.tenant_id=v_tenant
  AND h.store_id=v_store AND h.market_id=v_market AND h.country=v_country AND h.code=v_code FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'delivery_service_unavailable' USING ERRCODE='PT422'; END IF;
 SELECT s.enabled,s.delivery_kind,s.mode,s.policy_method INTO v_current_service FROM fulfillment.service_versions s
  WHERE s.tenant_id=v_tenant AND s.store_id=v_store AND s.market_id=v_market AND s.country=v_country
  AND s.code=v_code AND s.version=v_current;
 IF NOT FOUND OR NOT v_current_service.enabled
  OR v_current_service.delivery_kind IS DISTINCT FROM v_service->>'delivery_kind'
  OR v_current_service.mode IS DISTINCT FROM v_service->>'mode'
  OR v_current_service.policy_method IS DISTINCT FROM v_service->>'policy_method' THEN
  RAISE EXCEPTION 'delivery_service_unavailable' USING ERRCODE='PT422';
 END IF;
 v_result:='null'::jsonb;
 IF v_service->>'delivery_kind'<>'home' THEN
  IF jsonb_typeof(v_pickup) IS DISTINCT FROM 'object'
   OR v_pickup->>'kind' IS DISTINCT FROM v_service->>'delivery_kind'
   OR v_pickup->>'namespace' IS DISTINCT FROM 'attempt.'||p_attempt::text
   OR v_pickup->>'verification_kind' IS DISTINCT FROM 'BUYER_ENTERED'
   OR v_pickup->>'create_attempt_id' IS DISTINCT FROM p_attempt::text
   OR v_pickup->>'buyer_owner_id' IS DISTINCT FROM v_owner::text
   OR v_pickup->'version' IS DISTINCT FROM '1'::jsonb OR v_pickup->>'country' IS DISTINCT FROM v_country
   OR (v_pickup->>'attested_at')::timestamptz IS DISTINCT FROM v_admitted
   OR (v_pickup->>'valid_until')::timestamptz IS DISTINCT FROM v_admitted+interval '24 hours'
   OR v_pickup->>'id' IS NOT NULL THEN
   RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
  END IF;
  PERFORM pg_advisory_xact_lock(hashtextextended('fulfillment.pickup|'||v_tenant::text||'|'||v_store::text||'|'
   ||(v_pickup->>'kind')||'|'||(v_pickup->>'namespace')||'|'||(v_pickup->>'code'),0));
  PERFORM 1 FROM fulfillment.pickup_heads h WHERE h.tenant_id=v_tenant AND h.store_id=v_store
   AND h.kind=v_pickup->>'kind' AND h.namespace=v_pickup->>'namespace' AND h.code=v_pickup->>'code' FOR UPDATE;
  IF FOUND THEN RAISE EXCEPTION 'attempt_pickup_conflict' USING ERRCODE='PT409'; END IF;
  v_id:=gen_random_uuid();
  INSERT INTO fulfillment.pickup_versions(tenant_id,store_id,id,kind,namespace,code,version,country,name,address,
   verification_kind,evidence_ref,principal_id,attested_at,valid_until,buyer_owner_id,create_attempt_id)
  VALUES(v_tenant,v_store,v_id,v_pickup->>'kind',v_pickup->>'namespace',v_pickup->>'code',1,v_country,
   v_pickup->>'name',v_pickup->>'address','BUYER_ENTERED','attempt-entry:'||p_attempt::text,NULL,
   v_admitted,v_admitted+interval '24 hours',v_owner,p_attempt);
  INSERT INTO fulfillment.pickup_heads(tenant_id,store_id,kind,namespace,code,current_version,pickup_id,enabled)
  VALUES(v_tenant,v_store,v_pickup->>'kind',v_pickup->>'namespace',v_pickup->>'code',1,v_id,true);
  v_result:=(v_pickup-ARRAY['buyer_owner_id','create_attempt_id']::text[])||jsonb_build_object('id',v_id);
 ELSIF v_pickup IS DISTINCT FROM 'null'::jsonb THEN
  RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
 END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.buyer_id',coalesce(prior_buyer,''),true),set_config('app.buyer_session_id',coalesce(prior_session,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RETURN v_result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.buyer_id',coalesce(prior_buyer,''),true),set_config('app.buyer_session_id',coalesce(prior_session,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RAISE;
END $$;
ALTER FUNCTION fulfillment.order_attempt_materialize_pickup(uuid,text,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.order_attempt_materialize_pickup(uuid,text,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.order_attempt_materialize_pickup(uuid,text,uuid) TO commerce_buyer_writer;
COMMENT ON FUNCTION fulfillment.order_attempt_materialize_pickup(uuid,text,uuid) IS
 'Private OA1 storefront dependency: exact protected PENDING plan/cart, current enabled compatible service locked first, then fresh frozen attempt.UUID BUYER_ENTERED pickup. Requires 0182 atomic-cart COMMIT guard. Only buyer domain owner calls; no public capability, current display/fees, order/hold/pay or independent commit.';

CREATE FUNCTION storefront.order_attempt_materialize_destination(p_attempt uuid,p_plan_plaintext text,p_cart uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 v_context jsonb; v_binding jsonb; v_selection jsonb; v_service jsonb; v_pickup jsonb; v_home jsonb; v_destination jsonb;
 v_tenant uuid; v_store uuid; v_owner uuid; v_id uuid; v_pickup_id uuid; v_kind text; v_country text;
 v_admitted timestamptz; v_deadline timestamptz;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_buyer text:=current_setting('app.buyer_id',true); prior_session text:=current_setting('app.buyer_session_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 v_context:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 v_binding:=v_context->'binding'; v_selection:=v_context->'plan'->'facts'->'selection';
 v_service:=v_context->'plan'->'facts'->'catalog'->'service';
 v_tenant:=(v_binding->>'tenant_id')::uuid; v_store:=(v_binding->>'store_id')::uuid;
 v_owner:=(v_binding->>'buyer_owner_id')::uuid; v_kind:=v_service->>'delivery_kind';
 v_country:=v_selection->'delivery'->>'country';
 v_admitted:=(v_binding->>'admitted_at')::timestamptz; v_deadline:=(v_binding->>'execute_by')::timestamptz;
 PERFORM set_config('app.tenant_id',v_tenant::text,true),set_config('app.store_id',v_store::text,true),
  set_config('app.buyer_id',v_owner::text,true),set_config('app.buyer_session_id','',true),
  set_config('app.principal_id','',true),set_config('app.order_attempt_id',p_attempt::text,true);
 IF NOT EXISTS(SELECT 1 FROM storefront.carts c WHERE c.tenant_id=v_tenant AND c.store_id=v_store
  AND c.owner_id=v_owner AND c.id=p_cart AND c.cart_kind='order_attempt' AND c.create_attempt_id=p_attempt
  AND c.creator_session_id IS NULL AND c.version=1) THEN
  RAISE EXCEPTION 'attempt_cart_conflict' USING ERRCODE='PT409';
 END IF;
 IF v_kind IS NULL OR v_kind NOT IN ('home','cvs_711','cvs_familymart','cvs_hilife','cvs_okmart')
  OR v_country IS DISTINCT FROM v_service->>'country' OR v_deadline>v_admitted+interval '30 minutes' THEN
  RAISE EXCEPTION 'invalid_order_attempt_plan' USING ERRCODE='PT400';
 END IF;
 -- Fulfillment owns service/pickup writes; this call retains the service-first
 -- lock suffix even for home (where it returns JSON null and writes no pickup).
 v_pickup:=fulfillment.order_attempt_materialize_pickup(p_attempt,p_plan_plaintext,p_cart);
 v_pickup_id:=(v_pickup->>'id')::uuid;
 v_home:=jsonb_build_object('region','','city','','postal_code','','line1','','line2','');
 IF v_kind='home' THEN
  v_home:=jsonb_build_object('region',coalesce(v_selection->'delivery'->'home_address'->>'region',''),
   'city',coalesce(v_selection->'delivery'->'home_address'->>'city',''),
   'postal_code',coalesce(v_selection->'delivery'->'home_address'->>'postal_code',''),
   'line1',coalesce(v_selection->'delivery'->'home_address'->>'line1',''),
   'line2',coalesce(v_selection->'delivery'->'home_address'->>'line2',''));
 END IF;
 -- Fresh attempt cart: serialize the missing destination head with its normal
 -- cart advisory key. No owner/cart browser lock or earlier graph level is taken.
 PERFORM pg_advisory_xact_lock(hashtextextended('destination|'||v_tenant::text||'|'||v_store::text||'|'||v_owner::text||'|'||p_cart::text,0));
 IF EXISTS(SELECT 1 FROM storefront.destination_heads h WHERE h.tenant_id=v_tenant AND h.store_id=v_store
  AND h.owner_id=v_owner AND h.cart_id=p_cart) THEN
  RAISE EXCEPTION 'attempt_destination_conflict' USING ERRCODE='PT409';
 END IF;
 v_id:=gen_random_uuid();
 INSERT INTO storefront.destination_snapshots(tenant_id,store_id,owner_id,id,cart_id,cart_version,creator_session_id,version,
  kind,country,recipient_name,phone,region,city,postal_code,line1,line2,pickup_id,selected_at,expires_at,creator_kind,create_attempt_id)
 VALUES(v_tenant,v_store,v_owner,v_id,p_cart,1,NULL,1,v_kind,v_country,v_selection->'customer'->>'name',
  v_selection->'customer'->>'phone',v_home->>'region',v_home->>'city',v_home->>'postal_code',v_home->>'line1',
  v_home->>'line2',v_pickup_id,v_admitted,v_deadline,'order_attempt',p_attempt);
 INSERT INTO storefront.destination_heads(tenant_id,store_id,owner_id,cart_id,current_version,destination_id)
 VALUES(v_tenant,v_store,v_owner,p_cart,1,v_id);
 INSERT INTO storefront.destination_events(tenant_id,store_id,owner_id,id,destination_id,session_id,action,created_at,creator_kind,create_attempt_id)
 VALUES(v_tenant,v_store,v_owner,gen_random_uuid(),v_id,NULL,'destination.selected',v_admitted,'order_attempt',p_attempt);
 v_destination:=jsonb_build_object('id',v_id,'version',1,'cart_id',p_cart,'cart_version',1,'kind',v_kind,'country',v_country,
  'recipient_name',v_selection->'customer'->>'name','phone',v_selection->'customer'->>'phone','home_address',v_home,
  'selected_at',v_admitted,'expires_at',v_deadline);
 IF v_pickup_id IS NOT NULL THEN v_destination:=v_destination||jsonb_build_object('pickup',v_pickup); END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.buyer_id',coalesce(prior_buyer,''),true),set_config('app.buyer_session_id',coalesce(prior_session,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RETURN jsonb_build_object('destination_id',v_id,'destination',v_destination);
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.buyer_id',coalesce(prior_buyer,''),true),set_config('app.buyer_session_id',coalesce(prior_session,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RAISE;
END $$;
ALTER FUNCTION storefront.order_attempt_materialize_destination(uuid,text,uuid) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION storefront.order_attempt_materialize_destination(uuid,text,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION storefront.order_attempt_materialize_destination(uuid,text,uuid) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION storefront.order_attempt_materialize_destination(uuid,text,uuid) IS
 'Private C2 storefront materializer: exact protected plan/attempt-owned cart, service-before-pickup-before-destination locks, immutable C1 contact/home/BUYER_ENTERED source, admitted_at/execute_by destination lifetime. Requires 0182 atomic-cart COMMIT guard. Returns {destination_id,destination}; coordinator only; no browser cart/session, current price/visibility/address, order/hold/pay/claim or internal commit.';
