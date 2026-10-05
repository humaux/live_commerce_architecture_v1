-- Purpose: One-time backfill of a default single-warehouse allocation for enabled delivery services that have none (delivery-allocation P0).
-- Depends on: tables fulfillment.services/heads, fulfillment.allocation_*, inventory.warehouses (adds created_at IF NOT EXISTS).
-- Used by: cmd/migrate (forward-only, checksummed); tests/foundation TestDeliveryAllocationBackfillMigration.
-- delivery-allocation P0 backfill: a merchant who enabled a delivery service before this fix has no
-- allocation row, so the buyer never saw the option (checkout.ListOptions INNER JOINs allocation_heads).
-- Forward-only, checksummed, repeatable no-op: only enabled services that still lack an allocation head
-- are filled, each with a single-warehouse head pointing at the store's default warehouse (first active
-- by creation order; the deterministic id tiebreak covers warehouses created before this column existed).
-- Draft number 0117; the integrator assigns the final number.

-- The runtime needs a stable creation order to pick "the first warehouse" for multi-warehouse stores.
ALTER TABLE inventory.warehouses ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT clock_timestamp();

-- 1) One immutable version per missing allocation. Reuse the service's own principal for provenance
--    (service_versions.principal_id is already constrained to a valid identity.memberships row).
INSERT INTO fulfillment.allocation_versions(
    tenant_id,store_id,market_id,country,code,version,service_version,warehouse_count,principal_id)
SELECT sv.tenant_id, sv.store_id, sv.market_id, sv.country, sv.code, 1, sv.version, 1, sv.principal_id
FROM fulfillment.service_heads sh
JOIN fulfillment.service_versions sv
  ON (sv.tenant_id,sv.store_id,sv.market_id,sv.country,sv.code,sv.version) =
     (sh.tenant_id,sh.store_id,sh.market_id,sh.country,sh.code,sh.current_version)
WHERE sv.enabled
  AND NOT EXISTS (SELECT 1 FROM fulfillment.allocation_heads ah
      WHERE ah.tenant_id=sv.tenant_id AND ah.store_id=sv.store_id AND ah.market_id=sv.market_id
        AND ah.country=sv.country AND ah.code=sv.code)
  AND EXISTS (SELECT 1 FROM inventory.warehouses w
      WHERE w.tenant_id=sv.tenant_id AND w.store_id=sv.store_id AND w.active);

-- 2) One default-warehouse child per backfilled version (position 1 of 1).
INSERT INTO fulfillment.allocation_warehouses(
    tenant_id,store_id,market_id,country,code,version,position,warehouse_id)
SELECT sv.tenant_id, sv.store_id, sv.market_id, sv.country, sv.code, 1, 1,
       (SELECT w.id FROM inventory.warehouses w
        WHERE w.tenant_id=sv.tenant_id AND w.store_id=sv.store_id AND w.active
        ORDER BY w.created_at, w.id LIMIT 1)
FROM fulfillment.service_heads sh
JOIN fulfillment.service_versions sv
  ON (sv.tenant_id,sv.store_id,sv.market_id,sv.country,sv.code,sv.version) =
     (sh.tenant_id,sh.store_id,sh.market_id,sh.country,sh.code,sh.current_version)
WHERE sv.enabled
  AND NOT EXISTS (SELECT 1 FROM fulfillment.allocation_heads ah
      WHERE ah.tenant_id=sv.tenant_id AND ah.store_id=sv.store_id AND ah.market_id=sv.market_id
        AND ah.country=sv.country AND ah.code=sv.code)
  AND EXISTS (SELECT 1 FROM inventory.warehouses w
      WHERE w.tenant_id=sv.tenant_id AND w.store_id=sv.store_id AND w.active);

-- 3) The head itself. Heads are inserted last so the first two statements still see "no head".
INSERT INTO fulfillment.allocation_heads(tenant_id,store_id,market_id,country,code,current_version)
SELECT sv.tenant_id, sv.store_id, sv.market_id, sv.country, sv.code, 1
FROM fulfillment.service_heads sh
JOIN fulfillment.service_versions sv
  ON (sv.tenant_id,sv.store_id,sv.market_id,sv.country,sv.code,sv.version) =
     (sh.tenant_id,sh.store_id,sh.market_id,sh.country,sh.code,sh.current_version)
WHERE sv.enabled
  AND NOT EXISTS (SELECT 1 FROM fulfillment.allocation_heads ah
      WHERE ah.tenant_id=sv.tenant_id AND ah.store_id=sv.store_id AND ah.market_id=sv.market_id
        AND ah.country=sv.country AND ah.code=sv.code)
  AND EXISTS (SELECT 1 FROM inventory.warehouses w
      WHERE w.tenant_id=sv.tenant_id AND w.store_id=sv.store_id AND w.active);
