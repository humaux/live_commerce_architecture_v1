# PARITY-BUYER-AFTER FIX2 r1 — lane report

Run read: 41247231613 (head 8a9e6246). The jobs listed in the card's FAILS (e2e, returns-ui, live-claims) were from an older head and are green on this head; merge of origin/r3/integration was already up to date.

| Failing job / test | Evidence | Class | Change |
|---|---|---|---|
| foundation: `all eleven gate adapters preserve original assertions` (tests/ci/next-startup.test.mjs:284) | `buyer-comms-gate.mjs: original gate assertions changed (missing: expect(b.getByTestId("order-id")).toHaveText(orderID))` | a (signed frame replaced the full order-id row with the mail-number heading; the gate was already rewritten to the heading) | tests/ci/next-startup.test.mjs: `SIGNED_FRAME_REWRITES` maps the two baseline selectors/matchers to their rewritten form for that one file; every other baseline assertion is still compared unchanged |
| gate buyer-after-parity: `payment hand-off page · home/cvs` | screenshot 29-1064: page stayed on 完成付款 with 暫時無法確認付款狀態 (hand-off aborted) | test bug in my spec: it closed the provider tab the instant it opened, OrderPayment fences on `destination.ready()` and treats a closed tab as aborted | tests/storefront/buyer-after-parity.mjs: keep the provider tab open until the hand-off page is checked, close in `finally` |
| gate buyer-after-parity: `terminal order view · completed` | screenshot 63-1475: 無法載入訂單 | fixture invalid: `validShipment` (apps/storefront/lib/purchase.ts:339) rejects `carrier_code:"other"` with `carrier_name:null` | tests/storefront/buyer-after-parity.mjs: fixture `carrier_name: "其他物流"` |
| gate bundle:02 / `--browser-storefront` PARITY-STATES `expired-*` | `waiting for getByTestId('order-link').getByRole('link', { name: '前往訂單查詢' })` | a (signed frame 26:746 button reads 前往查詢訂單 / 前往查询订单 / Go to order lookup; heading and body asserts already pass) | tests/storefront/shop-real-gate.mjs: per-locale `expiredGo`, used for the expired recovery click |
| receipts / required | follow from the jobs above | n/a | none |

Not run (owner rule: CI only). Ownership regenerated and checked.

STATUS: DONE
