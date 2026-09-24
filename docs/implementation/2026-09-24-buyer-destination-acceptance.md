# 收货目的地与门市来源：内部验收

2026-09-24，最终代码／测试基线 `5cefa83`；合同与迁移 `f974391`，
门市来源 `8004778`，买家目的地 `00c20a5`。
本单元只实现结账前置数据和重验，**不是完整 checkout、正式门市地图、物流或支付接通**。

## 交付与调用关系

- [合同](../../contracts/buyer-destination-v1.md)／迁移 0012：门市不可变来源版本和
  当前 head、买家不可变收货快照和当前 head、只存 ID 的选择事件。
- `fulfillment.AttestPickup/RevokePickup` 复用商家权限、`command.Run/Audit`
  和 pgx 事务；鉴权先于历史重放，主体进入请求摘要，CAS 后追加版本。
  `MANUAL_ATTESTED` 是人工核验，不是官方目录验证；同一记录可服务多个买家，
  不要求逐单审批。代码及 namespace 原样保存，`017888` 不丢前导零。
- `ReadPickup/LockPickup` 的买家投影不包含核验证据引用或商家主体；当前来源
  必须启用、版本匹配且未过期。买家可加行锁，但连 no-op source UPDATE 也被拒绝。
- `storefront.SetDestination/GetDestination/RevalidateDestination` 复用
  `buyer.WithScope/CheckScope/RunCommand` 与现有购物车锁，不新增会话或计价系统。
  宅配地址由买家填写；超商地址只能引用同店可信来源，不接受买家复制的“已验证”地址。
- 命令回执只保留 `{id,version}`，个人信息在 owner 隔离的快照中。改选用 CAS，
  历史可读但不能作为当前选择；同买家另一有效会话可读历史，其他买家／店铺不可读。
- 锁顺序 cart → pickup source → destination head；后续 checkout 继续 service →
  allocation → warehouse/balance。最终数据库时钟在等待后再取；买家快照最长
  30 分钟且不能超过来源有效期。将来 checkout 若还有等待必须再次核时。
  普通买家直接写入的意图／回执不是库存保留或扣款授权。

## 分工与独立复核

两个独立 worktree 作者只改各自两份 Go 文件：来源作者原提交 `0c80ba9`，
目的地作者原提交 `ee6c3bc`；root 负责合同、DDL、合并和真实 PG 集成测试。
作者同包 race/vet 通过，不代替真实数据库验收。

独立静态复核 `af99a717-60bb-4c7f-b2a7-a17a93cb0d6a`：P0=0/P1=0。
测试复核 `a4db6f0c-7828-4366-a78d-76aa8ae4e58b` 提出来源 CAS、直接买家 SQL、
跨主体／GUC 回放三个缺口，root 在 `1d3b94a` 补齐并重新运行以下门禁。
复核随后指出两个 expired 测试会因选择时间早于来源核验时间而先失败，不能隔离
目的地过期谓词。`5cefa83` 改为来源核验后 1µs 选择、再过 1µs 到期，并用实际
LockPickup 与 SQL 时钟确认来源仍有效、仅目的地过期，再重跑完整门禁。
没有让作者自验代替独立审查。

最终独立复核 `6f4d72d0-4c5a-4be0-b8a5-127399cf42e0` 在 `5cefa83` 确认三项
覆盖缺口及上述因果假通过关闭，限定范围 P0=0/P1=0；独立读取最终日志并核对
164 PASS／0 FAIL／0 SKIP、77.527s 和 SHA256。未提升 checkout／供应商门禁。

## 实际验收

日志目录：`/Volumes/data/output/live-commerce-destination-tests/`。
运行时核对：Go 1.27.1；固定容器镜像的 PostgreSQL 18.6。

|执行|结果|日志|
|---|---|---|
|`bash scripts/dev/test-local.sh` 首轮|exit 0；foundation 77.115s；补测前结果保留|`root-initial.log`|
|补三组覆盖后轮|exit 0；164 顶层 PASS / 0 FAIL / 0 SKIP；race + vet；foundation 79.550s；尚未隔离上述因果缺口|`root-final.log`|
|因果修正后的最终轮|exit 0；164 顶层 PASS / 0 FAIL / 0 SKIP；race + vet；foundation 77.527s|`root-final-causal-expiry.log`|
|`bash scripts/dev/test-local.sh --browser-identity`|exit 0；真实 Chromium → Next → Go → PG，签名 MOCK IdP；1/1 PASS|`browser-identity.log`|

```text
ee44d7d48a4ee111417299173359fe5558907c40a2e3711a94aa3d79960a6be2  root-initial.log
79334c7fb24ec0e8a2671b9802683a932a7cc5a83865cab282cde77aca9ec9ee  root-final.log
8e5b312b21ea7df763e77c874feff465ef720843611071f466bc5cb8935a19c2  root-final-causal-expiry.log
eae0a34d3fceb60dd3463c47717c2d12e6f9b13137f7ffa2a89f46f8209e70f3  browser-identity.log
```

164 = 既有 150 + 4 个同包单元测试 + 10 个真实 PG 顶层测试，不是 164 个产品功能。
覆盖来源版本／撤销／重放、字符串店码和 namespace、home/CVS、回执隐私、跨买家／
会话／店铺、SQL 权限和复合 FK、双方 CAS／同 key 并发、cart/source 漂移、
event/audit/receipt 故障原子回滚、source/destination/cart 行锁持有至事务结束。
过期竞态测试先从 `pg_stat_activity` 观察真实锁等待，确认尚未过期，再以 DB
时间等待越过截止点，验证 Set 和 Revalidate 都拒绝；不是仅靠宿主机 sleep。
普通 buyer runtime 直接插入自己的 future/expired/revoked/beyond-source 行后，
重验拒绝；超过 30 分钟 TTL 则在 SQL CHECK 层拒绝。

浏览器工件：`output/playwright/identity-chain-20260924T090258.286536000/`。
无前端改动，所以这是既有身份链兼容回归，**不是尚未实现的选店／支付设置 UI 验收**。
各轮临时 PG 由带所有权标签的脚本回收，事后标签查询为空。保留日志与独立工作树。

## 未完成与继续顺序

后续更新：内部 DRAFT+HELD、锁定消费与 River 到期释放已在 `162b619` 实现并
[单独验收](2026-09-24-buyer-checkout-acceptance.md)。以下为本前置单元交付当时的边界，
不应据此判断后续 checkout 仍未实现；正式运输、支付和公开 UI 的缺口仍保留。

真实买家下单 DRAFT+HELD、锁定消费以上前置数据、到期处理、支付发起和库存竞态仍待实现。
正式门市目录／地图回调、运行时运输能力、COD、商家自有第三方账户连接、支付方式、
PSP／物流 adapter、sandbox/live、公开三语设置和结账页面仍为 NOT_RUN。
个人信息保留／删除、生产静态加密与商家订单个人信息权限仍需独立上线门禁。

用户截图对应的[服务设置覆盖表](../../contracts/merchant-service-settings-v1.md#6-实现覆盖核对2026-09-24)
明确这些缺口；人工方式不能替代第三方 API。没有改变客户现平台开关、直播、订单、资金或物流。
