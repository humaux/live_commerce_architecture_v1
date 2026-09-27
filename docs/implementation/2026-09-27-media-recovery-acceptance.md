# MRR90 restart state-check acceptance

Status at 2026-09-27 10:27 UTC: **CANDIDATE_BLOCKED / NOT_ACCEPTED**.
The independently frozen design is on `598eea4`. Candidate implementation and
independent tests are not merged into main. No production configuration,
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
