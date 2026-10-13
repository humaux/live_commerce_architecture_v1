# LC-U3 create-order drawer delivery

## WIP checkpoint — owner state-machine freeze (2026-10-13)

**WIP / NOT READY. Attempt, recovery and pay-link implementation is paused by the owner pending the integrator's frozen state-machine contract and table-driven test matrix.** Existing source was committed and pushed at `dd349c3c8e4a1b6b4ff2cec296bb9b6dd2951bd4`; this checkpoint changes only DELIVERY. No new fix was started after the stop instruction, including the newly reported privacy, erasure, prefill-completeness or restriction-header findings. There are no local tests, builds, browsers, dev servers or PG runs in this checkpoint.

Existing work: durable principal/store pending fence, original owner binding, settled transport-key replay, frozen-request partial completion, guarded link disclosure, CANCELLED rendering, linked email prefill, search-state reset, native strip-only constructors and exact migration pins are committed but not accepted as a complete system. The latest batch also fixes success-to503 mapping and premature historical settlement, filters superseded/insufficient claim origins, and repairs test fixtures without reducing exact money/auth/privacy assertions. Independent static review of the latest delta found no concrete finding (E1 only). These implementations remain candidates for the frozen contract; no thread is claimed fixed by this WIP handoff.

Known evidence:65404c07 focused38403107942 failed (PASS45 / FAIL17 / exit1), Foundation38403053706 failed three drawer fixture cases, and full required38403054381 was cancelled for the correction batch. g10 reservation/typed-pending cases passed in that focused run; unchanged card job115315897105 passed, so the eb5 card failure did not reproduce, without establishing its cause. At the pause, source-dd349c3c8 Foundation38407020021, required38407020062 and focused+card38407018681 were queued; they are not PASS. A DELIVERY-only successor triggers fresh automatic current-head runs; completed earlier evidence applies only to its stated source. All local acceptance remains NOT_RUN(owner CI-only).

Still-open inventory (implemented but unverified findings stay open):

| Theme | Findings awaiting the frozen contract or CI acceptance |
|---|---|
| A — attempt/recovery authority |4257479294 cross-tab/re-login fence;4257479276 aged Begin-only completion;4258064775 original owner across key rotation;4258686743 lost recovery response/settled replay;4257479307 never-sent preflight;4258686722 recovered CANCELLED terminal state.|
| A — erasure/privacy |4259170302 erased pending body strands the principal/store fence; g02 frozen D-S2 privacy-writer SELECT excess; TestAdsAttributionR3Erasure PT404. New findings are unmodified. Existing schema test currently expects the extra grants/UNKNOWN after erasure and must be redesigned against the frozen contract, not used to override the frozen list.|
| B — pay-link/DM validity |4257479285 lock-established/window_closed disclosure;4256912975 superseded Place token;4256890998 exact reminder order identity/two-buyer batch;4257755379 link_unavailable copy/reminder state. Existing fixes require consolidated current acceptance.|
| Prefill/completeness |4258064767 duplicate-SKU origin eligibility/catalog warning;4259170307 A15 LIMIT50 truncation without completeness signal. The latter is unmodified.|
| Isolated drawer UI/read seams |4258064753 picker search busy state;4259170294 restriction success no-store header (unmodified); review summary linked-customer latest-order email prefill. These are the only owner-permitted follow-up scopes before the contract, and no new work starts in this checkpoint.|
| Consolidated CI regressions |g07 TestLiveConsoleOrderForBuyerGrantsDoNotOutliveRequest/P2-3: latest eligible-origin and precise unused-grant fixture fix is unverified; g10 typed409/pending reservation currently passed at654, successor pending; Foundation strip-only import/fixture corrections require successor acceptance; R2 exact0171/0172 pin96 needs current CI.|
| Trunk/other gate investigation |manual_fulfilment_guards shipments.go call-site diagnostic: both internal/merchantorders/shipments.go and internal/httpapi/shipments.go have no PR32 diff from origin/r3/integration, so this unit did not introduce that missing comment. Card primary readOrder failure is not reproduced on unchanged rerun and has no response trace; no assertion was relaxed.|

No GitHub thread was resolved or replied to as fixed by this checkpoint. Previously integrator-resolved threads remain resolved. The pre-existing LC-B6 queued-DM-after-cancel finding stays in its separate unit. Migration0171/0172 and every failed artifact remain preserved. Await the owner contract; do not resume A/B patches from a heartbeat or a CI failure notification.

## PR32 r6 — server intent authority and settled recovery (2026-10-13)

**E1 source assembled; current acceptance PENDING, not READY.** Source head before this delivery update: `1d0fd9f03`. Refetched integration `0bfe36e53acb694f4697521202e24f0790638941` (already an ancestor). All tests, builds, browser modes and gates run on GitHub CI under the owner's 2026-10-13 directive; no local acceptance command was run. Earlier sections are historical receipts and do not accept this new source.

- Implementer Codex-3; runtime model/effort NOT_EXPOSED. Isolated backend/UI/test writers and read-only reviewers used explicit GPT-6.1-sol high/medium where exposed. Main branch/worktree: `unit/lc-u3-create-order-drawer`, `.worktrees/lc-u3-create-order-drawer`. Scope expanded by owner approval to the necessary backend contract and migration repair.
- Write paths: `apps/admin/components/{CreateOrderDrawer,ManualOrder,ManualOrderItemPicker}.tsx`; `apps/admin/lib/{create-order-attempt,create-order-client,create-order-model,create-order-copy,merchant-tools-client,merchant-tools-model,customers-client}.ts`; `apps/admin/app/api/stores/[store]/tools/[...resource]/route.ts`; `internal/merchanttools/{order_attempt,order_intent,order_intent_recovery,manual_recovery,order_for_buyer,manual}.go`; `internal/httpapi/merchanttools*.go`; `internal/httperror/{error.go,order_intent_test.go}`; `migrations/0172_order_intent_authority.sql`; `contracts/{live-console-v1,storefront-v2}.md`; corresponding `tests/admin/create-order*` / `manual-order-recovery.test.ts`, drawer/manual Go browser fixtures, and foundation intent/prefill/schema/retention/R2 tests; this DELIVERY and `r6-ci/` receipts. Owner `output/ext-agents/**` is preserved. Probe workflow diagnostics exist only on `codex/pr32-*-red` branches and are not merged into this PR.
- Migration numbers: existing `0171` reminder identity plus new reserved `0172` durable attempt authority. Latest fetched integration max is `0169`; R2's exact migration count now has one explicit comment and increment for each of 0171 and 0172 (`94 → 95 → 96`), without relaxing equality. No dependency or lockfile change.
- Contracts: live-console-v1 §5.1.2 and storefront-v2 G3 amendment. A new scoped generic recovery BFF/Go seam accepts exactly `{}` and an explicit transport key. Settled intent rows retain original owner/key/hash/order authority after private input erasure. The existing runtime receives narrowly scoped table/column and function grants; SECURITY DEFINER owners, `search_path`, PUBLIC revocation, exact ACL pins and privacy cleanup are retained.

Review changes (source, pending current CI):

| Thread | Root cause and repair |
|---|---|
| 4257479294 | Browser flags could not fence another tab/login. Commit one principal/store pending intent before money; reject every other key server-side. Client storage is UX only. |
| 4257479276 | Begin-only repair depended on the old source window. Original durable owner/Begin proof now permits completion of the frozen pipeline at any age; expiry never revives holds, links or DM. |
| 4257479307 | Pre-fetch session/CSRF refusal was classified as UNKNOWN. Explicit never-sent classification disarms only a fresh undispatched attempt; earlier UNKNOWN keeps its fence. |
| 4257479285 / 4256912975 | Unestablished or superseded tokens escaped. Establish exact validity under the order lock; suppress links on window_closed, competing reminders, cancellation and every unestablished path. |
| 4256890998 | Reminder plans lacked order identity. Prior 0171 exact order/bundle/journal guard remains; no store-wide legacy fallback. |
| 4257755379 | Copy appeared for link_unavailable. Hide it and show reminder-in-flight; no regeneration can supersede a planned DM link. |
| 4258064775 | Recovery rederived a private owner after secret rotation. Persist the original owner and recover only against its exact Begin; ambiguous legacy binding stays UNKNOWN. |
| 4258064767 | Merged duplicate-SKU claim origins silently selected one live price. Preserve quantity while making every ambiguous merged origin catalog-only, with a visible three-locale pre-submit warning; no client pricing. |
| 4258064753 | Search abort during picker expansion stranded its busy state. Reset the search state at expansion; actual component/client tests operate the controls. |
| 4258686743 | A lost created recovery response became none. Persist recovery transport-key→original intent before effects; replay terminal state from exact upper/closure proofs. Clients retain that transport key across ambiguity. |
| 4258686722 | CANCELLED recovered offline results were rejected. Strict null-link CANCELLED parsing and localized terminal rendering disable collection/link/payment controls. |
| Review summary, email | Explicit linked customer latest-order email was dropped. Read through a narrow definer requiring both orders:read and customers:read; recheck permissions before publishing optional contact. |

Independent source review identified two additional edge cases: waiting for the payment-link/order lock could outlive authority, so the 0172 definers recheck current scope after the wait; a committed mixed-price quote replay lost its catalog warning, so a scoped Boolean helper reads immutable original Quote provenance. Neither current claims nor client calculations reprice it.

CI red evidence (native logs/artifacts; queued runs are not yet proof):

| Run | Source / result |
|---|---|
| [38393555698](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38393555698) | Unchanged pre-fix product 05ac4f75: REAL_PG aged partial, cross-session fresh key, secret rotation and stale link targets, PASS0 FAIL4 SKIP0 exit1. |
| [38396454675](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38396454675) | Actual full-router and origin tests plus UI copy, warning, picker and guard seams fail on unchanged product. Native client import initially failed before preflight assertions; that failure is not preflight proof. |
| [38398217460](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38398217460) | Corrected probe executes actual client graph: changed/missing CSRF, async rotation and fresh pre-dispatch guards fail natively on unchanged product. Probe-only transform flag permits the old constructor; production test-node stays strip-only. |
| [38399265598](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38399265598) | Before post-lock recheck: all six true PG wait/revoke cases accept invalid authority; PASS0 FAIL1 SKIP0 exit1, job115303389094. |
| [38399874208](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38399874208) | INVALID causal proof: fixture queried nonexistent command_results.order_id; target assertion not reached. Corrected probe3731ef52 runs in38405421601 against the same pre-fix product. |
| [38401137148](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38401137148) | Native186 tests: PASS147 FAIL39. CANCELLED parsing/rendering, native-strip constructor and same-key generic component cases fail. Go step SKIPPED. Merged-warning component cases fail in fixture render loop before assertions; queued/copy nine cases already pass at this probe and are not causal RED. |
| [38401462373](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38401462373) | e72fa444, job115315601033: PASS0 FAIL2 exit1. Lose committed recovery HTTP response; replay incorrectly returns none instead of the same created/sealed target. |
| [38402253004](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38402253004) | ce9b0fb70, job115315759226: PASS0 FAIL2 exit1; authorized latest email is blank and public pending refusal becomes internal. |
| [38402626357](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38402626357) | 731f4314b, job115315528934: PASS1 FAIL1 exit1; exact fresh-key pending HTTP refusal is internal, not attempt_pending. |

CI regression triage at eb5f7e00:

- Foundation38396909571: native strip-types reached ReadError's parameter property. Explicit fields now fix ReadError and both attempt/guard constructors; the suite keeps its original strip-only command.
- Focused38396904363: PASS2 FAIL2 SKIP0 exit1; real aged partial/window_closed targets passed. Two sealed fixtures incorrectly assumed the seeded store had zero checkout orders. Exact before/after baseline comparisons replace that assumption; no money/stock/DM equality is relaxed. The original focused regex prefixes did not select all authority test names; final focused selection uses their actual names.
- g10 job115298159472: attempt_pending was missing from the public error table, hence 409 internal. Register the typed code; isolate each reservation crash subtest and use another authorized principal for the original bundle-protection probe, retaining exact bundle_already_ordered/order ID/no-new-order assertions. The crash helper now proves its after-Place hook ran. An added same-principal HTTP case checks the exact durable fence and same-key completion.
- Card-payments job115298159245, artifact11814201567: admin and derived storefront cases passed; primary desktop en failed loading order detail after View order, before collector assertions. Prior05ac used identical direct card/storefront source and passed. Missing network response trace prevents assigning a precise backend cause or a flake verdict. Raw artifact and log are retained under `r6-ci/card-triage/`; current CI must rerun this mode unchanged.

Follow-up native CI at65404c07:

- [Focused38403107942](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38403107942), job115315897240: PASS45 FAIL17 exit1. Both g10 reservation and typed pending HTTP regressions pass. A nil success error was still fed to mapForBuyerError, turning committed created/sealed recovery into503; skip that mapping on success. Historical not_created is now settled only after original pending/grant cleanup. Exact checkout lookup inconsistency remains UNKNOWN.
- The same run's unchanged [card job115315897105](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38403107942/job/115315897105) succeeds (TestBrowserCardPayments50.42s): eb5 primary failure did not reproduce. No card assertion, source or budget changed; flake classification remains owner-owned without a missing response trace.
- [Foundation38403053706](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38403053706), job115315373188: three actual drawer fixture render loops. Stable store-scope identity fixes the fixture; all warning, quantity and no-submit assertions remain intact.
- ForBuyer now counts only non-null-priced winners covering the full quantity and remaining allowance; superseded/insufficient lines do not hide the single eligible origin, and multiple eligible origins still use catalog price. Independent refused attempts receive independent fixtures, preserving the server fence. True Begin-only recovery uses a killed request, not a created intent with manually deleted upper receipts. Link cases assert explicit null on window_closed/queued replay and real old-token404/new-token200 redemption; order/hold/grant/DM/receipt counts stay exact.
- Required38403054381 was cancelled for these source corrections; no complete-current acceptance is claimed. The successor SHA must pass Foundation, all PR-selected Gates and the complete focused suite.

CI gates: automatic Foundation (test-node, check-gates, package docs and vulnerability scan), full PR-selected Gates including all Go shards and `--browser-manual-order`, `--browser-live-console`, `--browser-inbox`, `--browser-studio-ui`, `--browser-card-payments`, plus every registry-selected browser/click/visual mode. Production Next builds provide TypeScript validation; no local tsc ran. An additional focused order-intent/for-buyer/schema/retention run and unchanged card rerun will be registered on the assembled SHA. Original budgets/assertions remain unchanged. G07/release, production, LIVE, real messages and pre-existing LC-B6 queued-DM-after-cancel changes remain outside scope.

Completion/wake: GitHub run completion and the existing quiet thread heartbeat `pr36-lc-u3` track the current SHA; no new task is created. All outstanding review replies wait for current CI evidence and use the fix SHA; no thread is resolved by this implementer. No READY claim until the current required set is green.

---

## PR32 r5 — order-scoped queued reminder links (2026-10-13)

**Focused CI acceptance PASS30/30; complete PR required modes PENDING.** Code/test HEAD **`e9f07181`**, tested via pushed **`6490e5fd9d0d1ce21a782b8f3412b3fb23afadf2`**, includes product fix **`d563301ef21f7c09cdaa9a1277fc6f6bb05b4f5d`** and fixture corrections9f96ecf7/e9f07181, based on accepted unit head **`883eb869e378edbc00d5fb0a7c5a858c7b820170`** and its merged integration **`0bfe36e53acb694f4697521202e24f0790638941`**. This section supersedes the r4 store-wide legacy-guard limitation below. Owner's 2026-10-13 rule routes all new tests/builds/browser acceptance to GitHub CI; no local acceptance command has been restarted. This delivery/evidence successor changes no product or test source.

- Implementer: Codex-3; GPT-6 family instruction, actual runtime model/effort identifiers NOT_EXPOSED. Worktree `.worktrees/lc-u3-create-order-drawer`, branch `unit/lc-u3-create-order-drawer`. Read-only mapping/review and test-only child work used separate ownership; test writer branch `codex/pr32-r5-stale-link` at base883eb869, changes cherry-picked and reviewed. No recursive delegation.
- Allowed writes in this round: `migrations/0171_checkout_reminder_order_guard.sql`; `internal/merchanttools/{manual.go,checkout_reminders.go,order_for_buyer.go}`; `internal/inbox/reminders.go`; `contracts/{live-console-v1.md,storefront-v2.md}`; `tests/foundation/{queued_order_link_batch_test.go,queued_order_link_response_test.go,queued_order_link_target_test.go,checkout_reminder_test.go,claims_retention_test.go}`; this DELIVERY and its `r5-ci/` receipts. Migration0171 was reserved in coordination. Owner `output/ext-agents/**`, migration0144, dependencies, route/BFF shapes and all UI/browser specs are untouched by this delta.
- Evidence: **E3 focused REAL_PG CI red→green, Foundation SUCCESS; full PR acceptance pending.** REAL_PG tests use actual signed intake, merchant HTTP transactions, reminder scanner/planner, River and HPKE. Only disclosed synthetic buyer/order fixture insertion and test interposition are used; dispatcher is paused and Graph call count must remain0. Meta LIVE and G07 remain NOT_RUN. Independent read-only delta review at6490 found no concrete P0/P1/P2 (E1 static review, Humauxb62a762c-c657-425d-8b9f-1fe16c3ebb31); independent runtime approval remains integrator-owned.

Thread replies / fixes:

- **4256890998:** the missing reminder order identity made a nonterminal reminder protect every order in the store. New migration0171 records authoritative `crm_bundle_facts.order_id`; same-order transaction locking calls the scoped `inbox.order_pay_link_pending` definer. Legacy missing-order reminders correlate only through immutable order origins and the exact request bundle or same-store operation journal; no latest-order guess or store-wide fallback. Each newly planned operation is also checked against the order whose link was issued before commit. A candidate change rolls back that buyer's link/operation/secret/job and reports `not_remindable`, allowing independent buyers in the batch to continue.
- **4256912975:** a real competing reminder can supersede Place's original token before payment-DM planning. When planning reports `not_sent/link_unavailable`, the response and same-key replay now return explicit `buyer_link:null`. Other not-sent reasons, including `window_closed` with a still-valid link, retain their existing behavior.

New regression tests:

| Test | Required real-seam result |
|---|---|
| `TestQueuedOrderLinkTwoBuyerReminderBatch` | Two distinct unpaid buyers both queue, with one valid order-matching link/READY operation/sealed secret/held job each; same-order regeneration refuses while an unrelated order succeeds. |
| `TestQueuedOrderLinkLegacyReminderScope` | Remove only legacy order metadata; exact origin/journal association still protects that order while another buyer remains regenerable. |
| `TestQueuedOrderLinkForBuyerResponseAfterConcurrentPlan` | After real bank-transfer Place and the bundle receipt commit, interpose real RegenerateLink and a distinct-key real payment-DM plan. The original token dies, the sealed competing token remains valid, response/replay has no dead URL and order/receipt/bundle/DM counts remain exact. |
| `TestQueuedOrderLinkReminderCandidateIdentity` | Scan A, issue A, then recheck newer B in the same bundle: exact rollback/refusal for that buyer, independent C still queues; later B plan seals B and protects only B. |
| `TestCheckoutReminderMigration0144ExactACL` (extended) | Pin the new definer's exact owner, `SECURITY DEFINER`, `search_path=pg_catalog`, PUBLIC revoke and runtime-only EXECUTE; workers have no EXECUTE and no new table privilege is granted. |

CI receipts (as of2026-10-13 11:20UTC; queued/in-progress is not PASS):

| Role | Source HEAD | CI run | Current result |
|---|---|---|---|
| Tests-only RED1, unchanged product883 | `bd4829216d577994463cee2653960e7179ce904c` | [Gates38379906480](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38379906480) | Native batch assertion FAIL: queued1/refused1 instead of2/0; artifact11803888339. Entire run CANCELLED after a stalled test failure exit, so no complete focused exit verdict. Response failed its fixture sentinel, not the target defect; missing new ACL function also failed as expected. |
| Tests-only RED2, added candidate race and bounded test interposition | `0d093e9216e598734e76e756b7261427348b52b1` | [Gates38381851583](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38381851583) | CANCELLED before gate; NOT_RUN. |
| Initial fixed focused suite | `d563301ef21f7c09cdaa9a1277fc6f6bb05b4f5d` | [Gates38382615370](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38382615370) | CANCELLED before gate for callback safety; NOT_RUN. |
| Initial required PR modes | `d563301ef21f7c09cdaa9a1277fc6f6bb05b4f5d` | [Gates38382155444](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38382155444) | CANCELLED for invalid response fixture/failure-exit safety; no product acceptance verdict. Contract drift plan SUCCESS. |
| Fixed Foundation, Node/structure/security gates | same fix HEAD | [Foundation38382155046](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38382155046) | SUCCESS; Node, check-gates, dependency docs/map, secret lint and vulnerability scan. Go/PG acceptance remains in Gates. |
| Corrected tests-only RED3, unchanged product883 | `4d99d19de1b0c3af71ec6c0229df2d194aaf34e6` | [Gates38385863056](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38385863056) | focused exit1, PASS0/FAIL2; artifact11807412647. Both target assertions fail: queued1/refused1 and non-null dead buyer_link. Additional fixture count failures corrected below; product diff from883 is empty. |
| Fixed focused REAL_PG/security/compatibility, fixture failures | `aefd9848b9b6af1dca4f44e09db02607811de87c` | [Gates38385979096](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38385979096) | focused exit1, PASS27/FAIL3; artifact11807188026. Batch/legacy/ACL and existing compatibility pass; response target-null assertion passes. Response counts, concurrent hold CHECK and ordered-upgrade fixture fail. No full acceptance claimed. |
| Superseded required PR modes | same code/test HEAD | [Gates38385984021](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38385984021) | CANCELLED by documentation successor, not a test verdict. |
| Superseded Foundation | same code/test HEAD | [Foundation38385983363](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38385983363) | CANCELLED by documentation successor. |
| Documentation successor Foundation | `758bc4a47e5f5209b49b52fc0247400914557794` (same code/test) | [Foundation38387433872](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38387433872) | SUCCESS. |
| Documentation successor PR modes | same HEAD | [Gates38387434407](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38387434407) | Superseded by the fixture correction below; current head acceptance must be read afresh. |
| Final tests-only causal RED, unchanged product883 | `399aaa8b2cbe7aa3d85ca9485a1ca15f60c77242` | [Gates38388650319](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38388650319) | FAILURE, focused exit1 PASS0/FAIL2. Only the exact queued1/refused1 and dead non-null buyer_link assertions fail. Artifact11806974765; [native red.log](r5-ci/red.log), SHA25625aed873db92a8fe255501eb14dee924b1cfab142985022df8084eae8f89ef29. |
| Fixed focused REAL_PG/security/compatibility | `6490e5fd9d0d1ce21a782b8f3412b3fb23afadf2` | [Gates38388648466](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38388648466) | SUCCESS, focused exit0 PASS30/FAIL0/SKIP0,109.454s; required jobSUCCESS. Artifact11808310239; [native green.log](r5-ci/green.log), SHA256d3bd8a3cfb27ac3785abd74023f5d2a1a59976436c25cfa678c01c42e844c9e4. |
| Fixed Foundation, Node/structure/security | same HEAD | [Foundation38388622723](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38388622723) | SUCCESS; test-node, check-gates (83 documented modes), dependency docs/map, secret lint and vulnerability scan. Contract drift errors=0 (362 baseline warnings). |
| Full repo-selected PR acceptance | same HEAD | [Gates38388623570](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38388623570) | In progress at receipt time;78 jobs including all selected browser modes/shards, no mode exclusions. Documentation successors trigger their own current required run; read final PR checks, never infer PASS from this pending row. |

Fixture correction: ManualOrders supports bank_transfer, pay_at_pickup and COD, none of which places an AWAITING_PAYMENT order. The initial response fixture's real reminder was therefore ineligible; that setup failure is not causal RED. The corrected response case uses the supported bank-transfer state and the real competing payment-DM planner to reproduce the same stale-token branch, while the separate batch/legacy/candidate cases test actual checkout reminders. Exactly one competing regeneration audit is required. Error returns in nested transaction callbacks preserve rollback on assertion failure; no testing.Fatal/Goexit is used inside those callbacks.

CI-driven fixture corrections9f96ecf7/e9f07181 preserve the target assertions and strengthen positive identity checks: `lbuNew` already committed one exact storefront hold, so the response case pins that hold, one added merchant order and the exact A16 order/bundle/conversation/consumed quote grant/live-price use instead of assuming ordinary checkout origins. The candidate B shifts creation and expiry together by one minute, preserving A's existing legal card-hold duration. The populated upgrade holds0171 with0144 and then applies both in numeric order; the new exact0171 checksum assertion supplements all existing schema/ACL/data-preservation checks. These are fixture changes only; productd563 is unchanged.

Fixed focused regex: `TestQueuedOrderLink|TestCheckoutReminder|TestLiveClaimsKC03|TestClaimsRetentionCRP02|TestLiveConsoleOrderForBuyerPayLinkOutcomes|TestLiveConsoleOrderForBuyerKeyBoundToItsBody`. CI uses its existing focused/job budgets; no assertion, timeout or coverage bound was loosened. New response-test interposition alone has a10s context deadline so its real competing call cannot silently hang. Next production browser builds perform Next's TypeScript checks; the Foundation workflow does not independently run tsc. GitHub completion notifications, the integrator's round packet and the existing thread heartbeat `pr36-lc-u3` (updated to CI-only PR32 ownership, every15minutes, quiet without meaningful change) are the wake path. This owner reads checks/logs/artifacts via gh and retains PR ownership until merge. No new task or local test process was created.

CI gates: current PR's full selected set includes `--browser-manual-order`, `--browser-live-console`, `--browser-inbox` and `--browser-studio-ui`, plus every other mode selected by the repository registry and all click/Go shards. They remain pending until their native current-run conclusions are available. No drawer/UI source changed in r5, so there is no new standalone drawer-browser mutation calibration. Local tests/builds/browser modes and standalone tsc are NOT_RUN(owner CI-only); no budget was raised.

Historical acceptance at883 is now verified **SUCCESS**: [Gates38374853066](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38374853066), all78 jobs with no failure, and [Foundation38374852669](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38374852669). These accept r4 only, not the new migration/code. Pre-directive r5 local attempts are **NOT_RUN** (interruption143, Docker-unavailable1, owner cancellation1), preserved under ignored `output/playwright/lc-u3-k3-p2/r5-red-batch*`; none provides causal RED or PASS. All newly claimed runtime verdicts must come from the CI run IDs above.

---

## PR32 r4 — CI acceptance handoff (2026-10-13)

**READY for isolated GitHub CI; full current acceptance is PENDING.** Owner directed immediate unit-branch push and CI acceptance on 2026-10-13; local contention is not a product verdict. Source HEAD **c54f70dc2b2e17abcac52ea01e6e098f52d97773**, source SHA256 **894caba8911faf12be896603c5ab4fae898c47382026c1252d7ac9e95f3c00ac**. Merged trunk **0bfe36e53acb694f4697521202e24f0790638941** using merge **5a51ea2c**, no rebase; both UI actions, suites and all Node entries retained. This handoff adds only delivery evidence.

- Implementer: Codex-3; family instruction GPT-6, actual runtime model/effort identifiers not exposed. Worktree `.worktrees/lc-u3-create-order-drawer`; branch `unit/lc-u3-create-order-drawer`. Exact allowed source/contract/test and delivery paths: [allowed-write-paths.json](r4-final-money/allowed-write-paths.json). Owner `output/ext-agents/**` excluded and preserved; no new migrations, ACLs, routes or dependencies in this delta.
- Evidence level: **E3 for the source-pinned automated red/green and REAL_PG evidence below; current full browser/Node acceptance is delegated to CI, not claimed PASS**. Platform edges and identity are MOCK; PG and local browsers are real disposable environments. No LIVE or full G07 acceptance.

Review replies / fixes:

- **4254715306**: ManualOrder retains frozen request/key and uses authoritative A16R recovery or durable absence seal before a new attempt; partial/UNKNOWN still needs same-key completion. `TestManualRecoveryAbsentSeal`, `CreatedReadback`, `PartialBegin`, `AuthorityAfterWait`, `ConcurrentSeal` cover server recovery; actual UI/client/BFF and browser cases remain registered.
- **Final review (a)**: unresolved guard is keyed by server-derived opaque tenant/store/principal scope, stable across 401 and re-login; no CSRF/token/PII is stored, and legacy/missing scope fails closed. Actual relogin browser causal RED and real-PG principal isolation evidence retained.
- **Final review (b)**: explicit pre-dispatch `neverSent` validation settles/disarms only a fresh attempt; earlier UNKNOWN, network failure and server 422 remain retained. Actual zero-request browser causal RED and three-locale component/client green retained.
- **Final review (c)**: regeneration and reminder planning share a transaction-level order lock and reject a queued/planned payment DM; ForBuyer signs and queues the exact new link atomically. Go red→green suite: `TestQueuedOrderLinkHTTPRegeneration`, `HTTPReminder`, `PlanningSerializesHTTP`, `ReminderSealHTTP`, `ForBuyerAtomicHTTP`, `UnknownHTTP` (full names share the prefix `TestQueuedOrderLink`). Child complete REAL_PG run20/20 and integrated focused53/53 are historical source-pinned proof; see retained statuses, not a claim of a new current-head local run.
- **4254999659**: View order uses existing `/{locale}/orders?store={store.id}&order={id}`. Three-locale actual-source RED26/3→GREEN29/0. Genuine lost201→recovery200→visible-click causal browser RED has prior35 passing and only the wrong-path case failing; correct source restored exactly. New real case requires exact loaded order detail, reload persistence and one PG order. Completed receipt replays200 before option validation, so post-complete422 is unreachable; original UNKNOWN422 absence/partial cases were preserved.
- **Studio regression**: asynchronous A2 fallback changed the navigation group between sampling and toggling; a competing editor was closed before its real PATCH committed. Fixtures now wait for positive A2/A1 loaded destinations and PATCH200/version increment. Product shell/permission code is unchanged; old mutation, conflict, permission and persistence assertions and deadlines are retained. Historical fixed Studio6/6 is retained; current Studio is CI-pending.

Current local structure gates (each exit0, source/history/status unchanged; native logs and full commands in [gates.json](r4-final-money/gates.json)):

| Command | Exit | Seconds |
|---|---:|---:|
| `pnpm -C apps/admin exec tsc --noEmit -p .` | 0 | 31.834 |
| `bash scripts/dev/check-gates.sh` | 0 | 529.255 |
| `go run ./scripts/dev/contractdrift` | 0 | 13.68 |
| `go vet ./...` | 0 | 13.67 |
| `go vet -tags browser ./tests/foundation` | 0 | 13.686 |
| `go build ./...` | 0 | 59.528 |

Local contention/incomplete runs (classification does not rewrite the raw native exit code):

| Run / native evidence | Classification | Raw exit / detail |
|---|---|---|
| `r4-merged-studio-setup-busy` | NOT_RUN(host-contention) | 1; second migration apply exceeded the existing20s budget; UI assertions were not entered. |
| `r4-merged-node-busy`, `r4-merged-node-native-busy` | NOT_RUN(host-contention) | 1; native PR18 never-settles subprocess exceeded10s, including nativeNode24.15/GOMAXPROCS2 run. |
| `r4-merged-gates-busy` | NOT_RUN(host-contention) | 1; nativeGo600s; later identical current check-gates passed0. |
| `r4-merged-manual-busy` | NOT_RUN(host-contention) | 1; nativeGo300s, partial20-case result; no complete36/PG21 acceptance. |
| `r4-merged-manual` / `create-order-drawer-4256920045` | INCOMPLETE(local teardown; CI pending) | 1; `route.fetch` context disposed in capability helper24:26; PG16 expected21. Result missing and click-ledger empty: explicitly incomplete artifacts, not PASS. |
| current live-console, inbox and focusedREAL_PG | NOT_RUN(current source; delegated to CI) | No additional local heavy runs after owner routing decision. |

Exact status/log pointers are in [acceptance-status.json](r4-final-money/acceptance-status.json) and `r4-final-money/runs/` / `logs/`; raw hashes and any privacy transformations are in [receipt-provenance.json](r4-final-money/receipt-provenance.json). Source and historical output SHA before/after are included in every wrapper receipt. Malformed/empty artifacts retain an explicit INCOMPLETE marker, byte count and original hash. Signed MOCK cookies, CSRF, request keys, Authorization and buyer links are sanitized before retention; gzip is verified against decoded content hashes. Source files and all tracked acceptance inputs are hashed. Older sections and invalid/calibration/environment attempts retain their original evidence scope.

CI gates: the real selector chooses **all54 modes** listed in [r4-pr-modes-live.json](r4-final-money/r4-pr-modes-live.json), including **--browser-studio-ui, --browser-manual-order (drawer36/PG21 plus legacy10), --browser-live-console and --browser-inbox**. None excluded; no budgets raised. GitHub Gates and Foundation run IDs will be registered in Humaux and the READY response after the normal unit-only push. CI and final independent delta review remain pending; no GitHub thread replies/resolves, production mutations or G07 run by this unit.

Risks/limits: legacy reminder without order_id conservatively blocks store-wide regeneration; seeded legacy namespace repair and long lock-wait counterexamples NOT_RUN. Before/after source hashes, valid causal reds, incomplete runs and six current local green gates are separated explicitly. The old 422 created-flow and lock/teardown-blocked href attempts are not counted as valid link reds.

---

## PR32 r3 — manual-order authoritative recovery, thread4254715306 (2026-10-13)

**READY for CI and final independent review, E3 in the tested local environment.** This is the current handoff; older sections retain their original SHA and evidence scope. Base **`b1229fbda54e3aa4a1262387d864a20382ec90f0`**; unit and integration fetch/merge at task start were already current, integration **`5a54d41b7e86a72eeec2afbe1f7b0458daaf742b`**. Source commits: browser RED fixture `8d812b99`, backend `3aa35a7d`, UI/BFF `62ff64ef`, classified router test `5aaa72d8`, and loaded-orders/reload synchronization `187ca4d5`. Final gates bind exact source HEAD **`187ca4d56c4bddf7d123c3746c7ea05ec3bc8676`**; the handoff commit adds only this delivery and its evidence.

Implementer: Codex-3, parent runtime model/effort **NOT_EXPOSED**. Backend and browser writers each used their own worktree on b1229fbd; backend/browser/read-only helpers were explicitly configured `gpt-6.1-sol/high`, without inferring their actual runtime identity. No recursive delegation. Exact17 source/contract/test paths and owned delivery paths: [allowed-write-paths.json](r3-manual-recovery/allowed-write-paths.json). Owner findings SHA256 remains **`1e19b04a25cfb255b1d3a34f404178df7e4405c7b20fd6a24504377ecea1d23e`**. Migration0170, A16R, the drawer, shared attempt rules, OpenAPI, dependencies and lockfiles are byte-identical to this base. Only the normal unit branch push is authorized.

### Thread reply and resulting behavior

**4254715306:** ManualOrder now exposes authoritative recovery using the frozen original body and Idempotency-Key. Matching Begin plus the upper manual receipt proves a completed order; Begin alone remains503 UNKNOWN for explicit same-key retry. Proven absence is sealed atomically under the original checkout lock, and the old key returns409 `attempt_closed`. Only that verified seal permits an explicit new attempt. Lost responses, persistent refusals, concealment and reload cannot silently create a new key. Real clicks and PG readback prove both absence/new-intent and partial/explicit-retry flows create exactly one order each.

The root cause was a missing ordinary-manual recovery endpoint: the shared retention rule correctly kept an ambiguous request, but the page could never obtain authoritative proof needed to finish it. Additive `contracts/storefront-v2.md` G3 defines **`POST /v1/admin/stores/{store_id}/orders/manual/recover`**, implemented through the existing private attempt binding/receipt/seal mechanisms and the real admin BFF. The server requires current `inventory:reserve`, `catalog:read` and `orders:read`, canonical original body/key, exact tenant/store/private owner and original Begin lock. It rechecks authority after waiting. Removed delivery/payment options do not prevent proof; current validation is not substituted for the submitted body. No claims reservation cleanup or new migration/grant is introduced for ordinary manual orders.

The client keeps the editable draft separate from the frozen attempt and arms the opaque OrderGuard before submitting. Every retained definitive refusal exposes recovery. Verified `created` shows the existing order; verified `not_created` allows the explicit new-attempt button; partial/error/UNKNOWN never unlocks. Reload persists only the opaque guard, never body/key/PII, and blocks replacement submission while directing the merchant to order lookup. Native concealment, logout, permission loss and late recovery/link-regeneration replies clear private UI and preserve that guard. The existing InboxPrivacy lifecycle and real shared recovery client/BFF seams are reused.

### Red → green, including real browser and PG faults

- Component/client causal RED [manual-recovery-node-red3.log.gz](r3-manual-recovery/logs/manual-recovery-node-red3.log.gz):5 FAIL,6 PASS,0 skip; the loaded old page lacks the recovery control after the frozen retry. Backend real HTTP/PG RED [red.log.gz](r3-manual-recovery/backend/red.log.gz):PASS1 FAIL6 SKIP0 exit1, ordinary manual placement still passes and the missing recovery route returns404. Backend green, including existing A16R regressions, is PASS32 FAIL0 SKIP0 exit0, preserved with its original source digest and commands.
- Full browser causal RED [r3-manual-browser-red.status.json](r3-manual-recovery/runs/r3-manual-browser-red.status.json):exit1, source **`6fb4f655d2c699c656712bd27a887a2f0d4033dd16f36895bee2f65b677bad0e`**. Replace only ManualOrder with its exact b1229fbd component; all original31 drawer cases pass, then **DU3-manual-absence-recovery** fails at the missing visible recovery control after the real dropped422 and same-key retry. Recovery POST is not reached; the new manual backend route was not yet integrated into root. Original mutation metadata and exact restored backup hash remain in [browser-mutation.json](r3-manual-recovery/browser-mutation.json) and [receipt-provenance.json](r3-manual-recovery/receipt-provenance.json).
- Privacy causal RED: six concealment/late-recovery failures; native lifecycle fence makes them GREEN. A subsequent actual pending link-regeneration mutation test fails once on stale private result restoration; the same ticket/authority fence makes it GREEN. Final real component/client/BFF suite **33 PASS,0 FAIL/skip**, including10 delayed-recovery authority cases and4 delayed-regeneration cases. These are component-level tests with mocked presentation/transport hosts; they do not substitute for browser/PG acceptance.
- Current full browser **33 cases/PG18**, plus original ManualOrder10, is GREEN. Two new flows operate visible controls through the real BFF/API/PG: no-order dynamic refusal → recover/seal → old-key409 → explicit new intent; and actual PG request termination between Begin and upper receipt → recovery503/no seal → same-key retry completes source/link/audit exactly once. Both navigate to a positively loaded exact order row, reload, and verify that row and order-id detail again. [manual-recovery-pg-readback.json](r3-manual-recovery/browser-green/manual-recovery-pg-readback.json):orders2,upper receipts2,absence seals1; only a count of three observed private keys is retained. [click-ledger.json](r3-manual-recovery/browser-green/click-ledger.json):127 PASS rows, preserving original119 and adding8 including catalog operations; six new non-catalog real-click rows all PASS.
- Preserved failed iterations: initial component/BFF fixture setup failures are not product RED. The first full node run failed only because the new router test filename was not classified by CI-SELECT; its exact assertions were moved into the existing classified `order_recovery_test.go`, no registry coverage was removed. The first browser green attempt completed order/receipt/seal but failed the list/detail stage; immediate reload after a NextLink click lacked a navigation sentinel. The subsequent fix adds real URL and positively loaded table/exact-row waits before reload, preserving all persistence/id/count assertions. Safe helper file/line diagnostics were added. Both original failed logs remain; no retries, timeout increases or weakened assertions.

### Frozen-source local gates

All seven final runs use source SHA256 **`0e376f1375815ad3d19d5c5eab273691d4bf4d8ff572d6102eb6326f9153e79c`**. Source, historical output and tracked status remain unchanged throughout each run; clean tracked source before/after. Historical output before this additive curation is **`cd6edadfac53eb0f985a7d7cf1c3ea6d7240b4bb15bbd411e5acfebc4ca58dcc`**. Exact commands, outer process deadlines and run IDs: [gates.json](r3-manual-recovery/gates.json). Native product/test timeouts and assertions were unchanged.

| Command | Exit | Evidence / result |
| --- | --- | --- |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | **0** | `r3-manual-tsc-current`,4.387s |
| `bash scripts/dev/test-node.sh` | **0** | `r3-manual-node-final`,1697 PASS,0 FAIL/cancel/skip,83.039s |
| `bash scripts/dev/check-gates.sh` | **0** | `r3-manual-gates-final`,173.652s; evidence ratchet, headers,83 modes,1294 tests in exactly one of10 shards, all split HTTP files classified |
| `bash scripts/dev/test-local.sh --browser-manual-order` | **0** | `r3-manual-browser-green2`,111.297s;33/PG18 + legacy10;127 PASS click rows |
| `bash scripts/dev/test-focused.sh '^(TestManualRecovery\|TestMerchantToolsManualOrder$\|TestLiveConsoleOrderForBuyerKeyBoundToItsBody$)' ./tests/foundation ./internal/httpapi` | **0** | `r3-manual-realpg-root`,55.804s; **test-focused: top-level PASS=10 FAIL=0 SKIP=0 exit=0**, race REAL_PG |
| `go run ./scripts/dev/contractdrift` | **0** | `r3-manual-drift-current`,5.064s; **errors=0 warnings=362**, Go282/contract150/BFF305 |
| `go vet -tags browser ./tests/foundation` | **0** | `r3-manual-browser-vet`,8.895s |

New focused tests: `TestManualRecoveryAbsentSeal`, `TestManualRecoveryCreatedReadback`, `TestManualRecoveryPartialBegin`, `TestManualRecoveryAuthorityAndKind`, `TestManualRecoveryCachedCreatedNeedsReceipt`, `TestManualRecoveryAuthorityAfterWait`, `TestManualRecoveryConcurrentSeal`, `TestManualRecoveryRouter`; original `TestMerchantToolsManualOrder` and exact g09 `TestLiveConsoleOrderForBuyerKeyBoundToItsBody` also pass. The browser runs unfiltered with inherited engine/grep/calibration/evidence-root overrides cleared. Run IDs: drawer **create-order-drawer-1710973912**, legacy **manual-order-link-342683546**. Original cases remain unchanged; native gates clean up their own fixtures and all owned bounded processes have exited.

### CI gates and limits

Changed specs/helpers: `tests/admin/create-order-drawer-gate.mjs`, `tests/admin/create-order-manual-recovery.mjs`, `tests/foundation/browser_create_order_drawer_test.go`, `tests/foundation/browser_create_order_manual_recovery_test.go`; owned by existing **`--browser-manual-order`**, no new mode. `test-local.sh --list` exit0 has84 entries including foundation. Current [pr-modes.json](r3-manual-recovery/pr-modes.json) selects54 modes; manual-order ran here, other53 **NOT_RUN locally in this round**. Full foundation/G07, optional LiveKit suite, fresh new-key calibration, real providers, LIVE and production are NOT_RUN here. Earlier A16R/0170 and console/inbox proofs stay bound to their original hashes; they are not fresh runs of this new ordinary-manual path. Integrator runs final independent review including0170 and required CI; author has not posted/closed review threads. Read-only helper source/artifact inspection is E1, not independent runtime E4.

The existing10-minute manual-source marker window is unchanged: an aged Begin-only attempt can remain UNKNOWN if original replay cannot finish; there is no blind new key or speculative reconciliation. The local browser/PG boundary uses disposable REAL_PG and signed MOCK identity/platform/edge, not provider/deployment acceptance. Evidence contains counts/booleans/control labels and synthetic order pointers; no raw keys/bodies, buyer-link tokens, credentials or buyer PII. Lossless compressed logs are hashed against originals, with explicit failed/setup provenance: [FILES.sha256.json](r3-manual-recovery/FILES.sha256.json), [RAW-LOGS.sha256.json](r3-manual-recovery/RAW-LOGS.sha256.json), [source-files.sha256.json](r3-manual-recovery/source-files.sha256.json), [receipt-provenance.json](r3-manual-recovery/receipt-provenance.json). Owner findings, backups and historical evidence are preserved.

## PR32 r2 — 0170 recovery cleanup and merged-head local handoff (2026-10-13)

**READY for CI and independent review, E3 in the tested local environment.** This section supersedes the historical allocation-blocked checkpoint below. The owner approved adding the migration (“新增吧”); unused **0170** was reserved under the coordination lock and committed as **`eea9df08dda4268376ac8b905562ec2451ad67d6`**. Merge **`c2874c8a9ebaad3997fade0c8e004285bf970af6`** incorporates FETCH_HEAD **`5a54d41b7e86a72eeec2afbe1f7b0458daaf742b`**, including #42–#44, with no rebase/conflict. Both CommentStream create-order/label-print entries and both suite lists survive. Additional commit **`ca2151fcc3e1bf6f501927aa86337d576e7e750b`** fixes only the existing INU03 fault fixture's lifecycle and adds its positive loaded-denial sentinel. The final evidence commit changes only this unit's delivery artifacts.

Role: Codex-3, actual parent runtime model/effort **NOT_EXPOSED**; configured read-only audit helper `gpt-6.1-sol/high`, not a claim about its actual runtime identity. Exact allowed writes: [allowed-write-paths.json](r2-0170/allowed-write-paths.json). Owner findings remain untouched (`1e19b04a25cfb255b1d3a34f404178df7e4405c7b20fd6a24504377ecea1d23e`). No dependency/lockfile/OpenAPI changes, production operation or GitHub thread writes. Normal push is authorized only for `unit/lc-u3-create-order-drawer`.

### Six thread replies

- **4232390274:** queued redacted replay shows the queued state and hides copy; the handler cannot regenerate/supersede the queued payment DM's original valid URL.
- **4249176849:** both rendering and handler permit regeneration only for `send.state === 'not_sent'`; the shared queued-copy red→green and current full drawer mode cover both threads.
- **4232390282:** a 422 never proves absence. Explicit A16R uses the frozen original key/body and seals the original attempt under its checkout lock; only verified `not_created` unlocks a new intent. Pending cleanup is atomic with this seal.
- **4250496779:** Begin alone is partial; recovery stays503 UNKNOWN until the post-bundle/post-DM completion receipt exists. Real pre-Place request termination plus explicit same-key retry completes source, link, bundles and one queued DM exactly once.
- **4250747441:** every definitive response retaining the ambiguous attempt exposes recovery, including409 capability after actual `inbox:reply` revocation. Recovery does not need that reply grant; UNKNOWN/network/5xx still retain the frozen attempt and guard.
- **4250501391:** scoped `claims.for_buyer_request` locates only the original pending request after validating **every** matching key row's canonical body/private buyer owner/request identity, including released tails. Existing release and no-order seal/recovery receipt share one transaction; unrelated pending and placed requests are preserved, and any seal failure rolls the release back.

### Contract, DDL and red → green

`contracts/live-console-v1.md` §5.1.1 defines A16R's original reservation cleanup. Migration0170 adds only the read-only STABLE getter: `SECURITY DEFINER SET search_path=pg_catalog`, owner `commerce_claims_writer`, PUBLIC revoke, exact `commerce_runtime` EXECUTE and COMMENT. No new table/RLS grant or HTTP/BFF route. KC03 pins exact owner/ACL/volatility; the historical R2 upgrade count is94 and its populated upgrade passes. Current merchant authority and tenant/store scope are server-resolved; no stored merchant principal is guessed.

- Initial real CLI RED `r2-0170-lookup-red-20261013`: exit1, PASS0/FAIL7; getter absent and pending/grant remained despite a recovery seal. The first post-DDL run had one foreign-store fixture admission failure (PASS26/FAIL1), preserved as a fixture error; its authentic scope admission was fixed, with no assertion weakening, then PASS27/FAIL0/SKIP0.
- Causal mutation **`r2-0170-multi-red-20261013`**, source `e0f0ba008ad38fd5f7eb40e309330901705b55d4988fb1ae5f3891d63c5f7a30`: skip only the release call after the real lookup. Both tests and all single/double-bundle subcases fail with pending1/2 and live grants1/2 despite seal/recovery1. Restore exact Go SHA256 `331034ee420356f78922e184f34bdf3b30717eef49b1de2568d27110a43eca5d`; green2 and the merged rerun each **PASS27 FAIL0 SKIP0 exit0**.
- `r2-0170-multi-pg-green-20261013` exit1 was host PG startup/EOF failure, not product RED: assertions NOT_RUN. The unchanged-source green2 passes in52.356s. Both logs remain; no deadline/bound increase or assertion weakening.
- Required merged inbox run exposed an unchanged trunk INU03 fixture race (18 other cases pass). Native trace: second real health GET begins60147.505, teardown60197.435, response60207.604, then body disposed. Shell/thread hooks both read health; all original business assertions had passed, and their broad regex also matched loading fallback. Fix preserves every assertion, adds exact rendered **“Meta permission is missing.”**, and `finally unrouteAll({behavior:'wait'})` drains handlers without suppressing errors. Full mode then passes; no retry or timeout change. Safe trace event receipt: [inbox-teardown-cause.json](r2-0170/inbox-teardown-cause.json).

Earlier partial/capability/queued-copy/component/refresh causal reds and their exact restorations remain in [r2-partial-recovery/](r2-partial-recovery/) and preceding checkpoints. Current Drawer31/PG16 acceptance includes both new real fault flows and preserves legacy ManualOrder10; a fresh new-key calibration is **NOT_RUN** in this batch, and the already-calibrated key/model code is byte-identical.

### Frozen-source local gates

The merged runtime/Go gates bind HEADc2874c8a to source SHA256 **`25de15e7bffc2a9e15355067bfb51ffe657aaaea65c604faed73f9908c5b1619`**. The final inbox/static gates bind source **`ef42e87789b0ebd41385cf3b3a71c308047e4c9345360851a09f864374e0686b`**, committed as ca2151fc. Their only difference is `tests/admin/inbox-ui.spec.ts`: **all other2720 tracked source files are byte-identical**, documented in [handoff-source-difference.json](r2-0170/handoff-source-difference.json). Thus the earlier manual/console/Go evidence still binds their unchanged inputs. The inbox runtime was frozen WIP with exact final file hashes; static gates run on clean ca2151fc. Every wrapper verifies source/history/tracked status unchanged throughout; historical tracked output SHA256 **`f279a64ad2ec0b3247860704b1613042a2ce699e3a31a209587efefc19a81be0`**. No files read by gates were edited during execution.

| Command | Exit | Result / run ID |
| --- | --- | --- |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | **0** | TypeScript strict; `r2-0170-handoff-tsc-20261013`, 49.153s |
| `bash scripts/dev/test-node.sh` | **0** | 1669 PASS,0 FAIL/cancel/skip;33 invocations; `r2-0170-handoff-node-20261013`, 302.399s |
| `bash scripts/dev/check-gates.sh` | **0** | drift, registry83+foundation, headers, evidence ratchet,1287 tests exactly once/10 shards; `r2-0170-handoff-gates-20261013`, 484.100s |
| `go run ./scripts/dev/contractdrift` | **0** | errors=0 warnings=362; Go281/contract149/BFF304; `r2-0170-handoff-contract-20261013`, 10.260s |
| `bash scripts/dev/test-focused.sh 'TestLiveConsoleRecovery|TestLiveConsoleOrderForBuyerKeyBoundToItsBody|TestLiveClaimsKC03|TestClaimsRetentionCRP02|TestR2IntegrationUpgradeFromReleaseHead'` | **0** | REAL_PG top-level PASS=27 FAIL=0 SKIP=0 exit=0; `r2-0170-merged-pg-20261013`, 54.104s |
| `go vet ./...` | **0** | Go vet; `r2-0170-merged-vet-20261013`, 10.227s |
| `go vet -tags browser ./tests/foundation` | **0** | browser harness vet; `r2-0170-merged-browser-vet-20261013`, 10.230s |
| `go build ./...` | **0** | Go build; `r2-0170-merged-build-20261013`, 26.169s |
| `bash scripts/dev/test-local.sh --browser-manual-order` | **0** | Drawer31/PG16/119 passing clicks + unchanged ManualOrder10; `r2-0170-merged-manual-20261013`, 86.612s |
| `bash scripts/dev/test-local.sh --browser-live-console` | **0** | workspace23 + labels10; `r2-0170-merged-console-20261013`, 432.496s |
| `bash scripts/dev/test-local.sh --browser-inbox` | **0** | 19 Playwright cases/61 passing clicks; I11 credential verification; `r2-0170-inbox-teardown-sentinel-green-20261013`, 141.902s |

Exact commands, original deadlines plus wrapper outer bounds, exit codes and run IDs: [gates.json](r2-0170/gates.json). All three browser modes ran sequentially, unfiltered, with inherited grep/calibration/evidence-root overrides cleared. Optional LiveKit Node suite remains NOT_RUN. Registry `--list` has84 entries (83 modes plus foundation). Current selector returns54 modes: complete list in [pr-modes.json](r2-0170/pr-modes.json).

Native current artifacts: Drawer `create-order-drawer-3537389737`, unchanged ManualOrder `manual-order-link-2770448506`; console `run.c9si9oe2/live-console-2278774177` and labels `live-console-2054767302`; inbox `20261013T035533.483522000`. Console workspace23 and labels10 pass; original LCU2_404 all/private each yield **actual404, privateRows0, buyerPanel0, sessionRetained=true, postLossReads0**. Native hidden read count31→31, visible return1. The real en390 held-A1 refresh retains input17/original node, saves delta5/version1, and reloads. Original g09 exact key/bundle error assertion passes without contract/test weakening. Counts/booleans in PG readbacks prove source/link/complete receipts/bundles/one DM/original valid sealed URL; no buyer token or DM plaintext is retained.

### CI gates and limits

**CI and independent final review remain pending.** All54 selector modes (including foundation-shards and the three locally run modes) remain selected; the other51 were NOT_RUN locally. Full **`bash scripts/dev/release-gate.sh --strict --only G07`** is mandatory for0170 per PROCESS §2.6, both unit acceptance and post-merge; it is **NOT_RUN locally**, reserved for GitHub per the owner's RAM-heavy gate rule. Focused27 and local E3 are not a claim that full G07/required CI/release is green. The integrator/reviewer owns independent acceptance; no author self-posted `review/independent` or resolved review threads.

A partial pipeline beyond the original10-minute source-marker window remains UNKNOWN, with no blind new key or window expansion. This batch does not add speculative reconciliation. Evidence is disposable **REAL_PG + real browser with signed MOCK IdP/Meta/edge**, not LIVE messages, Linux CI, physical printer, production or merge clearance. Read-only source/artifact audit is E1, not an independent runtime rerun. Own gate processes exited and fixtures cleaned; pre-edit rollback backups and owner files are preserved.

Integrity/provenance: [FILES.sha256.json](r2-0170/FILES.sha256.json), [RAW-LOGS.sha256.json](r2-0170/RAW-LOGS.sha256.json), [source-files.sha256.json](r2-0170/source-files.sha256.json), [receipt-provenance.json](r2-0170/receipt-provenance.json). Gzip logs decode byte-for-byte to original hashes. Ledger receipts omit raw actual/expected/error fields with explicit transformations and source hashes; causal trace receipt omits URLs/params/annotations. No evidence rewrite is presented as original raw output.


## Historical checkpoint — allocation pending (superseded, 2026-10-13)

This historical checkpoint recorded the earlier allocation-blocked status and is superseded by the current r2-0170 section above. **No push had occurred at this checkpoint.** Five open-thread fixes and the two reported CI failures have local E3 evidence. Open thread **4250501391** still has a causal REAL_PG RED and an uninstalled proposal; the batch is **not ready to merge or push**.

Root commits: **`7b7cd89b`** (completed-pipeline recovery, exact legacy error type and retained-capability recovery), **`828255cb`** (stock draft survives an ordinary scoped refresh), **`827911163bf20415aa32c39e6c4e72995e6d5e76`** (merge FETCH_HEAD **`fef79203`**, including #42, without rebase or conflict). Existing CommentStream create-order and label-print entries, both Playwright suites and every Node list entry survive. `test-local.sh --list` exits0, 83 browser modes plus foundation.

Implementer: Codex-3. Parent runtime model/effort **NOT_EXPOSED**. Isolated backend/test/source-audit helpers use the previously configured `gpt-6.1-sol/high`; configuration is not an inferred runtime identity. No recursive delegation. Root independently reviewed and ran acceptance. Exact implemented, WIP, proposed and delivery write paths: [allowed-write-paths.json](r2-partial-recovery/allowed-write-paths.json). No dependency, shared schema, production host, provider or numbered migration was changed. The owner's `output/ext-agents/k3-review-lcu3/findings.md` remains untouched, SHA256 `1e19b04a25cfb255b1d3a34f404178df7e4405c7b20fd6a24504377ecea1d23e`.

### Replies for all six open threads

- **4232390274:** queued redacted replay keeps copy hidden; handler cannot regenerate a queued DM URL, and the current full drawer run preserves the original valid sealed link.
- **4249176849:** regeneration is allowed only when `send.state === 'not_sent'`, at both render and handler boundaries; the shared queued-copy red→green below and current full mode cover the same defect.
- **4232390282:** business 422 alone never frees the attempt; explicit authoritative A16R uses the frozen original body/key and only a durable no-order seal permits a new intent. Current real browser clicks retain that recovery flow. The remaining pending-reservation cleanup defect is separately tracked as 4250501391 below.
- **4250496779:** checkout Begin alone is partial. A final append-only receipt written after ManualOrder, bundle bookkeeping and requested DM side effects must exist before recovery reports `created`; otherwise503 `retry_later` retains the original attempt for explicit same-key/body retry.
- **4250747441:** every definitive response that still retains an ambiguous attempt exposes recovery, including409 capability after real `inbox:reply` revocation. This does not release OrderGuard, alter the frozen body/key or treat UNKNOWN/network/5xx as definitive. A16R needs no reply permission.
- **4250501391 — BLOCKED:** no-order recovery currently seals but leaves pre-Begin pending bundles/live grants. Two real crash/rollback tests reproduce it. A narrow scoped original-request lookup and existing release in the same seal transaction are prepared; allocation of a new migration number is still pending, so no production DDL or cleanup source is installed.

### Reported CI failures

- **g09 / `TestLiveConsoleOrderForBuyerKeyBoundToItsBody`:** the new attempt wrapper returned a `merchanttools` error where the established exact contract is `claims.ForBuyerError(409, idempotency_conflict)`. The mapper restores that domain error type; the original assertion and body/bundle isolation checks are unchanged. No contract or test was weakened. Earlier broad focused suite:29 PASS; current exact four-test focus:4 PASS below.
- **live-console / `run.VBsVPxAy`:** run38284163871, job114929577492, artifact11762339792 failed the existing en390 stock Save control, before opening the drawer; label suite10 passed. Same-authority refresh cleared workspace data, unmounted the edited stock row and lost its draft. The hook now retains loaded data across ordinary refresh, but still clears on disabled/hidden/logout/scope/cookie changes and authority loss. A node counterexample is red→green; existing en390 browser flow now holds the real A1 response, verifies input17 and original DOM node survive, waits for that exact response, saves delta5/version1 and reloads. Full current workspace23 and labels10 pass. The exact CI occurrence's lost-draft mechanism is inferred from its disabled Save/no inventory POST; the source bug and fix are directly reproduced.

### Current frozen-source gates

All runs below use HEAD `82791116` and tracked-source SHA256 **`8ac92baa1bf78913b6e1d2b00cd1ac1ea780e19206100ef94e60ad5b5aedf47d`**. This is a **file-hash checkpoint, not a clean-HEAD-only claim**: two new pending/lookup Go test files are staged, uncommitted WIP and were included in that digest/compilation. They were not selected by the four-test focus and are not counted as green. Every wrapper verifies unchanged source, historical output SHA256 `b6449b980cd1ecf29df34ccbb69f5fb1f7756e9c9512206489836e91edf8a637`, and unchanged tracked status throughout the run. Exact commands/bounds/statuses: [gates.json](r2-partial-recovery/gates.json); changed-file hashes: [source-files.sha256.json](r2-partial-recovery/source-files.sha256.json).

| Command | Exit | Current result / run ID |
| --- | --- | --- |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | **0** | `r2-merged-tsc-before-lookup-20261013`, 4.060s |
| `bash scripts/dev/test-node.sh` | **0** | `r2-merged-node-before-lookup-20261013`, **1647 pass,0 fail/cancel/skip**,32 invocations,56.605s |
| `bash scripts/dev/check-gates.sh` | **0** | `r2-merged-gates-before-lookup-20261013`,178.922s;83 modes;1286 tests exactly once in10 shards; backend70 dirs/90 split files/52 browser modes; headers and evidence ratchets |
| `go run ./scripts/dev/contractdrift` | **0** | `r2-merged-contract-before-lookup-20261013`, **errors=0 warnings=362** historical baseline; Go281/contract149/BFF304 |
| `bash scripts/dev/test-focused.sh 'TestLiveConsoleRecovery(PartialPipeline\|PartialPastSourceWindow\|OldCreatedNeedsCompletion)$\|TestLiveConsoleOrderForBuyerKeyBoundToItsBody'` | **0** | `r2-merged-pg-fixed-before-lookup-20261013`; **top-level PASS=4 FAIL=0 SKIP=0 exit=0**; actual Go7.255s, wrapper134.582s includes machine-wide PG queue |
| `go vet ./...` | **0** | `r2-merged-vet-before-lookup-20261013`,4.364s |
| `go vet -tags browser ./tests/foundation` | **0** | `r2-merged-browser-vet-before-lookup-20261013`,1.292s |
| `go build ./...` | **0** | `r2-merged-build-before-lookup-20261013`,10.964s |
| `go test -race ./internal/merchanttools ./internal/checkout ./internal/httpapi ./internal/httperror` | **0** | `r2-merged-race-before-lookup-20261013`,11.052s |
| `bash scripts/dev/test-local.sh --browser-manual-order` | **0** | `r2-manual-green-before-lookup-20261013`,118.204s; **Drawer31/PG16**, unchanged legacyManualOrder10;119 drawer click rows |
| `bash scripts/dev/test-local.sh --browser-live-console` | **0** | `r2-console-green-before-lookup-20261013`,411.598s; **workspace23 + labels10**, Go373.16s |
| `bash scripts/dev/test-local.sh --browser-inbox` | **0** | `r2-inbox-green-before-lookup-20261013`,90.643s; real I11 credential verification and inbox chain |

Browser modes ran sequentially and unfiltered; grep/calibration variables and inherited evidence-root overrides were cleared. Only outer runner bounds are added (900s browser,600s focused/static,180s compile); original product/test deadlines, assertions, retry policy and bounds are unchanged. No source was changed while a gate was running. The optional LiveKit Node suite is NOT_RUN, outside these32 invocations.

Current native console evidence is **`output/playwright/run.RTiZWkoG/live-console-637929488`** and labels **`live-console-3748403557`**. Both original LCU2_404 views yield actual404, privateRows0, buyerPanel0, retained login and postLossReads0; native hidden state has31→31 server reads. These receipts and real Playwright logs are in [console/](r2-partial-recovery/console/). A separate read-only agent recomputed the source hash and reviewed current native artifacts, including mobile held-A1 execution and label writes/reload: no concrete issue. E4 is limited to independent artifact review, not an independent runtime rerun. Current inbox evidence is `output/playwright/inbox-ui/20261012T184808.651470000`, curated in [inbox/](r2-partial-recovery/inbox/).

### Causal red → green

- **Partial pipeline:** REAL_PG `r2-partial-pg-red2-20261013`, pre-fix source `f507b1ed`, exit1: exact backend killed between committed Begin and manual receipt; recovery wrongly200 created instead503. `r2-partial-pg-green3-20261013` exit0 **29 PASS0 FAIL0 SKIP**, including BeginOnly, ManualOnly, DMCommitted, past-source-window and old-created receipt cases. Current four-test REAL_PG rerun above is green with the original g09 assertion.
- **Partial real browser:** mutation removed only the completion probe. `r2-partial-browser-red-20261013`, source `cb1b68bd`, exit1; original29 cases pass, sole new target `DU3-partial-begin-recovery` observes false200 while source/link/upper receipts/DM are absent. Exact Go backup restored SHA256 `ef4bd5be82bb922315f38c46a70e502d3e3d4871d54bd45117f5f7250fcf55d9`. Current full31 green [manual-green/](r2-partial-recovery/manual-green/): actual pre-manual PG kill, dropped response, exact same-body/key retry422, recovery503 preserves guard, restored valid settings + explicit original retry completes exactly one order/source/link/bundle/completion/DM job and a valid original token; persisted order reload.
- **Capability real browser:** actual UNKNOWN response loss after committed order, revoke only owned `inbox:reply`, exact original retry409 capability. Mutation reinstates the old422/attempt_closed-only recovery condition. `r2-capability-browser-red2-20261013`, source `9fd845ba`, exit1; preceding30 cases/PG16 pass, sole missing-recovery-control target. Exact drawer restored SHA256 `64c5ffadd8acedacfd5e1bbdbc6dc1a5b830a98690a2bbfc262cae31da23a3ef`. Current full31 green proves visible recovery200 created despite missing reply grant, same body/key, exactly one persisted pipeline/DM; exact grant restored. Three-locale actual-component/client node RED3→GREEN also retained.
- **Mobile refresh:** `r2-console-refresh-node-red-20261013` source `3a786f54`, exit1: held same-scope load clears existing stock12; `r2-console-refresh-node-green-20261013` source `08166fd3`, exit0; full real console above exercises the held real response, edit/save/reload and unchanged privacy fences.
- **Pending cleanup — still RED:** `r2-pending-pg-red3-20261013`, source `73602f32`, exit1: both `TestLiveConsoleRecoveryPendingReservationReleasesOnlyOriginalAttempt` and `TestLiveConsoleRecoveryPendingReservationReleaseRollsBackWithSeal` fail with Pending1/liveGrant1 despite seal1/recovery1. Original store baseline orders/holds are measured before the fault, rather than assumed zero; unrelated pending intent is preserved by the test. This is not green acceptance.

Non-causal failures are preserved, not relabelled as red proof: first pending test had a fixture zero-baseline mistake; second had missing pinned Docker image (exit125, NOT_RUN); exact immutable image was restored before causal red3. Initial capability browser mutation used an undefined identifier and stopped at typecheck, NOT causal; corrected mutation red2 reaches the intended control. Earlier broad PG iteration had an aged-fixture constraint mistake and then the exact g09 type mismatch; both are retained. No assertion/timeout/bound was weakened. Prior queued-copy and new-key-on-retry calibration evidence remains in earlier sections; no fresh new-key calibration was run in this checkpoint.

### Pending integration and explicit limits

AGENT-PREAMBLE §1 forbids editing **“migration numbers you were not given”** and says to write the need in DELIVERY. The allocation question is pending. Prepared isolated worker commit **`12603930661b78cc68f31f29fdd20e840cec8873`** is **not applied**; its four-file [source.patch](r2-partial-recovery/pending-design/source.patch) and [SQL proposal](r2-partial-recovery/pending-design/for_buyer_request.sql), SHA256 `c9d44c6c8dccb9a699bcc8d774093b440818d689df8f2a654a155a96be09ff7d`, are DESIGN/E1 only. Proposal validates every tenant/store/key row against one canonical body and private issued buyer owner; authentic inventory permission is rechecked; exact STABLE definer owner/search_path/revoke/runtime-only EXECUTE/comment are retained. Under the original Begin lock and same seal/recovery transaction, release runs only for original pending rows, never created/placed rows; historical not_created replay repairs pending without rewriting old receipts.

Preserved root WIP: `tests/foundation/live_console_order_recovery_pending_test.go` (259lines) and `live_console_order_recovery_lookup_test.go` (338lines). Read-only scope/binding/ACL/historical repair tests have not run because the getter is not installed. Isolated test worker **`bb88166f`** adds68 lines mixed pending+released-tail body/owner/request conflicts; [mixed-tail-test.patch](r2-partial-recovery/pending-design/mixed-tail-test.patch) is not applied. Exact WIP text snapshots are alongside the proposal.

After allocation: install one additive migration; apply prepared source/contract and mixed-tail test; exact KC03 function/owner/ACL pins are already in the patch; update `TestR2IntegrationUpgradeFromReleaseHead` migration count93→94 and its last-migration comment. CRP populated upgrade intentionally holds0129; only add the new DDL to its holdback if the final definition eagerly depends on0129 (read-only review does not claim that failure is proved). No new lriShims caller was found. Then run focused recovery/key-body/KC03/CRP and **full G07** per PROCESS §2.6, followed by affected current-source local/browser gates. These pending tests/full G07, other selector modes, independent money-path final review and current merged-head CI are **NOT_RUN**, not PASS.

Receipt integrity: [FILES.sha256.json](r2-partial-recovery/FILES.sha256.json) and [RAW-LOGS.sha256.json](r2-partial-recovery/RAW-LOGS.sha256.json). Gzip logs decode byte-for-byte to their original hashes. Label/inbox click ledgers omit raw actual/expected/error strings; transformations and source hashes are explicit in [receipt-provenance.json](r2-partial-recovery/receipt-provenance.json). No buyer token/DM plaintext is retained; DM validity proof is private PG boolean/count only. Own runners have exited and fixtures cleaned; owner output and rollback backups remain. **E3 applies only to the implemented/tested subset in this checkpoint, with disposable REAL_PG and signed MOCK IdP/Meta/edge; not LIVE, Linux CI, physical-printer or whole-batch readiness.**


## PR32 merge1200178c — READY, local E3 (2026-10-13)

This is the current handoff; earlier sections retain their historical evidence. The money-path fixes were committed and pushed as `09bf1158bacaeb383a97074aa2772f6732da0aee` before this merge. Following the owner's instruction, fetched `origin r3/integration` and **merged FETCH_HEAD without rebasing**. Merge commit **`48102a4ed4d1309060a7ad2940fcb614da27197c`**, parents `09bf1158` and `1200178c5ffe3decaee09077e2bb8b7f59ea715c`.

- `CommentStream.tsx` retains the catalog/order/inventory permission gate, create-order entry, reset and parent privacy revocation, plus trunk's label-print provider, toolbar and row actions. Printing uses the same active-session/privacy fence. The conflicting inbox component host retains both React hook/context/portal sets. `playwright.config.ts` retains both `live-console.spec.ts` and `comment-label-print.spec.ts`; every Node test entry from both parents is retained. Registry layout remains intact.
- Source SHA256 **`214fff07d38bf7dcc86e5652bfdca064a95849a02b6e21ff26179fdf4c5f9b64`** binds all six runs below. Each run started and finished with clean tracked status and identical source/historical-output hashes. The final delivery commit changes only this document and these owned receipts, preserving that source digest.
- The merged tree's entire `internal/` delta from `09bf1158` is empty. The eight checked money-path files, including drawer, attempt, checkout and browser assertions, are byte-identical: [`merge120-money-unchanged.json`](merge-1200178c/merge120-money-unchanged.json). Earlier red→green and calibration evidence remains below; no new behavioral workaround, retry, timeout increase or weakened assertion was introduced for this merge.
- Implementer: Codex-3; parent runtime model/effort **NOT_EXPOSED**. Read-only audit helper configured `gpt-6-luna/medium`, own canvas only, no recursive delegation. Allowed resolution paths: `apps/admin/components/CommentStream.tsx`, `tests/admin/inbox-review-host.test.ts`; auto-merged suite/registry changes retain both parents. Delivery write paths: this file and `output/lc-u3-create-order-drawer/merge-1200178c/`. Owner `output/ext-agents/`, paused backups and historical output remain untouched.

| Exact command | Exit | Current merged-head result / run ID |
| --- | --- | --- |
| `cd apps/admin && pnpm exec tsc --noEmit -p .` | **0** | `merge120-tsc-20261013`, 1.718s |
| `bash scripts/dev/test-node.sh` | **0** | `merge120-node-20261013`, **1642 pass, 0 fail/cancelled/skipped**, 32 invocations, 41.766s |
| `bash scripts/dev/check-gates.sh` | **0** | `merge120-gates-20261013`, 208.138s; includes contract drift **errors=0, warnings=362** historical baseline, 83 documented modes, registry/header/evidence/backend ratchets and 1277 tests in 10 shards |
| `bash scripts/dev/test-local.sh --browser-manual-order` | **0** | `merge120-manual-order-20261013`, 133.476s; **Drawer29 + legacy10, REAL_PG14/14**, 111 passing drawer click rows |
| `bash scripts/dev/test-local.sh --browser-live-console` | **0** | `merge120-live-console-20261013`, 425.885s; workspace368.57s and labels35.89s, **52 label click rows PASS** |
| `bash scripts/dev/test-local.sh --browser-inbox` | **0** | `merge120-inbox-20261013`, 107.859s; real inbox chain/I11 credential verification, **61 click rows PASS** |

The browser modes ran sequentially, unfiltered, with known grep/calibration variables cleared. Existing ignored wrapper supplies only outer process bounds (120/300/600/900s), `GOFLAGS=-p=1`, process-group cleanup and source/status checks; test deadlines are unchanged. Exact commands, environment overrides, timing and hashes are in [`gates.json`](merge-1200178c/gates.json). Lossless `.log.gz` records preserve original bytes; [`RAW-LOGS.sha256.json`](merge-1200178c/RAW-LOGS.sha256.json) verifies decoded logs. [`FILES.sha256.json`](merge-1200178c/FILES.sha256.json) verifies the retained receipt files.

Current live-console evidence comes from **`output/playwright/run.uaBuE0HH/live-console-2176389442`** (workspace) and **`live-console-3713550877`** (labels), as identified by this run's log. No old top-level run supplies this acceptance. Both unchanged LCU2_404 cases produce real 404, privateRows=0, buyerPanel=0, sessionRetained=true and postLossReads=0. Native hidden state leaves server reads31→31. Receipts: [`live-console/`](merge-1200178c/live-console/). Inbox's native visibility receipt confirms private state cleared and revalidation on return. Queued-copy and 422 recovery real-click/readback receipts are in [`manual/`](merge-1200178c/manual/). Original full artifacts remain ignored; retained label/inbox ledgers omit raw actual/expected strings, with original hashes and transformations in [`receipt-provenance.json`](merge-1200178c/receipt-provenance.json).

Auxiliary `test-local.sh --list` and `pr-modes` checks exit0: 84 registry entries including foundation; selector returns54 modes. **Other51 selector modes, full foundation/G07 and optional LiveKit Node suite: NOT_RUN in this batch.** Earlier focused PG/race/vet/build results belong to their original SHA, not fresh runs here. Source-only whitespace check against trunk is0; a whole-merge whitespace check reports inherited trunk raw-log whitespace, preserved byte-for-byte rather than rewritten.

**E3: automatic local browser + disposable REAL_PG with signed MOCK IdP/Meta/console/edge; no LIVE provider, physical-printer or production acceptance.** All six requested modes are green on the merged source. Own gate processes have exited; independent review and merged-head CI remain pending. Authorized push is only to `unit/lc-u3-create-order-drawer`; no GitHub thread reply/resolution or trunk push.

## PR32 money-path follow-up — READY, local E3 (2026-10-13)

This section supersedes the old uncommitted/Git-readonly handoffs below. The owner authorized the necessary backend/contract/REAL_PG expansion and a push of **only** `unit/lc-u3-create-order-drawer`. No GitHub thread reply or resolution was made.

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u3-create-order-drawer`; pulled remote `8a0ec752` and preserved the integrator's parent revocation and SCOPE-404 changes.
- Final source commit: **`273ecd458e3a1d759b6cad7b6583e151cf424fe4`**. The final evidence commit changes only this delivery and owned receipts. All normal root gates bind the exact same tracked-source SHA256 **`9ecd532c412a9029adf35e46c8800231d64b8640056b9ebc086d18b8ff98a666`** (run HEAD `6840a124` plus the four-file delta committed as `273ecd45`). Each wrapper verifies unchanged source, tracked status and historical output throughout the run.
- Implementer: Codex-3 UI/BFF/integration. Exact parent runtime model/effort: **NOT_EXPOSED**. Backend worker configured `gpt-6.1-sol/high`, test worker `gpt-6.1-sol/medium`, read-only seam reviewer `gpt-6.1-sol/high`; selectors are configuration, not inferred runtime identities. Separate child worktrees, at most two writers, no recursive delegation. Root independently reviewed the seam and reran REAL_PG.
- Exact allowed write paths: [`r1-money-final/allowed-write-paths.json`](r1-money-final/allowed-write-paths.json), 26 source/contract/registry files plus this delivery and its four owned receipt directories. File hashes: [`r1-money-final/source-files.sha256.json`](r1-money-final/source-files.sha256.json). Owner `output/ext-agents/`, paused backups and historical output were preserved. No migration, grant, go.mod, lockfile or dependency changes.

### Findings and resulting behavior

- **4232390274:** a queued replay with `buyer_link=null` previously offered copy, which regenerated and invalidated the URL inside the queued DM. The copy control is hidden and the handler refuses regeneration unless `result.send.state === 'not_sent'`; three locales show the queued-link notice. Real replay proves zero regeneration and an original valid sealed URL after order reload.
- **4249176849:** the same fix enforces the exact `not_sent` rule at both render and action boundaries. The shared queued-copy red→green below covers both threads. Planned `meta.dm_send` remains untouched.
- **4232390282:** an arbitrary business 422 cannot prove that an earlier UNKNOWN request created no order. Additive A16R `POST /v1/admin/stores/{store_id}/orders/for-buyer/recover` receives the **original frozen body and key** through the real BFF/client seams. Strict authoritative 200 `created` returns the existing order; strict `not_created` commits a closure seal before the UI frees OrderGuard and permits explicit new intent. No blind new key on 422, UNKNOWN, network failure, malformed reply, 5xx or lost recovery response.

The server binds canonical full body/kind before work and serializes the complete attempt. The absence seal and original checkout share the exact Begin lock: recovery retains it through seal COMMIT, and both ManualOrder/for-buyer money writers check the closure under the lock through checkout COMMIT. A lost outer connection cannot let a late writer create an order after a seal. Post-wait permission/store/session checks prevent stale reads. Normalization clones shared input before validating; recovery can seal invalid recipient/payment input without re-running that failed business validation. The Quote remains the only price.

### Root gates on frozen final source

All logs/status JSON below are committed in [`r1-money-final/`](r1-money-final/). Raw logs in all four receipt directories are stored as byte-preserving `.log.gz`; decode with `gzip -cd`. [`RAW-LOGS.sha256.json`](r1-money-final/RAW-LOGS.sha256.json) verifies the original decoded bytes; older child manifests retain their original `.log` names/hashes. Original uncompressed run files remain in their ignored run directories. Status records include exact command, outer timeout, exit, HEAD, source hash and historical-output hash. Outer wrapper time includes the machine-wide PG queue; product/test deadlines are unchanged.

| Command | Exit | Actual result / run ID |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | **0** | **1557 pass, 0 fail/skip**, 31 invocations; `r1-money-node-final-20261013` |
| `cd apps/admin && pnpm exec tsc --noEmit -p .` | **0** | `r1-money-tsc-final-20261013` |
| `bash scripts/dev/check-gates.sh` | **0** | `r1-money-gates-final-20261013`; 83 documented modes, 1277 tests in 10 shards, HTTP coverage and evidence/header ratchets |
| `go run ./scripts/dev/contractdrift` | **0** | `r1-money-contract-final-20261013`; **errors=0, warnings=362** historical baseline; Go281/contract149/BFF304 |
| `bash scripts/dev/test-focused.sh '^Test(LiveConsoleRecovery\|LiveConsoleOrderForBuyer(Resume\|ConcurrentStaff)\|MerchantToolsManualOrder)$\|^TestLiveConsoleRecovery'` | **0** | `r1-money-pg-final-20261013`; **PASS=16 FAIL=0 SKIP=0**, Go39.539s; disposable REAL_PG |
| `go test -race ./internal/merchanttools ./internal/checkout ./internal/httpapi ./internal/httperror` | **0** | `r1-money-go-race-final-20261013`; all four packages |
| `go vet ./...` | **0** | `r1-money-go-vet-final-20261013` |
| `go vet -tags browser ./tests/foundation` | **0** | `r1-money-browser-vet-final-20261013` |
| `go build ./...` | **0** | `r1-money-go-build-final-20261013` |
| `bash scripts/dev/test-local.sh --browser-manual-order` | **0** | `r1-money-browser-final-20261013`; **Drawer29 + legacy10, PG14/14**, Go72.396s; no grep/calibration |
| `LC_DRAWER_CALIBRATION=new-key-on-retry bash scripts/dev/test-local.sh --browser-manual-order` | **1, expected RED** | `r1-money-key-calibration-r2-20261013`; full29, only `DU3-single-order-en-1440` fails; same body/different key, replay201, request orders2/receipts2, **PG15/14** |
| `bash scripts/dev/test-local.sh --list`; `node scripts/dev/pr-modes.mjs origin/r3/integration`; `git diff --check` | **0** | 84 registry entries including foundation; 54 selected modes; syntax/whitespace unchanged |

REAL_PG names: 13 `TestLiveConsoleRecovery*` tests, `TestLiveConsoleOrderForBuyerResume`, `TestLiveConsoleOrderForBuyerConcurrentStaff`, `TestMerchantToolsManualOrder`; full names and causal backend reds in [`r1-money-backend/DELIVERY.md`](r1-money-backend/DELIVERY.md). Root rerun includes connection loss, six post-wait authority variants, two-connection admission, invalid recipient/payment, missing merchant/checkout receipts, legacy ambiguity and both concurrent ordering outcomes.

### Red → green and real-click receipts

- Copy unit red: three locale failures against actual pre-fix component; green17/17. Real browser mutation removed only the copy render/action `not_sent` guards: `r1-money-copy-browser-red3-20261012` **exit1, full29, exactly one queued-copy failure, PG14/14**. Exact component backup restored to SHA256 `5640a8f15e94f4ed6b9eaf7500c506c49a48b138e0166f53e668c8c9b3a500ba`; normal final run above **exit0**. [`copy-browser-red/result.json`](r1-money-final/copy-browser-red/result.json) versus [`browser-green/result.json`](r1-money-final/browser-green/result.json): queued positive result, copy blocked, regeneration POST0, sealed operations1/secrets1/original_link=true/valid_links1/regenerations0, real persisted order reload/detail click. Both threads share this counterexample.
- Recovery reds: actual UI/BFF/client tests fail before A16R; authority/lost-reply counterexamples remain strict. REAL_PG reds demonstrate a late order after an unsafe seal, stale authority returning a private ID, invalid recipient/payment recovery refusal and shared-input race; final root16 pass. Browser drops the first real bank-transfer refusal response, replays exact key/body, positively observes real 422, clicks recovery, reads a durable not_created seal with zero order/success receipt, then explicitly starts new intent and reloads. Click ledger: [`browser-green/click-ledger.json`](r1-money-final/browser-green/click-ledger.json).
- Original key calibration stays separate from queued-copy assertions so a deliberately new key reaches the named exact-order failure rather than aborting on `buyer_link` shape. All normal queued assertions remain enabled. [`key-calibration-red/result.json`](r1-money-final/key-calibration-red/result.json) and independent [`pg-readback.json`](r1-money-final/key-calibration-red/pg-readback.json) prove the duplicate.
- Preserved non-acceptance failures: first copy browser run stopped on bank-only home option disappearing (`invalid_request` instead of exact bank refusal); the next hit the target plus compared JSON-escaped `&` rather than decoded sealed text. Fixed only fixtures, preserving asserted codes/counts. One normal run stopped while waiting for a foreign live PG lock: **NOT_RUN**, wrapper child exit -9. First key calibration timed out applying migration0119 at the existing20s setup limit, browser **NOT_RUN**; fixture cleanup verified, high host load observed, then one unchanged rerun reached the expected target RED. These logs are retained, never counted as product red/green. No threshold, assertion, product timeout, PG bound or retry policy was loosened.

### CI gates, limits and lifecycle

Changed existing browser spec/harness: `tests/admin/create-order-drawer-gate.mjs`, `tests/foundation/browser_create_order_drawer_test.go`, owned by **`--browser-manual-order`**; no new mode/fixture family. Exact selector output is [`r1-money-final/pr-modes.json`](r1-money-final/pr-modes.json): **54 modes**, of which the normal manual-order mode was run here. Remaining selector modes and **full foundation/G07 required for checkout changes: NOT_RUN locally**, for CI/independent review. Integrator's earlier live-console/inbox passes predate this backend delta and are not reused as its acceptance.

**E3: automatic local BROWSER + disposable REAL_PG, with signed MOCK IdP/Meta ingress/edge; no dispatcher, provider, SANDBOX/LIVE or production acceptance.** Independent source seam review E1 found no specific P0/P1 in the final reviewed seam. New backend expansion still requires independent money-path review and CI before merge. Fleet rollout must drain older unfenced writers before enabling recovery; legacy partial orders without the canonical binding, expired capabilities and inconsistent receipts remain UNKNOWN. Own runners/fixtures are cleaned; original failed proof, exact rollback backup and owner evidence retained. Shared state updates use Humaux locks; changed Go/TS/JS indexed and recovery/attempt rationale linked.

## Historical receipts — superseded as current handoff instructions

## PR32第一轮三P2未提交交接 — READY（2026-10-12）

- 最新裁决：**所有修改保持未提交**，集成方负责commit、`git merge FETCH_HEAD`、合并后最终门禁和push。Codex-3本轮没有fetch/add/commit/merge/push，暂存区为空，HEAD保持`7d03d087f0b7fa629d6f49f18c4974835bea3d47`。READY仅指当前工作树可供复核／提交，不代表PR已合并。
- Implementer: Codex-3，UI角色；runtime model/effort本端未公开，未推断。两位只读helpers为`gpt-6.1-sol/high`。允许写路径即下面精确14项；没有产品Go、迁移、生产ACL、契约或锁文件变化。
- 三条修复：UNKNOWN只接受七个可证明未建单的receipt后业务拒绝来结算，仍保留400/415、网络／5xx／auth／未知码的原body/key；限制检查按实际A15 offer匹配A13场次，缺失／冲突时三语明确无法检查；三个创建入口均增加catalog:read，真实无catalog角色不发A15/options/A16。
- Browser fixture修正：角色信息改用真实页面fetch。当前Chromium HTTP loopback的Secure cookie在APIRequestContext上未发送，合成cookie探针为API401／native200；没有伪造权限或放宽HTTP200断言。Go严格矩阵从26增加到29，并独立要求三语言各一条唯一的真实权限PASS记录。PG14与既有超时均未改动。
- 本轮最终tracked源码hash：**`e0abd876cfc6aae9002817a15e2a28b8b417c4ef3cbf3a838b14c59d3332ad35`**。未跟踪的新增`create-order-review.test.ts`另有逐文件hash；red→green期间逐字节相同。每次run的源码／历史output／tracked状态前后均相同；下面结果不依赖历史轮次的通过声明。

| 当前工作树命令 | exit | 结果／run |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | **0** | **1367 passed，0 failed，29调用**；`p2-handoff-final-node-20261012`，包括真实BFF seams与严格TZ依赖断言 |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | **0** | `p2-handoff-final-tsc-20261012` |
| `bash scripts/dev/check-gates.sh` | **0** | `p2-handoff-final-gates-20261012`；82模式、headers／registry；内含contractdrift：**errors=0，warnings=362**历史baseline |
| `go vet -tags browser ./tests/foundation` | **0** | `p2-handoff-final-vet-20261012` |
| `bash scripts/dev/test-local.sh --browser-manual-order` | **0** | `p2-handoff-browser-final-20261012`；**Drawer29＋legacy10**；真实PG **14/14**；Go PASS |
| `LC_DRAWER_CALIBRATION=new-key-on-retry bash scripts/dev/test-local.sh --browser-manual-order` | **1（预期RED）** | `p2-handoff-calibration-20261012`；仅`DU3-single-order-en-1440`失败；同body／不同key，HTTP201，PG15/14；该请求两个订单／receipt |
| `git diff --check` | **0** | 暂存区为空，所有改动未提交 |

证据：`output/playwright/lc-u3-k3-p2/p2-handoff-*.{log,status.json}`及`output/playwright/lc-u3-k3-p2/handoff-20261012/manifest.json`（含逐文件hash、日志hash及精确提交路径）；正常Drawer `output/playwright/create-order-drawer-3833881137/{result,click-ledger,pg-readback}.json`；legacy `output/playwright/manual-order-link-2464377956/`；校准 `output/playwright/create-order-drawer-4043610404/`。浏览器三条权限案例为三语言×390px、真实PG fulfilment角色／签名MOCK OIDC，含两个入口和reload；直接Drawer守卫另由actual-source Node覆盖。

RED保留：原拒绝死锁`p2-attempt-red.log`；actual-source三项反例`p2-resume-red-20261012.log`（11目标失败＋2正对照）；严格TZ未登记依赖`p2-related-node-final-20261012.log`；首次浏览器角色读取失败`p2-handoff-browser-20261012.log`；随后29项全部通过但Go仍要求旧26项的`p2-handoff-browser-green-20261012.log`；cookie根因`p2-cookie-transport-20261012.log`。没有删除断言、改低阈值、增加重试或延长超时。

**证据级E3：当前工作树的自动化BROWSER＋REAL_PG，外部IdP／Meta／TLS edge为MOCK。** 独立源码复核E1没有指定范围P0/P1／隐私P2；K3最终复核、merged-head CI／更广pr-modes、LIVE／生产部署仍未完成，由集成方处理。现有receipt JOIN及merchant-manual计数不单独外推所有checkout孤儿订单不存在。

### 精确提交清单与消息

修复及其共用测试是一个原子review批次，避免拆开Drawer／host／browser fixture形成中间失败提交。第一个逻辑提交包含这13个文件：

```text
apps/admin/components/BuyerPanel.tsx
apps/admin/components/CommentStream.tsx
apps/admin/components/CreateOrderDrawer.tsx
apps/admin/lib/create-order-attempt.ts
apps/admin/lib/create-order-copy.ts
apps/admin/lib/create-order-model.ts
scripts/dev/test-node.sh
tests/admin/create-order-attempt.test.ts
tests/admin/create-order-model.test.ts
tests/admin/create-order-review.test.ts
tests/admin/create-order-drawer-gate.mjs
tests/admin/timezone-ui.test.mjs
tests/foundation/browser_create_order_drawer_test.go
```

```text
fix(lc-u3): resolve order drawer retry, session and catalog review findings

Settle only proven order refusals after UNKNOWN, resolve restriction checks
from the actual prefill, and require catalog access at every entry.
Verify immutable retries and narrow-role controls through real browser/PG,
and keep the timezone fixture on the real attempt/model seam.

Implemented-by: Codex-3
```

第二个逻辑提交仅`output/lc-u3-create-order-drawer/DELIVERY.md`：

```text
docs(lc-u3): record round-one P2 verification and uncommitted handoff

Implemented-by: Codex-3
```

**不要提交**owner的`output/ext-agents/`或ignored运行产物；原7项paused.patch、确切stash及before/final备份保留。原测试与失败证据未被重写。集成方合主干时保留两边意图，尤其`test-node.sh`两边的每个entry；merged-head再跑必过门禁。受限环境记录下面仅作为历史，不是当前BLOCKED状态。

## PR32第一轮三P2修复 — 2026-10-12，BLOCKED／未提交

- Implementer: Codex-3，UI角色；本端未公开确切runtime model/effort。只读测试／安全复核：`gpt-6.1-sol`，high reasoning。Branch `unit/lc-u3-create-order-drawer`，工作树 `.worktrees/lc-u3-create-order-drawer`，HEAD/base `7d03d087f0b7fa629d6f49f18c4974835bea3d47`。**没有新提交或合并，不push。** 当前代码绑定文件hash，不能用以下历史轮次的通过结果替代本轮验收。
- UNKNOWN后确定拒绝：仅保留在checkout durable receipt查询之后、可证明未建单的七码结算白名单；解析400/415、通用conflict/not_found、未知码、401/403、5xx／网络UNKNOWN仍保持原body/key。Drawer结算后清除uncertain和OrderGuard；新意图仍只从显式new-attempt进入。
- 限制名单：BuyerPanel删除`claims[0]`场次推断；Drawer用实际A15 offer集合匹配A13唯一有效session，当前CommentStream直接传真实场次。缺失、截断、冲突或多场次信息保持三语“无法检查”，不猜身份或场次。
- 权限：Drawer、BuyerPanel和CommentStream三处均要求`catalog:read`+`orders:read`+`inventory:reserve`，三语原因明确商品目录权限。实际源码组件测试证明缺catalog时不发A15/options/A16；正向加载sentinel避免空页面假通过。
- 测试准备：修复旧browser草稿decoy变量作用域；库存UNKNOWN案例由真实UI输入1000，网络边界只丢首个真实409回复，重放同key/body。新增真实PG fulfilment角色两店精确grant、普通签名MOCK OIDC登录、三语390px入口与reload零请求案例。没有提高超时或修改预期PG14。该浏览器案例尚未运行。
- 时区回归：严格导入表发现新模型依赖，补入真实`create-order-model`；未列出的依赖继续抛错，没有fallback stub。
- Contract/interface changes: 无生产HTTP、契约、迁移或ACL变更；只新增owned test mux的fixture角色控制。

### 本轮证据与检查（均为合并前工作树）

日志及每次命令／timeout／exit／源码和历史output哈希：`output/playwright/lc-u3-k3-p2/p2-*-20261012.{log,status.json}`；逐文件哈希及新untracked测试哈希：`output/playwright/lc-u3-k3-p2/resume-20261012/final-manifest.json`。最终tracked源码hash `879bc4da540b6cbe59e7ef5887bc2ae41f397ee1ba925c26df51328f922736b6`；运行中源码／历史output保持不变。新review测试在red→green期间逐字节未变。

| 命令／run | exit | 实际结果 |
| --- | --- | --- |
| `node --test tests/admin/create-order-review.test.ts` / `p2-resume-red-20261012` | 1 | 13项：11个目标断言失败、2正对照通过；原UNKNOWN拒绝死锁red另见保留的`p2-attempt-red.log` |
| `node --test --experimental-transform-types` + review/attempt/model/client/copy/timezone-ui六套 / `p2-pure-node-final-20261012` | 0 | **42/42**，组件／客户端MOCK证据，含实际源码和网络边界fixture |
| `TZ=UTC`、`TZ=America/Los_Angeles`，`node --test --experimental-strip-types tests/admin/timezone-ui.test.mjs` | 0、0 | 各**7/7**；新增依赖未登记的red见`p2-related-node-final-20261012.log` |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 | admin类型检查；随后只改测试／文档，产品TS输入未变 |
| `go vet -tags browser ./tests/foundation`；`node --check tests/admin/create-order-drawer-gate.mjs`；`git diff --check` | 0 | 结构检查；不代表PG／浏览器通过 |
| `bash scripts/dev/test-local.sh --list` | 0 | 82模式；本单元属于**`--browser-manual-order`**，没有独立create-order-drawer模式 |
| `bash scripts/dev/check-gates.sh` / `p2-final-gates-premerge-20261012` | 0 | 包含漂移检查；82模式与header／registry检查通过 |
| `go run ./scripts/dev/contractdrift` | 0 | **errors=0，warnings=362**历史baseline；Go278／contract146／BFF295 |
| `bash scripts/dev/test-node.sh` | 1 | 首批455：452通过、3失败；`team-bff`无法listen127.0.0.1（EPERM），两个deploy-prep测试遭Docker socket拒绝；整个门禁未完成 |
| 包含真实`create-order-bff.test.ts`的相关Node检查 | 1 | 本地HTTP listener遭EPERM；同轮TZ新增依赖错误随后修复为两TZ绿色；BFF网络验收仍BLOCKED |
| `bash scripts/dev/test-local.sh --browser-manual-order` | 1 | admin production build成功，Docker socket拒绝，未启动Go/PG/浏览器案例；**NOT_RUN**浏览器结果 |
| `git fetch origin r3/integration`；`git add -- <owned paths>` | 255、128 | 分别无法写`FETCH_HEAD`和`index.lock`：Operation not permitted；未尝试绕过权限或rebase |

当前证据级：**E3限于42个组件／客户端MOCK与本地静态门禁；真实PG／浏览器NOT_RUN，整个PR32不READY。** 源码只读独立复核未见指定范围P0/P1／隐私P2（E1），不能代替K3或真实浏览器。现有浏览器订单观察只证明merchant-manual总数和receipt绑定计数，不能单独证明所有checkout孤儿订单不存在。

### 允许写路径、保留与续接

- 仅`apps/admin/components/{CreateOrderDrawer,BuyerPanel,CommentStream}.tsx`，`apps/admin/lib/{create-order-attempt,create-order-model,create-order-copy}.ts`，`scripts/dev/test-node.sh`，`tests/admin/create-order-{attempt,model,review}.test.ts`，`tests/admin/create-order-drawer-gate.mjs`，`tests/admin/timezone-ui.test.mjs`，`tests/foundation/browser_create_order_drawer_test.go`，本DELIVERY及owned ignored证据。没有产品Go／SQL／migration／lockfile改动。`output/ext-agents/`未修改。
- 原七项WIP的`paused.patch` SHA256 `7cfd496df313c80bbec909d79b5ec236d931e4fe5e77e6be239052a9c37dd11b`及确切stash保持原样。修改前文件、当前完整文件与patch保存在`output/playwright/lc-u3-k3-p2/resume-20261012/`；没有丢失工作。
- BLOCKED：运行时`.git`只读、loopback监听／Docker socket访问受限。只有实际运行权限改变后才能提交、`git fetch origin r3/integration && git merge FETCH_HEAD`、保留两边意图解决冲突，`test-node.sh`必须保留两边所有entry，再完整复跑本地门禁。缓存`origin/r3/integration` SHA见manifest，不视为本端fetch结果。
- CI gates／待复跑：`--browser-manual-order`（legacy+drawer全部真实点击／PG），`LC_DRAWER_CALIBRATION=new-key-on-retry`应精确RED single-order；入口回归`--browser-live-console`、`--browser-inbox`，以及合并后pr-modes要求的模式。calibration本轮NOT_RUN；全foundation／G07、LIVE／生产发布未运行。

以下为保留的历史轮次记录；其中“暂停”和通过结果只属于各自日期／SHA。

## Scope-404复验 — PR36主干合并（2026-10-10）

- 本轮只复验 LCU2_404。合并提交/测试源码 **`463cd8f0bfffe7a2031d559b9f240a9c966a205a`** 包含 SCOPE-404 PR36 **`287e08aa6c694030333dd00797c0d3bbac15abc6`**。三条 LC-U3 P2 继续暂停；本轮不代表整 PR32 或待修代码验收。
- 七项暂停修改已逐文件备份，只暂存这七个 tracked 路径；确切 stash **`d1f7cc33821ae7c92ef8cf882b64f43f7ef82cb7`**，原 patch SHA256 **`7cfd496df313c80bbec909d79b5ec236d931e4fe5e77e6be239052a9c37dd11b`**。未暂存 owner 的 untracked 文件，不使用 autostash/pop0。报告提交 `a22a3a0b` 后已按该确切OID恢复：apply0、七个原始文件hash及整个patch逐字节相同，无冲突；stash/备份保留。恢复记录：`output/playwright/lc-u3-pr36-recheck/restoration.json`。
- `live-console.spec.ts`、Go harness、`test-local.sh` 与合并前逐字节相同；spec SHA256 `3df8c6fa47d7e559ff535ed7f146d3dec307ce4d973f66ab65906216b4d1182b`。原 LCU2_404 15s 等404和6500ms零轮询观察、Go780s模式边界均未改动；未过滤测试或开启 calibration。

| 现有原完整模式／本地检查 | 结果 |
| --- | --- |
| `env -u LC_BROWSER_CONSOLE_GREP -u LC_CONSOLE_CALIBRATION GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-live-console` | 0；Go PASS，Playwright **23 passed**；完整模式398.598s |
| `bash scripts/dev/test-node.sh` | 0；**1352** passes |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 |
| `bash scripts/dev/check-gates.sh` | 0 |
| `go run ./scripts/dev/contractdrift -json` | 0；errors0 |
| `bash scripts/dev/test-local.sh --list` | 0；registry保持主干布局 |

| LCU2_404视图 | 实际HTTP | privateRows / buyerPanel | sessionRetained | postLossReads |
| --- | --- | --- | --- | --- |
| all | 404 | 0 /0 | true | 0 |
| private | 404 | 0 /0 | true | 0 |

原 spec 先要求 comment/buyer可见，再真实PG撤 grant，等待实际网络404，验证全部私密状态清空、登录session不变。恢复grant后6500ms零读取，reload后重新出现comment；没有响应拦截代替平台修复。只读独立证据复核（E1，未复跑）确认这两份输出及源码未改，无本轮缺口。

证据 **E3 REAL_PG + Chromium/生产Next/BFF/Go，MOCK IdP/Graph**，不声称 LIVE。四个最终runner记录同一源码 hash **`d3659b938cdfa3e80a4cd32e549a2008ec809897daa181239d997dfd169d9ece`**、clean tracked status、历史output hash全程不变。原始日志及status：`output/playwright/lc-u3-merge-validation/pr36-{live-console-full,node,tsc,gates}.{log,status.json}`；browser：`output/playwright/run.2mqhP0B2/live-console-3145764804/grant-revocation-{all,private}.json`。独立索引、stash/逐文件备份及源码对照：`output/playwright/lc-u3-pr36-recheck/`。浏览器生成证据留在 ignored run roots，不新增tracked浏览器产物。

本轮不改产品、测试、路由、契约或超时；只合主干并记录验证。paused WIP 不属于测试源码或本轮验收，恢复后也不能用此结果声称WIP已通过。manual-order/新key calibration及三P2验证、全foundation/G07、真实provider/LIVE/部署均 NOT_RUN。提交不push、不回复GitHub；后续复核/推送由集成方执行。


## Historical batch — trunk merge + W3-U2 blocklist warning (2026-10-09)

- Source commit **`3a0f6aa1b36c246127986eca38342c0516544935`**, parents old K3-reviewed `f515dcdb` and trunk `4ff99766`. Branch `unit/lc-u3-create-order-drawer`; own worktree unchanged. No push/deployment. Parent model/effort is not exposed by this runtime; read-only explorers explicitly used `gpt-6.1-sol/high`.
- Trunk registry is byte-identical (82 modes, `--list` exit0), along with the 13 reviewed LC-U2a seams/tests. Merge preserves trunk privacy/cursor/parcel/import routes and reapplies the six U3 CommentStream drawer hooks.
- Warning consumes the existing **W3-05B boolean-only GET**, no contract change: regular BFF `/live-sessions/{sid}/claims/blocklist/check?bundle_id=...`, exact canonical query, HTTP200 `{restricted:boolean}`, no-store/no-referrer, bounded response. The SID comes from the server A13 claim snapshot or the current CommentStream and is only a store-level path prefix. Every known A15 bundle is checked under a common18s budget; auth failure always clears private state, non-auth lookup failure cannot claim clear. Three locales show restricted/unknown guidance; restriction never changes Quote, body, key, permission or submit eligibility.
- Own functional write paths: `apps/admin/components/{CreateOrderDrawer,BuyerPanel,CommentStream}.tsx`, `apps/admin/lib/{claims-request,create-order-client,create-order-copy}.ts`, `apps/admin/proxy.ts`, the root store catchall BFF, `tests/admin/create-order-{client,bff}.test.ts`, `tests/admin/create-order-drawer-gate.mjs`, `tests/foundation/browser_create_order_drawer_test.go`, and this unit's curated delivery/evidence. Merge-only paths preserve trunk versions. No product Go, SQL, migration, schema, lockfile or W3-U2 UI edits.
- Real fixture uses existing signed Meta ingress/consumer/intake in the trade store, validates APPLIED/ACCEPTED/facebook/quantity2, and executes real A14 link + restriction POST. Cleanup is limited to its six sessions/restrictions; first consumer stops before the differently-keyed comment harness. New drawer runs use ignored `output/playwright/create-order-drawer-*`.

| Local command | Exit/result | Raw evidence in `output/playwright/lc-u3-merge-validation/` |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0,1295 tests | `node-final.{log,status.json}` |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 | `tsc-final.{log,status.json}` |
| `bash scripts/dev/check-gates.sh` | 0,82 modes/headers | `gates-final.{log,status.json}` |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | `vet-final.{log,status.json}` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` | 0,legacy10 +drawer25,PG14/14 | `manual-order-green-ignored.{log,status.json}` |
| Same mode with `LC_DRAWER_CALIBRATION=new-key-on-retry` | 1 expected,only `DU3-single-order-en-1440`,PG15/14 | `manual-new-key-calibration.{log,status.json}` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-live-console` | 0 | `live-console-green.{log,status.json}` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-inbox` | 0,real chain/I11 | `inbox-green.{log,status.json}` |
| `bash scripts/dev/test-local.sh --browser-admin-shell` | mode0,24 cases +role/store/axe | `admin-shell-green.{log,status.json}` |

- E3 evidence binds every final run to unchanged source hash **`3a8ceac72784de4ee5e958501f299245e3f95387af60758e2a9c271c6d32a137`**, committed verbatim in `3a0f6aa1`. Curated metadata: `merge-warning-evidence.json`; actual browser/Go/PG with synthetic data and MOCK identity/Meta only. Independent read-only merge and source/evidence review found no newP0/P1; integrator's K3 delta re-review remains required.
- Warning browser: `output/playwright/create-order-drawer-3722851862/{result,click-ledger,pg-readback}.json` has25/25, six real BuyerPanel warning clicks (en/zh-TW/zh-CN×1440/390), seven same-key/body one-order/receipt retries, and PG14/14. Calibration root `create-order-drawer-4163076078` has25 cases and exactly the intentional changed-key failure: same body, two request orders/receipts, replay201, PG15/14.
- New BFF gate non-vacuity: remove only the GET grammar for the actual authenticated root handler → **RED405 vs200 (exit1)**, restore exact bytes → GREEN0. `blocklist-root-bff-{red,green}-final.log`, `blocklist-bff-red-green.json`. Initial tools-route probe and manual-bundle422 are setup failures, not causal product red proofs. One consumer wait also failed without a diagnostic; no unproven root cause is claimed. All final normal/calibration runs completed without worker/cleanup errors. Early submit-enable assertion was moved to the valid payment/Quote state; it remains required before the real201 submit.
- Trunk's old Admin shell mode overwrote tracked evidence; the protection wrapper correctly exited99 despite mode0. All88 generated files were preserved under `admin-shell-evidence/`, then historical output restored byte-for-byte to hash `9e02b5a43e89f1c334d1f6b3721f0366d4cda6db5db9f4bf68241ed92b4e8aac`; source stayed unchanged and git was clean. See `admin-shell-evidence-restore.json`. Earlier untracked drawer runs are preserved in `legacy-layout/`; no historical screenshots/logs are committed anew.

### CI gates

Changed browser spec/mode remains only `--browser-manual-order`: `create-order-drawer-gate.mjs` and its Go fixture; no added mode. Local hosts rerun: live-console, inbox, admin-shell. Run manual-order and its expected-red calibration in CI; retain **click-sweep and visual-lint**, NOT_RUN locally per the frozen brief. `node scripts/dev/pr-modes.mjs origin/r3/integration` selects53 modes due to shared BFF/proxy/forms; the complete exact list is in `merge-warning-evidence.json.pr_modes`. Remaining selected modes, full foundation/G07, SANDBOX/LIVE providers and deployment are NOT_RUN locally. Integrator re-reviews this delta and pushes.


## Prior addendum — ManualOrder P1 (4212512394)

- Implementation `d0422a6c`; real regressions `0d91f236`, `778de008`; base previous delivery `6aabe6b6`. This addendum supersedes the original claim that ManualOrder submission behavior is unchanged.
- Root cause: editing the live draft after UNKNOWN regenerated its request key/body. The real baseline `addendum-edit-unknown-red.log` failed only `DU3-manual-unknown-draft-edit`: two attempts, different key and body, HTTP201, two request-specific PG orders/receipts (store total15 instead of14).
- Shared `OrderAttempt` now retains its immutable snapshot after both ambiguous and definitive responses. `finish()` only records definitive settlement; only the explicit `startNew()` action clears the receipt for a fresh key. ManualOrder keeps editable draft fields separate; retries ignore draft edits and use the original request. Drawer keeps editing blocked for unresolved attempts. Both submit handlers and buttons reject a settled receipt until the merchant clicks the localized new-attempt control.
- ManualOrder HTTP200 replay accepts the server-redacted configured/null buyer link without treating a successful replay as UNKNOWN; HTTP201 remains strict. Existing regeneration recovers that link using the original submitted locale. The default result parser remains strict.
- **Final real browser GREEN:** `addendum-edit-unknown-green.log`, `browser/20261008T154854.154170000/result.json`: 25 cases, PG14/14, legacy10/10. The edited ManualOrder retry has same key/body, HTTP200, PG delta1, exact one order and receipt. Both real422 refusal subchecks prove editing cannot enable submit, explicit new-attempt enables it, and clicking that action sends no POST and creates no order.
- **Calibration:** `addendum-calibration-red.log` exits1 only for `DU3-single-order-en-1440`; all25 cases run, PG15 instead of14. The ManualOrder edited-draft regression remains GREEN during this drawer fault.
- **Final local commands:** `bash scripts/dev/test-node.sh`, `bash scripts/dev/check-gates.sh`, `pnpm --dir apps/admin exec tsc --noEmit -p .`, `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation`, `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` all exit0. `LC_DRAWER_CALIBRATION=new-key-on-retry GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` exits1 as required. Evidence: `addendum-{node,gates,typecheck,vet,edit-unknown-green,calibration-red}.{log,status.json}`.
- All final commands share unchanged source SHA-256 `091a4f21bac71315adf405441d5bb11e23520b26fd56342ed73a621dc0dcd7ea`, committed verbatim in `d0422a6c`. Node lifecycle red→green: `addendum-lifecycle-{red,green}.log`.
- E3, real Chromium/Next/BFF/Go/PG with synthetic data and MOCK identity/provider. Read-only source spotcheck found no additional money/state issue; K3 independent acceptance remains pending. Mode remains `--browser-manual-order`; no new mode, product Go or contract change.
- Test author configured `gpt-6.1-sol/high`, separate `unit/lc-u3-retry-tests` worktree, own paths limited to browser driver and Go fixture. Root implemented/reviewed product and ran every runtime gate. Temporary test worktree archived then removed; delivery worktree retained. No push.

## Original delivery evidence (historical, before this addendum)


- Branch: `unit/lc-u3-create-order-drawer`. Base: LC-U2a `9ce09803`; frozen brief `ba2bab32`. No trunk merge, push, PR or deployment performed.
- Tested implementation: `faac7bf2` (preceding integration `142a58c5`, API `97b1243d`, shared form `d00d04c7`). All final gates share source SHA-256 `16e8ac45852239ae02f80174697bfc84f96003e63459dd38e705124badb5e988`; each `*.status.json` records command, base HEAD, timeout, elapsed time, source hash and unchanged-source check. The five then-uncommitted changes were committed verbatim as `faac7bf2`.
- Implementer: Codex-3, GPT-6 runtime family (exact parent deployment/effort not exposed). Child explorers and writers configured `gpt-6.1-sol`, high effort except browser mapping medium. Child worktrees were isolated; no recursive delegation.
- Contract/interface changes: none; consumes live-console-v1 §5/§6/A15/A16 and remaining-quantity amendment. UI/BFF and Go browser fixtures only; no product Go, migration, schema or lockfile change.

## Implementation

- `apps/admin/components/ManualOrderFormFields.tsx` and `ManualOrderItemPicker.tsx` share existing fields. Legacy ManualOrder submission, result and link behavior remain intact.
- `CreateOrderDrawer.tsx:79` opens a native modal from BuyerPanel or a claimed comment. Only A15 explicit customer links prefill contact/delivery; original claim facts retain individual remaining quantities even when request SKU quantities aggregate.
- A15-ineligible/live-permission warning precedes confirmation. Catalog unit references are informational; the server Quote supplies all actual prices and totals. No client price fields, total computation or identity inference.
- Exact tools BFF resources: GET `inbox/order-prefill` with one canonical selector and POST `orders/for-buyer` with closed body/header rules. Tests execute real Request/auth/origin/CSRF/store helpers, replacing only upstream network.
- `create-order-attempt.ts:6` freezes request bytes/key; UNKNOWN locks edits and replays that same receipt. A coarse boolean guard survives hide/unmount without storing buyer data, body, key or link. First-dispatch 401/403 also retains the guard because A16 can reauthorize DM after order commit. Session loss clears drawer and parent private state.
- Result copy covers every allowed coded error in three locales; duplicate bundle gives only the existing-order action. A9 conversation binding must exactly match B1 `dm_session` capability. Payment URL remains a transient ref and goes only to native clipboard; replay-redacted URLs use the existing stable-key regenerate-link endpoint.

## Red → green evidence

- `api/red.log`: missing model before implementation; `api/green.log`: real-seam focused cases pass. Existing manual parser remains strict; only A16 accepts replay-redacted configured links.
- `attempt-red.log` → `attempt-green.log`; `postcommit-auth-red.log` → `postcommit-auth-green.log`: immutable retries, storage fail-closed and auth-after-commit protection.
- `copy-red.log` → `copy-green.log`: all coded errors/reasons and eligibility warning, all locales.
- `browser-missing-drawer-red.log`: legacy flow reached new test; absent drawer generated no A15 request. Initial setup failure is separately retained in `browser-baseline-red.log` (test fixture log-stream fd not ready; fixed by awaiting open). Neither is represented as a product success.
- `browser-drawer-green-1.log`: real creation, copy and duplicate flow passed; missing address control test IDs exposed and corrected without changing assertions.
- Normal final `browser-drawer-green-2.log`: **24/24 matrix cases; PG 13/13 orders**, plus direct signed-ingress comment trigger and six session-boundary purges; unchanged legacy **10/10** cases. Details/screenshots: `browser/20261008T152835.002287000/`.
- `browser-new-key-calibration-red.log`: exit **1**, exactly `DU3-single-order-en-1440` fails; all 24 cases run. Mutation preserves body but forwards a new retry key at the real HTTP edge: two attempts, two request-specific PG orders/receipts, replay status 201, PG total 14 instead of 13. Details: `browser/20261008T152955.378435000/result.json`. Mutation is opt-in test-only, no product calibration branch.

## Local gates

| Command | Exit | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | `root-node.log`, `.status.json` |
| `bash scripts/dev/check-gates.sh` | 0 | `root-gates.log`, `.status.json` |
| `cd apps/admin && pnpm exec tsc --noEmit -p .` | 0 | `root-typecheck.log`, `.status.json` (equivalent pnpm --dir invocation) |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | `root-browser-vet.log`, `.status.json` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` | 0 | `browser-drawer-green-2.log`, `.status.json` |
| `LC_DRAWER_CALIBRATION=new-key-on-retry GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` | 1 expected | `browser-new-key-calibration-red.log`, `.status.json` |

Evidence class: **E3, REAL_PG + real Chromium/Next/BFF/Go**, synthetic fixtures and signed MOCK OIDC/Meta only. Component/Node tests are MOCK, not substitutes for browser evidence. Representative 1440/390 screenshots inspected; full-page captures include the underlying page below the viewport-height native dialog. Independent source spotcheck corrected the selector/privacy/remaining-quantity findings; K3 acceptance remains pending.

## Modes/specs and CI gates

Changed browser mode: **`--browser-manual-order`**, `TestBrowserManualOrderLink` in `tests/foundation/browser_manual_order_link_test.go`, new `browser_create_order_drawer_test.go`, new `tests/admin/create-order-drawer-gate.mjs`. No new mode or fixture-only UI route. Existing legacy spec stays intact.

Integrator CI: `--browser-manual-order` plus its new-key calibration; `--browser-live-console`, `--browser-inbox`, `--browser-admin-shell`, `--browser-click-sweep`, `--browser-visual-lint` for affected hosts/shared fields. These additional modes were **NOT_RUN locally**; required manual-order mode was run at the owner's explicit request despite the general CI-only browser default. G07/full foundation, real provider sends, SANDBOX, LIVE and deployment NOT_RUN.

## Limits and handoff

- More than five prefill bundles is refused without truncation; open a single claim bundle.
- After an unresolved request loses private memory, no status-by-key endpoint exists. The current store/auth scope stays blocked; no new key is silently generated and no private retry journal is persisted. External reconciliation/session recovery is an operational limitation for review.
- Direct comment/bare bundle cannot infer a messaging identity and therefore uses catalog pricing with a visible warning.
- K3 deep review, push/PR and merge belong to the integrator. Merge trunk only after the integrator confirms LC-U2a merge. No migration, grant or schema action requested.

Cleanup: owned form/API/browser/UI temporary worktrees were archived under the main checkout `output/lc-u3-create-order-drawer/worker-archives/` and removed; their commits remain. Requested delivery worktree remains. All owned gate runners ended and test-local cleaned its fixture containers/ports.
