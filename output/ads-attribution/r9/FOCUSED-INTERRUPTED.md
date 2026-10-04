# Interrupted parallel focused run — NOT acceptance

`focused-final-source.log` on source `492aaa30` is **INTERRUPTED**, regardless
of its misleading `exit=0` trailer. The root stopped its own runner (PID 9382)
and child `go test` (9427) with TERM to remove overlapping PG load. The log
ends during AT4/AT9 and contains `Terminated: 15`, not a completed suite.

The root initially assumed the shared PG lock covered both commands. Source
inspection corrected that assumption: `test-local.sh:437` acquires this lock
only for `--stripe-browser` (plus a separate platform-site branch), not ordinary
browser modes or G07. Therefore this was **not** a browser lock being deleted.
Separately, `test-focused.sh`'s stale-PID command check recognizes only its own
script name; that is an additional shared-harness risk, not a proved event here.
No lock was manually removed and no foreign process was stopped.

An independent integrator G07 was observed in `.worktrees/r3-integration`
(runner PID 91790, test PID 94820). Leave it untouched. All subsequent root PG
commands must wait manually for active PG suites, run serially, and the final
G07 must wait for a quiet machine. Shared-harness hardening is outside this
attribution unit; do not change it to manufacture acceptance.

The interrupted run's orphan `lc-payment-worker-e6967a0cb00a` was confirmed as
this unit's exact AT4 fixture: matching start time, zero remaining DB clients,
0113 `orders.order_attribution` (absent on r3/integration), two ad drafts and
three intake rows. Root removed only that labelled temporary container. Its
synthetic data is reproducible by the test; no business data was removed.
