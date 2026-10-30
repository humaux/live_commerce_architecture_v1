# PARITY-ORDER-OPS — tests-only RED handoff

2026-10-30. **E1 syntax only; direct registration repair awaits CI. Causal RED pending. Implementation, GREEN and independent review NOT_RUN.**

## Assignment and authority

- One card/branch/worktree: `PARITY-ORDER-OPS`, `unit/parity-order-ops`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-order-ops`.
- Assigned by the integrator. Base `3ea359f796d36153bb4b8aa7259e4e0c27676b86`; fetched/merged `origin/r3/integration`: already up to date. Main checkout remained read-only.
- Role: dispatched Codex implementer. Actual runtime model/effort identifiers UNKNOWN; no subagents or external models used.
- Scope: integrator's current human card, `output/ui-parity/FIGMA-INVENTORY.md` §3 C5 and Appendix A buckets U:manual-order/U:returns/U:merchant-cvs-tool. CVS route ruling: `/[locale]/orders/cvs-match`.
- UI: file `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`. All 40 exact roots read through Fleet Bridge `get_design_context` AND `get_screenshot`, no Figma writes. Root exports and context SHA256s: [frames.json](frames.json). Bridge reported a connected Untitled document/fileKey:null; configured default file and exact known node identities are the available identity evidence.
- Current design handoffs read: `ux-cvs-match/output/ux-cvs-match/DELIVERY.md` fix2 (retained signed baseline plus current real-message additions); `ux-orders/output/ux-orders/DELIVERY.md` marks polish-short-number current and excludes older screenshots. Its 60-frame orders set is outside C5. No separate current manual-order/returns DELIVERY was found; the exact live C5 Figma roots remain their UI authority.
- Humaux MCP/code-index/coord tools are not available. Search/get/rejected enumeration/store/canvas/claim/lock were **not performed**. This artifact is a handoff, not a replacement memory authority. Integrator must retain the assignment/run receipt in Humaux.

## RED gate and static evidence

- New `tests/admin/parity-order-ops.spec.ts` and existing `tests/admin/returns-ui.spec.ts` are directly registered in `TestBrowserReturnsUI` / `--browser-returns-ui` with a signed MOCK IdP, real Go/Next and isolated PG. No indirect spec import is needed.
- Relay 1: head `7a8e296d140bbca04cc13006ccf04cac9af9aa1e`, Foundation run `41052644741` / job `124052242197` failed because `check-gates` does not recognize indirect spec registration. This is not causal assertion RED. Raw filtered receipt: [foundation-registration-failure.log](foundation-registration-failure.log). Targeted fix attempt 1/2 registers the new spec directly in the existing harness, preserves every assertion and regenerates ownership. Application implementation remains untouched until assertion RED.
- 24 cases: four flows × zh-TW/zh-CN/en × 1440×900/390×844. TW produces one screenshot for each of the 40 listed frames. A failed early assertion intentionally prevents later screenshots; an absent screenshot is NOT_RUN, never parity acceptance.
- Expected RED on original source: exact visible headings `1 · 商品`, `2 · 收件人與配送`, `退款與售後`, `查門市編號` (and translated counterparts); legacy flat headings are asserted absent. Missing CVS route must fail the visible heading assertion, not compilation.
- MOCK GET fixtures use current ProductSummary/ProductDetail/ManualOption/Rma wire shapes. Product names/prices are synthetic; CVS stores are public directory facts. Identity and store authorization are not stubbed. No POST fixture, actual refund, money calculation or production action.
- Click/fill/select operations drive draft/picker quantities, cancellation, RMA navigation/filter reload, CVS explicit selection/copy/change/paste and manual-order return. Clipboard/layout evaluates are reads only. Per-test `parity/clicks-*.json` records covered flow steps and actual results; exhaustive control accounting remains an implementation acceptance item.
- `node --check tests/admin/parity-order-ops.spec.ts`: exit 0; `node --check tests/admin/returns-ui.spec.ts`: exit 0.
- Relay 1 static commands repeated after registration repair: both syntax checks exit 0; `gofmt -l tests/foundation/browser_returns_ui_test.go` exit 0 with no formatting changes; no local tests/servers.
- `node scripts/dev/derive-browser-ownership.mjs`: exit 0 / 53 modes. `node scripts/dev/derive-browser-ownership.mjs --check`: exit 0. Generated artifact committed with registration.
- `git diff --check`: exit 0. TypeScript compiler/dependencies are absent locally; tsc NOT_RUN. No local tests, builds or servers started. Go/PG/browser/CI validation NOT_RUN until GitHub receipts.
- No new application imports yet, so component-harness import allowlists and Go dependency map are unchanged. Existing next-startup gate assertions are untouched.
- Red receipt: [red.log](red.log). Do not implement until the PR's CI demonstrates failures on the new assertions. Do not claim E3 from author expectation.

## Audit-first and legacy mapping

|Existing layout/control|Signed replacement / assertion|Existing intent retained|
|---|---|---|
|ManualOrder flat headings 商品 / 顧客 / 配送 / 付款; inline search/results|1 · 商品 / 2 · 收件人與配送; 加入商品 → 加入商品至本單 → 套用至本單; old headings absent|Search/pick distinct SKU, edit quantities, validate customer/delivery, idempotent server placement and buyer-link flow retained in existing manual-order gate.|
|ReturnsList 退貨管理 / Returns; direct RMA table as home|退款與售後 → 查看售後申請 → 售後申請; old title absent|RMA state filtering, persistent filter on refresh, record/order navigation and refund-gap handling retained. Existing pre-Figma tests will need view-entry mapping in implementation, never deletion.|
|ManualOrder free-text CVS code/name/address; no lookup route|Lookup at orders/cvs-match and shared explicit-choice picker in manual order|Leading-zero code, brand boundary, canonical public name/address; no automatic substitution or raw-paste persistence.|

## Data gaps / frame deferrals (do not invent DTO fields)

|Frames|Exact unavailable fact|Bounded work that proceeds|
|---|---|---|
|21:582/967, 750:3249/3317/3385/3457/3503/3549 and picker summary|Pre-placement server `subtotal_minor`, `shipping_minor`, `total_minor` and quote identity. ManualOption has no shipping price; ManualResult only returns total after placement.|Draft products, quantities, recipient/delivery and picker. Exact amount summary parity DEFERRED; never multiply prices or add fees in client. No API amendment assigned in this tests-only phase.|
|6:20488/20626, 30:1662/1731/1757/1818, 95:2858/2918|Rma has UUIDs/state/reason/lines/refund_id/timestamps, no human R-/LC- reference, buyer statement or merchant processing-note field. An RMA is not a refund record. Exact sample amounts must come from existing refund/order APIs, never inferred from RMA lines.|Home/list/detail composition with actual available facts; exact unsupported fields/rows DEFERRED. No buyer-note/PII addition; no synthetic field in application; refund remains MOCK and requires existing server authority.|

These partial screenshots are not a claim that deferred facts match the signed frame. Integrator must resolve permissible non-PII additive contract work separately if required; no migration number or R2 pin was assigned here.

## REQUIRED facts — desktop → mobile → summary/review

|Required field/fact|Desktop / mobile assertion|Summary/review obligation|
|---|---|---|
|Manual product, variant, quantity, catalog unit price|Both viewports use same synthetic catalog DTOs; quantity states and SKU count tested.|Same selected lines; cancellation preserves draft; client total calculation forbidden.|
|Recipient name and phone|Required by current ManualDraft/server; implementation must expose translated labels at both sizes.|Preserve only in live draft/session fence; no persistent browser PII storage or invented buyer data.|
|Destination brand, string code, canonical name, full address|All public CVS facts asserted at both sizes; leading 0 in 024967 retained.|Selected facts returned to order without submitting; query/brand changes invalidate choice.|
|Manual total and payment timing|Only server receipts are money authority; quote-summary fact gap above.|Existing placement/buyer-link gate must retain payment state and expiry checks.|
|RMA order relation, reason, state, quantities|Fixture UUID/state/reason/line data comes from existing shape; both sizes exercise navigation.|Closed/registered states separate; no RMA transition represents a refund.|
|CVS missing pasted code vs offered candidate|022305 notice and distinct unselected 024967; explicit click required.|Chosen code/address survive in chosen view; wrong input code is never silently saved.|
|CVS source/date|Use frozen public snapshot 2026-10-28, not a freshness promise.|Do not reproduce the design's YYYY-MM-DD placeholder as live data.|

## UI PROCESS — BOURDIEU Q1–Q6

Q1 task: merchant creates an order / handles after-sales / finds a store code. Q2 novice/expert means recent experience with that task, never class/device. Q3 consequences: real server placement distinct from draft; refunds never implied by return approval. Q4 recognition: product/quantity and all canonical store facts stay together. Q5 control/recovery: cancel picker leaves draft unchanged, change invalidates selection, filters survive reload. Q6 aesthetics: follow exact signed roots and owner light-shell ruling; new aesthetic judgments UNKNOWN, no redesign performed.

## UI PROCESS — PROJECT UNDERSTANDING

Read `架构.md` §§0/1/2/26/27: evidence boundaries, Go/PG transaction authority, merchant/payment boundaries, assigned write ownership, independent gates. Read `contracts/storefront-v2.md` G3: prices/server placement, inventory reservation, idempotency and buyer links; `contracts/returns-v1.md`: RMA lifecycle and refund separation. Read actual clients/parsers/components for those contracts: no new money or trust boundary may move to browser. Existing shared `packages/format/src/cvs-match.ts` is the matcher reuse point. Admin shell follows owner option A; module content follows exact roots.

## UI PROCESS — MOBBIN → EAGLE

Fresh read-only Eagle lookup: orders folder MVQL9YR448VTK success/8; CVS folder MVQL9YPJVZ5FT success/18. Existing IDs confirmed. Orders handoff references MVP0WJO2CM2IC (Squarespace: separate filters/list, aligned amounts/status) and MVP0WJO8UYAVE (Shopify: progressive same-task selection). CVS handoff references MVNM2RJ3M6VBW (Thrive: group selected facts), MVOVHC45BTJIX (Selfridges: preserve list→summary hierarchy), MVRSAQWBW1O4A (ZARA: address→options), MVRSAR4W5O6C1 (Subway: full address in row), MVNM2RJ5ULKUL (Ulta: explicit choice/change). Structural references only; no copied imagery, brands or business facts. No Mobbin tool or new import. Needed additional query: none for the signed states.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

Read project guide `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-audit/output/ux-audit/FIGMA-GUIDE.md` (official sources retrieved 2026-10-28). G1 auto layout informs draft/summary columns and mobile stacking; G2 semantic variables keeps language separate from money/store records; G3 components/variants informs shared lookup states; G4/G5 prototyping informs explicit choice/back/return paths; G6 exact-node/viewport/source annotations are preserved in frames.json; G7 reuse existing controls and matching function. This tests-only phase makes no Figma writes or new official-version claim. Figma copy-lint/density unchanged-design execution NOT_RUN; browser runtime layout/copy parity remains CI/reviewer work.

## Three-language visible-control acceptance copy

Frozen test expectations below use the signed TW wording and the current CVS DELIVERY translations where present; manual/returns translations preserve the same task meaning. Full product copy and server data rendering remain implementation work. Public store names/addresses and synthetic catalog records stay verbatim across locales.

|zh-TW|zh-CN|en|
|---|---|---|
|建立訂單|创建订单|Create order|
|1 · 商品|1 · 商品|1 · Items|
|2 · 收件人與配送|2 · 收件人与配送|2 · Recipient and delivery|
|加入商品|添加商品|Add products|
|加入商品至本單|添加商品到本单|Add products to this order|
|套用至本單|应用到本单|Apply to this order|
|取消|取消|Cancel|
|選取|选择|Select|
|數量|数量|Quantity|
|收件人|收件人|Recipient|
|聯絡電話|联系电话|Phone|
|配送方式|配送方式|Delivery method|
|檢查訂單|检查订单|Review order|
|退款與售後|退款与售后|Refunds and after-sales|
|售後申請|售后申请|After-sales requests|
|查看售後申請|查看售后申请|View after-sales requests|
|已完成的售後紀錄|已完成的售后记录|Completed after-sales record|
|查門市編號|查门店编号|Find store code|
|地址、門市名稱或編號|地址、门店名称或编号|Address, store name or code|
|更換|更换|Change|
|複製編號|复制编号|Copy code|
|複製全部|复制全部信息|Copy all details|
|已複製編號|编号已复制|Code copied|
|返回|返回|Back|
|取貨門市|自提门店|Pickup store|
|選擇取貨門市|选择自提门店|Choose pickup store|
|返回訂單|返回订单|Return to order|
|全家|全家|FamilyMart|
|此編號不在門市清單：022305|该编号未收录在门店清单中：022305|Code not in store list: 022305|

## Frame → browser artifact → fresh Figma export

Exports listed below are local review artifacts, not committed binary fixtures. Hashes and exact paths are in frames.json. Superseded DELIVERY PNGs are not used.

|Frame|Browser artifact (CI)|Fresh Figma export|
|---|---|---|
|21:582|parity/21-582.png|output/parity-order-ops/figma/get_screenshot-21-582-1793361278370-0.png|
|21:967|parity/21-967.png|output/parity-order-ops/figma/get_screenshot-21-967-1793361282024-0.png|
|750:3249|parity/750-3249.png|output/parity-order-ops/figma/get_screenshot-750-3249-1793361285876-0.png|
|750:3317|parity/750-3317.png|output/parity-order-ops/figma/get_screenshot-750-3317-1793361288653-0.png|
|750:3385|parity/750-3385.png|output/parity-order-ops/figma/get_screenshot-750-3385-1793361292595-0.png|
|750:3453|parity/750-3453.png|output/parity-order-ops/figma/get_screenshot-750-3453-1793361295794-0.png|
|750:3457|parity/750-3457.png|output/parity-order-ops/figma/get_screenshot-750-3457-1793361298130-0.png|
|750:3503|parity/750-3503.png|output/parity-order-ops/figma/get_screenshot-750-3503-1793361303094-0.png|
|750:3549|parity/750-3549.png|output/parity-order-ops/figma/get_screenshot-750-3549-1793361306615-0.png|
|750:3595|parity/750-3595.png|output/parity-order-ops/figma/get_screenshot-750-3595-1793361308875-0.png|
|6:20488|parity/6-20488.png|output/parity-order-ops/figma/get_screenshot-6-20488-1793361311146-0.png|
|6:20626|parity/6-20626.png|output/parity-order-ops/figma/get_screenshot-6-20626-1793361313216-0.png|
|30:1662|parity/30-1662.png|output/parity-order-ops/figma/get_screenshot-30-1662-1793361315871-0.png|
|30:1731|parity/30-1731.png|output/parity-order-ops/figma/get_screenshot-30-1731-1793361317471-0.png|
|30:1757|parity/30-1757.png|output/parity-order-ops/figma/get_screenshot-30-1757-1793361319257-0.png|
|30:1818|parity/30-1818.png|output/parity-order-ops/figma/get_screenshot-30-1818-1793361320355-0.png|
|95:2858|parity/95-2858.png|output/parity-order-ops/figma/get_screenshot-95-2858-1793361322332-0.png|
|95:2918|parity/95-2918.png|output/parity-order-ops/figma/get_screenshot-95-2918-1793361323289-0.png|
|1431:70774|parity/1431-70774.png|output/parity-order-ops/figma/get_screenshot-1431-70774-1793361324031-0.png|
|1431:70793|parity/1431-70793.png|output/parity-order-ops/figma/get_screenshot-1431-70793-1793361324942-0.png|
|1431:70856|parity/1431-70856.png|output/parity-order-ops/figma/get_screenshot-1431-70856-1793361326034-0.png|
|1431:70881|parity/1431-70881.png|output/parity-order-ops/figma/get_screenshot-1431-70881-1793361327371-0.png|
|1431:70906|parity/1431-70906.png|output/parity-order-ops/figma/get_screenshot-1431-70906-1793361329051-0.png|
|1431:70922|parity/1431-70922.png|output/parity-order-ops/figma/get_screenshot-1431-70922-1793361329968-0.png|
|1431:70938|parity/1431-70938.png|output/parity-order-ops/figma/get_screenshot-1431-70938-1793361331079-0.png|
|1431:70957|parity/1431-70957.png|output/parity-order-ops/figma/get_screenshot-1431-70957-1793361331890-0.png|
|1431:70976|parity/1431-70976.png|output/parity-order-ops/figma/get_screenshot-1431-70976-1793361332945-0.png|
|1431:71001|parity/1431-71001.png|output/parity-order-ops/figma/get_screenshot-1431-71001-1793361333750-0.png|
|1431:71026|parity/1431-71026.png|output/parity-order-ops/figma/get_screenshot-1431-71026-1793361334756-0.png|
|1431:71036|parity/1431-71036.png|output/parity-order-ops/figma/get_screenshot-1431-71036-1793361335915-0.png|
|1431:71280|parity/1431-71280.png|output/parity-order-ops/figma/get_screenshot-1431-71280-1793361336951-0.png|
|1431:71320|parity/1431-71320.png|output/parity-order-ops/figma/get_screenshot-1431-71320-1793361338084-0.png|
|1431:71405|parity/1431-71405.png|output/parity-order-ops/figma/get_screenshot-1431-71405-1793361339390-0.png|
|1431:71427|parity/1431-71427.png|output/parity-order-ops/figma/get_screenshot-1431-71427-1793361340784-0.png|
|1431:71443|parity/1431-71443.png|output/parity-order-ops/figma/get_screenshot-1431-71443-1793361342260-0.png|
|1431:71469|parity/1431-71469.png|output/parity-order-ops/figma/get_screenshot-1431-71469-1793361344310-0.png|
|1431:71485|parity/1431-71485.png|output/parity-order-ops/figma/get_screenshot-1431-71485-1793361345366-0.png|
|1431:71507|parity/1431-71507.png|output/parity-order-ops/figma/get_screenshot-1431-71507-1793361346646-0.png|
|1431:71523|parity/1431-71523.png|output/parity-order-ops/figma/get_screenshot-1431-71523-1793361347697-0.png|
|1431:71549|parity/1431-71549.png|output/parity-order-ops/figma/get_screenshot-1431-71549-1793361349261-0.png|

## Scope and handoff

This RED commit changes only new browser assertions, their existing suite registration, generated browser ownership and card evidence. No application source, API, contract, migration, dependency lockfile, R2 pin or Figma modification. Existing legacy browser assertions remain intact in RED; implementation must map their entry controls to the new views in the same GREEN push. No process remains running; no fixtures/containers/ports were started locally. Local Figma readbacks retained as task evidence.

Current NOT_RUN: CI RED/GREEN, application implementation, browser parity/click ledger outcomes, exhaustive controls, persisted writes, screenshot comparison, independent E4 review, typecheck, provider SANDBOX/LIVE and production deployment. E0 author expectations are not acceptance. No completion claim or automatic merge.
