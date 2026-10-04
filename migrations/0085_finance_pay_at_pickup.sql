-- 0085 finance: pay-at-pickup money collected by the carrier (docs/delivery/units/ops-polish.md OP3; amends
-- contracts/customers-billing-v1.md BD7).
--
-- Owns: the replacement of identity.read_finance_summary (0078) that adds, per (day, currency, environment), the two keys
-- pickup_collected_count / pickup_collected_minor, and the one extra column grant it needs. identity.export_finance_summary
-- (0078) calls read_finance_summary, so the CSV gets the columns without a second change.
--
-- Non-goals: pay-at-pickup money is a different money path (the carrier remits, no PSP fact exists), so it is NEVER added to
-- captured_* or net_minor; no write, no new table, no new column on checkout.orders, no change to refunds.
--
-- Depends on: 0027 (commerce_auth SELECT on checkout.orders incl. updated_at/currency/total_minor, policy merchant_order_projection
-- USING(true)), 0072/0073 (payment_mode, collection_state, fulfillment.cvs_shipments, GRANT SELECT(tenant_id,store_id,order_id,
-- attempt,state), policy cvs_shipment_auth_read USING(true), GRANT SELECT(payment_mode,collection_state) to commerce_auth), 0078.
--
-- Caller: internal/reporting.Finance / FinanceCSV through commerce_runtime only.
--
-- No new RLS policy: commerce_auth already reads checkout.orders and fulfillment.cvs_shipments through the USING(true) projection
-- policies named above (0027, 0073). Permissive policies are OR-ed, so a second "scoped" policy would add nothing; the scope fence is
-- the explicit tenant_id/store_id predicate inside the definer, exactly like the captured/refunded CTEs of 0078.

DO $$
BEGIN
 IF to_regprocedure('identity.read_finance_summary(bytea,uuid,date,date)') IS NULL
  OR to_regclass('fulfillment.cvs_shipments') IS NULL
  OR NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='checkout' AND table_name='orders' AND column_name='collection_state') THEN
  RAISE EXCEPTION '0085 requires 0078 (read_finance_summary) and 0072/0073 (cvs_shipments, collection_state)';
 END IF;
END $$;

-- The environment of a pay-at-pickup row (ruling, integrator R3): that of the order's latest fulfillment.cvs_shipments attempt;
-- when the order has none (buyer_entered store / merchant-shipped path) it is 'LIVE', because that money is real cash.
GRANT SELECT(environment) ON fulfillment.cvs_shipments TO commerce_auth;

CREATE OR REPLACE FUNCTION identity.read_finance_summary(p_hash bytea,p_store uuid,p_from date,p_to date)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_rows jsonb; v_auth_error text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_from IS NULL OR p_to IS NULL
  OR NOT isfinite(p_from) OR NOT isfinite(p_to) OR p_to-p_from NOT BETWEEN 0 AND 91
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid finance read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 WITH lim AS (
  SELECT (p_from::timestamp AT TIME ZONE 'Asia/Taipei') AS t0,((p_to+1)::timestamp AT TIME ZONE 'Asia/Taipei') AS t1
 ), cap AS (
  -- I05: amounts are the CAPTURED fact's own minor units; no currency conversion or cross-currency sum.
  SELECT (f.received_at AT TIME ZONE 'Asia/Taipei')::date AS day,f.currency,f.environment,
   count(*)::bigint AS n,sum(f.amount_minor)::bigint AS minor
  FROM payments.facts f,lim
  WHERE f.tenant_id=s.tenant_id AND f.store_id=p_store AND f.kind='CAPTURED'
   AND f.received_at>=lim.t0 AND f.received_at<lim.t1
  GROUP BY 1,2,3
 ), ref AS (
  SELECT (rf.received_at AT TIME ZONE 'Asia/Taipei')::date AS day,r.currency,a.environment,sum(r.amount_minor)::bigint AS minor
  FROM payments.refund_facts rf CROSS JOIN lim
  JOIN payments.stripe_refunds r ON r.tenant_id=rf.tenant_id AND r.store_id=rf.store_id AND r.id=rf.refund_id
  JOIN checkout.payment_attempts a ON a.tenant_id=r.tenant_id AND a.store_id=r.store_id AND a.id=r.attempt_id
  WHERE rf.tenant_id=s.tenant_id AND rf.store_id=p_store AND rf.kind='SUCCEEDED'
   AND rf.received_at>=lim.t0 AND rf.received_at<lim.t1
   AND NOT EXISTS(SELECT 1 FROM payments.refund_facts later WHERE later.tenant_id=rf.tenant_id AND later.store_id=rf.store_id
     AND later.refund_id=rf.refund_id AND later.kind IN ('FAILED','CANCELED') AND later.received_at>rf.received_at)
  GROUP BY 1,2,3
 ), pick AS (
  -- OP3 pay-at-pickup money the carrier collected. checkout.orders has no collected_at: COLLECTED is written only by
  -- fulfillment.ingest_ecpay_status and fulfillment.record_collection (0073), each `UPDATE ... SET collection_state, updated_at=now`,
  -- so orders.updated_at IS the COLLECTED transition time for as long as the order stays COLLECTED (a later REFUNDED_OFFLINE leaves
  -- the filter; any other write to a COLLECTED order would move its day, a known limit). Environment: latest cvs_shipments attempt,
  -- else 'LIVE' (real cash, see the GRANT above). I05: the order's own total_minor/currency, no conversion; never mixed into cap.
  SELECT (o.updated_at AT TIME ZONE 'Asia/Taipei')::date AS day,o.currency,
   coalesce(sh.environment,'LIVE') AS environment,count(*)::bigint AS n,sum(o.total_minor)::bigint AS minor
  FROM checkout.orders o CROSS JOIN lim
  LEFT JOIN LATERAL (SELECT x.environment FROM fulfillment.cvs_shipments x
    WHERE x.tenant_id=o.tenant_id AND x.store_id=o.store_id AND x.order_id=o.id ORDER BY x.attempt DESC LIMIT 1) sh ON true
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.payment_mode='pay_at_pickup' AND o.collection_state='COLLECTED'
   AND o.updated_at>=lim.t0 AND o.updated_at<lim.t1
  GROUP BY 1,2,3
 ), keys AS (
  SELECT day,currency,environment FROM cap UNION SELECT day,currency,environment FROM ref
  UNION SELECT day,currency,environment FROM pick
 ), merged AS (
  SELECT k.day,k.currency,k.environment,coalesce(c.n,0) AS n,coalesce(c.minor,0) AS cm,coalesce(r.minor,0) AS rm,
   coalesce(p.n,0) AS pn,coalesce(p.minor,0) AS pm
  FROM keys k
  LEFT JOIN cap c ON c.day=k.day AND c.currency=k.currency AND c.environment=k.environment
  LEFT JOIN ref r ON r.day=k.day AND r.currency=k.currency AND r.environment=k.environment
  LEFT JOIN pick p ON p.day=k.day AND p.currency=k.currency AND p.environment=k.environment
 )
 SELECT coalesce(jsonb_agg(jsonb_build_object('day',to_char(m.day,'YYYY-MM-DD'),'currency',m.currency,
   'environment',m.environment,'captured_count',m.n,'captured_minor',m.cm,'refunded_minor',m.rm,'net_minor',m.cm-m.rm,
   'pickup_collected_count',m.pn,'pickup_collected_minor',m.pm)
   ORDER BY m.day,m.currency,m.environment),'[]'::jsonb) INTO v_rows FROM merged m;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'finance read access denied' USING ERRCODE=v_auth_error; END IF;
 IF octet_length(v_rows::text)>1048576 THEN RAISE EXCEPTION 'finance read unavailable' USING ERRCODE='PT503'; END IF;
 RETURN v_rows;
END $$;
ALTER FUNCTION identity.read_finance_summary(bytea,uuid,date,date) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_finance_summary(bytea,uuid,date,date) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_finance_summary(bytea,uuid,date,date) TO commerce_runtime;

COMMENT ON FUNCTION identity.read_finance_summary(bytea,uuid,date,date) IS
 'internal/reporting.Finance only; EXECUTE commerce_runtime. orders:read; 0..91 day range; days in Asia/Taipei; captured/refunded/net by currency and environment; refund counted on its SUCCEEDED day unless a later FAILED/CANCELED fact exists. 0085 adds pickup_collected_count/minor: pay_at_pickup orders in COLLECTED on the day of orders.updated_at (the COLLECTED transition; no collected_at exists), environment of the latest cvs_shipments attempt else LIVE; never part of captured or net.';
COMMENT ON COLUMN fulfillment.cvs_shipments.environment IS 'SANDBOX or LIVE of the connection; LIVE Create additionally needs CVS_ECPAY_LIVE_CREATE. Readable by commerce_auth (0085) only so identity.read_finance_summary can give a pay-at-pickup row its environment.';
