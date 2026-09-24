# 买家开始支付：内部事务验收

2026-09-24；合同 `0d5e509`，集成代码及测试 `eff808f`，最终完整验收通过。
范围：[payment-start-v1](../../contracts/payment-start-v1.md)，首期信用卡的内部
Go → 普通 checkout SQL 权限 → PostgreSQL/River 事务，不是公开结账或真实收款。

## 实现与边界

- 买家能力鉴权、owner级永久幂等，同键同请求复用，改参数/执行环境冲突。
  新会话可接续同owner订单，预留创建会话与本次支付会话保留各自来源。
- 锁订单、预留、市场、方法head、账户、绑定、资格，冻结已有订单金额、方法版本、
  账户/凭据版本、资格及保留完整128位的随机交易参考号；不接受买家金额。
- 同事务创建attempt、UNKNOWN query-only operation、事件、回执、River任务，
  订单转AWAITING_PAYMENT、预留转PAYMENT_PENDING并推进代际。旧到期任务不能释放
  待支付库存，异常整体回滚。
- BUYER_PAYMENT_QUERY使用NULL商家principal及明确buyer/attempt外键，不伪造会员。
  普通merchant Dispatcher/Get排除此family；复用Claim的lease/token/generation，
  仅允许reconcile；停用新收款不阻断历史目标查询。
- 0015的CHECK(NOT enabled)明确替换为启用必须引用同范围资格证据的约束。
  没有应用资格签发权限，SetMethod(enabled=true)仍拒绝；测试owner只向一次性PG
  注入PROVIDER_MOCK，SANDBOX/LIVE配置拒绝该证据。
- 无真实资格签发服务、表单释放、查询worker、历史凭据读取、回调、入账或退款。
  本单元不注册payment_query_v1消费者，不开放生产入口；不能上线收款，不能把
  UNKNOWN或操作状态当作钱已收到，也不能据此释放库存。

## 根因和有效反例

独立审查发现“下单后商家停用市场”仍可能发起新付款，SQL补市场SHARE锁和active
检查，含预停用和实际等待中停用测试。既有价格仍来自订单，不在此重新计价。

首次迁移失败是PL/pgSQL条件内CASE比较缺括号，修正后迁移成功。首次focused两项
失败是测试错误：session已被resolve_scope持SHARE后又同步撤销，却不释放订单锁，
形成测试锁环；改为预设600ms有效期、证明实际阻塞且仍有效、等DB时钟过期后解锁。
撤销本就会排在持SHARE的事务后。ACL名称查询遭schema42501，改由owner仅解析OID，
再由普通角色检查权限，没有授予新权限。

另用checkout.events的ACCESS EXCLUSIVE锁制造资格检查后的写入等待，在资格仍
有效时确认阻塞，等DB时间过期后释放；最终检查须回滚attempt、operation、订单和
任务。六处故障用非事务sequence证明真正到达注入点，不接受无关早期错误冒充通过。

## 实测

日志目录 `/Volumes/data/output/live-commerce-payment-start-tests/`。
PG18.6固定镜像、Go1.27.1；真实隔离数据库，外部供应商请求0次。

|阶段|结果|日志|
|---|---|---|
|首次完整命令|exit1，迁移语法失败|schema-baseline.log|
|修语法后完整命令|exit1，foundation包120s超时；无功能断言失败，不算通过|schema-second.log|
|首次--payment|exit1，session测试锁环及ACL名称解析；9.696s|payment-first.log|
|修测试后focused|exit0，7顶层PASS/0FAIL/0SKIP；10.025s|payment-second.log|
|补强后完整命令|exit1，245顶层PASS/1FAIL/0SKIP；107.615s；旧测试只认已替换的约束名|root-full.log|
|更新约束名后完整命令|exit0，246顶层PASS/0FAIL/0SKIP，race+vet；foundation106.688s|root-final.log|

246=既有235+作者2个unit+root9个真实PG顶层测试，不是246项业务功能。
PS01–PS05的本地边界均通过：双键单赢家、同键同结果、全部原子事实、故障命中后
回滚、scope/环境/金额/资格拒绝、实际等待后的时钟检查、新旧会话来源、旧到期
栅栏，以及把buyer operation塞给真实merchant River worker仍零适配器调用。

失败的旧测试 `TestPaymentMethodsSQLGuardAndScopedTargets/enabled` 仍要求新建未
获资格的方式被SQL拒绝（23514）。只把旧约束名method_not_admitted改成0016的
method_admission_reference，并保留因果校验，没有接受任意错误或放宽启用条件。

串行套件增长且既有进程崩溃恢复测试耗35.20s。经独立审查，仅把整个包runner预算
120s→240s，不改case锁超时、业务期限、竞态断言或故障门槛。120s的--payment
定向诊断不替代完整race/PG/vet。

```text
531472e73100389904276a3c75a997ea5e1af3d27f69768fec598be651009de3  schema-baseline.log
b86008cd05a893fdb2994fea6e4b65f2f9409b3c89addebb635f9442b267b1dc  schema-second.log
1144f47d59040d07cee71b09e373b677b2db2e95d2b2464e78160c1b0c3ddb05  payment-first.log
847a34bad995dac2fc2f1b82abbfde86d176e131e926038e3245f6c1b1a07eab  payment-second.log
f4f6564447380365a9ae7bf5ead6cb7d0206ffb724e19a38bf45a7733fb1e12a  root-full.log
f8c7328c5b1a9de0d8b3091d9664f2cdce831866d23531cc58180cb6f0a1033b  root-final.log
```

## 分工与剩余项

- root：合同、0016、core family筛选、独立真实PG测试、完整回归与文档。
- author：实际spawn为gpt-6-sol/high，base0d5e509，工作区
  `/Volumes/data/live-commerce-worktrees/payment-start-20260924`，分支
  `codex/payment-start-20260924`，提交edfe75d，仅payment.go/payment_test.go。
  自报medium与调用参数不符，以调用记录high为准。作者同包race/vet exit0；root先
  no-commit应用，完整审查后再集成。无递归委派。
- reviewer：gpt-6-sol/high，只读合同/SQL/Go/测试，独立运行checkout/core包race；
  最终复核完整日志/hash及9项PG测试，无P0/P1，Humaux
  `ef135ed8-f7cd-4958-bbc1-c08ab3f3cf7c`；没有另行跑完整PG。代码只在绿测及
  终审后提交。自有fixture标签查询为空，保留日志与作者worktree。

PS01–PS05仅内部gate，不是PSP或全SaaS gate。无前端变更，不宣称浏览器支付验收。
下一步：历史凭据读取、verified observation、query worker与回调入账，再释放
托管表单/公共入口及其他支付方式。ATM/CVS期限和分期选项未决定前不默认启用。

## 可追溯性

主线增量索引7个变更文件，其中6个Go文件解析出176个实体，SQL没有生成代码实体；
索引完成且无拒绝项。`NewPaymentStarter`及最终写入等待资格过期测试已关联Humaux
修复记录`0b31d03d-e13b-407a-9e79-01a408d6f426`，代码图谱锁已释放。
资料包结构检查和本地Markdown链接检查单独执行，不能替代上述真实PG运行证据。
