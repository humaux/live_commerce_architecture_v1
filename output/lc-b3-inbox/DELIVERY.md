# DELIVERY — unit lc-b3-inbox（直播控制台收件箱读侧：A8–A11/A13/A14 + §3.1 重订阅任务）

- 角色: DeepSeek V4-Pro（后端执行者；不合并/不推送/不部署，integrator/Claude Opus 负责）。
- Base SHA: `d90d7fe3`；worktree: `.worktrees/lc-b3-inbox`；branch: `unit/lc-b3-inbox`。
- 合同: `contracts/live-console-v1.md` §16 LC-B3（§3.1/§3.2/§3.6/§3.7，§11 A8–A11/A13/A14，§14.8；FROZEN 2026-10-02）。
- 迁移号: `0122`（`migrations/0122_live_console_inbox.sql`）。
- 状态: 后端实现 + 单元测试 + 静态门禁完成；REAL_PG 迁移应用、全仓 race、UI、LIVE 流量均 NOT_RUN；handler/cmd/claims-worker 接线与 `--inbox` 门禁不在本单元写入路径（见「Integrator 接线」）。

## 证据标签总览

| 证据 | 标签 |
|---|---|
| 本文件 / 迁移注释 / 源码注释 | DESIGN |
| `internal/inbox` 与 `internal/httpapi` 单元测试 | UNIT（MODEL_ONLY） |
| `go build` / `go vet` / `gofmt` / `check_packet.py` | 静态（未跑 PG/平台验收） |
| REAL_PG 迁移应用、`go test -race ./...`、浏览器 UI、真实 FB/IG 流量 | NOT_RUN |

## 交付物（唯一写入路径，均在合同清单内）

实现：
- `migrations/0122_live_console_inbox.sql` — 完整前向迁移（见「迁移校验」）。
- `internal/metaconnect/graph.go` — §3.1：`subscribe` 的 `subscribed_fields` 由 `{"feed"}` 改为 `{"feed,messages"}`（第 199 行），注释同步。
- `internal/inbox/keyring.go` — Meta payload 密钥环镜像（`LoadKeyring`/`newKeyring`/`open`/`eventContext`，AAD 与 `digest` 校验逐字节对齐 `meta/payload.go`；不引入新密钥语义）。
- `internal/inbox/classifier.go` — 冻结分类器重放（`replayMessage`/`messageView`/`attachmentView`，`!digits(sender)||!digits(recipient)||sender==asset||recipient!=asset||!messageID(mid)||(echo&&echo!=false)` 五条隔离规则对齐 `normalize.go:176`）。
- `internal/inbox/service.go` — `Service`/`NewService`（nil 密钥环即不挂载）与 A8–A11/A13/A14 的请求/响应模型。
- `internal/inbox/read.go` — `ListConversations`（A8 元数据，无密文）、`ReadThread`（A9 头 + 开箱/解密 + 分类器重放，失败 `unreadable` 不丢弃）、`conversationMeta`、`BuyerPanel`（A13）、`databaseError`（只透传 PT400/PT403/PT404/PT409/PT422）。
- `internal/inbox/write.go` — `MarkRead`（A10，`read_seq` 仅 `inbox:reply` 生效）、`Takeover`/`Release`（A11 CAS）、`CustomerLink`（A14 CAS + 审计）。
- `internal/integrations/metareply/resubscribe.go` — `Resubscriber`（§3.1 一次性 `meta_resubscribe_v1` 任务的 claims-worker 执行端：PENDING→LEASED→SUCCEEDED|FAILED|UNKNOWN，429/503 有界重试，其余 UNKNOWN 不盲重试，终态抹除 sealed token + 审计）。
- 测试：`internal/inbox/inbox_test.go`、`internal/httpapi/inbox_test.go`。

未触碰：`apps/**`、`go.mod`/`go.sum`、OpenAPI 共享 schema、pnpm 锁文件、`contracts/*`、`scripts/dev/test-local.sh`、`handler.go`/`cmd/**`/`claims-worker` 运行循环（integrator 所有权，见下）。

## 红→绿证据

- 单元 `go test -v ./internal/inbox/... ./internal/httpapi/... ./internal/integrations/metareply/... ./internal/metaconnect/...` — **全绿**（`livecommerce/internal/inbox` 9/9、`internal/httpapi` 全绿含新增 6 个 inbox 测试、`internal/integrations/metareply` 全绿、`internal/metaconnect` 全绿）。见 `evidence-test.log`。
- 静态（见 `evidence-static.log`）：
  - `go build ./...` — exit 0；
  - `go vet ./internal/inbox/... ./internal/httpapi/... ./internal/integrations/metareply/... ./internal/metaconnect/...` — exit 0；
  - `gofmt -l`（本单元文件）— 空（全格式干净）；
  - `python3 scripts/check_packet.py` — `PASS_PACKET_STRUCTURE_ONLY`（requirements 22 / invariants 24 / gates 15 / tasks 23 / risks 32 / sources 50 / roles 7 / openapi_draft_operations 8 / chapters 30）。
- `internal/inbox` 测试覆盖：密钥环开箱往返 / 篡改 / 错钥 / digest 不符 / 坏上下文 / env 加载；分类器 5 条隔离 + 合法提取；`databaseError` 透传。
- `internal/httpapi` 测试覆盖：A8 cursor 编解码往返、A8/A9/A13 查询解析（含 `invalid_filter` 400）、`inboxClassify`（PT409 区分 `version_conflict`/`takeover_changed`）、A14 `customer_id:null` 解链体解码（严格 JSON + DisallowUnknownFields）。

## Integrator 接线（不在本单元写入路径，需 integrator 合并时执行）

1. **handler.go**：`httpapi.Options` 增加 `Inbox *inbox.Service`，`NewHandler` 内调用 `registerInboxRoutes(mux, pool, configured.Inbox)`（svc 为 nil 时不挂载，与其他 nil-able 服务一致）。
2. **cmd/api**：用 `inbox.LoadKeyring(...)`（读 `COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID` + `COMMERCE_META_PAYLOAD_KEYS_JSON`，与 meta payload 同一对 env）构造 `inbox.NewService(...)` 填入 Options。
3. **claims-worker**：把 `metareply.NewResubscriber(pool, keys, v2, cfg)` 接入运行循环（只在该进程持私钥环）。
4. **scripts/dev/test-local.sh**：登记 `--inbox` 门禁（LCN03、LCN10、G07）。
5. **W1-01B**（`integration.binding_capabilities` 回写 + capability probe）不在本单元范围。

> 角色、实际模型、reasoning、base SHA、worktree 与允许路径已记录于上；迁移编号/OpenAPI/共享 schema/go.mod/go.sum/pnpm 锁文件只有 integrator 可最终合并。

## 迁移校验

- `0122` 前向、幂等：precondition DO 块先校验依赖表/角色存在，再按 0079/0089 模式**重派生** `identity.store_grants_permission_check`（`pg_get_constraintdef` + `regexp_matches` WITH ORDINALITY）后追加 `inbox:read`/`inbox:reply`/`inventory:live_adjust`，杜绝覆盖后续值。
- 角色捆绑：`staff_role_permissions`（`CREATE OR REPLACE` 保留原 owner/属性，非 SECURITY DEFINER）`live_operator` 增加三权限、`viewer` 显式排除 `inbox:read`；owner/admin/live_operator 存量员工**增量**回填（`ON CONFLICT DO NOTHING`，不删除任何权限）。
- 定义者/权限：读侧 `social.read_thread`/`list_conversations`/`conversation_meta`/`unread_conversation_count` 与 `inbox.dm_window` owner `commerce_meta_writer`（EXECUTE 分别给 `commerce_runtime`/`commerce_claims_worker`）；写侧 `inbox.mark_read`/`takeover`/`release`/`customer_link`/`thread_opened` owner `commerce_inbox_writer` EXECUTE `commerce_runtime`；所有定义者经 `inbox.principal_holds`（owner `commerce_auth`）按服务端 GUC 复核 `inbox:read`/`inbox:reply`/`customers:read`。
- `integration.meta_resubscribe_jobs` 复用 0100 耐久作业模式（去掉 SUPERSEDED）：唯一 `(tenant_id, store_id, page_id)`、attempts≤5、30s·2^(n-1) 退避、终态抹除 sealed token + 审计；backfill 每 active 连接一行、幂等。
- 本单元**未在 REAL_PG 上应用迁移**（本环境按 AGENTS.md T02 未建立真实 PG 开发脚本）；等价正确性由结构审查与静态门禁支撑，合并后由 integrator 在真实 PG 上应用两次（fresh-migrated-twice）+ 跑 LCN03/LCN10/G07。

## 风险与不变量

- 不变量 P0：租户/店铺/主体现由服务端 GUC 确定（`platform.WithScope` + `set_config(..., true)`），请求内 `tenant_id`/Host 头/客户端金额一律不信任；定义者用 `current_setting('app.*')` 复核，`buyer.owners` 的 A14 可见性由 policy（GUC 范围）而非调用方决定，越界/不存在客户统一 PT404。
- 密钥环与分类器为**镜像复制**（meta payload `open` 未导出）：已逐字节核对其 AAD、`digest`、`valid*` 与 `normalize.go:176` 隔离规则；上游变更需同步本单元两份镜像（已在 DELIVERY 注明，属已知重复维护风险）。
- **惰性接管过期一致性**（本次修掉两处缺陷）：读侧（`dm_window`/`list_conversations`/`conversation_meta`）对已过期 human 线程统一报有效态 `auto` + generation+1 且 `assignee`/`human_until` 为 NULL；`takeover`/`release` 转移时显式 `human_until = NULL`、`release` 显式 `assignee_principal = NULL`，避免旧 `human_until` 立即二次过期。
- A9 `at` 用 `COALESCE(occurred_at, received_at)`（`social.messages.occurred_at` 可空）。
- **反应排除边界（LCN06）**：反应到达 `social.messages` 时在 SQL 层与用户消息不可区分（未持久化 is_echo/kind 标记、LC-B3 不改消费者/分类器），归属 LC-B4 的 `--inbox-send` 门禁；见迁移注释 §3.6。
- A8 `session_id`/`live_comment` 被接受但恒为空（评论已读透传属 LC-B2，未投影任何评论会话）；A13 `bundle_id` 一律 404、`claims`/`orders`/`auto_reply`/`display_name` 恒为空（bundle/claims/orders 属 LC-B4）；`dm_window_for_bundle` 未建（需 `inbox.bundle_peers` = 0123）。
- `check_packet.py` 重生成 `experiments/results/packet-check.json`（时间戳/hash 前进），非本单元写入路径，integrator 可保留或丢弃。

## NOT_RUN / BLOCKED

- NOT_RUN：REAL_PG 迁移应用与数据集成测试；`go test -race ./...`（AGENTS.md：T02 建立真实开发脚本前不跑）；LCN03/LCN10/G07 门禁（需 `--inbox` 接线后）；真实 FB/IG 流量与 Page 订阅生效；浏览器 UI（LC-B4/LC-U2）。
- BLOCKED：无环境阻塞（UI 需 node_modules/浏览器，本单元为后端）。

## 协调备注

- 本单元是纯后端；UI 验收（真实点击台账）属 LC-U2/LC-B4，不在 LC-B3 范围。
- `Resubscriber` 只认「确定未应用」答案（429/503）才 RETRY，其余 UNKNOWN 不盲重试（合同「外部 UNKNOWN 不盲重试」）；`finish_meta_resubscribe` 的 `UNKNOWN`/`FAILED`/`SUCCEEDED` 终态都会抹除 sealed token 并审计。
- 同一阻塞最多两次定向修复；本单元无 P0/P1 未解决项。

## 完成与失败

- 交付：实现、测试命令与退出码、证据文件（`evidence-test.log`、`evidence-static.log`）、风险、NOT_RUN/BLOCKED 列表，均在本目录。
- 独立复跑：单元与静态门禁由本执行者跑通；REAL_PG 迁移/门禁/接线由 integrator 合并时独立复跑，作者不能作为唯一验收人。
