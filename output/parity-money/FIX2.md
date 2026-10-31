# PARITY-MONEY FIX2 relay 2 — lane report

Head analysed: fcfa2f118 (run 41241671361). Only red: gate bundle:07 member `--browser-customers-billing`
(job 124657605330); `receipts`/`required` fail solely because that receipt is FAIL (job 124664145182). Merge of origin/r3/integration: already up to date.

| Failing job / test | Evidence | Class | Change |
|---|---|---|---|
| bundle:07 / TestBrowserCustomersBilling → `customers-billing.spec.ts:94` "PARITY-MONEY finance signed states (zh-TW, 1440)" | `toHaveAttribute('datetime')` Expected `…21:16:38.83020813Z` (first summary response), Received `…38.971227153Z` (second read, +140 ms); playwright.log:98-102 in artifact gate-browser-customers-billing, run 41241671361 | (a) test oracle pinned the first response, but the page re-reads when the server render key (`crypto.randomUUID()`, apps/admin/app/[locale]/customers/page-data.ts:68) changes | tests/admin/customers-billing.spec.ts:99-108 records every summary response and polls until the shown `datetime` equals one server `generated_at`; same intent (server time, not browser time), real clicks unchanged |
| receipts, required | `--browser-customers-billing: status FAIL` | follows from the row above | none |

NOT_RUN: all tests (CI only). The other 23 tests of that spec did not run in CI after the first failure, so further reds there are possible.

WAIT_RUN
STATUS: DONE
