-- Purpose: LC-U3 server authority for one unresolved order intent per authenticated store/principal.
-- Depends on: authenticated merchant scope, checkout Begin receipts and private buyer owners (0013/0094).
-- Used by: merchanttools.withAttempt, frozen-request replay and the manual provenance/link checks.
-- Invariants: I01/I02/I03/I06/I11; pending never expires into permission for a new key; settled payloads are erased.
-- Status: REAL_PG acceptance in CI; no external effects or changes to the legacy ten-minute source marker.

CREATE TABLE ops.order_intents (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 principal_id uuid NOT NULL,
 idempotency_key text NOT NULL CHECK(length(idempotency_key) BETWEEN 8 AND 128 AND idempotency_key ~ '^[A-Za-z0-9._:-]+$'),
 operation text NOT NULL CHECK(operation IN ('merchanttools.order.manual','merchanttools.order.for_buyer')),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 frozen_request jsonb CHECK(frozen_request IS NULL OR (jsonb_typeof(frozen_request)='object' AND octet_length(frozen_request::text)<=131072)),
 buyer_owner_id uuid NOT NULL,
 begin_key text NOT NULL,
 state text NOT NULL CHECK(state IN ('pending','created','sealed')),
 order_id uuid,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,idempotency_key),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,buyer_owner_id) REFERENCES buyer.owners(tenant_id,store_id,id),
 CHECK((state='pending' AND order_id IS NULL)
    OR (state='created' AND frozen_request IS NULL AND order_id IS NOT NULL)
    OR (state='sealed' AND frozen_request IS NULL AND order_id IS NULL))
);
CREATE UNIQUE INDEX order_intents_one_pending ON ops.order_intents(tenant_id,store_id,principal_id) WHERE state='pending';
ALTER TABLE ops.order_intents OWNER TO commerce_checkout_writer;
ALTER TABLE ops.order_intents ENABLE ROW LEVEL SECURITY;
ALTER TABLE ops.order_intents FORCE ROW LEVEL SECURITY;
REVOKE ALL ON ops.order_intents FROM PUBLIC;
GRANT SELECT,INSERT ON ops.order_intents TO commerce_runtime;
GRANT UPDATE(state,order_id,frozen_request,updated_at) ON ops.order_intents TO commerce_runtime;
CREATE POLICY order_intent_scope ON ops.order_intents TO commerce_runtime,commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
   AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
   AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
COMMENT ON TABLE ops.order_intents IS 'merchanttools: authenticated store/principal unresolved-order authority. Immutable key/hash/private-owner binding; erasable frozen canonical request only while pending, erased atomically on created/sealed. No browser token, buyer capability, automatic expiry, delete or public access.';
COMMENT ON COLUMN ops.order_intents.frozen_request IS 'Private pending intent input for explicit authoritative recovery; never returned to a browser or stored in append-only command receipts. Clear on created/sealed.';

CREATE FUNCTION ops.order_intent_transition() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.state<>'pending' OR NEW.frozen_request IS NULL OR NEW.frozen_request->>'kind' IS DISTINCT FROM NEW.operation THEN
   RAISE EXCEPTION 'invalid intent admission' USING ERRCODE='23514'; END IF;
 ELSE
  IF ROW(NEW.tenant_id,NEW.store_id,NEW.principal_id,NEW.idempotency_key,NEW.operation,NEW.request_hash,NEW.buyer_owner_id,NEW.begin_key,NEW.created_at)
   IS DISTINCT FROM ROW(OLD.tenant_id,OLD.store_id,OLD.principal_id,OLD.idempotency_key,OLD.operation,OLD.request_hash,OLD.buyer_owner_id,OLD.begin_key,OLD.created_at)
   OR (NEW.frozen_request IS NOT NULL AND NEW.frozen_request IS DISTINCT FROM OLD.frozen_request)
   OR (OLD.state<>'pending' AND ROW(NEW.state,NEW.order_id) IS DISTINCT FROM ROW(OLD.state,OLD.order_id)) THEN
   RAISE EXCEPTION 'immutable intent binding' USING ERRCODE='23514'; END IF;
  IF NEW.state='created' AND OLD.state='pending' AND NOT EXISTS(
   SELECT 1 FROM ops.command_results r WHERE r.tenant_id=OLD.tenant_id AND r.store_id=OLD.store_id
    AND r.principal_id=OLD.principal_id AND r.idempotency_key=OLD.idempotency_key
    AND r.operation=CASE WHEN OLD.operation='merchanttools.order.for_buyer' THEN 'merchanttools.order.for_buyer.completed' ELSE 'merchanttools.order.manual' END
    AND r.response->>'order_id'=NEW.order_id::text) THEN
   RAISE EXCEPTION 'missing completed intent receipt' USING ERRCODE='23514'; END IF;
  IF NEW.state='sealed' AND OLD.state='pending' AND NOT EXISTS(
   SELECT 1 FROM ops.command_results r WHERE r.tenant_id=OLD.tenant_id AND r.store_id=OLD.store_id
    AND r.principal_id=OLD.principal_id AND r.operation='merchanttools.order.checkout.closed'
    AND r.idempotency_key=OLD.begin_key AND r.response->>'owner_id'=OLD.buyer_owner_id::text) THEN
   RAISE EXCEPTION 'missing intent closure' USING ERRCODE='23514'; END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION ops.order_intent_transition() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION ops.order_intent_transition() FROM PUBLIC;
COMMENT ON FUNCTION ops.order_intent_transition() IS 'merchanttools: immutable intent key/hash/private-owner/canonical payload; monotone settlement requires exact same-principal completed or private Begin closure receipt. Erasure may clear payload without changing pending authority. Trigger only, no runtime execute.';
CREATE TRIGGER order_intent_transition BEFORE INSERT OR UPDATE ON ops.order_intents FOR EACH ROW EXECUTE FUNCTION ops.order_intent_transition();

-- Adopt only an old durable key binding and its unique exact Begin receipt. Never derive an owner with today's secret.
CREATE FUNCTION fulfillment.order_intent_legacy_owner(p_hash bytea,p_store uuid,p_key text,p_begin text) RETURNS uuid
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; c record; v_owner uuid; v_count integer;
BEGIN
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'inventory:reserve');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 SELECT * INTO c FROM identity.resolve_access(p_hash,p_store,'catalog:read');
 IF c.access_status IS DISTINCT FROM 'ok' OR c.principal_id IS DISTINCT FROM s.principal_id THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF nullif(current_setting('app.tenant_id',true),'')::uuid IS DISTINCT FROM s.tenant_id
  OR nullif(current_setting('app.store_id',true),'')::uuid IS DISTINCT FROM p_store
  OR nullif(current_setting('app.principal_id',true),'')::uuid IS DISTINCT FROM s.principal_id THEN
  RAISE EXCEPTION 'invalid scope' USING ERRCODE='PT404'; END IF;
 IF NOT EXISTS(SELECT 1 FROM ops.command_results r WHERE r.tenant_id=s.tenant_id AND r.store_id=p_store
  AND r.principal_id=s.principal_id AND r.operation='merchanttools.order.attempt' AND r.idempotency_key=p_key) THEN RETURN NULL; END IF;
 SELECT count(*),(array_agg(r.owner_id))[1] INTO v_count,v_owner FROM checkout.command_results r
 JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.owner_id=r.owner_id AND o.id=r.order_id
 WHERE r.tenant_id=s.tenant_id AND r.store_id=p_store AND r.operation='checkout.begin' AND r.idempotency_key=p_begin;
 IF v_count<>1 THEN RETURN NULL; END IF;
 RETURN v_owner;
END $$;
ALTER FUNCTION fulfillment.order_intent_legacy_owner(bytea,uuid,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.order_intent_legacy_owner(bytea,uuid,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.order_intent_legacy_owner(bytea,uuid,text,text) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.order_intent_legacy_owner(bytea,uuid,text,text) IS 'merchanttools upgrade-only: authenticate same principal/store and existing original attempt binding; return its unique exact private-owner Begin receipt/order owner. Missing or conflicting durable evidence returns NULL/UNKNOWN, never absence from a re-derived capability.';

-- Read the exact durable checkout commit without depending on a transient capability or current delivery catalog.
CREATE FUNCTION fulfillment.order_intent_begin(p_hash bytea,p_store uuid,p_key text)
RETURNS TABLE(order_id uuid,commercial_state text,payment_mode text,total_minor bigint,currency text,expires_at timestamptz)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; c record; i record;
BEGIN
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'inventory:reserve');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 SELECT * INTO c FROM identity.resolve_access(p_hash,p_store,'catalog:read');
 IF c.access_status IS DISTINCT FROM 'ok' OR c.principal_id IS DISTINCT FROM s.principal_id THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF nullif(current_setting('app.tenant_id',true),'')::uuid IS DISTINCT FROM s.tenant_id
  OR nullif(current_setting('app.store_id',true),'')::uuid IS DISTINCT FROM p_store
  OR nullif(current_setting('app.principal_id',true),'')::uuid IS DISTINCT FROM s.principal_id THEN
  RAISE EXCEPTION 'invalid scope' USING ERRCODE='PT404'; END IF;
 SELECT * INTO i FROM ops.order_intents x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
  AND x.principal_id=s.principal_id AND x.idempotency_key=p_key AND x.state IN ('pending','created');
 IF NOT FOUND THEN RAISE EXCEPTION 'intent not found' USING ERRCODE='PT409'; END IF;
 IF (EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=i.tenant_id AND o.store_id=i.store_id AND o.owner_id=i.buyer_owner_id)
  OR EXISTS(SELECT 1 FROM checkout.command_results r WHERE r.tenant_id=i.tenant_id AND r.store_id=i.store_id
   AND r.owner_id=i.buyer_owner_id AND r.operation='checkout.begin' AND r.idempotency_key=i.begin_key))
  AND NOT EXISTS(SELECT 1 FROM checkout.command_results r JOIN checkout.orders o
   ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.owner_id=r.owner_id AND o.id=r.order_id
   WHERE r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.owner_id=i.buyer_owner_id
    AND r.operation='checkout.begin' AND r.idempotency_key=i.begin_key) THEN
  RAISE EXCEPTION 'unresolved checkout facts' USING ERRCODE='PT409'; END IF;
 RETURN QUERY SELECT o.id,o.commercial_state,o.payment_mode,o.total_minor,o.currency,o.expires_at
 FROM ops.order_intents x
 JOIN checkout.command_results r ON r.tenant_id=x.tenant_id AND r.store_id=x.store_id AND r.owner_id=x.buyer_owner_id
  AND r.operation='checkout.begin' AND r.idempotency_key=x.begin_key
 JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.owner_id=r.owner_id AND o.id=r.order_id
 WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.principal_id=s.principal_id
  AND x.idempotency_key=p_key AND x.state IN ('pending','created');
END $$;
ALTER FUNCTION fulfillment.order_intent_begin(bytea,uuid,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.order_intent_begin(bytea,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.order_intent_begin(bytea,uuid,text) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.order_intent_begin(bytea,uuid,text) IS 'merchanttools: exact authenticated principal/store intent + private-owner Begin receipt/order lookup for replay; inventory:reserve and catalog:read reverified. No arbitrary order read, buyer capability expiry dependency or writes.';

-- Only an admitted intent and its exact private-owner Begin commit can establish manual provenance beyond ten minutes.
CREATE FUNCTION fulfillment.mark_order_intent_manual(p_hash bytea,p_store uuid,p_key text,p_order uuid,p_link bytea) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v record; i record; v_now timestamptz:=clock_timestamp();
BEGIN
 IF p_link IS NULL OR octet_length(p_link)<>32 THEN RAISE EXCEPTION 'invalid link' USING ERRCODE='PT400'; END IF;
 SELECT * INTO v FROM fulfillment.order_intent_begin(p_hash,p_store,p_key);
 IF NOT FOUND OR v.order_id IS DISTINCT FROM p_order THEN RAISE EXCEPTION 'intent order not found' USING ERRCODE='PT409'; END IF;
 SELECT * INTO i FROM ops.order_intents WHERE tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=p_store AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid AND idempotency_key=p_key;
 PERFORM pg_advisory_xact_lock(hashtextextended('order-pay-link|'||i.tenant_id::text||'|'||p_store::text||'|'||p_order::text,0));
 PERFORM 1 FROM checkout.orders o WHERE o.tenant_id=i.tenant_id AND o.store_id=p_store AND o.owner_id=i.buyer_owner_id AND o.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'intent order not found' USING ERRCODE='PT409'; END IF;
 UPDATE checkout.orders SET source='merchant_manual' WHERE tenant_id=i.tenant_id AND store_id=p_store AND owner_id=i.buyer_owner_id AND id=p_order;
 INSERT INTO checkout.order_links(token_hash,tenant_id,store_id,owner_id,order_id,created_at,expires_at)
  VALUES(p_link,i.tenant_id,p_store,i.buyer_owner_id,p_order,v_now,v_now+interval '7 days') ON CONFLICT(token_hash) DO NOTHING;
END $$;
ALTER FUNCTION fulfillment.mark_order_intent_manual(bytea,uuid,text,uuid,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.mark_order_intent_manual(bytea,uuid,text,uuid,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.mark_order_intent_manual(bytea,uuid,text,uuid,bytea) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.mark_order_intent_manual(bytea,uuid,text,uuid,bytea) IS 'merchanttools: completes exact durable admitted intent provenance and original buyer-link hash after Begin, with no age limit. Authenticated principal/store, private buyer owner and immutable Begin receipt/order must agree. Existing 0094 arbitrary recent-source marker stays unchanged. Same order pay-link lock; no price, stock or payment changes.';

-- Do not broaden checkout's table rights just to establish that redemption's private owner is still active.
CREATE FUNCTION buyer.order_intent_owner_active(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS boolean
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF p_tenant IS DISTINCT FROM nullif(current_setting('app.tenant_id',true),'')::uuid
  OR p_store IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid THEN RETURN false; END IF;
 RETURN EXISTS(SELECT 1 FROM buyer.owners o WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_owner AND o.active);
END $$;
ALTER FUNCTION buyer.order_intent_owner_active(uuid,uuid,uuid) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.order_intent_owner_active(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.order_intent_owner_active(uuid,uuid,uuid) TO commerce_checkout_writer;
COMMENT ON FUNCTION buyer.order_intent_owner_active(uuid,uuid,uuid) IS 'merchanttools order-link proof only: checkout definer may test the exact active private owner in its authenticated tenant/store scope. Boolean only, no capability, owner table grant or runtime execute.';

CREATE FUNCTION fulfillment.order_pay_link_valid(p_hash bytea,p_store uuid,p_order uuid,p_link bytea) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; c record;
BEGIN
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'inventory:reserve');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 SELECT * INTO c FROM identity.resolve_access(p_hash,p_store,'catalog:read');
 IF c.access_status IS DISTINCT FROM 'ok' OR c.principal_id IS DISTINCT FROM s.principal_id THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF nullif(current_setting('app.tenant_id',true),'')::uuid IS DISTINCT FROM s.tenant_id
  OR nullif(current_setting('app.store_id',true),'')::uuid IS DISTINCT FROM p_store
  OR nullif(current_setting('app.principal_id',true),'')::uuid IS DISTINCT FROM s.principal_id THEN
  RAISE EXCEPTION 'invalid scope' USING ERRCODE='PT404'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('order-pay-link|'||s.tenant_id::text||'|'||p_store::text||'|'||p_order::text,0));
 RETURN EXISTS(SELECT 1 FROM checkout.order_links l JOIN checkout.orders o
  ON o.tenant_id=l.tenant_id AND o.store_id=l.store_id AND o.owner_id=l.owner_id AND o.id=l.order_id
  WHERE l.tenant_id=s.tenant_id AND l.store_id=p_store AND l.order_id=p_order AND l.token_hash=p_link
   AND l.redeemed_at IS NULL AND l.expires_at>clock_timestamp() AND o.source='merchant_manual'
   AND o.commercial_state<>'CANCELLED' AND buyer.order_intent_owner_active(l.tenant_id,l.store_id,l.owner_id));
END $$;
ALTER FUNCTION fulfillment.order_pay_link_valid(bytea,uuid,uuid,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.order_pay_link_valid(bytea,uuid,uuid,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.order_pay_link_valid(bytea,uuid,uuid,bytea) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.order_pay_link_valid(bytea,uuid,uuid,bytea) IS 'merchanttools: validates the exact currently redeemable order-link hash under the same order pay-link lock as regeneration and DM planning. Auth-derived tenant/store and inventory:reserve/catalog:read; no link secret read or writes.';

-- The privacy writer can erase only the payload for its exact owner, retaining the unresolved money fence.
GRANT SELECT(tenant_id,store_id,buyer_owner_id,state),UPDATE(frozen_request) ON ops.order_intents TO commerce_privacy_writer;
CREATE POLICY order_intent_privacy_read ON ops.order_intents FOR SELECT TO commerce_privacy_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY order_intent_privacy_update ON ops.order_intents FOR UPDATE TO commerce_privacy_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE FUNCTION customers.erase_order_intent_payload(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF p_tenant IS DISTINCT FROM nullif(current_setting('app.tenant_id',true),'')::uuid
  OR p_store IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid THEN
  RAISE EXCEPTION 'invalid erasure scope' USING ERRCODE='PT404'; END IF;
 UPDATE ops.order_intents SET frozen_request=NULL WHERE tenant_id=p_tenant AND store_id=p_store AND buyer_owner_id=p_owner AND state='pending';
END $$;
ALTER FUNCTION customers.erase_order_intent_payload(uuid,uuid,uuid) OWNER TO commerce_privacy_writer;
REVOKE ALL ON FUNCTION customers.erase_order_intent_payload(uuid,uuid,uuid) FROM PUBLIC;
COMMENT ON FUNCTION customers.erase_order_intent_payload(uuid,uuid,uuid) IS 'customers erasure-only: remove pending private order-intent input for the exact owner/store; never settle or release its money fence. No runtime execute or access to keys/hashes. Called by apply_erasure/replay_erasures.';
DO $patch$
DECLARE body text; needle text:='-- CD4: withdraw every granted pair';
BEGIN
 SELECT prosrc INTO body FROM pg_proc WHERE oid='customers.apply_erasure(uuid,uuid,uuid)'::regprocedure;
 IF position(needle IN body)=0 OR position('erase_order_intent_payload' IN body)>0 THEN RAISE EXCEPTION 'erasure baseline drift'; END IF;
 body:=replace(body,needle,'PERFORM customers.erase_order_intent_payload(p_tenant,p_store,p_owner); '||needle);
 EXECUTE 'CREATE OR REPLACE FUNCTION customers.apply_erasure(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS '||quote_literal(body);
END $patch$;

-- Historical cancellation completes bookkeeping for this admitted intent, without relaxing for_buyer_finish.
GRANT SELECT(tenant_id,store_id,principal_id,idempotency_key,operation,buyer_owner_id,begin_key,state) ON ops.order_intents TO commerce_claims_writer;
CREATE POLICY order_intent_claims_read ON ops.order_intents FOR SELECT TO commerce_claims_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
GRANT SELECT(tenant_id,store_id,owner_id,operation,idempotency_key,order_id) ON checkout.command_results TO commerce_claims_writer;
CREATE POLICY order_intent_cancelled_begin_read ON checkout.command_results FOR SELECT TO commerce_claims_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND operation='checkout.begin' AND EXISTS(SELECT 1 FROM ops.order_intents i
   WHERE i.tenant_id=checkout.command_results.tenant_id AND i.store_id=checkout.command_results.store_id
    AND i.principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
    AND i.operation='merchanttools.order.for_buyer' AND i.buyer_owner_id=checkout.command_results.owner_id
    AND i.begin_key=checkout.command_results.idempotency_key));
CREATE FUNCTION claims.close_cancelled_for_buyer(p_key_hash bytea,p_request_hash bytea,p_buyer uuid,p_order uuid)
RETURNS TABLE(bundle_id uuid,offer_id uuid,sku_id uuid,quantity bigint,live_price_minor bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; v_request uuid;
BEGIN
 v:=claims.for_buyer_scope();
 IF p_key_hash IS NULL OR octet_length(p_key_hash)<>32 OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_buyer IS NULL OR p_order IS NULL THEN
  RAISE EXCEPTION 'invalid intent request' USING ERRCODE='22023'; END IF;
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve','catalog:read']) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF NOT EXISTS(SELECT 1 FROM ops.order_intents i JOIN checkout.command_results r
  ON r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.owner_id=i.buyer_owner_id
   AND r.operation='checkout.begin' AND r.idempotency_key=i.begin_key AND r.order_id=p_order
  JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.owner_id=r.owner_id AND o.id=r.order_id
  WHERE i.tenant_id=v[1] AND i.store_id=v[2] AND i.principal_id=v[3] AND i.operation='merchanttools.order.for_buyer'
   AND i.buyer_owner_id=p_buyer AND i.state IN ('pending','created') AND o.commercial_state='CANCELLED'
   AND sha256(convert_to('order-for-buyer|'||i.store_id::text||'|'||i.idempotency_key,'UTF8'))=p_key_hash) THEN
  RAISE EXCEPTION 'unresolved intent' USING ERRCODE='PT409'; END IF;
 SELECT x.request_id INTO v_request FROM inbox.order_for_buyer x
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.idempotency_key_hash=p_key_hash LIMIT 1;
 IF NOT FOUND OR EXISTS(SELECT 1 FROM inbox.order_for_buyer x
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.idempotency_key_hash=p_key_hash
   AND (x.request_id IS DISTINCT FROM v_request OR x.request_hash IS DISTINCT FROM p_request_hash
    OR x.buyer_id IS DISTINCT FROM p_buyer OR (x.order_id IS NOT NULL AND x.order_id<>p_order))) THEN
  RAISE EXCEPTION 'unresolved intent request' USING ERRCODE='PT409'; END IF;
 -- Released history stays released; another buyer's current reservation is never read or overwritten.
 UPDATE inbox.order_for_buyer x SET state='released',updated_at=clock_timestamp()
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.request_id=v_request AND x.buyer_id=p_buyer AND x.state='pending';
 UPDATE claims.merchant_origin_grants g SET expires_at=clock_timestamp()
  WHERE g.tenant_id=v[1] AND g.store_id=v[2] AND g.request_id=v_request AND g.buyer_id=p_buyer
   AND g.consumed_at IS NULL AND g.expires_at>clock_timestamp();
 RETURN QUERY SELECT u.bundle_id,u.offer_id,l.sku_id,u.quantity::bigint,u.unit_price_minor
  FROM claims.live_price_uses u JOIN claims.lines l
   ON l.tenant_id=u.tenant_id AND l.store_id=u.store_id AND l.bundle_id=u.bundle_id AND l.offer_id=u.offer_id
  WHERE u.tenant_id=v[1] AND u.store_id=v[2] AND u.order_id=p_order ORDER BY u.bundle_id,u.offer_id;
END $$;
ALTER FUNCTION claims.close_cancelled_for_buyer(bytea,bytea,uuid,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.close_cancelled_for_buyer(bytea,bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.close_cancelled_for_buyer(bytea,bytea,uuid,uuid) TO commerce_runtime;
COMMENT ON FUNCTION claims.close_cancelled_for_buyer(bytea,bytea,uuid,uuid) IS 'internal/claims LC-U3 durable-intent cancellation reconciliation only: exact authenticated principal/store intent, original owner/Begin/CANCELLED order and original key/body-hash reservation must agree. Release only original pending rows, expire unconsumed grants and return historical consumed price lines. No Begin, stock, money, fresh grants, peer matching or other request changes; existing non-cancelled for_buyer_finish unchanged.';
