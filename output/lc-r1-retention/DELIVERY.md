# DELIVERY — unit lc-r1-retention（LC-R1 retention：A1.4 四类发送操作的有限脱敏 + UNKNOWN 过期租约脱敏）

- 角色: DeepSeek V4-Pro（后端执行者；不合并/不推送/不部署，integrator 负责）。实际模型: deepseek-v4-pro（会话驱动）。
- Base SHA: `bb71f966`（r3/integration）；worktree: `.worktrees/lc-r1-retention`；branch: `unit/lc-r1-retention`。
- 合同: `contracts/live-console-v1.md` Amendment 1（2026-10-05, K3 round-2）**A1.4 P1-4**（权威规格，L960-989）+ `contracts/claims-retention-purge-v1.md` §1 C4、§0 "Rejected: redacting non-terminal / UNKNOWN"、§4 特权行。
- 迁移号: `0127`（integrator 占位；合并时按空号重编号）。
- 状态: 迁移 + foundation 测试全绿（REAL_PG，`^TestClaimsRetention` 10/10 PASS，84.3s，含 integrator review round 1 的 P1-1/P1-2 修复与红→绿证据）；无 Go 源码改动；check-gates node 段与 spec_models 见 §NOT_RUN。

## 证据标签

| 证据 | 标签 |
|---|---|
| 本文件 / 迁移注释 / 源码注释 | DESIGN |
| `migrations/0127_lc_r1_retention.sql`（PL/pgSQL 定义者 + RLS 策略） | 实现（无真实 Meta 流量） |
| `tests/foundation/claims_retention_test.go`（一次性 PG 容器，`bash scripts/dev/test-focused.sh '^TestClaimsRetention'`） | REAL_PG |
| `output/lc-r1-retention/crp.log`（10/10 PASS，exit 0，84.3s） | REAL_PG 证据 |
| `output/lc-r1-retention/red.log`（0071-only 基线，5 FAIL，exit 1，见「红→绿证据」） | REAL_PG 红证据 |
| `bash scripts/dev/check-gates.sh` node 段、`python3 experiments/spec_models.py`、全仓 `go test -race ./...` | NOT_RUN |

## 交付物（唯一写入路径，均在合同清单内）

实现：
- `migrations/0127_lc_r1_retention.sql` — A1.4 前向迁移（见「迁移校验」）。review round 1（P1-1）追加 `CREATE OR REPLACE claims.erase_actor`：RD5 hold 从「非终态 `meta.private_reply`」扩到非终态 `meta.dm_send`（`peer_key=ANY(p_peer_keys)`）与非终态 `meta.public_reply`（`comment_ref` 落在 actor 的 intake refs），与 private_reply hold 同语义（v_retry=now+1h）。
- `tests/foundation/claims_retention_test.go`（改）— 扩展/新增 CRP 门禁：
  - `crEligible`（C4 可脱敏谓词）扩到四类 action + `request ?| array['comment_ref','conversation_id','peer_key']` +（终态五 OR `state='UNKNOWN' AND (lease_until IS NULL OR lease_until < clock_timestamp())`）。
  - `crWantMatrix`（CRP02 特权期望）operations SELECT 列集加 `lease_until`。
  - `operationC4` / 新增 `operationC4Req` / `operationC4ReqOn`：按 action 构造 request / 语义键前缀；覆盖 READY（gen 0）/ DISPATCHING（dispatch 租约）/ UNKNOWN+leaseExpr（reconcile 租约）三种租约形态；`operationC4Req` 是脱敏测试绑定特定 `peer_key`/`comment_ref` 的钩子，`operationC4ReqOn` 支持向任意 pool（CRP08 恢复库）播种。
  - CRP04 子测试 `C4-a14-send-actions`：终态 dm_send/public_reply 旧行脱敏（mdm-purged/mpub-purged）、年轻行保留、offer_recommend 无 id 不动。
  - `writerPolicyBehaviour`：加 `opDM/opPub/opRec`，四类 action 可读、dm_send/public_reply 脱敏放行、保留 peer_key 拒绝。
  - **`TestClaimsRetentionCRP13UnknownRedaction`**（重写）：四类 action 各四种形态——旧无租约 UNKNOWN（脱敏）/ 旧有租约 UNKNOWN（过期 `lease_mode='reconcile'` 租约，脱敏）/ 活租约 UNKNOWN（跳过，保持不动）/ 年轻行（保留）；断言 `request_hash` 四类 action 都不变、语义键改 `<prefix>-purged:<id>`、整行 `o::text` 不含 sentinel `peer_key`/`conversation_id`、二次 run 计数 0。
  - CRP06 `a14-send-actions-erasure`：`Erase`（PeerKeys=[actor.peer]）脱敏终态 dm_send（peer_key 匹配）与 public_reply（comment_ref 经 intake），另一 peer/comment 的同类 op 逐字节不动；`holds` 子测试新增「non-terminal dm_send (READY)」「non-terminal public_reply (READY)」两例，断言 held 且不写任何行。
  - CRP08 `replay-removes-pending-intake-and-redacts-nonterminal-operations`：新增非终态 public_reply（comment_ref 经 intake）重放脱敏（state 不变、mpub-purged）；dm_send 重放不脱敏为已知限制（见「风险与不变量」）。
  - `crUpgradeAndPreconditions`：与 0071 一同压住 0127 的 ledger 行（其 55000 precondition 依赖 0071 定义者），最终 Apply 前删除 0127 hold，使 "populated-upgrade" 落在 0071+0127 全量基线并复跑 `crAssertMatrix`。

未触碰：`internal/retention/**` 与任何 Go 源码（CRP10 守护：定义者只被 internal/retention 调用，本单元只需改迁移 + 测试）；`apps/**`、`go.mod`/`go.sum`、OpenAPI 共享 schema、pnpm 锁文件、`contracts/*`（含 `contracts/claims-retention-purge-v1.md` L184 的旧特权行——A1.4 P1-4 条款 5 明确 "CRP02 equality updated by the integrator"）。

## 红→绿证据

- **红证据（review round 1）**：`output/lc-r1-retention/red.log`。用临时标记文件（`output/lc-r1-retention/RED_EVIDENCE`，跑完即删）让 foundation fixture 与 CRP08 的 `migrations.Apply` 把 0127 记入 ledger 而跳过——即整门禁跑在 0071-only 基线上，`bash scripts/dev/test-focused.sh '^TestClaimsRetention'` 得到 **`test-focused: top-level PASS=5 FAIL=5 SKIP=0 exit=1`（93.8s）**，且失败点正是本轮新增/改动的断言：
  - CRP04 `C4-a14-send-actions`：终态 dm_send/public_reply 未脱敏（key 仍 `mdm:`/`mpub:`，`comment_ref`/`peer_key` 存活）。
  - CRP13：四类 action 的旧 UNKNOWN 行未脱敏（key 未改、`redacted` 缺失、整行仍含 `peer_key`/`conversation_id`/`comment_ref` sentinel）。
  - CRP06 `a14-send-actions-erasure`：Erase 后 dm_send/public_reply 未脱敏；`holds` 两例 `held=false`（0071 无 dm/public hold）。
  - CRP08 replay：非终态 public_reply 未脱敏（key 仍 `mpub:`、`comment_ref` 存活）。
  - CRP07 `writerPolicyBehaviour`：四类 action 的 writer 读/脱敏矩阵在 0071 下不成立。
- REAL_PG 门禁 `bash scripts/dev/test-focused.sh '^TestClaimsRetention'` — **`test-focused: top-level PASS=10 FAIL=0 SKIP=0 exit=0`（84.3s）**，证据 `output/lc-r1-retention/crp.log`：
  - `TestClaimsRetentionCRP10SourceGuards`（0.29s）、`CRP03ReportOnly`（1.28s）、`CRP04EnforcedPurge`（8.39s，含 `C4-a14-send-actions`）、`CRP13UnknownRedaction`（0.10s）、`CRP05Concurrency`（5.76s）、`CRP06Erasure`（10.11s，含 `a14-send-actions-erasure` 与 dm/public hold）、`CRP02Schema`（5.36s，含 populated-upgrade）、`CRP07Privacy`（14.69s）、`CRP08RestoreReplay`（4.95s，含 public_reply 重放）、`CRP09Worker`（31.61s）全部 PASS。
  - 既有 `TestClaimsRetention*` 全绿，无删改/放宽任何失败测试、阈值或 fixture。

## 迁移校验（0127）

- 前向：precondition 要求 `claims.run_retention(integer)`、`claims.apply_actor_erasure(text,text,uuid,uuid,uuid,text[])` 与 `claims.erase_actor(uuid,text,text,text,text,uuid,uuid,uuid,text[])` 存在（否则 55000）——即 0071 已应用。
- §4 特权：`GRANT SELECT(id,tenant_id,store_id,action,state,request,created_at,lease_until) ON integration.operations TO commerce_retention_writer`（加 `lease_until`，re-grant 幂等）。
- §4 策略：DROP+重建 `operation_retention_read`（USING 四类 action）/ `operation_retention_update`（USING/WITH CHECK = 四类 action AND（终态五 OR `state='UNKNOWN' AND (lease_until IS NULL OR lease_until < clock_timestamp())` OR replay 标志）；WITH CHECK 加 `NOT (request ?| array['comment_ref','conversation_id','peer_key']) AND semantic_key ~ '^(mpr|mdm|mpub|mrec)-purged:'`）。replay 标志路径（`lc.retention_replay`）不变。
- `claims.run_retention(integer)`（CREATE OR REPLACE）：C4 WHERE = 四类 action AND `created_at < v_now - intake_days` AND `request ?| array[三 id]` AND（终态五 OR UNKNOWN 过期/无租约）；UPDATE `request=(request - 'comment_ref' - 'conversation_id' - 'peer_key') || '{"redacted":true}'`、`semantic_key=CASE action → mpr/mdm/mpub/mrec-purged: || id`。owner `commerce_retention_writer`；EXECUTE 授 `commerce_retention_job, commerce_retention_operator`。
- `claims.apply_actor_erasure(...)`（CREATE OR REPLACE）：DECLARE 加 `v_comments text[]; v_n integer`；在 intake DELETE 之前 `SELECT array_agg(DISTINCT comment_ref) INTO v_comments` 捕获 actor 的评论 ref；三段 UPDATE（private_reply 经 bundle_id join → `mpr-purged:`；dm_send 经 `request->>'peer_key'=ANY(p_peer_keys)` → `mdm-purged:`；public_reply 经 `request->>'comment_ref'=ANY(v_comments)` → `mpub-purged:`），每段 `(v_replay OR 终态五)`，`GET DIAGNOSTICS v_n=ROW_COUNT` 累加 `n_ops`。offer_recommend 不携带任何人 id，永不 eligible（A1.4 条款 2）。private_reply 分支与 0071 逐字节一致（CRP06/CRP08 无回归）。
- `claims.erase_actor(...)`（CREATE OR REPLACE，P1-1）：整体复制 0071 函数体，仅在 `IF NOT v_single` 内、private_reply hold 之后追加两个 EXISTS hold——非终态 `meta.dm_send`（`request ? 'peer_key' AND request->>'peer_key'=ANY(p_peer_keys)`）与非终态 `meta.public_reply`（`request ? 'comment_ref' AND request->>'comment_ref' IN (SELECT comment_ref FROM claims.meta_intake WHERE actor_key=v_key)`），命中则 `v_hold:=true; v_retry:=greatest(coalesce(v_retry,v_now),v_now+interval '1 hour')`，与 private_reply hold 同语义。函数 owner `commerce_retention_writer`、`REVOKE ALL FROM PUBLIC`、`GRANT EXECUTE TO commerce_retention_operator` 与 0071 一致。
- COMMENT ON FUNCTION 三处标注 `internal/retention`。

## 风险与不变量

- P0：租户/店铺/主体现由服务端 GUC 确定，定义者按 `current_setting('app.*')` 复核；请求内 tenant_id/Host/客户端金额一律不信任。
- `request_hash` 保留原请求哈希（可对账），仅删三 id + `redacted:true`；state 不变（UNKNOWN 脱敏后仍 UNKNOWN，带 `redacted` 标志）。
- 租约栅栏：有租约的 reconciling UNKNOWN 行被跳过（`lease_until >= now` 不可脱敏）；worker 的 `complete_operation` 对行 `FOR UPDATE`，二者不会交错。adapter 对已脱敏请求 reconcile 返回 UNKNOWN、零 HTTP 调用（A1.4 条款 4）。
- 副作用幂等/可对账：run 与 erasure 共用 advisory key `claims-retention`；C4 由 `FOR UPDATE SKIP LOCKED` + `created_at` 排序分批；redact 幂等（`request ?|` 谓词已排除无 id 行，二次 run 计数 0）。
- **重放不脱敏 dm_send（已知限制，非本轮修复对象）**：`claims.replay_actor_erasures` 调 `apply_actor_erasure(platform, actor_key, NULL, NULL, NULL, NULL)`——`p_peer_keys` 恒为 NULL（墓碑只存 selector/actor digest，不含 peer key），故 `peer_key=ANY(p_peer_keys)` 永不命中，restore 重放无法脱敏 dm_send。实时路径由 P1-1 的 RD5 hold 保护（非终态 dm_send 会挡住 erasure）；重放路径下 dm_send 的 peer_key 残留是一个需要合同/架构裁决的缺口（apply_actor_erasure 的 public_reply 分支经 `v_comments` 无此问题）。CRP08 已显式注释该限制。
- **P2-5 幂等重试账本 TTL 结论（调查，无代码改动）**：A1.4 把 semantic_key 改为 `<prefix>-purged:<id>` 后，`internal/integrations/core` `Plan` 层按 `semantic_key` 去重（core/service.go:210-218）不再命中旧 key 的重试，只能靠 `internal/command` 的 HTTP 幂等账本 `ops.command_results`（按 `idempotency_key`=semantic_key，command.go:64/88）拦截。**调查结论：`ops.command_results` 全仓没有任何 TTL/purge（迁移与 Go 均未清理），即保留期 = ∞ ≥ intake_days，当前不变式「TTL ≥ intake_days」成立**——private_reply/dm_send/public_reply 的脱敏后重试由 layer-1 幂等账本捕获。任何未来给 `ops.command_results` 加 TTL 的改动，必须保证 TTL ≥ intake_days（且覆盖 private_reply/dm_send/public_reply 三类语义键），否则脱敏后的重试会越过去重、重新触达已被删除的 peer_key/comment_ref。此结论写入本 DELIVERY，供 integrator/架构裁决，不在本单元改代码。
- 架构与测试结果已区分 DESIGN / REAL_PG / NOT_RUN（见上表）。模型通过不等于产品通过；无 LIVE 流量、无真实 Meta 调用。

## NOT_RUN / BLOCKED

- NOT_RUN：全仓 `go test -race ./...`（AGENTS.md T02：T02 建立真实开发脚本后才跑；本单元只跑 touched foundation 子集）；`python3 experiments/spec_models.py --out experiments/results`（需要 owner 批准，本会话未运行）。
- BLOCKED（环境，与本单元无关）：`bash scripts/dev/check-gates.sh` 的 node 段在本 worktree 失败——`node_modules/typescript-api` 缺失（`package.json` 别名 `typescript-api: npm:typescript@6.0.3`，本 worktree 未安装）。本单元为纯 SQL + Go 测试变更，未触碰任何 node/UI 文件；其 Go/静态段已按脚本逻辑人工核对通过。合并方需先 `npm i` 再跑 `check-gates.sh`。

## Integrator 合并时

- 迁移号 `0127` 为占位，请按合并时实际空号重编号。
- **CRP02 equality（A1.4 P1-4 条款 5）**：`contracts/claims-retention-purge-v1.md` §4 的 `commerce_retention_writer / integration.operations` 特权行（L184）仍为旧值（`SELECT(...,created_at)`、`action='meta.private_reply'`、`semantic_key LIKE 'mpr-%'`）；请按本迁移 0127 的实际策略/GRANT 更新为四类 action + `lease_until` + 终态五 OR UNKNOWN 过期 + `-purged:` 正则。这是合同明确指定由 integrator 更新的行，本单元未触碰 `contracts/*`。
- 作者不能作为唯一验收人：REAL_PG 门禁由本执行者复跑（`test-focused '^TestClaimsRetention'`），合并后请 integrator 再复跑；`test-focused` 的共享锁 `/tmp/lc-test-pg.lock` 会串行化跨 Agent 的 PG 运行。

## 完成与失败

- 交付：实现、测试命令与退出码、证据文件（`output/lc-r1-retention/crp.log` 绿、`output/lc-r1-retention/red.log` 红）、风险、NOT_RUN/BLOCKED，均在本目录。
- review round 1 的 P1-1（erase_actor RD5 hold 扩到 dm_send/public_reply）与 P1-2（erasure/replay/hold 三路径测试）已实现并红→绿；P2 红证据（red.log）与 P2-5 幂等账本 TTL 结论已记录；本轮无 Go 源码改动（只改迁移 + 测试 + DELIVERY）。
- 本单元无 P0/P1 未解决项；无删除/放宽任何失败测试、阈值或 fixture；既有 `TestClaimsRetention*` 全绿；trunk 上与本单元无关的既有 foundation 红不在本单元范围。
- 遗留待裁决（非 P0/P1，已在本 DELIVERY 记录）：重放路径不脱敏 dm_send（`p_peer_keys` 恒 NULL）；`ops.command_results` 无 TTL（当前成立，未来加 TTL 须 ≥ intake_days）。
