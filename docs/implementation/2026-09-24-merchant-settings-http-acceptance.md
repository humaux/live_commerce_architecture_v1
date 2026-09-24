# 商家物流与金流方式：HTTP 验收

2026-09-24；合同 `891640f`，集成代码及测试 `d86aa84`。
接口：[merchant-settings-http-v1](../../contracts/merchant-settings-http-v1.md)。
范围是本地真实 HTTP handler → merchant runtime → PostgreSQL，不是设置 UI、
商家凭据连接页面或第三方收款/运输验收。

## 已交付

- 物流方式 GET/PUT、支付方式 GET/PUT、支付诊断 POST，共五条商家接口。
  完整三语 DTO、精确 path/body 目标校验、版本冲突与永久幂等复用现有领域逻辑。
- 物流启用/显示与费用政策分离，支付草稿显示与支付启用分离。可用性诊断只读，
  不创建配置、回执、审计、交易或任务。没有接收/返回 PSP 密钥。
- 复用 `httpapi.scoped/bodyRoute`、认证会话、`fulfillment`、`payments`，无新增
  依赖、表、队列、引擎或供应商请求。错误沿用 no-store、安全消息与服务器请求 ID。
- 支付方式及 API 物流的启用门禁没有移除。保存成功不是供应商资格或收款成功。

## 权限竞态：根因、修复、反证

独立预审发现 `fulfillment.SetService/GetService` 在等待前核验权限，但缺少返回前
复核。等待 command 锁时权限被撤销，新保存与回执重放仍能返回；读取等待表锁
也有同样缺口。仅在 HTTP 层补检查会遗漏其他领域调用者。

根因修复在两个领域函数：成功的写入/重放/读取后再次核验权限；拒绝时返回空 DTO，
由调用方 `WithScope` 回滚整个事务。未改租户、RLS、权限集合或查询范围。

独立测试持有 command advisory 锁或 `service_heads` 的 ACCESS EXCLUSIVE 锁，
用 `pg_blocking_pids` 确認请求确已等待，再撤销 grant 并解锁。旧代码 write/replay/read
三个子用例均返回 200（应为 403），修后均返回 403，拒绝的新写入不留下 revision。
这不是只在请求前撤权，也不是用固定 sleep 假定已经产生竞态。

## 实测

隔离 PG18.6（仓库固定镜像 digest）、Go1.27.1、虚构 merchant/session，零生产数据。
所有日志位于 `/Volumes/data/output/live-commerce-settings-http-tests/`。

|命令/阶段|结果|日志|
|---|---|---|
|作者 `go test -race ./internal/httpapi`；`go vet ./internal/httpapi`|均 exit0；独立 reviewer 也复跑同包 race|作者提交 `4d83a24` / Humaux `dd768b3b-a218-4aaf-8b6c-26c444c94202`|
|root `bash scripts/dev/test-local.sh`，修权限前|exit1；234顶层 PASS / 1 FAIL / 0 SKIP；3个撤权子例失败；foundation95.575s|`before-auth-fix.log`|
|root 同一完整命令，修权限后|exit0；235顶层 PASS / 0 FAIL / 0 SKIP；race + vet；foundation94.661s|`after-auth-fix.log`|

235 = 既有226 + 作者3个 transport unit + root6个真实 PG HTTP 顶层测试，
不是235项业务功能。root 首次编译曾使用不存在的 fixture 字段 `storeB1`，
按实际定义修正为 `storeB` 后才开始上述完整跑批；未改变业务判断或放宽测试。

```text
e3972a67e9818c1f842452d0e776e913a120412723218f87543d75125c9844d1  before-auth-fix.log
55111cdee10946f4ec9f83b4bfbe07ab6c07482300fc66a9a1be3d430f1faf46  after-auth-fix.log
```

H01–H06 内部 HTTP gate 通过：真实读回及新 handler 读取；独立启用/展示；同键旧结果
重放、改 body/旧版本冲突、双编辑单赢家；401/403/404、伪造 scope/目标拒绝；只读
诊断与启用门禁；错误 media/未知字段/尾随或超长 JSON/查询参数/空问号/错误方法；
错误响应不回显 token/私密输入；最终权限竞态；完整历史回归。

## 分工与收尾

- root：契约、独立 PG 测试、领域根因修复、完整复跑、集成和共享文档。
- commerce worker：`gpt-6-luna/high`，base `891640f`，独立工作区
  `/Volumes/data/live-commerce-worktrees/merchant-settings-http-20260924`，
  分支 `codex/merchant-settings-http-20260924`，作者提交 `4d83a24`。仅负责
  `settings.go/settings_test.go` 与 `handler.go` 注册调用；没有递归委派。
- reviewer：`gpt-6-sol/high`，只读合同、完整变更、测试因果与最终日志；
  Humaux `e77b1daa-6efb-4d4f-9516-5089f1aa4f8a`，无未解决 P0/P1。
  作者 patch 先以 no-commit 应用，根因修复和绿测/终审后才集成为 `d86aa84`。
- root canonical `live-commerce` 图谱增量5文件、96实体、0拒绝；关联撤权修复
  记忆 `bb3c3f1d-92a4-4220-9e9e-9b7495930d48`。自有测试容器标签查询为空；
  保留失败/成功证据与作者 worktree，不清理其他任务。

## 未运行与下一步

本轮无 UI 改动，没有把 HTTP 测试称作浏览器验收。设置页面、方式/供应商目录列表、
自有凭据连接 HTTP/UI、生产主密钥装配、买家 StartPayment/通知入账/退款/对账、
各供应商 sandbox/live 仍待实现或验收。全局 MS01–MS08/G01–G15 不因本切片升级。

StartPayment 独立预审已存 `2aa21a6e-abf6-4acd-986d-980f1ae9372a`：现有 T06
商家 principal 不能冒作买家；托管付款的后端任务只查询，须固定尝试的账户与凭据
版本，停用新收款不能阻断旧交易对账。该建议未在本单元实现。
