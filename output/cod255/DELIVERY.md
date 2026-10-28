# COD-255 — PR #285 implementation, CI acceptance pending

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
