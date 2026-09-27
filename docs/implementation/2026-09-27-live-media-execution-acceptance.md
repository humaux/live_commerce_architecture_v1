# T08 LME01–08: isolated MOCK media execution and recovery

Status: **PASS_INDEPENDENT_LOCAL_PG_AND_ROOT_FULL_REGRESSION** at code/test tree
`4345732`. Fixed source, final native-maintenance coverage and the root full
PG18/race/vet run passed. This is bounded local MOCK acceptance, not production
release approval. No customer service, real stream,
production credential or external provider was changed.
Root subsequently added a same-key-ID decryption-failure subcase at `f5db182`;
the 28-test focused rerun and foundation vet passed. Product source is unchanged
from `4345732`. The whole-suite run is at `4345732`, not a second whole-suite run
at the test-only `f5db182`; the latter's complete LME/LMP/LMA/LSP subset was rerun.

## Boundary and dependency chain

[LME contract](../../contracts/live-media-execution-v1.md), frozen at `1fbae8a`
with the partial-timestamp/lifetime clarification `e96c75f`, builds on the
[accepted LMP intent](2026-09-27-live-media-plan-acceptance.md). Existing
`MediaPlanner.PlanStart` and River `InsertTx` still own intent creation. The
new `live.NewMediaClient` consumes only `river_media` / `media_mock_v1`.

Separate native-worker and five-function executor pools must pass exact-role,
direct-grant, SET-reachability and same-physical-database checks. SQL owns the
operation lease/generation and immutable attempt identity. Dispatch is claim →
sealed load → existing LKM decrypt → committed one-time reservation → existing
LKP Start → durable observation. An uncertain reservation commit never permits
Start. Later work can only discover the exact room or query the pinned egress ID.
No database transaction crosses provider I/O.

Migration0036 adds the scoped execution projection and append-only observations;
post-River0007 grants the dedicated native lifecycle without opening merchant
UPDATE/DELETE or other River lanes. Startup checks exact function ACLs and guards.
Revocation denies new dispatch but does not discard already-owned external facts.
Unresolved results stay UNKNOWN, including bounded age/generation escalation.
Terminal resource state requires correlated positive end evidence, not an ACK.

No dependency, queue framework, SDK or second lease ledger was introduced.
Existing pgx, River, LKM/LKP clients, PostgreSQL constraints and Go standard
library cover this increment. Upgrade-sensitive call paths are maintained in
[dependencies](dependencies.md).

## Ownership and integration

|Role|Actual model / effort|Base and exclusive writes|
|---|---|---|
|Source integration worker|gpt-6-sol / medium|`1fbae8a`; migration0036, post0007, media_execution.go, media_runtime.go, narrow platform.go guards|
|Independent test worker|gpt-6-sol / high|`1fbae8a`; foundation live_media_execution*_test.go only; initial tests authored before source was supplied|
|Independent reviewer|Inherited model/effort not exposed|Read-only fixed SQL and final ACL/extra-EXEC changes|
|Root integrator|Inherited root session|Frozen contract, runner flag, integration, evidence/docs and independent regression|

Source author commits `8583f25`, `7045f9`, `e9abb99`, `9cb9ef2` map to main
`efb709a`, `afa07c9`, `52df43a`, `658cede`. Initial independent test commits
`10242a8`, `e6074c9`, `4e499a8` map to `ef9a16f`, `89b4036`, `4f94e0c`.
The final native-maintenance test commit `bf517dd` maps to main `4345732`.
Root test-only `f5db182` adds a previously unexercised GCM failure path, without
mutating persisted envelopes, sharing new global fixture state or changing source.
Source/tests used separate existing worktrees and non-overlapping paths; root
alone integrated shared migrations and platform changes.

Humaux receipts: source `ab126f8f-2162-4b27-8f6d-c972e266fdf7`, final independent
PG gate `5dd18f7f-6320-43fc-9bff-28d339997f07`, bounded final review
`f31a3902-7391-4278-ba97-3fe4e6b38ae1`, root focused
`5b50c41a-3161-4fc8-809c-cc390be8ba25` (historical), final root
`77b48151-90b8-48a2-905f-9f1dc11530df`. None of these receipts asserts the full
SaaS or production is ready.

## Accepted local gate evidence

|Gate|Causal checks|
|---|---|
|LME01|Real minimal-role positives; direct grants, mixed/SET roles, extra identity issuer EXEC, wrong physical DB and secret redaction negatives|
|LME02|Native River → local TLS → exact Start material and persisted nonterminal facts; blocked provider plus pg_stat_activity and independent NOWAIT prove no executor transaction across I/O|
|LME03|Observed database lock waits followed by authority/binding/revoke/deadline changes; otherwise-valid stale/wrong token and repeated/expired reservation negatives|
|LME04|Accepted Start loses reply; actual child-process death and replacement recover room/pinned ID without another Start or reconciliation stream secrets|
|LME05|Concurrent native workers; implicit reserve COMMIT acknowledgement loss after CommandComplete and idle ReadyForQuery; pre-reserve, observation and finish-event rollback|
|LME06|Closed statuses, absent/partial timestamps, coherent and contradictory cross-report projection, duration, duplicates, identity collision, empty/ambiguous room discovery|
|LME07|Populated0035 forward upgrade, repeated Apply and preserved initial-job guards; exact ACL/guard poisoning; actual native scheduler, rescuer and terminal cleaner with eligible foreign-job snapshot unchanged|
|LME08|Generation/age unresolved escalation; no credential fallback and same-key-ID GCM failure makes zero wire/facts; reserved lifetime revocation versus start-deadline semantics; task-owned process/pool shutdown|

Evidence logs are under `/Volumes/data/output/` and are retained, including red
runs. The crash-child entrypoint returns immediately in the normal parent run;
the actual child execution/death is asserted by LME04, not by that helper's PASS.

|Run|Result|Log|
|---|---|---|
|Author migration/old planning smoke|Passed after UUID repair; not new LME acceptance|`lme-author-20260927-migration2.log`, `lme-author-20260927-plan7.log`|
|Independent initial|Compile-only failure retained|`lme-independent-20260927-first.log`|
|Independent first actual PG|Failed fixture role/upgrade setup; retained|`lme-independent-20260927-compilefix.log`, `lme-independent-20260927-diagnostic.log`|
|Independent second|Exit0; 9 functional LME tests plus child helper, 17 prior LSP/LMA/LMP tests; race; foundation67.889s|`lme-independent-20260927-second.log`|
|Independent maintenance precursor|Retained test-only queue-pause cleanup failure, not a pass|`lme-independent-20260927-maintenance.log`|
|Independent final at source `9cb9ef2`, tests `bf517dd`|Exit0; 10 functional LME tests plus child helper, 17 prior tests; race; foundation66.870s; foundation vet also exit0|`lme-independent-20260927-maintenance-fix.log`|
|Root focused at `4f94e0c`|Exit0; same 9 functional LME tests plus child helper and 17 prior tests; race; foundation64.556s|`live-media-execution-root-focused-20260927.log`|
|Root full at `4345732`|Exit0; 623 top-level PASS, 0 FAIL/SKIP, 32 packages; foundation483.549s; full race and vet|`live-media-execution-root-full-20260927.log`|
|Root decryption coverage at `f5db182`|Exit0; 28 top-level PASS, 0 FAIL/SKIP; foundation69.704s, race; foundation vet also exit0|`live-media-execution-root-material-final-20260927.log`|

Independent second log SHA-256:
`0d52725a61a1f67ca19cc851c7d9bf5bc142e0dac1a5596a61912f4e4fe30dae`.
Root focused log SHA-256:
`fd978b05348ae864d2d7470bd2ab5efaaaf6613efc7396a99974dd6f90409721`.
Final independent log SHA-256:
`bdcbf5f5508632e0af1b31a8deb303f3b44b26391624e96c3c71267dac03e00e`.
Final root full log SHA-256:
`cbcf398fd6213b35380fdb7f47dd093e2017c0289ac3b195c4a8f2ca4f24c4c3`.
Final root decryption-coverage log SHA-256:
`c539e1d26e47596f553bef7bd76e3bcb7ab87f8c07c12f8a0833e3b58814bdc0`;
session1436 completed exit0 after the appended foundation vet command.
Root session78541 returned exit0 at 2026-09-27T00:33:40Z. The runner's final PASS
follows `go vet ./...`; the task-labelled `lc-foundation-test-34529` container
was absent in `docker ps -a` after exit. Owned TLS servers, pools and child
processes are stopped by the test fixtures; no other service was stopped.
The 623 count includes test helper entrypoints; it is a whole-Go-suite count,
not 623 new LME scenarios. Browser-tagged and LIVE gates were not included.

## Repairs and non-claims

The first planning smoke rejected legitimate UUIDs because the new post-River
guard had an extra four-character group. It was corrected to the UUID shape;
the initial-state/linkage guards were not removed. Independent SQL review also
required exact function ACLs, complete reserved-resource revocation reasons and
coherent partial timestamp merging. The final bounded `9cb9ef2` review closed
the prior ACL P1 without finding a new confirmed P0/P1 in those two diffs.

Root additionally found that checking seven known live function names did not
exclude a direct grant of `identity.issue_merchant_session`. The fix checks
executable cross-domain authority, and independent positive → poison → reject →
revoke → positive tests exercise the real login without minting a session.

The independent startup failure was a fixture defect: the reused role helper's
`IN ROLE` granted SET TRUE. Minimal native/executor fixtures now explicitly use
INHERIT TRUE, SET FALSE; the source guard remains strict. The old-upgrade fixture
also needed all historical River schemas before applying their post migrations.
The implicit-commit fault injector was adapted to the actual pgx protocol; no
production BEGIN was added merely to fit a test.
The maintenance fixture pauses only its synthetic native media queue while
testing scheduler/rescuer/cleaner behavior; it now restores the exact previous
pause state during cleanup, after stopping its client. The precursor failure
was not repaired by disabling later tests or unpausing a customer queue.

Stop dispatch/recovery, resource reclamation, LIVE authorization, Cloud/media
quality, user-facing studio/start routes, deployable media-worker configuration,
provider qualification and global G06 remain **NOT_RUN / NOT_IMPLEMENTED**.
An accepted MOCK Start/observation slice is not a working production broadcast
system. Broader T08 and the SaaS remain in progress.
