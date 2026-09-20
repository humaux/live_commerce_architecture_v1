# 买家匿名凭证：内部权限边界验收

2026-09-20；T03 局部切片。**真实隔离 PostgreSQL 验收通过，不是买家登录页面、
公开购物车、完整结账或生产上线。** 新界面继续遵循用户“先确认视觉稿再开发”。

## 实现及复用

- 冻结合同：`contracts/buyer-capability-v1.md`（初稿 `9352399`）。
- SQL / 角色检查：`7e064b6`，定向修正 `68e7618`、`cf1cb1d`、`a7150d5`。
- Go / 测试作者提交 `509d6450230c2b929c18c62cd81c816ee3c74abc`；root 整合为 `8c6684c`。
- 只复用现有 pgx、Go 标准库及 PG。没有新依赖、公开路由、外部服务调用或客户生产写入。
- 匿名能力凭证不是已验证个人身份。DB 派生 tenant/store/owner/session，只有随机
  32 字节凭证的 SHA-256 入库；不创建商家成员、不根据邮箱/姓名合并 Meta 身份。
- 买家运行、买家签发、商家运行、商家签发四种角色互斥。买家角色不能通过设置
  商家 scope 来取得商家表权限；私有 writer 不能被登录角色继承。
- 授权事务按 tenant → store → owner → session 加锁；锁后检查有效状态和 DB 时间。
  已获授权的请求可先完成，撤销等待其提交；撤销成功返回后，新请求被拒绝。
  超时会返回错误，不冒充撤销成功。过期/未知/已撤销凭证重复撤销不写新事件。

## 实际 gate

|验证|结果与边界|
|---|---|
|作者真实 PG|`bash scripts/dev/test-local.sh` exit 0；仅任务自建临时库。|
|独立审查|`commerce_t04_sql_review` 对精确作者 SHA 静态审查、compile-only、格式及证据核对，无剩余 P0/P1；没有以此替代 root 动态测试。|
|Root 合并后重跑|同一脚本 exit 0，86 个顶层测试组 PASS，0 FAIL / 0 SKIP；包含 race 和 vet。|
|买家子场景|7 个子组 PASS：签发/解析/撤销，校验和权限矩阵，回滚/panic/cancel/连接复用，撤销与过期锁竞争，以及 tenant/store/owner 三种停用竞态。|
|故障与隔离反例|事件写入失败导致 owner/session 一并回滚；跨店/跨 owner FK 被拒绝；SQL 权限拒绝明确验证 `42501`，FK 拒绝验证 `23503`；JSON 不暴露 token。|
|商家浏览器回归|`bash scripts/dev/test-local.sh --browser-identity` exit 0，Chromium → Next → Go → 真实 PG，1/1 PASS；**外部 IdP 是 RS256 签名 MOCK**。|
|架构包检查|`python3 scripts/check_packet.py` exit 0，仍只表示 `PASS_PACKET_STRUCTURE_ONLY`。|

## 真实失败与修复

1. 真实 PG 首跑发现：writer 普通 SELECT 可见店铺，但 SELECT FOR SHARE 返回零行，
   签发误报 unauthorized。PG 行锁读还检查 UPDATE 的已有行 RLS 可见性。新增仅
   writer 的 `UPDATE USING(true) WITH CHECK(false)` 策略：允许锁定，仍禁止实际
   UPDATE。正例签发和 `UPDATE id=id` 拒绝均已验证，没有给买家登录角色表权限。
2. 独立审查发现过期撤销与合同不符：原实现会多记 revoked 事件。现在拿到锁后
   检查 DB 过期时间，过期不写；测试验证 revoked_at 仍为空、仅一条 issued 事件。
3. 撤销不能沿用普通事务的一秒行锁超时；否则会早于合法请求结束而失败。
   撤销允许在五秒请求预算内等待；实测等待超过一秒后由原请求提交解除阻塞。

## 证据与复跑

工件目录：`/Volumes/data/output/live-commerce-buyer-tests/`。

|工件|SHA-256|
|---|---|
|`commerce-ui-finish-01-final2.log`|`11c0956d633f9818d0d0b044ae9eda007f3a2b07dcef090a71a51c5d9a60a50d`|
|`root-buyer-acceptance.log`|`7ad7ee0bf8b11fa11b222848ebe3eef6524572049adf387700a15883ba53a9d9`|
|`root-postmerge-browser.log`|`2771dae09c54eb7f40ce482bfa8b4e1cc4457fde211d8ccf559ff51d733ac260`|

保留失败原始日志 `commerce-ui-finish-01.log`、`commerce-ui-finish-01-targeted.log`；
浏览器子证据在 `output/playwright/identity-chain-20260920T080807.872328000/`。
测试脚本只创建有独占标签的 loopback/tmpfs PG，退出移除自身容器；浏览器 harness
回收自身 Go/Next/mock IdP 子进程与临时 session 文件，不使用已有 DATABASE_URL。

作者 worktree `/Volumes/data/live-commerce-wt-t03-buyer`，原始 base `9352399`，只写
`internal/buyer/buyer.go` 与 `tests/foundation/buyer_capability_test.go`。root 独占
迁移和公共合同；第二 agent 只读安全复核，无递归委派。模型继承宿主配置；工具
未披露精确模型变体/推理档位，不编造型号或费用。root 复跑不是作者自测的转述。

## 仍需完成

公开入口的受信域名/已发布店铺解析、Cookie/CSRF/限流、凭证保留与清理策略；
buyer-owned 购物车 RLS 和幂等 receipt；版本化 market/shipping/tax 与 Quote；
T06 外部操作台账；原子订单/库存预留与 PSP/退款/对账；真实提供商和生产部署。
不能把缺少的运费/税配置当零，也不能直接沿用 merchant membership FK 来代表买家。
`T03` 仍为 `IN_PROGRESS`，G03/G05 及整套 SaaS 上线门禁没有宣称通过。
