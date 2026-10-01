-- 0094 merchant tools (contracts/storefront-v2.md section G, R4 unit merchant-tools): the two read-only dashboard blocks that need
-- privileged tables, and the persisted order source of a merchant-created (manual) order.
--
-- Owns: identity.dashboard_orders, identity.dashboard_todos (owner commerce_auth, EXECUTE commerce_runtime), the column
-- checkout.orders.source, and fulfillment.mark_order_merchant_manual (owner commerce_checkout_writer, EXECUTE commerce_runtime). The marker
-- lives in schema fulfillment, not checkout: commerce_runtime has no USAGE on schema checkout (every merchant-callable checkout-writer definer
-- lives in payments, fulfillment or identity), and this unit must not widen that.
-- Owning package: internal/merchanttools (dashboard.go, manual.go).
--
-- Non-goals: no new table, no new index (merchant_orders_history 0027, orders_unshipped 0063 and bank_transfers_open 0088 cover the
-- reads), no new permission (orders:read for the dashboard, inventory:reserve for the manual order), no change to begin_hold, to the
-- merchant order projection or to the finance definer (the dashboard reuses read_finance_summary through internal/reporting, so there is
-- no second ledger), no stock or payment write.
--
-- Depends on: 0027 (commerce_auth projection policies), 0063 (manual_shipment_eligible, merchant_access_denied), 0072/0073
-- (cvs_shipments, payment_mode), 0062 (stripe_refunds, refund_facts), 0088 (bank_transfers, commerce_auth grants on them).
-- No new RLS policy: every table the dashboard definers read is already readable by commerce_auth through the USING(true) projection
-- policies named above; the scope fence is the explicit tenant_id/store_id predicate, exactly like read_finance_summary.

DO $$
BEGIN
 IF to_regprocedure('identity.merchant_access_denied(bytea,uuid,text[],uuid,uuid,bigint)') IS NULL
  OR to_regprocedure('fulfillment.manual_shipment_eligible(uuid,uuid,uuid)') IS NULL
  OR to_regclass('checkout.bank_transfers') IS NULL THEN
  RAISE EXCEPTION '0094 requires 0063 (merchant_access_denied, manual_shipment_eligible) and 0088 (bank_transfers)';
 END IF;
END $$;

-- ---------------------------------------------------------------------------------------------------
-- A. Dashboard block 1: placed-order counts of the Asia/Taipei day and of the last seven days (today and the six before).
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION identity.dashboard_orders(p_hash bytea,p_store uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_today date; v_t0 timestamptz; v_t7 timestamptz; v_t1 timestamptz; v_out jsonb; v_auth_error text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid dashboard read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 v_today:=(clock_timestamp() AT TIME ZONE 'Asia/Taipei')::date;
 v_t0:=v_today::timestamp AT TIME ZONE 'Asia/Taipei';
 v_t7:=(v_today-6)::timestamp AT TIME ZONE 'Asia/Taipei';
 v_t1:=(v_today+1)::timestamp AT TIME ZONE 'Asia/Taipei';
 -- A placed order is one that left the 15-minute DRAFT hold and was not cancelled; created_at is the placement time.
 SELECT jsonb_build_object('today',count(*) FILTER (WHERE o.created_at>=v_t0),'last_7_days',count(*)) INTO v_out
 FROM checkout.orders o
 WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.created_at>=v_t7 AND o.created_at<v_t1
  AND o.commercial_state IN ('AWAITING_PAYMENT','AWAITING_TRANSFER','CONFIRMED');
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'dashboard read access denied' USING ERRCODE=v_auth_error; END IF;
 RETURN v_out;
END $$;
ALTER FUNCTION identity.dashboard_orders(bytea,uuid) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.dashboard_orders(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.dashboard_orders(bytea,uuid) TO commerce_runtime;
COMMENT ON FUNCTION identity.dashboard_orders(bytea,uuid) IS
 'internal/merchanttools.Dashboard only; EXECUTE commerce_runtime. orders:read; {today,last_7_days} counts of orders created in the Asia/Taipei day / last seven days in AWAITING_PAYMENT, AWAITING_TRANSFER or CONFIRMED. Read-only; non-goals: no money (finance owns that), no DRAFT or CANCELLED order.';

-- ---------------------------------------------------------------------------------------------------
-- B. Dashboard block 2: the four to-do counters that need privileged tables (low stock is a plain scoped read in Go).
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION identity.dashboard_todos(p_hash bytea,p_store uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_out jsonb; v_auth_error text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid dashboard read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 -- The unshipped candidates are computed ONCE (the manual_shipment_eligible call is the expensive part): to_ship counts them, cvs_awaiting_label
 -- counts the CVS ones without a live ECPay attempt (FAILED/ABANDONED attempts do not count).
 WITH unshipped AS MATERIALIZED (
  SELECT o.tenant_id,o.store_id,o.id,o.snapshot#>>'{destination,kind}' AS kind FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.commercial_state='CONFIRMED' AND o.fulfillment_state='MANUAL_UNASSIGNED'
   -- The merchant order list's `unshipped` predicate (0073 read_merchant_orders), so the counter equals that list.
   AND fulfillment.manual_shipment_eligible(o.tenant_id,o.store_id,o.id))
 SELECT jsonb_build_object(
  -- 0088 bank_transfers_open: the buyer submitted proof and the merchant has not decided.
  'awaiting_transfer_confirmation',(SELECT count(*) FROM checkout.bank_transfers t
     WHERE t.tenant_id=s.tenant_id AND t.store_id=p_store AND t.state='SUBMITTED'),
  'to_ship',(SELECT count(*) FROM unshipped),
  'cvs_awaiting_label',(SELECT count(*) FROM unshipped u WHERE u.kind LIKE 'cvs\_%'
     AND NOT EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=u.tenant_id AND c.store_id=u.store_id
       AND c.order_id=u.id AND c.state NOT IN ('FAILED','ABANDONED'))),
  -- A Stripe refund with no terminal fact yet (SUCCEEDED, FAILED, CANCELED or REJECTED).
  'open_refunds',(SELECT count(*) FROM payments.stripe_refunds r WHERE r.tenant_id=s.tenant_id AND r.store_id=p_store
     AND NOT EXISTS(SELECT 1 FROM payments.refund_facts rf WHERE rf.tenant_id=r.tenant_id AND rf.store_id=r.store_id
       AND rf.refund_id=r.id))) INTO v_out;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'dashboard read access denied' USING ERRCODE=v_auth_error; END IF;
 RETURN v_out;
END $$;
ALTER FUNCTION identity.dashboard_todos(bytea,uuid) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.dashboard_todos(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.dashboard_todos(bytea,uuid) TO commerce_runtime;
COMMENT ON FUNCTION identity.dashboard_todos(bytea,uuid) IS
 'internal/merchanttools.Dashboard only; EXECUTE commerce_runtime. orders:read; counters awaiting_transfer_confirmation, to_ship (the unshipped list predicate), cvs_awaiting_label (CVS subset of to_ship without a live ECPay attempt), open_refunds (Stripe refund without a terminal fact). Read-only; non-goals: no row detail, no low-stock (a scoped read in Go).';

-- ---------------------------------------------------------------------------------------------------
-- C. Order source (attribution of merchant-created orders).
-- ---------------------------------------------------------------------------------------------------
ALTER TABLE checkout.orders ADD COLUMN source text NOT NULL DEFAULT 'storefront' CHECK(source IN ('storefront','merchant_manual'));
COMMENT ON COLUMN checkout.orders.source IS 'internal/merchanttools: where the order was created. storefront (default, every buyer-placed order) or merchant_manual (admin Create Order, set by fulfillment.mark_order_merchant_manual right after the buyer begin path placed it). Attribution only: it never changes pricing, stock or payment.';
GRANT UPDATE(source) ON checkout.orders TO commerce_checkout_writer;

-- ---------------------------------------------------------------------------------------------------
-- D. The buyer link of a manual order: a single-use, <= 7 day token stored ONLY as sha256, exchanged by the storefront BFF for a FULL buyer
-- capability of the order's owner (the owner a manual order creates is that order's alone, so "scoped to exactly that order" holds).
-- ---------------------------------------------------------------------------------------------------
CREATE TABLE checkout.order_links (
 token_hash bytea PRIMARY KEY CHECK(octet_length(token_hash)=32),
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,order_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 expires_at timestamptz NOT NULL,
 redeemed_at timestamptz,
 CHECK(expires_at>created_at AND expires_at<=created_at+interval '7 days'),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id)
);
CREATE INDEX order_links_order ON checkout.order_links(tenant_id,store_id,order_id);
ALTER TABLE checkout.order_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkout.order_links FORCE ROW LEVEL SECURITY;
REVOKE ALL ON checkout.order_links FROM PUBLIC;
GRANT SELECT,INSERT ON checkout.order_links TO commerce_checkout_writer;
GRANT UPDATE(redeemed_at) ON checkout.order_links TO commerce_checkout_writer;
-- The redeem definer looks a row up by the secret hash alone (no tenant/store GUC exists yet), so the writer's policy is open; every definer
-- below filters by store and order itself.
CREATE POLICY order_links_writer ON checkout.order_links TO commerce_checkout_writer USING(true) WITH CHECK(true);
COMMENT ON TABLE checkout.order_links IS 'internal/merchanttools (creates a row through fulfillment.mark_order_merchant_manual) and internal/buyerhttp orderlink.go (redeems through checkout.redeem_order_link): one single-use link token per manual order, stored only as sha256, expiring within 7 days. Roles: commerce_checkout_writer (owner of both definers) only. Non-goals: no plaintext token, no list or read route, no renewal.';

-- Same body as buyer.issue_order_capability (0090) but the session is a FULL capability of the owner (view_order_id NULL): a manual-order buyer
-- must be able to pay (bank-transfer proof, pay-at-pickup views), which a view-only guest-lookup session may not.
CREATE FUNCTION buyer.issue_owner_capability(p_store uuid, p_owner uuid, p_hash bytea, p_ttl bigint) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store_active boolean; v_tenant_active boolean; v_owner_active boolean; v_session uuid; v_created timestamptz;
BEGIN
 IF p_store IS NULL OR p_owner IS NULL OR p_hash IS NULL OR octet_length(p_hash)<>32 OR p_ttl IS NULL OR p_ttl<60 OR p_ttl>2592000 THEN
  RAISE EXCEPTION 'invalid buyer capability request' USING ERRCODE='PT400'; END IF;
 SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=p_store;
 SELECT t.active INTO v_tenant_active FROM control.tenants t WHERE t.id=v_tenant FOR SHARE OF t;
 SELECT s.active INTO v_store_active FROM control.stores s WHERE (s.tenant_id,s.id)=(v_tenant,p_store) FOR SHARE OF s;
 SELECT o.active INTO v_owner_active FROM buyer.owners o WHERE (o.tenant_id,o.store_id,o.id)=(v_tenant,p_store,p_owner) FOR SHARE OF o;
 IF v_tenant_active IS NOT TRUE OR v_store_active IS NOT TRUE OR v_owner_active IS NOT TRUE THEN
  RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 v_created:=clock_timestamp();
 INSERT INTO buyer.capability_sessions(tenant_id,store_id,owner_id,token_hash,created_at,expires_at,view_order_id)
  VALUES(v_tenant,p_store,p_owner,p_hash,v_created,v_created+make_interval(secs=>p_ttl),NULL) RETURNING id INTO v_session;
 INSERT INTO buyer.capability_events(tenant_id,store_id,owner_id,session_id,action) VALUES(v_tenant,p_store,p_owner,v_session,'capability.issued');
 RETURN v_session;
END $$;
ALTER FUNCTION buyer.issue_owner_capability(uuid,uuid,bytea,bigint) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.issue_owner_capability(uuid,uuid,bytea,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.issue_owner_capability(uuid,uuid,bytea,bigint) TO commerce_checkout_writer;
COMMENT ON FUNCTION buyer.issue_owner_capability(uuid,uuid,bytea,bigint) IS 'checkout.redeem_order_link only (EXECUTE commerce_checkout_writer): registers a FULL (not view-only) capability session for an EXISTING active buyer owner with the BFF-minted token hash; PT401 for an inactive owner, store or tenant. Non-goals: no new owner, no admission limit (the redeem throttles first).';

-- Exchange: throttle (counted before any read), find the link by (store, order, hash), refuse a used or expired one, mark it used and issue the
-- capability. EVERY refusal is the same empty answer (unknown order, wrong token, other store, used, expired, inactive owner): the caller cannot
-- tell them apart. p_token is sha256 of the BFF-minted capability token, p_link is sha256 of the link token.
CREATE FUNCTION checkout.redeem_order_link(p_store uuid, p_order uuid, p_link bytea, p_token bytea, p_ttl bigint, p_ip bytea)
RETURNS TABLE(order_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v record;
BEGIN
 IF p_store IS NULL OR p_order IS NULL OR p_link IS NULL OR octet_length(p_link)<>32 OR p_token IS NULL OR octet_length(p_token)<>32
  OR p_ttl IS NULL OR p_ttl<60 OR p_ttl>2592000 OR (p_ip IS NOT NULL AND octet_length(p_ip)<>32)
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid order link' USING ERRCODE='PT400'; END IF;
 PERFORM checkout.lookup_hit('olip:'||coalesce(encode(p_ip,'hex'),'none'),10);
 PERFORM checkout.lookup_hit('olord:'||p_store::text||':'||p_order::text,5);
 PERFORM checkout.lookup_hit('olstore:'||p_store::text,200);
 SELECT l.owner_id,l.order_id,l.redeemed_at,l.expires_at INTO v FROM checkout.order_links l
  WHERE l.token_hash=p_link AND l.store_id=p_store AND l.order_id=p_order FOR UPDATE;
 IF NOT FOUND OR v.redeemed_at IS NOT NULL OR v.expires_at<=clock_timestamp() THEN RETURN; END IF;
 BEGIN
  PERFORM buyer.issue_owner_capability(p_store,v.owner_id,p_token,p_ttl);
 EXCEPTION WHEN SQLSTATE 'PT401' THEN
  RETURN; -- inactive / erased owner, store or tenant: the same empty answer as a mismatch
 END;
 UPDATE checkout.order_links SET redeemed_at=clock_timestamp() WHERE token_hash=p_link;
 order_id:=v.order_id;
 RETURN NEXT;
END $$;
ALTER FUNCTION checkout.redeem_order_link(uuid,uuid,bytea,bytea,bigint,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.redeem_order_link(uuid,uuid,bytea,bytea,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.redeem_order_link(uuid,uuid,bytea,bytea,bigint,bytea) TO commerce_buyer_issuer;
COMMENT ON FUNCTION checkout.redeem_order_link(uuid,uuid,bytea,bytea,bigint,bytea) IS 'internal/buyerhttp orderlink.go (POST /v1/buyer/orders/link); EXECUTE commerce_buyer_issuer. Throttles (ip 10, order 5, store 200 per 10 min, the guest-lookup bucket table), then exchanges a single-use <= 7 day link token for a FULL buyer capability of the order''s owner. Returns the order id, or no row for every kind of refusal. Non-goals: no plaintext token, no view-only session, no renewal.';

-- The merchant tx of POST orders/manual calls this after checkout.Service.Begin committed the order under the server-held buyer
-- capability. The bearer is re-verified with inventory:reserve (the permission that guards a merchant stock hold), the order must be
-- in the caller's store, still `storefront` and at most 10 minutes old, so it cannot relabel an old buyer order. It also records the buyer
-- link (sha256 of the token, 7 days); a repeat is a no-op (same order, same derived token hash).
CREATE FUNCTION fulfillment.mark_order_merchant_manual(p_hash bytea,p_store uuid,p_order uuid,p_link bytea) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_source text; v_created timestamptz; v_owner uuid; v_now timestamptz:=clock_timestamp();
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_link IS NULL OR octet_length(p_link)<>32
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid manual order mark' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'inventory:reserve');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT o.source,o.created_at,o.owner_id INTO v_source,v_created,v_owner FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 IF v_source<>'merchant_manual' THEN
  IF v_created<clock_timestamp()-interval '10 minutes' THEN RAISE EXCEPTION 'order not recent' USING ERRCODE='PT409'; END IF;
  UPDATE checkout.orders SET source='merchant_manual' WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_order;
 END IF;
 INSERT INTO checkout.order_links(token_hash,tenant_id,store_id,owner_id,order_id,created_at,expires_at)
  VALUES(p_link,s.tenant_id,p_store,v_owner,p_order,v_now,v_now+interval '7 days') ON CONFLICT(token_hash) DO NOTHING;
END $$;
ALTER FUNCTION fulfillment.mark_order_merchant_manual(bytea,uuid,uuid,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.mark_order_merchant_manual(bytea,uuid,uuid,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.mark_order_merchant_manual(bytea,uuid,uuid,bytea) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.mark_order_merchant_manual(bytea,uuid,uuid,bytea) IS
 'internal/merchanttools.PlaceManual only (merchant transaction, commerce_runtime). inventory:reserve via resolve_access; sets checkout.orders.source=merchant_manual on an order of the caller''s store created within the last 10 minutes and still storefront, and records the order''s buyer link (sha256 of the token, expires in 7 days); a repeat is a no-op. Non-goals: no price, stock, state or payment change, no relabel of an older order, no plaintext token.';

-- ---------------------------------------------------------------------------------------------------
-- E. Merchant orders projection: the source of each order of a page (orders:read), so the Orders page can badge merchant-created orders.
-- ---------------------------------------------------------------------------------------------------
GRANT SELECT(source) ON checkout.orders TO commerce_auth;
CREATE FUNCTION identity.read_order_sources(p_hash bytea,p_store uuid,p_ids uuid[]) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_out jsonb; v_auth_error text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_ids IS NULL OR cardinality(p_ids) NOT BETWEEN 1 AND 101
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid source read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 SELECT coalesce(jsonb_object_agg(o.id::text,o.source),'{}'::jsonb) INTO v_out FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=ANY(p_ids);
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'source read access denied' USING ERRCODE=v_auth_error; END IF;
 RETURN v_out;
END $$;
ALTER FUNCTION identity.read_order_sources(bytea,uuid,uuid[]) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_order_sources(bytea,uuid,uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_order_sources(bytea,uuid,uuid[]) TO commerce_runtime;
COMMENT ON FUNCTION identity.read_order_sources(bytea,uuid,uuid[]) IS
 'internal/merchantorders List/Get only; EXECUTE commerce_runtime. orders:read; {order_id: source} for up to 101 order ids of the caller''s store (storefront | merchant_manual). Read-only; non-goals: no other column, no order outside the store (silently absent). Added beside identity.read_merchant_orders instead of rewriting that definer.';
