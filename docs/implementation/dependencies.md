# 依赖准入与当前清单

版本来自当前 `go.mod`/`go.sum` 和本阶段已记录的锁定证据；license 是上游声明，不等于本项目已完成法务准入。升级流程是确认候选版本、更新锁文件与本表、在候选版本上跑 gate，通过后再合并。

|依赖|固定版本|许可证|实际调用者|用途|升级测试|
|---|---|---|---|---|---|
|Go toolchain|1.27.1|BSD-3-Clause|全部 Go 包|编译、标准库、测试|`go test -race ./...`、`go vet ./...`、govulncheck；记录实际 toolchain|
|`github.com/jackc/pgx/v5`|v5.11.0|MIT|`internal/platform/platform.go`；`migrations/migrate.go`；`tests/foundation/*`|PG 协议、显式事务、连接池、事务类型|真实 PG18 基础 gate：RLS、scope 回收、超时、回滚、池上限、HTTP；再跑 unit/race/vet|
|`github.com/riverqueue/river`|v0.40.0|MPL-2.0|`migrations/migrate.go`；`internal/integrations/core/{service,dispatcher}.go`；`tests/foundation/{foundation_integration,external_operation,external_operation_authority,dispatcher}_test.go`|同库事务入队、受限真实 worker、幂等执行/只读对账、生命周期与 JobRescuer|迁移/权限、五事实原子回滚、busy snooze/terminal replay、真实 SIGKILL/JobRescuer、StopAndCancel、预算完成失败不取消、无原始敏感错误泄露|
|`river/riverdriver/riverpgxv5`|v0.40.0|MPL-2.0|同上|River 的 pgx driver|同 River gate；确认 schema 与权限不漂移|
|`github.com/coreos/go-oidc/v3`|v3.21.0|Apache-2.0|`internal/oidclogin`|发现、JWKS 签名及 issuer/audience/expiry 验证；nonce/subject/azp 由适配层额外验证。避免自制 JWT 验证器|签名 mock IdP 正负例、轮换/网络失败边界、真实提供商 sandbox 后才可上线|
|`golang.org/x/oauth2`|v0.37.0|BSD-3-Clause|`internal/oidclogin`|固定回调的授权码交换与 S256 PKCE；不存储提供商 token|错误 verifier、code replay、端点/重定向与超时负例|
|Node.js / pnpm|24.15.0 / 10.33.0|MIT / MIT|根 `package.json`；`packages/i18n/tests`；`payuni/acceptance_test.go` 调用 `scripts/dev/payuni-wire-check.mjs`|现有本机运行时；原生 TypeScript strip 测试与 pnpm 解析；Node/OpenSSL 独立核对 Go PAYUNi 表单（仅测试依赖）|`pnpm run test:i18n`；完整 Go gate 中的跨语言协议核对；所有 workspace 集成测试；不跨大版本自动升级|
|TypeScript|7.0.2|Apache-2.0|根 `package.json` 的 `typecheck:i18n`|编译期严格检查三语言字典和公共路由类型，非浏览器运行依赖|`pnpm run typecheck:i18n`；新 UI 接入后加入应用 strict build|
|Next.js / React / React DOM|16.3.5 / 19.3.0 / 19.3.0|MIT|`apps/admin/app`、`proxy.ts`、BFF route、客户端 Ledger|多语言 SSR 路由、交互和服务端令牌边界；遵循既定 Next 基线，未另外引入状态管理/表格库|strict typecheck、standalone build、三语言/手机/权限/幂等真实浏览器 gate|
|Playwright Test|1.63.0|Apache-2.0|`playwright.config.ts`、`tests/admin`|真实 Chromium 浏览器与网络丢响应/存储故障注入|九项台账回归 + BFF mock边界 + 实际PG/Next/Go签名mockIdP浏览器链，三组fixture隔离；不得把 mock 当生产验收|
|Prettier|3.9.8|MIT|开发格式化命令|将样式和 JSX 标准格式化以维护；不进运行时|格式化后 strict build 和浏览器回归|
|React/Node TypeScript types|React/DOM 19.3.0，Node 24.13.6|MIT|`apps/admin` TypeScript 编译|与已锁定运行时相配套，非运行依赖|strict typecheck|

间接模块由 Go module 解析，不在业务代码中直接调用；升级仍须保留 `go.sum`、扫描漏洞并跑上述 gate。不得用版本兼容或 CI 绿灯推断生产可用。

## Meta 入站协议复用关系

`internal/integrations/meta.NewHandler` → `Verifier.Verify` → 完整 `Batch` →
调用方同步提交回调，仅使用 Go 标准库 `net/http`、`crypto/hmac`、`crypto/sha256`、
`encoding/json`、`unicode/utf8`、`time` 等；没有新 SDK、队列或框架依赖。
原始字节先验签，再做 JSON token 校验；`pairedSurrogates` 补上标准 JSON decoder
会替换孤立 UTF-16 代理项的边界，防止不同消息内容的证据摘要合并。
`quarantineUnit` 仅附带小型 entry 上下文，不能改成每个未知单元复制全批次。
消息 MID 去重 Key 与 PayloadHash 分离；持久化冲突由下述 Inbox 接收层处理。
升级 Go 或改变归一化、ID、时间、错误响应时，须重跑同包及
`tests/integrations/meta` 的 raw-signature／Unicode／边界／ACK 阻塞验收和 race/vet。
此包当前未由运行时装配；PG、River、密钥轮换及可信资产归属没有通过此切片验收。
详见[协议验收与证据](2026-09-26-meta-webhook-protocol-acceptance.md)。

`newRawHandler` 在同一验签路径内把原始 `io.ReadAll` 自有字节同步交给私有回调；
公开 `NewHandler` 保留原兼容接口。`PayloadKeyring` 独立使用标准库 AES-256-GCM、
`crypto/rand` 12字节nonce及SHA-256，不复用支付密钥或其AAD。固定13元素JSON数组
AAD绑定类别、内部ID、app/object、摘要、tenant/store/route/epoch和KeyID；
`command.ValidID`复用既有规范UUID验证。变更数组编码就是协议迁移，不可随意换序。
升级Go/密钥配置/上下文/大小上限须重跑`payload_test.go`、独立
`payload_acceptance_test.go`互操作/轮换/篡改/错误摘要及`raw_handler_test.go`，
并保留完整MWP套件和race/vet。见[前置组件验收](2026-09-26-meta-inbox-primitives-acceptance.md)。

`NewInboxHandler` → 私有 raw 回调 → `Inbox.commit` → `pgx` READ COMMITTED
事务：`begin_batch` → 按 Key/hash/ordinal 排序的 `prepare_event` → AEAD →
已有 River `InsertTx` → `complete_event` → 原始批次 AEAD → `complete_batch` → COMMIT。
复用现有 `riverpgxv5`，不启动 worker，不新增消息代理/SDK。单次10秒预算、LOCAL
语句/锁超时及独立2秒回滚；历史收据先于可变路由解析，永久去重不随 River 清理。
`0028_meta_inbox.sql` 拆分可信资产注册、接收、审核清理三种专用权限；禁止共享
商家/worker/预定义系统角色。`post_river/0003_meta_inbox_queue.sql` 在上游迁移之后
限制固定 kind/queue/args，插入和更新均检查，延迟约束确保事件与任务同事务。
`purge_expired` 只由 curator 调用；年龄、显式终态证据及 River 终态三项均必要，
删除前以 FOR SHARE 锁定任务。全批次 raw 等待所有成员，元数据不删除。
预定义系统角色拒绝属于凭据配置门禁，不会撤销数据库管理员误授的直接 SQL 权限。
生产密钥加载器、可信 OAuth 签发、清理调度及公开处理器装配尚未实现；
消费者实现及独立验收边界见下段，不把入库验收外推到消费链路。
更改以上 SQL/角色/队列/AAD 要重跑 `scripts/dev/test-local.sh --meta-inbox`、完整
PG/race/vet 和独立审查；不能只跑 crypto 单测就宣称持久化可用。
当前本地门禁和运行范围见[入库验收](2026-09-26-meta-inbox-durability-acceptance.md)。

`NewConsumerWorker` → `platform.ValidateMetaConsumerPool` → 单个5秒
READ COMMITTED 事务 → `load_social_event` → `PayloadKeyring.open` →
`projectSocial` → `finish_social_event` → 延迟约束 → COMMIT。River 调度器必须
另用普通 worker pool，专用 consumer 登录不可修改 River 状态。分类器重用
`parseStrict`、`Verifier.change/message` 与 `emit`，重新核对原 Event.Key/hash；
不另写平台 JSON 解析器。`0029_meta_social_consumer.sql` 将结果保存至独立
`social` 域（会话、消息、评论观察），不写 webchat、identity 或交易域。
密文复制沿用永久 inbox event 的原 AAD，不重标类别；原入库正文过期后，社交域
仍保留独立密文。原时间戳不变，server_seq 仅代表成功入库顺序，不代表平台时序。
每个事实的 consumer_attempt 是延迟约束复核的真实任务代次，不是 caller GUC。
锁顺序固定为 tenant/store/binding/route/event/River/conversation；权限、授权证明
及任务状态在事务末尾再次核对。没有新增依赖、队列或服务。
启动权限检查的 `validatePoolAuthority` 同时检查继承 USAGE 与 SET 可达的对象
所有者，不能仅拒绝显式 SET。SQL 的 `require_authority` 保留原检查并追加新快照下
的递归 `pg_auth_members` 查询及当前数据库所有权检查；只重复 `pg_has_role` 不足以
覆盖已暖缓存、finish 后 COMMIT 前角色改变。原因、旧源失败复现及新源结果见
[消费验收记录](2026-09-26-meta-social-consumer-acceptance.md)。这段公共角色检查
影响所有专用连接池，改动必须跑完整回归，不能只跑 Meta 单测。
改动任一调用点须重跑 `--meta-consumer`、`--meta-inbox` 和完整 PG/race/vet，
并独立审查。当前接口[已冻结](../../contracts/meta-consumer-v1.md)，MC01–07
通过本地真实 PG/River、带数据升级、全仓 530 项回归和独立复核；详见上述验收
记录。尚无公开读取 UI、发送策略或已验收生产运行时装配，不等于完整 T07 已完成。

后续私有运行时候选的真实进程 MR04 **失败**：River v0.40.0 的 `Queues`
只限制取任务，leader 的 scheduler/rescuer/cleaner 仍覆盖整个 `Config.Schema`。
Meta-only worker 把付款任务从 scheduled 改为 available；不能把队列名当隔离边界。
详见[失败证据](2026-09-26-meta-runtime-acceptance.md)和
[隔离修订](../../contracts/meta-runtime-isolation-v1.md)。原生同 PG 独立 schema
方案须同时调整客户端、上游迁移、业务 SQL、触发器、权限及历史 jobID；仅改
Go 的 Schema 字段不够。旧 payment/expiry 共用 schema 的全局 rescuer 风险须另验。

## 商家账户接入复用关系

商家凭据HTTP/BFF不新增依赖：`cmd/api/accounts.go` 通过
`internal/integrations/accounts/env.go` 从受控环境读取 keyring，
复用 `accounts.NewKeyring`、`core.New` 和不启动 worker 的 River/pgx client；
`internal/httpapi/accounts.go` 复用 scoped transaction、严格 JSON/安全错误与既有
Create/Rotate/Get，`accounts/list.go` 复用绑定 tenant/store/collection 的 keyset 游标。
限流仅标准库 mutex/time，按已授权 tenant/store 分桶（60/min、4096 活跃桶），
不是分布式配额。`apps/admin/app/api/stores/[store]/[...resource]/route.ts` 复用
HttpOnly 会话、CSRF/Origin 和授权店铺列表，不转发浏览器提供的权限头。
0019只扩新店主 read/manage，升级须保留旧成员不回填和撤权后重放不复活测试。
升级这些入口须跑账户HTTP真实PG、密钥配置负例与真实浏览器BFF链；
配置及轮换边界见 [account-credential-configuration](account-credential-configuration.md)。

## 商家订单读取复用关系

商家订单只读增量复用 `platform.WithScope` / `RequirePermission`、`pagination.Page`、
pgx 显式事务和现有 `commerce_auth`，未增加依赖。`internal/merchantorders` 通过
`identity.read_merchant_orders` 读取原 checkout 快照和付款事实；不调用供应商、不
创建任务或新交易台账。0027 的列级授权、单语句数据快照、等待后的授权复查和
原商家成员不自动加权必须随 PG/pgx 升级重验，见
[商家订单读取与部署边界](merchant-order-reads.md)。该增量已通过
[MOR01–06 本地后端验收](2026-09-25-merchant-orders-acceptance.md)：最终 458 项
PG/race/vet 回归、跨店隔离和等待后权限复验；不含商家 UI 或正式部署。

浏览器接线另受 [MBT01–04 合同](../../contracts/merchant-orders-bff-v1.md) 约束：
复用现有 admin catch-all、HttpOnly 会话与店铺列表，禁止 fixture bearer 读取
收货资料；成功响应必须是 `private, no-store`。Next 的 URL 正规化也是调用链
的一部分，不能用直接构造 `Request` 的单元测试代替原始 HTTP 验证。
升级 Next 时须跑 `bash scripts/dev/test-local.sh --browser-merchant-orders-bff`，
并回归 `--browser-identity` 和 `--browser-merchant-buyer`。各轮结果、已发现的
正规化缺陷及当前验收状态见 [浏览器接线证据](2026-09-25-merchant-orders-bff-acceptance.md)；
该接线并不代表订单页面、发货或生产发布已验收。

## 独立付款 worker 复用关系

`cmd/payment-worker` → `accounts.LoadKeyring` / `platform.OpenWorkerPool` →
`payments.NewWorkerClient` → 原 `QueryWorker` / `CaptureWorker`。没有新增模块、
队列中间件或交易台账。`internal/jobqueue` 按服务端执行环境选择三条固定队列；
买家发起和查询后入账任务仍与各自业务写入同事务。

`migrations.Apply` 先应用业务 SQL，再运行 River v0.40.0 自身迁移，最后应用
带相对路径校验和的 `post_river/*.sql`。延迟约束触发器复用 PG18 提交时检查，
以冻结尝试／可信 QUERY 观察记录确定队列；旧客户端仍可默认入队。
既有非登录 integration writer 仅增加 River `UPDATE(queue)`，付款进程只取得
布尔型 `integration.payment_queue_ready()`，不直接读取凭据表。

升级 River/PG/pgx 时须重跑全量真实 PG/race/vet，尤其新库／旧任务升级、失败
原子回滚、旧通知队列与轮询、混合环境、重复入账及进程退出门禁。固定队列只覆盖
付款查询和入账，不会启动到期／默认队列的外部操作 worker。部署开关、历史密钥
及回退边界见 [付款 worker 运行说明](payment-worker-runtime.md)。

## 独立到期 worker 与共享生命周期

`cmd/expiry-worker` → `platform.OpenWorkerPool` → `checkout.NewExpiryClient` →
原 `ExpiryWorker` → `checkout.expire_held`。只消费 `jobqueue.CheckoutExpiry`，
不需要付款 keyring、供应商配置或新的库存 writer。`checkout.Begin` 的现有
`InsertTx` 显式设置同一队列，仍与订单和预留同事务。

`post_river/0002_checkout_expiry_queue.sql` 复用现有校验和／锁／迁移阶段，
将合法旧任务归队。NOLOGIN checkout writer 只新增 River `UPDATE(queue)`；
提交时 AFTER INSERT 触发器核验不可变 `orders.job_id` 和原始 generation=1，
不与付款后变化的当前版本比较。ready 函数只向 worker 返回布尔值。

`internal/jobqueue.Run` 抽出付款进程已经验收的 10 秒启动看门狗、15 秒正常／
5 秒取消停止逻辑，供付款和到期两个入口共同调用；各自保留连接池与错误映射。
没有新增依赖、通用进程框架或动态队列选择。升级 River/PG/pgx 必须复跑两个
真实进程的启停／强杀恢复、旧生产者轮询、付款与到期锁竞争和完整回归；
说明与停止线见 [到期 worker 运行说明](expiry-worker-runtime.md)。

## 商家设置向导复用关系

`SettingsWizard` → 同源 Next BFF → Go `settings_discovery.go` →
`pricing` / `payments` / `fulfillment` 的现有事务与命令回执；没有新增包或 migration。
市场分页复用 UUID keyset；配送服务目录按稳定 code 游标；台湾收款方式固定五种。
页面复用 `WorkspaceFrame`，所有语言由真实店铺和 session 决定上下文，语言不改币种。
账户 credential 只保留在内存输入框；请求派发前仅把原 key/非秘密上下文写入恢复日志，
不得记录密钥。非秘密草稿携带目标和已观测版本，显式重载会丢弃未保存账户绑定；
自动刷新不能给脏表单升级 CAS 基线。升级必须跑真实 browser-identity gate（包含
settings-real），保留崩溃恢复、双账户非首项回填、并发修改与跨会话零写测试；
Go 发现接口还须保留真实 PG 锁等待撤权负例。验收与外部资格边界见
[merchant-settings-wizard-acceptance](2026-09-24-merchant-settings-wizard-acceptance.md)。

## 计划依赖（当前 NOT_INSTALLED）

|依赖|状态与边界|准入条件|
|---|---|---|
|sqlc|NOT_INSTALLED；当前 SQL 为显式参数化，尚无生成物|冻结 queries/schema 后由 integrator 锁版本，审生成 diff，跑真实 PG/RLS/事务 gate|
|LiveKit|NOT_INSTALLED；媒体与 API 尚未接入|先完成托管/自托管能力与授权探针、mock→sandbox gate；媒体失败不得改变订单真源|
|PSP SDK|NOT_INSTALLED；PAYUNi 有标准库 wire adapter，PROTOCOL_MOCK；无真实收款/退款调用|先确认商家 MoR、sandbox 账户与 webhook 幂等/对账；live 需明确授权、金额、回执和回滚边界|

任何新依赖须说明为何标准库/现有包不能满足、调用者、license、版本来源、移除/升级测试和生产影响；未满足前标 `NOT_INSTALLED` 或 `BLOCKED_EXTERNAL`，不伪造可用性。

买家匿名凭证切片不新增依赖：`internal/buyer` 使用现有 pgx 的参数化 SQL/事务，
标准库 `crypto/rand` 生成 32 字节随机凭证、`crypto/sha256` 仅持久化摘要。
`platform.OpenBuyerPool/OpenBuyerIssuerPool` 复用启动角色检查，但不复用商家权限。
SQL 依赖入口及将来开放 HTTP 前的条件见 `contracts/buyer-capability-v1.md`；
改动 PG/pgx 或事务包装时，必须重跑真实数据库的角色矩阵、撤销/过期锁竞争和回滚负例。

已发布域名解析前置也不新增依赖：`internal/domains` 调用现有
`platform.ValidateBuyerIssuerPool`（复用启动角色检查）及 `command.ValidID`，
用 pgx 参数化调用 `buyer.resolve_published_store(text)`。0020 的两个 control
表原先仅给 non-login buyer writer SELECT；issuer 只可执行固定函数。
0023 的商家反向入口额外给现有 non-login commerce_auth 所需列的 SELECT，
配套单独只读 RLS policy；不扩大任何应用角色的表读取或写入权限。
解析没有缓存，不负责 DNS/TLS 验证、发布写入或买家身份。升级 PG、pgx、角色
包装或域名语法时必须跑 `TestPublishedStorefront*` 的真实 SQL/Go 语法一致性、
权限、租户 FK、停用和锁等待后过期用例，以及全量后端回归。合同：
[published-storefront-resolver-v1](../../contracts/published-storefront-resolver-v1.md)。
可信控制面写入与公开 HTTP/浏览器准入仍是独立门禁。

商家商品购买入口不新增外部依赖：`httpapi` GET → 既有 `platform.WithScope`
→ `catalog.ReadPurchaseEntry` → `identity.resolve_storefront_origin`，后者再次
验证会话与 catalog:read 并匹配事务 GUC，再读取已发布域名。URL 语法复用
`domains.ValidOrigin`，不维护第二套弱校验。SQL0023 的候选资格在同一 SELECT
内先过滤再 LIMIT 2，多域名不猜主域名，唯一域名在返回前复核时钟。既有
商品/SKU命令与原始幂等回执保持不变；URL读取不调用 PSP 或创建买家订单。
修改角色、校验、事务或投影须跑 `--purchase-entry` 真实 PG/HTTP 子集与
完整 race/vet；复制入口和实际买家页面仍必须另过浏览器 gate。

B 商品页复用上述公开 BFF 与 `buyer-client` 会话协调器：`ProductPurchase`
→ `purchase.ts` → `buyerRequest` → 私有 Go cart/quote。`purchase.ts` 只持久
非 PII 的 cart/quote 幂等请求，复用 `definiteError` 判定确定失败；不能扩展为
姓名/电话/地址或 bearer 存储。CurrentDestination GET 复用原 destination
投影与 RLS，发现 head 不证明丢失请求的归因。三语复用 workspace i18n；没有
新增外部库或第二交易引擎。更改请求/恢复须跑 storefront Node tests 和实际
`--browser-buyer`；[当前范围与未完成门禁](2026-09-25-buyer-inline-progress.md)。

买家私有 HTTP 同样不新增依赖：`cmd/api/buyer.go` 负责三个既有独立权限池
的装配/失败关闭，`internal/buyerhttp` 借用池并消费 `domains`、`buyer`、
`storefront`、`checkout`，不新增 SQL writer 或运行 River worker。标准库
`net/http` 负责严格路由和请求期限，`encoding/json` 负责有界类型解码，
`httperror` 负责公共错误封装；具体 projection 类型不序列化内部原始结构。
外层 mount 保留原始路径送入内层校验，不能换成自动清理路径的重定向。
升级上述入口、Go/pgx、权限、期限或 DTO 时须运行 `--buyer-http` 的真实
HTTP/PG和实际cmd/api三池清理测试，再跑全量race/vet。公开BFF/cookie/CSRF
和浏览器验收仍为独立前置。见 [私有HTTP验收](2026-09-25-buyer-private-http-acceptance.md)。

安全重试注册 `buyer.Service.RegisterForTrustedStore` 仍借用 issuer pool，标准库
SHA256 只持久化服务端已有随机 token 的摘要；不引入缓存、HMAC 派生或新依赖。
0021 的 `buyer.register_capability` 复用 0006 的 hash 唯一约束、issue 和
resolve_scope：只在异常子事务内插入，精确匹配 token constraint/schema/table
后回滚竞争失败方，再按既有锁顺序读取原身份和期限。PUBLIC/runtime 不得执行。
`buyerhttp` 新增内部 POST session/bootstrap，拒绝浏览器头和幂等 key，返回
固定两字段；旧 POST session 仍不可自动重试。升级 PG、pgx、会话规则时跑
`TestBuyerHTTPRegistration*`：丢响应、两个实例、强制 unique wait、无孤立 owner、
非目标23505传播、REPEATABLE READ失效快照、ACL、撤销和发布状态变化，然后全量
race/vet。它自身不解决首次 Cookie 多标签竞争；后续公开BFF见下一段。
见 [注册合同](../../contracts/buyer-session-registration-v1.md)。

公开买家传输 `apps/storefront/lib/buyer-server.ts` 使用已有 Next/React/TypeScript
版本及 Node crypto/net；固定上游、逐请求 published-origin、host-only HttpOnly
签名 cookie、独立 context/CSRF、严格 JSON（含 own-key）及有界流读取。无新认证
框架或支付 SDK。`buyer-client.ts` 仅用 Fetch/Web Locks/localStorage；pending
journal 不是身份凭据，未知交付禁止再次准备，明确失败可以恢复。prepare/reset
之外不写 cookie。修改 cookie/fetch/锁/Next/Node 时必须跑真实 `--browser-buyer`，
仅 stub fetch 单测无法证明浏览器 cookie 交付、原生重试、退出或多标签行为。

Go `RetireForTrustedStore`/HTTP retire 调0022的固定 SECURITY DEFINER 函数：
未知 hash 在同一事务 register+revoke，保留原行防止迟到 bootstrap 复活；原 DELETE
的 unknown204仍只是no-op。`RegisterForTrustedStore` 改调 limited wrapper，按店铺
advisory lock、插入时间所属UTC分钟的索引计数限600新行；非READ COMMITTED失败关闭。
不新建身份/配额表；旧私有issue不在公开限额承诺内。修改PG/锁/函数授权时运行
`TestBuyerHTTPRetirement*`/`TestBuyerHTTPSharedQuota*`及全量race/vet。
三语买家UI、分享路由、真实DNS/TLS控制面、PSP和可信CVS仍独立验收，详见
[公开传输验收](2026-09-25-buyer-browser-bff-acceptance.md)。

买家商品发现 `storefront.ListCatalog` 复用 `buyer.WithScope/CheckScope`、
0007 的 catalog/control 列级 SELECT 和 RLS，只读联结 active 商品/规格及
当前店铺币种；没有第二商品真源或新增角色。`pagination.Request/Page` 复用
列表类型，但不复用包含明文 tenant/store 的旧游标编码；新的标准库 JSON、
SHA256、base64url 只绑定分页位置，不提供身份授权。`buyerhttp.projectCatalog`
固定七个展示字段；只有规范 `GET /v1/buyer/catalog` 接受三个查询参数，旧路由
仍拒绝 query。修改这些边界时运行 `TestBuyerHTTPCatalog*` 的真实PG/HTTP、
域单元游标反例及完整race/vet；不得将商品展示价格当成最终报价或收款资格。
合同见 [buyer-catalog-discovery-v1](../../contracts/buyer-catalog-discovery-v1.md)。

`checkout.Service.ListOptions` 使用已校验的 checkout runtime pool，而不是增加
buyer pool 权限或伪造商家授权。0013 已提供所有参与表的只读权限及 RLS；
当前市场/政策/配送和分仓配置通过完整自然键联结。分仓保存时的 service_version
只是写入 CAS 来源，不应在服务改名后强制相等；每个已分配仓库仍须有效，但
发现列表不计算可售库存。`buyerhttp.projectOptions` 固定十五字段，最终 Quote/
Begin 继续作计价与锁库存校验。改动需复跑 `TestBuyerHTTPOptions*`、游标/投影
单测和全量 race/vet，见 [合同](../../contracts/buyer-checkout-options-v1.md)。

内部 pricing/cart/quote 同样不新增依赖：`internal/pricing` 以现有 pgx 和受限整数
计算；`internal/buyer/command.go` 复用事务、JSON/SHA-256；`internal/storefront`
消费这些服务及 catalog 的 PG 行锁。升级 PG/pgx 时追加运行
`tests/foundation/{pricing,cart_quote}_test.go` 的策略版本、金额、RLS、CAS、锁竞态、
并发 replay、原子回滚 gate。合同与证据分别见 `cart-quote-v1.md`、
`2026-09-20-cart-quote-acceptance.md`；没有引入第二交易引擎。

`internal/storefront/revalidate.go` 继续调用同一 `pricing.Calculate`、cart/catalog
锁读取与 pgx 事务，不另写 SQL 计价器或增加校验签名依赖。共享 `readQuote` 将
损坏的存储 JSON 统一映射为 conflict。升级这些入口/PG/pgx 时，追加运行
`tests/foundation/checkout_quote_test.go` 的版本/金额损坏、锁等待后过期和取消
后连接池复用 gate；它不替代完整 checkout/PSP/物流验收。

`internal/fulfillment/service.go` 复用 pgx、`platform.RequirePermission`、
`command.Run/Audit`、pricing 的版本化政策和市场锁；不调用 River 或供应商。
`pricing.ValidMethod/DeliveryMethod` 与 Quote 共用受限 method 闭集；不复制运费
计算。迁移 0010 保证同店历史政策/binding 外键、API-disabled 及历史不可改。
升级上述入口/PG/pgx 时须跑 `tests/foundation/delivery_service_test.go` 的并发 CAS、
replay、GUC/权限、policy/market 独立停用与审计/回执故障回滚。SQL 与 Go Unicode
名称规则的已知 P2 差异及精确边界见 `2026-09-24-delivery-service-acceptance.md`。

仓库配置继续复用 `fulfillment.authorize`、`command.Run/Audit`、pgx 与既有
inventory writer；`inventory.lock_warehouse` 只读加锁，不授 runtime UPDATE。
0011 的 header/children 延迟完整性约束和 scoped FK 是正式提交门禁，不可被
仅 Go 校验替代。`inventory.PlanAllocation` 是标准库排序／映射的纯函数，复用
Line/Balance/MaxQuantity，不建第二库存账簿。升级 PG/pgx/权限入口须跑
`delivery_allocation_test.go` 的提交约束、晚插入、跨 scope、回放、锁等待与故障回滚；
替换规划器需保留 800 行边界、缺货无部分计划和数量守恒测试。详见
`2026-09-24-delivery-allocation-acceptance.md`；后续 checkout 消费证据见下节。

门市来源／收货快照仍不新增依赖：`fulfillment/pickup.go` 复用已有鉴权、
`command.Run/Audit`、pgx；`storefront/destination.go` 复用 buyer scope/command、
cart share lock 和 fulfillment 的只读／加锁投影。标准库只做输入与时间校验，
没有新目录抓取、carrier SDK 或密钥服务。迁移 0012 的 scoped FK、FORCE RLS、
buyer no-op UPDATE 拒绝、主体／会话 provenance 不能由 UI 校验替代。
升级 PG/pgx/权限包装或上述依赖入口须跑 `buyer_destination_test.go` 的真实角色、
CAS、直接买家 SQL、回执隐私、事务回滚和真实锁等待过期 gate；将来正式目录 adapter
需另有来源证明及回调验收，不得沿用 MANUAL_ATTESTED 标签伪称官方验证。
见 `2026-09-24-buyer-destination-acceptance.md`；没有新增 checkout 库存写权限。

内部 checkout 不新增模块依赖：`checkout.Service.Begin/Get` 复用 pgx、
`buyer.WithScope/CheckScope`、`storefront.RevalidateQuote/RevalidateDestination`、
`fulfillment` 当前服务／仓库配置和 `inventory.PlanAllocation`。锁住余额后再分配，
最后重验 DB 时间与 capability。普通 buyer 权限不升级；新的专用 pool 使用
`platform.OpenCheckoutPool/ValidateCheckoutPool`，所有其他角色也拒绝混入此权限。
迁移 0013 的固定 SECURITY DEFINER writer 只授 checkout runtime EXECUTE，
重解 scope 并清除商家 GUC；价格真源仍是同一个 Go calculator，不能绕过 Go 层
把该可信服务凭证当作公开 SQL API。库存余额仍只由原 `inventory.apply_ledger`
更新，不复制台账或新增订单行价格引擎。BUYER／MERCHANT／SYSTEM_EXPIRY 的
actor 与父预留约束、scoped FK、永久私有回执一起阻止普通角色伪造 checkout。

`checkout.NewExpiryWorker` 使用已经固定的 River typed worker、`InsertTx` 和
`JobSnooze`；同事务插入 `checkout_expiry_v1`（只有 order ID/generation/version），
真正到期以锁后的 DB 时钟为准，不用调度器时间推导订单状态。SQL expire 函数
锁定订单／预留／余额，原子释放并写事件；早到 snooze，旧 generation／终态无操作。
这是内部可装配库，生产进程装配和到期任务巡检恢复尚未交付。升级 River／pgx／PG、
权限、计价器或规划器时必须跑 `buyer_checkout{,_edges}_test.go` 的真实 PG 全套，
包括最终等待后过期、7类写入故障因果、提交前不可见、legacy权限防绕过及实际
River due/early/stale/duplicate；`--checkout` 仅快速定向，不替代整套门禁。
完整证据见 `2026-09-24-buyer-checkout-acceptance.md`，无生产 PSP／物流调用。

商家供应商凭据 `internal/integrations/accounts` 继续复用 pgx、
`core.RegisterBinding`、`command.Run/Audit` 与 `platform.RequirePermission`，
不建新授权/队列。Go 标准库 AES-256-GCM 做本地存储加密，HMAC-SHA256 使用独立
稳定 replayKey；这与下述 PAYUNi 传输协议的加密/签名不是同一个功能，不能互换。
Keyring 复制输入，AAD/明文及 HMAC 字段顺序在 `merchant-accounts-v1.md` 冻结，
golden digest 防止无意改动永久幂等编码。正常加密 active key切换可继续原请求
重放，但不允许直接替换 replayKey；生产环境加载、密钥托管/备份与迁移尚未装配。
0014 复合 FK 绑定准确商户/环境，credential 正文只有 INSERT 权限，runtime仅能
读元数据，worker没有任意解密权限。升级 crypto/pgx/PG/command/binding 时重跑
`merchant_accounts_test.go`：实际持久密文独立解密、错scope/AAD、密钥切换重放、
不同key账户重复、轮换CAS、精确列权限与sequence证实故障回滚。生产取凭据须
以持久操作的准确账户和权限为入口，不能直接把内部密文表暴露为通用查询 API。

支付方法配置 `internal/payments` 不增加依赖，复用 pgx、`pricing.LockMarket`、
`platform.RequirePermission` 与 `command.Run/Audit`。不把支付配置塞进配送费政策，
不另建计价器、通用供应商框架或队列。0015 的 revision/head、完整范围 FK、RLS
和列权限保护历史；Go/SQL 同时禁止在没有 adapter 时启用。方法状态与连接密钥
版本分离，读取诊断只查账户/绑定元数据，不读密文。锁序 market→method→account
→binding 与账户轮换保持一致；所有等待后再核权限。升级这些依赖时必须重跑
`payment_methods_test.go`：并发同键/CAS、实际阻塞后撤权、环境与账户隔离、
SQL 约束错因核对及用 sequence 证明到达注入点的八类原子回滚。
`InspectMethod` 是商家诊断，不是给将来买家 StartPayment 缓存的授权票据。

托管付款内核（合同 `contracts/payment-hosted-v1.md`；HP01–07限定内部MOCK验收
通过，见 `2026-09-25-payment-hosted-acceptance.md`）：
`checkout.BeginHosted` → 共用 `startPaymentTx` → 原0016支付/库存/任务事务 →
`accounts.Keyring.BuildPaymentHosted` → 0025精确订单历史凭据 → 原PAYUNi
`BuildHosted` → 0025不可变表单，全部在同一 `buyer.WithScope` 事务中提交。
`TakeHosted` 只在一次性交付标记提交后返回表单；重放不返回表单、不更新时限。
0025函数和普通checkout SQL账号隔离；`platform.OpenHostedPool` 要求独立LOGIN
继承 `commerce_hosted_runtime`（INHERIT TRUE、SET FALSE），不持有writer身份。
配置的ReturnURL/NotifyURL由服务端固定，凭据沿用原Keyring/AAD，SQL不解密内层
交易金额；可信Go签名边界需由独立wire解密门禁验证。没有新增第三方依赖。
升级platform/pgx/PG/River/Keyring/PAYUNi时须重跑HP01–07及原支付/入账/T06全套；
只过此内核不表示公开付款路由、实际供应商收款、资格或生产部署已经可用。

买家付款私有接口（`contracts/buyer-payment-http-v1.md`）：
`cmd/api.loadBuyerConfig` → 默认关闭的 `loadBuyerPaymentConfig` → 原
`loadAccountKeys` + 独立 `OpenHostedPool` → `buyerhttp.New` 的可选 hosted 服务。
GET `PaymentView` → SQL0026 单快照查原订单、冻结支付事实、当前付款方式资格；
末尾重复 capability/DB 时间检查，HTTP 不返回账户标识、凭据或表单。
POST prepare → 原 `BeginHosted`，仅投影四个回执字段；POST handoff → 原
`TakeHosted`，禁止 body/replay key，所有交付失败不可自动重试。返回/通知URL
固定在服务端；API 只插入原 River 任务，不启动 worker。升级通用 buyer router、
Keyring、PG/pgx 或 hosted 内核须同时重跑 BPH01–06 和 HP01–07；
[私有接口验收](2026-09-25-buyer-payment-http-acceptance.md)保留实际角色装配、
连接回收、锁等待过期与三语 HTTP 证据。没有新增依赖；公开 BFF/付款UI及真实
供应商准入仍独立验收，不允许把通用前端重试日志用于 handoff 或保存付款表单。

买家付款公开传输（`contracts/buyer-payment-public-v1.md`）：
`handleBuyerRequest` → 原 cookie/context/Origin 检查 → 单次私有 Go 请求 →
`payment-contract.ts` 三个精确类型守卫。仅新付款响应使用可识别 null 的重复键
扫描，普通请求仍默认拒绝 null；准备回执限制为当前信用卡 TWD 整元准入。
`buyerRequest` 仅对精确的 POST `orders/{uuid}/payment/handoff` 放行空 body/key，
保留会话 journal 检查；它不负责重放、保存表单或认定已付款。新 UI 必须在原
购买锁内协调并仅用 GET 恢复未知交付，不能接入通用购买重试按钮。
没有新增依赖。升级 Next/fetch、cookie／JSON 校验、付款 DTO 或 Go hosted wire
时重跑 BPT01–06、原34项前端回归和 BPH01–06；真实付款浏览器链仍独立验收。
详见[公开传输验收边界](2026-09-25-buyer-payment-public-acceptance.md)。

买家 B 付款界面（`contracts/buyer-payment-ui-v1.md`，浏览器增量验收中）：
当前订单与历史详情共用 `OrderDetails` → `OrderPayment` → `order-payment.ts`
→ 原 `buyerRequest` / `readPurchase` → 上述 BFF / Go hosted 内核。
`order-payment.ts` 在原 `commerce-purchase-write-v1` Web Lock 内使用独立的
逐订单非敏感意图标记，不覆盖购买恢复日志或当前订单定位器；原 key/body
仅显式重放 prepare，handoff_started 持久化后仅 GET 恢复，不重放 Take。
浏览器原生 Window/DOM/form 提交只面向固定白名单；受控空白子页先断开 opener，
再以文档身份、所选订单代次和买家上下文作前后校验。表单不进入持久存储。
`payment-return.ts` 仅依赖 Node crypto 为固定样式产生 CSP hash；固定 GET/POST
路由完全不读取请求，无 cookie、跳转或交易写入。部署必须另外验证中央 HTTPS
ReturnURL 的实际域名与路由，不凭本地通过宣称生产就绪。
未新增依赖。升级浏览器 API、React 生命周期、Next headers 或既有会话锁时，
需重跑 BPU01–04 与 BPT/BPH 回归；三语、原生表单、历史选择切换和丢响应均为
验收项。提供商不可用/路由默认关闭与临时读取失败不在客户端猜测区分，统一
停止付款并允许 GET 刷新；只有有效 PaymentView 的空 methods 才表示无可用方式。

`internal/checkout/payment.go` 复用 buyer.WithScope、checkout pool 验证、River
InsertTx 和私有回执，调用0016的 `checkout.start_payment`；不依赖 HTTP/PSP wire，
不读密文。资格表当前没有应用签发者，MOCK只在隔离测试中注入。operation新增
BUYER_PAYMENT_QUERY family；merchant Get/Dispatcher排除，Claim只为此family允许
历史停用绑定的reconcile。payment_query_v1 worker读取frozen credential版本，
不复用商家Dispatcher的重试期限。升级这些模块/PG/River时重跑TestBuyerPayment
和全部T06；--payment仅诊断，全量包runner240s但各测试期限不变。无新增依赖；
边界及失败证据见 `2026-09-24-payment-start-acceptance.md`。
实证见 `2026-09-24-payment-methods-acceptance.md`，无供应商交易和支付页面验收。

`internal/httpapi/settings.go` 注册五个方式配置/诊断路由，复用同包
`scoped/bodyRoute`、`httperror` 与 `fulfillment.SetService/GetService`、
`payments.SetMethod/GetMethod/InspectMethod`。不接受凭据，不改变领域启用门禁。
升级 HTTP/认证/command/fulfillment/payments/PG 时重跑
`settings_test.go` 与 `merchant_settings_http_test.go`：精确目标、完整 PUT、永久
回执/CAS、诊断无写入、撤权后锁等待的新写/重放/读取拒绝。最终权限检查放在领域
函数中，不能退化为只有 HTTP 包装层检查。见 `2026-09-24-merchant-settings-http-acceptance.md`；
没有新增依赖、数据库迁移或浏览器/供应商调用。

`internal/integrations/psp/payuni` 不新增 Go module，不导入 SQL、River 或账户库。
标准库 `crypto/aes/cipher/sha256/subtle`、`net/http`、`net/url`、`encoding/*`
实现官方 UPP v2.0 与查询 wire；固定官方端点、TLS1.2+、10秒、一次请求无重定向。
客户自有 HashKey/IV 由查询读取器按 durable attempt 的准确账户/环境/历史版本加载，
不是任意商家可调用的通用解密 API；未签名的 outer MerID 不能提供账户权威。
供应商固定 IV/GCM 是 wire 兼容限制，禁止拿来替换本地随机 nonce/AAD 存储加密。
未知、取号、处理中、不完整记录只是 `Observation`，没有 `Paid` 或入账副作用。
未引入/复制上游 PHP SDK，其固定提交仅作 wire 参考（尤其不能复制其关闭 TLS 的设置）。
测试独立使用 Node24.15.0 内建 OpenSSL 核对官方固定向量及 Go 五种表单；无需 npm 包，
`test-local.sh` 在创建 PG 前检查 Node。更新 Go/Node/协议时须跑全部同包顶层测试、
完整 race/vet；供应商字段/原始 Result 编码/回调 ACK/轮换规则仍需商家沙箱验收。
`AmountTWDFromMinor` 仅精确转换本系统两位TWD minor units，不舍入或覆盖订单价格；
金额单位或Intl依赖变更须复核转换与界面缩放一致性。两种PG fixture均等待TCP就绪，
不可退回socket探针（官方镜像初始化临时服务会先通过socket后关闭）。
见 `2026-09-24-payuni-wire-acceptance.md`；适配库存在不等于已准入或可以启用收款。

`accounts.LoadPaymentQuery` → 0017精确租约/历史版本 → Keyring.open →
`payuni.NewQuery` → `payments.NewQueryWorker` 是查询执行链；复用 River、core
Claim/Complete、pgx及标准库，不增加依赖。query-only客户端禁表单/通知且不保留
空闲连接；仅MOCK允许显式RoundTripper，SANDBOX/LIVE不得注入。DB读事务先提交，
网络仅一次调用；结果与UNKNOWN完成同事务，错误/预算也在写入后再次核租约时限。
历史解密失败保留已验证年龄，防止绕过24小时预算；只有SQL成功后才可保留该年龄。
升级以上依赖或0017必须跑`payment_query_test.go`、全部T06及完整race/vet；涵盖
历史密钥轮换、坏签名/金额/商户、超时/panic、租约锁等待、跨attempt引用、并发去重、
预算持久化和父取消。详见[查询验收](2026-09-24-payment-query-acceptance.md)。
查询层只留存可信报告；后续入账由下述独立本地事务完成，不开放商家启用。

0018把该报告与 `payment_reconcile_v1` 的PG规范化hash/args一起提交；
`payments.NewCaptureWorker` → `payments.apply_capture` → 单一库存ledger触发器，
在订单→预留→排序余额锁内原子写财务事实、allocated、订单和商家待办。
复用已有checkout私有writer，不加新的stock writer、planner或空carrier job。
依赖River消费允许乱序：旧authorization-only缺少更晚阶段证据不等于冲突，
显式取消/失败、退款提示与不一致金额仍可粘性hold；原始checkout session
与后来付款session必须分开。SQL固定22023/PT409为永久任务拒绝，其余DB错误
重试且不泄露原文。River表读授权在upstream迁移后，不放入应用SQL建表阶段。
升级River/PG/pgx或金额/状态投影须跑`payment_capture_test.go`、全部既有支付/T06
和完整race/vet，包含真实队列、跨租户、逆序、多SKU及各写入点因果故障。
见[入账验收](2026-09-24-payment-capture-acceptance.md)。无新增依赖；银行结算、
完整退款历史、复核处理、真实资格、生产进程和公开页面仍未交付。

内部 external-operation 继续复用 pgx、River InsertTx、`command.Run` 与标准库
JSON/crypto：`internal/integrations/core` 负责精确权限、不可变意图摘要和租约 token
包装；迁移 0008 定义的受限 SQL 函数是 worker 状态迁移的唯一写入口，前向
迁移 0009 仅扩展合法首次 dispatch 的 BLOCKED_POLICY 完成结果。
`platform.OpenWorkerPool/ValidateWorkerPool` 检查 session_user 并拒绝 SET ROLE
伪装、owner/混合权限；`migrations/migrate.go` 在 River
迁移之后重设运行权限，并明确收回 worker 对 `river_migration` 的所有权限。
升级 River/PG/pgx 必须重跑 `external_operation*_test.go`：单赢家 claim、过期仅
reconcile、旧 token/generation 拒绝、binding 撤销仍保留远端已知结果、跨权限拒绝、
五类事实原子回滚、缺 receipt 后永久摘要仍冲突，以及普通 worker 的真实启停。
历史 probe 仅验队列权限，详见 `2026-09-20-external-operation-acceptance.md`。
实际 dispatcher 继续复用该依赖，见 `2026-09-20-dispatcher-acceptance.md`。
升级还须运行 `dispatcher_test.go` 的真实进程恢复、一次共享回调 deadline、
故障回滚后持久化预算、停止后只查询恢复及回调快照隔离 gate。River 短延迟
重试/snooze 可处于 available 而非 retryable；以锁定版本实现与持久化事实
校验，不把单个队列状态字符串误当业务失败或成功。仍无生产 provider。

2026-09-20：上述前端版本通过 npm registry 元数据与本地锁文件核对。`pnpm audit --prod` 返回 No known vulnerabilities found；这是当时依赖审计，不代表整体安全验收。Next 的匹配版本文档随 `apps/admin/node_modules/next/dist/docs` 提供；standalone 显式补齐 public/static，不使用有警告的 next start。

2026-09-20：OIDC 版本由 `GOTOOLCHAIN=go1.27.1 go list -m -json <module>@latest` 核对后固定，不使用漂移的 latest 构建。许可证已读取下载模块 LICENSE。调用边界参考 [go-oidc 官方示例](https://github.com/coreos/go-oidc/blob/v3/README.md) 与 [OAuth2 PKCE API](https://pkg.go.dev/golang.org/x/oauth2)。标准库没有完整 OIDC/JWKS 验证器；移除依赖前必须替换完整验证能力，不可改为只解码 JWT。真实 IdP、邮件和浏览器会话仍是独立 gate。
