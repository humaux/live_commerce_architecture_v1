# 直播电商 SaaS：UI 架构（v1 草案，2026-10-02，待 owner 确认）

依据：
- 现状盘点：`output/r5-research/UI-INVENTORY.md`
- SHOPLINE 后台对标：`output/r5-research/admin/REPORT.md`，以及 `docs/discovery/2026-10-02-shopline-admin-study.md`
- 1688 发布流程：`docs/delivery/units/product-editor.md`
- 前台对标：`output/r5-research/storefront/REPORT.md`
- 视觉 QA：`output/r5-research/admin/REPORT-admin-vqa.md`
- owner 偏好（2026-09-20）：先出视觉稿确认再开发；三语自由切换。

状态：DESIGN。本文定义「有哪些端、每端有哪些页面和功能、代码如何分模块、用什么门禁验收、怎么迁移」。业务规则仍以 contracts/ 为准，本文不改合同。

## 0. 现状（盘点事实）

| | 商家后台 apps/admin | 买家前台 apps/storefront |
|---|---|---|
| 页面路由 | 23 | 16 |
| 组件 / 行数 | 42 / 16,231 | 29 / 6,019 |
| lib 文件 / 行数 | 71 / 12,896 | 41 / 8,483 |
| 文案文件 | 18 | 13 |
| CSS | 10 个文件 / 2,699 行 | 3 个文件 / 1,180 行 |
| 超过 800 行的组件 | SettingsWizard 2788、Ledger 1072、MerchantOrders 981 | OrderFlow 838 |

主要问题：
- 同一实体两处可改：商品名称、描述、价格、图片、归档在 ProductEditor 和 Ledger 里各有一套。
- 「新增商品」有 3 个入口。
- 后台金额格式化有 4 套，时间格式化有 8 处，其中 2 处写死 UTC；价格输入用最小货币单位。
- 导航文案来自 7 个文案模块。
- 3 个页面没有浏览器测试。
- 6 个组件在内部直接 fetch。
- 侧栏的渠道状态是写死的。

根因：没有 UI 层的统一模块。页面、组件、格式化、导航、权限、文案各自生长，也没有门禁约束「一个功能只有一个入口、一个实体只在一处编辑」。

## 1. UI 端口（surfaces）

| 端 | 使用者 | 主要设备 | 应用 | 本期 |
|---|---|---|---|---|
| A. 商家后台（Merchant Console） | 店主、员工（按角色） | PC 1366–2000px，手机可用 | apps/admin | 重做（W0–W4） |
| B. 主播直播间（Live Host Room） | 主播、助播 | 手机或平板优先，横竖屏 | apps/admin 内的 `/live/{id}/room` 独立布局 | 新增（W3） |
| C. 买家前台（Storefront） | 买家（多从 FB/IG 直播进入，在内置浏览器里打开） | 手机优先 | apps/storefront | 改造（W5） |
| D. 入驻与账号（Auth & Onboarding） | 新商家、受邀员工 | PC 或手机 | apps/admin 的独立布局（无侧栏） | 整理（W0） |
| E. 平台运营端（Platform Ops） | SaaS 平台方 | PC | 现为 CLI `ops-admin.sh` | 暂不做 UI，升级信号见 §7 |

## 2. 商家后台信息架构（A + D）

一级导航分 10 组，按角色权限显示。下表的路由都带 `/{locale}` 前缀，以 `{s}` 表示当前店铺（经 `?store=` 或会话选择）。

| 组 | 页面 | 路由 | 模板 | 权限 | 核心功能 |
|---|---|---|---|---|---|
| 总览 | 总览 | `/` | Dashboard | orders:read | 待办卡（待付款、待出货、待交寄、待收款、留言未处理）；今日和近 7 日成交；正在或下一场直播入口；店铺未发布提示 |
| 订单 | 订单列表 | `/orders` | ListPage | orders:read | 状态页签带计数；搜索（单号、电话后 4 码、收件人、运单号、SKU）；筛选（付款方式、配送方式、来源、直播场次、日期）；跨页批量操作（建超商单并打印、标记已收款、导出） |
| | 订单详情 | `/orders/{id}` | DetailPage | orders:read | 商品、金额、收件、付款（含货到付款收款）、出货与运单、退款与退货、时间线、备注 |
| | 新建订单 | `/orders/new` | EditorPage | inventory:reserve | 后台建单并生成买家链接 |
| | 打印 | `/orders/print` | Utility | fulfillment:write | 超商标签与出货单 |
| 商品 | 商品列表 | `/products` | ListPage | catalog:read | 状态页签；智能筛选；批量上下架、分类、封存；行内改价和改库存；复制 |
| | 新增、编辑商品 | `/products/new`、`/products/{id}` | EditorPage | catalog:write | 1688 式一页表单（见 product-editor.md）：多图、规格矩阵、批量填充、∞ 库存、关键字、分类、SEO，一次保存 |
| | 分类 | `/products/collections` | ListPage + 抽屉 | catalog:write | 新增、排序、封面、成员 |
| | 关键字库 | `/products/keywords` | ListPage | live:manage | 全店关键字（一商品一码），按前缀建议下一个号，可批量导入（从 Studio 迁出） |
| | 导入导出 | `/products/import` | WizardPage | catalog:write | CSV 导入，全部成功或全部回滚 |
| 库存 | 库存 | `/inventory` | ListPage | inventory:read | SKU 的现有、预留、可售，追踪或 ∞；调整必须填原因；只管库存，不编辑商品 |
| | 库存流水 | `/inventory/movements` | ListPage | inventory:read | 每次变动的来源（订单、调整、退货） |
| 直播 | 直播场次 | `/live` | ListPage | live:read | 场次列表，显示订单数和金额；一键复制上一场（日期标题自动替换） |
| | 场次设置 | `/live/{id}` | EditorPage | live:manage | 商品与关键字、直播价、匹配规则（精确或受限包含）、留言来源（从 Page 直播中列表点选）、消息模板 |
| | 直播间（主播端） | `/live/{id}/room` | WorkspacePage | live:manage | 实时认领流、商品卡（库存、直播价、开关）、快速改价、上架下架关键字、成果计数 |
| | 认领记录 | `/live/{id}/claims` | ListPage | live:read | 认领、链接、下单、付款状态 |
| | 直播默认设置 | `/live/settings` | SettingsPage | live:manage | 默认匹配规则、消息模板、结账提醒 |
| 顾客 | 顾客列表、详情 | `/customers`、`/customers/{id}` | ListPage、DetailPage | customers:read | 订单汇总；隐私导出、删除；（下单黑名单在 W4 之后做） |
| 营销 | 优惠活动 | `/marketing` | ListPage（页签） | pricing:read | 页签分为优惠码和免运费（免运费从设置迁入） |
| | 广告 | `/marketing/ads` | 现有 Ads | ads:read | 不变，只换外壳 |
| 网店 | 店铺装修 | `/store/design` | EditorPage + 预览 | integration:read | 现有 Design |
| | 网址与发布 | `/store/domains` | SettingsPage | integration:manage | 平台子域（英文店铺 ID）、自有域名 DNS 指引、发布开关（store-domains 单元） |
| 数据 | 财务 | `/reports/finance` | ReportPage | orders:read | 按日、按付款方式（含货到付款、转账、取货付款）汇总，导出 CSV |
| 设置 | 店铺资料 | `/settings/store` | SettingsPage | store:read | 名称、英文店铺 ID、联系方式、营业信息 |
| | 付款 | `/settings/payments` | SettingsPage | integration:manage | 银行转账、刷卡（Stripe）、货到付款 |
| | 配送 | `/settings/delivery` | SettingsPage | integration:manage | 超商（绿界）、宅配物流商（黑猫、新竹）、运费 |
| | 结账与订单 | `/settings/checkout` | SettingsPage | integration:manage | 订单保留时长、锁定直播商品等 |
| | 渠道 | `/settings/channels` | SettingsPage | integration:manage | Facebook 和 IG 专页（多专页，meta-multi-page 单元） |
| | 通知 | `/settings/notifications` | SettingsPage | integration:manage | 买家邮件 |
| | 员工 | `/settings/team` | ListPage | 店长 | 邀请、角色、移除 |
| | 账单 | `/settings/billing` | SettingsPage | billing:manage | 平台订阅 |
| 入驻与账号（D） | 登录、注册、重置、受邀 | `/login`、`/signup`、`/reset`、`/invite/{t}` | AuthPage | — | 独立布局 |
| | 开店向导 | `/onboarding` | WizardPage | — | 店名、英文店铺 ID（生成子域）、币种、仓库 |

合计 33 个页面路由（现有 23 个）。SettingsWizard（2788 行）拆成 8 个设置页。Studio 拆进直播组。

导航规则：
- 一个功能只出现一次。
- 组可以折叠，侧栏可以滚动。
- 没有权限的条目隐藏，直接访问时显示 403 说明页。
- 所有状态徽标来自真实数据，不得写死。

## 3. 主播直播间（B）

| 区 | 内容 |
|---|---|
| 顶栏 | 场次名、直播状态、成果（认领数、订单数、金额）、匹配规则 |
| 左栏或上方（手机） | 商品卡流：图、关键字、直播价、可售（∞ 或数量）、开关；点按改价 |
| 右栏或下方（手机） | 实时认领流：买家标签、关键字加数量、状态（已认领、已发链接、已下单）；异常留言（不命中、需人工）单独一栏 |
| 底栏 | 发公告、暂停或恢复下单、结束直播 |

要求：
- 竖屏 390px 和横屏平板都能单手操作，点击目标至少 44px。
- 断网重连后只显示服务端确认过的状态。

## 4. 买家前台信息架构（C）

| 页面 | 路由 | 核心功能 |
|---|---|---|
| 首页 | `/` | 装修文档（主图、分类、精选、商品网格、图文），以及直播入口区块（S6） |
| 全部商品、分类列表、单个分类 | `/products`、`/collections`、`/collections/{slug}` | 分类 chips（S4）、排序、价格筛选、加载更多 |
| 商品详情 | `/products/{slug}` | 图库（可放大，S7）、规格选择、直播价标识、配送付款退货说明（S2）、规格表（S8）、相关商品（S3）、吸底购买栏 |
| 搜索 | `/search` | 按标题、描述、SKU 搜索 |
| 购物车 | `/cart` 和抽屉 | 直播价显示、免运进度、推荐 |
| 结账 | `/checkout` | 配送（超商选店、宅配）、付款（取货付款、货到付款、转账、刷卡）、优惠码、报价 |
| 订单、查询、链接 | `/orders/{id}`、`/orders/lookup`、`/order-link` | 订单状态（标题必须跟随真实状态，D06）、转账凭证、取消 |
| 认领 | `/claim` | 直播认领链接，加入购物车 |
| 直播 | `/live`（S6） | 当前或下一场直播、本场商品 |
| 内容 | `/pages/{slug}`、`/legal/{slug}`（含防诈骗 S5）、`/privacy`、`/data-deletion` | 静态和装修页面 |

全局模块：页眉（Logo、导航、搜索、购物车、语言）、公告栏、页脚（法务、联系、营业信息）、购物车抽屉、直播横幅。
合计 18 个页面路由。

## 5. 代码模块化

### 5.1 分层

```
packages/
  ui/        设计令牌、基础组件、模式组件
             基础：Button Input Select Checkbox Switch Tabs Badge Dialog Drawer Toast Tooltip
             模式：PageHeader DataTable(选择+批量栏) FilterBar Pagination EmptyState ErrorState Skeleton SectionNav(sticky)
             表单：Field MoneyInput StockInput(∞) StoreDateTime ImageUploader(多选+排序)
  format/    金额解析与显示（TWD 整数元）、店铺时区时间、各域状态字典（唯一实现）
  i18n/      已有：Locale，加运行时 key 一致性检查
apps/admin/
  src/routes.ts      路由注册表（单一真源）：{id, path, group, labelKey, icon, permission, template, spec}
                     → 侧栏、面包屑、权限门、403、check-gates 都从这里生成
  src/shell/         布局：侧栏、顶栏、店铺切换、语言、AuthLayout、RoomLayout
  src/features/<域>/ orders products inventory live customers marketing store reports settings account
     pages/  components/  model.ts（纯函数）  api.ts（唯一调 BFF 的地方）  copy.ts（三语，typeof en）
  app/[locale]/**    只做路由入口：page.tsx 读权限，渲染 features/<域>/pages/*
apps/storefront/
  src/features/      catalog product cart checkout orders claim live content privacy
```

### 5.2 规则

1. 只有 `features/<域>/api.ts` 能调 BFF，组件不 fetch。
2. 金额和时间只能用 `packages/format`。组件里禁止直接用 `Intl.NumberFormat` 或 `toLocaleString`，禁止出现「最小货币单位」输入。
3. 每个实体只有一个编辑页。其它地方只能放跳转链接或只读展示。
4. 文案按域拆分，三语类型一致，运行时再检查一次。组件中不得出现硬编码的用户可见字符串。
5. 样式用设计令牌加按域的 CSS Modules，不再往全局 CSS 里加。
6. 页面模板固定为 7 种：ListPage、DetailPage、EditorPage、SettingsPage、WizardPage、WorkspacePage、ReportPage。新页面必须选一种。
7. 组件超过 500 行要拆，超过 800 行门禁失败。

### 5.3 BFF

`app/api/stores/[store]/[...resource]` 的白名单改成每个 feature 自带一张表，由路由注册表汇总。BFF 仍是唯一出口，前端不直连 Go。

## 6. 验收门禁（每个迁移批次都要过）

| 门禁 | 内容 |
|---|---|
| G-UI1 注册表一致性 | 每个路由都在注册表里，带权限、三语 labelKey 和浏览器 spec；导航中没有重复功能；check-gates 执行 |
| G-UI2 版式 | 每个页面在 1366×768、1586×992、2000×1100、375×812 下截图：无横向滚动、无元素重叠（Playwright 用 elementFromPoint 做重叠检查），截图交 K3 视觉 QA，P1 为 0 |
| G-UI3 格式 | lint 检查：组件中无 Intl 或 toLocaleString，无最小货币单位文案，无 UTC 显示；`packages/format` 有单元测试（TWD 整数元） |
| G-UI4 无障碍 | axe 无 serious 及以上；焦点顺序；按钮 44px |
| G-UI5 体量 | 组件 ≤800 行；fetch 只出现在 api.ts |
| G-UI6 视觉稿 | 每个域开发前先出视觉稿（三语，桌面加手机），owner 确认后才开发 |
| G-UI7 回归 | 现有全部浏览器模式和 release-gate B-* 保持绿 |

## 7. 迁移计划（逐域替换，不推倒重写）

| 批次 | 内容 | 执行 |
|---|---|---|
| W0 外壳与基础 | packages/ui、format、路由注册表、新侧栏分组、AuthLayout；旧页面先挂进新外壳；修掉误导性的 P1（金额输入、D03、D04、D06） | K2.8 做 UI，DeepSeek 做注册表和 lint，Claude 审 |
| W1 商品与库存 | product-editor 单元，库存页收口 | DeepSeek 加 K2.8，K3 测 |
| W2 订单 | 列表 v2、详情页、批量操作、货到付款收款 | 同上 |
| W3 直播 | 场次、设置、主播直播间、关键字库迁移 | 同上 |
| W4 营销、顾客、网店、数据、设置 | 拆分 SettingsWizard，店铺网址页（store-domains） | 同上 |
| W5 买家前台 | 前台报告的前 10 项，以及模块化 | K2.8 加 DeepSeek |
| 以后 | 平台运营端 UI | 需求出现再做 |

每个批次的流程：视觉稿 → owner 确认 → 实现 → 独立测试 → K3 视觉 QA → 合并。旧组件在替换后立即删除，不留并存。

## 8. 已知局限与升级信号

- **主播端**：仍是网页，不做原生 App。升级信号：主播要求推流和直播间合一，或需要后台常驻提醒。
- **平台运营端**：仍是 CLI。升级信号：入驻商家超过约 10 家，或出现非技术运营人员。
- **工程结构**：单仓库两个 Next 应用。升级信号：前端多于 2 名开发并行，或页面超过 60 个，才考虑按端拆仓或引入组件文档站。

## 9. 不采用的方案

| 方案 | 原因 |
|---|---|
| 推倒重写 | 现有业务规则和门禁都绑在页面上，一次性重写风险高、周期长；逐域替换每步可验收 |
| 引入重型组件库（AntD、MUI） | 包体大，和现有设计令牌、三语、无障碍细节冲突；我们需要的是约 20 个组件的薄层 |
| SHOPLINE 式微前端（子域 iframe） | 平台级多团队方案，对我们是额外的部署和会话复杂度；实测还有 iframe 内语言不一致的问题 |
