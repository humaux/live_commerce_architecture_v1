# Unit W3-U1b — 订单批量操作 UI：拣货单 / 承运商导出 / 超商寄件单（Codex）

状态：DRAFT（2026-10-06，待 integrator 冻结）。Base `r3/integration` `bb71f966`。无迁移。
覆盖 IMPLEMENTATION-PLAN W3-U1 的拣货/导出/超商部分（运单回填 UI 已由 `unit/w3-01b-ui` 在途交付，本单元不碰 `TrackingImport.tsx`）。
worktree `.worktrees/w3-u1b-picklist-ui`（branch `unit/w3-u1b-picklist-ui`）。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件 → `docs/delivery/units/w3-02b-picklist-export.md`（API 形状，以冻结版为准）。

## Integrator 裁决（覆盖正文）
- 界面目标 = v5 示意页「直播後」段（owner 授权 v5 为 W2/W3 UI 目标）。
- `MerchantOrders.tsx` 与 `unit/w3-01b-ui` 共用 → **必须在 W3-U1（回填 UI）合并后开工**；之后的 W3-U4 也改此文件，串行。
- 打印只用浏览器打印 CSS，不生成 PDF、不引新依赖。

## 目标 / owner 流程
订单列表勾选（或「本場全部」）→ 工具列「撿貨單」打开打印预览页（SKU 汇总表在前，逐单明细在后）→ Ctrl/⌘+P；
「匯出」选模板下载 CSV；「超商寄件單」对勾选的超商单排队，结果表逐单显示 已排隊 / 已存在 / 失敗原因。

## 范围
1. 列表勾选列（全选当前页、跨页保留选择 ≤ 500，超出禁用并提示）。
2. `PickList.tsx`（新）：调用 `POST …/orders/pick-list`，打印样式 `@media print`（隐藏导航，A4 纵向，SKU 表不跨页断行）。
3. 导出对话框：模板单选 → 下载（`Content-Disposition` 文件名由服务器给）。
4. 超商批量：确认框写明「將建立 N 張超商寄件單」；`Idempotency-Key` 每次确认一个；`UNKNOWN` 行显示「不確定是否建立，請稍後重新整理，系統不會重送」。
5. BFF：`apps/admin/app/api/stores/**` 按现有模式加三条代理；`apps/admin/lib/picklist-*.ts`（请求/模型/精确键校验）；三语 zh-TW/zh-CN/en。

## Non-goals
回填 UI、合包 UI（W3-U4）、PDF、批量打印 ECPay 标签。

## 写入路径
`apps/admin/components/{MerchantOrders.tsx（勾选列与工具列）,PickList.tsx（新）,CarrierExport.tsx（新）,CvsBatch.tsx（新）}`、
`apps/admin/lib/picklist-*.ts`、`apps/admin/app/api/stores/**`（仅新增三条）、`tests/admin/picklist.spec.ts`、`tests/admin/picklist-model.test.ts`。

## 测试 / 门禁
- 模型单测：精确键、500 上限、模板枚举。
- 浏览器（MOCK 后端）：新模式 `bash scripts/dev/test-local.sh --browser-picklist`（integrator 加）：真实点击勾选 → 拣货单 DOM（totals 行数、`@media print`
  下导航隐藏用 `page.emulateMedia({media:'print'})` 断言）→ 导出下载文件头 → 超商批量结果表；点击台账；三语。
- 回归：`--browser-merchant-orders-ui`、`--browser-cvs`、`--browser-click-sweep`、`--browser-visual-lint`。
- `bash scripts/dev/test-node.sh`、admin typecheck/build、`bash scripts/dev/check-gates.sh`。

## 证据 / 角色 / 依赖
BROWSER（MOCK 后端）。Codex；K3 独立 spec。依赖：W3-02B API 冻结并合并；W3-U1（`unit/w3-01b-ui`）合并。
