# 直播电商 SaaS 架构与 Codex 启动包

## 2026-09-20 实施入口

已开始开发，当前已验收 **Go/PostgreSQL 基础、商品库存台账、商家登录/首店 C 分步向导、内部购物车报价、操作台账及下单预留／到期释放切片，不是完整 SaaS 或生产上线版本**。真实登录提供商、买家公开入口、支付及其余业务模块仍未交付。

- 当前实现：[foundation 合同](contracts/foundation-v1.md)、[基础 OpenAPI](contracts/foundation-openapi.json)、[实施说明](docs/implementation/2026-09-20-kickoff.md)。
- 本地一键验收：`bash scripts/dev/test-local.sh`。需要 Docker、Go 启动器和已下载的固定 PG18.6 镜像（镜像 digest 见脚本）；会创建本任务临时数据库，退出自动移除，不读取现有 `DATABASE_URL`。
- 仅编译/单测：`GOTOOLCHAIN=go1.27.1 go test -race ./...`。未配置隔离数据库时，集成测试明确 SKIP，不能视为数据库通过。
- 服务入口：`GOTOOLCHAIN=go1.27.1 go run ./cmd/api`。需要已迁移数据库和 **commerce_runtime 成员、非 owner 的独立登录**，通过环境变量 `DATABASE_URL` 传入。默认仅监听 `127.0.0.1:8080`；程序不会自动迁移、生成用户或放宽授权。
- GitHub 检查配置已加入 `.github/workflows/foundation.yml`，但尚无远端仓库或真实 CI 执行回执。
- 身份浏览器链：`bash scripts/dev/test-local.sh --browser-identity`。真实隔离 PG + Next/Go，外部 IdP 为签名 mock；[合同](contracts/merchant-browser-auth-v1.md)、[传输验收](docs/implementation/2026-09-20-browser-identity-acceptance.md)。用户已批准 C 分步向导，[界面与最终浏览器验收](docs/implementation/2026-09-20-entry-wizard-acceptance.md)；身份开关仍默认关闭，未接通生产 IdP。
- 买家匿名凭证：独立 SQL 角色、hash-only、过期/撤销与并发隔离已通过真实 PG 验收；[合同](contracts/buyer-capability-v1.md)、[证据](docs/implementation/2026-09-20-buyer-capability-acceptance.md)。仅内部权限内核，尚未开放买家 HTTP/购物车/结账。
- 内部购物车/报价：owner 隔离、版本化市场计价、不可变快照、并发与回滚通过；[合同](contracts/cart-quote-v1.md)、[96 项后端验收](docs/implementation/2026-09-20-cart-quote-acceptance.md)。未开放 HTTP，不创建订单或扣库存。
- 下单前报价重验：锁定当前购物车/政策/商品、复用原计价器核对金额，在锁等待后检查 DB 时间；[合同](contracts/checkout-quote-validation-v1.md)、[128 项回归与 40 个新细分场景](docs/implementation/2026-09-20-checkout-quote-acceptance.md)。仅成交前置校验，不代表已经创建订单、预留库存或允许扣款。
- 内部操作台账：精确权限、永久幂等、原子入队、租约 token/generation 和 SQL-only worker 写边界通过；[合同](contracts/external-operation-v1.md)、[106 项后端验收](docs/implementation/2026-09-20-external-operation-acceptance.md)。
- 内部 River 执行器：真实任务消费、并发去重、UNKNOWN 仅查询、进程 SIGKILL 后 JobRescuer 恢复通过；[合同](contracts/external-dispatcher-v1.md)、[123 项测试及浏览器回归](docs/implementation/2026-09-20-dispatcher-acceptance.md)。远端使用显式 mock，生产 provider 适配器尚未实现；本地下单到期任务见下文。
- 商家金流/物流设置：[合同](contracts/merchant-service-settings-v1.md)已按用户截图补充账户连接、方法启用与买家展示分离；支持商家自带物流，选店不要求固定承运商。当前为设计边界，UI/API及第三方接通仍待实现和逐项验收。
- 商家自有账户凭据：PAYUNi 分环境登记、AES-GCM 加密存储、不可变密钥版本、HMAC 幂等与正常轮换已有内部实现；[合同](contracts/merchant-accounts-v1.md)、[验收边界](docs/implementation/2026-09-24-merchant-accounts-acceptance.md)。保存后明确为 `CONFIGURED_UNVERIFIED`，不是已授权、已收款或可启用的支付方式；自助 HTTP/UI、生产密钥装配和供应商验证仍待实现。
- 支付方式配置：PAYUNi 五类方式的三语名称、展示、排序、金额限制及自有账户关联已有内部版本化配置与诊断；[合同](contracts/payment-methods-v1.md)、[验收边界](docs/implementation/2026-09-24-payment-methods-acceptance.md)。当前仅可保存禁用草稿，诊断明确 `ADAPTER_UNAVAILABLE`，不是已接通收款；实际支付链及设置页面仍待实现。
- PAYUNi wire 协议：标准库实现五种 UPP 托管付款表单、通知验签与单次交易查询；[合同](contracts/payuni-wire-v1.md)、[验收边界](docs/implementation/2026-09-24-payuni-wire-acceptance.md)。仅协议模拟与官方加密向量验证，未接入 StartPayment、通知入账、商家沙箱或支付页面；不解除上述启用限制。
- 商家配送配置内部内核：版本化启用／展示、独立运费、权限隔离、幂等与回滚已通过；[138 项后端回归及浏览器身份链证据](docs/implementation/2026-09-24-delivery-service-acceptance.md)。API 模式仍仅禁用草稿，不代表承运商已接通；公开设置页面和支付待实现。
- 配送仓库配置与纯分配算法：逐配送方式保存仓库优先级、按可用库存拆仓，缺货不输出部分计划；[150 项回归、随机输入及浏览器兼容证据](docs/implementation/2026-09-24-delivery-allocation-acceptance.md)。后续已由下述内部 checkout 在余额行锁内消费；该历史纯函数验收本身不是并发防超卖证据。
- 买家收货与门市来源：owner 隔离快照、可信门市引用、改选 CAS、回执隐私和锁等待后重验；[164 项真实 PG/race/vet 回归及浏览器兼容](docs/implementation/2026-09-24-buyer-destination-acceptance.md)。门市当前仅人工核验来源，不是官方目录／承运商接通；第三方账户与支付设置仍待实现。
- 内部买家下单与到期释放：独立 checkout 权限、同事务 DRAFT 订单／HELD 预留／台账／River 任务、永久幂等、锁等待后重验和重复释放防护；[合同](contracts/buyer-checkout-v1.md)、[187 项真实 PG/race/vet 及浏览器兼容证据](docs/implementation/2026-09-24-buyer-checkout-acceptance.md)。仅内部 Go/SQL 内核，没有公开结账 HTTP/UI、StartPayment、生产 worker 装配或第三方扣款。

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
