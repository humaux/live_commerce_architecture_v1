-- 0130_pick_list.sql — W3-02B (unit w3-02b-picklist): pick list and carrier export read path.
-- Purpose: one read-only definer fulfillment.read_pick_list shared by the pick-list projection and the carrier
--   export (Go adds orders:export + the audit event), plus claims.pick_list_session_orders so the checkout-owned
--   reader resolves a session's orders (live_price_uses UNION order_origins) without a claim-table grant.
-- Depends on: 0001 (ops.audit_events, control.stores), 0003 (identity.resolve_access), 0064/0073 (claims.live_price_uses,
--   claims.bundles, claims.order_origins), the checkout order/fulfillment state vocabulary (CONFIRMED,
--   AWAITING_COLLECTION, MANUAL_UNASSIGNED).
-- Used by: internal/merchantorders/picklist.go + carrier_export.go (read_pick_list), the session branch of the pick-list
--   route (pick_list_session_orders). Tests: TestPickList/TestPickList500/TestCarrierExport,
--   TestMerchantOrdersV2PickListReadAuthority, TestWAS02.
-- Invariants: SECURITY DEFINER SET search_path=pg_catalog, REVOKE ALL FROM PUBLIC, EXECUTE commerce_runtime only
--   (read_pick_list) / commerce_checkout_writer (pick_list_session_orders); collection rule commercial_state IN
--   (CONFIRMED, AWAITING_COLLECTION) AND fulfillment_state = MANUAL_UNASSIGNED; other-store and non-existent ids are
--   indistinguishable (order_not_found); a present but non-pickable id is not_pickable. No tables, columns or roles.

-- ---------------------------------------------------------------------------------------------------
-- claims.pick_list_session_orders: session -> order ids (PL-OPEN-1). Owned by the claims writer so RLS
-- on claims.live_price_uses / claims.order_origins admits it; the checkout reader only receives EXECUTE.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION claims.pick_list_session_orders(p_tenant uuid,p_store uuid,p_session uuid)
RETURNS TABLE(order_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT DISTINCT x.order_id FROM (
  SELECT u.order_id
  FROM claims.live_price_uses u
  JOIN claims.bundles b ON b.tenant_id=u.tenant_id AND b.store_id=u.store_id AND b.id=u.bundle_id
  WHERE u.tenant_id=p_tenant AND u.store_id=p_store
    AND b.tenant_id=p_tenant AND b.store_id=p_store AND b.session_id=p_session
  UNION
  SELECT o.order_id
  FROM claims.order_origins o
  WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.session_id=p_session
 ) x
$$;
ALTER FUNCTION claims.pick_list_session_orders(uuid,uuid,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.pick_list_session_orders(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.pick_list_session_orders(uuid,uuid,uuid) TO commerce_checkout_writer;
COMMENT ON FUNCTION claims.pick_list_session_orders(uuid,uuid,uuid) IS 'W3-02B session->orders resolution (PL-OPEN-1: live_price_uses UNION order_origins). commerce_claims_writer-only read; called by fulfillment.read_pick_list for {session_id} selections.';

-- ---------------------------------------------------------------------------------------------------
-- fulfillment.read_pick_list: the shared pick list / carrier export reader. Returns {orders, totals,
-- skipped}. Each order carries the destination/PII/money columns the carrier CSV needs plus the
-- sku lines (option_label is always null: the catalog has no variant model). orders:read only.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION fulfillment.read_pick_list(p_hash bytea,p_store uuid,p_orders uuid[],p_session uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_requested uuid[]; v_pickable uuid[]; v_orders jsonb; v_totals jsonb; v_skipped jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid pick list request' USING ERRCODE='PT400'; END IF;
 IF (p_orders IS NULL) = (p_session IS NULL) THEN
  RAISE EXCEPTION 'invalid pick list selection' USING ERRCODE='PT400'; END IF;
 IF p_orders IS NOT NULL AND cardinality(p_orders) NOT BETWEEN 1 AND 500 THEN
  IF cardinality(p_orders) > 500 THEN
   RAISE EXCEPTION 'too_many' USING ERRCODE='PT422';
  ELSE
   RAISE EXCEPTION 'invalid pick list selection' USING ERRCODE='PT400'; END IF; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 IF p_session IS NULL THEN
  SELECT coalesce(array_agg(x ORDER BY x),'{}'::uuid[]) INTO v_requested
   FROM (SELECT DISTINCT unnest(p_orders) AS x) d;
 ELSE
  SELECT coalesce(array_agg(order_id ORDER BY order_id),'{}'::uuid[]) INTO v_requested
   FROM (SELECT DISTINCT order_id FROM claims.pick_list_session_orders(s.tenant_id,p_store,p_session)) d;
 END IF;
 SELECT coalesce(array_agg(o.id ORDER BY o.id),'{}'::uuid[]) INTO v_pickable FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=ANY(v_requested)
   AND o.commercial_state IN ('CONFIRMED','AWAITING_COLLECTION')
   AND o.fulfillment_state='MANUAL_UNASSIGNED';
 SELECT coalesce(jsonb_agg(jsonb_build_object(
   'order_id',o.id,'order_number','LC-'||upper(replace(o.id::text,'-','')),
   'created_at_utc',to_char(o.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
   'service_code',o.service_code,'destination_kind',coalesce(o.snapshot#>>'{destination,kind}',''),
   'recipient_name',coalesce(o.snapshot#>>'{destination,recipient_name}',''),
   'phone',coalesce(o.snapshot#>>'{destination,phone}',''),'country',o.country,
   'region',coalesce(o.snapshot#>>'{destination,home_address,region}',''),
   'city',coalesce(o.snapshot#>>'{destination,home_address,city}',''),
   'postal_code',coalesce(o.snapshot#>>'{destination,home_address,postal_code}',''),
   'line1',coalesce(o.snapshot#>>'{destination,home_address,line1}',''),
   'line2',coalesce(o.snapshot#>>'{destination,home_address,line2}',''),
   'pickup_namespace',coalesce(o.snapshot#>>'{destination,pickup,namespace}',''),
   'pickup_code',coalesce(o.snapshot#>>'{destination,pickup,code}',''),
   'pickup_name',coalesce(o.snapshot#>>'{destination,pickup,name}',''),
   'pickup_address',coalesce(o.snapshot#>>'{destination,pickup,address}',''),
   'pickup_source',CASE o.snapshot#>>'{destination,pickup,verification_kind}' WHEN 'PROVIDER_DIRECTORY_VERIFIED' THEN 'ecpay_directory'
     WHEN 'BUYER_ENTERED' THEN 'buyer_entered' WHEN 'MANUAL_ATTESTED' THEN 'merchant_attested' ELSE '' END,
   'payment_mode',o.payment_mode,
   'total_minor',o.total_minor,'currency',o.currency,
   'collect_minor',o.total_minor+coalesce(o.cod_surcharge_minor,0),
   'lines',coalesce((SELECT jsonb_agg(jsonb_build_object('sku_id',line->>'sku_id','sku_code',line->>'code',
      'title',line->>'name','option_label',NULL,'qty',(line->>'quantity')::bigint) ORDER BY line->>'code')
     FROM jsonb_array_elements(o.snapshot#>'{quote,lines}') line),'[]'::jsonb)
  ) ORDER BY o.created_at ASC,o.id ASC),'[]'::jsonb) INTO v_orders
  FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=ANY(v_pickable);
 SELECT coalesce(jsonb_agg(t ORDER BY t->>'sku_code'),'[]'::jsonb) INTO v_totals FROM (
  SELECT jsonb_build_object('sku_id',line->>'sku_id','sku_code',line->>'code','title',line->>'name',
   'option_label',NULL,'qty',sum((line->>'quantity')::bigint)) AS t
  FROM checkout.orders o
  CROSS JOIN LATERAL jsonb_array_elements(o.snapshot#>'{quote,lines}') line
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=ANY(v_pickable)
  GROUP BY line->>'sku_id',line->>'code',line->>'name') tt;
 SELECT coalesce(jsonb_agg(jsonb_build_object('order_id',req.id,'code',
   CASE WHEN o.id IS NULL THEN 'order_not_found' ELSE 'not_pickable' END) ORDER BY req.id),'[]'::jsonb) INTO v_skipped
  FROM unnest(v_requested) AS req(id)
  LEFT JOIN checkout.orders o ON o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=req.id
  LEFT JOIN unnest(v_pickable) AS p(id) ON p.id=req.id
  WHERE p.id IS NULL;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF octet_length(v_orders::text)>8388608 THEN RAISE EXCEPTION 'pick list unavailable' USING ERRCODE='PT503'; END IF;
 RETURN jsonb_build_object('orders',v_orders,'totals',v_totals,'skipped',v_skipped);
END $$;
ALTER FUNCTION fulfillment.read_pick_list(bytea,uuid,uuid[],uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.read_pick_list(bytea,uuid,uuid[],uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.read_pick_list(bytea,uuid,uuid[],uuid) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.read_pick_list(bytea,uuid,uuid[],uuid) IS 'internal/merchantorders PickList/CarrierExport only; EXECUTE commerce_runtime. orders:read; picks CONFIRMED|AWAITING_COLLECTION + MANUAL_UNASSIGNED for order_ids (<=500) or session_id (live_price_uses UNION order_origins). Skipped ids carry not_pickable|order_not_found. option_label is always null (no variant model).';
