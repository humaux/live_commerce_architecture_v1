<!-- Purpose: owner-facing release-candidate inventory, CI evidence boundaries, outstanding work and launch decisions.
Depends on: the pinned r3/integration snapshot, linked GitHub PRs/CI runs/issues, frozen contracts, GATES.md and the integrator FOLLOWUPS record.
Used by: owner and integrator when reviewing the release candidate; this report neither authorizes production nor replaces the release gate. -->
# 交付报告：r3/integration 发布候选

报告日期：2026-10-13。范围固定在 [`0bfe36e53acb694f4697521202e24f0790638941`](https://github.com/humaux/live_commerce_architecture_v1/commit/0bfe36e53acb694f4697521202e24f0790638941)，即合入 [PR #28](https://github.com/humaux/live_commerce_architecture_v1/pull/28) 后的候选代码。后续提交须另补证据。**这是开发交付盘点；完整 release-gate CI 结果尚未加入，不能据此批准上线。** [验收定义](PROCESS.md#4-evidence-labels-agentsmd)、[发布检查清单](../runbooks/deploy.md#10-发布检查清单)。

## 证据怎么读

本报告只使用 GitHub CI 运行结果；本地日志、截图和作者自报 PASS 不计入验收。`MOCK` 表示合成数据、假身份或假供应商，可能同时运行真实 Next/Go/PostgreSQL；`SANDBOX` 须有真实供应商测试环境的收据；`LIVE` 须有获授权的真实平台收据；`NOT_RUN` 表示本报告未取得该范围的有效证据。**浏览器模式名、契约中的目标等级、数据库实测和配置成功，都不能把 MOCK 提升为 SANDBOX/LIVE。** [PROCESS §4](PROCESS.md#4-evidence-labels-agentsmd)、[GATES：Pull-request gates 的供应商例外](GATES.md)。

| 编号 | CI 运行及实际范围 | 结果与限制 |
| --- | --- | --- |
| C1 | [38357392862](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38357392862)，候选 SHA `0bfe36e5`，push；`shard:unit`、`g01`–`g10` 和 `required` | SUCCESS。证明当前代码的所选 Go/真实 PG 测试；此 push **没有浏览器任务**，deploy 和 aggregate 是 SKIPPED，不能算部署或浏览器 PASS。 |
| C2 | [38338136795](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795)，祖先 SHA `5a54d41b`，schedule；下表列出的浏览器模式、click-sweep ×10、visual-lint ×4、aggregate、required | SUCCESS。全浏览器回归发生在该祖先版本；**不是候选 SHA 的完整重跑**。它没有真实 Stripe/ECPay/Meta 凭据，也不是 release-gate。 |
| C3 | [38352001282](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38352001282)，[PR #28](https://github.com/humaux/live_commerce_architecture_v1/pull/28) head `3e97bd49`；`--browser-product-editor`、`--browser-catalog-core`、required；[Foundation 38352001264](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38352001264) | SUCCESS；PR 合入候选 SHA。覆盖新增商品矩阵台账检查；完整性仍受 [issue #45](https://github.com/humaux/live_commerce_architecture_v1/issues/45) 的明确缺口限制。 |
| RCG | 候选 SHA 的完整 release-gate CI run ID、各 tier 结果、artifact、独立验收 | **NOT_RUN／待集成方追加**。不得用 C1/C2/C3 替代。供应商 SANDBOX/LIVE 结果分别填入，缺项继续保留 NOT_RUN。 |

下表的 `C2 / --browser-…` 指 C2 中该实际成功的 job；单元或 PR 说明交付归属，契约说明能力边界。旧契约的 DESIGN/NOT_RUN 标题不覆盖新的 CI 结果；旧 CI 的成功也不覆盖未来版本。模式的定义及限制见 [GATES](GATES.md)。

## 业务能力清单

### 商品与库存（catalog）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 商品新增/编辑、草稿/上架、slug/SEO、规格矩阵、批量 SKU、追踪或不追踪库存 | catalog-core、product-editor；[#16](https://github.com/humaux/live_commerce_architecture_v1/pull/16)、[#28](https://github.com/humaux/live_commerce_architecture_v1/pull/28)；[storefront-v2 §A](../../contracts/storefront-v2.md)、[catalog-inventory A6](../../contracts/catalog-inventory-v1.md) | **MOCK**：C2 / `--browser-catalog-core`、`--browser-product-editor`；C3 验证新增矩阵台账。issue #45 未关闭，不能宣称每个规格控件已完整点击。 |
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
| 从控台/Inbox 评论直接打开建单抽屉（LC-U3） | [#32](https://github.com/humaux/live_commerce_architecture_v1/pull/32) 在本次查询时 **OPEN**；[live-console §5](../../contracts/live-console-v1.md) | **NOT_RUN（当前候选交付）**：未合入此快照；不将该 PR 的开发状态计为候选功能。 |

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
| 平台信用卡启用/停用、买家披露、按店结算单与明细 | W4-S1、W4-S2、[W4-U1](units/w4-u1-payment-activation-ui.md)、结算归属修复 [#4](https://github.com/humaux/live_commerce_architecture_v1/pull/4)；[stripe-platform-account §§0.3、3–6](../../contracts/stripe-platform-account-v1.md) | **MOCK**：C2 / `--browser-card-payments`；C1 的平台收款/结算 PG 测试。仅 allowlist 的同法人自有店可启用；第三方商户不开放。 |
| Stripe 捕获、退款、UNKNOWN 查询对账与金额/目录漂移保护 | stripe-b1/b2、refund-core；[#20](https://github.com/humaux/live_commerce_architecture_v1/pull/20)；[stripe-psp §§9、14](../../contracts/stripe-psp-v1.md)、[stripe-refund §§5–9](../../contracts/stripe-refund-v1.md) | **MOCK**：C2 / `--browser-refund-fulfilment`、`--browser-e2e`；C1 的交易 PG 测试。真实 Stripe Checkout/3DS/webhook/退款的 **SANDBOX、LIVE 均 NOT_RUN（本报告）**。`--stripe-browser` 不在无凭据的普通 PR CI 集合。 |

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
| 商品/渠道/漏斗/手工单四类报表、已应用日期、导出 CSV | W6-02B、W6-U1 [#3](https://github.com/humaux/live_commerce_architecture_v1/pull/3)、表格溢出修复 [#15](https://github.com/humaux/live_commerce_architecture_v1/pull/15)；[reporting-v2 §§1–6](../../contracts/reporting-v2.md) | **MOCK**：C2 / `--browser-reports`；按币种/支付环境分开，不是利润、成本或自动定时报表。 |

### 广告（ads）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 广告账户连接、草稿、批准、发布/暂停/复制 | ads-core/graph/ui；[meta-ads §§2、4–8](../../contracts/meta-ads-v1.md) | **MOCK**：C2 / `--browser-meta-ads`，假 OAuth/Graph + 真实 worker/PG。真实投放/预算消耗 **LIVE NOT_RUN**。 |
| 成效、归因、Insights/CAPI、金额显示 | ads-capi、ads-attribution；[#43](https://github.com/humaux/live_commerce_architecture_v1/pull/43)；[meta-ads §§6.2、6.4、6.5](../../contracts/meta-ads-v1.md) | **MOCK**：C2 / `--browser-meta-ads`、`--browser-ads-attribution`；供应商归因/insights 的 SANDBOX/LIVE 不由假 Graph 证明。 |
| 广告解绑、feed URL；失败/UNKNOWN 操作台账、取消/查询/受限重试 | W6-05B、W6-06B、[W6-U2](units/w6-u2-ops-ledger-ads-ui.md)；[external-operation W6-05B amendment](../../contracts/external-operation-v1.md) | **MOCK**：C2 / `--browser-operations-ads`；C1 的操作台账 PG 测试。UNKNOWN 必须先对账，不允许盲重试。 |

### 店铺前台（storefront）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 首页、集合筛选/排序/分页、商品规格/库存/SEO、购物车、买家订单/通知页面 | storefront-shell/integration、buyer-comms；[storefront-v2 §§A–E](../../contracts/storefront-v2.md) | **MOCK**：C2 / `--browser-storefront`、`--browser-buyer`、`--browser-buyer-comms`；没有真实邮箱投递或真实卡支付证明。 |
| 外观/导航/首页区块/信息页、预览、发布/取消发布/回滚、自定义域名流程 | store-design、storefront-publish、store-domains；[storefront-v2 §B](../../contracts/storefront-v2.md)、[published-storefront-resolver Writer (R3)](../../contracts/published-storefront-resolver-v1.md) | **MOCK**：C2 / `--browser-design`、`--browser-storefront-publish`、`--browser-store-domains`。真实 DNS、证书、公开域名路由 **NOT_RUN（本报告）**。 |
| 页面覆盖、实际点击与视觉规则 | [#22](https://github.com/humaux/live_commerce_architecture_v1/pull/22)、[#23](https://github.com/humaux/live_commerce_architecture_v1/pull/23)、[#39](https://github.com/humaux/live_commerce_architecture_v1/pull/39)、[#43](https://github.com/humaux/live_commerce_architecture_v1/pull/43)；[GATES 的 sharding/PR gates](GATES.md) | **MOCK**：C2 / click-sweep ×10、visual-lint ×4 和 aggregate 成功。自动规则不是全页面人工视觉签字；issue #45 仍限制商品控件完整性。 |

### 直播场次（studio）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 场次列表、创建/编辑/重开，BFF 与权限状态 | studio-backend、studio-ui；[studio-v1 Planning、Merchant API](../../contracts/studio-v1.md) | **MOCK**：C2 / `--browser-studio-bff`、`--browser-studio-ui`；planning 与媒体运行分离。 |
| 输入 token 传输、浏览器输入与 MOCK rehearsal 路径 | input-delivery、studio G2；[studio-v1 G2 amendment](../../contracts/studio-v1.md) | **MOCK**：C2 / `--browser-input-delivery`、`--live-browser-input`。真实 SFU 解码、LiveKit Cloud、目标平台可见开播 **NOT_RUN**；R1 的媒体开关限制仍按 [deploy 范围](../../deploy/README.md#known-limits-recorded-by-the-r1-deploy-unit)处理。 |

### 开店设置（settings）

| 能力 | 已合入单元 / PR；契约来源 | 证据等级与 CI |
| --- | --- | --- |
| 商家身份/开店向导、支付/物流配置与能力状态 | merchant-settings、[#14](https://github.com/humaux/live_commerce_architecture_v1/pull/14)；[merchant-settings-http Routes / No capability upgrade](../../contracts/merchant-settings-http-v1.md)、[merchant-service-settings §§2–5](../../contracts/merchant-service-settings-v1.md) | **MOCK**：C2 / `--browser-identity`、`--browser-password-auth`、`--browser-customers-billing`。配置成功不能说明供应商 LIVE 已开通。 |
| 平台订阅套餐/用量、欠费提醒、订阅 Checkout 与客户门户入口 | billing-core、customers-billing-ui；[customers-billing §0 BD1–BD6、§5 Admin screens](../../contracts/customers-billing-v1.md) | **MOCK**：C2 / `--browser-customers-billing`（托管 Checkout/portal 重定向由测试拦截）。欠费不阻断既有买家结账、退款或履约；真实平台订阅扣款、发票/门户 **SANDBOX / LIVE NOT_RUN**。 |
| Facebook/Instagram 自助连接/断开、重连提示、读留言/私讯/公开回复状态与“仅测试账号”限制 | meta-connect、W1-01B、W1-01U；[meta-claims-intake Merchant connect amendment](../../contracts/meta-claims-intake-v1.md)、[meta-connection-health §§3、5、7、11](../../contracts/meta-connection-health-v1.md) | **MOCK**：C2 / `--browser-meta-connect`、`--browser-meta-health-ui`；C1 的连接/健康 PG 测试。MCH12 LIVE_READ 与非应用角色授权 **NOT_RUN（本报告）**。 |

## 已知缺口与 FOLLOWUPS

来源是集成方 `output/integrator/FOLLOWUPS.md` 的 2026-10-13 工作记录及其中的 PR/thread；该文件不是已提交的 release ledger，包含已修复旧条目。下表按主题摘要，**不把“被记录”写成“已完成”**；状态变化须用新 PR/CI 收据关闭。数据完整性或隐私问题按 [PROCESS §2](PROCESS.md#2-per-unit-software-engineering-flow) 与集成方收敛裁决提升处理优先级。

| 缺口 / 后续工作 | 可核对来源与状态 |
| --- | --- |
| 商品编辑器控件完整性：删轴/删规格值按钮尚缺完整 inventory、390px 点击、回读与台账；Go/Node 参数别名、env 前缀、实际执行绑定、批量操作选项、未知 JSX 控件还有硬化项 | **OPEN [issue #45](https://github.com/humaux/live_commerce_architecture_v1/issues/45)**，P1 thread `4254703909`；owner 批准 #28 先合入不代表缺口消失。跟进 [#46](https://github.com/humaux/live_commerce_architecture_v1/pull/46)、[#47](https://github.com/humaux/live_commerce_architecture_v1/pull/47) 在快照查询时均 OPEN。 |
| LC-U3 建单抽屉还未计入此候选；提醒付款链接重生的关联范围、Studio read-only 测试断言另有后续项 | [#32](https://github.com/humaux/live_commerce_architecture_v1/pull/32) OPEN；FOLLOWUPS “#32 LC-U3 delta”及 [live-console §5](../../contracts/live-console-v1.md)。 |
| Inbox/评论：前台列表与未读更新、历史清除恢复失败、651–1150px 买家面板布局、窄屏键盘焦点、打开后锚底/新消息提示；长列表客户关联/付款链接来源恢复有上限 | [#8](https://github.com/humaux/live_commerce_architecture_v1/pull/8) threads `4212543434/3438/3445/3452/3457`；origin 扫描 `4209444305`；[#12](https://github.com/humaux/live_commerce_architecture_v1/pull/12) link-pending 先取50条后过滤场次的记录。此报告未取得对应后续修复 CI，不宣称关闭。 |
| 高流量 IG：同微秒 seq 游标、>1000 原始事件重建预算、刷新游标回退、HEAD 与增量窗口对齐；bridge 响应体上限、Strict Effects/backoff/UNKNOWN 恢复仍有后续 | [#18](https://github.com/humaux/live_commerce_architecture_v1/pull/18) FOLLOWUPS，threads `4226524062/4067/4071`、`4227138850`、`4230211367/1379`；[live-console §§2–4](../../contracts/live-console-v1.md)。不声称无限量会话或所有故障已验证。 |
| 客户/包裹/控台 UX：长标签目录定位、长备注就近编辑、重复包裹组显示/逐项 busy/运单错误定位、库存保存后旧值闪回、IG 提示误用于 FB | [#3](https://github.com/humaux/live_commerce_architecture_v1/pull/3) `4212542070/2075`；[#2](https://github.com/humaux/live_commerce_architecture_v1/pull/2) `4212524352/4369/4378`；[#5](https://github.com/humaux/live_commerce_architecture_v1/pull/5) `4212525437`及 K3 后续。 |
| 商品移动端 UX/视觉硬化：同名可访问标签、跨900px切换后焦点、TabStrip 每次重绘回卷、提示高度变化；内滚动截图只留像素，布局规则只测 pane 顶部 | [#43](https://github.com/humaux/live_commerce_architecture_v1/pull/43) FOLLOWUPS（91b9a6a8）；金额整值省略小数需 owner 确认统一风格。 |
| 证据/CI 硬化：evidence-root/env override 与递归 symlink、未列入 manifest 的文件/原始哈希可核性与重钉程序、CI-SELECT 删除覆盖守卫、CI-DRIFT 的 shadow/computed key/template 插值分析 | [#23](https://github.com/humaux/live_commerce_architecture_v1/pull/23) `4228878459/8471/4230486608`；[#43](https://github.com/humaux/live_commerce_architecture_v1/pull/43) r2；[#39](https://github.com/humaux/live_commerce_architecture_v1/pull/39) r4/r7；[#33](https://github.com/humaux/live_commerce_architecture_v1/pull/33) `4233809775/4234857444/4234857453`。日志完整性不能只靠作者自证。 |
| 测试矩阵与不稳定项：导入编码拒绝的 zh-CN 浏览器矩阵、长套件分片、非共享 helper 的端口交接、WebKit apt 观察、90秒媒体恢复/密码 rate-limit 时钟边界 | FOLLOWUPS：W5-U1 K3、[#37](https://github.com/humaux/live_commerce_architecture_v1/pull/37)、[#27](https://github.com/humaux/live_commerce_architecture_v1/pull/27)、[#43](https://github.com/humaux/live_commerce_architecture_v1/pull/43) run `38293593095`、[#44](https://github.com/humaux/live_commerce_architecture_v1/pull/44) run `38293840385`。当前成功结论含重跑，不证明不再有 flake。 |
| 商品退回/物流与结算限制：卡付超商退回的补库存需人工处理；无跨境 first-mile/海关验收；结算分配不等于打款，没有自动银行拨付；争议靠同步入账 | [returns-v1 §7a](../../contracts/returns-v1.md)、[taiwan-cvs §13](../../contracts/taiwan-cvs-logistics-v1.md)、[stripe-platform-account §9](../../contracts/stripe-platform-account-v1.md)。 |
| 报表与广告范围：无 profit/COGS、历史导入单/定时报表；漏斗 link_sent 只计 private_reply；无真实 Insights/CAPI/广告预算实时硬停止保证 | [reporting-v2 §§1、4](../../contracts/reporting-v2.md)、[meta-ads §12](../../contracts/meta-ads-v1.md)。 |

FOLLOWUPS 中以下旧问题已有合入修复，不能继续当成“未实现”：报表溢出/时区 fallback/订单等待 [#15](https://github.com/humaux/live_commerce_architecture_v1/pull/15)，购物车刷新丢旧读 [#25](https://github.com/humaux/live_commerce_architecture_v1/pull/25)，BO01 竞态 [#26](https://github.com/humaux/live_commerce_architecture_v1/pull/26)，打印事实/收据/关联保留期清理 [#35](https://github.com/humaux/live_commerce_architecture_v1/pull/35)，模式 registry [#22](https://github.com/humaux/live_commerce_architecture_v1/pull/22)，CI 后端覆盖/夜间全量 [#39](https://github.com/humaux/live_commerce_architecture_v1/pull/39)，手工单再生成/native CDP [#42](https://github.com/humaux/live_commerce_architecture_v1/pull/42)，店铺 collection chips 与 MOU03 关闭页签 [#44](https://github.com/humaux/live_commerce_architecture_v1/pull/44)。修复后的残余硬化或 flake 观察仍按上表保留；没有用它们的本地收据验收。

## 上线前 owner 要完成的步骤

| 步骤 | 所需记录 / 放行条件 | 本报告状态与来源 |
| --- | --- | --- |
| Meta LIVE 只读探针 | owner 批准测试资产/账号/范围；记录 Graph 版本、权限/任务与脱敏响应形状、时间、SHA、HTTP 结果。只读成功不能证明发送、广告写入或非角色用户可用；涉及真实 OAuth/撤权/重连的扩展步骤须另取明确授权 | **NOT_RUN**；[meta-connection-health §11.3 MCH12](../../contracts/meta-connection-health-v1.md)、[live-console §13.3](../../contracts/live-console-v1.md)、[Meta 接入 runbook §6.3/6.7](../runbooks/deploy.md)。不得把含 UI 变更的整套 §11.3 默认为纯只读授权。 |
| App Review / Access Tier 提交及通过 | owner 提交准确权限、业务验证与演示材料，记录 submission/result；仅在实际批准后更改 Advanced Access 配置并由 CI/授权探针补证；未通过保持“仅测试账号” | **NOT_RUN**；[meta-connection-health §13 MCH-OPEN-5](../../contracts/meta-connection-health-v1.md)、[merchant-onboarding §0/4.1](../runbooks/merchant-onboarding.md)。工程 MOCK 不等于 Meta 审核通过。 |
| 生产部署批准 | owner 明确 tag/SHA、店铺/功能范围、窗口、回滚与数据库备份/恢复安排；候选 release-gate CI、部署 smoke CI 和独立复核结果齐备后，仍需本次批准才可动生产 | **NOT_RUN／RCG 待追加**；[deploy runbook §§0、3、5、10](../runbooks/deploy.md)、[PROCESS §2](PROCESS.md)。本报告不核定已有试点主机的生产状态。 |
| 密钥轮换与保管确认 | owner/获授权运维按密钥 manifest 与 runbook 轮换，保存脱敏审计和恢复副本；keyring 只追加新 key/切 active，未重新密封前保留旧 key；BFF 两端协调；cookie、actor/replay/link key 的影响分别评估 | **NOT_RUN（实际轮换）**；[deploy runbook §7](../runbooks/deploy.md#7-密钥轮换按-deploysecretsmanifesttsv-的-rotation-列)、[secrets manifest](../../deploy/secrets.manifest.tsv)。CI 临时凭据不代表生产凭据已轮换。 |
| 收款/物流/真实消息的独立开通 | 确认同法人平台 Stripe allowlist；SANDBOX 凭据/回调验收与 LIVE 审批、额度/停收/对账分开记录；真实 canary 付款/退款 owner 亲自做；承运商合同、ECPay LIVE 出单另批 | **SANDBOX / LIVE 均 NOT_RUN（本报告）**；[stripe-platform-account AD-PF2 / §7](../../contracts/stripe-platform-account-v1.md)、[stripe-live runbook §§0–2](../runbooks/stripe-live.md)、[deploy runbook §6.6](../runbooks/deploy.md)。第三方商户不能以平台账户收款。 |

上述是需要补齐的 owner 记录，不是让 agent 在本机执行的命令清单。按 2026-10-13 指令，**全部测试、服务器运行及容器验证只在 GitHub CI**；生产操作属于另一次明确授权。报告撰写未启动服务、读取 live key、触及真实客户数据或进行平台变更。

## 保留为 owner-only 的红线

| 动作 | 边界及来源 |
| --- | --- |
| 真钱扣款/退款、LIVE 密钥与 live-canary、平台店铺 allowlist/停收策略 | 本次文档/CI授权不包含这些动作；owner 定范围并批准，真实 canary 由 owner 操作。[stripe-live runbook §0/步骤6](../runbooks/stripe-live.md)、[stripe-platform-account AD-PF2/§7](../../contracts/stripe-platform-account-v1.md)。 |
| 正式部署、真实数据删除、破坏性迁移/生产恢复、密码/密钥轮换 | 先批准影响面及回滚/备份，再交获授权运维执行；不得拿文档 PR 合并当生产批准。[AGENTS](../../AGENTS.md)、[deploy runbook §§5、7、10](../runbooks/deploy.md)。 |
| 真实广告发布/消耗预算、真实客户私讯/公开回复、营销重播、购买物流面单 | CI 只使用合成数据及假平台；每项真实动作仍需明确批准，权限/同意/消息窗口不可跳过。[AGENTS](../../AGENTS.md)、[meta-ads §11](../../contracts/meta-ads-v1.md)、[live-console §§3–4](../../contracts/live-console-v1.md)、[deploy runbook §6.6](../runbooks/deploy.md)。 |
| Meta App Review、商家/平台合同与商业资格、生产域名/主机/备份方案 | owner 负责真实资质与批准；agent 不补造资格，也不收集 FB/IG 密码或绕过官方权限。[PROCESS §1](PROCESS.md#1-release-plan)、[merchant-onboarding §0](../runbooks/merchant-onboarding.md)、[AGENTS](../../AGENTS.md)。 |

## 待追加：release-gate CI 收据

集成方后续在此追加：**候选 SHA、GitHub run URL/ID、完整 gate/tier 列表、PASS/FAIL/NOT_RUN、供应商环境、artifact 位置、独立审查结论、owner 批准引用**。当前留空的结论为 **NOT_RUN**；本报告不包含“可正式上线”或生产 LIVE 验收结论。[deploy-release 单元第4项](units/deploy-release.md)、[PROCESS §4](PROCESS.md#4-evidence-labels-agentsmd)。
