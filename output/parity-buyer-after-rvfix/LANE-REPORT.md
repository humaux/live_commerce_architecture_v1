# RVFIX-444 lane report (push 1, tests only)

| Finding | Class | Change |
|---|---|---|
| P1 shipped refunded card order shows 已付款 / no refund line | a (RED first) | tests/storefront/buyer-after-parity.mjs: pay.refund in makeOrder; 4 new cases (refunded, partially-refunded, refund-processing, review-required); expected failures in output/parity-buyer-after/red.log |

RED pushed; implementation (OrderStatusView.tsx:90 / OrderTerminalView.tsx:44) is for the relay's next push. Tests not run locally (owner rule).

STATUS: DONE "RED pushed"
