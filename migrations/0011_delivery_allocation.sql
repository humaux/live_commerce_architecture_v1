-- Merchant-owned warehouse priority is independent of delivery fees/carriers.
CREATE TABLE fulfillment.allocation_versions (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,market_id uuid NOT NULL,
 country text NOT NULL,code text NOT NULL,version bigint NOT NULL CHECK(version>0),
 service_version bigint NOT NULL,warehouse_count smallint NOT NULL CHECK(warehouse_count BETWEEN 0 AND 16),
 principal_id uuid NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,market_id,country,code,version),
 FOREIGN KEY(tenant_id,store_id,market_id,country,code,service_version)
  REFERENCES fulfillment.service_versions(tenant_id,store_id,market_id,country,code,version),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);
CREATE TABLE fulfillment.allocation_warehouses (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,market_id uuid NOT NULL,
 country text NOT NULL,code text NOT NULL,version bigint NOT NULL,
 position smallint NOT NULL CHECK(position BETWEEN 1 AND 16),warehouse_id uuid NOT NULL,
 PRIMARY KEY(tenant_id,store_id,market_id,country,code,version,position),
 UNIQUE(tenant_id,store_id,market_id,country,code,version,warehouse_id),
 FOREIGN KEY(tenant_id,store_id,market_id,country,code,version)
  REFERENCES fulfillment.allocation_versions(tenant_id,store_id,market_id,country,code,version),
 FOREIGN KEY(tenant_id,store_id,warehouse_id) REFERENCES inventory.warehouses(tenant_id,store_id,id)
);
CREATE TABLE fulfillment.allocation_heads (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,market_id uuid NOT NULL,
 country text NOT NULL,code text NOT NULL,current_version bigint NOT NULL,
 PRIMARY KEY(tenant_id,store_id,market_id,country,code),
 FOREIGN KEY(tenant_id,store_id,market_id,country,code,current_version)
  REFERENCES fulfillment.allocation_versions(tenant_id,store_id,market_id,country,code,version)
);

-- A successful commit cannot preserve a partial or gapped immutable priority list.
CREATE FUNCTION fulfillment.check_allocation_complete() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
DECLARE expected_count integer; actual_count bigint; last_position integer;
BEGIN
 SELECT warehouse_count INTO expected_count FROM fulfillment.allocation_versions
 WHERE tenant_id=NEW.tenant_id AND store_id=NEW.store_id AND market_id=NEW.market_id
  AND country=NEW.country AND code=NEW.code AND version=NEW.version;
 SELECT count(*),coalesce(max(position),0) INTO actual_count,last_position
 FROM fulfillment.allocation_warehouses
 WHERE tenant_id=NEW.tenant_id AND store_id=NEW.store_id AND market_id=NEW.market_id
  AND country=NEW.country AND code=NEW.code AND version=NEW.version;
 IF expected_count IS NULL OR actual_count<>expected_count OR last_position<>expected_count THEN
  RAISE EXCEPTION 'incomplete allocation warehouse list' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
REVOKE ALL ON FUNCTION fulfillment.check_allocation_complete() FROM PUBLIC;
CREATE CONSTRAINT TRIGGER allocation_complete AFTER INSERT ON fulfillment.allocation_versions
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fulfillment.check_allocation_complete();
-- Also reject later attempts to append to an already committed revision.
CREATE CONSTRAINT TRIGGER allocation_children_complete AFTER INSERT ON fulfillment.allocation_warehouses
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fulfillment.check_allocation_complete();

DO $$
DECLARE relation_name text;
BEGIN
 FOREACH relation_name IN ARRAY ARRAY['allocation_versions','allocation_warehouses','allocation_heads'] LOOP
  EXECUTE format('ALTER TABLE fulfillment.%I ENABLE ROW LEVEL SECURITY',relation_name);
  EXECUTE format('ALTER TABLE fulfillment.%I FORCE ROW LEVEL SECURITY',relation_name);
  EXECUTE format('CREATE POLICY merchant_read ON fulfillment.%I FOR SELECT TO commerce_runtime
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',relation_name);
  EXECUTE format('CREATE POLICY merchant_insert ON fulfillment.%I FOR INSERT TO commerce_runtime
   WITH CHECK(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',relation_name);
 END LOOP;
END $$;
CREATE POLICY allocation_actor ON fulfillment.allocation_versions AS RESTRICTIVE FOR INSERT TO commerce_runtime
 WITH CHECK(principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY merchant_update ON fulfillment.allocation_heads FOR UPDATE TO commerce_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT,INSERT ON fulfillment.allocation_versions,fulfillment.allocation_warehouses,fulfillment.allocation_heads TO commerce_runtime;
GRANT UPDATE(current_version) ON fulfillment.allocation_heads TO commerce_runtime;

-- SELECT FOR SHARE needs UPDATE privilege, but runtime must not gain a mutation
-- path. Reuse the non-login inventory writer and scoped lock_balance pattern.
GRANT SELECT,UPDATE(active) ON inventory.warehouses TO commerce_inventory_writer;
CREATE FUNCTION inventory.lock_warehouse(p_warehouse uuid)
RETURNS SETOF inventory.warehouses LANGUAGE sql VOLATILE SECURITY DEFINER
SET search_path=pg_catalog AS $$
 SELECT w.* FROM inventory.warehouses w
 WHERE w.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND w.store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND w.id=p_warehouse FOR SHARE
$$;
ALTER FUNCTION inventory.lock_warehouse(uuid) OWNER TO commerce_inventory_writer;
REVOKE ALL ON FUNCTION inventory.lock_warehouse(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inventory.lock_warehouse(uuid) TO commerce_runtime;
