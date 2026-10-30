-- Purpose: FEAT-BUYER-RMA separate buyer applications; approval atomically registers the unchanged merchant RMA.
-- Depends on: 0155 returns helpers, full buyer capability, trusted collection/shipment facts and forced scoped RLS.
-- Used by: internal/returns/request.go, buyerhttp and merchant httpapi request adapters.
-- Invariants: no money/stock writes; order lock serializes capacity; receipt + decision + linked RMA commit together.
CREATE TABLE returns.buyer_requests (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL, owner_id uuid NOT NULL, order_id uuid NOT NULL,
 number text NOT NULL, status text NOT NULL DEFAULT 'requested' CHECK(status IN ('requested','approved','rejected','withdrawn')),
 version bigint NOT NULL DEFAULT 1 CHECK(version BETWEEN 1 AND 9007199254740991), reason text NOT NULL CHECK(reason='damaged'),
 note text NOT NULL DEFAULT '' CHECK(length(note)<=1000 AND note !~ '[[:cntrl:]]'),
 rejection_reason text CHECK(length(btrim(rejection_reason)) BETWEEN 1 AND 1000 AND rejection_reason !~ '[[:cntrl:]]'),
 return_id uuid, created_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,id), UNIQUE(tenant_id,store_id,id,order_id), UNIQUE(tenant_id,store_id,number),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,return_id,order_id) REFERENCES returns.rmas(tenant_id,store_id,id,order_id),
 CHECK((status='approved')=(return_id IS NOT NULL)), CHECK((status='rejected')=(rejection_reason IS NOT NULL)),
 CHECK((status='requested' AND version=1) OR (status<>'requested' AND version=2))
);
CREATE INDEX buyer_requests_owner ON returns.buyer_requests(tenant_id,store_id,owner_id,created_at DESC,id);
CREATE INDEX buyer_requests_order ON returns.buyer_requests(tenant_id,store_id,order_id,status);
CREATE TABLE returns.buyer_request_lines (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, request_id uuid NOT NULL, order_id uuid NOT NULL,
 warehouse_id uuid NOT NULL, sku_id uuid NOT NULL, quantity bigint NOT NULL CHECK(quantity BETWEEN 1 AND 999999999),
 PRIMARY KEY(tenant_id,store_id,request_id,warehouse_id,sku_id),
 FOREIGN KEY(tenant_id,store_id,request_id,order_id) REFERENCES returns.buyer_requests(tenant_id,store_id,id,order_id),
 FOREIGN KEY(tenant_id,store_id,order_id,warehouse_id,sku_id) REFERENCES inventory.reservation_lines(tenant_id,store_id,reservation_id,warehouse_id,sku_id)
);
-- Receipts are actor-scoped, so a key cannot replay another buyer's or staff member's private answer.
CREATE TABLE returns.buyer_request_receipts (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, actor text NOT NULL, operation text NOT NULL,
 idempotency_key text NOT NULL, request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32), response jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(tenant_id,store_id,actor,operation,idempotency_key)
);
DO $$ DECLARE n text; BEGIN
 FOREACH n IN ARRAY ARRAY['buyer_requests','buyer_request_lines','buyer_request_receipts'] LOOP
  EXECUTE format('ALTER TABLE returns.%I ENABLE ROW LEVEL SECURITY',n);
  EXECUTE format('ALTER TABLE returns.%I FORCE ROW LEVEL SECURITY',n);
  EXECUTE format('REVOKE ALL ON returns.%I FROM PUBLIC,commerce_runtime,commerce_checkout_runtime,commerce_buyer_runtime,commerce_buyer_issuer',n);
  EXECUTE format('GRANT SELECT,INSERT ON returns.%I TO commerce_checkout_writer',n);
  EXECUTE format('CREATE POLICY private_writer ON returns.%I TO commerce_checkout_writer USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid) WITH CHECK(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',n);
 END LOOP;
END $$;
GRANT UPDATE(status,version,rejection_reason,return_id,updated_at) ON returns.buyer_requests TO commerce_checkout_writer;
COMMENT ON TABLE returns.buyer_requests IS '0210 buyer application, separate from merchant RMA; terminal decision CAS and linked return, never a refund.';
COMMENT ON TABLE returns.buyer_request_lines IS '0210 immutable selected order lines; one requested application per line enforced under the order row lock.';
COMMENT ON TABLE returns.buyer_request_receipts IS '0210 actor-scoped immutable command results; no direct runtime table privileges; retain with application.';
GRANT USAGE ON SCHEMA returns TO commerce_buyer_writer,commerce_buyer_runtime,commerce_checkout_runtime;

-- This narrow helper keeps capability table access in its existing owner; no old function ACL is widened.
CREATE FUNCTION returns.buyer_request_scope(p_hash bytea,p_store uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record;
BEGIN
 SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF EXISTS(SELECT 1 FROM buyer.capability_sessions c WHERE c.id=s.session_id AND c.view_order_id IS NOT NULL) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',s.store_id::text,true),
  set_config('app.buyer_id',s.owner_id::text,true),set_config('app.buyer_session_id',s.session_id::text,true);
 RETURN jsonb_build_object('t',s.tenant_id,'o',s.owner_id,'s',s.session_id);
END $$;
ALTER FUNCTION returns.buyer_request_scope(bytea,uuid) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION returns.buyer_request_scope(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION returns.buyer_request_scope(bytea,uuid) TO commerce_checkout_writer;
COMMENT ON FUNCTION returns.buyer_request_scope(bytea,uuid) IS '0210 internal only: resolve and lock full buyer authority, refuse view-only capability; establish server-derived scope.';

CREATE FUNCTION returns.buyer_request_json(p_t uuid,p_store uuid,p_id uuid) RETURNS jsonb
LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
 SELECT jsonb_build_object('id',r.id,'number',r.number,'order_id',r.order_id,'status',r.status,'version',r.version,
  'reason',r.reason,'note',r.note,'rejection_reason',r.rejection_reason,'return_id',r.return_id,
  'return_state',(SELECT m.state FROM returns.rmas m WHERE m.tenant_id=r.tenant_id AND m.store_id=r.store_id AND m.id=r.return_id),
  'created_at',r.created_at,'updated_at',r.updated_at,
  'lines',(SELECT jsonb_agg(jsonb_build_object('sku_id',l.sku_id,'warehouse_id',l.warehouse_id,'quantity',l.quantity) ORDER BY l.warehouse_id,l.sku_id)
    FROM returns.buyer_request_lines l WHERE l.tenant_id=r.tenant_id AND l.store_id=r.store_id AND l.request_id=r.id),
  'timeline',jsonb_build_array(jsonb_build_object('status','requested','at',r.created_at)) ||
    CASE WHEN r.status='requested' THEN '[]'::jsonb ELSE jsonb_build_array(jsonb_build_object('status',r.status,'at',r.updated_at)) END)
 FROM returns.buyer_requests r WHERE r.tenant_id=p_t AND r.store_id=p_store AND r.id=p_id
$$;
ALTER FUNCTION returns.buyer_request_json(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.buyer_request_json(uuid,uuid,uuid) FROM PUBLIC;
COMMENT ON FUNCTION returns.buyer_request_json(uuid,uuid,uuid) IS '0210 internal safe projection: no buyer/staff identity or contact fields; linked return state read live.';

CREATE FUNCTION returns.create_buyer_request(p_hash bytea,p_store uuid,p_order uuid,p_key text,p_reason text,p_note text,p_lines jsonb)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a jsonb; t uuid; own uuid; o record; x record; l record; n int; used bigint; deadline timestamptz;
 id uuid:=gen_random_uuid(); req bytea; saved bytea; answer jsonb; resolved jsonb:='[]';
 -- Named business constants: collected cooling-off 7 days; manual transit allowance 7 days + cooling-off 7 days.
 collected_window constant interval:=interval '7 days'; manual_shipped_window constant interval:=interval '14 days';
BEGIN
 a:=returns.buyer_request_scope(p_hash,p_store); t:=(a->>'t')::uuid; own:=(a->>'o')::uuid;
 IF p_order IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$' OR p_reason IS DISTINCT FROM 'damaged'
  OR p_note IS NULL OR length(p_note)>1000 OR p_note ~ '[[:cntrl:]]' OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422'; END IF;
 PERFORM 1 FROM returns.line_set(p_lines,'quantity',NULL);
 req:=sha256(convert_to(jsonb_build_object('order',p_order,'reason',p_reason,'note',p_note,'lines',p_lines)::text,'UTF8'));
 PERFORM pg_advisory_xact_lock(hashtextextended(t::text||p_store::text||own::text||'create'||p_key,0));
 SELECT * INTO o FROM checkout.orders k WHERE k.tenant_id=t AND k.store_id=p_store AND k.owner_id=own AND k.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 SELECT c.request_hash,c.response INTO saved,answer FROM returns.buyer_request_receipts c
  WHERE c.tenant_id=t AND c.store_id=p_store AND c.actor='buyer:'||own::text AND c.operation='create' AND c.idempotency_key=p_key;
 IF FOUND THEN
  IF saved<>req THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  IF NOT (o.fulfillment_state='MERCHANT_SHIPPED' OR (o.fulfillment_state='PROVIDER_LABEL_CREATED' AND EXISTS(
    SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=t AND c.store_id=p_store AND c.order_id=p_order AND c.state='PICKED_UP'))) THEN
   RAISE EXCEPTION 'not_shipped' USING ERRCODE='PT409'; END IF;
  IF NOT returns.order_returnable(o.commercial_state,o.payment_mode,o.collection_state) THEN
   RAISE EXCEPTION 'not_returnable' USING ERRCODE='PT409'; END IF;
  deadline:=o.collected_at+collected_window;
  IF deadline IS NULL THEN
   SELECT min(e.received_at)+collected_window INTO deadline FROM fulfillment.cvs_shipment_events e
    WHERE e.tenant_id=t AND e.store_id=p_store AND e.order_id=p_order AND e.to_state='PICKED_UP';
  END IF;
  IF deadline IS NULL THEN
   SELECT min(v.recorded_at)+manual_shipped_window INTO deadline FROM fulfillment.manual_shipment_versions v
    WHERE v.tenant_id=t AND v.store_id=p_store AND v.order_id=p_order AND v.status='SHIPPED';
  END IF;
  IF deadline IS NULL OR clock_timestamp()>deadline THEN RAISE EXCEPTION 'window_closed' USING ERRCODE='PT409'; END IF;
  FOR x IN SELECT * FROM returns.line_set(p_lines,'quantity',NULL) LOOP
   SELECT count(*) INTO n FROM inventory.reservation_lines q WHERE q.tenant_id=t AND q.store_id=p_store
    AND q.reservation_id=p_order AND q.sku_id=x.l_sku AND (x.l_wh IS NULL OR q.warehouse_id=x.l_wh);
   IF n=0 THEN RAISE EXCEPTION 'unknown_line' USING ERRCODE='PT422'; END IF;
   IF n>1 THEN RAISE EXCEPTION 'ambiguous_line' USING ERRCODE='PT422'; END IF;
   SELECT q.warehouse_id,q.quantity INTO l FROM inventory.reservation_lines q WHERE q.tenant_id=t AND q.store_id=p_store
    AND q.reservation_id=p_order AND q.sku_id=x.l_sku AND (x.l_wh IS NULL OR q.warehouse_id=x.l_wh);
   IF x.l_a<1 OR EXISTS(SELECT 1 FROM jsonb_array_elements(resolved) z WHERE z->>'sku_id'=x.l_sku::text AND z->>'warehouse_id'=l.warehouse_id::text) THEN
    RAISE EXCEPTION 'invalid_quantities' USING ERRCODE='PT422'; END IF;
   IF EXISTS(SELECT 1 FROM returns.buyer_request_lines q JOIN returns.buyer_requests r ON (r.tenant_id,r.store_id,r.id)=(q.tenant_id,q.store_id,q.request_id)
    WHERE q.tenant_id=t AND q.store_id=p_store AND q.order_id=p_order AND q.warehouse_id=l.warehouse_id AND q.sku_id=x.l_sku AND r.status='requested') THEN
    RAISE EXCEPTION 'request_open' USING ERRCODE='PT409'; END IF;
   SELECT coalesce(sum(q.qty_registered),0) INTO used FROM returns.rma_lines q JOIN returns.rmas r ON (r.tenant_id,r.store_id,r.id)=(q.tenant_id,q.store_id,q.rma_id)
    WHERE q.tenant_id=t AND q.store_id=p_store AND q.order_id=p_order AND q.warehouse_id=l.warehouse_id AND q.sku_id=x.l_sku AND r.state<>'CANCELLED';
   IF used+x.l_a>l.quantity THEN RAISE EXCEPTION 'exceeds_shipped' USING ERRCODE='PT422'; END IF;
   resolved:=resolved||jsonb_build_array(jsonb_build_object('sku_id',x.l_sku,'warehouse_id',l.warehouse_id,'quantity',x.l_a));
  END LOOP;
  INSERT INTO returns.buyer_requests(tenant_id,store_id,id,owner_id,order_id,number,reason,note)
   VALUES(t,p_store,id,own,p_order,'R-'||replace(id::text,'-',''),p_reason,p_note);
  INSERT INTO returns.buyer_request_lines(tenant_id,store_id,request_id,order_id,warehouse_id,sku_id,quantity)
   SELECT t,p_store,id,p_order,(z->>'warehouse_id')::uuid,(z->>'sku_id')::uuid,(z->>'quantity')::bigint FROM jsonb_array_elements(resolved) z;
  answer:=returns.buyer_request_json(t,p_store,id);
  INSERT INTO returns.buyer_request_receipts(tenant_id,store_id,actor,operation,idempotency_key,request_hash,response)
   VALUES(t,p_store,'buyer:'||own::text,'create',p_key,req,answer);
 END IF;
 IF returns.buyer_request_scope(p_hash,p_store)<>a THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 RETURN answer;
END $$;
ALTER FUNCTION returns.create_buyer_request(bytea,uuid,uuid,text,text,text,jsonb) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.create_buyer_request(bytea,uuid,uuid,text,text,text,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION returns.create_buyer_request(bytea,uuid,uuid,text,text,text,jsonb) TO commerce_buyer_runtime,commerce_checkout_runtime;
COMMENT ON FUNCTION returns.create_buyer_request(bytea,uuid,uuid,text,text,text,jsonb) IS '0210 buyer create: full own-order authority, trusted window, order lock, one open request per line, existing RMA capacity, actor-scoped replay, final authorization; no money.';

-- Shared read: merchant scope needs orders:read; buyer scope must be full and owns every projected row.
CREATE FUNCTION returns.read_buyer_requests(p_hash bytea,p_store uuid,p_id uuid,p_merchant boolean)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a jsonb; t uuid; own uuid; answer jsonb;
BEGIN
 IF p_merchant IS NULL THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422'; END IF;
 IF p_merchant THEN a:=returns.authorize(p_hash,p_store,'orders:read',NULL); t:=(a->>'t')::uuid;
 ELSE a:=returns.buyer_request_scope(p_hash,p_store); t:=(a->>'t')::uuid; own:=(a->>'o')::uuid; END IF;
 IF p_id IS NULL THEN
  SELECT jsonb_build_object('items',coalesce(jsonb_agg(returns.buyer_request_json(t,p_store,r.id) ORDER BY r.created_at DESC,r.id),'[]'::jsonb)) INTO answer
   FROM (SELECT q.* FROM returns.buyer_requests q WHERE q.tenant_id=t AND q.store_id=p_store AND (p_merchant OR q.owner_id=own) ORDER BY q.created_at DESC,q.id LIMIT 100) r;
 ELSE
  SELECT returns.buyer_request_json(t,p_store,r.id) INTO answer FROM returns.buyer_requests r
   WHERE r.tenant_id=t AND r.store_id=p_store AND r.id=p_id AND (p_merchant OR r.owner_id=own);
  IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 END IF;
 IF p_merchant THEN PERFORM returns.reauthorize(p_hash,p_store,'orders:read',NULL,a);
 ELSIF returns.buyer_request_scope(p_hash,p_store)<>a THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 RETURN answer;
END $$;
ALTER FUNCTION returns.read_buyer_requests(bytea,uuid,uuid,boolean) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.read_buyer_requests(bytea,uuid,uuid,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION returns.read_buyer_requests(bytea,uuid,uuid,boolean) TO commerce_runtime,commerce_buyer_runtime,commerce_checkout_runtime;
COMMENT ON FUNCTION returns.read_buyer_requests(bytea,uuid,uuid,boolean) IS '0210 scoped list/detail; merchant orders:read or full buyer own records; no contact/authority fields; reauthorize before returning.';

CREATE FUNCTION returns.decide_buyer_request(p_hash bytea,p_store uuid,p_id uuid,p_key text,p_expected bigint,p_action text,p_reason text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a jsonb; t uuid; own uuid; v_actor text; rr record; oid uuid; req bytea; saved bytea; answer jsonb; rma jsonb; ls jsonb;
BEGIN
 IF p_action IS NULL OR p_action NOT IN ('approve','reject','withdraw') OR p_id IS NULL OR p_expected IS NULL OR p_expected<1
  OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$' OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422'; END IF;
 IF p_action='withdraw' THEN
  a:=returns.buyer_request_scope(p_hash,p_store); t:=(a->>'t')::uuid; own:=(a->>'o')::uuid; v_actor:='buyer:'||own::text;
 ELSE
  a:=returns.authorize(p_hash,p_store,'fulfillment:write',NULL); t:=(a->>'t')::uuid; v_actor:='merchant:'||(a->>'p');
 END IF;
 IF p_action='reject' AND (p_reason IS NULL OR length(btrim(p_reason)) NOT BETWEEN 1 AND 1000 OR p_reason ~ '[[:cntrl:]]') THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422'; END IF;
 IF p_action<>'reject' AND coalesce(p_reason,'')<>'' THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422'; END IF;
 req:=sha256(convert_to(jsonb_build_object('id',p_id,'version',p_expected,'reason',p_reason)::text,'UTF8'));
 PERFORM pg_advisory_xact_lock(hashtextextended(t::text||p_store::text||v_actor||p_action||p_key,0));
 SELECT r.order_id INTO oid FROM returns.buyer_requests r WHERE r.tenant_id=t AND r.store_id=p_store AND r.id=p_id
  AND (p_action<>'withdraw' OR r.owner_id=own);
 IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 -- Same lock order as legacy register: order first. Decisions and new requests cannot race capacity.
 PERFORM 1 FROM checkout.orders o WHERE o.tenant_id=t AND o.store_id=p_store AND o.id=oid FOR UPDATE;
 SELECT r.* INTO rr FROM returns.buyer_requests r WHERE r.tenant_id=t AND r.store_id=p_store AND r.id=p_id FOR UPDATE;
 SELECT c.request_hash,c.response INTO saved,answer FROM returns.buyer_request_receipts c
  WHERE c.tenant_id=t AND c.store_id=p_store AND c.actor=v_actor AND c.operation=p_action AND c.idempotency_key=p_key;
 IF FOUND THEN
  IF saved<>req THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  IF rr.version<>p_expected THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
  IF rr.status<>'requested' THEN RAISE EXCEPTION 'invalid_state' USING ERRCODE='PT409'; END IF;
  IF p_action='approve' THEN
   SELECT jsonb_agg(jsonb_build_object('sku_id',l.sku_id,'warehouse_id',l.warehouse_id,'quantity',l.quantity) ORDER BY l.warehouse_id,l.sku_id) INTO ls
    FROM returns.buyer_request_lines l WHERE l.tenant_id=t AND l.store_id=p_store AND l.request_id=p_id;
   rma:=returns.register_rma(p_hash,p_store,oid,'buyer-rma:'||p_id::text,sha256(convert_to(p_id::text,'UTF8')),rr.reason,ls);
  END IF;
  UPDATE returns.buyer_requests SET status=CASE p_action WHEN 'approve' THEN 'approved' WHEN 'reject' THEN 'rejected' ELSE 'withdrawn' END,
   version=version+1,return_id=(rma->>'id')::uuid,rejection_reason=CASE WHEN p_action='reject' THEN btrim(p_reason) END,updated_at=clock_timestamp()
   WHERE tenant_id=t AND store_id=p_store AND id=p_id;
  answer:=returns.buyer_request_json(t,p_store,p_id);
  INSERT INTO returns.buyer_request_receipts(tenant_id,store_id,actor,operation,idempotency_key,request_hash,response)
   VALUES(t,p_store,v_actor,p_action,p_key,req,answer);
 END IF;
 IF p_action='withdraw' THEN
  IF returns.buyer_request_scope(p_hash,p_store)<>a THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 ELSE PERFORM returns.reauthorize(p_hash,p_store,'fulfillment:write',NULL,a); END IF;
 RETURN answer;
END $$;
ALTER FUNCTION returns.decide_buyer_request(bytea,uuid,uuid,text,bigint,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.decide_buyer_request(bytea,uuid,uuid,text,bigint,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION returns.decide_buyer_request(bytea,uuid,uuid,text,bigint,text,text) TO commerce_runtime,commerce_buyer_runtime,commerce_checkout_runtime;
COMMENT ON FUNCTION returns.decide_buyer_request(bytea,uuid,uuid,text,bigint,text,text) IS '0210 CAS/replay: full buyer withdraw, fulfillment:write approve/reject. Approval calls unchanged register_rma in same transaction. Rollback on any refusal, no refunds.';

COMMENT ON COLUMN returns.buyer_requests.tenant_id IS 'Server-derived tenant scope; forced RLS key.';
COMMENT ON COLUMN returns.buyer_requests.store_id IS 'Server-derived store scope; forced RLS key.';
COMMENT ON COLUMN returns.buyer_requests.id IS 'Server-generated immutable application UUID.';
COMMENT ON COLUMN returns.buyer_requests.owner_id IS 'Authenticated buyer owner of the referenced order, never from JSON.';
COMMENT ON COLUMN returns.buyer_requests.order_id IS 'Owned shipped order whose immutable reservation lines are requested.';
COMMENT ON COLUMN returns.buyer_requests.number IS 'Stable human reference: R- plus request UUID hex; unique per store.';
COMMENT ON COLUMN returns.buyer_requests.status IS 'Separate application status: requested, approved, rejected, withdrawn.';
COMMENT ON COLUMN returns.buyer_requests.version IS 'CAS version: 1 requested, 2 terminal; replay precedes version check.';
COMMENT ON COLUMN returns.buyer_requests.reason IS 'Closed frame-evidenced reason enum: damaged.';
COMMENT ON COLUMN returns.buyer_requests.note IS 'Buyer-supplied bounded application note; private, never logged.';
COMMENT ON COLUMN returns.buyer_requests.rejection_reason IS 'Nonblank bounded merchant explanation, present only for rejection.';
COMMENT ON COLUMN returns.buyer_requests.return_id IS 'Same-order existing RMA, created atomically on approval only.';
COMMENT ON COLUMN returns.buyer_requests.created_at IS 'Authoritative application submission instant.';
COMMENT ON COLUMN returns.buyer_requests.updated_at IS 'Submission or terminal decision instant; chronological timeline fact.';
COMMENT ON COLUMN returns.buyer_request_lines.tenant_id IS 'Authenticated tenant scope; forced RLS key.';
COMMENT ON COLUMN returns.buyer_request_lines.store_id IS 'Authenticated store scope; forced RLS key.';
COMMENT ON COLUMN returns.buyer_request_lines.request_id IS 'Immutable parent application; same-order composite FK.';
COMMENT ON COLUMN returns.buyer_request_lines.order_id IS 'Original order; composite FK ties the selected reservation line.';
COMMENT ON COLUMN returns.buyer_request_lines.warehouse_id IS 'Server-resolved original reservation warehouse.';
COMMENT ON COLUMN returns.buyer_request_lines.sku_id IS 'Original order SKU; cannot refer to an unrelated order.';
COMMENT ON COLUMN returns.buyer_request_lines.quantity IS 'Positive selected quantity, bounded against non-cancelled RMA capacity under order lock.';
COMMENT ON COLUMN returns.buyer_request_receipts.tenant_id IS 'Authenticated tenant scope; forced RLS key.';
COMMENT ON COLUMN returns.buyer_request_receipts.store_id IS 'Authenticated store scope; forced RLS key.';
COMMENT ON COLUMN returns.buyer_request_receipts.actor IS 'Server-resolved buyer owner or merchant principal namespace; prevents cross-actor replay.';
COMMENT ON COLUMN returns.buyer_request_receipts.operation IS 'Create, withdraw, approve or reject operation namespace.';
COMMENT ON COLUMN returns.buyer_request_receipts.idempotency_key IS 'Caller intent key within authenticated actor/operation; no automatic mutation retry.';
COMMENT ON COLUMN returns.buyer_request_receipts.request_hash IS 'Canonical JSON SHA-256 target/body/version binding.';
COMMENT ON COLUMN returns.buyer_request_receipts.response IS 'Original safe response snapshot; no contact/authority fields.';
COMMENT ON COLUMN returns.buyer_request_receipts.created_at IS 'Receipt commit transaction time; retained with application.';
