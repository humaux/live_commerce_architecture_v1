-- Purpose: 0220 immutable merchant authorization for one named quote adjustment, and private order-detail projection.
-- Depends on: buyer.owners, storefront.quotes, checkout.orders, identity access fences (0094/0129 patterns).
-- Used by: internal/merchanttools pipeline, internal/storefront quote/revalidation, internal/merchantorders detail.
-- Invariants: no buyer writer, no price override, FORCE RLS, one bound quote per authorization, forward-only additive schema.

GRANT USAGE ON SCHEMA storefront TO commerce_runtime;
CREATE TABLE storefront.manual_order_discounts (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, owner_id uuid NOT NULL,
 principal_id uuid NOT NULL, request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 name text, amount_minor bigint, quote_id uuid,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,owner_id),
 FOREIGN KEY(tenant_id,store_id,owner_id) REFERENCES buyer.owners(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id),
 FOREIGN KEY(tenant_id,store_id,owner_id,quote_id) REFERENCES storefront.quotes(tenant_id,store_id,owner_id,id),
 CHECK ((name IS NULL AND amount_minor IS NULL) OR
  (name IS NOT NULL AND amount_minor IS NOT NULL AND char_length(name) BETWEEN 1 AND 30
   AND name=btrim(name) AND name !~ '[[:cntrl:]]' AND amount_minor BETWEEN 1 AND 1000000000000))
);
ALTER TABLE storefront.manual_order_discounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE storefront.manual_order_discounts FORCE ROW LEVEL SECURITY;
REVOKE ALL ON storefront.manual_order_discounts FROM PUBLIC;
GRANT SELECT ON storefront.manual_order_discounts TO commerce_runtime;
GRANT SELECT ON storefront.manual_order_discounts TO commerce_buyer_runtime,commerce_checkout_runtime;
GRANT SELECT,UPDATE(quote_id) ON storefront.manual_order_discounts TO commerce_auth;
GRANT INSERT(tenant_id,store_id,owner_id,principal_id,request_hash,name,amount_minor)
 ON storefront.manual_order_discounts TO commerce_auth;
GRANT USAGE ON SCHEMA storefront TO commerce_auth;
GRANT SELECT(tenant_id,store_id,owner_id,id,currency,snapshot) ON storefront.quotes TO commerce_auth;
CREATE POLICY manual_binding_quote_read ON storefront.quotes FOR SELECT TO commerce_auth
 USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY merchant_authorization ON storefront.manual_order_discounts FOR SELECT TO commerce_runtime
 USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND nullif(current_setting('app.principal_id',true),'') IS NOT NULL);
CREATE POLICY buyer_authorization_read ON storefront.manual_order_discounts FOR SELECT TO commerce_buyer_runtime,commerce_checkout_runtime
 USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND current_setting('app.principal_id',true)='');
CREATE POLICY authorization_read ON storefront.manual_order_discounts FOR SELECT TO commerce_auth
 USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
  AND nullif(current_setting('app.buyer_id',true),'') IS NULL);
CREATE POLICY authorization_insert ON storefront.manual_order_discounts FOR INSERT TO commerce_auth
 WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
  AND nullif(current_setting('app.buyer_id',true),'') IS NULL AND quote_id IS NULL);
CREATE POLICY authorization_binding ON storefront.manual_order_discounts FOR UPDATE TO commerce_auth
 USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
  AND nullif(current_setting('app.buyer_id',true),'') IS NULL)
 WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
  AND nullif(current_setting('app.buyer_id',true),'') IS NULL);
COMMENT ON TABLE storefront.manual_order_discounts IS
 'storefront-v2 G3 amendment; immutable merchant-only request authorization, including null-discount tombstone; quote_id bound once. No buyer PII or plaintext capability. Retain with financial order/quote evidence; no automatic deletion or historical money rewrite.';

CREATE FUNCTION fulfillment.authorize_manual_order_discount(p_hash bytea,p_store uuid,p_owner uuid,
 p_request_hash bytea,p_name text,p_amount bigint) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_auth_error text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_owner IS NULL
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid discount authorization' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'inventory:reserve');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text
  OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['inventory:reserve','catalog:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'discount access denied' USING ERRCODE=v_auth_error; END IF;
 -- The caller cannot choose a principal, tenant, or initial quote binding.
 INSERT INTO storefront.manual_order_discounts(tenant_id,store_id,owner_id,principal_id,request_hash,name,amount_minor)
  VALUES(s.tenant_id,p_store,p_owner,s.principal_id,p_request_hash,p_name,p_amount)
  ON CONFLICT(tenant_id,store_id,owner_id) DO NOTHING;
 IF NOT EXISTS(SELECT 1 FROM storefront.manual_order_discounts d
  WHERE d.tenant_id=s.tenant_id AND d.store_id=p_store AND d.owner_id=p_owner AND d.principal_id=s.principal_id) THEN
  RAISE EXCEPTION 'discount authorization conflict' USING ERRCODE='PT409'; END IF;
 -- ON CONFLICT may have waited; a revoked scope must roll this transaction back.
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['inventory:reserve','catalog:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'discount access denied' USING ERRCODE=v_auth_error; END IF;
END $$;
ALTER FUNCTION fulfillment.authorize_manual_order_discount(bytea,uuid,uuid,bytea,text,bigint) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION fulfillment.authorize_manual_order_discount(bytea,uuid,uuid,bytea,text,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.authorize_manual_order_discount(bytea,uuid,uuid,bytea,text,bigint) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.authorize_manual_order_discount(bytea,uuid,uuid,bytea,text,bigint) IS
 'internal/merchanttools only; rechecks merchant inventory:reserve+catalog:read; inserts an immutable unbound authorization or retains its existing request hash for the caller replay check; no buyer, quote, money or provider writes.';

CREATE FUNCTION fulfillment.bind_manual_order_discount(p_hash bytea,p_store uuid,p_quote uuid) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_owner uuid; v_currency text; v_adjustments jsonb; v_auth_error text; n integer;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_quote IS NULL
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid discount binding' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'inventory:reserve');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text
  OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['inventory:reserve','catalog:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'discount access denied' USING ERRCODE=v_auth_error; END IF;
 SELECT q.owner_id,q.currency,coalesce(q.snapshot->'adjustments','[]'::jsonb)
  INTO v_owner,v_currency,v_adjustments FROM storefront.quotes q
  WHERE q.tenant_id=s.tenant_id AND q.store_id=p_store AND q.id=p_quote;
 IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 UPDATE storefront.manual_order_discounts d SET quote_id=p_quote
  WHERE d.tenant_id=s.tenant_id AND d.store_id=p_store AND d.owner_id=v_owner
   AND d.principal_id=s.principal_id AND (d.quote_id IS NULL OR d.quote_id=p_quote)
   -- Full JSON equality rejects extra lines/keys, wrong kinds/types/currencies,
   -- JSON null and malformed arrays; a tombstone allows only absent/empty adjustments.
   AND v_adjustments=CASE WHEN d.name IS NULL THEN '[]'::jsonb ELSE
    jsonb_build_array(jsonb_build_object('kind','manual_discount','name',d.name,
     'amount_minor',d.amount_minor,'currency',v_currency)) END
   AND (d.name IS NULL OR v_adjustments->0->>'amount_minor'=d.amount_minor::text);
 GET DIAGNOSTICS n=ROW_COUNT;
 IF n<>1 THEN RAISE EXCEPTION 'discount quote conflict' USING ERRCODE='PT409'; END IF;
 -- UPDATE can wait for another binder; fence before the transaction may commit.
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['inventory:reserve','catalog:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'discount access denied' USING ERRCODE=v_auth_error; END IF;
END $$;
ALTER FUNCTION fulfillment.bind_manual_order_discount(bytea,uuid,uuid) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION fulfillment.bind_manual_order_discount(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.bind_manual_order_discount(bytea,uuid,uuid) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.bind_manual_order_discount(bytea,uuid,uuid) IS
 'internal/merchanttools only; merchant-scoped inventory:reserve+catalog:read; bind a synthetic owner authorization to its immutable quote once; no money, inventory or provider writes.';

CREATE FUNCTION identity.read_order_adjustments(p_hash bytea,p_store uuid,p_order uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_result jsonb; v_auth_error text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid adjustment read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 SELECT coalesce(o.snapshot#>'{quote,adjustments}','[]'::jsonb) INTO v_result FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=p_order;
 -- The read can wait on checkout.orders; fence before reporting existence so a revoked caller gets the same denial either way.
 IF NOT FOUND THEN v_result:=NULL; END IF;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'adjustment read denied' USING ERRCODE=v_auth_error; END IF;
 IF v_result IS NULL THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION identity.read_order_adjustments(bytea,uuid,uuid) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_order_adjustments(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_order_adjustments(bytea,uuid,uuid) TO commerce_runtime;
COMMENT ON FUNCTION identity.read_order_adjustments(bytea,uuid,uuid) IS
 'internal/merchantorders.Get; orders:read re-verified, store-scoped frozen quote adjustments, no repricing or customer data.';
