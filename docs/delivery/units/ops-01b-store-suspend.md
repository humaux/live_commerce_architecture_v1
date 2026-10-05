# Unit OPS-01B — 平台运营 CLI：停用/恢复商家与店铺 + 运营审计（后端）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0143**。覆盖偏差 A8/B7（C-4 推荐「先 CLI」）、D9（租户暂停/恢复）、M21 #12 的最小部分。**不是**计划里的 OPS-01 SBOM（SBOM 另排，不在本批）。
worktree `.worktrees/ops-01b-store-suspend`（branch `unit/ops-01b-store-suspend`）。无 UI（owner 裁决：平台端只做 CLI）。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；`cmd/store-admin/doc.go`（运营 CLI 先例）、`deploy/scripts/ops-admin.sh` 头注释；架构 §6.1–6.2。

## Integrator 裁决（覆盖正文）
- 平台运营端 = **只有 CLI + 审计**，商家 > ~10 家或出现非技术运营人员时再做网页（owner 暂定，回写 07 §6 / 04:55 由 integrator 做）。
- 停用复用现有标志：`control.tenants.active`、`control.stores.active`（0001:11-23）——**不建新状态表**。
- 停用 ≠ 删除：不删任何数据、不撤销 PSP/Meta 绑定、不退款。**在途的钱照常入账**：PSP 回调/查询/对账、退款 worker 继续处理停用店铺的已有操作。
- 停用后**新的对外动作停止**：买家下单、认领自动回复/链接、提醒、广告/CAPI 新发送。
- 计费（按店、价格未定、`LC_BILLING_ENABLED` 保持关）与停用无关；本单元不碰 billing。

## 目标 / owner 流程
`ops-admin.sh platform-admin store-suspend --store <uuid> --operator <name> --ticket <ref> --reason <code>` → 商家后台立刻 403「店鋪已暫停」，
网店显示「暫停營業」，直播认领不再自动回复；`store-resume` 恢复。`tenant-suspend/resume` 作用于整个商家。`audit --since` 列出运营动作。

## 关键事实（bb71f966）
- `identity.resolve_access` 每次请求都 JOIN `t.active` 与 `s.active`（0001:94-95，最新定义见 0089）→ 商家端立即生效。
- 买家能力签发/使用检查 tenant/store active（0006:83-84,108-110）。
- **未核实**：已发布网店解析器（0093/0106 resolver）、Meta 认领 intake/claims-worker 自动回复、ads-worker、notify 邮件、expiry worker 是否看 active → 实现者逐一 `grep` 并补「停用 → 不对外」守卫（只在 SQL 定义者里加谓词，不在 Go 里散加）。
- 运营 CLI 先例：`cmd/store-admin`（专用登录角色 `commerce_storefront_registrar`，权限 = SQL EXECUTE 授权；stdout 只打 id/版本，stderr 固定码，不打 DSN）。
- `ops.audit_events.principal_id` FK 到 memberships（0001:54-63）→ 运营者不是成员，**不能**写进去 → 需要独立的运营审计表。

## 范围
1. 新 CLI `cmd/platform-admin`（同 store-admin 结构）：`store-suspend|store-resume --store`、`tenant-suspend|tenant-resume --tenant`、`status --tenant|--store`、
   `audit [--since RFC3339] [--limit ≤500]`。必填 `--operator`（1..40 `[a-z0-9._-]`）、`--ticket`（1..80 无控制字元）；suspend 必填 `--reason`
   （`fraud|non_payment|legal|owner_request|other`）。env `COMMERCE_PLATFORM_OPERATOR_DATABASE_URL`。幂等：已是目标状态 → 退出 0 + `unchanged`，仍写审计。
2. 定义者 `control.set_store_active(p_store, p_active, p_operator, p_ticket, p_reason)`、`control.set_tenant_active(...)`：同事务改标志 + 写 `control.operator_audit`；
   对停用的 store 的 OPEN 直播认领窗口**不强制关闭**（只停止对外动作），见 OPEN-1。
3. 「不对外」守卫：在已核实缺失的 SQL 入口加 `active` 谓词（例如 claims intake 规划自动回复、提醒规划、published resolver）；每加一处一个测试。
4. 表 `control.operator_audit(id, occurred_at, operator, db_user (session_user), action CHECK IN (...), tenant_id, store_id NULL, ticket, reason NULL, detail jsonb ≤ 1 KiB)`：
   **只追加**（无 UPDATE/DELETE 授权给任何登录角色；trigger 拒绝 UPDATE/DELETE）。OPS-02B 复用此表。
5. `deploy/scripts/ops-admin.sh` 加 `platform-admin` 分支（integrator 合并）。

## Non-goals
网页运营后台、数据删除、自动停用规则、给商家/买家发通知信、计费联动、SBOM。

## 合同修改
新增 `contracts/platform-operator-v1.md`（integrator 冻结）：CLI 子命令、停用语义（对内/对外/在途钱）、审计表、登录角色；`merchant-identity-v1.md` 补停用时的 403 码 `store_suspended`。

## SQL / 迁移 `0143_platform_operator.sql`
- 角色 `commerce_platform_operator NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION`（部署时给一个 LOGIN 成员，INHERIT TRUE、SET FALSE，同 stripe registrar 规则）。
- 定义者 owner `commerce_identity_writer`（或改 `control.*` 的现有 owner，实现前确认），SECURITY DEFINER，`search_path=pg_catalog`，EXECUTE **仅** `commerce_platform_operator`；
  `control.operator_audit` SELECT 仅经 `control.read_operator_audit` 定义者（同角色）。
- ACL 钉子：新 `tests/foundation/platform_operator_authority_test.go`（照 `stripe_authority_test.go` 正控/负控：正确 LOGIN 能停用；`commerce_runtime`、五个 worker、`commerce_storefront_registrar`、`commerce_stripe_ingress` 均无 EXECUTE）；
  `worker_authority_split_test.go` WAS02；`identity_independent_test.go`（resolve_access 未被改）。

## 写入路径
`cmd/platform-admin/{doc.go,main.go,main_test.go}`（新）、`migrations/0143_platform_operator.sql`、`tests/foundation/platform_operator_test.go`。
守卫若需改既有定义者：只在本迁移 `CREATE OR REPLACE`，并在 DELIVERY 列清单。integrator：`deploy/scripts/ops-admin.sh`、compose `platform-admin` 服务（profile ops）、secrets manifest。

## 测试
| # | 用例 | 期望 |
|---|---|---|
| OP01 | store-suspend 后商家任意 API | 403 `store_suspended`（或现有拒绝码），同租户其他店不受影响 |
| OP02 | 停用后买家开网店 / 下单 / 用旧 capability | 暂停页 / 拒绝 / 拒绝 |
| OP03 | 停用后留言认领 | 记录但不规划回复；不发提醒 |
| OP04 | 停用后 PSP 查询/回调到达 | 照常写 facts（在途钱不丢） |
| OP05 | resume | 全部恢复；无数据丢失 |
| OP06 | 重复 suspend | `unchanged`，审计 2 行 |
| OP07 | 审计表 UPDATE/DELETE（以 owner 以外任何角色） | 拒绝 |
| OP08 | runtime/worker 登录调用定义者 | permission denied |
| OP09 | CLI 输出扫描 | 无 DSN、无驱动错误原文 |

## 门禁
`bash scripts/dev/test-focused.sh '^TestPlatformOperator'`；`go test ./cmd/platform-admin/...`；回归 `bash scripts/dev/test-local.sh --browser-storefront`、`--meta-consumer`、`--payment-worker`；
`release-gate.sh --strict --only G07`；`check-gates.sh`。

## 证据 / 角色 / 依赖
REAL_PG/MOCK。DeepSeek；K3；**Claude 安全终审**。依赖：无硬依赖；若守卫要改 `integration.plan_claim_reply`，必须排在 W3-05B 合并之后（W3-04B/05B 都改该函数），
不得与二者并行。W3-03B 合并后补提醒守卫测试 OP03 后半。OPS-02B 依赖本单元（审计表、CLI）。

## OPEN（需 owner）
- OP-OPEN-1 停用时进行中的直播：**推荐**不强关窗口，只停对外动作；商家后台已不可用，等同冻结。
