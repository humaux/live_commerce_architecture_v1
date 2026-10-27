# OC-09a delivery (order-page / pre-order COD copy)
Scope: apps/storefront/lib/cod-copy.ts, apps/storefront/components/CodOrderStatus.tsx, tests/storefront/cod-footer.spec.ts, output/oc-09/**, plus one integrator-granted line in scripts/dev/test-node.sh (registration + header list).
Copy (zh-TW = owner-approved OWNER-APPROVED-COPY.md; zh-CN/en same-meaning translations):
- new key `recordedPending` = 到貨時要付的金額, shown on the order page ONLY while collection is PENDING (CodOrderStatus passes it as `label`). Every other state keeps `recorded` = 原訂單代收金額 (and existing zh-CN/en), so the approved label never names an amount already paid or returned.
- `orderStates.PENDING` and `codNote` use the approved text. The checkout 「到貨需付」 (`due`, OrderFlow.tsx:553) is unchanged.
- Order-page amount is still the server `cod_collect_minor`, no arithmetic.
Footer 「到貨需付」 in the checkout bottom bar is DEFERRED to OC-09b (the quote/options carry no COD due; the server must add it with a contract amendment). CheckoutFlow.tsx and OrderFlow.tsx untouched.
RED: Foundation checks run 40453966936 (job 122022857940) on head 1ddd806ef, assertion failures on the new strings; see red.log.
GREEN: Foundation checks run on the fix head, id in the PR comment (a file cannot name its own head).
NOT_RUN: browser/390px visual check (no layout change); Gates browser suites are the integrator's.
Risks: home-cod-buyer.mjs matches only /現金/, /cash/i and 「已貨到收款」, which the new text still satisfies (inspection, not a run).
