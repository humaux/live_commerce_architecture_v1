# Unit catalog-core — catalog v2: draft status, slugs/SEO, option axes, compare-at, collections, stock hints, admin product editor

Role: commerce_worker. Base `r3/integration`. Worktree `.worktrees/catalog-core`, branch `unit/catalog-core`. Migration **0086**.
Contract: `contracts/storefront-v2.md` §A (FROZEN — implement exactly). No delegation, no new dependency, no lockfile change. Comment standard PROCESS.md §5 mandatory. Scope/tenant from server auth only (AGENTS.md). Three admin locales via existing copy pattern. Static set must exit 0: go build/vet, gofmt, check-pkgdocs, depmap --check, check-gates, admin + storefront typecheck, test-node.sh, check_packet.py. Go unit tests for pure logic + author PG smoke via `bash scripts/dev/test-focused.sh` (serialized; Docker 1.9 GB). Evidence → /Volumes/data/live_commerce_architecture_v1/output/<unit>/. Commit on branch, do not merge.

## Scope
1. 0086 + Go (`internal/catalog`, buyer projection in `internal/storefront` / `internal/buyerhttp`, admin routes in `internal/httpapi`) for every §A field, admin route and buyer read; collection media `/media/c/...` reusing catalog-media's sniff/limits code (no copy).
2. Admin UI: a real product list page `apps/admin/app/[locale]/products/page.tsx` (search, status filter, cover thumbnail, price range, stock) and editor page `products/[product]/page.tsx` (title, description, status, slug, SEO, photos — reuse the catalog-media photo manager component, option axes editor that generates the SKU matrix, per-variant price/compare-at/SKU code/stock adjust), plus a collections page `collections/page.tsx` (create/edit, image, sort mode, add/remove/reorder products). Ledger stays as the inventory view; add nav entries in WorkspaceFrame (products, collections) — keep the ledger reachable.
3. Contract §A acceptance list appended to the contract (what the tester must prove).
## Write paths
migrations/0086_*.sql, internal/catalog/**, internal/storefront/**, internal/buyerhttp/**, internal/httpapi/** (catalog/collections routes only), apps/admin/app/[locale]/{products,collections}/**, apps/admin/components/{Product*,Collection*}.tsx (new), apps/admin/components/WorkspaceFrame.tsx (nav only), apps/admin/lib/**, BFF allowlist, contracts/catalog-inventory-openapi.json, contracts/storefront-v2.md (acceptance append only).
## Non-goals
Storefront pages (unit storefront-shell), bulk CSV (later), multi-language content.
