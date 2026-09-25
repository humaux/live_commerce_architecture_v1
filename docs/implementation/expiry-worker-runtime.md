# 订单到期 worker 运行说明

范围仅是已有 DRAFT／HELD 订单到期后的安全释放。复用原
`checkout.ExpiryWorker` → `checkout.expire_held`，不是新的库存写入或付款超时引擎。
验收依据 [EW01–05 合同](../../contracts/checkout-expiry-runtime-v1.md)及单独验收记录；
本文不代表已在客户生产部署。客户现有平台、直播与订单不能被迁移测试影响。

## 启动和依赖

构建：`go build -o /受控输出目录/expiry-worker ./cmd/expiry-worker`。
进程没有 HTTP 端口，不读取商家凭据、付款 keyring 或供应商地址。仅注册一种
到期任务，消费固定 `checkout_expiry_v1` 队列，不消费 `default` 或付款队列。
River 的 leader 调度维护是全局的，可能把其他队列已到期任务从 scheduled
置为 available；这不是领取或执行业务任务。队列隔离不等于该进程对其他
River 行完全零维护写入，不能据此宣称整行永远不变。

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
   新进程先保持关闭。检查没有运行中的旧到期任务，不停止客户旧平台业务。
2. 使用迁移 owner 执行现有迁移入口。`post_river/0002_checkout_expiry_queue.sql`
   在既有迁移锁内取得 River 表 `SHARE ROW EXCLUSIVE` 锁，会短时阻塞该表写入、
   领取和更新，必须安排获批的维护窗口，不能直接在忙碌生产尝试。
3. 只搬迁合法活动 `default` 到期任务的队列，原时间、ID、参数及历史保持不变。
   运行中、孤儿、错误参数、异常队列、唯一键或保留队列中的其他任务使整个
   post-River 事务回滚。业务迁移与此阶段分开提交，不宣称跨阶段全部回滚。
4. 新延迟 AFTER INSERT 触发器兼容“先插任务、后建订单”的旧生产者事务。
   它只认 `orders.job_id` 和原始 `generation=1/version=1`，**不匹配当前订单版本**：
   开始付款会提升版本，旧到期任务仍应运行并由业务层判定 STALE，而非释放库存。
5. 用普通 worker 权限执行 `SELECT checkout.expiry_queue_ready()`，必须为真。
   false 时受权诊断，不删除校验和、改已应用 SQL、批量修任务或回退 owner 连接。
6. 更新显式入队的 API 并启用新进程。旧生产者通知可能仍指向 default，固定队列
   通过正常轮询接收，不能据此承诺即时通知或特定秒数的业务 SLA。

回退先停新进程并保留数据、队列和触发器，再向前修复。不能把任务挪回 default
交给只注册部分任务的消费者；旧迁移二进制遇到未知版本应拒绝。迁移从不改变
已完成／取消／丢弃的历史任务；终态失败需要另行受权对账，不自动恢复为成功。

## 业务与恢复边界

- `expire_held` 仍在同一事务锁定订单、预留和库存：仅到期且版本匹配的
  DRAFT／HELD 可取消并写释放台账。未到期 snooze 使用数据库时间。
- 付款中、已付款、版本失效或已释放不能再次释放；重新投递同一任务不应增加
  台账或到期事件。丢失交付、支付返回页、未知供应商结果都不构成释放理由。
- 保留 River v0.40.0 的崩溃回收默认值：一小时年龄、30 秒扫描周期。测试对自有
  任务时间的老化只验证回收路径，不证明真实墙钟恢复速度或 15 分钟释放 SLA。
- 失败任务巡检／对账扫描、容量、指标告警、更短恢复 SLO 与受权部署仍需独立
  gate。`external_operation_v1` 保持未由此入口消费，不虚构供应商处理路由。
