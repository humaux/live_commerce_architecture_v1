# PARITY-STATES · RED handoff · 2026-10-30

Status: tests-only RED prepared; implementation, CI assertion RED, GREEN and independent review are PENDING. Author assessment E0. Static syntax checks are E1 only. No local tests, builds, servers, browsers, database or containers ran.

Task: integrator-dispatched PARITY-STATES; role ui implementation worker, runtime Codex GPT-6 (effort inherited, no override). Base `df35b338b5cfde5de34bb590e2ba118efc17e4a4`; branch `unit/parity-states`; worktree `parity-states`; PR target `r3/integration`. Main checkout is read-only. No recursive delegation.

## Authority and audit

Owner's card of 2026-10-30 and Figma `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`; current ux-states/DELIVERY.md is the single current 2026-10-28 section, not a superseded polish. References are read-only in the adjacent `ux-states` worktree. Fleet Bridge status returned a connected Untitled document, fileKey null, configured defaultFile matching the card; no claim of independently verified fileKey. Read `1429:35170`, `1429:34986` with get_design_context and `1429:35249` with get_screenshot; raw receipts and PNGs in `reference/`. No Figma writes or bridge restart.

Current source at the base SHA confirms the audit's symptom: StatePanel/InlineError are gallery-only, AdminForbidden uses PermissionState, AdminStateBlock uses EmptyState/ErrorState, WorkspaceFrame contains the raw shell denial and expiry branch, and cart/order/link/lookup use old state markup. CartProvider.refresh currently swallows failed reads, allowing an unavailable cart to look empty. This is a source observation, not a browser reproduction.

## Gate and staged delivery

1. One tests-only push includes ALL new assertions: admin real routes in existing browser-merchant-orders-ui; buyer real routes in existing browser-storefront. Existing parent assertions remain unchanged and in order. BFF read interception is explicitly MOCK fixture setup; production Next route/components still render the page.
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

This RED commit changes tests, two existing browser entry imports/calls, generated ownership and card evidence only. Explicit outside-write-path list for integrator: `tests/admin/orders-ui.spec.ts`, `tests/storefront/shop-real-gate.mjs`, `output/ci-select-backend-browser/browser-ownership.json`, `output/parity-states/**`. The entry changes only ADD reachability; all original gate assertions stay verbatim/in order. No application source changed yet.

Implementation will need same-app scope preapproval for WorkspaceFrame's existing shell denial/expiry branch (no standalone session-expired route exists), CartProvider's swallowed read failure, CartView and OrderLink state branches, and standalone order payment display while leaving CheckoutFlow untouched. List actual paths in the implementation PR body; STOP if contracts/lockfiles/backend need changes.

Risks/NOT_RUN: assertion RED, GREEN, browser click ledger, screenshot parity, density, tsc, local tests/server/build/browser/PG, provider/production/LIVE and independent review. No own process remains running. Integrator owns the CI-completion relay; poll cutoff ≤10 min per push, then fresh session from stored run IDs.

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
