-- 0107 home-cod (contracts/payment-methods-v1.md + docs/delivery/units/home-cod.md, FROZEN; R5 波次1).
--
-- Owns: (1) the cash_on_delivery payment mode as a FOURTH mode next to card, pay_at_pickup and bank_transfer, valid only on
-- home-delivery services whose merchant setting enables it. Begin (post_river/0020, which owns begin_hold itself) creates the
-- order AWAITING_COLLECTION with stock held exactly as pay_at_pickup (RESERVE then BUYER ALLOCATE commit at placement, no online
-- charge, no payment attempt) and reuses the pay_at_pickup money rules: whole-TWD total, a per-order cap from the COD setting,
-- and the pay_at_pickup open-orders limit. (2) the COD settings (enable switch, per-order cap, optional whole-TWD surcharge, the
-- carrier label 黑猫/新竹 — manual fulfilment, no carrier API). (3) the shared collection state machine: COD reuses
-- fulfillment.record_collection and inventory.release_pay_at_pickup UNCHANGED except that their payment_mode guards now admit
-- 'cash_on_delivery' and the cancel guard admits AWAITING_COLLECTION; 'returned' restocks exactly once through the same
-- ledger_pay_at_pickup_release_once index (0083 pattern) and 'collected' keeps the 0102 shipped-first guard. (4) finance:
-- identity.read_finance_summary gains the two COD collected columns (count + minor, environment LIVE, day = collected_at).
-- (5) the merchant order list/export/dashboard and the unshipped index admit AWAITING_COLLECTION, the buyer and merchant
-- notifications fire for COD placement/cancel, and the manual-shipment state guard admits a MERCHANT_SHIPPED order that is
-- AWAITING_COLLECTION.
--
-- Never: no PSP, no payments.facts row for a COD order (the offline fact is collection_state COLLECTED + the order total and
-- surcharge), no carrier API call (the carrier label is a manual-fulfilment hint), no amount from the client (total_minor is the
-- server quote total and cod_surcharge_minor is the server settings surcharge), no new role or grant to the retired
-- commerce_worker role, no change to ledger_checkout_actor (COD reuses the pay_at_pickup operation strings), no new third-party
-- dependency.
-- Depends on: 0072 (collection_state/payment_mode/ledger pay_at_pickup branches), 0073 (order_money_shippable, projections),
-- 0083 (release_pay_at_pickup/record_collection), 0088 (AWAITING_TRANSFER widening, read_finance_summary, settings pattern),
-- 0090 (notify triggers), 0094 (dashboard_todos/orders), 0099 (guard_pay_at_pickup_ledger 3-branch body), 0102
-- (record_collection collected guard), 0063 (manual-shipment state guard + unshipped index).
-- Used by: post_river/0020 (begin_hold COD branch), internal/checkout (modes + offer), internal/fulfillment (record/release
-- collection), internal/merchantorders (settings + order list), internal/reporting (finance), the admin/storefront apps.

-- ---------------------------------------------------------------------------------------------------
-- A. checkout.orders: cash_on_delivery mode, AWAITING_COLLECTION state, the surcharge column, the COD open index, the widened
-- one-active-cart index, the unshipped index, and the commerce_auth column grant. Constraint widening by shape (0088:30 pattern).
-- ---------------------------------------------------------------------------------------------------
DO $$
DECLARE v_item record; v_def text; v_name text;
BEGIN
 FOR v_item IN SELECT * FROM (VALUES
  ('checkout.orders','orders_payment_mode_check','''bank_transfer''::text]','''bank_transfer''::text, ''cash_on_delivery''::text]','cash_on_delivery'),
  ('checkout.orders','orders_commercial_state_check','''AWAITING_TRANSFER''::text]','''AWAITING_TRANSFER''::text, ''AWAITING_COLLECTION''::text]','AWAITING_COLLECTION'),
  ('checkout.events','events_action_check','''checkout.transfer_proof_submitted''::text]',
   '''checkout.transfer_proof_submitted''::text, ''checkout.cash_on_delivery_placed''::text]','cash_on_delivery_placed'),
  ('checkout.events','events_actor_action','''checkout.transfer_proof_submitted''::text]',
   '''checkout.transfer_proof_submitted''::text, ''checkout.cash_on_delivery_placed''::text]','cash_on_delivery_placed')
 ) AS t(rel,con,needle,repl,absent) LOOP
  SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c WHERE c.conrelid=v_item.rel::regclass AND c.conname=v_item.con;
  IF v_def IS NULL OR v_def LIKE '%'||v_item.absent||'%'
   OR length(v_def)-length(replace(v_def,v_item.needle,''))<>length(v_item.needle) THEN
   RAISE EXCEPTION '% % has an unexpected shape: %',v_item.rel,v_item.con,v_def; END IF;
  EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I',v_item.rel,v_item.con);
  EXECUTE format('ALTER TABLE %s ADD CONSTRAINT %I %s',v_item.rel,v_item.con,replace(v_def,v_item.needle,v_item.repl));
 END LOOP;
 -- collection_state is now carried by cash_on_delivery orders too (0072:98 equality CHECK widened).
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c WHERE c.conrelid='checkout.orders'::regclass
  AND c.conname='orders_payment_collection';
 IF v_def IS NULL OR v_def LIKE '%cash_on_delivery%' THEN
  RAISE EXCEPTION 'checkout.orders orders_payment_collection has an unexpected shape: %',v_def; END IF;
 ALTER TABLE checkout.orders DROP CONSTRAINT orders_payment_collection;
 ALTER TABLE checkout.orders ADD CONSTRAINT orders_payment_collection
  CHECK((payment_mode IN ('pay_at_pickup','cash_on_delivery'))=(collection_state IS NOT NULL));
END $$;
-- The surcharge is a SEPARATE column, never folded into total_minor (merchantorders validDetail asserts
-- Totals.TotalMinor == TotalMinor, which is the snapshot-derived order total). COD collect amount = total_minor + cod_surcharge_minor.
ALTER TABLE checkout.orders ADD COLUMN cod_surcharge_minor bigint
 CHECK(cod_surcharge_minor IS NULL OR (cod_surcharge_minor>=0 AND cod_surcharge_minor<=100000 AND cod_surcharge_minor%100=0));
ALTER TABLE checkout.orders ADD CONSTRAINT orders_cod_surcharge CHECK((payment_mode='cash_on_delivery')=(cod_surcharge_minor IS NOT NULL));
COMMENT ON COLUMN checkout.orders.cod_surcharge_minor IS 'internal/checkout (cash on delivery): the whole-TWD COD surcharge in minor units (0..100000, %100=0), snapshot at placement from checkout.cash_on_delivery_settings.surcharge_twd. The buyer pays total_minor + cod_surcharge_minor on delivery. Read by identity.read_finance_summary and the merchant order projection; never part of total_minor.';
-- P2-4: the carrier label (black_cat 黑猫 / hsinchu 新竹) is snapshotted at placement too, so a later settings change never
-- moves a placed order's carrier. Read by the buyer order projection and the options DTO; never part of the money path.
ALTER TABLE checkout.orders ADD COLUMN cod_carrier text
 CHECK(cod_carrier IS NULL OR cod_carrier IN ('black_cat','hsinchu'));
ALTER TABLE checkout.orders ADD CONSTRAINT orders_cod_carrier CHECK((payment_mode='cash_on_delivery')=(cod_carrier IS NOT NULL));
COMMENT ON COLUMN checkout.orders.cod_carrier IS 'internal/checkout (cash on delivery): the carrier label (black_cat 黑猫 / hsinchu 新竹) snapshotted at placement from checkout.cash_on_delivery_settings.carrier (manual fulfilment, no carrier API). Shown to the buyer; never part of the money path.';
-- P1-3b: collected_at is the finance day for COD and pickup collected cash. Set once by fulfillment.record_collection and never
-- overwritten by any later write (a return/refund, a tracking correction, or an erasure anonymising the row), so a collected order's
-- cash stays on the day it was actually collected.
ALTER TABLE checkout.orders ADD COLUMN collected_at timestamptz;
COMMENT ON COLUMN checkout.orders.collected_at IS 'internal/fulfillment (cash on delivery / pay at pickup): the instant the order was recorded COLLECTED, set once by fulfillment.record_collection and never overwritten. identity.read_finance_summary groups COD and pickup collected cash on this day. NULL while the order is not collected.';
-- Best-effort backfill for rows already COLLECTED before this column existed: updated_at was the collection transition at that time,
-- so it is an estimate of the true collection instant (labelled approximation, not a promise of exactness).
UPDATE checkout.orders SET collected_at=updated_at
 WHERE collection_state='COLLECTED' AND collected_at IS NULL;
-- R4-3 analogue: the per-store and per-owner open COD count (begin_hold) scans exactly this predicate.
CREATE INDEX orders_cod_open ON checkout.orders(tenant_id,store_id,owner_id)
 WHERE payment_mode='cash_on_delivery' AND collection_state='PENDING' AND fulfillment_state='MANUAL_UNASSIGNED';
-- One live checkout per cart version now includes the collection wait (0088:60).
DROP INDEX checkout.checkout_one_active_cart_version;
CREATE UNIQUE INDEX checkout_one_active_cart_version ON checkout.orders(tenant_id,store_id,owner_id,cart_id,cart_version)
 WHERE commercial_state IN ('DRAFT','AWAITING_PAYMENT','AWAITING_TRANSFER','CONFIRMED','AWAITING_COLLECTION');
-- The unshipped scan (0063:83) now covers COD orders waiting for a manual shipment.
DROP INDEX checkout.orders_unshipped;
CREATE INDEX orders_unshipped ON checkout.orders(tenant_id,store_id,created_at,id)
 WHERE commercial_state IN ('CONFIRMED','AWAITING_COLLECTION') AND fulfillment_state='MANUAL_UNASSIGNED';
-- finance (identity.read_finance_summary, owner commerce_auth) and the merchant order list (read_merchant_orders, owner commerce_auth)
-- both read the surcharge column, and finance groups COD/pickup cash on collected_at; the buyer order projection
-- (commerce_checkout_runtime, P1-1) reads the surcharge to compute cod_collect_minor and the snapshotted carrier (P2-4).
-- commerce_auth gets exactly cod_surcharge_minor and collected_at (contracts/merchant-orders-v1.md, amendment R5 home-cod): no
-- function owned by commerce_auth reads cod_carrier (the merchant DTO has no carrier key), so it is NOT granted to commerce_auth.
GRANT SELECT(cod_surcharge_minor,collected_at) ON checkout.orders TO commerce_auth;
GRANT SELECT(cod_surcharge_minor,cod_carrier) ON checkout.orders TO commerce_checkout_runtime;
GRANT UPDATE(collected_at) ON checkout.orders TO commerce_checkout_writer;

-- ---------------------------------------------------------------------------------------------------
-- B. Settings. FORCE RLS; only commerce_checkout_writer (the owner of every definer below) sees the rows; no commerce_auth or
-- buyer grant (begin_hold and read_cod_offer run as the writer definer).
-- ---------------------------------------------------------------------------------------------------
CREATE TABLE checkout.cash_on_delivery_settings (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 enabled boolean NOT NULL DEFAULT false,
 max_twd integer NOT NULL DEFAULT 20000 CHECK(max_twd BETWEEN 1 AND 20000),
 surcharge_twd integer NOT NULL DEFAULT 0 CHECK(surcharge_twd BETWEEN 0 AND 1000),
 carrier text NOT NULL DEFAULT 'black_cat' CHECK(carrier IN ('black_cat','hsinchu')),
 version bigint NOT NULL CHECK(version>0), updated_at timestamptz NOT NULL,
 PRIMARY KEY(tenant_id,store_id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
ALTER TABLE checkout.cash_on_delivery_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkout.cash_on_delivery_settings FORCE ROW LEVEL SECURITY;
REVOKE ALL ON checkout.cash_on_delivery_settings FROM PUBLIC;
GRANT SELECT,INSERT ON checkout.cash_on_delivery_settings TO commerce_checkout_writer;
GRANT UPDATE(enabled,max_twd,surcharge_twd,carrier,version,updated_at) ON checkout.cash_on_delivery_settings TO commerce_checkout_writer;
CREATE POLICY cash_on_delivery_settings_writer ON checkout.cash_on_delivery_settings TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);

CREATE FUNCTION payments.read_cash_on_delivery_settings(p_hash bytea,p_store uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; r record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid settings read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT c.* INTO r FROM checkout.cash_on_delivery_settings c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'integration:read');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF r.tenant_id IS NULL THEN
  RETURN jsonb_build_object('version',0,'enabled',false,'max_twd',20000,'surcharge_twd',0,'carrier','black_cat');
 END IF;
 RETURN jsonb_build_object('version',r.version,'enabled',r.enabled,'max_twd',r.max_twd,'surcharge_twd',r.surcharge_twd,'carrier',r.carrier);
END $$;
ALTER FUNCTION payments.read_cash_on_delivery_settings(bytea,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION payments.read_cash_on_delivery_settings(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.read_cash_on_delivery_settings(bytea,uuid) TO commerce_runtime;

CREATE FUNCTION payments.set_cash_on_delivery_settings(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,p_expected_version bigint,
 p_enabled boolean,p_max_twd integer,p_surcharge_twd integer,p_carrier text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; cur record; v_saved bytea; v_response jsonb; v_now timestamptz; v_ver bigint;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_expected_version IS NULL OR p_expected_version<0
  OR p_expected_version>=9223372036854775807 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid cash on delivery settings' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 PERFORM pg_advisory_xact_lock(hashtextextended('cash_on_delivery.settings|'||s.tenant_id||'|'||p_store,0));
 SELECT r.request_hash,r.response INTO v_saved,v_response FROM ops.command_results r WHERE r.tenant_id=s.tenant_id
  AND r.store_id=p_store AND r.operation='checkout.cash_on_delivery_settings.set' AND r.idempotency_key=p_key;
 IF FOUND THEN
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  -- CHECK-equivalent validation first so a bad body is a 422 invalid_settings, never a 500 from a constraint.
  IF p_enabled IS NULL OR p_max_twd IS NULL OR p_surcharge_twd IS NULL OR p_carrier IS NULL
   OR p_max_twd NOT BETWEEN 1 AND 20000 OR p_surcharge_twd NOT BETWEEN 0 AND 1000
   OR p_carrier NOT IN ('black_cat','hsinchu') THEN
   RAISE EXCEPTION 'invalid_settings' USING ERRCODE='PT422'; END IF;
  SELECT c.version INTO cur FROM checkout.cash_on_delivery_settings c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store FOR UPDATE;
  v_now:=clock_timestamp();
  IF NOT FOUND THEN
   IF p_expected_version<>0 THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
   v_ver:=1;
   INSERT INTO checkout.cash_on_delivery_settings(tenant_id,store_id,enabled,max_twd,surcharge_twd,carrier,version,updated_at)
   VALUES(s.tenant_id,p_store,p_enabled,p_max_twd,p_surcharge_twd,p_carrier,1,v_now);
  ELSE
   IF cur.version<>p_expected_version THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
   v_ver:=cur.version+1;
   UPDATE checkout.cash_on_delivery_settings SET enabled=p_enabled,max_twd=p_max_twd,surcharge_twd=p_surcharge_twd,carrier=p_carrier,
    version=v_ver,updated_at=v_now WHERE tenant_id=s.tenant_id AND store_id=p_store;
  END IF;
  -- A change never touches placed orders (each keeps its own surcharge snapshot); only future begin_hold calls read this row.
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
   VALUES(s.tenant_id,p_store,s.principal_id,'checkout.cash_on_delivery_settings_changed');
  v_response:=jsonb_build_object('version',v_ver,'enabled',p_enabled,'max_twd',p_max_twd,'surcharge_twd',p_surcharge_twd,'carrier',p_carrier);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(s.tenant_id,p_store,'checkout.cash_on_delivery_settings.set',p_key,p_request_hash,v_response,s.principal_id);
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF v_final.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION payments.set_cash_on_delivery_settings(bytea,uuid,text,bytea,bigint,boolean,integer,integer,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION payments.set_cash_on_delivery_settings(bytea,uuid,text,bytea,bigint,boolean,integer,integer,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.set_cash_on_delivery_settings(bytea,uuid,text,bytea,bigint,boolean,integer,integer,text) TO commerce_runtime;

-- Buyer options: is the mode on, what is the surcharge and carrier, and what is the cap (read_transfer_offer pattern, no secrets).
CREATE FUNCTION checkout.read_cod_offer(p_hash bytea,p_store uuid)
RETURNS TABLE(enabled boolean,surcharge_twd integer,carrier text,max_twd integer)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_scope record; c record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL THEN RAISE EXCEPTION 'invalid offer read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO v_scope FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM v_scope.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text THEN
  RAISE EXCEPTION 'offer scope' USING ERRCODE='PT403'; END IF;
 SELECT s.* INTO c FROM checkout.cash_on_delivery_settings s WHERE s.tenant_id=v_scope.tenant_id AND s.store_id=p_store;
 RETURN QUERY SELECT coalesce(c.enabled,false),coalesce(c.surcharge_twd,0),coalesce(c.carrier,'black_cat'),coalesce(c.max_twd,20000);
END $$;
ALTER FUNCTION checkout.read_cod_offer(bytea,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.read_cod_offer(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.read_cod_offer(bytea,uuid) TO commerce_checkout_runtime;

-- ---------------------------------------------------------------------------------------------------
-- C. Shared shippability + collection + release + ledger guard + the manual-shipment state guard + the notify triggers.
-- ---------------------------------------------------------------------------------------------------

-- order_money_shippable (0088:264 body) gains the COD branch: a home COD order is shippable while AWAITING_COLLECTION and PENDING,
-- with no payment attempt (the offline fact is the collection itself).
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
  WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_order
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
  WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_order
   AND o.commercial_state='CONFIRMED' AND o.payment_mode='pay_at_pickup' AND o.collection_state='PENDING'
   AND NOT EXISTS(SELECT 1 FROM checkout.payment_attempts pa WHERE pa.tenant_id=o.tenant_id
    AND pa.store_id=o.store_id AND pa.order_id=o.id))
 OR EXISTS(
  SELECT 1 FROM checkout.orders o
  JOIN checkout.bank_transfers t ON t.tenant_id=o.tenant_id AND t.store_id=o.store_id AND t.owner_id=o.owner_id AND t.order_id=o.id
  WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_order
   AND o.commercial_state='CONFIRMED' AND o.payment_mode='bank_transfer' AND t.state='CONFIRMED'
   AND t.confirmed_amount_minor=o.total_minor
   AND NOT EXISTS(SELECT 1 FROM checkout.payment_attempts pa WHERE pa.tenant_id=o.tenant_id
    AND pa.store_id=o.store_id AND pa.order_id=o.id))
 OR EXISTS(
  -- home-cod: a home cash_on_delivery order is shippable while AWAITING_COLLECTION and PENDING; it never has a payment attempt.
  SELECT 1 FROM checkout.orders o
  WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_order
   AND o.commercial_state='AWAITING_COLLECTION' AND o.payment_mode='cash_on_delivery' AND o.collection_state='PENDING'
   AND NOT EXISTS(SELECT 1 FROM checkout.payment_attempts pa WHERE pa.tenant_id=o.tenant_id
    AND pa.store_id=o.store_id AND pa.order_id=o.id))
$$;
ALTER FUNCTION fulfillment.order_money_shippable(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.order_money_shippable(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.order_money_shippable(uuid,uuid,uuid) TO commerce_checkout_writer,commerce_auth;
COMMENT ON FUNCTION fulfillment.order_money_shippable(uuid,uuid,uuid) IS 'internal/fulfillment: the MD6 payment/review/refund clauses of 0063 (card) plus the pay_at_pickup branch (§16.2), the bank_transfer branch (0088) and the cash_on_delivery branch (0107). SECURITY INVOKER: runs with the calling definer''s grants (commerce_checkout_writer, commerce_auth). Non-goal: does not authorize anyone or check fulfillment state.';

-- manual_shipment_eligible (0073:936 body): a COD order is MANUAL_UNASSIGNED + AWAITING_COLLECTION, and order_money_shippable
-- now answers true for it; the no-live-ECPay-attempt clause is trivially true (home orders have no CVS shipment).
CREATE OR REPLACE FUNCTION fulfillment.manual_shipment_eligible(p_tenant uuid,p_store uuid,p_order uuid)
RETURNS boolean LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
 SELECT EXISTS(
  SELECT 1 FROM checkout.orders o
  WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_order
   AND o.commercial_state IN ('CONFIRMED','AWAITING_COLLECTION') AND o.fulfillment_state='MANUAL_UNASSIGNED'
   AND fulfillment.order_money_shippable(o.tenant_id,o.store_id,o.id)
   AND NOT EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id
     AND c.order_id=o.id AND c.state NOT IN ('FAILED','ABANDONED')))
$$;
ALTER FUNCTION fulfillment.manual_shipment_eligible(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.manual_shipment_eligible(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.manual_shipment_eligible(uuid,uuid,uuid) TO commerce_checkout_writer,commerce_auth;
COMMENT ON FUNCTION fulfillment.manual_shipment_eligible(uuid,uuid,uuid) IS 'MD6 shipping eligibility (0063, replaced in 0073, widened in 0107): order_money_shippable AND (CONFIRMED or AWAITING_COLLECTION) + MANUAL_UNASSIGNED AND no live ECPay attempt. Single source for record_manual_shipment, request_cvs_shipment, the list filter and the export.';

-- record_collection (0102 body): the shared collection state machine now also admits cash_on_delivery orders; the shipped-first and
-- cvs_parcel_picked_up/returned guards are unchanged and answer true for a home order with no ECPay attempt.
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
  IF o.payment_mode NOT IN ('pay_at_pickup','cash_on_delivery') THEN RAISE EXCEPTION 'not_pay_at_pickup' USING ERRCODE='PT422'; END IF;
  v_to:=upper(p_state);
  IF p_state IN ('collected','returned') THEN
   IF o.fulfillment_state NOT IN ('MERCHANT_SHIPPED','PROVIDER_LABEL_CREATED') THEN
    RAISE EXCEPTION 'not_shipped' USING ERRCODE='PT422'; END IF;
   IF p_expected_state<>'PENDING' OR o.collection_state<>'PENDING' THEN
    RAISE EXCEPTION 'collection_state_changed' USING ERRCODE='PT409'; END IF;
   IF p_state='returned' AND NOT fulfillment.cvs_parcel_returned(s.tenant_id,p_store,p_order) THEN
    RAISE EXCEPTION 'parcel_not_returned' USING ERRCODE='PT409'; END IF;
   IF p_state='collected' AND NOT fulfillment.cvs_parcel_picked_up(s.tenant_id,p_store,p_order) THEN
    RAISE EXCEPTION 'parcel_not_picked_up' USING ERRCODE='PT409'; END IF;
  ELSE
   IF p_expected_state<>'COLLECTED' OR o.collection_state<>'COLLECTED' THEN
    RAISE EXCEPTION 'collection_state_changed' USING ERRCODE='PT409'; END IF;
  END IF;
  v_now:=clock_timestamp();
  -- P1-3b: collected_at is set once, only on the collected transition, and never overwritten by return/refund (ELSE keeps it).
  UPDATE checkout.orders SET collection_state=v_to,updated_at=v_now,
   collected_at=CASE WHEN v_to='COLLECTED' THEN COALESCE(collected_at,v_now) ELSE collected_at END
   WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_order;
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
COMMENT ON FUNCTION fulfillment.record_collection(bytea,uuid,uuid,text,bytea,text,text) IS 'internal/fulfillment CVS.RecordCollection only; EXECUTE commerce_runtime. fulfillment:write; manual collected/returned/refunded_offline of a pay_at_pickup or cash_on_delivery order; no money movement, no ledger row. returned also requires fulfillment.cvs_parcel_returned (PT409 parcel_not_returned, T21-01); collected also requires fulfillment.cvs_parcel_picked_up (PT409 parcel_not_picked_up, T21-02).';

-- P1-3b: ingest_ecpay_status (0083 body, patched in place rather than re-copied) is the OTHER pay_at_pickup COLLECTED writer
-- (the signed ECPay 2067 status post). Its collected transition must also stamp collected_at once, or an API-collected pickup
-- order would have collected_at NULL and finance (grouped on collected_at) would drop it.
DO $$
DECLARE v_item record; v_fn text;
BEGIN
 FOR v_item IN SELECT * FROM (VALUES
  ('fulfillment.ingest_ecpay_status(uuid,bytea,text,text,text,text,text,text,text,text)',
   'UPDATE checkout.orders SET collection_state=v_collect,updated_at=v_now',
   'UPDATE checkout.orders SET collection_state=v_collect,updated_at=v_now,collected_at=CASE WHEN v_collect=''COLLECTED'' THEN COALESCE(collected_at,v_now) ELSE collected_at END',1)
 ) AS t(fn,needle,repl,expect) LOOP
  v_fn:=pg_get_functiondef(v_item.fn::regprocedure);
  IF (length(v_fn)-length(replace(v_fn,v_item.needle,'')))<>length(v_item.needle)*v_item.expect THEN
   RAISE EXCEPTION '% has an unexpected shape for patch %',v_item.fn,left(v_item.needle,60); END IF;
  EXECUTE replace(v_fn,v_item.needle,v_item.repl);
 END LOOP;
END $$;

-- release_pay_at_pickup (0083 body): cancel now accepts an AWAITING_COLLECTION COD order (unshipped) and the payment_mode guard
-- admits cash_on_delivery; restock keeps the 0083 returned/restock-once shape (the ledger_pay_at_pickup_release_once index).
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
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT k.owner_id,k.creator_session_id,k.payment_mode,k.commercial_state,k.fulfillment_state,k.collection_state INTO o
  FROM checkout.orders k WHERE k.tenant_id=s.tenant_id AND k.store_id=p_store AND k.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 PERFORM set_config('app.buyer_id',o.owner_id::text,true),set_config('app.buyer_session_id',o.creator_session_id::text,true);
 SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=s.tenant_id
  AND c.store_id=p_store AND c.operation='inventory.pay_at_pickup.release' AND c.idempotency_key=p_key;
 IF FOUND THEN
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  IF o.payment_mode NOT IN ('pay_at_pickup','cash_on_delivery') THEN RAISE EXCEPTION 'not_pay_at_pickup' USING ERRCODE='PT422'; END IF;
  IF p_action='cancel' AND p_expected_state<>'PENDING' OR p_action='restock' AND p_expected_state<>'RETURNED' THEN
   RAISE EXCEPTION 'invalid release request' USING ERRCODE='PT400'; END IF;
  IF o.collection_state IS DISTINCT FROM p_expected_state THEN
   RAISE EXCEPTION 'collection_state_changed' USING ERRCODE='PT409'; END IF;
  PERFORM fulfillment.settle_cvs_attempt(s.tenant_id,p_store,p_order);
  IF p_action='cancel' THEN
   IF o.commercial_state NOT IN ('CONFIRMED','AWAITING_COLLECTION') OR o.fulfillment_state<>'MANUAL_UNASSIGNED' THEN
    RAISE EXCEPTION 'not_cancellable' USING ERRCODE='PT422'; END IF;
   IF EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store AND c.order_id=p_order
     AND c.state IN ('REQUESTED','UNKNOWN')) THEN RAISE EXCEPTION 'cvs_attempt_in_flight' USING ERRCODE='PT409'; END IF;
   IF EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store AND c.order_id=p_order
     AND c.state NOT IN ('FAILED','ABANDONED')) THEN RAISE EXCEPTION 'not_cancellable' USING ERRCODE='PT422'; END IF;
   v_op:='fulfillment.pay_at_pickup.cancel'; v_to:='CANCELLED';
  ELSE
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
COMMENT ON FUNCTION inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text) IS 'internal/fulfillment CVS.Release only; EXECUTE commerce_runtime. §16.8: the ONLY writer of DEALLOCATE ledger rows; cancel (PENDING, unshipped; CONFIRMED pay_at_pickup or AWAITING_COLLECTION cash_on_delivery) and restock (RETURNED, latest ECPay attempt UNCLAIMED or none handed over: fulfillment.cvs_parcel_returned) release the order allocation for exactly the allocated quantity; §11.5 evidence = order id + collection_state + actor on the ledger row; no payments/refund/operation/river row.';

-- guard_pay_at_pickup_ledger (0099 3-branch body): the BUYER ALLOCATE and the MERCHANT DEALLOCATE pay-at-pickup branches now admit
-- cash_on_delivery (a COD commit is the same checkout.pay_at_pickup.commit ALLOCATE on an AWAITING_COLLECTION + PENDING order; a COD
-- cancel/restock is the same guarded DEALLOCATE). The bank-transfer restock branch is untouched.
CREATE OR REPLACE FUNCTION inventory.guard_pay_at_pickup_ledger() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.actor_kind='BUYER' AND NEW.kind='ALLOCATE' THEN
  IF NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id
    AND o.owner_id=NEW.buyer_owner_id AND o.id=NEW.checkout_id AND o.creator_session_id=NEW.buyer_session_id
    AND o.payment_mode IN ('pay_at_pickup','cash_on_delivery') AND o.collection_state='PENDING'
    AND o.commercial_state IN ('CONFIRMED','AWAITING_COLLECTION'))
   OR NOT EXISTS(SELECT 1 FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id
    AND l.store_id=NEW.store_id AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id
    AND l.sku_id=NEW.sku_id AND l.quantity=-NEW.delta_reserved)
   OR EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
    AND a.order_id=NEW.checkout_id) THEN
   RAISE EXCEPTION 'pay-at-pickup ledger mismatch' USING ERRCODE='42501'; END IF;
 ELSIF NEW.actor_kind='MERCHANT' AND NEW.kind='DEALLOCATE' AND NEW.operation='checkout.bank_transfer.refund_restock' THEN
  IF NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id
    AND o.owner_id=NEW.buyer_owner_id AND o.id=NEW.checkout_id AND o.creator_session_id=NEW.buyer_session_id
    AND o.payment_mode='bank_transfer' AND o.commercial_state='CONFIRMED')
   OR NOT EXISTS(SELECT 1 FROM checkout.bank_transfers t WHERE t.tenant_id=NEW.tenant_id AND t.store_id=NEW.store_id
    AND t.order_id=NEW.checkout_id AND t.state='REFUNDED_OFFLINE' AND t.refunded_by=NEW.principal_id)
   OR NOT EXISTS(SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id
    AND r.id=NEW.reservation_id AND r.state='RELEASED')
   OR NOT EXISTS(SELECT 1 FROM inventory.ledger a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
    AND a.checkout_id=NEW.checkout_id AND a.warehouse_id=NEW.warehouse_id AND a.sku_id=NEW.sku_id
    AND a.kind='ALLOCATE' AND a.actor_kind='MERCHANT' AND a.operation='checkout.bank_transfer.confirm'
    AND a.delta_allocated=-NEW.delta_allocated)
   OR NOT EXISTS(SELECT 1 FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id
    AND l.store_id=NEW.store_id AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id
    AND l.sku_id=NEW.sku_id AND l.quantity=-NEW.delta_allocated) THEN
   RAISE EXCEPTION 'bank-transfer restock mismatch' USING ERRCODE='42501'; END IF;
 ELSIF NEW.actor_kind='MERCHANT' AND NEW.kind='DEALLOCATE' THEN
  IF NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id
    AND o.owner_id=NEW.buyer_owner_id AND o.id=NEW.checkout_id AND o.payment_mode IN ('pay_at_pickup','cash_on_delivery')
    AND ((NEW.operation='fulfillment.pay_at_pickup.cancel' AND o.collection_state='CANCELLED')
      OR (NEW.operation='fulfillment.pay_at_pickup.restock' AND o.collection_state='RESTOCKED')))
   OR NOT EXISTS(SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id
    AND r.id=NEW.reservation_id AND r.state='RELEASED')
   OR NOT EXISTS(SELECT 1 FROM inventory.ledger a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
    AND a.checkout_id=NEW.checkout_id AND a.warehouse_id=NEW.warehouse_id AND a.sku_id=NEW.sku_id
    AND a.kind='ALLOCATE' AND a.actor_kind='BUYER' AND a.delta_allocated=-NEW.delta_allocated)
   OR NOT EXISTS(SELECT 1 FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id
    AND l.store_id=NEW.store_id AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id
    AND l.sku_id=NEW.sku_id AND l.quantity=-NEW.delta_allocated) THEN
   RAISE EXCEPTION 'pay-at-pickup release mismatch' USING ERRCODE='42501'; END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inventory.guard_pay_at_pickup_ledger() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.guard_pay_at_pickup_ledger() FROM PUBLIC;
COMMENT ON FUNCTION inventory.guard_pay_at_pickup_ledger() IS 'inventory trigger guard (BEFORE INSERT on ledger): the only BUYER ALLOCATE is the pay-at-pickup/cash-on-delivery commit of a pay_at_pickup (CONFIRMED) or cash_on_delivery (AWAITING_COLLECTION) order for the reserved quantity; the only MERCHANT DEALLOCATE releases a cancelled/restocked pay_at_pickup or cash_on_delivery order (§16.2, §16.8, 0107) or the bank-transfer offline-refund restock of a REFUNDED_OFFLINE transfer for exactly the confirm allocation (0099 K3-02). DEFINER (commerce_checkout_writer).';

-- guard_manual_shipment_state (0063 body): a MERCHANT_SHIPPED order may now be AWAITING_COLLECTION (a COD order ships and is then
-- collected), not only CONFIRMED.
CREATE OR REPLACE FUNCTION fulfillment.guard_manual_shipment_state() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_order uuid; v_state text; v_commercial text;
 v_current bigint; v_status text; v_max bigint;
BEGIN
 v_tenant:=NEW.tenant_id; v_store:=NEW.store_id;
 IF TG_TABLE_NAME='orders' THEN v_order:=NEW.id; ELSE v_order:=NEW.order_id; END IF;
 SELECT o.fulfillment_state,o.commercial_state INTO v_state,v_commercial FROM checkout.orders o
  WHERE o.tenant_id=v_tenant AND o.store_id=v_store AND o.id=v_order;
 IF NOT FOUND THEN RETURN NULL; END IF;
 SELECT h.current_version INTO v_current FROM fulfillment.manual_shipment_heads h
  WHERE h.tenant_id=v_tenant AND h.store_id=v_store AND h.order_id=v_order;
 IF NOT FOUND THEN
  IF v_state='MERCHANT_SHIPPED' THEN
   RAISE EXCEPTION 'MERCHANT_SHIPPED order has no shipment head' USING ERRCODE='23514';
  END IF;
  RETURN NULL;
 END IF;
 SELECT v.status INTO v_status FROM fulfillment.manual_shipment_versions v
  WHERE v.tenant_id=v_tenant AND v.store_id=v_store AND v.order_id=v_order AND v.version=v_current;
 SELECT max(v.version) INTO v_max FROM fulfillment.manual_shipment_versions v
  WHERE v.tenant_id=v_tenant AND v.store_id=v_store AND v.order_id=v_order;
 IF v_status IS NULL OR v_max IS DISTINCT FROM v_current OR (v_status='SHIPPED')<>(v_state='MERCHANT_SHIPPED')
  OR (v_state='MERCHANT_SHIPPED' AND v_commercial NOT IN ('CONFIRMED','AWAITING_COLLECTION')) THEN
  RAISE EXCEPTION 'shipment head and order fulfillment state disagree' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION fulfillment.guard_manual_shipment_state() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.guard_manual_shipment_state() FROM PUBLIC;

-- record_manual_shipment (0063 body, P1-3a/P2-9 + P2-4): the command now also reads collection_state so a VOID after collection
-- (COLLECTED/RETURNED/REFUNDED_OFFLINE/CANCELLED/RESTOCKED — COD and pay_at_pickup alike) is refused with PT409
-- collection_state_changed. Without it, a ship -> collect -> void sequence books cash for a parcel that is no longer shipped and can
-- never be shipped again (order_money_shippable refuses a non-PENDING collection), defeating the 0102 collected-only-after-shipped
-- guard after the fact. A tracking-number correction stays allowed at any time and never touches the order row (P1-3b: rewriting
-- updated_at would move the collected cash to another finance day). The manual carrier enum also gains black_cat/hsinchu (P2-4).
CREATE OR REPLACE FUNCTION fulfillment.record_manual_shipment(p_hash bytea,p_store uuid,p_order uuid,p_key text,
 p_request_hash bytea,p_expected_version bigint,p_status text,p_carrier_code text,p_carrier_name text,
 p_tracking_number text,p_tracking_url text,p_note text,p_void_reason text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_expiry timestamptz; v_owner uuid; v_fulfillment text; v_collection text;
 v_head bigint; v_head_status text; v_prev record; v_row record; v_now timestamptz; v_action text;
 v_saved bytea; v_response jsonb; v_replay boolean:=false; v_fail_code text; v_fail_msg text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
  OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32
  OR p_expected_version IS NULL OR p_expected_version<0 OR p_expected_version>=9223372036854775807
  OR p_status IS NULL OR p_status NOT IN ('SHIPPED','VOIDED')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid shipment request' USING ERRCODE='PT400'; END IF;
 -- Authorize before any lock; the GUCs below then come from this result, never from the caller.
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 -- State-independent body rules (void-body rule, contract §5.1): no lock and no state involved.
 IF p_status='VOIDED' THEN
  IF p_carrier_code IS NOT NULL OR p_carrier_name IS NOT NULL OR p_tracking_number IS NOT NULL
   OR p_tracking_url IS NOT NULL OR p_note IS NOT NULL OR p_void_reason IS NULL
   OR p_void_reason NOT IN ('wrong_order','wrong_tracking','not_dispatched','other') THEN
   RAISE EXCEPTION 'invalid_void' USING ERRCODE='PT422'; END IF;
 ELSE
  IF p_void_reason IS NOT NULL THEN RAISE EXCEPTION 'invalid_void' USING ERRCODE='PT422'; END IF;
  IF p_carrier_code IS NULL OR p_carrier_code NOT IN ('seven_eleven_cvs','familymart_cvs','hilife_cvs','okmart_cvs',
   'sf_express','chunghwa_post','black_cat','hsinchu','other') OR (p_carrier_code='other' AND p_carrier_name IS NULL) THEN
   RAISE EXCEPTION 'invalid_carrier' USING ERRCODE='PT422'; END IF;
  IF p_tracking_number IS NULL OR p_tracking_number !~ '^[A-Za-z0-9][A-Za-z0-9 -]{0,63}$' OR p_tracking_number ~ ' $' THEN
   RAISE EXCEPTION 'invalid_tracking' USING ERRCODE='PT422'; END IF;
  IF p_tracking_url IS NOT NULL AND (octet_length(p_tracking_url)>512 OR p_tracking_url !~ '^https://([a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?\.)+[a-zA-Z]([a-zA-Z0-9-]*[a-zA-Z0-9])?([/?][!-~]*)?$') THEN
   RAISE EXCEPTION 'invalid_url' USING ERRCODE='PT422'; END IF;
 END IF;

 -- Lock order (contract §4.1): order -> work/refund reads (no lock: a refund request holds this same
 -- order lock, so held capacity cannot grow under us) -> head.
 SELECT o.owner_id,o.fulfillment_state,o.collection_state INTO v_owner,v_fulfillment,v_collection FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=p_order FOR UPDATE;
 IF NOT FOUND THEN
  v_fail_code:='PT404'; v_fail_msg:='order not found';
 ELSE
  PERFORM set_config('app.buyer_id',v_owner::text,true);
  SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c
   WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store
    AND c.operation='fulfillment.manual_shipment.record' AND c.idempotency_key=p_key;
  IF FOUND THEN
   IF v_saved<>p_request_hash THEN v_fail_code:='PT409'; v_fail_msg:='idempotency_conflict';
   ELSE v_replay:=true; END IF;
  ELSE
   SELECT h.current_version INTO v_head FROM fulfillment.manual_shipment_heads h
    WHERE h.tenant_id=s.tenant_id AND h.store_id=p_store AND h.order_id=p_order FOR UPDATE;
   IF v_head IS NOT NULL THEN
    SELECT v.status INTO v_head_status FROM fulfillment.manual_shipment_versions v
     WHERE v.tenant_id=s.tenant_id AND v.store_id=p_store AND v.order_id=p_order AND v.version=v_head;
   END IF;
   IF coalesce(v_head,0)<>p_expected_version THEN
    v_fail_code:='PT409'; v_fail_msg:='version_changed';
   ELSIF p_status='VOIDED' THEN
    IF v_collection IS NOT NULL AND v_collection<>'PENDING' THEN
     v_fail_code:='PT409'; v_fail_msg:='collection_state_changed';
    ELSIF v_head_status IS DISTINCT FROM 'SHIPPED' OR v_fulfillment<>'MERCHANT_SHIPPED' THEN
     v_fail_code:='PT422'; v_fail_msg:='void_requires_shipped';
    ELSE v_action:='fulfillment.shipment_voided'; END IF;
   ELSIF v_head_status='SHIPPED' AND v_fulfillment='MERCHANT_SHIPPED' THEN
    v_action:='fulfillment.shipment_corrected';   -- MD7: correction needs no MD6 re-check
   ELSIF NOT fulfillment.manual_shipment_eligible(s.tenant_id,p_store,p_order) THEN
    v_fail_code:='PT422'; v_fail_msg:='not_shippable';
   ELSE v_action:='fulfillment.shipment_recorded'; END IF;
  END IF;
 END IF;

 IF v_fail_code IS NULL AND NOT v_replay THEN
  IF p_status='VOIDED' THEN
   -- A void copies the carrier and tracking of the version it voids (history reads without joins).
   SELECT v.carrier_code,v.carrier_name,v.tracking_number,v.tracking_url INTO v_prev
    FROM fulfillment.manual_shipment_versions v
    WHERE v.tenant_id=s.tenant_id AND v.store_id=p_store AND v.order_id=p_order AND v.version=v_head;
   INSERT INTO fulfillment.manual_shipment_versions(tenant_id,store_id,owner_id,order_id,version,status,
    carrier_code,carrier_name,tracking_number,tracking_url,note,void_reason,principal_id)
   VALUES(s.tenant_id,p_store,v_owner,p_order,v_head+1,'VOIDED',v_prev.carrier_code,v_prev.carrier_name,
    v_prev.tracking_number,v_prev.tracking_url,NULL,p_void_reason,s.principal_id)
   RETURNING * INTO v_row;
  ELSE
   INSERT INTO fulfillment.manual_shipment_versions(tenant_id,store_id,owner_id,order_id,version,status,
    carrier_code,carrier_name,tracking_number,tracking_url,note,void_reason,principal_id)
   VALUES(s.tenant_id,p_store,v_owner,p_order,coalesce(v_head,0)+1,'SHIPPED',p_carrier_code,p_carrier_name,
    p_tracking_number,p_tracking_url,p_note,NULL,s.principal_id)
   RETURNING * INTO v_row;
  END IF;
  v_now:=clock_timestamp();
  IF v_head IS NULL THEN
   INSERT INTO fulfillment.manual_shipment_heads(tenant_id,store_id,owner_id,order_id,current_version,updated_at)
   VALUES(s.tenant_id,p_store,v_owner,p_order,v_row.version,v_now);
  ELSE
   UPDATE fulfillment.manual_shipment_heads h SET current_version=v_row.version,updated_at=v_now
    WHERE h.tenant_id=s.tenant_id AND h.store_id=p_store AND h.order_id=p_order;
  END IF;
  -- §11.5 n/a: fulfilment state only; no stock, ledger, reservation or payment write (MD2, RD6). A correction (P1-3b) never
  -- rewrites the order row: a later write would move the collected cash to another finance day (finance groups on updated_at).
  IF v_action<>'fulfillment.shipment_corrected' THEN
   UPDATE checkout.orders o SET fulfillment_state=CASE WHEN p_status='SHIPPED' THEN 'MERCHANT_SHIPPED' ELSE 'MANUAL_UNASSIGNED' END,
    updated_at=v_now
    WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.owner_id=v_owner AND o.id=p_order;
  END IF;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(s.tenant_id,p_store,s.principal_id,v_action);
  v_response:=jsonb_build_object('version',v_row.version,'status',v_row.status,'carrier_code',v_row.carrier_code,
   'carrier_name',v_row.carrier_name,'tracking_number',v_row.tracking_number,'tracking_url',v_row.tracking_url,
   'recorded_at',to_char(v_row.recorded_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
   'note',v_row.note,'void_reason',v_row.void_reason,'principal_id',v_row.principal_id);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
  VALUES(s.tenant_id,p_store,'fulfillment.manual_shipment.record',p_key,p_request_hash,v_response,s.principal_id);
 END IF;

 -- Final authority after every lock wait and write, also on the refusal paths (no post-revocation
 -- existence oracle). A failure here aborts the transaction, so no row survives.
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 SELECT se.expires_at INTO v_expiry FROM identity.sessions se
  WHERE se.token_hash=p_hash AND se.audience='merchant' AND se.revoked_at IS NULL;
 IF v_final.access_status='unauthorized' OR v_expiry IS NULL OR v_expiry<=clock_timestamp() THEN
  RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION fulfillment.record_manual_shipment(bytea,uuid,uuid,text,bytea,bigint,text,text,text,text,text,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.record_manual_shipment(bytea,uuid,uuid,text,bytea,bigint,text,text,text,text,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.record_manual_shipment(bytea,uuid,uuid,text,bytea,bigint,text,text,text,text,text,text,text) TO commerce_runtime;

-- P2-4: the manual-shipment carrier enum gains black_cat/hsinchu (0063:59 column CHECK widened by shape).
DO $$
DECLARE v_def text;
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c
  WHERE c.conrelid='fulfillment.manual_shipment_versions'::regclass AND c.conname='manual_shipment_versions_carrier_code_check';
 IF v_def IS NULL OR v_def NOT LIKE '%''seven_eleven_cvs''%' OR v_def NOT LIKE '%''chunghwa_post''%'
  OR v_def NOT LIKE '%''other''%' OR v_def LIKE '%''black_cat''%' OR v_def LIKE '%''hsinchu''%' THEN
  RAISE EXCEPTION 'manual_shipment_versions_carrier_code_check has an unexpected shape: %',v_def; END IF;
 ALTER TABLE fulfillment.manual_shipment_versions DROP CONSTRAINT manual_shipment_versions_carrier_code_check;
 ALTER TABLE fulfillment.manual_shipment_versions ADD CONSTRAINT manual_shipment_versions_carrier_code_check
  CHECK(carrier_code IN ('seven_eleven_cvs','familymart_cvs','hilife_cvs','okmart_cvs','sf_express','chunghwa_post','black_cat','hsinchu','other'));
END $$;

-- notify.on_order_insert (0090 body): a cash_on_delivery order placed AWAITING_COLLECTION is "placed" exactly like pay_at_pickup.
CREATE OR REPLACE FUNCTION notify.on_order_insert() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.commercial_state='AWAITING_TRANSFER' OR (NEW.commercial_state='CONFIRMED' AND NEW.payment_mode='pay_at_pickup')
  OR NEW.commercial_state='AWAITING_COLLECTION' THEN
  PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'placed');
  PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'merchant_new');
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION notify.on_order_insert() OWNER TO commerce_checkout_writer;

-- notify.on_order_update (0090 body): cancelling an AWAITING_COLLECTION order (release-cancel) also notifies.
CREATE OR REPLACE FUNCTION notify.on_order_update() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.commercial_state IS DISTINCT FROM OLD.commercial_state THEN
  IF NEW.commercial_state='AWAITING_TRANSFER' THEN
   PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'placed');
   PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'merchant_new');
  ELSIF NEW.commercial_state='CONFIRMED' THEN
   IF NEW.payment_mode='pay_at_pickup' THEN
    PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'placed');
    PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'merchant_new');
   ELSE
    PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'paid');
    IF NEW.payment_mode='card' AND OLD.commercial_state IN ('DRAFT','AWAITING_PAYMENT') THEN
     PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'merchant_new');
    END IF;
   END IF;
  ELSIF NEW.commercial_state='CANCELLED' AND OLD.commercial_state IN ('AWAITING_TRANSFER','CONFIRMED','AWAITING_COLLECTION') THEN
   PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'cancelled');
  END IF;
 END IF;
 IF NEW.fulfillment_state IS DISTINCT FROM OLD.fulfillment_state AND NEW.fulfillment_state='MERCHANT_SHIPPED' THEN
  PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'shipped');
 END IF;
 IF NEW.collection_state IS DISTINCT FROM OLD.collection_state AND NEW.collection_state='REFUNDED_OFFLINE' THEN
  PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'refunded');
 END IF;
 IF OLD.buyer_email IS NOT NULL AND NEW.buyer_email IS NULL THEN
  UPDATE notify.outbox SET recipient_hash=NULL,updated_at=clock_timestamp(),
    state=CASE WHEN state='PENDING' THEN 'SKIPPED' ELSE state END,
    skip_reason=CASE WHEN state='PENDING' THEN 'erased' ELSE skip_reason END
   WHERE order_id=NEW.id AND kind<>'merchant_new';
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION notify.on_order_update() OWNER TO commerce_checkout_writer;

-- ---------------------------------------------------------------------------------------------------
-- D. Merchant projections patched in place (verified by shape) instead of re-copying the long definers; each patch fails loudly
-- when the live definition is not the expected one (0088:657 pattern). Targets the POST-0088 bodies.
-- ---------------------------------------------------------------------------------------------------
DO $$
DECLARE v_item record; v_fn text;
BEGIN
 FOR v_item IN SELECT * FROM (VALUES
  -- P2-5: the validation list admits AWAITING_COLLECTION and the filter matches it on collection_state PENDING (not on
  -- commercial_state, which stays AWAITING_COLLECTION after COLLECTED/RETURNED/... by design). Two expect=1 patches, not the
  -- one expect=2 pair, because the filter now diverges from the validation list.
  ('identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text)',
   'NOT IN (''all'',''DRAFT'',''AWAITING_PAYMENT'',''AWAITING_TRANSFER'',''CONFIRMED'',''CANCELLED'',''shipped'',''unshipped'',''cvs_pending'')',
   'NOT IN (''all'',''DRAFT'',''AWAITING_PAYMENT'',''AWAITING_TRANSFER'',''AWAITING_COLLECTION'',''CONFIRMED'',''CANCELLED'',''shipped'',''unshipped'',''cvs_pending'')',1),
  ('identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text)',
   '(p_state IN (''DRAFT'',''AWAITING_PAYMENT'',''AWAITING_TRANSFER'',''CONFIRMED'',''CANCELLED'') AND o.commercial_state=p_state)',
   '(p_state IN (''DRAFT'',''AWAITING_PAYMENT'',''AWAITING_TRANSFER'',''CONFIRMED'',''CANCELLED'') AND o.commercial_state=p_state) OR (p_state=''AWAITING_COLLECTION'' AND o.commercial_state=''AWAITING_COLLECTION'' AND o.collection_state=''PENDING'')',1),
  ('identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text)',
   '(p_state=''unshipped'' AND o.commercial_state=''CONFIRMED'' AND o.fulfillment_state=''MANUAL_UNASSIGNED''',
   '(p_state=''unshipped'' AND o.commercial_state IN (''CONFIRMED'',''AWAITING_COLLECTION'') AND o.fulfillment_state=''MANUAL_UNASSIGNED''',1),
  ('identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text)',
   'o.payment_mode,o.collection_state,o.snapshot#>>''{destination,pickup,verification_kind}'' AS pickup_vk,',
   'o.payment_mode,o.collection_state,o.cod_surcharge_minor,o.snapshot#>>''{destination,pickup,verification_kind}'' AS pickup_vk,',1),
  -- P1-2: the merchant order DTO also carries cod_collect_minor = total + surcharge (the collect amount), next to the surcharge.
  ('identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text)',
   ',''payment_mode'',o.payment_mode,''collection_state'',o.collection_state,',
   ',''payment_mode'',o.payment_mode,''collection_state'',o.collection_state,''cod_surcharge_minor'',o.cod_surcharge_minor,''cod_collect_minor'',CASE WHEN o.payment_mode=''cash_on_delivery'' THEN o.total_minor+coalesce(o.cod_surcharge_minor,0) END,',1),
  ('identity.export_unshipped_orders(bytea,uuid,integer)',
   'AND o.commercial_state=''CONFIRMED''',
   'AND o.commercial_state IN (''CONFIRMED'',''AWAITING_COLLECTION'')',1),
  -- P1-2: the unshipped export appends payment_mode and collect_minor after pickup_source (B19: appending is backward compatible).
  ('identity.export_unshipped_orders(bytea,uuid,integer)',
   'WHEN ''MANUAL_ATTESTED'' THEN ''merchant_attested'' ELSE '''' END)',
   'WHEN ''MANUAL_ATTESTED'' THEN ''merchant_attested'' ELSE '''' END,''payment_mode'',o.payment_mode,''collect_minor'',o.total_minor+coalesce(o.cod_surcharge_minor,0))',1),
  ('identity.dashboard_todos(bytea,uuid)',
   'AND o.commercial_state=''CONFIRMED'' AND o.fulfillment_state=''MANUAL_UNASSIGNED''',
   'AND o.commercial_state IN (''CONFIRMED'',''AWAITING_COLLECTION'') AND o.fulfillment_state=''MANUAL_UNASSIGNED''',1),
  ('identity.dashboard_orders(bytea,uuid)',
   'AND o.commercial_state IN (''AWAITING_PAYMENT'',''AWAITING_TRANSFER'',''CONFIRMED'')',
   'AND o.commercial_state IN (''AWAITING_PAYMENT'',''AWAITING_TRANSFER'',''AWAITING_COLLECTION'',''CONFIRMED'')',1)
 ) AS t(fn,needle,repl,expect) LOOP
  v_fn:=pg_get_functiondef(v_item.fn::regprocedure);
  IF (length(v_fn)-length(replace(v_fn,v_item.needle,'')))<>length(v_item.needle)*v_item.expect THEN
   RAISE EXCEPTION '% has an unexpected shape for patch %',v_item.fn,left(v_item.needle,60); END IF;
  EXECUTE replace(v_fn,v_item.needle,v_item.repl);
 END LOOP;
END $$;

-- ---------------------------------------------------------------------------------------------------
-- E. Finance (0088:689 body): the projection gains two COD keys and the pickup/COD day moves from updated_at to collected_at.
-- COD collected (order COLLECTED on the day collected_at was set by record_collection; offline money is always environment LIVE;
-- the collect amount is total_minor + the placement-time surcharge). Never part of captured/net: the cash never touches a PSP.
-- ---------------------------------------------------------------------------------------------------
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
  SELECT (o.collected_at AT TIME ZONE 'Asia/Taipei')::date AS day,o.currency,
   coalesce((SELECT c.environment FROM fulfillment.cvs_shipments c WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id
     AND c.order_id=o.id ORDER BY c.attempt DESC LIMIT 1),'LIVE') AS environment,
   count(*)::bigint AS n,sum(o.total_minor)::bigint AS minor
  FROM checkout.orders o,lim
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.payment_mode='pay_at_pickup' AND o.collection_state='COLLECTED'
   AND o.collected_at>=lim.t0 AND o.collected_at<lim.t1
  GROUP BY 1,2,3
 ), trf AS (
  SELECT (t.confirmed_at AT TIME ZONE 'Asia/Taipei')::date AS day,t.currency,'LIVE'::text AS environment,
   count(*)::bigint AS n,sum(t.confirmed_amount_minor)::bigint AS minor
  FROM checkout.bank_transfers t,lim
  WHERE t.tenant_id=s.tenant_id AND t.store_id=p_store AND t.state='CONFIRMED'
   AND t.confirmed_at>=lim.t0 AND t.confirmed_at<lim.t1
  GROUP BY 1,2,3
 ), cod AS (
  -- home-cod: the order total plus the placement-time surcharge of a COLLECTED cash_on_delivery order, on the day collected_at was
  -- set by record_collection (never moved by a later return/refund/correction/erasure). Always environment LIVE.
  SELECT (o.collected_at AT TIME ZONE 'Asia/Taipei')::date AS day,o.currency,'LIVE'::text AS environment,
   count(*)::bigint AS n,sum(o.total_minor + o.cod_surcharge_minor)::bigint AS minor
  FROM checkout.orders o,lim
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.payment_mode='cash_on_delivery' AND o.collection_state='COLLECTED'
   AND o.collected_at>=lim.t0 AND o.collected_at<lim.t1
  GROUP BY 1,2,3
 ), keys AS (
  SELECT day,currency,environment FROM cap UNION SELECT day,currency,environment FROM ref
  UNION SELECT day,currency,environment FROM pick UNION SELECT day,currency,environment FROM trf
  UNION SELECT day,currency,environment FROM cod
 )
 SELECT coalesce(jsonb_agg(jsonb_build_object('day',to_char(k.day,'YYYY-MM-DD'),'currency',k.currency,
   'environment',k.environment,'captured_count',coalesce(c.n,0),'captured_minor',coalesce(c.minor,0),
   'refunded_minor',coalesce(r.minor,0),'net_minor',coalesce(c.minor,0)-coalesce(r.minor,0),
   'pickup_collected_count',coalesce(p.n,0),'pickup_collected_minor',coalesce(p.minor,0),
   'bank_transfer_confirmed_count',coalesce(t.n,0),'bank_transfer_confirmed_minor',coalesce(t.minor,0),
   'cod_collected_count',coalesce(d.n,0),'cod_collected_minor',coalesce(d.minor,0))
   ORDER BY k.day,k.currency,k.environment),'[]'::jsonb) INTO v_rows
 FROM keys k
 LEFT JOIN cap c ON c.day=k.day AND c.currency=k.currency AND c.environment=k.environment
 LEFT JOIN ref r ON r.day=k.day AND r.currency=k.currency AND r.environment=k.environment
 LEFT JOIN pick p ON p.day=k.day AND p.currency=k.currency AND p.environment=k.environment
 LEFT JOIN trf t ON t.day=k.day AND t.currency=k.currency AND t.environment=k.environment
 LEFT JOIN cod d ON d.day=k.day AND d.currency=k.currency AND d.environment=k.environment;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'finance read access denied' USING ERRCODE=v_auth_error; END IF;
 IF octet_length(v_rows::text)>1048576 THEN RAISE EXCEPTION 'finance read unavailable' USING ERRCODE='PT503'; END IF;
 RETURN v_rows;
END $$;

-- ---------------------------------------------------------------------------------------------------
-- F. Buyer mails (0098:19 body): the placed/shipped COD mail states the cash due on delivery. Two patches — read the
-- surcharge snapshot into the o record, then fold it into a cod_collect_minor payload key (null for non-COD orders).
-- ---------------------------------------------------------------------------------------------------
DO $$
DECLARE v_item record; v_fn text;
BEGIN
 FOR v_item IN SELECT * FROM (VALUES
  ('notify.claim_batch(integer,integer,integer)',
   'k.owner_id,k.buyer_email,k.payment_mode,k.commercial_state,k.total_minor,k.currency,k.expires_at,k.destination_id,k.locale',
   'k.owner_id,k.buyer_email,k.payment_mode,k.commercial_state,k.total_minor,k.currency,k.expires_at,k.destination_id,k.locale,k.cod_surcharge_minor',1),
  ('notify.claim_batch(integer,integer,integer)',
   '''total_minor'',o.total_minor,''currency'',o.currency,''payment_mode'',o.payment_mode,',
   '''total_minor'',o.total_minor,''currency'',o.currency,''payment_mode'',o.payment_mode,''cod_collect_minor'',CASE WHEN o.payment_mode=''cash_on_delivery'' THEN o.total_minor+coalesce(o.cod_surcharge_minor,0) END,',1)
 ) AS t(fn,needle,repl,expect) LOOP
  v_fn:=pg_get_functiondef(v_item.fn::regprocedure);
  IF (length(v_fn)-length(replace(v_fn,v_item.needle,'')))<>length(v_item.needle)*v_item.expect THEN
   RAISE EXCEPTION '% has an unexpected shape for patch %',v_item.fn,left(v_item.needle,60); END IF;
  EXECUTE replace(v_fn,v_item.needle,v_item.repl);
 END LOOP;
END $$;

-- ---------------------------------------------------------------------------------------------------
-- G. Erasure in-flight guard (0078:515 body, P2-11): a shipped-but-uncollected COD or pay_at_pickup
-- order is carrying the recipient snapshot the carrier's cash reconciliation needs, so erasure waits
-- while it is in flight (shipped: fulfillment_state MERCHANT_SHIPPED or PROVIDER_LABEL_CREATED, and
-- collection_state PENDING — the same "shipped" predicate record_collection gates collected on). The CD7
-- refusal gains one more disjunct. Least privilege (customers-billing-v1 §3.1 / D-S2 freeze the column list of
-- commerce_privacy_writer on checkout.orders): erase_owner does NOT get payment_mode/collection_state/fulfillment_state
-- column SELECT; it calls this narrow boolean predicate, a SECURITY DEFINER owned like its checkout siblings by
-- commerce_checkout_writer (which already reads checkout.orders), search_path pinned, EXECUTE only to commerce_privacy_writer.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION checkout.has_inflight_collection(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.owner_id=p_owner
  AND o.payment_mode IN ('cash_on_delivery','pay_at_pickup') AND o.collection_state='PENDING'
  AND o.fulfillment_state IN ('MERCHANT_SHIPPED','PROVIDER_LABEL_CREATED'))
$$;
ALTER FUNCTION checkout.has_inflight_collection(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.has_inflight_collection(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.has_inflight_collection(uuid,uuid,uuid) TO commerce_privacy_writer;
DO $$
DECLARE v_item record; v_fn text;
BEGIN
 FOR v_item IN SELECT * FROM (VALUES
  ('customers.erase_owner(bytea,uuid,uuid,text,uuid)',
   'AND o.commercial_state IN (''DRAFT'',''AWAITING_PAYMENT'',''AWAITING_TRANSFER'') AND o.expires_at>clock_timestamp())',
   E'AND o.commercial_state IN (''DRAFT'',''AWAITING_PAYMENT'',''AWAITING_TRANSFER'') AND o.expires_at>clock_timestamp())\n   OR checkout.has_inflight_collection(v_tenant,p_store,v_owner)',1)
 ) AS t(fn,needle,repl,expect) LOOP
  v_fn:=pg_get_functiondef(v_item.fn::regprocedure);
  IF (length(v_fn)-length(replace(v_fn,v_item.needle,'')))<>length(v_item.needle)*v_item.expect THEN
   RAISE EXCEPTION '% has an unexpected shape for patch %',v_item.fn,left(v_item.needle,60); END IF;
  EXECUTE replace(v_fn,v_item.needle,v_item.repl);
 END LOOP;
END $$;

-- ---------------------------------------------------------------------------------------------------
-- Documentation (PROCESS §5): owning package, allowed roles, non-goals.
-- ---------------------------------------------------------------------------------------------------
COMMENT ON TABLE checkout.cash_on_delivery_settings IS 'internal/checkout (cash on delivery): per-store COD switch, per-order cap (whole TWD), optional whole-TWD surcharge and the carrier label (black_cat 黑猫 / hsinchu 新竹; manual fulfilment, no carrier API). Written only by payments.set_cash_on_delivery_settings (integration:manage), read by begin_hold and read_cod_offer. No grant to runtime or buyer roles. Non-goal: no carrier API data; a missing row means off.';
COMMENT ON COLUMN checkout.cash_on_delivery_settings.max_twd IS 'Per-order COD cap in whole TWD (1..20000): begin_hold refuses a COD order whose total_minor + surcharge exceeds max_twd*100.';
COMMENT ON COLUMN checkout.cash_on_delivery_settings.surcharge_twd IS 'Optional COD surcharge in whole TWD (0..1000, 0 = none); begin_hold snapshots surcharge_twd*100 into checkout.orders.cod_surcharge_minor, which the buyer pays on delivery on top of the order total.';
COMMENT ON COLUMN checkout.cash_on_delivery_settings.carrier IS 'Manual-fulfilment label (black_cat 黑猫 / hsinchu 新竹) shown to the buyer at checkout and the merchant on the order; never a carrier API integration.';
COMMENT ON FUNCTION payments.read_cash_on_delivery_settings(bytea,uuid) IS 'internal/merchantorders only; EXECUTE commerce_runtime. integration:read, GUCs from resolve_access, fresh final fence. No row = off at version 0.';
COMMENT ON FUNCTION payments.set_cash_on_delivery_settings(bytea,uuid,text,bytea,bigint,boolean,integer,integer,text) IS 'internal/merchantorders only; EXECUTE commerce_runtime. integration:manage, version CAS (0 inserts), idempotent receipt in ops.command_results, one audit row checkout.cash_on_delivery_settings_changed. Never touches placed orders.';
COMMENT ON FUNCTION checkout.has_inflight_collection(uuid,uuid,uuid) IS 'internal/customers erasure only (customers.erase_owner, owner commerce_privacy_writer); EXECUTE commerce_privacy_writer. True when the owner has a shipped (MERCHANT_SHIPPED or PROVIDER_LABEL_CREATED) cash_on_delivery or pay_at_pickup order still collection_state PENDING. Narrow predicate so the privacy writer holds no payment_mode/collection_state/fulfillment_state column grant; never returns row data.';
COMMENT ON FUNCTION checkout.read_cod_offer(bytea,uuid) IS 'internal/checkout options only; EXECUTE commerce_checkout_runtime. Buyer-scope read of enabled/surcharge_twd/carrier/max_twd; never money details beyond the public offer.';
COMMENT ON FUNCTION identity.read_finance_summary(bytea,uuid,date,date) IS 'internal/reporting only; EXECUTE commerce_runtime. orders:read; daily captured/refunded/net by currency and environment plus the pay-at-pickup collected, bank-transfer confirmed and cash-on-delivery collected columns (never part of captured/net).';
