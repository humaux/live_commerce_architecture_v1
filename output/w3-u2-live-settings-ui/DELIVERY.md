# W3-U2 live settings delivery

- Branch/worktree: `unit/w3-u2-live-settings-ui`, `.worktrees/w3-u2-live-settings-ui`.
- Tested source: **`078e7920`**, PR22 registry dependency `76224b283a9f0b4009225c2b87c2103b5cee7e94`; final commit adds evidence only. `SOURCE.json` binds changed source hashes.
- Initial dependency `89499013`; LC-U2a round-2 privacy dependency `5f888bca` merged as `7dd45b1f`. UI privacy source115bd202 is retained unchanged by this registry merge.
- Author: Codex-4, exact parent model/effort not exposed. Isolated commerce_worker/test_worker collaborators implemented the authorized HTTP adapter and independent browser tests; gpt-6.1-sol/high. Bounded read-only review found and closed three P1s; report `REVIEW.md`. Integrator K3 remains required.
- Current coordination task `75a25502-95a7-49c4-b6c3-023e75beacdb`; parent662ad08e. No push.

## Result

`LiveSettings.tsx` provides manual reminder trigger/report/copy, merchant template publication followed by sold-out CAS save, and blocklist pagination/confirmed removal. Three locales zh-TW/zh-CN/en preserve the required24-hour and one-private-reply policy copy. Restricted follow-up rows have no copy action; blocklist displays platform/note/source bundle/time without inventing a display name. BuyerPanel restriction add/check/badge and CommentStream sold-out marks use actual frozen fields.

Dedicated BFF leaves use real signed-session/Origin/CSRF/store helpers and a closed method/body/query/response grammar. The actual Next proxy admits that grammar before the older Studio whitelist. GET/PUT sold-out-reply is a thin auth-scoped Go adapter over existing definers: live:read/live:manage, published merchant template ID/version, CAS expected_version and transactional idempotency receipt. Fixed templates remain immutable. Automatic reminder controls, delay inputs and drawer warning are excluded by RULINGS.

Private state clears on hide/departure/unmount; stale replies cannot repaint. Nested restriction404 revokes the whole containing console. UNKNOWN admits only its retained exact receipt. A captured old callback must pass the current fence before changing refs/state or sending. Three independent P1s were reproduced RED→GREEN before115bd202; `REVIEW.md` independently replays the departure counterexample successfully.

## Registry integration

Merge078e7920 resolves the runner conflict using PR22's native case registry. `--browser-live-settings` is **one** lc_build=admin / lc_fixture=pg / lc_prepare / lc_run block. Its prepare and Go command/env/timeout retain the earlier mode behavior. Usage, admission, build and dispatch derive from that block; no old if/elif layout remains.

A real CLI metadata test first fails with the missing mode, then passes. All83 pre-existing entries retain byte-identical prepare/run and build/fixture values; only live-settings is added (84 entries including foundation). `registry-resume/equivalence.json` records this. The earlier comparison of whole parser body included the displaced trailing APPEND comment; command-function/metadata comparison excludes that comment and is exact. No selector hand-list was introduced.

PR22 was verified **OPEN**, head76224b28, at handoff; origin/r3/integration remains50086616. Its branch is merged here. After PR22 lands, merge origin/r3/integration again per owner ruling; never rebase/force-push.

## Commits/interface

- Adapter:123b9ff0; **separate OpenAPI commit20d8b0c77eb175aced89f003e4d96563b0b8febf**, `contracts/live-settings-openapi.json`, for integrator final merge.
- Browser:8a2493d1+53b924bf; UI/BFF25a16b99; proxybf96e1f4; privacy/receipt9c00233e+115bd202; registry merge078e7920.
- Additive GET/PUT `/v1/admin/stores/{store_id}/live-settings/sold-out-reply`. Exact PUT `{enabled,template_id,template_version,expected_version}` + Idempotency-Key. Both return `{enabled,template_id,template_version,version}`. No migration/ACL/dependency changes. Existing top-level null422/field-null400 behavior preserved.

## Exact current gates

Commands, exits and plaintext log hashes: **`registry-resume/results.json`**. Browser source manifest and masked evidence are in `registry-resume/browser-evidence/`.

| Command | Exit | Result |
| --- | ---: | --- |
| `node --test --experimental-strip-types tests/admin/live-settings-model.test.ts` before registry entry | 1 | missing-mode RED, red.log |
| `node --test --experimental-strip-types tests/admin/live-settings-model.test.ts tests/ci/mode-registry.test.mjs tests/ci/pr-modes.test.mjs` | 0 | 33 PASS, green.log |
| `bash -n scripts/dev/test-local.sh` | 0 | native Bash syntax |
| Real registry comparison vs PR22 head | 0 |83 existing entries unchanged, only1 addition |
| `bash scripts/dev/test-node.sh` | 0 | **1219 PASS /0 FAIL /0 SKIP** |
| `pnpm typecheck:admin` | 0 | strict TypeScript |
| `bash scripts/dev/check-gates.sh` | 0 | **83 modes documented** (foundation excluded), all tracked tests assigned, headers green |
| `bash scripts/dev/test-local.sh --browser-live-settings` | 0 | **12/12 real-click cases** in production Next/BFF, three locales1440/390, native hidden/store/CSRF/readonly/CAS/UNKNOWN/clipboard |
| `bash scripts/dev/test-focused.sh 'TestLiveSettingsHTTP' ./tests/foundation ./internal/httpapi` | 0 | **2 top-level,24 subcases,0 skipped**, actual handler/PG403/404/409/CAS/replay/template rules |

Earlier valid RED evidence remains uncompressed: bff-red.log, receipt-red.log, proxy-red.log, restriction-404-red.log, unknown-red.log, departure-red.log, http/red-all.log, browser/red.log and browser/fixed-fixture-red.log. Earlier failed browser attempts were missing page/proxy404; their filenames/history are retained, not counted as current green. Historical child worktree missing-dependency and strip-vs-transform diagnostic failures remain in child delivery notes; final prepared parent gates above pass.

## Evidence boundary / NOT_RUN

**E3: tested REAL_PG + MOCK environment**, bound to078e7920/source hashes. Browser authorization uses real PG and signed MOCK OIDC, business endpoints are explicitly MOCK; separate REAL_PG adapter tests prove actual persistence. No provider LIVE or deployment claim. Six screenshots deliberately mask private rows/drafts/panel; desktop/mobile layout was inspected, all locale/width overflow assertions pass.

- K3 pre-review and GitHub CI on this batch: integrator pending. Commit, stop, no push.
- Broader original-brief/shared-component regressions `--browser-live-claims`, `--browser-admin-shell`, `--browser-click-sweep`, `--browser-live-console`, `--browser-inbox`: NOT_RUN locally in this unit; run in CI under the owner RAM rule. Local new feature gate was explicitly authorized by RULINGS.
- Full foundation, provider SANDBOX/LIVE, production deployment: NOT_RUN.
- Automatic reminders and drawer warning remain integrator follow-ups. READINESS.md is historical and superseded by RULINGS. No unresolved local acceptance blocker.
- All owned runners/PG/Next/browser processes cleaned up. Shared caches and other agents' processes untouched. Evidence logs retain raw whitespace/CR lines, so full evidence-only git diff whitespace check may report those; source-only check exits0.
