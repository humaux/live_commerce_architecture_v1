# PARITY-DOMAINS-FIX1 Relay 3 Report

| Failing job / test | Class | What changed | File:line |
|---|---|---|---|
| 124559978416: TestBrowserStoreDesign (`domains-parity.spec.ts:155` active domain view visibility after reload) | (b) | Persisted active `domains` workspace state and command journal recovery across page reload in `sessionStorage` under `lc.design.tab.${store}` and `storefront-domains-cmd:${store}`. Restored `tab = "domains"` on component mount when present; cleared on tab exit to return cleanly to `'hub'`. | `apps/admin/components/Design.tsx:19,208-235` |
| receipts (matrix-receipts) | Downstream | Downstream failure caused by failed member `--browser-design` in bundle:03 | N/A (downstream consequence) |
| aggregate | Downstream | Downstream failure caused by bundle:03 | N/A (downstream consequence) |
| required | Downstream | Downstream failure caused by bundle:03/receipts/aggregate | N/A (downstream consequence) |

STATUS: DONE
