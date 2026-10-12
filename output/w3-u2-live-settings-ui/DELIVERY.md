# W3-U2 PR #31 round-2 thread delivery

- Branch/worktree: `unit/w3-u2-live-settings-ui`, `.worktrees/w3-u2-live-settings-ui`. Unit pull was a no-op at **20b7ed27e50d142f771f65d821c11bc589d1e0ae**, including integrator trunk union, fdb7176a `lc_covers`, and sold-out PT403 mapping. None was reverted.
- New tested source commit: **d05ab56e1ce8124786a74d4bb80696c32967fd66**. Only `apps/admin/lib/live-settings-controller.ts` and `tests/admin/live-settings-hooks.test.ts` changed; the final artifact commit adds current evidence/docs.
- Author Codex-4, actual parent model/effort unexposed; read-only explorer requested GPT-6.1-sol/medium owns no files. Task `483fc6b2-e065-4e74-9b02-d6625cba249b`. No push, review reply or thread resolution; the integrator performs those actions.

## Nine-thread disposition

All eight round-one items already have fixes in this branch; each listed commit was checked as a current HEAD ancestor and its source/tests were inspected. No FOLLOWUPS are proposed for those existing fixes. The new privacy thread is fixed in this batch.

| Thread | Fix SHA | One-line reply |
| --- | --- | --- |
| 4231878479 | `3e5a9c4a` | Fixed in 3e5a9c4a: successful reminder receipts stay visible and report GETs require inbox:read; the regression asserts one POST and zero unauthorized GETs. |
| 4231878495 | `3e5a9c4a` + `c9608348` | Fixed in 3e5a9c4a and c9608348: same-address navigation preserves the privacy lifecycle, including an implicit default store; real scene/store/locale departures still clear synchronously. |
| 4231878505 | `3e5a9c4a` | Fixed in 3e5a9c4a: the picker consumes next_cursor and merges paged choices, while deep links load the selected scene's authoritative detail. |
| 4232189503 | `3e5a9c4a` | Covered by 3e5a9c4a: page-two scenes are selectable and deep links display the selected scene's authoritative title. |
| 4231878513 | `afe0317a` | Fixed in afe0317a: GET/POST preserve the 120-Unicode-rune name contract and legal lists beyond 500 rows under the existing 1 MiB bound; 121 runes remain rejected. |
| 4231878523 | `ac43103c` | Fixed in ac43103c: the sold-out sent badge requires confirmed delivery; rendered-component and browser matrices cover pending, failed, blocked and UNKNOWN states without a sent badge. |
| 4232031441 | `3e5a9c4a` + `c9608348`, `dea6b6aa`, `5b1da81a` | Fixed in 3e5a9c4a and hardened by c9608348/dea6b6aa/5b1da81a: proven unmounted reminder routes stay panel-local; actual permission or scope loss still clears private data. |
| 4232031446 | `3e5a9c4a` | Fixed in 3e5a9c4a: the empty placeholder is disabled after a scene is selected, including first-page and deep-linked scenes. |
| 4246128710 | `d05ab56e` | Fixed in d05ab56e: Studio not-found now uses the shared scoped-404 fence/clear path; a late private read cannot repaint and an old callback cannot POST, while a 503 retains authorized state. |

Machine-readable full SHAs, supporting commits and ancestor exit codes: [THREADS.json](round2-threads/THREADS.json). Copy-ready standalone replies: [THREADS.md](round2-threads/THREADS.md). Old-thread static audit `95fbade6` is E1; it does not claim a new independent browser run.

## New privacy fix / red → green

Studio's real client maps a scoped HTTP 404 to `StudioError("not-found")`. The controller's shared `fail()` previously cleared only `LiveSettingsError` 404, leaving the already loaded page visible after a refused Studio next-page read. It now routes both definite scope-not-found representations through the existing **fence invalidate(true) + clear()** path. Authentication/permission 401/403 still expire; the BFF-proven `reminders_unavailable` exception remains reminder-panel-local. There is no second Go/BFF SCOPE-404 implementation, route, contract, migration, dependency or registry change.

The new real-controller causal test first waits for authoritative loaded sessions, settings, report and a synthetic private blocklist row, then sets a draft/removal selection and holds a blocklist-page response. A real Studio pagination HTTP 404 must clear the private fields, abort the old fence, refuse the old POST callback, and prevent the later accepted response from repainting. The 503 control retains authorized state and permits the valid delayed read. It uses exact request/response promises and committed-state predicates, with no fixed tick count or shadow controller.

- RED: `node --test '--test-name-pattern=PR31 Studio pagination' tests/admin/live-settings-hooks.test.ts` → **exit 1** on unchanged controller; the 404 case retained its old sessions, while the 503 control passed. [red log](round2-threads/red-studio-pagination.log), [command/hash record](round2-threads/red-studio-pagination.json).
- GREEN: `node --test tests/admin/live-settings-hooks.test.ts` → **exit 0, 27 PASS / 0 FAIL / 0 SKIP**. [green log](round2-threads/green-hooks.log), [record](round2-threads/green-hooks.json).
- Independent focused rerun also returned 27/0/0 with unchanged source hashes (`a44b8769`). This is focused MOCK evidence, not new K3/CI approval.

## Gates on frozen d05ab56e

Exact argv/environment, UTC timestamps, bounded timeouts, PIDs, exits and actual log hashes are in [local-gates.json](round2-threads/local-gates.json), [browser-run.json](round2-threads/browser-run.json) and [focused-http-real-pg.json](round2-threads/focused-http-real-pg.json).

| Command | Exit | Evidence |
| --- | ---: | --- |
| `pnpm typecheck:admin` (runs `tsc --noEmit`) | 0 | `round2-threads/typecheck.log` |
| `bash scripts/dev/test-node.sh` | 0 | `round2-threads/test-node.log`; includes prior privacy, receipt, picker, badge and BFF regressions |
| `bash scripts/dev/check-gates.sh` | 0 | `round2-threads/check-gates.log`; current registry/test/header/static ratchets preserved |
| `LC_TEST_LOCK_WAIT=1800 LC_BROWSER_ENGINE=chromium LC_BROWSER_EVIDENCE_ROOT="$PWD/output/playwright/pr31-round2-threads-20261012" bash scripts/dev/test-local.sh --browser-live-settings` | 0 | `round2-threads/browser-live-settings.log`; **22 native tests passed, zero skips**, original browser spec/waits/assertions unchanged |
| `LC_TEST_LOCK_WAIT=1800 bash scripts/dev/test-focused.sh 'TestLiveSettingsHTTP' ./tests/foundation ./internal/httpapi` | 0 | `round2-threads/focused-http-real-pg.log`; existing REAL_PG adapter/router scope, CAS/replay and refusal gate |

The new Studio404 negative path is verified by the actual-controller Node tests. The full browser mode executes its existing 22 cases through production Next/BFF with MOCK business responses and REAL_PG signed authorization; no new browser404 case is claimed.

## Canonical evidence / risks / CI

[SOURCE.json](round2-threads/SOURCE.json) binds 62 current source files; source hashes stayed unchanged during every run. [browser-evidence.json](round2-threads/browser-evidence.json) binds native 22 per-test click ledgers plus the native aggregate, result metadata/log and 12 synthetic masked screenshots at zh-TW/zh-CN/en ×1440/390. No runnable generated spec snapshots, traces, videos, real buyer PII or credentials are copied. [MANIFEST.json](round2-threads/MANIFEST.json) binds delivered artifacts by SHA256.

Prior `pr31-r1/` and `pr36-integration/` evidence remains immutable historical proof for its recorded sources, including six native post-PR36 runs at7737cb87. It is not relabelled as d05 source acceptance. The old reconstructed STATE summary's pending runner flags were stale; actual current source/run records and the task canvas determine this handoff.

**E3: complete in the tested automated environment**, with focused MOCK causal privacy evidence, BROWSER MOCK business + REAL_PG authorization and a separate REAL_PG adapter gate. Current-head K3/remote CI, full foundation, other browser modes, provider SANDBOX/LIVE and deployment are **NOT_RUN** here. CI required: foundation-shards and the current PR planner's required modes, including live-settings. No approved live/provider action is implied.

All own runners/fixtures completed; shared browser lock released. Foreign `output/ext-agents/`, other worktrees/processes and shared caches remain untouched and unstaged. Commit only, stop; integrator pushes and resolves the nine threads.
