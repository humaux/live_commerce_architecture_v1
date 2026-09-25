-- Merchant order reads are a separate authority from buyer capabilities.
-- No existing member is elevated and onboarding replay never restores grants.
ALTER TABLE identity.store_grants DROP CONSTRAINT store_grants_permission_check;
ALTER TABLE identity.store_grants ADD CONSTRAINT store_grants_permission_check CHECK
 (permission IN ('store:read','audit:read','audit:write','catalog:read','catalog:write',
 'inventory:read','inventory:write','inventory:reserve','pricing:read','pricing:write',
 'integration:manage','integration:execute','integration:read','orders:read'));

-- Preserve 0019's principal/session lock order and idempotent onboarding. Only
-- the fresh-store grant list changes; existing stores need explicit provisioning.
CREATE OR REPLACE FUNCTION identity.create_initial_store(p_token bytea,p_key text,p_hash bytea,p_tenant_name text,p_store_name text,p_warehouse_name text,p_currency text)
RETURNS TABLE(tenant_id uuid,store_id uuid,warehouse_id uuid)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_principal uuid; v_tenant uuid; v_store uuid; v_warehouse uuid; v_key text; v_hash bytea;
BEGIN
    IF p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$' OR p_hash IS NULL OR octet_length(p_hash)<>32
       OR p_tenant_name IS NULL OR length(p_tenant_name) NOT BETWEEN 1 AND 120
       OR p_store_name IS NULL OR length(p_store_name) NOT BETWEEN 1 AND 120
       OR p_warehouse_name IS NULL OR length(p_warehouse_name) NOT BETWEEN 1 AND 120
       OR p_currency IS NULL OR p_currency !~ '^[A-Z]{3}$' THEN RAISE EXCEPTION 'invalid store request' USING ERRCODE='PT400'; END IF;
    SELECT s.principal_id INTO v_principal FROM identity.sessions s WHERE s.token_hash=p_token AND s.audience='merchant';
    IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    PERFORM 1 FROM identity.principals p WHERE p.id=v_principal AND p.active FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    PERFORM 1 FROM identity.sessions s WHERE s.token_hash=p_token AND s.principal_id=v_principal AND s.audience='merchant'
      AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    SELECT i.idempotency_key,i.request_hash,i.tenant_id,i.store_id,i.warehouse_id INTO v_key,v_hash,v_tenant,v_store,v_warehouse
      FROM identity.initial_stores i WHERE i.principal_id=v_principal;
    IF FOUND THEN
        IF v_key<>p_key OR v_hash<>p_hash THEN RAISE EXCEPTION 'initial store conflict' USING ERRCODE='PT409'; END IF;
        PERFORM 1 FROM identity.memberships m JOIN control.tenants t ON t.id=m.tenant_id AND t.active
          JOIN control.stores s ON s.tenant_id=t.id AND s.id=v_store AND s.active
          WHERE m.tenant_id=v_tenant AND m.principal_id=v_principal AND m.active;
        IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
        RETURN QUERY SELECT v_tenant,v_store,v_warehouse; RETURN;
    END IF;
    INSERT INTO control.tenants(id,name) VALUES(gen_random_uuid(),p_tenant_name) RETURNING id INTO v_tenant;
    INSERT INTO control.stores(tenant_id,id,name,currency) VALUES(v_tenant,gen_random_uuid(),p_store_name,p_currency) RETURNING id INTO v_store;
    INSERT INTO identity.memberships(tenant_id,principal_id) VALUES(v_tenant,v_principal);
    INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
      SELECT v_tenant,v_store,v_principal,p FROM unnest(ARRAY['store:read','audit:read','audit:write','catalog:read','catalog:write','inventory:read','inventory:write','inventory:reserve','pricing:read','pricing:write','integration:read','integration:manage','orders:read']) p;
    INSERT INTO inventory.warehouses(tenant_id,store_id,name) VALUES(v_tenant,v_store,p_warehouse_name) RETURNING id INTO v_warehouse;
    INSERT INTO identity.initial_stores(principal_id,idempotency_key,request_hash,tenant_id,store_id,warehouse_id)
      VALUES(v_principal,p_key,p_hash,v_tenant,v_store,v_warehouse);
    INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_tenant,v_store,v_principal,'merchant.store_created');
    RETURN QUERY SELECT v_tenant,v_store,v_warehouse;
END $$;
ALTER FUNCTION identity.create_initial_store(bytea,text,bytea,text,text,text,text) OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION identity.create_initial_store(bytea,text,bytea,text,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.create_initial_store(bytea,text,bytea,text,text,text,text) TO commerce_identity;

-- The NOLOGIN definer sees only the columns required for this projection.
-- Runtime still has no checkout schema/table access. Historic 0018 scoped
-- runtime grants on facts/review/work are unchanged, not silently revoked.
GRANT USAGE ON SCHEMA checkout,payments,fulfillment TO commerce_auth;
GRANT SELECT(tenant_id,store_id,owner_id,id,created_at,updated_at,currency,total_minor,
 commercial_state,fulfillment_state,country,service_code,snapshot)
 ON checkout.orders TO commerce_auth;
GRANT SELECT(tenant_id,store_id,owner_id,order_id,id,connection_id,execution_profile,
 environment,currency,amount_minor) ON checkout.payment_attempts TO commerce_auth;
GRANT SELECT(tenant_id,store_id,attempt_id,kind,connection_id,execution_profile,
 environment,currency,amount_minor) ON payments.facts TO commerce_auth;
GRANT SELECT(tenant_id,store_id,attempt_id) ON payments.review_cases TO commerce_auth;
GRANT SELECT(tenant_id,store_id,owner_id,order_id,attempt_id,state)
 ON fulfillment.payment_work_items TO commerce_auth;
CREATE POLICY merchant_order_projection ON checkout.orders FOR SELECT TO commerce_auth USING(true);
CREATE POLICY merchant_order_projection ON checkout.payment_attempts FOR SELECT TO commerce_auth USING(true);
CREATE POLICY merchant_order_projection ON payments.facts FOR SELECT TO commerce_auth USING(true);
CREATE POLICY merchant_order_projection ON payments.review_cases FOR SELECT TO commerce_auth USING(true);
CREATE POLICY merchant_order_projection ON fulfillment.payment_work_items FOR SELECT TO commerce_auth USING(true);
CREATE INDEX merchant_orders_history ON checkout.orders(tenant_id,store_id,created_at DESC,id DESC);

CREATE FUNCTION identity.read_merchant_orders(p_hash bytea,p_store uuid,p_order uuid,
 p_limit integer,p_after_created_at timestamptz,p_after_id uuid,p_state text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_result jsonb; v_auth_error text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL
 OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 101
 OR p_state IS NULL OR p_state NOT IN ('all','DRAFT','AWAITING_PAYMENT','CONFIRMED','CANCELLED')
 OR (p_after_created_at IS NULL)<>(p_after_id IS NULL)
 OR (p_after_created_at IS NOT NULL AND NOT isfinite(p_after_created_at))
 OR (p_order IS NOT NULL AND (p_limit<>1 OR p_after_id IS NOT NULL OR p_state<>'all'))
 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid order read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
 OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
 OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;

 -- ponytail: one bounded projection, no duplicate order engine. All data is
 -- derived in ONE statement snapshot, so capture cannot split state/facts/work.
 WITH owned AS MATERIALIZED (
  SELECT o.id,o.tenant_id,o.store_id,o.owner_id,o.created_at,o.updated_at,o.currency,
   o.total_minor,o.commercial_state,o.fulfillment_state,o.country,o.service_code,
   CASE WHEN p_order IS NULL THEN NULL ELSE o.snapshot END AS snapshot
  FROM checkout.orders o WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store
   AND (p_order IS NULL OR o.id=p_order)
   AND (p_state='all' OR o.commercial_state=p_state)
   AND (p_after_id IS NULL OR (o.created_at,o.id)<(p_after_created_at,p_after_id))
  ORDER BY o.created_at DESC,o.id DESC LIMIT p_limit
 ), projected AS (
  SELECT o.created_at,o.id,
  CASE WHEN a.id IS NOT NULL AND (a.currency<>o.currency OR a.amount_minor<>o.total_minor)
   THEN NULL ELSE jsonb_build_object(
   'order_id',o.id,'created_at',to_char(o.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
   'updated_at',to_char(o.updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
   'currency',o.currency,'total_minor',o.total_minor,'commercial_state',o.commercial_state,
   'fulfillment_state',o.fulfillment_state,
   'payment_state',CASE WHEN a.id IS NULL THEN 'NOT_STARTED'
     WHEN EXISTS(SELECT 1 FROM payments.review_cases rc WHERE rc.tenant_id=a.tenant_id
       AND rc.store_id=a.store_id AND rc.attempt_id=a.id) THEN 'REVIEW_REQUIRED'
     WHEN f.captured THEN 'CAPTURED' WHEN f.authorized THEN 'AUTHORIZED' ELSE 'PENDING' END,
   'test_mode',CASE WHEN a.id IS NULL THEN false ELSE a.execution_profile<>'LIVE' OR a.environment<>'LIVE' END,
   'work_state',coalesce(w.state,'NONE')) ||
  CASE WHEN p_order IS NULL THEN '{}'::jsonb ELSE jsonb_build_object(
   'country',o.country,'service_code',o.service_code,
   'items',(SELECT jsonb_agg(jsonb_build_object('sku_id',line->'sku_id','code',line->'code',
     'name',line->'name','quantity',line->'quantity','unit_price_minor',line->'unit_price_minor',
     'amount',jsonb_build_object('subtotal_minor',line#>'{amount,subtotal_minor}',
       'discount_minor',line#>'{amount,discount_minor}','tax_minor',line#>'{amount,tax_minor}',
       'total_minor',line#>'{amount,total_minor}')) ORDER BY position)
     FROM jsonb_array_elements(o.snapshot#>'{quote,lines}') WITH ORDINALITY AS item(line,position)),
   'totals',jsonb_build_object('subtotal_minor',o.snapshot#>'{quote,amount,subtotal_minor}',
     'discount_minor',o.snapshot#>'{quote,amount,discount_minor}',
     'shipping_minor',o.snapshot#>'{quote,amount,shipping_minor}',
     'shipping_tax_minor',o.snapshot#>'{quote,amount,shipping_tax_minor}',
     'tax_minor',o.snapshot#>'{quote,amount,tax_minor}','total_minor',o.snapshot#>'{quote,amount,total_minor}'),
   'destination',jsonb_build_object('kind',o.snapshot#>'{destination,kind}',
     'country',o.snapshot#>'{destination,country}','recipient_name',o.snapshot#>'{destination,recipient_name}',
     'phone',o.snapshot#>'{destination,phone}',
     'home_address',jsonb_build_object('region',o.snapshot#>'{destination,home_address,region}',
       'city',o.snapshot#>'{destination,home_address,city}','postal_code',o.snapshot#>'{destination,home_address,postal_code}',
       'line1',o.snapshot#>'{destination,home_address,line1}','line2',o.snapshot#>'{destination,home_address,line2}'),
     'pickup',CASE WHEN o.snapshot#>'{destination,pickup}' IS NULL
        OR o.snapshot#>'{destination,pickup}'='null'::jsonb THEN NULL ELSE
       jsonb_build_object('kind',o.snapshot#>'{destination,pickup,kind}',
        'namespace',o.snapshot#>'{destination,pickup,namespace}','code',o.snapshot#>'{destination,pickup,code}',
        'name',o.snapshot#>'{destination,pickup,name}','address',o.snapshot#>'{destination,pickup,address}',
        'verification_kind',o.snapshot#>'{destination,pickup,verification_kind}') END)) END END AS value
  FROM owned o LEFT JOIN checkout.payment_attempts a ON a.tenant_id=o.tenant_id
   AND a.store_id=o.store_id AND a.owner_id=o.owner_id AND a.order_id=o.id
  LEFT JOIN LATERAL (
   SELECT bool_or(fact.kind='CAPTURED') AS captured,bool_or(fact.kind='AUTHORIZED') AS authorized
    FROM payments.facts fact WHERE fact.tenant_id=a.tenant_id AND fact.store_id=a.store_id
     AND fact.attempt_id=a.id AND fact.connection_id=a.connection_id
     AND fact.execution_profile=a.execution_profile AND fact.environment=a.environment
     AND fact.currency=a.currency AND fact.amount_minor=a.amount_minor
     AND fact.currency=o.currency AND fact.amount_minor=o.total_minor
  ) f ON true
  LEFT JOIN fulfillment.payment_work_items w ON w.tenant_id=o.tenant_id AND w.store_id=o.store_id
   AND w.owner_id=o.owner_id AND w.order_id=o.id AND w.attempt_id=a.id
 )
 SELECT coalesce(jsonb_agg(value ORDER BY created_at DESC,id DESC),'[]'::jsonb) INTO v_result FROM projected;

 -- A nested STABLE resolve_access retains the outer statement's snapshot/time.
 -- This direct SELECT is a fresh VOLATILE inner statement after any data wait.
 -- Fence BEFORE empty/not-found branches: no post-revocation existence oracle.
 SELECT CASE WHEN p.id IS NULL THEN 'PT401'
   WHEN st.id IS NULL OR t.id IS NULL OR m.principal_id IS NULL OR gr.permission IS NULL THEN 'PT404'
   WHEN og.permission IS NULL OR st.tenant_id<>s.tenant_id OR p.id<>s.principal_id
     OR m.authz_revision<>s.authz_revision THEN 'PT403' ELSE NULL END INTO v_auth_error
 FROM (VALUES(1)) gate(n)
 LEFT JOIN identity.sessions login ON login.token_hash=p_hash AND login.audience='merchant'
   AND login.revoked_at IS NULL AND login.expires_at>clock_timestamp()
 LEFT JOIN identity.principals p ON p.id=login.principal_id AND p.active
 LEFT JOIN control.stores st ON st.id=p_store AND st.active
 LEFT JOIN control.tenants t ON t.id=st.tenant_id AND t.active
 LEFT JOIN identity.memberships m ON m.tenant_id=st.tenant_id AND m.principal_id=p.id AND m.active
 LEFT JOIN identity.store_grants gr ON gr.tenant_id=st.tenant_id AND gr.store_id=st.id
   AND gr.principal_id=p.id AND gr.permission='store:read'
 LEFT JOIN identity.store_grants og ON og.tenant_id=st.tenant_id AND og.store_id=st.id
   AND og.principal_id=p.id AND og.permission='orders:read';
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'order read access denied' USING ERRCODE=v_auth_error; END IF;
 IF p_order IS NOT NULL AND jsonb_array_length(v_result)=0 THEN
  RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 IF octet_length(v_result::text)>CASE WHEN p_order IS NULL THEN 131072 ELSE 262144 END THEN
  RAISE EXCEPTION 'order read unavailable' USING ERRCODE='PT503'; END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text) TO commerce_runtime;
