-- 0093 storefront-integration (docs/delivery/units/storefront-integration.md, contracts/taiwan-cvs-logistics-v1.md section 5.2).
--
-- Owns: widening the ECPay map return-path allowlist by exactly one form. The new storefront checkout page lives at /{locale}/checkout,
-- and the map returns the buyer to the path the browser opened it from (internal/checkout/cvs.go Open -> fulfillment.cvs_selections.return_path
-- -> fulfillment.record_cvs_map_return 302 Location). The allowlist is therefore
--     ^/(zh-TW|zh-CN|en)/(products/[A-Za-z0-9_-]{1,64}|checkout)$
-- Same strictness as the product form: three locales, no query, no fragment, no extra segment, no host. Two places hold it and both change here:
-- (1) the CHECK on fulfillment.cvs_selections.return_path (0072), (2) the open_cvs_selection definer (0073). The Go pattern in
-- internal/checkout/cvs.go answers 422 early; SQL stays the authority (open-redirect guard: a stored path is later concatenated to the
-- origin, so it must never be anything but one of these two shapes).
-- Existing rows keep passing: the product form is still allowed, so ADD CONSTRAINT validates against every old row.
-- Also (section B) the collection id in the two catalog/v2 collection reads, and (section C) the PUBLIC EXECUTE that two wave-1 trigger functions kept.
-- Non-goals: no other function body changes; the owner, EXECUTE grants and the replay/idempotency behaviour of open_cvs_selection are
-- exactly 0073's (CREATE OR REPLACE keeps owner and ACL; they are restated below so a fresh read of this file shows them).

ALTER TABLE fulfillment.cvs_selections DROP CONSTRAINT cvs_selections_return_path_check;
ALTER TABLE fulfillment.cvs_selections ADD CONSTRAINT cvs_selections_return_path_check
 CHECK(return_path ~ '^/(zh-TW|zh-CN|en)/(products/[A-Za-z0-9_-]{1,64}|checkout)$');
COMMENT ON COLUMN fulfillment.cvs_selections.return_path IS 'Allowlisted storefront path the buyer returns to after the map: /{locale}/checkout or /{locale}/products/{id} (migration 0093; never read from the map-return body).';

CREATE OR REPLACE FUNCTION fulfillment.open_cvs_selection(p_buyer_hash bytea,p_store uuid,p_key text,p_request_hash bytea,
 p_cart_version bigint,p_market uuid,p_service_code text,p_nonce_sha256 bytea,p_return_origin text,p_return_path text,
 p_payment_environment text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_scope record; v_now timestamptz; v_cart record; v_head bigint; v_svc record; v_prof record; v_sub text;
 v_saved bytea; v_id uuid:=gen_random_uuid(); v_expires timestamptz; v_chains text[]; v_response jsonb;
BEGIN
 IF p_buyer_hash IS NULL OR octet_length(p_buyer_hash)<>32 OR p_store IS NULL OR p_key IS NULL
  OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$' OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32
  OR p_cart_version IS NULL OR p_cart_version<1 OR p_market IS NULL OR p_service_code IS NULL
  OR p_service_code !~ '^[a-z][a-z0-9_-]{0,39}$' OR p_nonce_sha256 IS NULL OR octet_length(p_nonce_sha256)<>32
  OR p_return_origin IS NULL OR p_return_path IS NULL OR p_payment_environment IS NULL
  OR p_payment_environment NOT IN ('SANDBOX','LIVE','')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid cvs selection' USING ERRCODE='PT400'; END IF;
 -- Buyer scope before anything else (buyer.WithScope pattern, before replay).
 SELECT * INTO v_scope FROM buyer.resolve_scope(p_buyer_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 PERFORM set_config('app.tenant_id',v_scope.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.buyer_id',v_scope.owner_id::text,true),set_config('app.buyer_session_id',v_scope.session_id::text,true),
  set_config('app.principal_id','',true);
 PERFORM pg_advisory_xact_lock(hashtextextended('cvs.selection.open|'||v_scope.tenant_id||'|'||p_store||'|'||v_scope.owner_id||'|'||p_key,0));
 SELECT r.request_hash INTO v_saved FROM buyer.command_results r WHERE r.tenant_id=v_scope.tenant_id AND r.store_id=p_store
  AND r.owner_id=v_scope.owner_id AND r.session_id=v_scope.session_id AND r.operation='fulfillment.cvs_selection.open'
  AND r.idempotency_key=p_key;
 IF FOUND THEN
  -- The nonce is not re-derivable, so a replay cannot return the form: the client opens a new selection (§5.2).
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
  RAISE EXCEPTION 'selection_replay_new_key' USING ERRCODE='PT409';
 END IF;
 IF p_return_path !~ '^/(zh-TW|zh-CN|en)/(products/[A-Za-z0-9_-]{1,64}|checkout)$' THEN
  RAISE EXCEPTION 'bad_return_path' USING ERRCODE='PT422'; END IF;
 IF NOT EXISTS(SELECT 1 FROM control.storefront_domains d JOIN control.storefront_publications pu
   ON pu.tenant_id=d.tenant_id AND pu.store_id=d.store_id AND pu.published
   WHERE d.tenant_id=v_scope.tenant_id AND d.store_id=p_store AND d.state='ACTIVE' AND d.origin=p_return_origin) THEN
  RAISE EXCEPTION 'bad_return_origin' USING ERRCODE='PT422'; END IF;
 -- Lock order: cart FOR SHARE -> service head -> profile.
 SELECT c.id,c.version INTO v_cart FROM storefront.carts c WHERE c.tenant_id=v_scope.tenant_id AND c.store_id=p_store
  AND c.owner_id=v_scope.owner_id FOR SHARE;
 IF NOT FOUND OR v_cart.version<>p_cart_version OR NOT EXISTS(SELECT 1 FROM storefront.cart_lines l
   WHERE l.tenant_id=v_scope.tenant_id AND l.store_id=p_store AND l.owner_id=v_scope.owner_id AND l.cart_id=v_cart.id) THEN
  RAISE EXCEPTION 'cart changed' USING ERRCODE='PT409'; END IF;
 SELECT h.current_version INTO v_head FROM fulfillment.service_heads h WHERE h.tenant_id=v_scope.tenant_id
  AND h.store_id=p_store AND h.market_id=p_market AND h.country='TW' AND h.code=p_service_code FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'service_unavailable' USING ERRCODE='PT422'; END IF;
 SELECT s.delivery_kind,s.enabled,s.visible INTO v_svc FROM fulfillment.service_versions s
  WHERE s.tenant_id=v_scope.tenant_id AND s.store_id=p_store AND s.market_id=p_market AND s.country='TW'
   AND s.code=p_service_code AND s.version=v_head;
 IF NOT FOUND OR NOT v_svc.enabled OR NOT v_svc.visible
  OR v_svc.delivery_kind NOT IN ('cvs_711','cvs_familymart','cvs_hilife','cvs_okmart') THEN
  RAISE EXCEPTION 'service_unavailable' USING ERRCODE='PT422'; END IF;
 -- The store's single enabled, qualified profile (unique index ecpay_one_enabled_profile).
 SELECT pr.connection_id,pr.mode,pr.ok_verified,pr.hilife_verified,pr.qualified_credential_version,
  a.environment,a.account_id,a.credential_version INTO v_prof
  FROM integration.ecpay_logistics_profiles pr
  JOIN integration.merchant_accounts a ON a.tenant_id=pr.tenant_id AND a.store_id=pr.store_id AND a.id=pr.connection_id
  WHERE pr.tenant_id=v_scope.tenant_id AND pr.store_id=p_store AND pr.enabled FOR SHARE OF pr;
 IF NOT FOUND OR v_prof.qualified_credential_version IS DISTINCT FROM v_prof.credential_version
  OR p_payment_environment='' OR v_prof.environment<>p_payment_environment
  OR (v_svc.delivery_kind='cvs_okmart' AND NOT v_prof.ok_verified)
  OR (v_svc.delivery_kind='cvs_hilife' AND NOT v_prof.hilife_verified) THEN
  RAISE EXCEPTION 'service_unavailable' USING ERRCODE='PT422'; END IF;
 SELECT c.enabled_chains INTO v_chains FROM fulfillment.cvs_store_settings c
  WHERE c.tenant_id=v_scope.tenant_id AND c.store_id=p_store;
 IF NOT FOUND THEN v_chains:=ARRAY['cvs_711','cvs_familymart','cvs_hilife','cvs_okmart']; END IF;
 IF NOT v_svc.delivery_kind=ANY(v_chains) THEN RAISE EXCEPTION 'service_unavailable' USING ERRCODE='PT422'; END IF;
 v_sub:=CASE WHEN v_prof.mode='C2C' THEN
   CASE v_svc.delivery_kind WHEN 'cvs_711' THEN 'UNIMARTC2C' WHEN 'cvs_familymart' THEN 'FAMIC2C'
    WHEN 'cvs_hilife' THEN 'HILIFEC2C' WHEN 'cvs_okmart' THEN 'OKMARTC2C' END
  ELSE CASE v_svc.delivery_kind WHEN 'cvs_711' THEN 'UNIMART' WHEN 'cvs_familymart' THEN 'FAMI'
    WHEN 'cvs_hilife' THEN 'HILIFE' END END;
 IF v_sub IS NULL THEN RAISE EXCEPTION 'service_unavailable' USING ERRCODE='PT422'; END IF;
 IF (SELECT count(*) FROM fulfillment.cvs_selections x WHERE x.tenant_id=v_scope.tenant_id AND x.store_id=p_store
   AND x.owner_id=v_scope.owner_id AND x.cart_id=v_cart.id AND x.state='OPEN' AND x.expires_at>clock_timestamp())>=10 THEN
  RAISE EXCEPTION 'too_many_open_selections' USING ERRCODE='PT429'; END IF;
 v_now:=clock_timestamp(); v_expires:=v_now+interval '15 minutes';
 INSERT INTO fulfillment.cvs_selections(tenant_id,store_id,owner_id,id,session_id,cart_id,cart_version,kind,connection_id,
  credential_version,logistics_subtype,nonce_sha256,state,return_origin,return_path,created_at,expires_at,updated_at,version)
 VALUES(v_scope.tenant_id,p_store,v_scope.owner_id,v_id,v_scope.session_id,v_cart.id,p_cart_version,v_svc.delivery_kind,
  v_prof.connection_id,v_prof.credential_version,v_sub,p_nonce_sha256,'OPEN',p_return_origin,p_return_path,v_now,v_expires,v_now,1);
 v_response:=jsonb_build_object('selection_id',v_id,'subtype',v_sub,'merchant_id',v_prof.account_id,
  'environment',v_prof.environment,'expires_at',to_char(v_expires AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
 INSERT INTO buyer.command_results(tenant_id,store_id,owner_id,session_id,operation,idempotency_key,request_hash,response)
  VALUES(v_scope.tenant_id,p_store,v_scope.owner_id,v_scope.session_id,'fulfillment.cvs_selection.open',p_key,p_request_hash,
   jsonb_build_object('selection_id',v_id));
 SELECT * INTO v_scope FROM buyer.resolve_scope(p_buyer_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'buyer capability expired' USING ERRCODE='PT401'; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION fulfillment.open_cvs_selection(bytea,uuid,text,bytea,bigint,uuid,text,bytea,text,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.open_cvs_selection(bytea,uuid,text,bytea,bigint,uuid,text,bytea,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.open_cvs_selection(bytea,uuid,text,bytea,bigint,uuid,text,bytea,text,text,text) TO commerce_checkout_runtime;
COMMENT ON FUNCTION fulfillment.open_cvs_selection(bytea,uuid,text,bytea,bigint,uuid,text,bytea,text,text,text) IS 'internal/checkout BuyerCVS.Open only; EXECUTE commerce_checkout_runtime (checkout pool, X9). Buyer scope first; return_path must be /{locale}/checkout or /{locale}/products/{id} (0093); return_origin must be an ACTIVE published domain of the store; the profile environment must equal p_payment_environment; <=10 OPEN per owner/cart (PT429). A replay is PT409 selection_replay_new_key (the nonce is not re-derivable).';

-- ---------------------------------------------------------------------------------------------------
-- B. Collection id in the buyer reads (contracts/storefront-v2.md section A amendment). The storefront builds the collection photo URL
-- /media/c/{collection_id}/{image_id}; the frozen reads carried only the slug and the image id, so the URL could not be built.
-- Both definers (0086, owner commerce_catalog_media, EXECUTE commerce_buyer_runtime) gain exactly one key, "id" = catalog.collections.id of
-- an ACTIVE collection of the verified origin store. The id is not a secret (the image route needs it); nothing else changes. CREATE OR REPLACE
-- keeps owner and EXECUTE; internal/buyerhttp catalogv2.go decodes the key (strictDecode rejects unknown keys, so Go and SQL move together).
-- ---------------------------------------------------------------------------------------------------
-- Active collections with the count of their active products (catalog/v2/collections).
CREATE OR REPLACE FUNCTION catalog.buyer_v2_collections(p_origin text)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid; v_tenant uuid; v_out jsonb;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=v_store AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 -- ponytail: at most 200 collections per store (enforced by internal/catalog), so no pagination here.
 SELECT jsonb_build_object('collections', coalesce(jsonb_agg(jsonb_build_object('id',c.id,'slug',c.slug,'title',c.title,
            'image_id',(SELECT ci.id FROM catalog.collection_images ci WHERE ci.tenant_id=c.tenant_id AND ci.store_id=c.store_id AND ci.collection_id=c.id),
            'product_count',(SELECT count(*) FROM catalog.collection_products cp
                               JOIN catalog.products p ON p.tenant_id=cp.tenant_id AND p.store_id=cp.store_id AND p.id=cp.product_id AND p.status='active'
                              WHERE cp.tenant_id=c.tenant_id AND cp.store_id=c.store_id AND cp.collection_id=c.id)) ORDER BY c.created_at, c.id),'[]'::jsonb))
   INTO v_out FROM catalog.collections c WHERE c.tenant_id=v_tenant AND c.store_id=v_store AND c.status='active';
 RETURN v_out;
END $$;

-- One active collection (catalog/v2/collections/{slug}); PT404 for unknown / hidden / foreign.
CREATE OR REPLACE FUNCTION catalog.buyer_v2_collection(p_origin text, p_slug text)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid; v_tenant uuid; v_out jsonb;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=v_store AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT jsonb_build_object('id',c.id,'slug',c.slug,'title',c.title,'description',c.description,
            'image_id',(SELECT ci.id FROM catalog.collection_images ci WHERE ci.tenant_id=c.tenant_id AND ci.store_id=c.store_id AND ci.collection_id=c.id))
   INTO v_out FROM catalog.collections c
  WHERE c.tenant_id=v_tenant AND c.store_id=v_store AND c.slug=p_slug AND c.status='active';
 IF v_out IS NULL THEN RAISE EXCEPTION 'collection not found' USING ERRCODE='PT404'; END IF;
 RETURN v_out;
END $$;

COMMENT ON FUNCTION catalog.buyer_v2_collections(text) IS
 'catalog-core owner commerce_catalog_media; only caller internal/buyerhttp catalogv2.go. Active collections of the verified origin store with their id (0093), active product count and image id.';
COMMENT ON FUNCTION catalog.buyer_v2_collection(text,text) IS
 'catalog-core owner commerce_catalog_media; only caller internal/buyerhttp catalogv2.go. One active collection by slug with its id (0093) (PT404 for unknown or hidden).';

-- ---------------------------------------------------------------------------------------------------
-- C. Two trigger functions of the R4 wave-1 migrations were left with the default PUBLIC EXECUTE: catalog.products_default_slug() (0086) and
-- design.refuse_history_change() (0087). The authority validator of every restricted pool (internal/platform validateStripeAuthority: any function
-- a login can EXECUTE, PUBLIC included, that is not on its fixed list is "unsafe stripe database privileges") then refuses the Stripe registrar login,
-- so every gate that opens it (--browser-refund-fulfilment, ...) failed at setup on the integration branch. EXECUTE on a trigger function is checked only
-- when the trigger is CREATED (by the migration owner), never when it fires, so revoking it from PUBLIC changes no behaviour; the neighbouring guard
-- triggers (0088 inventory.guard_bank_transfer_ledger, ...) already do the same. Idempotent: a later unit that repeats the REVOKE is harmless.
-- ---------------------------------------------------------------------------------------------------
REVOKE ALL ON FUNCTION catalog.products_default_slug() FROM PUBLIC;
REVOKE ALL ON FUNCTION design.refuse_history_change() FROM PUBLIC;
