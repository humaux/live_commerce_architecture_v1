# Unit W3-05B — 限制下单名单（后端）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0133**。覆盖 IMPLEMENTATION-PLAN W3-05B（R5 波次 4 M27；M04–M11 报告 P2「黑名单」）。
worktree `.worktrees/w3-05b-blocklist`（branch `unit/w3-05b-blocklist`）。UI 在 W3-U2（名单管理）与买家面板按钮（W3-U2 一并做）。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/live-keyword-claims-v1.md` §1、§6；
`contracts/meta-claims-intake-v1.md` actor_key 节；`contracts/live-console-v1.md` §3.7、§5.1。

## Integrator 裁决（覆盖正文）
- 现状：无任何 blocklist。名单按店维护，键 = `actor_key`（按 app + 资产隔离，与 intake 同口径），**不存姓名/PSID 明文**。
- 名单内买家：认领**照记**，但**不签发链接、不发自动私密回复**，claim 标「已限制」；不对买家显示原因（不发「你被限制」）。
- 代建单（LC-B6）不硬拦，只给警告：本单元提供只读检查端点，**不改** LC-B6 文件。
- 必须在 W3-04B 合并之后开工（两者都 REPLACE `integration.plan_claim_reply`）。

## 目标 / owner 流程
商家在买家面板点「加入限制名單」（可写内部原因，仅后台可见）→ 之后该买家在任何场次留言认领，系统不再发链接，控台显示「已限制」；
商家可在直播設定的名单页移除。

## 关键事实
- 自动回复规划 `integration.plan_claim_reply`（0064:886；W3-04B 将在 0132 改写其体）。
- actor_key 生成规则见 meta-claims-intake（hash，按 app+资产）；买家面板读模型 LC-B3（0119）已有 actor/peer 映射。

## 范围
1. 表 `claims.blocked_actors(tenant_id, store_id, actor_key bytea CHECK octet_length=32, note text NULL CHECK length ≤ 200 且无控制字元,
   created_by uuid, created_at, PRIMARY KEY(tenant_id, store_id, actor_key))`，每店 ≤ 5000 行（I23）。
2. 定义者 `claims.block_actor(p_token, p_store, p_ref jsonb, p_note, p_key)` / `claims.unblock_actor(p_token, p_store, p_actor_key)`：
   `p_ref` 只接受服务器已知引用（`{comment_ref}` 或 `{conversation_id}` 或 `{bundle_id}`），由定义者自己解析出 actor_key——**客户端不能直接传
   actor_key**（防枚举/伪造）。权限 `live:manage`；审计 `claims.actor_blocked` / `claims.actor_unblocked`（不含 note）。
3. `plan_claim_reply` 分支：actor 在名单 → 不规划回复，claim 事件 `reply_kind='restricted'`，审计 `claim_reply_skipped:restricted`；
   **不消耗 `mpr:` 额度**（商家仍可人工私密回复一次）。
4. 读：`GET …/claims/blocklist?cursor`（列表：actor 的最近显示名取自现有 intake 读模型，`note`）；
   `GET …/claims/blocklist/check?bundle_id=` → `{restricted: bool}`（供建单抽屉与买家面板警告）。

## Non-goals
跨店/跨租户共享名单、按电话/地址拉黑、在 storefront 结账时拦截（买家没有登录，无法可靠识别）、自动规则。

## 合同修改
`contracts/live-keyword-claims-v1.md` 新增「Amendment W3-05B restricted actors」：键、`reply_kind=restricted`、额度不消耗、
不对外显示、端点与权限。`contracts/live-console-v1.md` §5.1 补「建单抽屉显示 restricted 警告」（integrator）。

## SQL / 迁移 `0133_claim_blocklist.sql`
- 表 FORCE RLS，策略 `app.tenant_id/app.store_id`；`commerce_runtime` 只 SELECT；`commerce_claims_intake` SELECT（经 `plan_claim_reply`
  定义者读，若定义者 owner 已有权限则不加直接 GRANT）。
- `block_actor`/`unblock_actor`：owner `commerce_claims_writer`（或 0060 claims 写定义者现有 owner，实现前 `grep` 确认），SECURITY DEFINER，
  `search_path=pg_catalog`，EXECUTE 仅 `commerce_runtime`，REVOKE PUBLIC。
- `CREATE OR REPLACE integration.plan_claim_reply`：签名、owner、EXECUTE 不变。
- ACL 钉子：`live_claims_schema_test.go`（KC03 新增两行）、`meta_claims_intake_schema_test.go`（MCI02 不变断言）、
  `worker_authority_split_test.go` WAS02。

## 写入路径
`internal/claims/blocklist.go`（新）、`internal/httpapi/blocklist.go`（新）、`migrations/0133_claim_blocklist.sql`、`tests/foundation/blocklist_test.go`。
integrator：`handler.go` 一行注册、合同 Amendment。

## 测试
| # | 用例 | 期望 |
|---|---|---|
| BL01 | 名单内买家留言认领 | claim 记录 `restricted`，0 外部操作，0 链接 |
| BL02 | 移除后再认领 | 正常发链接 |
| BL03 | 名单内买家，人工私密回复 | 允许一次（额度未被消耗） |
| BL04 | 客户端传 actor_key / 别店 comment_ref | 422 / 404 |
| BL05 | 无 `live:manage`（live_operator 只读角色） | 403 |
| BL06 | check 端点 | 名单内 true，其余 false；不泄露 note |
| BL07 | 5001 行 | `limit_reached` |
| BL08 | 日志/审计 | 无 note、无 actor_key 原值 |

## 门禁
`bash scripts/dev/test-focused.sh 'Blocklist'`；回归 `bash scripts/dev/test-local.sh --browser-live-claims`、`--meta-consumer`；
`release-gate.sh --strict --only G07`；`check-gates.sh`。

## 证据 / 角色 / 依赖
证据 MOCK。DeepSeek；K3；Sonnet 复核。依赖 **W3-04B 合并**（同函数）；可与 W3-02B 并行（路径不重叠）。

## OPEN
- BL-OPEN-1 IG 与 FB 同一个人 actor_key 不同：**推荐**接受（各自拉黑），身份图另立 verified-contact 合同。
