# product-core-rebase — P0 migration collision fix (DeepSeek V4-Pro)

Date: 2026-10-02. Worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/product-core`, branch `unit/product-core`.

## The bug

`post_river/0020_home_cod_begin_hold.sql` (home-cod cash-on-delivery) and the product-core
`0021_product_core_begin_hold.sql` both `CREATE OR REPLACE` the same `checkout.begin_hold`
function(s). The product-core 0021 body had been written against the pre-home-cod `0018` body, so
applying 0021 after 0020 silently deleted cash-on-delivery (`cash_on_delivery` vanished from the
payment-mode guard, and every home-cod rule — COD settings, surcharge/carrier snapshots,
AWAITING_COLLECTION, collection PENDING, the shared open cap, `cash_on_delivery_placed` event —
was lost).

## The fix

`migrations/post_river/0021_product_core_begin_hold.sql` was rebuilt by starting from the 0020
home-cod body **verbatim** (cash-on-delivery branch and every 0107 rule preserved) and adding only
the three A6 deltas:

1. plan count bound `0..800` (was `1..800`) — an all-untracked order sends an empty plan;
2. plan-to-quote conservation demand JOINs `catalog.skus` on `inventory_tracked` (tracked SKUs
   only), so an untracked SKU charged in the quote but absent from the plan cannot trip the
   FULL JOIN mismatch;
3. a new `PT422 max_per_order_exceeded` guard for untracked lines whose quantity exceeds their
   `max_per_order` (no balance row means the stock lock cannot bound them).

Backup of the fixed body: `0021_fixed.sql`.

## Red → green (would-have-caught test)

`tests/foundation/home_cod_test.go` gained `TestHomeCodAndUntrackedCoexist`, which in one test DB
places a home-cod order (asserts `AWAITING_COLLECTION` / `cash_on_delivery` / collection `PENDING` /
total 2500 / surcharge 5000) **and** an untracked-SKU card order (asserts no balance row, no
reservation lines, no ledger rows).

- RED (`red-TestHomeCodAndUntrackedCoexist.log`): with the pre-rebase 0021 the COD `Begin` fails
  `invalid request` — `cash_on_delivery` is no longer an accepted payment mode.
- GREEN (`green-TestHomeCodAndUntrackedCoexist.log`): with the rebuilt 0021 both orders succeed.

## Second fix surfaced by the red run: whole-TWD scope

`checkWholeTWD` had been added to the frozen catalog-core `catalog.CreateSKU` / `SetSKUPrice`
quick-add paths, which broke every legacy TWD fixture that uses a non-whole price
(`t04CreateStock` creates `PriceMinor: 1250`). The §f whole-dollar ruling is a product-editor
ruling, and D2 pins the legacy `CreateSKU`/`UpdateSKU` paths to "pre-A6 behaviour preserved
verbatim", so the check now lives only at the document boundary (`document.go` `SaveProductDocument`).
PE04 still passes (`TestProductEditorPE04TWDWholeDollar`).

## Gates (actual exit codes, worktree cwd)

| Command | Exit | Evidence |
|---|---:|---|
| `go build ./...` | 0 | clean |
| `go vet ./...` | 0 | clean |
| `gofmt -l internal/catalog/catalog.go tests/foundation/home_cod_test.go` | 0 | empty |
| `bash scripts/dev/check-gates.sh` | 0 | ok (57 modes) |
| `bash scripts/dev/depmap.sh --check` | 0 | up to date |
| `bash scripts/dev/test-focused.sh '^(TestProductEditor\|TestHomeCod\|TestCvs\|TestCheckoutOffline\|TestCheckout\|TestR2IntegrationUpgradeFromReleaseHead\|TestT06WorkerAuthorityAndFunctionACL)'` | 0 | PASS=63 FAIL=0 SKIP=0 (`focused.log`, REAL_PG) |

The coexistence test is covered by `TestHomeCod` in that regex (`TestHomeCodAndUntrackedCoexist` PASS).
