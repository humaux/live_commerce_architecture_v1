# Independent review — product-media-v2 backend (PM-B)

- Reviewer: Claude Opus 5.5 (read-only; not the author). Branch `unit/product-media-v2-backend` @ `7918ecbb`, diff `r3/integration...HEAD`.
- Brief `docs/delivery/units/product-media-v2.md`; delivery `output/product-media-v2-backend/DELIVERY.md`.
- Re-run by reviewer (DB-free only, PG suites not run on the shared machine):
  - `go build ./...` exit 0.
  - `go vet` on catalog, httpapi, buyerhttp, storefront, attribution, merchanttools and tests/foundation: exit 0.
  - `gofmt -l internal tests`: empty.
  - `go test ./internal/catalog ./internal/attribution ./internal/httpapi` (unit subset): ok.
  - `check-headers.sh`: exit 0.
  - node `product-media`, `backend-parity`, `shop`, `purchase` (storefront) and `product-media-model`, `backend-parity` (admin): 48/48 pass, exit 0.
- Evidence class of this review: static reading + DB-free re-run. The REAL_PG claims (TestPMv2*, green3.log) are the author's (E0 for me); CI must re-run them.

## Verdict: **MERGE**

There are no P0 or P1 findings. Two P2s change the migration text (P2-1, P2-2). They cost nothing now, but the 0149 checksum freezes on first apply, so I recommend folding them in before merge. The rest can follow later.

## Checklist results (what holds, with evidence)

1. **Migration 0149 data safety**
   - Forward-only, single transaction: the runner applies all versions in one tx (`migrations/migrate.go:93-128`).
   - Nothing is deleted: the data move is a single `UPDATE ... SET role='detail', position=position-4 WHERE position>=4` with a count assertion (`0149:36-45`).
   - 9 images become 4 main + 5 detail: pinned by `TestPMv2MigrationNineImages` (`tests/foundation/product_media_v2_test.go:132-230`). The test holds back 0149 and everything after it, seeds data, applies, then checks that a second Apply leaves the ledger unchanged.
   - Constraint order is correct:
     - The new UNIQUE `(role,position)` is added first (`:28`).
     - The old UNIQUE is dropped (`:30`). It has to go before the move, otherwise main 0 and detail 0 would collide.
     - The new per-role CHECK is added `NOT VALID` (`:31`), then the rows move, then it is validated (`:48`).
     - Only after that is the old `0..11` CHECK dropped (`:49`).
   - Constraint names match 0082/0109: `product_images_position` and `product_images_position_check`.
   - Locks and rewrites are cheap:
     - `ADD COLUMN ... DEFAULT 'main'` uses the fast default, so there is no table rewrite.
     - The UPDATE rewrites only the heap tuple. The ≤2 MiB bytea is already TOASTed and its pointer is reused.
     - VALIDATE scans the heap only.
     - ACCESS EXCLUSIVE on `product_images`/`products` is held for the length of a pilot-sized migration.
   - Function bodies: the replaced definers' latest bodies live in 0082/0086; no migration >0109 redefines them (`git grep`).
     - `buyer_v2_products` diffs from 0086 only by `AND i.role='main'`.
     - `buyer_v2_product` diffs only by the role filters and the new keys.
     - So no later patch is regressed.
   - Ordering against 0150/0151: the two in-flight migrations (`w4-s2-platform-settlement/0150`, `w3-04b-sold-out-reply/0151`) touch neither `product_images` nor any function 0149 replaces. No CREATE OR REPLACE collision.
2. **Tenant/store isolation**
   - Option-value links are enforced in SQL: the composite FK `(tenant,store,product,image,'sku')` points at the UNIQUE `(tenant,store,product,id,role)` (`0149:66-68`).
   - `TestPMv2SQLStructure` (`:290-304`) refuses each bad link:
     - another product: 23503
     - another store: 23503
     - a main-role image: 23503
     - a value not on the axis: 23514
     - a second image for one value: 23505
   - The new table has `FORCE RLS` and is scoped to `commerce_runtime`. The definer owner has a read policy. Buyers have no grant (pinned 42501, `:800`).
   - Every new function has `SET search_path=pg_catalog` and schema-qualified references.
   - `buyer_sku_images` and `buyer_feed_variant_images` are owned by `commerce_catalog_media`, revoked from PUBLIC, with EXECUTE for buyers only.
   - `sku_option_image` is INVOKER with EXECUTE only for the definer owner.
3. **Caps and concurrency**
   - The structural cap is the per-role CHECK plus the deferred UNIQUE `(role,position)`. Two racing uploads cannot produce 5 mains, because only positions 0..3 are legal and each is unique.
   - In Go, `editableProduct`/`lockProduct` (`catalog.go:558-577`, `SELECT ... FOR UPDATE`) serialize every image mutation of a product, so the count-then-insert does not race.
   - Move, reorder and delete each run in one command transaction (`image_roles.go:91-137`, `images.go:338-400`).
4. **Upload validation is unchanged:** `SniffImage` is untouched (magic bytes, 2 MiB, GIF/SVG refused).
   - The new query params are allowlisted by key (`httpapi/images.go:80-93`), and role/value pairing is checked in the domain (`images.go:161`). See P2-3 on ordering.
5. **Buyer API**
   - Every definer filters to the published store and active products. A draft product returns 404 (`test :617-621`).
   - `option_images` and `variants[].image_id` resolve only through same-product sku images (FK).
   - An old storefront ignores the new keys.
   - A new storefront tolerates a missing `detail_images` (`shop-contract.ts:128`).
6. **Meta feed**
   - Without a variant image: `image_link` = main[0] and `additional_image_link` = main[1..3].
   - With a variant image: `image_link` = the variant image and `additional_image_link` = main[0..3].
   - Detail images never enter the feed (`feed.go:189-213`).
   - The CSV quoting of the comma list is handled by `encoding/csv`. Pinned by `TestImageLinks` and `TestPMv2FeedLinks`.
7. **Test changes match the owner decision.** I found no hidden weakening:
   - PE06 cap goes 12→4 and catalog_media lifecycle goes 12→4 (main cap). The "exactly 2 MiB at position 7" case moves to position 3, the last legal main slot. Each change keeps a 409 assertion on the next upload and a row-count check.
   - The ACL pin grows by `role`, matching the new grant.
   - Key lists grow. Nothing was removed.
   - R2 count goes 73→74.
   - The merge with `storefront-image-cap` replaces `MAX_PRODUCT_IMAGES` parity with `MaxMainImages`/`MaxDetailImages` parity. The parser parity tests still read the Go constants from source, and the byte-cap parity survives (`tests/admin/backend-parity.test.ts:38-44`).
8. **Headers:** new and changed files carry Purpose/Depends on/Used by. `check-headers.sh` exits 0. Doc comments are present on the exported Go symbols and at the definer call sites.

## Findings

### P0 — none

### P1 — none

### P2-1 `SET CONSTRAINTS ALL IMMEDIATE` leaks into every later migration in the same Apply transaction
- **Evidence:** `migrations/0149_product_media_v2.sql:46`. The runner applies every pending version in one transaction (`migrations/migrate.go:125`).
- **Why it matters:**
  - On a DB that is behind, and on every fresh CI DB, all migrations after 0149 run with every deferrable constraint forced to IMMEDIATE.
  - On a DB already at 0149 they run with the normal deferral.
  - A future migration that renumbers rows under a deferrable UNIQUE would then behave differently on fresh and incremental applies.
  - It is latent today: 0150 and 0151 use no deferrable constraints.
- **Fix:**
  - Replace the line with `SET CONSTRAINTS catalog.product_images_role_position IMMEDIATE;`.
  - After the `VALIDATE`/`DROP` lines, add `SET CONSTRAINTS catalog.product_images_role_position DEFERRED;`.
  - This must happen before merge, because the checksum freezes on first apply.

### P2-2 The data move assumes historical positions are contiguous
- **Evidence:** `0149:40` (`position=position-4`).
- **Why it matters:** if any pilot product has a gap (for example 0,1,2,5), its detail rows start at 1. The next detail upload then computes position = count = 1, collides on the deferred UNIQUE at commit, and returns a 500 instead of a 409. Contiguity has been maintained since 0082 (delete closes gaps, reorder is a permutation), so the risk is low, but this is a live DB.
- **Fix (cheap hardening):** renumber with `row_number() OVER (PARTITION BY tenant_id,store_id,product_id ORDER BY position)`. Alternatively, run a pre-flight on the pilot DB, `SELECT product_id FROM catalog.product_images GROUP BY 1 HAVING max(position)+1 <> count(*)`, and expect 0 rows.

### P2-3 Upload role value is validated only after the body is read and decoded
- **Evidence:** `internal/httpapi/images.go:44-50`. `imageUploadQuery` checks keys only; `role=bogus` reaches `uploadRoute`, which reads ≤2 MiB and runs `MakeImageSizes` (a full decode) before `UploadImage` returns ErrInvalid (`images.go:161`).
- **Why it matters:** an authenticated merchant can burn decode CPU with typos. The outcome is still a correct 422.
- **Fix:** in `imageUploadQuery`, also require `role ∈ {main,detail,sku}`, `option_value` non-empty only for sku, and `utf8.RuneCountInString(option_value) ≤ 40`.

### P2-4 The admin validator counts UTF-16 units; Go and SQL count runes
- **Evidence:**
  - `apps/admin/lib/image-list-contract.ts:45-46,56` uses `.length ≤ 30/40` for `option_name`, `option_value` and `image_axis`.
  - Go `plainText` counts runes (`internal/catalog/options.go:101-103`), and SQL uses `char_length`.
- **Why it matters:** an axis name or value with astral characters (emoji) can be valid in Go and SQL but exceed the JS limit.
  - `validImageList` then fails and `images-client` returns 503 for the whole list.
  - The merchant loses even the main-photo manager for that product.
  - This is the same class of bug that `design.ts`'s `clip` fixes in this very branch.
- **Fix:** compare `[...s].length`, and add a parity case with an emoji value.

### P2-5 Deploy-window and older-client compatibility
- **Evidence:**
  - The storefront parser rejects more than 4 `images` (`shop-contract.ts:126`). This is acknowledged as DELIVERY risk (1).
  - The admin `validImage` checks exact keys, including the new `role` key.
- **Why it matters:**
  - On a single host, `deploy/compose.yml:78-82,345` makes the storefront wait for `migrate`, so storefront-first cannot happen there.
  - Two-host deploys rely on the runbook order.
  - An old admin (or an old API binary) against the migrated DB fails during the swap window:
    - An old admin refuses the list because of the new `role` key.
    - An old API's upload computes `position = count(all)`, which hits CHECK 23514.
- **Fix:**
  - Note in the release runbook: migrate, then api, then admin/storefront.
  - Optionally, have the storefront slice `images` to the cap instead of rejecting the page (pilot lesson: never fail the whole page on a cap).

### P2-6 Small semantic gaps (record only)
- `MoveImage` can move every main image to detail, which leaves an active product without a cover (`image_link` becomes empty). The brief says "≥1 to publish, unchanged rule", but no such rule existed, so nothing regresses. The 1–4 rule is not enforced either.
- The detail 6× ratio is unenforceable for WebP (no dimensions, `images.go:63-65`). It is documented in the code.
- `merchanttools/csvexport.go:83` `image_count` now counts all roles. It is informational only.
- `tests/foundation/product_media_v2_test.go` has 803 lines, above the ~600-line guidance in PREAMBLE §3.
- There is no explicit parallel-upload test. The cap is structural, so this is acceptable.
