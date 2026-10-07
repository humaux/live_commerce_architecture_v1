# Unit W3-U4 — 合并出货 UI（Codex）

状态：DRAFT（2026-10-06，待 integrator 冻结）。Base `r3/integration` `bb71f966`。无迁移。
覆盖 IMPLEMENTATION-PLAN W3-U4。worktree `.worktrees/w3-u4-parcel-merge-ui`。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件 → `w3-07b-parcel-merge.md`（冻结版 API）。

## Integrator 裁决（覆盖正文）
- 文案必须写明「只合併包裹，不合併付款或金額；每張訂單仍各自計價、各自收到出貨通知」。COD 与超商订单不显示合并入口。
- `MerchantOrders.tsx` 串行：在 W3-U1（回填）与 W3-U1b（拣货）之后。
- 修订（Opus 审查后，finisher）：reload 后 OPEN 组不得搁浅 → 新增只读 `GET …/parcel-groups`（迁移 0164，见 `w3-07b-parcel-merge.md`），订单页每次加载据此重建出货/解除面板；
  提示条只显示掩码收件人（与列表行同规则）；合并错误在无建议时仍可见；不确定的解除合并在 `group_not_open` 后对账（重取 OPEN 组）。

## 范围
1. 订单列表顶部提示条：`merge-suggestions` 非空时显示「N 組訂單可合併出貨」→ 展开分组卡片（买家显示名、单号、件数）→「合併」。
2. 合包组徽章（订单行显示「合包 #短码」）；组详情抽屉：成员、「解除合併」（仅 OPEN）、「填寫運單」（复用 `OrderShipment.tsx` 表单，提交到组出货路由）。
3. 单笔出货在组内订单上禁用并提示「此訂單在合包中，請在合包填寫運單」（服务器 `in_parcel_group` 码映射同文案）。
4. `apps/admin/lib/parcels-*.ts`、BFF 代理、三语。

## 写入路径
`apps/admin/components/{MerchantOrders.tsx（提示条/徽章）,ParcelGroup.tsx（新）}`、`apps/admin/components/OrderShipment.tsx`（只抽出可复用表单 props，不改行为）、
`apps/admin/lib/parcels-*.ts`、`apps/admin/app/api/stores/**`（仅新增）、`tests/admin/parcel-merge.spec.ts`。

## 测试 / 门禁
扩展 `bash scripts/dev/test-local.sh --browser-merchant-orders-ui`：建议 → 合并 → 组出货 → 三单「已出貨」；解除合并；组内单笔出货禁用；409 码文案。
回归 `--browser-home-cod`、`--browser-click-sweep`。`test-node.sh`、typecheck/build、`check-gates.sh`。

## 证据 / 角色 / 依赖
BROWSER（MOCK）。Codex；K3。依赖 W3-07B 合并、W3-U1b 合并。
