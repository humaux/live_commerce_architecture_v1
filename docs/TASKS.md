# 开发任务依赖与写路径

**本表是开发任务 DAG 和默认角色，不是实时完成状态。** 当前切片状态见
`contracts/tasks.json` 与 `docs/implementation/` 的实际验收记录；已落地的基础、
商品库存和身份切片不代表整个任务或全局门禁完成。实际 agent/worktree 另记在交付证据中。

T00由integrator记录只读explorer的结果；T01只冻结首个切片及公共不变量，不要求第一天建齐所有表。
任务gate表示贡献范围，不要求每个局部任务先通过整个全局门禁。真实外部资格另列，不得以mock结果代替。

|ID|工作包|依赖|角色|需求|门禁|
|---|---|---|---|---|---|
|T00|环境、商业范围、版本及Codex能力预检|无|integrator|R15, R16, R21, R22|G13, G14|
|T01|冻结OpenAPI/事件/领域schema与反例|T00|integrator|R01, R13, R16|G01, G02, G04|
|T02|Go骨架、PG事务、日志、健康检查、CI|T01|commerce_worker|R13, R15, R21|G04, G12, G14|
|T03|身份、店铺/域名服务、RLS授权与审计|T02|commerce_worker|R01, R14, R02|G01, G02, G11|
|T04|商品SKU/价格/库存账本/预留|T03|commerce_worker|R03, R06, R07|G03|
|T05|店铺模板、域名、SSR和购物车界面|T03|ui_worker|R02|G02, G11|
|T06|集成入口、binding、capability、操作台账与用途策略接口|T03|integration_worker|R13, R22, R14|G02, G04, G14|
|T07|Meta能力探针、评论/会话与合法发送适配|T06|integration_worker|R04, R05, R22|G06, G07, G14|
|T08|LiveKit适配、节目和目的地编排|T06|integration_worker|R04, R19|G06|
|T09|工作室、评论台和人工接管界面|T05, T07, T08|ui_worker|R04, R05|G06, G07|
|T10|关键词grammar/报价/声明/合并购物车|T04, T07|commerce_worker|R06|G03, G07|
|T11|结账、PSP、退款与支付对账|T04, T06|commerce_worker|R07|G03, G05|
|T12|核心成交闭环独立集成验收|T09, T10, T11|test_worker|R02, R04, R05, R06, R07, R22|G01, G03, G04, G05, G06, G07|
|T13|物流适配、仓库包裹、面单与RMA|T11|integration_worker|R08, R20|G08|
|T14|客户关系图、同意、删除与来源|T10, T11|commerce_worker|R09, R11, R14|G09|
|T15|广告账户/草稿/预算审批/报表|T06, T14|integration_worker|R10|G10|
|T16|CAPI/目录映射/受众导出|T14, T15|integration_worker|R09, R11|G09, G10|
|T17|套餐用量、欠费、财务视图|T11, T13|commerce_worker|R12, R18|G05, G08, G12|
|T18|物流/客户/广告/账单运营界面|T13, T14, T15, T17|ui_worker|R08, R09, R10, R12, R18|G08, G09, G10, G11|
|T19|语言基准与优化消融|T02, T04, T06|test_worker|R15, R17|G12, G15|
|T20|安全/隔离/故障注入与恢复|T12, T13, T16, T17|test_worker|R01, R13, R14|G01, G02, G04, G05, G08, G09, G12|
|T21|独立架构与安全审查|T18, T19, T20|security_reviewer|R01, R14, R16, R21|G01, G13, G14|
|T22|发布准入、真实商户试点与回滚演练|T21|integrator|R21, R22|G01, G02, G03, G04, G05, G06, G07, G08, G09, G10, G11, G12, G13, G14, G15|

## 写路径归属

主Agent发任务时还需在该清单内收窄到实际文件，记录worktree、base SHA和契约版本。迁移、锁文件、共享contract始终由integrator最终合并。

### T00 环境、商业范围、版本及Codex能力预检

`docs/discovery/**`, `contracts/versions.candidate.json`

### T01 冻结OpenAPI/事件/领域schema与反例

`contracts/**`, `migrations/**`, `go.mod`, `go.sum`, `package.json`, `pnpm-lock.yaml`

### T02 Go骨架、PG事务、日志、健康检查、CI

`cmd/**`, `internal/platform/**`, `tests/platform/**`, `.github/workflows/**`, `scripts/dev/**`, `infra/local/**`, `internal/ui_events/**`, `tests/ui_events/**`

### T03 身份、店铺/域名服务、RLS授权与审计

`internal/identity/**`, `internal/buyer/**`, `tests/foundation/buyer_capability_test.go`, `internal/tenancy/**`, `tests/tenancy/**`, `internal/stores/**`, `internal/domains/**`, `internal/audit/**`, `tests/domains/**`

### T04 商品SKU/价格/库存账本/预留

`internal/catalog/**`, `internal/inventory/**`, `tests/inventory/**`, `internal/pricing/**`, `tests/pricing/**`

### T05 店铺模板、域名、SSR和购物车界面

`apps/storefront/**`, `packages/ui/**`, `tests/storefront/**`

### T06 集成入口、binding、capability、操作台账与用途策略接口

`internal/integrations/core/**`, `tests/integrations/core/**`, `internal/privacy/policy/**`, `tests/privacy/policy/**`

### T07 Meta能力探针、评论/会话与合法发送适配

`internal/integrations/meta/**`, `tests/integrations/meta/**`, `docs/discovery/meta/**`, `internal/inbox/**`, `tests/inbox/**`

### T08 LiveKit适配、节目和目的地编排

`internal/live/**`, `internal/integrations/livekit/**`, `tests/live/**`, `internal/mediaassets/**`

### T09 工作室、评论台和人工接管界面

`apps/admin/studio/**`, `apps/admin/inbox/**`, `tests/studio/**`

### T10 关键词grammar/报价/声明/合并购物车

`internal/claims/**`, `internal/cart/**`, `tests/claims/**`

### T11 结账、PSP、退款与支付对账

`internal/checkout/**`, `internal/payments/**`, `tests/payments/**`, `internal/orders/**`, `internal/integrations/psp/**`, `tests/orders/**`

### T12 核心成交闭环独立集成验收

`tests/e2e/commerce/**`, `evidence/commerce/**`

### T13 物流适配、仓库包裹、面单与RMA

`internal/fulfillment/**`, `internal/integrations/shipping/**`, `tests/fulfillment/**`

### T14 客户关系图、同意、删除与来源

`internal/customers/**`, `internal/privacy/**`, `tests/privacy/**`

### T15 广告账户/草稿/预算审批/报表

`internal/ads/**`, `internal/integrations/meta_ads/**`, `tests/ads/**`

### T16 CAPI/目录映射/受众导出

`internal/attribution/**`, `internal/audiences/**`, `tests/attribution/**`, `internal/integrations/meta_catalog/**`

### T17 套餐用量、欠费、财务视图

`internal/billing/**`, `internal/reporting/**`, `tests/billing/**`, `internal/notifications/**`

### T18 物流/客户/广告/账单运营界面

`apps/admin/operations/**`, `tests/admin/**`

### T19 语言基准与优化消融

`bench/**`, `experiments/product/**`, `evidence/performance/**`

### T20 安全/隔离/故障注入与恢复

`tests/acceptance/**`, `tests/chaos/**`, `evidence/acceptance/**`

### T21 独立架构与安全审查

只读，不允许代码写入；审查结果交主Agent归档。

### T22 发布准入、真实商户试点与回滚演练

`deploy/**`, `docs/runbooks/**`, `evidence/release/**`
