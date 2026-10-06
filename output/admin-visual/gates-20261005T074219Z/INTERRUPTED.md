# Interrupted by turn cancellation

Source `e85654d00f684a855f7600d5af94a3ac29c67f46`. All rows already present in `results.tsv` are completed commands with their actual exit codes. `--browser-click-sweep` did not finish: its log stops around703seconds and no final TSV row/exit exists. It is **INTERRUPTED**, not PASS or a completed FAIL verdict.

After the interruption the root runner process was absent. The only confirmed orphan in this worktree was its production admin server wrapper49745/child49746; exact parent/cwd were checked and both were gracefully terminated. No foreign process or shared lock was removed.

The supplementary `080801Z` runner was paused to avoid concurrent builds during this sweep. It also disappeared during the interruption. Its merchant-orders child had already printed an actual PASS/exit0, but the paused parent had not appended a TSV row; that raw log remains intact. Do not synthesize a missing runner record.

Integrator subsequently requested a rebase; all evidence here is historical and cannot certify the rebased source. A new pinned run is required.
