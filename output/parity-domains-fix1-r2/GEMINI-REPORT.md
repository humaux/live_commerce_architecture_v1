# PARITY-DOMAINS-FIX1 Relay 2 Report

| Failing job / test | Class | What changed | File:line |
|---|---|---|---|
| 124554003084: TestBrowserStoreDesign (`domains-parity.spec.ts:132` request receipt visibility) | (b) | Corrected synthetic TXT proof verification token fixture length from 44 to 43 characters (`"domain-fixture-" + "a".repeat(28)`). Strict parser in `storefront-model.ts` requires `^[A-Za-z0-9_-]{43}$` per migration 0106 check constraint; 44 chars triggered uncertain write recovery view (`pending`) instead of `receipt`. | `tests/admin/domains-parity.spec.ts:20` |
| 124554003084: TestBrowserStoreDesign (browser ownership manifest) | (b) | Regenerated `output/ci-select-backend-browser/browser-ownership.json` for updated `domains-parity.spec.ts` SHA | `output/ci-select-backend-browser/browser-ownership.json:57` |
| receipts (matrix-receipts) | Downstream | Downstream failure caused by failed member `--browser-design` in bundle:04 | N/A (downstream consequence) |
| aggregate | Downstream | Downstream failure caused by bundle:04 | N/A (downstream consequence) |
| required | Downstream | Downstream failure caused by bundle:04/receipts/aggregate | N/A (downstream consequence) |

STATUS: DONE
