# PARITY-CHECKOUT — implementation, awaiting own-head CI

Current handoff: directed **FIX1** is documented in [FIX1.md](FIX1.md), with every failing Gates 41063668416 job classified from native artifacts. Previous relay sections below are historical. FIX1 has E1 static evidence only; its exact-head CI and independent review remain pending.

Status: E1 static structure/type/syntax only for implementation; author assessment E0. Causal tests-only RED confirmed by GitHub CI (E3 bounded to RED head); GREEN and independent parity acceptance pending. This is not completion.

Authority: dispatched card; ux-checkout/DELIVERY.md current **Checkout controls polish · 2026-10-29**, SHA-256 29321896334a674f1a3c2cf7a4ffcef168b533f3fce1ee3386005748552d3409. Fresh Fleet Bridge read-only context 1429:23484 and screenshot 1429:23435 are in relay2-figma/. No Figma writes. Current repository code/contracts govern transaction behavior; historical output is used only for the explicitly assigned signed design.

Role: dispatched UI implementer; runtime GPT-6 family, exact model/effort not exposed. Assigned branch unit/parity-checkout, worktree parity-checkout, tests-only base/head 2318e7c9beda70cb52872703cc1ea16988df9668. No delegation, merge, auto-merge or production work.

Changes: one review heading and recipient/destination card, optional details, final server payable and contextual help in the fixed bar. Keep native payment choices and re-confirmation; show submitting only during the checkout request. UNKNOWN remains read-only, withholds payable and preserves original intent/key. Expanded details retain existing optional email, consent, promo and lifecycle controls. Shipping labels omit raw country codes. Shared primary/disabled tokens are navy/solid light grey. Form errors reveal the editable fields. Legal/language footer remains reachable below the review viewport with measured bar clearance.

## UI PROCESS — Bourdieu

Q1: one review task with labelled disclosure, runtime comprehension UNKNOWN. Q2: optional details remain collapsed, repeat-buyer efficiency UNKNOWN. Q3: server money/contact/destination/timing always required. Q4: CVS internals owned by PARITY-CVS, unchanged. Q5: real desktop/mobile clicks required in CI; implementation result pending. Q6: aesthetics UNKNOWN until owner. Novice/expert means recent task experience, never class/device.

## UI PROCESS — Project understanding

Read 架构.md §§0/1/2/26/27 (evidence limits, single server transaction authority, store/buyer boundary, assigned worktree, independent acceptance); contracts/buyer-checkout-v1.md (immutable scoped quote/destination + idempotent durable receipt), buyer-checkout-options-v1.md (server-qualified delivery/payment), OrderFlow/CheckoutFlow and checkout component boundaries (existing recovery and COD guard preservation). Current owner card overrides generic auto-merge/delegation defaults: no merge/auto-merge/deployment, no local tests/servers.

## UI PROCESS — Mobbin → Eagle

Fresh read-only Eagle module folder MVQL9YPJVZ5FT returned 18 items, receipt eagle-list.json. Reuse signed handoff's MVNM2RJ3M6VBW (Thrive Market: grouped contact/destination and final-amount anchor) and MVOVHC45BTJIX (Selfridges: stable commitment area). No images/brands/product facts copied; no new design or Mobbin query needed.

## UI PROCESS — Figma official guidance

Read ux-checkout/output/ux-audit-ref/FIGMA-GUIDE.md: G1 auto layout maps to responsive grouping/fixed-bar clearance; G2 variables maps to shared storefront navy/disabled tokens; G3 reuse checkout components and existing native radios; G4/G5 real-click disclosure and explicit uncertainty/reconfirmation branches; G6 route/state/viewport artifact mapping; G7 existing signed design reuse. No prototype changes. These references do not constitute runtime acceptance.

## Desktop → mobile and review-state fact checklist

| Required fact | Desktop and mobile gate | Default/re-confirm/submitting/UNKNOWN |
|---|---|---|
| Destination including full address | review contains server destination city/line1 | Default/refresh/submitting/UNKNOWN assertions; re-confirm retains existing review |
| Recipient name | review contains server recipient_name | Same as above |
| Phone | review contains server phone | Same as above |
| Existing payment radio | real click; checked state and reload | Writable default/re-confirm; UNKNOWN read-only |
| One final total | exact server quote total/cod_due in footer, one visible instance | Re-confirm updated response; UNKNOWN withholds payable as existing contract requires |
| Payment timing | locale/method-specific footer copy | Original method retained; no new payment method |
| COD fee/changed-amount acknowledgment | original CHECKOUT-V2 assertions retained; separate new-total click | No client money authority; no unsafe retry |

## Visible copy — signed review and disclosures

| zh-TW | zh-CN | en |
|---|---|---|
| 返回 | 返回 | Back |
| 電話 | 电话 | Phone |
| 宅配 | 宅配 | Home delivery |
| 核對訂單 | 核对订单 | Review your order |
| 最終應付 | 最终应付 | Final amount due |
| 查看明細 | 查看明细 | View details |
| ? | ? | ? |
| 付款時機說明 | 付款时间说明 | Payment timing help |
| 送出後線上付款 | 提交后在线付款 | Pay online after placing the order |
| 貨到付款 | 货到付款 | Cash on delivery |
| 送出訂單 | 提交订单 | Place order |
| 正在送出訂單 | 正在提交订单 | Submitting order |
| 確認訂單狀態中 | 正在确认订单状态 | Checking order status |
| 確認新金額 | 确认新金额 | Confirm new total |

Payment explanations retain existing COD rules (cash to delivery personnel, no online charge):

| zh-TW | zh-CN | en |
|---|---|---|
| 送出訂單後，依訂單頁指示完成付款。 | 提交订单后，按订单页提示完成付款。 | After placing the order, follow the payment instructions on the order page. |
| 包裹送達時付現金給物流人員，網站不會先扣款。 | 包裹送达时付现金给物流人员，网站不会先扣款。 | Pay the delivery driver in cash when the parcel arrives. The site does not charge you beforehand. |
| 到選定門市取貨時付款，不會線上扣款。 | 到所选门店取货时付款，不会线上扣款。 | Pay when collecting at your selected store. No online charge. |

Entry-form/expanded-detail controls continue using the three-language purchase-copy, order-copy, bank-transfer-copy, cod-copy, cvs-copy and privacy-copy sources; no address/product/amount fact is translated or synthesized. Strings now live in packages/i18n/src/checkout.ts; existing entry-form, consent, order and server-option labels retain their locale sources.

## Parity sheet

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

This home-only fixture covers TW online/COD 16 signed frames; zh-CN/en home variants receive additional locale-named screenshots, never mislabeled as pickup. Remaining 32 pickup/CVS signed-frame comparisons NOT_RUN. Screenshots are viewport captures of real routes, not gallery frames. CI records actual per-control results in parity-checkout-click-ledger.json. No browser results asserted before receipt.


## CI RED / verification / limits

Tests-only head 2318e7c9beda70cb52872703cc1ea16988df9668: Gates **41051681920**, browser-home-cod job **124052345726** failed on the new signed assertions, not compile/startup. Synthetic merge dbf3b7edb274991b504adfcec564215ff2bb7ed4. Downloadable artifact **12976765423** contains the raw buyer log, 76-row parity ledger (68 FAIL), screenshot manifest and layout measurements. Existing checkout-v2 ledger had 19 rows and zero failures. Foundation run **41051681909**, job **124049017123** passed on the RED head; that is not implementation acceptance. See red.log and GitHub artifact; relay1 upload failure is historical.

The COD parity setup initially used one NT$12.50 unit, correctly excluded by the existing whole-TWD rule. It now buys two unchanged-priced units like the existing COD gate. No fixture pricing/eligibility/caps/threshold/exemption changed. Delivery-option read now waits for the asynchronously rendered select before checking labels. Older browser callers explicitly click the signed details disclosure to inspect optional fields/secondary form action; old money, refusal, persisted choice, UNKNOWN and idempotency assertions remain. Superseded timing/help expectations follow current signed copy/accessible names. RELAY4 restores the frozen quotation-heading expectations within explicitly expanded details; that section stays hidden on the default signed review screen.

Local static only: source TypeScript --noEmit --incremental false exit 0; node --check on every touched MJS exit 0; browser-ownership generate + --check exit 0; python3 scripts/check_packet.py exit 0 (packet structure only); git diff --check exit 0. First tsc resolved the read-only main checkout's stale built i18n package and reported missing existing cvsPickerCopy. Final tsc mapped @live-commerce/i18n directly to this worktree's current source; no CVS/source/dependency repair. Temporary config/symlinks cleaned afterward. source-hashes.json binds static evidence to files. No local runtime tests or servers.

Component-import harness search: only source-reading COD tests and legacy ledgers reference changed TSX filenames; no compiling component harness exists to amend. Header comments preserved. The next-startup checker, baseline.json, route-table-dispatchers.json, migrations, API, backend and lockfiles are untouched. RELAY4 corrects the earlier implementation's two replaced adapter assertions; the original quotation assertions are restored verbatim and in order. No Go dependency edges changed, so depmap regeneration unnecessary.

Outside explicit source paths: apps/storefront/components/OrderFlow.tsx (same-module controller: collapse confirmed form, pass details state, report submitting, reveal invalid email); tests/storefront/shop-helpers.mjs (real-click expanded-detail paths for existing checkout callers); generated output/ci-select-backend-browser/browser-ownership.json. Direct checkout browser callers under tests/storefront/ updated together, including CVS lifecycle callers, without touching CVS picker implementation. Generated ownership and card output/red.log/DELIVERY/source-hashes are authorized evidence artifacts. All are listed in the PR for integrator review.

NOT_RUN: implementation CI GREEN, local runtime tests/server (prohibited), independent screenshot parity/E4, remaining 32 pickup/CVS frame comparisons, production/payment-provider/carrier execution, Humaux store/canvas/coord (tools unavailable). No temporary process, server/container/port or dependency symlink remains. CI wake-up owner is the integrator; final PR comment records exact WAIT_RUN IDs, with no evidence-only follow-up commit after GREEN.

## RELAY 3 — Foundation harness repair (2026-10-30)

Current native receipt: PR351 implementation head `127f868bafa9ccabca06b63d342b1379ddbd8e5f`, Foundation `41056857665` / job `124066455551` failed BO03 with `confirmCheckoutAddress is not defined` (499/500 Node tests passed). After the browser call site adopted the shared confirmation helper, its VM harness lacked that binding. The harness now executes the actual helper/disclosure source, retaining its single-pointer focus-scroll assertion and adding the expected idle/disclosure sequence. Repair attempt 1/2 on this blocker; product source unchanged. `relay3-foundation-failure.log` preserves the native filtered failure.

Additional same-module path for integrator review: `apps/storefront/tests/order-gate-timeout.test.mjs`. Static `node --check`, browser ownership generator plus `--check`, and `git diff --check` exit 0 (E1); ownership output unchanged. No Go edges changed. Local runtime tests/server remain prohibited. This is not GREEN or independent parity acceptance (author E0). Humaux is available this relay: fix stored and own integrator-sub-parity-checkout canvas updated under coord lock; earlier unavailable statements describe prior sessions only. Existing causal RED and parity mapping above remain.

## RELAY 4 — frozen quotation assertions (2026-10-30)

Native Foundation `41058432128` / job `124071740737`, exact head `a10d46667152c994d76a557430699f6ae5947c31`, failed `tests/ci/next-startup.test.mjs:284`: original `Items in this quotation` heading assertion missing (71/106 assertion subsequence). Filtered raw receipt: `relay4-foundation-failure.log`. This is an implementation regression, not causal tests-only RED.

Root cause: implementation `127f868b` replaced two frozen `browser-gate.mjs` heading assertions with `View details`. Attempt 1/2 restores both originals verbatim/in order. The expanded quotation heading reuses `purchaseCopy.lines` in all three locales; the `View details` control and default hidden section remain. Language navigation resets disclosure, so the existing zh-CN item/heading assertions now follow a real details click. The new default legacy-absence, money, UNKNOWN and submitting assertions are unchanged. No new settings, dependencies, imports, contract or baseline changes.

Static E1 for this repair: `node --check tests/storefront/browser-gate.mjs`, card-required browser-ownership generator and `--check`, `git diff --check` exit 0. Ownership generated output and the three changed-file hashes are refreshed. Earlier typecheck belongs to the earlier files; typecheck/runtime acceptance for this repair remains pending CI. No local tests/server, Figma write, merge or auto-merge. No new outside-card source path; existing `tests/storefront/browser-gate.mjs` and generated/evidence exceptions remain for integrator review. Humaux lookup/store and own relay canvas were available and used under lock; no parent canvas overwritten. Final head and WAIT_RUN IDs go into the PR comment. Author E0, GREEN/E4 pending.

## FIX1 — directed failure repair (2026-10-30)

Merged integration base 3ea359f79 before editing. [FIX1.md](FIX1.md) inventories 13 failed execution jobs plus receipts/aggregate/required, including the WebKit job absent from the supplied FAILS list. Four visual shards have zero blocking findings and respectively 2/2/1/1 missing order shots caused by the old hidden-button setup. Sweep uses the signed fixed action. Other cases retain their original business assertions and click the signed Back, delivery, details and submit controls. Product accessibility repair connects the fixed disabled action to its existing localized confirmation reason; the three-locale/two-viewport gate now checks it too. No money, fixture, threshold, exemption, API or transaction changes.

UI PROCESS — Bourdieu: Q1 one task, Q2 optional details, Q3 server facts, Q4 existing CVS boundary, Q5 CI real-click checks, Q6 aesthetics UNKNOWN; the six verdicts above remain applicable. UI PROCESS — Project understanding: reread 架构.md §§0/1/2/26/27, buyer checkout/options contracts, actual controller/callers and native failing receipts; server money/reconfirmation/UNKNOWN remain binding. UI PROCESS — Mobbin → Eagle: no new design; reuse the signed structural references listed above, no additional imports. UI PROCESS — Figma official guidance: existing G1–G7 mapping above remains; no canvas/prototype edits or new strings. Required-fact checklist and three-language copy table above remain the acceptance scope.

Static E1: current-source storefront tsc and 10 MJS/TS node syntax checks exit 0; card-required ownership generated/checked; packet check is structure only. [fix1-failure-evidence.log](fix1-failure-evidence.log) preserves exact native assertions and lint summaries with downloaded-file hashes. [source-hashes.json](source-hashes.json) binds the changed sources. No local runtime test/server. Same assertion still RED after FIX1 means BLOCKED, no third approach. Humaux store/canvas/coord unavailable this session. Exact new head and WAIT_RUN IDs go into the PR comment; integrator relays completion. Author E0; GREEN and independent acceptance pending.
