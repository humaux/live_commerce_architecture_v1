-- Purpose: scoped, read-only buyer inventory hints for claim direct checkout (CDC01/CDC02).
-- Depends on: catalog.skus A6 tracking, inventory.balances, canonical buyer GUCs and RLS.
-- Used by: claims.PreviewLink/RedeemLink through commerce_buyer_runtime; Begin remains stock authority.
-- No reservation, ledger write, new table or raw inventory grant to a login role.

GRANT USAGE ON SCHEMA catalog TO commerce_inventory_writer;
GRANT SELECT(tenant_id,store_id,id,inventory_tracked) ON catalog.skus TO commerce_inventory_writer;
GRANT USAGE ON SCHEMA inventory TO commerce_buyer_runtime;

CREATE FUNCTION inventory.buyer_sku_availability(p_skus uuid[])
RETURNS TABLE(sku_id uuid, tracked boolean, available bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid;
BEGIN
 IF current_setting('transaction_isolation') <> 'read committed'
  OR coalesce(current_setting('app.tenant_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.store_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.buyer_session_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR current_setting('app.principal_id',true) IS DISTINCT FROM ''
  OR p_skus IS NULL OR cardinality(p_skus)>50 OR coalesce(array_ndims(p_skus),1)<>1 THEN
  RAISE EXCEPTION 'invalid buyer availability request' USING ERRCODE='22023';
 END IF;
 IF array_position(p_skus,NULL) IS NOT NULL THEN
  RAISE EXCEPTION 'invalid buyer availability request' USING ERRCODE='22023';
 END IF;
 v_tenant:=current_setting('app.tenant_id')::uuid;
 v_store:=current_setting('app.store_id')::uuid;
 -- Read-only snapshot across this store's warehouses. RLS plus explicit scope fence
 -- keeps an arbitrary UUID list from becoming a cross-store inventory oracle.
 RETURN QUERY SELECT s.id,s.inventory_tracked,
   coalesce(sum(b.on_hand-b.reserved-b.allocated-b.unavailable),0)::bigint
 FROM catalog.skus s
 LEFT JOIN inventory.balances b ON b.tenant_id=s.tenant_id AND b.store_id=s.store_id AND b.sku_id=s.id
 WHERE s.tenant_id=v_tenant AND s.store_id=v_store AND s.id=ANY(p_skus)
 GROUP BY s.id,s.inventory_tracked;
END $$;
ALTER FUNCTION inventory.buyer_sku_availability(uuid[]) OWNER TO commerce_inventory_writer;
REVOKE ALL ON FUNCTION inventory.buyer_sku_availability(uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inventory.buyer_sku_availability(uuid[]) TO commerce_buyer_runtime;
COMMENT ON FUNCTION inventory.buyer_sku_availability(uuid[]) IS
 'internal/claims Direct checkout: buyer-scoped A6 tracking and current available stock for <=50 SKU IDs; EXECUTE only commerce_buyer_runtime; read-only hint, never a stock hold or checkout authority.';
