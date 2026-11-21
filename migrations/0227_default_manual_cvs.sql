-- Purpose: provision actual, editable Taiwan manual pickup services at onboarding and upgrade.
-- Depends on: initial_stores, pricing revisions, service and allocation revisions (0007/0010/0011/0117).
-- Used by: identity.create_initial_store, checkout options, merchant settings and both order paths.
-- Only a missing store pickup setting may opt into manual collection. Existing settings and provider accounts stay intact.
CREATE FUNCTION fulfillment.seed_manual_cvs(p_tenant uuid,p_store uuid,p_actor uuid) RETURNS void
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
DECLARE wh uuid; market record; brand record; service_code text; pv bigint; tax record;
BEGIN
 IF NOT EXISTS (SELECT 1 FROM control.stores WHERE tenant_id=p_tenant AND id=p_store AND currency='TWD') THEN RETURN; END IF;
 SELECT id INTO wh FROM inventory.warehouses WHERE tenant_id=p_tenant AND store_id=p_store AND active
 ORDER BY created_at,id LIMIT 1;
 IF wh IS NULL THEN RETURN; END IF;
 IF NOT EXISTS (SELECT 1 FROM pricing.markets WHERE tenant_id=p_tenant AND store_id=p_store) THEN
  INSERT INTO pricing.markets(tenant_id,store_id,code,name,currency,principal_id) VALUES(p_tenant,p_store,'tw','台灣','TWD',p_actor);
 END IF;
 FOR market IN SELECT m.id FROM pricing.markets m WHERE m.tenant_id=p_tenant AND m.store_id=p_store AND m.active AND
  (m.code='tw' OR EXISTS (SELECT 1 FROM pricing.policy_heads h WHERE (h.tenant_id,h.store_id,h.market_id,h.country)=(p_tenant,p_store,m.id,'TW'))
   OR (NOT EXISTS (SELECT 1 FROM pricing.markets other WHERE (other.tenant_id,other.store_id)=(p_tenant,p_store) AND other.id<>m.id)
    AND NOT EXISTS (SELECT 1 FROM pricing.policy_heads h WHERE (h.tenant_id,h.store_id,h.market_id)=(p_tenant,p_store,m.id))))
 LOOP
  -- Only inherit a market-wide tax treatment when the existing Taiwan policies agree.
  SELECT count(DISTINCT (v.tax_mode,v.tax_basis,v.tax_rate_bps)) AS variants,
   coalesce(min(v.tax_mode),'inclusive') AS mode,coalesce(min(v.tax_basis),'goods') AS basis,coalesce(min(v.tax_rate_bps),0) AS rate
   INTO tax FROM pricing.policy_heads h JOIN pricing.policy_versions v ON
   (v.tenant_id,v.store_id,v.market_id,v.country,v.method,v.version)=(h.tenant_id,h.store_id,h.market_id,h.country,h.method,h.current_version)
   WHERE (h.tenant_id,h.store_id,h.market_id,h.country)=(p_tenant,p_store,market.id,'TW');
  FOR brand IN SELECT * FROM (VALUES
    ('cvs_711','7-ELEVEN','7-ELEVEN','7-ELEVEN',1),('cvs_familymart','全家','全家','FamilyMart',2),
    ('cvs_hilife','莱尔富','萊爾富','Hi-Life',3),('cvs_okmart','OK','OK','OK mart',4)
   ) b(kind,hans,hant,en,position)
  LOOP
   service_code := 'quickstart_' || brand.kind;
   IF EXISTS (SELECT 1 FROM fulfillment.service_heads h JOIN fulfillment.service_versions v ON
     (v.tenant_id,v.store_id,v.market_id,v.country,v.code,v.version)=(h.tenant_id,h.store_id,h.market_id,h.country,h.code,h.current_version)
     WHERE (h.tenant_id,h.store_id,h.market_id,h.country)=(p_tenant,p_store,market.id,'TW')
     AND (v.delivery_kind=brand.kind OR h.code=service_code)) THEN CONTINUE; END IF;
   SELECT current_version INTO pv FROM pricing.policy_heads WHERE
    (tenant_id,store_id,market_id,country,method)=(p_tenant,p_store,market.id,'TW','delivery:'||service_code);
   IF pv IS NULL THEN
    IF tax.variants>1 THEN CONTINUE; END IF;
    pv := 1;
    INSERT INTO pricing.policy_versions(tenant_id,store_id,market_id,country,method,version,currency,shipping_mode,shipping_minor,
     tax_mode,tax_basis,tax_rate_bps,quote_ttl_seconds,enabled,configuration_ref,principal_id)
    VALUES(p_tenant,p_store,market.id,'TW','delivery:'||service_code,pv,'TWD','country_flat',0,tax.mode,tax.basis,tax.rate,
     300,true,'default manual CVS',p_actor);
    INSERT INTO pricing.policy_heads VALUES(p_tenant,p_store,market.id,'TW','delivery:'||service_code,pv);
   END IF;
   INSERT INTO fulfillment.service_versions(tenant_id,store_id,market_id,country,code,version,policy_version,currency,
    name_hans,name_hant,name_en,delivery_kind,mode,enabled,visible,sort_order,principal_id)
   SELECT p_tenant,p_store,market.id,'TW',service_code,1,pv,'TWD',brand.hans,brand.hant,brand.en,brand.kind,'MANUAL',true,true,brand.position,p_actor
    FROM pricing.policy_versions WHERE (tenant_id,store_id,market_id,country,method,version)=(p_tenant,p_store,market.id,'TW','delivery:'||service_code,pv)
    AND enabled;
   IF NOT FOUND THEN CONTINUE; END IF;
   INSERT INTO fulfillment.service_heads VALUES(p_tenant,p_store,market.id,'TW',service_code,1);
   INSERT INTO fulfillment.allocation_versions(tenant_id,store_id,market_id,country,code,version,service_version,warehouse_count,principal_id)
    VALUES(p_tenant,p_store,market.id,'TW',service_code,1,1,1,p_actor);
   INSERT INTO fulfillment.allocation_warehouses VALUES(p_tenant,p_store,market.id,'TW',service_code,1,1,wh);
   INSERT INTO fulfillment.allocation_heads VALUES(p_tenant,p_store,market.id,'TW',service_code,1);
  END LOOP;
  -- The amount due is the order's frozen total. These are admission ceilings, in TWD and open orders.
  IF EXISTS (SELECT 1 FROM fulfillment.service_heads h JOIN fulfillment.service_versions v ON
   (v.tenant_id,v.store_id,v.market_id,v.country,v.code,v.version)=(h.tenant_id,h.store_id,h.market_id,h.country,h.code,h.current_version)
   WHERE (h.tenant_id,h.store_id,h.market_id,h.country)=(p_tenant,p_store,market.id,'TW')
    AND v.mode='MANUAL' AND v.delivery_kind IN ('cvs_711','cvs_familymart','cvs_hilife','cvs_okmart') AND v.enabled AND v.visible) THEN
   INSERT INTO fulfillment.cvs_store_settings(tenant_id,store_id,pay_at_pickup_enabled,pay_at_pickup_max_twd,pay_at_pickup_max_open,version,updated_at)
    VALUES(p_tenant,p_store,true,20000,20,1,clock_timestamp()) ON CONFLICT DO NOTHING;
  END IF;
 END LOOP;
END $$;
REVOKE ALL ON FUNCTION fulfillment.seed_manual_cvs(uuid,uuid,uuid) FROM PUBLIC;

-- The trigger can only read prerequisites and insert initial revisions. Its scope comes from
-- the identity definer's newly inserted creator row. Function-local settings restore on return.
CREATE ROLE commerce_manual_cvs_initializer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT USAGE ON SCHEMA control,inventory,pricing,fulfillment,identity TO commerce_manual_cvs_initializer;
GRANT SELECT(tenant_id,id,currency) ON control.stores TO commerce_manual_cvs_initializer;
GRANT SELECT(tenant_id,store_id,id,active,created_at) ON inventory.warehouses TO commerce_manual_cvs_initializer;
CREATE POLICY cvs_initializer_read ON control.stores FOR SELECT TO commerce_manual_cvs_initializer
 USING (tenant_id=current_setting('app.tenant_id',true)::uuid AND id=current_setting('app.store_id',true)::uuid);
CREATE POLICY cvs_initializer_read ON inventory.warehouses FOR SELECT TO commerce_manual_cvs_initializer
 USING (tenant_id=current_setting('app.tenant_id',true)::uuid AND store_id=current_setting('app.store_id',true)::uuid);
GRANT INSERT(tenant_id,store_id,pay_at_pickup_enabled,pay_at_pickup_max_twd,pay_at_pickup_max_open,version,updated_at)
 ON fulfillment.cvs_store_settings TO commerce_manual_cvs_initializer;
CREATE POLICY cvs_initializer_default ON fulfillment.cvs_store_settings FOR INSERT TO commerce_manual_cvs_initializer
 WITH CHECK(tenant_id=current_setting('app.tenant_id',true)::uuid AND store_id=current_setting('app.store_id',true)::uuid
  AND pay_at_pickup_enabled AND pay_at_pickup_max_twd=20000 AND pay_at_pickup_max_open=20 AND version=1
  AND enabled_chains=ARRAY['cvs_711','cvs_familymart','cvs_hilife','cvs_okmart']);
DO $$
DECLARE rel regclass; scope text := 'tenant_id=current_setting(''app.tenant_id'',true)::uuid AND store_id=current_setting(''app.store_id'',true)::uuid';
 actor text;
BEGIN
 FOREACH rel IN ARRAY ARRAY['pricing.markets','pricing.policy_versions','pricing.policy_heads',
   'fulfillment.service_versions','fulfillment.service_heads','fulfillment.allocation_versions',
   'fulfillment.allocation_heads','fulfillment.allocation_warehouses']::regclass[] LOOP
  actor := CASE WHEN rel IN ('pricing.markets'::regclass,'pricing.policy_versions'::regclass,
   'fulfillment.service_versions'::regclass,'fulfillment.allocation_versions'::regclass)
   THEN ' AND principal_id=current_setting(''app.principal_id'',true)::uuid' ELSE '' END;
  EXECUTE format('GRANT SELECT,INSERT ON %s TO commerce_manual_cvs_initializer',rel);
  EXECUTE format('CREATE POLICY cvs_initializer_read ON %s FOR SELECT TO commerce_manual_cvs_initializer USING (%s)',rel,scope);
  EXECUTE format('CREATE POLICY cvs_initializer_insert ON %s FOR INSERT TO commerce_manual_cvs_initializer WITH CHECK (%s%s)',rel,scope,actor);
 END LOOP;
END $$;
CREATE FUNCTION fulfillment.initial_store_manual_cvs() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog SET app.tenant_id='' SET app.store_id='' SET app.principal_id='' AS $$
BEGIN
 IF TG_RELID <> 'identity.initial_stores'::regclass THEN RAISE EXCEPTION 'invalid source'; END IF;
 PERFORM set_config('app.tenant_id',NEW.tenant_id::text,true),set_config('app.store_id',NEW.store_id::text,true),
  set_config('app.principal_id',NEW.principal_id::text,true);
 PERFORM fulfillment.seed_manual_cvs(NEW.tenant_id,NEW.store_id,NEW.principal_id);
 SET CONSTRAINTS fulfillment.allocation_complete,fulfillment.allocation_children_complete IMMEDIATE;
 SET CONSTRAINTS fulfillment.allocation_complete,fulfillment.allocation_children_complete DEFERRED;
 RETURN NULL;
END $$;
ALTER FUNCTION fulfillment.initial_store_manual_cvs() OWNER TO commerce_manual_cvs_initializer;
REVOKE ALL ON FUNCTION fulfillment.initial_store_manual_cvs() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.seed_manual_cvs(uuid,uuid,uuid) TO commerce_manual_cvs_initializer;
CREATE TRIGGER initial_store_manual_cvs AFTER INSERT ON identity.initial_stores
 FOR EACH ROW EXECUTE FUNCTION fulfillment.initial_store_manual_cvs();

-- One-time upgrade. For pre-onboarding stores the existing market owns the actor.
SELECT fulfillment.seed_manual_cvs(s.tenant_id,s.id,coalesce(i.principal_id,m.principal_id,o.principal_id))
FROM control.stores s LEFT JOIN identity.initial_stores i ON (i.tenant_id,i.store_id)=(s.tenant_id,s.id)
LEFT JOIN LATERAL (SELECT principal_id FROM pricing.markets WHERE (tenant_id,store_id)=(s.tenant_id,s.id)
 ORDER BY created_at,id LIMIT 1) m ON true
LEFT JOIN LATERAL (SELECT principal_id FROM identity.store_staff WHERE (tenant_id,store_id)=(s.tenant_id,s.id)
 AND role='owner' ORDER BY created_at,principal_id LIMIT 1) o ON true
WHERE s.currency='TWD' AND coalesce(i.principal_id,m.principal_id,o.principal_id) IS NOT NULL;

COMMENT ON ROLE commerce_manual_cvs_initializer IS 'internal/fulfillment: NOLOGIN scoped initializer for pricing, manual service, collection defaults and allocation inserts. No UPDATE, provider, payment or inventory write.';
COMMENT ON FUNCTION fulfillment.initial_store_manual_cvs() IS 'internal/identity onboarding trigger: trusted initial_store creator scope only, no login EXECUTE.';
COMMENT ON FUNCTION fulfillment.seed_manual_cvs(uuid,uuid,uuid) IS 'internal/fulfillment initialization: invoker rights, migration owner or onboarding trigger only. Preserves existing heads.';
