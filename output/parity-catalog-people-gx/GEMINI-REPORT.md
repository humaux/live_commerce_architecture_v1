# PARITY-CATALOG-PEOPLE-FIX2 — GEMINI REPORT

## Directed Fix Summary

This directed fix resolves the CI failures on PR #363 (run 41054099331) following the Integrator ruling of 2026-10-31 regarding the approved native Copy entry and confirmation/success states from signed K2 delivery (`1469:141623` / `1469:141703` and `1469:141754` / `1469:141832`). `origin/r3/integration` at `acf2ebd2c6a19dd731aa98acce509b45629b1f77` was merged first.

## Failure Classification and Changes

| Failing Job / Test | Class | What Changed (File:Line) |
|---|---|---|
| `124070684239` / `TestBrowserCustomersBilling` (`--browser-customers-billing` bundle member) | (b) Root cause product parity fix / (a) Signed frame migration | `apps/admin/components/Customers.tsx:130` (replace unsigned tools summary with signed `p.records` `標籤與紀錄`), `apps/admin/components/CustomerDetail.tsx:245,280-350` (replace summary with signed `p.records`, dedicated privacy outcome states, read existing request without repeating POST, permission enforcement), `packages/i18n/src/people.ts:35,63,87` (add `records` translation across all 3 locales) |
| `124070684239` / `tests/admin/people-v2.spec.ts` (unsigned list disclosure removed, privacy outcome/lookup assertions) | (a) Older test asserts UI signed Figma frame intentionally replaced | `tests/admin/people-v2.spec.ts:275,281` (update test locator to assert removal of unsigned `people-advanced` and presence of signed `people-customer-tools` / `people-list-tools` with `標籤與紀錄`) |
| `124070683720` / `TestCustomersBillingCB03List/PARITY_consent_state_distinguishes_never_withdrawn_and_regranted` | (b) Root cause product read projection | `internal/customers/read.go:65-80` (compute `consent_state` distinguishing never, withdrawn, granted, regranted from events), `internal/customers/types.go:95,138` (expose `ConsentState` in List/Detail DTOs) |
| `124070683720` / `TestCustomersBillingCB09HTTP/GET_customers` | (b) Root cause product read projection | `internal/customers/types.go:95,160-185` (serialize `consent_state` map in JSON DTO for GET `/customers`), `internal/customers/read.go:65-80` |
| `124070683720` / `TestCustomersBillingCB09HTTP/GET_customers/{id}` | (b) Root cause product read projection | `internal/customers/types.go:138,210-230` (serialize `consent_state` map in JSON DTO for GET `/customers/{id}`), `internal/customers/read.go:65-80` |
| `124070684122` / `TestParityConsentStateProjection` (`never`, `withdrawn`, `granted`, `regranted`) | (b) Missing DTO state projection | `internal/customers/types.go:95,138` (serialize non-nil `ConsentState` map for all consent states) |
| `124070683762` / `TestBrowserCatalogCore/PE12-17-TWD` (unsigned search removed, Advanced menu absent) | (a) Older test asserts UI signed Figma frame intentionally replaced / (b) Product parity | `apps/admin/components/ProductList.tsx:100-111,128-148` (remove unsigned search form and `products-advanced`; add signed K2 Product tasks `products-operations` with `products-copy-entry`, `product-copy-review`, and `product-batch-results`), `packages/i18n/src/catalog.ts:6,27,31` (add signed K2 copy strings), `tests/admin/catalog-core.spec.ts:132,247,274` (click inventory via `products-operations`; prepare test query via URL `?q=...`), `tests/admin/catalog-document-driver.ts:195-200` (click through `products-operations` -> `products-copy-entry` -> `product-copy-review`), `tests/admin/settings-v2.spec.ts:209,264` (use shared `selectProductForCopy`), `tests/admin/product-editor.acceptance.ts:455,2001,2020` (use `selectProductForCopy`, assert `products-copy-entry` disabled state), `tests/admin/catalog-v2-simplified.acceptance.ts:154-211` (assert signed Copy entry, confirmation, and result) |
| `124100320890` / `receipts` (unit, g09, customers-billing, product-editor) | Dependent aggregate job | Fails strictly due to the above 4 failing test jobs |
| `124103568945` / `required` (gate, receipts) | Dependent aggregate job | Fails strictly due to the above failing jobs |

STATUS: DONE
