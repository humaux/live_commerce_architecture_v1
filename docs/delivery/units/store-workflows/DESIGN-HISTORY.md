# 直播 SaaS 的 Figma 历史与本次取稿依据

2026-11-22。团队文件搜索“直播”打开的结果、设计页分享链接和仓库历史交付共同指向同一个文件。Figma 文件名为 Untitled。文件菜单的所属文件夹链接打开 Team folder。该目录当时展示三份设计，两份同名 Untitled 和一份服装 B2B。另一个 Untitled 的 key 为 `8NXcud0qbNgY95j2W65rJn`，仅见 Page 1；与直播产品的继承关系待证。此记录的覆盖范围是当前账号可见的文件夹与已识别文件。[文件夹](https://www.figma.com/files/team/1673558906340091320/project/643688742) · [主设计页](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=4-8829)

| Figma 中看到的对象 | 核对结果 | 设计取舍 |
|---|---|---|
| 文件版本历史 | 展开较早记录，侧栏显示一组 47 次自动保存，并有至少 38 条名为 Ready for dev 的记录。多条备注只交付静态设计，实际点击、持久化或运行验收留待后续。仍有“Show older”入口，统计为已见下限。 | 按场景核对版本说明和签核人。自动保存时间不能证明商品、售后或配送在该版本验收通过。 |
| 同一文件的七个页面 | `4:8829` 为主设计页；`3:1819` 为页面地图，`3:38` 为组件令牌，`3:565` 为旧后台索引，`11:13810` 为备份，`1763:26` 为售后候选，`0:1` 为 Scratch。 | 从业务入口定位原图，再按控件、交互和代码契约核对。 |
| 主设计页本次改稿前的直接子节点 | 3,829 个，其中可见 FRAME 3,695 个。按名称前缀计，DESIGN v2 有 2,383 个，DESIGN v3 有 152 个，已被取代有 197 个，SPEC 有 192 个，ROLLBACK 有 21 个。 | 152 个 v3 都属于直播控制台。商品编辑、改运费、退货审批各有自己的沿革。回滚副本和已退役目标保留作历史对照。 |
| 32 个首批修订的来源 | 10 个来自买家售后候选页，8 个来自契约标注 PROVISIONAL 的画板，6 个以登记退货单的布局呈现买家申请，另外 8 个源于设置、运费和商品列表。 | 商家审核应另对照申请求助队列。建立退货单、批准申请、退款对应三个业务动作。 |
| 后补两端宅配单项及保存回执 | 桌面 `1468:113817 / 1468:113959`、手机 `1468:113873 / 1468:114007` 提供布局。本店已测小计 NT$135、宅配 NT$60、总计 NT$195，宅配免运门槛未测得。 | 商品与支付设置仍从自己的入口办理。宅配编辑只展示此服务的费用，回执可返回总览。 |

两个固定代码版本为集成 `12c60ea6` 与试点 `a2d079ca`。下方列出原稿依据和逐状态映射；本次新画板进入 UI 实现前仍需负责人签核。[业务流程](README.md) · [现行修订与验收](UI-GAPS.md)

## 签核与裁决顺序

1. 10 月 25–26 日的 SHOPLINE 页面与备份保留旧稿。10 月 26 日 AD-03 规定新店可跳过的三步「店名 → 配送 → 第一件商品」，既有店无需重走；四步 provider wizard 移出商家界面。见两 SHA 的 `contracts/merchant-settings-wizard-v1.md:137`。
2. 10 月 28 日 owner 就精简商品目录/网店编辑器回复「嗯，可以，不要配置太复杂了」。Humaux human `4462d488-7102-4eb4-ae65-52c301b31abe`；两 SHA 的 `output/catalog-v2-impl/PLAN.md` 将商品模块签收定位到 15 对 1429 根与 89 条精简文案。签收适用于对应场景；已有商品编辑 `1429:64000/64049` 与空白新增使用不同业务身份。
3. 10 月 29–30 日设置 polish 后，`output/parity-settings/DELIVERY.md` 记录设置总览和单独的三个步骤。10 月 30 日 AD-04 human `4c07f243-21b3-49bc-88fd-11e15fb275a6` 要求运费默认 0、多选和全选，已写入两 SHA 的 settings 契约末节。覆盖快速上手/总览配送开关；日后改某一服务的价格仍是独立任务。同日 K2/K4/K7 卡要求采用各模块 DELIVERY 当前节，排除 BLOCKED/PROVISIONAL，且有 owner 共用商家 shell 选项 A。K7 的原卡 `output/integrator/tools/cards/cxf-parity-k7-settings-reports.md` 和 human `cc776b52-4456-4f76-a1e6-5ed50ee72907` 记载 67 对专门状态及只新增入口的范围；这些材料未给本轮每一帧逐项 owner 签核。
4. 10 月 31 日 `contracts/returns-v1.md:151` 明定买家申请/详情 `26:773/1050/815/1087`、商家列表/详情 `30:1662/1731/1757/1818`；第 158 行区分申请和 RMA，只有批准才建 RMA。11 月 1 日 `output/parity-newfeat/FRAME-COPY.md:150` 起逐条保留原 SPEC：`1468:99242/99291` 申请详情、`1468:99605/99653` 申请列表、`1468:107474/107559` 商家详情均 PROVISIONAL。实施原卡 `output/integrator/tools/cards/cxf-parity-newfeat-impl.md:2` 明确从该轮排除。文档标题“signed”只适用于六个实际导出的入口。契约锚点用于核对业务及旧画面，不能替代旧图未画出的撤回、拒绝等状态。
5. 11 月 13–14 日 M8 按 `26:*` 受保护源创建独立候选页，经两组原地修订。第一组 8 根仅 4 个改变，第二组核准/拒绝 4 根同 ID 修文案与结果。见 `output/m8-aftersales-design-candidate-1113/{REPORT.md,revision-1/REPORT.md,terminal-states/revision-1/REPORT.md}`。候选像素获根级审阅，允许原型/隔离实现参考，未找到其晋升为正式历史版本或 owner 签收的记录。Amendment 0210 本身也未规定只有正式晋升才可在隔离分支写实现。
6. 11 月 22 日 owner 对当前操作方式提出异议，human `7a47ece8-da8a-4e0c-b850-d5ec77968d25`。同日 `output/release-gate/a2d079ca-ego-checkout-acceptance.md` 已读到原店超商费 1,000、满 100 免运、宅配费 60，同一 135 购物车报价分别为 135 和 195；这支持修订使用的店铺数据。该次现场止于建单预览。经营路径文档 `output/merchant-product-architecture-e127df97/商家产品架构.md:16` 将日常入口重组列为提案。意见、提案与修订节点不构成逐帧批准，原店数值也不是新店默认值。

## 32 个根按实际用途核对

表中同一行的两个 ID 分属来源和本次修订。均使用前述文件 key。判断针对所列业务状态；跨文件替代均待项目目录与逐场景签核对应。

|修订项|源节点|新节点|源场景与替代判断|
|---|---|---|---|
|desktop-overview|1429:32319|1791:52720|设置 v2 总览，历史入口已由 AD-03/04 按新店/日常任务限定；未证实后来同场景签核的替代根。|
|desktop-fee|1468:113719|1791:52912|K7 满额免运编辑结构适用，原画面是宅配示例。此处改作原店超商费率；无较新签核替代 ID。|
|desktop-saved|1468:113959|1791:53004|K7 储存回执。原画面成功值不能充当这一次保存事实；未发现同场景经签替代。|
|mobile-overview|1429:32392|1791:52816|设置 v2 总览，历史入口已由 AD-03/04 按新店/日常任务限定；未证实后来同场景签核的替代根。|
|mobile-fee|1468:113779|1791:52969|K7 满额免运编辑结构适用，原画面是宅配示例。此处改作原店超商费率；无较新签核替代 ID。|
|mobile-saved|1468:114007|1791:53047|K7 储存回执。原画面成功值不能充当这一次保存事实；未发现同场景经签替代。|
|desktop-catalog|1429:62828|1791:53068|签收的 1429 非空商品列表，新增 CTA 属本次补充。指向 858 的普通新增，签收编辑页另为 1429:64000/64049。|
|mobile-catalog|1429:62906|1791:53156|签收的 1429 非空商品列表，新增 CTA 属本次补充。指向 858 的普通新增，签收编辑页另为 1429:64000/64049。|
|desktop-buyer-pending|1763:105|1791:53264|M8 候选请求详情，历史正式锚点 26:815/1087。1468:99242/99291 是暂定等待详情，无撤回按钮。尚无可替代整套撤回流程的已签根。|
|mobile-buyer-pending|1763:141|1791:53302|M8 候选请求详情，历史正式锚点 26:815/1087。1468:99242/99291 是暂定等待详情，无撤回按钮。尚无可替代整套撤回流程的已签根。|
|desktop-buyer-confirm|1763:168|1791:53331|M8 候选撤回确认；1468:99057/99109 确认的是新申请。未取得撤回确认的另一已签根。|
|mobile-buyer-confirm|1763:204|1791:53374|M8 候选撤回确认；1468:99057/99109 确认的是新申请。未取得撤回确认的另一已签根。|
|desktop-buyer-withdrawn|1763:231|1791:53408|M8 候选已撤回状态。0210 支持 withdrawn；当前查域无替代签核根。|
|mobile-buyer-withdrawn|1763:267|1791:53444|M8 候选已撤回状态。0210 支持 withdrawn；当前查域无替代签核根。|
|desktop-buyer-approved|1772:333|1791:53471|M8 第二组候选同 ID 修订一，结果读关联 RMA 当前状态。当前查域无替代签核根。|
|mobile-buyer-approved|1772:366|1791:53507|M8 第二组候选同 ID 修订一，结果读关联 RMA 当前状态。当前查域无替代签核根。|
|desktop-buyer-rejected|1772:390|1791:53534|M8 第二组候选同 ID 修订一，按实际拒绝原因显示。当前查域无替代签核根。|
|mobile-buyer-rejected|1772:423|1791:53570|M8 第二组候选同 ID 修订一，按实际拒绝原因显示。当前查域无替代签核根。|
|desktop-merchant-decision|1468:107474|1791:53597|NEWFEAT 的 PROVISIONAL 待审详情；源 CTA 为处理退货。契约正式商家详情 30:1757/1818，完整批准/拒绝选择画面签核 UNKNOWN。|
|mobile-merchant-decision|1468:107559|1791:53639|NEWFEAT 的 PROVISIONAL 待审详情；源 CTA 为处理退货。契约正式商家详情 30:1757/1818，完整批准/拒绝选择画面签核 UNKNOWN。|
|desktop-merchant-reason|1468:86170|1791:53659|K4 源是登记 RMA。拒绝申请需非空原因（0210）；该表只是布局依据。拒绝理由空表已签根 UNKNOWN。|
|mobile-merchant-reason|1468:86221|1791:53710|K4 源是登记 RMA。拒绝申请需非空原因（0210）；该表只是布局依据。拒绝理由空表已签根 UNKNOWN。|
|desktop-merchant-filled|1468:86170|1791:53739|K4 登记 RMA 表作布局，已填写拒绝原因是新增状态。该状态已签根 UNKNOWN。|
|mobile-merchant-filled|1468:86221|1791:53790|K4 登记 RMA 表作布局，已填写拒绝原因是新增状态。该状态已签根 UNKNOWN。|
|desktop-merchant-approved|1468:107474|1791:53819|PROVISIONAL 待审详情改成批准回执。批准后的申请＋关联 RMA 依 0210；相同状态替代签核 UNKNOWN。|
|mobile-merchant-approved|1468:107559|1791:53874|PROVISIONAL 待审详情改成批准回执。批准后的申请＋关联 RMA 依 0210；相同状态替代签核 UNKNOWN。|
|desktop-merchant-rejected|1468:107474|1791:53894|PROVISIONAL 待审详情改成拒绝回执。历史商家详情无相同裁决终态；替代签核 UNKNOWN。|
|mobile-merchant-rejected|1468:107559|1791:53936|PROVISIONAL 待审详情改成拒绝回执。历史商家详情无相同裁决终态；替代签核 UNKNOWN。|
|desktop-merchant-list|1468:86106|1791:53956|原对象是退货单。申请队列应对照契约及 C5 的 30:1662/1731 售后申请列表，按当前 requested 读模型。该对是更匹配的正式基线，但单一状态的现代布局签核 UNKNOWN。|
|mobile-merchant-list|1468:86149|1791:54001|原对象是退货单。申请队列应对照契约及 C5 的 30:1662/1731 售后申请列表，按当前 requested 读模型。该对是更匹配的正式基线，但单一状态的现代布局签核 UNKNOWN。|
|desktop-buyer-list|1468:99605|1791:54024|NEWFEAT 的 PROVISIONAL 买家申请列表，契约支持 GET list；未取得其他已签同场景根。|
|mobile-buyer-list|1468:99653|1791:54038|NEWFEAT 的 PROVISIONAL 买家申请列表，契约支持 GET list；未取得其他已签同场景根。|

### 空商品列表与空白编辑器是已有导航目标

桌面 `1253:16665 → 858:5505`，手机 `1253:16710 → 858:5506`；实际空列表动作和两个目标的导出见 `figops/refs/`。这四个 ID 不计入 32 个新根。10 月 30 日 `output/ui-parity/FIGMA-INVENTORY.md` §4/Appendix A 将 858 列入早期 catalog、1253 列入早期 states-v1，说明需 owner 判定。未查到对这两条「空店 → 普通新增」链的逐项废止或另一个签核 ID。不能因 1429 较晚便把 `1429:64000/64049` 已有商品编辑错接到空白 NEW；`1429:33613/33693` 是可跳过的开店首件录入；`1469:141881` 是复制已有商品所得的新草稿。它们的入口、数据和权限语义不同。历史清理曾明确保留 858 的新增规格入口；同一文件中的组件级导航与页面退役各需自己的记录。

## 商品与筛选的后续修订

两端各三件商品的载入态同时取两种来源。商品身份、售价、上下架及可售量从当端列表对应行读取，编辑布局取 `1429:64000 / 1429:64049`。同一模板曾把三件商品都显示为提袋，模板中的原售价、描述及其他 SKU 的资料不得覆盖行数据。另有两端各四个筛选或选中状态的源与目标如下。

| 场景 | 桌面原稿 | 手机原稿 |
|---|---|---|
| 在售 | [1429:63218](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1429-63218) | [1429:63287](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1429-63287) |
| 草稿 | [1429:63348](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1429-63348) | [1429:63405](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1429-63405) |
| 已归档 | [1429:63453](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1429-63453) | [1429:63499](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1429-63499) |
| 当前页已选 | [1429:63540](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1429-63540) | [1429:63620](https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X/Untitled?node-id=1429-63620) |
