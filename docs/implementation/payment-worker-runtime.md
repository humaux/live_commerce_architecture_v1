# 付款 worker 运行说明

本进程只负责已有付款尝试的查询和可信 QUERY 观察记录入账。供应商资格、
通知 ACK、商家收款开关、退款、订单过期任务不在此入口。到期任务由独立的
[expiry worker](expiry-worker-runtime.md)处理。构建通过或
进程存活不代表已经收款；核验结果以 [运行合同](../../contracts/payment-worker-runtime-v1.md)
和单独验收记录为准。未执行客户生产部署。

当前实现已按[家族隔离合同](../../contracts/legacy-runtime-isolation-v1.md)改用
`river_payment`，LRI01–06 已通过[独立本地验收](2026-09-26-legacy-runtime-isolation-acceptance.md)；
历史 PW 记录与本轮维护隔离证明分别保留，均不代表生产部署。
`cmd/api/buyer_payment.go` 和 query 后的 reconcile 生产者使用同一 schema，
但不启动额外 worker。普通 `commerce_worker` 仍跨旧业务家族共享 SQL 权限，
因此这是维护语义隔离，不是数据库身份安全隔离。

## 调用和权限

构建：`go build -o /受控输出目录/payment-worker ./cmd/payment-worker`。
不需要监听端口，不暴露 HTTP 健康路由。服务管理器使用进程退出状态识别启动失败；
成功完成启动预检、River Start 及启动看门狗解除后输出固定 `payment_worker_ready`
日志，不带账户或凭据；这只证明本地启动，不证明供应商可用或收到资金。
当前 River 原始日志被丢弃，避免任务错误泄密。完整运维指标与告警仍属上线门禁，
不能把安静日志当作没有错误。

|环境变量|规则|
|---|---|
|`COMMERCE_PAYMENT_WORKER_ENABLED`|空或 `0` 立即退出；只读取此开关。`1` 才接入数据库|
|`COMMERCE_PAYMENT_WORKER_DATABASE_URL`|独立普通 LOGIN，仅继承 `commerce_worker`；不接受迁移 owner 或混合 API 角色|
|`COMMERCE_PAYMENT_WORKER_PROFILE`|严格 `SANDBOX` 或 `LIVE`；每个进程只消费自身固定队列|
|`COMMERCE_PAYMENT_WORKER_CONCURRENCY`|默认 4，标准十进制整数 1–16|
|`COMMERCE_ACCOUNT_ACTIVE_KEY_ID` / `COMMERCE_ACCOUNT_KEYS_JSON` / `COMMERCE_ACCOUNT_REPLAY_KEY`|复用 API 的严格 keyring 格式，仅由密钥管理注入，不写日志或命令行|

保留仍有历史付款引用的旧加密密钥。更换当前商家配置不改变旧付款冻结的账户、
凭据版本和执行环境。`PROVIDER_MOCK` 仅供测试代码注入签名模拟 transport，
不能通过生产入口的环境变量开启，也没有任意供应商 URL 配置。

启动预检验证普通 worker 权限、延迟路由触发器及所有活动付款任务的冻结关联。
任何脏路由都失败关闭，进程不会修复／删除数据，不会回退到 owner 连接。
三种合法执行环境可以共存；`SANDBOX` 与 `LIVE` 需要各自进程。

## 升级顺序与停止线

1. **生产必须先取得批准并完成可恢复备份**；先在复制的隔离环境验收。
   这是新系统数据库变更，不得操作客户现有平台、直播或订单。
2. 保持新进程关闭；经批准停止并排空旧付款／到期生产者及消费者。
   post0005 按固定顺序取得旧 `river`、`river_payment`、`river_expiry` 的 job/queue
   表 `ACCESS EXCLUSIVE` 锁，也会阻塞旧表外部任务活动，须先批准影响窗口。
3. 用迁移 owner 执行新版迁移入口。业务 SQL 与 post-River SQL 分阶段提交；
   后阶段失败时修复明确问题后重新执行，不删除校验和、不改已应用 SQL。
4. 0032 在已安装旧版本上先关闭 readiness；首次安装缺谓词也失败关闭。
   后阶段验证并原样移动付款／到期任务所有字段、ID、暂停队列，推进独立序列。
   运行中、孤儿、错误关联、异常目标或 unique key 均回滚后阶段，准备阶段不回滚。
5. 用普通 worker 角色执行 `SELECT integration.payment_queue_ready()`，必须为真。
   该函数只给布尔值；false 时交由受权运维核查，不自动变更业务状态。
6. 更新全部 API/query 生产者后再启动匹配环境 worker。新 schema 中合法 default
   入队仍由提交时触发器归队；旧 schema 中的付款／到期写入会明确拒绝，不兼容
   未升级二进制。不能改守卫或复制回旧表绕过此停止线。

回退先关闭此新付款进程，保留数据库与任务，诊断后向前修复。不要为回退二进制
删除触发器或把 job 批量挪回默认队列；旧迁移二进制不认识新 ledger 版本时应拒绝。
`SIGINT` / `SIGTERM` 正常停止取任务，给在途任务最多 15 秒，然后最多 5 秒取消，
再关闭自有连接池；未确认远程效果保持原有 UNKNOWN／对账语义。
生命周期实现现位于 `internal/jobqueue.Run`，与到期进程复用；原有启动看门狗、
安全错误码、付款配置和信号边界保持不变，两个进程的真实门禁须一起回归。

当前沿用 River v0.40.0 的崩溃任务回收默认值：运行超过 1 小时才满足回收年龄，
维护扫描周期默认 30 秒；这不是付款恢复时效承诺。查询域仍使用 30 秒租约和
2 分钟正常重查间隔。若试点要求更快的崩溃恢复，应在容量／恢复 SLO 门禁中
调整回收配置并重跑强杀验收；测试中的人为老化任务不证明真实墙钟恢复时长。

## 必须保留的边界

- 不能用返回页、授权成功、query HTTP 200 或 worker 心跳推断 Paid。
- 不能因旧 key 不可读就重新创建付款尝试或释放不确定库存。
- 默认队列、退款、真实供应商验收、可信资格发行和容量／告警仍需各自验收。
- 当前按队列固定环境不是租户权限替代；租户／店铺／金额校验仍在原 SQL 域内。
