# RVFIX-444

| Finding | Change (file:line) | RED run | GREEN run |
|---|---|---|---|
| P1 shipped refunded card order shows 已付款, no refund line | push 1: tests/storefront/buyer-after-parity.mjs (makeOrder pay.refund; 4 new `shipped card order` cases). Implementation: pending (relay pushes after RED) | pending CI (see red.log) | pending |
