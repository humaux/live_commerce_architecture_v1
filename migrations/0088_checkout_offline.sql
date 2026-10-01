-- 0088 checkout-offline (contracts/storefront-v2.md §C, FROZEN; unit docs/delivery/units/checkout-offline.md).
--
-- Owns: (1) the bank_transfer payment mode as a THIRD mode next to card and pay_at_pickup: the order is placed AWAITING_TRANSFER with its
-- stock RESERVED (reservation HELD) until the merchant's transfer window ends; the buyer may submit {last5, amount, paid_at} (editable
-- until confirmed); the merchant CONFIRMS (order CONFIRMED, reservation COMMITTED, ledger ALLOCATE) or REJECTS the submission (the order
-- stays open until the window ends); expiry goes through the SAME checkout.expire_held / River job as a card hold. (2) the per-policy free
-- shipping threshold column read by the Go quote. (3) the buyer email column on checkout.orders (PII: export + erasure included).
-- (4) finance: identity.read_finance_summary gains the pay-at-pickup collected columns (ops-polish OP3, which never got its migration) and
-- the new bank-transfer confirmed columns.
--
-- Non-goals: no PSP, no payment attempt, no payments.facts row for a transfer (the offline fact is checkout.bank_transfers.confirmed_*),
-- no auto-confirmation of any kind (only a merchant act with payments:refund confirms; there is no webhook, poll or bank feed), no
-- amount from the client (confirmed_amount_minor is the server order total; the buyer's claimed amount is a hint the merchant reads),
-- no buyer-visible bank details outside the buyer's own order (checkout.read_bank_transfer_buyer).
-- begin_hold itself is re-created in post_river/0018 (post_river/0017 drops and re-creates it after every main migration, so a main
-- migration cannot own it). Callers: internal/checkout (buyer), internal/merchantorders (merchant), internal/reporting (finance).

-- ---------------------------------------------------------------------------------------------------
-- A. Free-shipping threshold (storefront-v2 §C): NULL = no threshold. Applied by pricing.Calculate in the server quote.
-- ---------------------------------------------------------------------------------------------------
ALTER TABLE pricing.policy_versions ADD COLUMN free_shipping_threshold_minor bigint
 CHECK(free_shipping_threshold_minor IS NULL OR free_shipping_threshold_minor BETWEEN 0 AND 1000000000000);
-- The buyer quote path (commerce_buyer_runtime) reads policy columns through an explicit column grant (0007:157).
GRANT SELECT(free_shipping_threshold_minor) ON pricing.policy_versions TO commerce_buyer_runtime;
COMMENT ON COLUMN pricing.policy_versions.free_shipping_threshold_minor IS 'internal/pricing: merchandise subtotal (minor units) at or above which the quote charges shipping 0; NULL = never free. Written by pricing.SetPolicy (merchant), read by pricing.LockCurrent; the quote snapshot freezes the outcome.';

-- ---------------------------------------------------------------------------------------------------
-- B. checkout.orders: bank_transfer mode, AWAITING_TRANSFER state, long hold, buyer email.
-- ---------------------------------------------------------------------------------------------------
DO $$
DECLARE v_item record; v_def text; v_name text;
BEGIN
 FOR v_item IN SELECT * FROM (VALUES
  ('checkout.orders','orders_payment_mode_check','''pay_at_pickup''::text]','''pay_at_pickup''::text, ''bank_transfer''::text]','bank_transfer'),
  ('checkout.orders','orders_commercial_state_check','''CANCELLED''::text]','''CANCELLED''::text, ''AWAITING_TRANSFER''::text]','AWAITING_TRANSFER'),
  ('checkout.events','events_action_check','''checkout.pay_at_pickup_placed''::text]',
   '''checkout.pay_at_pickup_placed''::text, ''checkout.bank_transfer_placed''::text, ''checkout.transfer_proof_submitted''::text]','bank_transfer_placed'),
  ('checkout.events','events_actor_action','''checkout.pay_at_pickup_placed''::text]',
   '''checkout.pay_at_pickup_placed''::text, ''checkout.bank_transfer_placed''::text, ''checkout.transfer_proof_submitted''::text]','bank_transfer_placed'),
  ('checkout.command_results','command_results_operation_check','''checkout.payment.start''::text]',
   '''checkout.payment.start''::text, ''checkout.transfer.submit''::text]','transfer.submit')
 ) AS t(rel,con,needle,repl,absent) LOOP
  SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c WHERE c.conrelid=v_item.rel::regclass AND c.conname=v_item.con;
  IF v_def IS NULL OR v_def LIKE '%'||v_item.absent||'%'
   OR length(v_def)-length(replace(v_def,v_item.needle,''))<>length(v_item.needle) THEN
   RAISE EXCEPTION '% % has an unexpected shape: %',v_item.rel,v_item.con,v_def; END IF;
  EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I',v_item.rel,v_item.con);
  EXECUTE format('ALTER TABLE %s ADD CONSTRAINT %I %s',v_item.rel,v_item.con,replace(v_def,v_item.needle,v_item.repl));
 END LOOP;
 -- 0013:40 caps every hold at 15 minutes; a bank-transfer hold lasts the merchant's window (6..168 h). The unnamed CHECK is found by shape.
 SELECT c.conname INTO v_name FROM pg_constraint c WHERE c.conrelid='checkout.orders'::regclass AND c.contype='c'
  AND pg_get_constraintdef(c.oid) LIKE '%expires_at > created_at%' AND pg_get_constraintdef(c.oid) LIKE '%00:15:01%';
 IF v_name IS NULL THEN RAISE EXCEPTION 'checkout.orders expiry CHECK not found'; END IF;
 EXECUTE format('ALTER TABLE checkout.orders DROP CONSTRAINT %I',v_name);
 ALTER TABLE checkout.orders ADD CONSTRAINT orders_expiry_window CHECK(expires_at>created_at AND
  (expires_at<=created_at+interval '15 minutes 1 second'
   OR (payment_mode='bank_transfer' AND expires_at<=created_at+interval '168 hours 1 second')));
END $$;
-- One live checkout per cart version now includes the transfer wait (0013:43).
DROP INDEX checkout.checkout_one_active_cart_version;
CREATE UNIQUE INDEX checkout_one_active_cart_version ON checkout.orders(tenant_id,store_id,owner_id,cart_id,cart_version)
 WHERE commercial_state IN ('DRAFT','AWAITING_PAYMENT','AWAITING_TRANSFER','CONFIRMED');
ALTER TABLE checkout.orders ADD COLUMN buyer_email text
 CHECK(buyer_email IS NULL OR (char_length(buyer_email) BETWEEN 3 AND 254 AND buyer_email ~ '^[^[:space:][:cntrl:]@]+@[^[:space:][:cntrl:]@]+$'));
COMMENT ON COLUMN checkout.orders.buyer_email IS 'internal/checkout: optional buyer email captured at checkout for notifications (PII). Set once by checkout.set_order_buyer_email in the Begin transaction; exported by customers.buyer_export_orders and set NULL by customers.apply_erasure through checkout.clear_buyer_email.';
-- The Begin transaction attaches the email through a definer (commerce_checkout_runtime has no UPDATE on orders).
GRANT UPDATE(buyer_email) ON checkout.orders TO commerce_checkout_writer;
-- Erasure (0078 apply_erasure) clears it through checkout.clear_buyer_email below, NOT through a column grant: the frozen customers-billing-v1
-- §3.1 privilege list of commerce_privacy_writer (enforced by TestCustomersBillingCB02Schema) stays exactly as it was.
CREATE FUNCTION checkout.clear_buyer_email(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS integer
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 WITH cleared AS (UPDATE checkout.orders SET buyer_email=NULL
   WHERE tenant_id=p_tenant AND store_id=p_store AND owner_id=p_owner AND buyer_email IS NOT NULL RETURNING 1)
 SELECT count(*)::integer FROM cleared
$$;
ALTER FUNCTION checkout.clear_buyer_email(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.clear_buyer_email(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.clear_buyer_email(uuid,uuid,uuid) TO commerce_privacy_writer;
COMMENT ON FUNCTION checkout.clear_buyer_email(uuid,uuid,uuid) IS 'customers.apply_erasure only (the definer of that function, commerce_privacy_writer, holds the sole EXECUTE): sets checkout.orders.buyer_email NULL for one owner and returns the row count. Non-goal: no other column, no authorization of its own (the caller holds the owner row lock and the scope).';

-- ---------------------------------------------------------------------------------------------------
-- C. Tables. Both FORCE RLS; only commerce_checkout_writer (the owner of every definer below) and, for finance, commerce_auth see them.
-- ---------------------------------------------------------------------------------------------------
CREATE TABLE checkout.bank_transfer_settings (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 enabled boolean NOT NULL DEFAULT false,
 allow_cvs boolean NOT NULL DEFAULT false,
 bank_name text NOT NULL DEFAULT '' CHECK(char_length(bank_name)<=60 AND bank_name !~ '[[:cntrl:]]'),
 branch text NOT NULL DEFAULT '' CHECK(char_length(branch)<=60 AND branch !~ '[[:cntrl:]]'),
 account_name text NOT NULL DEFAULT '' CHECK(char_length(account_name)<=60 AND account_name !~ '[[:cntrl:]]'),
 account_number text NOT NULL DEFAULT '' CHECK(account_number ~ '^([0-9][0-9 -]{3,31})?$'),
 window_hours integer NOT NULL DEFAULT 72 CHECK(window_hours BETWEEN 6 AND 168),
 version bigint NOT NULL CHECK(version>0), updated_at timestamptz NOT NULL,
 PRIMARY KEY(tenant_id,store_id),
 CHECK(NOT enabled OR (bank_name<>'' AND account_name<>'' AND account_number<>'')),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
CREATE TABLE checkout.bank_transfers (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, owner_id uuid NOT NULL, order_id uuid NOT NULL,
 state text NOT NULL CHECK(state IN ('AWAITING','SUBMITTED','REJECTED','CONFIRMED','EXPIRED','REFUNDED_OFFLINE')),
 -- Snapshot of the merchant's details at placement: a later settings change never moves an open order's account.
 bank_name text NOT NULL, branch text NOT NULL, account_name text NOT NULL, account_number text NOT NULL,
 window_hours integer NOT NULL CHECK(window_hours BETWEEN 6 AND 168),
 proof_last5 text CHECK(proof_last5 ~ '^[0-9]{5}$'),
 proof_amount_minor bigint CHECK(proof_amount_minor BETWEEN 1 AND 1000000000000),
 proof_paid_at timestamptz, proof_submitted_at timestamptz, proof_count integer NOT NULL DEFAULT 0 CHECK(proof_count>=0),
 reject_reason text CHECK(char_length(reject_reason) BETWEEN 1 AND 200 AND reject_reason !~ '[[:cntrl:]]'), rejected_at timestamptz,
 -- The offline fact (finance, shippability): server order total, merchant principal, time. Kept after an offline refund.
 confirmed_at timestamptz, confirmed_by uuid, confirmed_amount_minor bigint CHECK(confirmed_amount_minor BETWEEN 0 AND 1000000000000),
 currency text NOT NULL CHECK(currency ~ '^[A-Z]{3}$'),
 refunded_at timestamptz, refunded_by uuid,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY(tenant_id,store_id,order_id),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,confirmed_by) REFERENCES identity.memberships(tenant_id,principal_id),
 FOREIGN KEY(tenant_id,refunded_by) REFERENCES identity.memberships(tenant_id,principal_id),
 CHECK((state IN ('CONFIRMED','REFUNDED_OFFLINE'))=(confirmed_at IS NOT NULL AND confirmed_by IS NOT NULL AND confirmed_amount_minor IS NOT NULL)),
 CHECK((state='REFUNDED_OFFLINE')=(refunded_at IS NOT NULL AND refunded_by IS NOT NULL)),
 CHECK(state<>'SUBMITTED' OR proof_last5 IS NOT NULL),
 CHECK((reject_reason IS NULL)=(rejected_at IS NULL)),
 CHECK(state<>'REJECTED' OR reject_reason IS NOT NULL)
);
CREATE INDEX bank_transfers_open ON checkout.bank_transfers(tenant_id,store_id,state) WHERE state IN ('AWAITING','SUBMITTED','REJECTED');
DO $$ DECLARE r text; BEGIN
 FOREACH r IN ARRAY ARRAY['checkout.bank_transfer_settings','checkout.bank_transfers'] LOOP
  EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY',r);
  EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY',r);
  EXECUTE format('REVOKE ALL ON %s FROM PUBLIC',r);
 END LOOP;
END $$;
GRANT SELECT,INSERT ON checkout.bank_transfer_settings,checkout.bank_transfers TO commerce_checkout_writer;
GRANT UPDATE(enabled,allow_cvs,bank_name,branch,account_name,account_number,window_hours,version,updated_at) ON checkout.bank_transfer_settings TO commerce_checkout_writer;
GRANT UPDATE(state,proof_last5,proof_amount_minor,proof_paid_at,proof_submitted_at,proof_count,reject_reason,rejected_at,confirmed_at,confirmed_by,
 confirmed_amount_minor,refunded_at,refunded_by,updated_at) ON checkout.bank_transfers TO commerce_checkout_writer;
-- GUC-scoped like cvs_store_settings (0073): the definers set app.tenant_id/app.store_id first. One policy per table covers the FOR SHARE lock begin_hold takes.
CREATE POLICY bank_transfer_settings_writer ON checkout.bank_transfer_settings TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY bank_transfers_writer ON checkout.bank_transfers TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
-- commerce_auth (finance definer, order_money_shippable): read-only projection columns, scoped by the definer's own tenant/store filter (0073:175 pattern).
GRANT SELECT(tenant_id,store_id,owner_id,order_id,state,confirmed_at,confirmed_amount_minor,currency) ON checkout.bank_transfers TO commerce_auth;
CREATE POLICY bank_transfers_auth_read ON checkout.bank_transfers FOR SELECT TO commerce_auth USING(true);
-- Finance pay-at-pickup column environment (ops-polish OP3 proposal): the latest shipment attempt's environment.
GRANT SELECT(environment) ON fulfillment.cvs_shipments TO commerce_auth;

-- ---------------------------------------------------------------------------------------------------
-- D. Inventory ledger: ONE new row shape, MERCHANT ALLOCATE for a confirmed transfer (the only way a transfer order commits stock).
-- ---------------------------------------------------------------------------------------------------
DO $$
DECLARE v_def text; v_anchor text:=' OR ((actor_kind = ''SYSTEM_PAYMENT''::text)'; v_fn text;
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c WHERE c.conrelid='inventory.ledger'::regclass AND c.conname='ledger_checkout_actor';
 IF v_def IS NULL OR v_def LIKE '%checkout.bank_transfer.confirm%' OR length(v_def)-length(replace(v_def,v_anchor,''))<>length(v_anchor) THEN
  RAISE EXCEPTION 'inventory.ledger ledger_checkout_actor has an unexpected shape: %',v_def; END IF;
 ALTER TABLE inventory.ledger DROP CONSTRAINT ledger_checkout_actor;
 EXECUTE format('ALTER TABLE inventory.ledger ADD CONSTRAINT ledger_checkout_actor %s',replace(v_def,v_anchor,
  ' OR ((actor_kind = ''MERCHANT''::text) AND (kind = ''ALLOCATE''::text) AND (principal_id IS NOT NULL)'||
  ' AND (checkout_id IS NOT NULL) AND (buyer_owner_id IS NOT NULL) AND (buyer_session_id IS NOT NULL)'||
  ' AND (reservation_id = checkout_id) AND (payment_attempt_id IS NULL) AND (payment_fact_kind IS NULL)'||
  ' AND (operation = ''checkout.bank_transfer.confirm''::text) AND (command_key = (checkout_id)::text))'||v_anchor));
 -- 0072 guard_checkout_ledger: the merchant-with-checkout-columns branch (DEALLOCATE) also covers this ALLOCATE (same principal and buyer GUC rule).
 v_fn:=pg_get_functiondef('inventory.guard_checkout_ledger()'::regprocedure);
 IF length(v_fn)-length(replace(v_fn,'NEW.actor_kind=''MERCHANT'' AND NEW.kind=''DEALLOCATE''',''))<>length('NEW.actor_kind=''MERCHANT'' AND NEW.kind=''DEALLOCATE''') THEN
  RAISE EXCEPTION 'inventory.guard_checkout_ledger has an unexpected shape'; END IF;
 EXECUTE replace(v_fn,'NEW.actor_kind=''MERCHANT'' AND NEW.kind=''DEALLOCATE''','NEW.actor_kind=''MERCHANT'' AND NEW.kind IN (''DEALLOCATE'',''ALLOCATE'')');
END $$;
CREATE POLICY checkout_writer_bank_transfer_allocate ON inventory.ledger FOR INSERT TO commerce_checkout_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND actor_kind='MERCHANT' AND kind='ALLOCATE' AND principal_id IS NOT NULL AND operation='checkout.bank_transfer.confirm'
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND buyer_session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
-- One confirm allocation per order line, ever (a replay with a new key cannot double-allocate; mirrors ledger_pay_at_pickup_release_once).
CREATE UNIQUE INDEX ledger_bank_transfer_confirm_once ON inventory.ledger(tenant_id,store_id,checkout_id,warehouse_id,sku_id)
 WHERE operation='checkout.bank_transfer.confirm';
-- Provenance (mirror of inventory.guard_pay_at_pickup_ledger): the only MERCHANT ALLOCATE is the confirm of a CONFIRMED bank_transfer order whose
-- transfer row is CONFIRMED, for exactly the reserved quantity, on an order with no payment attempt.
CREATE FUNCTION inventory.guard_bank_transfer_ledger() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.actor_kind='MERCHANT' AND NEW.kind='ALLOCATE' THEN
  IF NOT EXISTS(SELECT 1 FROM checkout.orders o JOIN checkout.bank_transfers t ON t.tenant_id=o.tenant_id AND t.store_id=o.store_id
     AND t.owner_id=o.owner_id AND t.order_id=o.id
    WHERE o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id AND o.owner_id=NEW.buyer_owner_id AND o.id=NEW.checkout_id
     AND o.creator_session_id=NEW.buyer_session_id AND o.payment_mode='bank_transfer' AND o.commercial_state='CONFIRMED'
     AND t.state='CONFIRMED' AND t.confirmed_by=NEW.principal_id)
   OR NOT EXISTS(SELECT 1 FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id AND l.store_id=NEW.store_id
    AND l.reservation_id=NEW.reservation_id AND l.warehouse_id=NEW.warehouse_id AND l.sku_id=NEW.sku_id AND l.quantity=-NEW.delta_reserved)
   OR EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id AND a.order_id=NEW.checkout_id) THEN
   RAISE EXCEPTION 'bank-transfer ledger mismatch' USING ERRCODE='42501'; END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inventory.guard_bank_transfer_ledger() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.guard_bank_transfer_ledger() FROM PUBLIC;
CREATE TRIGGER zz_bank_transfer_ledger_guard BEFORE INSERT ON inventory.ledger
 FOR EACH ROW EXECUTE FUNCTION inventory.guard_bank_transfer_ledger();

-- ---------------------------------------------------------------------------------------------------
-- E. Expiry: checkout.expire_held (0013:551) also releases an AWAITING_TRANSFER hold. Same worker, same job kind, same ledger RELEASE row
-- (SYSTEM_EXPIRY); a CONFIRMED order is STALE exactly like a paid card order. The transfer row ends EXPIRED.
-- ---------------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION checkout.expire_held(p_order uuid,p_generation bigint)
 RETURNS TABLE(disposition text,retry_at timestamptz)
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_identity record; v_order checkout.orders%ROWTYPE; v_reservation inventory.reservations%ROWTYPE;
 v_line record; v_now timestamptz;
BEGIN
 IF p_order IS NULL OR p_generation IS NULL OR p_generation<1 THEN
  RAISE EXCEPTION 'invalid expiry input' USING ERRCODE='PT400'; END IF;
 SELECT o.tenant_id,o.store_id,o.owner_id,o.creator_session_id INTO v_identity
  FROM checkout.orders o WHERE o.id=p_order;
 IF NOT FOUND THEN RETURN QUERY SELECT 'STALE'::text,NULL::timestamptz; RETURN; END IF;
 PERFORM set_config('app.tenant_id',v_identity.tenant_id::text,true);
 PERFORM set_config('app.store_id',v_identity.store_id::text,true);
 PERFORM set_config('app.buyer_id',v_identity.owner_id::text,true);
 PERFORM set_config('app.buyer_session_id',v_identity.creator_session_id::text,true);
 PERFORM set_config('app.principal_id','',true);
 SELECT o.* INTO v_order FROM checkout.orders o WHERE o.id=p_order FOR UPDATE;
 -- A transfer order is AWAITING_TRANSFER until the merchant confirms; confirm/expiry race on this row lock, the loser sees STALE.
 IF NOT FOUND OR v_order.generation<>p_generation OR v_order.commercial_state NOT IN ('DRAFT','AWAITING_TRANSFER') THEN
  RETURN QUERY SELECT 'STALE'::text,NULL::timestamptz; RETURN; END IF;
 SELECT r.* INTO v_reservation FROM inventory.reservations r
  WHERE r.tenant_id=v_order.tenant_id AND r.store_id=v_order.store_id AND r.id=p_order FOR UPDATE;
 IF NOT FOUND OR v_reservation.checkout_id<>p_order OR v_reservation.buyer_owner_id<>v_order.owner_id
    OR v_reservation.buyer_session_id<>v_order.creator_session_id
    OR v_reservation.generation<>p_generation OR v_reservation.state<>'HELD' THEN
  RETURN QUERY SELECT 'STALE'::text,NULL::timestamptz; RETURN; END IF;
 FOR v_line IN SELECT l.warehouse_id,l.sku_id,l.quantity FROM inventory.reservation_lines l
  WHERE l.tenant_id=v_order.tenant_id AND l.store_id=v_order.store_id AND l.reservation_id=p_order
  ORDER BY l.warehouse_id,l.sku_id LOOP
  PERFORM 1 FROM inventory.lock_balance(v_line.warehouse_id,v_line.sku_id);
 END LOOP;
 v_now:=clock_timestamp();
 IF v_now<v_order.expires_at OR v_now<v_reservation.expires_at THEN
  RETURN QUERY SELECT 'NOT_DUE'::text,greatest(v_order.expires_at,v_reservation.expires_at); RETURN;
 END IF;
 FOR v_line IN SELECT l.warehouse_id,l.sku_id,l.quantity FROM inventory.reservation_lines l
  WHERE l.tenant_id=v_order.tenant_id AND l.store_id=v_order.store_id AND l.reservation_id=p_order
  ORDER BY l.warehouse_id,l.sku_id LOOP
  -- §11.5: evidence = order id + reservation HELD + due expires_at (checked above) + SYSTEM_EXPIRY actor.
  INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,
   operation,command_key,reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
   VALUES(v_order.tenant_id,v_order.store_id,v_line.warehouse_id,v_line.sku_id,'RELEASE',-v_line.quantity,
    'checkout.expire',p_order::text,p_order,NULL,p_order,v_order.owner_id,v_order.creator_session_id,'SYSTEM_EXPIRY');
 END LOOP;
 UPDATE inventory.reservations SET state='EXPIRED' WHERE tenant_id=v_order.tenant_id
  AND store_id=v_order.store_id AND id=p_order AND state='HELD';
 UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED',updated_at=v_now
  WHERE id=p_order AND generation=p_generation AND commercial_state IN ('DRAFT','AWAITING_TRANSFER');
 UPDATE checkout.bank_transfers SET state='EXPIRED',updated_at=v_now WHERE tenant_id=v_order.tenant_id AND store_id=v_order.store_id
  AND order_id=p_order AND state IN ('AWAITING','SUBMITTED','REJECTED');
 INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
 VALUES(v_order.tenant_id,v_order.store_id,v_order.owner_id,p_order,v_order.creator_session_id,
  p_generation,'checkout.expired','SYSTEM_EXPIRY');
 RETURN QUERY SELECT 'EXPIRED'::text,NULL::timestamptz;
END $$;

-- ---------------------------------------------------------------------------------------------------
-- F. Shippability: a CONFIRMED bank_transfer order whose transfer fact is CONFIRMED (and not refunded offline) is shippable.
-- 0073:898 body unchanged plus the third branch; SECURITY INVOKER as before (callers: commerce_checkout_writer, commerce_auth).
-- ---------------------------------------------------------------------------------------------------
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
  -- storefront-v2 §C: the merchant confirmed the transfer (offline fact = confirmed_amount_minor = the server order total, I05).
  SELECT 1 FROM checkout.orders o
  JOIN checkout.bank_transfers t ON t.tenant_id=o.tenant_id AND t.store_id=o.store_id AND t.owner_id=o.owner_id AND t.order_id=o.id
  WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_order
   AND o.commercial_state='CONFIRMED' AND o.payment_mode='bank_transfer' AND t.state='CONFIRMED'
   AND t.confirmed_amount_minor=o.total_minor
   AND NOT EXISTS(SELECT 1 FROM checkout.payment_attempts pa WHERE pa.tenant_id=o.tenant_id
    AND pa.store_id=o.store_id AND pa.order_id=o.id))
$$;

-- ---------------------------------------------------------------------------------------------------
-- G. Merchant settings (integration:read / integration:manage like cvs settings; 0073:1651 pattern).
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.read_bank_transfer_settings(p_hash bytea,p_store uuid) RETURNS jsonb
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
 SELECT c.* INTO r FROM checkout.bank_transfer_settings c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'integration:read');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF r.tenant_id IS NULL THEN
  RETURN jsonb_build_object('version',0,'enabled',false,'allow_cvs',false,'bank_name','','branch','','account_name','',
   'account_number','','window_hours',72);
 END IF;
 RETURN jsonb_build_object('version',r.version,'enabled',r.enabled,'allow_cvs',r.allow_cvs,'bank_name',r.bank_name,'branch',r.branch,
  'account_name',r.account_name,'account_number',r.account_number,'window_hours',r.window_hours);
END $$;
ALTER FUNCTION payments.read_bank_transfer_settings(bytea,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION payments.read_bank_transfer_settings(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.read_bank_transfer_settings(bytea,uuid) TO commerce_runtime;

CREATE FUNCTION payments.set_bank_transfer_settings(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,p_expected_version bigint,
 p_enabled boolean,p_allow_cvs boolean,p_bank_name text,p_branch text,p_account_name text,p_account_number text,p_window_hours integer)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; cur record; v_saved bytea; v_response jsonb; v_now timestamptz; v_ver bigint;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_expected_version IS NULL OR p_expected_version<0
  OR p_expected_version>=9223372036854775807 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid bank transfer settings' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 PERFORM pg_advisory_xact_lock(hashtextextended('bank_transfer.settings|'||s.tenant_id||'|'||p_store,0));
 SELECT r.request_hash,r.response INTO v_saved,v_response FROM ops.command_results r WHERE r.tenant_id=s.tenant_id
  AND r.store_id=p_store AND r.operation='checkout.bank_transfer_settings.set' AND r.idempotency_key=p_key;
 IF FOUND THEN
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  -- CHECK-equivalent validation first so a bad body is a 422 invalid_settings, never a 500 from a constraint.
  IF p_enabled IS NULL OR p_allow_cvs IS NULL OR p_bank_name IS NULL OR p_branch IS NULL OR p_account_name IS NULL
   OR p_account_number IS NULL OR p_window_hours IS NULL OR p_window_hours NOT BETWEEN 6 AND 168
   OR char_length(btrim(p_bank_name))>60 OR char_length(btrim(p_branch))>60 OR char_length(btrim(p_account_name))>60
   OR concat(p_bank_name,p_branch,p_account_name) ~ '[[:cntrl:]]'
   OR btrim(p_account_number) !~ '^([0-9][0-9 -]{3,31})?$'
   OR (p_enabled AND (btrim(p_bank_name)='' OR btrim(p_account_name)='' OR btrim(p_account_number)='')) THEN
   RAISE EXCEPTION 'invalid_settings' USING ERRCODE='PT422'; END IF;
  SELECT c.version INTO cur FROM checkout.bank_transfer_settings c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store FOR UPDATE;
  v_now:=clock_timestamp();
  IF NOT FOUND THEN
   IF p_expected_version<>0 THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
   v_ver:=1;
   INSERT INTO checkout.bank_transfer_settings(tenant_id,store_id,enabled,allow_cvs,bank_name,branch,account_name,account_number,window_hours,version,updated_at)
   VALUES(s.tenant_id,p_store,p_enabled,p_allow_cvs,btrim(p_bank_name),btrim(p_branch),btrim(p_account_name),btrim(p_account_number),p_window_hours,1,v_now);
  ELSE
   IF cur.version<>p_expected_version THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
   v_ver:=cur.version+1;
   UPDATE checkout.bank_transfer_settings SET enabled=p_enabled,allow_cvs=p_allow_cvs,bank_name=btrim(p_bank_name),branch=btrim(p_branch),
    account_name=btrim(p_account_name),account_number=btrim(p_account_number),window_hours=p_window_hours,version=v_ver,updated_at=v_now
    WHERE tenant_id=s.tenant_id AND store_id=p_store;
  END IF;
  -- A change never touches placed orders (each keeps its own snapshot); only future begin_hold calls read this row.
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
   VALUES(s.tenant_id,p_store,s.principal_id,'checkout.bank_transfer_settings_changed');
  v_response:=jsonb_build_object('version',v_ver,'enabled',p_enabled,'allow_cvs',p_allow_cvs,'bank_name',btrim(p_bank_name),'branch',btrim(p_branch),
   'account_name',btrim(p_account_name),'account_number',btrim(p_account_number),'window_hours',p_window_hours);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(s.tenant_id,p_store,'checkout.bank_transfer_settings.set',p_key,p_request_hash,v_response,s.principal_id);
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF v_final.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION payments.set_bank_transfer_settings(bytea,uuid,text,bytea,bigint,boolean,boolean,text,text,text,text,integer) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION payments.set_bank_transfer_settings(bytea,uuid,text,bytea,bigint,boolean,boolean,text,text,text,text,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.set_bank_transfer_settings(bytea,uuid,text,bytea,bigint,boolean,boolean,text,text,text,text,integer) TO commerce_runtime;

-- Buyer options: is the mode on, may a CVS destination use it, how long is the window (no bank details here; read_cvs_offer pattern).
CREATE FUNCTION checkout.read_transfer_offer(p_hash bytea,p_store uuid)
RETURNS TABLE(enabled boolean,allow_cvs boolean,window_hours integer)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_scope record; c record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL THEN RAISE EXCEPTION 'invalid offer read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO v_scope FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 -- The settings policy reads the caller's GUCs; a missing GUC would silently read "no row = off", so fail closed.
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM v_scope.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text THEN
  RAISE EXCEPTION 'offer scope' USING ERRCODE='PT403'; END IF;
 SELECT s.* INTO c FROM checkout.bank_transfer_settings s WHERE s.tenant_id=v_scope.tenant_id AND s.store_id=p_store;
 RETURN QUERY SELECT coalesce(c.enabled,false),coalesce(c.allow_cvs,false),coalesce(c.window_hours,72);
END $$;
ALTER FUNCTION checkout.read_transfer_offer(bytea,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.read_transfer_offer(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.read_transfer_offer(bytea,uuid) TO commerce_checkout_runtime;

-- ---------------------------------------------------------------------------------------------------
-- H. Buyer: attach the optional email, submit the proof, read the order's transfer view.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION checkout.set_order_buyer_email(p_hash bytea,p_store uuid,p_order uuid,p_email text) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_rows integer;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_email IS NULL
  OR char_length(p_email) NOT BETWEEN 3 AND 254 OR p_email !~ '^[^[:space:][:cntrl:]@]+@[^[:space:][:cntrl:]@]+$' THEN
  RAISE EXCEPTION 'invalid buyer email' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.buyer_id',s.owner_id::text,true),set_config('app.buyer_session_id',s.session_id::text,true),set_config('app.principal_id','',true);
 -- Set once, only by the creating session within the placing transaction's minute; never overwritten.
 UPDATE checkout.orders SET buyer_email=p_email WHERE tenant_id=s.tenant_id AND store_id=p_store AND owner_id=s.owner_id AND id=p_order
  AND creator_session_id=s.session_id AND buyer_email IS NULL AND created_at>=clock_timestamp()-interval '1 minute';
 GET DIAGNOSTICS v_rows=ROW_COUNT;
 IF v_rows<>1 THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
END $$;
ALTER FUNCTION checkout.set_order_buyer_email(bytea,uuid,uuid,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.set_order_buyer_email(bytea,uuid,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.set_order_buyer_email(bytea,uuid,uuid,text) TO commerce_checkout_runtime;

CREATE FUNCTION checkout.submit_transfer_proof(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,p_order uuid,
 p_last5 text,p_amount_minor bigint,p_paid_at timestamptz) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; o record; t record; v_saved record; v_now timestamptz; v_result jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_last5 IS NULL OR p_last5 !~ '^[0-9]{5}$'
  OR p_amount_minor IS NULL OR p_amount_minor NOT BETWEEN 1 AND 1000000000000 OR p_paid_at IS NULL OR NOT isfinite(p_paid_at) THEN
  RAISE EXCEPTION 'invalid transfer proof' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.buyer_id',s.owner_id::text,true),set_config('app.buyer_session_id',s.session_id::text,true),set_config('app.principal_id','',true);
 PERFORM pg_advisory_xact_lock(hashtextextended('checkout.transfer.submit|'||s.tenant_id||'|'||p_store||'|'||s.owner_id||'|'||p_key,0));
 SELECT x.request_hash,x.response INTO v_saved FROM checkout.command_results x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
  AND x.owner_id=s.owner_id AND x.operation='checkout.transfer.submit' AND x.idempotency_key=p_key;
 IF FOUND THEN
  IF v_saved.request_hash<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
  RETURN v_saved.response;
 END IF;
 SELECT x.generation,x.payment_mode,x.commercial_state,x.expires_at,x.created_at INTO o FROM checkout.orders x
  WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.owner_id=s.owner_id AND x.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 IF o.payment_mode<>'bank_transfer' THEN RAISE EXCEPTION 'not_bank_transfer' USING ERRCODE='PT422'; END IF;
 SELECT x.state,x.proof_count INTO t FROM checkout.bank_transfers x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.order_id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_bank_transfer' USING ERRCODE='PT422'; END IF;
 v_now:=clock_timestamp();
 -- Editable until confirmed: AWAITING/SUBMITTED/REJECTED may (re)submit while the window is open; CONFIRMED/EXPIRED/REFUNDED never.
 IF o.commercial_state<>'AWAITING_TRANSFER' OR t.state NOT IN ('AWAITING','SUBMITTED','REJECTED') THEN
  RAISE EXCEPTION 'transfer_not_open' USING ERRCODE='PT422'; END IF;
 IF v_now>=o.expires_at THEN RAISE EXCEPTION 'transfer_window_closed' USING ERRCODE='PT422'; END IF;
 IF p_paid_at>v_now+interval '1 hour' OR p_paid_at<o.created_at-interval '1 day' THEN RAISE EXCEPTION 'invalid_proof' USING ERRCODE='PT422'; END IF;
 UPDATE checkout.bank_transfers SET state='SUBMITTED',proof_last5=p_last5,proof_amount_minor=p_amount_minor,proof_paid_at=p_paid_at,
  proof_submitted_at=v_now,proof_count=t.proof_count+1,reject_reason=NULL,rejected_at=NULL,updated_at=v_now
  WHERE tenant_id=s.tenant_id AND store_id=p_store AND order_id=p_order;
 INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
 VALUES(s.tenant_id,p_store,s.owner_id,p_order,s.session_id,o.generation,'checkout.transfer_proof_submitted','BUYER');
 v_result:=jsonb_build_object('order_id',p_order,'state','SUBMITTED','proof_count',t.proof_count+1,
  'submitted_at',to_char(v_now AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
 INSERT INTO checkout.command_results(tenant_id,store_id,owner_id,operation,idempotency_key,creator_session_id,request_hash,order_id,response)
 VALUES(s.tenant_id,p_store,s.owner_id,'checkout.transfer.submit',p_key,s.session_id,p_request_hash,p_order,v_result);
 RETURN v_result;
END $$;
ALTER FUNCTION checkout.submit_transfer_proof(bytea,uuid,text,bytea,uuid,text,bigint,timestamptz) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.submit_transfer_proof(bytea,uuid,text,bytea,uuid,text,bigint,timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.submit_transfer_proof(bytea,uuid,text,bytea,uuid,text,bigint,timestamptz) TO commerce_checkout_runtime;

-- The one projection of a transfer row. p_buyer hides the bank details of an EXPIRED order; merchants always see the snapshot.
CREATE FUNCTION checkout.bank_transfer_json(p_tenant uuid,p_store uuid,p_order uuid,p_buyer boolean) RETURNS jsonb
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT jsonb_build_object('order_id',t.order_id,'state',t.state,'window_hours',t.window_hours,
  'deadline_at',to_char(o.expires_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
  'currency',o.currency,'amount_minor',o.total_minor,
  'bank',CASE WHEN p_buyer AND t.state='EXPIRED' THEN NULL ELSE jsonb_build_object('bank_name',t.bank_name,'branch',t.branch,
    'account_name',t.account_name,'account_number',t.account_number) END,
  'proof',CASE WHEN t.proof_last5 IS NULL THEN NULL ELSE jsonb_build_object('last5',t.proof_last5,'amount_minor',t.proof_amount_minor,
    'paid_at',to_char(t.proof_paid_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
    'submitted_at',to_char(t.proof_submitted_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'count',t.proof_count) END,
  'reject_reason',t.reject_reason,
  'confirmed_at',to_char(t.confirmed_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
  'refunded_at',to_char(t.refunded_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'))
 FROM checkout.bank_transfers t JOIN checkout.orders o ON o.tenant_id=t.tenant_id AND o.store_id=t.store_id AND o.owner_id=t.owner_id AND o.id=t.order_id
 WHERE t.tenant_id=p_tenant AND t.store_id=p_store AND t.order_id=p_order
$$;
ALTER FUNCTION checkout.bank_transfer_json(uuid,uuid,uuid,boolean) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.bank_transfer_json(uuid,uuid,uuid,boolean) FROM PUBLIC;

CREATE FUNCTION checkout.read_bank_transfer_buyer(p_hash bytea,p_store uuid,p_order uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_json jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL THEN RAISE EXCEPTION 'invalid transfer read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.buyer_id',s.owner_id::text,true),set_config('app.buyer_session_id',s.session_id::text,true),set_config('app.principal_id','',true);
 -- Owner scope is part of the lookup: another buyer's order id is "not found", never a bank detail.
 IF NOT EXISTS(SELECT 1 FROM checkout.bank_transfers t WHERE t.tenant_id=s.tenant_id AND t.store_id=p_store AND t.owner_id=s.owner_id AND t.order_id=p_order) THEN
  RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 v_json:=checkout.bank_transfer_json(s.tenant_id,p_store,p_order,true);
 RETURN v_json;
END $$;
ALTER FUNCTION checkout.read_bank_transfer_buyer(bytea,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.read_bank_transfer_buyer(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.read_bank_transfer_buyer(bytea,uuid,uuid) TO commerce_checkout_runtime;

-- ---------------------------------------------------------------------------------------------------
-- I. Merchant: read, confirm, reject, refund offline. Permission payments:refund (owners hold it since 0065) for the three writes,
-- orders:read for the read; resolve_access -> GUCs -> order lock -> buyer GUCs from the locked order -> replay -> rules -> write -> audit
-- -> receipt -> fresh final authorization (release_pay_at_pickup, 0073:1778, pattern).
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.read_bank_transfer_merchant(p_hash bytea,p_store uuid,p_order uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_json jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid transfer read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 v_json:=checkout.bank_transfer_json(s.tenant_id,p_store,p_order,false);
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF v_json IS NULL THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 RETURN v_json;
END $$;
ALTER FUNCTION payments.read_bank_transfer_merchant(bytea,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION payments.read_bank_transfer_merchant(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.read_bank_transfer_merchant(bytea,uuid,uuid) TO commerce_runtime;

-- p_action: 'confirm' | 'reject' | 'refund_offline'. One definer so the three acts share the authorization, the lock order, the receipt and
-- the audit; p_reason is the reject reason (NULL otherwise).
CREATE FUNCTION payments.decide_bank_transfer(p_hash bytea,p_store uuid,p_order uuid,p_key text,p_request_hash bytea,p_action text,p_reason text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; o record; t record; r record; l record; v_saved bytea; v_response jsonb; v_now timestamptz;
 v_lines integer:=0; v_op text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_action IS NULL OR p_action NOT IN ('confirm','reject','refund_offline')
  OR (p_action='reject')<>(p_reason IS NOT NULL)
  OR (p_action='reject' AND (char_length(btrim(p_reason)) NOT BETWEEN 1 AND 200 OR p_reason ~ '[[:cntrl:]]'))
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid transfer decision' USING ERRCODE='PT400'; END IF;
 v_op:='checkout.bank_transfer.'||p_action;
 -- Authorize before any lock (0063 A2); tenant/store/principal GUCs come from this result.
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'payments:refund');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT k.owner_id,k.creator_session_id,k.payment_mode,k.commercial_state,k.total_minor,k.currency,k.expires_at INTO o
  FROM checkout.orders k WHERE k.tenant_id=s.tenant_id AND k.store_id=p_store AND k.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 -- The ledger guard compares the buyer columns with these GUCs (0073:1796): they come from the locked order row.
 PERFORM set_config('app.buyer_id',o.owner_id::text,true),set_config('app.buyer_session_id',o.creator_session_id::text,true);
 SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c WHERE c.tenant_id=s.tenant_id
  AND c.store_id=p_store AND c.operation=v_op AND c.idempotency_key=p_key;
 IF FOUND THEN
  -- Idempotent replay: the same key and body returns the saved answer; another body under the key is a conflict.
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  IF o.payment_mode<>'bank_transfer' THEN RAISE EXCEPTION 'not_bank_transfer' USING ERRCODE='PT422'; END IF;
  SELECT x.state,x.confirmed_at INTO t FROM checkout.bank_transfers x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.order_id=p_order FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'not_bank_transfer' USING ERRCODE='PT422'; END IF;
  v_now:=clock_timestamp();
  IF p_action='refund_offline' THEN
   -- Offline refund: the merchant returned the money outside the platform (no PSP call); only a CONFIRMED transfer, once.
   IF t.state='REFUNDED_OFFLINE' THEN RAISE EXCEPTION 'already_refunded' USING ERRCODE='PT409'; END IF;
   IF t.state<>'CONFIRMED' OR o.commercial_state<>'CONFIRMED' THEN RAISE EXCEPTION 'transfer_not_confirmed' USING ERRCODE='PT422'; END IF;
   UPDATE checkout.bank_transfers SET state='REFUNDED_OFFLINE',refunded_at=v_now,refunded_by=s.principal_id,updated_at=v_now
    WHERE tenant_id=s.tenant_id AND store_id=p_store AND order_id=p_order;
   v_response:=jsonb_build_object('order_id',p_order,'state','REFUNDED_OFFLINE','commercial_state',o.commercial_state,'released_lines',0);
  ELSE
   IF t.state IN ('CONFIRMED','REFUNDED_OFFLINE') THEN RAISE EXCEPTION 'already_confirmed' USING ERRCODE='PT409'; END IF;
   IF t.state='EXPIRED' OR o.commercial_state<>'AWAITING_TRANSFER' OR t.state NOT IN ('AWAITING','SUBMITTED','REJECTED') THEN
    RAISE EXCEPTION 'transfer_not_open' USING ERRCODE='PT409'; END IF;
   IF p_action='reject' THEN
    -- Rejects the buyer's submission, never the order: the buyer may submit again until the window ends; expiry is the only cancel.
    IF t.state<>'SUBMITTED' THEN RAISE EXCEPTION 'transfer_not_submitted' USING ERRCODE='PT409'; END IF;
    UPDATE checkout.bank_transfers SET state='REJECTED',reject_reason=btrim(p_reason),rejected_at=v_now,updated_at=v_now
     WHERE tenant_id=s.tenant_id AND store_id=p_store AND order_id=p_order;
    v_response:=jsonb_build_object('order_id',p_order,'state','REJECTED','commercial_state',o.commercial_state,'released_lines',0);
   ELSE
    -- Confirm: only inside the window (after it the expiry may already be releasing the stock; it wins the order row lock).
    IF v_now>=o.expires_at THEN RAISE EXCEPTION 'transfer_window_closed' USING ERRCODE='PT409'; END IF;
    SELECT x.state,x.checkout_id,x.buyer_owner_id,x.buyer_session_id INTO r FROM inventory.reservations x
     WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
    IF NOT FOUND OR r.state<>'HELD' OR r.checkout_id<>p_order OR r.buyer_owner_id<>o.owner_id OR r.buyer_session_id<>o.creator_session_id THEN
     RAISE EXCEPTION 'transfer_not_open' USING ERRCODE='PT409'; END IF;
    UPDATE checkout.orders SET commercial_state='CONFIRMED',updated_at=v_now
     WHERE tenant_id=s.tenant_id AND store_id=p_store AND owner_id=o.owner_id AND id=p_order AND commercial_state='AWAITING_TRANSFER';
    UPDATE checkout.bank_transfers SET state='CONFIRMED',confirmed_at=v_now,confirmed_by=s.principal_id,confirmed_amount_minor=o.total_minor,updated_at=v_now
     WHERE tenant_id=s.tenant_id AND store_id=p_store AND order_id=p_order;
    UPDATE inventory.reservations SET state='COMMITTED' WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_order AND state='HELD';
    -- §11.5 HELD -> COMMITTED: evidence = order id + transfer row CONFIRMED + merchant principal on every ALLOCATE row
    -- (guard inventory.guard_bank_transfer_ledger); balances locked in the global (warehouse, sku) order first.
    FOR l IN SELECT x.warehouse_id,x.sku_id,x.quantity FROM inventory.reservation_lines x
     WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.reservation_id=p_order ORDER BY x.warehouse_id,x.sku_id LOOP
     PERFORM 1 FROM inventory.lock_balance(l.warehouse_id,l.sku_id);
     INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,operation,command_key,reservation_id,
      principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
     VALUES(s.tenant_id,p_store,l.warehouse_id,l.sku_id,'ALLOCATE',-l.quantity,l.quantity,'checkout.bank_transfer.confirm',p_order::text,p_order,
      s.principal_id,p_order,o.owner_id,o.creator_session_id,'MERCHANT');
     v_lines:=v_lines+1;
    END LOOP;
    IF v_lines=0 THEN RAISE EXCEPTION 'transfer_not_open' USING ERRCODE='PT409'; END IF;
    v_response:=jsonb_build_object('order_id',p_order,'state','CONFIRMED','commercial_state','CONFIRMED','released_lines',v_lines);
   END IF;
  END IF;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
   VALUES(s.tenant_id,p_store,s.principal_id,CASE p_action WHEN 'confirm' THEN 'checkout.bank_transfer_confirmed'
    WHEN 'reject' THEN 'checkout.bank_transfer_rejected' ELSE 'checkout.bank_transfer_refunded_offline' END);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(s.tenant_id,p_store,v_op,p_key,p_request_hash,v_response,s.principal_id);
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'payments:refund');
 IF v_final.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text) TO commerce_runtime;

-- ---------------------------------------------------------------------------------------------------
-- J. Merchant orders list filter (0073:1964) accepts AWAITING_TRANSFER; customers privacy (0078): the open transfer hold blocks erasure like
-- any unexpired hold, the erasure clears buyer_email, the buyer export carries it. Patched in place (verified by shape) instead of re-copying
-- three long definers; each patch fails loudly when the live definition is not the expected one.
-- ---------------------------------------------------------------------------------------------------
DO $$
DECLARE v_item record; v_fn text;
BEGIN
 FOR v_item IN SELECT * FROM (VALUES
  ('identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text)',
   '''AWAITING_PAYMENT'',''CONFIRMED'',''CANCELLED'',''shipped'',''unshipped'',''cvs_pending''',
   '''AWAITING_PAYMENT'',''AWAITING_TRANSFER'',''CONFIRMED'',''CANCELLED'',''shipped'',''unshipped'',''cvs_pending''',1),
  ('identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text)',
   'p_state IN (''DRAFT'',''AWAITING_PAYMENT'',''CONFIRMED'',''CANCELLED'')',
   'p_state IN (''DRAFT'',''AWAITING_PAYMENT'',''AWAITING_TRANSFER'',''CONFIRMED'',''CANCELLED'')',1),
  ('customers.erase_owner(bytea,uuid,uuid,text,uuid)',
   'o.commercial_state IN (''DRAFT'',''AWAITING_PAYMENT'')','o.commercial_state IN (''DRAFT'',''AWAITING_PAYMENT'',''AWAITING_TRANSFER'')',1),
  ('customers.buyer_export_orders(bytea,uuid)',
   '''currency'',o.currency,''total_minor'',o.total_minor,''snapshot'',o.snapshot-''allocation'',',
   '''currency'',o.currency,''total_minor'',o.total_minor,''payment_mode'',o.payment_mode,''buyer_email'',o.buyer_email,''snapshot'',o.snapshot-''allocation'',',1),
  ('customers.apply_erasure(uuid,uuid,uuid)',
   ' GET DIAGNOSTICS v_snapshots=ROW_COUNT;',
   E' GET DIAGNOSTICS v_snapshots=ROW_COUNT;\n -- storefront-v2 §C: the optional buyer email on this owner''s orders (the summary keys stay unchanged).\n PERFORM checkout.clear_buyer_email(p_tenant,p_store,p_owner);',1)
 ) AS t(fn,needle,repl,expect) LOOP
  v_fn:=pg_get_functiondef(v_item.fn::regprocedure);
  IF (length(v_fn)-length(replace(v_fn,v_item.needle,'')))<>length(v_item.needle)*v_item.expect THEN
   RAISE EXCEPTION '% has an unexpected shape for patch %',v_item.fn,left(v_item.needle,60); END IF;
  EXECUTE replace(v_fn,v_item.needle,v_item.repl);
 END LOOP;
END $$;

-- ---------------------------------------------------------------------------------------------------
-- K. Finance (0078:822): the BD7 projection gains four keys. pay-at-pickup collected (ops-polish OP3, orders COLLECTED on the day their
-- row last changed; environment = latest ECPay attempt, else LIVE) and bank-transfer confirmed (confirmed_at day; offline money is
-- always environment LIVE; an offline refund leaves the filter exactly like REFUNDED_OFFLINE leaves the pick-up one). Never part of
-- captured/net: different money paths (the carrier / the merchant's bank, not a PSP).
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
  -- I05: the order total of a COLLECTED pay_at_pickup order; no collected_at column exists, so updated_at (set by the collecting
  -- transition, 0073:1462/1738) is the day while the order stays COLLECTED.
  SELECT (o.updated_at AT TIME ZONE 'Asia/Taipei')::date AS day,o.currency,
   coalesce((SELECT c.environment FROM fulfillment.cvs_shipments c WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id
     AND c.order_id=o.id ORDER BY c.attempt DESC LIMIT 1),'LIVE') AS environment,
   count(*)::bigint AS n,sum(o.total_minor)::bigint AS minor
  FROM checkout.orders o,lim
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.payment_mode='pay_at_pickup' AND o.collection_state='COLLECTED'
   AND o.updated_at>=lim.t0 AND o.updated_at<lim.t1
  GROUP BY 1,2,3
 ), trf AS (
  -- I05: confirmed_amount_minor is the server order total at confirmation (never a client amount).
  SELECT (t.confirmed_at AT TIME ZONE 'Asia/Taipei')::date AS day,t.currency,'LIVE'::text AS environment,
   count(*)::bigint AS n,sum(t.confirmed_amount_minor)::bigint AS minor
  FROM checkout.bank_transfers t,lim
  WHERE t.tenant_id=s.tenant_id AND t.store_id=p_store AND t.state='CONFIRMED'
   AND t.confirmed_at>=lim.t0 AND t.confirmed_at<lim.t1
  GROUP BY 1,2,3
 ), keys AS (
  SELECT day,currency,environment FROM cap UNION SELECT day,currency,environment FROM ref
  UNION SELECT day,currency,environment FROM pick UNION SELECT day,currency,environment FROM trf
 )
 SELECT coalesce(jsonb_agg(jsonb_build_object('day',to_char(k.day,'YYYY-MM-DD'),'currency',k.currency,
   'environment',k.environment,'captured_count',coalesce(c.n,0),'captured_minor',coalesce(c.minor,0),
   'refunded_minor',coalesce(r.minor,0),'net_minor',coalesce(c.minor,0)-coalesce(r.minor,0),
   'pickup_collected_count',coalesce(p.n,0),'pickup_collected_minor',coalesce(p.minor,0),
   'bank_transfer_confirmed_count',coalesce(t.n,0),'bank_transfer_confirmed_minor',coalesce(t.minor,0))
   ORDER BY k.day,k.currency,k.environment),'[]'::jsonb) INTO v_rows
 FROM keys k
 LEFT JOIN cap c ON c.day=k.day AND c.currency=k.currency AND c.environment=k.environment
 LEFT JOIN ref r ON r.day=k.day AND r.currency=k.currency AND r.environment=k.environment
 LEFT JOIN pick p ON p.day=k.day AND p.currency=k.currency AND p.environment=k.environment
 LEFT JOIN trf t ON t.day=k.day AND t.currency=k.currency AND t.environment=k.environment;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'finance read access denied' USING ERRCODE=v_auth_error; END IF;
 IF octet_length(v_rows::text)>1048576 THEN RAISE EXCEPTION 'finance read unavailable' USING ERRCODE='PT503'; END IF;
 RETURN v_rows;
END $$;

-- ---------------------------------------------------------------------------------------------------
-- Documentation (PROCESS §5): owning package, allowed roles, non-goals.
-- ---------------------------------------------------------------------------------------------------
COMMENT ON TABLE checkout.bank_transfer_settings IS 'internal/checkout (offline payment): per-store bank-transfer switch, bank details and window hours (6..168). Written only by payments.set_bank_transfer_settings (integration:manage), read by begin_hold, read_transfer_offer and read_bank_transfer_settings. No grant to runtime or buyer roles. Non-goal: no PSP data; a missing row means off.';
COMMENT ON TABLE checkout.bank_transfers IS 'internal/checkout (offline payment): one row per bank_transfer order: bank-details snapshot, the buyer''s latest proof, the merchant''s reject reason and the offline money fact (confirmed_* = server order total at confirmation). Written only by begin_hold, submit_transfer_proof, decide_bank_transfer and expire_held (owner commerce_checkout_writer); commerce_auth reads the finance/shippability columns. Non-goal: never auto-confirmed, never a payments.facts row.';
COMMENT ON COLUMN checkout.bank_transfers.state IS 'AWAITING (no proof yet) -> SUBMITTED <-> REJECTED (the buyer may re-submit) -> CONFIRMED (merchant, payments:refund) -> REFUNDED_OFFLINE; AWAITING/SUBMITTED/REJECTED -> EXPIRED by checkout.expire_held. REJECTED rejects the submission, not the order.';
COMMENT ON COLUMN checkout.bank_transfers.confirmed_amount_minor IS 'I05: the server order total (checkout.orders.total_minor) at confirmation; never taken from the buyer''s claimed amount.';
COMMENT ON COLUMN checkout.bank_transfers.account_number IS 'Merchant bank account snapshot; shown to the buyer only through checkout.read_bank_transfer_buyer (owner-scoped) and to the merchant through read_bank_transfer_merchant.';
COMMENT ON COLUMN checkout.bank_transfer_settings.window_hours IS 'Hours a bank-transfer hold keeps stock RESERVED (6..168, default 72); begin_hold sets orders.expires_at = placement + window.';
COMMENT ON FUNCTION payments.read_bank_transfer_settings(bytea,uuid) IS 'internal/merchantorders only; EXECUTE commerce_runtime. integration:read, GUCs from resolve_access, fresh final fence. No row = off at version 0.';
COMMENT ON FUNCTION payments.set_bank_transfer_settings(bytea,uuid,text,bytea,bigint,boolean,boolean,text,text,text,text,integer) IS 'internal/merchantorders only; EXECUTE commerce_runtime. integration:manage, version CAS (0 inserts), idempotent receipt in ops.command_results, one audit row checkout.bank_transfer_settings_changed. Never touches placed orders.';
COMMENT ON FUNCTION checkout.read_transfer_offer(bytea,uuid) IS 'internal/checkout options only; EXECUTE commerce_checkout_runtime. Buyer-scope read of enabled/allow_cvs/window_hours; never bank details.';
COMMENT ON FUNCTION checkout.set_order_buyer_email(bytea,uuid,uuid,text) IS 'internal/checkout Begin only; EXECUTE commerce_checkout_runtime. Sets orders.buyer_email once, by the creating session, within a minute of placement.';
COMMENT ON FUNCTION checkout.submit_transfer_proof(bytea,uuid,text,bytea,uuid,text,bigint,timestamptz) IS 'internal/checkout only; EXECUTE commerce_checkout_runtime. The buyer''s own AWAITING_TRANSFER order, inside the window; idempotent receipt in checkout.command_results; editable until the merchant confirms. Coded PT422: not_bank_transfer, transfer_not_open, transfer_window_closed, invalid_proof.';
COMMENT ON FUNCTION checkout.bank_transfer_json(uuid,uuid,uuid,boolean) IS 'Internal projection helper of checkout.read_bank_transfer_buyer/merchant; EXECUTE nobody (called by the two definers, same owner). Non-goal: no authorization of its own.';
COMMENT ON FUNCTION checkout.read_bank_transfer_buyer(bytea,uuid,uuid) IS 'internal/checkout only; EXECUTE commerce_checkout_runtime. The buyer''s own order only (another owner''s id is 404); bank details hidden once EXPIRED.';
COMMENT ON FUNCTION payments.read_bank_transfer_merchant(bytea,uuid,uuid) IS 'internal/merchantorders only; EXECUTE commerce_runtime. orders:read, fresh final fence.';
COMMENT ON FUNCTION payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text) IS 'internal/merchantorders only; EXECUTE commerce_runtime. The ONLY writer of a transfer confirmation: payments:refund, order row lock, idempotent receipt (ops.command_results), one audit row per act (checkout.bank_transfer_confirmed|rejected|refunded_offline). confirm = order CONFIRMED + reservation COMMITTED + MERCHANT ALLOCATE ledger rows; reject = submission REJECTED with a reason; refund_offline = REFUNDED_OFFLINE, no PSP, no stock row.';
COMMENT ON FUNCTION inventory.guard_bank_transfer_ledger() IS 'inventory trigger guard (BEFORE INSERT on ledger): the only MERCHANT ALLOCATE is the confirm of a CONFIRMED bank_transfer order for exactly the reserved quantity (storefront-v2 §C). DEFINER (commerce_checkout_writer).';
COMMENT ON FUNCTION checkout.expire_held(uuid,bigint) IS 'internal/checkout ExpiryWorker only; EXECUTE commerce_worker. Releases a due DRAFT or AWAITING_TRANSFER hold (SYSTEM_EXPIRY RELEASE rows) and marks the transfer row EXPIRED; every other state is STALE.';
COMMENT ON FUNCTION identity.read_finance_summary(bytea,uuid,date,date) IS 'internal/reporting only; EXECUTE commerce_runtime. orders:read; daily captured/refunded/net by currency and environment plus the pay-at-pickup collected and bank-transfer confirmed columns (never part of captured/net).';
