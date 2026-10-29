# CATALOG-V2-IMPL · RED batch handoff

Status: tests-only candidate. Implementation NOT_STARTED pending causal CI RED. Author assessment E0. Syntax checks E1 only; no product completion claim.

## Authority and project understanding

UI: `.worktrees/ux-catalog/output/ux-catalog/DELIVERY.md`, current simplify 2026-10-28; SHA256 `815d8b7a56ecd725c66286eee0d428b010e88b9a9c9756b695b85ef0e9a4007b`.
Owner signed simplified goods design and density median ≤12 text blocks / ≤100 chars, every state ≤20 blocks. Architecture §§0/1/2 require evidence separation, one PG truth and B2C physical goods/single currency. §§26/27 and tasks DAG T04 require isolated ownership, no recursive delegation and independent review. `catalog-inventory-v1` §g/Boundary own merge-patch presence, CAS, inventory floor, atomic rollback and live-window archive refusal; product-media-v2 owns main4/color1/detail20 and unchanged publication prerequisite. Existing UI uses responses; no new settings, API or contracts are authorized.

## BOURDIEU Q1–Q6

Q1: recent task novices need one task per screen and recognizable actions. Q2: repeat users need stable row identity, filters and preserved selection. Q3: no checkout or currency inference from locale. Q4: no market/payment/shipping inference from device, language or social group. Q5: equal required facts desktop/mobile, native controls and 44px targets. Q6: aesthetics remain a separate question; owner sign-off is supplied by the card, no new author approval is inferred. This is implementation intent, not behavioral research.

## MOBBIN → EAGLE

Module folder `MVS1D5NC7KI5T`; candidates Shopify Products iOS `MVS1D603YAUS5`, Square Add variations `MVS1D5ZMW22YA`. Read receipt `design/eagle-read.json`. Signed layout takes compact identity/price/stock/status and adjacent action from Shopify; do not reuse reference images, brands or facts. No new visual direction or reference import. No Mobbin tool in this lane; no new pattern/query required for the signed scope.

## FIGMA OFFICIAL GUIDANCE

Read official-guide index `.worktrees/ux-catalog/output/ux-audit-ref/FIGMA-GUIDE.md` dated 2026-10-28: G1 nested auto layout → CSS flow; G2 semantic variables → incumbent UI tokens; G3 components/variants → existing control composition; G4 prototypes → explicit review/result/back paths; G5 failures → preserved inputs and per-item results; G6 route/data/permission annotations → test and plan boundary; G7 audit before implementation. Official primary links are in that authority index. No Figma writes, no copy-lint on unchanged frames.
Fleet Bridge status returned a connected document, no pending inspection. Read list root 1429:62906 via get_design_context, screenshot `design/list-mobile.png`. Its live image also contains “集合分類/草稿編輯”, which are absent from the signed current 89-string table. Record as live-design drift; do not silently add them to the implementation scope. Before implementation reconcile every screen against signed DELIVERY/SPEC and read the other roots. Existing source still has broad quick-edit/search/actions; no source UI modified in this RED batch.

## Design parity checklist (required desktop/mobile/review/result)

|Required fact|Desktop/mobile target|Review/result requirement|Additive browser assertions|
|---|---|---|---|
|Product name, price/currency, status, available/sold out|list/all/active/draft/archived and updated|archive review/result retain each identity|six list cases, locale-specific synthetic responses, real filter/edit clicks|
|Selected product count and individual archive outcomes|selected and bulk review|2 archived/1 unchanged, live refusal, old orders retained|archive confirm/cancel, exact three IDs, refusal reason, refresh counts|
|Name, description and selling status|edit|review/saved preserve values and status|fill and review/cancel/save; absent status patch; reload readback|
|Both variant identities, prices and stock8/5/3 plus0/0/0|variants|each SKU remains identifiable|variant names/codes, two price patches targeting original IDs|
|Counted total3 below committed5; all edits unsent|stock/error|retain3; keep original inventory|native input, exact visible error, zero write commands, recovery/back|
|Main4, one image per color, detail20, upload constraints|media|counts and publication unchanged|role labels, four native file chooser paths canceled; no uploads fabricated|
|Publication, delivery and historical orders unchanged|edit/review/saved|no unintended fields submitted|command patch preserves options/status; broader parity pending CI/implementation|

## Gate and click ledger

New `tests/admin/catalog-v2-simplified.acceptance.ts`: 6 locale/viewport list cases, 2 archive, 2 edit, 2 image and 1 aggregate density case. Existing catalog-core.spec imports/registers additively only when PRODUCT_EDITOR_ACCEPTANCE=1 (existing TWD fixture); original CC12 USD/PE assertions untouched. Native click/fill/filechooser and refresh; evaluation only reads DOM geometry/text. Each operation records page/control/operation/expected/actual/PASS-or-FAIL and attaches `catalog-v2-click-ledger.json`; current ledger status NOT_RUN. Density collects all 15 signed mobile states from visible DOM text plus input values, checks no missing state, max20/median12/100. Never hides text from its metric. Fixture response state survives reload within a test; evidence is MOCK, not PG durability.

## Local static receipts and limits

`node --check` new acceptance + existing spec: exit0. `git diff --check`: exit0. `node scripts/dev/derive-browser-ownership.mjs`: exit0,53 modes, new input attached to catalog-core/product-editor owners. `pnpm exec tsc ...`: exit254, tsc unavailable because this worktree has no installed dependencies; no installation/build/test attempted. This is NOT a causal RED. CI must execute and produce its own assertion failures. No local tests, browser, server, PG, container or provider action.

RED log: `red.log` (PENDING until CI). GREEN/implementation/independent review/real clicks/persistence/density/3-locale browser all NOT_RUN. No production/LIVE/deploy. Integrator relays completed Foundation/Gates runs into a fresh session; no polling beyond ~10 minutes, no background process owned here.

## Three-language signed copy inventory

Reference excerpt from the current signed authority, for implementation review only; its root authority remains the signed DELIVERY, not this copy.
（當前全部89條可見字串）

僅Figma畫面為zh-TW；下表為現行copy inventory，取代下方歷史表。應用端三語與瀏覽器驗證 **NOT_RUN**。

|zh-TW|zh-CN|en|
|---|---|---|
|2 件已歸檔，1 件未變更|2 件已归档，1 件未更改|2 archived, 1 unchanged|
|3|3|3|
|JPG、PNG、WebP；每張最多 2 MB；詳情圖高度最多寬度六倍。|JPG、PNG、WebP；每张最多 2 MB；详情图高度不超过宽度六倍。|JPG, PNG, WebP; max 2 MB each; detail height up to six times width.|
|NT$1,280|NT$1,280|NT$1,280|
|NT$1,280 · 可售 3|NT$1,280 · 可售 3|NT$1,280 · Available 3|
|NT$1,380|NT$1,380|NT$1,380|
|NT$490|NT$490|NT$490|
|NT$490 · 可售 16|NT$490 · 可售 16|NT$490 · Available 16|
|NT$690|NT$690|NT$690|
|NT$690 · 可售 0|NT$690 · 可售 0|NT$690 · Available 0|
|主圖 0 / 4 · 規格圖 0 / 2 · 詳情圖 0 / 20|主图 0 / 4 · 规格图 0 / 2 · 详情图 0 / 20|Main 0 / 4 · Variant 0 / 2 · Detail 0 / 20|
|主圖 0/4 · 上架必填|主图 0/4 · 上架必填|Main 0/4 · Required to publish|
|保溫杯|保温杯|Insulated cup|
|保留目前庫存|保留当前库存|Keep current stock|
|全部 3|全部 3|All 3|
|兩款售價 · TWD|两款售价 · TWD|Both prices · TWD|
|取消變更|取消更改|Cancel changes|
|可售 3|可售 3|Available 3|
|商品|商品|Products|
|商品名稱|商品名称|Product name|
|商品圖片 · 未加入|商品图片 · 未添加|Product images · None|
|商品已儲存|商品已保存|Product saved|
|商品說明|商品说明|Description|
|在售|在售|Active|
|在售 1|在售 1|Active 1|
|在售 2|在售 2|Active 2|
|在售 · 可售 3|在售 · 可售 3|Active · 3 available|
|在售 · 售罄|在售 · 售罄|Active · Sold out|
|在售 → 歸檔|在售 → 归档|Active → Archived|
|尚無已歸檔商品|暂无已归档商品|No archived products|
|已占用 5|已占用 5|Committed 5|
|已歸檔|已归档|Archived|
|已歸檔 0|已归档 0|Archived 0|
|已歸檔 2|已归档 2|Archived 2|
|已歸檔 · 可售 16|已归档 · 可售 16|Archived · 16 available|
|已歸檔 · 可售 3|已归档 · 可售 3|Archived · 3 available|
|已選 3 件|已选 3 件|3 selected|
|帆布提袋|帆布手提袋|Canvas tote|
|帆布提袋 · 圖片|帆布手提袋 · 图片|Canvas tote · Images|
|帆布提袋 · 米白庫存|帆布手提袋 · 米白库存|Canvas tote · Ivory stock|
|帆布提袋 · 規格與庫存|帆布手提袋 · 规格与库存|Canvas tote · Variants and stock|
|日常外出使用。|适合日常外出。|For everyday outings.|
|未儲存：3 件少於占用的 5 件；全部變更未送出。|未保存：3 件少于占用的 5 件；全部更改未提交。|Not saved: 3 is below 5 committed. No changes submitted.|
|未加入主圖；上架、配送與舊訂單不變。|未添加主图；上架、配送和原订单不变。|No main image. Publication, delivery and existing orders unchanged.|
|未變更 · 仍在售|未更改 · 仍在售|Unchanged · Active|
|查看商品列表|查看商品列表|View products|
|核對庫存|核对库存|Check stock|
|核對變更|核对更改|Review changes|
|棉質上衣|纯棉上衣|Cotton top|
|檢查變更|检查更改|Review changes|
|歸檔商品|归档商品|Archive products|
|歸檔後停止接單；舊訂單保留。|归档后停止接单；保留原订单。|Archiving stops new orders. Existing orders remain.|
|深藍 0 / 1|深蓝 0 / 1|Navy 0 / 1|
|深藍 · NT$1,280 → NT$1,380|深蓝 · NT$1,280 → NT$1,380|Navy · NT$1,280 → NT$1,380|
|深藍 · NT$1,380|深蓝 · NT$1,380|Navy · NT$1,380|
|深藍 · 售罄|深蓝 · 售罄|Navy · Sold out|
|現有 0 · 占用 0 · 可售 0|现有 0 · 占用 0 · 可售 0|On hand 0 · Committed 0 · Available 0|
|現有 0 · 占用 0 · 可售 0 · 售罄|现有 0 · 占用 0 · 可售 0 · 售罄|On hand 0 · Committed 0 · Available 0 · Sold out|
|現有 8|现有 8|On hand 8|
|現有 8 · 占用 5 · 可售 3|现有 8 · 占用 5 · 可售 3|On hand 8 · Committed 5 · Available 3|
|盤點總量至少 5 件。|盘点总量至少 5 件。|Counted total must be at least 5.|
|直播接單中，暫不能歸檔。|直播接单中，暂不能归档。|Live orders open. Cannot archive yet.|
|直播接單中，結束後再歸檔。|直播接单中，结束后再归档。|Archive after live orders close.|
|確認儲存|确认保存|Confirm save|
|確認歸檔|确认归档|Confirm archive|
|確認歸檔 3 件|确认归档 3 件|Archive 3 products|
|米白|米白|Ivory|
|米白 0 / 1|米白 0 / 1|Ivory 0 / 1|
|米白 · NT$1,280 → NT$1,380|米白 · NT$1,280 → NT$1,380|Ivory · NT$1,280 → NT$1,380|
|米白 · NT$1,380|米白 · NT$1,380|Ivory · NT$1,380|
|編輯|编辑|Edit|
|編輯商品|编辑商品|Edit product|
|繼續編輯|继续编辑|Continue editing|
|草稿 0|草稿 0|Draft 0|
|草稿 1|草稿 1|Draft 1|
|草稿 · 可售 16|草稿 · 可售 16|Draft · 16 available|
|草稿 → 歸檔|草稿 → 归档|Draft → Archived|
|規格與庫存|规格与库存|Variants and stock|
|詳情圖 0/20|详情图 0/20|Detail 0/20|
|調整庫存|调整库存|Adjust stock|
|調整後現有|调整后现有|New on-hand quantity|
|返回商品編輯|返回商品编辑|Back to product editor|
|返回編輯|返回编辑|Back to editor|
|返回規格|返回规格|Back to variants|
|返回選取|返回选择|Back to selection|
|選取 3 件|选择 3 件|Select 3|
|選擇圖片|选择图片|Choose images|
|選擇深藍圖片|选择深蓝图片|Choose navy image|
|選擇米白圖片|选择米白图片|Choose ivory image|


