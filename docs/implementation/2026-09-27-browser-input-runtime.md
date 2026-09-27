# Browser input custody implementation record — 2026-09-27

Status: **BIC_INDEPENDENT_PG_PASS; integrated root acceptance running**.
Source base `2b52e7d`; frozen contract:
[BIC/BRW](../../contracts/live-browser-input-runtime-v1.md).
Contract freeze `4809128`, cleanup-authority clarification `234f44a` (reviewed,
no ABI change): merchant revocation denies new grants but cannot strand the
original executor's cleanup responsibility.

## Root cause and boundary

The current original media job completes when Egress is terminal, before it can
retain browser publisher cleanup. The same loss exists in Stop-before-wire,
policy denial, uncertain finish and age/generation exhaustion. Extending only
the happy-path worker would leave those sibling paths unsafe.

Reuse the original operation/job and private MLC custody. Add one input child
and a fresh local-only profile/queue that an old worker cannot consume. Do not
relax historical MOCK profile or reinterpret R04 probe success as product/Cloud
acceptance. The first unit persists/fences liability and jointly controls
completion; token delivery and new-queue worker remain disabled until the next
unit's independent acceptance.

The private profile is deliberately limited to LOCAL_SFU_MOCK_EGRESS. This is
not a generic provider/profile framework. Local reserved input cannot be marked
strict CLOSED because cached/refreshed token revocation is not qualified here.
Held UNKNOWN is visible unresolved work, not success or automatic retry forever.

## Ownership and planned evidence

Read-only source preflight: `/root/media_runtime_source`, main `2b52e7d`.
Read-only browser/test preflight: `/root/media_runtime_tests`, same base.
Independent reviewer: `/root/livekit_protocol_impl`, no writes; final bounded
review found no remaining confirmed P0/P1. Corrections: legacy planner refuses
profile-attached authority, exact native queue guards, full current-admission
check after cached command replay, and finite attempt-anchored input lifetime.
Runtime model/effort of these reused agents is not exposed; no override claimed.
No recursive delegation. Source/test writing begins only after interface freeze,
in the existing separate worktrees; at most two concurrent writers.

Root owns final migration merge (`0040` and post-River `0009`), contract and
task metadata. Independent tests use existing actual PG18/race runner and
`TestLiveMediaExecutionBIC` prefix, not another harness. Acceptance covers BIC01–05
and preserves old assertions. Root reruns accepted source independently.

|Role|Branch/worktree|Write ownership|
|---|---|---|
|Source|`commerce/media-input-source-20260927`; `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`|0040, post0009; `internal/live/media_plan.go`, new `media_input.go`; `internal/platform/media_runtime.go`|
|Independent tests|`commerce/media-input-tests-20260927`; `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`|New `tests/foundation/live_media_input_custody_test.go` only unless an exact setup correction is approved|
|Integrator|main checkout|Contract, task metadata, this record; final source/test merge and root acceptance|

Source/test started at `4809128`; clarification cherry-picked as `90dd1c7` /
`12ea7c3` respectively. Prior untracked test `output/` evidence is preserved.
No other worktree is reset, deleted or archived for this increment.

Required independent/root commands; no BIC pass is claimed before their recorded
results and the two findings below are resolved:

```sh
bash scripts/dev/test-local.sh --live-media-stop
bash scripts/dev/test-local.sh --live-media-runtime
bash scripts/dev/test-local.sh --studio-backend
bash scripts/dev/test-local.sh --browser-studio-bff
bash scripts/dev/test-local.sh
```

The BIC kernel has no consumer or provider wire path. Tests for post-wire joint
completion may seed that future state as an isolated privileged fixture, then
exercise real fenced executor functions. Such tests are database invariant
evidence, never real Start/Stop or browser-to-provider evidence.

Preflight receipts: `1288c97f-bfcd-496d-8f06-c6c6f3548bb4`,
`44467d60-d774-4a8f-bb67-cebc15a00453`,
`67e04874-8a94-410a-8b4d-35d0238467d8` (Humaux).

No customer, production database, provider project, token or broadcast changed.
Orders C native visibility MOU03, Studio UI, real Cloud/Egress, BRI04–07 and
full T08/G06 remain separate unfinished acceptance. Fill source/test commit,
commands, exit codes, evidence hashes and cleanup only after actual execution.

## Fixed candidate and repair gate

Source candidate `ac4782d` changes only the five assigned source
files. Author unit tests and vet pass; existing isolated media-plan and media-stop
regressions pass. The latter does not contain the independent BIC tests and is
not BIC acceptance. Root inspected both successful logs:

- `/Volumes/data/output/bic-source-plan-second-20260927.log`, exit 0,
  foundation 20.685s, SHA256 `f43c53df6814de756f982926b67631270e4ab631ab904b3411152b3950e64782`.
- `/Volumes/data/output/bic-source-stop-first-20260927.log`, exit 0,
  foundation 232.847s, SHA256 `cd4e56a762e19059f83991939d744b740815ee50742137ef0486ca3b8309f242`.
- Initial parser failure is preserved in
  `/Volumes/data/output/bic-source-plan-first-20260927.log`.

Independent fixed-source review confirmed two P1s, assigned as targeted repair 1:

1. Direct UNISSUED budget exhaustion closes the child but incorrectly leaves the
   operation/event UNKNOWN and returns held. The next claim can then return
   terminal without a matching terminal row. Repair must decide combined
   completion first and keep child, operation, event and return consistent.
2. Input readiness omits some new private SECURITY DEFINER/register ACL and guard
   shape checks. PUBLIC/unauthorized EXECUTE poisoning and wrong trigger type
   must fail readiness, not leave a writable internal helper advertised as ready.

The earlier closed-admission state-regression candidate is excluded by the
source CHECK. `await_admission` is an accepted no-lease ABI clarification, now
explicit in the contract, not a new consumer implementation.
Review receipt: Humaux `d065745c-019b-425c-9b8f-bf606c79bf76`.
Independent tests run in their own worktree; tests must exercise direct budget
claim, not only Stop-then-claim, and fixed replay after RESERVED reconcile claim.
These findings were repaired in `3401101`. Independent fixed-diff review found
both P1s closed and no new confirmed P0/P1. Runtime worker schema USAGE was added
without child-table privileges so its intended readiness function can execute.
The initial source and repair are integrated as `8cb91bd` and `cadeff0`; independent
test-only commits are integrated through `e0a30f8`. Root verified the six source/
test files are byte-identical to the independently tested branch.

Independent PG18 `--live-media-stop` passed 53 top-level tests, including six BIC
tests, exit 0, foundation 241.986s:
`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926/output/bic-second-pg-20260927.log`,
SHA256 `f89963dd6717049307c2783e711b0c0d0b9d3e1cd9e18ef3cb65ef5c8d05467a`.
Direct UNISSUED 4096-generation/25-hour claims, native-job retention and readiness
PUBLIC/unauthorized EXECUTE/trigger-shape poisoning are included.

The first independent run is retained at the same directory's
`bic-first-pg-20260927.log`, SHA256
`796c1ee4a76376da38b83e04d029301160c069bc6d511169c9bbda398307230e`.
Its worker schema error is fixed. Its parent `media_attempts.created_at` owner
mutation assertion was mapped to the wrong contract: the input child's timestamp
is immutable; parent-owner corruption must instead make readiness false. The test
now proves child immutability and parent-corruption detection with exact restore,
without widening historical parent-schema rules. Existing LMR05 also failed in
the first run and passed the second unchanged; no timeout/assertion was relaxed,
and the timing variance remains unclassified pending root regression.

Root `--live-media-stop` passed at fixed `e0a30f8` (documentation-only HEAD
`7918e46`): exit 0, 53 top-level PASS, 0 FAIL, 0 SKIP, foundation 249.973s.
Log `/Volumes/data/output/bic-root-stop-20260927.log`, SHA256
`76f61d4a2946e46135650e6acb8d3e160d9962692fc5b09afca875cded7fe40c`.
The original LMR05 passed unchanged again. Its first-run variance remains
unclassified, not erased from the record.

Coverage review still found required BIC03 direct issued-input terminal projection
and finish-uncertain paths missing from the new tests. An independent test-only
complement is assigned before full BIC acceptance. It must exercise actual fenced
functions after explicitly labelled isolated future-state fixtures, not claim real
provider wire. Runtime/Studio/BFF/full root gates are also pending. This record does
not upgrade Cloud, browser input, Studio UI or deployment gates.

## Future deployment stop line

0040/post0009 add exact executor capabilities. Older binaries intentionally
reject that expanded allowlist; this is not a transparent rolling migration.
Before any production rollout, prove a compatible build/migration procedure in
an isolated staging database, inventory active original jobs, and obtain owner
approval for the concrete backup and maintenance scope. Do not stop or interrupt
customer broadcasts to make migration convenient. This local increment does not
authorize that rollout, a backward migration or deletion of held input liability.
