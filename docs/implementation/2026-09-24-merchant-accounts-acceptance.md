# 商家自有供应商账户凭据：内部验收

2026-09-24；实现 bbcbdf0，完整测试基线 3063ed7。
范围是 PAYUNi 商家账户凭据的内部登记／存储／轮换，不是收款接通或自助配置页面。
合同：[merchant-accounts-v1](../../contracts/merchant-accounts-v1.md)。

## 实现与权限

- `accounts.Service.Create/Rotate/Get` 复用现有商家 scope、integration权限、
  `command.Run/Audit` 与 `core.Service` binding，不开新微服务或任务系统。
- MerID/provider/environment 不可原地换绑；同店同商户同环境唯一；SANDBOX/LIVE
  分开。SQL复合 FK 防止绑错其他商户／环境／店铺，延迟版本 FK 保证头与密文原子提交。
- 密钥以 Go 标准库 AES-256-GCM 随机 nonce 密封，AAD 绑定租户／店铺／账户／
  环境／版本。SQL从不接收明文。不可变历史密文禁止普通角色 UPDATE/DELETE；
  merchant runtime 仅可 SELECT 凭据元数据列，不能 SELECT nonce/ciphertext。
- `integration:manage/read` 通过服务 API 鉴权，DB RLS是可信runtime的scope隔离，
  不宣称数据库独立判断每个商家权限。读取／写入／历史重放都在等待后再次核授权。
- 凭据 JSON／String／GoString 隐藏明文；命令只接受 HMAC 指纹。独立32byte
  replayKey 与 AES keys分离、复制入内存；加密钥切换不破坏永久幂等。固定JSON
  编码+golden vector见合同。直接更换 replayKey 需要迁移方案，不支持静默轮换。
- 正常秘密轮换不改变 binding semantic version，不重新开启已停用 binding。
  `State` 恒为 `CONFIGURED_UNVERIFIED`；binding 初始 enabled=true 沿用执行层默认，
  **既不是支付方式已启用，也不是账户验证或供应商准入**。

## 分工与修正

|任务|角色／实际模型|base与路径|产出|
|---|---|---|---|
|源码复用盘点|platform explorer，gpt-6-luna/medium|a21a7f7，只读|Humaux 42ed0455-2b51-4583-ae25-d6f9d81b1701|
|官方 PAYUNi 调研|platform explorer，gpt-6-sol/medium|仅官方公开文档|Humaux 2ac20792-5935-44c4-b4cc-bfb35254db84|
|Go实现|integration worker，gpt-6-sol/high|829757f，独立 `/Volumes/data/worktrees/live-commerce-merchant-accounts`|四accounts文件，作者acc0f620，主线bbcbdf0；同包race/vet通过|
|合同／DDL／真实PG验收|root integrator|主线；合同8467222，DDL829757f，验收8e03c68/3063ed7|七个真实PG顶层测试及依赖/验收文档|
|独立审查|security reviewer，gpt-6-sol/high|只读合同、DDL、Go和测试|preflight d98f10a2-afc1-40b3-a93d-c8f0f374aa1d；最终4ed8845b-0770-4f5d-8d01-2306ef6d49ac，无新增P0/P1|

预检发现两项 P1：允许弱输入时裸SHA256指纹可被离线枚举；只验证内存加解密
与PG元数据不能证明实际存储可解密。实现前改为独立秘密HMAC，root测试再独立
读出数据库nonce/cipher/keyID，用fixture钥匙按冻结AAD解密创建/轮换两版并
验证精确内容。不是单测自身的seal/open互相证明正确。

作者曾误向非指定项目名 `livecommerce` 提交4文件代码索引；root已停止其后续
图谱写入并留痕，未删除共享记录。规范项目索引仍为 `live-commerce`，由root统一更新。
这次编排偏差不影响源码版本，但不能宣称严格未发生共享写越界。

## 实测

日志目录 `/Volumes/data/output/live-commerce-accounts-tests/`；Go 1.27.1，脚本
固定 PG18.6 镜像。全部秘密是隔离fixture数据，无真实商户凭据或外部交易。

- 初轮 `bash scripts/dev/test-local.sh`，bbcbdf0：exit0，197顶层PASS、0FAIL、
  0SKIP，race+vet，foundation 55.130s，`root-initial.log`。
- 3063ed7补齐绑定目标和Rotate故障后，`bash scripts/dev/test-local.sh` exit0：
  199顶层PASS／0FAIL／0SKIP，race+vet，foundation89.727s，`root-final.log`。
- `bash scripts/dev/test-local.sh --browser-identity` exit0：实际 Chromium→Next→Go→PG
  + 签名MOCK IdP，1PASS，foundation4.725s；不是未开发的凭据UI验收。
  工件 `output/playwright/identity-chain-20260924T101239.171735000/`。

初轮 SHA256：`85a14cbd5238c605a73586e1bfa2bc1034718babd92b5b52e3569fc4ec57c3ed`。

```text
0f8ff3cbf51c90f7a3eeacc98e32e6ed68e826e2d9abd881ffc2202fc0a966c0  root-final.log
9ae3c51e6764386575ed0789016069840515d6ccc8e40708505941b283f8166a  browser-identity.log
```

199=之前187+本单元5个同包测试+7个PG顶层测试，不是199个业务功能。
最终独立复核逐一检查实现、测试因果并计数/核hash，确认上述两项P1关闭；
审查者没有另跑数据库。2026-09-24 10:15 UTC带所有权标签的fixture容器查询为空；
测试脚本只回收自己创建的PG，日志/浏览器工件/worktree保留。

门禁包含：注册/元数据读回、实际持久密文两版本解密、AAD篡改、同key并发重放、
不同key重复账户无孤儿binding、环境隔离、外店/租户/权限/伪scope拒绝、禁用后
轮换不复活、CAS单赢家、撤销授权后历史重放拒绝、独立加密钥切换仍可重放。
绑定外键负例验证23503及binding约束名，避免错因通过；Create五类INSERT和
Rotate四类INSERT/UPDATE故障用非事务sequence证明注入点，再验证全部事实不变。
不得仅因返回错误就宣称目标门禁生效。

## 未完成与后续

- 自助HTTP/UI和列表、生产环境密钥加载与托管、旧钥迁移与保留/删除策略。
- 供应商验证、支付方式配置、运行时availability、StartPayment、PSP传输协议、
  回调／查询／退款／对账与sandbox/live。账户登记不授予这些能力。
- 物流连接和其他PSP、完整G03/G05/MS产品gate，完整SaaS部署。

[PAYUNi UPP官方文档](https://docs.payuni.com.tw/web/#/7/34)和
[交易查询](https://docs.payuni.com.tw/web/#/7/164)支持后续接口设计；调研未找到
无需交易的公开credential验证接口，故本单元不会为健康检查创建假订单。
截图候选服务保留，没有改变客户现有开关、直播、订单、资金或物流。
