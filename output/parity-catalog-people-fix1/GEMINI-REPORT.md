# GEMINI-REPORT PARITY-CATALOG-PEOPLE-FIX1

| failing job/test | class | what changed |
|---|---|---|
| gate (bundle:03) / TestBrowserCatalogCore/PE12-17-TWD (member --browser-product-editor) | a | tests/admin/catalog-v2-simplified.acceptance.ts:239-240 real clicks on each variant summary before asserting price inputs (e547175ee) |
| same test, following assertions (tracked/delta count 0 on tracked row) | b + a | apps/admin/components/CatalogSimplifiedEditor.tsx:122-126 tracking toggle only for untracked rows, delta field removed; tests/admin/product-editor.acceptance.ts:650-653 delta steps rewritten to the total field, same stage/reload/persist coverage (47131fdb9) |
| TestBrowserCatalogCore, bundle member failure | derived | follow the PE12 failure |

- Merge origin/r3/integration: already up to date. Ownership: `derive-browser-ownership.mjs --check` exit 0.
- Risk: the signed stock frame was not re-read (no Figma access). The product change follows the new PARITY test assertions at catalog-v2-simplified.acceptance.ts:243-245. Nothing was run locally (CI only). Fixes are unpushed; the integrator pushes.
- WAIT_RUN: integrator push, then new Gates run on the PR head.

STATUS: DONE
