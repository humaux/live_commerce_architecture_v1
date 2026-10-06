-- Purpose: the two privileged reads behind the live-console read model A1 (live-console-v1 §7.1, unit LC-B7): per-session comment/buyer
--   counts plus the attributed (order, offer, sku) pairs (claims.console_session_facts), and the console sales read model
--   (identity.read_live_console_sales: orders/paid totals and per-offer ordered/paid quantity and amount).
-- Depends on: claims.meta_intake (0064), claims.events (0060/0064/0123 column grants), claims.bundles (0060), claims.live_price_uses (0105),
--   claims.order_origins (0113), live.offers (0060/0092), claims.session_orders (0118), live.order_session_labels (0110),
--   identity.resolve_access (0003), identity.principal_holds (0064), identity.merchant_access_denied (0063), payments.facts (0027),
--   payments.refund_facts / payments.stripe_refunds (0062), checkout.orders (0013/0027/0073/0107), checkout.payment_attempts (0016),
--   checkout.bank_transfers (0088), fulfillment.cvs_shipments (0073/0085).
-- Used by: internal/live/console.go (Console.Read), internal/httpapi/live_console.go (A1 GET .../live-sessions/{session_id}/console).
-- Invariants: I01 (scope from the authenticated transaction GUCs, never a request field), I05 (paid = LIVE-environment money only, net of
--   refunds, never summed across environments or currencies), I11 (no comment text, names, PSIDs: counts and ids only).
-- Status: REAL_PG (tests/foundation/live_console_read_test.go).
--
-- 0148 (LC-B7). Money口径 deliberately mirrors identity.read_live_session_results (0118): keep the cap/ref/pick/cod/trf CTEs in
-- lockstep with that function; TestLiveConsoleLCN16SalesAgreeWithResults asserts the two agree for the same session.
-- No new table or column grant: every table read below was already readable by the owning role of its definer.

-- ---------------------------------------------------------------------------------------
-- F1. claims.console_session_facts: domain-owned helper (owner commerce_claims_writer, EXECUTE commerce_auth only, like
-- claims.session_orders). keyword_comments = text-free intakes of the session with grammar_kind<>'NO_MATCH' plus manual claim events
-- that parsed as a keyword (reason NO_MATCH <=> grammar_kind NO_MATCH, 0060 CHECK; grammar_kind itself is not granted here);
-- buyers = bundles of the session; pairs = DISTINCT attributed (order, offer, sku) over live_price_uses UNION order_origins.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION claims.console_session_facts(p_tenant uuid,p_store uuid,p_session uuid)
RETURNS jsonb
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT jsonb_build_object(
  'keyword_comments',
   (SELECT count(*) FROM claims.meta_intake i WHERE i.tenant_id=p_tenant AND i.store_id=p_store AND i.session_id=p_session
     AND i.grammar_kind<>'NO_MATCH')
   +(SELECT count(*) FROM claims.events e WHERE e.tenant_id=p_tenant AND e.store_id=p_store AND e.session_id=p_session
     AND e.source_kind='manual' AND e.reason IS DISTINCT FROM 'NO_MATCH'),
  'buyers',(SELECT count(*) FROM claims.bundles b WHERE b.tenant_id=p_tenant AND b.store_id=p_store AND b.session_id=p_session),
  'pairs',coalesce((SELECT jsonb_agg(jsonb_build_object('order_id',x.order_id,'offer_id',x.offer_id,'sku_id',x.sku_id)) FROM (
    SELECT u.order_id,u.offer_id,o.sku_id
    FROM claims.live_price_uses u
    JOIN claims.bundles b ON b.tenant_id=u.tenant_id AND b.store_id=u.store_id AND b.id=u.bundle_id
    JOIN live.offers o ON o.tenant_id=u.tenant_id AND o.store_id=u.store_id AND o.id=u.offer_id
    WHERE u.tenant_id=p_tenant AND u.store_id=p_store AND b.session_id=p_session
    UNION
    SELECT g.order_id,g.offer_id,o.sku_id
    FROM claims.order_origins g
    JOIN live.offers o ON o.tenant_id=g.tenant_id AND o.store_id=g.store_id AND o.id=g.offer_id
    WHERE g.tenant_id=p_tenant AND g.store_id=p_store AND g.session_id=p_session
   ) x),'[]'::jsonb))
$$;
ALTER FUNCTION claims.console_session_facts(uuid,uuid,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.console_session_facts(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.console_session_facts(uuid,uuid,uuid) TO commerce_auth;
COMMENT ON FUNCTION claims.console_session_facts(uuid,uuid,uuid) IS
 'internal/claims (0148, LC-B7; SQL definer, only caller identity.read_live_console_sales on commerce_auth, behind internal/live Console); EXECUTE: commerce_auth. Per-session keyword_comments, buyers and the attributed (order,offer,sku) pairs (live_price_uses UNION order_origins). Counts and ids only: no comment text, names, actor keys or bundle owners.';

-- ---------------------------------------------------------------------------------------
-- F2. identity.read_live_console_sales: owner commerce_auth, EXECUTE commerce_runtime. Authenticated by live:read only (A1), unlike
-- read_live_session_results (orders:read + live:read), because the console shows the live-session sales to every live:read principal.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION identity.read_live_console_sales(p_hash bytea,p_store uuid,p_session uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog SET jit=off SET plan_cache_mode=force_custom_plan AS $$
DECLARE s record; v_result jsonb; v_facts jsonb; v_auth_error text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_session IS NULL
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid live console sales' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'live:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF NOT identity.principal_holds(s.tenant_id,p_store,s.principal_id,ARRAY['live:read']) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 -- I01: the session must be this store's (domain definer; commerce_auth has no live.sessions grant). Cross-store/tenant -> 404.
 IF (SELECT count(*) FROM live.order_session_labels(s.tenant_id,p_store,ARRAY[p_session]))<>1 THEN
  RAISE EXCEPTION 'session not found' USING ERRCODE='PT404'; END IF;
 v_facts:=claims.console_session_facts(s.tenant_id,p_store,p_session);
 WITH ord AS MATERIALIZED (
  SELECT so.order_id FROM claims.session_orders(s.tenant_id,p_store,ARRAY[p_session]) so
 ), cap AS MATERIALIZED (
  SELECT a.order_id,f.environment,sum(f.amount_minor)::bigint AS minor
  FROM payments.facts f JOIN checkout.payment_attempts a ON a.tenant_id=f.tenant_id AND a.store_id=f.store_id AND a.id=f.attempt_id
  WHERE f.tenant_id=s.tenant_id AND f.store_id=p_store AND f.kind='CAPTURED'
   AND a.order_id IN (SELECT order_id FROM ord)
  GROUP BY a.order_id,f.environment
 ), ref AS MATERIALIZED (
  SELECT a.order_id,a.environment,sum(r.amount_minor)::bigint AS minor
  FROM payments.refund_facts rf
  JOIN payments.stripe_refunds r ON r.tenant_id=rf.tenant_id AND r.store_id=rf.store_id AND r.id=rf.refund_id
  JOIN checkout.payment_attempts a ON a.tenant_id=r.tenant_id AND a.store_id=r.store_id AND a.id=r.attempt_id
  WHERE rf.tenant_id=s.tenant_id AND rf.store_id=p_store AND rf.kind='SUCCEEDED'
   AND a.order_id IN (SELECT order_id FROM ord)
   AND NOT EXISTS(SELECT 1 FROM payments.refund_facts later WHERE later.tenant_id=rf.tenant_id AND later.store_id=rf.store_id
     AND later.refund_id=rf.refund_id AND later.kind IN ('FAILED','CANCELED') AND later.received_at>rf.received_at)
  GROUP BY a.order_id,a.environment
 ), pick AS MATERIALIZED (
  SELECT o.id AS order_id,coalesce(c.environment,'LIVE') AS environment,sum(o.total_minor)::bigint AS minor
  FROM checkout.orders o
  LEFT JOIN LATERAL (SELECT x.environment FROM fulfillment.cvs_shipments x
   WHERE x.tenant_id=o.tenant_id AND x.store_id=o.store_id AND x.order_id=o.id ORDER BY x.attempt DESC LIMIT 1) c ON true
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.payment_mode='pay_at_pickup' AND o.collection_state='COLLECTED'
   AND o.id IN (SELECT order_id FROM ord)
  GROUP BY o.id,c.environment
 ), cod AS MATERIALIZED (
  SELECT o.id AS order_id,'LIVE'::text AS environment,sum(o.total_minor+coalesce(o.cod_surcharge_minor,0))::bigint AS minor
  FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.payment_mode='cash_on_delivery' AND o.collection_state='COLLECTED'
   AND o.id IN (SELECT order_id FROM ord)
  GROUP BY o.id
 ), trf AS MATERIALIZED (
  SELECT t.order_id,'LIVE'::text AS environment,sum(t.confirmed_amount_minor)::bigint AS minor
  FROM checkout.bank_transfers t
  WHERE t.tenant_id=s.tenant_id AND t.store_id=p_store AND t.state='CONFIRMED'
   AND t.order_id IN (SELECT order_id FROM ord)
  GROUP BY t.order_id
 ), paid AS MATERIALIZED (
  SELECT x.order_id,x.environment,sum(x.minor)::bigint AS minor FROM (
   SELECT ce.order_id,ce.environment,coalesce(c.minor,0)-coalesce(r.minor,0) AS minor
   FROM (SELECT order_id,environment FROM cap UNION SELECT order_id,environment FROM ref) ce
   LEFT JOIN cap c ON c.order_id=ce.order_id AND c.environment=ce.environment
   LEFT JOIN ref r ON r.order_id=ce.order_id AND r.environment=ce.environment
   UNION ALL SELECT order_id,environment,minor FROM pick
   UNION ALL SELECT order_id,environment,minor FROM cod
   UNION ALL SELECT order_id,environment,minor FROM trf
  ) x GROUP BY x.order_id,x.environment
 ), ord_paid AS MATERIALIZED (
  SELECT o.id AS order_id,o.currency,o.commercial_state,o.total_minor+coalesce(o.cod_surcharge_minor,0) AS amount_minor,
   coalesce(pl.minor,0) AS live_minor,
   CASE WHEN jsonb_typeof(o.snapshot#>'{quote,lines}')='array' THEN o.snapshot#>'{quote,lines}' ELSE '[]'::jsonb END AS lines
  FROM checkout.orders o
  LEFT JOIN paid pl ON pl.order_id=o.id AND pl.environment='LIVE'
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id IN (SELECT order_id FROM ord)
 ), cur AS MATERIALIZED (
  -- One currency per console (the UI shows one amount): the one with the most non-cancelled orders, ties by code.
  SELECT op.currency FROM ord_paid op WHERE op.commercial_state<>'CANCELLED'
  GROUP BY op.currency ORDER BY count(*) DESC,op.currency LIMIT 1
 ), live_orders AS MATERIALIZED (
  SELECT op.* FROM ord_paid op JOIN cur ON cur.currency=op.currency WHERE op.commercial_state<>'CANCELLED'
 ), per_offer AS MATERIALIZED (
  SELECT p.offer_id,
   coalesce(sum(l.qty),0)::bigint AS ordered_qty,
   coalesce(sum(l.qty) FILTER (WHERE lo.live_minor>0),0)::bigint AS paid_qty,
   coalesce(sum(l.amount) FILTER (WHERE lo.live_minor>0),0)::bigint AS paid_amount_minor
  FROM jsonb_to_recordset(v_facts->'pairs') AS p(order_id uuid,offer_id uuid,sku_id uuid)
  JOIN live_orders lo ON lo.order_id=p.order_id
  CROSS JOIN LATERAL (
   SELECT (li->>'quantity')::bigint AS qty,coalesce((li#>>'{amount,total_minor}')::bigint,0) AS amount
   FROM jsonb_array_elements(lo.lines) li
   WHERE li->>'sku_id'=p.sku_id::text AND li->>'quantity' ~ '^[0-9]{1,12}$'
    AND coalesce(li#>>'{amount,total_minor}','0') ~ '^[0-9]{1,15}$') l
  GROUP BY p.offer_id
 )
 SELECT jsonb_build_object('as_of',clock_timestamp(),
  'currency',(SELECT currency FROM cur),
  'keyword_comments',(v_facts->>'keyword_comments')::bigint,
  'buyers',(v_facts->>'buyers')::bigint,
  'orders',jsonb_build_object('count',(SELECT count(*) FROM live_orders),
    'amount_minor',(SELECT coalesce(sum(amount_minor),0)::bigint FROM live_orders)),
  'paid',jsonb_build_object('count',(SELECT count(*) FROM live_orders WHERE live_minor>0),
    'amount_minor',(SELECT coalesce(sum(live_minor),0)::bigint FROM live_orders WHERE live_minor>0)),
  'offers',coalesce((SELECT jsonb_agg(jsonb_build_object('offer_id',po.offer_id,'ordered_qty',po.ordered_qty,
    'paid_qty',po.paid_qty,'paid_amount_minor',po.paid_amount_minor) ORDER BY po.offer_id) FROM per_offer po),'[]'::jsonb))
 INTO v_result;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['live:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'live console sales access denied' USING ERRCODE=v_auth_error; END IF;
 IF octet_length(v_result::text)>1048576 THEN RAISE EXCEPTION 'live console sales unavailable' USING ERRCODE='PT503'; END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION identity.read_live_console_sales(bytea,uuid,uuid) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_live_console_sales(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_live_console_sales(bytea,uuid,uuid) TO commerce_runtime;
COMMENT ON FUNCTION identity.read_live_console_sales(bytea,uuid,uuid) IS
 'internal/live Console (0148, LC-B7, A1): fresh live:read authentication, then for ONE session of the store: keyword_comments, buyers, the dominant-currency non-CANCELLED order count and amount, LIVE-environment paid count and amount (net of refunds, I05; SANDBOX never counted) and per-offer ordered/paid quantity and amount over the attributed (order,offer,sku) pairs. Money口径 mirrors identity.read_live_session_results (0118). EXECUTE: commerce_runtime.';
