# CATALOG-V2-IMPL · exact write plan

Role: integrator-dispatched UI worker; no recursive delegation, no separate claim.
Branch: `unit/catalog-v2-ui`; worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/catalog-v2-ui`.
Base: `aa4c5eb65724fe14afa7e10a9aaa9cf140a1517f` (origin/r3/integration fetched and merge reported already up to date).
Actual model/effort: runtime does not expose exact identifier/effort; UNKNOWN.

## Authority and scope

Owner card CATALOG-V2-IMPL, owner sign-off 2026-10-28. UI authority ONLY `.worktrees/ux-catalog/output/ux-catalog/DELIVERY.md` latest simplify section, 15 paired frames 1429:62828–64561, current 89-string table. Do not promote historical 140-string table or output snapshots to current implementation status.
Read: AGENTS.md, 架构.md §§0/1/2/26/27; contracts/tasks.json DAG T04; catalog-inventory-v1 §g/Boundary/API/product-media-v2. PostgreSQL/API own money, stock, versions, archive refusal and atomicity. No API/schema/migration/dependency/settings changes.

## Exact implementation write paths

Identified from signed states and `rg -l 'Catalog|catalog|商品' apps/admin`, routes and direct imports:

- `apps/admin/components/ProductList.tsx` — list/filter/select/bulk confirmation/result.
- `apps/admin/components/ProductEditor.tsx` — existing product editor host.
- `apps/admin/components/ProductDocumentForm.tsx` — existing edit flow composition; preserve creation and frozen gates.
- `apps/admin/components/ProductDocumentVariants.tsx` — variants and stock presentation if required.
- `apps/admin/components/ProductDocumentMedia.tsx` — staged image composition if required.
- `apps/admin/components/ProductMediaManager.tsx` — existing-product image roles if required.
- `apps/admin/components/ProductDocument.css` — scoped signed layout using incumbent tokens.
- `apps/admin/components/ProductAdmin.css` — scoped list layout if required.
- `apps/admin/components/CatalogSimplifiedEditor.tsx` — signed edit/stock/media/review/result composition if existing form cannot express task screens.
- `apps/admin/components/CatalogSimplifiedList.tsx` — signed list flow only if decomposition is necessary.
- `apps/admin/lib/catalog-simplified-copy.ts` — typed facade only if needed; seller copy lives in i18n.
- `packages/i18n/src/catalog.ts` — typed zh-TW/zh-CN/en entries from current signed copy table.
- `packages/i18n/src/index.ts` — additive catalog copy export.
- `apps/admin/app/[locale]/products/page.tsx`, `apps/admin/app/[locale]/products/[product]/page.tsx` — existing routes only, if composition requires; no new route.
- `tests/admin/catalog-v2-simplified.acceptance.ts` — ONE additive browser acceptance registration; synthetic MOCK response state survives page refresh.
- `tests/admin/catalog-core.spec.ts` — additive registration/import; original assertions stay intact.
- `tests/admin/product-media-ui-model.test.ts`, `tests/admin/product-document.test.ts`, `tests/admin/shell-registry.test.ts` — update import harnesses only when changed imports require it; assertions never weakened.
- `output/catalog-v2-impl/**` — plan, delivery, read-only design receipts, RED evidence, static receipts.

Generated paths: `output/ci-select-backend-browser/browser-ownership.json` via `node scripts/dev/derive-browser-ownership.mjs` (stage new tracked specs first); `docs/engineering/dependency-map.md` via `scripts/dev/depmap.sh` only if CI requires.
Anything else goes in PR body for integrator. No backend/test-foundation changes, contract/OpenAPI, go.mod or lockfiles.

## RED → GREEN gate and handoff

One batch tests-only commit covering three locales × desktop/mobile main page; all four filters; selection, confirm/cancel archive, per-item partial result and refresh; edit, both prices, variant identity and stock; invalid stock retains input and submits nothing; image roles and picker; review/cancel/save/receipt and refresh. Browser operations click/fill/select/keyboard; evaluation reads geometry/text only. Fixture responses are MOCK and assert command shapes, never prove PG/publish/LIVE.

Preserve all previous assertions and suite order. Local verification only permitted static checks. CI selects existing catalog-core/product-editor owners through additive import; the new batch registers only for PRODUCT_EDITOR_ACCEPTANCE=1, whose existing fixture is TWD. Frozen CC12 USD assertions remain unchanged. Before adding spec: fetched/merged origin/r3/integration (no changes). Generate browser ownership in same RED push. No implementation before causal RED assertion log. No evidence-only push after GREEN. Poll at most ~10 minutes; register Foundation/Gates RUN IDs, then integrator relays completion in a fresh session. Author assessment E0; CI acceptance PENDING.
