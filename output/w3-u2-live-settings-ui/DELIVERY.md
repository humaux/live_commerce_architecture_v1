# W3-U2 live settings delivery

- Branch/worktree: `unit/w3-u2-live-settings-ui`, `.worktrees/w3-u2-live-settings-ui`; author Codex-4 (parent model/effort not exposed).
- Tested source: **`9fbe930193e1fd194e63e395155f78abe37e1e80`**. Base/current trunk: `4ff99766e1a4b9f7b7f6931444f8f71dacc7ea09`. Final commit packages evidence only; `trunk-resume/SOURCE.json` binds changed source hashes.
- Coordination task: `18975431-ecb4-4188-9ad1-04f8054b3fa8`. K3 PASS at original `b06df828` is historical; the integrator reviews this updated batch. No push.

## Result and interface

`LiveSettings.tsx` provides manual reminder trigger/report/copy, merchant template publication followed by sold-out CAS save, and blocklist pagination/confirmed removal. zh-TW/zh-CN/en preserve required 24-hour and one-private-reply copy. Restricted follow-up rows have no copy action; blocklist uses platform/note/source bundle/time, with no invented display name. BuyerPanel restriction add/check/badge and CommentStream sold-out marks use frozen fields.

BFF leaves use real signed-session/Origin/CSRF/store helpers and closed method/body/query/response grammars. The Next proxy admits these routes without removing trunk's host/comment/Studio guards. The authorized thin Go GET/PUT `/v1/admin/stores/{store_id}/live-settings/sold-out-reply` adapter uses live:read/live:manage, published merchant template ID/version, CAS and transactional idempotency. PUT is exactly `{enabled,template_id,template_version,expected_version}` plus Idempotency-Key; response adds current `version`. Fixed templates remain immutable. Existing null/field validation semantics remain.

Private state clears on hide/departure/unmount; stale callbacks cannot dispatch or repaint. UNKNOWN admits only the retained exact receipt. Restriction-check 404 revokes the containing console; ordinary A13 BuyerPanel 404 remains local as trunk requires. RULINGS.md authorizes the adapter and browser registration; automatic reminders/delay inputs/drawer warning remain outside scope.

## Current trunk merge and red → green

- **`f538a96df5a7f4e52ab059fe8b85f97212f5478a`** merges trunk after LC-U2a/PR22 landed. Sixteen conflicts resolve as a union. Eight dependency files and three selector files are byte-identical to trunk. CommentStream keeps trunk plus three W3 additions; BuyerPanel keeps trunk A13 semantics plus existing W3 restriction controls. The proxy retains both grammars. `trunk-resume/merge-proof.json` records comparisons; independent read-only source audits found no merge-specific issue.
- Registry uses one `--browser-live-settings` lc_prepare/lc_run/lc_build/lc_fixture block. All 83 previous entries retain their commands; only the W3 entry is added (84 entries including foundation). The new block follows trunk's Go 1.27.2 pin. **`2d28bce7431d405170db586840147c588efca517`** updates the exact-command test's one toolchain literal after its RED; every command/env/timeout assertion remains.
- First post-merge browser run had six manual-flow failures and six passes. Trunk's A2 parser now requires row `seq` and page `scan_exhausted`; the old W3 MOCK response omitted both, so no comment controls rendered. **`9fbe930193e1fd194e63e395155f78abe37e1e80`** first adds an actual Request/handler regression (RED), then adds only `seq: 1` and `scan_exhausted: true` to the complete fixture page (GREEN). Production parser/UI, browser waits and assertions are unchanged.
- Fresh `git fetch origin` and `git merge origin/r3/integration` before packaging both exit 0; merge says already up to date.

Earlier adapter/UI commits and evidence remain: adapter `123b9ff0`; separate OpenAPI `20d8b0c77eb175aced89f003e4d96563b0b8febf` (`contracts/live-settings-openapi.json`, integrator owns final merge); browser `8a2493d1`/`53b924bf`; UI/BFF `25a16b99`; proxy `bf96e1f4`; privacy `9c00233e`/`115bd202`. No new migration/ACL/dependency change in this resume.

## Exact current gates

Review logs are plaintext under `trunk-resume/`; `LOGS.json` maps them to full uncompressed raw logs and both SHA-256 hashes. Full raw logs are at `/Volumes/data/live_commerce_architecture_v1/output/w3-u2-live-settings-ui/trunk-resume/logs/`. `local-gates-current.json` binds all four local gates to tested source 9fbe9301.

| Command | Exit | Evidence/result |
| --- | ---: | --- |
| `bash scripts/dev/test-node.sh` before toolchain assertion alignment | 1 | `node-toolchain-red.log`; exactly one obsolete command-pin assertion |
| `node --test --experimental-strip-types tests/admin/live-settings-model.test.ts` | 0 | `toolchain-green.log`; command assertion retained |
| `GOTOOLCHAIN=go1.27.2 go test -tags browser -run '^TestBrowserLiveSettingsMockCommentsContract$' -count=1 -v ./tests/foundation` before fixture change | 1 | `fixture-comments-red.log`; missing seq and scan_exhausted |
| `GOTOOLCHAIN=go1.27.2 go test -tags browser -run '^TestBrowserLiveSettingsMock' -count=1 -v ./tests/foundation` | 0 | `fixture-comments-green.log`; both fixture contract tests pass |
| `bash scripts/dev/test-node.sh` | 0 | `node-current.log`; **1282 PASS / 0 FAIL / 0 SKIP** |
| `pnpm typecheck:admin` | 0 | `typecheck-current.log`; strict TS |
| `bash scripts/dev/test-local.sh --list` | 0 | `registry-list-current.log`; new registry mode listed |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates-current.log`; **83 documented modes**, all tracked tests run, headers green |
| `LC_TEST_LOCK_WAIT=600 bash scripts/dev/test-local.sh --browser-live-settings` | 1 → 0 | `browser-live-settings.log` / `browser-live-settings-green.log`; **6/12 → 12/12** |
| `bash scripts/dev/test-focused.sh 'TestLiveSettingsHTTP' ./tests/foundation ./internal/httpapi` | 0 | `http-focused.log`; **2 top-level, 24 subcases, 0 skips**, REAL_PG handler/403/404/409/CAS/replay/template rules |

Browser run `20261009T145356.100204000Z` uses the production Next/BFF, real PG signed authorization and MOCK business upstream. It executes three locales at 1440/390: manual operations, clipboard, readonly, hidden/store/CSRF boundaries, CAS and UNKNOWN. `trunk-resume/browser-evidence.json` hashes 12 passing per-test ledgers plus a separate native aggregate; the aggregate is not a thirteenth test. Six screenshots mask private rows/drafts/panel. Full uncompressed evidence:

- GREEN: `/Volumes/data/live_commerce_architecture_v1/output/w3-u2-live-settings-ui/trunk-resume/browser-green/`.
- RED: `/Volumes/data/live_commerce_architecture_v1/output/w3-u2-live-settings-ui/trunk-resume/browser-red/`.

Earlier valid red logs remain under this unit: BFF, receipt, proxy, restriction404, UNKNOWN, departure, actual HTTP and original browser cases. Old `registry-resume/` is historical evidence, superseded for current gate counts/source by `trunk-resume/`.

## Evidence class / CI gates / NOT_RUN

**E3: completed in the tested MOCK + REAL_PG environment**, bound to 9fbe9301 and source hashes. Browser business handlers are MOCK; the separate actual adapter REAL_PG test proves persistence. No provider LIVE or deployment claim. Read-only explorer audits are E1 source/evidence reviews, not an independent runtime rerun or new K3 PASS.

CI gates needed: the real `node scripts/dev/pr-modes.mjs origin/r3/integration` exits 0 and selects 54 modes including foundation-shards and `--browser-live-settings`; exact selection is in `trunk-resume/pr-modes.json`. Shared-component coverage includes live-console/inbox/live-claims/admin-shell/click-sweep. GitHub CI and independent review on this updated head are integrator-owned and pending. Other browser modes, full foundation, provider SANDBOX/LIVE and deployment are NOT_RUN locally in this resume. The requested local browser gate is explicitly authorized by the owner/RULINGS.

No local acceptance blocker remains. READINESS.md is historical and superseded by RULINGS.md. Owned runners/PG/Next/browser processes completed and cleaned up; shared caches and other agents' processes were untouched. Commit, stop, no push. IG seq-tie 0168 remains queued until its brief arrives.
