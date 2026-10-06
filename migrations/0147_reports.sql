-- 0147_reports.sql — W6-02B: read-only merchant reports (products, channels, funnel, manual orders) and one audited export.
-- Purpose: the four report definers (identity.read_report_products / _channels / _funnel / _manual_orders) and identity.export_report
--   (orders:export + audit row reports.export.<report>). Pure read model: no table, no column, no role, no cache, no materialized view; every
--   number is computed per request over at most 92 Asia/Taipei days (contracts/reporting-v2.md).
-- Depends on: identity.resolve_access (0003), identity.merchant_access_denied (0063), identity.principal_holds (0064),
--   payments.facts / payments.refund_facts / payments.stripe_refunds (0018/0062), checkout.orders (0013, snapshot + source 0094 + COD 0107),
--   checkout.payment_attempts (0016), checkout.bank_transfers (0088), fulfillment.cvs_shipments (0073), claims.bundles (0060/0078 commerce_auth read),
--   claims.order_origins (0113), claims.live_price_uses (0105), claims.events (0060), integration.operations (0008, meta.private_reply),
--   ops.command_results (merchanttools.order.manual receipt, 0094), ops.audit_events (0001), live.order_session_labels (0110).
-- Used by: internal/reporting (products.go, channels.go, funnel.go, manual.go), internal/httpapi/reports.go.
-- Invariants: I05 (money = payment facts / frozen order snapshot, per currency and environment, never recomputed from current prices),
--   I01 (store scope from the resolved access + GUCs, never a client id), finance parity (the money rules are identity.read_finance_summary's, 0107).
-- Status: REAL_PG (MOCK data).
--
-- ACL ruling (0110/0118): the reporting reads run as commerce_auth (no new table or column grant) and reach claims / checkout-private data only
-- through three domain-owned definers whose EXECUTE goes to commerce_auth alone. identity.report_open and identity.report_money_events are
-- internal helpers (no SECURITY DEFINER, no grant to any other role): they run only inside the definers below.
-- Deployment environment (LC-B7 A1): counts (funnel paid, channel / manual-order orders and cancelled_orders) follow p_environment, the deployment payment
--   environment passed by the Go caller (LIVE on a LIVE deployment, else SANDBOX): an order is in an environment by its payment attempt, or by the deployment when it has
--   none (offline modes, unpaid). Money stays split per (currency, environment).
-- Indexes (no new table or column): claims.order_origins(tenant_id,store_id,bundle_id), claims.events(tenant_id,store_id,occurred_at).
-- Deviation from the brief: one dispatcher identity.export_report(report name) instead of four export_report_* twins; it re-runs the matching
-- read definer and writes the single audit row, so the export can never diverge from the read.

-- ---------------------------------------------------------------------------------------
-- Domain definers (claims: owner commerce_claims_writer; checkout: owner commerce_checkout_writer)
-- ---------------------------------------------------------------------------------------
-- The funnel probes origins by bundle and claim events by day: without these two the funnel scans every origin / event of the table.
CREATE INDEX order_origins_by_bundle ON claims.order_origins(tenant_id,store_id,bundle_id);
CREATE INDEX events_by_occurred ON claims.events(tenant_id,store_id,occurred_at);
-- Distinct (order, bundle, session) provenance of the requested orders: consumed order origins UNION live-price uses (same union as 0118
-- claims.session_orders, but keeping the bundle so the caller can read the bundle platform).
CREATE FUNCTION claims.report_order_bundles(p_tenant uuid,p_store uuid,p_orders uuid[])
RETURNS TABLE(order_id uuid,bundle_id uuid,session_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT x.order_id,x.bundle_id,x.session_id FROM (
  SELECT o.order_id,o.bundle_id,o.session_id FROM claims.order_origins o
   WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.order_id=ANY(p_orders)
  UNION
  SELECT u.order_id,u.bundle_id,b.session_id
   FROM claims.live_price_uses u JOIN claims.bundles b ON b.tenant_id=u.tenant_id AND b.store_id=u.store_id AND b.id=u.bundle_id
   WHERE u.tenant_id=p_tenant AND u.store_id=p_store AND u.order_id=ANY(p_orders) AND b.tenant_id=p_tenant AND b.store_id=p_store
 ) x
$$;
ALTER FUNCTION claims.report_order_bundles(uuid,uuid,uuid[]) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.report_order_bundles(uuid,uuid,uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.report_order_bundles(uuid,uuid,uuid[]) TO commerce_auth;
COMMENT ON FUNCTION claims.report_order_bundles(uuid,uuid,uuid[]) IS
 'internal/claims (SQL definer for internal/reporting; only caller: identity.read_report_channels / read_report_manual_orders on commerce_auth); EXECUTE commerce_auth. Distinct (order, bundle, session) of the requested orders from order_origins UNION live_price_uses; no buyer identity, no comment text.';

-- One row per bundle with an ACCEPTED claim event in [t0,t1): was a private-reply link SUCCEEDED for it, and which orders came out of it.
-- Only meta.private_reply operations are readable by this owner (policy claim_reply_operation_read, 0064); meta.dm_send links are not counted. Operations are read only for created_at in [t0, t1 + 7 days).
CREATE FUNCTION claims.report_funnel_bundles(p_tenant uuid,p_store uuid,p_t0 timestamptz,p_t1 timestamptz,p_session uuid)
RETURNS TABLE(bundle_id uuid,link_sent boolean,order_ids uuid[])
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 WITH b AS MATERIALIZED (
  SELECT DISTINCT e.bundle_id FROM claims.events e
  WHERE e.tenant_id=p_tenant AND e.store_id=p_store AND e.outcome='ACCEPTED' AND e.occurred_at>=p_t0 AND e.occurred_at<p_t1
   AND (p_session IS NULL OR e.session_id=p_session)
 ), snt AS MATERIALIZED (
  SELECT DISTINCT o.request->>'bundle_id' AS bundle_id FROM integration.operations o
  WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.action='meta.private_reply' AND o.state='SUCCEEDED'
   -- a link is sent right after the claim (Meta's reply window is 7 days): bound the scan to the report window plus that window
   AND o.created_at>=p_t0 AND o.created_at<p_t1+interval '7 days'
   AND o.request->>'bundle_id' IN (SELECT x.bundle_id::text FROM b x)
 ), ord AS MATERIALIZED (
  SELECT y.bundle_id,array_agg(DISTINCT y.order_id) AS order_ids FROM (
   SELECT oo.bundle_id,oo.order_id FROM claims.order_origins oo
    WHERE oo.tenant_id=p_tenant AND oo.store_id=p_store AND oo.bundle_id IN (SELECT x.bundle_id FROM b x)
   UNION
   SELECT u.bundle_id,u.order_id FROM claims.live_price_uses u
    WHERE u.tenant_id=p_tenant AND u.store_id=p_store AND u.bundle_id IN (SELECT x.bundle_id FROM b x)
  ) y GROUP BY y.bundle_id
 )
 SELECT b.bundle_id,EXISTS(SELECT 1 FROM snt WHERE snt.bundle_id=b.bundle_id::text),coalesce(ord.order_ids,'{}'::uuid[])
 FROM b LEFT JOIN ord ON ord.bundle_id=b.bundle_id
$$;
ALTER FUNCTION claims.report_funnel_bundles(uuid,uuid,timestamptz,timestamptz,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.report_funnel_bundles(uuid,uuid,timestamptz,timestamptz,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.report_funnel_bundles(uuid,uuid,timestamptz,timestamptz,uuid) TO commerce_auth;
COMMENT ON FUNCTION claims.report_funnel_bundles(uuid,uuid,timestamptz,timestamptz,uuid) IS
 'internal/claims (SQL definer for internal/reporting; only caller: identity.read_report_funnel on commerce_auth); EXECUTE commerce_auth. Per claimed bundle (ACCEPTED event in range, optional session): link_sent = a SUCCEEDED meta.private_reply operation names the bundle; order_ids = orders from order_origins UNION live_price_uses. No actor key, no comment text.';

-- Merchant-created orders (checkout.orders.source=merchant_manual, 0094) created in [t0,t1) with the staff principal that created them
-- (the merchanttools.order.manual receipt of ops.command_results; NULL when the receipt is missing).
CREATE FUNCTION checkout.report_manual_creators(p_tenant uuid,p_store uuid,p_t0 timestamptz,p_t1 timestamptz)
RETURNS TABLE(order_id uuid,principal_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 WITH r AS MATERIALIZED (
  SELECT DISTINCT ON (c.response->>'order_id') c.response->>'order_id' AS order_id,c.principal_id
  FROM ops.command_results c
  WHERE c.tenant_id=p_tenant AND c.store_id=p_store AND c.operation='merchanttools.order.manual'
  ORDER BY c.response->>'order_id',c.created_at
 )
 SELECT o.id,r.principal_id FROM checkout.orders o LEFT JOIN r ON r.order_id=o.id::text
 WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.source='merchant_manual' AND o.created_at>=p_t0 AND o.created_at<p_t1
$$;
ALTER FUNCTION checkout.report_manual_creators(uuid,uuid,timestamptz,timestamptz) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.report_manual_creators(uuid,uuid,timestamptz,timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.report_manual_creators(uuid,uuid,timestamptz,timestamptz) TO commerce_auth;
COMMENT ON FUNCTION checkout.report_manual_creators(uuid,uuid,timestamptz,timestamptz) IS
 'internal/reporting (only caller: identity.read_report_manual_orders on commerce_auth); EXECUTE commerce_auth. Merchant-created orders of the store created in range and the staff principal of their merchanttools.order.manual receipt; no buyer data, no receipt body.';

-- ---------------------------------------------------------------------------------------
-- Internal helpers (owner commerce_auth, invoker rights, no EXECUTE for anyone else)
-- ---------------------------------------------------------------------------------------
-- Scope + range gate shared by every report: range 0..91 days, READ COMMITTED, resolve_access(permission), GUCs equal the resolved scope.
-- p_environment (NULL for the product report, which has no count) must be SANDBOX or LIVE. Returns the tenant, principal, authz revision and the [t0,t1) instants of the Asia/Taipei days.
CREATE FUNCTION identity.report_open(p_hash bytea,p_store uuid,p_from date,p_to date,p_permission text,p_environment text)
RETURNS TABLE(tenant_id uuid,principal_id uuid,revision bigint,t0 timestamptz,t1 timestamptz)
LANGUAGE plpgsql VOLATILE SET search_path=pg_catalog AS $$
DECLARE s record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_from IS NULL OR p_to IS NULL
  OR NOT isfinite(p_from) OR NOT isfinite(p_to) OR p_to-p_from NOT BETWEEN 0 AND 91
  OR (p_environment IS NOT NULL AND p_environment NOT IN ('SANDBOX','LIVE'))
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid report read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,p_permission);
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN QUERY SELECT s.tenant_id,s.principal_id,s.authz_revision,
  (p_from::timestamp AT TIME ZONE 'Asia/Taipei'),((p_to+1)::timestamp AT TIME ZONE 'Asia/Taipei');
END $$;
ALTER FUNCTION identity.report_open(bytea,uuid,date,date,text,text) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.report_open(bytea,uuid,date,date,text,text) FROM PUBLIC;
COMMENT ON FUNCTION identity.report_open(bytea,uuid,date,date,text,text) IS
 'internal/reporting helper, no EXECUTE for any role but its owner commerce_auth; called only inside the report definers. Range 0..91 days, READ COMMITTED, resolve_access(permission), GUC scope check; returns the Asia/Taipei day bounds.';

-- Money events of the store inside [t0,t1): the exact rows identity.read_finance_summary (0107) sums, one row per fact, keyed to the order.
-- kind: captured (CAPTURED fact, card), refunded (SUCCEEDED refund fact not later FAILED/CANCELED), offline (COD / pay-at-pickup COLLECTED on
-- collected_at, bank transfer CONFIRMED on confirmed_at). Offline money is never part of captured or net (OP3 / finance口径).
CREATE FUNCTION identity.report_money_events(p_tenant uuid,p_store uuid,p_t0 timestamptz,p_t1 timestamptz)
RETURNS TABLE(order_id uuid,paid_day date,currency text,environment text,kind text,minor bigint)
LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
 SELECT a.order_id,(f.received_at AT TIME ZONE 'Asia/Taipei')::date,f.currency,f.environment,'captured'::text,f.amount_minor::bigint
 FROM payments.facts f
 JOIN checkout.payment_attempts a ON a.tenant_id=f.tenant_id AND a.store_id=f.store_id AND a.id=f.attempt_id
 WHERE f.tenant_id=p_tenant AND f.store_id=p_store AND f.kind='CAPTURED' AND f.received_at>=p_t0 AND f.received_at<p_t1
 UNION ALL
 SELECT a.order_id,(rf.received_at AT TIME ZONE 'Asia/Taipei')::date,r.currency,a.environment,'refunded'::text,r.amount_minor::bigint
 FROM payments.refund_facts rf
 JOIN payments.stripe_refunds r ON r.tenant_id=rf.tenant_id AND r.store_id=rf.store_id AND r.id=rf.refund_id
 JOIN checkout.payment_attempts a ON a.tenant_id=r.tenant_id AND a.store_id=r.store_id AND a.id=r.attempt_id
 WHERE rf.tenant_id=p_tenant AND rf.store_id=p_store AND rf.kind='SUCCEEDED' AND rf.received_at>=p_t0 AND rf.received_at<p_t1
  AND NOT EXISTS(SELECT 1 FROM payments.refund_facts later WHERE later.tenant_id=rf.tenant_id AND later.store_id=rf.store_id
   AND later.refund_id=rf.refund_id AND later.kind IN ('FAILED','CANCELED') AND later.received_at>rf.received_at)
 UNION ALL
 SELECT o.id,(o.collected_at AT TIME ZONE 'Asia/Taipei')::date,o.currency,
  coalesce((SELECT c.environment FROM fulfillment.cvs_shipments c WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id
   AND c.order_id=o.id ORDER BY c.attempt DESC LIMIT 1),'LIVE'),'offline'::text,o.total_minor::bigint
 FROM checkout.orders o
 WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.payment_mode='pay_at_pickup' AND o.collection_state='COLLECTED'
  AND o.collected_at>=p_t0 AND o.collected_at<p_t1
 UNION ALL
 SELECT o.id,(o.collected_at AT TIME ZONE 'Asia/Taipei')::date,o.currency,'LIVE'::text,'offline'::text,(o.total_minor+coalesce(o.cod_surcharge_minor,0))::bigint
 FROM checkout.orders o
 WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.payment_mode='cash_on_delivery' AND o.collection_state='COLLECTED'
  AND o.collected_at>=p_t0 AND o.collected_at<p_t1
 UNION ALL
 SELECT t.order_id,(t.confirmed_at AT TIME ZONE 'Asia/Taipei')::date,t.currency,'LIVE'::text,'offline'::text,t.confirmed_amount_minor::bigint
 FROM checkout.bank_transfers t
 WHERE t.tenant_id=p_tenant AND t.store_id=p_store AND t.state='CONFIRMED' AND t.confirmed_at>=p_t0 AND t.confirmed_at<p_t1
$$;
ALTER FUNCTION identity.report_money_events(uuid,uuid,timestamptz,timestamptz) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.report_money_events(uuid,uuid,timestamptz,timestamptz) FROM PUBLIC;
COMMENT ON FUNCTION identity.report_money_events(uuid,uuid,timestamptz,timestamptz) IS
 'internal/reporting helper, no EXECUTE for any role but its owner commerce_auth; called only inside the report definers. The finance rows of read_finance_summary (0107) at fact granularity: captured / refunded / offline, with the order id.';

-- ---------------------------------------------------------------------------------------
-- Report 1: products. Per (SKU, currency, environment): every money event is spread over the order's FROZEN snapshot lines
-- (quote.lines[].amount.total_minor as weight, largest remainder so the parts sum exactly to the fact), never over current prices.
-- Sum over all rows = finance captured / refunded / net of the same range (RP01). At most 1000 rows, net descending.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION identity.read_report_products(p_hash bytea,p_store uuid,p_from date,p_to date) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog SET jit=off SET plan_cache_mode=force_custom_plan AS $$
DECLARE s record; v_rows jsonb; v_trunc boolean; v_result jsonb; v_auth_error text;
BEGIN
 SELECT * INTO s FROM identity.report_open(p_hash,p_store,p_from,p_to,'orders:read',NULL);
 WITH ev AS MATERIALIZED (
  SELECT row_number() OVER () AS eid,m.order_id,m.currency,m.environment,m.kind,m.minor
  FROM identity.report_money_events(s.tenant_id,p_store,s.t0,s.t1) m
 ), ln AS MATERIALIZED (
  SELECT l.order_id,l.created_at,x.idx,x.line->>'sku_id' AS sku_id,x.line->>'product_id' AS product_id,x.line->>'code' AS code,
   x.line->>'name' AS name,(x.line->>'quantity')::bigint AS qty,(x.line#>>'{amount,total_minor}')::bigint AS total
  FROM (SELECT o.id AS order_id,o.created_at,o.snapshot#>'{quote,lines}' AS lines FROM checkout.orders o
   WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id IN (SELECT e.order_id FROM ev e)) l
  CROSS JOIN LATERAL jsonb_array_elements(l.lines) WITH ORDINALITY AS x(line,idx)
 ), w AS MATERIALIZED (
  -- weight = frozen line total; an all-free order falls back to the quantities so the split is still defined.
  SELECT q.*,sum(CASE WHEN q.osum=0 THEN q.qty ELSE q.total END) OVER (PARTITION BY q.order_id) AS wsum,
   CASE WHEN q.osum=0 THEN q.qty ELSE q.total END AS wt
  FROM (SELECT ln.*,sum(ln.total) OVER (PARTITION BY ln.order_id) AS osum FROM ln) q
 ), alloc AS (
  SELECT e.eid,e.kind,e.currency,e.environment,e.minor,w.order_id,w.idx,w.sku_id,w.product_id,w.code,w.name,w.qty,w.created_at,
   floor(e.minor::numeric*w.wt/w.wsum) AS base,
   (e.minor::numeric*w.wt/w.wsum)-floor(e.minor::numeric*w.wt/w.wsum) AS frac
  FROM ev e JOIN w ON w.order_id=e.order_id
 ), ranked AS (
  SELECT a.*,a.minor-sum(a.base) OVER (PARTITION BY a.eid) AS rem,
   row_number() OVER (PARTITION BY a.eid ORDER BY a.frac DESC,a.idx) AS rn FROM alloc a
 ), fin AS (
  SELECT r.*,(r.base+CASE WHEN r.rn<=r.rem THEN 1 ELSE 0 END)::bigint AS share FROM ranked r
 ), per_sku AS (
  SELECT f.sku_id,f.currency,f.environment,
   coalesce(sum(f.share) FILTER (WHERE f.kind='captured'),0)::bigint AS captured_minor,
   coalesce(sum(f.share) FILTER (WHERE f.kind='refunded'),0)::bigint AS refunded_minor,
   coalesce(sum(f.share) FILTER (WHERE f.kind='offline'),0)::bigint AS offline_minor,
   (array_agg(f.product_id ORDER BY f.created_at DESC))[1] AS product_id,
   (array_agg(f.code ORDER BY f.created_at DESC))[1] AS code,
   (array_agg(f.name ORDER BY f.created_at DESC))[1] AS name
  FROM fin f GROUP BY f.sku_id,f.currency,f.environment
 ), units AS (
  SELECT d.sku_id,d.currency,d.environment,
   coalesce(sum(d.qty) FILTER (WHERE d.kind='captured'),0)::bigint AS units,
   coalesce(sum(d.qty) FILTER (WHERE d.kind='offline'),0)::bigint AS offline_units
  FROM (SELECT DISTINCT f.order_id,f.idx,f.sku_id,f.currency,f.environment,f.kind,f.qty FROM fin f WHERE f.kind IN ('captured','offline')) d
  GROUP BY d.sku_id,d.currency,d.environment
 ), rr AS (
  SELECT p.*,coalesce(u.units,0) AS units,coalesce(u.offline_units,0) AS offline_units,p.captured_minor-p.refunded_minor AS net_minor,
   row_number() OVER (ORDER BY p.captured_minor-p.refunded_minor DESC,p.sku_id,p.currency,p.environment) AS rk
  FROM per_sku p LEFT JOIN units u ON u.sku_id=p.sku_id AND u.currency=p.currency AND u.environment=p.environment
 )
 SELECT coalesce(jsonb_agg(jsonb_build_object('sku_id',rr.sku_id,'product_id',rr.product_id,'code',rr.code,'name',rr.name,
   'currency',rr.currency,'environment',rr.environment,'units',rr.units,'captured_minor',rr.captured_minor,
   'refunded_minor',rr.refunded_minor,'net_minor',rr.net_minor,'offline_units',rr.offline_units,'offline_minor',rr.offline_minor)
   ORDER BY rr.rk) FILTER (WHERE rr.rk<=1000),'[]'::jsonb),coalesce(bool_or(rr.rk>1000),false)
 INTO v_rows,v_trunc FROM rr;
 v_result:=jsonb_build_object('from',p_from,'to',p_to,'timezone','Asia/Taipei','truncated',v_trunc,'rows',v_rows);
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'product report access denied' USING ERRCODE=v_auth_error; END IF;
 IF octet_length(v_result::text)>1048576 THEN RAISE EXCEPTION 'product report unavailable' USING ERRCODE='PT503'; END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION identity.read_report_products(bytea,uuid,date,date) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_report_products(bytea,uuid,date,date) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_report_products(bytea,uuid,date,date) TO commerce_runtime;
COMMENT ON FUNCTION identity.read_report_products(bytea,uuid,date,date) IS
 'internal/reporting only; EXECUTE commerce_runtime. orders:read; per SKU/currency/environment units, captured, refunded, net and offline money from payment facts spread over the frozen order snapshot lines (largest remainder); <=1000 rows, net descending; sums equal read_finance_summary for the range.';

-- ---------------------------------------------------------------------------------------
-- Reports 2 and 4 share one shape: buckets of (key, currency) with order counts and per-environment money.
-- Channel of an order (exactly one): a facebook bundle origin -> facebook_live; else instagram -> instagram_live; else source
-- merchant_manual -> manual; else storefront. Orders = non-CANCELLED orders created in the range; money = facts dated in the range.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION identity.read_report_channels(p_hash bytea,p_store uuid,p_from date,p_to date,p_environment text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog SET jit=off SET plan_cache_mode=force_custom_plan AS $$
DECLARE s record; v_rows jsonb; v_result jsonb; v_auth_error text;
BEGIN
 IF p_environment IS NULL THEN RAISE EXCEPTION 'invalid report read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.report_open(p_hash,p_store,p_from,p_to,'orders:read',p_environment);
 WITH ev AS MATERIALIZED (
  SELECT m.order_id,m.currency,m.environment,m.kind,m.minor FROM identity.report_money_events(s.tenant_id,p_store,s.t0,s.t1) m
 ), ord AS MATERIALIZED (
  SELECT o.id,o.currency,o.commercial_state,o.source,(o.created_at>=s.t0 AND o.created_at<s.t1) AS created_in,
   coalesce((SELECT min(a.environment) FROM checkout.payment_attempts a WHERE a.tenant_id=o.tenant_id AND a.store_id=o.store_id AND a.order_id=o.id),p_environment) AS env
  FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store
   AND ((o.created_at>=s.t0 AND o.created_at<s.t1) OR o.id IN (SELECT e.order_id FROM ev e))
 ), bun AS MATERIALIZED (
  SELECT rb.order_id,bool_or(b.platform='facebook') AS fb,bool_or(b.platform='instagram') AS ig
  FROM claims.report_order_bundles(s.tenant_id,p_store,ARRAY(SELECT x.id FROM ord x)) rb
  JOIN claims.bundles b ON b.tenant_id=s.tenant_id AND b.store_id=p_store AND b.id=rb.bundle_id
  GROUP BY rb.order_id
 ), cls AS MATERIALIZED (
  SELECT o.id,o.currency,o.commercial_state,o.created_in,o.env,
   CASE WHEN bun.fb THEN 'facebook_live' WHEN bun.ig THEN 'instagram_live' WHEN o.source='merchant_manual' THEN 'manual' ELSE 'storefront' END AS channel
  FROM ord o LEFT JOIN bun ON bun.order_id=o.id
 ), cnt AS (
  SELECT c.channel,c.currency,count(*) FILTER (WHERE c.created_in AND c.env=p_environment AND c.commercial_state<>'CANCELLED') AS orders,
   count(*) FILTER (WHERE c.created_in AND c.env=p_environment AND c.commercial_state='CANCELLED') AS cancelled
  FROM cls c GROUP BY c.channel,c.currency
 ), mny AS (
  SELECT c.channel,e.currency,e.environment,
   count(*) FILTER (WHERE e.kind='captured') AS captured_count,
   coalesce(sum(e.minor) FILTER (WHERE e.kind='captured'),0)::bigint AS captured_minor,
   coalesce(sum(e.minor) FILTER (WHERE e.kind='refunded'),0)::bigint AS refunded_minor,
   count(*) FILTER (WHERE e.kind='offline') AS offline_count,
   coalesce(sum(e.minor) FILTER (WHERE e.kind='offline'),0)::bigint AS offline_minor
  FROM ev e JOIN cls c ON c.id=e.order_id GROUP BY c.channel,e.currency,e.environment
 ), keys AS (
  SELECT channel,currency FROM cnt UNION SELECT channel,currency FROM mny
 )
 SELECT coalesce(jsonb_agg(jsonb_build_object('channel',k.channel,'currency',k.currency,
   'orders',coalesce(n.orders,0),'cancelled_orders',coalesce(n.cancelled,0),
   'money',coalesce((SELECT jsonb_agg(jsonb_build_object('environment',m.environment,'captured_count',m.captured_count,
      'captured_minor',m.captured_minor,'refunded_minor',m.refunded_minor,'net_minor',m.captured_minor-m.refunded_minor,
      'offline_count',m.offline_count,'offline_minor',m.offline_minor) ORDER BY m.environment)
     FROM mny m WHERE m.channel=k.channel AND m.currency=k.currency),'[]'::jsonb))
   ORDER BY array_position(ARRAY['facebook_live','instagram_live','storefront','manual'],k.channel),k.currency),'[]'::jsonb)
 INTO v_rows FROM keys k LEFT JOIN cnt n ON n.channel=k.channel AND n.currency=k.currency;
 v_result:=jsonb_build_object('from',p_from,'to',p_to,'timezone','Asia/Taipei','rows',v_rows);
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'channel report access denied' USING ERRCODE=v_auth_error; END IF;
 IF octet_length(v_result::text)>1048576 THEN RAISE EXCEPTION 'channel report unavailable' USING ERRCODE='PT503'; END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION identity.read_report_channels(bytea,uuid,date,date,text) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_report_channels(bytea,uuid,date,date,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_report_channels(bytea,uuid,date,date,text) TO commerce_runtime;
COMMENT ON FUNCTION identity.read_report_channels(bytea,uuid,date,date,text) IS
 'internal/reporting only; EXECUTE commerce_runtime. orders:read; p_environment = the deployment payment environment; per channel (facebook_live, instagram_live, storefront, manual) and currency: non-cancelled orders of that environment created in range plus per-environment captured/refunded/net and offline money of facts dated in range; every order is in exactly one channel; sums equal read_finance_summary.';

CREATE FUNCTION identity.read_report_manual_orders(p_hash bytea,p_store uuid,p_from date,p_to date,p_environment text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog SET jit=off SET plan_cache_mode=force_custom_plan AS $$
DECLARE s record; v_rows jsonb; v_result jsonb; v_auth_error text;
BEGIN
 IF p_environment IS NULL THEN RAISE EXCEPTION 'invalid report read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.report_open(p_hash,p_store,p_from,p_to,'orders:read',p_environment);
 WITH mo AS MATERIALIZED (
  SELECT c.order_id,c.principal_id FROM checkout.report_manual_creators(s.tenant_id,p_store,s.t0,s.t1) c
 ), ses AS MATERIALIZED (
  -- an order that came from several sessions is reported under the lowest session id (one order, one row)
  SELECT rb.order_id,(array_agg(rb.session_id ORDER BY rb.session_id))[1] AS session_id
  FROM claims.report_order_bundles(s.tenant_id,p_store,ARRAY(SELECT x.order_id FROM mo x)) rb GROUP BY rb.order_id
 ), base AS MATERIALIZED (
  SELECT mo.principal_id,ses.session_id,o.currency,o.id AS order_id,o.commercial_state,
   coalesce((SELECT min(a.environment) FROM checkout.payment_attempts a WHERE a.tenant_id=o.tenant_id AND a.store_id=o.store_id AND a.order_id=o.id),p_environment) AS env
  FROM mo JOIN checkout.orders o ON o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=mo.order_id
  LEFT JOIN ses ON ses.order_id=mo.order_id
 ), cnt AS (
  SELECT b.principal_id,b.session_id,b.currency,count(*) FILTER (WHERE b.env=p_environment AND b.commercial_state<>'CANCELLED') AS orders,
   count(*) FILTER (WHERE b.env=p_environment AND b.commercial_state='CANCELLED') AS cancelled
  FROM base b GROUP BY b.principal_id,b.session_id,b.currency
 ), mny AS (
  SELECT b.principal_id,b.session_id,e.currency,e.environment,
   count(*) FILTER (WHERE e.kind='captured') AS captured_count,
   coalesce(sum(e.minor) FILTER (WHERE e.kind='captured'),0)::bigint AS captured_minor,
   coalesce(sum(e.minor) FILTER (WHERE e.kind='refunded'),0)::bigint AS refunded_minor,
   count(*) FILTER (WHERE e.kind='offline') AS offline_count,
   coalesce(sum(e.minor) FILTER (WHERE e.kind='offline'),0)::bigint AS offline_minor
  FROM identity.report_money_events(s.tenant_id,p_store,s.t0,s.t1) e JOIN base b ON b.order_id=e.order_id
  GROUP BY b.principal_id,b.session_id,e.currency,e.environment
 ), keys AS (
  SELECT principal_id,session_id,currency FROM cnt UNION SELECT principal_id,session_id,currency FROM mny
 )
 SELECT coalesce(jsonb_agg(jsonb_build_object('principal_id',k.principal_id,'session_id',k.session_id,'currency',k.currency,
   'orders',coalesce(n.orders,0),'cancelled_orders',coalesce(n.cancelled,0),
   'money',coalesce((SELECT jsonb_agg(jsonb_build_object('environment',m.environment,'captured_count',m.captured_count,
      'captured_minor',m.captured_minor,'refunded_minor',m.refunded_minor,'net_minor',m.captured_minor-m.refunded_minor,
      'offline_count',m.offline_count,'offline_minor',m.offline_minor) ORDER BY m.environment)
     FROM mny m WHERE m.principal_id IS NOT DISTINCT FROM k.principal_id AND m.session_id IS NOT DISTINCT FROM k.session_id
      AND m.currency=k.currency),'[]'::jsonb))
   ORDER BY k.principal_id NULLS LAST,k.session_id NULLS LAST,k.currency),'[]'::jsonb)
 INTO v_rows FROM keys k
 LEFT JOIN cnt n ON n.principal_id IS NOT DISTINCT FROM k.principal_id AND n.session_id IS NOT DISTINCT FROM k.session_id AND n.currency=k.currency;
 v_result:=jsonb_build_object('from',p_from,'to',p_to,'timezone','Asia/Taipei','rows',v_rows);
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'manual order report access denied' USING ERRCODE=v_auth_error; END IF;
 IF octet_length(v_result::text)>1048576 THEN RAISE EXCEPTION 'manual order report unavailable' USING ERRCODE='PT503'; END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION identity.read_report_manual_orders(bytea,uuid,date,date,text) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_report_manual_orders(bytea,uuid,date,date,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_report_manual_orders(bytea,uuid,date,date,text) TO commerce_runtime;
COMMENT ON FUNCTION identity.read_report_manual_orders(bytea,uuid,date,date,text) IS
 'internal/reporting only; EXECUTE commerce_runtime. orders:read; p_environment = the deployment payment environment (counts only); merchant-created (source=merchant_manual) orders created in range per creating staff principal (NULL = receipt missing), live session (NULL = none) and currency, with order counts and per-environment money of facts dated in range; returns principal ids, never names or buyer data.';

-- ---------------------------------------------------------------------------------------
-- Report 3: funnel claim -> link sent -> order -> paid, per session or whole store. Cohort = bundles with an ACCEPTED claim event in range;
-- the later stages are NOT date-limited (a claim of the range that is paid next week counts). Stages are nested, so counts never grow:
-- link_sent = claimed with a SUCCEEDED private-reply link; ordered = link_sent with a non-CANCELLED order; paid = ordered with a CAPTURED fact
-- or a COLLECTED COD/pay-at-pickup or a CONFIRMED bank transfer. ordered_without_link = ordered bundles whose link was not sent by the system.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION identity.read_report_funnel(p_hash bytea,p_store uuid,p_from date,p_to date,p_session uuid,p_environment text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog SET jit=off SET plan_cache_mode=force_custom_plan AS $$
DECLARE s record; v_counts jsonb; v_result jsonb; v_auth_error text;
BEGIN
 IF p_environment IS NULL THEN RAISE EXCEPTION 'invalid report read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.report_open(p_hash,p_store,p_from,p_to,'orders:read',p_environment);
 IF NOT identity.principal_holds(s.tenant_id,p_store,s.principal_id,ARRAY['live:read']) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 -- I01: the requested session must be this store's (domain definer; commerce_auth has no live.sessions grant).
 IF p_session IS NOT NULL AND (SELECT count(*) FROM live.order_session_labels(s.tenant_id,p_store,ARRAY[p_session]))<>1 THEN
  RAISE EXCEPTION 'session not found' USING ERRCODE='PT404'; END IF;
 WITH fb AS MATERIALIZED (
  SELECT f.bundle_id,f.link_sent,f.order_ids FROM claims.report_funnel_bundles(s.tenant_id,p_store,s.t0,s.t1,p_session) f
 ), live_orders AS MATERIALIZED (
  SELECT o.id FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.commercial_state<>'CANCELLED'
   AND o.id IN (SELECT unnest(f.order_ids) FROM fb f)
 ), paid AS MATERIALIZED (
  SELECT a.order_id AS id FROM payments.facts f JOIN checkout.payment_attempts a ON a.tenant_id=f.tenant_id AND a.store_id=f.store_id AND a.id=f.attempt_id
   WHERE f.tenant_id=s.tenant_id AND f.store_id=p_store AND f.kind='CAPTURED' AND f.environment=p_environment AND a.order_id IN (SELECT x.id FROM live_orders x)
  UNION
  SELECT o.id FROM checkout.orders o
   WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.collection_state='COLLECTED' AND o.payment_mode IN ('pay_at_pickup','cash_on_delivery')
    AND o.id IN (SELECT x.id FROM live_orders x)
  UNION
  SELECT t.order_id FROM checkout.bank_transfers t
   WHERE t.tenant_id=s.tenant_id AND t.store_id=p_store AND t.state='CONFIRMED' AND t.order_id IN (SELECT x.id FROM live_orders x)
 ), st AS (
  SELECT f.link_sent,
   EXISTS(SELECT 1 FROM unnest(f.order_ids) x WHERE x IN (SELECT id FROM live_orders)) AS has_order,
   EXISTS(SELECT 1 FROM unnest(f.order_ids) x WHERE x IN (SELECT id FROM paid)) AS has_paid
  FROM fb f
 )
 SELECT jsonb_build_object('claimed',count(*),'link_sent',count(*) FILTER (WHERE link_sent),
   'ordered',count(*) FILTER (WHERE link_sent AND has_order),'paid',count(*) FILTER (WHERE link_sent AND has_paid),
   'ordered_without_link',count(*) FILTER (WHERE NOT link_sent AND has_order))
 INTO v_counts FROM st;
 v_result:=jsonb_build_object('from',p_from,'to',p_to,'timezone','Asia/Taipei','session_id',p_session)||v_counts;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read','live:read'],s.tenant_id,s.principal_id,s.revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'funnel report access denied' USING ERRCODE=v_auth_error; END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION identity.read_report_funnel(bytea,uuid,date,date,uuid,text) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_report_funnel(bytea,uuid,date,date,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_report_funnel(bytea,uuid,date,date,uuid,text) TO commerce_runtime;
COMMENT ON FUNCTION identity.read_report_funnel(bytea,uuid,date,date,uuid,text) IS
 'internal/reporting only; EXECUTE commerce_runtime. orders:read AND live:read; p_environment = the deployment payment environment (a CAPTURED fact of another environment does not make a bundle paid); nested counts claimed >= link_sent >= ordered >= paid of the bundles with an ACCEPTED claim in range (optional session of this store, else 404) plus ordered_without_link; no actor key, no buyer data.';

-- ---------------------------------------------------------------------------------------
-- Audited export: orders:export AND orders:read, the same jsonb as the matching read definer, one audit row reports.export.<report>.
-- p_report: products | channels | funnel | manual_orders (p_session only for funnel); p_environment as the read definers. The audit action names the report:
-- reports.export.products | channels | funnel | manual_orders.
-- ---------------------------------------------------------------------------------------
CREATE POLICY auth_report_export_audit ON ops.audit_events FOR INSERT TO commerce_auth
 WITH CHECK (action IN ('reports.export.products','reports.export.channels','reports.export.funnel','reports.export.manual_orders')
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);

CREATE FUNCTION identity.export_report(p_hash bytea,p_store uuid,p_report text,p_from date,p_to date,p_session uuid,p_environment text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog SET jit=off SET plan_cache_mode=force_custom_plan AS $$
DECLARE s record; v_result jsonb; v_auth_error text;
BEGIN
 IF p_report IS NULL OR p_report NOT IN ('products','channels','funnel','manual_orders') OR (p_session IS NOT NULL AND p_report<>'funnel') THEN
  RAISE EXCEPTION 'invalid report export' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.report_open(p_hash,p_store,p_from,p_to,'orders:export',p_environment);
 -- The read definers re-authorize orders:read (and live:read for the funnel) and validate the range; same owner commerce_auth.
 v_result:=CASE p_report
  WHEN 'products' THEN identity.read_report_products(p_hash,p_store,p_from,p_to)
  WHEN 'channels' THEN identity.read_report_channels(p_hash,p_store,p_from,p_to,p_environment)
  WHEN 'funnel' THEN identity.read_report_funnel(p_hash,p_store,p_from,p_to,p_session,p_environment)
  ELSE identity.read_report_manual_orders(p_hash,p_store,p_from,p_to,p_environment) END;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:export','orders:read'],s.tenant_id,s.principal_id,s.revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'report export access denied' USING ERRCODE=v_auth_error; END IF;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(s.tenant_id,p_store,s.principal_id,'reports.export.'||p_report);
 RETURN v_result;
END $$;
ALTER FUNCTION identity.export_report(bytea,uuid,text,date,date,uuid,text) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.export_report(bytea,uuid,text,date,date,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.export_report(bytea,uuid,text,date,date,uuid,text) TO commerce_runtime;
COMMENT ON FUNCTION identity.export_report(bytea,uuid,text,date,date,uuid,text) IS
 'internal/reporting only; EXECUTE commerce_runtime. orders:export AND orders:read; the same jsonb as read_report_<report>, plus one ops.audit_events row reports.export.<report> (policy auth_report_export_audit); p_environment is the deployment payment environment.';
COMMENT ON POLICY auth_report_export_audit ON ops.audit_events IS 'commerce_auth may insert only the four reports.export.<report> audit rows, scoped to the GUCs identity.export_report verified.';
