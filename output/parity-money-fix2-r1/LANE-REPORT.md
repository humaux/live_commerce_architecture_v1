# PARITY-MONEY-FIX2 lane report (CI run 41238779136 on d53b095c9; base 41238442379 green)

Merge of origin/r3/integration: already up to date.
Evidence (not committed, ADR-20261031): run 41238779136, artifacts gate-browser-home-cod / gate-browser-customers-billing / gate-browser-product-editor.

| Failing job / test | Evidence | Class | Change |
|---|---|---|---|
| bundle:06 `TestBrowserHomeCod` → "COD settings and save-review frames (zh-TW, 1440)" 240 s timeout | page snapshot shows signed settings overview after `page.reload()`; test then clicked the step-2 button that only exists inside the wizard | a (test follows the signed Edit-settings entry, same coverage, real clicks) | `tests/admin/home-cod.spec.ts:140-143` re-enters via "Edit settings" before step 2 |
| bundle:07 `TestBrowserCustomersBilling` → "PARITY-MONEY finance signed states (zh-TW, 1440)" | `response.json: No resource with given identifier` (body evicted by navigation before read) | b (my new test, race) | `tests/admin/customers-billing.spec.ts:99-102` reads the body in the `waitForResponse` promise |
| bundle:03 `TestBrowserCatalogCore/PE12-17-TWD` → "CATV2 concurrent committed-stock refusal…" | strict-mode violation: `getByRole('alert')` matched editor alert + `__next-route-announcer__`; file untouched by this PR, base green | c/flake (no product code of this PR involved); locator scoped, not loosened | `tests/admin/catalog-v2-simplified.acceptance.ts:470` scoped to `catalog-task-editor` |
| receipts / required | fail only as consequence of the three jobs above | — | none |

Browser ownership regenerated (`derive-browser-ownership.mjs`, `--check` clean).

STATUS: DONE
