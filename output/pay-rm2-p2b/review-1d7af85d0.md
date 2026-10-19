**结论：APPROVE**

# PR #85 独立资金路径复核 · PAY-RM2 phase 2b（head 1d7af85d07fb8fcb546ab247f987d805f59b1592）

- 角色：独立 money-path reviewer，只读，未运行任何测试（NOT_RUN）。证据来自 `git show origin/unit/pay-rm2-p2b:<path>`、`gh pr view 85` 和 DELIVERY.md §Phase 2b（L498–515）。
- CI 采信 PR 所列：Gates 39236445742、Foundation 39236445781、focused 39236448246（P2B01–P2B04）均为 GREEN；RED 运行为 39230213089 和 39233787792。本复核没有重跑这些运行。
- 结果：**P0 0 条 · P1 0 条 · P2 6 条**。P2 不阻塞合并，按惯例由复核者开 issue、按模块攒批修。合并不等于上线，生产执行 0197 仍属 owner 红线。

## 1. 迁移

### 1.1 Stripe 分支字节等价（已核）
- `payments.apply_capture` 的定义历史如下：0018:214 首次 CREATE，0061:623 RENAME 为 `apply_capture_payuni_v1`，0061:882 重新 CREATE，0062:1150 CREATE OR REPLACE。之后直到 0197 都没有再次定义；post_river 中也没有它，全仓也没有动态 EXECUTE 重定义。所以前一版就是 0062。
- 用 `diff` 比较 0062:1150–1181 与 0197:37–66，函数体只有一处差异：删除 `IF v_provider='payuni' THEN PERFORM payments.apply_capture_payuni_v1(...)`，并把 `ELSIF v_provider='stripe'` 改为 `IF`。Stripe 的 refund/charge/observation 三个分支逐字节相同，PT409 兜底保留。
- 0197 中没有任何 ALTER OWNER/GRANT/REVOKE。CREATE OR REPLACE 会保留 OID、owner（commerce_checkout_writer）和 ACL（EXECUTE 仅授予 commerce_worker）。`SECURITY DEFINER` 和 `SET search_path=pg_catalog` 原样写出。P2B01（payuni_phase2b_test.go:97–104）以安装好的 pre-0197 函数为基准，比对 prosrc/owner/acl/proconfig/prosecdef；`stripe_schema_test` 以 0062 源文本为基准再比对一次，并断言函数体中不再含 `payuni`。
- 0197:13 头注中「byte-identical to 0062」的说法属实。

### 1.2 CASCADE、表、行、授权（已核）
- 两个文件都没有 CASCADE，没有 DROP TABLE/POLICY，也没有 DELETE/UPDATE。唯一被移除的授权是随函数一起消失的 EXECUTE。
- 外部 SQL 有两处正文依赖：dispatcher 依赖 `apply_capture_payuni_v1`，0197 先通过 CREATE OR REPLACE 去掉它，再 DROP（0197:37 先于 :70）；post_river/0005:446 的 `record_payment_query` 调用 `require_payment_query`，而 `record_payment_query` 本身也会被删掉。
- `hosted_guard` 只在 0025 中三个被删的 material 函数里被调用（0025:146/157/178/193/205–218）。`hosted_payment_view[_v2]` 不调用任何被删函数（0061:1294 那一处只是注释）。

### 1.3 Guard 22023（已核，有 P2 补强建议）
- 位置：0197:20–34，在全部 DDL（:37 起）之前。
- 事务：`migrations.Apply` 把所有待执行的编号迁移放在同一个 tx 里（migrate.go:77 Begin → :123 applyVersions → :126 Commit）。被拒时，同一批的 0196 等也会一起回滚，账本不变；P2B02 和 P2B04 都断言了这一点。
- 覆盖面（结合代码可以判定）：
  - `checkout.payment_attempts` 的 method_code 只可能是 `payuni_credit` 或 `stripe_checkout`（0061:76–79），所以子句 2 的过滤条件是完整的。
  - PAYUNi 的 operation action 只有 `payuni.query`，以及已被 0161 移除的 `payuni.notify_endpoint_set`。buyer family 本身就禁止 READY、DISPATCHING、BLOCKED_POLICY、STALE_BINDING（0016 中 operation_actor_family 约束）。所以 `READY/DISPATCHING/UNKNOWN/ACKNOWLEDGED` 已覆盖全部非终态。
  - PAYUNi 订单离开 AWAITING_PAYMENT 的唯一路径是 PAYUNi capture（0018:372，改为 CONFIRMED，同时 reservation 改为 COMMITTED）。0094:54 只是只读统计。因此只要子句 2 为 0，就不存在仍持有 PAYMENT_PENDING 库存、也不存在「已付款但还没落 fact」的 PAYUNi 订单。
- 剩余缺口见 P2-2、P2-3。

### 1.4 post_river 顺序（已核）
- 全新安装时：编号阶段 0016/0018 创建两个函数，0197 删除其余 8 个并提交；然后 post_river 按文件名排序执行，0005 用 CREATE OR REPLACE 重建 `start_payment` 和 `record_payment_query`，0026 再把这两个删掉（migrate.go:100 glob 排序，:174 postTx）。
- 升级时：0005 已登记，不会重跑，只执行 0026。
- 两条路径都由 P2B01 断言「10 个签名全部不存在」。0005 中没有任何对 0197 已删函数做 ALTER/GRANT 的语句，所以全新安装不会因为对象缺失而报错；CI fresh install 为绿，可以作为佐证。

### 1.5 全树残留调用方（已核：无）
- 对 10 个名字做 `git grep -w`，排除 contracts、output、docs 之后：
  - **Go 生产代码**：没有 SQL 调用。`internal/payments/capture_worker.go:104–108` 只对 provider=stripe 调用 `apply_capture`；`internal/checkout/payment.go:67` 的 `StartPayment` 在访问 PG 之前就直接返回 `ErrInvalid`。
  - **SQL**：只剩各函数自己的历史定义，以及 0161 和 post_river/0017 中的注释。
  - **测试**：要么经由 `p2bRetired` 系列 helper 期望得到 42883/PT409，要么运行在 pre-0032 的 legacy schema 上（这些函数在那个 schema 里本来就存在）。`home_cod_review_test.go:716` 和 `taiwan_cvs_pay_at_pickup_test.go:143` 走的是 Go starter，不碰 SQL。

## 2. 测试

### 2.1 Stripe a-映射抽查（抽 7 条，其中 4 条成立，1 条不变量成立但名字指错，2 条无 Stripe 等价）
| PAYUNi 测试（map 行） | map 所指 | 判定 |
|---|---|---|
| CaptureAtomicReplayAndFreshSession（map:75） | P2B03 stripe 子测 + SP08 | **成立**。P2B03 断言 apply 两次后：CAPTURED=1、ALLOCATE=1、READY work=1、CONFIRMED/COMMITTED（payuni_phase2b_test.go:292–309）。SP08 断言金额 2500 和 reference（stripe_flow_test.go:500–503）。 |
| CaptureLaterRefundAndReverseOrder（map:77） | RF07 | **成立**。RF07「replay in any order converges」（stripe_refund_flow_test.go:698）加上 amount/currency drift 无 fact，与「refund 信号既不抹掉也不重复 gross capture」等价。 |
| ExpiredHoldRace（map:37） | SP10 | **成立**。`expire_then_retrieve_closes_and_restores_exact_stock`（stripe_flow_test.go:769）。 |
| QueryProviderReferenceCannotFundTwoAttempts（map:65） | SP12 | **成立**。`duplicate_session_reviews_never_captures_or_closes`（stripe_flow_test.go:1198）加上 identity_mismatch。 |
| CaptureReleasedAnomalyKeepsRealMoney（map:81） | SP10/paid_in_the_last_second | **名字指错**。真正的等价测试是 `TestStripeSP12MoneyChecks/late_paid_after_closure_is_one_manual_refund_obligation_and_replay_converges`（stripe_flow_test.go:1277）：CAPTURED、PAID_ALLOCATION_FAILED、库存不动、replay 收敛。所以不变量是有 Stripe 守护的。见 P2-4。 |
| CaptureMultiSKUSingleCommit（map:84） | UntrackedMixedDrift、SP08 | **无等价**。Stripe 测试中没有任何一个对 ≥2 条 tracked 行的订单做 capture 并断言逐行守恒。`sucOrder` 是 1 条 tracked 加 1 条 untracked（stripe_untracked_capture_test.go:32–35）；SP08 只有单行（:491）。这是 Stripe 侧的既有缺口，因为旧 PAYUNi 测试测的是 0018 的函数体，从未覆盖 `apply_stripe_observation`，所以不算本 PR 造成的回归。见 P2-4。 |
| CaptureBalanceWaitIsAtomic（map:82） | UntrackedReservationRefusals、QuantityBoundary | **无等价**。这两个测试都不涉及「锁等待期间未提交的 capture 不可见」。同样属于既有缺口，不是回归。 |

### 2.2 断言删除与阈值（已核）
- 14 个被改动的测试文件里没有删除任何顶层 `func Test`。被删的 t.Run 子测都属于被删 SQL 的正向路径，由 `p2bRetired` 系列 helper（payment_sql_contract_helpers_test.go:243–313）给出 b 类替代断言：签名不存在、精确 SQLSTATE、租户业务行快照不变、历史行仍为 pending 且可读。
- 两处计数变化都按被删对象精确扣减，没有放宽：
  - T06 计数 97→93，扣掉 4 个 `*_payment_query`。比对仍是精确相等，被删定义一旦回来就会失败。
  - R2 账本 pin 123→125，即 +0197 和 +post_river/0026，每个文件各一行注释。
- `stripe_schema_test` 去掉 0018 body-hash 检查后，换成了更强的检查：拿 0062 源码删掉 PAYUNi 分支后与已安装函数体全文相等，且函数体不含 `payuni`。

### 2.3 `lriApply` 预登记与 P2B04（诚实）
- `lriApplyFull` 就是原来的 `lriApply` 函数体，只是改了名字（diff 中只有拆分，没有改动）。P2B04 在同一个 pre-0032 fixture（`lriPre0032Fixture` 加 `pwOldQuerySetupOn`）上调用 `lriApplyFull` 跑真实全链，没有预登记，断言 22023、账本不变、phase-2b 对象仍然存在（payuni_phase2b_test.go:254–284）。
- 原来那些「lriApply 必须失败」的门（tampered ledger、unknown version 等，legacy_runtime_upgrade_test.go:918/927/959/1099）在预登记之后，失败原因又回到了它们本来要测的那个，而不是被 0197 的拒绝顺带「误绿」。预登记使用真实 checksum。
- 代价是：「干净的 legacy 库升级穿过 0197」这条路径不在 legacy 门里，由 P2B01（从 phase-2a head 升级）覆盖。这点可以接受。

## 3. UNKNOWN 解答（代码可判定的部分）

**UNKNOWN 1（已解决，靠代码判定）：expiry 从来不释放 PAYUNi AWAITING_PAYMENT 订单的库存，这与 0197 无关。**
- 现行的 `checkout.expire_held`（0190:57–147）只处理 `commercial_state IN ('DRAFT','AWAITING_TRANSFER')` 且 reservation 为 `HELD` 的订单（0190:80–81、:96），对 AWAITING_PAYMENT 和 PAYMENT_PENDING 直接 `EXIT`。
- 被删的 PAYUNi query 函数（0017、0096、0018 中的 record，以及 post_river/0005:440 起）只写 `provider_observations`，不碰 orders、reservations 或 inventory。PAYUNi capture（0018）只会把库存推进到 COMMITTED/ALLOCATE，从来没有 RELEASE。
- 结论：「query 已终结但未付款」的 PAYUNi 订单本来就没有任何自动释放路径。被删的函数不在 expiry 路径上，删除它们不会改变 expiry 行为。0197 的子句 2 保证执行成功后不存在仍处于 AWAITING_PAYMENT（因而也不存在仍持有 PAYMENT_PENDING 库存）的 PAYUNi 订单。
- 仍待 owner 决定的只有一件事：运维对这类订单做对账时，必须同时写 RELEASE ledger 并把 reservation 改为终态（见 P2-2）。

**UNKNOWN 2（已解决，靠代码判定）：遗留授权目前无人使用，revoke 是安全的，保留则属于残余权限（P2-5）。**
- `checkout.hosted_payment_pages` 只有 0025:191（save_hosted_page 的 INSERT）和 0025:214（take_hosted_page 的 UPDATE）两处写入，两者都已被删。
- `hosted_credential_read` 策略（0025:47）只服务于 owner 为 checkout_writer 的 credential 读者。全仓中读取 `integration.account_credentials` 的定义函数里，唯一一个 owner 是 checkout_writer 的就是 `load_hosted_material`（0025:152，已删）。其余读者 `load_stripe_credential`（0061:951/0137:1272）、`load_stripe_refund`（0062:386）、`load_ecpay_*`（0073）的 owner 都是 `commerce_integration_writer`，不依赖这条策略。
- commerce_checkout_writer 是 `NOLOGIN NOINHERIT`（0013:4），所以当前的暴露面只限于新写的 checkout_writer definer。

**UNKNOWN 5（已解决，靠代码和部署配置判定）：参考部署中的 migrator 是 superuser。**
- `cmd/migrate/main.go:7–9` 写明 migration-owner DSN 就是 bootstrap superuser；`deploy/scripts/preflight.sh:306`（P04）强制要求 `dsn_migrate_owner` 内嵌 `pg_superuser_password`；`deploy/compose.yml:684` 只读取这一个 secret。superuser 会绕过 RLS，所以计数是完整的。
- 如果改用非 superuser 的表 owner，`checkout.payment_attempts`（0016:55）和 `integration.operations`（0008:85）都设置了 FORCE RLS，加上 `row_security=off`，会直接报错，即 fail-closed，不会出现「计数为 0 却放行」。
- 仍然 UNKNOWN 的只有一点：实际生产环境的 secrets 是否已经通过 preflight P04，这需要 owner 部署时确认。

## 4. 文件头（已核）
- 所有被改动的 .go/.sql 文件，以及 `output/pay-rm2-p2b/test-contract-map.md`，都在前 30 行内具备 Purpose / Depends on / Used by / Invariants。
- `contracts/*.md`（7 个）和 `experiments/results/packet-check.json` 没有这四项。不过仓库 100 个 contract 中只有 2 个带这种头，JSON 也没法写注释，这符合现有惯例，不算 finding。

## 5. 问题清单

**P0：无。P1：无。**

- **P2-1 · 编号阶段提交到 post_river 阶段提交之间，`checkout.start_payment` 仍可执行，且 0026 不再复查 guard。**
  - 位置：migrations/post_river/0026_payuni_phase2b_drop.sql:9–10；授权见 migrations/0016_buyer_payment_start.sql:223（`commerce_checkout_runtime`）。
  - 风险：在 migrate.go:126（编号阶段 commit）到 postTx commit 之间，或 post 阶段失败之后的任意时间，直接调 SQL 仍能新建 PAYUNi 的 AWAITING_PAYMENT 订单并占用库存。而此时 capture 返回 PT409，expiry 又不处理这种状态，库存会永久挂起。Go 侧已经拒绝这条路径，所以只有直连 SQL 才能触发。
  - 建议：在 0197 中加 `REVOKE EXECUTE ON FUNCTION checkout.start_payment(...) FROM commerce_checkout_runtime`。全新安装时 0005 的 CREATE OR REPLACE 会保留这个 ACL。或者在 0026 开头复制一份 guard 的 DO 块。
- **P2-2 · Guard 子句 2 只看订单状态，不看 reservation。**
  - 位置：migrations/0197_payuni_phase2b_drop.sql:26–28。
  - 风险：运维对账如果只把订单改成 CANCELLED、没写 RELEASE，guard 照样放行，PAYMENT_PENDING 库存就永久占用。被删函数本来也不释放库存，所以这不是 0197 引入的问题；但 guard 是对账的最后一道网。
  - 建议：在 join 中加上 `OR r.state='PAYMENT_PENDING'` 条件（关联 inventory.reservations，r.id=o.id），同时在 runbook 里写明对账必须执行 RELEASE。
- **P2-3 · Guard 检测不到「已存储但从未应用」的 PAYUNi 观测。**
  - 位置：migrations/0197_payuni_phase2b_drop.sql:20–34；相关代码 internal/payments/capture_worker.go:104–106。
  - 风险：phase 2a 起，worker 会把 PAYUNi reconcile job 直接 cancel。所以对于订单已经 CONFIRMED 之后才存进来的 refund 类 QUERY 报告（例如 CardRefundType），对应的 REVIEW_REQUIRED/refund 审核从来没有落库。0197 删掉了唯一能应用它们的 SQL（`apply_capture_payuni_v1`）。库里有没有这类行属于数据问题（UNKNOWN）。
  - 建议：上线前 owner 先跑一条只读查询：PAYUNi attempt 的 provider_observations 中 report_hash 没有对应 facts 或 review_cases 的行数。或者把这个计数并入 guard。
- **P2-4 · test-contract-map 中有 3 条 a-映射不准确。**
  - 位置：output/pay-rm2-p2b/test-contract-map.md:81/82/84。
  - :81 应改为指向 `TestStripeSP12MoneyChecks/late_paid_after_closure_…`（stripe_flow_test.go:1277）。
  - :82 和 :84 在 Stripe 侧没有等价测试，应标为「无 Stripe 等价（既有缺口）」。另开 issue，为 `apply_stripe_observation` 补两个门：2 条 tracked 行的 capture 加 replay 守恒（每行 ALLOCATE 恰好 1 条、reserved→allocated），以及锁等待期间不可见。
  - 不阻塞本 PR，因为 Stripe 侧的覆盖没有因为本 PR 而减少。
- **P2-5 · 残余权限。**
  - 位置：migrations/0025_hosted_payment.sql:46–53（`hosted_credential_read` 及 ciphertext 列授权）；`checkout.hosted_payment_pages` 的 INSERT 和 UPDATE(handed_out_at) 授权。
  - 按 UNKNOWN 2 的分析，现在已经没有任何使用者，建议 follow-up 中 revoke。
- **P2-6 · 部署 runbook 需要说明：0197 被拒会让整批编号迁移一起回滚。**
  - 位置：migrations/migrate.go:77–127。
  - 只要库里还有未对账的 PAYUNi 历史，这个 release 中的所有编号迁移（包括 0196）都无法上线，`cmd/migrate` 会以 exit 1 退出并输出 `code=22023`。P2B04 已经钉住这个行为，建议写进上线前检查清单。

## 6. 证据级
- 静态复核（读源码、diff、grep），没有运行任何测试，因此本复核本身属于 E1。
- 对「已在测试环境中完成」的判断，依据的是 PR 中记录的 CI 运行（E3，绑定 1d7af85d0）。本复核没有独立重跑这些运行，也没有逐个打开 CI 日志核对。
