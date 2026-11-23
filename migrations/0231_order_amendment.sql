-- Purpose: Add transactional pre-dispatch amendments and revision-checked fulfillment for home COD.
-- Depends on: the existing checkout, inventory, identity and manual fulfillment authorities.
-- Used by: internal/merchantorders amendments and shipment recording.

DO $$
DECLARE d text; permissions text[];
BEGIN
 SELECT pg_get_constraintdef(oid) INTO STRICT d FROM pg_constraint
  WHERE conrelid='identity.store_grants'::regclass AND conname='store_grants_permission_check';
 SELECT array_agg(m[1]) INTO permissions FROM regexp_matches(d,'''([a-z_]+:[a-z_]+)''::text','g') m;
 IF permissions IS NULL OR NOT 'orders:read'=ANY(permissions) OR 'orders:amend'=ANY(permissions) THEN
  RAISE EXCEPTION 'R03 permission vocabulary drift'; END IF;
 ALTER TABLE identity.store_grants DROP CONSTRAINT store_grants_permission_check;
 EXECUTE format('ALTER TABLE identity.store_grants ADD CONSTRAINT store_grants_permission_check CHECK (permission IN (%s))',
  (SELECT string_agg(quote_literal(p),',') FROM unnest(permissions||ARRAY['orders:amend']) p));
END $$;

-- Preserve the effective baseline bundles, including role-specific PII and inventory rules.
-- Only the catalogue input of the existing role function excludes the explicit R03 grant.
DO $$
DECLARE d text; catalogue constant text := 'identity.staff_permission_catalogue()';
BEGIN
 d:=pg_get_functiondef('identity.staff_role_permissions(text)'::regprocedure);
 IF (length(d)-length(replace(d,catalogue,'')))<>3*length(catalogue) THEN
  RAISE EXCEPTION 'R03 staff role bundle baseline drift'; END IF;
 EXECUTE replace(d,catalogue,'array_remove('||catalogue||',''orders:amend'')');
END $$;
COMMENT ON FUNCTION identity.staff_role_permissions(text) IS
 'Default store role bundles preserve the live-console role rules. orders:amend requires an explicit grant (R03).';

ALTER TABLE checkout.orders ADD COLUMN order_revision bigint NOT NULL DEFAULT 1 CHECK(order_revision>0);
GRANT SELECT(order_revision) ON checkout.orders TO commerce_auth;

-- Only the amendment persistence helper can update frozen order contents.
CREATE ROLE commerce_order_amendment_writer NOLOGIN NOINHERIT NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT USAGE ON SCHEMA checkout TO commerce_order_amendment_writer;
GRANT SELECT(tenant_id,store_id,owner_id,id,order_revision), UPDATE(order_revision,snapshot,total_minor,updated_at)
 ON checkout.orders TO commerce_order_amendment_writer;
CREATE POLICY order_amendment_writer ON checkout.orders TO commerce_order_amendment_writer
 USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid)
 WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid);
CREATE FUNCTION checkout.persist_order_amendment(p_tenant uuid,p_store uuid,p_owner uuid,p_order uuid,
 p_revision bigint,p_snapshot jsonb,p_total bigint)
RETURNS boolean LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 WITH modified AS (
  UPDATE checkout.orders k SET order_revision=p_revision+1,snapshot=p_snapshot,total_minor=p_total,updated_at=clock_timestamp()
  WHERE (k.tenant_id,k.store_id,k.owner_id,k.id,k.order_revision)=(p_tenant,p_store,p_owner,p_order,p_revision)
  RETURNING 1
 )
 SELECT EXISTS(SELECT 1 FROM modified)
$$;
ALTER FUNCTION checkout.persist_order_amendment(uuid,uuid,uuid,uuid,bigint,jsonb,bigint) OWNER TO commerce_order_amendment_writer;
REVOKE ALL ON FUNCTION checkout.persist_order_amendment(uuid,uuid,uuid,uuid,bigint,jsonb,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.persist_order_amendment(uuid,uuid,uuid,uuid,bigint,jsonb,bigint) TO commerce_checkout_writer;
COMMENT ON FUNCTION checkout.persist_order_amendment(uuid,uuid,uuid,uuid,bigint,jsonb,bigint) IS
 'Internal scoped write of the prepared R03 snapshot under the existing order lock. Only the checkout definer may execute.';

-- Like the order-referenced checkout destination (U08), revision snapshots retain the
-- order's legal record. No runtime role can query the historical recipient payload.
CREATE TABLE checkout.order_revisions (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, owner_id uuid NOT NULL, order_id uuid NOT NULL,
 revision bigint NOT NULL CHECK(revision>0), total_minor bigint NOT NULL,
 snapshot jsonb NOT NULL CHECK(jsonb_typeof(snapshot)='object' AND octet_length(snapshot::text)<=1048576),
 PRIMARY KEY(tenant_id,store_id,order_id,revision),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id)
);
ALTER TABLE checkout.order_revisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkout.order_revisions FORCE ROW LEVEL SECURITY;
CREATE POLICY order_revision_insert ON checkout.order_revisions FOR INSERT TO commerce_checkout_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid);
GRANT INSERT ON checkout.order_revisions TO commerce_checkout_writer;

CREATE FUNCTION checkout.amend_authority(p_hash bytea,p_store uuid,p_add boolean)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SET search_path=pg_catalog AS $$
DECLARE s record; extra record; permission text;
BEGIN
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' OR NOT EXISTS(SELECT 1 FROM identity.sessions se
  WHERE se.token_hash=p_hash AND se.audience='merchant' AND se.revoked_at IS NULL AND se.expires_at>clock_timestamp()) THEN
  RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 FOREACH permission IN ARRAY CASE WHEN p_add THEN ARRAY['orders:amend','catalog:read','inventory:reserve']
   ELSE ARRAY['orders:amend'] END LOOP
  SELECT * INTO extra FROM identity.resolve_access(p_hash,p_store,permission);
  IF extra.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
  IF extra.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
  IF extra.access_status IS DISTINCT FROM 'ok' OR extra.principal_id IS DISTINCT FROM s.principal_id
    OR extra.tenant_id IS DISTINCT FROM s.tenant_id OR extra.authz_revision IS DISTINCT FROM s.authz_revision THEN
   RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 END LOOP;
 RETURN jsonb_build_object('tenant',s.tenant_id,'principal',s.principal_id,'revision',s.authz_revision);
END $$;
ALTER FUNCTION checkout.amend_authority(bytea,uuid,boolean) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.amend_authority(bytea,uuid,boolean) FROM PUBLIC;

CREATE FUNCTION checkout.amend_recipient(p_destination jsonb,p_patch jsonb)
RETURNS jsonb LANGUAGE plpgsql IMMUTABLE SET search_path=pg_catalog AS $$
DECLARE field text; value text; max_length integer;
BEGIN
 IF p_patch IS NULL THEN RETURN p_destination; END IF;
 IF jsonb_typeof(p_patch) IS DISTINCT FROM 'object' OR p_patch='{}'::jsonb THEN
  RAISE EXCEPTION 'invalid amendment' USING ERRCODE='PT400'; END IF;
 FOR field IN SELECT jsonb_object_keys(p_patch) LOOP
  max_length:=CASE field WHEN 'recipient_name' THEN 120 WHEN 'phone' THEN 32
   WHEN 'region' THEN 100 WHEN 'city' THEN 100 WHEN 'postal_code' THEN 20 WHEN 'line1' THEN 200 WHEN 'line2' THEN 200 END;
  value:=p_patch->>field;
  IF max_length IS NULL OR jsonb_typeof(p_patch->field) IS DISTINCT FROM 'string' OR length(value)>max_length
   OR value<>btrim(value) OR value~'[[:cntrl:]]' OR (field IN ('recipient_name','phone','city','line1') AND value='')
   OR (field='phone' AND (value !~ '^[+() 0-9-]+$' OR length(regexp_replace(value,'[^0-9]','','g')) NOT BETWEEN 6 AND 20)) THEN
   RAISE EXCEPTION 'invalid amendment' USING ERRCODE='PT400'; END IF;
  p_destination:=jsonb_set(p_destination,CASE WHEN field IN ('recipient_name','phone') THEN ARRAY[field]
   ELSE ARRAY['home_address',field] END,to_jsonb(value));
 END LOOP;
 RETURN p_destination;
END $$;
ALTER FUNCTION checkout.amend_recipient(jsonb,jsonb) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.amend_recipient(jsonb,jsonb) FROM PUBLIC;

CREATE FUNCTION checkout.amend_tax(base bigint,rate bigint,mode text) RETURNS bigint
LANGUAGE sql IMMUTABLE SET search_path=pg_catalog AS $$
 SELECT CASE mode WHEN 'inclusive' THEN base-(base*10000+(10000+rate)/2)/(10000+rate)
  WHEN 'exclusive' THEN (base*rate+5000)/10000 ELSE 0 END
$$;
ALTER FUNCTION checkout.amend_tax(bigint,bigint,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.amend_tax(bigint,bigint,text) FROM PUBLIC;

CREATE FUNCTION checkout.amend_price(o checkout.orders,additions jsonb,locking boolean) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SET search_path=pg_catalog AS $$
DECLARE q jsonb:=o.snapshot->'quote'; old_lines jsonb:=q->'lines'; lines jsonb:=old_lines; amounts jsonb:='[]';
 a jsonb; line jsonb; ids uuid[]:='{}'; sku uuid; qty bigint; row record; policy record;
 subtotal bigint:=0; shipping bigint; tax bigint:=0; shipping_tax bigint; line_subtotal bigint; line_tax bigint; total bigint;
BEGIN
 IF (q#>>'{amount,discount_minor}')::bigint IS DISTINCT FROM 0::bigint OR
  jsonb_array_length(old_lines)+jsonb_array_length(additions)>50 THEN RETURN NULL; END IF;
 SELECT p.* INTO policy FROM fulfillment.service_versions s JOIN pricing.policy_versions p
  ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.market_id=s.market_id AND p.country=s.country
  AND p.method=s.policy_method AND p.version=s.policy_version AND p.currency=s.currency
  WHERE s.tenant_id=o.tenant_id AND s.store_id=o.store_id AND s.market_id=o.market_id AND s.country=o.country
   AND s.code=o.service_code AND s.version=o.service_version AND s.delivery_kind='home' AND s.mode='MANUAL'
   AND s.currency=o.currency;
 IF NOT FOUND OR policy.shipping_mode<>'country_flat' THEN RETURN NULL; END IF;
 -- Quote the existing lines against the frozen service policy before extending it.
 FOR line IN SELECT value FROM jsonb_array_elements(old_lines) LOOP
  line_subtotal:=(line->>'unit_price_minor')::bigint*(line->>'quantity')::bigint;
  line_tax:=checkout.amend_tax(line_subtotal,policy.tax_rate_bps,policy.tax_mode);
  IF (line#>>'{amount,subtotal_minor}')::bigint IS DISTINCT FROM line_subtotal
   OR (line#>>'{amount,tax_minor}')::bigint IS DISTINCT FROM line_tax THEN RETURN NULL; END IF;
  subtotal:=subtotal+line_subtotal;
  tax:=tax+line_tax;
  amounts:=amounts||jsonb_build_array(line->'amount');
 END LOOP;
 shipping:=CASE WHEN subtotal>=policy.free_shipping_threshold_minor THEN 0 ELSE policy.shipping_minor END;
 shipping_tax:=CASE WHEN policy.tax_basis='goods_and_shipping'
  THEN checkout.amend_tax(shipping,policy.tax_rate_bps,policy.tax_mode) ELSE 0 END;
 IF shipping IS DISTINCT FROM (q#>>'{amount,shipping_minor}')::bigint
  OR shipping_tax IS DISTINCT FROM (q#>>'{amount,shipping_tax_minor}')::bigint
  OR tax+shipping_tax IS DISTINCT FROM (q#>>'{amount,tax_minor}')::bigint
  OR subtotal+shipping+(CASE WHEN policy.tax_mode='exclusive' THEN tax+shipping_tax ELSE 0 END)<>o.total_minor THEN
  RETURN NULL;
 END IF;
 FOR a IN SELECT value FROM jsonb_array_elements(additions) LOOP
  IF jsonb_typeof(a) IS DISTINCT FROM 'object' OR a-'sku_id'-'quantity'<>'{}'::jsonb
   OR coalesce(a->>'sku_id','') !~ '^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$'
   OR coalesce(a->>'quantity','') !~ '^[1-9][0-9]{0,8}$' OR jsonb_typeof(a->'quantity') IS DISTINCT FROM 'number' THEN
   RAISE EXCEPTION 'invalid amendment' USING ERRCODE='PT400'; END IF;
  sku:=(a->>'sku_id')::uuid;
  IF sku=ANY(ids) THEN RAISE EXCEPTION 'duplicate amendment item' USING ERRCODE='PT400'; END IF;
  ids:=array_append(ids,sku);
 END LOOP;
 IF locking THEN
  PERFORM p.id FROM catalog.products p WHERE p.tenant_id=o.tenant_id AND p.store_id=o.store_id AND p.id IN
   (SELECT s.product_id FROM catalog.skus s WHERE s.tenant_id=o.tenant_id AND s.store_id=o.store_id AND s.id=ANY(ids))
   ORDER BY p.id FOR SHARE;
  PERFORM s.id FROM catalog.skus s WHERE s.tenant_id=o.tenant_id AND s.store_id=o.store_id AND s.id=ANY(ids)
   ORDER BY s.id FOR SHARE;
 END IF;
 FOR a IN SELECT value FROM jsonb_array_elements(additions) WITH ORDINALITY q(value,position) ORDER BY position LOOP
  sku:=(a->>'sku_id')::uuid; qty:=(a->>'quantity')::bigint;
  SELECT s.price_minor,s.version,s.code,s.inventory_tracked,s.max_per_order,p.name,p.version AS product_version
   INTO row FROM catalog.skus s JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
   WHERE s.tenant_id=o.tenant_id AND s.store_id=o.store_id AND s.id=sku AND s.status='active' AND p.status='active'
    AND s.currency=o.currency;
  IF NOT FOUND OR row.price_minor::numeric*qty>1000000000000 OR (NOT row.inventory_tracked AND
    (row.max_per_order IS NULL OR qty+coalesce((SELECT sum((x->>'quantity')::bigint)
       FROM jsonb_array_elements(old_lines) x WHERE x->>'sku_id'=sku::text),0)>row.max_per_order)) THEN RETURN NULL; END IF;
  line_subtotal:=row.price_minor*qty;
  line_tax:=checkout.amend_tax(line_subtotal,policy.tax_rate_bps,policy.tax_mode);
  subtotal:=subtotal+line_subtotal; tax:=tax+line_tax;
  line:=jsonb_build_object('subtotal_minor',line_subtotal,'discount_minor',0,'tax_minor',line_tax,
   'total_minor',line_subtotal+CASE WHEN policy.tax_mode='exclusive' THEN line_tax ELSE 0 END);
  amounts:=amounts||jsonb_build_array(line);
  lines:=lines||jsonb_build_array(jsonb_build_object('sku_id',sku,'code',row.code,'name',row.name,
   'quantity',qty,'unit_price_minor',row.price_minor,'amount',line,'inventory_tracked',row.inventory_tracked,
   'price_source','r03_catalog','sku_version',row.version,'product_version',row.product_version));
 END LOOP;
 shipping:=CASE WHEN subtotal>=policy.free_shipping_threshold_minor THEN 0 ELSE policy.shipping_minor END;
 shipping_tax:=CASE WHEN policy.tax_basis='goods_and_shipping'
  THEN checkout.amend_tax(shipping,policy.tax_rate_bps,policy.tax_mode) ELSE 0 END;
 tax:=tax+shipping_tax;
 total:=subtotal+shipping+CASE WHEN policy.tax_mode='exclusive' THEN tax ELSE 0 END;
 RETURN q||jsonb_build_object('lines',lines,'amount',jsonb_build_object('subtotal_minor',subtotal,
  'discount_minor',0,'shipping_minor',shipping,'shipping_tax_minor',shipping_tax,
  'tax_minor',tax,'total_minor',total,'lines',amounts));
END $$;
ALTER FUNCTION checkout.amend_price(checkout.orders,jsonb,boolean) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.amend_price(checkout.orders,jsonb,boolean) FROM PUBLIC;

CREATE FUNCTION inventory.amend_committed_allocation(o checkout.orders, added_lines jsonb,p_key text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SET search_path=pg_catalog AS $$
DECLARE warehouse record; entry record; added jsonb; planned jsonb:='[]'; stocks jsonb:='[]';
 warehouse_ids uuid[]:='{}'; sku_ids uuid[]; available bigint; remaining bigint; units bigint;
 principal uuid:=nullif(current_setting('app.principal_id',true),'')::uuid;
BEGIN
 SELECT array_agg((x->>'sku_id')::uuid ORDER BY (x->>'sku_id')::uuid) INTO sku_ids
  FROM jsonb_array_elements(added_lines) x WHERE (x->>'inventory_tracked')::boolean;
 IF sku_ids IS NULL THEN RETURN true; END IF;
 PERFORM 1 FROM inventory.reservations r WHERE r.tenant_id=o.tenant_id AND r.store_id=o.store_id
  AND r.id=o.id AND r.state='COMMITTED' AND r.buyer_owner_id=o.owner_id FOR UPDATE;
 IF NOT FOUND THEN RETURN false; END IF;
 FOR warehouse IN SELECT w.warehouse_id FROM fulfillment.allocation_warehouses w
  WHERE w.tenant_id=o.tenant_id AND w.store_id=o.store_id AND w.market_id=o.market_id
   AND w.country=o.country AND w.code=o.service_code AND w.version=o.allocation_version ORDER BY w.warehouse_id LOOP
  IF EXISTS(SELECT 1 FROM inventory.lock_warehouse(warehouse.warehouse_id) w WHERE w.active) THEN
   warehouse_ids:=array_append(warehouse_ids,warehouse.warehouse_id);
  END IF;
 END LOOP;
 FOR entry IN SELECT w warehouse_id,s sku_id FROM unnest(warehouse_ids) w CROSS JOIN unnest(sku_ids) s ORDER BY w,s LOOP
  SELECT b.on_hand-b.reserved-b.allocated-b.unavailable INTO available FROM inventory.lock_balance(entry.warehouse_id,entry.sku_id) b;
  IF available>0 THEN stocks:=stocks||jsonb_build_array(jsonb_build_object('warehouse_id',entry.warehouse_id,
   'sku_id',entry.sku_id,'available',available)); END IF;
 END LOOP;
 FOR added IN SELECT value FROM jsonb_array_elements(added_lines) WHERE (value->>'inventory_tracked')::boolean LOOP
  remaining:=(added->>'quantity')::bigint;
  FOR entry IN SELECT w.warehouse_id,(b->>'available')::bigint AS available FROM fulfillment.allocation_warehouses w
   JOIN jsonb_array_elements(stocks) b ON (b->>'warehouse_id')::uuid=w.warehouse_id AND b->>'sku_id'=added->>'sku_id'
   WHERE w.tenant_id=o.tenant_id AND w.store_id=o.store_id AND w.market_id=o.market_id
    AND w.country=o.country AND w.code=o.service_code AND w.version=o.allocation_version ORDER BY w.position LOOP
   units:=least(entry.available,remaining);
   planned:=planned||jsonb_build_array(jsonb_build_object('warehouse_id',entry.warehouse_id,
    'sku_id',added->'sku_id','quantity',units));
   remaining:=remaining-units;
   EXIT WHEN remaining=0;
  END LOOP;
  IF remaining<>0 THEN RETURN false; END IF;
 END LOOP;
 FOR entry IN SELECT * FROM jsonb_to_recordset(planned) AS p(warehouse_id uuid,sku_id uuid,quantity bigint)
  ORDER BY warehouse_id,sku_id LOOP
  INSERT INTO inventory.reservation_lines(tenant_id,store_id,reservation_id,warehouse_id,sku_id,quantity)
   VALUES(o.tenant_id,o.store_id,o.id,entry.warehouse_id,entry.sku_id,entry.quantity)
   ON CONFLICT(tenant_id,store_id,reservation_id,warehouse_id,sku_id)
   DO UPDATE SET quantity=reservation_lines.quantity+excluded.quantity;
  INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_allocated,operation,command_key,reservation_id,
   principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind,creator_kind,create_attempt_id)
   VALUES(o.tenant_id,o.store_id,entry.warehouse_id,entry.sku_id,'ALLOCATE',entry.quantity,'checkout.order_amendment',p_key,o.id,
    principal,o.id,o.owner_id,o.creator_session_id,'MERCHANT',o.creator_kind,o.create_attempt_id);
 END LOOP;
 RETURN true;
END $$;
ALTER FUNCTION inventory.amend_committed_allocation(checkout.orders,jsonb,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.amend_committed_allocation(checkout.orders,jsonb,text) FROM PUBLIC;

GRANT UPDATE(quantity) ON inventory.reservation_lines TO commerce_checkout_writer;
-- Flat comparisons retain the same stored policy expression after pg_dump/pg_restore.
CREATE POLICY amendment_reservation_lines ON inventory.reservation_lines FOR UPDATE TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND EXISTS(SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=reservation_lines.tenant_id
   AND r.store_id=reservation_lines.store_id AND r.id=reservation_lines.reservation_id
   AND r.state='COMMITTED' AND r.buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid));
CREATE POLICY amendment_ledger ON inventory.ledger FOR INSERT TO commerce_checkout_writer
 WITH CHECK(operation='checkout.order_amendment' AND actor_kind='MERCHANT' AND kind='ALLOCATE'
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
  AND EXISTS(SELECT 1 FROM checkout.orders o JOIN inventory.reservations r
   ON (r.tenant_id,r.store_id,r.id)=(o.tenant_id,o.store_id,o.id)
   WHERE o.tenant_id=ledger.tenant_id AND o.store_id=ledger.store_id
   AND o.id=ledger.checkout_id AND o.owner_id=ledger.buyer_owner_id
   AND o.commercial_state='AWAITING_COLLECTION' AND o.payment_mode='cash_on_delivery' AND o.collection_state='PENDING'
   AND o.fulfillment_state='MANUAL_UNASSIGNED' AND r.state='COMMITTED'));

CREATE FUNCTION inventory.guard_amendment_ledger() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE effective bigint; prior bigint;
BEGIN
 IF NEW.operation IS DISTINCT FROM 'checkout.order_amendment' THEN RETURN NEW; END IF;
 SELECT l.quantity INTO effective FROM inventory.reservation_lines l
  WHERE (l.tenant_id,l.store_id,l.reservation_id,l.warehouse_id,l.sku_id)=
   (NEW.tenant_id,NEW.store_id,NEW.reservation_id,NEW.warehouse_id,NEW.sku_id);
 SELECT coalesce(sum(a.delta_allocated),0) INTO prior FROM inventory.ledger a
  WHERE (a.tenant_id,a.store_id,a.reservation_id,a.warehouse_id,a.sku_id)=
   (NEW.tenant_id,NEW.store_id,NEW.reservation_id,NEW.warehouse_id,NEW.sku_id);
 IF NEW.kind<>'ALLOCATE' OR NEW.actor_kind<>'MERCHANT' OR NEW.delta_reserved<>0
  OR effective IS DISTINCT FROM prior+NEW.delta_allocated THEN
  RAISE EXCEPTION 'R03 allocation ledger mismatch' USING ERRCODE='42501'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inventory.guard_amendment_ledger() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.guard_amendment_ledger() FROM PUBLIC;
CREATE TRIGGER zz_amendment_ledger_guard BEFORE INSERT ON inventory.ledger
 FOR EACH ROW EXECUTE FUNCTION inventory.guard_amendment_ledger();

DO $$
DECLARE d text; def record;
BEGIN
 FOR def IN SELECT * FROM (VALUES
  ('ledger_check', ' OR ((kind = ''ALLOCATE''::text)',
   ' OR (kind=''ALLOCATE'' AND operation=''checkout.order_amendment'' AND delta_on_hand=0 AND delta_reserved=0
     AND delta_allocated>0 AND delta_unavailable=0 AND reservation_id IS NOT NULL)'),
  ('ledger_checkout_actor',' OR ((actor_kind = ''ORDER_ATTEMPT''::text)',
   ' OR (actor_kind=''MERCHANT'' AND kind=''ALLOCATE'' AND operation=''checkout.order_amendment''
     AND principal_id IS NOT NULL AND checkout_id IS NOT NULL AND buyer_owner_id IS NOT NULL AND reservation_id=checkout_id
     AND payment_attempt_id IS NULL AND payment_fact_kind IS NULL)')
 ) AS edits(constraint_name,anchor,addition) LOOP
  SELECT pg_get_constraintdef(oid) INTO STRICT d FROM pg_constraint WHERE conrelid='inventory.ledger'::regclass
   AND conname=def.constraint_name;
  IF (length(d)-length(replace(d,def.anchor,'')))<>length(def.anchor) THEN
   RAISE EXCEPTION 'R03 ledger baseline drift: %',def.constraint_name; END IF;
  EXECUTE format('ALTER TABLE inventory.ledger DROP CONSTRAINT %I',def.constraint_name);
  EXECUTE format('ALTER TABLE inventory.ledger ADD CONSTRAINT %I %s',def.constraint_name,replace(d,def.anchor,def.addition||def.anchor));
 END LOOP;
 d:=pg_get_functiondef('inventory.guard_bank_transfer_ledger()'::regprocedure);
 IF strpos(d,'IF NEW.actor_kind=''MERCHANT'' AND NEW.kind=''ALLOCATE'' THEN')=0 THEN
  RAISE EXCEPTION 'R03 bank transfer baseline drift'; END IF;
 EXECUTE replace(d,'IF NEW.actor_kind=''MERCHANT'' AND NEW.kind=''ALLOCATE'' THEN',
  'IF NEW.actor_kind=''MERCHANT'' AND NEW.kind=''ALLOCATE'' AND NEW.operation<>''checkout.order_amendment'' THEN');
 -- The existing generic returns guard already bounds releases by the remaining
 -- balance. The COD guard also accepts a sum of committed R03 allocation rows.
 d:=pg_get_functiondef('inventory.guard_pay_at_pickup_ledger()'::regprocedure);
 IF strpos(d,'AND a.kind=''ALLOCATE'' AND a.delta_allocated=-NEW.delta_allocated')=0 THEN
  RAISE EXCEPTION 'R03 effective release baseline drift'; END IF;
 d:=replace(d,'OR NOT EXISTS(SELECT 1 FROM inventory.ledger a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
    AND a.checkout_id=NEW.checkout_id AND a.warehouse_id=NEW.warehouse_id AND a.sku_id=NEW.sku_id
    AND a.kind=''ALLOCATE'' AND a.delta_allocated=-NEW.delta_allocated',
  'OR (NOT EXISTS(SELECT 1 FROM inventory.ledger a WHERE a.tenant_id=NEW.tenant_id AND a.store_id=NEW.store_id
    AND a.checkout_id=NEW.checkout_id AND a.warehouse_id=NEW.warehouse_id AND a.sku_id=NEW.sku_id
    AND a.kind=''ALLOCATE'' AND a.delta_allocated=-NEW.delta_allocated');
 IF strpos(d,'OR (NOT EXISTS(SELECT 1 FROM inventory.ledger a')=0 THEN
  RAISE EXCEPTION 'R03 effective release origin drift'; END IF;
 d:=replace(d,'AND a.command_key=NEW.checkout_id::text)))',
  'AND a.command_key=NEW.checkout_id::text)))
    AND NOT EXISTS(SELECT 1 FROM inventory.ledger a JOIN checkout.orders o
     ON (o.tenant_id,o.store_id,o.id)=(a.tenant_id,a.store_id,a.checkout_id)
     WHERE (a.tenant_id,a.store_id,a.checkout_id,a.warehouse_id,a.sku_id)=
      (NEW.tenant_id,NEW.store_id,NEW.checkout_id,NEW.warehouse_id,NEW.sku_id)
     AND o.payment_mode=''cash_on_delivery'' AND o.order_revision>1
     AND a.operation=''checkout.order_amendment'' AND a.kind=''ALLOCATE''))');
 IF strpos(d,'AND o.order_revision>1')=0 THEN RAISE EXCEPTION 'R03 effective release guard drift'; END IF;
 EXECUTE d;
END $$;

CREATE FUNCTION identity.amend_order(p_hash bytea,p_store uuid,p_order uuid,p_expected bigint,
 p_recipient jsonb,p_additions jsonb,p_key text,p_proposal text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a jsonb; tenant uuid; principal uuid; o checkout.orders%ROWTYPE; r inventory.reservations%ROWTYPE;
 destination jsonb; quote jsonb; next_snapshot jsonb; review jsonb; proposal text; receipt jsonb; digest bytea; total bigint;
 saved record; cap bigint; has_additions boolean; failure text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_expected IS NULL
  OR p_expected NOT BETWEEN 1 AND 9007199254740990
  OR (p_key IS NULL)<>(p_proposal IS NULL) OR (p_key IS NOT NULL AND
    (p_key !~ '^[A-Za-z0-9_.:-]{8,128}$' OR p_proposal !~ '^[a-f0-9]{64}$'))
  OR current_setting('transaction_isolation')<>'read committed'
  OR coalesce(jsonb_typeof(p_additions),'array')<>'array'
  OR coalesce(jsonb_array_length(p_additions),0)>50 THEN
  RAISE EXCEPTION 'invalid amendment' USING ERRCODE='PT400'; END IF;
 has_additions:=coalesce(jsonb_array_length(p_additions),0)>0;
 IF p_recipient IS NULL AND NOT has_additions THEN RAISE EXCEPTION 'invalid amendment' USING ERRCODE='PT400'; END IF;
 a:=checkout.amend_authority(p_hash,p_store,has_additions);
 tenant:=(a->>'tenant')::uuid; principal:=(a->>'principal')::uuid;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',principal::text,true);
 IF p_key IS NOT NULL THEN
  -- Use the existing operation/tenant/store/key uniqueness as the replay domain.
  PERFORM pg_advisory_xact_lock(hashtextextended('checkout.amendment|'||tenant||'|'||p_store||'|'||p_key,0));
  digest:=sha256(convert_to(jsonb_build_array(principal,p_store,p_order,p_expected,p_recipient,p_additions,p_proposal)::text,'UTF8'));
  SELECT * INTO o FROM checkout.orders k WHERE k.tenant_id=tenant AND k.store_id=p_store AND k.id=p_order FOR UPDATE;
 ELSE
  SELECT * INTO o FROM checkout.orders k WHERE k.tenant_id=tenant AND k.store_id=p_store AND k.id=p_order FOR SHARE;
 END IF;
 IF a IS DISTINCT FROM checkout.amend_authority(p_hash,p_store,has_additions) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF o.id IS NULL THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF p_key IS NOT NULL THEN
  SELECT c.request_hash,c.response INTO saved FROM ops.command_results c WHERE c.tenant_id=tenant AND c.store_id=p_store
   AND c.operation='checkout.order_amendment' AND c.idempotency_key=p_key;
  IF FOUND THEN
   IF saved.request_hash<>digest THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
   RETURN saved.response;
  END IF;
 END IF;
 <<prepare>>
 BEGIN
  IF o.order_revision<>p_expected THEN failure:='version_changed'; EXIT prepare; END IF;
  IF o.payment_mode IS DISTINCT FROM 'cash_on_delivery' OR o.collection_state IS DISTINCT FROM 'PENDING'
    OR o.currency<>'TWD' OR o.country<>'TW' OR o.commercial_state<>'AWAITING_COLLECTION'
    OR o.fulfillment_state<>'MANUAL_UNASSIGNED' OR o.snapshot#>>'{destination,kind}' IS DISTINCT FROM 'home'
    OR EXISTS(SELECT 1 FROM fulfillment.manual_shipment_heads h WHERE h.tenant_id=tenant AND h.store_id=p_store AND h.order_id=p_order)
    OR EXISTS(SELECT 1 FROM fulfillment.parcel_group_orders g WHERE g.tenant_id=tenant AND g.store_id=p_store AND g.order_id=p_order)
    OR EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=tenant AND c.store_id=p_store AND c.order_id=p_order)
    OR EXISTS(SELECT 1 FROM returns.rmas x WHERE x.tenant_id=tenant AND x.store_id=p_store AND x.order_id=p_order)
    OR EXISTS(SELECT 1 FROM checkout.payment_attempts x WHERE x.tenant_id=tenant AND x.store_id=p_store AND x.order_id=p_order) THEN
   failure:='amendment_on_hold'; EXIT prepare; END IF;
  PERFORM set_config('app.buyer_id',o.owner_id::text,true),
   set_config('app.buyer_session_id',coalesce(o.creator_session_id::text,''),true),
   set_config('app.order_attempt_id',coalesce(o.create_attempt_id::text,''),true);
  SELECT * INTO r FROM inventory.reservations x WHERE x.tenant_id=tenant AND x.store_id=p_store AND x.id=p_order;
  IF r.state IS DISTINCT FROM 'COMMITTED' OR r.buyer_owner_id IS DISTINCT FROM o.owner_id
    OR r.creator_kind IS DISTINCT FROM o.creator_kind OR r.checkout_id IS DISTINCT FROM p_order THEN
   failure:='amendment_on_hold'; EXIT prepare; END IF;
  SELECT c.max_twd::bigint*100 INTO cap FROM checkout.cash_on_delivery_settings c
   WHERE c.tenant_id=tenant AND c.store_id=p_store FOR SHARE;
  destination:=checkout.amend_recipient(o.snapshot->'destination',p_recipient);
  quote:=CASE WHEN has_additions THEN checkout.amend_price(o,p_additions,p_key IS NOT NULL) ELSE o.snapshot->'quote' END;
  total:=(quote#>>'{amount,total_minor}')::bigint;
  IF cap IS NULL OR total IS NULL OR o.cod_surcharge_minor IS NULL OR (total+o.cod_surcharge_minor)%100<>0
   OR total+o.cod_surcharge_minor>cap THEN
   failure:=CASE WHEN p_key IS NULL THEN 'amendment_on_hold' ELSE 'version_changed' END; EXIT prepare; END IF;
  next_snapshot:=o.snapshot||jsonb_build_object('quote',quote,'destination',destination);
  review:=jsonb_build_object('order_id',p_order,'order_revision',o.order_revision,'items',quote->'lines',
   'totals',quote->'amount','cod_surcharge_minor',o.cod_surcharge_minor,'cod_collect_minor',total+o.cod_surcharge_minor);
  proposal:=encode(sha256(convert_to(jsonb_build_array(a,p_store,p_order,p_expected,p_recipient,p_additions,next_snapshot,cap)::text,'UTF8')),'hex');
  IF p_key IS NULL THEN
   receipt:=review||jsonb_build_object('proposal',proposal);
   EXIT prepare;
  END IF;
  IF proposal<>p_proposal THEN failure:='version_changed'; EXIT prepare; END IF;
  IF has_additions AND NOT inventory.amend_committed_allocation(o,
   (SELECT jsonb_agg(x.value ORDER BY x.position) FROM jsonb_array_elements(quote->'lines') WITH ORDINALITY x(value,position)
    WHERE x.position>jsonb_array_length(o.snapshot#>'{quote,lines}')),p_key) THEN
   failure:='insufficient_stock'; EXIT prepare;
  END IF;
  IF o.order_revision=1 THEN
   INSERT INTO checkout.order_revisions VALUES(tenant,p_store,o.owner_id,p_order,1,o.total_minor,o.snapshot);
  END IF;
  INSERT INTO checkout.order_revisions VALUES(tenant,p_store,o.owner_id,p_order,o.order_revision+1,total,next_snapshot);
  IF NOT checkout.persist_order_amendment(tenant,p_store,o.owner_id,p_order,o.order_revision,next_snapshot,total) THEN
   RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409';
  END IF;
  receipt:=jsonb_build_object('order_id',p_order,'order_revision',o.order_revision+1,'cod_collect_minor',total+o.cod_surcharge_minor);
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(tenant,p_store,principal,'checkout.order_amended');
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(tenant,p_store,'checkout.order_amendment',p_key,digest,receipt,principal);
 END prepare;
 IF a IS DISTINCT FROM checkout.amend_authority(p_hash,p_store,has_additions) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF failure IS NOT NULL THEN RAISE EXCEPTION '%',failure USING ERRCODE='PT409'; END IF;
 RETURN receipt;
END $$;
ALTER FUNCTION identity.amend_order(bytea,uuid,uuid,bigint,jsonb,jsonb,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION identity.amend_order(bytea,uuid,uuid,bigint,jsonb,jsonb,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.amend_order(bytea,uuid,uuid,bigint,jsonb,jsonb,text,text) TO commerce_runtime;

ALTER TABLE fulfillment.manual_shipment_versions ADD COLUMN order_revision bigint CHECK(order_revision>0);
COMMENT ON COLUMN fulfillment.manual_shipment_versions.order_revision IS
 'The reviewed order revision for R03 first dispatch, retained by corrections and voids. Legacy and non-R03 receipts have NULL.';

CREATE FUNCTION fulfillment.authorize_order_document(p_hash bytea,p_store uuid,p_order uuid,p_revision bigint)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE initial record; final record; o checkout.orders%ROWTYPE; eligible boolean;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_revision IS NULL
  OR p_revision<1 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid document request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO initial FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF initial.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF initial.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF initial.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 SELECT * INTO o FROM checkout.orders k
  WHERE (k.tenant_id,k.store_id,k.id)=(initial.tenant_id,p_store,p_order) FOR SHARE;
 eligible:=o.payment_mode='cash_on_delivery' AND o.snapshot#>>'{destination,kind}'='home'
  AND fulfillment.manual_shipment_eligible(initial.tenant_id,p_store,p_order);
 SELECT * INTO final FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF final.access_status='unauthorized' OR NOT EXISTS(SELECT 1 FROM identity.sessions s
  WHERE s.token_hash=p_hash AND s.audience='merchant' AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp()) THEN
  RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF final.access_status='not_found' OR o.id IS NULL THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF final.access_status IS DISTINCT FROM 'ok' OR final.tenant_id IS DISTINCT FROM initial.tenant_id
  OR final.principal_id IS DISTINCT FROM initial.principal_id OR final.authz_revision IS DISTINCT FROM initial.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF o.order_revision<>p_revision THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
 IF eligible IS DISTINCT FROM true THEN RAISE EXCEPTION 'not_shippable' USING ERRCODE='PT422'; END IF;
END $$;
ALTER FUNCTION fulfillment.authorize_order_document(bytea,uuid,uuid,bigint) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.authorize_order_document(bytea,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.authorize_order_document(bytea,uuid,uuid,bigint) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.authorize_order_document(bytea,uuid,uuid,bigint) IS
 'Scoped warehouse issuance/print check under an order SHARE lock. Order revision, payment and logistics eligibility are read-only.';

DO $$
DECLARE d text; patch record;
BEGIN
 d:=pg_get_functiondef('fulfillment.record_manual_shipment(bytea,uuid,uuid,text,bytea,bigint,text,text,text,text,text,text,text)'::regprocedure);
 FOR patch IN SELECT * FROM (VALUES
  ('p_void_reason text)', 'p_void_reason text, p_order_revision bigint)'),
  ('v_head bigint;', 'v_head bigint; v_revision bigint; v_is_r03 boolean; v_reviewed bigint;'),
  ('SELECT o.owner_id,o.fulfillment_state,o.collection_state INTO v_owner,v_fulfillment,v_collection',
   'SELECT o.owner_id,o.fulfillment_state,o.collection_state,o.order_revision,
      o.payment_mode=''cash_on_delivery'' AND o.snapshot#>>''{destination,kind}''=''home''
     INTO v_owner,v_fulfillment,v_collection,v_revision,v_is_r03'),
  ('SELECT v.status INTO v_head_status', 'SELECT v.status,v.order_revision INTO v_head_status,v_reviewed'),
  ('ELSIF NOT fulfillment.manual_shipment_eligible(s.tenant_id,p_store,p_order) THEN',
   'ELSIF v_is_r03 AND p_order_revision IS DISTINCT FROM v_revision THEN
      v_fail_code:=''PT409''; v_fail_msg:=''version_changed'';
    ELSIF v_is_r03 AND EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c
      WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store AND c.order_id=p_order AND c.state NOT IN (''FAILED'',''ABANDONED'')) THEN
      v_fail_code:=''PT422''; v_fail_msg:=''not_shippable'';
    ELSIF NOT fulfillment.manual_shipment_eligible(s.tenant_id,p_store,p_order) THEN'),
  ('tracking_number,tracking_url,note,void_reason,principal_id)',
   'tracking_number,tracking_url,note,void_reason,principal_id,order_revision)'),
  ('v_prev.tracking_url,NULL,p_void_reason,s.principal_id)'||chr(10),
   'v_prev.tracking_url,NULL,p_void_reason,s.principal_id,v_reviewed)'||chr(10)),
  ('p_tracking_url,p_note,NULL,s.principal_id)'||chr(10),
   'p_tracking_url,p_note,NULL,s.principal_id,CASE WHEN v_is_r03 AND v_action=''fulfillment.shipment_recorded''
     THEN p_order_revision ELSE v_reviewed END)'||chr(10)),
  ('INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)',
   'IF v_row.order_revision IS NOT NULL THEN
     v_response:=v_response||jsonb_build_object(''order_revision'',v_row.order_revision); END IF;
    INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)')
 ) AS edits(needle,replacement) LOOP
  IF strpos(d,patch.needle)=0 THEN RAISE EXCEPTION 'R03 shipment baseline drift: %',patch.needle; END IF;
  d:=replace(d,patch.needle,patch.replacement);
 END LOOP;
 EXECUTE d;
END $$;
ALTER FUNCTION fulfillment.record_manual_shipment(bytea,uuid,uuid,text,bytea,bigint,text,text,text,text,text,text,text,bigint) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.record_manual_shipment(bytea,uuid,uuid,text,bytea,bigint,text,text,text,text,text,text,text,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.record_manual_shipment(bytea,uuid,uuid,text,bytea,bigint,text,text,text,text,text,text,text,bigint) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.record_manual_shipment(bytea,uuid,uuid,text,bytea,bigint,text,text,text,text,text,text,text,bigint) IS
 'Merchant shipment command. Checks shipment-head CAS and, for first home COD dispatch, the reviewed order version under the checkout order lock. No provider action.';

-- Includes direct DB callers and bulk commands using the original signature.
CREATE OR REPLACE FUNCTION fulfillment.record_manual_shipment(p_hash bytea,p_store uuid,p_order uuid,p_key text,
 p_request_hash bytea,p_expected_version bigint,p_status text,p_carrier_code text,p_carrier_name text,
 p_tracking_number text,p_tracking_url text,p_note text,p_void_reason text)
RETURNS jsonb LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT fulfillment.record_manual_shipment(p_hash,p_store,p_order,p_key,p_request_hash,p_expected_version,p_status,
  p_carrier_code,p_carrier_name,p_tracking_number,p_tracking_url,p_note,p_void_reason,NULL)
$$;

DO $$
DECLARE d text; patch record;
BEGIN
 FOR patch IN SELECT * FROM (VALUES
  ('identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text)',
   'o.total_minor,o.commercial_state,o.fulfillment_state,o.country,o.service_code,',
   'o.total_minor,o.commercial_state,o.fulfillment_state,o.country,o.service_code,o.order_revision,'),
  ('identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text)',
   'CASE WHEN p_order IS NULL THEN ''{}''::jsonb ELSE jsonb_build_object(',
   'CASE WHEN p_order IS NULL THEN ''{}''::jsonb ELSE
    CASE WHEN o.payment_mode=''cash_on_delivery'' AND o.snapshot#>>''{destination,kind}''=''home''
     THEN jsonb_build_object(''order_revision'',o.order_revision) ELSE ''{}''::jsonb END || jsonb_build_object('),
  ('fulfillment.read_manual_shipment_history(bytea,uuid,uuid)',
   '''principal_id'',v.principal_id) ORDER BY v.version',
   '''principal_id'',v.principal_id) || CASE WHEN v.order_revision IS NULL THEN ''{}''::jsonb
     ELSE jsonb_build_object(''order_revision'',v.order_revision) END ORDER BY v.version'),
  ('identity.export_unshipped_orders(bytea,uuid,integer)',
   'AND o.commercial_state IN (''CONFIRMED'',''AWAITING_COLLECTION'')',
   'AND o.commercial_state IN (''CONFIRMED'',''AWAITING_COLLECTION'') AND o.payment_mode<>''cash_on_delivery''')
 ) AS edits(proc,needle,replacement) LOOP
  d:=pg_get_functiondef(patch.proc::regprocedure);
  IF (length(d)-length(replace(d,patch.needle,'')))<>length(patch.needle) THEN
   RAISE EXCEPTION 'R03 projection baseline drift: %',patch.proc; END IF;
  EXECUTE replace(d,patch.needle,patch.replacement);
 END LOOP;
END $$;
