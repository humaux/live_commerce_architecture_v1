-- 0157_dashboard_todos_perf.sql — perf fix (unit perf-dashboard-todos): the to-do counters of the merchant dashboard stay O(candidates)
--   even when the planner statistics of checkout.orders are stale.
-- Purpose: fulfillment.order_money_shippable and fulfillment.manual_shipment_eligible (bodies of 0107, result-identical) read the order row
--   by the unique id (orders_id_key) and keep tenant_id/store_id as non-sargable IS NOT DISTINCT FROM guards. Before: the three-column
--   predicate (tenant_id, store_id, id) let the planner choose a (tenant_id, store_id)-prefix index scan (checkout_one_active_cart_version)
--   whenever it believed the store holds ~1 order (a store that grew since the last ANALYZE: bulk import, fixture, new tenant). That scan reads
--   EVERY order of the store and filters on id: once in each of the pay_at_pickup / bank_transfer / cash_on_delivery branches of
--   order_money_shippable (2,596 buffers each) and again in manual_shipment_eligible, per candidate. identity.dashboard_todos on a store with
--   10,000 orders and 100 unshipped candidates read 779,511 buffers and took 500-650 ms (MTO03 budget 300 ms); after ANALYZE the same call
--   takes 6-9 ms. Now each lookup is one index probe (3 buffers) whatever the statistics say: 1,914 buffers, 11 ms on the stale statistics.
-- No index and no table rewrite: CREATE OR REPLACE FUNCTION only (catalog row lock, no lock on checkout.orders or any pilot table).
-- Depends on: 0063/0073/0088/0107 (the function history; the 0107 bodies are copied, only the order lookup predicate changes), checkout.orders
--   unique index orders_id_key(id) (0013), roles commerce_checkout_writer and commerce_auth (owner/EXECUTE re-asserted as in 0107).
-- Used by: identity.dashboard_todos (0094/0107), identity.read_merchant_orders / the unshipped filter (0073/0110), identity.export_unshipped_orders,
--   fulfillment.read_pick_list (0130), fulfillment.parcel_merge_block (0146), fulfillment.record_manual_shipment, request_cvs_shipment.
-- Invariants: output identical for every input (tenant_id and store_id are NOT NULL, so IS NOT DISTINCT FROM equals =); signatures, owners, grants
--   and SECURITY INVOKER/STABLE attributes unchanged. Tests: TestMerchantToolsDashboardScale (MTO03), TestMerchantToolsDashboard (MTO02).

DO $$
BEGIN
 IF to_regclass('checkout.orders_id_key') IS NULL
  OR to_regprocedure('fulfillment.order_money_shippable(uuid,uuid,uuid)') IS NULL
  OR to_regprocedure('fulfillment.manual_shipment_eligible(uuid,uuid,uuid)') IS NULL THEN
  RAISE EXCEPTION '0157 needs checkout.orders_id_key and the 0107 order_money_shippable/manual_shipment_eligible'; END IF;
END $$;

CREATE OR REPLACE FUNCTION fulfillment.order_money_shippable(p_tenant uuid,p_store uuid,p_order uuid)
RETURNS boolean LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
 SELECT EXISTS(
  SELECT 1 FROM checkout.orders o
  JOIN checkout.payment_attempts a ON a.tenant_id=o.tenant_id AND a.store_id=o.store_id
   AND a.owner_id=o.owner_id AND a.order_id=o.id
  JOIN fulfillment.payment_work_items w ON w.tenant_id=o.tenant_id AND w.store_id=o.store_id
   AND w.owner_id=o.owner_id AND w.order_id=o.id AND w.attempt_id=a.id AND w.state='READY'
  JOIN payments.facts f ON f.tenant_id=a.tenant_id AND f.store_id=a.store_id AND f.attempt_id=a.id
   AND f.kind='CAPTURED' AND f.connection_id=a.connection_id AND f.execution_profile=a.execution_profile
   AND f.environment=a.environment AND f.currency=a.currency AND f.amount_minor=a.amount_minor
   AND f.currency=o.currency AND f.amount_minor=o.total_minor
  WHERE o.id=p_order AND o.tenant_id IS NOT DISTINCT FROM p_tenant AND o.store_id IS NOT DISTINCT FROM p_store
   AND o.commercial_state='CONFIRMED' AND o.payment_mode='card'
   AND NOT EXISTS(SELECT 1 FROM payments.review_cases rc WHERE rc.tenant_id=a.tenant_id
     AND rc.store_id=a.store_id AND rc.attempt_id=a.id AND rc.reason<>'PROVIDER_PRESENTMENT_DRIFT')
   AND NOT EXISTS(SELECT 1 FROM (
     SELECT coalesce(sum(r.amount_minor),0) AS held FROM payments.stripe_refunds r
      WHERE r.tenant_id=a.tenant_id AND r.store_id=a.store_id AND r.attempt_id=a.id
       AND NOT EXISTS(SELECT 1 FROM payments.refund_facts rf WHERE rf.tenant_id=r.tenant_id
        AND rf.store_id=r.store_id AND rf.refund_id=r.id AND rf.kind IN ('FAILED','CANCELED','REJECTED'))
    ) h WHERE h.held>0 AND h.held>=f.amount_minor))
 OR EXISTS(
  SELECT 1 FROM checkout.orders o
  WHERE o.id=p_order AND o.tenant_id IS NOT DISTINCT FROM p_tenant AND o.store_id IS NOT DISTINCT FROM p_store
   AND o.commercial_state='CONFIRMED' AND o.payment_mode='pay_at_pickup' AND o.collection_state='PENDING'
   AND NOT EXISTS(SELECT 1 FROM checkout.payment_attempts pa WHERE pa.tenant_id=o.tenant_id
    AND pa.store_id=o.store_id AND pa.order_id=o.id))
 OR EXISTS(
  SELECT 1 FROM checkout.orders o
  JOIN checkout.bank_transfers t ON t.tenant_id=o.tenant_id AND t.store_id=o.store_id AND t.owner_id=o.owner_id AND t.order_id=o.id
  WHERE o.id=p_order AND o.tenant_id IS NOT DISTINCT FROM p_tenant AND o.store_id IS NOT DISTINCT FROM p_store
   AND o.commercial_state='CONFIRMED' AND o.payment_mode='bank_transfer' AND t.state='CONFIRMED'
   AND t.confirmed_amount_minor=o.total_minor
   AND NOT EXISTS(SELECT 1 FROM checkout.payment_attempts pa WHERE pa.tenant_id=o.tenant_id
    AND pa.store_id=o.store_id AND pa.order_id=o.id))
 OR EXISTS(
  -- home-cod: a home cash_on_delivery order is shippable while AWAITING_COLLECTION and PENDING; it never has a payment attempt.
  SELECT 1 FROM checkout.orders o
  WHERE o.id=p_order AND o.tenant_id IS NOT DISTINCT FROM p_tenant AND o.store_id IS NOT DISTINCT FROM p_store
   AND o.commercial_state='AWAITING_COLLECTION' AND o.payment_mode='cash_on_delivery' AND o.collection_state='PENDING'
   AND NOT EXISTS(SELECT 1 FROM checkout.payment_attempts pa WHERE pa.tenant_id=o.tenant_id
    AND pa.store_id=o.store_id AND pa.order_id=o.id))
$$;
ALTER FUNCTION fulfillment.order_money_shippable(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.order_money_shippable(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.order_money_shippable(uuid,uuid,uuid) TO commerce_checkout_writer,commerce_auth;
COMMENT ON FUNCTION fulfillment.order_money_shippable(uuid,uuid,uuid) IS 'internal/fulfillment: the MD6 payment/review/refund clauses of 0063 (card) plus the pay_at_pickup branch (§16.2), the bank_transfer branch (0088) and the cash_on_delivery branch (0107). SECURITY INVOKER: runs with the calling definer''s grants (commerce_checkout_writer, commerce_auth). Non-goal: does not authorize anyone or check fulfillment state. 0157: the order row is read by the unique id (orders_id_key) with tenant and store as non-sargable guards, so the plan is a one-row lookup whatever the planner statistics say.';

-- manual_shipment_eligible (0073:936 body): a COD order is MANUAL_UNASSIGNED + AWAITING_COLLECTION, and order_money_shippable
-- now answers true for it; the no-live-ECPay-attempt clause is trivially true (home orders have no CVS shipment).
CREATE OR REPLACE FUNCTION fulfillment.manual_shipment_eligible(p_tenant uuid,p_store uuid,p_order uuid)
RETURNS boolean LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
 SELECT EXISTS(
  SELECT 1 FROM checkout.orders o
  WHERE o.id=p_order AND o.tenant_id IS NOT DISTINCT FROM p_tenant AND o.store_id IS NOT DISTINCT FROM p_store
   AND o.commercial_state IN ('CONFIRMED','AWAITING_COLLECTION') AND o.fulfillment_state='MANUAL_UNASSIGNED'
   AND fulfillment.order_money_shippable(o.tenant_id,o.store_id,o.id)
   AND NOT EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id
     AND c.order_id=o.id AND c.state NOT IN ('FAILED','ABANDONED')))
$$;
ALTER FUNCTION fulfillment.manual_shipment_eligible(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.manual_shipment_eligible(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.manual_shipment_eligible(uuid,uuid,uuid) TO commerce_checkout_writer,commerce_auth;
COMMENT ON FUNCTION fulfillment.manual_shipment_eligible(uuid,uuid,uuid) IS 'MD6 shipping eligibility (0063, replaced in 0073, widened in 0107): order_money_shippable AND (CONFIRMED or AWAITING_COLLECTION) + MANUAL_UNASSIGNED AND no live ECPay attempt. Single source for record_manual_shipment, request_cvs_shipment, the list filter and the export. 0157: same one-row lookup by orders_id_key as order_money_shippable.';
