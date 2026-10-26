<!-- Purpose: 平台代收 → 商家结算接口的规划：现状清单、缺口与最小方案、契约草案、拆卡和合规 owner gate。
Depends on: contracts/stripe-platform-account-v1.md §0.3/§6/§8–§11, migrations/0150_platform_settlement.sql, migrations/0163_settlement_resolve.sql,
  internal/payments/{stripeadmin,settlement}, internal/httpapi/settlements.go, cmd/stripe-admin/settlement.go, apps/admin 结算页（全部读自 origin/r3/integration@4a2bf1fe8）。
Used by: integrator（裁决、冻结、分配迁移号）、车队实现者（commerce_worker / ui_worker / test_worker）、Opus 金额复核。 -->
# 结算接口规划（settlement-api-plan）

**状态：DESIGN 草案，待 integrator 裁决。** 作者 claude-settlement-plan，2026-10-26，只读规划：没有写代码，没有跑测试。
**基线：** `origin/r3/integration@4a2bf1fe8`，最新迁移是 `0199`。下文的迁移号 `02xx` 只是占位，由 integrator 分配。
**证据级：** 本文是 E0 设计。文中「已测」只引用仓库里已有的 E3 记录（作者测试 + Opus 复核，REAL_PG/MOCK）。本轮没有复跑。

## 0. 权威与输入

| 域 | 权威 |
|---|---|
| 需求 | owner 原话。2026-10-25：「全部收款由平台统一代收（平台 Stripe 账户）；商家不配置 API/密钥/商户号，不接自有 PSP/Connect」。2026-10-26：「结算接口安排车队做」 |
| 架构 / 契约 | `contracts/stripe-platform-account-v1.md` §6（账本）、§6.6（未归属来源处理）、§8（LIVE 前 owner 清单）、§9（已知局限）、§11（owner 问题） |
| 实现 | `migrations/0150_platform_settlement.sql`、`0163_settlement_resolve.sql`，以及 `internal/payments/stripeadmin/settlement.go` 等代码 |
| 验收记录 | `output/w4-s2-platform-settlement/DELIVERY.md`、`REVERIFY-opus.md`（结论：MERGE，0 个 P0/P1）；`output/settlement-resolve/DELIVERY.md`（E3，20 个顶层测试 PASS） |
| 合规研究 | Humaux `0d608f39`（2026-10-06，GPT Pro 研究，integrator 用 Stripe 官方文档交叉核对过；Service Terms 的条号没有逐字核对） |

**纠正一条库内说法。** 10-25 的 decision 写着「平台代收的实际结算 API 仍未在仓库落地」，这句话不准确。r3/integration 上的结算账本已经能跑：同步 Stripe 余额交易 → 关账 → 记录打款 → 商家只读查询 → 导出 CSV。用的人是运维（CLI）和商家（只读 HTTP 和 UI）。真正缺的是本文 §2 列出的「可信度」缺口。

**规范冲突，报告一行，按 gate 走。** 契约 §0.3 AD-PF2（owner 2026-10-06）规定平台账户只替**同一法人的自营店**代收，第三方店铺必须先进 allowlist。10-25 的 owner 原话是「全部」平台代收。两者冲突时按 gate 处理：allowlist 继续作为 LIVE 的技术开关。在 SANDBOX 和开发环境里，所有店铺都按平台代收来设计和测试。§5 的 owner gate 通过之后，由 owner 本人放开第三方店铺。§0.3 的修订文本由 integrator 写（见卡 SA-0）。

## 1. 现状清单（r3/integration@4a2bf1fe8）

计数：**已测 12 · 已实现但测试不完整 4 · 只有契约未实现 5 · 完全缺失 6**。

### 1.1 已测（实现 + 自动化测试，CI 的 `foundation-shards` 覆盖全部 `tests/foundation`）

| # | 环节 | 实现 | 测试 / gate |
|---|---|---|---|
| T1 | 结算单周期：每周一 00:00 到下周一 00:00（Asia/Taipei），周期结束满 72 小时后才能关账 | `close_settlement`（0150:500 起，`period_not_closable`） | `TestPlatformSettlement/PF11_close` |
| T2 | 算法：`net = captured − refunded − dispute + stripe_fee(负) − platform_fee + carried_in`。平台费 = `round_half_up((captured−refunded)×bps/10000)`，按 TWD 100 的步长取整 | 表级 CHECK（0150:50），close 里的计算（0150:590 起） | `PF09_schema`；`PF11_close` 的费率向量（1046×100000/25640 → −4100）；mutation-fix 的 5 个单点注错都变红 |
| T3 | 金流费换算：HKD 手续费按该笔 charge 的比例折算到 TWD，按绝对值四舍五入 | `settlement_fee_store`（0150:238） | `PF11_close` |
| T4 | 余额交易同步 + SQL 内归属（数据只来自 Stripe API，不信任入参范围和 metadata） | `SettlementSync` / `record_settlement_lines` | `PF10_sync`、`PF10_ambiguous_payment_intent`、`PF10_platform_connection_assertion`、`PF10_recheck_transient_mismatch` |
| T5 | 关账：幂等重放、上一期未关拒绝（`previous_period_open`）、同步没覆盖拒绝（`sync_required`）、有 mismatch 拒绝、有未处理的 unmapped 拒绝全环境 | 0150:500–617 | `PF11_close` |
| T6 | 未归属来源处理：`not_store_revenue` / `assigned_to_store` 只追加、写入后不可再改、已关周期拒绝 | 0163，CLI `settlement-resolve` | `TestPlatformSettlementPF15Resolve`（8 个子测） |
| T7 | 负额结转：上一期 net < 0 时记入下一期的 `carried_in`（≤ 0） | close 里的 `v_carry:=least(prev.net,0)` | `PF11_close`，mutation「只改 carry-forward」变红 |
| T8 | 商家查询 HTTP：`GET /v1/admin/stores/{store_id}/settlements[?before&limit]`、`…/{statement_id}`；店铺由 token 决定；需要 `billing:manage` | `internal/httpapi/settlements.go`，`read_store_settlements` | `PF13_merchant_read_and_csv`，`internal/httpapi/settlements_test.go` |
| T9 | 商家 UI：列表、详情、付款状态、三种语言、手机宽度 | `apps/admin/components/Settlements.tsx`，`card-payments-settlements-model.ts`，BFF 白名单 `settlements|settlements/{uuid}` | `tests/admin/settlements.spec.ts` STL1–STL5（BFF fixture，MOCK） |
| T10 | 平台方打款**记录**：只能写一次；金额必须等于 net；net ≤ 0 拒绝；`paid_at` 不能晚于当前时间；相同内容重放返回原记录 | `record_settlement_payout`（0150:619），CLI `settlement-payout` | `PF12_payout` |
| T11 | 审计：sync、close、payout、resolve 都在同一事务里写 `ops.audit_events`，ticket 写进 details | 0150:490/608/648，0163:143 | `PF12_ticket_in_audit`，`PF15_cli_e2e` |
| T12 | 对账单导出：CSV，文件权限 0600，拒绝覆盖已有文件，防公式注入，无 PII 列 | `internal/payments/settlement/export.go` | `export_test.go`（2 个测试），`PF13` |

### 1.2 已实现，但测试不完整

| # | 环节 | 现状 | 证据 |
|---|---|---|---|
| P1 | 对账（同步窗口） | 每次 sync 打印 `stripe_net / recorded_net / difference`，PF10 断言差额为 0。**结果只打印，不入库**，关账也不检查它 | `SyncReport`（stripeadmin/settlement.go:60–69）；`settlement_sync_runs` 只有窗口列（0150:107–115） |
| P2 | 商家能否找到结算页 | 路由 `nav: false`，只能从「卡付款」页的链接进入（`card-settlements-link`） | `apps/admin/src/features/settings/routes.ts:63`，`CardPayments.tsx:229` |
| P3 | LIVE 前的 attestation 码（`platform_stripe_terms` 等） | 已写进 live-approve | `git grep -c platform_stripe_terms` → `0137_platform_stripe.sql`:2，`stripeadmin/live.go`:1；本轮没有逐个核对测试名 |
| P4 | `assigned_to_store` | 已经记录，并在关账时打印为 operator note，但**不进商家结算单**。钱由 owner 线下处理 | 契约 §9；`PF15_close_with_notes` |

### 1.3 只有契约，没有实现

| # | 内容 | 出处 | 证据 |
|---|---|---|---|
| C1 | K3 独立 gate `tests/foundation/platform_settlement_gate_test.go`（PF09–PF13） | w4-s2 brief「Independent tests (Kimi K3)」 | `git ls-tree -r origin/r3/integration tests/` 里没有这个文件；DELIVERY:54 写的是 NOT_RUN |
| C2 | PF-SBX：两家 SANDBOX 店真实同步 | 契约 §10 | 两份 DELIVERY 都标 NOT_RUN（没有 owner 的测试密钥） |
| C3 | mismatch 的处置路径 | 契约 §6.2：「operator resolves it outside this contract (escalation, NOT_IMPLEMENTED)」 | `git grep -iE 'mismatch.*resolv'` 在 migrations/internal/cmd 下只命中 0062 和 0073 里不相关的行 |
| C4 | 已关周期里晚到的 unmapped 行如何处理 | 契约 §9「Follow-up amendment (open)」 | 0163 遇到这种行返回 `period_already_closed`，并且会一直挡住整个环境的关账 |
| C5 | 「关账后 5 个工作日内付款」（OQ-2 的默认值） | 契约 §11 | 结算单没有应付日期字段，UI 也不显示 |

### 1.4 完全缺失

| # | 内容 | grep 证据（在 origin/r3/integration 下，`-- cmd internal migrations contracts`） |
|---|---|---|
| M1 | 打款**退回 / 更正**（银行退汇、登记错误）。payout 三元组只能写一次，现在唯一的修正手段是红线 SQL | `payout_revers|payout_correct|settlement_payout_event|bank_return` → 0 命中 |
| M2 | 对账结果入库，以及基于它的关账门 | `settlement_reconcil` → 0 命中 |
| M3 | 平台应付清单（已关账、未打款的结算单列表）。现在只能从 close 的 JSON 输出里抄 statement_id | `settlement-(payables|list|unpaid)` → 0 命中；CLI 只有 sync/close/export/payout/resolve 五个子命令（settlement.go:2–7） |
| M4 | `assigned_to_store` 的钱进入商家结算单（账本可见） | 契约 §9 明确不做；0163 文件头写着 v1 moves NO money |
| M5 | Stripe → 平台银行的 payout 对账 | balance txn 的 `payout` 类型落到 `unsupported_type` 的 unattributed 行（PF10），没有对应的对账 |
| M6 | 多币种结算单 | close 里把 `'TWD'` 写死了（0150 INSERT）；契约 §9 规定只支持一种币种 |

范围外（有意不做）：货到付款、银行转账、门市取货的钱不经过 Stripe，不进本账本；自动打款、银行 API、Connect transfer 都是 owner 红线，契约 §9 和 w4-s2 的 Non-goals 已经否决。

## 2. 缺口与最小方案

目标有两个：**商家能在后台看到可信的结算单**，**平台能把一笔打款登记为已结清**。这两条主路径都已经能跑。下面三项会让「可信」和「已结清」在出错时失真，按优先级排：

1. **M1 打款退回（P1）。** 银行退汇之后，结算单仍然显示「已付」，而钱实际回到了平台。最小方案：新增一张只追加的退回表。退回金额作为 `returned_in_minor` 加进该店的下一期结算单，原来的已付记录不改，留作历史。
2. **M2 对账入库 + 关账门（P1）。** 现在 sync 的差额只打印一次，事后无法证明某张结算单和 Stripe 账对得上。最小方案：sync 在**同一事务**里，把每个币种的「Stripe 侧合计 vs 入库合计」写进对账表，两边都由 SQL 从输入批次和表里重新计算，不采用 Go 传进来的合计。关账只承认差额为 0 的同步窗口。
3. **M3 应付清单（P2，运维必需）。** 只读的 SQL 定义函数加一个 CLI `settlement-payables`，让 owner 打款前能看到「该付谁、付多少、关账多久了」。没有它，登记打款要靠手抄 id。

同时修 P2：把 `nav` 改成 `true`，让结算页出现在设置导航里，并显示退回状态。

**这轮不做，以及何时再做：**
- M4（assigned 入账）：做的话需要拿到店铺币种的金额，但 unmapped 行只有 HKD 结算额，而且不能信任运维填的金额。等第一例真实发生再做（沿用 S2-OPEN-1 的建议）。
- C3 / C4（mismatch 处置、已关周期晚到行）：继续升级给 integrator 处理。第一次有一家店因此卡住一个周期，就开卡。
- C5 应付日期：用 M3 的 `closed_at` 和账龄代替。
- M5 / M6：要等 Stripe 的 payout 报表和多币种需求出现再做。
- 平台运营 Web 控制台：platform-operator-v1 已有 owner 裁决，商家超过约 10 家之前只用 CLI。

**P0 规则怎么落地：**
- **PG 唯一真源。** 新表、新函数都在 `payments` schema 里，用 FORCE RLS，撤销 PUBLIC 权限，没有 DELETE。
- **关键写入与任务同事务。** 退回、对账、审计行，都和主写入在同一个 SECURITY DEFINER 函数内完成。
- **幂等、可对账。** 退回以 `statement_id` 为主键，相同内容重放返回原记录，内容不同返回 PT409。对账以 `sync_run_id` 为键。
- **UNKNOWN 不盲重试。** Stripe 读失败或超过页数上限时返回 `ErrUncertain/ErrProvider`，一行都不写。sync 只是 GET，人工重跑是安全的。打款和退回都只是**记录人工已完成的事实**，系统不发起任何外部写操作，所以不存在外部 UNKNOWN。
- **范围由服务端决定。** 运维函数先调用 `require_stripe_registrar_scope`（平台店），目标店铺由 statement 行本身推出，不接受入参。商家读接口的店铺由 token 决定。
- **不信任客户端金额。** 退回金额必须等于该 statement 的 `payout_minor`，否则拒绝；它只做确认，不作为金额来源。对账合计由 SQL 计算。

## 3. 契约草案（作为 `stripe-platform-account-v1` §6.7–§6.9 的修订，由 integrator 冻结）

### 3.1 表（迁移 `02xx_settlement_returns_recon.sql`，只能前滚，单个事务）

```sql
-- §6.7 打款退回（只追加；每张 statement 最多一次退回）
CREATE TABLE payments.settlement_payout_returns (
 statement_id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL, store_id uuid NOT NULL,              -- 目标店（由 statement 推出）
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 return_ref text NOT NULL CHECK(return_ref ~ '^[A-Za-z0-9._:/-]{4,80}$'),
 returned_minor bigint NOT NULL CHECK(returned_minor>0),
 returned_at timestamptz NOT NULL,
 recorded_by text NOT NULL CHECK(recorded_by ~ '^[A-Za-z0-9._:@-]{2,64}$'),
 ticket text NOT NULL CHECK(ticket ~ '^[A-Za-z0-9._:-]{8,128}$'),
 carried_into_statement_id uuid,                                 -- 只能写一次：由哪一张后续 statement 带回
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(tenant_id,store_id,statement_id) REFERENCES payments.settlement_statements(tenant_id,store_id,id)
);
ALTER TABLE payments.settlement_statements
 ADD COLUMN returned_in_minor bigint NOT NULL DEFAULT 0 CHECK(returned_in_minor>=0);
-- 按名字替换 net 恒等式 CHECK（旧行 returned_in_minor=0，恒等式不变）：
-- net_payable_minor = captured − refunded − dispute + stripe_fee − platform_fee + carried_in + returned_in
-- guard_settlement_statement：returned_in_minor 和其他金额列一样不可改。

-- §6.8 同步对账（只追加；每个 sync run、每个结算币种一行）
CREATE TABLE payments.settlement_sync_reconciliations (
 sync_run_id uuid NOT NULL REFERENCES payments.settlement_sync_runs(id),
 settle_currency text NOT NULL CHECK(settle_currency ~ '^[A-Z]{3}$'),
 tenant_id uuid NOT NULL, store_id uuid NOT NULL,                -- 平台店的 scope
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 stripe_net_minor bigint NOT NULL,     -- 本批输入的 Σ net（SQL 从 p_lines 重新计算）
 recorded_net_minor bigint NOT NULL,   -- 窗口内 Σ lines.settle_net + Σ unattributed net（SQL 查表计算）
 difference_minor bigint GENERATED ALWAYS AS (stripe_net_minor-recorded_net_minor) STORED,
 PRIMARY KEY(sync_run_id,settle_currency)
);
```
授权与 0150 相同：owner 是 `commerce_payment_registry_writer`，只授予 SELECT,INSERT（退回表另加 `UPDATE(carried_into_statement_id)`，并由触发器限制只能写一次）。worker、ingress、runtime 没有任何权限。

### 3.2 SQL 入口

| 函数 | 授权 | 规则 |
|---|---|---|
| `payments.record_settlement_payout_return(p_tenant uuid,p_store uuid,p_principal uuid,p_statement uuid,p_return_ref text,p_returned_minor bigint,p_returned_at timestamptz,p_operator text,p_ticket text) RETURNS timestamptz` | EXECUTE 只给 `commerce_payment_registrar` | 未付款 → `payout_not_recorded`；`p_returned_minor ≠ payout_minor` → `return_amount_mismatch`；`returned_at < paid_at` 或晚于当前时间 → 22023；相同内容重放 → 返回原值；内容不同 → `return_already_recorded`（PT409）；同一事务内写审计 `stripe.settlement.payout_return` |
| `payments.read_settlement_payables(p_tenant uuid,p_store uuid,p_principal uuid,p_environment text,p_limit integer) RETURNS jsonb` | 同上 | 只读：列出 `net_payable_minor>0 AND payout_ref IS NULL` 的结算单，按 `closed_at` 升序；字段 `statement_id, store_id, period_start, net_payable_minor, closed_at`；不含 PII |
| `record_settlement_lines`（用仓库惯用的 `pg_get_functiondef` + replace 做原地补丁，锚点必须唯一） | 不变 | 写 `settlement_sync_runs` 的同一事务里，为每个币种写一行对账 |
| `close_settlement`（原地补丁） | 不变 | ① 覆盖检查只算所有币种 `difference_minor=0` 的 sync run；差额非 0 的窗口不算覆盖，关账报 `sync_unreconciled`（PT409）。② `returned_in_minor` = 该店尚未带回的退回之和，同一事务里写 `carried_into_statement_id` |
| `settlement_statement_json`（原地补丁） | 不变 | 商家 DTO 新增 `returned_in_minor`、`payout_returned`（bool）、`payout_returned_at`（omitempty）；仍然不含 return_ref、Stripe id 和结算币种金额 |

### 3.3 Go（签名草案）

```go
// internal/payments/stripeadmin
func (r *Registrar) SettlementPayoutReturn(ctx context.Context, s Scope, statement, returnRef string,
	amountMinor int64, returnedAt time.Time, operator, ticket string) (time.Time, error)
func (r *Registrar) SettlementPayables(ctx context.Context, s Scope, environment string, limit int) ([]Payable, error)
type Payable struct {
	StatementID     string    `json:"statement_id"`
	StoreID         string    `json:"store_id"`
	PeriodStart     string    `json:"period_start"`
	NetPayableMinor int64     `json:"net_payable_minor"`
	ClosedAt        time.Time `json:"closed_at"`
}
// SyncReport 新增：SyncRunID string `json:"sync_run_id"`，取 SQL 返回的值；Difference 改为读回入库值，不再由 Go 计算。

// internal/payments/settlement.Statement 新增
ReturnedInMinor  int64   `json:"returned_in_minor"`
PayoutReturned   bool    `json:"payout_returned"`
PayoutReturnedAt *string `json:"payout_returned_at,omitempty"`
```

CLI（`cmd/stripe-admin`，加入 `ops-admin.sh` 白名单，不接受任何密钥，不调用 Stripe）：
- `settlement-payout-return --statement --return-ref --amount --returned-at --operator --ticket`
- `settlement-payables --environment [--limit 1..200] --operator --ticket`

**HTTP：不新增路由。** 商家的两个 GET 路由保持不变，DTO 只做加法。admin 的 `card-payments-settlements-model.ts` 严格解析器要把三个新键加进去（`returned_in_minor` 是必填键，另外两个是可选键）。

### 3.4 不变量（新增 S-I1…S-I6，挂到 `contracts/invariants.json` 的 I05/I06/I19/I20 之下）

- **S-I1** `net = captured − refunded − dispute + stripe_fee − platform_fee + carried_in + returned_in`，由表级 CHECK 保证。
- **S-I2** 一张 statement 最多一次退回；退回金额等于原 payout 金额；退回只被一张后续 statement 带回一次（`carried_into_statement_id` 只能写一次 + UNIQUE）。
- **S-I3** 关账依据的每个同步窗口，在所有币种上的 `difference_minor` 都为 0。
- **S-I4** 系统从不调用银行、Stripe payout 或 transfer 的写接口。打款和退回都只是记录。
- **S-I5** 所有新写入都和审计行在同一事务；没有 UPDATE（只能写一次的列除外），没有 DELETE。
- **S-I6** 商家响应里没有 Stripe id、return_ref、其他店的数据、结算币种金额。

## 4. 拆卡

依赖关系：**SA-0 冻结 → {SA-1, SA-2, SA-3} 并行 → SA-2 的真实端到端在 SA-1 合并后再跑**。每张卡用独立的 worktree 和分支，最终合并迁移号、ACL pin、`ops-admin.sh` 白名单和 R2 迁移计数 pin 的只有 integrator。

| 卡 | 角色 | ELEVATE | 写入路径（唯一归属） | 依赖 | 红测试（先写，注错→红） | 验收 gate |
|---|---|---|---|---|---|---|
| **SA-0** 契约冻结 | integrator（架构） | ✅ 金额规则 | `contracts/stripe-platform-account-v1.md`（§0.3 记录 10-25 决定 + allowlist 作为 LIVE gate、§6.7–§6.9、§9、§10 PF16–PF18） | 无 | 不适用（文档）；冻结的 PF16–PF18 向量必须写出算术过程 | `python3 scripts/check_packet.py`；Opus 金额复核签字 |
| **SA-1** 后端：退回 + 对账 + 应付清单 | commerce_worker | ✅ | `migrations/02xx_settlement_returns_recon.sql`；`internal/payments/stripeadmin/settlement.go`；`internal/payments/settlement/service.go`；`cmd/stripe-admin/settlement.go`；`tests/foundation/settlement_returns_test.go`（作者冒烟测试） | SA-0 | 在当前基线上先跑：①登记打款后无法表达退回（PF16 红）；②sync 差额 ≠ 0 时关账仍然成功（PF17 红）；③没有 payables 读入口（PF18 红）。红跑日志存 `output/settlement-returns/red.log` | `bash scripts/dev/test-focused.sh '^(TestPlatformSettlement|TestR2IntegrationUpgradeFromReleaseHead|TestStripeAuthority|TestStripeAdmin|TestOpsAdmin)'`；`go vet ./...`；`bash scripts/dev/check-gates.sh`；CI `foundation-shards`；Opus 金额复核 0 个 P0/P1 |
| **SA-2** 商家 UI：退回状态、带回金额列、进导航 | ui_worker | ✅ 金额展示与符号 | `apps/admin/lib/card-payments-settlements-model.ts`、`…-copy.ts`；`apps/admin/components/Settlements.tsx`；`apps/admin/src/features/settings/routes.ts`（只改 settlements 条目的 `nav`）；`tests/admin/settlements.spec.ts`；`tests/admin/fixtures/card-payments-wire/settlement-returned.json` | SA-0 冻结 DTO（用 fixture 并行开发）；真实端到端依赖 SA-1 合并 | STL6：带有 `payout_returned=true` 的 fixture，在旧解析器上应当失败；STL7：设置导航里应当有「结算单」入口，当前 `nav:false` 时失败 | `tests/admin/settlements.spec.ts` STL1–STL7（含 WebKit 和 390px 宽度）；admin typecheck；`card-payments-model.test.ts` 的符号和恒等式断言包含 returned_in |
| **SA-3** 独立验收 gate | test_worker（不是 SA-1 的作者） | ✅ | `tests/foundation/platform_settlement_gate_test.go`（新文件：补齐 PF09–PF13 的 K3 债务，再加 PF16–PF18） | SA-0（只按契约写，不读 SA-1 的实现）；和 SA-1 并行 | 在当前基线上：PF16–PF18 必须红；PF09–PF13 必须绿（证明测试本身有效），再做一次单点注错（例如把 carry 改成 0）让它变红并留日志 | 先在 SA-1 分支上全绿，再在合并后的 SHA 上全绿；结果绑定 SHA，写进 `output/settlement-returns/gate-*.log` |

PF16–PF18（由 SA-0 冻结）：
- **PF16 退回**：只能退一次；金额 ≠ payout 时拒绝；未付款时拒绝；`returned_at < paid_at` 时拒绝；相同内容重放返回原值；下一期的 `returned_in` 等于退回金额且只带回一次；net 恒等式成立；有审计行；商家 DTO 不含 return_ref。
- **PF17 对账**：fixture 中丢一行，造出差额 → 写入对账行（difference ≠ 0），关账报 `sync_unreconciled`；再做一次干净的 sync，差额为 0，关账成功；对账表不可 UPDATE/DELETE。
- **PF18 应付清单**：只列出 net > 0 且未付的单；登记打款后从清单里消失；runtime 和商家角色没有 EXECUTE 权限。

## 5. 合规：上线收真钱前的 owner gate（不阻塞开发）

owner 已经决定平台代收（10-25），本文不推翻。下面列的是**收第一笔 LIVE 真钱之前**必须备齐的书面材料。没有这些材料，`live-approve` 的 attestation 码（契约 §8，代码里已经有：`platform_stripe_terms`、`merchant_terms`、`tax_invoice`、`payout_ops`）就不应该由 owner 签。SANDBOX 和全部开发工作不受影响。

| 风险（研究 `0d608f39`，2026-10-06） | 需要的书面材料 | 对应的 attestation |
|---|---|---|
| Stripe 服务条款 §3.4(b)：普通账户不得充当 payment facilitator / aggregator，也不得「代收不是自己提供的商品或服务的款项」，否则可能被限制或冻结（条号未逐字核对） | Stripe 的**书面批准**（support ticket 或客户经理邮件），明确允许这个香港账户为台湾第三方店铺做 business of record 代收，或者给出指定的替代方案；ticket id 写进 `approval_ref` | `platform_stripe_terms`（OQ-1，阻塞 LIVE） |
| 台湾《电子支付机构管理条例》第 3 条（代理收付实质交易款项）、第 5 条小额例外；《洗钱防制法》第 6 条第三方支付登记（违者有刑责）；代收款需要信托专户或银行履约保证 | **台湾执业律师的书面意见**：说明平台的模式是否构成代理收付、需要哪些登记或例外、代收款如何保管 | 新增建议码 `tw_legal_opinion`（由 SA-0 决定是否加进 §8） |
| 发票和税：卖方主体是谁、平台费单独开票 | 会计师的方案（统一发票由谁开、如何开） | `tax_invoice`（OQ-5） |
| 和商家的代收款约定 | 代收款协议文本和版本（默认 `pf-2026-10`） | `merchant_terms`（OQ-4） |
| 向台湾商家打 TWD 款的银行路线 | owner 指定的银行路线，以及 Stripe payout 已开通 | `payout_ops`（OQ-6） |

**放开第三方店铺**（把 AD-PF2 的 allowlist 从「只限自营店」扩大到所有店铺）也属于这个 gate：材料齐了之后由 owner 本人执行 `platform-allow`，agent 不代做。**真实转账**永远由 owner 本人在银行完成，系统只记录（S-I4）。
