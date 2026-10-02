# R5 计划：对标 SHOPLINE 与 1688，按商家真实用法改造（2026-10-02）

依据：
- `docs/discovery/2026-10-02-shopline-admin-study.md`
- `output/r5-research/admin/REPORT.md`（后台对标，P1 12 项 / P2 16 项 / P3 4 项）
- `output/r5-research/storefront/REPORT.md`（前台对标 SHOPLINE 与 1688，P1 6 项 / P2 6 项 / P3 5 项）
- owner 决策：试点先升级到 R4，再补多专页和宅配货到付款；店铺地址自动分配（DNS 已配好）。

编排遵循 PROCESS.md §3：
- 后端实现交给 DeepSeek V4-Pro；**`apps/` 下的任何界面改动只交给 Codex**（owner 2026-10-02：DeepSeek 不做视觉和 UI；UI 任务交给 Codex）；K2.8/K3 做视觉 QA。
- 独立测试和跨家族审查交给 K3。
- 视觉 QA 交给 K2.8 / K3。
- 合并、合同裁决、钱路和安全终审由 Claude 负责。
- 同时最多 2 个写入任务，每个单元有独立 worktree 和分支。

## 波次 1：业务能力缺口（后端为主）

| 单元 | 迁移 | 实现 | 独立测试 | 终审 |
|---|---|---|---|---|
| store-domains：英文店铺 ID 自动生成子域，商家可自助绑定自有域名 | 0106 | DeepSeek V4-Pro | K3 | Claude（域名、TLS、跨店隔离） |
| home-cod：宅配货到付款，物流商加黑猫和新竹 | 0107 | DeepSeek V4-Pro | K3 | Claude（钱路） |
| meta-multi-page：一个店最多连 10 个专页 | 0108 | DeepSeek V4-Pro | K3 | Claude（Page token 保管） |

## 波次 2：后台贴合真实用法（来自 admin 报告前 10 项）

| 编号 | 改动 | 覆盖项 | 执行者 |
|---|---|---|---|
| A1 | 导航修复：可滚动、删掉写死的渠道状态（分组由 W0 外壳做） | M07、M08 | 止血包（Claude Sonnet，已在做） |
| A2 | 全站时间按店铺时区显示；金额按「元」输入，台币显示整数 | M06、M11 | 止血包（Claude Sonnet），由 Claude 审钱的展示 |
| A3 | 订单列表 v2：可读订单号、搜索、快捷分类、筛选、来源列、中文状态词 | M04、M20 | Go 查询部分 DeepSeek，UI 部分 Codex |
| A4 | 一屏上架商品（多图、价格、数量、关键字一次保存） | M09、M18 | 保存命令（Go/SQL）DeepSeek；页面 Codex（按视觉稿 03） |
| A5 | 直播流程：场次显示订单数和金额、一键复制上一场、从专页的直播中列表点选绑定 | M10、M14、M16、M17 | 后端 DeepSeek；界面 Codex（按视觉稿 05/06） |
| A6 | SKU 可设不追踪库存（∞）（**owner 2026-10-02 已批准**；不追踪的 SKU 不锁货，只受单次上限约束；追踪的 SKU 仍严格不超卖） | M01 | DeepSeek，K3 测并发 |
| A7 | 留言匹配新增受限的「包含 KW+N」模式 KEYWORD_QTY_CONTAINS（**owner 2026-10-02 已批准**；按场次可选，默认仍为 EXACT；否定词、问句、多个关键字、粘连写法一律不命中） | M02 | DeepSeek，K3 做反例对抗测试 |
| A8 | 订单批量操作（超商建单打印、回填运单号、批量标记已收款） | M05 | 需先撤销 M-7 裁决；放在波次 3 |

## 波次 3：买家前台（来自 storefront 报告前 10 项）

| 编号 | 改动 | 执行者 |
|---|---|---|
| S1 | 图片按尺寸输出（先试 next/image 加 sharp） | Codex（unit storefront-r5，与 S2–S5、S7 同批） |
| S2 | 商品页显示配送、付款、退货说明 | Codex（storefront-r5） |
| S3 | 相关商品 | Codex（storefront-r5） |
| S4 | 列表页分类 chips | Codex（storefront-r5） |
| S5 | 防诈骗宣导页及转账提示 | Codex（storefront-r5） |
| S6 | 直播入口（要改设计合同 §B） | DeepSeek 负责 schema，Codex 负责 UI |
| S7–S10 | 图库放大、规格表、标签、首页分类区与热销排序 | 视情况安排 |

## 波次 4：消息与运营
结账提醒和无库存回复（M15）、下单黑名单（M27）、统一消息工作台（M26）、留言标签打印（M29）。

## 每个单元的验收门禁
- 先写 brief 并冻结合同；实现测试先红后绿。
- 独立测试作者不能是实现者。
- 对应的浏览器模式在生产配置原样下运行。
- 合入后跑 release-gate --strict 子集；每个波次结束跑一次全量门禁。

## 已知风险
- 商家商品使用奢侈品牌名，标题里有「特級配全套包裝」等字样。开启 Meta 商品目录同步或广告之前，owner 需要核对 Meta 关于仿冒品和商标的商务政策。
