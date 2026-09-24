# 直播电商 SaaS 架构与 Codex 启动包

## 2026-09-20 实施入口

已开始开发，当前已验收 **Go/PostgreSQL 基础、商品库存台账、商家登录/首店 C 分步向导、买家浏览器传输及购物车报价、操作台账、下单预留／到期释放及内部发起支付事务，不是完整 SaaS 或生产上线版本**。真实登录提供商、买家商品/支付页面、真实收款及其余业务模块仍未交付。

- 当前实现：[foundation 合同](contracts/foundation-v1.md)、[基础 OpenAPI](contracts/foundation-openapi.json)、[实施说明](docs/implementation/2026-09-20-kickoff.md)。
- 本地一键验收：`bash scripts/dev/test-local.sh`。需要 Docker、Go 启动器和已下载的固定 PG18.6 镜像（镜像 digest 见脚本）；会创建本任务临时数据库，退出自动移除，不读取现有 `DATABASE_URL`。
- 仅编译/单测：`GOTOOLCHAIN=go1.27.1 go test -race ./...`。未配置隔离数据库时，集成测试明确 SKIP，不能视为数据库通过。
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
- PAYUNi wire 协议：标准库实现五种 UPP 托管付款表单、通知验签与单次交易查询；[合同](contracts/payuni-wire-v1.md)、[验收边界](docs/implementation/2026-09-24-payuni-wire-acceptance.md)。协议模拟与官方加密向量验证通过，查询现已由下述内部 worker 调用；未接通托管付款表单、通知入账、商家沙箱或支付页面，不解除上述启用限制。
- Stripe 操作端沙盒：用户授权的 HKD10 测试链接已创建，API持久回读与独立收银台显示通过；[证据及限制](docs/implementation/2026-09-25-stripe-sandbox-link-verification.md)。这是 Codex 插件核验，不是服务器 Stripe 适配器接入；测试付款、订单回调及对账尚未执行，正式账户未改动。
- 商品自动收款入口：已纳入[架构及 PE01–PE10 合同](contracts/product-payment-entry-v1.md)。保存价格后自动提供稳定购买入口，确认规格/数量/配送后按订单生成支付页，无需逐商品手工建 PSP 链接；当前为需求/设计，完整功能未实现，不能用上述沙盒链接替代验收。
- 买家商品数据前置：私有 `GET /v1/buyer/catalog` 已支持当前有效商品/SKU/现价、店铺隔离、受限查询和分页；[345 项回归与独立审查](docs/implementation/2026-09-25-buyer-catalog-discovery-acceptance.md)通过。这不是公开商城页面，也未生成分享链接或支付页。
- 买家结账选项：私有 `GET /v1/buyer/checkout-options` 提供当前市场、地区、三语配送名称及下单所需版本；[353 项回归及 HTTP 选项→报价→宅配订单证据](docs/implementation/2026-09-25-buyer-checkout-options-acceptance.md)。复用现有结账权限与规则，不保证库存或承运资格；公开页面、可信超商选店和支付页仍待完成。
- 买家会话重试前置：可信 BFF 用同一随机凭证调用私有 `POST /v1/buyer/session/bootstrap`，响应丢失或实例并发时保留同一身份、购物车及原期限；[17 项定向、361 项全量 race/vet 证据](docs/implementation/2026-09-25-buyer-session-registration-acceptance.md)。该历史切片仅为内部前置，后续浏览器传输见下一项。
- 买家浏览器传输：`apps/storefront` 已有默认关闭的公开BFF、HttpOnly/context-CSRF、跨标签协调、持久退出与共享配额；[366项后端、7项前端单测、11场景真实Chromium→Next→Go→PG验收](docs/implementation/2026-09-25-buyer-browser-bff-acceptance.md)。实际链路到DRAFT订单/唯一库存预留，支持断网和跨实例重试；用户已批准[B商品详情直接选购视觉稿](docs/implementation/2026-09-25-buyer-surface-proposal.md)，但实际买家页面、商品分享路由、真实DNS/TLS与PSP支付页仍待实现。
- 商家商品购买入口后端：只读返回已授权商品的三语购买URL，不增加PSP操作或修改原商品/SKU收据；[真实PG/HTTP与375项全量race/vet记录](docs/implementation/2026-09-25-merchant-purchase-entry-acceptance.md)。configured仅说明当前数据库发布配置，复制/打开UI必须与真实B买家路由一起验收；不是已完成自动收款。
- 内部发起支付：真实买家权限、冻结金额/账户版本、订单与待支付库存/attempt/查询意图/回执/队列同事务；[合同](contracts/payment-start-v1.md)、[246项真实PG/race/vet及失败修正证据](docs/implementation/2026-09-24-payment-start-acceptance.md)。仅信用卡PROVIDER_MOCK内核；后续查询执行见下项，真实账户资格、付款表单、通知入账/退款/对账及公开页面仍待完成。
- 支付查询与可信报告：精确租约读取历史凭据、实际 River 查询执行、验签后报告与 UNKNOWN 完成同事务、并发去重及持久化查询时限；[合同](contracts/payment-query-v1.md)、[查询阶段验收](docs/implementation/2026-09-24-payment-query-acceptance.md)。查询成功不是收款成功；后续财务判断由独立本地入账流程处理。
- 信用卡入账与库存承诺：完整请款证据、不可变财务事实、预留转待履约库存、订单确认和耐久商家待办同事务；逆序授权不会误锁履约，退款／晚款异常转粘性复核；[合同](contracts/payment-capture-v1.md)、[284项真实PG/race/vet证据](docs/implementation/2026-09-24-payment-capture-acceptance.md)。供应商仍是签名报文模拟，无真实PSP调用；不等于银行结算、退款功能或上线准入，不开放商家收款开关。
- 商家配送配置内部内核：版本化启用／展示、独立运费、权限隔离、幂等与回滚已通过；[138 项后端回归及浏览器身份链证据](docs/implementation/2026-09-24-delivery-service-acceptance.md)。A向导已覆盖显式计价和商家自行安排配送；API模式仍仅禁用草稿，不代表承运商已接通，也不承诺任何物流都能投递超商。
- 配送仓库配置与纯分配算法：逐配送方式保存仓库优先级、按可用库存拆仓，缺货不输出部分计划；[150 项回归、随机输入及浏览器兼容证据](docs/implementation/2026-09-24-delivery-allocation-acceptance.md)。后续已由下述内部 checkout 在余额行锁内消费；该历史纯函数验收本身不是并发防超卖证据。
- 买家收货与门市来源：owner 隔离快照、可信门市引用、改选 CAS、回执隐私和锁等待后重验；[164 项真实 PG/race/vet 回归及浏览器兼容](docs/implementation/2026-09-24-buyer-destination-acceptance.md)。门市当前仅人工核验来源，不是官方目录／承运商接通；商家自助页面与完整支付链仍待实现。
- 内部买家下单与到期释放：独立 checkout 权限、同事务 DRAFT 订单／HELD 预留／台账／River 任务、永久幂等、锁等待后重验和重复释放防护；[合同](contracts/buyer-checkout-v1.md)、[187 项真实 PG/race/vet 及浏览器兼容证据](docs/implementation/2026-09-24-buyer-checkout-acceptance.md)。后续内部StartPayment见上方；仍没有公开结账HTTP/UI、生产worker装配或第三方扣款。

下文保留原设计包的基线说明；`MANIFEST.sha256` 对应原包，不能拿来验证新增实现。商品/结账、四域会话业务、台湾超商/跨境物流、Meta/支付/直播和三端 UI 仍需继续实现与独立验收。全局 G01–G15 未宣称通过。

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
