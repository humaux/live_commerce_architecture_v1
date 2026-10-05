# Unit W4-03B — PAYUNi 信用卡退款（后端，钱路）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0138**（+ 若需 `post_river/` 一条，integrator 给号）。覆盖 M14-11（偏差 B23「必须与启用同批交付」）。
**ID 说明**：计划 W4-03B（日对账）延后；本 ID 用于 PAYUNi 退款。worktree `.worktrees/w4-03b-payuni-refund`。UI 复用现有 `OrderRefunds.tsx`（W4-U1 只改文案/可用性）。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/stripe-refund-v1.md` RD1–RD11（状态机与额度规则照搬）、
`contracts/payment-capture-v1.md`「refund hint」段、`contracts/payuni-wire-v1.md`、`contracts/external-operation-v1.md`（UNKNOWN 规则）；架构 §12.3。

## Integrator 裁决（覆盖正文）
- 状态与额度规则**照搬 Stripe 退款**：`REQUESTED → SUBMITTING → ACKNOWLEDGED/PENDING → SUCCEEDED/FAILED/UNKNOWN`；
  `已成功 + 在途（含 UNKNOWN）+ 本次 ≤ 已捕获`，资金桶加锁；UNKNOWN 占额度、不盲重试、只查询对账（RD 系列）。
- RD6 不变：退款不改库存/订单/履约（R-3 维持；回库存只经 W3-08B）。
- 只做 `payuni_credit` 一次付清的退款；分期/ATM/CVS/LINE Pay 退款 out。
- PAYUNi 退款 API（端点、字段、部分退款、可退期限、请款前「取消授权」vs 请款后「退款」区别）**官方文档未在仓库核实** → 实现者第一步读官方文档并在合同 Amendment 引用页面 URL + 读取日期 +
  官方测试向量；查不到 → BLOCKED（不得猜字段）。查询 v2 只暴露「最后一笔退款」（payment-capture-v1）→ 不能用查询重建退款总额，本地账本为准。
- 证据上限 PROVIDER_MOCK；SANDBOX 需 owner 测试商户；**永不真钱**。

## 目标 / owner 流程
商家在订单详情点「退款」，选全额或部分金额 → 系统向 PAYUNi 发退款 → 状态「處理中 → 已退款 / 失敗 / 不確定（請到 PAYUNi 後台核對）」；
买家收到退款通知信沿用现有 refund 通知（若 Stripe 路径已有）。

## 关键事实（bb71f966）
- 退款入口 `merchantorders.RequestRefundIn` / `RefreshRefundIn`（`internal/merchantorders/refunds.go:134,241`），当前只走 `request_stripe_refund`；
  `payments:refund` 权限；env 守卫 `identity.merchant_refund_environment`（stripe-live 单元）。
- 账本表 `payments.stripe_refunds`、`payments.refund_facts`（0062）；Stripe 退款 worker `internal/payments/stripe_refund.go`。
- PAYUNi 凭据读取与 wire 加解密在 `internal/integrations/psp/payuni/`；payment worker 进程 `cmd/payment-worker`。

## 范围
1. `RequestRefundIn` 按尝试 provider 分支：`payuni` → 定义者 `payments.request_payuni_refund(...)`（同 Stripe 的锁与额度校验，写 `payments.payuni_refunds` 行 +
   外部操作 READY + River 作业，同事务）；`stripe` 路径不变。
2. River kind `payuni_refund_v1`（payment worker）：读凭据 → 发 PAYUNi 退款请求（单次，超时 → UNKNOWN）→ 写结果；`refund_facts` 只在 provider 明确成功时写（SUCCEEDED）。
3. Refresh：`RefreshRefundIn` 对 PAYUNi 行发查询（只读），用「最后一笔退款记录」匹配本地最新一笔；匹配不上 → 保持 UNKNOWN + review case。
4. 退款通知：PAYUNi 若对退款也发 NotifyURL（未证实），W4-01B 接收器只入列 refresh，不直接改状态。

## Non-goals
争议/拒付、结算报表、日对账（延后）、退款双人审批（B22 延后）、ATM/CVS 退款、自动退款、退款回库存。

## 合同修改
新 Amendment 写进 `contracts/stripe-refund-v1.md`「Amendment W4-03B PAYUNi provider」（把 RD1「PAYUNi refunds out」改为 credit 已纳入），
或独立 `contracts/payuni-refund-v1.md`——**推荐**前者（共享状态机与额度规则，单一真源）。`payuni-wire-v1.md` 加退款请求/响应字段表（官方引用）。

## SQL / 迁移 `0138_payuni_refund.sql`
- 表 `payments.payuni_refunds`（同 `stripe_refunds` 形状：tenant/store/id/attempt_id/amount_minor/currency/state/operation_id/provider_ref NULL/
  version/created_at…），ENABLE+FORCE RLS，`commerce_runtime` 只 SELECT 本店行。
- 定义者 `payments.request_payuni_refund`：owner 与 `request_stripe_refund` 相同（实现前 `grep` 0062 确认），SECURITY DEFINER，`search_path=pg_catalog`，
  `payments:refund`，EXECUTE 仅 `commerce_runtime`；结果写入定义者 EXECUTE 仅 `commerce_payment_worker`。
- 额度校验必须把 `payuni_refunds` 与 `stripe_refunds` 都计入同一订单资金桶（虽然一单只会有一个 provider，防御性求和）。
- ACL 钉子：`tests/foundation/stripe_refund_schema_test.go` 与 `stripe_refund_guards_test.go`（新增 PAYUNi 行；Stripe 行不变）、
  `worker_authority_split_test.go` WAS02（claims/expiry/ads/media worker 无退款 EXECUTE；payment worker 有）、`external_operation_authority_test.go` 函数 ACL。

## 写入路径
`internal/payments/payuni_refund.go`（新）、`internal/integrations/psp/payuni/refund.go`（新，wire 请求/响应）、`internal/merchantorders/refunds.go`（只加 provider 分支）、
`cmd/payment-worker/main.go`（只注册 worker）、`migrations/0138_payuni_refund.sql`、`tests/foundation/payuni_refund_test.go`。

## 测试
| # | 用例 | 期望 |
|---|---|---|
| PR01 | 全额退款 MOCK 成功 | SUCCEEDED，1 条 refund_fact，库存/订单不变 |
| PR02 | 两次部分退款合计 = 捕获 | 都成功；第三次 422 `exceeds_refundable` |
| PR03 | 并发两笔各 70%（-race） | 恰一笔被接受 |
| PR04 | provider 超时 | UNKNOWN，占额度，不重发；refresh 查询匹配后 SUCCEEDED |
| PR05 | 查询「最后一笔退款」与本地不符 | 保持 UNKNOWN + review case |
| PR06 | 非 credit 方法 / 未捕获 | 409 `refund_unsupported` / `not_captured` |
| PR07 | 无 `payments:refund` | 403 |
| PR08 | Stripe 退款回归 | 全绿 |
| PR09 | 日志扫描 | 无 EncryptInfo、卡号片段、HashKey/IV |

## 门禁
`bash scripts/dev/test-focused.sh '^TestPayuniRefund'`；新模式 `bash scripts/dev/test-local.sh --payuni-refund`（MOCK）；回归 `--payment-worker`、`--payment`、
`--browser-refund-fulfilment`、`--stripe-browser`；`release-gate.sh --strict --only G07`；`check-gates.sh`；`PAYUNI-SANDBOX` 行（无凭据 NOT_RUN）。

## 证据 / 角色 / 依赖
PROVIDER_MOCK；SANDBOX 需 owner；LIVE 永不。DeepSeek；K3；**Claude 钱路终审**。
依赖：W4-01B 合并（refresh 入列）；与 W4-02B 并行开发、**同批上线**（`LC_PAYUNI_ENABLED` 开关在两者都过 SANDBOX 门禁前保持 0）。
