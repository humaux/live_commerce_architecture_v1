# Unit LP-1 — 會員積分後端（members + append-only points ledger + manual-order award）

状态：DRAFT round 2（contract writer 2026-11-05，review 549 round 1 已改，待 integrator 冻结）。Base `r3/integration` `b8305338b`。
Role `commerce_worker`；worktree `.worktrees/loyalty-points-backend`（branch `unit/loyalty-points-backend`）。不得再委派。
迁移号 **0223**（provisional，integrator 分配后为准）。合同：`contracts/loyalty-points-v1.md`（LP1–LP19，LP9 已撤回；§1、§2、§4、§5）。
先读：AGENTS.md → 本文件 → 合同全文；`contracts/storefront-v2.md` §G3 + Amendment 2026-11-01；`contracts/customers-billing-v1.md` CD1/CD2/CD6–CD8 + Amendment W6-01B；`contracts/merchant-identity-v1.md` Amendment W6-01B；`internal/command/command.go`（receipt 规则）。

## 依赖
- 已合并：T14（customers 隐私定义者、`commerce_privacy_writer`）、T11（结账/`begin_hold`）、W6-01B（0139 权限 CHECK 重导模式）、FEAT-MANUAL-DISCOUNT（0220，`orders/manual` 可选 body key 的先例）。
- 与 PARITY-MANUAL-ORDER（手工單页面 owner）无文件重叠；但 `internal/merchanttools/manual.go` 若被其他在途卡修改，先 rebase，冲突交 integrator。
- LP-2（UI）在本单元合并、路由冻结后才开始。

## 范围
1. `migrations/0223_loyalty_points.sql`：`loyalty` schema、`members`、`point_entries`（reason 仅 `manual_order_award` / `manual_adjust`，CHECK 无 `[erased]` 逃逸分支）、不可变触发器（仅允许 LP11：已清除会员的非 NULL note → `[erased]`）、RLS ENABLE+FORCE、定义者（合同 §1 表）、权限 CHECK 重导加 `customers:points`、owner/admin 角色包种子 + 回填。**无**每表幂等键列。
2. Go 包 `internal/loyalty`（新）：输入校验（NFC、长度、控制字元）、每个写操作在同一事务里 `command.Run`（operation `loyalty.member.create|update|export|erase`、`loyalty.points.adjust`）+ 定义者调用、receipt 的 response 只存 id/数字/版本/状态（不存姓名、联系备注、备注、账本备注），HTTP 回应由同事务读取当前行构建；会员导出超过 1 MiB 拒绝 `export_too_large`；错误映射到合同 §2 拒绝码（23514 不得出现在 HTTP）；文件头注释写职责/依赖/调用方/不变量。
3. `internal/merchanttools/manual.go`：可选 `member_award` 进入 `command.Run` 请求哈希；**在任何管线写入之前**调用 `loyalty.precheck_award`（权限、会员存在且同店、未清除、上限），**仅在 probe 未找到 receipt 时执行**（重放永远返回已存 receipt），失败直接返回拒绝码、不下单；在 §G3 最终商家事务里、`mark_order_merchant_manual` 之后调用 `loyalty.try_award_manual_order`，它只返回 `awarded|not_awarded` 状态、绝不抛错（LP18），结果写入 receipt 与 `order.manual_created.details.member_award`（只含 id、数字、状态）。`for-buyer` 拒绝此键（LP16）。
4. HTTP：`internal/httpapi/members.go`（新）挂合同 §2 的八条 `/members**` 路由（搜索为 `POST /members/search`，body 传 `q`）；`orders/manual` 解析加可选键。
5. 合同修订：`contracts/merchant-identity-v1.md` 加 Amendment LP（`customers:points`，owner/admin）；`contracts/storefront-v2.md` 加 Amendment LP（`member_award` 键与 `state`）；`contracts/loyalty-points-v1.md` 状态行只改证据标签。

## write_paths
`migrations/0223_loyalty_points.sql`、`internal/loyalty/**`、`internal/merchanttools/manual.go`、`internal/merchanttools/manual_member_award_test.go`、
`internal/httpapi/members.go`、`internal/httpapi/members_test.go`、`internal/httpapi/merchanttools.go`（仅 body 键）、
`tests/foundation/loyalty_points_test.go`、`tests/foundation/customers_billing_schema_test.go`（追加 ACL 钉子）、`tests/foundation/worker_authority_split_test.go`（追加 WAS02 项）、
`contracts/merchant-identity-v1.md`、`contracts/storefront-v2.md`、`contracts/loyalty-points-v1.md`、`output/loyalty-points-backend/**`。
**integrator only**：迁移号最终分配、Go route table / `scripts/dev/contractdrift/baseline.json` / `contracts/route-table-dispatchers.json`、go.mod/go.sum（本单元不应新增依赖）。

## 不做
买家端页面/余额查询、通知或私讯、积分规则或自动计算、到期、折抵/兑换、赠品目录、自动撤回（取消/退款不动积分）、独立的 reversal 语义或按单上限（LP9 已撤回）、会员合并、与 `buyer.owners` 的任何关联、CSV 导入导出、广告/受众/报表使用、`for-buyer` 发积分、每表幂等键、搜索词进 URL、新 SQLSTATE、新依赖、River 任务。

## 门禁（先红后绿；每项在 `output/loyalty-points-backend/` 留「注错→红→修→绿」日志）
合同 §5 LPG01a、LPG01b、LPG02–LPG06、LPG06r、LPG07–LPG11 全部；每个失败调用断言确切拒绝码。最低命令（T02 脚本存在时）：`go test -race ./internal/loyalty/... ./internal/merchanttools/... ./internal/httpapi/...`、REAL_PG `go test ./tests/foundation -run 'Loyalty|CustomersBillingSchema|WorkerAuthority'`；迁移改动后跑全量 G07 与所有 ACL 类门禁（lesson：授权迁移后不只跑本单元测试）。
交付记录：角色、实际模型、reasoning、base SHA、worktree、write_paths、命令与退出码、证据文件、NOT_RUN/BLOCKED 列表。作者 PASS = E0；独立复核由 test_worker + security_reviewer。
