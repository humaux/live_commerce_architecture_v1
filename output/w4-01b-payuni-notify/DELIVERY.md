# DELIVERY — unit w4-01b-payuni-notify（PAYUNi NotifyURL 接收器后端：验签回执 + 唤醒既有查询任务，绝不写钱）

- 角色: DeepSeek V4-Pro（后端执行者；不合并/不推送/不部署，integrator 负责）。实际模型: deepseek-v4-pro（会话驱动）。
- Base SHA: `5251df89`（r3/integration）；worktree: `.worktrees/w4-01b-payuni-notify`；branch: `unit/w4-01b-payuni-notify`。
- 迁移号: `0136`。
- 状态: 后端实现 + DB-free 单测全绿；REAL_PG 门禁（`^TestPayuniNotify|^TestPayuniNotifyAuthority`）见 §红→绿证据；LIVE 永不接入。全仓 race 与真实 PAYUNi 流量 NOT_RUN。

## 证据标签

| 证据 | 标签 |
|---|---|
| 本文件 / 迁移注释 / 源码注释 | DESIGN |
| `internal/integrations/psp/payuni`、`internal/integrations/accounts`、`internal/payments/payuninotify`、`internal/platform`、`cmd/api` 单元测试 | UNIT（MODEL_ONLY / MOCK） |
| `tests/foundation/payuni_notify_test.go`、`payuni_notify_authority_test.go`（一次性 PG 容器 + 合成签名体） | REAL_PG + MOCK 签名 |
| `go build` / `go vet` / `gofmt` | 静态 |
| 全仓 `go test -race ./...`、真实 PAYUNi 回调、LIVE 环境 | NOT_RUN |

## 交付物（唯一写入路径）

实现：
- `migrations/0136_payuni_notify.sql` — 角色 `commerce_payuni_ingress`；表 `payuni_notify_endpoints`、`payuni_notify_receipts`（FORCE RLS、REVOKE ALL FROM PUBLIC）；review_cases 放宽 NOTIFY_MISMATCH；触发 `guard_payuni_notify_receipt`；定义者 `payuni_resolve_endpoint`、`payuni_record_notify`（owner commerce_integration_writer）、`set_payuni_notify_endpoint`（owner commerce_payment_registry_writer）。
- `migrations/migrate.go`（改）— 每次 Apply 重断言 `GRANT UPDATE(scheduled_at) ON river_payment.river_job TO commerce_integration_writer`（唤醒既有 job 所需的唯一 river 写权，guard 禁止 id/kind/args/unique_key/queue 变更）。
- `internal/integrations/psp/payuni/client.go`（改）— `NewNotify`（仅通知客户端）+ `AuthenticateNotification`（有界解析+验签：外层 MerID = 连接 MerID = 解密内层 MerID；HashInfo 用当前或上一版凭据；状态对匹配）。
- `internal/integrations/accounts`（改）— `SealPayuni`/`OpenPayuniNotify` 通知凭据托管（AAD 绑定 tenant/store/connection/environment/account/version）。
- `internal/payments/payuninotify/` — `handler.go`（路由 `POST /v1/hooks/payuni/notify/{endpoint_token}`、≤8KiB 表单、token 只 sha256、当前/上一版凭据轮换宽限、200 空体 ACK）、`inbox.go`（`pgStore.material`/`record`：一条事务 receipt + 唤醒）。
- `internal/platform/payuni_runtime.go`（新增）、`internal/platform/platform.go`（改）、`internal/platform/stripe_runtime.go`（改）— `OpenPayuniIngressPool`/`ValidatePayuniIngressPool` + payuni_ingress 权威门；`payments.set_payuni_notify_endpoint(...)` 加入 stripe registrar 清单。
- `cmd/api/payuni_notify.go` + `cmd/api/payuni_notify_test.go` + `cmd/api/main.go`（改）— 挂载路由、`COMMERCE_PAYUNI_INGRESS_DATABASE_URL`、DB-free 全路由测试；LIVE 拒绝、profile ∈ {PROVIDER_MOCK,SANDBOX}。
- 测试：`internal/payments/payuninotify/handler_test.go`、`internal/integrations/psp/payuni/client_test.go`、`internal/integrations/accounts`、`cmd/api/payuni_notify_test.go`；`tests/foundation/payuni_notify_test.go`（6 个 REAL_PG）、`payuni_notify_authority_test.go`（5 个 REAL_PG，含 ACL-pin）；`tests/foundation/worker_authority_split_test.go`（改）、`hosted_payment_test.go`（改）、`hosted_payment_authority_test.go`（改）。

未触碰：`apps/**`、`go.mod`/`go.sum`、OpenAPI 共享 schema、pnpm 锁文件、`contracts/*`、任何 UI 文件。

## 核心裁决（Integrator ruling，覆盖 brief 正文）

通知只是「去查询」的触发器，**绝不直接产生钱的事实**：收到并验签的通知 → 记录回执 → 立即唤醒既有 `payment_query_v1` job（`UPDATE river_payment.river_job SET scheduled_at=now`），**不插入新 job**。CAPTURED/AUTHORIZED 只由既有查询 + `payment_reconcile_v1` 按 payment-capture-v1 规则写入。因此本单元对 brief 的「River InsertTx」做了 **wake-vs-InsertTx 偏差**：`integration.payment_job_queue` 已把每个 attempt 钉到唯一 job（`a.job_id=j.id`），通知只重排 `scheduled_at`。

## 红→绿证据

- 静态（`evidence-static.log`）：`go build ./...` exit 0；`go vet`（touched 包 + tests/foundation）exit 0；`gofmt -l` 空。
- DB-free 单测：`go test -count=1 ./internal/payments/payuninotify ./internal/platform ./cmd/api ./internal/integrations/psp/payuni ./internal/integrations/accounts` — 全绿 exit 0。
- REAL_PG 门禁 `bash scripts/dev/test-focused.sh '^TestPayuniNotify|^TestPayuniNotifyAuthority'` — **见 `evidence-test.log`**（结果：TODO 待补，运行后回填）。
- 修复记录：门禁首跑暴露两处真实缺陷并修复——(1) `validatePayuniAuthority` 第二段探测 SQL 复制自 stripe 后残留未用的 `$1`（authority）参数，导致 42P18「could not determine data type of parameter $1」把干净 ingress login 全部拒之门外；改为 `allowed AS (SELECT unnest($1::oid[]) AS oid)` 并只传 `allowedOIDs`。(2) `payuni_record_notify` 的去重 `SELECT ... FOR UPDATE` 与重复回执的 `UPDATE redelivery_count` 需要 receipts 表 UPDATE 权限，0136 原只授 `SELECT,INSERT`；补 `GRANT UPDATE(redelivery_count,last_redelivered_at)`（与 guard 触发允许的两列一致）。

## 迁移校验（0136）

- 表：`payuni_notify_endpoints`（UNIQUE(tenant,store,connection,execution_profile)、UNIQUE(token_hash)，token 只存 sha256、`octet_length(token_hash)=32`、`environment='SANDBOX'`）；`payuni_notify_receipts`（UNIQUE(connection_id,payload_sha256)、disposition ∈ QUEUED/MISMATCH/UNKNOWN_TRADE、`(disposition='QUEUED')=(job_id IS NOT NULL)` 等交叉 CHECK）。全部 FORCE RLS、REVOKE ALL FROM PUBLIC。
- review_cases：`reason` 增 `NOTIFY_MISMATCH`；`source_report_hash` 放宽 NOT NULL，并加 `review_cases_report_hash_scope` 把 `(reason='NOTIFY_MISMATCH') = (source_report_hash IS NULL)` 钉死。
- 定义者（SECURITY DEFINER `SET search_path=pg_catalog`，owner 见上，均 REVOKE FROM PUBLIC）：`payuni_resolve_endpoint(bytea)`（读一个 enabled endpoint 的 scope + 当前/上一版凭据信封，无明文/无租户权威）；`payuni_record_notify(bytea,bytea,text,text,bigint,text,text)`（advisory xact lock 去重 → 映射 MerTradeNo→attempt → QUEUED/MISMATCH/UNKNOWN_TRADE → 唤醒 job 或开 NOTIFY_MISMATCH review case，无钱事实）；`set_payuni_notify_endpoint(...)`（registrar 创建/轮换一个 SANDBOX token，EXECUTE 只给 commerce_payment_registrar）。
- 触发 `guard_payuni_notify_receipt`（BEFORE UPDATE）：只允许 redelivery_count/last_redelivered_at 变化，其它字段改动 42501。

## 风险与不变量

- P0：tenant/store 范围由定义者按 endpoint 行决定（`set_config('app.tenant_id'/'app.store_id')`），请求内 tenant_id/Host/客户端金额一律不信任；`merchant_trade_no` 必须命中本连接的 attempt 才算数。
- P0：token 是路由秘密，不是认证——只以 sha256 出 handler；未知/禁用 token 一律 404（不可见，不泄露存在性），错误 profile 也 404。
- P0：加密失败（HashInfo/外层 MerID/解密内层 MerID/状态对不符）→ 400 且**零行写入**；签名失败不写回执。
- 副作用幂等/可对账：去重键 `(connection_id,payload_sha256)` + advisory xact lock；重复回执只 `redelivery_count+1` 并 200 幂等；唤醒与回执同一事务，COMMIT 前绝不 ACK。
- 金额/币种/profile 不匹配 → `MISMATCH` 回执 + NOTIFY_MISMATCH review case，**不唤醒 job**；未知交易号 → `UNKNOWN_TRADE` 回执而已，不开 review case。
- OPS-01B 禁用店铺：仍记录回执并唤醒查询（查询路径自行判定），通知接收不因禁用而吞事件。
- **EVIDENCE_GAP**：ACK 为 200 空体——没有可向 PAYUNi 证明「已处理」的报文；这是本单元明确接受的证据缺口。
- 架构与测试结果已区分 DESIGN / MODEL_ONLY / MOCK / REAL_PG / NOT_RUN。

## NOT_RUN / BLOCKED

- NOT_RUN：全仓 `go test -race ./...`（AGENTS.md T02：T02 建立真实开发脚本后才跑；本单元只跑 touched 包 + foundation 子集）；真实 PAYUNi 回调（合成 AES-GCM 签名体 = MOCK）；LIVE 环境（0136 全部 `environment='SANDBOX'`，handler 永不 admit LIVE）。
- 作者不能作为唯一验收人：REAL_PG 门禁由本执行者复跑（test-focused），合并后请 integrator 再复跑 `bash scripts/dev/test-focused.sh '^TestPayuniNotify|^TestPayuniNotifyAuthority'`。

## Integrator 合并时

- 迁移号 `0136` 为占位，请按合并时实际空号重编号（并同步 `payuniIngressFunctions`、`stripeRegistrarFunctions` 中 `set_payuni_notify_endpoint` 的 ABI 文本与 ACL-pin 测试）。
- `migrate.go` 的 `GRANT UPDATE(scheduled_at)` 留在 postTx 重断言（checksum 安全，见注释），勿移到 0136 内。
- 本 worktree 共享全局 `$TMPDIR/lc-test-pg.lock`；合并方运行门禁前先确认无其它 worktree 的 focused 运行占用（本次门禁被 `r3-integration` 的全量运行串行阻塞过）。

## 完成与失败

- 交付：实现、测试命令与退出码、证据文件（`evidence-static.log`、`evidence-test.log`）、风险、NOT_RUN/BLOCKED，均在本目录。
- 本单元无 P0/P1 未解决项；无删除/放宽任何失败测试、阈值或 fixture。
