-- 0130_pick_list.sql — W3-02B (unit w3-02b-picklist): pick list + carrier export read path and the CVS batch version read.
-- Purpose: one read-only definer fulfillment.read_pick_list shared by the pick-list projection (money-only eligibility:
--   fulfillment.order_money_shippable) and the carrier export (full eligibility: fulfillment.manual_shipment_eligible plus a
--   no-CVS-destination rule so home-delivery templates never list store-pickup orders), claims.pick_list_session_orders so the
--   checkout-owned reader resolves a session's orders (live_price_uses UNION order_origins) without a claim-table grant, and
--   fulfillment.read_cvs_shipment_version so the CVS batch plans each order at the live attempt's version (P1-3). The pure
--   fulfillment.pick_list_lines_ok helper keeps a single malformed snapshot row out of the collection instead of raising.
-- Depends on: 0001 (ops.audit_events, control.stores), 0003 (identity.resolve_access), 0064/0073 (claims.live_price_uses,
--   claims.bundles, claims.order_origins, fulfillment.order_money_shippable, fulfillment.manual_shipment_eligible,
--   fulfillment.settle_cvs_attempt, fulfillment.cvs_shipments), the checkout order/fulfillment state vocabulary (CONFIRMED,
--   AWAITING_COLLECTION, MANUAL_UNASSIGNED), 0113 (claims.order_origins).
-- Used by: internal/merchantorders/picklist.go + carrier_export.go (read_pick_list), the session branch of the pick-list route
--   (pick_list_session_orders), internal/fulfillment/cvs.go requestBatchEntry (read_cvs_shipment_version), the carrier-export
--   audit (ops.audit_events.details). Tests: TestPickList/TestPickList500/TestCarrierExport/TestCarrierExportCOD,
--   TestCVSBatch/TestCVSBatchUnknown/TestPickListSession/TestMerchantOrdersV2PickListReadAuthority/TestWAS02/TestTaiwanCvsSchema.
-- Invariants: SECURITY DEFINER SET search_path=pg_catalog, REVOKE ALL FROM PUBLIC, EXECUTE commerce_runtime only
--   (read_pick_list, read_cvs_shipment_version) / commerce_checkout_writer (pick_list_session_orders); collection rule
--   commercial_state IN (CONFIRMED, AWAITING_COLLECTION) AND fulfillment_state = MANUAL_UNASSIGNED AND (pick list:
--   order_money_shippable; carrier export: manual_shipment_eligible AND currency TWD AND destination NOT IN (cvs_711,
--   cvs_familymart, cvs_hilife, cvs_okmart)); other-store and non-existent ids are indistinguishable (order_not_found); a
--   present but non-pickable id is not_pickable (including a malformed snapshot row and a CVS-destination order in an export).
--   One new nullable column (ops.audit_events.details) and three functions; no tables or roles.

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
-- fulfillment.pick_list_lines_ok: a pure helper that answers whether one order's quote.lines is a
-- well-formed non-empty array (<=50) of sku lines the pick list can project. A malformed snapshot row is
-- reported not_pickable instead of raising 22P02/22003 and turning the whole list into a 503. INVOKER, no
-- grants: only the read_pick_list definer (same owner) calls it.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION fulfillment.pick_list_lines_ok(p_snapshot jsonb) RETURNS boolean
LANGUAGE sql IMMUTABLE SET search_path=pg_catalog AS $$
 SELECT CASE
  WHEN jsonb_typeof(p_snapshot#>'{quote,lines}') IS DISTINCT FROM 'array' THEN false
  WHEN jsonb_array_length(p_snapshot#>'{quote,lines}') NOT BETWEEN 1 AND 50 THEN false
  ELSE NOT EXISTS(
   SELECT 1 FROM jsonb_array_elements(p_snapshot#>'{quote,lines}') line
   WHERE jsonb_typeof(line->'sku_id') IS DISTINCT FROM 'string'
     OR jsonb_typeof(line->'code') IS DISTINCT FROM 'string'
     OR jsonb_typeof(line->'name') IS DISTINCT FROM 'string'
     OR jsonb_typeof(line->'quantity') IS DISTINCT FROM 'number'
     OR (line->>'sku_id') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
     OR (line->>'code') !~ '^[A-Za-z0-9_.-]{1,64}$'
     OR (line->>'name') = '' OR length(line->>'name') > 120
     OR (line->>'quantity') !~ '^[0-9]+$'
     OR (line->>'quantity')::bigint < 1 OR (line->>'quantity')::bigint > 1000000000
  )
 END
$$;
ALTER FUNCTION fulfillment.pick_list_lines_ok(jsonb) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.pick_list_lines_ok(jsonb) FROM PUBLIC;
COMMENT ON FUNCTION fulfillment.pick_list_lines_ok(jsonb) IS 'W3-02B internal only (called by fulfillment.read_pick_list): well-formedness of quote.lines so one malformed row is not_pickable rather than a 22P02/22003 abort. Pure jsonb, no table access, no grants.';

-- ---------------------------------------------------------------------------------------------------
-- fulfillment.read_pick_list: the shared pick list / carrier export reader. p_export=false (pick list)
-- admits every money-shippable order (order_money_shippable, so a CVS order that already holds a live
-- label stays pickable); p_export=true (carrier export) admits only manual_shipment_eligible TWD orders
-- whose destination is home (never a CVS store pickup). Returns {orders, totals, skipped}. Each order
-- carries the destination/PII/money columns the carrier CSV needs plus the sku lines (option_label is
-- always null: the catalog has no variant model). orders:read only.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION fulfillment.read_pick_list(p_hash bytea,p_store uuid,p_orders uuid[],p_session uuid,p_export boolean)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_requested uuid[]; v_pickable uuid[]; v_orders jsonb; v_totals jsonb; v_skipped jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_export IS NULL
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
  IF cardinality(v_requested) > 500 THEN
   RAISE EXCEPTION 'too_many' USING ERRCODE='PT422'; END IF;
 END IF;
 SELECT coalesce(array_agg(o.id ORDER BY o.id),'{}'::uuid[]) INTO v_pickable FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=ANY(v_requested)
   AND o.commercial_state IN ('CONFIRMED','AWAITING_COLLECTION')
   AND o.fulfillment_state='MANUAL_UNASSIGNED'
   AND fulfillment.pick_list_lines_ok(o.snapshot)
   AND (CASE WHEN p_export THEN
      fulfillment.manual_shipment_eligible(s.tenant_id,p_store,o.id)
      AND o.currency='TWD'
      AND coalesce(o.snapshot#>>'{destination,kind}','') NOT IN ('cvs_711','cvs_familymart','cvs_hilife','cvs_okmart')
    ELSE
      fulfillment.order_money_shippable(s.tenant_id,p_store,o.id)
    END);
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
   'collect_minor',CASE WHEN o.payment_mode='cash_on_delivery' THEN o.total_minor+coalesce(o.cod_surcharge_minor,0) ELSE 0 END,
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
ALTER FUNCTION fulfillment.read_pick_list(bytea,uuid,uuid[],uuid,boolean) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.read_pick_list(bytea,uuid,uuid[],uuid,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.read_pick_list(bytea,uuid,uuid[],uuid,boolean) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.read_pick_list(bytea,uuid,uuid[],uuid,boolean) IS 'internal/merchantorders PickList/CarrierExport only; EXECUTE commerce_runtime. orders:read; picks CONFIRMED|AWAITING_COLLECTION + MANUAL_UNASSIGNED for order_ids (<=500) or session_id (<=500 resolved, live_price_uses UNION order_origins), pick list by order_money_shippable, carrier export by manual_shipment_eligible + TWD + non-CVS destination. Skipped ids carry not_pickable|order_not_found. option_label is always null (no variant model). collect_minor = total+cod_surcharge for cash_on_delivery else 0 (whole-TWD formatting is Go).';

-- ---------------------------------------------------------------------------------------------------
-- fulfillment.read_cvs_shipment_version: the CVS batch's per-order version read (P1-3). Settles any
-- stuck attempt (the same settle request_cvs_shipment performs), then returns the latest attempt's
-- version and state (0/NULL when none) so the batch plans the request at the exact CAS version instead
-- of always 0, and can report a live attempt as already. fulfillment:write.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION fulfillment.read_cvs_shipment_version(p_hash bytea,p_store uuid,p_order uuid)
RETURNS TABLE(version bigint,state text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; o record; l record; v_has boolean;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid cvs version read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 -- Lock order -> settle/attempts (the same order request_cvs_shipment takes), so the version read and the
 -- request that follows it in the same transaction cannot interleave with a concurrent request.
 SELECT x.owner_id INTO o FROM checkout.orders x
  WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 PERFORM set_config('app.buyer_id',o.owner_id::text,true);
 PERFORM fulfillment.settle_cvs_attempt(s.tenant_id,p_store,p_order);
 SELECT x.version,x.state INTO l FROM fulfillment.cvs_shipments x
  WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.order_id=p_order ORDER BY x.attempt DESC LIMIT 1;
 v_has := FOUND;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF NOT v_has THEN
  RETURN QUERY SELECT 0::bigint,NULL::text;
 ELSE
  RETURN QUERY SELECT l.version,l.state;
 END IF;
END $$;
ALTER FUNCTION fulfillment.read_cvs_shipment_version(bytea,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.read_cvs_shipment_version(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.read_cvs_shipment_version(bytea,uuid,uuid) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.read_cvs_shipment_version(bytea,uuid,uuid) IS 'internal/fulfillment CVS.requestBatchEntry only; EXECUTE commerce_runtime. fulfillment:write; settles a stuck attempt then returns (latest version, latest state) or (0, NULL). No writes beyond settle_cvs_attempt.';

-- ---------------------------------------------------------------------------------------------------
-- ops.audit_events.details: the carrier export records its template and row count (W3-02B P2-11). One
-- nullable object column; existing audit rows stay NULL and every existing writer is unaffected.
-- ---------------------------------------------------------------------------------------------------
ALTER TABLE ops.audit_events ADD COLUMN details jsonb
  CHECK (details IS NULL OR (jsonb_typeof(details)='object' AND octet_length(details::text)<=1024));
COMMENT ON COLUMN ops.audit_events.details IS 'W3-02B optional structured audit detail (carrier export: {template, rows}); <=1 KiB object or NULL.';
