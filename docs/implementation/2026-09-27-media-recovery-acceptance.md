# MRR90 restart state-check acceptance

Status at 2026-09-27 11:41 UTC: **FOCUSED_ACCEPTED / FULL_REGRESSION_PENDING**.
The independently frozen design is on `598eea4`. Source `86b641a` and independent
tests `f3b731b` with evidence `2e00b00a` are integrated as `74459ae`.
The initial independent and root focused runs each passed 17 tests. Independent
supplement `cf4720e` passed 21 tests; root fixed-tree full regression remains
required. No production configuration,
customer broadcast, provider account or River policy was changed.

## Meaning of the owner's 90-second target

Within 90 seconds of the supervisor's monotonic start, obtain committed fresh
state for the admitted original media operation, or retain unresolved custody
and emit a deadline alarm. This is not a promise to stop a broadcast or finish
cleanup in 90 seconds. Known-empty scope must not create a resource alarm;
unknown coverage must not masquerade as known-empty or complete recovery.

Contract: `contracts/live-media-recovery-observer-v1.md`.
Owner decision: Humaux `f0b36444-a4ea-4112-926d-596cfb7fd89f`.
Independent design freeze: `50f8e16c-88cb-4d18-99d1-856c2bc9eeb0`.

## Fixed first candidate and actual run

- Source: `d0cb89642862474643dd13a5f3ae816edf7b94bf`, including SQL `ffc1f67`.
- Independent test author: `04cb1359e6378e3ae4cf9d6184e93a1110d58616`.
- Isolated integration under test: `44d75be` in
  `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`.
- Command: `bash scripts/dev/test-local.sh --live-media-recovery`.
- Actual exit: **1**; foundation package duration **75.835 seconds**;
  **11 top-level FAIL / 0 PASS**. This is not 90-second acceptance evidence.
- Log: `output/mrr-focused-44d75be.log` in that test worktree.
- SHA-256: `33212559b393a0d33be13486527a4570c10138b1383de12859d7d0c7192e7543`.
- The test worker reported its fixture removed by the runner trap. Root has
  read the fixed-source test changes and actual failure log. Root has not yet
  rerun the candidate or independently completed its process acceptance.

The repeatable SQL failure is PostgreSQL `42702`: the admission query's
`x.operation_id` collides with its PL/pgSQL record variable. Earlier output
column ambiguities were repaired incompletely. The three process tests could
not obtain an episode and ended before their intended deadline scenarios.

A separate `23514` at new test line 548 is a **test fixture defect**, not proof
of a legacy Stop defect: clearing only `lease_until` violates the existing
atomic lease tuple constraint. Repair the fixture through a valid transition;
do not relax `operations_check` or change a process test's clocks.

## Review and repair boundaries

The first SQL-only review (`050b8bad-a341-490d-b6ed-e5016daf4131`) blocked on
name ambiguity, terminal labels inferred from incoherent raw provider times,
and witness-first/timeout lock interleaving. Candidate `d0cb896` contains text
repairs, but the admission ambiguity above still blocks execution. The second
targeted ambiguity repair must cover aliases and output variables throughout
the affected function, not just its first failing expression.

The complete candidate also needs its frozen child control protocol restored:
parent-to-child stdin release before work and EOF cancellation, not a
child-to-parent readiness ACK. Restart admission and a deadline alarm that
does not wait behind database I/O require independent review. Review findings
remain open until a fixed revision and its counterexamples are verified.

| Gate | Evidence currently available | Acceptance |
| --- | --- | --- |
| MRR01 original-job real crash and fresh ROOM/QUERY readback | Tests authored; process setup blocked before episode | NOT_RUN beyond failed setup |
| MRR02 real 90-second miss, scope and failure cases | Tests authored; SQL admission failed | FAILED / remaining cases NOT_RUN |
| MRR03 authority, fences, cleanup custody | Static review and authored negatives; common SQL failure plus fixture defect | FAILED / incomplete |
| MRR04 migration, admission, lifecycle, root and full regression | Candidate Go packages compile; no accepted complete integration | NOT_ACCEPTED |

## Independent obligations still open

Original LMR05 full-run evidence remains **668 PASS / 1 FAIL / 0 SKIP** at
`/Volumes/data/output/bic-root-full-frozen-20260927.log`; its original
35-second predicate is unchanged. MRR90 does not waive that failure.

Studio B's candidate remains unmerged. Its core browser readback and limited
visual review are not a pass for native calendar/concealment gates. See
`2026-09-27-studio-ui-acceptance.md` for those separate obligations.

Next: one coordinated source/test repair, fixed-source independent review,
isolated focused run, root rerun, then fixed-tree full regression. Do not edit
an executing runner, discard the red run, or infer real-provider/human-alert
delivery from local MOCK tests.

## Second source candidate: root unit check only

Root independently ran source `7987f5055af745c65273e9b8fb37bb31b7245285`
with a clean, author-frozen worktree before and after the command:
`GOTOOLCHAIN=go1.27.1 go test -race -count=1 ./cmd/media-worker ./internal/live ./internal/platform ./internal/integrations/livekit`.
Actual exit **0**; all four packages passed (2.231 / 1.402 / 1.643 / 12.709s).
Log: `/Volumes/data/output/mrr-unit-7987f50-root-20260927.log`, SHA-256
`076f74a3701f408714586ab1c61de176096a36b58e5155a5ec3fe69a6637345d`.

This does not close PG or real-clock gates. Independent review still blocks on
timely readback attestation: sample once immediately after a committed batch
read, preserve that time for every member, and distinguish pending Witness
persistence from an actual missed readback deadline. A later synchronous
Witness call must not make an already-read member appear late. Tests remain
fixed separately at `2754509`; the next PG run awaits that source repair.

## Second focused run: RLS visibility defect, not acceptance

The targeted timing review of source `b90d1914602d09e5dbca21ec6b2082e9ce784dd5`
closed that specific static P1 (receipt `8dfcde71-3b68-42e5-97fd-9f8dd676d438`).
It did not approve the whole runtime. Independent tests `2754509` and that source
were merged only in the isolated test branch as
`804065e302a1f53de08920d7c91a27d8b5e3a5bb`.

- Command: `bash scripts/dev/test-local.sh --live-media-recovery`.
- Actual exit **1**; package duration **241.417s**. Completed top-level results:
  **1 PASS / 11 FAIL**, plus the final real-90-second test interrupted by the
  outer Go test runner's **240s** timeout. No complete real-90-second verdict.
- Log: `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926/output/mrr-focused-804065e.log`.
- SHA-256: `2e32c15b280371987388ee22252774aa345288f646019eed76e2137348c530d7`.
- Root independently read the failures and verified the log hash. Test worker
  reported task fixture cleanup; protected upgrade fixture remained untouched.

The earlier `42702` is absent. The newly reached claim stage fails with
`ME409: media recovery membership unavailable`. Root and source author found
that `commerce_media_writer` has a table SELECT grant on operation events but
no SELECT RLS policy. Its SECURITY DEFINER functions cannot see admitted events
written by the same role. This affects admission replay, qualification,
readback, witness and timeout, not just the first failed claim.

The independent reviewer also identified an unsafe empty-set inference:
timeout can return `already_finished` for nonzero captured membership when
RLS hides all admitted events; read can infer completion from a visible subset.
Repair must add only the matched media-event read policy, pin it in readiness,
and fail closed when a member-bearing scope's visible membership differs from
its recorded count. Prior-episode checks must not overwrite hidden unresolved
members. Known-empty and fail-whole capacity/prior scopes remain distinct.

A separate negative-test setup fails with `42501` because resolving a function
by name requires schema USAGE absent from the old buyer/meta roles. Resolve its
OID as fixture owner and test privilege by OID; do not grant those roles USAGE.

The measured serial process cases can exceed the outer 240s budget. Root
authorized **360s only for the focused selector's execution timeout**. The
owner's 90s deadline, original LMR05 35s predicate and default full-run 900s
timeout are unchanged. Fixed-source repair, independent review, focused/root
reruns and full regression remain required. Neither candidate is merged into
main, and no production/customer state was changed.

## Final RLS candidate and third focused receipt

Source `86b641a909277223fa630022cd4d0df6f7d22ad8` closes the narrow RLS,
membership-count and readiness checks. After two targeted repairs, root
adjudicated the remaining definer-role baseline check rather than continuing
an automatic repair loop. Readiness now also rejects extra applicable policies
and elevated definer role flags. Independent static receipt:
`e54b77ce-3b22-42b6-90ee-25c6ca0189fd`; **STATIC_CONDITIONAL_GREEN**, not runtime acceptance.

Root independently reran the four Go race packages at that clean fixed source:
**exit 0**, package times 2.102 / 2.361 / 1.411 / 14.177s. Log
`/Volumes/data/output/mrr-unit-86b641a-root-20260927.log`, SHA-256
`f997eea1848bee12683f4edd4d9fa0da5ccee789296a0e02ff74d241a66434a9`.

Independent integration `952e57a02ac2800e06ace3771bd23bde4c63dfb4`
(source `86b641a`, tests `596ab4f`) ran the focused selector with outer 360s
budget and fail-fast. Actual **exit 1**, package **3.857s**:
**2 top-level PASS / 1 FAIL**; all subsequent tests **NOT_RUN**.
Normal PG18 readiness, scope/clock/witness, and initial authority/fence tests
passed. The old-role test stopped at its clean Meta worker baseline, before
its synthetic observer grant: the generic fixture login permits `SET ROLE`,
but Meta admission requires `INHERIT TRUE, SET FALSE`. Root verified the
fixture and validator; only that fixture membership may be corrected.

Log: `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926/output/mrr-focused-952e57a.log`.
SHA-256: `37364c9028898ddd1b921179e6134ec30648f1b5f0e721d124a89bcf365cc94d`.
Correction to the previous setup summary: in `804065e`, the buyer subcase had
`42501`, while Meta already failed this clean-baseline check; they were not
both schema-name-resolution failures. Raw failed logs remain unchanged.
The third run's isolated fixture was cleaned. Real 90s, the added RLS
counterexamples, root PG acceptance and full regression are still unaccepted.

## Fourth and fifth focused receipts; local integration only

Fourth fixed test `419feb817f6082114dae23bc56fba06ae1034564` ran against
unchanged source `86b641a`: actual **exit 1**, **18.563s**, **13 top-level PASS /
1 FAIL**. The internal-child release, readiness, EOF and bounded reap completed,
but an extra assertion incorrectly attributed one legal native-worker provider
request to the recovery observer. Root authorized removal only of that invalid
aggregate zero-I/O assertion. The configured endpoint counter remains diagnostic;
observer zero Start/Stop and the EOF timing/reap assertions remain enforced.
Log in the test worktree: `output/mrr-focused-419feb8.log`, SHA-256
`824cd185bd75f1ff88a155a0e76bbbdcd1e1ba3d9a638421eea8150e94d893a5`.

Fifth fixed test `f3b731bcfba4d91ae373a9f0af2ac4b550529aa9` against the same
source ran `bash scripts/dev/test-local.sh --live-media-recovery`: actual
**exit 0**, foundation **151.064s**, **17 top-level PASS / 0 FAIL / 0 SKIP**.
Root read the complete 55-line log and independently verified its SHA-256:
`3d40495d8dd80d90f1dc8ba27fae3b98cb342c7fb1574b0c7679e1956abedf9c` at
`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926/output/mrr-focused-f3b731b.log`.
Real escaped-Start/restart ROOM test took **35.87s**, known-ID QUERY **4.42s**,
and actual parent-alive 90-second miss **91.59s**. Original River attempt was
**1 to 1**; no retry-policy or original LMR05 threshold was changed.
The test worker reported fixture cleanup; root verified the fourth-run cleanup
directly. Failed-run logs are retained, not superseded by the pass.

| Gate | Latest limited evidence | Remaining acceptance |
| --- | --- | --- |
| MRR01 | Independent real-crash ROOM and known-ID QUERY passed, plus root four-package race | Root integrated PG rerun |
| MRR02 | Scope/RLS/capacity/order SQL gates and actual 90s provider-fault miss passed | Native capacity coexistence, transient DB/delayed capture and complete timing-path coverage audit |
| MRR03 | Authority, fences, old-role, physical-DB, cleanup, INPUT and wrong-job checks passed | Explicit missing-original-job case and root rerun |
| MRR04 | Migration/readiness, EOF/reap and independent focused passed | Complete enabled/disabled startup modes, root rerun and fixed-tree full regression |

Root integrated the reviewed source and independent tests locally at `74459ae`;
this is not deployment or complete MRR acceptance. Original LMR05 red evidence
and Studio native NOT_RUN gates remain separate obligations. External human
alert delivery, real provider operation and customer deployment remain NOT_RUN.

## Root fixed-tree focused readback

Root independently ran `bash scripts/dev/test-local.sh --live-media-recovery`
on clean fixed main `048eca1e7b427edb7cb503c490f87575be69458e`: actual **exit 0**,
foundation **153.078s**, **17 top-level PASS / 0 FAIL / 0 SKIP**.
Positive ROOM took **35.85s**, known-ID QUERY **4.28s**, actual deadline miss
**91.74s**; original River attempt remained **1 to 1**.
Log: `/Volumes/data/output/mrr-focused-048eca1-root-20260927.log`, SHA-256
`07b9583bb8fb205b3d06dc2dd940381430c13892274e93203d948fb66f9f5bce`.
Tracked tree stayed clean throughout. Root verified no owned foundation
fixture remained; the protected upgrade fixture was untouched. Fixture window
lock was released after process exit and cleanup readback.

Contract audit still requires the explicit mode matrix, real DB delay/recovery,
capacity-degraded native cleanup, crossing-deadline witness/readback paths,
missing-original-job control, and fixed-tree full regression. These are not
replaced by this focused pass. Correction to the independent evidence table:
`integration.operations.job_id` has **no retention FK** (`0008`, lines 43–44);
its claim that a physically absent River job cannot be constructed is wrong.
A wrong job argument is distinct from an absent original row. An isolated
exact-job deletion negative is authorized; no schema constraint weakening is
needed or allowed. The test author will correct that evidence table with the
new fixed test batch. Production remains unchanged.

## Supplemented focused gate and full-run freeze

Fixed independent `cf4720ece7b6688400bde61bb6c15d1a272b7817` against unchanged
source `86b641a` ran the focused selector with **exit 0**, foundation **344.101s**,
**21 top-level PASS / 0 FAIL / 0 SKIP**. Root verified all result lines and hash.
Log: `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926/output/mrr-focused-cf4720e.log`;
SHA-256: `1a52ca252a554bfd35f42c3d60723d095bf2b0836cabec32b6e299fe4db7a711`.

The new checks cover mode/configuration branches, physically missing original
job, capacity overflow with continued native cleanup, timely readback with
post-90s Witness persistence (**92.50s**), and first recovery-role DB access
returning after the deadline (**94.60s**). The last case changes only the owned
fixture login's LOGIN flag; it does **not** claim a complete PG-server outage.
The existing real miss (**91.59s**) and ROOM/QUERY positives also passed. Static
test review `5bf6bf06-d0e4-4fc7-930e-2d17e73f65c2` found no blocking P0/P1.

Root additionally committed `9b44275`: a narrow ROOM/QUERY first-late-readback
classifier test at 90001ms, without prior proof. Targeted Go race **exit 0**,
**1.683s**; log `/Volumes/data/output/mrr-first-late-classifier-root-20260927.log`,
SHA-256 `3f53a7b79400985e365db5c8c4fc04c98ef3fb39b8ff8747b935e099d484a705`.
Independent review `a83c0d0d-1bab-43f1-b488-3d2437f89783` accepted this limited
classification evidence, not a real-clock process test.

Coverage adjudication `9085f413-6f4f-42d0-8d78-1cada625a31a` maps MRR02 to
combined evidence: actual timely-readback/late-Witness process; SQL late record
rejection and timeout-first stickiness; first-late classifier rejection; and
the parent sampling elapsed only after its committed read returns. A separate
known-member fresh-read response deliberately delayed across 90s remains an
**unexecuted stress variant**, not a new literal contract requirement. Do not
describe it as tested, or replace any of the existing required checks with it.

The measured pre-MRR full foundation baseline was **715.044s**, and the new
MRR focused suite alone takes **344.101s**. Root therefore permits the full Go
suite's outer execution envelope to increase **900s to 1500s** before freezing
the run. The focused envelope is 540s. No 90s recovery, 35s LMR05, lease,
process-exit or assertion deadline is changed; no River policy is changed.
One fixed-tree full run will also independently rerun the supplemented MRR
checks, avoiding an unnecessary extra 344s focused run. Root full acceptance,
original LMR05 and Studio native acceptance remain pending until evidenced.
