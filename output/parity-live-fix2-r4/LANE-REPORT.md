# PARITY-LIVE FIX2 relay 4 — head cef50227 (run 41255492647)

| Failing job / test | Evidence | Class | Change |
|---|---|---|---|
| gate --browser-live-console-parity: `panic: test timed out after 13m0s` in TestBrowserLiveConsoleParityRealChain; only ONE `=== RUN` appears, i.e. shard 1/3 (~37 serial real-click tests) alone overran 700s (~19s/test) | job 124698483936, log 00:16:46→00:29:46 | b | signed block now runs as 8 shards (~14 tests, ~270s each): scripts/dev/test-local.sh:549 (parity lc_run); comment tests/foundation/browser_live_console_test.go:~625. No test removed, no timeout/threshold raised. |
| receipts / required | only the `--browser-live-console-parity` FAIL receipt (`matrix-receipts: RED (1 problem)`) | derived | none |

Not run locally (owner rule). Ownership regenerated and checked.
Residual risk: per-test timing is invisible (diagnostics buffered); if a shard still overruns, a test is hanging, not slow.

WAIT_RUN
STATUS: DONE
