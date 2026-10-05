# Unit W5-U1 — 搬家导入向导 UI（Codex）

状态：DRAFT（2026-10-06，待 integrator 冻结）。Base `r3/integration` `bb71f966`。无迁移。
覆盖 IMPLEMENTATION-PLAN W5-U1 的顾客/订单部分（W5-04B 发送总开关不在本批）。worktree `.worktrees/w5-u1-import-wizard-ui`。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件 → `w5-02b-customer-import.md`、`w5-03b-order-history-import.md`、`contracts/migration-import-v1.md`（冻结版）。

## Integrator 裁决（覆盖正文）
- 向导明确写「行銷同意不會匯入，匯入的顧客一律視為未同意」「歷史訂單只供查看，不計入營收與報表」。
- 文件只在内存中读取并原样上传；**不把 CSV 内容写入 localStorage/sessionStorage/IndexedDB**，不在前端日志打印行内容。
- 映射保存 = 随请求发送的 `mapping`（服务器批次记录）；前端只可在 `localStorage` 记最近一次映射（列名对应，不含数据，try/catch）。

## 范围
1. `ImportWizard.tsx`（新）：步骤 选类型（顾客 / 历史订单；订单步骤在顾客导入完成前禁用并提示）→ 上传（UTF-8 提示，Big5 错误码给「另存為 CSV UTF-8」说明）
   → 列映射（自动猜 + 下拉改）→ 预览表（失败优先，`consent_ignored` 徽章）→ 确认提交（「將新增 N、更新 M」）→ 结果 + 下载失败行。
2. `CustomerDetail.tsx`：「歷史訂單（SHOPLINE）」分区（分页 50）；客户列表「匯入」徽章。
3. 入口：设置页或客户页「從 SHOPLINE 搬家」（一条路由）；`apps/admin/lib/import-*.ts`、BFF（`text/csv` 透传，大小 2 MiB 前端先挡）、三语。

## 写入路径
`apps/admin/components/{ImportWizard.tsx（新）,CustomerDetail.tsx（历史订单分区）,Customers.tsx（徽章）}`、`apps/admin/lib/import-*.ts`、`apps/admin/app/api/stores/**`（仅新增）、
`apps/admin/src/features/customers/routes.ts`（一条）、`tests/admin/import-wizard.spec.ts`（合成 CSV fixture）。

## 测试 / 门禁
新模式 `bash scripts/dev/test-local.sh --browser-migration-import`（integrator 加；MOCK 后端）：两类导入全流程真实点击、映射修改、失败下载、重放提示、storage 中无 CSV 内容断言。
回归 `--browser-customers-billing`、`--browser-click-sweep`。`test-node.sh`、typecheck/build、`check-gates.sh`。

## 证据 / 角色 / 依赖
BROWSER（MOCK，合成数据）。Codex；K3。依赖 W5-02B、W5-03B 合并；W6-U1 合并（同改 `CustomerDetail.tsx`/`Customers.tsx`）。
