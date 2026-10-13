# OA-1 delivery — in progress

- Branch: `unit/oa-1-order-attempt-core`; current merged base:
  `256e9740ced9c5de05a83f58eb9f4c79a231b7b2` (fetch reconfirmed 2026-10-14).
- Implementer: Codex-3; reasoning: high. Allowed paths and boundaries:
  `docs/delivery/units/oa-1-order-attempt-core.md`.
- Contract: frozen order-attempt-v1 plus reviewed A1/A1.1 at
  `cb4256f37586aad5139f1216fc92d6226fa70003`, merged in `0f1060a72`.
  Retired scopes reject fresh CAS, but preserve authorized exact replay during
  retention. Backend separate from paused PR #32.
- Current artifact: native C1 admission, complete single-transaction C2,
  inline/River execution and periodic sweep. Their scoped CI receipts below
  do not constitute complete OA-1 acceptance: A2 retirement is verified below;
  privacy compatibility, the remaining crash/mutation matrix and final gates
  are still under verification. The public placement path is not enabled.

## Evidence

Current acceptance index (OA-1 implementation; the detailed receipts retain
their tested SHA below):

| Contract | Native/model oracle |
| --- | --- |
| §6a atomic admission/execution | `TestOrderAttemptC1CrashBeforeCommit`, `TestOrderAttemptExecutorCrashBarrierMatrix`, `TestOrderAttemptMatrixCVSPickupCrash`, both `*CommitAcknowledgementLoss` tests, `TestOrderAttemptExecutorNativeCommitAndLostResponseReplay` |
| §6b/c server identity and replay | `TestOrderAttemptServiceConcurrentFreshKeysOneCAS`, `TestOrderAttemptMatrixTerminalCASRace`, `TestOrderAttemptTerminalIdentityReplay`, `TestOrderAttemptCrossClient*`; retained same-key terminal receipts and explicit next-generation CAS |
| §6d/k owner and key custody | `TestOrderAttemptMatrixConfiguredRotationPreservesAdmittedOwner`, `TestOrderAttemptExecutorPlanRotationAndMissingOriginalKey`, `TestOrderAttemptKeyRetirement*`, exact schema/materialization ACL tests and actual runtime/worker first-fence barriers |
| §6e privacy prerequisites | `TestOrderAttemptErasure*`, native D-S2/ads erasure compatibility and populated historical upgrade. Full retention/legacy cutover remains OA-3. |
| §6f cancellation compatibility | `TestOrderAttemptNativeCancel*`, `TestOrderAttemptCancelNativeCommittedRecovery`, native expiry tests; no renewed stock or credential on replay |
| §6i liveness and errors | `TestOrderAttemptSweep*`, `TestOrderAttemptDeadlineExecution*`, `TestOrderAttemptExecutorAbortClassification`, actual enabled/disabled worker process and discarded-job periodic recovery |
| §6j frozen facts | `TestOrderAttemptFrozenExecutor*`, `TestOrderAttemptCrossClientSameBuyerIndependentCarts`, native quote/destination/allocation/claim origin tests |
| §6k process composition | `TestOrderAttemptProcessAPIRejectsSplitBuyerPools`, `TestOrderAttemptProcessWorkerRejectsSplitParticipantPools`, `TestOrderAttemptWorkerRealConsumerRacesInline` |
| 4259504799 continuation | `TestOrderAttemptNotificationResume*`: worker completes without placement-session/grant re-entry; current messaging refusal becomes VOID |
| §6l local C2 prerequisite | `TestOrderAttemptNotificationNoBindingVoidNativeAndRollback`, `TestOrderAttemptNotificationReadyNativeActualCrypto`, strict NULL-operation CHECKs. Dispatch/lease sweep/reminder/CAS and M2a/M4/M7 remain OA-2. |
| Go model / testing.F | `TestOrderAttemptModelSeeds`, `TestOrderAttemptModelOracleMutations`, `FuzzOrderAttemptModel`; explicitly MODEL_ONLY |

Public A15/A16/recovery adapters, browser pre-admission (§6h), 90-day retention
and legacy partial cutover (§6m) remain OA-3. OA-1 does not activate the new
placement route before OA-2/OA-3 can enforce those boundaries.

| Revision | CI run | Gate | Result |
| --- | --- | --- | --- |
| `b58b834d4` | [38422920898](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38422920898) | `focused:^TestOrderAttemptSchema` | RED: `PASS=0 FAIL=3 SKIP=0 exit=1`; missing identities/tables/execution entries after successful REAL_PG setup; job `115385256709`, `schema-red-ci.log` |
| `0232733e2` | [38424566780](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38424566780) | `focused:^TestOrderAttempt(Provenance\|Model)` | REAL_PG provenance RED (5); MODEL_ONLY seeds/oracle PASS (2), `PASS=2 FAIL=5 SKIP=0 exit=1`; job `115391256051`, `provenance-model-red-ci.log` |
| `efcdd3998` | [38426328576](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38426328576) | `focused:^TestOrderAttempt(Schema\|Provenance\|Model)` | REAL_PG strengthened baseline RED; `PASS=2 FAIL=11 SKIP=0 exit=1`, job `115396767591`, `strengthened-red-ci.log` |
| `1e616d320` | [38426867350](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38426867350) | storage identity/authority/incomplete-admission + model | `PASS=4 FAIL=1 SKIP=0 exit=1`; incomplete-admission fixture stopped before commit with SQLSTATE 42P08 (UUID/text parameter inference). Not behavioral proof; fixed explicit UUID cast, assertions unchanged. Job `115399906105`, `storage-first-ci.log` |
| `68ba72cc9` | [38428406310](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38428406310) | storage/provenance/state/model/authority | `PASS=16 FAIL=4 SKIP=0 exit=1`; actual storage commit and state tests PASS; three authority tests RED on absent C1 entries, one provenance RED on missing direct ledger/session FK. Job `115404898781`, `storage-provenance-authority-ci.log`. Exact FK added; assertions unchanged. |
| `14065204e` | [38429245975](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38429245975) | `focused:^TestOrderAttemptFacts` | RED: `PASS=0 FAIL=6 SKIP=0 exit=1`; successful REAL_PG setup, coordinator has no checkout access and facts function is absent. Job `115406897065`, `facts-red-ci.log`. |
| `2dc37d8cc` | [38430215747](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38430215747) | OA focused seams | `PASS=7 FAIL=25 SKIP=0 exit=1`; two model and five codec tests passed. PG setup failed on 0173 PL/pgSQL CASE parsing (42601), so PG behavior was NOT_RUN. Job `115410251105`, `admission-seams-first-ci.log`. Parenthesized IF-contained CASE conditions in `fa6a57dd7`; assertions unchanged. |
| `b54e4ee0b` | [38430742365](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38430742365) | real C1 preparation | `PASS=0 FAIL=2 SKIP=0 exit=1`; same migration setup failure, not causal C1 proof. Job `115412090363`, `admission-first-ci.log`. |
| `ce14e8892` | [38432101298](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38432101298) | `focused:^TestOrderAttempt` | `PASS=32 FAIL=18 SKIP=0 exit=1`; storage/provenance/authority/facts and codec/model pass; absent C1, subject and job helpers are causal RED. One subject isolation fixture has a wrong tenant/store pair (23503) before its assertion; corrected separately, not counted as isolation proof. Job `115416759643`, `c1-seams-red-ci.log`. |
| `4f1d17643` | [38432782403](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38432782403) | A1 retirement/schema baseline | RED `PASS=1 FAIL=5 SKIP=0 exit=1`: missing partial unique predicate, reactivation refused, pending retirement accepted, principal lock absent. Job `115419129700`, `scope-retirement-red-ci.log`. |
| `21cf485b3` | [38433099080](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38433099080) | claim/job implementations plus native fixtures and replay RED | `PASS=27 FAIL=2 SKIP=0 exit=1`; claims, job guards, state, facts and provenance pass. Two exact-replay tests causally RED because prepare is absent (42883). Job `115421727992`, `c1-claims-jobs-ci.log`. |
| `ad755a9e9` | [38434570109](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38434570109) | retired scope, subjects and C1 SQL; catalog quantity boundary RED | `PASS=26 FAIL=1 SKIP=0 exit=1`; retirement, retained exact replay and other admission tests pass. Quantity 1000 causally refused with PT400; fixed in `66f0167a0` without changing claim ledger allowances. Job `115427121077`, `admission-boundaries-ci.log`. |
| `11bb42f68` | [38434841389](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38434841389) | Go C1/calculation and replay authorization during scope wait | `PASS=3 FAIL=2 SKIP=0 exit=1`; revoke orders:read/session while waiting returned a receipt instead of PT403/PT401. Catalog-only revocation replay passed. Fixed final current-read check in `999f4282d`. Service atomic positive readback used nonexistent `orders.buyer_owner_id`; corrected to actual `owner_id` in `a9f1b3b74`, assertions unchanged. Job `115427367242`, `c1-service-replay-red-ci.log`. |
| `598aa6b44` | [38436791173](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38436791173) | C1 repairs plus execution-fence/context tests FIRST | `PASS=20 FAIL=7 SKIP=0 exit=1`: C1 repairs, exact retired replay, post-wait read revocation and atomic admission PASS. Seven execution-fence cases causally RED on absent entry/ACL. Job `115433959896`, `execution-fence-red-ci.log`. |
| `8cdb9f8a4` | [38438426265](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38438426265) | frozen origin/CVS source facts FIRST | RED `PASS=0 FAIL=5 SKIP=0 exit=1`: missing frozen origins/pickup facts, zero origins and invalid/disabled CVS codes accepted. Pay-at-pickup positive also returned unavailable and requires separate diagnosis. Job `115439047589`, `materialization-facts-red-ci.log`. |
| `ad9a0efa9` | [38439304096](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38439304096) | CI-only formatting receipt before focused fence/quote tests | Formatting emitted `oa1-gofmt.patch` then exited 1; Gate step did not run: **NOT_RUN**, not behavioral RED. Applied the CI-produced formatting diff without executing a local formatter, and removed the temporary workflow step. Job `115441543152`, `execution-fence-first-ci.log`; artifact `gate-focused_TestOrderAttempt_ExecutionFence_QuoteMaterialization`. |
| `b89e34670` | [38439812016](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38439812016) | `focused:^TestOrderAttempt(ExecutionFence\|QuoteMaterialization\|MaterializationFacts)` | `PASS=13 FAIL=5 SKIP=0 exit=1`: five materialization-facts tests, seven fence/context tests and legacy DTO test PASS. Actual-logins failure is only the new worker NULL-store case (PT400); fixed nullable worker discovery. Quote helper/schema ACL absent and typed checkout snapshot loses origins: causal RED. Readback fixture separately fails UUID/text parameter inference (42P08), not product proof; fix explicit UUID cast. Job `115443290001`, `fence-quote-facts-ci.log`. |
| `d765b64c3` | [38441649790](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38441649790) | quote/fence/facts/abort | **CANCELLED**, no completed aggregate. Two Go Abort tests causally RED on missing method after real C1; SQL NoKey/Reasons/Expired tests reach missing `ops.abort_order_attempt` (42883). Reasons omitted rollback on fatal assertions, exhausting the eight-connection pool; later setup times out. Those later behaviors **NOT_RUN**, not product failures. Root canceled its hung run; bounded cleanup repair `8f6466f9b` preserves assertions. Job `115449623737`, `quote-abort-ci.log`. |
| `3a9659561` | [38442987734](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38442987734) | destination/arithmetic/atomic boundary FIRST | Compile failure: undefined test helper `oaStateCode`; `PASS=0 FAIL=0 SKIP=0 exit=1`. Behavioral **NOT_RUN**. Corrected to existing `oaFenceCode` in `03e831309`. Job `115454631450`, `destination-atomic-red-ci.log`. |
| `447f0b64b` | [38443900143](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38443900143) | same destination/arithmetic/atomic boundary assertions, positive browser-cart sentinel | Causal RED `PASS=0 FAIL=8 SKIP=0 exit=1`: frozen arithmetic verifier absent; native PG COMMIT incorrectly accepts a partial C2 while PENDING; six destination groups reach absent helper/ACL after real C1/fence/quote. Job `115457395814`, `destination-atomic-behavior-red-ci.log`. |
| `02e32a928` | [38444964650](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38444964650) | execution fence, frozen calculation, quote and source facts | GREEN `PASS=19 FAIL=0 SKIP=0 exit=0`; job `115462207527`, `quote-calculation-green-ci.log`. This verifies these seams, not full C2 placement. |
| `5c4b05944` | [38445363970](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38445363970) | abort implementation | `PASS=5 FAIL=5 SKIP=0 exit=1`; odd hex (22023), scoped subject fixture and own-admission refusal require repair. Job `115464038511`, `abort-first-ci.log`. |
| `f86a8538c` | [38445601229](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38445601229) | atomic commit guard plus abort | `PASS=6 FAIL=7 SKIP=0 exit=1`; atomic guard assertions pass, abort errors remain and worker earlier-lock test times out. Job `115464160350`, `atomic-abort-first-ci.log`. No abort GREEN claim. |
| `d0427f355` | [38446333503](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38446333503) | destination plus atomic commit guard | `PASS=9 FAIL=1 SKIP=0 exit=1`; native fields and destination guards pass; typed snapshot roundtrip compares equivalent UTC instants with different Go location objects. Replaced struct identity with complete JSON roundtrip equality plus instant equality, retaining all field checks. Job `115465711516`, `destination-first-ci.log`. |
| `cdb39af25` | [38447000640](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38447000640) | allocation tests FIRST | Causal RED `PASS=0 FAIL=6 SKIP=0 exit=1`; real C1/fence/owners/quote preparation succeeds, private catalog/allocation helpers and ACL are absent (42883). Job `115468047167`, `allocation-red-ci.log`. |
| `8a5d1c574` | [38447322750](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38447322750) | fence, abort, destination and atomic commit guard | GREEN `PASS=28 FAIL=0 SKIP=0 exit=0`; native worker no-earlier-lock barrier and 750ms bound unchanged. Job `115469135979`, `fence-abort-destination-green-ci.log`. |

| `5db1ba040` | [38448591686](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38448591686) | allocation | GREEN `PASS=6 FAIL=0 SKIP=0 exit=0`; native aggregate/mixed/untracked allocation facts, current safety, frozen prices, barriers and exact ACL. Job `115473491889`, `allocation-green-ci.log`. |
| `31130f771` | [38449036549](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38449036549) | native Begin FIRST | Causal RED `PASS=0 FAIL=4 SKIP=0 exit=1`: real C1, quote, destination and allocation succeed; native Begin helper/ACL absent (42883). Job `115475000788`, `begin-red-ci.log`. |
| `1c591f311` | [38449590596](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38449590596) | payment-entry codec FIRST | Compile/API absence `PASS=0 FAIL=0 SKIP=0 exit=1`: undefined PayEntry types. Behavioral **NOT_RUN**, not a red security proof; independent AAD mutation remains required. Job `115476875309`, `pay-entry-api-absent-ci.log`. |

Owner trunk update: fetched `origin/r3/integration` at `228a1c877` and merged
without conflicts as `a35423055`. The merged frozen contract includes A1/A1.1;
its file has no difference from trunk. Trunk root migration maximum remains
0169; the next root slots must be checked again before the PR. No local gates
ran during the merge. This does not claim complete C2 or OA-1 acceptance.

Further CI receipts (same assertions; no local execution):

| SHA | CI run / native job | Result |
| --- | --- | --- |
| `829c39c0d` | [38450594745](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38450594745), `115480342107` | Causal RED `PASS=2 FAIL=4 SKIP=0 exit=1`: real C1 omitted accepted `line_version`; injected codec mutation removed order from AAD and broke wrong-order binding, independent AES oracle and valid ciphertext assertions. `codec-mutation-origin-version-red-ci.log`. Mutation restored in `41ea35dae`; version fixed in `e2fbab344`. |
| `e2fbab344` | [38451230174](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38451230174), `115482543388` | GREEN `PASS=6 FAIL=0 SKIP=0 exit=0`: all five codec groups plus actual accepted-origin version. `codec-origin-version-green-ci.log`. |
| `6b0188e36` | [38450817342](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38450817342), `115481095581` | Causal RED `PASS=0 FAIL=3 SKIP=0 exit=1`: actual runtime/worker C1 and fence succeeded; composite entry absent (42883). `composite-prepare-red-ci.log`. |
| `1d8506bf8` | [38451519568](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38451519568), `115483469675` | `PASS=3 FAIL=1 SKIP=0 exit=1`: native Begin effects/rollback, forged allocation and exact ACL pass. Frozen-settings home fixture used an empty CVS-map entry and failed before Begin. `begin-first-ci.log`. |
| `70c9e330f` | [38451701800](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38451701800), `115484125017` | `PASS=6 FAIL=1 SKIP=0 exit=1`: all three actual-login composite prepare groups pass, including native partial-COMMIT rejection and no placement re-gating. Same home fixture failure; not product acceptance for frozen-settings case. `composite-begin-first-ci.log`. |

| `153343faa` | [38452442812](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38452442812), `115486656817` | Causal RED `PASS=1 FAIL=1 SKIP=0 exit=1`: actual prepare succeeds, composite Begin rejects absent stage. `composite-begin-red-ci.log`. |
| `26b1724fd` | [38452606460](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38452606460), `115487243016` | Causal RED `PASS=0 FAIL=11 SKIP=0 exit=1`: native C1/Begin prerequisites reach absent claim-completion/pay-entry helpers. `claim-pay-store-red-ci.log`. |
| `bde893cb4` | [38453470440](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38453470440), `115490195920` | `PASS=8 FAIL=1 SKIP=0 exit=1`: native Begin and both executor-login stages pass; altered-stage inputs leak underlying PT409 rather than the required PT400. `composite-begin-ci37.log`. |
| `7320538cf` | [38454408486](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38454408486) | GREEN `PASS=20 FAIL=0 SKIP=0 exit=0`: all composite/Begin/claim-completion/pay-entry storage groups. `composite-claim-pay-green-ci.log`. |

| `e796bd02e` | [38455374619](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38455374619), `115496804514` | Settlement FIRST: causal RED `PASS=0 FAIL=4 SKIP=0 exit=1`. Actual C1/native Begin reach the absent `pay` stage, PT400. `settlement-red-ci.log`. |
| `3484ad3cd` | [38454795431](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38454795431) | Notification first attempt `PASS=0 FAIL=6 exit=1`; fixture retained checkout-writer role after the native pay helper. This setup failure is not notification behavior proof. `notification-red-ci.log`. |
| `f38c6a47d` | [38455899509](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38455899509) | After the isolated RESET ROLE fixture repair: `PASS=0 FAIL=6 SKIP=0 exit=1`, reaching absent coordinator schema/helper authority. `notification-fixture-corrected-red-ci.log`. |
| `acdab5f0b` | [38456246349](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38456246349) | Expanded notification FIRST: `PASS=0 FAIL=8 SKIP=0 exit=1`; signed native ingestion and C1/Begin/pay reach missing notification preparation/ACL/FKs; origin helper missing. `notification-ready-red-ci.log`. |

CI `38456636061` at `761f4fb4f` completed with exit 1 and zero matched test
results because the newly required `Executor`/`NewExecutor` API was absent.
This is API-only RED, not executed behavior proof; `executor-api-red-ci.log`
retains the exact compile failure. Native behavior and mutation acceptance
must execute after implementation. The existing workflow bound is unchanged.

CI `38458877458` at `054d325f0` did **not** execute either requested gate:
the temporary formatter evidence step treated Go 1.27's diff exit 1 as a
step failure. `full-c2-first-ci.log` and `unit-first-ci.log` retain that setup
failure. The generated formatting patch is tied to that exact SHA and applied
without local formatter execution. The evidence collector now records this
status; actual structural/format gates still test the unchanged checkout.
This run proves neither C2 behavior nor a product red/green.

CI `38459352221` at `2e407f05d` is the replacement run for
`focused:^TestOrderAttempt(Notification|Settlement|Executor|Expiry)` and
`shard:unit`, including the actual signed-ingress READY executor case. Root
monitors the unchanged 150-minute workflow bound and its logs/artifacts.
Focused job `115512700152` reached migration 0189 but failed its PL/pgSQL
IF/CASE parsing (42601), before any C2 assertion: `PASS=0 FAIL=24 SKIP=0 exit=1`.
`full-c2-ci45.log` retains the setup failure; behavioral acceptance is NOT_RUN.
The RHS CASE expressions are now parenthesized without changing predicates.
The unit job `115512700279` also finished red: 0189 prevented other native
package fixtures from loading, and `LoadKeys` was absent at compile time.
`unit-ci45.log` records both. Configuration behavior remains NOT_RUN.

CI `38459781617` at `f781f06cf`, job `115511819647`, gives sweep API-only RED:
missing `Sweep`, `AddWorkers` and `PeriodicJob`, `PASS=0 FAIL=0 SKIP=0 exit=1`.
`sweep-api-red-ci.log` preserves the compile diagnostics. Native sweep behavior
must run after the implementation; this is not a SQL behavior result.

CI `38460669227` at `882527e4b`, job `115514848500`, reached native tests:
`PASS=5 FAIL=28 SKIP=0 exit=1`, `full-c2-ci47.log`. C2's pay stage causally
failed with 42P01: a function-name qualifier was used for a local PL/pgSQL
variable. It now uses an unambiguous local name. Sweep SQL absence is causal
RED (42883 after real C1); its product migration follows this receipt. The
concurrency fixture now observes transitive native row-lock waiters, and the
timeout assertion reads the public redacted AdmissionError SQLState rather
than assuming a raw pgconn error; it still requires exactly 55P03 and additionally
requires the retryable code and a known-uncommitted outcome.

CI `38461005406` at `c57e09eaa` runs the API physical-database boundary FIRST
test through the packaged binary. Root monitors this pending result under the
unchanged workflow bound; no process acceptance is claimed yet.

The full C2 composite and terminal proof are in progress; notification helpers,
Go executor, native expiry compatibility, worker/sweep and the complete matrix
remain unverified. No OA-1 completion claim is made.

CI is the only execution environment. Local tests/builds/gates/server/PG:
**NOT_RUN(owner CI-only directive)**. Static source review does not prove C2.
CI workflow gate timeout is 150 minutes; the parent tracks the run and reads
its terminal result/log. No acceptance claim until assertions execute.

The worker abort lock failure was an implicit PostgreSQL foreign-key recheck:
updating the attempt's witness and then its terminal state in the same transaction
reacquired the immutable scope after the attempt. `8a5d1c574` moves the witness to
a protected, current-transaction fence table; C2 locks the attempt without
updating it before settlement. The separate witness has no runtime DML grants
and cascades with its retained attempt. The real held-scope barrier now passes;
PENDING requeue and terminal execution must continue to use separate transactions
so a future job-pointer update cannot reintroduce that dependency.

## Contract-clause → test mapping

- §1/§2/§6k → `TestOrderAttemptSchemaDurableIdentities`,
  `TestOrderAttemptSchemaTableAuthority`, `TestOrderAttemptSchemaExecutionDefinerACL`.
- §2/A1/A1.1 → `TestOrderAttemptScopeRetirement*`,
  `TestOrderAttemptAdmissionReplayPendingAndRetiredTerminal`,
  `TestOrderAttemptAdmissionReplayKeyOwnershipAndOriginalCAS`:
  fresh active identity/CAS, exact retained replay and all-state key ownership.
- §1/§2/§6a–d → `TestOrderAttemptService*`: actual runtime C1,
  frozen calculator, sealed plan, one same-Tx job, rollback and competing keys.
- §5 current-read authority →
  `TestOrderAttemptAdmissionReplayRechecksReadAuthorityAfterScopeWait`:
  native scope wait plus orders/session revocation; catalog-only revocation
  remains an allowed exact replay.
- §2/§6j → `TestOrderAttemptMaterializationFacts*`,
  `TestOrderAttemptQuoteMaterialization*`: frozen origins, CVS source,
  separate attempt quote, real buyer cart preservation and typed origin DTO.
- §1/§3.1/§6k → `TestOrderAttemptExecutionFence*`: actual login roles,
  attempt-first native waits, protected current-transaction witness,
  current-read recheck and retained terminal replay.
- §1/§3 all-or-nothing C2 → `TestOrderAttemptExecutionAtomicBoundary*`,
  `TestOrderAttemptCommitGuard*`: native COMMIT refuses a PENDING/ABORTED
  cart, fake order binding or old execution witness; lock-only PENDING stays legal.
- §2 frozen monetary facts → `TestOrderAttemptExecutionArithmetic*` verifies
  the accepted calculation using the original pricing algorithm and ordered lines.
- §2/§6j frozen destination and no browser side effects →
  `TestOrderAttemptDestination*`: home/CVS native rows, immutable source facts,
  enabled service barrier, private helper ACL and attempt namespace isolation.
- Historical checkpoint (superseded by the later CI receipts below): remaining
  §6 matrix and mutation proofs were NOT_RUN / implementation pending;
  planned mapping is in the unit brief. No schema-only or MODEL_ONLY result
  is described as REAL_PG placement acceptance.

## CI gates and open work

Focused OA model/fuzz and REAL_PG matrix; all affected foundation shards;
full migration upgrade/count/ACL pins; contract drift and repository structural
gates; process wiring and affected browser modes; independent money/privacy
review. Exact migration numbers will be rechecked against trunk before PR.
Local notification planning and erasure lock prefix are prerequisites of C2;
provider dispatch and public cutover remain dependent units.

CI49 `38461389382` at `f07cfefdd` reached native behavior:
`PASS=20 FAIL=5 SKIP=0 exit=1`, job `115517283149`,
`full-c2-ci49.log`. Actual runtime and worker full C2 without notification,
claims, lost-response replay, concurrent executor fencing, transient locks,
definitive stock abort, terminal custody and settled receipt groups pass.
Two expiry failures authorize the 0190 compatibility fix: actual bank expiry
remains retryable, and expiry fails to wait on the canonical order lock.
READY through the real Go executor fails native `lcn_emit` transaction
provenance; Go savepoints assign the inserted River row a subtransaction xid.
The fix removes those savepoints; an exact definitive refusal rolls back C2
fully before a subsequent fenced absence proof, as permitted by §1. Any late
notification reclassification rolls back the complete C2 and retains PENDING.
The direct READY fixture also needed schema qualification of its deferred
constraint name; its 23514 tamper assertion and all positive counts are intact.

CI48 `38461005406` at `c57e09eaa`, job `115515972477`, is a causal
API startup RED: the same-physical-database positive control passes, while
all three cloned-database cases keep listening instead of refusing startup.
`process-api-first-ci48.log` retains `PASS=0 FAIL=1 SKIP=0 exit=1` (one top-level
test, three negative subcases). API startup now uses the existing physical
advisory-lock probe for merchant→issuer, merchant→buyer and merchant→checkout.
The expiry producer borrows that exact checkout pool. No extra role grants.

At this historical checkpoint, the worker repair helpers in 0191/post-River0025
followed CI47's native missing-helper RED. The then-current count was 114;
0190 was reserved. The current integrated count is 118, recorded below.
Eight real-PG crash barriers were added tests-first in `cd32362f0`, with a full
combined claims/READY control. They were NOT_RUN at this checkpoint; CI50 below
supplies their native result.
All local execution remains NOT_RUN by owner directive; no complete OA1 claim.

CI50 `38462169006` is registered at `159de6e18` for the native notification,
settlement, executor (including crash matrix), sweep and real API process cases.
Root monitors its unchanged 150-minute workflow bound and focused-test budget.
0190 expiry compatibility was integrated as `74f97e277` after CI49's two causal
expiry failures. It retains the existing `expire_held` ABI and exact expiry-only
EXECUTE grant, takes the shared order advisory first, and carries the admitted
creator through SYSTEM_EXPIRY release/event rows. Upgrade count becomes 115.
The next CI run owns the result; source integration is not expiry acceptance.

CI50 `38462169006`, job `115519994332`, at `159de6e18`: **GREEN**
`PASS=31 FAIL=0 SKIP=0 exit=0` (E3 REAL_PG, scoped). The actual runtime/worker
READY paths now satisfy the unchanged native transaction-xid guard. Eight
witnessed backend terminations, complete combined claims/READY control, immutable
replay, API physical-DB negatives, all native sweep repair/ACL/lock cases and the
real startup/30-second periodic orphan recovery pass. Exact log:
`full-c2-sweep-crash-process-ci50.log`.

CI51 `38462734896` at `2a8f3052a` runs expiry compatibility and existing expiry
regressions. CI52 `38462848122` at `12625d15c` runs the independent model with a
30-second `testing.F` fuzz campaign, generated metadata evidence, and `shard:unit`
(including strict keyring loading). These are registered, not yet accepted.
Root monitors both runs; unchanged workflow bounds. The temporary authoring
steps produce evidence without changing the source used by ordinary gates and
will be removed before the PR.

CI51 `38462734896` at `2a8f3052a`, job `115521922484`: expiry **GREEN**, native
`PASS=4 FAIL=0 SKIP=0 exit=0`, including the actual expiry worker, canonical
order-lock barrier and unchanged storefront provenance controls.
`expiry-compat-green-ci51.log` is the receipt.

CI52 `38462848122` at `12625d15c`: model job `115522330047` **GREEN**, fixed
`PASS=2 FAIL=0 SKIP=0 exit=0`, plus the actual 30-second `testing.F` campaign:
186047 executions, seven corpus entries, no failure. This is **MODEL_ONLY**,
not transaction proof. `shard:unit` job `115522330075` also completed GREEN;
it includes the strict plan/pay key configuration tests. Logs are
`model-fuzz-ci52.log` and `unit-key-config-green-ci52.log`.

CI53 `38463120346` at `a21718187`, job `115523242905`: actual claims-worker
startup **RED**, `PASS=0 FAIL=1 SKIP=0 exit=1`. The positive physical-database
control passes; each of the three same-name independent clusters keeps running
beyond the existing 14-second refusal envelope. The worker compared only
`current_database()`. `8b90761a2` uses `platform.ValidateSameDatabase` for the
existing worker→intake/retention/store-verification pairs. The shared worker
pool remains the exact River producer/consumer pool. CI57 `38464543045` owns
the subsequent result. Red log: `worker-probe-first-ci53.log`.

CI54 `38464089312` at `cce0834b9`, job `115526550914`: **GREEN**
`PASS=3 FAIL=0 SKIP=0 exit=0`. Actual complete C2 honors frozen unit/live price,
policy, shipping and COD fees, preserves separate origins and the locked buyer
cart; disabled SKU/service aborts leave zero effects. Real COMMITTED/ABORTED
receipts are non-disclosing across principals and stable after relogin, a newer
generation and scope retirement. `frozen-identity-green-ci54.log`.

CI55 `38464375959` at `f957aafbf`, job `115527522821`: `PASS=3 FAIL=1 SKIP=0
exit=1`. Native wire faults positively witness successful PG COMMIT and drop
its acknowledgement for C1, C2 and the terminal recovery replay; all preserve
one complete order and unchanged credential. Current notification refusals
also pass the full worker write-set oracle. The late grant-revocation barrier
proves complete rollback and PENDING custody, then fails because
`notification_reprepare` is exposed as `order_attempt_conflict`. Fix
`73c745928` maps this one known rollback disposition to retryable, without
broadening abort rules. CI58 `38464813911` verifies it. Log:
`ack-resume-first-ci55.log`.

CI56 `38464446087` registers opt-in worker configuration tests FIRST on
`f957aafbf`; root monitors it before accepting composition changes. Native
product mutations at `ff84da1c1` are separately dispatched: M1 fee-version
`38464950233`, M5 principal disclosure `38464954147`, M3 disabled sweep
`38464957914`. The real consumer/inline race and first SQL-fence tests are
FIRST in `ac9b51b8d`: normal CI `38465027466`, M6 lock-removal CI
`38465032646`. These runs retain the unchanged test and workflow budgets;
root owns monitoring. Injection records exact source SHA and before/after
hashes in the CI artifact and never applies to ordinary PR runs.

Current status remains **INCOMPLETE**: worker composition, cancellation/privacy
compatibility, remaining contract matrix, product mutation restored-green
proof and final full gates/independent review are still required. No local
test/build/formatter/server/container was run. OA2/OA3 public activation remains
disabled. Trunk `256e9740` is merged; root migration maximum remains 0169,
while this unit currently adds 0170–0191 and post-River0023–0025 (upgrade115).

## A2 and native acceptance continuation

Owner approved DB-enforced plan-key retirement on 2026-10-14. Contract amendment
A2 is `71907ac74`: dedicated admin login/role with one EXECUTE entry, permanent
version marker, C1 key-row-before-attempt locking, pending-reference refusal,
and independent unmarked PLAN_UNAVAILABLE cleanup. Its implementation and
A2-M1 mutation require independent review with this PR. Provisional slots 0192
(erasure prefix), 0193 (native cancellation), 0194 (A2) are separate child-owned
changes; final numbering is checked against trunk before the PR.

| Run / source | Result and exact scope |
| --- | --- |
| CI56 `38464446087` / `f957aafbf` | **RED**: all three actual worker configuration tests fail (off flag unrecognized, on flag ignored, incomplete custody accepted). `worker-config-first-ci56.log`. Composition `6e7de3753` follows this red; green run `38466720785` pending. |
| CI57 `38464543045` / `8b90761a2` | **GREEN** `PASS=1 FAIL=0 SKIP=0 exit=0`: actual worker physical-DB positive and three independent same-name-cluster refusals. `worker-probe-green-ci57.log`. |
| CI58 `38464813911` / `73c745928` | **GREEN** `PASS=4 FAIL=0 SKIP=0 exit=0`: real C1/C2/recovery COMMIT acknowledgement losses and native late message-revocation continuation. `ack-resume-green-ci58.log`. |
| CI59 `38464950233` / `ff84da1c1` | M1 **RED**, `PASS=0 FAIL=1`: current-fee-version refusal breaks frozen-money execution. `mutation-m1-ci59.log`. |
| CI60 `38464954147` / `ff84da1c1` | M5 **RED**, `PASS=0 FAIL=1`: removing principal checks discloses terminal receipt. `mutation-m5-ci60.log`. |
| CI61 `38464957914` / `ff84da1c1` | M3 **RED**, `PASS=0 FAIL=1`: real startup/periodic consumer cannot repair its orphan after Sweep is disabled. `mutation-m3-ci61.log`. |
| CI62 `38465027466` / `ac9b51b8d` | `PASS=1 FAIL=1`: all four actual-login first-fence/stale-generation barriers pass. Consumer race stops at nullable River errors readback, before its oracle; **NOT_RUN** for that behavior. `ccb50a9d2` counts NULL error history as zero with unchanged exact one-attempt/zero-error assertions. `worker-race-first-ci62.log`. |
| CI63 `38465032646` / `ac9b51b8d` | M6 **RED**, `PASS=0 FAIL=1`: all four SQL fence barriers return before a held FOR NO KEY UPDATE lock when the fence FOR UPDATE is removed. `mutation-m6-ci63.log`. |
| CI64 `38465241103` / `81f98d8e2` | **RED** `PASS=0 FAIL=3`: pickup/COD cancel unavailable, unpaid bank cancel refused; native erasure neither seals PENDING nor locks attempt before owner. `cancel-erasure-first-ci64.log`. |

Registered and root-monitored, **not yet accepted**: genuine unchanged 15-minute
C1 deadline through actual periodic River (`38465747856` @`5ef841656`);
restored M1/M3/M5/M6 plus real consumer race (`38465859673` @`5ef841656`);
pending-only replay mutation (`38466883637` @`6e7de3753`);
generation CAS/unique and partial Begin-commit mutations (`38467152494`,
`38467159295`, `38467163852` @`9e648aff0`); actual packaged enabled worker
(`38467298823` @`6a2f0cea9`). Existing workflow and per-test bounds are unchanged.
GitHub job-status calls intermittently returned HTTP 502; those are monitoring
failures, not product test verdicts. No local run substitutes for CI.

## Native deadline, composition and mutation receipts (continuation)

| Run / source / job | Result and exact scope |
| --- | --- |
| CI65 `38465747856` / `5ef841656` / `115532316750` | **GREEN** `PASS=1 FAIL=0 SKIP=0 exit=0`; `TestOrderAttemptDeadlineRealClaimPeriodicLiveness` takes 905.28s across the unchanged real 15-minute admission deadline. The actual periodic consumer releases native claims/reservations and permits the next generation. This case deliberately suppresses placement; the separate C2-versus-expiry race remains pending. `deadline-native-green-ci65.log`. |
| CI68 `38466720785` / `6e7de3753` / `115535984686` | **GREEN** unit shard, including the three actual worker configuration cases that failed in CI56. Correct off/on behavior and strict key custody now pass. This alone does not prove the packaged worker consumes an order. `worker-composition-green-ci68.log`. |
| CI69 `38466883637` / `6e7de3753` / `115535991062` | Pending-only transport lookup mutation **RED**, `PASS=0 FAIL=1 exit=1`: native terminal replay no longer follows the retained authority. `pending-only-replay-mutation-ci69.log`. |
| CI70 `38467152494` / `9e648aff0` / `115537014792` | Omitted generation CAS mutation **RED**, `PASS=0 FAIL=1 exit=1`: the required typed stale-generation refusal becomes an unavailable error. `generation-cas-mutation-ci70.log`. |
| CI71 `38467159295` / `9e648aff0` / `115537015335` | Omitted all-state generation uniqueness **RED**, `PASS=0 FAIL=1 exit=1`: the exact durable-schema invariant fails. `generation-unique-mutation-ci71.log`. |
| CI72 `38467163852` / `9e648aff0` / `115537374698` | Disabled atomic COMMIT guard **RED**, `PASS=0 FAIL=1 exit=1`: both actual executor logins commit a Begin-only partial pipeline when the deferred guard is removed. `partial-commit-mutation-ci72.log`. |
| CI73 `38467298823` / `6a2f0cea9` / `115540242595` | `PASS=0 FAIL=1 exit=1`; actual enabled worker commits, but its receipt fails the queued-notification oracle. Static inspection found the isolated fixture omitted the native published storefront origin needed for READY. `4fe3f5eea` adds the existing `bhPublish` seam plus a positive active/public/unexpired-proof sentinel; every READY assertion remains. CI81 owns verification; this is not a product PASS. `enabled-worker-first-ci73.log`. |
| CI75 `38468199865` / `7d655349f` / `115540614733` | A2 tests FIRST **RED**, `PASS=0 FAIL=5 exit=1`: after native C1 prerequisites, the dedicated admin role and version-bound eight-argument admission entry are absent (42704/42883). Concurrency behavior remains NOT_RUN until implementation and A2-M1 calibration. `key-retirement-first-ci75.log`. |
| CI76 `38468285275` / `b83c6c66e` / `115541778243` | Erasure-prefix authority FIRST **RED**, `PASS=0 FAIL=1 SKIP=0 exit=1`: the three exact private erasure entries are absent (42883). `erasure-prefix-first-ci76.log`. |

Root-monitored registrations, not accepted results: real River/inline consumer
race CI74 `38467528473`; native cancellation plus legacy regressions CI77
`38468325801`; erasure/C1 race FIRST CI78 `38468487041`; CVS crash/key-rotation/
terminal-CAS matrix CI79 `38468605984`; erasure product and legacy privacy CI80
`38468649299`; enabled process CI81 `38469036123`; genuine C2-versus-15-minute
deadline CI82 `38469249744`; owner-derivation mutation CI83 `38469282415`; final
deadline-check mutation CI84 `38469287812`. All run on isolated GitHub runners
with unchanged per-case and workflow budgets; root reads their jobs/logs.

Historical pre-A2 checkpoint: 0192 erasure lock-prefix and 0193 native cancel
compatibility used upgrade pin 117. This is superseded by integrated 0194 and
pin 118 below. The admin
credential follows the existing external-operator provisioning pattern: it is
not a deployed core login or a secret mounted into API/claims-worker. Final
numbering, metadata, whole-unit gates and independent review remain required.

## A2 implementation and current acceptance receipts

A2 is integrated in `54c1d8d2c` (0194), with the exact upgrade pin at 118 in
`d806d7745`. Fresh C1 supplies and locks its chosen key version before the
subject/scope/attempt prefix; its finish cannot substitute another version.
Retirement uses the same row lock and global pending-reference check, through
the isolated admin login's sole definer. Exact terminal replay and unmarked
PLAN_UNAVAILABLE settlement retain their independent paths. The admin credential
is an external operator credential, never a core deployment/worker secret.
The runbook and an injected-custody regression are in `a75a15f78`.

| CI / source / job | Observed result |
| --- | --- |
| 74 `38467528473` / `d08d72741` / `115540476298` | **GREEN** `PASS=1 FAIL=0 exit=0`: actual River consumer versus inline executor; `worker-consumer-race-green-ci74.log`. |
| 77 `38468325801` / `a3e005c6b` / `115542054097` | **GREEN** `PASS=17 FAIL=0 exit=0`: native attempt cancellation and existing cancellation/returns/CVS regressions; `native-cancel-green-ci77.log`. |
| 78 `38468487041` / `b718fe476` / `115542098647` | **RED** `PASS=0 FAIL=1 exit=1`: C1 reaches owner/refusal without the required erasure subject barrier; `erasure-c1-race-first-ci78.log`. |
| 79 `38468605984` / `f922272dd` / `115543720389` | **GREEN** `PASS=3 FAIL=0 exit=0`: native CVS crash, actual configured-key rotation and terminal generation CAS; `native-matrix-green-ci79.log`. |
| 80 `38468649299` / `b2b58d998` / `115542921233` | `PASS=6 FAIL=2 exit=1`: erasure settlement/authority cases pass; native post-erasure C1 returns unavailable instead of ERASED, and the historical upgrade fixture applies OA before its held 0129 dependency. `erasure-prefix-ci80.log`. |
| 81 `38469036123` / `4fe3f5eea` / `115546036519` | **GREEN** `PASS=1 FAIL=0 exit=0`: packaged worker enabled/native READY, disabled non-consumption and malformed enabled custody rejection; `native-worker-process-green-ci81.log`. |
| 83 `38469282415` / `422d4a355` / `115547012797` | Owner-rederivation mutation **RED**, `PASS=0 FAIL=1 exit=1`: configured key rotation causes actual C2 `order_attempt_conflict` at matrix test line 143; `owner-rotation-mutation-ci83.log`. |

CI80 supplies causal native red for `73d95f008`: only exact PT422/ERASED maps
to ERASED, while every other business/unknown error retains its classification.
The additional 11-case unit counterexample was committed first (`435448289`)
and dispatched separately as CI92. Historical upgrade still applies every real
OA migration/checksum on populated data; it now holds the dependent suffix until
the native reservation table exists. No historical assertion is removed.

Pending root-monitored CI registrations (not PASS): 82 `38469249744` genuine
15-minute C2 deadline; 84 `38469287812` final-check mutation; 85 `38470325208`
A2 normal; 86 `38470330821` A2-M1; 87 `38470580829` model/fuzz, generated
metadata, backend-classification acceptance and normal admin custody; 88
`38470585873` injected admin custody; 89 `38470591029` omitted cancellation
revocation; 90 `38470594970` body shredding before settlement; 91 `38470600332`
rejected CANCELLED replay; 92 `38471182736` exact ERASED unit FIRST. Existing
workflow/per-case deadlines are unchanged. All local runtime/tests remain
**NOT_RUN (owner CI-only directive)**. Whole-unit acceptance and independent
review remain incomplete.

## A2 and matrix receipts through CI96

| CI run / source / job | Exact result and evidence |
| --- | --- |
| 82 `38469249744` / `bb3dba04a` / `115546788286` | **GREEN**, `PASS=2 FAIL=0 SKIP=0 exit=0`. Native C2 finishes before the unchanged deadline, or crosses the real 15-minute clock (904.95 s) and rolls back. `deadline-competition-green-ci82.log`. |
| 85 `38470325208` / `d806d7745`; 86 `38470330821` / `d806d7745`; 89 `38470591029` / `8161d6792` | 0194 setup fails with `0A000`: a policy cannot depend on a shared `regrole` constant. Behavioral **NOT_RUN**, including both mutations. `key-retirement-ci85.log`, `key-retirement-mutation-ci86.log`, `cancel-revocation-mutation-ci89.log`. `0efc46e81` replaces the membership predicate with the same exact-role check via `pg_roles`; authority is unchanged. |
| 87 `38470580829` / `8161d6792` / `115551643716` | External admin credential-custody **GREEN**, `PASS=1 FAIL=0 exit=0`; `key-admin-custody-green-ci87.log`. Separate model job `115551643813`: 30-second fuzz **MODEL_ONLY GREEN** (74,005 executions, seven corpus entries); backend classification **RED**, 74/78 node cases pass and four identify the new unclassified package. `56cb13ffc` adds the shared executor to the existing conservative shared-backend classification, preserving broad coverage. `model-classification-ci87.log`. |
| 88 `38470585873` / `8161d6792` / `115551827528` | **Causal RED**, `PASS=0 FAIL=1 exit=1`: injecting the admin login into the core deployment list violates the isolated credential-custody assertions. `key-admin-custody-mutation-ci88.log`. |
| 92 `38471182736` / `435448289` / `115552043574` | Test-FIRST unit **RED** at `TestAdmissionFailureExactErasedOwner`: native `PT422/ERASED` is mapped to unavailable. Other real-PG packages stop at the same 0194 setup problem and are not behavior proof. `erased-error-unit-red-ci92.log`. The mapping repair is `73d95f008`; the migration setup repair is `0efc46e81`. |
| 95 `38472123118` / `0efc46e81` / `115553705187` | A2/native admission **GREEN**, `PASS=17 FAIL=0 SKIP=0 exit=0`: exact admin grants, both lock race orders, cross-store PENDING references, admitted-version binding, retained replay and permanent unmarked key-loss cleanup. `key-retirement-green-ci95.log`. |
| 96 `38472127865` / `0efc46e81` / `115553752820` | A2-M1 **causal RED**, `PASS=0 FAIL=1 SKIP=0 exit=1`: removing C1's in-lock retirement check fails the fixed `PT409/plan_key_retired` refusal assertion. `key-retirement-mutation-ci96.log`. |

Root canceled its own CI90, CI91 and CI94 after confirming their source had
the same setup failure; these are **NOT_RUN**, not causal mutation results.
CI93 was canceled before acceptance to avoid replacing the pending test-FIRST
unit run. No other lane's run or local process was stopped.

Current-source registrations on `008b52dec` (root-monitored, **pending**, no
PASS claim): CI97 `38472994627` C1 crash/cross-principal barriers plus model,
fuzz, formatter and generated metadata; CI98 `38473112949` full OA1 focused
suite and legacy privacy/upgrade; CI99 `38473118535` M2b; CI100 `38473121014`
erase-before-seal; CI101 `38473126804` reject-CANCELLED; CI102 `38473180467`
unit restoration; CI103 `38473185280` finalize-from-Begin; CI104 `38473190215`
expose quote helper; CI105 `38473195477` resume through placement gates;
CI106 `38473201544` merge claim origins. Existing workflow and per-test bounds
remain unchanged. CI alone applies mutations to disposable checkouts; the
working branch retains the original assertions and production code.

## Current matrix and mutation results

| Run / source / job | Result |
| --- | --- |
| CI84 `38469287812` / `422d4a355` / `115547129468` | Removed final-deadline check **causal RED**, `PASS=0 FAIL=1 exit=1`, after the real 907.30-second crossing. `final-deadline-mutation-ci84.log`; normal CI82 retains the original deadline assertions. |
| CI97 `38472994627` / `008b52dec` / `115556684233` | Native C1 precommit termination and independent same-buyer carts **PASS**. `PASS=2 FAIL=1`: the third test incorrectly expected the pending bundle code after C2 had exhausted its claim quantity. `27a20b227` keeps the exact pending refusal, adds a positive remaining=0/placed=1/uses=1 sentinel, then asserts the exact settled internal conflict. No product mapping changes or public OA3 error-code claim. `c1-crossclient-first-ci97.log`. |
| CI97 / `008b52dec` / `115556684272` | **GREEN** model seeds `PASS=2 FAIL=0 exit=0`; timed fuzz 305,937 executions/nine corpus entries (MODEL_ONLY); backend/pr-modes Node 78/78; formatter exit 0 with an empty patch. `model-classification-green-ci97.log`. CI-produced dependency map and shard plan applied as artifacts; temporary authoring steps removed in `728db61e6`. |
| CI98 `38473112949` / `008b52dec` / `115557144033` | Privacy/upgrade `PASS=2 FAIL=2 exit=1`: exact migration pin omitted three post-River files; historical fixture also failed to hold those dependent files. `privacy-upgrade-ci98.log`. `9dd635f1c` adds one exact comment/pin increment per file: **121**, not 118. Fixture repair and restoration CI follow; all checksum/upgrade assertions remain. |
| CI99 `38473118535` / `008b52dec` / `115557138716` | M2b **causal RED**, `PASS=0 FAIL=1`: native cancel no longer revokes the credential, with unchanged version/material/lifetime oracle. `cancel-revocation-mutation-ci99.log`. |
| CI100 `38473121014` / `008b52dec` / `115557160731` | Erase-before-seal **causal RED**, `PASS=0 FAIL=1`: native erasure cannot settle the pending attempt. `erase-before-seal-mutation-ci100.log`. |
| CI101 `38473126804` / `008b52dec` / `115557159688` | Reject-CANCELLED **causal RED**, `PASS=0 FAIL=1`: all three native cancelled commercial states fail their terminal replay. `cancelled-replay-mutation-ci101.log`. |
| CI103 `38473185280` / `008b52dec` / `115557348063` | Finalize-from-Begin **causal RED**, `PASS=0 FAIL=1`: incomplete C2 cannot commit through either actual executor login. `begin-finalization-mutation-ci103.log`. |
| CI104 `38473190215` / `008b52dec` / `115557388173` | Exposed internal quote helper **causal RED**, `PASS=0 FAIL=1`: exact ACL oracle rejects the runtime grant. `quote-authority-mutation-ci104.log`. |
| CI105 `38473195477` / `008b52dec` / `115557366344` | Resume via placement permissions **causal RED**, `PASS=0 FAIL=1`: all three durable continuation cases fail with forbidden. `placement-resume-mutation-ci105.log`. |
| CI106 `38473201544` / `008b52dec` / `115557450204` | Merge claim origins **causal RED**, `PASS=0 FAIL=1`: native frozen money/origin/stock readback rejects the collapsed SKU lines. `merged-origins-mutation-ci106.log`. |

Fresh fetch before PR preparation confirms trunk `256e9740`, root maximum
0169 and post-River maximum 0022. This unit uses root **0170–0194** and
post-River **0023–0025**. The public route remains disabled; final whole-unit
CI and independent review are still pending. All local runtime execution is
**NOT_RUN (owner CI-only directive)**.
