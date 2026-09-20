# 内部购物车与不可变报价验收

2026-09-20；后端基线 `e2cbfd5`。这是内部 Go/PG 切片，不是公开购物车、
订单结账、税务合规结论或生产上线许可。未连接任何客户库或外部支付物流服务。

## 实现边界

- `contracts/cart-quote-v1.md` 与 `0007_cart_quote.sql` 冻结稳定 Market、显式
  country-flat 运费/税配置、版本头 CAS、买家 owner 购物车及不可变 Quote。
- 买家命令 receipt 按 tenant/store/owner/session/operation/key 隔离；请求规范化
  后散列，同键异参拒绝，事务内同时提交业务记录、事件与 receipt。
- 购物车是绝对数量替换，最多 50 个不同 SKU，以版本 CAS 防覆盖；不扣库存。
- Quote 按 cart → market → policy head → 排序 product/SKU 加锁，读取锁后版本，
  保存名称、金额、版本与 DB 时间。读历史报价不会重新计算，也不会延长有效期。
- 报价资源按 owner 授权，创建 session 仅是来源；同 owner 的新会话可读取历史报价，
  但不能复用旧会话的命令 receipt。过期报价可读不等于可结账。
- 所有金额使用受限整数 minor units；税费采用合同明确的逐行舍入及运费税，
  不进行 FX、地区税法推断或“缺运费配置默认为零”。
- 新商户显式获得 pricing 权限；已有成员不自动加权。商家命令沿用既有店铺级
  receipt，actor 进入请求 hash，不将其误称为 actor 独立主键。

## Gate 与证据

`bash scripts/dev/test-local.sh` exit 0：独占、loopback、tmpfs PG18.6；
Go 1.27.1，`-race -count=1` 全套 **96 顶层 PASS / 0 FAIL / 0 SKIP**，随后 vet 通过。

|已测范围|关键反例|
|---|---|
|角色/RLS/FK|跨 owner、跨店、商家/issuer 越权、私有配置和凭证泄露、不可变报价写入均拒绝|
|原子性|事件插入失败、callback 失败、非法/超大结果回滚业务变更及 receipt|
|幂等/并发|8 路同键同请求得到一个 cart/version/event/receipt；不同请求冲突；购物车 CAS 仅一胜者|
|报价快照|等待 cart/market/policy/SKU 锁，读取新版本或停用后拒绝，不留下部分 Quote|
|历史与过期|价格/策略变更不修改历史；过期 replay 原 receipt，不重新延长时间|
|金额|现有政策的 inclusive/exclusive、显式零、舍入与上界；`TestPricingCalculation` 无 DB 也实际执行|

Root 工件：`/Volumes/data/output/live-commerce-cart-tests/root-lock-gates.log`；
SHA-256 `250cbebf2c289d1827c61e84509eae53ce81a25c88d44a634a2e4951ecf7b895`。
此前 `root-final2.log` 为 94 PASS，新增锁竞态与 8 路 replay 后以 96 项记录为准。
过期测试使用隔离库 owner 定向回拨合成 fixture 的时间及 receipt，不冒充生产时间流逝。

独立 SQL 审查 `b79844f` 与 Go 集成审查 `e2cbfd5` 在各自范围 P0/P1 均为 0。
审查提出缺少 cart/market/policy 竞态 gate 后，由 root 新增实测关闭，未放宽阈值。
Humaux：SQL `4b5261b7-a67a-40a3-9ab3-b9950ef81d7c`；集成审查
`8f77e313-8195-4aef-aaa9-e2124ea844ca`；root 验收
`6be611ad-555a-455a-b2bf-8e16499329da`。

## 修复记录与复用

- 初始新 SQL 使用错误的 buyer GUC；`b79844f` 统一为既有 `app.buyer_id`。
- PG 锁读要求 UPDATE RLS 可见性：允许受限 `FOR SHARE`，但 `WITH CHECK(false)`
  仍拒绝实际更新。immutable policy 先锁 head 再读版本，不给版本表 UPDATE。
- `root-final.log` 保留一次失败：FK 测试向原本应为空的共享 storeB 放了 fixture，
  破坏旧账簿断言；`35eccdc` 改为新建独立 foreign store，并未删除旧测试或改其预期。
- Pricing 作者 `7ad42eb`/`fa9b527` 在独立 worktree，经 root cherry-pick 为
  `78dca87`/`5950b80`；root 写迁移、buyer command 和 storefront；只读 agent 独立审查。
  代码使用现有 pgx、JSON/SHA-256 与 PG 事务，不新增服务、缓存或依赖。

## 下一步与禁止偷换

公开 buyer 入口仍需受信域名解析、已发布店铺、Cookie/CSRF/限流；买家 UI 未实现。
BeginCheckout 必须重新检查报价有效期、cart/catalog/market/policy/destination 版本，
并在一个事务内建立订单快照和库存预留。外部操作台账、PSP、退款、补偿、对账与
完整 expiry worker 尚未实现。`G03/G05` 及完整 SaaS 门禁不能标 PASS。
