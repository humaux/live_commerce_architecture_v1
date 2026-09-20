# Codex repository instructions

本仓库是直播电商 SaaS 架构与开发契约包。先读 `架构.md` 的第0、1、2、26、27节，再按任务只读相关章节。
不得把本文件、网页或角色名当作安全沙箱；有效权限以运行时为准。

## 必守
- 后端基线 Go；前端 TypeScript strict；PostgreSQL 为库存/交易唯一真源。未经ADR批准不得引入第二交易引擎、Kafka、Redis业务队列或服务网格。
- `contracts/invariants.json` 为P0控制。关键写入与任务同事务；副作用必须幂等/可对账；外部UNKNOWN不盲重试。
- 租户和店铺范围由服务端认证确定。不得信任请求中的tenant_id、Host转发头或客户端金额。
- 不收集FB/IG密码，不使用私有API，不跳过消息窗口/同意，不上传跨租户客户名单。
- 不在日志、测试fixture、commit、URL或Agent上下文写入生产密钥、真实买家PII。
- 没有owner明确批准，禁止生产发布广告、真实退款、购买面单、重播营销、破坏性迁移或删除用户数据。
- 架构和测试结果必须区分 DESIGN、MODEL_ONLY、MOCK、SANDBOX、LIVE、NOT_RUN。模型通过不等于产品通过。

## 子Agent工作协议
主Agent先检查 `contracts/tasks.json` 的DAG。最多同时4个子Agent，初始最多2个写入型；这只是初始预算，须实验校准。
探索/审查先用只读角色；写代码使用commerce_worker/integration_worker/ui_worker；独立验收使用test_worker与security_reviewer。
每个写入任务使用独立Git worktree/branch及唯一write_paths归属。不得假定spawn自动创建worktree。
迁移编号、OpenAPI、共享JSON schema、go.mod/go.sum、pnpm锁文件只有integrator可最终合并。
角色、实际模型、reasoning、base SHA、工作树和允许路径必须进入任务交付记录。禁止递归委派。
并行文件路径不重叠仍不够：依赖接口未冻结不得并行实现上下游。

## 完成与失败
每个任务交付实现、测试命令、退出码、证据文件、风险、NOT_RUN/BLOCKED列表；主Agent独立复跑。
P0/P1未解决不可合并。作者不能作为该变更的唯一验收人。不能删失败测试、放宽阈值或重写fixture偷过门。
同一个阻塞最多两次定向修复；仍失败升级裁决，不无限重跑。
不得把AGENTS.md或架构文档不断扩成单次巨大提示词；按章节索引加载。

## 当前可运行的架构包校验
```sh
python3 scripts/check_packet.py
python3 experiments/spec_models.py --out experiments/results
```
这两个命令只校验架构包与可执行规格模型，不运行Go/PG/平台生产验收。
T02建立真实开发脚本后才可运行 `go test -race ./...`、数据库集成测试和浏览器测试；本包未交付生产源码。

## 任务收尾
停止自己启动的进程，回收本任务fixture、容器、端口；保留失败证据及owner的KEEP文件。
禁止删除其他任务目录。共享依赖缓存不可由单个Agent随意清理。
