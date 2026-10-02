-- 0109 product-core (docs/delivery/units/product-editor.md, section f integrator rulings 2026-10-02).
-- Owning packages: internal/catalog (SKU model, document command, images, product list) and internal/checkout
-- (Begin untracked path), with contract amendments in contracts/catalog-inventory-v1.md and
-- contracts/buyer-checkout-v1.md.
-- Adds: (1) catalog.skus.inventory_tracked + max_per_order (A6: an untracked SKU is never locked or deducted at
-- checkout Begin and is bounded only by max_per_order); (2) widens the product-image position CHECK from 0..7 to
-- 0..11 (image cap 8 -> 12); (3) column grants so commerce_runtime can write the two new SKU columns.
--
-- A6 shape (why the CHECK is asymmetric): a tracked SKU keeps the no-oversell invariant, so it carries no per-order
-- cap (max_per_order is NULL); an untracked SKU is never locked/deducted, so it MUST carry max_per_order 1..999 to
-- bound a single order. The `IS NOT NULL` on the untracked disjunct makes the CHECK 3VL-safe: without it,
-- inventory_tracked=false AND max_per_order=NULL evaluates the whole CHECK to NULL, which SQL treats as pass, so an
-- untracked SKU could be stored with no cap and become unbounded at checkout. DEFAULT true keeps every pre-A6 SKU
-- (which has balances) tracked. Forward-only: migrate.go
-- never edits or re-runs an applied file, so this is a new numbered file, not a change to 0002/0082.
--
-- Non-goals: no change to the buyer storefront definers (the merchant list reads inventory_tracked directly as
-- commerce_runtime); no per-order cap for tracked SKUs; no TWD whole-dollar DB CHECK (that is a Go-only check,
-- refusal code amount_not_whole_twd, so legacy non-whole TWD rows do not block this migration).

-- (1) A6 inventory tracking + per-order cap.
ALTER TABLE catalog.skus
    ADD COLUMN inventory_tracked boolean NOT NULL DEFAULT true,
    ADD COLUMN max_per_order integer,
    ADD CONSTRAINT skus_inventory_max_per_order CHECK (
        (inventory_tracked AND max_per_order IS NULL)
        OR (NOT inventory_tracked AND max_per_order IS NOT NULL AND max_per_order BETWEEN 1 AND 999));

-- commerce_runtime is the only writer of these columns; the checkout roles already hold table-level SELECT on
-- catalog.skus (0013), which covers new columns automatically, so no new checkout grant is needed.
GRANT INSERT(inventory_tracked,max_per_order), UPDATE(inventory_tracked,max_per_order) ON catalog.skus TO commerce_runtime;

-- (2) Image cap 8 -> 12: widen the position CHECK (0082). The deferred UNIQUE product_images_position already permits
-- any count; only this CHECK pins the top position. The auto-generated column-check name follows the codebase
-- convention ({table}_{column}_check, see 0016 command_results_operation_check).
ALTER TABLE catalog.product_images DROP CONSTRAINT product_images_position_check;
ALTER TABLE catalog.product_images ADD CONSTRAINT product_images_position_check CHECK (position BETWEEN 0 AND 11);

-- (3) Documentation (PROCESS section 5).
COMMENT ON COLUMN catalog.skus.inventory_tracked IS
 'internal/catalog + internal/checkout (A6): true = tracked, checkout Begin locks and deducts stock with the no-oversell invariant and no per-order cap; false = untracked, checkout Begin skips the lock/deduct and enforces max_per_order. Written only by commerce_runtime through the product document command.';
COMMENT ON COLUMN catalog.skus.max_per_order IS
 'internal/checkout (A6): per-order quantity cap, 1..999, REQUIRED for an untracked SKU (inventory_tracked=false) and NULL for a tracked one. Enforced at checkout Begin as PT422 max_per_order_exceeded instead of a stock lock.';
