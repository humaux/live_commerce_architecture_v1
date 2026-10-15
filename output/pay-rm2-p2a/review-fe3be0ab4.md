# 独立复核：PR #70 `0617d49ef..fe3be0ab4`

结论：**APPROVE**

- 复核方式：只读。gofmt -l tests/foundation 退出码 0；go vet ./tests/foundation 退出码 0。按约束没有在本地跑 Go 测试，CI 重跑之前状态是 NOT_RUN。
- 证据级：E1（结构与静态检查）+ 对源码逐项核对。本复核不代表 g07 已经变绿，要等 CI。
- Head：fe3be0ab43902c5df0df81c70372803f8d6a5e20。产品代码（internal/）在此区间没有改动，diff 只涉及 tests/foundation 下 4 个文件。

## 逐项核对

| 主张 | 结论 | 依据 |
|---|---|---|
| River v0.40.0 中，短 snooze 会让 job 保持 'available'，并带一个未来的 scheduled_at | 成立 | `river@v0.40.0/internal/jobexecutor/job_executor.go:397`：当 `nextAttemptScheduledAt - now <= SchedulerInterval` 时走 `JobSetStateSnoozedAvailable`。`riverdriver@v0.40.0/river_driver_interface.go:626` 写入 `State: available, ScheduledAt: &scheduledAt`。`maintenance/job_scheduler.go:25` 定义 `JobSchedulerIntervalDefault = 5s`，`client.go:518` 用 `cmp.Or` 取它作默认值，internal/ 里没有覆盖。fetch 查询带 `scheduled_at <= now`（riverpgxv5 river_job.sql:213），所以这个 job 在 pin+5s 之前取不到 |
| sstWake 跳过上述状态 | 成立 | `stripe_start_test.go:827-831` 只更新 `state IN ('scheduled','retryable')` |
| stripe_query 的 create 成功后用 5s 的 delay | 成立 | `stripe_query.go:154` 和 `:187` 都是 `delay: 5*time.Second`，`:110` 执行 `river.JobSnooze(step.delay)`。5s ≤ 5s，按上面的规则落到 available |
| clientForClaim 在每个 query step 都调用 VerifyAccount | 成立 | `stripe_runtime.go:286-288`：每次 `stripeExecute` 都会走 `clientForClaim`（`stripe_query.go:122`），先发 GET /v1/account，再发 GET session（`stripeRetrieve`） |
| 观测到 capture 后不再有 provider 调用 | 成立 | `stripe_query.go:81-82`：snapshot.Captured 时直接 `finishStripe(...,"stripe_terminal_observed",0)`，不会进 `stripeExecute`。session 状态为 complete/paid，所以不会触发 expire 分支 |
| rm2WakeQuery 中 operation_id = AttemptID | 成立 | `internal/checkout/stripe.go:141` 插入 `paymentQueryArgs{OperationID: attemptID}`，`:159` 校验 `out.JobID == job.Job.ID`，`:197` 断言 `out.OperationID == out.AttemptID`。按 id + kind + operation_id 三重绑定，并要求 RowsAffected==1，否则 Fatal |
| dbd1c94e0 的 42883 修复 | 正确 | `$1` 只对比 `args->>'operation_id'`（text），`$2::uuid` 对比 `provider_observations.attempt_id`，传入 AttemptID。语义不变（Stripe 下 OperationID==AttemptID），只修正参数类型推断，也把错位的实参改回了正确的 |

## 是否削弱保证

- 崩溃救援测试：旧断言 `calls==1` 统计的是全部 transport 调用。因为 VerifyAccount 必然会发，这条断言在当前代码下根本不可能满足，所以它本身就是错的前提。新断言是 `calls==2 && retrieves==1`，同时钉死了总调用数和 retrieve 数。多出任何一个 provider step 都会让两者一起变红，所以不弱于旧断言。report/CAPTURED 计数保持不变。
- rm2WakeQuery 只改 scheduled_at，不改 state，也不重写代码路径。它比 sstWake 收得更窄（只动一个确定的 job id），找不到这个 job 时直接 Fatal，不会静默跳过。
- 文件头注释（Purpose/Depends on/Used by/Invariant）只增不减。

## 残留（P2，不阻塞合并）

1. 救援测试有时序窗口：CAPTURED 由 reconcile job 异步写入。如果 reconcile 晚于 query 的下一次 5s 轮询，第二个 step 会看到 !Captured，再发一次 retrieve，测试会变红。这类失败偏向安全一侧，只会误报，不会漏报。只有 CI 真出现 calls=4 时再处理。
2. 还有其他 sstWake 调用点（stripe_flow/stripe_process/stripe_create_rejection_final/stripe_retained_invariants）。它们大多在轮询循环里，available 的 job 在 5s 内本来就会自然到期，所以不受影响。本区间没有动它们，这里只记录一下。
3. rm2WakeQuery 处理 'scheduled'/'retryable' 时不翻转 state，需要等 JobScheduler（≤5s）。当前两处调用的前提都是 'available'，所以不受影响。
