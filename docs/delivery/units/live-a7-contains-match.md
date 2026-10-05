# Unit live-a7-contains-match — 受限「包含 KW+N」匹配模式 `KEYWORD_QTY_CONTAINS`（R5 A7）

状态：DRAFT（起草人 2026-10-05，待 integrator 审核/冻结）。Base `r3/integration` `4863db0`。
迁移号建议 **0115**（**待 integrator 确认**；与 live-a5-session-flow 的 0114 无依赖，可任意先后合并）。
后端 worktree `.worktrees/live-a7-contains-match`（branch `unit/live-a7-contains-match`）；UI 另开 Codex 任务
（`.worktrees/live-a7-contains-match-ui`，后端合入后开始，与 A5 UI 串行）。先读 `docs/delivery/PROCESS.md`、`AGENTS.md`、本文件；
合同只读 `contracts/live-keyword-claims-v1.md` §0.1(e)、§1、§2、§3（claim_windows/events）、§4.3、§5、§11.1 主持人提示，
`contracts/meta-claims-intake-v1.md` §3、§4、§4.4、§5.1、§5.3，以及 `架构.md` §11.1。

## 目标与 owner 原话

owner 裁决 2026-10-02（R5-PLAN A7 / M02，Humaux `2ccc47cf`）：
「留言中恰好出现一个带边界的「KW+N」才命中。出现否定词（不要/取消）、问句、多个关键字或「2H1+1」这类粘连时不命中，转人工。
按场次可选，默认仍为 EXACT。仍然否决纯「包含关键字、不带数量」的宽松模式。验收：用模拟器跑反例；「我要H1+1」「H1+1謝謝」命中，
「不要H1+1」「H1+1多少錢?」「H1+1 H2+1」「2H1+1」不命中。」
背景：试点商家在 SHOPLINE 用的是「包含 關鍵字+數量」（`output/live-console-research/competitors.md` A8、§6 第 2 条），
kw-v1 整条精确匹配会漏掉「我要H1+1」。

## 关键事实（读代码核实）

- 语法包 `internal/claims/grammar`（`grammar.go`，`Version="kw-v1"`，纯函数，`Parse/NormalizeKeyword/NormalizeLabel`）；
  包注释明写 non-goal「no substring/"contains" matching … no negation or question interpretation」；合同 §2.2
  「Never substring/"contains" matching (SHOPLINE "contains" mode rejected)」。
- 模式在 ingest 时应用：`claims.offerReason`（`internal/claims/ingest.go:328`），`MatchMode` 常量在 `internal/claims/claims.go:66-71`，
  `validMode`（`merchant.go:267`），窗口写入 `writeWindow`（`merchant.go:227`，模式只能在 CLOSED 时改，§0.1(e)）。
- 两个解析入口：手动 `claims.RecordManualClaim` → `grammar.Parse`（`manual.go:74`）；Meta 在 meta-worker 消费者里
  `qualifyClaim` → `grammar.Parse(text)`（`internal/integrations/meta/claim_intake.go:177`），只把结果（kind/keyword/quantity/explicit）
  经 `meta_inbox.stage_claim_intake` → `claims.insert_meta_intake`（0064，`'kw-v1'` 写死在 :538）暂存，**评论原文不落库**；
  claims-worker 的 `IngestMetaIntake` 在 apply 时才读窗口模式（`matchMetaWindow`，`meta_intake.go:104`，读的是窗口**当前**模式，
  已记录为「documented limit」）。
- 去重 `readSource`（`ingest.go`）比较 `grammar_version`、kind、quantity/explicit 等不可变事实。
- SQL CHECK（0060/0064）：`live.claim_windows.match_mode IN ('EXACT','KEYWORD_QTY_ONLY')`；`claims.events.grammar_version='kw-v1'`、
  `match_mode` 同上、`reason='QUANTITY_REQUIRED'` 要求 `match_mode='KEYWORD_QTY_ONLY'`；`claims.meta_intake.grammar_version='kw-v1'`。
- UI：`apps/admin/lib/claims-model.ts:11` `MatchMode` 精确两值；`claims-copy.ts` `hostPrompt`（冻结主持人提示，EXACT 版含
  「請只留關鍵字，不要加其他文字」）；`StudioClaims.tsx` `changeMode`（:328）。

## 范围

### 1. 语法 `kwc-v1`（纯函数，`internal/claims/grammar`）
新增 `ParseContains(text string) Result`（`Version="kwc-v1"`）与 `ParseForIngest(text string) Result`：
`p := Parse(text); if p.Kind == NoMatch { if c := ParseContains(text); c.Kind != NoMatch { return c } }; return p`。
`ParseContains` 规则，按序：
1. kw-v1 §2.1 第 0–3 步原样作用于整条（>256 字节或非法 UTF-8 → NO_MATCH；全角 U+FF01–FF5E −0xFEE0、U+3000/U+00A0 → 空格；
   去首尾空白；仅 ASCII a–z 转大写）。不做 NFKC，§2.1 末行的陷阱字符（﹢ ➕ ⁺ ① ² ſ ı K U+200B U+FEFF）照旧原样保留——它们在下一步都是「分隔符」。
2. **否定**：规范化后的整条含下列任一子串 → NO_MATCH：`不` `沒` `没` `別` `别` `勿` `莫` `取消` `算了` `退`。
   （故意偏保守：「H1+1 不錯」也不命中，转人工/重留言；漏判方向安全。）
3. **问句**：含 `?`（全角 `？` 已在第 1 步映射为 `?`）或 `嗎` `吗` `呢` `嘛` `幾` `几` `多少` `怎` `哪` `什麼` `什么` `啥` `是否` `可否` → NO_MATCH。
4. **取片段**：把整条切成「由 `[A-Z0-9+]` 组成的最大连续片段」（其余任何字符——CJK、空格、标点、emoji、换行、陷阱字符——都是边界）。
   片段数 ≠ 1 → NO_MATCH（覆盖：多个关键字「H1+1 H2+1」、`A1+2 B2`、价格「A1+2 1000元」、电话「A1+2 0912345678」、英文夹杂「I want A1+2」、`A1 +2`、`A1 2件`）。
5. 唯一片段 `s` 按 kw-v1 §2.2 解析（split 首个 `+`）：head `^[A-Z0-9]{1,16}$`，否则 NO_MATCH；无 `+` → MATCH quantity 1 explicit=false；
   tail 空或含非数字（`A1+2+3`、`A01+1A02+1`、`+2`）→ NO_MATCH；tail >3 位或前导 0 → INVALID_QUANTITY；否则 MATCH 1..999 explicit=true。
   粘连「2H1+1」「A01A02+1」整段成为 head `2H1` / `A01A02`：只有 session 里真有这个关键字才会解析到 offer，否则 `UNKNOWN_KEYWORD`——
   **永不从粘连里截出 H1**。
- 性质（必须有测试）：对任意文本，若 `Parse` ≠ NO_MATCH，则 `ParseContains` 的 (Kind, Keyword, Quantity, Explicit) 与之相同。
  因此 `ParseForIngest` 与模式无关、确定性（保留 §2「Parsing is mode-independent」）。
- 最大数量：仍 1..999、无前导 0；再受 offer 的 `max_quantity_per_claim`（`QUANTITY_OVER_MAX`）。

### 2. 模式规则（ingest，`internal/claims`）
- `MatchKeywordQtyContains MatchMode = "KEYWORD_QTY_CONTAINS"`；`validMode` 接受三值；默认仍 `EXACT`（`writeWindow`/`ensureWindow` 不变）。
- `effective(p, mode)`（纯）：`mode != CONTAINS && p.Version == "kwc-v1"` → `Result{Version:"kw-v1", Kind:NoMatch}`；否则 p。
  ingest 与 `readSource` 都用它（`readSource` 用**已存事件的** `match_mode`），所以同一来源重投在不同模式下仍得同一结果，不会 409。
- `offerReason`：`QUANTITY_REQUIRED` 条件改为 `(mode==KEYWORD_QTY_ONLY || mode==KEYWORD_QTY_CONTAINS) && !p.Explicit`——
  CONTAINS 下整条只写「A1」也不登记（owner：「仍否决不带数量」）。
- 事件 `grammar_version` 写 `p.Version`（不再写死 `grammar.Version`，`ingest.go` `record()` 与 `result()` 两处）。
- `A1+2` 仍是「把目标数量设为 2」（幂等，§1、架构 §11.1）；后到的 `我要A1+3` 设为 3；重复投递同一来源 = duplicate，不写。
- 手动入口 `RecordManualClaim` 改调 `grammar.ParseForIngest`；Meta 消费者 `qualifyClaim` 同样改调（meta-worker，纯内存）。

### 3. Meta 暂存与 apply
- `claims.meta_intake.grammar_version` 放宽为 `('kw-v1','kwc-v1')`；暂存函数新增 `p_version text` 参数（新签名
  `meta_inbox.stage_claim_intake(…,p_version)` → `claims.insert_meta_intake(…,p_version)`，旧签名在同一迁移 DROP），
  keyword 仍只用于解析 offer、不存。`IngestMetaIntake` 读出 `grammar_version` 填 `p.Version`。
- 修掉「apply 用窗口当前模式」的限制：`live.claim_window_intervals` 加 `match_mode`，由现有触发器
  `live.track_claim_window_interval()` 在 OPEN 时写入 `NEW.match_mode`；回填现有行 = 所属窗口当前模式；
  `matchMetaWindow` 改用匹配到的 interval 的模式（CONTAINS 的误判风险高于两种旧模式，必须按评论发生时的模式判）。
- 隐私：非 CONTAINS 场次的评论如果 kwc-v1 解析到 offer，暂存行会带 `offer_id`（不含原文/未解析关键字），apply 时降级为 NO_MATCH。
  这是对 R5「只在 head 命中 offer 时存 offer_id」的轻微扩展，需安全终审接受（见风险）。

### 4. 商家设置（admin）
- 按场次：现有 M2 `POST …/claims/window` 的 `match_mode` 多一个取值，body 键不变；仍只能在 CLOSED 时切换。
- UI：留言窗口区的模式选择改为三选一（EXACT「只留關鍵字」/ KEYWORD_QTY_ONLY「關鍵字+數量」/ KEYWORD_QTY_CONTAINS「留言中含 關鍵字+數量」），
  CONTAINS 选项旁给反例说明（否定、问句、两个关键字、粘连不会登记）；主持人提示新增 CONTAINS 版（替换第一句**且**删掉
  「請只留關鍵字，不要加其他文字」那一句），文案进合同冻结：
  zh-TW「留言「{KW}+數量」就能登記，例如「我要{KW}+2」；只留 {KW} 不會登記；一則留言只寫一個商品，不要問句」；
  zh-CN「评论“{KW}+数量”即可登记，例如“我要{KW}+2”；只发 {KW} 不会登记；一条评论只写一个商品，不要问句」；
  en "Comment {KW}+quantity, e.g. \"I'll take {KW}+2\"; {KW} alone is not counted; one item per comment, no questions"（其余句子沿用 EXACT 版）。
- 不做店级「默认模式」：复制上一场（live-a5-session-flow A5-2）已带模式；若 owner 仍要店级默认，另开小单元。

## Out of scope
纯「包含关键字不带数量」（继续否决）、一条留言多商品、语义/AI 解析、「+1」无关键字、商家自定义否定词表、留言转人工的 UI（波次 4 消息工作台）。

## 合同修改（integrator 冻结，实现前）
`contracts/live-keyword-claims-v1.md` 末尾新增「Amendment "KEYWORD_QTY_CONTAINS" (R5 A7, migration 0115)」，逐条替换：
1. §0.1(e) 与 §1：`match_mode` 取值加 `KEYWORD_QTY_CONTAINS`；「`KEYWORD` alone = target 1 (EXACT only)」不变。
2. §2 开头「Parsing is mode-independent」保留，补「ingest 使用 `ParseForIngest`；`kwc-v1` 结果只在 CONTAINS 窗口生效」；
   §2.2 末段「Never substring/"contains" matching」改为「Never unrestricted contains matching; the only contains form is kwc-v1 (§2.5)」。
3. 新增 §2.5 `kwc-v1`（上文 1 的完整规则与否定/问句字表，表为冻结数据）与 §2.6 向量文件 `tests/claims/kwc-v1-vectors.json`。
4. §2.3 precedence：`QUANTITY_REQUIRED` 条件加 CONTAINS。
5. §3 DDL 与 §3.1：三处 CHECK 放宽 + `CHECK (grammar_version='kw-v1' OR match_mode='KEYWORD_QTY_CONTAINS')`。
6. §4.3：`readSource` 比较 `effective(p, stored.match_mode)`。
7. §11.1 主持人提示：加 CONTAINS 版本（上文）。
`contracts/meta-claims-intake-v1.md` §4/§5.1 新签名与 `grammar_version` 取值、§5.3 step 2「interval 的模式」，删去
「late webhook uses the current mode」的限制说明；§4.3 privilege delta 表随新签名更新（MCI02/KC03 精确比对——**integrator 记录这一变化**）。
OpenAPI：无新 path（M2 不在任何 OpenAPI 文件中，`core-openapi.json` 只有 8 个 path）；合同 §7.1 表即权威。

## 数据 / 迁移 `0115_claims_contains_mode.sql`（编号待 integrator 确认）
- `ALTER TABLE live.claim_windows` DROP/ADD match_mode CHECK（三值）。
- `claims.events`：放宽 `grammar_version`、`match_mode` CHECK；替换 QUANTITY_REQUIRED CHECK；新增 kwc-v1⇒CONTAINS CHECK。约束名先在迁移里
  `SELECT conname FROM pg_constraint` 断言存在再删（防漂移，0110 的 `drift` 写法）。
- `claims.meta_intake.grammar_version` CHECK 放宽。
- `live.claim_window_intervals ADD COLUMN match_mode text`（回填后 `SET NOT NULL` + 三值 CHECK）；`CREATE OR REPLACE live.track_claim_window_interval()`。
- `claims.insert_meta_intake` / `meta_inbox.stage_claim_intake` 新签名（owner、EXECUTE 与 0064 相同；旧签名 DROP；`COMMENT ON` 更新）。
- 不新增角色、表 GRANT 或 River kind。

## 不变量
I02（同一来源/幂等键只对应一种规范化结果：`ParseForIngest` 确定性 + `effective` 用已存模式）、I04（暂存与社交事实同事务不变）、
I09（actor ≠ 人；不新增身份字段）、I11（评论原文、未解析关键字仍不落库、不进日志；`Result` 的格式化仍只输出 Version/Kind）、
I14（模式只在 CLOSED 改，CAS 不变）、I18（向量文件为空/SKIP 不算 PASS）。I03/I05 不受影响（认领仍不碰库存/钱）。

## 写入路径（唯一归属）
后端（DeepSeek V4-Pro）：`internal/claims/grammar/**`、`internal/claims/{claims.go,ingest.go,manual.go,merchant.go,meta_intake.go}`、
`internal/integrations/meta/claim_intake.go`、`internal/integrations/meta/{consumer.go,runtime.go}`（只改 stage 调用参数/签名引用）、
`migrations/0115_claims_contains_mode.sql`、`tests/claims/kwc-v1-vectors.json`（**只放合同 §2.5 的规范向量**）、
作者冒烟 `tests/foundation/live_claims_contains_test.go`。
UI（Codex）：`apps/admin/lib/{claims-model.ts,claims-copy.ts}`、`apps/admin/components/StudioClaims.tsx`（只动窗口/模式区，挂载点不动；
与 A5 UI 串行）、三语。
独立对抗测试（K3，不同模型家族，不得看实现 diff 之前先写）：`tests/claims/kwc-v1-adversarial.json`、
`tests/foundation/live_claims_contains_gate_test.go`、`internal/claims/grammar/contains_fuzz_test.go`、`tests/admin/claims-ui.spec.ts`（扩展用例）。
integrator：`contracts/*`、`docs/delivery/GATES.md`、KC03/MCI02 精确清单测试的预期行。

## 执行角色
后端 DeepSeek V4-Pro；`apps/` 只 Codex；K3 写独立反例语料与对抗测试并做跨家族审查；Claude 终审（隐私扩展、privilege delta、
是否接受否定/问句字表）。

## 测试计划（先红后绿）
**必备反例语料**（K3 产出 ≥120 条，每类 ≥8 条，每条标注预期 kind/keyword/quantity/explicit 以及 EXACT/QTY_ONLY/CONTAINS 三模式下的 ingest 结果；
起草人给的种子，必须全部包含）：
| 类别 | 种子 | 预期（CONTAINS） |
|---|---|---|
| 命中 | `我要H1+1`、`H1+1謝謝`、`h1+1`、`ｈ１＋２ 感恩`、`+++我要A1+3!!!`→（片段数 2：`+++`、`A1+3`）NO_MATCH、`A1+2😍` | H1×1、H1×1、H1×1、H1×2、NO_MATCH、A1×2 |
| 否定 | `不要H1+1`、`H1+1不用了`、`取消H1+1`、`H1+1 算了`、`別幫我H1+1`、`沒有要H1+1`、`H1+1退掉` | NO_MATCH |
| 问句 | `H1+1多少錢?`、`H1+1？`、`H1+1嗎`、`H1+1是不是紅色`、`H1+1有幾個`、`哪個是H1+1` | NO_MATCH |
| 多关键字 | `H1+1 H2+1`、`H1+1\nH2+1`、`H1+1,H2+1`、`H1+1 和 B2`、`H1+1 H1+1` | NO_MATCH |
| 粘连 | `2H1+1`、`A01A02`、`A01A02+1`、`H1+1H2+1`、`H1+12+1`、`H1+1x2` | UNKNOWN_KEYWORD 或 NO_MATCH，**永不** H1 |
| 价格/电话/数字 | `A1+2 1000元`、`1000`、`A1+1000`、`A1+02`、`0912345678`、`A1+2 0912-345-678`、`A1+0` | NO_MATCH / UNKNOWN_KEYWORD / INVALID_QUANTITY（逐条标） |
| 空白/全角/大小写 | `A1 +2`、`A1+ 2`、`Ａ１　＋２`、`a1+2`、`\tA1+2\n`、`A1​+2`（U+200B） | NO_MATCH、NO_MATCH、NO_MATCH、A1×2、A1×2、NO_MATCH |
| 无数量 | `我要A1`、`A1`、`A1好看` | QUANTITY_REQUIRED（已知 offer）/ NO_MATCH 规则逐条标 |
| 边界 | 256/257 字节、非法 UTF-8、`A1➕2`、`A1﹢2`、`ABCDEFGHIJKLMNOP+1`（16）、17 位 head | 按 §2.1/§2.2 |
- KCC01 UNIT：kw-v1 向量全绿不变（回归）；kwc-v1 规范向量 + K3 对抗语料逐条；性质「Parse≠NO_MATCH ⇒ ParseContains 等价」；
  `FuzzParseContains`（不得 panic、Keyword 必为输入某片段、含否定/问句字符必 NO_MATCH、追加任一 `[A-Z0-9]` 片段必 NO_MATCH）。
  红跑：在 `ParseContains` 里去掉否定检查 → 否定类必须失败。
- KCC02 REAL_PG「模拟器」：同一语料经 `RecordManualClaim` 在 EXACT / QTY_ONLY / CONTAINS 三个场次跑真实 ingest，断言 outcome/reason、
  事件 `grammar_version/match_mode`、`我要A1+2`→`我要A1+3`→`A1` 的目标数量 2→3→QUANTITY_REQUIRED（不回 1）；同来源重投 duplicate。
- KCC03 REAL_PG（MOCK Meta）：签名 webhook → meta-worker 消费者 → 暂存（`grammar_version='kwc-v1'`）→ claims-worker apply；
  CONTAINS 窗口命中、EXACT 窗口降级 NO_MATCH 且无 offer 写入事件；关闭→改模式→重开后，迟到 webhook 按**旧 interval 模式**判（红跑：现
  `matchMetaWindow` 读当前模式）。
- KCC04 SQL：三处 CHECK 双向（`kwc-v1` + `EXACT` 插入被拒）；KC03/MCI02 精确权限清单只变动新签名两行。
- KCC05 BROWSER（MOCK）：`--browser-live-claims` 中 CLOSED 时切到 CONTAINS、主持人提示三语、手动记录「我要A1+2」被接受、「不要A1+2」显示「看不懂」计数。

## 门禁命令（已核实存在）
- 静态：`go build ./... && go vet ./...`、`gofmt -l`、`bash scripts/dev/check-pkgdocs.sh`、`bash scripts/dev/depmap.sh --check`、
  `bash scripts/dev/check-gates.sh`、`bash scripts/dev/test-node.sh`、`python3 scripts/check_packet.py`、admin typecheck。
- 单元：`go test ./internal/claims/...`；`go test -run='^$' -fuzz='^FuzzParseContains$' -fuzztime=60s ./internal/claims/grammar`（与现有 `FuzzParse` 同写法）（作者与 K3 各一次，记录 corpus 数）。
- PG：`bash scripts/dev/test-focused.sh '^(TestLiveClaims|TestLiveClaimsContains|TestMetaClaimsMCI0[2-6]|TestClaimSource|TestLiveTools)'`。
- 迁移：`bash scripts/dev/release-gate.sh --strict --only G07`（单元验收与合并后各一次）。
- 浏览器：`bash scripts/dev/test-local.sh --browser-live-claims`（现有模式，扩展 spec，无新模式）；回归 `--meta-consumer`。

## 证据标签
语法/ingest：UNIT + REAL_PG（MOCK 手动入口与 MOCK 签名 webhook）。真实 FB/IG 留言流 = NOT_RUN。本文件 = DESIGN。
证据 → `/Volumes/data/live_commerce_architecture_v1/output/live-a7-contains-match/`。

## 风险
- R1 误单（最高风险）：包含式天然比整条匹配更易误判；缓解 = 单片段 + 否定/问句字表 + 必须带数量 + 默认 EXACT + 按评论发生时的模式判；
  K3 对抗语料不过不合并。字表偏保守会漏单（「H1+1 不錯」），由商家看「看不懂」计数后手动记录。
- R2 英文/混写留言（`I want A1+2`）一律不命中——台湾中文场景可接受；若试点反馈多，再开字表扩展单元，不在本单元放宽。
- R3 暂存行在非 CONTAINS 场次也可能带 `offer_id`（隐私轻微扩展），需 Claude 终审签字。
- R4 新签名替换 0064 函数会动 MCI02/KC03 精确清单；漏改清单 = G07 红（PROCESS.md §2.6 的 R5 wave 2 教训）。
- R5 kwc-v1 字表一旦冻结即进事件的 `grammar_version`；以后改字表必须是 `kwc-v2`，不能原地改。

## NOT_RUN / BLOCKED
- NOT_RUN：真实 Meta 留言（LIVE）下的命中率/误判率；IG `live_comments` 的文本形态；繁简以外语言；与 SHOPLINE 实际判定逐条对照（无其后台访问）。
- BLOCKED：合同修改待 integrator 冻结；否定/问句字表需 integrator 确认（可请 owner 看一眼）；UI 等后端合入且与 A5 UI 串行。

## Integrator 裁决（Claude Opus，2026-10-05）
1. 复制场次**连同直播价一起复制**（对齐 SHOPLINE「复制上一场」，owner 2026-10-05 要求一键复制开播）；R4 live-tools 规则 2「不复制直播价」仅对「从其他场次复制关键字商品」保留，对整场复制作废。
2. 场次归属采用 `claims.live_price_uses ∪ claims.order_origins(0113)`，替换 `claims.order_live_sources`；订单页场次筛选随之改变，须在测试里覆盖「无直播价的认领订单」归入场次。
3. 一单跨多场次：各场全额计入，接口返回 `multi_session_orders` 供界面脚注；不做分摊。
4. A7 `kwc-v1` 否定词/问句词表按草案的保守版本冻结；非 CONTAINS 场次的暂存行可带 `offer_id`（不含留言原文）——接受。
5. MCI02/KC03 的精确权限清单由 integrator 在合并时更新，执行者不改。
6. 已有购物车：**合并**（不替换）；兑换前先 `continueShopping`，已下单的购物车不得二次下单。
7. 迁移号暂定 0114（A5）/0115（A7）/0116（售罄读模型），以合并顺序为准由 integrator 最终分配。
8. 新增门禁模式批准：`--browser-live-flow`、`--browser-claim-checkout`，`--browser-webkit` 增第 7 步 `claim-checkout`（release-gate 行 6→7 步）。
9. 粉专直播中影片挑选保持 MOCK；status 取值与 U1（影片 post_id 是否等于留言 webhook 的 post_id）待 owner 测试账号做 LIVE 探针。
10. 本轮不做店铺级默认匹配模式（复制场次已带模式）。

## §2.5b kwc-v2（2026-10-05，K3 对抗测试 F1–F3 后）
- 新的 CONTAINS 解析一律用 `kwc-v2`；已存事件按其记录的 `grammar_version` 重放（kwc-v1 冻结不变，I02）。
- 问句表在 kwc-v1 基础上加 `問`（含 請問）、`如何`、`價格`、`價錢`。
- 多字否定/问句词容忍中间夹的边界字符（空格、emoji、标点、零宽字符）：`取 消A01+2`、`算😍了A01+2` 均不成单。
- 片段前紧邻非 ASCII 字母（形近字，如西里尔 А、希腊 Α）且带显式 `+N` 时拒绝（NO_MATCH），不切出另一个关键字。代价：CONTAINS 下 `我要01+2` 转人工。
- 证据：unit/k3-a7-adversarial（K3，160 例）+ unit/kwc-v2（152c614c/f17cd431）；迁移 0120 放宽 grammar_version 白名单。
