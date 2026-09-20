# 网页 GPT 独立成交模型复核

获取：2026-09-20；使用用户授权的 Kimi WebBridge 会话 `commerce-build-20260920`，没有客户数据、密钥或生产操作。会话：[架构复核](https://chatgpt.com/c/6aaf4faa-dbfc-83e8-9dd3-66e134fd1778)。这是调研意见，不是代码验收。

结论采纳到后续合同：

- 保留 Go 模块化单体与 PG 真源；无证据需要 Redis/Kafka/微服务或全站事件溯源。
- 库存四桶、复合 FK、append-only 流水；跨行守恒由受控写入口、真实并发测试和对账证明，不能只靠一条 CHECK。
- 成交统一锁序候选为命令→订单→付款→保留→排序库存；未来 expiry worker 必须遵循同序，不得预先锁 reservation 再反向锁 order。当前 T04 尚无订单/付款，因此只证明当前锁序，不能声称完整竞态已通过。
- PAYMENT_PENDING 不按旧 TTL 释放；迟到实收但无库存必须阻断履约并产生补偿工作，不伪造库存分配。退款不直接回补物理库存。
- 未知、待处理退款占用退款额度；外部成功但本地超时需查询收敛，不能换新键盲重试。第三方幂等保留期不等于永久可查询凭证。
- River 至少一次意味着业务幂等仍是必须项；同库入队仅保证事务可见性，不保证外部副作用恰好一次。

主代理另外打开并核对的官方依据：

- [PostgreSQL 18 constraints](https://www.postgresql.org/docs/18/ddl-constraints.html)：CHECK 不能保证跨行数据约束；FK/UNIQUE 或受控写入另行承担。
- [PostgreSQL 18 locking](https://www.postgresql.org/docs/18/explicit-locking.html)：一致锁序、防死锁和事务级锁语义。
- [Stripe idempotent requests](https://docs.stripe.com/api/idempotent_requests)：参数匹配和至少 24 小时后的清理边界；此文不意味着项目已选定或接通 Stripe。
- [River reliable workers](https://riverqueue.com/docs/reliable-workers)：至少一次执行与幂等设计。

后续必测：跨店、同键异参、金额溢出/快照、末件并抢、反序多 SKU 全回滚、开始付款与过期竞态、重复释放、迟到付款无库存、伪造/乱序回调、超时 UNKNOWN、并发退款额度、审计/队列/提交故障。真实 PG 与 provider sandbox 分开记；mock、sandbox 均不等于 live。
