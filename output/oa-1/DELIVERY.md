# OA-1 delivery — in progress

- Branch: `unit/oa-1-order-attempt-core`; base: `55e98ba0434557e94e3b7c9e31ff0e98352623b1`.
- Implementer: Codex-3; reasoning: high. Allowed paths and boundaries:
  `docs/delivery/units/oa-1-order-attempt-core.md`.
- Contract: frozen order-attempt-v1 plus reviewed A1/A1.1 at
  `cb4256f37586aad5139f1216fc92d6226fa70003`, merged in `0f1060a72`.
  Retired scopes reject fresh CAS, but preserve authorized exact replay during
  retention. Backend separate from paused PR #32.
- Current artifact: storage, private admission helpers, C1 SQL coordinator and
  Go orchestration; C1 is under CI review. C2 execution remains incomplete;
  the new placement path is not enabled.

## Evidence

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
- Remaining §6 matrix and mutation proofs: NOT_RUN / implementation pending;
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

The worker repair helpers in 0191/post-River0025 follow CI47's native missing
helper RED. The exact upgrade count is 114; reserved 0190 is not yet present.
Eight real-PG crash barriers were added tests-first in `cd32362f0`, with a full
combined claims/READY control. They remain NOT_RUN until the next CI result.
All local execution remains NOT_RUN by owner directive; no complete OA1 claim.

CI50 `38462169006` is registered at `159de6e18` for the native notification,
settlement, executor (including crash matrix), sweep and real API process cases.
Root monitors its unchanged 150-minute workflow bound and focused-test budget.
0190 expiry compatibility was integrated as `74f97e277` after CI49's two causal
expiry failures. It retains the existing `expire_held` ABI and exact expiry-only
EXECUTE grant, takes the shared order advisory first, and carries the admitted
creator through SYSTEM_EXPIRY release/event rows. Upgrade count becomes 115.
The next CI run owns the result; source integration is not expiry acceptance.
