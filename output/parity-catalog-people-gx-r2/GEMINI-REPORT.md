# PARITY-CATALOG-PEOPLE-FIX2 — GEMINI REPORT (RELAY 2)

## Directed Fix Summary

This directed fix resolves the CI failures on head `71658dab` of PR #363 (Foundation run `41171222063`, Gates run `41171222055`). Root causes across foundation hex ratchets, editor stock testids, customer privacy table duplicate elements, and visual lint scroll capture bounds have been resolved at the product code level without loosening any thresholds or deleting tests.

## Failure Classification and Changes

| Failing Job / Test | Class | What Changed (File:Line) |
|---|---|---|
| `124449390363` (foundation) / `tests/ui/ds-hex-ratchet.test.mjs:87:1` | (b) Root cause product fix | `apps/admin/components/catalog/parity.css:7` (replaced hex color `#eaf6f4` with design token `var(--ui-primary-soft)`) |
| `124449410189` (bundle:03 - `gate-browser-product-editor`) / `TestBrowserCatalogCore/PE12-17-TWD` | (b) Root cause product fix | `apps/admin/components/CatalogSimplifiedEditor.tsx:122-127` (restored `data-testid="catalog-stock-tracked"`, `data-testid="catalog-stock-delta"`, and reason text in stock edit mode) |
| `124449411993` (bundle:07 - `gate-browser-customers-billing`) / `TestBrowserCustomersBilling` | (b) Root cause product fix | `apps/admin/components/CustomerDetail.tsx:404-412` (removed duplicate `privacy-actions` table that caused Playwright strict mode violation on `getByTestId('privacy-actions')`) |
| `124449412586` & `124449413201` (`visual-lint` shards 1/4, 4/4) / `admin|products-product` inner-scroll `end-not-visible` | (b) Root cause product fix | `apps/admin/components/catalog/parity.css:22-23,39` (removed `min-height: calc(100dvh - 160px)` and mobile `min-height: calc(100dvh - 120px)` on `.cat-task-editor > form`; added `max-height: calc(100dvh - 180px); overflow-y: auto; overscroll-behavior: contain;` on `.cat-task-editor` so container fits within viewport or scrolls smoothly via native wheel) |
| `124449413800` (receipts) & `124449414200` (required) | Dependent aggregate jobs | Resolved transitively by fixing the underlying foundation and browser gate jobs above |

STATUS: DONE
