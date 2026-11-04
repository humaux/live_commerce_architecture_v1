# PARITY-LIVE-FIX1 — directed repair of PR #355

Status: FIX1 relay directed repair 2 prepared; replacement own-head CI PENDING. Author correctness E0.
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


## FIX1 post-push relay receipt (no evidence commit)

Head f51a0c2db2a5262e1083150d95ed0f73d014dae0. WAIT_RUN Foundation41106774257/job124238545940; Gates41106774405. Both pending; integrator relays a new session on completion. GREEN/E3/E4 and independent acceptance are not established.
Correction: final `git diff HEAD^ HEAD --check` exits 2: output/PARITY-LIVE/fix1-evidence/assertions.log:150 new blank line at EOF. Pre-artifact whitespace check had exited 0; final whitespace PASS must not be claimed. Source syntax and generated ownership checks exited 0. No second push per FIX1 single-push constraint.

## FIX1 RELAY 1 — desktop hidden-tab locator root (2026-10-31)

Authority: current owner relay/card; raw completed job 124240810483 of Gates 41106774405 at head f51a0c2db2a5262e1083150d95ed0f73d014dae0, synthetic merge 0e2f4f452cb7166d53dbbf28d95d7a948ee78b84. `gh run view --job --log-failed` refused while the parent run was unfinished; completed-job `/actions/jobs/124240810483/logs` supplied the raw receipt. Artifact 13001713943 was downloaded successfully. Integration 629f42152f864621a99ac2247c3baf5d751a422f was merged before this repair; existing task evidence preserved. Merge commit includes the required co-author trailer.

|Failing job / run|Failing test|Evidence|Class / repair|
|---|---|---|---|
|124240810483 / 41106774405, --browser-live-settings|manual reminders template and restriction real clicks; en/zh-TW/zh-CN at 1440|All 3 original error-context.md files: `expect(locator).toBeAttached()`; `getByTestId('live-console').getByRole('tab').first()` not found, 10000ms. Preserved byte-for-byte in fix1-relay1/*.md.gz; original hashes and test names in assertions.json.|(b) Shared setup helper regression introduced in FIX1 attempt 1. `getByRole` excludes CSS-hidden desktop mobile tabs. Include hidden roles for the existing attachment assertion; retain isVisible guard before real clicks and selected-state assertion.|
|124240810483 / 41106774405, --browser-live-settings|five sold-out delivery states real clicks and masked badges; en/zh-TW/zh-CN at 1440|Same 3 exact attachment failures in independent contexts; run summary 6 failed / 16 passed. No mobile failures.|(b) Same root, one shared-helper fix. All five state assertions, sent-only badge, privacy, requests, persistence, and real-click paths remain unchanged.|

Current source confirms LiveConsole.tsx always renders the three mobile tabs; components/studio/live-console.css hides them on desktop (`display:none`) and displays them at the mobile breakpoint. No product redesign or additional UI strings. Both consumers (live-console.spec.ts and live-settings.spec.ts) use the repaired helper; its other methods keep real summary clicks.

Resolved historical job members, not current acceptance: latest ui-gallery member succeeded with 6 matrix cases / 1267 ledger rows; Foundation 41106774257/job124238545940 succeeded on f51a0c2d. Gates overall is still pending with a failed bundle. Base 41105181453 remains queued at lookup, so no identical-base class (c) or base GREEN claim. Receipts/required are not diagnosed as standalone failures while unfinished.

Attempt bound: this is the second directed shared-helper repair. If this same test/assertion remains red, STOP with BLOCKED and exact evidence; do not attempt a third approach.

Scope outside original card paths: tests/admin/fixtures/live-console-controls.ts, generated output/ci-select-backend-browser/browser-ownership.json, output/PARITY-LIVE/{FIX1.md,DELIVERY.md,red.log,fix1-relay1/} evidence. Existing full artifact extraction stays untracked; only failure contexts, hashes and filtered receipt are committed. No new component imports, no source headers removed, no Go dependency changes (depmap unchanged). No gates, exemptions, fixtures, thresholds or timeouts changed. `tests/ci/next-startup.test.mjs` untouched.

Local static commands: node --check tests/admin/fixtures/live-console-controls.ts exit 0; browser ownership generation exit 0; browser ownership --check exit 0. Replacement CI, actual clicks/persistence/parity/density and independent review PENDING; no local tests or servers. Humaux search/store/canvas/coord and code-index tools unavailable (NOT_RUN). Integrator supplies fresh-session wakeup on registered own-head CI runs; no local watcher.
