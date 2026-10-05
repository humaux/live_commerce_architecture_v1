# Unit W4-U1 — PAYUNi 收款开通 UI（Codex）

状态：DRAFT（2026-10-06，待 integrator 冻结）。Base `r3/integration` `bb71f966`。无迁移。
覆盖 IMPLEMENTATION-PLAN W4-U1 的 PAYUNi 部分（Stripe 自助取消 → 无 Stripe 绑定界面）。worktree `.worktrees/w4-u1-payment-activation-ui`。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件 → `w4-02b-payuni-activation.md`、`w4-03b-payuni-refund.md`（冻结版）。

## Integrator 裁决（覆盖正文）
- 密钥输入框：`type=password`、`autocomplete=off`，提交后不回显；页面、BFF、日志、URL、storage 均不得出现 Hash Key/IV（I11）。
- 沙箱验证页写明「使用 PAYUNi 官方測試卡，不會產生真實扣款」；LIVE 启用确认框写明「啟用後買家可用信用卡付款，款項進入你的 PAYUNi 帳戶」。
- 平台开关关闭时整页显示「刷卡收款尚未開放」，不显示启用按钮。

## 范围
1. `PaymentActivation.tsx`（新）步骤：① 凭据（已有接口，SANDBOX/LIVE 各一组）② 沙箱验证（打开 hosted form 新窗口 → 轮询状态 → 「沙箱已驗證」）
   ③ LIVE 探针 → ④ 启用 `payuni_credit`（名称三语、金额范围、排序）。状态徽章取 `InspectMethod.Reasons`（三语映射，未知码显示原码）。
2. `SettingsWizard.tsx`：收款步骤链接到新页（只加入口）。
3. `OrderRefunds.tsx`：PAYUNi 订单的退款按钮可用（读订单 provider），UNKNOWN 文案「不確定是否退款成功，請到 PAYUNi 後台核對，系統不會重送」。
4. `apps/admin/lib/payment-activation-*.ts`、BFF、三语。

## 写入路径
`apps/admin/components/{PaymentActivation.tsx（新）,SettingsWizard.tsx（入口）,OrderRefunds.tsx（provider 文案）}`、`apps/admin/lib/payment-activation-*.ts`、
`apps/admin/app/api/stores/**`（仅新增）、`apps/admin/src/features/settings/routes.ts`（一条）、`tests/admin/payment-activation.spec.ts`。

## 测试 / 门禁
新模式 `bash scripts/dev/test-local.sh --browser-payment-activation`（MOCK 后端 + PROVIDER_MOCK hosted 页）：全流程点击、密钥不在 DOM/网络响应/storage（断言）、
平台开关关闭、凭据轮换后徽章变化、PAYUNi 退款按钮。回归 `--browser-checkout-offline`、`--browser-refund-fulfilment`、`--browser-click-sweep`。
`test-node.sh`、typecheck/build、`check-gates.sh`。

## 证据 / 角色 / 依赖
BROWSER（MOCK）；SANDBOX 浏览器跑法仅 owner 测试商户到位后。Codex；K3；Claude 复核密钥不外泄。依赖 W4-02B、W4-03B 合并。
