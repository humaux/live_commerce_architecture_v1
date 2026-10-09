# W3-U2 PR #31 review batch delivery

- Branch/worktree: `unit/w3-u2-live-settings-ui`, `.worktrees/w3-u2-live-settings-ui`; base `539f1cb8` includes integrator changes, both suites, registry and #23 evidence paths. Frozen tested source: **`0ff7ca92fea1e31607c16099ea5429b6dd882a7b`**. Final commit packages evidence only; `pr31-r1/extra/SOURCE.json` and browser manifest bind all current runtime/test files.
- Author Codex-4 (parent model/effort unexposed); isolated UI/test workers GPT-6.1-sol/high or medium and read-only source/security review. Task `e0cab99c-5ed3-4d22-a3cd-88700fb57c71`. No push; CI priority #29/#33, then integrator pushes this batch.
- Original K3 PASS at `b06df828` and old `trunk-resume/`, `browser-before-extra/`, `pr31-r1/extra/before-extra/` results are historical, not current-head independent review.

## Seven review items

1. Reminder POST retains the safe aggregate receipt. No report GET without inbox:read; real permission loss still clears private state.
2. Same-address navigation preserves privacy lifetime, including resolved default-store URLs. Other scene/store/locale departure clears synchronously and late reads cannot repaint.
3. Scene selector consumes next_cursor and uses authoritative detail/title for a deep-linked scene beyond page 20. Real fixture has 21 scenes.
4. Shared templates accept 120 Unicode runes and 501/5001 legal rows under the unchanged 1MiB response bound. 121 runes, malformed rows and oversized bodies remain refused through actual Request/route tests.
5. Sold-out sent badge requires commentSendState(sent), verified for success/queued/failed/blocked/UNKNOWN in zh-TW/zh-CN/en at 1440/390. No packages/i18n change.
6. Optional reminder 404 is local only after exact Go plain404 plus fresh signed store/method authority. Store not_found and 401/403 clear private data; upstream JSON cannot forge the internal absence marker. Healthy sold-out/list controls and known POST receipts remain.
7. Selected-scene placeholder is disabled. Explicit useId/htmlFor association retains its name/native option state and the same grid layout; browser assertions/waits unchanged.

Captured run/save/copy/page/more callbacks retain their render's privacy generation. Hide/reveal cannot lend new authority to discarded work; visible focus preserves the fence and UNKNOWN exact-key retry. Actual six negative counterexamples were RED, then GREEN. Uncompressed logs and corrected Node24 navigator fixture diagnostic remain under pr31-r1.

## Additional actual CI failures

**OP4:** exact toEqual navigation list and both live-group inventories include nav-live-settings after nav-claims in all three locales. Counts 8/11 remain; no contains-style weakening. Actual consumer AST went RED, then 6/6 GREEN. Full ops mode passes twice.

**LCU2_404:** the newly mounted restriction check can observe grant loss before A2/A8, then expire the containing console and stop those polls. The actual emitted GET drives the original browser waiter in Node. Its added arm uses the exact current store/session check path, GET and 404; all four original arms and every later clear/cookie/login/zero-read assertion, 15s waiter and 6.5s observation budget remain.

Backend bundle missing/purged also yields PT404. The BFF separately proves store_scope_lost from fresh signed authority; upstream cannot forge it. Ordinary missing bundle clears the selected buyer and local fence without revoking the console. True store loss clears everything immediately. A queued A2 timer before passive cleanup genuinely issued one extra read RED; the effect-generation/blocked admission guard prevents it and borrowing a new fence.

Both full console repetitions pass the two grant-revocation cases. Current real response ledgers show **buyer-panel** and **restriction check** as first 404 observers. Canonical records are `pr31-r1/extra/console-{1,2}-observers.json` and `browser-current/console-{1,2}/scope-reads-*.json`. They contain only pathname/category/method/status, without query, header, body, buyer name or message text.

## Interface and evidence

Review-batch production changes are TS/BFF only. New fixed internal BFF codes reminders_unavailable/store_scope_lost distinguish positively proven conditions; Go routes, permissions, OpenAPI, SQL, migrations and lockfiles are unchanged. The existing authorized thin sold-out GET/PUT adapter remains CAS/idempotent; the final REAL_PG handler tests prove scope, replay and refusal behavior.

Current canonical browser evidence is **committed** under `pr31-r1/extra/browser-current/`: both console ledgers/logs, both ops admin/storefront results, 22 live-settings per-test ledgers plus native aggregate and 12 synthetic screenshots, and live-claims result/ledger/log/screens. Comment-author/body screenshot masks remain. Screens use synthetic fixtures; no real buyer PII is asserted. Hash manifests and `browser-evidence.json` bind each file and frozen source. The unchanged KC16 harness allocates its own fresh ignored output/playwright/live-claims-* directory; its exact actual path is recorded, not assumed from the caller root.

## Commands / exit codes

Full exact argv/environment, exits and log hashes: `pr31-r1/extra/{final-local-gates,final-http-real-pg,browser-runs}.json`. Browser commands below use `LC_TEST_LOCK_WAIT=1800 LC_BROWSER_ENGINE=chromium` and a unique `LC_BROWSER_EVIDENCE_ROOT` under output/playwright; exact values are in each run record. Queue waiting changes no product/test timeout.

| Command | Exit | Result/evidence |
| --- | ---: | --- |
| `bash scripts/dev/test-node.sh` | 0 | **1401 PASS / 0 FAIL**; `pr31-r1/extra/final-node.log` |
| `pnpm typecheck:admin` | 0 | `pr31-r1/extra/final-typecheck.log` |
| `bash scripts/dev/test-local.sh --list` | 0 | `pr31-r1/extra/final-registry-list.log` |
| `bash scripts/dev/check-gates.sh` | 0 | **83 modes all documented**, every tracked test run and header ratchet green; `pr31-r1/extra/final-check-gates.log` |
| `LC_TEST_LOCK_WAIT=1800 bash scripts/dev/test-focused.sh 'TestLiveSettingsHTTP' ./tests/foundation ./internal/httpapi` | 0 | **2 top-level / 24 subcases / 0 skips**, REAL_PG scope/CAS/replay and full router; `pr31-r1/extra/final-http-real-pg.log` |
| `bash scripts/dev/test-local.sh --browser-live-console` #1 | 0 | `pr31-r1/extra/browser-live-console-1.log` |
| `bash scripts/dev/test-local.sh --browser-live-console` #2 | 0 | `pr31-r1/extra/browser-live-console-2.log` |
| `bash scripts/dev/test-local.sh --browser-ops-polish` #1 | 0 | `pr31-r1/extra/browser-ops-polish-1.log` |
| `bash scripts/dev/test-local.sh --browser-ops-polish` #2 | 0 | `pr31-r1/extra/browser-ops-polish-2.log` |
| `bash scripts/dev/test-local.sh --browser-live-settings` #1 | 0 | `pr31-r1/extra/browser-live-settings-1.log` |
| `bash scripts/dev/test-local.sh --browser-live-claims` #1 | 0 | `pr31-r1/extra/browser-live-claims-1.log` |
| `node scripts/dev/pr-modes.mjs origin/r3/integration` | 0 | **54 modes** selected; `pr31-r1/extra/pr-modes.json` |

Focused parent current Node 132/132 and BFF 38/38 pass; worker cards retain their actual RED commands and raw logs. Current live-settings is **22/22**, KC16 result **21 cases / 3 locales**; both console and ops full repetitions exit 0. The original live-settings 5b run was 20/22: a wrapping label redirected Playwright's option state check to the enabled select. Explicit label association fixed it, with a new actual view RED→GREEN counterexample and no browser assertion change. Earlier claims attempt exit 2 was **NOT_RUN** due another owner's PG lock, not a browser failure. Both records remain preserved.

## Shared GET 403 race follow-up

The reviewed local runs observed first scoped **404** responses; they do not establish the cause of CI run `37956489889`, job `113918794414` at `539f1cb8`. Its two LCU2_404 cases time out after 15s, but the retained artifact records only one initial A2 200 per case and lacks network/HAR/trace data for the other scoped responses. Their first 403/404 status is **UNKNOWN** from this evidence.

Current GET 403 handling is still direct; the unit-specific 404 observer/purged-bundle fix does **not** normalize an admitted request that becomes forbidden in Go. The shared candidate `58510545` proves store disappearance with the same token before mapping GET 403 to 404. Per the integrator ruling, this unit does not duplicate that implementation and awaits its reviewed trunk integration. That commit changes catchall inbox handling; the dedicated live-settings-proxy used by blocklist/check is outside its scope, so its coverage must be confirmed during shared integration.

Source/artifact check and sanitized CI references: `pr31-r1/extra/scope403-check/REPORT.json`. No product source or browser assertion changed in this follow-up. Earlier E3 local passes remain valid for their tested cases; they are not evidence that the shared 403 race is fixed or that the PR is merge-ready without the shared dependency.

## Evidence class / remaining gates

**E3: complete in the tested BROWSER + MOCK business + REAL_PG signed-authority/adapter environment**, bound to the current frozen source and file hashes. Read-only audits are E1; current-head independent K3/CI remains pending with the integrator. No new K3 PASS or production/provider acceptance claim.

CI gates needed: foundation-shards and the planner's 54-mode set. Four relevant modes were run here (console/ops twice, settings/claims once); other 50 modes, full foundation, provider SANDBOX/LIVE and deployment are NOT_RUN locally. The integrator prioritizes #29/#33, then pushes this batch. Local gates passed; shared 403-race integration remains an external dependency.

Owned browser/PG/Next runners finished, mutex released; other agents' processes, fixtures and shared caches untouched. RULINGS supersede old READINESS questions. Commit only, stop; no push.
