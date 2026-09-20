# 可执行规格模型实验记录

**范围：Python 离散模型；不是 SaaS 产品、数据库、真实平台或语言性能验收。**

运行时间（UTC）：`2026-09-07T20:50:13.900961+00:00`

脚本 SHA-256：`9e3288a624d7eb2410faf439b26ec730e439bb37e64fbb801f5f1107307795a2`

## 安全机制的负对照／变异测试

| ID | 删除的机制 | 基线满足判据 | 删除后被判据捕获 |
|---|---|---|---|
| M01 | comment-idempotency | True | True |
| M02 | tenant-scoped-lookup | True | True |
| M03 | transactional-intent | True | True |
| M04 | atomic-stock-reservation | True | True |
| M05 | dispatch-consent-recheck | True | True |
| M06 | frozen-credential-binding | True | True |
| M07 | unknown-outcome-no-blind-retry | True | True |
| M08 | attribution-not-identity | True | True |
| M09 | late-payment-no-oversell | True | True |
| M10 | hostname-scoped-cache | True | True |
| M11 | message-expiry-at-dispatch | True | True |
| M12 | payment-account-binding | True | True |

这些机制不得在生产关闭。测试仅说明预设反例在本规格模型中被识别；
不能证明所有故障已覆盖，也不能证明真实实现正确。原始观测见 JSON。

## A00：公平调度优化消融（合成模型）

相同的 510 个任务、单服务槽、每任务 1 tick、全部在时刻 0 可用。A 有 500 个、B 有 10 个。
比较轮询租户调度与全局 FIFO，除调度规则外没有其他变动。

| 变体 | B 的 P95 完成时间（tick） | 总服务时间（tick） | 全体平均完成时间（tick） |
|---|---:|---:|---:|
| round_robin | 20 | 510 | 255.5 |
| fifo | 510 | 510 | 255.5 |

解释：公平性在这个刻意构造的洪峰场景中改变小租户等待，不增加总吞吐。
tick 不是毫秒；这是确定性模型，无独立真实采样，不能给生产性能置信区间。
不能据此宣称 River 已有相同性能，更不能据此证明 Go 比 Rust 快。

## 尚未执行

语言对比、真实 PostgreSQL/River 压测、LiveKit 多路推流、外部 API 和 Codex 子 Agent 的 1/2/4 并行实验均为 NOT_RUN。
