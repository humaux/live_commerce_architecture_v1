# T06 内部操作台账与 worker 权限边界验收

2026-09-20；代码基线 `9bac8e4`。这是内部 Go/PG 切片，不是完整 T06、
外部业务 dispatcher、支付/物流/Meta 接通或生产上线验收。没有操作客户库、
直播、订单、支付或消息。合同见 `contracts/external-operation-v1.md`。

## 本次交付

- `0008_external_operations.sql`：binding、永久 operation、追加式 event；
  FORCE RLS、复合外键、不可变意图字段、精确的三项 integration 权限。
- `internal/integrations/core`：RegisterBinding、SetBindingEnabled、Plan、Get、
  Claim、Complete。复用调用方事务、command.Run、River InsertTx；无新依赖。
- Plan 的 SHA-256 包含 actor、binding/version、用途、动作和规范化 JSON；
  保留大整数精度。命令 receipt 或原 River job 被清理后仍返回原始历史 ID，
  同键异参冲突，不能把历史 replay 误作再次发起外部操作。
- 普通 worker 无业务表直接写权，只能调用固定 SECURITY DEFINER 函数；
  owner/混合角色启动被拒绝，River 迁移历史明确不可改。
- Claim/Complete 使用锁后数据库时钟、generation、32 字节随机租约 token；
  仅存摘要，token 不进入 JSON/job/日志。过期 DISPATCHING 只能 reconcile，
  UNKNOWN/ACKNOWLEDGED 不会重新 dispatch。binding 变化不会抹除已知远端结果。

## Gate 与证据

`bash scripts/dev/test-local.sh` exit 0：隔离 PG18.6、Go1.27.1、全套
`-race -count=1` **106 顶层 PASS / 0 FAIL / 0 SKIP**，随后 `go vet ./...` 通过。
最终日志 `/Volumes/data/output/live-commerce-t06-tests/root-reviewed.log`；
SHA-256 `f1d1710eb2b2289cfd288054877889a58f32180b0a8c411fea4d99b3626edf30`。

|范围|已验证的关键反例|
|---|---|
|永久幂等|8 路同键一条 operation/job；独立计算精确 SHA；删除 receipt 后变 actor/request 均冲突，无新 receipt/job；删除 job 且撤销 binding 后精确历史回读|
|五类事实原子性|job、operation、event、audit、receipt；强制队列/event/audit 失败全部回滚|
|权限|无精确 manage/read/execute 拒绝；scope/GUC 不一致拒绝；跨 tenant/store、buyer/issuer、owner/混合角色、直接状态写与迁移历史改写拒绝|
|并发与租约|并发 claim 仅一胜者；busy 不发新事件；错 token、旧 generation、过期 lease 均拒绝；有效流程精确 7 个 event|
|结果保真|发送前撤销为 STALE_BINDING；发送后撤销仍保留成功或 UNKNOWN，不切换资产；重复 blocked_binding 不增加事件|
|River 生命周期|普通 worker 启动，消费唯一 probe job，持久状态 completed，停止；不是业务 dispatcher|

身份浏览器回归 `bash scripts/dev/test-local.sh --browser-identity` exit 0：
真实 Chromium → production Next → Go → 隔离 PG，外部 IdP 为 RS256 **签名 mock**，
1/1 PASS。日志 `root-browser-regression.log` SHA-256
`29e9b7b0cb974bf5990e4a0d92936a23254ee485fc46023ffb77bc3a87eee7d2`；
工件 `output/playwright/identity-chain-20260920T095816.233939000/`。
本轮没有 UI 改动；不重复宣称生产 IdP 或全站视觉已验收。

## 独立审查与修复

只读 agent 独立审 SQL 和 Go 服务，最终 PASS_BOUNDED，无未关闭 P0/P1。
SQL 审查收回 River migration 表权限；Go 审查要求补全 receiptless 永久摘要
负例，并移除失败日志中整个 ClaimResult 的格式化，避免泄露 token。
上述修正提交 `9bac8e4`，最终全套 gate 保持通过。Humaux 最终审查：
`5aea5539-fa64-4dae-b20e-2e1219065e83`，标题
`T06 Go operation service final bounded security PASS at 9bac8e4`。

保留 `root-authority.log` 的一次失败：原负例使用 INSERT SELECT * 向 GENERATED
ALWAYS id 插值，先触发 428C9，未实际验证 ACL。改为合法显式列 INSERT 后得到
预期 42501；`root-authority-fixed.log` 及之后全套通过。未放宽权限或删除负例。

## 未覆盖与下一步

实际 external-operation dispatcher、冻结请求的适配器策略、跨进程崩溃恢复、
查询式 reconcile、超时/配额/重试调度、inbox/webhook、授权取消、真实 provider
sandbox/live 均 NOT_RUN。受限队列探针不等于这些能力。买家 BeginCheckout 的
独立权限桥与 order/reservation/operation 原子事务尚待实现，不能给 buyer
冒用 merchant 权限。完整 G02/G04/G14 和 T06 不因这个切片而变成 PASS。

验收脚本已自动移除自身临时容器；失败日志和浏览器证据保留供追溯。
