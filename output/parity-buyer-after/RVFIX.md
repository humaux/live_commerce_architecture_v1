# RVFIX-444

| Finding | Change (file:line) | RED run | GREEN run |
|---|---|---|---|
| P1 shipped refunded card order shows 已付款, no refund line | push 1: tests/storefront/buyer-after-parity.mjs (makeOrder pay.refund; 4 new `shipped card order` cases). Impl: OrderStatusView.tsx (shippedPay read + loading gate before the completed return) and OrderTerminalView.tsx (payLabel; refund-processing / refund-succeeded lines) | RED: CI head f69c135f, gate --browser-buyer-after-parity (see red.log) | pending CI on the implementation push |
