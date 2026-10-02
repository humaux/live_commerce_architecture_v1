# product-core — backend (DeepSeek V4-Pro)

Date: 2026-10-02. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/product-core`, branch `unit/product-core`, base `05653365`.
Scope: Go + SQL + contracts + PG/Go tests only; nothing under `apps/` was touched. All §f rulings implemented and gated.

## Per-item delivery

| §f item | Status | Where |
|---|---|---|
| A6 `inventory_tracked` + `max_per_order` | DONE | `migrations/0109_product_core.sql` (column + CHECK + image-cap widen + grants), `catalog.SKU`/`document.go` `normalizeStock`/`writeSKU`, checkout `planLocked` drops untracked, `post_river/0020_product_core_begin_hold.sql` (0..800 plan, tracked-only conservation, PT422 `max_per_order_exceeded`) |
| Document save (create/edit, one transaction) | DONE | `internal/catalog/document.go` `SaveProductDocument` — operations `product.save` / `product.save:<id>` via `internal/command.Run`; products+options+SKUs+stock+keyword+collections in one tx; conflict rolls back everything; axis change archives ordered SKUs |
| Images cap 12 | DONE | migration 0109 CHECK + `internal/catalog/images.go` (8→12); `images-client.ts` deferred to product-ui (Codex) |
| TWD whole-dollar | DONE | `catalog.checkWholeTWD`, refusal `amount_not_whole_twd` (422) |
| List additions | DONE | `internal/catalog/productlist.go` — `items[].keyword`, `items[].inventory_tracked`, `items[].updated_at`, `total`, `status_counts{draft,active,archived}` |
| Bulk status ≤100 | DONE | `BulkSetProductStatus` (op `catalog.product.bulk_status`), per-item result, `live_window_open` refusal per item |
| Copy | DONE | `CopyProduct` (op `catalog.product.copy`) — draft, name +「（复制）」, fresh slug/SKU codes, no images/keywords |
| R2 migration count | DONE | `r2_integration_upgrade_test.go` 39→41 |

## Gates (actual exit codes, worktree cwd)

| Command | Exit | Evidence |
|---|---:|---|
| `go build ./...` | 0 | (clean) |
| `go vet ./...` | 0 | (clean) |
| `go vet -tags browser ./...` | 0 | (clean) |
| `gofmt -l <changed .go>` | 0 | (empty) |
| `bash scripts/dev/check-gates.sh` | 0 | ok (55 modes) |
| `bash scripts/dev/depmap.sh --check` | 0 | up to date |
| `bash scripts/dev/check-pkgdocs.sh` | 0 | ok |

## Red → green

`bash scripts/dev/test-focused.sh 'TestProductEditor' ./tests/foundation` — 13 tests:
`TestProductEditorPE01SingleTransaction`, `PE02Idempotency`, `PE03KeywordAndSlugConflict`, `PE04TWDWholeDollar`,
`PE05UntrackedTrackedFlags`, `PE06ImagesCap`, `PE07DraftVisibility`, `PE08AxisChangeArchives`, `PE09StaleVersion`,
`PE10Isolation`, `PE11BulkStatus`, `TestProductEditorCopyCommand`, `TestProductEditorListSummaries`.
Final: **PASS=13 FAIL=0 SKIP=0 exit=0** — `output/product-core/green-PE01-PE11.log` (REAL_PG).

## Deviations / NOT_RUN / handoff

- Deviations (intentional, reviewed): `output/product-core/DEVIATIONS.md` — D1 (create is `POST /products/document`,
  not `POST /products`, which stays catalog-core's frozen quick-add), D2 (`SKUInput` omits the A6 flags; set only via
  `DocumentSKUInput.Stock`), plus the §f DB CHECK 3VL gap (untracked+NULL passes SQL; enforced in Go).
- K3 adversarial passes (PE02/PE05/PE09/PE10 concurrency) are owned by K3, not this unit — NOT_RUN here.
- `images-client.ts` (12-cap UI) deferred to product-ui (Codex).
- No apps/, no push, no merge, no deploy.
