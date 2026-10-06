# Unit W4-01B — PAYUNi NotifyURL 接收器（后端，钱路）

**已合并（651c5744, 0136）但从未部署；owner 2026-10-06 取消 PAYUNi → 保持关闭（`COMMERCE_PAYUNI_NOTIFY_ENABLED` 不设），由 `pay-remove-payuni.md` 删除。**

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0136**。覆盖 IMPLEMENTATION-PLAN W4-01B 的「补上 NotifyURL（B4）」部分（拆成独立单元：激活 W4-02B 依赖它证明回调可达）。
worktree `.worktrees/w4-01b-payuni-notify`（branch `unit/w4-01b-payuni-notify`）。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/payuni-wire-v1.md`（全文短，重点 :14-27、Acceptance and next boundary）、
`contracts/payment-query-v1.md`、`contracts/payment-capture-v1.md`「Decisions」与「SQL authority」、`contracts/payment-hosted-v1.md` HostedConfig；架构 §12.2。

## Integrator 裁决（覆盖正文）
- **通知只是「去查询」的触发器，绝不直接产生钱的事实。** 收到并验签的通知 → 记录回执 → 立即入列现有 `payment_query_v1`；
  CAPTURED/AUTHORIZED 只由现有查询 + `payment_reconcile_v1` 路径按 payment-capture-v1 规则写入。这样未核实的 ACK/重试语义不会影响资金正确性。
- PAYUNi 背景通知的传输、Content-Type、ACK 内容与重试策略**官方未证实**（payuni-wire :19-21）→ 本单元先读官方文档并在 DELIVERY 引用页面与日期；
  查不到 → ACK 用 HTTP 200 空体（**标 EVIDENCE_GAP**），在 SANDBOX 探针中观察 PAYUNi 是否重送，结果回写合同。**不得照抄 SDK 关闭 TLS 校验等做法。**
- 证据上限：PROVIDER_MOCK；owner 提供 PAYUNi 测试商户后可做 SANDBOX；**永不 LIVE、永不真钱**。
- 停用店铺（OPS-01B）的通知仍要记录并查询（在途的钱不能丢）。

## 目标
买家在 PAYUNi 付款完成后，即使没回到 ReturnURL，系统也能在秒级发现并确认订单（目前只靠轮询查询）。

## 关键事实（bb71f966）
- wire 包 `internal/integrations/psp/payuni/`（`client.go`、`amount.go`）有 `VerifyNotification` 有界表单解析，但**无 HTTP 接收器**，合同明令不得直接暴露。
- 查询作业 `paymentQueryArgs` Kind `payment_query_v1`（`internal/payments/query_worker.go:35`）；对账作业 `payment_reconcile_v1`（`capture_worker.go:33`）。
- `payments.provider_observations.source CHECK(source='QUERY')`（0017）——**保持不变**，通知不写观察表。
- Stripe 先例：独立入口角色 `commerce_stripe_ingress` + `internal/payments/stripewebhook/`（handler/inbox）+ `stripe_authority_test.go` 入口矩阵。

## 范围
1. 公开路由 `POST /v1/hooks/payuni/notify/{endpoint_token}`（cmd/api 挂载；`endpoint_token` = 每个连接一个 32 字节随机值的 base64url，
   落库存 sha256；用于把通知路由到 tenant/store/connection，**不当作认证**）。请求体 ≤ 8 KiB，form 解析复用 wire 包 `VerifyNotification`。
2. 验证：外层 MerID = 连接 MerID = 解密内层 MerID；HashInfo 用该连接**当前或上一凭据版本**校验（轮换宽限）；`MerTradeNo` 映射到本连接的支付尝试；
   金额/币种与冻结尝试不一致 → 记回执 `mismatch` + 现有 review case，不入列查询。验不过 → 400，不落任何业务行（只计数日志，无请求体）。
3. 回执表去重：`(connection_id, payload_sha256)` 唯一；重复通知 → 200（幂等），不重复入列。同一尝试 60 s 内最多入列一次查询（与轮询共用 River unique 选项）。
4. 入列：同一事务插回执 + `InsertTx(payment_query_v1{attempt_id, reason:'notify'})`；**不在请求处理中调 PAYUNi 查询**。
5. 响应：200 + ACK（按官方文档；未证实则空体）；数据库不可用 → 503（让对方重试，若对方会重试）。

## Non-goals
从通知直接确认付款、ATM/CVS 虚拟账号到账（后续）、退款通知（W4-03B 处理退款查询）、自动轮换 endpoint_token、多租户共享端点。

## 合同修改
`contracts/payuni-wire-v1.md` 新增「Amendment W4-01B notify receiver」：路由、验证顺序、回执去重、「通知 = 查询触发器」、ACK 证据状态；
`contracts/payment-query-v1.md` 加 `reason:'notify'`。

## SQL / 迁移 `0136_payuni_notify.sql`
- 角色 `commerce_payuni_ingress NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION`（stripe_ingress 同式）。
- 表 `payments.payuni_notify_endpoints(tenant_id, store_id, connection_id, token_hash bytea(32) UNIQUE, created_at, revoked_at)`、
  `payments.payuni_notify_receipts(tenant_id, store_id, id, connection_id, attempt_id NULL, payload_sha256 bytea(32), outcome CHECK IN
  ('queued','duplicate','mismatch','unknown_trade'), received_at, UNIQUE(connection_id,payload_sha256))`；ENABLE+FORCE RLS；无直接 GRANT 给登录角色。
- 定义者 `payments.payuni_resolve_endpoint(p_token_hash bytea)`（返回 scope + 凭据引用版本，不返回密钥明文；密钥按现有 accounts 解密路径在 Go 取）
  与 `payments.payuni_record_notify(...)`：owner `commerce_payment_registry_writer`，SECURITY DEFINER，`search_path=pg_catalog`，
  EXECUTE **仅** `commerce_payuni_ingress`；River 入列走该函数内 `river` 授权（若需 post_river 授权，integrator 给号）。
- ACL 钉子（新增+更新）：新 `tests/foundation/payuni_notify_authority_test.go`（照 `stripe_authority_test.go` 的正控/负控矩阵：正确开通的 LOGIN 能开池并完成真实记录；
  runtime/worker/stripe_ingress 无 EXECUTE）；`worker_authority_split_test.go` WAS01/WAS02（五个 worker 无此 EXECUTE）；`hosted_payment_authority_test.go`（NotifyURL 现在指向真实路由，断言不变式）。

## 写入路径
`internal/payments/payuninotify/**`（新：handler、验证、记录）、`cmd/api/main.go`（只挂路由 + env `COMMERCE_PAYUNI_INGRESS_DATABASE_URL`）、
`migrations/0136_payuni_notify.sql`、`tests/foundation/payuni_notify_test.go`。integrator：compose/secrets manifest、合同、`internal/platform` 池常量（若需）。

## 测试（红跑 → `output/w4-01b-payuni-notify/red.log`）
| # | 用例 | 期望 |
|---|---|---|
| PN01 | 官方 golden vector 构造的合法通知（MOCK） | 回执 `queued`，1 个 `payment_query_v1`，0 条 facts |
| PN02 | 同体重放 ×5 | 1 回执 + 4 次 `duplicate`，1 个作业 |
| PN03 | 错 HashInfo / 外内 MerID 不一致 / 错 endpoint_token | 400 / 400 / 404，0 行 |
| PN04 | 金额与冻结尝试不符 | `mismatch` + review case，无作业 |
| PN05 | 凭据轮换后用旧版本签名 | 宽限内接受，超出拒绝 |
| PN06 | 通知声称成功但查询返回未付 | 订单仍未确认（facts 只来自查询） |
| PN07 | 9 KiB 体、重复字段、非 form | 413 / 400 / 415 |
| PN08 | 停用店铺的通知 | 仍记录并入列 |
| PN09 | 日志扫描 | 无 EncryptInfo、无 HashKey/IV、无请求体 |
| PN10 | 路由冲突 | DB-free 测试构建完整 `cmd/api` mux |

## 门禁
`bash scripts/dev/test-focused.sh '^TestPayuniNotify|^TestPayuniNotifyAuthority'`；新模式 `bash scripts/dev/test-local.sh --payuni-notify`（integrator 加，MOCK）；
回归 `--payment`、`--payment-worker`、`--browser-payment`；`release-gate.sh --strict --only G07`；`check-gates.sh`；新门禁行 `PAYUNI-SANDBOX`（无凭据 = NOT_RUN）。

## 证据 / 角色 / 依赖
PROVIDER_MOCK；SANDBOX 仅 owner 提供测试商户后；LIVE 永不在本单元。DeepSeek；K3（含 golden vector 独立复算）；**Claude 钱路终审**。
依赖：无（现有 wire/查询已合并）。可与 W3 任意单元并行；W4-02B 依赖本单元。
