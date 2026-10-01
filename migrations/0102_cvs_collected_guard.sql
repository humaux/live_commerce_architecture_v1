-- 0102 CVS pay-at-pickup collected guard (contracts/taiwan-cvs-logistics-v1.md §16.4).
--
-- Root cause: fulfillment.record_collection accepted 'collected' while the order's latest ECPay attempt was still UNCLAIMED (or any
-- other not-yet-picked-up state), so a merchant could mark cash as collected for a parcel the buyer never picked up, and finance
-- (0085 finance_pay_at_pickup) then counted it. record_collection('returned') already had the T21-01 cvs_parcel_returned guard; the
-- 'collected' branch had none.
--
-- Owns: fulfillment.cvs_parcel_picked_up (ONE predicate, mirroring fulfillment.cvs_parcel_returned from 0083) and a forward-only
-- CREATE OR REPLACE of fulfillment.record_collection. Signature, owner, search_path, grants and COMMENT for record_collection are
-- updated; the body differs from 0083 only at the T21-02 line. No other 0073/0083 function changes.
-- Non-goals: no data backfill (the pilot has no orders), no new table/column/permission, no change to card orders.
-- Depends on: 0072, 0073, 0083. Callers: fulfillment.record_collection (checkout_writer, GUCs already set).

-- Positive rule (CR2): the buyer picked up the parcel (so the merchant's collected record is grounded) iff the latest ECPay attempt
-- is PICKED_UP, or no attempt was ever handed to ECPay (empty or FAILED/ABANDONED-only history = manual / MERCHANT_SHIPPED path).
-- UNCLAIMED, REQUESTED, UNKNOWN, CREATED, AT_DC and AT_STORE all answer false. INVOKER like settle_cvs_attempt: runs with the
-- calling definer's grants.
CREATE FUNCTION fulfillment.cvs_parcel_picked_up(p_tenant uuid,p_store uuid,p_order uuid) RETURNS boolean
LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
 SELECT NOT EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=p_tenant AND c.store_id=p_store AND c.order_id=p_order
   AND c.state NOT IN ('FAILED','ABANDONED'))
  OR coalesce((SELECT c.state='PICKED_UP' FROM fulfillment.cvs_shipments c WHERE c.tenant_id=p_tenant AND c.store_id=p_store
   AND c.order_id=p_order ORDER BY c.attempt DESC LIMIT 1),false)
$$;
ALTER FUNCTION fulfillment.cvs_parcel_picked_up(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.cvs_parcel_picked_up(uuid,uuid,uuid) FROM PUBLIC;
COMMENT ON FUNCTION fulfillment.cvs_parcel_picked_up(uuid,uuid,uuid) IS 'fulfillment helper (no caller EXECUTE): true iff the latest ECPay attempt is PICKED_UP or no attempt was ever handed to ECPay (FAILED/ABANDONED-only). The single predicate behind merchant-recorded collected (T21-02); callers hold the tenant/store GUCs. Read-only.';

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
   -- T21-02: a merchant-recorded 'collected' needs PICKED_UP (or never handed to ECPay): cash for a parcel the buyer never
   -- picked up must never be recordable, or finance (0085) counts it.
   IF p_state='collected' AND NOT fulfillment.cvs_parcel_picked_up(s.tenant_id,p_store,p_order) THEN
    RAISE EXCEPTION 'parcel_not_picked_up' USING ERRCODE='PT409'; END IF;
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
COMMENT ON FUNCTION fulfillment.record_collection(bytea,uuid,uuid,text,bytea,text,text) IS 'internal/fulfillment CVS.RecordCollection only; EXECUTE commerce_runtime. fulfillment:write; manual collected/returned/refunded_offline of a pay_at_pickup order; no money movement, no ledger row. returned also requires fulfillment.cvs_parcel_returned (PT409 parcel_not_returned, T21-01); collected also requires fulfillment.cvs_parcel_picked_up (PT409 parcel_not_picked_up, T21-02).';
