# Unit W4-02B — PAYUNi 信用卡商家自助开通（验证状态机，后端，钱路）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0137**。覆盖 IMPLEMENTATION-PLAN W4-01B 的开通部分（M14-01；M02 #11；SUMMARY P0-3；偏差 A7/B23）。
**ID 说明**：计划里的 W4-02B（Stripe 商家自助绑定）按 owner 裁决取消，本 ID 改给 PAYUNi 开通；计划 W4-03B（日对账）延后，见 INDEX。
worktree `.worktrees/w4-02b-payuni-activation`（branch `unit/w4-02b-payuni-activation`）。UI 在 W4-U1。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/payment-methods-v1.md`（全文）、`contracts/merchant-accounts-v1.md` 凭据节、
`contracts/payment-start-v1.md`（qualification 使用处）、`contracts/payuni-wire-v1.md`、`w4-01b-payuni-notify.md`。

## Integrator 裁决（覆盖正文）
- 台湾刷卡 = **PAYUNi 商家自助**（owner 暂定）。Stripe 只给 owner 香港主体店、运维登记、保留逐店审批（D3）——**本批不做 Stripe 自助**。
- v1 只开 `payuni_credit`（`account_qualifications.code CHECK(code='payuni_credit')` 已如此）；分期/ATM/CVS 代码/LINE Pay 保持 disabled draft。
- **启用必须和退款（W4-03B）同批上线**（偏差 B23）：平台开关 `LC_PAYUNI_ENABLED` 默认 0，W4-02B 与 W4-03B 都合并且 SANDBOX 门禁过后才由 owner 打开。
- 手工单（LC-B6 代建单）可用 PAYUNi 刷卡（owner 裁决 G3）：不另开单元，代建单的卡支付可用性读同一 enabled 方法。
- 代理/实现者**永不**触发 LIVE 交易；LIVE 路径代码存在但证据 = NOT_RUN。

## 目标 / owner 流程
商家在「收款設定」填 PAYUNi 商店代号、Hash Key、Hash IV（已有凭据接口）→ 点「測試連線」（SANDBOX）→ 系统开一笔 NT$1 沙箱交易，
商家用 PAYUNi 公开测试卡付款 → 查询 + 通知都证明成功 → 状态「沙箱已驗證」→ 商家切 LIVE 凭据 → 「正式啟用」通过 LIVE 凭据探针 → 买家结账出现刷卡。

## 关键事实（bb71f966）
- `payments.SetMethod` 对 `Enabled=true` 直接 `ErrConflict`（`internal/payments/methods.go:83`）；SQL 约束 `method_not_admitted CHECK(NOT enabled)`（0015:22）。
- `payments.account_qualifications`（0016:4）：`proof_class IN ('PROVIDER_MOCK','REAL_SANDBOX','REAL_LIVE')`、`expires_at`、`revoked_at`；
  目前**只有 Stripe 注册路径插入**（0061:1552）——PAYUNi 无签发流程。
- 凭据 Create/Get/Rotate（AES-256-GCM，AAD 绑定 scope/版本）已存在于 `internal/integrations/accounts`。

## 范围
1. 状态机（每 `connection × environment × code`）：`CONFIGURED_UNVERIFIED → VERIFYING → VERIFIED_SANDBOX`（SANDBOX）/`VERIFIED_LIVE_PROBE`（LIVE）
   → 方法可 `enabled=true`。凭据轮换 → 资格 `revoked_at` → 方法自动不可用（`Available=false`，`CREDENTIAL_ROTATED`）。
2. SANDBOX 验证 `POST …/payments/payuni/verify`（`integration:manage`，`Idempotency-Key`）：建一笔 NT$1 验证尝试（不关联订单、不碰库存，`purpose='verification'`），
   返回 hosted form；签发 `REAL_SANDBOX` 资格当且仅当：查询得到 payment-capture-v1 定义的完整 CAPTURED **且** 收到 W4-01B 验签通知回执；有效 180 天。
   MOCK 环境签 `PROVIDER_MOCK`（只在 `LC_PAYUNI_PROFILE=mock` 时可用，生产构建拒绝）。
3. LIVE 探针 `POST …/payments/payuni/live-probe`：用 LIVE 凭据对一个不存在的 MerTradeNo 发查询，**验签通过的「查无交易」**即证明 MerID/Key/IV 正确 →
   签 `REAL_LIVE` 资格（`evidence_ref='live-probe:<receipt>'`）。**不发起任何交易。**（见 OPEN-1）
4. `SetMethod(Enabled=true)`：定义者内校验存在未撤销、未过期、同 connection+environment+code 的资格，且 `LC_PAYUNI_ENABLED`（Go 侧）为真；否则 `409 not_qualified`。
5. `InspectMethod` 的 `Reasons` 增加 `NOT_QUALIFIED`、`QUALIFICATION_EXPIRED`、`PLATFORM_DISABLED`。

## Non-goals
Stripe 自助（取消）、日对账（延后）、非信用卡方法、分期期数、商户进件代办、自动续期资格。

## 合同修改
`contracts/payment-methods-v1.md` 新增「Amendment W4-02B activation」：删除「SQL rejects enabled=true」，改为资格门槛 + 平台开关；
`contracts/payuni-wire-v1.md` 加 LIVE 探针（查询不存在交易）为允许调用；`payment-start-v1.md` 记录 `purpose='verification'` 尝试不建订单。

## SQL / 迁移 `0137_payuni_activation.sql`
- `ALTER TABLE payments.method_versions DROP CONSTRAINT method_not_admitted`；enabled 的写入只经 `SetMethod` 的定义者，定义者内校验资格（跨表规则不用 CHECK）。
- 定义者 `payments.issue_payuni_qualification(...)`：owner `commerce_payment_registry_writer`，SECURITY DEFINER，`search_path=pg_catalog`；
  EXECUTE 仅 `commerce_payment_worker`（查询/对账作业在证据齐全时调用）与 `commerce_runtime`（只 LIVE 探针结果，函数内要求探针回执行存在且验签标记为真）。
  **不得**有「调用方传 proof_class」的捷径：proof_class 由函数从证据行推出。
- 验证尝试：`checkout.payment_attempts`（0016:29）加 `purpose text NOT NULL DEFAULT 'order' CHECK (purpose IN ('order','verification'))`；verification 尝试 order_id 规则由实现者按 0016 约束提出，写进 DELIVERY 由 integrator 裁决（不得为此放宽订单 FK 之外的约束）。
- ACL 钉子：`tests/foundation/payment_start_test.go`、`payment_capture_test.go`（qualification 读写 ACL）、`hosted_payment_authority_test.go`、
  `stripe_schema_test.go`（Stripe 签发路径不受影响）、`worker_authority_split_test.go` WAS02（只有 payment worker 可签发）。

## 写入路径
`internal/payments/{methods.go,payuni_activation.go（新）}`、`internal/integrations/psp/payuni/probe.go`（新，只读查询探针）、
`internal/httpapi/payment_activation.go`（新）、`migrations/0137_payuni_activation.sql`、`tests/foundation/payuni_activation_test.go`。
不改 `internal/payments/payuninotify/**`（W4-01B 所有）。

## 测试
| # | 用例 | 期望 |
|---|---|---|
| PA01 | 无资格 SetMethod(enabled) | 409 `not_qualified` |
| PA02 | MOCK 验证：CAPTURED + 通知 | 签 `PROVIDER_MOCK`；方法可启用；结账可见 |
| PA03 | 只有查询成功无通知 / 只有通知无 CAPTURED | 不签发 |
| PA04 | 验证尝试 | 不建订单、不写 inventory ledger、不进 finance 汇总 |
| PA05 | 凭据轮换 | 资格撤销，方法 `Available=false CREDENTIAL_ROTATED` |
| PA06 | `LC_PAYUNI_ENABLED=0` | 启用被拒 `PLATFORM_DISABLED` |
| PA07 | LIVE 探针：验签通过的查无交易 / 签名错 / 超时 | 签 REAL_LIVE / 不签 / 不签（UNKNOWN 不重试） |
| PA08 | 调用方伪造 proof_class / 跨店 connection | 拒绝 / 404 |
| PA09 | 手工单卡支付 | 方法启用后 LC-B6 卡选项可用；未启用不可用 |

## 门禁
`bash scripts/dev/test-focused.sh '^TestPayuniActivation'`；新模式 `bash scripts/dev/test-local.sh --payuni-activation`（MOCK，integrator 加）；
回归 `--payment`、`--payment-worker`、`--browser-payment`、`--browser-checkout-offline`、`--checkout`；`release-gate.sh --strict --only G07`；`check-gates.sh`；
`PAYUNI-SANDBOX` 门禁行（owner 测试商户；无则 NOT_RUN）。

## 证据 / 角色 / 依赖
PROVIDER_MOCK；SANDBOX 需 owner 测试商户；LIVE NOT_RUN（红线）。DeepSeek；K3；**Claude 钱路终审**。
依赖 W4-01B 合并。可与 W4-03B 并行开发（写入路径不重叠），但**同批上线**。

## OPEN（需 owner）
- PA-OPEN-1 LIVE 启用门槛：**推荐**「LIVE 凭据探针 + 平台开关」，首笔 LIVE 成交在后台提示商家去 PAYUNi 后台核对；备选「运维逐店批准」（同 Stripe D3）。
