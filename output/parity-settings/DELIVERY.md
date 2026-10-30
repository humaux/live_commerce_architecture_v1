# PARITY-SETTINGS implementation hand-off · 2026-10-30

Status: implementation pushed; relay 4 repairs the settings J4 caller after native CI failure. Current-head GREEN/E4 acceptance PENDING. Author PASS would be E0.
Role: dispatched UI implementer; model family GPT-6 (runtime model/effort not independently exposed); no delegation.
Branch/worktree: unit/parity-settings / .worktrees/parity-settings. Initial base df35b338b5cf; integration refresh 3ea359f79 merged before ownership regeneration. Only this worktree is writable.

## Authority and result

Owner card and verbatim AD-04 appended to contracts/merchant-settings-wizard-v1.md; UX-settings DELIVERY current “UX-settings polish · 2026-10-29” section. Fleet read-only get_design_context/get_screenshot/get_metadata readbacks succeeded on this relay for overview, three-step flow and review. Local PNGs under figma/ are design reads, not product acceptance. No design edits or restart.

Top actions now precede the three summary rows: primary start, secondary full editor. Setup remains available after server completion. No extra Advanced button or sessionStorage view choice. OAuth meta_connect/meta_error/#facebook-instagram still open the existing full connection card. Existing provider, storefront, COD, bank and notification controls remain in the full editor.

Explicit delivery save uses existing market → policy → MANUAL service commands sequentially. Defaults: TW/tw/台灣/store currency; shipping 0, inclusive tax at 0 bps, country_flat, goods basis, existing 300s quote default, exact owner reference. Existing services keep policy and other values; only enabled flips. Missing/ambiguous/incomplete reads remain unavailable. An existing inactive tw market or conflicting orphan policy requires full-editor correction; neither is silently overwritten. Each command is journaled before dispatch; only explicit save replays an uncertain command with identical key/body. No carrier call, label, tracking, shipping capability or payment eligibility is inferred.

## UI PROCESS · BOURDIEU

|Question|Verdict|
|---|---|
|Q1 novice task understanding|Three task rows and direct names; novice means recent experience with this task. Real five-second comprehension UNKNOWN.|
|Q2 experienced direct access|Full editor one click away; setup optional and repeatable. CI click evidence PENDING.|
|Q3 payment/delivery consequences|Persisted delivery count is configuration only; review keeps Stripe timing/eligibility, no COD, merchant-arranged delivery and checkout caveats.|
|Q4 memory/search burden|No merchant-entered service code, policy version, market or carrier setup in the simple flow. Existing defaults reused only under AD-04.|
|Q5 device parity|Same fields, summary facts and commands on desktop/mobile; shared component. CI screenshots/click persistence PENDING.|
|Q6 aesthetics|UNKNOWN until owner; no author aesthetic approval.|

## UI PROCESS · PROJECT UNDERSTANDING

Read AGENTS.md and 架构.md §§0/1/2/26/27: evidence modes stay separate; Go/PG owns transaction facts; authenticated store scope controls values; isolated worktree and independent review required. Read merchant-settings-wizard-v1.md, settings model/client, SettingsWizard, SettingsV2 and pricing validation: bounded read discovery, existing API only, sequential policy/service writes, scoped journals and unchanged-session fences. Owner AD-04 supersedes no-auto-seeding on these two surfaces; no other policy or contract revision.

## UI PROCESS · MOBBIN → EAGLE

Read existing Eagle folder MVQL9YZXCBO8X via item/list: success, 8 items (eagle-settings.json, local evidence). Existing imported Mobbin candidates MVOVLMG9N4TCQ (Shopify) contributes task grouping; MVOVLMGBSNKXG (Wise) contributes one selection/status per row. No imagery, brands or product facts copied. This is a signed implementation, with no new reference import or missing Mobbin query. Reference images were not re-audited this relay.

## UI PROCESS · FIGMA OFFICIAL GUIDANCE

Read ux-settings/output/ux-audit-ref/FIGMA-GUIDE.md (project guide dated 2026-10-28). Applied G1 [auto layout](https://help.figma.com/hc/en-us/articles/360040451373-Guide-to-auto-layout-in-Figma) as flex/gap layouts; G2 [variables](https://help.figma.com/hc/en-us/articles/14506821864087-Overview-of-variables-collections-and-modes) as existing semantic CSS tokens; G3 [components/variants](https://help.figma.com/hc/en-us/articles/360056440594-Create-and-use-variants) as shared controls/state; G4 [prototyping](https://help.figma.com/hc/en-us/articles/360040314193-Guide-to-prototyping-in-Figma) as real forward/back/review/save/recovery paths. No Figma write or new frame; copy-lint of changed design frames is not applicable.

## DESIGN PARITY · required fields and facts

|Required desktop fact|Mobile equivalent|Summary/review evidence target|
|---|---|---|
|Start setup + Edit settings; three rows|32319 → 32392, shared overview|Ordered exact-role/name assertions in all three locales; screenshot pair|
|Store name, email, optional phone|33165 → 33243|Same draft fields retained in review 32781/32846 and persisted profile; existing click journey|
|Five named delivery kinds and independent choices; added owner Select all|33321 → 33467|Each choice in review; enabled count derived from saved services after refresh|
|First product name, store currency/price, stock, draft status|33613 → 33693|Product facts retained in review; existing product/SKU/stock replay safety journey|
|Stripe eligibility/timing and no cash on delivery|32781 → 32846|Shared review copy, independent of enabled delivery count|
|Merchant arranges shipping, no automatic labels; checkout confirms availability|32781 → 32846|Shared review copy; no capability inference|
|Public store URL only if one serving domain is returned|Shared review component|No invented example domain; absent data is omitted|

Screenshot parity index (real-route CI artifacts pending):

|Frame|Browser artifact|DELIVERY PNG under `.worktrees/ux-settings/output/ux-settings/`|
|---|---|---|
|1429:32319|parity/1429-32319.png|polish-r4/get_screenshot-1429-32319-1793203940222-0.png|
|1429:32392|parity/1429-32392.png|polish-r4/get_screenshot-1429-32392-1793203940313-0.png|
|1429:33165|parity/1429-33165.png|shots-simple/get_screenshot-1429-33165-1793203339018-0.png|
|1429:33243|parity/1429-33243.png|shots-simple/get_screenshot-1429-33243-1793203339286-0.png|
|1429:33321|parity/1429-33321.png|shots-simple/get_screenshot-1429-33321-1793203339572-0.png|
|1429:33467|parity/1429-33467.png|shots-simple/get_screenshot-1429-33467-1793203339816-0.png|
|1429:33613|parity/1429-33613.png|shots-simple/get_screenshot-1429-33613-1793203340091-0.png|
|1429:33693|parity/1429-33693.png|shots-simple/get_screenshot-1429-33693-1793203340331-0.png|
|1429:32781|parity/1429-32781.png|shots-simple/get_screenshot-1429-32781-1793203337303-0.png|
|1429:32846|parity/1429-32846.png|shots-simple/get_screenshot-1429-32846-1793203337585-0.png|

## Click ledger

All actual results PENDING on implementation-head CI; test source is not a PASS receipt.

|Page/control|Action|Expected visible/persistent result|Actual|
|---|---|---|---|
|Overview start/edit/three rows, three locales|click|Setup, full editor or simple editor according to signed entry|PENDING|
|Full editor, reload, locale change|reload/select + click Edit settings|Overview by default; draft/command safety retained on explicit reopening|PENDING|
|OAuth return routes|navigate return URL|Facebook/Instagram card|PENDING|
|Quick setup fields, next/back, skip|fill/click|Draft preserved; explicit marker dismissal retains truthful derived steps|PENDING|
|Zero-market home and Select all|check/uncheck|Independent five choices, no write on read; review shows values|PENDING|
|Review/back/save|click|Market then policy then service; lost policy reply stops later writes|PENDING|
|Uncertain save, refresh, explicit save|reload/click|Same key/body replay, five persisted services, saved count 5|PENDING|
|Existing delivery switches|check/save/reload|Own codes/prices/policies retained, five enabled choices|PENDING|
|Profile/product/stock/session safety|fill/click/reload|Existing conflict, replay, changed-session and initial-stock assertions remain|PENDING|

## Density and three-language copy

Read the integrator density-probe.json counting convention (visible text nodes and characters); the existing CI real-route overview probe enforces ≤12 blocks/≤100 Chinese characters. The probe is not run locally. Rendered module-wide medians/max and visual comparison are PENDING; no new density PASS claim. No hard-coded visible strings were added; the exact market name and configuration reference are contract data. All settings-v2 visible strings follow, generated from current locale entries; existing full-editor copy is unchanged in settings-copy and sibling localized copy modules.

|Key|zh-TW|zh-CN|en|
|---|---|---|---|
|title|店鋪設定|店铺设置|Store settings|
|edit|編輯設定|编辑设置|Edit settings|
|complete|店舖資料 · 已填寫|店铺资料 · 已填写|Store details · Complete|
|incomplete|店舖資料 · 未填寫|店铺资料 · 未填写|Store details · Incomplete|
|delivery|配送方式 · 已開啟 {n} 種|配送方式 · 已启用 {n} 种|Delivery methods · {n} enabled|
|payment|收款 · Stripe（平台代收）|收款 · Stripe（平台代收）|Payments · Stripe (collected by platform)|
|start|開始三步設定|开始三步设置|Start quick setup|
|step1|1 / 3 · 店名|1 / 3 · 店名|1 / 3 · Store name|
|step2|2 / 3 · 配送|2 / 3 · 配送|2 / 3 · Delivery|
|step3|3 / 3 · 第一個商品|3 / 3 · 首件商品|3 / 3 · First product|
|name|店名|店名|Store name|
|email|信箱|邮箱|Email|
|phone|電話（選填）|电话（选填）|Phone (optional)|
|selectAll|全選|全选|Select all|
|home|宅配|送货上门|Home delivery|
|seven|7-ELEVEN|7-ELEVEN|7-ELEVEN|
|family|全家|全家|FamilyMart|
|hilife|萊爾富|莱尔富|Hi-Life|
|ok|OK|OK|OK|
|on|開放|已开启|on|
|off|關閉|已关闭|off|
|shipping|自行出貨；寄出後登記承運商與單號。|自行发货；寄出后登记承运商和运单号。|Arrange shipping; record the carrier and tracking number after dispatch.|
|shippingReview|自行出貨；登記承運商與單號，不自動建單或購買面單。|自行发货；登记承运商和运单号，系统不自动建单或购买面单。|Arrange shipping and record the carrier and tracking number. No automatic shipment creation or label purchase.|
|paymentReview|Stripe 收款待確認；付款時間依買家方式，不含貨到付款。|Stripe 收款资格待确认；付款时间取决于买家选择的方式，不含货到付款。|Stripe payment eligibility pending; payment timing depends on the buyer’s method. No cash on delivery.|
|checkout|配送以下單確認為準。|能否配送以下单时确认为准。|Delivery availability is confirmed at checkout.|
|publicURL|商店網址|店铺网址|Store URL|
|unavailable|未填寫|未填写|Not provided|
|prerequisite|請在編輯設定確認市場與配送費用。|请在编辑设置中核对市场和配送费用。|Check the market and delivery pricing in Edit settings.|
|review|核對設定|核对设置|Review settings|
|save|儲存設定|保存设置|Save settings|
|back|返回總覽，保留草稿|返回概览，保留草稿|Back to overview, keep draft|
|backEdit|返回修改|返回修改|Back to edit|
|nextDelivery|下一步：配送|下一步：配送|Next: delivery|
|nextProduct|下一步：商品|下一步：商品|Next: product|
|backName|上一步：店名|上一步：店名|Back: store name|
|backDelivery|上一步：配送|上一步：配送|Back: delivery|
|skip|略過|跳过|Skip|
|product|商品名稱|商品名称|Product name|
|price|售價（{currency}）|售价（{currency}）|Price ({currency})|
|stock|庫存（件）|库存（件）|Stock (units)|
|draft|商品儲存為草稿，尚未上架。|商品将保存为草稿，尚未上架。|Product is saved as a draft, not published.|
|productDraft|商品草稿|商品草稿|Product draft|
|saved|設定已儲存|设置已保存|Settings saved|
|eligibility|收款資格仍待確認。|收款资格仍待确认。|Payment eligibility is still pending.|
|failed|設定未儲存|设置未保存|Settings not saved|
|retained|原設定不變，草稿已保留。|原设置未变，草稿已保留。|Saved settings are unchanged; your draft is retained.|
|partial|部分設定已儲存；請重新讀取後核對。|部分设置已保存；请重新加载并核对。|Some settings were saved. Reload and review them.|
|uncertain|尚未取得最新狀態|暂时无法获取最新状态|Latest status unavailable|
|reloadHelp|請重新讀取；原設定保留。|请重新加载；原设置保留。|Reload; your saved settings are retained.|
|reload|重新讀取|重新加载|Reload|
|advanced|進階|进阶|Advanced|
|loading|正在讀取設定…|正在加载设置…|Loading settings…|
|session|登入狀態已變更，請重新登入。|登录状态已更改，请重新登录。|Your session changed. Sign in again.|
|pending|上次儲存尚待確認；請核對並儲存原草稿，或重新讀取最新狀態。|上次保存仍待确认；请核对并保存原草稿，或重新加载最新状态。|Your last save still needs confirmation. Review and save the original draft, or reload the latest status.|

## Evidence and scope

RED: 0cb0b29f, run/job 41049964850/124044946555; missing start button assertion (red.log and red-artifact/playwright.log). Synthetic merge ea3f8509265ac6778df62a75df9ef1d773c28510. E3 applies only to that assertion failure; later new assertions were not reached. Foundation 41049964792/124043291284 success on tests-only head is not implementation acceptance.
Static commands: tsc --noEmit with output/parity-settings/tsconfig.json (admin + two settings specs); python3 scripts/check_packet.py; node scripts/dev/derive-browser-ownership.mjs then --check; bash scripts/dev/depmap.sh; git diff --check. Final receipts in red.log. Local tests and servers NOT_RUN by instruction.
No SettingsWizard/SettingsV2 compilation harness was found by targeted tests/ search; no new runtime imports were added, only existing type imports expanded. Gate adapter assertions, exemptions, baselines and thresholds untouched. Literal outside-card paths: actual components/SettingsV2.tsx location, components/settings.css, generated browser-ownership.json and this evidence directory; same-module/generated pre-approval, integrator review requested. Depmap regeneration produced no diff.
No production/SANDBOX/LIVE/provider/independent acceptance. No Humaux tool surface: store/canvas/coord/code-index/link unavailable, no shared-state write or completion claim. Integrator relays a new session after WAIT_RUN completion. Temporary dependency symlinks are removed before hand-off; no local server/fixture/container was started.

Source SHA256 before implementation push (static source binding only):

- `apps/admin/components/SettingsV2.tsx`: `bdc561dc85febf65962f85810844b8b567ada3b3cacfb813865500652cf8759d`
- `apps/admin/components/SettingsWizard.tsx`: `9d6dbbff0f8f363fa7010a4fde5d0c7a055b96633f6c02564a1ef965e105149e`
- `apps/admin/components/settings.css`: `230a2b4bcb5b7c62d2895ce2f828234a1432666337ce578d3b503fa2d853d186`
- `apps/admin/lib/settings-v2.ts`: `dd6d8aaecf824f80f98fedbd9874c491ca514560af7a02207d1100f9999fc456`
- `packages/i18n/src/index.ts`: `af8f249198cb9b8301df0d738beca143e5288f1ffb596a9f0465af1d21a91004`
- `tests/admin/settings-v2.spec.ts`: `04ea788915169cff4ff34dafc2e4ba05476fd9c784e5bf109df719e1cc3ed7ef`
- `tests/admin/settings-real.spec.ts`: `be0b148436b818f720fbb0a53f0d13b42c6eb23fc4c9330022fdcdd5b0b9f8f8`

## Relay 4 — settings journey entry and refresh

Native CI at implementation head `6db6e486d20b660ca2ff88f645593ec8fe14252b`:
Gates `41054881458` / job `124070751799`, artifact `12980896041`, synthetic merge
`44dae94e3d3d915fd75f5e123e57d81756f776b2`. J4 timed out clicking removed「進階」;
raw failure retained in `relay4-browser.log` (E3 failure only). Foundation
`41054881503` / `124059758089` succeeded; this is partial evidence, not GREEN.

Targeted fix attempt 1 updates the settings caller in `tests/ui/click-sweep.mjs`
to click「編輯設定」at entry and after refresh. This follows the card's full editor
entry and nonpersistent view choice. DNS instructions, TXT format, CNAME and
persisted domain `REQUESTED` assertions are unchanged. No visual/product changes.

|Page/control|Real operation|Expected|Actual|
|---|---|---|---|
|zh-TW settings / 編輯設定|click after navigation|full editor and domains card visible|PENDING new-head CI|
|domains / Request|fill hostname and click|TXT and CNAME instructions|PENDING new-head CI|
|settings after refresh / 編輯設定|click|domain remains REQUESTED|PENDING new-head CI|

Fetched and merged `origin/r3/integration` before spec change: already up to date.
Static node syntax, ownership generation/--check and diff check exit 0; depmap
regenerated with no diff. No local runtime test/server. Extra literal path:
`tests/ui/click-sweep.mjs` (settings J4 only; integrator review required).
Humaux is available this relay; earlier unavailable statements are historical.
Store/canvas receipts and the final WAIT_RUN IDs are reported in the handoff/PR.
Integrator supplies a fresh session on completion/failure/cancellation; queue >3h
is escalated to integrator. GREEN, independent parity review, E4 and LIVE remain pending.
