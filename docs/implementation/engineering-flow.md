# 工程交付流（可复用）

本流程服务于可部署的 live-commerce SaaS 开发；它只描述本地/隔离环境，不能授权客户生产写入。

## 交付顺序

1. **需求**：写清用户结果、边界、风险、证据等级（DESIGN/MODEL_ONLY/MOCK/SANDBOX/LIVE/NOT_RUN），并从 `contracts/tasks.json` 核对依赖。
2. **契约冻结**：先冻结需求、OpenAPI/事件/schema、不变量、错误语义、权限、幂等键和 gate。共享契约、迁移、锁文件只由 integrator 写；未冻结接口不得并行实现。
3. **范围与隔离**：主Agent分发 `task_id/base_sha/branch/worktree/write_paths/depends_on/budget/stop_conditions`。每个写任务独立 worktree；初始最多 2 个写入 agent，作者不作唯一验收人。
4. **实现**：先读调用图和实际依赖，复用现有代码/标准库；每次只改声明路径。事务、权限、外部副作用和不可信输入按冻结不变量实现。
5. **独立测试与审查**：作者提交命令、退出码、证据、失败与 NOT_RUN；test/security reviewer 独立运行并审 diff。P0/P1 未解决不得合并。
6. **root replay**：主Agent在整合基线重读 diff，独立重跑关键 gate、故障反例和安全检查；失败最多两次定向修复，仍失败升级裁决。
7. **证据**：保存脱敏命令回执、测试结果、版本/镜像摘要、source hash、资源 postflight；PASS 必须与实际环境绑定，NOT_RUN/BLOCKED_EXTERNAL 保留。
8. **合并**：integrator 按 DAG 合并，先契约/迁移/依赖再实现；合并后再跑 root replay。部署前另有备份、审批、回滚和 live readback gate。

## 按风险选择 gate

|风险|最小验收|
|---|---|
|纯函数/DTO|unit + vet；边界和错误分支|
|解析、授权、金额、幂等|unit + fuzz/属性反例；禁止秘密、PII、客户端权限/金额替代服务端事实|
|PG/RLS/事务/并发|隔离真实 PG；双租户/双店、撤权、FK、回滚/panic/cancel、连接复用；必要时 race|
|浏览器/UI|独立浏览器 smoke、权限/错误/不可信 header 反例；无真实账号写入|
|provider/媒体/支付/物流|mock 契约→sandbox 回执→获授权的 live probe；UNKNOWN 不盲重试，幂等和对账可见|
|部署/迁移|固定摘要、启动/ready、备份恢复演练、迁移重入/锁竞争、回滚和公开/API/health 分层 readback|

## Agent 与 token 经济

机械检索/小改动用满足 gate 的最低成本模型；交易、RLS、竞态和独立审查升级到已验证强推理档。预算记录真实消耗，不把模型名或并行数写成性能结论。并行只用于契约已冻结且路径不重叠的工作；同一数据库/fixture/端口须命名空间隔离。连续两次同一阻塞失败即停线升级，不无限重跑。

## 代码注释约定

只写能防止错误维护的注释：`WHY:` 说明选择及不可替代原因；`TX:` 说明事务边界、提交/回滚；`LOCK:` 说明锁顺序、持有范围、超时；`INV:` 说明不变量；`TIMEOUT:` 说明预算与清理；`PROVIDER:` 说明外部 UNKNOWN、幂等和能力假设。不要逐行复述代码；刻意简化写 `ponytail:` 并说明升级信号。
