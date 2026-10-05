# Unit W3-03B — 直播后结账提醒（仅 24 小时窗口内，后端）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0131**（+ 若需 `migrations/post_river/` 一条 River kind 授权，号由 integrator 给）。
覆盖 IMPLEMENTATION-PLAN W3-03B（M08 #5；R5 波次 4 M15；v5「直播後：未付款 N 筆」「直播後自動提醒」）。
worktree `.worktrees/w3-03b-checkout-reminder`（branch `unit/w3-03b-checkout-reminder`）。UI 在 W3-U2。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/live-console-v1.md` §3.3、§3.6、§4.1–§4.4、§10、§15 OPEN-11。

## Integrator 裁决（覆盖正文）
- **只在买家最近 24 小时内有过入站互动时发送**（`messaging_type=RESPONSE`）。Meta「实用讯息/固定格式」政策未核实（EVIDENCE_GAP；
  message tag 已于 2026-02-09 移除）→ 本单元**不得**使用任何 tag、UPDATE、utility 模板。窗口外买家只进「待人工跟进」清单 + 复制链接兜底。
- 发送一律走 LC-B4 的出站账本（`meta.dm_send` 的 auto 变体），**不另造发送路径**；takeover 规则按 §3.6 `origin=auto`。
- 每个买家每场至多一次提醒（不论手动还是自动触发）。

## 目标 / owner 流程
场次结束（或商家手动点「提醒未付款」）→ 系统列出「已认领未下单」与「已下单未付款」的买家 → 窗口内者排队发提醒（附结账/付款链接），
窗口外者列在「需买家先私讯」清单，商家可复制链接自行处理。商家在直播設定里可开「场次结束后 N 分钟自动提醒」（默认关）。

## 关键事实
- 24 h 窗口读：`inbox.dm_window`（0119:233）；按 bundle 的 `dm_window_for_bundle` 与 `inbox.bundle_peers` 由 **LC-B4** 交付（0119 头注释）。
- 模板：`internal/msgtemplates`（0121，`template_id` CHECK 现只有 `order-pay-link/v1`、`offer-recommend/v1`，0121:73）。
- 发送：LC-B4（branch `unit/lc-b4-sends`，进行中）交付 `inbox.plan_dm`、send secret、adapter route、Finish hook。本单元只调用其规划函数。
- 未付款订单：`checkout.orders.commercial_state='AWAITING_PAYMENT'`（0013:22）；认领未下单 = `claims.bundles` 有 ACCEPTED 行而无 `claims.order_origins`（0113）。

## 范围
1. 新模板 `checkout-reminder/v1`（0121 的 template_id CHECK 扩展；`public_safe=false`；变量 `{display_name?, link}`）。
2. 规划定义者 `inbox.plan_checkout_reminders(p_token bytea, p_store uuid, p_session uuid, p_trigger text) RETURNS jsonb`
   （`p_trigger IN ('manual','auto')`）：在一个事务里枚举候选买家 → 每人判 `dm_window_for_bundle` → 窗口内调用 LC-B4 `inbox.plan_dm`
   （origin=auto、`conversation_known=false`、`template_id='checkout-reminder/v1'`），窗口外写 `followup` 行；
   去重键 `crm:` + hex(sha256(session|owner_or_peer))[:48]（唯一索引）→ 第二次触发 `already_reminded`。
3. 自动触发：River kind `checkout_reminder_v1`（claims-worker 默认 lane），场次 ENDED 事件后 `delay_minutes`（店级设置 0=关，10..1440）入列。
4. 读：`GET …/live-sessions/{sid}/reminders` → `{sent:[…], queued, followup:[{bundle_id, display_name, reason: window_closed|human_takeover|no_peer, link_copy_allowed}]}`。
   「复制链接」返回现有 claim link / pay link（不新签发）。
5. 店级设置：`PUT …/live-settings/reminder {enabled, delay_minutes}`（`live:manage`，`expected_version` CAS）。

## Non-goals
窗口外发送、message tag、SMS/Email 提醒、多次提醒、模板编辑器（LC-B5 已有）、取消已排队提醒（走 LC-B4 现有状态）。

## 合同修改
`contracts/live-console-v1.md` 新增「Amendment W3-03B checkout reminders」：动作 = `meta.dm_send` origin=auto purpose `service`；
去重键 `crm:`；窗口外规则；设置路由；§4.4 send_state 复用。`contracts/live-keyword-claims-v1.md` 无改动。

## SQL / 迁移 `0131_checkout_reminders.sql`
- 表 `inbox.checkout_reminders(tenant_id, store_id, id, session_id, owner_id NULL, peer_key NULL, semantic_key UNIQUE, outcome
  CHECK IN ('queued','followup'), reason, operation_id NULL, created_at)`；`live.reminder_settings(tenant_id, store_id PK, enabled,
  delay_minutes CHECK 10..1440, version)`；两表 ENABLE+FORCE RLS，策略 `app.tenant_id/app.store_id` GUC；`commerce_runtime` 只 SELECT。
- 定义者 owner `commerce_integration_writer`（与 `inbox.plan_dm` 同 owner），EXECUTE `commerce_runtime` + `commerce_claims_worker`
  （自动触发）；`identity.resolve_access(…,'inbox:reply')`（manual）或作业 payload 中冻结的 store + 设置 enabled（auto）。
- ACL 钉子：`tests/foundation/live_console_inbox_test.go` 权限清单、`tests/foundation/worker_authority_split_test.go` WAS01/WAS02
  （claims worker 多一个 EXECUTE，payment/expiry/ads/media worker 仍无）、`external_operation_authority_test.go` 函数 ACL 段。
- 保留期：随 claims retention C3（`intake_days`），在 `claims-retention-purge-v1` 清单加一行（integrator）。

## 写入路径
`internal/inbox/reminders.go`（新）、`internal/httpapi/reminders.go`（新）、`cmd/claims-worker/main.go`（只注册 worker）、
`internal/msgtemplates/models.go`（只加常量）、`migrations/0131_checkout_reminders.sql`、`tests/foundation/checkout_reminder_test.go`。

## 测试（REAL_PG + 假 Graph `httptest`）
| # | 用例 | 期望 |
|---|---|---|
| CR01 | 3 买家：窗口内/窗口外/人工接管中 | 1 条 `meta.dm_send`；1 条 followup `window_closed`；1 条 followup `human_takeover` |
| CR02 | 同场再次触发（manual 后 auto） | 0 新操作，`already_reminded` |
| CR03 | 窗口在规划与 Check 之间关闭 | Check → BLOCKED_POLICY `window_closed`，不重试 |
| CR04 | Graph 超时 | UNKNOWN，不重发；`send_state=unknown` |
| CR05 | 已付款的单不提醒；取消的单不提醒 | 候选集合正确 |
| CR06 | 无 `inbox:reply`；跨店 | 403；404 |
| CR07 | 自动设置关闭 / delay 未到 | 不入列 / 到点入列一次 |
| CR08 | 日志扫描 | 无 PSID、无链接 token、无正文 |

## 门禁
`bash scripts/dev/test-focused.sh '^TestCheckoutReminder'`；新模式 `bash scripts/dev/test-local.sh --reminders`（integrator 加）；
回归 `--inbox`、`--live-console`、`--msg-templates`；`release-gate.sh --strict --only G07`；`check-gates.sh`。

## 证据 / 角色 / 依赖
证据 MOCK（假 Graph）；Meta LIVE = NOT_RUN（随 LC-X1 探针）。DeepSeek 后端；K3 门禁；Claude 终审（不得出现窗口外发送）。
依赖：**LC-B4 合并**（`plan_dm`、`bundle_peers`、`dm_window_for_bundle`）、LC-B5（0121）已合并。可与 W3-04B 并行（写入路径不重叠；
迁移都改 0121 的 template_id CHECK → 合并时 integrator 串行改写该 CHECK）。

## OPEN
- CR-OPEN-1 认领未下单的买家没有 PSID 映射（只有评论）时：**推荐**列 followup `no_peer`，不用私密回复额度发提醒（额度留给认领链接）。
