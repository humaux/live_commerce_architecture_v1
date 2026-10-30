# PARITY-SHELL — implementation awaiting PR CI (2026-10-30)

PR #350, branch unit/parity-shell, assigned worktree only. Author assessment **E0**; static evidence **E1**. **Not complete; GREEN and independent review pending.**

## RELAY 2 — current handoff

Tests-only PR head `32ca8367c3b17d1e3d4d3c86efca69d045c91951`: Gates run `41049926127` / parity job `124044902136` failed its new white-rail assertion in all 48 capture cases (actual rgb(31,41,55), expected rgb(255,255,255)); six shell click journeys passed. CI synthetic merge checkout `660bfe7edc41a6e193a3f535292205a0afa8e089`. Foundation run `41049926003` / job `124043161518` passed. **E3 applies only to causal RED at that tests-only head.** See red.log; old pending entries there are historical.

Merged origin/r3/integration at `3ea359f79` before modifying the browser spec. The upstream storefront Dockerfile/smoke changes are inherited, not card-authored. No production action. Implementation commit is the head of PR #350; final workflow IDs are added to the PR body/comment after push, not through an evidence-only commit.

Implemented the light 240px rail, 16px inset, 44px nav targets with 4px gaps, signed muted/active text and #eaf6f2 selected fill. Ten existing permission-filtered groups stay in registry order, including Settings; the authenticated store switcher is below the desktop brand and in the mobile topbar. One responsive store control is mounted at a time. Desktop breadcrumb moves into the 64px topbar. Mobile uses compact store context and keeps navigation/locale/help/account accessible. Existing unsaved-navigation guard, session fence, drawer focus/Escape and server scope remain. The static reference's store name becomes a 44px interactive target; the real store name is never replaced with illustrative design data.

Admin token overrides are in the existing app/globals.css. Both UI and DS primary aliases resolve to #247965; shared package/storefront source is unchanged. Shell styles have greater specificity than the shared defaults, so stylesheet chunk order cannot restore the old white-on-navy styling. No component imports added/renamed; every WorkspaceFrame harness was located and requires no new import/global injection.

All prior parity assertions retained. Added a real breadcrumb click + refresh and synchronized logout with document navigation before refreshing; this closes the pre-existing test race in which the session fence hid controls before logout completed. No assertion/wait/fixture/threshold/exemption/baseline weakened. No edits to next-startup.test.mjs, contracts, migrations, R2 pin, lockfiles or module content.

Local static commands: node --check parity-shell-browser.mjs; bash -n test-local.sh and release-gate.sh; derive-browser-ownership generation + --check; python3 scripts/check_packet.py; git diff --check: exit 0. Packet check is structure only; its timestamped generated file was restored. `pnpm --filter @live-commerce/admin exec tsc --noEmit`: exit 254, compiler unavailable; NOT_RUN, no dependency install. Design detector/browser visual QA not run locally under the card's explicit command allowlist.

Humaux search/store/canvas/coord/code-index are unavailable; no successful database claim or store is represented. This file is task evidence, not a replacement database. Role: integrator-dispatched UI implementer, not independent reviewer. Runtime exact model/effort UNKNOWN. No child agents, local tests, builds, servers, production or Figma writes.

Additional path needing integrator approval: docs/delivery/GATES.md (prior required registration row). Card-requested output/parity-shell/{DELIVERY.md,red.log} contains evidence. No other card-authored out-of-scope path. Shared Go dependency map is unaffected (no Go/import changes); not hand-edited. Rollback reference: pushed tests-only `32ca8367`; no destructive rollback executed.

## UI PROCESS — BOURDIEU

|Question|Verdict for shell implementation|
|---|---|
|Q1 Recent task novice|All ten merchant module names remain visible in desktop navigation; drawer provides mobile access. First-use comprehension UNKNOWN.|
|Q2 Recent task expert|Existing route, store and locale semantics retained; real-click time UNKNOWN until CI.|
|Q3 Money/delivery facts|No page content or business fact changed; existing server responses only.|
|Q4 Input/recovery|Existing authenticated scope, logout, focus trap and Escape preserved; CI journeys pending.|
|Q5 Same mobile goal|Route/store/locale/help/account access must remain; tests exercise navigation at 390×844.|
|Q6 Aesthetics|UNKNOWN until independent/owner review; option A specifies target, not acceptance.|

Novice/expert means recent experience with the task, never class or device.

## UI PROCESS — PROJECT UNDERSTANDING

Read AGENTS.md and 架构.md §0/1/2/26/27 at base `df35b338b5cfde5de34bb590e2ba118efc17e4a4`: distinguish MOCK/design from real acceptance; TypeScript strict; one worktree/branch; author cannot self-accept. Read contracts/invariants.json: scope remains server-authenticated, no client-authority change. Read admin route registry and shell session lifecycle: hide unauthorized routes, preserve logout/expired-session fences. Gates are the current native registry and PR CI; no local execution.

UI authority is owner card option A (2026-10-30) and current UX-orders DELIVERY section “UX-orders polish · 2026-10-29 · 当前交付”; live Fleet Bridge reads desktop 1429:29491/rail 1429:29492 and mobile 1429:28313. Desktop: white 240px rail, 16px padding, 44px rows separated by 4px, #eaf6f2 active row, #247965 green primary. Mobile: compact 64px app bar with store context. Brand/store data remains real authenticated server data, not the frame's illustrative name.

## UI PROCESS — MOBBIN → EAGLE

Reuses the approved layout rather than sourcing a new design. Read existing Eagle receipt in ux-orders/output/ux-orders/simplify2/eagle-orders.json, folder MVQL9YR448VTK. MVP0WJO8UYAVE (Shopify iOS): task context with compact navigation. MVP0WJO2CM2IC (Squarespace): separation of navigation/context and task content. These are inherited reference receipts, not a fresh API read or new import. No images/brands/product facts copied. No new pattern query needed for this shell replication; direct Mobbin tool unavailable.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

Read ux-orders/output/ux-audit-ref/FIGMA-GUIDE.md (2026-10-28); the corresponding main-checkout path is missing. G1: preserve gap/padding and responsive layout. G2: align existing admin semantic tokens to signed resolved colors. G3: reuse AppShell and existing controls; no new design system. G4: real clicks, keyboard, visible result and refresh in new browser gate. G6: manifest/click ledger binds route/viewport/locale to SHA; no claim of independent acceptance. No Figma changes, so copy-lint and prototype-write gates are NOT_APPLICABLE.

## Required shell parity checklist (CI pending)

|Required desktop field/control|Mobile requirement|Summary/review states|Gate|
|---|---|---|---|
|DaWan Live + authenticated store|Compact store context; navigation accessible|Shared shell on every real route|Brand/store assertions + viewport captures|
|Ten modules, original permission filtering|All accessible via drawer|Same shell across eight routes|Exact three-locale nav assertions|
|Current section breadcrumb in topbar|Current task context remains visible|No module summary content altered|Desktop topbar placement + screenshots|
|Store/locale/help/account/logout|Accessible without overlapping task content|Selection persists after refresh|Real controls + click ledger|
|White rail, active fill, green primary|Same token semantics|No separate summary theme|Resolved-color assertions|

## CI artifact and continuation

Mode --browser-admin-parity: 48 capture tests + six shell-control journeys. Root admin-parity/, screenshots parity/<route>-<viewport>.png; zh-CN/en suffix variants; orders aliases parity/1429-29491.png and parity/1429-28313.png. Current design PNGs: desktop get_screenshot-1429-29491-1793204025925-0.png; mobile get_screenshot-1429-28313-1793204023512-0.png. These compare shell only; orders content is not the reference for other modules. No pixel diff. Fixture uses empty lists and explicit unavailable domain reads; cannot accept business flows from these captures.

Next session: inspect only the current implementation head’s CI and click ledger; independently compare the real-route PNGs with signed desktop/mobile PNGs. Outside-card blocker: STOP. Maximum two targeted fixes per blocker. All old assertions and original AppShell remain; browser ownership is regenerated.

release-gate.sh is browser-mode registration; ownership JSON is generated. GATES.md is the additional path disclosed above. Raw design readbacks/images remain uncommitted task evidence. No temporary process remains. CI jobs are under workflow timeouts, and integrator relays completion; no local poller.

NOT_RUN/PENDING: implementation-head CI GREEN, independent visual/click review, tsc, local product tests/servers (prohibited), production, Humaux store/canvas. The implementation-head click ledger and screenshot manifest are generated by CI, never fabricated here. Integrator relays workflow completion; no local poller remains.

## Visible shell copy — existing three-locale authority

No new copy key. From apps/admin/src/shell-copy.ts; all nav/breadcrumb/expanded/recovery strings are included. Store names come from authenticated responses; brand comes from existing company.productName. OperatorFooter is unchanged module content. Locale choices keep their self-names in each locale (繁體中文 / 简体中文 / English). Decorative disclosure symbols carry aria-hidden.

|zh-TW|zh-CN|en|
|---|---|---|
|DaWan Live|DaWan Live|DaWan Live|
|繁體中文 / 简体中文 / English|繁體中文 / 简体中文 / English|繁體中文 / 简体中文 / English|
|總覽|总览|Overview|
|直播|直播|Live|
|訂單與出貨|订单与发货|Orders & shipping|
|商品與庫存|商品与库存|Products & inventory|
|訊息中心|消息中心|Inbox|
|顧客|顾客|Customers|
|行銷優惠|营销优惠|Marketing|
|網路商店|网店|Online store|
|收款與報表|收款与报表|Payments & reports|
|設定|设置|Settings|
|總覽|总览|Overview|
|直播場次|直播场次|Live sessions|
|直播控台|直播控台|Live console|
|直播設定|直播设置|Live settings|
|留言認領|留言认领|Comment claims|
|訂單|订单|Orders|
|建立訂單|创建订单|Create order|
|退貨|退货|Returns|
|列印寄件標籤|打印寄件标签|Print shipping label|
|商品|商品|Products|
|編輯商品|编辑商品|Edit product|
|新增商品|新增商品|New product|
|分類|分类|Collections|
|庫存|库存|Inventory|
|匯入商品|导入商品|Import products|
|顧客|顾客|Customers|
|從 SHOPLINE 搬家|从 SHOPLINE 搬家|Move from SHOPLINE|
|顧客詳情|顾客详情|Customer details|
|優惠碼|优惠码|Discount codes|
|廣告|广告|Ads|
|廣告歸因|广告归因|Ad attribution|
|商店設計|店铺装修|Store design|
|財務|财务|Finance|
|銷售報表|销售报表|Sales reports|
|商店設定|店铺设置|Store settings|
|失敗台賬|失败台账|Operations ledger|
|員工|员工|Team|
|帳單|账单|Billing|
|信用卡收款|信用卡收款|Card payments|
|結算單|结算单|Settlements|
|接受邀請|接受邀请|Accept invitation|
|重設密碼|重设密码|Reset password|
|建立帳號|创建账号|Create account|
|工作區導覽|工作区导航|Workspace navigation|
|開啟導覽|打开导航|Open navigation|
|關閉導覽|关闭导航|Close navigation|
|語言|语言|Language|
|切換商店|切换店铺|Switch store|
|說明|帮助|Help|
|選擇工作區入口；可存取內容由商店角色決定。收款和配送連線請進入商店設定。|选择工作区入口；可访问内容由店铺角色决定。收款和配送连接请进入店铺设置。|Choose a workspace section. Access is limited by your store role. For payment and shipping connections, open Store settings.|
|帳號|账号|Account|
|登出|退出登录|Sign out|
|正在登出…|正在退出…|Signing out…|
|登出失敗，請重試。|退出失败，请重试。|Sign-out failed. Try again.|
|跳至內容|跳转到内容|Skip to content|
|正在載入工作區…|正在载入工作区…|Loading workspace…|
|無法載入工作區權限。|无法载入工作区权限。|Workspace access could not be loaded.|
|登入已過期，請重新登入後繼續。|登录已过期，请重新登录后继续。|Your session has expired. Sign in again to continue.|
|重新登入|重新登录|Sign in again|
|重試|重试|Try again|
|403 · 你沒有存取此頁面的權限。|403 · 你没有访问此页面的权限。|403 · You do not have access to this page.|
|前往可用頁面|前往可用页面|Go to a page you can open|

## Static evidence file hashes

- `apps/admin/components/WorkspaceFrame.tsx` SHA256 `e0b872238b7523f2a64b4a771b9423de828b01925fc17bdb73ecb4aab0d1268d`.
- `apps/admin/app/globals.css` SHA256 `d9196f87e50772ed69198b646e4d89ec180685d32d685850a4a7700d0e5469cb`.
- `tests/admin/parity-shell-browser.mjs` SHA256 `bdc209514b4b16093c653a8478653f1a206dfd864ac25f865f542ea6ce8a2986`.
- `output/ci-select-backend-browser/browser-ownership.json` SHA256 `de62c85dc28508c6664e4d2588463f709d257d28624b7337299e65503082a776`.

## RELAY 3 — hex-ratchet root-cause repair (attempt 1 of 2)

Current implementation head `956cca62fb5692da529a46cb7ae22186c4109942` failed Foundation run `41053216976`, job `124054163359`, in `Node unit suites (storefront, i18n)`. Its sole failing test was `tests/ui/ds-hex-ratchet.test.mjs:87`: admin hex occurrences 390 > 386; globals.css 81 > 77. This is an implementation regression, not the tests-only causal RED recorded above. Raw failed-job log: `relay3-foundation.log` (local failure evidence); relevant original excerpt is appended to red.log. No browser-ownership drift reported.

Root cause: shell A added duplicated color literals rather than reusing existing same-value semantic tokens. Replaced rail white with `--ds-nav-bg` (#fff), rail secondary text with `--ds-crumb-text` (#596579), ink with `--ds-ink` (#142942), and input caret with the existing admin `--ui-primary` (#247965). All four values are identical to the literals they replace; signed colors and root specificity remain. Four literal occurrences removed, no new literals. No scanner, baseline, test, fixture, assertion, threshold, exemption, component import, route, registration or ownership file changed in this repair. Source authority: current globals.css and imported packages/ui/src/tokens.css; owner option A/current UX-orders DELIVERY remains visual authority. No new design or Figma write.

Local validation for this repair: `git diff --check` exit 0; read the imported token definitions to confirm equal values and no alias cycles. No local test, gate, build, server or browser run. No new generator required because no spec/page/route/dependency changed. Evidence is E1 static only; author assessment E0. New-head Foundation and parity GREEN, screenshot/click ledger review and independent acceptance remain PENDING. Exact new workflow IDs will be posted to PR #350 after the single repair push; integrator owns completion relay, no local polling process.

Humaux tools remain unavailable: store/canvas/coord/code-index NOT_RUN; no shared-state write or claim is represented. Only this worktree and assigned branch touched. Repair adds no out-of-card path; docs/delivery/GATES.md remains the previously disclosed additional path requiring integrator approval. Existing untracked design/PR evidence was preserved. Local raw failed-job log retained as failure evidence. No temporary process started.


## RELAY 4 — workspace token scope and tablet store cap (attempt 1 of 2 each)

Current pre-repair head `74bfb17372d2d45e22ad5e076957b5e93aa23faa` is implementation, not tests-only RED. Gates run `41054487504` checks the synthetic merge `e7eea6a88dc1615effa16cab37fc42353e48acd8`:

- bundle:09 / job `124071482550`: `--browser-admin-shell` failed at shell-browser.mjs:156. The 1366/1586/2000/1024 desktop screenshots were produced; the first tablet case (768×1024) failed `clipped: true` with actual false. CI artifact `12982872450` includes the screenshot: the smaller signed 12px store copy fits in the expanded tablet slot. This is **not horizontal overflow**. The original mobile rule removed the shared store control's 420px cap. Retain that existing cap when the control moves to the tablet/mobile topbar, through `--ui-shell-store-max-width`; full name remains in the native select and title. Existing test, fixture and clipping assertion unchanged.
- bundle:01 / job `124071481930`: `--browser-ui-gallery` failed 11 exact DS-color checks. Root cause: six DS aliases were globally overridden at html:root, leaking the new workspace palette into the standalone AppShellV2 gallery. Move those six overrides to the existing `[data-ui-shell]` workspace boundary. Real workspace pages still inherit signed green/active fill, while the standalone library gallery continues to use its own existing token authority. No route exception, exemption, alternate test style or baseline change.
- Foundation run `41054487515` / job `124058457938`: success at this head. New parity mode job `124071481694`: success. These are partial old-head results, not whole-PR GREEN and not acceptance of the repair.

`gh pr checks 350` and `gh run view 41054487504 --log-failed --job 124071482550` were read; the latter refused until the whole workflow finishes. Raw completed-job logs were obtained via `gh api .../actions/jobs/<id>/logs` into `relay4-bundle{01,09}.log`. Failure screenshot/text extracted from the actual CI artifact; no local browser/test/server. Earlier tests-only causal RED remains `41049926127 / 124044902136`, preserved in red.log.

Fresh visual authority read through Fleet Bridge: rail `1429:29492` remains white/240px/16px inset/4px gap; mobile `1429:28313` remains 390×844 with 64px appbar and 12px store text. Current UX-orders DELIVERY section read, not historical snapshots. Only globals.css plus card evidence change this relay; no import/spec/page/route/registration/dependency edits, so no ownership/depmap regeneration needed. Existing dependency-map and gate assertions unchanged. Static `git diff --check`: exit 0 (E1 only). CSS hash above updated; prior CSS evidence is historical. No local tests, builds or servers. New-head CI and independent visual/click review remain PENDING; author judgment E0.

Humaux tools appeared after a directory refresh; prior unavailable notes describe prior relays only. This relay searched exact card name, read development/dispatch details, enumerated rejected representative 5/9 (coverage0.5556, truncated), read coord/canvas and wrote only `integrator-sub-parity-shell` under acquired lock. No separate claim or child agent (integrator-dispatched card). Root-cause store accepted; no platform/local memory used. Final run IDs and updated handoff are stored after the single repair push. Integrator completion relay is the wake-up mechanism; workflow/job timeouts remain in force, no local poller. Stop after two targeted attempts on the same blocker.

NOT_RUN/PENDING: repaired-head CI GREEN, independent side-by-side review, tsc (prior missing compiler), all local product tests/servers (prohibited), production. No new out-of-card source path; prior docs/delivery/GATES.md registration row remains the one requiring integrator approval. Existing untracked files preserved; this relay's CI failure logs/screenshots are retained as evidence, not acceptance.
