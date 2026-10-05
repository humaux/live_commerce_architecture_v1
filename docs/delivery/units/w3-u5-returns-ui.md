# Unit W3-U5 — 退货 / 商家取消 UI（Codex）

状态：DRAFT（2026-10-06，待 integrator 冻结）。Base `r3/integration` `bb71f966`。无迁移。
覆盖 IMPLEMENTATION-PLAN W3-U5。worktree `.worktrees/w3-u5-returns-ui`。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件 → `w3-08b-returns.md` 与 `contracts/returns-v1.md`（冻结版）。

## Integrator 裁决（覆盖正文）
- 退货与退款是两个按钮：退货面板**不**提供「同时退款」；验货完成后显示「如需退款，請使用退款」并链接现有 `OrderRefunds.tsx`。
- 取消已付款订单时，服务器 `refund_first` → 显示「請先完成全額退款，再取消訂單」，不在前端自动发起退款。

## 范围
1. `OrderReturns.tsx`（新）挂在订单详情：登记（选订单行与数量、原因枚举 + 备注）→ 收货（实收数量）→ 验货（每行 可再售 / 不可售 数量，和必须等于实收）
   → 关闭；状态时间线；撤销登记（仅 REGISTERED）。
2. 订单详情「取消訂單」：确认框（原因、说明「會釋放已保留的庫存」），按服务器码显示 `payment_in_flight` / `refund_first` / `already_shipped`。
3. 订单列表筛选「有退貨」（`OrderListFilters.tsx` 加一项）。
4. `apps/admin/lib/returns-*.ts`、BFF、三语。处置按钮在无 `inventory:write` 时禁用（读 session 权限，服务器仍为准）。

## 写入路径
`apps/admin/components/{OrderReturns.tsx（新）,OrderDetailPanel.tsx（挂载与取消按钮）,OrderListFilters.tsx（一项）}`、`apps/admin/lib/returns-*.ts`、
`apps/admin/app/api/stores/**`（仅新增）、`tests/admin/returns.spec.ts`。

## 测试 / 门禁
新模式 `bash scripts/dev/test-local.sh --browser-returns`（integrator 加）：完整 RMA 流程真实点击、数量校验、权限禁用、取消三种拒绝码、取消成功后状态。
回归 `--browser-refund-fulfilment`、`--browser-merchant-orders-ui`、`--browser-click-sweep`。`test-node.sh`、typecheck/build、`check-gates.sh`。

## 证据 / 角色 / 依赖
BROWSER（MOCK）。Codex；K3。依赖 W3-08B 合并。可与 W3-U4 并行（文件不重叠：本单元不碰 `MerchantOrders.tsx`）。
