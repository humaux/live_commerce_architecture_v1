# FIX-COD-422 relay 1 lane report

| failing job/test or finding | class | what changed (file:line) |
|---|---|---|
| foundation: tests/admin/shell-architecture.test.mjs, OrderFlow.tsx lines 855 > 854 | a (my change grew the file) | OrderFlow.tsx:231-233 comment trimmed from 4 to 3 lines |
| gate bundle:03 / TestBrowserHomeCod: checkout-v2.mjs:393 "zh-TW/1440/cash_on_delivery required facts" looked for 最終應付 in the COD bar | a (stale expectation of the old label, now `due.label`) | tests/storefront/checkout-v2.mjs:399-401 add `codDue`, :479 COD expects `codDue`, other modes `total` |
| receipts, required | c (cascade of bundle:03 FAIL) | none |

Not re-run locally (owner rule); GREEN needs CI on the next head.

STATUS: DONE
