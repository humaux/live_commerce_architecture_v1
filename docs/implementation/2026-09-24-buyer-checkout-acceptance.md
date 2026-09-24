# 内部买家下单、库存预留与到期释放验收

2026-09-24，代码／测试基线 `162b619`。范围是内部 Go/SQL 下单聚合，
**不是公开结账、StartPayment、第三方支付／物流接通或可部署完整 SaaS**。
合同：[buyer-checkout-v1](../../contracts/buyer-checkout-v1.md)。

## 交付与调用关系

- `checkout.New/Service.Begin/Get` 使用独立 `commerce_checkout_runtime` pool。
  普通商家、buyer、issuer、worker、owner、混合角色或 SET ROLE 伪装不能装配。
  它是可信服务权限，不是新的买家身份；买家仍使用既有 opaque capability。
- 一个事务内重验购物车／报价、门市来源／目的地、当前配送配置和仓库优先级，
  复用原 Go calculator、余额行锁和纯分配器。锁等待后再核 capability 与报价、
  目的地、来源截止时间；缺货不产生部分预留。物流 API 模式仍不可启用。
- 固定 SQL writer 重解 scope，写入 DRAFT 订单、HELD 预留、库存台账、事件、
  私有永久幂等回执及同事务 River job。订单与预留共享 UUID；原库存台账仍是
  唯一余额 writer。新增 actor family 和 parent guards 防止商家旧接口绕过买家订单。
- 幂等按 owner/store/key，不按 session；同一买家换有效会话仍重放同一结果，
  不延长保留时间。不同参数冲突；同 cart/version 新 key 也不能再次预留。
  读取返回 owner 隔离的不可变商业快照与当前状态，任务／回执不存收件人信息。
- `NewExpiryWorker` 消费私有 `checkout_expiry_v1`，默认保留 15 分钟。
  SQL 固定入口锁订单→预留→排序余额，按 DB 时间和 generation 判定：
  DRAFT+HELD 到期才 CANCELLED/EXPIRED 并 RELEASE；早到 snooze，旧代或终态 no-op。
  未新增依赖，也未在库存事务中调用任何外部供应商。

## 分工、修复与审查

根代理先在 `1641699` 冻结接口、权限、锁顺序和事务合同，再安排两个不重叠作者。

|角色|实际模型／推理|base／独立工作树|允许写入及提交|
|---|---|---|---|
|Go 作者 checkout_go_worker|gpt-6-sol／high|1641699；`/Volumes/data/worktrees/live-commerce-checkout-go`|checkout 三文件、platform 两文件及 buyer.go 注释；作者 ebfee20，合入 0f9fdde|
|SQL 作者 checkout_sql_worker|gpt-6-sol／high|1641699；`/Volumes/data/worktrees/live-commerce-checkout-sql`|0013 迁移及 migrate.go；作者 8eb467e，合入 a2f495a；安全投影 91a5f26、generation e903067、宅配修复 bb3dc90|
|Root 集成|当前会话配置|main|合同、真实 PG 测试、runner、证据和文档；22b5fd4、9ed8aa8、162b619|
|独立 reviewer|gpt-6-sol／high|只读 main|不改代码、不作为作者；Go+SQL 合并审查及日志／测试因果复核|

首轮真实 PG 失败，未掩盖：安全投影改用 `RECORD` 后，宅配路径没有初始化门市
record，复合 IF 仍解析其字段，导致下单失败。`bb3dc90` 将其检查放入明确非空分支；
修复记忆 `ae40c987-79b9-4fde-b4ec-566c0f77ec8a`。
同时发现“只要报错就通过”的故障测试可能提前失败而未触及注入点；`9ed8aa8`
加入非事务 sequence 标记，必须证明指定 trigger 执行后才接受回滚证据。
没有删除负例、降低阈值或让作者单测代替真实数据库验收。

合并 Go/SQL 独立复核在 `bb3dc90` 未确认新增 P0/P1，记忆
`c7407dae-016e-43f3-a3f6-39ff3d12f68e`。审查者独立读过 184 项前轮日志，
未自己重跑 PG。随后 root 增补提交前不可见、最后等待后来源／目的地过期和
外店／外买家对象三组测试到 `162b619`。最终复核记忆
`8c07e875-b84f-4816-b0c6-461ecfe1e090`：无新增 P0/P1，独立计数 187 顶层 PASS、
429 嵌套 PASS、0 FAIL／SKIP，浏览器 1 PASS，并核对两份 SHA256；未自己重跑 PG。
保留一项 P2 证据精度限制：该超商测试让来源与目的地同时过期，只证明最后余额
等待后的共同到期拒绝，不能独立证明 source TTL 分支。正常 invariant 是目的地
到期不晚于来源；不将共同失败虚报为两个谓词各自隔离通过。
可信 checkout 角色直接 SQL 的更广泛 forged-GUC 组合测试可继续补充；现验收
不是任意 SQL 输入的穷尽证明，也不允许将该角色交给终端用户。

## 实际运行与证据

日志目录：`/Volumes/data/output/live-commerce-checkout-tests/`。
运行 Go 1.27.1、脚本固定 PostgreSQL 18.6 镜像；测试使用专有临时数据库。

|命令／轮次|结果|日志|
|---|---|---|
|完整首轮|exit 1；宅配下单路径失败，85.942s；保留失败证据|root-initial.log|
|修复后 `bash scripts/dev/test-local.sh --checkout`|exit 0；定向真实 PG，10.666s；不代替完整 gate|root-focused-1.log|
|9ed8aa8 完整回归|exit 0；184 顶层 PASS／0 FAIL／0 SKIP；race + vet，84.802s|root-full-final.log|
|162b619 `bash scripts/dev/test-local.sh`|exit 0；187 顶层 PASS／0 FAIL／0 SKIP；race + vet，68.828s|root-final-boundaries.log|
|`bash scripts/dev/test-local.sh --browser-identity`|exit 0；真实 Chromium→Next→Go→PG，签名 MOCK IdP；1/1，foundation 6.046s|browser-identity.log|

```text
27514b08117ce294a561cb79004b02a0a4442183270f1441b46ea50c45dd4d23  root-final-boundaries.log
83cf9bed3eae50493aa71c5c2232b6760abbaccabf9c9091314f43707ab79a92  browser-identity.log
```

187 是仓库顶层测试数量，不是 187 个产品功能。新增真实 PG 覆盖：

- 聚合事实完整、精确快照、原子入队、提交前 job／库存／订单不可见。
- 同 key 并发、同买家跨会话重放、不同买家同 key 不碰撞、同 cart 不二次保留；
  两买家竞争最后库存且多 SKU 反序请求，无超卖或部分结果。
- 购物车／报价／配置漂移、隐藏／停用／仓库变更、外店／外买家引用均无写入；
  门市字符串代码 `017888` 保留，选店不绑定承运商。
- 普通角色不能调用私有 writer、改订单／回执；旧商家预留命令、直接台账／
  reservation/line 写入不能改变 checkout 库存。追加行使用第二 SKU，避免重复主键假通过。
- 七种写入故障用 sequence 证明注入点，任何事实失败都不留部分订单／库存／任务。
- 用 `pg_stat_activity` 证明真实锁等待，SQL 时钟确认开始时尚未到期，再越过截止点：
  capability、receipt replay、quote 及 source/destination 共同到期在最终检查拒绝；
  后者不是 source TTL 分支独立隔离证据，见上面的 P2 限制。
- 到期回滚／重试守恒、原 key 历史重放、取消后新 key、重复／旧 generation no-op；
  实际 River worker 的 due、early、stale、duplicate 四模式均运行通过。

浏览器工件：`output/playwright/identity-chain-20260924T094540.558300000/`。
本轮没有 UI 修改；这是既有身份链兼容门禁，**不是未开发的结账或服务设置页面验收**。
2026-09-24 09:50 UTC 事后 `docker ps -a --filter label=livecommerce.fixture` 为空，
仅脚本自动回收本任务数据库；日志、浏览器工件和独立工作树保留。

## 明确未完成

- StartPayment、PSP／回调／退款／对账。PAYMENT_PENDING／COMMITTED 拒绝释放
  只通过 fixture 状态栅栏，不等于真实付款与到期并发验收。
- 正式 checkout HTTP、买家三语页面、生产进程和 worker 装配、丢失／废弃到期
  任务的巡检恢复；完整 G03/G05 不提升。
- 商家自有第三方账户连接、支付方式设置、运行时可用性、正式门市地图、承运商
  adapter、sandbox/live。人工配送不是第三方 API 的替代交付。
- 生产身份、个人信息治理、部署与全 SaaS 门禁。没有修改客户现平台的直播、
  物流／收款开关、订单、资金或会话。
