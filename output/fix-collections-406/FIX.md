# FIX-COLLECTIONS-406 — Implementation Resolution

## Defect and Root Cause
In `internal/catalog/collections.go`, automatic collections evaluate membership dynamically via rule match against active products with store-currency SKUs. Unlike manual collections which are capped at 500 items (`maxCollectionProducts = 500`, position CHECK `0..499`), automatic collections have no 500-item cap (per `contracts/storefront-v2.md` lines 450-454 and `contracts/catalog-inventory-openapi.json` lines 8485-8489): `collectionSelect` counts all matching active products, and `readCollectionDetail` returns all matches with zero-based ranks (`(row_number() OVER (...) - 1)::integer`).

However, `apps/admin/lib/catalog-v2-model.ts`:
1. `parseCollection` did not parse `membership_mode` and unconditionally checked `int(r.product_count, 0, limits.collectionProducts)` (0..500). Any collection with > 500 products threw `Error: invalid`.
2. `parseCollectionDetail` unconditionally enforced `list(r.products, limits.collectionProducts)` and `int(m.position, 0, limits.collectionProducts - 1)` (0..499). Any automatic collection detail with > 500 products threw `Error: invalid`.
3. `parseCollectionPage` mapped each item through `parseCollection`, causing `readCollections` to throw `Error: invalid` on any page containing an automatic collection holding > 500 products.

Consequently:
- `CollectionManager` navigation and list failed when loading collections containing an automatic collection with > 500 products;
- `ProductDocumentForm` threw during initial `Promise.all([readCollections, readWarehouses])`, leaving `referencesReady` permanently false and blocking the editor;
- Collection detail view for automatic collections with > 500 products failed in `readCollection` with `c.unavailable`.

## CI Run and Job IDs (Causal RED Evidence)
The RED phase added unit tests reproducing the defect (`tests/admin/catalog-v2-model.test.ts`), registered them in `scripts/dev/test-node.sh`, and pushed commit `76b50e02dfc6b07168b608941c13f5dff03ea806`.

CI Failures:
- **Foundation checks workflow**:
  - Run ID: `41209062858`
  - Job ID: `124560035845` (`foundation`)
  - URL: `https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41209062858/job/124560035845`
  - Failing tests:
    - `tests/admin/catalog-v2-model.test.ts:25:1`: `automatic collection with 501 products is accepted without truncation (#406)` (failed at `int` in `parseCollection` at `apps/admin/lib/catalog-v2-model.ts:205:20`)
    - `tests/admin/catalog-v2-model.test.ts:194:1`: `collection page mixing automatic 501 collection with manual collections is accepted` (failed at `int` in `parseCollection` at `apps/admin/lib/catalog-v2-model.ts:205:20`)
- **Gates (GitHub runners) workflow**:
  - Run ID: `41209062779`
  - Job ID: `124560981768` (`gate (shard:unit)`)
  - URL: `https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41209062779/job/124560981768`

## Code Changes
File: `apps/admin/lib/catalog-v2-model.ts`
- Lines 1-3: Added standard header comment (`Purpose`, `Depends on`, `Used by`) preserving existing docstrings.
- Line 72: Defined `CollectionMembershipMode = "manual" | "automatic"`.
- Line 83: Added `membership_mode: CollectionMembershipMode` to `Collection` type.
- Lines 206-218 (`parseCollection`):
  - Read `membership_mode` strictly: `oneOf(r.membership_mode, ["manual", "automatic"] as const)`.
  - For backward compatibility with legacy collections predating `membership_mode`, `undefined` defaults strictly to `"manual"`. Any other value (such as `null`, non-matching strings, or non-strings) is rejected as invalid.
  - Bound `product_count` to a non-negative integer: manual retains `limits.collectionProducts` (500), while automatic accepts unbounded non-negative integers up to `Number.MAX_SAFE_INTEGER`. Count is preserved without silent truncation.
- Lines 221-235 (`parseCollectionDetail`):
  - Retained `limits.collectionProducts` (500) and position `0..499` for manual collections.
  - For automatic collections, accept products as returned (up to `Number.MAX_SAFE_INTEGER`) with positions validated as zero-based ranks (`int(m.position, 0, Number.MAX_SAFE_INTEGER)`).

## Consumer Verification
1. `CollectionManager` (`apps/admin/components/CollectionManager.tsx`):
   - List view (`readCollections`): `items.map` reads `item.product_count` via `c.count(item.product_count)`, formatting counts cleanly without truncation or assertion failures.
   - Detail view (`readCollection`): `CollectionDetail` loads successfully with > 500 members. Products are rendered in rank order in `ol.collection-members`.
   - Type-checks cleanly with strict TypeScript compiler.
2. `ProductDocumentForm` (`apps/admin/components/ProductDocumentForm.tsx`):
   - References hook (`readCollections`): `Promise.all` resolves cleanly; `setCollections(col.items)` populates checkboxes for all collections including automatic collections > 500 items; `referencesReady` becomes true without blocking the form.
   - Type-checks cleanly with strict TypeScript compiler.
3. Collection detail view:
   - Evaluated in `CollectionForm` and `Members`; preserves exact existing API contract with no new endpoints introduced.

## Contract Integrity Check
- `internal/catalog/collections.go`: Lines 69-83 (`validCollectionMode`, `Collection` struct), 146-154 (`collectionSelect`), 387-421 (`readCollectionDetail`).
- `contracts/storefront-v2.md`: Section A lines 430-460.
- `contracts/catalog-inventory-openapi.json`: Lines 8485-8510.
- Result: 100% agreement across backend Go models, SQL schemas, contracts, and frontend TypeScript wire models. No backend changes required.
