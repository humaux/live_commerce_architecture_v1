# Unit live-a5-session-flow — 场次成果（订单数/金额）、复制上一场、从专页直播中列表点选绑定（R5 A5）

状态：DRAFT（起草人 2026-10-05，待 integrator 审核/冻结）。Base `r3/integration` `4863db0`。
迁移号建议 **0114**（当前最高 `0113_ads_attribution.sql`；**待 integrator 确认**）。
覆盖 R5-PLAN 波次 2 A5（M10、M14、M16、M17）。拆三段：A5-1 成果读模型、A5-2 复制场次、A5-3 专页直播中视频选择器。
后端一个 worktree `.worktrees/live-a5-session-flow`（branch `unit/live-a5-session-flow`）；UI 另开 Codex 任务
（worktree `.worktrees/live-a5-session-flow-ui`，在后端 API 冻结并合入后开始）。
先读：`docs/delivery/PROCESS.md`、`AGENTS.md`、本文件；合同只读下文点名的章节。

## 目标与 owner 原话

- R5-PLAN A5：「直播流程：场次显示订单数和金额、一键复制上一场、从专页的直播中列表点选绑定」。
- 依据：`output/live-console-research/current-admin-live-map.md` §2 #1/#9、§4 #4、「其他缺口」表（场次列表无订单数/金额；
  只能复制关键字商品、不复制标题/设置；没有「列出专页正在直播的视频 → 一点绑定」）；
  `output/live-console-research/competitors.md` A2（SHOPLINE「複製直播」）、A12（只列「直播中」「已排程」）、§6 第 1 条
  （试点商家每天复制上一场，含关键字、直播价、设置）。

## 关键事实（起草时读代码核实）

1. 场次列表 `GET /v1/admin/stores/{store_id}/live-sessions` = `live.ListDrafts`（`internal/live/studio.go:190`），
   返回 `Draft`（`internal/live/draft.go:37`），studio-v1 FROZEN，**不改它的响应形状**。
2. **orders v2 的 `session_id` 只来自 `claims.live_price_uses`**（`migrations/0110_merchant_orders_v2.sql`
   `claims.order_live_sources`；`contracts/merchant-orders-v2.md`「durable live-price-use -> bundle -> session chain」）。
   没用直播价的认领订单**不归属任何场次**。0113 已新增不依赖直播价的来源表 `claims.order_origins`
   （`claims.capture_order_origins`，checkout Begin 同事务写入），但只有广告复盘在用（`claims.attribution_session_orders`）。
   直接复用 orders v2 的 `session_id` 会少算订单 → 本单元把两条来源统一（见「数据/迁移」）。
3. 钱的口径以 0107 版 `identity.read_finance_summary`（`migrations/0107_home_cod.sql` E 段）为准：
   card = `payments.facts` CAPTURED − `payments.refund_facts` SUCCEEDED（且其后无 FAILED/CANCELED，`refund_id,kind` 为主键，
   每笔退款只减一次）；pay_at_pickup = `collection_state='COLLECTED'` 的 `total_minor`，环境取最新 `fulfillment.cvs_shipments`
   否则 LIVE；cash_on_delivery = COLLECTED 的 `total_minor + cod_surcharge_minor`（LIVE）；bank_transfer =
   `checkout.bank_transfers.state='CONFIRMED'` 的 `confirmed_amount_minor`（LIVE）。线下退款会让订单离开这些条件。
   **不要**照抄 `orders.attribution_metrics`（0113）：它的 refunded 没排除后来 FAILED/CANCELED 的退款，也不分环境。
4. 「从其他场次复制」已存在：`POST …/claims/offer-import` `source=session`（`claims.ImportOffers`，
   `internal/claims/keyword_library.go:203`，`importCandidates`/`planImport`），只复制 **active** offers 的
   keyword/SKU/上限；合同「Live tools (R4)」第 2 条规定 **`live_price_minor` 从不复制**。
5. API 进程**不能打开 Page token**（`internal/metaconnect/doc.go`：只持 HPKE 公钥；`TestMetaConnectAPIHoldsNoPagePrivateKey`
   守护）。持私钥的是 `cmd/claims-worker`（`deploy/compose.yml` claims-worker 段）。现成模板：广告复盘的
   `/{page}/live_videos?fields=id,post_id` 读取，走 operation ledger + River `external_operation_v1` + 路由
   `(facebook, meta.live_insights, service)`（`internal/integrations/metareply/audience.go` `AudienceRoutes`、
   `pageSecretLoader`（`routes.go:221`）、SQL `integration.plan_meta_audience/check_meta_audience/
   load_meta_audience_token/finish_meta_audience`，0113）。`/{page}/live_videos` 可读已由 2026-10-03 只读探测验证
   （`docs/delivery/units/ads-attribution.md`「Verified 2026-10-03」），但 `status` 字段取值、`broadcast_status` 过滤、
   以及 **live video 的 `post_id` 与 feed webhook 送来的 `post_id` 是否同形（U1）均未验证**。
6. 绑定写入已存在：`PUT …/live-sessions/{session_id}/claim-source`（`claims.PutClaimSource`），`input` 接受
   `<page>_<post>`，`ref.Page` 精确选中该专页的 binding（多专页 0108 下也唯一）。**选择器不新增写路径**。

## 范围

### A5-1 场次成果读模型（M14）
- 新读 `GET /v1/admin/stores/{store_id}/live-sessions/results?session_id=<uuid>&session_id=…`（1..50 个，重复/非法 → 422；
  任一 id 不属于本店 → 404）。权限：Go `scoped(pool,"live:read",…)`；SQL definer 另要求 `orders:read`
  （无 `orders:read` → 403，UI 隐藏金额列）。响应：
  `{"as_of":ts,"items":[{"session_id","orders","paid_orders","multi_session_orders",
  "money":[{"currency","order_minor","paid_minor","sandbox_paid_minor"}]}]}`。
  - `orders` = 归属该场次且 `commercial_state<>'CANCELLED'` 的订单数；`order_minor` = 这些订单
    `total_minor+coalesce(cod_surcharge_minor,0)`（COD 商家当晚「已收」接近 0，必须同时给下单金额）。
  - `paid_minor` / `sandbox_paid_minor` = 事实 3 的逐单口径，按环境拆开，**永不相加**（I05）；不按订单状态过滤（与财务一致）。
    `paid_orders` = 至少一项入账 > 0 的订单数。
  - 按 `currency` 分组，不换汇。一单含多场次商品 → 在每个场次都计（与 orders v2「Multiple sessions per order are retained」一致），
    `multi_session_orders` 给 UI 加脚注。
- 不改 `GET live-sessions`（studio-v1 FROZEN）；UI 拿到一页 20 场后再调一次 results。
### A5-2 复制场次（M16/M17）
- `POST /v1/admin/stores/{store_id}/live-sessions/{session_id}/copy`，`Idempotency-Key` 必填，body 精确
  `{"title","scheduled_at","expected_version"}`（`scheduled_at` 可 null，同 `DraftInput`）；`live:manage`。
  `command.Run` receipt `live.session.copy`、audit `live.session.copied`，一个事务：
  1. 锁源场次 `live.sessions … FOR SHARE`，`version != expected_version` → `command.ErrConflict`（409，与 `UpdateDraft` 同 CAS 风格）；
  2. 新建 session + program（同 `CreateDraft` 的两条 INSERT；`aspect_ratio` 取源 program；title/scheduled_at 取 body，
     走 `canonicalInput`）；
  3. 新场次 `live.claim_windows` 一行：`CLOSED`、generation 0、`match_mode` = 源窗口的模式（源无窗口 → `EXACT`）——
     用 `claims.writeWindow` 的建行路径，不手写 SQL；
  4. offers：复用 `importCandidates(source=session)`（源场次 **active** offers）+ `planImport`，逐条创建：
     keyword、SKU、`max_quantity_per_claim`、**`live_price_minor`（新：复制）**、active=true；冲突照旧返回
     `ImportConflict`（新场次上只可能出现 `sku_unavailable`）。
  - 响应 `{"session":Draft,"window":Window,"created":[Offer],"conflicts":[ImportConflict],"source_version":int}`。
  - **不复制**：认领（bundles/lines/events）、`claims.links`、窗口状态/轮次/intervals、`live.claim_sources`（留言来源绑定的是某一支视频）、
    `live_price_uses`、关键字库（店级，不需要复制）。
  - 标题规则在 UI：源标题含 `M/D` 或 `MM/DD` 日期 → 替换成目标日期（台北时区，取 scheduled_at，否则今天）；否则原样。服务器只校验。
  - 留言来源的偏好（私讯开关、回复语言）由 UI 读源场次 `GET claim-source` 预填，不落库。
### A5-3 专页「直播中」视频选择器（M10，MOCK 直到 LIVE 探测）
- `POST /v1/admin/stores/{store_id}/live-sessions/{session_id}/page-live-videos/read`，`Idempotency-Key` 必填，body 精确
  `{"binding_id":uuid}`；→ `{"operation_id","state"}`。同一 binding 已有 PENDING 读取 → 返回那一条，不建新 job（I23）。
- `GET /v1/admin/stores/{store_id}/live-sessions/{session_id}/page-live-videos?binding_id=<uuid>` →
  `{"binding_id","state":"none|pending|succeeded|failed|unknown","code","requested_at","fetched_at",
  "items":[{"video_id","post_id","title","status","started_at"}]}`（≤25 条，`LIVE` 在前）。路径挂在任一场次下只为
  admin BFF 的 `live-sessions/` 私有作用域（与关键字库同理），快照按 `(store, binding)` 存。
- claims-worker 新路由 `(facebook, meta.live_videos, service)`：`GET /{page-id}/live_videos?fields=id,post_id,title,status,creation_time&limit=25`，
  单页不翻页；`metaoauth.Graph`（host allowlist、不跟随跳转、token 只在 header）；15 s 超时；UNKNOWN 不自动重放（I06）。
  `post_id` 归一为 `<page_id>_<post>`（裸 id 前缀补 asset；带前缀但前缀≠asset → 整个结果 `bad_result`）。
  `title` 视为不可信文本：≤255 rune、去控制字符，UI 纯文本渲染。
- 选中 → UI 用现有 `PUT claim-source`（`input`=归一后的 `post_id`，`platform:"facebook"`）。FB 来源继续显示「未验證」直到 U1。
- **Meta 权限**：Page access token + `pages_read_engagement`（meta-connect 已强制要求，`contracts/meta-claims-intake-v1.md`
  「Amendment: Merchant connect」第 2 条；不新增 App Review 权限）。`pageSecretLoader(..., []string{"pages_read_engagement"}, …)`。
  应用仍在开发模式：LIVE 只能由 owner 用 app 角色账号探测。
- 不做 Instagram（IG 直播列表需另一个端点与权限，且只在直播中可见）→ NOT_RUN。

## Out of scope
场次生命周期（开播/结束/归档/删除）、实时推送、逐品成效表、复盘页搬家、IG 直播列表、店级「默认匹配模式」设置（复制已带模式）。

## 合同修改（实现前由 integrator 冻结）
1. `contracts/studio-v1.md` 末尾新增「Amendment: session results + copy (R5 A5)」：上面 A5-1/A5-2 的路由、body、权限、错误码
   （`invalid_request` 422、`not_found` 404、`conflict` 409、`forbidden` 403），并写明 `GET live-sessions` 形状不变。
2. `contracts/live-keyword-claims-v1.md`「Amendment "Live tools (R4)"」第 2 条后追加：「`POST …/copy` 是唯一复制
   `live_price_minor` 的路径（owner A5：复制上一场含直播价）；`offer-import source=session` 仍不复制」。**需 integrator 裁决**（见风险 R1）。
3. `contracts/merchant-orders-v2.md`「Response」中 "Sessions are only those proved by … live-price-use -> bundle -> session chain"
   改为「live-price-use 链 ∪ `claims.order_origins`（0113）」，并注明 0113 之前、无直播价的订单仍无归属。
4. `contracts/meta-claims-intake-v1.md` §2 末尾追加「Page live-video picker (A5)」：读取路由、operation 形状、权限、
   `post_id` 归一规则、U1 未验证前 MOCK；§12 gates 加 LSF 行（下）。
5. OpenAPI：新文件 `contracts/live-session-flow-openapi.json`（仿 `claim-source-openapi.json`），4 个 path：
   `/v1/admin/stores/{store_id}/live-sessions/results`、`…/live-sessions/{session_id}/copy`、
   `…/live-sessions/{session_id}/page-live-videos`、`…/page-live-videos/read`。实现方起草，integrator 合并。

## 数据 / 迁移 `0114_live_session_flow.sql`（编号待 integrator 确认）
- `claims.session_orders(p_tenant uuid,p_store uuid,p_sessions uuid[]) RETURNS TABLE(session_id uuid,order_id uuid)`：
  `claims.order_origins` ∪（`claims.live_price_uses` ⋈ `claims.bundles`），DISTINCT；SECURITY DEFINER，owner
  `commerce_claims_writer`，`search_path=pg_catalog`，EXECUTE 仅 `commerce_auth`。
- `CREATE OR REPLACE claims.order_live_sources(...)`（0110）改为同一并集，保持签名/owner/ACL 不变 → orders 页筛选与场次数字一致。
- `identity.read_live_session_results(p_hash bytea,p_store uuid,p_sessions uuid[]) RETURNS jsonb`：owner `commerce_auth`，
  `identity.resolve_access(p_hash,p_store,'orders:read')` + `identity.principal_holds(…,ARRAY['live:read'])` + GUC 复核
  （照抄 `read_finance_summary` 的前导）；READ COMMITTED；`jit=off`、`plan_cache_mode=force_custom_plan`（0110/0113 的教训）。
  EXECUTE `commerce_runtime`。不新增任何表/列 GRANT 给登录角色（0110 ACL ruling）。
- `live.page_live_video_snapshots(tenant_id,store_id,binding_id PK…,operation_id,requested_at,fetched_at,state,code,items jsonb
  CHECK jsonb_typeof='array' AND jsonb_array_length<=25)`，FORCE RLS；只由 `integration.finish_meta_live_videos` 写；
  `commerce_runtime` 只读本店（RLS `S`）。
- `integration.plan_meta_live_videos / check_meta_live_videos / load_meta_live_videos_token / finish_meta_live_videos`：
  逐条仿 0113 的 `*_meta_audience` 四件套（plan 需 `live:manage`+`integration:execute`，binding enabled 且 provider=facebook；
  load 校验 scopes_attested ⊇ `pages_read_engagement`；finish 只接受 lease 正确的 completion）。
- 每个新对象 `COMMENT ON`（owner 包、允许角色、non-goals）。复制场次不需要新表：offer INSERT 走现有 `commerce_runtime` 权限。

## 不变量（`contracts/invariants.json`）
I01（store 只来自 auth；results 的 id 列表逐个校验属本店）、I02（copy/read 的幂等键）、I04（copy 一事务）、
I05（金额不跨币种/环境相加，只记可信支付事实）、I06/I07（Graph 读：不可变 operation 键、执行前复核 binding/授权、UNKNOWN 不盲重试）、
I09（结果无买家身份）、I11（Page token 只在 claims-worker 内存；title 不进日志）、I14（copy 的 version CAS）、
I18（空集合不算 PASS）、I23（每 binding 至多一个在途读取，≤25 条，超时有界）。

## 写入路径（唯一归属）
后端（DeepSeek V4-Pro）：`migrations/0114_live_session_flow.sql`；`internal/live/copy.go`（新）+ `internal/live/results.go`（新）；
`internal/claims/session_copy.go`（新：offer/窗口复制，调用 `importCandidates`/`planImport`/`writeWindow`，只给 `importCandidate`
加 `LivePriceMinor *int64` 字段——`keyword_library.go` 仅此一处改动）；`internal/integrations/metareply/live_videos.go`（新）；
`cmd/claims-worker/main.go`（只加一行路由注册）；`internal/httpapi/live_flow.go`（新，4 条路由 + methodless 回退）；
`internal/httpapi/studio.go`（只加一行 `registerLiveFlowRoutes` 调用）；`internal/httperror`（如需新码）；
`tests/foundation/live_session_flow_test.go`（作者冒烟）；`contracts/live-session-flow-openapi.json`（草稿）；
`docs/engineering/dependency-map.md`（重生成）。
UI（Codex，API 冻结后）：`apps/admin/components/Studio.tsx`（列表成果列 + 复制按钮/对话框）、
`apps/admin/components/LiveVideoPicker.tsx`（新）+ 在 `StudioClaims.tsx` 留言来源区**只加挂载一行**、
`apps/admin/lib/studio-client.ts`、`apps/admin/lib/live-flow-{client,model,copy}.ts`（新）、
`apps/admin/app/api/stores/[store]/[...resource]/route.ts`（allowlist：`live-sessions/results` GET + 重复 `session_id` query、
`live-sessions/{uuid}/copy` POST、`page-live-videos` GET/`page-live-videos/read` POST）、`apps/admin/lib/claims-request.ts`（同上 grammar）。
**与 live-a7-contains-match UI 共用 `StudioClaims.tsx`：两个 Codex 任务串行，不并行。**
独立测试（K3）：`tests/foundation/live_session_flow_gate_test.go`、`tests/admin/live-flow.spec.ts`、`tests/admin/live-flow-model.test.ts`。
integrator：`contracts/*`、`docs/delivery/GATES.md`、`scripts/dev/test-local.sh`（新模式）、`scripts/dev/release-gate.sh`。

## 执行角色
后端实现 DeepSeek V4-Pro（`PROVIDER=deepseek MODEL=deepseek-v4-pro`，不得改 `apps/`）；`apps/` 全部 Codex；
独立测试与跨家族对抗 K3（不得由实现者写）；钱口径（A5-1）与 Page token/权限（A5-3）的终审 Claude。

## 测试计划（先红后绿；每条 gate 记录一次红跑）
| ID | 层 | 内容 | 红跑方式 |
|---|---|---|---|
| LSF01 | REAL_PG | 无直播价的认领订单（只有 order_origins）计入 results 且出现在 orders v2 `session_id` 筛选 | 现 0110 版 `order_live_sources` 下该订单缺失 |
| LSF02 | REAL_PG | 钱口径：card 部分退款、退款后 FAILED、COD 未收/已收、自取 SANDBOX、转账确认/线下退款；LIVE 与 SANDBOX 分列；一单两场次各计一次且 `multi_session_orders=1` | 把 refund 条件换成 0113 写法 → 必须失败 |
| LSF03 | REAL_PG | 一致性：同一夹具，全部订单都属场次时，Σ场次 `paid_minor`(LIVE) = 同日期范围 `read_finance_summary` 的 net+pickup+cod+transfer(LIVE) | 改任一口径即红 |
| LSF04 | REAL_PG | 权限：无 `orders:read` 403；跨店 session id 404；51 个 id 422；撤权后重放仍拒 | — |
| LSF05 | REAL_PG | copy：标题/时间/比例、窗口 CLOSED+模式、offers 含 `live_price_minor` 与上限、无 bundles/links/sources；同键重放同结果、不同 body 同键 409（I02）；`expected_version` 过期 409；源 SKU 下架 → `sku_unavailable` | 先跑现 `offer-import` 证明直播价不复制 |
| LSF06 | REAL_PG + fake Graph | 选择器：plan→job→claims-worker dispatch（`httptest` Graph）→ 快照；`post_id` 裸/带前缀/前缀不符；token 缺 scope → FAILED；同 binding 在途只 1 个；Graph 超时 UNKNOWN 不重放；API 进程加载不到私钥（复用 MCI08 断言） | — |
| LSF07 | BROWSER（MOCK） | 新模式 `--browser-live-flow`：场次列表显示订单数/金额（三语，390/1586）、复制上一场 → 新场次关键字与直播价在、选择器点选 → claim-source 已绑定；点击台账 | — |

## 门禁命令（均已核实存在，新增者已注明）
- 静态：`go build ./... && go vet ./...`、`gofmt -l`、`bash scripts/dev/check-pkgdocs.sh`、`bash scripts/dev/depmap.sh --check`、
  `bash scripts/dev/check-gates.sh`、`bash scripts/dev/test-node.sh`、`python3 scripts/check_packet.py`、admin typecheck。
- PG：`bash scripts/dev/test-focused.sh '^TestLiveSessionFlow'`；回归 `'^(TestMerchantOrdersV2|TestAdsAttribution|TestLiveTools|TestStudioBackend|TestMetaClaimsMCI0[27]|TestMetaClaimsMCI08)'`。
- 迁移触及 GRANT/definer：`bash scripts/dev/release-gate.sh --strict --only G07`（PROCESS.md §2.6，单元验收与合并后各一次）。
- 浏览器：现有 `bash scripts/dev/test-local.sh --browser-studio-ui`、`--browser-live-claims`、`--browser-merchant-orders-ui` 回归；
  **新模式 `--browser-live-flow`**（`TestBrowserLiveFlow`，`tests/foundation/browser_live_flow_test.go`，复用 click-sweep 的
  `tcvNew` 种子栈；GATES.md 加行，check-gates 会强制）——由 integrator 加入 `test-local.sh`。

## 证据标签
A5-1 / A5-2：目标 REAL_PG + BROWSER（MOCK IdP）。A5-3：MOCK（fake Graph）；Graph 读 LIVE 只读探测与 U1 = NOT_RUN。
本文件本身 = DESIGN。证据写到 `/Volumes/data/live_commerce_architecture_v1/output/live-a5-session-flow/`。

## 风险
- R1 复制直播价与 R4 冻结条款相反：误把旧直播价带进新场（SKU 已调价）。缓解：UI 对复制来的直播价高亮「沿用上一場」，
  直播价高于 SKU 价仍只警告（R4 规则 3）；报价权威不变（`claims.live_prices`）。
- R2 并集改 `order_live_sources` 会改变 orders 页的场次筛选结果（变多）——这是修正，但要在合并说明里写清。
- R3 COD 为主的商家「已收」长期偏低，UI 必须同时显示下单金额，否则误读。
- R4 多场次订单在各场重复计数，场次合计 > 店总额；脚注 + `multi_session_orders`。
- R5 `live_videos` 的 `status` 取值与过滤未验证；拿不到 `status` 时全部列出、按 `creation_time` 倒序，不报错。
- R6 U1：选中视频的 `post_id` 若与 webhook 送来的形式不同，绑定后收不到留言——LIVE 前一律标「未驗證」。

## NOT_RUN / BLOCKED
- NOT_RUN：Graph `/{page}/live_videos` 带 `status` 的 LIVE 探测；U1（live video `post_id` 与 feed webhook `post_id` 对应）；
  IG 直播列表；真实 Page token 下的 `pages_read_engagement` 实测；WebKit 下的 `--browser-live-flow`；性能（>1万单/店的 results 耗时）。
- BLOCKED：合同修改 2（直播价复制）待 integrator/owner 裁决；`--browser-live-flow` 模式与 GATES 行待 integrator 加；
  UI 等后端 API 冻结；LIVE 探测等 owner（应用开发模式，需 app 角色账号）。

## Integrator 裁决（Claude Opus，2026-10-05）
1. 复制场次**连同直播价一起复制**（对齐 SHOPLINE「复制上一场」，owner 2026-10-05 要求一键复制开播）；R4 live-tools 规则 2「不复制直播价」仅对「从其他场次复制关键字商品」保留，对整场复制作废。
2. 场次归属采用 `claims.live_price_uses ∪ claims.order_origins(0113)`，替换 `claims.order_live_sources`；订单页场次筛选随之改变，须在测试里覆盖「无直播价的认领订单」归入场次。
3. 一单跨多场次：各场全额计入，接口返回 `multi_session_orders` 供界面脚注；不做分摊。
4. A7 `kwc-v1` 否定词/问句词表按草案的保守版本冻结；非 CONTAINS 场次的暂存行可带 `offer_id`（不含留言原文）——接受。
5. MCI02/KC03 的精确权限清单由 integrator 在合并时更新，执行者不改。
6. 已有购物车：**合并**（不替换）；兑换前先 `continueShopping`，已下单的购物车不得二次下单。
7. 迁移号暂定 0114（A5）/0115（A7）/0116（售罄读模型），以合并顺序为准由 integrator 最终分配。
8. 新增门禁模式批准：`--browser-live-flow`、`--browser-claim-checkout`，`--browser-webkit` 增第 7 步 `claim-checkout`（release-gate 行 6→7 步）。
9. 粉专直播中影片挑选保持 MOCK；status 取值与 U1（影片 post_id 是否等于留言 webhook 的 post_id）待 owner 测试账号做 LIVE 探针。
10. 本轮不做店铺级默认匹配模式（复制场次已带模式）。
