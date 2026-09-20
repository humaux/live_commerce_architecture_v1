# 首个 Go/PostgreSQL 开发切片验收

验收时间：2026-09-20（北京时间）。结论：**PASS_FOUNDATION_ONLY / REAL_PG**。
不是完整 SaaS 验收、性能压测、浏览器验收、供应商联调或生产上线。T00–T03 保持 IN_PROGRESS；G01–G15 保持 NOT_RUN_PRODUCT。

## 已有可执行实现

- `cmd/api`：Go HTTP 服务；健康、数据库 readiness、授权店铺读取、店铺审计读取四个接口。
- `internal/platform`：数据库内解析的 merchant 会话、显式店铺权限、事务级 Scope、5 秒查询预算；拒绝不安全的数据库登录角色。
- `migrations`：租户、店铺、主体、会话、成员权限、追加式审计；复合外键和 FORCE RLS。迁移与运行身份分离，checksum 防漂移、并发迁移忙锁快返、River 上游迁移可重入。
- `scripts/dev/test-local.sh`：只创建带唯一标签的临时 PG18.6，绑定 localhost 动态端口、512MiB 内存限额、tmpfs 数据；不使用现有业务 DATABASE_URL，结束移除。
- GitHub Actions 配置：固定 action commit 与 PG 镜像 digest，执行同一验收脚本。**配置已写，远端 CI 未运行**。

## 实际执行回执

|检查|实际结果|证据|
|---|---|---|
|真实 PG18.6 + Go race|4 个基础单测、11 组 PG 集成测试全部通过，无 SKIP，exit 0|[原始命令回执](../../evidence/2026-09-20-foundation/real-pg-pass.json)|
|go vet|同一隔离脚本实际执行，exit 0|同上；set -e 下只有两步均成功才输出最终 PASS|
|govulncheck v1.1.4|Go1.27.1 / x/text0.39.0 下 No vulnerabilities found，exit 0|[扫描回执](../../evidence/2026-09-20-foundation/vuln-pass.json)|
|独立复核|实现与测试分工；迁移并发连接 P1 修复后由非作者复读；主线独立完整复跑|本节角色与修复记录，Humaux 对应记忆|
|资源回收|`docker ps -a --filter label=livecommerce.fixture` 无结果；原 humaux-thread-pg / qdrant 仍运行|[资源回执](../../evidence/2026-09-20-foundation/postflight.json)|

测试证明的边界：双租户/同租户双店授权、直接 SQL 的 RLS 拦截、buyer audience 不可访问 merchant API、过期/撤销会话、撤销 grant、跨租户复合外键、auth 表不可读改、审计不可改删、异常/panic/取消回滚、池复用无 Scope 残留、低权限登录实际 `current_user`、MaxConns=1 迁移及锁竞争回收、5s/1s/5s 数据库超时设置、HTTP 不信任 Host/X-Tenant/Cookie、审计列表固定 50 条、audit 与 River 任务共同提交或共同回滚。

这些测试**不证明**完整任务队列租户隔离、库存不超卖、支付/退款对账、外部副作用幂等、IdP 登录、会话四域业务或商家实际经营效果。

## 验收中发现并修复的问题

1. **River 迁移事务边界**：全量 MigrateTx 在新 enum `pending` 提交前使用它，PG 报 55P04。改用上游 Migrate 的逐步事务；业务 checksum 迁移单独提交，统一 session lock 覆盖整个过程。
2. **并发迁移连接上限**：先 Hijack 再等锁可使等待者突破池上限。改成池内 try-lock、输者 Release+ErrMigrationBusy、仅赢家 Hijack；查询响应丢失时关闭连接，防锁残留。MaxConns=1 与锁竞争反例实测通过。
3. **测试连接身份错误**：修改 pgx Config 字段后 ConnString 仍返回原 owner URL。独立测试作者改用 url.UserPassword 重建 synthetic URL，回读 current_user；没有放宽生产 OpenPool 的拒绝逻辑。保留[首次失败回执](../../evidence/2026-09-20-foundation/first-integrator-pg-run.json)。
4. **River 入队权限**：上游 InsertTx 使用 ON CONFLICT DO UPDATE SET kind，因此 INSERT 权限不足。仅额外授 UPDATE(kind)；state 更新和 DELETE 仍在真实 PG 被拒绝，不授 worker 全表 UPDATE。
5. **已知依赖漏洞**：初始 Go1.26.2+x/text0.38 扫描报告 9 个可达漏洞。固定本项目 Go1.27.1+x/text0.39 后重扫通过；未替换系统 Go，仍需后续持续扫描。

## 交付分工与证据等级

- 实现子任务：commerce_foundation_impl / gpt-5.6-terra high，提交 02fac70、4439c29。
- 独立测试：commerce_contract_review / gpt-5.6-sol high，提交 799d319、6e5ec66；未修改生产 SQL/依赖来让测试变绿。
- integrator：冻结共享合同/迁移/依赖，修复迁移，补直接 SQL/HTTP 验收，独立复跑最终合并版本。
- 非作者迁移审查：commerce_foundation_impl 提出并复核连接上限 P1。Humaux 记忆标题 `Migration lock pool-cap P1 resolved at c27a2c1`。
- 当前源码 SHA256 与角色/版本记录分别见[文件摘要](../../evidence/2026-09-20-foundation/source-hashes.json)、[开工记录](2026-09-20-kickoff.md)。不把文档 gate 当产品 gate。

## 下一阶段与停线

先冻结 T01 的 SKU / Quote / Reservation / PaymentFact / Refund / PickupSelection 正式 schema，再实施商品库存→报价→模拟支付→订单/退款的可验收闭环；不继续照抄旧 core-openapi 的 DRAFT 泛化会话接口。

下一阶段必须交付：库存 1 的双连接并发仅 1 成功、同键异请求拒绝、expiry/支付成功两种交错、迟到付款补偿、并发部分退款不超额、业务事件/River 原子提交、重复回调/UNKNOWN 不重复副作用。四域会话正式合同和超商 nonce/门市字符串/线路资格也需独立反例，不能因本次基础层通过而省略。

仍未实施：商品/购物车/结账/退款、店铺配置写入、完整员工/买家/访客登录、域名解析、站内聊天/Meta会话/平台支持、超商回填/入台承运商、媒体直播、商家后台/平台后台/独立站三端 UI。真实支付、物流、Meta审核等外部资格继续单独核实；这些不阻止下一轮明确标记的本地开发。

生产停线：未修改客户 SHOPLINE、直播、订单、DNS、现有数据库；未发布消息、广告、付款退款或面单。上线前必须另过全局产品门禁并取得具体生产操作授权。
