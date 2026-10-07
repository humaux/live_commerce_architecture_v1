# fix-merchant-buyer-pmv2 delivery
- Branch/commit: unit/fix-merchant-buyer-pmv2 (SHA = the commit containing this file; see `git log -1`)   Base: 0d6b4b5f (origin/r3/integration)   Model: Claude Sonnet 5.5
- Evidence class: MODEL_ONLY (Node source/guard test) + static (vet/tsc). Browser/PG run NOT_RUN locally (RAM rule); CI gates below are the acceptance.

## Symptom
Trunk CI run 37596925787 (trunk 49859ae2, after PM-U) fails `TestBrowserMerchantBuyerRealChain` in `--browser-merchant-buyer` and `--browser-webkit`:
`tests/foundation/browser_merchant_buyer_chain_test.go:208 expected exactly four merchant command receipts (...)`. The Playwright gate itself passed all 8 cases
(`result.json` cases=8; product created, activated, buyer URLs served), so the product flow works; only the receipt count differs. Evidence: artifact
`gate-browser-merchant-buyer` (`ci-gates/--browser-merchant-buyer.log:181`).

## Root cause (product regression, not an intended contract change)
`apps/admin/lib/use-product-document.ts:142` (as of 0d6b4b5f) guarded the create workflow's image-axis stage with `op.imageAxis !== undefined`, but the only caller,
`apps/admin/components/ProductDocumentForm.tsx:273`, always passes `... ? imageAxis : null` in create mode, i.e. `null`, never `undefined`. So every new product, even
one without a chosen image axis, issued `POST products/{id}/image-axis {axis:null}` -> Go `SetImageAxis` (`internal/catalog/image_roles.go:209`, operation
`catalog.image.axis`) -> one extra `ops.command_results` row + one `catalog.product.image_axis_set` audit event. `null` means "the first axis" (contracts/catalog-inventory-v1.md
"Image axis": `NULL` = the first axis; `POST image-axis` null = "back to the first axis"), which is already a new product's stored value, so the write is a pure no-op.
Introduced by 4fea09c6 (PM-U role editor). The create sequence document (`product.save`) -> upload (`catalog.image.upload`) -> order (`catalog.image.reorder`, one per role that
has images) -> publication (`catalog.product.bulk_status`) is otherwise unchanged (`use-product-document.ts:156-229`). Delta was therefore 5 instead of 4 (the four stages
are mandatory for a published product with a photo, and the image-axis stage is the only other write in the flow; confirmed by reading every `send`/upload call of the hook).

Not intended: neither docs/delivery/units/product-media-v2.md nor contracts/catalog-inventory-v1.md (0149 amendment) say the editor must write the axis for a product that did not
choose one (brief: "default = first option axis"); `tests/foundation/browser_product_media_v2_test.go:497` asserts `image_axis` stays NULL when "the merchant never changed it".
The test's c6/A sequence (document, image, order, publish) was written before PM-v2 and the contract does not change it, so the test is kept exact.

## Change
- `apps/admin/lib/use-product-document.ts:142-144`: guard is now `typeof op.imageAxis === "string"`: the image-axis stage runs only for a chosen axis (unchanged behaviour then);
  null/undefined skips it. No contract, API, SQL or Go change.
- `tests/admin/product-media-ui-model.test.ts` (new test at the end): extracts the hook's actual guard from source and evaluates it: runs for a chosen axis, never for null/undefined/already-set/edit.
- `tests/foundation/browser_merchant_buyer_chain_test.go`: `expectedOperations` and the per-operation exactly-once check are UNCHANGED (not loosened); only the failure message now prints the
  actual delta (`got %d`) and a comment cites the contract rule. No other harness/spec asserts these receipts or the axis command (grep `catalog.image.`, `image order`, `image-axis`,
  `ops.command_results` over tests/ and scripts/: only this test; `browser_product_media_v2_test.go` / `product_media_v2_test.go` call image-axis directly against Go, unaffected).

## Commands
| Command | Exit | Evidence |
|---|---:|---|
| `node --test --experimental-strip-types tests/admin/product-media-ui-model.test.ts` before fix | 1 | `output/fix-merchant-buyer-pmv2/red.log` (15 pass / 1 fail: `{"edit":false,"imageAxis":null}` true !== false) |
| same after fix | 0 | `green.log` (16 pass / 0 fail) |
| `node --test --experimental-strip-types tests/admin/product-media-model.test.ts tests/admin/product-media-ui-model.test.ts` | 0 | `node-media.log` (21 pass / 0 fail) |
| `go vet ./...` | 0 | `go-vet.log` |
| `go vet -tags browser ./tests/foundation` | 0 | `go-vet-browser.log` |
| `(cd apps/admin && npx tsc --noEmit -p .)` | 0 | `tsc-admin.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates.log` (77 modes, headers OK) |

## CI gates (integrator pushes this branch, runs `.github/workflows/gates.yml`)
1. `--browser-merchant-buyer` (the red gate; must now see exactly document/upload/reorder/bulk_status once each)
2. `--browser-webkit` (runs the same chain on WebKit)
3. `--browser-product-editor` (editor create/edit paths; create no longer posts image-axis for a product without a chosen axis)
4. `--browser-product-media-v2` (chosen-axis path and option-image flow must still pass)
5. `--browser-catalog-media` (shares `createProductInEditor`)
Optional, same shared helper: `--browser-storefront-publish`, `--browser-design`.

## Risks
- A merchant who picks an image axis in the create form still gets the image-axis command (unchanged); only the null default is skipped. If the server ever started to default `image_axis` to non-NULL, this guard would need revisiting (contract currently says NULL = first axis).
- The new Node test reads the guard from source with a regex, in the style of the neighbouring tests; a reformat of that `if` line must keep `op.axisCommand` on the next line.

## NOT_RUN / BLOCKED
Any `--browser-*` mode, full foundation suite, PostgreSQL, WebKit (RAM rule; CI gates above). `scripts/dev/test-node.sh` full suite not run (only the media model tests listed). No memory store (instructed).
Local node_modules were temporary symlinks to the r3-integration worktree for the Node/tsc checks and were removed again.
