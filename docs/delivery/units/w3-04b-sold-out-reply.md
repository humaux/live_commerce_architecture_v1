# Unit W3-04B — 没货自动回复（后端）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0132**。覆盖 IMPLEMENTATION-PLAN W3-04B（M11 #15；M08 #1 缺货；v5「沒貨回覆模板」）。
worktree `.worktrees/w3-04b-sold-out-reply`（branch `unit/w3-04b-sold-out-reply`）。UI（模板编辑入口）在 W3-U2。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/live-console-v1.md` §4.2（额度规则已固定）、§14.1；
`contracts/meta-claims-intake-v1.md` 自动私密回复节；`contracts/live-keyword-claims-v1.md` §0.1、§6；`docs/delivery/units/claim-direct-checkout.md`。

## Integrator 裁决（覆盖正文）
- 现状：没货自动回复**只在合同里**（live-console §4.2 给了优先级），代码未实现。
- 没货回复**消耗该留言唯一一次私密回复**（`mpr:` 键，first writer wins）。补货后不能再对同一留言私密回复；只能等买家私讯（24 h 窗口内
  走 DM）或买家重新留言。合同与 UI 文案必须写明。
- 售完判断与 claim-direct-checkout 同口径：SKU `inventory_tracked` 且可售量 < 认领数量；A6 不追踪 SKU 永不售完。offer 暂停/停用也算没货。
- 认领照常记录（不丢买家意向，商家可在控台看到「没货已回覆」），但**不签发链接**。

## 目标 / owner 流程
买家留言关键字 → 命中的商品已售完（或商家已暂停）→ 系统自动私密回复「抱歉，XX 已售完，补货会在直播中通知」（店级可编辑模板），
而不是发一个点开才发现没货的结账链接。

## 关键事实（bb71f966）
- 自动回复规划：`integration.plan_claim_reply(uuid,uuid,bytea,text,bigint)`（0064:886，EXECUTE `commerce_claims_intake`）；
  Go 侧 `internal/claims/meta_intake.go`（`IngestMetaIntake`:61）、`internal/integrations/metareply/routes.go`。
- 售完读模型 `inventory.buyer_sku_availability` 属于 claim-direct-checkout（0116）——**在 bb71f966 未合并**（迁移目录无 0114/0116）。
- 可售量公式：`Σ(on_hand − reserved − allocated − unavailable)`（0086:200）。
- LC-B4（进行中）把 `mpr:` 规则、`link_pending_manual`、manual 让位逻辑落进代码；本单元在其之上加一个 auto 分支。

## 范围
1. 模板 `sold-out-reply/v1`（0121 template_id CHECK 扩展；`public_safe=false`；变量 `{offer_title}`；店级可发布版本，LC-B5 机制）。
2. 在自动回复规划处加分支：认领命中 offer 时，同事务内读售完（复用 0116 定义者或同公式）→ 售完则规划 `meta.private_reply`
   message_type `sold_out_reply`（同 `mpr:` 键、同唯一索引），不签发链接，写审计 `claim_reply_sold_out`；claim 事件带 `reply_kind='sold_out'`。
3. 店级开关 `sold_out_reply_enabled`（默认开；关闭时售完认领**不回复**，保留额度给人工），存本单元自己的 `claims.sold_out_settings`
   （不与 W3-03B 的设置表共用，避免迁移耦合）。
4. 控台读：评论标记 `send_state` 照 §4.4；`reply_kind` 让 UI 显示「沒貨已回覆」。

## Non-goals
补货通知（无合法通道）、自动暂停售完 offer（live-console §7.2 明确 v1 不做）、公开回复「已售完」、库存锁定。

## 合同修改
`contracts/live-keyword-claims-v1.md` 新增「Amendment W3-04B sold-out reply」：判定口径、额度消耗、不签发链接、开关；
`contracts/live-console-v1.md` §4.2 补一句 message_type `sold_out_reply` 与 auto 同级（integrator）。

## SQL / 迁移 `0132_sold_out_reply.sql`
- `CREATE OR REPLACE integration.plan_claim_reply(...)`（**签名不变**，同 owner、同 EXECUTE `commerce_claims_intake`；SECURITY DEFINER，
  `search_path=pg_catalog`）加售完分支；`meta.private_reply` 的 message_type CHECK 扩展 `sold_out_reply`。
- `claims.sold_out_settings(tenant_id, store_id PK, enabled bool default true, version)`，FORCE RLS，`commerce_runtime` SELECT；
  写入经定义者 `claims.set_sold_out_reply(p_token, p_store, p_enabled, p_expected_version)`（`live:manage`）。
- ACL 钉子：`tests/foundation/meta_claims_intake_schema_test.go`（MCI02 期望行，函数体变但 ACL 不变须断言）、`live_claims_schema_test.go`
  （KC03 新定义者行）、`worker_authority_split_test.go` WAS02（非 claims worker 无 EXECUTE）。

## 写入路径
`internal/claims/sold_out_reply.go`（新）、`internal/integrations/metareply/routes.go`（只加 message_type 分支）、
`internal/msgtemplates/models.go`（只加常量）、`migrations/0132_sold_out_reply.sql`、`tests/foundation/sold_out_reply_test.go`。
**与 W3-05B 冲突**（两者都 REPLACE `plan_claim_reply`）→ 串行：W3-05B 在本单元合并后基于新函数体再改。

## 测试（REAL_PG + 假 Graph）
| # | 用例 | 期望 |
|---|---|---|
| SO01 | 追踪 SKU 可售 0，留言认领 | 1 条 `sold_out_reply` 私密回复，0 链接，claim 记录 `reply_kind=sold_out` |
| SO02 | 不追踪 SKU 可售 0 | 正常发链接 |
| SO03 | offer 暂停 | 没货回复 |
| SO04 | 没货回复后商家手动私密回复同一留言 | `409 used` |
| SO05 | 开关关闭 | 不回复，额度保留，人工可回复一次 |
| SO06 | 人工先回复，再进认领（售完） | `claim_reply_skipped:reply_used`，不发第二条 |
| SO07 | 并发 20 次相同留言重放 | 恰好 1 个操作 |
| SO08 | Graph UNKNOWN | 不重发，`send_state=unknown` |

## 门禁
`bash scripts/dev/test-focused.sh 'SoldOutReply'`；`bash scripts/dev/test-local.sh --browser-live-claims`、`--meta-consumer`、`--inbox`（回归）；
`release-gate.sh --strict --only G07`；`check-gates.sh`。

## 证据 / 角色 / 依赖
证据 MOCK；Meta LIVE NOT_RUN。DeepSeek；K3；Claude 终审（额度不被双花）。
依赖：claim-direct-checkout 后端（0116 售罄读）合并、**LC-B4 合并**、LC-B5（0121）。与 W3-03B 并行安全（只 0121 CHECK 需 integrator 串行改写）。

## OPEN
- SO-OPEN-1 部分售完（认领 3 件只剩 1 件）：**推荐**按售完处理（回没货），不做部分链接——链接按认领数量签发，部分签发是新语义。
