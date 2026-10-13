-- Purpose: exclude attempt-owned reservations/grants from every legacy claims path.
-- Depends on: 0175 attempt identity columns; exact forward bodies from 0129 and 0158.
-- Used by: existing merchant and storefront claim entry points; C2 uses separate helpers.
-- Invariants: legacy NULL-attempt behavior unchanged; only fenced C2/sweep may settle
-- attempt resources. CREATE OR REPLACE preserves the existing owner/ACL/comments.

CREATE OR REPLACE FUNCTION claims.for_buyer_begin(p_key_hash bytea, p_request_hash bytea, p_conversation uuid, p_bundles uuid[], p_buyer uuid)
RETURNS TABLE(o_request uuid, o_reason text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; v_req uuid; v_reason text; b uuid; r record; v_order uuid; v_live uuid; v_own boolean; v_hash bytea; v_now timestamptz:=clock_timestamp(); v_sorted uuid[];
BEGIN
 v:=claims.for_buyer_scope();
 IF p_key_hash IS NULL OR octet_length(p_key_hash)<>32 OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_bundles IS NULL OR cardinality(p_bundles)>5
  OR (cardinality(p_bundles)>0 AND p_buyer IS NULL) THEN
  RAISE EXCEPTION 'invalid for-buyer request' USING ERRCODE='22023';
 END IF;
 v_sorted:=ARRAY(SELECT DISTINCT x FROM unnest(p_bundles) x ORDER BY 1);
 IF cardinality(v_sorted)<>cardinality(p_bundles) OR EXISTS (SELECT 1 FROM unnest(p_bundles) x WHERE x IS NULL) THEN
  RAISE EXCEPTION 'invalid for-buyer request' USING ERRCODE='22023';
 END IF;
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF cardinality(v_sorted)=0 THEN
  o_request:=gen_random_uuid(); o_reason:='no_bundle'; RETURN NEXT; RETURN;   -- a plain manual order: nothing to reserve, no price
 END IF;
 IF (SELECT count(*) FROM claims.bundles x WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.id=ANY(v_sorted) AND x.purged_at IS NULL)<>cardinality(v_sorted) THEN
  RAISE EXCEPTION 'not found' USING ERRCODE='PT404';
 END IF;
 -- §5.3 fail-closed eligibility, evaluated before anything is written (a mismatch creates nothing).
 v_reason:=claims.for_buyer_peer_state(p_conversation,v_sorted);
 IF v_reason='bundle_buyer_mismatch' THEN RAISE EXCEPTION 'bundle_buyer_mismatch' USING ERRCODE='PT409'; END IF;
 IF v_reason='' AND NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['live:manage']) THEN v_reason:='permission'; END IF;
 -- The request this Idempotency-Key already started (resume), else a new one. The same key with another request body is refused (P2-4).
 SELECT x.request_id,x.request_hash INTO v_req,v_hash FROM inbox.order_for_buyer x
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.idempotency_key_hash=p_key_hash LIMIT 1;
 IF NOT FOUND THEN v_req:=gen_random_uuid();
 ELSIF v_hash<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 IF EXISTS(SELECT 1 FROM inbox.order_for_buyer x WHERE x.tenant_id=v[1] AND x.store_id=v[2]
  AND x.request_id=v_req AND x.create_attempt_id IS NOT NULL) THEN
  RAISE EXCEPTION 'attempt_owned_reservation' USING ERRCODE='PT409';
 END IF;
 FOREACH b IN ARRAY v_sorted LOOP
  -- §5.1 3a: lock the live row of the bundle (sorted order = lock order); another request's row blocks unless its order is CANCELLED or the
  -- row is a pending one that has not been touched for 15 minutes AND whose capability owner has no non-CANCELLED order (a crash between Place and
  -- the record leaves a pending row WITH an order: that bundle is ordered, never stealable; a live resume refreshes updated_at).
  SELECT x.request_id,x.state,x.order_id,x.updated_at,x.buyer_id,x.create_attempt_id INTO r FROM inbox.order_for_buyer x
   WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=b AND x.state IN ('pending','placed') FOR UPDATE;
  IF FOUND AND r.create_attempt_id IS NOT NULL THEN
   RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409';
  END IF;
  v_own:=FOUND AND r.request_id=v_req;
  IF FOUND AND NOT v_own THEN
   SELECT o.id INTO v_live FROM checkout.orders o WHERE o.tenant_id=v[1] AND o.store_id=v[2] AND o.owner_id=r.buyer_id AND o.commercial_state<>'CANCELLED'
    ORDER BY o.created_at DESC LIMIT 1;
   IF (r.state='placed' AND EXISTS (SELECT 1 FROM checkout.orders o WHERE o.tenant_id=v[1] AND o.store_id=v[2] AND o.id=r.order_id AND o.commercial_state='CANCELLED'))
    OR (r.state='pending' AND v_live IS NULL AND r.updated_at<v_now-interval '15 minutes') THEN
    UPDATE inbox.order_for_buyer x SET state='released',updated_at=v_now
     WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=b AND x.request_id=r.request_id AND x.state IN ('pending','placed');
   ELSE
    RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409', DETAIL=coalesce(r.order_id::text,v_live::text,'');
   END IF;
  END IF;
  IF v_own THEN
   UPDATE inbox.order_for_buyer x SET updated_at=v_now WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=b AND x.request_id=v_req AND x.state='pending';
  END IF;
  -- A bundle whose claim lines are all fully consumed (the buyer ordered through the link) is also already ordered; a resume of THIS request
  -- (its own live row) is exempt, because its own order is what consumed the lines.
  IF NOT v_own AND EXISTS (SELECT 1 FROM claims.lines l WHERE l.tenant_id=v[1] AND l.store_id=v[2] AND l.bundle_id=b)
   AND NOT EXISTS (SELECT 1 FROM claims.lines l WHERE l.tenant_id=v[1] AND l.store_id=v[2] AND l.bundle_id=b
    AND l.quantity>(SELECT coalesce(sum(u.quantity),0) FROM claims.live_price_uses u
      JOIN checkout.orders r2 ON r2.tenant_id=u.tenant_id AND r2.store_id=u.store_id AND r2.id=u.order_id AND r2.commercial_state<>'CANCELLED'
      WHERE u.tenant_id=l.tenant_id AND u.store_id=l.store_id AND u.bundle_id=l.bundle_id AND u.offer_id=l.offer_id)) THEN
   SELECT u.order_id INTO v_order FROM claims.live_price_uses u JOIN checkout.orders r2 ON r2.tenant_id=u.tenant_id AND r2.store_id=u.store_id
    AND r2.id=u.order_id AND r2.commercial_state<>'CANCELLED'
    WHERE u.tenant_id=v[1] AND u.store_id=v[2] AND u.bundle_id=b ORDER BY r2.created_at DESC LIMIT 1;
   RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409', DETAIL=coalesce(v_order::text,'');
  END IF;
  BEGIN
   INSERT INTO inbox.order_for_buyer AS x(tenant_id,store_id,bundle_id,request_id,idempotency_key_hash,request_hash,buyer_id,state,conversation_id,principal_id)
    VALUES(v[1],v[2],b,v_req,p_key_hash,p_request_hash,p_buyer,'pending',p_conversation,v[3])
    ON CONFLICT (tenant_id,store_id,request_id,bundle_id) DO UPDATE SET state='pending',updated_at=v_now WHERE x.state='released';
  EXCEPTION WHEN unique_violation THEN
   RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409', DETAIL='';   -- the partial unique index lost a race with another request
  END;
 END LOOP;
 IF v_reason='' THEN
  FOREACH b IN ARRAY v_sorted LOOP
   -- §5.3: one single-use grant per bundle for THIS buyer; a resume of an unbound, unconsumed grant renews the 15 minutes.
   INSERT INTO claims.merchant_origin_grants AS g(tenant_id,store_id,request_id,buyer_id,bundle_id,principal_id,expires_at)
    VALUES(v[1],v[2],v_req,p_buyer,b,v[3],v_now+interval '15 minutes')
    ON CONFLICT (tenant_id,store_id,request_id,bundle_id) DO UPDATE SET expires_at=v_now+interval '15 minutes'
     WHERE g.consumed_at IS NULL AND g.quote_id IS NULL AND g.buyer_id=p_buyer;
  END LOOP;
 END IF;
 IF v_reason<>'' THEN
  -- A replay whose eligibility now fails must not leave the grants an earlier run wrote (P2-5): expire them.
  UPDATE claims.merchant_origin_grants g SET expires_at=v_now
   WHERE g.tenant_id=v[1] AND g.store_id=v[2] AND g.request_id=v_req AND g.consumed_at IS NULL AND g.expires_at>v_now;
 END IF;
 o_request:=v_req; o_reason:=v_reason; RETURN NEXT;
END $$;

CREATE OR REPLACE FUNCTION claims.for_buyer_finish(p_request uuid, p_order uuid)
RETURNS TABLE(bundle_id uuid, offer_id uuid, sku_id uuid, quantity bigint, live_price_minor bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; v_total bigint; v_n bigint; v_buyer uuid;
BEGIN
 v:=claims.for_buyer_scope();
 IF p_request IS NULL OR p_order IS NULL THEN RAISE EXCEPTION 'invalid for-buyer request' USING ERRCODE='22023'; END IF;
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF EXISTS(SELECT 1 FROM inbox.order_for_buyer x WHERE x.tenant_id=v[1] AND x.store_id=v[2]
  AND x.request_id=p_request AND x.create_attempt_id IS NOT NULL) THEN
  RAISE EXCEPTION 'attempt_owned_reservation' USING ERRCODE='PT409';
 END IF;
 SELECT x.buyer_id INTO v_buyer FROM inbox.order_for_buyer x WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.request_id=p_request LIMIT 1;
 IF NOT FOUND THEN RAISE EXCEPTION 'reservation_lost' USING ERRCODE='PT409'; END IF;
 -- The order must be a non-cancelled order of THIS request's capability owner.
 PERFORM 1 FROM checkout.orders o WHERE o.tenant_id=v[1] AND o.store_id=v[2] AND o.id=p_order AND o.owner_id=v_buyer AND o.commercial_state<>'CANCELLED';
 IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 SELECT count(*) INTO v_total FROM inbox.order_for_buyer x WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.request_id=p_request;
 UPDATE inbox.order_for_buyer x SET state='placed',order_id=p_order,updated_at=clock_timestamp()
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.request_id=p_request AND x.state IN ('pending','placed') AND (x.order_id IS NULL OR x.order_id=p_order);
 GET DIAGNOSTICS v_n=ROW_COUNT;
 -- Every bundle row of the request must now be placed with this order; a released or taken row means the one-live-order rule was lost (P2-1).
 IF v_n<>v_total THEN RAISE EXCEPTION 'reservation_lost' USING ERRCODE='PT409'; END IF;
 -- The request is done: grants it did not consume (a bundle none of whose lines were priced live) expire now (P2-3).
 UPDATE claims.merchant_origin_grants g SET expires_at=clock_timestamp()
  WHERE g.tenant_id=v[1] AND g.store_id=v[2] AND g.request_id=p_request AND g.consumed_at IS NULL AND g.expires_at>clock_timestamp();
 RETURN QUERY
 SELECT u.bundle_id,u.offer_id,l.sku_id,u.quantity::bigint,u.unit_price_minor
 FROM claims.live_price_uses u
 JOIN claims.lines l ON l.tenant_id=u.tenant_id AND l.store_id=u.store_id AND l.bundle_id=u.bundle_id AND l.offer_id=u.offer_id
 WHERE u.tenant_id=v[1] AND u.store_id=v[2] AND u.order_id=p_order ORDER BY u.bundle_id,u.offer_id;
END $$;

CREATE OR REPLACE FUNCTION claims.for_buyer_release(p_request uuid) RETURNS integer
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; n integer;
BEGIN
 v:=claims.for_buyer_scope();
 IF p_request IS NULL THEN RAISE EXCEPTION 'invalid for-buyer request' USING ERRCODE='22023'; END IF;
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF EXISTS(SELECT 1 FROM inbox.order_for_buyer x WHERE x.tenant_id=v[1] AND x.store_id=v[2]
  AND x.request_id=p_request AND x.create_attempt_id IS NOT NULL) THEN
  RAISE EXCEPTION 'attempt_owned_reservation' USING ERRCODE='PT409';
 END IF;
 UPDATE inbox.order_for_buyer x SET state='released',updated_at=clock_timestamp()
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.request_id=p_request AND x.state='pending'
   -- never release a bundle that has an order: Place can fail AFTER begin_hold (the manual-source mark), and the order still exists (P2-1 c)
   AND NOT EXISTS (SELECT 1 FROM checkout.orders o WHERE o.tenant_id=x.tenant_id AND o.store_id=x.store_id AND o.owner_id=x.buyer_id AND o.commercial_state<>'CANCELLED');
 GET DIAGNOSTICS n=ROW_COUNT;
 UPDATE claims.merchant_origin_grants g SET expires_at=clock_timestamp()
  WHERE g.tenant_id=v[1] AND g.store_id=v[2] AND g.request_id=p_request AND g.consumed_at IS NULL;
 RETURN n;
END $$;

CREATE OR REPLACE FUNCTION claims.bind_merchant_origin_grant(p_quote uuid) RETURNS integer
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_buyer uuid; v_bundles uuid[]; n integer;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
  OR coalesce(current_setting('app.tenant_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.store_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_session_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR current_setting('app.principal_id',true) IS DISTINCT FROM '' OR p_quote IS NULL THEN
  RAISE EXCEPTION 'invalid grant binding' USING ERRCODE='22023';
 END IF;
 v_tenant:=current_setting('app.tenant_id')::uuid; v_store:=current_setting('app.store_id')::uuid; v_buyer:=current_setting('app.buyer_id')::uuid;
 -- The bundles come from the buyer's own quote snapshot (priced lines), never from an argument.
 SELECT array_agg(DISTINCT (x->>'claim_bundle_id')::uuid) INTO v_bundles
  FROM storefront.quotes q CROSS JOIN LATERAL jsonb_array_elements(q.snapshot->'lines') x
  WHERE q.tenant_id=v_tenant AND q.store_id=v_store AND q.owner_id=v_buyer AND q.id=p_quote AND x->>'price_rule'='live_claim';
 IF v_bundles IS NULL THEN RETURN 0; END IF;
 UPDATE claims.merchant_origin_grants g SET quote_id=p_quote
  WHERE g.create_attempt_id IS NULL AND g.tenant_id=v_tenant AND g.store_id=v_store AND g.buyer_id=v_buyer AND g.bundle_id=ANY(v_bundles)
   AND g.consumed_at IS NULL AND g.expires_at>clock_timestamp() AND (g.quote_id IS NULL OR g.quote_id=p_quote);
 GET DIAGNOSTICS n=ROW_COUNT;
 RETURN n;
END $$;

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
       AND EXISTS (SELECT 1 FROM claims.merchant_origin_grants g WHERE g.create_attempt_id IS NULL AND g.tenant_id=b.tenant_id AND g.store_id=b.store_id AND g.buyer_id=v_buyer
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

CREATE OR REPLACE FUNCTION claims.consume_live_prices(p_order uuid) RETURNS integer
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_buyer uuid; v_quote uuid; r record; v_claimed integer; v_held bigint; v_count integer:=0;
 v_via boolean; v_bundles uuid[]:='{}'; v_n integer;
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
   (x->>'quantity')::bigint AS quantity,(x->>'unit_price_minor')::bigint AS unit_price
  FROM storefront.quotes q CROSS JOIN LATERAL jsonb_array_elements(q.snapshot->'lines') x
  WHERE q.tenant_id=v_tenant AND q.store_id=v_store AND q.owner_id=v_buyer AND q.id=v_quote AND x->>'price_rule'='live_claim'
  ORDER BY 1,2 LOOP
  -- The serialisation point: every consumption of this claim line waits here, and the sum below (READ COMMITTED: a fresh snapshot per
  -- statement) sees every use committed before it, so two placements can never both take the last units.
  SELECT l.quantity,(b.owner_id IS DISTINCT FROM v_buyer) INTO v_claimed,v_via FROM claims.lines l JOIN claims.bundles b ON b.tenant_id=l.tenant_id AND b.store_id=l.store_id
   AND b.id=l.bundle_id
   AND (b.owner_id=v_buyer OR (b.purged_at IS NULL AND EXISTS (SELECT 1 FROM live.sessions z WHERE z.tenant_id=b.tenant_id AND z.store_id=b.store_id
        AND z.id=b.session_id AND z.lifecycle<>'archived')))
   WHERE l.tenant_id=v_tenant AND l.store_id=v_store AND l.bundle_id=r.bundle_id AND l.offer_id=r.offer_id AND l.sku_id=r.sku_id
   FOR UPDATE OF l;
  IF NOT FOUND THEN RAISE EXCEPTION 'live price no longer available' USING ERRCODE='PT409'; END IF;
  -- 0129: a bundle the buyer does not own is priced only through a merchant-origin grant bound to THIS order's quote (single use: the row
  -- lock below plus consumed_at at the end serialise two Begins of one grant; no grant, a consumed, expired or unbound one = PT409, fail closed).
  IF v_via THEN
   PERFORM 1 FROM claims.merchant_origin_grants g WHERE g.create_attempt_id IS NULL AND g.tenant_id=v_tenant AND g.store_id=v_store AND g.buyer_id=v_buyer
    AND g.bundle_id=r.bundle_id AND g.quote_id=v_quote AND g.consumed_at IS NULL AND g.expires_at>clock_timestamp() FOR UPDATE;
   IF NOT FOUND THEN RAISE EXCEPTION 'live price no longer available' USING ERRCODE='PT409'; END IF;
   IF NOT (r.bundle_id=ANY(v_bundles)) THEN v_bundles:=v_bundles||r.bundle_id; END IF;
  END IF;
  SELECT coalesce(sum(u.quantity),0) INTO v_held FROM claims.live_price_uses u
   JOIN checkout.orders o ON o.tenant_id=u.tenant_id AND o.store_id=u.store_id AND o.id=u.order_id AND o.commercial_state<>'CANCELLED'
   WHERE u.tenant_id=v_tenant AND u.store_id=v_store AND u.bundle_id=r.bundle_id AND u.offer_id=r.offer_id;
  IF r.quantity IS NULL OR r.quantity<1 OR v_held+r.quantity>v_claimed THEN
   RAISE EXCEPTION 'live price no longer available' USING ERRCODE='PT409'; END IF;
  INSERT INTO claims.live_price_uses(tenant_id,store_id,bundle_id,offer_id,order_id,quantity,unit_price_minor)
   VALUES(v_tenant,v_store,r.bundle_id,r.offer_id,p_order,r.quantity,r.unit_price);
  v_count:=v_count+1;
 END LOOP;
 IF cardinality(v_bundles)>0 THEN
  UPDATE claims.merchant_origin_grants g SET consumed_at=clock_timestamp() WHERE g.create_attempt_id IS NULL AND g.tenant_id=v_tenant AND g.store_id=v_store AND g.buyer_id=v_buyer
   AND g.bundle_id=ANY(v_bundles) AND g.quote_id=v_quote AND g.consumed_at IS NULL;
  GET DIAGNOSTICS v_n=ROW_COUNT;
  IF v_n<>cardinality(v_bundles) THEN RAISE EXCEPTION 'live price no longer available' USING ERRCODE='PT409'; END IF;
 END IF;
 RETURN v_count;
END $$;
