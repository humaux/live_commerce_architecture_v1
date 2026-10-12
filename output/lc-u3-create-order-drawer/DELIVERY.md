# LC-U3 create-order drawer delivery

## PR32 merge1200178c — READY, local E3 (2026-10-13)

This is the current handoff; earlier sections retain their historical evidence. The money-path fixes were committed and pushed as `09bf1158bacaeb383a97074aa2772f6732da0aee` before this merge. Following the owner's instruction, fetched `origin r3/integration` and **merged FETCH_HEAD without rebasing**. Merge commit **`48102a4ed4d1309060a7ad2940fcb614da27197c`**, parents `09bf1158` and `1200178c5ffe3decaee09077e2bb8b7f59ea715c`.

- `CommentStream.tsx` retains the catalog/order/inventory permission gate, create-order entry, reset and parent privacy revocation, plus trunk's label-print provider, toolbar and row actions. Printing uses the same active-session/privacy fence. The conflicting inbox component host retains both React hook/context/portal sets. `playwright.config.ts` retains both `live-console.spec.ts` and `comment-label-print.spec.ts`; every Node test entry from both parents is retained. Registry layout remains intact.
- Source SHA256 **`214fff07d38bf7dcc86e5652bfdca064a95849a02b6e21ff26179fdf4c5f9b64`** binds all six runs below. Each run started and finished with clean tracked status and identical source/historical-output hashes. The final delivery commit changes only this document and these owned receipts, preserving that source digest.
- The merged tree's entire `internal/` delta from `09bf1158` is empty. The eight checked money-path files, including drawer, attempt, checkout and browser assertions, are byte-identical: [`merge120-money-unchanged.json`](merge-1200178c/merge120-money-unchanged.json). Earlier red→green and calibration evidence remains below; no new behavioral workaround, retry, timeout increase or weakened assertion was introduced for this merge.
- Implementer: Codex-3; parent runtime model/effort **NOT_EXPOSED**. Read-only audit helper configured `gpt-6-luna/medium`, own canvas only, no recursive delegation. Allowed resolution paths: `apps/admin/components/CommentStream.tsx`, `tests/admin/inbox-review-host.test.ts`; auto-merged suite/registry changes retain both parents. Delivery write paths: this file and `output/lc-u3-create-order-drawer/merge-1200178c/`. Owner `output/ext-agents/`, paused backups and historical output remain untouched.

| Exact command | Exit | Current merged-head result / run ID |
| --- | --- | --- |
| `cd apps/admin && pnpm exec tsc --noEmit -p .` | **0** | `merge120-tsc-20261013`, 1.718s |
| `bash scripts/dev/test-node.sh` | **0** | `merge120-node-20261013`, **1642 pass, 0 fail/cancelled/skipped**, 32 invocations, 41.766s |
| `bash scripts/dev/check-gates.sh` | **0** | `merge120-gates-20261013`, 208.138s; includes contract drift **errors=0, warnings=362** historical baseline, 83 documented modes, registry/header/evidence/backend ratchets and 1277 tests in 10 shards |
| `bash scripts/dev/test-local.sh --browser-manual-order` | **0** | `merge120-manual-order-20261013`, 133.476s; **Drawer29 + legacy10, REAL_PG14/14**, 111 passing drawer click rows |
| `bash scripts/dev/test-local.sh --browser-live-console` | **0** | `merge120-live-console-20261013`, 425.885s; workspace368.57s and labels35.89s, **52 label click rows PASS** |
| `bash scripts/dev/test-local.sh --browser-inbox` | **0** | `merge120-inbox-20261013`, 107.859s; real inbox chain/I11 credential verification, **61 click rows PASS** |

The browser modes ran sequentially, unfiltered, with known grep/calibration variables cleared. Existing ignored wrapper supplies only outer process bounds (120/300/600/900s), `GOFLAGS=-p=1`, process-group cleanup and source/status checks; test deadlines are unchanged. Exact commands, environment overrides, timing and hashes are in [`gates.json`](merge-1200178c/gates.json). Lossless `.log.gz` records preserve original bytes; [`RAW-LOGS.sha256.json`](merge-1200178c/RAW-LOGS.sha256.json) verifies decoded logs. [`FILES.sha256.json`](merge-1200178c/FILES.sha256.json) verifies the retained receipt files.

Current live-console evidence comes from **`output/playwright/run.uaBuE0HH/live-console-2176389442`** (workspace) and **`live-console-3713550877`** (labels), as identified by this run's log. No old top-level run supplies this acceptance. Both unchanged LCU2_404 cases produce real 404, privateRows=0, buyerPanel=0, sessionRetained=true and postLossReads=0. Native hidden state leaves server reads31→31. Receipts: [`live-console/`](merge-1200178c/live-console/). Inbox's native visibility receipt confirms private state cleared and revalidation on return. Queued-copy and 422 recovery real-click/readback receipts are in [`manual/`](merge-1200178c/manual/). Original full artifacts remain ignored; retained label/inbox ledgers omit raw actual/expected strings, with original hashes and transformations in [`receipt-provenance.json`](merge-1200178c/receipt-provenance.json).

Auxiliary `test-local.sh --list` and `pr-modes` checks exit0: 84 registry entries including foundation; selector returns54 modes. **Other51 selector modes, full foundation/G07 and optional LiveKit Node suite: NOT_RUN in this batch.** Earlier focused PG/race/vet/build results belong to their original SHA, not fresh runs here. Source-only whitespace check against trunk is0; a whole-merge whitespace check reports inherited trunk raw-log whitespace, preserved byte-for-byte rather than rewritten.

**E3: automatic local browser + disposable REAL_PG with signed MOCK IdP/Meta/console/edge; no LIVE provider, physical-printer or production acceptance.** All six requested modes are green on the merged source. Own gate processes have exited; independent review and merged-head CI remain pending. Authorized push is only to `unit/lc-u3-create-order-drawer`; no GitHub thread reply/resolution or trunk push.

## PR32 money-path follow-up — READY, local E3 (2026-10-13)

This section supersedes the old uncommitted/Git-readonly handoffs below. The owner authorized the necessary backend/contract/REAL_PG expansion and a push of **only** `unit/lc-u3-create-order-drawer`. No GitHub thread reply or resolution was made.

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u3-create-order-drawer`; pulled remote `8a0ec752` and preserved the integrator's parent revocation and SCOPE-404 changes.
- Final source commit: **`273ecd458e3a1d759b6cad7b6583e151cf424fe4`**. The final evidence commit changes only this delivery and owned receipts. All normal root gates bind the exact same tracked-source SHA256 **`9ecd532c412a9029adf35e46c8800231d64b8640056b9ebc086d18b8ff98a666`** (run HEAD `6840a124` plus the four-file delta committed as `273ecd45`). Each wrapper verifies unchanged source, tracked status and historical output throughout the run.
- Implementer: Codex-3 UI/BFF/integration. Exact parent runtime model/effort: **NOT_EXPOSED**. Backend worker configured `gpt-6.1-sol/high`, test worker `gpt-6.1-sol/medium`, read-only seam reviewer `gpt-6.1-sol/high`; selectors are configuration, not inferred runtime identities. Separate child worktrees, at most two writers, no recursive delegation. Root independently reviewed the seam and reran REAL_PG.
- Exact allowed write paths: [`r1-money-final/allowed-write-paths.json`](r1-money-final/allowed-write-paths.json), 26 source/contract/registry files plus this delivery and its four owned receipt directories. File hashes: [`r1-money-final/source-files.sha256.json`](r1-money-final/source-files.sha256.json). Owner `output/ext-agents/`, paused backups and historical output were preserved. No migration, grant, go.mod, lockfile or dependency changes.

### Findings and resulting behavior

- **4232390274:** a queued replay with `buyer_link=null` previously offered copy, which regenerated and invalidated the URL inside the queued DM. The copy control is hidden and the handler refuses regeneration unless `result.send.state === 'not_sent'`; three locales show the queued-link notice. Real replay proves zero regeneration and an original valid sealed URL after order reload.
- **4249176849:** the same fix enforces the exact `not_sent` rule at both render and action boundaries. The shared queued-copy red→green below covers both threads. Planned `meta.dm_send` remains untouched.
- **4232390282:** an arbitrary business 422 cannot prove that an earlier UNKNOWN request created no order. Additive A16R `POST /v1/admin/stores/{store_id}/orders/for-buyer/recover` receives the **original frozen body and key** through the real BFF/client seams. Strict authoritative 200 `created` returns the existing order; strict `not_created` commits a closure seal before the UI frees OrderGuard and permits explicit new intent. No blind new key on 422, UNKNOWN, network failure, malformed reply, 5xx or lost recovery response.

The server binds canonical full body/kind before work and serializes the complete attempt. The absence seal and original checkout share the exact Begin lock: recovery retains it through seal COMMIT, and both ManualOrder/for-buyer money writers check the closure under the lock through checkout COMMIT. A lost outer connection cannot let a late writer create an order after a seal. Post-wait permission/store/session checks prevent stale reads. Normalization clones shared input before validating; recovery can seal invalid recipient/payment input without re-running that failed business validation. The Quote remains the only price.

### Root gates on frozen final source

All logs/status JSON below are committed in [`r1-money-final/`](r1-money-final/). Raw logs in all four receipt directories are stored as byte-preserving `.log.gz`; decode with `gzip -cd`. [`RAW-LOGS.sha256.json`](r1-money-final/RAW-LOGS.sha256.json) verifies the original decoded bytes; older child manifests retain their original `.log` names/hashes. Original uncompressed run files remain in their ignored run directories. Status records include exact command, outer timeout, exit, HEAD, source hash and historical-output hash. Outer wrapper time includes the machine-wide PG queue; product/test deadlines are unchanged.

| Command | Exit | Actual result / run ID |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | **0** | **1557 pass, 0 fail/skip**, 31 invocations; `r1-money-node-final-20261013` |
| `cd apps/admin && pnpm exec tsc --noEmit -p .` | **0** | `r1-money-tsc-final-20261013` |
| `bash scripts/dev/check-gates.sh` | **0** | `r1-money-gates-final-20261013`; 83 documented modes, 1277 tests in 10 shards, HTTP coverage and evidence/header ratchets |
| `go run ./scripts/dev/contractdrift` | **0** | `r1-money-contract-final-20261013`; **errors=0, warnings=362** historical baseline; Go281/contract149/BFF304 |
| `bash scripts/dev/test-focused.sh '^Test(LiveConsoleRecovery\|LiveConsoleOrderForBuyer(Resume\|ConcurrentStaff)\|MerchantToolsManualOrder)$\|^TestLiveConsoleRecovery'` | **0** | `r1-money-pg-final-20261013`; **PASS=16 FAIL=0 SKIP=0**, Go39.539s; disposable REAL_PG |
| `go test -race ./internal/merchanttools ./internal/checkout ./internal/httpapi ./internal/httperror` | **0** | `r1-money-go-race-final-20261013`; all four packages |
| `go vet ./...` | **0** | `r1-money-go-vet-final-20261013` |
| `go vet -tags browser ./tests/foundation` | **0** | `r1-money-browser-vet-final-20261013` |
| `go build ./...` | **0** | `r1-money-go-build-final-20261013` |
| `bash scripts/dev/test-local.sh --browser-manual-order` | **0** | `r1-money-browser-final-20261013`; **Drawer29 + legacy10, PG14/14**, Go72.396s; no grep/calibration |
| `LC_DRAWER_CALIBRATION=new-key-on-retry bash scripts/dev/test-local.sh --browser-manual-order` | **1, expected RED** | `r1-money-key-calibration-r2-20261013`; full29, only `DU3-single-order-en-1440` fails; same body/different key, replay201, request orders2/receipts2, **PG15/14** |
| `bash scripts/dev/test-local.sh --list`; `node scripts/dev/pr-modes.mjs origin/r3/integration`; `git diff --check` | **0** | 84 registry entries including foundation; 54 selected modes; syntax/whitespace unchanged |

REAL_PG names: 13 `TestLiveConsoleRecovery*` tests, `TestLiveConsoleOrderForBuyerResume`, `TestLiveConsoleOrderForBuyerConcurrentStaff`, `TestMerchantToolsManualOrder`; full names and causal backend reds in [`r1-money-backend/DELIVERY.md`](r1-money-backend/DELIVERY.md). Root rerun includes connection loss, six post-wait authority variants, two-connection admission, invalid recipient/payment, missing merchant/checkout receipts, legacy ambiguity and both concurrent ordering outcomes.

### Red → green and real-click receipts

- Copy unit red: three locale failures against actual pre-fix component; green17/17. Real browser mutation removed only the copy render/action `not_sent` guards: `r1-money-copy-browser-red3-20261012` **exit1, full29, exactly one queued-copy failure, PG14/14**. Exact component backup restored to SHA256 `5640a8f15e94f4ed6b9eaf7500c506c49a48b138e0166f53e668c8c9b3a500ba`; normal final run above **exit0**. [`copy-browser-red/result.json`](r1-money-final/copy-browser-red/result.json) versus [`browser-green/result.json`](r1-money-final/browser-green/result.json): queued positive result, copy blocked, regeneration POST0, sealed operations1/secrets1/original_link=true/valid_links1/regenerations0, real persisted order reload/detail click. Both threads share this counterexample.
- Recovery reds: actual UI/BFF/client tests fail before A16R; authority/lost-reply counterexamples remain strict. REAL_PG reds demonstrate a late order after an unsafe seal, stale authority returning a private ID, invalid recipient/payment recovery refusal and shared-input race; final root16 pass. Browser drops the first real bank-transfer refusal response, replays exact key/body, positively observes real 422, clicks recovery, reads a durable not_created seal with zero order/success receipt, then explicitly starts new intent and reloads. Click ledger: [`browser-green/click-ledger.json`](r1-money-final/browser-green/click-ledger.json).
- Original key calibration stays separate from queued-copy assertions so a deliberately new key reaches the named exact-order failure rather than aborting on `buyer_link` shape. All normal queued assertions remain enabled. [`key-calibration-red/result.json`](r1-money-final/key-calibration-red/result.json) and independent [`pg-readback.json`](r1-money-final/key-calibration-red/pg-readback.json) prove the duplicate.
- Preserved non-acceptance failures: first copy browser run stopped on bank-only home option disappearing (`invalid_request` instead of exact bank refusal); the next hit the target plus compared JSON-escaped `&` rather than decoded sealed text. Fixed only fixtures, preserving asserted codes/counts. One normal run stopped while waiting for a foreign live PG lock: **NOT_RUN**, wrapper child exit -9. First key calibration timed out applying migration0119 at the existing20s setup limit, browser **NOT_RUN**; fixture cleanup verified, high host load observed, then one unchanged rerun reached the expected target RED. These logs are retained, never counted as product red/green. No threshold, assertion, product timeout, PG bound or retry policy was loosened.

### CI gates, limits and lifecycle

Changed existing browser spec/harness: `tests/admin/create-order-drawer-gate.mjs`, `tests/foundation/browser_create_order_drawer_test.go`, owned by **`--browser-manual-order`**; no new mode/fixture family. Exact selector output is [`r1-money-final/pr-modes.json`](r1-money-final/pr-modes.json): **54 modes**, of which the normal manual-order mode was run here. Remaining selector modes and **full foundation/G07 required for checkout changes: NOT_RUN locally**, for CI/independent review. Integrator's earlier live-console/inbox passes predate this backend delta and are not reused as its acceptance.

**E3: automatic local BROWSER + disposable REAL_PG, with signed MOCK IdP/Meta ingress/edge; no dispatcher, provider, SANDBOX/LIVE or production acceptance.** Independent source seam review E1 found no specific P0/P1 in the final reviewed seam. New backend expansion still requires independent money-path review and CI before merge. Fleet rollout must drain older unfenced writers before enabling recovery; legacy partial orders without the canonical binding, expired capabilities and inconsistent receipts remain UNKNOWN. Own runners/fixtures are cleaned; original failed proof, exact rollback backup and owner evidence retained. Shared state updates use Humaux locks; changed Go/TS/JS indexed and recovery/attempt rationale linked.

## Historical receipts — superseded as current handoff instructions

## PR32第一轮三P2未提交交接 — READY（2026-10-12）

- 最新裁决：**所有修改保持未提交**，集成方负责commit、`git merge FETCH_HEAD`、合并后最终门禁和push。Codex-3本轮没有fetch/add/commit/merge/push，暂存区为空，HEAD保持`7d03d087f0b7fa629d6f49f18c4974835bea3d47`。READY仅指当前工作树可供复核／提交，不代表PR已合并。
- Implementer: Codex-3，UI角色；runtime model/effort本端未公开，未推断。两位只读helpers为`gpt-6.1-sol/high`。允许写路径即下面精确14项；没有产品Go、迁移、生产ACL、契约或锁文件变化。
- 三条修复：UNKNOWN只接受七个可证明未建单的receipt后业务拒绝来结算，仍保留400/415、网络／5xx／auth／未知码的原body/key；限制检查按实际A15 offer匹配A13场次，缺失／冲突时三语明确无法检查；三个创建入口均增加catalog:read，真实无catalog角色不发A15/options/A16。
- Browser fixture修正：角色信息改用真实页面fetch。当前Chromium HTTP loopback的Secure cookie在APIRequestContext上未发送，合成cookie探针为API401／native200；没有伪造权限或放宽HTTP200断言。Go严格矩阵从26增加到29，并独立要求三语言各一条唯一的真实权限PASS记录。PG14与既有超时均未改动。
- 本轮最终tracked源码hash：**`e0abd876cfc6aae9002817a15e2a28b8b417c4ef3cbf3a838b14c59d3332ad35`**。未跟踪的新增`create-order-review.test.ts`另有逐文件hash；red→green期间逐字节相同。每次run的源码／历史output／tracked状态前后均相同；下面结果不依赖历史轮次的通过声明。

| 当前工作树命令 | exit | 结果／run |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | **0** | **1367 passed，0 failed，29调用**；`p2-handoff-final-node-20261012`，包括真实BFF seams与严格TZ依赖断言 |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | **0** | `p2-handoff-final-tsc-20261012` |
| `bash scripts/dev/check-gates.sh` | **0** | `p2-handoff-final-gates-20261012`；82模式、headers／registry；内含contractdrift：**errors=0，warnings=362**历史baseline |
| `go vet -tags browser ./tests/foundation` | **0** | `p2-handoff-final-vet-20261012` |
| `bash scripts/dev/test-local.sh --browser-manual-order` | **0** | `p2-handoff-browser-final-20261012`；**Drawer29＋legacy10**；真实PG **14/14**；Go PASS |
| `LC_DRAWER_CALIBRATION=new-key-on-retry bash scripts/dev/test-local.sh --browser-manual-order` | **1（预期RED）** | `p2-handoff-calibration-20261012`；仅`DU3-single-order-en-1440`失败；同body／不同key，HTTP201，PG15/14；该请求两个订单／receipt |
| `git diff --check` | **0** | 暂存区为空，所有改动未提交 |

证据：`output/playwright/lc-u3-k3-p2/p2-handoff-*.{log,status.json}`及`output/playwright/lc-u3-k3-p2/handoff-20261012/manifest.json`（含逐文件hash、日志hash及精确提交路径）；正常Drawer `output/playwright/create-order-drawer-3833881137/{result,click-ledger,pg-readback}.json`；legacy `output/playwright/manual-order-link-2464377956/`；校准 `output/playwright/create-order-drawer-4043610404/`。浏览器三条权限案例为三语言×390px、真实PG fulfilment角色／签名MOCK OIDC，含两个入口和reload；直接Drawer守卫另由actual-source Node覆盖。

RED保留：原拒绝死锁`p2-attempt-red.log`；actual-source三项反例`p2-resume-red-20261012.log`（11目标失败＋2正对照）；严格TZ未登记依赖`p2-related-node-final-20261012.log`；首次浏览器角色读取失败`p2-handoff-browser-20261012.log`；随后29项全部通过但Go仍要求旧26项的`p2-handoff-browser-green-20261012.log`；cookie根因`p2-cookie-transport-20261012.log`。没有删除断言、改低阈值、增加重试或延长超时。

**证据级E3：当前工作树的自动化BROWSER＋REAL_PG，外部IdP／Meta／TLS edge为MOCK。** 独立源码复核E1没有指定范围P0/P1／隐私P2；K3最终复核、merged-head CI／更广pr-modes、LIVE／生产部署仍未完成，由集成方处理。现有receipt JOIN及merchant-manual计数不单独外推所有checkout孤儿订单不存在。

### 精确提交清单与消息

修复及其共用测试是一个原子review批次，避免拆开Drawer／host／browser fixture形成中间失败提交。第一个逻辑提交包含这13个文件：

```text
apps/admin/components/BuyerPanel.tsx
apps/admin/components/CommentStream.tsx
apps/admin/components/CreateOrderDrawer.tsx
apps/admin/lib/create-order-attempt.ts
apps/admin/lib/create-order-copy.ts
apps/admin/lib/create-order-model.ts
scripts/dev/test-node.sh
tests/admin/create-order-attempt.test.ts
tests/admin/create-order-model.test.ts
tests/admin/create-order-review.test.ts
tests/admin/create-order-drawer-gate.mjs
tests/admin/timezone-ui.test.mjs
tests/foundation/browser_create_order_drawer_test.go
```

```text
fix(lc-u3): resolve order drawer retry, session and catalog review findings

Settle only proven order refusals after UNKNOWN, resolve restriction checks
from the actual prefill, and require catalog access at every entry.
Verify immutable retries and narrow-role controls through real browser/PG,
and keep the timezone fixture on the real attempt/model seam.

Implemented-by: Codex-3
```

第二个逻辑提交仅`output/lc-u3-create-order-drawer/DELIVERY.md`：

```text
docs(lc-u3): record round-one P2 verification and uncommitted handoff

Implemented-by: Codex-3
```

**不要提交**owner的`output/ext-agents/`或ignored运行产物；原7项paused.patch、确切stash及before/final备份保留。原测试与失败证据未被重写。集成方合主干时保留两边意图，尤其`test-node.sh`两边的每个entry；merged-head再跑必过门禁。受限环境记录下面仅作为历史，不是当前BLOCKED状态。

## PR32第一轮三P2修复 — 2026-10-12，BLOCKED／未提交

- Implementer: Codex-3，UI角色；本端未公开确切runtime model/effort。只读测试／安全复核：`gpt-6.1-sol`，high reasoning。Branch `unit/lc-u3-create-order-drawer`，工作树 `.worktrees/lc-u3-create-order-drawer`，HEAD/base `7d03d087f0b7fa629d6f49f18c4974835bea3d47`。**没有新提交或合并，不push。** 当前代码绑定文件hash，不能用以下历史轮次的通过结果替代本轮验收。
- UNKNOWN后确定拒绝：仅保留在checkout durable receipt查询之后、可证明未建单的七码结算白名单；解析400/415、通用conflict/not_found、未知码、401/403、5xx／网络UNKNOWN仍保持原body/key。Drawer结算后清除uncertain和OrderGuard；新意图仍只从显式new-attempt进入。
- 限制名单：BuyerPanel删除`claims[0]`场次推断；Drawer用实际A15 offer集合匹配A13唯一有效session，当前CommentStream直接传真实场次。缺失、截断、冲突或多场次信息保持三语“无法检查”，不猜身份或场次。
- 权限：Drawer、BuyerPanel和CommentStream三处均要求`catalog:read`+`orders:read`+`inventory:reserve`，三语原因明确商品目录权限。实际源码组件测试证明缺catalog时不发A15/options/A16；正向加载sentinel避免空页面假通过。
- 测试准备：修复旧browser草稿decoy变量作用域；库存UNKNOWN案例由真实UI输入1000，网络边界只丢首个真实409回复，重放同key/body。新增真实PG fulfilment角色两店精确grant、普通签名MOCK OIDC登录、三语390px入口与reload零请求案例。没有提高超时或修改预期PG14。该浏览器案例尚未运行。
- 时区回归：严格导入表发现新模型依赖，补入真实`create-order-model`；未列出的依赖继续抛错，没有fallback stub。
- Contract/interface changes: 无生产HTTP、契约、迁移或ACL变更；只新增owned test mux的fixture角色控制。

### 本轮证据与检查（均为合并前工作树）

日志及每次命令／timeout／exit／源码和历史output哈希：`output/playwright/lc-u3-k3-p2/p2-*-20261012.{log,status.json}`；逐文件哈希及新untracked测试哈希：`output/playwright/lc-u3-k3-p2/resume-20261012/final-manifest.json`。最终tracked源码hash `879bc4da540b6cbe59e7ef5887bc2ae41f397ee1ba925c26df51328f922736b6`；运行中源码／历史output保持不变。新review测试在red→green期间逐字节未变。

| 命令／run | exit | 实际结果 |
| --- | --- | --- |
| `node --test tests/admin/create-order-review.test.ts` / `p2-resume-red-20261012` | 1 | 13项：11个目标断言失败、2正对照通过；原UNKNOWN拒绝死锁red另见保留的`p2-attempt-red.log` |
| `node --test --experimental-transform-types` + review/attempt/model/client/copy/timezone-ui六套 / `p2-pure-node-final-20261012` | 0 | **42/42**，组件／客户端MOCK证据，含实际源码和网络边界fixture |
| `TZ=UTC`、`TZ=America/Los_Angeles`，`node --test --experimental-strip-types tests/admin/timezone-ui.test.mjs` | 0、0 | 各**7/7**；新增依赖未登记的red见`p2-related-node-final-20261012.log` |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 | admin类型检查；随后只改测试／文档，产品TS输入未变 |
| `go vet -tags browser ./tests/foundation`；`node --check tests/admin/create-order-drawer-gate.mjs`；`git diff --check` | 0 | 结构检查；不代表PG／浏览器通过 |
| `bash scripts/dev/test-local.sh --list` | 0 | 82模式；本单元属于**`--browser-manual-order`**，没有独立create-order-drawer模式 |
| `bash scripts/dev/check-gates.sh` / `p2-final-gates-premerge-20261012` | 0 | 包含漂移检查；82模式与header／registry检查通过 |
| `go run ./scripts/dev/contractdrift` | 0 | **errors=0，warnings=362**历史baseline；Go278／contract146／BFF295 |
| `bash scripts/dev/test-node.sh` | 1 | 首批455：452通过、3失败；`team-bff`无法listen127.0.0.1（EPERM），两个deploy-prep测试遭Docker socket拒绝；整个门禁未完成 |
| 包含真实`create-order-bff.test.ts`的相关Node检查 | 1 | 本地HTTP listener遭EPERM；同轮TZ新增依赖错误随后修复为两TZ绿色；BFF网络验收仍BLOCKED |
| `bash scripts/dev/test-local.sh --browser-manual-order` | 1 | admin production build成功，Docker socket拒绝，未启动Go/PG/浏览器案例；**NOT_RUN**浏览器结果 |
| `git fetch origin r3/integration`；`git add -- <owned paths>` | 255、128 | 分别无法写`FETCH_HEAD`和`index.lock`：Operation not permitted；未尝试绕过权限或rebase |

当前证据级：**E3限于42个组件／客户端MOCK与本地静态门禁；真实PG／浏览器NOT_RUN，整个PR32不READY。** 源码只读独立复核未见指定范围P0/P1／隐私P2（E1），不能代替K3或真实浏览器。现有浏览器订单观察只证明merchant-manual总数和receipt绑定计数，不能单独证明所有checkout孤儿订单不存在。

### 允许写路径、保留与续接

- 仅`apps/admin/components/{CreateOrderDrawer,BuyerPanel,CommentStream}.tsx`，`apps/admin/lib/{create-order-attempt,create-order-model,create-order-copy}.ts`，`scripts/dev/test-node.sh`，`tests/admin/create-order-{attempt,model,review}.test.ts`，`tests/admin/create-order-drawer-gate.mjs`，`tests/admin/timezone-ui.test.mjs`，`tests/foundation/browser_create_order_drawer_test.go`，本DELIVERY及owned ignored证据。没有产品Go／SQL／migration／lockfile改动。`output/ext-agents/`未修改。
- 原七项WIP的`paused.patch` SHA256 `7cfd496df313c80bbec909d79b5ec236d931e4fe5e77e6be239052a9c37dd11b`及确切stash保持原样。修改前文件、当前完整文件与patch保存在`output/playwright/lc-u3-k3-p2/resume-20261012/`；没有丢失工作。
- BLOCKED：运行时`.git`只读、loopback监听／Docker socket访问受限。只有实际运行权限改变后才能提交、`git fetch origin r3/integration && git merge FETCH_HEAD`、保留两边意图解决冲突，`test-node.sh`必须保留两边所有entry，再完整复跑本地门禁。缓存`origin/r3/integration` SHA见manifest，不视为本端fetch结果。
- CI gates／待复跑：`--browser-manual-order`（legacy+drawer全部真实点击／PG），`LC_DRAWER_CALIBRATION=new-key-on-retry`应精确RED single-order；入口回归`--browser-live-console`、`--browser-inbox`，以及合并后pr-modes要求的模式。calibration本轮NOT_RUN；全foundation／G07、LIVE／生产发布未运行。

以下为保留的历史轮次记录；其中“暂停”和通过结果只属于各自日期／SHA。

## Scope-404复验 — PR36主干合并（2026-10-10）

- 本轮只复验 LCU2_404。合并提交/测试源码 **`463cd8f0bfffe7a2031d559b9f240a9c966a205a`** 包含 SCOPE-404 PR36 **`287e08aa6c694030333dd00797c0d3bbac15abc6`**。三条 LC-U3 P2 继续暂停；本轮不代表整 PR32 或待修代码验收。
- 七项暂停修改已逐文件备份，只暂存这七个 tracked 路径；确切 stash **`d1f7cc33821ae7c92ef8cf882b64f43f7ef82cb7`**，原 patch SHA256 **`7cfd496df313c80bbec909d79b5ec236d931e4fe5e77e6be239052a9c37dd11b`**。未暂存 owner 的 untracked 文件，不使用 autostash/pop0。报告提交 `a22a3a0b` 后已按该确切OID恢复：apply0、七个原始文件hash及整个patch逐字节相同，无冲突；stash/备份保留。恢复记录：`output/playwright/lc-u3-pr36-recheck/restoration.json`。
- `live-console.spec.ts`、Go harness、`test-local.sh` 与合并前逐字节相同；spec SHA256 `3df8c6fa47d7e559ff535ed7f146d3dec307ce4d973f66ab65906216b4d1182b`。原 LCU2_404 15s 等404和6500ms零轮询观察、Go780s模式边界均未改动；未过滤测试或开启 calibration。

| 现有原完整模式／本地检查 | 结果 |
| --- | --- |
| `env -u LC_BROWSER_CONSOLE_GREP -u LC_CONSOLE_CALIBRATION GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-live-console` | 0；Go PASS，Playwright **23 passed**；完整模式398.598s |
| `bash scripts/dev/test-node.sh` | 0；**1352** passes |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 |
| `bash scripts/dev/check-gates.sh` | 0 |
| `go run ./scripts/dev/contractdrift -json` | 0；errors0 |
| `bash scripts/dev/test-local.sh --list` | 0；registry保持主干布局 |

| LCU2_404视图 | 实际HTTP | privateRows / buyerPanel | sessionRetained | postLossReads |
| --- | --- | --- | --- | --- |
| all | 404 | 0 /0 | true | 0 |
| private | 404 | 0 /0 | true | 0 |

原 spec 先要求 comment/buyer可见，再真实PG撤 grant，等待实际网络404，验证全部私密状态清空、登录session不变。恢复grant后6500ms零读取，reload后重新出现comment；没有响应拦截代替平台修复。只读独立证据复核（E1，未复跑）确认这两份输出及源码未改，无本轮缺口。

证据 **E3 REAL_PG + Chromium/生产Next/BFF/Go，MOCK IdP/Graph**，不声称 LIVE。四个最终runner记录同一源码 hash **`d3659b938cdfa3e80a4cd32e549a2008ec809897daa181239d997dfd169d9ece`**、clean tracked status、历史output hash全程不变。原始日志及status：`output/playwright/lc-u3-merge-validation/pr36-{live-console-full,node,tsc,gates}.{log,status.json}`；browser：`output/playwright/run.2mqhP0B2/live-console-3145764804/grant-revocation-{all,private}.json`。独立索引、stash/逐文件备份及源码对照：`output/playwright/lc-u3-pr36-recheck/`。浏览器生成证据留在 ignored run roots，不新增tracked浏览器产物。

本轮不改产品、测试、路由、契约或超时；只合主干并记录验证。paused WIP 不属于测试源码或本轮验收，恢复后也不能用此结果声称WIP已通过。manual-order/新key calibration及三P2验证、全foundation/G07、真实provider/LIVE/部署均 NOT_RUN。提交不push、不回复GitHub；后续复核/推送由集成方执行。


## Historical batch — trunk merge + W3-U2 blocklist warning (2026-10-09)

- Source commit **`3a0f6aa1b36c246127986eca38342c0516544935`**, parents old K3-reviewed `f515dcdb` and trunk `4ff99766`. Branch `unit/lc-u3-create-order-drawer`; own worktree unchanged. No push/deployment. Parent model/effort is not exposed by this runtime; read-only explorers explicitly used `gpt-6.1-sol/high`.
- Trunk registry is byte-identical (82 modes, `--list` exit0), along with the 13 reviewed LC-U2a seams/tests. Merge preserves trunk privacy/cursor/parcel/import routes and reapplies the six U3 CommentStream drawer hooks.
- Warning consumes the existing **W3-05B boolean-only GET**, no contract change: regular BFF `/live-sessions/{sid}/claims/blocklist/check?bundle_id=...`, exact canonical query, HTTP200 `{restricted:boolean}`, no-store/no-referrer, bounded response. The SID comes from the server A13 claim snapshot or the current CommentStream and is only a store-level path prefix. Every known A15 bundle is checked under a common18s budget; auth failure always clears private state, non-auth lookup failure cannot claim clear. Three locales show restricted/unknown guidance; restriction never changes Quote, body, key, permission or submit eligibility.
- Own functional write paths: `apps/admin/components/{CreateOrderDrawer,BuyerPanel,CommentStream}.tsx`, `apps/admin/lib/{claims-request,create-order-client,create-order-copy}.ts`, `apps/admin/proxy.ts`, the root store catchall BFF, `tests/admin/create-order-{client,bff}.test.ts`, `tests/admin/create-order-drawer-gate.mjs`, `tests/foundation/browser_create_order_drawer_test.go`, and this unit's curated delivery/evidence. Merge-only paths preserve trunk versions. No product Go, SQL, migration, schema, lockfile or W3-U2 UI edits.
- Real fixture uses existing signed Meta ingress/consumer/intake in the trade store, validates APPLIED/ACCEPTED/facebook/quantity2, and executes real A14 link + restriction POST. Cleanup is limited to its six sessions/restrictions; first consumer stops before the differently-keyed comment harness. New drawer runs use ignored `output/playwright/create-order-drawer-*`.

| Local command | Exit/result | Raw evidence in `output/playwright/lc-u3-merge-validation/` |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0,1295 tests | `node-final.{log,status.json}` |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 | `tsc-final.{log,status.json}` |
| `bash scripts/dev/check-gates.sh` | 0,82 modes/headers | `gates-final.{log,status.json}` |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | `vet-final.{log,status.json}` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` | 0,legacy10 +drawer25,PG14/14 | `manual-order-green-ignored.{log,status.json}` |
| Same mode with `LC_DRAWER_CALIBRATION=new-key-on-retry` | 1 expected,only `DU3-single-order-en-1440`,PG15/14 | `manual-new-key-calibration.{log,status.json}` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-live-console` | 0 | `live-console-green.{log,status.json}` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-inbox` | 0,real chain/I11 | `inbox-green.{log,status.json}` |
| `bash scripts/dev/test-local.sh --browser-admin-shell` | mode0,24 cases +role/store/axe | `admin-shell-green.{log,status.json}` |

- E3 evidence binds every final run to unchanged source hash **`3a8ceac72784de4ee5e958501f299245e3f95387af60758e2a9c271c6d32a137`**, committed verbatim in `3a0f6aa1`. Curated metadata: `merge-warning-evidence.json`; actual browser/Go/PG with synthetic data and MOCK identity/Meta only. Independent read-only merge and source/evidence review found no newP0/P1; integrator's K3 delta re-review remains required.
- Warning browser: `output/playwright/create-order-drawer-3722851862/{result,click-ledger,pg-readback}.json` has25/25, six real BuyerPanel warning clicks (en/zh-TW/zh-CN×1440/390), seven same-key/body one-order/receipt retries, and PG14/14. Calibration root `create-order-drawer-4163076078` has25 cases and exactly the intentional changed-key failure: same body, two request orders/receipts, replay201, PG15/14.
- New BFF gate non-vacuity: remove only the GET grammar for the actual authenticated root handler → **RED405 vs200 (exit1)**, restore exact bytes → GREEN0. `blocklist-root-bff-{red,green}-final.log`, `blocklist-bff-red-green.json`. Initial tools-route probe and manual-bundle422 are setup failures, not causal product red proofs. One consumer wait also failed without a diagnostic; no unproven root cause is claimed. All final normal/calibration runs completed without worker/cleanup errors. Early submit-enable assertion was moved to the valid payment/Quote state; it remains required before the real201 submit.
- Trunk's old Admin shell mode overwrote tracked evidence; the protection wrapper correctly exited99 despite mode0. All88 generated files were preserved under `admin-shell-evidence/`, then historical output restored byte-for-byte to hash `9e02b5a43e89f1c334d1f6b3721f0366d4cda6db5db9f4bf68241ed92b4e8aac`; source stayed unchanged and git was clean. See `admin-shell-evidence-restore.json`. Earlier untracked drawer runs are preserved in `legacy-layout/`; no historical screenshots/logs are committed anew.

### CI gates

Changed browser spec/mode remains only `--browser-manual-order`: `create-order-drawer-gate.mjs` and its Go fixture; no added mode. Local hosts rerun: live-console, inbox, admin-shell. Run manual-order and its expected-red calibration in CI; retain **click-sweep and visual-lint**, NOT_RUN locally per the frozen brief. `node scripts/dev/pr-modes.mjs origin/r3/integration` selects53 modes due to shared BFF/proxy/forms; the complete exact list is in `merge-warning-evidence.json.pr_modes`. Remaining selected modes, full foundation/G07, SANDBOX/LIVE providers and deployment are NOT_RUN locally. Integrator re-reviews this delta and pushes.


## Prior addendum — ManualOrder P1 (4212512394)

- Implementation `d0422a6c`; real regressions `0d91f236`, `778de008`; base previous delivery `6aabe6b6`. This addendum supersedes the original claim that ManualOrder submission behavior is unchanged.
- Root cause: editing the live draft after UNKNOWN regenerated its request key/body. The real baseline `addendum-edit-unknown-red.log` failed only `DU3-manual-unknown-draft-edit`: two attempts, different key and body, HTTP201, two request-specific PG orders/receipts (store total15 instead of14).
- Shared `OrderAttempt` now retains its immutable snapshot after both ambiguous and definitive responses. `finish()` only records definitive settlement; only the explicit `startNew()` action clears the receipt for a fresh key. ManualOrder keeps editable draft fields separate; retries ignore draft edits and use the original request. Drawer keeps editing blocked for unresolved attempts. Both submit handlers and buttons reject a settled receipt until the merchant clicks the localized new-attempt control.
- ManualOrder HTTP200 replay accepts the server-redacted configured/null buyer link without treating a successful replay as UNKNOWN; HTTP201 remains strict. Existing regeneration recovers that link using the original submitted locale. The default result parser remains strict.
- **Final real browser GREEN:** `addendum-edit-unknown-green.log`, `browser/20261008T154854.154170000/result.json`: 25 cases, PG14/14, legacy10/10. The edited ManualOrder retry has same key/body, HTTP200, PG delta1, exact one order and receipt. Both real422 refusal subchecks prove editing cannot enable submit, explicit new-attempt enables it, and clicking that action sends no POST and creates no order.
- **Calibration:** `addendum-calibration-red.log` exits1 only for `DU3-single-order-en-1440`; all25 cases run, PG15 instead of14. The ManualOrder edited-draft regression remains GREEN during this drawer fault.
- **Final local commands:** `bash scripts/dev/test-node.sh`, `bash scripts/dev/check-gates.sh`, `pnpm --dir apps/admin exec tsc --noEmit -p .`, `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation`, `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` all exit0. `LC_DRAWER_CALIBRATION=new-key-on-retry GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` exits1 as required. Evidence: `addendum-{node,gates,typecheck,vet,edit-unknown-green,calibration-red}.{log,status.json}`.
- All final commands share unchanged source SHA-256 `091a4f21bac71315adf405441d5bb11e23520b26fd56342ed73a621dc0dcd7ea`, committed verbatim in `d0422a6c`. Node lifecycle red→green: `addendum-lifecycle-{red,green}.log`.
- E3, real Chromium/Next/BFF/Go/PG with synthetic data and MOCK identity/provider. Read-only source spotcheck found no additional money/state issue; K3 independent acceptance remains pending. Mode remains `--browser-manual-order`; no new mode, product Go or contract change.
- Test author configured `gpt-6.1-sol/high`, separate `unit/lc-u3-retry-tests` worktree, own paths limited to browser driver and Go fixture. Root implemented/reviewed product and ran every runtime gate. Temporary test worktree archived then removed; delivery worktree retained. No push.

## Original delivery evidence (historical, before this addendum)


- Branch: `unit/lc-u3-create-order-drawer`. Base: LC-U2a `9ce09803`; frozen brief `ba2bab32`. No trunk merge, push, PR or deployment performed.
- Tested implementation: `faac7bf2` (preceding integration `142a58c5`, API `97b1243d`, shared form `d00d04c7`). All final gates share source SHA-256 `16e8ac45852239ae02f80174697bfc84f96003e63459dd38e705124badb5e988`; each `*.status.json` records command, base HEAD, timeout, elapsed time, source hash and unchanged-source check. The five then-uncommitted changes were committed verbatim as `faac7bf2`.
- Implementer: Codex-3, GPT-6 runtime family (exact parent deployment/effort not exposed). Child explorers and writers configured `gpt-6.1-sol`, high effort except browser mapping medium. Child worktrees were isolated; no recursive delegation.
- Contract/interface changes: none; consumes live-console-v1 §5/§6/A15/A16 and remaining-quantity amendment. UI/BFF and Go browser fixtures only; no product Go, migration, schema or lockfile change.

## Implementation

- `apps/admin/components/ManualOrderFormFields.tsx` and `ManualOrderItemPicker.tsx` share existing fields. Legacy ManualOrder submission, result and link behavior remain intact.
- `CreateOrderDrawer.tsx:79` opens a native modal from BuyerPanel or a claimed comment. Only A15 explicit customer links prefill contact/delivery; original claim facts retain individual remaining quantities even when request SKU quantities aggregate.
- A15-ineligible/live-permission warning precedes confirmation. Catalog unit references are informational; the server Quote supplies all actual prices and totals. No client price fields, total computation or identity inference.
- Exact tools BFF resources: GET `inbox/order-prefill` with one canonical selector and POST `orders/for-buyer` with closed body/header rules. Tests execute real Request/auth/origin/CSRF/store helpers, replacing only upstream network.
- `create-order-attempt.ts:6` freezes request bytes/key; UNKNOWN locks edits and replays that same receipt. A coarse boolean guard survives hide/unmount without storing buyer data, body, key or link. First-dispatch 401/403 also retains the guard because A16 can reauthorize DM after order commit. Session loss clears drawer and parent private state.
- Result copy covers every allowed coded error in three locales; duplicate bundle gives only the existing-order action. A9 conversation binding must exactly match B1 `dm_session` capability. Payment URL remains a transient ref and goes only to native clipboard; replay-redacted URLs use the existing stable-key regenerate-link endpoint.

## Red → green evidence

- `api/red.log`: missing model before implementation; `api/green.log`: real-seam focused cases pass. Existing manual parser remains strict; only A16 accepts replay-redacted configured links.
- `attempt-red.log` → `attempt-green.log`; `postcommit-auth-red.log` → `postcommit-auth-green.log`: immutable retries, storage fail-closed and auth-after-commit protection.
- `copy-red.log` → `copy-green.log`: all coded errors/reasons and eligibility warning, all locales.
- `browser-missing-drawer-red.log`: legacy flow reached new test; absent drawer generated no A15 request. Initial setup failure is separately retained in `browser-baseline-red.log` (test fixture log-stream fd not ready; fixed by awaiting open). Neither is represented as a product success.
- `browser-drawer-green-1.log`: real creation, copy and duplicate flow passed; missing address control test IDs exposed and corrected without changing assertions.
- Normal final `browser-drawer-green-2.log`: **24/24 matrix cases; PG 13/13 orders**, plus direct signed-ingress comment trigger and six session-boundary purges; unchanged legacy **10/10** cases. Details/screenshots: `browser/20261008T152835.002287000/`.
- `browser-new-key-calibration-red.log`: exit **1**, exactly `DU3-single-order-en-1440` fails; all 24 cases run. Mutation preserves body but forwards a new retry key at the real HTTP edge: two attempts, two request-specific PG orders/receipts, replay status 201, PG total 14 instead of 13. Details: `browser/20261008T152955.378435000/result.json`. Mutation is opt-in test-only, no product calibration branch.

## Local gates

| Command | Exit | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | `root-node.log`, `.status.json` |
| `bash scripts/dev/check-gates.sh` | 0 | `root-gates.log`, `.status.json` |
| `cd apps/admin && pnpm exec tsc --noEmit -p .` | 0 | `root-typecheck.log`, `.status.json` (equivalent pnpm --dir invocation) |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | `root-browser-vet.log`, `.status.json` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` | 0 | `browser-drawer-green-2.log`, `.status.json` |
| `LC_DRAWER_CALIBRATION=new-key-on-retry GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` | 1 expected | `browser-new-key-calibration-red.log`, `.status.json` |

Evidence class: **E3, REAL_PG + real Chromium/Next/BFF/Go**, synthetic fixtures and signed MOCK OIDC/Meta only. Component/Node tests are MOCK, not substitutes for browser evidence. Representative 1440/390 screenshots inspected; full-page captures include the underlying page below the viewport-height native dialog. Independent source spotcheck corrected the selector/privacy/remaining-quantity findings; K3 acceptance remains pending.

## Modes/specs and CI gates

Changed browser mode: **`--browser-manual-order`**, `TestBrowserManualOrderLink` in `tests/foundation/browser_manual_order_link_test.go`, new `browser_create_order_drawer_test.go`, new `tests/admin/create-order-drawer-gate.mjs`. No new mode or fixture-only UI route. Existing legacy spec stays intact.

Integrator CI: `--browser-manual-order` plus its new-key calibration; `--browser-live-console`, `--browser-inbox`, `--browser-admin-shell`, `--browser-click-sweep`, `--browser-visual-lint` for affected hosts/shared fields. These additional modes were **NOT_RUN locally**; required manual-order mode was run at the owner's explicit request despite the general CI-only browser default. G07/full foundation, real provider sends, SANDBOX, LIVE and deployment NOT_RUN.

## Limits and handoff

- More than five prefill bundles is refused without truncation; open a single claim bundle.
- After an unresolved request loses private memory, no status-by-key endpoint exists. The current store/auth scope stays blocked; no new key is silently generated and no private retry journal is persisted. External reconciliation/session recovery is an operational limitation for review.
- Direct comment/bare bundle cannot infer a messaging identity and therefore uses catalog pricing with a visible warning.
- K3 deep review, push/PR and merge belong to the integrator. Merge trunk only after the integrator confirms LC-U2a merge. No migration, grant or schema action requested.

Cleanup: owned form/API/browser/UI temporary worktrees were archived under the main checkout `output/lc-u3-create-order-drawer/worker-archives/` and removed; their commits remain. Requested delivery worktree remains. All owned gate runners ended and test-local cleaned its fixture containers/ports.
