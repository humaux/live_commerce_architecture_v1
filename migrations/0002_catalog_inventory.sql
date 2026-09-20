-- Catalog and physical inventory. No payment or fulfillment authority here.
CREATE SCHEMA catalog;
CREATE ROLE commerce_inventory_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
CREATE SCHEMA inventory AUTHORIZATION commerce_inventory_writer;
REVOKE ALL ON SCHEMA catalog, inventory FROM PUBLIC;
GRANT USAGE ON SCHEMA catalog, inventory TO commerce_runtime;
ALTER TABLE identity.store_grants DROP CONSTRAINT store_grants_permission_check;
ALTER TABLE identity.store_grants ADD CONSTRAINT store_grants_permission_check
CHECK (permission IN ('store:read','audit:read','audit:write','catalog:read','catalog:write','inventory:read','inventory:write','inventory:reserve'));

CREATE TABLE ops.command_results (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    operation text NOT NULL CHECK (operation ~ '^[a-z][a-z0-9_.:]{0,79}$'),
    idempotency_key text NOT NULL CHECK (idempotency_key ~ '^[A-Za-z0-9_.:-]{8,128}$'),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
    response jsonb NOT NULL,
    principal_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id,store_id,operation,idempotency_key),
    FOREIGN KEY (tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
    FOREIGN KEY (tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);
CREATE TABLE catalog.products (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '' CHECK (length(description)<=8000),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    version bigint NOT NULL DEFAULT 1 CHECK (version>0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
CREATE TABLE catalog.skus (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    product_id uuid NOT NULL,
    code text NOT NULL CHECK (code ~ '^[A-Za-z0-9_.-]{1,64}$'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    price_minor bigint NOT NULL CHECK (price_minor BETWEEN 0 AND 1000000000000),
    version bigint NOT NULL DEFAULT 1 CHECK (version>0),
    weight_grams bigint NOT NULL DEFAULT 0 CHECK (weight_grams BETWEEN 0 AND 1000000000),
    length_mm bigint NOT NULL DEFAULT 0 CHECK (length_mm BETWEEN 0 AND 1000000),
    width_mm bigint NOT NULL DEFAULT 0 CHECK (width_mm BETWEEN 0 AND 1000000),
    height_mm bigint NOT NULL DEFAULT 0 CHECK (height_mm BETWEEN 0 AND 1000000),
    origin_country text NOT NULL DEFAULT '' CHECK (origin_country='' OR origin_country ~ '^[A-Z]{2}$'),
    customs_name text NOT NULL DEFAULT '' CHECK (length(customs_name)<=240),
    hs_candidate text NOT NULL DEFAULT '' CHECK (hs_candidate='' OR hs_candidate ~ '^[0-9]{6,12}$'),
    hs_confirmed text,
    hs_confirmed_by uuid,
    hs_confirmed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id,store_id,id),
    UNIQUE (tenant_id,store_id,code),
    FOREIGN KEY (tenant_id,store_id,product_id) REFERENCES catalog.products(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,hs_confirmed_by) REFERENCES identity.memberships(tenant_id,principal_id),
    CHECK ((hs_confirmed IS NULL AND hs_confirmed_by IS NULL AND hs_confirmed_at IS NULL)
        OR (hs_confirmed IS NOT NULL AND hs_confirmed ~ '^[0-9]{6,12}$' AND hs_confirmed_by IS NOT NULL AND hs_confirmed_at IS NOT NULL))
);
CREATE INDEX skus_product ON catalog.skus(tenant_id,store_id,product_id,id);
CREATE TABLE catalog.price_history (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    sku_id uuid NOT NULL,
    version bigint NOT NULL CHECK (version>0),
    price_minor bigint NOT NULL CHECK (price_minor BETWEEN 0 AND 1000000000000),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    principal_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id,store_id,sku_id,version),
    FOREIGN KEY (tenant_id,store_id,sku_id) REFERENCES catalog.skus(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);
CREATE TABLE inventory.warehouses (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    active boolean NOT NULL DEFAULT true,
    PRIMARY KEY (tenant_id,store_id,id),
    UNIQUE (tenant_id,store_id,name),
    FOREIGN KEY (tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
CREATE TABLE inventory.balances (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    sku_id uuid NOT NULL,
    on_hand bigint NOT NULL DEFAULT 0 CHECK (on_hand BETWEEN 0 AND 1000000000),
    reserved bigint NOT NULL DEFAULT 0 CHECK (reserved BETWEEN 0 AND 1000000000),
    allocated bigint NOT NULL DEFAULT 0 CHECK (allocated BETWEEN 0 AND 1000000000),
    unavailable bigint NOT NULL DEFAULT 0 CHECK (unavailable BETWEEN 0 AND 1000000000),
    version bigint NOT NULL DEFAULT 0 CHECK (version>=0),
    PRIMARY KEY (tenant_id,store_id,warehouse_id,sku_id),
    FOREIGN KEY (tenant_id,store_id,warehouse_id) REFERENCES inventory.warehouses(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id,sku_id) REFERENCES catalog.skus(tenant_id,store_id,id),
    CHECK (reserved+allocated+unavailable<=on_hand)
);
CREATE TABLE inventory.reservations (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    state text NOT NULL CHECK (state IN ('HELD','PAYMENT_PENDING','COMMITTED','RELEASED','EXPIRED')),
    expires_at timestamptz NOT NULL DEFAULT (clock_timestamp()+interval '15 minutes'),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
CREATE TABLE inventory.reservation_lines (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    reservation_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    sku_id uuid NOT NULL,
    quantity bigint NOT NULL CHECK (quantity BETWEEN 1 AND 1000000000),
    PRIMARY KEY (tenant_id,store_id,reservation_id,warehouse_id,sku_id),
    FOREIGN KEY (tenant_id,store_id,reservation_id) REFERENCES inventory.reservations(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id,warehouse_id,sku_id) REFERENCES inventory.balances(tenant_id,store_id,warehouse_id,sku_id)
);
CREATE TABLE inventory.ledger (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    warehouse_id uuid NOT NULL,
    sku_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('ADJUST','RESERVE','RELEASE')),
    delta_on_hand bigint NOT NULL DEFAULT 0 CHECK (delta_on_hand BETWEEN -1000000000 AND 1000000000),
    delta_reserved bigint NOT NULL DEFAULT 0 CHECK (delta_reserved BETWEEN -1000000000 AND 1000000000),
    delta_allocated bigint NOT NULL DEFAULT 0 CHECK (delta_allocated BETWEEN -1000000000 AND 1000000000),
    delta_unavailable bigint NOT NULL DEFAULT 0 CHECK (delta_unavailable BETWEEN -1000000000 AND 1000000000),
    operation text NOT NULL,
    command_key text NOT NULL,
    reservation_id uuid,
    reason text NOT NULL DEFAULT '' CHECK (length(reason)<=240),
    principal_id uuid NOT NULL,
    balance_version bigint NOT NULL DEFAULT 1 CHECK (balance_version>0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id,store_id,id),
    UNIQUE (tenant_id,store_id,operation,command_key,warehouse_id,sku_id,kind),
    FOREIGN KEY (tenant_id,store_id,warehouse_id) REFERENCES inventory.warehouses(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id,sku_id) REFERENCES catalog.skus(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id,reservation_id) REFERENCES inventory.reservations(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id),
    CHECK (delta_allocated=0 AND delta_unavailable=0 AND (
       (kind='ADJUST' AND delta_on_hand<>0 AND delta_reserved=0 AND reservation_id IS NULL AND length(reason)>0)
       OR (kind='RESERVE' AND delta_on_hand=0 AND delta_reserved>0 AND reservation_id IS NOT NULL)
       OR (kind='RELEASE' AND delta_on_hand=0 AND delta_reserved<0 AND reservation_id IS NOT NULL)))
);

-- TX: ledger is the only runtime write entry to balances. FORCE RLS still applies
-- to the non-login trigger owner, with a fixed search_path and no membership grant.
CREATE FUNCTION inventory.apply_ledger() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE resulting_version bigint;
BEGIN
    IF NEW.tenant_id IS DISTINCT FROM nullif(current_setting('app.tenant_id',true),'')::uuid
       OR NEW.store_id IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid
       OR NEW.principal_id IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid THEN
        RAISE EXCEPTION 'inventory scope mismatch' USING ERRCODE='42501';
    END IF;
    INSERT INTO inventory.balances(tenant_id,store_id,warehouse_id,sku_id)
    VALUES (NEW.tenant_id,NEW.store_id,NEW.warehouse_id,NEW.sku_id) ON CONFLICT DO NOTHING;
    UPDATE inventory.balances SET
        on_hand=on_hand+NEW.delta_on_hand, reserved=reserved+NEW.delta_reserved,
        allocated=allocated+NEW.delta_allocated, unavailable=unavailable+NEW.delta_unavailable,
        version=version+1
    WHERE tenant_id=NEW.tenant_id AND store_id=NEW.store_id
        AND warehouse_id=NEW.warehouse_id AND sku_id=NEW.sku_id
    RETURNING version INTO resulting_version;
    IF resulting_version IS NULL THEN
        RAISE EXCEPTION 'inventory scope missing' USING ERRCODE='42501';
    END IF;
    UPDATE inventory.ledger SET balance_version=resulting_version
    WHERE tenant_id=NEW.tenant_id AND store_id=NEW.store_id AND id=NEW.id;
    RETURN NEW;
END $$;
ALTER FUNCTION inventory.apply_ledger() OWNER TO commerce_inventory_writer;
REVOKE ALL ON FUNCTION inventory.apply_ledger() FROM PUBLIC;
-- AFTER INSERT is essential: BEFORE would change balance even when an INSERT
-- ON CONFLICT DO NOTHING skips the ledger row. Never count an uninserted event.
CREATE TRIGGER ledger_balance AFTER INSERT ON inventory.ledger
FOR EACH ROW EXECUTE FUNCTION inventory.apply_ledger();
GRANT SELECT,INSERT,UPDATE ON inventory.balances TO commerce_inventory_writer;
GRANT SELECT,UPDATE(balance_version) ON inventory.ledger TO commerce_inventory_writer;

-- SELECT FOR UPDATE itself requires UPDATE privilege. Expose only a scoped
-- lock/read function, not UPDATE(balance), so domains cannot bypass the ledger.
CREATE FUNCTION inventory.lock_balance(p_warehouse uuid,p_sku uuid)
RETURNS SETOF inventory.balances LANGUAGE sql VOLATILE SECURITY DEFINER
SET search_path=pg_catalog AS $$
    SELECT b.* FROM inventory.balances b
    WHERE b.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
      AND b.store_id=nullif(current_setting('app.store_id',true),'')::uuid
      AND b.warehouse_id=p_warehouse AND b.sku_id=p_sku FOR UPDATE
$$;
ALTER FUNCTION inventory.lock_balance(uuid,uuid) OWNER TO commerce_inventory_writer;
REVOKE ALL ON FUNCTION inventory.lock_balance(uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inventory.lock_balance(uuid,uuid) TO commerce_runtime;

-- All tenant/store relations, including append-only histories, fail closed without Scope.
DO $$
DECLARE relation_name text;
BEGIN
    FOREACH relation_name IN ARRAY ARRAY['ops.command_results','catalog.products','catalog.skus',
        'catalog.price_history','inventory.warehouses','inventory.balances','inventory.reservations',
        'inventory.reservation_lines','inventory.ledger'] LOOP
        EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY',relation_name);
        EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY',relation_name);
        EXECUTE format('CREATE POLICY scope_access ON %s TO commerce_runtime,commerce_inventory_writer
           USING (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
               AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)
           WITH CHECK (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
               AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',relation_name);
    END LOOP;
END $$;
CREATE POLICY command_actor ON ops.command_results AS RESTRICTIVE FOR INSERT TO commerce_runtime
WITH CHECK (principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY price_actor ON catalog.price_history AS RESTRICTIVE FOR INSERT TO commerce_runtime
WITH CHECK (principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY ledger_actor ON inventory.ledger AS RESTRICTIVE FOR INSERT TO commerce_runtime
WITH CHECK (principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
GRANT SELECT,INSERT ON ops.command_results,catalog.price_history,inventory.reservation_lines,inventory.ledger TO commerce_runtime;
GRANT SELECT,INSERT,UPDATE ON catalog.products TO commerce_runtime;
GRANT SELECT ON catalog.skus TO commerce_runtime;
-- HS confirmation is a separate not-yet-implemented merchant attestation.
GRANT INSERT(tenant_id,store_id,id,product_id,code,status,currency,price_minor,version,weight_grams,length_mm,width_mm,height_mm,origin_country,customs_name,hs_candidate) ON catalog.skus TO commerce_runtime;
GRANT UPDATE(code,status,price_minor,version,weight_grams,length_mm,width_mm,height_mm,origin_country,customs_name,hs_candidate,updated_at) ON catalog.skus TO commerce_runtime;
GRANT SELECT,INSERT ON inventory.warehouses TO commerce_runtime;
GRANT SELECT,INSERT,UPDATE(state) ON inventory.reservations TO commerce_runtime;
GRANT SELECT ON inventory.balances TO commerce_runtime;
