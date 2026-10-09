<!-- Purpose: current W3-U3 scoped UI handoff, exact gate evidence and the retained A3 backend blocker.
Depends on: frozen brief, live-console-v1 section 7.4, PR18 trunk and approved test-only extensions.
Used by: integrator K3/PR preparation and backend replay-receipt owner; no all-green claim. -->
# W3-U3 comment label print — DELIVERY

**Status: BLOCKED_BACKEND (A3 same-key replay). UI changes committed; do not call this unit all green.**

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

## Current results and exit codes

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
- Owner explicitly ruled **backend unit fixes this, UI scope retained**. Backend task **f4c26fcb-ab2d-4a1b-9e29-0908e02406d3** submitted for assignment, not claimed/implemented here.
- Details / required backend replay negatives: `A3-IDEMPOTENCY-BLOCKER.md`. No assertion deleted, relaxed, retried or converted to an expected failure.
- Prior feature/host red→green logs and c012 evidence remain historical under the existing directories, not current all-green proof.

## Current screenshots / click ledger

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

Evidence class: **MOCK Graph + REAL_PG browser E3 for tested paths; backend P1 keeps whole unit BLOCKED**.
All owned runs/servers/fixtures ended; shared caches untouched. No push/deploy/production/provider mutation. Commit and stop for integrator review; backend handoff remains open.
