# 配送仓库配置与分配算法：内部验收

2026-09-24，最终代码／测试基线 `8f83201`。合同及迁移 `10b2375`，
Go 作者 `d253a58` 合并为 `776266f`。本单元是结账前置配置和纯算法，
**不代表已建立买家订单、预留库存、调用第三方物流或收款**。

## 交付与调用关系

- [冻结合同](../../contracts/delivery-allocation-v1.md)：`fulfillment.SetAllocation`
  保存每个逻辑配送服务的 0..16 个有序同店仓库；空列表清除分配就绪状态。
  `GetAllocation` 读取历史一致的当前配置；服务改名／运费改版不重写仓库偏好。
- 复用 `authorize`、`command.Run/Audit`、pgx 调用方事务。凭据和 GUC 一致性
  在历史重放之前核验，操作人进入摘要；新配置同时校验服务版本及配置 CAS。
- 0011：不可变版本／仓库明细和可变 head；scoped FK、FORCE RLS、actor 策略。
  头和子表 INSERT 的延迟约束保证提交时仓库数量和顺序完整，也拒绝向旧版本追加。
- 锁序是 service head → allocation head → 按 UUID 排序的仓库。
  `inventory.lock_warehouse` 复用现有非登录 inventory writer 做 scoped FOR SHARE；
  普通 runtime 不获得仓库 UPDATE 权限。checkout 合同已同步相对锁序。
- `inventory.PlanAllocation` 只接收将来由服务端锁定并验证的库存／需求；
  重算可用库存、按商家顺序拆仓，输出按 warehouse/SKU 排序。
  任一 SKU 不足即返回错误和空计划，不返回局部分配；最大 50 SKU × 16 仓 = 800 行。
  没有新库存真源，没有调用旧 merchant Reserve 取得买家写权限，没有新依赖。

## 分工与复核

作者 allocation_config_impl：commerce_worker、gpt-6-sol/high，base `10b2375`；
独立 worktree `/Volumes/data/live-commerce-allocation-config-20260924`，
分支 `commerce/allocation-config-20260924`，只提交 fulfillment/inventory 的
allocation Go 与同包测试，共四个文件。root 负责合同、0011 和真实 PG 测试。

delivery_final_review：security_reviewer、gpt-6-sol/high，只读独立静态复核。
先指出 checkout 合同锁顺序歧义，已在 `83afdcb` 关闭；最终复核在 `8f83201`
确认 P0=0/P1=0。三个 P2 测试缺口（非空历史追加、跨店／租户读写、合法其他主体
直接写入）均补齐。Humaux `1785fd7c-c432-49a0-b279-b36b3c4670b8`。
reviewer 未自行运行 PG；以下实际执行和结果核验由 root 完成。

流程偏差：作者在收到原始禁止共享图谱写入的任务后仍执行了一次 code_index，
root 要求停止后未再写入；最终由持锁 root 重建六个相关文件索引并链接独立复核。
没有将该偏差当成正常授权步骤；worktree 文件写入边界未越界。

## 实际 gate

日志目录 `/Volumes/data/output/live-commerce-allocation-tests/`，均保留。

|执行|结果|日志|
|---|---|---|
|`bash scripts/dev/test-local.sh`|exit 0；Go1.27.1 + 实际 PG18.6；150 顶层 PASS / 0 FAIL / 0 SKIP；race + vet；foundation 75.630s|`root-final-20260924.log`|
|`bash scripts/dev/test-local.sh --browser-identity`|exit 0；实际 Chromium→Next→Go→PG、签名 MOCK IdP；1/1 PASS|`root-browser-20260924.log`|
|`GOTOOLCHAIN=go1.27.1 go test -run '^$' -fuzz '^FuzzPlanAllocation$' -fuzztime=2s ./internal/inventory`|exit 0；334,833 次执行；有限时长输入测试，不是完整证明|`root-fuzz-20260924.log`|

```text
91cd806740bf2a00d81e99437f9ce0ddfbf79d609462a6782423c7983080d3a7  root-final-20260924.log
5d3bb3492ba857dbf2cc4ea2e8a5685ff35067ba26654e08fd3b58735641f222  root-browser-20260924.log
dcbc50512491a039abaf1807cd0b3a086ec9915396a014bceadd4163657785f3  root-fuzz-20260924.log
```

150 = 既有 138 + 5 个新顶层单元测试 + 1 个 fuzz seed case + 6 个实际 PG 集成测试，
不是功能数量。PG 覆盖版本／顺序／清空、同 key 并发重放、CAS 单赢家、服务独立
改版、撤权后重放、跨 actor/store/tenant、GUC 伪造、SQL 权限与完整性、audit/
receipt 故障回滚、提交时约束错误回滚、仓库停用锁等待后的重验。

浏览器证据 `output/playwright/identity-chain-20260924T083418.342313000/`。
没有 UI 改动，因此此处是既有身份链兼容回归，不是尚未实现的配置／checkout UI 验收。
任务自有临时 PG 容器已由脚本回收，并经标签列表为空复核；保留测试日志与工作树。

## 下一步与 NOT_RUN

真实 checkout 对配置的锁定消费、两个买家争抢同一库存、订单 DRAFT+HELD、
原子库存账簿、到期与 StartPayment 竞态尚未实现／验收。本单元的纯算法数量守恒
不等于并发库存不超卖证明。目的地正式来源和买家选择快照、第三方账号连接与
sandbox/live、公开三语配置和结账页面、完整 G03/G05 及 SaaS 上线继续待完成。
不要求商家先固定承运商；不改客户现有生产开关、直播、订单、资金或物流。
