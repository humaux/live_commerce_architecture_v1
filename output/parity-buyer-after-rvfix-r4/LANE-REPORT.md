# RVFIX-444 relay 4 lane report

CI on head 02fc74b6 (run 41262875311): only `gate (bundle:07)` failed (so `receipts`/`required` follow); every other shard and the buyer-after-parity members are PASS.

| Failing job/test | Class | Finding / change |
|---|---|---|
| gate (bundle:07) → `--browser-cvs` → `TestBrowserTaiwanCvs/WebKit` (55.9s, `browser_taiwan_cvs_test.go:159`: `tests/storefront/cvs-buyer.mjs` exit 1). Chromium `TestBrowserTaiwanCvs/MOCK` PASSED in the same job (78s). | c (unrelated / suspected WebKit flake) | `cvs-buyer.mjs` drives checkout (`create-order` → `order-section` = `OrderDetails`, which this PR keeps) and the CVS map round trip; it never visits `/orders/[id]` (OrderStatusView/OrderTerminalView, the only RVFIX-444 files) and asserts no changed copy. The script's own log (`cvs-buyer.mjs.log`) is an unpublished CI artifact, so the failing step is not visible in the job log; no root cause can be shown from a source change. No code changed (no evidence-based fix exists). |
| receipts / required | follow-on | Red only because the `--browser-cvs` receipt is FAIL; clears when that mode passes. |

Browser ownership: derive-browser-ownership.mjs and --check run; see commit.

Integrator: re-run the failed job `gate (bundle:07)` on the same head. If WebKit CVS fails again, pull `cvs-buyer.mjs.log` from the evidence artifact (not to be committed) and send it back.

WAIT_RUN
STATUS: DONE
