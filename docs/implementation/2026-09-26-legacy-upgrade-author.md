# LRI04 historical populated upgrade author evidence

Status: **REPAIRED CANDIDATE / LOCAL AND ROOT REPLAY PASS / FINAL REVIEW PENDING**. No product,
SQL, script, dependency, provider, or production change is part of this author
checkpoint.

- Role `test_worker`; actual `gpt-6-sol/high`; original base `237b880`,
  directed repair against root `1711ce6` and the four frozen follow-up test
  helper commits `b899650`, `667376e`, `e51e76c`, `b275d78`; branch
  `commerce/legacy-upgrade-tests-20260926`; worktree
  `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`.
- Write paths: `tests/foundation/legacy_runtime_upgrade_test.go` and this note.
  Historical helper from independent author `3757320` remains byte-unchanged.
  Cherry-picked helper/admission commits touch other paths; the author edits
  in this repair remain confined to the two paths above. The tests use only
  task-owned loopback PG18 fixtures.
- Frozen gate: `contracts/legacy-runtime-isolation-v1.md` LRI04. Old producer
  fixtures explicitly use the pre-0032 `river` schema. After the cutover,
  production constructors select the fixed payment/expiry family schemas.

## Executed proof

`bash scripts/dev/test-local.sh --legacy-isolation` ran actual isolated PG18
with `go test -race`. On the integrated helper tree, exact final HEAD author
run was **exit 0, 10 top-level PASS / 0 FAIL / 0 SKIP** (plus 26 LRI03
subtests), foundation 31.498s. Log
`/Volumes/data/output/legacy-upgrade-author-final-20260926.log`, SHA256
`777e431858a3b1480d2d442bd7bbfd552a5eb8461edc856452957f2c40098ed7`.
The immediately preceding integrated-helper run also exited 0 with 10
top-level PASS, foundation 31.011s; its preserved log is
`/Volumes/data/output/legacy-upgrade-author-integrated1-20260926.log`, SHA256
`7e19bfa8f4833e55f5aca2b603402070d2f7478de8bb490136459b65f045587c`.
`GOTOOLCHAIN=go1.27.1 go vet ./tests/foundation` exited 0;
`git diff --check` exited 0.

Root separately reran the integrated candidate at `0cc4e88`: actual selector
**exit 0, 10 top-level PASS / 0 FAIL / 0 SKIP**, foundation 27.848s. The
root-owned log `/Volumes/data/output/legacy-isolation-root-upgrade2-20260926.log`
has verified SHA256
`02fd89dbdb73992a611f27d52d9cd54c4661ea8ece7070d6212f92c9bcbbabfe`.
This is independent execution, not final source/security signoff.

The populated case admits real old-schema payment and expiry jobs. MOCK,
SANDBOX and LIVE use actual pre-0032 StartPayment/InsertTx with explicit
`river`, with internally consistent account, method, qualification,
environment, binding and queue snapshots. SANDBOX/LIVE qualifications are
owner-seeded **synthetic local proof**, never provider verification. It also
includes a linked reconciliation observation and external
and Meta jobs in their respective old/current lanes. It asserts all seven
retained non-running states including an explicit `available` row, terminal
default queues, paused queue rows, old
checksum/full-job/full-queue/business-row equality, unchanged old sequence,
strict new sequence high-water, repeated Apply equality, and actual migrated
expiry consumption. Separate sequence cases cover both mirrored pruned
permanent-reference/destination-sequence maxima, plus genuine copied rows
above the old source sequence; old source sequence is never rewritten.

Negative cases exercise current `migrations.Apply`: tampered checksum and
unknown version reject before preparation, while running/poison source,
nonempty destination, conflicting destination queue, and an observed actual
PostgreSQL lock wait reject the post phase without a checksum or partial row
move. Corrected retry after committed native ledgers succeeds. A **valid**
payment reconciliation ID collides with a real expiry ID and its report commits;
another expiry-only ID cannot authorize a reconciliation or write an
observation.

## Preserved failures and scope

Earlier author iterations are retained, not relabelled as product failures:
candidate1 exit 1 (test fixture exhausted PG connections), candidate2 exit 1
(test fixture violated River terminal-state constraint), candidate4 exit 130
(interrupted after a test lock probe failed, to release its owned fixture),
candidate5 exit 1 (the lock probe timed out before observing the lock under
shared-pool contention). Candidate3, 6 and 7 passed earlier narrower source
versions. Root's first `1711ce6` replay **exited 1**, 4 PASS / 4 FAIL,
because six old-schema calls did not explicitly select `river` after the
independent helper change; log
`/Volumes/data/output/legacy-isolation-root-upgrade1-20260926.log`, SHA256
`f25fc8b94345ac114d5118abae8b6cb584b908ed24d083a3d97cad8dba6a4e05`.
Repair1 exited 1 on the new explicit state-set assertion (`available` absent);
its log `/Volumes/data/output/legacy-upgrade-author-repair1-20260926.log`
has SHA256 `ac021f88342e01b629ee1f8bcd4f33478f77ecfa0912fbd59b61f80356f9ebe5`.
Repair2 exited 0 on the then-current helper tree, before four additional test
helper commits. Earlier logs remain preserved under
`/Volumes/data/output/legacy-upgrade-author-candidate{1..7}-20260926.log`.

Final source/security review, full regression, provider qualifications, and
production cutover are **NOT_RUN** by this author. Prior affected browser gates
are root-owned evidence, not claims of this author. This note does not accept
the overall LRI unit or SaaS release.
