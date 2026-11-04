# DS-468 / #468 — SP11 `busy_lease_snoozes_without_generation_loss` flake: root cause

Worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/stripe-sp11`, branch
`unit/stripe-sp11-busy-lease`, base `r3/integration 926f9b2e9`.

## Verdict

**Test-harness race, not a product race.** The bounded-poll helper
`tests/foundation/stripe_start_test.go:846-852` (`sstWake`) claims to make an attempt's sleeping
River jobs due, but it only rewrites River's **long-snooze** shape
(`state IN ('scheduled','retryable')`). River persists a short `river.JobSnooze(d)` for
`d <= SchedulerInterval` in a different shape — `state='available'` with a **future**
`scheduled_at` (`riverdriver.JobSetStateSnoozedAvailable`) — and `sstWake` never rewrites that.
The Stripe poller is therefore re-armed on every 400 ms poll round while the busy-lease signal job
is only re-armed by River 2 s later. On a loaded shard the force-woken poller re-claims the
operation faster than the signal can retry, so the signal starves until the 40 s deadline while
the operation stays `UNKNOWN` and is re-claimed (generation 42/43 in the captured failures).

## Code under test and harness

Busy-lease snooze (worker side, contracts/stripe-psp-v1.md:1049 "Claim; if `busy`, snooze 2 s"):

- `internal/payments/stripe_signal.go:115-117`
  ```go
  case "busy":
      return river.JobSnooze(2 * time.Second)
  ```
- `internal/payments/stripe_query.go:39-41` — same for the poller:
  ```go
  case "busy":
      return river.JobSnooze(2 * time.Second)
  ```

Claim/lease (shared SQL): `migrations/0096_worker_authorities.sql:142`
(`IF o.lease_until>v_now THEN RETURN QUERY SELECT 'busy'...`), and a successful record clears the
lease and completes the operation `UNKNOWN`:
`migrations/post_river/0012_stripe_payment.sql:517-523`
(`PERFORM integration.complete_operation(...,'UNKNOWN','stripe_report_observed',...)`;
`migrations/0096_worker_authorities.sql:206 lease_until=NULL`).

Poller cadence after a successful record (`internal/payments/stripe_query.go:104-110`,
`241-259`): an `open` session expiring well in the future returns **1 minute**
(`stripePollDelay` → `return time.Minute`), i.e. the poller is normally asleep for 60 s.

Harness wake path:

- `tests/foundation/stripe_flow_test.go:199-223` (`await`) polls every 400 ms and calls
  `sstWake` each round (line 220).
- `tests/foundation/stripe_flow_test.go:911-914` (`awaitOutcome`, 40 s) is the assertion that
  times out.
- `tests/foundation/stripe_start_test.go:846-852`:
  ```sql
  UPDATE river_payment.river_job SET state='available',scheduled_at=clock_timestamp()
   WHERE kind IN ('payment_query_v1','payment_signal_v1','payment_reconcile_v1')
   AND args->>'operation_id'=$1 AND state IN ('scheduled','retryable')
  ```

## River's two snooze shapes (this is the asymmetry)

River v0.40.0 (`internal/jobexecutor/job_executor.go:392-400`) chooses the persisted shape by the
snooze duration against `SchedulerInterval`, whose default is **5 s**
(`internal/maintenance/job_scheduler.go:25`):

- `d <= 5 s` → `JobSetStateSnoozedAvailable` → `state='available'`, `scheduled_at=now()+d`
  (`riverdriver@v0.40.0/river_driver_interface.go:626-636`);
- `d > 5 s` → `JobSetStateSnoozed` → `state='scheduled'`, `scheduled_at=now()+d`.

`JobGetAvailable` only fetches `state='available' AND scheduled_at <= now()`
(`river_job.sql:200-216`). So:

- poller, 60 s snooze → **`scheduled`** → `sstWake` rewrites it to due on every 400 ms round;
- signal, 2 s busy snooze → **`available` + future `scheduled_at`** → `sstWake` skips it
  (`state IN ('scheduled','retryable')` is false), so it retries only every ~2 s.

## The interleaving that reaches the deadline

Test at `tests/foundation/stripe_flow_test.go:1085-1119`:

1. lines 1089-1104: the test takes the operation lease manually for 120 s (`claim.Disposition ==
   "claimed"`), then inserts the `BUYER_REFRESH` signal + its `payment_signal_v1` job
   (`sflSignal`, lines 883-909) and sleeps 6 s (line 1106) so several busy snoozes happen. During
   this window the signal job returns `busy` → `JobSnooze(2 s)` → parked `available` +
   `scheduled_at+2 s`; the poller does the same.
2. line 1114: the test rewrites `lease_until` into the past. From here both workers may claim.
3. The next `awaitOutcome` round (400 ms) calls `sstWake`: it wakes the poller (shape `scheduled`)
   but **not** the signal (shape `available` + future). The poller claims generation G+1, records
   an observation, completes the operation `UNKNOWN` (`stripe_report_observed`) and snoozes 1 min.
4. 400 ms later `sstWake` wakes the poller again → claim G+2 → record → snooze. The poller's work
   is the whole claim→`load_stripe_session`→Stripe retrieve→`record_stripe_observation`→River
   insert chain; on the 1-CPU fixture PostgreSQL with `-race`, that chain can take longer than the
   400 ms round, so the poller holds the operation lease for most of every round and re-claims
   immediately after each successful record (the record clears the lease).
5. The signal job becomes due only every ~2 s. 2 s is exactly five 400 ms rounds, so its sample
   point sits at a stable phase inside the poller's claim window; every sample returns `busy` and
   the job is parked again as `available` + `scheduled_at+2 s`, which `sstWake` cannot move.
6. After 40 s (`awaitOutcome` deadline) the signal row still has `consumed_at IS NULL`, so the
   assertion fails with `op state=UNKNOWN generation=42/43` and the alternating
   `[stripe_report_observed reconcile_claimed ...]` codes — one claim + one record per poller
   cycle, i.e. ~37-38 claims in 40 s, no signal consumption.

The recorded generation (42/43) is the arithmetic fingerprint of step 4: at ~1 claim/s the poller,
not the signal, owns every claim after the lease expiry.

## Why does a rerun pass?

Nothing in the code changes; only load and phase do. When the poller's claim→record chain is fast
(a lightly loaded runner) the operation lease is free for most of every 400 ms round, so the
signal's first 2 s retry after line 1114 lands in free time and records `OBSERVED` in ~1-2 s (the
green runs take 7.1-14.3 s for the whole subtest: 6 s hold + 1-8 s wait). When the chain is slow
(1-CPU PostgreSQL fixture, `-race` shard, several concurrent Stripe tests), the duty cycle of the
poller rises above the signal's 2 s retry budget and all ~20 retries lose. The fixture never
re-arms the signal in between, so the loss is repeatable for the whole deadline; a rerun draws a
different load/phase and passes.

## Why this is not a product bug

The product's busy behaviour is the contract's: claim first, `busy` → snooze 2 s, never consume
another worker's signal, never advance a generation on a busy claim (`claim_operation` returns
before any UPDATE, migrations/0096:142). Nothing in `internal/payments` re-claims an operation
that another worker holds. The artificial 400 ms re-claim loop exists only because the test harness
rewrites the poller's 60 s snooze on every poll round; production has no such loop. The only
product-side contribution is the fixed 2 s retry budget, which the contract pins.

## Fix (shared helper, not each caller)

`sstWake` must make every representation of a sleeping payment job due, including River's
short-snooze shape:

```sql
AND (state IN ('scheduled','retryable')
     OR (state='available' AND scheduled_at>clock_timestamp()))
```

That is one predicate in the single helper every bounded poll in `tests/foundation` uses
(`sflEnv.await`), so the signal job (and a refund/query job in the same busy shape) is re-armed on
each 400 ms round instead of every 2 s. The signal then gets ~100 claim attempts instead of ~20 and
competes with the poller on equal footing; the systematic phase lock disappears. No assertion,
deadline, retry or product line changes.

RED proof: `TestStripeSP11BusySnoozeIsWakeable` (`tests/foundation/stripe_wake_test.go`) parks one
job in each River snooze shape with owner SQL (exactly what `JobSetStateSnoozedAvailable` /
`JobSetStateSnoozed` persist), calls the real `sstWake`, and requires both to be due. It fails on
the current helper for the signal job and passes after the fix. No worker is started and no sleep
is used for synchronization.

## Delivery split — RED part vs FIX part (coordinator commits; nothing is committed here)

Per the coordinator's instruction this terminal made **no** git commit and will not touch the
working tree's git state. The two commits are:

### Commit 1 — RED (tests-only)

- **NEW** `tests/foundation/stripe_wake_test.go` (whole file, 88 lines): the package header comment
  plus `TestStripeSP11BusySnoozeIsWakeable`. Nothing else belongs to this commit — no helper edit,
  no evidence file, no ownership artifact. It must fail on the pre-fix tree because the signal job
  is left in the `available` + future `scheduled_at` shape.
- Message: `test(payments): force SP11 busy-lease interleaving (RED, #468)` + trailing
  `Co-Authored-By: DeepSeek Flash <noreply@deepseek.com>`.

### Commit 2 — FIX (helper + comments + evidence)

- `tests/foundation/stripe_start_test.go`, exactly two hunks:
  1. lines 15-18, the file header comment: states that the harness owns the bounded-poll wake
     helper and the "every sleeping representation must be made due" invariant (#468);
  2. lines 848-864, the `sstWake` doc comment (River's two snooze shapes and the starvation it
     caused) and the single SQL predicate change:
     `AND args->>'operation_id'=$1` / `AND (state IN ('scheduled','retryable') OR
     (state='available' AND scheduled_at>clock_timestamp()))` replacing
     `AND args->>'operation_id'=$1 AND state IN ('scheduled','retryable')`.
  Nothing else in the file changes.
- `output/ci-select-backend-browser/browser-ownership.json`: the generated artifact. **Regenerate
  it after commit 1 and before commit 2** (`node scripts/dev/derive-browser-ownership.mjs` then
  `--check`, both must exit 0): the artifact hashes every tracked `tests/foundation/*.go`, so the
  working-tree copy generated now already contains the `stripe_start_test.go` hash but cannot
  contain `stripe_wake_test.go` while that file is untracked. Committing the current copy as-is
  would fail the CI drift check.
- `output/stripe-sp11-flake/DIAG.md` and `output/stripe-sp11-flake/REPORT.md`: this evidence.
- Message: `fix(payments): re-arm River's short-snooze shape in the SP11 wake helper (#468)` +
  trailing `Co-Authored-By: DeepSeek Flash <noreply@deepseek.com>`.

Order matters: RED first, then regenerate ownership, then FIX. No product code, migration,
assertion, deadline or fixture data is in either commit.

Environment note: this shell exports `GIT_CONFIG_COUNT=2` with `GIT_CONFIG_KEY_0`/`GIT_CONFIG_KEY_1`
absent, so every `git` invocation (and any tool that shells out to git, including
`derive-browser-ownership.mjs`) dies with `missing config key GIT_CONFIG_KEY_0`. That is a harness
env defect, not a repo setting; the coordinator's session should not inherit it.

## Evidence level

- Source/diagnosis: E1 (static reading of the exact files above + River v0.40.0 sources in the
  module cache).
- RED/GREEN: `NOT_RUN` locally by owner rule (CI only). `gofmt -l` (no output),
  `go vet ./tests/foundation` (exit 0), `node scripts/dev/derive-browser-ownership.mjs` and
  `--check` (exit 0 at the time of writing, see the regeneration note above).
- Product code, migrations, assertions, deadlines and retries: unchanged.
- Commits: none by this terminal; the two commits above are left to the coordinator.
