<!-- Purpose: current W3-U3 author READY handoff with pinned runtime and regression evidence.
Depends on: frozen label brief, approved BFF scope-race fix, PR30 receipts and current trunk.
Used by: integrator K3/PR preparation; local E3 is not independent or production acceptance. -->
# W3-U3 comment label print — DELIVERY

**Current status: WAITING_SCOPE404 — BFF candidate reverted as e4e48ec5; not current full-mode READY. No push/deploy.**

## Latest integrator ruling (2026-10-10)

K3 passed the implementation of58510545, but the integrator rejected shipping the BFF second-opinion classification:
LCN03 belongs to Go, covers every domain, and stale views can turn a genuine403 into404 and alter BuyerPanel revocation.
**Exact revert e4e48ec581c8be7309521f20dc4497bee9ca08cc** restores auth.ts, the store catchall and inbox-bff.test.ts to
58510545's parent. The original14 BFF re-proof cases/helper design is preserved in `scope404-revert/inbox-bff-test-design.patch`;
it is not silently skipped or kept as an active, rejected contract. The print/label/P2 files and original browser spec are unchanged.

On e4e48ec5: **test-node1376/1376 exit0, admin tsc0, check-gates0**. Current two original full browser runs are **NOT_RUN**,
waiting for the separate **SCOPE-404** Go unit to merge into trunk. W3-U3 will not duplicate that Go fix or revive BFF re-proof.
After confirmed merge: merge origin/r3/integration, freeze a source SHA and run `--browser-live-console` twice serially,
unchanged waiters/assertions/timeouts. Quiet thread follow-up is registered; details: `scope404-revert/DELIVERY.md`.
Previously reported3952ed32/d8744793 full-mode greens below are historical candidate evidence, not current acceptance.

## Historical candidate correction and BFF fix (superseded)

- The previous LC-U2a ownership attribution is withdrawn. Task **8f1bb58f** is canceled as **误交接**; W3-U3 owned and diagnosed the failure. Main-trunk control `b1bfbeb3` passed; integrator also supplied two passing `729afff9` controls.
- Required unchanged serial runs on `baab02a1`: **exit 0 (23 workspace +9 labels)**, then **exit 1 (22 workspace +9 labels; private wait timeout)**. These historical reds remain in `correction-reruns/`.
- Captured root: A13 returned **403 after its BFF admission check crossed grant revocation**. Correct child/parent privacy expiry immediately cleared the stream and canceled A2; the untouched waiter required a 404. No print request or hidden-page transition caused this failure. Evidence: `correction-reruns/hook-red/`.
- Owner approved the narrow shared-BFF repair: only known private GET/403 is rechecked with the same server-authenticated bearer. Confirmed lost store scope becomes404; genuine permission denial, unknown/unavailable proof and writes remain unchanged. Proof is capped at500ms within a7s absolute server budget and cancels with the caller, so a known refusal cannot be delayed into the client's8s timeout/503.
- Fix commit **58510545**; current trunk **729afff9** merged as **d8744793a03a64281d4481d2bbe90ba5f2b2e98d**. All temporary observations were removed. Both hooks and the full original workspace/label specs are byte-identical to `baab02a1`; no Go/SQL/DTO/contract change.
- Final author gates: **live-console exit0 (23+9)**, **inbox exit0 (13)**, **Node1390/1390 exit0**, **admin tsc0**, **check-gates0**. A3 lost ACK remains same key/count **[7,7]**; final9 ledgers contain45 independently observed PASS rows and18 screenshots.

**Current commands, exact source hashes, red/green evidence, CI gates and NOT_RUN: [correction-reruns/DELIVERY.md](correction-reruns/DELIVERY.md).** Independent K3/PR CI and physical printing remain pending; READY is not release approval. The older sections below are historical checkpoints, not current blockers.

## Historical post-PR30 checkpoint (superseded)

Merged origin `b1bfbeb3` (#30 principal-bound receipts), #23/#25 as **042758c5**. Original label/workspace specs and label component unchanged.
Original lost-ACK case now GREEN: actual sameKey=true, counts=[7,7]. Labels9/9 and current45 observed ledger rows all PASS.
Full `--browser-live-console` still exits **1**, solely because two existing LCU2_404 workspace waits timeout (workspace21/23).
No test, timeout or assertion changed. Node1366/0, tsc/check-gates exit0. The former LC-U2a handoff is canceled, not an external dependency.
**Historical commands/exits,18 screenshots and ledger: `pr30-ready/DELIVERY.md`. Superseded by the final acceptance above.**

## Historical K3 round-one P2 batch (before PR30 integration)

- Current tested source **b245fef897837aaebefcd56ac90a1b475cd195bc**. Two requested P2 fixes are implemented; current full browser acceptance remains **NOT_RUN / NOT_READY**.
- Ledger actual now records observed DOM/API counts, states, dimensions and reload values, with outcomes derived from those values. It never copies expected; failures retain safe actual state. Names/text/ref/key are not written to the ledger.
- Label 401/403/404 resets busy/running locally before onDenied, independently of remounting.
- Red→green: 3 same-mounted component denials plus 2 actual-source recorder cases. New tests are registered in test-node. Full Node **1319/0**, admin tsc **0**, check-gates **0**; spec tsc **0** with existing admin Node types. Browser collection is 9 cases, not acceptance.
- Original browser expect-call sequence **53→53, identical**; lost-ACK key/count assertions and network transaction/drop logic are retained.
- Evidence, commands/exits and limitations: **`k3-p2/DELIVERY.md`**. Previous browser shots and copied-expected ledgers below are **historical**, not current observation evidence.
- Read-only origin at **7975160e** still has the pre-fix PrintComment signature. PR **#30 / unit/lc-a3-print-idempotency /34587db4** is not integrated into this branch. Rerun after backend PR merge before reporting READY.

## Source and scope

- Branch `unit/w3-u3-comment-label-print`, own worktree only.
- Resumed `c012cf08`; merged fetched `origin/r3/integration 4ff99766` (#18) as **53023aaf**. Kept the single mode registry and every trunk privacy/cursor fix.
- **5200bf06**: label scoped-404 authority clear, latest management permission guard across awaits, lost-ACK regression.
- **26d665f2**: each real control and paper-preference reload assertions; reset case runs last.
- **7877c4ef014604d4f16617364568cef958374235**: final tested source; per-case ledgers survive Playwright worker restarts. Final evidence commit changes no product/test source.
- Root author: Codex-1; exact runtime model/effort unavailable. One read-only explorer `gpt-6.1-sol` / medium, one merge audit + bounded recheck; E1, not independent K3.
- Product write paths: brief's CommentLabelPrint / print.css / minimal seven-line CommentStream entry. Approved tests: Playwright registration, isolated Go TEST labels fixture and faithful Context/Portal host. No product Go/SQL/DTO/contract, dependency or lockfile change.
- Test-only merger preserves trunk's digest sync-throw/quiescence fixes and all original inbox review/recovery assertions. Those test files have zero diff against trunk.

## Implemented path

Current keyword rows can be checked/un-checked or printed singly. Preview shows transient name, keyword, quantity, Taipei time and short session code; native CSS supports 60×40mm / A4 three columns. Only the paper preference enters storage. Labels remain browser memory/DOM, not the API or DB. No new printing library or duplicate comment/reply reader.

A3 sends exactly `{}` with one UUID key per ref through the existing CSRF/session transport. Badges use only confirmed counts and survive a real A2 reload. Network failures still permit native printing without claiming a new record; UNKNOWN retains the same key for an explicit retry, never an automatic retry. Scoped 401/403/**404** expires private state. Latest management permission is checked before/after awaits; the portal also gates on current authority.

## Historical initial acceptance (7877, before the K3 P2 changes)

| Command / tested input | Exit | Counts / evidence |
|---|---:|---|
| `bash scripts/dev/test-node.sh` on 7877 | 0 | **1308 executions, 0 failures**; `resume/node-delivery.log` |
| `pnpm --filter admin exec tsc --noEmit` on 7877 | 0 | `resume/tsc-delivery.log` |
| `bash scripts/dev/check-gates.sh` on 7877 | 0 | 82 modes documented, 1253 top-level Go inventory; `resume/gates-delivery.log` |
| `bash scripts/dev/test-local.sh --list` | 0 | Single registry preserved; `resume/modes.txt` |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` on 7877 | **1** | **workspace 23/23; labels 8/9**. Only red is real backend same-key replay; `resume/browser-live-console-final.log` |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-inbox` | 0 | **13/13**, `resume/browser-inbox.log` / `resume/inbox-results.txt` |
| `LC_TEST_LOCK_WAIT=14400 LC_FOCUSED_TIMEOUT=900s bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN05PrintAndMarks$'` | 0 | 1 PASS / 0 FAIL / 0 SKIP; `resume/pg-print.log`; does **not** test key replay |
| `node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` | 0 | 53 selected modes; `resume/pr-modes-delivery.json` |
| scoped Impeccable detector | 0 | `resume/detector.json` = []; not visual acceptance |

Inbox and focused PG ran before the final **test-reporting-only** 7877 commit; their product/harness inputs remain identical (no inbox / Go / SQL change between 26d and 7877). Final full console/labels, node, tsc and check-gates bind directly to 7877. `resume/SOURCE-DELIVERY-SHA256.txt` records exact feature/fixture/registration hashes.

All test-local/PG runs strictly serial with the current heartbeat lock. The last console run queued until another holder finished; no lock removal or other-agent process termination.

## Red → green and backend handoff

- Actual A3 scoped 404: pre-fix label count **1**, expected **0**; `resume/privacy-red.log` / `privacy-red-error.txt`. Filtered workspace had zero matching tests, not PASS. The same unchanged privacy regression passes in both subsequent full runs.
- Same-key UNKNOWN counterexample is deliberately **still red**: real first transaction commits, ACK is dropped, next real click sends identical key/body; count **7 → 8**. Key equality passes. Evidence: `resume/idempotency-final-error.txt`, labels artifact below.
- Root: `internal/httpapi/live_stream.go:53–62` bypasses command receipts; `PrintComment` takes no key; SQL `live.comment_print` increments every call. Contract §7.4 requires per-key idempotency.
- Integrator confirmed **PrintComment bypasses command.Run** and explicitly accepts this UI delivery with only that case recorded as **BLOCKED(backend f4c26fcb)**.
- Backend task **f4c26fcb-ab2d-4a1b-9e29-0908e02406d3** is now claimed by **qwen-aliyun-backend**, branch **unit/lc-a3-print-idempotency**. Its fix is a `command.Run` receipt keyed by `Idempotency-Key`, with Go red→green tests. No backend implementation is performed here.
- Details / required backend replay negatives: `A3-IDEMPOTENCY-BLOCKER.md`. No assertion deleted, relaxed, retried or converted to an expected failure.
- Prior feature/host red→green logs and c012 evidence remain historical under the existing directories, not current all-green proof.

## Historical initial screenshots / click ledger

- Workspace artifact: `output/playwright/live-console-843701277/` (23 passed).
- Labels artifact: `output/playwright/live-console-2056379755/` (8 passed, 1 failed).
- Inbox artifact: `output/playwright/inbox-ui/20261009T142832.855140000/` (13 passed).
- Committed `resume/delivery-evidence/`: **18 screenshots**, 1586×992 / 390×844 × zh-TW/zh-CN/en × preview/A4/small; **9 per-case ledger files, 45 rows (44 PASS / 1 FAIL)**.
- Ledger covers keyword filter, check/uncheck/recheck, preview, A4/small, print, close, single-print, reload, real scoped revocation, failed-record truthfulness/reset and the failed same-key replay.
- Author inspected current zh-TW390/en1586 previews; modal controls and labels visible, desktop three-column alignment intact. Self-QA only; not independent visual acceptance.
- Historical 26d global ledger was overwritten on a worker restart; it is retained under `resume/evidence/` but **not** cited as current coverage. Per-case 7877 artifacts fix that evidence loss.

## CI gates / NOT_RUN / stop

Integrator GitHub gates (standing RAM-heavy rule):
- `bash scripts/dev/test-local.sh --browser-live-console` after the backend fix (must be fully green).
- `bash scripts/dev/test-local.sh --browser-inbox`.
- `bash scripts/dev/test-local.sh --browser-click-sweep` — **NOT_RUN locally**.
- `bash scripts/dev/test-local.sh --browser-visual-lint` — **NOT_RUN locally**.
- Remaining selector-required modes from `resume/pr-modes-delivery.json`; recompute on the PR after backend integration.

New K3/required PR CI, physical printer/OS print dialog/paper pagination, LIVE Meta and production: **NOT_RUN**.
No migrations/GRANT/checkout runtime were changed, so no local full G07 was run. Native `window.print` was intercepted only to count invocation; A3/A2/PG and print CSS stayed real.

Evidence class: **MOCK Graph + REAL_PG browser E3 for tested paths; backend P1 keeps full acceptance BLOCKED**.
The same-key assertions and real ACK-loss operations are retained, with no skip, weakened assertion or expected-failure conversion. The confirmed `c9e211695289fa765dc271daa0bcd7d78db075ce52d844c468077d39f72cef19` is the historical 992cff85 spec hash; K3-requested ledger instrumentation changes the current hash to `c5f1b3f7a685c1fd284cc03416f6b96cdaf509f71f114acb19bf3bf3bd12b156`.
After the backend PR merges, merge `origin/r3/integration` (keep the registry layout) and rerun `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console`. Record the unchanged case turning green before closing this blocker; do not reclassify the existing red log as PASS.
All owned runs/servers/fixtures ended; shared caches untouched. No push/deploy/production/provider mutation. UI delivery committed; stop for integrator K3 review and backend merge.
