# PARITY-STATES · implementation handoff · 2026-10-30

## FIX1 · directed root correction (2026-10-31)

Merged integration a3dc78e4d first (merge 107418797). Inspected PR head 88c147747: Gates 41079438742 / job 124153748956 has four buyer offline retry failures; job 124153748986 has one parcel denial-copy failure, while all 45 main orders cases passed. Full per-job classification, original evidence pointers, acceptance contract, commands/exit codes, click ledger and stop condition are in [FIX1.md](FIX1.md); original filtered evidence in [FIX1-ci.log](FIX1-ci.log). Product correction waits for the clicked cart read before showing its failure. Browser coverage adds a held-read/no-premature-alert assertion in all three locales and both viewports. Parcel 403 coverage now requires the signed heading/body/note/link; all purge/stale-response checks and deadlines remain. Existing fixtures and no-replay controls are retained. New-head CI and independent acceptance PENDING; E1 static only, author assessment E0. No new visual/copy design, strings, settings, Figma writes or source imports; the existing BOURDIEU/project/Mobbin→Eagle/Figma guidance/copy/parity sections remain applicable. Humaux operations UNKNOWN (tools unavailable); integrator owns wakeup. No completion claim.

## RELAY 4 · CI handler-lifecycle repair (2026-10-30)

Current inspected PR head `7a45c5d6facd8c5f98bbd51c2a803a4b8fe6f5ae`: Gates `41070114352` / bundle:09 `124130717361` failed in merchant-orders UI. Artifact `12988537262` shows 41/45 PASS, four shell recovery tests failing at route.fetch response parsing during context teardown; all original 21 cases passed. The trace confirms URL/denial-absence assertions finished while the destination shell was loading. Add destination store-selector value plus visible merchant-studio assertions, and drain route handlers with `unrouteAll({ behavior: "wait" })` before teardown. Existing assertions, read fixture, limits and product source remain unchanged. This is lifecycle blocker repair attempt 1; the earlier buyer alert locator is a different blocker. Static E1: node --check and generated ownership + --check exit 0. New-head CI, visual parity, independent review and GREEN remain PENDING. No local runtime verification. Raw trace retained locally; concise receipt `RELAY4.log`. Integrator relay owns wakeup; Humaux remains unavailable.

Status: causal assertion RED confirmed on b3c4736a; implementation prepared for one batch push. GREEN and independent review PENDING. Author assessment E0; local static checks E1 only. No local tests, builds, servers, browsers, database or containers ran.

Task: integrator-dispatched PARITY-STATES; role ui implementation worker, runtime Codex GPT-6 (effort inherited, no override). Base `df35b338b5cfde5de34bb590e2ba118efc17e4a4`; branch `unit/parity-states`; worktree `parity-states`; PR target `r3/integration`. Main checkout is read-only. No recursive delegation.

## Authority and audit

Owner's card of 2026-10-30 and Figma `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`; current ux-states/DELIVERY.md is the single current 2026-10-28 section, not a superseded polish. References are read-only in the adjacent `ux-states` worktree. Fleet Bridge status returned a connected Untitled document, fileKey null, configured defaultFile matching the card; no claim of independently verified fileKey. Read `1429:35170`, `1429:34986` with get_design_context and `1429:35249` with get_screenshot; raw receipts and PNGs in `reference/`. No Figma writes or bridge restart.

Current source at the base SHA confirms the audit's symptom: StatePanel/InlineError are gallery-only, AdminForbidden uses PermissionState, AdminStateBlock uses EmptyState/ErrorState, WorkspaceFrame contains the raw shell denial and expiry branch, and cart/order/link/lookup use old state markup. CartProvider.refresh currently swallows failed reads, allowing an unavailable cart to look empty. This is a source observation, not a browser reproduction.

## Gate and staged delivery

1. One tests-only push includes ALL new assertions: admin real routes in existing browser-merchant-orders-ui; buyer real routes in existing browser-storefront. Existing storefront gate-adapter assertions remain verbatim and in order. Admin assertions for superseded denial copy now require the exact current signed copy; privacy/control assertions remain. BFF read interception is explicitly MOCK fixture setup; production Next route/components still render the page.
2. RED is accepted only if the new signed-copy/panel assertions fail, not compilation or fixture setup. Record original run/job/head and tested merge SHA in red.log after CI reaches them. No implementation before causal RED.
3. Then change shared internals and buyer state branches in one implementation push, preserving session/ACL/read/command semantics. Existing caller text remains contextual; do not claim an empty orders/customer filter is first-time collection setup.
4. GREEN is this PR's own CI. Independent reviewer compares actual route screenshots to DELIVERY PNGs; author PASS remains E0. No merge, auto-merge, force push, status posting or deployment.

Browser coverage: three locales × desktop 1440×900 / mobile 390×844. Admin: read denial, shared loading/error/retry/empty, shell role denial (raw 403 removal), session expiry. Buyer: empty, cart offline and repeated-read failure, cart denial, order denial, order read failure, expired/missing link, payment uncertainty and failed status read, lookup failure without losing in-page input. No CheckoutFlow modification.

Click ledger is emitted by the tests only after real click/fill/reload assertions. Current results: NOT_RUN. Refresh checks cover persistent server denial, successful cart reread and empty results; lookup values intentionally clear after refresh per privacy contract. No draft persistence promise. Uncertainty recovery forbids ALL command/refresh-signal POSTs and reads the same order/payment locator.

## Required desktop/mobile/review facts

|Fact from signed desktop|Mobile assertion|Read-failure/review assertion|Current evidence|
|---|---|---|---|
|Buyer empty means current cart has no products, not purchase history|Exact title/body/note/action, both viewports|Successful reread becomes the same empty state and survives reload|NOT_RUN|
|No access reveals no foreign order data|Same generic title/body/note and return link|No order table, no raw diagnostic|NOT_RUN|
|Cart offline is a read, never order submission|Exact body and server-cart note|InlineError after failed retry; successful retry and reload show server result; no writes|NOT_RUN|
|Expired link directs to order lookup, never reorders|Exact title/body/note/action|Real link click reaches lookup, missing link remains refused on refresh|NOT_RUN|
|Payment uncertainty keeps the original order and total|2,620 remains in order page; only status-read recovery|Failed read retains amount and no pay control; no command/signal POST|NOT_RUN|
|Admin denial states the next available action|Exact signed body/note, return to workspace|Denial survives refresh; shell raw 403 removed|NOT_RUN|
|Admin loading/error/empty preserve original task context|Same shared panel/skeleton on both viewports|Retry fails inline then succeeds empty; read method stays GET|NOT_RUN|

No synthetic design order label, sample amount, customer identity or design-disclosure scaffolding may enter production copy. Order/amount facts come from existing responses. Merchant operation UNKNOWN/Meta OAuth expired and collection-first-use frames are reference semantics; this card does not edit their module pages or fabricate data to make those contexts appear. Session expiry uses the existing sign-in message with the shared expired panel, not a false Meta-authorization message.

## Parity sheet (CI artifacts, not local screenshots)

Paths beneath the harness artifact root. The ux-states PNG column is relative to its `output/ux-states/shots/` directory. Buyer captures are made by this spec on REAL routes with MOCK read fixtures; admin captures are from the signed-identity real orders route. Signed shell/prototype disclosures are outside the content panel comparison; retain current authorized shell and real merchant branding.

|Frame|Artifact screenshot|DELIVERY PNG|
|---|---|---|
|1429:34986|parity/1429-34986.png|merchant-forbidden-desktop.png|
|1429:35006|parity/1429-35006.png|merchant-forbidden-mobile.png|
|1429:35150|parity/1429-35150.png|buyer-empty-desktop.png|
|1429:35170|parity/1429-35170.png|buyer-empty-mobile.png|
|1429:35190|parity/1429-35190.png|buyer-forbidden-desktop.png|
|1429:35210|parity/1429-35210.png|buyer-forbidden-mobile.png|
|1429:35230|parity/1429-35230.png|buyer-offline-desktop.png|
|1429:35249|parity/1429-35249.png|buyer-offline-mobile.png|
|1429:35268|parity/1429-35268.png|buyer-expired-desktop.png|
|1429:35288|parity/1429-35288.png|buyer-expired-mobile.png|
|1429:35308|parity/1429-35308.png|buyer-unknown-desktop.png|
|1429:35331|parity/1429-35331.png|buyer-unknown-mobile.png|
|1429:35448|parity/1429-35448.png|buyer-offline-desktop-read-failure.png|
|1429:35466|parity/1429-35466.png|buyer-offline-mobile-read-failure.png|
|1429:35484|parity/1429-35484.png|buyer-unknown-desktop-read-failure.png|
|1429:35506|parity/1429-35506.png|buyer-unknown-mobile-read-failure.png|

## BOURDIEU · Q1–Q6

Q1 novice/expert refers only to recent recovery-task experience. Q2 both need the cause and one safe next action. Q3 role or device is not a proxy for social class or skill. Q4 signed one-action panels reduce recovery decisions; no advanced setting added. Q5 access/session authority remains with existing server responses. Q6 aesthetic preference is UNKNOWN until owner/reviewer validation; no invented user study.

## PROJECT UNDERSTANDING

Read 架构.md §0 (evidence classes, no model-as-product claim), §1 (Go/PG truth, TypeScript strict/Next), §2 (buyer storefront boundary and card-payment deviation), §26 (isolated worktree, no recursive delegation, independent acceptance), §27 (module gates and staged integration). Read contracts/invariants.json I01/I02/I05: authenticated scope, idempotency and trusted payment facts. Read signed SPEC mapping to contracts/storefront-v2.md §E5/G3 and stripe-buyer-ui-v1.md: non-leaking lookup/link refusals, uncertainty reads same locator. No API, migration, contract or payment-state-machine change is authorized.

## MOBBIN → EAGLE

Existing module folder `MVQL9Z39HZLH0` read via the existing Eagle API; 8 items returned, no library writes. `reference/eagle-live.json` preserves the receipt. Used structural principles from approved references: `MVOVLMVSA15MW` Copilot — one explanation/one primary action; `MVOVLO8WKRBD6` Bevel — recovery in original task position; `MVRSAPZL4DT5D` Toggl — neutral permission exit. No images, brands or product facts copied. No new Mobbin pattern needed; signed components already supply the pattern.

## FIGMA OFFICIAL GUIDANCE

Read the module's read-only `output/ux-audit-ref/FIGMA-GUIDE.md`, source dated 2026-10-28. Apply G1 auto-layout as responsive CSS; G2 variables as DS tokens; G3 one StatePanel/InlineError with variants; G4 real, semantically safe recovery targets; G5 repeated-read failure preserves uncertainty; G6 route/permission/locale mapping and click evidence; G7 reuse signed components. Official sources are linked in that authoritative checklist. No Figma modification, so copy-lint of changed frames is NOT_APPLICABLE; no changed frames. Density probe and independent visual comparison are PENDING CI artifacts, not inferred from source.

## Scope and risks

Implementation changes shared admin state internals and the existing WorkspaceFrame expiry/denial boundary. Buyer pages use one StorefrontState/StorefrontReadFailure; CartProvider exposes readProblem separately from write recovery, preserving its observation-order fence. Same-locator payment GET failure keeps the original pending view/amount, hides payment actions until a successful read, and shows InlineError. Permission/context refusal clears that payment view. The original Stripe Continue/Cancel and bounded polling/signal contracts stay in place; the signed Check status action itself only GETs. CheckoutFlow is untouched.

Same-app scope preapproval used: WorkspaceFrame.tsx, admin-state-block.css, shell-copy.ts; CartProvider.tsx, CartView.tsx, OrderLink.tsx and OrderPayment.tsx. SSR harness allowlists load the real signed components. Admin old-copy expectations are replaced with current signed copy (no authorization/privacy assertions removed); storefront gate-adapter assertions are unchanged. No API/contract/migration/backend/package manifest/lockfile change.

Static commands: node --check on registered browser entries, updated SSR harnesses, staff-team.spec.ts and state-recovery.ts: exit 0. Browser ownership generator then --check: exit 0 (53 modes). depmap.sh: exit 0, no content change. python3 scripts/check_packet.py: exit 0, PASS_PACKET_STRUCTURE_ONLY; its timestamp-only result was restored, not committed. tsc NOT_RUN: no local compiler/dependencies; CI owns type/build/browser/runtime verification.

Risks/NOT_RUN: implementation CI GREEN, screenshot parity, density and independent review are pending. All local tests/server/build/browser/PG/provider/production/LIVE NOT_RUN. No completion claim. Humaux tools are absent; store/canvas/coord/rejected lookup UNKNOWN, no shared record claimed. Integrator owns CI-completion relay; handoff includes the real Foundation/Gates run IDs, never a plan job.

Downloaded failed CI artifacts are retained locally in output/parity-states/relay2/ (24 MB), not staged into the source commit. No local runtime process remains. No bridge or Figma write/restart. Bridge status/read context rechecked this session; connected fileKey remains null, defaultFile matches the card.

## Three-language copy contract

The following table is the independent test contract for signed panel strings, with faithful zh-CN/en translations. It is not evidence that current production renders them. Shared loading/contextual order/lookup/session copy stays in existing locale sources; add any implementation-only visible strings before final hand-in.

|Key|zh-TW|zh-CN|en|
|---|---|---|---|
|empty|購物車還沒有商品|购物车还没有商品|Your cart is empty|
|emptyBody|先挑選想買的商品，再回到購物車確認數量與配送方式。|先挑选想买的商品，再回到购物车确认数量与配送方式。|Choose products first, then return to your cart to confirm quantities and delivery.|
|emptyNote|選好商品後，可在購物車確認數量。|选好商品后，可在购物车确认数量。|You can check quantities in your cart after choosing products.|
|browse|瀏覽商品|浏览商品|Browse products|
|forbidden|目前無法開啟此頁|暂时无法打开此页面|This page cannot be opened|
|forbiddenBody|此連結無法開啟，或目前購物連線無法使用這項功能。請從商店重新開始。|此链接无法打开，或当前购物会话无法使用此功能。请从商店重新开始。|This link cannot be opened, or your shopping session cannot use this feature. Start again from the store.|
|forbiddenNote|如需查看訂單，請使用商店的訂單查詢。|如需查看订单，请使用商店的订单查询。|Use the store’s order lookup to view an order.|
|home|返回商店|返回商店|Return to store|
|offline|暫時無法載入購物車|暂时无法加载购物车|Your cart could not be loaded|
|offlineBody|請檢查網路連線，再重新載入。這次只讀取購物車，不會送出訂單。|请检查网络连接，再重新加载。本次只读取购物车，不会提交订单。|Check your connection and reload. This only reads your cart and does not place an order.|
|offlineNote|商品與數量以原購物連線重新讀取的結果為準。|商品与数量以当前购物会话重新读取的结果为准。|Products and quantities come from re-reading your original shopping session.|
|retry|重新載入|重新加载|Reload|
|failed|目前仍無法連線。請保留原頁，稍後重新載入。|暂时仍无法连接。请保留当前页面，稍后重新加载。|We still cannot connect. Keep this page open and reload later.|
|expired|無法開啟這個連結|无法打开此链接|This link cannot be opened|
|expiredBody|請從訂單查詢重新開始，並準備訂單編號與電子郵件。|请从订单查询重新开始，并准备订单编号与邮箱。|Start again from order lookup, with your order number and email ready.|
|expiredNote|連結不適用時，請勿再次送出原訂單。|链接不可用时，请勿重新提交原订单。|Do not place the original order again if the link cannot be used.|
|lookup|前往訂單查詢|前往订单查询|Go to order lookup|
|unknown|付款結果尚未確認|付款结果尚未确认|Your payment result is not confirmed|
|unknownBody|請先查詢這筆訂單的付款狀態。確認前請勿再次付款。|请先查询这笔订单的付款状态。确认前请勿再次付款。|Check this order’s payment status first. Do not pay again until it is confirmed.|
|check|查詢狀態|查询状态|Check status|
|unknownFailed|目前無法讀取最新狀態。請稍後查詢同一筆訂單。|暂时无法读取最新状态。请稍后查询同一笔订单。|The latest status could not be loaded. Check the same order again later.|
|lookupSubmit|查詢|查询|Find order|
|lookupFailed|暫時無法查詢，請稍後再試。|暂时无法查询，请稍后再试。|Lookup is unavailable right now. Please try again later.|
|lookupNoMatch|找不到符合的訂單，請確認訂單編號與電子郵件或手機號碼。|找不到匹配的订单，请确认订单编号与邮箱或手机号码。|We could not find a matching order. Check the order number and the email or phone number.|

## RED push receipt (relay)

PR #356, head `7b2ab178b90b80bc18c099babdf8e371637a0d21`. Foundation WAIT_RUN `41050630088`, foundation job `124045491838` queued; Gates WAIT_RUN `41050630085`, test jobs not registered at handoff. No causal RED or GREEN yet. Receipt updates are local pending the implementation commit; no evidence-only push. Integrator relays this session when the CI concludes.

## RELAY 1 · gate registration repair (attempt 1)

Original head `7b2ab178b90b80bc18c099babdf8e371637a0d21`: Foundation `41050630088`, job `124045491838`, failed in “Package docs and gate map are current”. Both new imported spec fragments were reported as “run by no gate”. This is a registration failure, **not assertion RED**. Original receipt is in red.log.

All new assertions/fixtures now live directly in the already registered `tests/admin/orders-ui.spec.ts` and `tests/storefront/shop-real-gate.mjs`; the two import-only fragments are removed. Existing gate assertions and call order remain unchanged. No gate checker, exemption, threshold, fixture expectation or production source changed. This tests-only repair precedes implementation.

Merged `origin/r3/integration` at `3ea359f79` before spec deletion, then regenerated ownership (53 modes, `--check` exit 0). The merge contains upstream deployment changes only; no deployment file was authored by this card. `node --check` on both registered entries exits 0 (E1). No local runtime verification. New CI registration and assertion RED remain PENDING; integrator will relay after the next runs.

Submission correction: `0f197e893` accidentally pushed fragment deletions without their unstaged destinations after git add failed. A follow-up commit includes both destinations and generated ownership. No assertions removed from the final tree; no force push. Count the follow-up as attempt 2 for this blocker; STOP if it persists.

## RELAY 2 · causal assertion RED and implementation

PR head b3c4736a53279bb9557755e4540351f01c1ce126; Gates run 41052469401 / bundle:01 job 124056245071; tested merge c6e49cf79ab86b427a5ee676ef87650a8200e50a. Foundation 41052469470 / job 124051677598 succeeded. Admin: 24 new PARITY-STATES failures, 21 existing cases passed. Buyer: the final PARITY-STATES assertion lists missing signed panels and a legacy lookup paragraph (expected DIV, received P). These are causal UI assertion failures, not build/setup/registration failures. See red.log; raw artifacts 12976717841 and 12976991799.

One new test expectation was corrected after tracing current route permissions: removing orders:read removes both Orders and Overview; the safe existing home is /studio with the original store query. The test now asserts that exact destination and that the denial is gone. It does not route a denied role to a second denial or change grants to clear the test.

The Figma sample brand, design-demo labels, sample-order text and demonstration controls are not product facts. Real store/order values stay server-derived. Payment uncertainty repeats the real quoted total in the signed panel, on both viewports and failed-read state. CSS places this recovery before long delivery details without remounting the payment controller; all original order details and disclosures remain available. Independent visual comparison of actual CI screenshots is still required; no E4 parity claim.

### Admin signed copy · zh-TW | zh-CN | en

|Key|zh-TW|zh-CN|en|
|---|---|---|---|
|title|目前無法開啟此頁|暂时无法打开此页面|This page cannot be opened|
|body|此連結無法開啟，或目前帳號沒有此操作權限。請返回可用的功能。|此链接无法打开，或当前账号没有操作权限。请返回可用的功能。|This link cannot be opened, or your account cannot use this feature. Return to an available feature.|
|note|若需這項功能，請向店鋪擁有者確認權限。|如需使用此功能，请向店铺所有者确认权限。|Ask the store owner if you need access to this feature.|
|home|返回工作台|返回工作台|Back to workspace|
|retry|重試|重试|Try again|
|expired|登入已過期，請重新登入後繼續。|登录已过期，请重新登录后继续。|Your session has expired. Sign in again to continue.|
|signIn|重新登入|重新登录|Sign in again|

Admin loading/error/empty titles retain the caller's existing three-locale task copy. Session recovery retains shellCopy's existing three-locale sessionExpired/signIn. Order offline retains lookupCopy.loadFailed; payment initial read failure retains paymentCopy.failed. No new visible literal outside the locale copy tables. Copy inventory of those existing contextual states is below.

|Context key|zh-TW|zh-CN|en|
|---|---|---|---|
|admin.errorTitle|無法載入這個頁面|无法加载这个页面|We couldn’t load this page|
|admin.errorDescription|頁面暫時無法載入，請重試或返回儀表板。|页面暂时无法加载，请重试或返回仪表盘。|The page could not be loaded. Try again, or return to the dashboard.|
|admin.notFoundTitle|找不到這個頁面|找不到这个页面|We couldn’t find this page|
|admin.notFoundDescription|頁面可能已移動或不存在，請返回儀表板繼續操作。|页面可能已移动或不存在，请返回仪表盘继续操作。|The page may have moved or no longer exists. Return to the dashboard to continue.|
|admin.loading|正在載入…|正在加载…|Loading…|
|admin.retry|重試|重试|Try again|
|admin.home|返回儀表板|返回仪表盘|Back to dashboard|
|lookup.loadFailed|無法載入訂單。|无法加载订单。|The order could not be loaded.|
|lookup.invalid|請檢查訂單編號與聯絡方式的格式。|请检查订单编号与联系方式的格式。|Check the format of the order number and the contact.|
|lookup.tooMany|嘗試次數過多，請稍後再試。|尝试次数过多，请稍后再试。|Too many attempts. Please try again later.|
|payment.failed|暫時無法確認付款狀態，請重新整理此訂單查詢，不要重複下單。|暂时无法确认付款状态，请刷新此订单查询，不要重复下单。|Payment status could not be confirmed. Refresh this order to check again; do not place it again.|

## RELAY 3 · recovery-alert locator repair (attempt 1)

Current implementation head `121d16300c44c2a2e19c62b5c769c7829f9889df`: Foundation `41058992626` / `124073641046` succeeded; Gates `41058992663` / bundle:01 `124091034229` succeeded. Bundle:02 `124091034606` failed in the storefront member. These are CI observations on the PR synthetic merge `9773319304fbdc872b5eff7e8f2a8bce12e24e06`, not full GREEN or independent visual acceptance.

Artifact `12984047644` contains 48 buyer ledger rows: 36 PASS, 12 FAIL, all offline/unknown failed-read assertions across three locales and two viewports. Next.js adds a route-announcer `role="alert"`, so the new full-page alert locator matches it alongside the expected signed InlineError. Scope those new alert assertions to the existing signed panel; also scope order-offline's visibility assertion so a route announcement cannot satisfy it. Exact copy, accessibility role, real click operations, read-only/no-replay assertions, fixtures and waits remain. Production source and original gate-adapter assertions remain unchanged. This is attempt 1 for the locator blocker. Raw CI artifacts stay local; compact receipt: `RELAY3.log`.

Static E1 only: `node --check tests/storefront/shop-real-gate.mjs`, ownership generation and `--check` exit 0. New head CI and screenshot review PENDING. Local tests/servers NOT_RUN. Humaux is not available; store/canvas/coord cannot be claimed.
