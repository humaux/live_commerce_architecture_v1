<!-- Purpose: owner-facing release-candidate inventory, CI evidence boundaries, outstanding work and launch decisions.
Depends on: the pinned R1 evidence and R2 r3/integration source snapshots, linked GitHub PRs/CI runs/issues, frozen contracts, GATES.md and the integrator FOLLOWUPS record.
Used by: owner and integrator when reviewing the release candidate; this report neither authorizes production nor replaces the release gate. -->
# 交付报告：r3/integration 发布候选

**R3 修订日期：2026-10-14**（本周发布候选更新，见下方「R3 本周变更」；以下 R2 段落保留作历史基线）。R2 修订日期：2026-10-14（北京时间）。本次文档核查基于 [`228a1c877f63b2be0955060d03ed621a05f4bdcf`](https://github.com/humaux/live_commerce_architecture_v1/commit/228a1c877f63b2be0955060d03ed621a05f4bdcf)，即合入 [PR #49](https://github.com/humaux/live_commerce_architecture_v1/pull/49) 后的 `r3/integration`。R1 的运行证据仍固定在 [#28 合并候选 `0bfe36e5`](https://github.com/humaux/live_commerce_architecture_v1/commit/0bfe36e53acb694f4697521202e24f0790638941) 及下表各自的实际测试树；**R2 的代码/契约核查不把旧 CI 提升为当前快照的运行验收**。后续提交须另补证据。**这是开发交付盘点；完整 release-gate CI 结果尚未加入，不能据此批准上线。** [R2 作者自述记录（来源索引与未跑项，不是独立验收）](../../output/delivery-report-r2/DELIVERY.md)、[验收定义](PROCESS.md#4-evidence-labels-agentsmd)、[发布检查清单](../runbooks/deploy.md#10-发布检查清单)。

## R3 本周变更

以下为 R2 之后（集成方已核对）合入 `r3/integration` 的 PR，以及尚待合并的 PR。本节只列变更，不提升任何证据等级。

| PR | 内容 | 状态 |
| --- | --- | --- |
| [#53](https://github.com/humaux/live_commerce_architecture_v1/pull/53) | OA-1 原子下单核心与数据库 key 退役；**默认关闭**（`LC_ORDER_ATTEMPT_ENABLED=0`） | 已合入 |
| [#56](https://github.com/humaux/live_commerce_architecture_v1/pull/56) | 测试 setup 连接池释放与浏览器 PG 容量守卫 | 已合入 |
| [#57](https://github.com/humaux/live_commerce_architecture_v1/pull/57) | 不稳定测试修复（密码 rate-limit、评论流、媒体恢复） | 已合入 |
| [#60](https://github.com/humaux/live_commerce_architecture_v1/pull/60) | card-payments 买家半程连接池收敛（释放 admin 半程连接池以满足 PG 容量守卫） | 已合入 |
| [#58](https://github.com/humaux/live_commerce_architecture_v1/pull/58) | 公司官网 `/about`（香港大碗貿易有限公司为平台主体） | **待合并**（集成方合并后更新） |
| [#59](https://github.com/humaux/live_commerce_architecture_v1/pull/59) | PAY-RM2 第一阶段：从商家后台/店面/部署/文档移除 PAYUNi | **待合并**（集成方合并后更新） |
| [#61](https://github.com/humaux/live_commerce_architecture_v1/pull/61) | CI Go 模块预取重试 | **待合并**（集成方合并后更新） |

注意：#53 合入的新下单路径默认关闭，下文 R2 时间快照中「OA-1 未合入」的描述已被取代，但开启路径的 SANDBOX/LIVE 与运行验收仍为 **NOT_RUN**。#58/#59/#61 未合入前不得计入候选。

## 证据怎么读

本报告只使用 GitHub CI 运行结果；本地日志、截图和作者自报 PASS 不计入验收。`MOCK` 表示合成数据、假身份或假供应商，可能同时运行真实 Next/Go/PostgreSQL；`SANDBOX` 须有真实供应商测试环境的收据；`LIVE` 须有获授权的真实平台收据；`NOT_RUN` 表示本报告未取得该范围的有效证据。**浏览器模式名、契约中的目标等级、数据库实测和配置成功，都不能把 MOCK 提升为 SANDBOX/LIVE。** [PROCESS §4](PROCESS.md#4-evidence-labels-agentsmd)、[GATES：Pull-request gates 的供应商例外](GATES.md)。

| 编号 | CI 运行及实际范围 | 结果与限制 |
| --- | --- | --- |
| C1 | [38357392862](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38357392862)，R1 候选 SHA `0bfe36e5`，push；`shard:unit`、`g01`–`g10` 和 `required` | SUCCESS。证明该 R1 候选的所选 Go/真实 PG 测试，不覆盖 R2 后续代码；此 push **没有浏览器任务**，deploy 和 aggregate 是 SKIPPED，不能算部署或浏览器 PASS。 |
| C2 | [38338136795](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795)，合并前祖先 SHA `5a54d41b`，schedule；下表列出的浏览器模式、click-sweep ×10、visual-lint ×4、aggregate、required | SUCCESS，**合并前祖先证据，不是候选 SHA 的完整重跑**。该 [C2 SHA 的 gates.yml](https://github.com/humaux/live_commerce_architecture_v1/blob/5a54d41b7e86a72eeec2afbe1f7b0458daaf742b/.github/workflows/gates.yml#L9-L18) 未配置真实供应商凭据注入；这不是断言仓库或环境中不存在任何秘密。该 schedule **未执行 `--stripe-browser`**；需要 Stripe 测试 key 的 SANDBOX 步骤为 **NOT_RUN**，绿色 MOCK 模式不证明 SANDBOX。它也不是 release-gate。 |
| C3 | [38352001282](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38352001282)，[PR #28](https://github.com/humaux/live_commerce_architecture_v1/pull/28) head `3e97bd49`；`--browser-product-editor`、`--browser-catalog-core`、required；[Foundation 38352001264](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38352001264) | SUCCESS，**合并前 PR-head 证据**。新增商品矩阵台账检查通过于 `3e97bd49`，不等于合并 SHA `0bfe36e5` 的浏览器 PASS；该合并 SHA 的商品编辑器/矩阵台账浏览器验收 **NOT_RUN（本报告）**。完整性还受 [issue #45](https://github.com/humaux/live_commerce_architecture_v1/issues/45) 限制。 |
| 注 | `pull_request` 事件的 Foundation/Gates 使用不带 `ref` 的 `actions/checkout`，实际测试的是 GitHub 合成的 PR merge ref（head 合入当时的 base），不是 head 提交本身。C3 的 SUCCESS 归属于 `3e97bd49` 当时的合成 merge 树；该 merge SHA 未记录，只能作为近似证据。C1 为 push 事件，检出即候选 SHA `0bfe36e5`。 | 证据归属限制 |
| RC1 | [Gates 38547370204](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38547370204)（workflow_dispatch，full browser universe），SHA [`625544483a2e5a33530ec256aabeee64b0878690`](https://github.com/humaux/live_commerce_architecture_v1/commit/625544483a2e5a33530ec256aabeee64b0878690)；[Deploy smoke 38547373998](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38547373998) | Gates **SUCCESS**：78 个 job 通过 / 0 失败 / 1 个 skipped（deploy）；Deploy smoke **SUCCESS**。**MOCK 范围**，SHA 是 #53 合入点（含 #56/#57），不含其后合入的 #60，也不含待合并的 #58/#59/#61。同一 SHA 的 Foundation **NOT_RUN**（`foundation.yml` 无 dispatch，仅 PR 触发）。 |
| RCG | 最终候选 SHA（#58/#59/#61 合入后）的完整 release-gate CI run ID、各 tier 结果、artifact、独立验收 | **待集成方填写**；当前 **NOT_RUN**。不得用 C1/C2/C3/RC1 替代。供应商 SANDBOX/LIVE 结果分别填入，**SANDBOX/LIVE 各 tier 均 NOT_RUN**。 |

下表的 `C2 / --browser-…` 指 C2 中该实际成功的 job；证据等级适用于所引用 CI 的 SHA，不能自动转移到合并后的候选 SHA。C2 早于 #28，不证明 #28 的新增矩阵台账；C3 是合并前 PR-head 证据。单元或 PR 说明交付归属，契约说明能力边界。旧契约的 DESIGN/NOT_RUN 标题不覆盖新的 CI 结果；旧 CI 的成功也不覆盖未来版本。模式的定义及限制见 [GATES](GATES.md)；缺 key 时的 Stripe 排除条件见 [C2 SHA 的 EXCLUDED_MODES](https://github.com/humaux/live_commerce_architecture_v1/blob/5a54d41b7e86a72eeec2afbe1f7b0458daaf742b/scripts/dev/pr-modes.mjs)。

**2026-10-13 19:31 UTC 重新查询 C2**：run `38338136795` attempt1、head `5a54d41b7e86a72eeec2afbe1f7b0458daaf742b`、SUCCESS；[staff/password-auth job115102886582](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795/job/115102886582)、[platform-site job115102886936](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795/job/115102886936)、[design job115102886321](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795/job/115102886321) 均 SUCCESS。刷新查询不改变这些收据的历史 SHA 归属，也不证明当前候选或真实买家 preview 已通过。

C2 的供应商子项另有明确日志边界：[RF11(b) 退款浏览器 job](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795/job/115102886304)、[RF10 所在 g06](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795/job/115102886740) 和 [CB10 所在 g04](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795/job/115102886923) 均记录凭据/flag 依赖的 SANDBOX 子测试 **NOT_RUN**。这些不是 `--stripe-browser` 的执行收据；整个 schedule 没有该模式的 job，整体 SUCCESS 不能代替这些缺失验收。

**T3 文档勘误（保留 CI 边界）**：[GATES 的 T3 行及其后两句](GATES.md) 仍写“NOT_RUN in CI／仅 release-gate 或开发者调用”，是陈旧描述；同文件的 Pull-request gates 节与 [当前 gates.yml](../../.github/workflows/gates.yml) 已定义 PR 按变更选择浏览器、nightly 全量浏览器、workflow_dispatch 门禁。C2 上述实际 job 也证明浏览器已在 CI 运行。普通 trunk push 不选择浏览器，供应商凭据例外仍排除 `--stripe-browser`；不能把“支持 CI”写成每个 push 或当前 SHA 全量浏览器已通过。本轮仅标明此勘误，未改不在授权路径内的 GATES.md。

## 业务能力清单

### 商品与库存（catalog）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 商品新增/编辑、草稿/上架、slug/SEO、规格矩阵、批量 SKU、追踪或不追踪库存 | catalog-core、product-editor；[#16](https://github.com/humaux/live_commerce_architecture_v1/pull/16)、[#28](https://github.com/humaux/live_commerce_architecture_v1/pull/28)；[storefront-v2 §A](../../contracts/storefront-v2.md)、[catalog-inventory A6](../../contracts/catalog-inventory-v1.md) | **MOCK（合并前证据）**：C2 / `--browser-catalog-core`、`--browser-product-editor` 是祖先版本的既有编辑能力；新增矩阵台账使用 C3 **合并前 PR-head** `3e97bd49` 证据。合并 SHA `0bfe36e5` 的这两项浏览器验收 **NOT_RUN（本报告）**。issue #45 未关闭，不能宣称每个规格控件已完整点击。 |
| 主图、规格图、详情图、上传缩图、排序，以及商品/购物车图片展示 | catalog-media、[product-media-v2（PM-B/PM-U）](units/product-media-v2.md)；[catalog-inventory Product-media-v2 amendment](../../contracts/catalog-inventory-v1.md) | **MOCK**：C2 / `--browser-catalog-media`、`--browser-product-media-v2`，真实文件上传与应用/PG 回读。 |

### 订单（orders）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 订单列表/筛选/详情、店铺权限、状态与买家记录 | merchant-orders、merchant-orders-ui；[merchant-orders-v2](../../contracts/merchant-orders-v2.md)、[merchant-orders-ui-v1](../../contracts/merchant-orders-ui-v1.md) | **MOCK**：C2 / `--browser-merchant-orders-bff`、`--browser-merchant-orders-ui`、`--browser-merchant-buyer`；C1 的相关 PG 测试。 |
| 手工建单、复制一次性付款链接、买家打开后恢复订单 | merchant-tools；[storefront-v2 §G3](../../contracts/storefront-v2.md)；[#42](https://github.com/humaux/live_commerce_architecture_v1/pull/42) 修复链接再生成测试竞态 | **MOCK**：C2 / `--browser-manual-order`。这不证明尚未合入的 LC-U3 抽屉。 |
| 揀货清单预览/打印与导出 | W3-02B、[W3-U1b](units/w3-u1b-picklist-ui.md)；[W3-02B 导出契约](units/w3-02b-picklist-export.md) | **MOCK**：C2 / `--browser-picklist`；C1 的相关 PG 测试。 |
| 合并出货建议、包裹组、出货/解散 | W3-07B、W3-U4 [#2](https://github.com/humaux/live_commerce_architecture_v1/pull/2)；[W3-U4 裁决与验收](units/w3-u4-parcel-merge-ui.md) | **MOCK**：C2 / `--browser-merchant-orders-ui`；C1 的包裹组 PG 测试。 |
| 退货申请、收货/检查/关闭/撤销、退款缺口记录 | W3-08B、[W3-U5](units/w3-u5-returns-ui.md)；[returns-v1 §§2–8](../../contracts/returns-v1.md) | **MOCK**：C2 / `--browser-returns-ui`；真实退款另列 NOT_RUN。 |

### 直播控台（live console）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 工作区、场次/统计、商品开关、直播价、库存 CAS、推荐商品 | LC-U1 [#5](https://github.com/humaux/live_commerce_architecture_v1/pull/5)、LC-B7；[live-console §§7.1–7.3、9](../../contracts/live-console-v1.md) | **MOCK**：C2 / `--browser-live-console`；C1 的相关 PG 测试。推荐的 pin 是控台高亮，不能当成 Meta pin API。 |
| 评论分页/筛选、私密/公开回复、UNKNOWN 防重复提交、买家删评论清除 | LC-U2a [#18](https://github.com/humaux/live_commerce_architecture_v1/pull/18)、LC-B4、[#24](https://github.com/humaux/live_commerce_architecture_v1/pull/24)；[live-console §§2–4](../../contracts/live-console-v1.md) | **MOCK**：C2 / `--browser-live-console`，真实 PG + 假 Graph；权限/消息窗口、真实平台发送分别需 LIVE 补证。 |
| 评论标签打印、服务端幂等、打印记录保留期清理 | W3-U3 [#40](https://github.com/humaux/live_commerce_architecture_v1/pull/40)、[#30](https://github.com/humaux/live_commerce_architecture_v1/pull/30)、LC-R2 [#35](https://github.com/humaux/live_commerce_architecture_v1/pull/35)；[live-console §7.4、LC-R2 amendment](../../contracts/live-console-v1.md) | **MOCK**：C2 / `--browser-live-console`；C1 的打印/保留期 PG 测试。没有真实打印机验收。 |
| 手动提醒、没货回复模板、限制买家名单 | W3-U2 [#31](https://github.com/humaux/live_commerce_architecture_v1/pull/31)；[W3-U2 已交付/排除范围](units/w3-u2-live-settings-ui.md) | **MOCK**：C2 / `--browser-live-settings`。自动定时提醒、匹配模拟器与店级默认匹配模式未由该 UI 单元交付。 |
| 从控台/Inbox 评论直接打开建单抽屉（LC-U3） | [#32](https://github.com/humaux/live_commerce_architecture_v1/pull/32) 查询于 **2026-10-13 19:31 UTC：OPEN**，head `77e08616`；[live-console §5](../../contracts/live-console-v1.md)、[order-attempt §5/§7](../../contracts/order-attempt-v1.md) | **NOT_RUN（R2 快照交付）**：#32 未合入，后续范围为 **UI-only**，须消费订单尝试后端，不再自行实现第二套交易权威。OPEN 是查询时状态，作范围或上线决定前须重查 PR/CI。 |

### 讯息收件匣（inbox）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 会话列表、未读、历史、接管、回复与权限丢失清空 | LC-U2b [#8](https://github.com/humaux/live_commerce_architecture_v1/pull/8)、LC-B3/B4、[#36](https://github.com/humaux/live_commerce_architecture_v1/pull/36)；[live-console §§3.6、11（A8–A12）](../../contracts/live-console-v1.md) | **MOCK**：C2 / `--browser-inbox`，签名假 OIDC + 真实应用/PG。真实客户消息没有 LIVE 证据。 |
| 完整 BuyerPanel：认领、价格、订单、自动回复事实；客户关联/解除、两页 CAS 冲突显式重试 | LC-B3b [#12](https://github.com/humaux/live_commerce_architecture_v1/pull/12)、LC-U2b2 [#41](https://github.com/humaux/live_commerce_architecture_v1/pull/41)；[A13/A14 冻结验收](units/lc-u2b2-customer-link.md) | **MOCK**：C2 / `--browser-inbox`，三语 ×1440/390；无 `orders:read` 时省略订单字段，不能从 UI 补取。 |

### 结账与支付（checkout and payments）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 关键词认领链接直接进入结账、合并已有购物车、已下单购物车恢复、售罄/失效处理 | [claim-direct-checkout](units/claim-direct-checkout.md)；[live-keyword-claims Direct checkout amendment](../../contracts/live-keyword-claims-v1.md) | **MOCK**：C2 / `--browser-claim-checkout`、`--browser-webkit`；Meta 入口、Messenger/IG 内置浏览器、真机 iOS 和 LIVE 域名未据此验证。 |
| 配送地址、报价、库存保留、优惠码与买家订单恢复 | buyer-order-ui、promotions；[buyer-order-ui](../../contracts/buyer-order-ui-v1.md)、[storefront-v2 §F](../../contracts/storefront-v2.md) | **MOCK**：C2 / `--browser-order`、`--browser-promotions`、`--browser-payment`；C1 的交易/PG 测试。 |
| 银行转账证明、确认/拒绝、逾时释放；宅配货到付款 | checkout-offline、home-cod；[storefront-v2 §C](../../contracts/storefront-v2.md)、[宅配 COD 裁决](units/home-cod.md) | **MOCK**：C2 / `--browser-checkout-offline`、`--browser-home-cod`。转账核对和物流收款是人工记录，无银行/宅配商接口证明。 |
| 平台信用卡启用/停用、买家披露、按店结算单与明细 | W4-S1、W4-S2、[W4-U1](units/w4-u1-payment-activation-ui.md)、结算归属修复 [#4](https://github.com/humaux/live_commerce_architecture_v1/pull/4)；[stripe-platform-account §§0.3、3–6](../../contracts/stripe-platform-account-v1.md) | **MOCK**：C2 / `--browser-card-payments`；C1 的平台收款/结算 PG 测试。仅 allowlist 的同法人自有店可启用；第三方商户不开放。**风险：[stripe-platform-account](../../contracts/stripe-platform-account-v1.md) 仍为 DRAFT（FROZEN-ready，未冻结，PF gates NOT_RUN）**，绿色 CI 不等于已验收。 |
| Stripe 捕获、退款、UNKNOWN 查询对账与金额/目录漂移保护 | stripe-b1/b2、refund-core；[#20](https://github.com/humaux/live_commerce_architecture_v1/pull/20)；[stripe-psp §§9、14](../../contracts/stripe-psp-v1.md)、[stripe-refund §§5–9](../../contracts/stripe-refund-v1.md) | **MOCK**：C2 / `--browser-refund-fulfilment`、`--browser-e2e`；C1 的交易 PG 测试。真实 Stripe Checkout/3DS/webhook/退款的 **SANDBOX、LIVE 均 NOT_RUN（本报告）**。C2 schedule 未运行 `--stripe-browser` 的任何步骤；SP18 SANDBOX 需要测试 key，缺 key 时 NOT_RUN。它也不在无凭据的普通 PR CI 集合；绿色 MOCK job 不证明 SANDBOX。 |

**R3 收款路径 owner 决策（2026-10-06、2026-10-14）**：**不使用 PAYUNi**；商家收款使用平台的 Stripe（香港大碗貿易有限公司账户，HK）；平台 Stripe 白名单**只开 owner 自营店**；第三方商家上线前**不得开放**（Stripe §3.4(b) 与台湾第三方支付登记风险，见 research 2026-10-06；该 research 为未核验输入）。PAYUNi 移除分两阶段：第一阶段（商家后台/店面/部署/文档）为待合并 [#59](https://github.com/humaux/live_commerce_architecture_v1/pull/59)；**第二阶段（Go 代码 + 前向迁移）待做**。上表 PAYUNi 相关历史描述（如 `--browser-order` 中的买家 PAYUNi 分支）以本决策为准，PAYUNi 不是上线路径。

### 超商与物流（CVS and logistics）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 超商选店、目录校验、取货付款、面单/打印、状态更新、取消/补库存 | cvs-core/ecpay/ui/tests、cvs-restock-fix；[taiwan-cvs-logistics §§5、7、8、16](../../contracts/taiwan-cvs-logistics-v1.md) | **MOCK**：C2 / `--browser-cvs`（ECPay 假服务器）。真实 ECPay stage 地图/密钥、SANDBOX 出单和 LIVE 面单 **NOT_RUN**。 |
| 人工发货、承运商/运单录入；CSV 上传或粘贴、逐行预览、确认及失败文件 | fulfilment-core、W3-01B、[w3-01b UI](units/w3-01b-tracking-backfill.md)；[manual-fulfilment §§2–6](../../contracts/manual-fulfilment-v1.md) | **MOCK**：C2 / `--browser-refund-fulfilment`、`--browser-tracking-backfill`；不证明真实物流通知或购买面单。 |

### 客户与报表（customers）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 客户列表/详情、标签管理/筛选、备注和个人备注权限、隐私导出/删除 | W6-01B、W6-U1 [#3](https://github.com/humaux/live_commerce_architecture_v1/pull/3)；[customers-billing W6-01B amendment](../../contracts/customers-billing-v1.md) | **MOCK**：C2 / `--browser-customers-billing`；C1 的标签/备注/隐私 PG 测试。测试客户均为合成数据。 |
| 买家隐私中心、未预勾选的同意选择、撤回、导出与删除说明 | customers-core、customers-billing-ui；[customers-billing §0 CD4–CD8、§5 Buyer/Storefront](../../contracts/customers-billing-v1.md) | **MOCK**：C2 / `--browser-customers-billing`。认领链接、下单或 CSV 导入不能自动建立身份合并或营销同意；真实客户删除不属本次授权。 |
| 客户与历史订单 CSV 导入、预览、确认、编码拒绝、私密错误文件、清除/丢弃计数 | W5-02B、W5-03B、W5-U1 [#13](https://github.com/humaux/live_commerce_architecture_v1/pull/13)；[导入验收](units/w5-u1-import-wizard-ui.md) | **MOCK**：C2 / `--browser-migration-import`；导入不会授予营销同意，历史归档单不计入经营报表。 |
| 商品/渠道/漏斗/手工单四类报表、已应用日期、导出 CSV | W6-02B、W6-U1 [#3](https://github.com/humaux/live_commerce_architecture_v1/pull/3)、表格溢出修复 [#15](https://github.com/humaux/live_commerce_architecture_v1/pull/15)；[reporting-v2 §§1–6](../../contracts/reporting-v2.md) | **MOCK**：C2 / `--browser-reports`；按币种/支付环境分开，不是利润、成本或自动定时报表。**风险：[reporting-v2](../../contracts/reporting-v2.md) 仍为 DRAFT（未冻结）**；按 [PROCESS §2](PROCESS.md) 契约冻结前，报表范围的验收标准仍可变，绿色 C2 不等于已验收。 |

### 广告（ads）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 广告账户连接、草稿、批准、发布/暂停/复制 | ads-core/graph/ui；[meta-ads §§2、4–8](../../contracts/meta-ads-v1.md) | **MOCK**：C2 / `--browser-meta-ads`，假 OAuth/Graph + 真实 worker/PG。真实投放/预算消耗 **LIVE NOT_RUN**。 |
| 成效、归因、Insights/CAPI、金额显示 | ads-capi、ads-attribution；金额显示 [#43](https://github.com/humaux/live_commerce_architecture_v1/pull/43)（仅 zh-TW 金额格式，不是归因来源）；[meta-ads §§6.2、6.4、6.5](../../contracts/meta-ads-v1.md) | **MOCK**：C2 / `--browser-meta-ads`、`--browser-ads-attribution`；供应商归因/insights 的 SANDBOX/LIVE 不由假 Graph 证明。 |
| 广告解绑、feed URL；失败/UNKNOWN 操作台账、取消/查询/受限重试 | W6-05B、W6-06B、[W6-U2](units/w6-u2-ops-ledger-ads-ui.md)；[external-operation W6-05B amendment](../../contracts/external-operation-v1.md) | **MOCK**：C2 / `--browser-operations-ads`；C1 的操作台账 PG 测试。UNKNOWN 必须先对账，不允许盲重试。 |

### 店铺前台（storefront）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 首页、集合筛选/排序/分页、商品规格/库存/SEO、购物车、买家订单/通知页面 | storefront-shell/integration、buyer-comms；[storefront-v2 §§A–E](../../contracts/storefront-v2.md) | **MOCK**：C2 / `--browser-storefront`、`--browser-buyer`、`--browser-buyer-comms`；没有真实邮箱投递或真实卡支付证明。 |
| 外观/导航/首页区块/信息页、后台草稿预览、发布/取消发布/回滚、自定义域名流程 | store-design、storefront-publish、store-domains；[storefront-v2 §B](../../contracts/storefront-v2.md)、[published-storefront-resolver Writer (R3)](../../contracts/published-storefront-resolver-v1.md) | **MOCK**：C2 / `--browser-design`、`--browser-storefront-publish`、`--browser-store-domains`。design 门禁只验证后台预览和 preview token；**真实买家 storefront 的设计文档渲染及其 origin 上的 Preview tab 为 NOT_RUN（本报告）**（[GATES design 限制](GATES.md)、[harness 明确排除项](../../tests/foundation/browser_store_design_test.go)）。已发布商品可见性测试不能代替这项预览。真实 DNS、证书、公开域名路由也为 **NOT_RUN**。 |
| 页面覆盖、实际点击与视觉规则 | [#22](https://github.com/humaux/live_commerce_architecture_v1/pull/22)、[#23](https://github.com/humaux/live_commerce_architecture_v1/pull/23)、[#39](https://github.com/humaux/live_commerce_architecture_v1/pull/39)、[#43](https://github.com/humaux/live_commerce_architecture_v1/pull/43)；[GATES 的 sharding/PR gates](GATES.md) | **MOCK**：C2 / click-sweep ×10、visual-lint ×4 和 aggregate 成功。自动规则不是全页面人工视觉签字。click-sweep 对同类控件（带 id 的行）**抽样每类 3 个**（[GATES](GATES.md) `--browser-click-sweep`），因此“每个可操作控件都被点击”的验收为 **部分／NOT_RUN**；issue #45 另限制商品控件完整性。 |

### 直播场次（studio）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 场次列表、创建/编辑/重开，BFF 与权限状态 | studio-backend、studio-ui；[studio-v1 Planning、Merchant API](../../contracts/studio-v1.md) | **MOCK**：C2 / `--browser-studio-bff`、`--browser-studio-ui`；planning 与媒体运行分离。 |
| 输入 token 传输、浏览器输入与 MOCK rehearsal 路径 | input-delivery、studio G2；[studio-v1 G2 amendment](../../contracts/studio-v1.md) | **MOCK**：C2 / `--browser-input-delivery`、`--live-browser-input`。真实 SFU 解码、LiveKit Cloud、目标平台可见开播 **NOT_RUN**；R1 的媒体开关限制仍按 [deploy 范围](../../deploy/README.md#known-limits-recorded-by-the-r1-deploy-unit)处理。 |

### 开店设置（settings）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 商家身份/开店向导、支付/物流配置与能力状态 | merchant-settings、[#14](https://github.com/humaux/live_commerce_architecture_v1/pull/14)；[merchant-settings-http Routes / No capability upgrade](../../contracts/merchant-settings-http-v1.md)、[merchant-service-settings §§2–5](../../contracts/merchant-service-settings-v1.md) | **MOCK**：C2 / `--browser-identity`、`--browser-password-auth`、`--browser-customers-billing`。配置成功不能说明供应商 LIVE 已开通。 |
| 店员/团队邀请、接受、角色变更与撤销，至少保留一位 owner、权限按请求检查及审计 | [staff-team](units/staff-team.md)、[storefront-v2 §D / D acceptance](../../contracts/storefront-v2.md)、[0119 的 Inbox 角色权限修订](../../migrations/0119_live_console_inbox.sql) | **MOCK（C2 历史证据）**：[password-auth job 115102886582](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795/job/115102886582) 实际执行并通过 `TestBrowserStaffTeam`：zh-TW/en ×桌面/390，浏览器→Next→Go→PG，邀请邮件使用真实 SMTP adapter 对 loopback 假邮箱。一次性邀请 72h；角色权限以服务端当前 bundle 为准（viewer 不含 `inbox:read`）。真实邮箱投递 **NOT_RUN**，不把 C2 移作 R2 全量验收。 |
| 公共平台网站：平台经营主体、首页、privacy/terms/data-deletion/contact、三语和后台入口 | [platform-site / PS1–PS5](units/platform-site.md)、[公司资料单一来源](../../apps/admin/lib/company.ts)、[deploy §4.2 平台站附加检查](../runbooks/deploy.md) | **MOCK（C2 历史证据）**：[platform-site job 115102886936](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795/job/115102886936) 记录 PS1/PS2/PS4 PASS（30 页面、15 SSR、16 标签用例，390 实际点击）。公开页面由 admin Next 在平台 Host 服务，与各商户买家隐私页不同。PS3 边缘核验是独立 `tests/deploy/platform-edge.mjs`，不由这个 job 证明；公开 DNS/www、生产配置、法律终稿及 Meta URLs 放行仍 **NOT_RUN**，见上线步骤。 |
| 平台订阅套餐/用量、欠费提醒、订阅 Checkout 与客户门户入口 | billing-core、customers-billing-ui；[customers-billing §0 BD1–BD6、§5 Admin screens](../../contracts/customers-billing-v1.md) | **MOCK**：C2 / `--browser-customers-billing`（托管 Checkout/portal 重定向由测试拦截）。欠费不阻断既有买家结账、退款或履约；真实平台订阅扣款、发票/门户 **SANDBOX / LIVE NOT_RUN**。C2 schedule 没有执行 `--stripe-browser`；需要测试 key 的 SANDBOX 步骤 NOT_RUN，不能由这个绿色 MOCK mode 推定通过。 |
| Facebook/Instagram 自助连接/断开、重连提示、读留言/私讯/公开回复状态与“仅测试账号”限制 | meta-connect、W1-01B、W1-01U；[meta-claims-intake Merchant connect amendment](../../contracts/meta-claims-intake-v1.md)、[meta-connection-health §§3、5、7、11](../../contracts/meta-connection-health-v1.md) | **MOCK**：C2 / `--browser-meta-connect`、`--browser-meta-health-ui`；C1 的连接/健康 PG 测试。MCH12 LIVE_READ 与非应用角色授权 **NOT_RUN（本报告）**。**风险：[meta-connection-health](../../contracts/meta-connection-health-v1.md) 仍为 DRAFT（待对抗审查与冻结，gates NOT_RUN）**，绿色 CI 不等于已验收。 |

## 已知缺口与 FOLLOWUPS

输入来源之一是集成方 **本机未跟踪（untracked）的本地文件** `output/integrator/FOLLOWUPS.md`，取自 2026-10-13 工作记录。**它未随本报告提交，owner 无法通过本 PR 验证其原件；它不是已提交的 release ledger 或 CI 证据。所有由该文件摘录的行均明确标为“未核验摘要（FOLLOWUPS）”**：列出的 PR/thread 是追查索引，本轮未逐项重查原评论、后续修复及其 CI，不能凭摘要认定当前仍未修复、已经修复或关闭。该本地文件包含已修复旧条目，**不把“被记录”写成“已完成”**；状态变化须用新 PR/CI 收据关闭。数据完整性或隐私问题按 [PROCESS §2](PROCESS.md#2-per-unit-software-engineering-flow) 与集成方收敛裁决提升处理优先级。

| 缺口 / 后续工作 | 可核对来源与状态 |
| --- | --- |
| **R3 后续（来自 [FOLLOWUPS.md](../../output/integrator/FOLLOWUPS.md)，本地未跟踪文件，未核验摘要）**：HARNESS-PG-CAPACITY；OA1-NOTIFY-FLAKE；CI Go proxy；PAY-RM2 第二阶段；#32 LC-U3 | HARNESS-PG-CAPACITY：由 [#60](https://github.com/humaux/live_commerce_architecture_v1/pull/60) 修复（守卫见 [#56](https://github.com/humaux/live_commerce_architecture_v1/pull/56)）。OA1-NOTIFY-FLAKE：1 次 rerun（详见 FOLLOWUPS）。CI Go proxy：[#61](https://github.com/humaux/live_commerce_architecture_v1/pull/61)（待合并）。PAY-RM2 第二阶段（Go + 前向迁移）：待做，第一阶段 [#59](https://github.com/humaux/live_commerce_architecture_v1/pull/59) 待合并。[#32](https://github.com/humaux/live_commerce_architecture_v1/pull/32) LC-U3：RC 之后在 OA-1 上返工（UI-only）。 |
| 商品编辑器控件完整性：删轴/删规格值按钮尚缺完整 inventory、390px 点击、回读与台账；Go/Node 参数别名、env 前缀、实际执行绑定、批量操作选项、未知 JSX 控件还有硬化项 | [issue #45](https://github.com/humaux/live_commerce_architecture_v1/issues/45) 查询于 **2026-10-13 19:31 UTC：OPEN**，P1 thread `4254703909`；owner 批准 #28 先合入不代表缺口消失。跟进 [#46](https://github.com/humaux/live_commerce_architecture_v1/pull/46)、[#47](https://github.com/humaux/live_commerce_architecture_v1/pull/47) 同时查询：**均 OPEN**。作范围、验收或上线决定前须重新查询 issue/PR 状态及对应 CI。 |
| 订单尝试重构在交付管线中；LC-U3 #32 待后端完成后以 UI-only 接入 | **2026-10-13 19:31 UTC 重查** [#50](https://github.com/humaux/live_commerce_architecture_v1/pull/50)：MERGED，合并时间 **16:54:58 UTC**（`2d97c2027`）；[order-attempt-v1 顶部状态](../../contracts/order-attempt-v1.md) 为 **FROZEN / DESIGN，运行验收 NOT_RUN**，正文仍留有 Draft 历史交接措辞，不能据此降回未冻结或升级为已实现。OA-1/2/3 的分工与排队状态见下表的独立时间快照；[#32](https://github.com/humaux/live_commerce_architecture_v1/pull/32) **19:31 UTC 查询仍 OPEN**，未计入 R2 快照。作决定前须重查 PR/CI。 |
| **未核验摘要（FOLLOWUPS）**：Inbox/评论前台列表与未读更新、历史清除恢复失败、651–1150px 买家面板布局、窄屏键盘焦点、打开后锚底/新消息提示；长列表客户关联/付款链接来源恢复上限 | [#8](https://github.com/humaux/live_commerce_architecture_v1/pull/8) threads `4212543434/3438/3445/3452/3457`；origin 扫描 `4209444305`；[#12](https://github.com/humaux/live_commerce_architecture_v1/pull/12) link-pending 记录。未重查对应后续修复 CI，不认定当前开闭状态。 |
| **未核验摘要（FOLLOWUPS）**：高流量 IG 同微秒 seq 游标、>1000 原始事件重建预算、刷新游标回退、HEAD/增量窗口对齐；bridge 响应体上限、Strict Effects/backoff/UNKNOWN 恢复 | [#18](https://github.com/humaux/live_commerce_architecture_v1/pull/18) threads `4226524062/4067/4071`、`4227138850`、`4230211367/1379`；[live-console §§2–4](../../contracts/live-console-v1.md)。不声称无限量会话或所有故障已验证。 |
| **未核验摘要（FOLLOWUPS）**：客户/包裹/控台 UX 的长标签目录定位、长备注就近编辑、重复包裹组显示/逐项 busy/运单错误定位、库存保存后旧值闪回、IG 提示误用于 FB | [#3](https://github.com/humaux/live_commerce_architecture_v1/pull/3) `4212542070/2075`；[#2](https://github.com/humaux/live_commerce_architecture_v1/pull/2) `4212524352/4369/4378`；[#5](https://github.com/humaux/live_commerce_architecture_v1/pull/5) `4212525437`及 K3 后续。 |
| **未核验摘要（FOLLOWUPS）**：商品移动端 UX/视觉硬化的内滚动截图、pane 顶部布局检查、金额整值省略小数的统一风格 | [#43](https://github.com/humaux/live_commerce_architecture_v1/pull/43) FOLLOWUPS（91b9a6a8）；残余项未经本轮逐项复核。另经 **2026-10-13 19:31 UTC GitHub 重查**，[#49](https://github.com/humaux/live_commerce_architecture_v1/pull/49) 已于 **18:51:40 UTC MERGED** 到 `228a1c877`；其合入范围含同名可访问标签、跨900px焦点、重绘回卷、提示高度及 chips 等待，不扩大到其他视觉缺口或整站验收。 |
| **未核验摘要（FOLLOWUPS）**：evidence-root/env override、递归 symlink、manifest/原始哈希可核性、重钉程序、CI-SELECT 删除覆盖、CI-DRIFT shadow/computed key/template 分析 | [#23](https://github.com/humaux/live_commerce_architecture_v1/pull/23) `4228878459/8471/4230486608`；[#43](https://github.com/humaux/live_commerce_architecture_v1/pull/43) r2；[#39](https://github.com/humaux/live_commerce_architecture_v1/pull/39) r4/r7；[#33](https://github.com/humaux/live_commerce_architecture_v1/pull/33) `4233809775/4234857444/4234857453`。日志完整性不能只靠作者自证。 |
| **未核验摘要（FOLLOWUPS）**：导入编码拒绝 zh-CN 矩阵、长套件分片、非共享 helper 端口交接、WebKit apt、90秒媒体恢复/密码 rate-limit 时钟边界 | W5-U1 K3、[#37](https://github.com/humaux/live_commerce_architecture_v1/pull/37)、[#27](https://github.com/humaux/live_commerce_architecture_v1/pull/27)、[#43](https://github.com/humaux/live_commerce_architecture_v1/pull/43) run `38293593095`、[#44](https://github.com/humaux/live_commerce_architecture_v1/pull/44) run `38293840385`。历史重跑不能证明不再有 flake。 |
| 商品退回/物流与结算限制：卡付超商退回的补库存需人工处理；无跨境 first-mile/海关验收；结算分配不等于打款，没有自动银行拨付；争议靠同步入账 | [returns-v1 §7a](../../contracts/returns-v1.md)、[taiwan-cvs §13](../../contracts/taiwan-cvs-logistics-v1.md)、[stripe-platform-account §9](../../contracts/stripe-platform-account-v1.md)。 |
| 报表与广告范围：无 profit/COGS、历史导入单/定时报表；漏斗 link_sent 只计 private_reply；无真实 Insights/CAPI/广告预算实时硬停止保证 | [reporting-v2 §§1、4](../../contracts/reporting-v2.md)、[meta-ads §12](../../contracts/meta-ads-v1.md)。 |

**旧修复索引亦为未核验摘要（FOLLOWUPS）**：该文件记录报表溢出/时区 fallback/订单等待 [#15](https://github.com/humaux/live_commerce_architecture_v1/pull/15)，购物车刷新丢旧读 [#25](https://github.com/humaux/live_commerce_architecture_v1/pull/25)，BO01 竞态 [#26](https://github.com/humaux/live_commerce_architecture_v1/pull/26)，打印事实/收据/关联保留期清理 [#35](https://github.com/humaux/live_commerce_architecture_v1/pull/35)，模式 registry [#22](https://github.com/humaux/live_commerce_architecture_v1/pull/22)，CI 后端覆盖/夜间全量 [#39](https://github.com/humaux/live_commerce_architecture_v1/pull/39)，手工单再生成/native CDP [#42](https://github.com/humaux/live_commerce_architecture_v1/pull/42)，店铺 collection chips 与 MOU03 关闭页签 [#44](https://github.com/humaux/live_commerce_architecture_v1/pull/44)。本轮未逐一重查这些修复收据；不能把本地记录单独当成关闭证明，或把其残余项当成整个功能未实现。没有用本地收据验收。

订单尝试管线状态（**2026-10-13 19:05 UTC 查询快照，均不等于 R2 发布功能**）：[OA-1 已推送的单元边界](https://github.com/humaux/live_commerce_architecture_v1/blob/70c9e330fc60a733defcfc2992e7dcd6cc403f0e/docs/delivery/units/oa-1-order-attempt-core.md) 说明三段分工，[同 SHA 的 DELIVERY](https://github.com/humaux/live_commerce_architecture_v1/blob/70c9e330fc60a733defcfc2992e7dcd6cc403f0e/output/oa-1/DELIVERY.md) 明示仍在开发、新下单路径未启用；[R2 来源记录](../../output/delivery-report-r2/DELIVERY.md#订单尝试状态来源) 保存协调任务标识，提交前或放行前须重查。

| 单元 | 分工与进度 | 本报告验收 |
| --- | --- | --- |
| OA-1 | 原子订单尝试核心：C1、完整 C2、job/sweep；协调状态 working，分支 `unit/oa-1-order-attempt-core` 处于实现中，未合入 R2 | **NOT_RUN（完整功能）**；分支上的分段 CI 不等于整条新协议完成 |
| OA-2 | 通知桥 C3/C4、派发/取消约束、copy/reminder/CAS；协调状态 submitted（已排队，尚未认领） | **NOT_RUN**；不把管线推进中写成已经交付 |
| OA-3 | 版本化 admission/recovery API、保留/旧数据切换、A15 与 #32 UI-only 接入；协调状态 submitted（已排队，尚未认领） | **NOT_RUN**；须依赖后端协议及自己的验收 |

## 上线前 owner 要完成的步骤

| 步骤 | 所需记录 / 放行条件 | 本报告状态与来源 |
| --- | --- | --- |
| Meta LIVE 只读探针 | owner 批准测试资产/账号/范围；记录 Graph 版本、权限/任务与脱敏响应形状、时间、SHA、HTTP 结果。只读成功不能证明发送、广告写入或非角色用户可用；涉及真实 OAuth/撤权/重连的扩展步骤须另取明确授权 | **NOT_RUN**；[meta-connection-health §11.3 MCH12](../../contracts/meta-connection-health-v1.md)、[live-console §13.3](../../contracts/live-console-v1.md)、[Meta 接入 runbook §6.3/6.7](../runbooks/deploy.md)。不得把含 UI 变更的整套 §11.3 默认为纯只读授权。 |
| App Review / Access Tier 提交及通过（B6） | owner 提交准确权限、业务验证与演示材料，记录 submission/result；仅在实际批准后更改 Advanced Access 配置并由 CI/授权探针补证；未通过保持“仅测试账号”。**G2 工程路由已挂载/关闭，不解除 B6 外部审核阻塞** | **NOT_RUN（B6）**；[deploy §0 B6 / §6.3 第7项](../runbooks/deploy.md)、[meta-connection-health §13 MCH-OPEN-5](../../contracts/meta-connection-health-v1.md)、[merchant-onboarding §0/4.1](../runbooks/merchant-onboarding.md)。工程 MOCK 不等于 Meta 审核通过。 |
| 公共平台网站上线放行 | owner／律师批准平台法律文本，核对公司名称、编号、地址与 CI/BRC；配置平台/后台 Host、联系邮箱的构建与运行环境；批准裸域/www DNS-only 指向 edge，验证 www 301 保留路径/查询、三语公共页200、robots/sitemap、公开 Host 的 `/api/*`404；完成域名验证并更新 Meta App Domains、Website、Privacy、Terms、Data Deletion Instructions URLs | **NOT_RUN（生产上线门槛）**；[platform-site Owner actions / PS1–PS5](units/platform-site.md)、[deploy §4.2 平台站与 Meta 外网核验](../runbooks/deploy.md)。2026-10-04“无 DNS”的旧观察不作为当前 DNS 状态；本站不虚构 signed_request 自动删除回调。 |
| 生产部署批准 | owner 明确 tag/SHA、店铺/功能范围、窗口、回滚与数据库备份/恢复安排；满足 [deploy §10](../runbooks/deploy.md) preflight `--online`、下列独立备份/演练记录、离线密钥副本、smoke `full`、`deployments.log`；候选 release-gate CI、部署 smoke CI 和独立复核结果齐备后，仍需本次批准才可动生产 | **NOT_RUN／RCG 待追加**；[deploy §§0、3、5、10](../runbooks/deploy.md)、[PROCESS §2](PROCESS.md)。本报告不核定试点主机的生产状态。 |
| 本次升级前备份 | **每次升级**取得 `pre-upgrade-<tag>` 成功收据；失败即中止升级，不能由旧恢复演练代替 | **NOT_RUN（本次升级）**；[deploy §4 升级 / §10](../runbooks/deploy.md)、[deploy.sh 的备份调用](../../deploy/scripts/deploy.sh)。 |
| 最近30天恢复演练 | 保留最近30天内一次恢复演练的独立收据，另核对离线密钥副本；本次备份成功不能代替恢复能力证明 | **NOT_RUN（生产恢复演练）**；[deploy §10](../runbooks/deploy.md)、[backup-restore runbook](../runbooks/backup-restore.md)。CI 合成恢复测试不替代生产运维记录。 |
| 密钥轮换与保管确认 | owner/获授权运维按密钥 manifest 与 runbook 轮换，保存脱敏审计和恢复副本；keyring 只追加新 key/切 active，未重新密封前保留旧 key；BFF 两端协调；cookie、actor/replay/link key 的影响分别评估 | **NOT_RUN（实际轮换）**；[deploy runbook §7](../runbooks/deploy.md#7-密钥轮换按-deploysecretsmanifesttsv-的-rotation-列)、[secrets manifest](../../deploy/secrets.manifest.tsv)。CI 临时凭据不代表生产凭据已轮换。 |
| 上线前经营决策 U01–U08（[架构.md §2.3](../../架构.md)）：总览 | 各项阻塞范围见 §2.3 表；下列 U01–U08/LG01 逐项为 R3 更新。**以下建议、费用、期限与法规摘要均来自 2026-10-14 web GPT research（[gpt-owner-decisions-20261014.md](../../output/integrator/research/gpt-owner-decisions-20261014.md)），属 research、未核验，不是法律或合规结论**。agent 不得以“行业平均”代填。 | **NOT_RUN（owner 事项）** |
| U01 发货地 | owner 事实：香港直发，或台湾仓 | **待 owner 填写** |
| U02 首批 SKU | owner 事实：首批 SKU 清单，及是否含电池/液体/化妆品/品牌货 | **待 owner 填写** |
| U03 Meta 验证与审核 | 香港大碗 Business 验证 + App Review 提交（预留 4–6 周）。`xgdwm.com` 网域验证 token 已于 2026-10-14 换为 `6qt099…`（pilot env）；**owner 需在 Meta 后台点「验证」** | **NOT_RUN（owner 动作）**；见上方 B6 行 |
| U04 成本与并发 | 月度成本上限（research 建议 $300）与并发直播数 | **待 owner 确认** |
| U05 ECPay 超商物流 | research：纯香港公司不能开 ECPay 普通特约商户（需台湾登记主体）→ 在有台湾主体/3PL 前，超商取货不能对 owner 自营店开通；替代方案 = 香港直发跨境宅配（追问中） | **阻塞／待 owner 决策**；ECPay LIVE 仍 NOT_RUN |
| U07 / B2 商家登录 | owner 二选一：Logto Cloud 日本区 + MFA + Postmark（约 $135/月，research 估价）；或继续内建密码登录并补 MFA/恢复。须先开通专用 SMTP 邮箱（[mail runbook](../runbooks/mail.md)、[merchant-onboarding O2](../runbooks/merchant-onboarding.md)）。当前身份证据仅为签名 MOCK OIDC 与 MOCK SMTP 的密码认证测试 | **NOT_RUN（上线阻塞 B2）**；[deploy runbook §0 B2](../runbooks/deploy.md)。 |
| U08 保存期 | research 建议，owner 确认：香港商业记录 7 年；台湾会计凭证 5 年/账簿 10 年；安全审计 5 年；原始 webhook 7–30 天；调试日志 30–90 天；备份滚动 35 天 | **待 owner 确认**（律师/会计复核另议） |
| 部署批准：#58 | [#58](https://github.com/humaux/live_commerce_architecture_v1/pull/58)（Caddy `/about` 白名单 + admin 镜像）上线须 owner 批准的部署，并符合上方「生产部署批准」条件 | **NOT_RUN／待 owner 批准** |
| 单机 Compose 生产形态的风险接受（B8） | owner 通过 ADR O1 接受单机 Docker Compose 非高可用风险（架构.md §4.2 将 Compose 定位为开发/验收环境） | **NOT_RUN（上线阻塞 B8）**；[deploy runbook §0](../runbooks/deploy.md)。 |
| T21 独立架构与安全审查（B7） | T21-01..03 历史修复：[cvs-restock-fix `58a54a40`](https://github.com/humaux/live_commerce_architecture_v1/commit/58a54a401e5b671284ef464517b1ca7c749811e9)、[worker-authority-split merge `dc52c110`](https://github.com/humaux/live_commerce_architecture_v1/commit/dc52c110500dfbf044ef3e2f43c19f7450736b92)。Git 祖先校验确认二者属于 R1 `0bfe36e5` 和 R2 `228a1c877`；GitHub commit→PR 查询未返回关联 PR，故不编造“由 PR #N 合入”。[R2 记录](../../output/delivery-report-r2/DELIVERY.md#历史提交与-pr-归属) 保留查询和退出码；[tasks.json T21](../../contracts/tasks.json) 仍为 NOT_STARTED，需独立审查结论 | **NOT_RUN（完整 T21 / B7）**；[deploy §0](../runbooks/deploy.md)。历史修复的祖先关系不是完整独立安全验收。 |
| PAYUNi 移除（PAY-RM2；原 B3 / OPEN-1） | **owner 已决定（2026-10-06、2026-10-14）：不使用 PAYUNi**，B3（PAYUNi 资质）不再是 owner 事项。第一阶段（商家后台/店面/部署/文档）= 待合并 [#59](https://github.com/humaux/live_commerce_architecture_v1/pull/59)；第二阶段（Go + 前向迁移）待做 | **第一阶段待合并／第二阶段 NOT_RUN**；[pay-remove-payuni](units/pay-remove-payuni.md)、[payuni-wire PAY-RM1 amendment](../../contracts/payuni-wire-v1.md)。deploy §0 B3 的旧文字待 #59 合并后核对。 |
| PAYUNi NotifyURL 接收端（B4） | **B4: receiver removed from code (`cmd/api/payuni_notify.go` absent); deploy.md §0 updated**。这是主动移除功能的源代码状态，不是待建接收端；整体移除由 PAY-RM2 完成 | **代码移除已核对／LIVE NOT_RUN**；[payuni-wire PAY-RM1 amendment](../../contracts/payuni-wire-v1.md)、[deploy §0 B4](../runbooks/deploy.md)；现有 [路由移除测试定义](../../cmd/api/payuni_removed_test.go) 本轮未运行。 |
| 直播 Studio 媒体（B5） | LiveKit 仅 MOCK；上线 Studio 推流须另取真实媒体服务配置与验收，否则以"Studio 不开放"范围上线 | **MOCK／LIVE NOT_RUN（B5）**；[deploy runbook §0](../runbooks/deploy.md)。 |
| 法律政策页终稿（LG01） | owner 提供/批准隐私、条款、退款、配送、联系页文本。research 要点（未核验）：七日鉴赏期按日历日；§19-2 十五日取回/退款；留言≠订单成立；个资法 §8 六项告知。`LC_LEGAL_REQUIRE_FINAL=1 node tests/storefront/legal-pages.mjs` 在 CI 全绿后才能填写 Stripe `policy_pages` 证明 | **NOT_RUN／BLOCKED（Stripe LIVE 前置）**：页脚政策链接在候选 `0bfe36e5` 已挂载（`ShopFooter` / `.sf-footer__legal`，runbook 该项描述已过时）；剩余阻塞是政策文本中的 owner/草稿占位与 LG01 终稿门禁；[stripe-live runbook §步骤 policy_pages](../runbooks/stripe-live.md)、`apps/storefront/lib/legal-copy.ts`（草稿页面不发布）。 |
| CAPI 的保留类与擦除前置 | **CAPI 生产挂载阻塞**分两项：`ads.insights_daily` / `ads.capi_events` 的保留类，以及 `ads.capi_contexts` 的擦除处理，须按 T14 / customers-billing CD7 登记；不能把三表都笼统称为“保留类”。现有上下文清理（超过8天或撤回同意，单批≤1000）与撤回/擦除阻断发送机制不自动关闭登记阻塞，已发事件也不能召回 | **NOT_RUN（生产登记/放行）**；以 [meta-ads §12 / §6.4](../../contracts/meta-ads-v1.md) 为准，[0080 清理 definer](../../migrations/0080_meta_capi.sql)、[attribution sweeper](../../internal/attribution/sweeper.go) 是现有实现来源；[deploy §6.4 第5项](../runbooks/deploy.md) 已同步区分两项登记。 |
| 每店广告权限与额度分别核对/开通 | **当前 0089 会给新店创建者应用完整 owner bundle，并回填既有 owner，包含 `ads:*`**；旧授予脚本的“创建者未获广告权限”是 0074 阶段说明，不应再要求所有店铺重复补发。仅对实际缺权的历史主体核查授权并按 owner 批准的补发步骤处理。**有权限不等于可以花费：每店 allowance 默认 NT$0（关闭）**，运营须另设 `max_active_budget_minor` / currency；内部批准预算上限不等于 Meta campaign `spend_cap` 或实时硬停止，也不替代真实投放批准 | **NOT_RUN（生产核对/开通）**；[0089 创建者触发器与 owner 回填](../../migrations/0089_staff_team.sql)、[storefront-v2 D acceptance](../../contracts/storefront-v2.md)、[旧 grant-ads-permissions.sql](../../scripts/ops/grant-ads-permissions.sql)、[meta-ads store_settings / O4 / §12](../../contracts/meta-ads-v1.md)、[meta-admin ads-settings](../../cmd/meta-admin/ads.go)；[deploy §6.4 第6项](../runbooks/deploy.md) 已同步核对／按需补发步骤。 |
| R2 旧店权限核对 | deploy §6.5 第3项记载 0079 前旧店的 `customers:read`、`customers:privacy`、`billing:manage` 补发。当前 0089 已回填 owner 完整权限集；应先核对实际主体/角色/权限，确有缺权且批准后才按旧补发步骤处理，不把店铺创建日期当成必需补发的唯一条件 | **NOT_RUN（生产核对/补发）**；[deploy §6.5 第3项](../runbooks/deploy.md)、[0089 owner 回填](../../migrations/0089_staff_team.sql)、[storefront-v2 D acceptance](../../contracts/storefront-v2.md)。这与广告额度开通是不同步骤。 |
| 收款/物流/真实消息的独立开通 | 平台 Stripe（香港大碗貿易有限公司 HK 账户）allowlist 只开 owner 自营店，第三方商家上线前不得开放（owner 决策 2026-10-06/2026-10-14）；不使用 PAYUNi；SANDBOX 凭据/回调验收与 LIVE 审批、额度/停收/对账分开记录；真实 canary 付款/退款 owner 亲自做；承运商合同、ECPay LIVE 出单另批 | **SANDBOX / LIVE 均 NOT_RUN（本报告）**；[stripe-platform-account AD-PF2 / §7](../../contracts/stripe-platform-account-v1.md)、[stripe-live runbook §§0–2](../runbooks/stripe-live.md)、[deploy runbook §6.6](../runbooks/deploy.md)。第三方商户不能以平台账户收款。 |

上述是需要补齐的 owner 记录，不是让 agent 在本机执行的命令清单。按 2026-10-13 指令，**全部测试、服务器运行及容器验证只在 GitHub CI**；生产操作属于另一次明确授权。报告撰写未启动服务、读取 live key、触及真实客户数据或进行平台变更。

## 保留为 owner-only 的红线

| 动作 | 边界及来源 |
| --- | --- |
| 真钱扣款/退款、LIVE 密钥与 live-canary、平台店铺 allowlist/停收策略 | 本次文档/CI授权不包含这些动作；owner 定范围并批准，真实 canary 由 owner 操作。[stripe-live runbook §0/步骤6](../runbooks/stripe-live.md)、[stripe-platform-account AD-PF2/§7](../../contracts/stripe-platform-account-v1.md)。 |
| 正式部署、真实数据删除、破坏性迁移/生产恢复、密码/密钥轮换 | 先批准影响面及回滚/备份，再交获授权运维执行；不得拿文档 PR 合并当生产批准。[AGENTS](../../AGENTS.md)、[deploy runbook §§5、7、10](../runbooks/deploy.md)。 |
| 真实广告发布/消耗预算、真实客户私讯/公开回复、营销重播、购买物流面单 | CI 只使用合成数据及假平台；每项真实动作仍需明确批准，权限/同意/消息窗口不可跳过。[AGENTS](../../AGENTS.md)、[meta-ads §11](../../contracts/meta-ads-v1.md)、[live-console §§3–4](../../contracts/live-console-v1.md)、[deploy runbook §6.6](../runbooks/deploy.md)。 |
| 向真实买家投递电子邮件或通知、重放生产通知队列 | 包括订单、付款、取货及售后通知；测试邮箱/SMTP fake 的成功不授权真实投递。启用真实通知通道或重放前，owner 须明确批准店铺、收件范围、消息和发送额度。[storefront-v2 §E](../../contracts/storefront-v2.md)、[deploy runbook §§2、4 的买家邮件开关](../runbooks/deploy.md)、[AGENTS](../../AGENTS.md)。 |
| 接入、轮换或启用 ECPay LIVE 凭据，以及 LIVE 物流出单/购买真实面单 | 文档、MOCK 回调和选店测试不包含 LIVE 凭据操作或真实面单购买；须有独立 owner 批准，不能以 SANDBOX 配置或绿色 CI 代替。[deploy runbook §6.6](../runbooks/deploy.md)、[taiwan-cvs-logistics §13](../../contracts/taiwan-cvs-logistics-v1.md)、[AGENTS](../../AGENTS.md)。 |
| 自定义域名的生产 DNS 与 TLS 切换 | 包括变更 DNS 记录、生产证书签发/续期配置、证书或边缘路由 cutover；须由 owner 批准域名、流量影响、窗口和回滚，再交获授权运维执行。MOCK 域名状态测试不授权生产切换。[published-storefront-resolver Writer (R3)](../../contracts/published-storefront-resolver-v1.md)、[deploy runbook §§2、4](../runbooks/deploy.md)、[AGENTS](../../AGENTS.md)。 |
| Meta App Review、商家/平台合同与商业资格、生产域名/主机/备份方案 | owner 负责真实资质与批准；agent 不补造资格，也不收集 FB/IG 密码或绕过官方权限。[PROCESS §1](PROCESS.md#1-release-plan)、[merchant-onboarding §0](../runbooks/merchant-onboarding.md)、[AGENTS](../../AGENTS.md)。 |

## 待追加：release-gate CI 收据

集成方后续在此追加：**候选 SHA、GitHub run URL/ID、完整 gate/tier 列表、PASS/FAIL/NOT_RUN、供应商环境、artifact 位置、独立审查结论、owner 批准引用**。当前留空的结论为 **NOT_RUN**。注意 `scripts/dev/release-gate.sh` 与 deploy-release 单元只覆盖 R1（G01–G07、G90–G99 及浏览器模式）；[tasks.json](../../contracts/tasks.json) 的 T22 要求 G01–G15，而 T19（G12/G15 基准与消融）为 **NOT_STARTED**。G08–G15 须单独取证，缺项保持 NOT_RUN，绿色 R1 不等于完整候选验收；本报告不包含“可正式上线”或生产 LIVE 验收结论。[deploy-release 单元第4项](units/deploy-release.md)、[PROCESS §4](PROCESS.md#4-evidence-labels-agentsmd)。
