-- 0103 live-claim checkout fixes (independent live-tools gate output/live-tools/tests/DEFECTS.md, D1 P0 + D2 P2).
--   * D1 (P0, 0092): checkout.Begin runs storefront.RevalidateQuote on the CHECKOUT pool (commerce_checkout_runtime), which calls
--     claims.live_prices for every cart that holds a claim-origin line. 0092 granted it to commerce_buyer_runtime only and the checkout
--     role had no USAGE on schema claims, so Begin of ANY cart redeemed from a claim link failed with 42501 ("checkout database
--     unavailable"): the core comment -> claim -> cart -> checkout loop could not place an order. Fix: the checkout role gets schema USAGE
--     (name resolution only; it holds no table privilege in claims) and EXECUTE on the live-price definer, nothing else.
--   * D2 (P2, 0092): claims.live_prices proved the origin (bundle bound to the buyer, unexpired link, claim line, active priced offer) but
--     not the claimed QUANTITY, trusting the cart line's own claim_quantity; a buyer role able to write raw SQL could live-price 99 units
--     of a 2-unit claim. The definer now takes the cart quantity of each origin and returns a row only when it is within
--     claims.lines.quantity (one claim line per (bundle, offer): PRIMARY KEY in 0060), so a line above its claim pays the catalog price
--     for the whole line, exactly what applyLivePrices already did for honest carts.
-- The 3-argument function is dropped (its only caller, internal/storefront/claim_price.go, moves to the 4-argument one in the same
-- release; every app process is stopped while migrations run, deploy.sh upgrade step 3). Owner, search_path, volatility (0101) and the
-- buyer fence are unchanged.
-- Non-goals: preview_live_prices (display only, the Quote decides the charge) is untouched; no table, role or Go pool change.

DROP FUNCTION claims.live_prices(uuid[],uuid[],uuid[]);

CREATE FUNCTION claims.live_prices(p_bundles uuid[], p_offers uuid[], p_skus uuid[], p_quantities bigint[])
RETURNS TABLE(sku_id uuid, bundle_id uuid, offer_id uuid, live_price_minor bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_buyer uuid;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
  OR coalesce(current_setting('app.tenant_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.store_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_session_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR current_setting('app.principal_id',true) IS DISTINCT FROM ''
  OR p_bundles IS NULL OR p_offers IS NULL OR p_skus IS NULL OR p_quantities IS NULL
  OR array_ndims(p_bundles)<>1 OR array_ndims(p_offers)<>1 OR array_ndims(p_skus)<>1 OR array_ndims(p_quantities)<>1
  OR cardinality(p_bundles) NOT BETWEEN 1 AND 50
  OR cardinality(p_bundles)<>cardinality(p_offers) OR cardinality(p_bundles)<>cardinality(p_skus)
  OR cardinality(p_bundles)<>cardinality(p_quantities)
  OR EXISTS (SELECT 1 FROM unnest(p_bundles) x WHERE x IS NULL)
  OR EXISTS (SELECT 1 FROM unnest(p_offers) x WHERE x IS NULL)
  OR EXISTS (SELECT 1 FROM unnest(p_skus) x WHERE x IS NULL)
  OR EXISTS (SELECT 1 FROM unnest(p_quantities) x WHERE x IS NULL OR x<1) THEN
  RAISE EXCEPTION 'invalid live price request' USING ERRCODE='22023';
 END IF;
 v_tenant:=current_setting('app.tenant_id')::uuid;
 v_store:=current_setting('app.store_id')::uuid;
 v_buyer:=current_setting('app.buyer_id')::uuid;
 -- Every join below is one clause of the money rule (see 0092's header): bound to THIS buyer, link unexpired, the claim line exists for
 -- that SKU and covers the cart quantity (D2), the offer is active, belongs to the bundle's session and has a price.
 RETURN QUERY
 SELECT u.sku_id, u.bundle_id, u.offer_id, o.live_price_minor
 FROM unnest(p_bundles,p_offers,p_skus,p_quantities) AS u(bundle_id,offer_id,sku_id,quantity)
 JOIN claims.bundles b ON b.tenant_id=v_tenant AND b.store_id=v_store AND b.id=u.bundle_id AND b.owner_id=v_buyer
 JOIN claims.links k ON k.tenant_id=b.tenant_id AND k.store_id=b.store_id AND k.bundle_id=b.id
  AND k.expires_at>clock_timestamp()
 JOIN claims.lines l ON l.tenant_id=b.tenant_id AND l.store_id=b.store_id AND l.bundle_id=b.id
  AND l.offer_id=u.offer_id AND l.sku_id=u.sku_id AND u.quantity<=l.quantity
 JOIN live.offers o ON o.tenant_id=l.tenant_id AND o.store_id=l.store_id AND o.id=l.offer_id
  AND o.session_id=b.session_id AND o.sku_id=u.sku_id AND o.active AND o.live_price_minor IS NOT NULL;
END $$;
ALTER FUNCTION claims.live_prices(uuid[],uuid[],uuid[],bigint[]) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.live_prices(uuid[],uuid[],uuid[],bigint[]) FROM PUBLIC;
-- D1: Quote (buyer pool) and Begin's RevalidateQuote (checkout pool) both price claim origins through this one definer.
GRANT USAGE ON SCHEMA claims TO commerce_checkout_runtime;
GRANT EXECUTE ON FUNCTION claims.live_prices(uuid[],uuid[],uuid[],bigint[]) TO commerce_buyer_runtime, commerce_checkout_runtime;
COMMENT ON FUNCTION claims.live_prices(uuid[],uuid[],uuid[],bigint[]) IS
 'internal/claims (SQL definer; its only caller is internal/storefront applyLivePrices, from Quote on commerce_buyer_runtime and from checkout.Begin''s RevalidateQuote on commerce_checkout_runtime); EXECUTE: those two roles. Returns the live price of each (bundle,offer,sku,cart quantity) origin that is bound to the caller, has an unexpired link, a claim line for that SKU whose quantity covers the cart quantity, and an active priced offer of the bundle''s session. Zero rows = catalog price.';
