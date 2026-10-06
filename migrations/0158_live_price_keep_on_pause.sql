-- 0158_live_price_keep_on_pause.sql — owner decision 2026-10-07: 「已经认领的买家保留直播价，只拒绝新的认领。」 (money path)
-- Purpose: pausing (deactivating) a live offer used to end the live price of buyers who had ALREADY claimed it (their next quote fell back to the catalog
--   price, a link not yet opened skipped the line as offer_inactive). The owner ruled that only NEW claims are refused: a claim line (bundle, offer, SKU) was
--   granted while the offer was active (ingest rejects a paused offer as OFFER_INACTIVE), so it keeps the price it was granted, for at most the claimed units,
--   until its link expires under the existing rules. This migration drops the `live.offers.active` clause from the three functions that priced a claim line:
--   claims.live_prices (quote and checkout.Begin's RevalidateQuote: the authority), claims.preview_live_prices (the link display) and claims.for_buyer_lines
--   (the merchant order-for-buyer prefill, so the merchant-attested origin prices the same lines the buyer's own checkout would; a line superseded on its SKU by another
--   line of the bundle reports no live price, the same ranking as internal/claims skuWinners: active offer, then already applied, then lowest offer id).
-- Not changed on purpose: claims.consume_live_prices never read `active` (it only re-proves the claim line, the bound owner and the remaining quantity);
--   claims.preview_link / claims.redeem_link keep returning `offer_active` (a fact about the offer; Go no longer uses it as a gate, internal/claims/buyer.go);
--   ingest, the sold-out reply and offer recommend/send checks (all about NEW claims or promotion) keep refusing a paused offer.
-- Every other money condition of claims.live_prices is byte-for-byte the 0129 body: bound to THIS buyer (or the 0129 merchant-origin grant) with an unexpired
--   link, the claim line exists for that SKU, its quantity minus the units held by non-CANCELLED orders covers the cart quantity (the cap per claim line), the
--   offer belongs to the bundle's session and targets that SKU and has a price. Expiry is not extended; stock is still decided at checkout.
-- Depends on: 0092 (preview_live_prices body, VOLATILE since 0101), 0105 (claims.live_price_uses), 0129 (live_prices and for_buyer_lines bodies copied from
--   there), roles commerce_claims_writer (owner), commerce_buyer_runtime / commerce_checkout_runtime / commerce_runtime (EXECUTE unchanged).
-- Used by: internal/storefront claim_price.go (live_prices, from Quote and RevalidateQuote), internal/claims buyer.go (preview_live_prices, B1),
--   internal/claims live_price_grant.go and internal/merchanttools order_for_buyer.go (for_buyer_lines).
-- Invariants: contracts/live-keyword-claims-v1.md amendment "Live price kept on pause" (supersedes rule 7 of "Live tools (R4)" for paused offers and gate LTG03's
--   catalog-after-pause step). Signatures, owners, EXECUTE lists, volatility, search_path and SECURITY DEFINER are unchanged (CREATE OR REPLACE): no table,
--   column, grant or role change, so the KC03 ACL pins stand. Tests: TestLivePriceKeepOnPause*, TestLiveToolsGateOfferLifecycleAndExpiry,
--   TestKeywordToolsBatchDeactivateIsAPause, TestLiveToolsExpiryOffersAndForgedOrigin, TestRedeemPlanning*.

DO $$
BEGIN
 IF to_regprocedure('claims.live_prices(uuid[],uuid[],uuid[],bigint[])') IS NULL
  OR to_regprocedure('claims.preview_live_prices(bytea)') IS NULL
  OR to_regprocedure('claims.for_buyer_lines(uuid,uuid)') IS NULL THEN
  RAISE EXCEPTION '0158 needs the 0129 claims.live_prices / claims.for_buyer_lines and the 0092 claims.preview_live_prices'; END IF;
END $$;

-- ---------------------------------------------------------------------------------------
-- claims.live_prices (same signature/owner/ACL/volatility as 0129): the offer need not be active
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION claims.live_prices(p_bundles uuid[], p_offers uuid[], p_skus uuid[], p_quantities bigint[])
RETURNS TABLE(sku_id uuid, bundle_id uuid, offer_id uuid, live_price_minor bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_buyer uuid; v_quote text;
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
 -- 0129: RevalidateQuote names the quote it re-prices (storefront); CreateQuote leaves it unset, so a grant already bound to a quote prices nothing new.
 v_quote:=nullif(current_setting('app.quote_id',true),'');
 -- Every join below is one clause of the money rule (see 0092's header): bound to THIS buyer, link unexpired, the claim line exists for
 -- that SKU and its REMAINING quantity (claimed minus held uses, R4S-01) covers the cart quantity (D2), the offer belongs to the bundle's
 -- session and has a price. 0158 (owner decision 2026-10-07): the offer need NOT be active. A claim line was granted while the offer was
 -- active (ingest refuses a paused offer), so pausing it ends only NEW claims; the buyers who already claimed keep the price until their link
 -- expires, for at most the claimed units (the cap above is unchanged).
 RETURN QUERY
 SELECT u.sku_id, u.bundle_id, u.offer_id, o.live_price_minor
 FROM unnest(p_bundles,p_offers,p_skus,p_quantities) AS u(bundle_id,offer_id,sku_id,quantity)
 JOIN claims.bundles b ON b.tenant_id=v_tenant AND b.store_id=v_store AND b.id=u.bundle_id
  AND ((b.owner_id=v_buyer AND EXISTS (SELECT 1 FROM claims.links k WHERE k.tenant_id=b.tenant_id AND k.store_id=b.store_id
        AND k.bundle_id=b.id AND k.expires_at>clock_timestamp()))
   -- 0129 merchant-attested origin (live-console-v1 §5.3): the cart's buyer holds a single-use grant of THIS request for this bundle (one
   -- quote only: unbound at CreateQuote, bound to one quote id afterwards), the bundle is not purged and its session not archived.
   -- The merchant never chooses whose price applies: the grant row is written by claims.for_buyer_begin after it compared peer keys.
   OR (b.purged_at IS NULL
       AND EXISTS (SELECT 1 FROM claims.merchant_origin_grants g WHERE g.tenant_id=b.tenant_id AND g.store_id=b.store_id AND g.buyer_id=v_buyer
            AND g.bundle_id=b.id AND g.consumed_at IS NULL AND g.expires_at>clock_timestamp() AND (g.quote_id IS NULL OR g.quote_id::text=v_quote))
       AND EXISTS (SELECT 1 FROM live.sessions z WHERE z.tenant_id=b.tenant_id AND z.store_id=b.store_id AND z.id=b.session_id AND z.lifecycle<>'archived')))
 JOIN claims.lines l ON l.tenant_id=b.tenant_id AND l.store_id=b.store_id AND l.bundle_id=b.id
  AND l.offer_id=u.offer_id AND l.sku_id=u.sku_id
  AND u.quantity<=l.quantity-(SELECT coalesce(sum(x.quantity),0) FROM claims.live_price_uses x
   JOIN checkout.orders r ON r.tenant_id=x.tenant_id AND r.store_id=x.store_id AND r.id=x.order_id AND r.commercial_state<>'CANCELLED'
   WHERE x.tenant_id=l.tenant_id AND x.store_id=l.store_id AND x.bundle_id=l.bundle_id AND x.offer_id=l.offer_id)
 JOIN live.offers o ON o.tenant_id=l.tenant_id AND o.store_id=l.store_id AND o.id=l.offer_id
  AND o.session_id=b.session_id AND o.sku_id=u.sku_id AND o.live_price_minor IS NOT NULL;
END $$;
ALTER FUNCTION claims.live_prices(uuid[],uuid[],uuid[],bigint[]) OWNER TO commerce_claims_writer;
COMMENT ON FUNCTION claims.live_prices(uuid[],uuid[],uuid[],bigint[]) IS
 'internal/claims (SQL definer; its only caller is internal/storefront applyLivePrices, from Quote on commerce_buyer_runtime and from checkout.Begin''s RevalidateQuote on commerce_checkout_runtime); EXECUTE: those two roles. Returns the live price of each (bundle,offer,sku,cart quantity) origin that is EITHER bound to the caller with an unexpired link OR (0129) covered by an unconsumed merchant-origin grant of the caller bound to no quote (CreateQuote) or to the quote being revalidated (app.quote_id); in both cases a claim line whose quantity minus the uses held by non-CANCELLED orders covers the cart quantity and a priced offer of the bundle''s session for that SKU. 0158 (owner decision 2026-10-07): the offer may be PAUSED, because a claim line was granted while it was active and a pause refuses only new claims; the claimed quantity and the link expiry still bound the price. Zero rows = catalog price.';

-- ---------------------------------------------------------------------------------------
-- claims.preview_live_prices (0092 body, VOLATILE since 0101): the link display shows the price of paused offers too
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION claims.preview_live_prices(p_hash bytea)
RETURNS TABLE(keyword text, live_price_minor bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
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
  AND o.live_price_minor IS NOT NULL;
END $$;
ALTER FUNCTION claims.preview_live_prices(bytea) OWNER TO commerce_claims_writer;
COMMENT ON FUNCTION claims.preview_live_prices(bytea) IS
 'internal/claims.PreviewLink only; EXECUTE: commerce_buyer_runtime. Read-only live prices of the link''s priced offers, active or paused (0158, owner decision 2026-10-07: a paused offer keeps the price of the claims already granted); display only, the Quote decides the charge.';

-- ---------------------------------------------------------------------------------------
-- claims.for_buyer_lines (0129 body): the merchant prefill reports the live price of a paused offer's claim line too
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION claims.for_buyer_lines(p_conversation uuid, p_bundle uuid)
RETURNS TABLE(bundle_id uuid, offer_id uuid, sku_id uuid, keyword text, quantity integer, live_price_minor bigint, live_remaining bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; c record; v_bundles uuid[];
BEGIN
 v:=claims.for_buyer_scope();
 IF (p_conversation IS NULL)=(p_bundle IS NULL) THEN RAISE EXCEPTION 'invalid prefill request' USING ERRCODE='22023'; END IF;
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF p_conversation IS NOT NULL THEN
  SELECT x.peer_key,x.app_id,x.object,x.asset_id INTO c FROM social.conversations x WHERE x.id=p_conversation AND x.tenant_id=v[1] AND x.store_id=v[2];
  IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
  -- live-console-v1 §3.7: the bundles whose recorded peer is this thread's peer; none = an empty answer (P2-7 c), not a 404.
  v_bundles:=ARRAY(SELECT DISTINCT p.bundle_id FROM inbox.bundle_peers p WHERE p.tenant_id=v[1] AND p.store_id=v[2]
   AND p.peer_key=c.peer_key AND p.app_id=c.app_id AND p.object=c.object AND p.asset_id=c.asset_id ORDER BY 1);
 ELSE
  PERFORM 1 FROM claims.bundles b WHERE b.tenant_id=v[1] AND b.store_id=v[2] AND b.id=p_bundle AND b.purged_at IS NULL;
  IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
  v_bundles:=ARRAY[p_bundle];
 END IF;
 RETURN QUERY
 SELECT l.bundle_id,l.offer_id,l.sku_id,o.keyword,l.quantity,
  -- 0158 (owner decision 2026-10-07): a paused offer keeps its price for the claim lines already granted; NULL when the offer has none OR when the line is SUPERSEDED on its SKU by
  -- another line of the bundle. The cart carries one claim origin per SKU, so one line wins it: rank (1) the line of the ACTIVE offer, (2) the line already applied into the cart
  -- (applied_version = version), (3) the lowest offer id. This is internal/claims skuWinners (buyer redeem and link preview) written as SQL: a superseded line must not be priced here
  -- either, or merchanttools.pickOrigins (first qualifying line per SKU) would fall through to it when the winner does not cover the ordered quantity (Opus review P2-5).
  CASE WHEN NOT EXISTS (SELECT 1 FROM claims.lines x JOIN live.offers xo ON xo.tenant_id=x.tenant_id AND xo.store_id=x.store_id AND xo.id=x.offer_id
    WHERE x.tenant_id=l.tenant_id AND x.store_id=l.store_id AND x.bundle_id=l.bundle_id AND x.sku_id=l.sku_id AND x.offer_id<>l.offer_id
     AND (ROW(xo.active,x.applied_version IS NOT DISTINCT FROM x.version)>ROW(o.active,l.applied_version IS NOT DISTINCT FROM l.version)
      OR (ROW(xo.active,x.applied_version IS NOT DISTINCT FROM x.version)=ROW(o.active,l.applied_version IS NOT DISTINCT FROM l.version) AND x.offer_id<l.offer_id)))
   THEN o.live_price_minor END,
  greatest(0,l.quantity-(SELECT coalesce(sum(u.quantity),0) FROM claims.live_price_uses u
   JOIN checkout.orders r ON r.tenant_id=u.tenant_id AND r.store_id=u.store_id AND r.id=u.order_id AND r.commercial_state<>'CANCELLED'
   WHERE u.tenant_id=l.tenant_id AND u.store_id=l.store_id AND u.bundle_id=l.bundle_id AND u.offer_id=l.offer_id))::bigint
 FROM claims.lines l
 JOIN claims.bundles b ON b.tenant_id=l.tenant_id AND b.store_id=l.store_id AND b.id=l.bundle_id AND b.purged_at IS NULL
 JOIN live.offers o ON o.tenant_id=l.tenant_id AND o.store_id=l.store_id AND o.id=l.offer_id
 WHERE l.tenant_id=v[1] AND l.store_id=v[2] AND l.bundle_id=ANY(v_bundles)
 ORDER BY l.bundle_id,o.keyword,l.offer_id LIMIT 50;
END $$;
ALTER FUNCTION claims.for_buyer_lines(uuid,uuid) OWNER TO commerce_claims_writer;
COMMENT ON FUNCTION claims.for_buyer_lines(uuid,uuid) IS
 'internal/claims (0129 LC-B6, GET inbox/order-prefill; caller commerce_runtime inside the merchant transaction, inventory:reserve re-verified by identity.principal_holds; the route adds orders:read): the claim lines (capped at 50) of one bundle, or of every bundle whose recorded peer is the conversation''s peer (none = zero rows), with the offer keyword, the live price of the offer (0158: also when the offer is paused, because the claim line was granted while it was active; NULL when the offer has none) and live_remaining = claimed minus units held in claims.live_price_uses by non-CANCELLED orders, ordered by bundle, keyword, offer id. 0158: NULL live price also for a line superseded on its SKU by another line of the bundle (active offer, then already applied, then lowest offer id). PT404 for a conversation or bundle outside the store. Read-only.';
