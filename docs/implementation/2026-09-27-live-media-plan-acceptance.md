# T08 LMP01–07: durable MOCK media start intent

Status: **PASS_INDEPENDENT_LOCAL_PG_AND_ROOT_FULL_REGRESSION** at main `0d51b9e`.
This increment persists a start intent; it does not execute a stream. Real
provider calls, LIVE authority, controller/worker execution, studio browser,
Cloud/media quality, G06 and deployment remain **NOT_RUN**. Customer services,
streams, credentials and production data were not changed.

## Implemented boundary and dependencies

[Frozen contract](../../contracts/live-media-plan-v1.md): `625a6b6`, DB-clock
addendum `b120ec0`. `MediaPlanner.PlanStart` reuses existing authorization,
`command.Run`, PostgreSQL READ COMMITTED and River `InsertTx`. Within one caller
transaction it records an immutable scoped attempt, a typed MEDIA_ATTEMPT
operation/event, a native `river_media` job and a frozen READY command receipt.
An exact receipt replay does not re-plan or overwrite the original principal.
Current caller authority is rechecked even on replay and after lock waits.

Forward migration0035 and post-River migration0006 provide the composite identity
constraints, dedicated private writer, operation/event isolation, immutable draft
and attempt guards, native queue linkage and readiness fence. Existing payment,
expiry and Meta lanes cannot accept the reserved media family. The legacy worker
cannot read/claim/complete a MEDIA_ATTEMPT operation. No HTTP start route, worker
state transitions, provider call or lease-bound secret resolver is enabled.

No dependency was added: existing pgx/River, PostgreSQL constraints/locks and Go
standard-library hashing/JSON cover the slice (ponytail reuse). The durable lane
is deliberately inert: only immediate `available`, attempt0, unfinalized jobs
are admitted. Execution states require a separately frozen worker contract.

## Ownership and commit manifest

|Role|Actual model/effort|Base and exclusive writes|
|---|---|---|
|Source integration worker|gpt-6-sol / medium|`625a6b6`; migration0035, post-River0006, migrations/migrate.go, internal/live/media_plan.go and its unit test|
|Independent test worker|gpt-6-sol / high|`625a6b6`; tests/foundation/live_media_plan_test.go and comment-only clarification in live_media_authorization_test.go|
|Independent reviewer|Inherited model/effort not exposed|Read-only contract, source repairs and final test/evidence review|
|Root integrator|Inherited root session|Contract, runner flag, dependency/acceptance records, main integration and full regression|

Source worktree `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`, branch
`commerce/media-plan-20260927`, clean final source `01f8537`. Independent worktree
`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`, branch
`commerce/media-plan-tests-20260927`, clean final `30f2e39`. No recursive agents.

- Source `ad29f14`, `a1194fc`, `01f8537` integrated as `ecd23c3`, `06ddf37`,
  `34a91a7`.
- Tests `93d68f0`, `a262f05`, `0602a43`, `4693b9b`, `077d9d5`, `30f2e39`
  integrated as `f125556`, `e5c477b`, `7c1d469`, `047d732`, `53ceccd`, `4f323f0`.
- Existing merchant-orders test-harness correction `e1a6176` and its separate
  [browser evidence](2026-09-27-merchant-orders-ui-acceptance.md) are not counted
  as a full browser pass by this backend gate.

## Gate coverage and retained repair evidence

|Gate|Independent causal checks|
|---|---|
|LMP01|Exact receipt/attempt/principal/operation/event/native job and same-transaction direct-SQL positive control|
|LMP02|Exact replay, changed input conflict, one-session admission and concurrent deduplication|
|LMP03|Invalid scope, versions, layout, binding, authority and deadline reject with zero partial facts|
|LMP04|Actual runtime/worker roles, secret and MEDIA isolation, frozen draft and legacy positive controls|
|LMP05|Wrong native schema, family, args, state and linkage fail atomically; DB-clock immediate scheduling|
|LMP06|Observed real binding lock waits, grant/revision revoke via direct SQL without Go final authorization; final-event wait then DB deadline expiry|
|LMP07|Populated forward upgrade, exact native job/readiness fences and legacy operation compatibility|

The first review/run found three source problems: the private writer lacked
schema USAGE for readiness lookup of the four legacy River schemas; the typed
MEDIA actor permitted a null original principal; and a linked native job could
already be terminal. Repair1 grants schema lookup only (not legacy table read),
requires the original attempt principal, and guards initial native job state.

Repair1's strict schedule comparison then rejected legitimate native inserts.
River's pgx driver supplies `time.Now().UTC()` when ScheduledAt is omitted. A
rollback-only diagnostic captured the **same rejected row at its actual guard**:
all other conditions were true, with `scheduled_delta_us=143.000000`. An earlier
after-round-trip clock comparison was inconclusive and is not used as proof.
Repair2 normalizes available INSERT scheduling to `clock_timestamp()` inside
the existing BEFORE INSERT trigger. It adds no arbitrary skew window and does
not allow scheduled/pending/terminal jobs. Explicit future available jobs become
immediate; UPDATE still cannot change schedule/state/identity through producer
privileges. Diagnostic SQL and temporary tests were removed after diagnosis.

A proposed deferred prerequisite on Go command-receipt existence was rejected:
it would silently break the separately frozen direct-SQL planner. It is absent
from accepted source. Fixture-only fixes corrected SQL placeholder/UUID/enum
types, cleanup on fatal paths, early waiter-error handling and Go return-value
evaluation before a callback populates the result. No product assertion or
success threshold was removed or weakened.

All following logs are under `/Volumes/data/output/`:

|Executor/command|Observed result|Log|
|---|---|---|
|Author `test-local.sh --live-authority`|0; prior LSP/LMA and migrations smoke only|`lmp-author-20260927-authority5.log`|
|Author `go test -race ./internal/live`; vet|0 / 0|`lmp-author-20260927-final-live-race.log`, `lmp-author-20260927-final-live-vet.log`|
|Independent `test-local.sh --live-media-plan`, original/repair1|Retained failures, not passes|`lmp-independent-20260927-first.log`, `lmp-independent-20260927-repair1.log`|
|Rollback-only exact-row schedule diagnostic|+143µs at the rejecting guard|`lmp-independent-20260927-guard-diagnostic.log`|
|Independent repair2 fixture iterations|Retained interrupted/red evidence|`lmp-independent-20260927-repair2.log`, `lmp-independent-20260927-repair2-fixedfixture.log`|
|Independent final `bash scripts/dev/test-local.sh --live-media-plan`|0; 17 top-level PASS, 0 FAIL/SKIP; foundation20.976s, race|`lmp-independent-20260927-final.log`|
|Independent `GOTOOLCHAIN=go1.27.1 go vet ./tests/foundation`|0; empty stdout|`lmp-independent-20260927-vet.log`|
|Root `bash scripts/dev/test-local.sh` at `4f323f0`|1; 611 top-level PASS, 1 FAIL, 0 SKIP; foundation448.865s; vet not reached|`live-media-plan-root-full-20260927.log`|
|Root mistyped selector `--legacy-runtime`|2; usage only, no fixture started|`lmp-legacy-upgrade-root-repair1-20260927.log`|
|Root corrected `--legacy-isolation` at `0d51b9e`|0; 10 top-level PASS, 0 FAIL/SKIP; foundation31.834s|`lmp-legacy-upgrade-root-corrected-20260927.log`|
|Root full rerun at `0d51b9e`|0; 612 top-level PASS, 0 FAIL/SKIP in 32 test packages; foundation451.649s; full race and vet pass|`live-media-plan-root-full-repair1-20260927.log`|

Focused count includes seven LMP, five LMA and five LSP top-level tests. Native
browser-tagged tests and provider qualification are not included in that count.

SHA-256: final independent log
`4cbb423c55bd8b0ce356b7aab13854cb3d643480798782bb440b45fe3ee92ed3`;
exact-row diagnostic
`d74b6c0c8557c45520e2d5f16b5eef35417adfa092ac067a2ffc16590a9c28e5`.

Root's first full run caught one historical-test shape drift:
`TestLegacyRuntimeIsolationPopulatedUpgrade` snapshots whole pre0032 rows, but
the latest Apply now adds nullable `media_attempt_id` in0035. Test-only `0d51b9e`
adds precisely that null field to every old expected operation row and retains
full-row/full-column JSONB equality with identical sorting. No column is ignored;
old values/counts and null media links remain mandatory. Source and migrations
are unchanged. Independent review confirmed the 11-line change does not weaken
the gate, and the entire ten-test legacy subset passed. First root log hash:
`902bed5677d484c85c9a11aabcada35f57e0cfa945f3f6bff46ac3e5a54769eb`;
corrected legacy log:
`1d546de10db632b97ef929332dd7f300b02bef6707329a77257a05a4a71fd1e5`.

Final root full log SHA-256:
`a3b9c018b1a10a374cf9f04e72edd88ed276a8a563774c50dadffcb88dd984cd`.
Root terminal session68403 returned exit0; the runner's final PASS follows
`go vet ./...`. Its task-labelled `lc-foundation-test-26797` container is absent
after exit. No existing customer or development service was stopped.

## Handoff and non-claims

Source receipt `5b6982e5-d152-4e4d-90a9-c737cdb5e9ca` predates independent green;
newer independent receipt `a255a234-ebfb-4076-8db5-d601226f3c84` records that gate.
Final focused independent review: `88ea1e9c-02fe-451f-b4e8-3d0e1de6a701`;
root's test-only additive-column repair review:
`9b8187f7-0cbe-4a25-945f-4c40858cf3af`. Both bounded reviews found no confirmed
P0/P1; neither substitutes for a successful root rerun. Failed logs and worktrees
are retained; runners clean only their task-labelled fixtures, never another
task's container/service. Root rerun subsequently passed as recorded above.

The next increment is the typed dedicated media worker: lease/observation and
secret-resolution authority, conservative Start-response-loss recovery, bounded
same-ID Stop accounting and reconciliation. Current persisted READY is not a
running stream. Full T08 remains IN_PROGRESS and G06 remains NOT_RUN.
