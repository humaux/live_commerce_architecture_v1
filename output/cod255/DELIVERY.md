# COD-255 — PR #285 implementation, CI acceptance pending

## Relay 6: persisted amount readback repair (attempt 1 for this blocker)

Current-head authority: e798452b162338ffbd37dcde6e70527700b80327; required fetch/merge returned
Already up to date. Foundation run 40790612363/job 123174615029 passed. Gates run
40790612417/job 123177001415 failed only its home-COD member: the final PG readback
referenced nonexistent column `cod_collect_minor` (SQLSTATE 42703).
The job log was retrieved through the jobs logs API because `gh run view --log-failed`
refused while the workflow was in progress. Failed artifact 12862800730 was downloaded
with `gh run download`; its buyer log records all 12 COD-255 scenarios reaching PASS.
The tested synthetic merge SHA is 281c682c09b015a9281034eff9679c9027cb1611.

Schema authority is migrations/0107_home_cod.sql:61–65; checkout.Service.Get at
internal/checkout/checkout.go:517–527 projects the same stored total plus surcharge
as `cod_collect_minor`. The test now reads that persisted sum. Store/order/mode/state,
surcharge and exact collection amount predicates remain; no assertion or fixture is weakened.
No product UI/copy/import/layout change, migration or backend edit. Browser ownership
is regenerated for the changed Go gate source; dependency imports are unchanged.
Raw failure evidence is retained locally under output/cod255/relay6 (not committed).
New-head GREEN is PENDING; prior-head browser progress is not current-head acceptance.
Local tests/servers remain NOT_RUN. Humaux is available in this relay; fix/handoff and
the lane's own canvas element are recorded there. Integrator owns the CI completion relay.

## Relay 3: generated ownership repair (attempt 1 for this blocker)

Head ce9e96384cc568cd55e12bbcf6c93a03803401bf failed Foundation run 40761401812,
job 123074002991, at tests/ci/backend-coverage.test.mjs:274. The only failed assertion
was `CI-SPEED-3: generated browser ownership is current and covers the registry`;
the generator reported `browser ownership drift`. The previous wait-order gate passed.
Root cause: relay 2 changed home-cod-buyer.mjs but did not regenerate its recorded source hash.
Ran the card-prescribed static generator `node scripts/dev/derive-browser-ownership.mjs`
(exit 0, 53 modes), which changed exactly that one hash in the generated ownership file.
No generator, registry, mode, test, assertion, fixture, timeout, threshold or application source changed.
No new/renamed/deleted browser spec, page or route; the existing required integration merge
is 881c5ded343136fb51d41f35fba9a49a567ecdd8. No further base merge is needed for a hash-only repair.
This is static metadata generation, not local test execution or CI acceptance (E1 only).
New Foundation/Gates receipts will be recorded in the PR comment without an evidence-only push.
Humaux remains unavailable: no store/canvas/coord receipt claimed. No local tests or servers.

## Relay 2: targeted CI repair (attempt 1)

Implementation head e887218c851ab6a64194be655933cd467cea3ab5 failed Foundation run 40759941214,
job 123068822047, in tests/ci/gate-waits.test.mjs:18. The scanner identified two new waits in
tests/storefront/home-cod-buyer.mjs (old lines 228 and 262) awaited only after `await create.click()`.
If the click failed first, the outstanding wait could reject unhandled and hide the real failure.
Both response/request waits now join the real click in Promise.all, attaching rejection handlers before
the click settles. All assertions, fixtures, timeouts and gate rules are preserved. No spec/page/route
was added, renamed or deleted. Existing source headers are unchanged.
Raw failed receipt: output/cod255/foundation-implementation-failed.raw.log (local retained evidence).
Local validation: node --check tests/storefront/home-cod-buyer.mjs and git diff --check, exit 0 (E1 only).
No local tests or servers. New Foundation/Gates acceptance remains PENDING; no GREEN claimed.
Humaux tools remain unavailable: no store/canvas/coord receipt is claimed.

Role: UI worker dispatched by integrator; model: GPT-6 (exact runtime effort unavailable).
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/cod255-v2-ui.
Branch: unit/cod255-v2-ui; target: r3/integration; implementation base: 881c5ded343136fb51d41f35fba9a49a567ecdd8.
Only MOCK fixtures are authorized. No local tests/servers, production actions, merge, auto-merge or subagents.

## Root cause and implementation

The current option's surcharge previously authorized checkout while the parent still displayed an older quote due.
`checkoutInput` now rejects a missing/mismatched COD due. `refreshCodQuote` compares server facts and, when needed,
reads GET quotes/{id}; this endpoint already computes the current COD due. It checks identity, scope, expiry and
fee consistency before publishing the quote to CheckoutFlow. No client-calculated due is rendered or sent.
OrderFlow clears option and address confirmation during every option refresh. Option publication waits for the
matching server quote; failure leaves confirmation/placement disabled. Existing Reload recovers by requoting.
An intervening second fee change fails closed, rather than looping requests. Existing server surcharge checks remain.
No backend, contract, migration, pin, dependency, route, exemption or assertion is weakened.

## Evidence and acceptance gate

RED (E3 for the negative test only): head 881c5ded343136fb51d41f35fba9a49a567ecdd8;
Foundation run 40757493836 / job 123060219251, Node unit suites, exit 1:
`Missing expected exception: COD-255: reject old due with refreshed surcharge 0`
at apps/storefront/tests/purchase.test.mjs:377. Raw receipt retrieved with
`gh run view 40757493836 --log-failed --job 123060219251`; causal extract in red.log.
RED browser Gates run 40757493902 was still in progress when implementation began; no browser RED is claimed.

Implementation evidence: E1 syntax only, not acceptance. Local `node --check` on purchase.ts,
purchase.test.mjs and home-cod-buyer.mjs plus `git diff --check`: exit 0.
No local tsc: compiler/dependencies absent. OrderFlow stays at 852 lines, below its unchanged 854-line allowance;
only redundant header prose was compressed, preserving Purpose/Depends on/Used by and adding Invariants.

Required GREEN: this PR's Foundation and Gates at the implementation SHA, including existing home-COD browser
gate with real merchant settings changes in isolated PG. Cases cover up/down, initial quote/options race,
zero fee, failed refresh, disabled COD and cap, zh-TW/zh-CN/en and mobile/desktop. Inline/footer/order money
must match, request controls must block while reads are held, and order money must survive reload.
GREEN/PW/PG/race/independent review: PENDING, not PASS. Integrator relays run conclusions to the next session.
No local processes or fixtures were started. No generated dependency-map change is needed for TypeScript-only imports.

## Click ledger (CI assertions; results pending)

| Page | Control | Real operation | Expected visible result | Actual |
|---|---|---|---|---|
| Checkout | Delivery / Quote | click | Server quote and delivery form | PENDING CI |
| Checkout | COD mode | check | Matching server due inline and footer | PENDING CI |
| Checkout | Recipient/address | fill | Entered values available for confirmation | PENDING CI |
| Checkout | Confirm address | click | Disabled during refresh; explicit confirmation after new due | PENDING CI |
| Checkout | Create order | click | Real stale-fee refusal, then order using refreshed quote/due | PENDING CI |
| Order | Reload browser | reload | Same order and server collection amount | PENDING CI |

No new controls are added. Existing harnesses referencing OrderFlow.tsx/CheckoutFlow.tsx were located with rg;
the only new named import is on the existing purchase module in OrderFlow, with no compiling harness allowlist.
Existing CheckoutFlow render harnesses receive no new imports. No Figma writes or design acceptance claimed.

## Required-field parity

Both widths and all payment summary states use the same source components. Static source parity only;
actual mobile/desktop browser results remain pending. No fields are added, removed or renamed.

| Required field/fact | Desktop and mobile source | Confirmation/review/order state |
|---|---|---|
| Recipient name and phone | OrderFlow native inputs | Explicit destination confirmation; OrderDetails snapshot |
| Destination | OrderFlow address fields | Existing immutable order destination snapshot |
| Lines and base totals | CheckoutFlow quote lines/totals | OrderDetails server quote snapshot |
| COD fee | Current option, published after quote match | CodAmount fee; immutable order surcharge |
| Due on delivery | Server quote.cod_due_minor, inline and footer | Server order.cod_collect_minor, including reload |
| Payment timing | Existing three-locale COD copy | Existing order collection status copy |

## BOURDIEU

Q1–Q6 definitions and module owner records are UNKNOWN because Humaux is unavailable; no invented rubric or
owner preference is used. For each Q1/Q2/Q3/Q4/Q5: verdict UNKNOWN, no aesthetic or layout change made.
Task-experience interpretation: novice/expert refers only to recent checkout experience, never class/device.
Q6 aesthetics: UNKNOWN until owner. Behavior repair preserves the incumbent presentation and requires the
same explicit amount/address confirmation for every buyer.

## PROJECT UNDERSTANDING

Read current AGENTS.md and 架构.md §§0,1,2,26,27: separate evidence levels; Go/PG owns transactions;
TypeScript strict consumes server money; scoped worktree; author cannot self-accept.
Read issue #255 body: refreshed server due must agree with option before confirmation, never client summation.
Read contracts/buyer-http-v1.md quote projection amendment: GET and POST project current server-computed due;
contracts/storefront-v2.md COD rules and begin_hold source: expected surcharge is checked against current settings.
The current task card is authority for CI-only verification and one implementation push. No Humaux owner decisions
or rejected-solution enumeration were available; no new UX decision, setting or architecture is introduced.

## MOBBIN → EAGLE

No visual reference used or imported for this state-consistency repair. Module Eagle folder/candidate IDs are
UNKNOWN (Humaux unavailable); no new pattern requires a Mobbin query. Existing controls/layout remain.
No external reference image, brand or product fact is copied. No Eagle API call made.

## FIGMA OFFICIAL GUIDANCE

No Figma nodes, layout, component, variables or prototype are changed, so no new guide application is claimed.
output/ux-audit-ref/FIGMA-GUIDE.md is absent in this worktree. Copy lint/density probe and Figma readback:
NOT_RUN, no changed frames. Official guidance provenance is UNKNOWN; no substitute aesthetic authority invented.

## Three-language copy

No visible string is added or changed. Existing codCopy/orderCopy/purchaseCopy supply zh-TW, zh-CN, en;
the regression browser cases assert the main checkout in all three. New-string translation table is empty:

| zh-TW | zh-CN | en |
|---|---|---|
| No new visible strings | No new visible strings | No new visible strings |

## Scope, risks and NOT_RUN

Implementation paths: apps/storefront/lib/purchase.ts, components/OrderFlow.tsx, components/CheckoutFlow.tsx
under apps/storefront. Same-module root-cause scope is pre-approved in the current task; disclose in PR body.
Additional delivery paths: output/cod255/red.log and DELIVERY.md. Prior tests-only push also generated
output/ci-select-backend-browser/browser-ownership.json; no new browser spec/page/route inventory change here.
Not touched: other apps/backend, migrations/contracts/OpenAPI/go.mod/locks, R2 pin, dispatcher table, drift baseline.
NOT_RUN: local tests/servers/builds, local tsc, provider SANDBOX/LIVE, production, Figma, independent acceptance.
Humaux search/store/coord/canvas/code-index tools are absent: no store/claim/canvas receipt is claimed.
Risk: transient quote/options inconsistency intentionally requires existing Reload recovery; no automatic write retry.

## Resume after #306 / #313 (2026-10-29)

Authority: integrator resume instruction for PR #285; integration bf174df6b8a2c23e44dc8290a1baddf926149c6e contains readiness fix #306 and HTTP error mapping #313. Starting local head 5e9234555; remote PR head fadc622f1eef85129021575529a320bdc6955ce7.

Merge resolves CheckoutFlow/OrderFlow conflicts by preserving both the incoming CVS brand-switch draft/quotation props and COD-255 onQuote/refreshCodQuote behavior. Both httperror files match integration verbatim. Browser ownership is regenerated from the merged source, never hand-edited. No new imports beyond the existing card implementation and incoming baseline; component harness references inspected. No changes to assertions, fixtures, thresholds, routes, contract baselines or dispatcher lists. Existing RED remains output/cod255/red.log (Foundation 40757493836/job 123060219251).

Current-head acceptance is PENDING CI; author assessment E0. No local tests or servers. Storefront tsc NOT_RUN: this worktree has no local node_modules/.bin/tsc. Whole-merge whitespace check reports only incoming historical logs under output/cvs-pick-impl and output/people-v2-impl; those upstream evidence files are preserved. Card diff whitespace and node syntax checks are recorded in the PR relay comment. No new visual/UI controls, locale strings or Figma deliverables. Existing process/parity disclosures above remain scoped to the original implementation.

This relay adds no out-of-card authored source repair: only existing card components, DELIVERY.md and generated browser ownership. Upstream-only paths arrive through the requested base merge. No backend/package/migration/lockfile/pin edits authored. Rollback: revert the relay merge with its first parent on this task branch via the same PR process. CI results will be relayed by the integrator into a fresh session; no local background task or server remains.

## RELAY 3 synchronization (2026-10-29)

Current instruction explicitly requests merge, ownership regeneration and push. Starting head
62e13c814b180af0773b498001ab98b3df70aee1; fetched integration
aa4c5eb65724fe14afa7e10a9aaa9cf140a1517f (git rev-parse origin/r3/integration).
The requested merge is conflict-free. #306 and #313 were already in the starting head;
both internal/httperror files still match integration. This relay authors no product source repair.
Incoming #221 paths, including its contract/baseline changes, are upstream-only merge content.

Re-read PR checks and failed required job 123435870227 in Gates run 40865131138.
The originating failure is bundle:07 job 123419600464, TestBrowserTaiwanCvs/MOCK admin half.
Fresh official artifact 12894619037 (gate-browser-cvs) is retained locally in relay18/cvs.
Its playwright.log reports the unchanged 180000 ms total test deadline, with openOrders
at tests/admin/taiwan-cvs.spec.ts:54 called from the locale/viewport print loop at :168.
Browser closure follows the deadline; the root cause of the navigation delay is UNKNOWN.
Under the explicit scope STOP rule, no admin/shared-harness change, timeout increase,
fixture/assertion weakening or targeted fix was attempted. The merge push is only the
explicitly requested baseline synchronization; it does not claim to fix this external blocker.

Historical causal RED remains Foundation 40757493836/job 123060219251 (red.log).
Starting-head Foundation 40865131209/job 123417850037 passed; starting-head Gates
40865131138 failed. Neither is acceptance of the new merge head. New-head CI is PENDING.
Ownership generator exit 0 (53 modes); regenerated output matches the automatic merge.
Card-relative diff whitespace, node syntax for home-cod-buyer.mjs and ownership generator,
and httperror integration parity checks exit 0 (E1 only). No new imports or UI strings.
Existing UI process, parity, locale and click-ledger disclosures above remain applicable.

NOT_RUN: local tests/servers/builds/tsc, provider SANDBOX/LIVE, production, Figma,
independent acceptance. Humaux tools are absent; no store/coord/canvas receipt claimed.
Integrator owns CI completion relay. No server or test process was started locally.
Rollback point is the starting head; revert the merge with first-parent mainline through
the same task PR if required. No integration/main push, PR merge, auto-merge or deployment.
