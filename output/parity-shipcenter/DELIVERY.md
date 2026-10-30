# PARITY-SHIPCENTER · implementation handoff · 2026-10-30

Status: E1 static structure/typecheck only. Browser GREEN, physical print, independent parity and production acceptance are PENDING/NOT_RUN. PR #353 targets `r3/integration`; branch `unit/parity-shipcenter`; worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-shipcenter`. Dispatched child role: UI implementer; precise runtime model/effort UNKNOWN. No recursive delegation. Initial head `e23c1167780288321bb9d9775c2be93a5441bf4a`; merged integration `3ea359f79` before adding the route per ownership rule.

Humaux memory/coord/lock tools are unavailable. Claim/store/canvas: NOT_RUN; this local evidence is not a substitute. Integrator must relay it to the parent canvas. Platform memory guidance was read under the runtime's higher-priority instruction; no memory was written and no old result is used as current project evidence.

## RELAY 2 — language selector clipping, targeted fix 1/2

Current PR head `755232406e2e3a9789a9234018ec37a09e391a51` failed Gates run `41054087713`, job `124059044047` (`--browser-visual-lint@1/4`), on tested synthetic merge `0f71326d0880a5f0852d35a1f96291a621925fc5`. Artifact `12978519153`, `run.REibB4Nq/ui-visual-audit/20261030T130336Z/lint.md`, identifies the shipping shell's language select on `/orders/shipping`, zh-CN 1586×992: R7 `select-value-clipped`, value `简中` width 28px, available 26px, control 64px. All 85/85 enumerated shots loaded; one blocking instance, plus one R11 narrow-control warning. Foundation run `41054087554` / job `124057097040` passed on this old head; it does not accept the repair.

Root cause: fixed 64px select width leaves insufficient space after native-arrow and padding allowances. Fix: use 96px (the existing G-UI9 desktop usable minimum) and prevent flex shrinking. Content, locales, test assertions, thresholds and exemptions are unchanged. Existing G-UI9 is the causal regression gate; no local browser/tests/server ran. Repair acceptance remains PENDING the next push's own CI; independent parity/physical print/production remain NOT_RUN. No new source import, route, page or spec change, so no harness/ownership regeneration is required for this CSS-only repair.

## Requirement and development authority

Owner dispatch card 2026-10-30 is the requirements/authorization source. `AGENTS.md` and `架构.md` §0/1/2/26/27 impose evidence separation, existing Go/PG transaction authority, scoped worktrees and independent acceptance. `contracts/merchant-orders-v2.md`, current `orders-client.ts`, `orders-model.ts`, `orders-v2.ts`, `auth.ts` and shell route contracts control DTOs, server store membership, read failures, pagination and permissions. No new API, contract, lockfile, migration, backend or order-list/detail edits. Local checks were limited to allowed static checks and the mandated ownership generator.

Design authority: Figma `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`, and ux-orders DELIVERY current polish 2026-10-29 / `polish-short-number/exports.json`. Fleet Bridge status returned a connected document; read-only contexts/screenshots for 1429:28034, 28313, 28353, 28407, 28471 and 29491 are retained in `design/`. No Figma writes. Visible flags were checked: mobile has one three-line order link and a chevron; hidden desktop controls/help are not copied into it. Result return buttons are stacked secondary controls. Owner shell option A is scoped to shipping, with authenticated permission-filtered navigation.

## UI PROCESS — BOURDIEU

| Question | Verdict |
|---|---|
| Q1 task novice | One heading, one next action and a concise printing boundary. First-time comprehension UNKNOWN until independent test. |
| Q2 task expert | Batch selection and refresh-safe URL context; completion speed NOT_RUN. |
| Q3 cost/payment/delivery | Printing buys no label, changes no shipment or payment, and proves no carrier handover. |
| Q4 recovery | Real-click tests cover cancel/edit/back, refresh, failed read Retry, and both print-return paths. Results PENDING CI. |
| Q5 mobile task | Signed mobile exposes one order link per row; batch printing remains a desktop task. |
| Q6 aesthetics | UNKNOWN until owner/independent comparison of real screenshots with the signed frames. |

Novice/expert means recent experience with this task, never class/device.

## UI PROCESS — PROJECT UNDERSTANDING

| Source read | Screen constraint |
|---|---|
| 架构.md §0/1/2 | Distinguish static/mock/live evidence; preserve existing server data authority and production red lines. |
| 架构.md §26/27, AGENTS.md | Worktree isolation, single card, real clicks and independent acceptance. |
| merchant-orders-v2 contract | Read existing scoped DTOs; opaque server cursor; no recipient data in URLs/storage. |
| orders-model/client and server matchesState | Legacy unshipped filter excludes review-required orders; retain those via active read pages and preserve visible review state. |
| Current ux-orders DELIVERY / live Fleet read | Short display numbers, full request identity, all selection states, native print boundary and mobile single-order task. |
| Owner 2026-10-30 card / shell decision | Signed content is production UI, light sidebar, green primary, all three locales. |

## UI PROCESS — MOBBIN → EAGLE

No redesign/import. The signed design's existing structural references are Squarespace list `MVP0WJO2CM2IC` (separate controls/aligned facts) and Shopify iOS `MVP0WJO8UYAVE` (same task with concise mobile context), module Eagle folder `MVQL9YR448VTK`. Source is the current ux-orders DELIVERY; historical `simplify2/eagle-orders.json` is attribution only, not a fresh API verification. No extra Mobbin query needed; no images, brands or product facts copied from references.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

Read the module's `output/ux-audit-ref/FIGMA-GUIDE.md`: G1 Auto layout (fixed actions outside scroll region), G2 Variables (reuse semantic theme colors and separate locale copy from order facts), G3 Components/variants (one row/state/control family across combinations), G4 Prototyping (real navigation and return paths). This is implementation against signed frames; no new design/prototype or Figma copy-lint claim. Official source links remain in that guide; no fresh online documentation acceptance is claimed.

## Required facts across states

| Required field/fact | Desktop center | Review | Preview/print | Result | Mobile |
|---|---|---|---|---|---|
| Order # | Short display; full request ID | Selected only | One slip per ID | Same selected IDs | Same display/link identity |
| Product/quantity | Existing detail items | Same items | Same items, including print | Same items | Same list facts |
| Ready/review-needed | Existing server states | Printing never authorizes shipping | No shipment mutation | Original state on return | Same server status |
| Store | Authenticated server name | Same scope | Every slip | Same scope | Signed demo label only for server test-mode data; otherwise server store name |
| Recipient | No fabricated list identity | Not a signed review field | Existing destination recipient on every slip | No invented handover | Existing order-detail destination |
| Delivery | No invented fact | No invented fact | Existing home/pickup kind; localized method | No shipment claim | Existing detail link |
| Address/phone | Never URL/storage | Memory only | Signed source note on screen; actual existing destination printed | No state write | Existing detail route |
| Label/print boundary | Exact note | Exact boundary | Exact boundary | Exact cancellation/outcome note | Exact desktop-only batch note |

The signed source does not define a mobile batch-review/print state. No recipient/payment fields are fabricated to fill that gap. Click ledger is emitted by `tests/admin/shipping-center.spec.ts` for page/control/operation/expected/actual/result; CI has not yet exercised the implementation.

## Parity sheet

All 30 current mapped states (29 desktop + one mobile; current export inventory governs the card's approximate count). REAL-route screenshots use 1440×900 or 390×844 and will be saved under the existing browser artifact. They do not exist for this implementation until CI runs.

| Signed frame | REAL-route CI screenshot | Current DELIVERY PNG |
|---|---|---|
| 1429:28034 | `parity/1429-28034.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28034-1793204023173-0.png` |
| 1429:28127 | `parity/1429-28127.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28127-1793204023308-0.png` |
| 1429:28220 | `parity/1429-28220.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28220-1793204023438-0.png` |
| 1429:28313 | `parity/1429-28313.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28313-1793204023512-0.png` |
| 1429:28353 | `parity/1429-28353.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28353-1793204023640-0.png` |
| 1429:28407 | `parity/1429-28407.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28407-1793204023770-0.png` |
| 1429:28471 | `parity/1429-28471.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28471-1793204023902-0.png` |
| 1429:28518 | `parity/1429-28518.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28518-1793204024028-0.png` |
| 1429:28575 | `parity/1429-28575.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28575-1793204024159-0.png` |
| 1429:28652 | `parity/1429-28652.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28652-1793204024284-0.png` |
| 1429:28700 | `parity/1429-28700.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28700-1793204024409-0.png` |
| 1429:28793 | `parity/1429-28793.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28793-1793204024534-0.png` |
| 1429:28847 | `parity/1429-28847.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28847-1793204024662-0.png` |
| 1429:28911 | `parity/1429-28911.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28911-1793204024789-0.png` |
| 1429:28958 | `parity/1429-28958.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-28958-1793204024912-0.png` |
| 1429:29051 | `parity/1429-29051.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29051-1793204025037-0.png` |
| 1429:29105 | `parity/1429-29105.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29105-1793204025160-0.png` |
| 1429:29169 | `parity/1429-29169.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29169-1793204025285-0.png` |
| 1429:29216 | `parity/1429-29216.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29216-1793204025421-0.png` |
| 1429:29309 | `parity/1429-29309.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29309-1793204025544-0.png` |
| 1429:29366 | `parity/1429-29366.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29366-1793204025674-0.png` |
| 1429:29443 | `parity/1429-29443.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29443-1793204025796-0.png` |
| 1429:29491 | `parity/1429-29491.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29491-1793204025925-0.png` |
| 1429:29584 | `parity/1429-29584.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29584-1793204026052-0.png` |
| 1429:29641 | `parity/1429-29641.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29641-1793204026181-0.png` |
| 1429:29718 | `parity/1429-29718.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29718-1793204026307-0.png` |
| 1429:29766 | `parity/1429-29766.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29766-1793204026430-0.png` |
| 1429:29859 | `parity/1429-29859.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29859-1793204026556-0.png` |
| 1429:29919 | `parity/1429-29919.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-29919-1793204026688-0.png` |
| 1429:30009 | `parity/1429-30009.png` | `output/ux-orders/polish-short-number/shots/get_screenshot-1429-30009-1793204026817-0.png` |

## Three-locale visible copy

Runtime order/store/recipient/item facts remain server values and are not translated or replaced with sample facts. Dynamic count uses `{n}`. Shared navigation and its copy remain the existing `src/shell-copy.ts`; the new route title is included below. Proper product name comes from existing company authority. The locale selector displays the per-locale short label below.

| zh-TW | zh-CN | en |
|---|---|---|
| 出貨中心 | 发货中心 | Shipping center |
| 返回訂單 | 返回订单 | Back to orders |
| 列印不購面單、不登記出貨。 | 打印不购买面单，也不登记发货。 | Printing does not buy a label or record shipment. |
| 核對選取訂單 | 核对所选订单 | Review selected orders |
| 返回選取 | 返回选择订单 | Back to selection |
| 修改選取 | 修改所选订单 | Edit selection |
| 確認並預覽 | 确认并预览 | Confirm and preview |
| 列印預覽 | 打印预览 | Print preview |
| 列印 | 打印 | Print |
| 完成列印 | 完成打印 | Complete printing |
| 檢視 | 查看 | View |
| 查看 | 查看 | View |
| 取消 | 取消 | Cancel |
| 返回列印預覽 | 返回打印预览 | Back to print preview |
| 返回出貨中心 | 返回发货中心 | Back to shipping center |
| 收件人 | 收件人 | Recipient |
| 配送方式 | 配送方式 | Delivery method |
| 商家自行安排配送 | 商家自行安排配送 | Seller-arranged delivery |
| 訂單 | 订单 | Order |
| 尚未選取訂單 | 尚未选择订单 | No orders selected |
| 列印不證明交寄、不登記出貨、不購面單。 | 打印不代表物流已揽收，不登记发货，也不购买面单。 | Printing does not prove carrier handover, record shipment or buy a label. |
| 取消列印不改出貨狀態；列印不證明交寄。 | 取消打印不改变发货状态；打印不代表物流已揽收。 | Canceling print keeps shipment status; printing does not prove handover. |
| 批次列印請用桌面。 | 批量打印请使用电脑。 | Use desktop for batch printing. |
| 待出貨 | 待发货 | Ready to ship |
| 需覆核 | 需复核 | Review needed |
| 已選 {n} 筆 | 已选 {n} 笔 | {n} selected |
| 地址與電話取自訂單 | 地址和电话取自订单 | Address and phone come from the order |
| 配送目的地 | 配送目的地 | Destination |
| 電話 | 电话 | Phone |
| 7-ELEVEN 取貨 | 7-ELEVEN 取货 | 7-ELEVEN pickup |
| 全家取貨 | 全家取货 | FamilyMart pickup |
| 萊爾富取貨 | 莱尔富取货 | Hi-Life pickup |
| OK 超商取貨 | OK 便利店取货 | OK Mart pickup |
| 正在載入訂單… | 正在加载订单… | Loading orders… |
| 暫時無法載入訂單，請重試。 | 暂时无法加载订单，请重试。 | Orders could not be loaded. Try again. |
| 重試 | 重试 | Retry |
| 登入已到期，請重新登入。 | 登录已过期，请重新登录。 | Your session has expired. Sign in again. |
| 登入 | 登录 | Sign in |
| 你沒有查看這些訂單的權限。 | 你没有查看这些订单的权限。 | You do not have access to these orders. |
| 目前沒有待處理訂單。 | 目前没有待处理订单。 | No orders to prepare. |
| 部分選取訂單已無法查看，請重新核對選取。 | 部分所选订单已无法查看，请重新核对所选订单。 | Some selected orders are no longer available. Review your selection. |
| 返回第一頁 | 返回第一页 | Back to first page |
| 下一頁 | 下一页 | Next page |
| 無法開啟列印，請重試。 | 无法打开打印窗口，请重试。 | Printing could not open. Try again. |
| 示範店 | 示例店铺 | Demo store |
| 語言 | 语言 | Language |
| 繁中 | 简中 | EN |


### Existing shell and expanded navigation copy

Source: `apps/admin/src/shell-copy.ts`; authenticated role decides which existing links appear.

| zh-TW | zh-CN | en |
|---|---|---|
| 廣告 | 广告 | Ads |
| 廣告歸因 | 广告归因 | Ad attribution |
| 帳單 | 账单 | Billing |
| 信用卡收款 | 信用卡收款 | Card payments |
| 商品與庫存 | 商品与库存 | Products & inventory |
| 留言認領 | 留言认领 | Comment claims |
| 分類 | 分类 | Collections |
| 顧客詳情 | 顾客详情 | Customer details |
| 從 SHOPLINE 搬家 | 从 SHOPLINE 搬家 | Move from SHOPLINE |
| 顧客 | 顾客 | Customers |
| 顧客 | 顾客 | Customers |
| 列印寄件標籤 | 打印寄件标签 | Print shipping label |
| 總覽 | 总览 | Overview |
| 商店設計 | 店铺装修 | Store design |
| 收款與報表 | 收款与报表 | Payments & reports |
| 財務 | 财务 | Finance |
| 庫存 | 库存 | Inventory |
| 接受邀請 | 接受邀请 | Accept invitation |
| 直播 | 直播 | Live |
| 直播控台 | 直播控台 | Live console |
| 行銷優惠 | 营销优惠 | Marketing |
| 訊息中心 | 消息中心 | Inbox |
| 工作區導覽 | 工作区导航 | Workspace navigation |
| 訂單 | 订单 | Orders |
| 建立訂單 | 创建订单 | Create order |
| 訂單與出貨 | 订单与发货 | Orders & shipping |
| 總覽 | 总览 | Overview |
| 編輯商品 | 编辑商品 | Edit product |
| 匯入商品 | 导入商品 | Import products |
| 商品 | 商品 | Products |
| 優惠碼 | 优惠码 | Discount codes |
| 銷售報表 | 销售报表 | Sales reports |
| 重設密碼 | 重设密码 | Reset password |
| 退貨 | 退货 | Returns |
| 設定 | 设置 | Settings |
| 商店設定 | 店铺设置 | Store settings |
| 結算單 | 结算单 | Settlements |
| 出貨中心 | 发货中心 | Shipping center |
| 建立帳號 | 创建账号 | Create account |
| 網路商店 | 网店 | Online store |
| 直播場次 | 直播场次 | Live sessions |
| 員工 | 员工 | Team |

## Commands, evidence and limits

- Causal CI RED: `41049898125` / job `124044734532`, artifact `12974094813`, head `e23c1167`, tested merge `bea89583`; 15 new missing-heading assertion failures, 21 existing passed. Foundation run `41049898111` / job `124043064745` passed at RED head only. See red.log and red-evidence/assertions.log.
- `node --check` on shipping-center spec, new shipping i18n, shell-copy and order route metadata: exit 0.
- `node .../typescript/bin/tsc --project apps/admin/tsconfig.json --noEmit --incremental false`: exit 0 after resolving existing dependency links within this worktree (no install, main checkout read-only). Initial missing workspace-package links were environment setup, not code acceptance.
- `git diff --check`: exit 0. Ownership generator and `--check`: exit 0 after merging integration; generated content unchanged from the RED commit. No Go dependency edges changed, so depmap was not regenerated or hand-edited.
- Direct Node --check cannot parse TSX (ERR_UNKNOWN_FILE_EXTENSION); TypeScript strict above covers all changed TSX instead. No runtime test was substituted.
- Local tests, local servers, local browser, OS modal/physical print, independent review, production/deploy: NOT_RUN. Existing spec intercepts only synthetic read DTOs and native OS modal; it also includes an un-intercepted signed BFF-read case.
- Source fixture/threshold/baseline/gate assertions were not weakened. Two integration-merge files are imported unchanged, not card edits. Scope additions are listed in PR-BODY.md.
- Rollback: revert the implementation commit and, separately if necessary, its tests-only commit; no business data has changed. No local process/container/port was started. Own temporary dependency links and downloaded artifact archive are removed after static checks; small failed-assertion evidence/design reads remain in this card directory.

## Handoff

GREEN PENDING. After the implementation push, record current Gates/Foundation run IDs in the PR and final relay; integrator wakes a fresh session when the job concludes. Do not poll longer than ~10 minutes. Do not merge, enable auto-merge, deploy or push evidence-only after GREEN.

## E1 checked source hashes

TypeScript strict and ownership static checks refer to these unchanged source bytes (product tests remain CI-only).

| Source | SHA256 |
|---|---|
| `apps/admin/app/[locale]/orders/shipping/page.tsx` | `bd2444b4632761c4927bb08428fea72bd3075caf0bef85a690718e312c155604` |
| `apps/admin/components/shipping/ShippingCenter.tsx` | `c19b1cc90e1addddf3e4a45e5b8a383a672315d27c93f7eed919a13cb469752e` |
| `apps/admin/components/shipping/ShippingShell.tsx` | `5a7746a875d935869e70d14e3aa6e650abf13ec2ce8c3d6799841d969d8489b2` |
| `apps/admin/components/shipping/shipping.module.css` | `61f85f532eee688db504bbba9ba0d3f78f7a78019f7e6029b4ad2f52e92b2d9c` |
| `apps/admin/src/features/orders/routes.ts` | `3255fa3414f64d716b7dc685a9c976c34f99ea8056c2b398b28399ae91af7b41` |
| `apps/admin/src/shell-copy.ts` | `ee4b6a030fa3497ca8d893044ae937bf9999d0736b193e634ebf945ba241536b` |
| `packages/i18n/src/index.ts` | `daefac9b5b9fcff6818acf471f1012ceca90f1c35c0223229f10d55db30bb6f8` |
| `packages/i18n/src/shipping.ts` | `f501973a210e1ecafacec3155c10a3809e3c5c3eaf359b4127252f45fa915c3f` |
