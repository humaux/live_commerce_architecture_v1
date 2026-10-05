# Unit W3-08B — 商家侧最小退货（RMA）与商家取消订单（后端）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0135**。覆盖 IMPLEMENTATION-PLAN W3-08B（M15-15/16/19；M15-17 最小异常；偏差 A4/D5 的 RMA 部分）。
worktree `.worktrees/w3-08b-returns`（branch `unit/w3-08b-returns`）。UI 在 W3-U5。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/stripe-refund-v1.md` RD1–RD11、R-2、R-3；
`contracts/manual-fulfilment-v1.md` §2；`contracts/checkout-expiry-runtime-v1.md`（释放预留）；架构 §12.3、§13.4。

## Integrator 裁决（覆盖正文）
- **退货 = 商家侧最小版**（owner 暂定）：商家登记 → 收货 → 验货 → 处置（回可售 / 不可售）。无买家自助申请、无退货面单、无换货。
- 退款与退货**解耦**（I13，RD6 保持）：RMA 可关联已有退款，但不触发退款；退款也不回库存。**库存只在 RMA 处置为「可售」时回补**。
- M-8：已出货订单可直接全额退款而不走退货——**允许**（现有退款路径不变）。R-3：退款不回库存——**维持**；释放库存只经「商家取消」或 RMA 处置。
- 商家取消已付款订单：必须先全额退款（成功+在途 = 已捕获），否则 `409 refund_first`；本单元不自动发起退款。

## 目标 / owner 流程
- 退货：买家私讯说要退 → 商家在订单详情点「登記退貨」选品项与数量、原因 → 包裹寄回后点「已收到」→ 验货选「可再售 / 不可售」→
  可再售的数量回到库存；需要退钱则另点现有「退款」。
- 取消：未付款或未出货的订单，商家点「取消訂單」→ 订单 CANCELLED，预留/配货库存释放；已付款的提示先退款。

## 关键事实（bb71f966）
- 订单商业状态 CHECK 含 `CANCELLED`（0013:22），但**无任何取消函数**（grep `FUNCTION *cancel*` 为空）。未付款到期释放走 `checkout.expire_held`（最新 0096:349）。
- 库存写入唯一入口 `inventory.apply_ledger` + `guard_*_ledger` 守卫族（0018/0088 等）；I03 不得有第二个库存写手。
- 退款：`merchantorders.RequestRefundIn`（`refunds.go:134`）、`payments.refund_facts`（0062）。
- 无 returns 表、无 packages 表（偏差 A4）。

## 范围
1. **RMA 状态机**：`REGISTERED → RECEIVED → INSPECTED → CLOSED`，旁路 `CANCELLED`（REGISTERED 时可撤）。行级：
   `returns.rma_lines(order_line_id, qty_registered, qty_received, qty_restock, qty_scrap)`，约束 `qty_restock+qty_scrap = qty_received ≤ qty_registered`，
   同一订单行所有 RMA 的 `qty_registered` 合计 ≤ 已出货数量。只允许已出货（MERCHANT_SHIPPED 或 CVS 已出货）订单登记。
2. 处置（INSPECTED→CLOSED）在同一事务内调用 `inventory.apply_ledger`（新 reason `rma_restock`，新守卫 `inventory.guard_rma_ledger`），回补
   `qty_restock` 到 on_hand；`qty_scrap` 只记录。可选 `refund_id` 关联（只读校验属于同一订单）。
3. **商家取消** `POST …/orders/{id}/cancel`（`Idempotency-Key`、`expected_version`、`reason`）：
   - AWAITING_PAYMENT：有 PAYMENT_PENDING/UNKNOWN 的支付尝试 → `409 payment_in_flight`；否则释放预留（复用 expire_held 的释放分支或抽出同一 SQL 片段，不复制逻辑）→ CANCELLED。
   - CONFIRMED 且未出货：已捕获 > 0 且（成功+在途退款）< 已捕获 → `409 refund_first`；否则释放配货（新 reason `merchant_cancel`）→ CANCELLED。
     COD 未代收（AWAITING_COLLECTION）可直接取消。
   - 在合包组（W3-07B）内：先移出，组剩 1 单则 DISSOLVED。已出货 → `409 already_shipped`（走退货）。
   - 审计 `orders.merchant_cancelled`；买家通知复用 `notify` 现有 `cancelled` 类型若存在，否则不发（列入 DELIVERY，不新增信件模板）。
4. 读：`GET …/orders/{id}/returns`；列表页可筛「有退货」。

## Non-goals
买家端退货申请、退货面单、换货/补寄、部分取消、自动退款、丢件索赔、多包裹。

## 合同修改
新增 `contracts/returns-v1.md`（integrator 起草冻结，内容 = 本 brief §范围 1–4 + 状态机 + 码表）；`stripe-refund-v1.md` R-3/M-8 标记 RESOLVED
并指向 returns-v1；`manual-fulfilment-v1.md` 加取消对出货资格的影响。

## SQL / 迁移 `0135_returns_cancel.sql`
- 新 schema `returns`（REVOKE ALL FROM PUBLIC）；表 `returns.rmas`、`returns.rma_lines`，ENABLE+FORCE RLS，`commerce_runtime` 只 SELECT。
- 定义者 `returns.register/receive/inspect/close/cancel_rma`、`checkout.merchant_cancel_order`：owner 现有写角色（RMA 用 `commerce_checkout_writer`，
  库存部分经 `inventory.apply_ledger` 由 `commerce_inventory_writer` 拥有的守卫放行），SECURITY DEFINER，`search_path=pg_catalog`，
  EXECUTE 仅 `commerce_runtime`。权限：RMA 登记/收货 `fulfillment:write`；处置回库存另需 `inventory:write`；取消 `fulfillment:write`。
- ACL 钉子：`tests/foundation/manual_fulfilment_schema_test.go`、`stripe_refund_schema_test.go`（退款函数 ACL 不变）、
  `worker_authority_split_test.go` WAS02（worker 无 RMA/取消 EXECUTE）、`expiry_admission_test.go`（expire_held 若抽片段须保持 ACL）。

## 写入路径
`internal/returns/**`（新）、`internal/fulfillment/cancel.go`（新）、`internal/httpapi/returns.go`（新）、`migrations/0135_returns_cancel.sql`、
`tests/foundation/returns_test.go`。integrator：`handler.go` 注册、`contracts/returns-v1.md`、新模式。

## 测试
| # | 用例 | 期望 |
|---|---|---|
| RT01 | 已出货 2 件，登记 2 → 收 2 → 1 可售 1 报废 | on_hand +1，一条 `rma_restock` ledger，`payments.facts` 不变 |
| RT02 | 登记超过已出货数量 / 未出货订单登记 | 422 / 409 |
| RT03 | 处置无 `inventory:write` | 403，状态不变 |
| RT04 | 取消未付款单（无在途尝试） | CANCELLED，预留释放，可售量恢复 |
| RT05 | 取消有 PAYMENT_PENDING 尝试的单 | `payment_in_flight` |
| RT06 | 取消已付款未出货、未退款 | `refund_first`；全额退款 SUCCEEDED 后再取消 → 成功、配货释放 |
| RT07 | 退款本身 | 不写 ledger、不改状态（RD6 回归） |
| RT08 | 合包组内取消 | 移出组；剩 1 单组 DISSOLVED |
| RT09 | 同键重放 / 版本漂移 | 原结果 / 409 `version_changed` |
| RT10 | 并发取消 + 买家付款 capture | 恰一个赢；capture 赢则取消 `refund_first`，取消赢则 capture 进 review case（现有多收款路径） |

## 门禁
`bash scripts/dev/test-focused.sh '^TestReturns|^TestMerchantCancel'`；新模式 `bash scripts/dev/test-local.sh --returns`（integrator 加）；
回归 `--merchant-orders`、`--expiry-worker`、`--payment-worker`、`--browser-refund-fulfilment`；`release-gate.sh --strict --only G07`；`check-gates.sh`。

## 证据 / 角色 / 依赖
证据 REAL_PG/MOCK。DeepSeek；K3；**Claude 终审**（钱路与库存，§11.5 库存证据、I03、I05、I13）。
依赖：W3-07B 合并（取消读组表）。不可与 W3-07B 并行；可与 W4 单元并行。

## OPEN
- RT-OPEN-1 RT10 capture 后到的处理是否沿用现有「多收款 review case」：**推荐**是，不新增自动退款。
