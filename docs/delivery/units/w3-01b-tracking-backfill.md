# Unit W3-01B — 运单批量回填（CSV 上传 / 贴上多行 → 预览 → 幂等提交 → 标记已出货 → 买家出货信）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-05，待 integrator 审核/冻结）。Base `r3/integration` `d90d7fe3`。
迁移号占位 **0128**（**待 integrator 分配**；与 0127 meta-connection-health 无依赖）。
覆盖 `output/arch-conformance/IMPLEMENTATION-PLAN.md` W3-01B 卡片（M15-11；M14-17 回填部分；M22 #10；R5 A8）。
后端 worktree `.worktrees/w3-01b-tracking-backfill`（branch `unit/w3-01b-tracking-backfill`）；UI 另开 Codex 任务 W3-U1
（`.worktrees/w3-01b-tracking-backfill-ui`，后端 API 冻结并合入后开始）。
先读：`docs/delivery/AGENT-PREAMBLE.md`、`AGENTS.md`、`docs/delivery/PROCESS.md`、本文件；合同只读
`contracts/manual-fulfilment-v1.md` §2、§3、§4.1、§5.1、Integrator rulings；`contracts/merchant-orders-v2.md`「Response」（order_number）；
`contracts/storefront-v2.md` §E3（邮件额度）；`docs/delivery/units/home-cod.md` P1-3a/P2-4。

## Integrator 裁决（覆盖正文）

- **M-7 撤销**（integrator 2026-10-05，依 owner 全链条授权；IMPLEMENTATION-PLAN「Integrator 裁决」）：批量导入运单号不再延后，本单元可派。
  integrator 在冻结时把下文「合同修改」的 Amendment 文本写进 `manual-fulfilment-v1.md`，实现者不改合同。
- 合并订单只合并包裹不合并款项（同日裁决）→ 同一运单号可出现在多行（多张订单同一包裹），**不是**错误。

## 目标

直播爆单后，商家一次上传一份 CSV（或从试算表贴上多行）回填全部宅配运单号，不再逐单手填。每一行走与单笔「记录出货」
完全相同的命令与审计，订单变 `MERCHANT_SHIPPED`，买家收到现有的「已出货」信。失败行可下载成结果档修正后再上传。

## 关键事实（读代码核实，`d90d7fe3`）

- 单笔出货命令：`merchantorders.RecordShipment(ctx, tx, scope, token, key, orderID, ShipmentInput)`
  （`internal/merchantorders/shipments.go:187`）→ `fulfillment.record_manual_shipment`（最新定义 `migrations/0107_home_cod.sql:564`）。
  它自己做 `fulfillment:write` 鉴权、幂等（`ops.command_results`，operation `fulfillment.manual_shipment.record`）、order `FOR UPDATE`、
  `expected_version` CAS、MD6 资格、审计 `fulfillment.shipment_recorded`。**只接受 READ COMMITTED**。
- 承运商码（0107:594）：`seven_eleven_cvs, familymart_cvs, hilife_cvs, okmart_cvs, sf_express, chunghwa_post, black_cat, hsinchu, other`；
  `other` 必须带 `carrier_name`。黑貓=`black_cat`、新竹=`hsinchu`、郵局=`chunghwa_post` **都已存在，迁移不需要改承运商 CHECK**。
- 资格 `fulfillment.manual_shipment_eligible`（0107:283）：`CONFIRMED|AWAITING_COLLECTION` + `MANUAL_UNASSIGNED` + `order_money_shippable`
  + 无进行中的 ECPay 尝试。**它不排除「还没建 ECPay 单的超商订单」**——所以本单元必须自己挡超商订单（见规则 4）。
- 买家出货信：`notify.on_order_update` 在 `fulfillment_state → MERCHANT_SHIPPED` 时同事务入列 `shipped`（0090:128），信里带
  carrier/tracking（0090:237）。**本单元不碰通知代码**，回填成功即自动寄信。
- 邮件额度：每店每滚动小时 30 封买家信（`internal/notify/worker.go:32`），PENDING 超过 24 h 变 `SKIPPED stale`（0090 claim_batch）。
  一次回填 N 单 ≈ ⌈N/30⌉ 小时寄完 → 单档上限 500 行（≈17 h < 24 h）。
- CSV 先例：`internal/merchanttools/csvimport.go`（预览 = 同一 apply 代码整笔回滚；提交 = `command.Run` 文件哈希幂等；每行 savepoint）、
  `csvfile.go`（`MaxCSVBytes = 2 MiB`，UTF-8 + 可选 BOM，RFC 4180，`guardCell/unguardCell` 防公式注入）。HTTP 先例
  `internal/httpapi/merchanttools.go:58`（`/products/import/{preview,commit}`，`text/csv`，60 s `WithScopeBudget`）。
- 订单号：v2 列表的 `order_number = 'LC-' || upper(去掉连字号的 uuid)`（0110:167）；未出货导出 `unshipped.csv` 第一列是 `order_id`（uuid）。
- `merchanttools` 已 import `merchantorders`（无循环）。`internal/httpapi/shipments.go` 头注释写着「no bulk tracking import (M-7)」，需更新。

## 范围

### 1. 输入格式（服务端只收 CSV；「贴上」由 UI 转成同样的 CSV 字节）
- `Content-Type: text/csv`，≤ 2 MiB，UTF-8（可带 BOM），逗号分隔，RFC 4180 引号；非 UTF-8 → 文件级错误 `encoding_not_utf8`
  （台湾 Excel 默认 Big5，UI 提示「另存為 CSV UTF-8」或改用贴上）。
- 第一行是表头，按名称识别列（大小写/全半形/前后空白不敏感），**未知列忽略**（所以商家可以直接在 `unshipped.csv` 后面加两列再上传）：

| 列 | 别名 | 必填 | 规则 |
|---|---|---|---|
| `order_number` | `order_id`、`訂單編號`、`订单编号` | 是 | `LC-`+32 位十六进制，或标准 uuid；规范化成 uuid |
| `carrier` | `物流商`、`承運商`、`承运商` | 是 | 码或别名：`black_cat`/黑貓/黑猫/宅急便/統一速達；`hsinchu`/新竹/新竹物流；`chunghwa_post`/郵局/邮局/中華郵政；`other`/其他 |
| `tracking_number` | `運單號碼`、`运单号`、`託運單號` | 是 | manual-fulfilment §3.2（`^[A-Za-z0-9][A-Za-z0-9 -]{0,63}$`，无尾空格，原样存） |
| `carrier_name` | `物流商名稱` | `other` 时必填 | §3.1（1..80，NFC，无控制字元） |
| `tracking_url` | `追蹤網址` | 否 | §3.2 / M-2（https，Go 规范化，SQL 复核） |

- 每格先 `unguardCell`；数据行 ≤ **500**（超过 → 文件级 `too_many_rows`，不处理任何行）；空行跳过不计。

### 2. 行规则（预览与提交跑同一份代码）
1. 格式错误 → `required` / `invalid_order_ref` / `invalid_carrier` / `carrier_name_required` / `invalid_tracking` / `invalid_url`。
2. 同一订单在档内出现两次 → 两行都 `duplicate_order`（不猜哪行对）。同一运单号多行 → 允许（合并包裹裁决）。
3. 订单不存在或属于别店 → `order_not_found`（两者不可区分，I01）。
4. **超商订单一律拒绝** `cvs_order`：订单冻结目的地 `snapshot#>>'{destination,pickup,kind}'` 是任一超商 kind，**或**该单存在任何
   `fulfillment.cvs_shipments` 行（任何状态）。ECPay 超商单走自己的寄件单流程；手动超商（交貨便）仍可用单笔 PUT。本单元只做宅配。
5. 已有 SHIPPED head：carrier_code + tracking_number（+ carrier_name、tracking_url）与本行相同 → 结果 `unchanged`（不算失败、不写入）；
   不同 → `already_shipped`（**不覆盖**；改号请走单笔「更正」）。
6. 其余交给 `RecordShipment`（`status=SHIPPED`，`expected_version` = head 版本或 0，`note=null`）：它返回的
   `not_shippable` / `version_changed` / `invalid_*` 原样记为该行的码。

### 3. 预览 → 提交
- **预览** `POST /v1/admin/stores/{store_id}/shipments/tracking-import/preview`：一个 `WithScopeBudget(60 s)` READ COMMITTED 事务，
  逐行 savepoint 真跑 `RecordShipment`，最后整笔回滚。回 `{file_sha256, rows_total, apply_rows, unchanged_rows, failed_rows,
  rows:[{row, order_number, carrier_code, tracking_number, outcome: apply|unchanged|failed, code?}], mail_eta_hours}`。
  `mail_eta_hours = ceil(apply_rows/30)`（只是提示）。
- **提交** `POST …/tracking-import/commit?expected_apply_rows=N`（同一份 CSV 字节）：同一事务里重跑；失败行**跳过**并记录；
  - 实际可套用行数 ≠ N → 整笔回滚，`409 preview_stale`（附新预览），**一行都不写**；
  - 可套用行数 = 0 → `422 nothing_to_apply`，不建批次；
  - 否则提交：所有可套用行写入、一行批次记录、`200 {batch_id, applied, unchanged, failed, replayed}`。
- **幂等**：整批 `command.Run(operation "fulfillment.tracking_import", key "trk-"+sha256[:32], request {sha256, expected_apply_rows})`；
  同档同 N 再提交 → 回放第一次的摘要（`replayed:true`），不重写不重寄信；同档不同 N → `409 idempotency_conflict`。
  每行 `RecordShipment` 的 key = `trk-<sha256[:24]>-r<row>`（合 `^[A-Za-z0-9_.:-]{8,128}$`），请求哈希沿用其 v1 规则。
  另一份**不同**的档里含已回填的单 → 规则 5（`unchanged` 或 `already_shipped`），永远不会二次出货、二次寄信。
- **结果档** `GET …/tracking-import/{batch_id}/result.csv`：每行 `row, order_number, carrier, tracking_number, outcome, code`
  （`guardCell`，UTF-8 BOM 方便 Excel）；`?only=failed` 只给失败行。没有收件人、电话、地址。

### 4. 权限
三条路由都要 `fulfillment:write`；结果档另需 `orders:read`。`fulfilment` 角色（0089:112）两者都有。tenant/store 只从 bearer 解析（I01）。

## Out of scope
超商（ECPay 或手动）订单；承运商 API / 追踪轮询 / 内建追踪网址模板（M-2 不变）；改号与作废（用单笔 PUT）；拣货单与导出（W3-02B）；
Big5 解码；改邮件额度或通知模板；任何 UI（W3-U1）。

## 合同修改（integrator 冻结时写入，实现者不改）

`manual-fulfilment-v1.md` 追加：

> **Amendment: M-7 revoked (integrator 2026-10-05, unit W3-01B).** Bulk tracking backfill for **home-delivery** orders:
> `POST …/shipments/tracking-import/{preview,commit}` and `GET …/tracking-import/{batch_id}/result.csv` (`fulfillment:write`;
> result also `orders:read`). CSV columns `order_number, carrier, tracking_number` (+ optional `carrier_name`, `tracking_url`;
> unknown columns ignored), ≤ 2 MiB, ≤ 500 rows, UTF-8. Every applied row goes through `fulfillment.record_manual_shipment`
> unchanged (same auth, CAS, MD6, audit, shipped mail). Rows for CVS-destination orders or orders with any ECPay attempt are
> refused (`cvs_order`); an existing SHIPPED head is never overwritten. Invalid rows are skipped and reported; a commit whose
> applicable set differs from the preview's count commits nothing (`409 preview_stale`). Idempotent per file hash. §8 "no
> tracking import (M-7)" no longer applies.

`shipments.go` 头注释的「no bulk tracking import (M-7)」改为指向本 Amendment（只改注释）。

## 数据 / 迁移 `0128_tracking_import.sql`（编号待 integrator 分配）

- 表 `fulfillment.tracking_import_batches(tenant_id, store_id, id uuid, file_sha256 bytea CHECK(octet_length=32), principal_id uuid,
  rows_total smallint, applied smallint, unchanged smallint, failed smallint, results jsonb CHECK(jsonb_typeof='array' AND jsonb_array_length ≤ 500),
  created_at, PRIMARY KEY(tenant_id,store_id,id), UNIQUE(tenant_id,store_id,file_sha256))`；FORCE RLS；
  `commerce_runtime` SELECT/INSERT/DELETE，策略 `app.tenant_id`/`app.store_id` GUC（与 catalog 表同式）。`results` 每项只含
  `{row, order_id, carrier_code, carrier_name?, tracking_number, outcome, code?}`。提交时顺手删除本店 30 天前的批次（≤ 50 行，I23）。
- 预检定义者 `fulfillment.tracking_import_precheck(p_hash bytea, p_store uuid, p_orders uuid[]) RETURNS jsonb`（owner
  `commerce_checkout_writer`，EXECUTE `commerce_runtime`，`identity.resolve_access(…,'fulfillment:write')`，数组 ≤ 500）：每单回
  `{order_id, found, cvs (规则 4), head_version, head_status, carrier_code, carrier_name, tracking_number, tracking_url}`；
  别店/不存在 → `found:false`。不加锁（锁与最终判断由 `record_manual_shipment` 负责；预检只给规则 4/5 和 `expected_version`）。
- 不改 `record_manual_shipment`、承运商 CHECK、通知触发器；不新增角色、River kind 或队列。精确权限差异写进 DELIVERY.md，integrator 更新
  MF/KC03 期望行。

## 不变量
I01（店铺只从认证来）、I02（同 key 同请求；同档不同 N → 409）、I06（无外部动作；出货信由现有 outbox 一次性入列）、I11（日志不写运单号、
订单号以外的内容和请求体；结果档无 PII）、I14（每行 CAS = head 版本；订单行锁在 `record_manual_shipment` 内）、I23（2 MiB / 500 行 /
60 s / 批次 30 天）。

## 写入路径（唯一归属）
后端（DeepSeek V4-Pro）：`internal/merchanttools/tracking_import.go`（新；放 merchanttools 而非卡片写的 merchantorders，因为 CSV 解析、
`guardCell`、`MaxCSVBytes`、`ErrPreviewRolledBack` 都在这里且已依赖 merchantorders）、`internal/httpapi/merchanttools.go`（只加三条路由）、
`internal/httpapi/shipments.go`（只改头注释）、`internal/httperror` 码表（新码 `preview_stale`、`nothing_to_apply`）、
`migrations/0128_tracking_import.sql`、作者冒烟 `tests/foundation/tracking_import_test.go`。
UI（Codex，W3-U1）：订单页工具列「批次回填運單」→ 对话框（上传 CSV / 贴上两个分页；范本下载 = 前端常量表头；预览表格按失败优先；
确认按钮写明「將出貨 N 筆、約 H 小時內寄出出貨通知」；结果页下载失败行）；`apps/admin/components/TrackingImport.tsx`、
`apps/admin/lib/tracking-import-*.ts`、BFF allowlist、zh-TW/zh-CN/en。贴上：按 Tab 或逗号切分、补表头、转成 RFC 4180 CSV 字节后走同一 API。
独立测试（Kimi K3，先于读 diff 写）：`tests/foundation/tracking_import_gate_test.go`、`tests/admin/tracking-import.spec.ts`。
integrator：`contracts/manual-fulfilment-v1.md` Amendment、`docs/delivery/GATES.md`、迁移号、MF/KC03 期望行。

## 执行角色
后端 DeepSeek V4-Pro；`apps/` 只 Codex；K3 对抗测试；Claude 终审（只看出货写路径与超商排除）。

## 门禁模式
- **`--tracking-import`**（新，REAL_PG；`scripts/dev/test-local.sh` 加模式，跑 `TestTrackingImport*`）。
- **`--browser-tracking-import`**（新，BROWSER，MOCK 后端；W3-U1 交付，真实点击 + 点击台账）。
- 回归：`--merchant-orders`、`--browser-refund-fulfilment`、`--browser-home-cod`、`--expiry-worker`（出货信）保持全绿。
- `release-gate.sh --strict --only G07`（迁移 + GRANT），`bash scripts/dev/check-gates.sh`（含头注释棘轮）。

## 测试计划（先红后绿；红跑存 `output/w3-01b-tracking-backfill/red.log`）
| # | 用例 | 期望 |
|---|---|---|
| TI01 | 3 行宅配合法（黑貓/新竹/郵局各一）预览后提交 | 3 单 `MERCHANT_SHIPPED`，3 条审计 `fulfillment.shipment_recorded`，3 条 `notify.outbox` `shipped`，1 个批次 |
| TI02 | 同档同 N 再提交 | `replayed:true`，无新版本、无新 outbox、无新批次 |
| TI03 | 同档不同 N | 409 `idempotency_conflict` |
| TI04 | 预览后另一请求把其中一单单笔出货，再提交原 N | 409 `preview_stale`，其余行**都没写** |
| TI05 | 10 行含 2 行非法（格式 + 别店订单） | 提交 N=8 → 8 行写入，2 行在结果档，`order_not_found` 与不存在单不可区分 |
| TI06 | 超商订单（有 ECPay CREATED 尝试；无尝试但目的地 cvs_711；手动超商目的地） | 全部 `cvs_order`，订单不变 |
| TI07 | 已出货同值 / 异值 | `unchanged` / `already_shipped`，head 版本不变 |
| TI08 | 档内同单两次；两单同运单号 | 前者两行 `duplicate_order`；后者两单都出货 |
| TI09 | `other` 无 carrier_name；`tracking_url` 为 http / IP 主机 | `carrier_name_required`；`invalid_url` |
| TI10 | 501 行；2 MiB+1；Big5；缺 `tracking_number` 列；`unshipped.csv` 加两列 | `too_many_rows`；413；`encoding_not_utf8`；文件级 `required`；正常处理（未知列忽略） |
| TI11 | COD 订单（AWAITING_COLLECTION）回填 | 出货成功、出货信含代收金额；之后作废受 P1-3a 规则约束（不在本单元改） |
| TI12 | 无 `fulfillment:write`（viewer、live_operator）；跨租户 | 403；404 |
| TI13 | 结果档 | 公式注入格被 `'` 保护；无收件人/电话/地址；`?only=failed` |
| TI14 | 500 行计时 | 预览、提交各 < 30 s（记录实测值进 DELIVERY.md；超过则降上限并报告，不放宽预算） |
| TI15 | 日志/审计扫描 | 请求体、运单号不进访问日志 |

## OPEN（每项附推荐答案，integrator 冻结时裁决）
- **T-OPEN-1 部分套用还是整批不提交？** 计划卡写「部分非法行导致整批不提交」，任务要求「失败行结果档」。**推荐：跳过非法行、套用合法行**，
  但以 `expected_apply_rows` 钉住预览结果——预览后状态漂移则整批不提交（TI04）。理由：直播后 300 单里 3 行打错不该卡住 297 单出货；
  非法行永远不会被写入，计划门禁的本意（不写坏数据）不变。
- **T-OPEN-2 是否允许超商订单？** **推荐：不允许**（规则 4，只做宅配）。ECPay 单有自己的寄件单；手动超商量小，用单笔 PUT。
- **T-OPEN-3 承运商范围？** **推荐：只开 黑貓、新竹、郵局、其他**（任务点名的四个）；`sf_express` 先不开（可用「其他 + 順豐」），有需求再加别名，
  不需迁移。
- **T-OPEN-4 行数上限？** **推荐：500**（邮件 30 封/小时/店 × 24 h 过期 → 500 封约 17 h 寄完；也远低于 60 s 预算）。大场分多档上传；
  若单日要寄 > 720 封出货信，属 notify 额度问题，另开单元调 `storeHourly`，本单元不改。
- **T-OPEN-5 已出货不同值要不要「覆盖」选项？** **推荐：不要**。改号走单笔更正（MD7，有版本与审计），批次只做首次回填。
- **T-OPEN-6 订单号格式？** **推荐：只收 `LC-`+32 hex 与 uuid**；不收信件里的 12 位短号（`XXXX-XXXX-XXXX` 可能碰撞）。
- **T-OPEN-7 批次记录保留多久？** **推荐：30 天**，提交时惰性清理；结果档过期后商家重新上传即可（幂等回放或 `unchanged`）。
- **T-OPEN-8 Big5？** **推荐：不解码**（多一套编码依赖与误判风险），UI 引导用 UTF-8 或贴上。

## 交付
`output/w3-01b-tracking-backfill/DELIVERY.md`（PREAMBLE §5 模板）：迁移号待分配、精确权限差异、TI14 实测、NOT_RUN（BROWSER 归 W3-U1）。
证据等级：MOCK/REAL_PG 即可完成，无外部系统。
