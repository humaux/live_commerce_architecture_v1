# T08 LMR01–06: bounded MOCK Stop and resource cleanup

Status: **PASS_LOCAL_MOCK_ONLY** (2026-09-27). Product source at main
`2190654` is byte-identical to the independently reviewed author `fa3f72f`.
Root full source/test tree `ff57e32` passed 637 top-level tests, 0 FAIL / 0 SKIP,
with PG18, race and vet. This is not a release approval;
real Cloud, customer streams, studio UI and global G06 remain untested here.

## Scope and dependency chain

[LMR contract](../../contracts/live-media-stop-v1.md), frozen at `395b10d` with
the already-running legacy-worker clarification `de621f6`, extends the
[accepted local LME execution](2026-09-27-live-media-execution-acceptance.md).
There is no new queue, operation type, lease, SDK or dependency version.

`MediaPlanner.RequestStop` → current `live:manage` authorization and existing
command receipt → `live.request_media_stop` → the original MEDIA operation.
Before committed Start reservation it cancels scheduling, without claiming a
remote resource ended. Otherwise it records sticky cleanup intent.

The existing worker claims → loads the frozen target → makes a fresh exact-ID
Query → `live.record_media_cleanup_query` validates and commits a reservation
→ existing LKP `Client.Stop` → shared observation projection. Unknown commit
acknowledgement cannot permit wire I/O. No business transaction spans network
I/O. Two persisted reservations are the ceiling, including crashes before wire;
the second requires a new generation, ACTIVE evidence and at least five seconds.
The five-second freshness bound is the final database decision, not a remote
wire-time guarantee after COMMIT. Only coherent terminal proof closes liability.

## Source, ownership and review

- Source worker: `gpt-6-sol / medium`, base `395b10d`, branch
  `commerce/media-stop-20260927`, isolated worktree
  `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`.
  Author commits `1cc19fc`, `fe3be43`, `9e1053a`, `fa3f72f`; root equivalents
  `0c08d41`, `0326443`, `a34b851`, `2190654`. Allowed source paths were the new
  0037/post0008 migrations and the narrow live/media runtime Go files.
- Independent test worker: `gpt-6-sol / high`, same frozen base, branch
  `commerce/media-stop-tests-20260927`, isolated worktree
  `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`.
  Test-only changes own foundation fixtures and the local runner, not source.
  Their final test-only head is `b2d9416`; root integrated through `4216c5f`.
  Root then added the frozen-admission executable test in `4c797be` and fixed
  its two fixture errors in `ff57e32`. No product source changed after review.
- Independent read-only review: Humaux `b81cd543-bc67-4d91-8e27-bb10937a028f`,
  task `2dd904a9-e753-46eb-8e9a-67f82393cab0`. Actual reviewer model/effort was
  not exposed. Two confirmed SQL P1 findings were closed in fixed `fa3f72f`;
  no additional confirmed P0/P1 in the scoped Go review. Static review is not
  PG/TLS acceptance. Root integrates migrations and independently reruns gates.

## Repairs and retained evidence

1. Stop evidence creates a projection↔observation FK cycle. The new pointer FK
   is `DEFERRABLE INITIALLY IMMEDIATE`: normal writes still check immediately;
   an explicitly deferred owner cleanup transaction can remove both sides.
2. A nonterminal reply to the second Stop now records exhausted budget and its
   event immediately, rather than relying on another Query arriving later.
3. Readiness checks actual constraint definitions, not only names. Five PG18
   `pg_get_constraintdef` digests detect drift; they are not encryption. A PG
   upgrade or legitimate DDL change requires inspecting the semantic definitions
   and rerunning admission, authority and fault gates before updating digests.
4. Startup rejects the old executor allowlist, but an already-running old process
   would skip startup. Its old nonterminal cleanup Query path now fails ME409;
   valid terminal evidence is still accepted. Deployment must drain old workers;
   this is not an automatic upgrade or post-escalation recovery workflow.
5. The first independent actual-PG run failed fixture schema lookup, absent
   initial projection, and a four-second helper used against five-second pacing.
   Test-only repairs do not lower production thresholds. Original log retained:
   `/Volumes/data/output/lmr-independent-20260927-first.log` (125.426s, exit 1).
   `lmr-independent-20260927-focused2.log` skipped without DB consent: NOT_RUN,
   never counted as a database pass.
6. The second actual-PG run reached the immutable-update trigger before the
   intended CHECK-constraint negative. The test now disables that trigger only
   in its rollback-only owner transaction; normal writes still require ME409.
   `lmr-independent-20260927-second.log` (180.126s, exit 1) remains evidence.
7. The third run passed foundation tests (184.037s) but its Bash wrapper exited
   127 after the runner file was edited while executing. The stable script was
   syntax-checked and rerun without edits; the final gate below exited 0.
   `lmr-independent-20260927-third.log` is not a complete gate pass. Never edit
   an active acceptance runner. The full-suite aggregate timeout increased from
   600s to 900s for the additional cases; individual SQL, lease, freshness and
   fault bounds are unchanged. The specialty runner remains bounded to 300s.

## Gate evidence

- Author actual local admission: `--live-media-plan` at `fa3f72f`, exit 0,
  `/Volumes/data/output/lmr-author-20260927-readiness-repair.log`. Not an LMR gate.
- Independent `bash scripts/dev/test-local.sh --live-media-stop` at test-only
  `b2d9416` plus source `fa3f72f`: exit 0, 41 top-level PASS / 0 FAIL / 0 SKIP;
  foundation race 183.779s. Log
  `/Volumes/data/output/lmr-independent-20260927-final.log`, SHA256
  `ca1e6653479c34c365ce7f2eba6413946baf1392f6030df3e7e49152324c4174`.
  Independent vet of live/platform/foundation also exited 0. This run covers
  the old SQL admission predicate, not the subsequently added old executable.
- Initial root full `bash scripts/dev/test-local.sh` at `c2ff2b4`: deliberately
  interrupted after an additional test-only runtime-role correction was found
  in `c739fb2`. Log `/Volumes/data/output/lmr-root-full-20260927.log` retained
  (foundation terminated at 210.766s, exit 1); this is not a pass or a product
  failure verdict. Only the owned test process was stopped; its labeled local
  PostgreSQL fixture `lc-foundation-test-40891` was removed by the runner trap.
- The second root full run at `4c797be` was also deliberately interrupted:
  independent review found two errors in the new legacy executable test fixture
  (login name passed as a DSN, and inherited role allowing SET ROLE). Fix
  `ff57e32` uses the actual pool connection string and the existing accepted
  LME membership setup. Log `lmr-root-full-final-20260927.log` is retained and
  is NOT a pass. Owned PostgreSQL fixture `lc-foundation-test-42511` was removed.
- Independent supplemental frozen legacy admission executable at root test commits
  `4c797be` + `ff57e32`: exit 0, one top-level PASS, 0 FAIL / 0 SKIP; test 6.68s,
  package 8.866s. Log `/Volumes/data/output/lmr-independent-20260927-legacy.log`,
  SHA256 `7db0b8deb882b030d6a6af37662d53e2cd6890a6d1c3d77e67b7c10162f64c19`.
  This is a separate run from the 41-test specialty gate above.
- Final root `bash scripts/dev/test-local.sh` at `ff57e32`: exit 0,
  **637 top-level PASS / 0 FAIL / 0 SKIP**, 32 tested packages; foundation race
  **616.716s**, followed by successful `go vet ./...`. Log
  `/Volumes/data/output/lmr-root-full-accepted-20260927.log`, SHA256
  `293bd6666ac36c6644776512b9713219047a76cfda75c01347409e51c6cba3d8`.
  This same final tree includes the specialty tests and the supplemental frozen
  legacy admission executable (3.65s in this run). No duplicate root focused
  run was needed. Browser-tagged and real-provider tests are not part of this
  backend-only full run. The runner removed its owned PostgreSQL fixture.

The supplemental legacy test compiles the actual platform/HTTP-error package
sources and module locks from frozen Git `395b10d`, then runs admission before
and after schema upgrade, with a current-code positive on the same connection.
It is an admission executable, not a complete old streaming deployment. Test
checkouts must retain that Git object; missing history fails rather than skips.
It does not copy a SQL predicate or change production code.

## Release boundary

All database and TLS fixtures are local and task-owned. No production data,
credentials, provider configuration, billing resource or customer broadcast was
changed. Real Cloud duplicate Stop/fault behavior, LIVE authorization,
media-worker executable configuration, HTTP/BFF, studio/browser, deployment,
post-escalation operator resolution and full T08/G06 remain outside this gate.
