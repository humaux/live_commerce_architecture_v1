# 独立复审 0617d49ef — PR #70 (pay-rm2-p2a)

- 结论：**APPROVE**
- 复审人：独立复审子代理（只读；未推送；未本地跑 Go 测试；gofmt/go vet 由调用方报 OK）
- 基线：32b5df70e（已在 review-32b5df70e.md 批准）；HEAD 0617d49efe73b1f82a40a391a2ef67ab95798ce5
- 范围：`tests/foundation/legacy_runtime_admission_test.go`（+11/-3，仅测试）
- 触发：CI 38769037654 g07 `TestLegacyRuntimeIsolationAdmissionAndReadiness:96` 报 "invalid request"。原因是测试仍在驱动已退役的 `checkout.PaymentStarter.StartPayment`，该方法现在无条件返回 `command.ErrInvalid`（`internal/checkout/payment.go:67-68`）。

## 1. 覆盖有没有丢 —— 没丢。旧 Go 用例本来就是 DB 触发器的代理

旧用例的路径：`psStarterIn(..., "river_expiry")` 用 `p.pool`（checkout runtime 角色）做 `InsertTx(payment_query_v1)`，写进的是 `river_expiry.river_job`。

- ACL 不会先拦：`migrations/post_river/0005_legacy_river_isolation.sql:669-671` 授予 `commerce_checkout_runtime` 对 `river_expiry.river_job` 的 `SELECT,INSERT,UPDATE(kind)` 以及序列 `USAGE`。所以 INSERT 能到达行级触发器，不会先报 42501。
- 拦截点是 `expiry_job_family`（BEFORE INSERT OR UPDATE，`0005:643-644`），对应函数 `checkout.guard_expiry_job_family()`（`0005:615-637`）。其中 `NEW.kind<>'checkout_expiry_v1'` → `RAISE ... ERRCODE='22023'`，随后由 Go 的 safeError 映射为 "checkout database unavailable"。旧断言就是这个结果。
- 同一个测试里保留的 SQL 用例 `"expiry foreign kind"`（`INSERT INTO river_expiry.river_job ... VALUES('payment_query_v1',...)` → 22023）走的是同一个触发器、同一个分支。该触发器是 SECURITY DEFINER，以表所有者执行时同样会触发。此外，同一测试的 disable/drop/owner/search_path 子用例证明：触发器一旦缺失，就绪状态会转为 false。
- 因此数据库层面的保证完全保留。失去的只有"Go 错误映射 → 'checkout database unavailable'"这一层，而它属于已退役的入口，没有保留价值。

## 2. Stripe 生产者（`internal/checkout/stripe.go:141`、`:319`）

- 角色：hosted pool 使用 `commerce_hosted_runtime`，它继承 `commerce_checkout_runtime`（`migrations/0025_hosted_payment.sql:4`），因此对 `river_expiry.river_job` 也有 INSERT 权限。如果 River client 的 schema 接错成 river_expiry，`InsertTx(payment_query_v1 / payment_signal_v1)` 会被同一个 `expiry_job_family` 触发器以 22023 拒绝，整个事务回滚。
- 第二道防线：`checkout.start_stripe_payment` 要求 `river_payment.river_job` 中存在 `id=p_job AND kind='payment_query_v1'` 的行（`post_river/0012_stripe_payment.sql` 函数体约第 72 行）；signal 路径对 `payment_signal_v1` 做同样检查（约第 186-187 行）。错误 schema 下拿到的 job id 不可能出现在 river_payment 里。
- 生产接线是硬编码：`cmd/api/buyer_payment.go:90` 使用 `Schema: "river_payment"`；Stripe 相关测试的 client 也全部用 river_payment。
- **NOT_COVERED（P2，不阻塞合并）**：没有任何测试用错误 schema 的 River client 驱动 Stripe 生产者，并断言失败关闭（无写入、错误被脱敏）。不阻塞的理由是两层 DB 守卫都已在 SQL 层被覆盖：触发器由本测试的 "expiry foreign kind" 覆盖，start 函数的 job 校验由 stripe start 系列测试覆盖。缺的只是 Go 接线这一层的端到端代理。建议后续补一个用例：`river_expiry` client 加 hosted pool，调用 `StartStripePayment`，期望得到脱敏错误，并且 facts 与三条 lane 都不变。

## 3. 正向路径与后续子用例

- `p.start`（`tests/foundation/payment_start_test.go:166-171`）在 `p.sql != nil` 时优先走 SQL 启动器。负向段执行时 `p.sql` 还没设置，所以确实打到退役的 Go 入口；随后设置 `p.sql = psSQLStarter(...)`，正向路径因此改走 `rm2StartSQLTx`。这与 `psSQLSetup` 的用法一致。
- `goodPayment.JobID` 来自 SQL 合约直接写入的 `river_payment.river_job`（kind `payment_query_v1`，带 queue 路由和 link）。后续的 "payment wrong queue"、"payment orphan" 复制这一行再插入，期望 22023；"job identity rewrite" 对这一行做 UPDATE args，期望 22023。这些用例都依赖"该行是合法且已 link 的 payment 家族 job"，SQL 启动器满足这个前提，所以语义不变。

## 4. 小问题（P3，不要求修改）

- 现在的负向段对 `StartPayment` 是同义反复：无论 schema 是什么，它都无条件返回 ErrInvalid。它唯一的价值是"退役入口不写任何东西"的回归守卫。注释已说明这一点，可以接受。如果想更精简，也可以删除这段。

## 证据级

- E2（静态审读加迁移交叉核对）。本复审未运行 PG 测试；CI 重跑结果须以 0617d49ef 的 g07 绿为准，状态 NOT_RUN（由本复审人执行的部分）。
