-- Checkout is a separate trusted server authority, never a buyer or merchant
-- membership. Login roles are provisioned outside migrations and remain disjoint.
CREATE ROLE commerce_checkout_runtime NOLOGIN NOINHERIT NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE commerce_checkout_writer NOLOGIN NOINHERIT NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE SCHEMA checkout AUTHORIZATION commerce_checkout_writer;
REVOKE ALL ON SCHEMA checkout FROM PUBLIC;
GRANT USAGE ON SCHEMA checkout TO commerce_checkout_runtime;
GRANT USAGE ON SCHEMA checkout TO commerce_worker;
GRANT USAGE ON SCHEMA buyer,control,storefront,catalog,pricing,fulfillment,inventory,river TO commerce_checkout_writer;
GRANT USAGE ON SCHEMA buyer,control,storefront,catalog,pricing,fulfillment,inventory,river TO commerce_checkout_runtime;
GRANT EXECUTE ON FUNCTION buyer.resolve_scope(bytea,uuid) TO commerce_checkout_runtime,commerce_checkout_writer;

CREATE TABLE checkout.orders (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,id uuid NOT NULL,
 creator_session_id uuid NOT NULL,cart_id uuid NOT NULL,cart_version bigint NOT NULL CHECK(cart_version>0),
 quote_id uuid NOT NULL,destination_id uuid NOT NULL,market_id uuid NOT NULL,
 country text NOT NULL CHECK(country ~ '^[A-Z]{2}$'),
 service_code text NOT NULL CHECK(service_code ~ '^[a-z][a-z0-9_-]{0,39}$'),
 service_version bigint NOT NULL CHECK(service_version>0),allocation_version bigint NOT NULL CHECK(allocation_version>0),
 currency text NOT NULL CHECK(currency ~ '^[A-Z]{3}$'),
 total_minor bigint NOT NULL CHECK(total_minor BETWEEN 0 AND 1000000000000),
 commercial_state text NOT NULL CHECK(commercial_state IN ('DRAFT','AWAITING_PAYMENT','CONFIRMED','CANCELLED')),
 fulfillment_state text NOT NULL CHECK(fulfillment_state IN ('MANUAL_UNASSIGNED','CANCELLED')),
 generation bigint NOT NULL CHECK(generation>0),expires_at timestamptz NOT NULL,
 job_id bigint NOT NULL UNIQUE,
 snapshot jsonb NOT NULL CHECK(jsonb_typeof(snapshot)='object' AND octet_length(snapshot::text)<=1048576),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,owner_id,id),UNIQUE(id),
 UNIQUE(tenant_id,store_id,id,owner_id,creator_session_id),
 FOREIGN KEY(tenant_id,store_id,owner_id,creator_session_id)
  REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,cart_id,quote_id)
  REFERENCES storefront.quotes(tenant_id,store_id,owner_id,cart_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,destination_id)
  REFERENCES storefront.destination_snapshots(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,market_id,country,service_code,service_version)
  REFERENCES fulfillment.service_versions(tenant_id,store_id,market_id,country,code,version),
 FOREIGN KEY(tenant_id,store_id,market_id,country,service_code,allocation_version)
  REFERENCES fulfillment.allocation_versions(tenant_id,store_id,market_id,country,code,version),
 CHECK(expires_at>created_at AND expires_at<=created_at+interval '15 minutes 1 second')
);
CREATE UNIQUE INDEX checkout_one_active_cart_version ON checkout.orders
 (tenant_id,store_id,owner_id,cart_id,cart_version)
 WHERE commercial_state IN ('DRAFT','AWAITING_PAYMENT','CONFIRMED');
CREATE TABLE checkout.command_results (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,
 operation text NOT NULL CHECK(operation='checkout.begin'),
 idempotency_key text NOT NULL CHECK(idempotency_key ~ '^[A-Za-z0-9_.:-]{8,128}$'),
 creator_session_id uuid NOT NULL,request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 order_id uuid NOT NULL,response jsonb NOT NULL CHECK(jsonb_typeof(response)='object' AND octet_length(response::text)<=4096),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,owner_id,operation,idempotency_key),
 FOREIGN KEY(tenant_id,store_id,owner_id,creator_session_id)
  REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id)
);
CREATE TABLE checkout.events (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,id uuid NOT NULL DEFAULT gen_random_uuid(),
 order_id uuid NOT NULL,session_id uuid NOT NULL,generation bigint NOT NULL CHECK(generation>0),
 action text NOT NULL CHECK(action IN ('checkout.held','checkout.expired')),
 actor_kind text NOT NULL CHECK(actor_kind IN ('BUYER','SYSTEM_EXPIRY')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,session_id)
  REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id),
 CHECK((action='checkout.held' AND actor_kind='BUYER') OR (action='checkout.expired' AND actor_kind='SYSTEM_EXPIRY'))
);

-- Keep the order and its stock reservation as one scoped aggregate. Deferred
-- opposite-direction FKs allow their creation in one transaction, never alone.
ALTER TABLE inventory.reservations
 ADD COLUMN checkout_id uuid,ADD COLUMN buyer_owner_id uuid,ADD COLUMN buyer_session_id uuid,
 ADD COLUMN generation bigint;
ALTER TABLE inventory.reservations ADD CONSTRAINT reservation_checkout_family CHECK
 ((checkout_id IS NULL AND buyer_owner_id IS NULL AND buyer_session_id IS NULL AND generation IS NULL)
  OR (checkout_id IS NOT NULL AND checkout_id=id AND buyer_owner_id IS NOT NULL
      AND buyer_session_id IS NOT NULL AND generation>0));
ALTER TABLE inventory.reservations ADD CONSTRAINT reservation_buyer_session_fk
 FOREIGN KEY(tenant_id,store_id,buyer_owner_id,buyer_session_id)
 REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id);
ALTER TABLE inventory.reservations ADD CONSTRAINT reservation_checkout_identity UNIQUE
 (tenant_id,store_id,id,buyer_owner_id,buyer_session_id);
ALTER TABLE inventory.reservations ADD CONSTRAINT reservation_order_fk
 FOREIGN KEY(tenant_id,store_id,checkout_id,buyer_owner_id,buyer_session_id)
 REFERENCES checkout.orders(tenant_id,store_id,id,owner_id,creator_session_id)
 DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE checkout.orders ADD CONSTRAINT order_reservation_fk
 FOREIGN KEY(tenant_id,store_id,id,owner_id,creator_session_id)
 REFERENCES inventory.reservations(tenant_id,store_id,id,buyer_owner_id,buyer_session_id)
 DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE inventory.ledger ALTER COLUMN principal_id DROP NOT NULL;
ALTER TABLE inventory.ledger
 ADD COLUMN checkout_id uuid,ADD COLUMN buyer_owner_id uuid,ADD COLUMN buyer_session_id uuid,
 ADD COLUMN actor_kind text NOT NULL DEFAULT 'MERCHANT';
ALTER TABLE inventory.ledger ADD CONSTRAINT ledger_checkout_actor CHECK
 ((actor_kind='MERCHANT' AND principal_id IS NOT NULL AND checkout_id IS NULL
   AND buyer_owner_id IS NULL AND buyer_session_id IS NULL)
  OR (actor_kind='BUYER' AND principal_id IS NULL AND checkout_id IS NOT NULL
   AND buyer_owner_id IS NOT NULL AND buyer_session_id IS NOT NULL
   AND reservation_id=checkout_id AND kind='RESERVE')
  OR (actor_kind='SYSTEM_EXPIRY' AND principal_id IS NULL AND checkout_id IS NOT NULL
   AND buyer_owner_id IS NOT NULL AND buyer_session_id IS NOT NULL
   AND reservation_id=checkout_id AND kind='RELEASE'));
ALTER TABLE inventory.ledger ADD CONSTRAINT ledger_checkout_fk
 FOREIGN KEY(tenant_id,store_id,checkout_id,buyer_owner_id,buyer_session_id)
 REFERENCES checkout.orders(tenant_id,store_id,id,owner_id,creator_session_id);

-- Existing merchant commands may still operate on their own reservations.
-- They cannot release or append facts to a checkout-owned aggregate.
CREATE POLICY reservation_merchant_fence ON inventory.reservations AS RESTRICTIVE
 FOR INSERT TO commerce_runtime WITH CHECK(checkout_id IS NULL);
CREATE POLICY reservation_merchant_update_fence ON inventory.reservations AS RESTRICTIVE
 FOR UPDATE TO commerce_runtime USING(checkout_id IS NULL) WITH CHECK(checkout_id IS NULL);
CREATE POLICY reservation_line_merchant_fence ON inventory.reservation_lines AS RESTRICTIVE
 FOR INSERT TO commerce_runtime WITH CHECK(EXISTS (
  SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=reservation_lines.tenant_id
   AND r.store_id=reservation_lines.store_id AND r.id=reservation_lines.reservation_id
   AND r.checkout_id IS NULL));
CREATE POLICY ledger_merchant_fence ON inventory.ledger AS RESTRICTIVE
 FOR INSERT TO commerce_runtime WITH CHECK(actor_kind='MERCHANT' AND checkout_id IS NULL
  AND (reservation_id IS NULL OR EXISTS (
   SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=ledger.tenant_id
    AND r.store_id=ledger.store_id AND r.id=ledger.reservation_id AND r.checkout_id IS NULL)));

-- Actor provenance is checked before the existing apply_ledger AFTER trigger.
-- apply_ledger remains the only balance writer.
CREATE FUNCTION inventory.guard_checkout_ledger() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_checkout uuid; v_owner uuid; v_session uuid;
BEGIN
 IF NEW.actor_kind='MERCHANT' THEN
  IF NEW.principal_id IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
     OR NEW.checkout_id IS NOT NULL OR NEW.buyer_owner_id IS NOT NULL OR NEW.buyer_session_id IS NOT NULL THEN
   RAISE EXCEPTION 'ledger actor mismatch' USING ERRCODE='42501'; END IF;
 ELSE
  IF NEW.principal_id IS NOT NULL OR current_setting('app.principal_id',true)<>''
     OR NEW.buyer_owner_id IS DISTINCT FROM nullif(current_setting('app.buyer_id',true),'')::uuid
     OR NEW.buyer_session_id IS DISTINCT FROM nullif(current_setting('app.buyer_session_id',true),'')::uuid
     OR NEW.checkout_id IS DISTINCT FROM NEW.reservation_id THEN
   RAISE EXCEPTION 'ledger actor mismatch' USING ERRCODE='42501'; END IF;
 END IF;
 IF NEW.reservation_id IS NOT NULL THEN
  SELECT r.checkout_id,r.buyer_owner_id,r.buyer_session_id INTO v_checkout,v_owner,v_session
   FROM inventory.reservations r WHERE r.tenant_id=NEW.tenant_id AND r.store_id=NEW.store_id
    AND r.id=NEW.reservation_id;
  IF NOT FOUND OR v_checkout IS DISTINCT FROM NEW.checkout_id
     OR v_owner IS DISTINCT FROM NEW.buyer_owner_id OR v_session IS DISTINCT FROM NEW.buyer_session_id THEN
   RAISE EXCEPTION 'ledger reservation mismatch' USING ERRCODE='42501'; END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inventory.guard_checkout_ledger() OWNER TO commerce_inventory_writer;
GRANT SELECT ON inventory.reservations TO commerce_inventory_writer;
REVOKE ALL ON FUNCTION inventory.guard_checkout_ledger() FROM PUBLIC;
CREATE TRIGGER checkout_ledger_guard BEFORE INSERT ON inventory.ledger
 FOR EACH ROW EXECUTE FUNCTION inventory.guard_checkout_ledger();

DO $$ DECLARE r text; BEGIN
 FOREACH r IN ARRAY ARRAY['orders','command_results','events'] LOOP
  EXECUTE format('ALTER TABLE checkout.%I ENABLE ROW LEVEL SECURITY',r);
  EXECUTE format('ALTER TABLE checkout.%I FORCE ROW LEVEL SECURITY',r);
  EXECUTE format('CREATE POLICY writer_access ON checkout.%I TO commerce_checkout_writer USING(true) WITH CHECK(true)',r);
 END LOOP;
 FOREACH r IN ARRAY ARRAY['orders','command_results'] LOOP
  EXECUTE format('CREATE POLICY owner_read ON checkout.%I FOR SELECT TO commerce_checkout_runtime
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid
    AND owner_id=nullif(current_setting(''app.buyer_id'',true),'''')::uuid)',r);
 END LOOP;
END $$;
GRANT SELECT ON checkout.orders TO commerce_checkout_runtime;
GRANT SELECT,INSERT ON checkout.orders TO commerce_checkout_writer;
GRANT UPDATE(commercial_state,fulfillment_state,generation,expires_at,updated_at)
 ON checkout.orders TO commerce_checkout_writer;
GRANT SELECT,INSERT ON checkout.command_results,checkout.events TO commerce_checkout_writer;

-- Private writer sees only the transaction's scoped rows on old domains.
DO $$ DECLARE r text; BEGIN
 FOREACH r IN ARRAY ARRAY['storefront.carts','storefront.cart_lines','storefront.quotes',
  'storefront.destination_snapshots','storefront.destination_heads'] LOOP
  EXECUTE format('CREATE POLICY checkout_writer_read ON %s FOR SELECT TO commerce_checkout_writer
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid
    AND owner_id=nullif(current_setting(''app.buyer_id'',true),'''')::uuid)',r);
  EXECUTE format('CREATE POLICY checkout_runtime_read ON %s FOR SELECT TO commerce_checkout_runtime
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid
    AND owner_id=nullif(current_setting(''app.buyer_id'',true),'''')::uuid)',r);
 END LOOP;
 FOREACH r IN ARRAY ARRAY['pricing.markets','pricing.policy_versions','pricing.policy_heads',
  'catalog.products','catalog.skus','fulfillment.service_versions','fulfillment.service_heads',
  'fulfillment.allocation_versions','fulfillment.allocation_warehouses','fulfillment.allocation_heads',
  'fulfillment.pickup_versions','fulfillment.pickup_heads','inventory.warehouses','inventory.balances',
  'inventory.reservations','inventory.reservation_lines','inventory.ledger'] LOOP
  EXECUTE format('CREATE POLICY checkout_writer_read ON %s FOR SELECT TO commerce_checkout_writer
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',r);
  EXECUTE format('CREATE POLICY checkout_runtime_read ON %s FOR SELECT TO commerce_checkout_runtime
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',r);
 END LOOP;
END $$;
CREATE POLICY checkout_writer_order_stock ON inventory.reservations TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND checkout_id IS NOT NULL AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND buyer_session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
CREATE POLICY checkout_writer_lines ON inventory.reservation_lines FOR INSERT TO commerce_checkout_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND EXISTS(SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=reservation_lines.tenant_id
   AND r.store_id=reservation_lines.store_id AND r.id=reservation_lines.reservation_id
   AND r.checkout_id IS NOT NULL AND r.buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid));
CREATE POLICY checkout_writer_ledger ON inventory.ledger FOR INSERT TO commerce_checkout_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND actor_kind IN ('BUYER','SYSTEM_EXPIRY') AND principal_id IS NULL
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND buyer_session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);

GRANT SELECT ON storefront.carts,storefront.cart_lines,storefront.quotes,
 storefront.destination_snapshots,storefront.destination_heads,
 pricing.markets,pricing.policy_versions,pricing.policy_heads,catalog.products,catalog.skus,
 fulfillment.service_versions,fulfillment.service_heads,
 fulfillment.allocation_versions,fulfillment.allocation_warehouses,fulfillment.allocation_heads,
 fulfillment.pickup_heads,inventory.warehouses,inventory.balances,
 inventory.reservations,inventory.reservation_lines,inventory.ledger TO commerce_checkout_writer;
GRANT SELECT(tenant_id,store_id,id,kind,namespace,code,version,country,name,address,
 verification_kind,attested_at,valid_until) ON fulfillment.pickup_versions TO commerce_checkout_writer;
GRANT INSERT,UPDATE(state) ON inventory.reservations TO commerce_checkout_writer;
GRANT INSERT ON inventory.reservation_lines,inventory.ledger TO commerce_checkout_writer;
GRANT SELECT ON control.stores TO commerce_checkout_writer;
CREATE POLICY checkout_writer_store_read ON control.stores FOR SELECT TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND id=nullif(current_setting('app.store_id',true),'')::uuid);

GRANT SELECT ON storefront.carts,storefront.cart_lines,storefront.quotes,
 storefront.destination_snapshots,storefront.destination_heads,
 pricing.markets,pricing.policy_versions,pricing.policy_heads,catalog.products,catalog.skus,
 fulfillment.service_versions,fulfillment.service_heads,
 fulfillment.allocation_versions,fulfillment.allocation_warehouses,fulfillment.allocation_heads,
 fulfillment.pickup_heads,inventory.warehouses,inventory.balances TO commerce_checkout_runtime;
GRANT SELECT(tenant_id,store_id,id,kind,namespace,code,version,country,name,address,
 verification_kind,attested_at,valid_until) ON fulfillment.pickup_versions TO commerce_checkout_runtime;
GRANT SELECT ON control.stores TO commerce_checkout_runtime;
CREATE POLICY checkout_runtime_store_read ON control.stores FOR SELECT TO commerce_checkout_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND id=nullif(current_setting('app.store_id',true),'')::uuid);

-- PostgreSQL requires an UPDATE column privilege for FOR SHARE. Every such
-- checkout-runtime policy rejects even an attempted no-op UPDATE.
DO $$ DECLARE r text; BEGIN
 FOREACH r IN ARRAY ARRAY['storefront.carts','storefront.quotes','storefront.destination_snapshots',
  'storefront.destination_heads'] LOOP
  EXECUTE format('CREATE POLICY checkout_runtime_lock ON %s FOR UPDATE TO commerce_checkout_runtime
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid
    AND owner_id=nullif(current_setting(''app.buyer_id'',true),'''')::uuid)
   WITH CHECK(false)',r);
 END LOOP;
 FOREACH r IN ARRAY ARRAY['pricing.markets','pricing.policy_heads','catalog.products','catalog.skus',
  'fulfillment.pickup_heads','fulfillment.service_heads','fulfillment.allocation_heads'] LOOP
  EXECUTE format('CREATE POLICY checkout_runtime_lock ON %s FOR UPDATE TO commerce_checkout_runtime
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)
   WITH CHECK(false)',r);
 END LOOP;
END $$;
GRANT UPDATE(version) ON storefront.carts TO commerce_checkout_runtime;
GRANT UPDATE(id) ON storefront.quotes,storefront.destination_snapshots,pricing.markets,
 catalog.products,catalog.skus TO commerce_checkout_runtime;
GRANT UPDATE(current_version) ON storefront.destination_heads,pricing.policy_heads,
 fulfillment.service_heads,fulfillment.allocation_heads,fulfillment.pickup_heads TO commerce_checkout_runtime;
GRANT EXECUTE ON FUNCTION inventory.lock_balance(uuid,uuid),inventory.lock_warehouse(uuid)
 TO commerce_checkout_runtime;
GRANT EXECUTE ON FUNCTION inventory.lock_balance(uuid,uuid) TO commerce_checkout_writer;
GRANT SELECT ON checkout.command_results TO commerce_checkout_runtime;

-- The private writer locks only the existing rows it must recheck. It cannot
-- change them: FOR SHARE's required UPDATE privilege has WITH CHECK(false).
DO $$ DECLARE r text; BEGIN
 FOREACH r IN ARRAY ARRAY['storefront.carts','storefront.destination_heads'] LOOP
  EXECUTE format('CREATE POLICY checkout_writer_lock ON %s FOR UPDATE TO commerce_checkout_writer
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid
    AND owner_id=nullif(current_setting(''app.buyer_id'',true),'''')::uuid)
   WITH CHECK(false)',r);
 END LOOP;
 FOREACH r IN ARRAY ARRAY['pricing.markets','pricing.policy_heads','fulfillment.pickup_heads',
  'fulfillment.service_heads','fulfillment.allocation_heads'] LOOP
  EXECUTE format('CREATE POLICY checkout_writer_lock ON %s FOR UPDATE TO commerce_checkout_writer
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)
   WITH CHECK(false)',r);
 END LOOP;
END $$;
GRANT UPDATE(version) ON storefront.carts TO commerce_checkout_writer;
GRANT UPDATE(current_version) ON storefront.destination_heads,pricing.policy_heads,
 fulfillment.pickup_heads,fulfillment.service_heads,fulfillment.allocation_heads TO commerce_checkout_writer;
GRANT UPDATE(id) ON pricing.markets TO commerce_checkout_writer;

CREATE FUNCTION checkout.begin_hold(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,
 p_order uuid,p_snapshot jsonb,p_lines jsonb,p_job_id bigint) RETURNS jsonb
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_scope record; v_tenant uuid; v_owner uuid; v_session uuid;
 v_previous record; v_cart storefront.carts%ROWTYPE; v_quote storefront.quotes%ROWTYPE;
 v_dest storefront.destination_snapshots%ROWTYPE; v_market pricing.markets%ROWTYPE;
 v_policy pricing.policy_versions%ROWTYPE; v_service fulfillment.service_versions%ROWTYPE;
 v_allocation fulfillment.allocation_versions%ROWTYPE;
 v_source record; v_source_enabled boolean;
 v_head_version bigint; v_head_id uuid; v_line record; v_quote_line record;
 v_now timestamptz; v_expires timestamptz; v_result jsonb;
 v_count integer; v_distinct integer; v_mismatch integer;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
    OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
    OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_job_id IS NULL OR p_job_id<1
    OR p_snapshot IS NULL OR jsonb_typeof(p_snapshot)<>'object' OR octet_length(p_snapshot::text)>1048576
    OR p_lines IS NULL OR jsonb_typeof(p_lines)<>'array' OR octet_length(p_lines::text)>1048576
    OR jsonb_typeof(p_snapshot->'quote')<>'object'
    OR jsonb_typeof(p_snapshot->'destination')<>'object'
    OR jsonb_typeof(p_snapshot->'service')<>'object'
    OR jsonb_typeof(p_snapshot->'allocation')<>'object'
    OR (SELECT count(*) FROM jsonb_object_keys(p_snapshot))<>4 THEN
  RAISE EXCEPTION 'invalid checkout input' USING ERRCODE='PT400';
 END IF;
 SELECT * INTO v_scope FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 v_tenant:=v_scope.tenant_id; v_owner:=v_scope.owner_id; v_session:=v_scope.session_id;
 PERFORM set_config('app.tenant_id',v_tenant::text,true);
 PERFORM set_config('app.store_id',p_store::text,true);
 PERFORM set_config('app.buyer_id',v_owner::text,true);
 PERFORM set_config('app.buyer_session_id',v_session::text,true);
 PERFORM set_config('app.principal_id','',true);
 PERFORM pg_advisory_xact_lock(hashtextextended(
  'checkout.begin|'||v_tenant||'|'||p_store||'|'||v_owner||'|'||p_key,0));
 SELECT r.request_hash,r.response INTO v_previous FROM checkout.command_results r
  WHERE r.tenant_id=v_tenant AND r.store_id=p_store AND r.owner_id=v_owner
   AND r.operation='checkout.begin' AND r.idempotency_key=p_key;
 IF FOUND THEN
  -- Go resolves an authenticated replay before inserting the River job. A
  -- direct repeat at this writer boundary must abort any newly inserted job.
  RAISE EXCEPTION 'checkout request already recorded' USING ERRCODE='PT409';
 END IF;

 SELECT q.* INTO v_quote FROM storefront.quotes q WHERE q.tenant_id=v_tenant
  AND q.store_id=p_store AND q.owner_id=v_owner AND q.id=(p_snapshot->'quote'->>'id')::uuid;
 IF NOT FOUND OR v_quote.snapshot IS DISTINCT FROM p_snapshot->'quote' THEN
  RAISE EXCEPTION 'quote changed' USING ERRCODE='PT409'; END IF;
 SELECT c.* INTO v_cart FROM storefront.carts c WHERE c.tenant_id=v_tenant
  AND c.store_id=p_store AND c.owner_id=v_owner AND c.id=v_quote.cart_id FOR SHARE;
 IF NOT FOUND OR v_cart.version<>v_quote.cart_version OR v_cart.currency<>v_quote.currency
    OR NOT EXISTS(SELECT 1 FROM storefront.cart_lines l WHERE l.tenant_id=v_tenant
      AND l.store_id=p_store AND l.owner_id=v_owner AND l.cart_id=v_cart.id) THEN
  RAISE EXCEPTION 'cart changed' USING ERRCODE='PT409'; END IF;
 IF EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=v_tenant AND o.store_id=p_store
  AND o.owner_id=v_owner AND o.cart_id=v_cart.id AND o.cart_version=v_cart.version
  AND o.commercial_state IN ('DRAFT','AWAITING_PAYMENT','CONFIRMED')) THEN
  RAISE EXCEPTION 'active checkout exists' USING ERRCODE='PT409'; END IF;
 SELECT m.* INTO v_market FROM pricing.markets m WHERE m.tenant_id=v_tenant
  AND m.store_id=p_store AND m.id=v_quote.market_id FOR SHARE;
 IF NOT FOUND OR NOT v_market.active OR v_market.currency<>v_quote.currency
    OR v_market.version<>v_quote.market_version THEN
  RAISE EXCEPTION 'market changed' USING ERRCODE='PT409'; END IF;
 SELECT h.current_version INTO v_head_version FROM pricing.policy_heads h
  WHERE h.tenant_id=v_tenant AND h.store_id=p_store AND h.market_id=v_quote.market_id
   AND h.country=v_quote.country AND h.method=v_quote.method FOR SHARE;
 IF NOT FOUND OR v_head_version<>v_quote.policy_version THEN
  RAISE EXCEPTION 'policy changed' USING ERRCODE='PT409'; END IF;
 SELECT p.* INTO v_policy FROM pricing.policy_versions p WHERE p.tenant_id=v_tenant
  AND p.store_id=p_store AND p.market_id=v_quote.market_id AND p.country=v_quote.country
  AND p.method=v_quote.method AND p.version=v_quote.policy_version;
 IF NOT FOUND OR NOT v_policy.enabled OR v_policy.currency<>v_quote.currency
    OR p_snapshot->'quote'->'policy'->>'method'<>v_quote.method THEN
  RAISE EXCEPTION 'policy changed' USING ERRCODE='PT409'; END IF;

 SELECT d.* INTO v_dest FROM storefront.destination_snapshots d WHERE d.tenant_id=v_tenant
  AND d.store_id=p_store AND d.owner_id=v_owner AND d.id=(p_snapshot->'destination'->>'id')::uuid;
 IF NOT FOUND OR v_dest.cart_id<>v_cart.id OR v_dest.cart_version<>v_cart.version
    OR v_dest.country<>v_quote.country OR v_dest.kind IS DISTINCT FROM p_snapshot->'destination'->>'kind'
    OR p_snapshot->'destination'->>'cart_id' IS DISTINCT FROM v_cart.id::text
    OR p_snapshot->'destination'->>'country' IS DISTINCT FROM v_dest.country
    OR p_snapshot->'destination'->>'recipient_name' IS DISTINCT FROM v_dest.recipient_name
    OR p_snapshot->'destination'->>'phone' IS DISTINCT FROM v_dest.phone
    OR p_snapshot->'destination'->'home_address' IS DISTINCT FROM
     jsonb_build_object('region',v_dest.region,'city',v_dest.city,'postal_code',v_dest.postal_code,
      'line1',v_dest.line1,'line2',v_dest.line2) THEN
  RAISE EXCEPTION 'destination changed' USING ERRCODE='PT409'; END IF;
 IF v_dest.pickup_id IS NOT NULL THEN
  SELECT p.id,p.kind,p.namespace,p.code,p.version,p.country,p.verification_kind,
   p.attested_at,p.valid_until INTO v_source FROM fulfillment.pickup_versions p WHERE p.tenant_id=v_tenant
   AND p.store_id=p_store AND p.id=v_dest.pickup_id;
  IF NOT FOUND OR v_source.kind<>v_dest.kind OR v_source.country<>v_dest.country
     OR v_source.verification_kind<>'MANUAL_ATTESTED'
     OR p_snapshot->'destination'->'pickup'->>'id' IS DISTINCT FROM v_source.id::text THEN
   RAISE EXCEPTION 'pickup source changed' USING ERRCODE='PT409'; END IF;
  SELECT h.pickup_id,h.current_version,h.enabled INTO v_head_id,v_head_version,v_source_enabled
   FROM fulfillment.pickup_heads h WHERE h.tenant_id=v_tenant AND h.store_id=p_store
    AND h.kind=v_source.kind AND h.namespace=v_source.namespace AND h.code=v_source.code FOR SHARE;
  IF NOT FOUND OR NOT v_source_enabled OR v_head_id<>v_source.id OR v_head_version<>v_source.version THEN
   RAISE EXCEPTION 'pickup source changed' USING ERRCODE='PT409'; END IF;
 ELSIF v_dest.kind<>'home' THEN
  RAISE EXCEPTION 'pickup source missing' USING ERRCODE='PT409';
 END IF;
 SELECT h.destination_id,h.current_version INTO v_head_id,v_head_version
  FROM storefront.destination_heads h WHERE h.tenant_id=v_tenant AND h.store_id=p_store
   AND h.owner_id=v_owner AND h.cart_id=v_cart.id FOR SHARE;
 IF NOT FOUND OR v_head_id<>v_dest.id OR v_head_version<>v_dest.version
    OR (p_snapshot->'destination'->>'version')::bigint IS DISTINCT FROM v_dest.version THEN
  RAISE EXCEPTION 'destination changed' USING ERRCODE='PT409'; END IF;

 IF v_quote.method NOT LIKE 'delivery:%' THEN
  RAISE EXCEPTION 'delivery service missing' USING ERRCODE='PT409'; END IF;
 SELECT h.current_version INTO v_head_version FROM fulfillment.service_heads h
  WHERE h.tenant_id=v_tenant AND h.store_id=p_store AND h.market_id=v_quote.market_id
   AND h.country=v_quote.country AND h.code=substring(v_quote.method FROM 10) FOR SHARE;
 IF NOT FOUND OR v_head_version IS DISTINCT FROM (p_snapshot->'service'->>'version')::bigint THEN
  RAISE EXCEPTION 'service changed' USING ERRCODE='PT409'; END IF;
 SELECT s.* INTO v_service FROM fulfillment.service_versions s WHERE s.tenant_id=v_tenant
  AND s.store_id=p_store AND s.market_id=v_quote.market_id AND s.country=v_quote.country
  AND s.code=substring(v_quote.method FROM 10) AND s.version=v_head_version;
 IF NOT FOUND OR NOT v_service.enabled OR NOT v_service.visible OR v_service.mode<>'MANUAL'
    OR v_service.delivery_kind<>v_dest.kind OR v_service.currency<>v_quote.currency
    OR v_service.policy_version<>v_quote.policy_version
    OR p_snapshot->'service'->>'code' IS DISTINCT FROM v_service.code
    OR p_snapshot->'service'->>'market_id' IS DISTINCT FROM v_service.market_id::text
    OR p_snapshot->'service'->>'country' IS DISTINCT FROM v_service.country THEN
  RAISE EXCEPTION 'service changed' USING ERRCODE='PT409'; END IF;
 SELECT h.current_version INTO v_head_version FROM fulfillment.allocation_heads h
  WHERE h.tenant_id=v_tenant AND h.store_id=p_store AND h.market_id=v_quote.market_id
   AND h.country=v_quote.country AND h.code=v_service.code FOR SHARE;
 IF NOT FOUND OR v_head_version IS DISTINCT FROM (p_snapshot->'allocation'->>'version')::bigint THEN
  RAISE EXCEPTION 'allocation changed' USING ERRCODE='PT409'; END IF;
 SELECT a.* INTO v_allocation FROM fulfillment.allocation_versions a WHERE a.tenant_id=v_tenant
  AND a.store_id=p_store AND a.market_id=v_quote.market_id AND a.country=v_quote.country
  AND a.code=v_service.code AND a.version=v_head_version;
 IF NOT FOUND OR v_allocation.warehouse_count<1
    OR p_snapshot->'allocation'->>'market_id' IS DISTINCT FROM v_allocation.market_id::text
    OR p_snapshot->'allocation'->>'country' IS DISTINCT FROM v_allocation.country
    OR p_snapshot->'allocation'->>'code' IS DISTINCT FROM v_allocation.code
    OR p_snapshot->'allocation'->'warehouse_ids' IS DISTINCT FROM
     (SELECT jsonb_agg(a.warehouse_id::text ORDER BY a.position)
       FROM fulfillment.allocation_warehouses a WHERE a.tenant_id=v_tenant AND a.store_id=p_store
        AND a.market_id=v_allocation.market_id AND a.country=v_allocation.country
        AND a.code=v_allocation.code AND a.version=v_allocation.version) THEN
  RAISE EXCEPTION 'allocation unavailable' USING ERRCODE='PT409'; END IF;

 -- Structural plan and exact SKU conservation only. Go's locked, current
 -- price calculation remains the one monetary authority.
 SELECT count(*),count(DISTINCT (l.warehouse_id,l.sku_id)) INTO v_count,v_distinct
  FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint);
 IF v_count NOT BETWEEN 1 AND 800 OR v_count<>v_distinct THEN
  RAISE EXCEPTION 'invalid stock plan' USING ERRCODE='PT400'; END IF;
 SELECT count(*),count(DISTINCT q.sku_id) INTO v_count,v_distinct
  FROM jsonb_to_recordset(v_quote.snapshot->'lines') AS q(sku_id uuid,quantity bigint);
 IF v_count NOT BETWEEN 1 AND 50 OR v_count<>v_distinct THEN
  RAISE EXCEPTION 'invalid quote lines' USING ERRCODE='PT400'; END IF;
 SELECT count(*) INTO v_mismatch FROM (
  WITH demand AS (SELECT q.sku_id,sum(q.quantity) AS quantity
    FROM jsonb_to_recordset(v_quote.snapshot->'lines') AS q(sku_id uuid,quantity bigint)
    GROUP BY q.sku_id),
   plan AS (SELECT l.sku_id,sum(l.quantity) AS quantity
    FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint)
    GROUP BY l.sku_id)
  SELECT 1 FROM demand d FULL JOIN plan p USING(sku_id)
   WHERE d.sku_id IS NULL OR p.sku_id IS NULL OR d.quantity IS DISTINCT FROM p.quantity) mismatch;
 IF v_mismatch<>0 THEN RAISE EXCEPTION 'stock plan differs from quote' USING ERRCODE='PT409'; END IF;
 FOR v_line IN SELECT l.warehouse_id,l.sku_id,l.quantity
  FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint)
  ORDER BY l.warehouse_id,l.sku_id LOOP
  IF v_line.warehouse_id IS NULL OR v_line.sku_id IS NULL
     OR v_line.quantity IS NULL OR v_line.quantity NOT BETWEEN 1 AND 1000000000
     OR NOT EXISTS(SELECT 1 FROM fulfillment.allocation_warehouses a WHERE a.tenant_id=v_tenant
      AND a.store_id=p_store AND a.market_id=v_quote.market_id AND a.country=v_quote.country
      AND a.code=v_service.code AND a.version=v_allocation.version
      AND a.warehouse_id=v_line.warehouse_id)
     OR NOT EXISTS(SELECT 1 FROM inventory.warehouses w WHERE w.tenant_id=v_tenant
      AND w.store_id=p_store AND w.id=v_line.warehouse_id AND w.active) THEN
   RAISE EXCEPTION 'invalid stock plan' USING ERRCODE='PT409'; END IF;
  IF NOT EXISTS(SELECT 1 FROM inventory.balances b WHERE b.tenant_id=v_tenant
    AND b.store_id=p_store AND b.warehouse_id=v_line.warehouse_id AND b.sku_id=v_line.sku_id
    AND b.on_hand-b.reserved-b.allocated-b.unavailable>=v_line.quantity) THEN
   RAISE EXCEPTION 'insufficient stock' USING ERRCODE='PT402'; END IF;
 END LOOP;
 FOR v_quote_line IN SELECT q.sku_id,q.product_id,q.quantity,q.unit_price_minor
  FROM jsonb_to_recordset(v_quote.snapshot->'lines')
   AS q(sku_id uuid,product_id uuid,quantity bigint,unit_price_minor bigint) LOOP
  IF v_quote_line.sku_id IS NULL OR v_quote_line.product_id IS NULL
     OR v_quote_line.quantity IS NULL OR v_quote_line.quantity NOT BETWEEN 1 AND 1000000000
     OR v_quote_line.unit_price_minor IS NULL OR v_quote_line.unit_price_minor NOT BETWEEN 0 AND 1000000000000 THEN
   RAISE EXCEPTION 'invalid quote lines' USING ERRCODE='PT400'; END IF;
 END LOOP;

 -- Final time follows every row and advisory wait. An expiry job is a
 -- scheduled housekeeping task, never a runnable payment attempt.
 v_now:=clock_timestamp();
 SELECT * INTO v_scope FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND OR v_scope.tenant_id<>v_tenant OR v_scope.owner_id<>v_owner
    OR v_scope.session_id<>v_session THEN
  RAISE EXCEPTION 'buyer capability expired' USING ERRCODE='PT401'; END IF;
 IF v_quote.created_at>v_now OR v_quote.expires_at<=v_now
    OR v_dest.selected_at>v_now OR v_dest.expires_at<=v_now
    OR (v_dest.pickup_id IS NOT NULL AND (v_source.attested_at>v_dest.selected_at
      OR v_source.attested_at>v_now OR v_source.valid_until<=v_now
      OR v_dest.expires_at>v_source.valid_until)) THEN
  RAISE EXCEPTION 'checkout source expired' USING ERRCODE='PT409'; END IF;
 IF NOT EXISTS(SELECT 1 FROM river.river_job j WHERE j.id=p_job_id
  AND j.kind='checkout_expiry_v1' AND j.args->>'order_id'=p_order::text
  AND j.args->>'generation'='1' AND j.args->>'version'='1') THEN
  RAISE EXCEPTION 'expiry job missing' USING ERRCODE='PT409'; END IF;
 v_expires:=v_now+interval '15 minutes';
 v_result:=jsonb_build_object('order_id',p_order::text,'reservation_id',p_order::text,
  'generation',1,'expires_at',v_expires,'job_id',p_job_id);
 INSERT INTO checkout.orders(tenant_id,store_id,owner_id,id,creator_session_id,cart_id,cart_version,
  quote_id,destination_id,market_id,country,service_code,service_version,allocation_version,
  currency,total_minor,commercial_state,fulfillment_state,generation,expires_at,job_id,snapshot,created_at,updated_at)
 VALUES(v_tenant,p_store,v_owner,p_order,v_session,v_cart.id,v_cart.version,v_quote.id,v_dest.id,
  v_quote.market_id,v_quote.country,v_service.code,v_service.version,v_allocation.version,
  v_quote.currency,(v_quote.snapshot->'amount'->>'total_minor')::bigint,
  'DRAFT','MANUAL_UNASSIGNED',1,v_expires,p_job_id,p_snapshot,v_now,v_now);
 INSERT INTO inventory.reservations(tenant_id,store_id,id,state,expires_at,
  checkout_id,buyer_owner_id,buyer_session_id,generation)
 VALUES(v_tenant,p_store,p_order,'HELD',v_expires,p_order,v_owner,v_session,1);
 FOR v_line IN SELECT l.warehouse_id,l.sku_id,l.quantity
  FROM jsonb_to_recordset(p_lines) AS l(warehouse_id uuid,sku_id uuid,quantity bigint)
  ORDER BY l.warehouse_id,l.sku_id LOOP
  INSERT INTO inventory.reservation_lines(tenant_id,store_id,reservation_id,warehouse_id,sku_id,quantity)
   VALUES(v_tenant,p_store,p_order,v_line.warehouse_id,v_line.sku_id,v_line.quantity);
  INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,
   operation,command_key,reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
   VALUES(v_tenant,p_store,v_line.warehouse_id,v_line.sku_id,'RESERVE',v_line.quantity,
    'checkout.begin',p_order::text,p_order,NULL,p_order,v_owner,v_session,'BUYER');
 END LOOP;
 INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
 VALUES(v_tenant,p_store,v_owner,p_order,v_session,1,'checkout.held','BUYER');
 INSERT INTO checkout.command_results(tenant_id,store_id,owner_id,operation,idempotency_key,
  creator_session_id,request_hash,order_id,response)
 VALUES(v_tenant,p_store,v_owner,'checkout.begin',p_key,v_session,p_request_hash,p_order,v_result);
 RETURN v_result;
EXCEPTION WHEN invalid_text_representation OR numeric_value_out_of_range
 OR invalid_datetime_format THEN
 RAISE EXCEPTION 'invalid checkout input' USING ERRCODE='PT400';
END $$;

CREATE FUNCTION checkout.expire_held(p_order uuid,p_generation bigint)
 RETURNS TABLE(disposition text,retry_at timestamptz)
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_identity record; v_order checkout.orders%ROWTYPE; v_reservation inventory.reservations%ROWTYPE;
 v_line record; v_now timestamptz;
BEGIN
 IF p_order IS NULL OR p_generation IS NULL OR p_generation<1 THEN
  RAISE EXCEPTION 'invalid expiry input' USING ERRCODE='PT400'; END IF;
 -- A UUID is globally unique, so the fixed worker entrypoint may locate the
 -- durable scope before setting any caller-supplied GUCs.
 SELECT o.tenant_id,o.store_id,o.owner_id,o.creator_session_id INTO v_identity
  FROM checkout.orders o WHERE o.id=p_order;
 IF NOT FOUND THEN RETURN QUERY SELECT 'STALE'::text,NULL::timestamptz; RETURN; END IF;
 PERFORM set_config('app.tenant_id',v_identity.tenant_id::text,true);
 PERFORM set_config('app.store_id',v_identity.store_id::text,true);
 PERFORM set_config('app.buyer_id',v_identity.owner_id::text,true);
 PERFORM set_config('app.buyer_session_id',v_identity.creator_session_id::text,true);
 PERFORM set_config('app.principal_id','',true);
 SELECT o.* INTO v_order FROM checkout.orders o WHERE o.id=p_order FOR UPDATE;
 IF NOT FOUND OR v_order.generation<>p_generation OR v_order.commercial_state<>'DRAFT' THEN
  RETURN QUERY SELECT 'STALE'::text,NULL::timestamptz; RETURN; END IF;
 SELECT r.* INTO v_reservation FROM inventory.reservations r
  WHERE r.tenant_id=v_order.tenant_id AND r.store_id=v_order.store_id AND r.id=p_order FOR UPDATE;
 IF NOT FOUND OR v_reservation.checkout_id<>p_order OR v_reservation.buyer_owner_id<>v_order.owner_id
    OR v_reservation.buyer_session_id<>v_order.creator_session_id
    OR v_reservation.generation<>p_generation OR v_reservation.state<>'HELD' THEN
  RETURN QUERY SELECT 'STALE'::text,NULL::timestamptz; RETURN; END IF;
 -- Balance locks are globally sorted; ledger inserts use the same order.
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
  INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,
   operation,command_key,reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
   VALUES(v_order.tenant_id,v_order.store_id,v_line.warehouse_id,v_line.sku_id,'RELEASE',-v_line.quantity,
    'checkout.expire',p_order::text,p_order,NULL,p_order,v_order.owner_id,v_order.creator_session_id,'SYSTEM_EXPIRY');
 END LOOP;
 UPDATE inventory.reservations SET state='EXPIRED' WHERE tenant_id=v_order.tenant_id
  AND store_id=v_order.store_id AND id=p_order AND state='HELD';
 UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED',updated_at=v_now
  WHERE id=p_order AND generation=p_generation AND commercial_state='DRAFT';
 INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
 VALUES(v_order.tenant_id,v_order.store_id,v_order.owner_id,p_order,v_order.creator_session_id,
  p_generation,'checkout.expired','SYSTEM_EXPIRY');
 RETURN QUERY SELECT 'EXPIRED'::text,NULL::timestamptz;
END $$;

ALTER FUNCTION checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint)
 OWNER TO commerce_checkout_writer;
ALTER FUNCTION checkout.expire_held(uuid,bigint) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint),
 checkout.expire_held(uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint)
 TO commerce_checkout_runtime;
GRANT EXECUTE ON FUNCTION checkout.expire_held(uuid,bigint) TO commerce_worker;
