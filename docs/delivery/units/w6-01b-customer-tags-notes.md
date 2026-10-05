# Unit W6-01B — 客户标签与备注、列表按标签筛选（后端）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0139**。覆盖 IMPLEMENTATION-PLAN W6-01B（M16 #5；偏差 B25「先做标签/备注」）。
worktree `.worktrees/w6-01b-customer-tags-notes`（branch `unit/w6-01b-customer-tags-notes`）。UI 在 W6-U1。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/customers-billing-v1.md` §2、§3.1、§5、§6、§7、§12；架构 §6.1、§14.3–14.4。

## Integrator 裁决（覆盖正文）
- owner 暂定：会员/积分/分润**不进首版**；客户标签 + 备注先做（直播后跟进熟客）。
- 标签是**商家手打**的事实标签，不是推断标签：不做自动打标、不做 RFM、不做受众导出（架构 §14.3 推断标签另立合同）。
- 新权限 `customers:write`（加标签/写备注）；`customers:read` 可看。备注属于买家个人资料：**删除（erasure）必须连同删除**，买家自助导出包含备注（见 OPEN-1）。
- 必须在 W5-02B 之前合并（两者都 REPLACE `identity.read_merchant_customers`）；W5-02B 基于本单元的新函数体再改。

## 目标 / owner 流程
商家在客户列表/详情给买家加标签（「熟客」「愛殺價」「VIP」，店内自定义，带颜色）和备注（「只收 7-11」「上次少寄一件已補」）；
客户列表可按标签筛选；直播控台买家面板（LC-U2）只读显示标签。

## 关键事实（bb71f966）
- 客户 = `buyer.owners`（0006:11），无 profile 表；列表/详情定义者 `identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text)`
  （0078:816，EXECUTE `commerce_runtime`，`customers:read`；显示名/电话末 3 位取自最近订单 snapshot）。实现前 `grep` 确认 0078 之后无更新定义。
- 删除 `customers.erase_owner`（0078:460）、导出 `customers.record_export` / `buyer_read_privacy`；写角色 `commerce_privacy_writer`。
- 权限 CHECK `store_grants_permission_check` 用 0063 的「`pg_get_constraintdef` 重导」模式扩展（0078 §3.1 先例）。
- Go：`internal/customers/{read.go,types.go}`（`Customer`:75、`ListRequest`:129）。

## 范围
1. 表 `customers.tags(tenant_id, store_id, id, name text 1..20 NFC 无控制字元, color CHECK IN 8 色, created_at, UNIQUE(tenant_id,store_id,lower(name)))`，每店 ≤ 100；
   `customers.owner_tags(tenant_id, store_id, owner_id, tag_id, PRIMARY KEY(...))`，每客户 ≤ 20；
   `customers.notes(tenant_id, store_id, id, owner_id, body text 1..1000, author_id, created_at, edited_at, version)`，每客户 ≤ 200 条。
2. 定义者：`customers.create_tag/rename_tag/delete_tag`、`customers.set_owner_tags(p_owner, p_tag_ids uuid[], p_expected_revision)`（整组替换，CAS）、
   `customers.add_note/edit_note/delete_note`（作者或 owner/admin 可编辑删除）。全部 `customers:write`、审计 `customers.tagged` / `customers.note_*`（**不含正文**）、
   `Idempotency-Key`。
3. `read_merchant_customers` 增加：返回 `tags:[{id,name,color}]`；新参数 `p_tag uuid DEFAULT NULL` 过滤（**新签名 = 新函数**，旧签名 DROP，Go 同步改）。
   详情另加 `notes`（最新 50 条，分页参数）。
4. `erase_owner`：同事务删 `owner_tags` 与 `notes`；`buyer_read_privacy(p_detail=true)` 导出包含 notes 正文与标签名。
5. HTTP：`/v1/admin/stores/{store_id}/customers/tags`（CRUD）、`…/customers/{id}/tags`（PUT）、`…/customers/{id}/notes`（CRUD）、列表 `?tag=`。

## Non-goals
自动标签、分群、受众导出、会员等级、积分、跨店共享标签、批量打标（W5 导入不导标签）。

## 合同修改
`contracts/customers-billing-v1.md` 新增「Amendment W6-01B tags and notes」：表、上限、权限 `customers:write`、erasure/export 覆盖、列表筛选签名。
`contracts/merchant-identity-v1.md`：角色包加 `customers:write`（owner/admin + 客服包；按「角色矩阵采用计划推荐版本」裁决）。

## SQL / 迁移 `0139_customer_tags_notes.sql`
- 三表 ENABLE+FORCE RLS，策略 GUC；**无登录角色直接 GRANT**（与 customers.* 一致，只经定义者）。
- 定义者 owner `commerce_privacy_writer`，SECURITY DEFINER，`search_path=pg_catalog`，EXECUTE 仅 `commerce_runtime`；`erase_owner` 保持原 ACL。
- `store_grants_permission_check` 重导加 `customers:write`；角色包种子行（0089/0119 角色表）加 `customers:write`。
- ACL 钉子：`tests/foundation/customers_billing_schema_test.go`（函数/权限清单）、`customers_billing_consent_test.go`（erasure 覆盖新表）、
  `buyer_retirement_test.go`（若触及 owner 删除路径）、`worker_authority_split_test.go` WAS02（worker 无 EXECUTE）。

## 写入路径
`internal/customers/{tags.go（新）,notes.go（新）,read.go（新参数）,types.go（字段）}`、`internal/httpapi/customers.go`（路由）、
`migrations/0139_customer_tags_notes.sql`、`tests/foundation/customer_tags_test.go`。

## 测试
| # | 用例 | 期望 |
|---|---|---|
| CT01 | 建标签、打标、按标签筛选 | 列表只含该标签客户，分页稳定 |
| CT02 | 同名（大小写不同）标签 | 409 `tag_exists` |
| CT03 | 101 个标签 / 21 个打标 / 1001 字备注 | `limit_reached` / 422 |
| CT04 | set_owner_tags 版本漂移 | 409 `version_changed` |
| CT05 | 无 `customers:write`（viewer）/ 跨店 | 403 / 404 |
| CT06 | erasure | 标签关联与备注全删；审计无正文 |
| CT07 | 买家自助导出 | 含备注正文与标签名 |
| CT08 | 删除标签 | 关联同删，客户不受影响 |
| CT09 | 日志扫描 | 无备注正文 |

## 门禁
`bash scripts/dev/test-focused.sh '^TestCustomerTags|^TestCustomerNotes'`；回归 `bash scripts/dev/test-local.sh --browser-customers-billing`；
`release-gate.sh --strict --only G07`；`check-gates.sh`。

## 证据 / 角色 / 依赖
REAL_PG/MOCK。DeepSeek；K3；Sonnet 复核（PII 覆盖）。依赖：无。**必须先于 W5-02B 合并**；可与 W3/W4 任意单元并行。

## OPEN
- CT-OPEN-1 备注是否进买家自助导出：**推荐**进（是关于该买家的个人资料）；若 owner 认为内部备注不应给买家看，则需法律确认后改为仅删除不导出。
