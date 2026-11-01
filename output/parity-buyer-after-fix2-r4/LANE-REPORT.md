# PARITY-BUYER-AFTER FIX2 r4 — lane report

CI head a36bd52d9 (run 41254330184): only `gate (--browser-buyer-after-parity)` failed (job 124695018607); receipts/required fail only because of it. The four older bundle members (storefront, e2e, returns-ui, live-claims) passed in this run.

| Failing test | Evidence | Class | Change |
|---|---|---|---|
| `payment hand-off page · home` and `· cvs` (tests/storefront/buyer-after-parity.mjs:402) | Screenshot 29-1064.png shows the pay panel with "暫時無法確認付款狀態…" (opFailed) instead of 前往付款, so `payOrder` threw. Root cause: the mock `prepare` left `methods: [stripe_checkout]` on a PENDING/READY view; `validOrderPayment` (apps/storefront/lib/payment-contract.ts) rejects a non-fresh view that lists methods, so `stripeHandoff` re-read threw `invalid_response`. The real server clears methods on prepare. | mock fidelity (test-support, no product change) | tests/storefront/buyer-after-parity.mjs (`prepare` branch of the mock): `methods: []` added to the prepare transition. |
| receipts / required | fail only on `--browser-buyer-after-parity` FAIL receipt | derived | none |

Also: merged origin/r3/integration, resolved the browser-ownership.json conflict by regeneration (`derive-browser-ownership.mjs` then `--check` clean).

Tests were not run locally (owner rule); verification is CI.

STATUS: DONE
