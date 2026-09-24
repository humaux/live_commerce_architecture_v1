# 支付方式配置：内部验收

2026-09-24；实现 b49e6e2，完整测试基线 1b38df5。
合同：[payment-methods-v1](../../contracts/payment-methods-v1.md)。
本单元是内部商家配置和诊断，不是已开通收款或设置页面交付。

## 实现与边界

- PAYUNi 信用卡、分期、ATM、超商代码付款、LINE Pay 五个方法标识可保存
  独立的三语名称、排序、展示、金额限制及自有商户账户关联。仅支持 TW/TWD；
  金额为本地 minor units 限制，不声明供应商限额或传输单位。
- 复用 Scope、integration:manage/read、command 永久幂等与审计、pricing market。
  未新增依赖、支付计价器、队列或通用供应商框架；其他供应商仍待接入。
- 0015 使用不可变 revision + current head、完整租户/店铺/市场/账户/provider/
  environment 外键、FORCE RLS 及列权限。没有 buyer/checkout/worker 直接访问授权。
  RLS 约束可信服务的 scope；各商家操作权限仍由 Go 在重放前和最终等待后复核。
- 锁序 market→method→account→binding；账户轮换不改变方法语义版本。
  原连接的旧 binding revision 可继续编辑禁用草稿、隐藏或解除关联；新关联必须
  核验当前绑定版本，不能把其他店铺或 sandbox 凭据混到 live。
- `InspectMethod` 固定顺序独立列出版本、启用、隐藏、市场、环境、币种、金额、
  连接及绑定阻塞原因。它是商家诊断，**不是买家授权或 StartPayment 许可**。
  目前没有 adapter/商户资格准入证据，Go+SQL 均拒绝 enabled=true；所有诊断
  恒含 `ADAPTER_UNAVAILABLE` 且 Available=false。保存成功不等于已验证/可支付。
- 不读取账户密文、不改变 binding、订单或库存、不创建 provider/River 操作。
  不把超商代码缴费等同门市取货/COD，不默认分期期数或费用。

## 软件工程分工

|任务|角色与模型|范围/证据|
|---|---|---|
|只读复用定位|platform explorer，gpt-6-luna/medium|abaa318；Humaux《直播SaaS支付方式设置最小复用点 at abaa318》|
|合同/DDL/独立数据库测试|root integrator|861ffa2 / b93c0f3 / 1b38df5；共享文件唯一写入者|
|Go 实现|commerce worker，gpt-6-sol/high|base861ffa2；独立 worktree `/Volumes/data/worktrees/live-commerce-payment-methods`；仅两Go文件；作者17df487，合并b49e6e2|
|独立审查|security reviewer，gpt-6-sol/high|合同05236c63-c635-452d-ab96-880ce7123c9a；最终173155ca-4ebc-43d3-8216-90d80faf39c2，无可证实P0/P1|

模型配置取父任务实际 spawn 参数；作者回执说运行时未自报模型，并不意味着默认克隆。
作者同包 race/vet 通过；root 独立运行真实数据库全回归。审查者检查代码、测试因果、
日志计数及哈希，没有另行复跑 PG。所有环境使用独立虚构数据，无真实商户密钥。

## 实测与证据

日志目录 `/Volumes/data/output/live-commerce-payment-method-tests/`。

|命令|结果|证据|
|---|---|---|
|`bash scripts/dev/test-local.sh`|exit0，210顶层PASS / 0FAIL / 0SKIP；真实PG18.6，Go1.27.1 race+vet；foundation89.502s|root-initial.log|
|`bash scripts/dev/test-local.sh --browser-identity`|exit0，1PASS，foundation4.553s；真实Chromium→Next→Go→PG，签名MOCK IdP|browser-identity.log；output/playwright/identity-chain-20260924T103355.451408000|

210 = 之前199 + 本单元4个同包测试 + 7个PG顶层测试，不是210项业务功能。
浏览器测试仅证明既有身份链兼容，不是未实现的支付设置UI或支付流程验收。

```text
ebf0ed5d1bbdb89fd5260b331a180ef5f80abeef2a1437aa42bc8bdb372493ab  root-initial.log
f468ce15284c7d7c5ac7bcd5d8cae08ae84833549bb3085219f1f45a48a111e4  browser-identity.log
```

测试包括：五方法写入读回，独立展示、历史重放与修改冲突、跨店/租户/账户/环境
拒绝、LIVE禁用草稿与SANDBOX分离、金额边界和独立原因、凭据旋转与绑定停用、
inactive市场/stale绑定仍可隐藏解绑、同键并发与CAS单赢家、SQL直接启用拒绝、
不可变历史及worker无权、按具体SQLSTATE+constraint验证失败原因。
撤权测试先用 `pg_blocking_pids` 证明真实等待，再撤销权限并解锁，验证结果拒绝
且配置/审计/回执均未提交。四类表 × create/update 的8种写入故障，通过独立
非事务 sequence 证明实际命中注入点，再验证零部分写入；不是只凭有错误就通过。

两次脚本只清理自身带标签的临时PG容器；10:35 UTC复核fixture列表为空。
保留日志、浏览器工件及作者worktree，不清理用户数据或其他任务资源。

## 下一交付与仍未验收

PAYUNi hosted UPP协议、操作范围内凭据读取、StartPayment与永久交易标识、
验签回调/查询/退款/对账仍未实现。后续不能只删除 enabled 的SQL限制来开放收款，
必须同时接入可证实的adapter/环境/商家资格与下单最终重验。

三语设置HTTP/UI、买家支付选择及实际支付浏览器验收、生产密钥装配、
供应商sandbox/live与完整G03/G05/MS01–MS08仍未通过。不修改客户现用平台
开关、直播、订单、物流或资金；完整SaaS尚不具备上线验收结论。
