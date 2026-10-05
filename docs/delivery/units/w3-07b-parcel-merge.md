# Unit W3-07B — 合并出货（同买家同地址多单一个包裹，不合并钱，后端）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0134**。覆盖 IMPLEMENTATION-PLAN W3-07B（M15-10 合包；v5「合併訂單提示」）。
worktree `.worktrees/w3-07b-parcel-merge`（branch `unit/w3-07b-parcel-merge`）。UI 在 W3-U4。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/manual-fulfilment-v1.md` §2–§5 与 One-parcel 段（:296-302）、
`docs/delivery/units/w3-01b-tracking-backfill.md`（同运单号多行合法）；架构 §13.2。

## Integrator 裁决（覆盖正文）
- 「合并订单」= **只合并包裹/出货**：多张订单装一个包裹、共用一个运单号。**不合并款项、不改订单、不改金额/运费、不取消重建**。
- 首版一单一包裹（owner 暂定）：一张订单最多属于一个合包组，不做一单拆多包。
- **COD（货到付款）订单不参与合包**：承运商按包裹代收一个金额，合并会改变代收金额 = 钱路，v1 不做（`409 cod_not_mergeable`）。
- 超商订单不参与（ECPay 一单一寄件单）。

## 目标 / owner 流程
订单页出现提示「王小明有 3 張待出貨訂單、地址相同，可合併出貨」→ 商家确认合并 → 拣货单/导出把它们排在一起 →
回填一个运单号（单笔或 W3-01B 批量）即三单同时「已出货」，买家各收一封出货信。商家可在出货前解除合并。

## 关键事实
- 单笔出货：`merchantorders.RecordShipment`（`shipments.go:187`）→ `fulfillment.record_manual_shipment`（0107:564），按单 CAS、审计、出货信。
- W3-01B 已允许同运单号多行（多单同包裹）。出货头表 `fulfillment.manual_shipment_heads`（0063）按单。
- 资格 `fulfillment.manual_shipment_eligible`（0107:283）。

## 范围
1. 表 `fulfillment.parcel_groups(tenant_id, store_id, id, owner_id, destination_hash bytea(32), state CHECK IN ('OPEN','SHIPPED','DISSOLVED'),
   version, created_by, created_at)` + `fulfillment.parcel_group_orders(tenant_id, store_id, group_id, order_id, PRIMARY KEY(tenant_id,store_id,order_id))`
   ——主键保证一单至多一组。组内 2..20 单。
2. 建议查询 `GET …/orders/merge-suggestions`：同 owner、同 `destination_hash`（规范化收件人+电话+地址的 sha256，服务器算）、
   均为可宅配出货、非 COD、非超商、未在组内 → 返回分组建议（只读）。
3. `POST …/parcel-groups`（`Idempotency-Key`，`{order_ids}`）建组：服务器复核同 owner / 同 destination_hash / 资格；
   `DELETE …/parcel-groups/{id}`（`expected_version`）解散（仅 OPEN）。
4. **出货**：`PUT …/parcel-groups/{id}/shipment`（同单笔 ShipmentInput）= 一个 READ COMMITTED 事务内按 order_id 排序逐单调用
   `RecordShipment`（同运单号），任一单失败整组回滚，组 → SHIPPED。单笔出货命令对组内订单返回 `409 in_parcel_group`（避免组内只出一单）——
   这需要在 `record_manual_shipment` 前加一个检查：**放在 Go 的 `RecordShipment` 调用方（W3-01B 批量与单笔路由）之前读组表**，
   不改 `record_manual_shipment` SQL。
5. 拣货单/导出（W3-02B）：组内订单相邻输出，带 `parcel_group_id` 列。

## Non-goals
合并付款、合并运费退款、跨地址合包、一单拆多包、COD 合包、超商合包、自动合并。

## 合同修改
`contracts/manual-fulfilment-v1.md`：把「One parcel per order」改为「一单至多属于一个合包组；组以一个运单号出货，出货逐单记录」；
新增「Amendment W3-07B parcel groups」：表、路由、COD/超商排除、`in_parcel_group` 码、I05（不重复计费）。

## SQL / 迁移 `0134_parcel_groups.sql`
- 两表 ENABLE+FORCE RLS，`app.tenant_id/app.store_id` 策略；`commerce_runtime` 只 SELECT。
- 定义者 `fulfillment.create_parcel_group` / `dissolve_parcel_group` / `mark_parcel_group_shipped`：owner `commerce_checkout_writer`，
  SECURITY DEFINER，`search_path=pg_catalog`，`identity.resolve_access(…,'fulfillment:write')`，订单 `FOR UPDATE` 按 id 排序（防死锁），
  EXECUTE 仅 `commerce_runtime`。`destination_hash` 在定义者内从订单 snapshot 计算，不收客户端值。
- 只读 `fulfillment.read_merge_suggestions(p_token, p_store)`（`orders:read`）。
- ACL 钉子：`tests/foundation/manual_fulfilment_schema_test.go`（MF 期望行）、`merchant_orders_v2_acl_test.go`、`worker_authority_split_test.go` WAS02。

## 写入路径
`internal/fulfillment/parcels.go`（新）、`internal/httpapi/parcels.go`（新）、`internal/merchantorders/shipments.go`（只在单笔路由前加组检查）、
`internal/merchanttools/tracking_import.go`（只加组检查一处）、`migrations/0134_parcel_groups.sql`、`tests/foundation/parcel_groups_test.go`。

## 测试
| # | 用例 | 期望 |
|---|---|---|
| PG01 | 同买家同地址 3 单 | 建议 1 组；建组成功 |
| PG02 | 地址不同 / owner 不同 / COD / 超商 | 不建议；强建 → 409 对应码 |
| PG03 | 一单已在组内再建组 | 409（主键冲突映射） |
| PG04 | 组出货 | 3 单 MERCHANT_SHIPPED，同运单号，3 封出货信，3 条审计；`payments.facts` 不变（I05） |
| PG05 | 组内一单版本漂移 | 整组回滚，0 单出货 |
| PG06 | 单笔出货 / 批量回填组内单 | `in_parcel_group` |
| PG07 | 解散后单笔出货 | 成功 |
| PG08 | 并发建两个组抢同一单 | 恰一个成功 |

## 门禁
`bash scripts/dev/test-focused.sh '^TestParcelGroup'`；回归 `bash scripts/dev/test-local.sh --merchant-orders`、`--browser-home-cod`、
`--browser-merchant-orders-ui`；`release-gate.sh --strict --only G07`；`check-gates.sh`。

## 证据 / 角色 / 依赖
证据 MOCK/REAL_PG（无外部系统）。DeepSeek；K3；**Claude 终审**（I05 不重复计费、出货原子性）。
依赖：W3-01B、W3-02B 合并。不可与 W3-08B 并行（W3-08B 的取消要读组表）。

## OPEN
- PG-OPEN-1 组内一单在出货前被取消：**推荐**取消命令（W3-08B）先把该单移出组，组剩 1 单则自动 DISSOLVED。
