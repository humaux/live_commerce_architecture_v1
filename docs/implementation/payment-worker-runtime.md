# 付款 worker 运行说明

本进程只负责已有付款尝试的查询和可信 QUERY 观察记录入账。供应商资格、
通知 ACK、商家收款开关、退款、默认队列的订单过期任务不在此入口。构建通过或
进程存活不代表已经收款；核验结果以 [运行合同](../../contracts/payment-worker-runtime-v1.md)
和单独验收记录为准。未执行客户生产部署。

## 调用和权限

构建：`go build -o /受控输出目录/payment-worker ./cmd/payment-worker`。
不需要监听端口，不暴露 HTTP 健康路由。服务管理器使用进程退出状态识别启动失败；
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
2. 保持新付款 worker 关闭；确认没有运行中的付款 job。迁移会取得 River 表的
   `SHARE ROW EXCLUSIVE` 锁，短时阻塞该表写入／领取，须安排批准的维护窗口。
3. 用迁移 owner 执行新版迁移入口。业务 SQL 与 post-River SQL 分阶段提交；
   后阶段失败时修复明确问题后重新执行，不删除校验和、不改已应用 SQL。
4. 后阶段仅移动合法活动付款 job 的 `queue`，终态历史和其他任务不改。
   运行中、孤儿、错误关联、意外队列或 unique key 会使整个后阶段回滚。
5. 用普通 worker 角色执行 `SELECT integration.payment_queue_ready()`，必须为真。
   该函数只给布尔值；false 时交由受权运维核查，不自动变更业务状态。
6. 更新 API / query 生产者并启动匹配环境的 worker。旧版本默认入队仍由提交时
   触发器归队；旧通知指向 default，需靠 River 轮询接收，不能要求即时通知。

回退先关闭此新付款进程，保留数据库与任务，诊断后向前修复。不要为回退二进制
删除触发器或把 job 批量挪回默认队列；旧迁移二进制不认识新 ledger 版本时应拒绝。
`SIGINT` / `SIGTERM` 正常停止取任务，给在途任务最多 15 秒，然后最多 5 秒取消，
再关闭自有连接池；未确认远程效果保持原有 UNKNOWN／对账语义。

## 必须保留的边界

- 不能用返回页、授权成功、query HTTP 200 或 worker 心跳推断 Paid。
- 不能因旧 key 不可读就重新创建付款尝试或释放不确定库存。
- 默认队列、退款、真实供应商验收、可信资格发行和容量／告警仍需各自验收。
- 当前按队列固定环境不是租户权限替代；租户／店铺／金额校验仍在原 SQL 域内。
