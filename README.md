# 直播电商 SaaS 架构与 Codex 启动包

## 2026-09-20 实施入口

已开始开发，当前已有 **Go/PostgreSQL 基础、商品库存台账、商家 C 分步向导、买家 B 商品直购／订单／付款页面及独立付款／到期 worker 的局部本地验收，不是完整 SaaS 或生产上线版本**。付款供应商仍为签名模拟；真实身份提供商、收款准入、部署及其余业务模块仍需完成。

- 当前实现：[foundation 合同](contracts/foundation-v1.md)、[基础 OpenAPI](contracts/foundation-openapi.json)、[实施说明](docs/implementation/2026-09-20-kickoff.md)。
- 本地一键验收：`bash scripts/dev/test-local.sh`。需要 Docker、Go 启动器和已下载的固定 PG18.6 镜像（镜像 digest 见脚本）；会创建本任务临时数据库，退出自动移除，不读取现有 `DATABASE_URL`。
- 仅编译/单测：`GOTOOLCHAIN=go1.27.1 go test -p 1 -race ./...`。未配置隔离数据库时，集成测试明确 SKIP，不能视为数据库通过。
- 服务入口：`GOTOOLCHAIN=go1.27.1 go run ./cmd/api`。需要已迁移数据库和 **commerce_runtime 成员、非 owner 的独立登录**，通过环境变量 `DATABASE_URL` 传入。默认仅监听 `127.0.0.1:8080`；程序不会自动迁移、生成用户或放宽授权。
- GitHub 检查配置已加入 `.github/workflows/foundation.yml`，但尚无远端仓库或真实 CI 执行回执。
- 身份浏览器链：`bash scripts/dev/test-local.sh --browser-identity`。真实隔离 PG + Next/Go，外部 IdP 为签名 mock；[合同](contracts/merchant-browser-auth-v1.md)、[传输验收](docs/implementation/2026-09-20-browser-identity-acceptance.md)。用户已批准 C 分步向导，[界面与最终浏览器验收](docs/implementation/2026-09-20-entry-wizard-acceptance.md)；身份开关仍默认关闭，未接通生产 IdP。
- 买家匿名凭证：独立 SQL 角色、hash-only、过期/撤销与并发隔离已通过真实 PG 验收；[合同](contracts/buyer-capability-v1.md)、[证据](docs/implementation/2026-09-20-buyer-capability-acceptance.md)。内部权限内核由私有 HTTP 消费，后续浏览器传输验收见下。
- 已发布域名解析前置：独立发布状态、精确 HTTPS 域名映射、验证期限、停用与角色隔离已有本地实现；[5项定向PG及317项后端回归](docs/implementation/2026-09-25-published-storefront-resolver-acceptance.md)。测试域名事实均为合成；可信域名/发布写入、公开买家接口和真实部署仍待完成。
- 买家私有 HTTP：独立 BFF 凭据、逐请求已发布域名解析、买家权限、严格 JSON、白名单响应、启动失败池清理；[合同](contracts/buyer-http-v1.md)、[4项真实HTTP/PG及339项race/vet回归](docs/implementation/2026-09-25-buyer-private-http-acceptance.md)。仅默认关闭的 loopback 传输，不是公开 BFF、买家页面或真实支付接通。
- 内部购物车/报价：owner 隔离、版本化市场计价、不可变快照、并发与回滚通过；[合同](contracts/cart-quote-v1.md)、[96 项历史后端验收](docs/implementation/2026-09-20-cart-quote-acceptance.md)。现有私有 HTTP 包装；购物车/报价本身不创建订单或扣库存。
- 下单前报价重验：锁定当前购物车/政策/商品、复用原计价器核对金额，在锁等待后检查 DB 时间；[合同](contracts/checkout-quote-validation-v1.md)、[128 项回归与 40 个新细分场景](docs/implementation/2026-09-20-checkout-quote-acceptance.md)。仅成交前置校验，不代表已经创建订单、预留库存或允许扣款。
- 内部操作台账：精确权限、永久幂等、原子入队、租约 token/generation 和 SQL-only worker 写边界通过；[合同](contracts/external-operation-v1.md)、[106 项后端验收](docs/implementation/2026-09-20-external-operation-acceptance.md)。
- 内部 River 执行器：真实任务消费、并发去重、UNKNOWN 仅查询、进程 SIGKILL 后 JobRescuer 恢复通过；[合同](contracts/external-dispatcher-v1.md)、[123 项测试及浏览器回归](docs/implementation/2026-09-20-dispatcher-acceptance.md)。远端使用显式 mock，生产 provider 适配器尚未实现；本地下单到期任务见下文。
- 商家金流/物流设置：按用户截图区分账户连接、方法启用与买家展示；选店不要求固定承运商。[A横向步骤＋右侧状态](.impeccable/merchant-settings-brief.md)已实现本地账户登记、市场/方式发现、禁用收款草稿及商家自行安排配送；[308项后端、3项真实浏览器链及独立复核证据](docs/implementation/2026-09-24-merchant-settings-wizard-acceptance.md)。三语、手机、崩溃重试、并发修改、账户绑定回填已验收；真实第三方接通和生产资格仍 NOT_RUN。
- 商家自有账户凭据：PAYUNi 分环境登记、AES-GCM 加密存储、不可变密钥版本、HMAC 幂等与正常轮换；新增四条账户 HTTP、安全 BFF 和默认关闭的环境装配，见[HTTP合同](contracts/merchant-account-http-v1.md)、[302项回归及浏览器证据](docs/implementation/2026-09-24-merchant-account-http-acceptance.md)、[部署配置边界](docs/implementation/account-credential-configuration.md)。保存后仍为 `CONFIGURED_UNVERIFIED`，不是已授权、已收款或可启用的支付方式；自助 UI、真实密钥托管部署和供应商验证仍待验收。
- 支付方式配置：PAYUNi 五类方式的三语名称、展示、排序、金额限制及自有账户关联已有版本化配置、诊断与本地设置向导；[合同](contracts/payment-methods-v1.md)、[内部配置验收边界](docs/implementation/2026-09-24-payment-methods-acceptance.md)。当前商家仅可保存禁用草稿，诊断明确 `ADAPTER_UNAVAILABLE`，不是已接通真实收款；内部支付链证据与真实供应商准入分开。
- PAYUNi wire 协议：标准库实现五种 UPP 托管付款表单、通知验签与单次交易查询；[合同](contracts/payuni-wire-v1.md)、[验收边界](docs/implementation/2026-09-24-payuni-wire-acceptance.md)。查询与信用卡表单现已由下述内部支付链调用；通知入账、真实商家沙箱或公开支付页面仍未接通，不解除上述启用限制。
- Stripe 操作端沙盒：用户授权的 HKD10 测试链接已创建，API持久回读与独立收银台显示通过；[证据及限制](docs/implementation/2026-09-25-stripe-sandbox-link-verification.md)。这是 Codex 插件核验，不是服务器 Stripe 适配器接入；测试付款、订单回调及对账尚未执行，正式账户未改动。
- 商品自动收款入口：已纳入[架构及 PE01–PE10 合同](contracts/product-payment-entry-v1.md)。购买URL、B商品直购及未付款订单已有下述验收；按原订单生成托管付款表单的内部能力也已通过，但公开付款页面与真实收款仍待接通。无需逐商品手工建 PSP 链接，不能用上述沙盒链接替代完整验收。
- 买家商品数据前置：私有 `GET /v1/buyer/catalog` 已支持当前有效商品/SKU/现价、店铺隔离、受限查询和分页；[345 项回归与独立审查](docs/implementation/2026-09-25-buyer-catalog-discovery-acceptance.md)通过。这不是公开商城页面，也未生成分享链接或支付页。
- 买家结账选项：私有 `GET /v1/buyer/checkout-options` 提供当前市场、地区、三语配送名称及下单所需版本；[353 项回归及 HTTP 选项→报价→宅配订单证据](docs/implementation/2026-09-25-buyer-checkout-options-acceptance.md)。复用现有结账权限与规则，不保证库存或承运资格；公开页面、可信超商选店和支付页仍待完成。
- 买家会话重试前置：可信 BFF 用同一随机凭证调用私有 `POST /v1/buyer/session/bootstrap`，响应丢失或实例并发时保留同一身份、购物车及原期限；[17 项定向、361 项全量 race/vet 证据](docs/implementation/2026-09-25-buyer-session-registration-acceptance.md)。该历史切片仅为内部前置，后续浏览器传输见下一项。
- 买家浏览器与B商品直购：`apps/storefront` 的默认关闭BFF、HttpOnly/context-CSRF、跨标签协调、持久退出与共享配额已有[传输验收](docs/implementation/2026-09-25-buyer-browser-bff-acceptance.md)。真实页面现覆盖商品→购物车→报价→宅配地址确认→未付款订单，以及显式继续选购和当前身份历史订单；[34项前端、382项后端/PG/race/vet、23场景订单/历史浏览器证据](docs/implementation/2026-09-25-buyer-history-progress.md)。三语保持身份及币种；独立历史视觉增量结论为有限范围 `ship`。支付UI、过期身份安全恢复、联合发布和真实DNS/TLS仍待完成。
- 商家商品购买入口后端：只读返回已授权商品的三语购买URL，不增加PSP操作或修改原商品/SKU收据；[真实PG/HTTP与375项全量race/vet记录](docs/implementation/2026-09-25-merchant-purchase-entry-acceptance.md)。configured仅说明当前数据库发布配置，复制/打开UI必须与真实B买家路由一起验收；不是已完成自动收款。
- 内部发起支付：真实买家权限、冻结金额/账户版本、订单与待支付库存/attempt/查询意图/回执/队列同事务；[合同](contracts/payment-start-v1.md)、[246项真实PG/race/vet及失败修正证据](docs/implementation/2026-09-24-payment-start-acceptance.md)。仅信用卡PROVIDER_MOCK内核；后续托管表单和查询执行见下项，真实账户资格、通知入账/退款/对账及公开付款页面仍待完成。
- 订单绑定托管付款内核：沿用原订单/支付尝试冻结金额、账户和凭据版本，同事务保存一份表单；独立签名角色、一次性交付、失去COMMIT应答不补发及过期/撤权防护通过。[HP01–07合同](contracts/payment-hosted-v1.md)、[404项全量真实PG/race/vet及独立复核](docs/implementation/2026-09-25-payment-hosted-acceptance.md)。没有供应商网络请求；公开HTTP/BFF/付款界面由下列后续增量记录，不代表支付成功或允许商家启用。
- 买家付款私有接口：默认关闭的独立角色装配、只读付款状态／方式、按原订单准备及一次性交付，已通过[421项全量真实PG/race/vet及独立复核](docs/implementation/2026-09-25-buyer-payment-http-acceptance.md)。包含三语实际HTTP、配置变化、等待锁期间过期及连接回收；不代表公开付款UI、真实供应商准入或商家收款开关已开放。
- 买家付款公开传输：已接入原 Next BFF，严格校验付款状态、准备回执与一次性表单；保留原会话／跨站防护，handoff 失败不自动重试。[BPT01–06证据](docs/implementation/2026-09-25-buyer-payment-public-acceptance.md)记录45项Node、类型检查／构建、未变更后端421项PG回归及独立复核；公开UI与浏览器证据见下一条，供应商收款仍未验收。
- B 买家付款界面：当前／历史订单共用单次付款入口，支持原请求恢复及交付不确定后的只读查询。[BPU01–04证据](docs/implementation/2026-09-25-buyer-payment-ui-acceptance.md)记录55项Node、11项实际Next→Go→PG浏览器检查（含移动触控和身份切换）、23项原订单回归、中立返回页安全响应头及原生模拟PSP提交；独立视觉审查提出的币种／状态标签两项修正均已复核通过。真实供应商／部署验收及商家收款开关仍未开放。
- 付款后台运行：独立默认关闭进程、三环境队列、旧生产者提交时路由及启动预检；[PW01–05验收](docs/implementation/2026-09-25-payment-worker-acceptance.md)记录当时433项真实PG/race/vet、55项Node和11项付款浏览器回归。实际River完成双租户模拟查询入账、去重重启、强杀回收及正常停止；不消费到期／外部操作默认队列。[运行与升级说明](docs/implementation/payment-worker-runtime.md)保留恢复时限和生产停止线；真实PSP和生产部署仍未完成，到期运行入口见下一项。
- 订单到期后台运行：独立默认关闭进程、固定到期队列、旧任务原子迁移和提交时路由，复用原库存事务。[EW01–05验收](docs/implementation/2026-09-25-expiry-worker-acceptance.md)记录最终444项真实PG/race/vet、55项Node、11项付款及23项订单浏览器回归；涵盖双租户释放、重复投递、实际付款锁竞争、SIGTERM清理和SIGKILL后River回收。付款／到期共用启动停止逻辑，无新依赖；保留并修正测试误报证据。[部署与回退说明](docs/implementation/expiry-worker-runtime.md)明确真实恢复时限、失败任务巡检及生产部署仍待验收。
- 商家订单列表／详情后端：独立 `orders:read` 权限、跨店隔离、冻结商品／收货快照和等待后权限重验。[MOR01–06 验收](docs/implementation/2026-09-25-merchant-orders-acceptance.md)记录最终 458 项真实 PG/race/vet 回归、12 种观察到阻塞后的权限验证及独立复核；无新依赖、不改原成员权限。尚不含商家订单页面、发货或生产部署。
- 商家订单浏览器接线：[MBT01–04 实链验收](docs/implementation/2026-09-25-merchant-orders-bff-acceptance.md)覆盖真实登录 Cookie → Next → Go → 隔离 PG、严格原始查询校验、跨店/测试模式隔离和禁止缓存。修复了 Next 在业务校验前重写查询的问题，旧登录/设置/三语言商品入口已回归；这里只是读取接线，不是新订单页面或正式收款上线。
- 支付查询与可信报告：精确租约读取历史凭据、实际 River 查询执行、验签后报告与 UNKNOWN 完成同事务、并发去重及持久化查询时限；[合同](contracts/payment-query-v1.md)、[查询阶段验收](docs/implementation/2026-09-24-payment-query-acceptance.md)。查询成功不是收款成功；后续财务判断由独立本地入账流程处理。
- 信用卡入账与库存承诺：完整请款证据、不可变财务事实、预留转待履约库存、订单确认和耐久商家待办同事务；逆序授权不会误锁履约，退款／晚款异常转粘性复核；[合同](contracts/payment-capture-v1.md)、[284项真实PG/race/vet证据](docs/implementation/2026-09-24-payment-capture-acceptance.md)。供应商仍是签名报文模拟，无真实PSP调用；不等于银行结算、退款功能或上线准入，不开放商家收款开关。
- 商家配送配置内部内核：版本化启用／展示、独立运费、权限隔离、幂等与回滚已通过；[138 项后端回归及浏览器身份链证据](docs/implementation/2026-09-24-delivery-service-acceptance.md)。A向导已覆盖显式计价和商家自行安排配送；API模式仍仅禁用草稿，不代表承运商已接通，也不承诺任何物流都能投递超商。
- 配送仓库配置与纯分配算法：逐配送方式保存仓库优先级、按可用库存拆仓，缺货不输出部分计划；[150 项回归、随机输入及浏览器兼容证据](docs/implementation/2026-09-24-delivery-allocation-acceptance.md)。后续已由下述内部 checkout 在余额行锁内消费；该历史纯函数验收本身不是并发防超卖证据。
- 买家收货与门市来源：owner 隔离快照、可信门市引用、改选 CAS、回执隐私和锁等待后重验；[164 项真实 PG/race/vet 回归及浏览器兼容](docs/implementation/2026-09-24-buyer-destination-acceptance.md)。门市当前仅人工核验来源，不是官方目录／承运商接通；商家自助页面与完整支付链仍待实现。
- 买家下单与到期释放内核：独立 checkout 权限、同事务 DRAFT 订单／HELD 预留／台账／River 任务、永久幂等、锁等待后重验和重复释放防护；[合同](contracts/buyer-checkout-v1.md)、[历史内核验收](docs/implementation/2026-09-24-buyer-checkout-acceptance.md)。默认关闭的结账HTTP/UI已在上述B商品直购切片验收；独立付款／到期worker见上方，失败任务巡检、真实供应商和生产部署仍待完成。

下文保留原设计包的基线说明；`MANIFEST.sha256` 对应原包，不能拿来验证新增实现。商品/结账、四域会话业务、台湾超商/跨境物流、Meta/支付/直播和三端 UI 仍需继续实现与独立验收。全局 G01–G15 未宣称通过。

- Meta 入站协议切片：原始字节验签、严格 JSON／Unicode、完整批次归一化及同步提交回调后 ACK，已通过[52 项本地 race 回归与独立审查](docs/implementation/2026-09-26-meta-webhook-protocol-acceptance.md)。此早期协议门禁不包含持久化，后续入库验收见下；公开路由与真实 Meta 资格未启用，不等于 T07 完成。
- Meta 入库：原始报文交接及 AES-GCM 前置组件已有[独立验收](docs/implementation/2026-09-26-meta-inbox-primitives-acceptance.md)。SQL/Go 原子接收、可信租户路由、专用 River 队列和受控密文清理通过[真实 PG 本地验收](docs/implementation/2026-09-26-meta-inbox-durability-acceptance.md)：冻结产品源码全量 507 项通过，再合入 IG 测试后专项 20 项通过，race/vet 均通过。消费者、OAuth 资格和公开回调尚未启用。

## 原设计包基线

核验基准：2026-09-08。版本1.0。**架构设计与规格模型，不是已经完成的SaaS。**

主文档是 [`架构.md`](架构.md)。交付明确选择Go业务后端、PostgreSQL+River、TypeScript网页层和托管LiveKit媒体，不宣称语言性能或外部账号能力已验证。

## 阅读/启动

先读架构第0–3节，理解范围、选型和外部阻塞；第26–28节规定Codex分工、依赖与验收。将本目录放在独立项目仓库根目录，审查并信任项目配置后，把 `BOOTSTRAP_PROMPT.md` 交给Codex。不要不经核对覆盖既有项目配置。

本包带7个自定义子Agent角色候选，但当前没有运行真实Codex。默认继承用户当前模型，最多4个子Agent、2个并行写任务；T00先验证角色/模型/沙箱/工作树。

## 目录

|文件|作用|
|---|---|
|架构.md|完整30节（0–29）架构、状态机、风险和原始来源|
|AGENTS.md / .codex/|短指令、主配置及7种角色|
|contracts/requirements.json / invariants.json|22项需求、24项关键不变量|
|contracts/gates.json / tasks.json / risks.json|15个产品门禁、23个任务依赖包、32个开放风险|
|contracts/core-openapi.json|8个核心HTTP操作的OpenAPI 3.1草案；T01补齐全量API|
|contracts/*schema.json|事件及任务结果JSON Schema；不是身份校验和真实执行证明|
|contracts/sources.json / versions.candidate.json|50个原始来源及候选版本；T00仍需正式pin|
|experiments/PLAN.md|安全负对照、优化消融、语言和Agent并行实验方案|
|experiments/spec_models.py / results/|本次实际运行的规格模型及原始结果|
|scripts/check_packet.py|包结构、引用、DAG、schema、结果完整性校验|
|BOOTSTRAP_PROMPT.md|给Codex的第一条执行指令|

## 可在本包运行的检查

需要Python 3.11或更新版本；仅标准库。以下命令不会启动产品或付费外部API：

```sh
python3 experiments/spec_models.py --out experiments/results
python3 scripts/check_packet.py --delivery-baseline
```

结果仅属于SPEC_MODEL/PACKET_STRUCTURE。模型中12项基线通过、12项删除控制的变体被捕获；一项合成调度消融不增加总吞吐，只改善构造场景里的小租户等待。**不是实际性能压测、完整安全证明、真实API测试或Codex并行试验。**

全部产品门禁仍为NOT_RUN_PRODUCT。语言比较、PG/River压测、平台审核、真实支付/物流/媒体以及1/2/4 Agent实验均尚未执行。

生成文件的SHA-256清单见 `MANIFEST.sha256`；它只能检查文件变化，不能证明结论真实或生产正确。

开发开始后用`python3 scripts/check_packet.py`核对包结构；`--delivery-baseline`专用于核对这次交付没有伪造产品执行状态。两者均不验证产品门禁结果的真实性。
