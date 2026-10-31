# FIX-COD-422-RED lane report

| Finding | Class | Change |
|---|---|---|
| #422 item 2(b): COD bar label should be 到貨需付/到货需付/Due on delivery | a (test gap) | tests/storefront/home-cod-buyer.mjs place(): COD-selected footer assertions expect `c.due`, not 最終應付 (two spots) |
| #422 payment-mode save failure hidden / submit lacks worded reason | a | tests/storefront/home-cod-buyer.mjs: new `modeSaveFailure()` (injected setItem throw, visible alert, disabled submit with aria-describedby reason), run for orders A,B |
| Expected failures | - | output/fix-cod-422/red.log |

No product change; nothing run locally (CI only).

STATUS: DONE
