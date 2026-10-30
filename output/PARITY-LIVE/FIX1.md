# PARITY-LIVE-FIX1 — directed repair of PR #355

Status: diagnosis recorded before repairs; current-head CI PENDING. Author correctness E0.
Worker: integrator-dispatched Codex UI/test maintainer, no recursive delegation or new claim. Runtime model/effort metadata is not exposed. Worktree `parity-live`, branch `unit/parity-live` only.
Input head `8296581fb4e84058b9c0f35d52af4b0cb4e496a0`; first merged integration `a3dc78e4d5ec72831aaf6a376793f0f6a7a56425` in `a3f0514d6`.
Existing unstaged DELIVERY/red.log and untracked prior relay/Figma material were preserved.

## Authority and classification (before implementation)

Authority: FIX1 owner card; AGENTS.md; 架构.md §§0/1/2/26/27; contracts/live-console-v1.md §§3.3/4.2/4.4; signed UX-LIVE v3 frame IDs 1429:39502/1429:39607 previously read through Fleet Bridge; current LiveConsole/CommentStream markup and original GitHub receipts below. No new design/Figma write.
The supplied run 41065893297 is Foundation. Job 124101780679 belongs to Gates **41065893191**, as confirmed by `gh pr checks 355` and its raw receipt. Both runs tested synthetic merge `4ab11bbbb9a1149f1861be4256ce03952dcee10d` for PR head `8296581fb4e84058b9c0f35d52af4b0cb4e496a0`.

|Failing job / run|Failing test|Original evidence|Class and directed repair|
|---|---|---|---|
|124101780679 / Gates 41065893191, member --browser-live-settings|TestBrowserLiveSettingsUIRealChain; manual reminders template and restriction real clicks, all 3 locales × 2 sizes|Artifact 12985931500, six click ledgers fail at live-settings.spec.ts:170:113 after successful blocklist-remove/confirm. `buyer-panel` is inside closed `[data-live-buyer-details]`.|(a) Signed buyer details intentionally collapsed. Click Buyer tab on mobile and Buyer details disclosure on both sizes; retain panel, note-limit, exact request, restricted persistence and privacy assertions.|
|124101780679 / Gates 41065893191, member --browser-live-settings|five sold-out delivery states real clicks and masked badges, all 3 locales × 2 sizes|Same artifact, six ledgers fail at live-settings.spec.ts:421:202 after comment-select-700_801. 12 failed / 10 passed in original playwright.log.|(a) Same signed disclosure. Use shared real-click helper; retain all 5 exact state strings, sent-only badge, row selection and reload assertions. Reopen Comments before selecting each next mobile row.|
|124101780679 / Gates 41065893191, member --browser-ui-gallery|tests/ui/ds-hex-ratchet.test.mjs:87, live tree holds baseline|Artifact 12986235964 and raw bundle01 log: `total hex colours 410 > baseline 386`; `live-console.css: 24 hex colours > baseline 0`.|(b) Product regression. Replace new raw colors with existing semantic tokens; baseline/scanner unchanged.|
|124097075104 / Foundation 41065893297|Same ds-hex-ratchet test|Raw Foundation log 3704–3730: exact same 24-color violation.|(b) Same product root; one CSS fix, no separate exemption.|
|124101780824 / Gates 41065893191, --browser-live-console|LC-U1 MOCK contract real clicks; five completed error contexts (en desktop/mobile, zh-TW desktop/mobile, zh-CN desktop), sixth interrupted by outer timeout|Artifact 12986652259: `locator('[data-live-tools]:not([open]) > summary').nth(1)` waits until 110000ms; outer TestBrowserLiveConsoleRealChain/workspace timeout 13m.|(b) Prior adaptation helper regression: `.all()` creates index-based locators into a shrinking closed-disclosure set. Use stable disclosure identities, open by real clicks once; re-enter Live/tools after reload for stock/lifecycle checks. Keep assertions, fixture, cadence, timeout and budgets.|
|124138761509 / Gates 41065893191, receipts|Non-PASS member receipts|Raw receipt log: --browser-live-console, --browser-live-settings, --browser-ui-gallery FAIL at tested merge SHA.|Dependent failure, not standalone missing browser-spec evidence. No receipt code changes.|
|124139237097 / Gates 41065893191, required|Required dependency verdict|Raw log: `plan=success build=success ... gate=failure aggregate=success ... receipts=failure`.|Dependent failure. Aggregate itself succeeded. No aggregate/required changes.|

Base Gates 41105181453 on a3dc78e4d was incomplete at diagnosis; no class (c), base GREEN or identical-base exemption is claimed. No assertion/fixture/baseline/timeout/exemption changes.
This is targeted attempt 1 for these distinct roots in FIX1. If the same assertion remains red on the next CI, report BLOCKED to integrator; do not try a third approach.

## Verification and handoff

Existing causal signed-frame RED: tests-only head 898e71a829d7f4bce261eebc95ec5028693f8901; Gates 41050616128/job124051353169, exact heading absent and mobile tab absent (original CRC-verified diagnostics in red.log). The failures above are the CI RED for these directed repairs; FIX1 requires one batched fix push.
Only permitted static checks and required ownership/depmap generators run locally. All local Node/Go/browser/PG tests and servers: NOT_RUN. GREEN, full parity/density/click/persistence acceptance, independent review, SANDBOX/LIVE/production: PENDING or NOT_RUN. No local runtime PASS claimed.

Outside original implementation paths: tests/admin/live-settings.spec.ts and shared tests/admin/fixtures/live-console-controls.ts (direct same-module consumers); generated browser ownership and dependency map only if changed; output/PARITY-LIVE/FIX1.md and red.log (card-required receipts). List in PR body for integrator review.
Rollback: revert the single FIX1 implementation commit, retain RED evidence; no production action or merge/auto-merge authorized.

Static receipts (E1, exit 0): `node --check` for live-console.spec.ts, live-settings.spec.ts, fixtures/live-console-controls.ts; `node scripts/dev/derive-browser-ownership.mjs` and `--check`; `git diff --check`. Generator includes the shared helper in both browser modes, with no new unresolved import. No component imports changed, so no component compilation allowlist changes required. No Go dependency change; depmap unchanged.
