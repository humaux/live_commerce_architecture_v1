# 商家与买家的界面缺口

先登记问题，再改正式设计。本表基于 2026-11-22 的集成版本 `12c60ea6` 与试点 `a2d079ca`。两树均有 44 个后台、21 个店面页面模板。65 是路由模板数量，同一路由有多种角色和状态。Figma 正式页 `4:8829` 当次读到 3,829 个直接子项，其中 3,695 个为可见 FRAME，包含旧版和替代稿。完整签核帧数和整店通过率仍未知。

## 合并裁决与修改顺序

| 用户操作 | 合并后的发现 | 设计或实施去向 |
|---|---|---|
| 已开店改配送 | 日常操作绕入旧四步，手工配送呈现绿界凭据，摘要“未配置”没有解释是未选服务还是读不到 | 修设置总览及单项运费双端稿。按实际状态显示已启用、无服务或读取失败。列服务、固定运费及免运门槛。凭据归主动进入的物流连接 |
| 开店后新增第一件商品 | 旧 AD-03 把可跳过的商品草稿排成 3/3。已有独立商品编辑器 | 结束建店回工作台，从商品列表新增；与日后新增共用编辑、校验和保存。旧引导退场时恢复已有草稿及未决命令 |
| 商品列表找批量导入 | 原店正常进入商品操作只见复制商品、库存台账。历史上直接访问可预览 | 在有资格的商品列表接到既有导入入口；预览、逐行错误、整批确认、结果再读纳入任务卡 |
| 切店盘点及仓储角色 | 在已授权 A、B 两店的测试中，选 B 的地址读 A；仓储角色遇到导航及台账权限两处断路 | 先修服务调用和职能权限，突出店与仓。真实库存误写未验证。不能通过扩大整套目录权限修复 |
| 售后及换取件门市 | 买家申请与商家 RMA 列表未接通；取消换门市会清掉已确认选择 | 售后对照 `1468:107474 / 1468:107559` 现有处理稿后补缺；门市复用 `1431:71232 / 1431:71249`，先修取消与确认各自的状态 |
| 订单收款与结算 | 登记已收货款后列表摘要仍可能待收；部分岗位看得到无权使用的入口 | 摘要与专用收款区读同一收款事实；资格与后端一致，已交付、待收款、已收款分别呈现 |
| 查一笔订单再回店购物 | 手工单链接和访客查单会替换同浏览器购物会话。两次换 cookie 已离线复现；旧现场见购物车内容变化及原单失联 | 查单权限明确限定为该单；返店能恢复原购物车。正常会话、单笔只读、过期、返回后加购分别验收 |
| 买家给商店留言 | 买家端和读写后端已具备，商家收件匣没有商店询问入口 | 复用站内询问与标记处理命令，区分站内询问和社交平台回复权限 |
| 直播回放 | 已有发布 BFF 与回放列表，商家缺发布控件，买家缺默认可发现入口 | 对照已有商家回放及买家列表双端稿，补发布、撤回、买家进入与空态。无资格时不给出可发布的承诺 |
| 填写后切标签 | 商品编辑器已有修复。售后失焦卸载，优惠码与客户表单切标签卸载，消息模板切页清空且只有本地校验 | 离开再回来应保留同一店和身份下的输入；退出或失权时保护原敏感数据。模板本地检查与已保存须有不同结果 |

## Figma 改稿简报

1. 已开店店主从设置能找到当前服务和费用，从商品列表能添加第一件商品；日常操作去掉开店步骤计数。
2. 优先改设置总览、单项运费、商品草稿三组桌面和手机状态，后续组按现有正式节点逐项比对。
3. 设置只列本店已启用服务、金额与免运门槛；查看、编辑、保存、冲突和待确认有明确返回路径。
4. 商品保存后能看到草稿状态。库存、订单与售后操作始终显示正在操作的店，角色受限时只提供可用动作。
5. 复用正式变量、字号、组件及自动布局。正式帧绑定变量与 `packages/ui/src/tokens.css` 的 DS-1/DS-3 为依据（导航 --ds-nav-bg、画布 --ds-page、主动作 --ds-primary、正文 --ds-ink）。未迁移页面以 --ui-* 为界，代码执行现有 check-hex-ratchet.mjs，原有差异单独留证。
6. 桌面保留服务和价格的横向比较，手机纵向显示名称与金额。检查窄屏长文案、键盘顺序和焦点恢复。
7. Eagle 对照 Shopify 的配送分组与 Squarespace 的订单筛选；负例是把开店清单长期放在日常工作台。
8. 改前导出原稿、记录源 ID。改后回读新 ID、节点与交互，导出同尺寸图，测量 token diff；用三问检查所在店、可做动作和结果。
9. 正式文件与批复以实际 Figma 节点及独立截图回执为准。UI 设计、隔离运行、原店复验分别记证。

## 本轮正式节点逐项裁决

| 任务 | 桌面 / 手机源 | 核对结果 |
|---|---|---|
| 日常设置与改单项运费 | 总览 1429:32319 / 1429:32392；服务 1468:113545 / 1468:113598；满额编辑 1468:113719 / 1468:113779；宅配 1468:113817 / 1468:113873；回执 1468:113959 / 1468:114007 | 重排已开店总览，指向对应服务。在超商修改和回执采用同一组费用和门槛 |
| 首件与日常新增 | 空列表 1253:16665 / 1253:16710 → 空白编辑器 858:5505 / 858:5506；非空列表 1429:62828 / 1429:62906 | 已读空列表原型的跳转，复用空白编辑器。非空列表缺可见新增，补同一动作，设置总览移出商品表单 |
| 售后待处理 | 商家 1468:107474 / 1468:107559 → 1468:86170 / 1468:86221；买家 1468:99242 / 1468:99291 | 商家“处理退货”指向登记 RMA，买家待商家确认只支持返回列表。复用候选页已画出的待处理、决定、撤回及反馈结构，回正式页设计修订。批准关联 RMA、拒绝须有原因，消费者在待处理时可撤回 |
| 消息及门市 | 早期 Meta/客服的 4:2881、4:3429、4:3528、4:3823 实际均在备份页 11:13810。售后候选页为 1763:26。现行门市核对 1431:71232 / 1431:71249 | 初验表内这些历史节点按所属页解释。门市核对、店内消息和回放优先接通已存在的正常入口，复用正式稿可表达的业务状态 |

## 初验矩阵与原始证据

以下保留设计岗最初的 14 组记录（1 项设计需改、3 项实现偏离、10 项缺验证）。上方合并裁决已纳入随后到达的商家和买家审查。例如 06 已有选店错读的定向反例，03 的业务入口按本次裁决重排。两者统计口径不同。

历史材料 H 指 5dffb337 原店走查，P 指 a2d079ca 原店复核，D 指 2026-10-25 正式 Figma 导出，Q 指首轮独立设计对照。矩阵内的历史 PNG 路径相对项目的 output/figma-ui-completion-20261025，Q 对应 output/merchant-product-architecture-e127df97/quality/findings.md。正式文件 key 为 `iJ9lDdFIXgTlKkt5Tvrc0X`。页面树清单 SHA-256 为 `737b9be3dece307bd85945e17de96482dc067350bc344a0673c3a8cfd981d85d`。

| # | 分类、入口和状态 | 正式设计、实图与证据边界 | 操作反馈及验收 |
|---|---|---|---|
| 01 | **缺验证** · 商家根页工作台、店切换、岗位待办、空/错误/无权限 | `4:12467 / 4:12615`。D `full-review/figma-brand-final/4-12467-official-brand-after.png`、`full-review/admin-mobile-review/4-12615.png` 历史设计；现行 `Dashboard.tsx:41–75`，H 有新商店建立和刷新，岗位×设备的实际截图 UNKNOWN。 | 登录后从看板切换授权店，继续点订单和库存，店名、列表和待办同店；切换或禁止时清除上一店数据，空和读失败分开、失败能重读，限制角色 UI 和服务器结果一致。现有 shell/browser 及 dashboard 测试逐项绑定。 |
| 02 | **缺验证** · 商品列表 → 新增、编辑规格/主图、草稿、切页保持、冲突/未知结果 | 当前索引 `1429:62828 / 1429:62906` 列表、`1429:64000 / 1429:64049` 编辑；`4:10436 / 4:10599` 已标取代，`858:5505 / 858:5506` 为历史新增。D `product-editor-flow-20261026/after-review/4-10436.png`、`product-editor-mobile.png` 和 `CURRENT-FINAL-READBACK.json`。H 旧切页清空 RED 后，`7bb2985f` 桌面同文档重挂载和同图 RGBA 复验 PASS；当前两 SHA 的跨刷新/会话及手机证据 UNKNOWN。 | 走 `/products` → `/products/new`，单规格价、库存、主图仅存草稿，出现可重开的稳定身份；刷新核字段、SKU、媒体顺序、唯一初始库存入账，买家不可购。对未保存离开保留草稿或明确取消；未知提交查原回执，冲突保留可重读上下文。沿用 `--browser-catalog-core` 及媒体模式。 |
| 03 | **实现偏离** · 快速开店 3/3 首件商品的构图与动作层级 | `1429:33613 / 1429:33693`。Q 记录本日正式桌面 PNG 1440×1000，SHA-256 `06e6f2ac84e844f33537df7bf4b8c17a0b784cf1ac322a30d6965e6272de59f4`。owner B 截图及旧 browser 附件呈宽式左对齐、按钮挤在左下。当前 owner 截图的确切 SHA/视口/路径及手机原件 UNKNOWN。 | 快速开店的可跳过草稿和「上一步／核对设置」保留任务上下文，桌面同状态双列动作、主次清晰；三语 390px 长串不遮挡。日常建品依 02 验收，独立于开店流程。 |
| 04 | **设计自身需改** · 已开店日常运费、启用配送、高级物流三类入口辨识 | 主控现读 `1429:32319`；手机总览签核索引 `1429:32392`。快速配送 `1429:33321 / 1429:33467`，历史组合配送 `6:21203 / 6:21322`。现读总览图将快速设置置于主按钮，配送摘要只示已开启 0 种，固定运费直达及已开服务状态未表达；已启用服务的总览帧 UNKNOWN。当前根索引已有运费专题 `1468:113545 / 1468:113598`、预设免运 `1468:113629 / 1468:113685`、满额免运 `1468:113719 / 1468:113779`，其像素、交互和签核 UNKNOWN。D `settings-20261026/delivery-D.png` 确认历史构图把多方式费率与一次保存放在同页，owner A 是高级集成截面，不能与总览作像素对比。 | 已有服务时，直接从设置进入固定运费，先明确品牌/服务和币种，再显示单价；近邻展示免运门槛是独立报价条件，编辑高级集成使用明确入口。准备未配置、未变化、保存中、成功、冲突/未知的短状态。设计验收两端均能从总览识别任务与结果。 |
| 05 | **缺验证** · 运费单项保存、免运后的报价和旧订单冻结 | 同 04，当前运费专题节点已定位，单项收费、成功与未知回执的精确帧 UNKNOWN。P 的原店设置读到宅配 60、超商 1000，超商命中 100 元门槛故新报价 0，两个报价均 200；无现场保存。源码 `SettingsAmountEditor.tsx:135–167` 已有 fee selector、禁用/提示、重读和 same intent。 | 相同测试车且未达免运门槛，改宅配 60→80，保存成功后刷新仍 80，新报价只涨 20，其他服务、旧单金额不动；另跑达门槛的零运费。验证一次 `PUT …/flat-fee` 的双版本提交，失败或未知保留明确的查询/同原意图恢复。复用 `settings-real.spec.ts:483–503`（`--browser-identity`）及 P 的 PG 见证。 |
| 06 | **缺验证** · 库存 SKU 选择、数量原因、校验、冲突/未知与台账 | 列表 `21:480 / 21:899`，调整 `6:19050 / 6:19202`，台账 `6:22818 / 6:22986`，复核/成功/冲突/未知 `840:5246–5253`。D `product-inventory-20261026/{desktop-inventory-after,mobile-inventory-after,native-mobile-B21-matching-receipt}.png` 有历史设计和原型；H 的原店已测调整范围另见原回执。该 SHA 负库存、竞态、跨仓/切店与视觉绑定 UNKNOWN。 | 从正常页选仓库及唯一 SKU，输入有效增量和原因，核对调整前后现货/可售及仅一笔台账；缺原因/负数下限不提交，冲突重读同 SKU，未知用原操作查核，移动台账回返保留上下文。 |
| 07 | **缺验证** · 订单、出货、拣货、面单/门市配对与配送、待收/已收 | 当前 `1429:25977 / 1429:27276` 全部订单、`1468:83732 / 1468:83778` 拣货、`1431:70774 / 1431:70793` 门市工具。`4:8904 / 4:9104` 已标取代。D `merchant-fulfillment-20261026/{desktop-orders-after,mobile-orders-after}.png` 与 `DELIVERY.md` 为历史设计。拣货/门市工具根已定位，同状态 PNG 与交互证据 UNKNOWN。P 已有建单选品 2→3→2、两组配送、姓名手机校验、返回修改；H 原单曾出现配送摘要待收与专用收款已收不一致，当前 a2 该终态 NOT_RUN。 | 按同订单核选品/SKU/拣货量、门市、运单、履约和应收；标记取付/代收后两处同义状态一致、金额和刷新结果一致；未取退回给手工收回/恢复库存反馈，不能隐式复原。实际出单依厂商凭证分层，角色和购买门禁随原方案。 |
| 08 | **实现偏离** · 买家申请进入商家售后队列并可处理 | 契约钉商家列表 `30:1662 / 30:1731`、详情 `30:1757 / 30:1818`；当前正式页面另有 `1468:107474 / 1468:107559` 买家申请处理根，签核及原图 UNKNOWN；D `full-review/{admin-desktop-review/30-1757.png,native-review/desktop-30-1757-aftersales-detail-after.png}`。H 06:27 原单唯一 requested v1 无 RMA，商家正常列表 0 行，07:15 再验未见。两 SHA `ReturnsList.tsx:210–244` 只走 RMA 清单，Go `internal/httpapi/return_requests.go:29` 有独立申请路由，Admin 组件中无申请接口引用。P 未回归，部署同状态仍 UNKNOWN。 | 买家提交后商家刷新见唯一对应申请，而原 RMA 区保持各自语义。提交审批或拒绝后展示真实结果，刷新和买家原链接同请求；同版本并发只有一个终态。批准才创建关联 RMA，退件/退款/库存各有独立动作和回执；权限错误不泄露申请。复用 returns UI 门禁并加入真正的双端申请旅程。 |
| 09 | **缺验证** · 当前正式售后详情的完整决策及撤回覆盖 | `30:1757 / 30:1818` 历史所见为待商家确认后引向退款，没有同意/拒绝。买家 `26:815 / 26:1087` 的 2026-11-13 原始树/PNG 未见撤回入口，`m8-aftersales-design-inputs-1113/REPORT.md`；`contracts/returns-v1.md` Amendment 0210 为 authority。候选 `1763:26` 的批准/拒绝/撤回稿不计入正式覆盖。当前整页索引另见买家 `1468:99242 / 1468:99291` 和商家 `1468:107474 / 1468:107559`。前两节点由 M8-AFTERSALES-ADMIT 报告列为候选线索，后两节点签核/控制树待读。当前正式变体是否仍缺审批和撤回 UNKNOWN。 | 只在符合条件时展示审批、拒绝或撤回及其后果，明确结果/失败/版本冲突；买卖端同申请链绑定。退款由原订单及资金权限另行触发。先核现读 D/M 两端，签入对应状态后逐帧验证。 |
| 10 | **实现偏离** · 买家结账更换门市后取消会清旧确认 | 当前站内目录 `1429:14065 / 1429:14122` 搜索、`1429:14738 / 1429:14800` 暂选、`1431:71232 / 1431:71249` 核对，由固定 SHA `tests/storefront/cvs-picker.mjs` 绑定。`18:366 / 18:415` 已标取代，`1198:7656 / 1198:7763` 为历史编辑态。D `checkout-20261026/verify/M-18-415-checkout-store.png` 实源图，历史可见原卡、换店和编辑动作。当前目录设计原图 UNKNOWN。留存 11-07 运行图 `output/fleet-control-20261107/ci-pr440-1182/selected-artifacts/gate-browser-webkit/playwright/run.eMAKfWUy/taiwan-cvs/20261107T130834.625115663/parity/{1429-14800,1431-71249}.png`，`1204:*` 明标地图非主路径，需分开。H 取消立即关闭和选候选后取消均留 RED，P 未验证。两 SHA `CvsPickup.tsx:235–240,605–612` 打开时清 store/callback，关闭只关 picker。 | 在同店同品牌确认 A → 打开换店 → 直接关，以及选 B 未确认再关，两条都保留 A 的品牌、店号/名/地址、收件字段及既有报价/目的地状态，零确认请求或订单写；确认 B 才替换，真实换品牌须重新确认且不串门市。现 `tests/storefront/cvs-picker.mjs:489–501` 明断关闭后无已验证文案，不能把该绿当作恢复 A。这条需改成先证 RED，再保留原取消语义做精确回归。 |
| 11 | **缺验证** · 390px 可达性、键盘、长文案、多端状态一致 | 结账前 `782:3904 / 782:4087`，核对 `1429:23435 / 1429:23484`。D `buyer-browse-cart-20261026/independent-after-mobile/782-4087.png`，历史 `output/m8-checkout-782-706/REPORT.md` 有 706 隔离截图＋三语 E3、1429 不变像素回归，1429 手机正式直出缺失。部分旧手机稿首屏底栏盖付款区下缘，滚动是否可达及当前真实设备 UNKNOWN。 | 1440×900、390×844，必要时 375px，最长三语、键盘打开/关闭、系统返回。付款/店号等字段可滚到固定栏之上，焦点不被盖住，主要目标至少 44px，焦点返回来源控件，信息/金额/店铺上下文与桌面等价。分清 782 取价与 1429 最终建单阶段。 |
| 12 | **缺验证** · 直播场次/留言/库存及认领、未连接/等待/降级 | `tests/admin/live-console.spec.ts:931–945` 绑定 `1429:39502 / 1429:39607` 等控制台样本，`4:10687 / 4:10860` 已标取代；D `live-console-20261026/DELIVERY.md` 和 `full-review/figma-native-copy-final/M-4-10860.png` 历史原型。现有 `--browser-live-console(-parity)` 含真实页面与 MOCK 业务/Graph 边界，真实提供方和该 SHA 全程图 UNKNOWN。 | 正常目录开同一场，媒体、评论接入、消息发送权益各有状态；暂停、空、权限拒绝、断线复连可区分。切场/切店清旧数据；待确认操作可查原命令，确认后目标 SKU/认领/库存保持一致。 |
| 13 | **缺验证** · 收件箱、模板、消息窗口/权限、失败与未知结果 | 历史 Meta `4:3528 / SPEC 4:3823` 与网站客服 `4:2881 / SPEC 4:3429` 是早期分域稿；当前根 `1429:37361 / 1429:39076` 对话、`1429:38090 / 1429:39229` 窗口关、`1429:38274 / 1429:39307` 失败保文。签稿像素 UNKNOWN，模板 `673:2847 / 673:2916`。源码 `apps/admin/app/[locale]/messages`、`internal/httpapi/{inbox,templates}.go`，现有 `--browser-inbox` 原生点击覆盖夹具，LIVE Meta NOT_RUN。 | 真实店内测试线程进入正确渠道、正确会话，权限/窗口限制阻发且给下一步；明确失败保留草稿，响应未知保留原意图并查回执，切换不把文本发到别的会话。保存模板重开读回；真实发送资格另由独立沙盒/渠道回执证明。 |
| 14 | **缺验证** · 财务/报表、结算/退款、无数据和导出/拒绝状态 | `4:12097 / 4:12246`、`1173:10738–10747`；D `finance-20261026/verify2/mobile-finance-home-4-12246.png` 与 `DELIVERY.md` 记录设计。H 有独立同单 TEST 收款/退款触点，覆盖不延伸到该 SHA 财务界面三语/双端。 | 切换日期、币种及 TEST/LIVE 过滤后，收款、履约、退款、结算和应收由各自权威事实回读，日期保留，空态和无权区分；导出遵守相同筛选与权限，金额逐笔对账。 |

## 修订画板及回读

2026-11-22 最终留存 **50 张双端画板、271 个可点控件、307 项顶层点击动作**。一处点击可能依次更新原型状态并导航。图稿和元数据逐根绑定，索引 SHA-256 为 `383fc7865399acb34897f39625855b8597736b1e11f33f488b1570244aadcddb`。下面的 32 张为首批修订，后补 4 张宅配及 14 张商品／筛选画板。

在桌面和手机分别实点保温杯售价，从 NT$490 试改为 NT$590。取消后回到列表仍为 NT$490。保存后列表与再次进入的保温杯均为 NT$590，棉质上衣仍为 NT$690，帆布提袋仍为 NT$1,280。每端按 12 张播放器截图核价。示范采用预置金额，动作只改变播放器内该 SKU 的状态。

繁中菜单已校正，手机商品菜单三项的点击面均为 358×44。已撤回采用正式中性状态，已拒绝及拒绝操作采用正式危险状态。50 张画板均与各自的来源对照。确认撤回的桌面页脚底边为 898/900，两个宅配桌面页的商品导航均回到本次非空商品列表。其余设计签核范围包括旧页面与现行 DS 的颜色对应、两种配送之外的服务、CSV 后续批次操作，以及订单摘要与退货卡片的品名一致性。原型使用的三组商品状态变量独立于设计颜色及真实商品数据。

正式文件页面 `4:8829` 中，首批 16 个业务状态各有桌面和手机画板，已按 48 条操作连线核对目标并导出 32 张原尺寸 PNG。另补两端宅配编辑及保存回执。首批 32 根与各自来源比较，新增视觉变量别名、填充或描边颜色、样式 ID 均为 0。后补的桌面宅配回执复用了同端次按钮实例 1468:113871，带入原回执未使用的边框变量 3:13；原生读回确认该按钮与来源同一主组件，填充、描边及样式完全一致。删去旧控件后，部分旧变量已不再使用。单根移动版筛选溢出已修复，终版几何回读无横向越界。

普通运费编辑保留费用、免运开关与门槛，移出填写变更原因的额外要求。以 NT$135 商品小计演示两种配送，商家能看到固定运费、何时免运和买家合计。

| 业务状态 | 桌面修订 | 手机修订 |
|---|---|---|
| 日常设置与两种运费 | [1791:52720](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-52720) | [1791:52816](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-52816) |
| 编辑 7-ELEVEN 运费及免运门槛 | [1791:52912](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-52912) | [1791:52969](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-52969) |
| 保存费率后读回 | [1791:53004](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53004) | [1791:53047](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53047) |
| 单独编辑宅配费用 | [1791:66505](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-66505) | [1791:66561](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-66561) |
| 宅配费已保存 | [1791:66595](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-66595) | [1791:66647](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-66647) |
| 非空商品列表新增商品 | [1791:53068](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53068) | [1791:53156](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53156) |
| 商家待处理申请 | [1791:53956](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53956) | [1791:54001](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-54001) |
| 商家批准或拒绝 | [1791:53597](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53597) | [1791:53639](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53639) |
| 拒绝原因尚未填写 | [1791:53659](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53659) | [1791:53710](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53710) |
| 填写拒绝原因后可以提交 | [1791:53739](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53739) | [1791:53790](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53790) |
| 已批准并关联退货单 | [1791:53819](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53819) | [1791:53874](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53874) |
| 已拒绝并保留原因 | [1791:53894](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53894) | [1791:53936](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53936) |
| 买家申请列表 | [1791:54024](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-54024) | [1791:54038](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-54038) |
| 等待商家处理，可撤回 | [1791:53264](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53264) | [1791:53302](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53302) |
| 买家确认撤回或继续等待 | [1791:53331](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53331) | [1791:53374](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53374) |
| 已撤回 | [1791:53408](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53408) | [1791:53444](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53444) |
| 买家查看已批准 | [1791:53471](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53471) | [1791:53507](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53507) |
| 买家查看已拒绝及原因 | [1791:53534](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53534) | [1791:53570](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Live-Commerce?node-id=1791-53570) |

这些回执覆盖设计节点、目标动作和图片。店铺网页实际写入、隔离数据库、真实商家无指导任务与生产发布由业务卡另行验收。全文件 REST lint 在本机缺少 FIGMA_TOKEN，未取得该项报告；已有节点级原生回读单独归档。

## 逐件核对商品与筛选

旧列表的三行曾指向同一件帆布提袋，编辑器与列表连售价也不一致。两端各补三件商品的载入态，并将“全部 → 在售 → 草稿 → 已归档”和勾选状态接回各自的正常列表。选中的筛选保留选中外观，点击其他筛选才切换。普通商品列表保留新增、逐件编辑，并从商品操作进入既有 CSV 导入与匯出。店内实际上传、导入、库存写入和商品发布仍由 TW06 验收。

| 目录状态及商品事实 | 桌面 | 手机 |
|---|---|---|
| 帆布提袋 · NT$1,280 · 在售 · 可售 3 | [1791:66709](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-66709) | [1791:66775](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-66775) |
| 棉質上衣 · NT$690 · 在售 · 售罄 | [1791:66920](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-66920) | [1791:66971](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-66971) |
| 保溫杯 · NT$490 · 草稿 · 可售 16 | [1791:66822](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-66822) | [1791:66873](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-66873) |
| 只看在售 | [1791:67018](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-67018) | [1791:67092](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-67092) |
| 只看草稿 | [1791:67176](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-67176) | [1791:67237](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-67237) |
| 只看已归档（空列表） | [1791:67307](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-67307) | [1791:67354](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-67354) |
| 已选当前页 | [1791:67414](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-67414) | [1791:67504](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1791-67504) |

设置总览每端恢复店铺资料、收款、账单、使用说明、资产授权等六个正常去向，失效的“加入团队”从此处移出。邀请收件人从自己的邀请链接办理。商家售后队列亦可查看所选申请的订单。手机撤回确认页的“我的订单”点击区域为 y=789 至 833，高 44 像素，处于 844 像素画面内。

工程师在 Figma 播放器实际走过手机宅配保存、买家撤回页返回我的订单、设置进入店铺资料、商家售后进入订单、手机跨筛选打开保温杯、桌面三件商品分别进入并返回。回执和原尺寸图指向具体画板。应用持久化与商家无指导人测仍依 TW14。

日常编辑须从同一商品保存或取消。未修改时显示当前售价，改动后再允许保存，取消后列表仍读原价。保存后打开相同商品及其筛选列表均须读到新价，其他商品保留原价。可售库存沿用库存模块的真值。Figma 播放器可使用预置金额演示此过程，任意金额、员工权限、并发改价及刷新持久化由应用和数据库验证。

台湾繁中菜单、手机操作区 44 像素、拒绝与撤回的状态颜色，以及多个历史页面调色板与现行 DS 的差异归入 TW02b。改图时按正式组件及令牌核对，原型演示、局部令牌对照与全文件 lint 的结果分别记录。目录的三件示范商品用于核对身份，保存回执须同时核同一商品和金额。
