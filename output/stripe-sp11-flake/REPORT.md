# DS-468 / #468 delivery report

Worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/stripe-sp11`, branch
`unit/stripe-sp11-busy-lease`, base `r3/integration 926f9b2e9`.

## Status

**Delivered as uncommitted working-tree changes, per the coordinator's instruction (no git commits
from this terminal).** The DSH sandbox denies writes to the worktree's git metadata
(`fatal: Unable to create '/Volumes/data/live_commerce_architecture_v1/.git/worktrees/stripe-sp11/index.lock':
Operation not permitted`); the one-shot escalation to `danger-full-access` was declined, and the
coordinator then asked for the split instead of commits. `output/stripe-sp11-flake/DIAG.md`
("Delivery split") names the exact RED commit (one new test file) and FIX commit (two hunks in
`stripe_start_test.go` + regenerated ownership + this evidence), including the required order:
RED commit -> regenerate `browser-ownership.json` -> FIX commit. Nothing is committed or pushed.

## Root cause (test-harness race)

`sstWake` (`tests/foundation/stripe_start_test.go:846-852`, before this change) only re-armed
River's long-snooze shape `state IN ('scheduled','retryable')`.

- Busy-lease path: `internal/payments/stripe_signal.go:115-117` returns
  `river.JobSnooze(2 * time.Second)`.
- River v0.40.0 persists a snooze `<= SchedulerInterval` (default 5 s,
  `internal/maintenance/job_scheduler.go:25`) as `state='available'` with a **future**
  `scheduled_at` (`job_executor.go:396-400` + `riverdriver.JobSetStateSnoozedAvailable`,
  `river_driver_interface.go:626-636`); `JobGetAvailable` needs `scheduled_at <= now`
  (`river_job.sql:207-209`). So `sstWake` never moved it.
- The poller's successful record snoozes `stripePollDelay` = 1 minute for an open session
  (`internal/payments/stripe_query.go:241-259`) => shape `scheduled` => `sstWake` re-armed it on
  every 400 ms poll round (`tests/foundation/stripe_flow_test.go:220`).

Interleaving after the test expires the manual lease (`stripe_flow_test.go:1114`): the poller claims
(`migrations/0096_worker_authorities.sql:142`), records and completes the operation UNKNOWN
(`migrations/post_river/0012_stripe_payment.sql:517-523`), which clears the lease
(`0096:206`), then snoozes 1 min; the next 400 ms round wakes it again. The signal job samples the
lease only every ~2 s (exactly five 400 ms rounds => stable phase) and always loses; the operation
stays UNKNOWN and is re-claimed (generation 42/43, ~1 claim/s). `awaitOutcome`
(`stripe_flow_test.go:911-914`) hits its 40 s deadline.

A rerun passes because the poller's claim→record duty cycle is load-dependent: on a fast run the
lease is free for most of each round, so the signal's first 2 s retry records OBSERVED (~1-2 s).

This is not a product race: a `busy` disposition never mutates, consumes or advances a generation
(`claim_operation` returns before any UPDATE), and the 2 s snooze is the frozen contract
(`contracts/stripe-psp-v1.md:1049`).

## RED proof

`TestStripeSP11BusySnoozeIsWakeable` (`tests/foundation/stripe_wake_test.go`, new) parks the
attempt's `payment_signal_v1` job in River's short-snooze shape and its `payment_query_v1` job in the
long-snooze shape with owner SQL (no worker is assembled, no sleep is used), calls the real
`sstWake`, and requires both to be due. It fails on the current helper for the signal job.

## Fix

`tests/foundation/stripe_start_test.go`: one predicate in the shared wake helper plus header
comments (file header and helper comment state the two snooze shapes and the starvation invariant).

```sql
AND (state IN ('scheduled','retryable') OR (state='available' AND scheduled_at>clock_timestamp()))
```

No product code, assertion, deadline, retry or fixture data changes.

## Checks run (no tests executed, owner rule)

- `gofmt -l tests/foundation/stripe_start_test.go tests/foundation/stripe_wake_test.go` -> no output.
- `go vet ./tests/foundation` -> exit 0 (compile-only).
- `node scripts/dev/derive-browser-ownership.mjs` -> wrote
  `output/ci-select-backend-browser/browser-ownership.json`; `--check` -> exit 0.
  Note: the new test file is untracked in this worktree, so ownership must be regenerated once more
  after the RED commit makes it tracked.
- `NOT_RUN`: any `go test`, servers, Playwright, local CI reproduction (CI only).

## Commit commands (for the coordinator; not executed here)

RED (test file only):

```sh
git add tests/foundation/stripe_wake_test.go
git commit -m "test(payments): force SP11 busy-lease interleaving (RED, #468)" \
  -m "Co-Authored-By: DeepSeek Flash <noreply@deepseek.com>"
```

Then regenerate ownership (the new file is now tracked) and verify:

```sh
node scripts/dev/derive-browser-ownership.mjs
node scripts/dev/derive-browser-ownership.mjs --check
```

FIX (helper hunks + evidence + regenerated ownership):

```sh
git add tests/foundation/stripe_start_test.go output/stripe-sp11-flake output/ci-select-backend-browser/browser-ownership.json
git commit -m "fix(payments): re-arm River's short-snooze shape in the SP11 wake helper (#468)" \
  -m "Co-Authored-By: DeepSeek Flash <noreply@deepseek.com>"
```

The exact RED/FIX file-and-hunk split is in `output/stripe-sp11-flake/DIAG.md` ("Delivery split").
This shell exports `GIT_CONFIG_COUNT=2` without `GIT_CONFIG_KEY_0/1`, which breaks every git
invocation and any tool shelling out to git (including the ownership script); the coordinator's
session should not inherit that env.

## Risks

- Test-only change; the product busy/claim contract is untouched.
- Waking short-snoozed jobs also re-arms 5 s error-snooze retries earlier in other Stripe tests;
  those waits already force-wake `scheduled`/`retryable` jobs, so the delta is bounded to short
  snoozes of the three payment kinds.
- The RED test asserts harness state, not end-to-end timing; it is complemented by the existing
  subtest, which must stay green in CI.

STATUS: DONE — diagnosis, RED test, fix and evidence are in the working tree and compile-checked;
the two commits are delegated to the coordinator (no git writes from this terminal, per its
instruction).
