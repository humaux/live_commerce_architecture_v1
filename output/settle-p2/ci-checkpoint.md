settle-p2 CI handoff — head 4e447ea1d502f6de2799c7959978511043f9af4c

Confirmed RED (#220.3): Gates 40628121605 / g09 job 122621731233, at prior head caded567501aa169d47d0f3759e425167bc21bdb (synthetic checkout 749eff3a0baf71a33b32a593fb57c76eefd68f88). Both monetary-key assertions and the discovered return-function key set failed on their own expectations (E3). Full filtered receipt is committed under output/settle-p2/.

Remaining four items are now in one tests-only RED batch. Current Gates run 40642386557 and Foundation run 40642386434 are queued, bound to the head above. No current matrix job IDs allocated yet; these are workflow RUN IDs, not plan jobs.

WAIT_RUN: 40642386557 (Gates)
WAIT_RUN: 40642386434 (Foundation)

Integrator: relay Gates completion to resume; read failed assertion jobs before the single implementation push. Existing mapping remains intentionally unfixed so its verified RED remains present. Other expected negatives: PF16_fixed_Monday_T; GatePayableValues bad-field subtests; PF17a_r0_immutability; GateProbeVisibility. Implementation steps and PostgreSQL exception-rollback limits are in handoff.md. No local tests/servers run; gofmt and diff check exit 0; no files outside write paths. GREEN/contractdrift/current acceptance PENDING. No local background processes remain. GitHub workflow timeouts/completion notifications plus integrator relay provide the wake mechanism.

## Integrator ruling and implementation continuation — 2026-10-28
The previous STOP interpretation is corrected by the integrator's current dispatch: indented matrix FAIL lines are expected child-scenario output; the parent passes. Top-level g09 failures are only this card's six test functions. No unrelated blocker remains in that RED receipt.
All five #220 units implemented together after batch assertion RED 40642386557 / 122680364875 (head 4e447ea1d502f6de2799c7959978511043f9af4c; synthetic 875994d3c1fe21194e74c954da5c74167b3c8344).
Changes: fixed Monday paid_at; exact PF18 tenant/store/period/net/closed_at comparisons; distinct amount_minor/returned_minor keys with matching return inputs; r0 mutation guard before fixture deletion; query probe transaction before outer rollback. Probe runs real SQL under SET LOCAL ROLE commerce_payment_registrar, then resets role for owner-only observation. Savepoint recovery cannot reveal writes already undone by PostgreSQL failed-statement rollback; no such proof claimed. Existing post-rollback assertions remain.
Static gofmt and git diff --check exit 0. No new imports/dependencies, so dependency map unchanged. No out-of-scope paths, migrations or frozen contract edits. Local tests/servers NOT_RUN. Current GREEN/contractdrift/independent acceptance PENDING until implementation CI. No background local process. GitHub workflow timeout/completion notification and integrator relay provide wakeup.
