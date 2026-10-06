-- 0155_returns.sql — W3-08B (unit w3-08b-returns): merchant-side minimal returns (RMA) and merchant order cancel.
-- Purpose: returns.rmas / returns.rma_lines and the definers that register, receive, inspect, close and cancel an RMA, plus
--   fulfillment.merchant_cancel_order (cancel an unshipped order). Stock moves ONLY through the inventory ledger: an RMA line
--   disposed 'sellable' releases its units from `allocated` (DEALLOCATE, operation returns.rma.restock); a merchant cancel of an unshipped
--   paid order releases the allocation (DEALLOCATE) and of an unpaid hold releases the reservation (RELEASE), both operation
--   checkout.merchant_cancel. A refund never writes the ledger (RD6); a return never refunds (I13); nothing here writes payments.*.
-- Depends on: 0002/0013/0018/0072/0088/0107 (inventory.ledger, its constraints and guard triggers, checkout.orders, reservations),
--   0062 (payments.stripe_refunds, ops.command_results policies), 0063 (manual shipment), 0073 (fulfillment.settle_cvs_attempt),
--   0107 (inventory.release_pay_at_pickup), 0146 (fulfillment.parcel_groups / parcel_group_orders), identity.resolve_access,
--   roles commerce_runtime and commerce_checkout_writer.
-- Used by: internal/returns (Go wrappers of the returns.* definers), internal/fulfillment/cancel.go (merchant_cancel_order),
--   internal/httpapi/returns.go (routes). Tests: TestReturns*, TestMerchantCancel* (tests/foundation/returns_test.go), TestWAS02,
--   merchant_orders_v2_acl_test, manual_fulfilment_schema_test.
-- Invariants: I03 (every stock change is a ledger row with provenance guard inventory.guard_returns_ledger: no double restock,
--   never more than is allocated), I05 (no payment write), I13 (refund, return, restock and cancel stay separate), I02 (every command
--   carries an Idempotency-Key replayed from ops.command_results). Every definer is SECURITY DEFINER SET search_path=pg_catalog, REVOKE
--   ALL FROM PUBLIC, EXECUTE commerce_runtime only, owner commerce_checkout_writer.
-- Deviations from the brief (decided by the implementer, see contracts/returns-v1.md): sellable stock is a DEALLOCATE of `allocated`
--   (shipping never lowered on_hand, so crediting on_hand would double count; same model as the §16.8 pay-at-pickup restock);
--   RMA lines are keyed (warehouse_id, sku_id) because an order has no line-id table (its lines are inventory.reservation_lines);
--   the cancel definer lives in schema fulfillment (commerce_runtime has no USAGE on schema checkout); order has no version column, so
--   the cancel CAS is expected_state (the commercial state the merchant saw).

DO $$
BEGIN
 IF to_regclass('fulfillment.parcel_group_orders') IS NULL
  OR to_regprocedure('inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text)') IS NULL
  OR to_regprocedure('fulfillment.settle_cvs_attempt(uuid,uuid,uuid)') IS NULL THEN
  RAISE EXCEPTION '0155 requires 0107 (release_pay_at_pickup), 0073 (settle_cvs_attempt) and 0146 (parcel groups)'; END IF;
END $$;

CREATE SCHEMA returns AUTHORIZATION commerce_checkout_writer;
REVOKE ALL ON SCHEMA returns FROM PUBLIC;
GRANT USAGE ON SCHEMA returns TO commerce_runtime;

-- ---------------------------------------------------------------------------------------------------
-- Tables. Go never SELECTs them: every read goes through a definer (parcel-group pattern).
-- ---------------------------------------------------------------------------------------------------
CREATE TABLE returns.rmas (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL, owner_id uuid NOT NULL, order_id uuid NOT NULL,
 state text NOT NULL CHECK(state IN ('REGISTERED','RECEIVED','INSPECTED','CLOSED','CANCELLED')),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 240),
 refund_id uuid,
 created_by uuid NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 received_by uuid, received_at timestamptz, inspected_by uuid, inspected_at timestamptz,
 closed_by uuid, closed_at timestamptz, cancelled_by uuid, cancelled_at timestamptz,
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,id),
 UNIQUE(tenant_id,store_id,id,order_id),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES identity.memberships(tenant_id,principal_id),
 FOREIGN KEY(tenant_id,received_by) REFERENCES identity.memberships(tenant_id,principal_id),
 FOREIGN KEY(tenant_id,inspected_by) REFERENCES identity.memberships(tenant_id,principal_id),
 FOREIGN KEY(tenant_id,closed_by) REFERENCES identity.memberships(tenant_id,principal_id),
 FOREIGN KEY(tenant_id,cancelled_by) REFERENCES identity.memberships(tenant_id,principal_id),
 -- the state implies which stage columns are set, so a row can never claim CLOSED without having been received and inspected
 CHECK(state NOT IN ('RECEIVED','INSPECTED','CLOSED') OR (received_at IS NOT NULL AND received_by IS NOT NULL)),
 CHECK(state NOT IN ('INSPECTED','CLOSED') OR (inspected_at IS NOT NULL AND inspected_by IS NOT NULL)),
 CHECK(state<>'CLOSED' OR (closed_at IS NOT NULL AND closed_by IS NOT NULL)),
 CHECK(state<>'CANCELLED' OR (cancelled_at IS NOT NULL AND cancelled_by IS NOT NULL)),
 CHECK(refund_id IS NULL OR state='CLOSED')
);
CREATE INDEX rmas_order ON returns.rmas(tenant_id,store_id,order_id,created_at,id);
CREATE INDEX rmas_state ON returns.rmas(tenant_id,store_id,state,created_at DESC,id);

-- One row per (order reservation line): an order has no line-id table, its lines are inventory.reservation_lines. The FK ties every
-- RMA line to a real, allocated line of ITS OWN order; qty_restock + qty_scrap = qty_received <= qty_registered once inspected.
CREATE TABLE returns.rma_lines (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, rma_id uuid NOT NULL, order_id uuid NOT NULL,
 warehouse_id uuid NOT NULL, sku_id uuid NOT NULL,
 qty_registered bigint NOT NULL CHECK(qty_registered BETWEEN 1 AND 1000000000),
 qty_received bigint CHECK(qty_received BETWEEN 0 AND 1000000000),
 qty_restock bigint CHECK(qty_restock BETWEEN 0 AND 1000000000),
 qty_scrap bigint CHECK(qty_scrap BETWEEN 0 AND 1000000000),
 PRIMARY KEY(tenant_id,store_id,rma_id,warehouse_id,sku_id),
 FOREIGN KEY(tenant_id,store_id,rma_id,order_id) REFERENCES returns.rmas(tenant_id,store_id,id,order_id),
 FOREIGN KEY(tenant_id,store_id,order_id,warehouse_id,sku_id)
  REFERENCES inventory.reservation_lines(tenant_id,store_id,reservation_id,warehouse_id,sku_id),
 CHECK(qty_received IS NULL OR qty_received<=qty_registered),
 CHECK((qty_restock IS NULL)=(qty_scrap IS NULL)),
 CHECK(qty_restock IS NULL OR (qty_received IS NOT NULL AND qty_restock+qty_scrap=qty_received))
);
CREATE INDEX rma_lines_order_sku ON returns.rma_lines(tenant_id,store_id,order_id,warehouse_id,sku_id);

ALTER TABLE returns.rmas ENABLE ROW LEVEL SECURITY;
ALTER TABLE returns.rmas FORCE ROW LEVEL SECURITY;
ALTER TABLE returns.rma_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE returns.rma_lines FORCE ROW LEVEL SECURITY;
REVOKE ALL ON returns.rmas, returns.rma_lines FROM PUBLIC;
GRANT SELECT,INSERT ON returns.rmas, returns.rma_lines TO commerce_checkout_writer;
GRANT UPDATE(state,version,refund_id,received_by,received_at,inspected_by,inspected_at,closed_by,closed_at,cancelled_by,cancelled_at,updated_at)
 ON returns.rmas TO commerce_checkout_writer;
GRANT UPDATE(qty_received,qty_restock,qty_scrap) ON returns.rma_lines TO commerce_checkout_writer;
CREATE POLICY rmas_writer ON returns.rmas TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY rma_lines_writer ON returns.rma_lines TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
COMMENT ON TABLE returns.rmas IS 'W3-08B: one merchant-registered return of a SHIPPED order (REGISTERED -> RECEIVED -> INSPECTED -> CLOSED, REGISTERED -> CANCELLED). Written only by the returns.* definers; commerce_runtime has no direct grant. Never a refund (I13).';
COMMENT ON TABLE returns.rma_lines IS 'W3-08B: per (warehouse, sku) quantities of an RMA: registered <= shipped quantity of that order line (summed over live RMAs), received, restock (back to sellable) and scrap (recorded only). Closing writes one DEALLOCATE ledger row per line with qty_restock>0.';

-- ---------------------------------------------------------------------------------------------------
-- inventory.ledger: new MERCHANT row shapes (DEALLOCATE for RMA restock and paid-order cancel, RELEASE for an unpaid-hold cancel).
-- The unique index that allowed ONE DEALLOCATE per order line is relaxed ONLY for RMA restocks (several partial returns of one line).
-- ---------------------------------------------------------------------------------------------------
DO $$
DECLARE v_def text; v_anchor text:=' OR ((actor_kind = ''SYSTEM_PAYMENT''::text)'; v_fn text; v_from text; v_to text;
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c WHERE c.conrelid='inventory.ledger'::regclass AND c.conname='ledger_checkout_actor';
 IF v_def IS NULL OR v_def LIKE '%returns.rma.restock%' OR length(v_def)-length(replace(v_def,v_anchor,''))<>length(v_anchor) THEN
  RAISE EXCEPTION 'inventory.ledger ledger_checkout_actor has an unexpected shape: %',v_def; END IF;
 ALTER TABLE inventory.ledger DROP CONSTRAINT ledger_checkout_actor;
 EXECUTE format('ALTER TABLE inventory.ledger ADD CONSTRAINT ledger_checkout_actor %s',replace(v_def,v_anchor,
  ' OR ((actor_kind = ''MERCHANT''::text) AND (kind = ''DEALLOCATE''::text) AND (principal_id IS NOT NULL)'||
  ' AND (checkout_id IS NOT NULL) AND (buyer_owner_id IS NOT NULL) AND (buyer_session_id IS NOT NULL)'||
  ' AND (reservation_id = checkout_id) AND (payment_attempt_id IS NULL) AND (payment_fact_kind IS NULL)'||
  ' AND (((operation = ''checkout.merchant_cancel''::text) AND (command_key = (checkout_id)::text))'||
  ' OR (operation = ''returns.rma.restock''::text)))'||
  ' OR ((actor_kind = ''MERCHANT''::text) AND (kind = ''RELEASE''::text) AND (principal_id IS NOT NULL)'||
  ' AND (checkout_id IS NOT NULL) AND (buyer_owner_id IS NOT NULL) AND (buyer_session_id IS NOT NULL)'||
  ' AND (reservation_id = checkout_id) AND (payment_attempt_id IS NULL) AND (payment_fact_kind IS NULL)'||
  ' AND (operation = ''checkout.merchant_cancel''::text) AND (command_key = (checkout_id)::text))'||v_anchor));

 -- guard_checkout_ledger (0013/0072, widened by 0088): the "merchant row with checkout columns" branch also covers the cancel RELEASE.
 v_fn:=pg_get_functiondef('inventory.guard_checkout_ledger()'::regprocedure);
 v_from:='NEW.actor_kind=''MERCHANT'' AND NEW.kind IN (''DEALLOCATE'',''ALLOCATE'')';
 v_to:='NEW.actor_kind=''MERCHANT'' AND (NEW.kind IN (''DEALLOCATE'',''ALLOCATE'') OR (NEW.kind=''RELEASE'' AND NEW.operation=''checkout.merchant_cancel''))';
 IF length(v_fn)-length(replace(v_fn,v_from,''))<>length(v_from) THEN
  RAISE EXCEPTION 'inventory.guard_checkout_ledger has an unexpected shape'; END IF;
 EXECUTE replace(v_fn,v_from,v_to);

 -- guard_pay_at_pickup_ledger (0107): its catch-all "MERCHANT DEALLOCATE" branch is the pay-at-pickup release; it must not fire for
 -- the two operations owned by guard_returns_ledger below.
 v_fn:=pg_get_functiondef('inventory.guard_pay_at_pickup_ledger()'::regprocedure);
 v_from:='ELSIF NEW.actor_kind=''MERCHANT'' AND NEW.kind=''DEALLOCATE'' THEN';
 v_to:='ELSIF NEW.actor_kind=''MERCHANT'' AND NEW.kind=''DEALLOCATE'' AND NEW.operation IN (''fulfillment.pay_at_pickup.cancel'',''fulfillment.pay_at_pickup.restock'') THEN';
 IF length(v_fn)-length(replace(v_fn,v_from,''))<>length(v_from) THEN
  RAISE EXCEPTION 'inventory.guard_pay_at_pickup_ledger has an unexpected shape'; END IF;
 EXECUTE replace(v_fn,v_from,v_to);
END $$;

CREATE POLICY checkout_writer_merchant_cancel_release ON inventory.ledger FOR INSERT TO commerce_checkout_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND actor_kind='MERCHANT' AND kind='RELEASE' AND principal_id IS NOT NULL AND operation='checkout.merchant_cancel'
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND buyer_session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);

DROP INDEX inventory.ledger_pay_at_pickup_release_once;
CREATE UNIQUE INDEX ledger_pay_at_pickup_release_once ON inventory.ledger(tenant_id,store_id,checkout_id,warehouse_id,sku_id)
 WHERE kind='DEALLOCATE' AND operation<>'returns.rma.restock';
COMMENT ON INDEX inventory.ledger_pay_at_pickup_release_once IS '§16.8 + W3-08B: at most one DEALLOCATE ledger row per order line, whatever the idempotency key, EXCEPT returns.rma.restock rows (partial returns of one line; their total is bounded by inventory.guard_returns_ledger and unique per (rma, line)).';

-- Provenance of the two new operations. Any row carrying either operation name must be the exact shape the definers write, whoever
-- inserts it (a direct INSERT by another authority is refused here). BEFORE INSERT, so the definers update the order/RMA/reservation
-- state first and the guard reads the new state (same order as the pay-at-pickup release).
CREATE FUNCTION inventory.guard_returns_ledger() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_line bigint; v_alloc bigint; v_qty bigint;
BEGIN
 IF NEW.operation NOT IN ('returns.rma.restock','checkout.merchant_cancel') THEN RETURN NEW; END IF;
 IF NEW.actor_kind<>'MERCHANT' OR NEW.checkout_id IS NULL OR NEW.reservation_id IS DISTINCT FROM NEW.checkout_id
  OR NEW.principal_id IS NULL OR NEW.kind NOT IN ('DEALLOCATE','RELEASE') THEN
  RAISE EXCEPTION 'returns ledger mismatch' USING ERRCODE='42501'; END IF;
 -- the order lines of THIS reservation: quantity of the line and what the ledger still holds allocated for it
 SELECT l.quantity INTO v_line FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id AND l.store_id=NEW.store_id
  AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id AND l.sku_id=NEW.sku_id;
 SELECT coalesce(sum(a.delta_allocated),0) INTO v_alloc FROM inventory.ledger a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
  AND a.checkout_id=NEW.checkout_id AND a.warehouse_id=NEW.warehouse_id AND a.sku_id=NEW.sku_id AND a.kind IN ('ALLOCATE','DEALLOCATE');
 IF NEW.operation='returns.rma.restock' THEN
  IF NEW.kind<>'DEALLOCATE' OR NEW.reason<>'rma_restock' OR NEW.delta_allocated>=0
   OR NEW.command_key !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
   RAISE EXCEPTION 'rma restock ledger mismatch' USING ERRCODE='42501'; END IF;
  -- CLOSED RMA of this order whose line says exactly this quantity is sellable, closed by this principal; order still CONFIRMED with a
  -- COMMITTED reservation (a cancelled/released order has nothing allocated to give back); never more than is still allocated.
  IF NOT EXISTS(SELECT 1 FROM returns.rmas r
    JOIN returns.rma_lines l ON l.tenant_id=r.tenant_id AND l.store_id=r.store_id AND l.rma_id=r.id
    JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.owner_id=r.owner_id AND o.id=r.order_id
    JOIN inventory.reservations v ON v.tenant_id=o.tenant_id AND v.store_id=o.store_id AND v.id=o.id
    WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id AND r.id=NEW.command_key::uuid AND r.order_id=NEW.checkout_id
     AND r.state='CLOSED' AND r.closed_by=NEW.principal_id AND l.warehouse_id=NEW.warehouse_id AND l.sku_id=NEW.sku_id
     AND l.qty_restock=-NEW.delta_allocated AND o.commercial_state='CONFIRMED' AND o.owner_id=NEW.buyer_owner_id
     AND o.creator_session_id=NEW.buyer_session_id AND v.state='COMMITTED') OR v_alloc<-NEW.delta_allocated THEN
   RAISE EXCEPTION 'rma restock ledger mismatch' USING ERRCODE='42501'; END IF;
  RETURN NEW;
 END IF;
 -- checkout.merchant_cancel: the order is already CANCELLED/CANCELLED and the reservation RELEASED (the definer wrote both first), for the
 -- exact line quantity, command_key = order id.
 IF NEW.reason<>'merchant_cancel' OR NEW.command_key<>NEW.checkout_id::text OR NOT EXISTS(SELECT 1 FROM checkout.orders o
   JOIN inventory.reservations v ON v.tenant_id=o.tenant_id AND v.store_id=o.store_id AND v.id=o.id
   WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id AND o.id=NEW.checkout_id AND o.owner_id=NEW.buyer_owner_id
    AND o.creator_session_id=NEW.buyer_session_id AND o.commercial_state='CANCELLED' AND o.fulfillment_state='CANCELLED'
    AND v.state='RELEASED') THEN
  RAISE EXCEPTION 'merchant cancel ledger mismatch' USING ERRCODE='42501'; END IF;
 IF NEW.kind='RELEASE' THEN
  -- unpaid hold: reserved goes down by exactly the line quantity; the order never had a payment attempt
  IF NEW.delta_reserved>=0 OR v_line IS DISTINCT FROM -NEW.delta_reserved OR EXISTS(SELECT 1 FROM checkout.payment_attempts a
    WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id AND a.order_id=NEW.checkout_id) THEN
   RAISE EXCEPTION 'merchant cancel ledger mismatch' USING ERRCODE='42501'; END IF;
  RETURN NEW;
 END IF;
 -- paid card order: the whole remaining allocation of the line goes back, only after a CAPTURED fact and refunds (succeeded + in flight)
 -- covering the whole capture (stripe-refund-v1 RD6: the refund itself never released stock; this cancel does).
 IF NEW.delta_allocated>=0 OR v_line IS DISTINCT FROM -NEW.delta_allocated OR v_alloc IS DISTINCT FROM v_line
  OR NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id AND o.id=NEW.checkout_id
    AND o.payment_mode='card')
  OR NOT EXISTS(SELECT 1 FROM checkout.payment_attempts a JOIN payments.facts f ON f.tenant_id=a.tenant_id AND f.store_id=a.store_id
    AND f.attempt_id=a.id AND f.kind='CAPTURED' AND f.amount_minor=a.amount_minor
    WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id AND a.order_id=NEW.checkout_id
     AND (SELECT coalesce(sum(r.amount_minor),0) FROM payments.stripe_refunds r WHERE r.tenant_id=a.tenant_id AND r.store_id=a.store_id
       AND r.attempt_id=a.id AND NOT EXISTS(SELECT 1 FROM payments.refund_facts rf WHERE rf.tenant_id=r.tenant_id AND rf.store_id=r.store_id
        AND rf.refund_id=r.id AND rf.kind IN ('FAILED','CANCELED','REJECTED')))>=f.amount_minor) THEN
  RAISE EXCEPTION 'merchant cancel ledger mismatch' USING ERRCODE='42501'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inventory.guard_returns_ledger() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.guard_returns_ledger() FROM PUBLIC;
CREATE TRIGGER zz_returns_ledger_guard BEFORE INSERT ON inventory.ledger
 FOR EACH ROW EXECUTE FUNCTION inventory.guard_returns_ledger();
COMMENT ON FUNCTION inventory.guard_returns_ledger() IS 'inventory trigger guard (BEFORE INSERT on ledger): the only rows with operation returns.rma.restock (DEALLOCATE of a CLOSED RMA line for exactly qty_restock, never more than still allocated, order CONFIRMED + reservation COMMITTED) or checkout.merchant_cancel (RELEASE of an unpaid hold, or DEALLOCATE of the whole allocation of a paid card order whose CAPTURED amount is covered by succeeded + in-flight refunds), each after the definer set order/RMA/reservation state. No caller EXECUTE.';

-- ---------------------------------------------------------------------------------------------------
-- Helpers (no grants: only the definers below, same owner, call them).
-- ---------------------------------------------------------------------------------------------------
-- Authorize before any lock; the GUCs then come from this result, never from the caller. p_perm2 (inventory:write for the stock-deciding
-- steps) must also be held by the same principal. Returns {t: tenant, p: principal, r: authz_revision}.
CREATE FUNCTION returns.authorize(p_hash bytea,p_store uuid,p_perm text,p_perm2 text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SET search_path=pg_catalog AS $$
DECLARE s record;
BEGIN
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,p_perm);
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF p_perm2 IS NOT NULL THEN
  IF (SELECT x.access_status FROM identity.resolve_access(p_hash,p_store,p_perm2) x) IS DISTINCT FROM 'ok' THEN
   RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 RETURN jsonb_build_object('t',s.tenant_id,'p',s.principal_id,'r',s.authz_revision);
END $$;
ALTER FUNCTION returns.authorize(bytea,uuid,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.authorize(bytea,uuid,text,text) FROM PUBLIC;
COMMENT ON FUNCTION returns.authorize(bytea,uuid,text,text) IS 'W3-08B internal helper (no grants): identity.resolve_access for one or two permissions, then the tx-local tenant/store/principal GUCs. Returns {t,p,r}.';

-- Final re-check after the writes (same session, same principal, same authorization revision, session not expired).
CREATE FUNCTION returns.reauthorize(p_hash bytea,p_store uuid,p_perm text,p_perm2 text,p_auth jsonb) RETURNS void
LANGUAGE plpgsql VOLATILE SET search_path=pg_catalog AS $$
DECLARE f record; v_expiry timestamptz;
BEGIN
 SELECT * INTO f FROM identity.resolve_access(p_hash,p_store,p_perm);
 SELECT se.expires_at INTO v_expiry FROM identity.sessions se WHERE se.token_hash=p_hash AND se.audience='merchant' AND se.revoked_at IS NULL;
 IF f.access_status='unauthorized' OR v_expiry IS NULL OR v_expiry<=clock_timestamp() THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF f.access_status<>'ok' OR f.tenant_id::text IS DISTINCT FROM (p_auth->>'t') OR f.principal_id::text IS DISTINCT FROM (p_auth->>'p')
  OR f.authz_revision::text IS DISTINCT FROM (p_auth->>'r') THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF p_perm2 IS NOT NULL AND (SELECT x.access_status FROM identity.resolve_access(p_hash,p_store,p_perm2) x) IS DISTINCT FROM 'ok' THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
END $$;
ALTER FUNCTION returns.reauthorize(bytea,uuid,text,text,jsonb) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.reauthorize(bytea,uuid,text,text,jsonb) FROM PUBLIC;
COMMENT ON FUNCTION returns.reauthorize(bytea,uuid,text,text,jsonb) IS 'W3-08B internal helper (no grants): re-resolves the access of authorize() after the writes; PT401/PT403 when the session, principal or authorization revision changed.';

-- Shape check of a request line array [{sku_id, warehouse_id|null, <p_a>, <p_b>}], 1..50 lines. Malformed = PT400.
CREATE FUNCTION returns.line_set(p_lines jsonb,p_a text,p_b text) RETURNS TABLE(l_sku uuid,l_wh uuid,l_a bigint,l_b bigint)
LANGUAGE plpgsql IMMUTABLE SET search_path=pg_catalog AS $$
DECLARE e jsonb; v_uuid text:='^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
BEGIN
 IF jsonb_typeof(p_lines) IS DISTINCT FROM 'array' THEN RAISE EXCEPTION 'invalid returns request' USING ERRCODE='PT400'; END IF;
 IF jsonb_array_length(p_lines) NOT BETWEEN 1 AND 50 THEN RAISE EXCEPTION 'invalid returns request' USING ERRCODE='PT400'; END IF;
 FOR e IN SELECT t.x FROM jsonb_array_elements(p_lines) AS t(x) LOOP
  IF jsonb_typeof(e) IS DISTINCT FROM 'object' OR coalesce(e->>'sku_id','') !~ v_uuid
   OR (e ? 'warehouse_id' AND jsonb_typeof(e->'warehouse_id') IS DISTINCT FROM 'null' AND coalesce(e->>'warehouse_id','') !~ v_uuid)
   OR jsonb_typeof(e->p_a) IS DISTINCT FROM 'number' OR coalesce(e->>p_a,'') !~ '^[0-9]{1,9}$'
   OR (p_b IS NOT NULL AND (jsonb_typeof(e->p_b) IS DISTINCT FROM 'number' OR coalesce(e->>p_b,'') !~ '^[0-9]{1,9}$')) THEN
   RAISE EXCEPTION 'invalid returns request' USING ERRCODE='PT400'; END IF;
  l_sku:=(e->>'sku_id')::uuid; l_wh:=(e->>'warehouse_id')::uuid; l_a:=(e->>p_a)::bigint;
  l_b:=CASE WHEN p_b IS NULL THEN NULL ELSE (e->>p_b)::bigint END;
  RETURN NEXT;
 END LOOP;
END $$;
ALTER FUNCTION returns.line_set(jsonb,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.line_set(jsonb,text,text) FROM PUBLIC;
COMMENT ON FUNCTION returns.line_set(jsonb,text,text) IS 'W3-08B internal helper (no grants): validates and unnests the request line array (sku_id, optional warehouse_id, one or two bounded integer fields).';

-- The RMA projection every command returns and the reads list.
CREATE FUNCTION returns.rma_json(p_tenant uuid,p_store uuid,p_rma uuid) RETURNS jsonb
LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
 SELECT jsonb_build_object('id',r.id,'order_id',r.order_id,'state',r.state,'version',r.version,'reason',r.reason,'refund_id',r.refund_id,
  'created_at',r.created_at,'updated_at',r.updated_at,
  'lines',coalesce((SELECT jsonb_agg(jsonb_build_object('warehouse_id',l.warehouse_id,'sku_id',l.sku_id,'qty_registered',l.qty_registered,
    'qty_received',l.qty_received,'qty_restock',l.qty_restock,'qty_scrap',l.qty_scrap) ORDER BY l.warehouse_id,l.sku_id)
   FROM returns.rma_lines l WHERE l.tenant_id=r.tenant_id AND l.store_id=r.store_id AND l.rma_id=r.id),'[]'::jsonb))
 FROM returns.rmas r WHERE r.tenant_id=p_tenant AND r.store_id=p_store AND r.id=p_rma
$$;
ALTER FUNCTION returns.rma_json(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.rma_json(uuid,uuid,uuid) FROM PUBLIC;
COMMENT ON FUNCTION returns.rma_json(uuid,uuid,uuid) IS 'W3-08B internal helper (no grants): the RMA + lines projection (SECURITY INVOKER, runs under the calling definer and its tenant/store GUCs).';

-- ---------------------------------------------------------------------------------------------------
-- returns.register_rma (fulfillment:write, Idempotency-Key): a SHIPPED order only; quantity per line <= shipped quantity minus live RMAs.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION returns.register_rma(p_hash bytea,p_store uuid,p_order uuid,p_key text,p_request_hash bytea,p_reason text,p_lines jsonb)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_auth jsonb; v_t uuid; v_p uuid; o record; x record; l record; v_saved bytea; v_response jsonb; v_fail_code text; v_fail_msg text;
 v_rma uuid:=gen_random_uuid(); v_already bigint; v_n int;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_reason IS NULL OR length(btrim(p_reason)) NOT BETWEEN 1 AND 240
  OR p_lines IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid returns request' USING ERRCODE='PT400'; END IF;
 v_auth:=returns.authorize(p_hash,p_store,'fulfillment:write',NULL);
 v_t:=(v_auth->>'t')::uuid; v_p:=(v_auth->>'p')::uuid;
 PERFORM 1 FROM returns.line_set(p_lines,'quantity',NULL);  -- shape check (PT400) before any lock
 <<work>>
 BEGIN
  SELECT k.owner_id,k.commercial_state,k.fulfillment_state,k.collection_state INTO o FROM checkout.orders k
   WHERE k.tenant_id=v_t AND k.store_id=p_store AND k.id=p_order FOR UPDATE;  -- the order lock serializes registrations of one order
  IF NOT FOUND THEN v_fail_code:='PT404'; v_fail_msg:='order not found'; EXIT work; END IF;
  SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=v_t AND c.store_id=p_store
   AND c.operation='returns.rma.register' AND c.idempotency_key=p_key;
  IF FOUND THEN
   IF v_saved<>p_request_hash THEN v_fail_code:='PT409'; v_fail_msg:='idempotency_conflict'; END IF;
   EXIT work;
  END IF;
  -- shipped: a manual shipment, or a CVS parcel the buyer picked up; paid-on-delivery orders only once collected
  IF NOT (o.fulfillment_state='MERCHANT_SHIPPED' OR (o.fulfillment_state='PROVIDER_LABEL_CREATED' AND EXISTS(
    SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=v_t AND c.store_id=p_store AND c.order_id=p_order AND c.state='PICKED_UP'))) THEN
   v_fail_code:='PT409'; v_fail_msg:='not_shipped'; EXIT work; END IF;
  IF o.commercial_state<>'CONFIRMED' OR (o.collection_state IS NOT NULL AND o.collection_state<>'COLLECTED') THEN
   v_fail_code:='PT409'; v_fail_msg:='not_returnable'; EXIT work; END IF;
  INSERT INTO returns.rmas(tenant_id,store_id,id,owner_id,order_id,state,reason,created_by)
   VALUES(v_t,p_store,v_rma,o.owner_id,p_order,'REGISTERED',btrim(p_reason),v_p);
  FOR x IN SELECT * FROM returns.line_set(p_lines,'quantity',NULL) LOOP
   SELECT count(*)::int INTO v_n FROM inventory.reservation_lines rl WHERE rl.tenant_id=v_t AND rl.store_id=p_store
    AND rl.reservation_id=p_order AND rl.sku_id=x.l_sku AND (x.l_wh IS NULL OR rl.warehouse_id=x.l_wh);
   IF v_n=0 THEN v_fail_code:='PT422'; v_fail_msg:='unknown_line'; EXIT work; END IF;
   IF v_n>1 THEN v_fail_code:='PT422'; v_fail_msg:='ambiguous_line'; EXIT work; END IF;
   SELECT rl.warehouse_id,rl.quantity INTO l FROM inventory.reservation_lines rl WHERE rl.tenant_id=v_t AND rl.store_id=p_store
    AND rl.reservation_id=p_order AND rl.sku_id=x.l_sku AND (x.l_wh IS NULL OR rl.warehouse_id=x.l_wh);
   IF x.l_a<1 OR EXISTS(SELECT 1 FROM returns.rma_lines d WHERE d.tenant_id=v_t AND d.store_id=p_store AND d.rma_id=v_rma
     AND d.warehouse_id=l.warehouse_id AND d.sku_id=x.l_sku) THEN
    v_fail_code:='PT422'; v_fail_msg:='invalid_quantities'; EXIT work; END IF;
   SELECT coalesce(sum(il.qty_registered),0) INTO v_already FROM returns.rma_lines il JOIN returns.rmas ir
    ON ir.tenant_id=il.tenant_id AND ir.store_id=il.store_id AND ir.id=il.rma_id
    WHERE il.tenant_id=v_t AND il.store_id=p_store AND il.order_id=p_order AND il.warehouse_id=l.warehouse_id AND il.sku_id=x.l_sku
     AND ir.state<>'CANCELLED' AND ir.id<>v_rma;
   IF v_already+x.l_a>l.quantity THEN v_fail_code:='PT422'; v_fail_msg:='exceeds_shipped'; EXIT work; END IF;
   INSERT INTO returns.rma_lines(tenant_id,store_id,rma_id,order_id,warehouse_id,sku_id,qty_registered)
    VALUES(v_t,p_store,v_rma,p_order,l.warehouse_id,x.l_sku,x.l_a);
  END LOOP;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_t,p_store,v_p,'returns.rma_registered');
  v_response:=returns.rma_json(v_t,p_store,v_rma);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(v_t,p_store,'returns.rma.register',p_key,p_request_hash,v_response,v_p);
 END work;
 PERFORM returns.reauthorize(p_hash,p_store,'fulfillment:write',NULL,v_auth);
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION returns.register_rma(bytea,uuid,uuid,text,bytea,text,jsonb) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.register_rma(bytea,uuid,uuid,text,bytea,text,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION returns.register_rma(bytea,uuid,uuid,text,bytea,text,jsonb) TO commerce_runtime;
COMMENT ON FUNCTION returns.register_rma(bytea,uuid,uuid,text,bytea,text,jsonb) IS 'internal/returns Register only; EXECUTE commerce_runtime. fulfillment:write, Idempotency-Key (ops.command_results returns.rma.register). Locks the order; refuses not_shipped / not_returnable (409), unknown_line / ambiguous_line / exceeds_shipped / invalid_quantities (422). Writes the RMA, its lines, one audit row; never stock, never payments.';

-- ---------------------------------------------------------------------------------------------------
-- returns.receive_rma / inspect_rma / cancel_rma: CAS on version, one transition each, all lines named exactly once.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION returns.receive_rma(p_hash bytea,p_store uuid,p_rma uuid,p_key text,p_request_hash bytea,p_expected_version bigint,p_lines jsonb)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_auth jsonb; v_t uuid; v_p uuid; rr record; x record; l record; v_saved bytea; v_response jsonb; v_fail_code text; v_fail_msg text;
 v_n int; v_sum bigint:=0; v_total int; v_done int;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_rma IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_expected_version IS NULL OR p_expected_version<1 OR p_lines IS NULL
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid returns request' USING ERRCODE='PT400'; END IF;
 v_auth:=returns.authorize(p_hash,p_store,'fulfillment:write',NULL);
 v_t:=(v_auth->>'t')::uuid; v_p:=(v_auth->>'p')::uuid;
 PERFORM 1 FROM returns.line_set(p_lines,'qty_received',NULL);
 <<work>>
 BEGIN
  SELECT r.state,r.version INTO rr FROM returns.rmas r WHERE r.tenant_id=v_t AND r.store_id=p_store AND r.id=p_rma FOR UPDATE;
  IF NOT FOUND THEN v_fail_code:='PT404'; v_fail_msg:='rma not found'; EXIT work; END IF;
  SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=v_t AND c.store_id=p_store
   AND c.operation='returns.rma.receive' AND c.idempotency_key=p_key;
  IF FOUND THEN
   IF v_saved<>p_request_hash THEN v_fail_code:='PT409'; v_fail_msg:='idempotency_conflict'; END IF;
   EXIT work;
  END IF;
  IF rr.version<>p_expected_version THEN v_fail_code:='PT409'; v_fail_msg:='version_changed'; EXIT work; END IF;
  IF rr.state<>'REGISTERED' THEN v_fail_code:='PT409'; v_fail_msg:='invalid_state'; EXIT work; END IF;
  FOR x IN SELECT * FROM returns.line_set(p_lines,'qty_received',NULL) LOOP
   SELECT count(*)::int INTO v_n FROM returns.rma_lines d WHERE d.tenant_id=v_t AND d.store_id=p_store AND d.rma_id=p_rma
    AND d.sku_id=x.l_sku AND (x.l_wh IS NULL OR d.warehouse_id=x.l_wh);
   IF v_n=0 THEN v_fail_code:='PT422'; v_fail_msg:='unknown_line'; EXIT work; END IF;
   IF v_n>1 THEN v_fail_code:='PT422'; v_fail_msg:='ambiguous_line'; EXIT work; END IF;
   SELECT d.warehouse_id,d.qty_registered,d.qty_received INTO l FROM returns.rma_lines d WHERE d.tenant_id=v_t AND d.store_id=p_store
    AND d.rma_id=p_rma AND d.sku_id=x.l_sku AND (x.l_wh IS NULL OR d.warehouse_id=x.l_wh);
   IF l.qty_received IS NOT NULL THEN v_fail_code:='PT422'; v_fail_msg:='invalid_quantities'; EXIT work; END IF;  -- the same line twice
   IF x.l_a>l.qty_registered THEN v_fail_code:='PT422'; v_fail_msg:='exceeds_registered'; EXIT work; END IF;
   UPDATE returns.rma_lines SET qty_received=x.l_a WHERE tenant_id=v_t AND store_id=p_store AND rma_id=p_rma
    AND warehouse_id=l.warehouse_id AND sku_id=x.l_sku;
   v_sum:=v_sum+x.l_a;
  END LOOP;
  SELECT count(*)::int,count(d.qty_received)::int INTO v_total,v_done FROM returns.rma_lines d WHERE d.tenant_id=v_t AND d.store_id=p_store AND d.rma_id=p_rma;
  IF v_total<>v_done THEN v_fail_code:='PT422'; v_fail_msg:='lines_incomplete'; EXIT work; END IF;
  IF v_sum=0 THEN v_fail_code:='PT422'; v_fail_msg:='nothing_received'; EXIT work; END IF;
  UPDATE returns.rmas SET state='RECEIVED',version=version+1,received_by=v_p,received_at=clock_timestamp(),updated_at=clock_timestamp()
   WHERE tenant_id=v_t AND store_id=p_store AND id=p_rma;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_t,p_store,v_p,'returns.rma_received');
  v_response:=returns.rma_json(v_t,p_store,p_rma);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(v_t,p_store,'returns.rma.receive',p_key,p_request_hash,v_response,v_p);
 END work;
 PERFORM returns.reauthorize(p_hash,p_store,'fulfillment:write',NULL,v_auth);
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION returns.receive_rma(bytea,uuid,uuid,text,bytea,bigint,jsonb) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.receive_rma(bytea,uuid,uuid,text,bytea,bigint,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION returns.receive_rma(bytea,uuid,uuid,text,bytea,bigint,jsonb) TO commerce_runtime;
COMMENT ON FUNCTION returns.receive_rma(bytea,uuid,uuid,text,bytea,bigint,jsonb) IS 'internal/returns Receive only; EXECUTE commerce_runtime. fulfillment:write, Idempotency-Key, CAS on version. REGISTERED -> RECEIVED with qty_received per line (every line named once, <= registered, at least one unit). No stock, no payment.';

CREATE FUNCTION returns.inspect_rma(p_hash bytea,p_store uuid,p_rma uuid,p_key text,p_request_hash bytea,p_expected_version bigint,p_lines jsonb)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_auth jsonb; v_t uuid; v_p uuid; rr record; x record; l record; v_saved bytea; v_response jsonb; v_fail_code text; v_fail_msg text;
 v_n int; v_total int; v_done int;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_rma IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_expected_version IS NULL OR p_expected_version<1 OR p_lines IS NULL
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid returns request' USING ERRCODE='PT400'; END IF;
 -- the disposition decides what goes back on sale: it needs inventory:write on top of fulfillment:write
 v_auth:=returns.authorize(p_hash,p_store,'fulfillment:write','inventory:write');
 v_t:=(v_auth->>'t')::uuid; v_p:=(v_auth->>'p')::uuid;
 PERFORM 1 FROM returns.line_set(p_lines,'qty_restock','qty_scrap');
 <<work>>
 BEGIN
  SELECT r.state,r.version INTO rr FROM returns.rmas r WHERE r.tenant_id=v_t AND r.store_id=p_store AND r.id=p_rma FOR UPDATE;
  IF NOT FOUND THEN v_fail_code:='PT404'; v_fail_msg:='rma not found'; EXIT work; END IF;
  SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=v_t AND c.store_id=p_store
   AND c.operation='returns.rma.inspect' AND c.idempotency_key=p_key;
  IF FOUND THEN
   IF v_saved<>p_request_hash THEN v_fail_code:='PT409'; v_fail_msg:='idempotency_conflict'; END IF;
   EXIT work;
  END IF;
  IF rr.version<>p_expected_version THEN v_fail_code:='PT409'; v_fail_msg:='version_changed'; EXIT work; END IF;
  IF rr.state<>'RECEIVED' THEN v_fail_code:='PT409'; v_fail_msg:='invalid_state'; EXIT work; END IF;
  FOR x IN SELECT * FROM returns.line_set(p_lines,'qty_restock','qty_scrap') LOOP
   SELECT count(*)::int INTO v_n FROM returns.rma_lines d WHERE d.tenant_id=v_t AND d.store_id=p_store AND d.rma_id=p_rma
    AND d.sku_id=x.l_sku AND (x.l_wh IS NULL OR d.warehouse_id=x.l_wh);
   IF v_n=0 THEN v_fail_code:='PT422'; v_fail_msg:='unknown_line'; EXIT work; END IF;
   IF v_n>1 THEN v_fail_code:='PT422'; v_fail_msg:='ambiguous_line'; EXIT work; END IF;
   SELECT d.warehouse_id,d.qty_received,d.qty_restock INTO l FROM returns.rma_lines d WHERE d.tenant_id=v_t AND d.store_id=p_store
    AND d.rma_id=p_rma AND d.sku_id=x.l_sku AND (x.l_wh IS NULL OR d.warehouse_id=x.l_wh);
   IF l.qty_restock IS NOT NULL THEN v_fail_code:='PT422'; v_fail_msg:='invalid_quantities'; EXIT work; END IF;  -- the same line twice
   IF x.l_a+x.l_b<>l.qty_received THEN v_fail_code:='PT422'; v_fail_msg:='quantities_mismatch'; EXIT work; END IF;
   UPDATE returns.rma_lines SET qty_restock=x.l_a,qty_scrap=x.l_b WHERE tenant_id=v_t AND store_id=p_store AND rma_id=p_rma
    AND warehouse_id=l.warehouse_id AND sku_id=x.l_sku;
  END LOOP;
  SELECT count(*)::int,count(d.qty_restock)::int INTO v_total,v_done FROM returns.rma_lines d WHERE d.tenant_id=v_t AND d.store_id=p_store AND d.rma_id=p_rma;
  IF v_total<>v_done THEN v_fail_code:='PT422'; v_fail_msg:='lines_incomplete'; EXIT work; END IF;
  UPDATE returns.rmas SET state='INSPECTED',version=version+1,inspected_by=v_p,inspected_at=clock_timestamp(),updated_at=clock_timestamp()
   WHERE tenant_id=v_t AND store_id=p_store AND id=p_rma;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_t,p_store,v_p,'returns.rma_inspected');
  v_response:=returns.rma_json(v_t,p_store,p_rma);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(v_t,p_store,'returns.rma.inspect',p_key,p_request_hash,v_response,v_p);
 END work;
 PERFORM returns.reauthorize(p_hash,p_store,'fulfillment:write','inventory:write',v_auth);
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION returns.inspect_rma(bytea,uuid,uuid,text,bytea,bigint,jsonb) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.inspect_rma(bytea,uuid,uuid,text,bytea,bigint,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION returns.inspect_rma(bytea,uuid,uuid,text,bytea,bigint,jsonb) TO commerce_runtime;
COMMENT ON FUNCTION returns.inspect_rma(bytea,uuid,uuid,text,bytea,bigint,jsonb) IS 'internal/returns Inspect only; EXECUTE commerce_runtime. fulfillment:write AND inventory:write, Idempotency-Key, CAS on version. RECEIVED -> INSPECTED recording qty_restock + qty_scrap = qty_received per line (every line named once). Records the decision only; stock moves at close.';

CREATE FUNCTION returns.cancel_rma(p_hash bytea,p_store uuid,p_rma uuid,p_key text,p_request_hash bytea,p_expected_version bigint)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_auth jsonb; v_t uuid; v_p uuid; rr record; v_saved bytea; v_response jsonb; v_fail_code text; v_fail_msg text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_rma IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_expected_version IS NULL OR p_expected_version<1
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid returns request' USING ERRCODE='PT400'; END IF;
 v_auth:=returns.authorize(p_hash,p_store,'fulfillment:write',NULL);
 v_t:=(v_auth->>'t')::uuid; v_p:=(v_auth->>'p')::uuid;
 <<work>>
 BEGIN
  SELECT r.state,r.version INTO rr FROM returns.rmas r WHERE r.tenant_id=v_t AND r.store_id=p_store AND r.id=p_rma FOR UPDATE;
  IF NOT FOUND THEN v_fail_code:='PT404'; v_fail_msg:='rma not found'; EXIT work; END IF;
  SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=v_t AND c.store_id=p_store
   AND c.operation='returns.rma.cancel' AND c.idempotency_key=p_key;
  IF FOUND THEN
   IF v_saved<>p_request_hash THEN v_fail_code:='PT409'; v_fail_msg:='idempotency_conflict'; END IF;
   EXIT work;
  END IF;
  IF rr.version<>p_expected_version THEN v_fail_code:='PT409'; v_fail_msg:='version_changed'; EXIT work; END IF;
  IF rr.state<>'REGISTERED' THEN v_fail_code:='PT409'; v_fail_msg:='invalid_state'; EXIT work; END IF;
  UPDATE returns.rmas SET state='CANCELLED',version=version+1,cancelled_by=v_p,cancelled_at=clock_timestamp(),updated_at=clock_timestamp()
   WHERE tenant_id=v_t AND store_id=p_store AND id=p_rma;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_t,p_store,v_p,'returns.rma_cancelled');
  v_response:=returns.rma_json(v_t,p_store,p_rma);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(v_t,p_store,'returns.rma.cancel',p_key,p_request_hash,v_response,v_p);
 END work;
 PERFORM returns.reauthorize(p_hash,p_store,'fulfillment:write',NULL,v_auth);
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION returns.cancel_rma(bytea,uuid,uuid,text,bytea,bigint) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.cancel_rma(bytea,uuid,uuid,text,bytea,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION returns.cancel_rma(bytea,uuid,uuid,text,bytea,bigint) TO commerce_runtime;
COMMENT ON FUNCTION returns.cancel_rma(bytea,uuid,uuid,text,bytea,bigint) IS 'internal/returns Cancel only; EXECUTE commerce_runtime. fulfillment:write, Idempotency-Key, CAS on version. REGISTERED -> CANCELLED (frees its quantity for a new RMA). Nothing else is cancellable.';

-- ---------------------------------------------------------------------------------------------------
-- returns.close_rma (fulfillment:write + inventory:write): INSPECTED -> CLOSED and, per line with qty_restock>0, ONE DEALLOCATE ledger
-- row (operation returns.rma.restock, command_key = rma id). The RMA row lock plus the balance row lock (inventory.lock_balance, global
-- (warehouse, sku) order) serialize concurrent closes; the RMA state machine, the ledger unique key and inventory.guard_returns_ledger
-- each independently refuse a second restock. Optional refund_id is only checked to belong to the same order; no payment is touched.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION returns.close_rma(p_hash bytea,p_store uuid,p_rma uuid,p_key text,p_request_hash bytea,p_expected_version bigint,p_refund uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_auth jsonb; v_t uuid; v_p uuid; rr record; od record; rv record; l record; v_saved bytea; v_response jsonb; v_fail_code text; v_fail_msg text;
 v_restock bigint; v_alloc bigint;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_rma IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_expected_version IS NULL OR p_expected_version<1
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid returns request' USING ERRCODE='PT400'; END IF;
 v_auth:=returns.authorize(p_hash,p_store,'fulfillment:write','inventory:write');
 v_t:=(v_auth->>'t')::uuid; v_p:=(v_auth->>'p')::uuid;
 <<work>>
 BEGIN
  SELECT r.state,r.version,r.order_id INTO rr FROM returns.rmas r WHERE r.tenant_id=v_t AND r.store_id=p_store AND r.id=p_rma FOR UPDATE;
  IF NOT FOUND THEN v_fail_code:='PT404'; v_fail_msg:='rma not found'; EXIT work; END IF;
  SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=v_t AND c.store_id=p_store
   AND c.operation='returns.rma.close' AND c.idempotency_key=p_key;
  IF FOUND THEN
   IF v_saved<>p_request_hash THEN v_fail_code:='PT409'; v_fail_msg:='idempotency_conflict'; END IF;
   EXIT work;
  END IF;
  IF rr.version<>p_expected_version THEN v_fail_code:='PT409'; v_fail_msg:='version_changed'; EXIT work; END IF;
  IF rr.state<>'INSPECTED' THEN v_fail_code:='PT409'; v_fail_msg:='invalid_state'; EXIT work; END IF;
  SELECT k.owner_id,k.creator_session_id,k.commercial_state INTO od FROM checkout.orders k
   WHERE k.tenant_id=v_t AND k.store_id=p_store AND k.id=rr.order_id FOR UPDATE;
  PERFORM set_config('app.buyer_id',od.owner_id::text,true),set_config('app.buyer_session_id',od.creator_session_id::text,true);
  IF p_refund IS NOT NULL AND NOT EXISTS(SELECT 1 FROM payments.stripe_refunds sr WHERE sr.tenant_id=v_t AND sr.store_id=p_store
    AND sr.id=p_refund AND sr.order_id=rr.order_id) THEN
   v_fail_code:='PT422'; v_fail_msg:='refund_mismatch'; EXIT work; END IF;
  SELECT coalesce(sum(d.qty_restock),0) INTO v_restock FROM returns.rma_lines d WHERE d.tenant_id=v_t AND d.store_id=p_store AND d.rma_id=p_rma;
  IF v_restock>0 THEN
   SELECT x.state INTO rv FROM inventory.reservations x WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=rr.order_id FOR UPDATE;
   IF NOT FOUND OR rv.state<>'COMMITTED' OR od.commercial_state<>'CONFIRMED' THEN
    v_fail_code:='PT409'; v_fail_msg:='not_restockable'; EXIT work; END IF;
  END IF;
  UPDATE returns.rmas SET state='CLOSED',version=version+1,closed_by=v_p,closed_at=clock_timestamp(),refund_id=p_refund,updated_at=clock_timestamp()
   WHERE tenant_id=v_t AND store_id=p_store AND id=p_rma;
  FOR l IN SELECT d.warehouse_id,d.sku_id,d.qty_restock FROM returns.rma_lines d WHERE d.tenant_id=v_t AND d.store_id=p_store
   AND d.rma_id=p_rma AND d.qty_restock>0 ORDER BY d.warehouse_id,d.sku_id LOOP
   PERFORM 1 FROM inventory.lock_balance(l.warehouse_id,l.sku_id);
   SELECT coalesce(sum(a.delta_allocated),0) INTO v_alloc FROM inventory.ledger a WHERE a.tenant_id=v_t AND a.store_id=p_store
    AND a.checkout_id=rr.order_id AND a.warehouse_id=l.warehouse_id AND a.sku_id=l.sku_id AND a.kind IN ('ALLOCATE','DEALLOCATE');
   IF v_alloc<l.qty_restock THEN v_fail_code:='PT409'; v_fail_msg:='not_restockable'; EXIT work; END IF;
   -- Writes inventory.ledger DEALLOCATE (returns.rma.restock; provenance: inventory.guard_returns_ledger, contracts/returns-v1.md §5).
   INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_allocated,operation,command_key,reservation_id,reason,
    principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
   VALUES(v_t,p_store,l.warehouse_id,l.sku_id,'DEALLOCATE',-l.qty_restock,'returns.rma.restock',p_rma::text,rr.order_id,'rma_restock',
    v_p,rr.order_id,od.owner_id,od.creator_session_id,'MERCHANT');
  END LOOP;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_t,p_store,v_p,'returns.rma_closed');
  v_response:=returns.rma_json(v_t,p_store,p_rma)||jsonb_build_object('restocked_units',v_restock);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(v_t,p_store,'returns.rma.close',p_key,p_request_hash,v_response,v_p);
 END work;
 PERFORM returns.reauthorize(p_hash,p_store,'fulfillment:write','inventory:write',v_auth);
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION returns.close_rma(bytea,uuid,uuid,text,bytea,bigint,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.close_rma(bytea,uuid,uuid,text,bytea,bigint,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION returns.close_rma(bytea,uuid,uuid,text,bytea,bigint,uuid) TO commerce_runtime;
COMMENT ON FUNCTION returns.close_rma(bytea,uuid,uuid,text,bytea,bigint,uuid) IS 'internal/returns Close only; EXECUTE commerce_runtime. fulfillment:write AND inventory:write, Idempotency-Key, CAS on version. INSPECTED -> CLOSED; per line with qty_restock>0 one DEALLOCATE ledger row (the ONLY stock write of a return; scrap writes nothing). Optional refund_id must belong to the same order (refund_mismatch 422); never starts or changes a refund.';

-- ---------------------------------------------------------------------------------------------------
-- Reads (orders:read): one order's RMAs, and the store list (newest first, optional state filter, max 100).
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION returns.read_order_returns(p_hash bytea,p_store uuid,p_order uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_auth jsonb; v_t uuid; v_out jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid returns request' USING ERRCODE='PT400'; END IF;
 v_auth:=returns.authorize(p_hash,p_store,'orders:read',NULL);
 v_t:=(v_auth->>'t')::uuid;
 IF NOT EXISTS(SELECT 1 FROM checkout.orders k WHERE k.tenant_id=v_t AND k.store_id=p_store AND k.id=p_order) THEN
  RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 SELECT coalesce(jsonb_agg(returns.rma_json(v_t,p_store,r.id) ORDER BY r.created_at,r.id),'[]'::jsonb) INTO v_out
  FROM returns.rmas r WHERE r.tenant_id=v_t AND r.store_id=p_store AND r.order_id=p_order;
 PERFORM returns.reauthorize(p_hash,p_store,'orders:read',NULL,v_auth);
 RETURN v_out;
END $$;
ALTER FUNCTION returns.read_order_returns(bytea,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.read_order_returns(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION returns.read_order_returns(bytea,uuid,uuid) TO commerce_runtime;
COMMENT ON FUNCTION returns.read_order_returns(bytea,uuid,uuid) IS 'internal/returns ForOrder only; EXECUTE commerce_runtime. orders:read; the RMAs of one order of THIS store (another store''s order is 404), oldest first. Read only.';

CREATE FUNCTION returns.list_returns(p_hash bytea,p_store uuid,p_state text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_auth jsonb; v_t uuid; v_out jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR (p_state IS NOT NULL AND p_state NOT IN ('REGISTERED','RECEIVED','INSPECTED','CLOSED','CANCELLED'))
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid returns request' USING ERRCODE='PT400'; END IF;
 v_auth:=returns.authorize(p_hash,p_store,'orders:read',NULL);
 v_t:=(v_auth->>'t')::uuid;
 SELECT coalesce(jsonb_agg(returns.rma_json(v_t,p_store,x.id) ORDER BY x.created_at DESC,x.id),'[]'::jsonb) INTO v_out FROM (
  SELECT r.id,r.created_at FROM returns.rmas r WHERE r.tenant_id=v_t AND r.store_id=p_store AND (p_state IS NULL OR r.state=p_state)
   ORDER BY r.created_at DESC,r.id LIMIT 100) x;
 PERFORM returns.reauthorize(p_hash,p_store,'orders:read',NULL,v_auth);
 RETURN v_out;
END $$;
ALTER FUNCTION returns.list_returns(bytea,uuid,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION returns.list_returns(bytea,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION returns.list_returns(bytea,uuid,text) TO commerce_runtime;
COMMENT ON FUNCTION returns.list_returns(bytea,uuid,text) IS 'internal/returns List only; EXECUTE commerce_runtime. orders:read; the 100 newest RMAs of the store, optionally one state. Read only.';

-- ---------------------------------------------------------------------------------------------------
-- fulfillment.merchant_cancel_order (fulfillment:write, Idempotency-Key, CAS on the commercial state the merchant saw).
--   DRAFT (an unpaid hold, no payment attempt): reservation HELD -> RELEASED, RELEASE ledger rows.
--   AWAITING_PAYMENT: 409 payment_in_flight (a payment attempt exists; the payment worker closes it, never the merchant).
--   CONFIRMED card, unshipped: only when succeeded + in-flight refunds cover the whole CAPTURED amount (409 refund_first otherwise;
--     never an automatic refund), then COMMITTED -> RELEASED and DEALLOCATE of the whole allocation; leaves an OPEN parcel group.
--   pay-at-pickup / COD, unshipped: delegates to inventory.release_pay_at_pickup(cancel) (the §16.8 definer), unchanged.
--   shipped -> 409 already_shipped (returns path); bank transfer / anything else -> 422 not_cancellable.
-- Lock order: parcel group (if any) -> order -> reservation -> balances: the same direction as begin_parcel_group_shipment.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION fulfillment.merchant_cancel_order(p_hash bytea,p_store uuid,p_order uuid,p_key text,p_request_hash bytea,p_expected_state text,p_reason text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_auth jsonb; v_t uuid; v_p uuid; o record; r record; l record; g record; v_group uuid; v_group2 uuid; v_saved bytea; v_response jsonb;
 v_fail_code text; v_fail_msg text; v_now timestamptz; v_lines int:=0; v_capt bigint; v_held bigint; v_attempt uuid; v_alloc bigint; v_left int;
 v_grp jsonb:=NULL; v_delegate jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_reason IS NULL OR length(btrim(p_reason)) NOT BETWEEN 1 AND 240
  OR p_expected_state IS NULL OR p_expected_state NOT IN ('DRAFT','AWAITING_PAYMENT','CONFIRMED','AWAITING_COLLECTION')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid cancel request' USING ERRCODE='PT400'; END IF;
 v_auth:=returns.authorize(p_hash,p_store,'fulfillment:write',NULL);
 v_t:=(v_auth->>'t')::uuid; v_p:=(v_auth->>'p')::uuid;
 -- parcel group first (group -> order, like the group shipment), found without a lock and re-checked under the order lock below
 SELECT m.group_id INTO v_group FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.order_id=p_order;
 IF v_group IS NOT NULL THEN
  PERFORM 1 FROM fulfillment.parcel_groups x WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=v_group FOR UPDATE; END IF;
 <<work>>
 BEGIN
  SELECT k.owner_id,k.creator_session_id,k.payment_mode,k.commercial_state,k.fulfillment_state,k.collection_state INTO o
   FROM checkout.orders k WHERE k.tenant_id=v_t AND k.store_id=p_store AND k.id=p_order FOR UPDATE;
  IF NOT FOUND THEN v_fail_code:='PT404'; v_fail_msg:='order not found'; EXIT work; END IF;
  PERFORM set_config('app.buyer_id',o.owner_id::text,true),set_config('app.buyer_session_id',o.creator_session_id::text,true);
  SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=v_t AND c.store_id=p_store
   AND c.operation='fulfillment.merchant_cancel' AND c.idempotency_key=p_key;
  IF FOUND THEN
   IF v_saved<>p_request_hash THEN v_fail_code:='PT409'; v_fail_msg:='idempotency_conflict'; END IF;
   EXIT work;
  END IF;
  IF o.commercial_state='CANCELLED' THEN v_fail_code:='PT409'; v_fail_msg:='already_cancelled'; EXIT work; END IF;
  IF o.commercial_state<>p_expected_state THEN v_fail_code:='PT409'; v_fail_msg:='state_changed'; EXIT work; END IF;
  IF o.commercial_state='AWAITING_PAYMENT' THEN v_fail_code:='PT409'; v_fail_msg:='payment_in_flight'; EXIT work; END IF;
  IF o.fulfillment_state IN ('MERCHANT_SHIPPED','PROVIDER_LABEL_CREATED') THEN v_fail_code:='PT409'; v_fail_msg:='already_shipped'; EXIT work; END IF;
  IF o.fulfillment_state<>'MANUAL_UNASSIGNED' THEN v_fail_code:='PT422'; v_fail_msg:='not_cancellable'; EXIT work; END IF;
  v_now:=clock_timestamp();

  IF o.payment_mode IN ('pay_at_pickup','cash_on_delivery') THEN
   -- the §16.8 definer owns this path (auth, replay under its own operation, CVS attempt settle, ledger); its refusals propagate as-is
   IF o.collection_state IS DISTINCT FROM 'PENDING' THEN v_fail_code:='PT409'; v_fail_msg:='state_changed'; EXIT work; END IF;
   -- Calls inventory.release_pay_at_pickup(cancel) (taiwan-cvs-logistics-v1 §16.8); same Idempotency-Key and request hash.
   v_delegate:=inventory.release_pay_at_pickup(p_hash,p_store,p_order,p_key,p_request_hash,'cancel','PENDING');
   v_lines:=coalesce((v_delegate->>'released_lines')::int,0);
  ELSIF o.commercial_state='DRAFT' THEN
   IF EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.tenant_id=v_t AND a.store_id=p_store AND a.order_id=p_order) THEN
    v_fail_code:='PT409'; v_fail_msg:='payment_in_flight'; EXIT work; END IF;
   SELECT x.state INTO r FROM inventory.reservations x WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
   IF NOT FOUND OR r.state<>'HELD' THEN v_fail_code:='PT409'; v_fail_msg:='state_changed'; EXIT work; END IF;
   UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED',updated_at=v_now
    WHERE tenant_id=v_t AND store_id=p_store AND owner_id=o.owner_id AND id=p_order;
   UPDATE inventory.reservations SET state='RELEASED' WHERE tenant_id=v_t AND store_id=p_store AND id=p_order;
   FOR l IN SELECT x.warehouse_id,x.sku_id,x.quantity FROM inventory.reservation_lines x WHERE x.tenant_id=v_t AND x.store_id=p_store
    AND x.reservation_id=p_order ORDER BY x.warehouse_id,x.sku_id LOOP
    PERFORM 1 FROM inventory.lock_balance(l.warehouse_id,l.sku_id);
    -- Writes inventory.ledger RELEASE (checkout.merchant_cancel; provenance: inventory.guard_returns_ledger, contracts/returns-v1.md §5).
    INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,operation,command_key,reservation_id,reason,
     principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
    VALUES(v_t,p_store,l.warehouse_id,l.sku_id,'RELEASE',-l.quantity,'checkout.merchant_cancel',p_order::text,p_order,'merchant_cancel',
     v_p,p_order,o.owner_id,o.creator_session_id,'MERCHANT');
    v_lines:=v_lines+1;
   END LOOP;
  ELSIF o.payment_mode='card' AND o.commercial_state='CONFIRMED' THEN
   PERFORM fulfillment.settle_cvs_attempt(v_t,p_store,p_order);
   IF EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=v_t AND c.store_id=p_store AND c.order_id=p_order
     AND c.state IN ('REQUESTED','UNKNOWN')) THEN v_fail_code:='PT409'; v_fail_msg:='cvs_attempt_in_flight'; EXIT work; END IF;
   IF EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=v_t AND c.store_id=p_store AND c.order_id=p_order
     AND c.state NOT IN ('FAILED','ABANDONED')) THEN v_fail_code:='PT409'; v_fail_msg:='already_shipped'; EXIT work; END IF;
   IF EXISTS(SELECT 1 FROM returns.rmas m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.order_id=p_order AND m.state<>'CANCELLED') THEN
    v_fail_code:='PT409'; v_fail_msg:='has_returns'; EXIT work; END IF;
   SELECT a.id,f.amount_minor INTO v_attempt,v_capt FROM checkout.payment_attempts a JOIN payments.facts f ON f.tenant_id=a.tenant_id
    AND f.store_id=a.store_id AND f.attempt_id=a.id AND f.kind='CAPTURED' AND f.amount_minor=a.amount_minor
    WHERE a.tenant_id=v_t AND a.store_id=p_store AND a.order_id=p_order;
   IF v_attempt IS NULL THEN v_fail_code:='PT422'; v_fail_msg:='not_cancellable'; EXIT work; END IF;
   IF EXISTS(SELECT 1 FROM payments.review_cases rc WHERE rc.tenant_id=v_t AND rc.store_id=p_store AND rc.attempt_id=v_attempt
     AND rc.reason<>'PROVIDER_PRESENTMENT_DRIFT') THEN v_fail_code:='PT409'; v_fail_msg:='payment_review_open'; EXIT work; END IF;
   -- refund held = every refund that is not failed/cancelled/rejected (succeeded + in flight), the MD6 definition
   SELECT coalesce(sum(sr.amount_minor),0) INTO v_held FROM payments.stripe_refunds sr WHERE sr.tenant_id=v_t AND sr.store_id=p_store
    AND sr.attempt_id=v_attempt AND NOT EXISTS(SELECT 1 FROM payments.refund_facts rf WHERE rf.tenant_id=sr.tenant_id AND rf.store_id=sr.store_id
     AND rf.refund_id=sr.id AND rf.kind IN ('FAILED','CANCELED','REJECTED'));
   IF v_held<v_capt THEN v_fail_code:='PT409'; v_fail_msg:='refund_first'; EXIT work; END IF;
   SELECT x.state INTO r FROM inventory.reservations x WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
   IF NOT FOUND OR r.state<>'COMMITTED' THEN v_fail_code:='PT409'; v_fail_msg:='state_changed'; EXIT work; END IF;
   UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED',updated_at=v_now
    WHERE tenant_id=v_t AND store_id=p_store AND owner_id=o.owner_id AND id=p_order;
   UPDATE inventory.reservations SET state='RELEASED' WHERE tenant_id=v_t AND store_id=p_store AND id=p_order;
   FOR l IN SELECT x.warehouse_id,x.sku_id,x.quantity FROM inventory.reservation_lines x WHERE x.tenant_id=v_t AND x.store_id=p_store
    AND x.reservation_id=p_order ORDER BY x.warehouse_id,x.sku_id LOOP
    PERFORM 1 FROM inventory.lock_balance(l.warehouse_id,l.sku_id);
    SELECT coalesce(sum(a.delta_allocated),0) INTO v_alloc FROM inventory.ledger a WHERE a.tenant_id=v_t AND a.store_id=p_store
     AND a.checkout_id=p_order AND a.warehouse_id=l.warehouse_id AND a.sku_id=l.sku_id AND a.kind IN ('ALLOCATE','DEALLOCATE');
    IF v_alloc<>l.quantity THEN v_fail_code:='PT409'; v_fail_msg:='not_cancellable'; EXIT work; END IF;  -- allocation missing or already released
    -- Writes inventory.ledger DEALLOCATE (checkout.merchant_cancel; provenance: inventory.guard_returns_ledger, contracts/returns-v1.md §5).
    INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_allocated,operation,command_key,reservation_id,reason,
     principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
    VALUES(v_t,p_store,l.warehouse_id,l.sku_id,'DEALLOCATE',-l.quantity,'checkout.merchant_cancel',p_order::text,p_order,'merchant_cancel',
     v_p,p_order,o.owner_id,o.creator_session_id,'MERCHANT');
    v_lines:=v_lines+1;
   END LOOP;
   -- W3-07B parcel group: an OPEN group loses this order (a group needs >= 2 orders, so the last survivor dissolves it).
   SELECT m.group_id INTO v_group2 FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.order_id=p_order;
   IF v_group2 IS NOT NULL THEN
    -- joined a group after the unlocked read above (creator committed first): lock it now; a deadlock here is retried by the client (503)
    SELECT x.state,x.version INTO g FROM fulfillment.parcel_groups x WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=v_group2 FOR UPDATE;
    IF g.state<>'OPEN' THEN v_fail_code:='PT409'; v_fail_msg:='already_shipped'; EXIT work; END IF;
    DELETE FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.order_id=p_order;
    SELECT count(*)::int INTO v_left FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.group_id=v_group2;
    IF v_left<=1 THEN
     DELETE FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=v_t AND m.store_id=p_store AND m.group_id=v_group2;
     UPDATE fulfillment.parcel_groups x SET state='DISSOLVED',version=x.version+1 WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=v_group2;
     INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_t,p_store,v_p,'fulfillment.parcel_group_dissolved');
     v_grp:=jsonb_build_object('id',v_group2,'state','DISSOLVED','version',g.version+1);
    ELSE
     UPDATE fulfillment.parcel_groups x SET version=x.version+1 WHERE x.tenant_id=v_t AND x.store_id=p_store AND x.id=v_group2;
     v_grp:=jsonb_build_object('id',v_group2,'state','OPEN','version',g.version+1);
    END IF;
   END IF;
  ELSE
   v_fail_code:='PT422'; v_fail_msg:='not_cancellable'; EXIT work;  -- bank transfer (offline refund path), anything else
  END IF;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_t,p_store,v_p,'orders.merchant_cancelled');
  v_response:=jsonb_build_object('order_id',p_order,'commercial_state','CANCELLED','released_lines',v_lines,'parcel_group',v_grp,'reason',btrim(p_reason));
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(v_t,p_store,'fulfillment.merchant_cancel',p_key,p_request_hash,v_response,v_p);
 END work;
 PERFORM returns.reauthorize(p_hash,p_store,'fulfillment:write',NULL,v_auth);
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION fulfillment.merchant_cancel_order(bytea,uuid,uuid,text,bytea,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.merchant_cancel_order(bytea,uuid,uuid,text,bytea,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.merchant_cancel_order(bytea,uuid,uuid,text,bytea,text,text) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.merchant_cancel_order(bytea,uuid,uuid,text,bytea,text,text) IS 'internal/fulfillment CancelOrder only; EXECUTE commerce_runtime. fulfillment:write, Idempotency-Key, CAS on expected_state. DRAFT hold -> RELEASE rows; CONFIRMED unshipped card order -> only after refunds cover the capture (else 409 refund_first) -> DEALLOCATE of the whole allocation, leaves its OPEN parcel group; pay-at-pickup/COD delegates to inventory.release_pay_at_pickup; AWAITING_PAYMENT 409 payment_in_flight; shipped 409 already_shipped. Never starts a refund, never writes payments.*.';
