# W3-U2 live settings checkpoint — PAUSED by owner priority

- Branch: `unit/w3-u2-live-settings-ui`; tested product commit: `115bd20251213e8be8fd20e149ff89f2a37754d8`. The final delivery-only commit is its descendant; `SOURCE.json` pins every changed source file.
- Dependency: `89499013` initially; privacy follow-up `origin/unit/lc-u2a-comment-stream` `5f888bca864e4b6d54ea296ea60264b51accacfb` merged as `7dd45b1fd969f3f6394e4bc263f0a28f9470b91c`.
- Task: `662ad08e-b312-4b1f-bbf8-f2ffdb25d14d`; role: Codex-4 UI/BFF integrator with explicitly authorized thin Go adapter. Parent model runtime name not exposed; no unsupported model claim. Backend/browser collaborators used isolated worktrees, commerce_worker/test_worker, gpt-6.1-sol high. Independent review used a read-only agent.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/w3-u2-live-settings-ui`.

## Result and scope

- `apps/admin/components/LiveSettings.tsx:20`: manual trigger/report/copy, merchant template publication then sold-out CAS save, blocklist pagination and confirmed removal. Three locales: zh-TW, zh-CN, en. Required 24-hour and one-private-reply copy retained. Restricted follow-ups have no copy action; blocklist shows platform/note/source bundle/time, never an invented name.
- `apps/admin/lib/live-settings-controller.ts:57`: visible-scope fences, immutable UNKNOWN receipt retry, bounded reads and explicit refresh. Hidden/private state never goes into storage, URLs or logs.
- `apps/admin/components/BuyerPanel.tsx:310`: restriction check/add and badge. First nested 404 revokes the parent console; a held A8 completion cannot repaint. `CommentStream.tsx:266` adds the sold-out badge using the actual `out_of_stock` mark.
- Dedicated BFF leaves validate real signed-session scope, CSRF, methods, bodies, queries and response receipts. `apps/admin/proxy.ts:80` admits their exact grammar before the older Studio guard; encoded paths and unknown resources remain refused.
- `internal/httpapi/sold_out_settings.go:29`: registered GET/PUT over existing definers. live:read/live:manage, merchant-published template ID/version, expected_version CAS, transactional idempotency receipt; no migration/ACL/dependency changes.
- Automatic reminders, delay input, blocklist display names and order-drawer warning are excluded per `RULINGS.md`. No placeholder automatic control. Existing saved merchant template can be toggled; a new draft is published before selection. Fixed templates remain immutable and are rejected for PUT.

## Commits and interface

- `123b9ff0ea29ee70aefede37581e141e55c7cdd8`: adapter/tests.
- **Separate OpenAPI commit** `20d8b0c77eb175aced89f003e4d96563b0b8febf`: `contracts/live-settings-openapi.json`; integrator owns final shared-contract merge.
- `8a2493d1`, `53b924bf`: browser gate plus strict merchant-template fixture.
- `25a16b99`: UI/BFF/mode registration; `bf96e1f4`: raw-path proxy integration; `9c00233e`: nested privacy and UNKNOWN admission fixes.
- Additive GET/PUT `/v1/admin/stores/{store_id}/live-settings/sold-out-reply`. Exact PUT `{enabled,template_id,template_version,expected_version}` plus Idempotency-Key. GET/PUT return `{enabled,template_id,template_version,version}`. Existing parser top-level null422 remains; null fields400. No existing wire was relaxed.

## Red → green and exact commands

All logs below are uncompressed. Commands run from this worktree unless the child delivery explicitly states its isolated worktree.

| Command / stage | Exit / result | Evidence |
| --- | --- | --- |
| `node --test --experimental-strip-types tests/admin/live-settings-bff.test.ts` before new leaves | 1 → 0 | `bff-red.log`, `bff-green.log` |
| Same real-handler test, mismatched successful receipt | 1 → 0 | `receipt-red.log`, `receipt-green.log` |
| Same test, actual Next proxy admission | 1 (404 instead of200) → 0 | `proxy-red.log`; final full Node contains green |
| `node --test --experimental-transform-types tests/admin/live-settings-bff.test.ts tests/admin/live-console-bff.test.ts` | 0,18 passed | `proxy-green-transform.log` |
| `node --test --experimental-transform-types --test-name-pattern='W3-U2 restriction' tests/admin/comment-stream-hooks.test.ts` before nested privacy fix | 1, first404 left console live | `restriction-404-red.log` |
| `node --test --experimental-transform-types tests/admin/live-settings-hooks.test.ts` before admission fix | 1, fresh key admitted during UNKNOWN | `unknown-red.log` |
| `node --test --experimental-transform-types tests/admin/live-settings-hooks.test.ts tests/admin/comment-stream-hooks.test.ts` | 0,20 passed | `review-green.log` |
| `bash scripts/dev/test-focused.sh 'TestLiveSettingsHTTP' ./tests/foundation ./internal/httpapi` before adapter | 1,2 top-level failed; 403/404/409 cases red | `http/red-all.log` |
| Same focused REAL_PG command after integration | 0,2 top-level +24 subcases;0 skipped | `real-pg-final.log` |
| `bash scripts/dev/test-local.sh --browser-live-settings` on missing-page base | 1 | `browser/red.log`, child source receipt |
| Same browser command at25a16b99 | 1,11 failed/1 passed; proxy404 | `browser-green.log` (historical failed attempt despite filename) |
| `LC_BROWSER_SETTINGS_GREP='manual reminders template and restriction real clicks zh-TW-1440' bash scripts/dev/test-local.sh --browser-live-settings` | 1, diagnostic-only first failure location/status | `browser-diagnostic.log` |
| `bash scripts/dev/test-local.sh --browser-live-settings` at115bd202 | **0,12/12 passed** | `browser-accepted.log`, `browser-accepted-final/source.json`, click ledgers |
| `bash scripts/dev/test-node.sh` at115bd202 | **0,1196 passed/0 failed/0 skipped** | `node-accepted.log` |
| `pnpm typecheck:admin` | **0** | `typecheck-accepted.log` |
| `bash scripts/dev/check-gates.sh` | **0,83 modes documented; all tracked tests run; headers green** | `gates-accepted.log` |
| `git diff --check origin/unit/lc-u2a-comment-stream..HEAD` | 0 | tool receipt; repeated before final commit |

The mixed proxy/console test was initially invoked with strip-types; console's existing parameter properties require transform-types (exit1 diagnostic in `proxy-green.log`). Correct command above passes; no assertion changed. Child backend worktree lacked Node dependencies and its check-gates failed; the prepared parent final gate passes. Those old failures are retained as history, not current blockers.

REAL_PG ran at bf96e1f4; the later9c00233e only changes UI/Node tests. All Go/API source hashes match the tested adapter (child `http/hashes.txt` and final `SOURCE.json`). Browser/Node/typecheck/gates ran on115bd202. Browser uses production Next and real BFF/PG signed authority, with explicitly MOCK business endpoints; separate REAL_PG tests exercise the actual adapter and persistence.

## Evidence and review boundary

- E3 within the tested environment: automated red→green above. No LIVE/provider or production claim.
- `browser-accepted/` contains sanitized source/receipt/click ledgers and six masked screenshots. Desktop/mobile zh-TW/en screenshots visually inspected: controls fit, no horizontal overflow; automated three-locale 1440/390 overflow assertions pass. Private rows/drafts/panel are deliberately masked.
- Independent read-only review found nested404 and UNKNOWN admission defects; both reproduced then fixed. Third finding: stale callbacks after hide/departure/unmount dispatched despite revoked fence. Actual controller RED1!=0 then fence.current(ticket) before refs/state/transport; focused24 tests green. Source115bd202. `departure-red.log` and `departure-green.log` retain evidence. Final independent recheck pending at priority pause; integrator K3 remains the acceptance authority.
- Current READINESS blockers are resolved; that file is explicitly historical. PAUSED by owner for PR20 money-path round1. New registry integration is pending; this is not a final handoff.

## CI gates / NOT_RUN

- Integrator: K3 pre-review, then push. No author push, deployment or live message.
- Required feature CI: foundation-shards (including adapter and Node), `--browser-live-settings`.
- Broader regressions from the original brief: `--browser-live-claims`, `--browser-admin-shell`, `--browser-click-sweep`; also shared BuyerPanel/template-list `--browser-live-console` and `--browser-inbox`. **NOT_RUN locally in this unit**; run in CI under the owner RAM policy. Local new-mode acceptance was explicitly authorized by RULINGS.
- Full foundation, provider SANDBOX/LIVE and production deployment: NOT_RUN. Automatic reminders and order drawer remain integrator follow-ups.
- Harness-owned PG/Next/browser processes exited through cleanup; no owned background runner remains. Other agents' processes and shared caches were untouched.

## Owner priority pause — 2026-10-09
Current product115bd202 completed its in-flight gates: browser12/12, Node1196/0/0, admin tsc0, check-gates0. No running owned process remains. Pending before W3-U2 resumes: merge origin/unit/ci-pr-modes-coverage (PR22) BEFORE touching scripts/dev/test-local.sh; replace the old mode wiring with ONE lc_prepare/lc_run/lc_build/lc_fixture registry block, then re-merge origin/r3/integration after PR22 lands. Do not edit old if/elif layout. Re-run impacted selector/gates/browser checks after the merge. PR20 money-path packet pr20-66f99761 takes priority. No push.

Evidence packaging: full cached `git diff --check` exits2 only for raw test-log whitespace/CR progress lines; raw logs intentionally remain exact. Source-only diff check exits0.
