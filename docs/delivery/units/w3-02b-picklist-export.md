# Unit W3-02B — 拣货单、承运商模板导出、超商寄件单批量建立（后端）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0130**（待 integrator 分配）。覆盖 IMPLEMENTATION-PLAN W3-02B（M14-17 批量建单与打印；M15-12；M22 #9；v5「撿貨單 / 超商寄件單 / 匯出」）。
worktree `.worktrees/w3-02b-picklist-export`（branch `unit/w3-02b-picklist-export`）。UI 另开 W3-U1b。
先读：`docs/delivery/AGENT-PREAMBLE.md` → `AGENTS.md` → `docs/delivery/PROCESS.md` → 本文件；合同只读
`contracts/manual-fulfilment-v1.md` §2–§4、`contracts/taiwan-cvs-logistics-v1.md`（建单命令与 UNKNOWN 节）、`contracts/merchant-orders-v2.md`「Response」。

## Integrator 裁决（覆盖正文）
- 一单一包裹（owner 暂定，§2.1 改为「首版一单一包裹」）；合并出货是 W3-07B 的事，本单元不建包裹表。
- 超商批量建单 = **逐单排队现有 ECPay 单笔建单命令**，不新增 ECPay 调用形状；官方「一次打印多张」未核实前，打印仍逐张（EVIDENCE_GAP）。
- 导出列只按承运商模板选列，不新增买家个人资料字段；电话沿用 `exportPhone` 现有遮罩规则。

## 目标 / owner 流程
直播结束 → 订单页勾选（或按场次）→ ①「撿貨單」：SKU 汇总数量 + 每单明细，浏览器打印；②「匯出」：选承运商模板（黑貓/新竹/郵局/通用）
下载 CSV 给物流后台；③「超商寄件單」：对勾选的超商订单一键排队建单，结果逐单回显（成功/失败码/UNKNOWN）。

## 关键事实（bb71f966，实现前再 `grep -n` 核实）
- 未出货导出：`internal/merchantorders/export.go` `ExportUnshipped`:132、`WriteUnshippedCSV`:56、`guardFormula`:90、`exportPhone`:103。
- 超商：`internal/fulfillment/cvs.go`（`NewCVS`:63，单笔建单 + River 作业 + UNKNOWN 处理已存在）；表 `fulfillment.cvs_shipments`（0072）。
- 运单回填 W3-01B 已合并为 `0126_tracking_import.sql`（`merchanttools/tracking_import.go`）——本单元不碰。

## 范围
1. **拣货单** `POST /v1/admin/stores/{store_id}/orders/pick-list`，body `{order_ids:[≤500]}` 或 `{session_id}`（二选一）；
   `orders:read`；返回 `{generated_at, orders:[{order_id, order_number, lines:[{sku_id, sku_code, title, option_label, qty}]}],
   totals:[{sku_id, sku_code, title, option_label, qty}]}`，按 sku_code 排序。只读、不落库、不改状态。
   只收 `CONFIRMED|AWAITING_COLLECTION` 且未出货的订单；其余放 `skipped:[{order_id, code: not_pickable|order_not_found}]`。
2. **承运商导出** `POST …/orders/export?template=black_cat|hsinchu|chunghwa_post|generic`（`orders:export`，审计
   `orders.carrier_export`）：同一订单集合规则；列表由 Go 常量模板定义（每模板 ≤ 15 列）；CSV 复用 `writeCSVLine`+`guardFormula`；
   UTF-8 BOM（Excel 友好）。COD 单输出代收金额列（取 `order_money_shippable` 同源字段，不重算）。
3. **超商批量建单** `POST …/shipments/cvs-batch`，`Idempotency-Key`，body `{order_ids:[≤100]}`，`fulfillment:write`：逐单调用现有单笔建单
   入口（同一 SQL 计划函数、同一 River kind），每单独立 savepoint；返回 `{results:[{order_id, outcome: queued|already|failed, code?}]}`。
   同键重放返回原结果（`command.Run`）。不新增 River kind；不并发打 ECPay（River 队列本身串行化，沿用现有速率）。

## Non-goals
不建包裹/合包（W3-07B）；不新增承运商 API（黑貓/新竹无 API，只导出）；不做异步大文件导出（≤500 单同步 30 s 内）；
不改 `ExportUnshipped` 现有形状；不做 ECPay 批量打印。

## 合同修改（integrator 冻结时写入）
- `contracts/manual-fulfilment-v1.md` 新增「Amendment W3-02B pick list and carrier export」：两个路由、模板列表、权限、500 上限。
- `contracts/taiwan-cvs-logistics-v1.md` 新增「Amendment W3-02B batch create」：批量 = 单笔命令逐单排队，结果码表，100 上限。

## SQL / 迁移 `0130_pick_list.sql`
- 只读定义者 `fulfillment.read_pick_list(p_token bytea, p_store uuid, p_orders uuid[], p_session uuid) RETURNS jsonb`：
  owner `commerce_checkout_writer`，`SECURITY DEFINER`，`search_path=pg_catalog`，`identity.resolve_access(…,'orders:read')`，
  数组 ≤ 500，别店订单当 `order_not_found`；EXECUTE **仅** `commerce_runtime`；REVOKE PUBLIC。
- 导出复用同一函数（Go 侧再要求 `orders:export`，并写审计）——不另建函数。
- 无表、无列、无角色。若实现发现批量建单需新的定义者，写 DELIVERY 由 integrator 裁决，不自行加。
- ACL 钉子：`tests/foundation/merchant_orders_v2_acl_test.go` 期望行加 `read_pick_list` EXECUTE=`commerce_runtime`；
  `tests/foundation/worker_authority_split_test.go` WAS02 确认五个 worker 登录对其无 EXECUTE（integrator 更新期望行）。

## 写入路径（唯一归属）
后端（DeepSeek）：`internal/merchantorders/picklist.go`（新）、`internal/merchantorders/carrier_export.go`（新）、
`internal/fulfillment/cvs_batch.go`（新）、`internal/httpapi/picklist.go`（新，路由）、`migrations/0130_pick_list.sql`、
`tests/foundation/pick_list_test.go`（作者冒烟）。
integrator 钩子：`internal/httpapi/handler.go` 注册一行、`scripts/dev/test-local.sh` 模式、合同 Amendment、GATES.md。

## 测试（先红后绿；红跑存 `output/w3-02b-picklist-export/red.log`）
| # | 用例 | 期望 |
|---|---|---|
| PL01 | 3 单 5 行（两单同 SKU） | totals 合并数量正确；订单明细与订单行一致 |
| PL02 | 含已出货、已取消、别店订单 | `not_pickable`/`order_not_found`，别店与不存在不可区分 |
| PL03 | 501 个 order_id | 422 `too_many` |
| PL04 | 导出各模板 | 列名/列序与模板常量一致；公式注入格被 `'` 保护；COD 列 = 应收金额 |
| PL05 | viewer 无 `orders:export` | 403；有 `orders:read` 可拣货单 |
| PL06 | 超商批量 3 单（MOCK ECPay） | 3 条现有 River 作业入列；同键重放无新作业；含宅配单 → `failed:not_cvs` |
| PL07 | 批量中一单 ECPay UNKNOWN | 该单 `queued`，之后由现有 UNKNOWN 流程处理，**不重排** |
| PL08 | 500 单拣货单计时 | < 10 s（实测写 DELIVERY） |

## 门禁
- `bash scripts/dev/test-focused.sh '^TestPickList|^TestCarrierExport|^TestCVSBatch'`
- `bash scripts/dev/test-local.sh --merchant-orders`、`--browser-cvs`、`--browser-merchant-orders-ui`（回归全绿）
- `bash scripts/dev/release-gate.sh --strict --only G07`、`bash scripts/dev/check-gates.sh`

## 证据 / 角色 / 依赖
- 证据：MOCK（REAL_PG + 假 ECPay）。ECPay SANDBOX 需 owner 测试账户，无则 NOT_RUN。
- 角色：DeepSeek V4-Pro（后端）；K3 独立门禁 `tests/foundation/pick_list_gate_test.go`；Sonnet 复核。
- 依赖：W3-01B 已合并（同 `merchantorders` 目录，避免并行冲突）。可与 W3-05B 并行。

## OPEN（附推荐）
- PL-OPEN-1 按场次选单的口径：**推荐**沿用 claim-direct-checkout 裁决 2 的场次归属（`live_price_uses ∪ order_origins`）。
- PL-OPEN-2 承运商模板列：**推荐**先按黑貓「宅急便 B2C 匯入」公开范本；拿不到官方范本 → 只出 `generic`，其余模板 BLOCKED 等 owner 给样本。
