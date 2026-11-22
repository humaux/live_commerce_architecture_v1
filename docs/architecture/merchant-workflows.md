# 直播电商平台的业务边界、端口与事务

**业务边界与实施约束 · DESIGN · 2026-11-22。** 以一次交易为线索划界：商家决定卖什么，店面呈现报价，交易接受订单，库存承诺数量，支付确认资金，履约交付包裹。直播与消息提供成交入口，客户同意决定信息可用于哪些目的。平台订阅决定商家使用 SaaS 的权益。商家后台、买家店面、工作进程分别消费这些能力；换入口不改变事实的写入者。

正式源 **I** 为 `12c60ea6eff3ca6a7f5c0c4d2e83702aeb3cd8f2`，试点源 **P** 为 `a2d079cac8004fb822705568c4b7d4e6a61b27c4`。两树差异只涉及七个 CI、验证脚本和文档文件，业务源码、迁移及契约一致。I 增加的 `docs/FEATURE_MAP.md` 导航到两端真实入口，`scripts/dev/verify` 选择本地轻量检查。P 没有这两个入口。以 `架构.md` §0–2、26–27 和 `contracts/invariants.json` 确定基线，以对应 SHA 下实际组合根、SQL 和业务端点判断实现。历史浏览器回执不能移植到 I。

## 系统边界与选型

保留一个 Go 模块化单体，PostgreSQL 18 承担交易与库存唯一真源。Next.js/TypeScript strict 负责浏览器交互和 BFF。API 在 `cmd/api` 装配按店授权的入站适配器，worker 按数据库角色和队列隔离出站权限。River 只负责持久调度；语义键、命令回执与供应商结果由业务表保留。领域规则可由 Go 用例或拥有写权限的 SQL 函数实现，事务由调用方传入的 pgx.Tx 或受限数据库函数维持。六边形在授权、事务及外部 I/O 边界落地，不要求每张表增设 repository 接口。

下表按事实和动作列出归属。每行列出代表性物理表和真实源文件。箭头从消费方向能力提供方读；“端口”包括已存在的 Go 方法、SQL 命令与查询，表名的 schema 不能单独决定写权限。报告中的路径以仓库根为起点。

|上下文及业务事实|物理表（代表性）|入站适配器 → 应用端口|出站适配器、依赖方向与事务界面|
|---|---|---|---|
|商家身份与店铺授权。会话、员工授权、近期认证决定可操作的店及动作| `identity.sessions`、`identity.store_grants`、`identity.memberships`、`control.stores` | `internal/identityhttp/handler.go` → Start / Complete / CreateInitialStore；密码与再认证走 `internal/identity/reauth.go` | `internal/identity/service.go` → OIDC Provider（`internal/oidclogin/oidclogin.go`）及 PG 事务。首次开店由 `internal/quickstart` 编排店铺初始设置及原有命令。敏感写在实际命令中核近期认证；浏览器选店不授权。|
|买家身份与访问权。持有人、店、订单及链接范围| `buyer.owners`、`buyer.capability_sessions`、`checkout.order_links` | `internal/buyerhttp/orderlink.go`、`internal/buyerhttp/handler.go` → 兑换链接、按买家范围读写 | 买家 capability 的 PG 适配器位于 `internal/buyer`；结账在写事务内复查同店 capability。商家员工会话与买家 cookie 分属不同入口。|
|商品目录。商品、规格、标价、组合、媒体角色和可见性| `catalog.products`、`catalog.skus`、`catalog.price_history`、`catalog.collections`、`catalog.product_images` | `internal/httpapi/handler.go`、`internal/httpapi/images.go` → 商品文档保存、媒体命令；`internal/buyerhttp/catalogv2.go` → 在售查询 | `internal/catalog/document.go` 用同一命令事务保存 SKU、集合、审计及可选期初库存。媒体上传另有回执。图片与直播流媒体有不同生命周期。|
|价格与促销。配送计价政策、折扣可用性和核销| `pricing.policy_versions`、`pricing.policy_heads`、`promotions.codes`、`promotions.redemptions` | `internal/httpapi/promotions.go` → 维护规则；店面 quote 消费价格策略 | `internal/pricing/pricing.go` 计算，`internal/promotions/promotions.go` 校验/核销，`internal/storefront/quote.go` 持久报价。提交订单重验冻结条件及金额；前端展示服务端报价。|
|库存。仓库、实物、可售、保留及分配| `inventory.warehouses`、`inventory.ledger`、`inventory.balances`、`inventory.reservations`、`inventory.reservation_lines` | 库存调整经 `internal/httpapi/handler.go` → `internal/inventory/inventory.go`；订单命令 → checkout.begin_hold | PG 账本与余额更新在同一事务；订单保留跟随建单事务，撤销、到期及退货按各自动作释放。不得由读模型或缓存写第二份可售量。|
|购物车、报价、建单与订单。草稿可改，提交冻结商品行、配送、币种及总额| `storefront.carts`、`storefront.quotes`、`storefront.destination_snapshots`、`checkout.orders`、`checkout.command_results`、`ops.order_attempts` | 买家 `internal/buyerhttp/handler.go` 与代客 `internal/merchanttools/manual.go` → checkout.Begin；冻结计划另有 `internal/orderattempt/executor.go` | `internal/checkout/checkout.go` → pgx、River InsertTx、库存/履约及直播价，见下文。商家读单由 `internal/merchantorders/orders.go` 投影；attempt 切换在具体部署的配置 UNKNOWN。|
|收款、退款与对账。可信到账事实、退款事实及待复查状态| `checkout.payment_attempts`、`payments.provider_observations`、`payments.facts`、`payments.stripe_refunds`、`payments.refund_facts` | `internal/buyerhttp/payment.go`、`internal/payments/stripewebhook/handler.go`、`internal/merchantorders/refunds.go` → 启动付款、接收信号、授权退款 | `internal/integrations/psp/stripe/client.go` 对接 PSP，`internal/payments/query_worker.go` 查证，`internal/payments/capture_worker.go` 仅在 PG 事务中应用已持久化观测。商家端卡款开关归 `internal/payments/platformstripe`，`cmd/payment-worker` 承接支付任务。卡款平台代收按 `contracts/stripe-platform-account-v1.md` 逐店归属；历史 PAYUNi 表不能证明现行提供该方式。|
|履约配送。服务、分仓、运费、包裹、手工出货与超商寄件| `fulfillment.service_versions`、`fulfillment.allocation_heads`、`fulfillment.manual_shipment_versions`、`fulfillment.cvs_shipments`、`fulfillment.parcel_groups` | `internal/httpapi/shipments.go` → `internal/merchantorders/shipments.go`；`internal/fulfillment/cvs.go` 接收门市选择/寄件命令 | PG 写履约版本与订单履约状态。供应商模式持久寄件及任务，worker 的 `internal/integrations/shipping/ecpay/ecpayroute/route.go` 对接绿界；手工模式仅记录商家动作。配送完成和款项收妥分别取证。|
|售后。买家申请与 RMA 分两种事实，审批、签收、质检、关闭各有前置状态| `returns.buyer_requests`、`returns.buyer_request_lines`、`returns.buyer_request_receipts`、`returns.rmas`、`returns.rma_lines` | `internal/buyerhttp/return_requests.go`、`internal/httpapi/return_requests.go` → `internal/returns/request.go`；`internal/httpapi/returns.go` → RMA 命令 | `migrations/0221_buyer_return_requests.sql` 中 approve 同事务登记 RMA 并关联申请；`migrations/0155_returns.sql` 约束收货/回库。退款另走授权支付命令。|
|开播与媒体。场次、节目、目的地、执行及恢复见证| `live.sessions`、`live.programs`、`live.media_attempts`、`live.media_observations`、`live.replay_videos` | `internal/httpapi/studio.go` → `internal/live/lifecycle.go`；媒体执行消费 River 媒体队列 | `cmd/media-worker` → `internal/live/media_execution.go` → `internal/integrations/livekit/client.go`；PG 保存计划、执行阶段及观测，跨网络后按 `internal/live/media_recovery.go` 恢复。保存开播计划、回放链接或收到 start ACK 均不能证明用户收到媒体。|
|直播认领。关键字、开窗、来源事件、认领行与兑换资格| `live.offers`、`live.claim_windows`、`claims.meta_intake`、`claims.events`、`claims.bundles`、`claims.lines` | `internal/claimsintake/poller.go` → `internal/claims/ingest.go`；买家 `internal/buyerhttp/claims.go` → 兑换 | 租约取 intake，按场次、来源和规则落 PG；`internal/claims/buyer.go` 对接店面购物车，结账消费直播价。认领成功与物理保留、支付成功分开显示。|
|消息与通知。社交通道收件、会话窗口、模板、发送计划及投递结果| `meta_inbox.events`、`social.conversations`、`social.messages`、`inbox.outbound_messages`、`msgtemplates.templates`、`notify.outbox`、`storefront.store_contact_messages`、`storefront.store_contact_throttle` | `internal/integrations/meta/handler.go` → 入站收件；`internal/httpapi/inbox.go` → 读会话；`internal/inbox/send.go` → 人工发送计划 | 计划与 River job 同店同事务；`internal/integrations/metareply/routes.go` 在 worker 执行。邮件另由 `internal/notify/worker.go` 领取、`internal/mail/smtp.go` 投递并记结果；模板保存不授予发送资格。站内咨询从 `internal/buyerhttp/contact.go` 提交，由 `internal/httpapi/contact.go` 按 inbox:read / inbox:reply 读取或标为已处理。|
|客户、导入及隐私。客户关系、标签/备注、按用途的同意、导出/删除处理| `customers.notes`、`customers.owner_tags`、`customers.consent_events`、`customers.privacy_actions`、`migrationimport.batches` | `internal/httpapi/customers.go`、`internal/buyerhttp/privacy.go` → 客户读模型、撤回及买家隐私动作；CSV 经 `internal/migrationimport/batch.go` | `internal/customers/privacy.go` → 按店授权 PG 函数；`internal/retention/job.go` 消费保留策略；`internal/migrationimport/customers.go` 以批次处理历史资料。资料导入与账号合并、营销同意分别授权。|
|会员积分。商家维护的会员和只追加点数账本| `loyalty.members`、`loyalty.point_entries` | `internal/httpapi/members.go` → 建会员、调点；代客订单可显式赠点 | `internal/loyalty/members.go` → loyalty.adjust_points；PG 检查权限、锁余额及防负值，订单关联维持可追溯。首版手工赠扣，不推断会员身份或兑换权益。|
|店面设计、发布和域名。草稿、已发布版本及可信主机映射| `design.documents`、`design.published_versions`、`design.preview_tokens`、`control.storefront_publications`、`control.storefront_domains` | `internal/httpapi/design.go` → SaveDraft / Publish / Rollback；店面 SSR 消费发布版本；`internal/tlsask/tlsask.go` 控制域名资格 | `internal/design/store.go` 在 PG 锁店和草稿后产生版本；`internal/storefrontdomains/verify.go` 将 Resolver / TLSProber 接到 SystemResolver / SystemProber，验证 DNS 和 TLS 后记录观测。草稿预览、已发布、已可提供 HTTPS 各自读回。|
|广告、促销投放与归因。账户、预算审批、广告草稿、订单触点及外部购买事件| `ads.connections`、`ads.campaign_drafts`、`ads.draft_approvals`、`ads.capi_events`、`ads.insights_daily`、`orders.order_attribution` | `internal/httpapi/ads.go` → `internal/ads/service.go`，worker 扫描已批准步骤；购买测量消费订单及同意事实 | `internal/integrations/meta_ads/routes.go` 对接 Graph；`internal/attribution/capiroute/route.go` 装配 Check/LoadSecret/Dispatch/Reconcile。订单内冻结来源；另一次派发重新检查授权、用途、租约和 token。真实广告资格、归因覆盖率 UNKNOWN。|
|财务投影与逐店结算。按环境币种看收退净额，生成对账、关账、拨付记录| `payments.facts`、`payments.refund_facts`（只读来源）、`payments.settlement_statements`、`payments.settlement_lines`、`payments.settlement_payout_returns` | `internal/httpapi/finance.go` → `internal/reporting/finance.go`；商家结算查询经 `internal/httpapi/settlements.go` | `internal/payments/settlement/service.go` 读本店明细；`internal/payments/stripeadmin/settlement.go` 是操作员端记录、关账等命令。PG 投影隔离币种及环境，代收/转账单列；已出结算单与已拨付分别记录。|
|SaaS 订阅。平台客户、店铺订阅和欠费权益| `billing.store_customers`、`billing.subscriptions` | `internal/httpapi/billing.go` → 订阅操作；`internal/billing/webhook.go` 接受订阅事件 | `internal/billing/stripe.go` 在数据库事务外读取 Stripe 订阅；入站角色单次调用 billing.apply_subscription 落 PG，`internal/billing/standing.go` 读权益。店铺销售入账不能抵 SaaS 欠费。|
|集成与运维。商户绑定、能力、不可变操作、尝试、审计与恢复| `integration.bindings`、`integration.binding_capabilities`、`integration.operations`、`integration.operation_events`、`ops.audit_events` | `cmd/claims-worker`、`cmd/ads-worker` → River → `internal/integrations/core/dispatcher.go`；`cmd/migrate` 执行带校验的迁移 | `internal/integrations/core/service.go` → PG 租约及操作回执；已提交 claim 后才按注册 route 派发；过期 dispatch、ACKNOWLEDGED 和 UNKNOWN 只许按原操作查证。`internal/metaconnect` 管理官方 Page 绑定。Meta 加密原件在 `meta_private.event_bodies`、`meta_private.quarantine_bodies`、`meta_private.raw_bodies`，`cmd/meta-worker` 消费入站任务。worker 权限、队列、备份恢复各有门禁；真实恢复时长 UNKNOWN。|

## 三处不能随包拆开的调用

1. **商品创建和期初库存。** `internal/catalog/document.go` 在保存新 SKU 的事务中，仅对 tracked 且非零的期初数量调用 writeOpeningStock；其后保存集合及审计。商品归 catalog，变动走库存账本；若改成提交商品后再发库存 RPC，则第二步失败可留下有价无期初量的 SKU。保留同库调用，将来抽接口也必须接受同一 tx。
2. **建单、库存与认领价。** `internal/checkout/checkout.go` 的 Begin 在同一 tx 中 InsertTx 到期任务、调用 checkout.begin_hold、ConsumeLivePrices，随后执行 orders.freeze_attribution、记录本单联系邮箱／语言，并在有优惠时调用 `promotions.Redeem`。优惠核销失败时，建单、保留、认领价和任务同事务回滚。冻结直播价被别单用尽时，订单、保留、任务与回执一起回滚。外部查询与支付派发在事务外，不能用“调用过 PSP”替代 payments.facts。过期任务由 `cmd/expiry-worker` 消费。代客 attempt 另由执行 fence 协调；不可把旧建单与新 attempt 连续调用作为超时恢复。
3. **从客户同意到 CAPI。** `internal/attribution/attribution.go` 在买家同意事务内记录上下文；`internal/attribution/capiroute/route.go` 的 Check 调 ads.check_capi，LoadSecret 在同一租约下取绑定 token 与获准的用户数据，随后由 Graph 适配器发送。取消同意或解绑发生在排队之后仍须阻断。端到端独验需覆盖“排队→撤回→worker”且供应商请求数为零。

## 缺口怎样派卡

**先恢复买家售后申请的商家入口。** I 的 `internal/httpapi/return_requests.go` 已注册 list/detail、approve/reject。读要求 orders:read，审批要求 fulfillment:write、expected_version 与幂等键。现有商家 `apps/admin/lib/returns-client.ts` 和 `apps/admin/components/ReturnsList.tsx` 处理 RMA；在两固定树的 apps/admin 中精确搜 return-requests 无命中。故“显示最近申请并审批”需补 BFF 适配器和 UI，沿用领域写端口。若产品要承诺“全店所有待处理”，须扩充 **查询端口**：当前 HTTP 拒绝 query，`migrations/0221_buyer_return_requests.sql` 的列表按 created_at DESC、id 排序，LIMIT 100。审批能力足够，重写状态机只会产生第二个申请来源。

**仅展示的改动。** 手工单和买家结账按宅配/超商分组、显示服务端报价与代收提示，P 的旧现场已读回。旧现场的“取消换店”是买家取消更换 7‑ELEVEN 取件门市。两树 `apps/storefront/components/CvsPickup.tsx` 中 change 路径先 setStore(null)，onClose 只关闭弹窗。建议在同一配送方式内只在确认新门市时替换已确认对象；取消时保留原名称、地址、门市码，重新报价依既有服务端规则。该 SHA 的实际浏览器行为待验。对账后需要给用户展示“结果待确认”时，先基于原操作查询和当前授权，不能向前端暴露“重新创建”捷径。

**证据缺口。** 真实商家 Meta 授权、Media Egress 成功覆盖、支付/物流实户资格、DNS 首签续期与恢复能力需要各自的 SANDBOX/LIVE 回执。目录中已有适配器时，先补相应证据；现阶段没有据以扩建统一供应商平台的事实。

## 商家操作怎样穿过边界

| 用户要做的事 | 入口与调用方向 | 谁能确认结果 |
|---|---|---|
| 开店后新增商品 | 商品列表 → 新增商品 → 商品文档命令。建店结束即回工作台，首件和第十件走同一编辑器 | Catalog 回读草稿状态，期初存量由 Inventory 同事务入账 |
| 改某项配送费用 | 设置 → 配送与运费 → 服务 → 金额及免运门槛。承运商连接属于单独的集成管理 | Pricing 按服务和版本保存，买家新报价按金额及币种计算。历史订单保留冻结值 |
| 切店后盘点 | 库存导航、SKU 选择、调整回执和台账贯穿同一已授权店和仓库 | 服务端认证选店，Inventory 负责写和台账。仓储角色只取完成盘点必需的 SKU 信息 |
| 查单后继续购物 | 订单链接或访客查单只进入获准订单，普通购物身份可恢复 | Buyer 验证访问范围，Storefront 管购物车，订单链接不授予查全部订单、售后或隐私的资格 |
| 处理买家申请或咨询 | 售后队列 → 申请详情 → 批准或拒绝。收件匣 → 商店询问 → 标为已处理 | Returns 保存申请和 RMA 关联；商店询问使用现有 contact-messages 命令。客服操作不产生退款或营销发送 |
| 直播后发布回放 | 已结束场次 → 有权限的发布操作 → 店面回放索引 → 场次详情 | Live 校验视频、权限和版本，买家读已发布视图；撤回后详情随之失效 |
| 核对代收 | 订单列表、详情与财务读相同订单的独立收款事实 | 履约登记取付或货到收款，Payments 按可信资金事实记账，报表按环境和币种汇总 |

导航按商家的工作组织。后台页面和 BFF 从已验证的店、角色和命令回执生成展示。界面改名或隐藏入口时，旧地址仍需保留其原有授权、选店和未完成命令的恢复能力。已发出的外部命令只能用原语义键查证。

本期交付以现有单体为单位。只有出现可量测的容量瓶颈、独立消费者或故障隔离要求时才评估拆分部署。逐项验收和允许写入的文件见[实施任务卡](../delivery/units/store-workflows/TASKS.md)，应用和供应商资格依[现有发布门禁](../delivery/GATES.md)执行。
