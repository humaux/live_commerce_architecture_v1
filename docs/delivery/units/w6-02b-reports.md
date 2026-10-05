# Unit W6-02B — 商品报表、渠道报表、全店漏斗、手工单来源（后端，只读）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0142**。覆盖 IMPLEMENTATION-PLAN W6-02B（M18 #2、#4、#5、#6、#11）。worktree `.worktrees/w6-02b-reports`。UI 在 W6-U1。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；`internal/reporting/doc.go`（口径注释）；合同只读 `contracts/customers-billing-v1.md` BD7/Q11（财务汇总口径）、
`contracts/live-keyword-claims-v1.md` §6（认领事件）、`contracts/meta-ads-v1.md` 报表三口径段；架构 §15.5。

## Integrator 裁决（覆盖正文）
- **只读**：不建汇总表、不建缓存、不建物化视图（与 `internal/reporting` 现有原则一致）；全部在定义者里按请求实时算，范围 ≤ 92 天（Asia/Taipei 日）。
- 钱只来自 `payments.facts` / `payments.refund_facts`（与 finance 汇总同源同口径）；线下款（COD 代收、银行转账确认）单列，不并入 captured（ops-polish OP3 口径）。
- 历史导入订单（W5-03B）**不计入**任何报表。客服报表（M18 #11 后半）**不在本单元**——等 LC-B3/LC-B4 数据稳定后另开。
- 利润报表保留在 W6 但**不在本批**（owner 未解除 M18 #8 延后）。

## 目标 / owner 流程
商家在「報表」选日期区间：①商品：每 SKU 售出件数、实收、退款、净额；②渠道：FB 直播 / IG 直播 / 网店 / 手工单（代建单）的订单数与净额；
③漏斗：认领 → 链接已发 → 已建单 → 已付款（按场次或全店）；④手工单来源：代建单由哪个员工、哪个场次建立。可导出 CSV。

## 关键事实（bb71f966）
- `internal/reporting/finance.go`（`Finance`:95、`FinanceCSV`:105、`ParseRange`:79）；定义者 `identity.read_finance_summary` / `export_finance_summary`（0078，EXECUTE `commerce_runtime`，
  `orders:read`；导出另需 `orders:export` 并写审计 `finance.exported`）——**照此模式**。
- 订单来源：`claims.order_origins`（0113）、`claims.live_price_uses`（0105）；手工/代建单来源字段由 LC-B6 写入（实现前 `grep` 其列名；LC-B6 未合并则 ④ BLOCKED）。

## 范围
1. 定义者（全部 `identity.read_report_*(p_token bytea, p_store uuid, p_from date, p_to date, …) RETURNS jsonb`）：
   `read_report_products`（≤ 1000 SKU，按净额降序）、`read_report_channels`、`read_report_funnel(p_session uuid NULL)`、`read_report_manual_orders`。
   每个有 `export_report_*` 版本（`orders:export` + 审计 `reports.exported`）。
2. 渠道归类规则（写进合同，测试钉住）：有 `order_origins` 且 platform=facebook → FB 直播；instagram → IG 直播；代建单 → 手工单；其余 → 网店。一单只归一类。
3. 漏斗定义：认领 = ACCEPTED claim 行数（按 bundle 去重）；链接已发 = 有 SUCCEEDED 链接操作的 bundle；已建单 = 有 `order_origins` 的 bundle；已付款 = 其订单有 CAPTURED fact 或线下款确认。
4. Go：`internal/reporting/{products.go,channels.go,funnel.go,manual.go}`（新），严格解码（精确键）、CSV 复用 `CSV` 风格与公式防护。
5. HTTP：`GET /v1/admin/stores/{store_id}/reports/{products|channels|funnel|manual-orders}?from&to[&session_id]`，`.csv` 变体。60 s 预算。

## Non-goals
利润/成本（W6-03B 延后）、客服报表、广告 ROAS（ads 已有）、跨币种、缓存、定时邮件报表、历史导入订单。

## 合同修改
新增 `contracts/reporting-v2.md`（integrator 冻结）：四个报表的字段、口径、渠道归类、漏斗定义、上限、权限、I05 声明；引用 finance 汇总为同源。

## SQL / 迁移 `0142_reports.sql`
- 只有定义者 + `COMMENT ON`；无表、无列、无角色。owner 与 `read_finance_summary` 相同（`commerce_auth`，实现前确认），SECURITY DEFINER，`search_path=pg_catalog`，
  `STABLE`，EXECUTE 仅 `commerce_runtime`，REVOKE PUBLIC。导出版写审计经现有 `auth_finance_export_audit` 同类策略（新增 `reports.exported` 动作的 INSERT 策略）。
- ACL 钉子：`customers_billing_schema_test.go`（finance/报表函数清单）、`ads_attribution_r12_test.go`（order_origins 读取 ACL 不变）、`worker_authority_split_test.go` WAS02。

## 测试
| # | 用例 | 期望 |
|---|---|---|
| RP01 | 合成数据：2 SKU、3 渠道、1 退款 | 每个数字与手算一致；净额总和 = finance 汇总同区间 net |
| RP02 | COD 代收与银行转账 | 单列，不进 captured/net |
| RP03 | W5-03B 历史订单存在 | 不出现在任何报表 |
| RP04 | 漏斗按场次 | 四级计数正确，单调不增 |
| RP05 | 93 天区间 / from>to | 422 |
| RP06 | 无 `orders:export` 导出 | 403；有则审计 1 行 |
| RP07 | 跨店 | 404，数字不泄露 |
| RP08 | 92 天 × 2000 单计时 | < 5 s（实测写 DELIVERY） |

## 门禁
`bash scripts/dev/test-focused.sh '^TestReport'`；新模式 `bash scripts/dev/test-local.sh --reports`（integrator 加）；回归 `--browser-customers-billing`（finance 页）、`--browser-ads-attribution`；
`release-gate.sh --strict --only G07`；`check-gates.sh`。

## 证据 / 角色 / 依赖
REAL_PG/MOCK。DeepSeek；K3（独立手算对照）；Sonnet 复核口径。依赖：LC-B6 合并（手工单来源列；未合并则 ④ 延后、①–③ 可先做）。可与 W5/W3 并行（只读）。
