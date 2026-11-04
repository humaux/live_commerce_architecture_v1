# PARITY-LIVE FIX2 relay 3 — head 6ad8d60f (run 41253058639)

| Failing job / test | Evidence | Class | Change |
|---|---|---|---|
| foundation: pr-modes "round 3: real PR #30 diff…" and "live_stream transport alone…" (ceiling 28, got 29) | job 124690400260, tests/ci/pr-modes.test.mjs:155,170 | a (new sibling mode legitimately selected) | ceilings 28→29 with PARITY-LIVE comment, tests/ci/pr-modes.test.mjs:155,170 |
| foundation: "metareply change selects exactly…" and "UI selections union…" | same job; actual list contains `--browser-live-console-parity` | a | expected lists now include the parity mode, tests/ci/pr-modes.test.mjs:247,692 |
| gate --browser-live-console-parity: `panic: test timed out after 13m0s` in TestBrowserLiveConsoleParityRealChain (cmd.Run, browser_live_console_test.go:637) | job 124691041977; ~110 serial tests (workers: 1) exceed one 700s budget | b | signed block runs as 3 independent go test shards (`--shard`, env LC_CONSOLE_PARITY_SHARD), each with its own 780s budget: tests/foundation/browser_live_console_test.go (~line 640), scripts/dev/test-local.sh parity lc_run. No test removed, no timeout raised. |
| receipts / required | only the `--browser-live-console-parity` FAIL receipt | derived | none |

Not run locally (owner rule). Ownership regenerated and checked.
Residual risk: if a shard still exceeds 700s the next step is more shards; per-test timing is not visible (diagnostics are buffered).

STATUS: DONE
