-- 0105 live-claim price consumption (R4 security review R4S-01, P1; contracts/live-keyword-claims-v1.md "Live tools (R4)" rules 5 and 7).
-- Defect: claims.live_prices (0092, 0103) proved binding, link expiry, per-line claimed quantity and offer state, but nothing recorded that
-- a claim line's quantity had already been committed to an order. Storefront carts keep the claim origin after checkout (carryOrigins), so
-- the same buyer could re-quote and Begin again at the live price until the 72 h link expired: N orders x the claimed quantity at the live
-- price for one claim. The 0092 header promise "a forged or stale cart origin can never produce a price a legitimate claim would not" was
-- false for a stale (already ordered) origin; this migration makes it true (0092/0103 are checksummed and cannot be edited, so the
-- correction is recorded here and in the contract amendment).
-- Consumption rule: a live price applies to at most claims.lines.quantity units ACROSS ALL orders that still hold that claim line. A use
-- is held while its order's commercial_state is not CANCELLED (DRAFT, AWAITING_PAYMENT, AWAITING_TRANSFER, CONFIRMED: paid, fulfilled and
-- refunded orders stay CONFIRMED and keep it, conservative for the merchant). Every path that releases an order's stock (expire_held,
-- payment close-unpaid, CVS unclaimed/cancel, bank-transfer expiry) sets CANCELLED, so it gives the quantity back with no extra write.
-- Adds: (1) claims.live_price_uses, the ledger (one row per order x claim line), written ONLY by (2) claims.consume_live_prices, which
-- checkout.Begin calls on the checkout pool (commerce_checkout_runtime) after checkout.begin_hold in the SAME transaction (I04), so an order
-- at a live price never commits without its ledger rows and a refused consumption rolls back the order, hold, expiry job and receipt;
-- (3) claims.live_prices (same signature, owner, search_path, volatility and buyer fence) subtracts the held uses from the claimed quantity.
-- Privileges of the definer owner commerce_claims_writer (NOLOGIN; reachable only through its definers): SELECT/INSERT on the ledger,
-- column SELECT of order state on checkout.orders (no snapshot, no PII) and of the buyer's own quote snapshot on storefront.quotes (priced
-- lines, no destination or PII; begin_hold proves it is byte-equal to the order snapshot's quote).
-- Non-goals: preview_live_prices (display only) is untouched; no Go pool, role or River change; no merchant view of the ledger.

CREATE TABLE claims.live_price_uses (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, bundle_id uuid NOT NULL, offer_id uuid NOT NULL, order_id uuid NOT NULL,
 quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 999),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (tenant_id,store_id,bundle_id,offer_id,order_id),
 FOREIGN KEY (tenant_id,store_id,bundle_id,offer_id) REFERENCES claims.lines(tenant_id,store_id,bundle_id,offer_id),
 FOREIGN KEY (order_id) REFERENCES checkout.orders(id));
ALTER TABLE claims.live_price_uses ENABLE ROW LEVEL SECURITY;
ALTER TABLE claims.live_price_uses FORCE ROW LEVEL SECURITY;
REVOKE ALL ON claims.live_price_uses FROM PUBLIC;
CREATE POLICY use_writer_read ON claims.live_price_uses FOR SELECT TO commerce_claims_writer USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
-- Buyer context only (P2(a): the write policy names its context).
CREATE POLICY use_writer_insert ON claims.live_price_uses FOR INSERT TO commerce_claims_writer WITH CHECK
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND nullif(current_setting('app.principal_id',true),'')::uuid IS NULL
  AND nullif(current_setting('app.buyer_id',true),'')::uuid IS NOT NULL);
GRANT SELECT(tenant_id,store_id,bundle_id,offer_id,order_id,quantity) ON claims.live_price_uses TO commerce_claims_writer;
GRANT INSERT(tenant_id,store_id,bundle_id,offer_id,order_id,quantity) ON claims.live_price_uses TO commerce_claims_writer;
COMMENT ON TABLE claims.live_price_uses IS
 'internal/claims (0105, R4S-01): live-price consumption ledger, one row per (claim line, order) whose quote applied the live price. Inserted only by claims.consume_live_prices in the checkout.Begin transaction; read by claims.live_prices. A row holds its quantity while checkout.orders.commercial_state<>''CANCELLED''. FORCE RLS; no runtime role holds any privilege. Non-goals: no buyer or merchant read, no release write (release is derived from order state), never deleted.';

-- Order state and the buyer's own quote, read by the two definers below (column grants: no order snapshot, no destination, no PII).
GRANT USAGE ON SCHEMA checkout, storefront TO commerce_claims_writer;
GRANT SELECT(tenant_id,store_id,owner_id,id,creator_session_id,quote_id,commercial_state,created_at) ON checkout.orders TO commerce_claims_writer;
CREATE POLICY claims_writer_order_read ON checkout.orders FOR SELECT TO commerce_claims_writer USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT(tenant_id,store_id,owner_id,id,snapshot) ON storefront.quotes TO commerce_claims_writer;
CREATE POLICY claims_writer_quote_read ON storefront.quotes FOR SELECT TO commerce_claims_writer USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid);

-- (3) The live-price authority now also subtracts held uses. Only the claim-line join changes (marked R4S-01).
CREATE OR REPLACE FUNCTION claims.live_prices(p_bundles uuid[], p_offers uuid[], p_skus uuid[], p_quantities bigint[])
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
 -- that SKU and its REMAINING quantity (claimed minus held uses, R4S-01) covers the cart quantity (D2), the offer is active, belongs to the
 -- bundle's session and has a price.
 RETURN QUERY
 SELECT u.sku_id, u.bundle_id, u.offer_id, o.live_price_minor
 FROM unnest(p_bundles,p_offers,p_skus,p_quantities) AS u(bundle_id,offer_id,sku_id,quantity)
 JOIN claims.bundles b ON b.tenant_id=v_tenant AND b.store_id=v_store AND b.id=u.bundle_id AND b.owner_id=v_buyer
 JOIN claims.links k ON k.tenant_id=b.tenant_id AND k.store_id=b.store_id AND k.bundle_id=b.id
  AND k.expires_at>clock_timestamp()
 JOIN claims.lines l ON l.tenant_id=b.tenant_id AND l.store_id=b.store_id AND l.bundle_id=b.id
  AND l.offer_id=u.offer_id AND l.sku_id=u.sku_id
  AND u.quantity<=l.quantity-(SELECT coalesce(sum(x.quantity),0) FROM claims.live_price_uses x
   JOIN checkout.orders r ON r.tenant_id=x.tenant_id AND r.store_id=x.store_id AND r.id=x.order_id AND r.commercial_state<>'CANCELLED'
   WHERE x.tenant_id=l.tenant_id AND x.store_id=l.store_id AND x.bundle_id=l.bundle_id AND x.offer_id=l.offer_id)
 JOIN live.offers o ON o.tenant_id=l.tenant_id AND o.store_id=l.store_id AND o.id=l.offer_id
  AND o.session_id=b.session_id AND o.sku_id=u.sku_id AND o.active AND o.live_price_minor IS NOT NULL;
END $$;
COMMENT ON FUNCTION claims.live_prices(uuid[],uuid[],uuid[],bigint[]) IS
 'internal/claims (SQL definer; its only caller is internal/storefront applyLivePrices, from Quote on commerce_buyer_runtime and from checkout.Begin''s RevalidateQuote on commerce_checkout_runtime); EXECUTE: those two roles. Returns the live price of each (bundle,offer,sku,cart quantity) origin that is bound to the caller, has an unexpired link, a claim line for that SKU whose quantity minus the uses held by non-CANCELLED orders (claims.live_price_uses, 0105) covers the cart quantity, and an active priced offer of the bundle''s session. Zero rows = catalog price.';

-- (2) Consumption: the only writer of the ledger.
CREATE FUNCTION claims.consume_live_prices(p_order uuid) RETURNS integer
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_buyer uuid; v_quote uuid; r record; v_claimed integer; v_held bigint; v_count integer:=0;
BEGIN
 -- Same buyer fence as claims.live_prices (begin_hold has just set these GUCs from the resolved capability).
 IF current_setting('transaction_isolation')<>'read committed'
  OR coalesce(current_setting('app.tenant_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.store_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_session_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR current_setting('app.principal_id',true) IS DISTINCT FROM ''
  OR p_order IS NULL THEN
  RAISE EXCEPTION 'invalid live price consumption' USING ERRCODE='22023';
 END IF;
 v_tenant:=current_setting('app.tenant_id')::uuid;
 v_store:=current_setting('app.store_id')::uuid;
 v_buyer:=current_setting('app.buyer_id')::uuid;
 -- The order must be this buyer's, placed by this session moments ago and not cancelled (promotions.redeem's fence).
 SELECT o.quote_id INTO v_quote FROM checkout.orders o WHERE o.tenant_id=v_tenant AND o.store_id=v_store AND o.owner_id=v_buyer
  AND o.id=p_order AND o.creator_session_id=current_setting('app.buyer_session_id')::uuid
  AND o.created_at>=clock_timestamp()-interval '1 minute' AND o.commercial_state<>'CANCELLED';
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 -- The live-priced lines come from the order's own quote (begin_hold proved it byte-equal to the order snapshot), never from arguments.
 -- Claim-line order is the lock order.
 FOR r IN SELECT (x->>'claim_bundle_id')::uuid AS bundle_id,(x->>'claim_offer_id')::uuid AS offer_id,(x->>'sku_id')::uuid AS sku_id,
   (x->>'quantity')::bigint AS quantity
  FROM storefront.quotes q CROSS JOIN LATERAL jsonb_array_elements(q.snapshot->'lines') x
  WHERE q.tenant_id=v_tenant AND q.store_id=v_store AND q.owner_id=v_buyer AND q.id=v_quote AND x->>'price_rule'='live_claim'
  ORDER BY 1,2 LOOP
  -- The serialisation point: every consumption of this claim line waits here, and the sum below (READ COMMITTED: a fresh snapshot per
  -- statement) sees every use committed before it, so two placements can never both take the last units.
  SELECT l.quantity INTO v_claimed FROM claims.lines l JOIN claims.bundles b ON b.tenant_id=l.tenant_id AND b.store_id=l.store_id
   AND b.id=l.bundle_id AND b.owner_id=v_buyer
   WHERE l.tenant_id=v_tenant AND l.store_id=v_store AND l.bundle_id=r.bundle_id AND l.offer_id=r.offer_id AND l.sku_id=r.sku_id
   FOR UPDATE OF l;
  IF NOT FOUND THEN RAISE EXCEPTION 'live price no longer available' USING ERRCODE='PT409'; END IF;
  SELECT coalesce(sum(u.quantity),0) INTO v_held FROM claims.live_price_uses u
   JOIN checkout.orders o ON o.tenant_id=u.tenant_id AND o.store_id=u.store_id AND o.id=u.order_id AND o.commercial_state<>'CANCELLED'
   WHERE u.tenant_id=v_tenant AND u.store_id=v_store AND u.bundle_id=r.bundle_id AND u.offer_id=r.offer_id;
  IF r.quantity IS NULL OR r.quantity<1 OR v_held+r.quantity>v_claimed THEN
   RAISE EXCEPTION 'live price no longer available' USING ERRCODE='PT409'; END IF;
  INSERT INTO claims.live_price_uses(tenant_id,store_id,bundle_id,offer_id,order_id,quantity)
   VALUES(v_tenant,v_store,r.bundle_id,r.offer_id,p_order,r.quantity);
  v_count:=v_count+1;
 END LOOP;
 RETURN v_count;
END $$;
ALTER FUNCTION claims.consume_live_prices(uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.consume_live_prices(uuid) FROM PUBLIC;
-- checkout.Begin runs on the checkout pool (the 0103 D1 lesson: grant the role that actually runs the call, never the buyer pool).
GRANT EXECUTE ON FUNCTION claims.consume_live_prices(uuid) TO commerce_checkout_runtime;
COMMENT ON FUNCTION claims.consume_live_prices(uuid) IS
 'internal/claims (SQL definer; its only caller is internal/storefront ConsumeLivePrices from internal/checkout Begin, after checkout.begin_hold in the same transaction on commerce_checkout_runtime); EXECUTE: commerce_checkout_runtime. For every live_claim line of the order''s quote: locks the claim line FOR UPDATE (bundle still bound to the caller), re-checks claimed minus held uses covers the line quantity, inserts the claims.live_price_uses row; returns the row count. PT409 (live price no longer available) rolls the whole placement back and the buyer re-quotes; PT404 when the order is not the caller''s fresh order. Non-goals: no price computation, no release (derived from order state).';
