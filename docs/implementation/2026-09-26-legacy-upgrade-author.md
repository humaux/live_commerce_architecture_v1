# LRI04 historical populated upgrade author evidence

Status: **CANDIDATE / LOCAL ONLY / ROOT REPLAY PENDING**. No product, SQL,
script, dependency, provider, or production change is part of this checkpoint.

- Role `test_worker`; actual `gpt-6-sol/high`; base `237b880`; branch
  `commerce/legacy-upgrade-tests-20260926`; worktree
  `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`.
- Write paths: `tests/foundation/legacy_runtime_upgrade_test.go` and this note.
  Historical helper from independent author `3757320`, integrated in base,
  remains byte-unchanged. The tests use only task-owned loopback PG18 fixtures.
- Frozen gate: `contracts/legacy-runtime-isolation-v1.md` LRI04. Old producer
  fixtures explicitly use the pre-0032 `river` schema. After the cutover,
  production constructors select the fixed payment/expiry family schemas.

## Executed proof

`bash scripts/dev/test-local.sh --legacy-isolation` ran actual isolated PG18
with `go test -race` and selected seven `TestLegacyRuntimeIsolation*` cases.
Final author run: **exit 0, 7 PASS / 0 FAIL / 0 SKIP**, foundation 22.289s.
Log `/Volumes/data/output/legacy-upgrade-author-candidate7-20260926.log`,
SHA256 `945c9f57acec4512a322f0651e83e96c1f1e9bd74e5f248d77c6ff22db78c3bb`.
`GOTOOLCHAIN=go1.27.1 go vet ./tests/foundation` exited 0;
`git diff --check` exited 0.

The populated case admits real old-schema payment and expiry jobs, all three
payment execution profiles, a linked reconciliation observation, and external
and Meta jobs in their respective old/current lanes. It asserts all seven
retained non-running states, terminal default queues, paused queue rows, old
checksum/full-job/full-queue/business-row equality, unchanged old sequence,
strict new sequence high-water, repeated Apply equality, and actual migrated
expiry consumption. A separate sequence case makes a genuine pruned expiry
reference greater than the old source sequence while the native payment
destination sequence is highest; both next IDs must still advance.

Negative cases exercise current `migrations.Apply`: tampered checksum and
unknown version reject before preparation, while running/poison source,
nonempty destination, conflicting destination queue, and an observed actual
PostgreSQL lock wait reject the post phase without a checksum or partial row
move. Corrected retry after committed native ledgers succeeds. Explicit equal
numeric IDs in payment and expiry cannot authorize the wrong reconciliation
job or write an observation.

## Preserved failures and scope

Earlier author iterations are retained, not relabelled as product failures:
candidate1 exit 1 (test fixture exhausted PG connections), candidate2 exit 1
(test fixture violated River terminal-state constraint), candidate4 exit 130
(interrupted after a test lock probe failed, to release its owned fixture),
candidate5 exit 1 (the lock probe timed out before observing the lock under
shared-pool contention). Candidate3 and candidate6 passed earlier narrower
versions; candidate7 is the final exact-source run. Logs remain under
`/Volumes/data/output/legacy-upgrade-author-candidate{1..7}-20260926.log`.

Root's independent replay, final security review, full regression, affected
browser gate, provider qualifications, and production cutover are **NOT_RUN**
by this author. This note does not accept the overall LRI unit or SaaS release.
