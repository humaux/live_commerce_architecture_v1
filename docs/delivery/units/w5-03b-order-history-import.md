# Unit W5-03B — 历史订单 CSV 导入（只读档案，后端）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0141**。覆盖 IMPLEMENTATION-PLAN W5-03B（M22 #7）。worktree `.worktrees/w5-03b-order-history-import`。UI 在 W5-U1（导入）与 W6-U1 之后的客户详情（显示）。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件 → `w5-02b-customer-import.md`；`contracts/migration-import-v1.md`（W5-02B 冻结后）；架构 §12.4。

## Integrator 裁决（覆盖正文）
- 历史订单是**只读档案**：不进 `checkout.orders`、不进 `payments.facts`/`refund_facts`、不写库存、不进财务汇总/报表/CAPI/归因（I05）。
- 只能挂在已导入的顾客（`external_ids` kind=customers）上；找不到顾客 → 行失败 `customer_not_imported`（不自动建顾客）。
- 金额只存展示用 `total_minor` + `currency='TWD'`；不存支付方式细节、卡号、银行账号；收件地址**不导入**（只存「城市」级，见 OPEN-1）。

## 目标 / owner 流程
导入顾客后，再上传 SHOPLINE 订单 CSV → 预览 → 提交 → 客户详情出现「歷史訂單（SHOPLINE）」列表（日期、单号、金额、状态、品项摘要），
让商家直播时知道这是老客。

## 范围
1. 表 `customers.historical_orders(tenant_id, store_id, id, owner_id, external_order_id text 1..64, ordered_at timestamptz, status text 1..40（原样）,
   total_minor bigint ≥0, currency CHECK='TWD', items_summary text ≤ 500（「商品名×數量」拼接）, city text NULL ≤ 20, imported_batch_id,
   UNIQUE(tenant_id,store_id,external_order_id))`；每 owner ≤ 2000 行。
2. 一订单多行（SHOPLINE 一行一品项）→ 按 `external_order_id` 聚合；同单重导 → 覆盖（`updated`）。
3. 预览/提交/结果路由同 W5-02B（`kind=orders`），上限 2 MiB、20000 行、60 s；权限 `customers:privacy`；审计 `customers.orders_imported`（计数）。
4. 读：`GET …/customers/{id}/historical-orders?cursor`（`customers:read`，每页 50）。客户列表行加 `historical_orders_count`。
5. erasure：`erase_owner` 同事务删该 owner 的历史订单。

## Non-goals
回填到真实订单表、补开发票、退款历史、物流轨迹、商品/SKU 关联、报表计入。

## 合同修改
`contracts/migration-import-v1.md` Amendment「orders」：表、聚合规则、I05 隔离声明、字段白名单；`customers-billing-v1.md`：erasure 覆盖。

## SQL / 迁移 `0141_order_history_import.sql`
- 表 ENABLE+FORCE RLS，无登录角色直接 GRANT；定义者 `migrationimport.import_order_row`、`identity.read_historical_orders`：owner `commerce_privacy_writer`，
  SECURITY DEFINER，`search_path=pg_catalog`，EXECUTE 仅 `commerce_runtime`。
- `REPLACE identity.read_merchant_customers`（加计数）与 `customers.erase_owner`：ACL 不变。
- **静态断言**（测试）：`identity.read_finance_summary`、reporting 定义者、`payments.*` 函数体不引用 `historical_orders`。
- ACL 钉子：`customers_billing_schema_test.go`、`customers_billing_consent_test.go`、`worker_authority_split_test.go` WAS02。

## 写入路径
`internal/migrationimport/orders.go`（新）、`internal/customers/historical.go`（新）、`internal/httpapi/imports.go`（只加 orders 路由）、
`migrations/0141_order_history_import.sql`、`tests/foundation/order_history_import_test.go`。

## 测试（合成 fixture）
| # | 用例 | 期望 |
|---|---|---|
| OH01 | 3 单 7 行 | 3 行 historical_orders，items_summary 聚合正确 |
| OH02 | 顾客未导入 | 行失败 `customer_not_imported` |
| OH03 | 导入后 finance 汇总、W6-02B 报表、库存 | 全部不变（I05） |
| OH04 | 地址列存在 | 只存 city，完整地址不落库（查询全库文本） |
| OH05 | 同单重导 | `updated`，行数不变 |
| OH06 | erasure | 历史订单删除 |
| OH07 | 无 `customers:privacy` / 跨店读 | 403 / 404 |
| OH08 | 20001 行 | `too_many_rows` |

## 门禁
`bash scripts/dev/test-focused.sh '^TestOrderHistoryImport'`；`bash scripts/dev/test-local.sh --migration-import`；回归 `--browser-customers-billing`；
`release-gate.sh --strict --only G07`；`check-gates.sh`。

## 证据 / 角色 / 依赖
REAL_PG/MOCK（合成）。DeepSeek；K3；**Claude 终审**（I05 历史金额不进财务实收）。依赖 W5-02B 合并。

## OPEN
- OH-OPEN-1 收件地址：**推荐**只存城市（够判断老客，PII 最少）；若 owner 要完整地址用于再寄，需另定保留期并进 erasure，另开单元。
