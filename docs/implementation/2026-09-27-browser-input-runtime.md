# Browser input custody implementation record — 2026-09-27

Status: **BIC_KERNEL_FOCUSED_AND_ROOT_FULL_PASS; product browser admission remains downstream**.
Source base `2b52e7d`; frozen contract:
[BIC/BRW](../../contracts/live-browser-input-runtime-v1.md).
Contract freeze `4809128`, cleanup-authority clarification `234f44a` (reviewed,
no ABI change): merchant revocation denies new grants but cannot strand the
original executor's cleanup responsibility.

## Current regression disposition

The historical LMR05 full-run failure below was not erased. After the owner's
explicit wait-only adjudication, root fixed tree `b99a2d1` passed the full
PG18/race/vet suite: **696 PASS / 0 FAIL / 0 SKIP**, actual exit 0, foundation
1132.853 s. All BIC kernel tests ran in that suite. See the
[MRR full receipt](2026-09-27-media-recovery-acceptance.md#current-full-regression-after-owner-adjudication)
for the raw log and hash. This closes the kernel integration regression, not
the disabled token-delivery/new-queue worker, product browser input or Cloud
qualification described below.

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

Coverage review found required BIC03 direct issued-input terminal projection
and finish-uncertain paths missing. Independent test-only complement
`8d77d6c` + `4c14717`, integrated as `acdbd4e` + `b6cfa4e`, now exercises both
executor observation wrappers plus remote-unknown/policy-denied uncertain finish.
Each proves the original operation/event/job retains issued input liability.
Future-wire state is explicitly owner-seeded in a disposable fixture: this is
actual fenced SQL evidence, never real provider wire.

The first complement run failed on its incoherent test report (100/130/140
start/update/end timestamps), correctly rejected as ME400. It was corrected to
100/140/130 without relaxing product validation; the failure log remains
`output/bic-postwire-pg-20260927.log` in the independent test worktree, SHA256
`df866893942fdfe0d907318cf6912197216ef724f0c3edddfddb8ddeb5d66ad5`.
Final independent full media-stop run passed 53 top-level tests, exit 0,
foundation 260.489s, `output/bic-postwire-final-pg-20260927.log`, SHA256
`40d7f5b20ececdff688cdb3f31e83f5c1ede657897b6344e2980355cfc79d901`.
Receipt: Humaux `35b7b9d2-c679-4d87-86e6-9308e10be964`.

Root fixed Go source/tests `b6cfa4e` passed these serial gates; evidence lives in
`/Volumes/data/output/`:

|Gate|Exit / result|Log / SHA256|
|---|---|---|
|`--live-media-input`|0; 6 PASS; foundation 19.058s|`bic-root-input-final-20260927.log`; `cd1f13f4c123d39b78e0f6363924c31f5f2f8bff01fe993336fe47f061d4a672`|
|`--live-media-runtime`|0; 8 PASS; foundation 30.660s|`bic-root-runtime-20260927.log`; `f3d19002021898fca8c20fe93026e797e117c8bb9c349c85e8f017fdb570f840`|
|`--studio-backend`|0; 8 PASS; foundation 13.648s|`bic-root-studio-20260927.log`; `6ff8af8a77bb5efb4dc6f76dde4a722218316d604bbc861453db046fb0fc5730`|
|`--browser-studio-bff`|0; signed chain 1 + Node query 2, admin build; foundation 6.069s|`bic-root-bff-20260927.log`; `6c85c43af6b761a7ca803f0cb0db67f53cd320727a1eee1b984444c2dd95a2b6`|

The new input-only selector in `50b1007` reuses the same isolated PG18/race runner
and rejects missing tests. It shortens repair feedback, not final coverage: the
prior stop group took 249.973s, its six BIC tests totalled 14.08s, and the actual
focused group above took 19.058s including package/setup overhead. The full
PG/race/vet root run produced 669 top-level PASS, 0 FAIL, 0 SKIP across 33 test
packages (foundation 752.728s), then reached the post-vet PASS marker. However,
the wrapper exited 127 afterward with `line 251: is: command not found`.
The integrator had changed the executing Bash script to register STU04; the
current script passes `bash -n` and has no such command. This is consistent with
the shell resuming at a changed byte offset. The wrapper result is a failed
acceptance, not a clean full pass. Preserve
`/Volumes/data/output/bic-root-full-20260927.log`, SHA256
`e7e8a37ac100fe160367166c11e6a958450ab4ba27f0b2219d40457d7187cfce`.
Separate root `go vet ./...` passed again. Freeze both code and runner throughout
the next full run; never edit an executing runner. Studio remains a separate,
unmerged candidate and must not delay this kernel's independent regression.
No test assertion, production lease or fault deadline was relaxed. Cloud,
browser input, Studio UI and deployment gates remain unchanged.

## Frozen full rerun — retained LMR05 failure

Main `549fc07` (Go source/tests still `b6cfa4e`) ran the entire unchanged runner
and exited 1: **668 top-level PASS, 1 FAIL, 0 SKIP**; foundation 715.044s.
The runner SHA256 remained
`3767bc9b924e186fca9397a3c0e295e26ef7054393c0ee0af5c309aa14d00bfb`
before and after. Log `/Volumes/data/output/bic-root-full-frozen-20260927.log`,
SHA256 `cc439c0337a9c7382692811acb6f9865df5ba1bec9940fd1998795eb88849995`.
The runner did not reach its post-test vet command. Its task-owned fixtures
were removed; unrelated containers were left untouched.

The only failed case was `TestLiveMediaStopLMR05RealCrashAndCommitAckLoss` at
`live_media_stop_test.go:1178`, after 57.86s. Its exact wait requires a fourth
observation and a closed lease, **not terminal resource closure**. Actual facts
were generation 3, observations 3, `UNKNOWN / media_stop_reserved / OBSERVED /
EGRESS_ACTIVE`, cleanup required, no open lease. The intended final assertion
also keeps liability UNKNOWN and forbids another external Start/Stop.

Independent read-only diagnosis `9b7a41a4-fdab-4c3d-8229-4790f8b35fd2`
identifies a test/native scheduling hypothesis: `lmrRescue` ages a running job,
but native River rescue scanning and retry backoff still precede the next Query.
The current log lacks native job state/timestamps at the failure and cannot
prove that hypothesis. Capture those facts before choosing a repair; do not
call it a flake, extend the timeout, change production rescue defaults, or count
this failed full run as BIC05 acceptance. The earlier focused passes stand only
at their stated scopes.

Diagnostic-only `51f9f3f` adds a bounded native-job snapshot (no args/error
payload) and `--live-media-crash`, an exact non-empty selector reusing the same
fixture. Independent review `5ea60a77-d59f-41b2-8739-93a1abf1428b` verified
cleanup ordering, field types and unchanged 35s/final assertions. The added
pre-restart SELECT can move scheduling phase by up to two seconds; a pass is
not evidence that the original failure is resolved.

Its first exact run exited 0, test 53.81s / foundation 58.036s. Log
`/Volumes/data/output/lmr05-native-diagnostic-20260927.log`, SHA256
`d8897778957a00cf8df6aeeaaa466c2898ed27ad5f883d100580684d790af60d`.
Before restart the native row was running, attempt 2, error count 1, no finalized
timestamp, and artificially aged attempted_at as required by the existing crash
fixture. This time the fourth Query reached the unchanged deadline; the failure
snapshot did not run. Native timing remains a hypothesis for the failed full
run. No production timing, timeout or assertion has changed. Fixtures were
removed; further work must gather causal scheduling evidence, not count retries
until green.

Phase-only diagnostic `4f721d3` then captured native leader/job transitions at
most four samples/second inside the same 35s predicate. Leader IDs remain only
in the in-memory change key; logs contain safe phase/timing fields. Independent
review `bed4c99d-c335-4450-8a46-eda075b8deac` found no blocking issue, noting
sampling can perturb timing and miss short-lived states. Its single exact run
exited 0, test 55.71s / foundation 57.475s:
`/Volumes/data/output/lmr05-native-phase-20260927.log`, SHA256
`bb1a61fe343acb3654eaedaea45690a677df2f099129353527c6a455e3b4ba1a`.

|UTC phase|Observed native fact|
|---|---|
|08:28:42.061, before restart|Running attempt 2, error count 1; killed leader lease still valid until 08:28:54.208|
|08:28:58.289|New leader elected|
|08:29:00.585|Rescued as retryable, error count 2, due at 08:29:16.088|
|08:29:11.372|Available, but still scheduled in the future|
|08:29:16.267|Available and due|
|08:29:16.724|Attempt 3; fourth-observation/closed-lease predicate reached|

About 34.66s elapsed from restart to the required observation. Pinned River
v0.40 source explains leader expiry/election plus default second-rescue retry
(16s ±10%) and fetch scheduling. This is measured support for a scheduling
budget mismatch, not a captured explanation of the prior failed full run.
Do not erase that failure or change the 35s assertion. A source-policy correction
needs its own safety review, including native attempt exhaustion and retained
resource liability; none has been implemented here. No more random diagnostic
reruns are authorized by this receipt.

Independent retry-policy review `bb58f909-eb14-4539-bc4a-e240243e801f`
found no product contract establishing a 35s recovery SLA. Rejection receipt
`a7cabd53-ad9c-40ce-a5e8-3ba27b5c8ef2` rules out unconditional five-second
native retry: it applies to ordinary errors as well as rescue, and native
`MaxAttempts=25` can discard the original job while UNKNOWN resource liability
remains. The old plain MOCK guard does not prove never-discard custody; the
4096-generation/24h business budget is not a native attempt guarantee.

Owner subsequently selected **90 seconds**, decision
`f0b36444-a4ea-4112-926d-596cfb7fd89f`. The outcome is recorded in the
[recovery-policy draft](../../contracts/live-media-recovery-v1.md); detailed
policy and implementation remain pending independent preflight. Before
changing policy, freeze clock boundaries, eligible failures, exhaustion
retention and independent fault gates. Current River rescue defaults to one
hour; the old LMR05 two-hour age shortcut does not prove this wall-clock target.
Neither a timeout increase nor a fast retry chosen to fit this one test is a
repair. No retry policy, deadline, external service or customer stream changed.

Maintenance/caller map: [media-input-custody.md](media-input-custody.md),
committed in `0eb3d59`.

## Future deployment stop line

0040/post0009 add exact executor capabilities. Older binaries intentionally
reject that expanded allowlist; this is not a transparent rolling migration.
Before any production rollout, prove a compatible build/migration procedure in
an isolated staging database, inventory active original jobs, and obtain owner
approval for the concrete backup and maintenance scope. Do not stop or interrupt
customer broadcasts to make migration convenient. This local increment does not
authorize that rollout, a backward migration or deletion of held input liability.
