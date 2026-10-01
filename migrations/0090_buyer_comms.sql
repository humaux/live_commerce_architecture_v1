-- 0090 buyer communications (contracts/storefront-v2.md §E, R4 unit buyer-comms; brief docs/delivery/units/buyer-comms.md).
--
-- Owns: (1) the notify.outbox transactional mail outbox (one row per (order, kind), filled by AFTER triggers on the order, shipment and
-- refund tables IN the transaction that moves the order, drained by internal/notify), its merchant opt-out table and the claim / record
-- definers with the daily and per-store caps; (2) the guest order lookup definer checkout.guest_order_lookup (+ its fixed-window throttle)
-- and buyer.issue_order_capability, which registers a capability session for an EXISTING buyer owner; (3) the merchant toggle definers.
--
-- Non-goals: no SMTP, no template and no message body anywhere in SQL (internal/notify renders; the log keeps kind, order, state, counts and a
-- recipient hash only); no recipient address is stored (it is read from checkout.orders.buyer_email at claim time, so erasure that NULLs that
-- column also ends delivery); no River job (post_river queue guards belong to other units, the worker polls this table); no change to any order,
-- payment or stock transition (the triggers only INSERT into notify.outbox and swallow their own failure with a WARNING: a mail problem must never
-- roll back a payment or a stock move); no login-code bucket (identity.auth_throttle is a separate ledger).
--
-- Owner role: commerce_checkout_writer (no new role). It already owns or reads every table the definers touch (orders, bank_transfers, stores,
-- storefront domains, shipments, refunds, destination snapshots); this file adds only column-level reads of identity.store_staff and
-- identity.memberships plus EXECUTE of identity.staff_password_email (0089 precedent: that role already reads identity.sessions) so merchant
-- mail can find the owner address without opening password_credentials (gate PA03 keeps that table to commerce_identity_writer).
-- Callers: internal/notify (commerce_worker: claim_batch, record_result; commerce_runtime: store settings), internal/buyerhttp
-- (commerce_buyer_issuer: guest_order_lookup). Depends on: 0013 (checkout.orders), 0062 (payments.stripe_refunds/refund_facts), 0063 (manual
-- shipments), 0072/0073 (cvs_shipments, collection_state), 0088 (buyer_email, bank_transfers), 0089 (identity.store_staff, staff_password_email).

CREATE SCHEMA notify AUTHORIZATION commerce_checkout_writer;
REVOKE ALL ON SCHEMA notify FROM PUBLIC;
GRANT USAGE ON SCHEMA notify TO commerce_worker, commerce_runtime;
-- guest_order_lookup lives in schema checkout (it reads orders); the issuer pool only gets USAGE plus EXECUTE of that one function.
GRANT USAGE ON SCHEMA checkout TO commerce_buyer_issuer;

GRANT SELECT (tenant_id, store_id, principal_id, role) ON identity.store_staff TO commerce_checkout_writer;
GRANT SELECT (tenant_id, principal_id, active) ON identity.memberships TO commerce_checkout_writer;
GRANT EXECUTE ON FUNCTION identity.staff_password_email(uuid) TO commerce_checkout_writer;
GRANT EXECUTE ON FUNCTION identity.resolve_access(bytea, uuid, text) TO commerce_checkout_writer;

-- ---------------------------------------------------------------------------------------------------
-- A. Tables
-- ---------------------------------------------------------------------------------------------------
CREATE TABLE notify.outbox (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, order_id uuid NOT NULL,
 kind text NOT NULL CHECK(kind IN ('placed','paid','shipped','cancelled','refunded','merchant_new')),
 state text NOT NULL DEFAULT 'PENDING' CHECK(state IN ('PENDING','SENDING','SENT','UNKNOWN','FAILED','SKIPPED')),
 attempts smallint NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 3),
 next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 batch_id uuid, claimed_at timestamptz, sent_at timestamptz,
 recipient_hash bytea CHECK(recipient_hash IS NULL OR octet_length(recipient_hash)=32),
 skip_reason text CHECK(skip_reason IN ('no_recipient','opted_out','stale','no_owner','no_order','erased')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(order_id, kind),
 CHECK((state='SKIPPED')=(skip_reason IS NOT NULL)),
 CHECK(state NOT IN ('SENDING','SENT','UNKNOWN') OR (batch_id IS NOT NULL AND claimed_at IS NOT NULL)),
 CHECK((state='SENT')=(sent_at IS NOT NULL))
);
CREATE INDEX outbox_pending ON notify.outbox(next_attempt_at, created_at) WHERE state='PENDING';
CREATE INDEX outbox_store_sent ON notify.outbox(store_id, claimed_at) WHERE state IN ('SENDING','SENT','UNKNOWN');
CREATE INDEX outbox_batch ON notify.outbox(batch_id) WHERE batch_id IS NOT NULL;

CREATE TABLE notify.store_settings (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 merchant_new_order_email boolean NOT NULL DEFAULT true,
 updated_at timestamptz NOT NULL, updated_by uuid NOT NULL,
 PRIMARY KEY(tenant_id, store_id),
 FOREIGN KEY(tenant_id, store_id) REFERENCES control.stores(tenant_id, id)
);

-- Fixed 10-minute windows, one row per (hashed bucket, window); same shape as identity.auth_throttle (0070) but a separate ledger.
CREATE TABLE checkout.lookup_throttle (
 bucket bytea NOT NULL CHECK(octet_length(bucket)=32), window_start timestamptz NOT NULL, hits integer NOT NULL CHECK(hits>0),
 PRIMARY KEY(bucket, window_start)
);

DO $$
DECLARE t text;
BEGIN
 FOREACH t IN ARRAY ARRAY['notify.outbox','notify.store_settings','checkout.lookup_throttle'] LOOP
  EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('CREATE POLICY owner_only ON %s TO commerce_checkout_writer USING(true) WITH CHECK(true)',t);
 END LOOP;
END $$;
GRANT SELECT, INSERT, UPDATE, DELETE ON notify.outbox, notify.store_settings, checkout.lookup_throttle TO commerce_checkout_writer;

-- ---------------------------------------------------------------------------------------------------
-- B. Enqueue (internal) and the triggers
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION notify.enqueue(p_tenant uuid, p_store uuid, p_order uuid, p_kind text) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 BEGIN
  INSERT INTO notify.outbox(tenant_id,store_id,order_id,kind) VALUES(p_tenant,p_store,p_order,p_kind) ON CONFLICT DO NOTHING;
 EXCEPTION WHEN OTHERS THEN
  -- A mail problem never rolls back the order/payment transaction that fired the trigger (the mail is lost, the money is not).
  RAISE WARNING 'notify.enqueue failed (sqlstate %)',SQLSTATE;
 END;
END $$;

CREATE FUNCTION notify.on_order_insert() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 -- begin_hold inserts bank_transfer orders as AWAITING_TRANSFER and pay_at_pickup orders as CONFIRMED (post_river/0018): that is "placed".
 IF NEW.commercial_state='AWAITING_TRANSFER' OR (NEW.commercial_state='CONFIRMED' AND NEW.payment_mode='pay_at_pickup') THEN
  PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'placed');
  PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'merchant_new');
 END IF;
 RETURN NULL;
END $$;

CREATE FUNCTION notify.on_order_update() RETURNS trigger
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
    -- a confirmed transfer was announced to the merchant at placement; a card capture is the first the merchant hears of the order
    IF NEW.payment_mode='card' AND OLD.commercial_state IN ('DRAFT','AWAITING_PAYMENT') THEN
     PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'merchant_new');
    END IF;
   END IF;
  -- abandoned DRAFT / AWAITING_PAYMENT card checkouts were never announced, so they are not "cancelled" either
  ELSIF NEW.commercial_state='CANCELLED' AND OLD.commercial_state IN ('AWAITING_TRANSFER','CONFIRMED') THEN
   PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'cancelled');
  END IF;
 END IF;
 IF NEW.fulfillment_state IS DISTINCT FROM OLD.fulfillment_state AND NEW.fulfillment_state='MERCHANT_SHIPPED' THEN
  PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'shipped');
 END IF;
 IF NEW.collection_state IS DISTINCT FROM OLD.collection_state AND NEW.collection_state='REFUNDED_OFFLINE' THEN
  PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.id,'refunded');
 END IF;
 -- Erasure (customers.apply_erasure -> checkout.clear_buyer_email) NULLs the address: end delivery and drop the recipient hash (§E7).
 IF OLD.buyer_email IS NOT NULL AND NEW.buyer_email IS NULL THEN
  UPDATE notify.outbox SET recipient_hash=NULL,updated_at=clock_timestamp(),
    state=CASE WHEN state='PENDING' THEN 'SKIPPED' ELSE state END,
    skip_reason=CASE WHEN state='PENDING' THEN 'erased' ELSE skip_reason END
   WHERE order_id=NEW.id AND kind<>'merchant_new';
 END IF;
 RETURN NULL;
END $$;

CREATE TRIGGER notify_order_insert AFTER INSERT ON checkout.orders FOR EACH ROW EXECUTE FUNCTION notify.on_order_insert();
CREATE TRIGGER notify_order_update AFTER UPDATE OF commercial_state, fulfillment_state, collection_state, buyer_email ON checkout.orders
 FOR EACH ROW EXECUTE FUNCTION notify.on_order_update();

CREATE FUNCTION notify.on_cvs_shipment() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 -- the parcel left the merchant: handed to the distribution centre (AT_DC) or already at the pickup store (AT_STORE)
 IF NEW.state IN ('AT_DC','AT_STORE') AND NEW.state IS DISTINCT FROM OLD.state THEN
  PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.order_id,'shipped');
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER notify_cvs_shipment AFTER UPDATE OF state ON fulfillment.cvs_shipments FOR EACH ROW EXECUTE FUNCTION notify.on_cvs_shipment();

CREATE FUNCTION notify.on_transfer_refund() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.state='REFUNDED_OFFLINE' AND NEW.state IS DISTINCT FROM OLD.state THEN
  PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,NEW.order_id,'refunded');
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER notify_transfer_refund AFTER UPDATE OF state ON checkout.bank_transfers FOR EACH ROW EXECUTE FUNCTION notify.on_transfer_refund();

CREATE FUNCTION notify.on_stripe_refund() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_order uuid;
BEGIN
 IF NEW.kind='SUCCEEDED' THEN
  -- the fact's own scope (the inserting definer set exactly these values, its WITH CHECK needs them): payments.stripe_refunds is GUC-scoped (0062)
  PERFORM set_config('app.tenant_id',NEW.tenant_id::text,true),set_config('app.store_id',NEW.store_id::text,true);
  -- payments.stripe_refunds: the refund row carries its order; one mail per order however many partial refunds follow (PK order, kind)
  SELECT r.order_id INTO v_order FROM payments.stripe_refunds r WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id AND r.id=NEW.refund_id;
  IF v_order IS NOT NULL THEN PERFORM notify.enqueue(NEW.tenant_id,NEW.store_id,v_order,'refunded'); END IF;
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER notify_stripe_refund AFTER INSERT ON payments.refund_facts FOR EACH ROW EXECUTE FUNCTION notify.on_stripe_refund();

-- ---------------------------------------------------------------------------------------------------
-- C. Claim and record (commerce_worker). The returned jsonb is the whole input of the renderer: nothing else is read by Go.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION notify.claim_batch(p_limit integer, p_daily_cap integer, p_store_hourly integer) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_now timestamptz:=clock_timestamp(); v_day timestamptz; v_used integer; v_h integer; v_out jsonb:='[]'::jsonb; v_batch uuid;
 c record; o record; s record; v_store_name text; v_origin text; v_bank jsonb; v_ship jsonb; v_pick jsonb; v_cvs jsonb; v_emails text[]; v_orders uuid[]; v_count integer;
 v_locale text;
BEGIN
 IF p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 50 OR p_daily_cap IS NULL OR p_daily_cap<0 OR p_store_hourly IS NULL OR p_store_hourly<1
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid claim' USING ERRCODE='PT400'; END IF;
 -- UTC+8 day start (same alignment as the login-code caps, identity.auth_throttle_hit)
 v_day:=to_timestamp((floor((extract(epoch FROM v_now)+28800)/86400)*86400-28800)::double precision);
 -- A worker that died between SENDING and the record may or may not have sent: UNKNOWN, never re-sent (I06).
 UPDATE notify.outbox SET state='UNKNOWN',updated_at=v_now WHERE state='SENDING' AND claimed_at<v_now-interval '15 minutes';
 UPDATE notify.outbox SET state='SKIPPED',skip_reason='stale',updated_at=v_now WHERE state='PENDING' AND created_at<v_now-interval '24 hours';
 DELETE FROM notify.outbox WHERE (order_id,kind) IN (SELECT x.order_id,x.kind FROM notify.outbox x WHERE x.state<>'PENDING' AND x.created_at<v_now-interval '180 days' ORDER BY x.created_at LIMIT 200);
 -- §E3: every notify mail of the day (a merchant batch counts once) shares one budget of 60% of the daily cap.
 SELECT count(DISTINCT coalesce(x.batch_id::text,x.order_id::text||x.kind)) INTO v_used FROM notify.outbox x
  WHERE x.state IN ('SENDING','SENT','UNKNOWN') AND x.claimed_at>=v_day;

 FOR c IN SELECT * FROM notify.outbox x WHERE x.state='PENDING' AND x.kind<>'merchant_new' AND x.next_attempt_at<=v_now
   ORDER BY x.created_at LIMIT p_limit FOR UPDATE SKIP LOCKED LOOP
  EXIT WHEN v_used>=p_daily_cap;
  SELECT count(*) INTO v_h FROM notify.outbox x WHERE x.store_id=c.store_id AND x.kind<>'merchant_new'
   AND x.state IN ('SENDING','SENT','UNKNOWN') AND x.claimed_at>=v_now-interval '1 hour';
  CONTINUE WHEN v_h>=p_store_hourly;
  SELECT k.owner_id,k.buyer_email,k.payment_mode,k.commercial_state,k.total_minor,k.currency,k.expires_at,k.destination_id,k.snapshot->>'locale' AS locale
   INTO o FROM checkout.orders k WHERE k.id=c.order_id AND k.tenant_id=c.tenant_id AND k.store_id=c.store_id;
  IF NOT FOUND THEN
   UPDATE notify.outbox SET state='SKIPPED',skip_reason='no_order',updated_at=v_now WHERE order_id=c.order_id AND kind=c.kind; CONTINUE; END IF;
  IF o.buyer_email IS NULL THEN
   UPDATE notify.outbox SET state='SKIPPED',skip_reason='no_recipient',updated_at=v_now WHERE order_id=c.order_id AND kind=c.kind; CONTINUE; END IF;
  -- commerce_checkout_writer reads stores, domains, bank transfers, pickups and destination snapshots only inside a scope (0013 / 0073 / 0088
  -- policies are keyed on these GUCs, exactly as every checkout definer sets them): scope this row before reading them.
  PERFORM set_config('app.tenant_id',c.tenant_id::text,true),set_config('app.store_id',c.store_id::text,true),set_config('app.buyer_id',o.owner_id::text,true);
  SELECT s2.name INTO v_store_name FROM control.stores s2 WHERE s2.tenant_id=c.tenant_id AND s2.id=c.store_id;
  -- link origin: the store's ACTIVE published storefront domain (control.storefront_domains); none = a mail without a link
  SELECT d.origin INTO v_origin FROM control.storefront_domains d JOIN control.storefront_publications p ON p.tenant_id=d.tenant_id AND p.store_id=d.store_id
   WHERE d.tenant_id=c.tenant_id AND d.store_id=c.store_id AND d.state='ACTIVE' AND d.valid_until>v_now AND p.published ORDER BY d.id LIMIT 1;
  v_bank:=NULL; v_ship:=NULL; v_pick:=NULL; v_cvs:=NULL;
  IF c.kind='placed' AND o.payment_mode='bank_transfer' THEN
   -- the order's own snapshot (a later settings change never moves an open order's account)
   SELECT jsonb_build_object('bank_name',t.bank_name,'branch',t.branch,'account_name',t.account_name,'account_number',t.account_number)
    INTO v_bank FROM checkout.bank_transfers t WHERE t.tenant_id=c.tenant_id AND t.store_id=c.store_id AND t.order_id=c.order_id;
  END IF;
  IF c.kind IN ('placed','shipped') THEN
   -- CVS pickup store of the order's destination snapshot (home delivery has none)
   SELECT jsonb_build_object('name',pv.name,'address',pv.address) INTO v_pick
    FROM storefront.destination_snapshots ds JOIN fulfillment.pickup_versions pv ON pv.tenant_id=ds.tenant_id AND pv.store_id=ds.store_id AND pv.id=ds.pickup_id
    WHERE ds.tenant_id=c.tenant_id AND ds.store_id=c.store_id AND ds.owner_id=o.owner_id AND ds.id=o.destination_id;
  END IF;
  IF c.kind='shipped' THEN
   SELECT jsonb_build_object('carrier_code',v.carrier_code,'carrier_name',v.carrier_name,'tracking_number',v.tracking_number,'tracking_url',v.tracking_url)
    INTO v_ship FROM fulfillment.manual_shipment_heads h JOIN fulfillment.manual_shipment_versions v
     ON v.tenant_id=h.tenant_id AND v.store_id=h.store_id AND v.order_id=h.order_id AND v.version=h.current_version AND v.status='SHIPPED'
    WHERE h.tenant_id=c.tenant_id AND h.store_id=c.store_id AND h.order_id=c.order_id;
   SELECT jsonb_build_object('shipping_no',coalesce(z.cvs_payment_no,z.shipment_no)) INTO v_cvs FROM fulfillment.cvs_shipments z
    WHERE z.tenant_id=c.tenant_id AND z.store_id=c.store_id AND z.order_id=c.order_id AND z.state IN ('AT_DC','AT_STORE','PICKED_UP') ORDER BY z.attempt DESC LIMIT 1;
  END IF;
  v_locale:=CASE WHEN o.locale IN ('zh-TW','zh-CN','en') THEN o.locale END;
  v_batch:=gen_random_uuid();
  UPDATE notify.outbox SET state='SENDING',attempts=attempts+1,claimed_at=v_now,batch_id=v_batch,updated_at=v_now WHERE order_id=c.order_id AND kind=c.kind;
  v_used:=v_used+1;
  v_out:=v_out||jsonb_build_array(jsonb_build_object('batch_id',v_batch,'kind',c.kind,'order_id',c.order_id,'to',jsonb_build_array(o.buyer_email),
   'store_name',v_store_name,'locale',v_locale,'origin',v_origin,'total_minor',o.total_minor,'currency',o.currency,'payment_mode',o.payment_mode,
   'expires_at',o.expires_at,'bank',v_bank,'pickup',v_pick,'shipment',v_ship,'cvs',v_cvs));
 END LOOP;

 -- Merchant new-order mail: one batch per store, at most one per 5 minutes, only for stores that did not opt out (§E6).
 FOR s IN SELECT DISTINCT x.tenant_id,x.store_id FROM notify.outbox x WHERE x.kind='merchant_new' AND x.state='PENDING' AND x.next_attempt_at<=v_now LOOP
  EXIT WHEN v_used>=p_daily_cap;
  IF EXISTS(SELECT 1 FROM notify.store_settings z WHERE z.tenant_id=s.tenant_id AND z.store_id=s.store_id AND NOT z.merchant_new_order_email) THEN
   UPDATE notify.outbox SET state='SKIPPED',skip_reason='opted_out',updated_at=v_now WHERE store_id=s.store_id AND kind='merchant_new' AND state='PENDING';
   CONTINUE;
  END IF;
  CONTINUE WHEN EXISTS(SELECT 1 FROM notify.outbox x WHERE x.store_id=s.store_id AND x.kind='merchant_new' AND x.state IN ('SENDING','SENT','UNKNOWN')
   AND x.claimed_at>v_now-interval '5 minutes');
  -- the store's owners: store_staff role owner, active membership, verified password email (identity definer; no direct credentials read)
  SELECT array_agg(DISTINCT e) INTO v_emails FROM (SELECT identity.staff_password_email(f.principal_id) AS e FROM identity.store_staff f
   JOIN identity.memberships m ON m.tenant_id=f.tenant_id AND m.principal_id=f.principal_id AND m.active
   WHERE f.tenant_id=s.tenant_id AND f.store_id=s.store_id AND f.role='owner') q WHERE e IS NOT NULL;
  IF v_emails IS NULL THEN
   UPDATE notify.outbox SET state='SKIPPED',skip_reason='no_owner',updated_at=v_now WHERE store_id=s.store_id AND kind='merchant_new' AND state='PENDING';
   CONTINUE;
  END IF;
  PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',s.store_id::text,true);
  SELECT array_agg(q.order_id), count(*) INTO v_orders, v_count FROM (SELECT x.order_id FROM notify.outbox x WHERE x.store_id=s.store_id AND x.kind='merchant_new'
   AND x.state='PENDING' AND x.next_attempt_at<=v_now ORDER BY x.created_at LIMIT 50 FOR UPDATE SKIP LOCKED) q;
  CONTINUE WHEN v_count=0;
  v_batch:=gen_random_uuid();
  UPDATE notify.outbox SET state='SENDING',attempts=attempts+1,claimed_at=v_now,batch_id=v_batch,updated_at=v_now
   WHERE kind='merchant_new' AND order_id=ANY(v_orders);
  v_used:=v_used+1;
  SELECT s2.name INTO v_store_name FROM control.stores s2 WHERE s2.tenant_id=s.tenant_id AND s2.id=s.store_id;
  v_out:=v_out||jsonb_build_array(jsonb_build_object('batch_id',v_batch,'kind','merchant_new','store_name',v_store_name,'to',to_jsonb(v_emails),
   'count',v_count,'order_ids',to_jsonb(v_orders[1:10])));
 END LOOP;
 RETURN v_out;
END $$;

CREATE FUNCTION notify.record_result(p_batch uuid, p_state text, p_recipient_hash bytea) RETURNS integer
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_n integer; v_now timestamptz:=clock_timestamp();
BEGIN
 IF p_batch IS NULL OR p_state IS NULL OR p_state NOT IN ('SENT','FAILED','UNKNOWN') OR (p_recipient_hash IS NOT NULL AND octet_length(p_recipient_hash)<>32) THEN
  RAISE EXCEPTION 'invalid record' USING ERRCODE='PT400'; END IF;
 -- Only from SENDING: a replayed record changes nothing. FAILED = the server certainly did not accept (mail.ErrFailed): retry 2 min, then 10 min,
 -- FAILED for good after the third attempt. UNKNOWN is final: SMTP has no idempotency key, a resend could deliver twice.
 UPDATE notify.outbox x SET updated_at=v_now,
   state=CASE p_state WHEN 'SENT' THEN 'SENT' WHEN 'UNKNOWN' THEN 'UNKNOWN' WHEN 'FAILED' THEN CASE WHEN x.attempts>=3 THEN 'FAILED' ELSE 'PENDING' END END,
   sent_at=CASE WHEN p_state='SENT' THEN v_now END,
   next_attempt_at=CASE WHEN p_state='FAILED' AND x.attempts<3 THEN v_now+CASE WHEN x.attempts=1 THEN interval '2 minutes' ELSE interval '10 minutes' END ELSE x.next_attempt_at END,
   -- an erasure that landed during the send must not leave a recipient hash behind
   recipient_hash=CASE WHEN p_recipient_hash IS NOT NULL AND (x.kind='merchant_new'
     OR EXISTS(SELECT 1 FROM checkout.orders k WHERE k.id=x.order_id AND k.buyer_email IS NOT NULL)) THEN p_recipient_hash ELSE x.recipient_hash END
  WHERE x.batch_id=p_batch AND x.state='SENDING';
 GET DIAGNOSTICS v_n=ROW_COUNT;
 RETURN v_n;
END $$;

-- ---------------------------------------------------------------------------------------------------
-- D. Merchant toggle (integration:read / integration:manage, the 0088 settings pattern). A PUT is a plain idempotent set.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION notify.read_store_settings(p_hash bytea, p_store uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; r record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid settings read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 SELECT z.merchant_new_order_email INTO r FROM notify.store_settings z WHERE z.tenant_id=s.tenant_id AND z.store_id=p_store;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'integration:read');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 -- no row = the default, on
 RETURN jsonb_build_object('merchant_new_order_email',coalesce(r.merchant_new_order_email,true));
END $$;

CREATE FUNCTION notify.set_store_settings(p_hash bytea, p_store uuid, p_enabled boolean) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_enabled IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid settings write' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 INSERT INTO notify.store_settings(tenant_id,store_id,merchant_new_order_email,updated_at,updated_by)
  VALUES(s.tenant_id,p_store,p_enabled,clock_timestamp(),s.principal_id)
  ON CONFLICT(tenant_id,store_id) DO UPDATE SET merchant_new_order_email=EXCLUDED.merchant_new_order_email,updated_at=EXCLUDED.updated_at,updated_by=EXCLUDED.updated_by;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN jsonb_build_object('merchant_new_order_email',p_enabled);
END $$;

-- ---------------------------------------------------------------------------------------------------
-- E. Guest order lookup (§E5): throttle first, then one index range scan, one hash compare, then the capability for the EXISTING owner.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION buyer.issue_order_capability(p_store uuid, p_owner uuid, p_hash bytea, p_ttl bigint) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store_active boolean; v_tenant_active boolean; v_owner_active boolean; v_session uuid; v_created timestamptz;
BEGIN
 IF p_store IS NULL OR p_owner IS NULL OR p_hash IS NULL OR octet_length(p_hash)<>32 OR p_ttl IS NULL OR p_ttl<60 OR p_ttl>2592000 THEN
  RAISE EXCEPTION 'invalid buyer capability request' USING ERRCODE='PT400'; END IF;
 SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=p_store;
 -- same lock order as issue_capability / resolve_scope: tenant -> store -> owner
 SELECT t.active INTO v_tenant_active FROM control.tenants t WHERE t.id=v_tenant FOR SHARE OF t;
 SELECT s.active INTO v_store_active FROM control.stores s WHERE (s.tenant_id,s.id)=(v_tenant,p_store) FOR SHARE OF s;
 SELECT o.active INTO v_owner_active FROM buyer.owners o WHERE (o.tenant_id,o.store_id,o.id)=(v_tenant,p_store,p_owner) FOR SHARE OF o;
 IF v_tenant_active IS NOT TRUE OR v_store_active IS NOT TRUE OR v_owner_active IS NOT TRUE THEN
  RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 v_created:=clock_timestamp();
 INSERT INTO buyer.capability_sessions(tenant_id,store_id,owner_id,token_hash,created_at,expires_at)
  VALUES(v_tenant,p_store,p_owner,p_hash,v_created,v_created+make_interval(secs=>p_ttl)) RETURNING id INTO v_session;
 INSERT INTO buyer.capability_events(tenant_id,store_id,owner_id,session_id,action) VALUES(v_tenant,p_store,p_owner,v_session,'capability.issued');
 RETURN v_session;
END $$;
ALTER FUNCTION buyer.issue_order_capability(uuid,uuid,bytea,bigint) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.issue_order_capability(uuid,uuid,bytea,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.issue_order_capability(uuid,uuid,bytea,bigint) TO commerce_checkout_writer;

CREATE FUNCTION checkout.lookup_hit(p_key text, p_limit integer) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_now timestamptz:=clock_timestamp(); v_start timestamptz; v_hits integer;
BEGIN
 v_start:=to_timestamp((floor(extract(epoch FROM v_now)/600)*600)::double precision);
 INSERT INTO checkout.lookup_throttle(bucket,window_start,hits) VALUES(sha256(convert_to(p_key,'UTF8')),v_start,1)
  ON CONFLICT(bucket,window_start) DO UPDATE SET hits=checkout.lookup_throttle.hits+1 RETURNING hits INTO v_hits;
 -- bounded purge, deterministic order (same shape as identity.auth_throttle_hit)
 DELETE FROM checkout.lookup_throttle t USING (SELECT a.bucket,a.window_start FROM checkout.lookup_throttle a
   WHERE a.window_start<v_now-interval '1 day' ORDER BY a.window_start,a.bucket LIMIT 100) d WHERE t.bucket=d.bucket AND t.window_start=d.window_start;
 IF v_hits>p_limit THEN RAISE EXCEPTION 'rate_limited' USING ERRCODE='PT429'; END IF;
END $$;

CREATE FUNCTION checkout.guest_order_lookup(p_store uuid, p_ref text, p_kind text, p_contact bytea, p_token bytea, p_ttl bigint, p_ip bytea)
RETURNS TABLE(order_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_hex text; v_lo uuid; v_hi uuid; r record; v_hit record; v_cmp bytea; v_ok boolean:=false; v_phone text; v_dummy boolean;
BEGIN
 IF p_store IS NULL OR p_ref IS NULL OR length(p_ref)>64 OR p_kind IS NULL OR p_kind NOT IN ('email','phone')
  OR p_contact IS NULL OR octet_length(p_contact)<>32 OR p_token IS NULL OR octet_length(p_token)<>32 OR p_ttl IS NULL OR p_ttl<60 OR p_ttl>2592000
  OR (p_ip IS NOT NULL AND octet_length(p_ip)<>32) OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid lookup' USING ERRCODE='PT400'; END IF;
 v_hex:=lower(regexp_replace(p_ref,'[^0-9a-fA-F]','','g'));
 -- Counted before anything is read and keyed by what the caller sent, so the 429 says nothing about whether an order exists.
 PERFORM checkout.lookup_hit('ip:'||coalesce(encode(p_ip,'hex'),'none'),10);
 PERFORM checkout.lookup_hit('ref:'||p_store::text||':'||v_hex,5);
 PERFORM checkout.lookup_hit('store:'||p_store::text,200);
 -- an inactive store / tenant / owner is refused by buyer.issue_order_capability below (same empty answer as a mismatch)
 IF length(v_hex) IN (12,32) THEN
  IF length(v_hex)=32 THEN v_lo:=v_hex::uuid; v_hi:=v_lo;
  ELSE v_lo:=(v_hex||repeat('0',20))::uuid; v_hi:=(v_hex||repeat('f',20))::uuid; END IF;
  -- uuid compares bytewise, so a 12-hex prefix is one range scan on checkout.orders(id) UNIQUE; DRAFT orders were never placed.
  FOR r IN SELECT k.tenant_id,k.owner_id,k.id,k.buyer_email,k.destination_id FROM checkout.orders k
    WHERE k.store_id=p_store AND k.id BETWEEN v_lo AND v_hi AND k.commercial_state<>'DRAFT' ORDER BY k.id LIMIT 5 LOOP
   -- storefront.destination_snapshots is GUC-scoped for this role (0013 checkout_writer_read): scope the candidate, read its phone (always, so an
   -- email lookup does the same work as a phone lookup)
   PERFORM set_config('app.tenant_id',r.tenant_id::text,true),set_config('app.store_id',p_store::text,true),set_config('app.buyer_id',r.owner_id::text,true);
   SELECT d.phone INTO v_phone FROM storefront.destination_snapshots d WHERE d.tenant_id=r.tenant_id AND d.store_id=p_store AND d.owner_id=r.owner_id AND d.id=r.destination_id;
   -- digest-to-digest compare: nothing about the stored value is revealed by how long it takes (§E5); phone = digits without 886 / leading 0
   v_cmp:=CASE p_kind WHEN 'email' THEN sha256(convert_to(lower(coalesce(r.buyer_email,'')),'UTF8'))
    ELSE sha256(convert_to(regexp_replace(regexp_replace(regexp_replace(coalesce(v_phone,''),'[^0-9]','','g'),'^886',''),'^0',''),'UTF8')) END;
   IF v_cmp=p_contact AND NOT v_ok AND ((p_kind='phone' AND v_phone IS NOT NULL) OR (p_kind='email' AND r.buyer_email IS NOT NULL)) THEN v_ok:=true; v_hit:=r; END IF;
  END LOOP;
 END IF;
 IF NOT v_ok THEN
  v_dummy:=sha256(convert_to(v_hex,'UTF8'))=p_contact; -- the same hash + compare a real candidate costs
  RETURN;
 END IF;
 BEGIN
  PERFORM buyer.issue_order_capability(p_store,v_hit.owner_id,p_token,p_ttl);
 EXCEPTION WHEN SQLSTATE 'PT401' THEN
  RETURN; -- erased / inactive owner, store or tenant: the same empty answer as a mismatch
 END;
 order_id:=v_hit.id;
 RETURN NEXT;
END $$;

-- ---------------------------------------------------------------------------------------------------
-- F. Ownership, grants, comments
-- ---------------------------------------------------------------------------------------------------
ALTER FUNCTION notify.enqueue(uuid,uuid,uuid,text) OWNER TO commerce_checkout_writer;
ALTER FUNCTION notify.on_order_insert() OWNER TO commerce_checkout_writer;
ALTER FUNCTION notify.on_order_update() OWNER TO commerce_checkout_writer;
ALTER FUNCTION notify.on_cvs_shipment() OWNER TO commerce_checkout_writer;
ALTER FUNCTION notify.on_transfer_refund() OWNER TO commerce_checkout_writer;
ALTER FUNCTION notify.on_stripe_refund() OWNER TO commerce_checkout_writer;
ALTER FUNCTION notify.claim_batch(integer,integer,integer) OWNER TO commerce_checkout_writer;
ALTER FUNCTION notify.record_result(uuid,text,bytea) OWNER TO commerce_checkout_writer;
ALTER FUNCTION notify.read_store_settings(bytea,uuid) OWNER TO commerce_checkout_writer;
ALTER FUNCTION notify.set_store_settings(bytea,uuid,boolean) OWNER TO commerce_checkout_writer;
ALTER FUNCTION checkout.lookup_hit(text,integer) OWNER TO commerce_checkout_writer;
ALTER FUNCTION checkout.guest_order_lookup(uuid,text,text,bytea,bytea,bigint,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION notify.enqueue(uuid,uuid,uuid,text), notify.on_order_insert(), notify.on_order_update(), notify.on_cvs_shipment(),
 notify.on_transfer_refund(), notify.on_stripe_refund(), notify.claim_batch(integer,integer,integer), notify.record_result(uuid,text,bytea),
 notify.read_store_settings(bytea,uuid), notify.set_store_settings(bytea,uuid,boolean), checkout.lookup_hit(text,integer),
 checkout.guest_order_lookup(uuid,text,text,bytea,bytea,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION notify.claim_batch(integer,integer,integer), notify.record_result(uuid,text,bytea) TO commerce_worker;
GRANT EXECUTE ON FUNCTION notify.read_store_settings(bytea,uuid), notify.set_store_settings(bytea,uuid,boolean) TO commerce_runtime;
GRANT EXECUTE ON FUNCTION checkout.guest_order_lookup(uuid,text,text,bytea,bytea,bigint,bytea) TO commerce_buyer_issuer;

COMMENT ON SCHEMA notify IS 'internal/notify: buyer and merchant notification mail outbox. Owner commerce_checkout_writer. Non-goals: no SMTP, no body, no stored address.';
COMMENT ON TABLE notify.outbox IS 'internal/notify: one row per (order, kind), inserted by the notify triggers in the order transaction, drained by notify.claim_batch / record_result (commerce_worker). Keeps kind, state, attempts, timestamps and a recipient hash only; never a body or an address. Non-goal: not a delivery guarantee for UNKNOWN (never re-sent, I06).';
COMMENT ON COLUMN notify.outbox.tenant_id IS 'Scope copied from the order at enqueue.';
COMMENT ON COLUMN notify.outbox.store_id IS 'Scope copied from the order at enqueue; the per-store caps and the merchant batch group by it.';
COMMENT ON COLUMN notify.outbox.order_id IS 'checkout.orders.id the mail is about (merchant_new rows: one row per order, mailed together).';
COMMENT ON COLUMN notify.outbox.kind IS 'placed | paid | shipped | cancelled | refunded (buyer) | merchant_new (store owners). Exactly once per (order, kind).';
COMMENT ON COLUMN notify.outbox.state IS 'PENDING -> SENDING -> SENT | UNKNOWN (final) | back to PENDING after a definite refusal (FAILED after attempt 3) | SKIPPED (see skip_reason).';
COMMENT ON COLUMN notify.outbox.attempts IS 'SMTP attempts claimed so far (max 3).';
COMMENT ON COLUMN notify.outbox.next_attempt_at IS 'Earliest next claim (backoff 2 min then 10 min after a definite refusal).';
COMMENT ON COLUMN notify.outbox.batch_id IS 'Set at claim; record_result addresses the rows of one send by it (a merchant batch shares one).';
COMMENT ON COLUMN notify.outbox.claimed_at IS 'When the row last went SENDING; the daily and hourly caps count by it.';
COMMENT ON COLUMN notify.outbox.sent_at IS 'Set only with state SENT.';
COMMENT ON COLUMN notify.outbox.recipient_hash IS 'sha256("order id : lowercase address") of the buyer address (merchant_new: of the sorted owner list); NULLed by erasure (§E7).';
COMMENT ON COLUMN notify.outbox.skip_reason IS 'Why a row was never sent: no_recipient, opted_out, stale (24 h), no_owner, no_order, erased.';
COMMENT ON COLUMN notify.outbox.created_at IS 'Enqueue time; rows older than 180 days are deleted by claim_batch.';
COMMENT ON COLUMN notify.outbox.updated_at IS 'Last state change.';
COMMENT ON TABLE notify.store_settings IS 'internal/notify: per-store opt-out of the merchant new-order mail. Written only by notify.set_store_settings (integration:manage), read by claim_batch. No row = on.';
COMMENT ON COLUMN notify.store_settings.tenant_id IS 'Scope of the store.';
COMMENT ON COLUMN notify.store_settings.store_id IS 'The store the toggle belongs to.';
COMMENT ON COLUMN notify.store_settings.merchant_new_order_email IS 'false = merchant_new rows of the store are SKIPPED (opted_out).';
COMMENT ON COLUMN notify.store_settings.updated_at IS 'Last change time.';
COMMENT ON COLUMN notify.store_settings.updated_by IS 'identity principal that changed it.';
COMMENT ON TABLE checkout.lookup_throttle IS 'internal/buyerhttp guest order lookup: fixed 10-minute windows per hashed bucket (client IP, order ref, store). Written only by checkout.lookup_hit. Separate from identity.auth_throttle (login-code buckets).';
COMMENT ON COLUMN checkout.lookup_throttle.bucket IS 'sha256 of the bucket key (never the raw IP or order ref).';
COMMENT ON COLUMN checkout.lookup_throttle.window_start IS 'Start of the 600 s window (UTC aligned); rows older than a day are purged 100 per call.';
COMMENT ON COLUMN checkout.lookup_throttle.hits IS 'Calls counted in the window; over the bucket limit the lookup answers rate_limited.';
COMMENT ON FUNCTION notify.enqueue(uuid,uuid,uuid,text) IS 'internal/notify: trigger helper only (no caller EXECUTE). INSERT ... ON CONFLICT DO NOTHING; any failure becomes a WARNING so the order transaction is never rolled back by mail.';
COMMENT ON FUNCTION notify.on_order_insert() IS 'internal/notify: AFTER INSERT trigger function of checkout.orders (no caller EXECUTE): placed + merchant_new for AWAITING_TRANSFER and pay_at_pickup CONFIRMED orders.';
COMMENT ON FUNCTION notify.on_order_update() IS 'internal/notify: AFTER UPDATE trigger function of checkout.orders (no caller EXECUTE): paid, shipped, cancelled, refunded (offline collection), merchant_new, and the erasure clean-up of recipient_hash.';
COMMENT ON FUNCTION notify.on_cvs_shipment() IS 'internal/notify: trigger function of fulfillment.cvs_shipments (no caller EXECUTE): shipped when the parcel reaches AT_DC or AT_STORE.';
COMMENT ON FUNCTION notify.on_transfer_refund() IS 'internal/notify: trigger function of checkout.bank_transfers (no caller EXECUTE): refunded on REFUNDED_OFFLINE.';
COMMENT ON FUNCTION notify.on_stripe_refund() IS 'internal/notify: trigger function of payments.refund_facts (no caller EXECUTE): refunded on a SUCCEEDED refund fact.';
COMMENT ON FUNCTION notify.claim_batch(integer,integer,integer) IS 'internal/notify worker only; EXECUTE commerce_worker. Housekeeping (UNKNOWN for dead SENDING, stale SKIPPED, 180-day delete), then claims up to p_limit buyer rows (FOR UPDATE SKIP LOCKED) under the daily budget p_daily_cap and p_store_hourly per store, plus at most one merchant batch per store per 5 minutes. Returns the renderer input as jsonb; the recipient address leaves SQL only here, in memory.';
COMMENT ON FUNCTION notify.record_result(uuid,text,bytea) IS 'internal/notify worker only; EXECUTE commerce_worker. SENT / FAILED (retry or give up) / UNKNOWN (final) for the rows of one batch, only from SENDING; returns the row count.';
COMMENT ON FUNCTION notify.read_store_settings(bytea,uuid) IS 'internal/notify merchant route; EXECUTE commerce_runtime. integration:read via identity.resolve_access with a fresh final fence; no row = on.';
COMMENT ON FUNCTION notify.set_store_settings(bytea,uuid,boolean) IS 'internal/notify merchant route; EXECUTE commerce_runtime. integration:manage; idempotent upsert of the opt-out flag.';
COMMENT ON FUNCTION buyer.issue_order_capability(uuid,uuid,bytea,bigint) IS 'checkout.guest_order_lookup only (EXECUTE commerce_checkout_writer): registers a capability session for an EXISTING active buyer owner with the BFF-minted token hash; PT401 for an inactive owner, store or tenant. Non-goal: no new owner, no admission limit of its own (the lookup throttles first).';
COMMENT ON FUNCTION checkout.lookup_hit(text,integer) IS 'checkout.guest_order_lookup only (no caller EXECUTE): +1 in the current 600 s window of one hashed bucket, PT429 over the limit.';
COMMENT ON FUNCTION checkout.guest_order_lookup(uuid,text,text,bytea,bytea,bigint,bytea) IS 'internal/buyerhttp lookup route; EXECUTE commerce_buyer_issuer. Throttles (ip 10, order ref 5, store 200 per 10 min), finds the order by a 12-hex id prefix (or full id) in the store, compares sha256(email) or sha256(normalised phone) with p_contact, and on a match issues a capability for the order owner (p_token = sha256 of the BFF token). Returns the order id, or no row for every kind of mismatch.';
