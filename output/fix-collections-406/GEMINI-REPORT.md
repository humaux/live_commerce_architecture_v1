# GEMINI-REPORT — FIX-COLLECTIONS-406

Card: FIX-COLLECTIONS-406 — implementation. Issue #406 (P1).
Branch: `unit/fix-collections-406`

## Failure Classification and Changes

| failing job / test | class | what changed (file:line) |
|---|---|---|
| Foundation checks (`41209062858` / `124560035845`), Gates `gate (shard:unit)` (`41209062779` / `124560981768`) / `tests/admin/catalog-v2-model.test.ts:25` (`automatic collection with 501 products is accepted without truncation (#406)`) | b | `apps/admin/lib/catalog-v2-model.ts:72,83,206-218,221-235` — defined `CollectionMembershipMode = "manual" \| "automatic"` and added to `Collection` type; parsed `membership_mode` strictly (defaulting legacy undefined to `"manual"`); allowed unbounded non-negative integer `product_count` for automatic collections without truncation while keeping 500 cap for manual; allowed unbounded `products` length and zero-based rank positions for automatic detail while keeping 500 items and position 0..499 bounds for manual |
| Foundation checks (`41209062858` / `124560035845`), Gates `gate (shard:unit)` (`41209062779` / `124560981768`) / `tests/admin/catalog-v2-model.test.ts:194` (`collection page mixing automatic 501 collection with manual collections is accepted`) | b | `apps/admin/lib/catalog-v2-model.ts:206-218` — `parseCollection` accepts automatic collections holding > 500 products without throwing `/invalid/`, allowing mixed collection pages in `parseCollectionPage` to parse without error |
| Code header requirement | a | `apps/admin/lib/catalog-v2-model.ts:1-3` — added Purpose / Depends on / Used by header comment while preserving existing commentary |

STATUS: DONE
