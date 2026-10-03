# REVIEW — product-core (backend unit, adversarial, independent)

Reviewer: K3 (test_worker + security_reviewer), not the implementer.
Base: `39bb8ecf` (unit/product-core incl. r3/integration + home-cod rebuild of post_river/0021).
Date: 2026-10-02. Method: read by symbol (migrations 0109, post_river/0021, internal/catalog
document.go/productlist.go/images.go/catalog.go, internal/checkout checkout.go planLocked+Begin,
internal/command, httpapi routes, implementer tests PE01–PE11), traced every order-placement path
(storefront Begin, manual order pipeline, claim link → cart → Begin) against the A6/§f rulings.

## Verdict

One P1 (cross-unit gate conflict), no P0. The money/stock core holds: every order path funnels into
`checkout.begin_hold` (post_river/0021), whose tracked-only conservation JOIN + PT422
`max_per_order_exceeded` guard close the A6 surface; the document save is one `command.Run`
transaction with the ledger UNIQUE and keyword UNIQUE as race backstops; expected_version is both
pre-checked under the product row lock and re-guarded in the UPDATE's WHERE. 6 P2 below.

### P1-0 — The unit's own migration 0109 breaks catalog-core's pre-existing gates (merge-blocker)
`bash scripts/dev/test-focused.sh '^(TestProductEditor|TestCatalog|…)'` exits 1 with 3 failures,
all caused by 0109's intended changes, none by test flakiness (evidence
`output/product-core-tests/final-focused.log`):
- `TestCatalogMediaSchemaSurface` + `TestCatalogMediaLifecycle` (`tests/foundation/catalog_media_test.go:609,929`):
  these catalog-core gates assert the OLD image contract — direct `INSERT position=8` must violate the
  CHECK, the 9th photo must be refused 409. 0109 widens `product_images_position_check` to 0..11 and the
  Go cap to 12 (the §f ruling), so both assertions now fail. The gates encode the pre-amendment contract
  and were never re-baselined when the image-cap-12 amendment landed.
- `TestCatalogCoreCC01PopulatedUpgrade` (`tests/foundation/catalog_core_upgrade_test.go:192`):
  the populated-upgrade guard digests every `catalog.skus` column except `option_values`/`compare_at_minor`
  before/after the 0086→head upgrade; 0109 adds `inventory_tracked`/`max_per_order` (with `DEFAULT true`)
  to populated rows, so the digest legitimately changes and the guard fires. The guard's allow-list of
  "documented changes" (commit 04c02e56: 0086/0088/0089/0092/0094/0097) does not include 0109.
Failure scenario (process, not runtime): the branch cannot merge while these gates are red, and blindly
re-baselining them is exactly the "rewrite fixtures to pass gates" anti-pattern — the integrator must
amend the catalog-core gates explicitly (exclude the two 0109-owned columns from the CC01 digest like the
earlier amendments; re-baseline media tests to cap 12 with a 13th-photo refusal). Per my mandate I did not
touch another unit's acceptance gates; I record the conflict with evidence.

## What was attacked and held (verified by reading + tests in product_document_k3_test.go)

- **Tracked oversell**: Go `planLocked` row-locks balances (`inventory.lock_balance`) in a stable
  order, `begin_hold` re-checks `on_hand-reserved-allocated-unavailable >= qty` per plan line in the
  same tx. 50-buyer/1-unit test: exactly 1 win (`TestProductEditorK3TrackedLastUnitFiftyBuyers`).
- **Untracked never locked, bounded per order**: `planLocked` drops untracked lines from the plan
  (checkout.go:760); `begin_hold` demand CTE joins `catalog.skus … AND s.inventory_tracked`
  (0021:303) and refuses `q.quantity > s.max_per_order` with PT422 (0021:338-341). All-untracked
  order = empty plan, bound 0..800. 100 concurrent buyers all succeed, zero ledger/reservation-line
  rows for the SKU (`TestProductEditorK3UntrackedConcurrentOrders`).
- **Every order path covered**: manual orders (merchanttools/manual.go:533 `m.checkout.Begin`) and
  claim-link carts (buyerhttp → storefront cart → Begin) both terminate in `begin_hold`; there is no
  second stock-committing path (`INSERT INTO checkout.orders` exists only in begin_hold).
- **Flag flip mid-checkout fails safe**: READ COMMITTED makes the Go plan read and the begin_hold
  conservation read see different committed states of `inventory_tracked`; both flip directions
  produce the FULL JOIN mismatch → PT409, never a silent lock skip or double count.
- **One-transaction document save**: product/options/SKUs/stock ADJUST/keyword/collections + audit +
  command result in one `command.Run` fn; PE01's mid-flight bogus-collection rollback is real.
- **Idempotency**: advisory lock per (tenant,store,operation,key), hash compare → replay / 409;
  concurrent same-key replay and same-key/different-bytes pinned
  (`TestProductEditorK3ConcurrentIdempotentReplay`).
- **expected_version**: `applyProductEdit` checks `cur.Version != in.ExpectedVersion` after
  `lockProduct` FOR UPDATE, and the UPDATE re-guards `version=$11` (document.go:364,374-376).
  Concurrent same-version edits: one 200, one 409 (`TestProductEditorK3ConcurrentSameVersionEdits`).
- **Axis change**: omitted active SKUs archived (row kept, order history intact), combination index
  slot freed, recreate works (PE08).
- **TWD whole-dollar on the document path**: `checkWholeTWD` refuses `price_minor % 100 != 0` and
  non-whole compare-at with 422 `amount_not_whole_twd` (catalog.go:630) — create and edit alike.
- **Image cap 12**: product row lock serializes the count check (images.go:132-148); CHECK widened to
  0..11 in 0109; PE06 covers 12 ok / 13 refused / order persisted.
- **Bulk status**: ≤100, per-item results, `live_window_open` per item without aborting the rest,
  per-item `not_found` for foreign ids (PE11 + my cross-store case).
- **Copy**: draft, name +「（复制）」, fresh slug via freeSlug, fresh SKU codes via freeSKUCode, no
  images/keywords, `expected_version` guards the source (document.go:746).
- **Isolation**: every new route resolves scope from the merchant token; foreign id → 404 (edit,
  copy) or per-item `not_found` (bulk); list tallies are tenant/store-scoped
  (`TestProductEditorK3IsolationNewRoutes`).
- **Draft visibility**: list/detail by id+slug hidden (PE07); buyer cart refuses a draft product's
  SKU (storefront/cart.go:286 product status / :301 SKU status) — pinned in my isolation test.

## Findings

### P2-1 — DB CHECK 3VL gap: untracked SKU with NULL cap is storable, and begin_hold then treats it as unbounded
`migrations/0109_product_core.sql:22-24`. The CHECK is the §f literal
`(inventory_tracked AND max_per_order IS NULL) OR (NOT inventory_tracked AND max_per_order BETWEEN 1 AND 999)`;
SQL CHECK treats NULL as pass, so `inventory_tracked=false, max_per_order=NULL` is storable.
`begin_hold`'s guard `q.quantity > s.max_per_order` (0021:340) is NULL → not true → **no refusal**:
such a SKU is untracked AND unbounded. Today only reachable by out-of-band SQL (the document command
validates 1..999 and legacy CreateSKU defaults tracked), so it is a defense-in-depth hole, not an
API-reachable defect — and it is already documented in DEVIATIONS.md as the §f literal. Recommend the
integrator amend the CHECK to `… OR (NOT inventory_tracked AND max_per_order IS NOT NULL AND
max_per_order BETWEEN 1 AND 999)` (the `IS NOT NULL` makes the disjunct false, not NULL).
Failure scenario: any future writer (or a Go validation regression) stores untracked+NULL; a buyer
orders quantity 1 000 000 000; begin_hold places it with no stock and no cap.
Test: `TestProductEditorK3UntrackedNullCapGap` pins the current (gapped) behavior with a loud comment;
flip it to expect rejection when the CHECK is tightened.

### P2-2 — Axis change strands the keyword of an omitted SKU
`internal/catalog/document.go:216-223` (archive of unreferenced SKUs) never clears
`live.keyword_library`; `writeKeyword`'s taken-check (:611) has no SKU-status filter; and the edit
path (:440) refuses archived SKU ids, so the stranded keyword can never be released or reused through
the document command — the editor's axis-change flow hits `keyword_taken` with no in-surface recovery
if the new SKU wants the old keyword. Workarounds exist (include the old SKU explicitly with
`active:false` + empty keyword, or the live keyword-library API `SetLibraryKeyword` clear), so P2 not
P1. Failure scenario: merchant re-axes "Color"→"Size", the archived Red SKU keeps keyword `A1`, the
new SKU with `A1` makes the whole save roll back 409.

### P2-3 — Idempotency key is scoped to (tenant, store, operation), not actor
`internal/command/command.go:64-66` + `migrations/0002_catalog_inventory.sql:20` (PK without
principal_id). Two staff members of the same store sharing a key: same bytes → the second actor gets
the first actor's stored result (e.g. a product id they never saw created); different bytes → 409.
This is frozen 0002 infrastructure shared by every command, not introduced by this unit; recorded
because the unit brief asks for tenant/store/actor scope. If actor scoping is ever required it is a
contract amendment on command.Run, not a product-core change. Behavior pinned by
`TestProductEditorK3IdempotencyKeyScopeIsStoreNotActor`.

### P2-4 — `planLocked` tracked filter relies on RLS alone for tenant/store scoping
`internal/checkout/checkout.go:760`: `SELECT id … FROM catalog.skus WHERE id=ANY($1) AND
inventory_tracked` has no explicit tenant/store predicate; scoping comes from the
`checkout_runtime_read` RLS policy via GUCs. It fails closed (missing GUCs → zero rows → empty plan →
PT409 mismatch), and the pre-A6 balance reads work the same way, so this is consistent with the
codebase's model — noted because a one-line defense-in-depth predicate would make the invariant
independent of policy drift.

### P2-5 — Copy refuses long source names with a bare 422
`internal/catalog/document.go:765` appends 「（复制）」 (+4 chars) but `catalog.products.name` is
`CHECK (length(name) BETWEEN 1 AND 120)` (0002:28): a source name of 117–120 chars makes the copy
INSERT fail 23514 → `invalid_request` 422 with no hint. Suggest trimming to 116 or a coded refusal.
Not silent corruption; no data written.

### P2-6 — Copy carries no stock and no collection membership (consistent with ruling; noted)
`document.go:780-791` copies SKU rows with `inventory_tracked`/`max_per_order` but writes no opening
stock (a tracked copy SKU has no balance row — buyers get `insufficient stock` until the merchant
sets stock; the copy is a draft, so nothing buyer-visible breaks) and no `collection_products` rows.
The ruling lists only 文字/规格/SKU/价格 as copied, so this matches; flagged so product-ui does not
promise otherwise.

### Observation (not a finding) — SKU status is not re-validated at placement
`begin_hold` validates balances and the plan but never re-checks `catalog.skus.status`; a SKU
archived after the quote can still be ordered within the 60 s quote TTL at the quoted price. This is
pre-existing frozen checkout behavior (the quote is the freeze point), unchanged by this unit; the
bulk-unlist `live_window_open` guard covers the live-selling case. No action requested in this unit.

## Evidence

- Red→green mutation proofs and the full focused run: `output/product-core-tests/` (this worktree).
  M1 drop tracked-only filter (checkout.go:760) → 0P/3F, reverted, green (`mutation-M1-red.log`,
  `mutations-green.log`); M2 skip `max_per_order` guard (0021:340) → 0P/2F at the over-cap assertions,
  reverted, green (`mutation-M2-red.log`); M3 bypass expected_version (document.go:364,375) → both edits
  win, silent overwrite at version 3 → 0P/2F, reverted, green (`mutation-M3-red.log`). Product diff empty
  after reverts (`git diff --stat internal/ migrations/ contracts/` clean).
- Full focused suite: 216 pass / 3 fail, all 3 being P1-0's stale catalog-core gates
  (`final-focused.log`). Implementer's gates: PE01–PE11 in `tests/foundation/product_document_test.go`
  (re-run green here).
