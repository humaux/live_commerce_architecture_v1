# GEMINI-REPORT PARITY-CATALOG-PEOPLE-FIX1 relay 1 (PR #363 head 424e7ba5, run 41177298991)

Only failing job: gate (bundle:03) (124468182631) → member --browser-product-editor → TestBrowserCatalogCore/PE12-17-TWD. receipts/required fail only as dependents; aggregate/build/foundation and all other gates pass. Base Gates 41177512748 (71e403e8f) is green, so no class (c).
Merge: origin/r3/integration (71e403e8f) is already an ancestor of HEAD (merge e1d0df69e); ownership `--check` exit 0.
No new source change in this relay: the root cause was already fixed in unpushed local commits e547175ee + 47131fdb9 (ahead 4 of origin, not yet pushed); this report re-verified them against the failing log.

| failing job / test | class | change (already on HEAD) |
|---|---|---|
| bundle:03 / PE12-17-TWD, PARITY signed controls (`catalog-price-0` hidden inside per-variant `<details>`) | a | tests/admin/catalog-v2-simplified.acceptance.ts:237-239 real click on each `catalog-variant-N` summary before asserting visibility |
| same test, next assertions: signed stock frame is total-only (tracked/delta count 0) | b + a | apps/admin/components/CatalogSimplifiedEditor.tsx:122-125 tracking toggle only for untracked rows, delta field removed; tests/admin/product-editor.acceptance.ts:650-653 delta steps rewritten to the total field (same stage/reload/persist coverage) |
| receipts, required | dependent | none (fail only because bundle:03 failed) |

Risk: signed stock frame not re-read (no Figma access here). NOT_RUN: all tests (CI only). Evidence level E1 static + E3 original failure log; fix unverified until CI on the pushed head.
WAIT_RUN: integrator must push HEAD; no push made here.

STATUS: DONE
