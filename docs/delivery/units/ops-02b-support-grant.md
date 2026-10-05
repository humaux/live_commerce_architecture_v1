# Unit OPS-02B — 平台支持授权（限时、只读、带审计，后端 CLI）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0144**。覆盖 M01 #7（平台支持授权）、偏差 A8 推荐的第二条 CLI。worktree `.worktrees/ops-02b-support-grant`。无 UI。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件 → `ops-01b-store-suspend.md`；架构 §6.1；`docs/delivery/units/owner-provisioning.md`（principal 开通方式）。

## Integrator 裁决（覆盖正文）
- 支持授权 = 运营者用 CLI 给**一个指定的平台支持 principal** 开**限时、只读**的店铺访问；默认 4 小时，最长 72 小时；到期**自动失效（每次请求判断，不靠清扫任务）**。
- 默认权限包（不含个人资料）：`store:read, orders:read, catalog:read, inventory:read, live:read, integration:read`。
  **不含** `customers:*`、`inbox:*`、`payments:refund`、任何 write——见 OPEN-1（PII 访问需 owner 定规则）。
- 每次授予/撤销写 `control.operator_audit`（OPS-01B）**并**写商家可见的 `ops.audit_events`（动作 `support.granted` / `support.revoked`），商家在审计页可见。
- 支持 principal 用现有登录方式（密码/OIDC）登录，**不新增登录方式、不做冒充（impersonation）**。

## 目标 / owner 流程
商家反映「订单页怪怪的」→ 运营 `ops-admin.sh platform-admin support-grant --store <uuid> --principal <uuid> --hours 4 --operator alice --ticket T-123` →
支持人员登录后台只读查看该店 → 4 小时后自动失去访问；必要时 `support-revoke` 立即收回；`support-list --store` 查看。

## 关键事实（bb71f966）
- 授权表：`identity.memberships(tenant_id, principal_id, active, authz_revision)`、`identity.store_grants(… permission)`（0001:28-43），**无过期列**。
- `identity.resolve_access` 每请求派生权限（最新定义 0089，实现前确认）；`identity.sessions.audience` 已含 `'support'`（0001:48）但无使用者。
- `ops.audit_events.principal_id` FK → memberships（0001:63）。

## 范围
1. 表 `identity.support_grants(id, tenant_id, store_id, principal_id, permissions text[] CHECK (<@ 默认包), granted_at, expires_at CHECK (expires_at ≤ granted_at + 72h),
   revoked_at NULL, operator, ticket)`；同一 (store, principal) 至多一条有效授权（部分唯一索引 `WHERE revoked_at IS NULL`）。
2. `identity.resolve_access` 增加一个分支：当 principal 对该 store 无常规授权时，若存在有效支持授权（未撤销、`expires_at > clock_timestamp()`、租户与店铺 active）
   且所需权限 ∈ `permissions` → 允许，scope 标记 `via_support=true`（写审计/日志时可区分）。**常规分支逻辑不改。**
3. 店铺列表派生（登录后可选店铺）同样并入有效支持授权（实现者定位 `allowed_store_ids` 派生处，同一谓词）。
4. 支持 principal 不需要常规 membership：为满足 `ops.audit_events` FK，授权定义者在需要时插入 `memberships(active=false)` 占位行（active=false 使常规分支永远不放行），
   或由实现者提出更小方案写入 DELIVERY 供 integrator 裁决。
5. CLI（扩展 OPS-01B 的 `cmd/platform-admin`）：`support-grant --store --principal --hours 1..72 [--perm 子集] --operator --ticket`、`support-revoke --grant|--store --principal`、
   `support-list --store`。撤销时 `authz_revision+1` 使现有会话的缓存权限失效（若有缓存）。
6. 写类请求在 `via_support=true` 时一律拒绝（防御：即便将来误加 write 权限进包，Go 中间件也拒绝），返回 403 `support_read_only`。

## Non-goals
冒充商家登录、写操作、PII 访问（待 OPEN-1）、商家端「同意支持」流程（待 OPEN-2）、网页运营后台、支持工单系统。

## 合同修改
`contracts/platform-operator-v1.md`（OPS-01B 新建）增「Support grants」节；`contracts/merchant-identity-v1.md` Amendment：resolve_access 支持分支、`via_support`、`support_read_only`。

## SQL / 迁移 `0144_support_grants.sql`
- 表 ENABLE+FORCE RLS，无登录角色直接 GRANT。`identity.grant_support / revoke_support / list_support_grants`：owner `commerce_identity_writer`，SECURITY DEFINER，`search_path=pg_catalog`，
  EXECUTE 仅 `commerce_platform_operator`。`CREATE OR REPLACE identity.resolve_access`：签名、owner、EXECUTE 不变。
- ACL 钉子：`tests/foundation/identity_independent_test.go`（resolve_access 新分支：过期/撤销/停用店/越权权限全部拒绝）、`platform_operator_authority_test.go`（OPS-01B 新建，加 3 个函数行）、
  `merchant_orders_v2_acl_test.go`（只读放行、写拒绝）、`worker_authority_split_test.go` WAS02、`password_schema_test.go`（登录路径未变）。

## 写入路径
`cmd/platform-admin/support.go`（新）、`migrations/0144_support_grants.sql`、`internal/identityhttp/**` 或 scope 中间件中 `via_support` 写拒绝（实现前定位，单处）、
`tests/foundation/support_grant_test.go`。

## 测试
| # | 用例 | 期望 |
|---|---|---|
| SG01 | 授予 4h 后支持 principal 读订单 | 200；审计两表各 1 行 |
| SG02 | 读客户 / 读收件匣 / 退款 / 任何写 | 403（不在包内）/ 403 / 403 / 403 `support_read_only` |
| SG03 | 到期（测试用短时 + DB 时钟） | 下一请求即 403，无需清扫 |
| SG04 | revoke | 立即 403 |
| SG05 | 店铺被 OPS-01B 停用 | 支持访问也拒绝 |
| SG06 | `--hours 73`、`--perm customers:read` | CLI 拒绝 + SQL CHECK 拒绝 |
| SG07 | 常规成员行为回归 | 全部不变（identity 全套） |
| SG08 | runtime/worker 调用 grant 定义者 | permission denied |

## 门禁
`bash scripts/dev/test-focused.sh '^TestSupportGrant|^TestIdentity'`；`go test ./cmd/platform-admin/...`；回归 `bash scripts/dev/test-local.sh --browser-identity`、`--browser-admin-shell`、
`--browser-password-auth`；`release-gate.sh --strict --only G07`；`check-gates.sh`。

## 证据 / 角色 / 依赖
REAL_PG/MOCK。DeepSeek；K3（对 resolve_access 做越权对抗）；**Claude 安全终审**（认证核心改动）。依赖 OPS-01B 合并。不与任何改 `resolve_access` 的单元并行（W6-04B 角色包若排入需串行）。

## OPEN（需 owner）
- SG-OPEN-1 支持人员能否看买家个人资料（客户、私讯）：**推荐**首版不能；需要时商家自己截图。若要开放，需商家逐次同意 + 单独权限包。
- SG-OPEN-2 授权前是否要商家同意：**推荐**首版不要求（只有 owner 自己的店），但商家审计页可见；第二个外部商家上线前重新裁决。
