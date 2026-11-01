# GEMINI-REPORT — PARITY-SHIPCENTER-FIX3 (Relay 2)

Card: PARITY-SHIPCENTER-FIX3 (PR #353, branch `unit/parity-shipcenter`)
Failing run: `41191310176` (head `9c2be014a04c07a0d035261bdef5c811a792fd6d`)
Base: `origin/r3/integration` (`8d0211009658d450914a20b4ba3a9f5e6c437d6b`, PR #356 merged green on Gates `41190997835`)

| Failing job / test | Class | What you changed, file:line |
|---|---|---|
| `gate (bundle:07)` (`124509605616`) / `TestBrowserTaiwanCvs/WebKit` (`--browser-cvs`) | (c) Transient upstream WebKit suite glitch / out-of-card storefront suite (`tests/storefront/cvs-picker.mjs:387` timeout waiting for `getByTestId('cvs-candidate-024793')`). Verified green on base PR #356 Gates `41190997835` / job `124508478707`. | Merged latest `origin/r3/integration` (`8d0211009`, incorporating PR #356, #426, #427, #425). Resolved merge conflicts in `packages/i18n/src/index.ts:7-11` (preserving both `shippingCopy` and `stateRecoveryCopy, adminSignedStateCopy, buyerSignedStateCopy`), `tests/admin/orders-ui.spec.ts:15-18` (retaining both `registerShippingCenterTests` and `browserEvidenceDirectory`), and regenerated `output/ci-select-backend-browser/browser-ownership.json:1-56`. |
| `gate (bundle:06)` (`124481686118` in prior run `41181778625`) / `TestBrowserInboxUIRealChain` (`--browser-inbox`) | (c) Known base issue / upstream race in `tests/admin/inbox-browser-support.ts enterMessages` | Brought in PR #356 upstream fix (`a122b3cd6`) via integration merge; verified **PASS** on head `9c2be014` in run `41191310176` (`receipt-browser-inbox.json`: `status=PASS`, `test_count=89`, `spec_count=88`). |
| `receipts` / `required` | (c) Downstream aggregator failure | None; downstream of `gate (bundle:07)`. Documented in `output/parity-shipcenter/FIX3.md:24`. |

STATUS: DONE
