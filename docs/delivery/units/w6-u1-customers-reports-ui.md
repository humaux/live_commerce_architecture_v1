# Unit W6-U1 — 客户标签/备注 UI 与报表页（Codex）

状态：DRAFT（2026-10-06，待 integrator 冻结）。Base `r3/integration` `bb71f966`。无迁移。
覆盖 IMPLEMENTATION-PLAN W6-U1 的 01、02 部分（利润 03 延后）。worktree `.worktrees/w6-u1-customers-reports-ui`。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件 → `w6-01b-customer-tags-notes.md`、`w6-02b-reports.md`（冻结版）。

## Integrator 裁决（覆盖正文）
- 备注输入框说明「備註會在買家要求刪除資料時一起刪除，買家申請匯出資料時也看得到」（依 W6-01B OPEN-1 最终裁决调整文案）。
- 报表数字旁注明口径：「實收 = 線上付款成功；貨到付款/轉帳另列」；不画「利潤」。图表只用 CSS/SVG，不引新依赖。

## 范围
1. `Customers.tsx`：列表标签徽章 + 标签筛选下拉；`CustomerDetail.tsx`：标签编辑（多选 + 新建）、备注列表（新增/编辑/删除，作者与时间）。
   标签管理（改名/改色/删除）在客户页的「標籤管理」对话框。
2. `Reports.tsx`（新）：日期区间（≤ 92 天）+ 四个分页：商品表（可排序）、渠道条形（SVG）、漏斗（四级条 + 转化率）、手工单表；每页「匯出 CSV」。
3. `apps/admin/lib/{customer-tags-*,reports-*}.ts`、BFF、`apps/admin/src/features/customers/routes.ts` 与 `finance` 或新 `reports` 路由一条、三语。

## 写入路径
`apps/admin/components/{Customers.tsx,CustomerDetail.tsx,Reports.tsx（新）,CustomerTags.tsx（新）}`、`apps/admin/lib/customer-tags-*.ts`、`apps/admin/lib/reports-*.ts`、
`apps/admin/app/api/stores/**`（仅新增）、`apps/admin/src/features/{customers,finance}/routes.ts`（各一条）、`tests/admin/{customer-tags,reports}.spec.ts`。

## 测试 / 门禁
扩展 `bash scripts/dev/test-local.sh --browser-customers-billing`（标签/备注真实点击、筛选、权限 403 文案）；新模式 `--browser-reports`（integrator 加；四分页数据渲染、区间校验、导出下载）。
回归 `--browser-admin-shell`、`--browser-click-sweep`、`--browser-visual-lint`。`test-node.sh`、typecheck/build、`check-gates.sh`。

## 证据 / 角色 / 依赖
BROWSER（MOCK）。Codex；K3。依赖 W6-01B、W6-02B 合并。W5-U1 在本单元后（同改 `CustomerDetail.tsx`）。
