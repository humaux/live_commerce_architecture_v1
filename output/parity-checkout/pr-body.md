Checkout now opens one signed review task with recipient/destination facts and a fixed final-payable bar. The details link and contextual payment help disclose optional information; primary/disabled actions use solid navy/light grey, country codes are absent from delivery labels, and an in-flight checkout says Submitting order. Re-quote acknowledgment, server-money authority and UNKNOWN read-only recovery remain.

Authority: ux-checkout/DELIVERY.md current **Checkout controls polish · 2026-10-29** (48 desktop/mobile frames), Figma iJ9lDdFIXgTlKkt5Tvrc0X / page 4:8829. Fresh Fleet Bridge read-only context 1429:23484 and screenshot 1429:23435; no design writes.

RED: tests-only head `2318e7c9beda70cb52872703cc1ea16988df9668`, [Gates run 41051681920 / job 124052345726](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41051681920/job/124052345726), synthetic merge `dbf3b7edb274991b504adfcec564215ff2bb7ed4`. New signed-frame assertions actually failed: heading/final label absent, dashed disabled edge, teal instead of navy, details/help absent, submitting/UNKNOWN copy mismatch. [Artifact 12976765423](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41051681920/artifacts/12976765423) contains the buyer log, 76-row parity ledger (68 FAIL), layout and screenshots. Foundation 41051681909 / job 124049017123 passed on that RED head; historical success is not implementation acceptance. First relay's colon-filename upload blocker was repaired without changing assertions.

GREEN: pending this implementation head's own CI. Exact run/job IDs will be recorded in the PR handoff comment. Author verdict E0; local static evidence E1; no completion/E4 claim.

Static verification: tsc --noEmit --incremental false exit 0 against current worktree i18n source; node --check on every changed MJS exit 0; browser ownership generate + --check exit 0; check_packet.py exit 0 (packet structure only); git diff --check exit 0. A first tsc read the main checkout's stale built i18n dependency; final static config resolved current source. Temporary bindings/config removed. No local tests/servers. The next-startup checker, baseline, dispatcher registry, contracts, migration pins and lockfiles are untouched. RELAY4 restores two original quotation adapter assertions that implementation127f868b had replaced; their original order and text are preserved.

Test maintenance preserves money/refusal/persistence/UNKNOWN/idempotency assertions. Existing lifecycle callers now click View details to inspect optional fields/secondary form action, and exact copy follows the current signoff. The new native-select check waits for async rendering. COD parity uses two unchanged NT$12.50 units to meet the existing whole-TWD requirement (one unit correctly did not offer COD); no eligibility/price/cap/threshold/exemption changes.

Outside explicit source write paths, for integrator review:
- `apps/storefront/components/OrderFlow.tsx`: same-app scope pre-approved by this card; collapse confirmed form/optional fields, pass disclosure state, report actual checkout submission, reveal invalid-email errors. No transaction/API change.
- `tests/storefront/shop-helpers.mjs`: shared real-click path for expanded checkout details. Direct checkout browser callers updated together: ads-attribution, browser-gate, buyer-comms-gate, claim-checkout, cvs-buyer, cvs-pap-only, cvs-picker, home-cod-buyer, offline-buyer, order-gate, privacy-buyer, promotions-gate, shop-gate and stripe-browser (all `.mjs`). CVS picker source remains untouched.
- `output/ci-select-backend-browser/browser-ownership.json`: required generator output.
- `output/parity-checkout/{DELIVERY.md,red.log,source-hashes.json}`: required delivery/RED/static receipts. Other downloaded artifacts stay local and on GitHub.

NOT_RUN: implementation CI GREEN until receipts, independent side-by-side review/E4, local runtime tests/servers (prohibited), remaining 32 pickup/CVS signed-frame comparisons, production/PSP/carrier execution, Humaux store/canvas/coord (tools unavailable). No merge/auto-merge/deployment.

Parity sheet: real /{locale}/checkout at 1440×900 or 390×844; frame filenames use hyphens because GitHub artifacts reject colons. Only TW home online/COD matches these 16 signed frames; zh-CN/en home gets locale filenames, never mislabeled as pickup. Click ledger, screenshot manifest and clearance measurements are in the browser artifact. The independent reviewer must compare the actual screenshots to these DELIVERY exports; screenshots alone do not prove parity.

| Signed frame | Real-route CI screenshot | Current DELIVERY export |
|---|---|---|
| 1429:23435 | parity/1429-23435.png | ux-checkout/shots-polish/get_screenshot-1429-23435-1793204737920-0.png |
| 1429:23484 | parity/1429-23484.png | ux-checkout/shots-polish/get_screenshot-1429-23484-1793204738001-0.png |
| 1429:23533 | parity/1429-23533.png | ux-checkout/shots-polish/get_screenshot-1429-23533-1793204738127-0.png |
| 1429:23577 | parity/1429-23577.png | ux-checkout/shots-polish/get_screenshot-1429-23577-1793204738209-0.png |
| 1429:23621 | parity/1429-23621.png | ux-checkout/shots-simple/get_screenshot-1429-23621-1793199651952-0.png |
| 1429:23661 | parity/1429-23661.png | ux-checkout/shots-simple/get_screenshot-1429-23661-1793199652036-0.png |
| 1429:23701 | parity/1429-23701.png | ux-checkout/shots-simple/get_screenshot-1429-23701-1793199652158-0.png |
| 1429:23741 | parity/1429-23741.png | ux-checkout/shots-simple/get_screenshot-1429-23741-1793199652240-0.png |
| 1429:24143 | parity/1429-24143.png | ux-checkout/shots-polish/get_screenshot-1429-24143-1793204738772-0.png |
| 1429:24194 | parity/1429-24194.png | ux-checkout/shots-polish/get_screenshot-1429-24194-1793204738853-0.png |
| 1429:24245 | parity/1429-24245.png | ux-checkout/shots-polish/get_screenshot-1429-24245-1793204738995-0.png |
| 1429:24291 | parity/1429-24291.png | ux-checkout/shots-polish/get_screenshot-1429-24291-1793204739084-0.png |
| 1429:24337 | parity/1429-24337.png | ux-checkout/shots-simple/get_screenshot-1429-24337-1793199653583-0.png |
| 1429:24379 | parity/1429-24379.png | ux-checkout/shots-simple/get_screenshot-1429-24379-1793199653663-0.png |
| 1429:24421 | parity/1429-24421.png | ux-checkout/shots-simple/get_screenshot-1429-24421-1793199653784-0.png |
| 1429:24463 | parity/1429-24463.png | ux-checkout/shots-simple/get_screenshot-1429-24463-1793199653862-0.png |

RELAY 3: implementation head `127f868b` Foundation `41056857665` / job `124066455551` failed BO03 VM harness (`confirmCheckoutAddress is not defined`, 499/500 Node tests passed). Repair head `a10d46667152c994d76a557430699f6ae5947c31` loads the actual shared confirmation/disclosure helper source in the VM. Original single-pointer assertion retained; idle/disclosure order assertion added. No UI/product source change or weakened test. First targeted attempt on this blocker. Additional same-module path requiring integrator review: `apps/storefront/tests/order-gate-timeout.test.mjs`; filtered CI receipt `output/parity-checkout/relay3-foundation-failure.log`. Static node syntax / ownership generate+check / diff check exit0, generated ownership unchanged; local runtime tests prohibited. GREEN pending new exact-head CI. Humaux available in this relay; prior unavailability statements above are historical.

RELAY 4: exact-head Foundation `41058432128` / job `124071740737` failed frozen adapter parity: missing original `Items in this quotation` assertion (71/106). First targeted repair restores both original assertions verbatim/in order and reuses the existing localized quotation heading only inside explicitly expanded details. The default signed review still hides this section; `View details` control, default legacy-absence and safety assertions stay unchanged. Add a real details click after zh-CN navigation before existing item/heading assertions. Browser ownership regenerated and checked. No new outside-card source path; existing browser-gate/generated/evidence exceptions above still require integrator review. Native filtered receipt `output/parity-checkout/relay4-foundation-failure.log`; Humaux fix stored and own relay canvas updated under lock. E1 syntax/ownership/whitespace only; author E0, own-head GREEN pending. No local tests/server. Exact final SHA and WAIT_RUN IDs will be posted in a PR comment.

FIX1 — directed repair of Gates 41063668416 (2026-10-30), head `de70834327901121f9f778ef5605c18b19f87cb9`:
- Merged `origin/r3/integration` base `3ea359f796d36153bb4b8aa7259e4e0c27676b86` first; one batched fix push. Full native failing-job → assertion → evidence → class table: [FIX1.md](https://github.com/humaux/live_commerce_architecture_v1/blob/de70834327901121f9f778ef5605c18b19f87cb9/output/parity-checkout/FIX1.md).
- Three sweep shards and four visual shards share an obsolete hidden `create-order` setup. They now click the signed fixed action. The four visual artifacts have zero blocking findings and 2/2/1/1 missing order shots; every route/journey/screenshot requirement stays.
- Other tests use signed Back/chain labels, navy token and real details disclosure before optional quotation/history/address inspection. CVS reads the actual selected delivery key before quote navigation unmounts the select. Existing business assertions are retained; frozen gate-adapter assertions remain verbatim/in order. No threshold, fixture, exemption or gate changes.
- Product accessibility fix: the fixed action reuses the existing localized address-confirmation description; new parity assertions cover all three locales/two viewports. No new visible strings, dependencies, money/transaction logic or Figma changes.
- E1 static only: current-source storefront tsc; 10 MJS/TS node syntax checks; generated ownership + final --check; packet structure; whitespace all exit 0. No local tests/server. File hashes and filtered native evidence committed. Exact-head GREEN and independent acceptance remain pending; final WAIT_RUN IDs are in the PR comment. If the same assertion remains RED after FIX1: BLOCKED, no third approach.

FIX1 files outside the original explicit source paths (integrator review, directed buyer-callers only):
- `apps/storefront/components/OrderFlow.tsx`: same-app pre-approved root accessibility fix.
- `tests/e2e/deal-loop.spec.ts`, `tests/e2e/live-tools.spec.ts`: signed checkout action/token/label paths; retained live-price/order checks.
- `tests/ui/click-sweep.mjs`, `tests/ui/visual-audit.mjs`: shared signed checkout order setup only; no unrelated app logic.
- `tests/storefront/claim-checkout.mjs`, `tests/storefront/cvs-buyer.mjs`, `tests/storefront/cvs-picker.mjs`, `tests/storefront/order-gate.mjs`, `tests/storefront/shop-real-gate.mjs`: direct buyer checkout callers; existing tests preserved.
- Generated `output/ci-select-backend-browser/browser-ownership.json`; card evidence `output/parity-checkout/{FIX1.md,fix1-failure-evidence.log,DELIVERY.md,red.log,source-hashes.json}`. Integration merge inherits `deploy/docker/storefront.Dockerfile` and `deploy/scripts/smoke.sh`; this fix does not edit them.

NOT_RUN for FIX1: own-head Foundation/Gates GREEN, independent E4, local runtime tests/servers (prohibited), provider/production execution, remaining 32 pickup/CVS signed-frame comparisons, Humaux search/store/canvas/coord/code-index (tools unavailable). Author assessment E0; static E1. No integration/main push, merge-to-integration, auto-merge or deployment.

FIX1 RELAY 1 (2026-10-30): the concurrent integration merge blocker is resolved on head `7351f892764b0b61b5c9088239ba34dc72adfa7c`. Merged integration `2f61bf86673b3ebf4acbd3ee0ecacd54d37cf46f`; native PR API reports MERGEABLE. Source conflict resolution keeps both the captured checkout delivery key and integration's signed CVS parity real-click exercise. Ownership regenerated and checked; no assertions removed or weakened. Two node syntax checks and whitespace check exit 0 (E1 only); local tests/servers prohibited. No new outside-card source path; inherited CVS product/i18n files are integration changes, not new lane edits. Full classification plus relay details: `output/parity-checkout/FIX1.md`. Humaux fix stored, code index submitted; code-memory link pending because index lookup did not find the entity. CI and independent acceptance remain pending.

WAIT_RUN Gates: [41082084811](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41082084811) — queued on exact head.
WAIT_RUN Foundation: [41082084746](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41082084746) — queued on exact head.
Integrator relays conclusions in a fresh session; no continuing poll and no third assertion-fix approach. No merge-to-integration, auto-merge or deployment.

🤖 Generated with Codex (dispatched by Claude Code integrator).

