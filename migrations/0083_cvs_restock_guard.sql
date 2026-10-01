-- 0083 CVS pay-at-pickup restock guard (T21-01, docs/delivery/units/cvs-restock-fix.md; contracts/taiwan-cvs-logistics-v1.md §16.4).
--
-- Root cause: inventory.release_pay_at_pickup refused restock with a deny-list of shipment states (CREATED/AT_DC/AT_STORE), so a
-- PICKED_UP parcel passed; and ingest_ecpay_status never reversed RETURNED when 7-ELEVEN re-delivered (2098) the parcel the 2074
-- report had marked unclaimed. Sequence 2030,2073,2074,2098,2067 left the order RETURNED while ECPay collected the cash, and the
-- merchant could then restock goods the buyer took. record_collection('returned') had no shipment check at all.
--
-- Owns: fulfillment.cvs_parcel_returned (the ONE predicate) and forward-only CREATE OR REPLACE of the three 0073 functions that use
-- or need it. Signatures, owners, search_path, grants unchanged; each body differs from 0073 only at the T21-01 lines.
-- Non-goals: no data backfill (the pilot has no orders), no new table/column/permission, no change to card orders.
-- Depends on: 0072, 0073. Callers: the three definers below (checkout_writer, GUCs already set).

-- Positive rule (CR1): the parcel is back with the merchant (or never left) iff the latest ECPay attempt is UNCLAIMED, or no attempt
-- was ever handed to ECPay (empty or FAILED/ABANDONED-only history = manual / MERCHANT_SHIPPED path). REQUESTED, UNKNOWN,
-- CREATED, AT_DC, AT_STORE and PICKED_UP all answer false. INVOKER like settle_cvs_attempt: runs with the calling definer's grants.
CREATE FUNCTION fulfillment.cvs_parcel_returned(p_tenant uuid,p_store uuid,p_order uuid) RETURNS boolean
LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
 SELECT NOT EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=p_tenant AND c.store_id=p_store AND c.order_id=p_order
   AND c.state NOT IN ('FAILED','ABANDONED'))
  OR coalesce((SELECT c.state='UNCLAIMED' FROM fulfillment.cvs_shipments c WHERE c.tenant_id=p_tenant AND c.store_id=p_store
   AND c.order_id=p_order ORDER BY c.attempt DESC LIMIT 1),false)
$$;
ALTER FUNCTION fulfillment.cvs_parcel_returned(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.cvs_parcel_returned(uuid,uuid,uuid) FROM PUBLIC;
COMMENT ON FUNCTION fulfillment.cvs_parcel_returned(uuid,uuid,uuid) IS 'fulfillment helper (no caller EXECUTE): true iff the latest ECPay attempt is UNCLAIMED or no attempt was ever handed to ECPay (FAILED/ABANDONED-only). The single predicate behind pay-at-pickup restock and merchant-recorded returned (T21-01); callers hold the tenant/store GUCs. Read-only.';

CREATE OR REPLACE FUNCTION fulfillment.ingest_ecpay_status(p_endpoint uuid,p_body_sha256 bytea,p_merchant_id text,p_merchant_trade_no text,
 p_logistics_id text,p_rtn_code text,p_rtn_msg text,p_update_date text,p_payment_no text,p_validation_no text)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE pr record; c fulfillment.cvs_shipments%ROWTYPE; o record; v_code text; v_date text; v_msg text; v_to text; v_now timestamptz:=clock_timestamp();
 v_inserted integer; v_seven boolean; v_lid text; v_pay text; v_val text; v_collect text; v_conflict boolean:=false; v_event uuid; v_ship_state text; c_dc text; c_store text; c_pick text; c_back text;
BEGIN
 IF p_endpoint IS NULL OR p_body_sha256 IS NULL OR octet_length(p_body_sha256)<>32 OR p_merchant_id IS NULL
  OR p_merchant_trade_no IS NULL THEN RAISE EXCEPTION 'invalid status report' USING ERRCODE='PT400'; END IF;
 SELECT x.tenant_id,x.store_id,x.connection_id INTO pr FROM integration.ecpay_logistics_profiles x WHERE x.endpoint_id=p_endpoint;
 IF NOT FOUND THEN RETURN 'ignored'; END IF;
 PERFORM set_config('app.tenant_id',pr.tenant_id::text,true),set_config('app.store_id',pr.store_id::text,true);
 IF NOT EXISTS(SELECT 1 FROM integration.merchant_accounts a WHERE a.tenant_id=pr.tenant_id AND a.store_id=pr.store_id
   AND a.id=pr.connection_id AND a.account_id=p_merchant_id) THEN RETURN 'ignored'; END IF;
 SELECT x.order_id INTO o FROM fulfillment.cvs_shipments x WHERE x.connection_id=pr.connection_id AND x.merchant_trade_no=p_merchant_trade_no;
 IF NOT FOUND THEN RETURN 'ignored'; END IF;
 PERFORM 1 FROM checkout.orders k WHERE k.tenant_id=pr.tenant_id AND k.store_id=pr.store_id AND k.id=o.order_id FOR UPDATE;
 SELECT x.* INTO c FROM fulfillment.cvs_shipments x WHERE x.connection_id=pr.connection_id AND x.merchant_trade_no=p_merchant_trade_no FOR UPDATE;
 PERFORM set_config('app.principal_id',c.principal_id::text,true);
 IF c.state='REQUESTED' THEN RETURN 'retry'; END IF;      -- the create's own Finish decides first; ECPay retries
 v_lid:=CASE WHEN p_logistics_id ~ '^[0-9A-Za-z_-]{1,40}$' THEN p_logistics_id END;
 IF c.provider_logistics_id IS NOT NULL AND v_lid IS DISTINCT FROM c.provider_logistics_id THEN RETURN 'ignored'; END IF;
 -- Normalisation (round 1, finding 13): non-conforming values become NULL, the message loses control chars and is truncated.
 v_code:=CASE WHEN p_rtn_code ~ '^[0-9]{1,8}$' THEN p_rtn_code END;
 v_date:=CASE WHEN p_update_date ~ '^[0-9]{4}/[0-9]{2}/[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}$' THEN p_update_date END;
 v_msg:=left(regexp_replace(coalesce(p_rtn_msg,''),'[[:cntrl:]]','','g'),200);
 INSERT INTO fulfillment.cvs_shipment_events(tenant_id,store_id,order_id,attempt,source,body_sha256,provider_code,provider_message,
  provider_updated_at)
 VALUES(c.tenant_id,c.store_id,c.order_id,c.attempt,'ecpay_status',p_body_sha256,v_code,nullif(v_msg,''),v_date)
 ON CONFLICT (tenant_id,store_id,order_id,attempt,body_sha256) DO NOTHING RETURNING id INTO v_event;
 IF v_event IS NULL THEN RETURN 'duplicate'; END IF;
 IF c.state IN ('FAILED','ABANDONED') THEN
  -- MD1 one-live index: never touched. The report proves a label may exist for an attempt the merchant gave up: alert only.
  INSERT INTO fulfillment.cvs_shipment_events(tenant_id,store_id,order_id,attempt,source,event_code,from_state,to_state)
   VALUES(c.tenant_id,c.store_id,c.order_id,c.attempt,'local','alert.duplicate_label_risk',c.state,c.state);
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
   VALUES(c.tenant_id,c.store_id,c.principal_id,'fulfillment.cvs_duplicate_label_risk');
  RETURN 'applied';
 END IF;
 v_ship_state:=c.state;
 -- UNKNOWN + a MAC-verified report of this trade proves the create landed: adopt the provider ids, then map the status.
 IF c.state='UNKNOWN' AND v_lid IS NOT NULL THEN
  v_pay:=CASE WHEN p_payment_no ~ '^[0-9A-Za-z_-]{1,40}$' THEN p_payment_no END;
  v_val:=CASE WHEN p_validation_no ~ '^[0-9A-Za-z_-]{1,40}$' THEN p_validation_no END;
  UPDATE fulfillment.cvs_shipments SET state='CREATED',provider_logistics_id=v_lid,cvs_payment_no=coalesce(v_pay,cvs_payment_no),
   cvs_validation_no=coalesce(v_val,cvs_validation_no),result_code='ecpay.created',updated_at=v_now,version=version+1
   WHERE tenant_id=c.tenant_id AND store_id=c.store_id AND order_id=c.order_id AND attempt=c.attempt RETURNING * INTO c;
  UPDATE checkout.orders SET fulfillment_state='PROVIDER_LABEL_CREATED',updated_at=v_now
   WHERE tenant_id=c.tenant_id AND store_id=c.store_id AND id=c.order_id;
  INSERT INTO fulfillment.cvs_shipment_events(tenant_id,store_id,order_id,attempt,source,event_code,from_state,to_state)
   VALUES(c.tenant_id,c.store_id,c.order_id,c.attempt,'local','ecpay.created','UNKNOWN','CREATED');
 ELSIF c.state='UNKNOWN' THEN
  RETURN 'applied';                                  -- event stored; the trade id is unproven, so no transition
 END IF;
 -- §6 exact code table; unknown code, OK mart code or a backwards jump = event only.
 v_seven:=c.logistics_subtype LIKE 'UNIMART%';
 v_to:=NULL;
 -- Expected code per transition for this subtype (F7, retrieved 2026-09-29): to DC 7-11 2030 / others 3024; at store 7-11 B2C 2063,
 -- 7-11 C2C 2073, others 3018; picked up 7-11 2067 / others 3022; unclaimed 7-11 2074 / others 3020; 7-11 re-delivered 2098.
 c_dc:=CASE WHEN v_seven THEN '2030' ELSE '3024' END;
 c_store:=CASE WHEN c.logistics_subtype='UNIMART' THEN '2063' WHEN c.logistics_subtype='UNIMARTC2C' THEN '2073' ELSE '3018' END;
 c_pick:=CASE WHEN v_seven THEN '2067' ELSE '3022' END;
 c_back:=CASE WHEN v_seven THEN '2074' ELSE '3020' END;
 IF c.logistics_subtype<>'OKMARTC2C' AND v_code IS NOT NULL THEN
  IF c.state='CREATED' AND v_code=c_dc THEN v_to:='AT_DC';
  ELSIF c.state IN ('CREATED','AT_DC') AND v_code=c_store THEN v_to:='AT_STORE';
  ELSIF c.state='AT_STORE' AND v_code=c_pick THEN v_to:='PICKED_UP';
  ELSIF c.state='AT_STORE' AND v_code=c_back THEN v_to:='UNCLAIMED';
  ELSIF c.state IN ('AT_STORE','UNCLAIMED') AND v_seven AND v_code='2098' THEN v_to:='AT_STORE';
  END IF;
 END IF;
 UPDATE fulfillment.cvs_shipments SET state=coalesce(v_to,state),last_status_code=coalesce(v_code,last_status_code),
  last_status_at=CASE WHEN v_code IS NOT NULL THEN v_now ELSE last_status_at END,updated_at=v_now,version=version+1
  WHERE tenant_id=c.tenant_id AND store_id=c.store_id AND order_id=c.order_id AND attempt=c.attempt;
 IF v_to IS NOT NULL THEN
  INSERT INTO fulfillment.cvs_shipment_events(tenant_id,store_id,order_id,attempt,source,event_code,from_state,to_state)
   VALUES(c.tenant_id,c.store_id,c.order_id,c.attempt,'local','shipment.status',c.state,v_to);
 END IF;
 -- T21-01: 7-ELEVEN 2098 re-delivery reverses the 2074 the RETURNED state was derived from: back to PENDING (audited), so the
 -- buyer's later 2067 reaches COLLECTED through the normal branch below. A merchant-restocked order is RESTOCKED, never touched.
 IF v_seven AND v_code='2098' AND c.state='UNCLAIMED' AND v_to='AT_STORE' THEN
  UPDATE checkout.orders SET collection_state='PENDING',updated_at=v_now
   WHERE tenant_id=c.tenant_id AND store_id=c.store_id AND id=c.order_id AND payment_mode='pay_at_pickup' AND collection_state='RETURNED';
  IF FOUND THEN
   -- §11.5 n/a: no stock, ledger, payment or refund row.
   INSERT INTO fulfillment.cvs_shipment_events(tenant_id,store_id,order_id,attempt,source,event_code,from_state,to_state)
    VALUES(c.tenant_id,c.store_id,c.order_id,c.attempt,'local','collection.reverted','RETURNED','PENDING');
   INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
    VALUES(c.tenant_id,c.store_id,c.principal_id,'fulfillment.collection_reported');
  END IF;
 END IF;
 -- §16.4 collection: 2067/3022 PICKED_UP => COLLECTED, 2074/3020 UNCLAIMED => RETURNED (pay_at_pickup orders only, audited).
 -- Keyed on the reported code, not on the transition: a pickup/return report that cannot move the shipment (e.g. 2067 after
 -- 2074 + merchant restock, TCV17) still raises collection_conflict against the state the merchant already recorded.
 IF c.logistics_subtype<>'OKMARTC2C' AND v_code IN (c_pick,c_back) THEN
  SELECT k.payment_mode,k.collection_state INTO o FROM checkout.orders k
   WHERE k.tenant_id=c.tenant_id AND k.store_id=c.store_id AND k.id=c.order_id;
  IF o.payment_mode='pay_at_pickup' THEN
   v_collect:=CASE v_code WHEN c_pick THEN 'COLLECTED' ELSE 'RETURNED' END;
   IF o.collection_state='PENDING' AND v_to IN ('PICKED_UP','UNCLAIMED') THEN
    UPDATE checkout.orders SET collection_state=v_collect,updated_at=v_now
     WHERE tenant_id=c.tenant_id AND store_id=c.store_id AND id=c.order_id;
    -- §11.5 n/a: no stock, ledger, payment or refund row; the platform never moves pay-at-pickup money.
    INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
     VALUES(c.tenant_id,c.store_id,c.principal_id,'fulfillment.collection_reported');
   ELSIF o.collection_state<>'PENDING' AND o.collection_state <> ALL(CASE v_collect WHEN 'COLLECTED'
     THEN ARRAY['COLLECTED','REFUNDED_OFFLINE'] ELSE ARRAY['RETURNED','RESTOCKED'] END) THEN
    -- A conflicting state the merchant already recorded (its own successors REFUNDED_OFFLINE/RESTOCKED are not a conflict):
    -- event + alert only, never a change.
    INSERT INTO fulfillment.cvs_shipment_events(tenant_id,store_id,order_id,attempt,source,event_code,from_state,to_state)
     VALUES(c.tenant_id,c.store_id,c.order_id,c.attempt,'local','alert.collection_conflict',o.collection_state,v_collect);
   END IF;
  END IF;
 END IF;
 RETURN 'applied';
END $$;
ALTER FUNCTION fulfillment.ingest_ecpay_status(uuid,bytea,text,text,text,text,text,text,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.ingest_ecpay_status(uuid,bytea,text,text,text,text,text,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.ingest_ecpay_status(uuid,bytea,text,text,text,text,text,text,text,text) TO commerce_runtime;

CREATE OR REPLACE FUNCTION fulfillment.record_collection(p_hash bytea,p_store uuid,p_order uuid,p_key text,p_request_hash bytea,
 p_expected_state text,p_state text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; o record; v_saved bytea; v_response jsonb; v_to text; v_now timestamptz;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_key IS NULL
  OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$' OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32
  OR p_expected_state IS NULL OR p_expected_state NOT IN ('PENDING','COLLECTED','RETURNED','REFUNDED_OFFLINE','CANCELLED','RESTOCKED')
  OR p_state IS NULL OR p_state NOT IN ('collected','returned','refunded_offline')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid collection record' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT k.owner_id,k.payment_mode,k.collection_state,k.fulfillment_state INTO o FROM checkout.orders k
  WHERE k.tenant_id=s.tenant_id AND k.store_id=p_store AND k.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 PERFORM set_config('app.buyer_id',o.owner_id::text,true);
 SELECT r.request_hash,r.response INTO v_saved,v_response FROM ops.command_results r WHERE r.tenant_id=s.tenant_id
  AND r.store_id=p_store AND r.operation='fulfillment.collection.record' AND r.idempotency_key=p_key;
 IF FOUND THEN
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  IF o.payment_mode<>'pay_at_pickup' THEN RAISE EXCEPTION 'not_pay_at_pickup' USING ERRCODE='PT422'; END IF;
  v_to:=upper(p_state);
  IF p_state IN ('collected','returned') THEN
   IF o.fulfillment_state NOT IN ('MERCHANT_SHIPPED','PROVIDER_LABEL_CREATED') THEN
    RAISE EXCEPTION 'not_shipped' USING ERRCODE='PT422'; END IF;
   IF p_expected_state<>'PENDING' OR o.collection_state<>'PENDING' THEN
    RAISE EXCEPTION 'collection_state_changed' USING ERRCODE='PT409'; END IF;
   -- T21-01: a merchant-recorded 'returned' needs the same evidence the restock needs (UNCLAIMED or never handed to ECPay).
   IF p_state='returned' AND NOT fulfillment.cvs_parcel_returned(s.tenant_id,p_store,p_order) THEN
    RAISE EXCEPTION 'parcel_not_returned' USING ERRCODE='PT409'; END IF;
  ELSE
   IF p_expected_state<>'COLLECTED' OR o.collection_state<>'COLLECTED' THEN
    RAISE EXCEPTION 'collection_state_changed' USING ERRCODE='PT409'; END IF;
  END IF;
  v_now:=clock_timestamp();
  UPDATE checkout.orders SET collection_state=v_to,updated_at=v_now WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_order;
  -- §11.5 n/a: no stock, ledger, payment or refund row; the merchant collects through its own channel or ECPay does.
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
   VALUES(s.tenant_id,p_store,s.principal_id,'fulfillment.collection_recorded');
  v_response:=jsonb_build_object('order_id',p_order,'collection_state',v_to);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(s.tenant_id,p_store,'fulfillment.collection.record',p_key,p_request_hash,v_response,s.principal_id);
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF v_final.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION fulfillment.record_collection(bytea,uuid,uuid,text,bytea,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.record_collection(bytea,uuid,uuid,text,bytea,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.record_collection(bytea,uuid,uuid,text,bytea,text,text) TO commerce_runtime;

CREATE OR REPLACE FUNCTION inventory.release_pay_at_pickup(p_hash bytea,p_store uuid,p_order uuid,p_key text,p_request_hash bytea,
 p_action text,p_expected_state text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; o record; r record; l record; v_saved bytea; v_response jsonb; v_op text; v_to text; v_lines integer:=0;
 v_now timestamptz;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_key IS NULL
  OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$' OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32
  OR p_action IS NULL OR p_action NOT IN ('cancel','restock')
  OR p_expected_state IS NULL OR p_expected_state NOT IN ('PENDING','RETURNED')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid release request' USING ERRCODE='PT400'; END IF;
 -- Authorize before any lock (A2, 0063:299-305); tenant/store/principal GUCs come from this result.
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT k.owner_id,k.creator_session_id,k.payment_mode,k.commercial_state,k.fulfillment_state,k.collection_state INTO o
  FROM checkout.orders k WHERE k.tenant_id=s.tenant_id AND k.store_id=p_store AND k.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 -- The ledger guard compares the buyer columns with these GUCs (0063:330 pattern): they come from the locked order row.
 PERFORM set_config('app.buyer_id',o.owner_id::text,true),set_config('app.buyer_session_id',o.creator_session_id::text,true);
 SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=s.tenant_id
  AND c.store_id=p_store AND c.operation='inventory.pay_at_pickup.release' AND c.idempotency_key=p_key;
 IF FOUND THEN
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  IF o.payment_mode<>'pay_at_pickup' THEN RAISE EXCEPTION 'not_pay_at_pickup' USING ERRCODE='PT422'; END IF;  -- card orders: RD6
  IF p_action='cancel' AND p_expected_state<>'PENDING' OR p_action='restock' AND p_expected_state<>'RETURNED' THEN
   RAISE EXCEPTION 'invalid release request' USING ERRCODE='PT400'; END IF;
  IF o.collection_state IS DISTINCT FROM p_expected_state THEN
   RAISE EXCEPTION 'collection_state_changed' USING ERRCODE='PT409'; END IF;
  PERFORM fulfillment.settle_cvs_attempt(s.tenant_id,p_store,p_order);
  IF p_action='cancel' THEN
   IF o.commercial_state<>'CONFIRMED' OR o.fulfillment_state<>'MANUAL_UNASSIGNED' THEN
    RAISE EXCEPTION 'not_cancellable' USING ERRCODE='PT422'; END IF;
   -- Not handed to a provider: every attempt FAILED or ABANDONED (REQUESTED/UNKNOWN may already be at ECPay).
   IF EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store AND c.order_id=p_order
     AND c.state IN ('REQUESTED','UNKNOWN')) THEN RAISE EXCEPTION 'cvs_attempt_in_flight' USING ERRCODE='PT409'; END IF;
   IF EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store AND c.order_id=p_order
     AND c.state NOT IN ('FAILED','ABANDONED')) THEN RAISE EXCEPTION 'not_cancellable' USING ERRCODE='PT422'; END IF;
   v_op:='fulfillment.pay_at_pickup.cancel'; v_to:='CANCELLED';
  ELSE
   -- T21-01 (positive rule, no deny-list): restock only when the latest attempt is UNCLAIMED or no attempt was ever handed to ECPay.
   IF NOT fulfillment.cvs_parcel_returned(s.tenant_id,p_store,p_order) THEN
    RAISE EXCEPTION 'parcel_not_returned' USING ERRCODE='PT409'; END IF;
   v_op:='fulfillment.pay_at_pickup.restock'; v_to:='RESTOCKED';
  END IF;
  v_now:=clock_timestamp();
  IF p_action='cancel' THEN
   UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED',collection_state='CANCELLED',updated_at=v_now
    WHERE tenant_id=s.tenant_id AND store_id=p_store AND owner_id=o.owner_id AND id=p_order;
  ELSE
   UPDATE checkout.orders SET collection_state='RESTOCKED',updated_at=v_now
    WHERE tenant_id=s.tenant_id AND store_id=p_store AND owner_id=o.owner_id AND id=p_order;
  END IF;
  SELECT x.state INTO r FROM inventory.reservations x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
  IF NOT FOUND OR r.state<>'COMMITTED' THEN RAISE EXCEPTION 'collection_state_changed' USING ERRCODE='PT409'; END IF;
  UPDATE inventory.reservations SET state='RELEASED' WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_order;
  -- §11.5: evidence = order id + collection_state + actor, all on the ledger row (operation, command_key, reason, principal).
  FOR l IN SELECT x.warehouse_id,x.sku_id,x.quantity FROM inventory.reservation_lines x
   WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.reservation_id=p_order ORDER BY x.warehouse_id,x.sku_id LOOP
   PERFORM 1 FROM inventory.lock_balance(l.warehouse_id,l.sku_id);
   INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_allocated,operation,command_key,reservation_id,
    reason,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
   VALUES(s.tenant_id,p_store,l.warehouse_id,l.sku_id,'DEALLOCATE',-l.quantity,v_op,p_order::text,p_order,p_expected_state,
    s.principal_id,p_order,o.owner_id,o.creator_session_id,'MERCHANT');
   v_lines:=v_lines+1;
  END LOOP;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
   VALUES(s.tenant_id,p_store,s.principal_id,CASE p_action WHEN 'cancel' THEN 'fulfillment.pay_at_pickup_cancelled' ELSE 'fulfillment.pay_at_pickup_restocked' END);
  v_response:=jsonb_build_object('order_id',p_order,'collection_state',v_to,
   'commercial_state',CASE WHEN p_action='cancel' THEN 'CANCELLED' ELSE o.commercial_state END,'released_lines',v_lines);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(s.tenant_id,p_store,'inventory.pay_at_pickup.release',p_key,p_request_hash,v_response,s.principal_id);
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF v_final.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text) TO commerce_runtime;

COMMENT ON FUNCTION fulfillment.ingest_ecpay_status(uuid,bytea,text,text,text,text,text,text,text,text) IS 'internal/fulfillment status route only; EXECUTE commerce_runtime (unauthenticated; Go verified CheckMacValue first). Returns applied|duplicate|retry|ignored; never raises on vendor data; exact §6 code table; pay_at_pickup collection moves with 2067/3022 and 2074/3020 (audited); a 7-ELEVEN 2098 re-delivery reverts RETURNED to PENDING (audited, T21-01).';
COMMENT ON FUNCTION fulfillment.record_collection(bytea,uuid,uuid,text,bytea,text,text) IS 'internal/fulfillment CVS.RecordCollection only; EXECUTE commerce_runtime. fulfillment:write; manual collected/returned/refunded_offline of a pay_at_pickup order; no money movement, no ledger row. returned also requires fulfillment.cvs_parcel_returned (PT409 parcel_not_returned, T21-01).';
COMMENT ON FUNCTION inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text) IS 'internal/fulfillment CVS.Release only; EXECUTE commerce_runtime. §16.8: the ONLY writer of DEALLOCATE ledger rows; cancel (PENDING, unshipped) and restock (RETURNED, latest ECPay attempt UNCLAIMED or none handed over: fulfillment.cvs_parcel_returned) release the order allocation for exactly the allocated quantity; §11.5 evidence = order id + collection_state + actor on the ledger row; no payments/refund/operation/river row.';
