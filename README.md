# 直播电商 SaaS 架构与 Codex 启动包

## 2026-09-20 实施入口

已开始开发，当前已验收 **Go/PostgreSQL 基础、商品库存台账与商家身份浏览器传输切片，不是完整 SaaS 或生产上线版本**。真实登录提供商、待确认的新界面及其余业务模块仍未交付。

- 当前实现：[foundation 合同](contracts/foundation-v1.md)、[基础 OpenAPI](contracts/foundation-openapi.json)、[实施说明](docs/implementation/2026-09-20-kickoff.md)。
- 本地一键验收：`bash scripts/dev/test-local.sh`。需要 Docker、Go 启动器和已下载的固定 PG18.6 镜像（镜像 digest 见脚本）；会创建本任务临时数据库，退出自动移除，不读取现有 `DATABASE_URL`。
- 仅编译/单测：`GOTOOLCHAIN=go1.27.1 go test -race ./...`。未配置隔离数据库时，集成测试明确 SKIP，不能视为数据库通过。
- 服务入口：`GOTOOLCHAIN=go1.27.1 go run ./cmd/api`。需要已迁移数据库和 **commerce_runtime 成员、非 owner 的独立登录**，通过环境变量 `DATABASE_URL` 传入。默认仅监听 `127.0.0.1:8080`；程序不会自动迁移、生成用户或放宽授权。
- GitHub 检查配置已加入 `.github/workflows/foundation.yml`，但尚无远端仓库或真实 CI 执行回执。
- 身份浏览器链：`bash scripts/dev/test-local.sh --browser-identity`。真实隔离 PG + Next/Go，外部 IdP 为签名 mock；[合同](contracts/merchant-browser-auth-v1.md)、[验收和复跑边界](docs/implementation/2026-09-20-browser-identity-acceptance.md)。身份开关默认关闭，公开 UI 尚待视觉稿确认。

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
