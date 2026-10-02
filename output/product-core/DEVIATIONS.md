# product-core — DEVIATIONS from the frozen contract

Unit: product-core (backend, DeepSeek V4-Pro). Date: 2026-10-02. Base: `05653365`.
These are the places where the implementation deliberately differs from the literal
`contracts/catalog-inventory-v1.md` "Exact Go API" or from the integrator's routing
surface. Neither changes a §f ruling: every §f ruling (A6 flags, whole-dollar TWD, image
cap 12, one-transaction document save, bulk status ≤100 with per-item `live_window_open`,
copy, list additions) is implemented and gated. Each deviation is intentional, reviewed,
and recorded here so the integrator can accept or amend the contract.

## D1 — Document create is `POST /products/document`, not `POST /products`

`POST /v1/admin/stores/{store}/products` is the frozen catalog-core quick-add route
(`catalog.CreateProduct`, migrations/0002). The product-editor document command cannot
reuse that path without changing a frozen route, so create lives at
`POST /v1/admin/stores/{store}/products/document` and edit at
`PUT /v1/admin/stores/{store}/products/{product_id}/document`.

- Operation keys are unchanged and contract-shaped: create `product.save`, edit
  `product.save:<id>` (one `internal/command.Run` each).
- `internal/httpapi/handler.go` documents this and points at this file.
- No catalog-core route was changed; `POST /products` still does quick-add.

## D2 — `SKUInput` does not carry `InventoryTracked` / `MaxPerOrder`

The contract amendment's "Exact Go API" lists
`SKUInput{…; InventoryTracked bool; MaxPerOrder int64; …}`. Go `SKUInput`
(`internal/catalog/catalog.go`) does **not** have those two fields.

Why:

- The A6 flags are written exclusively through the document command's
  `DocumentSKUInput.Stock{Mode,MaxPerOrder}` (`internal/catalog/document.go`
  `normalizeStock`/`writeSKU`), which can express "tracked" vs "untracked" and the
  1..999 cap with full validation. §f does not require the flags on the legacy
  `CreateSKU`/`UpdateSKU` inputs.
- A plain `bool` cannot express "defaults true": its zero value (`false`) means
  *untracked*, so a legacy `CreateSKU` that simply zero-fills the struct would silently
  create untracked SKUs. `SKUInput` omits the fields, and `CreateSKU`/`UpdateSKU` rely on
  the migration's `inventory_tracked boolean NOT NULL DEFAULT true` (tracked, `max_per_order`
  NULL) — the correct pre-A6 behaviour preserved verbatim.

Related literal drift (also on the `SKU`/`SKUInput` rows of the Exact Go API): the contract
writes `MaxPerOrder int64`; Go uses `*int64` (`SKU.MaxPerOrder *int64`, JSON
`max_per_order,omitempty`) because the column is nullable (NULL for every tracked SKU).

## What is NOT a deviation

- The asymmetric DB CHECK `(inventory_tracked AND max_per_order IS NULL) OR (NOT
  inventory_tracked AND max_per_order BETWEEN 1 AND 999)` is the §f literal. Because an
  SQL CHECK treats NULL as "pass", the DB does not by itself reject "untracked with a NULL
  cap"; that case is enforced in Go (`validDocumentInput`). `migrations/0109_product_core.sql`
  documents this in its header, and `TestProductEditorPE05UntrackedTrackedFlags` pins the
  two DB-level rejections (tracked+cap, untracked cap 0/1000) plus the Go-level refusals.
- No change to the buyer storefront definers; no per-order cap for tracked SKUs; no TWD
  whole-dollar DB CHECK (Go-only, so legacy non-whole TWD rows do not block 0109) — all
  per the migration header's non-goals.
