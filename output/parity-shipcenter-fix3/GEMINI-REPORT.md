# GEMINI-REPORT — PARITY-SHIPCENTER-FIX3

Card: PARITY-SHIPCENTER-FIX3 (PR #353, branch `unit/parity-shipcenter`)
Failing run: `41181778625` (head `d007cc6ed3bedb6ede0e7f3752a12409504ae4d9`)
Base: `origin/r3/integration` (`a9c3e65df`, base Gates `41190019292`)

| Failing job / test | Class | What you changed, file:line |
|---|---|---|
| `gate (bundle:06)` (`124481686118`) / `TestBrowserInboxUIRealChain` (`--browser-inbox`) | (c) Known base issue / upstream race in `tests/admin/inbox-browser-support.ts enterMessages` (`locator.click: Test timeout of 30000ms exceeded. waiting for getByRole('button', { name: 'Open navigation' })`) | Merged `origin/r3/integration` (`a9c3e65df`), which brings PR #356 fix `a122b3cd6` into `tests/admin/inbox-browser-support.ts:77-87`. Resolved merge conflict in `packages/i18n/src/index.ts:2-6` (retaining both `shippingCopy` and `entryParityCopy`, updating dependency comment header) and regenerated `output/ci-select-backend-browser/browser-ownership.json:1-56`. |
| `receipts` (`124486949073`) / `receipt-browser-inbox.json` check | (c) Downstream dependency failure | None; downstream of `gate (bundle:06)`. Documented in `output/parity-shipcenter/FIX3.md:27`. |
| `required` (`124486949072`) / workflow aggregator | (c) Downstream aggregator failure | None; downstream of `gate (bundle:06)` and `receipts`. Documented in `output/parity-shipcenter/FIX3.md:28`. |

STATUS: DONE
