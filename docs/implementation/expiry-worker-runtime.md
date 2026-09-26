# 订单到期 worker 运行说明

范围仅是已有 DRAFT／HELD 订单到期后的安全释放。复用原
`checkout.ExpiryWorker` → `checkout.expire_held`，不是新的库存写入或付款超时引擎。
验收依据 [EW01–05 合同](../../contracts/checkout-expiry-runtime-v1.md)及[局部验收记录](2026-09-25-expiry-worker-acceptance.md)；
本文不代表已在客户生产部署。客户现有平台、直播与订单不能被迁移测试影响。

## 启动和依赖

构建：`go build -o /受控输出目录/expiry-worker ./cmd/expiry-worker`。
进程没有 HTTP 端口，不读取商家凭据、付款 keyring 或供应商地址。仅注册一种
到期任务，消费固定 `checkout_expiry_v1` 队列，不消费 `default` 或付款队列。
当前候选将 API 生产者和消费者固定到 `river_expiry`。River 维护仍是
schema 全局，但不再与付款／外部任务共表；共享表旧版本已实测会误 discard。
[家族隔离合同](../../contracts/legacy-runtime-isolation-v1.md)的 LRI01–06 尚待通过，
不可用历史 EW 记录宣称这次修复已验收。普通 worker SQL 权限仍共享，
不承诺受侵数据库身份无法访问其他旧业务家族。

| 环境变量 | 规则 |
| --- | --- |
| `COMMERCE_EXPIRY_WORKER_ENABLED` | 空或 `0` 立即退出且只读此变量；`1` 才打开资源 |
| `COMMERCE_EXPIRY_WORKER_DATABASE_URL` | 独立普通 LOGIN，仅继承 `commerce_worker`；不接受 owner 或混合 API 角色 |
| `COMMERCE_EXPIRY_WORKER_CONCURRENCY` | 默认 4，标准十进制整数 1–16；不接受自选队列 |

`checkout.NewExpiryClient` 校验角色并在最多 5 秒的上下文内查询
`checkout.expiry_queue_ready()`。缺失／禁用／非延迟的路由触发器，活跃任务的
异常队列、唯一键、参数或订单关联均导致拒绝启动。只返回固定错误，不自动修复。

`internal/jobqueue.Run` 由付款、到期两个真实入口共用：同步 River Start 的
看门狗为 10 秒，成功后解除并等待看门狗结束，才记录固定 `expiry_worker_ready`。
正常工作上下文独立于停止信号，允许最多 15 秒完成在途任务，再最多 5 秒取消；
调用方最后关闭自有连接池。没有通用进程框架或新增第三方依赖。
ready 只证明本地启动，不代表所有到期任务已处理。River 原始日志仍丢弃以防泄密，
不能将安静日志当作零错误；完整指标、告警与恢复时效另需上线验收。

## 升级顺序与停止线

1. 生产变更必须取得批准、完成可恢复备份，并先在隔离副本运行门禁。
   新进程先保持关闭。经批准停止并排空旧付款／到期生产者及消费者，不擅自
   停止客户现有平台业务；无法获批窗口就不迁移。
2. owner 运行现有迁移入口。0032 先失败关闭 readiness，原生 runner 创建两套
   新 schema；post0005 按 `river`、`river_payment`、`river_expiry` 的 job/queue
   顺序取 `ACCESS EXCLUSIVE` 锁。会影响旧表外部任务，必须先审批影响面。
3. 只移动通过完整验证的付款／到期任务，原字段、ID、终态历史与暂停队列保持不变。
   运行中、孤儿、错误参数、异常队列、唯一键或保留队列中的其他任务使整个
   post-River 事务回滚。业务迁移与此阶段分开提交，不宣称跨阶段全部回滚。
4. 新 schema 的延迟 AFTER INSERT 触发器仍支持“先插任务、后建订单”的事务。
   它只认 `orders.job_id` 和原始 `generation=1/version=1`，**不匹配当前订单版本**：
   开始付款会提升版本，旧到期任务仍应运行并由业务层判定 STALE，而非释放库存。
5. 用普通 worker 权限执行 `SELECT checkout.expiry_queue_ready()`，必须为真。
   false 时受权诊断，不删除校验和、改已应用 SQL、批量修任务或回退 owner 连接。
6. 更新所有生产者的固定 schema 后才启用进程。旧 schema 的付款／到期写入
   明确失败关闭，不再靠跨 schema 通知或旧生产者兼容；不承诺特定恢复 SLA。

回退先停新进程并保留数据、队列和触发器，再向前修复。不能把任务挪回 default
交给只注册部分任务的消费者；旧迁移二进制遇到未知版本应拒绝。迁移从不改变
已完成／取消／丢弃的历史任务字段；这里只移动存放位置。终态失败需另行受权
对账，不自动恢复为成功。目标序列保留高水位，不能因任务已清理就复用历史 ID。

## 业务与恢复边界

- `expire_held` 仍在同一事务锁定订单、预留和库存：仅到期且版本匹配的
  DRAFT／HELD 可取消并写释放台账。未到期 snooze 使用数据库时间。
- 付款中、已付款、版本失效或已释放不能再次释放；重新投递同一任务不应增加
  台账或到期事件。丢失交付、支付返回页、未知供应商结果都不构成释放理由。
- 保留 River v0.40.0 的崩溃回收默认值：一小时年龄、30 秒扫描周期。测试对自有
  任务时间的老化只验证回收路径，不证明真实墙钟恢复速度或 15 分钟释放 SLA。
- 失败任务巡检／对账扫描、容量、指标告警、更短恢复 SLO 与受权部署仍需独立
  gate。`external_operation_v1` 保持未由此入口消费，不虚构供应商处理路由。
