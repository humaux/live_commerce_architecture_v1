-- 0129_lc_b6_order_for_buyer.sql — LC-B6 (W2-06B, live-console-v1 §5 order for a buyer, A15/A16, Amendment 1 A1.3 + P2-7 + P2-8). The
-- contract table names 0125; the integrator assigned 0129 (0125 = meta connection health, 0126 = tracking import, 0127 = LC-R1, 0128 = LC-B4).
-- Purpose: the merchant creates the order FOR a buyer who will not click the pushed link. Adds (1) inbox.order_for_buyer, the reservation rows
--   that make "one live order per bundle" true across Place's several transactions, (2) claims.merchant_origin_grants, the single-use,
--   quote-bound, 15-minute grant of the OPEN-13 merchant-attested live price, (3) seven claims definers (scope helper, peer state, prefill
--   lines, begin = reserve + eligibility + grant, finish, release, bind-to-quote), (4) the grant branch inside claims.live_prices and
--   claims.consume_live_prices (same signatures), (5) inbox.plan_dm: a for-buyer pay-link DM (p_order set) takes the semantic key
--   mdm:+sha256(conversation|order-pay-link|order).
-- Depends on: 0060/0092/0103/0105 (claims.live_prices, consume_live_prices, live_price_uses), 0122 (live.sessions.lifecycle), 0071 (purged_at),
--   0128 (inbox.bundle_peers, inbox.plan_dm/lcn_emit), 0119 (social.conversations), identity.principal_holds (0064).
-- Used by: internal/claims/live_price_grant.go (the definers), internal/merchanttools/order_for_buyer.go (A15/A16), internal/storefront
--   RevalidateQuote (sets app.quote_id), internal/inbox/send_order.go (the DM), internal/httpapi/merchanttools.go.
-- Invariants: I05/I08 (the Quote stays the only price; the grant only lets claims.live_prices return the offer's live price), I03 (stock is held by
--   begin_hold, nothing here touches stock), fail-closed eligibility (live-console-v1 §5.3 / integrator money-path ruling on OPEN-13):
--   the SERVER compares the bundle's peer key with the thread's peer key; unknown peer, no conversation or no live:manage = catalog price,
--   known and different = PT409 bundle_buyer_mismatch with nothing created; the 0105 consumption ledger stays the only quantity ceiling.
-- Deviations: (1) claims.merchant_origin_grants has request_id in its primary key (the contract names the columns, not the key); (2) the reservation
--   table carries updated_at; (3) refusals are PT409 with the code as message, bundle_already_ordered carries the order id in DETAIL (empty = none).
-- Status: MOCK (REAL_PG; no PSP, no Graph call here).

GRANT USAGE ON SCHEMA social TO commerce_claims_writer;

-- ---------------------------------------------------------------------------------------
-- Tables
-- ---------------------------------------------------------------------------------------
CREATE TABLE inbox.order_for_buyer (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    bundle_id uuid NOT NULL,
    request_id uuid NOT NULL,
    idempotency_key_hash bytea NOT NULL CHECK (octet_length(idempotency_key_hash) = 32),
    order_id uuid,
    state text NOT NULL CHECK (state IN ('pending','placed','released')),
    conversation_id uuid,
    principal_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, request_id, bundle_id),
    CHECK (state <> 'placed' OR order_id IS NOT NULL)
);
-- The one-live-order-per-bundle rule: Place spans several transactions, so the unique index (not a held lock) is the arbiter.
CREATE UNIQUE INDEX order_for_buyer_live ON inbox.order_for_buyer(tenant_id, store_id, bundle_id) WHERE state IN ('pending','placed');
CREATE INDEX order_for_buyer_key ON inbox.order_for_buyer(tenant_id, store_id, idempotency_key_hash);

CREATE TABLE claims.merchant_origin_grants (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    request_id uuid NOT NULL,
    buyer_id uuid NOT NULL,
    bundle_id uuid NOT NULL,
    quote_id uuid,
    principal_id uuid NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, request_id, bundle_id),
    CHECK (consumed_at IS NULL OR quote_id IS NOT NULL)
);
CREATE INDEX merchant_origin_grants_buyer ON claims.merchant_origin_grants(tenant_id, store_id, buyer_id, bundle_id);

ALTER TABLE inbox.order_for_buyer ENABLE ROW LEVEL SECURITY;
ALTER TABLE inbox.order_for_buyer FORCE ROW LEVEL SECURITY;
ALTER TABLE claims.merchant_origin_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE claims.merchant_origin_grants FORCE ROW LEVEL SECURITY;
REVOKE ALL ON inbox.order_for_buyer, claims.merchant_origin_grants FROM PUBLIC;

-- ---------------------------------------------------------------------------------------
-- commerce_claims_writer (NOLOGIN definer owner of every function below). No runtime role holds any privilege on these tables.
-- ---------------------------------------------------------------------------------------
GRANT SELECT, INSERT ON inbox.order_for_buyer TO commerce_claims_writer;
GRANT UPDATE(order_id, state, updated_at) ON inbox.order_for_buyer TO commerce_claims_writer;
CREATE POLICY order_for_buyer_writer ON inbox.order_for_buyer FOR ALL TO commerce_claims_writer
 USING (tenant_id = nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id = nullif(current_setting('app.store_id',true),'')::uuid
    AND nullif(current_setting('app.principal_id',true),'') IS NOT NULL AND nullif(current_setting('app.buyer_id',true),'') IS NULL)
 WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id = nullif(current_setting('app.store_id',true),'')::uuid
    AND nullif(current_setting('app.principal_id',true),'') IS NOT NULL AND nullif(current_setting('app.buyer_id',true),'') IS NULL);

GRANT SELECT, INSERT ON claims.merchant_origin_grants TO commerce_claims_writer;
GRANT UPDATE(quote_id, consumed_at, expires_at) ON claims.merchant_origin_grants TO commerce_claims_writer;
CREATE POLICY grant_writer_read ON claims.merchant_origin_grants FOR SELECT TO commerce_claims_writer
 USING (tenant_id = nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id = nullif(current_setting('app.store_id',true),'')::uuid);
-- Written by the merchant transaction only (for_buyer_begin); updated by the merchant (extend/expire) or the buyer transaction (bind, consume).
CREATE POLICY grant_writer_insert ON claims.merchant_origin_grants FOR INSERT TO commerce_claims_writer
 WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id = nullif(current_setting('app.store_id',true),'')::uuid
    AND nullif(current_setting('app.principal_id',true),'') IS NOT NULL AND nullif(current_setting('app.buyer_id',true),'') IS NULL);
CREATE POLICY grant_writer_update ON claims.merchant_origin_grants FOR UPDATE TO commerce_claims_writer
 USING (tenant_id = nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id = nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id = nullif(current_setting('app.store_id',true),'')::uuid);

-- Read-only facts the definers need (all scoped to the transaction's tenant/store GUCs).
GRANT SELECT(purged_at) ON claims.bundles TO commerce_claims_writer;
GRANT SELECT(tenant_id, store_id, id, lifecycle) ON live.sessions TO commerce_claims_writer;
CREATE POLICY session_claims_read ON live.sessions FOR SELECT TO commerce_claims_writer
 USING (tenant_id = nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id = nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT(id, tenant_id, store_id, app_id, object, asset_id, peer_key) ON social.conversations TO commerce_claims_writer;
CREATE POLICY conversation_claims_read ON social.conversations FOR SELECT TO commerce_claims_writer
 USING (tenant_id = nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id = nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT(tenant_id, store_id, bundle_id, peer_key, app_id, object, asset_id) ON inbox.bundle_peers TO commerce_claims_writer;
CREATE POLICY bundle_peers_claims_read ON inbox.bundle_peers FOR SELECT TO commerce_claims_writer
 USING (tenant_id = nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id = nullif(current_setting('app.store_id',true),'')::uuid);

-- ---------------------------------------------------------------------------------------
-- claims.for_buyer_scope: the merchant-transaction fence of the for-buyer definers (tenant, store and principal GUCs set, no buyer GUC).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION claims.for_buyer_scope() RETURNS uuid[]
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
  OR coalesce(current_setting('app.tenant_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.store_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.principal_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL THEN
  RAISE EXCEPTION 'invalid for-buyer scope' USING ERRCODE='22023';
 END IF;
 RETURN ARRAY[current_setting('app.tenant_id')::uuid,current_setting('app.store_id')::uuid,current_setting('app.principal_id')::uuid];
END $$;
ALTER FUNCTION claims.for_buyer_scope() OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.for_buyer_scope() FROM PUBLIC;
COMMENT ON FUNCTION claims.for_buyer_scope() IS
 'internal/claims (0129 LC-B6, owner-only helper of the for-buyer definers; EXECUTE: commerce_claims_writer only, no caller role): pins the merchant transaction scope (tenant, store, principal GUCs, no buyer GUC) and returns {tenant, store, principal}.';

-- ---------------------------------------------------------------------------------------
-- claims.for_buyer_peer_state: the server-side buyer comparison of live-console-v1 §5.3 (fail closed).
--   '' = every bundle's peers all equal the thread's peer; 'no_bundle' / 'no_conversation' / 'bundle_buyer_unverified' (a bundle has no peer
--   key) / 'bundle_buyer_mismatch' (a bundle has a peer key different from the thread's: the caller refuses with 409 and creates nothing).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION claims.for_buyer_peer_state(p_conversation uuid, p_bundles uuid[]) RETURNS text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; c record; b uuid; n bigint; m bigint; v_unverified boolean:=false;
BEGIN
 v:=claims.for_buyer_scope();
 IF p_bundles IS NULL OR cardinality(p_bundles)=0 THEN RETURN 'no_bundle'; END IF;
 IF p_conversation IS NULL THEN RETURN 'no_conversation'; END IF;
 SELECT x.peer_key,x.app_id,x.object,x.asset_id INTO c FROM social.conversations x WHERE x.id=p_conversation AND x.tenant_id=v[1] AND x.store_id=v[2];
 IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 FOREACH b IN ARRAY p_bundles LOOP
  SELECT count(*),count(*) FILTER (WHERE p.peer_key=c.peer_key AND p.app_id=c.app_id AND p.object=c.object AND p.asset_id=c.asset_id) INTO n,m
   FROM inbox.bundle_peers p WHERE p.tenant_id=v[1] AND p.store_id=v[2] AND p.bundle_id=b;
  IF n=0 THEN v_unverified:=true;
  ELSIF m<n THEN RETURN 'bundle_buyer_mismatch'; END IF;
 END LOOP;
 IF v_unverified THEN RETURN 'bundle_buyer_unverified'; END IF;
 RETURN '';
END $$;
ALTER FUNCTION claims.for_buyer_peer_state(uuid,uuid[]) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.for_buyer_peer_state(uuid,uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.for_buyer_peer_state(uuid,uuid[]) TO commerce_runtime;
COMMENT ON FUNCTION claims.for_buyer_peer_state(uuid,uuid[]) IS
 'internal/claims (0129 LC-B6, helper of for_buyer_begin and the order-prefill eligibility read; caller commerce_runtime inside the merchant transaction): compares the peer keys of the bundles (inbox.bundle_peers) with the thread''s peer key (social.conversations) inside the merchant transaction scope; returns the empty string only when every bundle has peers and all equal the thread''s; otherwise no_bundle | no_conversation | bundle_buyer_unverified | bundle_buyer_mismatch. Never trusts a caller-supplied peer.';

-- ---------------------------------------------------------------------------------------
-- claims.for_buyer_lines (A15): the open claim lines of the actor's bundles with the live price and the live quantity still available.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION claims.for_buyer_lines(p_conversation uuid, p_bundle uuid)
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
  CASE WHEN o.active AND o.live_price_minor IS NOT NULL THEN o.live_price_minor END,
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
REVOKE ALL ON FUNCTION claims.for_buyer_lines(uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.for_buyer_lines(uuid,uuid) TO commerce_runtime;
COMMENT ON FUNCTION claims.for_buyer_lines(uuid,uuid) IS
 'internal/claims (0129 LC-B6, GET inbox/order-prefill; caller commerce_runtime inside the merchant transaction, inventory:reserve re-verified by identity.principal_holds; the route adds orders:read): the claim lines (capped at 50) of one bundle, or of every bundle whose recorded peer is the conversation''s peer (none = zero rows), with the offer keyword, the live price when the offer is active and priced (else NULL) and live_remaining = claimed minus units held in claims.live_price_uses by non-CANCELLED orders. PT404 for a conversation or bundle outside the store. Read-only.';

-- ---------------------------------------------------------------------------------------
-- claims.for_buyer_begin (A16 steps 3a + 3b): reservation rows, fail-closed eligibility, the single-use grants.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION claims.for_buyer_begin(p_key_hash bytea, p_conversation uuid, p_bundles uuid[], p_buyer uuid)
RETURNS TABLE(o_request uuid, o_reason text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; v_req uuid; v_reason text; b uuid; r record; v_order uuid; v_own boolean; v_now timestamptz:=clock_timestamp(); v_sorted uuid[];
BEGIN
 v:=claims.for_buyer_scope();
 IF p_key_hash IS NULL OR octet_length(p_key_hash)<>32 OR p_bundles IS NULL OR cardinality(p_bundles)>5
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
 -- The request this Idempotency-Key already started (resume), else a new one.
 SELECT x.request_id INTO v_req FROM inbox.order_for_buyer x WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.idempotency_key_hash=p_key_hash LIMIT 1;
 IF NOT FOUND THEN v_req:=gen_random_uuid(); END IF;
 FOREACH b IN ARRAY v_sorted LOOP
  -- §5.1 3a: lock the live row of the bundle (sorted order = lock order); another request's row blocks unless its order is CANCELLED or the
  -- row is a pending one older than 15 minutes with no order.
  SELECT x.request_id,x.state,x.order_id,x.created_at INTO r FROM inbox.order_for_buyer x
   WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=b AND x.state IN ('pending','placed') FOR UPDATE;
  v_own:=FOUND AND r.request_id=v_req;
  IF FOUND AND NOT v_own THEN
   IF (r.state='placed' AND EXISTS (SELECT 1 FROM checkout.orders o WHERE o.tenant_id=v[1] AND o.store_id=v[2] AND o.id=r.order_id AND o.commercial_state='CANCELLED'))
    OR (r.state='pending' AND r.order_id IS NULL AND r.created_at<v_now-interval '15 minutes') THEN
    UPDATE inbox.order_for_buyer x SET state='released',updated_at=v_now
     WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=b AND x.request_id=r.request_id AND x.state IN ('pending','placed');
   ELSE
    RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409', DETAIL=coalesce(r.order_id::text,'');
   END IF;
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
   INSERT INTO inbox.order_for_buyer AS x(tenant_id,store_id,bundle_id,request_id,idempotency_key_hash,state,conversation_id,principal_id)
    VALUES(v[1],v[2],b,v_req,p_key_hash,'pending',p_conversation,v[3])
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
 o_request:=v_req; o_reason:=v_reason; RETURN NEXT;
END $$;
ALTER FUNCTION claims.for_buyer_begin(bytea,uuid,uuid[],uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.for_buyer_begin(bytea,uuid,uuid[],uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.for_buyer_begin(bytea,uuid,uuid[],uuid) TO commerce_runtime;
COMMENT ON FUNCTION claims.for_buyer_begin(bytea,uuid,uuid[],uuid) IS
 'internal/claims (0129 LC-B6, POST orders/for-buyer steps 3a+3b; caller commerce_runtime inside the merchant transaction, inventory:reserve re-verified): reserves one inbox.order_for_buyer row per bundle (sorted lock order; PT409 bundle_already_ordered with the order id in DETAIL when another live request or fully consumed claim lines hold it), evaluates the fail-closed buyer comparison (PT409 bundle_buyer_mismatch, nothing created) and, only when the comparison passed and the caller holds live:manage, writes one single-use 15-minute claims.merchant_origin_grants row per bundle for the capability owner p_buyer. Returns the request id and the reason no grant was written (empty string = granted). Idempotent per key hash (a resume finds its own rows).';

-- ---------------------------------------------------------------------------------------
-- claims.for_buyer_finish (step 3d): the order is placed; returns the live-priced lines its ledger rows record (audit input).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION claims.for_buyer_finish(p_request uuid, p_order uuid)
RETURNS TABLE(bundle_id uuid, offer_id uuid, sku_id uuid, quantity bigint, live_price_minor bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[];
BEGIN
 v:=claims.for_buyer_scope();
 IF p_request IS NULL OR p_order IS NULL THEN RAISE EXCEPTION 'invalid for-buyer request' USING ERRCODE='22023'; END IF;
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM 1 FROM checkout.orders o WHERE o.tenant_id=v[1] AND o.store_id=v[2] AND o.id=p_order AND o.commercial_state<>'CANCELLED';
 IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 UPDATE inbox.order_for_buyer x SET state='placed',order_id=p_order,updated_at=clock_timestamp()
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.request_id=p_request AND x.state IN ('pending','placed') AND (x.order_id IS NULL OR x.order_id=p_order);
 RETURN QUERY
 SELECT u.bundle_id,u.offer_id,l.sku_id,u.quantity::bigint,o.live_price_minor
 FROM claims.live_price_uses u
 JOIN claims.lines l ON l.tenant_id=u.tenant_id AND l.store_id=u.store_id AND l.bundle_id=u.bundle_id AND l.offer_id=u.offer_id
 JOIN live.offers o ON o.tenant_id=l.tenant_id AND o.store_id=l.store_id AND o.id=l.offer_id
 WHERE u.tenant_id=v[1] AND u.store_id=v[2] AND u.order_id=p_order ORDER BY u.bundle_id,u.offer_id;
END $$;
ALTER FUNCTION claims.for_buyer_finish(uuid,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.for_buyer_finish(uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.for_buyer_finish(uuid,uuid) TO commerce_runtime;
COMMENT ON FUNCTION claims.for_buyer_finish(uuid,uuid) IS
 'internal/claims (0129 LC-B6, POST orders/for-buyer step 3d; caller commerce_runtime inside the merchant transaction, inventory:reserve re-verified): marks the request''s pending/placed reservation rows placed with the order id and returns the live-priced lines of the order from claims.live_price_uses (empty = the order was placed at catalog price). PT404 for an order that is not a live order of the store.';

-- ---------------------------------------------------------------------------------------
-- claims.for_buyer_release: a refused placement gives its bundles back and kills the request's unconsumed grants.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION claims.for_buyer_release(p_request uuid) RETURNS integer
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; n integer;
BEGIN
 v:=claims.for_buyer_scope();
 IF p_request IS NULL THEN RAISE EXCEPTION 'invalid for-buyer request' USING ERRCODE='22023'; END IF;
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 UPDATE inbox.order_for_buyer x SET state='released',updated_at=clock_timestamp()
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.request_id=p_request AND x.state='pending';
 GET DIAGNOSTICS n=ROW_COUNT;
 UPDATE claims.merchant_origin_grants g SET expires_at=clock_timestamp()
  WHERE g.tenant_id=v[1] AND g.store_id=v[2] AND g.request_id=p_request AND g.consumed_at IS NULL;
 RETURN n;
END $$;
ALTER FUNCTION claims.for_buyer_release(uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.for_buyer_release(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.for_buyer_release(uuid) TO commerce_runtime;
COMMENT ON FUNCTION claims.for_buyer_release(uuid) IS
 'internal/claims (0129 LC-B6, POST orders/for-buyer after a coded Place refusal; caller commerce_runtime inside the merchant transaction, inventory:reserve re-verified): sets the request''s pending reservation rows released and expires its unconsumed grants; returns the released row count. A placed row is never touched.';

-- ---------------------------------------------------------------------------------------
-- claims.bind_merchant_origin_grant: CreateQuote cannot name the quote it is about to insert, so the for-buyer pipeline binds the grant to
-- the quote right after it (buyer transaction). A bound grant prices only the quote it is bound to (app.quote_id at RevalidateQuote).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION claims.bind_merchant_origin_grant(p_quote uuid) RETURNS integer
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
  WHERE g.tenant_id=v_tenant AND g.store_id=v_store AND g.buyer_id=v_buyer AND g.bundle_id=ANY(v_bundles)
   AND g.consumed_at IS NULL AND g.expires_at>clock_timestamp() AND (g.quote_id IS NULL OR g.quote_id=p_quote);
 GET DIAGNOSTICS n=ROW_COUNT;
 RETURN n;
END $$;
ALTER FUNCTION claims.bind_merchant_origin_grant(uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.bind_merchant_origin_grant(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.bind_merchant_origin_grant(uuid) TO commerce_buyer_runtime;
COMMENT ON FUNCTION claims.bind_merchant_origin_grant(uuid) IS
 'internal/claims (0129 LC-B6, called by internal/merchanttools ForBuyer right after CreateQuote on commerce_buyer_runtime, buyer fence as live_prices): binds the caller''s unconsumed, unexpired merchant-origin grants of every claim bundle that the caller''s own quote priced at the live price to that quote (a grant already bound to another quote is left alone); returns the number of grants now bound to it. Zero live lines = 0. No price, no read of another buyer''s data.';

-- ---------------------------------------------------------------------------------------
-- claims.live_prices (same signature/owner/volatility): + the merchant-origin grant branch
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
 -- that SKU and its REMAINING quantity (claimed minus held uses, R4S-01) covers the cart quantity (D2), the offer is active, belongs to the
 -- bundle's session and has a price.
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
  AND o.session_id=b.session_id AND o.sku_id=u.sku_id AND o.active AND o.live_price_minor IS NOT NULL;
END $$;

COMMENT ON FUNCTION claims.live_prices(uuid[],uuid[],uuid[],bigint[]) IS
 'internal/claims (SQL definer; its only caller is internal/storefront applyLivePrices, from Quote on commerce_buyer_runtime and from checkout.Begin''s RevalidateQuote on commerce_checkout_runtime); EXECUTE: those two roles. Returns the live price of each (bundle,offer,sku,cart quantity) origin that is EITHER bound to the caller with an unexpired link OR (0129) covered by an unconsumed merchant-origin grant of the caller bound to no quote (CreateQuote) or to the quote being revalidated (app.quote_id); in both cases a claim line whose quantity minus the uses held by non-CANCELLED orders covers the cart quantity and an active priced offer of the bundle''s session. Zero rows = catalog price.';

-- claims.consume_live_prices: + grant consumption
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
   (x->>'quantity')::bigint AS quantity
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
   PERFORM 1 FROM claims.merchant_origin_grants g WHERE g.tenant_id=v_tenant AND g.store_id=v_store AND g.buyer_id=v_buyer
    AND g.bundle_id=r.bundle_id AND g.quote_id=v_quote AND g.consumed_at IS NULL AND g.expires_at>clock_timestamp() FOR UPDATE;
   IF NOT FOUND THEN RAISE EXCEPTION 'live price no longer available' USING ERRCODE='PT409'; END IF;
   IF NOT (r.bundle_id=ANY(v_bundles)) THEN v_bundles:=v_bundles||r.bundle_id; END IF;
  END IF;
  SELECT coalesce(sum(u.quantity),0) INTO v_held FROM claims.live_price_uses u
   JOIN checkout.orders o ON o.tenant_id=u.tenant_id AND o.store_id=u.store_id AND o.id=u.order_id AND o.commercial_state<>'CANCELLED'
   WHERE u.tenant_id=v_tenant AND u.store_id=v_store AND u.bundle_id=r.bundle_id AND u.offer_id=r.offer_id;
  IF r.quantity IS NULL OR r.quantity<1 OR v_held+r.quantity>v_claimed THEN
   RAISE EXCEPTION 'live price no longer available' USING ERRCODE='PT409'; END IF;
  INSERT INTO claims.live_price_uses(tenant_id,store_id,bundle_id,offer_id,order_id,quantity)
   VALUES(v_tenant,v_store,r.bundle_id,r.offer_id,p_order,r.quantity);
  v_count:=v_count+1;
 END LOOP;
 IF cardinality(v_bundles)>0 THEN
  UPDATE claims.merchant_origin_grants g SET consumed_at=clock_timestamp() WHERE g.tenant_id=v_tenant AND g.store_id=v_store AND g.buyer_id=v_buyer
   AND g.bundle_id=ANY(v_bundles) AND g.quote_id=v_quote AND g.consumed_at IS NULL;
  GET DIAGNOSTICS v_n=ROW_COUNT;
  IF v_n<>cardinality(v_bundles) THEN RAISE EXCEPTION 'live price no longer available' USING ERRCODE='PT409'; END IF;
 END IF;
 RETURN v_count;
END $$;
ALTER FUNCTION claims.consume_live_prices(uuid) OWNER TO commerce_claims_writer;
COMMENT ON FUNCTION claims.consume_live_prices(uuid) IS
 'internal/claims (SQL definer; its only caller is internal/storefront ConsumeLivePrices from internal/checkout Begin, after checkout.begin_hold in the same transaction on commerce_checkout_runtime); EXECUTE: commerce_checkout_runtime. For every live_claim line of the order''s quote: locks the claim line FOR UPDATE (bundle bound to the caller, or 0129: a merchant-origin grant of the caller bound to this quote, locked and marked consumed), re-checks claimed minus held uses covers the line quantity, inserts the claims.live_price_uses row; returns the row count. PT409 (live price no longer available) rolls the whole placement back and the buyer re-quotes; PT404 when the order is not the caller''s fresh order. Non-goals: no price computation, no release (derived from order state).';

-- inbox.plan_dm (same signature/owner/ACL): semantic key of the for-buyer DM
CREATE OR REPLACE FUNCTION inbox.plan_dm(p_conversation uuid, p_expected_generation bigint, p_order uuid, p_operation uuid, p_job bigint,
 p_outbound uuid, p_body_hmac bytea, p_display_key_id text, p_display_nonce bytea, p_display_ciphertext bytea, p_secret_enc bytea,
 p_secret_sealed bytea, p_template_id text, p_template_version bigint)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_t uuid; v_s uuid; v_p uuid; c record; st record; b record; v_now timestamptz := clock_timestamp();
 v_provider text; v_hmac text; v_gen bigint; v_req jsonb; v_changed boolean := false;
BEGIN
    v := inbox.lcn_scope(); v_t := v[1]; v_s := v[2]; v_p := v[3];
    IF p_conversation IS NULL OR p_expected_generation IS NULL OR p_expected_generation < 0 OR p_body_hmac IS NULL
       OR octet_length(p_body_hmac) <> 32 OR (p_template_id IS NULL) <> (p_template_version IS NULL) THEN
        RAISE EXCEPTION 'invalid dm plan' USING ERRCODE = '22023';
    END IF;
    IF NOT identity.principal_holds(v_t, v_s, v_p, ARRAY['inbox:reply']) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    v_hmac := encode(p_body_hmac, 'hex');
    -- A1.3 clause 1: serialise same-body sends before reading for duplicates and before any insert.
    PERFORM pg_advisory_xact_lock(hashtextextended('lcn-dup|' || v_s::text || '|' || p_conversation::text || '|' || v_hmac, 0));
    IF EXISTS (SELECT 1 FROM integration.operations o WHERE o.tenant_id = v_t AND o.store_id = v_s AND o.action = 'meta.dm_send'
                AND o.request->>'conversation_id' = p_conversation::text AND o.request->>'body_hmac' = v_hmac
                AND o.created_at > v_now - interval '30 seconds') THEN
        RAISE EXCEPTION 'duplicate_recent' USING ERRCODE = 'PT409';
    END IF;
    PERFORM inbox.lcn_rate_check(v_t, v_s);
    SELECT x.id, x.object, x.asset_id, x.app_id, x.peer_key INTO c FROM social.conversations x
     WHERE x.id = p_conversation AND x.tenant_id = v_t AND x.store_id = v_s;
    IF NOT FOUND THEN RAISE EXCEPTION 'conversation_gone' USING ERRCODE = 'PT409'; END IF;
    SELECT s.mode, s.assignee_principal, s.takeover_generation, s.human_until, s.last_inbound_at INTO st FROM inbox.conversation_state s
     WHERE s.tenant_id = v_t AND s.store_id = v_s AND s.conversation_id = p_conversation FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'conversation_gone' USING ERRCODE = 'PT409'; END IF;
    -- §3.6 lazy takeover expiry, applied by this write.
    IF st.mode = 'human' AND st.human_until IS NOT NULL AND v_now >= st.human_until THEN
        st.mode := 'auto'; st.assignee_principal := NULL; st.takeover_generation := st.takeover_generation + 1; st.human_until := NULL;
    END IF;
    IF p_expected_generation <> st.takeover_generation THEN RAISE EXCEPTION 'takeover_changed' USING ERRCODE = 'PT409'; END IF;
    IF st.last_inbound_at IS NULL OR st.last_inbound_at + interval '24 hours' - interval '5 minutes' <= v_now THEN
        RAISE EXCEPTION 'window_closed' USING ERRCODE = 'PT409';
    END IF;
    v_provider := CASE c.object WHEN 'page' THEN 'facebook' ELSE 'instagram' END;
    SELECT z.id, z.semantic_version INTO b FROM integration.bindings z
     WHERE z.tenant_id = v_t AND z.store_id = v_s AND z.provider = v_provider AND z.external_asset_id = c.asset_id AND z.enabled
     ORDER BY z.id LIMIT 1 FOR SHARE;
    IF NOT FOUND OR coalesce(integration.binding_capability_state(v_t, v_s, b.id, 'dm_session', ARRAY[]::text[]), 'unknown')
                    NOT IN ('ok', 'review_required') THEN
        RAISE EXCEPTION 'capability' USING ERRCODE = 'PT409';
    END IF;
    -- §3.6 implicit takeover: only a real transition (auto -> human, or another assignee) bumps the generation.
    v_gen := st.takeover_generation;
    IF st.mode <> 'human' OR st.assignee_principal IS DISTINCT FROM v_p THEN v_gen := v_gen + 1; v_changed := true; END IF;
    UPDATE inbox.conversation_state s SET mode = 'human', assignee_principal = v_p, takeover_generation = v_gen,
           last_human_outbound_at = v_now, human_until = v_now + interval '6 hours', version = s.version + 1, updated_at = v_now
     WHERE s.tenant_id = v_t AND s.store_id = v_s AND s.conversation_id = p_conversation;
    IF v_changed THEN
        INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (v_t, v_s, v_p, 'inbox.takeover');
    END IF;
    v_req := jsonb_build_object('v', 1, 'kind', 'dm', 'message_type', 'dm', 'origin', 'human', 'platform', v_provider,
        'asset_id', c.asset_id, 'app_id', c.app_id, 'conversation_id', p_conversation, 'conversation_known', true,
        'peer_key', c.peer_key, 'outbound_id', p_outbound, 'body_hmac', v_hmac, 'policy', 'lcn-policy/v1',
        'takeover_generation', v_gen, 'principal_id', v_p,
        'deadline_at', to_char((st.last_inbound_at + interval '24 hours' - interval '5 minutes') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
    IF p_template_id IS NOT NULL THEN v_req := v_req || jsonb_build_object('template_id', p_template_id, 'template_version', p_template_version); END IF;
    IF p_order IS NOT NULL THEN v_req := v_req || jsonb_build_object('order_id', p_order); END IF;
    PERFORM inbox.lcn_emit(v_t, v_s, v_p, 'dm', 'meta.dm_send', b.id, b.semantic_version, v_provider, c.asset_id,
        -- 0129 (live-console-v1 §5.1 step 4): the for-buyer pay-link DM derives its key from (conversation, order), so no replay can plan two.
        'mdm:' || substr(encode(sha256(convert_to(p_conversation::text || '|' ||
            CASE WHEN p_order IS NULL THEN p_operation::text ELSE 'order-pay-link|' || p_order::text END, 'UTF8')), 'hex'), 1, 48),
        v_req, p_operation, p_job, p_outbound, p_conversation, NULL, p_body_hmac, p_display_key_id, p_display_nonce,
        p_display_ciphertext, p_secret_enc, p_secret_sealed, p_template_id, p_template_version, 'inbox.dm.planned');
    RETURN p_operation;
END $$;
-- (inbox.plan_dm keeps its owner, ACL and comment: CREATE OR REPLACE.)
