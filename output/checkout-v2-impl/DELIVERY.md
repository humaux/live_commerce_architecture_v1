# CHECKOUT-V2-IMPL — implementation handoff, 2026-10-28

PR #273 targets r3/integration, branch unit/checkout-v2-ui. Base d187d7fbac1492ca8fe6c507ab61340b6b9fb616; tests-only head 83fb57206dd7745c66c91ed7d54ecd8fe050227f. Implementation head is the commit containing this delivery (no evidence-only push after GREEN). Author assessment E0; static structure E1; current implementation CI/independent acceptance PENDING, not complete.

Delegated UI worker; assigned checkout-v2-ui worktree only; no recursive delegation, merge, auto-merge, force-push, statuses or production action. Runtime identifies GPT-6-based Codex; exact configured variant/effort UNKNOWN. No migration, API, route-table, lockfile or baseline change.

## Changes and acceptance gate

- Fixed submit bar uses the same native order form as the inline submit; server total and selected timing are shown. ResizeObserver measures bar height and reserves that height plus breathing room below content. No client amount arithmetic added.
- CheckoutReview is a small sibling component for server subtotal/shipping/COD fee/final due and same destination recipient/name/phone/address or selected CVS store. Existing home payment radios are moved into the final review, with selected state and >=44px labels; CVS picker remains untouched.
- Quote acknowledgment stores only quote/cart identifiers, currency and server totals in sessionStorage, scoped to buyer context. Re-quotes and refresh restore the old-to-new notice and disable both submits until the separate acknowledgment. Confirmed acknowledgment survives refresh. Payment mode is restored only if the current server option still offers it.
- UNKNOWN keeps normal submission disabled and relabels the existing recovery control to query the original attempt. Original key/body replay is unchanged. No new attempt, idempotency key, hold, provider method or API is added.
- COD timing uses arrival wording; pickup and online timing are separate translations. Payment help has a real SVG Chevron, aria-expanded/controls, localized state labels and >=44px targets.

Gate: existing registered TestBrowserHomeCod runs checkout-v2.mjs (real quote/address/radio/help/acknowledgment/fixed-submit clicks, visible result and reload); TestBrowserBuyerOrderUI retains every BO05/storage-failure/revoked assertion verbatim/in order and adds UNKNOWN copy/disabled-submit checks. All supported locales are exercised at 1440 and 390x844. MOCK quote-changing fixture is never submitted; a real fixture quote is used for the fixed submit/persistence case. No local test/server.

## Evidence and CI wake-up

Causal RED: Gates run40665214452, merge checkout ec2230626e7584adcc9244601fbc2f6254ed907c, PR head83fb57206dd7745c66c91ed7d54ecd8fe050227f. Job122784411665 (bundle06, TestBrowserHomeCod) fails CHECKOUT-V2 assertions for missing review/help/reconfirm at desktop/390 and refresh. Job122784411546 (bundle02, TestBrowserBuyerOrderUI) fails the added BO05 checkout-unknown assertion. Raw CI artifacts gate-browser-home-cod12818385680 and gate-browser-order12819118918 are retained under red-cod/ and red-order/. Summary: red.log.

Foundation40665214376 job122737878511 also fails gate-waits.test.mjs on the new quoted promise. Implementation corrects it with sanctioned Promise.all; no assertion is removed or softened. Existing home-COD fee-refusal fixture gets an ADD-only explicit acknowledgment click before its unchanged address/submit assertions.

Static commands: node --check tests/storefront/checkout-v2.mjs, home-cod-buyer.mjs and order-gate.mjs: exit0. tsc --noEmit --incremental false -p apps/storefront/tsconfig.json: exit0 via read-only existing toolchain/dependency link (tsc.log); the initial attempt lacked dependencies and the first resolved compile found one nullable access, fixed before this result. git diff --check: exit0. Static evidence only. No Go dependency change, so no depmap generation was required.

Next: read exact implementation SHA's Gates/Foundation runs. Integrator relays a fresh session when runs conclude; bounded observation <=10 minutes, no detached local process, no manual re-dispatch. Workflow timeout/failure conclusions are wake-up conditions. GREEN run/jobs remain PENDING; never infer GREEN from this delivery.

Historical scope question (implementation batch): promotions-gate.mjs needed a changed-total acknowledgment click. The fix-1 dispatch now authorizes ADD-only confirmation adaptation; see the fix-1 section below.

## Required desktop / mobile / review parity

Same CheckoutReview markup composes desktop and mobile. This is source coverage, not browser acceptance; all current execution cells PENDING CI.

| Required field/fact | Desktop and 390 review | Fixed/re-confirm / UNKNOWN |
|---|---|---|
| Subtotal and shipping | quote.amount.subtotal_minor / shipping_minor | Re-confirm keeps review; UNKNOWN retains quote detail |
| COD surcharge | selected server option.cod_surcharge_minor | No client sum; original COD path retained |
| Final total | non-COD quote.amount.total_minor; COD quote.cod_due_minor | Same server field in fixed bar; old -> new gate |
| Home destination | same Destination home_address | Same review during re-confirm; UNKNOWN reads the server destination only if its ID matches the original intent |
| Selected CVS store | same Destination pickup name/code/address | Picker excluded; UNKNOWN uses the same original-ID check; browser execution PENDING |
| Recipient name and phone | same Destination recipient_name / phone | Same review during re-confirm; UNKNOWN shows the same server snapshot after matching its original destination ID |
| Payment method / selected radio | existing offered modes; selected class and checked state | Timing reported to footer; UNKNOWN disables submit |
| Payment timing | checkout-copy in all three locales | Same selected timing at fixed action; original attempt query only |
| Help | localized disclosure/Chevron and >=44px | No invented payment capability |
| Scroll clearance | measured footer height reserves main padding | Layout assertion after real keyboard End at390 |

CheckoutPendingReview keeps UNKNOWN read-only and restores the server recipient snapshot only when its ID matches the original stored intent. Added BO05 assertions require recipient name/phone before and after reload. If another device has moved the destination head, that unrelated snapshot is omitted; no invented recipient or replacement attempt is made. Runtime parity remains PENDING CI.

## Click ledger

CI emits checkout-v2-click-ledger.json and checkout-v2-cod-<locale>-<width>.json in the existing browser evidence directory. Each row contains page, control, action, expected, actual and PASS/FAIL/NOT_RUN; failures retain their real assertion messages. Current implementation actuals are PENDING CI.

| Page/control | Operation | Expected / persisted result | Actual |
|---|---|---|---|
| Checkout delivery / quote | click | Server quote; visible final money | PENDING |
| Address inputs / confirm | fill and click | Server destination; re-confirm after reload | PENDING |
| Payment radios | real click | checked/selected; available saved mode restored | PENDING |
| Payment help | expand / collapse | content, Chevron/state label; closed after reload | PENDING |
| Re-quote / amount acknowledgment | delivery, quote, address and acknowledgment clicks | old->new; both submits blocked until acknowledgment, including refresh | PENDING |
| Fixed submit | real click | original order visible; same id after reload | PENDING |
| UNKNOWN query | existing real recovery click | exact original key/body; one original order | PENDING |
| Scroll | body click + keyboard End | content-end above fixed bar; no overflow at390 | PENDING |

## BOURDIEU

Novice/expert means recent experience with this checkout task, never class, education or device. Q1 plain costs/terms: source coverage, CI PENDING. Q2 repeat completion: optional help and revalidated saved payment choice; task-time/user evidence UNKNOWN. Q3 amount/destination/payment timing before commitment: source coverage, CI PENDING. Q4 store selection: excluded, consumes existing saved store; UNKNOWN. Q5 390 order/scroll/task parity: automated assertion added, execution PENDING. Q6 aesthetic preference separate from task success: UNKNOWN until owner; owner sign-off 「嗯，差不多」 authorizes this specific design, not a general aesthetic claim.

## PROJECT UNDERSTANDING

Read 架构.md §§0/1/2/26/27: distinguish evidence/environment, retain TS strict/Go transaction ownership, merchant/buyer boundaries, assigned worktree and independent CI acceptance. §§8/11/12 rules are applied through cart-quote-v1, buyer-checkout-v1, buyer-checkout-options-v1 and buyer-payment-ui-v1: server Quote is money authority, immutable destination/option versions gate checkout, original command receipt is idempotent recovery, payment is not inferred from placing an order. Latest card rulings permit option.cod_surcharge_minor and quote.cod_due_minor and authorize OrderFlow.tsx. Owner-signed frame set1429:23435..1429:25827 and ux-checkout/DELIVERY.md including fix-1 govern summary recipient/radios/help; live Fleet Bridge get_design_context1429:23484 read this session. No Figma writes/restarts.

## MOBBIN → EAGLE

Read existing module folder MVQL9YPJVZ5FT through http://localhost:41595/api/item/list?folders=MVQL9YPJVZ5FT; response saved as eagle-checkout.json. Reused audit candidates MVNM2RJ3M6VBW (Thrive Market: grouped destination/payment/summary and aligned amounts) and MVOVHC45BTJIX (Selfridges: step grouping and summary hierarchy), also identified in the approved design delivery. No images, brands, overseas methods, store facts or eligibility promises copied. No new import/query is needed for the card's existing patterns; CVS picker reference is separate scope.

## FIGMA OFFICIAL GUIDANCE

Applied the supplied ux-audit-ref/FIGMA-GUIDE.md (read2026-10-28): G1 auto-layout principles become flow layout and measured fixed-action clearance; G2 existing DS-3 ink/line/spacing; G3 summary/timing/action composition; G4/G5 explicit acknowledgment, disclosure and original-attempt recovery; G6 recipient/destination/money and language bindings; G7 audit incumbent controls before composing. Official references supplied in that guide: Figma Guide to auto layout, Variables/collections/modes, Components/Variants, Guide to prototyping and Prototype actions. No new Figma frame was authored, so Figma copy-lint on changed frames is NOT_RUN/not applicable. density-probe.json was inspected: it is a Figma-only merchant frame script requiring use_figma, which this implementation card forbids. No density PASS claimed; runtime density measurement remains NOT_RUN.

## New visible copy (existing unchanged copy remains in its original locale modules)

| zh-TW | zh-CN | en |
|---|---|---|
| 確認訂單 | 确认订单 | Review your order |
| 商品小計 | 商品小计 | Product subtotal |
| 運費 | 运费 | Delivery |
| 貨到付款手續費 | 货到付款手续费 | Cash-on-delivery fee |
| 最終應付 | 应付总额 | Final total |
| 配送至 | 配送地址 | Deliver to |
| 收件人 | 收货人 | Recipient |
| 付款方式 | 付款方式 | Payment method |
| 送出後付款 | 提交订单后付款 | Pay after placing your order |
| 貨到付款：到貨時付款 | 货到付款：送达时付款 | Cash on delivery: pay when your parcel arrives |
| 門市取貨付款 | 到店取货时付款 | Pay at store pickup |
| 金額已更新，請再確認 | 金额已更新，请重新确认 | Total updated. Please confirm again. |
| 確認更新後的金額 | 确认更新后的金额 | Confirm updated total |
| 確認訂單狀態中 | 正在查询订单状态 | Checking your order status |
| 查詢訂單狀態 | 查询订单状态 | Check order status |
| 送出訂單 | 提交订单 | Place order |
| 展開付款說明 | 展开付款说明 | Expand payment help |
| 收合付款說明 | 收起付款说明 | Collapse payment help |
| 送出前請確認金額與配送資料，並依所選方式顯示的時間付款。 | 提交前请核对金额和收货信息，并在所选付款方式提示的时间付款。 | Check the amount and delivery details before placing your order. Payment is due at the time shown for your selected method. |

## NOT_RUN and cleanup

Local tests/servers/browser/PG/race/vet/builds: NOT_RUN. GREEN: PENDING. Independent review, SANDBOX/LIVE provider, production, real buyer acceptance, new Figma Present/copy-lint/density acceptance: NOT_RUN. No temporary process remains. Read-only dependency symlink is removed after final tsc; retained evidence includes failed CI artifacts, source design PNGs and red.log. Rollback: revert this card's commits; no other worktree or main checkout is mutated.

## PR273 fix-1 — 2026-10-29 (CI acceptance PENDING)

Authority: integrator cxf-273-fix.md and original cxf-checkout-v2-impl.md; AGENTS.md; 架构.md §§0/1/2/26/27; buyer-checkout-v1 and checkout-quote-validation-v1. Same UI/UX design and three-language/parity/process contract above; no redesign or Figma writes. Delegated worker only; no recursive delegation or separate task claim. Current source hashes: fix1-hashes.txt. This batch first merged origin/r3/integration without conflicts (merge816030165a0681ae45de3b4e51b6f8b0045dd9c9, including required co-author trailer).

Read every failed PR job. Original implementation head8c096b508db6cc99899bf0f5d9731f83a96b5655 was tested as merge faf6278d4b99db75352c529061c0a25c88bbfd7c:

| Failed job | Observed root cause | Fix / unchanged acceptance |
|---|---|---|
| Foundation40714216294 /122907355910 | G-UI3/G-UI5 actual error is line-count ratchet: CheckoutFlow839>0, OrderFlow880>854; the dispatch's hard-coded-string diagnosis was inaccurate | Extract only quotation display and native payment choices into checkout siblings; CheckoutFlow798 lines, OrderFlow846 lines. No allowance/baseline or gate edits. Checkout v2 strings now live once in packages/i18n/src/checkout.ts with zh-TW/zh-CN/en; existing storefront import facade retained. |
| Gates40714216874 /122974825473 WebKit buyer + order | browser-gate228 and shop-helpers31 language links are visible but fixed bar intercepts clicks | Reserve the measured bar height after site-footer content, scoped to pages containing the bar. Existing real-click locale assertions unchanged. |
| Same WebKit job, claim-checkout | strict radio locator resolves to CVS picker and a second disabled summary radio | Remove the fake read-only fallback radio; keep original selectable CVS group and summary method/timing text. All claim-checkout assertions unchanged. |
| Gates40714216874 /122974825642 bundle05 promotions | promotions-gate254 expects create-order enabled after changed quote, but has not acknowledged the new total | ADD one real quote-reconfirm button click before existing confirm-address click. Every pre-existing assertion is verbatim and in order. |

Evidence: fix1-foundation.log / fix1-webkit.log / fix1-bundle05.log are raw job logs. gh run view --log-failed --job read Foundation; it refuses logs for a still-running Gates workflow, so the completed jobs were read through gh api actions/jobs/<id>/logs. Browser artifact12838715776 was slow; complete relevant buyer/order/claim browser.log entries were extracted from the downloading archive before a later incomplete entry caused tar's decompression error. Each retained log ends at Node's final error; the partial archive is not claimed complete, and the owned download was terminated after extracting required logs. Promotions artifact12840603221 downloaded in full. Retained logs pinpoint the above failures; no local browser, tests or servers ran.

Static E1: node --check promotions-gate.mjs, strict tsc --noEmit --incremental false, git diff --check all exit0 (fix1-static.log). Initial tsc without app dependency links was unavailable; read-only existing dependency links resolved it, with no install or build. Component import impact search found only cod-footer.spec.ts and oc09b_cod_due_test.go source-reading assertions; neither compiles these components with a custom import harness, so no harness allowlist change was needed. Browser ownership regenerated with the mandated derive-browser-ownership.mjs (53 modes) after merging integration, because storefront test hashes changed. No Go dependency changes; dependency-map generation unnecessary.

Source scope: two new checkout siblings, CheckoutFlow, previously authorized OrderFlow, CheckoutReview and storefront copy facade. packages/i18n/src/checkout.ts is explicitly required by this fix dispatch. Relative to the original card, promotions-gate.mjs (ADD-only confirm adaptation) and generated output/ci-select-backend-browser/browser-ownership.json must be reviewed by the integrator as the declared path extension. Evidence lives under output/checkout-v2-impl. All original next-startup gate assertions, fixtures, ceilings, exemptions, contractdrift baseline and route-table dispatchers remain unchanged.

Fix-1 gate: this PR's own exact-head Foundation + Gates, including original buyer/order/claim language/radio clicks, promotions confirmation, checkout3locale and persistence suites. Existing recorded causal RED is retained; this is the single authorized repair batch, not a new tests-only cycle. Author assessment E0, static E1; GREEN/independent review remain PENDING. NOT_RUN: local tests/server/browser/Go/PG/runtime gates, SANDBOX/LIVE/provider/production, runtime density and new Figma work. Integrator relays a fresh session on the new workflow conclusions; bounded CI observation <=10min and no detached process. Rollback: revert this fix commit; no production changes.

## CONT #273 · current-head root causes · 2026-10-29

Dispatched continuation; second targeted repair batch. Current RED head9919a3e393d9ffce6485c7ed42755fec6626fdc0, Gates40749460713, synthesized merge f327c2adfe3a9bc39ba837b151c81f9bdbaf99e6. Read all six failed jobs and downloaded all four relevant browser artifacts. Exact job mapping: red.log; both-engine raw errors: cont-failures.log. Foundation40749460453/job123031931976 passed **that old head only**. This delivery makes no current GREEN or acceptance claim.

Merged integration2bc6d482606a2b0583af15789086fdad727b27ec into this lane as b3cb130f39053c90d45258c0c6981c5601c388da. Regenerated the sole browser-ownership conflict; retained upstream changes. Rollback repair by reverting the implementation commit containing this section to b3cb130f3; no production operation.

Buyer had lost the existing localized unpaid quote fact; restored purchaseCopy.unpaid beside the authoritative quote, keeping every original browser assertion verbatim/in order. Claim's recovery **action locator** used the obsolete label; update it to the signed v2 query label and additionally assert disabled submission and exact visible query wording. No existing assertion, threshold, fixture or exemption changed.

Order's destination completion appended payment-choice geometry after the footer language target was positioned (Chromium +126.796875px). Options and destination now load independently. Ordinary address content waits until server option geometry is available; destination recovery still permits explicit metadata-intent replacement while options are delayed. A destination failure cannot be erased by the other request's completion. Payment radios stay disabled until head validation. The editable final recipient review shows the explicitly confirmed destination, so loading an unconfirmed head does not append recipient paragraphs or display stale recipient text after an edit. Read-only pending review keeps its existing original-intent destination-ID binding. Current BO01 desktop/mobile exact-position checks and R2 delayed-options/address race assertions are unchanged.

### CONT UI PROCESS · Bourdieu

Q1 existing unpaid fact and clear query action preserved; Q2 no new step; Q3 confirmed same-source recipient and payment timing retained; Q4 CVS selector unchanged; Q5 original desktop/mobile stability gate retained; Q6 aesthetics UNKNOWN until owner. Novice/expert remains task experience only. Source assessment E1, real interaction PENDING exact-head CI.

### CONT UI PROCESS · Project understanding

Read AGENTS.md; architecture §§0/1/2/26/27 (evidence, server authority, scope, independent acceptance), buyer-checkout-v1 Decisions (durable original intent and immutable snapshots), buyer-payment-ui-v1 Shared order entry/controller (never infer payment), current OrderFlow/CheckoutFlow and all failing original browser paths. Read signed checkout design entry and state contract in the design lane's output/ux-checkout/DELIVERY.md (reference only, not current product status). Its explicit re-confirmation and UNKNOWN query naming govern this repair. Current status comes solely from current source and exact-head CI, never historical output. No payment transport/contract/migration change.

### CONT UI PROCESS · Mobbin → Eagle

Retains the original card's module folder MVQL9YPJVZ5FT and reference provenance above: Thrive Market MVNM2RJ3M6VBW for grouped review; Selfridges MVOVHC45BTJIX for stable amount/commitment hierarchy. No new reference or visual pattern introduced. No Mobbin import needed; no reference image/brand/fact copied.

### CONT UI PROCESS · Figma official guidance

Retains original FIGMA-GUIDE G1 layout, G4/G5 state and recovery, G6 same-source bindings as documented above; no Figma write, frame change or new official-guidance research. Runtime position/real-click verification is GitHub CI only. Figma copy lint/density NOT_RUN (no design frames changed).

### CONT required desktop / mobile / review parity

| Required fact | Desktop + mobile editable review | Pending/review state |
|---|---|---|
| Recipient name + phone | Explicitly confirmed server destination; source preserved | Existing original-intent ID-matched server snapshot |
| Home address / selected CVS | Same confirmed snapshot | Existing matched snapshot; never substitute a different head |
| Subtotal, shipping, fee, total | Existing server fields, no arithmetic | Existing quote/original attempt |
| Payment choice + timing | Server options, disabled until head ready; fixed bar retained | Read-only original timing; disabled submit |
| Payment not taken | Existing purchaseCopy.unpaid restored in quotation, all3locales | Never claims capture/order success |

Existing three-language review/help/radio/refresh/fixed-submit suite and all original BO05/storage/revoked assertions remain. No new visible string: restored existing fact uses these existing entries:

| zh-TW | zh-CN | en |
|---|---|---|
| 尚未付款 | 尚未付款 | Not paid |
| 查詢訂單狀態 | 查询订单状态 | Check order status |

### CONT click ledger and gate

| Page/control | Real operation | Expected | Current result |
|---|---|---|---|
| Buyer quote/locale/refresh | Existing quote click, language click, refresh | Not paid and quote remain visible | RED missing fact; repaired CI PENDING |
| Order footer locale, desktop/mobile | Held destination GET then real click/tap | Exact target position unchanged, navigation succeeds | RED layout shift; repaired CI PENDING |
| Claim pending recovery | Existing new-claim navigation then query click | No new redeem, disabled submit, original order displayed | RED old name; repaired CI PENDING |

Evidence: cont-static.log and cont-hashes.txt. TypeScript strict/noEmit/incrementalfalse, node --check, diff check exit0; E1 only. No component import changes; every matching harness enumerated. Generated ownership53modes updated in same batch. No Go dependency changes require depmap regeneration. In-module scope extension: tests/storefront/claim-checkout.mjs compatibility action + stronger assertions; generated output/ci-select-backend-browser/browser-ownership.json required by card. Record both in integrator ledger. No other new source scope.

NOT_RUN: local tests/servers/browser/PG/runtime, provider SANDBOX/LIVE, production, independent review and runtime density. GREEN PENDING new push. Integrator relays a fresh session upon the recorded Gates/Foundation WAIT_RUN conclusion; no >10minute polling or silent background process. No merge, auto-merge, force push, statuses or production action.
