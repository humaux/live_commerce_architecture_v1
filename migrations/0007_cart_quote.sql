-- Internal-only cart/quote pricing. No inventory or provider writes.
CREATE SCHEMA pricing;
CREATE SCHEMA storefront;
REVOKE ALL ON SCHEMA pricing,storefront FROM PUBLIC;
GRANT USAGE ON SCHEMA pricing TO commerce_runtime,commerce_buyer_runtime;
GRANT USAGE ON SCHEMA storefront,catalog,control TO commerce_buyer_runtime;
ALTER TABLE control.stores ADD CONSTRAINT stores_currency_key UNIQUE(tenant_id,id,currency);
ALTER TABLE identity.store_grants DROP CONSTRAINT store_grants_permission_check;
ALTER TABLE identity.store_grants ADD CONSTRAINT store_grants_permission_check CHECK
(permission IN ('store:read','audit:read','audit:write','catalog:read','catalog:write','inventory:read','inventory:write','inventory:reserve','pricing:read','pricing:write'));

CREATE TABLE pricing.markets (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL DEFAULT gen_random_uuid(),
 code text NOT NULL CHECK(code ~ '^[a-z][a-z0-9_-]{0,39}$'),
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 120),
 currency text NOT NULL CHECK(currency ~ '^[A-Z]{3}$'),
 active boolean NOT NULL DEFAULT true, version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 principal_id uuid NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,id), UNIQUE(tenant_id,store_id,code),
 UNIQUE(tenant_id,store_id,id,currency),
 FOREIGN KEY(tenant_id,store_id,currency) REFERENCES control.stores(tenant_id,id,currency),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);
CREATE TABLE pricing.policy_versions (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, market_id uuid NOT NULL,
 country text NOT NULL CHECK(country ~ '^[A-Z]{2}$'),
 method text NOT NULL CHECK(method IN ('home','cvs_711','cvs_familymart')),
 version bigint NOT NULL CHECK(version>0), currency text NOT NULL,
 shipping_mode text NOT NULL CHECK(shipping_mode='country_flat'),
 shipping_minor bigint NOT NULL CHECK(shipping_minor BETWEEN 0 AND 1000000000000),
 tax_mode text NOT NULL CHECK(tax_mode IN ('none','exclusive','inclusive')),
 tax_basis text NOT NULL CHECK(tax_basis IN ('goods','goods_and_shipping')),
 tax_rate_bps bigint NOT NULL CHECK(tax_rate_bps BETWEEN 0 AND 10000),
 quote_ttl_seconds bigint NOT NULL CHECK(quote_ttl_seconds BETWEEN 60 AND 1800),
 enabled boolean NOT NULL, configuration_ref text NOT NULL CHECK(length(configuration_ref) BETWEEN 1 AND 240),
 principal_id uuid NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,market_id,country,method,version),
 UNIQUE(tenant_id,store_id,market_id,country,method,version,currency),
 CHECK(tax_mode<>'none' OR tax_rate_bps=0),
 FOREIGN KEY(tenant_id,store_id,market_id,currency) REFERENCES pricing.markets(tenant_id,store_id,id,currency),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);
CREATE TABLE pricing.policy_heads (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, market_id uuid NOT NULL,
 country text NOT NULL, method text NOT NULL, current_version bigint NOT NULL,
 PRIMARY KEY(tenant_id,store_id,market_id,country,method),
 FOREIGN KEY(tenant_id,store_id,market_id,country,method,current_version)
 REFERENCES pricing.policy_versions(tenant_id,store_id,market_id,country,method,version)
);
CREATE TABLE buyer.command_results (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,session_id uuid NOT NULL,
 operation text NOT NULL CHECK(operation ~ '^[a-z][a-z0-9_.:]{0,79}$'),
 idempotency_key text NOT NULL CHECK(idempotency_key ~ '^[A-Za-z0-9_.:-]{8,128}$'),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 response jsonb NOT NULL CHECK(octet_length(response::text)<=1048576),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,owner_id,session_id,operation,idempotency_key),
 FOREIGN KEY(tenant_id,store_id,owner_id,session_id)
 REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id)
);
CREATE TABLE storefront.carts (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,
 id uuid NOT NULL DEFAULT gen_random_uuid(),creator_session_id uuid NOT NULL,
 currency text NOT NULL,version bigint NOT NULL DEFAULT 0 CHECK(version>=0),
 PRIMARY KEY(tenant_id,store_id,owner_id,id),UNIQUE(tenant_id,store_id,owner_id),
 FOREIGN KEY(tenant_id,store_id,currency) REFERENCES control.stores(tenant_id,id,currency),
 FOREIGN KEY(tenant_id,store_id,owner_id,creator_session_id)
 REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id)
);
CREATE TABLE storefront.cart_lines (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,
 cart_id uuid NOT NULL,sku_id uuid NOT NULL,quantity bigint NOT NULL CHECK(quantity BETWEEN 1 AND 1000000000),
 PRIMARY KEY(tenant_id,store_id,owner_id,cart_id,sku_id),
 FOREIGN KEY(tenant_id,store_id,owner_id,cart_id) REFERENCES storefront.carts(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,sku_id) REFERENCES catalog.skus(tenant_id,store_id,id)
);
CREATE TABLE storefront.quotes (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,
 id uuid NOT NULL DEFAULT gen_random_uuid(),cart_id uuid NOT NULL,creator_session_id uuid NOT NULL,
 cart_version bigint NOT NULL CHECK(cart_version>0),
 market_id uuid NOT NULL,market_version bigint NOT NULL CHECK(market_version>0),
 country text NOT NULL,method text NOT NULL,policy_version bigint NOT NULL,currency text NOT NULL,
 created_at timestamptz NOT NULL,expires_at timestamptz NOT NULL,
 snapshot jsonb NOT NULL CHECK(jsonb_typeof(snapshot)='object' AND octet_length(snapshot::text)<=1048576),
 PRIMARY KEY(tenant_id,store_id,owner_id,id),
 UNIQUE(tenant_id,store_id,owner_id,cart_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,cart_id) REFERENCES storefront.carts(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,creator_session_id)
 REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,market_id,country,method,policy_version,currency)
 REFERENCES pricing.policy_versions(tenant_id,store_id,market_id,country,method,version,currency),
 CHECK(expires_at>created_at AND expires_at<=created_at+interval '30 minutes'),
 -- No caller-provided opaque snapshot: relational bindings also guard future readers.
 CHECK(coalesce(snapshot->>'id'=id::text AND snapshot->>'cart_id'=cart_id::text
   AND (snapshot->>'cart_version')::bigint=cart_version
   AND (snapshot->>'market_version')::bigint=market_version
   AND snapshot->>'currency'=currency
   AND snapshot->>'calculation_version'='v1'
   AND snapshot->'policy'->>'market_id'=market_id::text
   AND snapshot->'policy'->>'country'=country AND snapshot->'policy'->>'method'=method
   AND (snapshot->'policy'->>'version')::bigint=policy_version
   AND snapshot->'policy'->>'currency'=currency
   AND (snapshot->>'created_at')::timestamptz=created_at
   AND (snapshot->>'expires_at')::timestamptz=expires_at,false))
);
CREATE TABLE storefront.events (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,
 id uuid NOT NULL DEFAULT gen_random_uuid(),session_id uuid NOT NULL,cart_id uuid NOT NULL,quote_id uuid,
 action text NOT NULL CHECK(action IN ('cart.updated','quote.created')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,session_id)
 REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,cart_id) REFERENCES storefront.carts(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,cart_id,quote_id) REFERENCES storefront.quotes(tenant_id,store_id,owner_id,cart_id,id),
 CHECK((action='cart.updated' AND quote_id IS NULL) OR (action='quote.created' AND quote_id IS NOT NULL))
);

ALTER TABLE pricing.markets ENABLE ROW LEVEL SECURITY;
ALTER TABLE pricing.markets FORCE ROW LEVEL SECURITY;

ALTER TABLE pricing.policy_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE pricing.policy_versions FORCE ROW LEVEL SECURITY;

ALTER TABLE pricing.policy_heads ENABLE ROW LEVEL SECURITY;
ALTER TABLE pricing.policy_heads FORCE ROW LEVEL SECURITY;

ALTER TABLE buyer.command_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE buyer.command_results FORCE ROW LEVEL SECURITY;

ALTER TABLE storefront.carts ENABLE ROW LEVEL SECURITY;
ALTER TABLE storefront.carts FORCE ROW LEVEL SECURITY;

ALTER TABLE storefront.cart_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE storefront.cart_lines FORCE ROW LEVEL SECURITY;

ALTER TABLE storefront.quotes ENABLE ROW LEVEL SECURITY;
ALTER TABLE storefront.quotes FORCE ROW LEVEL SECURITY;

ALTER TABLE storefront.events ENABLE ROW LEVEL SECURITY;
ALTER TABLE storefront.events FORCE ROW LEVEL SECURITY;
CREATE POLICY merchant_read ON pricing.markets FOR SELECT TO commerce_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY merchant_insert ON pricing.markets FOR INSERT TO commerce_runtime WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY buyer_read ON pricing.markets FOR SELECT TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY merchant_read ON pricing.policy_versions FOR SELECT TO commerce_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY merchant_insert ON pricing.policy_versions FOR INSERT TO commerce_runtime WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY buyer_read ON pricing.policy_versions FOR SELECT TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY merchant_read ON pricing.policy_heads FOR SELECT TO commerce_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY merchant_insert ON pricing.policy_heads FOR INSERT TO commerce_runtime WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY buyer_read ON pricing.policy_heads FOR SELECT TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY merchant_update ON pricing.markets FOR UPDATE TO commerce_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid) WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY merchant_update ON pricing.policy_heads FOR UPDATE TO commerce_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid) WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT,INSERT ON pricing.markets,pricing.policy_versions,pricing.policy_heads TO commerce_runtime;
GRANT UPDATE(active,version) ON pricing.markets TO commerce_runtime;
GRANT UPDATE(current_version) ON pricing.policy_heads TO commerce_runtime;
GRANT SELECT(tenant_id,store_id,id,code,name,currency,active,version) ON pricing.markets TO commerce_buyer_runtime;
GRANT SELECT(tenant_id,store_id,market_id,country,method,version,currency,shipping_mode,shipping_minor,tax_mode,tax_basis,tax_rate_bps,quote_ttl_seconds,enabled) ON pricing.policy_versions TO commerce_buyer_runtime;
GRANT SELECT ON pricing.policy_heads TO commerce_buyer_runtime;
-- PG row locking needs both UPDATE privilege and row visibility. WITH CHECK
-- false prohibits UPDATE, including a no-op update; it does not prohibit SHARE.
GRANT UPDATE(id) ON pricing.markets,catalog.products,catalog.skus TO commerce_buyer_runtime;
GRANT UPDATE(current_version) ON pricing.policy_heads TO commerce_buyer_runtime;
CREATE POLICY buyer_lock ON pricing.markets FOR UPDATE TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid) WITH CHECK(false);
CREATE POLICY buyer_lock ON pricing.policy_heads FOR UPDATE TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid) WITH CHECK(false);
CREATE POLICY buyer_lock ON catalog.products FOR UPDATE TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid) WITH CHECK(false);
CREATE POLICY buyer_lock ON catalog.skus FOR UPDATE TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid) WITH CHECK(false);
CREATE POLICY buyer_read ON catalog.products FOR SELECT TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY buyer_read ON catalog.skus FOR SELECT TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY buyer_read ON control.stores FOR SELECT TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT(tenant_id,store_id,id,name,description,status,version) ON catalog.products TO commerce_buyer_runtime;
GRANT SELECT(tenant_id,store_id,id,product_id,code,status,currency,price_minor,version) ON catalog.skus TO commerce_buyer_runtime;
GRANT SELECT(tenant_id,id,currency) ON control.stores TO commerce_buyer_runtime;
CREATE POLICY receipt_read ON buyer.command_results FOR SELECT TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid AND session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
CREATE POLICY receipt_insert ON buyer.command_results FOR INSERT TO commerce_buyer_runtime WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid AND session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
GRANT SELECT,INSERT ON buyer.command_results TO commerce_buyer_runtime;
CREATE POLICY buyer_read ON storefront.carts FOR SELECT TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid);
CREATE POLICY buyer_insert ON storefront.carts FOR INSERT TO commerce_buyer_runtime WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid AND creator_session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
GRANT SELECT,INSERT ON storefront.carts TO commerce_buyer_runtime;
CREATE POLICY buyer_read ON storefront.cart_lines FOR SELECT TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid);
CREATE POLICY buyer_insert ON storefront.cart_lines FOR INSERT TO commerce_buyer_runtime WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid);
GRANT SELECT,INSERT ON storefront.cart_lines TO commerce_buyer_runtime;
CREATE POLICY buyer_read ON storefront.quotes FOR SELECT TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid);
CREATE POLICY buyer_insert ON storefront.quotes FOR INSERT TO commerce_buyer_runtime WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid AND creator_session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
GRANT SELECT,INSERT ON storefront.quotes TO commerce_buyer_runtime;
CREATE POLICY buyer_read ON storefront.events FOR SELECT TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid);
CREATE POLICY buyer_insert ON storefront.events FOR INSERT TO commerce_buyer_runtime WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid AND session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
GRANT SELECT,INSERT ON storefront.events TO commerce_buyer_runtime;
CREATE POLICY buyer_update ON storefront.carts FOR UPDATE TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid) WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid);
CREATE POLICY buyer_delete ON storefront.cart_lines FOR DELETE TO commerce_buyer_runtime USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid);
GRANT UPDATE(version) ON storefront.carts TO commerce_buyer_runtime;
GRANT DELETE ON storefront.cart_lines TO commerce_buyer_runtime;

-- Extend only newly onboarded owners; never backfill existing memberships.
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
    -- Global lock order is principal then session. Recheck session after locking
    -- principal: the preliminary lookup does not authorize the transaction.
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
      SELECT v_tenant,v_store,v_principal,p FROM unnest(ARRAY['store:read','audit:read','audit:write','catalog:read','catalog:write','inventory:read','inventory:write','inventory:reserve','pricing:read','pricing:write']) p;
    INSERT INTO inventory.warehouses(tenant_id,store_id,name) VALUES(v_tenant,v_store,p_warehouse_name) RETURNING id INTO v_warehouse;
    INSERT INTO identity.initial_stores(principal_id,idempotency_key,request_hash,tenant_id,store_id,warehouse_id)
      VALUES(v_principal,p_key,p_hash,v_tenant,v_store,v_warehouse);
    INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_tenant,v_store,v_principal,'merchant.store_created');
    RETURN QUERY SELECT v_tenant,v_store,v_warehouse;
END $$;
