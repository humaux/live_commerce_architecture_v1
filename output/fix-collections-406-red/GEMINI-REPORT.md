# GEMINI-REPORT — FIX-COLLECTIONS-406 (RED phase)

Card: FIX-COLLECTIONS-406 — RED tests only (no product code). Issue #406 (P1, base r3/integration).
Branch: `unit/fix-collections-406`

## Defect Summary
In `internal/catalog/collections.go`, `collectionSelect` returns the live count and `readCollectionDetail` returns every match with a zero-based rank without capping automatic collections at 500 items. But `apps/admin/lib/catalog-v2-model.ts:199-226` (`parseCollection` / `parseCollectionDetail` / `parseCollectionPage`):
- rejects `product_count > 500`, `products.length > 500`, and `position > 499`;
- ignores `membership_mode`.

This causes `readCollections` to reject any collection page containing an automatic collection holding > 500 products, breaking `CollectionManager`, `ProductDocumentForm` references, and collection detail.

## Test Coverage Added (RED phase)
Added `tests/admin/catalog-v2-model.test.ts` covering:
1. `automatic collection with 501 products is accepted without truncation (#406)`:
   - `parseCollection`: automatic collection with 501 products accepted; count preserved at 501 without truncation.
   - `parseCollectionDetail`: 501 products with positions 0..500 accepted.
2. `manual collection with 501 products or position > 499 is rejected`:
   - `parseCollection`: manual collection with 501 products throws `/invalid/`.
   - `parseCollectionDetail`: manual collection with > 500 products or position > 499 throws `/invalid/`.
3. `collection switched automatic -> manual with valid manual shape is accepted`:
   - collection with `membership_mode: "manual"` and <= 500 products accepted.
4. `empty automatic and empty manual collections are accepted`:
   - automatic and manual collections with 0 products and empty products array accepted.
5. `collection page mixing automatic 501 collection with manual collections is accepted`:
   - `parseCollectionPage`: page containing manual collections and one automatic 501 collection is accepted in full.
6. Boundary & regression:
   - manual collection at cap (500 products, positions 0..499) accepted.
   - legacy collection without `membership_mode` defaults to manual bounds.

Registered the new suite in `scripts/dev/test-node.sh` and regenerated `output/ci-select-backend-browser/browser-ownership.json` via `node scripts/dev/derive-browser-ownership.mjs` (verified with `--check`).

## Failure Classification and Changes

| failing job / test | class | what changed (file:line) |
|---|---|---|
| `test-node.sh` / `tests/admin/catalog-v2-model.test.ts` (`automatic collection with 501 products is accepted without truncation (#406)`) | b | `tests/admin/catalog-v2-model.test.ts:25-63` — added failing test asserting `parseCollection` and `parseCollectionDetail` accept automatic collection with 501 products (positions 0..500) and preserve count (reproduces product defect in `apps/admin/lib/catalog-v2-model.ts:205,212,216`) |
| `test-node.sh` / `tests/admin/catalog-v2-model.test.ts` (`collection page mixing automatic 501 collection with manual collections is accepted`) | b | `tests/admin/catalog-v2-model.test.ts:207-268` — added failing test asserting `parseCollectionPage` accepts a page containing an automatic collection of 501 products alongside manual collections (reproduces product defect in `apps/admin/lib/catalog-v2-model.ts:221-226`) |
| `scripts/dev/test-node.sh:45` | a | `scripts/dev/test-node.sh:45` — registered `tests/admin/catalog-v2-model.test.ts` under Node unit gate runner |
| `output/ci-select-backend-browser/browser-ownership.json:28,789` | a | `output/ci-select-backend-browser/browser-ownership.json:28,789` — regenerated ownership artifact via `node scripts/dev/derive-browser-ownership.mjs` to track `tests/admin/catalog-v2-model.test.ts` and `scripts/dev/test-node.sh` hash updates |

STATUS: DONE
