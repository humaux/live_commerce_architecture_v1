# DELIVERY — unit live-a7-contains (受限「包含 KW+N」匹配模式 `KEYWORD_QTY_CONTAINS`, R5 A7)

- 角色: DeepSeek V4-Pro（后端执行者，按 brief「执行角色」）。
- Base SHA: `3a894ef2`（r3/integration；brief 草稿写 `4863db0`，实际 worktree 基点为 `3a894ef2`）。
- Worktree: `.worktrees/live-a7-contains`；branch: `unit/live-a7-contains`。
- Brief: `docs/delivery/units/live-a7-contains-match.md`（以「Integrator 裁决」节为准）。
- 迁移号: 暂定 `0115`（brief 裁决 #7：以合并顺序为准由 integrator 最终分配）。
- 状态: 后端实现完成，author 冒烟与门禁已跑；合同冻结、K3 对抗语料、UI、以及 2 个 integrator 预期红项待 integrator 合并时处理。

## 证据标签总览

| 证据 | 标签 |
|---|---|
| 本文件 / brief / 迁移注释 | DESIGN |
| 语法单元测试 + 向量 + fuzz | UNIT（MODEL_ONLY） |
| author 冒烟 `TestLiveA7ContainsMode`、PG 门禁 | REAL_PG（SANDBOX：一次性 PG 容器，语句全回滚） |
| 真实 FB/IG 留言流、浏览器 UI | NOT_RUN |
| `check-gates.sh` / `test-node.sh` / admin typecheck | NOT_RUN / BLOCKED（环境缺 node_modules） |

## 交付物（唯一写入路径，均在 brief 清单内）

实现：
- `internal/claims/grammar/contains.go` — `ParseContains`（kwc-v1，§2.5 单片段 + 否定/问句冻结字表 + §2.2 head/数量规则）与 `ParseForIngest`（kw-v1 优先，NO_MATCH 才回退 kwc-v1，模式无关、确定性）。
- `internal/claims/grammar/grammar.go` — `VersionContains = "kwc-v1"`；`Result.Version` 语义与 `redacted()` 收编两版（格式化仍只输出 Version/Kind，I11）。
- `internal/claims/claims.go` — `MatchKeywordQtyContains = "KEYWORD_QTY_CONTAINS"`。
- `internal/claims/merchant.go` — `validMode` 三值（默认仍 EXACT，`writeWindow`/`ensureWindow` 未动）。
- `internal/claims/ingest.go` — `effective(p, mode)` 纯函数（非 CONTAINS 且 kwc-v1 → kw-v1 NO_MATCH）；ingest 用匹配窗口模式、`readSource` 用已存事件 `match_mode`（I02，同来源重投不 409）；`offerReason` QUANTITY_REQUIRED 扩到 CONTAINS 且仍 `!Explicit`；事件 `grammar_version` 写 `p.Version`（`record` 与 `result` 两处，不再写死）。
- `internal/claims/manual.go` — `RecordManualClaim` 改调 `ParseForIngest`。
- `internal/claims/meta_intake.go` — `IngestMetaIntake` 读出 `grammar_version` 填 `p.Version`；`matchMetaWindow` 改用匹配 interval 的 `match_mode`（评论发生时的模式，修掉「用当前模式」限制）。
- `internal/integrations/meta/claim_intake.go` — `qualifyClaim` 改调 `ParseForIngest`（纯内存）。
- `internal/integrations/meta/consumer.go` — `stageClaim` 传 `candidate.Parsed.Version` 作为新 `p_version`（12 参）。
- `migrations/0115_claims_contains_mode.sql` — 前向、带 SHA-256 校验（runner `migrations/migrate.go` 计算并防篡改）；三处 CHECK 放宽 + QUANTITY_REQUIRED 替换（按定义查名）+ 新增 kwc-v1⇒CONTAINS 交叉约束；`claim_window_intervals.match_mode` 新列 + 回填 + 触发器写入；`insert_meta_intake`/`stage_claim_intake` 新签名（owner/EXECUTE/COMMENT 与 0064 一致，旧签名同迁移 DROP，依赖先删）。
- 测试：`tests/claims/kwc-v1-vectors.json`（§2.5 规范向量，53 条）；`internal/claims/grammar/contains_test.go`（含 `FuzzParseContains`）；author 冒烟 `tests/foundation/live_claims_contains_test.go`。

未触碰：`apps/**`、`go.mod`/`go.sum`、OpenAPI 共享 schema、pnpm 锁文件、`contracts/*`、MCI02/KC03 精确清单测试的预期行（integrator 所有权）。

## 红→绿证据

先红：author 冒烟/向量测试在实现前于当前代码上失败（kwc-v1 常量、`ParseContains`、`MatchKeywordQtyContains`、三值 CHECK 均不存在时编译/断言失败），实现后转绿。当前实现已在真实 PG 上绿。

- 单元 `go test ./internal/claims/...` — **ok**（`livecommerce/internal/claims`、`.../claims/grammar` 全绿）。
- 向量 `TestContainsKC01Vectors`（ParseContains 跑 `kwc-v1-vectors.json` 53 条）— **PASS**。
- `FuzzParseContains` 60s — **PASS**：8,738,879 execs，0 panic，corpus 138（16 seed + 122 发现）。见 `fuzz.log`。
- author 冒烟 `TestLiveA7ContainsMode`（REAL_PG，`test-focused.sh '^TestLiveA7ContainsMode$'`）— **PASS**：
  窗口 `KEYWORD_QTY_CONTAINS` 可开、interval 回写 `match_mode`、kwc-v1+CONTAINS QUANTITY_REQUIRED 登记、kwc-v1 在 EXACT/QTY_ONLY 23514、kwc-v1 explicit=true 23514、kw-v2 23514、裸 CONTAINS 23514、kw-v1 QTY_ONLY 回归 ok、kw-v1 EXACT 23514；`meta_intake_grammar_version_check` 含 `'kwc-v1'`；`claims.insert_meta_intake` 收 `p_version`（kwc-v1 → NULL 无源、kw-v2 → 22023）。
- PG 门禁 `test-focused.sh '^(TestLiveClaims|TestLiveClaimsContains|TestMetaClaimsMCI0[2-6]|TestClaimSource|TestLiveTools)'` — **PASS=58 FAIL=1 SKIP=0**（见 `pg-gate.log`）。唯一 FAIL 为预期 integrator 红项 KC03（下详）。
- 静态：`go build ./...`、`go vet ./...`、`gofmt -l`、`check-pkgdocs.sh`、`depmap.sh --check`、`python3 scripts/check_packet.py` 全绿（见 `static-gates.log`）。

## Integrator 红项（ruling #5，执行者不改测试预期行）

1. **KC03 definers（REAL_PG）**：`TestLiveClaimsKC03Schema/definers` FAIL — `claims.insert_meta_intake` 新签名 17 参（追加 `p_version text`），`tests/foundation/live_claims_schema_test.go:419` 的 want 行仍为 16 参。integrator 在合并时更新精确权限清单行（privilege delta 仅「新签名两行」）。
2. **MCI01 qualification（UNIT）**：`TestMetaClaimsMCI01Qualification/ig_non-keyword_text_still_qualifies` FAIL — `qualifyClaim` 按合同改调 `ParseForIngest`，`"nice!"` 现解析为 kwc-v1 `MATCH "NICE"`（单片段），而 `internal/integrations/meta/claim_intake_gate_test.go:167` 仍硬编码 `grammar.Parse`（kw-v1 NO_MATCH）。期望行应改为 `grammar.ParseForIngest`（合同 §3 变更）。测试文件不在本单元写入路径。

> 说明：早期记录曾把 MCI02 也列为预期红项；本次最终迁移正确保留了 `stage_claim_intake`/`insert_meta_intake` 的 owner 与 EXECUTE（`commerce_meta_consumer`/`commerce_meta_writer`），故 `TestMetaClaimsMCI02UpgradeAndExactPrivilegeDelta` **PASS**（含 `privilege_delta_equals_section_4.3`、`definer_hygiene_and_COMMENT_ON`、`interval_backfill_and_widened_CHECKs`）。

## 迁移校验

- `migrations/migrate.go`：内嵌 `[0-9][0-9][0-9][0-9]_*.sql` 按字典序应用；每文件 SHA-256 记入 `public.lc_schema_migrations`；已应用版本校验和不符即停（前向、防篡改）。
- 迁移正确性由三处证实：author 冒烟（fresh 应用两次）、KC03 `fresh-migrated-twice` PASS、MCI02 `existing_data_untouched` + upgrade（hold-back 迁移后二次应用）PASS。
- 完整 `release-gate.sh --strict --only G07`（全仓 `go test -race ./...`）**NOT_RUN**：需 Docker 全仓 race 跑 + 本次环境对 release-gate.sh 需显式批准；等价的迁移/门禁证据已由 `test-focused.sh`（同锁、同容器、同 `migrations.Apply` 夹具）覆盖。合并后由 integrator 跑一次。

## 风险与不变量

- 不变量 I02/I04/I09/I11/I14/I18 已按 brief 实现（`effective` 用已存模式保证同源唯一结果；暂存仍与社交事实同事务；无新身份字段；评论原文/未解析关键字仍不落库、Result 格式化只输出 Version/Kind；模式仍只 CLOSED 改；向量为空/SKIP 不算 PASS）。
- R1 误单缓解：单片段 + 否定/问句冻结字表 + 必须带数量 + 默认 EXACT + 按评论发生时模式判（`claim_window_intervals.match_mode`）。
- R3（非 CONTAINS 场次暂存行可带 `offer_id`）经 integrator 裁决 #4 接受，仍需 Claude 终审签字（brief 风险 R3）。
- 否定/问句字表为冻结数据；改字表必须是 kwc-v2（R5），不可原地改。

## NOT_RUN / BLOCKED

- NOT_RUN：真实 FB/IG 留言流命中率/误判率；IG `live_comments` 文本形态；繁简以外语言；与 SHOPLINE 判定逐条对照；浏览器 UI（`--browser-live-claims` KCC05，Codex 与 A5 UI 串行）；`release-gate.sh --strict --only G07` 全仓 race（合并后 integrator 跑）。
- BLOCKED（环境）：`check-gates.sh`/`test-node.sh`/admin typecheck — worktree 无 `node_modules`（`Cannot find module 'typescript-api'`），与本单元改动无关。
- BLOCKED（integrator 冻结）：合同 `contracts/*` 修改、`docs/delivery/GATES.md`、K3 对抗语料与 `live_claims_contains_gate_test.go`/`contains_fuzz_test.go`/`claims-ui.spec.ts`、MCI02/KC03 精确清单预期行。

## 协调备注

- 本单元把 `FuzzParseContains` 放在作者写入路径 `internal/claims/grammar/contains_test.go`；brief 把 `contains_fuzz_test.go` 列为 K3 文件。K3 增补对抗种子时请勿重名定义 `FuzzParseContains`（或 integrator 合并时合并），否则编译冲突。
- `consumer.go`/`claim_intake.go` 的 `p_version` 以 `candidate.Parsed.Version`（kw-v1|kwc-v1）传入；`stage_claim_intake` 与 `insert_meta_intake` 均校验 `p_version IN ('kw-v1','kwc-v1')`，否则 22023。

## 完成与失败

- 交付：实现、测试命令、退出码、证据文件（`pg-gate.log`、`unit-claims-meta.log`、`fuzz.log`、`static-gates.log`）、风险、NOT_RUN/BLOCKED 列表，均在本目录。
- 独立复跑：author 冒烟与 PG 门禁由本执行者跑通；最终验收（KC03/MCI01 预期行更新后）由 integrator 独立复跑，作者不能作为唯一验收人。
