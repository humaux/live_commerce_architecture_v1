# PARITY-GROWTH — RED handoff (implementation pending)

Evidence: E1 syntax/static registration only. No local tests, builds, servers or browser runs.
Base: `3ea359f796d36153bb4b8aa7259e4e0c27676b86`. Branch: `unit/parity-growth`. Target: `r3/integration`.
Role: dispatched UI worker; model/effort runtime labels are not independently exposed. No delegation.

## Authority and project understanding

- Owner card dated 2026-10-30: Figma is the production UI, MOCK only; one batch RED push, then implementation only after assertion RED.
- AGENTS.md; 架构.md §§0/1/2/26/27: evidence separation, Go/PG authority, tenant/store authentication, isolated branch and independent acceptance.
- 架构.md §§14/15; contracts/meta-ads-v1.md; existing ads/attribution/promotions DTOs: consent boundaries, no client money calculations, no UNKNOWN replay; approval and external publication differ.
- UI authority: file `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`, C3 inventory. All 38 roots read via Fleet Bridge get_design_context AND get_screenshot on 2026-10-30; truncated content subtrees fetched separately, no Figma writes.
- No ux-ads/ux-marketing worktree is registered. Existing signed design checkpoints were located under output/figma-ui-completion-20261025/ads-{draft,list,reports}-20261026/DELIVERY.md. They supplied reference pointers only. CURRENT frame receipts determine copy and state.
- Read development rule 7bdb220b-0798-4b1d-aff5-16ee94cae75f and visual rule f6273516-2c72-4f2e-b4b3-54d3625809bb. Current card's CI-only/no-merge rules override generic defaults. Rejected representative coverage 5/9; lessons coverage 5/33, not an exhaustive claim.

## Current-frame findings / deferred data

- Marketing-named 4:11612/4:11750 currently contain the ads draft list; 1194:7740/7796 contain the same list with the feature-off notice. Historical names do not justify rebuilding a different UI.
- 5:17304/17438 currently contain the recovery heading 「從列表開啟正確的草稿」. No historical daily-budget/performance detail is restored.
- DEFERRED 6:18113/18232 customer segments: promotions DTO has no segment id/name/rules/version/member_count/updated_at or consent-filtered eligibility. The feature-off fallback is tested; no six demo segment counts are fabricated.
- Source inventory gap: ads settings contain authorized identities, not an inventory of owned published posts/products with display names and eligibility. Parity tests select a source already saved by the real MA09 fixture. New source discovery beyond existing server data remains DEFERRED pending exact `sources[{source_ref,display_name,kind,eligible}]` fields. No API/contract/migration is changed in this RED stage.
- Asset grant/sync metadata is limited to existing settings; unavailable authorization dates/permissions must stay absent rather than copying the example's dates.

## BOURDIEU (design interpretation, not user research)

| Verdict | Application |
|---|---|
| Q1 | New to this advertising task: explicit source → budget → review → save/approval sequence. |
| Q2 | Experienced with this task: preserve server values, draft revision and a direct return path. |
| Q3 | Knowledge burden: explain internal approval separately from Meta publication and delivery. |
| Q4 | Recovery cost: preserve local edits on conflict; unknown external results permit reads only. |
| Q5 | No class/device inference: desktop and mobile retain the same material facts. |
| Q6 | Aesthetic preference UNKNOWN until owner; implement the signed frame without inventing a new style. |

## MOBBIN → EAGLE

Read-only Eagle item/info on 2026-10-30 confirms both references present and nondeleted in folder `MVQL9YXQLTAAR`.
Google Ads `f1889379-f1cd-41a6-9bc9-c3ca9d27c8f4` → `MVOJWMITZO1XA`: staged creation/review hierarchy.
AirOps `a0c8e522-9ff7-4787-a203-648575e28504` → `MVOT8B5BK3QQ3`: date controls before independent metrics and titled groups.
Reference structure only; no copied images, branding, numbers or product claims. Receipts: eagle-*-reference.json. No missing reference needs importing for this signed-frame implementation.

## FIGMA OFFICIAL GUIDANCE

Read the existing UX audit's FIGMA-GUIDE.md (ux-audit/output/ux-audit; the card's ux-audit-ref alias does not exist).
G1 Auto layout → responsive task panels/action dock; G2 Variables → existing semantic tokens; G3 Components/variants → shared controls/state rendering; G4/G5 Prototyping → explicit next/return and conflict/unknown paths; G6 → frame-to-route/viewport handoff.
Primary guidance links recorded there: https://help.figma.com/hc/en-us/articles/360040451373-Guide-to-auto-layout-in-Figma ; https://help.figma.com/hc/en-us/articles/14506821864087-Overview-of-variables-collections-and-modes ; https://help.figma.com/hc/en-us/articles/360038662654-Guide-to-components-in-Figma ; https://help.figma.com/hc/en-us/articles/360040314193-Guide-to-prototyping-in-Figma . No new Figma design is delivered, so design copy lint is NOT_RUN.

## Desktop → mobile → summary/review fact checklist

| Required fact | Desktop / mobile target | Summary/review target | Current evidence |
|---|---|---|---|
| Server source and authorized identity | editor/source pair | saved + review pair | NOT_RUN; existing MA09 source only |
| Lifetime budget, not daily budget | settings pair | saved/review/status | NOT_RUN; 1,500 persistence assertion |
| Start/end, Taipei timezone, schedule validity | settings pair | saved/review/status | NOT_RUN; real fill and API save |
| Country and age range | settings pair | review pair | NOT_RUN; target fact labels |
| Draft id/revision and save vs approve vs publish | editor/saved/review | conflict/status | NOT_RUN; two-tab concurrent update |
| Selected period/source and three independent metric groups | index/results/detail | Meta breakdown | NOT_RUN; real report fixture |
| Pending payment excluded; missing metrics remain unknown | detail/breakdown pair | summary groups | NOT_RUN; existing AT assertions preserved |
| Segment counts/consent and rule versions | 6:18113/18232 | no invented summary | DEFERRED; required DTO absent |

## Click ledger and acceptance

Registered via existing tests/admin/ads.spec.ts, attribution.spec.ts and tests/storefront/promotions-gate.mjs. Parent gate assertions preserved verbatim; no existing test deleted, skipped or weakened.
The new journeys write growth-click-ledger.jsonl after visible assertions and parity/frames.jsonl after real-route screenshots. Marketing verifies refresh persistence in all locales; ads save and conflict verify server persistence after reload. Two-tab edits exercise real version conflict, not a synthetic 409. A delayed REAL GET is fixture preparation for the loading frame; no write is intercepted.
Every clickable flow remains NOT_RUN until the PR's own CI executes it. Existing MA09/AT/promotions intent must be mapped to the new task layout during implementation, never deleted.

## Legacy removal targets

- /ads: old H1 「廣告」 / Ads / 广告 and stacked connection + drafts + CAPI + report panels. New task screens have one purpose; connection/report remain reachable.
- /ads/attribution: old H1 「廣告歸因與直播復盤」 / Ad attribution & session review and simultaneously open draft/session panels; replace with selection → one detail → breakdown.
- /promotions: old page H1 「優惠碼」 / 优惠码 / Discount codes; existing discount-code creation/edit/toggle intent remains required in a reachable task, with its original gate retained.

## Gate and rollback

Static checks: node --check on both new helpers and all changed parent entry points; git diff --check; derive-browser-ownership.mjs then --check (53 modes). Exit 0. tsc NOT_RUN (worktree dependencies absent); no install or local server.
CI RED/GREEN, real browser clicks, persistence, screenshot parity, density and independent review: NOT_RUN/PENDING.
No process, DB, port or container was started. Read-only Fleet Bridge calls completed. Rollback is git revert of this tests-only commit on this branch; untracked evidence is retained, not removed by git.
Outside C3 application write paths: tests/admin/{ads,attribution,parity-growth}.spec.ts, tests/admin/parity-growth-marketing.spec.mjs, tests/storefront/promotions-gate.mjs (spec registration); generated output/ci-select-backend-browser/browser-ownership.json; output/parity-growth/** evidence. No application implementation changed.

## Frame parity sheet (all screenshots pending CI)

| Frame | Real route / state | Browser artifact | Signed reference PNG | Status |
|---|---|---|---|---|
| 5:17304 | /ads · Admin · 广告详情 · SHOPLINE | parity/5:17304.png | [figma/5-17304/get_screenshot-5-17304-1793361325193-0.png](figma/5-17304/get_screenshot-5-17304-1793361325193-0.png) | NOT_RUN |
| 5:17438 | /ads · Admin Mobile · 广告详情 · SHOPLINE | parity/5:17438.png | [figma/5-17438/get_screenshot-5-17438-1793361326127-0.png](figma/5-17438/get_screenshot-5-17438-1793361326127-0.png) | NOT_RUN |
| 5:17516 | /ads · Admin · 资产授权 · SHOPLINE | parity/5:17516.png | [figma/5-17516/get_screenshot-5-17516-1793361328107-0.png](figma/5-17516/get_screenshot-5-17516-1793361328107-0.png) | NOT_RUN |
| 5:17633 | /ads · Admin Mobile · 资产授权 · SHOPLINE | parity/5:17633.png | [figma/5-17633/get_screenshot-5-17633-1793361329143-0.png](figma/5-17633/get_screenshot-5-17633-1793361329143-0.png) | NOT_RUN |
| 6:20130 | /ads · Admin · 广告草稿 · SHOPLINE | parity/6:20130.png | [figma/6-20130/get_screenshot-6-20130-1793361330220-0.png](figma/6-20130/get_screenshot-6-20130-1793361330220-0.png) | NOT_RUN |
| 6:20255 | /ads · Admin Mobile · 广告草稿 · SHOPLINE | parity/6:20255.png | [figma/6-20255/get_screenshot-6-20255-1793361331165-0.png](figma/6-20255/get_screenshot-6-20255-1793361331165-0.png) | NOT_RUN |
| 6:23064 | /ads/attribution · Admin Desktop · 歸因報表 · DaWan Live | parity/6:23064.png | [figma/6-23064/get_screenshot-6-23064-1793361332168-0.png](figma/6-23064/get_screenshot-6-23064-1793361332168-0.png) | NOT_RUN |
| 6:23199 | /ads/attribution · Admin Mobile · 歸因報表 · DaWan Live | parity/6:23199.png | [figma/6-23199/get_screenshot-6-23199-1793361333035-0.png](figma/6-23199/get_screenshot-6-23199-1793361333035-0.png) | NOT_RUN |
| 1049:6945 | /ads · Admin Desktop · 廣告草稿 · source | parity/1049:6945.png | [figma/1049-6945/get_screenshot-1049-6945-1793361334045-0.png](figma/1049-6945/get_screenshot-1049-6945-1793361334045-0.png) | NOT_RUN |
| 1049:7058 | /ads · Admin Mobile · 廣告草稿 · source | parity/1049:7058.png | [figma/1049-7058/get_screenshot-1049-7058-1793361334851-0.png](figma/1049-7058/get_screenshot-1049-7058-1793361334851-0.png) | NOT_RUN |
| 1049:7120 | /ads · Admin Desktop · 廣告草稿 · settings | parity/1049:7120.png | [figma/1049-7120/get_screenshot-1049-7120-1793361336145-0.png](figma/1049-7120/get_screenshot-1049-7120-1793361336145-0.png) | NOT_RUN |
| 1049:7233 | /ads · Admin Mobile · 廣告草稿 · settings | parity/1049:7233.png | [figma/1049-7233/get_screenshot-1049-7233-1793361337052-0.png](figma/1049-7233/get_screenshot-1049-7233-1793361337052-0.png) | NOT_RUN |
| 1049:7295 | /ads · Admin Desktop · 廣告草稿 · review | parity/1049:7295.png | [figma/1049-7295/get_screenshot-1049-7295-1793361338469-0.png](figma/1049-7295/get_screenshot-1049-7295-1793361338469-0.png) | NOT_RUN |
| 1049:7408 | /ads · Admin Mobile · 廣告草稿 · review | parity/1049:7408.png | [figma/1049-7408/get_screenshot-1049-7408-1793361339565-0.png](figma/1049-7408/get_screenshot-1049-7408-1793361339565-0.png) | NOT_RUN |
| 1049:7470 | /ads · Admin Desktop · 廣告草稿 · saved | parity/1049:7470.png | [figma/1049-7470/get_screenshot-1049-7470-1793361341105-0.png](figma/1049-7470/get_screenshot-1049-7470-1793361341105-0.png) | NOT_RUN |
| 1049:7583 | /ads · Admin Mobile · 廣告草稿 · saved | parity/1049:7583.png | [figma/1049-7583/get_screenshot-1049-7583-1793361342801-0.png](figma/1049-7583/get_screenshot-1049-7583-1793361342801-0.png) | NOT_RUN |
| 1049:7645 | /ads · Admin Desktop · 廣告草稿 · status | parity/1049:7645.png | [figma/1049-7645/get_screenshot-1049-7645-1793361344573-0.png](figma/1049-7645/get_screenshot-1049-7645-1793361344573-0.png) | NOT_RUN |
| 1049:7758 | /ads · Admin Mobile · 廣告草稿 · status | parity/1049:7758.png | [figma/1049-7758/get_screenshot-1049-7758-1793361345566-0.png](figma/1049-7758/get_screenshot-1049-7758-1793361345566-0.png) | NOT_RUN |
| 1049:7820 | /ads · Admin Desktop · 廣告草稿 · leave | parity/1049:7820.png | [figma/1049-7820/get_screenshot-1049-7820-1793361346963-0.png](figma/1049-7820/get_screenshot-1049-7820-1793361346963-0.png) | NOT_RUN |
| 1049:7933 | /ads · Admin Mobile · 廣告草稿 · leave | parity/1049:7933.png | [figma/1049-7933/get_screenshot-1049-7933-1793361348136-0.png](figma/1049-7933/get_screenshot-1049-7933-1793361348136-0.png) | NOT_RUN |
| 1049:7995 | /ads · Admin Desktop · 廣告草稿 · conflict | parity/1049:7995.png | [figma/1049-7995/get_screenshot-1049-7995-1793361349622-0.png](figma/1049-7995/get_screenshot-1049-7995-1793361349622-0.png) | NOT_RUN |
| 1049:8108 | /ads · Admin Mobile · 廣告草稿 · conflict | parity/1049:8108.png | [figma/1049-8108/get_screenshot-1049-8108-1793361350258-0.png](figma/1049-8108/get_screenshot-1049-8108-1793361350258-0.png) | NOT_RUN |
| 1134:9548 | /ads · Admin Desktop · 廣告列表 · 狀態說明 | parity/1134:9548.png | [figma/1134-9548/get_screenshot-1134-9548-1793361351393-0.png](figma/1134-9548/get_screenshot-1134-9548-1793361351393-0.png) | NOT_RUN |
| 1134:9549 | /ads · Admin Mobile · 廣告列表 · 狀態說明 | parity/1134:9549.png | [figma/1134-9549/get_screenshot-1134-9549-1793361352015-0.png](figma/1134-9549/get_screenshot-1134-9549-1793361352015-0.png) | NOT_RUN |
| 1134:9550 | /ads · Admin Desktop · 廣告列表 · 讀取回饋 | parity/1134:9550.png | [figma/1134-9550/get_screenshot-1134-9550-1793361353077-0.png](figma/1134-9550/get_screenshot-1134-9550-1793361353077-0.png) | NOT_RUN |
| 1134:9551 | /ads · Admin Mobile · 廣告列表 · 讀取回饋 | parity/1134:9551.png | [figma/1134-9551/get_screenshot-1134-9551-1793361353601-0.png](figma/1134-9551/get_screenshot-1134-9551-1793361353601-0.png) | NOT_RUN |
| 1151:9420 | /ads/attribution · Admin Desktop · 成效總覽 · DaWan Live | parity/1151:9420.png | [figma/1151-9420/get_screenshot-1151-9420-1793361354607-0.png](figma/1151-9420/get_screenshot-1151-9420-1793361354607-0.png) | NOT_RUN |
| 1151:9421 | /ads/attribution · Admin Mobile · 成效總覽 · DaWan Live | parity/1151:9421.png | [figma/1151-9421/get_screenshot-1151-9421-1793361355231-0.png](figma/1151-9421/get_screenshot-1151-9421-1793361355231-0.png) | NOT_RUN |
| 1151:9422 | /ads/attribution · Admin Desktop · 廣告歸因明細 · DaWan Live | parity/1151:9422.png | [figma/1151-9422/get_screenshot-1151-9422-1793361356209-0.png](figma/1151-9422/get_screenshot-1151-9422-1793361356209-0.png) | NOT_RUN |
| 1151:9423 | /ads/attribution · Admin Mobile · 廣告歸因明細 · DaWan Live | parity/1151:9423.png | [figma/1151-9423/get_screenshot-1151-9423-1793361357321-0.png](figma/1151-9423/get_screenshot-1151-9423-1793361357321-0.png) | NOT_RUN |
| 1151:9428 | /ads/attribution · Admin Desktop · Meta 分組明細 · DaWan Live | parity/1151:9428.png | [figma/1151-9428/get_screenshot-1151-9428-1793361358350-0.png](figma/1151-9428/get_screenshot-1151-9428-1793361358350-0.png) | NOT_RUN |
| 1151:9429 | /ads/attribution · Admin Mobile · Meta 分組明細 · DaWan Live | parity/1151:9429.png | [figma/1151-9429/get_screenshot-1151-9429-1793361358989-0.png](figma/1151-9429/get_screenshot-1151-9429-1793361358989-0.png) | NOT_RUN |
| 4:11612 | /ads · Admin · 营销 · SHOPLINE | parity/4:11612.png | [figma/4-11612/get_screenshot-4-11612-1793361359948-0.png](figma/4-11612/get_screenshot-4-11612-1793361359948-0.png) | NOT_RUN |
| 4:11750 | /ads · Admin Mobile · 营销 · SHOPLINE | parity/4:11750.png | [figma/4-11750/get_screenshot-4-11750-1793361360696-0.png](figma/4-11750/get_screenshot-4-11750-1793361360696-0.png) | NOT_RUN |
| 6:18113 | /promotions · Admin · 客户分群 · SHOPLINE | parity/6:18113.png | [figma/6-18113/get_screenshot-6-18113-1793361361726-0.png](figma/6-18113/get_screenshot-6-18113-1793361361726-0.png) | DEFERRED: segment DTO absent |
| 6:18232 | /promotions · Admin Mobile · 客户分群 · SHOPLINE | parity/6:18232.png | [figma/6-18232/get_screenshot-6-18232-1793361362834-0.png](figma/6-18232/get_screenshot-6-18232-1793361362834-0.png) | DEFERRED: segment DTO absent |
| 1194:7740 | /promotions · Admin · 营销 · SHOPLINE · 功能關閉 | parity/1194:7740.png | [figma/1194-7740/get_screenshot-1194-7740-1793361364024-0.png](figma/1194-7740/get_screenshot-1194-7740-1793361364024-0.png) | NOT_RUN |
| 1194:7796 | /promotions · Admin Mobile · 营销 · SHOPLINE · 功能關閉 | parity/1194:7796.png | [figma/1194-7796/get_screenshot-1194-7796-1793361364571-0.png](figma/1194-7796/get_screenshot-1194-7796-1793361364571-0.png) | NOT_RUN |

## Copy table (RED targets; existing dictionaries retained)

| zh-TW | zh-CN | en |
|---|---|---|
| 廣告草稿 | 广告草稿 | Ad drafts |
| 新增草稿 | 新增草稿 | New draft |
| 建立廣告草稿 | 创建广告草稿 | Create ad draft |
| 選擇貼文或商品 | 选择帖文或商品 | Choose post or product |
| 修改預算、排期與對象 | 修改预算、排期和受众 | Edit budget, schedule and audience |
| 儲存草稿 | 保存草稿 | Save draft |
| 核准前確認 | 审批前确认 | Review before approval |
| 草稿已儲存 | 草稿已保存 | Draft saved |
| 要保留這些未儲存的修改嗎？ | 要保留这些未保存的修改吗？ | Keep these unsaved changes? |
| 另一位管理員已更新草稿 | 另一位管理员已更新草稿 | Another administrator updated this draft |
| 狀態說明 | 状态说明 | Status guide |
| 資產授權 | 资产授权 | Asset authorization |
| 歸因報表 | 归因报表 | Attribution report |
| 成效總覽 | 成效总览 | Results overview |
| 廣告歸因 | 广告归因 | Ad attribution |
| Meta 分組明細 | Meta 分组明细 | Meta breakdown |
| 重新讀取 | 重新读取 | Refresh |
| 平台啟用前，此商店的廣告功能為關閉狀態。 | 平台启用前，此店铺的广告功能为关闭状态。 | Ads are off for this store until the platform enables them. |

Full visible-string table and per-control executed ledger remain required before implementation acceptance. This RED handoff does not claim UI delivery.
