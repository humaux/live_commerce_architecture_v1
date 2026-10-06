# Unit W6-05B — 失败 / UNKNOWN 外部操作台账（后端）

状态：IMPLEMENTED，待 Opus 复核（Claude Sonnet 后端实现者 2026-10-07；随后 Opus 复核，因为触及外部副作用的重试）。Base `r3/integration` `6277731e`。
迁移 **0159_operations_ledger.sql**（0157 perf、0158 keyword tools 并行）。worktree `.worktrees/w6-05b-operations-ledger`（branch `unit/w6-05b-operations-ledger`）。
Owner 裁决 2026-10-07：「失败台账：需要做」。来源：`output/arch-conformance/IMPLEMENTATION-PLAN.md` W6-05B（M21 #3、M07 #5、D6）。UI 在 W6-U2（Codex，不属本单元）。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件 → `contracts/external-operation-v1.md`（含本单元 Amendment W6-05B）→ `contracts/external-dispatcher-v1.md` 的「预算 / 耗尽」段。

## Integrator 裁决（覆盖正文）
无（本文件由实现者按 integrator 派发卡落成；争议点见下「合同边界」，由 Opus 复核裁决）。

## 目标
商家在后台看到本店所有「失败 / 状态不明」的外部操作（Meta 回复 / 私信、广告、CVS 出货单、直播读取、CAPI……），知道它属于哪个订单 / 认领 / 会话，
并能做三件受控的事：**查询**（让既有对账路径再读一次提供商状态）、**取消**（尚未派发的）、**授权重试**（合同明确允许的少数情形）。UNKNOWN 永不盲重试（I06/I07）。

## 范围（API，均 `/v1/admin/stores/{store_id}/…`，店铺来自服务端认证）
1. `GET operations?state=&cursor=&limit=` — 默认 `attention`（FAILED_FINAL / UNKNOWN / ACKNOWLEDGED / BLOCKED_POLICY / STALE_BINDING / READY）；
   `state` 可取 `attention|FAILED|FAILED_FINAL|UNKNOWN|ACKNOWLEDGED|BLOCKED_POLICY|STALE_BINDING|READY`。权限 `integration:read`。游标 `(created_at,id)` 键集分页。
2. `GET operations/{operation_id}` — 同一 DTO + 最近 ≤50 条事件（generation/state/reason_code/时间）。
3. `POST operations/{id}/query` · `POST operations/{id}/cancel` · `POST operations/{id}/retry` — 权限 `integration:execute`；`Idempotency-Key` 必填；
   body 恰好 `{"expected_attempts": <int>}`（CAS，防对着过期画面点按钮）。每个动作写 `ops.audit_events`（`integration.operation_query_requested` /
   `integration.operation_cancelled` / `integration.operation_retry_authorized`）与 `integration.operation_events`。
4. DTO 精确键（见合同 Amendment）：`operation_id, provider, action, purpose, state, reason_code, attempts, created_at, updated_at, object{kind,id}|null, actions{query,cancel,retry}{available,reason}`；
   详情另有 `events[]`。**不含** request / semantic_key / external_asset_id / provider_reference / principal_id / lease token / 原始提供商 body / 买家 PII。
5. 动作可用性是权威的服务端判断，DTO 的 `actions.*` 只是同一函数的结果；不可用时 `reason` 是稳定机器码，POST 同条件返回 `409 <reason>`。

## 合同边界（实现者的裁决点，请 Opus 复核）
- `query`：把一个新的 `external_operation_v1` River job 放进同一事务；调度器领取后因状态为 UNKNOWN/ACKNOWLEDGED/过期 DISPATCHING 只会走 **reconcile**，永不 dispatch。
  已耗尽 reconcile 预算（`reconcile_budget_exhausted`）的操作无法再被查询 → 新增 `integration.operations.generation_floor`：query/retry 把它设为当前 generation，
  调度器预算按 `generation - generation_floor` 计（默认 0，行为不变）。这是对 `external-dispatcher-v1` 预算语义的最小修改。
- `cancel`：仅 `READY`（从未被 claim 为 dispatch，或重试后重新 READY）→ `CANCELLED`；永不用于 DISPATCHING/UNKNOWN/ACKNOWLEDGED。
- `retry`：**不发明 UNKNOWN 重试**——UNKNOWN/ACKNOWLEDGED/过期 DISPATCHING 一律 `409 reconcile_first`（先 query；只有对账路径产出的 FAILED_FINAL 才算「已证明未执行」）。
  FAILED_FINAL 仅对合同登记的「安全重开」种类开放：`facebook/meta.live_videos`、`facebook/meta.live_insights`（只读 Graph 调用，Finish 为 latest-wins 幂等 upsert）。
  其余副作用种类（Meta 私信/回复、CVS、广告、CAPI）→ `409 retry_not_supported`，由各自领域界面新建尝试（原因：发送密文在终态被 `inbox.wipe_send_secret` 抹掉，
  CAPI 的 fbc/fbp/IP 在终态被清除，CVS 出货单 `settle_cvs_attempt` 已把旧尝试标 FAILED 并允许新尝试，重开旧操作会造成重复）。
  `READY` 且无存活 job（River 有限重试耗尽后停在 READY）的「重新入队」对所有种类开放：它从未派发，首次派发不可能重复。
  重试复用同一 operation id / semantic key / `lc:<operation_id>` 幂等键；generation 单调递增。
- 保护性（停止类）操作永不可取消：action 任一段以 pause/stop/disable/revoke/unsubscribe 开头（`meta.ads.pause` 等）→ `409 protective_operation`（SQL 权威）；取消待发的暂停会让广告继续花钱（Opus 复核 P1-1）。
- `query` 硬上限：每个操作滚动 24 h 内最多 5 次，第 6 次 `429 query_limit` + `Retry-After`。
- ads 车道（`meta_ads`、`meta_dataset`）只读 + 取消（保护性除外）：post_river/0015 `guard_ads_job_link` 要求每个 ads 操作只有 `operations.job_id` 那一个 job，query/retry 的后续 job 无法提交 → `409 lane_unsupported`（开放需另行评审该 guard）。
- 范围外：payment / media 车道（actor_kind BUYER_PAYMENT_QUERY / PAYMENT_REFUND / MEDIA_ATTEMPT 被既有 RLS 对商家隐藏，退款有自己的状态机）。

## 写入路径
`migrations/0159_operations_ledger.sql`；`internal/integrations/core/{ledger.go,dispatcher.go}`；`internal/pagination/pagination.go`（`operations` 集合）；
`internal/httpapi/{operations.go,operations_test.go,handler.go}`；`internal/httperror/error.go`（13 个原因码文案）；`cmd/api/{main.go,studio.go}`（接线 River insert-only client）；
`tests/foundation/{operations_queue_test.go,operations_queue_helpers_test.go,operations_queue_flow_test.go,external_operation_authority_test.go,r2_integration_upgrade_test.go,legacy_runtime_upgrade_test.go}`；`scripts/dev/test-local.sh`、`docs/delivery/GATES.md`；
`contracts/external-operation-v1.md`（Amendment）。

## 验收 gate
`bash scripts/dev/test-local.sh --operations-queue`（隔离真 PG，MOCK 提供商）：店铺隔离、DTO 精确键无密 / PII、取消在派发后被拒、UNKNOWN 无证明不可重试、
证明后重试不产生第二次提供商副作用（假提供商计数）、审计行、并发重试只出一个、预算耗尽后 query 恢复对账；加 T06 授权 / R2 迁移计数 / ACL 钉子。
证据级别：REAL_PG + MOCK。生产适配器、真实提供商、UI 均 NOT_RUN。

## Non-goals
payment/退款台账、跨租户视图、批量动作、对 UNKNOWN 的「标记已处理 / 强制成功」、新的提供商客户端、UI。
