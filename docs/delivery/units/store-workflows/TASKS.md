# 台湾商家经营工作台实施任务书

**整店 HOLD，以下 14 张主卡及其可独立派发的修复子卡为拟派工作。** 先取得界面清单与 owner 审批，再改正常入口。最高优先级是买家查单覆盖原购物身份、非首店库存目标、仓储岗位盘点和买家申请漏进商家售后。只在隔离店做写入反例。通过的对象是具体角色在具体入口办完一件事，判据是刷新后的业务事实和独立核账。

## 使用基线

- I = 集成 `12c60ea6eff3ca6a7f5c0c4d2e83702aeb3cd8f2`；P = 试点 `a2d079cac8004fb822705568c4b7d4e6a61b27c4`。I 比 P 多 #764 的文档与检查。开发按 I 的接口定位，每次验收重新绑定候选 SHA。H = `output/release-gate/5dffb337-ego-business-acceptance.md`，不同部署分段按其原时间引用；P 的现场原件为同目录 `a2d079ca-ego-checkout-acceptance.md`。
- 依据：I 树的 `docs/TASKS.md` 与 `contracts/tasks.json` 给旧父项及 DAG，`contracts/invariants.json` 给 I01 等 P0 控制，`docs/delivery/units/<名>.md` 给现有交付单元。入口/模块/命令/证据层级按 `docs/FEATURE_MAP.md`、`docs/engineering/dependency-map.md`、`scripts/dev/verify`、`docs/delivery/GATES.md`。业务目标对照 `output/merchant-product-architecture-e127df97/商家产品架构.md`；本轮边界与事务见[架构](../../../architecture/merchant-workflows.md)，操作与正式稿按[UI 缺口](UI-GAPS.md)逐项对应，固定源码与操作证据见[范围与事实](README.md)。新卡细分旧任务，原 DAG、资格、门禁继续有效。
- 表内 `B 模式` 为完整命令 `bash scripts/dev/test-local.sh --browser-模式`；多个模式各跑一次。前端另跑 `scripts/dev/verify --module admin` 或 `--module storefront`（Node 24、无 DB）；浏览器/PG 正式复跑用项目 CI 对应模式。每张修复卡先把所列反例加入指定现有 suite，使旧代码在正常页面失败，再以同断言、同 fixture、同入口核 GREEN。资格卡先对空证据记 NOT_RUN，负对照预期拒绝，无产品 RED 时不改实现。
- 每卡同名独立 worktree/分支，路径均相对 I，UI 前缀 A=`apps/admin`、S=`apps/storefront`。下列是各卡独占写面；只读依赖可共享。共享契约、OpenAPI、迁移编号和跨模块 BFF 契约由 02 的 integrator 单写，按卡分配，先冻结再实施。若需新增未冻结接口，阻塞其消费者，回到 02 按该项补签。领域内部继续使用 Go 应用服务、现有 PG/事务队列；禁止新增通用治理脚本。
- 逐卡统一红线：作者交 SHA、角色、模型/档位、实际工作树、diff、测试命令与 exit、失败原件及新回执。主控按同 SHA 复跑，独立 test_worker/security_reviewer 审核，P0/P1 阻断。模型、浏览器 MOCK、REAL_PG、外部 SANDBOX、LIVE 分栏；同意、外部能力或收件资格不足记录 BLOCKED_EXTERNAL。真实客户消息、密钥、退款、广告、面单购买、拨款、删真数据、生产发布需 owner 另行授权，本批均不执行。

## 排期与冻结责任

01 → 02 为唯一设计放行线。02 在图稿旁冻结：服务端店铺/角色矩阵和深链传播；商品草稿命令与期初入账；版本化运费＋免运输入及旧请求摘要兼容；申请的列表游标、状态/版本与 RMA 关联；已确认门市与待选草稿；买家 full/只读凭据、一次性链接和恢复策略；金额环境与来源字段；UNKNOWN 的原语义键与查询端口。交付各端请求/回执/预期拒绝样例及 normal entry（正常入口）后，后端端口先通过契约测试，UI 才接入。设计或权限仍未知的一项保持 HOLD，其他已签并独立的卡可先走。

02 后可首批并行 05 与 08，再补 04 与 09（最多两写手起步）。03 接替 04 所用旧向导文件后摘除旧壳；06 等 03 的商品入口、05 的权限与库存形状稳定；07 等 05 的角色/店铺矩阵。11 可与 05/08 并行开发，和 08/09 合入后再验完整买家链。12、13 可先准备隔离数据；实际资格运行依各卡列出的输出。14 只接受已归并的同一候选。共享测试文件如遇冲突，按先卡合入再开后卡，测试不复刻另一个 harness。

### TW01 · 统计实际入口与正式设计
父 T00/T05/T18；依赖 无；platform_explorer，`plan/tw01-surfaces`，独占盘点证据 `output/tw01-surfaces/`。状态 DESIGN，可先认领。输入为 I/P 的页面模板、岗位、H/P 实图及正式 Figma 树；输出为“用户意图→正常入口→角色/设备/语言/状态→正式 key/node/实图→缺口”的逐项清单，交 02。
从店主、仓储、主播、买家各走菜单/正常链接，另列深链。用 `git ls-tree -r --name-only 12c60ea6 apps/admin/app apps/storefront/app` 对 page.tsx 去重；既有分母 44 Admin、21 Storefront，包括技术页。现有 14 项样本为设计需改 1、实现偏离 3、缺验证 10。figma/roots.json（SHA256 737b9be3dece307bd85945e17de96482dc067350bc344a0673c3a8cfd981d85d）枚举正式页 3,829 个根，其中 3,695 个可见 FRAME；逐根仍需区分取代、候选与签核。签核帧总数及全任务可达率 UNKNOWN。误把候选页、无权限地址、重复状态计作正式覆盖应红；逐项还原分母、未读标 UNKNOWN 为绿。I01/I18；只读，无产品运行资格。

### TW02 · 修正式 Figma、审批任务状态并冻结契约
父 T01/T18、OC-05/06/09/10/14；依赖 TW01；integrator（图稿执行 ui_worker，owner 审批），`design/tw02-approved`；独占正式设计修改窗口、已批准的契约增量、共享 schema/迁移分配与 `output/tw02-approved/`。状态为已授权主控在正式页建立可编辑修订副本，最终 owner 签核待办。主控持唯一 Figma 写锁；ui_worker 供稿，先留树/图/反应与可恢复副本。文件 `iJ9lDdFIXgTlKkt5Tvrc0X`，页 `4:8829`；本轮改稿以设置总览 `1429:32319 / 1429:32392`、单项配送及实际门槛、首件商品进入同一正式商品编辑器为范围，保留源帧。售后先核商家 `1468:107474 / 1468:107559`、买家 `1468:99242 / 1468:99291` 的签核和实际树；门市核搜索 `1429:14065 / 1429:14122`、暂选 `1429:14738 / 1429:14800`、确认 `1431:71232 / 1431:71249`。按当前任务授权裁定需要补画的状态，未签核覆盖保持 UNKNOWN。
输入为 01 清单与业务契约；输出为每个受影响任务的桌面/手机原型、权限/等待/成功/冲突/未知状态、入口到终态、经批准的接口样例。Owner 从已开店设置找单项运费、从商家申请作批准/拒绝、从买家撤回或取消换店；记录批准人、日期、版本及覆盖节点。旧图缺决策按钮/错误跳转为红；复读原尺寸导出和反应、token diff=0、无缺失目标、owner 明确签核才放行。`node scripts/design/figma-lint.mjs --file-key iJ9lDdFIXgTlKkt5Tvrc0X --page-id 4:8829` 仅算 lint；正式原型须能从根页面走到审批或保存结果，缺失状态记录节点、owner 决定和替代入口。读不到、候选稿或只有作者 ACK 均 HOLD。I01/I13/I18，未签节点不可开其 UI 实现。

### TW03 · 开店与日常新增统一到商品草稿
父 T03/T04/T18、OC-05/06；unit product-editor、catalog-core；依赖 TW02、TW04（旧向导交接）；ui_worker，`feat/tw03-product-entry`；独占 A/components/Dashboard.tsx、A/components/dashboard/DashboardTasks.tsx、A/components/SettingsV2.tsx、04 移交后的 A/components/SettingsWizard.tsx、商品新增的引导入口与 tests/admin/catalog-core.spec.ts。状态 DESIGN，旧入口重复。历史 /?view=first-product 与 /?view=delivery 保留店铺及语言；旧浏览器有未决草稿或期初请求先按原键查回执，键丢失时由店主选明确商品、核 SKU/流水，禁重播。工作台/设置的提示只读 quickstart，转到本店商品编辑器 → catalog.SaveProductDocument → PG 商品/SKU/库存期初流水，明确操作才改变 quickstart。
空店跳过提示仍能经营；新增 NT$1,280、初始 10，刷新见一草稿一 SKU 一笔 +10，买家不可购；初始 0 零流水。取消、只改费零商品写，已有店回设置不进入开店。旧版第三步及宽表单先红，签稿后 B catalog-core、B product-editor 验绿。保留 #745 同文档切页回归；专页承接渠道回跳、通知、COD、店面及历史只读转账后方可解除旧挂载。I02/I03/I04；发布店面另需批准。

### TW04 · 店主改运费、免运门槛并复核新报价
父 T04/T11/T13、OC-07/09；unit checkout-offline、delivery-allocation；依赖 TW02；commerce_worker，`feat/tw04-delivery-fee`；独占 internal/fulfillment/flat_fee.go、A/components/SettingsAmountEditor.tsx、配送单项的 settings 客户端及 tests/admin/settings-amount.acceptance.ts；旧向导装配先占后交 03。状态 DESIGN，P 仅回读。普通设置 → 配送服务/市场 → 单项保存 → 履约应用更新价格版本并重绑服务 → 买家重新报价；双版本 CAS。I01/I02/I04/I08/I14。
原宅配 60→80，同车未达门槛只涨 20，旧单冻结；门槛差 NT$1、恰达、零与明确关闭逐例，省略保持旧值、旧摘要可回放。缺 integration:manage 或 pricing:write 零写，冲突重读，未知查原命令。其他服务/税/仓不动，四家手工超商、已有 API 绑定和原禁用状态保留，日常操作不弹凭据。旧总览先红，B identity、B checkout-offline 加验已营业店与新车；无真实物流连接。

### TW05 · 选店后盘点本店，仓储员可找到并使用库存
父 T03/T04、OC-10；unit staff-team、catalog-core；依赖 TW02；commerce_worker，`fix/tw05-stock-scope`；独占 A/app/[locale]/inventory/page.tsx、A/lib/backend.ts、A/components/Ledger.tsx、A/src/routes.ts（商品组可见性）、A/src/features/catalog/routes.ts（库存项）、internal/httpapi/handler.go 的 scoped 库存读入口及 tests/admin/parity-data-ops.spec.ts。P1 离线 RED。库存页入站传所选店 → 已授权仓/最小 SKU 信息及库存读取 → adjust → inventory.ledger/balances；Go 鉴权，页面无权回退首店。通过现有库存读权限提供盘点所需数据，禁止向 fulfilment 增授 catalog:read。
实际 0119 角色包建 A/B 同名 SKU 不同库量，选非首店 B，从侧栏到库存，读仓、行、URL 同 B；实际加 1 后刷新 B 一笔、A 零笔。切回 A、只读/无权店、仓与 SKU 不匹配、过期版本、重复提交逐项核账。RED 为 B 页读 A、仓储菜单消失或读台账被拒；现有证据未执行错店写。B catalog-core（实际装载 parity-data-ops）、B admin-parity、B password-auth、B admin-legacy，扩 signed 店铺路径与隔离 PG 核验；只跑 fixture 模式不放行。I01/I02/I03/I14/I19。

### TW06 · 商品正常入口预览、提交和导出 CSV
父 T03/T04/T18、OC-03/04/15；unit merchant-tools；依赖 TW03、TW05；ui_worker，`fix/tw06-catalog-csv`；独占 A/components/ProductList.tsx、ProductImport.tsx（同目录）、A/lib/merchant-tools-client.ts 的 CSV 调用、tests/foundation/merchant_tools_csv_smoke_test.go，以及 03 移交的 tests/admin/catalog-core.spec.ts 的 CSV 场景。P2 源码缺入口；H 直达预览有回执。商品操作 → preview → 用户确认 commit → merchanttools 导入事务 → 商品/SKU/库存，导出走已有新鲜认证。
取单行合成 CSV 预览 1 条零写，提交/同键重读仅有一份，刷新列表见正确 SKU/数量；坏行整批零写；只读账号不得预览写入，具备双读权可导出，过 10 分钟展示复验并续原动作。跨店结果不可读；不把顾客迁移向导当商品 CSV。B catalog-core 扩正常入口；沿 CI 的 REAL_PG foundation 运行 `go test ./tests/foundation -run '^TestMerchantToolsCSV' -count=1`。I01/I02/I03/I04/I11；仅合成文件。

### TW07 · 仓储不误见建单，卖家和买家读到一致的收款事实
父 T11/T13/T18、OC-08/09/10/13/16；unit merchant-tools、home-cod；依赖 TW02、TW05；ui_worker，`fix/tw07-order-collection`；独占 A/src/features/orders/routes.ts、A/components/ManualOrder.tsx、A/components/{OrderDetailPanel,OrderDetailView}.tsx、orders-copy 及 tests/admin/orders-ui.spec.ts。P2 岗位入口离线 RED；N5（H 第 23、30 行）同单已收 NT$90、待收 0，商业摘要仍显示待收。选品→merchanttools 创建→checkout 冻结报价/保留；发货与收款读各自投影。owner 正常建宅配/超商合成单各一，履约后登记一次测试收款；同单应收归零，摘要、买家及财务显示一致，仓储不能创建但可履约。附未取退回另单，未收款、显式收回后只解除原分配；关停转账的新单入口不可见、历史可查。B manual-order、B home-cod、B merchant-orders-ui，I01/I02/I05/I08/I13；无承运商出单/真实收款。

### TW08 · 买家申请进入本店售后队列，超过 100 件仍能处理
父 T11/T13/T18、OC-10；unit w3-08b-returns、w3-u5-returns-ui；依赖 TW02；commerce_worker，`fix/tw08-returns-queue`；独占 internal/returns/request.go、internal/httpapi/return_requests.go、A/lib/{returns-client,orders-request}.ts、A/components/{OrderListFilters,ReturnsList}.tsx、A/app/[locale]/returns/、tests/admin/returns-ui.spec.ts。扩 PG definer 及 BFF 白名单须经 02 交接。P1 H 有 201/requested v1、卖方零行；I 的 0221 列表 LIMIT 100。
买家 full 提交 → 原店 B 订单/更多筛选 → 待处理 → 申请详情 → 决策端口，批准与关联 RMA 同事务。用 101 笔（含相同时戳、最旧目标）从正常队列续页找到 #101，按稳定时序/ID 无漏重，刷新/返回仍在 B；同单详情、买家回读同申请。viewer 只读、异店不可见，批准/拒绝/撤回竞态至多一个终态，原键回读一次；拒绝/撤回零 RMA，所有决定零自动退款/回库。B returns-ui 加双端路径，REAL_PG 覆盖游标、权限、CAS，B refund-fulfilment 核独立资金状态。I01/I02/I04/I09/I13/I14。

### TW09 · 买家取消换门市仍保留原确认
父 T05/T11/T13、OC-09；unit cvs-ui、cvs-tests；依赖 TW02；ui_worker，`fix/tw09-pickup-cancel`；独占 S/components/CvsPickup.tsx、tests/storefront/cvs-picker.mjs、S/tests/cvs-picker.test.mjs。H 原生 RED，I 对照现打开时清空状态。结账确认 A → 开选择器 → 暂选草稿 → 确认才写目的地/重新报价。直接关闭、选 B 未确认再关、查无结果退出均保留 A 品牌、编号、名、地址、联系人及原确认；提交 B 成功才替换，失败仍留 A，实际切品牌须重新确认。每条取消零写单、零换店确认。B cvs、B ops-polish，四品牌各记可运行深度，桌面及 390px；更新旧 suite 仅断言“关闭后无确认文案”的错误预期，先留原失败录像。I01/I02/I08/I11，站内目录和承运商地图分列，禁买面单。

### TW10 · 财务岗位看得到自己能用的结算入口
父 T03/T17/T18、OC-03/10；unit w4-s2-platform-settlement；依赖 TW02；ui_worker，`fix/tw10-finance-access`；独占 A/components/Finance.tsx、tests/admin/reports.spec.ts 的财务入口用例（随后交 13）。P2 I 离线发现 viewer/admin 能见结算单链接，目标页需 billing:manage。财务总览读 reporting；结算单交给 payments 只读查询，导出另验 orders:export。按服务端身份/店铺显示可用动作或说明。用同店 viewer、admin、owner 从财务正常进入并刷新，前两者无误导按钮、直接地址保留拒绝；owner 读本店样单，切店不显示前店明细。B reports（CI）、B customers-billing；在这两组的签名浏览器链加入财务入口反例，测试文件与 13 串行交接。I01/I11/I15。历史资金总额及尚未运行的结算能力留 TW13，禁止实际拨付。

### TW11 · 修复手工单链接和访客查单覆盖购物身份（N1）
父 T03/T05/T11/T14/T20、OC-09/11；unit buyer-comms、merchant-tools；依赖 TW02；commerce_worker，`fix/tw11-buyer-scope`；独占 S/lib/buyer-server.ts 的查单/链接凭据处理与购物恢复入口、tests/storefront/manual-order-link-gate.mjs。状态 P0/P1 修复待办，阻断本候选商家试点。H 第 33、98、103 行是原车/售后失联现象，P/I 的 Cookie 写路径和离线调用证明身份替换；当前生产完整复现 NOT_RUN。
从原车准备 SKU-A，并以同一完整会话保存自助订单 O1。在该浏览器领取手工单 O2，用 O2 的受限上下文读单，然后返回原车及 O1 售后；再以订单号及联系资料查单，返回商品页加购。旧代码覆盖唯一 commerce_buyer，原车变成手工单商品，或 lookup 后显示 active 却在 cart 收到 403。GREEN 要求保留原会话或让买家通过明确的隔离/恢复操作回原车；Cookie/访问权由 BFF 与服务端判定，购物和查看单单的权限不得互相继承，绝不合并两个买家的订单。已用链接、新浏览器、查错联系资料、过期、跨店均拒；只读查单不能查看 O1、改车、付款、撤回或申请售后。无原会话可按契约开始新的购物会话。B manual-order、B buyer-comms、B order，按同一受控浏览器连续执行、断言原车和权限并独立核账；B payment 仅验证支付 MOCK。I01/I06/I09/I11/I15；不记录链接、令牌或买家资料，不接真实付款。

### TW12 · 主播认领到成交、会员与通知资格
父 T06/T07/T08/T09/T10/T12/T14/T15/T16、OC-12；unit live-tools、claim-direct-checkout、loyalty-points-backend、buyer-comms、meta-intake-reply；依赖 TW07、TW11、N2、N3、N4T；test_worker，`qual/tw12-live-customer`；独占 tests/e2e/deal-loop.spec.ts 的本场景和 `output/tw12-live-customer/`。资格待验。主播从工作室选本店场次与 SKU 开窗 → 签名 MOCK 评论 → claims 认领 → 买家兑换购物车 → checkout 下单；核同场同 SKU、来源、冻结直播价及一次库存变动。重复评论、过期/售罄、跨店链接、断线复连/回放分别记结果；媒体输出、目的地状态、评论接入各自留证。无连接可继续已许可的店面经营。
会员另走“手工建单→明确选会员→赠 5 点→刷新会员记录”，只追加一笔关联记录；无选择、重复提交、跨店同名、撤销授权反例。通知从真实本地 outbox 到 loopback SMTP 一封，停用收件与未知回执不盲重发；收件匣窗口外/无同意拒发，平台订阅、购买、会员均不赋送营销权。B live-console、B live-claims、B claim-checkout、B e2e、B customers-billing、B inbox、B buyer-comms；保留 H 手工赠 5 的 E2。过窗仍发送或重复入账的对照应红；外部资格按非开发者商家、官方渠道、回执逐能力验，缺则 BLOCKED_EXTERNAL。I02/I03/I04/I06/I07/I09/I10/I12/I17/I20；真实开播、消息、广告、客户名单及营销回放不在执行许可内。

### TW13 · 财务报告、平台账单与岗位生命周期资格
父 T03/T11/T17/T18/T20、OC-02/03/04/15；unit w6-02b-reports、w4-s2-platform-settlement、billing-core、staff-team；依赖 TW07、TW10；test_worker，`qual/tw13-money-staff`；独占 tests/admin/reports.spec.ts 的对账样本及 `output/tw13-money-staff/`。状态 QUALIFICATION，H 有 TEST 175−25=150 和线下收款分别立账，P 无本链新回执。财务总览/日报→日期/环境/币种/店→报告/CSV/结算单，用 payments 事实与线下登记逐单复算实收、退款、净额、待收、费用及应结，离线两类不混 Stripe。对账差异或缺明细标差额和 UNKNOWN，切环境不能把 TEST 提升为实收；CSV 有/无导出权对照，读结算单不能触发付款。
另以隔离测试订阅、签名事件回读 SaaS 权益，重复或乱序 webhook 不多记，provider 关闭返回可读状态/不能发起新动作。loopback SMTP 邀请 fulfilment、接受、改 viewer、撤销，核下一次授权与 B 店 scope。B reports、B customers-billing、B password-auth，外部 SaaS SANDBOX 按 CB10、LIVE 按 CB12 单列；额外 SaaS 订阅费不并入货款。I01/I02/I05/I06/I07/I11/I17/I18。把应收计作到账、无权 CSV 可读任一均红；实际退款、结算、订阅付费、真员工邮件禁行。

## 独立修复子卡（资格卡只验收）

下面四卡可独立开 worktree/PR；先继承 02 的签稿与示例回执，再进各自写面。N2 与 N4T 不能并写收件箱壳层：由 N2 提供已冻结导航接点，N4T 在其合入后添加模板入口。其余子卡可与互不重叠的 TW05/TW08/TW11 并行，仍遵守初始两写手上限。所有子卡继承首页红线及同 SHA 独审。

#### N2 · 商家处理买家的站内询问
父 T06/T14/T18、OC-12；依赖 TW02；ui_worker，`fix/n2-store-inquiries`；独占 A/components/Inbox.tsx 的站内询问入口、同目录 inbox/ 下询问列表/详情组件、tests/admin/inbox-ui.spec.ts 的询问场景。P1 静态断链，现场提交与处理 NOT_RUN。买家 /contact → buyer BFF → 加密联系记录；商家 /messages → 现有 contact-messages 列表/详情 → handled 命令 → 同一记录刷新。隔离买家提交一件得 201，商家正常找不到该条先红；绿须显示同 ID、标已处理后刷新保持。inbox:read 可看、无 inbox:reply 不能标记；跨店无法读取，重复操作及版本冲突不多写，标已处理零外发。B inbox 扩双端场景与 PG 回读，I01/I02/I11/I14。

#### N3 · 主播管理回放，买家从店面发现回放
父 T05/T08/T09/T18；依赖 TW02（owner 裁决首发范围）；ui_worker，`fix/n3-replay-discovery`；独占 A/components/Studio.tsx 回放控件、S/components/ShopChrome.tsx 店面入口、tests/admin/studio-ui.spec.ts 回放段。状态 P1 条件修复，缺真实来源资格另列。已结束、有官方视频链接的隔离场次 → 工作室开回放 → 已有 GET/PUT replay 应用端口 → live.sessions 独立 CAS 版本；买家从店面首页到列表、同一场次详情，再由有权商家撤回。旧源码两端入口缺失应红；刷新可见、撤回后公开详情 404 为绿。未结束、缺官方链接、他店、无 live:manage、过期版本均不得发布。B studio-ui、B storefront 扩两个原生入口；记录 fixture 来源，真实 Meta 视频/正式发布单列。I01/I02/I07/I10/I14/I17。

#### N4F · 售后、优惠码及客户备注切页保住合成草稿
父 T05/T13/T14/T18、OC-10；依赖 TW02；ui_worker，`fix/n4-form-drafts`；独占 S/components/order-status/ReturnRequestForm.tsx、A/components/{Promotions,CustomerDetail}.tsx 及独立业务用例（顾客用 tests/admin/customer-tags.spec.ts；售后采用买家旅程、优惠码采用原 promotions 模式）；与 TW08 的卖方队列路径无交集。P1/P2 按表单拆验：H 第 61、80 行观察到表单脱离，当前静态复核指出 hidden/blur 清空路径，客户备注的现场复现缺证。分别在买家填写原订单申请、商家建立限次优惠码、客户详情写备注后失焦/切页。旧代码字段丢失为红；同一有效身份恢复后字段仍在，未确认零写，确认后通过各域原命令仅写一次。换店、退出或权限撤销必须遮蔽并清除原草稿；私密内容不落日志或跨会话持久缓存。B returns-ui 加买家表单、B promotions、B customers-billing；三项分别读回申请、优惠记录和客户备注。I01/I02/I09/I11/I15。

#### N4T · 商家知道消息模板能否保存
父 T07/T09/T18、OC-12；依赖 TW02、N2；ui_worker，`fix/n4-template-intent`；独占 A/components/inbox/MessageTemplateForm.tsx、对应模板列表、A/src/features/messages/routes.ts 及模板专项用例。状态 WAIT_CONTRACT，现存新建页切页 clear、仅本地校验；02 核对已冻结的模板种类/发布权限，owner 裁决开放保存范围。若仅校验，从 /messages 可找到“检查模板”、结果注明仅校验，零写；若正式开放维护，仅对后端已支持的模板从正常入口编辑/提交并刷新读回相同版本，未支持的自定义模板不可假报成功。两种裁决均要求同一授权会话切页保草稿、换店清除；未经授权不得发消息，未知提交查原键。B inbox，`bash scripts/dev/test-local.sh --msg-templates` 核支持的种类/授权/重放；测试须在旧实现看到入口缺失或草稿清空，再验证获批方案。I01/I02/I07/I11/I14。

### TW14 · 台湾商家无指导走查及整店发布裁决
父 T12/T20/T21/T22、OC-14；unit t12-e2e、deploy-release；依赖 TW01–TW13（01、02 以签核原件，03–13 以同候选证据交接）、N2、N3、N4F、N4T；独立 test_worker 主持、security_reviewer 核隔离、integrator 裁决，`qa/tw14-tw-usability`；独占 `output/tw14-tw-usability/`。状态 HOLD。输入为获准同一候选、正常入口及五角色账号；输出为每项成功/失败/阻塞、受影响范围和证据指针。台币/繁中、1440×900 和 390×844 为主，三语长串、键盘焦点/返回、窄屏遮挡与屏幕阅读状态在既有对应模式留证。
向台湾店主、仓储与运营岗位参与者发任务目标，另由买家用独立会话操作。不给 URL、答案或口头导航；依次找新增草稿、改费并报价、站内询问与回放、表单切页、查单返回原车、换 B 店盘点、CSV、代客建单、发货/记收、处理第 101 件申请、买家换店取消与查单、认领、会员及核账。记录岗位/经验、设备、第一次选择、耗时、是否求助/返工、错误和最终事实；独立人员比较同 SHA 截图、菜单、金额/店铺/状态及 PG。任务需要提示、错店、错账或无法恢复，即该行红。记录参与人数及每项已测/适用总数；无人参加为 NOT_RUN，不从自动化推断易用。
B admin-parity、B click-sweep、B visual-lint、B webkit 与各业务卡的原生 E3 组成 CI，另需获准商家现场 E4。隔离 CI 绿仍单列 provider、真实设备/岗位、外部授权和部署演练空项；保留 T19 性能证据、T21 独审及 T22 原 gate，P0/P1 或必过门缺证保持 HOLD。附恢复资格：在同一会话重开购物车时辨明 full 与 view-only，购物车报价过期重报价、跨标签目的地重确认、拒付/取消/3DS 回原订单重读，原键结果未知不重复付款；B checkout-offline、B payment、B buyer-after-parity，外部证据标 SANDBOX/LIVE/NOT_RUN。I01/I16/I17/I18。最终产出为逐项准入建议，正式上线由 owner 决定。

## 范围与交接

未被上述问题点名的现有能力同样进入 01 分母和 14 准入表。店面/域名以 T05 与 store-design、store-domains、storefront-publish 原单元验；广告、CAPI 和归因以 T15/T16 原单元与授权验；多语言、性能和恢复用 T19/T20 原验收；未适用、外部条件不足、未复验逐行标注。资格卡发现新产品缺陷，回原领域分配独占修复卡；保留原测试和 RED 再独审，资格卡不兼任作者。本文的验收命令是未来执行入口，当前批次仅交 DESIGN 任务书。
