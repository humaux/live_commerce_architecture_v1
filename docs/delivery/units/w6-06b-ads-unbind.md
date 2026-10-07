# Unit W6-06B — Meta ad account unbind + catalog feed URL entry (backend, small)

状态：DRAFT（起草人 Aliyun Qwen 子代理 2026-10-07；owner 决定 2026-10-07「广告解绑：可以做」）。
Base `6277731e`（trunk），worktree `.worktrees/w6-06b-ads-unbind`，branch `unit/w6-06b-ads-unbind`。
迁移号 **0160**（0157–0159 已被并行单元占用）。无 brief 文件在先：本文件即 brief。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/meta-ads-v1.md` §2、§4.1、§5.3、§7、
Lane close amendments（R2-ADS-PAUSE-1）与本单元 Amendment；`contracts/external-operation-v1.md` 状态规则 3–5；
`contracts/catalog-inventory-v1.md`「Meta catalog feed」。UI 不在本单元（Codex 后续）。

## 目标（owner 流程）
1. 商家可以**解绑**（disconnect）店铺已连接的 Meta 广告账户：解除绑定后不能再规划新的广告操作，历史
   （草稿、远端对象、洞察、CAPI 事实、审计、连接行）全部保留；重新绑定走既有 connect 流程。
2. 商家可以看到店铺的 **catalog feed URL**（`GET /v1/buyer/feeds/meta.csv` 的绝对地址，按已验证店面域名），
   用于粘贴进 Meta Commerce Manager 的 scheduled data feed。

## 关键事实（6277731e）
- 绑定 = `integration.bindings`（0008:13，`enabled`、`semantic_version`）；广告账户 provider `meta_ads`
  （asset = 数字 ad account id），AD2。绑定时 `integration.register_meta_ads_token`（0074:687）把 HPKE 密封
  token 复制到 `integration.meta_page_credentials`/`meta_page_heads`（0064，每 binding 一套版本）。
- 既有断开先例：`integration.meta_connect_disconnect`（0108:188，Page 断开）——同一事务销毁 head+versions、
  删连接行；「the caller then disables the bindings」（Go 侧 `core.SetBindingEnabled`，commerce_runtime
  UPDATE(enabled,semantic_version,updated_at)，0008:115）。
- 冻结触发器 `bindings_ads_disable_guard`（0074:498 `ads.guard_binding_disable`）：任何草稿仍按 AD6 计数时
  拒绝 disable（PT409 `binding_in_use`）＝ R2-ADS-PAUSE-1「先暂停、后断开」；Lane close amendment 明言
  该拒绝文案随**第一个暴露 disconnect 的路由**（= 本单元）上线。
- 运行中操作：external-operation-v1 规则 3–5 —— claim 先锁 binding 行；disabled binding 上仅从未派发的
  READY 变终态 STALE_BINDING；DISPATCHING/UNKNOWN/ACKNOWLEDGED 永不取消（「ACKNOWLEDGED is not success」）。
- feed：公开路由 `GET /v1/buyer/feeds/meta.csv`（`internal/attribution/feed.go`，无签名无 token，§7），由
  storefront 应用按已验证店面 host 代理；域名 = `control.storefront_domains`（0020:13，state ACTIVE），
  每店开店即有 platform 子域（0106 `ensure_store_platform_domain`）。`commerce_ads_writer` 已有
  SELECT(origin,state) 域名 + SELECT(published)  publication 的 GAP-2 授权（0074:343-346），无需新授权。
- Go：`internal/ads/service.go`（Bind/bindOne：只复用 enabled 的同 asset binding）、`internal/httpapi/ads.go`
  （路由 + `adsScope`）、`internal/ads/errors.go`（frozen code → status 表；ADnnn SQLSTATE 约定）。
- 权限词汇：`ads:manage`（解绑）/`ads:read`（feed URL；feed 本身公开无秘密，故不触发 brief 的
  「有 token 只给 ads:manage」条款）。

## 范围
1. **合同先行**：`contracts/meta-ads-v1.md` 追加 Amendment W6-06B（解绑语义、拒绝码、路由、definer 签名、
   non-goals、feed URL 语义）。
2. **迁移 `0160_ads_unbind.sql`**（仅两个函数，无表、无授权/策略变化，§4.4 privilege delta 不动）：
   - `integration.meta_ads_unbind(p_hash bytea,p_store uuid,p_ad_account text) RETURNS jsonb`
     OWNER `commerce_integration_writer`，EXECUTE `commerce_runtime`：ads:manage（resolve_access + GUC 等值）；
     FOR UPDATE 锁该 asset 的全部 enabled `meta_ads` binding（与 claim_operation 串行化）；任一 binding 上有
     DISPATCHING/UNKNOWN/ACKNOWLEDGED 操作 → 返回 `{refused:'operations_in_flight',operations:[…≤50],operations_total}`；
     否则删 head+versions（先 head 后版本，同 0108 路径），返回 `{unbound:true,bindings:[{binding_id,binding_version,credentials_destroyed}]}`；
     无 enabled binding → `{unbound:false,already_unbound:true}`（幂等 no-op）。
   - `ads.catalog_feed_url(p_hash bytea,p_store uuid) RETURNS jsonb` OWNER `commerce_ads_writer`，
     EXECUTE `commerce_runtime`：ads:read；返回店铺 published 且 ACTIVE 域名的 `{feed_url, domains:[{origin,feed_url}], path}`；
     无 → `feed_url:null, domains:[]`。
3. **Go**：`internal/ads/unbind.go`（新）`Service.Unbind`（command.Run `ads.meta.unbind`，definer → 拒绝转
   `Refusal{409,operations_in_flight,Details}`；成功则按返回的 binding_version CAS disable（0008:115 授权，
   触发器可能拒绝 PT409 `binding_in_use`）→ 审计 `ads.account_unbound`）与 `Service.CatalogFeed`（queryJSON）；
   `internal/ads/errors.go` 加 frozen code `operations_in_flight`/`binding_in_use`（409）、`Refusal.Details` +
   `ErrorDetails()`、PT409 且 message=`binding_in_use` 的定向映射；`internal/httpapi/ads.go` 加
   `POST {base}/meta/unbind`（keyed，body `{ad_account_id}`）与 `GET {base}/catalog-feed`（无 query），
   `adsScope` 改用 `respondErrorDetails`（其余路由无 Details，不受影响）。
4. **测试 red→green**（red 存 `output/w6-06b-ads-unbind/red.log`）：
   - REAL_PG（`internal/ads/unbind_pg_test.go`，复用 pg_flow_test.go 夹具模式，独立 store）：
     解绑后 binding disabled + token 行销毁 + 连接/草稿/审计历史保留；新草稿/审批被 `binding_disabled` 拒绝；
     in-flight 操作 → 409 `operations_in_flight` 且列表非空、凭据未销毁；同 key 重放与再次解绑（no-op）幂等；
     跨店 no-op（不解他店绑定）；无 `ads:manage` 拒绝；feed URL 只含本店 ACTIVE 域名、跨店不可见。
   - DB-free（`internal/httpapi/ads_test.go` 追加 transport 用例）：unbind 需 key/body 校验、catalog-feed 拒
     query/key、405 fallback。
   - 钉子：`tests/foundation/r2_integration_upgrade_test.go` 迁移数 83→84（诚实更新，不放宽）。
5. 收尾：`output/w6-06b-ads-unbind/DELIVERY.md`、小步 commit；不 merge/push。

## Non-goals
- 不动 `meta_dataset` 绑定与 CAPI（CAPI 开关仍是 `PUT capi {enabled:false}`）；不解绑 Page/IG（0108 已有）。
- 不调用 Meta（本地销毁密封 token ≠ 在 Meta 侧撤销授权；商家可自行在 Facebook 设置撤销——文案归 UI 单元）。
- 不做 UI（`apps/` 禁改）；不改 OpenAPI/共享 schema（integrator 合并时补 `contracts/ads-openapi.json`）。
- 不新增授权/策略/角色；不触碰 §4.4 privilege delta、MA02 钉子（两个新函数均复用既有授权）；T06 `integration.*` 函数钉子 89→90 与 R2 迁移数 83→84 须诚实更新（修复轮已更新）。
- 不改冻结的 `bindings_ads_disable_guard` 行为；卡死草稿仍走 deploy.md §6.4 运维路径。

## 完成标准
`bash scripts/dev/test-focused.sh 'TestAdsUnbind|TestAdsCatalogFeed' ./internal/ads` 绿；
`go test ./internal/httpapi -run 'TestAdsTransportRules'` 绿；`bash scripts/dev/check-gates.sh` 过；
`go vet ./... && gofmt -l internal cmd` 干净。重活（foundation 全套）列 DELIVERY「CI gates」交 integrator。
