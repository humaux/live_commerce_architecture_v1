# PAYUNi wire：协议层验收

2026-09-24；作者3497645，主线实现36e1897，独立测试5689c50，金额边界e69d5cb，
最终测试基线31dac4a（含隔离PG就绪竞态修复）。
合同：[payuni-wire-v1](../../contracts/payuni-wire-v1.md)。只接受
`PROTOCOL_MOCK` 结论，不是商家沙箱、收款或完整结账验收。

## 交付和依赖

- `internal/integrations/psp/payuni`：五种 UPP 托管表单、回调验签与绑定校验、
  单次查询、有限字段 Observation。表单构建不发请求；没有 Paid 标志或数据库写入。
- 无新 Go 模块/PSP SDK；标准库 crypto/HTTP/form/JSON，固定官方端点、TLS1.2+、
  10秒、无自动重试/重定向、响应限量、敏感错误不透传。
- 准确绑定金额、交易号、支付方式、网关与分期期数。结果保留取号/未知/待确认/
  查询不完整等状态，不根据 SUCCESS 字样入账。供应商固定 IV 仅用于该 wire，
  不能取代本地凭据库随机 nonce/AAD 的存储加密。
- 独立 Node24.15.0/OpenSSL 检查仅是测试依赖；已有前端运行时，无新增 npm 包。
  全回归脚本在创建 PG 前检查 Node，缺依赖时不跳过该门禁。
- 合同要求后续调用者从原始支付尝试冻结的 tenant/store/account/environment/
  credential-version 选择 Client；不能按未签名 MerID 或当前活跃密钥猜测归属。
- `AmountTWDFromMinor` 明确将本系统两位TWD minor units精确除100得到供应商整数元；
  非TWD、零/负数或有余数拒绝，不舍入、不改买家价格。已核本地Intl三语言结果及
  [SIX官方ISO4217列表](https://www.six-group.com/dam/download/financial-information/data-center/iso-currrency/lists/list-one.xml)的TWD minor units=2；转换不代替方法限额/准入校验。

## 分工与修复

|工作|角色/模型|证据|
|---|---|---|
|官方文档复核|integration explorer，gpt-6-sol/medium|Humaux 9b53bd52-2edf-4c61-aad2-0b3bc7e53617；UPP34、query164、crypto29/312|
|合同/独立验收|root integrator|b6a443c/5689c50；共享文档唯一写入者|
|Go 实现|integration_worker，gpt-6-sol/high|base b6a443c，独立 worktree `/Volumes/data/worktrees/live-commerce-payuni-wire`，仅client.go/client_test.go；作者3497645|
|独立审查|security reviewer，gpt-6-sol/high|31dac4a代码/测试/合同及完整日志复核；Humaux c8f4ebd2-4f49-49be-9ffc-380f9fc45818，无可证实P0/P1，已更正旧预检缺口|

审查发现的未来 attempt→准确凭据选择合同缺口已补齐。实现修复了首尾空格
HashKey/IV 与官方 PHP SDK trim 行为不一致、以及官方 outer `Unapproved` /
inner `UNAPPROVED` 大小写不同两点；独立测试覆盖，未放宽身份/金额校验。
审查者没有另外复跑数据库，作者不是唯一验收人。
审查者独立复跑e69d5cb同包race/vet通过；最终全仓日志计数/哈希已独立核对。
下一步复用盘点的TWD尺度口误已由实测纠正并supersede，正确记录
`5464609c-2fac-4e89-96ea-c4b6aee092a9`，不能引用旧记录的“0位”结论。

## 实测

`go test -race -count=1 ./internal/integrations/psp/payuni`、同包vet：exit0，16顶层PASS。
包括作者6项、root独立9项及金额转换1项；不是16种业务功能。

最终 `bash scripts/dev/test-local.sh`：**exit0，226顶层PASS / 0FAIL / 0SKIP**，
真实隔离PG18.6，Go1.27.1 race/vet，foundation94.475s。原始日志目录：
`/Volumes/data/output/live-commerce-payuni-wire-tests/`。

过程保留：5689c50的root-full.log为225项，终态成功标记可核但续跑后shell句柄已回收；
e69d5cb的root-final.log在createdb阶段exit1，未运行业务测试，不能套用旧通过结果。
根因：固定镜像entrypoint先启动socket-only临时PG，再关闭重启正式PG；socket
pg_isready误判导致createdb遇到关闭窗口。31dac4a将test-local/admin-fixture两处
检测改为`pg_isready -h 127.0.0.1`等待最终TCP服务，不加盲等、不放宽测试。
镜像原始entrypoint289–311及335–375已核。修复后完整重跑root-final-fixed.log。
admin-fixture额外启动成功，GET /healthz为200且status=ok；主动CtrlC退出130后
自有容器/临时文件已由脚本回收。这是后台fixture冒烟，不是新增支付页面浏览器验收。

```text
07a3cab783fddc05e78cc6c90b7ef01567517e526f0a062592354adb0df2fe0e  root-full.log
fa41cab3bb49eded3557f2293f0c346a25ae5b53c5512e27d7c820a747da4e1d  root-final.log (FAIL_BEFORE_TESTS)
dded891db4c29967a49ab2a74359245c5786cd6dc56863229e5be7010f854701  root-final-fixed.log
b434c1f0b86c8869f22c6f7016d44de74727645bafa1a5740c1c32295cc18da3  admin-fixture-smoke.log
```

226 = 之前210 + 本单元16项。包含：官方页312固定向量、独立测试编码器、
五方法×两环境共10组Go表单由Node解密并逐字段核对，禁止隐式开放其他付款方式；
输入/台湾日期/金额/期数边界；真正签名的错商家/错单/错金额/方式；UNKNOWN/
取号/待确认原样保留；重复转义字段、坏UTF8/编码、超量、坏hash/GCM；查询只用
一个交易标识、单条Result、多条/扁平结果拒绝、完整字段A/B投影；响应大小和
关闭、单次调用、重定向拒绝、取消保留、错误脱敏。HTTP全由本地RoundTripper
拦截，LIVE只测试端点字符串；没有实际访问供应商或创建交易。

无需新增浏览器截图：本次没有页面/HTTP handler改动。没有把既有身份链浏览器
兼容测试当作未实现的支付UI验收。自有fixture容器检查为空；保留日志和worktree。
图谱提交4文件、解析3个Go文件、120实体、0拒绝，主要符号关联修复记忆。
金额/fixture补充提交4文件、解析4文件、7实体、0拒绝，AmountTWDFromMinor关联单位修复记忆。

## 未完成边界和后续验收

- Query的`Result[0][Field]`字面编码来自官方数组说明与PHP parse_str推断，尚无
  真实商家原始响应；缺字段的DataSource=B会保守拒绝，不宣称兼容全部处理中响应。
- 官方回调ACK/重试规则、旧密钥通知/轮换查询行为未确认，不猜测ACK或自动换钥。
- 没有生产凭据读取/加载、durable attempt、StartPayment、付款等待库存转换、
  通知HTTP/inbox/幂等入账、退款/对账、真实商家资格和sandbox/live验收。
- 五种支付方式仍为禁用草稿，诊断`ADAPTER_UNAVAILABLE`/Available=false不变。
  开关可用需以上准入与三语设置/买家支付完整浏览器门禁，不能只删除SQL限制。
- 后续先完成内部事务及范围准确的凭据使用，再装配公开页面与供应商沙箱。
  超商代码缴费不是超商取货/COD；跨境物流和真实门市地图仍是独立接入工作。

客户现用平台、直播、订单、物流开关和真实资金均未变更；完整SaaS仍未可上线验收。
