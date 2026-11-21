# 核心功能地图

以下路由以 `/{locale}` 为前缀，买家在店面域，商家在后台域。使用隔离店铺与测试资料，按角色及店铺配置执行。
正常判据用于浏览器点击验收。以下为源码导航，浏览器、真实供应商和生产执行状态均为 NOT_RUN。

## 买家

### 买家 · 商品发现 · /、/products、/search、/collections/[slug]
- 操作：搜索有库存商品，打开集合，再进入商品详情。
- 前端 `apps/storefront/app/[locale]/page.tsx`、`apps/storefront/app/[locale]/products/(list)/page.tsx`、`apps/storefront/app/[locale]/search/page.tsx`、`apps/storefront/app/[locale]/collections/[slug]/page.tsx` · 后端 `internal/buyerhttp/catalogv2.go`
- 正常：列表按查询更新，所选商品的详情、现价与可售规格一致。浏览器控制台 0 error。

### 买家 · 商品与购物车 · /products/[slug]、/cart
- 操作：选择规格并加入购物车，调整数量后刷新。
- 前端 `apps/storefront/app/[locale]/products/[slug]/page.tsx`、`apps/storefront/app/[locale]/cart/page.tsx` · 后端 `internal/buyerhttp/handler.go`、`internal/buyerhttp/catalogv2.go`
- 正常：购物车保留 SKU 和数量，小计与服务端回执一致。浏览器控制台 0 error。

### 买家 · 结账 · /checkout
- 操作：填写测试收货资料，选择可用配送与支付方式，确认下单。
- 前端 `apps/storefront/app/[locale]/checkout/page.tsx` · 后端 `internal/buyerhttp/handler.go`、`internal/buyerhttp/cvs.go`、`internal/checkout/`
- 正常：收到唯一订单号，订单详情显示已冻结的金额与配送。浏览器控制台 0 error。

### 买家 · 订单与付款 · /orders/[orderID]
- 操作：打开所属订单，按已启用方式查看付款指示，刷新状态。
- 前端 `apps/storefront/app/[locale]/orders/[orderID]/page.tsx` · 后端 `internal/buyerhttp/payment.go`、`internal/buyerhttp/transfer.go`
- 正常：付款操作与订单状态相符，刷新延续原订单及付款状态。浏览器控制台 0 error。

### 买家 · 查单与订单链接 · /orders/lookup、/order-link
- 操作：使用测试订单号和对应联系方式查单，或打开有效订单访问链接。
- 前端 `apps/storefront/app/[locale]/orders/lookup/page.tsx`、`apps/storefront/app/[locale]/order-link/page.tsx` · 后端 `internal/buyerhttp/lookup.go`、`internal/buyerhttp/orderlink.go`
- 正常：有效输入进入所属订单，错误或失效输入给出可见提示。浏览器控制台 0 error。

### 买家 · 直播认领 · /claim
- 操作：打开有效认领链接，核对保留商品与数量，再进入结账。
- 前端 `apps/storefront/app/[locale]/claim/page.tsx` · 后端 `internal/buyerhttp/claims.go`、`internal/claims/`
- 正常：保留的 SKU、数量与金额在结账中一致。浏览器控制台 0 error。

### 买家 · 退货 · /orders/[orderID]/return
- 操作：对可退订单选择测试商品与原因，提交后刷新。
- 前端 `apps/storefront/app/[locale]/orders/[orderID]/return/page.tsx` · 后端 `internal/buyerhttp/return_requests.go`、`internal/returns/`
- 正常：订单显示唯一申请及其当前状态。浏览器控制台 0 error。

### 买家 · 回放 · /replays、/replays/[sessionID]
- 操作：从列表打开一条已发布的回放。
- 前端 `apps/storefront/app/[locale]/replays/page.tsx`、`apps/storefront/app/[locale]/replays/[sessionID]/page.tsx` · 后端 `internal/buyerhttp/replay.go`
- 正常：详情标题与所选场次一致，存在合格来源时显示播放器或回放入口。浏览器控制台 0 error。

### 买家 · 联系与隐私 · /contact、/privacy
- 操作：提交测试询问，或查看并保存当前买家可编辑的同意设置。
- 前端 `apps/storefront/app/[locale]/contact/page.tsx`、`apps/storefront/app/[locale]/privacy/page.tsx` · 后端 `internal/buyerhttp/contact.go`、`internal/buyerhttp/privacy.go`
- 正常：询问显示提交回执，同意设置刷新后保持。浏览器控制台 0 error。

## 商家

### 商家 · 登录与店铺工作台 · /
- 操作：登录测试账号，切换有权限的店铺，进入订单或库存。
- 前端 `apps/admin/app/[locale]/page.tsx`、`apps/admin/app/[locale]/layout.tsx` · 后端 `internal/identityhttp/handler.go`、`internal/httpapi/merchanttools.go`
- 正常：工作台显示当前店铺，导航后业务数据仍在该店铺范围内。浏览器控制台 0 error。

### 商家 · 商品、规格与媒体 · /products、/products/[product]
- 操作：创建或打开测试商品，编辑规格和图片并保存。
- 前端 `apps/admin/app/[locale]/products/page.tsx`、`apps/admin/app/[locale]/products/[product]/page.tsx` · 后端 `internal/httpapi/handler.go`、`internal/httpapi/images.go`、`internal/catalog/`
- 正常：列表可查到商品，重新打开后规格与媒体顺序保持。浏览器控制台 0 error。

### 商家 · 商品分组 · /collections
- 操作：创建测试集合并按集合类型选商品或配置规则。
- 前端 `apps/admin/app/[locale]/collections/page.tsx` · 后端 `internal/httpapi/collections.go`、`internal/catalog/`
- 正常：保存后列表可查，重新打开显示已保存的成员或规则。浏览器控制台 0 error。

### 商家 · 库存 · /inventory
- 操作：选择仓库，查找测试 SKU，记录一笔测试库存调整。
- 前端 `apps/admin/app/[locale]/inventory/page.tsx` · 后端 `internal/httpapi/handler.go`、`internal/inventory/`
- 正常：刷新后余额按调整量变化，台账显示相应记录。浏览器控制台 0 error。

### 商家 · 订单与手工建单 · /orders、/orders/new、/orders/[order]
- 操作：从测试商品建单，回到订单列表筛选并打开详情。
- 前端 `apps/admin/app/[locale]/orders/page.tsx`、`apps/admin/app/[locale]/orders/new/page.tsx`、`apps/admin/app/[locale]/orders/[order]/page.tsx` · 后端 `internal/httpapi/orders.go`、`internal/merchantorders/`
- 正常：新订单只出现一次，详情显示已保存的商品、金额和状态。浏览器控制台 0 error。

### 商家 · 出货与超商配对 · /orders/shipping、/orders/cvs-match
- 操作：在出货页选择待履约测试订单查看拣货信息，另打开门市工具按地区和门市资料查找。
- 前端 `apps/admin/app/[locale]/orders/shipping/page.tsx`、`apps/admin/app/[locale]/orders/cvs-match/page.tsx`、`apps/admin/components/CvsLookup.tsx` · 后端 `internal/httpapi/picklist.go`、`internal/httpapi/shipments.go`，门市工具读取公开 `/cvs/` 目录。
- 正常：拣货信息对应所选订单和 SKU，门市结果符合当前查询。浏览器控制台 0 error。

### 商家 · 退货处理 · /returns
- 操作：按状态筛选测试退货，打开退货记录，再跳转关联订单。
- 前端 `apps/admin/app/[locale]/returns/page.tsx` · 后端 `internal/httpapi/returns.go`、`internal/httpapi/orders.go`
- 正常：退货详情显示对应原因、状态和数量，订单链接进入关联订单。浏览器控制台 0 error。

### 商家 · 客户与会员 · /customers、/customers/[customer]、/members、/members/[member]
- 操作：筛选客户或会员，打开所属记录，保存一项测试备注。
- 前端 `apps/admin/app/[locale]/customers/page.tsx`、`apps/admin/app/[locale]/customers/[customer]/page.tsx`、`apps/admin/app/[locale]/members/page.tsx`、`apps/admin/app/[locale]/members/[member]/page.tsx` · 后端 `internal/httpapi/customers.go`、`internal/httpapi/customer_tags.go`、`internal/httpapi/members.go`
- 正常：重新打开显示对应档案及已存备注，其他档案保持各自内容。浏览器控制台 0 error。

### 商家 · 直播与认领 · /studio、/studio/console、/studio/claims
- 操作：从测试场次进入控制台，查看认领及库存，按权限配置关键词。
- 前端 `apps/admin/app/[locale]/studio/page.tsx`、`apps/admin/app/[locale]/studio/console/page.tsx`、`apps/admin/app/[locale]/studio/claims/page.tsx` · 后端 `internal/httpapi/studio.go`、`internal/httpapi/live_console.go`、`internal/httpapi/claims.go`
- 正常：控制台与认领列表对应同一场次，重开可读到已存的关键词。浏览器控制台 0 error。

### 商家 · 收件箱与模板 · /messages、/messages/templates
- 操作：在测试收件箱打开会话并查看可用模板，保存测试模板后刷新。
- 前端 `apps/admin/app/[locale]/messages/page.tsx`、`apps/admin/app/[locale]/messages/templates/page.tsx` · 后端 `internal/httpapi/inbox.go`、`internal/httpapi/templates.go`
- 正常：当前会话正确显示，模板列表可重新读到保存结果。浏览器控制台 0 error。

### 商家 · 店面设计 · /design
- 操作：编辑测试店面草稿并打开预览。
- 前端 `apps/admin/app/[locale]/design/page.tsx` · 后端 `internal/httpapi/design.go`、`internal/design/`
- 正常：预览反映已保存的草稿，重新进入编辑页可读到同一版本。浏览器控制台 0 error。

### 商家 · 配送、卡款与结算 · /settings、/settings/payments/card、/settings/settlements
- 操作：保存测试配送设置，查看平台卡款配置与结算列表。
- 前端 `apps/admin/app/[locale]/settings/page.tsx`、`apps/admin/app/[locale]/settings/payments/card/page.tsx`、`apps/admin/app/[locale]/settings/settlements/page.tsx` · 后端 `internal/httpapi/settings.go`、`internal/httpapi/payment_card.go`、`internal/httpapi/settlements.go`
- 正常：配送配置可在刷新后读回，卡款与结算状态按当前店铺显示。浏览器控制台 0 error。

### 商家 · 促销 · /promotions
- 操作：保存一项测试优惠，按状态查找并重新打开。
- 前端 `apps/admin/app/[locale]/promotions/page.tsx` · 后端 `internal/httpapi/promotions.go`、`internal/promotions/`
- 正常：名称、条件和状态与保存回执一致。浏览器控制台 0 error。

### 商家 · 广告与归因 · /ads、/ads/attribution
- 操作：用隔离数据查看账户状态，切换归因日期范围。
- 前端 `apps/admin/app/[locale]/ads/page.tsx`、`apps/admin/app/[locale]/ads/attribution/page.tsx` · 后端 `internal/httpapi/ads.go`、`internal/attribution/`
- 正常：页面显示所选日期的汇总或空态，账户状态与当前店铺相符。浏览器控制台 0 error。

### 商家 · 财务报表 · /finance、/finance/reports
- 操作：查看测试账务汇总，切换报表日期或筛选条件。
- 前端 `apps/admin/app/[locale]/finance/page.tsx`、`apps/admin/app/[locale]/finance/reports/page.tsx` · 后端 `internal/httpapi/finance.go`、`internal/httpapi/reports.go`
- 正常：结果、日期和币种与已选筛选一致，刷新后按当前查询恢复。浏览器控制台 0 error。
