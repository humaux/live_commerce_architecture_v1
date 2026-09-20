# T06 内部 River 执行器验收

日期：2026-09-20。代码与最终测试基线：`8e7c4d8`。
判定：**PASS_BOUNDED**；T06 仍为 IN_PROGRESS，完整 SaaS/全局门禁未验收。
前置：[内部台账 106 项验收](2026-09-20-external-operation-acceptance.md)。
执行契约：[external-dispatcher-v1.md](../../contracts/external-dispatcher-v1.md)。

## 交付与独立性

- Root/integrator：`codex-commerce-build-20260920`，main；合同/迁移/共享角色校验、最终真实 PG/River 故障测试与浏览器回归。
- implementation：`t06_operation_service`，`gpt-5.6-sol` / high，独立工作树 `/Volumes/data/live-commerce-t06-dispatcher-20260920`，分支 `commerce/t06-dispatcher-20260920`；基线 `9b84ef0`，作者 `4100b9f` → main `e8f9990`。仅负责 `internal/integrations/core/dispatcher{,_test}.go`。
- security/business lifecycle review：`t06_schema_review`，`gpt-5.6-sol` / high，只读、非作者；最终 `8e7c4d8` 无未关闭 P0/P1，Humaux 记录 `eaa97b1c-d26d-4a23-a173-f033f6d70674`。Root 实际运行 PG/River，reviewer 核对源码、日志、计数、摘要并独立运行 core race/vet；不把 reviewer 的无数据库单测称作真实 PG。
- 本轮未增加依赖、公共路由、生产 worker CLI 或假 provider。C 向导/UI 未改动。
- 文档独立收口复核：同一只读 reviewer 再查六份相关文档，修正旧 Complete outcomes 描述后 PASS_BOUNDED，无未关闭 P0/P1；记录 `dca54ed3-0f84-462e-a210-4026879916fd`。该复核不冒充第二次真实 PG 运行。

## 验收命令与证据

日志根目录：`/Volumes/data/output/live-commerce-t06-dispatcher-tests/`。
测试只使用独立本地 fixture、受限普通数据库登录、显式 mock 回调/HTTP 服务。

|Gate|实际命令/结果|证据|
|---|---|---|
|真实 PG18.6、Go1.27.1、race + vet|`bash scripts/dev/test-local.sh` exit 0；123 个 top-level PASS，0 FAIL、0 SKIP；foundation 65.119s|`root-dispatcher-final-fixed.log`|
|原始浏览器回归命令|`bash scripts/dev/test-local.sh --browser-identity` exit 0；Next production build、真实 Chromium → Next → Go → PG，RS256 签名 MOCK IdP，1/1 PASS|`root-browser-final.log`|
|实际界面证据|上述浏览器 fixture 生成，未连接客户 IdP|`output/playwright/identity-chain-20260920T103854.001907000/`|
|独立代码复核|PASS_BOUNDED；core race/vet exit 0|上述 Humaux review ID|

SHA-256：

```text
f176e38d129547fad9f14832a85f756b9d70d8d2f21d4a4ea1f064ececf05b07  root-dispatcher-final-fixed.log
21dfd6de9e39a8cf7285cb6d2585bd8f972e52b63b54a1611dd5ff924d93ad4c  root-browser-final.log
```

123 是运行器的顶层测试计数，包含正常模式下无操作的 child-process helper，
不是 123 个业务功能或全局门禁。全套回归包含已验收模块；不重复相加为 229。

归档检查：`python3 scripts/check_packet.py` 返回 PASS_PACKET_STRUCTURE_ONLY；
该命令不替代产品测试。`git diff --check` 通过。最终 dispatcher 测试源已重新
索引为 live-commerce 图谱，并把构造器、权限检查与两项关键故障测试关联到
验收记忆 `a54048ce-c59d-40c9-af69-1f6a5e983c8c`。收尾只读检查未发现
`livecommerce.fixture` 容器、foundation child process 或 3100/3101/18081/19111
测试端口监听；失败日志与浏览器工件保留，没有清理用户目录/共享缓存。

## 已证明的行为

1. Plan 创建真实 River job；Claim 已提交才调用回调。第二连接能 NOWAIT 锁住 operation，证明回调时不持有该事务锁。
2. 精确 provider/action/purpose 路由；未知路由不 claim、不发请求。Check 收到独立 request 副本，不能改写随后 Dispatch 的 frozen actor/asset/body；`lc:<operation UUID>` 稳定。
3. 实际并发重复 job 遇 busy 后持久化 snoozes，只有一次 Dispatch；终态 JobRetry 不重复远端效果。
4. 首次策略拒绝/Check 期间 binding 撤销阻断首次发送；已有 ACK/可能效果后再拒绝保留 UNKNOWN 与 provider reference，不伪装成未发送。
5. 错误、panic、非法 outcome、超时和 ACK 转 query-only Reconcile，不重复 Dispatch。原始错误/请求/凭证不写入 River errors/events。
6. Check、最终数据库检查与一次回调共享 deadline；超时后的 Complete 用新的受限清理 context。父级关闭不分离后台 I/O，保留租约供后续对账。
7. durable generation 预算不依赖不消耗 attempt 的 snooze。`reconcile_budget_exhausted` 必须与 event 持久化后才取消 job；故障注入使最终 event 写失败时，两者回滚且 job 不进入终态。移除故障后 cleanup-only claim 不再调用远端。
8. 真正 `StopAndCancel` 后的新 worker 只对账。两个真实子进程 SIGKILL 场景由真正 River JobRescuer 重新调度原 job：发送前杀死为 0 次 send/1 次 query，模拟远端效果后杀死为 1 次 send/1 次 query。
9. `session_user` 检查阻断 owner 通过 SET ROLE 伪装 worker；AfterConnect fixture 先证明 session_user/current_user 不同再断言拒绝，失败不擅自关闭调用方 pool。

进程故障测试通过父进程外部 mock HTTP 服务保留模拟远端事实，恢复时实际 HTTP GET。
测试只提前老化自己 fixture 的租约与原 River job attempted_at，不改 job state、不插替代 job。
这属于合成时间推进，不声称真实等待租约时长或生产 provider 的持久性。

## 保留的失败与更正

- `authority-policy.log`：测试字段名编译错误，改为 helper 实际字段。
- `authority-policy-fixed.log`：startup-role fixture 本身不能查询，不能证明 validator 正确；改用可先证明身份差异的 AfterConnect fixture，`authority-policy-masked-pool.log` 108 PASS。
- `compile-only.log`：未配置 LC_TEST 数据库，有显式 SKIP，仅编译检查。
- `root-dispatcher-lifecycle.log` / `root-dispatcher-reviewed.log`：错误测试假设仅允许 retryable；锁定 River v0.40.0 短延迟重试实际为 available。修正为先验证 durable UNKNOWN/generation/mode 与安全错误，再拒绝所有终态。
- `root-dispatcher-final.log`：首次 error 尚未写入时 nullable JSON 谓词扫描 bool 失败；使用 COALESCE(...,false) 后重跑通过。
- `root-browser-regression.log`：初跑 exit 2、在 Next build 后结束，根因未确定；保留失败。line-only ERR trap 诊断重跑通过，随后原始命令再次运行的 `root-browser-final.log` exit 0 才作为最终证据。

这些 fixture/测试判据修正未削弱生产执行器的租约/代次/事务/权限逻辑；保留原失败日志。

## 未验收与升级边界

- 生产 provider 适配器、真实 credentials、授权资格/用途策略、sandbox/live；当前 callbacks 只证明框架约束，不能证明平台准入。
- 全局配额、durable Retry-After/jitter、inbox/webhook、lease renewal、retention rescue scanner、授权恢复/取消 UI，以及真实注册 provider 的 worker CLI。
- BeginCheckout 买家权限桥、订单+HELD+operation 原子成交与外部支付闭环。
- G02/G04/G14 完整门禁、生产 IdP/部署与完整 SaaS。
- 不承诺网络效果 exactly-once；数据库 fencing 无法停止已在远端执行的旧请求。实际适配器必须单独证明 idempotency 与 query-only reconciliation。

下一步冻结 BeginCheckout 合同，复用已验收的报价/库存/operation 原语；不能把 buyer
伪装为 merchant principal，不能绕过库存或另造交易引擎。未触碰客户直播、订单、消息或支付。
