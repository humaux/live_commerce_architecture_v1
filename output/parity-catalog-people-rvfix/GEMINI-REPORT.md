# GEMINI-REPORT PARITY-CATALOG-PEOPLE RVFIX (PR #363 head 82cbe05a6)

Independent review (Claude Opus, `output/issue-107/review-363-82cbe05a6.md`) returned REQUEST_CHANGES on head `82cbe05a6`.
Per integrator ruling (2026-10-31), addressed the two P1 review findings:
1. P1-1 (`internal/customers/read.go:88`): compute `consent_state` in SQL via `migrations/0215_customer_consent_state.sql`, removing N+1 detail reads in `List`, decoding strictly from row in `decodeCustomer`, cross-checking against `consentState(history)` in `decodeDetail`, and bumping upgrade pin in `tests/foundation/r2_integration_upgrade_test.go:134`.
2. P1-2 (`apps/admin/components/ProductList.tsx:100`): add `aria-label={c.operations}` to `<summary data-testid="products-operations">` while keeping decorative text span `aria-hidden="true"`, and add accessible name assertions next to clicks on `products-operations` in `tests/admin/catalog-core.spec.ts:132` and `tests/admin/catalog-v2-simplified.acceptance.ts:176`.

| failing job/test | class | what you changed, file:line |
|---|---|---|
| Review finding P1-1: `internal/customers/read.go:88` (N+1 detail reads in List) / `CB03`/`CB09` list projection | b | `migrations/0215_customer_consent_state.sql:1-177` (additive migration computing `consent_state` in `identity.read_merchant_customers`); `internal/customers/read.go:80-86` (remove per-row `loadDetail` in `List`); `internal/customers/types.go:340,353-377,480` (add `consent_state` to `customerKeys`, validate in `validCustomer`, cross-check against `consentState(history)` in `decodeDetail`); `internal/customers/types_test.go:136,181-187,208-217,231-233` (align test fixtures and add rejection test cases); `tests/foundation/r2_integration_upgrade_test.go:134-136` (bump R2 migration pin from 141 to 142) |
| Review finding P1-2: `apps/admin/components/ProductList.tsx:100` (unnamed disclosure on `products-operations`) | b | `apps/admin/components/ProductList.tsx:100` (add `aria-label={c.operations}` to `<summary data-testid="products-operations">`); `tests/admin/catalog-core.spec.ts:132` (assert `toHaveAccessibleName(catalogTaskCopy[L].operations)`); `tests/admin/catalog-v2-simplified.acceptance.ts:176` (assert `toHaveAccessibleName(catalogTaskCopy[locale].operations)`); `output/ci-select-backend-browser/browser-ownership.json` (regenerated ownership map) |

STATUS: DONE
