-- 0092 live tools (contracts/live-keyword-claims-v1.md, amendment "Live tools (R4)", unit live-tools).
-- Owning package: internal/claims (offers, library, price resolution) with internal/storefront (cart origin columns, Quote).
-- Adds: (1) live.keyword_library, a store-level default keyword per SKU; (2) live.offers.live_price_minor, an optional
-- live-only unit price; (3) storefront.cart_lines.claim_* columns recording which claim line a cart line came from;
-- (4) claims.live_prices, the only function that turns a claim origin into a price; (5) claims.preview_live_prices,
-- the display twin for the claim link.
--
-- Money rule (why the price is resolved from the origin and not stored on the cart): a live price may apply ONLY to a cart
-- line that came from a claim bundle bound to this buyer, while that bundle's link is unexpired, for an active offer of the
-- bundle's own session that targets the same SKU. claims.live_prices re-proves every one of those facts at call time, so a
-- forged or stale cart origin can never produce a price a legitimate claim would not. The Quote (storefront.CreateQuote /
-- RevalidateQuote) stays the single price authority; nothing here lets a client send a price.
-- Non-goals: no price on the SKU, no per-buyer price, no discount engine, no stock effect, no change to claims ingest.

-- (1) Keyword library. A template only: it never claims anything by itself.
CREATE TABLE live.keyword_library (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, sku_id uuid NOT NULL,
 keyword text NOT NULL CHECK (keyword ~ '^[A-Z0-9]{1,16}$' AND octet_length(keyword)=char_length(keyword)),
 version bigint NOT NULL DEFAULT 1 CHECK (version>0),
 principal_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (tenant_id,store_id,sku_id),
 UNIQUE (tenant_id,store_id,keyword),
 FOREIGN KEY (tenant_id,store_id,sku_id) REFERENCES catalog.skus(tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id));
ALTER TABLE live.keyword_library ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.keyword_library FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.keyword_library FROM PUBLIC;
CREATE POLICY library_read ON live.keyword_library FOR SELECT TO commerce_runtime USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY library_insert ON live.keyword_library FOR INSERT TO commerce_runtime WITH CHECK
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY library_update ON live.keyword_library FOR UPDATE TO commerce_runtime USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY library_delete ON live.keyword_library FOR DELETE TO commerce_runtime USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT, INSERT, DELETE ON live.keyword_library TO commerce_runtime;
GRANT UPDATE(keyword,version,principal_id,updated_at) ON live.keyword_library TO commerce_runtime;

-- (2) Live-only price on the offer. >= 1 (a free live item is a catalog decision, not a claim price); upper bound equals
-- the checkout quote-line bound (0013: unit_price_minor <= 1e12).
ALTER TABLE live.offers ADD COLUMN live_price_minor bigint
 CHECK (live_price_minor IS NULL OR live_price_minor BETWEEN 1 AND 1000000000000);
GRANT UPDATE(live_price_minor) ON live.offers TO commerce_runtime;
-- claims_writer definers read the price and the offer's SKU (policy offer_writer_read already scopes the rows).
GRANT SELECT(sku_id,live_price_minor) ON live.offers TO commerce_claims_writer;

-- (3) Claim origin of a cart line. No FK to claims.lines: the retention purge (0071) deletes claim rows and an FK would
-- block it; claims.live_prices proves the origin on every read instead.
ALTER TABLE storefront.cart_lines
 ADD COLUMN claim_bundle_id uuid, ADD COLUMN claim_offer_id uuid, ADD COLUMN claim_quantity bigint,
 ADD CONSTRAINT cart_lines_claim_origin_shape CHECK (
  (claim_bundle_id IS NULL)=(claim_offer_id IS NULL) AND (claim_offer_id IS NULL)=(claim_quantity IS NULL)
  AND (claim_quantity IS NULL OR claim_quantity BETWEEN 1 AND 999));

-- (4) claims.live_prices: which of the given cart origins currently earn their offer's live price.
-- Buyer guard identical to claims.preview_link (READ COMMITTED, canonical GUCs, principal ''); arrays equal length 1..50,
-- one-dimensional, NULL-free. Zero rows = nobody earns a live price (the caller prices at catalog). STABLE, no lock, no write.
CREATE FUNCTION claims.live_prices(p_bundles uuid[], p_offers uuid[], p_skus uuid[])
RETURNS TABLE(sku_id uuid, bundle_id uuid, offer_id uuid, live_price_minor bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_buyer uuid;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
  OR coalesce(current_setting('app.tenant_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.store_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_session_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR current_setting('app.principal_id',true) IS DISTINCT FROM ''
  OR p_bundles IS NULL OR p_offers IS NULL OR p_skus IS NULL
  OR array_ndims(p_bundles)<>1 OR array_ndims(p_offers)<>1 OR array_ndims(p_skus)<>1
  OR cardinality(p_bundles) NOT BETWEEN 1 AND 50
  OR cardinality(p_bundles)<>cardinality(p_offers) OR cardinality(p_bundles)<>cardinality(p_skus)
  OR EXISTS (SELECT 1 FROM unnest(p_bundles) x WHERE x IS NULL)
  OR EXISTS (SELECT 1 FROM unnest(p_offers) x WHERE x IS NULL)
  OR EXISTS (SELECT 1 FROM unnest(p_skus) x WHERE x IS NULL) THEN
  RAISE EXCEPTION 'invalid live price request' USING ERRCODE='22023';
 END IF;
 v_tenant:=current_setting('app.tenant_id')::uuid;
 v_store:=current_setting('app.store_id')::uuid;
 v_buyer:=current_setting('app.buyer_id')::uuid;
 -- Every join below is one clause of the money rule (see the file header): bound to THIS buyer, link unexpired,
 -- the claim line exists for that SKU, the offer is active, belongs to the bundle's session and has a price.
 RETURN QUERY
 SELECT u.sku_id, u.bundle_id, u.offer_id, o.live_price_minor
 FROM unnest(p_bundles,p_offers,p_skus) AS u(bundle_id,offer_id,sku_id)
 JOIN claims.bundles b ON b.tenant_id=v_tenant AND b.store_id=v_store AND b.id=u.bundle_id AND b.owner_id=v_buyer
 JOIN claims.links k ON k.tenant_id=b.tenant_id AND k.store_id=b.store_id AND k.bundle_id=b.id
  AND k.expires_at>clock_timestamp()
 JOIN claims.lines l ON l.tenant_id=b.tenant_id AND l.store_id=b.store_id AND l.bundle_id=b.id
  AND l.offer_id=u.offer_id AND l.sku_id=u.sku_id
 JOIN live.offers o ON o.tenant_id=l.tenant_id AND o.store_id=l.store_id AND o.id=l.offer_id
  AND o.session_id=b.session_id AND o.sku_id=u.sku_id AND o.active AND o.live_price_minor IS NOT NULL;
END $$;
ALTER FUNCTION claims.live_prices(uuid[],uuid[],uuid[]) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.live_prices(uuid[],uuid[],uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.live_prices(uuid[],uuid[],uuid[]) TO commerce_buyer_runtime;

-- (5) claims.preview_live_prices: the claim-link display twin. Same link guards as preview_link (unexpired, unbound or
-- bound to the caller). Active offers with a live price only, keyed by keyword (unique per session, the key of the
-- preview lines); the Quote still decides what is charged.
CREATE FUNCTION claims.preview_live_prices(p_hash bytea)
RETURNS TABLE(keyword text, live_price_minor bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_buyer uuid;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
  OR coalesce(current_setting('app.tenant_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.store_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_session_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR current_setting('app.principal_id',true) IS DISTINCT FROM ''
  OR p_hash IS NULL OR octet_length(p_hash)<>32 THEN
  RAISE EXCEPTION 'invalid claim link request' USING ERRCODE='22023';
 END IF;
 v_tenant:=current_setting('app.tenant_id')::uuid;
 v_store:=current_setting('app.store_id')::uuid;
 v_buyer:=current_setting('app.buyer_id')::uuid;
 RETURN QUERY
 SELECT o.keyword, o.live_price_minor
 FROM claims.links k
 JOIN claims.bundles b ON b.tenant_id=k.tenant_id AND b.store_id=k.store_id AND b.id=k.bundle_id
 JOIN claims.lines l ON l.tenant_id=b.tenant_id AND l.store_id=b.store_id AND l.bundle_id=b.id
 JOIN live.offers o ON o.tenant_id=l.tenant_id AND o.store_id=l.store_id AND o.id=l.offer_id
  AND o.session_id=b.session_id AND o.sku_id=l.sku_id
 WHERE k.token_hash=p_hash AND k.tenant_id=v_tenant AND k.store_id=v_store
  AND b.tenant_id=v_tenant AND b.store_id=v_store
  AND k.expires_at>clock_timestamp()
  AND (b.owner_id IS NULL OR b.owner_id=v_buyer)
  AND o.active AND o.live_price_minor IS NOT NULL;
END $$;
ALTER FUNCTION claims.preview_live_prices(bytea) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.preview_live_prices(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.preview_live_prices(bytea) TO commerce_buyer_runtime;

COMMENT ON TABLE live.keyword_library IS
 'internal/claims (keyword_library.go): store-level default keyword per SKU, a template for importing offers into a session. RW: commerce_runtime under RLS. Non-goal: claims nothing by itself; never read by ingest.';
COMMENT ON COLUMN live.keyword_library.keyword IS 'kw-v1 canonical keyword (same CHECK as live.offers.keyword); unique per store.';
COMMENT ON COLUMN live.offers.live_price_minor IS
 'internal/claims: optional live-only unit price in SKU currency minor units (1..1e12). Applied by claims.live_prices ONLY to cart lines that originate from a bound, unexpired claim bundle of this offer''s session; NULL = none. Written by commerce_runtime only. Never copied between sessions.';
COMMENT ON COLUMN storefront.cart_lines.claim_bundle_id IS
 'internal/storefront: claim bundle this line was applied from (set only by claims.RedeemLink through SetCart; never client input). Evidence of origin only: claims.live_prices re-proves binding, link expiry and offer on every Quote.';
COMMENT ON COLUMN storefront.cart_lines.claim_offer_id IS 'internal/storefront: claim offer of the origin (with claim_bundle_id); no FK so the claims retention purge is not blocked.';
COMMENT ON COLUMN storefront.cart_lines.claim_quantity IS
 'internal/storefront: quantity the claim granted; a live price survives only while the line quantity is <= this value.';
COMMENT ON FUNCTION claims.live_prices(uuid[],uuid[],uuid[]) IS
 'internal/claims (SQL definer; its only caller is internal/storefront Quote/Revalidate, as commerce_buyer_runtime); EXECUTE: commerce_buyer_runtime. Returns the live price of each (bundle,offer,sku) origin that is bound to the caller, has an unexpired link, an existing claim line and an active priced offer of the bundle''s session for that SKU. Zero rows = catalog price.';
COMMENT ON FUNCTION claims.preview_live_prices(bytea) IS
 'internal/claims.PreviewLink only; EXECUTE: commerce_buyer_runtime. Read-only live prices of the link''s active offers (display; the Quote decides the charge).';
