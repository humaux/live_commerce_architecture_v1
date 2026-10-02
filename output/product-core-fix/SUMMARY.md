# product-core-fix — K3 review targeted backend fix round (DeepSeek V4-Pro)

Date: 2026-10-03. Worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/product-core-tests`,
branch `unit/product-core-tests` (implementer 39bb8ec + K3 992c900). Backend only; `apps/` untouched.

Source of truth: `output/ext-agents/REVIEW-product-core.md`; integrator rulings below executed exactly,
red-first where the red was observable.

## Rulings executed

### P1-0 — catalog-core gates re-baselined to the new contract (assert, do not delete)
- `tests/foundation/catalog_media_test.go`: image cap is now 12 photos; positions 0..11 accepted, a 13th
  photo refused 409. The lifecycle cap block and the `"position 12"` lifecycle fixture were re-baselined.
- `tests/foundation/catalog_core_upgrade_test.go`: the CC01 digest allow-list (line ~191) now includes
  `inventory_tracked` and `max_per_order` so the release-head upgrade digest stays green.

### P2-1 — DB CHECK made 3VL-safe + begin_hold defence-in-depth (0109 edited in place)
- `migrations/0109_product_core.sql`: the SKU CHECK is now
  `(inventory_tracked AND max_per_order IS NULL) OR (NOT inventory_tracked AND max_per_order IS NOT NULL AND max_per_order BETWEEN 1 AND 999)`.
  The `IS NOT NULL` on the untracked disjunct is what makes it 3VL-safe — without it,
  `inventory_tracked=false AND max_per_order=NULL` evaluates to NULL (pass), so an untracked SKU could be
  stored unbounded. Header comment documents the asymmetry.
- `migrations/post_river/0021_product_core_begin_hold.sql`: the `PT422 max_per_order_exceeded` guard now
  also refuses an untracked line whose cap is NULL (`WHERE NOT s.inventory_tracked AND (s.max_per_order IS NULL OR q.quantity>s.max_per_order)`),
  so even a row that somehow bypasses the CHECK is still bounded at Begin.

### P2-2 — axis change releases an omitted SKU's keyword in the same transaction
- `internal/catalog/document.go` `SaveProductDocument`: the archive pass was moved to a pre-pass (before the
  main SKU loop). For each existing active SKU not referenced by the document it archives the row **and**
  clears its keyword via `writeKeyword(..., "")` **before** new SKUs are written — so a fresh SKU can reuse an
  omitted SKU's keyword instead of rolling back `keyword_taken`.
- Test `TestProductEditorK3AxisChangeReleasesKeyword` uses a unique `pdKeyword()` (not a literal) to avoid the
  `live.keyword_library` `UNIQUE(tenant,store,keyword)` collisions other tests could create in `storeA1`.

### P2-4 — explicit tenant/store predicates in the hold path (not RLS alone)
- `internal/checkout/checkout.go` `planLocked`: signature now takes `scope`, and the tracked-SKU filter is
  `SELECT id::text FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND id=ANY($3::uuid[]) AND inventory_tracked`
  — explicit `tenant_id`/`store_id` predicates instead of relying on RLS.

### P2-5 — copy clamps the source name + one extra root-cause fix
- `internal/catalog/document.go` `CopyProduct`: `name` is clamped to `120 - runes("（复制）")` runes before the
  suffix is appended, so name +「（复制）」 fits the 120-rune `catalog.products.name` CHECK (no bare 422).
- **Extra fix the 120-rune test forced out:** `freeSKUCode` did not clamp its first candidate, and the SKU
  `code` CHECK is `^[A-Za-z0-9_.-]{1,64}$` while the slug allows 80. A long product name → 80-char slug →
  80-char `code` → 23514 → bare 422 on **both create and copy** (not just copy). Added a 64-char clamp before
  the loop, mirroring the `n>1` suffix logic. This is a genuine latent bug, not scope creep: P2-5's "copy a
  120-char name should succeed" cannot pass without it.
- Test `TestProductEditorK3CopyClampsLongName`: create a 120-rune product, copy it, assert the copy name is
  exactly `name[:116] + "（复制）"` = 120 runes.

### P2-3, P2-6 — no change (per ruling).

## Gates (actual exit codes, worktree cwd)

| Command | Exit | Evidence |
|---|---:|---|
| `go build ./...` | 0 | clean |
| `go vet ./...` | 0 | clean |
| `gofmt -l .` | 0 | empty |
| `bash scripts/dev/check-gates.sh` | 0 | ok (57 modes) |
| `bash scripts/dev/depmap.sh --check` | 0 | up to date |
| `bash scripts/dev/test-focused.sh '^(TestProductEditor\|TestProductDocument\|TestCatalog\|TestCheckout\|TestHomeCod\|TestLive\|TestR2IntegrationUpgradeFromReleaseHead\|TestT06WorkerAuthorityAndFunctionACL)'` | 0 | PASS=221 FAIL=0 SKIP=0 (`final-focused.log`, REAL_PG, 822s) |

## Red-first evidence

- `TestProductEditorK3CopyClampsLongName` first run was RED: `POST /products/document` → 422 `invalid_request`
  (the freeSKUCode 64-char gap; a 119-rune name also failed, proving it was the code CHECK, not the name
  CHECK). After the `freeSKUCode` clamp it is GREEN.
- The P2-1/P2-2/P2-4 tests assert the new contract values directly (the prior gates already pinned the old
  values, so flipping the assertions to the new contract is the red→green move).
