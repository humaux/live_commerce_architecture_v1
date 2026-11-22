# 交付流程

本文件是仓库的现行执行指南。范围与授权以本轮负责人指令和派发卡为准，业务不变量见 [P0 契约](../../contracts/invariants.json)，门禁以 [GATES](GATES.md)、仓库检查器和 CI 工作流为准。先从 [AGENTS](../../AGENTS.md) 定位架构章节，再按当前任务读契约。遇到指令与有效门禁冲突时，保留门禁并报负责人裁决。查不到的状态记 `UNKNOWN`。

## 1. Release plan

现行任务依赖查 [任务 DAG](../../contracts/tasks.json)，实现和验收状态分别核对固定 SHA、CI 回执和任务证据。旧交付报告和 `units/` 卡片可用于追溯已冻结契约、裁决与验收判据。其中的厂商岗位、会话路由、迁移号段、执行状态和操作授权只适用于当时的任务。

历史索引：R1 是 Stripe 测试支付、退款、手工履约、Meta 评论认领与成交闭环的首批交付。R1-8 指发布检查和部署 smoke，入口是 [release-gate.sh](../../scripts/dev/release-gate.sh)。R2 规划物流、客户同意、账单和运营界面。R3 的 T15/T16 指广告和归因，T19/T22 指基准与试点。完整候选的要求见 [架构 §28](../../架构.md#section-28) 和任务 DAG。Meta App Review、商家与平台资质以及生产环境批准由负责人确认。

## 2. Per-unit software-engineering flow

1. **定范围。** 每卡绑定宪章条目、固定 base SHA、分支和 worktree、允许写入路径及依赖。卡片指明契约章节、可判定的验收条件、验证入口、授权与轮次上限。界面入口先查 [Feature Map](../FEATURE_MAP.md)。按改动面定位调用者和入口，优先复用已有实现。
2. **冻结接口。** 涉及 API、事件、共享 schema 或 SQL 签名时，先按获准范围写契约，经过独立评审并由 integrator 裁决、冻结。上下游即使文件路径互斥，也须等共同接口冻结。收紧契约规则时查构造该载荷的客户端，并在合入前告知在途调用方。
3. **实现。** 按已冻结契约做最小变更。新服务、路由和 worker 要接入实际进程，并用测试覆盖装配。HTTP 路由变更用无数据库的整路由器构建测试发现冲突。BFF 测试走真实 handler、`Request` 和共享认证、Cookie、CSRF、错误响应 helper。测试可以替换上游网络边界。
4. **验证。** 修 bug 先留真实失败证据。新门禁保留「注错→红→修→绿」。本地优先使用 [verify](../../scripts/dev/verify) 跑适用的 DB-free smoke，需要真实 PostgreSQL 时走 `bash scripts/dev/test-focused.sh '<regex>'`。全量 foundation、浏览器模式、点击扫测、视觉 lint 和重型矩阵在 CI 跑。脚本与套件入口见 [GATES](GATES.md)。UI 验收按下文执行。
5. **独立审查。** 主控核查 diff、白名单与原始证据，并独立复跑关键检查。CI 通过后，由异家族非作者独审，高风险的资金、权限、契约和上线变更做深审。P0/P1 阻断合并，范围外 P2 记入 `review/p2`。同一阻塞最多两次定向修复，卡片有更严上限时按卡片执行。保留失败测试、阈值和夹具的判别力。
6. **集成。** 通过 PR 进入 `r3/integration`，所需模式由 [pr-modes.mjs](../../scripts/dev/pr-modes.mjs) 和 [gates.yml](../../.github/workflows/gates.yml) 决定。文档 PR 也执行仓库为它选择的 CI 门禁。仅在 `Gates (GitHub runners) / required` 绿且 PR 当前 head SHA 的 `review/independent` 成功后 squash 合并。校准与故障注入走独立的 `workflow_dispatch`。触及 `migrations/`、GRANT、POLICY、SECURITY DEFINER 或 checkout/storefront 运行路径时，用 `bash scripts/dev/release-gate.sh --strict --only G07` 在 CI 完成单元验收，并在合并后对集成 SHA 重跑完整 G07。定向用例仅用于定位。权限变更同时查受影响表和角色的 ACL/schema 清单及旧版升级夹具。
7. **留证并清理。** 按 [§4](#4-evidence-labels-agentsmd) 交付证据并按当前 Humaux 规则更新任务记录。外接数据盘承载 worktree、缓存和测试产物。收尾只停止本任务进程、回收自己的夹具、容器和端口，保留失败原件与 KEEP 文件。确认交回提交和证据后再清理本任务 worktree。

### UI 验收

按任务卡范围和 [GATES](GATES.md) 覆盖每个页面的每个可操作控件。用真实浏览器的 `click`、`fill`、`selectOption` 或键盘操作，断言用户可见结果，涉及持久化则刷新并读回。交付点击台账，逐项记录页面、控件、操作、期望、实际和判定。控件不可用或操作无效按缺陷记录。截图、直接 API、改 DOM、`dispatchEvent` 和 `force: true` 不能代替这条验收路径。允许按 gate 读值、量测和准备夹具。布局验收按批准的设计稿、令牌和真实桌面及移动端截图执行，保留相应视口、语言和浏览器证据。

## 3. Agents and worktree isolation

- 使用 [架构 §26.2](../../架构.md#262-角色和写入边界) 的角色。探索用只读 `platform_explorer`，安全审查用只读 `security_reviewer`。实现按 `commerce_worker`、`integration_worker`、`ui_worker` 的写入边界划分，`test_worker` 独立验收。共享迁移编号、OpenAPI、JSON schema、`go.mod`、`go.sum` 和 pnpm 锁文件由 integrator 最终整合。角色名和 worktree 不授予运行时权限。
- 初始预算为同时最多 4 个子任务、最多 2 个写入任务。主控先查 DAG，可用实测校准预算。写入任务独立 worktree/branch 和互斥写入路径，禁止递归派发。测试按任务隔离数据库、schema、端口和夹具标识。同一台开发机的 PG 测试共用 [test-lock.sh](../../scripts/dev/test-lock.sh) 的串行锁，等待占用释放。
- 用当前 Orca 与 Humaux 编排细则按风险和实际可用能力选车道。历史角色分配和模型候选不提供当前授权。缺合格异家族复核时标 `REVIEW_PENDING`。Orca worker 通过原生 `worker_done` 汇报，主控收结论和证据指针。任务记录注明角色、实际提供方和模型、推理档位、base/head SHA、worktree 和写入路径，未获回执的字段标 `UNKNOWN`。

### Third-party models

历史脚本 [ext-agent.sh](../../scripts/agents/ext-agent.sh) 中的路由与预算只描述该脚本，当前是否可调度由负责人的本轮指令与现行编排规则决定。

## 4. Evidence labels (`AGENTS.md`)

记录 `DESIGN`、`MODEL_ONLY`、`MOCK`、`SANDBOX`、`LIVE` 或 `NOT_RUN`，另列 `BLOCKED` 和风险。真实 PostgreSQL、真实浏览器和模拟供应商可以同时出现，分别注明。供应商 SANDBOX/LIVE 需要对应环境的原始回执。零匹配、取消、跳过或缺日志不能判 PASS，适用例外查 [GATES](GATES.md)。

交付记录包含变更与契约指针、命令、退出码、适用的 PASS/FAIL/SKIP 数量、证据级和未跑项。证据绑当前提交或文件哈希，CI 记录 run ID/链接与实际测试树。长日志和浏览器产物写入仓库认可的忽略运行目录或 CI artifact，清理 worktree 前归档，回报用稳定路径。浏览器路径和 Git 证据保留要求分别由 [check-browser-evidence.mjs](../../scripts/dev/check-browser-evidence.mjs) 和 [check-evidence-retention.mjs](../../scripts/dev/check-evidence-retention.mjs) 执行。

## 5. Source documentation checks

现行检查以代码为准。仓库不新增普遍性的文件头、调用点和导出声明注释要求。注释优先解释类型和代码无法表达的外部约束，并指向契约、ADR 或 Issue；保留适用契约和检查器要求的注释。

- [check-headers.sh](../../scripts/dev/check-headers.sh) 由 [check-gates.sh](../../scripts/dev/check-gates.sh) 与适用 CI 路径调用。基线差异中的新增受检源码（含未跟踪源码）须在前 25 行有 `Purpose:`、`Depends on:`、`Used by:`。默认修改已有受检源码只检查前 8 行中的 leading doc comment。strict 模式对新增和修改均检查三个标签。适用扩展名和测试、生成物、文档等豁免以脚本匹配为准。旧文档中的「每个改动文件都加三行」已由实际检查范围取代。保留并通过现行门禁，策略更改由单独获准的门禁卡处理。
- [check-pkgdocs.sh](../../scripts/dev/check-pkgdocs.sh) 检查 Go 包注释同时包含 `owns` 与 `never`。[depmap.sh](../../scripts/dev/depmap.sh) 生成并校验 [Go 依赖图](../engineering/dependency-map.md)，`check-gates.sh` 核对 [依赖登记](../engineering/dependencies.md) 中 Go 直接依赖和版本。旧卡提到的 [gen-deps.sh](../../scripts/dev/gen-deps.sh) 与其生成物用于查询，CI 的依赖图判据见工作流。
- 继续遵守特定契约的注释与所有权检查。例如 Meta 评论接入的 [MCI schema 测试](../../tests/foundation/meta_claims_intake_schema_test.go) 检查相应表、列与 definer 的 `COMMENT ON`，[MCI guard 测试](../../tests/foundation/meta_claims_intake_guards_test.go) 检查包注释及受检 wire 常量的官方文档 URL。

## 6. Hard boundaries

- 生产上线、真钱、live 密钥、真实广告／退款／客户消息、购买真实面单、重播营销、破坏性迁移及删除真实用户数据，须有本次明确的负责人授权。合并提交、历史试点授权、角色名及测试通过都不能授权生产操作。生产或破坏性写入先核对状态，准备备份、影响范围与回滚点，再等审批。上线条件见 [部署运行手册](../runbooks/deploy.md#10-发布检查清单)。
- 后端基线为 Go，前端为 TypeScript strict，PostgreSQL 是库存和交易唯一真源。第二交易引擎、Kafka、Redis 业务队列及服务网格须有获准 ADR。
- 租户和店铺范围由服务端认证确定，客户端金额和请求提供的作用域不能作为权威。关键写入与任务同事务，外部副作用幂等、可对账，UNKNOWN 不盲重试。Meta 使用官方 API，遵守消息窗口与同意，不收集 FB/IG 密码、不上传跨租户客户名单。
- 密钥通过获准环境注入。生产密钥和真实买家 PII 不进入代码、fixture、日志、提交、URL 或 agent 上下文。支付测试使用测试凭据与供应商 sandbox，Meta 开发验证限已授权测试资产和测试用户。测试夹具使用合成数据并通过 CI 密钥扫描。
