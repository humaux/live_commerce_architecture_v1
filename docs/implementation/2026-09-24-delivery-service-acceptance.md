# 商家配送服务配置：内部验收

2026-09-24，代码及最终测试基线 `2a5e43c`，Go 实现 `80ca6bb`，迁移/合同
`c609fe7`。本单元支持版本化商家配送配置及独立运费报价，**不是已接通物流、
收款、公开配置页面或买家下单**。全局产品 gate 不因此提升为 PASS。

## 实现与边界

- [合同](../../contracts/delivery-service-v1.md)：`fulfillment.SetService/GetService`
  使用既有 merchant scope、权限、幂等和审计，同一调用方事务提交。
- 0010 增加不可变 service revision 和 CAS head；复用唯一 `pricing.Calculate`，
  通过受限 `delivery:<code>` 为两个相同配送类型的服务配置不同运费。
  Quote 改用同一 `pricing.ValidMethod`，没有重复校验闭集。
- Enabled、Visible 分开保存；MANUAL 不要求虚构承运商；API 只能保存禁用草稿，
  Go 和 SQL 均拒绝启用。关联 binding 不表示已具备承运／供应商能力。
- 旧政策或市场停用时，允许保留原配置精确关闭服务；不可借关闭换成其他历史
  政策、改名称／排序等。锁序为 command→market→policy→service→binding。
- 无新依赖、供应商调用、支付／物流任务、客户生产数据或 UI 改动。

## 分工与独立复核

作者 `t06_operation_service` 在专用 worktree
`/Volumes/data/live-commerce-delivery-config-20260920`、分支
`commerce/delivery-config-20260920`，base `c609fe7`；角色 commerce_worker，
`gpt-5.6-sol` / high。写入仅限 fulfillment Go 与 pricing 校验及同包测试；
作者 `24abda0` 合并为 `80ca6bb`。共享迁移、Quote 接入及真实 PG 测试由 root 负责。

早期 reviewer 提出三处测试缺口：Unicode 输入、精确 authority 错误、分别停用
market/policy。root 补齐后执行原完整命令。早期代码 reviewer 因额度中断，
没有把其未完成结论当 PASS。

最终 `delivery_final_review`，security_reviewer、`gpt-6-sol` / high、只读审查
上述迁移/代码及 root 测试；独立同包测试 exit 0，P0=0/P1=0。
Humaux：`2ca0099c-8ed0-497a-b2ab-1460e6b8830f`。作者不是唯一验收者；
root 独立运行真实 PG 和浏览器回归。没有声称这等于统计独立安全证明。

## 实际 gate

日志目录：`/Volumes/data/output/live-commerce-delivery-service-tests/`。

|命令|结果|日志|
|---|---|---|
|`bash scripts/dev/test-local.sh`|exit 0；Go1.27.1 / PG18.6；138 顶层 PASS、0 FAIL、0 SKIP；race + vet；foundation 74.357s|`root-final-20260924.log`|
|`bash scripts/dev/test-local.sh --browser-identity`|exit 0；真实 Chromium→Next→Go→PG，签名 MOCK IdP；1/1 PASS|`root-browser-20260924.log`|

```text
377a66fbc67e3a1598a90dd90c2fcd1f39d4723fc176f7a787cd1cbc5af3d733  root-final-20260924.log
2fb60609407a7af76778113aafc7891ebea4ca3c4275d90a169414b050423bdb  root-browser-20260924.log
```

138 包含既有 128 项、4 项同包测试和 6 项新 service 集成测试，不是业务功能数量。
浏览器日志工件：`output/playwright/identity-chain-20260924T080335.949765000/`。
这是既有商家身份链兼容回归，不是尚不存在的配送设置／买家 checkout 页面验收。

覆盖：创建/读回/版本修改/历史回放、两个 home 服务不同报价、并发同 key 仅一次
及同版本 CAS 单赢家；policy-only、market-only、both 精确停用；非法输入、跨店
binding/过期 binding、三语名称；跨租户/店铺/actor、缺读写权限、撤权后回放、
实际 GUC mismatch；buyer/worker/issuer SQL 拒绝；API-enabled、NULL binding
version、外键及 ASCII 空白/控制字符/超长名称 SQL 约束；审计或 receipt 故障
完整回滚 history/head/audit/receipt。API 草稿保存不新增 integration operation。

## 明示 P2：SQL Unicode 不等价 Go

正式 SetService 对无效 UTF-8、Unicode 空白和非 printable 格式字符均拒绝。
SQL 额外实施长度、ASCII 空白和控制字符约束，但不复制 Go 的全部 Unicode 表。
可信 runtime 直接写 SQL 仍可写入部分 Go 拒绝的不可见名称；这不是公开写入口，
不影响 API-disabled、scope、binding/FK、金额或 checkout 权限约束。

隔离 PG18.6 探针（`unicode-pg18-probe.log`）显示 U+200B/U+200D、私用字符即使
在 `pg_c_utf8` 下也被 `[:print:]` 接受，故未做正则替换伪修复。
[PostgreSQL 官方字符分类说明](https://www.postgresql.org/docs/18/collation.html)
区分其 Unicode POSIX 兼容规则；不能假定它等于 Go IsPrint。
新增写入口或开放更多 SQL 使用方时须重新评审此边界，不能绕过 Go 校验。

## 下一单元与未完成项

按 [buyer-checkout 合同](../../contracts/buyer-checkout-v1.md)继续冻结／实现
destination 来源与版本、服务端仓库 allocation、独立 checkout runtime/writer、
订单 DRAFT + HELD、generation-aware 到期任务；复用现有库存账簿。
随后接入 StartPayment、第三方适配器及公开三语 UI／浏览器验收。

商家连接自己的支付／物流账户、第三方 sandbox/live、选店正式数据源与回填、
完整订单支付物流，以及全 SaaS 上线均仍待完成。测试 fixture 已按脚本移除，
额外无网络 Unicode 探针容器也经精确标签确认后移除；日志保留。未影响客户业务。
